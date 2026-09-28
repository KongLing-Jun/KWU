package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/projecttrust"
	"kwu/internal/provider"
	"kwu/internal/skill"
	"kwu/internal/userctx"
)

func TestReasoningLanguageCommandUsesControllerProfile(t *testing.T) {
	isolateUserConfig(t)
	actor := testCLIProfileActor(t, "work")
	paths := config.NewProfilePathResolver(actor)

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot})
	m.runReasoningLanguageCommand("/reasoning-language zh")

	cfg := config.LoadForEditWithPaths(paths)
	if got := cfg.ReasoningLanguage(); got != "zh" {
		t.Fatalf("profile reasoning language = %q, want zh", got)
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("/reasoning-language should not touch default profile config, stat err=%v", err)
	}

	m = newTestChatTUI()
	m.ctrl = control.New(control.Options{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot})
	m.runReasoningLanguageCommand("/reasoning-language")
	if got := strings.Join(*m.pendingCommit, "\n"); !strings.Contains(got, "reasoning-language: zh") {
		t.Fatalf("/reasoning-language status read %q, want current profile zh", got)
	}
}

func TestThemeCommandPersistsToControllerProfile(t *testing.T) {
	isolateUserConfig(t)
	t.Setenv("KWU_THEME", "")
	t.Setenv("KWU_THEME_STYLE", "")
	defer restoreThemeForTest(colorEnabled, activeCLITheme)
	colorEnabled = true
	configureCLIThemeWithStyle("dark", "graphite")

	actor := testCLIProfileActor(t, "work")
	paths := config.NewProfilePathResolver(actor)
	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot})

	m.runThemeSubcommand("/theme aurora")

	cfg := config.LoadForEditWithPaths(paths)
	if cfg.UI.Theme != "dark" || cfg.UI.ThemeStyle != "aurora" {
		t.Fatalf("profile theme = %q/%q, want dark/aurora", cfg.UI.Theme, cfg.UI.ThemeStyle)
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("/theme should not touch default profile config, stat err=%v", err)
	}
}

func TestSkillsToggleAndPathsUseControllerProfile(t *testing.T) {
	isolateUserConfig(t)
	actor := testCLIProfileActor(t, "work")
	paths := config.NewProfilePathResolver(actor)
	customRoot := filepath.Join(t.TempDir(), "custom-skills")
	nestedRoot := filepath.Join(customRoot, "group", "nested")
	if err := os.MkdirAll(nestedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedRoot, skill.SkillFile), []byte("---\nname: nested\ndescription: nested skill\n---\n\n# Nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	excludedRoot := filepath.Join(actor.WorkspaceRoot, ".kwu", "skills")

	cfg := config.Default()
	cfg.Skills.Paths = []string{customRoot}
	cfg.Skills.ExcludedPaths = []string{excludedRoot}
	cfg.Skills.MaxDepth = 1
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatal(err)
	}

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{
		Actor:         actor,
		WorkspaceRoot: actor.WorkspaceRoot,
		AllSkills:     []skill.Skill{{Name: "review", Scope: skill.ScopeBuiltin, Path: "(builtin)"}},
	})
	high := "high"
	m.effortOverride = &high
	var gotEffort *string
	m.buildController = func(ref string, effortOverride *string, history []provider.Message, path string, actor userctx.ActorContext) (*control.Controller, error) {
		gotEffort = cloneStringPtr(effortOverride)
		return control.New(control.Options{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot}), nil
	}
	m.skillSaveEnabledChanges(map[string]bool{"review": false})
	if m.pendingModelSwitch == nil {
		t.Fatal("skill toggle should schedule a session refresh")
	}
	if msg, ok := m.pendingModelSwitch().(modelSwitchMsg); !ok || msg.err != nil {
		t.Fatalf("skill refresh = %#v, want successful modelSwitchMsg", msg)
	}
	if gotEffort == nil || *gotEffort != "high" {
		t.Fatalf("skill refresh effort override = %v, want high", gotEffort)
	}

	saved := config.LoadForEditWithPaths(paths)
	if !saved.IsSkillDisabled("review") {
		t.Fatal("profile config should disable review skill")
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("/skills toggle should not touch default profile config, stat err=%v", err)
	}

	roots := m.skillStore().Roots()
	if !hasSkillRoot(roots, customRoot) {
		t.Fatalf("skill roots = %+v, want custom root %q from profile config", roots, customRoot)
	}
	if hasSkillRoot(roots, excludedRoot) {
		t.Fatalf("skill roots = %+v, should exclude %q from profile config", roots, excludedRoot)
	}
	for _, sk := range m.skillStore().List() {
		if sk.Name == "nested" {
			t.Fatalf("max_depth=1 should hide nested custom skill, got %+v", sk)
		}
	}
}

func TestMCPTrustProjectUsesProfileTrustPath(t *testing.T) {
	isolateUserConfig(t)
	actor := testCLIProfileActor(t, "work")
	paths := config.NewProfilePathResolver(actor)
	cwd := mustGetwd(t)

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot})
	m.runMCPSubcommand("/mcp trust-project")

	trustPath := paths.ProjectTrustPath(cwd)
	if !(projecttrust.Store{Path: trustPath, Fallbacks: paths.ProjectTrustFallbackPaths(cwd)}).IsTrusted(cwd) {
		t.Fatalf("profile trust path %q should trust %q", trustPath, cwd)
	}
	if (projecttrust.Store{Path: config.NewProfilePathResolver(userctx.ActorContext{}).ProjectTrustPath(cwd)}).IsTrusted(cwd) {
		t.Fatal("/mcp trust-project for non-default profile should not update default trust store")
	}
}

func TestReviewLoadsConfigAndSkillPathsFromCurrentProfile(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "work")
	actor := defaultCLIActorContext()
	paths := config.NewProfilePathResolver(actor)

	defaultCfg := config.Default()
	defaultCfg.DefaultModel = "default-profile/model"
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defaultCfg.SaveToScope(config.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatal(err)
	}

	customRoot := filepath.Join(t.TempDir(), "review-skills")
	nestedRoot := filepath.Join(customRoot, "group", "nested")
	if err := os.MkdirAll(nestedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedRoot, skill.SkillFile), []byte("---\nname: nested\ndescription: nested skill\n---\n\n# Nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	excludedRoot := filepath.Join(mustGetwd(t), ".kwu", "skills")
	profileCfg := config.Default()
	profileCfg.DefaultModel = "work-profile/model"
	profileCfg.Skills.Paths = []string{customRoot}
	profileCfg.Skills.ExcludedPaths = []string{excludedRoot}
	profileCfg.Skills.MaxDepth = 1
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := profileCfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadReviewConfig(actor)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultModel != "work-profile/model" {
		t.Fatalf("review config DefaultModel = %q, want current profile", loaded.DefaultModel)
	}
	store := newReviewSkillStore(mustGetwd(t), loaded)
	if !hasSkillRoot(store.Roots(), customRoot) {
		t.Fatalf("review skill roots = %+v, want profile custom root %q", store.Roots(), customRoot)
	}
	if hasSkillRoot(store.Roots(), excludedRoot) {
		t.Fatalf("review skill roots = %+v, should exclude %q", store.Roots(), excludedRoot)
	}
	for _, sk := range store.List() {
		if sk.Name == "nested" {
			t.Fatalf("review max_depth=1 should hide nested custom skill, got %+v", sk)
		}
	}
}

func TestCLIConfigSourcePathForActorUsesCurrentProfile(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "work")
	actor := defaultCLIActorContext()
	paths := config.NewProfilePathResolver(actor)

	defaultCfg := config.Default()
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defaultCfg.SaveToScope(config.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatal(err)
	}
	if got := cliConfigSourcePathForActor(actor); got != "" {
		t.Fatalf("source path with only default profile config = %q, want empty for current profile", got)
	}

	profileCfg := config.Default()
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := profileCfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatal(err)
	}
	if got := cliConfigSourcePathForActor(actor); filepath.Clean(got) != filepath.Clean(paths.UserConfigPath()) {
		t.Fatalf("source path = %q, want current profile config %q", got, paths.UserConfigPath())
	}

	if err := os.WriteFile("kwu.toml", []byte(`default_model = "project/model"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := cliConfigSourcePathForActor(actor); filepath.Clean(got) != filepath.Clean("kwu.toml") {
		t.Fatalf("source path with project config = %q, want project config priority", got)
	}
}

func hasSkillRoot(roots []skill.Root, want string) bool {
	want = filepath.Clean(want)
	for _, root := range roots {
		if filepath.Clean(root.Dir) == want {
			return true
		}
	}
	return false
}
