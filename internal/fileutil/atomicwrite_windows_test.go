//go:build windows

package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileStrictReplacesExistingOnWindows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pricing-cache.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"new", "newer", "final"} {
		if err := AtomicWriteFileStrict(path, []byte(value), 0o600); err != nil {
			t.Fatalf("replace existing cache with %q: %v", value, err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != value {
			t.Fatalf("cache after replacement = %q, %v", got, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "pricing-cache.json" {
		t.Fatalf("entries after replacements = %+v", entries)
	}
}
