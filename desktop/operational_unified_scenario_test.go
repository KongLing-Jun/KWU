package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"kwu/internal/event"
)

func TestOperationalUnifiedLifecycleScenario(t *testing.T) {
	delivered := make(chan wireEventTab, 32)
	sink := &tabEventSink{tabID: "tab-unified", ctx: context.Background()}
	sink.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != eventChannel {
			t.Errorf("event channel = %q", name)
		}
		delivered <- payload[0].(wireEventTab)
	}
	emitter := &event.OperationalEmitter{Sink: sink, Now: func() time.Time { return time.Unix(1_788_200_000, 0).UTC() }}
	sessionScope := event.OperationalScope{SessionID: "session-unified"}
	planScope := event.OperationalScope{SessionID: "session-unified", PlanID: "plan-unified"}
	taskScope := event.OperationalScope{SessionID: "session-unified", PlanID: "plan-unified", TaskID: "task-unified"}
	callScope := event.OperationalScope{SessionID: "session-unified", PlanID: "plan-unified", TaskID: "task-unified", CallID: "call-unified"}
	toolScope := event.OperationalScope{SessionID: "session-unified", PlanID: "plan-unified", TaskID: "task-unified", CallID: "call-unified", ToolCallID: "tool-unified"}
	dashboardScope := event.OperationalScope{SessionID: "session-unified", PlanID: "plan-unified", TaskID: "task-unified", CallID: "call-unified", ToolCallID: "tool-unified", DashboardWriteID: "dashboard-unified"}
	facts := []event.OperationalFact{
		lifecycleFact(event.AggregateSession, "session-unified", sessionScope, event.SessionCreated, "created", "", ""),
		lifecycleFact(event.AggregateSession, "session-unified", sessionScope, event.SessionActivated, "activated", "created", ""),
		lifecycleFact(event.AggregatePlan, "plan-unified", planScope, event.PlanStarted, "started", "", "session-unified"),
		lifecycleFact(event.AggregatePlan, "plan-unified", planScope, event.PlanProposed, "proposed", "started", "session-unified"),
		lifecycleFact(event.AggregatePlan, "plan-unified", planScope, event.PlanApproved, "approved", "proposed", "session-unified"),
		lifecycleFact(event.AggregateTask, "task-unified", taskScope, event.TaskCreated, "created", "", "plan-unified"),
		lifecycleFact(event.AggregateTask, "task-unified", taskScope, event.TaskReady, "ready", "created", "plan-unified"),
		lifecycleFact(event.AggregateTask, "task-unified", taskScope, event.TaskRunning, "running", "ready", "plan-unified"),
		lifecycleFact(event.AggregateModelCall, "call-unified", callScope, event.ModelCallPrepared, "prepared", "", "task-unified"),
		lifecycleFact(event.AggregateModelCall, "call-unified", callScope, event.ModelCallRunning, "running", "prepared", "task-unified"),
		lifecycleFact(event.AggregateToolCall, "tool-unified", toolScope, event.ToolCallProposed, "proposed", "", "call-unified"),
		lifecycleFact(event.AggregateToolCall, "tool-unified", toolScope, event.ToolCallApprovalPending, "approval_pending", "proposed", "call-unified"),
		lifecycleFact(event.AggregateDashboardWrite, "dashboard-unified", dashboardScope, event.DashboardWriteApprovalPending, "approval_pending", "", "tool-unified"),
		lifecycleFact(event.AggregateToolCall, "tool-unified", toolScope, event.ToolCallRunning, "running", "approval_pending", "call-unified"),
		lifecycleFact(event.AggregateDashboardWrite, "dashboard-unified", dashboardScope, event.DashboardWriteRequested, "requested", "approval_pending", "tool-unified"),
		lifecycleFact(event.AggregateDashboardWrite, "dashboard-unified", dashboardScope, event.DashboardWriteCommitted, "committed", "requested", "tool-unified"),
		lifecycleFact(event.AggregateToolCall, "tool-unified", toolScope, event.ToolCallCompleted, "completed", "running", "call-unified"),
		lifecycleFact(event.AggregateModelCall, "call-unified", callScope, event.ModelCallCompleted, "completed", "running", "task-unified"),
		lifecycleFact(event.AggregateTask, "task-unified", taskScope, event.TaskCompleted, "completed", "running", "plan-unified"),
		lifecycleFact(event.AggregatePlan, "plan-unified", planScope, event.PlanExited, "exited", "approved", "session-unified"),
		lifecycleFact(event.AggregateSession, "session-unified", sessionScope, event.SessionClosed, "closed", "activated", ""),
	}
	for _, fact := range facts {
		emitter.Emit(fact)
	}
	got := make([]wireEventTab, 0, len(facts))
	for len(got) < len(facts) {
		select {
		case wire := <-delivered:
			got = append(got, wire)
		case <-time.After(2 * time.Second):
			t.Fatalf("delivered %d/%d operational events", len(got), len(facts))
		}
	}
	sequences := map[string]uint64{}
	seenEvents := map[string]bool{}
	for _, wire := range got {
		if wire.Operational == nil || wire.Operational.Scope.TabID != "tab-unified" {
			t.Fatalf("invalid routed event: %+v", wire.Operational)
		}
		key := wire.Operational.Aggregate.Kind + ":" + wire.Operational.Aggregate.ID
		want := sequences[key] + 1
		if wire.Operational.Aggregate.Sequence != want {
			t.Fatalf("%s sequence = %d, want %d", key, wire.Operational.Aggregate.Sequence, want)
		}
		sequences[key] = want
		if wire.Operational.CausationID != "" && !seenEvents[wire.Operational.CausationID] {
			t.Fatalf("event %s has unresolved causation %q", wire.Operational.EventID, wire.Operational.CausationID)
		}
		seenEvents[wire.Operational.EventID] = true
	}
	if len(sequences) != 6 {
		t.Fatalf("aggregate count = %d, want 6", len(sequences))
	}
	waitControlStats(t, &sink.runtimeEvents, func(stats runtimeControlStats) bool { return stats.Emitted == uint64(len(facts)) })
	stats := sink.runtimeEvents.ControlStats()
	if stats.Accepted != uint64(len(facts)) || stats.QueueDepth != 0 || stats.QueueWaitSamples != uint64(len(facts)) || stats.DeliverySamples != uint64(len(facts)) {
		t.Fatalf("transport stats = %+v", stats)
	}
	evidence := sink.operationalDiagnosticTrace().Evidence()
	if evidence.Counters.Received != uint64(len(facts)) || evidence.Counters.Recorded != uint64(len(facts)) || len(evidence.Records) != len(facts) {
		t.Fatalf("diagnostic evidence = %+v", evidence.Counters)
	}
	payload, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("SECRET_PROMPT"), []byte("SECRET_TOOL_ARGUMENTS"), []byte(`C:\Users`)} {
		if bytes.Contains(payload, forbidden) {
			t.Fatalf("metadata-only evidence contains forbidden content %q", forbidden)
		}
	}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"tab-unified": {ID: "tab-unified", sink: sink}}}
	exported, err := app.ExportOperationalEvidence("tab-unified")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Transport   runtimeControlStats `json:"transport"`
		Diagnostics struct {
			CaptureID    string `json:"captureId"`
			Quiescent    bool   `json:"quiescent"`
			FirstOrdinal uint64 `json:"firstOrdinal"`
			LastOrdinal  uint64 `json:"lastOrdinal"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(exported), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Transport.Emitted != uint64(len(facts)) || snapshot.Transport.QueueDepth != 0 || snapshot.Transport.QueueHighWater == 0 || snapshot.Transport.DeliverySamples != uint64(len(facts)) {
		t.Fatalf("exported transport stats = %+v", snapshot.Transport)
	}
	if snapshot.Diagnostics.CaptureID == "" || !snapshot.Diagnostics.Quiescent || snapshot.Diagnostics.FirstOrdinal != 1 || snapshot.Diagnostics.LastOrdinal != uint64(len(facts)) {
		t.Fatalf("exported capture boundary = %+v", snapshot.Diagnostics)
	}
}

func lifecycleFact(kind event.OperationalAggregateKind, id string, scope event.OperationalScope, eventKind event.OperationalKind, state, previous, causation string) event.OperationalFact {
	return event.OperationalFact{
		AggregateKind: kind, AggregateID: id, Scope: scope, Kind: eventKind,
		Stage: "controller", Outcome: state, Severity: "info", UserImpact: "none",
		Payload: &event.LifecycleOperationalPayload{State: state, PreviousState: previous},
	}
}
