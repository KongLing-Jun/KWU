//go:build windows

package cloud

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestIgnoreDurableDirectorySyncErrorOnWindows(t *testing.T) {
	t.Parallel()
	unsupported := &os.PathError{Op: "sync", Path: `C:\profile\cloud`, Err: syscall.ERROR_ACCESS_DENIED}
	if !ignoreDurableDirectorySyncError(unsupported) {
		t.Fatal("Windows directory sync ERROR_ACCESS_DENIED was not recognized")
	}
	if ignoreDurableDirectorySyncError(errors.New("ordinary sync failure")) {
		t.Fatal("ordinary directory sync failure was ignored")
	}
	if ignoreDurableDirectorySyncError(syscall.Errno(6)) {
		t.Fatal("unexpected Windows error was ignored")
	}
}
