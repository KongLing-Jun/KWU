package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/bot"
	"kwu/internal/config"
	"kwu/internal/userctx"
)

func TestDesktopAutomationDeliveryAdapterResolvesOpaqueFeishuTarget(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "alice", ProfileID: "work", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	mapping := config.BotConnectionSessionMapping{
		RemoteID: "oc-secret-chat", SessionID: "topic:daily", ChatType: "group", UserID: "ou-user",
		Scope: "project", WorkspaceRoot: actor.WorkspaceRoot,
	}
	conn := config.BotConnectionConfig{
		ID: "feishu-team", Provider: "feishu", Domain: "feishu", Enabled: true, Status: "connected",
		Credential:      config.BotConnectionCredential{AppID: "app-id", AppSecretEnv: "SECRET_ENV"},
		SessionMappings: []config.BotConnectionSessionMapping{mapping},
	}
	snapshot := desktopAutomationDeliverySnapshot{
		Actor: actor, WorkspaceRoot: actor.WorkspaceRoot,
		Paths:         config.NewProfilePathResolver(actor),
		Config:        &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}},
		feishuSecrets: map[string]string{conn.ID: "profile-secret"},
	}

	var sentTo, sentBody string
	var hadDeadline bool
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) { return snapshot, nil },
		desktopAutomationDeliverySenders{
			feishu: func(ctx context.Context, got config.FeishuBotConfig, remoteID, body, secret string) (bot.SendResult, error) {
				_, hadDeadline = ctx.Deadline()
				if got.AppID != "app-id" || got.AppSecretEnv != "SECRET_ENV" || got.Domain != "feishu" {
					t.Fatalf("resolved Feishu config = %+v", got)
				}
				if secret != "profile-secret" {
					t.Fatalf("resolved profile secret = %q", secret)
				}
				sentTo, sentBody = remoteID, body
				return bot.SendResult{MessageID: "om-1"}, nil
			},
		},
		250*time.Millisecond,
	)
	ref := desktopAutomationDeliveryTargetRef(actor, actor.WorkspaceRoot, conn, mapping)
	if strings.Contains(ref, mapping.RemoteID) || strings.Contains(ref, conn.ID) {
		t.Fatalf("target ref leaks routing identifiers: %q", ref)
	}

	request := desktopAutomationDeliveryRequest(actor, "feishu", ref)
	request.Body = "daily summary"
	result, err := adapter.Deliver(context.Background(), request)
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if result.ProviderMessageID != "om-1" || sentTo != mapping.RemoteID || sentBody != "daily summary" || !hadDeadline {
		t.Fatalf("result=%+v sentTo=%q sentBody=%q hadDeadline=%v", result, sentTo, sentBody, hadDeadline)
	}
}

func TestDesktopAutomationDeliveryAdapterResolvesLarkAndWeixinILink(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "alice", ProfileID: "work", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	tests := []struct {
		name       string
		targetType string
		connection config.BotConnectionConfig
		wantDomain string
	}{
		{
			name: "lark", targetType: "lark", wantDomain: "lark",
			connection: config.BotConnectionConfig{ID: "lark-team", Provider: "feishu", Domain: "lark", Enabled: true, Status: "connected",
				SessionMappings: []config.BotConnectionSessionMapping{{RemoteID: "oc-lark", Scope: "global"}}},
		},
		{
			name: "weixin", targetType: "weixin_ilink",
			connection: config.BotConnectionConfig{ID: "weixin-team", Provider: "weixin", Domain: "weixin", Enabled: true, Status: "connected",
				Credential:      config.BotConnectionCredential{AccountID: "work-account", TokenEnv: "WEIXIN_OTHER_PROFILE_TOKEN"},
				SessionMappings: []config.BotConnectionSessionMapping{{RemoteID: "wx-chat", Scope: "global"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{tt.connection}}}
			snapshot := desktopAutomationDeliverySnapshot{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot, Paths: config.NewProfilePathResolver(actor), Config: cfg, feishuSecrets: map[string]string{tt.connection.ID: "profile-secret"}}
			called := ""
			adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
				func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) { return snapshot, nil },
				desktopAutomationDeliverySenders{
					feishu: func(_ context.Context, got config.FeishuBotConfig, remoteID, _, _ string) (bot.SendResult, error) {
						called = "feishu:" + got.Domain + ":" + remoteID
						return bot.SendResult{MessageID: "feishu-message"}, nil
					},
					weixin: func(_ context.Context, got config.WeixinBotConfig, paths config.ProfilePathResolver, remoteID, _ string) (bot.SendResult, error) {
						if got.TokenEnv != "" || got.AccountID != "work-account" {
							t.Fatalf("profile-safe Weixin config = %+v", got)
						}
						called = "weixin:" + paths.ProfileID() + ":" + remoteID
						return bot.SendResult{MessageID: "weixin-message"}, nil
					},
				},
				time.Second,
			)
			mapping := tt.connection.SessionMappings[0]
			ref := desktopAutomationDeliveryTargetRef(actor, actor.WorkspaceRoot, tt.connection, mapping)
			_, err := adapter.Deliver(context.Background(), desktopAutomationDeliveryRequest(actor, tt.targetType, ref))
			if err != nil {
				t.Fatalf("Deliver() error = %v", err)
			}
			if tt.targetType == "lark" && called != "feishu:"+tt.wantDomain+":oc-lark" {
				t.Fatalf("called = %q", called)
			}
			if tt.targetType == "weixin_ilink" && called != "weixin:work:wx-chat" {
				t.Fatalf("called = %q", called)
			}
		})
	}
}

func TestDesktopAutomationDeliveryAdapterResolvesWeComSeparatelyFromWeixinILink(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "alice", ProfileID: "work", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	conn := config.BotConnectionConfig{
		ID: "wecom-team", Provider: "wecom", Domain: "wecom", Label: "Enterprise notifications",
		Enabled: true, Status: "connected", WorkspaceRoot: actor.WorkspaceRoot,
		Credential: config.BotConnectionCredential{TokenEnv: "WECOM_TEAM_WEBHOOK_KEY"},
	}
	snapshot := desktopAutomationDeliverySnapshot{
		Actor: actor, WorkspaceRoot: actor.WorkspaceRoot, Paths: config.NewProfilePathResolver(actor),
		Config:       &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}},
		wecomSecrets: map[string]string{conn.ID: "profile-wecom-key"},
	}
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) { return snapshot, nil },
		desktopAutomationDeliverySenders{wecom: func(_ context.Context, key, body string) (bot.SendResult, error) {
			if key != "profile-wecom-key" || body != "daily summary" {
				t.Fatalf("WeCom send = key %q body %q", key, body)
			}
			return bot.SendResult{}, nil
		}}, time.Second,
	)
	views, err := desktopAutomationDeliveryCatalog(context.Background(), adapter, actor.WorkspaceRoot, desktopAutomationPrincipal(actor))
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(views) != 1 || views[0].Type != "wecom" || views[0].Provider != "wecom" || !views[0].Available {
		t.Fatalf("WeCom catalog = %+v", views)
	}
	payload, _ := json.Marshal(views)
	if strings.Contains(string(payload), "profile-wecom-key") || strings.Contains(string(payload), conn.Credential.TokenEnv) || strings.Contains(string(payload), conn.ID) {
		t.Fatalf("WeCom catalog leaks configuration: %s", payload)
	}
	request := desktopAutomationDeliveryRequest(actor, "wecom", views[0].Ref)
	request.Body = "daily summary"
	if _, err := adapter.Deliver(context.Background(), request); err != nil {
		t.Fatalf("Deliver WeCom: %v", err)
	}
}

func TestDesktopAutomationEngineRegistersWeComDeliveryAdapterType(t *testing.T) {
	want := desktopAutomationDeliveryTypeWeCom
	for _, kind := range desktopAutomationDeliveryAdapterTypes() {
		if kind == want {
			return
		}
	}
	t.Fatalf("desktop automation delivery adapter types = %v, missing %q", desktopAutomationDeliveryAdapterTypes(), want)
}

func TestValidateBotConnectionCredentialReferencesRejectsWeComSecrets(t *testing.T) {
	tests := []string{
		"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=secret",
		"raw-webhook-key-with-dashes",
		"",
	}
	for _, reference := range tests {
		err := validateBotConnectionCredentialReferences([]config.BotConnectionConfig{{
			ID: "wecom-team", Provider: "wecom", Domain: "wecom",
			Credential: config.BotConnectionCredential{TokenEnv: reference},
		}})
		if err == nil {
			t.Errorf("credential reference %q was accepted", reference)
		}
	}
	if err := validateBotConnectionCredentialReferences([]config.BotConnectionConfig{{
		ID: "wecom-team", Provider: "wecom", Domain: "wecom",
		Credential: config.BotConnectionCredential{TokenEnv: "WECOM_TEAM_WEBHOOK_KEY"},
	}}); err != nil {
		t.Fatalf("valid credential reference rejected: %v", err)
	}
}

func TestDesktopAutomationDeliveryAdapterFailsClosedForRevokedOrCrossWorkspaceWeCom(t *testing.T) {
	actorA := userctx.ActorContext{ActorID: "alice", ProfileID: "work", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	actorB := userctx.ActorContext{ActorID: "alice", ProfileID: "work", WorkspaceRoot: `C:\work\beta`}.Normalize()
	actorOtherProfile := userctx.ActorContext{ActorID: "alice", ProfileID: "other", WorkspaceRoot: actorA.WorkspaceRoot}.Normalize()
	base := config.BotConnectionConfig{
		ID: "wecom-team", Provider: "wecom", Domain: "wecom", Enabled: true, Status: "connected",
		WorkspaceRoot: actorA.WorkspaceRoot, Credential: config.BotConnectionCredential{TokenEnv: "WECOM_TEAM_WEBHOOK_KEY"},
	}
	disabled := base
	disabled.Enabled = false
	ref := desktopAutomationDeliveryTargetRef(actorA, actorA.WorkspaceRoot, base, desktopAutomationDeliveryMappings(base)[0])
	tests := []struct {
		name     string
		request  userctx.ActorContext
		snapshot desktopAutomationDeliverySnapshot
	}{
		{
			name: "disabled", request: actorA,
			snapshot: desktopAutomationDeliverySnapshot{Actor: actorA, WorkspaceRoot: actorA.WorkspaceRoot, Paths: config.NewProfilePathResolver(actorA),
				Config:       &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{disabled}}},
				wecomSecrets: map[string]string{base.ID: "profile-wecom-key"}},
		},
		{
			name: "credential removed", request: actorA,
			snapshot: desktopAutomationDeliverySnapshot{Actor: actorA, WorkspaceRoot: actorA.WorkspaceRoot, Paths: config.NewProfilePathResolver(actorA),
				Config: &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{base}}}},
		},
		{
			name: "other workspace", request: actorB,
			snapshot: desktopAutomationDeliverySnapshot{Actor: actorB, WorkspaceRoot: actorB.WorkspaceRoot, Paths: config.NewProfilePathResolver(actorB),
				Config:       &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{base}}},
				wecomSecrets: map[string]string{base.ID: "profile-wecom-key"}},
		},
		{
			name: "other profile", request: actorOtherProfile,
			snapshot: desktopAutomationDeliverySnapshot{Actor: actorOtherProfile, WorkspaceRoot: actorOtherProfile.WorkspaceRoot, Paths: config.NewProfilePathResolver(actorOtherProfile),
				Config:       &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{base}}},
				wecomSecrets: map[string]string{base.ID: "profile-wecom-key"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
				func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) {
					return tt.snapshot, nil
				},
				desktopAutomationDeliverySenders{wecom: func(context.Context, string, string) (bot.SendResult, error) {
					called = true
					return bot.SendResult{}, nil
				}}, time.Second,
			)
			_, err := adapter.Deliver(context.Background(), desktopAutomationDeliveryRequest(tt.request, "wecom", ref))
			assertDesktopDeliveryFailure(t, err, automation.DeliveryFailureTargetRevoked)
			if called {
				t.Fatal("WeCom sender called after target revocation or owner scope mismatch")
			}
		})
	}
}

func TestDesktopAutomationDeliveryAdapterFailsClosedForRawOrCrossProfileRefs(t *testing.T) {
	actorA := userctx.ActorContext{ActorID: "alice", ProfileID: "profile-a", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	actorB := userctx.ActorContext{ActorID: "alice", ProfileID: "profile-b", WorkspaceRoot: actorA.WorkspaceRoot}.Normalize()
	mapping := config.BotConnectionSessionMapping{RemoteID: "oc-secret-chat", Scope: "global"}
	conn := config.BotConnectionConfig{ID: "feishu-team", Provider: "feishu", Domain: "feishu", Enabled: true, Status: "connected", SessionMappings: []config.BotConnectionSessionMapping{mapping}}
	snapshot := desktopAutomationDeliverySnapshot{Actor: actorB, WorkspaceRoot: actorB.WorkspaceRoot, Paths: config.NewProfilePathResolver(actorB), Config: &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}}}
	called := false
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) { return snapshot, nil },
		desktopAutomationDeliverySenders{feishu: func(context.Context, config.FeishuBotConfig, string, string, string) (bot.SendResult, error) {
			called = true
			return bot.SendResult{}, nil
		}},
		time.Second,
	)

	refs := []string{
		mapping.RemoteID,
		"https://example.test/webhook/token",
		"FEISHU_BOT_APP_SECRET",
		"token=super-secret",
		desktopAutomationDeliveryTargetRef(actorA, actorA.WorkspaceRoot, conn, mapping),
	}
	for _, ref := range refs {
		_, err := adapter.Deliver(context.Background(), desktopAutomationDeliveryRequest(actorB, "feishu", ref))
		assertDesktopDeliveryFailure(t, err, automation.DeliveryFailureTargetRevoked)
	}
	if called {
		t.Fatal("sender called for an untrusted target ref")
	}
}

func TestDesktopAutomationDeliveryAdapterUsesRunOwnerInsteadOfActiveWorkspace(t *testing.T) {
	runActor := userctx.ActorContext{ActorID: "alice", ProfileID: "scheduled", WorkspaceRoot: `C:\work\scheduled`}.Normalize()
	activeActor := userctx.ActorContext{ActorID: "bob", ProfileID: "active", WorkspaceRoot: `C:\work\active`}.Normalize()
	mapping := config.BotConnectionSessionMapping{RemoteID: "oc-scheduled", Scope: "project", WorkspaceRoot: runActor.WorkspaceRoot}
	conn := config.BotConnectionConfig{ID: "feishu-scheduled", Provider: "feishu", Domain: "feishu", Enabled: true, Status: "connected", SessionMappings: []config.BotConnectionSessionMapping{mapping}}
	ref := desktopAutomationDeliveryTargetRef(runActor, runActor.WorkspaceRoot, conn, mapping)
	called := false
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(_ context.Context, workspaceRoot, principal string) (desktopAutomationDeliverySnapshot, error) {
			if workspaceRoot == activeActor.WorkspaceRoot || principal == desktopAutomationPrincipal(activeActor) {
				t.Fatal("loader received active route instead of immutable run owner")
			}
			if workspaceRoot != runActor.WorkspaceRoot || principal != desktopAutomationPrincipal(runActor) {
				t.Fatalf("loader owner = %q / %q", workspaceRoot, principal)
			}
			return desktopAutomationDeliverySnapshot{
				Actor: runActor, WorkspaceRoot: runActor.WorkspaceRoot, Paths: config.NewProfilePathResolver(runActor),
				Config: &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}}, feishuSecrets: map[string]string{conn.ID: "profile-secret"},
			}, nil
		},
		desktopAutomationDeliverySenders{feishu: func(context.Context, config.FeishuBotConfig, string, string, string) (bot.SendResult, error) {
			called = true
			return bot.SendResult{}, nil
		}},
		time.Second,
	)
	if _, err := adapter.Deliver(context.Background(), desktopAutomationDeliveryRequest(runActor, "feishu", ref)); err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !called {
		t.Fatal("sender was not called")
	}
}

func TestDesktopAutomationDeliveryAdapterUsesGlobalWorkspaceForWorkspaceOptionalTask(t *testing.T) {
	globalRoot := globalTabWorkspaceRoot()
	actor := userctx.ActorContext{ActorID: "alice", ProfileID: "global", WorkspaceRoot: globalRoot}.Normalize()
	mapping := config.BotConnectionSessionMapping{RemoteID: "oc-global", Scope: "global"}
	conn := config.BotConnectionConfig{
		ID: "feishu-global", Provider: "feishu", Domain: "feishu", Enabled: true, Status: "connected",
		SessionMappings: []config.BotConnectionSessionMapping{mapping},
	}
	ref := desktopAutomationDeliveryTargetRef(actor, globalRoot, conn, mapping)
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(_ context.Context, workspaceRoot, principal string) (desktopAutomationDeliverySnapshot, error) {
			if workspaceRoot != globalRoot || principal != desktopAutomationPrincipal(actor) {
				t.Fatalf("global loader owner = %q / %q", workspaceRoot, principal)
			}
			return desktopAutomationDeliverySnapshot{
				Actor: actor, WorkspaceRoot: globalRoot, Paths: config.NewProfilePathResolver(actor),
				Config:        &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}},
				feishuSecrets: map[string]string{conn.ID: "profile-secret"},
			}, nil
		},
		desktopAutomationDeliverySenders{feishu: func(context.Context, config.FeishuBotConfig, string, string, string) (bot.SendResult, error) {
			return bot.SendResult{MessageID: "global-message"}, nil
		}},
		time.Second,
	)
	request := desktopAutomationDeliveryRequest(actor, desktopAutomationDeliveryTypeFeishu, ref)
	request.WorkspaceRoot = ""
	result, err := adapter.Deliver(context.Background(), request)
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if result.ProviderMessageID != "global-message" {
		t.Fatalf("ProviderMessageID = %q", result.ProviderMessageID)
	}
}

func TestDesktopAutomationDeliveryAdapterRejectsSnapshotOwnerMismatch(t *testing.T) {
	requestActor := userctx.ActorContext{ActorID: "alice", ProfileID: "profile-a", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	loadedActor := userctx.ActorContext{ActorID: "alice", ProfileID: "profile-b", WorkspaceRoot: `C:\work\beta`}.Normalize()
	mapping := config.BotConnectionSessionMapping{RemoteID: "oc-secret-chat", Scope: "global"}
	conn := config.BotConnectionConfig{ID: "feishu-team", Provider: "feishu", Domain: "feishu", Enabled: true, Status: "connected", SessionMappings: []config.BotConnectionSessionMapping{mapping}}
	snapshot := desktopAutomationDeliverySnapshot{Actor: loadedActor, WorkspaceRoot: loadedActor.WorkspaceRoot, Paths: config.NewProfilePathResolver(loadedActor), Config: &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}}}
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) { return snapshot, nil },
		desktopAutomationDeliverySenders{}, time.Second,
	)
	ref := desktopAutomationDeliveryTargetRef(requestActor, requestActor.WorkspaceRoot, conn, mapping)
	_, err := adapter.Deliver(context.Background(), desktopAutomationDeliveryRequest(requestActor, "feishu", ref))
	failure := assertDesktopDeliveryFailure(t, err, automation.DeliveryFailureTargetRevoked)
	if failure.Code != "owner_mismatch" {
		t.Fatalf("failure code = %q, want owner_mismatch", failure.Code)
	}
}

func TestDesktopAutomationDeliveryAdapterFailsClosedWhenConnectionRevoked(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "alice", ProfileID: "work", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	mapping := config.BotConnectionSessionMapping{RemoteID: "wx-chat", Scope: "global"}
	base := config.BotConnectionConfig{ID: "weixin-team", Provider: "weixin", Domain: "weixin", Enabled: true, Status: "connected", SessionMappings: []config.BotConnectionSessionMapping{mapping}}
	ref := desktopAutomationDeliveryTargetRef(actor, actor.WorkspaceRoot, base, mapping)
	for _, mutate := range []func(*config.BotConnectionConfig){
		func(conn *config.BotConnectionConfig) { conn.Enabled = false },
		func(conn *config.BotConnectionConfig) { conn.Status = "disconnected" },
		func(conn *config.BotConnectionConfig) { conn.SessionMappings = nil },
	} {
		conn := base
		mutate(&conn)
		snapshot := desktopAutomationDeliverySnapshot{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot, Paths: config.NewProfilePathResolver(actor), Config: &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}}}
		adapter := newDesktopAutomationDeliveryAdapterWithDependencies(func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) { return snapshot, nil }, desktopAutomationDeliverySenders{}, time.Second)
		_, err := adapter.Deliver(context.Background(), desktopAutomationDeliveryRequest(actor, "weixin_ilink", ref))
		assertDesktopDeliveryFailure(t, err, automation.DeliveryFailureTargetRevoked)
	}
}

func TestDesktopAutomationDeliveryAdapterSanitizesAndClassifiesSenderErrors(t *testing.T) {
	actor := userctx.Default(`C:\work\alpha`)
	mapping := config.BotConnectionSessionMapping{RemoteID: "wx-secret-chat", Scope: "global"}
	conn := config.BotConnectionConfig{ID: "weixin-team", Provider: "weixin", Enabled: true, Status: "connected", Credential: config.BotConnectionCredential{TokenEnv: "WEIXIN_PRIVATE_ENV"}, SessionMappings: []config.BotConnectionSessionMapping{mapping}}
	snapshot := desktopAutomationDeliverySnapshot{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot, Paths: config.NewProfilePathResolver(actor), Config: &config.Config{Bot: config.BotConfig{Connections: []config.BotConnectionConfig{conn}}}}
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error) { return snapshot, nil },
		desktopAutomationDeliverySenders{weixin: func(context.Context, config.WeixinBotConfig, config.ProfilePathResolver, string, string) (bot.SendResult, error) {
			return bot.SendResult{}, errors.New("HTTP 429 body=token-super-secret env=WEIXIN_PRIVATE_ENV")
		}},
		time.Second,
	)
	ref := desktopAutomationDeliveryTargetRef(actor, actor.WorkspaceRoot, conn, mapping)
	_, err := adapter.Deliver(context.Background(), desktopAutomationDeliveryRequest(actor, "weixin_ilink", ref))
	failure := assertDesktopDeliveryFailure(t, err, automation.DeliveryFailureRateLimited)
	if failure.Code != "provider_rate_limited" || strings.Contains(err.Error(), "token-super-secret") || strings.Contains(err.Error(), "WEIXIN_PRIVATE_ENV") {
		t.Fatalf("unsafe or unclassified error: %#v / %v", failure, err)
	}
	metadata := failure.DeliveryFailureMetadata()
	if metadata.Class != automation.DeliveryFailureRateLimited || metadata.Code != "provider_rate_limited" || metadata.Message != failure.Message {
		t.Fatalf("DeliveryFailureMetadata() = %+v", metadata)
	}
}

func TestDesktopAutomationDeliveryCatalogIsOwnerScopedAndSecretFree(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "alice", ProfileID: "work", WorkspaceRoot: `C:\work\alpha`}.Normalize()
	connections := []config.BotConnectionConfig{
		{
			ID: "feishu-primary", Provider: "feishu", Domain: "feishu", Label: "Team updates", Enabled: true, Status: "connected",
			Credential:      config.BotConnectionCredential{AppID: "app-id", AppSecretEnv: "PRIVATE_SECRET_ENV"},
			SessionMappings: []config.BotConnectionSessionMapping{{RemoteID: "oc-secret-primary", Scope: "global"}},
		},
		{
			ID: "lark-disabled", Provider: "feishu", Domain: "lark", Label: "Lark archive", Enabled: false, Status: "connected",
			SessionMappings: []config.BotConnectionSessionMapping{{RemoteID: "oc-secret-disabled", Scope: "project", WorkspaceRoot: actor.WorkspaceRoot}},
		},
		{
			ID: "feishu-other-workspace", Provider: "feishu", Domain: "feishu", Label: "Other project", Enabled: true, Status: "connected",
			SessionMappings: []config.BotConnectionSessionMapping{{RemoteID: "oc-secret-other", Scope: "project", WorkspaceRoot: `C:\work\other`}},
		},
	}
	snapshot := desktopAutomationDeliverySnapshot{
		Actor: actor, WorkspaceRoot: actor.WorkspaceRoot, Paths: config.NewProfilePathResolver(actor),
		Config:        &config.Config{Bot: config.BotConfig{Connections: connections}},
		feishuSecrets: map[string]string{"feishu-primary": "token-super-secret", "feishu-other-workspace": "other-secret"},
	}
	adapter := newDesktopAutomationDeliveryAdapterWithDependencies(
		func(_ context.Context, workspaceRoot, principal string) (desktopAutomationDeliverySnapshot, error) {
			if workspaceRoot != actor.WorkspaceRoot || principal != desktopAutomationPrincipal(actor) {
				t.Fatalf("catalog loader owner = %q / %q", workspaceRoot, principal)
			}
			return snapshot, nil
		},
		desktopAutomationDeliverySenders{}, time.Second,
	)
	views, err := desktopAutomationDeliveryCatalog(context.Background(), adapter, actor.WorkspaceRoot, desktopAutomationPrincipal(actor))
	if err != nil {
		t.Fatalf("desktopAutomationDeliveryCatalog() error = %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("catalog = %+v, want current-workspace mappings only", views)
	}
	hasAvailable, hasUnavailable := false, false
	for _, view := range views {
		if view.Available {
			hasAvailable = true
		} else if view.UnavailableReason != "" {
			hasUnavailable = true
		}
	}
	if !hasAvailable || !hasUnavailable {
		t.Fatalf("catalog availability states = %+v", views)
	}
	payload, err := json.Marshal(views)
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	text := string(payload)
	for _, secret := range []string{"oc-secret-primary", "oc-secret-disabled", "oc-secret-other", "PRIVATE_SECRET_ENV", "token-super-secret", "other-secret", "app-id", "feishu-primary"} {
		if strings.Contains(text, secret) {
			t.Fatalf("catalog leaks %q: %s", secret, text)
		}
	}
}

func assertDesktopDeliveryFailure(t *testing.T, err error, want automation.DeliveryFailureClass) *DesktopAutomationDeliveryError {
	t.Helper()
	if err == nil {
		t.Fatalf("Deliver() error = nil, want %s", want)
	}
	var failure *DesktopAutomationDeliveryError
	if !errors.As(err, &failure) {
		t.Fatalf("error type = %T, want *DesktopAutomationDeliveryError: %v", err, err)
	}
	if failure.Class != want {
		t.Fatalf("failure class = %q, want %q", failure.Class, want)
	}
	return failure
}

func desktopAutomationDeliveryRequest(actor userctx.ActorContext, targetType, ref string) automation.DeliveryRequest {
	actor = actor.Normalize()
	return automation.DeliveryRequest{
		AttemptID: "attempt-1", TaskID: "task-1", RunID: "run-1",
		WorkspaceRoot: actor.WorkspaceRoot, Principal: desktopAutomationPrincipal(actor),
		Target: automation.DeliveryTarget{Type: targetType, Ref: ref}, Body: "hello",
	}
}
