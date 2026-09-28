package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"kwu/internal/event"
)

func (s *service) listVisibleRunsPage(ctx context.Context, filter RunFilter, cursor string) (RunPage, error) {
	page, _, err := s.store.indexedRunPage(ctx, filter, cursor, nil)
	return page, err
}

func normalizedRunPageLimit(limit int) (int, error) {
	if limit < 0 || limit > 100 {
		return 0, fmt.Errorf("automation: run page limit must be between 1 and 100")
	}
	if limit == 0 {
		return 20, nil
	}
	return limit, nil
}

func (s *service) requireStore() error {
	if s.store == nil {
		return errors.New("automation: service store is required")
	}
	return nil
}

func (s *service) prepareManualTrigger(ctx context.Context, req TriggerNowRequest) (Task, error) {
	if s.engine == nil {
		return Task{}, errors.New("automation: service engine is required")
	}
	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		return Task{}, ErrTaskNotFound
	}
	if err := s.requireStore(); err != nil {
		return Task{}, err
	}
	task, err := s.store.Task(taskID)
	if err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *service) executeManualTrigger(ctx context.Context, req TriggerNowRequest, task Task) (TriggerNowResponse, error) {
	runID := newRunID(s.now().UTC())
	runID, err := s.engine.trigger(ctx, TriggerRequest{TaskID: task.ID, Reason: req.Reason, RunID: runID})
	response := TriggerNowResponse{TaskID: task.ID, RunID: runID}
	if runID == "" {
		return response, err
	}
	if manifest, readErr := s.store.ReadRunManifest(task.ID, runID); readErr == nil {
		response.Status = manifest.Status
	}
	return response, err
}

func compactRunEventKind(kind event.Kind) bool {
	switch kind {
	case event.Reasoning, event.Text, event.ToolProgress, event.TaskNodeOutput:
		return true
	default:
		return false
	}
}

func appendRunEventText(current, next string) string {
	if current == "" {
		return strings.TrimLeft(next, " \t\r\n")
	}
	if next == "" {
		return current
	}
	trimmedNext := strings.TrimLeft(next, " \t\r\n")
	if trimmedNext == "" || strings.HasSuffix(current, " ") || strings.HasSuffix(current, "\n") || next != trimmedNext || !shouldSpaceRunEventText(current, trimmedNext) {
		return current + next
	}
	return current + " " + trimmedNext
}

func shouldSpaceRunEventText(left, right string) bool {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes) == 0 || len(rightRunes) == 0 {
		return false
	}
	last := leftRunes[len(leftRunes)-1]
	first := rightRunes[0]
	if isASCIIDigit(last) && isASCIIDigit(first) {
		return false
	}
	return (isASCIIWordRune(last) || strings.ContainsRune(".,:;!?", last)) && isASCIIWordRune(first)
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isASCIIWordRune(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func previewRunes(value string, limit int) (string, bool) {
	if limit <= 0 {
		return "", strings.TrimSpace(value) != ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	return string(runes[:limit]), true
}

type runningRunRegistry struct {
	mu         sync.Mutex
	cancel     map[string]context.CancelFunc
	operations map[string]context.CancelFunc
	draining   bool
	drained    chan struct{}
	cleanupErr error
}

func newRunningRunRegistry() *runningRunRegistry {
	return &runningRunRegistry{cancel: map[string]context.CancelFunc{}, operations: map[string]context.CancelFunc{}}
}

func (r *runningRunRegistry) add(taskID, runID string, cancel context.CancelFunc) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.draining {
		return false
	}
	r.cancel[runKey(taskID, runID)] = cancel
	return true
}

func (r *runningRunRegistry) remove(taskID, runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancel, runKey(taskID, runID))
	r.signalDrainedLocked()
}

func (r *runningRunRegistry) cancelRun(taskID, runID string) bool {
	r.mu.Lock()
	cancel := r.cancel[runKey(taskID, runID)]
	r.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (r *runningRunRegistry) active(taskID, runID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancel[runKey(taskID, runID)] != nil
}

func (r *runningRunRegistry) beginDrain() <-chan struct{} {
	r.mu.Lock()
	if r.draining {
		done := r.drained
		r.mu.Unlock()
		return done
	}
	r.draining = true
	r.drained = make(chan struct{})
	cancels := make([]context.CancelFunc, 0, len(r.cancel))
	for _, cancel := range r.cancel {
		cancels = append(cancels, cancel)
	}
	for _, cancel := range r.operations {
		cancels = append(cancels, cancel)
	}
	done := r.drained
	r.signalDrainedLocked()
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return done
}

// Admission and draining share the existing registry mutex. An operation owns
// its registration until its last Store access, including the IPC result read.
func (r *runningRunRegistry) admit(ctx context.Context) (context.Context, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.draining {
		return nil, nil, ErrShuttingDown
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	id := event.NewOperationID()
	r.operations[id] = cancel
	return ctx, func() {
		cancel()
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.operations, id)
		r.signalDrainedLocked()
	}, nil
}

func (r *runningRunRegistry) signalDrainedLocked() {
	if r.draining && len(r.operations) == 0 && len(r.cancel) == 0 {
		select {
		case <-r.drained:
		default:
			close(r.drained)
		}
	}
}

func (r *runningRunRegistry) recordCleanupError(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanupErr = errors.Join(r.cleanupErr, err)
}

func (r *runningRunRegistry) cleanupError() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cleanupErr
}

func runKey(taskID, runID string) string {
	return taskID + "\x00" + runID
}
