package main

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"kwu/internal/plugin"
	"kwu/internal/userctx"
)

// sharedPluginHost is a reference-counted plugin.Host shared across tabs
// that share the same profile and workspace root. Multiple controllers (one per
// tab) use the same Host so MCP subprocesses (CodeGraph, etc.) are spawned once.
type sharedPluginHost struct {
	host *plugin.Host
	refs int
}

const (
	sharedHostKeySep             = "\x00"
	sharedHostGlobalWorkspaceKey = "__global__"
)

func sharedHostKey(profileID, workspaceRoot string) string {
	profileID = userctx.NormalizeProfileID(profileID)
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		root = sharedHostGlobalWorkspaceKey
	} else {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		root = filepath.Clean(root)
	}
	return profileID + sharedHostKeySep + root
}

func sharedHostKeyForActor(actor userctx.ActorContext, workspaceRoot string) string {
	actor = actor.Normalize()
	return sharedHostKey(actor.ProfileID, workspaceRoot)
}

// acquireSharedHost returns a shared *plugin.Host for the given shared host key.
// The first call creates the host; subsequent calls increment a refcount and
// return the same host. The caller must call releaseSharedHost when the tab
// no longer needs the host.
func (a *App) acquireSharedHost(key string) *plugin.Host {
	a.sharedHostsMu.Lock()
	defer a.sharedHostsMu.Unlock()

	if a.sharedHosts == nil {
		a.sharedHosts = make(map[string]*sharedPluginHost)
	}

	entry, ok := a.sharedHosts[key]
	if ok {
		entry.refs++
		slog.Debug("shared host acquired (reused)", "key", key, "refs", entry.refs)
		return entry.host
	}

	host := plugin.NewHost()
	a.sharedHosts[key] = &sharedPluginHost{host: host, refs: 1}
	slog.Debug("shared host acquired (new)", "key", key)
	return host
}

// lookupSharedHost returns an existing shared host for the given key, or nil.
// Unlike acquireSharedHost, it does NOT increment the refcount — use this when
// rebuilding a controller for an existing tab that already holds a reference.
func (a *App) lookupSharedHost(key string) *plugin.Host {
	a.sharedHostsMu.Lock()
	defer a.sharedHostsMu.Unlock()
	if a.sharedHosts == nil {
		return nil
	}
	entry, ok := a.sharedHosts[key]
	if !ok {
		return nil
	}
	return entry.host
}

// reapOrphanCodeGraph kills any codegraph MCP subprocess that is not a
// direct child of the current KWU process. This cleans up orphaned
// processes from a previous crash or from older versions that leaked them,
// preventing accumulation across restarts.
func (a *App) reapOrphanCodeGraph() {
	myPID := os.Getpid()

	// Collect the PIDs of our direct children (the ones we own).
	// pgrep -P exits non-zero when there are no children; treat that as an
	// empty set and continue scanning for orphans rather than skipping the
	// entire reaping step.
	ours := map[int]bool{}
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(myPID)).Output()
	if err == nil {
		for _, f := range strings.Fields(string(out)) {
			if pid, err := strconv.Atoi(f); err == nil {
				ours[pid] = true
			}
		}
	}

	// Find every codegraph MCP process.
	out, err = exec.Command("pgrep", "-f", "codegraph\\.js serve --mcp").Output()
	if err != nil {
		return
	}
	for _, f := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(f)
		if err != nil || pid == myPID || ours[pid] {
			continue
		}
		// Verify the process is truly orphaned before killing it:
		// check its parent PID — if the parent is alive and isn't ours,
		// this codegraph belongs to another active KWU session.
		ppidOut, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(strings.TrimSpace(string(ppidOut)))
		if err != nil || ppid == 0 {
			continue
		}
		// ppid==1 means the parent died and init reparented it — truly orphaned.
		if ppid != 1 {
			continue
		}
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
			slog.Debug("reaped orphan codegraph", "pid", pid)
		}
	}
}

// releaseSharedHost decrements the refcount for the shared host key and closes
// the shared host when no tabs reference it any more. Safe to call even when
// no acquire was made (no-op).
func (a *App) releaseSharedHost(key string) {
	a.sharedHostsMu.Lock()
	defer a.sharedHostsMu.Unlock()

	entry, ok := a.sharedHosts[key]
	if !ok {
		return
	}
	entry.refs--
	if entry.refs > 0 {
		slog.Debug("shared host released (still in use)", "key", key, "refs", entry.refs)
		return
	}

	delete(a.sharedHosts, key)
	entry.host.Close()
	slog.Debug("shared host closed", "key", key)
}

func (a *App) releaseTabSharedHost(tab *WorkspaceTab) {
	if tab == nil || tab.SharedHostKey == "" {
		return
	}
	key := tab.SharedHostKey
	tab.SharedHostKey = ""
	a.releaseSharedHost(key)
}

// closeAllSharedHosts closes every shared host. Called during app shutdown.
func (a *App) closeAllSharedHosts() {
	a.sharedHostsMu.Lock()
	defer a.sharedHostsMu.Unlock()

	for key, entry := range a.sharedHosts {
		delete(a.sharedHosts, key)
		entry.host.Close()
		slog.Debug("shared host closed (shutdown)", "key", key)
	}
}
