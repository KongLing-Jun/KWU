//go:build !windows

package diagnostics

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func acquireJournalLock(dir string, timeout time.Duration) (*journalLock, bool) {
	file, err := os.OpenFile(dir+string(os.PathSeparator)+lockFilename, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false
	}
	_ = file.Chmod(0o600)
	deadline := time.Now().Add(timeout)
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
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
	return err == syscall.EWOULDBLOCK || err == syscall.EAGAIN || err == syscall.EACCES
}

type journalLock struct {
	file *os.File
}

func (l *journalLock) Close() {
	if l == nil || l.file == nil {
		return
	}
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	_ = l.file.Close()
}
