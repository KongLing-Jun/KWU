package cloud

import (
	"context"
	"errors"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestAuthApplicationCaptchaGateAndRegisterStoresOnlyProfileSession(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())

	if err := app.SendRegisterSMS(ctx, actor, false, "13800000000"); !errors.Is(err, ErrCaptchaVerificationRequired) {
		t.Fatalf("SendRegisterSMS without captcha error = %v", err)
	}
	if got := client.SendRegisterSMSCalls(); got != 0 {
		t.Fatalf("SMS calls = %d, want 0", got)
	}
	if err := app.SendRegisterSMS(ctx, actor, true, "13800000000"); err != nil {
		t.Fatal(err)
	}
	if got := client.SendRegisterSMSCalls(); got != 1 {
		t.Fatalf("SMS calls = %d, want 1", got)
	}

	view, err := app.Register(ctx, actor, RegisterInput{Phone: "13800000000", Password: "temporary password", SMSCode: "123456"})
	if err != nil {
		t.Fatal(err)
	}
	if view.State != AccountAuthenticated || view.Binding.CloudUserID != "usr_01" {
		t.Fatalf("register view = %#v", view)
	}
	if _, ok := any(view).(interface{ RefreshToken() string }); ok {
		t.Fatalf("UI view exposed refresh token")
	}
	session, ok, err := app.sessions.Load(ctx, actor)
	if err != nil || !ok || session.RefreshToken == "" {
		t.Fatalf("stored session = %#v, %v, %v", session, ok, err)
	}
	if got := client.RegisterCalls(); got != 1 {
		t.Fatalf("register calls = %d, want 1", got)
	}
}

func TestAuthApplicationLoginUsesPhoneAndPasswordFields(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Login: "auth_success"})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())
	view, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil || view.State != AccountAuthenticated || view.Binding.CloudUserID != "usr_01" {
		t.Fatalf("Login = %#v, %v", view, err)
	}
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000"}); err == nil {
		t.Fatal("Login accepted an empty password")
	}
}

func TestAuthApplicationFreshSecondaryLoginUsesRepositoryGeneration(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "secondary-profile"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())

	view, err := app.Login(ctx, actor, LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatalf("secondary Login: %v", err)
	}
	wantBinding := ProfileBinding{
		ActorID:           actor.ActorID,
		ProfileID:         actor.ProfileID,
		CloudUserID:       "usr_02",
		Status:            BindingBound,
		BindingGeneration: 1,
	}
	if view.State != AccountAuthenticated || view.Binding != wantBinding || view.User.CloudUserID != wantBinding.CloudUserID {
		t.Fatalf("secondary Login view = %#v, want authenticated %#v", view, wantBinding)
	}
	stored, ok, err := app.sessions.Load(ctx, actor)
	if err != nil || !ok {
		t.Fatalf("stored secondary session = %#v, %t, %v", stored, ok, err)
	}
	wantScope, err := wantBinding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	if stored.Scope != wantScope {
		t.Fatalf("stored secondary session scope = %#v, want %#v", stored.Scope, wantScope)
	}
}

func TestAuthApplicationRejectsExpiredCaptchaAndProductionEncryptionBeforeClient(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewPasswordEncryptor())
	expired, err := NewMockCaptchaGateway("captcha_ticket_01", time.Now().Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	app.captcha = expired
	if err := app.SendRegisterSMS(ctx, actor, true, "13800000000"); !errors.Is(err, ErrCaptchaPermitExpired) {
		t.Fatalf("expired captcha error = %v", err)
	}
	if got := client.SendRegisterSMSCalls(); got != 0 {
		t.Fatalf("expired captcha SMS calls = %d, want 0", got)
	}
	if _, err := app.Register(ctx, actor, RegisterInput{Phone: "13800000000", Password: "temporary password", SMSCode: "123456"}); !errors.Is(err, ErrPasswordEncryptionNotReady) {
		t.Fatalf("production encryption error = %v", err)
	}
	if got := client.RegisterCalls(); got != 0 {
		t.Fatalf("unencrypted register calls = %d, want 0", got)
	}
}

func TestAuthApplicationRefreshRecoveryLogoutAndMismatchIsolation(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Refresh: "refresh_success"})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	view, err := app.Refresh(ctx, actor)
	if err != nil || view.State != AccountAuthenticated {
		t.Fatalf("Refresh = %#v, %v", view, err)
	}
	stored, ok, err := app.sessions.Load(ctx, actor)
	if err != nil || !ok || stored.AccessToken != "fixture_access_rotated" {
		t.Fatalf("rotated session = %#v, %v, %v", stored, ok, err)
	}
	if err := app.Logout(ctx, actor); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := app.sessions.Load(ctx, actor); err != nil || ok {
		t.Fatalf("logout session present=%v err=%v", ok, err)
	}
	if view, err = app.Recover(ctx, actor); err != nil || view.State != AccountAnonymous {
		t.Fatalf("recovery after logout = %#v, %v", view, err)
	}

	mismatchClient, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Login: "profile_mismatch"})
	if err != nil {
		t.Fatal(err)
	}
	mismatch := newAuthApplication(t, mismatchClient, NewTestPasswordEncryptor())
	if _, err := mismatch.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); !isProfileMismatch(err) {
		t.Fatalf("mismatch login error = %v", err)
	}
	if _, ok, err := mismatch.sessions.Load(ctx, actor); ok || !isProfileMismatch(err) {
		t.Fatalf("mismatch stored a session: %v, %v", ok, err)
	}
}

func TestAuthApplicationDoesNotRecoverAnOldBindingGeneration(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.bindings.Rebind(ctx, actor, "usr_02"); err != nil {
		t.Fatal(err)
	}
	view, err := app.Recover(ctx, actor)
	if err != nil || view.State != AccountAnonymous {
		t.Fatalf("recovery after rebind = %#v, %v", view, err)
	}
}

func TestAuthApplicationLogoutClearsCurrentScopeWhenRemoteLogoutFails(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	delegate, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	remoteErr := errors.New("fixture remote logout failed")
	app := newAuthApplication(t, logoutErrorClient{CloudAccountClient: delegate, err: remoteErr}, NewTestPasswordEncryptor())
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Logout(ctx, actor); !errors.Is(err, remoteErr) {
		t.Fatalf("Logout error = %v, want remote error", err)
	}
	if _, ok, err := app.sessions.Load(ctx, actor); err != nil || ok {
		t.Fatalf("session after failed remote logout: ok=%v err=%v", ok, err)
	}
	view, err := app.State(ctx, actor)
	if err != nil || view.State != AccountAnonymous {
		t.Fatalf("view after failed remote logout = %#v, %v", view, err)
	}
}

func TestAuthApplicationRefreshFailureOfflineAndSyncDoNotChangeAuthenticationFact(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Refresh: "refresh_failed"})
	if err != nil {
		t.Fatal(err)
	}
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	if view, err := app.Refresh(ctx, actor); !errors.Is(err, errFixtureAuth) || view.State != AccountExpired {
		t.Fatalf("failed refresh = %#v, %v", view, err)
	}

	offline := newAuthApplication(t, fixtureErrorClient{err: &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}}, NewTestPasswordEncryptor())
	if _, err := offline.bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope, _ := NewBindingScope(actor, "usr_01", 1)
	if err := offline.sessions.Save(ctx, actor, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if view, err := offline.Refresh(ctx, actor); !isNetworkUnavailable(err) || view.State != AccountOffline {
		t.Fatalf("offline refresh = %#v, %v", view, err)
	}

	for _, sync := range bundle.SyncCases {
		if err := app.ApplyThirdPartySync(ctx, actor, sync); err != nil {
			t.Fatalf("ApplyThirdPartySync(%q): %v", sync.Status, err)
		}
		view, err := app.State(ctx, actor)
		if err != nil || view.State != AccountExpired {
			t.Fatalf("sync %q changed auth state to %#v, %v", sync.Status, view, err)
		}
	}
}

func TestAuthApplicationExposesAuthenticatingAndRefreshingOnlyWhileOperationsRun(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bundle := loadA0Fixture(t)
	delegate, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	client := newBlockingAccountClient(delegate)
	app := newAuthApplication(t, client, NewTestPasswordEncryptor())
	loginDone := make(chan error, 1)
	go func() {
		_, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
		loginDone <- err
	}()
	<-client.loginStarted
	if view, err := app.State(ctx, actor); err != nil || view.State != AccountAuthenticating {
		t.Fatalf("during login state = %#v, %v", view, err)
	}
	close(client.loginRelease)
	if err := <-loginDone; err != nil {
		t.Fatal(err)
	}

	refreshDone := make(chan error, 1)
	go func() { _, err := app.Refresh(ctx, actor); refreshDone <- err }()
	<-client.refreshStarted
	if view, err := app.State(ctx, actor); err != nil || view.State != AccountRefreshing {
		t.Fatalf("during refresh state = %#v, %v", view, err)
	}
	close(client.refreshRelease)
	if err := <-refreshDone; err != nil {
		t.Fatal(err)
	}
}

func newAuthApplication(t *testing.T, client CloudAccountClient, encryptor PasswordEncryptor) *AuthApplication {
	t.Helper()
	bindings := NewBindingRepository()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	cleanup := NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{})
	captcha, err := NewMockCaptchaGateway("captcha_ticket_01", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewAuthApplication(AuthApplicationDependencies{Bindings: bindings, Sessions: sessions, Cleanup: cleanup, Captcha: captcha, Encryptor: encryptor, Client: client, FixtureVersion: A0FixtureVersion})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

type noopScopedStore struct{}

func (noopScopedStore) Clear(context.Context, BindingScope) error { return nil }

type fixtureErrorClient struct{ err error }

func (c fixtureErrorClient) SendRegisterSMS(context.Context, AccountFixtureRequest) error {
	return c.err
}
func (c fixtureErrorClient) Register(context.Context, AccountFixtureRequest) (AccountState, Session, error) {
	return AccountState{}, Session{}, c.err
}
func (c fixtureErrorClient) Login(context.Context, AccountFixtureRequest) (AccountState, Session, error) {
	return AccountState{}, Session{}, c.err
}
func (c fixtureErrorClient) RefreshSession(context.Context, Session) (Session, error) {
	return Session{}, c.err
}
func (c fixtureErrorClient) Logout(context.Context, BindingScope) error { return c.err }
func (c fixtureErrorClient) GetCurrentUser(context.Context, BindingScope) (AccountState, error) {
	return AccountState{}, c.err
}

type logoutErrorClient struct {
	CloudAccountClient
	err error
}

func (c logoutErrorClient) Logout(context.Context, BindingScope) error { return c.err }

type blockingAccountClient struct {
	CloudAccountClient
	loginStarted, loginRelease     chan struct{}
	refreshStarted, refreshRelease chan struct{}
}

func newBlockingAccountClient(client CloudAccountClient) *blockingAccountClient {
	return &blockingAccountClient{CloudAccountClient: client, loginStarted: make(chan struct{}), loginRelease: make(chan struct{}), refreshStarted: make(chan struct{}), refreshRelease: make(chan struct{})}
}
func (c *blockingAccountClient) Login(ctx context.Context, request AccountFixtureRequest) (AccountState, Session, error) {
	close(c.loginStarted)
	<-c.loginRelease
	return c.CloudAccountClient.Login(ctx, request)
}
func (c *blockingAccountClient) RefreshSession(ctx context.Context, session Session) (Session, error) {
	close(c.refreshStarted)
	<-c.refreshRelease
	return c.CloudAccountClient.RefreshSession(ctx, session)
}
