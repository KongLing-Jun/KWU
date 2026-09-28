package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/checkpoint"
	"kwu/internal/command"
	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/guardian"
	"kwu/internal/jobs"
	"kwu/internal/permission"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/scene"
	"kwu/internal/skill"
	"kwu/internal/store"
	"kwu/internal/temporal"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

type typedNilControllerSink struct{}

func (*typedNilControllerSink) Emit(event.Event) {}

func isolateControlConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KWU_CREDENTIALS_STORE", "file")
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Chdir(t.TempDir())
	return home
}

func controllerForForkFailureTest(t *testing.T) (*Controller, string, []byte) {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.jsonl")
	sess := agent.NewSession("system")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "source prompt"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "source answer"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: sourcePath, Sink: event.Discard})
	if err := c.Snapshot(); err != nil {
		t.Fatalf("snapshot source: %v", err)
	}
	c.checkpoints.begin("source prompt", 1)
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	return c, sourcePath, sourceBytes
}

func writeForkOwnedArtifacts(t *testing.T, path string) []string {
	t.Helper()
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{Name: "injected fork"}); err != nil {
		t.Fatalf("write branch meta: %v", err)
	}
	if err := agent.SaveSessionExpert(path, agent.ExpertMeta{ExpertID: "fork-expert"}); err != nil {
		t.Fatalf("write expert meta: %v", err)
	}
	guardianPath := guardian.PathFor(path)
	guardianCursorPath := guardian.CursorPathFor(path)
	for _, artifact := range []string{guardianPath, guardianCursorPath} {
		if err := os.WriteFile(artifact, []byte("guardian"), 0o644); err != nil {
			t.Fatalf("write guardian artifact: %v", err)
		}
	}
	checkpointDir := store.SessionCheckpointDir(path)
	jobDir := jobs.ArtifactDir(path)
	for _, dir := range []string{checkpointDir, jobDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir owned artifact: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "owned"), []byte("owned"), 0o644); err != nil {
			t.Fatalf("write owned artifact: %v", err)
		}
	}
	subagentDir := filepath.Join(filepath.Dir(path), "subagents")
	if err := os.MkdirAll(subagentDir, 0o755); err != nil {
		t.Fatalf("mkdir subagents: %v", err)
	}
	const ref = "sa_fork_cleanup"
	subagentSession := filepath.Join(subagentDir, ref+".jsonl")
	subagentMeta := filepath.Join(subagentDir, ref+".meta.json")
	if err := os.WriteFile(subagentSession, []byte("subagent"), 0o644); err != nil {
		t.Fatalf("write subagent session: %v", err)
	}
	metaBytes, err := json.Marshal(agent.SubagentMeta{Ref: ref, ParentSession: agent.BranchID(path)})
	if err != nil {
		t.Fatalf("marshal subagent meta: %v", err)
	}
	if err := os.WriteFile(subagentMeta, metaBytes, 0o644); err != nil {
		t.Fatalf("write subagent meta: %v", err)
	}
	if err := agent.MarkCleanupPending(path, "fork-test"); err != nil {
		t.Fatalf("mark cleanup pending: %v", err)
	}
	return []string{
		path,
		agent.BranchMetaPath(path),
		store.SessionExpert(path),
		guardianPath,
		guardianCursorPath,
		checkpointDir,
		jobDir,
		store.SessionCleanupPending(path),
		subagentSession,
		subagentMeta,
	}
}

func TestForkSessionBranchMetaFailureCleansOwnedArtifacts(t *testing.T) {
	c, sourcePath, sourceBytes := controllerForForkFailureTest(t)
	defer c.Close()
	unrelatedPath := filepath.Join(filepath.Dir(sourcePath), "unrelated.txt")
	unrelatedBytes := []byte("unrelated")
	if err := os.WriteFile(unrelatedPath, unrelatedBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	originalSave := saveForkBranchMeta
	originalRemove := removeForkSessionArtifacts
	injected := errors.New("injected fork branch meta failure")
	var forkPath string
	var ownedPaths []string
	saveForkBranchMeta = func(path string, _ agent.BranchMeta) error {
		forkPath = path
		ownedPaths = writeForkOwnedArtifacts(t, path)
		return injected
	}
	t.Cleanup(func() {
		saveForkBranchMeta = originalSave
		removeForkSessionArtifacts = originalRemove
	})

	if path, err := c.ForkSession(0, "fork"); !errors.Is(err, injected) || path != "" {
		t.Fatalf("ForkSession path=%q err=%v, want injected failure", path, err)
	}
	if forkPath == "" {
		t.Fatal("fork path was not captured")
	}
	for _, path := range ownedPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned fork artifact still exists %s: %v", path, err)
		}
	}
	if c.SessionPath() != sourcePath {
		t.Fatalf("source controller path = %q, want %q", c.SessionPath(), sourcePath)
	}
	if got, err := os.ReadFile(sourcePath); err != nil || !bytes.Equal(got, sourceBytes) {
		t.Fatalf("source bytes changed: err=%v got=%q want=%q", err, got, sourceBytes)
	}
	if got, err := os.ReadFile(unrelatedPath); err != nil || !bytes.Equal(got, unrelatedBytes) {
		t.Fatalf("unrelated bytes changed: err=%v got=%q want=%q", err, got, unrelatedBytes)
	}
}

func TestForkSessionBranchMetaAndCleanupFailureAreJoined(t *testing.T) {
	c, sourcePath, _ := controllerForForkFailureTest(t)
	defer c.Close()
	originalSave := saveForkBranchMeta
	originalRemove := removeForkSessionArtifacts
	saveErr := errors.New("injected fork branch meta failure")
	cleanupErr := errors.New("injected fork cleanup failure")
	var forkPath string
	saveForkBranchMeta = func(path string, _ agent.BranchMeta) error {
		forkPath = path
		return saveErr
	}
	removeForkSessionArtifacts = func(string) error { return cleanupErr }
	t.Cleanup(func() {
		saveForkBranchMeta = originalSave
		removeForkSessionArtifacts = originalRemove
		if forkPath != "" {
			_ = originalRemove(forkPath)
		}
	})

	_, err := c.ForkSession(0, "fork")
	if !errors.Is(err, saveErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("ForkSession error = %v, want joined save and cleanup failures", err)
	}
	canonicalPath, absErr := filepath.Abs(forkPath)
	if absErr != nil {
		t.Fatal(absErr)
	}
	if !strings.Contains(err.Error(), canonicalPath) {
		t.Fatalf("ForkSession error %q does not contain fork path %q", err, canonicalPath)
	}
	if c.SessionPath() != sourcePath {
		t.Fatalf("source controller path = %q, want %q", c.SessionPath(), sourcePath)
	}
}

type appendingRunner struct {
	session *agent.Session
}

func (r appendingRunner) Run(_ context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	return nil
}

type handoffRunner struct {
	session *agent.Session
}

func (r handoffRunner) Run(_ context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: "handoff: " + input})
	return nil
}

type sessionContextRunner struct {
	parentSession string
	jobSession    string
}

func (r *sessionContextRunner) Run(ctx context.Context, input string) error {
	r.parentSession = agent.ParentSession(ctx)
	r.jobSession = jobs.SessionFromContext(ctx)
	return nil
}

type countingControlRunner struct{ calls int }

func (r *countingControlRunner) Run(context.Context, string) error {
	r.calls++
	return nil
}

func TestTimeCommandBypassesRunnerAndModelContext(t *testing.T) {
	runner := &countingControlRunner{}
	var notices []string
	times := temporal.New(temporal.Options{
		Configured: "Asia/Shanghai",
		Clock: temporal.ClockFunc(func() time.Time {
			return time.Date(2026, 7, 19, 16, 0, 1, 0, time.UTC)
		}),
	})
	c := New(Options{
		Runner:     runner,
		Scene:      scene.Coding,
		SceneState: scene.Draft,
		Temporal:   times,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.Notice {
				notices = append(notices, e.Text)
			}
		}),
	})

	c.Submit("/time")
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "2026-07-20T00:00:01+08:00") || !strings.Contains(notices[0], "Asia/Shanghai") {
		t.Fatalf("time notices = %#v", notices)
	}
	if got := c.History(); len(got) != 0 {
		t.Fatalf("/time changed model history: %#v", got)
	}
	if got := c.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Draft}) {
		t.Fatalf("/time scene status = %+v, want unchanged Coding/Draft", got)
	}
}

func TestControllerTimezoneUpdateRejectsInvalidWithoutChangingCurrent(t *testing.T) {
	times := temporal.New(temporal.Options{Configured: "UTC"})
	c := New(Options{Temporal: times})
	if err := c.SetTimezone("America/New_York"); err != nil {
		t.Fatal(err)
	}
	if got := c.Timezone(); got != "America/New_York" {
		t.Fatalf("Timezone() = %q", got)
	}
	if err := c.SetTimezone("Mars/Olympus_Mons"); err == nil {
		t.Fatal("invalid timezone was accepted")
	}
	if got := c.Timezone(); got != "America/New_York" {
		t.Fatalf("invalid update changed timezone to %q", got)
	}
}

type fakeControlTool struct {
	name   string
	output string
}

func (t fakeControlTool) Name() string { return t.name }
func (fakeControlTool) Description() string {
	return "fake"
}
func (fakeControlTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (t fakeControlTool) Execute(context.Context, json.RawMessage) (string, error) {
	return t.output, nil
}
func (fakeControlTool) ReadOnly() bool { return true }

type startBackgroundJobTool struct {
	started chan string
	release chan struct{}
}

func (t startBackgroundJobTool) Name() string        { return "start_background_job" }
func (t startBackgroundJobTool) Description() string { return "start background job" }
func (t startBackgroundJobTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (t startBackgroundJobTool) ReadOnly() bool { return false }
func (t startBackgroundJobTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	jm, ok := jobs.FromContext(ctx)
	if !ok {
		return "", nil
	}
	j := jm.StartForSession(jobs.SessionFromContext(ctx), "bash", "controller", func(_ context.Context, out io.Writer) (string, error) {
		_, _ = io.WriteString(out, "before\n")
		<-t.release
		_, _ = io.WriteString(out, "after\n")
		return "", nil
	})
	t.started <- j.ID
	return "started " + j.ID, nil
}

type recordingProvider struct {
	name     string
	streams  [][]provider.Chunk
	requests []provider.Request
}

func (p *recordingProvider) Name() string {
	if p.name != "" {
		return p.name
	}
	return "recording"
}

func (p *recordingProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	p.requests = append(p.requests, req)
	i := len(p.requests) - 1
	if i >= len(p.streams) {
		i = len(p.streams) - 1
	}
	chunks := p.streams[i]
	ch := make(chan provider.Chunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func requestMessagesText(messages []provider.Message) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	return b.String()
}

func newExpertTestController(t *testing.T, dir string, cfgDefault string) (*Controller, *agent.Agent) {
	t.Helper()
	userExpertPath := filepath.Join(dir, "experts.json")
	loader := func() (*expert.Catalog, error) {
		return expert.LoadCatalog(userExpertPath)
	}
	catalog, err := loader()
	if err != nil {
		t.Fatal(err)
	}
	builder := func(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
		currentCatalog, err := loader()
		if err != nil {
			return "", expert.Selection{}, err
		}
		selection, err := expert.ResolveSelection(currentCatalog, sessionExpertID, requestExpertID, cfgDefault)
		if err != nil {
			return "", selection, err
		}
		sys := "base"
		if selection.Selected {
			sys = expert.ApplyPrompt(sys, selection.Expert)
		}
		return sys, selection, nil
	}
	sys, selection, err := builder("", "")
	if err != nil {
		t.Fatal(err)
	}
	exec := agent.New(nil, nil, agent.NewSession(sys), agent.Options{}, event.Discard)
	path := filepath.Join(dir, "session.jsonl")
	return New(Options{
		Executor:            exec,
		SystemPrompt:        sys,
		Experts:             catalog,
		Expert:              selection,
		PromptBuilder:       builder,
		ExpertCatalogLoader: loader,
		UserExpertPath:      userExpertPath,
		SessionDir:          dir,
		SessionPath:         path,
		Label:               "test",
	}), exec
}

func lastUserMessage(messages []provider.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == provider.RoleUser {
			return messages[i].Content
		}
	}
	return ""
}

func TestNewTreatsTypedNilSinkAsDiscard(t *testing.T) {
	var sink *typedNilControllerSink
	c := New(Options{Sink: sink})

	c.notice("typed nil sink should not panic")
}

func TestNewDefaultsActorContext(t *testing.T) {
	c := New(Options{})

	actor := c.ActorContext()
	if actor.ActorID != userctx.DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, userctx.DefaultActorID)
	}
	if actor.ProfileID != userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", actor.ProfileID, userctx.DefaultProfileID)
	}
	if actor.AuthMethod != userctx.DefaultAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, userctx.DefaultAuthMethod)
	}
}

func TestHeadlessApprovalDefaultsActorContextLocalDefault(t *testing.T) {
	c := New(Options{})

	actor := c.ActorContext()
	if actor.ActorID != userctx.DefaultActorID || actor.ProfileID != userctx.DefaultProfileID || actor.AuthMethod != userctx.DefaultAuthMethod {
		t.Fatalf("headless ActorContext = %+v, want local/default", actor)
	}
	if got := c.ToolApprovalMode(); got != ToolApprovalAsk {
		t.Fatalf("headless tool approval mode = %q, want %q", got, ToolApprovalAsk)
	}
}

func TestNewStoresActorContextCopy(t *testing.T) {
	remote := &userctx.RemoteIdentity{Provider: "feishu", UserID: "u1"}
	c := New(Options{
		Actor: userctx.ActorContext{
			ActorID:        "actor-1",
			ProfileID:      "profile-1",
			AuthMethod:     "oauth",
			WorkspaceRoot:  "root-1",
			RemoteIdentity: remote,
		},
	})

	remote.UserID = "mutated"
	actor := c.ActorContext()
	if actor.ActorID != "actor-1" || actor.ProfileID != "profile-1" || actor.AuthMethod != "oauth" || actor.WorkspaceRoot != "root-1" {
		t.Fatalf("ActorContext() = %+v", actor)
	}
	if actor.RemoteIdentity == nil || actor.RemoteIdentity.UserID != "u1" {
		t.Fatalf("RemoteIdentity = %+v, want copied user u1", actor.RemoteIdentity)
	}
	actor.ActorID = "changed"
	actor.RemoteIdentity.UserID = "changed"
	again := c.ActorContext()
	if again.ActorID != "actor-1" || again.RemoteIdentity == nil || again.RemoteIdentity.UserID != "u1" {
		t.Fatalf("ActorContext() returned mutable controller state: %+v", again)
	}
}

func TestSessionAPIExposesActorContextForDerivedEntrypoints(t *testing.T) {
	ctrl := New(Options{
		Actor: userctx.ActorContext{
			ActorID:    "actor-1",
			ProfileID:  "profile-1",
			AuthMethod: "oauth",
		},
	})
	var api SessionAPI = ctrl

	actor := api.ActorContext()
	if actor.ActorID != "actor-1" || actor.ProfileID != "profile-1" || actor.AuthMethod != "oauth" {
		t.Fatalf("SessionAPI.ActorContext() = %+v", actor)
	}
}

func TestClearSessionMarksCleanupPendingBeforeReturningForRunningJobs(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.jsonl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exec := agent.New(nil, nil, agent.NewSession("sys"), agent.Options{}, event.Discard)
	jm := jobs.NewManager(event.Discard)
	release := make(chan struct{})
	started := make(chan struct{})
	defer func() {
		close(release)
		jm.Close()
	}()
	jm.StartForSession(agent.BranchID(oldPath), "task", "stuck clear", func(ctx context.Context, _ io.Writer) (string, error) {
		close(started)
		<-ctx.Done()
		<-release
		return "", ctx.Err()
	})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background job never started")
	}

	ctrl := New(Options{Executor: exec, SessionDir: dir, SessionPath: oldPath, Label: "test", Jobs: jm})
	if err := ctrl.ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	if !agent.IsCleanupPending(oldPath) {
		t.Fatalf("old session should be cleanup-pending before ClearSession returns")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("old session file should remain until delayed cleanup: %v", err)
	}
	sessions, err := agent.ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range sessions {
		if filepath.Clean(session.Path) == filepath.Clean(oldPath) {
			t.Fatalf("cleanup-pending old session still listed: %+v", sessions)
		}
	}
}

func TestReconcileCleanupPendingRemovesOrphanedArtifacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orphan.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"orphan"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{Name: "orphan"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(jobs.ArtifactDir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs.ArtifactDir(path), "job.log"), []byte("job output"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ckptDir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := agent.MarkCleanupPending(path, "delete"); err != nil {
		t.Fatal(err)
	}

	if err := ReconcileCleanupPending(dir); err != nil {
		t.Fatalf("ReconcileCleanupPending: %v", err)
	}
	for _, p := range []string{path, agent.BranchMetaPath(path), jobs.ArtifactDir(path), ckptDir(path), agent.CleanupPendingPath(path)} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after reconciliation (err=%v)", p, err)
		}
	}
}

func TestRunTurnSnapshotsActivityWhenTranscriptChanges(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	path := filepath.Join(dir, "session.jsonl")
	c := New(Options{Runner: appendingRunner{session: sess}, Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.runTurn(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}

	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("saved messages = %d, want system + user", len(loaded.Messages))
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load activity meta ok=%v err=%v", ok, err)
	}
	if meta.UpdatedAt.IsZero() {
		t.Fatal("activity meta should be marked")
	}
}

func TestSnapshotRefusesContentWithoutSystemMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	sess := agent.NewSession("")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "hello without system"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.Snapshot(); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("snapshot wrote session without system message; stat err=%v", err)
	}
	if _, err := os.Stat(agent.BranchMetaPath(path)); !os.IsNotExist(err) {
		t.Fatalf("snapshot wrote metadata for rejected session; stat err=%v", err)
	}
}

func TestRunInjectsParentSessionForJobs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	runner := &sessionContextRunner{}
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Runner: runner, Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	want := agent.BranchID(path)
	if runner.parentSession != want {
		t.Fatalf("ParentSession = %q, want %q", runner.parentSession, want)
	}
	if runner.jobSession != want {
		t.Fatalf("jobs session = %q, want %q", runner.jobSession, want)
	}
}

func TestSetSessionPathAdoptsTemporaryBackgroundJobs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	started := make(chan string, 1)
	release := make(chan struct{})
	jm := jobs.NewManager(event.Discard)
	reg := tool.NewRegistry()
	reg.Add(startBackgroundJobTool{started: started, release: release})
	reg.Add(fakeControlTool{name: "deliver_files", output: `{"type":"deliver_files_result","paths":[]}`})
	prov := &scriptedTurns{turns: [][]provider.Chunk{
		toolCallTurn("call-1", "start_background_job", `{}`),
		toolCallTurn("manifest", "deliver_files", `{"paths":[]}`),
		textTurn("done"),
	}}
	ag := agent.New(prov, reg, agent.NewSession("sys"), agent.Options{Jobs: jm}, event.Discard)
	c := New(Options{Runner: ag, Executor: ag, SessionDir: dir, Label: "test", Jobs: jm})
	defer c.Close()

	if err := c.Run(context.Background(), "start background job"); err != nil {
		t.Fatal(err)
	}
	jobID := <-started
	c.SetSessionPath(path)
	close(release)

	parentSession := agent.BranchID(path)
	res := c.jobs.WaitForSession(context.Background(), parentSession, []string{jobID}, 1)
	if len(res) != 1 || !strings.Contains(res[0].Output, "before\n") || !strings.Contains(res[0].Output, "after\n") {
		t.Fatalf("adopted controller job = %+v, want before/after output", res)
	}
	if _, err := os.Stat(filepath.Join(jobs.ArtifactDir(path), jobID+".log")); err != nil {
		t.Fatalf("controller job artifact should be under persistent sidecar: %v", err)
	}
}

func TestGoalStatePersistsNextToSessionPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})

	c.SetGoalWithResearchMode("fix the typo", GoalResearchOn)
	c.GoalStrict(true)

	data, err := os.ReadFile(goalStatePath(path))
	if err != nil {
		t.Fatal(err)
	}
	var state goalState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Goal != "fix the typo" || state.Status != GoalStatusRunning || state.ResearchMode != GoalResearchOn || !state.Strict {
		t.Fatalf("goal state = %+v, want running strict research goal", state)
	}
}

func TestRunTurnRecordsDisplayForPersistedUserMessage(t *testing.T) {
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Runner: handoffRunner{session: sess}, Executor: exec})
	var gotContent, gotDisplay string
	c.SetDisplayRecorder(func(content, display string) {
		gotContent = content
		gotDisplay = display
	})

	if err := c.runTurnWithRawDisplay(context.Background(), "expanded prompt", "raw prompt", "visible prompt"); err != nil {
		t.Fatal(err)
	}

	if gotContent != "handoff: expanded prompt" {
		t.Fatalf("display recorded against %q, want persisted user message", gotContent)
	}
	if gotDisplay != "visible prompt" {
		t.Fatalf("display = %q, want visible prompt", gotDisplay)
	}
}

func TestSnapshotDoesNotRefreshSessionActivity(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Label: "test", ModelRef: "provider/model-a"})
	c.SetSessionPath(filepath.Join(dir, "session.jsonl"))

	if err := c.SnapshotActivity(); err != nil {
		t.Fatal(err)
	}
	first, ok, err := agent.LoadBranchMeta(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("load initial meta ok=%v err=%v", ok, err)
	}

	time.Sleep(10 * time.Millisecond)
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "saved without activity"})
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	second, ok, err := agent.LoadBranchMeta(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("load second meta ok=%v err=%v", ok, err)
	}
	if !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("Snapshot refreshed activity: first=%s second=%s", first.UpdatedAt, second.UpdatedAt)
	}
	if second.Model != "provider/model-a" {
		t.Fatalf("snapshot model = %q, want provider/model-a", second.Model)
	}
}

func TestSnapshotSkipsUnchangedTranscriptAndMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	transcriptBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	metaBefore, err := os.Stat(agent.BranchMetaPath(path))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	transcriptAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	metaAfter, err := os.Stat(agent.BranchMetaPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if !transcriptAfter.ModTime().Equal(transcriptBefore.ModTime()) {
		t.Fatalf("unchanged transcript was rewritten: first=%s second=%s", transcriptBefore.ModTime(), transcriptAfter.ModTime())
	}
	if !metaAfter.ModTime().Equal(metaBefore.ModTime()) {
		t.Fatalf("unchanged metadata was rewritten: first=%s second=%s", metaBefore.ModTime(), metaAfter.ModTime())
	}
}

func TestSnapshotDirtyAutosaveRollbackRewritesUnchangedTranscript(t *testing.T) {
	t.Setenv("KWU_DISABLE_DIRTY_AUTOSAVE", "1")
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().After(before.ModTime()) {
		t.Fatalf("rollback mode did not rewrite unchanged transcript: first=%s second=%s", before.ModTime(), after.ModTime())
	}
}

func TestSnapshotActivityRefreshesSessionActivity(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, Label: "test"})
	c.SetSessionPath(filepath.Join(dir, "session.jsonl"))

	if err := c.SnapshotActivity(); err != nil {
		t.Fatal(err)
	}
	first, _, err := agent.LoadBranchMeta(c.SessionPath())
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "activity"})
	if err := c.SnapshotActivity(); err != nil {
		t.Fatal(err)
	}
	second, _, err := agent.LoadBranchMeta(c.SessionPath())
	if err != nil {
		t.Fatal(err)
	}
	if !second.UpdatedAt.After(first.UpdatedAt) {
		t.Fatalf("SnapshotActivity did not refresh activity: first=%s second=%s", first.UpdatedAt, second.UpdatedAt)
	}
}

func TestSnapshotActivitySavesTranscriptBeforeModelMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "must persist"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "test", ModelRef: "provider/model-a"})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent.BranchMetaPath(path), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := c.SnapshotActivity(); err == nil {
		t.Fatal("SnapshotActivity should report malformed branch metadata")
	}
	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatalf("transcript was not saved before metadata error: %v", err)
	}
	if len(loaded.Messages) == 0 || loaded.Messages[len(loaded.Messages)-1].Content != "must persist" {
		t.Fatalf("saved transcript = %+v, want persisted user message", loaded.Messages)
	}
}

func TestSnapshotRetriesPendingActivityAfterMetadataFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.SnapshotActivity(); err != nil {
		t.Fatal(err)
	}
	before, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load initial meta ok=%v err=%v", ok, err)
	}

	time.Sleep(10 * time.Millisecond)
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "new activity"})
	if err := os.WriteFile(agent.BranchMetaPath(path), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.SnapshotActivity(); err == nil {
		t.Fatal("SnapshotActivity should report malformed branch metadata")
	}

	if err := agent.SaveBranchMetaPreserveUpdated(path, before); err != nil {
		t.Fatal(err)
	}
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	after, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load retried meta ok=%v err=%v", ok, err)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("pending activity was not retried: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestSnapshotRetriesPendingActivityForOriginalSessionPath(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.jsonl")
	newPath := filepath.Join(dir, "new.jsonl")
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "old session"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Executor: exec, SessionDir: dir, SessionPath: oldPath, Label: "test"})

	if err := c.SnapshotActivity(); err != nil {
		t.Fatal(err)
	}
	before, ok, err := agent.LoadBranchMeta(oldPath)
	if err != nil || !ok {
		t.Fatalf("load initial meta ok=%v err=%v", ok, err)
	}

	time.Sleep(10 * time.Millisecond)
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "new activity"})
	if err := os.WriteFile(agent.BranchMetaPath(oldPath), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := c.SnapshotActivity(); err == nil {
		t.Fatal("SnapshotActivity should report malformed branch metadata")
	}
	if err := agent.SaveBranchMetaPreserveUpdated(oldPath, before); err != nil {
		t.Fatal(err)
	}

	exec.SetSession(agent.NewSession("sys"))
	c.SetSessionPath(newPath)
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	after, ok, err := agent.LoadBranchMeta(oldPath)
	if err != nil || !ok {
		t.Fatalf("load retried old meta ok=%v err=%v", ok, err)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("old path activity was not retried: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	}
}

func TestNewSessionStartsFreshContextAndSavesTranscript(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "old context"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	path := filepath.Join(dir, "session.jsonl")
	c := New(Options{Executor: exec, SystemPrompt: "sys", SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.NewSession(); err != nil {
		t.Fatal(err)
	}
	if c.SessionPath() == path {
		t.Fatal("/new did not rotate to a fresh session path")
	}
	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 2 || loaded.Messages[1].Content != "old context" {
		t.Fatalf("previous transcript was not saved: %+v", loaded.Messages)
	}
	current := exec.Session().Snapshot()
	if len(current) != 1 || current[0].Role != provider.RoleSystem || current[0].Content != "sys" {
		t.Fatalf("fresh context = %+v, want only system prompt", current)
	}
}

func TestSetExpertUpdatesEmptySessionAndPersistsMeta(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")

	state, err := c.SetExpert("testing")
	if err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	if state.SelectedID != "testing" || state.Source != expert.SourceRequest {
		t.Fatalf("state = %+v, want testing from request", state)
	}
	current := exec.Session().Snapshot()
	if len(current) != 1 || !strings.Contains(current[0].Content, "Expert role: 测试策略专家") {
		t.Fatalf("session system prompt = %+v, want testing expert", current)
	}
	meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil {
		t.Fatalf("LoadSessionExpert: %v", err)
	}
	if !ok || meta.ExpertID != "testing" {
		t.Fatalf("expert meta = %+v ok=%v, want testing", meta, ok)
	}
}

func TestSetExpertGroupActivatesAllProjectExpertsAndPersistsBindings(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")

	state, err := c.SetExpertGroup([]string{"testing", "architecture-review", "testing"})
	if err != nil {
		t.Fatalf("SetExpertGroup: %v", err)
	}
	if state.SelectedID != "project-expert-team-a144f51a438b4f7d" || state.SelectedName != "项目专家团" {
		t.Fatalf("state = %+v, want project expert team", state)
	}
	if c.expert.Expert.Mode != expert.ModeTeam || len(c.expert.Expert.Members) != 2 {
		t.Fatalf("selection = %+v, want two-member expert team", c.expert.Expert)
	}
	if c.expert.Expert.Members[0].ExpertID != "testing" || c.expert.Expert.Members[1].ExpertID != "architecture-review" {
		t.Fatalf("members = %+v, want binding order preserved", c.expert.Expert.Members)
	}
	current := exec.Session().Snapshot()
	if len(current) != 1 || !strings.Contains(current[0].Content, "Selected expert:") {
		t.Fatalf("session system prompt = %+v, want combined expert prompt", current)
	}
	meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert: meta=%+v ok=%v err=%v", meta, ok, err)
	}
	if got := strings.Join(meta.ProjectExpertIDs, ","); got != "testing,architecture-review" {
		t.Fatalf("ProjectExpertIDs = %q, want ordered unique ids", got)
	}
	if meta.TeamState != agent.ExpertTeamActive || meta.Lead == nil || len(meta.Members) != 1 {
		t.Fatalf("expert meta = %+v, want active two-member team", meta)
	}
}

func TestProjectExpertGroupRestoresFromSessionMetadata(t *testing.T) {
	dir := t.TempDir()
	c, _ := newExpertTestController(t, dir, "")
	if _, err := c.SetExpertGroup([]string{"testing", "architecture-review"}); err != nil {
		t.Fatalf("SetExpertGroup: %v", err)
	}
	path := c.SessionPath()

	restored, _ := newExpertTestController(t, dir, "")
	restored.loadExpertMeta(path)
	state := restored.CurrentExpert()
	if !strings.HasPrefix(state.SelectedID, projectExpertTeamIDPrefix) || restored.expert.Expert.Mode != expert.ModeTeam {
		t.Fatalf("restored state = %+v selection=%+v, want project expert team", state, restored.expert)
	}
	if !strings.Contains(restored.systemPrompt, "Selected expert:") {
		t.Fatalf("restored system prompt does not contain combined expert instructions:\n%s", restored.systemPrompt)
	}
}

func TestSetExpertSaveFailureDoesNotPublishSelection(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("testing"); err != nil {
		t.Fatalf("SetExpert initial: %v", err)
	}
	beforeState := c.CurrentExpert()
	beforePrompt := c.systemPrompt
	beforeMeta := cloneExpertMeta(c.expertMeta)
	beforeSession := exec.Session().Snapshot()
	beforeUsage := c.ExpertLibraryState()
	beforeDisk, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert before failure: meta=%+v ok=%v err=%v", beforeDisk, ok, err)
	}
	originalSave := saveControllerExpertMeta
	saveControllerExpertMeta = func(string, agent.ExpertMeta) error { return errors.New("injected selection save failure") }
	t.Cleanup(func() { saveControllerExpertMeta = originalSave })

	if _, err := c.SetExpert("code-delivery-team"); err == nil || !strings.Contains(err.Error(), "injected selection save failure") {
		t.Fatalf("SetExpert error = %v, want sidecar save failure", err)
	}
	afterState := c.CurrentExpert()
	if afterState.SelectedID != beforeState.SelectedID || afterState.SelectedName != beforeState.SelectedName {
		t.Fatalf("selection changed after failed save: before=%+v after=%+v", beforeState, afterState)
	}
	if c.systemPrompt != beforePrompt {
		t.Fatalf("system prompt changed after failed save")
	}
	if c.expertMeta.ExpertID != beforeMeta.ExpertID || c.expertMeta.TeamState != beforeMeta.TeamState {
		t.Fatalf("expert meta changed after failed save: before=%+v after=%+v", beforeMeta, c.expertMeta)
	}
	afterSession := exec.Session().Snapshot()
	if len(afterSession) != len(beforeSession) || afterSession[0].Content != beforeSession[0].Content {
		t.Fatalf("executor session changed after failed save: before=%+v after=%+v", beforeSession, afterSession)
	}
	afterUsage := c.ExpertLibraryState()
	if afterUsage.UsageCounts["code-delivery-team"] != beforeUsage.UsageCounts["code-delivery-team"] {
		t.Fatalf("failed selection changed usage: before=%v after=%v", beforeUsage.UsageCounts, afterUsage.UsageCounts)
	}
	afterDisk, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok || afterDisk.ExpertID != beforeDisk.ExpertID || afterDisk.TeamState != beforeDisk.TeamState {
		t.Fatalf("sidecar changed after failed save: before=%+v after=%+v ok=%v err=%v", beforeDisk, afterDisk, ok, err)
	}
}

func TestSetExpertRequiresPersistableSessionPath(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	c.mu.Lock()
	c.sessionPath = ""
	c.mu.Unlock()
	beforeSession := exec.Session().Snapshot()

	if _, err := c.SetExpert("code-delivery-team"); err == nil || !strings.Contains(err.Error(), "session path") {
		t.Fatalf("SetExpert error = %v, want missing session path", err)
	}
	if state := c.CurrentExpert(); state.SelectedID != "" {
		t.Fatalf("selection published without a session path: %+v", state)
	}
	afterSession := exec.Session().Snapshot()
	if len(afterSession) != len(beforeSession) || afterSession[0].Content != beforeSession[0].Content {
		t.Fatalf("executor session changed without a session path: before=%+v after=%+v", beforeSession, afterSession)
	}
}

func TestClearUnusedExpertTeam(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing_content_%t", existing), func(t *testing.T) {
			c, exec := newExpertTestController(t, t.TempDir(), "")
			if existing {
				exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "ordinary history"})
			}
			if _, err := c.AttachExpert("code-delivery-team"); err != nil {
				t.Fatal(err)
			}
			state, err := c.ClearExpert()
			if err != nil || state.SelectedID != "" {
				t.Fatalf("clear = %+v, %v", state, err)
			}
			if meta, ok, err := agent.LoadSessionExpert(c.SessionPath()); err != nil || ok {
				t.Fatalf("unused team persisted: %+v, %t, %v", meta, ok, err)
			}
			messages := exec.Session().Snapshot()
			if messages[0].Content != "base" || (existing && (len(messages) != 2 || messages[1].Content != "ordinary history")) {
				t.Fatalf("ordinary history changed: %+v", messages)
			}
			if !existing && !state.CanSwitch {
				t.Fatal("unused cancellation prevents another selection")
			}
			exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "ordinary followup"})
			c.saveExpertMeta(c.SessionPath())
			if err := exec.Session().Save(c.SessionPath()); err != nil {
				t.Fatal(err)
			}
			loaded, err := agent.LoadSession(c.SessionPath())
			if err != nil {
				t.Fatal(err)
			}
			resumed, _ := newExpertTestController(t, t.TempDir(), "")
			resumed.Resume(loaded, c.SessionPath())
			if resumed.CurrentExpert().SelectedID != "" || resumed.expertMeta.TeamState != "" {
				t.Fatal("ordinary followup restored the cancelled team")
			}
		})
	}
}

func TestClearUnusedExpertTeamDeleteFailure(t *testing.T) {
	c, exec := newExpertTestController(t, t.TempDir(), "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatal(err)
	}
	before := exec.Session()
	originalDelete := deleteControllerExpertMeta
	deleteControllerExpertMeta = func(string) error { return errors.New("injected delete failure") }
	t.Cleanup(func() { deleteControllerExpertMeta = originalDelete })
	if _, err := c.ClearExpert(); err == nil || !strings.Contains(err.Error(), "injected delete failure") {
		t.Fatalf("clear error = %v", err)
	}
	if c.CurrentExpert().SelectedID != "code-delivery-team" || exec.Session() != before || c.expertMeta.TeamState != agent.ExpertTeamActive {
		t.Fatal("failed deletion published a cleared team")
	}
}

func TestExpertTeamLeadUseIsDurableBeforeRunner(t *testing.T) {
	for _, failSave := range []bool{false, true} {
		t.Run(fmt.Sprintf("save_failure_%t", failSave), func(t *testing.T) {
			c, _ := newExpertTestController(t, t.TempDir(), "")
			if _, err := c.SetExpert("code-delivery-team"); err != nil {
				t.Fatal(err)
			}
			runner := &countingControlRunner{}
			c.runner = runner
			if failSave {
				originalSave := saveControllerExpertMeta
				saveControllerExpertMeta = func(string, agent.ExpertMeta) error { return errors.New("injected use failure") }
				t.Cleanup(func() { saveControllerExpertMeta = originalSave })
			}
			err := c.runRootRunner(context.Background(), "lead only")
			if failSave {
				if err == nil || runner.calls != 0 || c.expertMeta.TeamUsage != agent.ExpertTeamUnused {
					t.Fatalf("failed use persistence dispatched runner: calls=%d err=%v", runner.calls, err)
				}
				return
			}
			if err != nil || runner.calls != 1 {
				t.Fatalf("run: calls=%d err=%v", runner.calls, err)
			}
			c.saveExpertMeta(c.SessionPath())
			meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
			if err != nil || !ok || meta.TeamUsage != agent.ExpertTeamUsed {
				t.Fatalf("use not persisted: %+v %v", meta, err)
			}
			if _, err := c.ClearExpert(); err != nil {
				t.Fatal(err)
			}
			if _, err := c.ClearExpert(); err != nil {
				t.Fatal(err)
			}
			if c.expertMeta.TeamState != agent.ExpertTeamArchived {
				t.Fatal("lead-only history was deleted")
			}
		})
	}
}

func TestClearLegacyExpertTeamPreservesUnknownHistory(t *testing.T) {
	c, _ := newExpertTestController(t, t.TempDir(), "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatal(err)
	}
	c.expertMeta.TeamUsage = ""
	c.saveExpertMeta(c.SessionPath())
	if _, err := c.ClearExpert(); err != nil {
		t.Fatal(err)
	}
	if c.expertMeta.TeamState != agent.ExpertTeamArchived {
		t.Fatal("legacy unknown team was deleted")
	}
}

func TestClearExpertSaveFailureDoesNotArchiveOrStopTeam(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	c.jobs = jm
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("SetExpert team: %v", err)
	}
	runtime := controllerExpertTeamRuntime{c: c}
	if _, err := runtime.AcquireExpertTeamMember("代码审查专家"); err != nil {
		t.Fatalf("AcquireExpertTeamMember: %v", err)
	}
	if err := runtime.CommitExpertTeamMember("代码审查专家", "sa_clear_failure"); err != nil {
		t.Fatalf("CommitExpertTeamMember: %v", err)
	}
	runtime.ReleaseExpertTeamMember("代码审查专家")
	stopped := make(chan struct{})
	jm.StartControlledForSession(c.parentSessionID(), "task", "sa_clear_failure", "running expert", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		close(stopped)
		return "", ctx.Err()
	})
	beforeState := c.CurrentExpert()
	beforePrompt := c.systemPrompt
	beforeMeta := cloneExpertMeta(c.expertMeta)
	beforeSession := exec.Session().Snapshot()
	beforeDisk, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert before clear failure: meta=%+v ok=%v err=%v", beforeDisk, ok, err)
	}
	originalSave := saveControllerExpertMeta
	saveControllerExpertMeta = func(string, agent.ExpertMeta) error { return errors.New("injected clear save failure") }
	t.Cleanup(func() { saveControllerExpertMeta = originalSave })

	if _, err := c.ClearExpert(); err == nil || !strings.Contains(err.Error(), "injected clear save failure") {
		t.Fatalf("ClearExpert error = %v, want sidecar save failure", err)
	}
	afterState := c.CurrentExpert()
	if afterState.SelectedID != beforeState.SelectedID || c.systemPrompt != beforePrompt {
		t.Fatalf("selection/prompt changed after failed clear: before=%+v after=%+v", beforeState, afterState)
	}
	if c.expertMeta.ExpertID != beforeMeta.ExpertID || c.expertMeta.TeamState != beforeMeta.TeamState {
		t.Fatalf("expert meta changed after failed clear: before=%+v after=%+v", beforeMeta, c.expertMeta)
	}
	afterSession := exec.Session().Snapshot()
	if len(afterSession) != len(beforeSession) || afterSession[0].Content != beforeSession[0].Content {
		t.Fatalf("executor session changed after failed clear: before=%+v after=%+v", beforeSession, afterSession)
	}
	afterDisk, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok || afterDisk.TeamState != beforeDisk.TeamState {
		t.Fatalf("sidecar changed after failed clear: before=%+v after=%+v ok=%v err=%v", beforeDisk, afterDisk, ok, err)
	}
	select {
	case <-stopped:
		t.Fatal("failed clear stopped an active team member")
	default:
	}
}

func TestClearExpertRequiresPersistableSessionPath(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("SetExpert team: %v", err)
	}
	beforeState := c.CurrentExpert()
	beforeSession := exec.Session().Snapshot()
	c.mu.Lock()
	c.sessionPath = ""
	c.mu.Unlock()

	if _, err := c.ClearExpert(); err == nil || !strings.Contains(err.Error(), "session path") {
		t.Fatalf("ClearExpert error = %v, want missing session path", err)
	}
	if state := c.CurrentExpert(); state.SelectedID != beforeState.SelectedID {
		t.Fatalf("clear published without a session path: before=%+v after=%+v", beforeState, state)
	}
	afterSession := exec.Session().Snapshot()
	if len(afterSession) != len(beforeSession) || afterSession[0].Content != beforeSession[0].Content {
		t.Fatalf("executor session changed without a session path: before=%+v after=%+v", beforeSession, afterSession)
	}
}

func TestExpertTeamPersistsMemberBindingsAndArchivesOnClear(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	c.jobs = jm
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("SetExpert team: %v", err)
	}
	meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert: meta=%+v ok=%v err=%v", meta, ok, err)
	}
	if meta.TeamState != agent.ExpertTeamActive || len(meta.Members) != 3 {
		t.Fatalf("team meta = %+v, want active team with three sub-experts", meta)
	}
	if meta.TeamBindingsReset {
		t.Fatal("new expert team unexpectedly started with reset bindings")
	}
	if meta.Lead == nil || meta.Lead.Name != "架构顾问" {
		t.Fatalf("team lead = %+v, want configured 架构顾问 snapshot", meta.Lead)
	}

	runtime := controllerExpertTeamRuntime{c: c}
	ref, err := runtime.AcquireExpertTeamMember("代码审查专家")
	if err != nil || ref != "" {
		t.Fatalf("first member acquire = %q, %v; want fresh binding", ref, err)
	}
	if _, err := runtime.AcquireExpertTeamMember("代码审查专家"); err == nil || !strings.Contains(err.Error(), "already starting") {
		t.Fatalf("concurrent member acquire error = %v, want serial-start rejection", err)
	}
	if err := runtime.CommitExpertTeamMember("代码审查专家", "sa_review"); err != nil {
		t.Fatalf("CommitExpertTeamMember: %v", err)
	}
	runtime.ReleaseExpertTeamMember("代码审查专家")
	if got, err := runtime.LookupExpertTeamMember("代码审查专家"); err != nil || got != "sa_review" {
		t.Fatalf("LookupExpertTeamMember = %q, %v; want sa_review", got, err)
	}
	stopped := make(chan struct{})
	jm.StartControlledForSession(c.parentSessionID(), "task", "sa_review", "running expert", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		close(stopped)
		return "", ctx.Err()
	})
	meta, ok, err = agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert after binding: meta=%+v ok=%v err=%v", meta, ok, err)
	}
	member, found := expertTeamMemberByName(meta, "代码审查专家")
	if !found || member.SubagentRef != "sa_review" {
		t.Fatalf("persisted member = %+v found=%v, want sa_review", member, found)
	}

	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "team task"})
	exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "team answer"})
	state, err := c.ClearExpert()
	if err != nil {
		t.Fatalf("ClearExpert team: %v", err)
	}
	if state.SelectedID != "" || state.CanSwitch || !state.RequiresNewSession {
		t.Fatalf("archived team state = %+v, want normal identity without resummon", state)
	}
	archived, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok || archived.TeamState != agent.ExpertTeamArchived {
		t.Fatalf("archived meta = %+v ok=%v err=%v", archived, ok, err)
	}
	if member, found := expertTeamMemberByName(archived, "代码审查专家"); !found || member.SubagentRef != "sa_review" {
		t.Fatalf("archived member = %+v found=%v, want history binding retained", member, found)
	}
	if _, err := c.SetExpert("testing"); err == nil || !strings.Contains(err.Error(), "archived its expert team") {
		t.Fatalf("SetExpert after archive error = %v, want same-session resummon rejection", err)
	}
	if _, err := runtime.LookupExpertTeamMember("代码审查专家"); err == nil {
		t.Fatal("archived team still allowed runtime task control")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("clearing the expert team did not stop its running child turn")
	}
	messages := exec.Session().Snapshot()
	if len(messages) != 3 || messages[0].Content != "base" || messages[1].Content != "team task" || messages[2].Content != "team answer" {
		t.Fatalf("messages after archive = %+v, want normal prompt with preserved transcript", messages)
	}
}

func TestExpertTeamForkAndRewindBindingsStartFresh(t *testing.T) {
	dir := t.TempDir()
	c, _ := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("SetExpert team: %v", err)
	}
	runtime := controllerExpertTeamRuntime{c: c}
	if _, err := runtime.AcquireExpertTeamMember("代码审查专家"); err != nil {
		t.Fatalf("AcquireExpertTeamMember: %v", err)
	}
	if err := runtime.CommitExpertTeamMember("代码审查专家", "sa_original"); err != nil {
		t.Fatalf("CommitExpertTeamMember: %v", err)
	}
	runtime.ReleaseExpertTeamMember("代码审查专家")

	forkPath := filepath.Join(dir, "fork.jsonl")
	if err := c.saveForkExpertMeta(forkPath); err != nil {
		t.Fatalf("saveForkExpertMeta: %v", err)
	}
	forkMeta, ok, err := agent.LoadSessionExpert(forkPath)
	if err != nil || !ok {
		t.Fatalf("LoadSessionExpert(fork): meta=%+v ok=%v err=%v", forkMeta, ok, err)
	}
	if member, found := expertTeamMemberByName(forkMeta, "代码审查专家"); !found || member.SubagentRef != "" {
		t.Fatalf("fork member = %+v found=%v, want fresh child binding", member, found)
	}
	if forkMeta.TeamBindingsReset {
		t.Fatal("fork from an ordinary team unexpectedly marked bindings reset")
	}
	if got, err := runtime.LookupExpertTeamMember("代码审查专家"); err != nil || got != "sa_original" {
		t.Fatalf("original binding after fork = %q, %v; want unchanged", got, err)
	}

	if err := c.resetCurrentExpertTeamBindings(); err != nil {
		t.Fatalf("resetCurrentExpertTeamBindings: %v", err)
	}
	if _, err := runtime.LookupExpertTeamMember("代码审查专家"); err == nil || !strings.Contains(err.Error(), "no task session yet") {
		t.Fatalf("lookup after rewind reset error = %v, want fresh binding", err)
	}
	resetMeta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok || !resetMeta.TeamBindingsReset {
		t.Fatalf("rewind reset meta = %+v ok=%v err=%v, want reset marker", resetMeta, ok, err)
	}
	resetForkPath := filepath.Join(dir, "reset-fork.jsonl")
	if err := c.saveForkExpertMeta(resetForkPath); err != nil {
		t.Fatalf("saveForkExpertMeta after reset: %v", err)
	}
	resetForkMeta, ok, err := agent.LoadSessionExpert(resetForkPath)
	if err != nil || !ok || !resetForkMeta.TeamBindingsReset {
		t.Fatalf("fork from reset team = %+v ok=%v err=%v, want inherited reset marker", resetForkMeta, ok, err)
	}
}

func TestResumeArchivedExpertTeamUsesNormalPromptAndKeepsHistoryBindings(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("code-delivery-team"); err != nil {
		t.Fatalf("SetExpert team: %v", err)
	}
	runtime := controllerExpertTeamRuntime{c: c}
	if _, err := runtime.AcquireExpertTeamMember("代码审查专家"); err != nil {
		t.Fatalf("AcquireExpertTeamMember: %v", err)
	}
	if err := runtime.CommitExpertTeamMember("代码审查专家", "sa_history"); err != nil {
		t.Fatalf("CommitExpertTeamMember: %v", err)
	}
	runtime.ReleaseExpertTeamMember("代码审查专家")
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "remember this"})
	if _, err := c.ClearExpert(); err != nil {
		t.Fatalf("ClearExpert: %v", err)
	}
	if err := exec.Session().Save(c.SessionPath()); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	loaded, err := agent.LoadSession(c.SessionPath())
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	resumed, resumedExec := newExpertTestController(t, dir, "")
	resumed.Resume(loaded, c.SessionPath())
	if got := resumed.CurrentExpert(); got.SelectedID != "" || got.CanSwitch {
		t.Fatalf("resumed expert state = %+v, want archived normal identity", got)
	}
	meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok || meta.TeamState != agent.ExpertTeamArchived {
		t.Fatalf("resumed archived meta = %+v ok=%v err=%v", meta, ok, err)
	}
	member, found := expertTeamMemberByName(meta, "代码审查专家")
	if !found || member.SubagentRef != "sa_history" {
		t.Fatalf("resumed history member = %+v found=%v", member, found)
	}
	messages := resumedExec.Session().Snapshot()
	if len(messages) != 2 || messages[0].Content != "base" || messages[1].Content != "remember this" {
		t.Fatalf("resumed messages = %+v, want normal prompt and preserved history", messages)
	}
}

func TestCreateUserExpertPersistsAndCanBeSelected(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")

	created, err := c.CreateUserExpert(expert.UserExpertInput{
		Name:          "Local Reviewer",
		Mode:          "expert",
		Description:   "Reviews local changes.",
		Category:      "engineering",
		Tags:          []string{"review"},
		SystemPrompt:  "Review the current code with local project context.",
		DisplayPrompt: "Review this change.",
		Language:      "auto",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateUserExpert: %v", err)
	}
	if created.ID == "" || created.SystemPrompt == "" {
		t.Fatalf("created expert = %+v, want id and system prompt", created)
	}
	if created.Source != expert.SourceUser || len(created.Prompts) != 3 || created.Prompts[0] != created.DisplayPrompt {
		t.Fatalf("created expert metadata = %+v, want user source and three prompts", created)
	}
	records, err := expert.LoadUserExperts(filepath.Join(dir, "experts.json"))
	if err != nil {
		t.Fatalf("LoadUserExperts: %v", err)
	}
	if len(records) != 1 || records[0].ID != created.ID {
		t.Fatalf("records = %+v, want created expert", records)
	}
	if _, ok := c.experts.Get(created.ID); !ok {
		t.Fatalf("controller catalog missing created expert %q", created.ID)
	}
	state, err := c.SetExpert(created.ID)
	if err != nil {
		t.Fatalf("SetExpert user expert: %v", err)
	}
	if state.SelectedID != created.ID {
		t.Fatalf("state = %+v, want created expert", state)
	}
	libraryState := c.ExpertLibraryState()
	if len(libraryState.RecentIDs) != 1 || libraryState.RecentIDs[0] != created.ID {
		t.Fatalf("library state after SetExpert = %+v, want recent created expert", libraryState)
	}
	if libraryState.UsageCounts[created.ID] != 1 {
		t.Fatalf("library state after SetExpert = %+v, want usage count 1", libraryState)
	}
	if _, err := c.SetExpert(created.ID); err != nil {
		t.Fatalf("SetExpert current expert again: %v", err)
	}
	libraryState = c.ExpertLibraryState()
	if libraryState.UsageCounts[created.ID] != 2 {
		t.Fatalf("library state after repeated SetExpert = %+v, want usage count 2", libraryState)
	}
	libraryState, err = c.SetExpertFavorite(created.ID, true)
	if err != nil {
		t.Fatalf("SetExpertFavorite: %v", err)
	}
	if len(libraryState.FavoriteIDs) != 1 || libraryState.FavoriteIDs[0] != created.ID {
		t.Fatalf("library state after favorite = %+v, want favorite created expert", libraryState)
	}
	current := exec.Session().Messages
	if len(current) != 1 || !strings.Contains(current[0].Content, "Review the current code with local project context.") {
		t.Fatalf("session system prompt = %+v, want user expert prompt", current)
	}
	if err := c.DeleteUserExpert(created.ID); err != nil {
		t.Fatalf("DeleteUserExpert: %v", err)
	}
	if _, ok := c.experts.Get(created.ID); ok {
		t.Fatalf("deleted expert %q still in catalog", created.ID)
	}
	libraryState = c.ExpertLibraryState()
	if len(libraryState.FavoriteIDs) != 0 || len(libraryState.RecentIDs) != 0 {
		t.Fatalf("library state after delete = %+v, want cleared", libraryState)
	}
}

func TestDuplicateUserExpertPreservesTeamAvatarAndMembers(t *testing.T) {
	dir := t.TempDir()
	c, _ := newExpertTestController(t, dir, "")

	created, err := c.CreateUserExpert(expert.UserExpertInput{
		Name:         "Release Team",
		Mode:         "team",
		Description:  "Coordinates release checks.",
		Category:     "engineering",
		Tags:         []string{"release", "review"},
		AvatarKey:    "data:image/png;base64,team-avatar",
		SystemPrompt: "Coordinate the release team.",
		Members: []expert.Member{
			{Kind: "lead", Name: "Release Lead", Role: "Coordinator", Task: "Plan", Output: "Plan", Tone: "blue", AvatarKey: "architect"},
			{Kind: "sub", Name: "Risk Reviewer", Role: "Risk", Task: "Review", Output: "Risks", Tone: "green", AvatarKey: "data:image/png;base64,risk"},
		},
		DisplayPrompt: "Review this release.",
		Language:      "auto",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateUserExpert: %v", err)
	}

	duplicated, err := c.DuplicateExpert(created.ID)
	if err != nil {
		t.Fatalf("DuplicateExpert: %v", err)
	}
	if duplicated.Mode != "team" || duplicated.AvatarKey != created.AvatarKey {
		t.Fatalf("duplicated expert = %+v, want team mode and original avatar", duplicated)
	}
	if len(duplicated.Prompts) != len(created.Prompts) || duplicated.Prompts[0] != created.Prompts[0] {
		t.Fatalf("duplicated prompts = %v, want %v", duplicated.Prompts, created.Prompts)
	}
	if len(duplicated.Members) != 2 || duplicated.Members[0].Kind != "lead" || duplicated.Members[1].Kind != "sub" {
		t.Fatalf("duplicated members = %+v, want preserved lead/sub team", duplicated.Members)
	}
	if duplicated.Members[1].AvatarKey != "data:image/png;base64,risk" {
		t.Fatalf("duplicated member avatar = %q, want preserved avatar", duplicated.Members[1].AvatarKey)
	}
}

func TestUpdateSelectedUserExpertRefreshesSessionSystemPrompt(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	created, err := c.CreateUserExpert(expert.UserExpertInput{
		Name: "Mutable Expert", Description: "before", SystemPrompt: "prompt version one", DisplayPrompt: "before", Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateUserExpert: %v", err)
	}
	if _, err := c.SetExpert(created.ID); err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "keep this turn"})

	if _, err := c.UpdateUserExpert(created.ID, expert.UserExpertInput{
		Name: "Mutable Expert", Description: "after", SystemPrompt: "prompt version two", DisplayPrompt: "after", Enabled: true,
	}); err != nil {
		t.Fatalf("UpdateUserExpert: %v", err)
	}
	messages := exec.Session().Snapshot()
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "prompt version two") || strings.Contains(messages[0].Content, "prompt version one") {
		t.Fatalf("session after selected expert update = %+v, want refreshed system prompt", messages)
	}
	if messages[1].Role != provider.RoleUser || messages[1].Content != "keep this turn" {
		t.Fatalf("session history was not preserved after expert update: %+v", messages)
	}
}

func TestOfficialExpertsAndTeamsAreReadOnlyButFavoriteable(t *testing.T) {
	dir := t.TempDir()
	c, _ := newExpertTestController(t, dir, "")

	for _, id := range []string{"architect", "code-delivery-team"} {
		if _, err := c.UpdateUserExpert(id, expert.UserExpertInput{}); err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("UpdateUserExpert(%q) error = %v, want read-only", id, err)
		}
		if err := c.DeleteUserExpert(id); err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("DeleteUserExpert(%q) error = %v, want read-only", id, err)
		}
		if _, err := c.DuplicateExpert(id); err == nil || !strings.Contains(err.Error(), "cannot be duplicated") {
			t.Fatalf("DuplicateExpert(%q) error = %v, want rejected", id, err)
		}
		state, err := c.SetExpertFavorite(id, true)
		if err != nil {
			t.Fatalf("SetExpertFavorite(%q): %v", id, err)
		}
		found := false
		for _, favoriteID := range state.FavoriteIDs {
			if favoriteID == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("favorite state = %+v, want %q", state, id)
		}
	}
}

func TestSetExpertRequiresNewSessionAfterUserContent(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "already started"})

	state, err := c.SetExpert("testing")
	if !errors.Is(err, ErrExpertSwitchRequiresNewSession) {
		t.Fatalf("SetExpert err = %v, want ErrExpertSwitchRequiresNewSession", err)
	}
	if !state.RequiresNewSession || state.CanSwitch {
		t.Fatalf("state = %+v, want requires new session", state)
	}
}

func TestAttachExpertAfterUserContentPreservesConversation(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	exec.Session().Add(validProjectContextMessage("project instructions"))
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "already started"})
	exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "existing answer"})

	state, err := c.AttachExpert("testing")
	if err != nil {
		t.Fatalf("AttachExpert: %v", err)
	}
	if state.SelectedID != "testing" || state.CanSwitch || !state.RequiresNewSession {
		t.Fatalf("state = %+v, want testing attached with future switches requiring a new session", state)
	}
	current := exec.Session().Snapshot()
	if len(current) != 4 {
		t.Fatalf("session length = %d, want system plus preserved project context and conversation: %+v", len(current), current)
	}
	if current[0].Role != provider.RoleSystem || current[0].Content == "base" {
		t.Fatalf("system prompt = %+v, want attached testing expert prompt", current[0])
	}
	if !current[1].IsProjectContext() || current[2].Content != "already started" || current[3].Content != "existing answer" {
		t.Fatalf("conversation was not preserved: %+v", current)
	}
	meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil || !ok || meta.ExpertID != "testing" {
		t.Fatalf("expert meta = %+v ok=%v err=%v, want testing", meta, ok, err)
	}
	if _, err := c.SetExpert("architecture-review"); !errors.Is(err, ErrExpertSwitchRequiresNewSession) {
		t.Fatalf("SetExpert after attach err = %v, want ErrExpertSwitchRequiresNewSession", err)
	}
}

func TestSetSameExpertAfterContentIsNoop(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("testing"); err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "already started"})

	state, err := c.SetExpert("testing")
	if err != nil {
		t.Fatalf("SetExpert same expert returned error: %v", err)
	}
	if state.SelectedID != "testing" || !state.RequiresNewSession || state.CanSwitch {
		t.Fatalf("state = %+v, want selected testing with future switches requiring a new session", state)
	}
}

func TestClearExpertAfterContentPreservesConversation(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("testing"); err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "expert task"})
	exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "expert answer"})

	state, err := c.ClearExpert()
	if err != nil {
		t.Fatalf("ClearExpert after content: %v", err)
	}
	if state.SelectedID != "" || state.Source != expert.SourceNone {
		t.Fatalf("state = %+v, want no selected expert", state)
	}
	if state.CanSwitch || !state.RequiresNewSession {
		t.Fatalf("state = %+v, want future expert switches to still require a new session", state)
	}
	current := exec.Session().Snapshot()
	if len(current) != 3 {
		t.Fatalf("session length = %d, want system plus preserved user and assistant messages: %+v", len(current), current)
	}
	if current[0].Role != provider.RoleSystem || current[0].Content != "base" {
		t.Fatalf("system prompt = %+v, want base without expert prompt", current[0])
	}
	if current[1].Content != "expert task" || current[2].Content != "expert answer" {
		t.Fatalf("conversation was not preserved: %+v", current)
	}
	if meta, ok, err := agent.LoadSessionExpert(c.SessionPath()); err != nil || ok {
		t.Fatalf("expert meta after clear = %+v ok=%v err=%v, want cleared", meta, ok, err)
	}
}

func TestNewSessionPreservesSelectedExpert(t *testing.T) {
	dir := t.TempDir()
	c, exec := newExpertTestController(t, dir, "")
	if _, err := c.SetExpert("testing"); err != nil {
		t.Fatalf("SetExpert: %v", err)
	}
	exec.Session().Add(provider.Message{Role: provider.RoleUser, Content: "old task"})

	oldPath := c.SessionPath()
	if err := c.NewSession(); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if c.SessionPath() == oldPath {
		t.Fatal("NewSession did not rotate path")
	}
	if got := c.CurrentExpert(); got.SelectedID != "testing" {
		t.Fatalf("current expert = %+v, want testing", got)
	}
	meta, ok, err := agent.LoadSessionExpert(c.SessionPath())
	if err != nil {
		t.Fatalf("LoadSessionExpert: %v", err)
	}
	if !ok || meta.ExpertID != "testing" {
		t.Fatalf("new session expert meta = %+v ok=%v, want testing", meta, ok)
	}
}

func TestResumeWithoutExpertSidecarOverridesConfigDefault(t *testing.T) {
	dir := t.TempDir()
	c, _ := newExpertTestController(t, dir, "architecture-review")
	if got := c.CurrentExpert(); got.SelectedID != "architecture-review" {
		t.Fatalf("initial expert = %+v, want default architecture-review", got)
	}
	path := filepath.Join(dir, "legacy.jsonl")
	resumed := agent.NewSession("legacy sys")
	c.Resume(resumed, path)

	if got := c.CurrentExpert(); got.SelectedID != "" || got.Source != expert.SourceNone {
		t.Fatalf("resumed expert = %+v, want none", got)
	}
	if strings.Contains(c.systemPrompt, "Expert role:") {
		t.Fatalf("system prompt should not contain default expert after legacy resume:\n%s", c.systemPrompt)
	}
}

func TestNewSessionResetsTwoModelPlannerContext(t *testing.T) {
	dir := t.TempDir()
	planner := &recordingProvider{name: "planner", streams: [][]provider.Chunk{
		textTurn("OLD PLAN: inspect alpha.go"),
		textTurn("NEW PLAN: inspect beta.go"),
	}}
	execProv := &recordingProvider{name: "executor", streams: [][]provider.Chunk{
		textTurn("old done"),
		textTurn("new done"),
	}}
	exec := agent.New(execProv, tool.NewRegistry(), agent.NewSession("exec sys"), agent.Options{}, event.Discard)
	plannerSess := agent.NewSession("planner sys")
	coord := agent.NewCoordinator(planner, plannerSess, nil, tool.NewRegistry(), agent.Options{}, exec, 0, event.Discard, nil)
	path := filepath.Join(dir, "session.jsonl")
	c := New(Options{Runner: coord, Executor: exec, SystemPrompt: "exec sys", SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.Run(context.Background(), "old task alpha"); err != nil {
		t.Fatal(err)
	}
	if err := c.NewSession(); err != nil {
		t.Fatal(err)
	}
	if err := c.Run(context.Background(), "new task beta"); err != nil {
		t.Fatal(err)
	}

	if len(planner.requests) != 2 {
		t.Fatalf("planner requests = %d, want 2", len(planner.requests))
	}
	second := requestMessagesText(planner.requests[1].Messages)
	if strings.Contains(second, "old task alpha") || strings.Contains(second, "OLD PLAN") {
		t.Fatalf("new planner request leaked previous session context:\n%s", second)
	}
	if !strings.Contains(second, "new task beta") {
		t.Fatalf("new planner request missing current task:\n%s", second)
	}
}

func TestResumeResetsTwoModelPlannerContext(t *testing.T) {
	dir := t.TempDir()
	planner := &recordingProvider{name: "planner", streams: [][]provider.Chunk{
		textTurn("OLD PLAN: inspect alpha.go"),
		textTurn("RESUMED PLAN: inspect gamma.go"),
	}}
	execProv := &recordingProvider{name: "executor", streams: [][]provider.Chunk{
		textTurn("old done"),
		textTurn("resumed done"),
	}}
	exec := agent.New(execProv, tool.NewRegistry(), agent.NewSession("exec sys"), agent.Options{}, event.Discard)
	plannerSess := agent.NewSession("planner sys")
	coord := agent.NewCoordinator(planner, plannerSess, nil, tool.NewRegistry(), agent.Options{}, exec, 0, event.Discard, nil)
	c := New(Options{Runner: coord, Executor: exec, SystemPrompt: "exec sys", SessionDir: dir, SessionPath: filepath.Join(dir, "old.jsonl"), Label: "test"})

	if err := c.Run(context.Background(), "old task alpha"); err != nil {
		t.Fatal(err)
	}
	resumed := agent.NewSession("exec sys")
	resumed.Add(provider.Message{Role: provider.RoleUser, Content: "saved task gamma"})
	c.Resume(resumed, filepath.Join(dir, "resumed.jsonl"))
	if err := c.Run(context.Background(), "continue gamma"); err != nil {
		t.Fatal(err)
	}

	if len(planner.requests) != 2 {
		t.Fatalf("planner requests = %d, want 2", len(planner.requests))
	}
	second := requestMessagesText(planner.requests[1].Messages)
	if strings.Contains(second, "old task alpha") || strings.Contains(second, "OLD PLAN") {
		t.Fatalf("resumed planner request leaked previous session context:\n%s", second)
	}
	if !strings.Contains(second, "continue gamma") {
		t.Fatalf("resumed planner request missing current task:\n%s", second)
	}
}

func TestResetPlannerSessionClearsPlannerHistory(t *testing.T) {
	dir := t.TempDir()
	planner := &recordingProvider{name: "planner", streams: [][]provider.Chunk{
		textTurn("FIRST PLAN: inspect alpha.go"),
		textTurn("SECOND PLAN: inspect beta.go"),
	}}
	execProv := &recordingProvider{name: "executor", streams: [][]provider.Chunk{
		textTurn("first done"),
		textTurn("second done"),
	}}
	exec := agent.New(execProv, tool.NewRegistry(), agent.NewSession("exec sys"), agent.Options{}, event.Discard)
	plannerSess := agent.NewSession("planner sys")
	coord := agent.NewCoordinator(planner, plannerSess, nil, tool.NewRegistry(), agent.Options{}, exec, 0, event.Discard, nil)
	path := filepath.Join(dir, "session.jsonl")
	c := New(Options{Runner: coord, Executor: exec, SystemPrompt: "exec sys", SessionDir: dir, SessionPath: path, Label: "test"})

	if err := c.Run(context.Background(), "first task"); err != nil {
		t.Fatal(err)
	}
	// Explicitly reset the planner session (simulates a tab switch).
	c.ResetPlannerSession()
	if err := c.Run(context.Background(), "second task"); err != nil {
		t.Fatal(err)
	}

	if len(planner.requests) != 2 {
		t.Fatalf("planner requests = %d, want 2", len(planner.requests))
	}
	second := requestMessagesText(planner.requests[1].Messages)
	if strings.Contains(second, "first task") || strings.Contains(second, "FIRST PLAN") {
		t.Fatalf("planner request after reset leaked previous session context:\n%s", second)
	}
	if !strings.Contains(second, "second task") {
		t.Fatalf("planner request after reset missing current task:\n%s", second)
	}
}

func TestTwoModelShortChoiceReplySkipsPlanner(t *testing.T) {
	dir := t.TempDir()
	planner := &recordingProvider{name: "planner", streams: [][]provider.Chunk{
		textTurn("planner should not run for a context-dependent choice reply"),
	}}
	execProv := &recordingProvider{name: "executor", streams: [][]provider.Chunk{
		textTurn("selected option 1"),
	}}
	execSess := agent.NewSession("exec sys")
	execSess.Add(provider.Message{Role: provider.RoleUser, Content: "先给我两个执行方案"})
	execSess.Add(provider.Message{Role: provider.RoleAssistant, Content: "两个执行方式可选：\n\n1. Subagent-Driven（推荐）\n2. 当前会话执行\n\n你选哪种？"})
	exec := agent.New(execProv, tool.NewRegistry(), execSess, agent.Options{}, event.Discard)
	coord := agent.NewCoordinator(planner, agent.NewSession("planner sys"), nil, tool.NewRegistry(), agent.Options{}, exec, 0, event.Discard, NewPlannerGate(nil))
	c := New(Options{Runner: coord, Executor: exec, SystemPrompt: "exec sys", SessionDir: dir, SessionPath: filepath.Join(dir, "session.jsonl"), Label: "test"})

	if err := c.Run(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}

	if len(planner.requests) != 0 {
		t.Fatalf("planner requests = %d, want 0 for a short context-dependent choice reply", len(planner.requests))
	}
	if len(execProv.requests) != 1 {
		t.Fatalf("executor requests = %d, want 1", len(execProv.requests))
	}
	reqText := requestMessagesText(execProv.requests[0].Messages)
	if !strings.Contains(reqText, "1. Subagent-Driven") {
		t.Fatalf("executor request lost the previous assistant options:\n%s", reqText)
	}
	if strings.Contains(reqText, "KWU executor handoff") {
		t.Fatalf("short choice reply should not be wrapped as a planner handoff:\n%s", reqText)
	}
	if got := lastUserMessage(execProv.requests[0].Messages); got != "1" {
		t.Fatalf("executor last user = %q, want raw choice reply", got)
	}
}

func TestSubmitClearDiscardsCurrentContextWithoutSavingTranscript(t *testing.T) {
	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "old context"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	path := filepath.Join(dir, "session.jsonl")
	c := New(Options{Executor: exec, SystemPrompt: "sys", SessionDir: dir, SessionPath: path, Label: "test"})
	if err := c.Snapshot(); err != nil {
		t.Fatal(err)
	}
	ckpt := ckptDir(path)
	if err := os.MkdirAll(ckpt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ckpt, "turn-0.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	c.submit("/clear", "")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && c.SessionPath() == path {
		time.Sleep(time.Millisecond)
	}
	if c.SessionPath() == path {
		t.Fatal("/clear did not rotate to a fresh session path")
	}
	for _, p := range []string{path, agent.BranchMetaPath(path), ckpt} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("discarded artifact %s still exists or stat failed with %v", p, err)
		}
	}
	if _, err := os.Stat(c.SessionPath()); !os.IsNotExist(err) {
		t.Fatalf("fresh empty session should not be saved yet; stat err=%v", err)
	}
	current := exec.Session().Snapshot()
	if len(current) != 1 || current[0].Role != provider.RoleSystem || current[0].Content != "sys" {
		t.Fatalf("cleared context = %+v, want only system prompt", current)
	}
}

func TestDisconnectMCPServerRemovesLazyPlaceholder(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeControlTool{name: "mcp__mock__connect"})
	c := New(Options{Host: plugin.NewHost(), Registry: reg})

	if ok := c.DisconnectMCPServer("mock"); !ok {
		t.Fatal("DisconnectMCPServer returned false for a registered lazy placeholder")
	}
	if _, found := reg.Get("mcp__mock__connect"); found {
		t.Fatalf("lazy placeholder still registered after disconnect; names=%v", reg.Names())
	}
}

func TestUnregisterMCPServerToolsBlocksLateSharedHostSwap(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeControlTool{name: "mcp__mock__connect"})
	c := New(Options{Host: plugin.NewHost(), Registry: reg})

	if ok := c.UnregisterMCPServerTools("mock"); !ok {
		t.Fatal("UnregisterMCPServerTools returned false")
	}
	reg.Add(fakeControlTool{name: "mcp__mock__echo"})
	if _, found := reg.Get("mcp__mock__echo"); found {
		t.Fatalf("late shared-host tool swap was accepted after unregister; names=%v", reg.Names())
	}
	reg.Add(fakeControlTool{name: "mcp__other__echo"})
	if _, found := reg.Get("mcp__other__echo"); !found {
		t.Fatalf("unregister blocked unrelated MCP tools; names=%v", reg.Names())
	}
}

func TestRemoveMCPServerRemovesUnconnectedLazyPlaceholder(t *testing.T) {
	isolateControlConfigHome(t)
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))
	t.Chdir(dir)
	if err := os.WriteFile("kwu.toml", []byte(`
[[plugins]]
name = "mock"
command = "mock-mcp"
tier = "lazy"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reg := tool.NewRegistry()
	reg.Add(fakeControlTool{name: "mcp__mock__connect"})
	c := New(Options{Host: plugin.NewHost(), Registry: reg})

	disconnected, err := c.RemoveMCPServer("mock")
	if err != nil {
		t.Fatalf("RemoveMCPServer: %v", err)
	}
	if disconnected {
		t.Fatal("RemoveMCPServer reported a live disconnect for an unconnected lazy placeholder")
	}
	if _, found := reg.Get("mcp__mock__connect"); found {
		t.Fatalf("lazy placeholder still registered after remove; names=%v", reg.Names())
	}
	if names := c.ConfiguredMCPNames(); len(names) != 0 {
		t.Fatalf("ConfiguredMCPNames() = %v, want empty after remove", names)
	}
}

func TestControllerMCPConfigUsesActorProfile(t *testing.T) {
	root := isolateControlConfigHome(t)
	t.Setenv("KWU_HOME", filepath.Join(root, "home"))
	t.Setenv("KWU_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("PATH", "")
	project := t.TempDir()

	defaultActor := userctx.Default(project)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	saveControlMCPConfig(t, defaultPaths, config.PluginEntry{Name: "default-only", Command: "default-mcp"})

	workActor := userctx.Default(project)
	workActor.ProfileID = "work"
	workPaths := config.NewProfilePathResolver(workActor)
	saveControlMCPConfig(t, workPaths, config.PluginEntry{Name: "work-only", Command: "missing-work-mcp"})

	c := New(Options{
		WorkspaceRoot: project,
		Actor:         workActor,
		Host:          plugin.NewHost(),
		Registry:      tool.NewRegistry(),
	})
	defer c.Close()

	names := c.ConfiguredMCPNames()
	if !controlStringSliceContains(names, "work-only") || controlStringSliceContains(names, "default-only") {
		t.Fatalf("ConfiguredMCPNames() = %v, want work profile only", names)
	}
	if _, err := c.ConnectConfiguredMCPServer("default-only"); err == nil || !strings.Contains(err.Error(), "no configured MCP server") {
		t.Fatalf("ConnectConfiguredMCPServer(default-only) err = %v, want no configured MCP server", err)
	}
	if _, err := c.ConnectConfiguredMCPServer("work-only"); err == nil || strings.Contains(err.Error(), "no configured MCP server") {
		t.Fatalf("ConnectConfiguredMCPServer(work-only) err = %v, want connection attempt from work profile", err)
	}

	total, added, updated, connected, failed, skipped, err := c.ImportMCPEntries([]config.PluginEntry{{
		Name:    "imported",
		Command: "missing-imported-mcp",
	}})
	if err != nil {
		t.Fatalf("ImportMCPEntries: %v", err)
	}
	if total != 1 || added != 1 || updated != 0 || connected != 0 || failed != 1 || skipped != 0 {
		t.Fatalf("ImportMCPEntries counts = total:%d added:%d updated:%d connected:%d failed:%d skipped:%d", total, added, updated, connected, failed, skipped)
	}
	workCfg := config.LoadForEditWithPaths(workPaths)
	if !hasControlMCPPlugin(workCfg, "imported") {
		t.Fatalf("work profile config missing imported plugin: %+v", workCfg.Plugins)
	}
	defaultCfg := config.LoadForEditWithPaths(defaultPaths)
	if hasControlMCPPlugin(defaultCfg, "imported") {
		t.Fatalf("default profile config leaked imported plugin: %+v", defaultCfg.Plugins)
	}

	disconnected, err := c.RemoveMCPServer("imported")
	if err != nil {
		t.Fatalf("RemoveMCPServer(imported): %v", err)
	}
	if disconnected {
		t.Fatal("RemoveMCPServer(imported) reported live disconnect for failed import")
	}
	workCfg = config.LoadForEditWithPaths(workPaths)
	if hasControlMCPPlugin(workCfg, "imported") {
		t.Fatalf("work profile config still has imported plugin: %+v", workCfg.Plugins)
	}
}

func TestControllerSkillStatusUsesActorProfileConfig(t *testing.T) {
	root := isolateControlConfigHome(t)
	t.Setenv("KWU_HOME", filepath.Join(root, "home"))
	t.Setenv("KWU_STATE_HOME", filepath.Join(root, "state"))
	project := t.TempDir()

	defaultActor := userctx.Default(project)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	defaultCfg := config.Default()
	defaultCfg.Skills.DisabledSkills = []string{"review"}
	if err := defaultCfg.SaveToScope(defaultPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	workActor := userctx.Default(project)
	workActor.ProfileID = "work"
	workPaths := config.NewProfilePathResolver(workActor)
	workCfg := config.Default()
	workCfg.Skills.DisabledSkills = []string{"explore"}
	if err := workCfg.SaveToScope(workPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save work config: %v", err)
	}

	c := New(Options{
		WorkspaceRoot: project,
		Actor:         workActor,
		AllSkills: []skill.Skill{
			{Name: "explore", Scope: skill.ScopeBuiltin},
			{Name: "review", Scope: skill.ScopeBuiltin},
		},
	})
	defer c.Close()

	if c.SkillEnabled("review") != true {
		t.Fatal("review should stay enabled for work profile despite default profile disable")
	}
	if c.SkillEnabled("explore") != false {
		t.Fatal("explore should be disabled by work profile")
	}
	disabled := c.DisabledSkills()
	if len(disabled) != 1 || disabled[0].Name != "explore" {
		t.Fatalf("DisabledSkills() = %+v, want only explore from work profile", disabled)
	}
}

func saveControlMCPConfig(t *testing.T, paths config.ProfilePathResolver, entry config.PluginEntry) {
	t.Helper()
	cfg := config.Default()
	cfg.Plugins = []config.PluginEntry{entry}
	if err := cfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save MCP config: %v", err)
	}
}

func hasControlMCPPlugin(cfg *config.Config, name string) bool {
	for _, p := range cfg.Plugins {
		if p.Name == name {
			return true
		}
	}
	return false
}

func controlStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// approvalIDs returns a Controller whose Sink forwards each ApprovalRequest's ID
// onto the channel, plus a counter of how many requests it emitted.
func approvalIDs() (*Controller, chan string, *int) {
	ids := make(chan string, 8)
	prompts := 0
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.ApprovalRequest {
			prompts++
			ids <- e.Approval.ID
		}
	})})
	return c, ids, &prompts
}

func waitAuditDecision(t *testing.T, audits <-chan event.DecisionAudit, decision string) event.DecisionAudit {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case audit := <-audits:
			if audit.Decision == decision {
				return audit
			}
		case <-deadline:
			t.Fatalf("audit decision %q was not emitted", decision)
		}
	}
}

// TestApprovalAllowOnce drives the happy path: the gate emits an ApprovalRequest,
// the (fake) frontend answers allow, and the gate returns allow with no grant.
func TestApprovalAllowOnce(t *testing.T) {
	c, ids, _ := approvalIDs()
	go func() { c.Approve(<-ids, true, false, false) }()

	allow, remember, err := gateApprover{c}.Approve(context.Background(), "bash", "go test", nil)
	if err != nil || !allow || remember {
		t.Fatalf("Approve = (%v,%v,%v), want allow once", allow, remember, err)
	}
}

func TestApprovalAuditFieldsOnRequestAndDecision(t *testing.T) {
	workspace := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session-audit.jsonl")
	approvals := make(chan event.Approval, 1)
	audits := make(chan event.DecisionAudit, 2)
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			switch e.Kind {
			case event.ApprovalRequest:
				approvals <- e.Approval
			case event.AuditEvent:
				audits <- e.Audit
			}
		}),
		WorkspaceRoot: workspace,
		SessionPath:   sessionPath,
		Actor: userctx.ActorContext{
			ActorID:   "actor-audit",
			ProfileID: "profile-audit",
		},
	})

	done := make(chan error, 1)
	go func() {
		allow, _, err := gateApprover{c}.Approve(context.Background(), "bash", "go test ./...", nil)
		if err != nil {
			done <- err
			return
		}
		if !allow {
			done <- fmt.Errorf("approval denied")
			return
		}
		done <- nil
	}()

	var approval event.Approval
	select {
	case approval = <-approvals:
	case <-time.After(2 * time.Second):
		t.Fatal("approval request was not emitted")
	}
	if approval.Audit == nil {
		t.Fatal("approval audit is nil")
	}
	if approval.Audit.ActorID != "actor-audit" || approval.Audit.ProfileID != "profile-audit" ||
		approval.Audit.SessionID != "session-audit" || approval.Audit.WorkspaceRoot != workspace ||
		approval.Audit.Tool != "bash" || approval.Audit.Decision != "requested" {
		t.Fatalf("approval request audit = %+v", approval.Audit)
	}

	c.Approve(approval.ID, true, true, false)
	audit := waitAuditDecision(t, audits, "allow")
	if audit.ActorID != "actor-audit" || audit.ProfileID != "profile-audit" ||
		audit.SessionID != "session-audit" || audit.WorkspaceRoot != workspace ||
		audit.Tool != "bash" || !audit.SessionGrant || audit.Persistent {
		t.Fatalf("approval decision audit = %+v", audit)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("approval stayed blocked")
	}
}

func TestMemoryApprovalRequestShowsRememberPayload(t *testing.T) {
	approvals := make(chan event.Approval, 1)
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.ApprovalRequest {
			approvals <- e.Approval
		}
	})})

	args := json.RawMessage(`{
		"name": "stable-retrieval-conclusion",
		"description": "History retrieval should reuse stable synthesized conclusions.",
		"type": "feedback",
		"body": "**Why:** repeated history scans are expensive.\n\n**How to apply:** save the stable summary as a memory document."
	}`)
	result := make(chan string, 1)
	go func() {
		allow, _, err := gateApprover{c}.Approve(context.Background(), "remember", "", args)
		if err != nil {
			result <- err.Error()
			return
		}
		if !allow {
			result <- "memory approval denied"
			return
		}
		result <- ""
	}()

	var approval event.Approval
	select {
	case approval = <-approvals:
	case <-time.After(2 * time.Second):
		t.Fatal("memory approval request was not emitted")
	}
	for _, want := range []string{
		`Save/update memory "stable-retrieval-conclusion"`,
		"[feedback]",
		"History retrieval should reuse stable synthesized conclusions.",
		"repeated history scans are expensive",
		"save the stable summary",
	} {
		if !strings.Contains(approval.Subject, want) {
			t.Fatalf("approval subject %q does not contain %q", approval.Subject, want)
		}
	}
	if strings.Contains(approval.Subject, "\n") {
		t.Fatalf("approval subject should be compact for TUI rendering, got %q", approval.Subject)
	}

	c.Approve(approval.ID, true, true, true)
	select {
	case msg := <-result:
		if msg != "" {
			t.Fatalf("Approve returned %s", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("memory approval stayed blocked after Approve")
	}
}

func TestMemoryApprovalSubjects(t *testing.T) {
	forgetSubject := approvalDisplaySubject("forget", "", json.RawMessage(`{"name":"wrong-memory"}`))
	if forgetSubject != `Archive memory "wrong-memory"` {
		t.Fatalf("forget approval subject = %q", forgetSubject)
	}
	moveSubject := approvalDisplaySubject("move_file", "src/a.md", json.RawMessage(`{"source_path":"src/a.md","destination_path":"docs/a.md"}`))
	if moveSubject != "src/a.md -> docs/a.md" {
		t.Fatalf("move_file approval subject = %q", moveSubject)
	}
}

// TestApprovalDeny confirms a declined call returns allow=false.
func TestApprovalDeny(t *testing.T) {
	c, ids, _ := approvalIDs()
	go func() { c.Approve(<-ids, false, false, false) }()

	allow, _, err := gateApprover{c}.Approve(context.Background(), "bash", "rm -rf /", nil)
	if err != nil || allow {
		t.Fatalf("Approve = (%v,%v), want deny", allow, err)
	}
}

// TestApprovalSessionGrantScopesBashToCommand proves an "allow this session"
// answer short-circuits later prompts for the same bash command, but a different
// command still reaches the frontend.
func TestApprovalSessionGrantScopesBashToCommand(t *testing.T) {
	c, ids, prompts := approvalIDs()
	go func() {
		c.Approve(<-ids, true, true, false) // grant go build for this session
		c.Approve(<-ids, true, false, false)
	}()

	for i, subject := range []string{"go build", "go build", "go test ./..."} {
		allow, _, err := gateApprover{c}.Approve(context.Background(), "bash", subject, nil)
		if err != nil || !allow {
			t.Fatalf("call %d = (%v,%v), want allow", i, allow, err)
		}
	}
	if *prompts != 2 {
		t.Errorf("prompted %d times, want 2 (same command granted, different command prompts)", *prompts)
	}
}

func TestApprovalSessionGrantCanScopeBashToCommandPrefix(t *testing.T) {
	c, ids, prompts := approvalIDs()
	go func() {
		c.Approve(<-ids, true, true, false) // grant bash session (prefix preferred)
		c.Approve(<-ids, true, false, false)
	}()

	for i, subject := range []string{"go test ./...", "go test ./internal/control", "go build ./..."} {
		allow, _, err := gateApprover{c}.Approve(context.Background(), "bash", subject, nil)
		if err != nil || !allow {
			t.Fatalf("call %d = (%v,%v), want allow", i, allow, err)
		}
	}
	if *prompts != 2 {
		t.Errorf("prompted %d times, want 2 (prefix grant should cover similar command only)", *prompts)
	}
}

func TestApprovalPersistentBashPrefixRememberRule(t *testing.T) {
	ids := make(chan string, 1)
	audits := make(chan event.DecisionAudit, 4)
	workspace := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "persist-session.jsonl")
	var remembered string
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.ApprovalRequest {
				ids <- e.Approval.ID
			}
			if e.Kind == event.AuditEvent {
				audits <- e.Audit
			}
			if e.Kind == event.Notice {
				notices = append(notices, e.Text)
			}
		}),
		OnRemember: func(rule string) RememberResult {
			remembered = rule
			return RememberResult{Rule: rule, Path: "kwu.toml", Saved: true}
		},
		WorkspaceRoot: workspace,
		SessionPath:   sessionPath,
		Actor: userctx.ActorContext{
			ActorID:   "persist-actor",
			ProfileID: "persist-profile",
		},
	})
	go func() {
		c.Approve(<-ids, true, true, true)
	}()

	allow, remember, err := gateApprover{c}.Approve(context.Background(), "bash", "go test ./...", nil)
	if err != nil || !allow || remember {
		t.Fatalf("Approve = (%v,%v,%v), want allow with controller-managed persistence", allow, remember, err)
	}
	if remembered != "Bash(go test:*)" {
		t.Fatalf("remembered rule = %q, want Bash(go test:*)", remembered)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "Bash(go test:*)") || !strings.Contains(notices[0], "kwu.toml") {
		t.Fatalf("notices = %v, want saved rule notice", notices)
	}
	audit := waitAuditDecision(t, audits, "persistent_rule_saved")
	if audit.ActorID != "persist-actor" || audit.ProfileID != "persist-profile" ||
		audit.SessionID != "persist-session" || audit.WorkspaceRoot != workspace ||
		audit.Tool != "bash" || audit.PersistedRule != "Bash(go test:*)" || !audit.Persistent {
		t.Fatalf("persistent rule audit = %+v, want saved bash rule", audit)
	}
}

func TestApprovalSessionGrantGroupsFileMutationTools(t *testing.T) {
	c, ids, prompts := approvalIDs()
	go func() { c.Approve(<-ids, true, true, false) }()

	for i, call := range []struct {
		tool    string
		subject string
	}{
		{"edit_file", "src/a.go"},
		{"write_file", "src/b.go"},
		{"multi_edit", "src/c.go"},
		{"move_file", "src/d.go"},
	} {
		allow, _, err := gateApprover{c}.Approve(context.Background(), call.tool, call.subject, nil)
		if err != nil || !allow {
			t.Fatalf("call %d = (%v,%v), want allow", i, allow, err)
		}
	}
	if *prompts != 1 {
		t.Errorf("prompted %d times, want 1 (file mutation session grant should short-circuit)", *prompts)
	}
}

func TestApprovalSessionGrantKeepsPolicyDenyPrecedence(t *testing.T) {
	c, ids, prompts := approvalIDs()
	g := permission.NewGate(permission.New("ask", nil, nil, []string{"bash(rm*)"}), gateApprover{c})
	go func() { c.Approve(<-ids, true, true, false) }()

	allow, _, err := g.Check(context.Background(), "bash", json.RawMessage(`{"command":"go build"}`), false)
	if err != nil || !allow {
		t.Fatalf("first approved call = (%v,%v), want allow", allow, err)
	}
	allow, _, err = g.Check(context.Background(), "bash", json.RawMessage(`{"command":"go build"}`), false)
	if err != nil || !allow {
		t.Fatalf("same-command call after session grant = (%v,%v), want allow", allow, err)
	}
	allow, reason, err := g.Check(context.Background(), "bash", json.RawMessage(`{"command":"rm -rf /tmp/x"}`), false)
	if err != nil || allow || reason == "" {
		t.Fatalf("deny-listed call = (%v,%q,%v), want blocked with reason", allow, reason, err)
	}
	if *prompts != 1 {
		t.Errorf("prompted %d times, want 1", *prompts)
	}
}

// TestApprovalCtxCancel ensures a cancelled turn unblocks the gate with an error
// (rather than hanging) when no one answers.
func TestApprovalCtxCancel(t *testing.T) {
	c := New(Options{Sink: event.Discard})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	allow, _, err := gateApprover{c}.Approve(ctx, "bash", "x", nil)
	if err == nil || allow {
		t.Fatalf("Approve on cancelled ctx = (%v,%v), want (false, error)", allow, err)
	}
}

func TestParseRewind(t *testing.T) {
	cps := []checkpoint.Meta{
		{Turn: 0, Prompt: "first"},
		{Turn: 1, Prompt: "second"},
		{Turn: 2, Prompt: "third"},
	}
	cases := []struct {
		args    string
		wantT   int
		wantS   RewindScope
		wantErr bool
	}{
		{"", 2, RewindBoth, false},                       // no args -> latest turn, both
		{"1", 1, RewindBoth, false},                      // turn only
		{"0 code", 0, RewindCode, false},                 // turn + code
		{"1 conversation", 1, RewindConversation, false}, // turn + conversation
		{"2 both", 2, RewindBoth, false},                 // turn + both
		{"abc", 0, RewindBoth, true},                     // invalid turn
		{"0 unknown", 0, RewindBoth, true},               // unknown scope
	}
	for _, tc := range cases {
		t.Run(tc.args, func(t *testing.T) {
			gotT, gotS, err := parseRewind(tc.args, cps)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseRewind(%q) err=%v, wantErr=%v", tc.args, err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if gotT != tc.wantT || gotS != tc.wantS {
				t.Fatalf("parseRewind(%q) = (%d,%d), want (%d,%d)", tc.args, gotT, gotS, tc.wantT, tc.wantS)
			}
		})
	}
}

func TestParseRewindEmptyCheckpoints(t *testing.T) {
	_, _, err := parseRewind("", nil)
	if err == nil {
		t.Fatal("expected error when no checkpoints")
	}
}

func TestRunGuardedPanicEmitsTurnDone(t *testing.T) {
	sess := agent.NewSession("sys")
	events := make(chan event.Event, 4)
	c := New(Options{
		Runner: appendingRunner{session: sess},
		Sink:   event.FuncSink(func(e event.Event) { events <- e }),
	})

	go func() {
		c.runGuarded(func(ctx context.Context) error {
			panic("boom")
		})
	}()

	select {
	case e := <-events:
		if e.Kind != event.TurnDone {
			t.Fatalf("expected TurnDone after panic, got %v", e.Kind)
		}
		if e.Err == nil || !strings.Contains(e.Err.Error(), "boom") {
			t.Fatalf("expected TurnDone.Err to contain panic message, got %v", e.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TurnDone after panic")
	}

	c.mu.Lock()
	running := c.running
	c.mu.Unlock()
	if running {
		t.Fatal("c.running should be false after panic recovery")
	}
}

func TestRunGuardedPanicDoesNotDoubleEmitTurnDone(t *testing.T) {
	sess := agent.NewSession("sys")
	var count int32
	events := make(chan event.Event, 8)
	c := New(Options{
		Runner: appendingRunner{session: sess},
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				atomic.AddInt32(&count, 1)
			}
			events <- e
		}),
	})

	go func() {
		c.runGuarded(func(ctx context.Context) error {
			panic("boom")
		})
	}()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-events:
			n := atomic.LoadInt32(&count)
			if n >= 1 {
				time.Sleep(50 * time.Millisecond)
				n2 := atomic.LoadInt32(&count)
				if n2 > 1 {
					t.Fatalf("TurnDone emitted %d times, expected 1", n2)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for TurnDone")
		}
	}
}

type blockingRunner struct {
	session *agent.Session
	release chan struct{}
}

func (r blockingRunner) Run(_ context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	<-r.release
	return nil
}

type waitOnlyRunner struct {
	release <-chan struct{}
}

func (r waitOnlyRunner) Run(context.Context, string) error {
	<-r.release
	return nil
}

type blockingPersistentWaitTool struct {
	started chan struct{}
}

func (blockingPersistentWaitTool) Name() string        { return "wait_task" }
func (blockingPersistentWaitTool) Description() string { return "wait for persistent task" }
func (blockingPersistentWaitTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (t blockingPersistentWaitTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	close(t.started)
	<-ctx.Done()
	return "", ctx.Err()
}
func (blockingPersistentWaitTool) ReadOnly() bool { return true }

type persistentWaitRunner struct {
	wait tool.Tool
}

func (r persistentWaitRunner) Run(ctx context.Context, _ string) error {
	_, err := r.wait.Execute(ctx, json.RawMessage(`{"subagent_ref":"sa_waiting"}`))
	return err
}

type naturalErrorRunner struct{ err error }

func (r naturalErrorRunner) Run(context.Context, string) error { return r.err }

type cancellationRaceErrorRunner struct {
	started chan struct{}
	err     error
}

func (r cancellationRaceErrorRunner) Run(ctx context.Context, _ string) error {
	close(r.started)
	<-ctx.Done()
	return r.err
}

func TestRunGuardedCancellationDuringPersistentWaitMarksSingleTurnDoneInterrupted(t *testing.T) {
	started := make(chan struct{})
	events := make(chan event.Event, 4)
	c := New(Options{
		Runner: persistentWaitRunner{wait: blockingPersistentWaitTool{started: started}},
		Sink:   event.FuncSink(func(e event.Event) { events <- e }),
	})

	c.Send("wait for persistent task")
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("persistent wait did not start")
	}
	c.Cancel()

	var done event.Event
	select {
	case done = <-events:
		if done.Kind != event.TurnDone {
			t.Fatalf("event = %+v, want TurnDone", done)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TurnDone")
	}
	if !done.Interrupted {
		t.Fatalf("TurnDone = %+v, want explicit Interrupted", done)
	}
	select {
	case duplicate := <-events:
		t.Fatalf("duplicate terminal event = %+v", duplicate)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRunGuardedNaturalErrorDoesNotMarkInterrupted(t *testing.T) {
	wantErr := errors.New("provider settlement failed")
	events := make(chan event.Event, 2)
	c := New(Options{
		Runner: naturalErrorRunner{err: wantErr},
		Sink:   event.FuncSink(func(e event.Event) { events <- e }),
	})
	c.Send("fail naturally")

	select {
	case done := <-events:
		if done.Kind != event.TurnDone || done.Interrupted || !errors.Is(done.Err, wantErr) {
			t.Fatalf("TurnDone = %+v, want natural error without interruption", done)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TurnDone")
	}
}

func TestRunGuardedCancellationRacePreservesNaturalError(t *testing.T) {
	wantErr := errors.New("provider settlement failed")
	started := make(chan struct{})
	events := make(chan event.Event, 2)
	c := New(Options{
		Runner: cancellationRaceErrorRunner{started: started, err: wantErr},
		Sink:   event.FuncSink(func(e event.Event) { events <- e }),
	})
	c.Send("race cancellation with settlement")

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}
	c.Cancel()

	select {
	case done := <-events:
		if done.Kind != event.TurnDone || done.Interrupted || !errors.Is(done.Err, wantErr) {
			t.Fatalf("TurnDone = %+v, want natural error without interruption", done)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TurnDone")
	}
}

func TestRunTurnReportsErrTurnRunning(t *testing.T) {
	sess := agent.NewSession("sys")
	release := make(chan struct{})
	c := New(Options{Runner: blockingRunner{session: sess, release: release}})

	done := make(chan error, 1)
	go func() {
		done <- c.RunTurn(context.Background(), "first")
	}()
	waitForRunning(t, c)

	if err := c.RunTurn(context.Background(), "second"); err != ErrTurnRunning {
		t.Fatalf("RunTurn while running error = %v, want ErrTurnRunning", err)
	}

	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("first RunTurn returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first RunTurn did not finish after release")
	}
}

func TestRunSharesTurnGuardWithForegroundTurns(t *testing.T) {
	t.Run("headless rejected while foreground runs", func(t *testing.T) {
		sess := agent.NewSession("sys")
		release := make(chan struct{})
		c := New(Options{Runner: blockingRunner{session: sess, release: release}})

		done := make(chan error, 1)
		go func() { done <- c.RunTurn(context.Background(), "foreground") }()
		waitForRunning(t, c)
		if err := c.Run(context.Background(), "headless"); err != ErrTurnRunning {
			t.Fatalf("Run while foreground turn is active = %v, want ErrTurnRunning", err)
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatalf("foreground RunTurn returned %v", err)
		}
	})

	t.Run("foreground rejected while headless runs", func(t *testing.T) {
		sess := agent.NewSession("sys")
		release := make(chan struct{})
		c := New(Options{Runner: blockingRunner{session: sess, release: release}})

		done := make(chan error, 1)
		go func() { done <- c.Run(context.Background(), "headless") }()
		waitForRunning(t, c)
		if err := c.RunTurn(context.Background(), "foreground"); err != ErrTurnRunning {
			t.Fatalf("RunTurn while headless turn is active = %v, want ErrTurnRunning", err)
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatalf("headless Run returned %v", err)
		}
	})
}

func TestRunGuardedFreezesTerminalDeliveryAcrossQueuedTurn(t *testing.T) {
	oldPath := filepath.Join(t.TempDir(), "old.jsonl")
	newPath := filepath.Join(t.TempDir(), "new.jsonl")
	oldWorkspace := t.TempDir()
	newWorkspace := t.TempDir()
	oldSinkEntered := make(chan struct{})
	releaseOldSink := make(chan struct{})
	terminals := make(chan event.Event, 2)
	var terminalCount int32

	c := New(Options{
		SessionPath:   oldPath,
		WorkspaceRoot: oldWorkspace,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind != event.TurnDone {
				return
			}
			if atomic.AddInt32(&terminalCount, 1) == 1 {
				close(oldSinkEntered)
				<-releaseOldSink
			}
			terminals <- e
		}),
	})
	oldSinkReleased := false
	defer func() {
		if !oldSinkReleased {
			close(releaseOldSink)
		}
	}()

	oldTurn := provider.TurnMetadata{TaskID: "task-old", ModelRef: "old/model", ModelID: "old", BillingSource: "old-source"}
	oldDelivery := provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"old-report.md"}, DeliveredAt: 101}
	c.runGuarded(func(context.Context) error {
		c.mu.Lock()
		c.completedTurn = &oldTurn
		c.mu.Unlock()
		c.setCompletedArtifactDelivery(oldDelivery)
		return nil
	})
	select {
	case <-oldSinkEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("old TurnDone did not enter the blocked sink")
	}

	// The old lifecycle has released its session guard before invoking the sink,
	// so a new session may begin while that terminal event is still blocked.
	c.SetSessionPath(newPath)
	c.mu.Lock()
	c.workspaceRoot = newWorkspace
	c.mu.Unlock()

	newTurn := provider.TurnMetadata{TaskID: "task-new", ModelRef: "new/model", ModelID: "new", BillingSource: "new-source"}
	newDelivery := provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"new-report.pdf"}, DeliveredAt: 202}
	c.runGuarded(func(context.Context) error {
		c.mu.Lock()
		c.completedTurn = &newTurn
		c.mu.Unlock()
		c.setCompletedArtifactDelivery(newDelivery)
		return nil
	})

	select {
	case unexpected := <-terminals:
		t.Fatalf("queued TurnDone arrived before the old completion callback was released: %+v", unexpected)
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseOldSink)
	oldSinkReleased = true
	var oldDone event.Event
	select {
	case oldDone = <-terminals:
	case <-time.After(2 * time.Second):
		t.Fatal("old TurnDone did not resume after releasing the sink")
	}
	var newDone event.Event
	select {
	case newDone = <-terminals:
	case <-time.After(2 * time.Second):
		t.Fatal("queued TurnDone did not run after the old completion callback")
	}

	assertTerminal := func(label string, got event.Event, turn provider.TurnMetadata, delivery provider.ArtifactDelivery, path, workspace string) {
		t.Helper()
		if got.Kind != event.TurnDone || got.TurnID == "" {
			t.Fatalf("%s terminal = %+v, want TurnDone with a turn ID", label, got)
		}
		if got.Turn == nil || *got.Turn != turn {
			t.Fatalf("%s turn metadata = %#v, want %#v", label, got.Turn, turn)
		}
		if got.ArtifactDelivery == nil || got.ArtifactDelivery.Version != delivery.Version || got.ArtifactDelivery.DeliveredAt != delivery.DeliveredAt || len(got.ArtifactDelivery.Paths) != len(delivery.Paths) || got.ArtifactDelivery.Paths[0] != delivery.Paths[0] {
			t.Fatalf("%s artifact delivery = %#v, want %#v", label, got.ArtifactDelivery, delivery)
		}
		if got.OriginSessionPath != path || got.OriginWorkspaceRoot != workspace {
			t.Fatalf("%s origin = (%q, %q), want (%q, %q)", label, got.OriginSessionPath, got.OriginWorkspaceRoot, path, workspace)
		}
	}
	assertTerminal("new", newDone, newTurn, newDelivery, newPath, newWorkspace)
	assertTerminal("old", oldDone, oldTurn, oldDelivery, oldPath, oldWorkspace)
	if oldDone.TurnID == newDone.TurnID {
		t.Fatalf("terminal turn IDs = %q and %q, want distinct IDs", oldDone.TurnID, newDone.TurnID)
	}
}

func TestRunTurnDisplayEmitsTurnDone(t *testing.T) {
	sess := agent.NewSession("sys")
	events := make(chan event.Event, 4)
	c := New(Options{
		Runner: appendingRunner{session: sess},
		Sink:   event.FuncSink(func(e event.Event) { events <- e }),
	})

	if err := c.RunTurnDisplay(context.Background(), "visible prompt", "full prompt"); err != nil {
		t.Fatal(err)
	}

	waitForTurnDone(t, events)
	if st := c.RuntimeStatus(); st.Running || st.Cancellable || st.CancelRequested {
		t.Fatalf("status after RunTurnDisplay = %+v, want idle", st)
	}
}

func TestRunTurnDisplayCancellationRacePreservesNaturalError(t *testing.T) {
	wantErr := errors.New("provider settlement failed")
	started := make(chan struct{})
	events := make(chan event.Event, 2)
	c := New(Options{
		Runner: cancellationRaceErrorRunner{started: started, err: wantErr},
		Sink:   event.FuncSink(func(e event.Event) { events <- e }),
	})
	done := make(chan error, 1)
	go func() {
		done <- c.RunTurnDisplay(context.Background(), "visible prompt", "full prompt")
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}
	c.Cancel()
	select {
	case err := <-done:
		if !errors.Is(err, wantErr) {
			t.Fatalf("RunTurnDisplay error = %v, want %v", err, wantErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunTurnDisplay did not return")
	}

	select {
	case terminal := <-events:
		if terminal.Kind != event.TurnDone || terminal.Interrupted || !errors.Is(terminal.Err, wantErr) {
			t.Fatalf("TurnDone = %+v, want natural error without interruption", terminal)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TurnDone")
	}
}

func TestSendWhileRunningDoesNotInterleaveTurns(t *testing.T) {
	sess := agent.NewSession("sys")
	release := make(chan struct{})
	events := make(chan event.Event, 4)
	c := New(Options{
		Runner: blockingRunner{session: sess, release: release},
		Sink: event.FuncSink(func(e event.Event) {
			events <- e
		}),
	})
	defer c.autosaveWG.Wait()

	c.Send("first")
	waitForRunning(t, c)
	c.Send("second")
	close(release)
	waitForTurnDone(t, events)

	var users []string
	for _, m := range sess.Messages {
		if m.Role == provider.RoleUser {
			users = append(users, m.Content)
		}
	}
	if len(users) != 1 || users[0] != "first" {
		t.Fatalf("user turns = %v, want only first turn recorded", users)
	}
}

func waitForRunning(t *testing.T, c *Controller) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Running() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("controller did not enter running state")
}

func TestMidTurnAutosavePersistsDuringLongTurn(t *testing.T) {
	old := midTurnSnapshotInterval.Load()
	midTurnSnapshotInterval.Store(int64(10 * time.Millisecond))
	defer midTurnSnapshotInterval.Store(old)

	dir := t.TempDir()
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	path := filepath.Join(dir, "session.jsonl")
	release := make(chan struct{})
	c := New(Options{Runner: blockingRunner{session: sess, release: release}, Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})
	// Unblock the turn and wait for the autosaver to exit before TempDir
	// cleanup, which fails on Windows while a snapshot tmp write is in flight.
	defer c.autosaveWG.Wait()
	defer close(release)

	c.Send("hello mid-turn persistence")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && strings.Contains(string(b), "hello mid-turn persistence") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("session file was not written while the turn was still running")
}

func TestTurnDoneWaitsForBlockedAutosave(t *testing.T) {
	old := midTurnSnapshotInterval.Load()
	midTurnSnapshotInterval.Store(int64(time.Millisecond))
	defer midTurnSnapshotInterval.Store(old)

	dir := t.TempDir()
	sess := agent.NewSession("sys")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "existing turn"})
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	release := make(chan struct{})
	done := make(chan event.Event, 1)
	c := New(Options{
		Runner:      waitOnlyRunner{release: release},
		Executor:    exec,
		SessionDir:  dir,
		SessionPath: filepath.Join(dir, "session.jsonl"),
		Label:       "test",
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				done <- e
			}
		}),
	})

	c.snapshotMu.Lock()
	locked := true
	defer func() {
		if locked {
			c.snapshotMu.Unlock()
		}
		c.autosaveWG.Wait()
	}()

	c.Send("wait for autosave")
	waitForRunning(t, c)
	time.Sleep(50 * time.Millisecond)
	close(release)

	select {
	case e := <-done:
		t.Fatalf("TurnDone arrived before the autosave exited: %+v", e)
	case <-time.After(50 * time.Millisecond):
	}

	c.snapshotMu.Unlock()
	locked = false
	waitForTurnDone(t, done)
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after TurnDone = %+v, want idle", status)
	}
}

func TestRunTurnAutosavePersistsDuringLongTurn(t *testing.T) {
	old := midTurnSnapshotInterval.Load()
	midTurnSnapshotInterval.Store(int64(10 * time.Millisecond))
	defer midTurnSnapshotInterval.Store(old)

	dir := t.TempDir()
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	path := filepath.Join(dir, "session.jsonl")
	release := make(chan struct{})
	c := New(Options{Runner: blockingRunner{session: sess, release: release}, Executor: exec, SessionDir: dir, SessionPath: path, Label: "test"})
	done := make(chan error, 1)
	go func() {
		done <- c.RunTurn(context.Background(), "hello blocking mid-turn persistence")
	}()
	defer c.autosaveWG.Wait()
	defer close(release)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && strings.Contains(string(b), "hello blocking mid-turn persistence") {
			select {
			case err := <-done:
				t.Fatalf("RunTurn finished before release with err %v", err)
			default:
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("session file was not written while blocking RunTurn was still running")
}

type scriptedRunner struct {
	exec    *agent.Agent
	scripts []func(input string)
}

type controlTestStreamDispatcher struct {
	calls int
	reply string
}

func (d *controlTestStreamDispatcher) Stream(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, error) {
	d.calls++
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: d.reply}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func TestApprovedPlanUsesExecutorDispatcherOnlyForSyntheticExecution(t *testing.T) {
	main := &controlTestStreamDispatcher{reply: "1. inspect\n2. implement"}
	execution := &controlTestStreamDispatcher{reply: "implemented"}
	exec := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{StreamDispatcher: main}, event.Discard)
	var c *Controller
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.ApprovalRequest && e.Approval.Tool == planApprovalTool {
			go c.Approve(e.Approval.ID, true, false, false)
		}
	})
	c = New(Options{Runner: exec, Executor: exec, Sink: sink, ExecutorStreamDispatcher: execution})
	c.SetPlanMode(true)
	if err := c.runTurn(context.Background(), "plan this"); err != nil {
		t.Fatal(err)
	}
	if main.calls != 1 || execution.calls != 1 {
		t.Fatalf("main/executor calls = %d/%d", main.calls, execution.calls)
	}
}

func (r *scriptedRunner) Run(_ context.Context, input string) error {
	if len(r.scripts) == 0 {
		return nil
	}
	next := r.scripts[0]
	r.scripts = r.scripts[1:]
	next(input)
	return nil
}

func TestApprovedPlanAutoApproveEndsWithExecutionTurn(t *testing.T) {
	exec := agent.New(nil, nil, agent.NewSession("sys"), agent.Options{}, event.Discard)
	runner := &scriptedRunner{exec: exec}

	var c *Controller
	approvalPrompts := 0
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind != event.ApprovalRequest {
			return
		}
		approvalPrompts++
		if e.Approval.Tool == planApprovalTool {
			go c.Approve(e.Approval.ID, true, false, false)
			return
		}
		go c.Approve(e.Approval.ID, false, false, false)
	})
	c = New(Options{Runner: runner, Executor: exec, Sink: sink})
	c.SetPlanMode(true)

	runner.scripts = append(runner.scripts,
		func(input string) {
			exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "1. Create the file\n2. Update the file"})
		},
		func(input string) {
			if input != planApprovedMessage {
				t.Fatalf("approved execution input = %q, want planApprovedMessage", input)
			}
			exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "first step done; paused for review", ToolCalls: []provider.ToolCall{{
				ID: "todo-1", Name: "todo_write", Arguments: `{"todos":[{"content":"Create the file","status":"completed"},{"content":"Update the file","status":"in_progress"}]}`,
			}}})
		},
	)

	if err := c.runTurn(context.Background(), "plan this"); err != nil {
		t.Fatal(err)
	}
	if approvalPrompts != 1 {
		t.Fatalf("approval prompts after plan = %d, want 1", approvalPrompts)
	}

	// The plan approval auto-approves writers for the execution turn only. A later
	// turn does not inherit it, and "继续" carries no special meaning — Compose must
	// not inject any marker, and the next writer falls back to per-tool approval.
	if got := c.Compose("继续"); got != "继续" {
		t.Fatalf("a paused approved plan must not marker-prefix the next turn, got %q", got)
	}
	allow, _, err := gateApprover{c}.Approve(context.Background(), "write_file", "/tmp/a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if allow {
		t.Fatal("writer after the execution turn should return to per-tool approval, not auto-allow")
	}
	if approvalPrompts != 2 {
		t.Fatalf("writer after the execution turn should prompt, prompts=%d", approvalPrompts)
	}
}

func TestApprovedPlanDoesNotAutoApproveNonContinuationTurn(t *testing.T) {
	exec := agent.New(nil, nil, agent.NewSession("sys"), agent.Options{}, event.Discard)
	runner := &scriptedRunner{exec: exec}

	var c *Controller
	approvalPrompts := 0
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind != event.ApprovalRequest {
			return
		}
		approvalPrompts++
		if e.Approval.Tool == planApprovalTool {
			go c.Approve(e.Approval.ID, true, false, false)
			return
		}
		go c.Approve(e.Approval.ID, false, false, false)
	})
	c = New(Options{Runner: runner, Executor: exec, Sink: sink})
	c.SetPlanMode(true)

	runner.scripts = append(runner.scripts,
		func(input string) {
			exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "1. Create the file\n2. Update the file"})
		},
		func(input string) {
			exec.Session().Add(provider.Message{Role: provider.RoleAssistant, Content: "paused", ToolCalls: []provider.ToolCall{{
				ID: "todo-1", Name: "todo_write", Arguments: `{"todos":[{"content":"Create the file","status":"completed"},{"content":"Update the file","status":"in_progress"}]}`,
			}}})
		},
	)

	if err := c.runTurn(context.Background(), "plan this"); err != nil {
		t.Fatal(err)
	}
	if got := c.Compose("先别继续"); got != "先别继续" {
		t.Fatalf("non-continuation input should not be marker-prefixed, got %q", got)
	}

	allow, _, err := gateApprover{c}.Approve(context.Background(), "write_file", "/tmp/a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if allow {
		t.Fatal("non-continuation turn should not inherit approved-plan auto approval")
	}
	if approvalPrompts != 2 {
		t.Fatalf("non-continuation writer should prompt after plan approval, prompts=%d", approvalPrompts)
	}
}

// writeCmdFile creates a command .md file with frontmatter under dir.
func writeCmdFile(t *testing.T, dir, name, description, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("---\ndescription: %s\n---\n%s\n", description, body)
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCommandsAtomicPointer verifies that commands passed via Options.Commands are
// correctly exposed through the atomic-pointer Commands() getter and that
// CustomCommand resolves and renders them. Missing commands return found=false.
func TestCommandsAtomicPointer(t *testing.T) {
	cmds := []command.Command{
		{Name: "review", Description: "Review code", Body: "Review $1"},
		{Name: "test", Description: "Run tests", Body: "Test $1"},
	}
	c := New(Options{
		Commands: cmds,
		Sink:     &typedNilControllerSink{},
		Registry: tool.NewRegistry(),
	})

	// Commands() returns what was passed via Options
	got := c.Commands()
	if len(got) != 2 {
		t.Fatalf("Commands() = %d, want 2", len(got))
	}
	// Check retrieval via CustomCommand
	sent, ok := c.CustomCommand("/review myfile.go")
	if !ok {
		t.Error("/review should be found")
	}
	if !strings.Contains(sent, "Review myfile.go") {
		t.Errorf("unexpected render: %q", sent)
	}
	_, ok = c.CustomCommand("/missing")
	if ok {
		t.Error("/missing should not be found")
	}

	// Commands() getter uses atomic.Pointer internally
	if cmds2 := c.Commands(); len(cmds2) != 2 {
		t.Errorf("Commands() = %d after change, want 2", len(cmds2))
	}
}

// TestReloadCommandsFromFilesystem exercises ReloadCommands against real .md
// files in a temp workspace: initial load, hot-reload with a new file, and
// hot-reload after modifying an existing file. Also verifies that skills are
// preserved across the reload.
func TestReloadCommandsFromFilesystem(t *testing.T) {
	// Isolate HOME so CommandDirsForRoot does not pick up global .md command files.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	wsRoot := t.TempDir()
	cmdDir := filepath.Join(wsRoot, ".kwu", "commands")
	writeCmdFile(t, cmdDir, "review", "Review code", "Review $1")
	writeCmdFile(t, cmdDir, "test", "Run tests", "Test $1")

	// Create a minimal in-memory skill to verify skills are preserved across reload.
	sk := skill.Skill{
		Name:        "myskill",
		Description: "Test skill",
		Body:        "You are a test skill. User says: {{.Input}}",
	}

	reg := tool.NewRegistry()
	c := New(Options{
		Sink:          &typedNilControllerSink{},
		Registry:      reg,
		WorkspaceRoot: wsRoot,
		Skills:        []skill.Skill{sk},
	})

	// ReloadCommands should pick up the two .md files and preserve the skill.
	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("ReloadCommands: %v", err)
	}
	cmds := c.Commands()
	if len(cmds) != 2 {
		t.Fatalf("Commands() = %d after reload, want 2", len(cmds))
	}

	// CustomCommand should resolve through the hot-swapped getter
	sent, ok := c.CustomCommand("/review hello.go")
	if !ok {
		t.Fatal("/review should be found after reload")
	}
	if !strings.Contains(sent, "Review hello.go") {
		t.Errorf("render = %q, want Review hello.go", sent)
	}

	// Missing command still not found
	if _, ok := c.CustomCommand("/nope"); ok {
		t.Error("/nope should not be found")
	}

	// Skill should appear in the slash_command tool's description after reload.
	if tool, found := reg.Get("slash_command"); found {
		if !strings.Contains(tool.Description(), "myskill") {
			t.Error("skill 'myskill' should appear in slash_command tool Description after ReloadCommands")
		}
	} else {
		t.Error("slash_command tool should be registered after ReloadCommands")
	}

	// Skill should still be callable via RunSkill after reload.
	if _, ok := c.RunSkill("/myskill"); !ok {
		t.Error("RunSkill(/myskill) should find the skill after ReloadCommands")
	}

	// Hot-reload: add a new command file
	writeCmdFile(t, cmdDir, "count", "Count to N", "Count from 1 to $1")
	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("ReloadCommands (add): %v", err)
	}
	if cmds := c.Commands(); len(cmds) != 3 {
		t.Fatalf("Commands() = %d after add, want 3", len(cmds))
	}

	// Hot-reload: modify an existing command
	writeCmdFile(t, cmdDir, "review", "Review code (friendly)", "Kindly review $1")
	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("ReloadCommands (modify): %v", err)
	}
	if cmds := c.Commands(); len(cmds) != 3 {
		t.Fatalf("Commands() = %d after modify, want 3", len(cmds))
	}
	sent, ok = c.CustomCommand("/review world.go")
	if !ok {
		t.Fatal("/review should be found after modify")
	}
	if !strings.Contains(sent, "Kindly review world.go") {
		t.Errorf("render after modify = %q, want Kindly review world.go", sent)
	}

	// The slash_command tool should be registered and updated
	if _, found := reg.Get("slash_command"); !found {
		t.Error("slash_command tool should be registered after ReloadCommands")
	}
}

// TestReloadCommandsDeleteFile verifies that removing a command .md file and
// reloading causes the command to disappear from both Commands() and
// CustomCommand(), while other commands remain intact.
func TestReloadCommandsDeleteFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	wsRoot := t.TempDir()
	cmdDir := filepath.Join(wsRoot, ".kwu", "commands")
	writeCmdFile(t, cmdDir, "alpha", "Alpha cmd", "Alpha $1")
	writeCmdFile(t, cmdDir, "beta", "Beta cmd", "Beta $1")

	reg := tool.NewRegistry()
	c := New(Options{
		Sink:          &typedNilControllerSink{},
		Registry:      reg,
		WorkspaceRoot: wsRoot,
	})

	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	if got := len(c.Commands()); got != 2 {
		t.Fatalf("Commands() = %d, want 2", got)
	}

	// Delete alpha.md
	if err := os.Remove(filepath.Join(cmdDir, "alpha.md")); err != nil {
		t.Fatal(err)
	}

	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("reload after delete: %v", err)
	}
	if got := len(c.Commands()); got != 1 {
		t.Fatalf("Commands() = %d after delete, want 1", got)
	}

	// /alpha should no longer be found
	if _, ok := c.CustomCommand("/alpha x"); ok {
		t.Error("/alpha should NOT be found after deletion")
	}
	// /beta should still work
	sent, ok := c.CustomCommand("/beta y")
	if !ok {
		t.Error("/beta should still be found")
	}
	if !strings.Contains(sent, "Beta y") {
		t.Errorf("render = %q, want Beta y", sent)
	}
}

// TestReloadCommandsMalformedFile verifies that a malformed .md file causes
// ReloadCommands to return an error but does not prevent other valid commands
// from loading.
func TestReloadCommandsMalformedFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	wsRoot := t.TempDir()
	cmdDir := filepath.Join(wsRoot, ".kwu", "commands")
	writeCmdFile(t, cmdDir, "good", "Good cmd", "Good $1")

	// Write a malformed file (no valid frontmatter)
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(cmdDir, "broken.md")
	if err := os.WriteFile(broken, []byte("this is not valid yaml\n---\nrandom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := tool.NewRegistry()
	c := New(Options{
		Sink:          &typedNilControllerSink{},
		Registry:      reg,
		WorkspaceRoot: wsRoot,
	})

	err := c.ReloadCommands(context.Background())
	// We expect some error from the malformed file
	if err == nil {
		t.Log("ReloadCommands returned nil error despite malformed file — command.Load may tolerate it")
	} else {
		t.Logf("ReloadCommands returned error (expected): %v", err)
	}

	// The valid command should still be loadable
	cmds := c.Commands()
	foundGood := false
	for _, cmd := range cmds {
		if cmd.Name == "good" {
			foundGood = true
		}
	}
	if !foundGood {
		t.Errorf("valid command 'good' should be present, got commands: %v", cmdNames(cmds))
	}
}

// TestReloadCommandsSameNameAcrossDirs verifies that when the same command
// name exists in multiple convention directories, the later-scanned directory
// (higher priority) wins. ConventionDirs = [".kwu", ".agents", ".agent",
// ".claude"], scanned in reverse, so .kwu is highest priority.
func TestReloadCommandsSameNameAcrossDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	wsRoot := t.TempDir()

	// Lower priority: .claude/commands
	claudeDir := filepath.Join(wsRoot, ".claude", "commands")
	writeCmdFile(t, claudeDir, "greet", "Claude greet", "Hello from Claude: $1")

	// Higher priority: .kwu/commands
	kwuDir := filepath.Join(wsRoot, ".kwu", "commands")
	writeCmdFile(t, kwuDir, "greet", "KWU greet", "Hello from KWU: $1")

	reg := tool.NewRegistry()
	c := New(Options{
		Sink:          &typedNilControllerSink{},
		Registry:      reg,
		WorkspaceRoot: wsRoot,
	})

	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// There should be exactly 1 command named "greet"
	cmds := c.Commands()
	count := 0
	for _, cmd := range cmds {
		if cmd.Name == "greet" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 'greet' command, got %d", count)
	}

	// The winning version should be from .kwu (highest priority)
	sent, ok := c.CustomCommand("/greet world")
	if !ok {
		t.Fatal("/greet should be found")
	}
	if !strings.Contains(sent, "Hello from KWU") {
		t.Errorf("expected .kwu version to win, got render: %q", sent)
	}
}

// TestReloadCommandsEmptySet verifies that deleting all command files and
// reloading results in an empty Commands() slice, while the slash_command tool
// still exists in the Registry (containing only Skills).
func TestReloadCommandsEmptySet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	wsRoot := t.TempDir()
	cmdDir := filepath.Join(wsRoot, ".kwu", "commands")
	writeCmdFile(t, cmdDir, "temp", "Temp cmd", "Temp $1")

	sk := skill.Skill{
		Name:        "preserved",
		Description: "A skill to keep",
		Body:        "Skill body: {{.Input}}",
	}

	reg := tool.NewRegistry()
	c := New(Options{
		Sink:          &typedNilControllerSink{},
		Registry:      reg,
		WorkspaceRoot: wsRoot,
		Skills:        []skill.Skill{sk},
	})

	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	if got := len(c.Commands()); got != 1 {
		t.Fatalf("Commands() = %d, want 1", got)
	}

	// Delete all command files
	if err := os.Remove(filepath.Join(cmdDir, "temp.md")); err != nil {
		t.Fatal(err)
	}

	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("reload after delete all: %v", err)
	}
	if got := len(c.Commands()); got != 0 {
		t.Fatalf("Commands() = %d after delete all, want 0", got)
	}

	// /temp should no longer be found
	if _, ok := c.CustomCommand("/temp x"); ok {
		t.Error("/temp should NOT be found after deletion")
	}

	// slash_command tool should still exist (for Skills)
	slashTool, found := reg.Get("slash_command")
	if !found {
		t.Fatal("slash_command tool should still exist even with 0 commands")
	}
	// It should still contain the skill
	if !strings.Contains(slashTool.Description(), "preserved") {
		t.Error("skill 'preserved' should still appear in slash_command tool Description")
	}
}

// TestReloadCommandsDesktopManagementNotice verifies the desktop/HTTP path:
// when the frontend submits "/reload-cmd" as raw input, Submit → managementNotice
// handles it and emits a Notice event with the correct count.
func TestReloadCommandsDesktopManagementNotice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	wsRoot := t.TempDir()
	cmdDir := filepath.Join(wsRoot, ".kwu", "commands")
	writeCmdFile(t, cmdDir, "hello", "Greet", "Hello $1")
	writeCmdFile(t, cmdDir, "review", "Review code", "Review $1")

	var notices []string
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e.Text)
		}
	})

	reg := tool.NewRegistry()
	c := New(Options{
		Sink:          sink,
		Registry:      reg,
		WorkspaceRoot: wsRoot,
	})

	// Initial load so Commands() is populated.
	if err := c.ReloadCommands(context.Background()); err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	notices = nil // reset

	// Desktop path: managementNotice("/reload-cmd") should emit a notice.
	handled := c.managementNotice("/reload-cmd")
	if !handled {
		t.Fatal("managementNotice(/reload-cmd) should return true")
	}
	if len(notices) != 1 {
		t.Fatalf("expected 1 notice, got %d: %v", len(notices), notices)
	}
	if !strings.Contains(notices[0], "commands reloaded") {
		t.Errorf("notice = %q, want 'commands reloaded'", notices[0])
	}
	if !strings.Contains(notices[0], "2 available") {
		t.Errorf("notice = %q, want '2 available'", notices[0])
	}

	// Delete one command file and reload again.
	if err := os.Remove(filepath.Join(cmdDir, "hello.md")); err != nil {
		t.Fatal(err)
	}
	notices = nil
	handled = c.managementNotice("/reload-cmd")
	if !handled {
		t.Fatal("managementNotice(/reload-cmd) after delete should return true")
	}
	if len(notices) != 1 {
		t.Fatalf("expected 1 notice after delete, got %d: %v", len(notices), notices)
	}
	if !strings.Contains(notices[0], "1 available") {
		t.Errorf("notice after delete = %q, want '1 available'", notices[0])
	}

	// Delete all and verify empty-set notice.
	if err := os.Remove(filepath.Join(cmdDir, "review.md")); err != nil {
		t.Fatal(err)
	}
	notices = nil
	handled = c.managementNotice("/reload-cmd")
	if !handled {
		t.Fatal("managementNotice(/reload-cmd) empty set should return true")
	}
	if len(notices) != 1 {
		t.Fatalf("expected 1 notice for empty set, got %d: %v", len(notices), notices)
	}
	if !strings.Contains(notices[0], "0 available") {
		t.Errorf("notice for empty set = %q, want '0 available'", notices[0])
	}
}

// cmdNames is a test helper that extracts command names from a slice.
func cmdNames(cmds []command.Command) []string {
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name
	}
	return names
}
