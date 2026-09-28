//go:build windows

package diagnostics

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func acquireJournalLock(dir string, timeout time.Duration) (*journalLock, bool) {
	file, err := os.OpenFile(dir+string(os.PathSeparator)+lockFilename, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false
	}
	_ = file.Chmod(0o600)
	deadline := time.Now().Add(timeout)
	for {
		err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
		if err == nil {
			return &journalLock{file: file}, true
		}
		if !isLockBusy(err) || time.Now().After(deadline) {
			_ = file.Close()
			return nil, false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func isLockBusy(err error) bool {
	return errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

type journalLock struct {
	file *os.File
}

func (l *journalLock) Close() {
	if l == nil || l.file == nil {
		return
	}
	_ = windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &windows.Overlapped{})
	_ = l.file.Close()
}
