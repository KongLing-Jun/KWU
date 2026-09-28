package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kwu/internal/control"
	"kwu/internal/proc"
)

const workspaceGitTimeout = 5 * time.Second
const workspaceGitBranchCacheTTL = 2 * time.Second

type workspaceGitBranchCacheEntry struct {
	branch     string
	expires    time.Time
	refreshing bool
}

var workspaceGitBranchCache = struct {
	sync.Mutex
	entries map[string]workspaceGitBranchCacheEntry
}{entries: map[string]workspaceGitBranchCacheEntry{}}

var workspaceGitBranchForMetaProbe = workspaceGitBranch

type gitStatusEntry struct {
	Path    string
	OldPath string
	Status  string
}

type workspaceChangeAccumulator struct {
	view       WorkspaceChangeView
	hasSession bool
	hasGit     bool
}

func (a *App) WorkspaceChanges(tabID string) WorkspaceChangesView {
	out := WorkspaceChangesView{Files: []WorkspaceChangeView{}, GitAvailable: true}
	tabID = strings.TrimSpace(tabID)

	workspaceRoot, ctrl, ok := a.workspaceChangesTarget(tabID)
	if !ok {
		out.GitAvailable = false
		out.GitErr = fmt.Sprintf("tab %q not found", tabID)
		return out
	}

	base, err := workspaceBaseFromRoot(workspaceRoot)
	if err != nil {
		out.GitAvailable = false
		out.GitErr = err.Error()
		return out
	}

	out.GitBranch = workspaceGitBranch(base)

	changes := map[string]*workspaceChangeAccumulator{}
	add := func(path string) *workspaceChangeAccumulator {
		path = normalizeWorkspaceRelPath(base, path)
		if path == "" {
			return nil
		}
		if changes[path] == nil {
			changes[path] = &workspaceChangeAccumulator{view: WorkspaceChangeView{Path: path}}
		}
		return changes[path]
	}

	if ctrl != nil {
		for _, meta := range ctrl.Checkpoints() {
			for _, path := range meta.Paths {
				acc := add(path)
				if acc == nil {
					continue
				}
				acc.hasSession = true
				if len(acc.view.Turns) == 0 || acc.view.Turns[len(acc.view.Turns)-1] != meta.Turn {
					acc.view.Turns = append(acc.view.Turns, meta.Turn)
				}
				if meta.Time.UnixMilli() >= acc.view.LatestTime {
					acc.view.LatestPrompt = meta.Prompt
					acc.view.LatestTime = meta.Time.UnixMilli()
				}
			}
		}
	}

	gitEntries, gitErr := workspaceGitStatus(base)
	if gitErr != nil {
		out.GitAvailable = false
		out.GitErr = gitErr.Error()
	}
	for _, entry := range gitEntries {
		acc := add(entry.Path)
		if acc == nil {
			continue
		}
		acc.hasGit = true
		acc.view.GitStatus = entry.Status
		acc.view.OldPath = normalizeWorkspaceRelPath(base, entry.OldPath)
	}

	out.Files = make([]WorkspaceChangeView, 0, len(changes))
	for _, acc := range changes {
		if acc.hasSession {
			acc.view.Sources = append(acc.view.Sources, "session")
		}
		if acc.hasGit {
			acc.view.Sources = append(acc.view.Sources, "git")
		}
		out.Files = append(out.Files, acc.view)
	}
	sort.Slice(out.Files, func(i, j int) bool {
		a, b := out.Files[i], out.Files[j]
		if len(a.Sources) != len(b.Sources) {
			return len(a.Sources) > len(b.Sources)
		}
		return strings.ToLower(a.Path) < strings.ToLower(b.Path)
	})
	return out
}

func (a *App) workspaceChangesTarget(tabID string) (string, control.SessionAPI, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var tab *WorkspaceTab
	if tabID == "" {
		tab = a.activeTabLocked()
	} else {
		tab = a.tabRuntimes[tabID]
	}
	if tab == nil {
		return "", nil, tabID == ""
	}
	return tab.WorkspaceRoot, tab.Ctrl, true
}

func (a *App) workspaceBaseForTab(tabID string) (string, error) {
	tabID = strings.TrimSpace(tabID)
	workspaceRoot, _, ok := a.workspaceChangesTarget(tabID)
	if !ok {
		return "", fmt.Errorf("tab %q not found", tabID)
	}
	return workspaceBaseFromRoot(workspaceRoot)
}

// workspaceBaseForOwnedTab resolves only an explicitly named tab. File APIs
// use this strict variant so missing ownership metadata can never fall back to
// the mutable active workspace. Legacy callers that intentionally accept an
// empty tab ID keep using workspaceBaseForTab.
func (a *App) workspaceBaseForOwnedTab(tabID string) (string, error) {
	tabID = strings.TrimSpace(tabID)
	if tabID == "" {
		return "", fmt.Errorf("tab ID is required")
	}
	return a.workspaceBaseForTab(tabID)
}

// workspaceGit builds a console-hidden git probe: CREATE_NO_WINDOW so git's own
// children inherit the invisible console, fsmonitor/auto-maintenance off so a
// probe never spawns a background daemon that opens a console of its own (#3906).
func workspaceGit(args ...string) *exec.Cmd {
	return workspaceGitCommand(context.Background(), args...)
}

func workspaceGitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.fsmonitor=false", "-c", "maintenance.auto=false"}, args...)...)
	proc.HideWindow(cmd)
	return cmd
}

func workspaceGitWithTimeout(args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), workspaceGitTimeout)
	return workspaceGitCommand(ctx, args...), cancel
}

func workspaceGitStatus(base string) ([]gitStatusEntry, error) {
	cmd, cancel := workspaceGitWithTimeout("-C", base, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	defer cancel()
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	entries := parseGitStatusPorcelainZ(raw)
	topCmd, topCancel := workspaceGitWithTimeout("-C", base, "rev-parse", "--show-toplevel")
	defer topCancel()
	topRaw, err := topCmd.Output()
	if err != nil {
		return nil, err
	}
	repoRoot := strings.TrimSpace(string(topRaw))
	if repoRoot == "" {
		return entries, nil
	}
	out := make([]gitStatusEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Path = workspaceRelPathFromGitStatus(repoRoot, base, entry.Path)
		if entry.Path == "" {
			continue
		}
		entry.OldPath = workspaceRelPathFromGitStatus(repoRoot, base, entry.OldPath)
		out = append(out, entry)
	}
	return out, nil
}

func parseGitStatusPorcelainZ(raw []byte) []gitStatusEntry {
	parts := bytes.Split(raw, []byte{0})
	out := make([]gitStatusEntry, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if len(part) < 4 {
			continue
		}
		status := string(part[:2])
		path := string(part[3:])
		entry := gitStatusEntry{Path: path, Status: strings.TrimSpace(status)}
		if strings.ContainsAny(status, "RC") && i+1 < len(parts) {
			i++
			entry.OldPath = string(parts[i])
		}
		out = append(out, entry)
	}
	return out
}

func normalizeWorkspaceRelPath(base, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(base, path); err == nil {
			path = rel
		}
	}
	path = filepath.Clean(path)
	if path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(path)
}

func workspaceRelPathFromGitStatus(repoRoot, base, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, filepath.FromSlash(path))
	}
	return normalizeWorkspaceRelPath(base, path)
}

func validateGitBranch(branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.HasPrefix(branch, "-") ||
		strings.Contains(branch, "..") || strings.Contains(branch, "@{") ||
		strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") ||
		strings.HasSuffix(branch, ".") || strings.Contains(branch, "//") ||
		strings.ContainsAny(branch, " \t\r\n~^:?*[\\") {
		return "", fmt.Errorf("invalid git branch")
	}
	for _, r := range branch {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("invalid git branch")
		}
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return "", fmt.Errorf("invalid git branch")
		}
	}
	return branch, nil
}

func validateGitCommitHash(hash string) (string, error) {
	hash = strings.TrimSpace(hash)
	if len(hash) < 7 || len(hash) > 64 {
		return "", fmt.Errorf("invalid git commit hash")
	}
	for _, r := range hash {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return "", fmt.Errorf("invalid git commit hash")
	}
	return hash, nil
}

func validateWorkspaceGitPath(base, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	rel := normalizeWorkspaceRelPath(base, path)
	if rel == "" {
		return "", fmt.Errorf("invalid workspace path")
	}
	return rel, nil
}

// workspaceGitBranchForMeta is the cached variant used by high-frequency UI
// metadata refreshes. It never waits for git on the caller path: stale branch
// metadata is less harmful than blocking tab activation or hydration. Workflows
// that need an immediate git read, such as WorkspaceChanges, should call
// workspaceGitBranch directly.
func workspaceGitBranchForMeta(base string) string {
	key := filepath.Clean(base)
	now := time.Now()

	workspaceGitBranchCache.Lock()
	if cached, ok := workspaceGitBranchCache.entries[key]; ok {
		branch := cached.branch
		if now.Before(cached.expires) || cached.refreshing {
			workspaceGitBranchCache.Unlock()
			return branch
		}
		cached.refreshing = true
		workspaceGitBranchCache.entries[key] = cached
		workspaceGitBranchCache.Unlock()
		go refreshWorkspaceGitBranchForMeta(key, base)
		return branch
	}

	workspaceGitBranchCache.entries[key] = workspaceGitBranchCacheEntry{
		expires:    now.Add(workspaceGitBranchCacheTTL),
		refreshing: true,
	}
	workspaceGitBranchCache.Unlock()

	go refreshWorkspaceGitBranchForMeta(key, base)
	return ""
}

func refreshWorkspaceGitBranchForMeta(key, base string) {
	branch := ""
	defer func() {
		storeNow := time.Now()
		workspaceGitBranchCache.Lock()
		if len(workspaceGitBranchCache.entries) > 256 {
			for cacheKey, cached := range workspaceGitBranchCache.entries {
				if storeNow.After(cached.expires) {
					delete(workspaceGitBranchCache.entries, cacheKey)
				}
			}
		}
		workspaceGitBranchCache.entries[key] = workspaceGitBranchCacheEntry{
			branch:  branch,
			expires: storeNow.Add(workspaceGitBranchCacheTTL),
		}
		workspaceGitBranchCache.Unlock()
	}()

	branch = workspaceGitBranchForMetaProbe(base)
}

// workspaceGitBranch returns the current git branch name for the repo rooted
// at base, or an empty string when base is not inside a git repository or when
// git is unavailable.
func workspaceGitBranch(base string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := workspaceGitCommand(ctx, "-C", base, "branch", "--show-current")
	raw, err := cmd.Output()
	if err != nil {
		return ""
	}
	if branch := strings.TrimSpace(string(raw)); branch != "" {
		return branch
	}

	headCmd := workspaceGitCommand(ctx, "-C", base, "rev-parse", "--short", "HEAD")
	raw, err = headCmd.Output()
	if err != nil {
		return ""
	}
	short := strings.TrimSpace(string(raw))
	if short == "" {
		return ""
	}
	return "@" + short
}

// GitBranches returns all local git branches for the active workspace's repo.
func (a *App) GitBranches() ([]string, error) {
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return nil, err
	}
	cmd, cancel := workspaceGitWithTimeout("-C", base, "branch", "--format=%(refname:short)")
	defer cancel()
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	branches := strings.FieldsFunc(strings.TrimSpace(string(raw)), func(r rune) bool { return r == '\n' })
	return branches, nil
}

// GitCheckout switches the active workspace's git branch and returns the
// current branch name, or an error when git is unavailable.
func (a *App) GitCheckout(branch string) error {
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return err
	}
	branch, err = validateGitBranch(branch)
	if err != nil {
		return err
	}
	cmd, cancel := workspaceGitWithTimeout("-C", base, "checkout", branch)
	defer cancel()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return fmt.Errorf("git checkout: %s", strings.TrimSpace(string(out)))
		}
		return err
	}
	return nil
}

type GitCommitView struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

type GitCommitDetailView struct {
	Diff  *string  `json:"diff,omitempty"`
	Files []string `json:"files,omitempty"`
}

func (a *App) WorkspaceGitHistory(tabID string, path string) ([]GitCommitView, error) {
	base, err := a.workspaceBaseForTab(tabID)
	if err != nil {
		return nil, err
	}
	path, err = validateWorkspaceGitPath(base, path)
	if err != nil {
		return nil, err
	}

	args := []string{"-C", base, "log", "--pretty=format:%H%x00%an%x00%ad%x00%s", "-z", "-n", "100"}
	if path != "" {
		args = append(args, "--", path)
	}

	cmd, cancel := workspaceGitWithTimeout(args...)
	defer cancel()
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	parts := bytes.Split(raw, []byte{0})
	var out []GitCommitView
	// 4 parts per commit: hash, author, date, message
	for i := 0; i+3 < len(parts); i += 4 {
		out = append(out, GitCommitView{
			Hash:    string(parts[i]),
			Author:  string(parts[i+1]),
			Date:    string(parts[i+2]),
			Message: string(parts[i+3]),
		})
	}
	return out, nil
}

func (a *App) WorkspaceGitCommitDetail(tabID string, hash string, path string) (GitCommitDetailView, error) {
	base, err := a.workspaceBaseForTab(tabID)
	if err != nil {
		return GitCommitDetailView{}, err
	}
	hash, err = validateGitCommitHash(hash)
	if err != nil {
		return GitCommitDetailView{}, err
	}
	path, err = validateWorkspaceGitPath(base, path)
	if err != nil {
		return GitCommitDetailView{}, err
	}

	if path != "" {
		// Single file diff
		cmd, cancel := workspaceGitWithTimeout("-C", base, "show", "--relative", "--pretty=format:", "--patch", hash, "--", path)
		defer cancel()
		raw, err := cmd.Output()
		if err != nil {
			return GitCommitDetailView{}, err
		}
		diffStr := strings.TrimSpace(string(raw))
		return GitCommitDetailView{Diff: &diffStr}, nil
	}

	// Project level: list of files changed
	cmd, cancel := workspaceGitWithTimeout("-C", base, "diff-tree", "--relative", "--no-commit-id", "--name-only", "-r", hash)
	defer cancel()
	raw, err := cmd.Output()
	if err != nil {
		return GitCommitDetailView{}, err
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return GitCommitDetailView{Files: files}, nil
}
