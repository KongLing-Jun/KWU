package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/bot"
	"kwu/internal/botruntime"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestDesktopBotRouteUsesSubscriptionCatalogInsteadOfBYOKKey(t *testing.T) {
	route, err := desktopBotRouteFromCandidates([]ModelInfo{{
		Ref: "kwu/gpt-5.4-mini", Provider: "airclaude_subscription", Model: "gpt-5.4-mini",
		BillingSource: string(modelbilling.BillingSubscriptionPool),
	}}, "kwu/gpt-5.4-mini")
	if err != nil {
		t.Fatalf("resolve bot route: %v", err)
	}
	if route.ModelRef != "kwu/gpt-5.4-mini" || route.BillingSource != modelbilling.BillingSubscriptionPool {
		t.Fatalf("bot route = %+v, want subscription route", route)
	}
}

func TestDesktopBotCatalogUsesSessionActorProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	writeProfile := func(actor userctx.ActorContext, providerName, keyName string) {
		t.Helper()
		paths := config.NewProfilePathResolver(actor)
		cfg := config.Default()
		cfg.Desktop.ProviderAccess = []string{providerName}
		cfg.Providers = append(cfg.Providers, config.ProviderEntry{
			Name: providerName, Kind: "openai", BaseURL: "https://example.invalid/v1",
			Models: []string{"chat-model"}, Default: "chat-model", APIKeyEnv: keyName,
		})
		if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
			t.Fatal(err)
		}
		if _, err := config.SetCredentialWithPaths(paths, keyName, "sk-test"); err != nil {
			t.Fatal(err)
		}
	}
	workspaceRoot := t.TempDir()
	actorA := userctx.Default(workspaceRoot)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(workspaceRoot)
	actorB.ProfileID = "profile-b"
	writeProfile(actorA, "provider-a", "PROVIDER_A_KEY")
	writeProfile(actorB, "provider-b", "PROVIDER_B_KEY")

	app.tabRuntimes["active-b"] = &WorkspaceTab{ID: "active-b", actor: actorB}
	app.activeTabID = "active-b"
	loaded, err := config.LoadForRootWithActor(workspaceRoot, actorA)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Providers) == 0 {
		t.Fatalf("profile A config has no providers: path=%s", config.NewProfilePathResolver(actorA).UserConfigPath())
	}
	models := app.automationModelsForActor(workspaceRoot, actorA)
	if len(models) != 1 || models[0].Ref != "provider-a/chat-model" {
		t.Fatalf("profile A bot catalog = %+v providers=%+v access=%+v path=%s actor=%+v, want only provider-a/chat-model", models, loaded.Providers, loaded.Desktop.ProviderAccess, config.NewProfilePathResolver(actorA).UserConfigPath(), actorA)
	}
}

func TestDesktopBotSubscriptionRouteUsesOwningDesktopActor(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	app := NewApp()
	owner := userctx.Default(t.TempDir())
	scope := cloud.BindingScope{ProfileID: owner.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	taskScope := modelbilling.TaskScope{ActorID: owner.ActorID, ProfileID: owner.ProfileID, CloudBinding: &scope}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		authState: cloud.AccountAuthenticated, bindingStatus: cloud.BindingBound,
		authorized: true, authorizedActor: owner, authorizedScope: scope,
		bindingActor: owner, bindingScope: scope, hasBinding: true,
		active: &desktopBillingWorkerProcess{scope: taskScope, done: make(chan struct{})},
	}
	const modelRef = "kwu/subscription-model"
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-test",
	}, []string{"subscription-model"})

	serviceActor := bot.ServiceActorContext(owner, owner.WorkspaceRoot)
	serviceActor.RemoteIdentity = &userctx.RemoteIdentity{
		Provider: "feishu", Domain: "feishu", UserID: "remote-user", ChatID: "remote-chat",
	}
	if _, err := app.desktopBotRouteForActor(serviceActor, owner.WorkspaceRoot, modelRef); err == nil {
		t.Fatal("bot service actor unexpectedly inherited the desktop subscription binding")
	}
	sessionActor := desktopBotSessionActor(owner, serviceActor, owner.WorkspaceRoot)
	if sessionActor.ActorID != owner.ActorID || sessionActor.ProfileID != owner.ProfileID || sessionActor.AuthMethod != owner.AuthMethod {
		t.Fatalf("session actor = %+v, want owning desktop identity", sessionActor)
	}
	if sessionActor.RemoteIdentity == nil || sessionActor.RemoteIdentity.UserID != "remote-user" {
		t.Fatalf("session remote identity = %+v, want inbound Feishu identity", sessionActor.RemoteIdentity)
	}
	route, err := app.desktopBotRouteForActor(sessionActor, owner.WorkspaceRoot, modelRef)
	if err != nil {
		t.Fatalf("resolve bot route for owning desktop actor: %v", err)
	}
	if route.ModelRef != modelRef || route.BillingSource != modelbilling.BillingSubscriptionPool {
		t.Fatalf("owner route = %+v, want subscription route", route)
	}
}

func TestDesktopBotRuntimePlanStartsSavedConnections(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Enabled = true
	cfg.Bot.Allowlist.Enabled = true
	cfg.Bot.Allowlist.FeishuUsers = []string{"ou-installer"}
	cfg.Bot.Allowlist.WeixinUsers = []string{"wx-user"}
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "feishu-feishu", Provider: "feishu", Domain: "feishu", Enabled: true},
		{ID: "feishu-lark", Provider: "feishu", Domain: "lark", Enabled: true},
		{ID: "weixin-weixin", Provider: "weixin", Domain: "weixin", Enabled: true},
	}

	plan := desktopBotRuntimePlan(cfg)
	if !plan.Start {
		t.Fatalf("plan = %+v, want start", plan)
	}
	if !plan.Enabled[bot.PlatformFeishu] || !plan.Enabled[bot.PlatformWeixin] {
		t.Fatalf("enabled = %+v, want feishu/lark and weixin platforms", plan.Enabled)
	}
}

func TestDesktopBotRuntimePlanBlocksWithoutAllowlist(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Enabled = true
	cfg.Bot.Allowlist.Enabled = true
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "feishu-lark", Provider: "feishu", Domain: "lark", Enabled: true},
	}

	plan := desktopBotRuntimePlan(cfg)
	if plan.Start || plan.Status != "blocked" {
		t.Fatalf("plan = %+v, want blocked without allowlist", plan)
	}
}

func TestDesktopBotRuntimePlanStopsWhenBotDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Bot.Enabled = false
	cfg.Bot.Allowlist.FeishuUsers = []string{"ou-installer"}
	cfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "feishu-lark", Provider: "feishu", Domain: "lark", Enabled: true},
	}

	plan := desktopBotRuntimePlan(cfg)
	if plan.Start || plan.Status != "stopped" {
		t.Fatalf("plan = %+v, want stopped when disabled", plan)
	}
}

func TestDesktopBotSessionRemembererWithRefreshRefreshesProjectTreeEvenWhenRememberFails(t *testing.T) {
	oldProjectCache := projectSessionCache
	projectSessionCache = &sessionListCache{byDir: map[string]sessionListCacheEntry{}}
	t.Cleanup(func() {
		projectSessionCache = oldProjectCache
	})

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "im.jsonl")
	token := projectSessionCache.versionToken()
	projectSessionCache.put(dir, []agent.SessionInfo{{Path: sessionPath}}, nil, nil, nil, token)
	if _, _, _, _, ok := projectSessionCache.get(dir); !ok {
		t.Fatalf("expected primed project tree cache")
	}

	app := NewApp()
	var refreshes int
	app.projectTreeChangedHook = func() {
		refreshes++
	}
	rememberErr := errors.New("remember failed")
	onSessionReady := desktopBotSessionRemembererWithRefresh(
		func(msg bot.InboundMessage, sessionID string) error {
			if sessionID != "path:"+sessionPath {
				t.Fatalf("remember sessionID = %q, want path:%s", sessionID, sessionPath)
			}
			return rememberErr
		},
		app.emitProjectTreeChanged,
	)

	err := onSessionReady(bot.InboundMessage{Platform: bot.PlatformFeishu}, "path:"+sessionPath)

	if !errors.Is(err, rememberErr) {
		t.Fatalf("callback err = %v, want %v", err, rememberErr)
	}
	if refreshes != 1 {
		t.Fatalf("project tree refreshes = %d, want 1", refreshes)
	}
	if got := app.ProjectTreeRevision(); got != 1 {
		t.Fatalf("project tree revision = %d, want 1", got)
	}
	if _, _, _, _, ok := projectSessionCache.get(dir); ok {
		t.Fatalf("project tree cache survived bot session update")
	}
}

func TestDesktopBotRuntimeConfigUsesUserBotSettings(t *testing.T) {
	isolateDesktopUserDirs(t)

	userCfg := config.LoadForEdit(config.UserConfigPath())
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.FeishuUsers = []string{"ou-installer"}
	userCfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "feishu-lark", Provider: "feishu", Domain: "lark", Enabled: true, Status: "connected"},
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
[bot]
enabled = false
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	plan := desktopBotRuntimePlan(got)
	if !plan.Start || !plan.Enabled[bot.PlatformFeishu] {
		t.Fatalf("desktop runtime plan = %+v, want user-level Lark connection to start", plan)
	}
}

func TestDesktopBotRuntimeConfigLoadsAllSavedCredentialsAfterRestart(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Cleanup(func() {
		_ = os.Unsetenv("FEISHU_BOT_APP_SECRET")
		_ = os.Unsetenv("LARK_BOT_APP_SECRET")
	})

	userCfg := config.Default()
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.FeishuUsers = []string{"ou-feishu-installer", "ou-lark-installer"}
	userCfg.Bot.Allowlist.WeixinUsers = []string{"wx-installer"}
	userCfg.Bot.Feishu.Enabled = true
	userCfg.Bot.Weixin.Enabled = true
	userCfg.Bot.Weixin.AccountID = "weixin-account"
	userCfg.Bot.Weixin.TokenEnv = "WEIXIN_BOT_TOKEN"
	userCfg.Bot.Connections = []config.BotConnectionConfig{
		{
			ID:       "feishu-feishu",
			Provider: "feishu",
			Domain:   "feishu",
			Enabled:  true,
			Status:   "connected",
			Credential: config.BotConnectionCredential{
				AppID:        "cli-feishu",
				AppSecretEnv: "FEISHU_BOT_APP_SECRET",
			},
		},
		{
			ID:       "feishu-lark",
			Provider: "feishu",
			Domain:   "lark",
			Enabled:  true,
			Status:   "connected",
			Credential: config.BotConnectionCredential{
				AppID:        "cli-lark",
				AppSecretEnv: "LARK_BOT_APP_SECRET",
			},
		},
		{
			ID:       "weixin-weixin",
			Provider: "weixin",
			Domain:   "weixin",
			Enabled:  true,
			Status:   "connected",
			Credential: config.BotConnectionCredential{
				AccountID: "weixin-account",
				TokenEnv:  "WEIXIN_BOT_TOKEN",
			},
		},
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(config.UserCredentialsPath()), 0o755); err != nil {
		t.Fatalf("create credentials dir: %v", err)
	}
	if err := os.WriteFile(config.UserCredentialsPath(), []byte("FEISHU_BOT_APP_SECRET=feishu-secret\nLARK_BOT_APP_SECRET=lark-secret\n"), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	weixinAccountPath := filepath.Join(config.MemoryUserDir(), "weixin", "accounts", "weixin-account.json")
	if err := os.MkdirAll(filepath.Dir(weixinAccountPath), 0o700); err != nil {
		t.Fatalf("create weixin account dir: %v", err)
	}
	if err := os.WriteFile(weixinAccountPath, []byte(`{"token":"weixin-token","base_url":"https://ilinkai.weixin.qq.com","user_id":"wx-installer"}`), 0o600); err != nil {
		t.Fatalf("write weixin account: %v", err)
	}
	_ = os.Unsetenv("FEISHU_BOT_APP_SECRET")
	_ = os.Unsetenv("LARK_BOT_APP_SECRET")

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	views := botConnectionViews(got.Bot.Connections)
	if len(views) != 3 {
		t.Fatalf("connection views = %+v, want Feishu, Lark, and Weixin", views)
	}
	for _, view := range views {
		if !view.Credential.SecretSet {
			t.Fatalf("connection %s credential = %+v, want saved credential loaded after restart", view.ID, view.Credential)
		}
	}
	plan := desktopBotRuntimePlan(got)
	if !plan.Start || !plan.Enabled[bot.PlatformFeishu] || !plan.Enabled[bot.PlatformWeixin] {
		t.Fatalf("desktop runtime plan = %+v, want saved Feishu/Lark/Weixin connections to start", plan)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bindings := botruntime.AdapterBindings(got, plan.Enabled, nil, logger)
	if len(bindings) != 3 {
		t.Fatalf("adapter bindings = %+v, want one per saved connection", bindings)
	}
}

func TestDesktopBotRuntimeConfigUsesActiveProfileCredentials(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	key := "FEISHU_BOT_PROFILE_RUNTIME_SECRET"
	t.Setenv(key, "")
	os.Unsetenv(key)

	defaultPaths := config.NewProfilePathResolver(userctx.Default(project))
	actorA := userctx.Default(project)
	actorA.ProfileID = "profile-a"
	pathsA := config.NewProfilePathResolver(actorA)
	actorB := userctx.Default(project)
	actorB.ProfileID = "profile-b"
	pathsB := config.NewProfilePathResolver(actorB)

	writeRuntimeConfig := func(paths config.ProfilePathResolver) {
		t.Helper()
		cfg := config.Default()
		cfg.Bot.Enabled = true
		cfg.Bot.Allowlist.Enabled = true
		cfg.Bot.Allowlist.FeishuUsers = []string{"ou-installer"}
		cfg.Bot.Connections = []config.BotConnectionConfig{{
			ID:       "feishu-lark",
			Provider: "feishu",
			Domain:   "lark",
			Enabled:  true,
			Status:   "connected",
			Credential: config.BotConnectionCredential{
				AppID:        "cli-lark",
				AppSecretEnv: key,
			},
		}}
		if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
			t.Fatalf("save profile bot config: %v", err)
		}
	}
	writeCredential := func(paths config.ProfilePathResolver, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(paths.UserCredentialsPath()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.UserCredentialsPath(), []byte(key+"="+value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeRuntimeConfig(pathsA)
	writeRuntimeConfig(pathsB)
	writeCredential(defaultPaths, "default-secret")
	writeCredential(pathsA, "profile-a-secret")
	writeCredential(pathsB, "profile-b-secret")

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"a": {ID: "a", WorkspaceRoot: project, actor: actorA},
		"b": {ID: "b", WorkspaceRoot: project, actor: actorB},
	}
	app.activeTabID = "a"
	cfgA, err := app.loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load profile A bot config: %v", err)
	}
	if got := os.Getenv(key); got != "profile-a-secret" {
		t.Fatalf("profile A runtime env = %q, want profile-a-secret", got)
	}
	if views := botConnectionViewsWithPaths(cfgA.Bot.Connections, pathsA); len(views) != 1 || !views[0].Credential.SecretSet {
		t.Fatalf("profile A connection views = %+v, want secret set", views)
	}

	app.activeTabID = "b"
	cfgB, err := app.loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load profile B bot config: %v", err)
	}
	if got := os.Getenv(key); got != "profile-b-secret" {
		t.Fatalf("profile B runtime env = %q, want profile-b-secret", got)
	}
	if views := botConnectionViewsWithPaths(cfgB.Bot.Connections, pathsB); len(views) != 1 || !views[0].Credential.SecretSet {
		t.Fatalf("profile B connection views = %+v, want secret set", views)
	}
}

func TestDesktopBotRuntimeUsesActiveProfileWeixinSavedAccount(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/getupdates" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ret":0,"errcode":0,"msgs":[]}`))
	}))
	defer server.Close()

	actorA := userctx.Default(project)
	actorA.ProfileID = "profile-a"
	pathsA := config.NewProfilePathResolver(actorA)
	actorB := userctx.Default(project)
	actorB.ProfileID = "profile-b"

	accountPath := filepath.Join(pathsA.MemoryUserDir(), "weixin", "accounts", "wx-account.json")
	if err := os.MkdirAll(filepath.Dir(accountPath), 0o700); err != nil {
		t.Fatalf("create profile A account dir: %v", err)
	}
	if err := os.WriteFile(accountPath, []byte(`{"token":"token-a","base_url":"`+server.URL+`","user_id":"user-a"}`), 0o600); err != nil {
		t.Fatalf("write profile A account: %v", err)
	}

	cfg := config.Default()
	cfg.Bot.Enabled = true
	cfg.Bot.Allowlist.AllowAll = true
	cfg.Bot.Weixin.Enabled = true
	cfg.Bot.Weixin.AccountID = "wx-account"
	cfg.Bot.Weixin.TokenEnv = ""
	cfg.Bot.Weixin.APIBase = ""

	runtimeB := newDesktopBotRuntime()
	err := runtimeB.apply(context.Background(), cfg, project, bot.ServiceActorContext(actorB, project), nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no saved weixin account") {
		t.Fatalf("profile B runtime error = %v, want missing saved account", err)
	}

	runtimeA := newDesktopBotRuntime()
	if err := runtimeA.apply(context.Background(), cfg, project, bot.ServiceActorContext(actorA, project), nil, nil, nil); err != nil {
		t.Fatalf("profile A runtime apply: %v", err)
	}
	runtimeA.stop("stopped", "test complete")
}

func TestDesktopBotRuntimeMigratesLegacyProjectBotSettings(t *testing.T) {
	isolateDesktopUserDirs(t)

	userCfg := config.Default()
	if err := userCfg.SetDesktopAppearance("dark", "graphite"); err != nil {
		t.Fatalf("set desktop appearance: %v", err)
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
[bot]
enabled = true

[bot.allowlist]
enabled = true
feishu_users = ["ou-legacy"]

[[bot.connections]]
id = "feishu-lark"
provider = "feishu"
domain = "lark"
label = "Lark"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	if !got.Bot.Enabled || len(got.Bot.Connections) != 1 || got.Bot.Connections[0].ID != "feishu-lark" {
		t.Fatalf("desktop bot config = %+v, want migrated legacy Lark connection", got.Bot)
	}

	persisted := config.LoadForEdit(config.UserConfigPath())
	if !persisted.Bot.Enabled || len(persisted.Bot.Connections) != 1 || persisted.Bot.Connections[0].ID != "feishu-lark" {
		t.Fatalf("persisted bot config = %+v, want migrated legacy Lark connection", persisted.Bot)
	}
	if persisted.DesktopTheme() != "dark" {
		t.Fatalf("desktop theme = %q, want preserved user preference", persisted.DesktopTheme())
	}
}

func TestDesktopBotRuntimePersistsLegacyProjectBotWhenUserConfigMissing(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
[desktop]
theme = "dark"

[bot]
enabled = true

[bot.allowlist]
enabled = true
feishu_users = ["ou-legacy"]

[[bot.connections]]
id = "feishu-lark"
provider = "feishu"
domain = "lark"
label = "Lark"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	if !got.Bot.Enabled || len(got.Bot.Connections) != 1 || got.Bot.Connections[0].ID != "feishu-lark" {
		t.Fatalf("desktop bot config = %+v, want migrated legacy Lark connection", got.Bot)
	}

	persisted := config.LoadForEdit(config.UserConfigPath())
	if !persisted.Bot.Enabled || len(persisted.Bot.Connections) != 1 || persisted.Bot.Connections[0].ID != "feishu-lark" {
		t.Fatalf("persisted bot config = %+v, want migrated legacy Lark connection", persisted.Bot)
	}
	if persisted.DesktopTheme() == "dark" {
		t.Fatal("legacy project desktop theme should not be persisted during bot-only migration")
	}
}

func TestDesktopSettingsBotMigrationPersistsOnlyBotBeforeFirstEdit(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
[desktop]
theme = "dark"
close_behavior = "quit"

[bot]
enabled = true

[bot.allowlist]
enabled = true
feishu_users = ["ou-legacy"]

[[bot.connections]]
id = "feishu-lark"
provider = "feishu"
domain = "lark"
label = "Lark"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	settings := NewApp().Settings()
	if !settings.Bot.Enabled || len(settings.Bot.Connections) != 1 || settings.Bot.Connections[0].ID != "feishu-lark" {
		t.Fatalf("settings bot = %+v, want migrated legacy Lark connection", settings.Bot)
	}
	if settings.DesktopTheme != "dark" || settings.CloseBehavior != "quit" {
		t.Fatalf("settings desktop prefs = theme:%q close:%q, want legacy seed visible before first edit", settings.DesktopTheme, settings.CloseBehavior)
	}

	persisted := config.LoadForEdit(config.UserConfigPath())
	if persisted.DesktopTheme() == "dark" || persisted.DesktopCloseBehavior() == "quit" {
		t.Fatalf("persisted desktop prefs = theme:%q close:%q, want bot-only migration", persisted.DesktopTheme(), persisted.DesktopCloseBehavior())
	}
}

func TestDesktopBotRuntimeMigrationDoesNotOverwriteUserBotSettings(t *testing.T) {
	isolateDesktopUserDirs(t)

	userCfg := config.Default()
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.WeixinUsers = []string{"wx-user"}
	userCfg.Bot.Connections = []config.BotConnectionConfig{
		{ID: "weixin-weixin", Provider: "weixin", Domain: "weixin", Enabled: true, Status: "connected"},
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
[bot]
enabled = true

[bot.allowlist]
enabled = true
feishu_users = ["ou-legacy"]

[[bot.connections]]
id = "feishu-lark"
provider = "feishu"
domain = "lark"
enabled = true
status = "connected"
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	got, err := NewApp().loadDesktopBotConfig()
	if err != nil {
		t.Fatalf("load desktop bot config: %v", err)
	}
	if len(got.Bot.Connections) != 1 || got.Bot.Connections[0].ID != "weixin-weixin" {
		t.Fatalf("desktop bot config = %+v, want existing user WeChat connection", got.Bot)
	}
}

func TestSummarizeBotRuntimeErrorsCapsOutput(t *testing.T) {
	got := summarizeBotRuntimeErrors([]error{
		errors.New("first"),
		nil,
		errors.New("second"),
		errors.New("third"),
		errors.New("fourth"),
	})

	for _, want := range []string{"first", "second", "third", "1 more"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "fourth") {
		t.Fatalf("summary = %q, should cap extra errors", got)
	}
}
