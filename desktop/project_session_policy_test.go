package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/project"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

func installWorkBuddyProjectSessionTab(t *testing.T, app *App, workspace string) (*control.Controller, string) {
	controller, path, _ := installWorkBuddyProjectSessionTabWithSession(t, app, workspace)
	return controller, path
}

func installWorkBuddyProjectSessionTabWithSession(t *testing.T, app *App, workspace string) (*control.Controller, string, *agent.Session) {
	t.Helper()
	stateRoot := t.TempDir()
	t.Setenv("HOME", stateRoot)
	t.Setenv("USERPROFILE", stateRoot)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(stateRoot, ".config"))
	t.Setenv("APPDATA", filepath.Join(stateRoot, "AppData"))
	sessionDir := t.TempDir()
	path := filepath.Join(sessionDir, "project-session.jsonl")
	session := agent.NewSession("system")
	executor := agent.New(nil, nil, session, agent.Options{}, event.Discard)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	controller := control.New(control.Options{
		Executor: executor, SessionDir: sessionDir, SessionPath: path,
		WorkspaceRoot: workspace, Actor: actor, Sink: event.Discard,
	})
	tab := &WorkspaceTab{ID: "project", Scope: "project", WorkspaceRoot: workspace, SessionPath: path, Ctrl: controller, Ready: true, actor: actor}
	app.tabRuntimes = map[string]*WorkspaceTab{"project": tab}
	app.activeTabID = tab.ID
	return controller, path, session
}

func TestProjectSessionTabMatchesBindingUsesDesktopPathSemantics(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "Workspace")
	equivalent := workspace
	if runtime.GOOS == "windows" {
		equivalent = strings.ToUpper(workspace)
	}
	tab := &WorkspaceTab{Scope: "project", WorkspaceRoot: workspace}
	if !projectSessionTabMatchesBinding(tab, true, equivalent) {
		t.Fatalf("equivalent desktop paths did not match: %q / %q", workspace, equivalent)
	}
	if projectSessionTabMatchesBinding(tab, false, equivalent) {
		t.Fatal("project tab matched a global project binding")
	}
}

func managedWorkBuddyProjectWorkspace(t *testing.T, state *workBuddyProjectAppState, projectID string) string {
	t.Helper()
	workspace, err := ensureCloudProjectWorkspace(context.Background(), state.services, projectID)
	if err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestWorkBuddyProjectNewSessionUsesDedicatedWorkspace(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := managedWorkBuddyProjectWorkspace(t, state, "project-1")
	otherWorkspace := managedWorkBuddyProjectWorkspace(t, state, "another-project")
	if sameDesktopPath(workspace, otherWorkspace) {
		t.Fatalf("projects share dedicated workspace %q", workspace)
	}
	_, path := installWorkBuddyProjectSessionTab(t, app, workspace)

	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{
		ProjectID:       "project-1",
		TabID:           "project",
		WorkspacePath:   workspace,
		ExecutionTarget: "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.SessionPath != path || view.ProjectID != "project-1" {
		t.Fatalf("selected-workspace project session = %#v", view)
	}
	bound, found, err := state.services.Bindings.Get(context.Background(), state.scope, "project-1")
	if err != nil || !found || bound.Status != project.LocalWorkspaceAvailable || !sameDesktopPath(bound.CanonicalPath, workspace) {
		t.Fatalf("selected workspace binding = %#v, %v, %v", bound, found, err)
	}
	bindings, err := state.services.Bindings.List(context.Background(), state.scope)
	if err != nil || len(bindings) != 2 {
		t.Fatalf("dedicated workspace bindings = %#v, %v", bindings, err)
	}
}

func TestCloudProjectFirstTurnRefreshesListingAndTreeCache(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	_, path, session := installWorkBuddyProjectSessionTabWithSession(t, app, "")
	app.tabRuntimes["project"].Scope = "global"
	app.tabRuntimes["project"].WorkspaceRoot = globalWorkspaceRoot()
	app.projectTreeChangedHook = func() {}

	if _, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{
		ProjectID: "project-1", TabID: "project", ExecutionTarget: "cloud",
	}); err != nil {
		t.Fatal(err)
	}
	// Reproduce the production ordering: binding refreshes the tree while the
	// transcript contains only the hidden project context, caching no session.
	dir := filepath.Dir(path)
	cacheToken := projectSessionCache.versionToken()
	projectSessionCache.put(dir, nil, nil, nil, nil, cacheToken)
	session.Add(provider.Message{Role: provider.RoleUser, Content: "first project turn"})
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "done"})

	var treeEvents int
	app.projectTreeChangedHook = func() { treeEvents++ }
	sink := &tabEventSink{tabID: "project", app: app}
	sink.recordTurnDone()

	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok || meta.Turns != 1 || meta.Preview != "first project turn" {
		t.Fatalf("first-turn listing meta = %#v, %v, %v", meta, ok, err)
	}
	if meta.ProjectContext == nil || meta.ProjectContext.ProjectID != "project-1" || !projectContextPinMatchesScope(*meta.ProjectContext, state.scope) {
		t.Fatalf("first-turn project binding = %#v", meta.ProjectContext)
	}
	if treeEvents != 1 {
		t.Fatalf("project tree events = %d, want 1", treeEvents)
	}
	if _, _, _, _, cached := projectSessionCache.get(dir); cached {
		t.Fatal("stale empty project session listing remained cached")
	}
}

func TestCloudProjectPendingFirstTurnStaysUnderCloudProject(t *testing.T) {
	app, _ := newWorkBuddyProjectApp(t, nil)
	_, path, session := installWorkBuddyProjectSessionTabWithSession(t, app, "")
	app.projectTreeChangedHook = func() {}
	topic, err := app.CreateTopic("global", "", "Project prompt")
	if err != nil {
		t.Fatal(err)
	}
	tab := app.tabRuntimes["project"]
	tab.Scope = "global"
	tab.WorkspaceRoot = globalWorkspaceRoot()
	tab.TopicID = topic.ID
	tab.TopicTitle = topic.Title

	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{
		ProjectID: "project-1", TabID: "project", ExecutionTarget: "cloud",
	})
	if err != nil {
		t.Fatal(err)
	}
	path = view.SessionPath
	projectSessionCache.put(filepath.Dir(path), nil, nil, nil, nil, projectSessionCache.versionToken())
	session.Add(provider.Message{Role: provider.RoleUser, Content: "first project turn"})

	var cloudSessionFound bool
	for _, node := range app.ListProjectTree() {
		if node.Kind == "global_folder" {
			for _, child := range node.Children {
				if child.TopicID == topic.ID || child.SessionPath == path {
					t.Fatalf("pending Cloud Project session leaked into Tasks: %#v", child)
				}
				for _, grandchild := range child.Children {
					if grandchild.SessionPath == path {
						t.Fatalf("pending Cloud Project session leaked into Tasks: %#v", grandchild)
					}
				}
			}
		}
		if node.Kind == "cloud_project" && node.CloudProjectID == "project-1" {
			for _, child := range node.Children {
				if child.SessionPath == path && child.CloudProjectID == "project-1" {
					cloudSessionFound = true
				}
			}
		}
	}
	if !cloudSessionFound {
		t.Fatalf("pending session %q was not classified under cloud project", path)
	}
}

func TestCloudProjectNewSessionIsImmediatelyFirstBeforeFirstTurn(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	_, newPath, _ := installWorkBuddyProjectSessionTabWithSession(t, app, "")
	tab := app.tabRuntimes["project"]
	tab.Scope = "global"
	tab.WorkspaceRoot = globalWorkspaceRoot()
	topic, err := app.CreateTopic("global", "", "新的会话")
	if err != nil {
		t.Fatal(err)
	}
	tab.TopicID = topic.ID
	tab.TopicTitle = topic.Title

	oldPath := filepath.Join(filepath.Dir(newPath), "older-cloud-project-session.jsonl")
	if err := os.WriteFile(oldPath, []byte(`{"role":"user","content":"旧会话"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldAt := time.Now().UTC().Add(-time.Hour)
	if err := agent.SaveBranchMetaPreserveUpdated(oldPath, agent.BranchMeta{
		CreatedAt: oldAt, UpdatedAt: oldAt, Scope: "global", TopicID: "older-cloud-topic", TopicTitle: "旧会话",
		Turns: 1, Preview: "旧会话", SchemaVersion: agent.BranchMetaCountsVersion,
		ProjectContext: &agent.ProjectContextPin{
			Environment:  agent.ProjectEnvironmentIdentity{EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL},
			BindingScope: agent.ProjectBindingScope{ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration},
			ProjectID:    "project-1", InstructionVersion: 1, ContentHash: strings.Repeat("c", 64),
		},
	}); err != nil {
		t.Fatal(err)
	}

	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project", ExecutionTarget: "cloud"})
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range app.ListProjectTree() {
		if node.Kind != "cloud_project" || node.CloudProjectID != "project-1" {
			continue
		}
		if len(node.Children) < 2 {
			t.Fatalf("cloud project children = %#v, want new and old sessions", node.Children)
		}
		if node.Children[0].SessionPath != view.SessionPath {
			t.Fatalf("first cloud project session = %#v, want newly created %q", node.Children[0], view.SessionPath)
		}
		if node.Children[0].LastActivityAt == 0 {
			t.Fatal("new cloud project session has no immediate sorting timestamp")
		}
		return
	}
	t.Fatal("cloud project node not found")
}

func TestCloudAutomationSessionStartsUnderCloudProjectBeforeContextCommit(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.ownedListResponse = `{"data":{"items":[{"project_id":"project-1","owner_user_id":"usr_project","name":"Alpha","description":null,"status":"active","version":1,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"}],"next_cursor":null},"error":null,"request_id":"req-list-project-1","server_time":"2026-08-05T04:00:02Z"}`
	if _, err := app.ListCloudProjects(CloudProjectListInput{}); err != nil {
		t.Fatal(err)
	}
	_, path, _ := installWorkBuddyProjectSessionTabWithSession(t, app, "")

	var treeEvents int
	app.projectTreeChangedHook = func() { treeEvents++ }
	topic, err := app.createTopic("global", "", "每日办公软件技巧", false)
	if err != nil {
		t.Fatal(err)
	}
	if treeEvents != 0 {
		t.Fatalf("cloud automation topic published before project identity: %d events", treeEvents)
	}
	tab := app.tabRuntimes["project"]
	tab.Scope = "global"
	tab.WorkspaceRoot = globalWorkspaceRoot()
	tab.TopicID = topic.ID
	tab.TopicTitle = topic.Title
	tab.pendingCloudProjectID = "project-1"
	app.emitProjectTreeChanged()

	if meta, ok, err := agent.LoadBranchMeta(path); err != nil || (ok && meta.ProjectContext != nil) {
		t.Fatalf("pre-commit branch metadata = %#v, %v, %v", meta, ok, err)
	}
	var cloudSessionFound bool
	for _, node := range app.ListProjectTree() {
		if node.Kind == "global_folder" {
			for _, child := range node.Children {
				if child.TopicID == topic.ID || child.SessionPath == path {
					t.Fatalf("pre-commit automation session leaked into Tasks: %#v", child)
				}
			}
		}
		if node.Kind == "cloud_project" && node.CloudProjectID == "project-1" {
			for _, child := range node.Children {
				if child.SessionPath == path {
					cloudSessionFound = true
					if child.Label != "每日办公软件技巧" {
						t.Fatalf("pre-commit automation label = %q, want task title", child.Label)
					}
				}
			}
		}
	}
	if !cloudSessionFound {
		t.Fatalf("pre-commit automation session %q was not classified under cloud project", path)
	}
}

func TestWorkBuddyProjectSessionPinPersistsAndRejectsAnotherAccount(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	app.projectTreeChangedHook = func() {}
	workspace := managedWorkBuddyProjectWorkspace(t, state, "project-1")
	_, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{
		ProjectID: "project-1", TabID: "project", WorkspacePath: workspace, ExecutionTarget: "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	path = view.SessionPath
	if err := agent.NewSession("system").Save(path); err != nil {
		t.Fatal(err)
	}
	if err := app.SetCloudProjectSessionPinned(path, true); err != nil {
		t.Fatalf("pin project session: %v", err)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok || !meta.ProjectSessionPinned {
		t.Fatalf("pinned branch meta = %#v, %v, %v", meta, ok, err)
	}
	nodes := []ProjectNode{
		{SessionPath: "recent.jsonl", LastActivityAt: 20},
		{SessionPath: "pinned.jsonl", LastActivityAt: 10, Pinned: meta.ProjectSessionPinned},
	}
	sortCloudProjectSessionNodes(nodes)
	if nodes[0].SessionPath != "pinned.jsonl" {
		t.Fatalf("cloud project session order = %#v", nodes)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := state.runtime.bindings.Rebind(context.Background(), actor, "usr_other"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetCloudProjectSessionPinned(path, false); err == nil {
		t.Fatal("cross-account project session unpin succeeded")
	}
	meta, _, err = agent.LoadBranchMeta(path)
	if err != nil || !meta.ProjectSessionPinned {
		t.Fatalf("rejected cross-account pin mutation changed meta = %#v, %v", meta, err)
	}
}

func TestDeleteLastCloudProjectSessionDoesNotReappearUnderGlobal(t *testing.T) {
	app, _ := newWorkBuddyProjectApp(t, nil)
	app.projectTreeChangedHook = func() {}
	app.ctx = nil
	controller, path, session := installWorkBuddyProjectSessionTabWithSession(t, app, "")
	app.tabRuntimes["project"].Scope = "global"
	app.tabRuntimes["project"].WorkspaceRoot = globalWorkspaceRoot()
	app.tabRuntimes["project"].TopicID = "topic-project-only"
	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project", ExecutionTarget: "cloud"})
	if err != nil {
		t.Fatal(err)
	}
	path = view.SessionPath
	session.Add(provider.Message{Role: provider.RoleUser, Content: "project-only topic"})
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "done"})
	if err := controller.Snapshot(); err != nil {
		t.Fatal(err)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok || meta.ProjectContext == nil || meta.TopicID == "" {
		t.Fatalf("project metadata = %#v, %v, %v", meta, ok, err)
	}
	if err := setTopicTitle("", meta.TopicID, "project-only topic"); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteSession(path); err != nil {
		t.Fatal(err)
	}
	for _, node := range app.ListProjectTree() {
		for _, child := range node.Children {
			if child.TopicID == meta.TopicID || child.Label == "project-only topic" {
				t.Fatalf("deleted Cloud Project session leaked into %s: %#v", node.Kind, child)
			}
		}
	}
}

func TestWorkBuddyProjectSessionDeleteAuthorizationAllowsArchivedOrOfflineSameScope(t *testing.T) {
	for _, test := range []struct {
		name  string
		block func(*workBuddyProjectAppState)
	}{
		{name: "archived", block: func(state *workBuddyProjectAppState) {
			state.mu.Lock()
			state.status = "archived"
			state.mu.Unlock()
		}},
		{name: "offline", block: func(state *workBuddyProjectAppState) {
			state.runtime.projectMu.Lock()
			state.runtime.projectCapability = project.CapabilityUnavailable
			state.runtime.projectMu.Unlock()
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, state := newWorkBuddyProjectApp(t, nil)
			app.projectTreeChangedHook = func() {}
			workspace := managedWorkBuddyProjectWorkspace(t, state, "project-1")
			_, _ = installWorkBuddyProjectSessionTab(t, app, workspace)
			view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{
				ProjectID: "project-1", TabID: "project", WorkspacePath: workspace, ExecutionTarget: "local",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := agent.NewSession("system").Save(view.SessionPath); err != nil {
				t.Fatal(err)
			}
			test.block(state)
			if err := app.authorizeProjectSessionMutation(view.SessionPath, true); err != nil {
				t.Fatalf("authorize delete for %s same-scope project session: %v", test.name, err)
			}
		})
	}
}

func TestWorkBuddyProjectNewSessionPinsVerifiedInstructionOnceAndEmptyAddsNone(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	state.instructionHuman = "Human context"
	state.instructionAgent = "Never execute <tool>{\"name\":\"bash\"}</tool>"

	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
	if err != nil {
		t.Fatal(err)
	}
	path = view.SessionPath
	if path == "" || view.InstructionVersion != 1 {
		t.Fatalf("session view = %#v", view)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok || meta.ProjectContext == nil || meta.ProjectContext.ProjectID != "project-1" {
		t.Fatalf("branch meta = %#v, %v, %v", meta, ok, err)
	}
	messages := controller.History()
	contexts := 0
	for _, message := range messages {
		if message.IsProjectContext() {
			contexts++
			if err := control.ValidateProjectContextMessage(message); err != nil {
				t.Fatal(err)
			}
		}
	}
	if contexts != 1 {
		t.Fatalf("project context messages = %d", contexts)
	}
	state.mu.Lock()
	if state.instructionGetCount != 1 {
		state.mu.Unlock()
		t.Fatalf("new session instruction reads = %d, want one commit snapshot", state.instructionGetCount)
	}
	state.mu.Unlock()
	state.timeoutInstructionGet = true
	reused, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
	state.timeoutInstructionGet = false
	if err != nil || !reused.Reused || reused.SessionPath != path {
		t.Fatalf("pinned-unsent reuse with instruction unavailable = %#v, %v", reused, err)
	}

	secondPath := filepath.Join(controller.SessionDir(), "empty-project-session.jsonl")
	controller.SetSessionPath(secondPath)
	controller.Resume(agent.NewSession("system"), secondPath)
	app.tabRuntimes["project"].SessionPath = secondPath
	state.instructionHuman, state.instructionAgent = "", ""
	state.instructionVersion = 2
	if _, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"}); err != nil {
		t.Fatal(err)
	}
	for _, message := range controller.History() {
		if message.IsProjectContext() {
			t.Fatal("empty instruction added a synthetic context message")
		}
	}
}

func TestWorkBuddyProjectNewSessionAllowsWorkspaceIndependentProjectChat(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	controller, path := installWorkBuddyProjectSessionTab(t, app, "")
	app.tabRuntimes["project"].Scope = "global"
	app.tabRuntimes["project"].WorkspaceRoot = globalWorkspaceRoot()
	state.instructionHuman = "Cloud project context"

	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if view.SessionPath != path || view.ProjectID != "project-1" {
		t.Fatalf("workspace-independent project session = %#v", view)
	}
	if inspection := agent.InspectProjectContext(controller.History()); inspection == nil || !strings.Contains(inspection.Body, "Cloud project context") {
		t.Fatalf("workspace-independent project context = %#v", inspection)
	}
}

func TestWorkBuddyProjectNewSessionCloudExecutionIgnoresLocalBinding(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	controller, path := installWorkBuddyProjectSessionTab(t, app, "")
	app.tabRuntimes["project"].Scope = "global"
	app.tabRuntimes["project"].WorkspaceRoot = globalWorkspaceRoot()
	state.instructionHuman = "Cloud project context"

	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{
		ProjectID: "project-1", TabID: "project", ExecutionTarget: "cloud",
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.SessionPath != path || view.ProjectID != "project-1" {
		t.Fatalf("cloud-execution project session = %#v", view)
	}
	if inspection := agent.InspectProjectContext(controller.History()); inspection == nil || !strings.Contains(inspection.Body, "Cloud project context") {
		t.Fatalf("cloud-execution project context = %#v", inspection)
	}
}

func TestWorkBuddyProjectNewSessionUsesCommitTimeInstructionRead(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	controller, _ := installWorkBuddyProjectSessionTab(t, app, workspace)
	state.instructionAgent = "preflight instruction"

	previousHook := newCloudProjectSessionBeforeCommit
	newCloudProjectSessionBeforeCommit = func() {
		state.mu.Lock()
		state.instructionAgent = "commit instruction"
		state.instructionVersion = 2
		state.mu.Unlock()
	}
	t.Cleanup(func() { newCloudProjectSessionBeforeCommit = previousHook })
	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
	if err != nil || view.InstructionVersion != 2 || view.ContentHash != project.InstructionContentSHA256("", "commit instruction") {
		t.Fatalf("commit instruction view = %#v, %v", view, err)
	}
	inspection := agent.InspectProjectContext(controller.History())
	if inspection == nil || !strings.Contains(inspection.Body, "commit instruction") || strings.Contains(inspection.Body, "preflight instruction") {
		t.Fatalf("pinned context = %#v", inspection)
	}
}

func TestWorkBuddyProjectNewSessionRejectsPinnedMetaTranscriptMismatch(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	otherContext, present, err := control.BuildProjectContextMessage(control.ProjectContextInput{
		ProjectID: "project-2", AgentInstruction: "other project secret", Version: 3,
		ContentHash: project.InstructionContentSHA256("", "other project secret"),
	})
	if err != nil || !present {
		t.Fatal(err)
	}
	controller.Resume(agent.NewSessionFromMessages([]provider.Message{{Role: provider.RoleSystem, Content: "sys"}, otherContext}), path)
	meta := agent.BranchMeta{ProjectContext: &agent.ProjectContextPin{
		Environment:  agent.ProjectEnvironmentIdentity{EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL},
		BindingScope: agent.ProjectBindingScope{ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration},
		ProjectID:    "project-1", InstructionVersion: 3, ContentHash: otherContext.Metadata.ContentHash,
	}}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"}); err == nil || !strings.Contains(err.Error(), "does not match metadata") {
		t.Fatalf("mismatched pinned transcript error = %v", err)
	}
}

func TestWorkBuddyProjectNewSessionUsesExplicitTabAndReusesConcurrentRetry(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	targetController, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	otherController := control.New(control.Options{
		SessionDir: t.TempDir(), SessionPath: filepath.Join(t.TempDir(), "other.jsonl"),
		WorkspaceRoot: t.TempDir(), Actor: userctx.ActorContext{ActorID: "local", ProfileID: "default"}, Sink: event.Discard,
	})
	t.Cleanup(otherController.Close)
	app.tabRuntimes["other"] = &WorkspaceTab{ID: "other", Scope: "project", WorkspaceRoot: t.TempDir(), Ctrl: otherController, Ready: true}
	app.activeTabID = "other"
	state.instructionHuman = "Pinned once"

	type result struct {
		view CloudProjectSessionView
		err  error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
			results <- result{view: view, err: err}
		}()
	}
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent create errors = %v / %v", first.err, second.err)
	}
	if first.view.TabID != "project" || second.view.TabID != "project" || first.view.SessionPath != path || second.view.SessionPath != path {
		t.Fatalf("explicit target results = %#v / %#v", first.view, second.view)
	}
	if first.view.Reused == second.view.Reused {
		t.Fatalf("expected exactly one idempotent reuse: %#v / %#v", first.view, second.view)
	}
	if app.cachedActiveTabID() != "other" || len(otherController.History()) != 0 {
		t.Fatalf("active tab was changed or populated: active=%q history=%#v", app.activeTabID, otherController.History())
	}
	contexts := 0
	for _, message := range targetController.History() {
		if message.IsProjectContext() {
			contexts++
		}
	}
	if contexts != 1 {
		t.Fatalf("project context messages = %d", contexts)
	}
}

func TestWorkBuddyProjectNewSessionRejectsTargetReplacementBeforeCommit(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	state.instructionHuman = "Must not leak to a replaced tab"

	previousHook := newCloudProjectSessionBeforeCommit
	newCloudProjectSessionBeforeCommit = func() {
		app.mu.Lock()
		app.tabRuntimes["project"] = &WorkspaceTab{ID: "project", Scope: "project", WorkspaceRoot: workspace, Ready: true}
		app.mu.Unlock()
	}
	t.Cleanup(func() { newCloudProjectSessionBeforeCommit = previousHook })

	_, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
	if err == nil || !strings.Contains(err.Error(), "changed before session creation") {
		t.Fatalf("replacement error = %v", err)
	}
	for _, message := range controller.History() {
		if message.IsProjectContext() {
			t.Fatal("replaced target received project context")
		}
	}
	if _, ok, loadErr := agent.LoadBranchMeta(path); loadErr != nil || ok {
		t.Fatalf("rejected replacement wrote metadata: ok=%v err=%v", ok, loadErr)
	}
}

func TestWorkBuddyProjectSessionPolicyHidesMismatchBlocksSendAndKeepsLocalEscape(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	contextMessage := desktopProjectContext("private policy context")
	controller.Resume(agent.NewSessionFromMessages([]provider.Message{{Role: provider.RoleSystem, Content: "system"}, contextMessage, {Role: provider.RoleUser, Content: "ordinary"}}), path)
	if err := controller.Snapshot(); err != nil {
		t.Fatal(err)
	}
	meta := agent.BranchMeta{ProjectContext: &agent.ProjectContextPin{
		Environment:  agent.ProjectEnvironmentIdentity{EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL},
		BindingScope: agent.ProjectBindingScope{ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration},
		ProjectID:    "project-1", InstructionVersion: 5, ContentHash: project.InstructionContentSHA256("", "private policy context"),
	}}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	app.invalidatePromptHistoryCache()
	if history, err := app.ScanPromptHistory(""); err != nil || len(history.Entries) != 1 || history.Entries[0].Text != "ordinary" {
		t.Fatalf("same-scope prompt history leaked typed context: %#v, %v", history, err)
	}
	if err := app.authorizeCloudProjectSessionSend(context.Background(), actor, path); err != nil {
		t.Fatalf("same-scope active send rejected: %v", err)
	}
	if err := app.retainProjectSessionAfterConfirmedAccountDeletion(path, *meta.ProjectContext); err == nil {
		t.Fatal("current live cloud identity was accepted as deleted")
	}
	state.runtime.projectMu.Lock()
	state.runtime.projectCapability = project.CapabilityUnavailable
	state.runtime.projectMu.Unlock()
	if visible, readOnly, reason := app.projectSessionVisible(context.Background(), actor, path); !visible || !readOnly || reason != projectSessionReasonOffline {
		t.Fatalf("same-scope offline disposition = %v/%v/%q", visible, readOnly, reason)
	}
	state.runtime.projectMu.Lock()
	state.runtime.projectCapability = project.CapabilityAvailable
	state.runtime.projectMu.Unlock()
	if err := app.authorizeCloudProjectSessionSend(context.Background(), actor, path); err != nil {
		t.Fatalf("original scope did not restore send: %v", err)
	}
	state.mu.Lock()
	state.status = "archived"
	state.mu.Unlock()
	if err := app.authorizeCloudProjectSessionSend(context.Background(), actor, path); err == nil || !strings.Contains(err.Error(), "archived") {
		t.Fatalf("archived send error = %v", err)
	}
	state.mu.Lock()
	state.status = "active"
	state.mu.Unlock()
	if _, err := state.runtime.bindings.Rebind(context.Background(), actor, "usr_other"); err != nil {
		t.Fatal(err)
	}
	if visible, _, _ := app.projectSessionListVisible(context.Background(), actor, path); visible {
		t.Fatal("account-switched session remained visible")
	}
	if err := app.authorizeCloudProjectSessionSend(context.Background(), actor, path); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("mismatched send error = %v", err)
	}
	if _, err := app.HistoryForTab("project"); err == nil {
		t.Fatal("mismatched scope exposed transcript body")
	}
	transcriptBefore, _ := os.ReadFile(path)
	metaBefore, _ := os.ReadFile(agent.BranchMetaPath(path))
	if err := app.DeleteSession(path); err == nil {
		t.Fatal("mismatched scope deleted a hidden session")
	}
	if err := app.RenameSession(path, "cross-account rename"); err == nil {
		t.Fatal("mismatched scope renamed a hidden session")
	}
	transcriptAfter, _ := os.ReadFile(path)
	metaAfter, _ := os.ReadFile(agent.BranchMetaPath(path))
	if !bytes.Equal(transcriptBefore, transcriptAfter) || !bytes.Equal(metaBefore, metaAfter) {
		t.Fatal("rejected cross-account mutation changed session artifacts")
	}
	if _, err := app.SubagentHistoryPageForTab("project", "child-ref", 0, 20, ""); err == nil || !strings.Contains(err.Error(), "current scope") {
		t.Fatalf("mismatched subagent history error = %v", err)
	}
	app.invalidatePromptHistoryCache()
	if history, err := app.ScanPromptHistory(""); err != nil || len(history.Entries) != 0 {
		t.Fatalf("mismatched scope prompt history = %#v, %v", history, err)
	}
	deleted := *meta.ProjectContext
	if err := app.retainProjectSessionAfterConfirmedAccountDeletion(path, deleted); err != nil {
		t.Fatal(err)
	}
	visible, readOnly, reason := app.projectSessionListVisible(context.Background(), actor, path)
	if !visible || !readOnly || reason != projectSessionReasonPermanent {
		t.Fatalf("retained local disposition = %v/%v/%q", visible, readOnly, reason)
	}
	if err := app.authorizeCloudProjectSessionSend(context.Background(), actor, path); err == nil || !strings.Contains(err.Error(), "permanently") {
		t.Fatalf("permanent send error = %v", err)
	}

	localPath := filepath.Join(t.TempDir(), "local.jsonl")
	if err := agent.SaveBranchMeta(localPath, agent.BranchMeta{Scope: "project", WorkspaceRoot: workspace}); err != nil {
		t.Fatal(err)
	}
	if err := app.authorizeCloudProjectSessionSend(context.Background(), actor, localPath); err != nil {
		t.Fatalf("legacy LocalWorkspace session was blocked: %v", err)
	}
}

func TestCloseCloudProjectTabsOutsideScopePreservesFilesAndLocalTabs(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}.Normalize()
	root := t.TempDir()
	cloudPath := filepath.Join(root, "cloud.jsonl")
	localPath := filepath.Join(root, "local.jsonl")
	if err := os.WriteFile(cloudPath, []byte(`{"role":"user","content":"private"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte(`{"role":"user","content":"local"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scopeA := project.Scope{
		Environment: cloud.EnvironmentIdentity{EnvironmentID: strings.Repeat("a", 64), CloudBaseURL: "https://cloud.example.test"},
		Binding:     cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "account-a", BindingGeneration: 1},
		Key:         strings.Repeat("b", 64),
	}
	if err := agent.SaveBranchMeta(cloudPath, agent.BranchMeta{
		Scope: "global", TopicID: "cloud-topic", ProjectContext: &agent.ProjectContextPin{
			Environment:        agent.ProjectEnvironmentIdentity{EnvironmentID: scopeA.Environment.EnvironmentID, CloudBaseURL: scopeA.Environment.CloudBaseURL},
			BindingScope:       agent.ProjectBindingScope{ProfileID: scopeA.Binding.ProfileID, CloudUserID: scopeA.Binding.CloudUserID, BindingGeneration: scopeA.Binding.BindingGeneration},
			ProjectID:          "project-a",
			InstructionVersion: 1,
			ContentHash:        strings.Repeat("c", 64),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(localPath, agent.BranchMeta{Scope: "global", TopicID: "local-topic"}); err != nil {
		t.Fatal(err)
	}
	metaBefore, err := os.ReadFile(agent.BranchMetaPath(cloudPath))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"cloud": {ID: "cloud", Scope: "global", SessionPath: cloudPath, Ready: true, actor: actor},
			"local": {ID: "local", Scope: "global", SessionPath: localPath, Ready: true, actor: actor},
		},
		tabOrder:    []string{"cloud", "local"},
		activeTabID: "cloud",
	}
	if !app.closeCloudProjectTabsOutsideScope(actor, nil) {
		t.Fatal("anonymous transition did not close the cloud project tab")
	}
	if app.tabRuntimes["cloud"] != nil || app.tabRuntimes["local"] == nil {
		t.Fatalf("tabs after anonymous transition = %#v", app.tabRuntimes)
	}
	if app.cachedActiveTabID() == "" || app.cachedActiveTabID() == "cloud" {
		t.Fatalf("anonymous transition active tab = %q, want safe fallback", app.activeTabID)
	}
	for _, path := range []string{cloudPath, localPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("account isolation removed transcript %q: %v", path, err)
		}
	}
	metaAfter, err := os.ReadFile(agent.BranchMetaPath(cloudPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(metaBefore, metaAfter) {
		t.Fatal("account isolation changed cloud branch metadata")
	}
}

func TestWorkBuddyProjectTrashHidesAndRejectsCrossAccountMutation(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	workspace := t.TempDir()
	controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	controller.Resume(agent.NewSessionFromMessages([]provider.Message{{Role: provider.RoleSystem, Content: "system"}, {Role: provider.RoleUser, Content: "private trash preview"}}), path)
	if err := controller.Snapshot(); err != nil {
		t.Fatal(err)
	}
	meta := agent.BranchMeta{ProjectContext: &agent.ProjectContextPin{
		Environment:  agent.ProjectEnvironmentIdentity{EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL},
		BindingScope: agent.ProjectBindingScope{ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration},
		ProjectID:    "project-1", InstructionVersion: 5, ContentHash: strings.Repeat("f", 64),
	}}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	dir := controller.SessionDir()
	_, key, err := validateSessionPath(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := trashSessionArtifacts(dir, path, key); err != nil {
		t.Fatal(err)
	}
	trashed, err := listTrashedSessionFiles(dir)
	if err != nil || len(trashed) != 1 {
		t.Fatalf("trashed paths = %#v, %v", trashed, err)
	}
	trashPath := trashed[0]
	before, _ := os.ReadFile(trashPath)
	if got := app.ListAllTrashedSessions(); len(got) != 1 || got[0].Preview != "private trash preview" {
		t.Fatalf("same-scope trash list = %#v", got)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := state.runtime.bindings.Rebind(context.Background(), actor, "usr_other"); err != nil {
		t.Fatal(err)
	}
	if got := app.ListAllTrashedSessions(); len(got) != 0 {
		t.Fatalf("cross-account trash list = %#v", got)
	}
	if err := app.RestoreSession(trashPath); err == nil {
		t.Fatal("cross-account trash restore succeeded")
	}
	if err := app.PurgeTrashedSession(trashPath); err == nil {
		t.Fatal("cross-account trash purge succeeded")
	}
	after, _ := os.ReadFile(trashPath)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected trash mutation changed transcript")
	}
}

func TestWorkBuddyProjectPermanentCopyAllowsOriginalProfileDeleteAndPurge(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	app.projectTreeChangedHook = func() {}
	workspace := t.TempDir()
	controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	if err := addProject(workspace, "Project workspace"); err != nil {
		t.Fatal(err)
	}
	projectSessionDir := desktopSessionDir(workspace)
	if err := os.MkdirAll(projectSessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(projectSessionDir, "project-session.jsonl")
	controller.SetSessionPath(path)
	app.tabRuntimes["project"].SessionPath = path
	controller.Resume(agent.NewSessionFromMessages([]provider.Message{{Role: provider.RoleSystem, Content: "system"}, {Role: provider.RoleUser, Content: "retained local copy"}}), path)
	if err := controller.Snapshot(); err != nil {
		t.Fatal(err)
	}
	meta := agent.BranchMeta{
		ProjectContext: &agent.ProjectContextPin{
			Environment:  agent.ProjectEnvironmentIdentity{EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL},
			BindingScope: agent.ProjectBindingScope{ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration},
			ProjectID:    "project-1", InstructionVersion: 5, ContentHash: strings.Repeat("f", 64),
		},
		ProjectContextPermanentlyReadOnly: true,
	}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	controller.Close()
	localDir := t.TempDir()
	localPath := filepath.Join(localDir, "local-active.jsonl")
	localController := control.New(control.Options{SessionDir: localDir, SessionPath: localPath, WorkspaceRoot: workspace, Actor: userctx.ActorContext{ActorID: "local", ProfileID: "default"}, Sink: event.Discard})
	t.Cleanup(localController.Close)
	app.tabRuntimes = map[string]*WorkspaceTab{"local": {ID: "local", Scope: "project", WorkspaceRoot: workspace, SessionPath: localPath, Ctrl: localController, Ready: true, actor: userctx.ActorContext{ActorID: "local", ProfileID: "default"}}}
	app.activeTabID = "local"

	if err := app.DeleteSession(path); err != nil {
		t.Fatalf("delete permanent local copy: %v", err)
	}
	trashed, err := listTrashedSessionFiles(projectSessionDir)
	if err != nil || len(trashed) != 1 {
		t.Fatalf("trashed paths = %#v, %v", trashed, err)
	}
	trashPath := trashed[0]
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("stat trashed transcript: %v", err)
	}
	if err := app.PurgeTrashedSession(trashPath); err != nil {
		t.Fatalf("purge permanent local copy: %v", err)
	}
	if _, err := os.Stat(trashPath); !os.IsNotExist(err) {
		t.Fatalf("trashed transcript still exists after purge: %v", err)
	}
}
