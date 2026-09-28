package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/provider"
	"kwu/internal/store"
)

func TestExpertMetaPersistenceSerializesSnapshotAndMemberCommit(t *testing.T) {
	dir := t.TempDir()
	c, _ := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	if err := c.resetCurrentExpertTeamBindings(); err != nil {
		t.Fatalf("resetCurrentExpertTeamBindings: %v", err)
	}

	originalSave := saveControllerExpertMeta
	oldSaveEntered := make(chan struct{})
	releaseOldSave := make(chan struct{})
	var oldOnce sync.Once
	saveControllerExpertMeta = func(path string, meta agent.ExpertMeta) error {
		member, _ := expertTeamMemberByName(meta, "代码审查专家")
		if member.SubagentRef == "" {
			oldOnce.Do(func() { close(oldSaveEntered) })
			<-releaseOldSave
		}
		return originalSave(path, meta)
	}
	t.Cleanup(func() { saveControllerExpertMeta = originalSave })

	oldSaveDone := make(chan struct{})
	go func() {
		defer close(oldSaveDone)
		c.saveExpertMeta(c.SessionPath())
	}()
	select {
	case <-oldSaveEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("old snapshot save did not reach the blocking hook")
	}

	runtime := controllerExpertTeamRuntime{c: c}
	commitLockAttempted := make(chan struct{})
	var commitAttemptOnce sync.Once
	c.expertMetaPersistenceMu.lockAttempt = func() {
		commitAttemptOnce.Do(func() { close(commitLockAttempted) })
	}
	commitDone := make(chan error, 1)
	go func() { commitDone <- runtime.CommitExpertTeamMember("代码审查专家", "sa_fresh_review") }()
	select {
	case <-commitLockAttempted:
	case <-time.After(3 * time.Second):
		t.Fatal("member commit did not attempt to acquire the persistence lock")
	}
	close(releaseOldSave)

	select {
	case <-oldSaveDone:
	case <-time.After(3 * time.Second):
		t.Fatal("old snapshot save deadlocked")
	}
	select {
	case err := <-commitDone:
		if err != nil {
			t.Fatalf("CommitExpertTeamMember: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("member commit deadlocked")
	}
	c.expertMetaPersistenceMu.lockAttempt = nil

	persisted, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	assertExpertMemberRef(t, persisted, "代码审查专家", "sa_fresh_review")
	if !persisted.TeamBindingsReset {
		t.Fatal("persisted reset marker was lost")
	}
	c.mu.Lock()
	inMemory := cloneExpertMeta(c.expertMeta)
	c.mu.Unlock()
	assertExpertMemberRef(t, inMemory, "代码审查专家", "sa_fresh_review")
	if !inMemory.TeamBindingsReset {
		t.Fatal("in-memory reset marker was lost")
	}
}

func TestEmptyExpertSnapshotPreservesExistingTeamSidecar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team.jsonl")
	meta := agent.ExpertMeta{
		ExpertID: "code-delivery-team", ExpertName: "代码交付专家团",
		TeamState: agent.ExpertTeamActive,
		Members:   []agent.ExpertTeamMemberMeta{{MemberKey: "expert:testing", Name: "测试策略专家", SubagentRef: "sa_history"}},
	}
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatalf("SaveSessionExpert: %v", err)
	}
	before, err := os.ReadFile(store.SessionExpert(path))
	if err != nil {
		t.Fatalf("read team sidecar before empty snapshot: %v", err)
	}

	c, _ := newExpertTestController(t, dir, "")
	c.mu.Lock()
	c.sessionPath = path
	c.expert = expert.Selection{Source: expert.SourceNone}
	c.expertMeta = agent.ExpertMeta{}
	c.mu.Unlock()
	c.saveExpertMeta(path)

	after, err := os.ReadFile(store.SessionExpert(path))
	if err != nil {
		t.Fatalf("read team sidecar after empty snapshot: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("empty snapshot changed team sidecar: before=%s after=%s", before, after)
	}
}

func TestExpertAutosaveDoesNotOverwriteCorruptSidecar(t *testing.T) {
	dir := t.TempDir()
	c, _ := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	sidecar := store.SessionExpert(c.SessionPath())
	corrupt := []byte("{not-json")
	if err := os.WriteFile(sidecar, corrupt, 0o644); err != nil {
		t.Fatalf("write corrupt sidecar: %v", err)
	}
	var notices []string
	c.sink = event.FuncSink(func(e event.Event) { notices = append(notices, e.Text) })

	c.saveExpertMeta(c.SessionPath())

	after, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("read corrupt sidecar after autosave: %v", err)
	}
	if !bytes.Equal(after, corrupt) {
		t.Fatalf("autosave overwrote corrupt sidecar: got %q want %q", after, corrupt)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "save expert metadata failed") {
		t.Fatalf("autosave notices = %v, want one diagnostic error", notices)
	}
}

func TestResumeUpgradesLegacyExpertTeamWithOwnedTaskBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy-team.jsonl")
	session := agent.NewSession("legacy system")
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
		{ID: "call-review", Name: "task", Arguments: `{"member_name":"代码审查专家","prompt":"review"}`},
		{ID: "call-testing", Name: "task", Arguments: `{"member_name":"测试策略专家","prompt":"test"}`},
		{ID: "call-ordinary", Name: "task", Arguments: `{"description":"ordinary task","prompt":"work"}`},
	}})
	if err := session.Save(path); err != nil {
		t.Fatalf("save parent session: %v", err)
	}
	legacy := agent.ExpertMeta{ExpertID: "code-delivery-team", ExpertName: "代码交付专家团", Source: "session"}
	if err := agent.SaveSessionExpert(path, legacy); err != nil {
		t.Fatalf("save legacy expert metadata: %v", err)
	}

	owned := writeExpertRecoverySubagent(t, dir, "sa_owned_review", agent.SubagentMeta{
		Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "call-review",
	})
	unowned := writeExpertRecoverySubagent(t, dir, "sa_unowned_testing", agent.SubagentMeta{
		Kind: "task", ParentSession: "another-parent", ParentToolCallID: "call-testing",
	})
	ordinary := writeExpertRecoverySubagent(t, dir, "sa_ordinary", agent.SubagentMeta{
		Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "call-ordinary",
	})
	before := snapshotExpertRecoveryFiles(t, append(append(owned, unowned...), ordinary...))

	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatalf("load parent session: %v", err)
	}
	c, _ := newExpertTestController(t, dir, "")
	c.Resume(loaded, path)

	got, ok, err := agent.LoadSessionExpert(path)
	if err != nil || !ok {
		t.Fatalf("load upgraded expert metadata: meta=%+v ok=%v err=%v", got, ok, err)
	}
	if got.TeamState != agent.ExpertTeamActive {
		t.Fatalf("team state = %q, want %q", got.TeamState, agent.ExpertTeamActive)
	}
	if got.TeamBindingsReset {
		t.Fatal("legacy sidecar without reset marker was treated as reset")
	}
	assertExpertMemberRef(t, got, "代码审查专家", "sa_owned_review")
	assertExpertMemberRef(t, got, "测试策略专家", "")
	assertExpertMemberRef(t, got, "发布风险顾问", "")
	assertExpertRecoveryFilesUnchanged(t, before)
}

func TestResumeRepairsIncompleteExpertTeamWithoutReplacingOrGuessingBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "incomplete-team.jsonl")
	session := agent.NewSession("team system")
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
		{ID: "call-testing-a", Name: "task", Arguments: `{"member_name":"测试策略专家","prompt":"first"}`},
		{ID: "call-testing-b", Name: "task", Arguments: `{"member_name":"测试策略专家","prompt":"second"}`},
		{ID: "call-release", Name: "task", Arguments: `{"member_name":"发布风险顾问","prompt":"release"}`},
	}})
	if err := session.Save(path); err != nil {
		t.Fatalf("save parent session: %v", err)
	}
	c, _ := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("select expert team: %v", err)
	}
	meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("load seeded expert metadata: meta=%+v ok=%v err=%v", meta, ok, err)
	}
	meta = setExpertMemberRef(t, meta, "代码审查专家", "sa_existing_review")
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatalf("save incomplete expert metadata: %v", err)
	}

	files := [][]string{
		writeExpertRecoverySubagent(t, dir, "sa_existing_review", agent.SubagentMeta{Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "obsolete-call"}),
		writeExpertRecoverySubagent(t, dir, "sa_testing_a", agent.SubagentMeta{Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "call-testing-a"}),
		writeExpertRecoverySubagent(t, dir, "sa_testing_b", agent.SubagentMeta{Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "call-testing-b"}),
		writeExpertRecoverySubagent(t, dir, "sa_release", agent.SubagentMeta{Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "call-release"}),
		writeExpertRecoverySubagent(t, dir, "sa_wrong_kind", agent.SubagentMeta{Kind: "skill", ParentSession: agent.BranchID(path), ParentToolCallID: "call-release"}),
	}
	var paths []string
	for _, pair := range files {
		paths = append(paths, pair...)
	}
	before := snapshotExpertRecoveryFiles(t, paths)

	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatalf("load parent session: %v", err)
	}
	resumed, _ := newExpertTestController(t, dir, "")
	resumed.Resume(loaded, path)

	got, ok, err := agent.LoadSessionExpert(path)
	if err != nil || !ok {
		t.Fatalf("load repaired expert metadata: meta=%+v ok=%v err=%v", got, ok, err)
	}
	assertExpertMemberRef(t, got, "代码审查专家", "sa_existing_review")
	assertExpertMemberRef(t, got, "测试策略专家", "")
	assertExpertMemberRef(t, got, "发布风险顾问", "sa_release")
	assertExpertRecoveryFilesUnchanged(t, before)
}

func TestResumeExpertTeamRepairSaveFailureKeepsPersistedAndMemoryMetaUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save-failure.jsonl")
	session := agent.NewSession("legacy system")
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "call-review", Name: "task", Arguments: `{"member_name":"代码审查专家","prompt":"review"}`,
	}}})
	if err := session.Save(path); err != nil {
		t.Fatalf("save parent session: %v", err)
	}
	legacy := agent.ExpertMeta{ExpertID: "code-delivery-team", ExpertName: "代码交付专家团", Source: "session"}
	if err := agent.SaveSessionExpert(path, legacy); err != nil {
		t.Fatalf("save legacy expert metadata: %v", err)
	}
	writeExpertRecoverySubagent(t, dir, "sa_owned_review", agent.SubagentMeta{
		Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "call-review",
	})
	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatalf("load parent session: %v", err)
	}

	originalSave := saveControllerExpertMeta
	saveControllerExpertMeta = func(string, agent.ExpertMeta) error { return errors.New("injected restore save failure") }
	t.Cleanup(func() { saveControllerExpertMeta = originalSave })
	var notices []string
	c, _ := newExpertTestController(t, dir, "")
	c.sink = event.FuncSink(func(e event.Event) { notices = append(notices, e.Text) })
	c.Resume(loaded, path)

	persisted, ok, err := agent.LoadSessionExpert(path)
	if err != nil || !ok {
		t.Fatalf("load expert metadata after failed repair: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	if persisted.TeamState != "" || len(persisted.Members) != 0 {
		t.Fatalf("failed repair changed persisted metadata: %+v", persisted)
	}
	c.mu.Lock()
	inMemory := cloneExpertMeta(c.expertMeta)
	c.mu.Unlock()
	if inMemory.TeamState != "" || len(inMemory.Members) != 0 {
		t.Fatalf("failed repair claimed unpersisted metadata in memory: %+v", inMemory)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "injected restore save failure") {
		t.Fatalf("repair notices = %v, want one save-failure warning", notices)
	}
}

func TestExpertTeamTaskMembersRejectsMalformedAndAmbiguousCalls(t *testing.T) {
	messages := []provider.Message{{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
		{ID: "valid", Name: "task", Arguments: `{"member_name":" 代码审查专家 "}`},
		{ID: "same", Name: "task", Arguments: `{"member_name":"测试策略专家"}`},
		{ID: "same", Name: "task", Arguments: `{"member_name":"测试策略专家"}`},
		{ID: "ambiguous", Name: "task", Arguments: `{"member_name":"测试策略专家"}`},
		{ID: "ambiguous", Name: "task", Arguments: `{"member_name":"发布风险顾问"}`},
		{ID: "malformed", Name: "task", Arguments: `{`},
		{ID: "missing", Name: "task", Arguments: `{"prompt":"work"}`},
		{ID: "empty", Name: "task", Arguments: `{"member_name":" "}`},
		{ID: "wrong-tool", Name: "parallel_tasks", Arguments: `{"member_name":"架构顾问"}`},
		{ID: "", Name: "task", Arguments: `{"member_name":"架构顾问"}`},
	}}}

	got := expertTeamTaskMembers(messages)
	want := map[string]string{
		"valid": "代码审查专家",
		"same":  "测试策略专家",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("task members = %#v, want %#v", got, want)
	}
}

func writeExpertRecoverySubagent(t *testing.T, sessionDir, ref string, meta agent.SubagentMeta) []string {
	t.Helper()
	dir := filepath.Join(sessionDir, "subagents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create subagent directory: %v", err)
	}
	meta.Ref = ref
	sessionPath := filepath.Join(dir, ref+".jsonl")
	metaPath := filepath.Join(dir, ref+".meta.json")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"system","content":"child history"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write subagent session %s: %v", ref, err)
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal subagent metadata %s: %v", ref, err)
	}
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatalf("write subagent metadata %s: %v", ref, err)
	}
	return []string{sessionPath, metaPath}
}

func snapshotExpertRecoveryFiles(t *testing.T, paths []string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read recovery fixture %s: %v", path, err)
		}
		result[path] = data
	}
	return result
}

func assertExpertRecoveryFilesUnchanged(t *testing.T, before map[string][]byte) {
	t.Helper()
	for path, want := range before {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read recovery fixture after resume %s: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("recovery rewrote child artifact %s", path)
		}
	}
}

func assertExpertMemberRef(t *testing.T, meta agent.ExpertMeta, memberName, want string) {
	t.Helper()
	member, ok := expertTeamMemberByName(meta, memberName)
	if !ok || member.SubagentRef != want {
		t.Fatalf("member %q = %+v found=%v, want ref %q", memberName, member, ok, want)
	}
}

func setExpertMemberRef(t *testing.T, meta agent.ExpertMeta, memberName, ref string) agent.ExpertMeta {
	t.Helper()
	for i := range meta.Members {
		if meta.Members[i].Name == memberName {
			meta.Members[i].SubagentRef = ref
			return meta
		}
	}
	t.Fatalf("member %q not found in %+v", memberName, meta.Members)
	return meta
}
