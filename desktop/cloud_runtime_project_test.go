package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cloud "kwu/internal/cloud"
	project "kwu/internal/project"
	"kwu/internal/userctx"
)

func TestWorkBuddyProjectCloudRuntimeCancelsPriorScopeBeforePublishingReplacement(t *testing.T) {
	priorLifecycle, cancelPriorLifecycle := context.WithCancel(context.Background())
	cancelEntered := make(chan struct{})
	allowCancel := make(chan struct{})
	prior := &cloudProjectServices{
		lifecycle: priorLifecycle,
		cancel: func() {
			close(cancelEntered)
			<-allowCancel
			cancelPriorLifecycle()
		},
	}
	replacement := &cloudProjectServices{}
	runtimeService := &cloudRuntime{
		projects:          prior,
		projectCapability: project.CapabilityAvailable,
		projectFeatures:   project.CapabilityFeatures{Instructions: true},
	}
	replaced := make(chan struct{})
	go func() {
		runtimeService.replaceProjectServices(
			project.CapabilityAvailable,
			replacement,
			project.CapabilityFeatures{WorkItems: true},
		)
		close(replaced)
	}()
	<-cancelEntered

	if runtimeService.projectMu.TryRLock() {
		runtimeService.projectMu.RUnlock()
		t.Fatal("replacement project scope became observable before prior scope cancellation completed")
	}

	close(allowCancel)
	<-replaced
	select {
	case <-priorLifecycle.Done():
	default:
		t.Fatal("prior project lifecycle remained active after replacement")
	}
	runtimeService.projectMu.RLock()
	defer runtimeService.projectMu.RUnlock()
	if runtimeService.projects != replacement || !runtimeService.projectFeatures.WorkItems || runtimeService.projectFeatures.Instructions {
		t.Fatalf("published project snapshot = services %#v features %#v", runtimeService.projects, runtimeService.projectFeatures)
	}
}

func TestWorkBuddyProjectCloudRuntimeCancelsPriorScopeOnIdentityChange(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	paths := desktopCloudTestProfilePathProvider(t.TempDir())
	environment := desktopTestCloudEnvironment(server.URL)
	runtimeService, err := newRealCloudRuntime(server.URL, paths, server.Client(), cloud.DeviceIdentity{DeviceID: "device-project-cancel", ClientVersion: "dev"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimeService.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindingA, err := runtimeService.bindings.ConfirmBinding(context.Background(), actor, "usr_project_a")
	if err != nil {
		t.Fatal(err)
	}
	capability := json.RawMessage(`{"enabled":true,"contract_version":"` + project.ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + project.CanonicalContractSHA256() + `","instructions":true,"work_items":true}`)
	if err := runtimeService.observeProjectBootstrap(context.Background(), actor, cloud.AccountState{Binding: bindingA, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: capability}}); err != nil {
		t.Fatal(err)
	}
	runtimeService.projectMu.RLock()
	servicesA := runtimeService.projects
	runtimeService.projectMu.RUnlock()
	operationA, cancelA := servicesA.operationContext(context.Background())
	defer cancelA()

	bindingB, err := runtimeService.bindings.Rebind(context.Background(), actor, "usr_project_b")
	if err != nil {
		t.Fatal(err)
	}
	if err := runtimeService.observeProjectBootstrap(context.Background(), actor, cloud.AccountState{Binding: bindingB, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: capability}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-operationA.Done():
	default:
		t.Fatal("prior project-scope operation was not cancelled after account change")
	}
	runtimeService.projectMu.RLock()
	servicesB := runtimeService.projects
	runtimeService.projectMu.RUnlock()
	if servicesB == nil || servicesB == servicesA || servicesB.Scope.Key == servicesA.Scope.Key {
		t.Fatalf("replacement project services = %#v, previous = %#v", servicesB, servicesA)
	}
	operationB, cancelB := servicesB.operationContext(context.Background())
	defer cancelB()
	if err := runtimeService.observeProjectBootstrap(context.Background(), actor, cloud.AccountState{Binding: bindingB, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: json.RawMessage(`{"enabled":false}`)}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-operationB.Done():
	default:
		t.Fatal("current project-scope operation was not cancelled when capability became unavailable")
	}
	runtimeService.projectMu.RLock()
	defer runtimeService.projectMu.RUnlock()
	if runtimeService.projects != nil || runtimeService.projectCapability != project.CapabilityUnavailable {
		t.Fatalf("unavailable capability retained project services: status=%q services=%#v", runtimeService.projectCapability, runtimeService.projects)
	}
}

func TestWorkBuddyProjectCloudRuntimeConfiguresFromAuthenticatedBootstrap(t *testing.T) {
	valid := `{"enabled":true,"contract_version":"` + project.ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + project.CanonicalContractSHA256() + `","instructions":true,"work_items":true}`
	validV2 := `{"enabled":true,"contract_version":"` + project.V2ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + project.CanonicalV2ContractSHA256() + `","slices":{"members":true}}`
	cases := []struct {
		name         string
		capability   string
		v2Capability string
		want         project.CapabilityStatus
	}{
		{name: "absent", want: project.CapabilityUnavailable},
		{name: "disabled", capability: `{"enabled":false}`, want: project.CapabilityUnavailable},
		{name: "supported", capability: valid, want: project.CapabilityAvailable},
		{name: "different version", capability: strings.Replace(valid, project.ContractVersion, "projects-v2", 1), want: project.CapabilityAvailable},
		{name: "different hash", capability: strings.Replace(valid, project.CanonicalContractSHA256(), strings.Repeat("0", 64), 1), want: project.CapabilityAvailable},
		{name: "unknown field", capability: strings.Replace(valid, `"enabled":true`, `"enabled":true,"future_flag":true`, 1), want: project.CapabilityAvailable},
		{name: "v2 different version", capability: valid, v2Capability: strings.Replace(validV2, project.V2ContractVersion, "projects-v3", 1), want: project.CapabilityAvailable},
		{name: "v2 different hash", capability: valid, v2Capability: strings.Replace(validV2, project.CanonicalV2ContractSHA256(), strings.Repeat("0", 64), 1), want: project.CapabilityAvailable},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method == http.MethodGet && request.URL.Path == "/api/v1/projects" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"data":{"items":[],"next_cursor":null},"error":null,"request_id":"req-project-list","server_time":"2026-08-05T04:00:02Z"}`))
					return
				}
				if request.Method != http.MethodGet || request.URL.Path != "/api/v1/me" {
					http.NotFound(w, request)
					return
				}
				capabilities := ""
				if test.capability != "" {
					capabilities = `,"capabilities":{"projects_v1":` + test.capability
					if test.v2Capability != "" {
						capabilities += `,"projects_v2":` + test.v2Capability
					}
					capabilities += `}`
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_project","display_name":"Project User","status":"active","role":"user","created_at":"2026-08-05T04:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0}` + capabilities + `},"error":null,"request_id":"req-project","server_time":"2026-08-05T04:00:01Z"}`))
			}))
			defer server.Close()

			root := t.TempDir()
			paths := desktopCloudTestProfilePathProvider(root)
			environment := desktopTestCloudEnvironment(server.URL)
			runtimeService, err := newRealCloudRuntime(server.URL, paths, server.Client(), cloud.DeviceIdentity{DeviceID: "device-project", ClientVersion: "dev"}, environment)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(runtimeService.close)
			actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
			binding, err := runtimeService.bindings.ConfirmBinding(context.Background(), actor, "usr_project")
			if err != nil {
				t.Fatal(err)
			}
			scope, err := binding.Scope()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			if err := runtimeService.sessions.Save(context.Background(), actor, cloud.Session{
				Scope: scope, State: cloud.SessionAuthenticated, AccessToken: "access-project", RefreshToken: "refresh-project", DeviceID: "device-project",
				AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
			}); err != nil {
				t.Fatal(err)
			}

			state, err := runtimeService.remoteAccount.CurrentUser(context.Background(), actor)
			if err != nil {
				t.Fatal(err)
			}
			if state.Balance.TotalBalance != "1000.0000" || state.Subscription.Plan != "free" {
				t.Fatalf("authoritative account state = %#v", state)
			}
			runtimeService.projectMu.RLock()
			services := runtimeService.projects
			status := runtimeService.projectCapability
			features := runtimeService.projectFeatures
			runtimeService.projectMu.RUnlock()
			if status != test.want {
				t.Fatalf("project capability = %q, want %q", status, test.want)
			}
			if test.want != project.CapabilityAvailable {
				if services != nil {
					t.Fatalf("unavailable capability constructed services: %#v", services)
				}
				return
			}
			if services == nil || services.Gateway == nil || services.Cache == nil || services.Bindings == nil || services.Journal == nil {
				t.Fatalf("supported capability services = %#v", services)
			}
			if _, err := services.Gateway.ListProjects(context.Background(), project.ListProjectsInput{}); err != nil {
				t.Fatalf("project request after bootstrap: %v", err)
			}
			if test.v2Capability != "" && (features.V2Status != project.CapabilityAvailable || !features.Members) {
				t.Fatalf("v2 capability features = %#v", features)
			}
			wantScope, err := project.NewScope(environment, scope)
			if err != nil {
				t.Fatal(err)
			}
			wantRoot := paths(actor).MemoryUserDir()
			if services.Scope.Key != wantScope.Key || services.Environment != environment || services.Actor.Normalize() != actor.Normalize() || services.StateRoot != wantRoot {
				t.Fatalf("project services identity = %#v", services)
			}
		})
	}
}

func TestWorkBuddyProjectCapabilityDataErrorDoesNotPoisonNextBootstrap(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	paths := desktopCloudTestProfilePathProvider(t.TempDir())
	environment := desktopTestCloudEnvironment(server.URL)
	runtimeService, err := newRealCloudRuntime(server.URL, paths, server.Client(), cloud.DeviceIdentity{DeviceID: "device-project-retry", ClientVersion: "dev"}, environment)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimeService.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	binding, err := runtimeService.bindings.ConfirmBinding(context.Background(), actor, "usr_project_retry")
	if err != nil {
		t.Fatal(err)
	}

	bad := cloud.AccountState{Binding: binding, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: json.RawMessage(`{"enabled":"true"}`)}}
	err = runtimeService.observeProjectBootstrap(context.Background(), actor, bad)
	var protocol *cloud.Error
	if !errors.As(err, &protocol) || protocol.Code != cloud.ErrorCode("CLOUD_PROTOCOL_ERROR") {
		t.Fatalf("invalid capability error = %#v", err)
	}
	runtimeService.projectMu.RLock()
	badStatus, badServices := runtimeService.projectCapability, runtimeService.projects
	runtimeService.projectMu.RUnlock()
	if badStatus != project.CapabilityUnavailable || badServices != nil {
		t.Fatalf("invalid capability state = %q %#v", badStatus, badServices)
	}

	goodCapability := json.RawMessage(`{"enabled":true,"contract_version":"future","contract_hash_algorithm":"future","contract_hash":"future","instructions":true,"work_items":true}`)
	good := cloud.AccountState{Binding: binding, Capabilities: cloud.CurrentUserCapabilities{ProjectsV1: goodCapability}}
	if err := runtimeService.observeProjectBootstrap(context.Background(), actor, good); err != nil {
		t.Fatalf("retry bootstrap: %v", err)
	}
	runtimeService.projectMu.RLock()
	goodStatus, goodServices := runtimeService.projectCapability, runtimeService.projects
	runtimeService.projectMu.RUnlock()
	if goodStatus != project.CapabilityAvailable || goodServices == nil || goodServices.Gateway == nil {
		t.Fatalf("recovered capability state = %q %#v", goodStatus, goodServices)
	}
}
