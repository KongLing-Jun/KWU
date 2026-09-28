package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"kwu/internal/control"
	"kwu/internal/expert"
)

type desktopExpertRuntime interface {
	UserExpertStorePath() string
	CurrentExpert() expert.State
	Running() bool
	ClearExpert() (expert.State, error)
	ReloadExpertCatalog()
	RefreshSelectedExpert(id string) error
}

type desktopUserExpertStore interface {
	UserExpertStorePath() string
	ReloadExpertCatalog()
}

func desktopUserExpertStoreFor(ctrl control.SessionAPI) (desktopUserExpertStore, error) {
	store, ok := ctrl.(desktopUserExpertStore)
	if !ok {
		return nil, fmt.Errorf("desktop user expert store is unavailable")
	}
	return store, nil
}

func (a *App) expertRuntimesFor(ctrl control.SessionAPI) []desktopExpertRuntime {
	target, ok := ctrl.(desktopExpertRuntime)
	if !ok {
		return nil
	}
	targetPath := target.UserExpertStorePath()
	a.mu.RLock()
	defer a.mu.RUnlock()
	runtimeTabs := a.runtimeTabsLocked()
	runtimes := make([]desktopExpertRuntime, 0, len(runtimeTabs))
	seen := map[desktopExpertRuntime]struct{}{}
	for _, tab := range runtimeTabs {
		runtime, ok := tab.Ctrl.(desktopExpertRuntime)
		if !ok || !sameExpertStorePath(runtime.UserExpertStorePath(), targetPath) {
			continue
		}
		if _, duplicate := seen[runtime]; duplicate {
			continue
		}
		seen[runtime] = struct{}{}
		runtimes = append(runtimes, runtime)
	}
	return runtimes
}

func sameExpertStorePath(a, b string) bool {
	a = filepath.Clean(strings.TrimSpace(a))
	b = filepath.Clean(strings.TrimSpace(b))
	return a != "." && b != "." && strings.EqualFold(a, b)
}

func ensureExpertRuntimesIdle(runtimes []desktopExpertRuntime, id string, clearAll bool) error {
	for _, runtime := range runtimes {
		selectedID := runtime.CurrentExpert().SelectedID
		if !clearAll && selectedID != id {
			continue
		}
		if selectedID != "" && runtime.Running() {
			return fmt.Errorf("cannot change expert %q while it is running in another tab", selectedID)
		}
	}
	return nil
}

func clearExpertRuntimes(runtimes []desktopExpertRuntime, id string, clearAll bool) error {
	for _, runtime := range runtimes {
		selectedID := runtime.CurrentExpert().SelectedID
		if selectedID == "" || (!clearAll && selectedID != id) {
			continue
		}
		if _, err := runtime.ClearExpert(); err != nil {
			return err
		}
	}
	return nil
}

func reloadExpertRuntimes(runtimes []desktopExpertRuntime, except control.SessionAPI) {
	for _, runtime := range runtimes {
		if any(runtime) == any(except) {
			continue
		}
		runtime.ReloadExpertCatalog()
	}
}

func refreshExpertRuntimes(runtimes []desktopExpertRuntime, id string, except control.SessionAPI) error {
	for _, runtime := range runtimes {
		if any(runtime) == any(except) {
			continue
		}
		if err := runtime.RefreshSelectedExpert(id); err != nil {
			return err
		}
	}
	return nil
}
