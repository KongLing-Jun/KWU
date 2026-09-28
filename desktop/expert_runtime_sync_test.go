package main

import (
	"testing"

	"kwu/internal/control"
	"kwu/internal/expert"
)

type fakeDesktopExpertSession struct {
	control.SessionAPI
	runtime *fakeDesktopExpertRuntime
}

func (f *fakeDesktopExpertSession) UserExpertStorePath() string {
	return f.runtime.UserExpertStorePath()
}
func (f *fakeDesktopExpertSession) CurrentExpert() expert.State { return f.runtime.CurrentExpert() }
func (f *fakeDesktopExpertSession) Running() bool               { return f.runtime.Running() }
func (f *fakeDesktopExpertSession) ClearExpert() (expert.State, error) {
	return f.runtime.ClearExpert()
}
func (f *fakeDesktopExpertSession) ReloadExpertCatalog() { f.runtime.ReloadExpertCatalog() }
func (f *fakeDesktopExpertSession) RefreshSelectedExpert(id string) error {
	return f.runtime.RefreshSelectedExpert(id)
}

type fakeDesktopExpertRuntime struct {
	path          string
	selectedID    string
	running       bool
	clearCalls    int
	reloadCalls   int
	refreshCalls  int
	lastRefreshID string
}

func (f *fakeDesktopExpertRuntime) UserExpertStorePath() string { return f.path }
func (f *fakeDesktopExpertRuntime) CurrentExpert() expert.State {
	return expert.State{SelectedID: f.selectedID}
}
func (f *fakeDesktopExpertRuntime) Running() bool { return f.running }
func (f *fakeDesktopExpertRuntime) ClearExpert() (expert.State, error) {
	f.clearCalls++
	f.selectedID = ""
	return expert.State{}, nil
}
func (f *fakeDesktopExpertRuntime) ReloadExpertCatalog() { f.reloadCalls++ }
func (f *fakeDesktopExpertRuntime) RefreshSelectedExpert(id string) error {
	f.refreshCalls++
	f.lastRefreshID = id
	return nil
}

func TestExpertRuntimeSyncRejectsRunningSelectedExpert(t *testing.T) {
	runtime := &fakeDesktopExpertRuntime{selectedID: "custom-reviewer", running: true}
	if err := ensureExpertRuntimesIdle([]desktopExpertRuntime{runtime}, "custom-reviewer", false); err == nil {
		t.Fatal("ensureExpertRuntimesIdle succeeded for a running selected expert")
	}
}

func TestExpertRuntimeSyncClearsAndRefreshesEveryMatchingTab(t *testing.T) {
	first := &fakeDesktopExpertRuntime{selectedID: "custom-reviewer"}
	second := &fakeDesktopExpertRuntime{selectedID: "custom-reviewer"}
	runtimes := []desktopExpertRuntime{first, second}
	if err := clearExpertRuntimes(runtimes, "custom-reviewer", false); err != nil {
		t.Fatalf("clearExpertRuntimes: %v", err)
	}
	if first.clearCalls != 1 || second.clearCalls != 1 {
		t.Fatalf("clear calls = %d, %d; want every selected tab cleared", first.clearCalls, second.clearCalls)
	}
	if err := refreshExpertRuntimes(runtimes, "custom-reviewer", nil); err != nil {
		t.Fatalf("refreshExpertRuntimes: %v", err)
	}
	if first.refreshCalls != 1 || second.refreshCalls != 1 || first.lastRefreshID != "custom-reviewer" || second.lastRefreshID != "custom-reviewer" {
		t.Fatalf("refresh state = %+v, %+v", first, second)
	}
}

func TestExpertRuntimeSyncIncludesDetachedSessions(t *testing.T) {
	visibleState := &fakeDesktopExpertRuntime{path: `C:\\Users\\A\\experts.json`}
	detachedState := &fakeDesktopExpertRuntime{
		path:       `C:\\Users\\A\\experts.json`,
		selectedID: "custom-reviewer",
		running:    true,
	}
	visible := &fakeDesktopExpertSession{runtime: visibleState}
	detached := &fakeDesktopExpertSession{runtime: detachedState}
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"visible": {ID: "visible", Ctrl: visible},
		},
		detachedSessions: map[string]*WorkspaceTab{
			"detached": {ID: "detached", Ctrl: detached},
		},
	}

	runtimes := app.expertRuntimesFor(visible)
	if err := ensureExpertRuntimesIdle(runtimes, "custom-reviewer", false); err == nil {
		t.Fatal("detached running expert was omitted from the preflight")
	}

	detachedState.running = false
	if err := refreshExpertRuntimes(runtimes, "custom-reviewer", nil); err != nil {
		t.Fatalf("refreshExpertRuntimes: %v", err)
	}
	if detachedState.refreshCalls != 1 || detachedState.lastRefreshID != "custom-reviewer" {
		t.Fatalf("detached refresh state = %+v, want one refresh", detachedState)
	}
	if err := clearExpertRuntimes(runtimes, "custom-reviewer", false); err != nil {
		t.Fatalf("clearExpertRuntimes: %v", err)
	}
	if detachedState.clearCalls != 1 || detachedState.selectedID != "" {
		t.Fatalf("detached clear state = %+v, want cleared selection", detachedState)
	}
}

func TestSameExpertStorePathIsCaseInsensitiveAndRejectsEmpty(t *testing.T) {
	if !sameExpertStorePath(`C:\\Users\\A\\experts.json`, `c:\\users\\a\\experts.json`) {
		t.Fatal("sameExpertStorePath rejected equivalent Windows paths")
	}
	if sameExpertStorePath("", "") {
		t.Fatal("sameExpertStorePath accepted empty paths")
	}
}
