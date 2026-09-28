package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestRemoteAccountApplicationRequiresExplicitExecutor(t *testing.T) {
	bindings := NewBindingRepository()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	if _, err := NewRemoteAccountApplication(bindings, sessions, &concurrentSessionClient{}, nil); err == nil {
		t.Fatal("NewRemoteAccountApplication accepted a nil executor")
	}
}

func TestRemoteAccountApplicationProjectsV1CapabilityPassThrough(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, err := bindings.ConfirmBinding(context.Background(), actor, "usr_remote")
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := binding.Scope()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	if err := sessions.Save(context.Background(), actor, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	snapshot := currentUserFixture(now)
	snapshot.Capabilities.ProjectsV1 = json.RawMessage(`{"enabled":false}`)
	snapshot.Capabilities.ProjectsV2 = json.RawMessage(`{"enabled":true,"slices":{}}`)
	client := &concurrentSessionClient{snapshot: snapshot}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	state, err := app.CurrentUser(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(state.Capabilities.ProjectsV1); got != `{"enabled":false}` {
		t.Fatalf("projects_v1 raw = %q", got)
	}
	if got := string(state.Capabilities.ProjectsV2); got != `{"enabled":true,"slices":{}}` {
		t.Fatalf("projects_v2 raw = %q", got)
	}
	snapshot.Capabilities.ProjectsV1[1] = 'X'
	snapshot.Capabilities.ProjectsV2[1] = 'X'
	if got := string(state.Capabilities.ProjectsV1); got != `{"enabled":false}` {
		t.Fatalf("account state aliased session snapshot: %q", got)
	}
	if got := string(state.Capabilities.ProjectsV2); got != `{"enabled":true,"slices":{}}` {
		t.Fatalf("account state aliased projects_v2 snapshot: %q", got)
	}
}

func TestRemoteAccountApplicationSerializesRefreshAndPersistsRotation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, err := bindings.ConfirmBinding(context.Background(), actor, "usr_remote")
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := binding.Scope()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	if err := sessions.Save(context.Background(), actor, Session{
		Scope: scope, State: SessionAuthenticated, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01",
		AccessTokenExpiresAt: now.Add(-time.Minute), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	client := &concurrentSessionClient{refresh: RemoteSession{
		AccessToken: "access-new", RefreshToken: "refresh-new", DeviceID: "device-01",
		AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(48 * time.Hour),
	}, snapshot: currentUserFixture(now)}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state, err := app.CurrentUser(context.Background(), actor)
			if err == nil && (state.User.CloudUserID != "usr_remote" || state.Balance.TotalBalance != "1000.0000" || state.Subscription.Version != 0) {
				err = &Error{Code: ErrorCode("TEST_FAILURE"), Message: "unexpected current user state"}
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if client.RefreshCalls() != 1 {
		t.Fatalf("refresh calls = %d, want 1", client.RefreshCalls())
	}
	stored, ok, err := sessions.Load(context.Background(), actor)
	if err != nil || !ok || stored.AccessToken != "access-new" || stored.RefreshToken != "refresh-new" {
		t.Fatalf("stored rotation = %#v, %t, %v", stored, ok, err)
	}
}

func TestRemoteAccountApplicationRetriesCurrentUserOnceOnTokenExpired(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, _ := bindings.ConfirmBinding(context.Background(), actor, "usr_remote")
	scope, _ := binding.Scope()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	_ = sessions.Save(context.Background(), actor, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)})
	client := &concurrentSessionClient{refresh: RemoteSession{AccessToken: "access-new", RefreshToken: "refresh-new", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(2 * time.Hour), RefreshTokenExpiresAt: now.Add(48 * time.Hour)}, snapshot: currentUserFixture(now), expireFirstCurrentUser: true}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CurrentUser(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	if client.RefreshCalls() != 1 || client.CurrentUserCalls() != 2 {
		t.Fatalf("refresh/current calls = %d/%d", client.RefreshCalls(), client.CurrentUserCalls())
	}
}

func TestRemoteAccountApplicationRetriesTerminalCurrentUserWithReplacement(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 4, 30, 0, 0, time.UTC)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, err := bindings.ConfirmBinding(ctx, actor, "usr_remote")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemorySessionStore()
	sessions := NewProfileSessionRepository(bindings, store)
	old := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)}
	if err := store.Save(ctx, old); err != nil {
		t.Fatal(err)
	}
	replacement := old
	replacement.AccessToken = "access-new"
	replacement.RefreshToken = "refresh-new"
	replacement.AccessTokenExpiresAt = now.Add(2 * time.Hour)
	replacement.RefreshTokenExpiresAt = now.Add(48 * time.Hour)
	client := &concurrentSessionClient{
		snapshot:            currentUserFixture(now),
		firstCurrentUserErr: &Error{Code: ErrorCode("REFRESH_TOKEN_REUSED"), Message: "refresh token was reused", RequestID: "req-late-reused"},
		currentUserHook: func(call int) {
			if call == 1 {
				if err := store.Save(ctx, replacement); err != nil {
					t.Fatalf("save replacement: %v", err)
				}
			}
		},
	}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	observerCalls := 0
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor, func(_ context.Context, _ userctx.ActorContext, state AccountState) error {
		observerCalls++
		if state.User.DisplayName != "Remote User" {
			t.Fatalf("observer display name = %q", state.User.DisplayName)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := app.CurrentUser(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if state.User.DisplayName != "Remote User" || client.CurrentUserCalls() != 2 || observerCalls != 1 {
		t.Fatalf("replacement hydration = display_name:%q current_calls:%d observer_calls:%d", state.User.DisplayName, client.CurrentUserCalls(), observerCalls)
	}
	current, found, err := store.Load(ctx, scope)
	if err != nil || !found || current.State != SessionAuthenticated || !sameObservedSession(current, replacement) {
		t.Fatalf("replacement session = %#v, found=%t, err=%v", current, found, err)
	}
}

func TestRemoteAccountApplicationReplacementTerminalScrubsWithoutObserver(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 5, 0, 0, 0, time.UTC)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, err := bindings.ConfirmBinding(ctx, actor, "usr_remote")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemorySessionStore()
	sessions := NewProfileSessionRepository(bindings, store)
	old := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)}
	if err := store.Save(ctx, old); err != nil {
		t.Fatal(err)
	}
	replacement := old
	replacement.AccessToken = "access-new"
	replacement.RefreshToken = "refresh-new"
	replacement.AccessTokenExpiresAt = now.Add(2 * time.Hour)
	replacement.RefreshTokenExpiresAt = now.Add(48 * time.Hour)
	client := &concurrentSessionClient{
		firstCurrentUserErr: &Error{Code: ErrorCode("REFRESH_TOKEN_REUSED"), Message: "refresh token was reused", RequestID: "req-late-reused"},
		currentUserErr:      &Error{Code: ErrorAuthRequired, Message: "authentication is required", RequestID: "req-replacement-auth"},
		currentUserHook: func(call int) {
			if call == 1 {
				if err := store.Save(ctx, replacement); err != nil {
					t.Fatalf("save replacement: %v", err)
				}
			}
		},
	}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	observerCalls := 0
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor, func(context.Context, userctx.ActorContext, AccountState) error {
		observerCalls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, currentErr := app.CurrentUser(ctx, actor)
	if !isCloudCode(currentErr, ErrorAuthRequired) {
		t.Fatalf("replacement terminal error code = %v", currentErr)
	}
	var cloudErr *Error
	if !errors.As(currentErr, &cloudErr) || cloudErr.RequestID != "req-replacement-auth" {
		t.Fatalf("replacement terminal error request_id = %v", currentErr)
	}
	if client.CurrentUserCalls() != 2 || observerCalls != 0 {
		t.Fatalf("replacement terminal calls = %d, observer_calls=%d", client.CurrentUserCalls(), observerCalls)
	}
	current, found, err := store.Load(ctx, scope)
	if err != nil || !found || current.State != SessionExpired || current.AccessToken != "" || current.RefreshToken != "" || current.DeviceID != "" || current.TerminationReason != ErrorAuthRequired || current.TerminatedAt.IsZero() {
		t.Fatalf("replacement terminal session = %#v, found=%t, err=%v", current, found, err)
	}
}

func TestRemoteAccountApplicationTerminalRetryGuards(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 5, 30, 0, 0, time.UTC)
	ctx := context.Background()
	cases := []struct {
		name          string
		state         SessionState
		firstErr      error
		currentErr    error
		mutateCurrent bool
		postLoadFail  bool
		switchBinding bool
		requestID     string
	}{
		{name: "current-expired", state: SessionAuthenticated, firstErr: &Error{Code: ErrorAuthRequired, Message: "expired", RequestID: "req-current-expired"}, mutateCurrent: true, requestID: "req-current-expired"},
		{name: "unchanged", state: SessionAuthenticated, firstErr: &Error{Code: ErrorCode("REFRESH_TOKEN_REUSED"), Message: "unchanged", RequestID: "req-unchanged"}, requestID: "req-unchanged"},
		{name: "offline", state: SessionOffline, currentErr: &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true, RequestID: "req-offline"}, requestID: "req-offline"},
		{name: "post-load-failure", state: SessionAuthenticated, firstErr: &Error{Code: ErrorAuthRequired, Message: "post-load", RequestID: "req-post-load"}, postLoadFail: true, requestID: "req-post-load"},
		{name: "binding-switch", state: SessionAuthenticated, firstErr: &Error{Code: ErrorAuthRequired, Message: "binding switched", RequestID: "req-binding-switch"}, switchBinding: true, requestID: "req-binding-switch"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
			baseBindings := NewBindingRepository()
			oldBinding, err := baseBindings.ConfirmBinding(ctx, actor, "usr_remote")
			if err != nil {
				t.Fatal(err)
			}
			bindings := BindingStore(baseBindings)
			var tracking *trackingSessionStore
			baseStore := NewMemorySessionStore()
			var sessionStore CloudSessionStore = baseStore
			if testCase.switchBinding {
				nextBinding := oldBinding
				nextBinding.CloudUserID = "usr_new"
				nextBinding.BindingGeneration++
				bindings = &switchingBindingStore{BindingRepository: baseBindings, old: oldBinding, next: nextBinding}
				tracking = &trackingSessionStore{CloudSessionStore: baseStore}
				sessionStore = tracking
			}
			if testCase.postLoadFail {
				sessionStore = &failOnLoadSessionStore{CloudSessionStore: sessionStore, failAt: 3, err: &Error{Code: ErrorCode("TEST_POST_LOAD"), Message: "post-load failed", RequestID: "req-post-load-check"}}
			}
			sessions := NewProfileSessionRepository(bindings, sessionStore)
			binding := oldBinding
			scope, err := binding.Scope()
			if err != nil {
				t.Fatal(err)
			}
			old := Session{Scope: scope, State: testCase.state, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)}
			if err := sessionStore.Save(ctx, old); err != nil {
				t.Fatal(err)
			}
			client := &concurrentSessionClient{snapshot: currentUserFixture(now), firstCurrentUserErr: testCase.firstErr, currentUserErr: testCase.currentErr}
			if testCase.mutateCurrent {
				client.currentUserHook = func(call int) {
					if call == 1 {
						if err := sessionStore.Save(ctx, terminateSession(old, ErrorAuthRequired, now)); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			executor := &scriptedAccountExecutor{store: sessionStore}
			observerCalls := 0
			app, err := NewRemoteAccountApplication(bindings, sessions, client, executor, func(context.Context, userctx.ActorContext, AccountState) error {
				observerCalls++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			_, currentErr := app.CurrentUser(ctx, actor)
			if !isCloudCode(currentErr, errorCodeForRemoteAccountTest(testCase.firstErr, testCase.currentErr)) {
				t.Fatalf("error code = %v", currentErr)
			}
			var cloudErr *Error
			if !errors.As(currentErr, &cloudErr) || cloudErr.RequestID != testCase.requestID {
				t.Fatalf("request_id = %v", currentErr)
			}
			if client.CurrentUserCalls() != 1 || executor.calls != 1 || observerCalls != 0 {
				t.Fatalf("terminal guard calls = current:%d executor:%d observer:%d", client.CurrentUserCalls(), executor.calls, observerCalls)
			}
			if tracking != nil {
				if len(tracking.scopes) == 0 {
					t.Fatal("binding switch did not load the original scope")
				}
				for _, loadedScope := range tracking.scopes {
					if loadedScope != scope {
						t.Fatalf("loaded scope = %#v, want original scope %#v", loadedScope, scope)
					}
				}
			}
		})
	}
}

func TestRemoteAccountApplicationKeepsNetworkErrorWhenRefreshAfterTokenExpiryFails(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, _ := bindings.ConfirmBinding(context.Background(), actor, "usr_remote")
	scope, _ := binding.Scope()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	_ = sessions.Save(context.Background(), actor, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)})
	client := &concurrentSessionClient{expireFirstCurrentUser: true, refreshErr: &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CurrentUser(context.Background(), actor); !isCloudCode(err, ErrorNetworkUnavailable) {
		t.Fatalf("CurrentUser() error = %v, want NETWORK_UNAVAILABLE", err)
	}
	stored, ok, err := sessions.Load(context.Background(), actor)
	if err != nil || !ok || stored.State != SessionOffline || stored.RefreshToken != "refresh-old" {
		t.Fatalf("offline session after refresh failure = %#v, %t, %v", stored, ok, err)
	}
}

func TestRemoteAccountApplicationRecoversOfflineSessionAfterNetworkReturns(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, _ := bindings.ConfirmBinding(context.Background(), actor, "usr_remote")
	scope, _ := binding.Scope()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	_ = sessions.Save(context.Background(), actor, Session{Scope: scope, State: SessionOffline, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)})
	client := &concurrentSessionClient{snapshot: currentUserFixture(now)}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CurrentUser(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	stored, ok, err := sessions.Load(context.Background(), actor)
	if err != nil || !ok || stored.State != SessionAuthenticated {
		t.Fatalf("recovered session = %#v, %t, %v", stored, ok, err)
	}
}

func TestRemoteAccountApplicationKeepsOfflineSessionWhenSnapshotIsInvalid(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	binding, _ := bindings.ConfirmBinding(context.Background(), actor, "usr_remote")
	scope, _ := binding.Scope()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	_ = sessions.Save(context.Background(), actor, Session{Scope: scope, State: SessionOffline, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)})
	client := &concurrentSessionClient{snapshot: CurrentUserSnapshot{}}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CurrentUser(context.Background(), actor); err == nil {
		t.Fatal("CurrentUser() error = nil, want invalid snapshot error")
	}
	stored, ok, err := sessions.Load(context.Background(), actor)
	if err != nil || !ok || stored.State != SessionOffline {
		t.Fatalf("session after invalid snapshot = %#v, %t, %v", stored, ok, err)
	}
}

func TestTerminalAuthenticatedRequestScrubsAndPersistsStableReason(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	environment := testEnvironmentIdentity()
	provider := testProfilePathProvider(t.TempDir())
	bindings := NewDurableBindingRepository(provider, environment)
	binding, err := bindings.ConfirmBinding(ctx, actor, "usr_remote")
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := binding.Scope()
	store := NewDurableCloudSessionStore(provider, environment)
	sessions := NewProfileSessionRepository(bindings, store)
	if err := sessions.Save(ctx, actor, Session{
		Scope: scope, State: SessionAuthenticated, AccessToken: "access-secret", RefreshToken: "refresh-secret", DeviceID: "session-device",
		AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	client := &concurrentSessionClient{currentUserErr: &Error{Code: ErrorAuthRequired, Message: "session is no longer active"}}
	executor := newRemoteAccountTestExecutor(t, sessions, client, now)
	app, err := NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CurrentUser(ctx, actor); !isCloudCode(err, ErrorAuthRequired) {
		t.Fatalf("CurrentUser error = %v", err)
	}
	restarted := NewDurableCloudSessionStore(provider, environment)
	expired, found, err := restarted.Load(ctx, scope)
	if err != nil || !found {
		t.Fatalf("restart load = %#v, %t, %v", expired, found, err)
	}
	if expired.State != SessionExpired || expired.AccessToken != "" || expired.RefreshToken != "" || expired.DeviceID != "" ||
		!expired.AccessTokenExpiresAt.IsZero() || !expired.RefreshTokenExpiresAt.IsZero() ||
		expired.TerminationReason != ErrorAuthRequired || !expired.TerminatedAt.Equal(now) {
		t.Fatalf("terminal session was not scrubbed: %#v", expired)
	}
}

func TestExplicitRefreshUsesTheSameTerminalScrubContract(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 5, 0, 0, 0, time.UTC)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	for _, code := range []ErrorCode{ErrorAuthRequired, ErrorCode("REFRESH_TOKEN_INVALID")} {
		t.Run(string(code), func(t *testing.T) {
			bindings := NewBindingRepository()
			binding, _ := bindings.ConfirmBinding(ctx, actor, "usr_remote")
			scope, _ := binding.Scope()
			store := NewMemorySessionStore()
			sessions := NewProfileSessionRepository(bindings, store)
			if err := sessions.Save(ctx, actor, Session{
				Scope: scope, State: SessionAuthenticated, AccessToken: "access-secret", RefreshToken: "refresh-secret", DeviceID: "session-device",
				AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
			}); err != nil {
				t.Fatal(err)
			}
			client := &concurrentSessionClient{refreshErr: &Error{Code: code, Message: "terminal refresh", Retryable: true}}
			app, err := NewAuthApplication(AuthApplicationDependencies{
				Bindings: bindings, Sessions: sessions,
				Cleanup:       NewIdentityCleanup(store, noopScopedStore{}, noopScopedStore{}),
				SessionClient: client,
			})
			if err != nil {
				t.Fatal(err)
			}
			view, err := app.Refresh(ctx, actor)
			if !isCloudCode(err, code) || view.State != AccountExpired {
				t.Fatalf("Refresh = %#v, %v", view, err)
			}
			expired, found, loadErr := sessions.Load(ctx, actor)
			if loadErr != nil || !found || expired.AccessToken != "" || expired.RefreshToken != "" ||
				expired.DeviceID != "" || !expired.AccessTokenExpiresAt.IsZero() ||
				!expired.RefreshTokenExpiresAt.IsZero() || expired.TerminationReason != code ||
				expired.TerminatedAt.IsZero() {
				t.Fatalf("explicit refresh terminal session = %#v, %t, %v", expired, found, loadErr)
			}
		})
	}
}

func TestLateAndRepeatedTerminalResponsesCannotOverwriteRotatedSession(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 6, 0, 0, 0, time.UTC)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	scope, _ := NewBindingScope(actor, "usr_remote", 1)
	store := NewMemorySessionStore()
	old := Session{
		Scope: scope, State: SessionAuthenticated, AccessToken: "access-old", RefreshToken: "refresh-old", DeviceID: "device-01",
		AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}
	if err := store.Save(ctx, old); err != nil {
		t.Fatal(err)
	}
	executor, err := NewAuthenticatedRequestExecutor(store, &concurrentSessionClient{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- executor.Do(ctx, scope, func(context.Context, string) error {
			close(started)
			<-release
			return &Error{Code: ErrorAuthRequired, Message: "late terminal"}
		})
	}()
	<-started
	rotated := old
	rotated.AccessToken = "access-new"
	rotated.RefreshToken = "refresh-new"
	rotated.AccessTokenExpiresAt = now.Add(2 * time.Hour)
	if err := store.Save(ctx, rotated); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; !isCloudCode(err, ErrorAuthRequired) {
		t.Fatalf("late request error = %v", err)
	}
	current, found, err := store.Load(ctx, scope)
	if err != nil || !found || current != rotated {
		t.Fatalf("late terminal overwrote rotation: %#v, %t, %v", current, found, err)
	}

	store = NewMemorySessionStore()
	_ = store.Save(ctx, old)
	executor, _ = NewAuthenticatedRequestExecutor(store, &concurrentSessionClient{}, func() time.Time { return now })
	terminal := func(context.Context, string) error {
		return &Error{Code: ErrorAuthRequired, Message: "terminal"}
	}
	if err := executor.Do(ctx, scope, terminal); !isCloudCode(err, ErrorAuthRequired) {
		t.Fatal(err)
	}
	first, _, _ := store.Load(ctx, scope)
	if err := executor.Do(ctx, scope, terminal); !isCloudCode(err, ErrorAuthRequired) {
		t.Fatal(err)
	}
	second, _, _ := store.Load(ctx, scope)
	if first != second {
		t.Fatalf("repeated termination was not idempotent: %#v -> %#v", first, second)
	}
}

func TestRetryableAuthenticatedRequestFailuresPreserveTokens(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 7, 0, 0, 0, time.UTC)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	scope, _ := NewBindingScope(actor, "usr_remote", 1)
	for _, code := range []ErrorCode{ErrorCode("SERVICE_UNAVAILABLE"), ErrorCode("RATE_LIMITED"), ErrorNetworkUnavailable} {
		t.Run(string(code), func(t *testing.T) {
			store := NewMemorySessionStore()
			original := Session{
				Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh", DeviceID: "device-01",
				AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
			}
			_ = store.Save(ctx, original)
			executor, _ := NewAuthenticatedRequestExecutor(store, &concurrentSessionClient{}, func() time.Time { return now })
			failure := &Error{Code: code, Message: "retry later", Retryable: true}
			if err := executor.Do(ctx, scope, func(context.Context, string) error { return failure }); !errors.Is(err, failure) {
				t.Fatalf("Do error = %v", err)
			}
			stored, found, err := store.Load(ctx, scope)
			if err != nil || !found || stored.State != SessionOffline ||
				stored.AccessToken != original.AccessToken || stored.RefreshToken != original.RefreshToken ||
				stored.DeviceID != original.DeviceID {
				t.Fatalf("retryable failure changed credentials: %#v, %t, %v", stored, found, err)
			}
		})
	}
}

func TestRetryableFlagCannotDowngradeTerminalAuthenticatedRequest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	scope, _ := NewBindingScope(actor, "usr_remote", 1)
	for _, code := range []ErrorCode{ErrorAuthRequired, ErrorCode("ACCOUNT_DISABLED")} {
		t.Run(string(code), func(t *testing.T) {
			store := NewMemorySessionStore()
			original := Session{
				Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh", DeviceID: "device-01",
				AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
			}
			if err := store.Save(ctx, original); err != nil {
				t.Fatal(err)
			}
			executor, err := NewAuthenticatedRequestExecutor(store, &concurrentSessionClient{}, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			failure := &Error{Code: code, Message: "terminal", Retryable: true}
			if err := executor.Do(ctx, scope, func(context.Context, string) error { return failure }); !errors.Is(err, failure) {
				t.Fatalf("Do error = %v", err)
			}
			stored, found, err := store.Load(ctx, scope)
			if err != nil || !found || stored.State != SessionExpired ||
				stored.AccessToken != "" || stored.RefreshToken != "" || stored.DeviceID != "" ||
				!stored.AccessTokenExpiresAt.IsZero() || !stored.RefreshTokenExpiresAt.IsZero() ||
				stored.TerminationReason != code || !stored.TerminatedAt.Equal(now) {
				t.Fatalf("retryable terminal was not scrubbed: %#v, %t, %v", stored, found, err)
			}
		})
	}
}

func currentUserFixture(now time.Time) CurrentUserSnapshot {
	return CurrentUserSnapshot{
		User:         CloudUser{CloudUserID: "usr_remote", DisplayName: "Remote User"},
		Balance:      BalanceSnapshot{TotalBalance: "1000.0000", FreeBalance: "1000.0000", SubscriptionBalance: "0.0000", AvailableBalance: "1000.0000", FreeAvailableBalance: "1000.0000", SubscriptionAvailableBalance: "0.0000", FrozenBalance: "0.0000", FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000", Version: 1},
		Subscription: CurrentSubscription{Plan: "free", Status: SubscriptionFree, Version: 0}, ServerTime: now,
	}
}

type concurrentSessionClient struct {
	mu                     sync.Mutex
	refresh                RemoteSession
	refreshErr             error
	snapshot               CurrentUserSnapshot
	refreshCalls           int
	currentUserCalls       int
	expireFirstCurrentUser bool
	firstCurrentUserErr    error
	currentUserHook        func(int)
	currentUserErr         error
}

func (c *concurrentSessionClient) Login(context.Context, LoginRequest) (AuthenticationResult, error) {
	return AuthenticationResult{}, nil
}

func (c *concurrentSessionClient) Refresh(context.Context, string) (RemoteSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshCalls++
	return c.refresh, c.refreshErr
}

func (c *concurrentSessionClient) Logout(context.Context, string) error { return nil }

func (c *concurrentSessionClient) CurrentUser(context.Context, string) (CurrentUserSnapshot, error) {
	c.mu.Lock()
	c.currentUserCalls++
	call := c.currentUserCalls
	hook := c.currentUserHook
	firstErr := c.firstCurrentUserErr
	currentErr := c.currentUserErr
	snapshot := c.snapshot
	expireFirst := c.expireFirstCurrentUser && call == 1
	c.mu.Unlock()
	if hook != nil {
		hook(call)
	}
	if expireFirst {
		return CurrentUserSnapshot{}, &Error{Code: ErrorCode("TOKEN_EXPIRED"), Message: "token expired"}
	}
	if call == 1 && firstErr != nil {
		return CurrentUserSnapshot{}, firstErr
	}
	if currentErr != nil {
		return CurrentUserSnapshot{}, currentErr
	}
	return snapshot, nil
}

func (c *concurrentSessionClient) RefreshCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshCalls
}

func (c *concurrentSessionClient) CurrentUserCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentUserCalls
}

func newRemoteAccountTestExecutor(t *testing.T, sessions *ProfileSessionRepository, client CloudSessionClient, now time.Time) AuthenticatedRequestExecutor {
	t.Helper()
	executor, err := NewAuthenticatedRequestExecutor(sessions.sessions, client, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func errorCodeForRemoteAccountTest(primary, fallback error) ErrorCode {
	for _, candidate := range []error{primary, fallback} {
		var cloudErr *Error
		if errors.As(candidate, &cloudErr) {
			return cloudErr.Code
		}
	}
	return ""
}

type scriptedAccountExecutor struct {
	store CloudSessionStore
	calls int
}

func (e *scriptedAccountExecutor) Do(ctx context.Context, scope BindingScope, request func(context.Context, string) error) error {
	e.calls++
	session, found, err := e.store.Load(ctx, scope)
	if err != nil {
		return err
	}
	if !found {
		return &Error{Code: ErrorAuthRequired, Message: "authentication is required"}
	}
	return request(ctx, session.AccessToken)
}

type failOnLoadSessionStore struct {
	CloudSessionStore
	failAt int
	loads  int
	err    error
}

func (s *failOnLoadSessionStore) Load(ctx context.Context, scope BindingScope) (Session, bool, error) {
	s.loads++
	if s.loads == s.failAt {
		return Session{}, false, s.err
	}
	return s.CloudSessionStore.Load(ctx, scope)
}

type trackingSessionStore struct {
	CloudSessionStore
	scopes []BindingScope
}

func (s *trackingSessionStore) Load(ctx context.Context, scope BindingScope) (Session, bool, error) {
	s.scopes = append(s.scopes, scope)
	return s.CloudSessionStore.Load(ctx, scope)
}

type switchingBindingStore struct {
	*BindingRepository
	old   ProfileBinding
	next  ProfileBinding
	calls int
}

func (s *switchingBindingStore) ResolveBinding(ctx context.Context, _ userctx.ActorContext) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	s.calls++
	if s.calls > 1 {
		return s.next, nil
	}
	return s.old, nil
}
