package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestAuthLoginDifferentCloudUserDoesNotMutateDurableState(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	provider := testProfilePathProvider(t.TempDir())
	bundle := loadA0Fixture(t)
	primaryClient, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Login: "auth_success"})
	if err != nil {
		t.Fatal(err)
	}

	bindings := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	sessionStore := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	assets := NewDurableAssetSnapshotRepository(provider)
	events := NewDurableEventStateRepository(provider)
	app := newDurableCandidateTestAuthApplication(t, bindings, sessions, sessionStore, events, assets, primaryClient)
	primary, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatalf("primary Login: %v", err)
	}
	primaryScope, err := primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	primarySnapshot := accountSnapshotForScope(t, bundle, primaryScope)
	if _, err := assets.StoreBalance(ctx, primaryScope, primarySnapshot.Balance); err != nil {
		t.Fatal(err)
	}
	if err := assets.StoreSubscription(ctx, primaryScope, subscriptionView(bundle.PlanCases, primarySnapshot.Subscription)); err != nil {
		t.Fatal(err)
	}
	if err := events.CommitApplied(ctx, primaryScope, eventStateEvent("candidate-guard", EventBalanceUpdated, primaryScope.CloudUserID, 1)); err != nil {
		t.Fatal(err)
	}

	stateFiles := []string{"binding.json", "sessions.json", "asset_snapshots.json", "event_state.json"}
	before := readDurableStateFiles(t, provider, actor, stateFiles)

	candidateScope, err := NewBindingScope(actor, "usr_candidate", primaryScope.BindingGeneration+1)
	if err != nil {
		t.Fatal(err)
	}
	candidateBinding := ProfileBinding{
		ActorID:           actor.ActorID,
		ProfileID:         actor.ProfileID,
		CloudUserID:       candidateScope.CloudUserID,
		Status:            BindingBound,
		BindingGeneration: candidateScope.BindingGeneration,
	}
	candidateSubscription := primarySnapshot.Subscription
	candidateSubscription.Scope = candidateScope
	candidateClient := candidateLoginClient{
		CloudAccountClient: primaryClient,
		state: AccountState{
			Binding:      candidateBinding,
			State:        AccountAuthenticated,
			User:         CloudUser{CloudUserID: candidateScope.CloudUserID, DisplayName: "Candidate"},
			Balance:      primarySnapshot.Balance,
			Subscription: candidateSubscription,
			LastSyncAt:   time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		session: Session{
			Scope:        candidateScope,
			State:        SessionAuthenticated,
			AccessToken:  "candidate-access",
			RefreshToken: "candidate-refresh",
		},
	}
	app.client = candidateClient
	candidate, err := app.Login(ctx, actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if !isProfileMismatch(err) {
		t.Fatalf("candidate Login error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	if candidate.State != AccountOffline || candidate.Binding != candidateBinding || candidate.User != candidateClient.state.User {
		t.Fatalf("candidate Login view = %#v, want token-safe candidate binding/user", candidate)
	}
	candidateJSON, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(candidateJSON), candidateClient.session.AccessToken) || strings.Contains(string(candidateJSON), candidateClient.session.RefreshToken) {
		t.Fatal("candidate Login view exposed a session token")
	}

	after := readDurableStateFiles(t, provider, actor, stateFiles)
	for _, name := range stateFiles {
		if !bytes.Equal(after[name], before[name]) {
			t.Fatalf("durable %s changed after candidate login", name)
		}
	}
	current, err := bindings.ResolveBinding(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if current != primary.Binding {
		t.Fatalf("binding after candidate login = %#v, want %#v", current, primary.Binding)
	}
	currentView, err := app.State(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if currentView.State != AccountAuthenticated || currentView.Binding != primary.Binding || currentView.User != primary.User {
		t.Fatalf("cached view after candidate login = %#v, want authenticated primary %#v", currentView, primary)
	}
}

func TestAuthCandidateRollbackDoesNotOverwriteConcurrentLogout(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())
	primary, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	blocking := newBlockingAccountClient(client)
	app.client = blocking
	loginDone := make(chan error, 1)
	go func() {
		_, err := app.Login(ctx, actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
		loginDone <- err
	}()
	<-blocking.loginStarted
	if err := app.Logout(ctx, actor); err != nil {
		t.Fatal(err)
	}
	close(blocking.loginRelease)
	if err := <-loginDone; !isProfileMismatch(err) {
		t.Fatalf("candidate Login error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	cached := app.cachedView(actor, primary.Binding)
	if cached.State != AccountAnonymous {
		t.Fatalf("cached view after concurrent logout = %#v, want anonymous", cached)
	}
}

func TestAuthFreshSecondaryLoginUsesFirstDurableGeneration(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "fresh-secondary"}
	provider := testProfilePathProvider(t.TempDir())
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	bindings := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	sessionStore := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	app := newDurableCandidateTestAuthApplication(
		t,
		bindings,
		sessions,
		sessionStore,
		NewDurableEventStateRepository(provider),
		NewDurableAssetSnapshotRepository(provider),
		client,
	)

	view, loginErr := app.Login(ctx, actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	wantBinding := ProfileBinding{
		ActorID:           actor.ActorID,
		ProfileID:         actor.ProfileID,
		CloudUserID:       "usr_02",
		Status:            BindingBound,
		BindingGeneration: 1,
	}
	storedBinding, resolveErr := bindings.ResolveBinding(ctx, actor)
	if loginErr != nil || resolveErr != nil || view.State != AccountAuthenticated || view.Binding != wantBinding || storedBinding != wantBinding {
		t.Fatalf("fresh secondary Login = view %#v, loginErr %v, binding %#v, resolveErr %v; want authenticated %#v", view, loginErr, storedBinding, resolveErr, wantBinding)
	}
	wantScope, err := wantBinding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	storedSession, ok, err := sessions.Load(ctx, actor)
	if err != nil || !ok || storedSession.Scope != wantScope {
		t.Fatalf("fresh secondary session = %#v, %t, %v; want scope %#v", storedSession, ok, err, wantScope)
	}
}

func TestAuthConcurrentCandidateLoginsRestoreStableView(t *testing.T) {
	for _, completionOrder := range []struct {
		name  string
		first int
	}{
		{name: "first attempt completes first", first: 0},
		{name: "second attempt completes first", first: 1},
	} {
		t.Run(completionOrder.name, func(t *testing.T) {
			ctx := context.Background()
			actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
			bundle := loadA0Fixture(t)
			delegate, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
			if err != nil {
				t.Fatal(err)
			}
			app := newAuthApplication(t, delegate, NewTestPasswordEncryptor())
			primary, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
			if err != nil {
				t.Fatal(err)
			}
			blocking := newOrderedCandidateClient(delegate)
			app.client = blocking
			results := []chan error{make(chan error, 1), make(chan error, 1)}
			for attempt := range results {
				attempt := attempt
				go func() {
					_, err := app.Login(ctx, actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
					results[attempt] <- err
				}()
				<-blocking.started[attempt]
			}

			second := 1 - completionOrder.first
			close(blocking.release[completionOrder.first])
			if err := <-results[completionOrder.first]; !isProfileMismatch(err) {
				t.Fatalf("candidate attempt %d error = %v, want PROFILE_SCOPE_MISMATCH", completionOrder.first, err)
			}
			close(blocking.release[second])
			if err := <-results[second]; !isProfileMismatch(err) {
				t.Fatalf("candidate attempt %d error = %v, want PROFILE_SCOPE_MISMATCH", second, err)
			}

			view, err := app.State(ctx, actor)
			if err != nil || view.State != AccountAuthenticated || view.Binding != primary.Binding || view.User != primary.User {
				t.Fatalf("view after concurrent candidates = %#v, %v; want authenticated primary %#v", view, err, primary)
			}
		})
	}
}

func TestAuthInvalidRemoteResultDoesNotCreateDurableBinding(t *testing.T) {
	for _, mutate := range []struct {
		name  string
		apply func(*AccountState, *Session)
	}{
		{
			name: "session scope",
			apply: func(_ *AccountState, session *Session) {
				session.Scope = BindingScope{ProfileID: "fresh-invalid", CloudUserID: "usr_01", BindingGeneration: 1}
			},
		},
		{
			name: "actor identity",
			apply: func(state *AccountState, _ *Session) {
				state.Binding.ActorID = "another-actor"
			},
		},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			ctx := context.Background()
			actor := userctx.ActorContext{ActorID: "local", ProfileID: "fresh-invalid"}
			provider := testProfilePathProvider(t.TempDir())
			bundle := loadA0Fixture(t)
			fixtureClient, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(LoginInput{Phone: "13900000000", Password: "test-password-02"})
			if err != nil {
				t.Fatal(err)
			}
			request := AccountFixtureRequest{
				Binding:        ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: BindingBinding},
				Operation:      AccountOperationLogin,
				FixtureVersion: A0FixtureVersion,
				Payload:        payload,
			}
			state, session, err := fixtureClient.Login(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			mutate.apply(&state, &session)

			bindings := NewDurableBindingRepository(provider, testEnvironmentIdentity())
			sessionStore := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
			sessions := NewProfileSessionRepository(bindings, sessionStore)
			app := newDurableCandidateTestAuthApplication(
				t,
				bindings,
				sessions,
				sessionStore,
				NewDurableEventStateRepository(provider),
				NewDurableAssetSnapshotRepository(provider),
				candidateLoginClient{state: state, session: session},
			)

			if _, err := app.Login(ctx, actor, LoginInput{Phone: "13900000000", Password: "test-password-02"}); !isProfileMismatch(err) {
				t.Fatalf("invalid remote Login error = %v, want PROFILE_SCOPE_MISMATCH", err)
			}
			binding, err := bindings.ResolveBinding(ctx, actor)
			if err != nil || binding.Status != BindingUnbound {
				t.Fatalf("binding after invalid remote result = %#v, %v; want unbound", binding, err)
			}
			path, err := durableCloudFile(provider, actor, "binding.json")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("binding file after invalid remote result error = %v, want not-exist", err)
			}
		})
	}
}

type candidateLoginClient struct {
	CloudAccountClient
	state   AccountState
	session Session
}

type orderedCandidateClient struct {
	CloudAccountClient
	mu      sync.Mutex
	calls   int
	started [2]chan struct{}
	release [2]chan struct{}
}

func newOrderedCandidateClient(client CloudAccountClient) *orderedCandidateClient {
	return &orderedCandidateClient{
		CloudAccountClient: client,
		started:            [2]chan struct{}{make(chan struct{}), make(chan struct{})},
		release:            [2]chan struct{}{make(chan struct{}), make(chan struct{})},
	}
}

func (c *orderedCandidateClient) Login(ctx context.Context, request AccountFixtureRequest) (AccountState, Session, error) {
	c.mu.Lock()
	call := c.calls
	c.calls++
	c.mu.Unlock()
	if call >= len(c.started) {
		return AccountState{}, Session{}, errors.New("unexpected candidate login call")
	}
	close(c.started[call])
	<-c.release[call]
	return c.CloudAccountClient.Login(ctx, request)
}

func (c candidateLoginClient) Login(context.Context, AccountFixtureRequest) (AccountState, Session, error) {
	return c.state, c.session, nil
}

func newDurableCandidateTestAuthApplication(
	t *testing.T,
	bindings BindingStore,
	sessions *ProfileSessionRepository,
	sessionStore CloudSessionStore,
	events EventStateRepository,
	assets AssetSnapshotRepository,
	client CloudAccountClient,
) *AuthApplication {
	t.Helper()
	captcha, err := NewMockCaptchaGateway("captcha_ticket_01", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings:       bindings,
		Sessions:       sessions,
		Cleanup:        NewIdentityCleanup(sessionStore, events, assets),
		Captcha:        captcha,
		Encryptor:      NewTestPasswordEncryptor(),
		Client:         client,
		FixtureVersion: A0FixtureVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func accountSnapshotForScope(t *testing.T, bundle FixtureBundle, scope BindingScope) AccountSnapshotFixture {
	t.Helper()
	for _, snapshot := range bundle.AccountSnapshots {
		if snapshot.Scope == scope {
			return snapshot
		}
	}
	t.Fatalf("missing account snapshot for scope %#v", scope)
	return AccountSnapshotFixture{}
}

func readDurableStateFiles(t *testing.T, provider ProfilePathProvider, actor userctx.ActorContext, names []string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte, len(names))
	for _, name := range names {
		path, err := durableCloudFile(provider, actor, name)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = content
	}
	return files
}
