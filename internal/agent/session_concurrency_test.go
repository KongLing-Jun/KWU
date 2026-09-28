package agent

import (
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"kwu/internal/provider"
)

// TestSessionConcurrentAddAndRead models the real hazard: the run loop appends
// messages while a frontend (serve /history, autosave) reads the log from another
// goroutine. Snapshot must copy under the lock; before it, an append racing the
// copy could tear the slice header and crash.
func TestSessionConcurrentAddAndRead(t *testing.T) {
	s := NewSession("sys")

	var wg sync.WaitGroup
	// One writer mimicking the turn goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5000; i++ {
			s.Add(provider.Message{Role: provider.RoleUser, Content: "msg"})
		}
	}()
	// Many readers mimicking frontends polling history.
	for r := 0; r < 16; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5000; i++ {
				snap := s.Snapshot()
				for _, m := range snap { // iterate the copy: must never tear
					_ = m.Content
				}
				_ = s.HasContent()
			}
		}()
	}
	wg.Wait()

	if got := len(s.Snapshot()); got != 5001 { // 5000 + the system prompt
		t.Fatalf("final message count = %d, want 5001", got)
	}
}

func TestSessionConcurrentAddAndSaveIfChanged(t *testing.T) {
	s := NewSession("sys")
	path := filepath.Join(t.TempDir(), "session.jsonl")

	var wg sync.WaitGroup
	var saveErr error
	var saveErrMu sync.Mutex
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			s.Add(provider.Message{Role: provider.RoleUser, Content: "msg"})
			runtime.Gosched()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if _, _, err := s.SaveIfChanged(path); err != nil {
				saveErrMu.Lock()
				saveErr = err
				saveErrMu.Unlock()
				return
			}
			runtime.Gosched()
		}
	}()
	wg.Wait()

	saveErrMu.Lock()
	err := saveErr
	saveErrMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveIfChanged(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(loaded.Messages); got != 501 {
		t.Fatalf("saved message count = %d, want 501", got)
	}
}
