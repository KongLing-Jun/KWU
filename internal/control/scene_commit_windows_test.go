package control

import (
	"context"
	"syscall"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/scene"
)

func openSceneMetaExclusively(t *testing.T, path string) syscall.Handle {
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
		t.Fatalf("open exclusive scene sidecar: %v", err)
	}
	return handle
}

func TestDraftSceneCommitRetriesTransientMetaSharingViolation(t *testing.T) {
	runner := &sceneCommitRunner{}
	c, path := newSceneCommitController(t, runner, nil)
	meta := agent.BranchMeta{}
	if err := meta.SetSceneStatus(scene.Status{Scene: scene.Coding, State: scene.Draft}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	handle := openSceneMetaExclusively(t, agent.BranchMetaPath(path))
	closed := make(chan struct{})
	go func() {
		time.Sleep(120 * time.Millisecond)
		_ = syscall.CloseHandle(handle)
		close(closed)
	}()

	started := time.Now()
	if err := c.runRootRunner(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	<-closed
	if elapsed := time.Since(started); elapsed < 100*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("scene metadata retry elapsed = %v", elapsed)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", meta, ok, err)
	}
	status, err := meta.SceneStatus()
	if err != nil || status != (scene.Status{Scene: scene.Coding, State: scene.Locked}) {
		t.Fatalf("scene status = %+v, %v", status, err)
	}
}
