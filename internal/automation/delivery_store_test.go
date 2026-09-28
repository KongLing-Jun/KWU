package automation

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDeliveryStoreEnqueueIsIdempotentUnderConcurrency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delivery", "attempts.jsonl")
	store, err := OpenDeliveryStore(path)
	if err != nil {
		t.Fatalf("OpenDeliveryStore() error = %v", err)
	}
	input := DeliveryAttemptInput{
		ID:             "delivery-1",
		IdempotencyKey: "run-1:wecom:team-news",
		TaskID:         "task-1",
		RunID:          "run-1",
		Target:         DeliveryTarget{Type: "wecom", Ref: "team-news"},
	}

	const callers = 20
	var wg sync.WaitGroup
	created := make(chan bool, callers)
	errs := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempt, wasCreated, err := store.Enqueue(input, time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC))
			if err == nil && attempt.ID != "delivery-1" {
				t.Errorf("attempt ID = %q", attempt.ID)
			}
			created <- wasCreated
			errs <- err
		}()
	}
	wg.Wait()
	close(created)
	close(errs)
	createdCount := 0
	for value := range created {
		if value {
			createdCount++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
	}
	if createdCount != 1 {
		t.Fatalf("created count = %d, want 1", createdCount)
	}
	if lines := countJSONLLines(t, path); lines != 1 {
		t.Fatalf("JSONL lines = %d, want 1", lines)
	}
}

func TestDeliveryStoreAppendsTransitionsAndRecoversLatestState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	store, err := OpenDeliveryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	queued, _, err := store.Enqueue(DeliveryAttemptInput{
		ID:             "delivery-2",
		IdempotencyKey: "run-2:local:inbox",
		TaskID:         "task-1",
		RunID:          "run-2",
		Target:         DeliveryTarget{Type: "local", Ref: "inbox"},
	}, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	sending, err := queued.MarkSending(time.Date(2026, 8, 1, 10, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(sending); err != nil {
		t.Fatalf("Append(sending) error = %v", err)
	}
	succeeded, err := sending.MarkSucceeded(time.Date(2026, 8, 1, 10, 0, 2, 0, time.UTC), "local-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(succeeded); err != nil {
		t.Fatalf("Append(succeeded) error = %v", err)
	}
	if lines := countJSONLLines(t, path); lines != 3 {
		t.Fatalf("JSONL lines = %d, want 3", lines)
	}

	reopened, err := OpenDeliveryStore(path)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	got, ok := reopened.Get("delivery-2")
	if !ok || got.Status != DeliverySucceeded || got.ProviderMessageID != "local-1" {
		t.Fatalf("recovered attempt = %+v, ok = %v", got, ok)
	}
	listed := reopened.List("run-2")
	if len(listed) != 1 || listed[0].ID != "delivery-2" {
		t.Fatalf("List() = %+v", listed)
	}
}

func TestDeliveryStoreRejectsInvalidTransitionWithoutAppending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	store, err := OpenDeliveryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	queued, _, err := store.Enqueue(DeliveryAttemptInput{
		ID:             "delivery-3",
		IdempotencyKey: "run-3:local:inbox",
		TaskID:         "task-1",
		RunID:          "run-3",
		Target:         DeliveryTarget{Type: "local", Ref: "inbox"},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	queued.Status = DeliverySucceeded
	queued.FinishedAt = time.Now().UTC()
	if err := store.Append(queued); err == nil {
		t.Fatal("Append() error = nil, want invalid transition")
	}
	if lines := countJSONLLines(t, path); lines != 1 {
		t.Fatalf("JSONL lines after rejected append = %d, want 1", lines)
	}
}

func TestDeliveryStorePersistsNoPayloadOrCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	store, err := OpenDeliveryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Enqueue(DeliveryAttemptInput{
		ID:             "delivery-4",
		IdempotencyKey: "run-4:wecom:configured-target",
		TaskID:         "task-1",
		RunID:          "run-4",
		Target:         DeliveryTarget{Type: "wecom", Ref: "configured-target"},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"secret-token-value", "daily summary body", "credential", "accessToken"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("store contains forbidden value %q: %s", forbidden, raw)
		}
	}
	if !strings.Contains(string(raw), `"ref":"configured-target"`) {
		t.Fatalf("store does not contain target reference: %s", raw)
	}
}

func countJSONLLines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return count
}
