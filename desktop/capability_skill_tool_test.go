package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/desktop/internal/capabilitypackage"
	"kwu/internal/agent"
	agenttest "kwu/internal/agent/testutil"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

func boundSkillTool(t *testing.T) (tool.Tool, string, string, capabilitySessionBinding) {
	t.Helper()
	root := t.TempDir()
	sessionPath := filepath.Join(root, "sessions", "skill.jsonl")
	identity := capabilityStableSessionIdentity(sessionPath)
	scope := testCapabilityScope()
	binding := capabilitySessionBinding{SchemaVersion: capabilityDashboardSchemaVersion, SessionHash: "skill-tab", SessionIdentity: identity, SessionDocumentID: scope.DocumentID, Scope: scope, UpdatedAt: "2026-09-04T00:00:00Z"}
	if _, err := pinCapabilityPackageBinding(&binding); err != nil {
		t.Fatal(err)
	}
	if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(root, identity), binding); err != nil {
		t.Fatal(err)
	}
	tools, err := (&App{}).capabilitySessionToolsAtRoot(root, sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range tools {
		if candidate.Name() == capabilityLoadSkillToolName {
			return candidate, root, identity, binding
		}
	}
	t.Fatal("bound session has no Skill loader")
	return nil, "", "", capabilitySessionBinding{}
}

func TestCapabilitySkillLoadsOnlyDeclaredNameAndHasNoWrites(t *testing.T) {
	loader, root, identity, _ := boundSkillTool(t)
	path := capabilityStableBindingPath(root, identity)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := loader.Execute(context.Background(), json.RawMessage(`{"name":"crossborder-research-plan"}`))
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Instructions     string
		AllowedTools     []string
		PermissionNotice string
	}
	if err := json.Unmarshal([]byte(result), &body); err != nil {
		t.Fatal(err)
	}
	if !loader.ReadOnly() || !strings.Contains(body.Instructions, "# 跨境研究计划草案") || body.AllowedTools == nil || len(body.AllowedTools) != 0 || !strings.Contains(body.PermissionNotice, "not revocation") {
		t.Fatal("unexpected loader contract")
	}
	for _, raw := range []string{`{}`, `{"name":"../AGENTS.md"}`, `{"name":"global-skill"}`, `{"name":" crossborder-research-plan"}`, `{"name":"crossborder-research-plan","path":"SKILL.md"}`, `{"name":"crossborder-research-plan"} {}`, strings.Repeat("x", 1025)} {
		if _, err := loader.Execute(context.Background(), json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loader.Execute(ctx, json.RawMessage(`{"name":"crossborder-research-plan"}`)); err == nil {
		t.Fatal("ignored cancellation")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("loader mutated binding")
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && path != capabilityStableBindingPath(root, identity) {
			t.Errorf("unexpected write: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCapabilitySkillRevalidatesAllPinsAndScopeAtInvocation(t *testing.T) {
	for name, mutate := range map[string]func(*capabilitySessionBinding){
		"identity":     func(b *capabilitySessionBinding) { b.SessionIdentity = "changed" },
		"capability":   func(b *capabilitySessionBinding) { b.Scope.CapabilityID = "other" },
		"task":         func(b *capabilitySessionBinding) { b.Scope.TaskID = "other" },
		"workspace":    func(b *capabilitySessionBinding) { b.Scope.WorkspaceID = "other" },
		"document":     func(b *capabilitySessionBinding) { b.SessionDocumentID = "other" },
		"version":      func(b *capabilitySessionBinding) { b.PackageVersion = "0.0.0" },
		"digest":       func(b *capabilitySessionBinding) { b.PackageDigest = strings.Repeat("0", 64) },
		"instructions": func(b *capabilitySessionBinding) { b.InstructionHash = strings.Repeat("0", 64) },
		"no pins":      func(b *capabilitySessionBinding) { b.PackageVersion = ""; b.PackageDigest = ""; b.InstructionHash = "" },
	} {
		t.Run(name, func(t *testing.T) {
			loader, root, identity, binding := boundSkillTool(t)
			mutate(&binding)
			if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(root, identity), binding); err != nil {
				t.Fatal(err)
			}
			if _, err := loader.Execute(context.Background(), json.RawMessage(`{"name":"crossborder-research-plan"}`)); err == nil {
				t.Fatal("changed binding was accepted")
			}
		})
	}
}

func TestCapabilitySkillRejectsPackageMismatchAndAbsentBinding(t *testing.T) {
	loader, root, identity, binding := boundSkillTool(t)
	// Both snapshots agree, but neither may override the immutable embedded
	// package. This covers resolution mismatch separately from binding drift.
	binding.PackageDigest = strings.Repeat("0", 64)
	if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(root, identity), binding); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadPinnedCapabilitySkill(root, identity, binding, "crossborder-research-plan"); err == nil {
		t.Fatal("unresolvable package pin accepted")
	}
	tools, err := (&App{}).capabilitySessionToolsAtRoot(root, filepath.Join(root, "sessions", "skill.jsonl"))
	if err == nil || len(tools) != 0 {
		t.Fatal("loader created for mismatched package")
	}
	if err := os.Remove(capabilityStableBindingPath(root, identity)); err != nil {
		t.Fatal(err)
	}
	if _, err := loader.Execute(context.Background(), json.RawMessage(`{"name":"crossborder-research-plan"}`)); err == nil {
		t.Fatal("missing binding accepted")
	}
	tools, err = (&App{}).capabilitySessionToolsAtRoot(root, filepath.Join(root, "sessions", "skill.jsonl"))
	if err != nil || len(tools) != 0 {
		t.Fatal("ordinary session received capability tools")
	}
}

// These are reviewed prompt-contract fixtures, not a model evaluation or a
// runtime business-policy interpreter. No model has produced their expectations.
func TestCapabilityResearchPlanFixturesDocumentRequiredBranches(t *testing.T) {
	for _, name := range []string{"complete", "missing-scope", "private", "execute", "unavailable"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "capability_skills", "research-plan-"+name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				Input                map[string]any `json:"input"`
				ExpectedStatus       string         `json:"expectedStatus"`
				RequiredInstructions []string       `json:"requiredInstructions"`
				Evaluation           string         `json:"evaluation"`
			}
			if err := decodeCapabilityJSON(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			if len(fixture.Input) == 0 || fixture.Evaluation != "static-contract-only" {
				t.Fatal("invalid fixture")
			}
			if fixture.ExpectedStatus != "ready" && fixture.ExpectedStatus != "needs_input" && fixture.ExpectedStatus != "blocked" {
				t.Fatal("invalid expected status")
			}
			if len(fixture.RequiredInstructions) == 0 {
				t.Fatal("fixture must define a future behavioral review rubric")
			}
		})
	}
}

func TestCapabilitySkillGenericLifecycleKeepsBodyOutOfOperationalEvidence(t *testing.T) {
	const sentinel = "CAPABILITY_SKILL_SENTINEL fake-credential C:/private/skill"
	loader := capabilitySkillTool{schema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`), load: func(string) (capabilitypackage.Skill, capabilitypackage.Package, error) {
		return capabilitypackage.Skill{Name: "fixture", Description: "synthetic", Body: sentinel}, capabilitypackage.Package{}, nil
	}}
	reg := tool.NewRegistry()
	reg.Add(loader)
	prov := agenttest.NewMock("offline-skill-fixture",
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "skill-call-one", Name: capabilityLoadSkillToolName, Arguments: `{"name":"fixture"}`}}},
		agenttest.Turn{Text: "done"})
	tabSink := &tabEventSink{tabID: "skill-test", ctx: context.Background()}
	tabSink.runtimeEvents.emit = func(context.Context, string, ...interface{}) {}
	var results []event.Event
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.ToolResult {
			results = append(results, e)
		}
		tabSink.Emit(e)
	})
	session := agent.NewSession("")
	ag := agent.New(prov, reg, session, agent.Options{}, sink)
	ctrl := control.New(control.Options{Runner: ag, Executor: ag, Sink: sink})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ctrl.RunTurn(ctx, "load fixture only"); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !strings.Contains(results[0].Tool.Output, sentinel) {
		t.Fatal("Skill body did not reach tool result")
	}
	messages, err := json.Marshal(session.Snapshot())
	if err != nil || !strings.Contains(string(messages), sentinel) {
		t.Fatal("Skill body did not reach model session")
	}
	evidence, err := tabSink.operationalDiagnosticTrace().MarshalEvidence()
	if err != nil {
		t.Fatal(err)
	}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"skill-test": {ID: "skill-test", sink: tabSink}}}
	exported, err := app.ExportOperationalEvidence("skill-test")
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{string(evidence), exported} {
		for _, forbidden := range []string{sentinel, "fake-credential", "C:/private/skill", `"args"`, `"output"`, `"rawOutput"`} {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("operational evidence leaked %q", forbidden)
			}
		}
		for _, kind := range []string{"tool_call.proposed", "tool_call.running", "tool_call.completed"} {
			if !strings.Contains(payload, kind) {
				t.Fatalf("missing actual lifecycle %q: %s", kind, payload)
			}
		}
	}
}
