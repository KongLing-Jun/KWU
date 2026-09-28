package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedCloudProjectWorkspacePathMatchesCanonicalLayout(t *testing.T) {
	stateRoot := t.TempDir()
	scopeKey := strings.Repeat("a", 64)
	projectDir := "project-" + strings.Repeat("b", 64)
	canonical := filepath.Join(stateRoot, cloudProjectWorkspacesDir, scopeKey, projectDir)
	profileCanonical := filepath.Join(stateRoot, "profiles", "previous-profile", cloudProjectWorkspacesDir, scopeKey, projectDir)
	profileStateRoot := filepath.Join(stateRoot, "profiles", "current-profile")
	otherStateRoot := t.TempDir()
	tests := []struct {
		name      string
		stateRoot string
		path      string
		want      bool
	}{
		{name: "canonical", path: canonical, want: true},
		{name: "case alias", path: filepath.Join(stateRoot, cloudProjectWorkspacesDir, strings.ToUpper(scopeKey), "PROJECT-"+strings.ToUpper(strings.Repeat("b", 64))), want: true},
		{name: "profile canonical", path: profileCanonical, want: true},
		{name: "profile root direct", stateRoot: profileStateRoot, path: filepath.Join(profileStateRoot, cloudProjectWorkspacesDir, scopeKey, projectDir), want: true},
		{name: "outside state root", path: filepath.Join(otherStateRoot, cloudProjectWorkspacesDir, scopeKey, projectDir)},
		{name: "invalid scope", path: filepath.Join(stateRoot, cloudProjectWorkspacesDir, "scope", projectDir)},
		{name: "invalid project hash", path: filepath.Join(stateRoot, cloudProjectWorkspacesDir, scopeKey, "project-short")},
		{name: "nested child", path: filepath.Join(canonical, "child")},
		{name: "nested profile", path: filepath.Join(stateRoot, "profiles", "one", "profiles", "two", cloudProjectWorkspacesDir, scopeKey, projectDir)},
		{name: "nested from profile root", stateRoot: profileStateRoot, path: filepath.Join(profileStateRoot, "profiles", "nested", cloudProjectWorkspacesDir, scopeKey, projectDir)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := test.stateRoot
			if root == "" {
				root = stateRoot
			}
			if got := isManagedCloudProjectWorkspacePath(root, test.path); got != test.want {
				t.Fatalf("isManagedCloudProjectWorkspacePath(%q, %q) = %v, want %v", root, test.path, got, test.want)
			}
		})
	}
}

func TestCloudProjectWorkspaceIsStableDedicatedAndOutsideGlobal(t *testing.T) {
	_, state := newWorkBuddyProjectApp(t, nil)
	first, err := ensureCloudProjectWorkspace(context.Background(), state.services, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	again, err := ensureCloudProjectWorkspace(context.Background(), state.services, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	other, err := ensureCloudProjectWorkspace(context.Background(), state.services, "project-2")
	if err != nil {
		t.Fatal(err)
	}
	if !sameDesktopPath(first, again) {
		t.Fatalf("project workspace changed: %q / %q", first, again)
	}
	if sameDesktopPath(first, other) {
		t.Fatalf("different projects share workspace %q", first)
	}
	wantParent := filepath.Join(state.services.StateRoot, cloudProjectWorkspacesDir, state.scope.Key)
	if filepath.Dir(first) != wantParent || filepath.Dir(other) != wantParent {
		t.Fatalf("workspace parents = %q / %q, want %q", filepath.Dir(first), filepath.Dir(other), wantParent)
	}
	if projectWorkspacePathWithin(first, filepath.Join(state.services.StateRoot, "global-workspace")) {
		t.Fatalf("project workspace is inside global workspace: %q", first)
	}
	if info, statErr := os.Stat(first); statErr != nil || !info.IsDir() {
		t.Fatalf("dedicated workspace was not created: %v", statErr)
	}
}

func TestJoinCloudProjectCreatesDedicatedWorkspace(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	joined, err := app.JoinCloudProject(JoinCloudProjectInput{ProjectID: "project-1"})
	if err != nil {
		t.Fatal(err)
	}
	if joined.WorkspacePath == "" || filepath.Dir(joined.WorkspacePath) != filepath.Join(state.services.StateRoot, cloudProjectWorkspacesDir, state.scope.Key) {
		t.Fatalf("joined project workspace = %q", joined.WorkspacePath)
	}
	if info, statErr := os.Stat(joined.WorkspacePath); statErr != nil || !info.IsDir() {
		t.Fatalf("joined project workspace is unavailable: %v", statErr)
	}
}
