package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/userctx"
)

func TestBuildTabControllerInitialActorContext(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)
	root := t.TempDir()

	cfg := config.Default()
	cfg.DefaultModel = "desktop/initial-model"
	cfg.Providers = []config.ProviderEntry{
		{Name: "desktop", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "initial-model"},
	}
	if err := cfg.SaveTo(filepath.Join(root, "kwu.toml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	tab := &WorkspaceTab{ID: "tab_actor", Scope: "project", WorkspaceRoot: root}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabControllerWithContext(tab, loadedTabSession{}, context.Background(), 0, nil)
	if tab.StartupErr != "" {
		t.Fatalf("StartupErr = %q", tab.StartupErr)
	}
	if tab.Ctrl == nil {
		t.Fatal("expected tab controller")
	}
	defer tab.Ctrl.Close()
	defer app.releaseTabSharedHost(tab)

	actor := tab.Ctrl.ActorContext()
	if actor.ActorID != userctx.DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, userctx.DefaultActorID)
	}
	if actor.ProfileID != userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", actor.ProfileID, userctx.DefaultProfileID)
	}
	if actor.AuthMethod != userctx.DefaultAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, userctx.DefaultAuthMethod)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, root)
	}
}

func TestBuildTabControllerUsesCurrentProfileActorContext(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)
	root := t.TempDir()

	current := userctx.ActorContext{ActorID: "desktop-user", ProfileID: "work", AuthMethod: "local"}
	profileActor := current
	profileActor.WorkspaceRoot = root
	paths := config.NewProfilePathResolver(profileActor)
	cfg := config.Default()
	cfg.DefaultModel = "desktop/current-model"
	cfg.Providers = []config.ProviderEntry{
		{Name: "desktop", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "current-model"},
	}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatalf("save profile config: %v", err)
	}

	app := NewApp()
	app.setCurrentActorContext(current)
	tab := app.createTabEntryWithID("project", root, "", "tab_current_profile")
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabControllerWithContext(tab, loadedTabSession{}, context.Background(), 0, nil)
	if tab.StartupErr != "" {
		t.Fatalf("StartupErr = %q", tab.StartupErr)
	}
	if tab.Ctrl == nil {
		t.Fatal("expected tab controller")
	}
	defer tab.Ctrl.Close()
	defer app.releaseTabSharedHost(tab)

	actor := tab.Ctrl.ActorContext()
	if actor.ActorID != "desktop-user" || actor.ProfileID != "work" || actor.AuthMethod != "local" {
		t.Fatalf("ActorContext = %+v, want current profile actor", actor)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, root)
	}
}

func TestEnsureBlankTabUsesCurrentProfileActorContext(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)

	for _, tc := range []struct {
		name         string
		scope        string
		indexedBlank bool
	}{
		{name: "project new topic", scope: "project"},
		{name: "project indexed blank topic", scope: "project", indexedBlank: true},
		{name: "global new topic", scope: "global"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			actualRoot := root
			if tc.scope == "global" {
				actualRoot = globalWorkspaceRoot()
			}

			current := userctx.ActorContext{ActorID: "desktop-user", ProfileID: "work", AuthMethod: "local"}
			wantActor := current
			wantActor.WorkspaceRoot = actualRoot
			wantActor = wantActor.Normalize()
			paths := config.NewProfilePathResolver(wantActor)
			cfg := config.Default()
			cfg.DefaultModel = "desktop/current-model"
			cfg.Providers = []config.ProviderEntry{
				{Name: "desktop", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "current-model"},
			}
			if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
				t.Fatalf("save profile config: %v", err)
			}

			app := NewApp()
			app.setCurrentActorContext(current)
			t.Cleanup(app.closeAllSharedHosts)
			if tc.indexedBlank {
				topic, err := app.CreateTopic(tc.scope, root, "")
				if err != nil {
					t.Fatalf("create indexed blank topic: %v", err)
				}
				if topic.Title != defaultTopicTitle {
					t.Fatalf("indexed topic title = %q, want %q", topic.Title, defaultTopicTitle)
				}
			}

			meta, err := app.EnsureBlankTab(tc.scope, root)
			if err != nil {
				t.Fatalf("EnsureBlankTab: %v", err)
			}
			app.mu.RLock()
			tab := app.tabRuntimes[meta.ID]
			app.mu.RUnlock()
			if tab == nil {
				t.Fatalf("created tab %q not found", meta.ID)
			}
			wantDir := desktopSessionDirForActor(wantActor, actualRoot)
			defaultDir := desktopSessionDir(actualRoot)
			if filepath.Clean(wantDir) == filepath.Clean(defaultDir) {
				t.Fatalf("test setup error: current profile dir %q matches default dir", wantDir)
			}
			if !actorContextsMatch(tab.actor, wantActor) {
				t.Fatalf("tab actor = %+v, want %+v", tab.actor.Normalize(), wantActor)
			}
			if gotDir := filepath.Dir(tab.SessionPath); filepath.Clean(gotDir) != filepath.Clean(wantDir) {
				t.Fatalf("precreated session dir = %q, want current profile dir %q (default dir %q)", gotDir, wantDir, defaultDir)
			}
			if tab.Ctrl == nil {
				t.Fatalf("expected built controller; startup error: %q", tab.StartupErr)
			}
			t.Cleanup(tab.Ctrl.Close)

			gotActor := tab.Ctrl.ActorContext()
			if !actorContextsMatch(gotActor, wantActor) {
				t.Fatalf("controller actor = %+v, want %+v", gotActor.Normalize(), wantActor)
			}
			if gotDir := tab.Ctrl.SessionDir(); filepath.Clean(gotDir) != filepath.Clean(wantDir) {
				t.Fatalf("controller session dir = %q, want current profile dir %q", gotDir, wantDir)
			}
			wantHostKey := sharedHostKeyForActor(wantActor, actualRoot)
			if tab.SharedHostKey != wantHostKey {
				t.Fatalf("SharedHostKey = %q, want %q", tab.SharedHostKey, wantHostKey)
			}
			defaultHostKey := sharedHostKeyForActor(userctx.Default(actualRoot), actualRoot)
			if tab.SharedHostKey == defaultHostKey {
				t.Fatalf("SharedHostKey used default profile key %q", defaultHostKey)
			}
		})
	}
}

func TestTabSessionDirUsesActorProfileProjectFallbackWhenControllerMissing(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	actor.ProfileID = "work"
	tab := &WorkspaceTab{
		ID:            "tab_profile_project",
		Scope:         "project",
		WorkspaceRoot: root,
		actor:         actor,
	}

	want := config.NewProfilePathResolver(actor).ProjectSessionDir(root)
	if got := tabSessionDir(tab); filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("tabSessionDir = %q, want actor profile project session dir %q", got, want)
	}
}

func TestTabSessionDirUsesActorProfileGlobalFallbackWhenControllerMissing(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	actor.ProfileID = "work"
	tab := &WorkspaceTab{
		ID:    "tab_profile_global",
		Scope: "global",
		actor: actor,
	}

	want := config.NewProfilePathResolver(actor).SessionDir()
	if got := tabSessionDir(tab); filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("tabSessionDir = %q, want actor profile global session dir %q", got, want)
	}
}

func TestSetModelForTabResolvesConfigWithControllerActorContext(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(""), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	actor := userctx.ActorContext{
		ActorID:       "work-user",
		ProfileID:     "work",
		AuthMethod:    "test",
		WorkspaceRoot: root,
	}
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir profile config: %v", err)
	}
	cfg := config.Default()
	cfg.DefaultModel = "desktop/next"
	cfg.Providers = []config.ProviderEntry{
		{Name: "desktop", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "next"},
	}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatalf("save profile config: %v", err)
	}

	oldCtrl := control.New(control.Options{
		Sink:          event.Discard,
		WorkspaceRoot: root,
		SessionDir:    filepath.Join(t.TempDir(), "sessions"),
		Actor:         actor,
	})
	defer oldCtrl.Close()

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{ID: "tab_actor_switch", Scope: "project", WorkspaceRoot: root, model: "desktop/old", Ctrl: oldCtrl, Ready: true}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.SetModelForTab(tab.ID, "desktop/next"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	defer tab.Ctrl.Close()
	defer app.releaseTabSharedHost(tab)

	gotActor := tab.Ctrl.ActorContext()
	if gotActor.ProfileID != "work" || gotActor.ActorID != "work-user" {
		t.Fatalf("rebuilt controller actor = %+v, want work profile actor", gotActor)
	}
	if tab.model != "desktop/next" {
		t.Fatalf("tab model = %q, want desktop/next", tab.model)
	}
}
