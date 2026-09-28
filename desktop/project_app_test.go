package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/project"
	"kwu/internal/userctx"
)

func TestWorkBuddyProjectWailsPayloadAndBoundArrays(t *testing.T) {
	tags := map[string]string{
		"ProjectScopeKey": "projectScopeKey", "Items": "items", "NextCursor": "nextCursor",
	}
	pageType := reflect.TypeOf(CloudProjectPageView{})
	for fieldName, want := range tags {
		field, ok := pageType.FieldByName(fieldName)
		if !ok || strings.Split(field.Tag.Get("json"), ",")[0] != want {
			t.Fatalf("CloudProjectPageView.%s json tag = %q", fieldName, field.Tag.Get("json"))
		}
	}
	for _, inputType := range []reflect.Type{
		reflect.TypeOf(CreateCloudProjectInput{}), reflect.TypeOf(PatchCloudProjectInput{}),
		reflect.TypeOf(ProjectVersionMutationInput{}),
	} {
		if _, ok := inputType.FieldByName("IdempotencyKey"); ok {
			t.Fatalf("%s exposes a UI-generated idempotency key", inputType.Name())
		}
	}
	if _, ok := reflect.TypeOf((*App)(nil)).MethodByName("ClaimCloudProjectOnboarding"); !ok {
		t.Fatal("App must expose the server-owned onboarding claim bridge")
	}
	if _, ok := reflect.TypeOf((*App)(nil)).MethodByName("ListJoinedCloudProjects"); !ok {
		t.Fatal("App must expose the server-owned joined project directory bridge")
	}

	app, state := newWorkBuddyProjectApp(t, nil)
	projectTreeChanges := 0
	app.projectTreeChangedHook = func() { projectTreeChanges++ }
	page, err := app.ListCloudProjects(CloudProjectListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if page.ProjectScopeKey != state.scope.Key || page.Items == nil || len(page.Items) != 0 || page.NextCursor.value != nil {
		t.Fatalf("empty page = %#v", page)
	}
	if projectTreeChanges != 1 {
		t.Fatalf("project tree changes = %d, want 1 after refreshing the cached cloud project directory", projectTreeChanges)
	}
	joined, err := app.ListJoinedCloudProjects(CloudProjectListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if joined.ProjectScopeKey != state.scope.Key || len(joined.Items) != 1 || joined.Items[0].ProjectID != "project-shared" || joined.Items[0].Name != "Shared" || joined.Stale {
		t.Fatalf("joined page = %#v", joined)
	}
	if projectTreeChanges != 2 {
		t.Fatalf("project tree changes = %d, want 2 after owned and joined directory refreshes", projectTreeChanges)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); !reflect.DeepEqual(got, []string{"project-shared"}) {
		t.Fatalf("workspace joined projects = %v", got)
	}
	payload, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"items":[]`) || !strings.Contains(string(payload), `"projectScopeKey":"`) {
		t.Fatalf("Wails page payload = %s", payload)
	}
}

func TestAuthenticatedCloudBootstrapPreloadsWorkspaceProjectDirectories(t *testing.T) {
	app, _ := newWorkBuddyProjectApp(t, nil)
	var projectTreeChanges atomic.Int32
	app.projectTreeChangedHook = func() { projectTreeChanges.Add(1) }

	view, err := app.CloudBootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if !cloudViewHasAuthenticatedBinding(view) {
		t.Fatalf("authenticated bootstrap = %#v", view.Account)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); !reflect.DeepEqual(got, []string{"project-shared"}) {
		t.Fatalf("workspace projects immediately after bootstrap = %v", got)
	}
	if got := projectTreeChanges.Load(); got != 2 {
		t.Fatalf("project tree changes = %d, want owned and joined login refreshes", got)
	}
}

func TestJoinedCloudProjectDirectoryRefreshesTreeAndAuthoritativeEmptyRemovesAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	managedRoot, err := ensureCloudProjectWorkspace(context.Background(), state.services, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := addProject(managedRoot, ""); err != nil {
		t.Fatal(err)
	}
	state.ownerUserID = "usr_owner"
	state.name = "Joined detail"
	state.joinedListResponse = `{"data":{"items":[{"project_id":"project-1","owner_user_id":"usr_owner","name":"Joined summary","description":null,"status":"active","version":1,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"}],"next_cursor":null},"error":null,"request_id":"req-list-joined-project-1","server_time":"2026-08-05T04:00:02Z"}`
	treeChanges := 0
	app.projectTreeChangedHook = func() { treeChanges++ }

	if _, err := app.ListJoinedCloudProjects(CloudProjectListInput{}); err != nil {
		t.Fatal(err)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); !reflect.DeepEqual(got, []string{"project-1"}) {
		t.Fatalf("workspace after joined refresh = %v", got)
	}
	if treeChanges != 1 {
		t.Fatalf("tree changes after joined refresh = %d", treeChanges)
	}

	if _, err := app.GetCloudProject("project-1"); err != nil {
		t.Fatal(err)
	}
	if treeChanges != 2 {
		t.Fatalf("tree changes after joined detail = %d", treeChanges)
	}
	if node := findCloudProjectTreeNode(app.ListProjectTree(), "project-1"); node == nil || node.Label != "Joined detail" {
		t.Fatalf("joined detail was not reflected immediately: %#v", node)
	}

	state.joinedListResponse = `{"data":{"items":[],"next_cursor":null},"error":null,"request_id":"req-list-joined-empty","server_time":"2026-08-05T04:00:03Z"}`
	if _, err := app.ListJoinedCloudProjects(CloudProjectListInput{}); err != nil {
		t.Fatal(err)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); len(got) != 0 {
		t.Fatalf("workspace after authoritative member removal = %v", got)
	}
	for _, node := range app.ListProjectTree() {
		if node.Kind == "project" && sameDesktopPath(node.Root, managedRoot) {
			t.Fatalf("revoked project leaked its internal workspace: %#v", node)
		}
	}
	if _, err := app.GetCloudProject("project-1"); err != nil {
		t.Fatal(err)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); len(got) != 0 {
		t.Fatalf("detail cache recreated revoked joined project = %v", got)
	}
}

func TestJoinedCloudProjectDirectoryRejectsOlderResponse(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.blockJoinedList = true
	state.joinedListStarted = make(chan struct{})
	state.joinedListRelease = make(chan struct{})
	state.joinedBlockedResponse = `{"data":{"items":[{"project_id":"project-old","owner_user_id":"usr_owner","name":"Old","description":null,"status":"active","version":1,"created_at":"2026-08-05T03:00:00Z","updated_at":"2026-08-05T03:00:02Z"}],"next_cursor":null},"error":null,"request_id":"req-list-joined-old","server_time":"2026-08-05T04:00:02Z"}`
	older := make(chan error, 1)
	go func() {
		_, err := app.ListJoinedCloudProjects(CloudProjectListInput{})
		older <- err
	}()
	select {
	case <-state.joinedListStarted:
	case <-time.After(time.Second):
		t.Fatal("older joined directory request did not start")
	}

	state.blockJoinedList = false
	state.joinedListResponse = `{"data":{"items":[{"project_id":"project-new","owner_user_id":"usr_owner","name":"New","description":null,"status":"active","version":2,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:03Z"}],"next_cursor":null},"error":null,"request_id":"req-list-joined-new","server_time":"2026-08-05T04:00:03Z"}`
	if _, err := app.ListJoinedCloudProjects(CloudProjectListInput{}); err != nil {
		t.Fatal(err)
	}
	close(state.joinedListRelease)
	if err := <-older; err != nil {
		t.Fatal(err)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); !reflect.DeepEqual(got, []string{"project-new"}) {
		t.Fatalf("workspace after older joined response = %v", got)
	}
}

func TestCloudProjectTreeProjectsDeduplicatesWithOwnedDirectoryPrecedence(t *testing.T) {
	_, state := newWorkBuddyProjectApp(t, nil)
	owned := project.Project{ProjectID: "project-1", OwnerUserID: state.scope.Binding.CloudUserID, Name: "Owned", Status: project.ProjectActive, Version: 2}
	joinedDuplicate := project.Project{ProjectID: "project-1", OwnerUserID: "usr_other", Name: "Member duplicate", Status: project.ProjectActive, Version: 3}
	joined := project.Project{ProjectID: "project-2", OwnerUserID: "usr_other", Name: "Member", Status: project.ProjectActive, Version: 1}
	projects := cloudProjectTreeProjects(project.ProjectCacheSnapshot{
		Summaries:               project.ProjectPage{Items: []project.Project{owned}},
		JoinedSummaries:         project.ProjectPage{Items: []project.Project{joinedDuplicate, joined}},
		OwnedDirectoryRefreshed: true, JoinedDirectoryRefreshed: true,
	}, state.scope)
	if len(projects) != 2 || projects[0].ProjectID != "project-1" || projects[0].Name != "Owned" || projects[0].OwnerUserID != state.scope.Binding.CloudUserID || projects[1].ProjectID != "project-2" {
		t.Fatalf("deduplicated projects = %#v", projects)
	}
}

func findCloudProjectTreeNode(nodes []ProjectNode, projectID string) *ProjectNode {
	for index := range nodes {
		if nodes[index].Kind == "cloud_project" && nodes[index].CloudProjectID == projectID {
			return &nodes[index]
		}
	}
	return nil
}

func TestAppDoesNotExposeLocalWorkspaceBindingOperations(t *testing.T) {
	for _, methodName := range []string{
		"BindCloudProjectWorkspace",
		"RebindCloudProjectWorkspace",
		"ListCloudProjectBindings",
		"UnbindCloudProjectWorkspace",
	} {
		if _, ok := reflect.TypeOf((*App)(nil)).MethodByName(methodName); ok {
			t.Fatalf("App must not expose the local workspace binding method %s", methodName)
		}
	}
}

func TestWorkBuddyProjectServicesDistinguishUnavailableFromInitializationFailure(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.runtime.projectMu.Lock()
	state.runtime.projectCapability = project.CapabilityAvailable
	state.runtime.projects = nil
	state.runtime.projectMu.Unlock()

	_, _, err := app.projectServicesAndFeaturesForOperation()
	var internal *cloud.Error
	if !errors.As(err, &internal) || internal.Code != cloud.ErrorCode("INTERNAL_ERROR") || !strings.Contains(internal.Message, "not initialized") {
		t.Fatalf("uninitialized services error = %#v", err)
	}

	state.runtime.projectMu.Lock()
	state.runtime.projectCapability = project.CapabilityUnavailable
	state.runtime.projectMu.Unlock()
	_, _, err = app.projectServicesAndFeaturesForOperation()
	var unavailable *cloud.Error
	if !errors.As(err, &unavailable) || unavailable.Code != cloud.ErrorCode("CAPABILITY_UNAVAILABLE") {
		t.Fatalf("unavailable capability error = %#v", err)
	}
	state.runtime.projectMu.Lock()
	state.runtime.projects = state.services
	state.runtime.projectMu.Unlock()
}

func TestWorkBuddyProjectWailsMutationReadbackJournalAndInternalBindings(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	projectTreeChanges := 0
	app.projectTreeChangedHook = func() { projectTreeChanges++ }
	description := "项目公开说明"
	humanDescription := "调研模板"
	agentInstruction := "# 角色\n请调研"
	skillID, expertID, connectorID := "@reasonix/market-research", "exp_market_analyst", "github"
	created, err := app.CreateCloudProject(CreateCloudProjectInput{
		Name: "Alpha", DescriptionSet: true, Description: &description, HumanDescription: &humanDescription, AgentInstruction: &agentInstruction,
		SkillID: &skillID, ExpertID: &expertID, ConnectorID: &connectorID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ProjectScopeKey != state.scope.Key || created.Project.ProjectID != "project-1" || created.Project.Version != 1 {
		t.Fatalf("created = %#v", created)
	}
	if created.WorkspacePath == "" || filepath.Dir(created.WorkspacePath) != filepath.Join(state.services.StateRoot, cloudProjectWorkspacesDir, state.scope.Key) {
		t.Fatalf("created project workspace = %q", created.WorkspacePath)
	}
	if info, statErr := os.Stat(created.WorkspacePath); statErr != nil || !info.IsDir() {
		t.Fatalf("created project workspace is unavailable: %v", statErr)
	}
	if projectTreeChanges != 1 {
		t.Fatalf("project tree changes = %d, want 1 after create cache readback", projectTreeChanges)
	}
	foundCreatedProject := false
	for _, node := range app.ListProjectTree() {
		if node.Kind == "cloud_project" && node.CloudProjectID == created.Project.ProjectID {
			foundCreatedProject = true
			break
		}
	}
	if !foundCreatedProject {
		t.Fatal("created CloudProject is absent from the workspace tree before any list refresh")
	}
	if state.lastKey == "" || strings.Contains(state.lastBody, "idempotency") {
		t.Fatalf("mutation key/body = %q / %s", state.lastKey, state.lastBody)
	}
	if !strings.Contains(state.lastBody, `"description":"项目公开说明"`) || !strings.Contains(state.lastBody, `"human_description":"调研模板"`) || !strings.Contains(state.lastBody, `"agent_instruction":"# 角色\n请调研"`) || !strings.Contains(state.lastBody, `"skill_id":"@reasonix/market-research"`) || !strings.Contains(state.lastBody, `"expert_id":"exp_market_analyst"`) || !strings.Contains(state.lastBody, `"connector_id":"github"`) || state.instructionVersion != 1 {
		t.Fatalf("initial instruction body/readback = %s / v%d", state.lastBody, state.instructionVersion)
	}
	assertWorkBuddyProjectJournalEmpty(t, state)
	_, err = app.CreateCloudProject(CreateCloudProjectInput{Name: ""})
	var validation *cloud.Error
	if !errors.As(err, &validation) || validation.Code != cloud.ErrorCode("VALIDATION_FAILED") {
		t.Fatalf("validation error = %#v", err)
	}

	name := "Beta"
	patched, err := app.PatchCloudProject(PatchCloudProjectInput{ProjectID: "project-1", ExpectedVersion: 1, Name: &name})
	if err != nil || patched.Project.Name != "Beta" || patched.Project.Version != 2 {
		t.Fatalf("patched = %#v, err=%v", patched, err)
	}
	archived, err := app.ArchiveCloudProject(ProjectVersionMutationInput{ProjectID: "project-1", ExpectedVersion: 2})
	if err != nil || archived.Project.Status != "archived" || archived.Project.Version != 3 {
		t.Fatalf("archived = %#v, err=%v", archived, err)
	}
	restored, err := app.RestoreCloudProject(ProjectVersionMutationInput{ProjectID: "project-1", ExpectedVersion: 3})
	if err != nil || restored.Project.Status != "active" || restored.Project.Version != 4 {
		t.Fatalf("restored = %#v, err=%v", restored, err)
	}
	read, err := app.GetCloudProject("project-1")
	if err != nil || read.ProjectScopeKey != state.scope.Key || read.Project.Version != 4 {
		t.Fatalf("read = %#v, err=%v", read, err)
	}
	assertWorkBuddyProjectJournalEmpty(t, state)

	first, second := t.TempDir(), t.TempDir()
	bound, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", first)
	if err != nil || bound.ProjectScopeKey != state.scope.Key || bound.Status != project.LocalWorkspaceAvailable {
		t.Fatalf("bound = %#v, err=%v", bound, err)
	}
	rebound, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", second)
	if err != nil || rebound.DisplayPath == bound.DisplayPath {
		t.Fatalf("rebound = %#v, err=%v", rebound, err)
	}
	bindings, err := state.services.Bindings.List(context.Background(), state.scope)
	if err != nil || len(bindings) != 1 || bindings[0].ProjectScopeKey != state.scope.Key {
		t.Fatalf("bindings = %#v, err=%v", bindings, err)
	}
	resolved, found, err := state.services.Bindings.Get(context.Background(), state.scope, "project-1")
	if err != nil || !found || resolved.DisplayPath != rebound.DisplayPath {
		t.Fatalf("internal binding read = %#v, found=%v, err=%v", resolved, found, err)
	}
	if err := state.services.Bindings.Unbind(context.Background(), state.scope, "project-1"); err != nil {
		t.Fatalf("internal unbind: %v", err)
	}
	bindings, err = state.services.Bindings.List(context.Background(), state.scope)
	if err != nil || bindings == nil || len(bindings) != 0 {
		t.Fatalf("empty bindings = %#v, err=%v", bindings, err)
	}
	if strings.Contains(state.allBodies(), first) || strings.Contains(state.allBodies(), second) {
		t.Fatal("local workspace path entered a Cloud request")
	}
}

func TestCreateCloudProjectPublishesStableTreeOrderAndRejectsOlderDirectoryResponse(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.ownedListResponse = `{"data":{"items":[{"project_id":"project-2","owner_user_id":"usr_project","name":"Older 2","description":null,"status":"active","version":1,"created_at":"2026-08-05T03:00:00Z","updated_at":"2026-08-05T03:00:02Z"},{"project_id":"project-3","owner_user_id":"usr_project","name":"Older 3","description":null,"status":"active","version":1,"created_at":"2026-08-05T02:00:00Z","updated_at":"2026-08-05T02:00:02Z"}],"next_cursor":null},"error":null,"request_id":"req-list-existing","server_time":"2026-08-05T04:00:02Z"}`
	if _, err := app.ListCloudProjects(CloudProjectListInput{}); err != nil {
		t.Fatal(err)
	}

	state.blockList = true
	state.listStarted = make(chan struct{})
	state.listRelease = make(chan struct{})
	staleResult := make(chan error, 1)
	go func() {
		_, err := app.ListCloudProjects(CloudProjectListInput{})
		staleResult <- err
	}()
	select {
	case <-state.listStarted:
	case <-time.After(time.Second):
		t.Fatal("older project directory request did not start")
	}

	created, err := app.CreateCloudProject(CreateCloudProjectInput{Name: "Newest"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{created.Project.ProjectID, "project-2", "project-3"}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); !reflect.DeepEqual(got, want) {
		t.Fatalf("first tree snapshot order = %v, want %v", got, want)
	}

	close(state.listRelease)
	if err := <-staleResult; err != nil {
		t.Fatalf("older directory response returned error: %v", err)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); !reflect.DeepEqual(got, want) {
		t.Fatalf("tree order after older response = %v, want %v", got, want)
	}

	state.blockList = false
	state.ownedListResponse = `{"data":{"items":[{"project_id":"project-1","owner_user_id":"usr_project","name":"Newest","description":null,"status":"active","version":1,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"},{"project_id":"project-2","owner_user_id":"usr_project","name":"Older 2","description":null,"status":"active","version":1,"created_at":"2026-08-05T03:00:00Z","updated_at":"2026-08-05T03:00:02Z"},{"project_id":"project-3","owner_user_id":"usr_project","name":"Older 3","description":null,"status":"active","version":1,"created_at":"2026-08-05T02:00:00Z","updated_at":"2026-08-05T02:00:02Z"}],"next_cursor":null},"error":null,"request_id":"req-list-refreshed","server_time":"2026-08-05T04:00:03Z"}`
	if _, err := app.ListCloudProjects(CloudProjectListInput{}); err != nil {
		t.Fatal(err)
	}
	if got := cloudProjectTreeIDs(app.ListProjectTree()); !reflect.DeepEqual(got, want) {
		t.Fatalf("tree order after authoritative refresh = %v, want %v", got, want)
	}
}

func cloudProjectTreeIDs(nodes []ProjectNode) []string {
	ids := make([]string, 0)
	for _, node := range nodes {
		if node.Kind == "cloud_project" {
			ids = append(ids, node.CloudProjectID)
		}
	}
	return ids
}

func TestWorkBuddyProjectWailsAggregateCreateCanonicalizesBindingPayload(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.runtime.projectMu.Lock()
	state.runtime.projectFeatures.Members = true
	state.runtime.projectMu.Unlock()
	created, err := app.CreateCloudProject(CreateCloudProjectInput{
		Name: "Bound",
		Bindings: []CloudProjectCapabilityBindingInput{{
			Kind: "connector", RegistryRef: "notion", Position: 0, Config: map[string]any{},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Project.ProjectID != "project-1" || !strings.Contains(state.lastBody, `"registry_ref":"notion"`) || strings.Contains(state.lastBody, `"registryRef"`) {
		t.Fatalf("aggregate create = %#v, body = %s", created, state.lastBody)
	}
	assertWorkBuddyProjectJournalEmpty(t, state)
}

func TestWorkBuddyProjectCreateDoesNotInviteDuplicateRetryAfterCacheFailure(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	invalidRoot := filepath.Join(t.TempDir(), "cache-root")
	if err := os.WriteFile(invalidRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	cache, err := project.NewProjectCache(project.ProjectCacheOptions{
		Root: invalidRoot,
		Scopes: cloudProjectScopeResolver{
			bindings: state.runtime.bindings, environment: state.runtime.projectEnvironment, actor: actor,
		},
		Now: time.Now, ContractVersion: project.ContractVersion, ContractHash: project.CanonicalContractSHA256(),
	})
	if err != nil {
		t.Fatal(err)
	}
	state.services.Cache = cache
	projectTreeChanges := 0
	app.projectTreeChangedHook = func() { projectTreeChanges++ }

	created, err := app.CreateCloudProject(CreateCloudProjectInput{Name: "Cache failure"})
	if err != nil || created.Project.ProjectID != "project-1" {
		t.Fatalf("committed create result = %#v, %v", created, err)
	}
	if projectTreeChanges != 0 {
		t.Fatalf("project tree changes = %d, want 0 after cache failure", projectTreeChanges)
	}
}

func TestWorkBuddyProjectDeleteEvictsWorkspaceTreeImmediately(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	projectTreeChanges := 0
	app.projectTreeChangedHook = func() { projectTreeChanges++ }
	created, err := app.CreateCloudProject(CreateCloudProjectInput{Name: "Delete cache"})
	if err != nil {
		t.Fatal(err)
	}
	managedRoot, err := ensureCloudProjectWorkspace(context.Background(), state.services, created.Project.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if err := addProject(managedRoot, ""); err != nil {
		t.Fatal(err)
	}
	state.runtime.projectMu.Lock()
	state.runtime.projectFeatures.Members = true
	state.runtime.projectMu.Unlock()
	projectTreeChanges = 0
	if err := app.DeleteCloudProject(DeleteCloudProjectInput{ProjectID: created.Project.ProjectID, ExpectedVersion: created.Project.Version}); err != nil {
		t.Fatal(err)
	}
	if projectTreeChanges != 1 {
		t.Fatalf("project tree changes = %d, want 1 after delete eviction", projectTreeChanges)
	}
	for _, node := range app.ListProjectTree() {
		if node.Kind == "cloud_project" && node.CloudProjectID == created.Project.ProjectID {
			t.Fatalf("deleted cloud project remains in workspace tree: %#v", node)
		}
		if node.Kind == "project" && sameDesktopPath(node.Root, managedRoot) {
			t.Fatalf("deleted project leaked its internal workspace: %#v", node)
		}
	}
	state.mu.Lock()
	deleted := state.projectDeleted
	state.mu.Unlock()
	if !deleted {
		t.Fatal("Cloud delete request was not committed before workspace eviction")
	}
}

func TestWorkBuddyProjectDeleteRejectsJoinedProjectMember(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.runtime.projectMu.Lock()
	state.runtime.projectFeatures.Members = true
	state.runtime.projectMu.Unlock()
	state.ownerUserID = "usr_other_owner"

	err := app.DeleteCloudProject(DeleteCloudProjectInput{ProjectID: "project-1", ExpectedVersion: 1})
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("FORBIDDEN") {
		t.Fatalf("DeleteCloudProject error = %#v, want FORBIDDEN", err)
	}
	if state.projectDeleted {
		t.Fatal("joined project member reached the delete endpoint")
	}
}

func TestWorkBuddyProjectWailsExpectedVersionSafeIntegerBoundary(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.mu.Lock()
	state.version = project.MaxExpectedVersion
	state.mu.Unlock()
	name := "Boundary"
	result, err := app.PatchCloudProject(PatchCloudProjectInput{ProjectID: "project-1", ExpectedVersion: project.MaxExpectedVersion, Name: &name})
	if err != nil || result.Project.Version != project.MaxSafeJSONInteger {
		t.Fatalf("maximum expected version result = %#v, %v", result, err)
	}
	beforeBodies := state.allBodies()
	_, err = app.PatchCloudProject(PatchCloudProjectInput{ProjectID: "project-1", ExpectedVersion: project.MaxSafeJSONInteger, Name: &name})
	var validation *cloud.Error
	if !errors.As(err, &validation) || validation.Code != cloud.ErrorCode("VALIDATION_FAILED") {
		t.Fatalf("unsafe expected version error = %#v", err)
	}
	for name, mutate := range map[string]func() error{
		"archive": func() error {
			_, mutationErr := app.ArchiveCloudProject(ProjectVersionMutationInput{ProjectID: "project-1", ExpectedVersion: project.MaxSafeJSONInteger})
			return mutationErr
		},
		"restore": func() error {
			_, mutationErr := app.RestoreCloudProject(ProjectVersionMutationInput{ProjectID: "project-1", ExpectedVersion: project.MaxSafeJSONInteger})
			return mutationErr
		},
	} {
		t.Run(name, func(t *testing.T) {
			var mutationValidation *cloud.Error
			if err := mutate(); !errors.As(err, &mutationValidation) || mutationValidation.Code != cloud.ErrorCode("VALIDATION_FAILED") {
				t.Fatalf("unsafe expected version error = %#v", err)
			}
		})
	}
	if state.allBodies() != beforeBodies {
		t.Fatal("unsafe expected version reached Cloud")
	}
	assertWorkBuddyProjectJournalEmpty(t, state)
}

func TestWorkBuddyProjectWailsTimeoutMarksUnknownAndMapsErrors(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, &http.Client{Timeout: 25 * time.Millisecond})
	state.timeoutMutation = true
	_, err := app.CreateCloudProject(CreateCloudProjectInput{Name: "Timeout"})
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorNetworkUnavailable || !typed.Retryable {
		t.Fatalf("timeout error = %#v", err)
	}
	pending, pendingErr := state.services.Journal.Pending(context.Background(), state.scope)
	if pendingErr != nil || len(pending) != 1 || pending[0].State != project.ProjectCommandUnknown {
		t.Fatalf("pending = %#v, err=%v", pending, pendingErr)
	}
}

func TestWorkBuddyProjectWailsConclusiveRejectionDiscardsJournal(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.conflictMutation = true
	_, err := app.CreateCloudProject(CreateCloudProjectInput{Name: "Conflict"})
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("CONFLICT") || typed.Retryable {
		t.Fatalf("conflict error = %#v", err)
	}
	assertWorkBuddyProjectJournalEmpty(t, state)
}

func TestWorkBuddyProjectWailsListNeverFallsBackToLocalCache(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, &http.Client{Timeout: 25 * time.Millisecond})
	fresh, err := app.ListCloudProjects(CloudProjectListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Stale || fresh.CachedAt.value != nil {
		t.Fatalf("fresh list marked stale: %#v", fresh)
	}
	state.timeoutList = true
	if _, err := app.ListCloudProjects(CloudProjectListInput{}); err == nil {
		t.Fatal("default project directory incorrectly used the local cache after a server failure")
	}
	if _, err := app.ListCloudProjects(CloudProjectListInput{Search: "Alpha"}); err == nil {
		t.Fatal("searched project directory incorrectly used the local cache after a server failure")
	}
}

func TestWorkBuddyProjectWailsDetailCacheIsStaleReadOnlyAndFailsClosed(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, &http.Client{Timeout: 25 * time.Millisecond})
	fresh, err := app.GetCloudProject("project-1")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Stale || fresh.CachedAt.value != nil {
		t.Fatalf("fresh detail marked stale: %#v", fresh)
	}
	state.timeoutGet = true
	state.timeoutGetFinished = make(chan struct{})
	stale, err := app.GetCloudProject("project-1")
	if err != nil {
		t.Fatal(err)
	}
	if !stale.Stale || stale.CachedAt.value == nil || stale.ProjectScopeKey != state.scope.Key || stale.Project.ProjectID != "project-1" {
		t.Fatalf("retryable detail fallback = %#v", stale)
	}
	select {
	case <-state.timeoutGetFinished:
	case <-time.After(time.Second):
		t.Fatal("timed-out detail handler did not finish")
	}
	state.timeoutGet = false
	state.notFoundGet = true
	if _, err := app.GetCloudProject("project-1"); err == nil {
		t.Fatal("not-found detail incorrectly used cache fallback")
	}
}

func TestWorkBuddyProjectWailsReadsDoNotUseGlobalCloudOperationMutex(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.runtime.operationMu.Lock()
	defer state.runtime.operationMu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := app.ListCloudProjects(CloudProjectListInput{})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("project read serialized behind the global cloud operation mutex")
	}
}

func TestWorkBuddyProjectWailsCancelsDelayedReadOnAccountScopeChange(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.blockList = true
	state.listStarted = make(chan struct{})
	state.listCanceled = make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := app.ListCloudProjects(CloudProjectListInput{})
		result <- err
	}()
	select {
	case <-state.listStarted:
	case <-time.After(time.Second):
		t.Fatal("delayed user-A project read did not start")
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindingB, err := state.runtime.bindings.Rebind(context.Background(), actor, "usr_project_b")
	if err != nil {
		t.Fatal(err)
	}
	capability := json.RawMessage(`{"enabled":true,"contract_version":"` + project.ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + project.CanonicalContractSHA256() + `","instructions":true,"work_items":true}`)
	if err := state.runtime.observeProjectBootstrap(context.Background(), actor, cloud.AccountState{Binding: bindingB, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: capability}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-state.listCanceled:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("account scope change did not cancel delayed user-A HTTP read")
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled user-A project read produced a visible result")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled user-A project read did not return")
	}
}

func TestWorkBuddyProjectWailsLogoutClearsProjectServices(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	operation, cancel := state.services.operationContext(context.Background())
	defer cancel()
	_, _ = app.CloudLogout()
	select {
	case <-operation.Done():
	default:
		t.Fatal("logout did not cancel project operations")
	}
	state.runtime.projectMu.RLock()
	defer state.runtime.projectMu.RUnlock()
	if state.runtime.projects != nil || state.runtime.projectCapability != project.CapabilityUnavailable {
		t.Fatalf("logout retained project services: status=%q services=%#v", state.runtime.projectCapability, state.runtime.projects)
	}
}

type workBuddyProjectAppState struct {
	mu                    sync.Mutex
	runtime               *cloudRuntime
	services              *cloudProjectServices
	scope                 project.Scope
	name                  string
	ownerUserID           string
	status                string
	version               uint64
	instructionHuman      string
	instructionAgent      string
	instructionVersion    uint64
	instructionGetCount   int
	lastKey               string
	lastBody              string
	lastReadbackPath      string
	bodies                []string
	timeoutMutation       bool
	networkMutation       bool
	conflictMutation      bool
	blockMutation         bool
	mutationStarted       chan struct{}
	mutationStartOnce     sync.Once
	timeoutList           bool
	timeoutGet            bool
	timeoutInstructionGet bool
	instructionConflict   bool
	workItemTitle         string
	workItemDescription   *string
	workItemStatus        string
	workItemRank          int64
	workItemVersion       uint64
	workItemDeleted       bool
	lastWorkItemRawQuery  string
	projectDeleted        bool
	timeoutGetFinished    chan struct{}
	timeoutGetOnce        sync.Once
	notFoundGet           bool
	blockList             bool
	listStarted           chan struct{}
	listCanceled          chan struct{}
	listStartOnce         sync.Once
	listCancelOnce        sync.Once
	listRelease           chan struct{}
	ownedListResponse     string
	joinedListResponse    string
	blockJoinedList       bool
	joinedListStarted     chan struct{}
	joinedListRelease     chan struct{}
	joinedBlockedResponse string
	joinedListStartOnce   sync.Once
}

func nullableProjectTestString(value *string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func newWorkBuddyProjectApp(t *testing.T, client *http.Client) (*App, *workBuddyProjectAppState) {
	t.Helper()
	state := &workBuddyProjectAppState{name: "Alpha", ownerUserID: "usr_project", status: "active", version: 1, instructionVersion: 1, workItemTitle: "First task", workItemStatus: "todo", workItemRank: 1000, workItemVersion: 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		state.serve(t, w, request)
	}))
	t.Cleanup(server.Close)
	if client == nil {
		client = server.Client()
	}
	root := t.TempDir()
	paths := desktopCloudTestProfilePathProvider(root)
	environment := desktopTestCloudEnvironment(server.URL)
	runtimeService, err := newRealCloudRuntime(server.URL, paths, client, cloud.DeviceIdentity{DeviceID: "device-project-app", ClientVersion: "test"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimeService.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	binding, err := runtimeService.bindings.ConfirmBinding(context.Background(), actor, "usr_project")
	if err != nil {
		t.Fatal(err)
	}
	bindingScope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := runtimeService.sessions.Save(context.Background(), actor, cloud.Session{
		Scope: bindingScope, State: cloud.SessionAuthenticated, AccessToken: "access-project", RefreshToken: "refresh-project", DeviceID: "device-project-app",
		AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtimeService.remoteAccount.CurrentUser(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	runtimeService.projectMu.RLock()
	state.services = runtimeService.projects
	state.scope = runtimeService.projects.Scope
	runtimeService.projectMu.RUnlock()
	state.runtime = runtimeService
	return &App{cloudRuntime: runtimeService, ctx: context.Background()}, state
}

func (state *workBuddyProjectAppState) serve(t *testing.T, w http.ResponseWriter, request *http.Request) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if request.URL.Path == "/api/v1/me" {
		fmt.Fprintf(w, `{"data":{"user":{"user_id":"usr_project","display_name":"Project User","status":"active","role":"user","created_at":"2026-08-05T04:00:00Z"},"balance":{"total_balance":"1.0000","free_balance":"1.0000","subscription_balance":"0.0000","available_balance":"1.0000","free_available_balance":"1.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0},"capabilities":{"projects_v1":{"enabled":true,"contract_version":%q,"contract_hash_algorithm":"sha256","contract_hash":%q,"instructions":true,"work_items":true}}},"error":null,"request_id":"req-me","server_time":"2026-08-05T04:00:01Z"}`, project.ContractVersion, project.CanonicalContractSHA256())
		return
	}
	if request.Method == http.MethodPost && request.URL.Path == "/api/v1/projects" && state.blockMutation {
		state.mutationStartOnce.Do(func() { close(state.mutationStarted) })
		select {
		case <-request.Context().Done():
			return
		case <-time.After(750 * time.Millisecond):
		}
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects" && state.blockList {
		state.listStartOnce.Do(func() { close(state.listStarted) })
		release := state.listRelease
		if release != nil {
			select {
			case <-request.Context().Done():
				state.listCancelOnce.Do(func() { close(state.listCanceled) })
				return
			case <-release:
			}
		} else {
			select {
			case <-request.Context().Done():
				state.listCancelOnce.Do(func() { close(state.listCanceled) })
				return
			case <-time.After(750 * time.Millisecond):
			}
		}
		fmt.Fprint(w, `{"data":{"items":[],"next_cursor":null},"error":null,"request_id":"req-stale-list","server_time":"2026-08-05T04:00:02Z"}`)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects" && state.timeoutList {
		time.Sleep(100 * time.Millisecond)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects/joined" && state.blockJoinedList {
		state.joinedListStartOnce.Do(func() { close(state.joinedListStarted) })
		select {
		case <-request.Context().Done():
			return
		case <-state.joinedListRelease:
		}
		fmt.Fprint(w, state.joinedBlockedResponse)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects/project-1" && state.timeoutGet {
		defer state.timeoutGetOnce.Do(func() { close(state.timeoutGetFinished) })
		select {
		case <-request.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects/project-1" && state.notFoundGet {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"data":null,"error":{"code":"NOT_FOUND","message":"Project not found.","details":null,"retryable":false},"request_id":"req-not-found","server_time":"2026-08-05T04:00:03Z"}`)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects/project-1/instructions" && state.timeoutInstructionGet {
		time.Sleep(100 * time.Millisecond)
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if request.URL.Path == "/api/v1/projects/aggregates" && request.Method == http.MethodPost {
		var body json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		var input struct {
			Name             string                        `json:"name"`
			HumanDescription string                        `json:"human_description"`
			AgentInstruction string                        `json:"agent_instruction"`
			Bindings         []project.ProjectBindingInput `json:"bindings"`
		}
		if err := json.Unmarshal(body, &input); err != nil || len(input.Bindings) == 0 {
			t.Fatalf("aggregate create body = %s, error = %v", body, err)
		}
		state.lastBody = string(body)
		state.bodies = append(state.bodies, string(body))
		state.lastKey = request.Header.Get("X-Idempotency-Key")
		state.name = input.Name
		state.version = 1
		state.instructionHuman = input.HumanDescription
		state.instructionAgent = input.AgentInstruction
		state.instructionVersion = 1
		fmt.Fprint(w, `{"data":{"project_id":"project-1","project_version":1,"instruction_version":1,"binding_ids":["binding-1"],"created_at":"2026-08-05T04:00:00Z"},"error":null,"request_id":"req-aggregate-create","server_time":"2026-08-05T04:00:02Z"}`)
		return
	}
	if request.URL.Path == "/api/v1/projects/project-1/instructions" {
		switch request.Method {
		case http.MethodGet:
			state.instructionGetCount++
			state.lastReadbackPath = request.URL.Path
			fmt.Fprintf(w, `{"data":{"project_id":"project-1","human_description":%q,"agent_instruction":%q,"content_hash":%q,"version":%d,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"},"error":null,"request_id":"req-instruction-get","server_time":"2026-08-05T04:00:03Z"}`,
				state.instructionHuman, state.instructionAgent, project.InstructionContentSHA256(state.instructionHuman, state.instructionAgent), state.instructionVersion)
			return
		case http.MethodPut:
			var body json.RawMessage
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			state.lastBody = string(body)
			state.bodies = append(state.bodies, string(body))
			state.lastKey = request.Header.Get("X-Idempotency-Key")
			if state.instructionConflict || state.status == "archived" {
				w.WriteHeader(http.StatusConflict)
				reason := "version_mismatch"
				if state.status == "archived" {
					reason = "project_archived"
				}
				fmt.Fprintf(w, `{"data":null,"error":{"code":"CONFLICT","message":"Conflict.","details":{"reason":%q},"retryable":false},"request_id":"req-instruction-conflict","server_time":"2026-08-05T04:00:02Z"}`, reason)
				return
			}
			var input struct {
				ExpectedVersion  uint64 `json:"expected_version"`
				HumanDescription string `json:"human_description"`
				AgentInstruction string `json:"agent_instruction"`
			}
			if err := json.Unmarshal(body, &input); err != nil {
				t.Fatal(err)
			}
			if input.ExpectedVersion != state.instructionVersion {
				t.Fatalf("instruction expected version=%d current=%d", input.ExpectedVersion, state.instructionVersion)
			}
			state.instructionHuman, state.instructionAgent = input.HumanDescription, input.AgentInstruction
			state.instructionVersion++
			fmt.Fprintf(w, `{"data":{"operation_id":"op-instruction-%d","resource_kind":"instruction","resource_id":"project-1","project_id":"project-1","committed_version":%d},"error":null,"request_id":"req-instruction-put","server_time":"2026-08-05T04:00:02Z"}`, state.instructionVersion, state.instructionVersion)
			return
		}
	}
	if request.URL.Path == "/api/v1/projects/project-1/work-items" {
		switch request.Method {
		case http.MethodGet:
			state.lastWorkItemRawQuery = request.URL.RawQuery
			if state.workItemDeleted {
				fmt.Fprint(w, `{"data":{"items":[],"next_cursor":null,"total_count":0,"undated_count":0},"error":null,"request_id":"req-work-list","server_time":"2026-08-05T04:00:03Z"}`)
				return
			}
			fmt.Fprintf(w, `{"data":{"items":[{"work_item_id":"work-1","project_id":"project-1","title":%q,"description":%s,"status":%q,"rank":%d,"version":%d,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"}],"next_cursor":null,"total_count":1,"undated_count":1},"error":null,"request_id":"req-work-list","server_time":"2026-08-05T04:00:03Z"}`, state.workItemTitle, nullableProjectTestString(state.workItemDescription), state.workItemStatus, state.workItemRank, state.workItemVersion)
			return
		case http.MethodPost:
			var body json.RawMessage
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			var input struct {
				Title       string  `json:"title"`
				Description *string `json:"description"`
				Status      string  `json:"status"`
			}
			if err := json.Unmarshal(body, &input); err != nil {
				t.Fatal(err)
			}
			state.lastBody = string(body)
			state.bodies = append(state.bodies, string(body))
			state.lastKey = request.Header.Get("X-Idempotency-Key")
			state.workItemTitle = input.Title
			state.workItemDescription = input.Description
			if input.Status != "" {
				state.workItemStatus = input.Status
			}
			state.workItemVersion = 1
			fmt.Fprint(w, `{"data":{"operation_id":"op-work-create","resource_kind":"work_item","resource_id":"work-1","project_id":"project-1","committed_version":1},"error":null,"request_id":"req-work-create","server_time":"2026-08-05T04:00:02Z"}`)
			return
		}
	}
	if request.URL.Path == "/api/v1/work-items/work-1" {
		if request.Method == http.MethodGet {
			state.lastReadbackPath = request.URL.Path
			if state.workItemDeleted {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"data":null,"error":{"code":"NOT_FOUND","message":"Work item not found.","details":null,"retryable":false},"request_id":"req-work-not-found","server_time":"2026-08-05T04:00:03Z"}`)
				return
			}
			fmt.Fprintf(w, `{"data":{"work_item_id":"work-1","project_id":"project-1","title":%q,"description":%s,"status":%q,"rank":%d,"version":%d,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"},"error":null,"request_id":"req-work-get","server_time":"2026-08-05T04:00:03Z"}`, state.workItemTitle, nullableProjectTestString(state.workItemDescription), state.workItemStatus, state.workItemRank, state.workItemVersion)
			return
		}
		if request.Method == http.MethodPatch {
			var body json.RawMessage
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			var input struct {
				Title       *string `json:"title"`
				Description *string `json:"description"`
			}
			if err := json.Unmarshal(body, &input); err != nil {
				t.Fatal(err)
			}
			state.lastBody = string(body)
			state.bodies = append(state.bodies, string(body))
			state.lastKey = request.Header.Get("X-Idempotency-Key")
			if input.Title != nil {
				state.workItemTitle = *input.Title
			}
			state.workItemDescription = input.Description
			state.workItemVersion++
			fmt.Fprintf(w, `{"data":{"operation_id":"op-work-patch","resource_kind":"work_item","resource_id":"work-1","project_id":"project-1","committed_version":%d},"error":null,"request_id":"req-work-patch","server_time":"2026-08-05T04:00:02Z"}`, state.workItemVersion)
			return
		}
		if request.Method == http.MethodDelete {
			var body json.RawMessage
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			state.lastBody = string(body)
			state.bodies = append(state.bodies, string(body))
			state.lastKey = request.Header.Get("X-Idempotency-Key")
			state.workItemVersion++
			state.workItemDeleted = true
			fmt.Fprintf(w, `{"data":{"operation_id":"op-work-delete","resource_kind":"work_item","resource_id":"work-1","project_id":"project-1","committed_version":%d},"error":null,"request_id":"req-work-delete","server_time":"2026-08-05T04:00:02Z"}`, state.workItemVersion)
			return
		}
	}
	if request.URL.Path == "/api/v1/work-items/work-1/move" && request.Method == http.MethodPost {
		var body json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		var input struct {
			TargetStatus string `json:"target_status"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			t.Fatal(err)
		}
		state.lastBody = string(body)
		state.bodies = append(state.bodies, string(body))
		state.lastKey = request.Header.Get("X-Idempotency-Key")
		state.workItemStatus = input.TargetStatus
		state.workItemVersion++
		fmt.Fprintf(w, `{"data":{"operation_id":"op-work-move","resource_kind":"work_item","resource_id":"work-1","project_id":"project-1","committed_version":%d},"error":null,"request_id":"req-work-move","server_time":"2026-08-05T04:00:02Z"}`, state.workItemVersion)
		return
	}
	if request.URL.Path == "/api/v1/projects/project-1" && request.Method == http.MethodDelete {
		var body struct {
			ExpectedVersion uint64 `json:"expected_version"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.ExpectedVersion != state.version {
			t.Fatalf("delete expected version=%d current=%d", body.ExpectedVersion, state.version)
		}
		state.lastKey = request.Header.Get("X-Idempotency-Key")
		state.projectDeleted = true
		state.version++
		fmt.Fprintf(w, `{"data":{"operation_id":"op-project-delete","resource_kind":"project","resource_id":"project-1","project_id":"project-1","committed_version":%d},"error":null,"request_id":"req-project-delete","server_time":"2026-08-05T04:00:02Z"}`, state.version)
		return
	}
	if request.URL.Path == "/api/v1/projects/join" && request.Method == http.MethodPost {
		var body struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.ProjectID != "project-1" {
			t.Fatalf("join project id = %q", body.ProjectID)
		}
		fmt.Fprint(w, `{"data":{"project_id":"project-1","user_id":"usr_project","role":"member","status":"active","display_name":"Project User","version":1,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"},"error":null,"request_id":"req-join","server_time":"2026-08-05T04:00:02Z"}`)
		return
	}
	if request.Method == http.MethodPost || request.Method == http.MethodPatch {
		var body json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		state.lastBody = string(body)
		state.bodies = append(state.bodies, string(body))
		state.lastKey = request.Header.Get("X-Idempotency-Key")
		if state.networkMutation {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"data":null,"error":{"code":"NETWORK_UNAVAILABLE","message":"Network unavailable.","details":null,"retryable":true},"request_id":"req-network","server_time":"2026-08-05T04:00:02Z"}`)
			return
		}
		if state.conflictMutation {
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"data":null,"error":{"code":"CONFLICT","message":"Conflict.","details":{"reason":"version_mismatch"},"retryable":false},"request_id":"req-conflict","server_time":"2026-08-05T04:00:02Z"}`)
			return
		}
		if state.timeoutMutation {
			time.Sleep(100 * time.Millisecond)
			return
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			t.Fatal(err)
		}
		if raw := fields["name"]; raw != nil {
			_ = json.Unmarshal(raw, &state.name)
		}
		if raw := fields["status"]; raw != nil {
			_ = json.Unmarshal(raw, &state.status)
		}
		if raw := fields["human_description"]; raw != nil {
			_ = json.Unmarshal(raw, &state.instructionHuman)
		}
		if raw := fields["agent_instruction"]; raw != nil {
			_ = json.Unmarshal(raw, &state.instructionAgent)
		}
		if request.Method == http.MethodPatch {
			state.version++
		}
		fmt.Fprintf(w, `{"data":{"operation_id":"op-%d","resource_kind":"project","resource_id":"project-1","project_id":"project-1","committed_version":%d},"error":null,"request_id":"req-command","server_time":"2026-08-05T04:00:02Z"}`, state.version, state.version)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects" {
		if request.URL.Query().Get("ownership") != "owned" {
			t.Fatalf("desktop owned project query = %v", request.URL.Query())
		}
		response := state.ownedListResponse
		if response == "" {
			response = `{"data":{"items":[],"next_cursor":null},"error":null,"request_id":"req-list","server_time":"2026-08-05T04:00:02Z"}`
		}
		fmt.Fprint(w, response)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects/joined" {
		if request.URL.Query().Has("ownership") {
			t.Fatalf("desktop joined project query unexpectedly set ownership: %v", request.URL.Query())
		}
		response := state.joinedListResponse
		if response == "" {
			response = `{"data":{"items":[{"project_id":"project-owned","owner_user_id":"usr_project","name":"Owned duplicate","description":null,"status":"active","version":1,"created_at":"2026-08-05T03:00:00Z","updated_at":"2026-08-05T03:00:02Z"},{"project_id":"project-shared","owner_user_id":"usr_owner","name":"Shared","description":null,"status":"active","version":2,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"}],"next_cursor":null},"error":null,"request_id":"req-list-joined","server_time":"2026-08-05T04:00:02Z"}`
		}
		fmt.Fprint(w, response)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects/project-1" {
		state.lastReadbackPath = request.URL.Path
		fmt.Fprintf(w, `{"data":{"project_id":"project-1","owner_user_id":%q,"name":%q,"description":null,"status":%q,"version":%d,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:02Z"},"error":null,"request_id":"req-get","server_time":"2026-08-05T04:00:03Z"}`, state.ownerUserID, state.name, state.status, state.version)
		return
	}
	http.NotFound(w, request)
}

func (state *workBuddyProjectAppState) allBodies() string {
	state.mu.Lock()
	defer state.mu.Unlock()
	return strings.Join(state.bodies, "\n")
}

func assertWorkBuddyProjectJournalEmpty(t *testing.T, state *workBuddyProjectAppState) {
	t.Helper()
	pending, err := state.services.Journal.Pending(context.Background(), state.scope)
	if err != nil || len(pending) != 0 {
		t.Fatalf("journal pending = %#v, err=%v", pending, err)
	}
}
