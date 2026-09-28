package cloud

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestAuthApplicationRealRegistrationSendsSMSWithDevelopmentPermit(t *testing.T) {
	now := time.Now().UTC()
	client := &recordingRegistrationClient{smsResult: RegistrationSMSResult{Phone: "+8613900000002", ExpiresInSeconds: 300}}
	app := newRemoteRegistrationApplication(t, &recordingSessionClient{}, client, now)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if err := app.SendRegisterSMS(context.Background(), actor, true, "+8613900000002"); err != nil {
		t.Fatal(err)
	}
	if client.smsRequest.Phone != "+8613900000002" || !strings.HasPrefix(client.smsRequest.CaptchaTicket, "dev-captcha-") {
		t.Fatalf("SMS request = %#v", client.smsRequest)
	}
	if err := app.SendRegisterSMS(context.Background(), actor, false, "+8613900000002"); !errors.Is(err, ErrCaptchaVerificationRequired) {
		t.Fatalf("unverified SMS error = %v", err)
	}
	if client.smsCalls != 1 {
		t.Fatalf("SMS transport calls = %d", client.smsCalls)
	}
}

func TestAuthApplicationRealRegistrationPersistsBoundSession(t *testing.T) {
	now := time.Now().UTC()
	client := &recordingRegistrationClient{
		crypto: validRemoteRegistrationCrypto(now),
		result: validRemoteRegistrationResult(now, "usr_registered"),
	}
	app := newRemoteRegistrationApplication(t, &recordingSessionClient{}, client, now)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	view, err := app.Register(context.Background(), actor, RegisterInput{
		Phone: "+8613900000002", Password: "ReasonixDev#2026", SMSCode: "123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.State != AccountAuthenticated || view.Binding.CloudUserID != "usr_registered" || view.User.CloudUserID != "usr_registered" {
		t.Fatalf("registration view = %#v", view)
	}
	if !reflect.DeepEqual(client.calls, []string{"password_crypto", "register"}) {
		t.Fatalf("registration calls = %v", client.calls)
	}
	if client.registerRequest.Phone != "+8613900000002" || client.registerRequest.SMSCode != "123456" || client.registerRequest.EncryptedPassword == "ReasonixDev#2026" {
		t.Fatalf("registration request = %#v", client.registerRequest)
	}
	if got := decryptRegistrationPassword(t, client.crypto, client.registerRequest.EncryptedPassword); got != "ReasonixDev#2026" {
		t.Fatalf("decrypted password = %q", got)
	}
	stored, ok, err := app.sessions.Load(context.Background(), actor)
	if err != nil || !ok {
		t.Fatalf("stored session = %#v, %t, %v", stored, ok, err)
	}
	if stored.AccessToken != "access-registered" || stored.RefreshToken != "refresh-registered" || stored.Scope.CloudUserID != "usr_registered" {
		t.Fatalf("stored registration session = %#v", stored)
	}
}

func TestAuthApplicationRealRegistrationPersistsSessionBeforeBinding(t *testing.T) {
	now := time.Now().UTC()
	var calls []string
	baseBindings := NewBindingRepository()
	bindings := &orderedRegistrationBindingStore{BindingStore: baseBindings, calls: &calls}
	baseSessions := NewMemorySessionStore()
	sessionsStore := &orderedRegistrationSessionStore{CloudSessionStore: baseSessions, calls: &calls}
	sessions := NewProfileSessionRepository(bindings, sessionsStore)
	cleanup := NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{})
	random := bytes.NewReader(bytes.Repeat([]byte{0x5a}, 256))
	app, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions, Cleanup: cleanup, SessionClient: &recordingSessionClient{},
		RegistrationClient: &recordingRegistrationClient{
			crypto: validRemoteRegistrationCrypto(now),
			result: validRemoteRegistrationResult(now, "usr_registered"),
		},
		RegistrationEncryptor: NewRegistrationPasswordEncryptor(random, func() time.Time { return now }),
		Captcha:               NewDevelopmentCaptchaGateway(random, func() time.Time { return now }),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Register(context.Background(), userctx.ActorContext{ActorID: "local", ProfileID: "default"}, RegisterInput{
		Phone: "+8613900000002", Password: "ReasonixDev#2026", SMSCode: "123456",
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"session", "binding"}) {
		t.Fatalf("identity persistence order = %v", calls)
	}
}

func TestAuthApplicationRealRegistrationSessionFailureDoesNotBindProfile(t *testing.T) {
	now := time.Now().UTC()
	bindings := NewBindingRepository()
	sessionsStore := &failingRegistrationSessionStore{CloudSessionStore: NewMemorySessionStore(), saveErr: errors.New("session unavailable")}
	sessions := NewProfileSessionRepository(bindings, sessionsStore)
	cleanup := NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{})
	random := bytes.NewReader(bytes.Repeat([]byte{0x5a}, 256))
	app, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions, Cleanup: cleanup, SessionClient: &recordingSessionClient{},
		RegistrationClient: &recordingRegistrationClient{
			crypto: validRemoteRegistrationCrypto(now),
			result: validRemoteRegistrationResult(now, "usr_registered"),
		},
		RegistrationEncryptor: NewRegistrationPasswordEncryptor(random, func() time.Time { return now }),
		Captcha:               NewDevelopmentCaptchaGateway(random, func() time.Time { return now }),
	})
	if err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := app.Register(context.Background(), actor, RegisterInput{
		Phone: "+8613900000002", Password: "ReasonixDev#2026", SMSCode: "123456",
	}); !errors.Is(err, sessionsStore.saveErr) {
		t.Fatalf("registration error = %v", err)
	}
	binding, err := bindings.ResolveBinding(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Status != BindingUnbound {
		t.Fatalf("binding after session failure = %#v", binding)
	}
}

func TestAuthApplicationRealRegistrationFailurePreservesAuthenticatedIdentity(t *testing.T) {
	now := time.Now().UTC()
	sessionClient := &recordingSessionClient{login: AuthenticationResult{
		User: CloudUser{CloudUserID: "usr_existing", DisplayName: "Existing"},
		Session: RemoteSession{
			AccessToken: "access-existing", AccessTokenExpiresAt: now.Add(time.Hour),
			RefreshToken: "refresh-existing", RefreshTokenExpiresAt: now.Add(24 * time.Hour), DeviceID: "device-a12",
		},
		ServerTime: now,
	}}
	registrationClient := &recordingRegistrationClient{
		crypto: validRemoteRegistrationCrypto(now), registerErr: errors.New("registration unavailable"),
	}
	app := newRemoteRegistrationApplication(t, sessionClient, registrationClient, now)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := app.Login(context.Background(), actor, LoginInput{Phone: "+8613900000001", Password: "existing-password", CaptchaPassed: true}); err != nil {
		t.Fatal(err)
	}
	view, err := app.Register(context.Background(), actor, RegisterInput{Phone: "+8613900000002", Password: "ReasonixDev#2026", SMSCode: "123456"})
	if err == nil {
		t.Fatal("expected registration error")
	}
	if view.State != AccountAuthenticated || view.Binding.CloudUserID != "usr_existing" {
		t.Fatalf("failure view = %#v", view)
	}
	current, stateErr := app.State(context.Background(), actor)
	if stateErr != nil || current.State != AccountAuthenticated || current.Binding.CloudUserID != "usr_existing" {
		t.Fatalf("current identity = %#v, %v", current, stateErr)
	}
	stored, ok, loadErr := app.sessions.Load(context.Background(), actor)
	if loadErr != nil || !ok || stored.RefreshToken != "refresh-existing" {
		t.Fatalf("existing session = %#v, %t, %v", stored, ok, loadErr)
	}
}

func TestAuthApplicationRealRegistrationConflictKeepsCurrentIdentityAndClearsCandidateSession(t *testing.T) {
	now := time.Now().UTC()
	sessionClient := &recordingSessionClient{login: AuthenticationResult{
		User: CloudUser{CloudUserID: "usr_existing", DisplayName: "Existing"},
		Session: RemoteSession{
			AccessToken: "access-existing", AccessTokenExpiresAt: now.Add(time.Hour),
			RefreshToken: "refresh-existing", RefreshTokenExpiresAt: now.Add(24 * time.Hour), DeviceID: "device-a12",
		},
		ServerTime: now,
	}}
	registrationClient := &recordingRegistrationClient{
		crypto: validRemoteRegistrationCrypto(now), result: validRemoteRegistrationResult(now, "usr_candidate"),
	}
	app := newRemoteRegistrationApplication(t, sessionClient, registrationClient, now)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := app.Login(context.Background(), actor, LoginInput{Phone: "+8613900000001", Password: "existing-password", CaptchaPassed: true}); err != nil {
		t.Fatal(err)
	}
	candidate, err := app.Register(context.Background(), actor, RegisterInput{
		Phone: "+8613900000002", Password: "ReasonixDev#2026", SMSCode: "123456",
	})
	if !isProfileMismatch(err) {
		t.Fatalf("registration conflict error = %v", err)
	}
	if candidate.Binding.CloudUserID != "usr_candidate" || candidate.Binding.BindingGeneration != 2 {
		t.Fatalf("candidate view = %#v", candidate)
	}
	current, stateErr := app.State(context.Background(), actor)
	if stateErr != nil || current.Binding.CloudUserID != "usr_existing" || current.State != AccountAuthenticated {
		t.Fatalf("current identity = %#v, %v", current, stateErr)
	}
	currentSession, ok, loadErr := app.sessions.Load(context.Background(), actor)
	if loadErr != nil || !ok || currentSession.RefreshToken != "refresh-existing" {
		t.Fatalf("current session = %#v, %t, %v", currentSession, ok, loadErr)
	}
	candidateScope, scopeErr := candidate.Binding.Scope()
	if scopeErr != nil {
		t.Fatal(scopeErr)
	}
	if staged, found, loadErr := app.sessions.sessions.Load(context.Background(), candidateScope); loadErr != nil || found {
		t.Fatalf("candidate session was not cleared = %#v, %t, %v", staged, found, loadErr)
	}
}

func TestAuthApplicationRealRegistrationExpiredCryptoDoesNotRetry(t *testing.T) {
	now := time.Now().UTC()
	crypto := validRemoteRegistrationCrypto(now)
	crypto.ExpiresAt = now.Add(-time.Second)
	client := &recordingRegistrationClient{crypto: crypto, result: validRemoteRegistrationResult(now, "usr_registered")}
	app := newRemoteRegistrationApplication(t, &recordingSessionClient{}, client, now)
	view, err := app.Register(context.Background(), userctx.ActorContext{ActorID: "local", ProfileID: "default"}, RegisterInput{
		Phone: "+8613900000002", Password: "ReasonixDev#2026", SMSCode: "123456",
	})
	if err == nil {
		t.Fatal("expected expired crypto error")
	}
	if view.State != AccountAnonymous {
		t.Fatalf("expired crypto view = %#v", view)
	}
	if !reflect.DeepEqual(client.calls, []string{"password_crypto"}) {
		t.Fatalf("expired crypto calls = %v", client.calls)
	}
}

func TestAuthApplicationRejectsPartialRealRegistrationDependencies(t *testing.T) {
	bindings := NewBindingRepository()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	cleanup := NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{})
	_, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions, Cleanup: cleanup,
		SessionClient: &recordingSessionClient{}, RegistrationClient: &recordingRegistrationClient{},
	})
	if err == nil || !strings.Contains(err.Error(), "registration") {
		t.Fatalf("partial registration dependencies error = %v", err)
	}
}

func newRemoteRegistrationApplication(t *testing.T, sessionClient CloudSessionClient, registrationClient CloudRegistrationClient, now time.Time) *AuthApplication {
	t.Helper()
	bindings := NewBindingRepository()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	cleanup := NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{})
	random := bytes.NewReader(bytes.Repeat([]byte{0x5a}, 256))
	app, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions, Cleanup: cleanup, SessionClient: sessionClient,
		RegistrationClient:    registrationClient,
		RegistrationEncryptor: NewRegistrationPasswordEncryptor(random, func() time.Time { return now }),
		Captcha:               NewDevelopmentCaptchaGateway(random, func() time.Time { return now }),
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

type recordingRegistrationClient struct {
	captchaConfig   CaptchaConfig
	crypto          RegistrationPasswordCrypto
	cryptoErr       error
	smsResult       RegistrationSMSResult
	smsErr          error
	result          RegistrationResult
	registerErr     error
	calls           []string
	smsCalls        int
	smsRequest      SendRegistrationSMSRequest
	registerRequest RegistrationRequest
}

func (c *recordingRegistrationClient) CaptchaConfig(context.Context) (CaptchaConfig, error) {
	return c.captchaConfig, nil
}

type orderedRegistrationBindingStore struct {
	BindingStore
	calls *[]string
}

func (s *orderedRegistrationBindingStore) ConfirmBinding(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	*s.calls = append(*s.calls, "binding")
	return s.BindingStore.ConfirmBinding(ctx, actor, cloudUserID)
}

type orderedRegistrationSessionStore struct {
	CloudSessionStore
	calls *[]string
}

func (s *orderedRegistrationSessionStore) Save(ctx context.Context, session Session) error {
	*s.calls = append(*s.calls, "session")
	return s.CloudSessionStore.Save(ctx, session)
}

type failingRegistrationSessionStore struct {
	CloudSessionStore
	saveErr error
}

func (s *failingRegistrationSessionStore) Save(context.Context, Session) error {
	return s.saveErr
}

func (c *recordingRegistrationClient) PasswordCrypto(context.Context) (RegistrationPasswordCrypto, error) {
	c.calls = append(c.calls, "password_crypto")
	return c.crypto, c.cryptoErr
}

func (c *recordingRegistrationClient) SendRegistrationSMS(_ context.Context, request SendRegistrationSMSRequest) (RegistrationSMSResult, error) {
	c.smsCalls++
	c.smsRequest = request
	return c.smsResult, c.smsErr
}

func (c *recordingRegistrationClient) Register(_ context.Context, request RegistrationRequest) (RegistrationResult, error) {
	c.calls = append(c.calls, "register")
	c.registerRequest = request
	return c.result, c.registerErr
}

func validRemoteRegistrationCrypto(now time.Time) RegistrationPasswordCrypto {
	return validRegistrationPasswordCrypto(now)
}

func validRemoteRegistrationResult(now time.Time, userID string) RegistrationResult {
	result := validRegistrationResult(now)
	result.Authentication.User = CloudUser{CloudUserID: userID, DisplayName: "Registered"}
	result.Authentication.Session = RemoteSession{
		AccessToken: "access-registered", AccessTokenExpiresAt: now.Add(time.Hour),
		RefreshToken: "refresh-registered", RefreshTokenExpiresAt: now.Add(30 * 24 * time.Hour), DeviceID: "device-a12",
	}
	result.Authentication.ServerTime = now
	return result
}

var _ CloudRegistrationClient = (*recordingRegistrationClient)(nil)
