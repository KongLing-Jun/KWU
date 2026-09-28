package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"kwu/internal/config"
)

// crash_pending.go captures Go-side panics to disk and ships them on the next
// launch when a real reporting service is configured. Frontend crashes remain
// local and copyable; unavailable reporting discards this temporary upload file.

const pendingCrashFile = "crash-pending.json"

func pendingCrashPath() string {
	return filepath.Join(config.MemoryUserDir(), pendingCrashFile)
}

// recoverToPending records a panicking goroutine to the pending-crash file and
// re-raises, so the process still crashes exactly as before — the stack is now
// available for the next-launch reporting check instead of being lost.
func (a *App) recoverToPending(site string) {
	r := recover()
	if r == nil {
		return
	}
	recordGoPanicFailure(site)
	writePendingCrash(site, r, debug.Stack())
	panic(r)
}

func writePendingCrash(site string, r any, stack []byte) {
	stackText := string(stack)
	msg := sanitizeCrashText(fmt.Sprintf("[go panic] %s: %v\n\n%s", site, r, stackText), maxCrashDetailBytes)
	report := baseCrashReport("crash")
	report.SchemaVersion = 2
	report.Source = "go"
	report.Label = sanitizeCrashField(site, 64)
	report.ErrorType = sanitizeCrashField(fmt.Sprintf("%T", r), 128)
	report.ErrorMessage = sanitizeCrashText(fmt.Sprint(r), maxCrashFieldBytes)
	topFrame := topFrameFromStack(stackText)
	report.Stack = sanitizeCrashText(stackText, maxCrashStackBytes)
	report.TopFrame = topFrame
	report.Message = msg
	body, err := json.Marshal(report)
	if err != nil {
		return
	}
	path := pendingCrashPath()
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	_ = os.WriteFile(path, body, 0o644)
}

func (a *App) goSafe(site string, fn func()) {
	go func() {
		defer a.recoverToPending(site)
		fn()
	}()
}

// flushPendingCrash drains a Go panic captured on a prior run. It drops the file
// when reporting is unavailable or disabled, and only POSTs to a real endpoint.
func (a *App) flushPendingCrash() {
	if version == "dev" {
		return
	}
	path := pendingCrashPath()
	if !reportEndpointAvailable(crashEndpoint) {
		_ = os.Remove(path)
		return
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	cfg, err := a.desktopNonRouteConfig()
	if err != nil || !cfg.DesktopTelemetry() {
		_ = os.Remove(path)
		return
	}
	var r crashReport
	if json.Unmarshal(body, &r) != nil {
		_ = os.Remove(path)
		return
	}
	c, err := httpClient()
	if err != nil {
		return
	}
	if postCrashReport(a.bootContext(), c, crashEndpoint, r) == nil {
		_ = os.Remove(path)
	}
}
