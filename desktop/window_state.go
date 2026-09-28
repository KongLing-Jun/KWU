package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"kwu/internal/config"
)

// DesktopWindowState captures the window geometry to restore across launches.
type DesktopWindowState struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximised bool `json:"maximised"`
}

func windowStatePath() string {
	return config.DesktopWindowStatePath()
}

func windowStatePathForPaths(paths config.ProfilePathResolver) string {
	if paths.ProfileID() == "" {
		return windowStatePath()
	}
	return paths.DesktopWindowStatePath()
}

func (a *App) windowStatePath() string {
	return windowStatePathForPaths(a.currentProfilePathResolver(""))
}

// loadWindowState reads the saved window geometry. The second return value is
// false when no saved state exists (first launch, missing file, corrupt JSON).
// Callers must not restore position when ok is false — zero values are not a
// valid window origin.
func loadWindowState() (DesktopWindowState, bool) {
	if _, err := ensureDesktopProfileInitialized(); err != nil {
		return DesktopWindowState{}, false
	}
	return loadWindowStateFromPath(windowStatePath())
}

func (a *App) loadWindowState() (DesktopWindowState, bool) {
	return loadWindowStateFromPath(a.windowStatePath())
}

func loadWindowStateFromPath(path string) (DesktopWindowState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DesktopWindowState{}, false
	}
	var s DesktopWindowState
	if err := json.Unmarshal(data, &s); err != nil {
		return DesktopWindowState{}, false
	}
	if s.Width < 0 {
		s.Width = 0
	} else if s.Width > 0 && s.Width < minimumWindowWidth {
		s.Width = minimumWindowWidth
	}
	if s.Height < 0 {
		s.Height = 0
	} else if s.Height > 0 && s.Height < minimumWindowHeight {
		s.Height = minimumWindowHeight
	}
	// Migration guard: the previous version saved (0,0) for first-launch zero
	// values. Treat an all-zero valid file the same as missing — let domReady
	// center the window instead of parking it at the screen origin.
	if s.Width == 0 && s.Height == 0 && s.X == 0 && s.Y == 0 {
		return DesktopWindowState{}, false
	}
	return s, true
}

// SaveWindowState is the bound method the frontend calls to persist the current
// window geometry before quit and periodically during use.
func (a *App) SaveWindowState(state DesktopWindowState) error {
	path := a.windowStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// saveWindowStateSync saves the current window geometry from the Go side while
// OnBeforeClose still owns a valid native window. OnShutdown is too late on
// Windows because Wails may already have torn down the window's DPI context.
func (a *App) saveWindowStateSync() {
	if a.ctx == nil {
		return
	}
	w, h := runtime.WindowGetSize(a.ctx)
	x, y := runtime.WindowGetPosition(a.ctx)
	max := runtime.WindowIsMaximised(a.ctx)
	_ = a.SaveWindowState(DesktopWindowState{
		Width:     w,
		Height:    h,
		X:         x,
		Y:         y,
		Maximised: max,
	})
}
