package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type collectorPathWriter struct {
	root string
}

func (collectorPathWriter) Name() string            { return "write_file" }
func (collectorPathWriter) Description() string     { return "write one test artifact" }
func (collectorPathWriter) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (collectorPathWriter) ReadOnly() bool          { return false }
func (collectorPathWriter) toolArtifactCollectionPolicy(context.Context, json.RawMessage) toolArtifactCollectionPolicy {
	return toolArtifactCollectionPolicy{observeWorkspace: true, includeNested: true}
}
func (w collectorPathWriter) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", err
	}
	path := filepath.FromSlash(params.Path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(w.root, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(params.Content), 0o644); err != nil {
		return "", err
	}
	return "written", nil
}

type collectorNestedTool struct {
	name string
	run  func(context.Context) (string, error)
}

func (t collectorNestedTool) Name() string          { return t.name }
func (collectorNestedTool) Description() string     { return "invoke a nested test tool" }
func (collectorNestedTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (collectorNestedTool) ReadOnly() bool          { return true }
func (t collectorNestedTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	return t.run(ctx)
}

type collectorUnobservedMutationTool struct {
	root string
	fail bool
}

func (collectorUnobservedMutationTool) Name() string { return "mcp__dynamic__export" }
func (collectorUnobservedMutationTool) Description() string {
	return "write through a dynamic tool without artifact observation"
}
func (collectorUnobservedMutationTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (collectorUnobservedMutationTool) ReadOnly() bool { return false }
func (t collectorUnobservedMutationTool) Execute(context.Context, json.RawMessage) (string, error) {
	if err := os.WriteFile(filepath.Join(t.root, "dynamic-output.md"), []byte("changed"), 0o644); err != nil {
		return "", err
	}
	if t.fail {
		return "partial output", errors.New("dynamic export failed after mutation")
	}
	return "exported", nil
}

func TestUnknownNonReadOnlyToolInvalidatesManifestWithoutPublishingArtifacts(t *testing.T) {
	for _, test := range []struct {
		name string
		fail bool
	}{
		{name: "success"},
		{name: "failure", fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			registry := tool.NewRegistry()
			registry.Add(collectorUnobservedMutationTool{root: root, fail: test.fail})
			runner := New(nil, registry, NewSession("system"), Options{}, event.Discard)
			runner.evidence.Record(evidence.Receipt{
				ToolName:         "deliver_files",
				Success:          true,
				ArtifactDecision: true,
			})
			ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))

			outcome := runner.executeOneWithRegistry(ctx, registry, provider.ToolCall{
				ID: "dynamic", Name: "mcp__dynamic__export", Arguments: `{}`,
			})
			if (outcome.errMsg != "") != test.fail {
				t.Fatalf("outcome error = %q, want failure %v", outcome.errMsg, test.fail)
			}
			if len(outcome.artifacts) != 0 {
				t.Fatalf("unobserved dynamic tool published artifacts: %+v", outcome.artifacts)
			}
			if _, err := os.Stat(filepath.Join(root, "dynamic-output.md")); err != nil {
				t.Fatalf("dynamic tool did not mutate the workspace: %v", err)
			}
			paths, pending := runner.evidence.PendingArtifactDelivery()
			if !pending || len(paths) != 0 {
				t.Fatalf("pending artifact delivery = %v, %v; want invalidated manifest without candidates", paths, pending)
			}
		})
	}
}

func TestNestedFailedToolDoesNotLeakSuccessfulDescendantArtifacts(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	registry.Add(collectorPathWriter{root: root})

	var runner *Agent
	registry.Add(collectorNestedTool{name: "failed_child", run: func(ctx context.Context) (string, error) {
		outcome := runner.executeOneWithRegistry(ctx, registry, provider.ToolCall{
			ID: "write", Name: "write_file", Arguments: `{"path":"discarded.md","content":"partial"}`,
		})
		if outcome.errMsg != "" {
			return "", errors.New(outcome.errMsg)
		}
		return "child failed after writing", errors.New("child failed")
	}})
	registry.Add(collectorNestedTool{name: "successful_parent", run: func(ctx context.Context) (string, error) {
		outcome := runner.executeOneWithRegistry(ctx, registry, provider.ToolCall{
			ID: "failed-child", Name: "failed_child", Arguments: `{}`,
		})
		if outcome.errMsg == "" {
			return "", errors.New("failed child unexpectedly succeeded")
		}
		return "failure handled", nil
	}})
	runner = New(nil, registry, NewSession("system"), Options{}, event.Discard)
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))

	outcome := runner.executeOneWithRegistry(ctx, registry, provider.ToolCall{
		ID: "parent", Name: "successful_parent", Arguments: `{}`,
	})
	if outcome.errMsg != "" {
		t.Fatalf("parent outcome error = %q", outcome.errMsg)
	}
	if len(outcome.artifacts) != 0 {
		t.Fatalf("failed nested call leaked artifacts: %+v", outcome.artifacts)
	}
	if _, err := os.Stat(filepath.Join(root, "discarded.md")); err != nil {
		t.Fatalf("test descendant did not create its partial file: %v", err)
	}
}

func TestToolArtifactCollectorConcurrentEquivalentPathsDeduplicate(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, "reports", "answer.md")
	equivalent := filepath.Dir(canonical) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(canonical)
	collector := newToolArtifactCollector()

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, artifact := range []provider.ToolArtifact{
		{Path: canonical, Source: SubagentArtifactSourceSnapshot},
		{Path: equivalent, Source: SubagentArtifactSourceWriter},
		{Path: canonical, Source: SubagentArtifactSourceManifest},
	} {
		artifact := artifact
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			collector.add([]provider.ToolArtifact{artifact})
		}()
	}
	close(start)
	wg.Wait()

	got := collector.snapshot()
	if len(got) != 1 || artifactDeliveryPathKey(got[0].Path) != artifactDeliveryPathKey(canonical) {
		t.Fatalf("collector artifacts = %+v, want one canonical-equivalent path", got)
	}
	if got[0].Source != SubagentArtifactSourceManifest {
		t.Fatalf("collector source = %q, want strongest manifest receipt", got[0].Source)
	}
	got[0].Path = "mutated"
	if snapshot := collector.snapshot(); len(snapshot) != 1 || snapshot[0].Path == "mutated" {
		t.Fatalf("collector exposed mutable state: %+v", snapshot)
	}
}

type collectorParallelProvider struct{}

func (collectorParallelProvider) Name() string { return "collector-parallel" }

func (collectorParallelProvider) Stream(_ context.Context, request provider.Request) (<-chan provider.Chunk, error) {
	prompt := collectorLastUser(request)
	hasToolResult := false
	for _, message := range request.Messages {
		if message.Role == provider.RoleTool && message.Name == "write_file" {
			hasToolResult = true
			break
		}
	}

	chunks := make(chan provider.Chunk, 2)
	if !hasToolResult {
		path := "successful.json"
		if prompt == "failed branch" {
			path = "failed.json"
		}
		args, _ := json.Marshal(map[string]string{"path": path, "content": prompt})
		chunks <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "write-" + path, Name: "write_file", Arguments: string(args),
		}}
		chunks <- provider.Chunk{Type: provider.ChunkDone}
		close(chunks)
		return chunks, nil
	}
	if prompt == "failed branch" {
		chunks <- provider.Chunk{Type: provider.ChunkError, Err: errors.New("branch failed after write")}
		close(chunks)
		return chunks, nil
	}
	chunks <- provider.Chunk{Type: provider.ChunkText, Text: "successful branch complete"}
	chunks <- provider.Chunk{Type: provider.ChunkDone}
	close(chunks)
	return chunks, nil
}

func collectorLastUser(request provider.Request) string {
	for i := len(request.Messages) - 1; i >= 0; i-- {
		if request.Messages[i].Role == provider.RoleUser {
			return request.Messages[i].Content
		}
	}
	return ""
}

func TestParallelTasksPublishesOnlySuccessfulBranchArtifacts(t *testing.T) {
	root := t.TempDir()
	registry := tool.NewRegistry()
	registry.Add(collectorPathWriter{root: root})
	task := newArtifactHandoffTestTask(t, collectorParallelProvider{}, registry, root)
	parallel := NewParallelTasksTool(task, registry)
	registry.Add(parallel)
	runner := New(nil, registry, NewSession("system"), Options{}, event.Discard)
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))
	args := `{"tasks":[{"prompt":"successful branch","tools":["write_file"]},{"prompt":"failed branch","tools":["write_file"]}]}`

	outcome := runner.executeOneWithRegistry(ctx, registry, provider.ToolCall{
		ID: "parallel", Name: "parallel_tasks", Arguments: args,
	})
	if outcome.errMsg != "" {
		t.Fatalf("parallel outcome error = %q\n%s", outcome.errMsg, outcome.output)
	}
	if len(outcome.artifacts) != 1 || filepath.Base(outcome.artifacts[0].Path) != "successful.json" {
		t.Fatalf("parallel artifacts = %+v, want only successful branch", outcome.artifacts)
	}
	if _, err := os.Stat(filepath.Join(root, "failed.json")); err != nil {
		t.Fatalf("failed branch did not leave its partial file for attribution test: %v", err)
	}
}

type collectorOpaqueParallelTool struct {
	root          string
	started       chan<- struct{}
	failedWritten <-chan struct{}
}

func (collectorOpaqueParallelTool) Name() string        { return "opaque_export" }
func (collectorOpaqueParallelTool) Description() string { return "write an opaque test export" }
func (collectorOpaqueParallelTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (collectorOpaqueParallelTool) ReadOnly() bool { return false }
func (collectorOpaqueParallelTool) ObserveWorkspaceArtifactChanges(json.RawMessage) bool {
	return true
}
func (t collectorOpaqueParallelTool) Execute(context.Context, json.RawMessage) (string, error) {
	t.started <- struct{}{}
	<-t.failedWritten
	if err := os.WriteFile(filepath.Join(t.root, "successful.pdf"), []byte("success"), 0o644); err != nil {
		return "", err
	}
	return "exported", nil
}

type collectorFailingParallelWriter struct {
	root          string
	opaqueStarted <-chan struct{}
	failedWritten chan<- struct{}
}

func (collectorFailingParallelWriter) Name() string        { return "write_file" }
func (collectorFailingParallelWriter) Description() string { return "write then fail" }
func (collectorFailingParallelWriter) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (collectorFailingParallelWriter) ReadOnly() bool { return false }
func (t collectorFailingParallelWriter) Execute(context.Context, json.RawMessage) (string, error) {
	<-t.opaqueStarted
	if err := os.WriteFile(filepath.Join(t.root, "failed.pdf"), []byte("partial"), 0o644); err != nil {
		return "", err
	}
	t.failedWritten <- struct{}{}
	return "partial", errors.New("failed after write")
}

type collectorOpaqueParallelProvider struct{}

func (collectorOpaqueParallelProvider) Name() string { return "collector-opaque-parallel" }

func (collectorOpaqueParallelProvider) Stream(_ context.Context, request provider.Request) (<-chan provider.Chunk, error) {
	prompt := collectorLastUser(request)
	hasToolResult := false
	for _, message := range request.Messages {
		if message.Role == provider.RoleTool {
			hasToolResult = true
			break
		}
	}
	chunks := make(chan provider.Chunk, 2)
	if !hasToolResult {
		name := "opaque_export"
		if prompt == "failed branch" {
			name = "write_file"
		}
		chunks <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: name + "-call", Name: name, Arguments: `{}`}}
		chunks <- provider.Chunk{Type: provider.ChunkDone}
	} else if prompt == "failed branch" {
		chunks <- provider.Chunk{Type: provider.ChunkError, Err: errors.New("branch failed")}
	} else {
		chunks <- provider.Chunk{Type: provider.ChunkText, Text: "successful branch complete"}
		chunks <- provider.Chunk{Type: provider.ChunkDone}
	}
	close(chunks)
	return chunks, nil
}

func TestParallelTasksDoNotCrossAttributeOpaqueWorkspaceChanges(t *testing.T) {
	root := t.TempDir()
	opaqueStarted := make(chan struct{}, 1)
	failedWritten := make(chan struct{}, 1)
	registry := tool.NewRegistry()
	registry.Add(collectorOpaqueParallelTool{root: root, started: opaqueStarted, failedWritten: failedWritten})
	registry.Add(collectorFailingParallelWriter{root: root, opaqueStarted: opaqueStarted, failedWritten: failedWritten})
	task := newArtifactHandoffTestTask(t, collectorOpaqueParallelProvider{}, registry, root)
	parallel := NewParallelTasksTool(task, registry)
	registry.Add(parallel)
	runner := New(nil, registry, NewSession("system"), Options{}, event.Discard)
	ctx := WithWorkspaceArtifactObservation(context.Background(), NewWorkspaceArtifactObservation(root))

	outcome := runner.executeOneWithRegistry(ctx, registry, provider.ToolCall{
		ID: "parallel", Name: "parallel_tasks",
		Arguments: `{"tasks":[{"prompt":"successful branch","tools":["opaque_export"]},{"prompt":"failed branch","tools":["write_file"]}]}`,
	})
	if outcome.errMsg != "" {
		t.Fatalf("parallel outcome error = %q\n%s", outcome.errMsg, outcome.output)
	}
	if len(outcome.artifacts) != 0 {
		t.Fatalf("parallel opaque scan cross-attributed artifacts: %+v", outcome.artifacts)
	}
	for _, name := range []string{"successful.pdf", "failed.pdf"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("test file %s was not created: %v", name, err)
		}
	}
}

func TestManagedTaskArtifactPolicyIsolatesBackgroundCollector(t *testing.T) {
	manager := jobs.NewManager(event.Discard)
	defer manager.Close()
	ctx := WithParentSession(context.Background(), "session.jsonl")
	ctx = jobs.WithManager(ctx, manager)

	managed := artifactCollectionPolicyFor(ctx, &TaskTool{}, json.RawMessage(`{}`))
	if managed.observeWorkspace || managed.includeNested {
		t.Fatalf("managed task policy = %+v, want fully isolated dispatch", managed)
	}
	foreground := artifactCollectionPolicyFor(context.Background(), &TaskTool{}, json.RawMessage(`{}`))
	if foreground.observeWorkspace || !foreground.includeNested {
		t.Fatalf("foreground task policy = %+v, want nested receipts only", foreground)
	}
}
