package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveExportFileRejectsUnpickedPath(t *testing.T) {
	target := filepath.Join(t.TempDir(), "session.md")

	err := (&App{}).SaveExportFile(target, "payload", false)
	if err == nil || !strings.Contains(err.Error(), "export") {
		t.Fatalf("SaveExportFile error = %v, want export token error", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("unpicked export path was written, stat err = %v", statErr)
	}
}

func TestSaveExportFileConsumesPickedToken(t *testing.T) {
	app := &App{}
	target := filepath.Join(t.TempDir(), "session.md")
	token, err := app.rememberExportFile(target)
	if err != nil {
		t.Fatalf("rememberExportFile: %v", err)
	}
	if token == "" || token == target {
		t.Fatalf("rememberExportFile token = %q, want opaque token", token)
	}

	if err := app.SaveExportFile(token, "payload", false); err != nil {
		t.Fatalf("SaveExportFile: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("export payload = %q, want payload", got)
	}

	if err := app.SaveExportFile(token, "again", false); err == nil {
		t.Fatal("SaveExportFile reused a consumed token")
	}
	got, err = os.ReadFile(target)
	if err != nil {
		t.Fatalf("read export after rejected reuse: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("consumed token overwrote export payload: %q", got)
	}
}
