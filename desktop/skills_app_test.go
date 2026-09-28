package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"kwu/internal/boot"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/skill"
	"kwu/internal/userctx"
)

func TestNormalizeSkillPathDirectoryLayout(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: x\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := normalizeSkillPath(skillDir); got != root {
		t.Fatalf("normalizeSkillPath(%q) = %q, want %q", skillDir, got, root)
	}
}

func TestSkillRootsViewCountsProjectSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	project := t.TempDir()
	root := filepath.Join(project, ".kwu", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proj.md"), []byte("---\ndescription: project\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}

	roots := skillRootsView()
	want := realTestPath(root)
	for _, r := range roots {
		if realTestPath(r.Dir) == want {
			if r.Status != "ok" || r.Skills != 1 || r.Scope != "project" {
				t.Fatalf("project root view = %+v", r)
			}
			if len(r.SkillItems) != 1 || r.SkillItems[0].Name != "proj" || r.SkillItems[0].Description != "project" {
				t.Fatalf("project root skill items = %+v", r.SkillItems)
			}
			return
		}
	}
	t.Fatalf("project skill root %q not found in %+v", root, roots)
}

func TestSkillRootsViewMarksEnvConfiguredCustomRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	project := t.TempDir()
	root := filepath.Join(home, "custom-skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "custom.md"), []byte("---\ndescription: custom\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KWU_TEST_SKILL_ROOT", root)
	cfgPath := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[skills]\npaths = [\"${KWU_TEST_SKILL_ROOT}\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}

	roots := skillRootsView()
	want := realTestPath(root)
	for _, r := range roots {
		if realTestPath(r.Dir) == want {
			if !r.Configured || r.Skills != 1 || r.Scope != "custom" {
				t.Fatalf("custom root view = %+v, want configured custom root with one skill", r)
			}
			if len(r.SkillItems) != 1 || r.SkillItems[0].Name != "custom" || r.SkillItems[0].Scope != "custom" {
				t.Fatalf("custom root skill items = %+v", r.SkillItems)
			}
			return
		}
	}
	t.Fatalf("custom skill root %q not found in %+v", root, roots)
}

func TestSkillRootsViewDedupesConfiguredConventionRoot(t *testing.T) {
	home := isolateDesktopUserDirs(t)
	project := t.TempDir()
	root := filepath.Join(home, ".kwu", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "global.md"), []byte("---\ndescription: global\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[skills]\npaths = [\"~/.kwu/skills\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}

	roots := skillRootsView()
	want := realTestPath(root)
	var matches []SkillRootView
	for _, r := range roots {
		if realTestPath(r.Dir) == want {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("skill root %q appears %d times, want once: %+v", root, len(matches), matches)
	}
	if !matches[0].Configured || matches[0].Skills != 1 || len(matches[0].SkillItems) != 1 {
		t.Fatalf("deduped root should keep configured metadata and skills, got %+v", matches[0])
	}
}

func TestSkillRootsViewDedupesConfiguredProjectConventionRoot(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	root := filepath.Join(project, ".kwu", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "project.md"), []byte("---\ndescription: project\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(cwd, ".kwu", "skills")
	t.Setenv("KWU_TEST_PROJECT_SKILL_ROOT", root)
	cfgPath := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[skills]\npaths = ["+strconv.Quote(root)+"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	roots := skillRootsView()
	want := realTestPath(root)
	var matches []SkillRootView
	for _, r := range roots {
		if realTestPath(r.Dir) == want {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("project skill root %q appears %d times, want once: %+v", root, len(matches), matches)
	}
	if !matches[0].Configured || matches[0].Skills != 1 || len(matches[0].SkillItems) != 1 {
		t.Fatalf("deduped project root should keep configured metadata and skills, got %+v", matches[0])
	}
	if matches[0].Status == "inactive" {
		t.Fatalf("deduped project root should stay active, got %+v", matches[0])
	}
}

func TestSkillRootsViewOmitsExcludedConventionRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	project := t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "noisy.md"), []byte("---\ndescription: noisy\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[skills]\nexcluded_paths = [\"~/.agents/skills\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}

	roots := skillRootsView()
	want := realTestPath(root)
	for _, r := range roots {
		if realTestPath(r.Dir) == want {
			t.Fatalf("excluded convention root should be hidden, got %+v in %+v", r, roots)
		}
	}
}

func TestRemoveSkillPathPseudoDeletesConventionRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	path := filepath.Join(home, ".agents", "skills")
	app := NewApp()

	if err := app.RemoveSkillPath(path); err != nil {
		t.Fatalf("RemoveSkillPath: %v", err)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if len(cfg.Skills.ExcludedPaths) != 1 || realTestPath(cfg.Skills.ExcludedPaths[0]) != realTestPath(path) {
		t.Fatalf("excluded paths = %v, want %q", cfg.Skills.ExcludedPaths, path)
	}
}

func TestAddSkillPathRestoresConventionRootWithoutCustomPath(t *testing.T) {
	home := isolateDesktopUserDirs(t)
	path := filepath.Join(home, ".agents", "skills")
	cfgPath := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[skills]\nexcluded_paths = [\"~/.agents/skills\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := NewApp()

	if err := app.AddSkillPath(path); err != nil {
		t.Fatalf("AddSkillPath: %v", err)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if len(cfg.Skills.ExcludedPaths) != 0 {
		t.Fatalf("excluded paths after restore = %v, want empty", cfg.Skills.ExcludedPaths)
	}
	if len(cfg.Skills.Paths) != 0 {
		t.Fatalf("restored convention root should not become custom path: %v", cfg.Skills.Paths)
	}
}

func TestCapabilitiesIncludesDisabledSkills(t *testing.T) {
	a := NewApp()
	a.setTestCtrl(control.New(control.Options{
		Skills: []skill.Skill{
			{Name: "explore", Description: "enabled", Scope: skill.ScopeBuiltin, RunAs: skill.RunSubagent},
		},
		AllSkills: []skill.Skill{
			{Name: "explore", Description: "enabled", Scope: skill.ScopeBuiltin, RunAs: skill.RunSubagent},
			{Name: "review", Description: "disabled", Scope: skill.ScopeBuiltin, RunAs: skill.RunSubagent},
		},
	}), "")
	defer a.activeCtrl().Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	cfgPath := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("[skills]\ndisabled_skills = [\"review\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	settingsView := a.SkillsSettings()
	assertSkillStates := func(t *testing.T, skills []SkillView) {
		t.Helper()
		states := map[string]bool{}
		for _, sk := range skills {
			states[sk.Name] = sk.Enabled
		}
		if states["explore"] != true {
			t.Fatalf("explore should be enabled in skills view: %+v", skills)
		}
		enabled, ok := states["review"]
		if !ok || enabled {
			t.Fatalf("review should be disabled but present in skills view: %+v", skills)
		}
	}
	assertSkillStates(t, settingsView.Skills)
	assertSkillStates(t, a.Capabilities().Skills)
}

func TestSkillsSettingsWithoutControllerDiscoversWorkspaceSkills(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	skillRoot := filepath.Join(workspaceRoot, ".kwu", "skills")
	skillDir := filepath.Join(skillRoot, "workspace-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: workspace skill\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.tabRuntimes = map[string]*WorkspaceTab{"booting": {
		ID:            "booting",
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		disabledMCP:   map[string]ServerView{},
	}}
	a.activeTabID = "booting"

	view := a.SkillsSettings()
	found := false
	for _, sk := range view.Skills {
		if sk.Name == "workspace-review" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("workspace skill missing while controller is unavailable: %+v", view.Skills)
	}
	if got := skillRootCount(view.SkillRoots, skillRoot); got != 1 {
		t.Fatalf("workspace skill root count = %d, want 1; roots=%+v", got, view.SkillRoots)
	}
}

func TestSkillsSettingsUseActiveTabProfileConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	t.Chdir(workspaceRoot)

	defaultActor := userctx.Default(workspaceRoot)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	defaultRoot := filepath.Join(workspaceRoot, "default-skills")
	if err := os.MkdirAll(defaultRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultRoot, "default.md"), []byte("---\ndescription: default\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	defaultCfg := config.Default()
	defaultCfg.Skills.DisabledSkills = []string{"review"}
	defaultCfg.Skills.Paths = []string{defaultRoot}
	if err := defaultCfg.SaveToScope(defaultPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	workActor := userctx.Default(workspaceRoot)
	workActor.ProfileID = "work"
	workPaths := config.NewProfilePathResolver(workActor)
	workRoot := filepath.Join(workspaceRoot, "work-skills")
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workRoot, "work.md"), []byte("---\ndescription: work\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	workCfg := config.Default()
	workCfg.Skills.DisabledSkills = []string{"explore"}
	workCfg.Skills.Paths = []string{workRoot}
	if err := workCfg.SaveToScope(workPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save work config: %v", err)
	}

	ctrl := control.New(control.Options{
		Actor:         workActor,
		WorkspaceRoot: workspaceRoot,
		AllSkills: []skill.Skill{
			{Name: "explore", Description: "work disabled", Scope: skill.ScopeBuiltin, RunAs: skill.RunSubagent},
			{Name: "review", Description: "default disabled", Scope: skill.ScopeBuiltin, RunAs: skill.RunSubagent},
		},
	})
	defer ctrl.Close()
	a := NewApp()
	a.tabRuntimes = map[string]*WorkspaceTab{"work": {
		ID:            "work",
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		Ready:         true,
		Ctrl:          ctrl,
		disabledMCP:   map[string]ServerView{},
	}}
	a.activeTabID = "work"

	view := a.SkillsSettings()
	states := map[string]bool{}
	for _, sk := range view.Skills {
		states[sk.Name] = sk.Enabled
	}
	if states["review"] != true {
		t.Fatalf("review should stay enabled for work profile despite default profile disable: %+v", view.Skills)
	}
	if states["explore"] != false {
		t.Fatalf("explore should be disabled by work profile: %+v", view.Skills)
	}
	if got := skillRootCount(view.SkillRoots, workRoot); got != 1 {
		t.Fatalf("work profile skill root count = %d, want 1; roots=%+v", got, view.SkillRoots)
	}
	if got := skillRootCount(view.SkillRoots, defaultRoot); got != -1 {
		t.Fatalf("default profile skill root leaked into work view: count=%d roots=%+v", got, view.SkillRoots)
	}
}

func TestAcceptedProfileGlobalSkillVisibleInRootsAndBoot(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	t.Chdir(workspaceRoot)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "kwu.toml"), []byte(`
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	defaultActor := userctx.Default(workspaceRoot)
	profileActor := userctx.Default(workspaceRoot)
	profileActor.ProfileID = "work"
	profilePaths := config.NewProfilePathResolver(profileActor)

	a := NewApp()
	a.tabRuntimes = map[string]*WorkspaceTab{"work": {
		ID:            "work",
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		actor:         profileActor,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}}
	a.activeTabID = "work"

	path, err := a.AcceptSkillSuggestionForTab("work", SkillSuggestion{
		ID:          "profile-global",
		Name:        "profile-global",
		Description: "Profile global workflow",
		Scope:       "global",
		Body:        "Use the profile global skill root.",
	})
	if err != nil {
		t.Fatalf("AcceptSkillSuggestionForTab: %v", err)
	}
	wantSkillPath := filepath.Join(profilePaths.KWUHomeDir(), "skills", "profile-global", "SKILL.md")
	if filepath.Clean(path) != filepath.Clean(wantSkillPath) {
		t.Fatalf("skill path = %q, want profile global path %q", path, wantSkillPath)
	}

	profileRoot := filepath.Join(profilePaths.KWUHomeDir(), "skills")
	profileRoots := a.cachedSkillRootsView(workspaceRoot, profileActor)
	root, ok := skillRootViewFor(profileRoots, profileRoot)
	if !ok || root.Skills != 1 || !skillRootViewHasSkill(root, "profile-global") {
		t.Fatalf("profile skill root view missing accepted skill; root=%+v ok=%v roots=%+v", root, ok, profileRoots)
	}
	defaultRoots := a.cachedSkillRootsView(workspaceRoot, defaultActor)
	if _, ok := skillRootViewFor(defaultRoots, profileRoot); ok {
		t.Fatalf("default profile roots leaked profile global root %q: %+v", profileRoot, defaultRoots)
	}

	ctrl, err := boot.Build(context.Background(), boot.Options{
		Sink:          event.Discard,
		WorkspaceRoot: workspaceRoot,
		Actor:         profileActor,
	})
	if err != nil {
		t.Fatalf("boot.Build profile actor: %v", err)
	}
	defer ctrl.Close()
	sk, ok := skillByName(ctrl.Skills(), "profile-global")
	if !ok || filepath.Clean(sk.Path) != filepath.Clean(wantSkillPath) {
		t.Fatalf("boot profile skills missing accepted skill at profile path; skill=%+v ok=%v all=%+v", sk, ok, ctrl.Skills())
	}

	defaultCtrl, err := boot.Build(context.Background(), boot.Options{
		Sink:          event.Discard,
		WorkspaceRoot: workspaceRoot,
		Actor:         defaultActor,
	})
	if err != nil {
		t.Fatalf("boot.Build default actor: %v", err)
	}
	defer defaultCtrl.Close()
	if sk, ok := skillByName(defaultCtrl.AllSkills(), "profile-global"); ok {
		t.Fatalf("default boot leaked profile global skill: %+v", sk)
	}
}

func TestSkillsSettingsUseExplicitCurrentActorSnapshot(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	t.Chdir(workspaceRoot)

	tabActor := userctx.Default(workspaceRoot)
	tabActor.ProfileID = "tab-profile"
	tabPaths := config.NewProfilePathResolver(tabActor)
	tabCfg := config.Default()
	tabCfg.Skills.DisabledSkills = []string{"review"}
	if err := tabCfg.SaveToScope(tabPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save tab config: %v", err)
	}

	currentActor := userctx.Default(workspaceRoot)
	currentActor.ProfileID = "current-profile"
	currentPaths := config.NewProfilePathResolver(currentActor)
	currentCfg := config.Default()
	currentCfg.Skills.DisabledSkills = []string{"explore"}
	if err := currentCfg.SaveToScope(currentPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save current config: %v", err)
	}

	ctrl := control.New(control.Options{
		Actor:         tabActor,
		WorkspaceRoot: workspaceRoot,
		AllSkills: []skill.Skill{
			{Name: "explore", Description: "current disabled", Scope: skill.ScopeBuiltin, RunAs: skill.RunSubagent},
			{Name: "review", Description: "tab disabled", Scope: skill.ScopeBuiltin, RunAs: skill.RunSubagent},
		},
	})
	defer ctrl.Close()
	a := NewApp()
	a.setCurrentActorContext(currentActor)
	a.tabRuntimes = map[string]*WorkspaceTab{"tab": {
		ID:            "tab",
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		Ready:         true,
		Ctrl:          ctrl,
		disabledMCP:   map[string]ServerView{},
	}}
	a.activeTabID = "tab"

	view := a.SkillsSettings()
	states := map[string]bool{}
	for _, sk := range view.Skills {
		states[sk.Name] = sk.Enabled
	}
	if states["review"] != true {
		t.Fatalf("review should stay enabled for explicit current profile despite tab profile disable: %+v", view.Skills)
	}
	if states["explore"] != false {
		t.Fatalf("explore should be disabled by explicit current profile: %+v", view.Skills)
	}
}

func TestSkillsSettingsRefreshInvalidatesSkillRootsCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	project := t.TempDir()
	root := filepath.Join(project, ".kwu", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "one.md"), []byte("---\ndescription: one\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.setTestCtrl(control.New(control.Options{}), "")
	defer a.activeCtrl().Close()

	first := a.SkillsSettings()
	if got := skillRootCount(first.SkillRoots, root); got != 1 {
		t.Fatalf("initial project skill count = %d, want 1; roots=%+v", got, first.SkillRoots)
	}
	if err := os.WriteFile(filepath.Join(root, "two.md"), []byte("---\ndescription: two\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	cached := a.SkillsSettings()
	if got := skillRootCount(cached.SkillRoots, root); got != 1 {
		t.Fatalf("cached project skill count = %d, want 1 before refresh; roots=%+v", got, cached.SkillRoots)
	}
	if err := a.RefreshSkills(); err != nil {
		t.Fatalf("RefreshSkills: %v", err)
	}
	refreshed := a.SkillsSettings()
	if got := skillRootCount(refreshed.SkillRoots, root); got != 2 {
		t.Fatalf("refreshed project skill count = %d, want 2; roots=%+v", got, refreshed.SkillRoots)
	}
}

func skillRootCount(roots []SkillRootView, path string) int {
	want := realTestPath(path)
	for _, r := range roots {
		if realTestPath(r.Dir) == want {
			return r.Skills
		}
	}
	return -1
}

func skillRootViewFor(roots []SkillRootView, path string) (SkillRootView, bool) {
	want := realTestPath(path)
	for _, r := range roots {
		if realTestPath(r.Dir) == want {
			return r, true
		}
	}
	return SkillRootView{}, false
}

func skillRootViewHasSkill(root SkillRootView, name string) bool {
	for _, item := range root.SkillItems {
		if item.Name == name {
			return true
		}
	}
	return false
}

func skillByName(skills []skill.Skill, name string) (skill.Skill, bool) {
	for _, sk := range skills {
		if sk.Name == name {
			return sk, true
		}
	}
	return skill.Skill{}, false
}

func realTestPath(path string) string {
	if p, err := filepath.EvalSymlinks(path); err == nil {
		path = p
	}
	return config.CanonicalSkillPath(path)
}
