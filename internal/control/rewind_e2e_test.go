package control

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/store"
	"kwu/internal/tool"
)

func runTwoTurns(t *testing.T) (*Controller, *agent.Agent, *[]event.Event) {
	t.Helper()
	dir := t.TempDir()
	prov := &scriptedTurns{turns: [][]provider.Chunk{
		textTurn("first answer"),
		textTurn("second answer"),
	}}
	ag := agent.New(prov, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{}, event.Discard)
	var events []event.Event
	c := New(Options{
		Runner:     ag,
		Executor:   ag,
		SessionDir: dir,
		Label:      "test",
		Sink:       event.FuncSink(func(e event.Event) { events = append(events, e) }),
	})
	c.SetSessionPath(agent.NewSessionPath(dir, "test"))
	if err := c.runTurnWithRaw(context.Background(), "first prompt", "first prompt"); err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if err := c.runTurnWithRaw(context.Background(), "second prompt", "second prompt"); err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	return c, ag, &events
}

type compactNowRunner struct {
	agent *agent.Agent
}

func (r compactNowRunner) Run(ctx context.Context, _ string) error {
	return r.agent.CompactNow(ctx, "")
}

func newCheckpointCompactionController(t *testing.T, compactable bool) (*Controller, *agent.Agent) {
	t.Helper()
	session := agent.NewSession("sys")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "first prompt"})
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: strings.Repeat("older work ", 80)})
	if compactable {
		session.Add(provider.Message{Role: provider.RoleUser, Content: "second prompt"})
		session.Add(provider.Message{Role: provider.RoleAssistant, Content: strings.Repeat("more older work ", 80)})
		session.Add(provider.Message{Role: provider.RoleUser, Content: "recent prompt"})
		session.Add(provider.Message{Role: provider.RoleAssistant, Content: "recent answer"})
	}
	prov := &scriptedTurns{turns: [][]provider.Chunk{textTurn("summary")}}
	ag := agent.New(prov, tool.NewRegistry(), session, agent.Options{RecentKeep: 2}, event.Discard)
	sessionDir := t.TempDir()
	sessionPath := agent.NewSessionPath(sessionDir, "checkpoint-compaction")
	c := New(Options{
		Runner: compactNowRunner{agent: ag}, Executor: ag, SessionDir: sessionDir, SessionPath: sessionPath,
	})
	t.Cleanup(c.Close)
	c.checkpoints.begin("before compaction", 1)
	return c, ag
}

func TestManualCompactionClearsCheckpointBoundariesOnlyAfterStructuralRewrite(t *testing.T) {
	for _, tc := range []struct {
		name        string
		compactable bool
		wantCleared bool
	}{
		{name: "compaction committed", compactable: true, wantCleared: true},
		{name: "compaction no-op", compactable: false, wantCleared: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, ag := newCheckpointCompactionController(t, tc.compactable)
			before := ag.Session().CompactionVersion()
			if err := c.Compact(context.Background(), ""); err != nil {
				t.Fatalf("Compact: %v", err)
			}
			_, hasBoundary := c.checkpoints.boundary(0)
			if gotCleared := !hasBoundary; gotCleared != tc.wantCleared {
				t.Fatalf("checkpoint cleared = %v, want %v", gotCleared, tc.wantCleared)
			}
			if changed := ag.Session().CompactionVersion() != before; changed != tc.wantCleared {
				t.Fatalf("compaction version changed = %v, want %v", changed, tc.wantCleared)
			}
		})
	}
}

func TestRunRootRunnerClearsCheckpointBoundariesAfterInTurnCompaction(t *testing.T) {
	c, _ := newCheckpointCompactionController(t, true)
	if err := c.runRootRunner(context.Background(), "ignored"); err != nil {
		t.Fatalf("runRootRunner: %v", err)
	}
	if _, ok := c.checkpoints.boundary(0); ok {
		t.Fatal("in-turn compaction left a stale conversation checkpoint boundary")
	}
}

func TestCompactionBoundaryInvalidationSurvivesControllerRestart(t *testing.T) {
	c, ag := newCheckpointCompactionController(t, true)
	if err := c.Compact(context.Background(), ""); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if err := ag.Session().Save(c.SessionPath()); err != nil {
		t.Fatalf("save compacted transcript: %v", err)
	}
	loaded, err := agent.LoadSession(c.SessionPath())
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	reloadedAgent := agent.New(&scriptedTurns{turns: [][]provider.Chunk{textTurn("done")}}, tool.NewRegistry(), loaded, agent.Options{}, event.Discard)
	reloaded := New(Options{
		Runner: reloadedAgent, Executor: reloadedAgent, SessionDir: filepath.Dir(c.SessionPath()), SessionPath: c.SessionPath(),
	})
	t.Cleanup(reloaded.Close)
	if reloaded.CheckpointHasBoundary(0) {
		t.Fatal("restart revived a pre-compaction conversation boundary")
	}
	if checkpoints := reloaded.Checkpoints(); len(checkpoints) != 1 || checkpoints[0].Turn != 0 {
		t.Fatalf("restart removed code checkpoint metadata: %+v", checkpoints)
	}
	reloaded.checkpoints.begin("after compaction", len(loaded.Snapshot()))
	if !reloaded.CheckpointHasBoundary(1) {
		t.Fatal("new post-compaction turn did not establish a conversation boundary")
	}
}

// TestRewindConversationFailsLoudlyAfterCompaction reproduces #3598: once
// compaction shrinks the message log below a turn's recorded boundary, a
// conversation/both rewind to that turn skipped the truncation but still emitted
// a success notice — code rolled back, conversation silently did not.
func TestRewindConversationFailsLoudlyAfterCompaction(t *testing.T) {
	c, ag, events := runTwoTurns(t)

	c.checkpoints.mu.Lock()
	lastTurn := c.checkpoints.turn - 1
	boundary := c.checkpoints.bound[lastTurn]
	c.checkpoints.mu.Unlock()
	if boundary <= 1 {
		t.Fatalf("expected the latest turn's boundary above 1, got bound=%v", c.checkpoints.bound)
	}

	// Auto-compaction replaces the prefix with a summary, shrinking the log below
	// the recorded boundary; compaction does not rewrite checkpoint boundaries.
	sess := ag.Session()
	sess.Messages = []provider.Message{{Role: provider.RoleUser, Content: "summary"}}

	*events = nil
	err := c.Rewind(lastTurn, RewindBoth)
	if err == nil || !strings.Contains(err.Error(), "compacted") {
		t.Fatalf("Rewind after compaction error = %v, want a 'compacted past' failure", err)
	}
	for _, e := range *events {
		if e.Kind == event.Notice && strings.Contains(e.Text, "rewound conversation") {
			t.Fatalf("emitted a false conversation-rewind success after skipping truncation: %q", e.Text)
		}
	}
	if got := len(ag.Session().Messages); got != 1 {
		t.Fatalf("session messages = %d, want the compacted log left intact at 1", got)
	}
}

// TestRewindConversationSucceedsWithLiveBoundary is the companion happy path: a
// boundary still within the log truncates the conversation and reports success.
func TestRewindConversationSucceedsWithLiveBoundary(t *testing.T) {
	c, ag, events := runTwoTurns(t)

	c.checkpoints.mu.Lock()
	lastTurn := c.checkpoints.turn - 1
	boundary := c.checkpoints.bound[lastTurn]
	c.checkpoints.mu.Unlock()

	*events = nil
	if err := c.Rewind(lastTurn, RewindConversation); err != nil {
		t.Fatalf("Rewind with a live boundary: %v", err)
	}
	if got := len(ag.Session().Messages); got != boundary {
		t.Fatalf("session truncated to %d messages, want boundary %d", got, boundary)
	}
	ok := false
	for _, e := range *events {
		if e.Kind == event.Notice && strings.Contains(e.Text, "rewound conversation") {
			ok = true
		}
	}
	if !ok {
		t.Fatal("expected a conversation-rewind success notice")
	}
}

func TestRewindConversationClearsArchivedExpertBindingsAndPreservesOwnedArtifacts(t *testing.T) {
	c, ag, _ := runTwoTurns(t)
	meta, artifacts := installRewindExpertTeam(t, c, agent.ExpertTeamArchived)
	beforeArtifacts := snapshotExpertRecoveryFiles(t, artifacts)
	lastTurn, boundary := latestRewindTurn(t, c)

	if err := c.Rewind(lastTurn, RewindConversation); err != nil {
		t.Fatalf("Rewind archived expert team: %v", err)
	}
	if got := len(ag.Session().Snapshot()); got != boundary {
		t.Fatalf("session messages = %d, want rewound boundary %d", got, boundary)
	}

	persisted, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert after rewind: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	assertRewoundExpertTeam(t, persisted, meta)
	if !persisted.TeamBindingsReset {
		t.Fatal("archived rewind did not persist the binding reset marker")
	}
	assertExpertRecoveryFilesUnchanged(t, beforeArtifacts)
}

func TestRewindConversationClearsActiveExpertBindingsAndNextMemberStartsFresh(t *testing.T) {
	c, ag, _ := runTwoTurns(t)
	messages := ag.Session().Snapshot()
	if len(messages) < 3 || messages[2].Role != provider.RoleAssistant {
		t.Fatalf("unexpected first-turn transcript: %+v", messages)
	}
	messages[2].ToolCalls = []provider.ToolCall{{
		ID: "task-rewind-review", Name: "task", Arguments: `{"member_name":"代码审查专家","prompt":"review"}`,
	}}
	ag.Session().Replace(messages)
	meta, artifacts := installRewindExpertTeam(t, c, agent.ExpertTeamActive)
	beforeArtifacts := snapshotExpertRecoveryFiles(t, artifacts)
	lastTurn, _ := latestRewindTurn(t, c)

	if err := c.Rewind(lastTurn, RewindConversation); err != nil {
		t.Fatalf("Rewind active expert team: %v", err)
	}
	persisted, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert after rewind: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	assertRewoundExpertTeam(t, persisted, meta)
	if !persisted.TeamBindingsReset {
		t.Fatal("active rewind did not persist the binding reset marker")
	}
	assertExpertRecoveryFilesUnchanged(t, beforeArtifacts)

	runtime := controllerExpertTeamRuntime{c: c}
	ref, err := runtime.AcquireExpertTeamMember("代码审查专家")
	if err != nil || ref != "" {
		t.Fatalf("member acquire after rewind = %q, %v; want fresh empty binding", ref, err)
	}
	runtime.ReleaseExpertTeamMember("代码审查专家")

	ag.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "ordinary snapshot after rewind"})
	if err := c.Snapshot(); err != nil {
		t.Fatalf("ordinary Snapshot after rewind: %v", err)
	}
	persisted, ok, err = agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert after ordinary Snapshot: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	assertExpertMemberRef(t, persisted, "代码审查专家", "")
	if !persisted.TeamBindingsReset {
		t.Fatal("ordinary Snapshot did not preserve the binding reset marker")
	}

	loaded, err := agent.LoadSession(c.SessionPath())
	if err != nil {
		t.Fatalf("LoadSession after ordinary Snapshot: %v", err)
	}
	resumed, _ := newExpertTestController(t, filepath.Join(t.TempDir(), "resumed"), "")
	resumed.Resume(loaded, c.SessionPath())
	persisted, ok, err = agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert after Controller resume: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	assertExpertMemberRef(t, persisted, "代码审查专家", "")
	if !persisted.TeamBindingsReset {
		t.Fatal("Controller resume did not preserve the binding reset marker")
	}

	resumedRuntime := controllerExpertTeamRuntime{c: resumed}
	ref, err = resumedRuntime.AcquireExpertTeamMember("代码审查专家")
	if err != nil || ref != "" {
		t.Fatalf("member acquire after resume = %q, %v; want fresh empty binding", ref, err)
	}
	if err := resumedRuntime.CommitExpertTeamMember("代码审查专家", "sa_fresh_review"); err != nil {
		t.Fatalf("CommitExpertTeamMember after reset: %v", err)
	}
	resumedRuntime.ReleaseExpertTeamMember("代码审查专家")
	persisted, ok, err = agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert after fresh binding: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	assertExpertMemberRef(t, persisted, "代码审查专家", "sa_fresh_review")
	if !persisted.TeamBindingsReset {
		t.Fatal("explicit fresh binding cleared the binding reset marker")
	}
	assertExpertRecoveryFilesUnchanged(t, beforeArtifacts)
}

func TestRewindCodeOnlyPreservesExpertBindings(t *testing.T) {
	c, _, _ := runTwoTurns(t)
	_, artifacts := installRewindExpertTeam(t, c, agent.ExpertTeamActive)
	beforeArtifacts := snapshotExpertRecoveryFiles(t, artifacts)
	lastTurn, _ := latestRewindTurn(t, c)

	if err := c.Rewind(lastTurn, RewindCode); err != nil {
		t.Fatalf("code-only Rewind: %v", err)
	}
	persisted, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert after code rewind: meta=%+v ok=%v err=%v", persisted, ok, err)
	}
	assertExpertMemberRef(t, persisted, "代码审查专家", "sa_rewind_review")
	if persisted.TeamBindingsReset {
		t.Fatal("code-only rewind unexpectedly marked team bindings reset")
	}
	assertExpertRecoveryFilesUnchanged(t, beforeArtifacts)
}

func TestRewindConversationExpertSaveFailurePreservesParentHistory(t *testing.T) {
	c, ag, _ := runTwoTurns(t)
	_, artifacts := installRewindExpertTeam(t, c, agent.ExpertTeamActive)
	beforeArtifacts := snapshotExpertRecoveryFiles(t, artifacts)
	beforeMessages := ag.Session().Snapshot()
	beforeCheckpoints := c.Checkpoints()
	lastTurn, _ := latestRewindTurn(t, c)

	sidecar := store.SessionExpert(c.SessionPath())
	backup := sidecar + ".rewind-test-backup"
	if err := os.Rename(sidecar, backup); err != nil {
		t.Fatalf("move expert sidecar: %v", err)
	}
	if err := os.Mkdir(sidecar, 0o755); err != nil {
		t.Fatalf("replace expert sidecar with directory: %v", err)
	}
	defer func() {
		_ = os.Remove(sidecar)
		_ = os.Rename(backup, sidecar)
	}()

	err := c.Rewind(lastTurn, RewindConversation)
	if err == nil || !strings.Contains(err.Error(), "reset expert-team bindings") {
		t.Fatalf("Rewind save failure = %v, want expert binding save error", err)
	}
	if got := ag.Session().Snapshot(); !reflect.DeepEqual(got, beforeMessages) {
		t.Fatalf("parent history changed after failed sidecar save: got=%+v want=%+v", got, beforeMessages)
	}
	if got := c.Checkpoints(); !reflect.DeepEqual(got, beforeCheckpoints) {
		t.Fatalf("checkpoints changed after failed sidecar save: got=%+v want=%+v", got, beforeCheckpoints)
	}
	c.mu.Lock()
	inMemory := cloneExpertMeta(c.expertMeta)
	c.mu.Unlock()
	assertExpertMemberRef(t, inMemory, "代码审查专家", "sa_rewind_review")
	if inMemory.TeamBindingsReset {
		t.Fatal("failed sidecar save committed the binding reset marker in memory")
	}
	backupRaw, readErr := os.ReadFile(backup)
	if readErr != nil {
		t.Fatalf("read original sidecar after failed save: %v", readErr)
	}
	var backupMeta agent.ExpertMeta
	if err := json.Unmarshal(backupRaw, &backupMeta); err != nil {
		t.Fatalf("decode original sidecar after failed save: %v", err)
	}
	assertExpertMemberRef(t, backupMeta, "代码审查专家", "sa_rewind_review")
	if backupMeta.TeamBindingsReset {
		t.Fatal("failed sidecar save changed the persisted binding reset marker")
	}
	assertExpertRecoveryFilesUnchanged(t, beforeArtifacts)
}

func latestRewindTurn(t *testing.T, c *Controller) (int, int) {
	t.Helper()
	c.checkpoints.mu.Lock()
	defer c.checkpoints.mu.Unlock()
	turn := c.checkpoints.turn - 1
	boundary, ok := c.checkpoints.bound[turn]
	if turn < 0 || !ok {
		t.Fatalf("latest rewind checkpoint unavailable: turn=%d bounds=%v", turn, c.checkpoints.bound)
	}
	return turn, boundary
}

func installRewindExpertTeam(t *testing.T, c *Controller, state string) (agent.ExpertMeta, []string) {
	t.Helper()
	path := c.SessionPath()
	meta := agent.ExpertMeta{
		ExpertID: "code-delivery-team", ExpertName: "代码交付专家团", TeamState: state,
		Lead: &agent.ExpertTeamMemberMeta{MemberKey: "member:0", Name: "架构顾问", Role: "系统设计"},
		Members: []agent.ExpertTeamMemberMeta{
			{MemberKey: "member:1", Name: "代码审查专家", Role: "实现质量", SubagentRef: "sa_rewind_review"},
			{MemberKey: "member:2", Name: "测试策略专家", Role: "验证覆盖"},
		},
	}
	c.mu.Lock()
	c.expertMeta = cloneExpertMeta(meta)
	if state == agent.ExpertTeamActive {
		c.expert = expert.Selection{
			ID: meta.ExpertID, Name: meta.ExpertName, Source: expert.SourceRequest, Selected: true,
			Expert: expert.Expert{ID: meta.ExpertID, Name: meta.ExpertName, Mode: expert.ModeTeam, Members: []expert.Member{
				{Kind: expert.MemberKindLead, Name: "架构顾问", Role: "系统设计"},
				{Kind: expert.MemberKindSub, Name: "代码审查专家", Role: "实现质量"},
				{Kind: expert.MemberKindSub, Name: "测试策略专家", Role: "验证覆盖"},
			}},
		}
	}
	c.expertTeamStarts["member:1"] = true
	c.mu.Unlock()
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatalf("SaveSessionExpert fixture: %v", err)
	}
	artifacts := writeExpertRecoverySubagent(t, filepath.Dir(path), "sa_rewind_review", agent.SubagentMeta{
		Kind: "task", ParentSession: agent.BranchID(path), ParentToolCallID: "task-rewind-review",
	})
	jobDir := jobs.ArtifactDir(path)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatalf("create job artifact directory: %v", err)
	}
	jobPath := filepath.Join(jobDir, "task-rewind-review.json")
	if err := os.WriteFile(jobPath, []byte(`{"id":"task-rewind-review","status":"completed"}`), 0o644); err != nil {
		t.Fatalf("write job artifact: %v", err)
	}
	return meta, append(artifacts, jobPath)
}

func assertRewoundExpertTeam(t *testing.T, got, want agent.ExpertMeta) {
	t.Helper()
	if got.TeamState != want.TeamState || got.ExpertID != want.ExpertID || got.ExpertName != want.ExpertName {
		t.Fatalf("team identity/state = %+v, want state/identity preserved from %+v", got, want)
	}
	if got.Lead == nil || want.Lead == nil || *got.Lead != *want.Lead || len(got.Members) != len(want.Members) {
		t.Fatalf("lead/roster = %+v/%+v, want preserved %+v/%+v", got.Lead, got.Members, want.Lead, want.Members)
	}
	for i, member := range got.Members {
		if member.MemberKey != want.Members[i].MemberKey || member.Name != want.Members[i].Name || member.Role != want.Members[i].Role {
			t.Fatalf("member %d = %+v, want roster entry %+v", i, member, want.Members[i])
		}
		if member.SubagentRef != "" {
			t.Fatalf("member %d ref = %q, want cleared", i, member.SubagentRef)
		}
	}
}
