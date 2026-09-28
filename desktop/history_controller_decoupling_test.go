package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

func TestHistoryForTabReadsCompleteDiskHistoryWithoutController(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "local-history.jsonl")
	const submitted = "expanded local prompt"
	const taskCallID = "task_local_history"
	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: submitted})
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "local answer", ToolCalls: []provider.ToolCall{{
		ID: taskCallID, Name: "task", Arguments: `{"description":"Research Expert"}`,
	}}})
	if err := session.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := recordSessionDisplay(dir, path, submitted, "visible local prompt"); err != nil {
		t.Fatal(err)
	}
	if err := recordSessionPlannerDisplay(dir, path, submitted, []HistoryMessage{{Role: "phase", Content: "planner phase"}}); err != nil {
		t.Fatal(err)
	}
	ref := saveCompletedHistorySubagent(t, dir, agent.BranchID(path), taskCallID, []provider.Message{{
		Role: provider.RoleAssistant, Content: "subagent disk answer",
	}})

	app, tab := newControllerlessHistoryTab(root, path)
	if got := tabSessionDir(tab); filepath.Clean(got) != filepath.Clean(dir) {
		t.Fatalf("tab session dir = %q, want %q", got, dir)
	}
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) < 3 || history[0].Content != "visible local prompt" || history[0].SubmitText != submitted {
		t.Fatalf("disk history display = %+v", history)
	}
	if history[1].Role != "phase" || history[1].Content != "planner phase" {
		t.Fatalf("planner display missing from disk history: %+v", history)
	}
	call := requireHistoryToolCall(t, history, taskCallID)
	if call.SubagentRef != ref || len(call.SubagentMessages) != 0 {
		t.Fatalf("subagent disk history should be lazy: %+v", call)
	}
	subagent, err := app.SubagentHistoryForTab(tab.ID, ref)
	if err != nil || len(subagent) != 1 || subagent[0].Content != "subagent disk answer" {
		t.Fatalf("lazy subagent disk history = %+v / %v", subagent, err)
	}
}

func TestHistoryForTabPrefersLiveControllerHistory(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "live-priority.jsonl")
	writeHistoryTestSession(t, path, "disk prompt")
	live := agent.NewSession("")
	live.Add(provider.Message{Role: provider.RoleUser, Content: "live prompt"})
	executor := agent.New(nil, nil, live, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: executor, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	defer ctrl.Close()

	app, tab := newControllerlessHistoryTab(root, path)
	tab.Ctrl = ctrl
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) != 1 || history[0].Content != "live prompt" {
		t.Fatalf("HistoryForTab() = %+v, want live controller history", history)
	}
}

func TestTabSessionTurnsRefreshUsesLiveHistory(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "live-turns.jsonl")
	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	executor := agent.New(nil, nil, session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: executor, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	defer ctrl.Close()

	app, tab := newControllerlessHistoryTab(root, path)
	tab.Ctrl = ctrl
	tab.Ready = true
	app.refreshTabSessionTurns(tab.ID)
	tabs := app.ListTabs()
	if len(tabs) != 1 || tabs[0].SessionTurns == nil || *tabs[0].SessionTurns != 1 {
		t.Fatalf("live turns after first refresh = %+v, want 1", tabs)
	}

	session.Add(provider.Message{Role: provider.RoleUser, Content: "second"})
	app.refreshTabSessionTurns(tab.ID)
	tabs = app.ListTabs()
	if tabs[0].SessionTurns == nil || *tabs[0].SessionTurns != 2 {
		t.Fatalf("live turns after second refresh = %+v, want 2", tabs)
	}

	session.Replace(nil)
	app.refreshTabSessionTurns(tab.ID)
	tabs = app.ListTabs()
	if tabs[0].SessionTurns == nil || *tabs[0].SessionTurns != 0 {
		t.Fatalf("live turns after rewrite = %+v, want explicit zero", tabs)
	}
}

func TestHistoryForTabEmptyAndUnknownTabs(t *testing.T) {
	app, tab := newControllerlessHistoryTab(globalTabWorkspaceRoot(), "")
	history, err := app.HistoryForTab(tab.ID)
	if err != nil || history == nil || len(history) != 0 {
		t.Fatalf("empty tab history = %#v, %v; want [] nil", history, err)
	}
	if _, err := app.HistoryForTab("missing"); err == nil {
		t.Fatal("unknown tab history error = nil")
	}
}

func TestPinnedSessionRestorePrefersRegisteredTopicTitle(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	root := robustTempDir(t)
	if err := addProject(root, ""); err != nil {
		t.Fatal(err)
	}
	const topicID = "topic_registered_title"
	if err := saveTopicTitles(root, map[string]string{topicID: "正确的主题标题"}); err != nil {
		t.Fatal(err)
	}
	actor := app.actorForWorkspace(root)
	path, err := createEmptySessionFile(desktopSessionDirForActor(actor, root), "kwu/test-model")
	if err != nil {
		t.Fatal(err)
	}
	meta := agent.BranchMeta{
		Scope:         "project",
		WorkspaceRoot: root,
		TopicID:       topicID,
		TopicTitle:    defaultTopicTitle,
	}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}

	tab := &WorkspaceTab{
		ID: "registered-title", Scope: "project", WorkspaceRoot: root,
		TopicID: topicID, TopicTitle: "正确的主题标题", SessionPath: path,
		actor: actor, sink: &tabEventSink{tabID: "registered-title", app: app},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.reconcileTabWithPinnedSessionMeta(tab)
	if tab.TopicTitle != "正确的主题标题" {
		t.Fatalf("restored topic title = %q, want registered title", tab.TopicTitle)
	}
}

func TestPinnedSessionRestoreFallsBackToSessionTopicTitle(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	root := robustTempDir(t)
	if err := addProject(root, ""); err != nil {
		t.Fatal(err)
	}
	const topicID = "topic_session_title_fallback"
	actor := app.actorForWorkspace(root)
	path, err := createEmptySessionFile(desktopSessionDirForActor(actor, root), "kwu/test-model")
	if err != nil {
		t.Fatal(err)
	}
	meta := agent.BranchMeta{
		Scope:         "project",
		WorkspaceRoot: root,
		TopicID:       topicID,
		TopicTitle:    "Session 中保留的标题",
	}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}

	tab := &WorkspaceTab{
		ID: "session-title", Scope: "project", WorkspaceRoot: root,
		TopicID: topicID, TopicTitle: defaultTopicTitle, SessionPath: path,
		actor: actor, sink: &tabEventSink{tabID: "session-title", app: app},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.reconcileTabWithPinnedSessionMeta(tab)
	if tab.TopicTitle != "Session 中保留的标题" {
		t.Fatalf("restored topic title = %q, want Session fallback", tab.TopicTitle)
	}
}

func TestBlankProjectSessionMetadataDoesNotChangeItsRoute(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	root := robustTempDir(t)
	if err := addProject(root, ""); err != nil {
		t.Fatal(err)
	}
	actor := app.actorForWorkspace(root)
	dir := desktopSessionDirForActor(actor, root)
	path, err := createEmptySessionFile(dir, "kwu/stale-model")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.EnsureBranchMeta(path); err != nil {
		t.Fatal(err)
	}

	tab := &WorkspaceTab{
		ID: "blank-project", Scope: "project", WorkspaceRoot: root, SessionPath: path,
		actor: actor, sink: &tabEventSink{tabID: "blank-project", app: app},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.reconcileTabWithPinnedSessionMeta(tab)
	if tab.Scope != "project" || normalizeProjectRoot(tab.WorkspaceRoot) != normalizeProjectRoot(root) {
		t.Fatalf("blank project route = %q/%q, want project/%q", tab.Scope, tab.WorkspaceRoot, root)
	}
	history, err := app.HistoryForTab(tab.ID)
	if err != nil || len(history) != 0 {
		t.Fatalf("blank project history = %+v, %v; want empty history (tab dir %q, created dir %q, route %q)", history, err, tabSessionDir(tab), dir, tab.WorkspaceRoot)
	}
}

func TestBlankProjectSessionRepairsPreviouslyPersistedGlobalRoute(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	root := robustTempDir(t)
	if err := addProject(root, ""); err != nil {
		t.Fatal(err)
	}
	projectActor := app.actorForWorkspace(root)
	path, err := createEmptySessionFile(desktopSessionDirForActor(projectActor, root), "kwu/stale-model")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.EnsureBranchMeta(path); err != nil {
		t.Fatal(err)
	}

	tab := &WorkspaceTab{
		ID:            "broken-blank-project",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		SessionPath:   path,
		actor:         actorContextForWorkspace(projectActor, globalTabWorkspaceRoot()),
		sink:          &tabEventSink{tabID: "broken-blank-project", app: app},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.reconcileTabWithPinnedSessionMeta(tab)
	if tab.Scope != "project" || normalizeProjectRoot(tab.WorkspaceRoot) != normalizeProjectRoot(root) {
		t.Fatalf("repaired blank route = %q/%q, want project/%q", tab.Scope, tab.WorkspaceRoot, root)
	}
	persisted := app.loadTabsFile()
	if len(persisted.Tabs) != 1 || persisted.Tabs[0].Scope != "project" ||
		normalizeProjectRoot(persisted.Tabs[0].WorkspaceRoot) != normalizeProjectRoot(root) {
		t.Fatalf("persisted repaired route = %+v, want project/%q", persisted.Tabs, root)
	}
	history, err := app.HistoryForTab(tab.ID)
	if err != nil || len(history) != 0 {
		t.Fatalf("repaired blank history = %+v, %v; want empty history", history, err)
	}
}

func TestHistoryForTabDiskErrorsDoNotCreateBlankSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(dir, "corrupt.jsonl")
	if err := os.WriteFile(corrupt, []byte("{not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(dir, "pending.jsonl")
	writeHistoryTestSession(t, pending, "pending")
	if err := agent.MarkCleanupPending(pending, "delete"); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.jsonl")
	writeHistoryTestSession(t, outside, "outside")

	tests := []struct {
		name string
		path string
	}{
		{name: "missing", path: filepath.Join(dir, "missing.jsonl")},
		{name: "corrupt", path: corrupt},
		{name: "outside", path: outside},
		{name: "cleanup pending", path: pending},
	}
	escapeTarget := filepath.Join(t.TempDir(), "escape-target.jsonl")
	writeHistoryTestSession(t, escapeTarget, "escape")
	escapeLink := filepath.Join(dir, "escape.jsonl")
	if err := os.Symlink(escapeTarget, escapeLink); err == nil {
		tests = append(tests, struct {
			name string
			path string
		}{name: "symlink escape", path: escapeLink})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, tab := newControllerlessHistoryTab(root, test.path)
			if history, err := app.HistoryForTab(tab.ID); err == nil || history != nil {
				t.Fatalf("HistoryForTab() = %#v, %v; want disk error", history, err)
			}
			if tab.Ctrl != nil || tab.SessionPath != test.path {
				t.Fatalf("failed history changed tab to ctrl:%T path:%q", tab.Ctrl, tab.SessionPath)
			}
		})
	}
}

func TestHistoryForTabReadsDefaultProfileLegacyGlobalPath(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "legacy-global.jsonl")
	writeHistoryTestSession(t, path, "legacy global prompt")
	app, tab := newControllerlessHistoryTab(globalTabWorkspaceRoot(), path)
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) != 1 || history[0].Content != "legacy global prompt" {
		t.Fatalf("legacy global history = %+v", history)
	}
}

func TestResumeAndOpenChannelBindDiskHistoryWhenControllerBuildFails(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte("[[invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	resumePath := filepath.Join(dir, "resume.jsonl")
	channelPath := filepath.Join(dir, "channel.jsonl")
	writeHistoryTestSession(t, resumePath, "resume from disk")
	writeHistoryTestSession(t, channelPath, "channel from disk")

	app, tab := newControllerlessHistoryTab(root, "")
	tab.billingSource = "newapi_byok"
	history, err := app.ResumeSessionForTab(tab.ID, resumePath)
	if err != nil || len(history) != 1 || history[0].Content != "resume from disk" {
		t.Fatalf("ResumeSessionForTab() = %+v, %v", history, err)
	}
	if tab.Ctrl != nil || filepath.Clean(tab.SessionPath) != filepath.Clean(resumePath) || tab.StartupErr == "" {
		t.Fatalf("resume build failure tab = ctrl:%T path:%q startup:%q", tab.Ctrl, tab.SessionPath, tab.StartupErr)
	}

	history, err = app.OpenChannelSessionForTab(tab.ID, channelPath)
	if err != nil || len(history) != 1 || history[0].Content != "channel from disk" {
		t.Fatalf("OpenChannelSessionForTab() = %+v, %v", history, err)
	}
	if tab.Ctrl != nil || filepath.Clean(tab.SessionPath) != filepath.Clean(channelPath) || !tab.ReadOnly {
		t.Fatalf("channel build failure tab = ctrl:%T path:%q readOnly:%t", tab.Ctrl, tab.SessionPath, tab.ReadOnly)
	}
}

func TestControllerStartupFailuresKeepPreloadedSessionHistory(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "network failure", err: errors.New("network unavailable")},
		{name: "controller build failure", err: errors.New("controller construction failed")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			configureDesktopTestBYOKRoute(t)
			root := globalTabWorkspaceRoot()
			dir := desktopSessionDir(root)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "startup-failure.jsonl")
			writeHistoryTestSession(t, path, "history before controller")
			app, tab := newControllerlessHistoryTab(root, path)
			tab.billingSource = "newapi_byok"
			app.tabControllerBuilder = func(context.Context, boot.Options, userctx.ActorContext, string, string, string) (*control.Controller, error) {
				return nil, test.err
			}

			app.buildTabController(tab)
			if tab.Ctrl != nil || !tab.Ready || !strings.Contains(tab.StartupErr, test.err.Error()) || filepath.Clean(tab.SessionPath) != filepath.Clean(path) {
				t.Fatalf("startup failure tab = ctrl:%T ready:%t path:%q error:%q", tab.Ctrl, tab.Ready, tab.SessionPath, tab.StartupErr)
			}
			history := mustHistoryForTab(t, app, tab.ID)
			if len(history) != 1 || history[0].Content != "history before controller" {
				t.Fatalf("startup failure history = %+v", history)
			}
		})
	}
}

func TestControllerConfigFailureClearsAuthenticationPendingAndPersistsSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config-failure.jsonl")
	writeHistoryTestSession(t, path, "history before config failure")
	app, tab := newControllerlessHistoryTab(root, path)
	tab.billingSource = string(modelbilling.BillingSubscriptionPool)
	tab.subscriptionAuthPending = true
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte("[[invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	app.buildTabController(tab)
	if tab.Ctrl != nil || !tab.Ready || tab.subscriptionAuthPending || tab.StartupErr == "" {
		t.Fatalf("config failure tab = ctrl:%T ready:%t pending:%t error:%q", tab.Ctrl, tab.Ready, tab.subscriptionAuthPending, tab.StartupErr)
	}
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) != 1 || history[0].Content != "history before config failure" {
		t.Fatalf("config failure history = %+v", history)
	}
	persisted := app.loadTabsFile()
	if len(persisted.Tabs) != 1 || filepath.Clean(persisted.Tabs[0].SessionPath) != filepath.Clean(path) {
		t.Fatalf("persisted config failure tabs = %+v, want session %q", persisted.Tabs, path)
	}
}

func TestPersistentTabRestorePreloadsPinnedSessionBeforeController(t *testing.T) {
	for _, test := range []struct {
		name    string
		corrupt bool
	}{
		{name: "valid"},
		{name: "corrupt", corrupt: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			app := NewApp()
			configureDesktopTestBYOKRoute(t)
			root := globalTabWorkspaceRoot()
			dir := desktopSessionDir(root)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "restored.jsonl")
			if test.corrupt {
				if err := os.WriteFile(path, []byte("{broken\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				writeHistoryTestSession(t, path, "restored disk history")
			}
			state := desktopTabsFile{
				Tabs: []desktopTabEntry{{
					ID: "restored-tab", Scope: "global", WorkspaceRoot: root, TopicID: "restored-topic",
					SessionPath: path, Model: desktopTestBYOKModelRef, BillingSource: desktopTestBYOKBillingSource,
				}},
				ActiveTab: "restored-tab",
			}
			encoded, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(app.desktopConfigDir(), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(app.desktopConfigDir(), tabsFileName), encoded, 0o644); err != nil {
				t.Fatal(err)
			}
			buildCalls := 0
			app.tabControllerBuilder = func(context.Context, boot.Options, userctx.ActorContext, string, string, string) (*control.Controller, error) {
				buildCalls++
				return nil, errors.New("controller unavailable during restore")
			}

			app.restoreOrBuildTabs()
			tab := app.tabByID("restored-tab")
			if tab == nil || tab.Ctrl != nil || !tab.Ready || filepath.Clean(tab.SessionPath) != filepath.Clean(path) || tab.StartupErr == "" {
				t.Fatalf("restored tab = %+v", tab)
			}
			if test.corrupt {
				if buildCalls != 0 {
					t.Fatalf("corrupt restore reached controller builder %d times", buildCalls)
				}
				if history, err := app.HistoryForTab(tab.ID); err == nil || history != nil {
					t.Fatalf("corrupt restored history = %+v, %v; want error", history, err)
				}
				return
			}
			if buildCalls != 1 {
				t.Fatalf("valid restore controller build calls = %d, want 1", buildCalls)
			}
			history := mustHistoryForTab(t, app, tab.ID)
			if len(history) != 1 || history[0].Content != "restored disk history" {
				t.Fatalf("valid restored history = %+v", history)
			}
		})
	}
}

func TestPersistentTabRestorePublishesSessionTurnsBeforeControllerReady(t *testing.T) {
	for _, test := range []struct {
		name      string
		write     func(*testing.T, string)
		wantTurns *int
	}{
		{
			name: "zero turn",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantTurns: intPointer(0),
		},
		{
			name: "legacy nonzero",
			write: func(t *testing.T, path string) {
				t.Helper()
				writeHistoryTestSession(t, path, "restored legacy history")
			},
			wantTurns: intPointer(1),
		},
		{
			name: "corrupt remains unknown",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{broken\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			app := NewApp()
			configureDesktopTestBYOKRoute(t)
			root := globalTabWorkspaceRoot()
			dir := desktopSessionDir(root)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "restored.jsonl")
			test.write(t, path)
			state := desktopTabsFile{
				Tabs: []desktopTabEntry{{
					ID: "restored-tab", Scope: "global", WorkspaceRoot: root, TopicID: "restored-topic",
					SessionPath: path, Model: desktopTestBYOKModelRef, BillingSource: desktopTestBYOKBillingSource,
				}},
				ActiveTab: "restored-tab",
			}
			encoded, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(app.desktopConfigDir(), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(app.desktopConfigDir(), tabsFileName), encoded, 0o644); err != nil {
				t.Fatal(err)
			}

			oldLoader := preloadTabSessionFile
			loads := 0
			preloadTabSessionFile = func(path string) (*agent.Session, error) {
				loads++
				return oldLoader(path)
			}
			t.Cleanup(func() { preloadTabSessionFile = oldLoader })

			buildStarted := make(chan struct{})
			releaseBuild := make(chan struct{})
			app.tabControllerBuilder = func(context.Context, boot.Options, userctx.ActorContext, string, string, string) (*control.Controller, error) {
				close(buildStarted)
				<-releaseBuild
				return nil, errors.New("controller unavailable during restore")
			}
			restoreDone := make(chan struct{})
			go func() {
				app.restoreOrBuildTabs()
				close(restoreDone)
			}()

			if test.wantTurns == nil {
				select {
				case <-restoreDone:
				case <-time.After(5 * time.Second):
					t.Fatal("corrupt restore did not finish")
				}
			} else {
				select {
				case <-buildStarted:
				case <-time.After(5 * time.Second):
					t.Fatal("controller build did not reach gate")
				}
			}

			tabs := app.ListTabs()
			if len(tabs) != 1 || tabs[0].ID != "restored-tab" {
				t.Fatalf("restored tabs = %+v", tabs)
			}
			if tabs[0].ModelRef != desktopTestBYOKModelRef {
				t.Fatalf("restored modelRef = %q, want %q before controller ready", tabs[0].ModelRef, desktopTestBYOKModelRef)
			}
			app.ListTabs()
			app.ListTabs()
			if test.wantTurns == nil {
				if tabs[0].SessionTurns != nil {
					t.Fatalf("corrupt restored turns = %v, want missing", *tabs[0].SessionTurns)
				}
			} else {
				if tabs[0].Ready {
					t.Fatalf("restored tab became ready before gated controller build: %+v", tabs[0])
				}
				if tabs[0].SessionTurns == nil || *tabs[0].SessionTurns != *test.wantTurns {
					t.Fatalf("restored turns = %v, want %d", tabs[0].SessionTurns, *test.wantTurns)
				}
				payload, err := json.Marshal(tabs[0])
				if err != nil {
					t.Fatal(err)
				}
				if *test.wantTurns == 0 && !strings.Contains(string(payload), `"sessionTurns":0`) {
					t.Fatalf("zero-turn restored tab JSON = %s, want explicit sessionTurns=0", payload)
				}
				close(releaseBuild)
				select {
				case <-restoreDone:
				case <-time.After(5 * time.Second):
					t.Fatal("gated restore did not finish")
				}
			}
			if loads != 1 {
				t.Fatalf("restored session loads = %d, want one reused preload", loads)
			}
		})
	}
}

func TestActivateSessionForTabDoesNotWaitForFullTranscriptDecode(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	app := NewApp()
	tab := &WorkspaceTab{
		ID: "tab", Scope: "global", WorkspaceRoot: root, Ready: true,
		actor: userctx.Default(root), sink: &tabEventSink{tabID: "tab", app: app},
		disabledMCP: map[string]ServerView{},
	}
	dir := tabSessionDir(tab)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(dir, "current.jsonl")
	targetPath := filepath.Join(dir, "target.jsonl")
	writeHistoryTestSession(t, currentPath, "current prompt")
	writeHistoryTestSession(t, targetPath, "target prompt")
	tab.SessionPath = currentPath
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	oldLoader := preloadTabSessionFile
	loadStarted := make(chan struct{})
	releaseLoad := make(chan struct{})
	preloadTabSessionFile = func(path string) (*agent.Session, error) {
		close(loadStarted)
		<-releaseLoad
		return oldLoader(path)
	}
	t.Cleanup(func() { preloadTabSessionFile = oldLoader })

	if err := app.ActivateSessionForTab(tab.ID, targetPath); err != nil {
		t.Fatalf("ActivateSessionForTab: %v", err)
	}
	select {
	case <-loadStarted:
	case <-time.After(15 * time.Second):
		t.Fatalf(
			"background transcript decode did not start: path=%q target=%q ready=%t generation=%d startup=%q",
			tab.currentSessionPath(), targetPath, tab.Ready, tab.buildGeneration, tab.StartupErr,
		)
	}
	if got := tab.currentSessionPath(); got != targetPath {
		t.Fatalf("tab session path = %q, want %q", got, targetPath)
	}
	if tab.Ready {
		t.Fatal("tab became ready before the gated transcript decode completed")
	}

	page, err := app.HistoryPageForTab(tab.ID, 0, 200, "")
	if err != nil {
		t.Fatalf("HistoryPageForTab while controller loads: %v", err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Content != "target prompt" {
		t.Fatalf("paged history = %+v, want target prompt", page.Messages)
	}
	close(releaseLoad)
	waitForTabReady(t, app, tab.ID)
}

func TestPersistentTabRestoreReplacesInvalidSessionPathWithBlank(t *testing.T) {
	for _, test := range []struct {
		name        string
		persistedAt func(*testing.T, string, string) (string, []byte)
	}{
		{
			name: "missing",
			persistedAt: func(_ *testing.T, dir, _ string) (string, []byte) {
				return filepath.Join(dir, "missing.jsonl"), nil
			},
		},
		{
			name: "outside session dir",
			persistedAt: func(t *testing.T, _ string, topicID string) (string, []byte) {
				outsideDir := t.TempDir()
				path := writeTopicSessionWithPrompt(t, outsideDir, "outside.jsonl", topicID, "Outside", "", "outside history", time.Now())
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return path, body
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			app := NewApp()
			configureDesktopTestBYOKRoute(t)
			root := globalTabWorkspaceRoot()
			dir := desktopSessionDir(root)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			const topicID = "restored-invalid-topic"
			fallback := writeTopicSessionWithPrompt(t, dir, "same-topic.jsonl", topicID, "Same topic", "", "must not be restored", time.Now().Add(-time.Hour))
			persistedPath, originalBody := test.persistedAt(t, dir, topicID)
			state := desktopTabsFile{
				Tabs: []desktopTabEntry{{
					ID: "restored-tab", Scope: "global", WorkspaceRoot: root, TopicID: topicID,
					SessionPath: persistedPath, Model: desktopTestBYOKModelRef, BillingSource: desktopTestBYOKBillingSource, ReadOnly: true,
				}},
				ActiveTab: "restored-tab",
			}
			encoded, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(app.desktopConfigDir(), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(app.desktopConfigDir(), tabsFileName), encoded, 0o644); err != nil {
				t.Fatal(err)
			}

			app.restoreOrBuildTabs()
			tab := waitForTabReady(t, app, "restored-tab")
			if filepath.Clean(tab.SessionPath) == filepath.Clean(persistedPath) || filepath.Clean(tab.SessionPath) == filepath.Clean(fallback) {
				t.Fatalf("restored invalid path = %q, want a new blank session", tab.SessionPath)
			}
			if tab.ReadOnly {
				t.Fatal("replacement blank session remained read-only")
			}
			if _, validPath, err := resolveTabSessionPath(tab, tab.SessionPath); err != nil || filepath.Clean(validPath) != filepath.Clean(tab.SessionPath) {
				t.Fatalf("replacement path = %q, %v", validPath, err)
			}
			history, err := app.HistoryForTab(tab.ID)
			if err != nil {
				t.Fatalf("restored blank history = %+v, %v", history, err)
			}
			for _, message := range history {
				if message.Role != "system" {
					t.Fatalf("restored blank history contains conversation: %+v", history)
				}
			}
			persisted := app.loadTabsFile()
			if len(persisted.Tabs) != 1 || filepath.Clean(persisted.Tabs[0].SessionPath) != filepath.Clean(tab.SessionPath) {
				t.Fatalf("persisted replacement tabs = %+v", persisted.Tabs)
			}
			app.mu.RLock()
			tabCount := len(app.tabRuntimes)
			app.mu.RUnlock()
			if tabCount != 1 {
				t.Fatalf("restored tab count = %d, want 1", tabCount)
			}
			if originalBody == nil {
				if _, err := os.Stat(persistedPath); !os.IsNotExist(err) {
					t.Fatalf("missing source path was created: %v", err)
				}
			} else if after, err := os.ReadFile(persistedPath); err != nil || string(after) != string(originalBody) {
				t.Fatalf("outside source changed: %q, %v", after, err)
			}
		})
	}
}

func TestExplicitSessionOpenRejectsMissingPathWithoutChangingTab(t *testing.T) {
	for _, test := range []struct {
		name     string
		readOnly bool
		open     func(*App, string, string) ([]HistoryMessage, error)
	}{
		{
			name:     "resume",
			readOnly: true,
			open: func(app *App, tabID, path string) ([]HistoryMessage, error) {
				return app.ResumeSessionForTab(tabID, path)
			},
		},
		{
			name: "open channel",
			open: func(app *App, tabID, path string) ([]HistoryMessage, error) {
				return app.OpenChannelSessionForTab(tabID, path)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			root := globalTabWorkspaceRoot()
			dir := desktopSessionDir(root)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			currentPath := filepath.Join(dir, "current.jsonl")
			writeHistoryTestSession(t, currentPath, "current history")
			app, tab := newControllerlessHistoryTab(root, currentPath)
			ctrl := control.New(control.Options{SessionDir: dir, SessionPath: currentPath})
			defer ctrl.Close()
			tab.Ctrl = ctrl
			tab.ReadOnly = test.readOnly

			missing := filepath.Join(dir, "missing.jsonl")
			if history, err := test.open(app, tab.ID, missing); err == nil || history != nil {
				t.Fatalf("explicit missing open = %+v, %v; want error", history, err)
			}
			if tab.Ctrl != ctrl || filepath.Clean(tab.SessionPath) != filepath.Clean(currentPath) || tab.ReadOnly != test.readOnly {
				t.Fatalf("tab changed after rejected open: ctrl:%t path:%q readOnly:%t", tab.Ctrl == ctrl, tab.SessionPath, tab.ReadOnly)
			}
		})
	}
}

func TestActivateTopicRejectsCorruptSessionWithoutReplacingCurrentTab(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(dir, "current.jsonl")
	corruptPath := filepath.Join(dir, "corrupt-open.jsonl")
	writeHistoryTestSession(t, currentPath, "current")
	if err := os.WriteFile(corruptPath, []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, tab := newControllerlessHistoryTab(root, currentPath)
	tab.TopicID = "current-topic"
	if _, err := app.ActivateTopic("global", "", "broken-topic", corruptPath); err == nil {
		t.Fatal("ActivateTopic() corrupt session error = nil")
	}
	if app.cachedActiveTabID() != tab.ID || len(app.tabRuntimes) != 1 || filepath.Clean(tab.SessionPath) != filepath.Clean(currentPath) {
		t.Fatalf("corrupt activation replaced current tab: active=%q tabs=%d path=%q", app.activeTabID, len(app.tabRuntimes), tab.SessionPath)
	}
}

func TestStaleBuildCannotOverwriteLaterSessionSelection(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pathA := filepath.Join(dir, "a.jsonl")
	pathB := filepath.Join(dir, "b.jsonl")
	writeHistoryTestSession(t, pathA, "history A")
	writeHistoryTestSession(t, pathB, "history B")
	app, tab := newControllerlessHistoryTab(root, pathA)
	tab.buildGeneration = 1

	allowA := make(chan struct{})
	aResult := make(chan bool, 1)
	go func() {
		<-allowA
		aResult <- app.persistTabSessionPathForBuild(tab, pathA, 1)
	}()
	preloadedB, err := app.preloadTabSession(tab, pathB)
	if err != nil {
		t.Fatal(err)
	}
	app.mu.Lock()
	tab.SessionPath = preloadedB.Path
	tab.buildGeneration = 2
	app.mu.Unlock()
	close(allowA)
	if <-aResult {
		t.Fatal("stale A build persisted its session path")
	}
	history := mustHistoryForTab(t, app, tab.ID)
	if filepath.Clean(tab.SessionPath) != filepath.Clean(pathB) || len(history) != 1 || history[0].Content != "history B" {
		t.Fatalf("final selection = path:%q history:%+v, want B", tab.SessionPath, history)
	}
}

func newControllerlessHistoryTab(root, sessionPath string) (*App, *WorkspaceTab) {
	app := &App{tabRuntimes: map[string]*WorkspaceTab{}, detachedSessions: map[string]*WorkspaceTab{}}
	tab := &WorkspaceTab{
		ID:            "history-tab",
		Scope:         "global",
		WorkspaceRoot: root,
		SessionPath:   sessionPath,
		Ready:         true,
		sink:          &tabEventSink{tabID: "history-tab", app: app},
		disabledMCP:   map[string]ServerView{},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	return app, tab
}
