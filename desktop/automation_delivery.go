package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kwu/internal/automation"
	"kwu/internal/bot"
	"kwu/internal/bot/feishu"
	"kwu/internal/bot/wecom"
	"kwu/internal/bot/weixin"
	"kwu/internal/config"
	"kwu/internal/userctx"
)

const (
	desktopAutomationDeliveryTypeFeishu      = "feishu"
	desktopAutomationDeliveryTypeLark        = "lark"
	desktopAutomationDeliveryTypeWeixinILink = "weixin_ilink"
	desktopAutomationDeliveryTypeWeCom       = "wecom"
	desktopAutomationDeliveryRefPrefix       = "bot_target_v1_"
	desktopAutomationDeliveryTimeout         = 15 * time.Second
)

// DesktopAutomationDeliveryError deliberately carries only a stable class,
// code, and safe message. Provider response bodies and credential details are
// never wrapped because Engine persists DeliveryAdapter error strings.
type DesktopAutomationDeliveryError struct {
	Class   automation.DeliveryFailureClass
	Code    string
	Message string
}

func (e *DesktopAutomationDeliveryError) Error() string {
	if e == nil {
		return "automation delivery failed"
	}
	return e.Message
}

func (e *DesktopAutomationDeliveryError) DeliveryFailureMetadata() automation.DeliveryFailure {
	if e == nil {
		return automation.DeliveryFailure{
			Class: automation.DeliveryFailurePermanent, Code: "delivery_error_missing", Message: "automation delivery cannot be completed",
		}
	}
	return automation.DeliveryFailure{Class: e.Class, Code: e.Code, Message: e.Message}
}

type desktopAutomationDeliverySnapshot struct {
	Actor         userctx.ActorContext
	WorkspaceRoot string
	Paths         config.ProfilePathResolver
	Config        *config.Config
	feishuSecrets map[string]string
	wecomSecrets  map[string]string
}

type desktopAutomationDeliverySnapshotLoader func(context.Context, string, string) (desktopAutomationDeliverySnapshot, error)

type desktopAutomationDeliverySenders struct {
	feishu func(context.Context, config.FeishuBotConfig, string, string, string) (bot.SendResult, error)
	weixin func(context.Context, config.WeixinBotConfig, config.ProfilePathResolver, string, string) (bot.SendResult, error)
	wecom  func(context.Context, string, string) (bot.SendResult, error)
}

type desktopAutomationDeliveryAdapter struct {
	loadSnapshot desktopAutomationDeliverySnapshotLoader
	senders      desktopAutomationDeliverySenders
	timeout      time.Duration
}

// DesktopAutomationDeliveryTargetView is the only bot-routing projection that
// automation UI may consume. It intentionally excludes remote IDs, connection
// IDs, URLs, environment names, credentials, and provider response data.
type DesktopAutomationDeliveryTargetView struct {
	Type              string `json:"type"`
	Ref               string `json:"ref"`
	Label             string `json:"label"`
	Provider          string `json:"provider"`
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailableReason,omitempty"`
	Scope             string `json:"scope"`
}

// newDesktopAutomationDeliveryAdapter resolves bot configuration from the
// immutable run owner and workspace on every send. It intentionally does not
// consult the active tab: a scheduled run may belong to another profile or
// workspace. Re-resolving makes disable, disconnect, mapping removal, profile
// mismatch, and grant revocation fail closed.
func newDesktopAutomationDeliveryAdapter(_ *App) *desktopAutomationDeliveryAdapter {
	loader := func(ctx context.Context, workspaceRoot, principal string) (desktopAutomationDeliverySnapshot, error) {
		if err := ctx.Err(); err != nil {
			return desktopAutomationDeliverySnapshot{}, err
		}
		actor, err := desktopAutomationActorFromPrincipal(principal, workspaceRoot)
		if err != nil {
			return desktopAutomationDeliverySnapshot{}, err
		}
		paths := config.NewProfilePathResolver(actor)
		// Metadata loading avoids the legacy process-wide credential environment.
		// Bot secrets are resolved explicitly from this profile below.
		cfg, err := config.LoadForRootMetadataWithActor(strings.TrimSpace(workspaceRoot), actor)
		if err != nil {
			return desktopAutomationDeliverySnapshot{}, err
		}
		if err := ctx.Err(); err != nil {
			return desktopAutomationDeliverySnapshot{}, err
		}
		feishuSecrets := make(map[string]string)
		wecomSecrets := make(map[string]string)
		for _, conn := range cfg.Bot.Connections {
			connType, supported := desktopAutomationConnectionDeliveryType(conn)
			if !supported {
				continue
			}
			secretName := ""
			switch connType {
			case desktopAutomationDeliveryTypeFeishu, desktopAutomationDeliveryTypeLark:
				secretName = firstNonEmptyBot(strings.TrimSpace(conn.Credential.AppSecretEnv), cfg.Bot.Feishu.AppSecretEnv)
			case desktopAutomationDeliveryTypeWeCom:
				secretName = strings.TrimSpace(conn.Credential.TokenEnv)
			default:
				continue
			}
			resolution := config.ResolveCredentialForRootGlobalFirstWithPaths(workspaceRoot, paths, secretName)
			if resolution.Set && strings.TrimSpace(resolution.Value) != "" {
				if connType == desktopAutomationDeliveryTypeWeCom {
					wecomSecrets[strings.TrimSpace(conn.ID)] = resolution.Value
				} else {
					feishuSecrets[strings.TrimSpace(conn.ID)] = resolution.Value
				}
			}
		}
		return desktopAutomationDeliverySnapshot{
			Actor: actor, WorkspaceRoot: workspaceRoot, Paths: paths, Config: cfg,
			feishuSecrets: feishuSecrets, wecomSecrets: wecomSecrets,
		}, nil
	}
	return newDesktopAutomationDeliveryAdapterWithDependencies(loader, desktopAutomationDeliverySenders{
		feishu: func(ctx context.Context, cfg config.FeishuBotConfig, remoteID, body, secret string) (bot.SendResult, error) {
			return feishu.SendTextWithSecret(ctx, cfg, remoteID, body, secret)
		},
		weixin: func(ctx context.Context, cfg config.WeixinBotConfig, paths config.ProfilePathResolver, remoteID, body string) (bot.SendResult, error) {
			return weixin.SendTextWithPaths(ctx, cfg, remoteID, body, paths)
		},
		wecom: func(ctx context.Context, key, body string) (bot.SendResult, error) {
			return wecom.SendText(ctx, key, body)
		},
	}, desktopAutomationDeliveryTimeout)
}

func newDesktopAutomationDeliveryAdapterWithDependencies(
	loader desktopAutomationDeliverySnapshotLoader,
	senders desktopAutomationDeliverySenders,
	timeout time.Duration,
) *desktopAutomationDeliveryAdapter {
	if timeout <= 0 {
		timeout = desktopAutomationDeliveryTimeout
	}
	return &desktopAutomationDeliveryAdapter{loadSnapshot: loader, senders: senders, timeout: timeout}
}

func desktopAutomationDeliveryCatalog(
	ctx context.Context,
	adapter *desktopAutomationDeliveryAdapter,
	workspaceRoot, principal string,
) ([]DesktopAutomationDeliveryTargetView, error) {
	if adapter == nil || adapter.loadSnapshot == nil {
		return nil, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTemporary, "resolver_unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, adapter.timeout)
	defer cancel()
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if _, err := desktopAutomationActorFromPrincipal(principal, workspaceRoot); err != nil {
		return nil, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "owner_invalid")
	}
	snapshot, err := adapter.loadSnapshot(ctx, workspaceRoot, principal)
	if err != nil {
		return nil, classifyDesktopAutomationDeliveryError(err, "config_load_failed")
	}
	if snapshot.Config == nil || desktopAutomationPrincipal(snapshot.Actor) != principal || snapshot.Paths.ProfileID() != snapshot.Actor.Normalize().ProfileID || normalizeDesktopAutomationWorkspace(snapshot.WorkspaceRoot) != normalizeDesktopAutomationWorkspace(workspaceRoot) {
		return nil, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "owner_mismatch")
	}

	views := make([]DesktopAutomationDeliveryTargetView, 0)
	seen := make(map[string]struct{})
	for _, conn := range snapshot.Config.Bot.Connections {
		targetType, supported := desktopAutomationConnectionDeliveryType(conn)
		if !supported {
			continue
		}
		available, reason := desktopAutomationConnectionAvailability(snapshot, conn, targetType)
		label := strings.TrimSpace(conn.Label)
		if label == "" {
			label = desktopAutomationDeliveryDefaultLabel(targetType)
		}
		provider := "weixin"
		if targetType == desktopAutomationDeliveryTypeFeishu || targetType == desktopAutomationDeliveryTypeLark {
			provider = "feishu"
		} else if targetType == desktopAutomationDeliveryTypeWeCom {
			provider = "wecom"
		}
		for _, mapping := range desktopAutomationDeliveryMappings(conn) {
			if strings.TrimSpace(mapping.RemoteID) == "" || !desktopAutomationMappingAllowed(mapping, conn, workspaceRoot) {
				continue
			}
			ref := desktopAutomationDeliveryTargetRef(snapshot.Actor, workspaceRoot, conn, mapping)
			key := targetType + "\x00" + ref
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			scope := "global"
			if strings.EqualFold(strings.TrimSpace(mapping.Scope), "project") || strings.TrimSpace(firstNonEmptyBot(mapping.WorkspaceRoot, conn.WorkspaceRoot)) != "" {
				scope = "project"
			}
			views = append(views, DesktopAutomationDeliveryTargetView{
				Type: targetType, Ref: ref, Label: label, Provider: provider,
				Available: available, UnavailableReason: reason, Scope: scope,
			})
		}
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Label != views[j].Label {
			return views[i].Label < views[j].Label
		}
		if views[i].Type != views[j].Type {
			return views[i].Type < views[j].Type
		}
		return views[i].Ref < views[j].Ref
	})
	if views == nil {
		return []DesktopAutomationDeliveryTargetView{}, nil
	}
	return views, nil
}

func desktopAutomationDeliveryCatalogForApp(
	ctx context.Context,
	app *App,
	adapter *desktopAutomationDeliveryAdapter,
) ([]DesktopAutomationDeliveryTargetView, error) {
	principal := automationActorPrincipal(app)
	globalRoot := globalTabWorkspaceRoot()
	globalTargets, err := desktopAutomationDeliveryCatalog(ctx, adapter, globalRoot, principal)
	if err != nil {
		return nil, err
	}
	views := make([]DesktopAutomationDeliveryTargetView, 0, len(globalTargets))
	for _, target := range globalTargets {
		if target.Scope == "global" {
			views = append(views, target)
		}
	}

	activeRoot := strings.TrimSpace(app.activeWorkspaceRoot())
	if activeRoot != "" && normalizeDesktopAutomationWorkspace(activeRoot) != normalizeDesktopAutomationWorkspace(globalRoot) {
		projectTargets, err := desktopAutomationDeliveryCatalog(ctx, adapter, activeRoot, principal)
		if err != nil {
			return nil, err
		}
		for _, target := range projectTargets {
			if target.Scope == "project" {
				views = append(views, target)
			}
		}
	}
	return views, nil
}

func (a *desktopAutomationDeliveryAdapter) Deliver(ctx context.Context, request automation.DeliveryRequest) (automation.DeliveryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	targetType, supported := normalizeDesktopAutomationDeliveryType(request.Target.Type)
	ref := strings.TrimSpace(request.Target.Ref)
	if strings.TrimSpace(request.AttemptID) == "" || strings.TrimSpace(request.TaskID) == "" || strings.TrimSpace(request.RunID) == "" {
		return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "invalid_request")
	}
	workspaceRoot := desktopAutomationDeliveryWorkspaceRoot(request.WorkspaceRoot)
	principal := strings.TrimSpace(request.Principal)
	if principal == "" || principal != request.Principal {
		return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "owner_missing")
	}
	requestActor, ownerErr := desktopAutomationActorFromPrincipal(principal, workspaceRoot)
	if ownerErr != nil {
		return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "owner_invalid")
	}
	if !supported {
		return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "provider_unsupported")
	}
	if !validDesktopAutomationDeliveryTargetRef(ref) {
		return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "untrusted_target_ref")
	}
	if a == nil || a.loadSnapshot == nil {
		return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTemporary, "resolver_unavailable")
	}

	snapshot, err := a.loadSnapshot(ctx, workspaceRoot, principal)
	if err != nil {
		return automation.DeliveryResult{}, classifyDesktopAutomationDeliveryError(err, "config_load_failed")
	}
	if !sameDesktopActorProfile(snapshot.Actor, requestActor) || desktopAutomationPrincipal(snapshot.Actor) != principal || normalizeDesktopAutomationWorkspace(snapshot.WorkspaceRoot) != normalizeDesktopAutomationWorkspace(workspaceRoot) {
		return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "owner_mismatch")
	}
	resolved, err := resolveDesktopAutomationDeliveryTarget(snapshot, targetType, ref)
	if err != nil {
		return automation.DeliveryResult{}, err
	}

	var result bot.SendResult
	switch resolved.targetType {
	case desktopAutomationDeliveryTypeFeishu, desktopAutomationDeliveryTypeLark:
		if a.senders.feishu == nil {
			return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "sender_unavailable")
		}
		if strings.TrimSpace(resolved.feishuSecret) == "" {
			return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "provider_credential_missing")
		}
		result, err = a.senders.feishu(ctx, resolved.feishuConfig, resolved.remoteID, request.Body, resolved.feishuSecret)
	case desktopAutomationDeliveryTypeWeixinILink:
		if a.senders.weixin == nil {
			return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "sender_unavailable")
		}
		result, err = a.senders.weixin(ctx, resolved.weixinConfig, snapshot.Paths, resolved.remoteID, request.Body)
	case desktopAutomationDeliveryTypeWeCom:
		if a.senders.wecom == nil {
			return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "sender_unavailable")
		}
		if strings.TrimSpace(resolved.wecomKey) == "" {
			return automation.DeliveryResult{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "provider_credential_missing")
		}
		result, err = a.senders.wecom(ctx, resolved.wecomKey, request.Body)
	}
	if err == nil && result.Err != nil {
		err = result.Err
	}
	if err != nil {
		return automation.DeliveryResult{}, classifyDesktopAutomationDeliveryError(err, "provider_send_failed")
	}
	return automation.DeliveryResult{ProviderMessageID: strings.TrimSpace(result.MessageID)}, nil
}

func desktopAutomationDeliveryWorkspaceRoot(workspaceRoot string) string {
	if workspaceRoot = strings.TrimSpace(workspaceRoot); workspaceRoot != "" {
		return workspaceRoot
	}
	return globalTabWorkspaceRoot()
}

type resolvedDesktopAutomationDeliveryTarget struct {
	targetType   string
	remoteID     string
	feishuConfig config.FeishuBotConfig
	feishuSecret string
	weixinConfig config.WeixinBotConfig
	wecomKey     string
}

func resolveDesktopAutomationDeliveryTarget(snapshot desktopAutomationDeliverySnapshot, targetType, ref string) (resolvedDesktopAutomationDeliveryTarget, error) {
	actor := snapshot.Actor.Normalize()
	workspaceRoot := strings.TrimSpace(snapshot.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = strings.TrimSpace(actor.WorkspaceRoot)
	}
	if snapshot.Config == nil || snapshot.Paths.ProfileID() != actor.ProfileID {
		return resolvedDesktopAutomationDeliveryTarget{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "owner_mismatch")
	}

	var match *resolvedDesktopAutomationDeliveryTarget
	for _, conn := range snapshot.Config.Bot.Connections {
		connType, supported := desktopAutomationConnectionDeliveryType(conn)
		if !supported || connType != targetType || !conn.Enabled || !strings.EqualFold(strings.TrimSpace(conn.Status), "connected") {
			continue
		}
		for _, mapping := range desktopAutomationDeliveryMappings(conn) {
			remoteID := strings.TrimSpace(mapping.RemoteID)
			if remoteID == "" || !desktopAutomationMappingAllowed(mapping, conn, workspaceRoot) {
				continue
			}
			if desktopAutomationDeliveryTargetRef(actor, workspaceRoot, conn, mapping) != ref {
				continue
			}
			candidate := resolvedDesktopAutomationDeliveryTarget{targetType: connType, remoteID: remoteID}
			switch connType {
			case desktopAutomationDeliveryTypeFeishu, desktopAutomationDeliveryTypeLark:
				candidate.feishuConfig = snapshot.Config.Bot.Feishu
				candidate.feishuConfig.Enabled = true
				candidate.feishuConfig.Domain = firstNonEmptyBot(strings.TrimSpace(conn.Domain), candidate.feishuConfig.Domain)
				candidate.feishuConfig.AppID = firstNonEmptyBot(strings.TrimSpace(conn.Credential.AppID), candidate.feishuConfig.AppID)
				candidate.feishuConfig.AppSecretEnv = firstNonEmptyBot(strings.TrimSpace(conn.Credential.AppSecretEnv), candidate.feishuConfig.AppSecretEnv)
				candidate.feishuSecret = snapshot.feishuSecrets[strings.TrimSpace(conn.ID)]
			case desktopAutomationDeliveryTypeWeixinILink:
				candidate.weixinConfig = snapshot.Config.Bot.Weixin
				candidate.weixinConfig.Enabled = true
				candidate.weixinConfig.AccountID = firstNonEmptyBot(strings.TrimSpace(conn.Credential.AccountID), candidate.weixinConfig.AccountID)
				// iLink saved accounts are profile-scoped. An empty TokenEnv prevents
				// the adapter from observing a process-global token from another profile.
				candidate.weixinConfig.TokenEnv = ""
			case desktopAutomationDeliveryTypeWeCom:
				candidate.wecomKey = snapshot.wecomSecrets[strings.TrimSpace(conn.ID)]
			}
			if match != nil && match.remoteID != candidate.remoteID {
				return resolvedDesktopAutomationDeliveryTarget{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "target_ambiguous")
			}
			match = &candidate
		}
	}
	if match == nil {
		return resolvedDesktopAutomationDeliveryTarget{}, newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "target_revoked")
	}
	return *match, nil
}

// desktopAutomationDeliveryTargetRef returns a stable opaque reference. The
// raw connection ID and remote routing IDs are inputs to the digest only and
// are never stored in an automation Task.
func desktopAutomationDeliveryTargetRef(
	actor userctx.ActorContext,
	workspaceRoot string,
	conn config.BotConnectionConfig,
	mapping config.BotConnectionSessionMapping,
) string {
	actor = actor.Normalize()
	targetType, _ := desktopAutomationConnectionDeliveryType(conn)
	parts := []string{
		"reasonix-desktop-automation-delivery-v1",
		actor.ActorID,
		actor.ProfileID,
		normalizeDesktopAutomationWorkspace(workspaceRoot),
		targetType,
		strings.TrimSpace(conn.ID),
		strings.TrimSpace(conn.Provider),
		strings.TrimSpace(conn.Domain),
		strings.TrimSpace(mapping.RemoteID),
		strings.TrimSpace(mapping.SessionID),
		strings.TrimSpace(mapping.ChatType),
		strings.TrimSpace(mapping.UserID),
		strings.TrimSpace(mapping.ThreadID),
		strings.TrimSpace(mapping.Scope),
		normalizeDesktopAutomationWorkspace(firstNonEmptyBot(mapping.WorkspaceRoot, conn.WorkspaceRoot)),
	}
	var material strings.Builder
	for _, part := range parts {
		fmt.Fprintf(&material, "%d:%s|", len(part), part)
	}
	digest := sha256.Sum256([]byte(material.String()))
	return desktopAutomationDeliveryRefPrefix + base64.RawURLEncoding.EncodeToString(digest[:])
}

func validDesktopAutomationDeliveryTargetRef(ref string) bool {
	if !strings.HasPrefix(ref, desktopAutomationDeliveryRefPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(ref, desktopAutomationDeliveryRefPrefix)
	digest, err := base64.RawURLEncoding.DecodeString(encoded)
	return err == nil && len(digest) == sha256.Size
}

func desktopAutomationConnectionDeliveryType(conn config.BotConnectionConfig) (string, bool) {
	provider := strings.ToLower(strings.TrimSpace(conn.Provider))
	domain := strings.ToLower(strings.TrimSpace(conn.Domain))
	if provider == "lark" {
		provider, domain = "feishu", "lark"
	}
	switch provider {
	case "feishu":
		if domain == "lark" {
			return desktopAutomationDeliveryTypeLark, true
		}
		if domain == "" || domain == "feishu" {
			return desktopAutomationDeliveryTypeFeishu, true
		}
	case "weixin":
		if domain == "" || domain == "weixin" || domain == "ilink" {
			return desktopAutomationDeliveryTypeWeixinILink, true
		}
	case "wecom":
		if domain == "" || domain == "wecom" {
			return desktopAutomationDeliveryTypeWeCom, true
		}
	}
	return "", false
}

func desktopAutomationConnectionAvailability(snapshot desktopAutomationDeliverySnapshot, conn config.BotConnectionConfig, targetType string) (bool, string) {
	if !conn.Enabled {
		return false, "connection_disabled"
	}
	if !strings.EqualFold(strings.TrimSpace(conn.Status), "connected") {
		return false, "connection_not_connected"
	}
	switch targetType {
	case desktopAutomationDeliveryTypeFeishu, desktopAutomationDeliveryTypeLark:
		if strings.TrimSpace(snapshot.feishuSecrets[strings.TrimSpace(conn.ID)]) == "" {
			return false, "credential_missing"
		}
	case desktopAutomationDeliveryTypeWeixinILink:
		weixinConfig := snapshot.Config.Bot.Weixin
		weixinConfig.AccountID = firstNonEmptyBot(strings.TrimSpace(conn.Credential.AccountID), weixinConfig.AccountID)
		if !weixin.HasSavedAccountWithPaths(snapshot.Paths, weixinConfig.AccountID) {
			return false, "credential_missing"
		}
	case desktopAutomationDeliveryTypeWeCom:
		if strings.TrimSpace(snapshot.wecomSecrets[strings.TrimSpace(conn.ID)]) == "" {
			return false, "credential_missing"
		}
	}
	return true, ""
}

func desktopAutomationDeliveryDefaultLabel(targetType string) string {
	switch targetType {
	case desktopAutomationDeliveryTypeFeishu:
		return "Feishu"
	case desktopAutomationDeliveryTypeLark:
		return "Lark"
	case desktopAutomationDeliveryTypeWeixinILink:
		return "Weixin iLink"
	case desktopAutomationDeliveryTypeWeCom:
		return "WeCom"
	default:
		return "Bot"
	}
}

func normalizeDesktopAutomationDeliveryType(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case desktopAutomationDeliveryTypeFeishu:
		return desktopAutomationDeliveryTypeFeishu, true
	case desktopAutomationDeliveryTypeLark:
		return desktopAutomationDeliveryTypeLark, true
	case desktopAutomationDeliveryTypeWeixinILink, "weixin", "ilink":
		return desktopAutomationDeliveryTypeWeixinILink, true
	case desktopAutomationDeliveryTypeWeCom:
		return desktopAutomationDeliveryTypeWeCom, true
	default:
		return "", false
	}
}

func desktopAutomationDeliveryMappings(conn config.BotConnectionConfig) []config.BotConnectionSessionMapping {
	targetType, supported := desktopAutomationConnectionDeliveryType(conn)
	if !supported || targetType != desktopAutomationDeliveryTypeWeCom {
		return conn.SessionMappings
	}
	scope := "global"
	workspaceRoot := strings.TrimSpace(conn.WorkspaceRoot)
	if workspaceRoot != "" {
		scope = "project"
	}
	return []config.BotConnectionSessionMapping{{RemoteID: "wecom-webhook", Scope: scope, WorkspaceRoot: workspaceRoot}}
}

func desktopAutomationActorFromPrincipal(principal, workspaceRoot string) (userctx.ActorContext, error) {
	if principal == "" || strings.TrimSpace(principal) != principal || strings.TrimSpace(workspaceRoot) == "" {
		return userctx.ActorContext{}, errors.New("automation delivery owner is missing")
	}
	separator := strings.LastIndex(principal, "/")
	if separator <= 0 || separator == len(principal)-1 {
		return userctx.ActorContext{}, errors.New("automation delivery owner is invalid")
	}
	actorID, profileID := principal[:separator], principal[separator+1:]
	if strings.TrimSpace(actorID) != actorID || !userctx.ValidateProfileID(profileID) {
		return userctx.ActorContext{}, errors.New("automation delivery owner is invalid")
	}
	actor := userctx.ActorContext{
		ActorID: actorID, ProfileID: profileID, AuthMethod: userctx.DefaultAuthMethod, WorkspaceRoot: strings.TrimSpace(workspaceRoot),
	}.Normalize()
	if desktopAutomationPrincipal(actor) != principal {
		return userctx.ActorContext{}, errors.New("automation delivery owner is invalid")
	}
	return actor, nil
}

func desktopAutomationPrincipal(actor userctx.ActorContext) string {
	actor = actor.Normalize()
	return actor.ActorID + "/" + actor.ProfileID
}

func desktopAutomationMappingAllowed(mapping config.BotConnectionSessionMapping, conn config.BotConnectionConfig, ownerWorkspace string) bool {
	mappingWorkspace := strings.TrimSpace(firstNonEmptyBot(mapping.WorkspaceRoot, conn.WorkspaceRoot))
	scope := strings.ToLower(strings.TrimSpace(mapping.Scope))
	if scope == "project" || mappingWorkspace != "" {
		return normalizeDesktopAutomationWorkspace(mappingWorkspace) == normalizeDesktopAutomationWorkspace(ownerWorkspace)
	}
	return scope == "" || scope == "global"
}

func normalizeDesktopAutomationWorkspace(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	cleaned := filepath.Clean(value)
	if absolute, err := filepath.Abs(cleaned); err == nil {
		cleaned = absolute
	}
	return strings.ToLower(cleaned)
}

func classifyDesktopAutomationDeliveryError(err error, fallbackCode string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTemporary, "delivery_timeout")
	}
	if errors.Is(err, context.Canceled) {
		return newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "delivery_canceled")
	}
	if kind, ok := wecom.FailureKindOf(err); ok {
		switch kind {
		case wecom.FailureInvalidRequest:
			return newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "wecom_request_invalid")
		case wecom.FailureRateLimited:
			return newDesktopAutomationDeliveryFailure(automation.DeliveryFailureRateLimited, "wecom_rate_limited")
		case wecom.FailureTargetRevoked:
			return newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "wecom_target_revoked")
		default:
			return newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTemporary, "wecom_temporarily_unavailable")
		}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "429"), strings.Contains(message, "rate limit"), strings.Contains(message, "too many requests"):
		return newDesktopAutomationDeliveryFailure(automation.DeliveryFailureRateLimited, "provider_rate_limited")
	case strings.Contains(message, "404"), strings.Contains(message, "not found"), strings.Contains(message, "chat_id is empty"), strings.Contains(message, "target revoked"):
		return newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTargetRevoked, "provider_target_revoked")
	case strings.Contains(message, "401"), strings.Contains(message, "403"), strings.Contains(message, "unauthorized"), strings.Contains(message, "forbidden"), strings.Contains(message, "credential"), strings.Contains(message, "token is not configured"), strings.Contains(message, "not set"):
		return newDesktopAutomationDeliveryFailure(automation.DeliveryFailurePermanent, "provider_authorization_failed")
	default:
		return newDesktopAutomationDeliveryFailure(automation.DeliveryFailureTemporary, fallbackCode)
	}
}

func newDesktopAutomationDeliveryFailure(class automation.DeliveryFailureClass, code string) *DesktopAutomationDeliveryError {
	message := "automation delivery failed"
	switch class {
	case automation.DeliveryFailureTemporary:
		message = "automation delivery is temporarily unavailable"
	case automation.DeliveryFailureRateLimited:
		message = "automation delivery was rate limited"
	case automation.DeliveryFailurePermanent:
		message = "automation delivery cannot be completed"
	case automation.DeliveryFailureTargetRevoked:
		message = "automation delivery target is unavailable or no longer authorized"
	}
	return &DesktopAutomationDeliveryError{Class: class, Code: strings.TrimSpace(code), Message: message}
}
