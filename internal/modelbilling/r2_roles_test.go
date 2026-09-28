package modelbilling

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tasktree"
	"kwu/internal/tool"
)

type r2AgentProvider struct {
	client *http.Client
	url    string
}

type r2PendingFatalTool struct {
	taskID     string
	registered chan<- *foregroundCallRegistration
}

func (r2PendingFatalTool) Name() string            { return "derived" }
func (r2PendingFatalTool) Description() string     { return "derived model call" }
func (r2PendingFatalTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (r2PendingFatalTool) ReadOnly() bool          { return false }
func (t r2PendingFatalTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	pending, err := registerForegroundCall(ctx, t.taskID)
	if err != nil {
		return "", err
	}
	t.registered <- pending
	return "", agent.ErrMainStreamDispatcherCloseTimeout
}

type r2ParentToolProvider struct{ calls int }

func (*r2ParentToolProvider) Name() string { return "r2-parent" }
func (p *r2ParentToolProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	p.calls++
	chunks := make(chan provider.Chunk, 2)
	if p.calls == 1 {
		chunks <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "derived-call", Name: "derived", Arguments: `{}`}}
	} else {
		chunks <- provider.Chunk{Type: provider.ChunkText, Text: "must not continue"}
	}
	chunks <- provider.Chunk{Type: provider.ChunkDone}
	close(chunks)
	return chunks, nil
}

func (r2AgentProvider) Name() string { return "r2-agent-byok" }

func (p r2AgentProvider) Stream(ctx context.Context, request provider.Request) (<-chan provider.Chunk, error) {
	response, err := provider.SendWithRetry(ctx, p.client, provider.SendOptions{Provider: p.Name()}, func(requestCtx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(requestCtx, http.MethodPost, p.url, nil)
	})
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	reply := "final answer"
	if len(request.Messages) > 0 && strings.Contains(request.Messages[0].Content, "conversation summarizer") {
		reply = "summary"
	}
	ch := make(chan provider.Chunk, 3)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: reply}
	ch <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
		PromptTokens: 90, CompletionTokens: 2, TotalTokens: 92,
		Presence: provider.UsagePresence{Input: true, Output: true, ProviderTotal: true},
	}}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func TestB2CR2EveryDesktopCallRolePersistsARealCallRow(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks := NewSQLiteTaskStore(db)
	calls := NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_r2_roles")
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	taskCtx := storeContext(t, task)
	ctx, err := WithTaskContext(context.Background(), taskCtx)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &MainExecutorDispatcher{
		Calls: calls,
		Tasks: tasks,
		Evidence: DefaultCallEvidenceResolver{
			Store: NewSQLiteCallEvidenceStore(db),
			IDs:   NewRandomIDGenerator(),
		},
		IDs: NewRandomIDGenerator(),
	}
	roles := []CallRole{
		CallRoleMain,
		CallRolePlanner,
		CallRoleExecutor,
		CallRoleTasktreePlanner,
		CallRoleSubagent,
		CallRoleCompaction,
		CallRoleClassifier,
		CallRoleGuardian,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	base := httpDispatcherProvider{client: server.Client(), url: server.URL}
	err = (TaskLifecycleRunner{Tasks: tasks}).RunTask(ctx, func(ctx context.Context) error {
		for _, role := range roles {
			bound, err := NewRoleBoundStreamDispatcher(dispatcher, CallDispatchSpec{Role: role, ModelID: task.InitialModelID})
			if err != nil {
				return err
			}
			chunks, terminal, err := bound.StreamWithTerminalResult(ctx, base, provider.Request{})
			if err != nil {
				return err
			}
			for range chunks {
			}
			if err := <-terminal; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		rows, _ := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
		t.Fatalf("run roles: %v; rows=%+v", err, rows)
	}
	rows, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(roles) {
		t.Fatalf("call rows = %d, want %d: %#v", len(rows), len(roles), rows)
	}
	want := make(map[CallRole]bool, len(roles))
	for _, role := range roles {
		want[role] = true
	}
	for _, row := range rows {
		if !want[row.Role] {
			t.Fatalf("unexpected role row: %+v", row)
		}
		delete(want, row.Role)
		if row.State != CallCompleted || row.ModelID != task.InitialModelID || row.PricingEvidenceID != task.PricingEvidenceID || row.BillingSource != task.BillingSource || !row.Scope.Equal(task.Scope) {
			t.Fatalf("persisted role row = %+v", row)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing roles = %#v", want)
	}
}

func TestB2CR2AutomaticCompactionInheritsCurrentTaskAndUsesCompactionRole(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks := NewSQLiteTaskStore(db)
	calls := NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_r2_auto_compact")
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	dispatcher := &MainExecutorDispatcher{
		Calls: calls, Tasks: tasks,
		Evidence: DefaultCallEvidenceResolver{Store: NewSQLiteCallEvidenceStore(db), IDs: NewRandomIDGenerator()},
		IDs:      NewRandomIDGenerator(),
	}
	main, err := NewRoleBoundStreamDispatcher(dispatcher, CallDispatchSpec{Role: CallRoleMain, ModelID: task.InitialModelID})
	if err != nil {
		t.Fatal(err)
	}
	compaction, err := NewRoleBoundStreamDispatcher(dispatcher, CallDispatchSpec{Role: CallRoleCompaction, ModelID: task.InitialModelID})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	base := r2AgentProvider{client: server.Client(), url: server.URL}
	session := &agent.Session{Messages: []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: strings.Repeat("old context ", 200)},
		{Role: provider.RoleAssistant, Content: "old answer"},
		{Role: provider.RoleUser, Content: "middle"},
		{Role: provider.RoleAssistant, Content: "middle answer"},
		{Role: provider.RoleUser, Content: "recent"},
		{Role: provider.RoleAssistant, Content: "recent answer"},
	}}
	agentRunner := agent.New(base, tool.NewRegistry(), session, agent.Options{
		ContextWindow: 100, RecentKeep: 2, CompactRatio: 0.8, SoftCompactRatio: 0.5, CompactForceRatio: 0.9,
		StreamDispatcher: main, DerivedStreamDispatcher: compaction,
	}, event.Discard)
	ctx, err := WithTaskContext(context.Background(), storeContext(t, task))
	if err != nil {
		t.Fatal(err)
	}
	if err := (TaskLifecycleRunner{Tasks: tasks}).RunTask(ctx, func(ctx context.Context) error {
		return agentRunner.Run(ctx, "new request")
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("automatic compact rows = %#v", rows)
	}
	roles := map[CallRole]bool{}
	for _, row := range rows {
		roles[row.Role] = true
		if row.TaskID != task.TaskID || row.State != CallCompleted {
			t.Fatalf("automatic compact row = %+v", row)
		}
	}
	if !roles[CallRoleMain] || !roles[CallRoleCompaction] {
		t.Fatalf("automatic compact roles = %#v", roles)
	}
}

func TestB2CR2TaskTreePropagatesDispatcherCloseTimeoutWithoutWaitingOnPendingBarrier(t *testing.T) {
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	store := &barrierTaskStore{updates: make(chan TaskStateUpdate, 4)}
	tree := tasktree.NewTaskTree("tree_close_timeout", "propagate terminal persistence timeout", time.Now())
	tree.RootIDs = []string{"node"}
	tree.Nodes["node"] = &tasktree.TaskNode{ID: "node", Title: "node", Status: tasktree.StatusPending}
	orch := tasktree.NewOrchestrator(tasktree.Options{
		Planner: tasktree.PlannerFunc(func(context.Context, string) (*tasktree.TaskTree, error) { return tree, nil }),
		Store:   tasktree.NewMemoryStateStore(),
		Dispatcher: tasktree.NodeDispatcherFunc(func(context.Context, *tasktree.TaskTree, string, tasktree.NodeContext) (tasktree.NodeResult, error) {
			return tasktree.NodeResult{}, agent.ErrMainStreamDispatcherCloseTimeout
		}),
	})
	registered := make(chan *foregroundCallRegistration)
	result := make(chan error, 1)
	go func() {
		result <- (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(ctx context.Context) error {
			pending, registerErr := registerForegroundCall(ctx, task.TaskID)
			if registerErr != nil {
				return registerErr
			}
			registered <- pending
			return orch.Run(ctx, "execute task tree")
		})
	}()
	pending := <-registered
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case runErr := <-result:
		if !errors.Is(runErr, agent.ErrMainStreamDispatcherCloseTimeout) || ErrorCodeOf(runErr) != ErrorUsagePersistFailed {
			t.Fatalf("task-tree close-timeout error = %v", runErr)
		}
	case <-timer.C:
		pending.Complete(nil)
		<-result
		t.Fatal("task-tree swallowed close-timeout and lifecycle waited on the pending barrier")
	}
	pending.Complete(nil)
	pending.Complete(nil)
	assertCloseTimeoutRecoveryUpdates(t, store.snapshot())
}

func TestB2CR2ParentAgentPropagatesNestedCloseTimeoutWithoutWaitingOnPendingBarrier(t *testing.T) {
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	store := &barrierTaskStore{updates: make(chan TaskStateUpdate, 4)}
	registered := make(chan *foregroundCallRegistration)
	registry := tool.NewRegistry()
	registry.Add(r2PendingFatalTool{taskID: task.TaskID, registered: registered})
	base := &r2ParentToolProvider{}
	runner := agent.New(base, registry, agent.NewSession("system"), agent.Options{}, event.Discard)
	result := make(chan error, 1)
	go func() {
		result <- (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(ctx context.Context) error {
			return runner.Run(ctx, "run nested derived call")
		})
	}()
	pending := <-registered
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case runErr := <-result:
		if !errors.Is(runErr, agent.ErrMainStreamDispatcherCloseTimeout) || ErrorCodeOf(runErr) != ErrorUsagePersistFailed {
			t.Fatalf("nested close-timeout error = %v", runErr)
		}
	case <-timer.C:
		pending.Complete(nil)
		<-result
		t.Fatal("parent Agent swallowed close-timeout and lifecycle waited on the pending barrier")
	}
	pending.Complete(nil)
	pending.Complete(nil)
	if base.calls != 1 {
		t.Fatalf("parent provider calls = %d, want no fallback after close-timeout", base.calls)
	}
	assertCloseTimeoutRecoveryUpdates(t, store.snapshot())
}
