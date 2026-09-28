package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/desktop/internal/operationaldiag"
	"kwu/internal/event"
)

func TestTabEventSinkRoutesOperationalEventThroughBoundedControlLane(t *testing.T) {
	delivered := make(chan wireEventTab, 1)
	sink := &tabEventSink{tabID: "tab-exact", ctx: context.Background()}
	sink.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != eventChannel {
			t.Errorf("event channel = %q, want %q", name, eventChannel)
		}
		delivered <- payload[0].(wireEventTab)
	}
	sink.Emit(operationalFixtureEvent("event-operational"))
	select {
	case wire := <-delivered:
		if wire.Kind != "operational" || wire.Operational == nil {
			t.Fatalf("wire event = %+v", wire)
		}
		if wire.Operational.Scope.TabID != "tab-exact" || wire.Operational.Scope.SessionID != "session-fixture" {
			t.Fatalf("operational scope = %+v", wire.Operational.Scope)
		}
	case <-time.After(time.Second):
		t.Fatal("operational event was not delivered")
	}
	waitControlStats(t, &sink.runtimeEvents, func(stats runtimeControlStats) bool { return stats.Emitted == 1 })
}

func TestOperationalEvidenceProjectionCannotCapturePromptPayload(t *testing.T) {
	sink := &tabEventSink{tabID: "tab-evidence", ctx: context.Background()}
	sink.runtimeEvents.emit = func(context.Context, string, ...interface{}) {}
	sink.Emit(operationalFixtureEvent("event-evidence"))
	payload, err := sink.operationalDiagnosticTrace().MarshalEvidence()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"SECRET_SUBJECT", "SECRET_REASON", "SECRET_PROMPT", `C:\Users\alice`} {
		if bytes.Contains(payload, []byte(forbidden)) {
			t.Fatalf("operational evidence leaked %q: %s", forbidden, payload)
		}
	}
	for _, want := range []string{`"eventId": "event-evidence"`, `"promptId": "prompt-fixture"`} {
		if !bytes.Contains(payload, []byte(want)) {
			t.Fatalf("operational evidence missing %s: %s", want, payload)
		}
	}
}

func TestExportOperationalPromptEvidenceRequiresExactTab(t *testing.T) {
	sink := &tabEventSink{tabID: "tab-export"}
	record := operationalDiagnosticRecord(operationalFixtureEvent("event-export").Operational.Diagnostic())
	record.Scope.TabID = "tab-export"
	sink.operationalDiagnosticTrace().Record(record)
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"tab-export": {ID: "tab-export", sink: sink}}, activeTabID: "tab-export"}
	if _, err := app.ExportOperationalPromptEvidence(""); err == nil {
		t.Fatal("empty tab id borrowed the active tab")
	}
	payload, err := app.ExportOperationalPromptEvidence("tab-export")
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Diagnostics struct {
			Records []json.RawMessage `json:"records"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(payload), &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence.Diagnostics.Records) != 1 || !strings.Contains(string(evidence.Diagnostics.Records[0]), "event-export") {
		t.Fatalf("exported evidence = %s", payload)
	}
}

func TestOperationalEvidenceModeUsesMemoryForProductionAndPersistentForDevelopment(t *testing.T) {
	production := newOperationalEvidenceRuntimeForProcess(desktopProfile{}, "kwu-desktop.exe")
	if production.mode != operationaldiag.ModeMemory || production.Trace().Mode() != operationaldiag.ModeMemory {
		t.Fatalf("production evidence mode = %q", production.Trace().Mode())
	}
	developmentRoot := t.TempDir()
	development := newOperationalEvidenceRuntimeForProcess(desktopProfile{Development: true, CacheRoot: developmentRoot}, "kwu-desktop-dev.exe")
	if development.mode != operationaldiag.ModePersistent || development.Trace().Mode() != operationaldiag.ModePersistent {
		t.Fatalf("development evidence mode = %q code=%q", development.Trace().Mode(), development.initCode)
	}
	if !strings.HasPrefix(development.directory, filepath.Clean(developmentRoot)) {
		t.Fatalf("development evidence directory = %q", development.directory)
	}
	if err := development.Trace().Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	testProcess := newOperationalEvidenceRuntimeForProcess(desktopProfile{Development: true, CacheRoot: developmentRoot}, "desktop.test")
	if testProcess.mode != operationaldiag.ModeMemory {
		t.Fatalf("implicit test process mode = %q", testProcess.mode)
	}
}

func operationalFixtureEvent(eventID string) event.Event {
	return event.Event{
		Kind: event.Operational, EventID: eventID,
		Operational: &event.OperationalEnvelope{
			SchemaVersion: event.OperationalSchemaVersion, EventID: eventID, OperationID: "operation-fixture",
			Aggregate: event.OperationalAggregate{Kind: "prompt", ID: "prompt-fixture", Sequence: 2},
			Scope:     event.OperationalScope{SessionID: "session-fixture", PromptID: "prompt-fixture"},
			Kind:      event.PromptEmitted, Stage: "controller", Outcome: "pending", Severity: "info", UserImpact: "waiting_for_user",
			PrivacyClass: event.PrivacyUserContent,
			Prompt:       &event.PromptOperationalPayload{Kind: event.PromptKindApproval, Lifecycle: event.PromptLifecyclePending, Approval: &event.Approval{ID: "prompt-fixture", Tool: "read_file", Subject: `SECRET_SUBJECT C:\Users\alice`, Reason: "SECRET_REASON SECRET_PROMPT"}},
		},
	}
}
