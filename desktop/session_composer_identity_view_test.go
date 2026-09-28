package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/control"
	"kwu/internal/expert"
	"kwu/internal/modelbilling"
)

type composerIdentityExpertSession struct {
	control.SessionAPI
	state expert.State
}

func (s *composerIdentityExpertSession) CurrentExpert() expert.State {
	return s.state
}

func TestListSessionsExposesPersistedComposerIdentity(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	dir := app.activeSessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := writeTopicSessionWithPrompt(
		t,
		dir,
		"composer-identity.jsonl",
		"composer-identity-topic",
		"Composer identity",
		"",
		"verify identity",
		time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC),
	)
	meta, err := agent.EnsureBranchMeta(path)
	if err != nil {
		t.Fatalf("ensure branch meta: %v", err)
	}
	meta.Model = "provider/model-b"
	meta.BillingSource = string(modelbilling.BillingNewAPIByok)
	high := "high"
	meta.Effort = &high
	if err := agent.SaveBranchMetaPreserveUpdated(path, meta); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	if err := agent.SaveSessionExpert(path, agent.ExpertMeta{ExpertID: "team-b", ExpertName: "Team B"}); err != nil {
		t.Fatalf("save session expert: %v", err)
	}

	sessions := app.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("ListSessions len = %d, want 1: %+v", len(sessions), sessions)
	}
	got := sessions[0]
	if got.ModelRef != "provider/model-b" || got.BillingSource != string(modelbilling.BillingNewAPIByok) {
		t.Fatalf("persisted model identity = %q/%q, want provider/model-b with BYOK", got.ModelRef, got.BillingSource)
	}
	if got.Effort == nil || *got.Effort != high {
		t.Fatalf("persisted effort identity = %#v, want %q", got.Effort, high)
	}
	if got.ExpertID != "team-b" || got.ExpertName != "Team B" {
		t.Fatalf("persisted expert identity = %q/%q, want team-b/Team B", got.ExpertID, got.ExpertName)
	}
	if got.CollaborationMode != "normal" || got.ToolApprovalMode != control.ToolApprovalAsk || got.TokenMode != boot.TokenModeFull || got.Goal != "" {
		t.Fatalf("closed-session safe profile = collaboration:%q approval:%q token:%q goal:%q", got.CollaborationMode, got.ToolApprovalMode, got.TokenMode, got.Goal)
	}
}

func TestListSessionsOmitsUnknownPersistedEffort(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	dir := app.activeSessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := writeTopicSessionWithPrompt(
		t,
		dir,
		"legacy-composer-identity.jsonl",
		"legacy-composer-identity-topic",
		"Legacy composer identity",
		"",
		"verify legacy identity",
		time.Date(2026, 8, 4, 8, 30, 0, 0, time.UTC),
	)
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{
		Model:         "provider/legacy-model",
		BillingSource: string(modelbilling.BillingNewAPIByok),
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}

	sessions := app.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("ListSessions len = %d, want 1: %+v", len(sessions), sessions)
	}
	if sessions[0].Effort != nil {
		t.Fatalf("legacy persisted effort = %#v, want unknown", sessions[0].Effort)
	}
	body, err := json.Marshal(sessions[0])
	if err != nil {
		t.Fatalf("marshal session meta: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("unmarshal session meta: %v", err)
	}
	if _, ok := wire["effort"]; ok {
		t.Fatalf("legacy session JSON unexpectedly exposes effort: %s", body)
	}
}

func TestListProjectTreeExposesStaticAndRuntimeComposerIdentity(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	topic, err := app.CreateTopic("global", "", "Composer identity")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	dir := desktopSessionDir(globalWorkspaceRoot())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := writeTopicSessionWithPrompt(
		t,
		dir,
		"project-tree-composer-identity.jsonl",
		topic.ID,
		topic.Title,
		"",
		"verify project tree identity",
		time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC),
	)
	meta, err := agent.EnsureBranchMeta(path)
	if err != nil {
		t.Fatalf("ensure branch meta: %v", err)
	}
	meta.Model = "provider/static-model"
	meta.BillingSource = string(modelbilling.BillingNewAPIByok)
	high := "high"
	meta.Effort = &high
	if err := agent.SaveBranchMetaPreserveUpdated(path, meta); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	if err := agent.SaveSessionExpert(path, agent.ExpertMeta{ExpertID: "team-static", ExpertName: "Static Team"}); err != nil {
		t.Fatalf("save session expert: %v", err)
	}

	findTopic := func(nodes []ProjectNode) ProjectNode {
		t.Helper()
		for _, parent := range nodes {
			for _, child := range parent.Children {
				if child.TopicID == topic.ID {
					return child
				}
			}
		}
		t.Fatalf("topic %q missing from project tree: %+v", topic.ID, nodes)
		return ProjectNode{}
	}

	static := findTopic(app.ListProjectTree())
	if static.ModelRef != "provider/static-model" || static.ExpertID != "team-static" || static.ExpertName != "Static Team" {
		t.Fatalf("static identity = model:%q expert:%q/%q", static.ModelRef, static.ExpertID, static.ExpertName)
	}
	if static.Effort == nil || *static.Effort != high {
		t.Fatalf("static effort identity = %#v, want %q", static.Effort, high)
	}
	if static.CollaborationMode != "normal" || static.ToolApprovalMode != control.ToolApprovalAsk || static.TokenMode != boot.TokenModeFull {
		t.Fatalf("static safe profile = collaboration:%q approval:%q token:%q", static.CollaborationMode, static.ToolApprovalMode, static.TokenMode)
	}

	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "runtime"})
	ctrl.SetPlanMode(true)
	ctrl.SetToolApprovalMode(control.ToolApprovalYolo)
	defer ctrl.Close()
	tab := &WorkspaceTab{
		ID:               "runtime-composer-identity",
		Scope:            "global",
		WorkspaceRoot:    globalWorkspaceRoot(),
		TopicID:          topic.ID,
		TopicTitle:       topic.Title,
		SessionPath:      path,
		Ctrl:             ctrl,
		model:            "provider/runtime-model",
		billingSource:    string(modelbilling.BillingNewAPIByok),
		tokenMode:        boot.TokenModeEconomy,
		mode:             "plan-yolo",
		toolApprovalMode: control.ToolApprovalYolo,
		disabledMCP:      map[string]ServerView{},
	}
	low := "low"
	tab.effort = &low
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	runtime := findTopic(app.ListProjectTree())
	if runtime.ModelRef != "provider/runtime-model" || runtime.BillingSource != string(modelbilling.BillingNewAPIByok) {
		t.Fatalf("runtime model identity = %q/%q", runtime.ModelRef, runtime.BillingSource)
	}
	if runtime.Effort == nil || *runtime.Effort != low {
		t.Fatalf("runtime effort identity = %#v, want %q", runtime.Effort, low)
	}
	if runtime.CollaborationMode != "plan" || runtime.ToolApprovalMode != control.ToolApprovalYolo || runtime.TokenMode != boot.TokenModeEconomy {
		t.Fatalf("runtime profile = collaboration:%q approval:%q token:%q", runtime.CollaborationMode, runtime.ToolApprovalMode, runtime.TokenMode)
	}
	if filepath.Clean(runtime.SessionPath) != filepath.Clean(path) {
		t.Fatalf("runtime session path = %q, want %q", runtime.SessionPath, path)
	}
}

func TestListTabsKeepsCompletedSessionExpertIdentity(t *testing.T) {
	app := NewApp()
	ctrl := control.New(control.Options{})
	defer ctrl.Close()
	tab := &WorkspaceTab{
		ID:            "completed-expert-team",
		Scope:         "global",
		WorkspaceRoot: globalWorkspaceRoot(),
		TopicID:       "completed-topic",
		TopicTitle:    "Completed expert task",
		SessionPath:   filepath.Join(t.TempDir(), "completed.jsonl"),
		Ctrl: &composerIdentityExpertSession{
			SessionAPI: ctrl,
			state: expert.State{
				SelectedID:   "delivery-team",
				SelectedName: "Delivery Team",
				Source:       expert.SourceSession,
			},
		},
		Ready:       true,
		model:       "provider/model",
		disabledMCP: map[string]ServerView{},
	}
	max := "max"
	tab.effort = &max
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	metas := app.ListTabs()
	if len(metas) != 1 {
		t.Fatalf("ListTabs len = %d, want 1", len(metas))
	}
	if metas[0].ExpertID != "delivery-team" || metas[0].ExpertName != "Delivery Team" {
		t.Fatalf("completed tab expert identity = %q/%q, want delivery-team/Delivery Team", metas[0].ExpertID, metas[0].ExpertName)
	}
	if metas[0].Effort == nil || *metas[0].Effort != max {
		t.Fatalf("completed tab effort identity = %#v, want %q", metas[0].Effort, max)
	}
}
