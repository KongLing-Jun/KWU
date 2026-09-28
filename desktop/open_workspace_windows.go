//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

var shellExecuteWorkspacePath = windows.ShellExecute

var startWorkspaceOpenFallback = func(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

func openWorkspacePath(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	openErr := shellExecuteWorkspacePath(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
	if openErr == nil {
		return nil
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		return errors.Join(openErr, statErr)
	}

	var fallbackErr error
	switch {
	case info.IsDir():
		fallbackErr = startWorkspaceOpenFallback(windowsSystemExecutable("explorer.exe", false), path)
	case info.Mode().IsRegular() && isWindowsTextWorkspaceFile(path):
		// Markdown is the default expert deliverable, but a clean Windows
		// installation commonly has no .md file association. Notepad is always
		// available and gives these text artifacts a deterministic open path.
		fallbackErr = startWorkspaceOpenFallback(windowsSystemExecutable("notepad.exe", true), path)
	case info.Mode().IsRegular():
		// Office/PDF/archive artifacts may also lack an installed handler. Let
		// Windows offer the user an application instead of turning that missing
		// association into the generic "Could not open file" toast.
		fallbackErr = startWorkspaceOpenFallback(
			windowsSystemExecutable("rundll32.exe", true),
			"shell32.dll,OpenAs_RunDLL",
			path,
		)
	default:
		return openErr
	}
	if fallbackErr == nil {
		return nil
	}
	return fmt.Errorf("open workspace path %s: %w", path, errors.Join(openErr, fallbackErr))
}

func windowsSystemExecutable(name string, system32 bool) string {
	root := strings.TrimSpace(os.Getenv("SystemRoot"))
	if root == "" {
		root = strings.TrimSpace(os.Getenv("windir"))
	}
	if root == "" {
		return name
	}
	if system32 {
		return filepath.Join(root, "System32", name)
	}
	return filepath.Join(root, name)
}

func isWindowsTextWorkspaceFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case
		".bat", ".c", ".cc", ".cfg", ".conf", ".cpp", ".cs", ".css",
		".csv", ".env", ".go", ".h", ".hpp", ".htm", ".html", ".ini",
		".java", ".js", ".json", ".jsonl", ".jsx", ".log", ".lua",
		".md", ".markdown", ".mjs", ".ps1", ".py", ".r", ".rb", ".rs",
		".sh", ".sql", ".svg", ".toml", ".ts", ".tsx", ".tsv", ".txt",
		".xml", ".yaml", ".yml":
		return true
	default:
		return false
	}
}
