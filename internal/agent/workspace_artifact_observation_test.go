package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type observationWriterTool struct {
	root  string
	err   bool
	noise bool
}

func (observationWriterTool) Name() string            { return "document_generator" }
func (observationWriterTool) Description() string     { return "generate a document" }
func (observationWriterTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (observationWriterTool) ReadOnly() bool          { return false }
func (observationWriterTool) ObserveWorkspaceArtifactChanges(json.RawMessage) bool {
	return true
}
func (w observationWriterTool) Execute(context.Context, json.RawMessage) (string, error) {
	files := map[string]string{"generated.pdf": "pdf"}
	if w.noise {
		files["generator.log"] = "log"
		files["coverage/details.md"] = "coverage"
		files["README.md"] = "engineering metadata"
		files["docs/index.html"] = "engineering docs"
		files["src/assets/logo.png"] = "engineering asset"
		files["src/generated.go"] = "package generated"
		files["result.yml"] = "status: passed"
	}
	for relative, content := range files {
		path := filepath.Join(w.root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	if w.err {
		return "partial", errors.New("generation failed")
	}
	return "generated", nil
}

type unobservedWorkspaceWriter struct {
	root string
	name string
}

func (w unobservedWorkspaceWriter) Name() string { return w.name }
func (unobservedWorkspaceWriter) Description() string {
	return "write without an artifact observation capability"
}
func (unobservedWorkspaceWriter) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (unobservedWorkspaceWriter) ReadOnly() bool { return false }
func (w unobservedWorkspaceWriter) Execute(context.Context, json.RawMessage) (string, error) {
	return "written", os.WriteFile(filepath.Join(w.root, "unattributed.pdf"), []byte("partial"), 0o644)
}

type artifactObservationPolicyTool struct {
	name  string
	optIn bool
}

func (t artifactObservationPolicyTool) Name() string      { return t.name }
func (artifactObservationPolicyTool) Description() string { return "policy test" }
func (artifactObservationPolicyTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (artifactObservationPolicyTool) ReadOnly() bool { return false }
func (artifactObservationPolicyTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "", nil
}
func (t artifactObservationPolicyTool) ObserveWorkspaceArtifactChanges(json.RawMessage) bool {
	return t.optIn
}

func TestWorkspaceArtifactObservationBindsExactWriterToChangedTargets(t *testing.T) {
	root := t.TempDir()
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	observation := NewWorkspaceArtifactObservation(root)
	ctx := WithWorkspaceArtifactObservation(context.Background(), observation)
	call := beginWorkspaceArtifactToolCall(ctx, "write_file", json.RawMessage(`{"path":"joke.md","content":"joke"}`), false)
	if err := os.WriteFile(filepath.Join(root, "joke.md"), []byte("joke"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts := call.complete(true)
	if len(artifacts) != 1 || filepath.Base(artifacts[0].Path) != "joke.md" || artifacts[0].Source != SubagentArtifactSourceWriter {
		t.Fatalf("exact writer artifacts = %+v", artifacts)
	}

	readmeCall := beginWorkspaceArtifactToolCall(ctx, "write_file", json.RawMessage(`{"path":"README.md","content":"after"}`), false)
	if err := os.WriteFile(readme, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if artifacts := readmeCall.complete(true); len(artifacts) != 0 {
		t.Fatalf("existing source metadata became a delivery: %+v", artifacts)
	}

	agentsCall := beginWorkspaceArtifactToolCall(ctx, "write_file", json.RawMessage(`{"path":"AGENTS.md","content":"instructions"}`), false)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("instructions"), 0o644); err != nil {
		t.Fatal(err)
	}
	if artifacts := agentsCall.complete(true); len(artifacts) != 0 {
		t.Fatalf("new engineering metadata became a delivery: %+v", artifacts)
	}
}

func TestExactWriterReceiptsUseExplicitPathPolicyWithoutExtensionAllowlist(t *testing.T) {
	root := t.TempDir()
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	tests := []struct {
		path string
		want bool
	}{
		{path: "output.json", want: true},
		{path: "output.yaml", want: true},
		{path: "output.customer-format", want: true},
		{path: ".customer-delivery/report.custom", want: true},
		{path: "run.log"},
		{path: "main.go"},
		{path: "build/output.json"},
		{path: "coverage/output.yaml"},
		{path: "logs/output.customer-format"},
		{path: "src/output.json"},
		{path: "tests/output.yaml"},
		{path: ".tooling/output.customer-format"},
		{path: ".customer-delivery/build/output.json"},
		{path: ".customer-delivery/src/main.go"},
		{path: ".customer-delivery/logs/run.log"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			call := beginWorkspaceArtifactToolCall(ctx, "write_file", json.RawMessage(`{"path":"`+test.path+`","content":"result"}`), false)
			if call == nil {
				t.Fatal("exact writer did not produce an observation call")
			}
			path := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("result"), 0o644); err != nil {
				t.Fatal(err)
			}
			artifacts := call.complete(true)
			if got := len(artifacts) == 1; got != test.want {
				t.Fatalf("artifacts = %+v, want accepted=%v", artifacts, test.want)
			}
		})
	}
}

func TestWorkspaceArtifactObservationCapturesOpaqueDocumentWithoutNoise(t *testing.T) {
	root := t.TempDir()
	observation := NewWorkspaceArtifactObservation(root)
	registry := tool.NewRegistry()
	registry.Add(observationWriterTool{root: root, noise: true})
	agent := New(nil, registry, NewSession("system"), Options{}, event.Discard)
	ctx := WithWorkspaceArtifactObservation(context.Background(), observation)

	outcome := agent.executeOneWithRegistry(ctx, registry, providerToolCall("document_generator"))
	if outcome.errMsg != "" {
		t.Fatalf("tool outcome error = %q", outcome.errMsg)
	}
	if len(outcome.artifacts) != 1 || filepath.Base(outcome.artifacts[0].Path) != "generated.pdf" || outcome.artifacts[0].Source != SubagentArtifactSourceSnapshot {
		t.Fatalf("opaque artifacts = %+v, want only generated.pdf", outcome.artifacts)
	}
}

func TestWorkspaceArtifactObservationIncludesModifiedDocumentAndPriorMarkdown(t *testing.T) {
	root := t.TempDir()
	document := filepath.Join(root, "report.docx")
	markdown := filepath.Join(root, "report.md")
	for path, content := range map[string]string{document: "before-doc", markdown: "before-md"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	observation := NewWorkspaceArtifactObservation(root, "report.md")
	ctx := WithWorkspaceArtifactObservation(context.Background(), observation)
	for _, test := range []struct {
		path    string
		content string
	}{
		{path: "report.docx", content: "after-document-is-longer"},
		{path: "report.md", content: "after markdown is longer"},
	} {
		call := beginWorkspaceArtifactToolCall(ctx, "write_file", json.RawMessage(`{"path":"`+test.path+`","content":"x"}`), false)
		if err := os.WriteFile(filepath.Join(root, test.path), []byte(test.content), 0o644); err != nil {
			t.Fatal(err)
		}
		artifacts := call.complete(true)
		if len(artifacts) != 1 || filepath.Base(artifacts[0].Path) != test.path {
			t.Fatalf("modified %s artifacts = %+v", test.path, artifacts)
		}
	}
}

func TestWorkspaceArtifactObservationFailsClosedAndRejectsFailedPartialOutput(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "missing")
	observation := NewWorkspaceArtifactObservation(missingRoot)
	ctx := WithWorkspaceArtifactObservation(context.Background(), observation)
	if call := beginWorkspaceArtifactToolCall(ctx, "write_file", json.RawMessage(`{"path":"report.pdf"}`), false); call != nil {
		t.Fatalf("invalid workspace produced a tool observation: %+v", call)
	}

	root := t.TempDir()
	observation = NewWorkspaceArtifactObservation(root)
	registry := tool.NewRegistry()
	registry.Add(observationWriterTool{root: root, err: true})
	agent := New(nil, registry, NewSession("system"), Options{}, event.Discard)
	ctx = WithWorkspaceArtifactObservation(context.Background(), observation)
	outcome := agent.executeOneWithRegistry(ctx, registry, providerToolCall("document_generator"))
	if outcome.errMsg == "" {
		t.Fatal("failed writer returned no error")
	}
	if len(outcome.artifacts) != 0 {
		t.Fatalf("failed partial output became a delivery: %+v", outcome.artifacts)
	}
}

func TestWorkspaceArtifactObservationPolicyFailsClosed(t *testing.T) {
	foregroundBash := artifactObservationPolicyTool{name: "bash"}
	if !workspaceArtifactObservationAllowed(foregroundBash, json.RawMessage(`{"command":"generate"}`)) {
		t.Fatal("foreground bash was not admitted")
	}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"command":"generate","run_in_background":true}`),
		json.RawMessage(`{"command":"generate","preserve_background_processes":true}`),
	} {
		if workspaceArtifactObservationAllowed(foregroundBash, args) {
			t.Fatalf("unsafe bash args were admitted: %s", args)
		}
	}
	if workspaceArtifactObservationAllowed(artifactObservationPolicyTool{name: "mcp__files__export"}, json.RawMessage(`{}`)) {
		t.Fatal("unknown dynamic writer was admitted")
	}
	if !workspaceArtifactObservationAllowed(artifactObservationPolicyTool{name: "document_generator", optIn: true}, json.RawMessage(`{}`)) {
		t.Fatal("audited opaque writer opt-in was ignored")
	}
}

func TestWorkspaceArtifactObservationUnknownWriterProducesNoReceipt(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	registry.Add(unobservedWorkspaceWriter{root: root, name: "mcp__files__export"})
	runner := New(nil, registry, NewSession("system"), Options{}, event.Discard)
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))

	outcome := runner.executeOneWithRegistry(ctx, registry, providerToolCall("mcp__files__export"))
	if outcome.errMsg != "" {
		t.Fatalf("tool outcome error = %q", outcome.errMsg)
	}
	if len(outcome.artifacts) != 0 {
		t.Fatalf("unknown writer produced artifacts: %+v", outcome.artifacts)
	}
	if _, err := os.Stat(filepath.Join(root, "unattributed.pdf")); err != nil {
		t.Fatalf("test writer did not create its file: %v", err)
	}
}

func TestWorkspaceArtifactObservationDetectsSameMetadataRewrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "report.docx")
	fixed := time.Unix(1_700_000_000, 0)
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	call := beginWorkspaceArtifactToolCall(ctx, "write_file", json.RawMessage(`{"path":"report.docx","content":"after!"}`), false)
	if err := os.WriteFile(path, []byte("after!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	artifacts := call.complete(true)
	if len(artifacts) != 1 || filepath.Base(artifacts[0].Path) != "report.docx" {
		t.Fatalf("same-metadata rewrite artifacts = %+v", artifacts)
	}
}

func TestWorkspaceArtifactObservationCancelledScanFailsClosed(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx = WithWorkspaceArtifactObservation(ctx, NewWorkspaceArtifactObservation(root))
	if call := beginWorkspaceArtifactToolCall(ctx, "document_generator", json.RawMessage(`{}`), false); call != nil {
		t.Fatalf("cancelled workspace scan returned a call: %+v", call)
	}
}

func providerToolCall(name string) provider.ToolCall {
	return provider.ToolCall{ID: "tool-call", Name: name, Arguments: `{}`}
}
