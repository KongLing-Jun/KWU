package automation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/boot"
	"kwu/internal/event"
	"kwu/internal/i18n"
	"kwu/internal/provider"
)

type fakeAutomationController struct {
	runInput     string
	approvalMode string
	sessionPath  string
	usage        *provider.Usage
	history      []provider.Message
	events       []event.Event
	sink         event.Sink
	runErr       error
	snapshotErr  error
	skillOutputs map[string]string
	skillInputs  []string
	ensures      int
	snapshots    int
	closes       int
}

func (c *fakeAutomationController) RunTurn(_ context.Context, input string) error {
	c.runInput = input
	if c.sink != nil {
		for _, ev := range c.events {
			c.sink.Emit(ev)
		}
	}
	return c.runErr
}

func (c *fakeAutomationController) Snapshot() error {
	c.snapshots++
	return c.snapshotErr
}

func (c *fakeAutomationController) SessionPath() string { return c.sessionPath }

func (c *fakeAutomationController) History() []provider.Message { return c.history }

func (c *fakeAutomationController) SetToolApprovalMode(mode string) { c.approvalMode = mode }

func (c *fakeAutomationController) EnsureSessionPath() { c.ensures++ }

func (c *fakeAutomationController) LastUsage() *provider.Usage { return c.usage }

func (c *fakeAutomationController) Close() { c.closes++ }

func (c *fakeAutomationController) RunSkill(input string) (string, bool) {
	c.skillInputs = append(c.skillInputs, input)
	fields := strings.Fields(input)
	if len(fields) == 0 || c.skillOutputs == nil {
		return "", false
	}
	name := strings.TrimPrefix(fields[0], "/")
	out, ok := c.skillOutputs[name]
	return out, ok
}

func TestRunnerBuildOptions(t *testing.T) {
	runDir := t.TempDir()
	ctrl := &fakeAutomationController{
		sessionPath: filepath.Join(runDir, "session", "run.jsonl"),
		usage:       &provider.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
	}
	var got boot.Options
	runner := &BootRunner{
		build: func(_ context.Context, opts boot.Options) (automationController, error) {
			got = opts
			ctrl.sink = opts.Sink
			return ctrl, nil
		},
	}

	result, err := runner.Run(context.Background(), RunRequest{
		Task: Task{
			WorkspaceRoot: " D:/project/KWU ",
			Prompt: PromptConfig{
				Model:  " test-model ",
				Prompt: "run this",
			},
			Execution: ExecutionPolicy{
				ApprovalMode:    ApprovalModeYolo,
				ApprovalTimeout: "2m",
			},
		},
		RunDir: runDir,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got.Model != "test-model" {
		t.Fatalf("Model = %q, want test-model", got.Model)
	}
	if got.WorkspaceRoot != "D:/project/KWU" {
		t.Fatalf("WorkspaceRoot = %q, want trimmed root", got.WorkspaceRoot)
	}
	if got.SessionDir != filepath.Join(runDir, "session") {
		t.Fatalf("SessionDir = %q, want run session dir", got.SessionDir)
	}
	if !got.RequireKey {
		t.Fatal("RequireKey = false, want true")
	}
	if got.ApprovalTimeout != 2*time.Minute {
		t.Fatalf("ApprovalTimeout = %s, want 2m", got.ApprovalTimeout)
	}
	if ctrl.approvalMode != string(ApprovalModeYolo) {
		t.Fatalf("approval mode = %q, want yolo", ctrl.approvalMode)
	}
	if !strings.Contains(ctrl.runInput, "run this") || !strings.Contains(ctrl.runInput, "# Automation Task") {
		t.Fatalf("RunTurn input = %q, want compiled automation prompt", ctrl.runInput)
	}
	if result.SessionPath != ctrl.sessionPath || result.Usage != ctrl.usage {
		t.Fatalf("result = %+v, want session path and usage", result)
	}
}

func TestRunnerWritesSkillAssist(t *testing.T) {
	runDir := t.TempDir()
	ctrl := &fakeAutomationController{skillOutputs: map[string]string{
		"review": "rendered review skill",
	}}
	runner := &BootRunner{
		build: func(_ context.Context, opts boot.Options) (automationController, error) {
			ctrl.sink = opts.Sink
			return ctrl, nil
		},
	}

	_, err := runner.Run(context.Background(), RunRequest{
		Task: Task{
			ID: "task-expert",
			Prompt: PromptConfig{
				Skills: []string{"review"},
				Prompt: "run this",
			},
		},
		Manifest: RunManifest{TaskID: "task-expert", RunID: "run-expert"},
		RunDir:   runDir,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(ctrl.skillInputs) != 1 || !strings.Contains(ctrl.skillInputs[0], "/review") || !strings.Contains(ctrl.skillInputs[0], "run this") {
		t.Fatalf("RunSkill inputs = %+v, want review skill invoked with task prompt", ctrl.skillInputs)
	}
	if !strings.Contains(ctrl.runInput, "## Automation Assist") || !strings.Contains(ctrl.runInput, "rendered review skill") {
		t.Fatalf("RunTurn input missing skill assist:\n%s", ctrl.runInput)
	}
	body, err := os.ReadFile(filepath.Join(runDir, "assist.md"))
	if err != nil {
		t.Fatalf("ReadFile assist.md: %v", err)
	}
	if !strings.Contains(string(body), "## Skill: review") || !strings.Contains(string(body), "rendered review skill") {
		t.Fatalf("assist.md = %q, want rendered skill section", string(body))
	}
	events, err := readArchivedEvents(filepath.Join(runDir, eventsFileName))
	if err != nil {
		t.Fatalf("readArchivedEvents: %v", err)
	}
	if !hasArchivedTextContaining(events, "automation helpers: skills=review") {
		t.Fatalf("events missing helper notice: %+v", events)
	}
}

func TestRunnerFailsWhenRequiredSkillIsMissing(t *testing.T) {
	ctrl := &fakeAutomationController{}
	runner := &BootRunner{
		build: func(context.Context, boot.Options) (automationController, error) {
			return ctrl, nil
		},
	}

	result, err := runner.Run(context.Background(), RunRequest{
		Task:   Task{Prompt: PromptConfig{Skills: []string{"missing"}, Prompt: "run this"}},
		RunDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("Run returned nil error, want missing skill error")
	}
	if result.Status != RunStatusFailed || result.ErrorCode != RunErrorRunnerFailed {
		t.Fatalf("result = %+v, want runner_failed", result)
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v, want missing skill name", err)
	}
	if ctrl.runInput != "" {
		t.Fatalf("RunTurn input = %q, want no main turn after missing skill", ctrl.runInput)
	}
}

func TestApprovalTimeoutDefault(t *testing.T) {
	var got boot.Options
	runner := &BootRunner{
		build: func(_ context.Context, opts boot.Options) (automationController, error) {
			got = opts
			return &fakeAutomationController{}, nil
		},
	}

	if _, err := runner.Run(context.Background(), RunRequest{RunDir: t.TempDir()}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got.ApprovalTimeout != DefaultApprovalTimeout {
		t.Fatalf("ApprovalTimeout = %s, want %s", got.ApprovalTimeout, DefaultApprovalTimeout)
	}
}

func TestAutomationDoesNotInheritTabApproval(t *testing.T) {
	ctrl := &fakeAutomationController{approvalMode: string(ApprovalModeYolo)}
	runner := &BootRunner{
		build: func(context.Context, boot.Options) (automationController, error) {
			return ctrl, nil
		},
	}

	if _, err := runner.Run(context.Background(), RunRequest{RunDir: t.TempDir()}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if ctrl.approvalMode != string(DefaultApprovalMode) {
		t.Fatalf("approval mode = %q, want default %q", ctrl.approvalMode, DefaultApprovalMode)
	}
}

func TestRunnerCreatesFreshController(t *testing.T) {
	var builds int
	runner := &BootRunner{
		build: func(context.Context, boot.Options) (automationController, error) {
			builds++
			return &fakeAutomationController{}, nil
		},
	}
	req := RunRequest{RunDir: t.TempDir()}

	if _, err := runner.Run(context.Background(), req); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if _, err := runner.Run(context.Background(), req); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if builds != 2 {
		t.Fatalf("builds = %d, want 2", builds)
	}
}

func TestRunnerEnsuresSessionPathBeforeRun(t *testing.T) {
	ctrl := &fakeAutomationController{}
	runner := &BootRunner{
		build: func(context.Context, boot.Options) (automationController, error) {
			return ctrl, nil
		},
	}

	if _, err := runner.Run(context.Background(), RunRequest{RunDir: t.TempDir()}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if ctrl.ensures != 1 {
		t.Fatalf("EnsureSessionPath calls = %d, want 1", ctrl.ensures)
	}
}

func TestRunnerSnapshotsAndClosesController(t *testing.T) {
	ctrl := &fakeAutomationController{runErr: errors.New("turn failed")}
	runner := &BootRunner{
		build: func(context.Context, boot.Options) (automationController, error) {
			return ctrl, nil
		},
	}

	result, err := runner.Run(context.Background(), RunRequest{RunDir: t.TempDir()})
	if err == nil {
		t.Fatal("Run returned nil error, want turn error")
	}
	if result.Status != RunStatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if ctrl.snapshots != 1 {
		t.Fatalf("snapshots = %d, want 1", ctrl.snapshots)
	}
	if ctrl.closes != 1 {
		t.Fatalf("closes = %d, want 1", ctrl.closes)
	}
}

func TestRunnerWritesResultFromMessageEvent(t *testing.T) {
	runDir := t.TempDir()
	ctrl := &fakeAutomationController{events: []event.Event{{Kind: event.Message, Text: "final markdown"}}}
	runner := &BootRunner{
		build: func(_ context.Context, opts boot.Options) (automationController, error) {
			ctrl.sink = opts.Sink
			return ctrl, nil
		},
	}

	result, err := runner.Run(context.Background(), RunRequest{
		Task:     Task{ID: "task-1", Prompt: PromptConfig{Prompt: "run"}},
		Manifest: RunManifest{TaskID: "task-1", RunID: "run-1"},
		RunDir:   runDir,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(runDir, resultFileName))
	if err != nil {
		t.Fatalf("ReadFile result: %v", err)
	}
	if string(body) != "final markdown\n" {
		t.Fatalf("result.md = %q, want final markdown", string(body))
	}
	if result.Summary != "final markdown" || result.ResultPath == "" {
		t.Fatalf("result = %+v, want summary and path", result)
	}
}

func TestRunnerWritesEmptyResultExplanation(t *testing.T) {
	runDir := t.TempDir()
	ctrl := &fakeAutomationController{}
	runner := &BootRunner{
		build: func(_ context.Context, opts boot.Options) (automationController, error) {
			ctrl.sink = opts.Sink
			return ctrl, nil
		},
	}

	if _, err := runner.Run(context.Background(), RunRequest{RunDir: runDir}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(runDir, resultFileName))
	if err != nil {
		t.Fatalf("ReadFile result: %v", err)
	}
	if !strings.Contains(string(body), "No final assistant result") {
		t.Fatalf("result.md = %q, want empty result explanation", string(body))
	}
}

func TestBootRunnerEngineArchivesSuccessfulRun(t *testing.T) {
	ctrl := &fakeAutomationController{
		events: []event.Event{{Kind: event.Message, Text: "done"}},
		usage:  &provider.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
	}

	_, manifest, paths, events := runBootRunnerThroughEngine(t, ctrl)

	if manifest.Status != RunStatusSucceeded {
		t.Fatalf("manifest status = %q, want succeeded: %+v", manifest.Status, manifest)
	}
	if manifest.Summary != "done" || manifest.Usage == nil || manifest.Usage.TotalTokens != 5 {
		t.Fatalf("manifest result metadata = %+v, want summary and usage", manifest)
	}
	if manifest.SessionPath != ctrl.sessionPath {
		t.Fatalf("manifest session path = %q, want %q", manifest.SessionPath, ctrl.sessionPath)
	}
	assertUnderRoot(t, paths.Dir, manifest.SessionPath)
	body, err := os.ReadFile(paths.Result)
	if err != nil {
		t.Fatalf("ReadFile result: %v", err)
	}
	if string(body) != "done\n" {
		t.Fatalf("result.md = %q, want done", string(body))
	}
	if ctrl.snapshots != 1 || ctrl.closes != 1 {
		t.Fatalf("snapshot/close = %d/%d, want 1/1", ctrl.snapshots, ctrl.closes)
	}
	if !hasArchivedStatus(events, RunStatusQueued) || !hasArchivedStatus(events, RunStatusRunning) || !hasArchivedStatus(events, RunStatusSucceeded) {
		t.Fatalf("events missing run status lifecycle: %+v", events)
	}
	if !hasArchivedKind(events, event.Message) {
		t.Fatalf("events missing final message: %+v", events)
	}
}

func TestBootRunnerEngineClassifiesRunErrors(t *testing.T) {
	tests := []struct {
		name     string
		runErr   error
		want     RunStatus
		wantCode RunErrorCode
	}{
		{name: "failed", runErr: errors.New("turn failed"), want: RunStatusFailed, wantCode: RunErrorRunnerFailed},
		{name: "blocked", runErr: context.DeadlineExceeded, want: RunStatusBlocked, wantCode: RunErrorApprovalTimeout},
		{name: "canceled", runErr: context.Canceled, want: RunStatusCanceled, wantCode: RunErrorCanceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := &fakeAutomationController{runErr: tt.runErr}

			_, manifest, paths, events := runBootRunnerThroughEngine(t, ctrl)

			if manifest.Status != tt.want {
				t.Fatalf("manifest status = %q, want %q: %+v", manifest.Status, tt.want, manifest)
			}
			if manifest.ErrorCode != tt.wantCode || manifest.ErrorMessage == "" {
				t.Fatalf("manifest error = %q/%q, want %q with message", manifest.ErrorCode, manifest.ErrorMessage, tt.wantCode)
			}
			if !strings.Contains(manifest.Error, tt.runErr.Error()) {
				t.Fatalf("manifest error = %q, want %q", manifest.Error, tt.runErr.Error())
			}
			if ctrl.snapshots != 1 || ctrl.closes != 1 {
				t.Fatalf("snapshot/close = %d/%d, want 1/1", ctrl.snapshots, ctrl.closes)
			}
			if !hasArchivedStatus(events, tt.want) {
				t.Fatalf("events missing status %q: %+v", tt.want, events)
			}
			if !hasArchivedErrorContaining(events, tt.runErr.Error()) {
				t.Fatalf("events missing error %q: %+v", tt.runErr.Error(), events)
			}
			if _, err := os.Stat(paths.Result); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("result.md stat err = %v, want not exist for failed run without output", err)
			}
		})
	}
}

func TestClassifyRunError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		status    RunStatus
		errorCode RunErrorCode
	}{
		{name: "nil", status: RunStatusSucceeded},
		{name: "lease unavailable", err: ErrLeaseUnavailable, status: RunStatusSkipped, errorCode: RunErrorLeaseUnavailable},
		{name: "approval timeout", err: context.DeadlineExceeded, status: RunStatusBlocked, errorCode: RunErrorApprovalTimeout},
		{name: "canceled", err: context.Canceled, status: RunStatusCanceled, errorCode: RunErrorCanceled},
		{name: "runner failed", err: errors.New("boom"), status: RunStatusFailed, errorCode: RunErrorRunnerFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyRunError(tt.err)
			if got.Status != tt.status || got.ErrorCode != tt.errorCode {
				t.Fatalf("classifyRunError = %+v, want status %q code %q", got, tt.status, tt.errorCode)
			}
		})
	}
}

func TestClassifyRunErrorUsesUserFacingProviderMessage(t *testing.T) {
	raw := &provider.APIError{
		Provider: "airclaude_subscription",
		Status:   403,
		Code:     "pre_consume_token_quota_failed",
		Message:  "token quota is not enough",
	}
	got := classifyRunError(raw)
	if got.ErrorMessage != i18n.M.ProviderErrInsufficientCredits {
		t.Fatalf("ErrorMessage = %q, want %q", got.ErrorMessage, i18n.M.ProviderErrInsufficientCredits)
	}
	if raw.Error() == got.ErrorMessage {
		t.Fatal("automation summary should be friendly while the raw error remains diagnostic")
	}
}

func TestBootRunnerEngineRecordsSnapshotFailure(t *testing.T) {
	ctrl := &fakeAutomationController{
		events:      []event.Event{{Kind: event.Message, Text: "partial result"}},
		snapshotErr: errors.New("snapshot failed"),
	}

	_, manifest, paths, events := runBootRunnerThroughEngine(t, ctrl)

	if manifest.Status != RunStatusFailed {
		t.Fatalf("manifest status = %q, want failed: %+v", manifest.Status, manifest)
	}
	if manifest.ErrorCode != RunErrorSnapshotFailed || manifest.ErrorMessage == "" {
		t.Fatalf("manifest error = %q/%q, want snapshot_failed", manifest.ErrorCode, manifest.ErrorMessage)
	}
	if !strings.Contains(manifest.Error, "snapshot failed") {
		t.Fatalf("manifest error = %q, want snapshot failure", manifest.Error)
	}
	if manifest.FinishedAt.IsZero() {
		t.Fatalf("manifest finishedAt is zero: %+v", manifest)
	}
	body, err := os.ReadFile(paths.Result)
	if err != nil {
		t.Fatalf("ReadFile result: %v", err)
	}
	if string(body) != "partial result\n" {
		t.Fatalf("result.md = %q, want partial result", string(body))
	}
	if ctrl.closes != 1 {
		t.Fatalf("closes = %d, want 1", ctrl.closes)
	}
	if !hasArchivedErrorContaining(events, "snapshot failed") {
		t.Fatalf("events missing snapshot error: %+v", events)
	}
}

func runBootRunnerThroughEngine(t *testing.T, ctrl *fakeAutomationController) (*Store, RunManifest, RunPaths, []ArchivedEvent) {
	t.Helper()
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	task := validStoreTask()
	task.Schedule = Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)}
	store := newEngineTestStore(t, task)
	runner := &BootRunner{
		build: func(_ context.Context, opts boot.Options) (automationController, error) {
			ctrl.sink = opts.Sink
			if ctrl.sessionPath == "" {
				ctrl.sessionPath = filepath.Join(opts.SessionDir, sessionFileName)
			}
			return ctrl, nil
		},
	}
	engine := newEngineTestEngine(store, "test:runner", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	runs, err := store.ListRuns(task.ID, 1)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1: %+v", len(runs), runs)
	}
	manifest, err := store.ReadRunManifest(task.ID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	paths, err := store.RunPaths(task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("RunPaths: %v", err)
	}
	events, err := readArchivedEvents(paths.Events)
	if err != nil {
		t.Fatalf("readArchivedEvents: %v", err)
	}
	return store, manifest, paths, events
}

func hasArchivedErrorContaining(events []ArchivedEvent, text string) bool {
	for _, ev := range events {
		if strings.Contains(ev.Error, text) {
			return true
		}
	}
	return false
}

func hasArchivedTextContaining(events []ArchivedEvent, text string) bool {
	for _, ev := range events {
		if strings.Contains(ev.Text, text) {
			return true
		}
	}
	return false
}
