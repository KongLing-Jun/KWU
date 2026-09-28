package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

const taskSnapshotInterval = 250 * time.Millisecond

// ExpertTeamRuntime is supplied by the parent-session host. It owns the trusted
// member-to-subagent binding; the model only ever selects a configured name.
type ExpertTeamRuntime interface {
	AcquireExpertTeamMember(memberName string) (subagentRef string, err error)
	CommitExpertTeamMember(memberName, subagentRef string) error
	ReleaseExpertTeamMember(memberName string)
	LookupExpertTeamMember(memberName string) (subagentRef string, err error)
}

type expertTeamRuntimeContextKey struct{}

func WithExpertTeamRuntime(ctx context.Context, runtime ExpertTeamRuntime) context.Context {
	if runtime == nil {
		return ctx
	}
	return context.WithValue(ctx, expertTeamRuntimeContextKey{}, runtime)
}

func expertTeamRuntimeFromContext(ctx context.Context) (ExpertTeamRuntime, bool) {
	runtime, ok := ctx.Value(expertTeamRuntimeContextKey{}).(ExpertTeamRuntime)
	return runtime, ok && runtime != nil
}

func (t expertTeamBoundedTaskTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	args, memberName, err := t.prepareTaskArgs(ctx, args)
	if err != nil {
		return "", err
	}
	// The expert runtime's Acquire call reserves member state, so shared task
	// admission must reject scheduling outcomes before that side effect.
	if _, ok := t.inner.(*TaskTool); ok {
		params, decodeErr := decodeTaskParams(args)
		if decodeErr != nil {
			return "", decodeErr
		}
		if result, admitted := admitPersistentTaskDispatch(ctx, params); !admitted {
			return result, nil
		}
	}
	runtime, ok := expertTeamRuntimeFromContext(ctx)
	if !ok {
		return t.inner.Execute(ctx, args)
	}
	return t.executeMemberTask(ctx, runtime, memberName, args)
}

func (t expertTeamBoundedTaskTool) prepareTaskArgs(ctx context.Context, args json.RawMessage) (json.RawMessage, string, error) {
	memberName := ""
	if len(t.allowedMembers) > 0 {
		patched, err := withExpertTeamMemberIdentity(ctx, args, t.allowedMembers)
		if err != nil {
			return nil, "", err
		}
		args = patched
		var identity struct {
			MemberName string `json:"member_name"`
		}
		_ = json.Unmarshal(args, &identity)
		memberName = identity.MemberName
	}
	if patched, ok := withExpertTeamDefaultTaskMaxSteps(args); expertTeamTaskMaxSteps(args) <= 0 && ok {
		args = patched
	}
	if patched, ok := withExpertTeamParentCancellation(args); taskRunsInBackground(args) && ok {
		args = patched
	}
	return args, memberName, nil
}

func (t expertTeamBoundedTaskTool) executeMemberTask(ctx context.Context, runtime ExpertTeamRuntime, memberName string, args json.RawMessage) (string, error) {
	continueFrom, err := runtime.AcquireExpertTeamMember(memberName)
	if err != nil {
		return "", err
	}
	defer runtime.ReleaseExpertTeamMember(memberName)
	if continueFrom != "" {
		args, err = withTrustedTaskContinuation(args, continueFrom)
		if err != nil {
			return "", err
		}
	}
	result, err := t.inner.Execute(ctx, args)
	if err != nil {
		return result, err
	}
	ref := continueFrom
	if ref == "" {
		ref = taskStartSubagentRef(result)
	}
	if ref == "" {
		return "", fmt.Errorf("expert-team task started without a subagent reference")
	}
	if err := runtime.CommitExpertTeamMember(memberName, ref); err != nil {
		return "", fmt.Errorf("save expert-team member binding: %w", err)
	}
	return result, nil
}

func withTrustedTaskContinuation(args json.RawMessage, ref string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil {
		return nil, fmt.Errorf("invalid expert-team task args: %w", err)
	}
	obj["continue_from"], _ = json.Marshal(strings.TrimSpace(ref))
	return json.Marshal(obj)
}

func taskStartSubagentRef(result string) string {
	const prefix = "Subagent reference:"
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

type expertTeamTaskControlTool struct {
	inner          tool.Tool
	allowedMembers []string
}

type expertTeamControlParams struct {
	MemberName  string   `json:"member_name"`
	MemberNames []string `json:"member_names"`
	Ref         string   `json:"subagent_ref"`
	Refs        []string `json:"subagent_refs"`
}

func (t expertTeamTaskControlTool) Name() string { return t.inner.Name() }
func (t expertTeamTaskControlTool) Description() string {
	return t.inner.Description() + " In expert-team mode, select configured members by member_name; subagent references are resolved by the host."
}
func (t expertTeamTaskControlTool) ReadOnly() bool { return t.inner.ReadOnly() }
func (t expertTeamTaskControlTool) Schema() json.RawMessage {
	member := map[string]any{"type": "string", "enum": t.allowedMembers}
	if t.Name() == "stop_task" {
		return mustJSON(map[string]any{
			"type": "object", "properties": map[string]any{"member_name": member}, "required": []string{"member_name"},
		})
	}
	return mustJSON(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"member_name":  member,
			"member_names": map[string]any{"type": "array", "items": member},
		},
	})
}
func (t expertTeamTaskControlTool) toolTimeoutBudget(base time.Duration, args json.RawMessage) time.Duration {
	if budgeter, ok := t.inner.(interface {
		toolTimeoutBudget(time.Duration, json.RawMessage) time.Duration
	}); ok {
		return budgeter.toolTimeoutBudget(base, args)
	}
	return base
}
func (t expertTeamTaskControlTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	runtime, ok := expertTeamRuntimeFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("expert-team member bindings are unavailable")
	}
	_, names, err := t.controlParams(args)
	if err != nil {
		return "", err
	}
	refs, canonicalNames, err := t.memberRefs(runtime, names)
	if err != nil {
		return "", err
	}
	if t.Name() == "stop_task" {
		result, err := t.inner.Execute(ctx, mustJSON(map[string]any{"subagent_ref": refs[0]}))
		return bindTaskReceiptMembers(result, canonicalNames, err)
	}
	result, err := t.inner.Execute(ctx, mustJSON(map[string]any{"subagent_refs": refs}))
	return bindTaskReceiptMembers(result, canonicalNames, err)
}

func (t expertTeamTaskControlTool) controlParams(args json.RawMessage) (expertTeamControlParams, []string, error) {
	var params expertTeamControlParams
	if err := json.Unmarshal(args, &params); err != nil {
		return params, nil, fmt.Errorf("invalid expert-team %s args: %w", t.Name(), err)
	}
	if strings.TrimSpace(params.Ref) != "" || len(params.Refs) > 0 {
		return params, nil, fmt.Errorf("subagent references are unavailable in expert-team mode; use member_name")
	}
	names := append([]string(nil), params.MemberNames...)
	if strings.TrimSpace(params.MemberName) != "" {
		names = append([]string{params.MemberName}, names...)
	}
	if len(names) == 0 {
		return params, nil, fmt.Errorf("member_name is required")
	}
	if t.Name() == "stop_task" && len(names) != 1 {
		return params, nil, fmt.Errorf("stop_task requires exactly one member_name")
	}
	return params, names, nil
}

func (t expertTeamTaskControlTool) memberRefs(runtime ExpertTeamRuntime, names []string) ([]string, []string, error) {
	refs := make([]string, 0, len(names))
	canonicalNames := make([]string, 0, len(names))
	for _, name := range names {
		canonical, err := configuredExpertTeamMember(mustJSON(name), t.allowedMembers)
		if err != nil {
			return nil, nil, err
		}
		ref, err := runtime.LookupExpertTeamMember(canonical)
		if err != nil {
			return nil, nil, err
		}
		refs = append(refs, ref)
		canonicalNames = append(canonicalNames, canonical)
	}
	return refs, canonicalNames, nil
}

func mustJSON(value any) json.RawMessage {
	out, _ := json.Marshal(value)
	return out
}

func (s *SubagentStore) MarkRunning(run *SubagentRun) error {
	if s == nil || run == nil || run.Ref == "" || s.parentDestroyed(run) {
		return nil
	}
	var sessionErr error
	if run.Session != nil {
		sessionErr = run.Session.Save(s.sessionPath(run.Ref))
	}
	meta := run.Meta
	meta.Status = SubagentRunning
	meta.TurnArtifacts = nil
	meta.UpdatedAt = time.Now().UTC()
	if !meta.UpdatedAt.After(run.Meta.UpdatedAt) {
		meta.UpdatedAt = run.Meta.UpdatedAt.Add(time.Nanosecond)
	}
	run.Meta = meta
	return errors.Join(sessionErr, s.saveMeta(meta))
}

// SaveRunning incrementally persists a running child session. It is safe to
// call from a snapshot goroutine while the agent appends messages.
func (s *SubagentStore) SaveRunning(run *SubagentRun) error {
	if s == nil || run == nil || run.Ref == "" || run.Session == nil || s.parentDestroyed(run) {
		return nil
	}
	_, _, err := run.Session.SaveIfChanged(s.sessionPath(run.Ref))
	return err
}

func (s *SubagentStore) SaveCompleted(run *SubagentRun) error {
	if s == nil || run == nil || run.Ref == "" || s.parentDestroyed(run) {
		return nil
	}
	if err := run.Session.Save(s.sessionPath(run.Ref)); err != nil {
		return err
	}
	meta := run.Meta
	meta.Status = SubagentCompleted
	meta.UpdatedAt = time.Now().UTC()
	run.Meta = meta
	return s.saveMeta(meta)
}

func (s *SubagentStore) SaveFailed(run *SubagentRun) error {
	return s.saveTerminal(run, SubagentFailed)
}

func (s *SubagentStore) SaveInterrupted(run *SubagentRun) error {
	return s.saveTerminal(run, SubagentInterrupted)
}

func (s *SubagentStore) saveTerminal(run *SubagentRun, status SubagentStatus) error {
	if s == nil || run == nil || run.Ref == "" || s.parentDestroyed(run) {
		return nil
	}
	var sessionErr error
	if run.Session != nil {
		sessionErr = run.Session.Save(s.sessionPath(run.Ref))
	}
	meta := run.Meta
	meta.Status = status
	meta.TurnArtifacts = nil
	meta.UpdatedAt = time.Now().UTC()
	run.Meta = meta
	return errors.Join(sessionErr, s.saveMeta(meta))
}

type WaitTaskTool struct{ task *TaskTool }

func NewWaitTaskTool(task *TaskTool) *WaitTaskTool { return &WaitTaskTool{task: task} }

func (*WaitTaskTool) Name() string { return "wait_task" }

func (*WaitTaskTool) Description() string {
	return "Wait for one or more persistent sub-agent turns by subagent reference, or read their latest terminal result."
}

func (a *Agent) recordToolReceipt(call provider.ToolCall, called tool.Tool, result string, err error, artifactMutation bool, artifactGroups ...[]provider.ToolArtifact) {
	if a.evidence == nil {
		return
	}
	receipt := evidence.ReceiptFromToolCall(call.Name, json.RawMessage(call.Arguments), err == nil, called.ReadOnly())
	receipt.ArtifactMutation = artifactMutation
	if err == nil {
		populateSuccessfulArtifactReceipt(&receipt, call.Name, result, artifactGroups)
	}
	markNonDispatchedTaskReceipt(&receipt, call.Name, result, err)
	if err == nil && call.Name == "wait_task" {
		paths, completed := completedTaskReceiptEvidence(result)
		receipt.Read = !completed
		receipt.Write = completed && len(paths) > 0
		receipt.Paths = paths
		receipt.ArtifactPaths = append(receipt.ArtifactPaths, paths...)
		receipt.ArtifactMutation = taskReceiptTerminal(result)
	}
	a.evidence.Record(receipt)
	if err != nil {
		return
	}
	if call.Name == "complete_step" {
		a.advanceCanonicalTodo(receipt.Step)
	} else if call.Name == "todo_write" {
		a.setTodoState(receipt.Todos)
	}
}

func populateSuccessfulArtifactReceipt(receipt *evidence.Receipt, toolName, result string, artifactGroups [][]provider.ToolArtifact) {
	if toolName == "deliver_files" {
		if paths, valid := artifactDeliveryManifestPaths(result); valid {
			receipt.ArtifactDecision = true
			receipt.Paths = paths
		}
	}
	for _, artifacts := range artifactGroups {
		receipt.ArtifactPaths = append(receipt.ArtifactPaths, artifactDeliveryToolArtifactPaths(artifacts)...)
	}
}

func taskReceiptTerminal(output string) bool {
	receipts, _, err := decodeTaskReceipts(output)
	if err != nil || len(receipts) == 0 {
		return false
	}
	for _, receipt := range receipts {
		switch receipt.TurnStatus {
		case SubagentCompleted, SubagentFailed, SubagentInterrupted:
		default:
			return false
		}
	}
	return true
}

func markNonDispatchedTaskReceipt(receipt *evidence.Receipt, toolName, result string, err error) {
	if receipt == nil || err != nil || toolName != "task" || !isNonDispatchedTaskResult(result) {
		return
	}
	receipt.Read = true
	receipt.Write = false
}

func (*WaitTaskTool) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "subagent_ref":{"type":"string","description":"One subagent reference returned by task."},
  "subagent_refs":{"type":"array","items":{"type":"string"},"description":"One or more subagent references returned by task."}
}
}`)
}

func (*WaitTaskTool) ReadOnly() bool { return true }

func (*WaitTaskTool) toolTimeoutBudget(time.Duration, json.RawMessage) time.Duration {
	return tool.NoTimeoutBudget
}

func (w *WaitTaskTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if w == nil || w.task == nil || w.task.transcripts == nil {
		return "", fmt.Errorf("subagent transcript store is required")
	}
	var p struct {
		Ref  string   `json:"subagent_ref"`
		Refs []string `json:"subagent_refs"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	refs, err := normalizeTaskRefs(p.Ref, p.Refs)
	if err != nil {
		return "", err
	}
	parentSession := taskControlParentSession(ctx)
	jm, _ := jobs.FromContext(ctx)
	receipts := make([]taskTurnReceipt, 0, len(refs))
	for _, ref := range refs {
		receipt, err := w.task.waitTaskRef(ctx, jm, parentSession, ref, 0)
		if err != nil {
			return "", err
		}
		receipts = append(receipts, receipt)
	}
	return marshalTaskReceipts(receipts)
}

type StopTaskTool struct{ task *TaskTool }

func NewStopTaskTool(task *TaskTool) *StopTaskTool { return &StopTaskTool{task: task} }

func (*StopTaskTool) Name() string { return "stop_task" }

func (*StopTaskTool) Description() string {
	return "Interrupt the running turn for a persistent sub-agent reference while preserving its session history for a later continuation."
}

func (*StopTaskTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"subagent_ref":{"type":"string","description":"The subagent reference returned by task."}},"required":["subagent_ref"]}`)
}

func (*StopTaskTool) ReadOnly() bool { return false }

func (s *StopTaskTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if s == nil || s.task == nil || s.task.transcripts == nil {
		return "", fmt.Errorf("subagent transcript store is required")
	}
	var p struct {
		Ref string `json:"subagent_ref"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	ref := strings.TrimSpace(p.Ref)
	if ref == "" {
		return "", fmt.Errorf("subagent_ref is required")
	}
	parentSession := taskControlParentSession(ctx)
	meta, err := s.task.ownedTaskMeta(ref, parentSession)
	if err != nil {
		return "", err
	}
	if meta.Status != SubagentRunning {
		jm, _ := jobs.FromContext(ctx)
		receipt, receiptErr := s.task.waitTaskRef(ctx, jm, parentSession, ref, 0)
		if receiptErr != nil {
			return "", receiptErr
		}
		return marshalTaskReceipts([]taskTurnReceipt{receipt})
	}
	return s.stopRunningTask(ctx, parentSession, ref)
}

func (s *StopTaskTool) stopRunningTask(ctx context.Context, parentSession, ref string) (string, error) {
	jm, ok := jobs.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("task control is not available in this context")
	}
	if !jm.KillControlledForSession(parentSession, "task", ref) {
		return "", fmt.Errorf("subagent reference %q is running but its task job is unavailable", ref)
	}
	result, _ := jm.WaitControlledForSession(ctx, parentSession, "task", ref, 5)
	meta, err := s.task.ownedTaskMeta(ref, parentSession)
	if err != nil {
		return "", err
	}
	if meta.Status == SubagentRunning {
		return "", fmt.Errorf("subagent reference %q cancellation was requested but the turn is still stopping", ref)
	}
	receipt := s.task.receiptFromMeta(meta, strings.TrimSpace(result.Output), "")
	receipt.TaskRunID = result.ID
	receipt.DependsOn = append([]string(nil), result.DependsOn...)
	return marshalTaskReceipts([]taskTurnReceipt{receipt})
}

type taskTurnReceipt struct {
	SubagentRef string                      `json:"subagentRef"`
	TaskRunID   string                      `json:"taskRunId,omitempty"`
	DependsOn   []string                    `json:"dependsOn,omitempty"`
	MemberName  string                      `json:"memberName,omitempty"`
	TurnStatus  SubagentStatus              `json:"turnStatus"`
	FinalAnswer string                      `json:"finalAnswer,omitempty"`
	Artifacts   []SubagentWorkspaceArtifact `json:"artifacts,omitempty"`
	Validations []taskHostValidation        `json:"validations,omitempty"`
	Error       string                      `json:"error,omitempty"`
}

type taskHostValidation struct {
	Kind    string `json:"kind"`
	Success bool   `json:"success"`
	Path    string `json:"path,omitempty"`
	Status  string `json:"status,omitempty"`
}

func (t *TaskTool) waitTaskRef(ctx context.Context, jm *jobs.Manager, parentSession, ref string, timeoutSec int) (taskTurnReceipt, error) {
	meta, err := t.ownedTaskMeta(ref, parentSession)
	if err != nil {
		return taskTurnReceipt{}, err
	}
	if jm != nil {
		if result, ok := jm.WaitControlledForSession(ctx, parentSession, "task", ref, timeoutSec); ok {
			if err := ctx.Err(); err != nil {
				return taskTurnReceipt{}, err
			}
			meta, err = t.ownedTaskMeta(ref, parentSession)
			if err != nil {
				return taskTurnReceipt{}, err
			}
			return t.receiptFromJobResult(meta, result), nil
		}
	}
	return t.receiptFromMeta(meta, "", ""), nil
}

func (t *TaskTool) receiptFromJobResult(meta SubagentMeta, result jobs.Result) taskTurnReceipt {
	output := strings.TrimSpace(result.Output)
	receipt := t.receiptFromMeta(meta, output, "")
	receipt.TaskRunID = result.ID
	receipt.DependsOn = append([]string(nil), result.DependsOn...)
	if receipt.TurnStatus == SubagentFailed && receipt.Error == "" {
		receipt.Error = output
		if receipt.Error == "" && result.Status == jobs.Failed {
			receipt.Error = "subagent turn failed"
		}
	}
	return receipt
}

func (t *TaskTool) receiptFromMeta(meta SubagentMeta, output, errText string) taskTurnReceipt {
	receipt := taskTurnReceipt{SubagentRef: meta.Ref, TurnStatus: meta.Status, Error: strings.TrimSpace(errText)}
	receipt.Validations = []taskHostValidation{{Kind: "turn_status", Success: meta.Status == SubagentCompleted, Status: string(meta.Status)}}
	if meta.Status == SubagentCompleted {
		receipt.FinalAnswer, _ = t.latestSubagentAnswer(meta.Ref)
		if receipt.FinalAnswer == "" {
			receipt.FinalAnswer = strings.TrimSpace(output)
		}
		receipt.Artifacts = append([]SubagentWorkspaceArtifact(nil), meta.TurnArtifacts...)
		for _, artifact := range receipt.Artifacts {
			receipt.Validations = append(receipt.Validations, taskHostValidation{
				Kind: "artifact_observed", Success: true, Path: artifact.Path,
			})
		}
	}
	return receipt
}

func (t *TaskTool) ownedTaskMeta(ref, parentSession string) (SubagentMeta, error) {
	meta, err := t.transcripts.LoadMeta(strings.TrimSpace(ref))
	if err != nil {
		return SubagentMeta{}, err
	}
	if strings.TrimSpace(meta.ParentSession) != strings.TrimSpace(parentSession) {
		return SubagentMeta{}, fmt.Errorf("subagent reference %q belongs to parent session %q, current parent session is %q", meta.Ref, meta.ParentSession, parentSession)
	}
	if meta.Kind != "task" {
		return SubagentMeta{}, fmt.Errorf("subagent reference %q has kind %q, want task", meta.Ref, meta.Kind)
	}
	return meta, nil
}

func (t *TaskTool) latestSubagentAnswer(ref string) (string, error) {
	sess, err := LoadSession(t.transcripts.sessionPath(ref))
	if err != nil {
		return "", err
	}
	msgs := sess.Snapshot()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == provider.RoleAssistant && strings.TrimSpace(msgs[i].Content) != "" {
			return strings.TrimSpace(msgs[i].Content), nil
		}
	}
	return "", nil
}

func normalizeTaskRefs(single string, many []string) ([]string, error) {
	refs := append([]string(nil), many...)
	if strings.TrimSpace(single) != "" {
		refs = append([]string{single}, refs...)
	}
	seen := map[string]bool{}
	out := refs[:0]
	for _, raw := range refs {
		ref := strings.TrimSpace(raw)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("subagent_ref or subagent_refs is required")
	}
	return out, nil
}

func marshalTaskReceipts(receipts []taskTurnReceipt) (string, error) {
	var value any = receipts
	if len(receipts) == 1 {
		value = receipts[0]
	}
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeTaskReceipts(output string) ([]taskTurnReceipt, bool, error) {
	data := []byte(strings.TrimSpace(output))
	if len(data) == 0 {
		return nil, false, fmt.Errorf("empty task receipt")
	}
	if data[0] == '[' {
		var receipts []taskTurnReceipt
		if err := json.Unmarshal(data, &receipts); err != nil {
			return nil, false, err
		}
		return receipts, false, nil
	}
	var receipt taskTurnReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, true, err
	}
	return []taskTurnReceipt{receipt}, true, nil
}

func bindTaskReceiptMembers(output string, names []string, executeErr error) (string, error) {
	if executeErr != nil {
		return output, executeErr
	}
	receipts, single, err := decodeTaskReceipts(output)
	if err != nil {
		return "", fmt.Errorf("decode host task receipt: %w", err)
	}
	if len(receipts) != len(names) {
		return "", fmt.Errorf("host task receipt count %d does not match member count %d", len(receipts), len(names))
	}
	for i := range receipts {
		receipts[i].MemberName = names[i]
	}
	if single {
		return marshalTaskReceipts(receipts[:1])
	}
	return marshalTaskReceipts(receipts)
}

func completedTaskReceiptEvidence(output string) ([]string, bool) {
	receipts, _, err := decodeTaskReceipts(output)
	if err != nil || len(receipts) == 0 {
		return nil, false
	}
	var paths []string
	for _, receipt := range receipts {
		if receipt.TurnStatus != SubagentCompleted {
			return nil, false
		}
		paths = append(paths, validatedTaskArtifactPaths(receipt)...)
	}
	return paths, true
}

func validatedTaskArtifactPaths(receipt taskTurnReceipt) []string {
	validated := make(map[string]bool, len(receipt.Validations))
	for _, validation := range receipt.Validations {
		path := strings.TrimSpace(validation.Path)
		if validation.Kind == "artifact_observed" && validation.Success && path != "" {
			validated[path] = true
		}
	}
	var paths []string
	for _, artifact := range receipt.Artifacts {
		if validated[strings.TrimSpace(artifact.Path)] {
			paths = append(paths, artifact.Path)
		}
	}
	return paths
}

func taskControlParentSession(ctx context.Context) string {
	if parent := strings.TrimSpace(ParentSession(ctx)); parent != "" {
		return parent
	}
	return jobs.SessionFromContext(ctx)
}

func emitSubagentStatus(sink event.Sink, run *SubagentRun) {
	if sink == nil || run == nil || strings.TrimSpace(run.Ref) == "" || run.Meta.Status == "" {
		return
	}
	sink.Emit(event.Event{
		Kind:           event.SubagentStatusChanged,
		SubagentRef:    run.Ref,
		SubagentStatus: string(run.Meta.Status),
	})
}

func (t *TaskTool) markSubagentRunning(run *SubagentRun, sink event.Sink) error {
	if t.transcripts == nil || run == nil || run.Ref == "" {
		return nil
	}
	status, updatedAt := run.Meta.Status, run.Meta.UpdatedAt
	if err := t.transcripts.MarkRunning(run); err != nil {
		return err
	}
	if run.Meta.Status == status && run.Meta.UpdatedAt.Equal(updatedAt) {
		return nil
	}
	emitSubagentStatus(sink, run)
	return nil
}

func (t *TaskTool) saveSubagentCompleted(run *SubagentRun, sink event.Sink) error {
	if t.transcripts == nil || run == nil || run.Ref == "" {
		return nil
	}
	status, updatedAt := run.Meta.Status, run.Meta.UpdatedAt
	if err := t.transcripts.SaveCompleted(run); err != nil {
		return err
	}
	if run.Meta.Status == status && run.Meta.UpdatedAt.Equal(updatedAt) {
		return nil
	}
	emitSubagentStatus(sink, run)
	return nil
}

func (t *TaskTool) saveSubagentFailed(run *SubagentRun, sink event.Sink) error {
	if t.transcripts == nil || run == nil || run.Ref == "" {
		return nil
	}
	status, updatedAt := run.Meta.Status, run.Meta.UpdatedAt
	if err := t.transcripts.SaveFailed(run); err != nil {
		return err
	}
	if run.Meta.Status == status && run.Meta.UpdatedAt.Equal(updatedAt) {
		return nil
	}
	emitSubagentStatus(sink, run)
	return nil
}

func (t *TaskTool) saveSubagentInterrupted(run *SubagentRun, sink event.Sink) error {
	if t.transcripts == nil || run == nil || run.Ref == "" {
		return nil
	}
	status, updatedAt := run.Meta.Status, run.Meta.UpdatedAt
	if err := t.transcripts.SaveInterrupted(run); err != nil {
		return err
	}
	if run.Meta.Status == status && run.Meta.UpdatedAt.Equal(updatedAt) {
		return nil
	}
	emitSubagentStatus(sink, run)
	return nil
}

func (t *TaskTool) runPersistedTaskTurn(ctx context.Context, prompt string, registry *tool.Registry, sink event.Sink, maxSteps int, runtime subSessionRuntime, run *SubagentRun, a2a *expertTeamA2ARequest, taskID string) (string, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopSnapshots := make(chan struct{})
	snapshotDone := make(chan error, 1)
	go t.snapshotRunningTask(cancel, run, stopSnapshots, snapshotDone)
	snapshotsStopped := false
	defer func() {
		if !snapshotsStopped {
			close(stopSnapshots)
			<-snapshotDone
		}
	}()

	answer, artifacts, runErr := t.runSubSessionWithArtifactOutcome(runCtx, prompt, registry, sink, maxSteps, runtime, run.Session)
	close(stopSnapshots)
	snapshotErr := <-snapshotDone
	snapshotsStopped = true
	if snapshotErr != nil {
		return appendExpertTeamA2AFailure(answer, a2a, taskID, snapshotErr), errors.Join(snapshotErr, t.saveSubagentFailed(run, sink))
	}
	if ctx.Err() != nil {
		return appendExpertTeamA2AFailure(answer, a2a, taskID, ctx.Err()), errors.Join(ctx.Err(), t.saveSubagentInterrupted(run, sink))
	}
	if runErr != nil {
		return appendExpertTeamA2AFailure(answer, a2a, taskID, runErr), errors.Join(runErr, t.saveSubagentFailed(run, sink))
	}
	run.Meta.TurnArtifacts = append([]SubagentWorkspaceArtifact(nil), artifacts...)
	if err := t.saveSubagentCompleted(run, sink); err != nil {
		return appendExpertTeamA2AFailure(answer, a2a, taskID, err), errors.Join(err, t.saveSubagentFailed(run, sink))
	}
	return appendExpertTeamA2AResult(answer, artifacts, a2a, taskID), nil
}

func (t *TaskTool) snapshotRunningTask(cancel context.CancelFunc, run *SubagentRun, stop <-chan struct{}, done chan<- error) {
	ticker := time.NewTicker(taskSnapshotInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := t.transcripts.SaveRunning(run); err != nil {
				cancel()
				done <- err
				return
			}
		case <-stop:
			done <- t.transcripts.SaveRunning(run)
			return
		}
	}
}
