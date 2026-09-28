//go:build !windows

package main

import "kwu/internal/automation"

func probeDesktopAutomationOwner(string) automation.OwnerState {
	return automation.OwnerUnknown
}
