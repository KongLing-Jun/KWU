package control

import (
	"strings"
	"testing"
	"time"
)

func TestSnapshotWithTimeoutDoesNotWaitForeverForBusySnapshot(t *testing.T) {
	c := &Controller{}
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()

	started := time.Now()
	err := c.SnapshotWithTimeout(50 * time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "snapshot lock remained busy") {
		t.Fatalf("SnapshotWithTimeout() error = %v, want busy timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("SnapshotWithTimeout() took %s, want bounded return", elapsed)
	}
}

func TestSnapshotWithTimeoutUsesNormalSnapshotWhenAvailable(t *testing.T) {
	c := &Controller{}
	if err := c.SnapshotWithTimeout(time.Second); err != nil {
		t.Fatalf("SnapshotWithTimeout() error = %v", err)
	}
}
