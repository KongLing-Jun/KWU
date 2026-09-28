package automation

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAcquireLease(t *testing.T) {
	store := NewStore(t.TempDir())
	lease, ok, err := store.AcquireLease("task-1", "run-1", "desktop:1", time.Minute)
	if err != nil {
		t.Fatalf("AcquireLease returned error: %v", err)
	}
	if !ok {
		t.Fatal("first acquire returned ok=false")
	}
	if lease.TaskID != "task-1" || lease.RunID != "run-1" || lease.Owner != "desktop:1" {
		t.Fatalf("lease identity = %+v", lease)
	}
	if !lease.ExpiresAt.After(lease.AcquiredAt) || !lease.HeartbeatAt.Equal(lease.AcquiredAt) {
		t.Fatalf("lease times = %+v", lease)
	}
}

func TestAcquireLeaseContended(t *testing.T) {
	store := NewStore(t.TempDir())
	const contenders = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, ok, err := store.AcquireLease("task-1", "run-"+string(rune('a'+i)), "owner-"+string(rune('a'+i)), time.Minute)
			if err != nil {
				t.Errorf("AcquireLease contender %d: %v", i, err)
				return
			}
			if ok {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if successes != 1 {
		t.Fatalf("successful acquires = %d, want 1", successes)
	}
}

func TestAcquireLeaseRemovesStaleLock(t *testing.T) {
	store := NewStore(t.TempDir())
	path, err := store.leasePath("task-1")
	if err != nil {
		t.Fatalf("leasePath: %v", err)
	}
	lockPath := path + leaseLockSuffix
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	old := time.Now().Add(-leaseLockStale - time.Second)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if _, ok, err := store.AcquireLease("task-1", "run-1", "desktop:1", time.Minute); err != nil || !ok {
		t.Fatalf("AcquireLease with stale lock ok=%v err=%v", ok, err)
	}
}

func TestLeaseExpires(t *testing.T) {
	store := NewStore(t.TempDir())
	path, err := store.leasePath("task-1")
	if err != nil {
		t.Fatalf("leasePath: %v", err)
	}
	old := Lease{
		TaskID:      "task-1",
		RunID:       "run-old",
		Owner:       "desktop:1",
		AcquiredAt:  time.Now().Add(-2 * time.Minute).UTC(),
		HeartbeatAt: time.Now().Add(-2 * time.Minute).UTC(),
		ExpiresAt:   time.Now().Add(-time.Minute).UTC(),
	}
	if err := writeLease(path, old); err != nil {
		t.Fatalf("writeLease: %v", err)
	}

	lease, ok, err := store.AcquireLease("task-1", "run-new", "serve:1", time.Minute)
	if err != nil {
		t.Fatalf("AcquireLease returned error: %v", err)
	}
	if !ok {
		t.Fatal("expired lease was not acquired")
	}
	if lease.Owner != "serve:1" || lease.RunID != "run-new" {
		t.Fatalf("lease = %+v, want new owner/run", lease)
	}
}

func TestAcquireLeaseHeldByOtherOwner(t *testing.T) {
	store := NewStore(t.TempDir())
	held, ok, err := store.AcquireLease("task-1", "run-1", "desktop:1", time.Minute)
	if err != nil || !ok {
		t.Fatalf("initial AcquireLease = %+v, %v, %v", held, ok, err)
	}

	got, ok, err := store.AcquireLease("task-1", "run-2", "serve:1", time.Minute)
	if err != nil {
		t.Fatalf("contended AcquireLease returned error: %v", err)
	}
	if ok {
		t.Fatal("contended AcquireLease returned ok=true")
	}
	if got.Owner != held.Owner || got.RunID != held.RunID {
		t.Fatalf("contended lease = %+v, want existing %+v", got, held)
	}
}

func TestRefreshLease(t *testing.T) {
	store := NewStore(t.TempDir())
	lease, ok, err := store.AcquireLease("task-1", "run-1", "desktop:1", 10*time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("AcquireLease = %+v, %v, %v", lease, ok, err)
	}
	time.Sleep(time.Millisecond)

	if err := store.RefreshLease("task-1", "run-1", "desktop:1", time.Minute); err != nil {
		t.Fatalf("RefreshLease returned error: %v", err)
	}
	path, err := store.leasePath("task-1")
	if err != nil {
		t.Fatalf("leasePath: %v", err)
	}
	refreshed, err := readLease(path)
	if err != nil {
		t.Fatalf("readLease: %v", err)
	}
	if !refreshed.ExpiresAt.After(lease.ExpiresAt) || !refreshed.HeartbeatAt.After(lease.HeartbeatAt) {
		t.Fatalf("refreshed lease = %+v, want later expiry/heartbeat than %+v", refreshed, lease)
	}
}

func TestReleaseLease(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, ok, err := store.AcquireLease("task-1", "run-1", "desktop:1", time.Minute); err != nil || !ok {
		t.Fatalf("AcquireLease ok=%v err=%v", ok, err)
	}
	path, err := store.leasePath("task-1")
	if err != nil {
		t.Fatalf("leasePath: %v", err)
	}

	if err := store.ReleaseLease("task-1", "run-2", "serve:1"); err != nil {
		t.Fatalf("non-owner ReleaseLease returned error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("non-owner release removed lease: %v", err)
	}

	if err := store.ReleaseLease("task-1", "run-1", "desktop:1"); err != nil {
		t.Fatalf("owner ReleaseLease returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("owner release left lease file, stat err=%v", err)
	}
}

func TestLeasePathCannotEscapeRoot(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	path, err := store.leasePath("..\\..//outside")
	if err != nil {
		t.Fatalf("leasePath returned error: %v", err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("Rel(%q, %q): %v", root, path, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		t.Fatalf("lease path escaped root: %s", path)
	}
}
