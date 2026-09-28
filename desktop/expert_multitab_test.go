package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/provider"
)

func newDesktopExpertTestController(t *testing.T, storePath, sessionPath string) (*control.Controller, *agent.Agent) {
	t.Helper()
	loader := func() (*expert.Catalog, error) { return expert.LoadCatalog(storePath) }
	catalog, err := loader()
	if err != nil {
		t.Fatal(err)
	}
	builder := func(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
		current, err := loader()
		if err != nil {
			return "", expert.Selection{}, err
		}
		selection, err := expert.ResolveSelection(current, sessionExpertID, requestExpertID, "")
		if err != nil {
			return "", selection, err
		}
		system := "base"
		if selection.Selected {
			system = expert.ApplyPrompt(system, selection.Expert)
		}
		return system, selection, nil
	}
	system, selection, err := builder("", "")
	if err != nil {
		t.Fatal(err)
	}
	executor := agent.New(nil, nil, agent.NewSession(system), agent.Options{}, event.Discard)
	controller := control.New(control.Options{
		Executor: executor, SystemPrompt: system, Experts: catalog, Expert: selection,
		PromptBuilder: builder, ExpertCatalogLoader: loader, UserExpertPath: storePath,
		SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath, Label: "desktop-test",
	})
	return controller, executor
}

func newDesktopSQLiteExpertTestController(t *testing.T, sqlitePath, userPath, sessionPath string) (*control.Controller, *agent.Agent) {
	t.Helper()
	loader := func() (*expert.Catalog, error) {
		store := expert.NewStore(sqlitePath, expert.BuiltinDefinitions())
		return expert.LoadStoreCatalog(context.Background(), store, userPath)
	}
	catalog, err := loader()
	if err != nil {
		t.Fatal(err)
	}
	builder := func(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
		current, err := loader()
		if err != nil {
			return "", expert.Selection{}, err
		}
		selection, err := expert.ResolveSelection(current, sessionExpertID, requestExpertID, "")
		if err != nil {
			return "", selection, err
		}
		system := "base"
		if selection.Selected {
			system = expert.ApplyPrompt(system, selection.Expert)
		}
		return system, selection, nil
	}
	executor := agent.New(nil, nil, agent.NewSession("base"), agent.Options{}, event.Discard)
	controller := control.New(control.Options{
		Executor: executor, SystemPrompt: "base", Experts: catalog,
		PromptBuilder: builder, ExpertCatalogLoader: loader, UserExpertPath: userPath,
		SessionDir: filepath.Dir(sessionPath), SessionPath: sessionPath, Label: "desktop-sqlite-test",
	})
	return controller, executor
}

func TestUserExpertMutationSynchronizesEveryTabController(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "experts.json")
	controllerA, _ := newDesktopExpertTestController(t, storePath, filepath.Join(dir, "a.jsonl"))
	controllerB, executorB := newDesktopExpertTestController(t, storePath, filepath.Join(dir, "b.jsonl"))
	defer controllerA.Close()
	defer controllerB.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"a": {ID: "a", Ctrl: controllerA, Ready: true},
			"b": {ID: "b", Ctrl: controllerB, Ready: true},
		},
		activeTabID: "a",
	}

	created, err := app.CreateUserExpert(expert.UserExpertInput{
		Name: "Shared Expert", Description: "shared", SystemPrompt: "prompt one", DisplayPrompt: "one", Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateUserExpert: %v", err)
	}
	if _, err := app.SetExpertForTab("b", created.ID); err != nil {
		t.Fatalf("SetExpertForTab after cross-tab catalog reload: %v", err)
	}
	if state := app.CurrentExpertForTab("b"); state.SelectedID != created.ID {
		t.Fatalf("CurrentExpertForTab = %+v, want %q", state, created.ID)
	}
	if users, err := app.ListUserExpertsForTab("b"); err != nil || len(users) != 1 || users[0].ID != created.ID {
		t.Fatalf("ListUserExpertsForTab = %+v, want created expert", users)
	}
	executorB.Session().Add(provider.Message{Role: provider.RoleUser, Content: "preserve this turn"})

	if _, err := app.UpdateUserExpert(created.ID, expert.UserExpertInput{
		Name: "Shared Expert", Description: "updated", SystemPrompt: "prompt two", DisplayPrompt: "two", Enabled: true,
	}); err != nil {
		t.Fatalf("UpdateUserExpert: %v", err)
	}
	messages := executorB.Session().Snapshot()
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "prompt two") || strings.Contains(messages[0].Content, "prompt one") {
		t.Fatalf("tab B system prompt was not refreshed: %+v", messages)
	}
	if messages[1].Content != "preserve this turn" {
		t.Fatalf("tab B history was not preserved: %+v", messages)
	}

	if err := app.DeleteUserExpert(created.ID); err != nil {
		t.Fatalf("DeleteUserExpert: %v", err)
	}
	if state := controllerB.CurrentExpert(); state.SelectedID != "" {
		t.Fatalf("tab B kept deleted expert selection: %+v", state)
	}
	if messages := executorB.Session().Snapshot(); len(messages) != 2 || strings.Contains(messages[0].Content, "prompt two") {
		t.Fatalf("tab B kept deleted expert system prompt: %+v", messages)
	}
}

func TestSQLiteExpertMutationSynchronizesEveryTabAndCleansLibraryState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KWU_STATE_HOME", dir)
	t.Setenv("KWU_HOME", dir)
	sqlitePath := filepath.Join(config.MemoryUserDir(), "experts.sqlite")
	userPath := config.UserExpertsPath()
	controllerA, _ := newDesktopSQLiteExpertTestController(t, sqlitePath, userPath, filepath.Join(dir, "a-sqlite.jsonl"))
	controllerB, executorB := newDesktopSQLiteExpertTestController(t, sqlitePath, userPath, filepath.Join(dir, "b-sqlite.jsonl"))
	defer controllerA.Close()
	defer controllerB.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"a": {ID: "a", Ctrl: controllerA, Ready: true},
			"b": {ID: "b", Ctrl: controllerB, Ready: true},
		},
		activeTabID: "a",
	}
	t.Cleanup(app.closeExpertStore)
	input := ExpertInfo{
		ID:          "custom-sqlite-reviewer",
		Mode:        "expert",
		Name:        "SQLite Reviewer",
		Role:        "Review",
		Description: "Initial SQLite mission.",
		Categories:  []string{"Engineering"},
		Tags:        []string{"sqlite"},
		Prompts:     []string{"Review this.", "List risks.", "Suggest fixes."},
		Enabled:     true,
	}
	created, err := app.CreateExpert(input)
	if err != nil {
		t.Fatalf("CreateExpert: %v", err)
	}
	if created.Source != expert.SourceUser {
		t.Fatalf("created source = %q, want user", created.Source)
	}
	if _, err := app.SetExpertForTab("b", created.ID); err != nil {
		t.Fatalf("SetExpertForTab after create reload: %v", err)
	}
	if _, err := app.SetExpertFavorite(created.ID, true); err != nil {
		t.Fatalf("SetExpertFavorite: %v", err)
	}
	executorB.Session().Add(provider.Message{Role: provider.RoleUser, Content: "preserve this turn"})

	input.Description = "Updated SQLite mission."
	if _, err := app.UpdateExpert(created.ID, input); err != nil {
		t.Fatalf("UpdateExpert: %v", err)
	}
	messages := executorB.Session().Snapshot()
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "Updated SQLite mission.") || strings.Contains(messages[0].Content, "Initial SQLite mission.") {
		t.Fatalf("tab B system prompt was not refreshed: %+v", messages)
	}
	if messages[1].Content != "preserve this turn" {
		t.Fatalf("tab B history was not preserved: %+v", messages)
	}

	if err := app.DeleteExpert(created.ID); err != nil {
		t.Fatalf("DeleteExpert: %v", err)
	}
	if state := controllerB.CurrentExpert(); state.SelectedID != "" {
		t.Fatalf("tab B kept deleted SQLite expert selection: %+v", state)
	}
	if _, err := app.SetExpertForTab("b", created.ID); err == nil {
		t.Fatalf("deleted SQLite expert %q remained in the runtime catalog", created.ID)
	}
	libraryState, err := expert.LoadExpertLibraryState(userPath)
	if err != nil {
		t.Fatalf("LoadExpertLibraryState: %v", err)
	}
	if containsID(libraryState.FavoriteIDs, created.ID) || containsID(libraryState.RecentIDs, created.ID) {
		t.Fatalf("library state kept deleted SQLite expert: %+v", libraryState)
	}
}

func containsID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
