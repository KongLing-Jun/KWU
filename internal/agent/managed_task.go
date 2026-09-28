package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"kwu/internal/event"
	"kwu/internal/jobs"
	"kwu/internal/tool"
)

type managedTask struct {
	parentTurnCtx      context.Context
	parentSession      string
	label              string
	a2aTaskID          string
	completeTurnWork   func(error)
	legacyCrossTurnRun bool
	nested             event.Sink
}

const maxPersistentTasksPerParentTurn = 4

func (t *TaskTool) startManagedTask(ctx context.Context, args json.RawMessage, p taskParams, subReg *tool.Registry, maxSteps int, runtime subSessionRuntime, run *SubagentRun, parentID string, parent event.Sink, legacyBackground bool) (string, bool, error) {
	legacyCrossTurnRun := run.Ref == "" && legacyBackground
	if run.Ref == "" && !legacyCrossTurnRun {
		return "", false, nil
	}
	manager, ok := jobs.FromContext(ctx)
	if !ok {
		return "", false, nil
	}
	managed, err := t.prepareManagedTask(ctx, p, run, parentID, parent, manager, legacyCrossTurnRun)
	if err != nil {
		run.Release()
		return "", true, err
	}
	turnCtx := managed.parentTurnCtx
	if managed.legacyCrossTurnRun {
		turnCtx = nil
	}
	runJob := t.managedTaskRunner(p, subReg, maxSteps, runtime, run, managed)
	job := startTaskJob(manager, turnCtx, managed.parentSession, run.Ref, managed.label, p.DependsOn, runJob)
	if managed.legacyCrossTurnRun {
		return startLegacyTaskResult(ctx, args, manager, job, managed.label, p.A2A, managed.a2aTaskID), true, nil
	}
	return formatTaskStart(managed.label, run, job, p.DependsOn, p.A2A, managed.a2aTaskID), true, nil
}

type taskDispatchResult struct {
	Status          string   `json:"status"`
	RunningTaskRuns []string `json:"running_task_runs,omitempty"`
	DependsOn       []string `json:"depends_on,omitempty"`
	Message         string   `json:"message"`
}

func marshalTaskDispatchResult(result taskDispatchResult) string {
	data, _ := json.Marshal(result)
	return string(data)
}

func isNonDispatchedTaskResult(output string) bool {
	var result taskDispatchResult
	if json.Unmarshal([]byte(strings.TrimSpace(output)), &result) != nil {
		return false
	}
	switch result.Status {
	case "dispatch_decision_required", "dependency_not_satisfied", "capacity_wait_required":
		return true
	default:
		return false
	}
}

// admitPersistentTaskDispatch performs every scheduling decision before child
// preparation. It creates no refs, jobs, transcript metadata, events, or expert
// member state.
func admitPersistentTaskDispatch(ctx context.Context, p taskParams) (string, bool) {
	manager, ok := jobs.FromContext(ctx)
	parentSession := jobs.SessionFromContext(ctx)
	if p.InvalidDependsOn {
		return marshalTaskDispatchResult(taskDispatchResult{Status: "dependency_not_satisfied", Message: "Every dependency must be a non-empty task_run_id."}), false
	}
	if len(p.DependsOn) > 0 && (!ok || parentSession == "") {
		return marshalTaskDispatchResult(taskDispatchResult{Status: "dependency_not_satisfied", DependsOn: p.DependsOn, Message: "Dependencies must identify successful persistent task runs in the current parent session."}), false
	}
	if result, admitted := admitTaskDependencies(manager, parentSession, p.DependsOn); !admitted {
		return result, false
	}
	if !ok || parentSession == "" {
		return "", true
	}
	return admitTaskConcurrency(manager, parentSession, p.Independent)
}

func admitTaskConcurrency(manager *jobs.Manager, parentSession string, independent bool) (string, bool) {
	runningIDs := runningPersistentTaskIDs(manager, parentSession)
	if len(runningIDs) > 0 && !independent {
		return marshalTaskDispatchResult(taskDispatchResult{Status: "dispatch_decision_required", RunningTaskRuns: runningIDs, Message: "Wait for running tasks or retry with independent=true only if this task needs none of their results."}), false
	}
	if len(runningIDs) >= maxPersistentTasksPerParentTurn {
		return marshalTaskDispatchResult(taskDispatchResult{Status: "capacity_wait_required", RunningTaskRuns: runningIDs, Message: "Wait for or stop a running task before retrying."}), false
	}
	return "", true
}

func admitTaskDependencies(manager *jobs.Manager, parentSession string, dependsOn []string) (string, bool) {
	var runningDependencies []string
	for _, id := range dependsOn {
		run, found := manager.TaskRunForSession(parentSession, id)
		if !found || run.Status == jobs.Failed || run.Status == jobs.Killed {
			return marshalTaskDispatchResult(taskDispatchResult{Status: "dependency_not_satisfied", DependsOn: dependsOn, Message: "A dependency is unavailable or did not complete successfully in this parent session."}), false
		}
		if run.Status == jobs.Running {
			runningDependencies = append(runningDependencies, run.ID)
			continue
		}
		if run.Status != jobs.Done {
			return marshalTaskDispatchResult(taskDispatchResult{Status: "dependency_not_satisfied", DependsOn: dependsOn, Message: "A dependency did not complete successfully."}), false
		}
	}
	if len(runningDependencies) > 0 {
		return marshalTaskDispatchResult(taskDispatchResult{Status: "dispatch_decision_required", RunningTaskRuns: runningDependencies, DependsOn: dependsOn, Message: "Wait for the running dependency, inspect its result, and retry with a newly composed prompt."}), false
	}
	return "", true
}

func runningPersistentTaskIDs(manager *jobs.Manager, parentSession string) []string {
	running := manager.RunningForSession(parentSession)
	runningIDs := make([]string, 0, len(running))
	for _, view := range running {
		if view.Kind == "task" {
			runningIDs = append(runningIDs, view.ID)
		}
	}
	return runningIDs
}

func (t *TaskTool) prepareManagedTask(ctx context.Context, p taskParams, run *SubagentRun, parentID string, parent event.Sink, manager *jobs.Manager, legacyCrossTurnRun bool) (managedTask, error) {
	managed := managedTask{
		parentTurnCtx:      toolCallParentContext(ctx),
		parentSession:      jobs.SessionFromContext(ctx),
		label:              strings.TrimSpace(p.Description),
		completeTurnWork:   func(error) {},
		legacyCrossTurnRun: legacyCrossTurnRun,
	}
	if managed.label == "" {
		managed.label = "task"
	}
	if err := t.registerManagedTurnWork(&managed, run); err != nil {
		return managed, err
	}
	taskCtx, _ := event.TaskTreeContextFromContext(ctx)
	managed.nested = subSinkForContext(parentID, parent, taskCtx, run.Ref)
	managed.a2aTaskID = managedA2ATaskID(p.A2A, run.Ref)
	if err := t.markManagedTaskRunning(run, managed.nested, managed.completeTurnWork); err != nil {
		return managed, err
	}
	return managed, nil
}

func (t *TaskTool) registerManagedTurnWork(managed *managedTask, run *SubagentRun) error {
	if run.Ref == "" {
		return nil
	}
	complete, err := RegisterTurnWork(managed.parentTurnCtx)
	if err != nil {
		return fmt.Errorf("register parent-turn sub-agent work: %w", err)
	}
	managed.completeTurnWork = complete
	return nil
}

func (t *TaskTool) markManagedTaskRunning(run *SubagentRun, sink event.Sink, completeTurnWork func(error)) error {
	if t.transcripts == nil || run == nil || run.Ref == "" {
		return nil
	}
	if err := t.markSubagentRunning(run, sink); err != nil {
		completeTurnWork(nil)
		return err
	}
	return nil
}

func managedA2ATaskID(a2a *expertTeamA2ARequest, ref string) string {
	if a2a == nil {
		return ""
	}
	if ref = strings.TrimSpace(ref); ref != "" {
		return ref
	}
	return newExpertTeamA2AID("task")
}

func (t *TaskTool) managedTaskRunner(p taskParams, subReg *tool.Registry, maxSteps int, runtime subSessionRuntime, run *SubagentRun, managed managedTask) func(context.Context, io.Writer) (string, error) {
	return func(jobCtx context.Context, _ io.Writer) (result string, err error) {
		// Managed/background children can overlap in one workspace. Whole-tree
		// before/after scans cannot attribute a sibling's write to the owning job;
		// exact nested receipts and verified manifests remain available.
		jobCtx = withoutOpaqueWorkspaceArtifactObservation(jobCtx)
		defer run.Release()
		defer func() { managed.completeTurnWork(nil) }()
		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("internal error: panic: %v\n%s", recovered, debug.Stack())
				result = appendExpertTeamA2AFailure("", p.A2A, managed.a2aTaskID, panicErr)
				err = errors.Join(panicErr, t.saveSubagentFailed(run, managed.nested))
			}
		}()
		return t.runPersistedTaskTurn(jobCtx, p.Prompt, subReg, managed.nested, maxSteps, runtime, run, p.A2A, managed.a2aTaskID)
	}
}
