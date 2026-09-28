//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
void installKWUSystemQuitHook(void);
*/
import "C"

import "sync"

var installSystemQuitHookOnce sync.Once

func installSystemQuitHook() {
	installSystemQuitHookOnce.Do(func() {
		C.installKWUSystemQuitHook()
	})
}

//export KWUMarkSystemQuit
func KWUMarkSystemQuit() {
	markSystemQuitRequested()
}
