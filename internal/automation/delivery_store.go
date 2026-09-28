package automation

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const deliveryEventSchemaVersion = 1

type deliveryEvent struct {
	SchemaVersion int             `json:"schemaVersion"`
	RecordedAt    time.Time       `json:"recordedAt"`
	Attempt       DeliveryAttempt `json:"attempt"`
}

// DeliveryStore is an append-only JSONL projection of delivery attempts. The
// in-memory indexes are rebuilt from the log on open, so no mutable sidecar can
// diverge from delivery history.
type DeliveryStore struct {
	mu            sync.Mutex
	path          string
	attempts      map[string]DeliveryAttempt
	byIdempotency map[string]string
	now           func() time.Time
}

func OpenDeliveryStore(path string) (*DeliveryStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("%w: delivery store path is required", ErrInvalidDelivery)
	}
	store := &DeliveryStore{
		path:          path,
		attempts:      make(map[string]DeliveryAttempt),
		byIdempotency: make(map[string]string),
		now:           time.Now,
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// Enqueue appends exactly one queued record for an idempotency key. Repeated
// concurrent calls return the existing attempt without growing the log.
func (s *DeliveryStore) Enqueue(input DeliveryAttemptInput, now time.Time) (DeliveryAttempt, bool, error) {
	if s == nil {
		return DeliveryAttempt{}, false, fmt.Errorf("%w: nil delivery store", ErrInvalidDelivery)
	}
	attempt, err := NewDeliveryAttempt(input, now)
	if err != nil {
		return DeliveryAttempt{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byIdempotency[attempt.IdempotencyKey]; ok {
		existing := s.attempts[id]
		if !sameDeliveryIntent(existing, attempt) {
			return DeliveryAttempt{}, false, fmt.Errorf("%w: idempotency key %q was used for another delivery", ErrInvalidDelivery, attempt.IdempotencyKey)
		}
		return existing, false, nil
	}
	if _, exists := s.attempts[attempt.ID]; exists {
		return DeliveryAttempt{}, false, fmt.Errorf("%w: duplicate delivery attempt id %q", ErrInvalidDelivery, attempt.ID)
	}
	if err := s.appendLocked(attempt); err != nil {
		return DeliveryAttempt{}, false, err
	}
	s.rememberLocked(attempt)
	return attempt, true, nil
}

// Append records a valid next state. Existing lines are never rewritten.
func (s *DeliveryStore) Append(next DeliveryAttempt) error {
	if s == nil {
		return fmt.Errorf("%w: nil delivery store", ErrInvalidDelivery)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.attempts[next.ID]
	if !ok {
		return fmt.Errorf("%w: delivery attempt %q was not queued", ErrInvalidDelivery, next.ID)
	}
	if !sameDeliveryIdentity(current, next) {
		return fmt.Errorf("%w: delivery attempt identity is immutable", ErrInvalidDelivery)
	}
	if err := validateStoredDeliveryTransition(current, next); err != nil {
		return err
	}
	if err := s.appendLocked(next); err != nil {
		return err
	}
	s.rememberLocked(next)
	return nil
}

func (s *DeliveryStore) Get(id string) (DeliveryAttempt, bool) {
	if s == nil {
		return DeliveryAttempt{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	attempt, ok := s.attempts[strings.TrimSpace(id)]
	return attempt, ok
}

// List returns the latest attempt state for a run, ordered by queue time.
// An empty runID lists all attempts.
func (s *DeliveryStore) List(runID string) []DeliveryAttempt {
	if s == nil {
		return []DeliveryAttempt{}
	}
	runID = strings.TrimSpace(runID)
	s.mu.Lock()
	defer s.mu.Unlock()
	attempts := make([]DeliveryAttempt, 0, len(s.attempts))
	for _, attempt := range s.attempts {
		if runID == "" || attempt.RunID == runID {
			attempts = append(attempts, attempt)
		}
	}
	sort.Slice(attempts, func(i, j int) bool {
		if attempts[i].QueuedAt.Equal(attempts[j].QueuedAt) {
			return attempts[i].ID < attempts[j].ID
		}
		return attempts[i].QueuedAt.Before(attempts[j].QueuedAt)
	})
	return attempts
}

func (s *DeliveryStore) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open delivery store: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if err := s.loadLine(scanner.Bytes(), line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("read delivery store: truncated JSONL: %w", err)
		}
		return fmt.Errorf("read delivery store: %w", err)
	}
	return nil
}

func (s *DeliveryStore) loadLine(data []byte, line int) error {
	var event deliveryEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("read delivery store line %d: %w", line, err)
	}
	if event.SchemaVersion != deliveryEventSchemaVersion {
		return fmt.Errorf("read delivery store line %d: unsupported schema version %d", line, event.SchemaVersion)
	}
	if err := s.validateLoadedAttempt(event.Attempt); err != nil {
		return fmt.Errorf("read delivery store line %d: %w", line, err)
	}
	s.rememberLocked(event.Attempt)
	return nil
}

func (s *DeliveryStore) validateLoadedAttempt(attempt DeliveryAttempt) error {
	if current, ok := s.attempts[attempt.ID]; ok {
		if !sameDeliveryIdentity(current, attempt) {
			return fmt.Errorf("delivery identity changed")
		}
		return validateStoredDeliveryTransition(current, attempt)
	}
	if err := validateQueuedDelivery(attempt); err != nil {
		return err
	}
	if priorID, exists := s.byIdempotency[attempt.IdempotencyKey]; exists && priorID != attempt.ID {
		return fmt.Errorf("duplicate idempotency key")
	}
	return nil
}

func (s *DeliveryStore) appendLocked(attempt DeliveryAttempt) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("append delivery store: create dir: %w", err)
	}
	event := deliveryEvent{SchemaVersion: deliveryEventSchemaVersion, RecordedAt: s.now().UTC(), Attempt: attempt}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("append delivery store: encode: %w", err)
	}
	data = append(data, '\n')
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("append delivery store: open: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("append delivery store: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("append delivery store: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("append delivery store: close: %w", err)
	}
	return nil
}

func (s *DeliveryStore) rememberLocked(attempt DeliveryAttempt) {
	s.attempts[attempt.ID] = attempt
	s.byIdempotency[attempt.IdempotencyKey] = attempt.ID
}

func sameDeliveryIntent(existing, candidate DeliveryAttempt) bool {
	return existing.TaskID == candidate.TaskID && existing.RunID == candidate.RunID && existing.Target == candidate.Target
}

func sameDeliveryIdentity(current, next DeliveryAttempt) bool {
	return current.ID == next.ID && current.IdempotencyKey == next.IdempotencyKey && sameDeliveryIntent(current, next) && current.QueuedAt.Equal(next.QueuedAt)
}

func validateQueuedDelivery(attempt DeliveryAttempt) error {
	if !hasCompleteDeliveryIdentity(attempt) {
		return fmt.Errorf("%w: incomplete queued delivery", ErrInvalidDelivery)
	}
	if !hasCleanQueuedState(attempt) {
		return fmt.Errorf("%w: invalid queued delivery state", ErrInvalidDelivery)
	}
	return nil
}

func hasCompleteDeliveryIdentity(attempt DeliveryAttempt) bool {
	return attempt.ID != "" && attempt.IdempotencyKey != "" && attempt.TaskID != "" && attempt.RunID != "" && attempt.Target.Type != "" && attempt.Target.Ref != "" && !attempt.QueuedAt.IsZero()
}

func hasCleanQueuedState(attempt DeliveryAttempt) bool {
	return attempt.Status == DeliveryQueued && attempt.StartedAt.IsZero() && attempt.FinishedAt.IsZero() && !attempt.Retryable && attempt.Failure == (DeliveryFailure{})
}

func validateStoredDeliveryTransition(current, next DeliveryAttempt) error {
	if !validStoredDeliveryTransition(current.Status, next.Status) {
		return invalidDeliveryTransition(current.Status, next.Status)
	}
	return validateStoredDeliveryState(next)
}

func validStoredDeliveryTransition(from, to DeliveryStatus) bool {
	switch from {
	case DeliveryQueued:
		return to == DeliverySending || to == DeliveryCanceled
	case DeliverySending:
		return to == DeliverySucceeded || to == DeliveryFailed || to == DeliveryCanceled
	default:
		return false
	}
}

func validateStoredDeliveryState(next DeliveryAttempt) error {
	if next.Status == DeliverySending && next.StartedAt.IsZero() {
		return fmt.Errorf("%w: sending delivery requires startedAt", ErrInvalidDelivery)
	}
	if next.Status != DeliverySending && next.FinishedAt.IsZero() {
		return fmt.Errorf("%w: terminal delivery requires finishedAt", ErrInvalidDelivery)
	}
	if next.Status == DeliveryFailed {
		return validateStoredDeliveryFailure(next)
	}
	if next.Retryable || next.Failure != (DeliveryFailure{}) {
		return fmt.Errorf("%w: failure details require failed status", ErrInvalidDelivery)
	}
	return nil
}

func validateStoredDeliveryFailure(next DeliveryAttempt) error {
	if !validDeliveryFailureClass(next.Failure.Class) {
		return fmt.Errorf("%w: failed delivery requires failure class", ErrInvalidDelivery)
	}
	wantRetryable := next.Failure.Class == DeliveryFailureTemporary || next.Failure.Class == DeliveryFailureRateLimited
	if next.Retryable != wantRetryable {
		return fmt.Errorf("%w: retryable does not match failure class", ErrInvalidDelivery)
	}
	return nil
}
