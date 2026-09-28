package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
)

var (
	renameGoal2ResultFile   = os.Rename
	restrictGoal2ResultFile = restrictGoal2SensitiveFile
)

func goal2FileSHA256(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("%x", digest[:]), nil
}

func writeGoal2E2EResult(path string, value any) (returnErr error) {
	if !filepath.IsAbs(path) {
		return errors.New("Goal 2 result path must be absolute")
	}
	directory := filepath.Dir(filepath.Clean(path))
	if err := secureGoal2SensitiveDirectory(directory); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	keep := false
	defer func() {
		if keep {
			return
		}
		for _, artifact := range []string{temporary, path} {
			if cleanupErr := os.Remove(artifact); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove sensitive Goal 2 result artifact %s: %w", filepath.Base(artifact), cleanupErr))
			}
		}
	}()
	if err := os.WriteFile(temporary, append(payload, '\n'), 0o600); err != nil {
		return err
	}
	if err := renameGoal2ResultFile(temporary, path); err != nil {
		return err
	}
	if err := restrictGoal2ResultFile(path); err != nil {
		return err
	}
	keep = true
	return nil
}

func secureGoal2SensitiveDirectory(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("Goal 2 sensitive directory path must be absolute")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Chmod(path, 0o700)
	}
	return setGoal2WindowsACL(path, true)
}

func restrictGoal2SensitiveFile(path string) error {
	if runtime.GOOS != "windows" {
		return os.Chmod(path, 0o600)
	}
	return setGoal2WindowsACL(path, false)
}

func setGoal2WindowsACL(path string, directory bool) error {
	current, err := user.Current()
	if err != nil {
		return fmt.Errorf("resolve Goal 2 artifact owner: %w", err)
	}
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	kind := "FileSecurity"
	inheritance := "None"
	if directory {
		kind = "DirectorySecurity"
		inheritance = "ContainerInherit,ObjectInherit"
	}
	script := `$securityModule=Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1'; Import-Module $securityModule -ErrorAction Stop; $account=New-Object System.Security.Principal.NTAccount($env:GOAL2_ACL_USER); $type='System.Security.AccessControl.'+$env:GOAL2_ACL_KIND; $acl=New-Object $type; $acl.SetOwner($account); $acl.SetAccessRuleProtection($true,$false); $rule=New-Object System.Security.AccessControl.FileSystemAccessRule($account,'FullControl',$env:GOAL2_ACL_INHERITANCE,'None','Allow'); [void]$acl.AddAccessRule($rule); Set-Acl -LiteralPath $env:GOAL2_ACL_PATH -AclObject $acl; $check=Get-Acl -LiteralPath $env:GOAL2_ACL_PATH; $bad=@($check.Access | Where-Object { $_.IsInherited -or $_.AccessControlType -ne 'Allow' -or $_.IdentityReference.Value -ine $env:GOAL2_ACL_USER -or (($_.FileSystemRights -band [System.Security.AccessControl.FileSystemRights]::FullControl) -ne [System.Security.AccessControl.FileSystemRights]::FullControl) }); if($check.Access.Count -ne 1 -or $bad.Count -ne 0){ Write-Error 'Goal 2 artifact ACL verification failed'; exit 1 }`
	command := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", script)
	command.Env = append(os.Environ(), "GOAL2_ACL_PATH="+path, "GOAL2_ACL_USER="+current.Username, "GOAL2_ACL_KIND="+kind, "GOAL2_ACL_INHERITANCE="+inheritance)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("secure Goal 2 artifact ACL: %w: %s", err, output)
	}
	return nil
}
