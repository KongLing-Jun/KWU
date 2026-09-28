package main

import (
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/scene"
)

func TestPersistedDesktopSceneEntryCompatibility(t *testing.T) {
	known, err := sceneStatusFromPersistedEntry(desktopTabEntry{Scene: "coding", SceneState: "draft"})
	if err != nil || known != (scene.Status{Scene: scene.Coding, State: scene.Draft}) {
		t.Fatalf("known entry = %+v, err=%v", known, err)
	}
	legacy, err := sceneStatusFromPersistedEntry(desktopTabEntry{})
	if err != nil || !legacy.IsLegacy() {
		t.Fatalf("missing entry = %+v, err=%v, want legacy", legacy, err)
	}
	illegal, err := sceneStatusFromPersistedEntry(desktopTabEntry{Scene: "code", SceneState: "locked"})
	if err == nil || !illegal.IsLegacy() {
		t.Fatalf("illegal entry = %+v, err=%v, want diagnosed legacy", illegal, err)
	}
}

func TestDesktopSceneRefreshAndRestartRestoration(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	draft := &WorkspaceTab{ID: "draft", Scene: scene.Coding, SceneState: scene.Draft}
	app.tabRuntimes = map[string]*WorkspaceTab{draft.ID: draft}
	app.tabOrder = []string{draft.ID}
	_, entries, _, _ := app.saveTabsCollectLocked()
	if len(entries) != 1 {
		t.Fatalf("saved tab entries = %d, want 1", len(entries))
	}
	if got, err := sceneStatusFromPersistedEntry(entries[0]); err != nil || got != (scene.Status{Scene: scene.Coding, State: scene.Draft}) {
		t.Fatalf("refreshed Draft scene = %+v err=%v", got, err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "locked-restart.jsonl")
	if err := agent.NewSession("saved system").Save(path); err != nil {
		t.Fatal(err)
	}
	meta := agent.BranchMeta{Scope: "global"}
	if err := meta.SetSceneStatus(scene.Status{Scene: scene.Design, State: scene.Locked}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	restored := &WorkspaceTab{ID: "restored", Scope: "global", SessionPath: path, Scene: scene.Office, SceneState: scene.Draft}
	app.tabRuntimes = map[string]*WorkspaceTab{restored.ID: restored}
	app.tabOrder = []string{restored.ID}
	app.reconcileTabWithPinnedSessionMeta(restored)
	if got := tabSceneStatus(restored); got != (scene.Status{Scene: scene.Design, State: scene.Locked}) {
		t.Fatalf("restart scene = %+v, want BranchMeta Design/Locked", got)
	}
}

func TestSetSceneForTabUsesStrictPerTabDraftState(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	makeTab := func(id string) *WorkspaceTab {
		dir := t.TempDir()
		path := filepath.Join(dir, id+".jsonl")
		exec := agent.New(nil, nil, agent.NewSession("office"), agent.Options{}, event.Discard)
		ctrl := control.New(control.Options{
			Executor:     exec,
			SystemPrompt: "office",
			SessionDir:   dir,
			SessionPath:  path,
			Scene:        scene.Office,
			SceneState:   scene.Draft,
		})
		t.Cleanup(ctrl.Close)
		return &WorkspaceTab{ID: id, Ctrl: ctrl, Ready: true, Scene: scene.Office, SceneState: scene.Draft}
	}
	first := makeTab("first")
	second := makeTab("second")
	app.tabRuntimes = map[string]*WorkspaceTab{first.ID: first, second.ID: second}
	app.tabOrder = []string{first.ID, second.ID}
	app.activeTabID = first.ID

	if _, err := app.SetSceneForTab(first.ID, "code"); err == nil {
		t.Fatal("UI-only code alias crossed the backend boundary")
	}
	updated, err := app.SetSceneForTab(first.ID, "coding")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Scene != "coding" || updated.SceneState != "draft" {
		t.Fatalf("updated tab = %+v", updated)
	}
	if got := tabSceneStatus(second); got != (scene.Status{Scene: scene.Office, State: scene.Draft}) {
		t.Fatalf("second tab scene changed: %+v", got)
	}
}

func TestSaveTabSessionMetaSceneRules(t *testing.T) {
	t.Run("known status is saved", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "known.jsonl")
		tab := &WorkspaceTab{Scene: scene.Design, SceneState: scene.Locked}
		if err := saveTabSessionMeta(tab, path); err != nil {
			t.Fatal(err)
		}
		meta, ok, err := agent.LoadBranchMeta(path)
		if err != nil || !ok {
			t.Fatalf("load metadata: ok=%v err=%v", ok, err)
		}
		if got, err := meta.SceneStatus(); err != nil || got != (scene.Status{Scene: scene.Design, State: scene.Locked}) {
			t.Fatalf("saved scene = %+v, err=%v", got, err)
		}
	})

	t.Run("legacy does not rewrite invalid historical fields", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "legacy.jsonl")
		if err := agent.SaveBranchMeta(path, agent.BranchMeta{Scene: "code", SceneState: "locked"}); err != nil {
			t.Fatal(err)
		}
		tab := &WorkspaceTab{Scene: scene.Unset, SceneState: scene.Legacy, TopicTitle: "updated"}
		if err := saveTabSessionMeta(tab, path); err != nil {
			t.Fatal(err)
		}
		meta, ok, err := agent.LoadBranchMeta(path)
		if err != nil || !ok {
			t.Fatalf("load metadata: ok=%v err=%v", ok, err)
		}
		if meta.Scene != "code" || meta.SceneState != "locked" {
			t.Fatalf("legacy metadata was normalized: %+v", meta)
		}
	})
}
