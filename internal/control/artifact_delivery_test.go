package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/agent"
	agenttest "kwu/internal/agent/testutil"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
	"kwu/internal/scene"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
)

func structuredDeliverFilesResult(paths ...string) string {
	if paths == nil {
		paths = []string{}
	}
	encoded, _ := json.Marshal(map[string]any{
		"type":  builtin.DeliverFilesResultType,
		"paths": paths,
	})
	return string(encoded)
}

func TestFinalizeArtifactDeliveryUsesExactTurnAndDeliveredFiles(t *testing.T) {
	workspaceRoot := t.TempDir()
	goodPath := filepath.Join(workspaceRoot, "deliverable.md")
	failedExistingPath := filepath.Join(workspaceRoot, "failed-existing.md")
	outsidePath := filepath.Join(t.TempDir(), "outside.md")
	directoryPath := filepath.Join(workspaceRoot, "folder")
	for path, body := range map[string]string{failedExistingPath: "old", outsidePath: "outside"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := os.Mkdir(directoryPath, 0o755); err != nil {
		t.Fatalf("mkdir candidate: %v", err)
	}
	observation := agent.NewWorkspaceArtifactObservation(workspaceRoot)
	if err := os.WriteFile(goodPath, []byte("good"), 0o644); err != nil {
		t.Fatalf("write %s: %v", goodPath, err)
	}
	args := func(path string) string {
		raw, err := json.Marshal(map[string]string{"path": path, "content": "body"})
		if err != nil {
			t.Fatalf("marshal args: %v", err)
		}
		return string(raw)
	}

	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "old turn"})
	session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "old final"})
	const artifactTurnID = "artifact-turn-current"
	session.BeginArtifactDeliveryTurn(artifactTurnID)
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create deliverable"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
		{ID: "good", Name: "write_file", Arguments: args(goodPath)},
		{ID: "failed", Name: "write_file", Arguments: args(failedExistingPath)},
		{ID: "outside", Name: "write_file", Arguments: args(outsidePath)},
		{ID: "directory", Name: "write_file", Arguments: args(directoryPath)},
		{ID: "edit", Name: "edit_file", Arguments: args(goodPath)},
	}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "good", Name: "write_file", Content: "wrote file", ToolArtifacts: []provider.ToolArtifact{{Path: goodPath, Source: agent.SubagentArtifactSourceWriter}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "failed", Name: "write_file", Content: "error: denied"})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "outside", Name: "write_file", Content: "wrote file"})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "directory", Name: "write_file", Content: "wrote file"})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "edit", Name: "edit_file", Content: "edited file"})
	manifestArgs, err := json.Marshal(map[string][]string{"paths": []string{goodPath}})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "manifest", Name: "deliver_files", Arguments: string(manifestArgs)}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "manifest", Name: "deliver_files", Content: structuredDeliverFilesResult(goodPath)})
	session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "[ghost](ghost.md)"})
	session.EndArtifactDeliveryTurn(artifactTurnID)

	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	controller := New(Options{Executor: executor, WorkspaceRoot: workspaceRoot, Sink: event.Discard})
	t.Cleanup(controller.Close)
	delivery, ok := controller.finalizeArtifactDelivery(artifactTurnID, 100, observation)
	if !ok || delivery.DeliveredAt != 100 || len(delivery.Paths) != 1 || delivery.Paths[0] != "deliverable.md" {
		t.Fatalf("delivery = %#v, %v", delivery, ok)
	}
	history := controller.History()
	if history[2].ArtifactDelivery != nil {
		t.Fatalf("old final received current delivery: %#v", history[2].ArtifactDelivery)
	}
	if got := history[len(history)-1].ArtifactDelivery; got == nil || got.DeliveredAt != 100 {
		t.Fatalf("current final delivery = %#v", got)
	}

	replayed, ok := controller.finalizeArtifactDelivery(artifactTurnID, 200, observation)
	if !ok || replayed.DeliveredAt != 100 || len(replayed.Paths) != 1 || replayed.Paths[0] != "deliverable.md" {
		t.Fatalf("replayed delivery = %#v, %v", replayed, ok)
	}
}

func TestFinalizeArtifactDeliveryCommitsExplicitEmptyContract(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "README.md"), []byte("updated"), 0o644); err != nil {
		t.Fatal(err)
	}
	observation := agent.NewWorkspaceArtifactObservation(workspaceRoot)
	session := agent.NewSession("system")
	const artifactTurnID = "artifact-turn-empty"
	session.BeginArtifactDeliveryTurn(artifactTurnID)
	session.Add(provider.Message{Role: provider.RoleUser, Content: "edit source"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "edit", Name: "edit_file", Arguments: `{"path":"README.md"}`}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "edit", Name: "edit_file", Content: "edited"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "deliver-empty", Name: "deliver_files", Arguments: `{"paths":[]}`}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "deliver-empty", Name: "deliver_files", Content: structuredDeliverFilesResult()})
	session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "README updated."})
	session.EndArtifactDeliveryTurn(artifactTurnID)
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	controller := New(Options{Executor: executor, WorkspaceRoot: workspaceRoot, Sink: event.Discard})
	t.Cleanup(controller.Close)

	delivery, ok := controller.finalizeArtifactDelivery(artifactTurnID, 100, observation)
	if !ok || delivery.Paths == nil || len(delivery.Paths) != 0 {
		t.Fatalf("empty delivery = %#v, %v", delivery, ok)
	}
}

func TestFinalizeArtifactDeliveryUsesExactAgentLedgerAfterManifestCompaction(t *testing.T) {
	workspaceRoot := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: workspaceRoot}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	prov := agenttest.NewMock("artifact-ledger",
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "write", Name: "write_file", Arguments: `{"path":"report.md","content":"report"}`}}},
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`}}},
		agenttest.Turn{Text: "report complete"},
	)
	session := agent.NewSession("system")
	const artifactTurnID = "compacted-ledger-turn"
	session.BeginArtifactDeliveryTurn(artifactTurnID)
	executor := agent.New(prov, registry, session, agent.Options{}, event.Discard)
	ctx := agent.WithWorkspaceArtifactObservation(context.Background(), agent.NewWorkspaceArtifactObservation(workspaceRoot))
	if err := executor.Run(ctx, "create report"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Model a real compaction outcome that folded the tool transcript while the
	// host ledger remained in memory. The exact-turn ledger, not the summary,
	// must remain the commit authority.
	var compacted []provider.Message
	for _, message := range session.Snapshot() {
		if message.Role == provider.RoleSystem || message.MessageKind == provider.MessageKindUncommittedFinalAnswer {
			compacted = append(compacted, message)
		}
	}
	compacted = append(compacted[:1], append([]provider.Message{{Role: provider.RoleUser, Content: "<compaction-summary>folded</compaction-summary>"}}, compacted[1:]...)...)
	session.Replace(compacted)
	if got := agent.ArtifactDeliveryCandidates(session.Snapshot()); len(got) != 0 {
		t.Fatalf("compacted transcript unexpectedly retained manifest: %v", got)
	}

	controller := New(Options{Executor: executor, WorkspaceRoot: workspaceRoot, Sink: event.Discard})
	t.Cleanup(controller.Close)
	delivery, ok := controller.finalizeArtifactDelivery(artifactTurnID, 100, agent.NewWorkspaceArtifactObservation(workspaceRoot))
	if !ok || len(delivery.Paths) != 1 || delivery.Paths[0] != "report.md" {
		t.Fatalf("delivery after compaction = %#v, %v", delivery, ok)
	}
}

func TestFinalizeArtifactDeliveryDoesNotReviveInvalidatedLedgerManifest(t *testing.T) {
	workspaceRoot := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: workspaceRoot}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	prov := agenttest.NewMock("invalidated-artifact-ledger",
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "write-first", Name: "write_file", Arguments: `{"path":"report.md","content":"first"}`}}},
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`}}},
		agenttest.Turn{Text: "first run complete"},
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "write-second", Name: "write_file", Arguments: `{"path":"report.md","content":"second"}`}}},
	)
	session := agent.NewSession("system")
	const artifactTurnID = "invalidated-ledger-turn"
	session.BeginArtifactDeliveryTurn(artifactTurnID)
	executor := agent.New(prov, registry, session, agent.Options{}, event.Discard)
	ctx := agent.WithWorkspaceArtifactObservation(context.Background(), agent.NewWorkspaceArtifactObservation(workspaceRoot))
	if err := executor.Run(ctx, "create report"); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := executor.Run(ctx, "revise report without a new manifest"); err == nil {
		t.Fatal("continued Run unexpectedly completed without a scripted re-verification")
	}
	if got := agent.ArtifactDeliveryCandidates(session.Snapshot()); len(got) != 1 || filepath.Clean(got[0]) != filepath.Join(workspaceRoot, "report.md") {
		t.Fatalf("transcript setup lost stale manifest: %v", got)
	}

	controller := New(Options{Executor: executor, WorkspaceRoot: workspaceRoot, Sink: event.Discard})
	t.Cleanup(controller.Close)
	delivery, ok := controller.finalizeArtifactDelivery(artifactTurnID, 100, agent.NewWorkspaceArtifactObservation(workspaceRoot))
	if !ok || delivery.Paths == nil || len(delivery.Paths) != 0 {
		t.Fatalf("invalidated ledger fell back to stale transcript: %#v, %v", delivery, ok)
	}
}

func TestResolvedArtifactDeliveryPathsCapsUniqueCards(t *testing.T) {
	root := t.TempDir()
	candidates := make([]string, 0, provider.MaxArtifactDeliveryPaths+8)
	for i := 0; i < provider.MaxArtifactDeliveryPaths+8; i++ {
		name := fmt.Sprintf("report-%02d.md", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("report"), 0o644); err != nil {
			t.Fatal(err)
		}
		candidates = append(candidates, name)
	}

	paths := resolvedArtifactDeliveryPaths(root, candidates)
	if len(paths) != provider.MaxArtifactDeliveryPaths {
		t.Fatalf("resolved path count = %d, want %d", len(paths), provider.MaxArtifactDeliveryPaths)
	}
}

func TestResolvedArtifactDeliveryPathsAcceptsExplicitWorkspaceRegularFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".hidden-parent", "workspace")
	for path, content := range map[string]string{
		"report.docx":                           "report",
		"output.json":                           "json",
		"result.yaml":                           "yaml",
		"artifact.custom":                       "custom",
		"docs/workbuddy-report.md":              "document",
		"assets/chart.png":                      "image",
		"public/final.pdf":                      "published document",
		"static/export.json":                    "export",
		"main.go":                               "source",
		"package.json":                          "project config",
		".support/trace.txt":                    "trace",
		".codex/state.json":                     "tool state",
		"build/generated.pdf":                   "build",
		"coverage/report.html":                  "coverage",
		"logs/run.txt":                          "log",
		"internal/generated.json":               "source",
		"README.md":                             "source",
		".customer-delivery/manifest.private":   "explicit",
		".customer-delivery/docs/report.custom": "explicit document",
		".customer-delivery/build/output.json":  "escaped structure",
		".customer-delivery/src/main.go":        "escaped source",
		".customer-delivery/logs/run.log":       "escaped log",
	} {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	paths := resolvedArtifactDeliveryPaths(root, []string{
		filepath.Join(root, "report.docx"),
		filepath.Join(root, "output.json"),
		filepath.Join(root, "result.yaml"),
		filepath.Join(root, "artifact.custom"),
		filepath.Join(root, "docs", "workbuddy-report.md"),
		filepath.Join(root, "assets", "chart.png"),
		filepath.Join(root, "public", "final.pdf"),
		filepath.Join(root, "static", "export.json"),
		filepath.Join(root, "main.go"),
		filepath.Join(root, "package.json"),
		filepath.Join(root, ".support", "trace.txt"),
		filepath.Join(root, ".codex", "state.json"),
		filepath.Join(root, "build", "generated.pdf"),
		filepath.Join(root, "coverage", "report.html"),
		filepath.Join(root, "logs", "run.txt"),
		filepath.Join(root, "internal", "generated.json"),
		filepath.Join(root, "README.md"),
		filepath.Join(root, ".customer-delivery", "manifest.private"),
		filepath.Join(root, ".customer-delivery", "docs", "report.custom"),
		filepath.Join(root, ".customer-delivery", "build", "output.json"),
		filepath.Join(root, ".customer-delivery", "src", "main.go"),
		filepath.Join(root, ".customer-delivery", "logs", "run.log"),
	})
	want := []string{
		"report.docx",
		"output.json",
		"result.yaml",
		"artifact.custom",
		"docs/workbuddy-report.md",
		"assets/chart.png",
		"public/final.pdf",
		"static/export.json",
		"main.go",
		"package.json",
		".support/trace.txt",
		".codex/state.json",
		"build/generated.pdf",
		"coverage/report.html",
		"logs/run.txt",
		"internal/generated.json",
		"README.md",
		".customer-delivery/manifest.private",
		".customer-delivery/docs/report.custom",
		".customer-delivery/build/output.json",
		".customer-delivery/src/main.go",
		".customer-delivery/logs/run.log",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("resolved paths = %v, want %v", paths, want)
	}
}

type artifactDeliveryRunner struct {
	session       *agent.Session
	workspaceRoot string
}

type draftArtifactDeliveryRunner struct {
	executor      *agent.Agent
	workspaceRoot string
}

func (r draftArtifactDeliveryRunner) Run(_ context.Context, input string) error {
	session := r.executor.Session()
	path := filepath.Join(r.workspaceRoot, "draft-report.md")
	session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "write-draft-report", Name: "write_file", Arguments: `{"path":"draft-report.md","content":"report"}`,
	}}})
	if err := os.WriteFile(path, []byte("report"), 0o644); err != nil {
		return err
	}
	session.Add(provider.Message{
		Role: provider.RoleTool, ToolCallID: "write-draft-report", Name: "write_file", Content: "wrote draft-report.md",
		ToolArtifacts: []provider.ToolArtifact{{Path: path, Source: agent.SubagentArtifactSourceWriter}},
	})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "manifest-draft-report", Name: "deliver_files", Arguments: `{"paths":["draft-report.md"]}`,
	}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "manifest-draft-report", Name: "deliver_files", Content: structuredDeliverFilesResult(path)})
	session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "Draft report complete."})
	return nil
}

func (r artifactDeliveryRunner) Run(_ context.Context, input string) error {
	path := filepath.Join(r.workspaceRoot, "joke.md")
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "write", Name: "write_file", Arguments: `{"path":"joke.md","content":"joke"}`,
	}}})
	if err := os.WriteFile(path, []byte("joke"), 0o644); err != nil {
		return err
	}
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "write", Name: "write_file", Content: "wrote joke.md", ToolArtifacts: []provider.ToolArtifact{{Path: path, Source: agent.SubagentArtifactSourceWriter}}})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["joke.md"]}`,
	}}})
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "manifest", Name: "deliver_files", Content: structuredDeliverFilesResult(path)})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "完成 `joke.md`。"})
	return nil
}

type compactingArtifactDeliveryRunner struct {
	session       *agent.Session
	workspaceRoot string
}

type bashArtifactDeliveryRunner struct {
	session       *agent.Session
	workspaceRoot string
}

type linkedMarkdownArtifactRunner struct {
	session       *agent.Session
	workspaceRoot string
}

type staleStampedArtifactRunner struct {
	session       *agent.Session
	workspaceRoot string
}

type replacingArtifactRunner struct {
	session       *agent.Session
	workspaceRoot string
	useCurrentID  bool
}

type explicitNestedArtifactRunner struct {
	session       *agent.Session
	workspaceRoot string
	relativePath  string
}

type artifactFatReadTool struct{}

func (artifactFatReadTool) Name() string            { return "fat_read" }
func (artifactFatReadTool) Description() string     { return "Return a large read-only result." }
func (artifactFatReadTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (artifactFatReadTool) ReadOnly() bool          { return true }
func (artifactFatReadTool) Execute(context.Context, json.RawMessage) (string, error) {
	return strings.Repeat("large read-only context ", 2_000), nil
}

func (r explicitNestedArtifactRunner) Run(_ context.Context, input string) error {
	absolute := filepath.Join(r.workspaceRoot, filepath.FromSlash(r.relativePath))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return err
	}
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "write-nested", Name: "write_file", Arguments: `{"path":"docs/workbuddy-report.md","content":"report"}`,
	}}})
	if err := os.WriteFile(absolute, []byte("report"), 0o644); err != nil {
		return err
	}
	r.session.Add(provider.Message{
		Role: provider.RoleTool, ToolCallID: "write-nested", Name: "write_file", Content: "wrote nested deliverable",
		ToolArtifacts: []provider.ToolArtifact{{Path: absolute, Source: agent.SubagentArtifactSourceWriter}},
	})
	manifestArgs, err := json.Marshal(map[string][]string{"paths": []string{r.relativePath}})
	if err != nil {
		return err
	}
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "manifest-nested", Name: "deliver_files", Arguments: string(manifestArgs),
	}}})
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "manifest-nested", Name: "deliver_files", Content: structuredDeliverFilesResult(absolute)})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "WorkBuddy report complete."})
	return nil
}

func (r linkedMarkdownArtifactRunner) Run(_ context.Context, input string) error {
	path := filepath.Join(r.workspaceRoot, "report.md")
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "write-report", Name: "write_file", Arguments: `{"path":"report.md","content":"report"}`,
	}}})
	if err := os.WriteFile(path, []byte("report"), 0o644); err != nil {
		return err
	}
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "write-report", Name: "write_file", Content: "wrote report.md", ToolArtifacts: []provider.ToolArtifact{{Path: path, Source: agent.SubagentArtifactSourceWriter}}})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`,
	}}})
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "manifest", Name: "deliver_files", Content: structuredDeliverFilesResult(path)})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "[Download report](report.md)"})
	return nil
}

func (r staleStampedArtifactRunner) Run(_ context.Context, input string) error {
	const staleTurnID = "stale-runner-turn"
	forged := provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"ghost.md"}, DeliveredAt: 1,
	}
	add := func(message provider.Message) {
		message.ArtifactTurnID = staleTurnID
		r.session.Add(message)
	}
	path := filepath.Join(r.workspaceRoot, "report.md")
	add(provider.Message{Role: provider.RoleUser, Content: input})
	add(provider.Message{
		Role: provider.RoleAssistant, ArtifactDelivery: &forged,
		ToolCalls: []provider.ToolCall{{
			ID: "write-report", Name: "write_file", Arguments: `{"path":"report.md","content":"report"}`,
		}},
	})
	if err := os.WriteFile(path, []byte("report"), 0o644); err != nil {
		return err
	}
	add(provider.Message{
		Role: provider.RoleTool, ToolCallID: "write-report", Name: "write_file", Content: "wrote report.md",
		ToolArtifacts: []provider.ToolArtifact{{Path: path, Source: agent.SubagentArtifactSourceWriter}},
	})
	add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`,
	}}})
	add(provider.Message{Role: provider.RoleTool, ToolCallID: "manifest", Name: "deliver_files", Content: structuredDeliverFilesResult(path)})
	add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer,
		Content: "[Download report](report.md)", ArtifactDelivery: &forged,
	})
	return nil
}

func (r replacingArtifactRunner) Run(_ context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	messages := r.session.Snapshot()
	stamp := "stale-runner-turn"
	if r.useCurrentID {
		stamp = messages[len(messages)-1].ArtifactTurnID
	}
	path := filepath.Join(r.workspaceRoot, "report.md")
	if err := os.WriteFile(path, []byte("report"), 0o644); err != nil {
		return err
	}
	forged := provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"ghost.md"}, DeliveredAt: 1,
	}
	messages = append(messages,
		provider.Message{
			Role: provider.RoleAssistant, ArtifactTurnID: stamp, ArtifactDelivery: &forged,
			ToolCalls: []provider.ToolCall{{ID: "write-report", Name: "write_file", Arguments: `{"path":"report.md","content":"report"}`}},
		},
		provider.Message{
			Role: provider.RoleTool, ToolCallID: "write-report", Name: "write_file", Content: "wrote report.md", ArtifactTurnID: stamp,
			ToolArtifacts: []provider.ToolArtifact{{Path: path, Source: agent.SubagentArtifactSourceWriter}},
		},
		provider.Message{
			Role: provider.RoleAssistant, ArtifactTurnID: stamp,
			ToolCalls: []provider.ToolCall{{ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`}},
		},
		provider.Message{Role: provider.RoleTool, ToolCallID: "manifest", Name: "deliver_files", Content: structuredDeliverFilesResult(path), ArtifactTurnID: stamp},
		provider.Message{
			Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "[Download report](report.md)",
			ArtifactTurnID: stamp, ArtifactDelivery: &forged,
		},
	)
	r.session.Replace(messages)
	return nil
}

type inspectAfterRunTaskLifecycle struct {
	afterRun func() error
	result   error
}

type inspectingArtifactTaskLifecycleStore struct {
	*controlTaskLifecycleStore
	beforeUpdate func(modelbilling.TaskStateUpdate) error
}

func (s *inspectingArtifactTaskLifecycleStore) UpdateState(ctx context.Context, update modelbilling.TaskStateUpdate) error {
	if s.beforeUpdate != nil {
		if err := s.beforeUpdate(update); err != nil {
			return err
		}
	}
	return s.controlTaskLifecycleStore.UpdateState(ctx, update)
}

func (l inspectAfterRunTaskLifecycle) RunTask(ctx context.Context, run func(context.Context) error) error {
	if err := run(ctx); err != nil {
		return err
	}
	if l.afterRun != nil {
		if err := l.afterRun(); err != nil {
			return err
		}
	}
	return l.result
}

type countingRunner struct {
	calls int
}

func (r *countingRunner) Run(context.Context, string) error {
	r.calls++
	return nil
}

func (r bashArtifactDeliveryRunner) Run(_ context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "bash-create", Name: "bash", Arguments: `{"command":"generate report"}`,
	}}})
	if err := os.WriteFile(filepath.Join(r.workspaceRoot, "report.docx"), []byte("document"), 0o644); err != nil {
		return err
	}
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "bash-create", Name: "bash", Content: "generation complete", ToolArtifacts: []provider.ToolArtifact{{Path: filepath.Join(r.workspaceRoot, "report.docx"), Source: agent.SubagentArtifactSourceSnapshot}}})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["report.docx"]}`,
	}}})
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "manifest", Name: "deliver_files", Content: structuredDeliverFilesResult(filepath.Join(r.workspaceRoot, "report.docx"))})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "报告已完成。"})
	return nil
}

type overwriteSourceRunner struct {
	session       *agent.Session
	workspaceRoot string
}

func (r overwriteSourceRunner) Run(_ context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "write-source", Name: "write_file", Arguments: `{"path":"README.md","content":"after"}`,
	}}})
	if err := os.WriteFile(filepath.Join(r.workspaceRoot, "README.md"), []byte("after"), 0o644); err != nil {
		return err
	}
	r.session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "write-source", Name: "write_file", Content: "wrote README.md"})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "README updated."})
	return nil
}

func (r compactingArtifactDeliveryRunner) Run(ctx context.Context, input string) error {
	base := artifactDeliveryRunner{session: r.session, workspaceRoot: r.workspaceRoot}
	if err := base.Run(ctx, input); err != nil {
		return err
	}

	messages := r.session.Snapshot()
	artifactTurnID := messages[len(messages)-1].ArtifactTurnID
	if artifactTurnID == "" {
		return errors.New("current artifact turn was not stamped")
	}
	compacted := make([]provider.Message, 0, len(messages))
	if messages[0].Role == provider.RoleSystem {
		compacted = append(compacted, messages[0])
	}
	compacted = append(compacted, provider.Message{Role: provider.RoleUser, Content: "Summary of earlier conversation."})
	for _, message := range messages {
		if message.ArtifactTurnID == artifactTurnID {
			compacted = append(compacted, message)
		}
	}
	r.session.Replace(compacted)
	return nil
}

func TestRunTurnPersistsArtifactDeliveryBeforeTurnDone(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	session := agent.NewSession("system")
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	controller := New(Options{
		Runner: artifactDeliveryRunner{session: session, workspaceRoot: workspaceRoot}, Executor: executor,
		WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind != event.TurnDone {
				return
			}
			terminal = e
			deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
			if err != nil || len(deliveries) != 1 || len(deliveries[0].Paths) != 1 || deliveries[0].Paths[0] != "joke.md" {
				t.Errorf("persisted delivery at TurnDone = %#v, %v", deliveries, err)
			}
		}),
	})
	t.Cleanup(controller.Close)

	if err := controller.RunTurn(context.Background(), "create joke"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if terminal.ArtifactDelivery == nil || terminal.ArtifactDelivery.DeliveredAt <= 0 || len(terminal.ArtifactDelivery.Paths) != 1 || terminal.ArtifactDelivery.Paths[0] != "joke.md" {
		t.Fatalf("TurnDone artifact delivery = %#v", terminal.ArtifactDelivery)
	}
	if terminal.OriginSessionPath != sessionPath || terminal.OriginWorkspaceRoot != workspaceRoot {
		t.Fatalf("TurnDone origin = (%q, %q), want (%q, %q)", terminal.OriginSessionPath, terminal.OriginWorkspaceRoot, sessionPath, workspaceRoot)
	}
}

func TestRunTurnDraftSceneFirstTurnPersistsArtifactDelivery(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "draft-session.jsonl")
	actor := testActor()
	task := testTaskContext(t, "task_draft_artifact_success", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	recordingStore := &controlTaskLifecycleStore{}
	inspectedTerminalCommit := false
	store := &inspectingArtifactTaskLifecycleStore{
		controlTaskLifecycleStore: recordingStore,
		beforeUpdate: func(update modelbilling.TaskStateUpdate) error {
			if update.To != modelbilling.TaskCompleted {
				return nil
			}
			loaded, err := agent.LoadSession(sessionPath)
			if err != nil {
				return fmt.Errorf("load session before completed update: %w", err)
			}
			final, ok := artifactDeliveryFinalByContent(loaded.Snapshot(), "Draft report complete.")
			if !ok || final.MessageKind != provider.MessageKindFinalAnswer || !provider.IsDurableArtifactDeliveryV1(final.ArtifactDelivery) ||
				len(final.ArtifactDelivery.Paths) != 1 || final.ArtifactDelivery.Paths[0] != "draft-report.md" {
				return fmt.Errorf("persisted final before completed update = %#v", final)
			}
			deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
			if err != nil {
				return fmt.Errorf("load deliveries before completed update: %w", err)
			}
			if len(deliveries) != 1 || !reflect.DeepEqual(deliveries[0], *final.ArtifactDelivery) {
				return fmt.Errorf("persisted deliveries before completed update = %#v, want %#v", deliveries, final.ArtifactDelivery)
			}
			inspectedTerminalCommit = true
			return nil
		},
	}
	executor := agent.New(nil, tool.NewRegistry(), agent.NewSession("office prompt"), agent.Options{}, event.Discard)
	var terminal event.Event
	controller := New(Options{
		Actor:         actor,
		Runner:        draftArtifactDeliveryRunner{executor: executor, workspaceRoot: workspaceRoot},
		Executor:      executor,
		WorkspaceRoot: workspaceRoot,
		SessionDir:    filepath.Dir(sessionPath),
		SessionPath:   sessionPath,
		SystemPrompt:  "office prompt",
		Scene:         scene.Coding,
		SceneState:    scene.Draft,
		ScenePromptBuilder: func(selected scene.Scene, _, _ string) (string, expert.Selection, error) {
			return fmt.Sprintf("%s prompt", selected), expert.Selection{}, nil
		},
		TaskContextInitializer: initializer,
		TaskLifecycle:          modelbilling.TaskLifecycleRunner{Tasks: store},
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	if got := controller.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Draft}) {
		t.Fatalf("initial scene status = %+v, want Coding/Draft", got)
	}
	if err := controller.RunTurn(context.Background(), "create a draft report"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if got := controller.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Locked}) {
		t.Fatalf("final scene status = %+v, want Coding/Locked", got)
	}
	if !inspectedTerminalCommit {
		t.Fatal("task lifecycle did not inspect a durable artifact contract before completion")
	}
	updates := recordingStore.snapshot()
	if len(updates) != 2 || updates[0].TaskID != task.TaskID || updates[0].From != modelbilling.TaskReady || updates[0].To != modelbilling.TaskRunning ||
		updates[1].TaskID != task.TaskID || updates[1].From != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskCompleted {
		t.Fatalf("task lifecycle updates = %#v, want Ready->Running->Completed", updates)
	}
	if info, err := os.Stat(filepath.Join(workspaceRoot, "draft-report.md")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("delivered file is not real: info=%v err=%v", info, err)
	}

	history := controller.History()
	final, ok := artifactDeliveryFinalByContent(history, "Draft report complete.")
	if !ok {
		t.Fatal("final answer missing")
	}
	if final.ArtifactTurnID == "" {
		t.Fatal("final answer has no artifact turn ID")
	}
	if final.ArtifactDelivery == nil || final.ArtifactDelivery.Version != provider.ArtifactDeliveryVersion ||
		len(final.ArtifactDelivery.Paths) != 1 || final.ArtifactDelivery.Paths[0] != "draft-report.md" {
		t.Fatalf("final artifact delivery = %#v", final.ArtifactDelivery)
	}
	contractCount := 0
	for _, message := range history {
		if message.ArtifactDelivery != nil {
			contractCount++
		}
	}
	if contractCount != 1 {
		t.Fatalf("in-memory artifact delivery contracts = %d, want 1", contractCount)
	}

	loaded, err := agent.LoadSession(sessionPath)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	persistedMessages := loaded.Snapshot()
	persistedFinal, ok := artifactDeliveryFinalByContent(persistedMessages, "Draft report complete.")
	if !ok || persistedFinal.ArtifactTurnID != final.ArtifactTurnID || !reflect.DeepEqual(persistedFinal.ArtifactDelivery, final.ArtifactDelivery) {
		t.Fatalf("persisted final delivery = %#v, want %#v", persistedFinal.ArtifactDelivery, final.ArtifactDelivery)
	}
	persistedContractCount := 0
	for _, message := range persistedMessages {
		if message.ArtifactDelivery != nil {
			persistedContractCount++
		}
	}
	if persistedContractCount != 1 {
		t.Fatalf("persisted artifact delivery contracts = %d, want 1", persistedContractCount)
	}
	if terminal.ArtifactDelivery == nil || !reflect.DeepEqual(terminal.ArtifactDelivery, final.ArtifactDelivery) {
		t.Fatalf("TurnDone artifact delivery = %#v, want %#v", terminal.ArtifactDelivery, final.ArtifactDelivery)
	}
	if terminal.Err != nil || terminal.Turn == nil || terminal.Turn.TaskID != task.TaskID {
		t.Fatalf("TurnDone = err %v metadata %#v, want successful task %q", terminal.Err, terminal.Turn, task.TaskID)
	}
}

func TestRunTurnDraftSceneCommitFailureRunsInsideTaskLifecycle(t *testing.T) {
	wantErr := errors.New("scene session save failed")
	actor := testActor()
	task := testTaskContext(t, "task_draft_scene_commit_failure", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	store := &controlTaskLifecycleStore{}
	sessionPath := filepath.Join(t.TempDir(), "draft-session.jsonl")
	executor := agent.New(nil, tool.NewRegistry(), agent.NewSession("office prompt"), agent.Options{}, event.Discard)
	runner := &countingRunner{}
	var terminal event.Event
	controller := New(Options{
		Actor:         actor,
		Runner:        runner,
		Executor:      executor,
		WorkspaceRoot: t.TempDir(),
		SessionDir:    filepath.Dir(sessionPath),
		SessionPath:   sessionPath,
		SystemPrompt:  "office prompt",
		Scene:         scene.Coding,
		SceneState:    scene.Draft,
		ScenePromptBuilder: func(selected scene.Scene, _, _ string) (string, expert.Selection, error) {
			return fmt.Sprintf("%s prompt", selected), expert.Selection{}, nil
		},
		SceneSessionSaver: func(*agent.Session, string) error {
			return wantErr
		},
		TaskContextInitializer: initializer,
		TaskLifecycle:          modelbilling.TaskLifecycleRunner{Tasks: store},
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	runErr := controller.RunTurn(context.Background(), "create a draft report")
	if !errors.Is(runErr, wantErr) {
		t.Fatalf("RunTurn error = %v, want %v", runErr, wantErr)
	}
	updates := store.snapshot()
	if len(updates) != 2 || updates[0].TaskID != task.TaskID || updates[0].From != modelbilling.TaskReady || updates[0].To != modelbilling.TaskRunning ||
		updates[1].TaskID != task.TaskID || updates[1].From != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskFailed {
		t.Fatalf("task lifecycle updates = %#v, want Ready->Running->Failed", updates)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
	if got := controller.SceneStatus(); got != (scene.Status{Scene: scene.Coding, State: scene.Draft}) {
		t.Fatalf("scene status = %+v, want Coding/Draft", got)
	}
	if terminal.ArtifactDelivery != nil {
		t.Fatalf("TurnDone artifact delivery = %#v, want nil", terminal.ArtifactDelivery)
	}
	if terminal.Turn == nil || terminal.Turn.TaskID != task.TaskID {
		t.Fatalf("TurnDone metadata = %#v, want task ID %q", terminal.Turn, task.TaskID)
	}
	for _, message := range controller.History() {
		if message.ArtifactTurnID != "" || message.ArtifactDelivery != nil {
			t.Fatalf("failed pre-artifact turn stamped message = %#v", message)
		}
	}
}

func TestRunTurnDeliversExplicitDocumentDirectoryArtifact(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	session := agent.NewSession("system")
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	const relative = "docs/workbuddy-report.md"
	controller := New(Options{
		Runner:   explicitNestedArtifactRunner{session: session, workspaceRoot: workspaceRoot, relativePath: relative},
		Executor: executor, WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	if err := controller.RunTurn(context.Background(), "write a WorkBuddy report under docs"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if terminal.ArtifactDelivery == nil || len(terminal.ArtifactDelivery.Paths) != 1 || terminal.ArtifactDelivery.Paths[0] != relative {
		t.Fatalf("TurnDone artifact delivery = %#v, want %q", terminal.ArtifactDelivery, relative)
	}
	if info, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(relative))); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("delivered file is not real: info=%v err=%v", info, err)
	}
}

func TestRunTurnFailsClosedWhenArtifactHasNoPersistentSessionPath(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionDir := t.TempDir()
	session := agent.NewSession("system")
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	const relative = "docs/workbuddy-report.md"
	controller := New(Options{
		Runner:   explicitNestedArtifactRunner{session: session, workspaceRoot: workspaceRoot, relativePath: relative},
		Executor: executor, WorkspaceRoot: workspaceRoot, SessionDir: sessionDir,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	runErr := controller.RunTurn(context.Background(), "write a WorkBuddy report under docs")
	if !errors.Is(runErr, errNoSessionPath) {
		t.Fatalf("RunTurn error = %v, want %v", runErr, errNoSessionPath)
	}
	if info, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(relative))); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("runner did not produce the real report: info=%v err=%v", info, err)
	}
	if terminal.Err == nil || terminal.ArtifactDelivery != nil {
		t.Fatalf("TurnDone = err %v delivery %#v, want error without a card", terminal.Err, terminal.ArtifactDelivery)
	}
	final := controller.History()[len(controller.History())-1]
	if final.MessageKind != provider.MessageKindUncommittedFinalAnswer || final.ArtifactDelivery != nil {
		t.Fatalf("in-memory final = %#v, want uncommitted without a contract", final)
	}
	if matches, err := filepath.Glob(filepath.Join(sessionDir, "*.jsonl")); err != nil || len(matches) != 0 {
		t.Fatalf("path-less turn persisted transcripts = %v, err = %v", matches, err)
	}
}

func TestRunTurnDeliversBashCreatedArtifactWithoutFinalPath(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), ".hidden-parent", "workspace")
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	session := agent.NewSession("system")
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	controller := New(Options{
		Runner: bashArtifactDeliveryRunner{session: session, workspaceRoot: workspaceRoot}, Executor: executor,
		WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath, Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	if err := controller.RunTurn(context.Background(), "create report"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if terminal.ArtifactDelivery == nil || len(terminal.ArtifactDelivery.Paths) != 1 || terminal.ArtifactDelivery.Paths[0] != "report.docx" {
		t.Fatalf("TurnDone artifact delivery = %#v", terminal.ArtifactDelivery)
	}
}

func TestRunTurnDoesNotDeliverOverwrittenExistingSource(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "README.md"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	session := agent.NewSession("system")
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	controller := New(Options{
		Runner: overwriteSourceRunner{session: session, workspaceRoot: workspaceRoot}, Executor: executor,
		WorkspaceRoot: workspaceRoot, Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	if err := controller.RunTurn(context.Background(), "update source"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if terminal.ArtifactDelivery == nil || terminal.ArtifactDelivery.Paths == nil || len(terminal.ArtifactDelivery.Paths) != 0 {
		t.Fatalf("TurnDone artifact delivery = %#v, want explicit empty contract", terminal.ArtifactDelivery)
	}
}

func TestRunTurnDeliversArtifactAfterCompactionRebasesCurrentTurn(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	session := agent.NewSession("system")
	for i := 0; i < 3; i++ {
		session.Add(provider.Message{Role: provider.RoleUser, Content: "old prompt"})
		session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "old answer"})
	}
	originalTurnStart := len(session.Snapshot())
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	controller := New(Options{
		Runner: compactingArtifactDeliveryRunner{session: session, workspaceRoot: workspaceRoot}, Executor: executor,
		WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	if err := controller.RunTurn(context.Background(), "create joke"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if terminal.ArtifactDelivery == nil || len(terminal.ArtifactDelivery.Paths) != 1 || terminal.ArtifactDelivery.Paths[0] != "joke.md" {
		t.Fatalf("TurnDone artifact delivery = %#v", terminal.ArtifactDelivery)
	}

	history := controller.History()
	toolCallIndex := -1
	finalIndex := -1
	for i, message := range history {
		if len(message.ToolCalls) == 1 && message.ToolCalls[0].Name == "write_file" {
			toolCallIndex = i
		}
		if message.MessageKind == provider.MessageKindFinalAnswer && message.ArtifactTurnID != "" {
			finalIndex = i
		}
	}
	if toolCallIndex < 0 || toolCallIndex >= originalTurnStart {
		t.Fatalf("compacted write_file index = %d, want before old turn start %d", toolCallIndex, originalTurnStart)
	}
	if finalIndex < 0 || history[finalIndex].ArtifactDelivery == nil || history[finalIndex].ArtifactDelivery.Paths[0] != "joke.md" {
		t.Fatalf("compacted final answer = %#v", history)
	}

	deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
	if err != nil {
		t.Fatalf("LoadArtifactDeliveries: %v", err)
	}
	if len(deliveries) != 1 || len(deliveries[0].Paths) != 1 || deliveries[0].Paths[0] != "joke.md" {
		t.Fatalf("persisted deliveries = %#v", deliveries)
	}
}

func TestRunTurnDeliversArtifactAfterRealAgentCompaction(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: workspaceRoot}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	registry.Add(artifactFatReadTool{})
	prov := agenttest.NewMock("real-artifact-compaction",
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "write", Name: "write_file", Arguments: `{"path":"report.md","content":"report"}`}}},
		agenttest.Turn{ToolCalls: []provider.ToolCall{{ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`}}},
		agenttest.Turn{
			ToolCalls: []provider.ToolCall{{ID: "fat-read", Name: "fat_read", Arguments: `{}`}},
			Usage:     &provider.Usage{PromptTokens: 95, TotalTokens: 95},
		},
		agenttest.Turn{Text: "- earlier work summarized"},
		agenttest.Turn{Text: "report complete"},
	)
	var terminal event.Event
	compactions := 0
	sink := event.FuncSink(func(emitted event.Event) {
		if emitted.Kind == event.CompactionDone && emitted.Compaction.Summary != "" {
			compactions++
		}
		if emitted.Kind == event.TurnDone {
			terminal = emitted
		}
	})
	session := agent.NewSession("system")
	executor := agent.New(prov, registry, session, agent.Options{
		ContextWindow: 100, CompactRatio: 0.8, CompactForceRatio: 0.9, RecentKeep: 2,
	}, sink)
	controller := New(Options{
		Runner: executor, Executor: executor, WorkspaceRoot: workspaceRoot,
		SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath, Sink: sink,
	})
	t.Cleanup(controller.Close)

	if err := controller.RunTurn(context.Background(), "create report and inspect context"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if compactions == 0 {
		t.Fatal("real Agent.Run did not execute compaction")
	}
	if terminal.ArtifactDelivery == nil || len(terminal.ArtifactDelivery.Paths) != 1 || terminal.ArtifactDelivery.Paths[0] != "report.md" {
		t.Fatalf("TurnDone artifact delivery = %#v", terminal.ArtifactDelivery)
	}
	if info, err := os.Stat(filepath.Join(workspaceRoot, "report.md")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("real delivered file missing: info=%v err=%v", info, err)
	}
	deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
	if err != nil || len(deliveries) != 1 || len(deliveries[0].Paths) != 1 || deliveries[0].Paths[0] != "report.md" {
		t.Fatalf("persisted deliveries after compaction = %#v, %v", deliveries, err)
	}
}

func TestRunTurnArtifactCommitIsCrashSafeAcrossOuterLifecycleOutcome(t *testing.T) {
	tests := []struct {
		name      string
		result    error
		committed bool
	}{
		{name: "success", committed: true},
		{name: "failure", result: errors.New("settlement failed after agent completion")},
		{name: "cancellation", result: context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
			session := agent.NewSession("system")
			executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
			var terminal event.Event
			lifecycle := inspectAfterRunTaskLifecycle{result: tt.result}
			lifecycle.afterRun = func() error {
				// Artifact commit is the durable prerequisite for successful task
				// settlement, so the JSONL contract must already exist while the
				// outer lifecycle is still deciding its terminal outcome.
				loaded, err := agent.LoadSession(sessionPath)
				if err != nil {
					return fmt.Errorf("reload mid-lifecycle artifact commit: %w", err)
				}
				candidate, ok := artifactDeliveryFinalByContent(loaded.Snapshot(), "[Download report](report.md)")
				if !ok {
					return errors.New("mid-lifecycle committed final missing")
				}
				if candidate.MessageKind != provider.MessageKindFinalAnswer || !provider.IsDurableArtifactDeliveryV1(candidate.ArtifactDelivery) ||
					len(candidate.ArtifactDelivery.Paths) != 1 || candidate.ArtifactDelivery.Paths[0] != "report.md" {
					return fmt.Errorf("mid-lifecycle final = %#v, want committed v1 report contract", candidate)
				}
				deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
				if err != nil {
					return fmt.Errorf("load mid-lifecycle deliveries: %w", err)
				}
				if len(deliveries) != 1 || !reflect.DeepEqual(deliveries[0], *candidate.ArtifactDelivery) {
					return fmt.Errorf("mid-lifecycle deliveries = %#v, want %#v", deliveries, candidate.ArtifactDelivery)
				}
				return nil
			}
			controller := New(Options{
				Runner: linkedMarkdownArtifactRunner{session: session, workspaceRoot: workspaceRoot}, Executor: executor,
				WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath,
				TaskLifecycle: lifecycle,
				Sink: event.FuncSink(func(e event.Event) {
					if e.Kind == event.TurnDone {
						terminal = e
					}
				}),
			})
			t.Cleanup(controller.Close)

			runErr := controller.RunTurn(context.Background(), "create report")
			if tt.result == nil && runErr != nil {
				t.Fatalf("RunTurn: %v", runErr)
			}
			if tt.result != nil && !errors.Is(runErr, tt.result) {
				t.Fatalf("RunTurn error = %v, want original lifecycle error %v", runErr, tt.result)
			}
			if _, err := os.Stat(filepath.Join(workspaceRoot, "report.md")); err != nil {
				t.Fatalf("real report output missing: %v", err)
			}

			loaded, err := agent.LoadSession(sessionPath)
			if err != nil {
				t.Fatalf("LoadSession after lifecycle: %v", err)
			}
			final, ok := artifactDeliveryFinalByContent(loaded.Snapshot(), "[Download report](report.md)")
			if !ok {
				t.Fatal("persisted linked assistant message missing")
			}
			deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
			if err != nil {
				t.Fatalf("LoadArtifactDeliveries: %v", err)
			}
			if tt.committed {
				if final.MessageKind != provider.MessageKindFinalAnswer || final.ArtifactDelivery == nil || len(final.ArtifactDelivery.Paths) != 1 || final.ArtifactDelivery.Paths[0] != "report.md" {
					t.Fatalf("persisted successful final = %#v", final)
				}
				memoryFinal, memoryOK := artifactDeliveryFinalByContent(controller.History(), "[Download report](report.md)")
				if !memoryOK || memoryFinal.MessageKind != provider.MessageKindFinalAnswer || !reflect.DeepEqual(memoryFinal.ArtifactDelivery, final.ArtifactDelivery) {
					t.Fatalf("in-memory successful final = %#v, want persisted %#v", memoryFinal, final)
				}
				if terminal.Err != nil || terminal.ArtifactDelivery == nil || len(deliveries) != 1 {
					t.Fatalf("successful terminal = err %v delivery %#v; persisted deliveries = %#v", terminal.Err, terminal.ArtifactDelivery, deliveries)
				}
				return
			}
			if final.MessageKind != provider.MessageKindUncommittedFinalAnswer || final.ArtifactDelivery != nil {
				t.Fatalf("persisted unsuccessful final = %#v, want uncommitted without contract", final)
			}
			memoryFinal, memoryOK := artifactDeliveryFinalByContent(controller.History(), "[Download report](report.md)")
			if !memoryOK || memoryFinal.MessageKind != provider.MessageKindUncommittedFinalAnswer || memoryFinal.ArtifactDelivery != nil {
				t.Fatalf("in-memory unsuccessful final = %#v, want uncommitted without contract", memoryFinal)
			}
			if terminal.Err == nil || terminal.ArtifactDelivery != nil || len(deliveries) != 0 {
				t.Fatalf("unsuccessful terminal = err %v delivery %#v; persisted deliveries = %#v", terminal.Err, terminal.ArtifactDelivery, deliveries)
			}
		})
	}
}

func TestRunTurnBYOKArtifactSessionSaveFailureFailsTaskWithoutDelivery(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.Mkdir(sessionPath, 0o755); err != nil {
		t.Fatalf("create directory at session path: %v", err)
	}
	actor := testActor()
	task := testTaskContext(t, "task_artifact_session_save_failure", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	store := &controlTaskLifecycleStore{}
	session := agent.NewSession("system")
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	controller := New(Options{
		Actor:                  actor,
		Runner:                 linkedMarkdownArtifactRunner{session: session, workspaceRoot: workspaceRoot},
		Executor:               executor,
		WorkspaceRoot:          workspaceRoot,
		SessionDir:             filepath.Dir(sessionPath),
		SessionPath:            sessionPath,
		TaskContextInitializer: initializer,
		TaskLifecycle:          modelbilling.TaskLifecycleRunner{Tasks: store},
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	runErr := controller.RunTurn(context.Background(), "create report")
	if runErr == nil {
		t.Fatal("RunTurn unexpectedly succeeded when artifact contract could not be persisted")
	}
	if !strings.Contains(runErr.Error(), "persist artifact delivery transcript") {
		t.Fatalf("RunTurn error = %v, want artifact transcript persistence failure", runErr)
	}
	if info, err := os.Stat(filepath.Join(workspaceRoot, "report.md")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("runner did not produce the real report before persistence failed: info=%v err=%v", info, err)
	}
	updates := store.snapshot()
	if len(updates) != 2 || updates[0].TaskID != task.TaskID || updates[0].From != modelbilling.TaskReady || updates[0].To != modelbilling.TaskRunning ||
		updates[1].TaskID != task.TaskID || updates[1].From != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskFailed {
		t.Fatalf("task lifecycle updates = %#v, want Ready->Running->Failed", updates)
	}
	for _, update := range updates {
		if update.To == modelbilling.TaskCompleted {
			t.Fatalf("artifact persistence failure reached Completed: %#v", updates)
		}
	}
	if terminal.Err == nil || terminal.ArtifactDelivery != nil {
		t.Fatalf("TurnDone = err %v delivery %#v, want error without card", terminal.Err, terminal.ArtifactDelivery)
	}
	if terminal.Turn == nil || terminal.Turn.TaskID != task.TaskID {
		t.Fatalf("TurnDone metadata = %#v, want task ID %q", terminal.Turn, task.TaskID)
	}
	final, ok := artifactDeliveryFinalByContent(controller.History(), "[Download report](report.md)")
	if !ok || final.MessageKind != provider.MessageKindUncommittedFinalAnswer || final.ArtifactDelivery != nil {
		t.Fatalf("in-memory final = %#v, want uncommitted without contract", final)
	}
	for _, message := range controller.History() {
		if message.ArtifactDelivery != nil {
			t.Fatalf("in-memory transcript retained artifact contract: %#v", message)
		}
	}
}

func TestRunTurnOverridesStaleRunnerArtifactIdentity(t *testing.T) {
	wantFailure := errors.New("outer lifecycle failed")
	for _, tt := range []struct {
		name      string
		result    error
		committed bool
	}{
		{name: "success", committed: true},
		{name: "failure", result: wantFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
			session := agent.NewSession("system")
			executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
			var terminal event.Event
			controller := New(Options{
				Runner: staleStampedArtifactRunner{session: session, workspaceRoot: workspaceRoot}, Executor: executor,
				WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath,
				TaskLifecycle: inspectAfterRunTaskLifecycle{result: tt.result},
				Sink: event.FuncSink(func(e event.Event) {
					if e.Kind == event.TurnDone {
						terminal = e
					}
				}),
			})
			t.Cleanup(controller.Close)

			runErr := controller.RunTurn(context.Background(), "create report")
			if tt.result == nil && runErr != nil {
				t.Fatalf("RunTurn: %v", runErr)
			}
			if tt.result != nil && !errors.Is(runErr, tt.result) {
				t.Fatalf("RunTurn error = %v, want %v", runErr, tt.result)
			}
			loaded, err := agent.LoadSession(sessionPath)
			if err != nil {
				t.Fatalf("LoadSession: %v", err)
			}
			final, ok := artifactDeliveryFinalByContent(loaded.Snapshot(), "[Download report](report.md)")
			if !ok {
				t.Fatal("persisted final missing")
			}
			if final.ArtifactTurnID == "" || final.ArtifactTurnID == "stale-runner-turn" {
				t.Fatalf("final artifact turn ID = %q, want current host identity", final.ArtifactTurnID)
			}
			contractCount := 0
			for _, message := range loaded.Snapshot() {
				if message.ArtifactDelivery != nil {
					contractCount++
				}
			}
			deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
			if err != nil {
				t.Fatalf("LoadArtifactDeliveries: %v", err)
			}
			if tt.committed {
				if final.MessageKind != provider.MessageKindFinalAnswer || final.ArtifactDelivery == nil ||
					len(final.ArtifactDelivery.Paths) != 1 || final.ArtifactDelivery.Paths[0] != "report.md" {
					t.Fatalf("committed final = %#v", final)
				}
				if terminal.Err != nil || terminal.ArtifactDelivery == nil || len(deliveries) != 1 || contractCount != 1 {
					t.Fatalf("successful terminal = err %v delivery %#v; persisted deliveries = %#v", terminal.Err, terminal.ArtifactDelivery, deliveries)
				}
				return
			}
			if final.MessageKind != provider.MessageKindUncommittedFinalAnswer || final.ArtifactDelivery != nil {
				t.Fatalf("failed final = %#v, want uncommitted without contract", final)
			}
			if terminal.Err == nil || terminal.ArtifactDelivery != nil || len(deliveries) != 0 || contractCount != 0 {
				t.Fatalf("failed terminal = err %v delivery %#v; persisted deliveries = %#v", terminal.Err, terminal.ArtifactDelivery, deliveries)
			}
		})
	}
}

func TestRunTurnSanitizesRunnerOwnedContractsAfterReplace(t *testing.T) {
	wantFailure := errors.New("outer lifecycle failed")
	for _, tt := range []struct {
		name         string
		useCurrentID bool
		result       error
		committed    bool
	}{
		{name: "stale_id_success", committed: true},
		{name: "current_id_success", useCurrentID: true, committed: true},
		{name: "stale_id_failure", result: wantFailure},
		{name: "current_id_failure", useCurrentID: true, result: wantFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
			session := agent.NewSession("system")
			executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
			var terminal event.Event
			controller := New(Options{
				Runner:   replacingArtifactRunner{session: session, workspaceRoot: workspaceRoot, useCurrentID: tt.useCurrentID},
				Executor: executor, WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath,
				TaskLifecycle: inspectAfterRunTaskLifecycle{result: tt.result},
				Sink: event.FuncSink(func(e event.Event) {
					if e.Kind == event.TurnDone {
						terminal = e
					}
				}),
			})
			t.Cleanup(controller.Close)

			runErr := controller.RunTurn(context.Background(), "create report")
			if tt.result == nil && runErr != nil {
				t.Fatalf("RunTurn: %v", runErr)
			}
			if tt.result != nil && !errors.Is(runErr, tt.result) {
				t.Fatalf("RunTurn error = %v, want %v", runErr, tt.result)
			}
			loaded, err := agent.LoadSession(sessionPath)
			if err != nil {
				t.Fatalf("LoadSession: %v", err)
			}
			final, ok := artifactDeliveryFinalByContent(loaded.Snapshot(), "[Download report](report.md)")
			if !ok {
				t.Fatal("persisted final missing")
			}
			contractCount := 0
			for _, message := range loaded.Snapshot() {
				if message.ArtifactDelivery != nil {
					contractCount++
				}
			}
			deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
			if err != nil {
				t.Fatalf("LoadArtifactDeliveries: %v", err)
			}
			if tt.committed {
				if final.MessageKind != provider.MessageKindFinalAnswer || final.ArtifactDelivery == nil ||
					len(final.ArtifactDelivery.Paths) != 1 || final.ArtifactDelivery.Paths[0] != "report.md" {
					t.Fatalf("committed final = %#v", final)
				}
				if terminal.ArtifactDelivery == nil || len(deliveries) != 1 || contractCount != 1 {
					t.Fatalf("successful terminal = %#v; persisted deliveries = %#v; contracts = %d", terminal, deliveries, contractCount)
				}
				return
			}
			if final.MessageKind != provider.MessageKindUncommittedFinalAnswer || final.ArtifactDelivery != nil {
				t.Fatalf("failed final = %#v, want uncommitted without contract", final)
			}
			if terminal.Err == nil || terminal.ArtifactDelivery != nil || len(deliveries) != 0 || contractCount != 0 {
				t.Fatalf("failed terminal = %#v; persisted deliveries = %#v; contracts = %d", terminal, deliveries, contractCount)
			}
		})
	}
}

func artifactDeliveryFinalByContent(messages []provider.Message, content string) (provider.Message, bool) {
	for _, message := range messages {
		if message.Role == provider.RoleAssistant && message.Content == content {
			return message, true
		}
	}
	return provider.Message{}, false
}

func TestRunTurnCommitsArtifactDeliveryWhenAuxiliarySnapshotMetaFails(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	// A directory at the sidecar path makes the ordinary activity snapshot fail
	// after publishing JSONL. Auxiliary metadata cannot decide whether the turn
	// and its artifact manifest are committed.
	if err := os.Mkdir(sessionPath+".meta", 0o755); err != nil {
		t.Fatal(err)
	}
	session := agent.NewSession("system")
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var terminal event.Event
	controller := New(Options{
		Runner: linkedMarkdownArtifactRunner{session: session, workspaceRoot: workspaceRoot}, Executor: executor,
		WorkspaceRoot: workspaceRoot, SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				terminal = e
			}
		}),
	})
	t.Cleanup(controller.Close)

	runErr := controller.RunTurn(context.Background(), "create report")
	if runErr != nil {
		t.Fatalf("RunTurn failed because auxiliary metadata could not persist: %v", runErr)
	}
	if terminal.Err != nil || terminal.ArtifactDelivery == nil || len(terminal.ArtifactDelivery.Paths) != 1 || terminal.ArtifactDelivery.Paths[0] != "report.md" {
		t.Fatalf("TurnDone = err %v delivery %#v", terminal.Err, terminal.ArtifactDelivery)
	}
	history := controller.History()
	committedFinal := history[len(history)-1]
	if committedFinal.MessageKind != provider.MessageKindFinalAnswer || committedFinal.ArtifactDelivery == nil {
		t.Fatalf("committed in-memory final = %#v", committedFinal)
	}
	loaded, err := agent.LoadSession(sessionPath)
	if err != nil {
		t.Fatalf("LoadSession after auxiliary metadata failure: %v", err)
	}
	loadedFinal := loaded.Snapshot()[len(loaded.Snapshot())-1]
	if loadedFinal.MessageKind != provider.MessageKindFinalAnswer || loadedFinal.ArtifactDelivery == nil {
		t.Fatalf("committed persisted final = %#v", loadedFinal)
	}
	deliveries, err := agent.LoadArtifactDeliveries(sessionPath)
	if err != nil {
		t.Fatalf("LoadArtifactDeliveries: %v", err)
	}
	if len(deliveries) != 1 || len(deliveries[0].Paths) != 1 || deliveries[0].Paths[0] != "report.md" {
		t.Fatalf("persisted deliveries = %#v", deliveries)
	}
}
