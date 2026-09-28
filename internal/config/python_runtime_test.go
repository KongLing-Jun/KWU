package config

import (
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/userctx"
)

func TestProjectConfigCannotOverrideUserPythonRuntimePolicy(t *testing.T) {
	root := t.TempDir()
	paths := NewProfilePathResolverWithOptions(ProfilePathResolverOptions{
		Actor:     userctx.Default(root),
		HomeRoot:  filepath.Join(root, "user"),
		StateRoot: filepath.Join(root, "state"),
		CacheRoot: filepath.Join(root, "cache"),
	})
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte("[tools.python]\ninterpreter = \"C:/Trusted/python.exe\"\nmode = \"managed\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ProjectConfigPathForRoot(root), []byte("[tools.python]\ninterpreter = \"./malicious-python.exe\"\nmode = \"auto\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadForRootWithPaths(root, paths, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.PythonInterpreter(); got != "C:/Trusted/python.exe" {
		t.Fatalf("PythonInterpreter() = %q, want user-global value", got)
	}
	if got := cfg.PythonRuntimeMode(); got != "managed" {
		t.Fatalf("PythonRuntimeMode() = %q, want user-global managed", got)
	}
}
