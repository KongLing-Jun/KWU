package main

import (
	"context"
	"path/filepath"
	"testing"
)

func sharedHostRefsForTest(t *testing.T, app *App, root string) (int, bool) {
	t.Helper()
	app.sharedHostsMu.Lock()
	defer app.sharedHostsMu.Unlock()
	entry, ok := app.sharedHosts[root]
	if !ok {
		return 0, false
	}
	return entry.refs, true
}

func TestCloneDetachedRuntimeTabPreservesSharedHostKey(t *testing.T) {
	tab := &WorkspaceTab{ID: "tab", SharedHostKey: "workspace-root"}
	detached := cloneDetachedRuntimeTab(tab, "session-key")
	if detached == nil {
		t.Fatal("cloneDetachedRuntimeTab returned nil")
	}
	if detached.SharedHostKey != tab.SharedHostKey {
		t.Fatalf("detached SharedHostKey = %q, want %q", detached.SharedHostKey, tab.SharedHostKey)
	}
}

func TestApplyRuntimeTabTransfersDetachedSharedHostKey(t *testing.T) {
	target := &WorkspaceTab{ID: "visible", SharedHostKey: "new-build-ref"}
	source := &WorkspaceTab{ID: "detached", SharedHostKey: "detached-ref"}

	applyRuntimeTab(target, source, "session-key", context.Background(), NewApp())

	if target.SharedHostKey != "detached-ref" {
		t.Fatalf("target SharedHostKey = %q, want detached-ref", target.SharedHostKey)
	}
}

func TestCloseRemovedSessionRuntimesReleasesSharedHost(t *testing.T) {
	app := NewApp()
	key := sharedHostKey("profile-a", t.TempDir())
	app.acquireSharedHost(key)

	tab := &WorkspaceTab{ID: "tab", SharedHostKey: key}
	app.closeRemovedSessionRuntimes([]removedSessionRuntime{{tab: tab}})

	if tab.SharedHostKey != "" {
		t.Fatalf("tab SharedHostKey = %q, want cleared", tab.SharedHostKey)
	}
	if refs, ok := sharedHostRefsForTest(t, app, key); ok {
		t.Fatalf("shared host still present with refs=%d, want released", refs)
	}
}

func TestSharedHostKeyIncludesProfileAndNormalizesWorkspace(t *testing.T) {
	root := t.TempDir()
	profileA := sharedHostKey("profile-a", root)
	profileAAgain := sharedHostKey("profile-a", filepath.Join(root, "."))
	profileB := sharedHostKey("profile-b", root)

	if profileA != profileAAgain {
		t.Fatalf("same profile/workspace keys differ: %q vs %q", profileA, profileAAgain)
	}
	if profileA == profileB {
		t.Fatalf("different profiles produced same shared host key: %q", profileA)
	}
}

func TestAcquireSharedHostUsesProfileAwareKey(t *testing.T) {
	app := NewApp()
	t.Cleanup(app.closeAllSharedHosts)

	root := t.TempDir()
	profileAKey := sharedHostKey("profile-a", root)
	profileAKeyAgain := sharedHostKey("profile-a", filepath.Join(root, "."))
	profileBKey := sharedHostKey("profile-b", root)

	profileAHost := app.acquireSharedHost(profileAKey)
	if got := app.acquireSharedHost(profileAKeyAgain); got != profileAHost {
		t.Fatal("same profile/workspace did not reuse shared host")
	}
	profileBHost := app.acquireSharedHost(profileBKey)
	if profileBHost == profileAHost {
		t.Fatal("different profiles reused the same shared host")
	}

	if refs, ok := sharedHostRefsForTest(t, app, profileAKey); !ok || refs != 2 {
		t.Fatalf("profile A refs = %d, ok=%v, want refs=2 ok=true", refs, ok)
	}
	if refs, ok := sharedHostRefsForTest(t, app, profileBKey); !ok || refs != 1 {
		t.Fatalf("profile B refs = %d, ok=%v, want refs=1 ok=true", refs, ok)
	}

	app.releaseSharedHost(profileAKey)
	if refs, ok := sharedHostRefsForTest(t, app, profileAKey); !ok || refs != 1 {
		t.Fatalf("profile A refs after one release = %d, ok=%v, want refs=1 ok=true", refs, ok)
	}
	if refs, ok := sharedHostRefsForTest(t, app, profileBKey); !ok || refs != 1 {
		t.Fatalf("profile B refs after profile A release = %d, ok=%v, want refs=1 ok=true", refs, ok)
	}
}

func TestReleaseTabSharedHostDoesNotReleaseOtherProfile(t *testing.T) {
	app := NewApp()
	t.Cleanup(app.closeAllSharedHosts)

	root := t.TempDir()
	profileAKey := sharedHostKey("profile-a", root)
	profileBKey := sharedHostKey("profile-b", root)
	app.acquireSharedHost(profileAKey)
	app.acquireSharedHost(profileBKey)

	tabA := &WorkspaceTab{ID: "a", SharedHostKey: profileAKey}
	tabB := &WorkspaceTab{ID: "b", SharedHostKey: profileBKey}
	app.releaseTabSharedHost(tabA)

	if tabA.SharedHostKey != "" {
		t.Fatalf("profile A tab SharedHostKey = %q, want cleared", tabA.SharedHostKey)
	}
	if refs, ok := sharedHostRefsForTest(t, app, profileAKey); ok {
		t.Fatalf("profile A shared host still present with refs=%d, want released", refs)
	}
	if refs, ok := sharedHostRefsForTest(t, app, profileBKey); !ok || refs != 1 {
		t.Fatalf("profile B refs = %d, ok=%v, want refs=1 ok=true", refs, ok)
	}
	if tabB.SharedHostKey != profileBKey {
		t.Fatalf("profile B tab SharedHostKey = %q, want %q", tabB.SharedHostKey, profileBKey)
	}
}
