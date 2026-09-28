package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestAuthApplicationProductionLoginPersistsBoundRemoteSession(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC)
	client := &recordingSessionClient{login: AuthenticationResult{
		User: CloudUser{CloudUserID: "usr_remote", DisplayName: "Remote User"},
		Session: RemoteSession{
			AccessToken: "access-01", AccessTokenExpiresAt: now.Add(time.Hour),
			RefreshToken: "refresh-01", RefreshTokenExpiresAt: now.Add(30 * 24 * time.Hour), DeviceID: "device-01",
		},
		ServerTime: now,
	}}
	app := newRemoteAuthApplication(t, client)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}

	view, err := app.Login(context.Background(), actor, developmentLoginInput("13800138000", "local-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if view.State != AccountAuthenticated || view.Binding.CloudUserID != "usr_remote" || view.User.CloudUserID != "usr_remote" || view.LastSyncAt != now {
		t.Fatalf("unexpected auth view: %#v", view)
	}
	stored, ok, err := app.sessions.Load(context.Background(), actor)
	if err != nil || !ok {
		t.Fatalf("stored session = %#v, %t, %v", stored, ok, err)
	}
	if stored.State != SessionAuthenticated || stored.AccessToken != "access-01" || stored.RefreshToken != "refresh-01" || stored.DeviceID != "device-01" || stored.AccessTokenExpiresAt != now.Add(time.Hour) || stored.RefreshTokenExpiresAt != now.Add(30*24*time.Hour) {
		t.Fatalf("unexpected stored session: %#v", stored)
	}
	if client.loginRequest.Phone != "13800138000" || client.loginRequest.Password != "local-secret" || client.loginRequest.CaptchaTicket == "" || client.loginRequest.CaptchaProvider != LoginCaptchaProviderAliyunCaptchaV2 {
		t.Fatal("login request did not include the verified CAPTCHA contract")
	}
}

func TestAuthApplicationProductionLoginForwardsRawCaptchaProofUnchanged(t *testing.T) {
	client := &recordingSessionClient{}
	app := newRemoteAuthApplicationWithoutCaptcha(t, client)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	ticket := `{"sceneId":"register-scene","token":"opaque"}`

	_, _ = app.Login(context.Background(), actor, LoginInput{
		Phone: "13800138000", Password: "local-secret",
		CaptchaTicket: ticket, CaptchaProvider: LoginCaptchaProviderAliyunCaptchaV2,
	})
	if client.loginCalls != 1 || client.loginRequest.CaptchaTicket != ticket || client.loginRequest.CaptchaProvider != LoginCaptchaProviderAliyunCaptchaV2 {
		t.Fatalf("raw CAPTCHA proof was not forwarded unchanged: %#v", client.loginRequest)
	}
}

func TestAuthApplicationProductionLoginBlocksUnavailableCaptchaBeforeSessionClient(t *testing.T) {
	now := time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC)
	client := &recordingSessionClient{login: AuthenticationResult{
		User: CloudUser{CloudUserID: "usr_remote"},
		Session: RemoteSession{
			AccessToken: "access-01", AccessTokenExpiresAt: now.Add(time.Hour),
			RefreshToken: "refresh-01", RefreshTokenExpiresAt: now.Add(24 * time.Hour), DeviceID: "device-01",
		},
		ServerTime: now,
	}}
	app := newRemoteAuthApplicationWithoutCaptcha(t, client)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}

	if _, err := app.Login(context.Background(), actor, LoginInput{Phone: "13800138000", Password: "local-secret"}); err == nil {
		t.Fatal("Login accepted an unavailable CAPTCHA gateway")
	}
	if client.loginCalls != 0 {
		t.Fatalf("session client login calls = %d, want 0", client.loginCalls)
	}
}

func TestNewAuthApplicationAllowsProductionLoginCaptchaWithoutRegistration(t *testing.T) {
	bindings := NewBindingRepository()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	_, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions,
		Cleanup:       NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{}),
		SessionClient: &recordingSessionClient{},
		Captcha:       NewDevelopmentCaptchaGateway(nil, nil),
	})
	if err != nil {
		t.Fatalf("production login CAPTCHA configuration rejected: %v", err)
	}
}

func TestAuthApplicationProductionLoginConsumesCaptchaBeforeSessionClient(t *testing.T) {
	now := time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC)
	client := &recordingSessionClient{login: AuthenticationResult{
		User: CloudUser{CloudUserID: "usr_remote"},
		Session: RemoteSession{
			AccessToken: "access-01", AccessTokenExpiresAt: now.Add(time.Hour),
			RefreshToken: "refresh-01", RefreshTokenExpiresAt: now.Add(24 * time.Hour), DeviceID: "device-01",
		},
		ServerTime: now,
	}}
	app := newRemoteAuthApplication(t, client)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}

	if _, err := app.Login(context.Background(), actor, LoginInput{Phone: "13800138000", Password: "local-secret"}); !errors.Is(err, ErrCaptchaVerificationRequired) {
		t.Fatalf("unverified login error = %v", err)
	}
	if client.loginCalls != 0 {
		t.Fatalf("session client calls after unverified CAPTCHA = %d, want 0", client.loginCalls)
	}
	if _, err := app.Login(context.Background(), actor, developmentLoginInput("13800138000", "local-secret")); err != nil {
		t.Fatal(err)
	}
	if client.loginCalls != 1 {
		t.Fatalf("session client calls after verified CAPTCHA = %d, want 1", client.loginCalls)
	}
	if client.loginRequest.CaptchaTicket == "" || client.loginRequest.CaptchaProvider != LoginCaptchaProviderAliyunCaptchaV2 {
		t.Fatal("verified CAPTCHA was not forwarded to the session client")
	}
}

func TestAuthApplicationProductionLoginBlocksExpiredCaptchaBeforeSessionClient(t *testing.T) {
	client := &recordingSessionClient{}
	expiredCaptcha := NewDevelopmentCaptchaGateway(nil, func() time.Time { return time.Now().Add(-developmentCaptchaPermitTTL) })
	app := newRemoteAuthApplicationWithCaptcha(t, client, expiredCaptcha)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}

	if _, err := app.Login(context.Background(), actor, developmentLoginInput("13800138000", "local-secret")); !errors.Is(err, ErrCaptchaPermitExpired) {
		t.Fatalf("expired CAPTCHA login error = %v", err)
	}
	if client.loginCalls != 0 {
		t.Fatalf("session client calls after expired CAPTCHA = %d, want 0", client.loginCalls)
	}
}

func TestAuthApplicationProductionRefreshRotatesBeforeUseAndLogoutClearsLocally(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC)
	client := &recordingSessionClient{
		login:     AuthenticationResult{User: CloudUser{CloudUserID: "usr_remote"}, Session: RemoteSession{AccessToken: "access-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshToken: "refresh-01", RefreshTokenExpiresAt: now.Add(24 * time.Hour), DeviceID: "device-01"}, ServerTime: now},
		refresh:   RemoteSession{AccessToken: "access-02", AccessTokenExpiresAt: now.Add(2 * time.Hour), RefreshToken: "refresh-02", RefreshTokenExpiresAt: now.Add(48 * time.Hour), DeviceID: "device-01"},
		logoutErr: errors.New("remote unavailable"),
	}
	app := newRemoteAuthApplication(t, client)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := app.Login(context.Background(), actor, developmentLoginInput("13800138000", "local-secret")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Refresh(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	stored, ok, err := app.sessions.Load(context.Background(), actor)
	if err != nil || !ok || stored.RefreshToken != "refresh-02" {
		t.Fatalf("rotated session = %#v, %t, %v", stored, ok, err)
	}
	if client.refreshToken != "refresh-01" {
		t.Fatalf("refresh token = %q", client.refreshToken)
	}
	if err := app.Logout(context.Background(), actor); err == nil {
		t.Fatal("expected remote logout warning")
	}
	if _, ok, err := app.sessions.Load(context.Background(), actor); err != nil || ok {
		t.Fatalf("local session survived logout: %t, %v", ok, err)
	}
	if client.logoutToken != "refresh-02" {
		t.Fatalf("logout token = %q", client.logoutToken)
	}
	view, err := app.State(context.Background(), actor)
	if err != nil || view.State != AccountAnonymous {
		t.Fatalf("state after logout = %#v, %v", view, err)
	}
}

func TestAuthApplicationProductionAccountSwitchCommitsOnlyAfterConfirmation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC)
	result := func(user, suffix string) AuthenticationResult {
		return AuthenticationResult{User: CloudUser{CloudUserID: user, DisplayName: user}, Session: RemoteSession{
			AccessToken: "access-" + suffix, AccessTokenExpiresAt: now.Add(time.Hour),
			RefreshToken: "refresh-" + suffix, RefreshTokenExpiresAt: now.Add(24 * time.Hour), DeviceID: "device-01",
		}, ServerTime: now}
	}
	client := &recordingSessionClient{loginByPhone: map[string]AuthenticationResult{"13800138000": result("usr_a", "a"), "13900139000": result("usr_b", "b")}}
	app := newRemoteAuthApplication(t, client)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := app.Login(context.Background(), actor, developmentLoginInput("13800138000", "secret-a")); err != nil {
		t.Fatal(err)
	}
	var switchInput LoginInput
	if err := json.Unmarshal([]byte(`{"Phone":"13900139000","Password":"secret-b","CaptchaTicket":"verified-switch-captcha","CaptchaProvider":"aliyun_captcha_v2"}`), &switchInput); err != nil {
		t.Fatal(err)
	}
	preparation, err := app.PrepareAccountSwitch(context.Background(), actor, switchInput)
	if err != nil {
		t.Fatal(err)
	}
	if client.loginRequest.CaptchaTicket == "" || client.loginRequest.CaptchaProvider != LoginCaptchaProviderAliyunCaptchaV2 {
		t.Fatal("account switch request omitted verified CAPTCHA fields")
	}
	if current, err := app.State(context.Background(), actor); err != nil || current.Binding.CloudUserID != "usr_a" {
		t.Fatalf("candidate changed current identity: %#v, %v", current, err)
	}
	if preparation.View().Binding.CloudUserID != "usr_b" {
		t.Fatalf("candidate view = %#v", preparation.View())
	}
	committed, err := app.CommitAccountSwitch(context.Background(), preparation)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Binding.CloudUserID != "usr_b" || committed.Binding.BindingGeneration != 2 {
		t.Fatalf("committed view = %#v", committed)
	}
	stored, ok, err := app.sessions.Load(context.Background(), actor)
	if err != nil || !ok || stored.RefreshToken != "refresh-b" || stored.Scope.CloudUserID != "usr_b" {
		t.Fatalf("stored switched session = %#v, %t, %v", stored, ok, err)
	}
}

func TestAuthApplicationProductionDifferentLoginReturnsCandidateIdentity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 13, 3, 0, 0, 0, time.UTC)
	result := func(user string) AuthenticationResult {
		return AuthenticationResult{User: CloudUser{CloudUserID: user}, Session: RemoteSession{AccessToken: "access-" + user, AccessTokenExpiresAt: now.Add(time.Hour), RefreshToken: "refresh-" + user, RefreshTokenExpiresAt: now.Add(24 * time.Hour), DeviceID: "device-01"}, ServerTime: now}
	}
	client := &recordingSessionClient{loginByPhone: map[string]AuthenticationResult{"13800138000": result("usr_a"), "13900139000": result("usr_b")}}
	app := newRemoteAuthApplication(t, client)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := app.Login(context.Background(), actor, developmentLoginInput("13800138000", "secret-a")); err != nil {
		t.Fatal(err)
	}
	candidate, err := app.Login(context.Background(), actor, developmentLoginInput("13900139000", "secret-b"))
	if !isProfileMismatch(err) {
		t.Fatalf("candidate error = %v", err)
	}
	if candidate.User.CloudUserID != "usr_b" || candidate.Binding.CloudUserID != "usr_b" || candidate.State != AccountOffline {
		t.Fatalf("candidate identity = %#v", candidate)
	}
	current, err := app.State(context.Background(), actor)
	if err != nil || current.Binding.CloudUserID != "usr_a" || current.State != AccountAuthenticated {
		t.Fatalf("current identity changed = %#v, %v", current, err)
	}
}

func newRemoteAuthApplication(t *testing.T, client CloudSessionClient) *AuthApplication {
	return newRemoteAuthApplicationWithCaptcha(t, client, NewDevelopmentCaptchaGateway(nil, nil))
}

func newRemoteAuthApplicationWithoutCaptcha(t *testing.T, client CloudSessionClient) *AuthApplication {
	return newRemoteAuthApplicationWithCaptcha(t, client, nil)
}

func newRemoteAuthApplicationWithCaptcha(t *testing.T, client CloudSessionClient, captcha CaptchaGateway) *AuthApplication {
	t.Helper()
	bindings := NewBindingRepository()
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	cleanup := NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{})
	app, err := NewAuthApplication(AuthApplicationDependencies{Bindings: bindings, Sessions: sessions, Cleanup: cleanup, SessionClient: client, Captcha: captcha})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func developmentLoginInput(phone, password string) LoginInput {
	return LoginInput{Phone: phone, Password: password, CaptchaPassed: true}
}

type recordingSessionClient struct {
	login        AuthenticationResult
	loginByPhone map[string]AuthenticationResult
	loginCalls   int
	refresh      RemoteSession
	loginRequest LoginRequest
	refreshToken string
	logoutToken  string
	logoutErr    error
}

func (c *recordingSessionClient) Login(_ context.Context, request LoginRequest) (AuthenticationResult, error) {
	c.loginCalls++
	c.loginRequest = request
	if c.loginByPhone != nil {
		return c.loginByPhone[request.Phone], nil
	}
	return c.login, nil
}

func (c *recordingSessionClient) Refresh(_ context.Context, token string) (RemoteSession, error) {
	c.refreshToken = token
	return c.refresh, nil
}

func (c *recordingSessionClient) Logout(_ context.Context, token string) error {
	c.logoutToken = token
	return c.logoutErr
}

func (c *recordingSessionClient) CurrentUser(context.Context, string) (CurrentUserSnapshot, error) {
	return CurrentUserSnapshot{}, errors.New("not used")
}

var _ CloudSessionClient = (*recordingSessionClient)(nil)
