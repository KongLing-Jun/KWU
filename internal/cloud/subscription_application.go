package cloud

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/userctx"
)

const subscriptionPlansPath = "/api/v1/subscription-plans"

const maasRechargeOrdersPath = "/api/v1/maas-recharge-orders"

type MaaSRechargeStatus string

const (
	MaaSRechargeCreating       MaaSRechargeStatus = "creating"
	MaaSRechargePaymentPending MaaSRechargeStatus = "payment_pending"
	MaaSRechargePolling        MaaSRechargeStatus = "polling"
	MaaSRechargePaidVerified   MaaSRechargeStatus = "paid_verified"
	MaaSRechargeCredited       MaaSRechargeStatus = "credited"
	MaaSRechargeCreateFailed   MaaSRechargeStatus = "create_failed"
	MaaSRechargePaymentFailed  MaaSRechargeStatus = "payment_failed"
	MaaSRechargeTimeout        MaaSRechargeStatus = "timeout"
	MaaSRechargeManualRequired MaaSRechargeStatus = "manual_required"
)

var validMaaSRechargeStatuses = map[MaaSRechargeStatus]struct{}{
	MaaSRechargeCreating: {}, MaaSRechargePaymentPending: {}, MaaSRechargePolling: {},
	MaaSRechargePaidVerified: {}, MaaSRechargeCredited: {}, MaaSRechargeCreateFailed: {},
	MaaSRechargePaymentFailed: {}, MaaSRechargeTimeout: {}, MaaSRechargeManualRequired: {},
}

type MaaSRechargeOrder struct {
	OrderID               string             `json:"order_id"`
	Amount                string             `json:"amount"`
	AmountUnit            string             `json:"amount_unit"`
	Currency              string             `json:"currency"`
	PaymentMethod         string             `json:"payment_method"`
	RechargeKeyName       string             `json:"recharge_key_name"`
	ConversionRuleVersion string             `json:"conversion_rule_version"`
	QuotaPerUSD           string             `json:"quota_per_usd"`
	CNYPerUSD             string             `json:"cny_per_usd"`
	QuotaDelta            int64              `json:"quota_delta"`
	Status                MaaSRechargeStatus `json:"status"`
	QRCodeURL             string             `json:"qr_code_url"`
	ExpiresAt             time.Time          `json:"expires_at"`
	Paid                  bool               `json:"paid"`
	Credited              bool               `json:"credited"`
	PaidAt                *time.Time         `json:"paid_at,omitempty"`
	CreditedAt            *time.Time         `json:"credited_at,omitempty"`
	RemoteQuotaAfter      *int64             `json:"remote_quota_after,omitempty"`
	Version               uint64             `json:"version"`
}

func (o MaaSRechargeOrder) Validate() error {
	if err := validateMaaSRechargeOrderFields(o); err != nil {
		return err
	}
	if err := validateMaaSRechargeOrderQR(o); err != nil {
		return err
	}
	return validateMaaSRechargeOrderSettlement(o)
}

func validateMaaSRechargeOrderFields(o MaaSRechargeOrder) error {
	if err := validateMaaSRechargeOrderIdentity(o); err != nil {
		return err
	}
	if err := validateMaaSRechargeConversion(o); err != nil {
		return err
	}
	return validateMaaSRechargeLifecycle(o)
}

func validateMaaSRechargeOrderQR(o MaaSRechargeOrder) error {
	if o.QRCodeURL != "" && !validMaaSRechargeQRCode(o.QRCodeURL) {
		return protocolError(fmt.Errorf("cloud: invalid MaaS recharge qr_code_url"))
	}
	awaitingPayment := o.Status == MaaSRechargePaymentPending || o.Status == MaaSRechargePolling
	if awaitingPayment && o.QRCodeURL == "" {
		return protocolError(fmt.Errorf("cloud: MaaS recharge qr_code_url is required while awaiting payment"))
	}
	return nil
}

func validateMaaSRechargeOrderSettlement(o MaaSRechargeOrder) error {
	if o.Credited {
		return validateCreditedMaaSRechargeOrder(o)
	}
	if o.Status == MaaSRechargeCredited {
		return protocolError(fmt.Errorf("cloud: credited MaaS recharge order is incomplete"))
	}
	if o.Paid && o.PaidAt == nil {
		return protocolError(fmt.Errorf("cloud: paid MaaS recharge order is missing paid_at"))
	}
	if !o.Credited && o.RemoteQuotaAfter != nil {
		return protocolError(fmt.Errorf("cloud: uncredited MaaS recharge order includes remote quota"))
	}
	return nil
}

type MaaSRechargeClient interface {
	Create(context.Context, BindingScope, string, string, string) (MaaSRechargeOrder, error)
	Get(context.Context, BindingScope, string) (MaaSRechargeOrder, error)
}

type httpMaaSRechargeClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPMaaSRechargeClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (MaaSRechargeClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: HTTP MaaS recharge client is not configured")
	}
	return &httpMaaSRechargeClient{executor: executor, transport: transport}, nil
}

func (c *httpMaaSRechargeClient) Create(ctx context.Context, scope BindingScope, amount, paymentMethod, idempotencyKey string) (MaaSRechargeOrder, error) {
	if err := scope.Validate(); err != nil {
		return MaaSRechargeOrder{}, err
	}
	amount = strings.TrimSpace(amount)
	if err := validateYuanAmount(amount); err != nil {
		return MaaSRechargeOrder{}, err
	}
	paymentMethod = strings.TrimSpace(paymentMethod)
	if paymentMethod == "" {
		paymentMethod = "alipay"
	}
	if paymentMethod != "alipay" {
		return MaaSRechargeOrder{}, fmt.Errorf("cloud: unsupported MaaS recharge payment method %q", paymentMethod)
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return MaaSRechargeOrder{}, fmt.Errorf("cloud: MaaS recharge idempotency key is required")
	}
	var order MaaSRechargeOrder
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		headers := make(http.Header)
		headers.Set("X-Idempotency-Key", idempotencyKey)
		body := struct {
			Amount        string `json:"amount"`
			PaymentMethod string `json:"payment_method"`
		}{Amount: amount, PaymentMethod: paymentMethod}
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, maasRechargeOrdersPath, body, token, headers, &order)
	})
	if err != nil {
		return MaaSRechargeOrder{}, err
	}
	if err := order.Validate(); err != nil {
		return MaaSRechargeOrder{}, err
	}
	if !equalDecimal(order.Amount, amount) || order.PaymentMethod != paymentMethod {
		return MaaSRechargeOrder{}, protocolError(fmt.Errorf("cloud: MaaS recharge order does not match request"))
	}
	return order, nil
}

func (c *httpMaaSRechargeClient) Get(ctx context.Context, scope BindingScope, orderID string) (MaaSRechargeOrder, error) {
	if err := scope.Validate(); err != nil {
		return MaaSRechargeOrder{}, err
	}
	orderID = strings.TrimSpace(orderID)
	if err := requireTrimmed("order_id", orderID); err != nil {
		return MaaSRechargeOrder{}, err
	}
	var order MaaSRechargeOrder
	orderPath := maasRechargeOrdersPath + "/" + url.PathEscape(orderID)
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, orderPath, nil, token, nil, &order)
	})
	if err != nil {
		return MaaSRechargeOrder{}, err
	}
	if err := order.Validate(); err != nil {
		return MaaSRechargeOrder{}, err
	}
	if order.OrderID != orderID {
		return MaaSRechargeOrder{}, protocolError(fmt.Errorf("cloud: MaaS recharge order identity mismatch"))
	}
	return order, nil
}

// SubscriptionView is the cloud-authoritative subscription data safe for
// desktop display. It deliberately retains the configured plan fields instead
// of deriving prices, credits, periods, or entitlement from local state.
type SubscriptionView struct {
	Plans        []SubscriptionPlan `json:"plans"`
	Subscription Subscription       `json:"subscription"`
}

type SubscriptionApplicationDependencies struct {
	Bindings  BindingResolver
	Accounts  CloudAccountClient
	Plans     SubscriptionPlanClient
	Snapshots AssetSnapshotRepository
}

// SubscriptionApplication refreshes cloud-owned plan configuration and the
// current entitlement. Confirmed snapshots are isolated by the full binding
// scope so an older binding is never visible after a rebind.
type SubscriptionApplication struct {
	bindings  BindingResolver
	accounts  CloudAccountClient
	plans     SubscriptionPlanClient
	snapshots AssetSnapshotRepository
}

func NewSubscriptionApplication(deps SubscriptionApplicationDependencies) (*SubscriptionApplication, error) {
	if isNilDependency(deps.Bindings) || isNilDependency(deps.Accounts) || isNilDependency(deps.Plans) {
		return nil, fmt.Errorf("cloud: subscription application is not configured")
	}
	snapshots := deps.Snapshots
	if isNilDependency(snapshots) {
		snapshots = NewMemoryAssetSnapshotRepository()
	}
	return &SubscriptionApplication{
		bindings:  deps.Bindings,
		accounts:  deps.Accounts,
		plans:     deps.Plans,
		snapshots: snapshots,
	}, nil
}

// Refresh obtains both configured plans and entitlement for the actor's
// current bound scope. A failed refresh returns only that scope's last
// confirmed snapshot with the original error.
func (a *SubscriptionApplication) Refresh(ctx context.Context, actor userctx.ActorContext) (SubscriptionView, error) {
	binding, scope, err := a.currentBindingScope(ctx, actor)
	if err != nil {
		return SubscriptionView{}, err
	}
	cached, _, err := a.snapshots.LoadSubscription(ctx, scope)
	if err != nil {
		return SubscriptionView{}, err
	}

	state, err := a.currentSubscriptionAccount(ctx, binding, scope)
	if err != nil {
		return cached, err
	}

	plans, err := a.plans.ListSubscriptionPlans(ctx, scope)
	if err != nil {
		return cached, err
	}
	if err := validateSubscriptionPlans(plans); err != nil {
		return cached, err
	}

	view := subscriptionView(plans, state.Subscription)
	if err := a.snapshots.StoreSubscription(ctx, scope, view); err != nil {
		return cached, err
	}
	return subscriptionView(view.Plans, view.Subscription), nil
}

func (a *SubscriptionApplication) currentSubscriptionAccount(ctx context.Context, binding ProfileBinding, scope BindingScope) (AccountState, error) {
	state, err := a.accounts.GetCurrentUser(ctx, scope)
	if err != nil {
		return AccountState{}, err
	}
	if err := state.Validate(); err != nil {
		return AccountState{}, err
	}
	stateScope, err := state.Binding.Scope()
	if err != nil {
		return AccountState{}, err
	}
	if stateScope != scope {
		return AccountState{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: subscription snapshot belongs to another binding"}
	}
	state.Binding = binding
	return state, nil
}

func validateSubscriptionPlans(plans []SubscriptionPlan) error {
	for _, plan := range plans {
		if err := plan.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type httpSubscriptionPlanClient struct {
	transport *HTTPSessionClient
}

// NewHTTPSubscriptionPlanClient reads the public cloud plan catalog. The
// binding scope is still validated so callers cannot mix catalog refreshes
// with an invalid account context.
func NewHTTPSubscriptionPlanClient(transport *HTTPSessionClient) (SubscriptionPlanClient, error) {
	if transport == nil {
		return nil, fmt.Errorf("cloud: HTTP subscription plan client is not configured")
	}
	return &httpSubscriptionPlanClient{transport: transport}, nil
}

func (c *httpSubscriptionPlanClient) ListSubscriptionPlans(ctx context.Context, scope BindingScope) ([]SubscriptionPlan, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	var response struct {
		Plans []SubscriptionPlan `json:"plans"`
	}
	if err := c.transport.DoAuthenticatedJSON(ctx, http.MethodGet, subscriptionPlansPath, nil, "", nil, &response); err != nil {
		return nil, err
	}
	if err := validateSubscriptionPlans(response.Plans); err != nil {
		return nil, protocolError(err)
	}
	for _, plan := range response.Plans {
		if strings.TrimSpace(plan.PlanID) == "" {
			return nil, protocolError(fmt.Errorf("cloud: subscription plan_id is required"))
		}
	}
	return append([]SubscriptionPlan(nil), response.Plans...), nil
}

// View returns the last confirmed subscription snapshot only for the actor's
// current complete binding scope.
func (a *SubscriptionApplication) View(ctx context.Context, actor userctx.ActorContext) (SubscriptionView, bool, error) {
	_, scope, err := a.currentBindingScope(ctx, actor)
	if err != nil {
		return SubscriptionView{}, false, err
	}
	return a.snapshots.LoadSubscription(ctx, scope)
}

func (a *SubscriptionApplication) currentBindingScope(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, BindingScope, error) {
	actor = actor.Normalize()
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return ProfileBinding{}, BindingScope{}, err
	}
	if binding.ActorID != actor.ActorID || binding.ProfileID != actor.ProfileID || binding.Status != BindingBound {
		return ProfileBinding{}, BindingScope{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: subscription requires the actor's bound profile"}
	}
	scope, err := binding.Scope()
	if err != nil {
		return ProfileBinding{}, BindingScope{}, err
	}
	return binding, scope, nil
}

func subscriptionView(plans []SubscriptionPlan, subscription Subscription) SubscriptionView {
	return SubscriptionView{Plans: append([]SubscriptionPlan(nil), plans...), Subscription: subscription}
}

const currentMaaSKeyPath = "/api/v1/me/maas-key"

// MaaSKeyStatus is the cloud-side lifecycle state for the current user's
// dedicated model-gateway credential.
type MaaSKeyStatus string

const (
	MaaSKeyCreatePending MaaSKeyStatus = "create_pending"
	MaaSKeyCreating      MaaSKeyStatus = "creating"
	MaaSKeyReady         MaaSKeyStatus = "ready"
	MaaSKeyRetryWait     MaaSKeyStatus = "retry_wait"
	MaaSKeyManual        MaaSKeyStatus = "manual_required"
)

// MaaSKeyAccount is scoped to the authenticated BindingScope supplied to Get.
// Key is intentionally kept out of all String/Error representations.
type MaaSKeyAccount struct {
	Status  MaaSKeyStatus
	KeyName string
	KeyID   *int64
	Key     string
}

func (a MaaSKeyAccount) Validate(scope BindingScope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := a.validateIdentity(scope); err != nil {
		return err
	}
	return a.validateState()
}

func (a MaaSKeyAccount) validateIdentity(scope BindingScope) error {
	if strings.TrimSpace(a.KeyName) == "" || a.KeyName != strings.TrimSpace(a.KeyName) {
		return fmt.Errorf("cloud: MaaS key_name is required")
	}
	if a.KeyName != scope.CloudUserID {
		return fmt.Errorf("cloud: MaaS key identity does not match the authenticated user")
	}
	return nil
}

func (a MaaSKeyAccount) validateState() error {
	switch a.Status {
	case MaaSKeyCreatePending, MaaSKeyCreating, MaaSKeyRetryWait, MaaSKeyManual:
		if a.Key != "" {
			return fmt.Errorf("cloud: MaaS key must be empty before ready")
		}
	case MaaSKeyReady:
		if a.KeyID == nil || *a.KeyID <= 0 {
			return fmt.Errorf("cloud: ready MaaS key_id is required")
		}
		if a.Key != strings.TrimSpace(a.Key) || !strings.HasPrefix(a.Key, "sk-") || len(a.Key) <= len("sk-") {
			return fmt.Errorf("cloud: ready MaaS key is invalid")
		}
	default:
		return fmt.Errorf("cloud: unsupported MaaS key status %q", a.Status)
	}
	return nil
}

// MaaSKeyClient returns the current authenticated user's dedicated credential.
type MaaSKeyClient interface {
	Get(context.Context, BindingScope) (MaaSKeyAccount, error)
}

type httpMaaSKeyClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPMaaSKeyClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (MaaSKeyClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: HTTP MaaS key client is not configured")
	}
	return &httpMaaSKeyClient{executor: executor, transport: transport}, nil
}

func (c *httpMaaSKeyClient) Get(ctx context.Context, scope BindingScope) (result MaaSKeyAccount, err error) {
	if err := scope.Validate(); err != nil {
		return MaaSKeyAccount{}, err
	}
	err = c.executor.Do(ctx, scope, func(ctx context.Context, token string) error {
		var wire maasKeyAccountWire
		if requestErr := c.transport.DoAuthenticatedJSON(ctx, http.MethodGet, currentMaaSKeyPath, nil, token, nil, &wire); requestErr != nil {
			return requestErr
		}
		candidate := wire.domain()
		if validateErr := candidate.Validate(scope); validateErr != nil {
			return protocolError(validateErr)
		}
		result = candidate
		return nil
	})
	return result, err
}

type maasKeyAccountWire struct {
	Status  MaaSKeyStatus `json:"status"`
	KeyName string        `json:"key_name"`
	KeyID   *int64        `json:"key_id"`
	Key     *string       `json:"maas_key"`
}

func (w maasKeyAccountWire) domain() MaaSKeyAccount {
	result := MaaSKeyAccount{Status: w.Status, KeyName: w.KeyName, KeyID: w.KeyID}
	if w.Key != nil {
		result.Key = *w.Key
	}
	return result
}

const videoGenerationTasksPath = "/api/v1/video-generation-tasks"

type VideoGenerationRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	ImageURL    string `json:"image_url,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	CameraFixed *bool  `json:"camera_fixed,omitempty"`
	Watermark   *bool  `json:"watermark,omitempty"`
}

type VideoGenerationTask struct {
	TaskID            string    `json:"task_id"`
	Model             string    `json:"model"`
	Status            string    `json:"status"`
	SettlementStatus  string    `json:"settlement_status"`
	ReservedCredits   string    `json:"reserved_credits"`
	PricingVersion    string    `json:"pricing_version"`
	TotalTokens       *int64    `json:"total_tokens,omitempty"`
	CostCNY           *string   `json:"cost_cny,omitempty"`
	CostUSD           *string   `json:"cost_usd,omitempty"`
	ChargedCredits    *string   `json:"charged_credits,omitempty"`
	VideoURL          *string   `json:"video_url,omitempty"`
	ProviderErrorCode *string   `json:"provider_error_code,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type VideoGenerationProgress struct {
	TaskID            string    `json:"task_id"`
	Status            string    `json:"status"`
	SettlementStatus  string    `json:"settlement_status"`
	ProgressPercent   int       `json:"progress_percent"`
	ProgressBasis     string    `json:"progress_basis"`
	Terminal          bool      `json:"terminal"`
	Succeeded         bool      `json:"succeeded"`
	ChargedCredits    *string   `json:"charged_credits,omitempty"`
	RetryAfterSeconds int       `json:"retry_after_seconds"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type VideoGenerationClient interface {
	Create(context.Context, BindingScope, string, VideoGenerationRequest) (VideoGenerationTask, error)
	Get(context.Context, BindingScope, string) (VideoGenerationTask, error)
	GetProgress(context.Context, BindingScope, string) (VideoGenerationProgress, error)
}

type httpVideoGenerationClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPVideoGenerationClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (VideoGenerationClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: video generation client is not configured")
	}
	return &httpVideoGenerationClient{executor: executor, transport: transport}, nil
}

func (c *httpVideoGenerationClient) Create(ctx context.Context, scope BindingScope, idempotencyKey string, input VideoGenerationRequest) (VideoGenerationTask, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 256 {
		return VideoGenerationTask{}, fmt.Errorf("cloud: video generation idempotency key is invalid")
	}
	var task VideoGenerationTask
	headers := make(http.Header)
	headers.Set("X-Idempotency-Key", idempotencyKey)
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, accessToken string) error {
		return c.transport.DoAuthenticatedJSON(
			callCtx, http.MethodPost, videoGenerationTasksPath, input, accessToken, headers, &task,
		)
	})
	return task, err
}

func (c *httpVideoGenerationClient) Get(ctx context.Context, scope BindingScope, taskID string) (VideoGenerationTask, error) {
	taskID, err := normalizeVideoGenerationTaskID(taskID)
	if err != nil {
		return VideoGenerationTask{}, fmt.Errorf("cloud: video generation task ID is invalid")
	}
	var task VideoGenerationTask
	err = c.executor.Do(ctx, scope, func(callCtx context.Context, accessToken string) error {
		return c.transport.DoAuthenticatedJSON(
			callCtx, http.MethodGet, videoGenerationTasksPath+"/"+url.PathEscape(taskID), nil, accessToken, nil, &task,
		)
	})
	return task, err
}

func (c *httpVideoGenerationClient) GetProgress(ctx context.Context, scope BindingScope, taskID string) (VideoGenerationProgress, error) {
	taskID, err := normalizeVideoGenerationTaskID(taskID)
	if err != nil {
		return VideoGenerationProgress{}, fmt.Errorf("cloud: video generation task ID is invalid")
	}
	var progress VideoGenerationProgress
	err = c.executor.Do(ctx, scope, func(callCtx context.Context, accessToken string) error {
		return c.transport.DoAuthenticatedJSON(
			callCtx, http.MethodGet, videoGenerationTasksPath+"/"+url.PathEscape(taskID)+"/progress", nil, accessToken, nil, &progress,
		)
	})
	return progress, err
}

func normalizeVideoGenerationTaskID(taskID string) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || len(taskID) > 512 || url.PathEscape(taskID) != taskID {
		return "", fmt.Errorf("invalid task ID")
	}
	return taskID, nil
}

var _ VideoGenerationClient = (*httpVideoGenerationClient)(nil)

const (
	mediaUnderstandingsPath = "/api/v1/media-understandings"
	MediaUnderstandingModel = "doubao-seed-2-0-lite-260428"
)

type MediaUnderstandingRequest struct {
	MediaType string `json:"media_type"`
	MediaURL  string `json:"media_url"`
	Prompt    string `json:"prompt,omitempty"`
}

type MediaUnderstandingUsage struct {
	InputTokens      int64 `json:"input_tokens"`
	AudioInputTokens int64 `json:"audio_input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type MediaUnderstandingResult struct {
	TaskID           string                  `json:"task_id"`
	Model            string                  `json:"model"`
	MediaType        string                  `json:"media_type"`
	Text             string                  `json:"text"`
	Status           string                  `json:"status"`
	SettlementStatus string                  `json:"settlement_status"`
	Usage            MediaUnderstandingUsage `json:"usage"`
	PricingVersion   string                  `json:"pricing_version"`
	CostCNY          string                  `json:"cost_cny"`
	ChargedCredits   string                  `json:"charged_credits"`
	CreatedAt        time.Time               `json:"created_at"`
	CompletedAt      time.Time               `json:"completed_at"`
}

type MediaUnderstandingClient interface {
	UnderstandURL(context.Context, BindingScope, string, MediaUnderstandingRequest) (MediaUnderstandingResult, error)
	UnderstandFile(context.Context, BindingScope, string, string, string, string, []byte) (MediaUnderstandingResult, error)
}

type httpMediaUnderstandingClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPMediaUnderstandingClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (MediaUnderstandingClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: media understanding client is not configured")
	}
	return &httpMediaUnderstandingClient{executor: executor, transport: transport}, nil
}

func (c *httpMediaUnderstandingClient) UnderstandURL(
	ctx context.Context,
	scope BindingScope,
	idempotencyKey string,
	input MediaUnderstandingRequest,
) (MediaUnderstandingResult, error) {
	headers, err := mediaUnderstandingHeaders(idempotencyKey)
	if err != nil {
		return MediaUnderstandingResult{}, err
	}
	var result MediaUnderstandingResult
	err = c.executor.Do(ctx, scope, func(callCtx context.Context, accessToken string) error {
		return c.transport.DoAuthenticatedJSON(
			callCtx, http.MethodPost, mediaUnderstandingsPath, input, accessToken, headers, &result,
		)
	})
	return result, err
}

func (c *httpMediaUnderstandingClient) UnderstandFile(
	ctx context.Context,
	scope BindingScope,
	idempotencyKey string,
	mediaType string,
	prompt string,
	filename string,
	content []byte,
) (MediaUnderstandingResult, error) {
	headers, err := mediaUnderstandingHeaders(idempotencyKey)
	if err != nil {
		return MediaUnderstandingResult{}, err
	}
	payload, contentType, err := mediaUnderstandingMultipart(mediaType, prompt, filename, content)
	if err != nil {
		return MediaUnderstandingResult{}, err
	}
	var result MediaUnderstandingResult
	err = c.executor.Do(ctx, scope, func(callCtx context.Context, accessToken string) error {
		return c.transport.DoAuthenticatedMultipartJSON(
			callCtx, http.MethodPost, mediaUnderstandingsPath, payload, contentType, accessToken, headers, &result,
		)
	})
	return result, err
}

func mediaUnderstandingHeaders(idempotencyKey string) (http.Header, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 256 {
		return nil, fmt.Errorf("cloud: media understanding idempotency key is invalid")
	}
	headers := make(http.Header)
	headers.Set("X-Idempotency-Key", idempotencyKey)
	return headers, nil
}

func mediaUnderstandingMultipart(mediaType, prompt, filename string, content []byte) ([]byte, string, error) {
	filename = filepath.Base(strings.ReplaceAll(strings.TrimSpace(filename), `\`, "/"))
	if filename == "" || filename == "." || len(content) == 0 {
		return nil, "", fmt.Errorf("cloud: media understanding file is invalid")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("media_type", mediaType); err != nil {
		return nil, "", fmt.Errorf("cloud: encode media_type: %w", err)
	}
	if prompt != "" {
		if err := writer.WriteField("prompt", prompt); err != nil {
			return nil, "", fmt.Errorf("cloud: encode prompt: %w", err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", fmt.Errorf("cloud: encode media file: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", fmt.Errorf("cloud: encode media file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("cloud: finish media form: %w", err)
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

var _ MediaUnderstandingClient = (*httpMediaUnderstandingClient)(nil)

const expertFavoritesPath = "/api/v1/expert-favorites"
const expertGroupFavoritesPath = "/api/v1/expert-group-favorites"

type ExpertFavorite struct {
	FavoriteID string    `json:"favorite_id"`
	ExpertID   string    `json:"expert_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ExpertGroupFavorite struct {
	FavoriteID    string    `json:"favorite_id"`
	ExpertGroupID string    `json:"expert_group_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ExpertFavoritesClient interface {
	ListExpertFavorites(context.Context, BindingScope) ([]ExpertFavorite, error)
	ListExpertGroupFavorites(context.Context, BindingScope) ([]ExpertGroupFavorite, error)
	CreateExpertFavorite(context.Context, BindingScope, string) (ExpertFavorite, error)
	CreateExpertGroupFavorite(context.Context, BindingScope, string) (ExpertGroupFavorite, error)
	DeleteExpertFavorite(context.Context, BindingScope, string) error
	DeleteExpertGroupFavorite(context.Context, BindingScope, string) error
}

func (c *httpExpertAssetsClient) ListExpertFavorites(ctx context.Context, scope BindingScope) ([]ExpertFavorite, error) {
	seenFavorites := make(map[string]struct{})
	seenExperts := make(map[string]struct{})
	return listUserExpertAssets(ctx, c, scope, expertFavoritesPath, jsonContentTypeHeaders(), func(item *ExpertFavorite, _ BindingScope) error {
		if err := normalizeExpertFavorite(item); err != nil {
			return err
		}
		if _, duplicate := seenFavorites[item.FavoriteID]; duplicate {
			return fmt.Errorf("expert favorites contain duplicate favorite id %q", item.FavoriteID)
		}
		if _, duplicate := seenExperts[item.ExpertID]; duplicate {
			return fmt.Errorf("expert favorites contain duplicate expert id %q", item.ExpertID)
		}
		seenFavorites[item.FavoriteID] = struct{}{}
		seenExperts[item.ExpertID] = struct{}{}
		return nil
	})
}

func (c *httpExpertAssetsClient) ListExpertGroupFavorites(ctx context.Context, scope BindingScope) ([]ExpertGroupFavorite, error) {
	seenFavorites := make(map[string]struct{})
	seenGroups := make(map[string]struct{})
	return listUserExpertAssets(ctx, c, scope, expertGroupFavoritesPath, jsonContentTypeHeaders(), func(item *ExpertGroupFavorite, _ BindingScope) error {
		if err := normalizeExpertGroupFavorite(item); err != nil {
			return err
		}
		if _, duplicate := seenFavorites[item.FavoriteID]; duplicate {
			return fmt.Errorf("expert group favorites contain duplicate favorite id %q", item.FavoriteID)
		}
		if _, duplicate := seenGroups[item.ExpertGroupID]; duplicate {
			return fmt.Errorf("expert group favorites contain duplicate expert group id %q", item.ExpertGroupID)
		}
		seenFavorites[item.FavoriteID] = struct{}{}
		seenGroups[item.ExpertGroupID] = struct{}{}
		return nil
	})
}

func (c *httpExpertAssetsClient) CreateExpertFavorite(ctx context.Context, scope BindingScope, expertID string) (ExpertFavorite, error) {
	expertID = strings.TrimSpace(expertID)
	if expertID == "" {
		return ExpertFavorite{}, fmt.Errorf("expert id is required")
	}
	var response struct {
		Favorite ExpertFavorite `json:"expert_favorite"`
	}
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, expertFavoritesPath, map[string]string{
			"expert_id": expertID,
		}, token, nil, &response)
	})
	if err != nil {
		return ExpertFavorite{}, err
	}
	if err := normalizeExpertFavorite(&response.Favorite); err != nil {
		return ExpertFavorite{}, protocolError(err)
	}
	if response.Favorite.ExpertID != expertID {
		return ExpertFavorite{}, protocolError(fmt.Errorf("create expert favorite returned mismatched expert_id"))
	}
	return response.Favorite, nil
}

func (c *httpExpertAssetsClient) CreateExpertGroupFavorite(ctx context.Context, scope BindingScope, expertGroupID string) (ExpertGroupFavorite, error) {
	expertGroupID = strings.TrimSpace(expertGroupID)
	if expertGroupID == "" {
		return ExpertGroupFavorite{}, fmt.Errorf("expert group id is required")
	}
	var response struct {
		Favorite ExpertGroupFavorite `json:"expert_group_favorite"`
	}
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, expertGroupFavoritesPath, map[string]string{
			"expert_group_id": expertGroupID,
		}, token, nil, &response)
	})
	if err != nil {
		return ExpertGroupFavorite{}, err
	}
	if err := normalizeExpertGroupFavorite(&response.Favorite); err != nil {
		return ExpertGroupFavorite{}, protocolError(err)
	}
	if response.Favorite.ExpertGroupID != expertGroupID {
		return ExpertGroupFavorite{}, protocolError(fmt.Errorf("create expert group favorite returned mismatched expert_group_id"))
	}
	return response.Favorite, nil
}

func (c *httpExpertAssetsClient) DeleteExpertFavorite(ctx context.Context, scope BindingScope, favoriteID string) error {
	return c.deleteFavorite(ctx, scope, expertFavoritesPath, favoriteID)
}

func (c *httpExpertAssetsClient) DeleteExpertGroupFavorite(ctx context.Context, scope BindingScope, favoriteID string) error {
	return c.deleteFavorite(ctx, scope, expertGroupFavoritesPath, favoriteID)
}

func (c *httpExpertAssetsClient) deleteFavorite(ctx context.Context, scope BindingScope, collectionPath, favoriteID string) error {
	favoriteID = strings.TrimSpace(favoriteID)
	if favoriteID == "" {
		return fmt.Errorf("favorite id is required")
	}
	var response struct {
		DeletedFavoriteID string `json:"deleted_favorite_id"`
	}
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodDelete, collectionPath+"/"+url.PathEscape(favoriteID), nil, token, jsonContentTypeHeaders(), &response)
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(response.DeletedFavoriteID) != favoriteID {
		return protocolError(fmt.Errorf("delete favorite returned mismatched deleted_favorite_id"))
	}
	return nil
}

func normalizeExpertFavorite(item *ExpertFavorite) error {
	item.FavoriteID = strings.TrimSpace(item.FavoriteID)
	item.ExpertID = strings.TrimSpace(item.ExpertID)
	if item.FavoriteID == "" || item.ExpertID == "" || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return fmt.Errorf("expert favorite response is incomplete")
	}
	return nil
}

func normalizeExpertGroupFavorite(item *ExpertGroupFavorite) error {
	item.FavoriteID = strings.TrimSpace(item.FavoriteID)
	item.ExpertGroupID = strings.TrimSpace(item.ExpertGroupID)
	if item.FavoriteID == "" || item.ExpertGroupID == "" || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return fmt.Errorf("expert group favorite response is incomplete")
	}
	return nil
}

func jsonContentTypeHeaders() http.Header {
	return http.Header{"Content-Type": []string{"application/json"}}
}
