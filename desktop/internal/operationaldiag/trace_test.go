package operationaldiag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func validRecord(eventID string, sequence uint64) Record {
	return Record{
		EventID: eventID, OperationID: "op-fixture",
		Aggregate:  Aggregate{Kind: AggregatePrompt, ID: "prompt-fixture", Sequence: sequence},
		Scope:      Scope{TabID: "tab-fixture", SessionID: "session-fixture", TurnID: "turn-fixture", PromptID: "prompt-fixture"},
		PromptKind: PromptApproval, Kind: KindPromptEmitted, Stage: StageTransport,
		Outcome: OutcomePending, Severity: SeverityInfo, UserImpact: ImpactWaiting,
		PrivacyClass: PrivacyMetadataOnly,
	}
}

func TestTraceExportsDeterministicMetadataOnlyEvidence(t *testing.T) {
	trace := New(4, WithOccurrenceGenerator(func() string { return "o-fixture" }), WithClock(func() time.Time { return time.Unix(1_788_200_000, 0) }))
	occurrenceID := trace.NewOccurrenceID()
	first := validRecord("event-1", 1)
	first.OccurrenceID = occurrenceID
	first.Actions = []UserAction{ActionRetry, ActionCancel, ActionReport, ActionRetry}
	second := validRecord("event-2", 2)
	second.Kind, second.Stage, second.Outcome = KindPromptReplyFailed, StageReply, OutcomeFailed
	second.Severity, second.UserImpact, second.ReceiptStatus = SeverityError, ImpactActionRequired, ReceiptFailed
	second.OccurrenceID, second.Retryable = occurrenceID, true
	if !trace.Record(first) || !trace.Record(second) {
		t.Fatal("valid records were rejected")
	}
	one, err := trace.MarshalEvidence()
	if err != nil {
		t.Fatal(err)
	}
	two, err := trace.MarshalEvidence()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one, two) {
		t.Fatalf("evidence is not deterministic:\n%s\n%s", one, two)
	}
	for _, want := range []string{`"occurrenceId": "o-fixture"`, `"receiptStatus": "failed"`, `"recorded": 2`} {
		if !bytes.Contains(one, []byte(want)) {
			t.Fatalf("evidence missing %s: %s", want, one)
		}
	}
}

func TestPersistentTraceFlushesMetadataOnlyJSONLAndRecoversRun(t *testing.T) {
	root := t.TempDir()
	options := PersistentOptions{
		Directory: root, RunID: "run-fixture", Capacity: 4, QueueCapacity: 16,
		TerminalReserve: 4, MaxFileBytes: 1024, MaxFiles: 3,
		Now: func() time.Time { return time.Unix(1_788_200_000, 0) },
	}
	trace, err := NewPersistent(options, WithOccurrenceGenerator(func() string { return "capture-fixture" }))
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 12; index++ {
		record := validRecord(fmt.Sprintf("event-%d", index), uint64(index+1))
		record.OccurredAt = time.Unix(1_788_200_000+int64(index), 0)
		if !trace.Record(record) {
			t.Fatalf("record %d rejected", index)
		}
	}
	committed, err := trace.Flush(context.Background())
	if err != nil || committed != 12 {
		t.Fatalf("flush = %d, %v", committed, err)
	}
	evidence := trace.Capture(true)
	if evidence.Mode != ModePersistent || evidence.RunID != "run-fixture" || evidence.CommittedOrdinal != 12 || !evidence.Quiescent || !evidence.Truncated {
		t.Fatalf("evidence = %+v", evidence)
	}
	if evidence.Counters.PersistenceWritten != 12 || evidence.Counters.PersistenceFlushed != 1 || evidence.Counters.PersistenceRotated == 0 {
		t.Fatalf("persistence counters = %+v", evidence.Counters)
	}
	if err := trace.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	runDir := filepath.Join(root, "run-fixture")
	entries, err := os.ReadDir(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 || len(entries) > 3 {
		t.Fatalf("rotated files = %d", len(entries))
	}
	for _, entry := range entries {
		payload, err := os.ReadFile(filepath.Join(runDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(payload), []byte{'\n'}) {
			var record Record
			if err := json.Unmarshal(line, &record); err != nil {
				t.Fatalf("invalid jsonl %s: %v", entry.Name(), err)
			}
			if record.PrivacyClass != PrivacyMetadataOnly {
				t.Fatalf("privacy class = %q", record.PrivacyClass)
			}
		}
	}
	latestPath := filepath.Join(runDir, entries[len(entries)-1].Name())
	file, err := os.OpenFile(latestPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"incomplete"`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewPersistent(options)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Counters().PersistenceRecovered != 1 || restarted.Counters().PersistenceTailTruncated != 1 {
		t.Fatalf("restart counters = %+v", restarted.Counters())
	}
	if !restarted.Record(validRecord("event-restarted", 1)) {
		t.Fatal("restart record rejected")
	}
	if _, err := restarted.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if restarted.Capture(true).CommittedOrdinal != 13 {
		t.Fatalf("restart committed ordinal = %d", restarted.Capture(true).CommittedOrdinal)
	}
	if err := restarted.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPersistentTraceReservesQueueSpaceForTerminalFacts(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	trace, err := NewPersistent(PersistentOptions{
		Directory: t.TempDir(), RunID: "queue-fixture", Capacity: 32,
		QueueCapacity: 4, TerminalReserve: 1, MaxFileBytes: 1 << 20, MaxFiles: 1,
		beforeWrite: func() {
			once.Do(func() { close(started) })
			<-release
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	trace.Record(validRecord("event-0", 1))
	<-started
	for index := 1; index <= 10; index++ {
		trace.Record(validRecord(fmt.Sprintf("event-%d", index), uint64(index+1)))
	}
	terminal := validRecord("event-terminal", 12)
	terminal.Kind, terminal.Outcome = KindPromptResolved, OutcomeResolved
	trace.Record(terminal)
	close(release)
	committed, err := trace.Flush(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	counters := trace.Counters()
	if counters.PersistenceQueueFull == 0 || counters.PersistenceDropped == 0 || committed != 12 {
		t.Fatalf("queue counters = %+v committed=%d", counters, committed)
	}
	if !trace.Capture(true).Truncated {
		t.Fatal("queue loss was not marked truncated")
	}
	if err := trace.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTraceRejectsDuplicateAndBoundsHistory(t *testing.T) {
	trace := New(2)
	if !trace.Record(validRecord("event-1", 1)) {
		t.Fatal("first record rejected")
	}
	if trace.Record(validRecord("event-1", 1)) {
		t.Fatal("duplicate event was accepted")
	}
	if !trace.Record(validRecord("event-2", 2)) || !trace.Record(validRecord("event-3", 3)) {
		t.Fatal("bounded records were rejected")
	}
	snapshot := trace.Snapshot()
	if len(snapshot) != 2 || snapshot[0].EventID != "event-2" || snapshot[1].EventID != "event-3" {
		t.Fatalf("bounded snapshot = %+v", snapshot)
	}
	counters := trace.Counters()
	if counters.Received != 4 || counters.Recorded != 3 || counters.Duplicate != 1 || counters.BufferFull != 1 {
		t.Fatalf("counters = %+v", counters)
	}
}

func TestTraceAcceptsEveryOperationalAggregate(t *testing.T) {
	tests := []struct {
		aggregate AggregateKind
		kind      Kind
		scope     Scope
	}{
		{AggregateTask, "task.running", Scope{TaskID: "task-1"}},
		{AggregateModelCall, "model_call.completed", Scope{TaskID: "task-1", CallID: "call-1"}},
		{AggregateToolCall, "tool_call.failed", Scope{ToolCallID: "tool-1"}},
		{AggregateSession, "session.recovered", Scope{SessionID: "session-1"}},
		{AggregateDashboardWrite, "dashboard_write.committed", Scope{DashboardWriteID: "write-1"}},
		{AggregatePlan, "plan.proposed", Scope{PlanID: "plan-1"}},
	}
	trace := New(len(tests))
	for index, test := range tests {
		record := Record{
			EventID: fmt.Sprintf("event-%d", index), OperationID: fmt.Sprintf("operation-%d", index),
			Aggregate: Aggregate{Kind: test.aggregate, ID: fmt.Sprintf("aggregate-%d", index), Sequence: 1},
			Scope:     test.scope, Kind: test.kind, Stage: StageAgent, Outcome: "running", Severity: SeverityInfo,
		}
		if !trace.Record(record) {
			t.Fatalf("aggregate %s was rejected", test.aggregate)
		}
	}
	if got := len(trace.Snapshot()); got != len(tests) {
		t.Fatalf("records = %d, want %d", got, len(tests))
	}
}

func TestTraceConcurrentWritersRemainBoundedAndUnique(t *testing.T) {
	trace := New(32)
	var group sync.WaitGroup
	for index := 0; index < 128; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			record := validRecord(fmt.Sprintf("event-%d", index), uint64(index+1))
			if !trace.Record(record) {
				t.Errorf("record %d rejected", index)
			}
		}(index)
	}
	group.Wait()
	records := trace.Snapshot()
	if len(records) != 32 {
		t.Fatalf("record count = %d, want 32", len(records))
	}
	seen := map[string]bool{}
	for _, record := range records {
		if seen[record.EventID] {
			t.Fatalf("duplicate snapshot record %q", record.EventID)
		}
		seen[record.EventID] = true
	}
	if counters := trace.Counters(); counters.Recorded != 128 || counters.BufferFull != 96 {
		t.Fatalf("counters = %+v", counters)
	}
}

func TestTracePoisonDataCannotEnterEvidence(t *testing.T) {
	trace := New(2)
	record := validRecord("event-safe", 1)
	record.CausationID = `C:\Users\alice\secret.txt`
	record.OccurrenceID = "Bearer secret-token-value"
	record.Scope.TabID = "alice@example.com"
	record.Scope.SessionID = "/home/alice/private.jsonl"
	record.Scope.CandidateID = "prompt body must not survive"
	record.Actions = []UserAction{UserAction("password=secret"), ActionReport}
	if !trace.Record(record) {
		t.Fatal("record with poisoned optional metadata was rejected")
	}
	payload, err := trace.MarshalEvidence()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"alice", "secret", "Bearer", "private.jsonl", "prompt body", "password"} {
		if bytes.Contains(payload, []byte(forbidden)) {
			t.Fatalf("evidence leaked %q: %s", forbidden, payload)
		}
	}
}

func TestDiagnosticRecordHasNoPromptPayloadSurface(t *testing.T) {
	forbidden := map[string]bool{"Prompt": true, "Questions": true, "Answers": true, "Text": true, "Reasoning": true, "Tool": true, "Subject": true, "Args": true, "Result": true, "Path": true}
	for _, value := range []reflect.Type{reflect.TypeOf(Record{}), reflect.TypeOf(Scope{})} {
		for index := 0; index < value.NumField(); index++ {
			if forbidden[value.Field(index).Name] {
				t.Fatalf("diagnostic type exposes forbidden field %s", value.Field(index).Name)
			}
		}
	}
}

func TestDisabledAndPanickingProjectorsAreIsolated(t *testing.T) {
	trace := New(2)
	trace.SetEnabled(false)
	projector := NewIsolatedProjector(trace, nil)
	if projector.Project(validRecord("event-disabled", 1)) {
		t.Fatal("disabled trace accepted a record")
	}
	trace.SetEnabled(true)
	panicking := NewIsolatedProjector(trace, func(Record) bool { panic("synthetic projector failure") })
	if panicking.Project(validRecord("event-panic", 2)) {
		t.Fatal("panicking projector reported success")
	}
	if !projector.Project(validRecord("event-survivor", 3)) {
		t.Fatal("healthy projector did not survive isolated failure")
	}
	counters := trace.Counters()
	if counters.Disabled != 1 || counters.ProjectorFailed != 1 || counters.Recorded != 1 {
		t.Fatalf("isolation counters = %+v", counters)
	}
}

func TestTransportCountersUseClosedOutcomes(t *testing.T) {
	trace := New(1)
	trace.ObserveTransport(OutcomeQueueFull)
	trace.ObserveTransport(OutcomeDisabled)
	trace.ObserveTransport(OutcomeLate)
	trace.ObserveTransport(OutcomeFailed)
	trace.ObserveTransport(OutcomeAccepted)
	counters := trace.Counters()
	if counters.QueueFull != 1 || counters.Disabled != 1 || counters.Late != 1 || counters.DeliveryFailed != 1 {
		t.Fatalf("transport counters = %+v", counters)
	}
}

func BenchmarkTraceRecordMemory(b *testing.B) {
	trace := New(256)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		trace.Record(validRecord(fmt.Sprintf("event-%d", index), uint64(index+1)))
	}
}

func BenchmarkTraceRecordPersistentEnqueue(b *testing.B) {
	trace, err := NewPersistent(PersistentOptions{
		Directory: b.TempDir(), RunID: "benchmark", Capacity: 256,
		QueueCapacity: 4096, TerminalReserve: 256, MaxFileBytes: 64 << 20, MaxFiles: 2,
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		trace.Record(validRecord(fmt.Sprintf("event-%d", index), uint64(index+1)))
	}
	b.StopTimer()
	if _, err := trace.Flush(context.Background()); err != nil {
		b.Fatal(err)
	}
	if err := trace.Close(context.Background()); err != nil {
		b.Fatal(err)
	}
}
