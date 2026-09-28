package provider

import (
	"os"
	"syscall"
	"testing"
)

func TestIsConnResetRejectsWindowsFilesystemSharingViolation(t *testing.T) {
	for _, errno := range []syscall.Errno{32, 33} {
		err := &os.PathError{Op: "open", Path: `C:\sessions\scene.jsonl.meta`, Err: errno}
		if IsConnReset(err) {
			t.Fatalf("filesystem errno %d was classified as a connection reset: %v", errno, err)
		}
	}
}
