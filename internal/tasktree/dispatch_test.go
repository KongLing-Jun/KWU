package tasktree

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/tool"
)

func TestRoutedDispatcherSendsReadOnlyNodesToReadOnlyTaskTool(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	tree.Nodes["patch"].Executor = &ExecutorSpec{Kind: ExecutorReadOnlyTask}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	reg := tool.NewRegistry()
	readOnly := &captureDispatchTool{name: "read_only_task", output: "read-only evidence"}
	task := &captureDispatchTool{name: "task", output: "write answer"}
	reg.Add(readOnly)
	reg.Add(task)
	sink := &captureSink{}
	orch := NewOrchestrator(Options{Store: store, Registry: reg, Sink: sink})

	if err := orch.ExecuteOne(context.Background(), "patch"); err != nil {
		t.Fatalf("ExecuteOne: %v", err)
	}
	if readOnly.calls != 1 {
		t.Fatalf("read_only_task calls = %d, want 1", readOnly.calls)
	}
	if task.calls != 0 {
		t.Fatalf("task calls = %d, want 0", task.calls)
	}
	args := decodeDispatchArgs(t, readOnly.lastArgs)
	if args.Description != "Patch implementation" {
		t.Fatalf("description = %q", args.Description)
	}
	if !strings.Contains(args.Prompt, `"node_id": "patch"`) ||
		!strings.Contains(args.Prompt, `"node_description": "Apply the context contract"`) ||
		!strings.Contains(args.Prompt, `"dependency_results"`) {
		t.Fatalf("prompt missing local node context:\n%s", args.Prompt)
	}

	final, _, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if final.Tree.Nodes["patch"].Result.Summary != "read-only evidence" {
		t.Fatalf("patch result = %+v", final.Tree.Nodes["patch"].Result)
	}
	assertTaskTreePhase(t, sink.events, "task_tree: node_dispatch read_only_task patch")
}

func TestRoutedDispatcherInjectsTaskTreeNodeContextForSubagentEvents(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	tree.Nodes["patch"].Executor = &ExecutorSpec{Kind: ExecutorTask}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	reg := tool.NewRegistry()
	task := &contextStreamingTaskTool{}
	reg.Add(task)
	sink := &captureSink{}
	orch := NewOrchestrator(Options{Store: store, Registry: reg, Sink: sink})

	if err := orch.ExecuteOne(context.Background(), "patch"); err != nil {
		t.Fatalf("ExecuteOne: %v", err)
	}
	if task.treeID != "tree-ctx" || task.nodeID != "patch" {
		t.Fatalf("tool task-tree context = %q/%q, want tree-ctx/patch", task.treeID, task.nodeID)
	}
	if task.parentID == "" || !strings.Contains(task.parentID, "patch") {
		t.Fatalf("tool call context parentID = %q, want synthetic node parent", task.parentID)
	}
	output := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeOutput, "patch", "")
	if output.OutputDelta != "subagent stream" {
		t.Fatalf("node output payload = %+v", output)
	}
	var toolEvent event.Event
	for _, e := range sink.events {
		if e.Kind == event.ToolDispatch && e.Tool.Name == "bash" {
			toolEvent = e
			break
		}
	}
	if toolEvent.Kind != event.ToolDispatch {
		t.Fatalf("missing nested tool event in %+v", sink.events)
	}
	if toolEvent.Tool.TaskTreeID != "tree-ctx" || toolEvent.Tool.NodeID != "patch" || toolEvent.Tool.ParentID != task.parentID {
		t.Fatalf("nested tool event ownership = %+v", toolEvent.Tool)
	}
}

func TestRoutedDispatcherSendsTaskNodesToTaskTool(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	tree.Nodes["patch"].Executor = &ExecutorSpec{Kind: ExecutorTask}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	reg := tool.NewRegistry()
	task := &captureDispatchTool{name: "task", output: "Subagent reference: sa_patch\n\nFinal answer:\npatched via subagent"}
	reg.Add(task)
	orch := NewOrchestrator(Options{Store: store, Registry: reg})

	if err := orch.ExecuteOne(context.Background(), "patch"); err != nil {
		t.Fatalf("ExecuteOne: %v", err)
	}
	if task.calls != 1 {
		t.Fatalf("task calls = %d, want 1", task.calls)
	}
	final, _, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	result := final.Tree.Nodes["patch"].Result
	if result.Summary != "patched via subagent" {
		t.Fatalf("summary = %q", result.Summary)
	}
	if result.RawOutputRef != "subagent://sa_patch" {
		t.Fatalf("raw output ref = %q", result.RawOutputRef)
	}
}

type contextStreamingTaskTool struct {
	treeID   string
	nodeID   string
	parentID string
}

func (t *contextStreamingTaskTool) Name() string { return "task" }

func (t *contextStreamingTaskTool) Description() string { return "context streaming task" }

func (t *contextStreamingTaskTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (t *contextStreamingTaskTool) ReadOnly() bool { return false }

func (t *contextStreamingTaskTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	taskCtx, ok := event.TaskTreeContextFromContext(ctx)
	if !ok {
		return "", errors.New("missing task tree context")
	}
	t.treeID = taskCtx.TreeID
	t.nodeID = taskCtx.NodeID
	parentID, _, _, ok := agent.CallContext(ctx)
	if !ok {
		return "", errors.New("missing call context")
	}
	t.parentID = parentID
	nested := agent.NestedSink(ctx, event.Discard)
	nested.Emit(event.Event{Kind: event.Text, Text: "subagent stream"})
	nested.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "tool_1", Name: "bash", Args: `{"command":"go test"}`}})
	return "final result", nil
}

func TestRoutedDispatcherFallsBackToMainExecutor(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	tree.Nodes["patch"].Executor = &ExecutorSpec{Kind: ExecutorReadOnlyTask}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	runner := &testRunner{}
	orch := NewOrchestrator(Options{Store: store, Executor: runner, Registry: tool.NewRegistry()})

	if err := orch.ExecuteOne(context.Background(), "patch"); err != nil {
		t.Fatalf("ExecuteOne: %v", err)
	}
	if len(runner.inputs) != 1 {
		t.Fatalf("main executor inputs = %d, want 1", len(runner.inputs))
	}
	if !strings.Contains(runner.inputs[0], `"node_id": "patch"`) {
		t.Fatalf("main executor prompt missing node context:\n%s", runner.inputs[0])
	}
}

func TestRoutedDispatcherConvertsSubagentToolErrorToFailedNode(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	tree.Nodes["patch"].Executor = &ExecutorSpec{Kind: ExecutorTask}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	dispatchErr := errors.New("subagent failed")
	reg := tool.NewRegistry()
	reg.Add(&captureDispatchTool{name: "task", output: "partial answer", err: dispatchErr})
	orch := NewOrchestrator(Options{Store: store, Registry: reg})

	err := orch.ExecuteOne(context.Background(), "patch")
	if !errors.Is(err, dispatchErr) {
		t.Fatalf("ExecuteOne error = %v, want subagent error", err)
	}
	final, _, loadErr := store.LoadSnapshot(context.Background())
	if loadErr != nil {
		t.Fatalf("LoadSnapshot: %v", loadErr)
	}
	node := final.Tree.Nodes["patch"]
	if node.Status != StatusFailed {
		t.Fatalf("patch status = %s, want failed", node.Status)
	}
	if node.Result == nil || node.Result.Summary != "partial answer" || node.Result.Error != "subagent failed" {
		t.Fatalf("patch result = %+v", node.Result)
	}
}

func TestWrapNodeOutputExtractsSubagentFinalAnswerAndReference(t *testing.T) {
	result := WrapNodeOutput("Subagent reference: sa_123\nTo continue this same subagent transcript...\n\nFinal answer:\nFound the issue.")
	if result.Summary != "Found the issue." {
		t.Fatalf("summary = %q", result.Summary)
	}
	if result.RawOutputRef != "subagent://sa_123" {
		t.Fatalf("raw output ref = %q", result.RawOutputRef)
	}
}

type captureDispatchTool struct {
	name     string
	output   string
	err      error
	readOnly bool
	calls    int
	lastArgs json.RawMessage
}

func (t *captureDispatchTool) Name() string { return t.name }

func (t *captureDispatchTool) Description() string { return t.name }

func (t *captureDispatchTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"prompt":{"type":"string"},"description":{"type":"string"}},"required":["prompt"]}`)
}

func (t *captureDispatchTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	t.calls++
	t.lastArgs = append(json.RawMessage(nil), args...)
	return t.output, t.err
}

func (t *captureDispatchTool) ReadOnly() bool { return t.readOnly }

type dispatchArgs struct {
	Prompt      string `json:"prompt"`
	Description string `json:"description"`
}

func decodeDispatchArgs(t *testing.T, raw json.RawMessage) dispatchArgs {
	t.Helper()
	var args dispatchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatalf("decode dispatch args: %v\n%s", err, string(raw))
	}
	return args
}
