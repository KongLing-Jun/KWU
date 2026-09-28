package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/event"
)

func TestDashboardAddWidgetToolPublishesClosedWidgetContract(t *testing.T) {
	var schema struct {
		Properties struct {
			Widget struct {
				Properties struct {
					Type struct {
						Enum []string `json:"enum"`
					} `json:"type"`
					Surface struct {
						Enum []string `json:"enum"`
					} `json:"surface"`
					Status struct {
						Enum []string `json:"enum"`
					} `json:"status"`
					Props struct {
						Required   []string `json:"required"`
						Properties struct {
							Kind struct {
								Enum []string `json:"enum"`
							} `json:"kind"`
							Metrics struct {
								Type  string `json:"type"`
								Items struct {
									Required []string `json:"required"`
								} `json:"items"`
							} `json:"metrics"`
							Findings struct {
								Type  string `json:"type"`
								Items struct {
									Required []string `json:"required"`
								} `json:"items"`
							} `json:"findings"`
						} `json:"properties"`
					} `json:"props"`
				} `json:"properties"`
			} `json:"widget"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(dashboardAddWidgetToolSchema, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	wantTypes := []string{"research.summary", "trend.chart", "comparison.bars", "source.coverage", "product.catalog", "chart.bar", "chart.donut", "chart.histogram"}
	widget := schema.Properties.Widget.Properties
	if !reflect.DeepEqual(widget.Type.Enum, wantTypes) || !reflect.DeepEqual(widget.Props.Properties.Kind.Enum, wantTypes) {
		t.Fatalf("widget type contract is open or inconsistent: type=%v kind=%v", widget.Type.Enum, widget.Props.Properties.Kind.Enum)
	}
	if !reflect.DeepEqual(widget.Surface.Enum, []string{"research-canvas"}) {
		t.Fatalf("surface enum = %v", widget.Surface.Enum)
	}
	if !reflect.DeepEqual(widget.Status.Enum, []string{"queued", "loading", "partial", "ready", "stale", "error"}) {
		t.Fatalf("status enum = %v", widget.Status.Enum)
	}
	if !reflect.DeepEqual(widget.Props.Required, []string{"kind"}) {
		t.Fatalf("props required = %v", widget.Props.Required)
	}
	if widget.Props.Properties.Metrics.Type != "array" || !reflect.DeepEqual(widget.Props.Properties.Metrics.Items.Required, []string{"label", "value", "detail"}) {
		t.Fatalf("metrics contract = %#v", widget.Props.Properties.Metrics)
	}
	if widget.Props.Properties.Findings.Type != "array" || !reflect.DeepEqual(widget.Props.Properties.Findings.Items.Required, []string{"label", "text"}) {
		t.Fatalf("findings contract = %#v", widget.Props.Properties.Findings)
	}
	if description := (dashboardAddWidgetTool{}).Description(); !strings.Contains(description, "research.summary") || !strings.Contains(description, "props.kind must exactly equal widget.type") {
		t.Fatalf("description does not teach the closed contract: %q", description)
	}
}

func TestCapabilityDashboardToolPinsStableSessionAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "sessions", "session-a.jsonl")
	identity := capabilityStableSessionIdentity(sessionPath)
	scope := testCapabilityScope()
	binding := capabilitySessionBinding{
		SchemaVersion: capabilityDashboardSchemaVersion, SessionHash: "tab-a", SessionIdentity: identity,
		SessionDocumentID: scope.DocumentID, Scope: scope, UpdatedAt: "2026-09-01T00:00:00Z",
	}
	if _, err := pinCapabilityPackageBinding(&binding); err != nil {
		t.Fatal(err)
	}
	if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(root, identity), binding); err != nil {
		t.Fatal(err)
	}
	document := testCapabilityDocument()
	if err := writeCapabilityJSONAtomic(capabilityDashboardPath(root, scope), document); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	tools, err := app.capabilitySessionToolsAtRoot(root, sessionPath)
	if err != nil || len(tools) != 2 {
		t.Fatalf("session tools: count=%d err=%v", len(tools), err)
	}
	writer := tools[0]
	if tools[1].Name() != capabilityLoadSkillToolName || !tools[1].ReadOnly() {
		t.Fatal("expected a separate read-only Skill loader")
	}
	if writer.Name() != dashboardAddWidgetToolName || writer.ReadOnly() {
		t.Fatalf("unexpected writer contract: %s readonly=%v", writer.Name(), writer.ReadOnly())
	}
	args := json.RawMessage(`{
      "operationId":"tool-call:one",
      "baseRevision":1,
      "widget":{"id":"catalog-one","type":"product.catalog","version":"1.0.0","surface":"research-canvas","status":"ready","title":"Catalog","props":{"kind":"product.catalog","products":[]},"evidenceRefs":[],"revision":1}
    }`)
	var facts []event.OperationalEnvelope
	emitter := &event.OperationalEmitter{Sink: event.FuncSink(func(value event.Event) { facts = append(facts, *value.Operational) })}
	ctx := event.WithOperationalEmitter(context.Background(), emitter)
	ctx = event.WithOperationalScope(ctx, event.OperationalScope{SessionID: "session-one", TaskID: "task-one", CallID: "model-call-one"})
	ctx = agent.WithCallContext(ctx, "tool-call-one", event.Discard, nil, false)
	emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateModelCall, AggregateID: "model-call-one",
		Scope: event.OperationalScope{SessionID: "session-one", TaskID: "task-one", CallID: "model-call-one"},
		Kind:  event.ModelCallCompleted, Stage: "persistence", Outcome: "completed", Severity: "info", UserImpact: "none",
	})
	toolEventID := emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateToolCall, AggregateID: "tool-call-one",
		Scope: event.OperationalScope{TaskID: "task-one", ToolCallID: "tool-call-one"},
		Kind:  event.ToolCallRunning, Stage: "agent", Outcome: "running", Severity: "info", UserImpact: "none",
	})
	if _, err := writer.Execute(ctx, args); err != nil {
		t.Fatalf("first execute: %v", err)
	}
	if _, err := writer.Execute(ctx, args); err != nil {
		t.Fatalf("idempotent execute: %v", err)
	}
	var got capabilityDashboardDocument
	if err := readCapabilityJSON(capabilityDashboardPath(root, scope), &got); err != nil {
		t.Fatal(err)
	}
	if got.Revision != 2 || len(got.Widgets) != 2 || got.Widgets[1].ID != "catalog-one" {
		t.Fatalf("unexpected document: %#v", got)
	}

	changed := binding
	changed.Scope.TaskID = "another-task"
	if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(root, identity), changed); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Execute(ctx, args); err == nil {
		t.Fatal("expected changed binding to invalidate pinned tool")
	}
	wantKinds := []event.OperationalKind{
		event.DashboardWriteRequested, event.DashboardWriteCommitted,
		event.DashboardWriteRequested, event.DashboardWriteCommitted,
		event.DashboardWriteRequested, event.DashboardWriteRejected,
	}
	dashboardFacts := make([]event.OperationalEnvelope, 0, len(wantKinds))
	for _, fact := range facts {
		if fact.Aggregate.Kind == event.AggregateDashboardWrite {
			dashboardFacts = append(dashboardFacts, fact)
		}
	}
	if len(dashboardFacts) != len(wantKinds) {
		t.Fatalf("dashboard lifecycle count = %d, want %d", len(dashboardFacts), len(wantKinds))
	}
	for index, want := range wantKinds {
		if dashboardFacts[index].Kind != want || dashboardFacts[index].Aggregate.Sequence != uint64(index+1) {
			t.Fatalf("dashboard fact %d = %+v", index, dashboardFacts[index])
		}
		if dashboardFacts[index].Scope.SessionID != "session-one" || dashboardFacts[index].Scope.TaskID != "task-one" || dashboardFacts[index].Scope.CallID != "model-call-one" || dashboardFacts[index].Scope.ToolCallID != "tool-call-one" {
			t.Fatalf("dashboard fact %d scope = %+v", index, dashboardFacts[index].Scope)
		}
	}
	if dashboardFacts[0].CausationID != toolEventID {
		t.Fatalf("dashboard first causation = %q, want %q", dashboardFacts[0].CausationID, toolEventID)
	}
}

func TestCapabilityDashboardToolIsAbsentOutsideBoundPackage(t *testing.T) {
	app := &App{}
	root := t.TempDir()
	if tools, err := app.capabilitySessionToolsAtRoot(root, filepath.Join(root, "missing.jsonl")); err != nil || len(tools) != 0 {
		t.Fatalf("unbound tools: count=%d err=%v", len(tools), err)
	}
	sessionPath := filepath.Join(root, "other.jsonl")
	identity := capabilityStableSessionIdentity(sessionPath)
	binding := capabilitySessionBinding{
		SchemaVersion: capabilityDashboardSchemaVersion, SessionHash: "other", SessionIdentity: identity,
		SessionDocumentID: "primary", Scope: CapabilityTaskScope{CapabilityID: "another-capability", WorkspaceID: "workspace", TaskID: "task", DocumentID: "primary"},
	}
	if _, err := pinCapabilityPackageBinding(&binding); err == nil {
		t.Fatal("unexpected package pin for unknown capability")
	}
	if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(root, identity), binding); err != nil {
		t.Fatal(err)
	}
	if tools, err := app.capabilitySessionToolsAtRoot(root, sessionPath); err != nil || len(tools) != 0 {
		t.Fatalf("other capability tools: count=%d err=%v", len(tools), err)
	}
}

func TestCapabilityDashboardToolRejectsCorruptBinding(t *testing.T) {
	app := &App{}
	root := t.TempDir()
	sessionPath := filepath.Join(root, "corrupt.jsonl")
	identity := capabilityStableSessionIdentity(sessionPath)
	if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(root, identity), map[string]any{"schemaVersion": 1, "unexpected": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.capabilitySessionToolsAtRoot(root, sessionPath); err == nil {
		t.Fatal("expected corrupt binding rejection")
	}
}

func TestDashboardAddWidgetToolRejectsUnknownArguments(t *testing.T) {
	writer := dashboardAddWidgetTool{apply: func(context.Context, capabilityDashboardAction) (string, error) { return "", nil }}
	if _, err := writer.Execute(context.Background(), json.RawMessage(`{"operationId":"x","baseRevision":0,"widget":{},"scope":"forbidden"}`)); err == nil {
		t.Fatal("expected unknown argument rejection")
	}
}
