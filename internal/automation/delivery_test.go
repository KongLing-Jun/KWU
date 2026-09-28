package automation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDeliveryAttemptLifecycleIsIndependentFromRunStatus(t *testing.T) {
	queuedAt := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	attempt, err := NewDeliveryAttempt(DeliveryAttemptInput{
		ID:             "delivery-1",
		IdempotencyKey: "run-1:wecom:team-news",
		TaskID:         "task-1",
		RunID:          "run-1",
		Target:         DeliveryTarget{Type: "wecom", Ref: "team-news"},
	}, queuedAt)
	if err != nil {
		t.Fatalf("NewDeliveryAttempt() error = %v", err)
	}
	if attempt.Status != DeliveryQueued || attempt.QueuedAt != queuedAt {
		t.Fatalf("new attempt = %+v", attempt)
	}

	sendingAt := queuedAt.Add(time.Second)
	attempt, err = attempt.MarkSending(sendingAt)
	if err != nil {
		t.Fatalf("MarkSending() error = %v", err)
	}
	if attempt.Status != DeliverySending || attempt.StartedAt != sendingAt {
		t.Fatalf("sending attempt = %+v", attempt)
	}

	finishedAt := sendingAt.Add(time.Second)
	attempt, err = attempt.MarkSucceeded(finishedAt)
	if err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}
	if attempt.Status != DeliverySucceeded || attempt.FinishedAt != finishedAt {
		t.Fatalf("succeeded attempt = %+v", attempt)
	}

	// A delivery succeeds or fails independently after the associated run has
	// already reached its own terminal status. DeliveryAttempt intentionally has
	// no RunStatus field to couple the two state machines.
	if _, err := attempt.MarkFailed(DeliveryFailure{Code: "late", Message: "late failure"}, finishedAt); !errors.Is(err, ErrInvalidDeliveryTransition) {
		t.Fatalf("terminal transition error = %v, want ErrInvalidDeliveryTransition", err)
	}
}

func TestDeliveryFailureClassificationControlsRetryEligibility(t *testing.T) {
	base, err := NewDeliveryAttempt(DeliveryAttemptInput{
		ID:             "delivery-2",
		IdempotencyKey: "run-2:webhook:alerts",
		TaskID:         "task-1",
		RunID:          "run-2",
		Target:         DeliveryTarget{Type: "webhook", Ref: "alerts"},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	base, err = base.MarkSending(time.Now())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		class     DeliveryFailureClass
		retryable bool
	}{
		{name: "temporary", class: DeliveryFailureTemporary, retryable: true},
		{name: "rate limited", class: DeliveryFailureRateLimited, retryable: true},
		{name: "permanent", class: DeliveryFailurePermanent, retryable: false},
		{name: "target revoked", class: DeliveryFailureTargetRevoked, retryable: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failed, err := base.MarkFailed(DeliveryFailure{Class: tt.class, Code: "provider_error", Message: "provider rejected request"}, time.Now())
			if err != nil {
				t.Fatalf("MarkFailed() error = %v", err)
			}
			if failed.Status != DeliveryFailed || failed.Retryable != tt.retryable || failed.Failure.Class != tt.class {
				t.Fatalf("failed attempt = %+v", failed)
			}
		})
	}
}

func TestDeliveryAttemptCanBeCanceledOnlyBeforeTerminalState(t *testing.T) {
	attempt, err := NewDeliveryAttempt(DeliveryAttemptInput{
		ID:             "delivery-3",
		IdempotencyKey: "run-3:local:test",
		TaskID:         "task-1",
		RunID:          "run-3",
		Target:         DeliveryTarget{Type: "local", Ref: "test"},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := attempt.Cancel(time.Now())
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if canceled.Status != DeliveryCanceled {
		t.Fatalf("status = %q, want %q", canceled.Status, DeliveryCanceled)
	}
	if _, err := canceled.MarkSending(time.Now()); !errors.Is(err, ErrInvalidDeliveryTransition) {
		t.Fatalf("MarkSending() after cancel error = %v", err)
	}
}

func TestLocalDeliveryAdapterUsesReferencesNotCredentials(t *testing.T) {
	adapter := NewLocalDeliveryAdapter()
	request := DeliveryRequest{
		AttemptID: "delivery-4",
		RunID:     "run-4",
		Target:    DeliveryTarget{Type: "local", Ref: "test-inbox"},
		Body:      "daily summary",
	}
	result, err := adapter.Deliver(context.Background(), request)
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if result.ProviderMessageID == "" {
		t.Fatal("provider message ID is empty")
	}
	messages := adapter.Messages()
	if len(messages) != 1 || messages[0].Request.Target.Ref != "test-inbox" || messages[0].Request.Body != "daily summary" {
		t.Fatalf("messages = %+v", messages)
	}
}
