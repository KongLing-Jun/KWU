package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGoal2E2EResultRemovesSensitiveArtifactsOnRenameOrACLFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		rename   func(string, string) error
		restrict func(string) error
	}{
		{name: "rename", rename: func(string, string) error { return errors.New("injected rename failure") }, restrict: restrictGoal2SensitiveFile},
		{name: "final ACL", rename: os.Rename, restrict: func(string) error { return errors.New("injected ACL failure") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalRename, originalRestrict := renameGoal2ResultFile, restrictGoal2ResultFile
			renameGoal2ResultFile, restrictGoal2ResultFile = test.rename, test.restrict
			t.Cleanup(func() { renameGoal2ResultFile, restrictGoal2ResultFile = originalRename, originalRestrict })
			path := filepath.Join(t.TempDir(), "result", "desktop-result.json")
			if err := writeGoal2E2EResult(path, map[string]any{"workspace_path": `C:\sensitive\workspace`}); err == nil {
				t.Fatal("injected failure unexpectedly succeeded")
			}
			for _, residual := range []string{path + ".tmp", path} {
				if _, err := os.Stat(residual); !os.IsNotExist(err) {
					t.Fatalf("sensitive artifact remained after %s failure: %s err=%v", test.name, residual, err)
				}
			}
		})
	}
}

func TestGoal2E2EResultRestrictsDirectoryBeforeSensitiveWrite(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows ACL acceptance")
	}
	directory := filepath.Join(t.TempDir(), "result")
	path := filepath.Join(directory, "desktop-result.json")
	if err := writeGoal2E2EResult(path, map[string]any{"workspace_path": `C:\sensitive\workspace`}); err != nil {
		t.Fatal(err)
	}
	assertGoal2OnlyCurrentUserACL(t, directory)
	assertGoal2OnlyCurrentUserACL(t, path)
}

func assertGoal2OnlyCurrentUserACL(t *testing.T, path string) {
	t.Helper()
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	script := `$securityModule=Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1'; Import-Module $securityModule -ErrorAction Stop; $acl=Get-Acl -LiteralPath $env:GOAL2_ACL_PATH; $current=[System.Security.Principal.WindowsIdentity]::GetCurrent().Name; $bad=@($acl.Access | Where-Object { $_.IsInherited -or $_.AccessControlType -ne 'Allow' -or $_.IdentityReference.Value -ine $current -or (($_.FileSystemRights -band [System.Security.AccessControl.FileSystemRights]::FullControl) -ne [System.Security.AccessControl.FileSystemRights]::FullControl) }); if($acl.Access.Count -ne 1 -or $bad.Count -ne 0){ Write-Error ('unsafe ACL rules='+$acl.Access.Count); exit 1 }; 'restricted'`
	command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", script)
	command.Env = append(os.Environ(), "GOAL2_ACL_PATH="+path)
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "restricted") {
		t.Fatalf("ACL for %s is not restricted to the current user: %v %s", path, err, output)
	}
}
