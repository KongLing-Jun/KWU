package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/config"
)

// The desktop is a GUI app: launched from Finder or `open`, it starts with the
// working directory set to "/" (read-only), so anything cwd-relative — config,
// .env writes, memory/skill discovery — fails or lands nowhere useful. We keep a
// real working folder instead: remember the last one the user picked and chdir
// into it at startup, falling back to the home directory when there's none and
// cwd isn't writable.

// workspaceStatePath is where the last working folder is remembered.
func workspaceStatePath() string {
	return config.DesktopWorkspaceStatePath()
}

func workspaceStatePathForPaths(paths config.ProfilePathResolver) string {
	if paths.ProfileID() == "" {
		return workspaceStatePath()
	}
	return paths.DesktopWorkspaceStatePath()
}

func workspaceListPath() string {
	return config.DesktopWorkspaceListPath()
}

func workspaceListPathForPaths(paths config.ProfilePathResolver) string {
	if paths.ProfileID() == "" {
		return workspaceListPath()
	}
	return paths.DesktopWorkspaceListPath()
}

func (a *App) workspaceStatePath() string {
	return workspaceStatePathForPaths(a.currentProfilePathResolver(""))
}

func (a *App) workspaceListPath() string {
	return workspaceListPathForPaths(a.currentProfilePathResolver(""))
}

// saveWorkspace records dir as the last working folder.
func saveWorkspace(dir string) {
	saveWorkspaceToPath(workspaceStatePath(), dir)
}

func (a *App) saveWorkspace(dir string) {
	saveWorkspaceToPath(a.workspaceStatePath(), dir)
}

func saveWorkspaceToPath(p, dir string) {
	if p == "" || dir == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, []byte(dir), 0o644)
}

// clearWorkspace removes the active workspace pointer file so that no stale
// reference remains after the current workspace is deleted.
func clearWorkspace() {
	clearWorkspaceAtPath(workspaceStatePath())
}

func (a *App) clearWorkspace() {
	clearWorkspaceAtPath(a.workspaceStatePath())
}

func clearWorkspaceAtPath(p string) {
	if p == "" {
		return
	}
	_ = os.Remove(p)
}

// loadWorkspace returns the remembered working folder, or "" if none.
func loadWorkspace() string {
	return loadWorkspaceFromPath(workspaceStatePath())
}

func (a *App) loadWorkspace() string {
	return loadWorkspaceFromPath(a.workspaceStatePath())
}

func loadWorkspaceFromPath(p string) string {
	if p == "" {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func loadWorkspaces() []string {
	return loadWorkspacesFromPath(workspaceListPath())
}

func (a *App) loadWorkspaces() []string {
	return loadWorkspacesFromPath(a.workspaceListPath())
}

func loadWorkspacesFromPath(p string) []string {
	if p == "" {
		return nil
	}
	var paths []string
	b, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(b, &paths) != nil {
		return nil
	}
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func rememberWorkspace(dir string) {
	rememberWorkspaceWithPaths(config.ProfilePathResolver{}, dir)
}

func (a *App) rememberWorkspace(dir string) {
	rememberWorkspaceWithPaths(a.currentProfilePathResolver(""), dir)
}

func rememberWorkspaceWithPaths(paths config.ProfilePathResolver, dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	listPath := workspaceListPathForPaths(paths)
	next := []string{dir}
	for _, path := range loadWorkspacesFromPath(listPath) {
		if path != dir {
			next = append(next, path)
		}
		if len(next) >= 12 {
			break
		}
	}
	p := listPath
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	if b, err := json.MarshalIndent(next, "", "  "); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}

func forgetWorkspace(dir string) {
	forgetWorkspaceWithPaths(config.ProfilePathResolver{}, dir)
}

func (a *App) forgetWorkspace(dir string) {
	forgetWorkspaceWithPaths(a.currentProfilePathResolver(""), dir)
}

func forgetWorkspaceWithPaths(paths config.ProfilePathResolver, dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	listPath := workspaceListPathForPaths(paths)
	workspaces := loadWorkspacesFromPath(listPath)
	next := make([]string, 0, len(workspaces))
	for _, path := range workspaces {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if path != dir {
			next = append(next, path)
		}
	}
	p := listPath
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	if b, err := json.MarshalIndent(next, "", "  "); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}

// ensureWorkspace establishes a writable working directory at startup: the
// remembered folder if it's still a directory, else the home directory when the
// current cwd isn't writable (the Finder/`open` "/" case). A writable cwd with no
// remembered folder (e.g. `wails dev` in the repo) is left untouched.
func ensureWorkspace() {
	if ws := loadWorkspace(); ws != "" {
		if info, err := os.Stat(ws); err == nil && info.IsDir() && os.Chdir(ws) == nil {
			return
		}
	}
	if cwdWritable() {
		return
	}
	if home, err := os.UserHomeDir(); err == nil {
		_ = os.Chdir(home)
	}
}

func (a *App) ensureWorkspace() {
	if ws := a.loadWorkspace(); ws != "" {
		if info, err := os.Stat(ws); err == nil && info.IsDir() && os.Chdir(ws) == nil {
			return
		}
	}
	if cwdWritable() {
		return
	}
	if home, err := os.UserHomeDir(); err == nil {
		_ = os.Chdir(home)
	}
}

// cwdWritable reports whether the current directory accepts a file write — the
// reliable test for the read-only "/" a GUI launch lands in.
func cwdWritable() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	f, err := os.CreateTemp(cwd, ".kwu-wtest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
