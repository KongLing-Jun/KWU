package agent

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kwu/internal/scene"
)

func TestBranchMetaMutationsSerializeAndPreserveFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := SaveBranchMeta(path, BranchMeta{}); err != nil {
		t.Fatal(err)
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- MutateBranchMeta(path, false, func(meta *BranchMeta) error {
			close(firstEntered)
			<-releaseFirst
			meta.TopicTitle = "concurrent title"
			return nil
		})
	}()
	<-firstEntered

	secondStarted := make(chan struct{})
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		secondDone <- MutateBranchMeta(path, false, func(meta *BranchMeta) error {
			close(secondEntered)
			meta.Model = "custom/model"
			return meta.SetSceneStatus(scene.Status{Scene: scene.Design, State: scene.Locked})
		})
	}()
	<-secondStarted
	select {
	case <-secondEntered:
		t.Fatal("second mutation entered while first mutation held the path lock")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}

	meta, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", meta, ok, err)
	}
	status, err := meta.SceneStatus()
	if err != nil {
		t.Fatal(err)
	}
	if meta.TopicTitle != "concurrent title" || meta.Model != "custom/model" || status.Scene != scene.Design || status.State != scene.Locked {
		t.Fatalf("metadata lost a concurrent field: %+v", meta)
	}
}

func TestBranchMetaConcurrentMutationAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := SaveBranchMeta(path, BranchMeta{}); err != nil {
		t.Fatal(err)
	}

	const writers = 8
	const mutationsPerWriter = 20
	start := make(chan struct{})
	errs := make(chan error, writers+4)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < mutationsPerWriter; j++ {
				if err := MutateBranchMeta(path, false, func(meta *BranchMeta) error {
					meta.Turns++
					return nil
				}); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < mutationsPerWriter; j++ {
				if _, ok, err := LoadBranchMeta(path); err != nil || !ok {
					errs <- fmt.Errorf("LoadBranchMeta ok=%v: %w", ok, err)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	meta, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", meta, ok, err)
	}
	if want := writers * mutationsPerWriter; meta.Turns != want {
		t.Fatalf("Turns = %d, want %d", meta.Turns, want)
	}
}

func TestBranchMetaLocksArePerPathAndReclaimed(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.jsonl")
	secondPath := filepath.Join(dir, "second.jsonl")
	for _, path := range []string{firstPath, secondPath} {
		if err := SaveBranchMeta(path, BranchMeta{}); err != nil {
			t.Fatal(err)
		}
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- MutateBranchMeta(firstPath, false, func(*BranchMeta) error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- MutateBranchMeta(secondPath, false, func(*BranchMeta) error {
			close(secondEntered)
			return nil
		})
	}()
	select {
	case <-secondEntered:
	case <-time.After(time.Second):
		t.Fatal("different sidecar path was blocked")
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}

	branchMetaPathLocks.Lock()
	entries := len(branchMetaPathLocks.entries)
	branchMetaPathLocks.Unlock()
	if entries != 0 {
		t.Fatalf("path lock entries = %d, want 0", entries)
	}
}

func TestMutateExistingBranchMetaDoesNotCreateMissingSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.jsonl")
	called := false
	ok, err := MutateExistingBranchMeta(path, false, func(*BranchMeta) error {
		called = true
		return nil
	})
	if err != nil || ok || called {
		t.Fatalf("MutateExistingBranchMeta = %v, %v; called=%v", ok, err, called)
	}
	if _, exists, err := LoadBranchMeta(path); err != nil || exists {
		t.Fatalf("missing sidecar was created: exists=%v err=%v", exists, err)
	}
}
