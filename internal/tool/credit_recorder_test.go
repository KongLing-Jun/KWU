package tool

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"kwu/internal/provider"
)

func TestCreditRecorderAggregatesConcurrentPaidToolsExactly(t *testing.T) {
	ctx, recorder := WithCreditRecorder(context.Background())
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			kind, value := provider.ToolChargeCredits, "0.2640"
			if index%2 == 0 {
				kind, value = provider.ToolChargeMaaSQuota, "125.5"
			}
			if err := RecordCredit(ctx, provider.ToolChargeMetadata{
				Tool: "paid", ServiceTaskID: fmt.Sprintf("task-%d", index), Kind: kind, Value: value,
			}); err != nil {
				t.Errorf("RecordCredit: %v", err)
			}
		}(i)
	}
	workers.Wait()

	quota, credits := recorder.Summary()
	if quota != "1255" || credits != "2.64" {
		t.Fatalf("Summary = quota %q, credits %q", quota, credits)
	}
	if got := len(recorder.Snapshot()); got != 20 {
		t.Fatalf("Snapshot length = %d, want 20", got)
	}
}

func TestRecordCreditOutsideTurnIsNoop(t *testing.T) {
	err := RecordCredit(context.Background(), provider.ToolChargeMetadata{})
	if err != nil {
		t.Fatalf("RecordCredit outside turn = %v", err)
	}
	if err := RecordCredit(nil, provider.ToolChargeMetadata{}); err != nil {
		t.Fatalf("RecordCredit nil context = %v", err)
	}
}

func TestCreditRecorderZeroValueCanRecord(t *testing.T) {
	recorder := &CreditRecorder{}
	if err := recorder.record(provider.ToolChargeMetadata{
		Tool: "generate_video", Kind: provider.ToolChargeCredits, Value: "1.25",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if quota, credits := recorder.Summary(); quota != "" || credits != "1.25" {
		t.Fatalf("Summary = quota %q, credits %q", quota, credits)
	}
}
