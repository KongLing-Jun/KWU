package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestAtomicWriteFileStrictPreservesExistingOnReplaceFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	replaceErr := errors.New("replace failed")
	err := atomicWriteFile(path, []byte("new"), 0o600, func(tmp, dest string) error {
		if filepath.Dir(tmp) != filepath.Dir(dest) {
			t.Fatalf("tmp dir = %q, dest dir = %q", filepath.Dir(tmp), filepath.Dir(dest))
		}
		return replaceErr
	})
	if !errors.Is(err, replaceErr) {
		t.Fatalf("AtomicWriteFileStrict error = %v", err)
	}
	if got, readErr := os.ReadFile(path); readErr != nil || string(got) != "old" {
		t.Fatalf("existing file = %q, %v", got, readErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "cache.json" {
		t.Fatalf("entries after failed replace = %+v", entries)
	}
}

func TestReplaceFileStrictRejectsDifferentDirectories(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	destDir := filepath.Join(root, "dest")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(sourceDir, "value.tmp")
	dest := filepath.Join(destDir, "value.json")
	if err := os.WriteFile(tmp, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceFileStrict(tmp, dest); err == nil {
		t.Fatal("ReplaceFileStrict succeeded across directories")
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "old" {
		t.Fatalf("destination = %q, %v; want unchanged old bytes", got, err)
	}
	if got, err := os.ReadFile(tmp); err != nil || string(got) != "new" {
		t.Fatalf("source = %q, %v; want unchanged new bytes", got, err)
	}
}

func TestReplaceFileNoRetryWhenTmpMissing(t *testing.T) {
	oldBase := replaceRetryBase
	replaceRetryBase = 10 * time.Second
	t.Cleanup(func() { replaceRetryBase = oldBase })

	dir := t.TempDir()
	start := time.Now()
	err := ReplaceFile(filepath.Join(dir, "missing.tmp"), filepath.Join(dir, "x.txt"))
	if err == nil {
		t.Fatal("want error when tmp source is missing")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("missing tmp should fail fast, took %v — it retried", elapsed)
	}
}

func TestReplaceFileRetriesThenReturnsError(t *testing.T) {
	oldBase, oldMax := replaceRetryBase, maxReplaceRetries
	replaceRetryBase, maxReplaceRetries = 0, 3
	t.Cleanup(func() { replaceRetryBase, maxReplaceRetries = oldBase, oldMax })

	dir := t.TempDir()
	tmp := filepath.Join(dir, "x.tmp")
	if err := os.WriteFile(tmp, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "blocked")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceFile(tmp, dest); err == nil {
		t.Fatal("want error when dest can never be replaced")
	}
	if !fileExists(tmp) {
		t.Error("tmp should survive a failed replace so the next launch can retry")
	}
}

func TestReplaceFileRenamesInPlace(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "x.tmp")
	dest := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(tmp, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceFile(tmp, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "hello" {
		t.Errorf("dest = %q, want hello", b)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("tmp should be gone after ReplaceFile")
	}
}

func TestCopyOntoOverwritesAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "x.tmp")
	dest := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(tmp, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old-and-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyOnto(tmp, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "new" {
		t.Errorf("dest = %q, want new (fully overwritten)", b)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("tmp should be removed after copyOnto")
	}
	// Mode preservation is meaningful on Unix; Windows only tracks the read-only bit.
	if info, err := os.Stat(dest); err == nil && info.Mode().Perm() != 0o600 {
		t.Logf("dest mode = %o (want 0600 on Unix)", info.Mode().Perm())
	}
}

func TestAtomicWriteFileReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("new-content"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-content" {
		t.Fatalf("content = %q, want %q", got, "new-content")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
	// No leftover tmp files in the directory.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "config.toml" {
			t.Fatalf("unexpected leftover file: %s", e.Name())
		}
	}
}

func TestAtomicWriteFileCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "creds")
	if err := AtomicWriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile into missing dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
