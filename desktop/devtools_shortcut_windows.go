//go:build windows

package main

import "syscall"

const (
	virtualKeyControl = 0x11
	virtualKeyShift   = 0x10
	virtualKeyF12     = 0x7B
	keyEventKeyUp     = 0x0002
)

var keybdEvent = syscall.NewLazyDLL("user32.dll").NewProc("keybd_event")

// Wails v2.12 enables WebView2 devtools in development builds, but its Windows
// accelerator handler opens them only for Ctrl+Shift+F12. The frontend calls
// this helper for a plain F12 press while the app window is focused.
func openDevToolsShortcut() bool {
	keybdEvent.Call(virtualKeyControl, 0, 0, 0)
	keybdEvent.Call(virtualKeyShift, 0, 0, 0)
	keybdEvent.Call(virtualKeyF12, 0, 0, 0)
	keybdEvent.Call(virtualKeyF12, 0, keyEventKeyUp, 0)
	keybdEvent.Call(virtualKeyShift, 0, keyEventKeyUp, 0)
	keybdEvent.Call(virtualKeyControl, 0, keyEventKeyUp, 0)
	return true
}
