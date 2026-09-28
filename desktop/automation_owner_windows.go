//go:build windows

package main

import (
	"errors"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
	"kwu/internal/automation"
)

// Desktop owner PIDs refer to this machine's profile. A reused PID remains
// alive/unknown, deliberately sacrificing recovery rather than ending its run.
func probeDesktopAutomationOwner(owner string) automation.OwnerState {
	raw, ok := strings.CutPrefix(owner, "desktop:")
	if !ok {
		return automation.OwnerUnknown
	}
	pid, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || pid == 0 {
		return automation.OwnerUnknown
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return automation.OwnerDead
	}
	if err != nil {
		return automation.OwnerUnknown
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return automation.OwnerUnknown
	}
	if status == windows.WAIT_OBJECT_0 {
		return automation.OwnerDead
	}
	if status == uint32(windows.WAIT_TIMEOUT) {
		return automation.OwnerAlive
	}
	return automation.OwnerUnknown
}
