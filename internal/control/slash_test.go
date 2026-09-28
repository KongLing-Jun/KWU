package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/memory"
	"kwu/internal/plugin"
	"kwu/internal/projecttrust"
	"kwu/internal/skill"
	"kwu/internal/userctx"
)

func labelsOf(items []SlashItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func has(items []SlashItem, label string) bool {
	for _, it := range items {
		if it.Label == label {
			return true
		}
	}
	return false
}

func TestSlashArgItems(t *testing.T) {
	data := ArgData{
		Skills:          []skill.Skill{{Name: "explore", Scope: skill.ScopeBuiltin}, {Name: "review", Scope: skill.ScopeBuiltin}},
		DisabledSkills:  []skill.Skill{{Name: "security-review", Scope: skill.ScopeBuiltin}},
		ServerNames:     []string{"fs", "git"},
		ConfiguredMCP:   []string{"fs", "linear"},
		DisconnectedMCP: []string{"optional"},
		ModelRefs:       []string{"deepseek-flash/deepseek-v4-flash", "deepseek-pro/deepseek-v4-pro"},
		CurrentModel:    "deepseek-flash/deepseek-v4-flash",
		EffortLevels:    []string{"auto", "high", "max"},
		ProviderNames:   []string{"deepseek-flash", "deepseek-pro", "custom"},
		CurrentProvider: "deepseek-flash",
		Experts:         []expert.Info{{ID: "architecture-review", Description: "Architecture"}, {ID: "testing", Description: "Testing"}},
		CurrentExpert:   "testing",
	}

	// /skills subcommands
	items, from := SlashArgItems("/skills ", data)
	if from != len("/skills ") {
		t.Errorf("from = %d, want %d", from, len("/skills "))
	}
	for _, w := range []string{"show", "enable", "disable", "new", "paths"} {
		if !has(items, w) {
			t.Errorf("/skills missing subcommand %q; got %v", w, labelsOf(items))
		}
	}
	if has(items, "manage") {
		t.Errorf("/skills should hide redundant manage subcommand; got %v", labelsOf(items))
	}
	if has(items, "list") {
		t.Errorf("/skills should hide redundant list subcommand; got %v", labelsOf(items))
	}
	// /skills show → skill names
	items, _ = SlashArgItems("/skills show ", data)
	if !has(items, "explore") || !has(items, "review") {
		t.Errorf("/skills show should list skill names; got %v", labelsOf(items))
	}
	// Legacy /skill still works as an alias.
	items, _ = SlashArgItems("/skill show ", data)
	if !has(items, "explore") || !has(items, "review") {
		t.Errorf("/skill show alias should list skill names; got %v", labelsOf(items))
	}
	items, _ = SlashArgItems("/skill disable ", data)
	if !has(items, "explore") || has(items, "security-review") {
		t.Errorf("/skill disable should list enabled skills only; got %v", labelsOf(items))
	}
	items, _ = SlashArgItems("/skill enable ", data)
	if !has(items, "security-review") || has(items, "review") {
		t.Errorf("/skill enable should list disabled skills only; got %v", labelsOf(items))
	}
	// /mcp subcommands + filtering
	items, _ = SlashArgItems("/mcp ", data)
	if has(items, "list") {
		t.Errorf("/mcp should hide redundant list subcommand; got %v", labelsOf(items))
	}
	items, _ = SlashArgItems("/mcp re", data)
	if len(items) != 1 || items[0].Label != "remove" {
		t.Errorf("/mcp re should filter to remove; got %v", labelsOf(items))
	}
	items, _ = SlashArgItems("/mcp ", data)
	if !has(items, "trust-project") {
		t.Errorf("/mcp should offer trust-project; got %v", labelsOf(items))
	}
	// /mcp remove → server names
	items, _ = SlashArgItems("/mcp remove ", data)
	if !has(items, "fs") || !has(items, "git") {
		t.Errorf("/mcp remove should list servers; got %v", labelsOf(items))
	}
	// /mcp connect -> disconnected configured server names
	items, _ = SlashArgItems("/mcp connect ", data)
	if !has(items, "optional") {
		t.Errorf("/mcp connect should list disconnected configured servers; got %v", labelsOf(items))
	}
	// /mcp show/tools -> connected + configured server names
	items, _ = SlashArgItems("/mcp show ", data)
	if !has(items, "fs") || !has(items, "linear") || !has(items, "optional") {
		t.Errorf("/mcp show should list known servers; got %v", labelsOf(items))
	}
	items, _ = SlashArgItems("/mcp tools ", data)
	if !has(items, "git") || !has(items, "linear") {
		t.Errorf("/mcp tools should list known servers; got %v", labelsOf(items))
	}
	// /model → refs, current marked
	items, _ = SlashArgItems("/model ", data)
	if !has(items, "deepseek-pro/deepseek-v4-pro") {
		t.Errorf("/model should list refs; got %v", labelsOf(items))
	}
	for _, it := range items {
		if it.Label == data.CurrentModel && it.Hint != "current" {
			t.Errorf("active model should be hinted 'current', got %q", it.Hint)
		}
	}
	// /provider → provider names, current marked
	items, _ = SlashArgItems("/provider ", data)
	if !has(items, "deepseek-pro") || !has(items, "custom") {
		t.Errorf("/provider should list provider names; got %v", labelsOf(items))
	}
	for _, it := range items {
		if it.Label == data.CurrentProvider && it.Hint != "current" {
			t.Errorf("active provider should be hinted 'current', got %q", it.Hint)
		}
	}
	// /provider de → filter to deepseek-*
	items, _ = SlashArgItems("/provider de", data)
	if len(items) != 2 {
		t.Errorf("/provider de should filter to 2 deepseek providers; got %v", labelsOf(items))
	}
	// /effort
	items, _ = SlashArgItems("/effort ", data)
	if !has(items, "auto") || !has(items, "high") || !has(items, "max") || has(items, "off") {
		t.Errorf("/effort should offer auto/high/max only; got %v", labelsOf(items))
	}
	// /auto-plan
	items, _ = SlashArgItems("/auto-plan ", data)
	if !has(items, "off") || !has(items, "on") || has(items, "ask") {
		t.Errorf("/auto-plan should offer only off/on; got %v", labelsOf(items))
	}
	// /goal
	items, _ = SlashArgItems("/goal ", data)
	if !has(items, "--research") || !has(items, "--simple") || !has(items, "status") || !has(items, "clear") {
		t.Errorf("/goal should offer research overrides and management commands; got %v", labelsOf(items))
	}
	if items, _ := SlashArgItems("/goal --research ", data); len(items) != 0 {
		t.Errorf("/goal after a research flag should accept free-form objectives; got %v", labelsOf(items))
	}
	// /expert
	items, _ = SlashArgItems("/expert ", data)
	if !has(items, "list") || !has(items, "use") || !has(items, "clear") {
		t.Errorf("/expert should offer list/use/clear; got %v", labelsOf(items))
	}
	items, _ = SlashArgItems("/expert use ", data)
	if !has(items, "architecture-review") || !has(items, "testing") {
		t.Errorf("/expert use should offer expert ids; got %v", labelsOf(items))
	}
	for _, it := range items {
		if it.Label == "testing" && it.Hint != "current" {
			t.Errorf("current expert should be hinted current, got %q", it.Hint)
		}
	}
	// /reasoning-language
	items, _ = SlashArgItems("/reasoning-language ", data)
	if !has(items, "auto") || !has(items, "zh") || !has(items, "en") || has(items, "中文") {
		t.Errorf("/reasoning-language should offer only auto/zh/en; got %v", labelsOf(items))
	}
	// /theme
	items, _ = SlashArgItems("/theme ", data)
	if !has(items, "auto") || !has(items, "light") || !has(items, "graphite") || !has(items, "glacier") {
		t.Errorf("/theme should offer modes and styles; got %v", labelsOf(items))
	}
	// a non-structured command yields nothing
	if items, _ := SlashArgItems("/help ", data); len(items) != 0 {
		t.Errorf("/help should have no arg items; got %v", labelsOf(items))
	}
	// a fully-typed terminal subcommand offers nothing (no lingering no-op) so the
	// caller can submit instead of "accepting" a no-op — the /skills list bug.
	if items, _ := SlashArgItems("/skills list", data); len(items) != 0 {
		t.Errorf("/skills list (token complete) should offer no suggestion; got %v", labelsOf(items))
	}
	// and hidden menu commands stay hidden while direct typed execution remains
	// handled by runSkillSubcommand.
	if items, _ := SlashArgItems("/skills li", data); len(items) != 0 {
		t.Errorf("/skills li should not offer hidden list suggestion; got %v", labelsOf(items))
	}
}

func TestFormatMCPListIncludesConfiguredDisconnectedAndRuntimeState(t *testing.T) {
	got := formatMCPList(
		[]string{"antv-chart", "docs", "playwright"},
		[]plugin.ServerStatus{{Name: "antv-chart", Transport: "stdio", Tools: 27}},
		[]plugin.Failure{{Name: "playwright", Transport: "stdio", Error: "pipe closed"}},
	)
	for _, want := range []string{
		"• antv-chart — connected · stdio · 27 tools",
		"• docs — configured · disconnected",
		"• playwright — failed · stdio",
		"  pipe closed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted MCP list missing %q:\n%s", want, got)
		}
	}
}

func TestFormatMCPListDeduplicatesNamesAndPrefersConnectedState(t *testing.T) {
	got := formatMCPList(
		[]string{"docs", "docs", ""},
		[]plugin.ServerStatus{{Name: "docs", Tools: 5}},
		[]plugin.Failure{{Name: "docs", Error: "stale failure"}},
	)
	if count := strings.Count(got, "• docs —"); count != 1 {
		t.Fatalf("formatted MCP list contains docs %d times, want 1:\n%s", count, got)
	}
	if !strings.Contains(got, "• docs — connected · unknown · 5 tools") {
		t.Fatalf("formatted MCP list missing connected state with transport fallback:\n%s", got)
	}
	if strings.Contains(got, "stale failure") {
		t.Fatalf("connected state should take precedence over stale failure:\n%s", got)
	}
}

func TestSlashArgItemsEffortUsesArgDataLevels(t *testing.T) {
	items, _ := SlashArgItems("/effort ", ArgData{
		CurrentModel: "default-profile/model",
		EffortLevels: []string{"auto", "low"},
	})
	if !has(items, "auto") || !has(items, "low") || has(items, "high") || has(items, "max") {
		t.Fatalf("/effort should use ArgData effort levels only; got %v", labelsOf(items))
	}

	items, _ = SlashArgItems("/effort ", ArgData{
		CurrentModel: "default-profile/model",
	})
	if len(items) != 0 {
		t.Fatalf("/effort should be empty when caller supplies no effort levels; got %v", labelsOf(items))
	}
}

func TestMemoryListTextIncludesSavedMemories(t *testing.T) {
	store := memory.Store{Dir: t.TempDir()}
	if _, err := store.Save(memory.Memory{
		Name:        "cache-first",
		Title:       "Cache first",
		Description: "Preserve prompt cache stability",
		Type:        memory.TypeProject,
		Body:        "Use retrieval tools instead of dynamic prefix injection.",
	}); err != nil {
		t.Fatal(err)
	}
	c := New(Options{Memory: &memory.Set{Store: store}})
	out := c.memoryListText()
	for _, want := range []string{"saved memories", "[Cache first](cache-first.md)", "Preserve prompt cache stability"} {
		if !strings.Contains(out, want) {
			t.Fatalf("/memory output missing %q:\n%s", want, out)
		}
	}
}

func TestMemoryListTextIncludesArchivedMemories(t *testing.T) {
	store := memory.Store{Dir: t.TempDir()}
	if _, err := store.Save(memory.Memory{
		Name:        "stale-plan",
		Title:       "Stale plan",
		Description: "Superseded by the new retrieval design",
		Type:        memory.TypeProject,
		Body:        "Old plan body.",
	}); err != nil {
		t.Fatal(err)
	}
	archive, err := store.Archive("stale-plan")
	if err != nil {
		t.Fatal(err)
	}
	c := New(Options{Memory: &memory.Set{Store: store}})
	out := c.memoryListText()
	for _, want := range []string{"archived memories", "[Stale plan](" + archive + ")", "Superseded by the new retrieval design"} {
		if !strings.Contains(out, want) {
			t.Fatalf("/memory output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "saved memories\n  [Stale plan]") {
		t.Fatalf("archived memory should not appear as active saved memory:\n%s", out)
	}
}

func TestMemoryListTextHidesProfileScopedSharedProjectMemories(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "workspace")
	userDir := filepath.Join(root, "profiles", "alpha")
	store := memory.StoreForProjectRoot(userDir, root, project)
	if _, err := store.Save(memory.Memory{
		Name:        "alpha-user",
		Title:       "Alpha user",
		Description: "current profile user",
		Type:        memory.TypeUser,
		Body:        "Current profile user body.",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(memory.Memory{
		Name:        "shared-project",
		Title:       "Shared project",
		Description: "workspace shared project",
		Type:        memory.TypeProject,
		Body:        "Shared project body.",
	}); err != nil {
		t.Fatal(err)
	}
	writeSlashMemoryFact(t, store.Dir, "leaked-user", memory.TypeUser, "other profile user")
	writeSlashArchivedMemoryFact(t, store.Dir, "20260102-010000.000-leaked-archived.md", "leaked-archived", memory.TypeFeedback, "other profile archived")
	writeSlashArchivedMemoryFact(t, store.Dir, "20260102-020000.000-shared-archived.md", "shared-archived", memory.TypeProject, "workspace shared archived")

	c := New(Options{Memory: &memory.Set{Store: store, CWD: project, UserDir: userDir, ProjectStoreRoot: root}})
	out := c.memoryListText()
	for _, want := range []string{"Alpha user", "Shared project", "shared-archived"} {
		if !strings.Contains(out, want) {
			t.Fatalf("/memory output missing visible memory %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"leaked-user", "other profile user", "leaked-archived", "other profile archived"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("/memory output leaked %q:\n%s", forbidden, out)
		}
	}
}

func writeSlashMemoryFact(t *testing.T, dir, name string, typ memory.Type, desc string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\ntype: " + string(typ) + "\n---\n\n" + desc + " body\n"
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSlashArchivedMemoryFact(t *testing.T, dir, file, name string, typ memory.Type, desc string) {
	t.Helper()
	archiveDir := filepath.Join(dir, ".archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\ntype: " + string(typ) + "\n---\n\n" + desc + " body\n"
	if err := os.WriteFile(filepath.Join(archiveDir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManagementMCPTrustUsesWorkspaceRoot(t *testing.T) {
	isolateControlConfigHome(t)
	project := t.TempDir()

	c := New(Options{WorkspaceRoot: project})
	if !c.managementNotice("/mcp trust-project") {
		t.Fatal("/mcp trust-project was not handled")
	}
	path := config.NewProfilePathResolver(userctx.Default(project)).ProjectTrustPath(project)
	if !(projecttrust.Store{Path: path}).IsTrusted(project) {
		t.Fatal("/mcp trust-project did not trust the controller workspace root")
	}
}

func TestManagementMCPTrustUsesActorProfileTrustPath(t *testing.T) {
	isolateControlConfigHome(t)
	project := t.TempDir()
	actorA := userctx.Default(project)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(project)
	actorB.ProfileID = "profile-b"

	c := New(Options{WorkspaceRoot: project, Actor: actorA})
	if !c.managementNotice("/mcp trust-project") {
		t.Fatal("/mcp trust-project was not handled")
	}
	pathA := config.NewProfilePathResolver(actorA).ProjectTrustPath(project)
	pathB := config.NewProfilePathResolver(actorB).ProjectTrustPath(project)
	if !(projecttrust.Store{Path: pathA}).IsTrusted(project) {
		t.Fatal("/mcp trust-project did not write active actor profile trust")
	}
	if (projecttrust.Store{Path: pathB}).IsTrusted(project) {
		t.Fatal("profile B should not inherit profile A project trust")
	}
	defaultPath := config.NewProfilePathResolver(userctx.Default(project)).ProjectTrustPath(project)
	if (projecttrust.Store{Path: defaultPath}).IsTrusted(project) {
		t.Fatal("non-default profile trust should not write default profile trust")
	}
}

func TestManagementModelUsesActorProfileConfig(t *testing.T) {
	root := isolateControlConfigHome(t)
	t.Setenv("KWU_HOME", filepath.Join(root, "home"))
	t.Setenv("KWU_STATE_HOME", filepath.Join(root, "state"))
	project := t.TempDir()

	defaultActor := userctx.Default(project)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	defaultCfg := config.Default()
	defaultCfg.Providers = []config.ProviderEntry{{
		Name:    "default-only",
		Kind:    "openai",
		BaseURL: "http://localhost:4111/v1",
		Model:   "default-chat",
	}}
	if err := defaultCfg.SaveToScope(defaultPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	actor := userctx.Default(project)
	actor.ProfileID = "work"
	paths := config.NewProfilePathResolver(actor)
	profileCfg := config.Default()
	profileCfg.Providers = []config.ProviderEntry{{
		Name:    "profile-only",
		Kind:    "openai",
		BaseURL: "http://localhost:4222/v1",
		Model:   "profile-chat",
	}}
	if err := profileCfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save profile config: %v", err)
	}

	var notices []string
	c := New(Options{
		WorkspaceRoot: project,
		Actor:         actor,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.Notice {
				notices = append(notices, e.Text)
			}
		}),
	})

	if !c.managementNotice("/model") {
		t.Fatal("/model was not handled")
	}
	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "profile-only/profile-chat") {
		t.Fatalf("/model output missing profile provider:\n%s", joined)
	}
	if strings.Contains(joined, "default-only/default-chat") {
		t.Fatalf("/model output leaked default provider:\n%s", joined)
	}

	notices = nil
	if !c.managementNotice("/provider") {
		t.Fatal("/provider was not handled")
	}
	joined = strings.Join(notices, "\n")
	if !strings.Contains(joined, "profile-only") {
		t.Fatalf("/provider output missing profile provider:\n%s", joined)
	}
	if strings.Contains(joined, "default-only") {
		t.Fatalf("/provider output leaked default provider:\n%s", joined)
	}

	notices = nil
	if !c.managementNotice("/provider profile-only") {
		t.Fatal("/provider profile-only was not handled")
	}
	joined = strings.Join(notices, "\n")
	if !strings.Contains(joined, "profile-chat") {
		t.Fatalf("/provider <name> output missing profile model:\n%s", joined)
	}
	if strings.Contains(joined, "default-chat") {
		t.Fatalf("/provider <name> output leaked default model:\n%s", joined)
	}
}

func TestManagementMigrateEmitsProgress(t *testing.T) {
	isolateControlConfigHome(t)
	var notices []string
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e.Text)
		}
	})})

	if !c.managementNotice("/migrate") {
		t.Fatal("/migrate was not handled")
	}
	joined := strings.Join(notices, "\n")
	for _, want := range []string{
		"migration rescue: checking legacy config and credentials",
		"migration rescue: scanning legacy memory",
		"migration rescue: scanning legacy sessions",
		"migration rescue complete:",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing notice %q in:\n%s", want, joined)
		}
	}
}
