package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kwu/internal/event"
)

func TestWireEventTabPreservesSharedRetryingFields(t *testing.T) {
	w := toWireTab(event.Event{Kind: event.Retrying, RetryAttempt: 3, RetryMax: 10}, "tab-1")
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{`"kind":"retrying"`, `"retryAttempt":3`, `"retryMax":10`, `"tabId":"tab-1"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("tab retrying JSON = %s, want it to contain %s", s, want)
		}
	}
}

func TestWireEventTabProjectsSuccessfulDeliverFilesPaths(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "report.md"), filepath.Join(root, "chart.png")}
	w := toWireTab(event.Event{Kind: event.ToolResult, Tool: event.Tool{
		ID: "deliver-1", Name: deliverFilesToolName, Output: "truncated output", RawOutput: mustDeliverFilesResultJSON(t, paths...), Truncated: true,
	}}, "tab-1")
	if w.Tool == nil || !slices.Equal(w.Tool.ArtifactPaths, paths) {
		t.Fatalf("wire artifact paths = %+v", w.Tool)
	}
	if w.Tool.Output != "truncated output" || !w.Tool.Truncated {
		t.Fatalf("wire display output changed = %+v", w.Tool)
	}

	failed := toWireTab(event.Event{Kind: event.ToolResult, Tool: event.Tool{
		ID: "deliver-2", Name: deliverFilesToolName, RawOutput: mustDeliverFilesResultJSON(t, paths...), Err: "delivery failed",
	}}, "tab-1")
	if failed.Tool == nil || len(failed.Tool.ArtifactPaths) != 0 {
		t.Fatalf("failed wire artifact paths = %+v", failed.Tool)
	}
}

func TestWireEventTabIncludesStableConversationRoute(t *testing.T) {
	w := toWireTab(event.Event{Kind: event.TurnDone}, "detached-1")
	w.ConversationScope = "project"
	w.ConversationWorkspaceRoot = `C:\repo`
	w.ConversationTopicID = "topic-1"
	w.ConversationSessionPath = `C:\sessions\one.jsonl`
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		`"conversationScope":"project"`,
		`"conversationWorkspaceRoot":"C:\\repo"`,
		`"conversationTopicId":"topic-1"`,
		`"conversationSessionPath":"C:\\sessions\\one.jsonl"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("tab conversation route JSON = %s, want it to contain %s", s, want)
		}
	}
}

func TestTabEventSinkConversationRouteFindsDetachedRuntime(t *testing.T) {
	sessionPath := sessionRuntimeKey(filepath.Join(t.TempDir(), "session.jsonl"))
	tabID := detachedRuntimeTabID(sessionPath)
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{},
		detachedSessions: map[string]*WorkspaceTab{
			sessionPath: {
				ID:            tabID,
				Scope:         "project",
				WorkspaceRoot: `C:\repo`,
				TopicID:       "topic-1",
				SessionPath:   sessionPath,
			},
		},
	}

	route := (&tabEventSink{tabID: tabID, app: app}).conversationRoute()
	if route.scope != "project" || route.workspaceRoot != `C:\repo` || route.topicID != "topic-1" || route.sessionPath != sessionPath {
		t.Fatalf("detached conversation route = %+v", route)
	}
}
