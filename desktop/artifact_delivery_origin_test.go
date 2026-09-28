package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

func TestTurnDoneArtifactUsesCapturedOriginAfterControllerSessionSwitch(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	originSession := writeTopicSessionWithPrompt(
		t, sessionDir, "origin.jsonl", "topic-origin", "Origin", workspaceRoot, "create report", time.Now(),
	)
	currentSession := writeTopicSessionWithPrompt(
		t, sessionDir, "current.jsonl", "topic-current", "Current", workspaceRoot, "new conversation", time.Now(),
	)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "report.md"), []byte("report"), 0o644); err != nil {
		t.Fatal(err)
	}

	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "new conversation"})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor: executor, SessionDir: sessionDir, SessionPath: currentSession,
		WorkspaceRoot: workspaceRoot, Sink: event.Discard,
	})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{ID: "artifact_origin_tab", Ctrl: ctrl, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	delivery := &provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"report.md"}, DeliveredAt: 1234,
	}
	tab.sink.Emit(event.Event{
		Kind: event.TurnDone, ArtifactDelivery: delivery,
		OriginSessionPath: originSession, OriginWorkspaceRoot: workspaceRoot,
	})
	waitForFile(t, artifactSidecarPath(originSession), "report.md")
	waitForAutosaveIdle(t, tab)

	if _, err := os.Stat(artifactSidecarPath(currentSession)); !os.IsNotExist(err) {
		t.Fatalf("current session received origin artifact sidecar: %v", err)
	}
}

func TestStaleTurnDoneKeepsCurrentTurnStateAndPersistsOriginArtifact(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	originSession := writeTopicSessionWithPrompt(
		t, sessionDir, "stale-origin.jsonl", "topic-stale-origin", "Stale origin", workspaceRoot, "create report", time.Now(),
	)
	currentSession := writeTopicSessionWithPrompt(
		t, sessionDir, "active.jsonl", "topic-active", "Active", workspaceRoot, "continue working", time.Now(),
	)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "report.md"), []byte("report"), 0o644); err != nil {
		t.Fatal(err)
	}

	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "continue working"})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor: executor, SessionDir: sessionDir, SessionPath: currentSession,
		WorkspaceRoot: workspaceRoot, Sink: event.Discard,
	})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{ID: "stale_turn_tab", Ctrl: ctrl, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	tab.sink.Emit(event.Event{Kind: event.TurnStarted, TurnID: "current-turn"})
	tab.sink.Emit(event.Event{
		Kind: event.Usage, TurnID: "current-turn",
		Usage: &provider.Usage{CompletionTokens: 11, TotalTokens: 11},
	})
	tab.sink.Emit(event.Event{Kind: event.Text, TurnID: "current-turn", Source: event.UsageSourcePlanner, Text: "current draft"})
	delivery := &provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"report.md"}, DeliveredAt: 1234,
	}
	tab.sink.Emit(event.Event{
		Kind: event.TurnDone, TurnID: "stale-turn", ArtifactDelivery: delivery,
		OriginSessionPath: originSession, OriginWorkspaceRoot: workspaceRoot,
	})

	waitForFile(t, artifactSidecarPath(originSession), "report.md")
	waitForAutosaveIdle(t, tab)
	_, startedAt, completionTokens := tab.contextUsageSnapshot()
	if startedAt <= 0 || completionTokens != 11 {
		t.Fatalf("active turn telemetry = started:%d completion:%d, want positive/11", startedAt, completionTokens)
	}
	app.mu.RLock()
	status, statusTurnID := tab.ActivityStatus, tab.activityTurnID
	app.mu.RUnlock()
	if status != topicStatusStreaming || statusTurnID != "current-turn" {
		t.Fatalf("active status = %q owned by %q, want %q/current-turn", status, statusTurnID, topicStatusStreaming)
	}
	tab.plannerDisplayMu.Lock()
	plannerDisplay := cloneHistoryMessages(tab.plannerDisplay)
	tab.plannerDisplayMu.Unlock()
	if len(plannerDisplay) != 1 || plannerDisplay[0].Content != "current draft" {
		t.Fatalf("current planner display was flushed by stale terminal: %#v", plannerDisplay)
	}
	if _, err := os.Stat(artifactSidecarPath(currentSession)); !os.IsNotExist(err) {
		t.Fatalf("current session received stale origin artifact sidecar: %v", err)
	}
}
