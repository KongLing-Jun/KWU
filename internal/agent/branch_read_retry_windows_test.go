package agent

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func openBranchMetaExclusively(t *testing.T, path string) syscall.Handle {
	t.Helper()
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(
		pointer,
		syscall.GENERIC_READ,
		0,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("open exclusive sidecar: %v", err)
	}
	return handle
}

func TestLoadBranchMetaRetriesTransientSharingViolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := SaveBranchMeta(path, BranchMeta{TopicTitle: "kept"}); err != nil {
		t.Fatal(err)
	}
	handle := openBranchMetaExclusively(t, BranchMetaPath(path))
	closed := make(chan struct{})
	go func() {
		time.Sleep(120 * time.Millisecond)
		_ = syscall.CloseHandle(handle)
		close(closed)
	}()

	started := time.Now()
	meta, ok, err := LoadBranchMeta(path)
	if err != nil || !ok || meta.TopicTitle != "kept" {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", meta, ok, err)
	}
	<-closed
	if elapsed := time.Since(started); elapsed < 100*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("transient read retry elapsed = %v", elapsed)
	}
}

func TestLoadBranchMetaReturnsPersistentSharingViolationWithinBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := SaveBranchMeta(path, BranchMeta{TopicTitle: "kept"}); err != nil {
		t.Fatal(err)
	}
	handle := openBranchMetaExclusively(t, BranchMetaPath(path))
	defer syscall.CloseHandle(handle)

	started := time.Now()
	_, ok, err := LoadBranchMeta(path)
	if err == nil || ok {
		t.Fatalf("LoadBranchMeta ok=%v err=%v", ok, err)
	}
	if elapsed := time.Since(started); elapsed < 600*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("persistent read retry elapsed = %v", elapsed)
	}
}

func TestBranchMetaStrictPublishPreservesExistingOnReplaceFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := SaveBranchMeta(path, BranchMeta{TopicTitle: "old"}); err != nil {
		t.Fatal(err)
	}
	handle := openBranchMetaExclusively(t, BranchMetaPath(path))
	if err := SaveBranchMeta(path, BranchMeta{TopicTitle: "new"}); err == nil {
		syscall.CloseHandle(handle)
		t.Fatal("SaveBranchMeta succeeded while destination was exclusively open")
	}
	if err := syscall.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(BranchMetaPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) == 0 {
		t.Fatal("strict publish truncated existing sidecar")
	}
	meta, ok, err := LoadBranchMeta(path)
	if err != nil || !ok || meta.TopicTitle != "old" {
		t.Fatalf("existing sidecar changed after failed publish: %+v, %v, %v", meta, ok, err)
	}
}
