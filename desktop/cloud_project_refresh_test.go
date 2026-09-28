package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/project"
)

func TestCloudProjectRefreshAllowsAnotherSessionAfterSuccessfulCreation(t *testing.T) {
	var block atomic.Bool
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	client := &http.Client{Transport: rtFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/api/v1/me" && block.Load() {
			started <- struct{}{}
			select {
			case <-release:
			case <-request.Context().Done():
				return nil, request.Context().Err()
			}
		}
		return http.DefaultTransport.RoundTrip(request)
	})}
	app, state := newWorkBuddyProjectApp(t, client)
	workspace := managedWorkBuddyProjectWorkspace(t, state, "project-1")
	controller, firstPath := installWorkBuddyProjectSessionTab(t, app, workspace)
	t.Cleanup(controller.Close)
	input := NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"}
	first, err := app.NewCloudProjectSession(input)
	if err != nil || first.Reused || first.SessionPath != firstPath {
		t.Fatalf("initial project session: %+v, %v", first, err)
	}
	path := filepath.Join(controller.SessionDir(), "second-project-session.jsonl")
	controller.SetSessionPath(path)
	controller.Resume(agent.NewSession("system"), path)
	app.tabRuntimes["project"].SessionPath = path
	block.Store(true)
	done := make(chan error, 1)
	go func() {
		_, err := app.cloudBootstrapForActor("cloud.refreshAssets", state.runtime, state.services.Actor)
		done <- err
	}()
	defer func() {
		close(release)
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("refresh: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("refresh did not finish")
		}
		assertCloudProjectRefreshServices(t, app, state)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not reach current-user request")
	}
	assertCloudProjectRefreshServices(t, app, state)
	// Opening another project conversation reloads its detail and instruction.
	if _, err := app.GetCloudProject(input.ProjectID); err != nil {
		t.Fatalf("reopen project during refresh: %v", err)
	}
	if _, err := app.GetCloudProjectInstruction(input.ProjectID); err != nil {
		t.Fatalf("reload project instruction during refresh: %v", err)
	}
	view, err := app.NewCloudProjectSession(input)
	if err != nil {
		t.Fatalf("new session during refresh: %v", err)
	}
	if view.Reused || view.SessionPath != path || view.SessionPath == first.SessionPath {
		t.Fatalf("second creation reused the previous conversation: %+v", view)
	}
	meta, found, err := agent.LoadBranchMeta(path)
	if err != nil || !found || meta.ProjectContext == nil || meta.ProjectContext.ProjectID != view.ProjectID {
		t.Fatalf("session was not committed to its original project: found=%v err=%v", found, err)
	}
	firstMeta, found, err := agent.LoadBranchMeta(firstPath)
	if err != nil || !found || firstMeta.ProjectContext == nil || firstMeta.ProjectContext.ProjectID != first.ProjectID {
		t.Fatalf("previous project conversation was lost: found=%v err=%v", found, err)
	}
}

func TestCloudProjectRefreshDoesNotCancelSessionReads(t *testing.T) {
	for _, stage := range []string{"project", "instruction"} {
		t.Run(stage, func(t *testing.T) {
			var app *App
			var refreshed atomic.Bool
			client := &http.Client{Transport: rtFunc(func(request *http.Request) (*http.Response, error) {
				target := "/api/v1/projects/project-1"
				if stage == "instruction" {
					target += "/instructions"
				}
				response, err := http.DefaultTransport.RoundTrip(request)
				if err == nil && app != nil && request.URL.Path == target && refreshed.CompareAndSwap(false, true) {
					if _, refreshErr := app.cloudBootstrapForActor("cloud.refreshAssets", app.cloudRuntime, app.cloudActorContext()); refreshErr != nil {
						t.Errorf("refresh between session reads: %v", refreshErr)
					}
					if request.Context().Err() != nil {
						t.Errorf("successful refresh canceled the %s read", stage)
					}
				}
				return response, err
			})}
			fixtureApp, state := newWorkBuddyProjectApp(t, client)
			app = fixtureApp
			workspace := t.TempDir()
			if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
				t.Fatal(err)
			}
			controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
			t.Cleanup(controller.Close)
			if _, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"}); err != nil {
				t.Fatalf("new session overlapping refresh publication: %v", err)
			}
			if !refreshed.Load() {
				t.Fatal("refresh did not overlap session creation")
			}
			assertCloudProjectRefreshServices(t, app, state)
			meta, found, err := agent.LoadBranchMeta(path)
			if err != nil || !found || meta.ProjectContext == nil || meta.ProjectContext.BindingScope.BindingGeneration != state.scope.Binding.BindingGeneration {
				t.Fatalf("session lost its original binding generation: found=%v err=%v", found, err)
			}
		})
	}
}

func assertCloudProjectRefreshServices(t *testing.T, app *App, state *workBuddyProjectAppState) {
	t.Helper()
	services, err := app.projectServicesForOperation()
	if err != nil || services != state.services {
		t.Errorf("refresh replaced available services: same=%v err=%v", services == state.services, err)
	}
	if err := state.services.lifecycle.Err(); err != nil {
		t.Errorf("refresh canceled project lifecycle: %v", err)
	}
	if got := state.runtime.projectReadinessSnapshot(); got.Phase != cloudProjectReadinessAvailable {
		t.Errorf("refresh readiness = %s, want available", got.Phase)
	}
}

func TestCloudProjectRefreshInvalidatesRejectedServices(t *testing.T) {
	for _, scenario := range []string{"disabled", "missing capability", "malformed", "instructions revoked", "network", "auth rejected"} {
		t.Run(scenario, func(t *testing.T) {
			var reject atomic.Bool
			client := &http.Client{Transport: rtFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/api/v1/me" || !reject.Load() {
					return http.DefaultTransport.RoundTrip(request)
				}
				if scenario == "network" {
					return nil, errors.New("synthetic transport failure")
				}
				response, err := http.DefaultTransport.RoundTrip(request)
				if err != nil {
					return response, err
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					return nil, err
				}
				var envelope map[string]any
				if err := json.Unmarshal(body, &envelope); err != nil {
					return nil, err
				}
				data := envelope["data"].(map[string]any)
				capability := data["capabilities"].(map[string]any)["projects_v1"].(map[string]any)
				switch scenario {
				case "disabled":
					capability["enabled"] = false
				case "missing capability":
					delete(data["capabilities"].(map[string]any), "projects_v1")
				case "malformed":
					capability["enabled"] = "invalid"
				case "instructions revoked":
					capability["instructions"] = false
				case "auth rejected":
					response.StatusCode = http.StatusUnauthorized
					envelope["data"] = nil
					envelope["error"] = map[string]any{"code": "AUTH_REQUIRED", "message": "authentication is required"}
				}
				body, err = json.Marshal(envelope)
				if err != nil {
					return nil, err
				}
				response.Body = io.NopCloser(strings.NewReader(string(body)))
				response.ContentLength = int64(len(body))
				return response, nil
			})}
			app, state := newWorkBuddyProjectApp(t, client)
			reject.Store(true)
			view, err := app.cloudBootstrapForActor("cloud.refreshAssets", state.runtime, state.services.Actor)
			if scenario == "network" || scenario == "auth rejected" || scenario == "malformed" {
				if err == nil && view.OperationError == nil {
					t.Fatal("revalidation failure was hidden")
				}
			}
			if scenario == "auth rejected" && (view.OperationError == nil || view.OperationError.Code != cloud.ErrorAuthRequired) {
				t.Fatalf("authentication rejection changed classification: %+v, %v", view.OperationError, err)
			}
			if _, err := app.projectServicesForInstructionOperation(); err == nil {
				t.Fatal("rejected services still allow instruction operations")
			}
			if state.services.lifecycle.Err() == nil {
				t.Fatal("rejected services were not canceled")
			}
			if got := state.runtime.projectReadinessSnapshot(); got.Refreshing || got.Phase == cloudProjectReadinessPending {
				t.Fatalf("refresh did not settle: %+v", got)
			}
		})
	}
}

func TestCloudProjectRefreshRejectsInvalidBundles(t *testing.T) {
	for _, scenario := range []string{"canceled", "partial", "unavailable", "actor", "profile", "generation", "environment"} {
		t.Run(scenario, func(t *testing.T) {
			app, state := newWorkBuddyProjectApp(t, nil)
			<-state.services.commandReplayDone
			r := state.runtime
			actor := state.services.Actor
			scope := state.scope
			switch scenario {
			case "canceled":
				state.services.cancel()
			case "partial":
				state.services.Gateway = nil
			case "unavailable":
				r.projectCapability = project.CapabilityUnavailable
			case "actor":
				actor.ActorID = "another-actor"
			case "profile":
				scope.Binding.ProfileID = "another-profile"
			case "generation":
				scope.Binding.BindingGeneration++
			case "environment":
				scope.Environment = desktopTestCloudEnvironment("http://127.0.0.1:1")
			}
			var err error
			scope, err = project.NewScope(scope.Environment, scope.Binding)
			if err != nil {
				t.Fatal(err)
			}
			r.beginProjectBootstrapForScope(actor, scope)
			if _, err := app.projectServicesForOperation(); err == nil {
				t.Fatal("invalid bundle remained available")
			}
			if state.services.lifecycle.Err() == nil {
				t.Fatal("invalid bundle lifecycle remained live")
			}
		})
	}
}

func TestCloudProjectRefreshRejectsStaleCompletion(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	r := state.runtime
	actor := state.services.Actor
	staleEpoch := r.beginProjectRefresh(context.Background(), actor)
	if _, ok := r.captureProjectBootstrapScope(actor); ok {
		t.Fatal("unfinished refresh was reusable as a completed bootstrap")
	}
	r.beginProjectBootstrap(actor)
	if _, err := app.projectServicesForOperation(); err == nil || state.services.lifecycle.Err() == nil {
		t.Fatal("hard invalidation did not immediately cancel retained services")
	}
	if _, err := r.remoteAccount.CurrentUser(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	current, err := app.projectServicesForOperation()
	if err != nil {
		t.Fatal(err)
	}
	features := r.projectFeatures
	if r.reuseProjectServices(staleEpoch, actor, state.scope, current.StateRoot, features) {
		t.Fatal("stale refresh reused current services")
	}
	if r.publishProjectServices(staleEpoch, actor, cloud.ProfileBinding{}, state.services, features) {
		t.Fatal("stale refresh resurrected canceled services")
	}
	r.finishProjectBootstrap(staleEpoch, actor, CloudBootstrapView{}, errors.New("old refresh failed"))
	if got, err := app.projectServicesForOperation(); err != nil || got != current || current.lifecycle.Err() != nil {
		t.Fatalf("stale completion affected current services: same=%v err=%v", got == current, err)
	}
}
