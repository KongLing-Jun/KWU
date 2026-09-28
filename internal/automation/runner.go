package automation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/boot"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/provider"
)

type ControllerFactory func(context.Context, boot.Options) (automationController, error)

type automationController interface {
	RunTurn(context.Context, string) error
	Snapshot() error
	SessionPath() string
	History() []provider.Message
	SetToolApprovalMode(string)
	LastUsage() *provider.Usage
	Close()
}

type automationSkillLoader interface {
	RunSkill(input string) (sent string, found bool)
}

// RunRequest contains the already-scheduled, leased, and condition-checked run.
type RunRequest struct {
	Task     Task
	Manifest RunManifest
	RunDir   string
	Trigger  Trigger
	Owner    string
	Sink     event.Sink
}

// RunResult is the runner's persisted outcome.
type RunResult struct {
	Status       RunStatus
	Summary      string
	ErrorCode    RunErrorCode
	ErrorMessage string
	Error        string
	SessionPath  string
	ResultPath   string
	Usage        *provider.Usage
}

// BootRunner creates a fresh background Controller for each automation run.
type BootRunner struct {
	build ControllerFactory
	sink  event.Sink
}

func NewBootRunner(sink event.Sink) *BootRunner {
	return &BootRunner{build: buildController, sink: sink}
}

func buildController(ctx context.Context, opts boot.Options) (automationController, error) {
	return boot.Build(ctx, opts)
}

func (r *BootRunner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	runDir := strings.TrimSpace(req.RunDir)
	if runDir == "" {
		err := errors.New("automation: run dir is required")
		return failedRunResult(err), err
	}
	approvalTimeout, err := parseApprovalTimeout(req.Task.Execution.ApprovalTimeout)
	if err != nil {
		return failedRunResult(err), err
	}
	build := r.build
	if build == nil {
		build = buildController
	}
	sink := req.Sink
	var archive *ArchiveSink
	if sink == nil {
		archive, err = NewArchiveSink(filepath.Join(runDir, eventsFileName), archiveMeta(req), r.sink)
		if err != nil {
			return failedRunResult(err), err
		}
		sink = archive
	}
	if sink == nil {
		sink = event.Discard
	}
	ctrl, err := build(ctx, boot.Options{
		Model:           strings.TrimSpace(req.Task.Prompt.Model),
		WorkspaceRoot:   strings.TrimSpace(req.Task.WorkspaceRoot),
		SessionDir:      filepath.Join(runDir, "session"),
		Sink:            sink,
		RequireKey:      true,
		ApprovalTimeout: approvalTimeout,
	})
	if err != nil {
		if archive != nil {
			err = errors.Join(err, archive.Close())
		}
		return runnerFailedRunResult(err), err
	}
	defer ctrl.Close()

	if pinner, ok := ctrl.(interface{ EnsureSessionPath() }); ok {
		pinner.EnsureSessionPath()
	}
	ctrl.SetToolApprovalMode(string(normalizeApprovalMode(req.Task.Execution.ApprovalMode)))
	assist, assistPath, err := PrepareAutomationAssist(req.Task, runDir, ctrl)
	if err != nil {
		result := runnerFailedRunResult(err)
		result.SessionPath = ctrl.SessionPath()
		result.Usage = ctrl.LastUsage()
		if archive != nil {
			err = errors.Join(err, archive.Close())
		}
		return result, err
	}
	EmitAutomationHelperNotice(sink, req.Task, assistPath)
	runErr := ctrl.RunTurn(ctx, PromptCompiler{}.CompileWithOptions(req.Task, req.Manifest, PromptCompileOptions{AssistPath: assistPath, Assist: assist}))
	snapshotErr := ctrl.Snapshot()
	events := resultEventsFromSink(sink)
	history := ctrl.History()
	extracted := ExtractResult(events, history)
	extractable := hasExtractableResult(events, history)
	resultPath := filepath.Join(runDir, resultFileName)
	classified := classifyRunError(runErr)
	status := classified.Status
	var writeErr error
	if status == RunStatusSucceeded || extractable {
		writeErr = WriteResult(resultPath, extracted)
	}
	for _, e := range []error{runErr, snapshotErr, writeErr} {
		emitRunError(sink, e)
	}
	archiveErr := archiveSinkErr(sink)
	err = errors.Join(runErr, snapshotErr, writeErr, archiveErr)
	if archive != nil {
		closeErr := archive.Close()
		err = errors.Join(err, closeErr)
		archiveErr = errors.Join(archiveErr, closeErr)
	}

	return runResultFromExecution(ctrl, extracted, resultPath, status, extractable, classified, err, snapshotErr, writeErr, archiveErr), err
}

// PrepareAutomationAssist renders required skills for a run and writes assist.md.
func PrepareAutomationAssist(task Task, runDir string, ctrl any) (string, string, error) {
	skills := normalizedSkillNames(task.Prompt.Skills)
	if len(skills) == 0 {
		return "", "", nil
	}
	runDir = strings.TrimSpace(runDir)
	if runDir == "" {
		return "", "", errors.New("automation: run dir is required")
	}
	loader, ok := ctrl.(automationSkillLoader)
	if !ok {
		return "", "", errors.New("automation: controller does not support required skills")
	}
	var b strings.Builder
	writeLine(&b, "# Automation Assist")
	writeLine(&b, "")
	for _, name := range skills {
		input := "/" + name
		if prompt := strings.TrimSpace(task.Prompt.Prompt); prompt != "" {
			input += " " + prompt
		}
		rendered, found := loader.RunSkill(input)
		if !found {
			return "", "", fmt.Errorf("automation: required skill %q is unavailable", name)
		}
		writeLine(&b, "## Skill: "+name)
		writeLine(&b, "")
		writeLine(&b, strings.TrimSpace(rendered))
		writeLine(&b, "")
	}
	assist := strings.TrimSpace(b.String()) + "\n"
	path := filepath.Join(runDir, assistFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, []byte(assist), 0o600); err != nil {
		return "", "", err
	}
	return assist, path, nil
}

// EmitAutomationHelperNotice records which selected helpers affected a run.
func EmitAutomationHelperNotice(sink event.Sink, task Task, assistPath string) {
	if sink == nil {
		return
	}
	skills := normalizedSkillNames(task.Prompt.Skills)
	if len(skills) == 0 {
		return
	}
	parts := make([]string, 0, 2)
	if len(skills) > 0 {
		parts = append(parts, "skills="+strings.Join(skills, ","))
	}
	if assistPath = strings.TrimSpace(assistPath); assistPath != "" {
		parts = append(parts, "assist="+assistPath)
	}
	sink.Emit(event.Event{
		Kind:  event.Notice,
		Level: event.LevelInfo,
		Text:  "automation helpers: " + strings.Join(parts, " "),
	})
}

func normalizedSkillNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimPrefix(strings.TrimSpace(value), "/")
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parseApprovalTimeout(raw string) (time.Duration, error) {
	raw = normalizeApprovalTimeout(raw)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%w %q: %v", ErrInvalidApprovalTimeout, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidApprovalTimeout, raw)
	}
	return d, nil
}

func failedRunResult(err error) RunResult {
	result := classifyRunError(err)
	result.Error = err.Error()
	return result
}

func runResultFromExecution(ctrl automationController, extracted Result, resultPath string, status RunStatus, extractable bool, classified RunResult, err, snapshotErr, writeErr, archiveErr error) RunResult {
	result := RunResult{
		Status:      status,
		SessionPath: ctrl.SessionPath(),
		Usage:       ctrl.LastUsage(),
	}
	if status == RunStatusSucceeded || extractable {
		result.Summary = extracted.Summary
	}
	if writeErr == nil && (status == RunStatusSucceeded || extractable) {
		result.ResultPath = resultPath
	}
	if err == nil {
		return result
	}
	if snapshotErr != nil {
		result.Status = RunStatusFailed
		result.ErrorCode = RunErrorSnapshotFailed
		result.ErrorMessage = snapshotErr.Error()
	} else if writeErr != nil || archiveErr != nil {
		result.Status = RunStatusFailed
		result.ErrorCode = RunErrorArchiveFailed
		result.ErrorMessage = errors.Join(writeErr, archiveErr).Error()
	} else {
		result.ErrorCode = classified.ErrorCode
		result.ErrorMessage = classified.ErrorMessage
	}
	result.Error = err.Error()
	return result
}

func runnerFailedRunResult(err error) RunResult {
	return RunResult{
		Status:       RunStatusFailed,
		ErrorCode:    RunErrorRunnerFailed,
		ErrorMessage: runErrorMessage(err),
		Error:        err.Error(),
	}
}

func statusFromRunErr(err error) RunStatus {
	return classifyRunError(err).Status
}

func classifyRunError(err error) RunResult {
	switch {
	case err == nil:
		return RunResult{Status: RunStatusSucceeded}
	case errors.Is(err, ErrLeaseUnavailable):
		return RunResult{Status: RunStatusSkipped, ErrorCode: RunErrorLeaseUnavailable, ErrorMessage: runErrorMessage(err)}
	case errors.Is(err, context.DeadlineExceeded):
		return RunResult{Status: RunStatusBlocked, ErrorCode: RunErrorApprovalTimeout, ErrorMessage: runErrorMessage(err)}
	case errors.Is(err, context.Canceled):
		return RunResult{Status: RunStatusCanceled, ErrorCode: RunErrorCanceled, ErrorMessage: runErrorMessage(err)}
	default:
		return RunResult{Status: RunStatusFailed, ErrorCode: RunErrorRunnerFailed, ErrorMessage: runErrorMessage(err)}
	}
}

func runErrorMessage(err error) string {
	if explained := control.ExplainError(err); explained != nil {
		return explained.Error()
	}
	return ""
}

func emitRunError(sink event.Sink, err error) {
	if sink == nil || err == nil {
		return
	}
	sink.Emit(event.Event{
		Kind:  event.Notice,
		Level: event.LevelWarn,
		Text:  "automation run error: " + runErrorMessage(err),
		Err:   err,
	})
}

func archiveMeta(req RunRequest) ArchiveEventMeta {
	meta := ArchiveEventMeta{
		TaskID: req.Manifest.TaskID,
		RunID:  req.Manifest.RunID,
	}
	if meta.TaskID == "" {
		meta.TaskID = req.Task.ID
	}
	return meta
}

func archiveSinkErr(sink event.Sink) error {
	if sink == nil {
		return nil
	}
	if s, ok := sink.(interface{ Err() error }); ok {
		return s.Err()
	}
	return nil
}

func resultEventsFromSink(sink event.Sink) []event.Event {
	if sink == nil {
		return nil
	}
	if s, ok := sink.(interface{ LastMessage() string }); ok {
		if message := s.LastMessage(); strings.TrimSpace(message) != "" {
			return []event.Event{{Kind: event.Message, Text: message}}
		}
	}
	return nil
}

func hasExtractableResult(events []event.Event, history []provider.Message) bool {
	for _, ev := range events {
		if ev.Kind == event.Message && strings.TrimSpace(ev.Text) != "" {
			return true
		}
	}
	for _, msg := range history {
		if msg.Role == provider.RoleAssistant && strings.TrimSpace(msg.Content) != "" {
			return true
		}
	}
	return false
}
