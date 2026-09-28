package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/project"
	"kwu/internal/userctx"
)

func TestListArtifactsHidesCloudProjectAfterMemberDirectoryRemoval(t *testing.T) {
	fixture := newArtifactCloudVisibilityFixture(t)
	fixture.refreshJoinedDirectory(t, true)
	assertArtifactSessions(t, fixture.app, fixture.cloudSessionPath, fixture.localSessionPath)
	fixture.assertWorkspaceNames(t)

	fixture.refreshJoinedDirectory(t, false)
	assertArtifactSessions(t, fixture.app, fixture.localSessionPath)
	if len(fixture.app.ListAllSessions()) != 2 {
		t.Fatal("artifact filtering changed the existing session-list policy")
	}
	// A retained detail must not resurrect membership removed by an
	// authoritative directory refresh, matching the sidebar's existing rule.
	if _, err := fixture.app.GetCloudProject("project-1"); err != nil {
		t.Fatal(err)
	}
	assertArtifactSessions(t, fixture.app, fixture.localSessionPath)
	fixture.assertFilesUnchanged(t)

	fixture.refreshJoinedDirectory(t, true)
	assertArtifactSessions(t, fixture.app, fixture.cloudSessionPath, fixture.localSessionPath)
	fixture.assertWorkspaceNames(t)
	fixture.assertFilesUnchanged(t)
}

func TestListArtifactsHidesCloudProjectAfterAccountAccessEnds(t *testing.T) {
	for _, scenario := range []string{"logout", "account switch"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := newArtifactCloudVisibilityFixture(t)
			fixture.refreshJoinedDirectory(t, true)
			assertArtifactSessions(t, fixture.app, fixture.cloudSessionPath, fixture.localSessionPath)

			if scenario == "logout" {
				if _, err := fixture.app.CloudLogout(); err != nil {
					t.Fatal(err)
				}
			} else {
				actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
				if _, err := fixture.state.runtime.bindings.Rebind(context.Background(), actor, "usr_other"); err != nil {
					t.Fatal(err)
				}
			}
			assertArtifactSessions(t, fixture.app, fixture.localSessionPath)
			fixture.assertFilesUnchanged(t)
		})
	}
}

type artifactCloudVisibilityFixture struct {
	app              *App
	state            *workBuddyProjectAppState
	cloudSessionPath string
	localSessionPath string
	workspaceRoot    string
	files            map[string][]byte
}

func newArtifactCloudVisibilityFixture(t *testing.T) artifactCloudVisibilityFixture {
	t.Helper()
	app, state := newWorkBuddyProjectApp(t, nil)
	state.ownerUserID = "usr_owner"
	state.name = "Joined project"
	workspace := t.TempDir()
	controller, activePath := installWorkBuddyProjectSessionTab(t, app, workspace)
	t.Cleanup(controller.Close)
	dir := filepath.Dir(activePath)
	cloudPath := writeTopicSessionWithPrompt(t, dir, "cloud-artifact.jsonl", "cloud-artifact", "Cloud report", workspace, "update cloud report", time.Now())
	localPath := writeTopicSessionWithPrompt(t, dir, "local-artifact.jsonl", "local-artifact", "Local report", workspace, "update local report", time.Now())
	meta, found, err := agent.LoadBranchMeta(cloudPath)
	if err != nil || !found {
		t.Fatalf("load cloud session metadata: found=%v err=%v", found, err)
	}
	meta.ProjectContext = &agent.ProjectContextPin{
		Environment: agent.ProjectEnvironmentIdentity{
			EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL,
		},
		BindingScope: agent.ProjectBindingScope{
			ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration,
		},
		ProjectID: "project-1", InstructionVersion: 1, ContentHash: project.InstructionContentSHA256("", ""),
	}
	if err := agent.SaveBranchMeta(cloudPath, meta); err != nil {
		t.Fatal(err)
	}
	fixture := artifactCloudVisibilityFixture{
		app: app, state: state, cloudSessionPath: cloudPath, localSessionPath: localPath,
		workspaceRoot: workspace, files: make(map[string][]byte),
	}
	for sessionPath, artifactName := range map[string]string{cloudPath: "cloud-report.xlsx", localPath: "local-report.md"} {
		path := filepath.Join(workspace, artifactName)
		body := []byte("retained " + artifactName)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := mergeLegacyArtifactSidecar(sessionPath, []string{artifactName}, time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
		fixture.files[path] = body
		sidecarPath := artifactSidecarPath(sessionPath)
		fixture.files[sidecarPath], err = os.ReadFile(sidecarPath)
		if err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func (f artifactCloudVisibilityFixture) assertWorkspaceNames(t *testing.T) {
	t.Helper()
	names := make(map[string]string)
	for _, artifact := range f.app.ListArtifacts() {
		names[sessionRuntimeKey(artifact.Session.Path)] = artifact.WorkspaceName
	}
	if got := names[sessionRuntimeKey(f.cloudSessionPath)]; got != "Joined project" {
		t.Fatalf("cloud artifact workspace name = %q, want Joined project", got)
	}
	if got := names[sessionRuntimeKey(f.localSessionPath)]; got != workspaceName(f.workspaceRoot) {
		t.Fatalf("local artifact workspace name = %q, want physical workspace name %q", got, workspaceName(f.workspaceRoot))
	}
}

func (f artifactCloudVisibilityFixture) refreshJoinedDirectory(t *testing.T, member bool) {
	t.Helper()
	response := `{"data":{"items":[],"next_cursor":null},"error":null,"request_id":"req-artifact-joined-empty","server_time":"2026-08-05T04:00:03Z"}`
	if member {
		response = `{"data":{"items":[{"project_id":"project-1","owner_user_id":"usr_owner","name":"Joined project","description":null,"status":"active","version":1,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"}],"next_cursor":null},"error":null,"request_id":"req-artifact-joined","server_time":"2026-08-05T04:00:02Z"}`
	}
	f.state.mu.Lock()
	f.state.joinedListResponse = response
	f.state.mu.Unlock()
	if _, err := f.app.ListJoinedCloudProjects(CloudProjectListInput{}); err != nil {
		t.Fatal(err)
	}
}

func (f artifactCloudVisibilityFixture) assertFilesUnchanged(t *testing.T) {
	t.Helper()
	for path, before := range f.files {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("artifact file or sidecar changed: %s, err=%v", path, err)
		}
	}
}

func assertArtifactSessions(t *testing.T, app *App, sessionPaths ...string) {
	t.Helper()
	artifacts := app.ListArtifacts()
	if len(artifacts) != len(sessionPaths) {
		t.Fatalf("ListArtifacts = %+v, want sessions %v", artifacts, sessionPaths)
	}
	want := make(map[string]bool, len(sessionPaths))
	for _, path := range sessionPaths {
		want[sessionRuntimeKey(path)] = true
	}
	for _, artifact := range artifacts {
		key := sessionRuntimeKey(artifact.Session.Path)
		if !want[key] {
			t.Fatalf("unexpected artifact session: %+v", artifact)
		}
		delete(want, key)
	}
}
