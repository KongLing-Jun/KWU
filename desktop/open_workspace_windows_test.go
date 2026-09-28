//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func stubWorkspacePathOpeners(
	t *testing.T,
	shellErr error,
	fallback func(name string, args ...string) error,
) {
	t.Helper()
	originalShellExecute := shellExecuteWorkspacePath
	originalFallback := startWorkspaceOpenFallback
	shellExecuteWorkspacePath = func(
		windows.Handle,
		*uint16,
		*uint16,
		*uint16,
		*uint16,
		int32,
	) error {
		return shellErr
	}
	startWorkspaceOpenFallback = fallback
	t.Cleanup(func() {
		shellExecuteWorkspacePath = originalShellExecute
		startWorkspaceOpenFallback = originalFallback
	})
}

func TestOpenWorkspacePathUsesShellAssociationWhenAvailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(path, []byte("# report"), 0o644); err != nil {
		t.Fatal(err)
	}
	fallbackCalls := 0
	stubWorkspacePathOpeners(t, nil, func(string, ...string) error {
		fallbackCalls++
		return nil
	})

	if err := openWorkspacePath(path); err != nil {
		t.Fatalf("openWorkspacePath() error = %v", err)
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0", fallbackCalls)
	}
}

func TestOpenWorkspacePathFallsBackToNotepadForMarkdown(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	path := filepath.Join(t.TempDir(), "专家报告.md")
	if err := os.WriteFile(path, []byte("# 专家报告"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotName string
	var gotArgs []string
	stubWorkspacePathOpeners(t, errors.New("no file association"), func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	})

	if err := openWorkspacePath(path); err != nil {
		t.Fatalf("openWorkspacePath() error = %v", err)
	}
	if gotName != `C:\Windows\System32\notepad.exe` {
		t.Fatalf("fallback executable = %q, want Notepad", gotName)
	}
	if len(gotArgs) != 1 || gotArgs[0] != path {
		t.Fatalf("fallback args = %q, want [%q]", gotArgs, path)
	}
}

func TestOpenWorkspacePathFallsBackToExplorerForDirectory(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	path := t.TempDir()
	var gotName string
	var gotArgs []string
	stubWorkspacePathOpeners(t, errors.New("shell open failed"), func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	})

	if err := openWorkspacePath(path); err != nil {
		t.Fatalf("openWorkspacePath() error = %v", err)
	}
	if gotName != `C:\Windows\explorer.exe` {
		t.Fatalf("fallback executable = %q, want Explorer", gotName)
	}
	if len(gotArgs) != 1 || gotArgs[0] != path {
		t.Fatalf("fallback args = %q, want [%q]", gotArgs, path)
	}
}

func TestOpenWorkspacePathOffersOpenWithForUnassociatedBinary(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	path := filepath.Join(t.TempDir(), "analysis.xlsx")
	if err := os.WriteFile(path, []byte("workbook"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotName string
	var gotArgs []string
	stubWorkspacePathOpeners(t, errors.New("no file association"), func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	})

	if err := openWorkspacePath(path); err != nil {
		t.Fatalf("openWorkspacePath() error = %v", err)
	}
	if gotName != `C:\Windows\System32\rundll32.exe` {
		t.Fatalf("fallback executable = %q, want rundll32", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "shell32.dll,OpenAs_RunDLL" || gotArgs[1] != path {
		t.Fatalf("fallback args = %q, want OpenAs for %q", gotArgs, path)
	}
}

func TestOpenWorkspacePathReturnsBothOpenFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(path, []byte("# report"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubWorkspacePathOpeners(t, errors.New("shell failure"), func(string, ...string) error {
		return errors.New("fallback failure")
	})

	err := openWorkspacePath(path)
	if err == nil {
		t.Fatal("openWorkspacePath() error = nil, want both failures")
	}
	for _, want := range []string{"shell failure", "fallback failure", path} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("openWorkspacePath() error = %q, want %q", err, want)
		}
	}
}

func TestOpenWorkspacePathDoesNotFallbackForMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.md")
	fallbackCalls := 0
	stubWorkspacePathOpeners(t, errors.New("shell failure"), func(string, ...string) error {
		fallbackCalls++
		return nil
	})

	err := openWorkspacePath(path)
	if err == nil {
		t.Fatal("openWorkspacePath() error = nil, want missing-file error")
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d, want 0 for a missing file", fallbackCalls)
	}
}

func TestIsWindowsTextWorkspaceFileCoversExpertTextOutputs(t *testing.T) {
	for _, path := range []string{
		"报告.md",
		"notes.MARKDOWN",
		"evidence.jsonl",
		"matrix.csv",
		"plan.yaml",
		"README.txt",
	} {
		if !isWindowsTextWorkspaceFile(path) {
			t.Errorf("isWindowsTextWorkspaceFile(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"report.pdf", "workbook.xlsx", "slides.pptx", "archive.zip"} {
		if isWindowsTextWorkspaceFile(path) {
			t.Errorf("isWindowsTextWorkspaceFile(%q) = true, want false", path)
		}
	}
}
