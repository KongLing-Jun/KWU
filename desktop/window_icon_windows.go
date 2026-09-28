//go:build windows

package main

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wailsAppIconResourceID = 3

	imageIcon = 1
	wmSetIcon = 0x0080
	iconSmall = 0
	iconBig   = 1

	smCXIcon   = 11
	smCYIcon   = 12
	smCXSmIcon = 49
	smCYSmIcon = 50
)

var (
	user32DLL                    = windows.NewLazySystemDLL("user32.dll")
	kernel32DLL                  = windows.NewLazySystemDLL("kernel32.dll")
	procFindWindowExW            = user32DLL.NewProc("FindWindowExW")
	procGetDpiForWindow          = user32DLL.NewProc("GetDpiForWindow")
	procGetWindowThreadProcessID = user32DLL.NewProc("GetWindowThreadProcessId")
	procGetSystemMetrics         = user32DLL.NewProc("GetSystemMetrics")
	procGetSystemMetricsForDPI   = user32DLL.NewProc("GetSystemMetricsForDpi")
	procDestroyIcon              = user32DLL.NewProc("DestroyIcon")
	procIsWindow                 = user32DLL.NewProc("IsWindow")
	procLoadImageW               = user32DLL.NewProc("LoadImageW")
	procSendMessageW             = user32DLL.NewProc("SendMessageW")
	procGetModuleHandleW         = kernel32DLL.NewProc("GetModuleHandleW")
)

type mainWindowIconAPI interface {
	findWindow(className string, processID uint32) (uintptr, error)
	windowDPI(window uintptr) uint32
	metricForDPI(index int, dpi uint32) int
	loadIcon(resourceID, width, height int) (uintptr, error)
	setIcon(window, iconType, icon uintptr)
	destroyIcon(icon uintptr)
	isWindow(window uintptr) bool
}

type nativeMainWindowIconAPI struct{}

type mainWindowIconHandles struct {
	small uintptr
	big   uintptr
}

type mainWindowIconState struct {
	window uintptr
	dpi    uint32
	icons  mainWindowIconHandles
}

var (
	mainWindowIconsMu      sync.Mutex
	mainWindowIconsState   mainWindowIconState
	mainWindowWatcherStart sync.Once
)

func setMainWindowIcons() error {
	api := nativeMainWindowIconAPI{}
	window, err := api.findWindow(windowsMainWindowClass, uint32(os.Getpid()))
	if err != nil {
		return fmt.Errorf("find main window: %w", err)
	}
	if window == 0 {
		return fmt.Errorf("main window class %q was not found for process %d", windowsMainWindowClass, os.Getpid())
	}

	mainWindowIconsMu.Lock()
	dpi := normalizedWindowDPI(api, window)
	if mainWindowIconsState.window == window &&
		mainWindowIconsState.dpi == dpi &&
		mainWindowIconsState.icons.small != 0 &&
		mainWindowIconsState.icons.big != 0 {
		// navigationCompleted may trigger OnDomReady again. Re-assert the same
		// owned handles without allocating another pair or watcher.
		api.setIcon(window, iconSmall, mainWindowIconsState.icons.small)
		api.setIcon(window, iconBig, mainWindowIconsState.icons.big)
		mainWindowIconsMu.Unlock()
		startMainWindowIconDPIWatcher(api)
		return nil
	}

	icons, appliedDPI, err := replaceMainWindowIcons(api, window, mainWindowIconsState.window, mainWindowIconsState.icons)
	if err != nil {
		mainWindowIconsMu.Unlock()
		return err
	}
	mainWindowIconsState = mainWindowIconState{window: window, dpi: appliedDPI, icons: icons}
	mainWindowIconsMu.Unlock()
	startMainWindowIconDPIWatcher(api)
	return nil
}

func applyMainWindowIcons(api mainWindowIconAPI, className string, processID uint32) (uintptr, uint32, mainWindowIconHandles, error) {
	window, err := api.findWindow(className, processID)
	if err != nil {
		return 0, 0, mainWindowIconHandles{}, fmt.Errorf("find main window: %w", err)
	}
	if window == 0 {
		return 0, 0, mainWindowIconHandles{}, fmt.Errorf("main window class %q was not found for process %d", className, processID)
	}
	icons, dpi, err := replaceMainWindowIcons(api, window, 0, mainWindowIconHandles{})
	return window, dpi, icons, err
}

func normalizedWindowDPI(api mainWindowIconAPI, window uintptr) uint32 {
	if dpi := api.windowDPI(window); dpi != 0 {
		return dpi
	}
	return 96
}

func loadMainWindowIconHandles(api mainWindowIconAPI, window uintptr) (mainWindowIconHandles, uint32, error) {
	dpi := normalizedWindowDPI(api, window)

	small, err := loadMainWindowIcon(api, api.metricForDPI(smCXSmIcon, dpi), api.metricForDPI(smCYSmIcon, dpi))
	if err != nil {
		return mainWindowIconHandles{}, dpi, fmt.Errorf("load small icon: %w", err)
	}
	big, err := loadMainWindowIcon(api, api.metricForDPI(smCXIcon, dpi), api.metricForDPI(smCYIcon, dpi))
	if err != nil {
		api.destroyIcon(small)
		return mainWindowIconHandles{}, dpi, fmt.Errorf("load big icon: %w", err)
	}
	return mainWindowIconHandles{small: small, big: big}, dpi, nil
}

func replaceMainWindowIcons(
	api mainWindowIconAPI,
	window uintptr,
	previousWindow uintptr,
	previous mainWindowIconHandles,
) (mainWindowIconHandles, uint32, error) {
	icons, dpi, err := loadMainWindowIconHandles(api, window)
	if err != nil {
		return mainWindowIconHandles{}, dpi, err
	}
	if previousWindow != 0 && previousWindow != window && api.isWindow(previousWindow) {
		api.setIcon(previousWindow, iconSmall, 0)
		api.setIcon(previousWindow, iconBig, 0)
	}
	// Wails v2 only assigns ICON_SMALL. Assign both handles so the title bar and
	// taskbar/Alt+Tab select the matching frame from the same ICO resource.
	api.setIcon(window, iconSmall, icons.small)
	api.setIcon(window, iconBig, icons.big)
	destroyMainWindowIconHandles(api, previous)
	return icons, dpi, nil
}

func destroyMainWindowIconHandles(api mainWindowIconAPI, icons mainWindowIconHandles) {
	if icons.small != 0 {
		api.destroyIcon(icons.small)
	}
	if icons.big != 0 && icons.big != icons.small {
		api.destroyIcon(icons.big)
	}
}

// Windows does not resize a WM_SETICON handle when a PerMonitorV2 window moves
// to a monitor with a different scale. One process-lifetime watcher replaces
// the owned HICON pair when the target DPI changes and releases it after the
// native window is destroyed.
func startMainWindowIconDPIWatcher(api mainWindowIconAPI) {
	mainWindowWatcherStart.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for range ticker.C {
				mainWindowIconsMu.Lock()
				state := mainWindowIconsState
				if state.window == 0 {
					mainWindowIconsMu.Unlock()
					continue
				}
				if !api.isWindow(state.window) {
					mainWindowIconsState = mainWindowIconState{}
					mainWindowIconsMu.Unlock()
					destroyMainWindowIconHandles(api, state.icons)
					continue
				}
				if normalizedWindowDPI(api, state.window) == state.dpi {
					mainWindowIconsMu.Unlock()
					continue
				}
				icons, dpi, err := replaceMainWindowIcons(api, state.window, state.window, state.icons)
				if err == nil {
					mainWindowIconsState = mainWindowIconState{window: state.window, dpi: dpi, icons: icons}
				}
				mainWindowIconsMu.Unlock()
			}
		}()
	})
}

func loadMainWindowIcon(api mainWindowIconAPI, width, height int) (uintptr, error) {
	if width <= 0 || height <= 0 {
		return 0, fmt.Errorf("invalid icon size %dx%d", width, height)
	}
	return api.loadIcon(wailsAppIconResourceID, width, height)
}

func (nativeMainWindowIconAPI) findWindow(className string, processID uint32) (uintptr, error) {
	classNamePtr, err := windows.UTF16PtrFromString(className)
	if err != nil {
		return 0, err
	}

	var after uintptr
	for {
		window, _, _ := procFindWindowExW.Call(0, after, uintptr(unsafe.Pointer(classNamePtr)), 0)
		if window == 0 {
			return 0, nil
		}
		var ownerProcessID uint32
		procGetWindowThreadProcessID.Call(window, uintptr(unsafe.Pointer(&ownerProcessID)))
		if ownerProcessID == processID {
			return window, nil
		}
		after = window
	}
}

func (nativeMainWindowIconAPI) windowDPI(window uintptr) uint32 {
	if err := procGetDpiForWindow.Find(); err != nil {
		return 96
	}
	dpi, _, _ := procGetDpiForWindow.Call(window)
	return uint32(dpi)
}

func (nativeMainWindowIconAPI) metricForDPI(index int, dpi uint32) int {
	if dpi != 0 && procGetSystemMetricsForDPI.Find() == nil {
		value, _, _ := procGetSystemMetricsForDPI.Call(uintptr(index), uintptr(dpi))
		if value != 0 {
			return int(value)
		}
	}
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(value)
}

func (nativeMainWindowIconAPI) loadIcon(resourceID, width, height int) (uintptr, error) {
	instance, _, callErr := procGetModuleHandleW.Call(0)
	if instance == 0 {
		return 0, win32CallError("GetModuleHandleW", callErr)
	}
	icon, _, callErr := procLoadImageW.Call(
		instance,
		uintptr(resourceID),
		imageIcon,
		uintptr(width),
		uintptr(height),
		0,
	)
	if icon == 0 {
		return 0, win32CallError("LoadImageW", callErr)
	}
	return icon, nil
}

func (nativeMainWindowIconAPI) setIcon(window, iconType, icon uintptr) {
	procSendMessageW.Call(window, wmSetIcon, iconType, icon)
}

func (nativeMainWindowIconAPI) destroyIcon(icon uintptr) {
	procDestroyIcon.Call(icon)
}

func (nativeMainWindowIconAPI) isWindow(window uintptr) bool {
	valid, _, _ := procIsWindow.Call(window)
	return valid != 0
}

func win32CallError(name string, err error) error {
	if err == nil || err == windows.ERROR_SUCCESS {
		return fmt.Errorf("%s failed", name)
	}
	return fmt.Errorf("%s failed: %w", name, err)
}
