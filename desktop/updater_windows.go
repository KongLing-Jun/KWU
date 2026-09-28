//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var desktopUninstallRegistryKeys = []string{
	`Software\Microsoft\Windows\CurrentVersion\Uninstall\ClouviaCloudOffice`,
	`Software\Microsoft\Windows\CurrentVersion\Uninstall\快务AI快务AI`,
	`Software\Microsoft\Windows\CurrentVersion\Uninstall\蹇姟AI蹇姟AI`,
	`Software\Microsoft\Windows\CurrentVersion\Uninstall\esengine燧元`,
	`Software\Microsoft\Windows\CurrentVersion\Uninstall\ReasonixReasonix`,
}

// installerCommand runs the NSIS updater silently and passes the current process
// ID so the installer can wait for this application to exit before overwriting
// it. NSIS requires /D= to be the final, unquoted token taken verbatim to the end
// of the line, so the raw command line is set directly — exec.Command would quote
// a path containing spaces (e.g. C:\Users\Jane Doe\...) and NSIS would then
// mis-parse the target directory.
func installerCommand(name, dir string, parentPID int) *exec.Cmd {
	cmd := exec.Command(name)
	cmdLine := fmt.Sprintf(`"%s" /S /UPDATEPID=%d`, name, parentPID)
	if dir != "" {
		cmdLine += fmt.Sprintf(` /D=%s`, dir)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine}
	return cmd
}

func runningFromRegisteredInstall() bool {
	return registeredWindowsInstallMatches(
		currentInstallDir(),
		readDesktopInstallRegistration,
		windowsUninstallerExists,
	)
}

func registeredWindowsInstallMatches(
	current string,
	lookup func(string) (installDir, displayIcon string, ok bool),
	uninstallerExists func(string) bool,
) bool {
	if current == "" {
		return false
	}
	for _, keyPath := range desktopUninstallRegistryKeys {
		installDir, displayIcon, ok := lookup(keyPath)
		if !ok {
			continue
		}
		if installDir == "" {
			if strings.TrimSpace(displayIcon) == "" {
				continue
			}
			installDir = filepath.Dir(windowsDisplayIconPath(displayIcon))
		}
		if sameWindowsDirectory(current, installDir) && uninstallerExists(current) {
			return true
		}
	}
	return false
}

func readDesktopInstallRegistration(keyPath string) (installDir, displayIcon string, ok bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", "", false
	}
	defer key.Close()
	installDir, _, _ = key.GetStringValue("InstallLocation")
	displayIcon, _, _ = key.GetStringValue("DisplayIcon")
	return installDir, displayIcon, true
}

func windowsUninstallerExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "uninstall.exe"))
	return err == nil && !info.IsDir()
}

func windowsDisplayIconPath(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, `"`) {
		if end := strings.Index(value[1:], `"`); end >= 0 {
			return value[1 : end+1]
		}
	}
	if comma := strings.LastIndex(value, ","); comma >= 0 {
		value = value[:comma]
	}
	return strings.Trim(strings.TrimSpace(value), `"`)
}

func sameWindowsDirectory(left, right string) bool {
	left = normalizedWindowsDirectory(left)
	right = normalizedWindowsDirectory(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func normalizedWindowsDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if expanded, err := registry.ExpandString(path); err == nil {
		path = expanded
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		return ""
	}
	return filepath.Clean(path)
}
