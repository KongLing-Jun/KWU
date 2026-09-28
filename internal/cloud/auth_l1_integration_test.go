package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestL1AuthCandidateAccountLeavesCurrentProfileBindingAndSessionUnchanged(t *testing.T) {
	ctx := context.Background()
	system := newL1AuthSystem(t)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}

	primary, _ := system.app(t, FixtureAccountScenarios{Login: "auth_success"})
	primaryView, err := primary.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatalf("primary Login: %v", err)
	}
	if got, want := primaryView.Binding, fixtureBinding(t, system.bundle, "auth_success"); got != want {
		t.Fatalf("primary binding = %#v, want %#v", got, want)
	}
	beforeSession, ok, err := system.sessions.Load(ctx, actor)
	if err != nil || !ok {
		t.Fatalf("primary session before candidate: ok=%v err=%v", ok, err)
	}

	candidate, _ := system.app(t, FixtureAccountScenarios{})
	if _, err := candidate.Login(ctx, actor, LoginInput{Phone: "13900000000", Password: "test-password-02"}); !isProfileMismatch(err) {
		t.Fatalf("candidate Login error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	afterBinding, err := system.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if afterBinding != primaryView.Binding {
		t.Fatalf("candidate Login changed binding = %#v, want %#v", afterBinding, primaryView.Binding)
	}
	afterSession, ok, err := system.sessions.Load(ctx, actor)
	if err != nil || !ok || afterSession != beforeSession {
		t.Fatalf("candidate Login changed session = %#v, ok=%v err=%v, want %#v", afterSession, ok, err, beforeSession)
	}
}

func TestL1AuthFixtureRefreshLogoutRecoveryAndRetry(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}

	t.Run("refresh succeeds and logout restart is anonymous", func(t *testing.T) {
		system := newL1AuthSystem(t)
		app, _ := system.app(t, FixtureAccountScenarios{Login: "auth_success", Refresh: "refresh_success"})
		if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
			t.Fatal(err)
		}
		refreshed, err := app.Refresh(ctx, actor)
		if err != nil || refreshed.State != AccountAuthenticated {
			t.Fatalf("Refresh = %#v, %v", refreshed, err)
		}
		if err := app.Logout(ctx, actor); err != nil {
			t.Fatal(err)
		}
		restarted, _ := system.app(t, FixtureAccountScenarios{})
		if recovered, err := restarted.Recover(ctx, actor); err != nil || recovered.State != AccountAnonymous {
			t.Fatalf("restart after logout = %#v, %v", recovered, err)
		}
	})

	t.Run("fixture failure expires then a retry recovers", func(t *testing.T) {
		system := newL1AuthSystem(t)
		app, _ := system.app(t, FixtureAccountScenarios{Login: "auth_success", Refresh: "refresh_failed"})
		if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
			t.Fatal(err)
		}
		if view, err := app.Refresh(ctx, actor); !errors.Is(err, errFixtureAuth) || view.State != AccountExpired {
			t.Fatalf("failed fixture refresh = %#v, %v", view, err)
		}
		retryClient, err := NewFixtureAccountClient(system.bundle, FixtureAccountScenarios{Refresh: "refresh_success"})
		if err != nil {
			t.Fatal(err)
		}
		app.client = retryClient
		if view, err := app.Refresh(ctx, actor); err != nil || view.State != AccountAuthenticated {
			t.Fatalf("retry refresh = %#v, %v", view, err)
		}
	})

	t.Run("network failure remains offline until retry", func(t *testing.T) {
		system := newL1AuthSystem(t)
		app, _ := system.app(t, FixtureAccountScenarios{Login: "auth_success"})
		if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
			t.Fatal(err)
		}
		app.client = l1FixtureErrorClient{err: &Error{Code: ErrorNetworkUnavailable, Message: "fixture offline", Retryable: true}}
		if view, err := app.Refresh(ctx, actor); !isNetworkUnavailable(err) || view.State != AccountOffline {
			t.Fatalf("offline refresh = %#v, %v", view, err)
		}
		retryClient, err := NewFixtureAccountClient(system.bundle, FixtureAccountScenarios{Refresh: "refresh_success"})
		if err != nil {
			t.Fatal(err)
		}
		app.client = retryClient
		if view, err := app.Refresh(ctx, actor); err != nil || view.State != AccountAuthenticated {
			t.Fatalf("offline retry = %#v, %v", view, err)
		}
	})
}

func TestL1AuthCaptchaSMSAndFixtureSMSCodeError(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}

	t.Run("unverified and expired captcha never call the fake SMS client", func(t *testing.T) {
		system := newL1AuthSystem(t)
		app, client := system.app(t, FixtureAccountScenarios{})
		if err := app.SendRegisterSMS(ctx, actor, false, "13800000000"); !errors.Is(err, ErrCaptchaVerificationRequired) {
			t.Fatalf("unverified captcha error = %v", err)
		}
		if got := client.SendRegisterSMSCalls(); got != 0 {
			t.Fatalf("SMS calls after unverified captcha = %d, want 0", got)
		}
		expired, err := NewMockCaptchaGateway("captcha_ticket_01", time.Now().Add(-time.Second))
		if err != nil {
			t.Fatal(err)
		}
		app.captcha = expired
		if err := app.SendRegisterSMS(ctx, actor, true, "13800000000"); !errors.Is(err, ErrCaptchaPermitExpired) {
			t.Fatalf("expired captcha error = %v", err)
		}
		if got := client.SendRegisterSMSCalls(); got != 0 {
			t.Fatalf("SMS calls after expired captcha = %d, want 0", got)
		}
	})

	t.Run("verified captcha calls fake SMS and fixture owns SMS-code validity", func(t *testing.T) {
		system := newL1AuthSystem(t)
		app, client := system.app(t, FixtureAccountScenarios{Register: "sms_code_invalid"})
		if err := app.SendRegisterSMS(ctx, actor, true, "13800000000"); err != nil {
			t.Fatalf("verified SendRegisterSMS: %v", err)
		}
		if got := client.SendRegisterSMSCalls(); got != 1 {
			t.Fatalf("SMS calls after verified captcha = %d, want 1", got)
		}
		_, err := app.Register(ctx, actor, RegisterInput{Phone: "13800000000", Password: "temporary password", SMSCode: "locally-unverified"})
		var cloudErr *Error
		if !errors.As(err, &cloudErr) || cloudErr.Code != ErrorSMSCodeInvalid {
			t.Fatalf("Register error = %v, want fixture SMS_CODE_INVALID", err)
		}
	})
}

func TestL1AuthAccountViewIsCredentialFreeAndSyncNeverChangesAuthentication(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	system := newL1AuthSystem(t)
	app, _ := system.app(t, FixtureAccountScenarios{Login: "auth_success"})
	auth, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	accountJSON, err := json.Marshal(AccountViewFromAuth(auth))
	if err != nil {
		t.Fatal(err)
	}
	assertL1NoCredentialSurface(t, reflect.TypeOf(AccountView{}), accountJSON)
	for _, sync := range system.bundle.SyncCases {
		if err := app.ApplyThirdPartySync(ctx, actor, sync); err != nil {
			t.Fatalf("ApplyThirdPartySync(%q): %v", sync.Status, err)
		}
		if view, err := app.State(ctx, actor); err != nil || view.State != AccountAuthenticated {
			t.Fatalf("sync %q changed successful authentication: %#v, %v", sync.Status, view, err)
		}
	}
}

type l1AuthSystem struct {
	bundle   FixtureBundle
	bindings *BindingRepository
	store    *MemorySessionStore
	sessions *ProfileSessionRepository
	cleanup  *IdentityCleanup
}

func newL1AuthSystem(t *testing.T) *l1AuthSystem {
	t.Helper()
	bundle := loadA0Fixture(t)
	bindings := NewBindingRepository()
	store := NewMemorySessionStore()
	return &l1AuthSystem{
		bundle: bundle, bindings: bindings, store: store,
		sessions: NewProfileSessionRepository(bindings, store),
		cleanup:  NewIdentityCleanup(store, l1NoopScopedStore{}, l1NoopScopedStore{}),
	}
}

func (s *l1AuthSystem) app(t *testing.T, scenarios FixtureAccountScenarios) (*AuthApplication, *FixtureAccountClient) {
	t.Helper()
	client, err := NewFixtureAccountClient(s.bundle, scenarios)
	if err != nil {
		t.Fatal(err)
	}
	captcha, err := NewMockCaptchaGateway("captcha_ticket_01", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: s.bindings, Sessions: s.sessions, Cleanup: s.cleanup, Captcha: captcha,
		Encryptor: NewTestPasswordEncryptor(), Client: client, FixtureVersion: A0FixtureVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	return app, client
}

type l1NoopScopedStore struct{}

func (l1NoopScopedStore) Clear(context.Context, BindingScope) error { return nil }

type l1FixtureErrorClient struct{ err error }

func (c l1FixtureErrorClient) SendRegisterSMS(context.Context, AccountFixtureRequest) error {
	return c.err
}
func (c l1FixtureErrorClient) Register(context.Context, AccountFixtureRequest) (AccountState, Session, error) {
	return AccountState{}, Session{}, c.err
}
func (c l1FixtureErrorClient) Login(context.Context, AccountFixtureRequest) (AccountState, Session, error) {
	return AccountState{}, Session{}, c.err
}
func (c l1FixtureErrorClient) RefreshSession(context.Context, Session) (Session, error) {
	return Session{}, c.err
}
func (c l1FixtureErrorClient) Logout(context.Context, BindingScope) error { return c.err }
func (c l1FixtureErrorClient) GetCurrentUser(context.Context, BindingScope) (AccountState, error) {
	return AccountState{}, c.err
}

func fixtureBinding(t *testing.T, bundle FixtureBundle, name string) ProfileBinding {
	t.Helper()
	for _, fixture := range bundle.AuthCases {
		if fixture.Name == name {
			return fixture.Binding
		}
	}
	t.Fatalf("missing frozen auth fixture %q", name)
	return ProfileBinding{}
}

func assertL1NoCredentialSurface(t *testing.T, typ reflect.Type, payload []byte) {
	t.Helper()
	for _, forbidden := range []string{"access_token", "refresh_token", "password", "encrypted_password"} {
		if strings.Contains(string(payload), `"`+forbidden+`"`) {
			t.Fatalf("UI JSON exposed %q: %s", forbidden, payload)
		}
	}
	assertL1NoCredentialFields(t, typ)
}

func assertL1NoCredentialFields(t *testing.T, typ reflect.Type) {
	t.Helper()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return
	}
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "" {
			jsonName = field.Name
		}
		switch jsonName {
		case "access_token", "refresh_token", "password", "encrypted_password":
			t.Fatalf("UI DTO exposes %q", jsonName)
		}
		assertL1NoCredentialFields(t, field.Type)
	}
}
