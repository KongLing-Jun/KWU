package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type DeliveryStatus string

const (
	DeliveryQueued    DeliveryStatus = "queued"
	DeliverySending   DeliveryStatus = "sending"
	DeliverySucceeded DeliveryStatus = "succeeded"
	DeliveryFailed    DeliveryStatus = "failed"
	DeliveryCanceled  DeliveryStatus = "canceled"
)

type DeliveryFailureClass string

const (
	DeliveryFailureTemporary     DeliveryFailureClass = "temporary"
	DeliveryFailureRateLimited   DeliveryFailureClass = "rate_limited"
	DeliveryFailurePermanent     DeliveryFailureClass = "permanent"
	DeliveryFailureTargetRevoked DeliveryFailureClass = "target_revoked"
)

var (
	ErrInvalidDelivery           = errors.New("automation: invalid delivery")
	ErrInvalidDeliveryTransition = errors.New("automation: invalid delivery transition")
)

type DeliveryFailure struct {
	Class   DeliveryFailureClass `json:"class"`
	Code    string               `json:"code,omitempty"`
	Message string               `json:"message,omitempty"`
}

type DeliveryAttemptInput struct {
	ID             string
	IdempotencyKey string
	TaskID         string
	RunID          string
	Target         DeliveryTarget
}

// DeliveryAttempt tracks notification delivery separately from RunStatus.
// It stores target references only; connector credentials remain in their
// owning credential store and are resolved by the production adapter.
type DeliveryAttempt struct {
	ID                string          `json:"id"`
	IdempotencyKey    string          `json:"idempotencyKey"`
	TaskID            string          `json:"taskId"`
	RunID             string          `json:"runId"`
	Target            DeliveryTarget  `json:"target"`
	Status            DeliveryStatus  `json:"status"`
	Retryable         bool            `json:"retryable,omitempty"`
	Failure           DeliveryFailure `json:"failure,omitempty"`
	ProviderMessageID string          `json:"providerMessageId,omitempty"`
	QueuedAt          time.Time       `json:"queuedAt"`
	StartedAt         time.Time       `json:"startedAt,omitempty"`
	FinishedAt        time.Time       `json:"finishedAt,omitempty"`
}

func NewDeliveryAttempt(input DeliveryAttemptInput, now time.Time) (DeliveryAttempt, error) {
	attempt := DeliveryAttempt{
		ID:             strings.TrimSpace(input.ID),
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		TaskID:         strings.TrimSpace(input.TaskID),
		RunID:          strings.TrimSpace(input.RunID),
		Target: DeliveryTarget{
			Type: strings.ToLower(strings.TrimSpace(input.Target.Type)),
			Ref:  strings.TrimSpace(input.Target.Ref),
		},
		Status:   DeliveryQueued,
		QueuedAt: now.UTC(),
	}
	if attempt.ID == "" || attempt.IdempotencyKey == "" || attempt.TaskID == "" || attempt.RunID == "" || attempt.Target.Type == "" || attempt.Target.Ref == "" || now.IsZero() {
		return DeliveryAttempt{}, fmt.Errorf("%w: id, idempotency key, task, run, target, and queue time are required", ErrInvalidDelivery)
	}
	return attempt, nil
}

func (a DeliveryAttempt) MarkSending(now time.Time) (DeliveryAttempt, error) {
	if a.Status != DeliveryQueued || now.IsZero() {
		return a, invalidDeliveryTransition(a.Status, DeliverySending)
	}
	a.Status = DeliverySending
	a.StartedAt = now.UTC()
	return a, nil
}

func (a DeliveryAttempt) MarkSucceeded(now time.Time, providerMessageID ...string) (DeliveryAttempt, error) {
	if a.Status != DeliverySending || now.IsZero() {
		return a, invalidDeliveryTransition(a.Status, DeliverySucceeded)
	}
	a.Status = DeliverySucceeded
	a.Retryable = false
	a.Failure = DeliveryFailure{}
	a.FinishedAt = now.UTC()
	if len(providerMessageID) > 0 {
		a.ProviderMessageID = strings.TrimSpace(providerMessageID[0])
	}
	return a, nil
}

func (a DeliveryAttempt) MarkFailed(failure DeliveryFailure, now time.Time) (DeliveryAttempt, error) {
	if a.Status != DeliverySending || now.IsZero() {
		return a, invalidDeliveryTransition(a.Status, DeliveryFailed)
	}
	failure.Code = strings.TrimSpace(failure.Code)
	failure.Message = strings.TrimSpace(failure.Message)
	if !validDeliveryFailureClass(failure.Class) {
		return a, fmt.Errorf("%w: unknown failure class %q", ErrInvalidDelivery, failure.Class)
	}
	a.Status = DeliveryFailed
	a.Failure = failure
	a.Retryable = failure.Class == DeliveryFailureTemporary || failure.Class == DeliveryFailureRateLimited
	a.FinishedAt = now.UTC()
	return a, nil
}

func (a DeliveryAttempt) Cancel(now time.Time) (DeliveryAttempt, error) {
	if (a.Status != DeliveryQueued && a.Status != DeliverySending) || now.IsZero() {
		return a, invalidDeliveryTransition(a.Status, DeliveryCanceled)
	}
	a.Status = DeliveryCanceled
	a.Retryable = false
	a.FinishedAt = now.UTC()
	return a, nil
}

func validDeliveryFailureClass(class DeliveryFailureClass) bool {
	switch class {
	case DeliveryFailureTemporary, DeliveryFailureRateLimited, DeliveryFailurePermanent, DeliveryFailureTargetRevoked:
		return true
	default:
		return false
	}
}

func invalidDeliveryTransition(from, to DeliveryStatus) error {
	return fmt.Errorf("%w: %s -> %s", ErrInvalidDeliveryTransition, from, to)
}

type DeliveryRequest struct {
	AttemptID     string         `json:"attemptId"`
	TaskID        string         `json:"taskId"`
	RunID         string         `json:"runId"`
	WorkspaceRoot string         `json:"workspaceRoot,omitempty"`
	Principal     string         `json:"principal,omitempty"`
	Target        DeliveryTarget `json:"target"`
	Body          string         `json:"body"`
}

type ClassifiedDeliveryError interface {
	error
	DeliveryFailureMetadata() DeliveryFailure
}

type DeliveryResult struct {
	ProviderMessageID string `json:"providerMessageId"`
}

type DeliveryAdapter interface {
	Deliver(context.Context, DeliveryRequest) (DeliveryResult, error)
}

type LocalDeliveryMessage struct {
	Request           DeliveryRequest
	ProviderMessageID string
	DeliveredAt       time.Time
}

// LocalDeliveryAdapter is a deterministic, no-secret adapter for integration
// and UI tests. It never resolves or accepts connector credentials.
type LocalDeliveryAdapter struct {
	mu       sync.Mutex
	messages []LocalDeliveryMessage
	now      func() time.Time
}

func NewLocalDeliveryAdapter() *LocalDeliveryAdapter {
	return &LocalDeliveryAdapter{now: time.Now}
}

func (a *LocalDeliveryAdapter) Deliver(ctx context.Context, request DeliveryRequest) (DeliveryResult, error) {
	if err := ctx.Err(); err != nil {
		return DeliveryResult{}, err
	}
	request.AttemptID = strings.TrimSpace(request.AttemptID)
	request.RunID = strings.TrimSpace(request.RunID)
	request.Target.Type = strings.ToLower(strings.TrimSpace(request.Target.Type))
	request.Target.Ref = strings.TrimSpace(request.Target.Ref)
	if request.AttemptID == "" || request.RunID == "" || request.Target.Type == "" || request.Target.Ref == "" {
		return DeliveryResult{}, fmt.Errorf("%w: local delivery request is incomplete", ErrInvalidDelivery)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	messageID := fmt.Sprintf("local-%d", len(a.messages)+1)
	a.messages = append(a.messages, LocalDeliveryMessage{
		Request:           request,
		ProviderMessageID: messageID,
		DeliveredAt:       a.now().UTC(),
	})
	return DeliveryResult{ProviderMessageID: messageID}, nil
}

func (a *LocalDeliveryAdapter) Messages() []LocalDeliveryMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]LocalDeliveryMessage(nil), a.messages...)
}
