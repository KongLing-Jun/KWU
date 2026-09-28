//go:build darwin

package main

import "fmt"

// macOS releases currently use the verified manual-download flow. Keeping the
// apply hook closed prevents an available signing certificate from silently
// changing the client update policy.
func applyMac(string) error {
	return fmt.Errorf("macOS automatic update is not supported")
}
