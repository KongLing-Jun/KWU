package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/config"
	"kwu/internal/control"
)

func TestAutomationRoutes(t *testing.T) {
	fake := &fakeAutomationService{
		tasks: []automation.TaskView{{Task: automation.Task{ID: "task-1", Name: "Task 1"}}},
		runs:  []automation.RunSummary{{TaskID: "task-1", RunID: "run-1", Status: automation.RunStatusSucceeded}},
		detail: automation.RunDetail{Manifest: automation.RunManifest{
			TaskID: "task-1",
			RunID:  "run-1",
			Status: automation.RunStatusSucceeded,
		}},
		trigger: automation.TriggerNowResponse{TaskID: "task-1", RunID: "run-1", Status: automation.RunStatusSucceeded},
		cancel:  automation.CancelRunResponse{TaskID: "task-1", RunID: "run-1", Canceled: true, Status: string(automation.RunStatusCanceled)},
		migration: automation.HeartbeatMigrationRunResult{
			Status:       automation.HeartbeatMigrationStatusFailed,
			ErrorSummary: "invalid legacy heartbeat config",
		},
	}
	srv := newAutomationTestServer(t, fake)

	var tasks []automation.TaskView
	getJSON(t, srv.URL+"/automation/tasks", &tasks)
	if len(tasks) != 1 || tasks[0].ID != "task-1" {
		t.Fatalf("tasks = %+v, want fake task", tasks)
	}

	resp := postJSON(t, srv.URL+"/automation/tasks", `{"tasks":[{"id":"task-2","name":"Task 2","schedule":{"kind":"interval","every":"30m"},"prompt":{"prompt":"run"}}]}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("save status = %d, want 204", resp.StatusCode)
	}
	if len(fake.savedTasks) != 1 || fake.savedTasks[0].ID != "task-2" {
		t.Fatalf("saved tasks = %+v, want task-2", fake.savedTasks)
	}

	var trigger automation.TriggerNowResponse
	postAndDecode(t, srv.URL+"/automation/tasks/task-1/trigger", `{}`, &trigger)
	if trigger.RunID != "run-1" || fake.triggerReq.Reason != "serve manual trigger" {
		t.Fatalf("trigger = %+v, req = %+v", trigger, fake.triggerReq)
	}

	var runs []automation.RunSummary
	getJSON(t, srv.URL+"/automation/runs?taskId=task-1&limit=5", &runs)
	if len(runs) != 1 || runs[0].RunID != "run-1" || fake.runFilter.Limit != 5 {
		t.Fatalf("runs = %+v, filter = %+v", runs, fake.runFilter)
	}

	var detail automation.RunDetail
	getJSON(t, srv.URL+"/automation/runs/task-1/run-1", &detail)
	if detail.Manifest.RunID != "run-1" || fake.readTaskID != "task-1" {
		t.Fatalf("detail = %+v, read task = %q", detail, fake.readTaskID)
	}

	var cancel automation.CancelRunResponse
	postAndDecode(t, srv.URL+"/automation/runs/task-1/run-1/cancel", `{}`, &cancel)
	if !cancel.Canceled || fake.cancelReq.RunID != "run-1" {
		t.Fatalf("cancel = %+v, req = %+v", cancel, fake.cancelReq)
	}

	var migration automation.HeartbeatMigrationRunResult
	getJSON(t, srv.URL+"/automation/migration/heartbeat", &migration)
	if migration.Status != automation.HeartbeatMigrationStatusFailed || migration.ErrorSummary == "" {
		t.Fatalf("migration = %+v, want visible failed migration status", migration)
	}
}

func TestAutomationDisabled(t *testing.T) {
	srv := newAutomationTestServer(t, nil)

	resp, err := http.Get(srv.URL + "/automation/tasks")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("disabled status = %d, want 503", resp.StatusCode)
	}
}

func TestAutomationPostRequiresJSON(t *testing.T) {
	fake := &fakeAutomationService{}
	srv := newAutomationTestServer(t, fake)

	resp, err := http.Post(srv.URL+"/automation/tasks", "text/plain", strings.NewReader(`{"tasks":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", resp.StatusCode)
	}
	if fake.saveCalls != 0 {
		t.Fatalf("SaveTasks called %d times, want 0", fake.saveCalls)
	}
}

func TestAutomationServiceDoesNotChangeSessionRoutes(t *testing.T) {
	bc := NewBroadcaster()
	got := make(chan string, 1)
	ctrl := control.New(control.Options{Runner: fakeRunner{got: got}, Sink: bc})
	server := New(ctrl, bc, config.ServeConfig{})
	automationSvc := &countingAutomationService{}
	server.SetAutomationService(automationSvc)
	handler := server.Handler()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	resp := postJSON(t, srv.URL+"/submit", `{"input":"hello"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("submit status = %d, want 202", resp.StatusCode)
	}
	select {
	case gotInput := <-got:
		if gotInput != "hello" {
			t.Fatalf("runner input = %q, want hello", gotInput)
		}
	case <-time.After(time.Second):
		t.Fatal("runner never received submit input")
	}

	for _, path := range []string{"/history", "/status"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, resp.StatusCode)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("/events status = %d, want 200", rec.Code)
	}
	if calls := automationSvc.calls.Load(); calls != 0 {
		t.Fatalf("automation service calls = %d, want 0 for session routes", calls)
	}
}

func TestAutomationRoutesReportRealMigrationFailure(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("KWU_STATE_HOME", stateDir)
	if err := os.WriteFile(filepath.Join(stateDir, "heartbeat-tasks.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("WriteFile legacy heartbeat config: %v", err)
	}
	svc := automation.NewService(automation.NewStore(filepath.Join(stateDir, "automation")), nil)
	srv := newAutomationTestServer(t, svc)

	var migration automation.HeartbeatMigrationRunResult
	getJSON(t, srv.URL+"/automation/migration/heartbeat", &migration)
	if migration.Status != automation.HeartbeatMigrationStatusFailed || migration.ErrorSummary == "" {
		t.Fatalf("migration = %+v, want visible failed migration status", migration)
	}

	var tasks []automation.TaskView
	getJSON(t, srv.URL+"/automation/tasks", &tasks)
	if len(tasks) != 0 {
		t.Fatalf("tasks = %+v, want service usable with no migrated tasks", tasks)
	}
}

func newAutomationTestServer(t *testing.T, svc automation.Service) *httptest.Server {
	t.Helper()
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	server := New(ctrl, bc, config.ServeConfig{})
	server.SetAutomationService(svc)
	srv := httptest.NewServer(server.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func getJSON(t *testing.T, url string, out any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func postAndDecode(t *testing.T, url, body string, out any) {
	t.Helper()
	resp := postJSON(t, url, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s status = %d, want 200", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

type fakeAutomationService struct {
	tasks []automation.TaskView
	runs  []automation.RunSummary

	trigger   automation.TriggerNowResponse
	detail    automation.RunDetail
	cancel    automation.CancelRunResponse
	migration automation.HeartbeatMigrationRunResult

	savedTasks []automation.Task
	saveCalls  int
	triggerReq automation.TriggerNowRequest
	runFilter  automation.RunFilter
	readTaskID string
	readRunID  string
	cancelReq  automation.CancelRunRequest
}

type countingAutomationService struct {
	calls atomic.Int32
}

func (s *countingAutomationService) record() {
	s.calls.Add(1)
}

func (s *countingAutomationService) MigrationStatus(context.Context) (automation.HeartbeatMigrationRunResult, error) {
	s.record()
	return automation.HeartbeatMigrationRunResult{}, nil
}

func (s *countingAutomationService) ListTasks(context.Context) ([]automation.TaskView, error) {
	s.record()
	return nil, nil
}

func (s *countingAutomationService) SaveTasks(context.Context, automation.SaveTasksRequest) error {
	s.record()
	return nil
}

func (s *countingAutomationService) CreateTask(context.Context, automation.CreateTaskRequest) (automation.TaskView, error) {
	s.record()
	return automation.TaskView{}, nil
}
func (s *countingAutomationService) UpdateTask(context.Context, automation.UpdateTaskRequest) (automation.TaskView, error) {
	s.record()
	return automation.TaskView{}, nil
}
func (s *countingAutomationService) SetTaskEnabled(context.Context, automation.SetTaskEnabledRequest) (automation.TaskView, error) {
	s.record()
	return automation.TaskView{}, nil
}
func (s *countingAutomationService) DeleteTask(context.Context, automation.DeleteTaskRequest) error {
	s.record()
	return nil
}

func (s *countingAutomationService) ListTemplates(context.Context) ([]automation.TaskTemplate, error) {
	s.record()
	return nil, nil
}

func (s *countingAutomationService) SaveTaskTemplate(context.Context, automation.SaveTaskTemplateRequest) (automation.TaskTemplate, error) {
	s.record()
	return automation.TaskTemplate{}, nil
}

func (s *countingAutomationService) DeleteTaskTemplate(context.Context, automation.DeleteTaskTemplateRequest) error {
	s.record()
	return nil
}

func (s *countingAutomationService) RestoreTaskTemplate(context.Context, automation.RestoreTaskTemplateRequest) error {
	s.record()
	return nil
}

func (s *countingAutomationService) TriggerNow(context.Context, automation.TriggerNowRequest) (automation.TriggerNowResponse, error) {
	s.record()
	return automation.TriggerNowResponse{}, nil
}

func (s *countingAutomationService) ListRuns(context.Context, automation.RunFilter) ([]automation.RunSummary, error) {
	s.record()
	return nil, nil
}

func (s *countingAutomationService) ListRunsV2(context.Context, automation.RunFilter, string) (automation.RunPage, error) {
	s.record()
	return automation.RunPage{}, nil
}

func (s *countingAutomationService) ReadRun(context.Context, string, string) (automation.RunDetail, error) {
	s.record()
	return automation.RunDetail{}, nil
}
func (s *countingAutomationService) RenameRun(context.Context, automation.RenameRunRequest) (automation.RunManifest, error) {
	s.record()
	return automation.RunManifest{}, nil
}
func (s *countingAutomationService) TrashRun(context.Context, automation.TrashRunRequest) (automation.RunManifest, error) {
	s.record()
	return automation.RunManifest{}, nil
}
func (s *countingAutomationService) RestoreRun(context.Context, automation.RestoreRunRequest) (automation.RunManifest, error) {
	s.record()
	return automation.RunManifest{}, nil
}

func (s *countingAutomationService) ListDeliveries(context.Context, string, string) ([]automation.DeliveryAttempt, error) {
	s.record()
	return nil, nil
}
func (s *countingAutomationService) RetryDelivery(context.Context, automation.RetryDeliveryRequest) (automation.DeliveryAttempt, error) {
	s.record()
	return automation.DeliveryAttempt{}, nil
}

func (s *countingAutomationService) ExportRunResult(context.Context, automation.RunResultExportRequest) (automation.RunResultExport, error) {
	s.record()
	return automation.RunResultExport{}, nil
}

func (s *countingAutomationService) CancelRun(context.Context, automation.CancelRunRequest) (automation.CancelRunResponse, error) {
	s.record()
	return automation.CancelRunResponse{}, nil
}

func (s *countingAutomationService) Shutdown(context.Context) error {
	s.record()
	return nil
}

func (s *fakeAutomationService) MigrationStatus(context.Context) (automation.HeartbeatMigrationRunResult, error) {
	return s.migration, nil
}

func (s *fakeAutomationService) ListTasks(context.Context) ([]automation.TaskView, error) {
	return s.tasks, nil
}

func (s *fakeAutomationService) SaveTasks(_ context.Context, req automation.SaveTasksRequest) error {
	s.saveCalls++
	s.savedTasks = req.Tasks
	return nil
}

func (s *fakeAutomationService) CreateTask(_ context.Context, req automation.CreateTaskRequest) (automation.TaskView, error) {
	return automation.TaskView{Task: req.Task}, nil
}
func (s *fakeAutomationService) UpdateTask(_ context.Context, req automation.UpdateTaskRequest) (automation.TaskView, error) {
	return automation.TaskView{Task: req.Task}, nil
}
func (s *fakeAutomationService) SetTaskEnabled(context.Context, automation.SetTaskEnabledRequest) (automation.TaskView, error) {
	return automation.TaskView{}, nil
}
func (s *fakeAutomationService) DeleteTask(context.Context, automation.DeleteTaskRequest) error {
	return nil
}

func (s *fakeAutomationService) ListTemplates(context.Context) ([]automation.TaskTemplate, error) {
	return nil, nil
}

func (s *fakeAutomationService) SaveTaskTemplate(context.Context, automation.SaveTaskTemplateRequest) (automation.TaskTemplate, error) {
	return automation.TaskTemplate{}, nil
}

func (s *fakeAutomationService) DeleteTaskTemplate(context.Context, automation.DeleteTaskTemplateRequest) error {
	return nil
}

func (s *fakeAutomationService) RestoreTaskTemplate(context.Context, automation.RestoreTaskTemplateRequest) error {
	return nil
}

func (s *fakeAutomationService) TriggerNow(_ context.Context, req automation.TriggerNowRequest) (automation.TriggerNowResponse, error) {
	s.triggerReq = req
	return s.trigger, nil
}

func (s *fakeAutomationService) ListRuns(_ context.Context, filter automation.RunFilter) ([]automation.RunSummary, error) {
	s.runFilter = filter
	return s.runs, nil
}

func (s *fakeAutomationService) ListRunsV2(_ context.Context, filter automation.RunFilter, _ string) (automation.RunPage, error) {
	s.runFilter = filter
	return automation.RunPage{Items: s.runs}, nil
}

func (s *fakeAutomationService) ReadRun(_ context.Context, taskID, runID string) (automation.RunDetail, error) {
	s.readTaskID = taskID
	s.readRunID = runID
	return s.detail, nil
}
func (s *fakeAutomationService) RenameRun(context.Context, automation.RenameRunRequest) (automation.RunManifest, error) {
	return automation.RunManifest{}, nil
}
func (s *fakeAutomationService) TrashRun(context.Context, automation.TrashRunRequest) (automation.RunManifest, error) {
	return automation.RunManifest{}, nil
}
func (s *fakeAutomationService) RestoreRun(context.Context, automation.RestoreRunRequest) (automation.RunManifest, error) {
	return automation.RunManifest{}, nil
}

func (s *fakeAutomationService) ListDeliveries(context.Context, string, string) ([]automation.DeliveryAttempt, error) {
	return nil, nil
}
func (s *fakeAutomationService) RetryDelivery(context.Context, automation.RetryDeliveryRequest) (automation.DeliveryAttempt, error) {
	return automation.DeliveryAttempt{}, nil
}

func (s *fakeAutomationService) ExportRunResult(context.Context, automation.RunResultExportRequest) (automation.RunResultExport, error) {
	return automation.RunResultExport{}, nil
}

func (s *fakeAutomationService) CancelRun(_ context.Context, req automation.CancelRunRequest) (automation.CancelRunResponse, error) {
	s.cancelReq = req
	return s.cancel, nil
}

func (s *fakeAutomationService) Shutdown(context.Context) error {
	return nil
}
