package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/tool"
)

const dashboardAddWidgetToolName = "dashboard_add_widget"

var dashboardAddWidgetToolSchema = json.RawMessage(`{
  "type":"object",
  "required":["operationId","baseRevision","widget"],
  "additionalProperties":false,
  "properties":{
    "operationId":{"type":"string","minLength":1,"maxLength":160},
    "baseRevision":{"type":"integer","minimum":0},
    "widget":{"type":"object","required":["id","type","version","surface","status","title","props"],"additionalProperties":false,"properties":{
      "id":{"type":"string"},
      "type":{"type":"string","enum":["research.summary","trend.chart","comparison.bars","source.coverage","product.catalog","chart.bar","chart.donut","chart.histogram"]},
      "version":{"type":"string","enum":["1.0.0"]},
      "surface":{"type":"string","enum":["research-canvas"]},
      "status":{"type":"string","enum":["queued","loading","partial","ready","stale","error"]},
      "title":{"type":"string","minLength":1},
      "props":{"type":"object","required":["kind"],"properties":{
        "kind":{"type":"string","enum":["research.summary","trend.chart","comparison.bars","source.coverage","product.catalog","chart.bar","chart.donut","chart.histogram"]},
        "eyebrow":{"type":"string"},
        "conclusion":{"type":"string"},
        "confidence":{"type":"number","minimum":0,"maximum":100},
        "summary":{"type":"string"},
        "metrics":{"type":"array","items":{"type":"object","required":["label","value","detail"],"properties":{"label":{"type":"string"},"value":{"type":"string"},"detail":{"type":"string"},"tone":{"type":"string","enum":["positive","neutral","warning"]}}}},
        "findings":{"type":"array","items":{"type":"object","required":["label","text"],"properties":{"label":{"type":"string"},"text":{"type":"string"}}}}
      }},
      "evidenceRefs":{"type":"array","items":{"type":"string"}},
      "revision":{"type":"integer","minimum":0}
    }}
  }
}`)

type dashboardAddWidgetArgs struct {
	OperationID  string                    `json:"operationId"`
	BaseRevision int                       `json:"baseRevision"`
	Widget       capabilityDashboardWidget `json:"widget"`
}

type dashboardAddWidgetTool struct {
	apply func(context.Context, capabilityDashboardAction) (string, error)
}

func (dashboardAddWidgetTool) Name() string { return dashboardAddWidgetToolName }

func (dashboardAddWidgetTool) Description() string {
	return "Add one validated widget to the Dashboard bound by the Host to this capability session. The Host owns scope and storage; provide only the expected revision, a stable operation ID, and the widget. Use only the widget type values enumerated by the schema. This tool writes the visible capability side panel, so surface must be research-canvas. props.kind must exactly equal widget.type. For a text-like result, use type research.summary, status ready, and props with kind research.summary plus eyebrow, conclusion, confidence (0 to 100), summary, metrics (an array), and findings (an array)."
}

func (dashboardAddWidgetTool) Schema() json.RawMessage { return dashboardAddWidgetToolSchema }

func (dashboardAddWidgetTool) ReadOnly() bool { return false }

func (t dashboardAddWidgetTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	if t.apply == nil {
		return "", errors.New("dashboard_add_widget: capability session is unavailable")
	}
	var args dashboardAddWidgetArgs
	if err := decodeCapabilityJSON(raw, &args); err != nil {
		return "", fmt.Errorf("dashboard_add_widget: invalid arguments: %w", err)
	}
	action := capabilityDashboardAction{
		Type: "dashboard.add_widget", OperationID: strings.TrimSpace(args.OperationID),
		BaseRevision: args.BaseRevision, At: time.Now().UTC().Format(time.RFC3339Nano), Widget: &args.Widget,
	}
	return t.apply(ctx, action)
}

func (a *App) capabilitySessionTools(_ context.Context, workspaceRoot, sessionPath string) ([]tool.Tool, error) {
	stateRoot := a.currentProfilePathResolver(strings.TrimSpace(workspaceRoot)).MemoryUserDir()
	return a.capabilitySessionToolsAtRoot(stateRoot, sessionPath)
}

func (a *App) capabilitySessionToolsAtRoot(stateRoot, sessionPath string) ([]tool.Tool, error) {
	identity := capabilityStableSessionIdentity(sessionPath)
	if identity == "" {
		return nil, nil
	}
	var binding capabilitySessionBinding
	if err := readCapabilityJSON(capabilityStableBindingPath(stateRoot, identity), &binding); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read capability session binding: %w", err)
	}
	if binding.SessionIdentity != identity || validateCapabilitySessionBinding(binding) != nil {
		return nil, errors.New("capability session binding is invalid")
	}
	pkg, found, err := resolveCapabilityPackage(binding.Scope.CapabilityID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	if binding.PackageVersion != pkg.Manifest.Version || binding.PackageDigest != pkg.ContentDigest || binding.InstructionHash != pkg.InstructionHash {
		return nil, errors.New("capability package pin mismatch")
	}
	pinned := binding
	scope := capabilitySessionScope(binding)
	writer := dashboardAddWidgetTool{apply: func(ctx context.Context, action capabilityDashboardAction) (string, error) {
		return a.applyCapabilityDashboardActionForStableBinding(ctx, stateRoot, identity, pinned, scope, action)
	}}
	var tools []tool.Tool
	if pkg.AllowsHostAction("dashboard.add_widget") {
		tools = append(tools, writer)
	}
	if len(pkg.Skills) > 0 {
		tools = append(tools, newCapabilitySkillTool(stateRoot, identity, pinned, pkg))
	}
	return tools, nil
}

func (a *App) applyCapabilityDashboardActionForStableBinding(ctx context.Context, stateRoot, identity string, pinned capabilitySessionBinding, scope CapabilityTaskScope, action capabilityDashboardAction) (string, error) {
	emitDashboardWriteLifecycle(ctx, action.OperationID, event.DashboardWriteRequested, "requested", "", false)
	a.capabilityDashboardMu.Lock()
	var current capabilitySessionBinding
	if err := readCapabilityJSON(capabilityStableBindingPath(stateRoot, identity), &current); err != nil {
		a.capabilityDashboardMu.Unlock()
		emitDashboardWriteLifecycle(ctx, action.OperationID, event.DashboardWriteFailed, "failed", "requested", true)
		return "", fmt.Errorf("capability binding changed: %w", err)
	}
	if current.SessionIdentity != identity || current.SessionIdentity != pinned.SessionIdentity || current.Scope != pinned.Scope || capabilitySessionScope(current) != scope || validateCapabilitySessionBinding(current) != nil {
		a.capabilityDashboardMu.Unlock()
		emitDashboardWriteLifecycle(ctx, action.OperationID, event.DashboardWriteRejected, "rejected", "requested", false)
		return "", errors.New("capability binding changed")
	}
	result, changed, err := mutateCapabilityDashboardAtRoot(stateRoot, scope, action)
	a.capabilityDashboardMu.Unlock()
	if err != nil {
		kind := event.DashboardWriteFailed
		state := "failed"
		retryable := true
		if strings.Contains(err.Error(), "revision conflict") || strings.Contains(err.Error(), "invalid dashboard") {
			kind, state, retryable = event.DashboardWriteRejected, "rejected", false
		}
		emitDashboardWriteLifecycle(ctx, action.OperationID, kind, state, "requested", retryable)
		return "", err
	}
	emitDashboardWriteLifecycle(ctx, action.OperationID, event.DashboardWriteCommitted, "committed", "requested", false)
	if changed && a.ctx != nil {
		// The event scope is the same immutable scope captured by the session tool.
		runtimeEmitCapabilityDashboardChanged(a, scope)
	}
	return result, nil
}

func emitDashboardWriteLifecycle(ctx context.Context, operationID string, kind event.OperationalKind, state, previous string, retryable bool) {
	emitter, ok := event.OperationalEmitterFrom(ctx)
	if !ok || strings.TrimSpace(operationID) == "" {
		return
	}
	toolCallID, _, _, _ := agent.CallContext(ctx)
	scope := event.OperationalScopeFrom(ctx)
	scope.DashboardWriteID = operationID
	scope.ToolCallID = toolCallID
	emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateDashboardWrite,
		AggregateID:   operationID,
		Scope:         scope,
		Kind:          kind, Stage: "host", Outcome: state,
		Severity:   map[bool]string{true: "error", false: "info"}[retryable],
		UserImpact: map[bool]string{true: "blocked", false: "none"}[retryable],
		Retryable:  retryable,
		Payload:    &event.LifecycleOperationalPayload{State: state, PreviousState: previous},
	})
}

func runtimeEmitCapabilityDashboardChanged(a *App, scope CapabilityTaskScope) {
	// Kept in one helper so tests can exercise the store without a Wails context.
	if a != nil && a.ctx != nil {
		runtime.EventsEmit(a.ctx, capabilityDashboardChangedEvent, scope)
	}
}

var _ tool.Tool = dashboardAddWidgetTool{}
