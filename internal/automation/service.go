package automation

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"kwu/internal/event"
)

const (
	defaultResultSummaryRunes         = 4096
	defaultRunEventLimit              = 50
	defaultEventTextRunes             = 500
	defaultRunCancellationWaitTimeout = 15 * time.Second
)

// Service is the shared automation API used by desktop, serve, and future hosts.
type Service interface {
	MigrationStatus(context.Context) (HeartbeatMigrationRunResult, error)
	ListTasks(context.Context) ([]TaskView, error)
	SaveTasks(context.Context, SaveTasksRequest) error
	CreateTask(context.Context, CreateTaskRequest) (TaskView, error)
	UpdateTask(context.Context, UpdateTaskRequest) (TaskView, error)
	SetTaskEnabled(context.Context, SetTaskEnabledRequest) (TaskView, error)
	DeleteTask(context.Context, DeleteTaskRequest) error
	ListTemplates(context.Context) ([]TaskTemplate, error)
	SaveTaskTemplate(context.Context, SaveTaskTemplateRequest) (TaskTemplate, error)
	DeleteTaskTemplate(context.Context, DeleteTaskTemplateRequest) error
	RestoreTaskTemplate(context.Context, RestoreTaskTemplateRequest) error
	TriggerNow(context.Context, TriggerNowRequest) (TriggerNowResponse, error)
	ListRuns(context.Context, RunFilter) ([]RunSummary, error)
	ListRunsV2(context.Context, RunFilter, string) (RunPage, error)
	ReadRun(context.Context, string, string) (RunDetail, error)
	RenameRun(context.Context, RenameRunRequest) (RunManifest, error)
	TrashRun(context.Context, TrashRunRequest) (RunManifest, error)
	RestoreRun(context.Context, RestoreRunRequest) (RunManifest, error)
	ListDeliveries(context.Context, string, string) ([]DeliveryAttempt, error)
	RetryDelivery(context.Context, RetryDeliveryRequest) (DeliveryAttempt, error)
	ExportRunResult(context.Context, RunResultExportRequest) (RunResultExport, error)
	CancelRun(context.Context, CancelRunRequest) (CancelRunResponse, error)
	Shutdown(context.Context) error
}

func (s *service) ListDeliveries(ctx context.Context, taskID, runID string) ([]DeliveryAttempt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.requireStore(); err != nil {
		return nil, err
	}
	return s.store.ListDeliveries(strings.TrimSpace(taskID), strings.TrimSpace(runID))
}

func (s *service) RetryDelivery(ctx context.Context, req RetryDeliveryRequest) (DeliveryAttempt, error) {
	if err := ctx.Err(); err != nil {
		return DeliveryAttempt{}, err
	}
	taskID := strings.TrimSpace(req.TaskID)
	runID := strings.TrimSpace(req.RunID)
	if s.engine == nil {
		return DeliveryAttempt{}, errors.New("automation: engine is required")
	}
	return s.engine.RetryDelivery(ctx, taskID, runID, strings.TrimSpace(req.AttemptID))
}

// TaskView is the host-facing task DTO. Task contains no provider keys or tokens.
type TaskView struct {
	Task
	NextRunAt *time.Time `json:"nextRunAt,omitempty"`
}

func (view TaskView) MarshalJSON() ([]byte, error) {
	encodedTask, err := json.Marshal(view.Task)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encodedTask, &fields); err != nil {
		return nil, err
	}
	delete(fields, "nextRunAt")
	if view.NextRunAt != nil {
		encodedNextRunAt, err := json.Marshal(view.NextRunAt)
		if err != nil {
			return nil, err
		}
		fields["nextRunAt"] = encodedNextRunAt
	}
	return json.Marshal(fields)
}

type SaveTasksRequest struct {
	Tasks []Task `json:"tasks"`
}

type RetryDeliveryRequest struct {
	TaskID    string `json:"taskId"`
	RunID     string `json:"runId"`
	AttemptID string `json:"attemptId"`
}

type CreateTaskRequest struct {
	Task Task `json:"task"`
}

type UpdateTaskRequest struct {
	Task             Task  `json:"task"`
	ExpectedRevision int64 `json:"expectedRevision"`
}

type SetTaskEnabledRequest struct {
	ID               string `json:"id"`
	Enabled          bool   `json:"enabled"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type DeleteTaskRequest struct {
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type SaveTaskTemplateRequest struct {
	Task Task `json:"task"`
}

type DeleteTaskTemplateRequest struct {
	ID string `json:"id"`
}

type RestoreTaskTemplateRequest struct {
	ID string `json:"id"`
}

type TriggerNowRequest struct {
	TaskID string `json:"taskId"`
	Reason string `json:"reason,omitempty"`
}

type TriggerNowResponse struct {
	TaskID string    `json:"taskId"`
	RunID  string    `json:"runId"`
	Status RunStatus `json:"status,omitempty"`
}

type RunFilter struct {
	TaskID  string  `json:"taskId,omitempty"`
	Scope   string  `json:"scope,omitempty"`
	Trigger Trigger `json:"trigger,omitempty"`
	Search  string  `json:"search,omitempty"`
	Limit   int     `json:"limit,omitempty"`
}

type RunTaskFilterOption struct {
	TaskID   string `json:"taskId"`
	TaskName string `json:"taskName"`
}

type RunPage struct {
	TaskFilterOptions []RunTaskFilterOption `json:"taskFilterOptions"`
	Items             []RunSummary          `json:"items"`
	NextCursor        string                `json:"nextCursor,omitempty"`
	HasMore           bool                  `json:"hasMore"`
}

type RunDetail struct {
	Manifest               RunManifest             `json:"manifest"`
	Deliveries             []DeliveryAttempt       `json:"deliveries,omitempty"`
	ResultSummary          string                  `json:"resultSummary,omitempty"`
	ResultTruncated        bool                    `json:"resultTruncated,omitempty"`
	AvailableResultFormats []RunResultExportFormat `json:"availableResultFormats,omitempty"`
	Events                 []RunEventSummary       `json:"events,omitempty"`
}

type RunResultExportRequest struct {
	TaskID string `json:"taskId"`
	RunID  string `json:"runId"`
	Format string `json:"format"`
}

type RunResultExportFormat struct {
	Format    string `json:"format"`
	Label     string `json:"label"`
	Extension string `json:"extension"`
	MimeType  string `json:"mimeType"`
}

type RunResultExport struct {
	Filename      string `json:"filename"`
	MimeType      string `json:"mimeType"`
	Payload       string `json:"payload"`
	Base64Encoded bool   `json:"base64Encoded"`
}

type RunEventSummary struct {
	Time   time.Time   `json:"time"`
	Kind   event.Kind  `json:"kind"`
	Level  event.Level `json:"level"`
	Text   string      `json:"text,omitempty"`
	Status RunStatus   `json:"status,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type CancelRunRequest struct {
	TaskID string `json:"taskId"`
	RunID  string `json:"runId"`
}

type CancelRunResponse struct {
	TaskID   string `json:"taskId"`
	RunID    string `json:"runId"`
	Canceled bool   `json:"canceled"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

type TaskQuotaService interface {
	TaskQuota(context.Context) (TaskQuotaView, error)
}

type service struct {
	store                      *Store
	engine                     *Engine
	taskQuota                  *TaskQuotaPolicy
	running                    *runningRunRegistry
	now                        func() time.Time
	migrationMu                sync.Mutex
	heartbeatMigration         HeartbeatMigrationRunResult
	runCancellationWaitTimeout time.Duration
}

func NewService(store *Store, engine *Engine) Service {
	svc := newService(store, engine)
	_, _ = svc.MigrateHeartbeatIfNeeded(context.Background())
	return svc
}

func NewServiceWithTaskQuota(store *Store, engine *Engine, resolver TaskQuotaResolver) Service {
	svc := newService(store, engine)
	svc.taskQuota = NewTaskQuotaPolicy(resolver)
	if engine != nil {
		engine.taskQuota = svc.taskQuota
	}
	_, _ = svc.MigrateHeartbeatIfNeeded(context.Background())
	return svc
}

func newService(store *Store, engine *Engine) *service {
	if store == nil && engine != nil {
		store = engine.store
	}
	running := newRunningRunRegistry()
	if engine != nil && engine.running != nil {
		running = engine.running
	}
	return &service{
		store:                      store,
		engine:                     engine,
		running:                    running,
		now:                        time.Now,
		runCancellationWaitTimeout: defaultRunCancellationWaitTimeout,
	}
}

func (s *service) ListTasks(ctx context.Context) ([]TaskView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.requireStore(); err != nil {
		return nil, err
	}
	tasks, err := s.store.LoadTasks()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left := strings.ToLower(tasks[i].Name)
		right := strings.ToLower(tasks[j].Name)
		if left == right {
			return tasks[i].ID < tasks[j].ID
		}
		return left < right
	})
	views := make([]TaskView, 0, len(tasks))
	for _, task := range tasks {
		views = append(views, taskViewWithNextRun(task, s.now().UTC()))
	}
	return views, nil
}

func taskViewWithNextRun(task Task, now time.Time) TaskView {
	view := TaskView{Task: task}
	next, err := PreviewNextRuns(task, now, 1)
	if err == nil && len(next) == 1 {
		runAt := next[0].UTC()
		view.NextRunAt = &runAt
	}
	return view
}

func (s *service) SaveTasks(ctx context.Context, req SaveTasksRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.requireStore(); err != nil {
		return err
	}
	existing, err := s.store.LoadTasks()
	if err != nil {
		return err
	}
	tasks, err := normalizeTasksForSave(req.Tasks, existing, s.now().UTC())
	if err != nil {
		return err
	}
	if err := s.validateBulkTaskQuota(ctx, existing, tasks); err != nil {
		return err
	}
	return s.store.SaveTasks(tasks)
}

func (s *service) CreateTask(ctx context.Context, req CreateTaskRequest) (TaskView, error) {
	candidate, existing, err := s.prepareCreateTask(ctx, req.Task)
	if err != nil {
		return TaskView{}, err
	}
	if err := s.validateCreateTaskQuota(ctx, existing, candidate); err != nil {
		return TaskView{}, err
	}
	candidate.CreatedAt = s.now().UTC()
	candidate.UpdatedAt = candidate.CreatedAt
	created, err := s.store.CreateTask(candidate)
	if err != nil {
		return TaskView{}, err
	}
	return taskViewWithNextRun(created, s.now().UTC()), nil
}

func (s *service) UpdateTask(ctx context.Context, req UpdateTaskRequest) (TaskView, error) {
	if err := ctx.Err(); err != nil {
		return TaskView{}, err
	}
	if err := s.requireStore(); err != nil {
		return TaskView{}, err
	}
	if err := ValidateSchedule(req.Task.Schedule); err != nil {
		return TaskView{}, err
	}
	updated, err := s.store.UpdateTask(req.Task, req.ExpectedRevision)
	if err != nil {
		return TaskView{}, err
	}
	return taskViewWithNextRun(updated, s.now().UTC()), nil
}

func (s *service) SetTaskEnabled(ctx context.Context, req SetTaskEnabledRequest) (TaskView, error) {
	if err := ctx.Err(); err != nil {
		return TaskView{}, err
	}
	if err := s.requireStore(); err != nil {
		return TaskView{}, err
	}
	updated, err := s.store.SetTaskEnabled(req.ID, req.Enabled, req.ExpectedRevision)
	if err != nil {
		return TaskView{}, err
	}
	return taskViewWithNextRun(updated, s.now().UTC()), nil
}

func (s *service) DeleteTask(ctx context.Context, req DeleteTaskRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.requireStore(); err != nil {
		return err
	}
	return s.store.DeleteTask(req.ID, req.ExpectedRevision)
}

func normalizeTasksForSave(requested, existing []Task, now time.Time) ([]Task, error) {
	byID := make(map[string]Task, len(existing))
	for _, task := range existing {
		byID[task.ID] = task
	}
	tasks := make([]Task, 0, len(requested))
	requestedIDs := make(map[string]bool, len(requested))
	for i, task := range requested {
		normalized, err := normalizeTaskForBulkSave(task, byID, now)
		if err != nil {
			return nil, fmt.Errorf("task %d: %w", i, err)
		}
		requestedIDs[normalized.ID] = true
		tasks = append(tasks, normalized)
	}
	if err := rejectBulkDeletes(existing, requestedIDs); err != nil {
		return nil, err
	}
	return tasks, nil
}

func normalizeTaskForBulkSave(task Task, existing map[string]Task, now time.Time) (Task, error) {
	normalized, err := NormalizeTask(task)
	if err != nil {
		return Task{}, err
	}
	if err := ValidateSchedule(normalized.Schedule); err != nil {
		return Task{}, err
	}
	if prev, ok := existing[normalized.ID]; ok {
		if task.Revision != prev.Revision {
			return Task{}, taskRevisionConflict(prev.ID, task.Revision, prev.Revision)
		}
		normalized.Revision = prev.Revision + 1
		normalized.CreatedAt, normalized.LastRunAt, normalized.LastRunID = prev.CreatedAt, prev.LastRunAt, prev.LastRunID
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = now
	}
	normalized.UpdatedAt = now
	return normalized, nil
}

func rejectBulkDeletes(existing []Task, requestedIDs map[string]bool) error {
	for _, prev := range existing {
		if !requestedIDs[prev.ID] {
			return fmt.Errorf("%w: bulk save cannot delete task %s; use revisioned delete", ErrTaskRevisionConflict, prev.ID)
		}
	}
	return nil
}

func (s *service) TaskQuota(ctx context.Context) (TaskQuotaView, error) {
	if err := ctx.Err(); err != nil {
		return TaskQuotaView{}, err
	}
	if err := s.requireStore(); err != nil {
		return TaskQuotaView{}, err
	}
	tasks, err := s.store.LoadTasks()
	if err != nil {
		return TaskQuotaView{}, err
	}
	if s.taskQuota == nil {
		return TaskQuotaView{}, ErrTaskQuotaUnavailable
	}
	return s.taskQuota.View(ctx, tasks)
}

func (s *service) ListTemplates(ctx context.Context) ([]TaskTemplate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.requireStore(); err != nil {
		return nil, err
	}
	deletedTemplates, err := s.store.DeletedTaskTemplateIDs()
	if err != nil {
		return nil, err
	}
	defaultTemplates := DefaultTaskTemplates()
	templates := make([]TaskTemplate, 0, len(defaultTemplates))
	builtinIDs := make(map[string]bool, len(defaultTemplates))
	for _, template := range defaultTemplates {
		builtinIDs[template.ID] = true
		if !deletedTemplates[template.ID] {
			templates = append(templates, template)
		}
	}
	userTemplates, err := s.store.LoadTaskTemplates()
	if err != nil {
		return nil, err
	}
	for _, template := range userTemplates {
		if !builtinIDs[template.ID] {
			templates = append(templates, template)
		}
	}
	return templates, nil
}

func (s *service) SaveTaskTemplate(ctx context.Context, req SaveTaskTemplateRequest) (TaskTemplate, error) {
	if err := ctx.Err(); err != nil {
		return TaskTemplate{}, err
	}
	if err := s.requireStore(); err != nil {
		return TaskTemplate{}, err
	}
	task, err := NormalizeTask(req.Task)
	if err != nil {
		return TaskTemplate{}, err
	}
	template := TaskTemplateFromTask(task)
	for _, builtin := range DefaultTaskTemplates() {
		if builtin.ID == template.ID {
			return TaskTemplate{}, fmt.Errorf("%w: template id %q conflicts with built-in template", ErrInvalidTask, template.ID)
		}
	}
	return s.store.SaveTaskTemplate(template)
}

func (s *service) DeleteTaskTemplate(ctx context.Context, req DeleteTaskTemplateRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.requireStore(); err != nil {
		return err
	}
	return s.store.DeleteTaskTemplate(req.ID)
}

func (s *service) RestoreTaskTemplate(ctx context.Context, req RestoreTaskTemplateRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.requireStore(); err != nil {
		return err
	}
	return s.store.RestoreTaskTemplate(req.ID)
}

func (s *service) TriggerNow(ctx context.Context, req TriggerNowRequest) (TriggerNowResponse, error) {
	ctx, finish, err := s.running.admit(ctx)
	if err != nil {
		return TriggerNowResponse{}, err
	}
	defer finish()
	task, err := s.prepareManualTrigger(ctx, req)
	if err != nil {
		return TriggerNowResponse{}, err
	}
	return s.executeManualTrigger(ctx, req, task)
}

func (s *service) ListRuns(ctx context.Context, filter RunFilter) ([]RunSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.requireStore(); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(filter.TaskID)
	runs, err := s.store.ListRuns(taskID, filter.Limit)
	if err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *service) ListRunsV2(ctx context.Context, filter RunFilter, cursor string) (RunPage, error) {
	if err := ctx.Err(); err != nil {
		return RunPage{}, err
	}
	if err := s.requireStore(); err != nil {
		return RunPage{}, err
	}
	filter, err := normalizeRunFilter(filter)
	if err != nil {
		return RunPage{}, err
	}
	return s.listVisibleRunsPage(ctx, filter, cursor)
}

func (s *service) ReadRun(ctx context.Context, taskID, runID string) (RunDetail, error) {
	if err := ctx.Err(); err != nil {
		return RunDetail{}, err
	}
	if err := s.requireStore(); err != nil {
		return RunDetail{}, err
	}
	manifest, err := s.store.ReadRunManifest(taskID, runID)
	if err != nil {
		return RunDetail{}, err
	}
	detail := RunDetail{Manifest: manifest}
	if result, err := s.store.ReadResult(taskID, runID); err == nil {
		detail.ResultSummary, detail.ResultTruncated = previewRunes(result, defaultResultSummaryRunes)
		detail.AvailableResultFormats = availableRunResultExportFormats(result)
	} else if !errors.Is(err, os.ErrNotExist) {
		return RunDetail{}, err
	}
	paths, err := s.store.RunPaths(taskID, runID)
	if err != nil {
		return RunDetail{}, err
	}
	events, err := readRunEventSummaries(paths.Events, defaultRunEventLimit)
	if err != nil {
		return RunDetail{}, err
	}
	detail.Events = events
	detail.Deliveries, err = s.store.ListDeliveries(taskID, runID)
	if err != nil {
		return RunDetail{}, err
	}
	return detail, nil
}

func (s *service) CancelRun(ctx context.Context, req CancelRunRequest) (CancelRunResponse, error) {
	if err := ctx.Err(); err != nil {
		return CancelRunResponse{}, err
	}
	taskID := strings.TrimSpace(req.TaskID)
	runID := strings.TrimSpace(req.RunID)
	if taskID == "" || runID == "" {
		return CancelRunResponse{}, errors.New("automation: cancel task id and run id are required")
	}
	if s.running.cancelRun(taskID, runID) {
		return CancelRunResponse{
			TaskID:   taskID,
			RunID:    runID,
			Canceled: true,
			Status:   string(RunStatusCanceled),
			Reason:   "cancel requested",
		}, nil
	}
	return CancelRunResponse{
		TaskID: taskID,
		RunID:  runID,
		Status: "not_cancelable",
		Reason: "run is not running in this process",
	}, nil
}

func (s *service) Shutdown(ctx context.Context) error {
	done := s.running.beginDrain()
	if s.engine != nil {
		if err := s.engine.stop(ctx); err != nil {
			return &ShutdownError{Err: err}
		}
	}
	select {
	case <-done:
		if err := s.running.cleanupError(); err != nil {
			return &ShutdownError{Drained: true, Err: err}
		}
		return nil
	case <-ctx.Done():
		return &ShutdownError{Err: ctx.Err()}
	}
}

// ShutdownError distinguishes a cleanup failure from operations still in flight.
type ShutdownError struct {
	Drained bool
	Err     error
}

func (e *ShutdownError) Error() string {
	return fmt.Sprintf("automation: shutdown (drained=%t): %v", e.Drained, e.Err)
}
func (e *ShutdownError) Unwrap() error { return e.Err }

func readRunEventSummaries(path string, limit int) ([]RunEventSummary, error) {
	if limit <= 0 {
		limit = defaultRunEventLimit
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read run events: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	events := make([]RunEventSummary, 0, limit)
	var stream *RunEventSummary
	flushStream := func() {
		if stream == nil {
			return
		}
		stream.Text, _ = previewRunes(stream.Text, defaultEventTextRunes)
		if len(events) < limit {
			events = append(events, *stream)
		}
		stream = nil
	}
	for scanner.Scan() {
		var raw struct {
			Time   time.Time   `json:"time"`
			Kind   event.Kind  `json:"kind"`
			Level  event.Level `json:"level"`
			Text   string      `json:"text"`
			Error  string      `json:"error"`
			Status RunStatus   `json:"status"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			return nil, fmt.Errorf("read run events: decode: %w", err)
		}
		next := RunEventSummary{Time: raw.Time, Kind: raw.Kind, Level: raw.Level, Status: raw.Status, Error: raw.Error}
		if compactRunEventKind(raw.Kind) && raw.Status == "" && raw.Error == "" {
			if stream != nil && stream.Kind == raw.Kind && stream.Level == raw.Level {
				stream.Text = appendRunEventText(stream.Text, raw.Text)
				continue
			}
			flushStream()
			next.Text = raw.Text
			stream = &next
			continue
		}
		flushStream()
		next.Text, _ = previewRunes(raw.Text, defaultEventTextRunes)
		if len(events) < limit {
			events = append(events, next)
		}
	}
	flushStream()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read run events: %w", err)
	}
	return events, nil
}
