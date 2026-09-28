package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/project"
	"kwu/internal/userctx"
)

type goal2CloudManifest struct {
	BaseURL                        string `json:"base_url"`
	AccessTokenA                   string `json:"access_token_a"`
	AccessTokenB                   string `json:"access_token_b"`
	UserA                          string `json:"user_a"`
	UserB                          string `json:"user_b"`
	ContractVersion                string `json:"contract_version"`
	ContractSHA256                 string `json:"contract_sha256"`
	CloudCandidateManifestSHA256   string `json:"cloud_candidate_manifest_sha256"`
	DesktopCandidateManifestSHA256 string `json:"desktop_candidate_manifest_sha256"`
}

// TestWorkBuddyGoal2CloudDesktopJointE2E retains its stable Goal 2 selector and
// now also proves the final Instruction/session and WorkItem vertical slices.
// WebView and Cloud Admin remain separate evidence tiers.
func TestWorkBuddyGoal2CloudDesktopJointE2E(t *testing.T) {
	if os.Getenv("WORKBUDDY_GOAL2_E2E") != "1" {
		t.Skip("set WORKBUDDY_GOAL2_E2E=1 and WORKBUDDY_GOAL2_MANIFEST for the named joint E2E")
	}
	manifestPath := os.Getenv("WORKBUDDY_GOAL2_MANIFEST")
	if manifestPath == "" {
		t.Fatal("WORKBUDDY_GOAL2_MANIFEST is required")
	}
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest goal2CloudManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.BaseURL == "" || manifest.AccessTokenA == "" || manifest.AccessTokenB == "" || manifest.UserA == "" || manifest.UserB == "" {
		t.Fatal("joint E2E manifest is incomplete")
	}
	stateRoot := t.TempDir()
	workspacePath := t.TempDir()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	environment := desktopTestCloudEnvironment(manifest.BaseURL)
	capability := json.RawMessage(`{"enabled":true,"contract_version":"` + manifest.ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + manifest.ContractSHA256 + `","instructions":true,"work_items":true}`)

	runtimeA, bindingA := newGoal2E2ERuntime(t, stateRoot, manifest.BaseURL, environment, actor, manifest.UserA, manifest.AccessTokenA, capability)
	app := &App{cloudRuntime: runtimeA, ctx: context.Background()}
	description := "Goal 2 authoritative Cloud fact"
	created, err := app.CreateCloudProject(CreateCloudProjectInput{Name: "Goal 2 Joint E2E", DescriptionSet: true, Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if created.Project.ProjectID == "" || created.Project.Name != "Goal 2 Joint E2E" || created.Project.Version != 1 || created.ProjectScopeKey == "" {
		t.Fatalf("create/readback=%#v", created)
	}
	projectID := created.Project.ProjectID
	readback, err := app.GetCloudProject(projectID)
	if err != nil || readback.Project.ProjectID != projectID || readback.Project.Description.value == nil || *readback.Project.Description.value != description {
		t.Fatalf("authoritative readback=%#v err=%v", readback, err)
	}
	listed, err := app.ListCloudProjects(CloudProjectListInput{Search: "goal 2 joint"})
	if err != nil || len(listed.Items) != 1 || listed.Items[0].ProjectID != projectID {
		t.Fatalf("server search=%#v err=%v", listed, err)
	}
	runtimeA.projectMu.RLock()
	servicesA := runtimeA.projects
	runtimeA.projectMu.RUnlock()
	bound, err := servicesA.Bindings.Bind(context.Background(), servicesA.Scope, projectID, workspacePath)
	if err != nil || bound.ProjectID != projectID || bound.Status != project.LocalWorkspaceAvailable {
		t.Fatalf("binding=%#v err=%v", bound, err)
	}
	controller, _ := installWorkBuddyProjectSessionTab(t, app, workspacePath)
	app.projectTreeChangedHook = func() {}
	instruction, err := app.EditCloudProjectInstruction(EditCloudProjectInstructionInput{ProjectID: projectID, ExpectedVersion: 1, HumanDescription: "Joint E2E", AgentInstruction: "Keep project work scoped."})
	if err != nil || instruction.Version != 2 {
		t.Fatalf("instruction=%#v err=%v", instruction, err)
	}
	session, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: projectID, TabID: "project"})
	if err != nil || session.ProjectID != projectID || session.InstructionVersion != 2 {
		t.Fatalf("project session=%#v err=%v", session, err)
	}
	contextCount := 0
	for _, message := range controller.History() {
		if message.IsProjectContext() {
			contextCount++
		}
	}
	if contextCount != 1 {
		t.Fatalf("project context count=%d", contextCount)
	}
	workItem, err := app.CreateCloudProjectWorkItem(CreateCloudProjectWorkItemInput{ProjectID: projectID, Title: "Joint E2E item", Status: "todo"})
	if err != nil || workItem.WorkItem.Version != 1 {
		t.Fatalf("work item create=%#v err=%v", workItem, err)
	}
	moved, err := app.MoveCloudProjectWorkItem(MoveCloudProjectWorkItemInput{ProjectID: projectID, WorkItemID: workItem.WorkItem.WorkItemID, ExpectedVersion: 1, TargetStatus: "in_progress"})
	if err != nil || moved.WorkItem.Status != "in_progress" || moved.WorkItem.Version != 2 {
		t.Fatalf("work item move=%#v err=%v", moved, err)
	}
	workItems, err := app.ListCloudProjectWorkItems(CloudProjectWorkItemListInput{ProjectID: projectID})
	if err != nil || len(workItems.Items) != 1 || workItems.Items[0].WorkItemID != workItem.WorkItem.WorkItemID {
		t.Fatalf("work item list=%#v err=%v", workItems, err)
	}
	runtimeA.projectMu.RLock()
	pending, err := runtimeA.projects.Journal.Pending(context.Background(), runtimeA.projects.Scope)
	runtimeA.projectMu.RUnlock()
	if err != nil || len(pending) != 0 {
		t.Fatalf("post-readback journal=%#v err=%v", pending, err)
	}
	runtimeA.close()

	// Reconstruct the production runtime and its project services over the same
	// profile root. This is the non-GUI restart seam for cache/binding/journal.
	runtimeA2, _ := newGoal2E2ERuntimeFromExisting(t, stateRoot, manifest.BaseURL, environment, actor, bindingA, manifest.AccessTokenA, capability)
	defer runtimeA2.close()
	app = &App{cloudRuntime: runtimeA2, ctx: context.Background()}
	runtimeA2.projectMu.RLock()
	restartedServices := runtimeA2.projects
	runtimeA2.projectMu.RUnlock()
	bindings, err := restartedServices.Bindings.List(context.Background(), restartedServices.Scope)
	if err != nil || len(bindings) != 1 || bindings[0].DisplayPath == "" || bindings[0].ProjectID != projectID {
		t.Fatalf("restart binding=%#v err=%v", bindings, err)
	}
	if _, err := app.GetCloudProject(projectID); err != nil {
		t.Fatalf("restart authoritative readback: %v", err)
	}
	if restoredInstruction, err := app.GetCloudProjectInstruction(projectID); err != nil || restoredInstruction.Version != 2 {
		t.Fatalf("restart instruction=%#v err=%v", restoredInstruction, err)
	}
	if restoredItems, err := app.ListCloudProjectWorkItems(CloudProjectWorkItemListInput{ProjectID: projectID}); err != nil || len(restoredItems.Items) != 1 || restoredItems.Items[0].Status != "in_progress" {
		t.Fatalf("restart work items=%#v err=%v", restoredItems, err)
	}
	archived, err := app.ArchiveCloudProject(ProjectVersionMutationInput{ProjectID: projectID, ExpectedVersion: readback.Project.Version})
	if err != nil || archived.Project.Status != "archived" || archived.Project.Version != 2 {
		t.Fatalf("archive=%#v err=%v", archived, err)
	}

	// Rebind the same local profile to account B. The exact binding generation
	// changes, and neither Cloud facts nor device-local state may cross it.
	bindingB, err := runtimeA2.bindings.Rebind(context.Background(), actor, manifest.UserB)
	if err != nil {
		t.Fatal(err)
	}
	scopeB, err := bindingB.Scope()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := runtimeA2.sessions.Save(context.Background(), actor, cloud.Session{
		Scope: scopeB, State: cloud.SessionAuthenticated, AccessToken: manifest.AccessTokenB,
		RefreshToken: "goal2-e2e-refresh-b", DeviceID: "device-goal2-e2e",
		AccessTokenExpiresAt: now.Add(45 * time.Minute), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := runtimeA2.observeProjectBootstrap(context.Background(), actor, cloud.AccountState{Binding: bindingB, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: capability}}); err != nil {
		t.Fatal(err)
	}
	app = &App{cloudRuntime: runtimeA2, ctx: context.Background()}
	accountB, err := app.ListCloudProjects(CloudProjectListInput{Search: "goal2-account-b-isolation"})
	if err != nil || len(accountB.Items) != 0 {
		t.Fatalf("account B cloud isolation=%#v err=%v", accountB, err)
	}
	runtimeA2.projectMu.RLock()
	accountBServices := runtimeA2.projects
	runtimeA2.projectMu.RUnlock()
	bindingsB, err := accountBServices.Bindings.List(context.Background(), accountBServices.Scope)
	if err != nil || len(bindingsB) != 0 {
		t.Fatalf("account B local isolation=%#v err=%v", bindingsB, err)
	}
	if _, err := app.GetCloudProject(projectID); err == nil {
		t.Fatal("account B read account A project")
	}
	foreignWorkItems, err := app.ListCloudProjectWorkItems(CloudProjectWorkItemListInput{ProjectID: projectID})
	if err != nil || len(foreignWorkItems.Items) != 0 {
		t.Fatalf("account B work item isolation=%#v err=%v", foreignWorkItems, err)
	}

	for _, cloudOwned := range []string{projectID, created.Project.Name, description} {
		if strings.Contains(cloudOwned, workspacePath) {
			t.Fatal("LocalWorkspace path entered a CloudProject fact")
		}
	}
	resultPath := os.Getenv("WORKBUDDY_GOAL2_RESULT")
	if resultPath != "" {
		result := map[string]any{"project_id": projectID, "owner_user_id": manifest.UserA, "other_user_id": manifest.UserB, "status": archived.Project.Status, "version": archived.Project.Version, "workspace_path": workspacePath, "state_root": stateRoot, "scope_a": created.ProjectScopeKey, "scope_b": accountB.ProjectScopeKey}
		if err := writeGoal2E2EResult(resultPath, result); err != nil {
			t.Fatal(err)
		}
	}
}

func newGoal2E2ERuntime(t *testing.T, root, baseURL string, environment cloud.EnvironmentIdentity, actor userctx.ActorContext, userID, token string, capability json.RawMessage) (*cloudRuntime, cloud.ProfileBinding) {
	t.Helper()
	runtimeService, err := newRealCloudRuntime(baseURL, desktopCloudTestProfilePathProvider(root), &http.Client{Timeout: 5 * time.Second}, cloud.DeviceIdentity{DeviceID: "device-goal2-e2e", ClientVersion: "goal2-e2e"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := runtimeService.bindings.ConfirmBinding(context.Background(), actor, userID)
	if err != nil {
		runtimeService.close()
		t.Fatal(err)
	}
	return newGoal2E2ERuntimeFromExistingRuntime(t, runtimeService, actor, binding, token, capability), binding
}

func newGoal2E2ERuntimeFromExisting(t *testing.T, root, baseURL string, environment cloud.EnvironmentIdentity, actor userctx.ActorContext, binding cloud.ProfileBinding, token string, capability json.RawMessage) (*cloudRuntime, cloud.ProfileBinding) {
	t.Helper()
	runtimeService, err := newRealCloudRuntime(baseURL, desktopCloudTestProfilePathProvider(root), &http.Client{Timeout: 5 * time.Second}, cloud.DeviceIdentity{DeviceID: "device-goal2-e2e", ClientVersion: "goal2-e2e"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := runtimeService.bindings.ResolveBinding(context.Background(), actor)
	if err != nil || resolved != binding {
		runtimeService.close()
		t.Fatalf("restart binding=%#v want=%#v err=%v", resolved, binding, err)
	}
	return newGoal2E2ERuntimeFromExistingRuntime(t, runtimeService, actor, resolved, token, capability), resolved
}

func newGoal2E2ERuntimeFromExistingRuntime(t *testing.T, runtimeService *cloudRuntime, actor userctx.ActorContext, binding cloud.ProfileBinding, token string, capability json.RawMessage) *cloudRuntime {
	t.Helper()
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := runtimeService.sessions.Save(context.Background(), actor, cloud.Session{
		Scope: scope, State: cloud.SessionAuthenticated, AccessToken: token,
		RefreshToken: "goal2-e2e-refresh", DeviceID: "device-goal2-e2e",
		AccessTokenExpiresAt: now.Add(45 * time.Minute), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		runtimeService.close()
		t.Fatal(err)
	}
	if err := runtimeService.observeProjectBootstrap(context.Background(), actor, cloud.AccountState{Binding: binding, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: capability}}); err != nil {
		runtimeService.close()
		t.Fatal(err)
	}
	return runtimeService
}
