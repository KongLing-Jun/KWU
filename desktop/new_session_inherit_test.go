package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
)

func TestEnsureBlankTabInheritsOnlyExecutionLocalSettings(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspace := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(workspace, "kwu.toml"),
		[]byte(""), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	effort := "max"
	app := NewApp()
	src := &WorkspaceTab{
		ID:               "src",
		Scope:            "project",
		WorkspaceRoot:    workspace,
		TopicID:          "topic_src",
		SessionPath:      filepath.Join(workspace, "src.jsonl"), // non-empty so src isn't reused as the blank tab
		model:            "inherit/model",
		effort:           &effort,
		tokenMode:        "economy",
		mode:             "plan",
		toolApprovalMode: control.ToolApprovalYolo,
		disabledMCP:      map[string]ServerView{"srv-x": {}},
		mcpOrder:         []string{"srv-x", "srv-y"},
	}
	src.sink = &tabEventSink{tabID: "src", app: app}
	app.tabRuntimes["src"] = src
	app.tabOrder = []string{"src"}
	app.activeTabID = "src"

	meta, err := app.EnsureBlankTab("project", workspace)
	if err != nil {
		t.Fatalf("EnsureBlankTab: %v", err)
	}
	if meta.ID == "" || meta.ID == "src" {
		t.Fatalf("expected a fresh tab, got meta.ID=%q", meta.ID)
	}

	created := app.tabRuntimes[meta.ID]
	if created == nil {
		t.Fatalf("new tab %q missing from app.tabRuntimes", meta.ID)
	}
	if created.effort != nil && *created.effort == "max" {
		t.Fatalf("effort = %v, must not inherit active session max", created.effort)
	}
	if got := app.EffortForTab(created.ID).Current; got != "auto" {
		t.Fatalf("new session effort = %q, want global default auto", got)
	}
	if created.tokenMode != "economy" {
		t.Fatalf("tokenMode = %q, want inherited \"economy\"", created.tokenMode)
	}
	if created.toolApprovalMode != control.ToolApprovalAsk {
		t.Fatalf("toolApprovalMode = %q, want global default ask", created.toolApprovalMode)
	}
	if created.mode != "normal" {
		t.Fatalf("mode = %q, want global default normal", created.mode)
	}
	if _, ok := created.disabledMCP["srv-x"]; !ok {
		t.Fatalf("disabledMCP = %v, want inherited key \"srv-x\"", created.disabledMCP)
	}
	if len(created.mcpOrder) != 2 || created.mcpOrder[0] != "srv-x" {
		t.Fatalf("mcpOrder = %v, want inherited [srv-x srv-y]", created.mcpOrder)
	}
}

func TestEnsureBlankTabUsesGlobalSessionDefaultsForModelAndToolApproval(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspace := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(workspace, "kwu.toml"),
		[]byte(""), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	if err := cfg.SetDefaultModel(testKWUDefaultModelRef); err != nil {
		t.Fatalf("SetDefaultModel: %v", err)
	}
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingSubscriptionPool)
	if err := cfg.SetDesktopDefaultToolApprovalMode(control.ToolApprovalAuto); err != nil {
		t.Fatalf("SetDesktopDefaultToolApprovalMode: %v", err)
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	app := NewApp()
	src := &WorkspaceTab{
		ID:               "src",
		Scope:            "project",
		WorkspaceRoot:    workspace,
		TopicID:          "topic_src",
		SessionPath:      filepath.Join(workspace, "src.jsonl"),
		model:            testKWUFlashModelRef,
		mode:             "plan",
		toolApprovalMode: control.ToolApprovalAsk,
		disabledMCP:      map[string]ServerView{},
	}
	src.sink = &tabEventSink{tabID: "src", app: app}
	app.tabRuntimes["src"] = src
	app.tabOrder = []string{"src"}
	app.activeTabID = "src"

	meta, err := app.EnsureBlankTab("project", workspace)
	if err != nil {
		t.Fatalf("EnsureBlankTab: %v", err)
	}
	created := app.tabRuntimes[meta.ID]
	if created == nil {
		t.Fatalf("new tab %q missing from app.tabRuntimes", meta.ID)
	}
	if created.model != testKWUDefaultModelRef {
		t.Fatalf("new tab model = %q, want global default model", created.model)
	}
	if created.toolApprovalMode != control.ToolApprovalAuto {
		t.Fatalf("new tab toolApprovalMode = %q, want global default auto", created.toolApprovalMode)
	}
	if src.model != testKWUFlashModelRef || src.toolApprovalMode != control.ToolApprovalAsk {
		t.Fatalf("existing tab should not be overwritten, got model=%q approval=%q", src.model, src.toolApprovalMode)
	}
	if !strings.Contains(filepath.Base(created.SessionPath), "deepseek-v4-flash") {
		t.Fatalf("new session path = %q, want filename seeded by global default model", created.SessionPath)
	}
}

func TestEnsureBlankTabRefreshesReusableTabToolApprovalFromGlobalDefault(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	first, err := app.EnsureBlankTab("global", "")
	if err != nil {
		t.Fatalf("first EnsureBlankTab: %v", err)
	}
	if first.ToolApprovalMode != control.ToolApprovalAsk {
		t.Fatalf("initial toolApprovalMode = %q, want ask", first.ToolApprovalMode)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	if err := cfg.SetDesktopDefaultToolApprovalMode(control.ToolApprovalAuto); err != nil {
		t.Fatalf("SetDesktopDefaultToolApprovalMode: %v", err)
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	second, err := app.EnsureBlankTab("global", "")
	if err != nil {
		t.Fatalf("second EnsureBlankTab: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("reusable blank tab ID = %q, want %q", second.ID, first.ID)
	}
	if second.ToolApprovalMode != control.ToolApprovalAuto {
		t.Fatalf("reused toolApprovalMode = %q, want refreshed global default auto", second.ToolApprovalMode)
	}
}

func TestChangedToolApprovalModeBecomesNextSessionDefaultFromDatabase(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	first, err := app.EnsureBlankTab("global", "")
	if err != nil {
		t.Fatalf("first EnsureBlankTab: %v", err)
	}
	if err := app.SetToolApprovalModeForTab(first.ID, control.ToolApprovalAuto); err != nil {
		t.Fatalf("SetToolApprovalModeForTab: %v", err)
	}

	restarted := NewApp()
	second, err := restarted.EnsureBlankTab("global", "")
	if err != nil {
		t.Fatalf("second EnsureBlankTab: %v", err)
	}
	if second.ToolApprovalMode != control.ToolApprovalAuto {
		t.Fatalf("next session toolApprovalMode = %q, want persisted auto", second.ToolApprovalMode)
	}
}
