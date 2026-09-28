package main

import (
	"encoding/json"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/provider"
)

type TaskLifecycleView struct {
	SubagentRef string                        `json:"subagentRef"`
	TaskRunID   string                        `json:"taskRunId,omitempty"`
	DependsOn   []string                      `json:"dependsOn,omitempty"`
	MemberName  string                        `json:"memberName,omitempty"`
	TurnStatus  agent.SubagentStatus          `json:"turnStatus"`
	FinalAnswer string                        `json:"finalAnswer,omitempty"`
	Artifacts   []TaskLifecycleArtifactView   `json:"artifacts,omitempty"`
	Validations []TaskLifecycleValidationView `json:"validations,omitempty"`
	Error       string                        `json:"error,omitempty"`
}

type TaskLifecycleArtifactView struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Source string `json:"source,omitempty"`
}

type TaskLifecycleValidationView struct {
	Kind    string `json:"kind"`
	Success bool   `json:"success"`
	Path    string `json:"path,omitempty"`
	Status  string `json:"status,omitempty"`
}

type historyToolLocation struct{ message, call int }

func attachTaskLifecycles(messages []HistoryMessage, raw []provider.Message) []HistoryMessage {
	locations := make(map[string]historyToolLocation)
	for i := range messages {
		for j := range messages[i].ToolCalls {
			if id := strings.TrimSpace(messages[i].ToolCalls[j].ID); id != "" {
				locations[id] = historyToolLocation{message: i, call: j}
			}
		}
	}
	toolNames := make(map[string]string)
	latestTaskByRef := make(map[string]string)
	for _, message := range raw {
		for _, call := range message.ToolCalls {
			toolNames[call.ID] = call.Name
		}
		if message.Role != provider.RoleTool || strings.TrimSpace(message.ToolCallID) == "" {
			continue
		}
		name := strings.TrimSpace(message.Name)
		if name == "" {
			name = toolNames[message.ToolCallID]
		}
		switch name {
		case "task":
			ref := taskStartRef(message.Content)
			if ref == "" || !strings.Contains(message.Content, "Turn status: running") {
				continue
			}
			setHistoryTaskLifecycle(messages, locations, message.ToolCallID, TaskLifecycleView{
				SubagentRef: ref, TaskRunID: taskStartLine(message.Content, "Task run ID:"), DependsOn: taskStartList(message.Content, "Dependencies:"), TurnStatus: agent.SubagentRunning,
			})
			latestTaskByRef[ref] = message.ToolCallID
		case "wait_task", "stop_task":
			receipts, ok := decodeTaskLifecycleViews(message.Content)
			if !ok {
				continue
			}
			for _, receipt := range receipts {
				ref := strings.TrimSpace(receipt.SubagentRef)
				if ref == "" {
					continue
				}
				if taskCallID := latestTaskByRef[ref]; taskCallID != "" {
					setHistoryTaskLifecycle(messages, locations, taskCallID, receipt)
				}
			}
			setHistoryTaskControlReceipts(messages, locations, message.ToolCallID, receipts)
		}
	}
	return messages
}

func setHistoryTaskLifecycle(messages []HistoryMessage, locations map[string]historyToolLocation, callID string, lifecycle TaskLifecycleView) {
	location, ok := locations[callID]
	if !ok {
		return
	}
	call := &messages[location.message].ToolCalls[location.call]
	call.SubagentRef = lifecycle.SubagentRef
	call.TaskLifecycle = cloneTaskLifecycleView(&lifecycle)
}

func setHistoryTaskControlReceipts(messages []HistoryMessage, locations map[string]historyToolLocation, callID string, receipts []TaskLifecycleView) {
	location, ok := locations[callID]
	if !ok || len(receipts) == 0 {
		return
	}
	messages[location.message].ToolCalls[location.call].TaskLifecycleReceipts = cloneTaskLifecycleViews(receipts)
}

func decodeTaskLifecycleViews(output string) ([]TaskLifecycleView, bool) {
	data := []byte(strings.TrimSpace(output))
	if len(data) == 0 {
		return nil, false
	}
	if data[0] == '[' {
		var receipts []TaskLifecycleView
		if json.Unmarshal(data, &receipts) != nil || len(receipts) == 0 {
			return nil, false
		}
		return receipts, true
	}
	var receipt TaskLifecycleView
	if json.Unmarshal(data, &receipt) != nil || strings.TrimSpace(receipt.SubagentRef) == "" {
		return nil, false
	}
	return []TaskLifecycleView{receipt}, true
}

func taskStartRef(output string) string {
	return taskStartLine(output, "Subagent reference:")
}

func taskStartLine(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func taskStartList(output, prefix string) []string {
	line := taskStartLine(output, prefix)
	if line == "" {
		return nil
	}
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cloneTaskLifecycleView(in *TaskLifecycleView) *TaskLifecycleView {
	if in == nil {
		return nil
	}
	out := *in
	out.DependsOn = append([]string(nil), in.DependsOn...)
	out.Artifacts = append([]TaskLifecycleArtifactView(nil), in.Artifacts...)
	out.Validations = append([]TaskLifecycleValidationView(nil), in.Validations...)
	return &out
}

func cloneTaskLifecycleViews(in []TaskLifecycleView) []TaskLifecycleView {
	out := make([]TaskLifecycleView, len(in))
	for i := range in {
		out[i] = *cloneTaskLifecycleView(&in[i])
	}
	return out
}
