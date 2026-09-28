package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/i18n"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

func TestTurnDoneOnceSinkForwardsOneFriendlyTerminalEvent(t *testing.T) {
	raw := &provider.APIError{
		Provider: "airclaude_subscription",
		Status:   403,
		Code:     "pre_consume_token_quota_failed",
		Message:  "token quota is not enough",
	}
	var events []event.Event
	sink := &turnDoneOnceSink{target: event.FuncSink(func(e event.Event) {
		events = append(events, e)
	})}

	sink.Emit(event.Event{Kind: event.Text, Text: "partial"})
	sink.Emit(event.Event{Kind: event.TurnDone, Err: raw})
	sink.Emit(event.Event{Kind: event.TurnDone, Err: errors.New("duplicate")})

	if len(events) != 2 || events[0].Kind != event.Text || events[1].Kind != event.TurnDone {
		t.Fatalf("events = %#v, want text plus one terminal event", events)
	}
	if events[1].Err == nil || events[1].Err.Error() != i18n.M.ProviderErrInsufficientCredits {
		t.Fatalf("terminal error = %v, want friendly quota message", events[1].Err)
	}
}

// fakeAdapter 是一个内存中的假适配器，用于测试 BotGateway。
type fakeAdapter struct {
	mu       sync.Mutex
	platform Platform
	name     string
	msgCh    chan InboundMessage
	sent     []OutboundMessage
	started  bool
	startErr error
}

func newFakeAdapter(platform Platform, name string) *fakeAdapter {
	return &fakeAdapter{
		platform: platform,
		name:     name,
		msgCh:    make(chan InboundMessage, 16),
	}
}

func (f *fakeAdapter) Platform() Platform              { return f.platform }
func (f *fakeAdapter) Name() string                    { return f.name }
func (f *fakeAdapter) Messages() <-chan InboundMessage { return f.msgCh }

func (f *fakeAdapter) Start(ctx context.Context) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.mu.Lock()
	f.started = true
	f.mu.Unlock()
	return nil
}

func (f *fakeAdapter) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.msgCh != nil {
		close(f.msgCh)
		f.msgCh = nil
	}
	return nil
}

func (f *fakeAdapter) Send(ctx context.Context, msg OutboundMessage) (SendResult, error) {
	f.mu.Lock()
	f.sent = append(f.sent, msg)
	f.mu.Unlock()
	return SendResult{MessageID: "fake_msg_1"}, nil
}

func (f *fakeAdapter) SendTyping(ctx context.Context, chatID string) error { return nil }

func (f *fakeAdapter) sentMessages() []OutboundMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]OutboundMessage, len(f.sent))
	copy(out, f.sent)
	return out
}

type fakeReactionAdapter struct {
	*fakeAdapter
	reactions []string
}

type gatewayFakeProvider struct{}

func (gatewayFakeProvider) Name() string { return "fake" }

func (gatewayFakeProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk)
	close(ch)
	return ch, nil
}

const botActorTestProviderKind = "bot-actor-test"

var botActorTestProviderOnce sync.Once

func registerBotActorTestProvider() {
	botActorTestProviderOnce.Do(func() {
		provider.Register(botActorTestProviderKind, func(provider.Config) (provider.Provider, error) {
			return gatewayFakeProvider{}, nil
		})
	})
}

func isolateBotActorConfigHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KWU_HOME", filepath.Join(dir, "home"))
	t.Setenv("KWU_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("KWU_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("KWU_CREDENTIALS_STORE", "file")
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
}

func writeBotActorTestConfig(t *testing.T, root string) {
	t.Helper()
	body := `
default_model = "bot-actor-test"

[agent]
system_prompt = "BASE"

[[providers]]
name = "bot-actor-test"
kind = "bot-actor-test"
base_url = "http://127.0.0.1:1/v1"
model = "model"
`
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBotActorTestConfigWithDesktopYolo(t *testing.T, root string) {
	t.Helper()
	body := `
default_model = "bot-actor-test"

[agent]
system_prompt = "BASE"

[desktop]
default_tool_approval_mode = "yolo"

[[providers]]
name = "bot-actor-test"
kind = "bot-actor-test"
base_url = "http://127.0.0.1:1/v1"
model = "model"
`
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeReactionAdapter) AddPendingReaction(ctx context.Context, messageID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reactions = append(f.reactions, messageID)
	return nil
}

func TestFakeAdapterInterface(t *testing.T) {
	fa := newFakeAdapter(PlatformQQ, "fake-qq")

	if fa.Platform() != PlatformQQ {
		t.Error("wrong platform")
	}
	if fa.Name() != "fake-qq" {
		t.Error("wrong name")
	}

	ctx := context.Background()
	if err := fa.Start(ctx); err != nil {
		t.Fatal("start:", err)
	}
	if !fa.started {
		t.Error("should be started")
	}

	_, err := fa.Send(ctx, OutboundMessage{ChatID: "c1", Text: "hello"})
	if err != nil {
		t.Fatal("send:", err)
	}

	sent := fa.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if sent[0].Text != "hello" {
		t.Errorf("sent text = %q, want %q", sent[0].Text, "hello")
	}

	if err := fa.Stop(); err != nil {
		t.Fatal("stop:", err)
	}
}

func TestGatewayConstructAndStop(t *testing.T) {
	cfg := GatewayConfig{
		Model:         "test",
		MaxSteps:      10,
		WorkspaceRoot: ".",
		Enabled:       map[Platform]bool{PlatformQQ: true},
		Allowlist:     AllowlistConfig{Enabled: false},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, map[Platform]Adapter{
		PlatformQQ: newFakeAdapter(PlatformQQ, "fake-qq"),
	}, logger)

	// 网关不应该 panic
	if gw == nil {
		t.Fatal("gateway should not be nil")
	}
	gw.Stop()
}

func TestGatewayStartsHealthyAdaptersWhenOneFails(t *testing.T) {
	cfg := GatewayConfig{
		Enabled:   map[Platform]bool{PlatformFeishu: true, PlatformWeixin: true},
		Allowlist: AllowlistConfig{AllowAll: true},
	}
	good := newFakeAdapter(PlatformFeishu, "good-feishu")
	bad := newFakeAdapter(PlatformWeixin, "bad-weixin")
	bad.startErr = errors.New("missing token")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGatewayWithAdapterBindings(cfg, []AdapterBinding{
		{ID: "feishu-lark", Platform: PlatformFeishu, Adapter: good},
		{ID: "weixin-weixin", Platform: PlatformWeixin, Adapter: bad},
	}, logger)

	if err := gw.Start(context.Background()); err != nil {
		t.Fatalf("start should keep healthy adapters running: %v", err)
	}
	if got := gw.AdapterCount(); got != 1 {
		t.Fatalf("adapter count = %d, want 1", got)
	}
	if !good.started {
		t.Fatal("healthy adapter was not started")
	}
	if bad.started {
		t.Fatal("failing adapter should not be marked started")
	}
	startErr := gw.StartErrors()
	if len(startErr) != 1 || !strings.Contains(startErr[0].Error(), "weixin-weixin") {
		t.Fatalf("start errors = %#v, want wrapped connection error", startErr)
	}
}

func TestGatewayReturnsErrorWhenAllAdaptersFail(t *testing.T) {
	cfg := GatewayConfig{
		Enabled:   map[Platform]bool{PlatformWeixin: true},
		Allowlist: AllowlistConfig{AllowAll: true},
	}
	bad := newFakeAdapter(PlatformWeixin, "bad-weixin")
	bad.startErr = errors.New("missing token")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGatewayWithAdapterBindings(cfg, []AdapterBinding{
		{ID: "weixin-weixin", Platform: PlatformWeixin, Adapter: bad},
	}, logger)

	err := gw.Start(context.Background())
	if err == nil {
		t.Fatal("start should fail when every adapter fails")
	}
	if !strings.Contains(err.Error(), "weixin-weixin") {
		t.Fatalf("error = %v, want connection id", err)
	}
	if got := gw.AdapterCount(); got != 0 {
		t.Fatalf("adapter count = %d, want 0", got)
	}
	if len(gw.StartErrors()) != 1 {
		t.Fatalf("start errors = %#v, want one", gw.StartErrors())
	}
}

func TestGatewayAllowlistCheck(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				PlatformQQ: {"allowed_user_1"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(PlatformQQ, InboundMessage{Platform: PlatformQQ, ChatType: ChatDM, UserID: "allowed_user_1"}) {
		t.Error("allowed user should pass")
	}
	if gw.checkAllowlist(PlatformQQ, InboundMessage{Platform: PlatformQQ, ChatType: ChatDM, UserID: "unknown_user"}) {
		t.Error("unknown user should not pass")
	}
	// 不同平台
	if gw.checkAllowlist(PlatformFeishu, InboundMessage{Platform: PlatformFeishu, ChatType: ChatDM, UserID: "allowed_user_1"}) {
		t.Error("QQ allowlist should not apply to feishu")
	}
}

func TestGatewayAllowlistDoesNotApplyGroupsToDirectMessages(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				PlatformQQ: {"allowed_user"},
			},
			Groups: map[Platform][]string{
				PlatformQQ: {"allowed_group"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(PlatformQQ, InboundMessage{Platform: PlatformQQ, ChatType: ChatDirect, ChatID: "guild-dm", UserID: "allowed_user"}) {
		t.Error("direct message should not be rejected by group allowlist")
	}
	if gw.checkAllowlist(PlatformQQ, InboundMessage{Platform: PlatformQQ, ChatType: ChatGroup, ChatID: "unknown_group", UserID: "allowed_user"}) {
		t.Error("unknown group should still be rejected by group allowlist")
	}
}

func TestGatewayAllowlistGatesOnOperatorNotCardRequester(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				PlatformFeishu: {"requester"},
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	stranger := InboundMessage{Platform: PlatformFeishu, ChatType: ChatGroup, ChatID: "chat", UserID: "requester", OperatorID: "stranger"}
	if gw.checkAllowlist(PlatformFeishu, stranger) {
		t.Error("a non-allowlisted operator must be rejected even when the card carries an allowlisted requester id")
	}

	allowed := InboundMessage{Platform: PlatformFeishu, ChatType: ChatGroup, ChatID: "chat", UserID: "requester", OperatorID: "requester"}
	if !gw.checkAllowlist(PlatformFeishu, allowed) {
		t.Error("an allowlisted operator should pass")
	}
}

func TestGatewayAllowlistAcceptsAnyTrustedOperatorID(t *testing.T) {
	for _, allowedID := range []string{"open-clicker", "union-clicker", "user-clicker"} {
		t.Run(allowedID, func(t *testing.T) {
			cfg := GatewayConfig{
				Allowlist: AllowlistConfig{
					Enabled: true,
					Users: map[Platform][]string{
						PlatformFeishu: {allowedID},
					},
				},
			}
			gw := NewGateway(cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
			msg := InboundMessage{
				Platform:    PlatformFeishu,
				ChatType:    ChatDM,
				UserID:      "requester",
				OperatorID:  "union-clicker",
				OperatorIDs: []string{"union-clicker", "open-clicker", "user-clicker", "open-clicker", ""},
			}
			if !gw.checkAllowlist(PlatformFeishu, msg) {
				t.Fatalf("operator alias %q should pass", allowedID)
			}
		})
	}
}

func TestGatewayAllowlistOperatorAliasesNeverFallbackToRequester(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				PlatformFeishu: {"requester"},
			},
		},
	}
	gw := NewGateway(cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	msg := InboundMessage{
		Platform:    PlatformFeishu,
		ChatType:    ChatDM,
		UserID:      "requester",
		OperatorID:  "unknown-union",
		OperatorIDs: []string{"unknown-union", "unknown-open", "unknown-user"},
	}
	if gw.checkAllowlist(PlatformFeishu, msg) {
		t.Fatal("untrusted operator aliases must not fall back to the allowlisted card requester")
	}
}

func TestGatewayAllowlistPreservesGroupCollaboratorRulesForOperatorAliases(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{
			Enabled: true,
			Users: map[Platform][]string{
				PlatformFeishu: {"requester", "collaborator-open"},
			},
			Groups: map[Platform][]string{
				PlatformFeishu: {"allowed-chat"},
			},
		},
	}
	gw := NewGateway(cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	collaborator := InboundMessage{
		Platform:    PlatformFeishu,
		ChatType:    ChatGroup,
		ChatID:      "allowed-chat",
		UserID:      "requester",
		OperatorID:  "collaborator-union",
		OperatorIDs: []string{"collaborator-union", "collaborator-open"},
	}
	if !gw.checkAllowlist(PlatformFeishu, collaborator) {
		t.Fatal("an allowlisted group collaborator alias should pass for an allowed group")
	}
	if gw.checkAllowlist(PlatformFeishu, InboundMessage{
		Platform:    PlatformFeishu,
		ChatType:    ChatGroup,
		ChatID:      "blocked-chat",
		UserID:      "requester",
		OperatorID:  "collaborator-union",
		OperatorIDs: []string{"collaborator-union", "collaborator-open"},
	}) {
		t.Fatal("operator aliases must not bypass the group allowlist")
	}
	if gw.checkAllowlist(PlatformFeishu, InboundMessage{
		Platform:    PlatformFeishu,
		ChatType:    ChatGroup,
		ChatID:      "allowed-chat",
		UserID:      "requester",
		OperatorID:  "stranger-union",
		OperatorIDs: []string{"stranger-union", "stranger-open"},
	}) {
		t.Fatal("an untrusted group clicker must not inherit the requester's access")
	}
}

func TestGatewayAllowlistDisabledRejectsByDefault(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{Enabled: false},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if gw.checkAllowlist(PlatformQQ, InboundMessage{Platform: PlatformQQ, ChatType: ChatDM, UserID: "any_user"}) {
		t.Error("disabled allowlist should reject unless allow_all is explicit")
	}
}

func TestGatewayAllowAll(t *testing.T) {
	cfg := GatewayConfig{
		Allowlist: AllowlistConfig{AllowAll: true},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(cfg, nil, logger)

	if !gw.checkAllowlist(PlatformQQ, InboundMessage{Platform: PlatformQQ, ChatType: ChatDM, UserID: "any_user"}) {
		t.Error("allow_all should allow everyone")
	}
}

func TestGatewayNormalizesNumericApprovalShortcutsOnlyWhenPending(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	key := "session-key"

	if _, ok := gw.normalizeApprovalShortcut(key, "1"); ok {
		t.Fatal("numeric text without a pending approval should stay a normal message")
	}

	gw.controllers[key] = &sessionState{
		pendingApprovals: map[string]event.Approval{
			"42": {ID: "42", Tool: "explore"},
		},
		lastApprovalID: "42",
	}

	got, ok := gw.normalizeApprovalShortcut(key, "1")
	if !ok || got != "/approve 42" {
		t.Fatalf("normalize 1 = %q,%v; want /approve 42,true", got, ok)
	}
	got, ok = gw.normalizeApprovalShortcut(key, "2")
	if !ok || got != "/deny 42" {
		t.Fatalf("normalize 2 = %q,%v; want /deny 42,true", got, ok)
	}
	gw.forgetPendingApproval(key, "42")
	if _, ok := gw.normalizeApprovalShortcut(key, "1"); ok {
		t.Fatal("numeric text after approval is forgotten should stay a normal message")
	}
}

func TestGatewayNormalizesNumericAskShortcutOnlyForSingleChoicePendingAsk(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	key := "session-key"

	if _, ok := gw.normalizeAskShortcut(key, "1"); ok {
		t.Fatal("numeric text without a pending ask should stay a normal message")
	}

	gw.controllers[key] = &sessionState{
		pendingAsks: map[string][]event.AskQuestion{
			"ask-1": {{
				ID:     "q1",
				Prompt: "Choose one",
				Options: []event.AskOption{
					{Label: "Allow once"},
					{Label: "Deny"},
				},
			}},
		},
		lastAskID: "ask-1",
	}

	got, ok := gw.normalizeAskShortcut(key, "2")
	if !ok || got != "/answer ask-1 2" {
		t.Fatalf("normalize 2 = %q,%v; want /answer ask-1 2,true", got, ok)
	}
	if _, ok := gw.normalizeAskShortcut(key, "1;2"); ok {
		t.Fatal("compound numeric text should stay a normal message")
	}

	gw.controllers[key].pendingAsks["ask-2"] = []event.AskQuestion{
		{ID: "q1", Prompt: "First", Options: []event.AskOption{{Label: "A"}}},
		{ID: "q2", Prompt: "Second", Options: []event.AskOption{{Label: "B"}}},
	}
	gw.controllers[key].lastAskID = "ask-2"
	if _, ok := gw.normalizeAskShortcut(key, "1"); ok {
		t.Fatal("numeric shortcut should not answer multi-question asks")
	}
}

func TestGatewaySessionOptionsUseConnectionToolApprovalOverride(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:            "default-model",
		ToolApprovalMode: "auto",
		Channels: map[Platform]ChannelConfig{
			PlatformFeishu: {Model: "platform-model", ToolApprovalMode: "ask"},
		},
		ConnectionChannels: map[string]ChannelConfig{
			"feishu-lark": {Model: "lark-model", ToolApprovalMode: "yolo"},
		},
	}, nil, logger)

	model, _, mode := gw.sessionOptionsForMessage(InboundMessage{
		Platform:     PlatformFeishu,
		ConnectionID: "feishu-lark",
	})
	if model != "lark-model" || mode != "yolo" {
		t.Fatalf("lark session options = model %q mode %q, want lark-model/yolo", model, mode)
	}

	model, _, mode = gw.sessionOptionsForMessage(InboundMessage{Platform: PlatformFeishu})
	if model != "platform-model" || mode != "ask" {
		t.Fatalf("platform session options = model %q mode %q, want platform-model/ask", model, mode)
	}
}

func TestGatewayCreatesControllerWithBotActorContext(t *testing.T) {
	isolateBotActorConfigHome(t)
	registerBotActorTestProvider()
	workspaceRoot := t.TempDir()
	writeBotActorTestConfig(t, workspaceRoot)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:         "bot-actor-test",
		WorkspaceRoot: workspaceRoot,
	}, nil, logger)
	msg := InboundMessage{
		Platform:   PlatformFeishu,
		Domain:     "lark",
		ChatType:   ChatThread,
		ChatID:     "chat-123",
		UserID:     "remote-user-456",
		OperatorID: "allowlisted-operator",
		ThreadID:   "thread-789",
	}
	key := BuildSessionKey(msg.Session())

	state := gw.getOrCreateSession(context.Background(), key, msg)
	if state == nil || state.ctrl == nil {
		t.Fatal("getOrCreateSession returned nil controller")
	}
	defer state.ctrl.Close()

	identity, ok := state.ctrl.(control.RuntimeIdentity)
	if !ok {
		t.Fatal("bot controller does not expose runtime identity")
	}
	actor := identity.ActorContext()
	if actor.ActorID != botServiceActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, botServiceActorID)
	}
	if actor.ActorID == msg.UserID || actor.ActorID == msg.OperatorID {
		t.Fatalf("ActorID = %q, want service actor rather than remote user/operator", actor.ActorID)
	}
	if actor.ProfileID != userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", actor.ProfileID, userctx.DefaultProfileID)
	}
	if actor.AuthMethod != botAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, botAuthMethod)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(workspaceRoot) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, workspaceRoot)
	}
	if actor.RemoteIdentity == nil {
		t.Fatal("RemoteIdentity is nil")
	}
	remote := *actor.RemoteIdentity
	if remote.Provider != "feishu" || remote.Domain != "lark" || remote.UserID != msg.UserID || remote.ChatID != msg.ChatID || remote.ThreadID != msg.ThreadID {
		t.Fatalf("RemoteIdentity = %#v, want inbound platform/domain/user/chat/thread", remote)
	}
}

func TestGatewayCreatesControllerWithConfiguredServiceActorProfile(t *testing.T) {
	isolateBotActorConfigHome(t)
	registerBotActorTestProvider()
	workspaceRoot := t.TempDir()
	writeBotActorTestConfig(t, workspaceRoot)

	owner := userctx.Default(workspaceRoot)
	owner.ActorID = "desktop-user"
	owner.ProfileID = "profile-bot"
	serviceActor := ServiceActorContext(owner, workspaceRoot)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:         "bot-actor-test",
		WorkspaceRoot: workspaceRoot,
		Actor:         serviceActor,
	}, nil, logger)
	msg := InboundMessage{
		Platform:   PlatformFeishu,
		Domain:     "lark",
		ChatType:   ChatThread,
		ChatID:     "chat-123",
		UserID:     "remote-user-456",
		OperatorID: "allowlisted-operator",
		ThreadID:   "thread-789",
	}
	key := BuildSessionKey(msg.Session())

	state := gw.getOrCreateSession(context.Background(), key, msg)
	if state == nil || state.ctrl == nil {
		t.Fatal("getOrCreateSession returned nil controller")
	}
	defer state.ctrl.Close()

	identity, ok := state.ctrl.(control.RuntimeIdentity)
	if !ok {
		t.Fatal("bot controller does not expose runtime identity")
	}
	actor := identity.ActorContext()
	if actor.ActorID != botServiceActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, botServiceActorID)
	}
	if actor.ActorID == msg.UserID || actor.ActorID == msg.OperatorID {
		t.Fatalf("ActorID = %q, want service actor rather than remote user/operator", actor.ActorID)
	}
	if actor.ProfileID != "profile-bot" {
		t.Fatalf("ProfileID = %q, want profile-bot", actor.ProfileID)
	}
	if actor.AuthMethod != botAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, botAuthMethod)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(workspaceRoot) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, workspaceRoot)
	}
	expectedSessionDir := config.NewProfilePathResolver(serviceActor).ProjectSessionDir(workspaceRoot)
	if got := state.ctrl.SessionDir(); filepath.Clean(got) != filepath.Clean(expectedSessionDir) {
		t.Fatalf("SessionDir = %q, want configured actor profile project dir %q", got, expectedSessionDir)
	}
	if actor.RemoteIdentity == nil {
		t.Fatal("RemoteIdentity is nil")
	}
	remote := *actor.RemoteIdentity
	if remote.Provider != "feishu" || remote.Domain != "lark" || remote.UserID != msg.UserID || remote.ChatID != msg.ChatID || remote.ThreadID != msg.ThreadID {
		t.Fatalf("RemoteIdentity = %#v, want inbound platform/domain/user/chat/thread", remote)
	}
}

func TestGatewayCreatesSessionThroughConfiguredControllerBuilder(t *testing.T) {
	isolateBotActorConfigHome(t)
	registerBotActorTestProvider()
	workspaceRoot := t.TempDir()
	writeBotActorTestConfig(t, workspaceRoot)

	var received boot.Options
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:         "bot-actor-test",
		WorkspaceRoot: workspaceRoot,
		ControllerBuilder: func(ctx context.Context, options boot.Options) (*control.Controller, error) {
			received = options
			return boot.Build(ctx, options)
		},
	}, nil, logger)
	msg := InboundMessage{
		Platform: PlatformFeishu,
		Domain:   "lark",
		ChatType: ChatDM,
		ChatID:   "chat-builder",
		UserID:   "remote-builder",
	}
	state := gw.getOrCreateSession(context.Background(), BuildSessionKey(msg.Session()), msg)
	if state == nil || state.ctrl == nil {
		t.Fatal("getOrCreateSession returned nil controller")
	}
	defer state.ctrl.Close()

	if received.Model != "bot-actor-test" || filepath.Clean(received.WorkspaceRoot) != filepath.Clean(workspaceRoot) {
		t.Fatalf("controller builder options = model %q workspace %q", received.Model, received.WorkspaceRoot)
	}
	if received.Actor.RemoteIdentity == nil || received.Actor.RemoteIdentity.UserID != msg.UserID {
		t.Fatalf("controller builder actor = %#v, want inbound remote identity", received.Actor)
	}
}

func TestGatewayActorContextForMessageKeepsProfileAndRemoteIdentitySeparate(t *testing.T) {
	owner := userctx.ActorContext{
		ActorID:    "desktop-user",
		ProfileID:  "profile-bot",
		AuthMethod: "local",
		RemoteIdentity: &userctx.RemoteIdentity{
			Provider: "stale",
			UserID:   "stale-user",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{Actor: owner}, nil, logger)
	msg := InboundMessage{
		Platform:   PlatformFeishu,
		Domain:     "lark",
		ChatType:   ChatThread,
		ChatID:     "chat-123",
		UserID:     "remote-user-456",
		OperatorID: "allowlisted-operator",
		ThreadID:   "thread-789",
	}

	actor := gw.actorContextForMessage(msg, "C:\\work\\project")

	if actor.ActorID != botServiceActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, botServiceActorID)
	}
	if actor.ActorID == msg.UserID || actor.ActorID == msg.OperatorID {
		t.Fatalf("ActorID = %q, want service actor rather than remote user/operator", actor.ActorID)
	}
	if actor.ProfileID != "profile-bot" {
		t.Fatalf("ProfileID = %q, want configured profile", actor.ProfileID)
	}
	if actor.ProfileID == userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want non-default profile preserved", actor.ProfileID)
	}
	if actor.AuthMethod != botAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, botAuthMethod)
	}
	if actor.WorkspaceRoot != "C:\\work\\project" {
		t.Fatalf("WorkspaceRoot = %q, want C:\\work\\project", actor.WorkspaceRoot)
	}
	if actor.RemoteIdentity == nil {
		t.Fatal("RemoteIdentity is nil")
	}
	remote := *actor.RemoteIdentity
	if remote.Provider != "feishu" || remote.Domain != "lark" || remote.UserID != msg.UserID || remote.ChatID != msg.ChatID || remote.ThreadID != msg.ThreadID {
		t.Fatalf("RemoteIdentity = %#v, want inbound platform/domain/user/chat/thread", remote)
	}
	if remote.UserID == "stale-user" {
		t.Fatalf("RemoteIdentity reused stale configured owner identity: %#v", remote)
	}
}

func TestGatewayBotSessionDefaultsToAskDespiteDesktopYolo(t *testing.T) {
	isolateBotActorConfigHome(t)
	registerBotActorTestProvider()
	workspaceRoot := t.TempDir()
	writeBotActorTestConfigWithDesktopYolo(t, workspaceRoot)

	desktopCtrl := control.New(control.Options{})
	desktopCtrl.SetToolApprovalMode(control.ToolApprovalYolo)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:         "bot-actor-test",
		WorkspaceRoot: workspaceRoot,
	}, nil, logger)
	msg := InboundMessage{
		Platform: PlatformFeishu,
		Domain:   "lark",
		ChatType: ChatDM,
		ChatID:   "chat-123",
		UserID:   "remote-user-456",
	}
	key := BuildSessionKey(msg.Session())

	state := gw.getOrCreateSession(context.Background(), key, msg)
	if state == nil || state.ctrl == nil {
		t.Fatal("getOrCreateSession returned nil controller")
	}
	defer state.ctrl.Close()

	if got := state.ctrl.ToolApprovalMode(); got != control.ToolApprovalAsk {
		t.Fatalf("bot session approval mode = %q, want ask despite desktop yolo", got)
	}
	if got := desktopCtrl.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("desktop controller mode = %q, want yolo setup to remain isolated", got)
	}
}

func TestGatewayNumericApprovalShortcutActiveWithoutPendingSendsGuidance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{Allowlist: AllowlistConfig{AllowAll: true}}, nil, logger)
	adapter := newFakeAdapter(PlatformWeixin, "fake-weixin")
	binding := AdapterBinding{ID: "weixin-weixin", Domain: "weixin", Platform: PlatformWeixin, Adapter: adapter}
	msg := InboundMessage{
		Platform:     PlatformWeixin,
		ConnectionID: "weixin-weixin",
		Domain:       "weixin",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "seed",
	}
	key := BuildSessionKey(msg.Session())
	if acquired, _ := gw.sessions.TryAcquire(key, msg); !acquired {
		t.Fatal("failed to mark session active")
	}

	msg.Text = "1"
	gw.handleMessage(context.Background(), binding, msg)

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if !strings.Contains(sent[0].Text, "没有找到可匹配的待处理操作") {
		t.Fatalf("sent text = %q, want pending operation guidance", sent[0].Text)
	}
}

func TestGatewayApproveWithoutSessionSendsGuidance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	adapter := newFakeAdapter(PlatformWeixin, "fake-weixin")
	msg := InboundMessage{ChatType: ChatDM, ChatID: "chat", UserID: "user", Text: "/approve 1"}

	gw.handleSlashCommand(context.Background(), adapter, "missing-session", msg)

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if !strings.Contains(sent[0].Text, "没有找到当前会话中的待审批操作") {
		t.Fatalf("sent text = %q, want missing approval guidance", sent[0].Text)
	}
}

func TestGatewayNewSessionRemembersRotatedSessionPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var remembered string
	gw := NewGateway(GatewayConfig{
		OnSessionReady: func(msg InboundMessage, sessionID string) error {
			remembered = sessionID
			return nil
		},
	}, nil, logger)
	adapter := newFakeAdapter(PlatformWeixin, "fake-weixin")
	msg := InboundMessage{
		Platform:     PlatformWeixin,
		ConnectionID: "weixin-weixin",
		Domain:       "weixin",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/new",
	}
	key := BuildSessionKey(msg.Session())
	sessionDir := t.TempDir()
	oldPath := agent.NewSessionPath(sessionDir, "old-model")
	exec := agent.New(gatewayFakeProvider{}, tool.NewRegistry(), agent.NewSession("system"), agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: exec, SessionDir: sessionDir, SessionPath: oldPath, Label: "fake-model"})
	gw.controllers[key] = &sessionState{ctrl: ctrl}

	gw.handleSlashCommand(context.Background(), adapter, key, msg)

	if remembered == "" || !strings.HasPrefix(remembered, "path:") {
		t.Fatalf("remembered session = %q, want path target", remembered)
	}
	if remembered == "path:"+oldPath {
		t.Fatalf("remembered session = %q, want rotated path", remembered)
	}
	if ctrl.SessionPath() == oldPath {
		t.Fatalf("controller session path was not rotated")
	}
}

type sessionUpdatedController struct {
	botController
	sessionPath string
	onRun       func(string) error
}

func (c *sessionUpdatedController) RunTurn(ctx context.Context, input string) error {
	if c.onRun != nil {
		return c.onRun(input)
	}
	return nil
}

func (c *sessionUpdatedController) SessionPath() string {
	return c.sessionPath
}

func TestGatewayNotifiesSessionUpdatedAfterTurn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionPath := filepath.Join(t.TempDir(), "im.jsonl")
	events := []string{}
	ctrl := &sessionUpdatedController{
		sessionPath: sessionPath,
		onRun: func(input string) error {
			events = append(events, "run:"+input)
			return nil
		},
	}
	gw := NewGateway(GatewayConfig{
		OnSessionReady: func(msg InboundMessage, sessionID string) error {
			events = append(events, "ready:"+sessionID)
			return nil
		},
		OnSessionUpdated: func(msg InboundMessage, sessionID string) error {
			events = append(events, "updated:"+sessionID)
			return nil
		},
	}, nil, logger)
	adapter := newFakeAdapter(PlatformWeixin, "fake-weixin")
	msg := InboundMessage{
		Platform:     PlatformWeixin,
		ConnectionID: "weixin-weixin",
		Domain:       "weixin",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "hello",
	}
	key := BuildSessionKey(msg.Session())
	gw.controllers[key] = &sessionState{ctrl: ctrl, sink: &sessionEventSink{}}

	gw.runTurn(context.Background(), adapter, key, msg)

	want := strings.Join([]string{"ready:path:" + sessionPath, "run:hello", "updated:path:" + sessionPath}, "|")
	if got := strings.Join(events, "|"); got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

func TestGatewayNotifiesSessionUpdatedAfterTurnError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionPath := filepath.Join(t.TempDir(), "im-error.jsonl")
	turnErr := &provider.APIError{
		Provider: "airclaude_subscription",
		Status:   403,
		Code:     "pre_consume_token_quota_failed",
		Message:  "token quota is not enough",
	}
	var updated string
	ctrl := &sessionUpdatedController{
		sessionPath: sessionPath,
		onRun: func(input string) error {
			return turnErr
		},
	}
	gw := NewGateway(GatewayConfig{
		OnSessionUpdated: func(msg InboundMessage, sessionID string) error {
			updated = sessionID
			return nil
		},
	}, nil, logger)
	adapter := newFakeAdapter(PlatformWeixin, "fake-weixin")
	msg := InboundMessage{
		Platform:     PlatformWeixin,
		ConnectionID: "weixin-weixin",
		Domain:       "weixin",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "hello",
	}
	key := BuildSessionKey(msg.Session())
	gw.controllers[key] = &sessionState{ctrl: ctrl, sink: &sessionEventSink{}}

	gw.runTurn(context.Background(), adapter, key, msg)

	if want := "path:" + sessionPath; updated != want {
		t.Fatalf("updated session = %q, want %q", updated, want)
	}
	sent := adapter.sentMessages()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, i18n.M.ProviderErrInsufficientCredits) {
		t.Fatalf("messages = %#v, want one friendly fallback error", sent)
	}
}

func TestGatewayYoloCommandUpdatesCurrentSessionAndConnectionDefault(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var persistedMode string
	var persistedConnection string
	gw := NewGateway(GatewayConfig{
		ToolApprovalMode: "ask",
		ConnectionChannels: map[string]ChannelConfig{
			"feishu-lark": {ToolApprovalMode: "ask"},
		},
		OnToolApprovalModeChange: func(msg InboundMessage, mode string) error {
			persistedConnection = msg.ConnectionID
			persistedMode = mode
			return nil
		},
	}, nil, logger)
	adapter := newFakeAdapter(PlatformFeishu, "fake-lark")
	msg := InboundMessage{
		Platform:     PlatformFeishu,
		ConnectionID: "feishu-lark",
		Domain:       "lark",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
		Text:         "/yolo on",
	}
	key := BuildSessionKey(msg.Session())
	ctrl := control.New(control.Options{})
	ctrl.SetToolApprovalMode(control.ToolApprovalAsk)
	gw.controllers[key] = &sessionState{ctrl: ctrl}

	gw.handleSlashCommand(context.Background(), adapter, key, msg)

	if got := ctrl.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("current session mode = %q, want yolo", got)
	}
	if got := gw.cfg.ConnectionChannels["feishu-lark"].ToolApprovalMode; got != control.ToolApprovalYolo {
		t.Fatalf("connection default mode = %q, want yolo", got)
	}
	if persistedConnection != "feishu-lark" || persistedMode != control.ToolApprovalYolo {
		t.Fatalf("persisted = %q/%q, want feishu-lark/yolo", persistedConnection, persistedMode)
	}
	sent := adapter.sentMessages()
	if len(sent) != 1 || !strings.Contains(sent[0].Text, "已开启 YOLO") {
		t.Fatalf("sent = %#v, want yolo confirmation", sent)
	}
}

func TestGatewayModeCommandSupportsAskAutoAndStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		ConnectionChannels: map[string]ChannelConfig{
			"weixin-weixin": {ToolApprovalMode: "ask"},
		},
	}, nil, logger)
	adapter := newFakeAdapter(PlatformWeixin, "fake-weixin")
	msg := InboundMessage{
		Platform:     PlatformWeixin,
		ConnectionID: "weixin-weixin",
		Domain:       "weixin",
		ChatType:     ChatDM,
		ChatID:       "chat",
		UserID:       "user",
	}
	key := BuildSessionKey(msg.Session())

	msg.Text = "/mode auto"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	if got := gw.cfg.ConnectionChannels["weixin-weixin"].ToolApprovalMode; got != control.ToolApprovalAuto {
		t.Fatalf("/mode auto default = %q, want auto", got)
	}

	msg.Text = "/yolo off"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	if got := gw.cfg.ConnectionChannels["weixin-weixin"].ToolApprovalMode; got != control.ToolApprovalAsk {
		t.Fatalf("/yolo off default = %q, want ask", got)
	}

	msg.Text = "/mode"
	gw.handleSlashCommand(context.Background(), adapter, key, msg)
	sent := adapter.sentMessages()
	if len(sent) != 3 {
		t.Fatalf("sent count = %d, want 3", len(sent))
	}
	if !strings.Contains(sent[2].Text, "当前工具审批模式：询问") {
		t.Fatalf("status = %q, want ask status", sent[2].Text)
	}
}

func TestGatewayHelpMentionsYoloCommands(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	adapter := newFakeAdapter(PlatformFeishu, "fake-feishu")
	msg := InboundMessage{ChatType: ChatDM, ChatID: "chat", UserID: "user", Text: "/help"}

	gw.handleSlashCommand(context.Background(), adapter, "session-key", msg)

	sent := adapter.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent count = %d, want 1", len(sent))
	}
	if !strings.Contains(sent[0].Text, "/yolo on|off|auto|status") || !strings.Contains(sent[0].Text, "/mode yolo|ask|auto") {
		t.Fatalf("help = %q, want yolo commands", sent[0].Text)
	}
}

func TestGatewayAddsPendingReactionWhenAdapterSupportsIt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{}, nil, logger)
	fa := &fakeReactionAdapter{fakeAdapter: newFakeAdapter(PlatformFeishu, "fake-feishu")}

	gw.addPendingReaction(context.Background(), PlatformFeishu, fa, InboundMessage{MessageID: "om_123"})

	if len(fa.reactions) != 1 || fa.reactions[0] != "om_123" {
		t.Fatalf("reactions = %#v, want [om_123]", fa.reactions)
	}
}

func TestGatewaySessionOptionsUseChannelOverride(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:         "global-model",
		WorkspaceRoot: "/global",
		Channels: map[Platform]ChannelConfig{
			PlatformFeishu: {Model: "feishu-model", WorkspaceRoot: "/feishu"},
			PlatformWeixin: {WorkspaceRoot: "/weixin"},
		},
	}, nil, logger)

	model, root, mode := gw.sessionOptionsForMessage(InboundMessage{Platform: PlatformFeishu})
	if model != "feishu-model" || root != "/feishu" {
		t.Fatalf("feishu options = %q,%q; want channel override", model, root)
	}
	if mode != "ask" {
		t.Fatalf("feishu tool approval mode = %q, want ask", mode)
	}

	model, root, mode = gw.sessionOptionsForMessage(InboundMessage{Platform: PlatformWeixin})
	if model != "global-model" || root != "/weixin" {
		t.Fatalf("weixin options = %q,%q; want global model and channel root", model, root)
	}
	if mode != "ask" {
		t.Fatalf("weixin tool approval mode = %q, want ask", mode)
	}

	model, root, mode = gw.sessionOptionsForMessage(InboundMessage{Platform: PlatformQQ})
	if model != "global-model" || root != "/global" {
		t.Fatalf("qq options = %q,%q; want global defaults", model, root)
	}
	if mode != "ask" {
		t.Fatalf("qq tool approval mode = %q, want ask", mode)
	}
}

func TestGatewaySessionOptionsPreferConnectionOverride(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gw := NewGateway(GatewayConfig{
		Model:         "global-model",
		WorkspaceRoot: "/global",
		Channels: map[Platform]ChannelConfig{
			PlatformFeishu: {Model: "feishu-model", WorkspaceRoot: "/feishu"},
		},
		ConnectionChannels: map[string]ChannelConfig{
			"feishu-lark": {Model: "lark-model", WorkspaceRoot: "/lark"},
		},
	}, nil, logger)

	model, root, mode := gw.sessionOptionsForMessage(InboundMessage{Platform: PlatformFeishu, ConnectionID: "feishu-lark"})
	if model != "lark-model" || root != "/lark" {
		t.Fatalf("lark options = %q,%q; want connection override", model, root)
	}
	if mode != "ask" {
		t.Fatalf("lark tool approval mode = %q, want ask", mode)
	}
}

func TestBotSessionDirUsesProjectWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	got := botSessionDir(userctx.Default(root), root)
	if got == "" || got == botSessionDir(userctx.Default(""), "") {
		t.Fatalf("project session dir = %q, want project-specific dir", got)
	}
}
