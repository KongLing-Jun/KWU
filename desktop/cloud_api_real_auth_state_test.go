package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cloud "kwu/internal/cloud"
	"kwu/internal/userctx"
)

const (
	cloudBootstrapTestBuild         = "test/source-under-test"
	cloudBootstrapTestVersion       = "test/source-under-test"
	cloudBootstrapTestCommit        = "not-recorded"
	cloudBootstrapProjectExecutorID = "project-executor"
	cloudBootstrapProjectRefreshID  = "project-refresh"
	cloudBootstrapSharedExecutorID  = "shared-executor"
)

func TestCloudBootstrapSharedExecutorHydratesReplacementAndExpiresAfterRejection(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	diagnosticAt := time.Now()
	diagnosticAtUTC := diagnosticAt.UTC().Format(time.RFC3339Nano)
	localTimezoneName, localTimezoneOffsetSeconds := diagnosticAt.Zone()
	testNow := diagnosticAt.UTC()
	replacementAccessExpiry := testNow.Add(time.Hour)
	replacementRefreshExpiry := testNow.Add(48 * time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	firstRefresh := make(chan struct{})
	releaseFirstRefresh := make(chan struct{})
	var refreshCalls atomic.Int32
	var currentUserCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/auth/refresh":
			call := refreshCalls.Add(1)
			switch call {
			case 1:
				close(firstRefresh)
				select {
				case <-releaseFirstRefresh:
				case <-request.Context().Done():
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{"data":{"session":{"access_token":"access-cross-new","access_token_expires_at":%q,"refresh_token":"refresh-cross-new","refresh_token_expires_at":%q}},"error":null,"request_id":"req-cross-refresh-success","server_time":%q}`, replacementAccessExpiry.Format(time.RFC3339Nano), replacementRefreshExpiry.Format(time.RFC3339Nano), testNow.Format(time.RFC3339Nano))
			default:
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"data":null,"error":{"code":"TEST_UNEXPECTED_REFRESH","message":"unexpected refresh attempt"},"request_id":"req-cross-unexpected","server_time":"2026-08-20T15:00:03Z"}`))
			}
		case "/api/v1/me":
			call := currentUserCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			if call == 1 {
				_, _ = fmt.Fprintf(w, `{"data":{"user":{"user_id":"usr_cross_desktop","display_name":"Cross Desktop User"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"plan":"free","status":"free","version":0},"capabilities":{}},"error":null,"request_id":"req-cross-current-success","server_time":%q}`, testNow.Format(time.RFC3339Nano))
			} else {
				_, _ = w.Write([]byte(`{"data":null,"error":{"code":"AUTH_REQUIRED","message":"current session is no longer authorized"},"request_id":"req-cross-current-auth","server_time":"2026-08-20T15:01:00Z"}`))
			}
		case "/api/v1/me/maas-key":
			_, _ = w.Write([]byte(`{"data":{"status":"create_pending","key_name":"usr_cross_desktop","key_id":null},"error":null,"request_id":"req-cross-maas-pending","server_time":"2026-08-20T15:00:04Z"}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	paths := desktopCloudTestProfilePathProvider(t.TempDir())
	runtime, err := newRealCloudRuntimeWithRegistration(
		server.URL,
		paths,
		server.Client(),
		cloud.DeviceIdentity{DeviceID: "device-cross-desktop", ClientVersion: "auth-regression-test"},
		cloudRegistrationDevelopment,
		desktopTestCloudEnvironment(server.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)

	actor := userctx.ActorContext{ActorID: "local", ProfileID: "cross-desktop"}.Normalize()
	binding, err := runtime.bindings.ConfirmBinding(ctx, actor, "usr_cross_desktop")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	old := cloud.Session{
		Scope:                 scope,
		State:                 cloud.SessionAuthenticated,
		AccessToken:           "access-cross-old",
		RefreshToken:          "refresh-cross-old",
		DeviceID:              "device-cross-desktop",
		AccessTokenExpiresAt:  testNow.Add(-time.Minute),
		RefreshTokenExpiresAt: testNow.Add(24 * time.Hour),
	}
	if err := runtime.sessionStore.Save(ctx, old); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: ctx, cloudRuntime: runtime}
	app.setCurrentActorContext(actor)
	sharedResult := make(chan error, 1)
	go func() {
		err := runtime.projectExecutor.Do(ctx, scope, func(context.Context, string) error { return nil })
		sharedResult <- err
	}()
	select {
	case <-firstRefresh:
	case <-ctx.Done():
		t.Fatalf("shared executor did not reach refresh barrier: %v", ctx.Err())
	}

	type bootstrapResult struct {
		view CloudBootstrapView
		err  error
	}
	bootstrapDone := make(chan bootstrapResult, 1)
	bootstrapStarted := make(chan struct{})
	go func() {
		close(bootstrapStarted)
		view, err := app.CloudBootstrap()
		bootstrapDone <- bootstrapResult{view: view, err: err}
	}()
	<-bootstrapStarted
	close(releaseFirstRefresh)

	sharedErr := <-sharedResult
	if sharedErr != nil {
		t.Fatalf("shared project executor winner returned %v", sharedErr)
	}
	first := <-bootstrapDone
	if first.err != nil {
		t.Fatalf("first CloudBootstrap returned %v", first.err)
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("first bootstrap refresh calls = %d, want 1", refreshCalls.Load())
	}
	if first.view.Account.Status != cloud.AccountAuthenticated || first.view.Account.BindingStatus != cloud.BindingBound || first.view.Account.BindingGeneration != binding.BindingGeneration || first.view.Account.DisplayName != "Cross Desktop User" {
		t.Fatalf("first bootstrap account state = %s/%s/generation-%d", first.view.Account.Status, first.view.Account.BindingStatus, first.view.Account.BindingGeneration)
	}
	if first.view.Account.CloudUserID == "" || first.view.OperationError != nil {
		t.Fatalf("first bootstrap identity/error presence = %t/%t", first.view.Account.CloudUserID != "", first.view.OperationError != nil)
	}
	replacement, found, err := runtime.sessionStore.Load(ctx, scope)
	if err != nil || !found {
		t.Fatalf("replacement session load failed: found=%t state=%s", found, replacement.State)
	}
	if replacement.State != cloud.SessionAuthenticated || replacement.AccessToken == "" || replacement.RefreshToken == "" || replacement.AccessTokenExpiresAt == old.AccessTokenExpiresAt || replacement.RefreshTokenExpiresAt == old.RefreshTokenExpiresAt {
		t.Fatalf("replacement session state/presence/expiry = %s/%t/%t/access-changed-%t/refresh-changed-%t", replacement.State, replacement.AccessToken != "", replacement.RefreshToken != "", replacement.AccessTokenExpiresAt != old.AccessTokenExpiresAt, replacement.RefreshTokenExpiresAt != old.RefreshTokenExpiresAt)
	}
	t.Logf("test_build=%s test_version=%s test_commit=%s observed_at_utc=%s local_timezone_name=%s local_timezone_offset_seconds=%d base_environment=localhost_test source=project profile=%s generation=%d session=%s access_present=%t refresh_present=%t access_expiry_before=%s refresh_expiry_before=%s access_expiry_after=%s refresh_expiry_after=%s executor_test_id=%s refresh_attempt_id=%s refresh_attempt_count=%d trigger_type=local_expiry http_status=200 error_code=none request_id=req-cross-refresh-success sameObservedSession=n/a termination_skipped=n/a session_revision_supported=false bootstrap_stage=refresh_winner", cloudBootstrapTestBuild, cloudBootstrapTestVersion, cloudBootstrapTestCommit, diagnosticAtUTC, localTimezoneName, localTimezoneOffsetSeconds, scope.ProfileID, scope.BindingGeneration, replacement.State, replacement.AccessToken != "", replacement.RefreshToken != "", old.AccessTokenExpiresAt.UTC().Format(time.RFC3339), old.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), replacement.AccessTokenExpiresAt.UTC().Format(time.RFC3339), replacement.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), cloudBootstrapProjectExecutorID, cloudBootstrapProjectRefreshID, refreshCalls.Load())
	t.Logf("test_build=%s test_version=%s test_commit=%s observed_at_utc=%s local_timezone_name=%s local_timezone_offset_seconds=%d base_environment=localhost_test source=account profile=%s generation=%d session=%s access_present=%t refresh_present=%t access_expiry_before=%s refresh_expiry_before=%s access_expiry_after=%s refresh_expiry_after=%s executor_test_id=%s refresh_attempt_id=not_applicable refresh_attempt_count=%d trigger_type=local_expiry http_status=200 error_code=none request_id=none sameObservedSession=false termination_skipped=false session_revision_supported=false bootstrap_stage=first bootstrap_account_status=%s binding_status=%s binding_generation=%d cloud_user_id_present=%t display_name_present=%t operation_error_code=none operation_error_request_id=none", cloudBootstrapTestBuild, cloudBootstrapTestVersion, cloudBootstrapTestCommit, diagnosticAtUTC, localTimezoneName, localTimezoneOffsetSeconds, scope.ProfileID, scope.BindingGeneration, replacement.State, replacement.AccessToken != "", replacement.RefreshToken != "", old.AccessTokenExpiresAt.UTC().Format(time.RFC3339), old.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), replacement.AccessTokenExpiresAt.UTC().Format(time.RFC3339), replacement.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), cloudBootstrapSharedExecutorID, refreshCalls.Load(), first.view.Account.Status, first.view.Account.BindingStatus, first.view.Account.BindingGeneration, first.view.Account.CloudUserID != "", first.view.Account.DisplayName != "")

	second, err := app.CloudBootstrap()
	if err != nil {
		t.Fatalf("second CloudBootstrap returned %v", err)
	}
	if second.Account.Status != cloud.AccountExpired || second.Account.DisplayName != "" || second.OperationError == nil || second.OperationError.Code != cloud.ErrorAuthRequired || second.OperationError.RequestID != "req-cross-current-auth" {
		code, requestID := "", ""
		if second.OperationError != nil {
			code, requestID = string(second.OperationError.Code), second.OperationError.RequestID
		}
		t.Fatalf("second bootstrap state/display/error = %s/%t/%s/%s", second.Account.Status, second.Account.DisplayName != "", code, requestID)
	}
	expired, found, err := runtime.sessionStore.Load(ctx, scope)
	if err != nil || !found {
		t.Fatalf("expired session load failed: found=%t state=%s", found, expired.State)
	}
	if expired.State != cloud.SessionExpired || expired.AccessToken != "" || expired.RefreshToken != "" || expired.DeviceID != "" || !expired.AccessTokenExpiresAt.IsZero() || !expired.RefreshTokenExpiresAt.IsZero() || expired.TerminationReason != cloud.ErrorAuthRequired || expired.TerminatedAt.IsZero() {
		t.Fatalf("expired session state/presence/reason = %s/%t/%t/%s/%t", expired.State, expired.AccessToken != "", expired.RefreshToken != "", expired.TerminationReason, !expired.TerminatedAt.IsZero())
	}
	t.Logf("test_build=%s test_version=%s test_commit=%s observed_at_utc=%s local_timezone_name=%s local_timezone_offset_seconds=%d base_environment=localhost_test source=account profile=%s generation=%d session=%s access_present=%t refresh_present=%t access_expiry_before=%s refresh_expiry_before=%s access_expiry_after=%s refresh_expiry_after=%s executor_test_id=%s refresh_attempt_id=not_applicable refresh_attempt_count=%d trigger_type=protected_request http_status=200 error_code=%s request_id=%s sameObservedSession=true termination_skipped=false session_revision_supported=false bootstrap_stage=second bootstrap_account_status=%s binding_status=%s binding_generation=%d cloud_user_id_present=%t display_name_present=%t operation_error_code=%s operation_error_request_id=%s termination_time_present=%t", cloudBootstrapTestBuild, cloudBootstrapTestVersion, cloudBootstrapTestCommit, diagnosticAtUTC, localTimezoneName, localTimezoneOffsetSeconds, scope.ProfileID, scope.BindingGeneration, expired.State, expired.AccessToken != "", expired.RefreshToken != "", replacementAccessExpiry.Format(time.RFC3339), replacementRefreshExpiry.Format(time.RFC3339), expired.AccessTokenExpiresAt.UTC().Format(time.RFC3339), expired.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), cloudBootstrapSharedExecutorID, refreshCalls.Load(), second.OperationError.Code, second.OperationError.RequestID, second.Account.Status, second.Account.BindingStatus, second.Account.BindingGeneration, second.Account.CloudUserID != "", second.Account.DisplayName != "", second.OperationError.Code, second.OperationError.RequestID, !expired.TerminatedAt.IsZero())
}

func TestRemoteAccountUsesInjectedExecutorForSharedRefreshBoundary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	now := time.Now().UTC()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "shared-executor"}
	bindings := cloud.NewBindingRepository()
	binding, err := bindings.ConfirmBinding(ctx, actor, "usr_shared_executor")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	store := cloud.NewMemorySessionStore()
	sessions := cloud.NewProfileSessionRepository(bindings, store)
	if err := store.Save(ctx, cloud.Session{
		Scope: scope, State: cloud.SessionAuthenticated, AccessToken: "access-shared",
		RefreshToken: "refresh-shared", DeviceID: "device-shared",
		AccessTokenExpiresAt: now.Add(-time.Minute), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	client := &desktopExecutorProbeClient{snapshot: cloud.CurrentUserSnapshot{
		User: cloud.CloudUser{CloudUserID: scope.CloudUserID, DisplayName: "Shared Executor User"},
		Balance: cloud.BalanceSnapshot{
			TotalBalance: "1.0000", FreeBalance: "1.0000", SubscriptionBalance: "0.0000",
			AvailableBalance: "1.0000", FreeAvailableBalance: "1.0000", SubscriptionAvailableBalance: "0.0000",
			FrozenBalance: "0.0000", FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000", Version: 1,
		},
		Subscription: cloud.CurrentSubscription{Plan: "free", Status: cloud.SubscriptionFree}, ServerTime: now,
	}}
	executor := newDesktopExecutorProbe()
	remoteAccount, err := cloud.NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	projectDone := make(chan error, 1)
	go func() {
		projectDone <- executor.Do(ctx, scope, func(context.Context, string) error { return nil })
	}()
	select {
	case <-executor.firstEntered:
	case <-ctx.Done():
		t.Fatalf("project executor did not enter: %v", ctx.Err())
	}
	accountDone := make(chan error, 1)
	go func() {
		_, err := remoteAccount.CurrentUser(ctx, actor)
		accountDone <- err
	}()
	select {
	case <-executor.secondEntered:
	case <-ctx.Done():
		t.Fatalf("account executor did not enter before refresh release: %v", ctx.Err())
	}
	close(executor.releaseFirst)
	if err := <-projectDone; err != nil {
		t.Fatalf("project executor = %v", err)
	}
	if err := <-accountDone; err != nil {
		t.Fatalf("account hydration = %v", err)
	}
	if executor.calls.Load() != 2 || client.currentUserCalls.Load() != 1 {
		t.Fatalf("shared executor calls = %d, current user calls = %d", executor.calls.Load(), client.currentUserCalls.Load())
	}
}

type desktopExecutorProbe struct {
	mu            sync.Mutex
	calls         atomic.Int32
	firstEntered  chan struct{}
	secondEntered chan struct{}
	releaseFirst  chan struct{}
}

func newDesktopExecutorProbe() *desktopExecutorProbe {
	return &desktopExecutorProbe{firstEntered: make(chan struct{}), secondEntered: make(chan struct{}), releaseFirst: make(chan struct{})}
}

func (e *desktopExecutorProbe) Do(ctx context.Context, _ cloud.BindingScope, request func(context.Context, string) error) error {
	call := e.calls.Add(1)
	if call == 2 {
		close(e.secondEntered)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if call == 1 {
		close(e.firstEntered)
		select {
		case <-e.releaseFirst:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return request(ctx, "access-shared")
}

type desktopExecutorProbeClient struct {
	snapshot         cloud.CurrentUserSnapshot
	currentUserCalls atomic.Int32
}

func (c *desktopExecutorProbeClient) Login(context.Context, cloud.LoginRequest) (cloud.AuthenticationResult, error) {
	return cloud.AuthenticationResult{}, nil
}

func (c *desktopExecutorProbeClient) Refresh(context.Context, string) (cloud.RemoteSession, error) {
	return cloud.RemoteSession{}, &cloud.Error{Code: cloud.ErrorCode("TEST_UNEXPECTED_REFRESH"), Message: "refresh is controlled by the executor probe"}
}

func (c *desktopExecutorProbeClient) Logout(context.Context, string) error { return nil }

func (c *desktopExecutorProbeClient) CurrentUser(context.Context, string) (cloud.CurrentUserSnapshot, error) {
	c.currentUserCalls.Add(1)
	return c.snapshot, nil
}
