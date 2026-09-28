package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kwu/internal/tool"
)

const (
	expertTeamTaskDefaultMaxSteps  = 12
	expertTeamLeadDefaultMaxSteps  = 24
	expertTeamMaxStepsKey          = "expert_team.max_steps"
	expertTeamA2AVersion           = "1.0"
	expertTeamA2ABinding           = "urn:reasonix:a2a:binding:in-process:v1"
	expertTeamA2ATopologyExtension = "urn:reasonix:a2a:extension:lead-member-only:v1"
)

var expertTeamOrchestrationToolNames = []string{
	"task",
	"wait_task",
	"stop_task",
	"todo_write",
	"complete_step",
	"deliver_files",
	"ask",
}

func expertTeamOrchestrationToolsForInput(parent *tool.Registry, input string) (*tool.Registry, bool) {
	allowedMembers, isTeam := expertTeamConfiguredMembers(input)
	if parent == nil || !isTeam {
		return nil, false
	}
	filtered := tool.NewRegistry()
	for _, name := range expertTeamOrchestrationToolNames {
		tl, ok := parent.Get(name)
		if !ok {
			continue
		}
		wrapped, include := expertTeamOrchestrationTool(tl, name, allowedMembers)
		if include {
			filtered.Add(wrapped)
		}
	}
	return filtered, true
}

func (a *Agent) turnExecutionConfig(ctx context.Context, input string) (*tool.Registry, int, string) {
	turnTools := a.taskToolRegistry(ctx)
	filtered, expertTeam := expertTeamOrchestrationToolsForInput(turnTools, input)
	if !expertTeam {
		return turnTools, a.maxSteps, a.maxStepsKey
	}
	if a.maxSteps > 0 {
		return filtered, a.maxSteps, a.maxStepsKey
	}
	return filtered, expertTeamLeadDefaultMaxSteps, expertTeamMaxStepsKey
}

func expertTeamOrchestrationTool(tl tool.Tool, name string, allowedMembers []string) (tool.Tool, bool) {
	if len(allowedMembers) == 0 && (name == "task" || name == "wait_task" || name == "stop_task") {
		return nil, false
	}
	switch name {
	case "task":
		return expertTeamBoundedTaskTool{inner: tl, allowedMembers: allowedMembers}, true
	case "wait_task", "stop_task":
		return expertTeamTaskControlTool{inner: tl, allowedMembers: allowedMembers}, true
	default:
		return tl, true
	}
}

type expertTeamBoundedTaskTool struct {
	inner          tool.Tool
	allowedMembers []string
}

func (t expertTeamBoundedTaskTool) Name() string { return t.inner.Name() }

func (t expertTeamBoundedTaskTool) Description() string {
	return t.inner.Description() + " In expert-team mode, member_name must identify a configured sub-expert; unconfigured identities are rejected. Omitting max_steps defaults to a finite child-expert budget so tool errors cannot trigger an unbounded retry loop."
}

func (t expertTeamBoundedTaskTool) Schema() json.RawMessage {
	return withExpertTeamTaskMemberSchema(t.inner.Schema(), t.allowedMembers)
}

func (t expertTeamBoundedTaskTool) ReadOnly() bool { return t.inner.ReadOnly() }

func (t expertTeamBoundedTaskTool) toolTimeoutBudget(base time.Duration, args json.RawMessage) time.Duration {
	if taskRunsInBackground(args) {
		return base
	}
	return delegatedTaskTimeoutBudget(base, expertTeamTaskMaxSteps(args))
}

type expertTeamBoundedParallelTasksTool struct {
	inner          tool.Tool
	allowedMembers []string
}

func (t expertTeamBoundedParallelTasksTool) Name() string { return t.inner.Name() }

func (t expertTeamBoundedParallelTasksTool) Description() string {
	return t.inner.Description() + " In expert-team mode, every item must identify a configured sub-expert with member_name; the entire call is rejected if any identity is unconfigured. Each child task without max_steps gets a finite budget so failed tool attempts do not keep the team waiting indefinitely."
}

func (t expertTeamBoundedParallelTasksTool) Schema() json.RawMessage {
	return withExpertTeamParallelMemberSchema(t.inner.Schema(), t.allowedMembers)
}

func (t expertTeamBoundedParallelTasksTool) ReadOnly() bool { return t.inner.ReadOnly() }

func (t expertTeamBoundedParallelTasksTool) toolTimeoutBudget(base time.Duration, args json.RawMessage) time.Duration {
	var params struct {
		Tasks []struct {
			MaxSteps int `json:"max_steps"`
		} `json:"tasks"`
	}
	if json.Unmarshal(args, &params) != nil {
		return base
	}
	steps := 0
	for _, task := range params.Tasks {
		candidate := task.MaxSteps
		if candidate <= 0 {
			candidate = expertTeamTaskDefaultMaxSteps
		}
		if candidate > steps {
			steps = candidate
		}
	}
	return delegatedTaskTimeoutBudget(base, steps)
}

func (t expertTeamBoundedParallelTasksTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if len(t.allowedMembers) > 0 {
		patched, err := withExpertTeamParallelMemberIdentities(ctx, args, t.allowedMembers)
		if err != nil {
			return "", err
		}
		args = patched
	}
	if patched, ok := withExpertTeamDefaultParallelTaskMaxSteps(args); ok {
		args = patched
	}
	return t.inner.Execute(ctx, args)
}

func withExpertTeamTaskMemberSchema(schema json.RawMessage, allowedMembers []string) json.RawMessage {
	if len(allowedMembers) == 0 {
		return schema
	}
	var obj map[string]any
	if err := json.Unmarshal(schema, &obj); err != nil {
		return schema
	}
	props, _ := obj["properties"].(map[string]any)
	if props == nil {
		return schema
	}
	delete(props, "continue_from")
	delete(props, "tools")
	props["member_name"] = map[string]any{
		"type":        "string",
		"enum":        allowedMembers,
		"description": "Configured expert-team member that must own this sub-agent conversation.",
	}
	obj["required"] = appendRequiredString(obj["required"], "member_name")
	out, err := json.Marshal(obj)
	if err != nil {
		return schema
	}
	return out
}

func withExpertTeamParallelMemberSchema(schema json.RawMessage, allowedMembers []string) json.RawMessage {
	if len(allowedMembers) == 0 {
		return schema
	}
	var obj map[string]any
	if err := json.Unmarshal(schema, &obj); err != nil {
		return schema
	}
	props, _ := obj["properties"].(map[string]any)
	tasks, _ := props["tasks"].(map[string]any)
	items, _ := tasks["items"].(map[string]any)
	itemProps, _ := items["properties"].(map[string]any)
	if itemProps == nil {
		return schema
	}
	itemProps["member_name"] = map[string]any{
		"type":        "string",
		"enum":        allowedMembers,
		"description": "Configured expert-team member that must own this sub-agent conversation.",
	}
	items["required"] = appendRequiredString(items["required"], "member_name")
	out, err := json.Marshal(obj)
	if err != nil {
		return schema
	}
	return out
}

func appendRequiredString(raw any, value string) []string {
	required := make([]string, 0)
	if entries, ok := raw.([]any); ok {
		for _, entry := range entries {
			if name, ok := entry.(string); ok && name != "" {
				required = append(required, name)
			}
		}
	}
	for _, name := range required {
		if name == value {
			return required
		}
	}
	return append(required, value)
}

func withExpertTeamMemberIdentity(ctx context.Context, args json.RawMessage, allowedMembers []string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil {
		return nil, fmt.Errorf("invalid expert-team task args: %w", err)
	}
	if _, exists := obj["continue_from"]; exists {
		return nil, fmt.Errorf("continue_from is unavailable in expert-team mode because prior transcripts are not bound to the configured team member")
	}
	// A configured member owns one persistent child session. Let the shared task
	// runtime apply its default complete subagent-safe registry on every turn;
	// accepting a per-call whitelist would change that session's persisted tool
	// identity and make a later continuation fail.
	delete(obj, "tools")
	canonical, err := configuredExpertTeamMember(obj["member_name"], allowedMembers)
	if err != nil {
		return nil, err
	}
	var prompt string
	if err := json.Unmarshal(obj["prompt"], &prompt); err != nil || strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("expert-team task prompt is required")
	}
	prompt = fmt.Sprintf("You are the configured expert-team member %q. Work only in that identity.\n\n%s", canonical, prompt)
	obj["member_name"], _ = json.Marshal(canonical)
	obj["description"], _ = json.Marshal(canonical)
	obj["prompt"], _ = json.Marshal(prompt)
	obj["a2a"], _ = json.Marshal(newExpertTeamA2ARequest(ctx, canonical, prompt, taskRunsInBackground(args)))
	patched, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encode expert-team task args: %w", err)
	}
	return patched, nil
}

func withExpertTeamParallelMemberIdentities(ctx context.Context, args json.RawMessage, allowedMembers []string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil {
		return nil, fmt.Errorf("invalid expert-team parallel_tasks args: %w", err)
	}
	var tasks []json.RawMessage
	if err := json.Unmarshal(obj["tasks"], &tasks); err != nil || len(tasks) == 0 {
		return nil, fmt.Errorf("expert-team parallel_tasks requires at least one task")
	}
	patchedTasks := make([]json.RawMessage, len(tasks))
	for i, taskArgs := range tasks {
		patched, err := withExpertTeamMemberIdentity(ctx, taskArgs, allowedMembers)
		if err != nil {
			return nil, fmt.Errorf("expert-team parallel task %d: %w", i+1, err)
		}
		patchedTasks[i] = patched
	}
	obj["tasks"], _ = json.Marshal(patchedTasks)
	patched, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encode expert-team parallel_tasks args: %w", err)
	}
	return patched, nil
}

func configuredExpertTeamMember(raw json.RawMessage, allowedMembers []string) (string, error) {
	var requested string
	if err := json.Unmarshal(raw, &requested); err != nil || strings.TrimSpace(requested) == "" {
		return "", fmt.Errorf("member_name is required and must name a configured expert-team member")
	}
	requested = strings.TrimSpace(requested)
	for _, member := range allowedMembers {
		if strings.EqualFold(requested, member) {
			return member, nil
		}
	}
	return "", fmt.Errorf("member_name %q is not a configured expert-team member; allowed members: %s", requested, strings.Join(allowedMembers, ", "))
}

// expertTeamA2ARequest is the JSON-RPC binding of an A2A SendMessage request.
// Reasonix uses an in-process custom binding, but keeps the canonical A2A
// Message/Part/Task/Artifact shapes so the boundary can later move to a remote
// transport without changing expert-team semantics.
type expertTeamA2ARequest struct {
	JSONRPC string                  `json:"jsonrpc"`
	ID      string                  `json:"id"`
	Method  string                  `json:"method"`
	Params  expertTeamA2ASendParams `json:"params"`
}

type expertTeamA2ASendParams struct {
	Message       expertTeamA2AMessage       `json:"message"`
	Configuration expertTeamA2AConfiguration `json:"configuration"`
	Metadata      expertTeamA2AMetadata      `json:"metadata"`
}

type expertTeamA2AMessage struct {
	MessageID  string              `json:"messageId"`
	ContextID  string              `json:"contextId,omitempty"`
	TaskID     string              `json:"taskId,omitempty"`
	Role       string              `json:"role"`
	Parts      []expertTeamA2APart `json:"parts"`
	Extensions []string            `json:"extensions,omitempty"`
}

type expertTeamA2APart struct {
	Text      string         `json:"text,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Filename  string         `json:"filename,omitempty"`
	MediaType string         `json:"mediaType,omitempty"`
}

type expertTeamA2AConfiguration struct {
	AcceptedOutputModes []string `json:"acceptedOutputModes"`
	ReturnImmediately   bool     `json:"returnImmediately,omitempty"`
}

type expertTeamA2AMetadata struct {
	ProtocolVersion string                `json:"protocolVersion"`
	ProtocolBinding string                `json:"protocolBinding"`
	Topology        expertTeamA2ATopology `json:"topology"`
}

type expertTeamA2ATopology struct {
	Mode          string `json:"mode"`
	SenderRole    string `json:"senderRole"`
	SenderName    string `json:"senderName"`
	RecipientRole string `json:"recipientRole"`
	RecipientName string `json:"recipientName"`
}

type expertTeamA2AResponse struct {
	JSONRPC string               `json:"jsonrpc"`
	ID      string               `json:"id"`
	Result  *expertTeamA2AResult `json:"result,omitempty"`
	Error   *expertTeamA2AError  `json:"error,omitempty"`
}

type expertTeamA2AResult struct {
	Task expertTeamA2ATask `json:"task"`
}

type expertTeamA2ATask struct {
	ID        string                  `json:"id"`
	ContextID string                  `json:"contextId"`
	Status    expertTeamA2ATaskStatus `json:"status"`
	Artifacts []expertTeamA2AArtifact `json:"artifacts,omitempty"`
	History   []expertTeamA2AMessage  `json:"history,omitempty"`
	Metadata  expertTeamA2AMetadata   `json:"metadata"`
}

type expertTeamA2ATaskStatus struct {
	State     string                `json:"state"`
	Timestamp time.Time             `json:"timestamp"`
	Message   *expertTeamA2AMessage `json:"message,omitempty"`
}

type expertTeamA2AArtifact struct {
	ArtifactID string              `json:"artifactId"`
	Name       string              `json:"name,omitempty"`
	Parts      []expertTeamA2APart `json:"parts"`
}

type expertTeamA2AError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newExpertTeamA2ARequest(ctx context.Context, memberName, prompt string, returnImmediately bool) expertTeamA2ARequest {
	contextID := ParentSession(ctx)
	if contextID == "" {
		contextID = newExpertTeamA2AID("context")
	}
	messageID := newExpertTeamA2AID("message")
	return expertTeamA2ARequest{
		JSONRPC: "2.0",
		ID:      messageID,
		Method:  "SendMessage",
		Params: expertTeamA2ASendParams{
			Message: expertTeamA2AMessage{
				MessageID:  messageID,
				ContextID:  contextID,
				Role:       "ROLE_USER",
				Parts:      []expertTeamA2APart{{Text: prompt, MediaType: "text/plain"}},
				Extensions: []string{expertTeamA2ATopologyExtension},
			},
			Configuration: expertTeamA2AConfiguration{
				AcceptedOutputModes: []string{"text/plain", "application/vnd.reasonix.workspace-file+json"},
				ReturnImmediately:   returnImmediately,
			},
			Metadata: expertTeamA2AMetadata{
				ProtocolVersion: expertTeamA2AVersion,
				ProtocolBinding: expertTeamA2ABinding,
				Topology: expertTeamA2ATopology{
					Mode:          "lead-member-only",
					SenderRole:    "lead",
					SenderName:    "lead-expert",
					RecipientRole: "member",
					RecipientName: memberName,
				},
			},
		},
	}
}

func validateExpertTeamA2ARequest(request *expertTeamA2ARequest, memberName string) (string, error) {
	if request == nil {
		return "", nil
	}
	if err := validateExpertTeamA2AEnvelope(*request, memberName); err != nil {
		return "", err
	}
	return expertTeamA2AMessageText(request.Params.Message)
}

func validateExpertTeamA2AEnvelope(request expertTeamA2ARequest, memberName string) error {
	if request.JSONRPC != "2.0" || request.Method != "SendMessage" || strings.TrimSpace(request.ID) == "" {
		return fmt.Errorf("invalid expert-team A2A JSON-RPC SendMessage envelope")
	}
	metadata := request.Params.Metadata
	if metadata.ProtocolVersion != expertTeamA2AVersion || metadata.ProtocolBinding != expertTeamA2ABinding {
		return fmt.Errorf("unsupported expert-team A2A version or binding")
	}
	return validateExpertTeamA2ATopology(metadata.Topology, memberName)
}

func expertTeamA2AMessageText(message expertTeamA2AMessage) (string, error) {
	if message.Role != "ROLE_USER" || strings.TrimSpace(message.MessageID) == "" || strings.TrimSpace(message.ContextID) == "" {
		return "", fmt.Errorf("expert-team A2A request requires a ROLE_USER message with messageId and contextId")
	}
	if message.TaskID != "" {
		return "", fmt.Errorf("expert-team A2A SendMessage cannot address an existing child task")
	}
	for _, part := range message.Parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			return text, nil
		}
	}
	return "", fmt.Errorf("expert-team A2A request requires a non-empty text Part")
}

func validateExpertTeamA2ATopology(topology expertTeamA2ATopology, memberName string) error {
	if topology.Mode != "lead-member-only" || topology.SenderRole != "lead" || topology.RecipientRole != "member" {
		return fmt.Errorf("expert-team A2A topology permits only lead-to-member communication; member-to-member communication is forbidden")
	}
	if strings.TrimSpace(topology.SenderName) == "" || !strings.EqualFold(strings.TrimSpace(topology.RecipientName), strings.TrimSpace(memberName)) {
		return fmt.Errorf("expert-team A2A recipient must match the configured member %q", memberName)
	}
	return nil
}

func appendExpertTeamA2AResult(answer string, files []subagentWorkspaceFile, request *expertTeamA2ARequest, taskID string) string {
	if request == nil {
		return appendSubagentArtifactHandoff(answer, files)
	}
	if strings.TrimSpace(taskID) == "" {
		taskID = newExpertTeamA2AID("task")
	}
	artifacts := []expertTeamA2AArtifact{{
		ArtifactID: newExpertTeamA2AID("artifact"),
		Name:       "final-answer",
		Parts:      []expertTeamA2APart{{Text: strings.TrimSpace(answer), MediaType: "text/plain"}},
	}}
	for _, file := range files {
		artifacts = append(artifacts, expertTeamA2AFileArtifact(file))
	}
	response := expertTeamA2AResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: &expertTeamA2AResult{Task: expertTeamA2ATask{
			ID:        taskID,
			ContextID: request.Params.Message.ContextID,
			Status:    expertTeamA2ATaskStatus{State: "TASK_STATE_COMPLETED", Timestamp: time.Now().UTC()},
			Artifacts: artifacts,
			History:   []expertTeamA2AMessage{request.Params.Message},
			Metadata:  request.Params.Metadata,
		}},
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return appendSubagentArtifactHandoff(answer, files)
	}
	return appendExpertTeamA2AResponse(answer, encoded)
}

// appendExpertTeamA2AFailure closes an A2A task that was previously reported as
// WORKING. Without a terminal response, the lead can only keep polling the old
// task even though the local child has already failed or been cancelled.
func appendExpertTeamA2AFailure(text string, request *expertTeamA2ARequest, taskID string, taskErr error) string {
	if request == nil {
		return text
	}
	if strings.TrimSpace(taskID) == "" {
		taskID = newExpertTeamA2AID("task")
	}
	state := "TASK_STATE_FAILED"
	if errors.Is(taskErr, context.Canceled) || errors.Is(taskErr, context.DeadlineExceeded) {
		state = "TASK_STATE_CANCELED"
	}
	messageText := strings.TrimSpace(taskErr.Error())
	status := expertTeamA2ATaskStatus{State: state, Timestamp: time.Now().UTC()}
	if messageText != "" {
		status.Message = &expertTeamA2AMessage{
			MessageID: newExpertTeamA2AID("message"),
			ContextID: request.Params.Message.ContextID,
			TaskID:    taskID,
			Role:      "ROLE_AGENT",
			Parts:     []expertTeamA2APart{{Text: messageText, MediaType: "text/plain"}},
		}
	}
	response := expertTeamA2AResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: &expertTeamA2AResult{Task: expertTeamA2ATask{
			ID:        taskID,
			ContextID: request.Params.Message.ContextID,
			Status:    status,
			History:   []expertTeamA2AMessage{request.Params.Message},
			Metadata:  request.Params.Metadata,
		}},
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return text
	}
	return appendExpertTeamA2AResponse(text, encoded)
}

func appendExpertTeamA2AOutcome(text string, files []subagentWorkspaceFile, request *expertTeamA2ARequest, taskID string, taskErr error) string {
	if taskErr != nil {
		return appendExpertTeamA2AFailure(text, request, taskID, taskErr)
	}
	return appendExpertTeamA2AResult(text, files, request, taskID)
}

func appendExpertTeamA2AFailureIfEmpty(text string, request *expertTeamA2ARequest, taskErr error) string {
	if strings.TrimSpace(text) != "" {
		return text
	}
	return appendExpertTeamA2AFailure("", request, newExpertTeamA2AID("task"), taskErr)
}

func appendExpertTeamA2AResponse(text string, encoded []byte) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "A2A response: " + string(encoded)
	}
	return text + "\n\nA2A response: " + string(encoded)
}

func appendExpertTeamA2AWorking(text string, request *expertTeamA2ARequest, taskID string) string {
	if request == nil {
		return text
	}
	if strings.TrimSpace(taskID) == "" {
		taskID = newExpertTeamA2AID("task")
	}
	response := expertTeamA2AResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: &expertTeamA2AResult{Task: expertTeamA2ATask{
			ID:        taskID,
			ContextID: request.Params.Message.ContextID,
			Status:    expertTeamA2ATaskStatus{State: "TASK_STATE_WORKING", Timestamp: time.Now().UTC()},
			History:   []expertTeamA2AMessage{request.Params.Message},
			Metadata:  request.Params.Metadata,
		}},
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return text
	}
	return appendExpertTeamA2AResponse(text, encoded)
}

func expertTeamA2AFileArtifact(file subagentWorkspaceFile) expertTeamA2AArtifact {
	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(file.Path)))
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return expertTeamA2AArtifact{
		ArtifactID: newExpertTeamA2AID("artifact"),
		Name:       filepath.Base(file.Path),
		Parts: []expertTeamA2APart{{
			Data:      map[string]any{"path": file.Path, "size": file.Size},
			Filename:  filepath.Base(file.Path),
			MediaType: mediaType,
		}},
	}
}

func newExpertTeamA2AID(prefix string) string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

func expertTeamTaskMaxSteps(args json.RawMessage) int {
	var p struct {
		MaxSteps int `json:"max_steps"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return 0
	}
	return p.MaxSteps
}

func taskRunsInBackground(args json.RawMessage) bool {
	var p struct {
		RunInBackground bool `json:"run_in_background"`
	}
	return json.Unmarshal(args, &p) == nil && p.RunInBackground
}

func withExpertTeamParentCancellation(args json.RawMessage) (json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil {
		return args, false
	}
	obj["cancel_with_parent"] = json.RawMessage("true")
	patched, err := json.Marshal(obj)
	if err != nil {
		return args, false
	}
	return patched, true
}

func withExpertTeamDefaultTaskMaxSteps(args json.RawMessage) (json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil {
		if strings.TrimSpace(string(args)) != "" {
			return args, false
		}
	}
	if obj == nil {
		obj = map[string]json.RawMessage{}
	}
	obj["max_steps"] = json.RawMessage(strconv.Itoa(expertTeamTaskDefaultMaxSteps))
	patched, err := json.Marshal(obj)
	if err != nil {
		return args, false
	}
	return patched, true
}

func withExpertTeamDefaultParallelTaskMaxSteps(args json.RawMessage) (json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil {
		return args, false
	}
	tasksRaw, ok := obj["tasks"]
	if !ok {
		return args, false
	}
	var tasks []map[string]json.RawMessage
	if err := json.Unmarshal(tasksRaw, &tasks); err != nil {
		return args, false
	}

	changed := false
	for i := range tasks {
		if rawMax, ok := tasks[i]["max_steps"]; ok && expertTeamJSONInt(rawMax) > 0 {
			continue
		}
		tasks[i]["max_steps"] = json.RawMessage(strconv.Itoa(expertTeamTaskDefaultMaxSteps))
		changed = true
	}
	if !changed {
		return args, false
	}

	patchedTasks, err := json.Marshal(tasks)
	if err != nil {
		return args, false
	}
	obj["tasks"] = patchedTasks
	patched, err := json.Marshal(obj)
	if err != nil {
		return args, false
	}
	return patched, true
}

func expertTeamJSONInt(raw json.RawMessage) int {
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0
	}
	return n
}

func isExpertTeamHiddenPrompt(input string) bool {
	_, ok := expertTeamConfiguredMembers(input)
	return ok
}

func expertTeamConfiguredMembers(input string) ([]string, bool) {
	routingInput := StripTransientUserBlocks(input)
	if goal, ok := leadingActiveGoalEnvelopeBody(routingInput); ok {
		routingInput = goal
	}
	attrs, _, ok := hiddenSystemPromptPrefixAttrs(routingInput)
	if !ok || !strings.EqualFold(strings.TrimSpace(attrs["source"]), "expert") ||
		!strings.EqualFold(strings.TrimSpace(attrs["mode"]), "team") {
		return nil, false
	}
	var rawNames []string
	if err := json.Unmarshal([]byte(attrs["member-names"]), &rawNames); err != nil {
		return nil, true
	}
	names := make([]string, 0, len(rawNames))
	seen := make(map[string]bool, len(rawNames))
	for _, raw := range rawNames {
		name := strings.TrimSpace(raw)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, name)
	}
	return names, true
}

func leadingActiveGoalEnvelopeBody(input string) (string, bool) {
	const (
		open  = "<active-goal>"
		close = "</active-goal>"
	)
	s := strings.TrimLeft(input, " \t\r\n")
	if !strings.HasPrefix(s, open) {
		return "", false
	}
	bodyStart := len(open)
	bodyEnd := strings.Index(s[bodyStart:], close)
	if bodyEnd < 0 {
		return "", false
	}
	return strings.TrimSpace(s[bodyStart : bodyStart+bodyEnd]), true
}

func hiddenSystemPromptPrefixAttrs(input string) (map[string]string, string, bool) {
	s := strings.TrimLeft(input, " \t\r\n")
	if !strings.HasPrefix(s, "<system-prompt") {
		return nil, "", false
	}
	decoder := xml.NewDecoder(strings.NewReader(s))
	token, err := decoder.Token()
	if err != nil {
		return nil, "", false
	}
	start, ok := token.(xml.StartElement)
	if !ok || start.Name.Local != "system-prompt" {
		return nil, "", false
	}
	attrs := make(map[string]string, len(start.Attr))
	for _, attr := range start.Attr {
		attrs[attr.Name.Local] = attr.Value
	}
	if !strings.EqualFold(strings.TrimSpace(attrs["hidden"]), "true") {
		return nil, "", false
	}
	bodyStart := int(decoder.InputOffset())
	end := strings.Index(strings.ToLower(s[bodyStart:]), "</system-prompt>")
	if end < 0 {
		return nil, "", false
	}
	body := s[bodyStart : bodyStart+end]
	return attrs, strings.TrimSpace(body), true
}
