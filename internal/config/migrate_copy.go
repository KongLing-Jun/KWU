package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func copyDataRoot(src, dst string) ([]string, error) {
	info, err := os.Lstat(src)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("legacy data root is not a regular directory")
	}
	if err := applyCopiedDirectoryMode(dst, info.Mode().Perm()); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	return copyDataEntries(src, dst, entries)
}

func applyCopiedDirectoryMode(dst string, perm os.FileMode) error {
	if perm == 0 {
		return nil
	}
	return os.Chmod(dst, perm)
}

func copyDataEntries(src, dst string, entries []os.DirEntry) ([]string, error) {
	var warnings []string
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		entryInfo, err := os.Lstat(srcPath)
		if err != nil {
			return warnings, err
		}
		entryWarnings, err := copyDataEntry(srcPath, dstPath, entryInfo)
		warnings = append(warnings, entryWarnings...)
		if err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func copyDataEntry(src, dst string, info os.FileInfo) ([]string, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return []string{"skipped symbolic link " + src}, nil
	}
	if info.IsDir() {
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
			return nil, err
		}
		return copyDataRoot(src, dst)
	}
	if info.Mode().IsRegular() {
		return nil, copyFile(src, dst)
	}
	return []string{"skipped non-regular file " + src}, nil
}

func copyFile(src, dst string) error {
	info, err := copyableFileInfo(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), copiedFileParentMode(info.Mode().Perm())); err != nil {
		return err
	}

	perm := copiedFileMode(info.Mode().Perm())
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	return copyFileContents(out, in, dst, perm)
}

func copyableFileInfo(src string) (os.FileInfo, error) {
	info, err := os.Lstat(src)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("source is not a regular file: %s", src)
	}
	return info, nil
}

func copiedFileParentMode(perm os.FileMode) os.FileMode {
	if perm&0o077 == 0 {
		return 0o700
	}
	return 0o755
}

func copiedFileMode(perm os.FileMode) os.FileMode {
	if perm == 0 {
		return 0o600
	}
	return perm
}

func copyFileContents(out *os.File, in io.Reader, dst string, perm os.FileMode) error {
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	perm := info.Mode().Perm()
	if perm == 0 {
		perm = 0o700
	}
	if err := os.MkdirAll(dst, perm); err != nil {
		return err
	}
	if err := os.Chmod(dst, perm); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
