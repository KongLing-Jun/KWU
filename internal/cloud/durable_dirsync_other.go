//go:build !windows

package cloud

func ignoreDurableDirectorySyncError(error) bool {
	return false
}
