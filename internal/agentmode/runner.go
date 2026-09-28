package agentmode

import (
	"fmt"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/tasktree"
	"kwu/internal/tool"
)

// RunnerDeps are the already-assembled pieces needed to select a mode-specific
// runner. Normal is the existing Agent/Coordinator path; task_tree wraps those
// dependencies in its own orchestrator boundary.
type RunnerDeps struct {
	Normal          agent.Runner
	Executor        *agent.Agent
	Registry        *tool.Registry
	Sink            event.Sink
	TaskTreePlanner tasktree.Planner
	TaskTreeStore   tasktree.StateStore
}

// BuildRunner returns the agent.Runner for the resolved agent mode.
func BuildRunner(mode Mode, deps RunnerDeps) (agent.Runner, error) {
	switch mode {
	case "", ModeNormal:
		if deps.Normal != nil {
			return deps.Normal, nil
		}
		if deps.Executor != nil {
			return deps.Executor, nil
		}
		return nil, fmt.Errorf("normal agent mode requires a runner")
	case ModeTaskTree:
		return tasktree.NewOrchestrator(tasktree.Options{
			BaseRunner: deps.Normal,
			Planner:    deps.TaskTreePlanner,
			Executor:   deps.Executor,
			Registry:   deps.Registry,
			Sink:       deps.Sink,
			Store:      deps.TaskTreeStore,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported agent mode %q", mode)
	}
}
