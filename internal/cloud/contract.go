// Package cloud defines the profile-bound domain boundary between the desktop
// application and the legacy Reasonix cloud contract. Reasonix remains the
// external protocol identity; wire DTOs and transports belong behind these
// interfaces.
package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"kwu/internal/userctx"
)

type BindingStatus string

const (
	BindingUnbound  BindingStatus = "unbound"
	BindingBinding  BindingStatus = "binding"
	BindingBound    BindingStatus = "bound"
	BindingMismatch BindingStatus = "mismatch"
	BindingExpired  BindingStatus = "expired"
)

// BindingScope is the persistence and recovery boundary for cloud-owned state.
type BindingScope struct {
	ProfileID         string `json:"profile_id"`
	CloudUserID       string `json:"cloud_user_id"`
	BindingGeneration uint64 `json:"binding_generation"`
}

func NewBindingScope(actor userctx.ActorContext, cloudUserID string, generation uint64) (BindingScope, error) {
	actor = actor.Normalize()
	scope := BindingScope{
		ProfileID:         actor.ProfileID,
		CloudUserID:       strings.TrimSpace(cloudUserID),
		BindingGeneration: generation,
	}
	return scope, scope.Validate()
}

func (s BindingScope) Validate() error {
	if !userctx.ValidateProfileID(s.ProfileID) {
		return fmt.Errorf("cloud: invalid profile_id %q", s.ProfileID)
	}
	if err := requireTrimmed("cloud_user_id", s.CloudUserID); err != nil {
		return err
	}
	if s.BindingGeneration == 0 {
		return fmt.Errorf("cloud: binding_generation must be positive")
	}
	return nil
}

func (s BindingScope) key() string {
	return fmt.Sprintf("%s\x00%s\x00%d", s.ProfileID, s.CloudUserID, s.BindingGeneration)
}

type ProfileBinding struct {
	ActorID           string        `json:"actor_id"`
	ProfileID         string        `json:"profile_id"`
	CloudUserID       string        `json:"cloud_user_id,omitempty"`
	Status            BindingStatus `json:"binding_status"`
	BindingGeneration uint64        `json:"binding_generation,omitempty"`
}

func (b ProfileBinding) Validate() error {
	if err := requireTrimmed("actor_id", b.ActorID); err != nil {
		return err
	}
	if !userctx.ValidateProfileID(b.ProfileID) {
		return fmt.Errorf("cloud: invalid profile_id %q", b.ProfileID)
	}
	switch b.Status {
	case BindingUnbound, BindingBinding, BindingBound, BindingMismatch, BindingExpired:
	default:
		return fmt.Errorf("cloud: invalid binding status %q", b.Status)
	}
	if b.Status == BindingBound {
		_, err := b.Scope()
		return err
	}
	return nil
}

func (b ProfileBinding) Scope() (BindingScope, error) {
	scope := BindingScope{
		ProfileID:         b.ProfileID,
		CloudUserID:       b.CloudUserID,
		BindingGeneration: b.BindingGeneration,
	}
	return scope, scope.Validate()
}

type BindingResolver interface {
	ResolveBinding(context.Context, userctx.ActorContext) (ProfileBinding, error)
}

// BindingStore is the mutable binding boundary used by authentication. It is
// implemented by both the in-memory repository and the durable repository.
type BindingStore interface {
	BindingResolver
	ConfirmBinding(context.Context, userctx.ActorContext, string) (ProfileBinding, error)
	Rebind(context.Context, userctx.ActorContext, string) (ProfileBinding, error)
	CompareAndRebind(context.Context, userctx.ActorContext, ProfileBinding, string) (ProfileBinding, error)
}

type SessionState string

const (
	SessionAuthenticated SessionState = "authenticated"
	SessionExpired       SessionState = "expired"
	SessionOffline       SessionState = "offline"
)

type Session struct {
	Scope                 BindingScope `json:"scope"`
	State                 SessionState `json:"state"`
	AccessToken           string       `json:"access_token,omitempty"`
	RefreshToken          string       `json:"refresh_token,omitempty"`
	DeviceID              string       `json:"device_id,omitempty"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at,omitempty"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at,omitempty"`
	TerminationReason     ErrorCode    `json:"termination_reason,omitempty"`
	TerminatedAt          time.Time    `json:"terminated_at,omitempty"`
	// IssuedAt and ExpiresAt are retained for deterministic A0 fixtures. Real
	// sessions use the explicit access/refresh expiry fields above.
	IssuedAt  time.Time `json:"issued_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

func (s Session) Validate() error {
	if err := s.Scope.Validate(); err != nil {
		return err
	}
	if s.State != SessionExpired && (s.TerminationReason != "" || !s.TerminatedAt.IsZero()) {
		return fmt.Errorf("cloud: termination metadata is only valid for expired sessions")
	}
	switch s.State {
	case SessionAuthenticated:
		return s.validateAuthenticated()
	case SessionExpired, SessionOffline:
		if s.State == SessionExpired {
			if (s.TerminationReason == "") != s.TerminatedAt.IsZero() {
				return fmt.Errorf("cloud: expired session termination reason and time must be recorded together")
			}
		}
		return nil
	default:
		return fmt.Errorf("cloud: invalid session state %q", s.State)
	}
}

func (s Session) validateAuthenticated() error {
	if strings.TrimSpace(s.AccessToken) == "" {
		return fmt.Errorf("cloud: authenticated session requires access_token")
	}
	if !s.hasProductionMetadata() {
		return nil
	}
	if err := requireTrimmed("refresh_token", s.RefreshToken); err != nil {
		return err
	}
	if err := requireTrimmed("device_id", s.DeviceID); err != nil {
		return err
	}
	if s.AccessTokenExpiresAt.IsZero() || s.RefreshTokenExpiresAt.IsZero() {
		return fmt.Errorf("cloud: production session requires access and refresh expiry")
	}
	return nil
}

func (s Session) hasProductionMetadata() bool {
	return s.DeviceID != "" || !s.AccessTokenExpiresAt.IsZero() || !s.RefreshTokenExpiresAt.IsZero()
}

type CloudSessionStore interface {
	Save(context.Context, Session) error
	Load(context.Context, BindingScope) (Session, bool, error)
	Clear(context.Context, BindingScope) error
}

// AccountStatus is the UI-facing authentication state. It is distinct from
// SessionState, which is the narrower set of states persisted in Session.
type AccountStatus string

const (
	AccountAnonymous      AccountStatus = "anonymous"
	AccountAuthenticating AccountStatus = "authenticating"
	AccountAuthenticated  AccountStatus = "authenticated"
	AccountRefreshing     AccountStatus = "refreshing"
	AccountExpired        AccountStatus = "expired"
	AccountOffline        AccountStatus = "offline"
)

// AccountOperation identifies a fixture-only account action. The final auth,
// captcha, and password-crypto wire formats remain gated by CLOUD-01/02/03.
type AccountOperation string

const (
	AccountOperationSendRegisterSMS AccountOperation = "send_register_sms"
	AccountOperationRegister        AccountOperation = "register"
	AccountOperationLogin           AccountOperation = "login"
)

// AccountFixtureRequest transports a versioned mock payload only. It is not a
// declaration of the cloud platform's eventual authentication wire format.
type AccountFixtureRequest struct {
	Binding        ProfileBinding   `json:"binding"`
	Operation      AccountOperation `json:"operation"`
	FixtureVersion string           `json:"fixture_version"`
	Payload        json.RawMessage  `json:"payload"`
}

func (r AccountFixtureRequest) Validate() error {
	if err := r.Binding.Validate(); err != nil {
		return err
	}
	if r.Binding.Status != BindingBinding || r.Binding.CloudUserID != "" || r.Binding.BindingGeneration != 0 {
		return fmt.Errorf("cloud: account fixture request requires a binding profile without a cloud scope")
	}
	switch r.Operation {
	case AccountOperationSendRegisterSMS, AccountOperationRegister, AccountOperationLogin:
	default:
		return fmt.Errorf("cloud: invalid account operation %q", r.Operation)
	}
	if err := requireTrimmed("fixture_version", r.FixtureVersion); err != nil {
		return err
	}
	if len(r.Payload) == 0 || !json.Valid(r.Payload) {
		return fmt.Errorf("cloud: account fixture payload must be valid JSON")
	}
	return nil
}

type CloudUser struct {
	CloudUserID string `json:"cloud_user_id"`
	DisplayName string `json:"display_name,omitempty"`
	Role        string `json:"role,omitempty"`
}

// DeviceIdentity is the installation-scoped identity attached to cloud auth
// requests. It is deliberately separate from anonymous telemetry identity.
type DeviceIdentity struct {
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name,omitempty"`
	ClientVersion string `json:"client_version"`
}

func (d DeviceIdentity) Validate() error {
	if err := requireTrimmed("device_id", d.DeviceID); err != nil {
		return err
	}
	if len(d.DeviceID) > 128 {
		return fmt.Errorf("cloud: device_id exceeds 128 bytes")
	}
	if d.DeviceName != "" {
		if strings.TrimSpace(d.DeviceName) != d.DeviceName {
			return fmt.Errorf("cloud: device_name must be trimmed")
		}
		if len(d.DeviceName) > 128 {
			return fmt.Errorf("cloud: device_name exceeds 128 bytes")
		}
	}
	if err := requireTrimmed("client_version", d.ClientVersion); err != nil {
		return err
	}
	if len(d.ClientVersion) > 64 {
		return fmt.Errorf("cloud: client_version exceeds 64 bytes")
	}
	return nil
}

const (
	LoginCaptchaProviderAliyunCaptchaV2 = "aliyun_captcha_v2"
	maxCaptchaTicketBytes               = 8192
)

type CaptchaConfig struct {
	Provider string `json:"provider"`
	Region   string `json:"region"`
	Prefix   string `json:"prefix"`
	SceneID  string `json:"scene_id"`
}

func (c CaptchaConfig) Validate() error {
	if c.Provider != LoginCaptchaProviderAliyunCaptchaV2 {
		return fmt.Errorf("cloud: unsupported captcha provider")
	}
	if c.Region != "cn" {
		return fmt.Errorf("cloud: unsupported captcha region")
	}
	if err := requireTrimmed("captcha prefix", c.Prefix); err != nil {
		return err
	}
	if err := requireTrimmed("captcha scene_id", c.SceneID); err != nil {
		return err
	}
	if c.Prefix == "reasonix-dev" || c.SceneID == "reasonix-register-dev" {
		return fmt.Errorf("cloud: development captcha configuration is not allowed")
	}
	return nil
}

type CaptchaProof struct {
	Ticket   string
	Provider string
}

func (p CaptchaProof) Validate() error {
	if err := requireTrimmed("captcha_ticket", p.Ticket); err != nil {
		return err
	}
	if len(p.Ticket) > maxCaptchaTicketBytes {
		return fmt.Errorf("cloud: captcha_ticket exceeds 8192 bytes")
	}
	if p.Provider != LoginCaptchaProviderAliyunCaptchaV2 {
		return fmt.Errorf("cloud: unsupported captcha_provider")
	}
	return nil
}

type LoginRequest struct {
	Phone           string
	Password        string
	CaptchaTicket   string
	CaptchaProvider string
}

func (r LoginRequest) Validate() error {
	if err := requireTrimmed("phone", r.Phone); err != nil {
		return err
	}
	if r.Password == "" {
		return fmt.Errorf("cloud: password is required")
	}
	return (CaptchaProof{Ticket: r.CaptchaTicket, Provider: r.CaptchaProvider}).Validate()
}

// RemoteSession is unbound wire state. AuthApplication assigns its local
// profile binding before converting it into a durable Session.
type RemoteSession struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	DeviceID              string
}

func (s RemoteSession) Validate() error {
	if err := requireTrimmed("access_token", s.AccessToken); err != nil {
		return err
	}
	if s.AccessTokenExpiresAt.IsZero() {
		return fmt.Errorf("cloud: access token expiry is required")
	}
	if err := requireTrimmed("refresh_token", s.RefreshToken); err != nil {
		return err
	}
	if s.RefreshTokenExpiresAt.IsZero() {
		return fmt.Errorf("cloud: refresh token expiry is required")
	}
	if err := requireTrimmed("device_id", s.DeviceID); err != nil {
		return err
	}
	return nil
}

type AuthenticationResult struct {
	User       CloudUser
	Session    RemoteSession
	ServerTime time.Time
}

// CloudSessionClient is the production authentication transport boundary.
// Registration remains a separate capability and never crosses this API.
type CloudSessionClient interface {
	Login(context.Context, LoginRequest) (AuthenticationResult, error)
	Refresh(context.Context, string) (RemoteSession, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (CurrentUserSnapshot, error)
}

func (u CloudUser) Validate() error {
	if err := requireTrimmed("cloud_user_id", u.CloudUserID); err != nil {
		return err
	}
	if u.DisplayName != "" && strings.TrimSpace(u.DisplayName) != u.DisplayName {
		return fmt.Errorf("cloud: display_name must be trimmed")
	}
	if u.Role != "" && strings.TrimSpace(u.Role) != u.Role {
		return fmt.Errorf("cloud: role must be trimmed")
	}
	return nil
}

type SubscriptionStatus string

const (
	SubscriptionFree    SubscriptionStatus = "free"
	SubscriptionActive  SubscriptionStatus = "active"
	SubscriptionExpired SubscriptionStatus = "expired"
)

// Subscription is the cloud-authoritative entitlement visible to a profile.
type Subscription struct {
	Scope     BindingScope       `json:"scope"`
	Plan      string             `json:"plan"`
	Status    SubscriptionStatus `json:"status"`
	PeriodEnd time.Time          `json:"period_end,omitempty"`
	Version   uint64             `json:"version"`
	// UpdatedAt remains populated by A0 fixtures. The production /me contract
	// is versioned and does not expose this field.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func (s Subscription) Validate() error {
	if err := s.Scope.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("plan", s.Plan); err != nil {
		return err
	}
	switch s.Status {
	case SubscriptionFree:
		if !s.PeriodEnd.IsZero() {
			return fmt.Errorf("cloud: free subscription must not have period_end")
		}
	case SubscriptionActive:
		if s.PeriodEnd.IsZero() {
			return fmt.Errorf("cloud: active subscription requires period_end")
		}
	case SubscriptionExpired:
	default:
		return fmt.Errorf("cloud: invalid subscription status %q", s.Status)
	}
	return nil
}

// AccountState contains UI-safe, profile-bound cloud state. It intentionally
// excludes access and refresh tokens, which remain inside CloudSessionStore.
type AccountState struct {
	Binding      ProfileBinding          `json:"binding"`
	State        AccountStatus           `json:"state"`
	User         CloudUser               `json:"user"`
	Balance      BalanceSnapshot         `json:"balance"`
	Subscription Subscription            `json:"subscription"`
	Capabilities CurrentUserCapabilities `json:"-"`
	LastSyncAt   time.Time               `json:"last_sync_at"`
}

func (s AccountState) Validate() error {
	if err := s.Binding.Validate(); err != nil {
		return err
	}
	scope, err := s.Binding.Scope()
	if err != nil {
		return err
	}
	if err := s.validateBoundState(scope); err != nil {
		return err
	}
	if err := s.validateEntitlements(scope); err != nil {
		return err
	}
	if err := validateCurrentUserCapabilities(s.Capabilities); err != nil {
		return err
	}
	if s.LastSyncAt.IsZero() {
		return fmt.Errorf("cloud: account last_sync_at is required")
	}
	return nil
}

func (s AccountState) validateBoundState(scope BindingScope) error {
	switch s.State {
	case AccountAnonymous, AccountAuthenticating, AccountAuthenticated, AccountRefreshing, AccountExpired, AccountOffline:
	default:
		return fmt.Errorf("cloud: invalid account state %q", s.State)
	}
	if err := s.User.Validate(); err != nil {
		return err
	}
	if s.User.CloudUserID != scope.CloudUserID {
		return fmt.Errorf("cloud: account user belongs to another binding")
	}
	return nil
}

func (s AccountState) validateEntitlements(scope BindingScope) error {
	if err := s.Balance.Validate(); err != nil {
		return err
	}
	if err := s.Subscription.Validate(); err != nil {
		return err
	}
	if s.Subscription.Scope != scope {
		return fmt.Errorf("cloud: subscription belongs to another binding")
	}
	return nil
}

// CloudAccountClient is adapter-owned. Before L2, its auth methods must use
// versioned fixtures; callers must not infer an actual REST or captcha protocol.
type CloudAccountClient interface {
	SendRegisterSMS(context.Context, AccountFixtureRequest) error
	Register(context.Context, AccountFixtureRequest) (AccountState, Session, error)
	Login(context.Context, AccountFixtureRequest) (AccountState, Session, error)
	RefreshSession(context.Context, Session) (Session, error)
	Logout(context.Context, BindingScope) error
	GetCurrentUser(context.Context, BindingScope) (AccountState, error)
}

type FreezeStatus string

const (
	FreezeCreated              FreezeStatus = "created"
	FreezeActive               FreezeStatus = "active"
	FreezeSettling             FreezeStatus = "settling"
	FreezeSettled              FreezeStatus = "settled"
	FreezeReleased             FreezeStatus = "released"
	FreezeExpiredReleased      FreezeStatus = "expired_released"
	FreezeExpiredPendingReview FreezeStatus = "expired_pending_review"
)

type FreezeRequest struct {
	Scope               BindingScope `json:"scope"`
	TaskID              string       `json:"task_id"`
	ModelID             string       `json:"model_id"`
	ClientTaskType      string       `json:"client_task_type"`
	ClientEstimatedCost string       `json:"client_estimated_cost"`
	IdempotencyKey      string       `json:"idempotency_key"`
}

func (r FreezeRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	required := []struct {
		name  string
		value string
	}{
		{"task_id", r.TaskID},
		{"model_id", r.ModelID},
		{"client_task_type", r.ClientTaskType},
		{"client_estimated_cost", r.ClientEstimatedCost},
		{"idempotency_key", r.IdempotencyKey},
	}
	for _, field := range required {
		if err := requireTrimmed(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

type Freeze struct {
	Scope                    BindingScope `json:"scope"`
	FreezeID                 string       `json:"freeze_id"`
	TaskID                   string       `json:"task_id"`
	Status                   FreezeStatus `json:"status"`
	FreezeAmount             string       `json:"freeze_amount"`
	FreeFrozenAmount         string       `json:"free_frozen_amount"`
	SubscriptionFrozenAmount string       `json:"subscription_frozen_amount"`
	LowBalanceMode           bool         `json:"low_balance_mode"`
	ExpiresAt                time.Time    `json:"expires_at"`
}

func (f Freeze) Validate() error {
	if err := f.Scope.Validate(); err != nil {
		return err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"freeze_id", f.FreezeID},
		{"task_id", f.TaskID},
		{"freeze_amount", f.FreezeAmount},
		{"free_frozen_amount", f.FreeFrozenAmount},
		{"subscription_frozen_amount", f.SubscriptionFrozenAmount},
	} {
		if err := requireTrimmed(field.name, field.value); err != nil {
			return err
		}
	}
	switch f.Status {
	case FreezeCreated, FreezeActive, FreezeSettling, FreezeSettled, FreezeReleased,
		FreezeExpiredReleased, FreezeExpiredPendingReview:
	default:
		return fmt.Errorf("cloud: invalid freeze status %q", f.Status)
	}
	if f.ExpiresAt.IsZero() {
		return fmt.Errorf("cloud: freeze expires_at is required")
	}
	return nil
}

type BalanceSnapshot struct {
	TotalBalance                 string `json:"total_balance"`
	FreeBalance                  string `json:"free_balance"`
	SubscriptionBalance          string `json:"subscription_balance"`
	AvailableBalance             string `json:"available_balance"`
	FreeAvailableBalance         string `json:"free_available_balance"`
	SubscriptionAvailableBalance string `json:"subscription_available_balance"`
	FrozenBalance                string `json:"frozen_balance"`
	FreeFrozenBalance            string `json:"free_frozen_balance"`
	SubscriptionFrozenBalance    string `json:"subscription_frozen_balance"`
	CreditPerCNY                 string `json:"credit_per_cny,omitempty"`
	Version                      uint64 `json:"version"`
}

func (b BalanceSnapshot) Validate() error {
	values := []struct {
		name  string
		value string
	}{
		{"total_balance", b.TotalBalance},
		{"free_balance", b.FreeBalance},
		{"subscription_balance", b.SubscriptionBalance},
		{"available_balance", b.AvailableBalance},
		{"free_available_balance", b.FreeAvailableBalance},
		{"subscription_available_balance", b.SubscriptionAvailableBalance},
		{"frozen_balance", b.FrozenBalance},
		{"free_frozen_balance", b.FreeFrozenBalance},
		{"subscription_frozen_balance", b.SubscriptionFrozenBalance},
	}
	for _, field := range values {
		if err := requireTrimmed(field.name, field.value); err != nil {
			return err
		}
	}
	if b.CreditPerCNY != "" && !positiveDecimal(b.CreditPerCNY) {
		return fmt.Errorf("cloud: balance credit_per_cny must be a positive decimal")
	}
	if b.Version == 0 {
		return fmt.Errorf("cloud: balance version must be positive")
	}
	return nil
}

type FreezeResult struct {
	Freeze  Freeze          `json:"freeze"`
	Balance BalanceSnapshot `json:"balance"`
}

func (r FreezeResult) Validate() error {
	if err := r.Freeze.Validate(); err != nil {
		return err
	}
	return r.Balance.Validate()
}

type CloudCreditClient interface {
	CreateFreeze(context.Context, FreezeRequest) (FreezeResult, error)
	GetFreeze(context.Context, BindingScope, string) (FreezeResult, error)
}

// CloudCreditFreezeReleaser is separate from CloudCreditClient so fixture and
// legacy clients cannot accidentally claim support for the real zero-usage
// cancellation contract.
type CloudCreditFreezeReleaser interface {
	ReleaseFreeze(context.Context, ReleaseFreezeRequest) (FreezeResult, error)
}

type EventType string

const (
	EventBalanceUpdated        EventType = "balance.updated"
	EventFreezeCreated         EventType = "freeze.created"
	EventFreezeExpired         EventType = "freeze.expired"
	EventSettlementCompleted   EventType = "settlement.completed"
	EventSettlementException   EventType = "settlement.exception"
	EventSubscriptionUpdated   EventType = "subscription.updated"
	EventPaymentUpdated        EventType = "payment.updated"
	EventThirdPartySyncUpdated EventType = "third_party_sync.updated"
)

type Event struct {
	EventID     string          `json:"event_id"`
	EventType   EventType       `json:"event_type"`
	CloudUserID string          `json:"cloud_user_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Version     uint64          `json:"version"`
	Data        json.RawMessage `json:"data"`
}

func (e Event) Validate() error {
	if err := requireTrimmed("event_id", e.EventID); err != nil {
		return err
	}
	if err := requireTrimmed("cloud_user_id", e.CloudUserID); err != nil {
		return err
	}
	switch e.EventType {
	case EventBalanceUpdated, EventFreezeCreated, EventFreezeExpired,
		EventSettlementCompleted, EventSettlementException, EventSubscriptionUpdated,
		EventPaymentUpdated, EventThirdPartySyncUpdated:
	default:
		return fmt.Errorf("cloud: invalid event type %q", e.EventType)
	}
	if e.OccurredAt.IsZero() || e.Version == 0 {
		return fmt.Errorf("cloud: occurred_at and positive version are required")
	}
	if len(e.Data) == 0 || !json.Valid(e.Data) {
		return fmt.Errorf("cloud: event data must be valid JSON")
	}
	return nil
}

// CloudEventSource connects a normalized event stream to one binding scope.
// Cancelling ctx disconnects the stream.
type CloudEventSource interface {
	Connect(context.Context, BindingScope) (<-chan Event, error)
}

type ThirdPartySyncStatus string

const (
	ThirdPartySyncPending              ThirdPartySyncStatus = "pending"
	ThirdPartySyncRunning              ThirdPartySyncStatus = "running"
	ThirdPartySyncSucceeded            ThirdPartySyncStatus = "succeeded"
	ThirdPartySyncFailedRetryable      ThirdPartySyncStatus = "failed_retryable"
	ThirdPartySyncFailedManualRequired ThirdPartySyncStatus = "failed_manual_required"
	ThirdPartySyncSkipped              ThirdPartySyncStatus = "skipped"
)

// ThirdPartySync is a normalized, credential-free update from the cloud.
// The desktop neither performs nor judges third-party synchronization.
type ThirdPartySync struct {
	Scope     BindingScope         `json:"scope"`
	Status    ThirdPartySyncStatus `json:"status"`
	UpdatedAt time.Time            `json:"updated_at"`
	Version   uint64               `json:"version"`
}

func (s ThirdPartySync) Validate() error {
	if err := s.Scope.Validate(); err != nil {
		return err
	}
	switch s.Status {
	case ThirdPartySyncPending, ThirdPartySyncRunning, ThirdPartySyncSucceeded,
		ThirdPartySyncFailedRetryable, ThirdPartySyncFailedManualRequired, ThirdPartySyncSkipped:
	default:
		return fmt.Errorf("cloud: invalid third-party sync status %q", s.Status)
	}
	if s.UpdatedAt.IsZero() || s.Version == 0 {
		return fmt.Errorf("cloud: third-party sync updated_at and positive version are required")
	}
	return nil
}

type ErrorCode string

const (
	ErrorAuthRequired          ErrorCode = "AUTH_REQUIRED"
	ErrorSliderCaptchaRequired ErrorCode = "SLIDER_CAPTCHA_REQUIRED"
	ErrorSliderCaptchaExpired  ErrorCode = "SLIDER_CAPTCHA_EXPIRED"
	ErrorSMSCodeInvalid        ErrorCode = "SMS_CODE_INVALID"
	ErrorPasswordDecryptFailed ErrorCode = "PASSWORD_DECRYPT_FAILED"
	ErrorSignupRewardGranted   ErrorCode = "SIGNUP_REWARD_ALREADY_GRANTED"
	ErrorProfileScopeMismatch  ErrorCode = "PROFILE_SCOPE_MISMATCH"
	ErrorInsufficientBalance   ErrorCode = "INSUFFICIENT_BALANCE"
	ErrorModelPricing          ErrorCode = "MODEL_PRICING_UNAVAILABLE"
	ErrorFreezeNotFound        ErrorCode = "FREEZE_NOT_FOUND"
	ErrorFreezeExpired         ErrorCode = "FREEZE_EXPIRED"
	ErrorTaskNotCancellable    ErrorCode = "TASK_NOT_CANCELLABLE"
	ErrorIdempotencyConflict   ErrorCode = "IDEMPOTENCY_CONFLICT"
	ErrorSettlementPending     ErrorCode = "SETTLEMENT_PENDING"
	ErrorSettlementNotFound    ErrorCode = "SETTLEMENT_NOT_FOUND"
	ErrorUsageDuplicated       ErrorCode = "USAGE_DUPLICATED"
	ErrorPaymentNotVerified    ErrorCode = "PAYMENT_NOT_VERIFIED"
	ErrorOrderAlreadyGranted   ErrorCode = "ORDER_ALREADY_GRANTED"
	ErrorUpgradeNotAllowed     ErrorCode = "UPGRADE_NOT_ALLOWED"
	ErrorNetworkUnavailable    ErrorCode = "NETWORK_UNAVAILABLE"
)

type Error struct {
	Code       ErrorCode       `json:"code"`
	Message    string          `json:"message"`
	Retryable  bool            `json:"retryable,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
	RequestID  string          `json:"request_id,omitempty"`
	HTTPStatus int             `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	message := e.Message
	if message == "" {
		message = string(e.Code)
	} else {
		message = string(e.Code) + ": " + message
	}
	if requestID := safeRequestID(e.RequestID); requestID != "" {
		message += " [request_id=" + requestID + "]"
	}
	return message
}

// AttachRequestID returns a copy of a structured cloud error with a bounded,
// display-safe request ID. Non-cloud errors are intentionally left unchanged.
func AttachRequestID(err error, requestID string) error {
	requestID = safeRequestID(requestID)
	if err == nil || requestID == "" {
		return err
	}
	var typed *Error
	if !errors.As(err, &typed) {
		return err
	}
	clone := *typed
	clone.RequestID = requestID
	return &clone
}

func safeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	const allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.:"
	for _, char := range value {
		if !strings.ContainsRune(allowed, char) {
			return ""
		}
	}
	return value
}

type RequestMetadata struct {
	RequestID      string `json:"request_id"`
	DeviceID       string `json:"device_id"`
	ClientVersion  string `json:"client_version"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type Envelope[T any] struct {
	Data       T         `json:"data"`
	Error      *Error    `json:"error"`
	RequestID  string    `json:"request_id"`
	ServerTime time.Time `json:"server_time"`
}

const (
	IdempotencyActionCreateFreeze            = "create-freeze"
	IdempotencyActionReleaseFreeze           = "release-freeze"
	IdempotencyActionCreateSubscriptionOrder = "create-subscription-order"
	IdempotencyActionCreateUpgradeOrder      = "create-upgrade-order"
	IdempotencyActionVerifyPaymentOrder      = "verify-payment-order"
	IdempotencyActionSubmitErrorReport       = "submit-error-report"
)

// BuildIdempotencyKey returns a stable, profile-bound key without exposing raw
// profile or cloud user identifiers in a request header.
func BuildIdempotencyKey(action string, scope BindingScope, resourceID string) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	action = strings.TrimSpace(action)
	resourceID = strings.TrimSpace(resourceID)
	if action == "" || resourceID == "" {
		return "", fmt.Errorf("cloud: action and resource_id are required")
	}
	sum := sha256.Sum256([]byte(action + "\x00" + scope.key() + "\x00" + resourceID))
	return "kwu_" + hex.EncodeToString(sum[:16]), nil
}

func requireTrimmed(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("cloud: %s is required", name)
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("cloud: %s must be trimmed", name)
	}
	return nil
}

// AccountView is the token-safe account DTO exposed to the desktop UI.
// It intentionally accepts only AuthView, never a Session.
type AccountView struct {
	Status            AccountStatus    `json:"status"`
	ActorID           string           `json:"actor_id,omitempty"`
	ProfileID         string           `json:"profile_id,omitempty"`
	CloudUserID       string           `json:"cloud_user_id,omitempty"`
	BindingStatus     BindingStatus    `json:"binding_status,omitempty"`
	BindingGeneration uint64           `json:"binding_generation,omitempty"`
	User              *AccountViewUser `json:"user,omitempty"`
	LastSyncAt        *time.Time       `json:"last_sync_at,omitempty"`
}

// AccountViewUser contains only the user data needed for account display.
type AccountViewUser struct {
	DisplayName string `json:"display_name,omitempty"`
}

// AccountViewFromAuth projects the current authentication view into the
// desktop-facing account DTO without exposing cloud session credentials.
func AccountViewFromAuth(auth AuthView) AccountView {
	view := AccountView{
		Status:    auth.State,
		ActorID:   auth.Binding.ActorID,
		ProfileID: auth.Binding.ProfileID,
	}
	if auth.State != AccountAuthenticated {
		return view
	}
	return AccountView{
		Status:            view.Status,
		ActorID:           view.ActorID,
		ProfileID:         view.ProfileID,
		CloudUserID:       auth.Binding.CloudUserID,
		BindingStatus:     auth.Binding.Status,
		BindingGeneration: auth.Binding.BindingGeneration,
		User:              &AccountViewUser{DisplayName: auth.User.DisplayName},
		LastSyncAt:        &auth.LastSyncAt,
	}
}
