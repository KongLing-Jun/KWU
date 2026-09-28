// Command kwu-desktop is the Wails shell around the Go kernel: a native
// window hosting a webview frontend, with the Go-side control.Controller bound
// directly to the UI (no HTTP hop 鈥?bindings in, runtime events out). It lives in
// a nested module (kwu/desktop) so the CGO/WebKit desktop build never touches
// the CLI's CGO_ENABLED=0 single-static-binary guarantee, while still importing
// the same internal/* kernel.
package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"kwu/desktop/internal/diagnostics"
	"kwu/internal/brand"
	"kwu/internal/config"

	// Blank imports wire compile-time built-ins into their registries, exactly as
	// cmd/kwu does 鈥?boot.Build resolves providers/tools from these registries.
	_ "kwu/internal/provider/anthropic"
	_ "kwu/internal/provider/openai"
	_ "kwu/internal/tool/builtin"
)

// assets embeds the built frontend. `all:` so dotfiles (e.g. the dist .gitkeep
// that keeps this directive compilable before the first `pnpm build`) are
// included. A real run requires `pnpm build` (or `wails build`) to populate dist.
//
//go:embed all:frontend/dist
var assets embed.FS

// version is injected at build time via `wails build -ldflags "-X main.version=..."`,
// mirroring cmd/kwu/main.go. The auto-updater reads it (App.Version) to compare
// against the published manifest; an un-injected dev build stays "dev" and never
// prompts to update.
var version = "dev"

// channel selects which updater pointer this build polls, injected via
// `-X main.channel=canary`. Default "stable" tracks the public release; "canary"
// tracks the opt-in pre-release line and never crosses over to stable.
var channel = "stable"

const (
	disableWebview2GPUEnv  = "KWU_DESKTOP_DISABLE_WEBVIEW2_GPU"
	linuxDRIRenderNodeGlob = "/dev/dri/renderD*"
	windowsMainWindowClass = "kwuDesktopWindow"
	defaultWindowWidth     = 1240
	defaultWindowHeight    = 800
	minimumWindowWidth     = 1200
	minimumWindowHeight    = 800
)

func windowsWebview2GPUDisabled() bool {
	if raw, ok := os.LookupEnv(disableWebview2GPUEnv); ok {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off", "":
			return false
		}
	}
	return channel == "canary"
}

func linuxWebviewGpuPolicy(pattern string) linux.WebviewGpuPolicy {
	matches, err := filepath.Glob(pattern)
	if err == nil {
		for _, path := range matches {
			f, err := os.OpenFile(path, os.O_RDWR, 0)
			if err == nil {
				_ = f.Close()
				return linux.WebviewGpuPolicyOnDemand
			}
		}
	}
	return linux.WebviewGpuPolicyNever
}

func main() {
	configureDesktopUserDataPolicy(runtime.GOOS, config.UseCanonicalUserDataOnly)
	app, singleInstance, saved, savedOK, err := bootstrapDesktopLaunch(
		initializeDesktopProfileAndMigrate,
		NewApp,
		singleInstanceLock,
		func(app *App) (DesktopWindowState, bool) { return app.loadWindowState() },
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	// Restore saved window size, or fall back to the default.
	width, height := defaultWindowWidth, defaultWindowHeight
	if savedOK {
		if saved.Width > 0 {
			width = saved.Width
		}
		if saved.Height > 0 {
			height = saved.Height
		}
	}

	err = wails.Run(&options.App{
		Title:     brand.DisplayName,
		Width:     width,
		Height:    height,
		MinWidth:  minimumWindowWidth,
		MinHeight: minimumWindowHeight,
		// Match the dark UI shell so the initial webview background doesn't flash
		// white before CSS loads 鈥?particularly visible on WebKitGTK.
		BackgroundColour:   &options.RGBA{R: 26, G: 26, B: 46, A: 255},
		AssetServer:        &assetserver.Options{Assets: assets, Middleware: app.workspaceMediaMiddleware()},
		OnStartup:          app.startup,
		OnDomReady:         app.domReady,
		OnBeforeClose:      app.beforeClose,
		OnShutdown:         app.shutdown,
		Bind:               []any{app},
		SingleInstanceLock: singleInstance,

		// Start hidden 鈥?domReady positions and shows the window after restoring
		// geometry, so the user never sees the default size/position flash.
		StartHidden: true,

		// Native application menu (File > Settings, Edit, Window).
		Menu: app.createAppMenu(),

		// Native OS file drops: the webview withholds dropped files' paths from the
		// HTML drop event, so the frontend (composer) reads them via runtime.OnFileDrop
		// against the --wails-drop-target element instead.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},

		// --- per-platform adaptation (see desktop/README.md for the rationale) ---
		Mac: &mac.Options{
			// Inset traffic-lights over a frameless-feeling header; the frontend
			// leaves a drag region at the top (CSS --wails-draggable).
			TitleBar: mac.TitleBarHiddenInset(),
			// Follow the OS appearance so the title bar matches light/dark system
			// preference instead of being locked to dark.
			Appearance: mac.DefaultAppearance,
		},
		Windows: &windows.Options{
			// Follow the OS theme so the title bar matches light/dark system
			// preference instead of being locked to dark.
			Theme:                windows.SystemDefault,
			WindowClassName:      windowsMainWindowClass,
			WebviewGpuIsDisabled: windowsWebview2GPUDisabled(),
			// An explicit path is useful for isolated desktop integration runs;
			// production keeps Wails' normal per-user data location.
			WebviewUserDataPath: desktopWebViewUserDataPath(),
		},
		Linux: &linux.Options{
			ProgramName: brand.DisplayName,
			// WebKitGTK GPU compositing is inconsistent across distros/drivers and
			// is the one real cross-platform rough edge for a Go+webview stack:
			// "always" can yield blank or flickering webviews on some setups, so
			// we let the webview decide on demand when a render node is usable, and
			// disable acceleration when remote/software-rendered sessions cannot
			// access /dev/dri.
			WebviewGpuPolicy: linuxWebviewGpuPolicy(linuxDRIRenderNodeGlob),
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

func configureDesktopUserDataPolicy(goos string, useCanonicalOnly func()) {
	canonical := goos == "windows" || goos == "darwin"
	desktopCanonicalPaths.Store(canonical)
	if canonical {
		useCanonicalOnly()
	}
}

func initializeDesktopProfileAndMigrate() (desktopProfile, error) {
	profile, err := ensureDesktopProfileInitialized()
	if err != nil {
		return desktopProfile{}, err
	}
	if err := preflightDevelopmentRuntime(profile); err != nil {
		return desktopProfile{}, err
	}
	installDesktopErrorJournal(profile)
	if _, err := config.MigrateLegacyIfNeeded(); err != nil {
		slog.Warn("migrate legacy user data failed; using the legacy path for this run", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "desktop.persistence", "err", err)
	}
	return profile, nil
}

func bootstrapDesktopLaunch(
	initializeProfile func() (desktopProfile, error),
	newApp func() *App,
	createSingleInstanceLock func(*App) (*options.SingleInstanceLock, error),
	loadWindowState func(*App) (DesktopWindowState, bool),
) (*App, *options.SingleInstanceLock, DesktopWindowState, bool, error) {
	if initializeProfile == nil {
		return nil, nil, DesktopWindowState{}, false, fmt.Errorf("desktop profile initializer is unavailable")
	}
	if _, err := initializeProfile(); err != nil {
		return nil, nil, DesktopWindowState{}, false, err
	}
	if newApp == nil {
		return nil, nil, DesktopWindowState{}, false, fmt.Errorf("desktop app factory is unavailable")
	}
	app := newApp()
	if createSingleInstanceLock == nil {
		return nil, nil, DesktopWindowState{}, false, fmt.Errorf("single instance lock factory is unavailable")
	}
	singleInstance, err := createSingleInstanceLock(app)
	if err != nil {
		return nil, nil, DesktopWindowState{}, false, err
	}
	if singleInstance == nil {
		return nil, nil, DesktopWindowState{}, false, fmt.Errorf("single instance lock is unavailable")
	}
	if loadWindowState == nil {
		return app, singleInstance, DesktopWindowState{}, false, nil
	}
	saved, ok := loadWindowState(app)
	return app, singleInstance, saved, ok, nil
}
