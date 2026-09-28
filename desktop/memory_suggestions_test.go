package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/brand"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/memory"
	"kwu/internal/provider"
	"kwu/internal/skill"
	"kwu/internal/userctx"
)

func TestMemorySuggestionsReturnsNonNilArraysBeforeStartup(t *testing.T) {
	isolateDesktopUserDirs(t)

	view := NewApp().MemorySuggestions()
	if view.Memories == nil || view.Skills == nil {
		t.Fatalf("MemorySuggestions() arrays must be non-nil before startup: %+v", view)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal MemorySuggestions(): %v", err)
	}
	for _, bad := range []string{`"memories":null`, `"skills":null`} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("MemorySuggestions() JSON contains %s; frontend expects []: %s", bad, raw)
		}
	}
}

func TestMemorySuggestionsAcceptMemoryCandidate(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	writeSuggestionSession(t, sessionDir, "pref.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "以后请始终用中文回复，除非我明确要求英文。"},
		provider.Message{Role: provider.RoleAssistant, Content: "好的。"},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: store, CWD: cwd, UserDir: userDir},
		SessionDir: sessionDir,
	}), "test-model")
	app.tabRuntimes["test"].WorkspaceRoot = cwd

	view := app.MemorySuggestions()
	if len(view.Memories) == 0 {
		t.Fatalf("MemorySuggestions() memories = %+v, want at least one candidate", view.Memories)
	}
	path, err := app.AcceptMemorySuggestion(view.Memories[0])
	if err != nil {
		t.Fatalf("AcceptMemorySuggestion: %v", err)
	}
	if path == "" {
		t.Fatal("AcceptMemorySuggestion returned empty path")
	}
	got := store.List()
	if len(got) != 1 || !strings.Contains(got[0].Body, "中文回复") {
		t.Fatalf("saved memories = %+v, want confirmed candidate body", got)
	}
}

func TestMemorySuggestionsForTabUsesSelectedTab(t *testing.T) {
	isolateDesktopUserDirs(t)
	activeUserDir := t.TempDir()
	selectedUserDir := t.TempDir()
	activeCwd := t.TempDir()
	selectedCwd := t.TempDir()
	activeSessionDir := t.TempDir()
	selectedSessionDir := t.TempDir()
	activeStore := memory.StoreFor(activeUserDir, activeCwd)
	selectedStore := memory.StoreFor(selectedUserDir, selectedCwd)
	writeSuggestionSession(t, selectedSessionDir, "selected.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "以后请始终用中文回复，除非我明确要求英文。"},
		provider.Message{Role: provider.RoleAssistant, Content: "好的。"},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: activeStore, CWD: activeCwd, UserDir: activeUserDir},
		SessionDir: activeSessionDir,
	}), "test-model")
	app.tabRuntimes["test"].WorkspaceRoot = activeCwd
	app.tabRuntimes["selected"] = &WorkspaceTab{
		ID:            "selected",
		Scope:         "project",
		WorkspaceRoot: selectedCwd,
		Ctrl: control.New(control.Options{
			Memory:     &memory.Set{Store: selectedStore, CWD: selectedCwd, UserDir: selectedUserDir},
			SessionDir: selectedSessionDir,
		}),
		Ready:       true,
		disabledMCP: map[string]ServerView{},
	}

	if view := app.MemorySuggestions(); len(view.Memories) != 0 {
		t.Fatalf("active tab suggestions = %+v, want none", view.Memories)
	}
	view := app.MemorySuggestionsForTab("selected")
	if len(view.Memories) == 0 {
		t.Fatalf("MemorySuggestionsForTab(selected) memories = %+v, want at least one candidate", view.Memories)
	}
	path, err := app.AcceptMemorySuggestionForTab("selected", view.Memories[0])
	if err != nil {
		t.Fatalf("AcceptMemorySuggestionForTab: %v", err)
	}
	if !strings.HasPrefix(path, selectedStore.Dir) && !strings.HasPrefix(path, selectedStore.GlobalDir) {
		t.Fatalf("memory path = %q, want selected store under %q or %q", path, selectedStore.Dir, selectedStore.GlobalDir)
	}
	if got := activeStore.List(); len(got) != 0 {
		t.Fatalf("active store should remain untouched, got %+v", got)
	}
	got := selectedStore.List()
	if len(got) != 1 || !strings.Contains(got[0].Body, "中文回复") {
		t.Fatalf("selected store = %+v, want confirmed candidate body", got)
	}

	skillPath, err := app.AcceptSkillSuggestionForTab("selected", SkillSuggestion{
		ID:          "selected-skill",
		Name:        "selected-workflow",
		Description: "Selected workspace workflow",
		Scope:       "project",
		Body:        "Use the selected workspace context before changing files.",
	})
	if err != nil {
		t.Fatalf("AcceptSkillSuggestionForTab: %v", err)
	}
	wantSkillPath := filepath.Join(selectedCwd, ".clouvia", "skills", "selected-workflow", "SKILL.md")
	if skillPath != wantSkillPath {
		t.Fatalf("skill path = %q, want %q", skillPath, wantSkillPath)
	}
	if _, err := os.Stat(filepath.Join(activeCwd, ".clouvia", "skills", "selected-workflow", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("active workspace should not receive selected skill, stat err = %v", err)
	}
	body, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read selected skill: %v", err)
	}
	if !strings.Contains(string(body), "selected workspace context") {
		t.Fatalf("selected skill body missing candidate content:\n%s", body)
	}
}

func TestMemorySuggestionsForTabUsesActorProfileSessionDir(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	root := t.TempDir()
	store := memory.StoreFor(userDir, root)

	actor := userctx.ActorContext{
		ActorID:       "profile-user",
		ProfileID:     "profile-a",
		AuthMethod:    "test",
		WorkspaceRoot: root,
	}
	profileDir := config.NewProfilePathResolver(actor).ProjectSessionDir(root)
	defaultDir := config.NewProfilePathResolver(userctx.Default(root)).ProjectSessionDir(root)
	otherActor := actor
	otherActor.ProfileID = "profile-b"
	otherDir := config.NewProfilePathResolver(otherActor).ProjectSessionDir(root)

	writeSuggestionSession(t, profileDir, "profile.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "always use profile scoped suggestions only"},
		provider.Message{Role: provider.RoleAssistant, Content: "noted"},
	)
	writeSuggestionSession(t, defaultDir, "default.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "always leak default suggestions"},
		provider.Message{Role: provider.RoleAssistant, Content: "noted"},
	)
	writeSuggestionSession(t, otherDir, "other.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "always leak other profile suggestions"},
		provider.Message{Role: provider.RoleAssistant, Content: "noted"},
	)

	ctrl := control.New(control.Options{
		Memory:        &memory.Set{Store: store, CWD: root, UserDir: userDir},
		WorkspaceRoot: root,
		Actor:         actor,
	})
	defer ctrl.Close()

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"profile-tab": {
			ID:            "profile-tab",
			Scope:         "project",
			WorkspaceRoot: root,
			Ctrl:          ctrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.tabOrder = []string{"profile-tab"}
	app.activeTabID = "profile-tab"

	view := app.MemorySuggestionsForTab("profile-tab")
	if len(view.Memories) == 0 {
		t.Fatalf("MemorySuggestionsForTab(profile-tab) memories = %+v, want profile candidate", view.Memories)
	}
	joined := ""
	for _, item := range view.Memories {
		joined += "\n" + item.Body
	}
	if !strings.Contains(joined, "profile scoped suggestions only") {
		t.Fatalf("profile suggestions missing from bodies:\n%s", joined)
	}
	for _, forbidden := range []string{"leak default suggestions", "leak other profile suggestions"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("memory suggestions leaked %q:\n%s", forbidden, joined)
		}
	}
}

func TestAcceptSkillSuggestionForTabUsesActorProfileSkillStore(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()

	defaultActor := userctx.Default(workspaceRoot)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	defaultCustomRoot := filepath.Join(workspaceRoot, "default-custom-skills")
	if err := os.MkdirAll(defaultCustomRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	defaultCfg := config.Default()
	defaultCfg.Skills.Paths = []string{defaultCustomRoot}
	if err := defaultCfg.SaveToScope(defaultPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	defaultSkillPath := profileSkillPath(defaultPaths, "profile-workflow")
	if err := os.MkdirAll(filepath.Dir(defaultSkillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultSkillPath, []byte("---\ndescription: default profile copy\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	profileActor := userctx.Default(workspaceRoot)
	profileActor.ProfileID = "work"
	profilePaths := config.NewProfilePathResolver(profileActor)
	profileCustomRoot := filepath.Join(workspaceRoot, "profile-custom-skills")
	if err := os.MkdirAll(profileCustomRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	profileCfg := config.Default()
	profileCfg.Skills.Paths = []string{profileCustomRoot}
	if err := profileCfg.SaveToScope(profilePaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save profile config: %v", err)
	}

	ctrl := control.New(control.Options{Actor: profileActor, WorkspaceRoot: workspaceRoot})
	defer ctrl.Close()
	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{"profile": {
		ID:            "profile",
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		Ctrl:          ctrl,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}}
	app.activeTabID = "profile"

	gotPath, err := app.AcceptSkillSuggestionForTab("profile", SkillSuggestion{
		ID:          "profile-workflow",
		Name:        "profile-workflow",
		Description: "Profile workflow",
		Scope:       "global",
		Body:        "Use the selected profile skill store.",
	})
	if err != nil {
		t.Fatalf("AcceptSkillSuggestionForTab: %v", err)
	}
	wantPath := profileSkillPath(profilePaths, "profile-workflow")
	if filepath.Clean(gotPath) != filepath.Clean(wantPath) {
		t.Fatalf("skill path = %q, want profile path %q", gotPath, wantPath)
	}
	if raw, err := os.ReadFile(defaultSkillPath); err != nil || !strings.Contains(string(raw), "default profile copy") {
		t.Fatalf("default profile skill changed or unreadable, err=%v body=%q", err, raw)
	}

	roots := skillStoreForWorkspace(workspaceRoot, profileActor).Roots()
	if !hasSkillRoot(roots, profileCustomRoot, skill.ScopeCustom) {
		t.Fatalf("profile custom root missing from actor-aware store: roots=%+v", roots)
	}
	if hasSkillRoot(roots, defaultCustomRoot, skill.ScopeCustom) {
		t.Fatalf("default custom root leaked into actor-aware store: roots=%+v", roots)
	}
}

func TestAcceptSkillSuggestionForTabUsesExplicitCurrentActorProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()

	tabActor := userctx.Default(workspaceRoot)
	tabActor.ProfileID = "tab-profile"
	tabPaths := config.NewProfilePathResolver(tabActor)
	tabCfg := config.Default()
	if err := tabCfg.SaveToScope(tabPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save tab config: %v", err)
	}

	currentActor := userctx.Default(workspaceRoot)
	currentActor.ProfileID = "current-profile"
	currentPaths := config.NewProfilePathResolver(currentActor)
	currentCfg := config.Default()
	if err := currentCfg.SaveToScope(currentPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save current config: %v", err)
	}

	ctrl := control.New(control.Options{Actor: tabActor, WorkspaceRoot: workspaceRoot})
	defer ctrl.Close()
	app := NewApp()
	app.setCurrentActorContext(currentActor)
	app.tabRuntimes = map[string]*WorkspaceTab{"tab": {
		ID:            "tab",
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		Ctrl:          ctrl,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}}
	app.activeTabID = "tab"

	gotPath, err := app.AcceptSkillSuggestionForTab("tab", SkillSuggestion{
		ID:          "current-workflow",
		Name:        "current-workflow",
		Description: "Current profile workflow",
		Scope:       "global",
		Body:        "Use the explicit current profile.",
	})
	if err != nil {
		t.Fatalf("AcceptSkillSuggestionForTab: %v", err)
	}
	wantPath := profileSkillPath(currentPaths, "current-workflow")
	if filepath.Clean(gotPath) != filepath.Clean(wantPath) {
		t.Fatalf("skill path = %q, want explicit current profile path %q", gotPath, wantPath)
	}
	if _, err := os.Stat(profileSkillPath(tabPaths, "current-workflow")); !os.IsNotExist(err) {
		t.Fatalf("tab profile should not receive explicit current profile skill, stat err = %v", err)
	}
}

func TestMemorySuggestionsAcceptSkillCandidate(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	sessionDir := t.TempDir()
	store := memory.StoreFor(userDir, cwd)
	writeSuggestionSession(t, sessionDir, "pr-a.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "把这个 PR 合并到本地并说明主要做了什么。"},
		provider.Message{Role: provider.RoleAssistant, Content: "已检查。"},
	)
	writeSuggestionSession(t, sessionDir, "pr-b.jsonl",
		provider.Message{Role: provider.RoleUser, Content: "解决该 pr 下机器人提出来的问题，合理的问题进行修复。"},
		provider.Message{Role: provider.RoleAssistant, Content: "已处理。"},
	)

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{
		Memory:     &memory.Set{Store: store, CWD: cwd, UserDir: userDir},
		SessionDir: sessionDir,
	}), "test-model")
	app.tabRuntimes["test"].WorkspaceRoot = cwd

	view := app.MemorySuggestions()
	var candidate SkillSuggestion
	for _, item := range view.Skills {
		if item.Name == "clouvia-pr-followup" {
			candidate = item
			break
		}
	}
	if candidate.Name == "" {
		t.Fatalf("MemorySuggestions() skills = %+v, want clouvia-pr-followup", view.Skills)
	}
	path, err := app.AcceptSkillSuggestion(candidate)
	if err != nil {
		t.Fatalf("AcceptSkillSuggestion: %v", err)
	}
	wantSuffix := filepath.Join(".clouvia", "skills", "clouvia-pr-followup", "SKILL.md")
	if !strings.HasSuffix(path, wantSuffix) {
		t.Fatalf("skill path = %q, want suffix %q", path, wantSuffix)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(body), "Review or update a "+brand.DisplayName+" GitHub PR") {
		t.Fatalf("skill body missing description: %s", body)
	}
}

func TestSuggestSkillsTreatsLegacyGeneratedNameAsExisting(t *testing.T) {
	sessions := []suggestionSession{
		{ID: "pr-a", Messages: []provider.Message{{Role: provider.RoleUser, Content: "检查这个 PR 并处理评审意见"}}},
		{ID: "pr-b", Messages: []provider.Message{{Role: provider.RoleUser, Content: "更新 GitHub pull request"}}},
	}
	got := suggestSkills(t.TempDir(), []skill.Skill{{Name: "kwu-pr-followup"}}, sessions)
	for _, candidate := range got {
		if candidate.Name == "clouvia-pr-followup" {
			t.Fatal("legacy kwu-pr-followup should suppress the renamed duplicate suggestion")
		}
	}
}

func writeSuggestionSession(t *testing.T, dir, name string, messages ...provider.Message) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sess := agent.NewSession("")
	for _, msg := range messages {
		sess.Add(msg)
	}
	if err := sess.Save(filepath.Join(dir, name)); err != nil {
		t.Fatalf("save session %s: %v", name, err)
	}
}

func profileSkillPath(paths config.ProfilePathResolver, name string) string {
	return filepath.Join(paths.KWUHomeDir(), "skills", name, "SKILL.md")
}

func hasSkillRoot(roots []skill.Root, dir string, scope skill.Scope) bool {
	want := filepath.Clean(dir)
	for _, root := range roots {
		if filepath.Clean(root.Dir) == want && root.Scope == scope {
			return true
		}
	}
	return false
}
