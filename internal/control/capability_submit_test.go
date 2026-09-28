package control

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent"
	agenttest "kwu/internal/agent/testutil"
	"kwu/internal/event"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/skill"
	"kwu/internal/tool"
)

type capabilityTestTool string

func (t capabilityTestTool) Name() string            { return string(t) }
func (t capabilityTestTool) Description() string     { return string(t) }
func (t capabilityTestTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (t capabilityTestTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "ok", nil
}
func (t capabilityTestTool) ReadOnly() bool { return true }

type failingCapabilityTestTool struct{ capabilityTestTool }

func (failingCapabilityTestTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "", errors.New("skill unavailable")
}

func TestRegistryForMCPServersRestrictsEachTurn(t *testing.T) {
	base := tool.NewRegistry()
	for _, name := range []string{
		"bash",
		"run_skill",
		"mcp__github__get_pull_request",
		"mcp__playwright__browser_navigate",
		"mcp__context7__resolve_library_id",
	} {
		base.Add(capabilityTestTool(name))
	}

	filtered := registryForMCPServers(base, []string{"github", "playwright"})
	names := filtered.Names()
	for _, want := range []string{
		"bash",
		"run_skill",
		"mcp__github__get_pull_request",
		"mcp__playwright__browser_navigate",
	} {
		if !slices.Contains(names, want) {
			t.Fatalf("filtered registry missing %q: %v", want, names)
		}
	}
	if slices.Contains(names, "mcp__context7__resolve_library_id") {
		t.Fatalf("unselected MCP server leaked into turn registry: %v", names)
	}
}

func TestRegistryForMCPServersExposesNoMCPWithoutSelection(t *testing.T) {
	base := tool.NewRegistry()
	base.Add(capabilityTestTool("bash"))
	base.Add(capabilityTestTool("mcp__github__get_pull_request"))

	names := registryForMCPServers(base, nil).Names()
	if !slices.Equal(names, []string{"bash"}) {
		t.Fatalf("names = %v, want only built-in tools", names)
	}
}

func TestRegistryForCapabilitiesRequiresExactToolAllowlist(t *testing.T) {
	base := tool.NewRegistry()
	for _, name := range []string{"bash", "mcp__github__read", "mcp__github__write"} {
		base.Add(capabilityTestTool(name))
	}

	names := registryForCapabilities(base, []string{"github"}, []string{"mcp__github__read"}).Names()
	if !slices.Contains(names, "bash") || !slices.Contains(names, "mcp__github__read") {
		t.Fatalf("authorized tools missing: %v", names)
	}
	if slices.Contains(names, "mcp__github__write") {
		t.Fatalf("tool outside immutable allowlist leaked into turn: %v", names)
	}
	if got := registryForCapabilities(base, []string{"github"}, nil).Names(); !slices.Equal(got, []string{"bash"}) {
		t.Fatalf("empty allowlist exposed MCP tools: %v", got)
	}
}

func TestRegistryForMCPServersUsesCanonicalServerPrefix(t *testing.T) {
	base := tool.NewRegistry()
	base.Add(capabilityTestTool("bash"))
	expected := plugin.ToolPrefix("图表 Server / Prod") + "draw"
	base.Add(capabilityTestTool(expected))
	base.Add(capabilityTestTool(plugin.ToolPrefix("other") + "draw"))

	names := registryForMCPServers(base, []string{"图表 Server / Prod"}).Names()
	if !slices.Contains(names, expected) {
		t.Fatalf("selected server with normalized name was filtered out: %v", names)
	}
	if slices.Contains(names, plugin.ToolPrefix("other")+"draw") {
		t.Fatalf("unselected MCP server leaked into turn registry: %v", names)
	}
}

func TestSelectedMCPContextOnlyNamesServersWithAvailableTools(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Add(capabilityTestTool(plugin.ToolPrefix("antv-chart") + "generate_column_chart"))

	got := selectedMCPContext([]string{"antv-chart", "missing", "antv-chart"}, registry)
	if !strings.Contains(got, "Enabled for this turn: antv-chart.") {
		t.Fatalf("context missing selected connector: %q", got)
	}
	if strings.Contains(got, "missing") {
		t.Fatalf("context claimed an unavailable connector: %q", got)
	}
	if !strings.Contains(got, "Never claim a connector was used unless its tool call succeeds") {
		t.Fatalf("context missing invocation evidence rule: %q", got)
	}
}

func TestSelectedMCPContextNamesEmptySelection(t *testing.T) {
	got := selectedMCPContext(nil, tool.NewRegistry())
	if !strings.Contains(got, "Enabled for this turn: none.") {
		t.Fatalf("context missing empty connector selection: %q", got)
	}
}

type recordingCapabilityTool struct {
	capabilityTestTool
	calls int
}

func (t *recordingCapabilityTool) Execute(context.Context, json.RawMessage) (string, error) {
	t.calls++
	return `{"chart":"https://example.test/chart.png"}`, nil
}

func TestSubmitCapabilitiesCompletesModelToMCPToolLoop(t *testing.T) {
	registry := tool.NewRegistry()
	chartTool := &recordingCapabilityTool{capabilityTestTool: capabilityTestTool(
		plugin.ToolPrefix("AntV Chart / Prod") + "generate_column_chart",
	)}
	registry.Add(chartTool)
	registry.Add(capabilityTestTool(plugin.ToolPrefix("other") + "search"))

	model := agenttest.NewMock("capability-flow",
		agenttest.Turn{ToolCalls: []provider.ToolCall{{
			ID:        "chart-call",
			Name:      chartTool.Name(),
			Arguments: `{}`,
		}}},
		agenttest.Turn{Text: "图表已生成"},
	)
	executor := agent.New(model, registry, agent.NewSession("system"), agent.Options{}, event.Discard)
	done := make(chan event.Event, 1)
	controller := New(Options{
		Runner:   executor,
		Executor: executor,
		Registry: registry,
		Sink: event.FuncSink(func(evt event.Event) {
			if evt.Kind == event.TurnDone {
				done <- evt
			}
		}),
	})

	controller.SubmitCapabilities(CapabilityTurn{
		Input:          "生成柱状图",
		Display:        "生成柱状图",
		MCPServerNames: []string{"AntV Chart / Prod"},
	})

	select {
	case evt := <-done:
		if evt.Err != nil {
			t.Fatalf("capability turn failed: %v", evt.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for capability turn")
	}
	if chartTool.calls != 1 {
		t.Fatalf("MCP tool calls = %d, want 1", chartTool.calls)
	}
	requests := model.Requests()
	if len(requests) != 2 {
		t.Fatalf("model requests = %d, want tool selection and final response", len(requests))
	}
	firstToolNames := make([]string, 0, len(requests[0].Tools))
	for _, schema := range requests[0].Tools {
		firstToolNames = append(firstToolNames, schema.Name)
	}
	if !slices.Contains(firstToolNames, chartTool.Name()) {
		t.Fatalf("selected MCP tool missing from model request: %v", firstToolNames)
	}
	if slices.Contains(firstToolNames, plugin.ToolPrefix("other")+"search") {
		t.Fatalf("unselected MCP tool leaked into model request: %v", firstToolNames)
	}
	userInput := requests[0].Messages[len(requests[0].Messages)-1].Content
	if !strings.Contains(userInput, "Enabled for this turn: AntV Chart / Prod.") ||
		!strings.HasSuffix(userInput, "生成柱状图") {
		t.Fatalf("model input missing hidden connector context or user text: %q", userInput)
	}
}

func TestSubmitCapabilitiesExpandsNativeSlashSkillWithSelectedMCP(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Add(capabilityTestTool(plugin.ToolPrefix("antv-chart") + "generate_column_chart"))
	model := agenttest.NewMock("slash-skill-capability",
		agenttest.Turn{Text: "已完成检查"},
	)
	executor := agent.New(model, registry, agent.NewSession("system"), agent.Options{}, event.Discard)
	done := make(chan event.Event, 1)
	controller := New(Options{
		Runner:   executor,
		Executor: executor,
		Registry: registry,
		Skills: []skill.Skill{{
			Name:        "review-chart",
			Description: "Review a chart request",
			Body:        "Follow the real chart review playbook.",
		}},
		Sink: event.FuncSink(func(evt event.Event) {
			if evt.Kind == event.TurnDone {
				done <- evt
			}
		}),
	})

	controller.SubmitCapabilities(CapabilityTurn{
		Input:          "/review-chart monthly sales",
		Display:        "/review-chart monthly sales",
		MCPServerNames: []string{"antv-chart"},
	})

	select {
	case evt := <-done:
		if evt.Err != nil {
			t.Fatalf("slash skill capability turn failed: %v", evt.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for slash skill capability turn")
	}
	requests := model.Requests()
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want 1", len(requests))
	}
	userInput := requests[0].Messages[len(requests[0].Messages)-1].Content
	if !strings.Contains(userInput, "Follow the real chart review playbook.") ||
		!strings.Contains(userInput, "Arguments: monthly sales") {
		t.Fatalf("native slash skill was not expanded before the model turn: %q", userInput)
	}
	if !strings.Contains(userInput, "Enabled for this turn: antv-chart.") {
		t.Fatalf("selected connector context missing after slash expansion: %q", userInput)
	}
	if strings.Contains(userInput, "/review-chart monthly sales") {
		t.Fatalf("raw slash invocation leaked to the model instead of RunSkill expansion: %q", userInput)
	}
}

func TestPrepareCapabilitiesValidatesWithoutExecutingSkill(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Add(failingCapabilityTestTool{"run_skill"})
	controller := New(Options{
		Registry: registry,
		Skills: []skill.Skill{{
			Name: "code-review",
			Body: "Review the requested change.",
		}},
		Sink: event.Discard,
	})

	turn := CapabilityTurn{Input: "review this", SkillNames: []string{"code-review"}}
	got, err := controller.PrepareCapabilities(context.Background(), turn)
	if err != nil {
		t.Fatalf("PrepareCapabilities error = %v", err)
	}
	if got.Input != turn.Input || !slices.Equal(got.SkillNames, turn.SkillNames) {
		t.Fatalf("PrepareCapabilities changed turn = %#v, want %#v", got, turn)
	}
}

func TestPrepareCapabilitiesValidatesAndClonesHostCommandEnv(t *testing.T) {
	controller := New(Options{Registry: tool.NewRegistry(), Sink: event.Discard})
	root := t.TempDir()
	env := map[string]string{
		tool.CommandEnvPATH:     filepath.Join(root, "node") + string(filepath.ListSeparator) + filepath.Join(root, "cli"),
		"CONNECTOR_CONFIG_HOME": filepath.Join(root, "config"),
		"CONNECTOR_NO_UPDATE":   "1", "CONNECTOR_NO_SKILLS_NOTICE": "1",
	}
	turn, err := controller.PrepareCapabilities(context.Background(), CapabilityTurn{Input: "test", CommandEnv: env})
	if err != nil {
		t.Fatal(err)
	}
	env["CONNECTOR_NO_UPDATE"] = "changed"
	if turn.CommandEnv["CONNECTOR_NO_UPDATE"] != "1" {
		t.Fatal("prepared turn retained caller environment map")
	}
	turn.CommandEnv["BAD-NAME"] = "value"
	if _, err := controller.PrepareCapabilities(context.Background(), turn); err == nil {
		t.Fatal("expected invalid command environment key rejection")
	}
}

func TestPrepareCapabilitiesRejectsUnknownSkill(t *testing.T) {
	controller := New(Options{Registry: tool.NewRegistry(), Sink: event.Discard})

	_, err := controller.PrepareCapabilities(context.Background(), CapabilityTurn{
		Input:      "review this",
		SkillNames: []string{"code-review"},
	})
	if err == nil || err.Error() != `selected skill "code-review" is unavailable` {
		t.Fatalf("PrepareCapabilities error = %v", err)
	}
}

func TestSubmitCapabilitiesExpandsSubagentSkillInlineWithoutExecutingRunSkill(t *testing.T) {
	registry := tool.NewRegistry()
	runSkill := &recordingCapabilityTool{capabilityTestTool: "run_skill"}
	registry.Add(runSkill)

	model := agenttest.NewMock("selected-subagent", agenttest.Turn{Text: "done"})
	executor := agent.New(model, registry, agent.NewSession("system"), agent.Options{}, event.Discard)
	done := make(chan event.Event, 1)
	controller := New(Options{
		Runner:   executor,
		Executor: executor,
		Registry: registry,
		Skills: []skill.Skill{{
			Name:  "explore",
			Body:  "Inspect the repository using the exploration playbook.",
			RunAs: skill.RunSubagent,
		}},
		Sink: event.FuncSink(func(evt event.Event) {
			if evt.Kind == event.TurnDone {
				done <- evt
			}
		}),
	})

	controller.SubmitCapabilities(CapabilityTurn{
		Input:      "inspect the repository",
		Display:    "inspect the repository",
		SkillNames: []string{"explore"},
	})

	select {
	case evt := <-done:
		if evt.Err != nil {
			t.Fatalf("capability turn failed: %v", evt.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for capability turn")
	}
	if runSkill.calls != 0 {
		t.Fatalf("run_skill calls = %d, want 0 for native slash-style expansion", runSkill.calls)
	}
	requests := model.Requests()
	if len(requests) != 1 {
		t.Fatalf("model requests = %d, want 1", len(requests))
	}
	userInput := requests[0].Messages[len(requests[0].Messages)-1].Content
	for _, want := range []string{
		"Inspect the repository using the exploration playbook.",
		"Arguments: inspect the repository",
	} {
		if !strings.Contains(userInput, want) {
			t.Fatalf("selected skill was not expanded with native slash semantics; missing %q in %q", want, userInput)
		}
	}
}

func TestExpandSelectedSkillsExpandsEveryUniqueSkill(t *testing.T) {
	controller := New(Options{
		Skills: []skill.Skill{
			{Name: "review", Description: "Review changes", Body: "Review playbook"},
			{Name: "security", Description: "Check security", Body: "Security playbook"},
		},
		Sink: event.Discard,
	})

	got, err := controller.expandSelectedSkills(
		"检查这次修改",
		[]string{"review", "security", "review"},
	)
	if err != nil {
		t.Fatalf("expandSelectedSkills: %v", err)
	}
	if strings.Count(got, "<selected-skill name=") != 2 {
		t.Fatalf("selected skill count = %d, want 2: %q", strings.Count(got, "<selected-skill name="), got)
	}
	for _, want := range []string{"Review playbook", "Security playbook", "Arguments: 检查这次修改"} {
		if !strings.Contains(got, want) {
			t.Fatalf("selected skill output missing %q: %q", want, got)
		}
	}
	if !strings.HasSuffix(got, "检查这次修改") {
		t.Fatalf("original task was not preserved after skill expansion: %q", got)
	}
}
