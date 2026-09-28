package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/tool"
)

func init() { tool.RegisterBuiltin(listDir{}) }

// listDir lists a directory. workDir, when non-empty, is the directory a
// relative path resolves against (see resolveIn).
type listDir struct {
	workDir string
	roots   []string
}

func (listDir) Name() string { return "ls" }

func (listDir) Description() string {
	return "Inspect a file or directory path. A file returns its name and byte size. A directory lists its entries, with trailing slashes for directories and byte sizes for files. Set recursive=true to list all nested files depth-first (skips .git/node_modules)."
}

func (listDir) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File or directory path (default \".\")"},"recursive":{"type":"boolean","description":"When true, recursively list all nested files in a directory (default false)"}}}`)
}

func (listDir) ReadOnly() bool { return true }

func (l listDir) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	p := struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}{Path: "."}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
	}
	var err error
	if p.Path, err = readablePath(l.workDir, l.roots, p.Path, "."); err != nil {
		return "", err
	}
	return l.listPath(p.Path, p.Recursive)
}

func (l listDir) listPath(path string, recursive bool) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("ls %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Sprintf("%s\t%d\n", filepath.Base(path), info.Size()), nil
	}
	if recursive {
		return l.listRecursive(path)
	}
	return l.listFlat(path)
}

func (listDir) listFlat(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("ls %s: %w", path, err)
	}

	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			fmt.Fprintf(&b, "%s/\n", e.Name())
			continue
		}
		size := int64(-1)
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		fmt.Fprintf(&b, "%s\t%d\n", e.Name(), size)
	}
	if b.Len() == 0 {
		return "(empty directory)", nil
	}
	return b.String(), nil
}

// listRecursive walks a directory tree depth-first, skipping noise dirs.
// Depth is capped to guard against symlink loops.
func (l listDir) listRecursive(root string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, wErr error) error {
		if wErr != nil {
			return wErr
		}
		if p == root {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".DS_Store", "__pycache__", ".idea", ".vscode":
				return filepath.SkipDir
			}
		}
		rel, rErr := filepath.Rel(root, p)
		if rErr != nil {
			rel = p
		}
		// Guard against excessive depth.
		if strings.Count(rel, string(os.PathSeparator)) > 50 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			rel += "/"
		} else if info, iErr := d.Info(); iErr == nil {
			rel += fmt.Sprintf("\t%d", info.Size())
		}
		b.WriteString(rel + "\n")
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("ls -R %s: %w", root, err)
	}
	if b.Len() == 0 {
		return "(empty directory tree)", nil
	}
	return b.String(), nil
}
