package expert

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const (
	userStoreReadRetries   = 8
	userStoreReadRetryBase = 20 * time.Millisecond
)

func readUserStoreBytes(path string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		body, err := readUserStoreAttempt(path)
		if err == nil {
			return body, nil
		}
		if attempt >= userStoreReadRetries ||
			(!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_LOCK_VIOLATION)) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt+1) * userStoreReadRetryBase)
	}
}

func readUserStoreAttempt(path string) ([]byte, error) {
	file, err := openUserStoreRead(path)
	if err != nil {
		return nil, err
	}
	// Close each attempt before retrying, including partially completed reads.
	defer file.Close()
	return io.ReadAll(file)
}

func openUserStoreRead(path string) (*os.File, error) {
	nativePath, err := userStoreNativePath(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	pointer, err := windows.UTF16PtrFromString(nativePath)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	// Delete sharing allows reads alongside compatible Windows rename handles.
	handle, err := windows.CreateFile(pointer, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

func userStoreNativePath(path string) (string, error) {
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) {
		return path, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// Native opens need an extended absolute path for long application-data paths.
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(abs, `\\`), nil
	}
	return `\\?\` + abs, nil
}
