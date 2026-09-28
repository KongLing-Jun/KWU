package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/cloud"
	"kwu/internal/project"
	"kwu/internal/userctx"
)

func TestCloudProjectReadinessPublishesAvailableOnlyWithCompleteServices(t *testing.T) {
	runtimeService := &cloudRuntime{available: true}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "initializing")
	actor, binding := cloudProjectReadinessIdentity()
	epoch := runtimeService.beginProjectBootstrap(actor)

	if runtimeService.publishProjectServices(epoch, actor, binding, &cloudProjectServices{}, project.CapabilityFeatures{}) {
		t.Fatal("incomplete project services published available readiness")
	}
	if got := runtimeService.projectReadinessSnapshot(); got.Phase != cloudProjectReadinessPending {
		t.Fatalf("readiness after incomplete publish = %+v, want pending", got)
	}

	services := completeCloudProjectServicesForReadinessTest()
	if !runtimeService.publishProjectServices(epoch, actor, binding, services, project.CapabilityFeatures{WorkItems: true}) {
		t.Fatal("complete project services did not publish")
	}
	got := runtimeService.projectReadinessSnapshot()
	if got.Phase != cloudProjectReadinessAvailable || got.ActorID != actor.ActorID || got.ProfileID != actor.ProfileID ||
		got.CloudUserID != binding.CloudUserID || got.BindingGeneration != binding.BindingGeneration {
		t.Fatalf("available readiness = %+v, want exact actor and binding", got)
	}
}

func TestCloudProjectReadinessDisabledCapabilityIsTerminalUnavailable(t *testing.T) {
	runtimeService := &cloudRuntime{available: true}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "initializing")
	actor, binding := cloudProjectReadinessIdentity()
	epoch := runtimeService.beginProjectBootstrap(actor)
	state := cloud.AccountState{Binding: binding, Capabilities: cloud.CurrentUserCapabilities{
		ProjectsV1: json.RawMessage(`{"enabled":false}`),
	}}

	if err := runtimeService.observeProjectBootstrapAtEpoch(context.Background(), actor, state, epoch); err != nil {
		t.Fatalf("observe disabled capability: %v", err)
	}
	if got := runtimeService.projectReadinessSnapshot(); got.Phase != cloudProjectReadinessUnavailable {
		t.Fatalf("disabled readiness = %+v, want unavailable", got)
	}
}

func TestCloudProjectReadinessProtocolFailureSettlesFailed(t *testing.T) {
	runtimeService := &cloudRuntime{available: true}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "initializing")
	actor, binding := cloudProjectReadinessIdentity()
	epoch := runtimeService.beginProjectBootstrap(actor)
	state := cloud.AccountState{Binding: binding, Capabilities: cloud.CurrentUserCapabilities{
		ProjectsV1: json.RawMessage(`{"enabled":"yes"}`),
	}}

	if err := runtimeService.observeProjectBootstrapAtEpoch(context.Background(), actor, state, epoch); err == nil {
		t.Fatal("malformed capability did not fail")
	}
	if got := runtimeService.projectReadinessSnapshot(); got.Phase != cloudProjectReadinessFailed {
		t.Fatalf("failed readiness = %+v, want failed", got)
	}
}

func TestCloudProjectReadinessRejectsStaleEpoch(t *testing.T) {
	runtimeService := &cloudRuntime{available: true}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "initializing")
	actor, binding := cloudProjectReadinessIdentity()
	staleEpoch := runtimeService.beginProjectBootstrap(actor)
	currentEpoch := runtimeService.beginProjectBootstrap(actor)

	if runtimeService.publishProjectServices(staleEpoch, actor, binding, completeCloudProjectServicesForReadinessTest(), project.CapabilityFeatures{}) {
		t.Fatal("stale epoch published project services")
	}
	if runtimeService.settleProjectBootstrap(staleEpoch, cloudProjectReadinessUnavailable, actor, binding, "stale") {
		t.Fatal("stale epoch settled readiness")
	}
	got := runtimeService.projectReadinessSnapshot()
	if got.Epoch != currentEpoch || got.Phase != cloudProjectReadinessPending {
		t.Fatalf("readiness after stale results = %+v, want current pending epoch %d", got, currentEpoch)
	}
}

func TestCloudProjectReadinessEpochInvalidationClearsServicesBeforeReturn(t *testing.T) {
	runtimeService := &cloudRuntime{available: true}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "initializing")
	actor, binding := cloudProjectReadinessIdentity()
	epoch := runtimeService.beginProjectBootstrap(actor)
	if !runtimeService.publishProjectServices(epoch, actor, binding, completeCloudProjectServicesForReadinessTest(), project.CapabilityFeatures{}) {
		t.Fatal("initial project services did not publish")
	}
	app := &App{cloudRuntime: runtimeService}
	if _, err := app.projectServicesForOperation(); err != nil {
		t.Fatalf("available project services: %v", err)
	}

	nextEpoch := runtimeService.beginProjectBootstrap(actor)
	if got := runtimeService.projectReadinessSnapshot(); got.Epoch != nextEpoch || got.Phase != cloudProjectReadinessPending {
		t.Fatalf("readiness after invalidation = %+v", got)
	}
	if _, err := app.projectServicesForOperation(); err == nil {
		t.Fatal("project operation obtained stale services after new epoch became visible")
	}
}

func TestSharedCloudBootstrapWaiterReusesLeaderResult(t *testing.T) {
	runtimeService := &cloudRuntime{available: true}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "initializing")
	actor := userctx.Default("").Normalize()
	epoch := runtimeService.beginProjectBootstrap(actor)
	runtimeService.settleProjectBootstrap(epoch, cloudProjectReadinessUnavailable, actor, cloud.ProfileBinding{}, "unavailable")
	scope, ok := runtimeService.captureProjectBootstrapScope(actor)
	if !ok {
		t.Fatal("capture waiter bootstrap scope")
	}
	flight := &cloudProjectBootstrapFlight{
		actorKey: actor.ActorID + "\x00" + actor.ProfileID,
		done:     make(chan struct{}),
		scope:    scope,
	}
	runtimeService.projectBootstrap.Store(flight)
	app := &App{cloudRuntime: runtimeService, ctx: context.Background()}
	type bootstrapResult struct {
		view CloudBootstrapView
		err  error
	}
	result := make(chan bootstrapResult, 1)
	go func() {
		view, _, err := app.sharedCloudBootstrap("cloud.bootstrap")
		result <- bootstrapResult{view: view, err: err}
	}()

	select {
	case <-result:
		t.Fatal("shared bootstrap waiter returned before leader completed")
	case <-time.After(20 * time.Millisecond):
	}
	flight.view = CloudBootstrapView{Available: true, Account: CloudAccountView{ProfileID: actor.ProfileID}}
	close(flight.done)
	select {
	case got := <-result:
		if got.err != nil || !got.view.Available || got.view.Account.ProfileID != actor.ProfileID {
			t.Fatalf("shared result = %+v, %v", got.view, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("shared bootstrap waiter did not receive leader result")
	}
}

func TestStartupBootstrapHandoffIsConsumedOnce(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	app := NewApp()
	t.Cleanup(app.cloudRuntime.close)

	want, actor, err := app.runSharedCloudBootstrap("cloud.bootstrap", true)
	if err != nil {
		t.Fatalf("startup bootstrap: %v", err)
	}
	startup := app.cloudRuntime.projectReadinessSnapshot()
	if app.cloudRuntime.projectBootstrap.Load() == nil {
		t.Fatal("startup bootstrap did not retain a one-shot handoff")
	}
	got, err := app.CloudBootstrap()
	if err != nil || got.Account.ProfileID != want.Account.ProfileID {
		t.Fatalf("consume startup handoff = %+v, %v", got, err)
	}
	if current := app.cloudRuntime.projectBootstrap.Load(); current != nil {
		t.Fatal("startup handoff remained after foreground consumption")
	}
	if consumed := app.cloudRuntime.projectReadinessSnapshot(); consumed.Epoch != startup.Epoch {
		t.Fatalf("handoff consumption advanced epoch from %d to %d", startup.Epoch, consumed.Epoch)
	}
	if _, err := app.CloudBootstrap(); err != nil {
		t.Fatalf("bootstrap after consumed handoff: %v", err)
	}
	refreshed := app.cloudRuntime.projectReadinessSnapshot()
	if refreshed.Epoch <= startup.Epoch || refreshed.ActorID != actor.ActorID || refreshed.ProfileID != actor.ProfileID {
		t.Fatalf("bootstrap after consumed handoff = %+v, want new epoch for %+v", refreshed, actor)
	}
}

func TestStartupBootstrapHandoffCannotCrossActorScope(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	app := NewApp()
	t.Cleanup(app.cloudRuntime.close)
	actorA := userctx.ActorContext{ActorID: "local", ProfileID: "actor-a"}.Normalize()
	actorB := userctx.ActorContext{ActorID: "local", ProfileID: "actor-b"}.Normalize()
	app.setCurrentActorContext(actorA)
	if _, _, err := app.runSharedCloudBootstrap("cloud.bootstrap", true); err != nil {
		t.Fatalf("actor A startup bootstrap: %v", err)
	}
	actorAEpoch := app.cloudRuntime.projectReadinessSnapshot().Epoch
	app.setCurrentActorContext(actorB)
	view, err := app.CloudBootstrap()
	if err != nil {
		t.Fatalf("actor B bootstrap: %v", err)
	}
	readiness := app.cloudRuntime.projectReadinessSnapshot()
	if view.Account.ProfileID != actorB.ProfileID || readiness.ActorID != actorB.ActorID || readiness.ProfileID != actorB.ProfileID || readiness.Epoch <= actorAEpoch {
		t.Fatalf("actor B received stale actor A bootstrap: view=%+v readiness=%+v", view.Account, readiness)
	}
}

func TestStartupBootstrapHandoffCannotCrossBindingGeneration(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	app := NewApp()
	t.Cleanup(app.cloudRuntime.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "shared-profile"}.Normalize()
	app.setCurrentActorContext(actor)
	epoch := app.cloudRuntime.beginProjectBootstrap(actor)
	currentBinding := cloud.ProfileBinding{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: "user-b",
		Status: cloud.BindingBound, BindingGeneration: 2,
	}
	app.cloudRuntime.settleProjectBootstrap(epoch, cloudProjectReadinessUnavailable, actor, currentBinding, "test binding")
	stale := &cloudProjectBootstrapFlight{
		actorKey: actor.ActorID + "\x00" + actor.ProfileID, retainResult: true, done: make(chan struct{}),
		scope: cloudProjectBootstrapScope{
			ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: "user-a",
			BindingGeneration: 1, Epoch: epoch,
		},
		view: CloudBootstrapView{Available: true, Account: CloudAccountView{
			Status: cloud.AccountAuthenticated, ProfileID: actor.ProfileID, CloudUserID: "user-a",
			BindingStatus: cloud.BindingBound, BindingGeneration: 1,
		}},
	}
	close(stale.done)
	app.cloudRuntime.projectBootstrap.Store(stale)

	view, err := app.CloudBootstrap()
	if err != nil {
		t.Fatalf("bootstrap after stale binding handoff: %v", err)
	}
	readiness := app.cloudRuntime.projectReadinessSnapshot()
	if view.Account.CloudUserID == "user-a" || readiness.Epoch <= epoch {
		t.Fatalf("stale binding handoff was returned: view=%+v readiness=%+v", view.Account, readiness)
	}
}

func TestFailedStartupBootstrapHandoffRetriesFresh(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	app := NewApp()
	t.Cleanup(app.cloudRuntime.close)
	actor := userctx.Default("").Normalize()
	app.setCurrentActorContext(actor)
	epoch := app.cloudRuntime.beginProjectBootstrap(actor)
	app.cloudRuntime.settleProjectBootstrap(epoch, cloudProjectReadinessFailed, actor, cloud.ProfileBinding{}, "transient startup failure")
	scope, ok := app.cloudRuntime.captureProjectBootstrapScope(actor)
	if !ok {
		t.Fatal("capture failed startup scope")
	}
	transientErr := errors.New("transient startup failure")
	failed := &cloudProjectBootstrapFlight{
		actorKey: actor.ActorID + "\x00" + actor.ProfileID, retainResult: true, done: make(chan struct{}),
		scope: scope, err: transientErr,
	}
	close(failed.done)
	app.cloudRuntime.projectBootstrap.Store(failed)

	view, err := app.CloudBootstrap()
	if errors.Is(err, transientErr) {
		t.Fatalf("foreground bootstrap reused failed startup handoff: %+v, %v", view, err)
	}
	if err != nil {
		t.Fatalf("fresh foreground bootstrap: %v", err)
	}
	readiness := app.cloudRuntime.projectReadinessSnapshot()
	if readiness.Epoch <= epoch || readiness.Phase == cloudProjectReadinessFailed {
		t.Fatalf("failed startup did not retry fresh: %+v", readiness)
	}
}

func TestCloudBootstrapSerializesActorScopedQuotaPublication(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	app := NewApp()
	t.Cleanup(app.cloudRuntime.close)
	actorA := userctx.ActorContext{ActorID: "local", ProfileID: "actor-a"}.Normalize()
	actorB := userctx.ActorContext{ActorID: "local", ProfileID: "actor-b"}.Normalize()
	app.setCurrentActorContext(actorA)
	atPublish := make(chan struct{})
	releasePublish := make(chan struct{})
	app.cloudBootstrapBeforePublish = func() {
		close(atPublish)
		<-releasePublish
	}
	bootstrapDone := make(chan error, 1)
	go func() {
		_, err := app.CloudBootstrap()
		bootstrapDone <- err
	}()
	<-atPublish
	if app.cloudRuntime.operationMu.TryLock() {
		app.cloudRuntime.operationMu.Unlock()
		t.Fatal("bootstrap released operation ownership before actor-scoped publication")
	}

	transitionStarted := make(chan struct{})
	transitionAcquired := make(chan struct{})
	transitionDone := make(chan struct{})
	go func() {
		close(transitionStarted)
		if err := app.cloudRuntime.beginOperation(); err != nil {
			close(transitionDone)
			return
		}
		close(transitionAcquired)
		app.setCurrentActorContext(actorB)
		epoch := app.cloudRuntime.beginProjectBootstrap(actorB)
		app.cloudRuntime.settleProjectBootstrap(epoch, cloudProjectReadinessUnavailable, actorB, cloud.ProfileBinding{}, "actor switched")
		app.clearAutomationTaskQuota()
		app.cloudRuntime.endOperation()
		close(transitionDone)
	}()
	<-transitionStarted
	select {
	case <-transitionAcquired:
		t.Fatal("actor transition acquired operation ownership during bootstrap publication")
	default:
	}
	close(releasePublish)
	if err := <-bootstrapDone; err != nil {
		t.Fatalf("actor A bootstrap: %v", err)
	}
	<-transitionDone
	entitlement, err := app.automationTaskQuotaResolver().ResolveTaskQuota(context.Background())
	if err != nil {
		t.Fatalf("resolve quota after actor transition: %v", err)
	}
	if entitlement.State != automation.TaskQuotaStateLoading {
		t.Fatalf("stale actor quota survived transition: %+v", entitlement)
	}
}

func TestFailedCloudLoginDoesNotLeaveProjectReadinessPending(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	app := NewApp()
	t.Cleanup(app.cloudRuntime.close)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatalf("initial CloudLogin: %v", err)
	}
	_, _ = app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "wrong-password"})
	if got := app.cloudRuntime.projectReadinessSnapshot(); got.Phase == cloudProjectReadinessPending {
		t.Fatalf("readiness after failed login = %+v, want terminal state", got)
	}
}

func TestBackendProjectBootstrapAdvancesWithoutFrontendCall(t *testing.T) {
	runtimeService, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimeService.close)
	app := &App{cloudRuntime: runtimeService, ctx: context.Background()}
	app.startCloudProjectBootstrap()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := runtimeService.projectReadinessSnapshot(); got.Phase != cloudProjectReadinessPending {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("backend bootstrap left readiness pending: %+v", runtimeService.projectReadinessSnapshot())
}

func TestAutomationTaskPreflightDefersOnlyPendingCloudProjects(t *testing.T) {
	runtimeService := &cloudRuntime{available: true}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "initializing")
	app := &App{cloudRuntime: runtimeService}
	legacyManagedRoot := filepath.Join(
		app.currentProfilePathResolver("").MemoryUserDir(),
		cloudProjectWorkspacesDir,
		strings.Repeat("a", 64),
		"project-"+strings.Repeat("b", 64),
	)
	tests := []struct {
		name string
		task automation.Task
		want automation.PreflightDisposition
	}{
		{name: "cloud project", task: automation.Task{ProjectID: "project-1"}, want: automation.PreflightDefer},
		{name: "legacy managed project", task: automation.Task{Scope: "project", WorkspaceRoot: legacyManagedRoot}, want: automation.PreflightDefer},
		{name: "local project", task: automation.Task{ProjectID: "project-1", WorkspaceRoot: t.TempDir()}, want: automation.PreflightReady},
		{name: "global", task: automation.Task{}, want: automation.PreflightReady},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := app.automationTaskPreflight(tt.task); got.Disposition != tt.want {
				t.Fatalf("preflight = %+v, want disposition %d", got, tt.want)
			}
		})
	}

	epoch := runtimeService.projectReadinessSnapshot().Epoch
	actor, binding := cloudProjectReadinessIdentity()
	runtimeService.settleProjectBootstrap(epoch, cloudProjectReadinessFailed, actor, binding, "offline")
	if got := app.automationTaskPreflight(automation.Task{ProjectID: "project-1"}); got.Disposition != automation.PreflightReady {
		t.Fatalf("terminal failed preflight = %+v, want runner fail-closed path", got)
	}
}

func cloudProjectReadinessIdentity() (userctx.ActorContext, cloud.ProfileBinding) {
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	return actor, cloud.ProfileBinding{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: "usr-1",
		Status: cloud.BindingBound, BindingGeneration: 3,
	}
}

func completeCloudProjectServicesForReadinessTest() *cloudProjectServices {
	return &cloudProjectServices{
		Gateway:  &project.ProjectGateway{},
		Cache:    &project.ProjectCache{},
		Bindings: &project.LocalWorkspaceBindingRepository{},
		Journal:  &project.ProjectCommandJournal{},
	}
}
