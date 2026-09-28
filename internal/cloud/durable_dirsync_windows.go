//go:build windows

package cloud

import (
	"errors"
	"syscall"
)

func ignoreDurableDirectorySyncError(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
}
