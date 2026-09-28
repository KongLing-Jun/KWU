package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
)

func deliverFilesSuccessResult(t *testing.T, paths ...string) string {
	t.Helper()
	if paths == nil {
		paths = []string{}
	}
	canonical := make([]string, len(paths))
	for i, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("resolve deliver_files result path %q: %v", path, err)
		}
		canonical[i] = absolute
	}
	encoded, err := json.Marshal(map[string]any{
		"type":  builtin.DeliverFilesResultType,
		"paths": canonical,
	})
	if err != nil {
		t.Fatalf("encode deliver_files result: %v", err)
	}
	return string(encoded)
}

func deliverFilesSuccessPaths(t *testing.T, paths ...string) []string {
	t.Helper()
	decoded, ok := builtin.DecodeDeliverFilesResult(deliverFilesSuccessResult(t, paths...))
	if !ok {
		t.Fatal("decode test deliver_files result")
	}
	return decoded
}

func TestArtifactDeliveryCandidatesUsesLastSuccessfulExplicitManifest(t *testing.T) {
	messages := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
			{ID: "write-ok", Name: "write_file", Arguments: `{"path":"joke.md","content":"joke"}`},
			{ID: "deliver-first", Name: "deliver_files", Arguments: `{"paths":["first.pdf"]}`},
			{ID: "deliver-failed", Name: "deliver_files", Arguments: `{"paths":["failed.pdf"]}`},
			{ID: "deliver-last", Name: "deliver_files", Arguments: `{"paths":["report.md","appendix.pdf"]}`},
		}},
		{Role: provider.RoleTool, ToolCallID: "write-ok", Name: "write_file", Content: "wrote joke.md", ToolArtifacts: []provider.ToolArtifact{{Path: "joke.md", Source: SubagentArtifactSourceWriter}}},
		{Role: provider.RoleTool, ToolCallID: "deliver-first", Name: "deliver_files", Content: deliverFilesSuccessResult(t, "first.pdf")},
		{Role: provider.RoleTool, ToolCallID: "deliver-failed", Name: "deliver_files", Content: "error: missing file"},
		{Role: provider.RoleTool, ToolCallID: "deliver-last", Name: "deliver_files", Content: deliverFilesSuccessResult(t, "report.md", "appendix.pdf")},
		{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "[unverified](ghost.md)"},
	}

	got := ArtifactDeliveryCandidates(messages)
	want := deliverFilesSuccessPaths(t, "report.md", "appendix.pdf")
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ArtifactDeliveryCandidates() = %v, want canonical paths %v", got, want)
	}
}

func TestArtifactDeliveryCandidatesTreatsLatestEmptyManifestAsAuthoritative(t *testing.T) {
	messages := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
			{ID: "first", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`},
			{ID: "empty", Name: "deliver_files", Arguments: `{"paths":[]}`},
		}},
		{Role: provider.RoleTool, ToolCallID: "first", Name: "deliver_files", Content: deliverFilesSuccessResult(t, "report.md")},
		{Role: provider.RoleTool, ToolCallID: "empty", Name: "deliver_files", Content: deliverFilesSuccessResult(t)},
	}
	if got := ArtifactDeliveryCandidates(messages); len(got) != 0 {
		t.Fatalf("ArtifactDeliveryCandidates() = %v, want latest empty manifest", got)
	}
}

func TestArtifactDeliveryCandidatesIgnoreFinalLinksAndMismatchedToolResults(t *testing.T) {
	messages := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "write", Name: "write_file", Arguments: `{"path":"report.md"}`}}},
		{Role: provider.RoleTool, ToolCallID: "write", Name: "edit_file", Content: "wrote report.md", ToolArtifacts: []provider.ToolArtifact{{Path: "report.md", Source: SubagentArtifactSourceWriter}}},
		{Role: provider.RoleTool, ToolCallID: "orphan", Name: "write_file", Content: "wrote orphan.md", ToolArtifacts: []provider.ToolArtifact{{Path: "orphan.md", Source: SubagentArtifactSourceWriter}}},
		{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "[report](report.md)"},
	}
	if got := ArtifactDeliveryCandidates(messages); len(got) != 0 {
		t.Fatalf("ArtifactDeliveryCandidates() = %v, want none", got)
	}
}

func TestArtifactDeliveryCandidatesAssociatesReusedToolIDsInTranscriptOrder(t *testing.T) {
	messages := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			ID: "reused", Name: "deliver_files", Arguments: `{"paths":["report.md"]}`,
		}}},
		{Role: provider.RoleTool, ToolCallID: "reused", Name: "deliver_files", Content: deliverFilesSuccessResult(t, "report.md")},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			ID: "reused", Name: "write_file", Arguments: `{"path":"source.go","content":"package main"}`,
		}}},
		{Role: provider.RoleTool, ToolCallID: "reused", Name: "write_file", Content: "wrote source.go"},
	}
	got := ArtifactDeliveryCandidates(messages)
	want := deliverFilesSuccessPaths(t, "report.md")
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("ArtifactDeliveryCandidates() = %v, want the earlier delivered manifest", got)
	}
}

func TestRunUsesObservedArtifactWithoutExplicitDeclaration(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: root}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	prov := &mockProvider{name: "observed-artifact", streams: [][]provider.Chunk{
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write", Name: "write_file", Arguments: `{"path":"joke.md","content":"joke"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	var rejected int
	a := New(prov, registry, session, Options{}, event.FuncSink(func(emitted event.Event) {
		if emitted.Kind == event.FinalCandidateRejected {
			rejected++
		}
	}))
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	if err := a.Run(ctx, "create a joke file"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rejected != 0 || len(prov.requests) != 2 {
		t.Fatalf("rejected finals = %d, provider requests = %d", rejected, len(prov.requests))
	}
	paths, participated, valid := a.ArtifactDeliveryTurnDecision("turn")
	wantPath := filepath.Join(root, "joke.md")
	if !participated || !valid || len(paths) != 1 || filepath.Clean(paths[0]) != filepath.Clean(wantPath) {
		t.Fatalf("observed turn decision = %v, participated=%v, valid=%v", paths, participated, valid)
	}
	if _, err := os.Stat(filepath.Join(root, "joke.md")); err != nil {
		t.Fatalf("delivered file missing: %v", err)
	}
}

func TestRunUsesObservedArtifactAfterOpaqueWorkspaceMutationWithoutDeclaration(t *testing.T) {
	root := t.TempDir()
	report := filepath.Join(root, "docs", "report.md")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := tool.NewRegistry()
	registry.Add(opaqueArtifactWriter{path: report})
	prov := &mockProvider{name: "observed-opaque-artifact", streams: [][]provider.Chunk{
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write", Name: "opaque_export", Arguments: `{}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	var rejected int
	a := New(prov, registry, session, Options{}, event.FuncSink(func(emitted event.Event) {
		if emitted.Kind == event.FinalCandidateRejected {
			rejected++
		}
	}))
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	if err := a.Run(ctx, "create a report under docs"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rejected != 0 || len(prov.requests) != 2 {
		t.Fatalf("rejected finals = %d, provider requests = %d", rejected, len(prov.requests))
	}
	if _, err := os.Stat(report); err != nil {
		t.Fatalf("report was not created: %v", err)
	}
	paths, participated, valid := a.ArtifactDeliveryTurnDecision("turn")
	if !participated || !valid || len(paths) != 0 {
		t.Fatalf("turn decision = %v, participated=%v, valid=%v", paths, participated, valid)
	}
}

func TestRunAcceptsExplicitEmptyDeliveryForObservedFile(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: root}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	prov := &mockProvider{name: "empty-manifest", streams: [][]provider.Chunk{
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write", Name: "write_file", Arguments: `{"path":"index.html","content":"<main></main>"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest", Name: "deliver_files", Arguments: `{"paths":[]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "source updated"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	a := New(prov, registry, session, Options{}, event.Discard)
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	if err := a.Run(ctx, "create application source"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(prov.requests) != 3 {
		t.Fatalf("provider requests = %d, want write, explicit empty delivery, and final", len(prov.requests))
	}
	if got := ArtifactDeliveryCandidates(session.Snapshot()); len(got) != 0 {
		t.Fatalf("empty manifest published files: %v", got)
	}
}

func TestRunInvalidatesDeliveryAfterLaterMutation(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: root}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	prov := &mockProvider{name: "stale-manifest", streams: [][]provider.Chunk{
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write-first", Name: "write_file", Arguments: `{"path":"joke.md","content":"first"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest-first", Name: "deliver_files", Arguments: `{"paths":["joke.md"]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write-second", Name: "write_file", Arguments: `{"path":"joke.md","content":"second"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "premature final"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest-second", Name: "deliver_files", Arguments: `{"paths":["joke.md"]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	var rejected int
	a := New(prov, registry, session, Options{}, event.FuncSink(func(emitted event.Event) {
		if emitted.Kind == event.FinalCandidateRejected {
			rejected++
		}
	}))
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	if err := a.Run(ctx, "create and revise a joke file"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rejected != 1 || len(prov.requests) != 6 {
		t.Fatalf("rejected finals = %d, provider requests = %d", rejected, len(prov.requests))
	}
	content, err := os.ReadFile(filepath.Join(root, "joke.md"))
	if err != nil || string(content) != "second" {
		t.Fatalf("final file = %q, %v", content, err)
	}
}

func TestRunKeepsArtifactDecisionAcrossSameLifecycleContinuations(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: root}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	prov := &mockProvider{name: "continued-artifact-turn", streams: [][]provider.Chunk{
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write-first", Name: "write_file", Arguments: `{"path":"joke.md","content":"first"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest-first", Name: "deliver_files", Arguments: `{"paths":["joke.md"]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "first run complete"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write-second", Name: "write_file", Arguments: `{"path":"joke.md","content":"second"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "premature continuation final"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest-second", Name: "deliver_files", Arguments: `{"paths":["joke.md"]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "second run complete"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("system")
	const artifactTurnID = "continued-turn"
	session.BeginArtifactDeliveryTurn(artifactTurnID)
	var rejected int
	a := New(prov, registry, session, Options{}, event.FuncSink(func(emitted event.Event) {
		if emitted.Kind == event.FinalCandidateRejected {
			rejected++
		}
	}))
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	if err := a.Run(ctx, "create a joke file"); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := a.Run(ctx, "revise the joke file during goal continuation"); err != nil {
		t.Fatalf("continued Run: %v", err)
	}
	if rejected != 1 || len(prov.requests) != 7 {
		t.Fatalf("rejected finals = %d, provider requests = %d; continuation mutation bypassed re-verification", rejected, len(prov.requests))
	}
	paths, participated, valid := a.ArtifactDeliveryTurnDecision(artifactTurnID)
	wantPath := filepath.Join(root, "joke.md")
	if !participated || !valid || len(paths) != 1 || filepath.Clean(paths[0]) != filepath.Clean(wantPath) {
		t.Fatalf("continued turn decision = %v, participated=%v, valid=%v", paths, participated, valid)
	}
	content, err := os.ReadFile(filepath.Join(root, "joke.md"))
	if err != nil || string(content) != "second" {
		t.Fatalf("final file = %q, %v", content, err)
	}
}

type failingWorkspaceMutationTool struct {
	path string
}

func (failingWorkspaceMutationTool) Name() string        { return "bash" }
func (failingWorkspaceMutationTool) Description() string { return "delete a workspace file, then fail" }
func (failingWorkspaceMutationTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (failingWorkspaceMutationTool) ReadOnly() bool { return false }
func (tool failingWorkspaceMutationTool) Execute(context.Context, json.RawMessage) (string, error) {
	if err := os.Remove(tool.path); err != nil {
		return "", err
	}
	return "file removed before command failed", errors.New("command failed after mutation")
}

func TestRunInvalidatesDeliveryAfterFailedWorkspaceMutation(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: root}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	registry.Add(failingWorkspaceMutationTool{path: filepath.Join(root, "joke.md")})
	prov := &mockProvider{name: "failed-stale-manifest", streams: [][]provider.Chunk{
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write", Name: "write_file", Arguments: `{"path":"joke.md","content":"joke"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest-first", Name: "deliver_files", Arguments: `{"paths":["joke.md"]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "delete-fails", Name: "bash", Arguments: `{"command":"remove joke"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "premature final"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest-empty", Name: "deliver_files", Arguments: `{"paths":[]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	var rejected int
	a := New(prov, registry, session, Options{}, event.FuncSink(func(emitted event.Event) {
		if emitted.Kind == event.FinalCandidateRejected {
			rejected++
		}
	}))
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	if err := a.Run(ctx, "create and then remove a joke file"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rejected != 1 || len(prov.requests) != 6 {
		t.Fatalf("rejected finals = %d, provider requests = %d", rejected, len(prov.requests))
	}
	if got := ArtifactDeliveryCandidates(session.Snapshot()); len(got) != 0 {
		t.Fatalf("latest empty manifest published files: %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "joke.md")); !os.IsNotExist(err) {
		t.Fatalf("failed mutation did not remove the test file: %v", err)
	}
}

type nonWorkspaceMutationTool struct{}

func (nonWorkspaceMutationTool) Name() string            { return "remote_state_update" }
func (nonWorkspaceMutationTool) Description() string     { return "update remote state only" }
func (nonWorkspaceMutationTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (nonWorkspaceMutationTool) ReadOnly() bool          { return false }
func (nonWorkspaceMutationTool) toolArtifactCollectionPolicy(context.Context, json.RawMessage) toolArtifactCollectionPolicy {
	return toolArtifactCollectionPolicy{}
}
func (nonWorkspaceMutationTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "remote state updated", nil
}

func TestRunKeepsDeliveryAfterNonWorkspaceMutation(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	for _, candidate := range (builtin.Workspace{Dir: root}).Tools("write_file", "deliver_files") {
		registry.Add(candidate)
	}
	registry.Add(nonWorkspaceMutationTool{})
	prov := &mockProvider{name: "remote-after-manifest", streams: [][]provider.Chunk{
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write", Name: "write_file", Arguments: `{"path":"joke.md","content":"joke"}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "manifest", Name: "deliver_files", Arguments: `{"paths":["joke.md"]}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "remote", Name: "remote_state_update", Arguments: `{}`}}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	var rejected int
	a := New(prov, registry, session, Options{}, event.FuncSink(func(emitted event.Event) {
		if emitted.Kind == event.FinalCandidateRejected {
			rejected++
		}
	}))
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	if err := a.Run(ctx, "create a joke file and update remote state"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rejected != 0 || len(prov.requests) != 4 {
		t.Fatalf("rejected finals = %d, provider requests = %d", rejected, len(prov.requests))
	}
	want := filepath.Join(root, "joke.md")
	if got := ArtifactDeliveryCandidates(session.Snapshot()); len(got) != 1 || filepath.Clean(got[0]) != filepath.Clean(want) {
		t.Fatalf("explicit manifest = %v, want canonical path %q", got, want)
	}
}

func TestLoadArtifactDeliveriesStreamsPersistedFinalContracts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	session := NewSession("system")
	session.Add(provider.Message{Role: provider.RoleTool, Content: strings.Repeat("large output", 10000)})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "first",
		ArtifactDelivery: &provider.ArtifactDelivery{Version: 1, Paths: []string{"first.md"}, DeliveredAt: 100},
	})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "second",
		ArtifactDelivery: &provider.ArtifactDelivery{Version: 1, Paths: []string{}, DeliveredAt: 200},
	})
	if err := session.Save(path); err != nil {
		t.Fatalf("save session: %v", err)
	}

	deliveries, err := LoadArtifactDeliveries(path)
	if err != nil {
		t.Fatalf("LoadArtifactDeliveries: %v", err)
	}
	if len(deliveries) != 2 || deliveries[0].DeliveredAt != 100 || len(deliveries[0].Paths) != 1 || deliveries[1].Paths == nil {
		t.Fatalf("deliveries = %#v", deliveries)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session disappeared: %v", err)
	}
}

func TestLoadArtifactDeliveryContractsReportsPresenceState(t *testing.T) {
	tests := []struct {
		name           string
		records        string
		wantState      ArtifactDeliveryContractState
		wantDeliveries int
	}{
		{
			name:      "absent",
			records:   `{"role":"assistant","message_kind":"final_answer","content":"report.md"}` + "\n",
			wantState: ArtifactDeliveryContractAbsent,
		},
		{
			name:           "valid",
			records:        `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"paths":[],"deliveredAt":100}}` + "\n",
			wantState:      ArtifactDeliveryContractValid,
			wantDeliveries: 1,
		},
		{
			name:           "valid explicit customer format",
			records:        `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"paths":[".customer-delivery/report.custom"],"deliveredAt":100}}` + "\n",
			wantState:      ArtifactDeliveryContractValid,
			wantDeliveries: 1,
		},
		{
			name:      "unknown version",
			records:   `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":2,"paths":["report.md"],"deliveredAt":100}}` + "\n",
			wantState: ArtifactDeliveryContractInvalid,
		},
		{
			name:      "missing paths",
			records:   `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"deliveredAt":100}}` + "\n",
			wantState: ArtifactDeliveryContractInvalid,
		},
		{
			name:      "wrong shape",
			records:   `{"role":"assistant","message_kind":"final_answer","artifact_delivery":"invalid"}` + "\n",
			wantState: ArtifactDeliveryContractInvalid,
		},
		{
			name:           "explicit source path",
			records:        `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"paths":["src/main.go"],"deliveredAt":100}}` + "\n",
			wantState:      ArtifactDeliveryContractValid,
			wantDeliveries: 1,
		},
		{
			name:           "explicit log path",
			records:        `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"paths":["logs/run.log"],"deliveredAt":100}}` + "\n",
			wantState:      ArtifactDeliveryContractValid,
			wantDeliveries: 1,
		},
		{
			name:           "explicit hidden build path",
			records:        `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"paths":[".customer-delivery/build/output.json"],"deliveredAt":100}}` + "\n",
			wantState:      ArtifactDeliveryContractValid,
			wantDeliveries: 1,
		},
		{
			name:           "mixed explicit deliverable and source paths",
			records:        `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"paths":["report.md",".customer-delivery/src/main.go"],"deliveredAt":100}}` + "\n",
			wantState:      ArtifactDeliveryContractValid,
			wantDeliveries: 1,
		},
		{
			name:      "explicit null is absent",
			records:   `{"role":"assistant","message_kind":"final_answer","artifact_delivery":null}` + "\n",
			wantState: ArtifactDeliveryContractAbsent,
		},
		{
			name: "invalid dominates while valid contracts remain available",
			records: `{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":1,"paths":["report.md"],"deliveredAt":100}}` + "\n" +
				`{"role":"assistant","message_kind":"final_answer","artifact_delivery":{"version":9,"paths":[],"deliveredAt":200}}` + "\n",
			wantState:      ArtifactDeliveryContractInvalid,
			wantDeliveries: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.jsonl")
			if err := os.WriteFile(path, []byte(tt.records), 0o600); err != nil {
				t.Fatalf("write session: %v", err)
			}
			loaded, err := LoadArtifactDeliveryContracts(path)
			if err != nil {
				t.Fatalf("LoadArtifactDeliveryContracts: %v", err)
			}
			if loaded.State != tt.wantState || len(loaded.Deliveries) != tt.wantDeliveries {
				t.Fatalf("loaded contracts = %#v, want state %v and %d deliveries", loaded, tt.wantState, tt.wantDeliveries)
			}
			if got := loaded.HasContract(); got != (tt.wantState != ArtifactDeliveryContractAbsent) {
				t.Fatalf("HasContract = %v for state %v", got, tt.wantState)
			}
		})
	}
}

func TestSessionClonePreservesMalformedArtifactDeliveryPaths(t *testing.T) {
	session := NewSessionFromMessages([]provider.Message{
		{
			Role:             provider.RoleAssistant,
			MessageKind:      provider.MessageKindFinalAnswer,
			ArtifactDelivery: &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, DeliveredAt: 100},
		},
	})
	snapshot := session.Snapshot()
	if len(snapshot) != 1 || snapshot[0].ArtifactDelivery == nil {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot[0].ArtifactDelivery.Paths != nil {
		t.Fatalf("snapshot paths = %#v, want nil malformed marker", snapshot[0].ArtifactDelivery.Paths)
	}
}
