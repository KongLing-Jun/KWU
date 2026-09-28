package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/control"
)

func persistTestTabState(t *testing.T, app *App, activeTabID string) {
	t.Helper()
	app.mu.Lock()
	app.setActiveTabIDLocked(activeTabID)
	app.saveTabsLocked()
	app.mu.Unlock()
}

func TestDesktopTabStoreRoundTripAndRejectsStaleSnapshot(t *testing.T) {
	dir := t.TempDir()
	store := newDesktopTabStore(dir)
	first := desktopTabsFile{
		Tabs:      []desktopTabEntry{{ID: "a", Scope: "global", TopicID: "topic-a"}},
		ActiveTab: "a",
		Version:   1,
	}
	if written, err := store.replace(first); err != nil || !written {
		t.Fatalf("replace first = %v, %v", written, err)
	}
	second := desktopTabsFile{
		Tabs:      []desktopTabEntry{{ID: "b", Scope: "project", WorkspaceRoot: t.TempDir(), TopicID: "topic-b"}},
		ActiveTab: "b",
		Version:   2,
	}
	if written, err := store.replace(second); err != nil || !written {
		t.Fatalf("replace second = %v, %v", written, err)
	}
	if written, err := store.replace(first); err != nil || written {
		t.Fatalf("replace stale = %v, %v", written, err)
	}

	got, err := store.load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || got.ActiveTab != "b" || len(got.Tabs) != 1 || got.Tabs[0].ID != "b" {
		t.Fatalf("loaded state = %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, tabsDatabaseName)); err != nil {
		t.Fatalf("tabs database missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyTabsFileName)); !os.IsNotExist(err) {
		t.Fatalf("unexpected JSON persistence file: %v", err)
	}
}

func TestDesktopTabStorePersistsDefaultToolApprovalMode(t *testing.T) {
	store := newDesktopTabStore(t.TempDir())
	if mode, ok, err := store.defaultToolApprovalMode(); err != nil || ok || mode != "" {
		t.Fatalf("initial default tool approval mode = %q, %v, %v", mode, ok, err)
	}
	if err := store.setDefaultToolApprovalMode(control.ToolApprovalAuto); err != nil {
		t.Fatal(err)
	}
	if mode, ok, err := store.defaultToolApprovalMode(); err != nil || !ok || mode != control.ToolApprovalAuto {
		t.Fatalf("persisted default tool approval mode = %q, %v, %v", mode, ok, err)
	}
	if err := store.setDefaultToolApprovalMode(control.ToolApprovalYolo); err != nil {
		t.Fatal(err)
	}
	if mode, ok, err := newDesktopTabStore(store.dir).defaultToolApprovalMode(); err != nil || !ok || mode != control.ToolApprovalYolo {
		t.Fatalf("reopened default tool approval mode = %q, %v, %v", mode, ok, err)
	}
}

func TestDesktopTabStoreMigratesLegacyJSONOnce(t *testing.T) {
	dir := t.TempDir()
	legacy := desktopTabsFile{
		Tabs:      []desktopTabEntry{{ID: "legacy", Scope: "global", TopicID: "topic-legacy"}},
		ActiveTab: "legacy",
	}
	body, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(dir, legacyTabsFileName)
	if err := os.WriteFile(legacyPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	got := loadDesktopTabsFromDir(dir)
	if got.ActiveTab != "legacy" || len(got.Tabs) != 1 || got.Tabs[0].ID != "legacy" {
		t.Fatalf("migrated state = %+v", got)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy JSON was not removed: %v", err)
	}
	reloaded := loadDesktopTabsFromDir(dir)
	if reloaded.ActiveTab != "legacy" || len(reloaded.Tabs) != 1 || reloaded.Tabs[0].ID != "legacy" {
		t.Fatalf("reloaded state = %+v", reloaded)
	}
}

func TestDesktopTabStoreActivateTabUsesCompareAndSwap(t *testing.T) {
	dir := t.TempDir()
	store := newDesktopTabStore(dir)
	state := desktopTabsFile{
		Tabs:      []desktopTabEntry{{ID: "a", Scope: "global"}, {ID: "b", Scope: "global"}},
		ActiveTab: "a",
		Version:   7,
	}
	if written, err := store.replace(state); err != nil || !written {
		t.Fatalf("seed tabs = %v, %v", written, err)
	}

	activation, err := store.activateTab("a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if activation.PreviousID != "a" || activation.ActiveID != "b" || activation.Version != 8 {
		t.Fatalf("activation = %+v", activation)
	}
	if _, err := store.activateTab("a", "b"); err == nil {
		t.Fatal("stale activity transition was accepted")
	}
	loaded, err := store.load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ActiveTab != "b" || loaded.Version != 8 || len(loaded.Tabs) != 2 {
		t.Fatalf("loaded state = %+v", loaded)
	}
}

func TestActiveTabUsesDatabaseAsSourceOfTruthAfterRestore(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	tabA := &WorkspaceTab{ID: "a", Scope: "global"}
	tabB := &WorkspaceTab{ID: "b", Scope: "global"}
	app.tabRuntimes = map[string]*WorkspaceTab{tabA.ID: tabA, tabB.ID: tabB}
	app.tabOrder = []string{tabA.ID, tabB.ID}
	app.activeTabID = tabA.ID
	app.mu.Lock()
	app.saveTabsLocked()
	version := app.tabsSaveVersion.Load()
	app.mu.Unlock()

	state := loadTabsFileFromDir(app.desktopConfigDir())
	state.ActiveTab = tabB.ID
	state.Version = version + 1
	if written, err := newDesktopTabStore(app.desktopConfigDir()).replace(state); err != nil || !written {
		t.Fatalf("replace active tab = %v, %v", written, err)
	}
	if got := app.activeTab(); got != tabB {
		t.Fatalf("active tab = %v, want database-selected tab b", got)
	}
}
