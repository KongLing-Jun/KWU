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

func TestPrepareAccountSwitchAuthenticatesWithoutMutatingCurrentIdentity(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	stateFiles := []string{"binding.json", "sessions.json", "asset_snapshots.json", "event_state.json"}
	before := readDurableStateFiles(t, h.provider, h.actor, stateFiles)

	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatalf("PrepareAccountSwitch: %v", err)
	}
	wantBinding := ProfileBinding{
		ActorID:           h.actor.ActorID,
		ProfileID:         h.actor.ProfileID,
		CloudUserID:       "usr_02",
		Status:            BindingBound,
		BindingGeneration: 2,
	}
	view := preparation.View()
	if view.Binding != wantBinding || view.User.CloudUserID != "usr_02" || view.State != AccountAuthenticated {
		t.Fatalf("candidate view = %#v, want authenticated usr_02 generation 2", view)
	}

	encoded, err := json.Marshal(preparation)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"test-password-02", "fixture_secondary_access", "fixture_secondary_refresh"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("candidate JSON exposed secret %q: %s", secret, encoded)
		}
	}
	after := readDurableStateFiles(t, h.provider, h.actor, stateFiles)
	for _, name := range stateFiles {
		if !bytes.Equal(after[name], before[name]) {
			t.Fatalf("durable %s changed during preparation", name)
		}
	}
	current, err := h.app.State(h.ctx, h.actor)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != AccountAuthenticated || current.Binding != h.primary.Binding || current.User != h.primary.User {
		t.Fatalf("current view after preparation = %#v, want unchanged %#v", current, h.primary)
	}
}

func TestCommitAccountSwitchClearsOldIdentityAndPersistsNewSession(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	oldScope, err := h.primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}

	view, err := h.app.CommitAccountSwitch(h.ctx, preparation)
	if err != nil {
		t.Fatalf("CommitAccountSwitch: %v", err)
	}
	if view.State != AccountAuthenticated || view.Binding.CloudUserID != "usr_02" || view.Binding.BindingGeneration != 2 {
		t.Fatalf("committed view = %#v, want authenticated usr_02 generation 2", view)
	}
	if _, found, err := h.sessionStore.Load(h.ctx, oldScope); err != nil || found {
		t.Fatalf("old session after switch = found %t, err %v", found, err)
	}
	if _, found, err := h.events.Load(h.ctx, oldScope); err != nil || found {
		t.Fatalf("old event state after switch = found %t, err %v", found, err)
	}
	if _, found, err := h.assets.LoadBalance(h.ctx, oldScope); err != nil || found {
		t.Fatalf("old balance after switch = found %t, err %v", found, err)
	}
	if _, found, err := h.assets.LoadSubscription(h.ctx, oldScope); err != nil || found {
		t.Fatalf("old subscription after switch = found %t, err %v", found, err)
	}
	newSession, found, err := h.sessions.Load(h.ctx, h.actor)
	if err != nil || !found {
		t.Fatalf("new session after switch = %#v, found %t, err %v", newSession, found, err)
	}
	newScope, err := view.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	if newSession.Scope != newScope || newSession.State != SessionAuthenticated || newSession.AccessToken != "fixture_secondary_access" {
		t.Fatalf("new session = %#v, want authenticated candidate in %#v", newSession, newScope)
	}
}

func TestCommitAccountSwitchAcceptsEveryRecoverableSourceSessionState(t *testing.T) {
	for _, test := range []struct {
		name  string
		state SessionState
		clear bool
	}{
		{name: "authenticated", state: SessionAuthenticated},
		{name: "offline", state: SessionOffline},
		{name: "expired", state: SessionExpired},
		{name: "missing", clear: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newDurableAccountSwitchHarness(t)
			oldScope, err := h.primary.Binding.Scope()
			if err != nil {
				t.Fatal(err)
			}
			current, found, err := h.sessionStore.Load(h.ctx, oldScope)
			if err != nil || !found {
				t.Fatalf("source session = %#v, found %t, err %v", current, found, err)
			}
			if test.clear {
				if err := h.sessionStore.Clear(h.ctx, oldScope); err != nil {
					t.Fatal(err)
				}
			} else {
				if test.state == SessionExpired {
					current = terminateSession(current, ErrorAuthRequired, time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC))
				} else {
					current.State = test.state
				}
				if err := h.sessionStore.Save(h.ctx, current); err != nil {
					t.Fatal(err)
				}
			}

			preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
			if err != nil {
				t.Fatalf("PrepareAccountSwitch: %v", err)
			}
			view, err := h.app.CommitAccountSwitch(h.ctx, preparation)
			if err != nil {
				t.Fatalf("CommitAccountSwitch: %v", err)
			}
			if view.State != AccountAuthenticated || view.Binding.CloudUserID != "usr_02" || view.Binding.BindingGeneration != 2 {
				t.Fatalf("committed view = %#v", view)
			}
		})
	}
}

func TestCommitAccountSwitchRejectsSessionAppearanceAfterMissingSnapshot(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	oldScope, err := h.primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	oldSession, found, err := h.sessionStore.Load(h.ctx, oldScope)
	if err != nil || !found {
		t.Fatalf("source session = %#v, found %t, err %v", oldSession, found, err)
	}
	if err := h.sessionStore.Clear(h.ctx, oldScope); err != nil {
		t.Fatal(err)
	}
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.sessionStore.Save(h.ctx, oldSession); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.CommitAccountSwitch(h.ctx, preparation); !isProfileMismatch(err) {
		t.Fatalf("CommitAccountSwitch error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	current, err := h.bindings.ResolveBinding(h.ctx, h.actor)
	if err != nil {
		t.Fatal(err)
	}
	if current != h.primary.Binding {
		t.Fatalf("binding after rejected switch = %#v, want %#v", current, h.primary.Binding)
	}
}

func TestAccountSwitchRoundTripIncrementsGeneration(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	secondary, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	secondaryView, err := h.app.CommitAccountSwitch(h.ctx, secondary)
	if err != nil {
		t.Fatal(err)
	}
	secondaryScope, err := secondaryView.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}

	primary, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	primaryView, err := h.app.CommitAccountSwitch(h.ctx, primary)
	if err != nil {
		t.Fatal(err)
	}
	if primaryView.Binding.CloudUserID != "usr_01" || primaryView.Binding.BindingGeneration != 3 || primaryView.State != AccountAuthenticated {
		t.Fatalf("round-trip view = %#v, want authenticated usr_01 generation 3", primaryView)
	}
	if _, found, err := h.sessionStore.Load(h.ctx, secondaryScope); err != nil || found {
		t.Fatalf("secondary session after switch back = found %t, err %v", found, err)
	}
	currentSession, found, err := h.sessions.Load(h.ctx, h.actor)
	if err != nil || !found || currentSession.Scope.BindingGeneration != 3 || currentSession.Scope.CloudUserID != "usr_01" {
		t.Fatalf("round-trip session = %#v, found %t, err %v", currentSession, found, err)
	}
}

func TestAccountSwitchPreparationFailureAndCancellationLeaveDurableStateUnchanged(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	stateFiles := []string{"binding.json", "sessions.json", "asset_snapshots.json", "event_state.json"}
	before := readDurableStateFiles(t, h.provider, h.actor, stateFiles)

	if _, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "wrong-password"}); err == nil {
		t.Fatal("PrepareAccountSwitch with wrong password succeeded")
	}
	h.app.client = candidateLoginClient{state: AccountState{}, session: Session{}}
	if _, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"}); err == nil {
		t.Fatal("PrepareAccountSwitch with invalid remote result succeeded")
	}
	h.app.client = h.client
	if _, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		t.Fatalf("cancelled preparation: %v", err)
	}

	assertDurableFilesEqual(t, h, stateFiles, before)
	current, err := h.app.State(h.ctx, h.actor)
	if err != nil || current != h.primary {
		t.Fatalf("current view after failed/cancelled preparation = %#v, %v; want %#v", current, err, h.primary)
	}
}

func TestAccountSwitchPreparationIsSingleUse(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.CommitAccountSwitch(h.ctx, preparation); err != nil {
		t.Fatal(err)
	}
	stateFiles := []string{"binding.json", "sessions.json", "asset_snapshots.json", "event_state.json"}
	before := readDurableStateFiles(t, h.provider, h.actor, stateFiles)
	if _, err := h.app.CommitAccountSwitch(h.ctx, preparation); err == nil {
		t.Fatal("second CommitAccountSwitch succeeded")
	}
	assertDurableFilesEqual(t, h, stateFiles, before)
	if _, err := h.app.CommitAccountSwitch(h.ctx, nil); err == nil {
		t.Fatal("nil CommitAccountSwitch succeeded")
	}
}

func TestAccountSwitchPreparationCopiesShareSingleUseState(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	copied := *preparation
	cancelled, cancel := context.WithCancel(h.ctx)
	cancel()
	if _, err := h.app.CommitAccountSwitch(cancelled, preparation); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled CommitAccountSwitch error = %v, want context.Canceled", err)
	}
	if _, err := h.app.CommitAccountSwitch(h.ctx, &copied); err == nil {
		t.Fatal("copied preparation was committed after original was consumed")
	}
	current, err := h.bindings.ResolveBinding(h.ctx, h.actor)
	if err != nil {
		t.Fatal(err)
	}
	if current != h.primary.Binding {
		t.Fatalf("binding after copied preparation = %#v, want unchanged %#v", current, h.primary.Binding)
	}
}

func TestCommitAccountSwitchRejectsIdentityChangesAfterPreparation(t *testing.T) {
	stateFiles := []string{"binding.json", "sessions.json", "asset_snapshots.json", "event_state.json"}
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *durableAccountSwitchHarness)
	}{
		{
			name: "logout",
			mutate: func(t *testing.T, h *durableAccountSwitchHarness) {
				t.Helper()
				if err := h.app.Logout(h.ctx, h.actor); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "session refresh",
			mutate: func(t *testing.T, h *durableAccountSwitchHarness) {
				t.Helper()
				if _, err := h.app.Refresh(h.ctx, h.actor); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "binding rebind",
			mutate: func(t *testing.T, h *durableAccountSwitchHarness) {
				t.Helper()
				if _, err := h.bindings.Rebind(h.ctx, h.actor, "usr_external"); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newDurableAccountSwitchHarness(t)
			preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, h)
			before := readDurableStateFiles(t, h.provider, h.actor, stateFiles)
			if _, err := h.app.CommitAccountSwitch(h.ctx, preparation); !isProfileMismatch(err) {
				t.Fatalf("CommitAccountSwitch error = %v, want PROFILE_SCOPE_MISMATCH", err)
			}
			assertDurableFilesEqual(t, h, stateFiles, before)
		})
	}
}

func TestCommitAccountSwitchDoesNotOverwriteExternalRebindDuringCleanup(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	blocker := &blockingSwitchScopedStore{started: make(chan struct{}), release: make(chan struct{})}
	h.app.cleanup = NewIdentityCleanup(h.sessionStore, blocker, h.assets)

	type result struct {
		view AuthView
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		view, err := h.app.CommitAccountSwitch(h.ctx, preparation)
		resultCh <- result{view: view, err: err}
	}()
	<-blocker.started
	external := NewDurableBindingRepository(h.provider, testEnvironmentIdentity())
	externalBinding, err := external.Rebind(h.ctx, h.actor, "usr_external")
	if err != nil {
		t.Fatal(err)
	}
	close(blocker.release)
	commit := <-resultCh
	if !isProfileMismatch(commit.err) {
		t.Fatalf("CommitAccountSwitch error = %v, want PROFILE_SCOPE_MISMATCH", commit.err)
	}
	if commit.view.State != AccountOffline || commit.view.Binding != externalBinding {
		t.Fatalf("fail-closed view = %#v, want offline external binding %#v", commit.view, externalBinding)
	}
	current, err := external.ResolveBinding(h.ctx, h.actor)
	if err != nil {
		t.Fatal(err)
	}
	if current != externalBinding || current.CloudUserID == "usr_02" {
		t.Fatalf("binding after raced commit = %#v, want external %#v", current, externalBinding)
	}
}

func TestCanceledAccountSwitchFailureCachesExternallyReboundIdentity(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	blocker := &blockingSwitchScopedStore{started: make(chan struct{}), release: make(chan struct{})}
	h.app.cleanup = NewIdentityCleanup(h.sessionStore, blocker, h.assets)
	defer closeChannelIfOpen(blocker.release)
	commitCtx, cancel := context.WithCancel(h.ctx)

	resultCh := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.CommitAccountSwitch(commitCtx, preparation)
		resultCh <- authCallResult{view: view, err: err}
	}()
	<-blocker.started
	external := NewDurableBindingRepository(h.provider, testEnvironmentIdentity())
	externalBinding, err := external.Rebind(h.ctx, h.actor, "usr_external")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	commit := <-resultCh
	if !errors.Is(commit.err, context.Canceled) {
		t.Fatalf("CommitAccountSwitch error = %v, want context.Canceled", commit.err)
	}
	if commit.view.State != AccountOffline || commit.view.Binding != externalBinding {
		t.Fatalf("canceled fail-closed view = %#v, want offline external binding %#v", commit.view, externalBinding)
	}
	if cached := h.app.cachedView(h.actor, externalBinding); cached != commit.view {
		t.Fatalf("cached canceled fail-closed view = %#v, want %#v", cached, commit.view)
	}
}

func TestRefreshResponseWaitsForAccountSwitchIdentityCommit(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	oldScope, err := h.primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	client := &barrierRefreshClient{
		CloudAccountClient: h.client,
		started:            make(chan struct{}),
		release:            make(chan struct{}),
		returning:          make(chan struct{}),
	}
	h.app.client = client
	cleanup := &blockingSwitchScopedStore{started: make(chan struct{}), release: make(chan struct{})}
	h.app.cleanup = NewIdentityCleanup(h.sessionStore, cleanup, h.assets)
	defer closeChannelIfOpen(cleanup.release)
	defer closeChannelIfOpen(client.release)

	refreshDone := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.Refresh(h.ctx, h.actor)
		refreshDone <- authCallResult{view: view, err: err}
	}()
	<-client.started

	commitDone := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.CommitAccountSwitch(h.ctx, preparation)
		commitDone <- authCallResult{view: view, err: err}
	}()
	<-cleanup.started
	close(client.release)
	<-client.returning
	close(cleanup.release)
	commit := <-commitDone
	refresh := <-refreshDone
	if commit.err != nil || commit.view.Binding.CloudUserID != "usr_02" {
		t.Fatalf("CommitAccountSwitch = %#v, %v", commit.view, commit.err)
	}
	if !isProfileMismatch(refresh.err) || refresh.view != commit.view {
		t.Fatalf("late Refresh = %#v, %v; want current switched view and PROFILE_SCOPE_MISMATCH", refresh.view, refresh.err)
	}
	if _, found, err := h.sessionStore.Load(h.ctx, oldScope); err != nil || found {
		t.Fatalf("old session after late Refresh = found %t, err %v", found, err)
	}
	newSession, found, err := h.sessions.Load(h.ctx, h.actor)
	if err != nil || !found || newSession.Scope.CloudUserID != "usr_02" {
		t.Fatalf("new session after late Refresh = %#v, found %t, err %v", newSession, found, err)
	}
	if cached := h.app.cachedView(h.actor, commit.view.Binding); cached != commit.view {
		t.Fatalf("cached view after late Refresh = %#v, want %#v", cached, commit.view)
	}
}

func TestLateLoginResponseCannotOverwriteSwitchedIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "successful response"},
		{name: "failed response", err: &Error{Code: ErrorAuthRequired, Message: "late login failed"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newDurableAccountSwitchHarness(t)
			oldScope, err := h.primary.Binding.Scope()
			if err != nil {
				t.Fatal(err)
			}
			preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
			if err != nil {
				t.Fatal(err)
			}
			client := &barrierLoginClient{
				CloudAccountClient: h.client,
				started:            make(chan struct{}),
				release:            make(chan struct{}),
				err:                test.err,
			}
			h.app.client = client
			defer closeChannelIfOpen(client.release)

			loginDone := make(chan authCallResult, 1)
			go func() {
				view, err := h.app.Login(h.ctx, h.actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
				loginDone <- authCallResult{view: view, err: err}
			}()
			<-client.started
			switched, err := h.app.CommitAccountSwitch(h.ctx, preparation)
			if err != nil {
				t.Fatal(err)
			}
			close(client.release)
			late := <-loginDone

			if !isProfileMismatch(late.err) || late.view != switched {
				t.Fatalf("late Login = %#v, %v; want current switched view and PROFILE_SCOPE_MISMATCH", late.view, late.err)
			}
			if _, found, err := h.sessionStore.Load(h.ctx, oldScope); err != nil || found {
				t.Fatalf("old session after late Login = found %t, err %v", found, err)
			}
			if cached := h.app.cachedView(h.actor, switched.Binding); cached != switched {
				t.Fatalf("cached view after late Login = %#v, want %#v", cached, switched)
			}
		})
	}
}

func TestLateRegisterResponseCannotOverwriteSwitchedIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "successful response"},
		{name: "failed response", err: &Error{Code: ErrorSMSCodeInvalid, Message: "late registration failed"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newDurableAccountSwitchHarness(t)
			oldScope, err := h.primary.Binding.Scope()
			if err != nil {
				t.Fatal(err)
			}
			preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
			if err != nil {
				t.Fatal(err)
			}
			client := &barrierRegisterClient{
				CloudAccountClient: h.client,
				started:            make(chan struct{}),
				release:            make(chan struct{}),
				err:                test.err,
			}
			h.app.client = client
			defer closeChannelIfOpen(client.release)

			registerDone := make(chan authCallResult, 1)
			go func() {
				view, err := h.app.Register(h.ctx, h.actor, RegisterInput{Phone: "13800000000", Password: "test-password-01", SMSCode: "123456"})
				registerDone <- authCallResult{view: view, err: err}
			}()
			<-client.started
			switched, err := h.app.CommitAccountSwitch(h.ctx, preparation)
			if err != nil {
				t.Fatal(err)
			}
			close(client.release)
			late := <-registerDone

			if !isProfileMismatch(late.err) || late.view != switched {
				t.Fatalf("late Register = %#v, %v; want current switched view and PROFILE_SCOPE_MISMATCH", late.view, late.err)
			}
			if _, found, err := h.sessionStore.Load(h.ctx, oldScope); err != nil || found {
				t.Fatalf("old session after late Register = found %t, err %v", found, err)
			}
			if cached := h.app.cachedView(h.actor, switched.Binding); cached != switched {
				t.Fatalf("cached view after late Register = %#v, want %#v", cached, switched)
			}
		})
	}
}

func TestStateSerializesIdentityReadAndViewWriteWithAccountSwitch(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	oldScope, err := h.primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	barrier := newFirstLoadBarrierSessionStore(h.sessionStore, oldScope)
	h.sessions.sessions = barrier
	defer closeChannelIfOpen(barrier.release)

	stateDone := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.State(h.ctx, h.actor)
		stateDone <- authCallResult{view: view, err: err}
	}()
	<-barrier.started
	if h.app.identityMu.TryLock() {
		h.app.identityMu.Unlock()
		t.Error("State session-load barrier did not hold identityMu")
	}
	commitDone := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.CommitAccountSwitch(h.ctx, preparation)
		commitDone <- authCallResult{view: view, err: err}
	}()

	close(barrier.release)
	state := <-stateDone
	commit := <-commitDone
	if state.err != nil {
		t.Fatalf("State: %v", state.err)
	}
	if commit.err != nil || commit.view.Binding.CloudUserID != "usr_02" {
		t.Fatalf("CommitAccountSwitch = %#v, %v", commit.view, commit.err)
	}
	if cached := h.app.cachedView(h.actor, commit.view.Binding); cached != commit.view {
		t.Fatalf("cached view after State/Commit interleave = %#v, want %#v", cached, commit.view)
	}
}

func TestRecoverSerializesIdentityReadAndViewWriteWithAccountSwitch(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	oldScope, err := h.primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	barrier := newFirstLoadBarrierSessionStore(h.sessionStore, oldScope)
	h.sessions.sessions = barrier
	defer closeChannelIfOpen(barrier.release)

	recoverDone := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.Recover(h.ctx, h.actor)
		recoverDone <- authCallResult{view: view, err: err}
	}()
	<-barrier.started
	if h.app.identityMu.TryLock() {
		h.app.identityMu.Unlock()
		t.Error("Recover session-load barrier did not hold identityMu")
	}
	commitDone := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.CommitAccountSwitch(h.ctx, preparation)
		commitDone <- authCallResult{view: view, err: err}
	}()

	close(barrier.release)
	recovered := <-recoverDone
	commit := <-commitDone
	if recovered.err != nil {
		t.Fatalf("Recover: %v", recovered.err)
	}
	if commit.err != nil || commit.view.Binding.CloudUserID != "usr_02" {
		t.Fatalf("CommitAccountSwitch = %#v, %v", commit.view, commit.err)
	}
	if cached := h.app.cachedView(h.actor, commit.view.Binding); cached != commit.view {
		t.Fatalf("cached view after Recover/Commit interleave = %#v, want %#v", cached, commit.view)
	}
}

func TestStateRejectsSessionFromDifferentResolvedBindingScope(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	wrongScope := BindingScope{ProfileID: h.actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 2}
	h.sessions.sessions = mismatchedLoadSessionStore{
		CloudSessionStore: h.sessionStore,
		session:           Session{Scope: wrongScope, State: SessionAuthenticated, AccessToken: "wrong-scope-access"},
	}

	view, err := h.app.State(h.ctx, h.actor)
	if !isProfileMismatch(err) {
		t.Fatalf("State error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	if view.State != AccountOffline || view.Binding != h.primary.Binding {
		t.Fatalf("State view = %#v, want offline current binding %#v", view, h.primary.Binding)
	}
	if cached := h.app.cachedView(h.actor, h.primary.Binding); cached != view {
		t.Fatalf("cached mismatched-scope view = %#v, want %#v", cached, view)
	}
}

func TestCanceledBoundAuthenticationAttemptsRestoreStableIdentity(t *testing.T) {
	for _, test := range []struct {
		name      string
		started   func(*cancelBarrierAccountClient) <-chan struct{}
		operation func(context.Context, *AuthApplication, userctx.ActorContext) (AuthView, error)
	}{
		{
			name:    "login",
			started: func(client *cancelBarrierAccountClient) <-chan struct{} { return client.loginStarted },
			operation: func(ctx context.Context, app *AuthApplication, actor userctx.ActorContext) (AuthView, error) {
				return app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
			},
		},
		{
			name:    "register",
			started: func(client *cancelBarrierAccountClient) <-chan struct{} { return client.registerStarted },
			operation: func(ctx context.Context, app *AuthApplication, actor userctx.ActorContext) (AuthView, error) {
				return app.Register(ctx, actor, RegisterInput{Phone: "13800000000", Password: "test-password-01", SMSCode: "123456"})
			},
		},
		{
			name:    "refresh",
			started: func(client *cancelBarrierAccountClient) <-chan struct{} { return client.refreshStarted },
			operation: func(ctx context.Context, app *AuthApplication, actor userctx.ActorContext) (AuthView, error) {
				return app.Refresh(ctx, actor)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newDurableAccountSwitchHarness(t)
			stateFiles := []string{"binding.json", "sessions.json", "asset_snapshots.json", "event_state.json"}
			before := readDurableStateFiles(t, h.provider, h.actor, stateFiles)
			client := newCancelBarrierAccountClient(h.client)
			h.app.client = client
			attemptCtx, cancel := context.WithCancel(h.ctx)
			resultCh := make(chan authCallResult, 1)
			go func() {
				view, err := test.operation(attemptCtx, h.app, h.actor)
				resultCh <- authCallResult{view: view, err: err}
			}()
			<-test.started(client)
			cancel()
			result := <-resultCh

			if !errors.Is(result.err, context.Canceled) {
				t.Fatalf("canceled %s error = %v, want context.Canceled", test.name, result.err)
			}
			if result.view != h.primary {
				t.Fatalf("canceled %s view = %#v, want prior %#v", test.name, result.view, h.primary)
			}
			state, err := h.app.State(h.ctx, h.actor)
			if err != nil || state != h.primary || state.State == AccountAuthenticating || state.State == AccountRefreshing {
				t.Fatalf("State after canceled %s = %#v, %v; want stable %#v", test.name, state, err, h.primary)
			}
			assertDurableFilesEqual(t, h, stateFiles, before)
		})
	}
}

func TestCanceledFirstLoginRestoresAnonymousUnboundIdentity(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "fresh-canceled"}
	provider := testProfilePathProvider(t.TempDir())
	bundle := loadA0Fixture(t)
	delegate, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	bindings := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	sessionStore := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	client := newCancelBarrierAccountClient(delegate)
	app := newDurableCandidateTestAuthApplication(
		t,
		bindings,
		sessions,
		sessionStore,
		NewDurableEventStateRepository(provider),
		NewDurableAssetSnapshotRepository(provider),
		client,
	)
	attemptCtx, cancel := context.WithCancel(ctx)
	resultCh := make(chan authCallResult, 1)
	go func() {
		view, err := app.Login(attemptCtx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
		resultCh <- authCallResult{view: view, err: err}
	}()
	<-client.loginStarted
	cancel()
	result := <-resultCh
	wantBinding := unboundBinding(actor)
	if !errors.Is(result.err, context.Canceled) || result.view.State != AccountAnonymous || result.view.Binding != wantBinding {
		t.Fatalf("canceled first Login = %#v, %v; want anonymous %#v", result.view, result.err, wantBinding)
	}
	state, err := app.State(ctx, actor)
	if err != nil || state.State != AccountAnonymous || state.Binding != wantBinding {
		t.Fatalf("State after canceled first Login = %#v, %v", state, err)
	}
	current, err := bindings.ResolveBinding(ctx, actor)
	if err != nil || current != wantBinding {
		t.Fatalf("binding after canceled first Login = %#v, %v", current, err)
	}
	for _, name := range []string{"binding.json", "sessions.json"} {
		path, err := durableCloudFile(provider, actor, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s after canceled first Login: %v, want not-exist", name, err)
		}
	}
}

func TestLateLoginAfterExternalRebindSettlesCurrentNonTransientIdentity(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	client := &barrierLoginClient{
		CloudAccountClient: h.client,
		started:            make(chan struct{}),
		release:            make(chan struct{}),
	}
	h.app.client = client
	defer closeChannelIfOpen(client.release)

	loginDone := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.Login(h.ctx, h.actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
		loginDone <- authCallResult{view: view, err: err}
	}()
	<-client.started
	external := NewDurableBindingRepository(h.provider, testEnvironmentIdentity())
	externalBinding, err := external.Rebind(h.ctx, h.actor, "usr_external")
	if err != nil {
		t.Fatal(err)
	}
	close(client.release)
	late := <-loginDone

	want := AuthView{State: AccountAnonymous, Binding: externalBinding}
	if !isProfileMismatch(late.err) || late.view != want {
		t.Fatalf("late Login after external rebind = %#v, %v; want %#v + PROFILE_SCOPE_MISMATCH", late.view, late.err, want)
	}
	if cached := h.app.cachedView(h.actor, externalBinding); cached != want {
		t.Fatalf("cached view after external rebind = %#v, want %#v", cached, want)
	}
	state, err := h.app.State(h.ctx, h.actor)
	if err != nil || state != want || state.State == AccountAuthenticating || state.State == AccountRefreshing {
		t.Fatalf("State after external rebind = %#v, %v; want %#v", state, err, want)
	}
}

func TestLateRefreshAfterExternalSessionChangeSettlesCurrentNonTransientIdentity(t *testing.T) {
	for _, test := range []struct {
		name      string
		mutate    func(context.Context, *DurableCloudSessionStore, BindingScope) error
		wantState AccountStatus
		wantFound bool
		wantToken string
	}{
		{
			name: "replace",
			mutate: func(ctx context.Context, store *DurableCloudSessionStore, scope BindingScope) error {
				return store.Save(ctx, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "external-replacement"})
			},
			wantState: AccountAuthenticated,
			wantFound: true,
			wantToken: "external-replacement",
		},
		{
			name: "clear",
			mutate: func(ctx context.Context, store *DurableCloudSessionStore, scope BindingScope) error {
				return store.Clear(ctx, scope)
			},
			wantState: AccountAnonymous,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newDurableAccountSwitchHarness(t)
			scope, err := h.primary.Binding.Scope()
			if err != nil {
				t.Fatal(err)
			}
			client := &barrierRefreshClient{
				CloudAccountClient: h.client,
				started:            make(chan struct{}),
				release:            make(chan struct{}),
				returning:          make(chan struct{}),
			}
			h.app.client = client
			defer closeChannelIfOpen(client.release)
			refreshDone := make(chan authCallResult, 1)
			go func() {
				view, err := h.app.Refresh(h.ctx, h.actor)
				refreshDone <- authCallResult{view: view, err: err}
			}()
			<-client.started
			external := NewDurableCloudSessionStore(h.provider, testEnvironmentIdentity())
			if err := test.mutate(h.ctx, external, scope); err != nil {
				t.Fatal(err)
			}
			close(client.release)
			late := <-refreshDone

			if !isProfileMismatch(late.err) || late.view.State != test.wantState || late.view.Binding != h.primary.Binding || late.view.State == AccountAuthenticating || late.view.State == AccountRefreshing {
				t.Fatalf("late Refresh after external %s = %#v, %v", test.name, late.view, late.err)
			}
			if cached := h.app.cachedView(h.actor, h.primary.Binding); cached != late.view {
				t.Fatalf("cached view after external %s = %#v, want %#v", test.name, cached, late.view)
			}
			state, err := h.app.State(h.ctx, h.actor)
			if err != nil || state.State != test.wantState || state.Binding != h.primary.Binding || state.State == AccountAuthenticating || state.State == AccountRefreshing {
				t.Fatalf("State after external %s = %#v, %v", test.name, state, err)
			}
			stored, found, err := external.Load(h.ctx, scope)
			if err != nil || found != test.wantFound || (found && stored.AccessToken != test.wantToken) {
				t.Fatalf("session after external %s = %#v, found %t, err %v", test.name, stored, found, err)
			}
		})
	}
}

func TestLoginCancellationAfterLocalCommitBoundaryCannotSplitBindingAndSession(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	bindings := newConfirmBarrierBindingStore()
	sessionStore := NewMemorySessionStore()
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	app := newDurableCandidateTestAuthApplication(t, bindings, sessions, sessionStore, NewMemoryEventStateRepository(), NewMemoryAssetSnapshotRepository(), client)
	attemptCtx, cancel := context.WithCancel(ctx)
	resultCh := make(chan authCallResult, 1)
	go func() {
		view, err := app.Login(attemptCtx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
		resultCh <- authCallResult{view: view, err: err}
	}()
	<-bindings.confirmStarted
	cancel()
	close(bindings.confirmRelease)
	result := <-resultCh

	if result.err != nil || result.view.State != AccountAuthenticated || result.view.Binding.CloudUserID != "usr_01" || result.view.Binding.BindingGeneration != 1 {
		t.Fatalf("Login after local commit boundary = %#v, %v; want complete authenticated identity", result.view, result.err)
	}
	scope, err := result.view.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	stored, found, err := sessionStore.Load(ctx, scope)
	if err != nil || !found || stored.State != SessionAuthenticated || stored.Scope != scope {
		t.Fatalf("session after local commit boundary = %#v, found %t, err %v", stored, found, err)
	}
	state, err := app.State(ctx, actor)
	if err != nil || state != result.view {
		t.Fatalf("State after local commit boundary = %#v, %v; want %#v", state, err, result.view)
	}
}

func TestRefreshCancellationAfterLocalCommitBoundaryCannotAbortSessionAndView(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	barrier := newSaveBarrierSessionStore(h.sessionStore)
	h.sessions.sessions = barrier
	attemptCtx, cancel := context.WithCancel(h.ctx)
	resultCh := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.Refresh(attemptCtx, h.actor)
		resultCh <- authCallResult{view: view, err: err}
	}()
	<-barrier.saveStarted
	cancel()
	close(barrier.saveRelease)
	result := <-resultCh

	if result.err != nil || result.view != h.primary {
		t.Fatalf("Refresh after local commit boundary = %#v, %v; want stable authenticated %#v", result.view, result.err, h.primary)
	}
	stored, found, err := h.sessions.Load(h.ctx, h.actor)
	if err != nil || !found || stored.AccessToken != "fixture_access_rotated" {
		t.Fatalf("session after refresh commit boundary = %#v, found %t, err %v", stored, found, err)
	}
	state, err := h.app.State(h.ctx, h.actor)
	if err != nil || state != h.primary {
		t.Fatalf("State after refresh commit boundary = %#v, %v; want %#v", state, err, h.primary)
	}
}

func TestLogoutRemoteWaitDoesNotHoldIdentityMutationLock(t *testing.T) {
	h := newDurableAccountSwitchHarness(t)
	oldScope, err := h.primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	client := &barrierLogoutClient{
		CloudAccountClient: h.client,
		started:            make(chan struct{}),
		release:            make(chan struct{}),
	}
	h.app.client = client
	defer closeChannelIfOpen(client.release)
	logoutDone := make(chan error, 1)
	go func() { logoutDone <- h.app.Logout(h.ctx, h.actor) }()
	<-client.started

	if !h.app.identityMu.TryLock() {
		t.Error("remote Logout wait still holds identityMu")
	} else {
		h.app.identityMu.Unlock()
	}
	if _, found, err := h.sessionStore.Load(h.ctx, oldScope); err != nil || found {
		t.Errorf("local session during remote Logout = found %t, err %v; want already cleared", found, err)
	}
	wantLocal := AuthView{State: AccountAnonymous, Binding: h.primary.Binding}
	if cached := h.app.cachedView(h.actor, h.primary.Binding); cached != wantLocal {
		t.Errorf("cached view during remote Logout = %#v, want %#v", cached, wantLocal)
	}
	close(client.release)
	if err := <-logoutDone; err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestLocalCommitContextCannotExpireBetweenBindingAndSessionWrites(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	bindings := newDeadlineCrossingBindingStore()
	sessionStore := NewMemorySessionStore()
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	app := newDurableCandidateTestAuthApplication(t, bindings, sessions, sessionStore, NewMemoryEventStateRepository(), NewMemoryAssetSnapshotRepository(), client)

	view, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil || view.State != AccountAuthenticated {
		t.Fatalf("Login across local commit context = %#v, %v; want complete authenticated identity", view, err)
	}
	if bindings.sawDeadline {
		t.Fatal("multi-step local commit context exposed an internal deadline")
	}
	scope, err := view.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := sessionStore.Load(ctx, scope); err != nil || !found {
		t.Fatalf("session after local commit = found %t, err %v", found, err)
	}
}

func TestFreshnessBindingReadErrorSettlesCurrentAttemptWithoutOverwritingNewRevision(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		name := "current revision"
		if advanced {
			name = "advanced revision"
		}
		t.Run(name, func(t *testing.T) {
			h := newMemoryFreshnessHarness(t)
			var preparation *AccountSwitchPreparation
			var err error
			if advanced {
				preparation, err = h.app.PrepareAccountSwitch(h.ctx, h.actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
				if err != nil {
					t.Fatal(err)
				}
			}
			client := &barrierLoginClient{CloudAccountClient: h.client, started: make(chan struct{}), release: make(chan struct{})}
			h.app.client = client
			defer closeChannelIfOpen(client.release)
			resultCh := make(chan authCallResult, 1)
			go func() {
				view, err := h.app.Login(h.ctx, h.actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
				resultCh <- authCallResult{view: view, err: err}
			}()
			<-client.started

			want := AuthView{State: AccountOffline}
			if advanced {
				want, err = h.app.CommitAccountSwitch(h.ctx, preparation)
				if err != nil {
					t.Fatal(err)
				}
			}
			h.bindings.fail = true
			close(client.release)
			result := <-resultCh
			if !errors.Is(result.err, errFreshnessRead) || result.view != want {
				t.Fatalf("freshness binding read error = %#v, %v; want %#v", result.view, result.err, want)
			}
			if cached := h.app.stableView(h.actor); cached != want || cached.State == AccountAuthenticating || cached.State == AccountRefreshing {
				t.Fatalf("stable view after binding read error = %#v, want %#v", cached, want)
			}
			_, stateErr := h.app.State(h.ctx, h.actor)
			if !errors.Is(stateErr, errFreshnessRead) {
				t.Fatalf("State error = %v, want freshness read error", stateErr)
			}
			if cached := h.app.stableView(h.actor); cached.State == AccountAuthenticating || cached.State == AccountRefreshing {
				t.Fatalf("State left transient cache after binding read error: %#v", cached)
			}
		})
	}
}

func TestFreshnessSessionReadErrorSettlesRefreshAttempt(t *testing.T) {
	h := newMemoryFreshnessHarness(t)
	sessions := &freshnessErrorSessionStore{CloudSessionStore: h.sessionStore}
	h.sessions.sessions = sessions
	client := &barrierRefreshClient{CloudAccountClient: h.client, started: make(chan struct{}), release: make(chan struct{}), returning: make(chan struct{})}
	h.app.client = client
	defer closeChannelIfOpen(client.release)
	resultCh := make(chan authCallResult, 1)
	go func() {
		view, err := h.app.Refresh(h.ctx, h.actor)
		resultCh <- authCallResult{view: view, err: err}
	}()
	<-client.started
	sessions.fail = true
	close(client.release)
	result := <-resultCh
	want := AuthView{State: AccountOffline}
	if !errors.Is(result.err, errFreshnessRead) || result.view != want {
		t.Fatalf("freshness session read error = %#v, %v; want %#v", result.view, result.err, want)
	}
	if cached := h.app.stableView(h.actor); cached != want {
		t.Fatalf("stable view after session read error = %#v, want %#v", cached, want)
	}
	state, stateErr := h.app.State(h.ctx, h.actor)
	if !errors.Is(stateErr, errFreshnessRead) || state.State == AccountAuthenticating || state.State == AccountRefreshing {
		t.Fatalf("State after session read error = %#v, %v", state, stateErr)
	}
	if cached := h.app.stableView(h.actor); cached.State == AccountAuthenticating || cached.State == AccountRefreshing {
		t.Fatalf("State left transient cache after session read error: %#v", cached)
	}
}

type durableAccountSwitchHarness struct {
	ctx          context.Context
	actor        userctx.ActorContext
	provider     ProfilePathProvider
	bundle       FixtureBundle
	client       *FixtureAccountClient
	bindings     *DurableBindingRepository
	sessionStore *DurableCloudSessionStore
	sessions     *ProfileSessionRepository
	events       *DurableEventStateRepository
	assets       *DurableAssetSnapshotRepository
	app          *AuthApplication
	primary      AuthView
}

func newDurableAccountSwitchHarness(t *testing.T) *durableAccountSwitchHarness {
	t.Helper()
	h := &durableAccountSwitchHarness{
		ctx:      context.Background(),
		actor:    userctx.ActorContext{ActorID: "local", ProfileID: "default"},
		provider: testProfilePathProvider(t.TempDir()),
		bundle:   loadA0Fixture(t),
	}
	var err error
	h.client, err = NewFixtureAccountClient(h.bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	h.bindings = NewDurableBindingRepository(h.provider, testEnvironmentIdentity())
	h.sessionStore = NewDurableCloudSessionStore(h.provider, testEnvironmentIdentity())
	h.sessions = NewProfileSessionRepository(h.bindings, h.sessionStore)
	h.events = NewDurableEventStateRepository(h.provider)
	h.assets = NewDurableAssetSnapshotRepository(h.provider)
	h.app = newDurableCandidateTestAuthApplication(t, h.bindings, h.sessions, h.sessionStore, h.events, h.assets, h.client)
	h.primary, err = h.app.Login(h.ctx, h.actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := h.primary.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := accountSnapshotForScope(t, h.bundle, scope)
	if _, err := h.assets.StoreBalance(h.ctx, scope, snapshot.Balance); err != nil {
		t.Fatal(err)
	}
	if err := h.assets.StoreSubscription(h.ctx, scope, subscriptionView(h.bundle.PlanCases, snapshot.Subscription)); err != nil {
		t.Fatal(err)
	}
	if err := h.events.CommitApplied(h.ctx, scope, eventStateEvent("switch-current", EventBalanceUpdated, scope.CloudUserID, 1)); err != nil {
		t.Fatal(err)
	}
	return h
}

func assertDurableFilesEqual(t *testing.T, h *durableAccountSwitchHarness, names []string, before map[string][]byte) {
	t.Helper()
	after := readDurableStateFiles(t, h.provider, h.actor, names)
	for _, name := range names {
		if !bytes.Equal(after[name], before[name]) {
			t.Fatalf("durable %s changed unexpectedly", name)
		}
	}
}

type blockingSwitchScopedStore struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (s *blockingSwitchScopedStore) Clear(ctx context.Context, _ BindingScope) error {
	s.once.Do(func() { close(s.started) })
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ BindingScopedStateStore = (*blockingSwitchScopedStore)(nil)

type authCallResult struct {
	view AuthView
	err  error
}

type barrierRefreshClient struct {
	CloudAccountClient
	started   chan struct{}
	release   chan struct{}
	returning chan struct{}
}

func (c *barrierRefreshClient) RefreshSession(ctx context.Context, session Session) (Session, error) {
	close(c.started)
	<-c.release
	refreshed, err := c.CloudAccountClient.RefreshSession(ctx, session)
	close(c.returning)
	return refreshed, err
}

type barrierLoginClient struct {
	CloudAccountClient
	started chan struct{}
	release chan struct{}
	err     error
}

func (c *barrierLoginClient) Login(ctx context.Context, request AccountFixtureRequest) (AccountState, Session, error) {
	close(c.started)
	<-c.release
	if c.err != nil {
		return AccountState{}, Session{}, c.err
	}
	return c.CloudAccountClient.Login(ctx, request)
}

type barrierRegisterClient struct {
	CloudAccountClient
	started chan struct{}
	release chan struct{}
	err     error
}

func (c *barrierRegisterClient) Register(ctx context.Context, request AccountFixtureRequest) (AccountState, Session, error) {
	close(c.started)
	<-c.release
	if c.err != nil {
		return AccountState{}, Session{}, c.err
	}
	return c.CloudAccountClient.Register(ctx, request)
}

func closeChannelIfOpen(channel chan struct{}) {
	select {
	case <-channel:
	default:
		close(channel)
	}
}

type firstLoadBarrierSessionStore struct {
	CloudSessionStore
	target  BindingScope
	mu      sync.Mutex
	blocked bool
	started chan struct{}
	release chan struct{}
}

func newFirstLoadBarrierSessionStore(delegate CloudSessionStore, target BindingScope) *firstLoadBarrierSessionStore {
	return &firstLoadBarrierSessionStore{
		CloudSessionStore: delegate,
		target:            target,
		started:           make(chan struct{}),
		release:           make(chan struct{}),
	}
}

func (s *firstLoadBarrierSessionStore) Load(ctx context.Context, scope BindingScope) (Session, bool, error) {
	shouldBlock := false
	if scope == s.target {
		s.mu.Lock()
		if !s.blocked {
			s.blocked = true
			shouldBlock = true
			close(s.started)
		}
		s.mu.Unlock()
	}
	if shouldBlock {
		select {
		case <-s.release:
		case <-ctx.Done():
			return Session{}, false, ctx.Err()
		}
	}
	return s.CloudSessionStore.Load(ctx, scope)
}

type mismatchedLoadSessionStore struct {
	CloudSessionStore
	session Session
}

func (s mismatchedLoadSessionStore) Load(context.Context, BindingScope) (Session, bool, error) {
	return s.session, true, nil
}

type cancelBarrierAccountClient struct {
	CloudAccountClient
	loginStarted    chan struct{}
	registerStarted chan struct{}
	refreshStarted  chan struct{}
}

func newCancelBarrierAccountClient(delegate CloudAccountClient) *cancelBarrierAccountClient {
	return &cancelBarrierAccountClient{
		CloudAccountClient: delegate,
		loginStarted:       make(chan struct{}),
		registerStarted:    make(chan struct{}),
		refreshStarted:     make(chan struct{}),
	}
}

func (c *cancelBarrierAccountClient) Login(ctx context.Context, _ AccountFixtureRequest) (AccountState, Session, error) {
	close(c.loginStarted)
	<-ctx.Done()
	return AccountState{}, Session{}, ctx.Err()
}

func (c *cancelBarrierAccountClient) Register(ctx context.Context, _ AccountFixtureRequest) (AccountState, Session, error) {
	close(c.registerStarted)
	<-ctx.Done()
	return AccountState{}, Session{}, ctx.Err()
}

func (c *cancelBarrierAccountClient) RefreshSession(ctx context.Context, _ Session) (Session, error) {
	close(c.refreshStarted)
	<-ctx.Done()
	return Session{}, ctx.Err()
}

type confirmBarrierBindingStore struct {
	delegate       *BindingRepository
	confirmStarted chan struct{}
	confirmRelease chan struct{}
}

func newConfirmBarrierBindingStore() *confirmBarrierBindingStore {
	return &confirmBarrierBindingStore{
		delegate:       NewBindingRepository(),
		confirmStarted: make(chan struct{}),
		confirmRelease: make(chan struct{}),
	}
}

func (s *confirmBarrierBindingStore) ResolveBinding(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, error) {
	return s.delegate.ResolveBinding(ctx, actor)
}

func (s *confirmBarrierBindingStore) ConfirmBinding(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	close(s.confirmStarted)
	<-s.confirmRelease
	return s.delegate.ConfirmBinding(context.WithoutCancel(ctx), actor, cloudUserID)
}

func (s *confirmBarrierBindingStore) Rebind(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	return s.delegate.Rebind(ctx, actor, cloudUserID)
}

func (s *confirmBarrierBindingStore) CompareAndRebind(ctx context.Context, actor userctx.ActorContext, expected ProfileBinding, cloudUserID string) (ProfileBinding, error) {
	return s.delegate.CompareAndRebind(ctx, actor, expected, cloudUserID)
}

type saveBarrierSessionStore struct {
	CloudSessionStore
	saveStarted chan struct{}
	saveRelease chan struct{}
}

func newSaveBarrierSessionStore(delegate CloudSessionStore) *saveBarrierSessionStore {
	return &saveBarrierSessionStore{
		CloudSessionStore: delegate,
		saveStarted:       make(chan struct{}),
		saveRelease:       make(chan struct{}),
	}
}

func (s *saveBarrierSessionStore) Save(ctx context.Context, session Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	close(s.saveStarted)
	<-s.saveRelease
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.CloudSessionStore.Save(ctx, session)
}

type barrierLogoutClient struct {
	CloudAccountClient
	started chan struct{}
	release chan struct{}
}

func (c *barrierLogoutClient) Logout(context.Context, BindingScope) error {
	close(c.started)
	<-c.release
	return nil
}

var _ BindingStore = (*confirmBarrierBindingStore)(nil)
var _ CloudSessionStore = (*saveBarrierSessionStore)(nil)

type deadlineCrossingBindingStore struct {
	delegate    *BindingRepository
	sawDeadline bool
}

func newDeadlineCrossingBindingStore() *deadlineCrossingBindingStore {
	return &deadlineCrossingBindingStore{delegate: NewBindingRepository()}
}

func (s *deadlineCrossingBindingStore) ResolveBinding(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, error) {
	return s.delegate.ResolveBinding(ctx, actor)
}

func (s *deadlineCrossingBindingStore) ConfirmBinding(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	if _, ok := ctx.Deadline(); ok {
		s.sawDeadline = true
		<-ctx.Done()
	}
	return s.delegate.ConfirmBinding(context.WithoutCancel(ctx), actor, cloudUserID)
}

func (s *deadlineCrossingBindingStore) Rebind(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	return s.delegate.Rebind(ctx, actor, cloudUserID)
}

func (s *deadlineCrossingBindingStore) CompareAndRebind(ctx context.Context, actor userctx.ActorContext, expected ProfileBinding, cloudUserID string) (ProfileBinding, error) {
	return s.delegate.CompareAndRebind(ctx, actor, expected, cloudUserID)
}

var errFreshnessRead = errors.New("freshness identity read failed")

type freshnessErrorBindingStore struct {
	delegate *BindingRepository
	fail     bool
}

func (s *freshnessErrorBindingStore) ResolveBinding(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, error) {
	if s.fail {
		return ProfileBinding{}, errFreshnessRead
	}
	return s.delegate.ResolveBinding(ctx, actor)
}

func (s *freshnessErrorBindingStore) ConfirmBinding(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	return s.delegate.ConfirmBinding(ctx, actor, cloudUserID)
}

func (s *freshnessErrorBindingStore) Rebind(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	return s.delegate.Rebind(ctx, actor, cloudUserID)
}

func (s *freshnessErrorBindingStore) CompareAndRebind(ctx context.Context, actor userctx.ActorContext, expected ProfileBinding, cloudUserID string) (ProfileBinding, error) {
	return s.delegate.CompareAndRebind(ctx, actor, expected, cloudUserID)
}

type freshnessErrorSessionStore struct {
	CloudSessionStore
	fail bool
}

func (s *freshnessErrorSessionStore) Load(ctx context.Context, scope BindingScope) (Session, bool, error) {
	if s.fail {
		return Session{}, false, errFreshnessRead
	}
	return s.CloudSessionStore.Load(ctx, scope)
}

type memoryFreshnessHarness struct {
	ctx          context.Context
	actor        userctx.ActorContext
	client       *FixtureAccountClient
	bindings     *freshnessErrorBindingStore
	sessionStore *MemorySessionStore
	sessions     *ProfileSessionRepository
	app          *AuthApplication
}

func newMemoryFreshnessHarness(t *testing.T) *memoryFreshnessHarness {
	t.Helper()
	h := &memoryFreshnessHarness{
		ctx:      context.Background(),
		actor:    userctx.ActorContext{ActorID: "local", ProfileID: "default"},
		bindings: &freshnessErrorBindingStore{delegate: NewBindingRepository()},
	}
	bundle := loadA0Fixture(t)
	var err error
	h.client, err = NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	h.sessionStore = NewMemorySessionStore()
	h.sessions = NewProfileSessionRepository(h.bindings, h.sessionStore)
	h.app = newDurableCandidateTestAuthApplication(t, h.bindings, h.sessions, h.sessionStore, NewMemoryEventStateRepository(), NewMemoryAssetSnapshotRepository(), h.client)
	if _, err := h.app.Login(h.ctx, h.actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	return h
}

var _ BindingStore = (*deadlineCrossingBindingStore)(nil)
var _ BindingStore = (*freshnessErrorBindingStore)(nil)
var _ CloudSessionStore = (*freshnessErrorSessionStore)(nil)
