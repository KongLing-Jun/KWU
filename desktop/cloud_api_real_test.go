package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/boot"
	cloud "kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	"kwu/internal/userctx"
)

func TestCloudLoginAndSwitchDevelopmentRuntimeForwardCaptchaVerification(t *testing.T) {
	var loginCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/auth/login":
			var body struct {
				Phone           string `json:"phone"`
				CaptchaTicket   string `json:"captcha_ticket"`
				CaptchaProvider string `json:"captcha_provider"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.CaptchaTicket == "" || body.CaptchaProvider != cloud.LoginCaptchaProviderAliyunCaptchaV2 {
				t.Fatal("development login omitted CAPTCHA verification")
			}
			loginCalls.Add(1)
			if body.Phone == "13900139000" {
				_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_switch","display_name":"Switch User","status":"active","created_at":"2026-07-14T04:00:00Z"},"session":{"access_token":"access-switch","access_token_expires_at":"2030-07-14T05:00:00Z","refresh_token":"refresh-switch","refresh_token_expires_at":"2030-08-13T04:00:00Z"}},"error":null,"request_id":"req-switch","server_time":"2026-07-14T04:00:00Z"}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_login","display_name":"Login User","status":"active","created_at":"2026-07-14T04:00:00Z"},"session":{"access_token":"access-login","access_token_expires_at":"2030-07-14T05:00:00Z","refresh_token":"refresh-login","refresh_token_expires_at":"2030-08-13T04:00:00Z"}},"error":null,"request_id":"req-login","server_time":"2026-07-14T04:00:00Z"}`))
		case "/api/v1/me":
			if request.Header.Get("Authorization") == "Bearer access-switch" {
				_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_switch","display_name":"Switch User","status":"active","created_at":"2026-07-14T04:00:00Z"},"balance":{"total_balance":"2000.0000","free_balance":"2000.0000","subscription_balance":"0.0000","available_balance":"2000.0000","free_available_balance":"2000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_amount":"0.0000","subscription_frozen_amount":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0}},"error":null,"request_id":"req-me-switch","server_time":"2026-07-14T04:00:01Z"}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_login","display_name":"Login User","status":"active","created_at":"2026-07-14T04:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_amount":"0.0000","subscription_frozen_amount":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0}},"error":null,"request_id":"req-me","server_time":"2026-07-14T04:00:01Z"}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	runtime, err := newRealCloudRuntimeWithRegistration(
		server.URL, desktopCloudTestProfilePathProvider(t.TempDir()), server.Client(),
		cloud.DeviceIdentity{DeviceID: "device-login", ClientVersion: "dev"}, cloudRegistrationDevelopment, desktopTestCloudEnvironment(server.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(userctx.ActorContext{ActorID: "local", ProfileID: "default"})

	var request CloudLoginRequest
	if err := json.Unmarshal([]byte(`{"phone":"13800138000","password":"local-secret","captcha_passed":true}`), &request); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CloudLogin(request); err != nil {
		t.Fatal(err)
	}
	if loginCalls.Load() != 1 {
		t.Fatalf("development login HTTP calls = %d, want 1", loginCalls.Load())
	}
	var switchRequest CloudLoginRequest
	if err := json.Unmarshal([]byte(`{"phone":"13900139000","password":"local-switch-secret","captcha_passed":true}`), &switchRequest); err != nil {
		t.Fatal(err)
	}
	view, err := app.CloudSwitchAccount(switchRequest)
	if err != nil {
		t.Fatal(err)
	}
	if loginCalls.Load() != 2 || view.Account.CloudUserID != "usr_switch" {
		t.Fatalf("development switch result = calls %d, account %q", loginCalls.Load(), view.Account.CloudUserID)
	}
}

func TestCloudSwitchAccountRestoresOnlyMatchingPendingSubscriptionTabs(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "SWITCH_AUTH_KEY")

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.setCurrentActorContext(actor)
	app.cloudRuntime.start(app.ctx)
	app.billingWorkerRuntime.start(app.ctx)
	t.Cleanup(func() {
		for _, tab := range app.tabRuntimes {
			if tab != nil && tab.Ctrl != nil {
				tab.Ctrl.Close()
			}
		}
		app.cloudRuntime.close()
		_ = app.billingWorkerRuntime.close()
	})

	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}

	matching := app.createTabEntryWithIDAndActorForRoute(
		"project", root, "", "switch-matching-pending", actor, modelbilling.BillingSubscriptionPool,
	)
	matching.model = modelRef
	matching.Ready = true
	matching.subscriptionAuthPending = true
	matching.StartupErr = "matching profile startup error"

	otherActor := actor
	otherActor.ProfileID += "-other"
	other := app.createTabEntryWithIDAndActorForRoute(
		"project", root, "", "switch-other-profile-pending", otherActor, modelbilling.BillingSubscriptionPool,
	)
	other.model = modelRef
	other.Ready = true
	other.subscriptionAuthPending = true
	other.StartupErr = "other profile startup error"

	app.tabRuntimes = map[string]*WorkspaceTab{matching.ID: matching, other.ID: other}
	app.tabOrder = []string{matching.ID, other.ID}
	app.activeTabID = matching.ID

	switched, err := app.CloudSwitchAccount(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	if switched.Account.CloudUserID != "usr_02" {
		t.Fatalf("switched account = %#v", switched.Account)
	}
	if matching.Ctrl == nil || !matching.Ready || matching.subscriptionAuthPending || matching.StartupErr != "" {
		t.Fatalf("matching tab = ctrl:%T ready:%v pending:%v startup:%q", matching.Ctrl, matching.Ready, matching.subscriptionAuthPending, matching.StartupErr)
	}
	if other.Ctrl != nil || !other.subscriptionAuthPending || other.StartupErr != "other profile startup error" {
		t.Fatalf("other profile tab = ctrl:%T pending:%v startup:%q", other.Ctrl, other.subscriptionAuthPending, other.StartupErr)
	}
}

func TestCloudRealCaptchaConfigAndRawProofCrossWailsUnchanged(t *testing.T) {
	rawTicket := strings.Repeat("x", 8192)
	loginTicket := `{"login":"opaque"}`
	resetTicket := `{"reset":"opaque"}`
	var smsTicket, smsProvider, resetSMSTicket, resetSMSProvider, forwardedLoginTicket, forwardedLoginProvider string
	var resetPhone, resetPassword, resetCode string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/auth/captcha-config":
			_, _ = w.Write([]byte(`{"data":{"provider":"aliyun_captcha_v2","region":"cn","prefix":"public-prefix","scene_id":"register-scene"},"error":null,"request_id":"req-config","server_time":"2026-07-14T04:00:00Z"}`))
		case "/api/v1/auth/sms/register/send":
			var body struct {
				CaptchaTicket   string `json:"captcha_ticket"`
				CaptchaProvider string `json:"captcha_provider"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			smsTicket, smsProvider = body.CaptchaTicket, body.CaptchaProvider
			_, _ = w.Write([]byte(`{"data":{"phone":"+8613900000002","expires_in_seconds":300},"error":null,"request_id":"req-sms","server_time":"2026-07-14T04:00:00Z"}`))
		case "/api/v1/auth/sms/password-reset/send":
			if request.Header.Get("Authorization") != "" {
				t.Fatalf("password reset SMS sent Authorization = %q", request.Header.Get("Authorization"))
			}
			var body struct {
				CaptchaTicket   string `json:"captcha_ticket"`
				CaptchaProvider string `json:"captcha_provider"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			resetSMSTicket, resetSMSProvider = body.CaptchaTicket, body.CaptchaProvider
			_, _ = w.Write([]byte(`{"data":{"phone":"13900000002","expires_in_seconds":300},"error":null,"request_id":"req-reset-sms","server_time":"2026-07-14T04:00:00Z"}`))
		case "/api/v1/auth/password/reset":
			if request.Header.Get("Authorization") != "" {
				t.Fatalf("password reset sent Authorization = %q", request.Header.Get("Authorization"))
			}
			var body struct {
				Phone       string `json:"phone"`
				NewPassword string `json:"new_password"`
				SMSCode     string `json:"sms_code"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			resetPhone, resetPassword, resetCode = body.Phone, body.NewPassword, body.SMSCode
			_, _ = w.Write([]byte(`{"data":{"reset":true},"error":null,"request_id":"req-reset","server_time":"2026-07-14T04:00:00Z"}`))
		case "/api/v1/auth/login":
			var body struct {
				CaptchaTicket   string `json:"captcha_ticket"`
				CaptchaProvider string `json:"captcha_provider"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			forwardedLoginTicket, forwardedLoginProvider = body.CaptchaTicket, body.CaptchaProvider
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_real","display_name":"Real User","status":"active","created_at":"2026-07-14T04:00:00Z"},"session":{"access_token":"access-real","access_token_expires_at":"2030-07-14T05:00:00Z","refresh_token":"refresh-real","refresh_token_expires_at":"2030-08-13T04:00:00Z"}},"error":null,"request_id":"req-login","server_time":"2026-07-14T04:00:00Z"}`))
		case "/api/v1/me":
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_real","display_name":"Real User","status":"active","created_at":"2026-07-14T04:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_amount":"0.0000","subscription_frozen_amount":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0}},"error":null,"request_id":"req-me","server_time":"2026-07-14T04:00:01Z"}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	runtime, err := newRealCloudRuntimeWithRegistration(
		server.URL, desktopCloudTestProfilePathProvider(t.TempDir()), server.Client(),
		cloud.DeviceIdentity{DeviceID: "device-real-captcha", ClientVersion: "dev"}, cloudRegistrationReal, desktopTestCloudEnvironment(server.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(userctx.ActorContext{ActorID: "local", ProfileID: "default"})

	config, err := app.CloudCaptchaConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Provider != cloud.LoginCaptchaProviderAliyunCaptchaV2 || config.Region != "cn" || config.Prefix != "public-prefix" || config.SceneID != "register-scene" {
		t.Fatalf("CAPTCHA config = %#v", config)
	}
	err = app.CloudSendRegisterSMS(CloudSendRegisterSMSRequest{
		Phone: "+8613900000002", CaptchaTicket: rawTicket, CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if smsTicket != rawTicket || smsProvider != cloud.LoginCaptchaProviderAliyunCaptchaV2 {
		t.Fatalf("raw proof changed across Wails: ticket bytes %d, provider %q", len(smsTicket), smsProvider)
	}
	err = app.CloudSendPasswordResetSMS(CloudSendPasswordResetSMSRequest{
		Phone: "+8613900000002", CaptchaTicket: resetTicket, CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resetSMSTicket != resetTicket || resetSMSProvider != cloud.LoginCaptchaProviderAliyunCaptchaV2 {
		t.Fatalf("password reset proof changed across Wails: ticket %q, provider %q", resetSMSTicket, resetSMSProvider)
	}
	err = app.CloudResetPassword(CloudResetPasswordRequest{
		Phone: "+8613900000002", NewPassword: "new-password-a12", SMSCode: "654321",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resetPhone != "+8613900000002" || resetPassword != "new-password-a12" || resetCode != "654321" {
		t.Fatalf("password reset changed across Wails: phone %q, password %q, code %q", resetPhone, resetPassword, resetCode)
	}
	view, err := app.CloudLogin(CloudLoginRequest{
		Phone: "13800138000", Password: "local-secret", CaptchaTicket: loginTicket, CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if forwardedLoginTicket != loginTicket || forwardedLoginProvider != cloud.LoginCaptchaProviderAliyunCaptchaV2 || view.Account.CloudUserID != "usr_real" {
		t.Fatalf("login proof changed across Wails: ticket %q, provider %q, account %q", forwardedLoginTicket, forwardedLoginProvider, view.Account.CloudUserID)
	}
}

func TestCloudLoginFreshFreeUserRestoresUniversalSubscriptionModel(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "")
	isolateDesktopUserDirs(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_remote","display_name":"Remote User","status":"active","created_at":"2026-07-13T03:00:00Z"},"session":{"access_token":"access-01","access_token_expires_at":"2030-07-13T05:00:00Z","refresh_token":"refresh-01","refresh_token_expires_at":"2030-08-12T04:00:00Z"}},"error":null,"request_id":"req-login","server_time":"2026-07-13T04:00:00Z"}`))
		case "/api/v1/me":
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_remote","display_name":"Remote User","status":"active","created_at":"2026-07-13T03:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"subscription_id":null,"plan":"free","plan_config_version":null,"status":"free","period_start":null,"period_end":null,"version":0}},"error":null,"request_id":"req-me","server_time":"2026-07-13T04:00:01Z"}`))
		case "/api/v1/subscription-plans":
			if r.Header.Get("Authorization") != "" {
				t.Fatalf("public plan request Authorization = %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"data":{"plans":[{"plan_id":"plan_lite_v1","plan":"lite","display_name":"Lite","price_amount":"9.90","currency":"CNY","credit_amount":"20000.0000","period_days":30,"enabled":true,"sort_order":10,"config_version":"catalog-v1"},{"plan_id":"plan_plus_v1","plan":"plus","display_name":"Plus","price_amount":"29.90","currency":"CNY","credit_amount":"60000.0000","period_days":30,"enabled":true,"sort_order":20,"config_version":"catalog-v1"},{"plan_id":"plan_pro_v1","plan":"pro","display_name":"Pro","price_amount":"59.90","currency":"CNY","credit_amount":"120000.0000","period_days":30,"enabled":true,"sort_order":30,"config_version":"catalog-v1"}]},"error":null,"request_id":"req-plans","server_time":"2026-07-13T04:00:01Z"}`))
		case "/api/v1/me/maas-key":
			_, _ = w.Write([]byte(`{"data":{"status":"ready","key_name":"usr_remote","key_id":9,"maas_key":"sk-fixture"},"error":null,"request_id":"req-maas","server_time":"2026-07-13T04:00:01Z"}`))
		case "/api/v1/auth/logout":
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"data":null,"error":{"code":"NETWORK_UNAVAILABLE","message":"temporarily unavailable","retryable":true},"request_id":"req-logout","server_time":"2026-07-13T04:00:02Z"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	runtime, err := newRealCloudRuntimeWithRegistration(
		server.URL, desktopCloudTestProfilePathProvider(t.TempDir()), server.Client(),
		cloud.DeviceIdentity{DeviceID: "device-01", DeviceName: "Windows PC", ClientVersion: "dev"}, cloudRegistrationDevelopment, desktopTestCloudEnvironment(server.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.billingWorkerRuntime.factory = func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}}, nil
	}
	root := t.TempDir()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default", WorkspaceRoot: root}.Normalize()
	cfg := config.Default()
	// Reproduce a fresh profile created by the previous release: the official
	// default is Pro-only GPT and no explicit desktop billing route was saved.
	cfg.DefaultModel = "kwu/gpt-5.4"
	cfg.Desktop.DefaultBillingSource = ""
	if err := cfg.SaveTo(config.NewProfilePathResolver(actor).UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	app.subscriptionModels.fetch = func(_ context.Context, _ netclient.ProxySpec, key string) ([]string, error) {
		if key != "sk-fixture" {
			t.Fatalf("catalog key = %q", key)
		}
		// The real subscription catalog is account-scoped but not plan-filtered:
		// GPT remains present for Pro while DeepSeek is available to every plan.
		return []string{"gpt-5.4", "gpt-5.4-mini", "deepseek-v4-flash"}, nil
	}
	app.setCurrentActorContext(actor)
	app.billingWorkerRuntime.start(context.Background())
	t.Cleanup(func() { _ = app.billingWorkerRuntime.close() })
	tab := app.createTabEntryWithIDAndActorForRoute("project", root, "", "stale-default", actor, "")
	tab.Ready = true
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	view, err := app.CloudLogin(CloudLoginRequest{Phone: "13800138000", Password: "local-secret", CaptchaPassed: true})
	if err != nil {
		t.Fatal(err)
	}
	if view.Account.Status != cloud.AccountAuthenticated || view.Account.CloudUserID != "usr_remote" || view.Balance == nil || view.Balance.TotalBalance != "1000.0000" {
		t.Fatalf("unexpected real login view: %#v", view)
	}
	if view.Subscription == nil || view.Subscription.Current.Plan != "free" || view.Subscription.Current.Version != 0 || len(view.Subscription.Plans) != 3 || view.Subscription.Plans[1].Plan != "plus" {
		t.Fatalf("unexpected real subscription: %#v", view.Subscription)
	}
	if !view.Capabilities.Registration || !view.Capabilities.Payments || view.Capabilities.Realtime {
		t.Fatalf("unexpected real capabilities: %#v", view.Capabilities)
	}
	if view.Capabilities.RegistrationMode != string(cloudRegistrationDevelopment) {
		t.Fatalf("real registration mode = %q", view.Capabilities.RegistrationMode)
	}
	if tab.Ctrl == nil || tab.model != "kwu/deepseek-v4-flash" || tab.billingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("restored fresh-user tab = ctrl:%T model:%q source:%q", tab.Ctrl, tab.model, tab.billingSource)
	}
	tab.Ctrl.Close()
	app.tabRuntimes = map[string]*WorkspaceTab{}
	app.tabOrder = nil
	app.activeTabID = ""
	modelRef := "kwu/deepseek-v4-flash"
	ctrl, err := app.buildDesktopController(context.Background(), boot.Options{
		Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard,
	}, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool))
	if err != nil {
		t.Fatalf("real subscription controller: %v", err)
	}
	ctrl.Close()
	projectScope, err := app.currentProjectScopeForActor(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	cloudSessionPath := filepath.Join(root, "logout-cloud-session.jsonl")
	if err := os.WriteFile(cloudSessionPath, []byte(`{"role":"user","content":"private cloud turn"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(cloudSessionPath, agent.BranchMeta{Scope: "project", WorkspaceRoot: root, TopicID: "cloud-topic", ProjectContext: &agent.ProjectContextPin{
		Environment:        agent.ProjectEnvironmentIdentity{EnvironmentID: projectScope.Environment.EnvironmentID, CloudBaseURL: projectScope.Environment.CloudBaseURL},
		BindingScope:       agent.ProjectBindingScope{ProfileID: projectScope.Binding.ProfileID, CloudUserID: projectScope.Binding.CloudUserID, BindingGeneration: projectScope.Binding.BindingGeneration},
		ProjectID:          "project-private",
		InstructionVersion: 1,
		ContentHash:        strings.Repeat("a", 64),
	}}); err != nil {
		t.Fatal(err)
	}
	cloudTab := &WorkspaceTab{ID: "logout-cloud", Scope: "project", WorkspaceRoot: root, SessionPath: cloudSessionPath, Ready: true, actor: actor}
	app.tabRuntimes = map[string]*WorkspaceTab{cloudTab.ID: cloudTab}
	app.tabOrder = []string{cloudTab.ID}
	app.activeTabID = cloudTab.ID
	persistTestTabState(t, app, cloudTab.ID)
	loggedOut, err := app.CloudLogout()
	if err != nil {
		t.Fatalf("remote logout warning escaped as bridge error: %v", err)
	}
	if loggedOut.Account.Status != cloud.AccountAnonymous || loggedOut.OperationError == nil || loggedOut.OperationError.Code != cloud.ErrorNetworkUnavailable {
		t.Fatalf("logout warning view = %#v", loggedOut)
	}
	if app.tabRuntimes[cloudTab.ID] != nil || app.cachedActiveTabID() == "" || app.cachedActiveTabID() == cloudTab.ID {
		t.Fatalf("remote-failed local logout retained cloud tab: active=%q tabs=%#v", app.cachedActiveTabID(), app.tabRuntimes)
	}
	if _, err := os.Stat(cloudSessionPath); err != nil {
		t.Fatalf("remote-failed local logout removed cloud transcript: %v", err)
	}
	if _, err := os.Stat(agent.BranchMetaPath(cloudSessionPath)); err != nil {
		t.Fatalf("remote-failed local logout removed cloud metadata: %v", err)
	}
	ctrl, err = app.buildDesktopController(context.Background(), boot.Options{
		Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard,
	}, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool))
	if ctrl != nil {
		ctrl.Close()
	}
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("logged-out subscription controller error = %v, want auth_required", err)
	}
}

func TestCloudRealRegistrationRuntimeHydratesAccount(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "")
	var requests []string
	var meAvailable atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/auth/sms/register/send":
			_, _ = w.Write([]byte(`{"data":{"phone":"+8613900000002","expires_in_seconds":300},"error":null,"request_id":"req-sms","server_time":"2026-07-14T04:00:00Z"}`))
		case "/api/v1/auth/register-password-crypto":
			_, _ = w.Write([]byte(`{"data":{"algorithm":"AES","key_size_bits":128,"mode":"CBC","padding":"PKCS7Padding","output":"Base64","ciphertext_encoding":"base64(iv || ciphertext)","key_version":"auth-key-v1","key_base64":"MDEyMzQ1Njc4OWFiY2RlZg==","expires_at":"2030-07-14T04:05:00Z"},"error":null,"request_id":"req-crypto","server_time":"2026-07-14T04:00:01Z"}`))
		case "/api/v1/auth/register":
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_registered","display_name":"Registered User","status":"active","created_at":"2026-07-14T04:00:02Z"},"session":{"access_token":"access-registered","access_token_expires_at":"2030-07-14T05:00:00Z","refresh_token":"refresh-registered","refresh_token_expires_at":"2030-08-13T04:00:00Z"},"signup_reward":{"granted":true,"amount":"1000.0000","ledger_id":"led_registered"},"third_party_sync":{"sync_id":"tps_registered","provider":"new-api","status":"pending","version":1,"updated_at":"2026-07-14T04:00:02Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":2}},"error":null,"request_id":"req-register","server_time":"2026-07-14T04:00:02Z"}`))
		case "/api/v1/me":
			if request.Header.Get("Authorization") != "Bearer access-registered" {
				t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
			}
			if !meAvailable.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"data":null,"error":{"code":"NETWORK_UNAVAILABLE","message":"temporarily unavailable","retryable":true},"request_id":"req-me-offline","server_time":"2026-07-14T04:00:03Z"}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_registered","display_name":"Registered User","status":"active","created_at":"2026-07-14T04:00:02Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":2},"subscription":{"plan":"free","status":"free","period_end":null,"version":0}},"error":null,"request_id":"req-me","server_time":"2026-07-14T04:00:03Z"}`))
		case "/api/v1/subscription-plans":
			_, _ = w.Write([]byte(`{"data":{"plans":[{"plan_id":"plan_lite_v1","plan":"lite","display_name":"Lite","price_amount":"9.90","currency":"CNY","credit_amount":"20000.0000","period_days":30,"enabled":true,"sort_order":10,"config_version":"catalog-v1"}]},"error":null,"request_id":"req-plans","server_time":"2026-07-14T04:00:03Z"}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	runtime, err := newRealCloudRuntimeWithRegistration(
		server.URL, desktopCloudTestProfilePathProvider(root), server.Client(),
		cloud.DeviceIdentity{DeviceID: "device-a12", DeviceName: "Windows PC", ClientVersion: "0.9.0"},
		cloudRegistrationReal, desktopTestCloudEnvironment(server.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(userctx.ActorContext{ActorID: "local", ProfileID: "default"})

	if err := app.CloudSendRegisterSMS(CloudSendRegisterSMSRequest{
		Phone:           "+8613900000002",
		CaptchaTicket:   `{"token":"opaque-register-proof"}`,
		CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2,
	}); err != nil {
		t.Fatal(err)
	}
	plaintext := "ReasonixDev#2026"
	view, err := app.CloudRegister(CloudRegisterRequest{Phone: "+8613900000002", Password: plaintext, SMSCode: "123456"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Account.Status != cloud.AccountOffline || view.OperationError == nil || view.OperationError.Code != cloud.ErrorNetworkUnavailable {
		t.Fatalf("offline registration view = %#v", view)
	}
	stored, ok, err := runtime.sessions.Load(app.bootContext(), app.currentActorContext())
	if err != nil || !ok || stored.State != cloud.SessionOffline || stored.RefreshToken != "refresh-registered" {
		t.Fatalf("offline registration session = %#v, %t, %v", stored, ok, err)
	}
	meAvailable.Store(true)
	view, err = app.CloudRefreshAssets()
	if err != nil {
		t.Fatal(err)
	}
	if !view.Capabilities.Registration || view.Capabilities.RegistrationMode != string(cloudRegistrationReal) || !view.Capabilities.Payments || view.Capabilities.Realtime {
		t.Fatalf("real capabilities = %#v", view.Capabilities)
	}
	if view.Account.Status != cloud.AccountAuthenticated || view.Account.CloudUserID != "usr_registered" || view.Account.DisplayName != "Registered User" {
		t.Fatalf("registered account = %#v", view.Account)
	}
	if view.Balance == nil || view.Balance.TotalBalance != "1000.0000" || view.Subscription == nil || view.Subscription.Current.Plan != "free" || len(view.Subscription.Plans) != 1 || view.Subscription.Plans[0].Plan != "lite" {
		t.Fatalf("registered assets = balance %#v subscription %#v", view.Balance, view.Subscription)
	}
	if got := strings.Join(requests, ","); got != "POST /api/v1/auth/sms/register/send,GET /api/v1/auth/register-password-crypto,POST /api/v1/auth/register,GET /api/v1/me,GET /api/v1/me,GET /api/v1/subscription-plans" {
		t.Fatalf("registration requests = %s", got)
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), plaintext) {
			t.Fatalf("plaintext password persisted in %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func hasCloudErrorCode(err error, code cloud.ErrorCode) bool {
	var typed *cloud.Error
	return errors.As(err, &typed) && typed.Code == code
}

func TestCloudWailsErrorMapsProductionAuthErrorsWithoutLeakingDetails(t *testing.T) {
	tests := []struct {
		name     string
		input    *cloud.Error
		wantCode cloud.ErrorCode
		wantMsg  string
	}{
		{
			name: "invalid credentials",
			input: &cloud.Error{
				Code:      cloud.ErrorCode("INVALID_CREDENTIALS"),
				Message:   "database lookup says the account was deleted",
				RequestID: "req-invalid-credentials",
			},
			wantCode: cloud.ErrorCode("INVALID_CREDENTIALS"),
			wantMsg:  "account does not exist or password is incorrect",
		},
		{
			name:     "reused refresh token",
			input:    &cloud.Error{Code: cloud.ErrorCode("REFRESH_TOKEN_REUSED"), Message: "refresh-token-secret"},
			wantCode: cloud.ErrorCode("REFRESH_TOKEN_REUSED"),
			wantMsg:  "cloud session expired; sign in again",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe := cloudWailsError("cloud.test", tt.input)
			var typed *cloud.Error
			if !errors.As(safe, &typed) || typed.Code != tt.wantCode || typed.Message != tt.wantMsg {
				t.Fatalf("safe production auth error = %#v", safe)
			}
			if typed.RequestID != tt.input.RequestID {
				t.Fatalf("request id = %q, want %q", typed.RequestID, tt.input.RequestID)
			}
			if strings.Contains(safe.Error(), tt.input.Message) {
				t.Fatal("safe production auth error leaked server detail")
			}
		})
	}
}

func TestCloudWailsErrorPreservesSafeRequestID(t *testing.T) {
	safe := cloudWailsError("cloud.test", &cloud.Error{
		Code: cloud.ErrorCode("SMS_RATE_LIMITED"), Message: "provider detail", RequestID: "req-a12-review",
	})
	var typed *cloud.Error
	if !errors.As(safe, &typed) {
		t.Fatalf("safe error = %#v", safe)
	}
	if typed.RequestID != "req-a12-review" {
		t.Fatalf("request id = %q", typed.RequestID)
	}
	if !strings.Contains(safe.Error(), "[request_id=req-a12-review]") {
		t.Fatalf("Wails error string lost request id: %q", safe.Error())
	}
	if strings.Contains(typed.Message, "provider detail") {
		t.Fatalf("safe message leaked provider detail: %q", typed.Message)
	}
	unsafe := cloudWailsError("cloud.test", &cloud.Error{
		Code: cloud.ErrorCode("SMS_RATE_LIMITED"), Message: "provider detail", RequestID: "req-safe\nsecret",
	})
	if !errors.As(unsafe, &typed) || typed.RequestID != "" {
		t.Fatalf("unsafe request id was exposed: %#v", unsafe)
	}
}

func TestCloudPaymentWailsErrorPreservesStableCodeAndRetryability(t *testing.T) {
	secret := "provider-or-database-secret"
	tests := []struct {
		code      cloud.ErrorCode
		message   string
		retryable bool
	}{
		{code: "VALIDATION_FAILED", message: "payment request is invalid or unavailable"},
		{code: "NOT_FOUND", message: "payment order was not found"},
		{code: "CONFLICT", message: "payment request conflicts with the current subscription"},
		{code: "IDEMPOTENCY_CONFLICT", message: "payment request conflicts with an existing order"},
		{code: "UPSTREAM_ERROR", message: "payment provider is temporarily unavailable", retryable: true},
		{code: "INTERNAL_ERROR", message: "cloud payment service is temporarily unavailable", retryable: true},
		{code: "CONCURRENT_OPERATION_RETRYABLE", message: "another payment operation is in progress; try again", retryable: true},
		{code: cloud.ErrorNetworkUnavailable, message: "cloud network is unavailable", retryable: true},
	}
	for _, test := range tests {
		t.Run(string(test.code), func(t *testing.T) {
			safe := cloudPaymentWailsError("cloud.test", errors.Join(
				errors.New(secret),
				&cloud.Error{
					Code:      test.code,
					Message:   secret,
					Retryable: test.retryable,
					Details:   json.RawMessage(`{"provider_secret":"hidden"}`),
					RequestID: "req-payment-safe",
				},
			))
			var typed *cloud.Error
			if !errors.As(safe, &typed) {
				t.Fatalf("safe payment error = %#v", safe)
			}
			if typed.Code != test.code || typed.Message != test.message || typed.Retryable != test.retryable {
				t.Fatalf("safe payment error = %#v, want %s/%q retryable=%v", typed, test.code, test.message, test.retryable)
			}
			if typed.Details != nil || typed.RequestID != "req-payment-safe" || strings.Contains(safe.Error(), secret) {
				t.Fatalf("safe payment error leaked details or lost request id: %#v", typed)
			}
		})
	}

	safe := cloudPaymentWailsError("cloud.test", &cloud.Error{
		Code: "UNKNOWN_PAYMENT_ERROR", Message: secret, Retryable: true,
	})
	var typed *cloud.Error
	if !errors.As(safe, &typed) || typed.Code != "CLOUD_ERROR" || typed.Retryable || strings.Contains(safe.Error(), secret) {
		t.Fatalf("unknown payment error was not fail-closed: %#v", safe)
	}
}

func TestCloudWailsErrorMapsRegistrationErrorsWithoutLeakingDetails(t *testing.T) {
	secret := "registration-sensitive-detail"
	tests := []struct {
		code    cloud.ErrorCode
		message string
	}{
		{code: cloud.ErrorCode("AUTH_RATE_LIMITED"), message: "too many authentication requests; try again later"},
		{code: cloud.ErrorCode("AUTH_CRYPTO_CONFIG_UNAVAILABLE"), message: "registration encryption is temporarily unavailable"},
		{code: cloud.ErrorCode("PHONE_INVALID"), message: "phone number is invalid"},
		{code: cloud.ErrorCode("SLIDER_CAPTCHA_REUSED"), message: "captcha verification was already used"},
		{code: cloud.ErrorCode("SMS_RATE_LIMITED"), message: "SMS requests are too frequent; try again later"},
		{code: cloud.ErrorCode("SMS_PROVIDER_UNAVAILABLE"), message: "SMS service is temporarily unavailable"},
		{code: cloud.ErrorCode("PASSWORD_INVALID"), message: "password must contain 8 to 20 characters"},
		{code: cloud.ErrorCode("SMS_CODE_EXPIRED"), message: "SMS code has expired"},
		{code: cloud.ErrorCode("SMS_CODE_ATTEMPTS_EXCEEDED"), message: "too many invalid SMS code attempts"},
		{code: cloud.ErrorCode("PHONE_ALREADY_REGISTERED"), message: "phone number is already registered"},
	}
	for _, test := range tests {
		t.Run(string(test.code), func(t *testing.T) {
			safe := cloudWailsError("cloud.test", &cloud.Error{Code: test.code, Message: secret})
			var typed *cloud.Error
			if !errors.As(safe, &typed) || typed.Code != test.code || typed.Message != test.message {
				t.Fatalf("safe registration error = %#v", safe)
			}
			if strings.Contains(safe.Error(), secret) {
				t.Fatal("safe registration error leaked cloud detail")
			}
		})
	}
}

func TestCloudLoginRealRuntimeKeepsSessionOfflineWhenMeTemporarilyFails(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "")
	var meAvailable atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/auth/login" {
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_remote","display_name":"Remote User","status":"active","created_at":"2026-07-13T03:00:00Z"},"session":{"access_token":"access-01","access_token_expires_at":"2030-07-13T05:00:00Z","refresh_token":"refresh-01","refresh_token_expires_at":"2030-08-12T04:00:00Z"}},"error":null,"request_id":"req-login","server_time":"2026-07-13T04:00:00Z"}`))
			return
		}
		if r.URL.Path == "/api/v1/me" {
			if meAvailable.Load() {
				_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_remote","display_name":"Remote User","status":"active","created_at":"2026-07-13T03:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0}},"error":null,"request_id":"req-me","server_time":"2026-07-13T04:00:02Z"}`))
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"data":null,"error":{"code":"NETWORK_UNAVAILABLE","message":"temporarily unavailable","retryable":true},"request_id":"req-me","server_time":"2026-07-13T04:00:01Z"}`))
			return
		}
		if r.URL.Path == "/api/v1/subscription-plans" {
			_, _ = w.Write([]byte(`{"data":{"plans":[{"plan_id":"plan_lite_v1","plan":"lite","display_name":"Lite","price_amount":"9.90","currency":"CNY","credit_amount":"20000.0000","period_days":30,"enabled":true,"sort_order":10,"config_version":"catalog-v1"}]},"error":null,"request_id":"req-plans","server_time":"2026-07-13T04:00:02Z"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	runtime, err := newRealCloudRuntimeWithRegistration(
		server.URL, desktopCloudTestProfilePathProvider(t.TempDir()), server.Client(),
		cloud.DeviceIdentity{DeviceID: "device-01", ClientVersion: "dev"}, cloudRegistrationDevelopment, desktopTestCloudEnvironment(server.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	view, err := app.CloudLogin(CloudLoginRequest{Phone: "13800138000", Password: "local-secret", CaptchaPassed: true})
	if err != nil {
		t.Fatal(err)
	}
	if view.Account.Status != cloud.AccountOffline || view.OperationError == nil || view.OperationError.Code != cloud.ErrorNetworkUnavailable {
		t.Fatalf("offline login view = %#v", view)
	}
	stored, ok, err := runtime.sessions.Load(app.bootContext(), actor)
	if err != nil || !ok || stored.State != cloud.SessionOffline || stored.RefreshToken != "refresh-01" {
		t.Fatalf("offline session = %#v, %t, %v", stored, ok, err)
	}
	meAvailable.Store(true)
	recovered, err := app.CloudRefreshAssets()
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Account.Status != cloud.AccountAuthenticated || recovered.Balance == nil || recovered.OperationError != nil {
		t.Fatalf("recovered view = %#v", recovered)
	}
	meAvailable.Store(false)
	failed, err := app.CloudRefreshAssets()
	if err != nil {
		t.Fatal(err)
	}
	if failed.OperationError == nil || failed.OperationError.Code != cloud.ErrorNetworkUnavailable {
		t.Fatalf("failed refresh error = %#v", failed.OperationError)
	}
	if failed.Balance != nil || failed.Subscription != nil {
		t.Fatalf("failed refresh reused cached assets: balance %#v subscription %#v", failed.Balance, failed.Subscription)
	}
}
