package control

import (
	"context"
	"errors"
	"slices"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/tool"
)

func TestBindSessionToolsScopesProviderToolsToCurrentTask(t *testing.T) {
	base := tool.NewRegistry()
	base.Add(fakeControlTool{name: "base_tool"})
	executor := agent.New(nil, base, agent.NewSession("system"), agent.Options{}, event.Discard)
	var providerPath string
	c := &Controller{
		executor:    executor,
		mcp:         newMcpManager(nil, base, context.Background()),
		sessionPath: "sessions/project.jsonl",
		sessionToolProvider: func(_ context.Context, path string) ([]tool.Tool, error) {
			providerPath = path
			return []tool.Tool{fakeControlTool{name: "project_assets"}}, nil
		},
	}

	ctx, err := c.bindSessionTools(context.Background())
	if err != nil {
		t.Fatalf("bindSessionTools: %v", err)
	}
	if providerPath != c.sessionPath {
		t.Fatalf("provider path = %q, want %q", providerPath, c.sessionPath)
	}
	scoped := agent.TaskToolRegistry(ctx, base)
	if got := scoped.Names(); !slices.Contains(got, "base_tool") || !slices.Contains(got, "project_assets") {
		t.Fatalf("scoped registry = %v", got)
	}
	if _, ok := base.Get("project_assets"); ok {
		t.Fatal("session tool leaked into the controller's long-lived registry")
	}

	capabilityScoped := registryForMCPServers(scoped, nil)
	if _, ok := capabilityScoped.Get("project_assets"); !ok {
		t.Fatal("capability turn dropped the session-scoped project tool")
	}
}

func TestBindSessionToolsLeavesOrdinarySessionRegistryUnchanged(t *testing.T) {
	base := tool.NewRegistry()
	base.Add(fakeControlTool{name: "base_tool"})
	executor := agent.New(nil, base, agent.NewSession("system"), agent.Options{}, event.Discard)
	c := &Controller{
		executor: executor,
		mcp:      newMcpManager(nil, base, context.Background()),
		sessionToolProvider: func(context.Context, string) ([]tool.Tool, error) {
			return nil, nil
		},
	}

	original := context.Background()
	ctx, err := c.bindSessionTools(original)
	if err != nil {
		t.Fatalf("bindSessionTools: %v", err)
	}
	if got := agent.TaskToolRegistry(ctx, base); got != base {
		t.Fatalf("ordinary session received a registry override: %v", got.Names())
	}
}

func TestRunTaskLifecycleDoesNotResolveSessionToolsBeforeGuardAuthorizes(t *testing.T) {
	wantErr := errors.New("session denied")
	providerCalled := false
	c := &Controller{
		sessionSendGuard: func(context.Context, string) error { return wantErr },
		sessionToolProvider: func(context.Context, string) ([]tool.Tool, error) {
			providerCalled = true
			return []tool.Tool{fakeControlTool{name: "project_assets"}}, nil
		},
	}
	runCalled := false
	err := c.runTaskLifecycle(context.Background(), func(context.Context) error {
		runCalled = true
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runTaskLifecycle error = %v, want %v", err, wantErr)
	}
	if providerCalled || runCalled {
		t.Fatalf("denied task reached provider=%v run=%v", providerCalled, runCalled)
	}
}
