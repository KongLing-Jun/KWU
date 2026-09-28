//go:build !windows

package expert

import "os"

func readUserStoreBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
