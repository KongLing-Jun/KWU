package cloud

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var yuanAmountPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]{1,2})?$`)

func validateMaaSRechargeOrderIdentity(o MaaSRechargeOrder) error {
	if err := requireTrimmed("order_id", o.OrderID); err != nil {
		return protocolError(err)
	}
	if err := validateYuanAmount(o.Amount); err != nil {
		return protocolError(err)
	}
	if o.AmountUnit != "yuan" || o.Currency != "CNY" || o.PaymentMethod != "alipay" {
		return protocolError(fmt.Errorf("cloud: invalid MaaS recharge currency or payment method"))
	}
	return nil
}

func validateMaaSRechargeConversion(o MaaSRechargeOrder) error {
	if strings.TrimSpace(o.RechargeKeyName) == "" || strings.TrimSpace(o.ConversionRuleVersion) == "" {
		return protocolError(fmt.Errorf("cloud: MaaS recharge key and conversion rule are required"))
	}
	if !positiveDecimal(o.QuotaPerUSD) || !positiveDecimal(o.CNYPerUSD) || o.QuotaDelta <= 0 {
		return protocolError(fmt.Errorf("cloud: invalid MaaS recharge conversion values"))
	}
	return nil
}

func validateMaaSRechargeLifecycle(o MaaSRechargeOrder) error {
	_, valid := validMaaSRechargeStatuses[o.Status]
	if !valid || o.ExpiresAt.IsZero() || o.Version == 0 {
		return protocolError(fmt.Errorf("cloud: invalid MaaS recharge status, expiry, or version"))
	}
	return nil
}

func validateCreditedMaaSRechargeOrder(o MaaSRechargeOrder) error {
	if !o.Paid || o.Status != MaaSRechargeCredited {
		return protocolError(fmt.Errorf("cloud: inconsistent credited MaaS recharge order"))
	}
	if o.CreditedAt == nil || o.RemoteQuotaAfter == nil {
		return protocolError(fmt.Errorf("cloud: credited MaaS recharge settlement is incomplete"))
	}
	return nil
}

func validMaaSRechargeQRCode(raw string) bool {
	return raw != "" && strings.TrimSpace(raw) == raw
}

func validateYuanAmount(amount string) error {
	if !yuanAmountPattern.MatchString(amount) || !positiveDecimal(amount) {
		return fmt.Errorf("cloud: MaaS recharge amount must be a positive yuan amount with at most two decimals")
	}
	return nil
}

func positiveDecimal(value string) bool {
	parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	return ok && parsed.Sign() > 0
}

func equalDecimal(left, right string) bool {
	leftValue, leftOK := new(big.Rat).SetString(strings.TrimSpace(left))
	rightValue, rightOK := new(big.Rat).SetString(strings.TrimSpace(right))
	return leftOK && rightOK && leftValue.Cmp(rightValue) == 0
}

const (
	maasKeyBalancePath         = "/api/v1/me/maas-key/balance"
	maasRechargeKeyBalancePath = "/api/v1/me/maas-recharge-key/balance"
)

const (
	maasKeyBalanceSource         = "maas_api_key"
	maasRechargeKeyBalanceSource = "maas_recharge_api_key"
)

var creditAmountPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]*)\.[0-9]{4}$`)

// MaaSKeyBalance is the server-authoritative converted credit balance for the
// authenticated user's subscription MaaS key.
type MaaSKeyBalance struct {
	AvailableBalance      string    `json:"available_balance"`
	RemainQuota           int64     `json:"remain_quota"`
	Source                string    `json:"source"`
	ConversionRuleVersion string    `json:"conversion_rule_version"`
	QuotaPerUSD           string    `json:"quota_per_usd"`
	CNYPerUSD             string    `json:"cny_per_usd"`
	CreditPerCNY          string    `json:"credit_per_cny"`
	QueriedAt             time.Time `json:"queried_at"`
}

func (b MaaSKeyBalance) Validate() error {
	return validateMaaSKeyBalance(b.AvailableBalance, b.RemainQuota, b.Source, maasKeyBalanceSource,
		b.ConversionRuleVersion, b.QuotaPerUSD, b.CNYPerUSD, b.CreditPerCNY, b.QueriedAt)
}

// MaaSKeyBalanceClient reads the balance for the authenticated user's
// subscription key. The API intentionally accepts no request body.
type MaaSKeyBalanceClient interface {
	Get(context.Context, BindingScope) (MaaSKeyBalance, error)
}

type httpMaaSKeyBalanceClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPMaaSKeyBalanceClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (MaaSKeyBalanceClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: HTTP MaaS subscription balance client is not configured")
	}
	return &httpMaaSKeyBalanceClient{executor: executor, transport: transport}, nil
}

func (c *httpMaaSKeyBalanceClient) Get(ctx context.Context, scope BindingScope) (MaaSKeyBalance, error) {
	if err := scope.Validate(); err != nil {
		return MaaSKeyBalance{}, err
	}
	var balance MaaSKeyBalance
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, maasKeyBalancePath, nil, token, nil, &balance)
	})
	if err != nil {
		return MaaSKeyBalance{}, err
	}
	if err := balance.Validate(); err != nil {
		return MaaSKeyBalance{}, err
	}
	return balance, nil
}

// MaaSRechargeKeyBalance is the current user's server-authoritative balance
// for the dedicated recharge (cz) key. It intentionally contains no API key.
type MaaSRechargeKeyBalance struct {
	KeyName               string    `json:"key_name"`
	AvailableBalance      string    `json:"available_balance"`
	RemainQuota           int64     `json:"remain_quota"`
	BalanceCNY            string    `json:"balance_cny"`
	Source                string    `json:"source"`
	ConversionRuleVersion string    `json:"conversion_rule_version"`
	QuotaPerUSD           string    `json:"quota_per_usd"`
	CNYPerUSD             string    `json:"cny_per_usd"`
	CreditPerCNY          string    `json:"credit_per_cny"`
	QueriedAt             time.Time `json:"queried_at"`
	NotFound              bool      `json:"-"`
}

func (b MaaSRechargeKeyBalance) Validate() error {
	if b.NotFound {
		return nil
	}
	if b.KeyName == "" || strings.TrimSpace(b.KeyName) != b.KeyName || !strings.HasSuffix(b.KeyName, "cz") {
		return protocolError(fmt.Errorf("cloud: invalid MaaS recharge key name"))
	}
	if b.BalanceCNY != "" && (strings.TrimSpace(b.BalanceCNY) != b.BalanceCNY || !nonNegativeDecimal(b.BalanceCNY)) {
		return protocolError(fmt.Errorf("cloud: invalid MaaS recharge yuan balance"))
	}
	return validateMaaSKeyBalance(b.AvailableBalance, b.RemainQuota, b.Source, maasRechargeKeyBalanceSource,
		b.ConversionRuleVersion, b.QuotaPerUSD, b.CNYPerUSD, b.CreditPerCNY, b.QueriedAt)
}

func validateMaaSKeyBalance(availableBalance string, remainQuota int64, source, expectedSource, ruleVersion, quotaPerUSD, cnyPerUSD, creditPerCNY string, queriedAt time.Time) error {
	if !creditAmountPattern.MatchString(availableBalance) || remainQuota < 0 {
		return protocolError(fmt.Errorf("cloud: invalid MaaS available balance or remaining quota"))
	}
	if source != expectedSource || !validMaaSRuleVersion(ruleVersion) {
		return protocolError(fmt.Errorf("cloud: invalid MaaS balance source or conversion rule"))
	}
	if !validMaaSConversionValues(quotaPerUSD, cnyPerUSD, creditPerCNY, queriedAt) {
		return protocolError(fmt.Errorf("cloud: invalid MaaS balance conversion values or query time"))
	}
	return nil
}

func validMaaSRuleVersion(ruleVersion string) bool {
	return ruleVersion != "" && strings.TrimSpace(ruleVersion) == ruleVersion
}

func validMaaSConversionValues(quotaPerUSD, cnyPerUSD, creditPerCNY string, queriedAt time.Time) bool {
	return strings.TrimSpace(quotaPerUSD) == quotaPerUSD &&
		strings.TrimSpace(cnyPerUSD) == cnyPerUSD &&
		strings.TrimSpace(creditPerCNY) == creditPerCNY &&
		positiveDecimal(quotaPerUSD) && positiveDecimal(cnyPerUSD) && positiveDecimal(creditPerCNY) &&
		!queriedAt.IsZero()
}

func nonNegativeDecimal(value string) bool {
	parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	return ok && parsed.Sign() >= 0
}

// MaaSRechargeKeyBalanceClient reads the balance for the authenticated user's
// recharge key. The API intentionally accepts no caller-supplied user or key.
type MaaSRechargeKeyBalanceClient interface {
	Get(context.Context, BindingScope) (MaaSRechargeKeyBalance, error)
}

type httpMaaSRechargeKeyBalanceClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPMaaSRechargeKeyBalanceClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (MaaSRechargeKeyBalanceClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: HTTP MaaS recharge balance client is not configured")
	}
	return &httpMaaSRechargeKeyBalanceClient{executor: executor, transport: transport}, nil
}

func (c *httpMaaSRechargeKeyBalanceClient) Get(ctx context.Context, scope BindingScope) (MaaSRechargeKeyBalance, error) {
	if err := scope.Validate(); err != nil {
		return MaaSRechargeKeyBalance{}, err
	}
	var balance MaaSRechargeKeyBalance
	var notFound bool
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		envelope, requestErr := c.transport.doWithHeaders(callCtx, http.MethodGet, maasRechargeKeyBalancePath, nil, token, nil, &balance)
		if requestErr != nil && envelope.StatusCode == http.StatusNotFound && isCloudCode(requestErr, ErrorCode("NOT_FOUND")) {
			notFound = true
			return nil
		}
		return requestErr
	})
	if err != nil {
		return MaaSRechargeKeyBalance{}, err
	}
	if notFound {
		return MaaSRechargeKeyBalance{AvailableBalance: "0.0000", NotFound: true}, nil
	}
	if err := balance.Validate(); err != nil {
		return MaaSRechargeKeyBalance{}, err
	}
	return balance, nil
}

var _ MaaSRechargeKeyBalanceClient = (*httpMaaSRechargeKeyBalanceClient)(nil)
var _ MaaSKeyBalanceClient = (*httpMaaSKeyBalanceClient)(nil)

// FixturePaymentClient is the deterministic CLOUD-08/09 L1 adapter backed by
// the canonical A0 create matrix and tapes. Callers select only domain inputs;
// fixture keys and canonical profile/generation never cross this boundary.
type FixturePaymentClient struct {
	authority *FixtureCloudAuthority
}

func NewFixturePaymentClient(bundle FixtureBundle, authority *FixtureCloudAuthority) (*FixturePaymentClient, error) {
	if bundle.Version != A0FixtureVersion {
		return nil, fmt.Errorf("cloud: fixture payment client requires %s", A0FixtureVersion)
	}
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	if authority == nil || authority.fixtureVersion != bundle.Version || isNilDependency(authority.orders) {
		return nil, fmt.Errorf("cloud: fixture payment client requires the canonical authority")
	}
	return &FixturePaymentClient{authority: authority}, nil
}

func (c *FixturePaymentClient) ListSubscriptionPlans(ctx context.Context, scope BindingScope) ([]SubscriptionPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := c.authority.canonicalAccount(scope); err != nil {
		return nil, err
	}
	return append([]SubscriptionPlan(nil), c.authority.plans...), nil
}

func (c *FixturePaymentClient) CreateSubscriptionOrder(ctx context.Context, request CreateSubscriptionOrderRequest) (PaymentOrderResult, error) {
	if err := ctx.Err(); err != nil {
		return PaymentOrderResult{}, err
	}
	if err := request.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	if _, err := c.authority.canonicalAccount(request.Scope); err != nil {
		return PaymentOrderResult{}, err
	}
	planID := request.PlanID
	if planID == "" {
		planID = request.Plan
	}
	plan, err := c.fixturePlanCode(planID)
	if err != nil {
		return PaymentOrderResult{}, err
	}
	result, err := c.create(request.Scope, fixturePaymentMatrixKey{
		Operation: PaymentOrderSubscriptionPurchase,
		Plan:      plan,
		Channel:   request.PaymentChannel,
	})
	return result, err
}

func (c *FixturePaymentClient) fixturePlanCode(planID string) (string, error) {
	for _, plan := range c.authority.plans {
		if plan.PlanID == planID {
			return plan.Plan, nil
		}
		if plan.Plan == planID {
			return plan.Plan, nil
		}
	}
	return "", fmt.Errorf("cloud: canonical fixture has no plan for plan_id")
}

func (c *FixturePaymentClient) CreateUpgradeOrder(ctx context.Context, request CreateUpgradeOrderRequest) (PaymentOrderResult, error) {
	if err := ctx.Err(); err != nil {
		return PaymentOrderResult{}, err
	}
	if err := request.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	account, err := c.authority.currentAccount(ctx, request.Scope)
	if err != nil {
		return PaymentOrderResult{}, err
	}
	return c.create(request.Scope, fixturePaymentMatrixKey{
		Operation: PaymentOrderSubscriptionUpgrade,
		FromPlan:  account.Subscription.Plan,
		Plan:      request.TargetPlan,
		Channel:   request.PaymentChannel,
	})
}

func (c *FixturePaymentClient) create(scope BindingScope, key fixturePaymentMatrixKey) (PaymentOrderResult, error) {
	fixture, ok := c.authority.creates[key]
	if !ok {
		if key.Operation == PaymentOrderSubscriptionUpgrade {
			return PaymentOrderResult{}, &Error{Code: ErrorUpgradeNotAllowed, Message: "cloud: canonical fixture does not allow this upgrade"}
		}
		return PaymentOrderResult{}, fmt.Errorf("cloud: canonical fixture has no payment create for the request")
	}
	return projectFixturePaymentResult(fixture.Result, scope)
}

func (c *FixturePaymentClient) GetPaymentOrder(ctx context.Context, scope BindingScope, orderID string) (PaymentOrder, error) {
	return c.advance(ctx, scope, orderID)
}

func (c *FixturePaymentClient) VerifyPaymentOrder(ctx context.Context, scope BindingScope, orderID string) (PaymentOrder, error) {
	return c.advance(ctx, scope, orderID)
}

func (c *FixturePaymentClient) advance(ctx context.Context, scope BindingScope, orderID string) (PaymentOrder, error) {
	record, tape, err := c.loadPaymentTapeCursor(ctx, scope, orderID)
	if err != nil {
		return PaymentOrder{}, err
	}
	return advancePaymentTapeCursor(scope, record, tape)
}

func (c *FixturePaymentClient) loadPaymentTapeCursor(ctx context.Context, scope BindingScope, orderID string) (PaymentOrderRecord, PaymentTapeFixture, error) {
	if err := ctx.Err(); err != nil {
		return PaymentOrderRecord{}, PaymentTapeFixture{}, err
	}
	if err := scope.Validate(); err != nil {
		return PaymentOrderRecord{}, PaymentTapeFixture{}, err
	}
	if err := requireTrimmed("order_id", orderID); err != nil {
		return PaymentOrderRecord{}, PaymentTapeFixture{}, err
	}
	if _, err := c.authority.canonicalAccount(scope); err != nil {
		return PaymentOrderRecord{}, PaymentTapeFixture{}, err
	}
	record, err := c.authority.savedOrder(ctx, scope, orderID)
	if err != nil {
		return PaymentOrderRecord{}, PaymentTapeFixture{}, err
	}
	tape, ok := c.authority.tapes[orderID]
	if !ok || len(tape.Results) == 0 {
		return PaymentOrderRecord{}, PaymentTapeFixture{}, paymentScopeMismatch("cloud: canonical payment tape belongs to another order")
	}
	return record, tape, nil
}

func advancePaymentTapeCursor(scope BindingScope, record PaymentOrderRecord, tape PaymentTapeFixture) (PaymentOrder, error) {
	for i, canonical := range tape.Results {
		projected, err := projectFixturePaymentResult(canonical, scope)
		if err != nil {
			return PaymentOrder{}, err
		}
		if !samePaymentTapeCursor(record.Result.Order, projected.Order) {
			continue
		}
		if i+1 == len(tape.Results) {
			return projected.Order, nil
		}
		next, err := projectFixturePaymentResult(tape.Results[i+1], scope)
		if err != nil {
			return PaymentOrder{}, err
		}
		return next.Order, nil
	}
	return PaymentOrder{}, paymentScopeMismatch("cloud: saved payment order drifted from the canonical tape")
}

func samePaymentTapeCursor(saved, canonical PaymentOrder) bool {
	return samePaymentOrderIdentity(saved, canonical) &&
		saved.Status == canonical.Status &&
		saved.GrantStatus == canonical.GrantStatus
}

func projectFixturePaymentResult(canonical PaymentOrderResult, scope BindingScope) (PaymentOrderResult, error) {
	if err := scope.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	if err := canonical.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	projected := canonical
	projected.Order.Scope = scope
	return projected, projected.Validate()
}

var _ CloudPaymentClient = (*FixturePaymentClient)(nil)

// FixturePlanClient exposes only the canonical bundle's authoritative plan
// list for known cloud accounts, projected into a validated local scope.
type FixturePlanClient struct {
	plans           []SubscriptionPlan
	cloudAccountIDs map[string]struct{}
}

func NewFixturePlanClient(bundle FixtureBundle) (*FixturePlanClient, error) {
	if bundle.Version != A0FixtureVersion {
		return nil, fmt.Errorf("cloud: fixture plan client requires %s", A0FixtureVersion)
	}
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	client := &FixturePlanClient{
		plans:           append([]SubscriptionPlan(nil), bundle.PlanCases...),
		cloudAccountIDs: make(map[string]struct{}, len(bundle.AccountSnapshots)),
	}
	for _, snapshot := range bundle.AccountSnapshots {
		cloudUserID := snapshot.Scope.CloudUserID
		if _, exists := client.cloudAccountIDs[cloudUserID]; exists {
			return nil, fmt.Errorf("cloud: duplicate canonical plan account identity")
		}
		client.cloudAccountIDs[cloudUserID] = struct{}{}
	}
	return client, nil
}

func (c *FixturePlanClient) ListSubscriptionPlans(ctx context.Context, scope BindingScope) ([]SubscriptionPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if _, ok := c.cloudAccountIDs[scope.CloudUserID]; !ok {
		return nil, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: plan fixture belongs to another binding"}
	}
	return append([]SubscriptionPlan(nil), c.plans...), nil
}

var _ SubscriptionPlanClient = (*FixturePlanClient)(nil)

const (
	paymentV3OrdersPath           = "/api/v1/payment-v3/orders"
	subscriptionUpgradeOrdersPath = "/api/v1/subscription-upgrade-orders"
	paymentOrdersPath             = "/api/v1/payment-orders"
)

type paymentV3Order struct {
	OrderID       string     `json:"order_id"`
	PlanID        string     `json:"plan_id"`
	Plan          string     `json:"plan"`
	PaymentMethod string     `json:"payment_method"`
	Amount        string     `json:"amount"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"`
	QRCodeURL     string     `json:"qr_code_url"`
	ExpiresAt     time.Time  `json:"expires_at"`
	Paid          bool       `json:"paid"`
	Granted       bool       `json:"granted"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	GrantedAt     *time.Time `json:"granted_at,omitempty"`
	Version       uint64     `json:"version"`
}

type paymentOrderWire struct {
	OrderID        string    `json:"order_id"`
	OrderType      string    `json:"order_type"`
	Plan           string    `json:"plan"`
	FromPlan       *string   `json:"from_plan"`
	Amount         string    `json:"amount"`
	AmountUnit     string    `json:"amount_unit"`
	Currency       string    `json:"currency"`
	PaymentChannel string    `json:"payment_channel"`
	Status         string    `json:"status"`
	GrantStatus    string    `json:"grant_status"`
	ExpiresAt      time.Time `json:"expires_at"`
	Version        uint64    `json:"version"`
}

type paymentEntryWire struct {
	Channel   string    `json:"channel"`
	EntryType string    `json:"entry_type"`
	QRCode    string    `json:"qr_code,omitempty"`
	CodeURL   string    `json:"code_url,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

type paymentOrderResultWire struct {
	PaymentOrder paymentOrderWire  `json:"payment_order"`
	PaymentEntry *paymentEntryWire `json:"payment_entry,omitempty"`
}

type httpPaymentV3Client struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPPaymentV3Client(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (CloudPaymentClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: HTTP payment v3 client is not configured")
	}
	return &httpPaymentV3Client{executor: executor, transport: transport}, nil
}

func (c *httpPaymentV3Client) ListSubscriptionPlans(ctx context.Context, scope BindingScope) ([]SubscriptionPlan, error) {
	client, err := NewHTTPSubscriptionPlanClient(c.transport)
	if err != nil {
		return nil, err
	}
	return client.ListSubscriptionPlans(ctx, scope)
}

func (c *httpPaymentV3Client) CreateSubscriptionOrder(ctx context.Context, request CreateSubscriptionOrderRequest) (PaymentOrderResult, error) {
	if err := request.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	if strings.TrimSpace(request.PlanID) == "" {
		return PaymentOrderResult{}, protocolError(fmt.Errorf("payment v3 plan_id is required"))
	}
	var order paymentV3Order
	err := c.executor.Do(ctx, request.Scope, func(callCtx context.Context, token string) error {
		headers := make(http.Header)
		headers.Set("X-Idempotency-Key", request.IdempotencyKey)
		body := struct {
			PlanID        string `json:"plan_id"`
			PaymentMethod string `json:"payment_method"`
		}{request.PlanID, string(request.PaymentChannel)}
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, paymentV3OrdersPath, body, token, headers, &order)
	})
	if err != nil {
		return PaymentOrderResult{}, err
	}
	return normalizePaymentV3Order(request.Scope, order)
}

func (c *httpPaymentV3Client) CreateUpgradeOrder(ctx context.Context, request CreateUpgradeOrderRequest) (PaymentOrderResult, error) {
	if err := request.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	var result paymentOrderResultWire
	err := c.executor.Do(ctx, request.Scope, func(callCtx context.Context, token string) error {
		headers := make(http.Header)
		headers.Set("X-Idempotency-Key", request.IdempotencyKey)
		body := struct {
			TargetPlan     string `json:"target_plan"`
			PaymentChannel string `json:"payment_channel"`
		}{request.TargetPlan, string(request.PaymentChannel)}
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, subscriptionUpgradeOrdersPath, body, token, headers, &result)
	})
	if err != nil {
		return PaymentOrderResult{}, err
	}
	return normalizePaymentOrderResult(request.Scope, result)
}

func (c *httpPaymentV3Client) GetPaymentOrder(ctx context.Context, scope BindingScope, orderID string) (PaymentOrder, error) {
	result, err := c.getPaymentOrderResult(ctx, scope, orderID)
	return result.Order, err
}

func (c *httpPaymentV3Client) getPaymentOrderResult(ctx context.Context, scope BindingScope, orderID string) (PaymentOrderResult, error) {
	if err := scope.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	if err := requireTrimmed("order_id", orderID); err != nil {
		return PaymentOrderResult{}, err
	}
	var result paymentOrderResultWire
	path := paymentOrdersPath + "/" + url.PathEscape(orderID)
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, path, nil, token, nil, &result)
	})
	if err != nil {
		return PaymentOrderResult{}, err
	}
	return normalizePaymentOrderResult(scope, result)
}

func (c *httpPaymentV3Client) getPaymentOrderResultFor(ctx context.Context, scope BindingScope, order PaymentOrder) (PaymentOrderResult, error) {
	if err := order.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	if order.Scope != scope {
		return PaymentOrderResult{}, paymentScopeMismatch("cloud: payment order belongs to another binding")
	}
	if order.OrderType == PaymentOrderSubscriptionPurchase && order.PlanID != "" {
		var wire paymentV3Order
		path := paymentV3OrdersPath + "/" + url.PathEscape(order.OrderID)
		err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
			return c.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, path, nil, token, nil, &wire)
		})
		if err != nil {
			return PaymentOrderResult{}, err
		}
		return normalizePaymentV3Order(scope, wire)
	}
	var result paymentOrderResultWire
	path := paymentOrdersPath + "/" + url.PathEscape(order.OrderID) + "/verify"
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, path, nil, token, nil, &result)
	})
	if err != nil {
		return PaymentOrderResult{}, err
	}
	return normalizePaymentOrderResult(scope, result)
}

func (c *httpPaymentV3Client) VerifyPaymentOrder(ctx context.Context, scope BindingScope, orderID string) (PaymentOrder, error) {
	if err := scope.Validate(); err != nil {
		return PaymentOrder{}, err
	}
	if err := requireTrimmed("order_id", orderID); err != nil {
		return PaymentOrder{}, err
	}
	var result paymentOrderResultWire
	path := paymentOrdersPath + "/" + url.PathEscape(orderID) + "/verify"
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, path, nil, token, nil, &result)
	})
	if err != nil {
		return PaymentOrder{}, err
	}
	normalized, err := normalizePaymentOrderResult(scope, result)
	return normalized.Order, err
}

func normalizePaymentOrderResult(scope BindingScope, wire paymentOrderResultWire) (PaymentOrderResult, error) {
	status, err := normalizePaymentOrderStatus(wire.PaymentOrder.Status, wire.PaymentOrder.GrantStatus)
	if err != nil {
		return PaymentOrderResult{}, err
	}
	fromPlan := ""
	if wire.PaymentOrder.FromPlan != nil {
		fromPlan = *wire.PaymentOrder.FromPlan
	}
	orderType := PaymentOrderType(wire.PaymentOrder.OrderType)
	channel := PaymentChannel(wire.PaymentOrder.PaymentChannel)
	amount, err := normalizePaymentWireAmount(wire.PaymentOrder.Amount, AmountUnit(wire.PaymentOrder.AmountUnit))
	if err != nil {
		return PaymentOrderResult{}, err
	}
	grant := GrantNotGranted
	if status == PaymentGranted {
		grant = GrantGranted
	}
	result := PaymentOrderResult{Order: PaymentOrder{
		Scope: scope, OrderID: wire.PaymentOrder.OrderID, OrderType: orderType,
		Plan: wire.PaymentOrder.Plan, FromPlan: fromPlan,
		Amount: amount, AmountUnit: AmountUnit(wire.PaymentOrder.AmountUnit),
		Currency: wire.PaymentOrder.Currency, PaymentChannel: channel,
		Status: status, GrantStatus: grant, ExpiresAt: wire.PaymentOrder.ExpiresAt,
		Version: wire.PaymentOrder.Version,
	}}
	if wire.PaymentEntry != nil {
		result.Entry = PaymentEntry{
			Channel:   PaymentChannel(wire.PaymentEntry.Channel),
			EntryType: PaymentEntryType(wire.PaymentEntry.EntryType),
			QRCode:    wire.PaymentEntry.QRCode, CodeURL: wire.PaymentEntry.CodeURL,
			ExpiresAt: wire.PaymentEntry.ExpiresAt,
		}
	}
	if err := result.Validate(); err != nil {
		return PaymentOrderResult{}, protocolError(err)
	}
	return result, nil
}

func normalizePaymentWireAmount(amount string, unit AmountUnit) (string, error) {
	value, ok := new(big.Rat).SetString(strings.TrimSpace(amount))
	if !ok || value.Sign() < 0 {
		return "", protocolError(fmt.Errorf("invalid payment amount %q", amount))
	}
	scale := int64(1)
	digits := 0
	switch unit {
	case AmountUnitYuan:
		scale, digits = 100, 2
	case AmountUnitFen:
	case "":
		return strings.TrimSpace(amount), nil
	default:
		return "", protocolError(fmt.Errorf("unsupported payment amount_unit %q", unit))
	}
	scaled := new(big.Rat).Mul(value, big.NewRat(scale, 1))
	if !scaled.IsInt() {
		return "", protocolError(fmt.Errorf("payment amount %q has excess precision for %q", amount, unit))
	}
	return value.FloatString(digits), nil
}

func normalizePaymentOrderStatus(status, grantStatus string) (PaymentStatus, error) {
	wireStatus := strings.TrimSpace(status)
	wireGrantStatus := strings.TrimSpace(grantStatus)
	normalized := normalizePaymentOrderStatusName(wireStatus, wireGrantStatus)
	if normalized == "" {
		return "", protocolError(fmt.Errorf("unsupported payment status %q", status))
	}
	if !validPaymentGrantFacts(normalized, wireGrantStatus) {
		return "", protocolError(fmt.Errorf("payment status %q conflicts with grant_status %q", status, grantStatus))
	}
	return normalized, nil
}

var paymentOrderStatusByWire = map[string]PaymentStatus{
	"created":                       PaymentCreated,
	"precreating":                   PaymentCreated,
	"payment_pending":               PaymentPending,
	"callback_received":             PaymentCallbackReceived,
	"verifying":                     PaymentVerifying,
	"granting":                      PaymentGranting,
	"granted":                       PaymentGranted,
	"closed":                        PaymentClosed,
	"expired":                       PaymentTimeout,
	"payment_failed":                PaymentFailed,
	"verify_failed_manual_required": PaymentVerifyFailedManualRequired,
}

func normalizePaymentOrderStatusName(status, grantStatus string) PaymentStatus {
	if status == "paid_verified" {
		if grantStatus == "failed" {
			return PaymentVerifyFailedManualRequired
		}
		return PaymentPaidVerified
	}
	return paymentOrderStatusByWire[status]
}

func validPaymentGrantFacts(status PaymentStatus, grantStatus string) bool {
	switch status {
	case PaymentGranting:
		return grantStatus == "granting"
	case PaymentGranted:
		return grantStatus == "granted"
	case PaymentVerifyFailedManualRequired:
		return grantStatus == "failed" || grantStatus == "not_granted"
	default:
		return grantStatus == "not_granted"
	}
}

func normalizePaymentV3Order(scope BindingScope, wire paymentV3Order) (PaymentOrderResult, error) {
	status, err := normalizePaymentV3Status(wire)
	if err != nil {
		return PaymentOrderResult{}, err
	}
	wireStatus := strings.TrimSpace(wire.Status)
	channel := PaymentChannel(wire.PaymentMethod)
	amountUnit := AmountUnit("")
	if channel == PaymentChannelAlipay {
		amountUnit = AmountUnitYuan
	}
	amount, err := normalizePaymentWireAmount(wire.Amount, amountUnit)
	if err != nil {
		return PaymentOrderResult{}, err
	}
	grant := GrantNotGranted
	if wire.Granted {
		grant = GrantGranted
	}
	entry, err := normalizePaymentV3Entry(wire, channel, wireStatus)
	if err != nil {
		return PaymentOrderResult{}, err
	}
	result := PaymentOrderResult{
		Order: PaymentOrder{
			Scope: scope, OrderID: wire.OrderID, PlanID: wire.PlanID, Plan: wire.Plan,
			OrderType: PaymentOrderSubscriptionPurchase, PaymentChannel: channel,
			Amount: amount, AmountUnit: amountUnit, Currency: wire.Currency, Status: status,
			GrantStatus: grant, ExpiresAt: wire.ExpiresAt, Version: wire.Version,
		},
		Entry: entry,
	}
	if err := result.Validate(); err != nil {
		return PaymentOrderResult{}, protocolError(err)
	}
	return result, nil
}

func normalizePaymentV3Entry(wire paymentV3Order, channel PaymentChannel, wireStatus string) (PaymentEntry, error) {
	entryless := wireStatus == "creating" || wireStatus == "create_failed"
	qrCodeURL := strings.TrimSpace(wire.QRCodeURL)
	if entryless {
		if qrCodeURL != "" {
			return PaymentEntry{}, protocolError(fmt.Errorf("payment v3 status %q must not include qr_code_url", wire.Status))
		}
		return PaymentEntry{}, nil
	}
	if qrCodeURL == "" {
		return PaymentEntry{}, protocolError(fmt.Errorf("payment v3 qr_code_url is required for status %q", wire.Status))
	}
	return PaymentEntry{
		Channel: channel, EntryType: PaymentEntryQRCode,
		QRCodeURL: wire.QRCodeURL, ExpiresAt: wire.ExpiresAt,
	}, nil
}

func normalizePaymentV3Status(wire paymentV3Order) (PaymentStatus, error) {
	if wire.Version == 0 {
		return "", protocolError(fmt.Errorf("payment v3 version is required"))
	}
	wireStatus := strings.TrimSpace(wire.Status)
	status := normalizePaymentV3StatusName(wireStatus)
	if status == "" {
		return "", protocolError(fmt.Errorf("unsupported payment v3 status %q", wire.Status))
	}
	if !validPaymentV3StatusFacts(wireStatus, wire.Paid, wire.Granted) {
		return "", protocolError(fmt.Errorf(
			"payment v3 status %q conflicts with paid=%t granted=%t",
			wire.Status, wire.Paid, wire.Granted,
		))
	}
	return status, nil
}

func normalizePaymentV3StatusName(status string) PaymentStatus {
	switch status {
	case "creating":
		return PaymentCreated
	case "payment_pending", "polling":
		return PaymentPending
	case "paid_verified":
		return PaymentPaidVerified
	case "granting":
		return PaymentGranting
	case "granted":
		return PaymentGranted
	case "create_failed", "payment_failed":
		return PaymentFailed
	case "timeout":
		return PaymentTimeout
	case "manual_required":
		return PaymentVerifyFailedManualRequired
	default:
		return ""
	}
}

func validPaymentV3StatusFacts(status string, paid, granted bool) bool {
	switch status {
	case "paid_verified", "granting":
		return paid && !granted
	case "granted":
		return paid && granted
	case "manual_required":
		return !granted
	default:
		return !paid && !granted
	}
}
