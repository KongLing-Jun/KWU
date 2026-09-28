//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestInstallerCommandPassesUnquotedDFlagLast(t *testing.T) {
	cmd := installerCommand(`C:\Temp\clouvia-update-1.exe`, `D:\工具\云智企办 App`, 4242)
	if cmd.SysProcAttr == nil {
		t.Fatal("expected a raw command line forcing the install dir")
	}
	got := cmd.SysProcAttr.CmdLine
	want := `"C:\Temp\clouvia-update-1.exe" /S /UPDATEPID=4242 /D=D:\工具\云智企办 App`
	if got != want {
		t.Fatalf("CmdLine = %q, want %q", got, want)
	}
}

func TestInstallerCommandWithoutDirRemainsSilent(t *testing.T) {
	cmd := installerCommand(`C:\Temp\clouvia-update-1.exe`, "", 4242)
	if cmd.SysProcAttr == nil {
		t.Fatal("expected a raw silent-update command line")
	}
	got := cmd.SysProcAttr.CmdLine
	want := `"C:\Temp\clouvia-update-1.exe" /S /UPDATEPID=4242`
	if got != want {
		t.Fatalf("CmdLine = %q, want %q", got, want)
	}
}

func TestDesktopUninstallRegistryKeysPreferCanonicalIdentity(t *testing.T) {
	want := []string{
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\ClouviaCloudOffice`,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\快务AI快务AI`,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\蹇姟AI蹇姟AI`,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\esengine燧元`,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\ReasonixReasonix`,
	}
	if len(desktopUninstallRegistryKeys) != len(want) {
		t.Fatalf("desktopUninstallRegistryKeys = %v, want %v", desktopUninstallRegistryKeys, want)
	}
	for i := range want {
		if desktopUninstallRegistryKeys[i] != want[i] {
			t.Fatalf("desktopUninstallRegistryKeys[%d] = %q, want %q", i, desktopUninstallRegistryKeys[i], want[i])
		}
	}
}

func TestRegisteredWindowsInstallMatchesCanonicalAndLegacyIdentities(t *testing.T) {
	current := filepath.Join(`D:\Tools With Spaces`, "云智企办")
	for _, keyPath := range desktopUninstallRegistryKeys {
		t.Run(filepath.Base(keyPath), func(t *testing.T) {
			got := registeredWindowsInstallMatches(
				current,
				func(candidate string) (string, string, bool) {
					if candidate != keyPath {
						return "", "", false
					}
					return current, "", true
				},
				func(dir string) bool { return dir == current },
			)
			if !got {
				t.Fatalf("registered install under %q did not match %q", keyPath, current)
			}
		})
	}
}

func TestRegisteredWindowsInstallFallsBackToDisplayIcon(t *testing.T) {
	current := filepath.Join(`D:\Tools With Spaces`, "云智企办")
	got := registeredWindowsInstallMatches(
		current,
		func(candidate string) (string, string, bool) {
			if candidate != desktopUninstallRegistryKeys[0] {
				return "", "", false
			}
			return "", `"` + filepath.Join(current, "clouvia-desktop.exe") + `",0`, true
		},
		func(dir string) bool { return dir == current },
	)
	if !got {
		t.Fatalf("DisplayIcon registration did not match %q", current)
	}
}

func TestRegisteredWindowsInstallRequiresUninstaller(t *testing.T) {
	current := filepath.Join(`D:\Tools With Spaces`, "云智企办")
	got := registeredWindowsInstallMatches(
		current,
		func(string) (string, string, bool) { return current, "", true },
		func(string) bool { return false },
	)
	if got {
		t.Fatal("registered directory without uninstall.exe was treated as installed")
	}
}

func TestRegisteredWindowsInstallRejectsEmptyRegistration(t *testing.T) {
	lookupCalls := 0
	got := registeredWindowsInstallMatches(
		".",
		func(string) (string, string, bool) {
			lookupCalls++
			return "", "", true
		},
		func(string) bool { return true },
	)
	if got {
		t.Fatal("empty InstallLocation and DisplayIcon matched the current directory")
	}
	if lookupCalls != len(desktopUninstallRegistryKeys) {
		t.Fatalf("registration lookups = %d, want %d", lookupCalls, len(desktopUninstallRegistryKeys))
	}
}

func TestWindowsDisplayIconPath(t *testing.T) {
	tests := map[string]string{
		`"D:\Tools\云智企办\clouvia-desktop.exe",0`: `D:\Tools\云智企办\clouvia-desktop.exe`,
		`D:\Tools\云智企办\clouvia-desktop.exe`:     `D:\Tools\云智企办\clouvia-desktop.exe`,
	}
	for input, want := range tests {
		if got := windowsDisplayIconPath(input); got != want {
			t.Errorf("windowsDisplayIconPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSameWindowsDirectoryIgnoresCaseAndCleansPath(t *testing.T) {
	dir := t.TempDir()
	if !sameWindowsDirectory(dir, filepath.Join(strings.ToUpper(dir), ".")) {
		t.Fatalf("sameWindowsDirectory should match %q case-insensitively", dir)
	}
}

func TestWindowsUninstallerExistsRequiresFile(t *testing.T) {
	dir := t.TempDir()
	if windowsUninstallerExists(dir) {
		t.Fatal("directory without uninstall.exe must be treated as portable")
	}
	if err := os.WriteFile(filepath.Join(dir, "uninstall.exe"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !windowsUninstallerExists(dir) {
		t.Fatal("registered directory with uninstall.exe must be eligible for installed update checks")
	}
}

func TestWriteAtomicPreservesLockedDestination(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cached-update.exe")
	const original = "verified old artifact"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	targetUTF16, err := windows.UTF16PtrFromString(target)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		targetUTF16,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("lock destination: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = windows.CloseHandle(handle)
		}
	})

	if err := writeAtomic(target, []byte("replacement"), 0o600); err == nil {
		t.Fatal("writeAtomic succeeded while the destination was exclusively locked")
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatalf("unlock destination: %v", err)
	}
	closed = true

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("locked destination = %q, want original content", data)
	}
	temporary, err := filepath.Glob(filepath.Join(dir, "."+filepath.Base(target)+".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 {
		t.Fatalf("writeAtomic left temporary files after lock failure: %v", temporary)
	}
}

func TestApplyWindowsFileReportsLaunchFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-installer.exe")
	if err := applyWindowsFile(missing); err == nil {
		t.Fatal("applyWindowsFile succeeded for a missing installer")
	}
}
