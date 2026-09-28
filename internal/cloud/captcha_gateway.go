package cloud

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrCaptchaVerificationRequired = errors.New("cloud: captcha verification is required")
	ErrCaptchaPermitExpired        = errors.New("cloud: captcha permit expired")
)

const githubConnectorOAuthStartPath = "/api/v1/connectors/oauth/github/start"

// CaptchaGateway is the L1 boundary that issues a permit for sending a
// registration SMS. It deliberately carries no provider protocol while
// CLOUD-02 remains unresolved.
type CaptchaGateway interface {
	Verify(context.Context, bool) (CaptchaPermit, error)
}

// CaptchaPermit is issued only by a CaptchaGateway. Its ticket cannot be
// supplied by callers until verification has succeeded.
type CaptchaPermit struct {
	ticket    string
	expiresAt time.Time
}

// Ticket returns the mock captcha ticket while its permit remains valid.
func (p CaptchaPermit) Ticket() (string, error) {
	if p.ticket == "" {
		return "", ErrCaptchaVerificationRequired
	}
	if !time.Now().UTC().Before(p.expiresAt) {
		return "", ErrCaptchaPermitExpired
	}
	return p.ticket, nil
}

// MockCaptchaGateway is an L1-only deterministic gateway. The boolean input
// represents the local fake verification result; it is not a slider provider
// payload or a declaration of its eventual wire format.
type MockCaptchaGateway struct {
	ticket    string
	expiresAt time.Time
}

func NewMockCaptchaGateway(ticket string, expiresAt time.Time) (*MockCaptchaGateway, error) {
	if strings.TrimSpace(ticket) == "" || strings.TrimSpace(ticket) != ticket {
		return nil, fmt.Errorf("cloud: captcha ticket must be non-empty and trimmed")
	}
	if expiresAt.IsZero() {
		return nil, fmt.Errorf("cloud: captcha permit expiry is required")
	}
	return &MockCaptchaGateway{ticket: ticket, expiresAt: expiresAt.UTC()}, nil
}

func (g *MockCaptchaGateway) Verify(ctx context.Context, passed bool) (CaptchaPermit, error) {
	if err := ctx.Err(); err != nil {
		return CaptchaPermit{}, err
	}
	if !passed {
		return CaptchaPermit{}, ErrCaptchaVerificationRequired
	}
	if g == nil || !time.Now().UTC().Before(g.expiresAt) {
		return CaptchaPermit{}, ErrCaptchaPermitExpired
	}
	return CaptchaPermit{ticket: g.ticket, expiresAt: g.expiresAt}, nil
}

var _ CaptchaGateway = (*MockCaptchaGateway)(nil)

const developmentCaptchaPermitTTL = 5 * time.Minute

// DevelopmentCaptchaGateway creates one short-lived ticket per verified local
// development action. The ticket shape matches the cloud development verifier;
// no provider secret or fixed SMS code is present in the desktop client.
type DevelopmentCaptchaGateway struct {
	random io.Reader
	now    func() time.Time
}

func NewDevelopmentCaptchaGateway(random io.Reader, now func() time.Time) *DevelopmentCaptchaGateway {
	if random == nil {
		random = rand.Reader
	}
	if now == nil {
		now = time.Now
	}
	return &DevelopmentCaptchaGateway{random: random, now: now}
}

func (g *DevelopmentCaptchaGateway) Verify(ctx context.Context, passed bool) (CaptchaPermit, error) {
	if err := ctx.Err(); err != nil {
		return CaptchaPermit{}, err
	}
	if !passed {
		return CaptchaPermit{}, ErrCaptchaVerificationRequired
	}
	if g == nil || g.random == nil || g.now == nil {
		return CaptchaPermit{}, fmt.Errorf("cloud: development captcha gateway is not configured")
	}
	var value [16]byte
	if _, err := io.ReadFull(g.random, value[:]); err != nil {
		return CaptchaPermit{}, fmt.Errorf("cloud: generate development captcha ticket: %w", err)
	}
	now := g.now().UTC()
	return CaptchaPermit{ticket: "dev-captcha-" + hex.EncodeToString(value[:]), expiresAt: now.Add(developmentCaptchaPermitTTL)}, nil
}

var _ CaptchaGateway = (*DevelopmentCaptchaGateway)(nil)

const creditFreezesPath = "/api/v1/credit-freezes"
const releaseCreditFreezePath = creditFreezesPath + "/release"

// ReleaseFreezeRequest identifies a previously frozen task credit to release.
type ReleaseFreezeRequest struct {
	Scope          BindingScope `json:"scope"`
	TaskID         string       `json:"task_id"`
	IdempotencyKey string       `json:"idempotency_key"`
}

// Validate verifies the binding, task, and idempotency fields required by the release API.
func (r ReleaseFreezeRequest) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("task_id", r.TaskID); err != nil {
		return err
	}
	return requireTrimmed("idempotency_key", r.IdempotencyKey)
}

type httpCreditClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPCreditClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (CloudCreditClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: HTTP credit client is not configured")
	}
	return &httpCreditClient{executor: executor, transport: transport}, nil
}

func (c *httpCreditClient) CreateFreeze(ctx context.Context, request FreezeRequest) (FreezeResult, error) {
	if err := request.Validate(); err != nil {
		return FreezeResult{}, err
	}
	return c.execute(ctx, request.Scope, http.MethodPost, creditFreezesPath, creditFreezeRequestWire{TaskID: request.TaskID, ModelID: request.ModelID, ClientTaskType: request.ClientTaskType, ClientEstimatedCost: request.ClientEstimatedCost}, http.Header{"X-Idempotency-Key": []string{request.IdempotencyKey}})
}

func (c *httpCreditClient) GetFreeze(ctx context.Context, scope BindingScope, freezeID string) (FreezeResult, error) {
	if err := scope.Validate(); err != nil {
		return FreezeResult{}, err
	}
	if err := requireTrimmed("freeze_id", freezeID); err != nil {
		return FreezeResult{}, err
	}
	if freezeID == "." || freezeID == ".." || url.PathEscape(freezeID) != freezeID {
		return FreezeResult{}, fmt.Errorf("cloud: freeze_id must be a safe path segment")
	}
	return c.execute(ctx, scope, http.MethodGet, creditFreezesPath+"/"+freezeID, nil, nil)
}

func (c *httpCreditClient) ReleaseFreeze(ctx context.Context, request ReleaseFreezeRequest) (FreezeResult, error) {
	if err := request.Validate(); err != nil {
		return FreezeResult{}, err
	}
	return c.execute(ctx, request.Scope, http.MethodPost, releaseCreditFreezePath, releaseCreditFreezeRequestWire{TaskID: request.TaskID}, http.Header{"X-Idempotency-Key": []string{request.IdempotencyKey}})
}

func (c *httpCreditClient) execute(ctx context.Context, scope BindingScope, method, path string, body any, headers http.Header) (result FreezeResult, err error) {
	err = c.executor.Do(ctx, scope, func(ctx context.Context, token string) error {
		var wire creditFreezeResultWire
		envelope, requestErr := c.transport.doWithHeaders(ctx, method, path, body, token, headers, &wire)
		if requestErr != nil {
			return requestErr
		}
		candidate := wire.domain(scope)
		if validateErr := candidate.Validate(); validateErr != nil {
			return AttachRequestID(protocolError(validateErr), envelope.RequestID)
		}
		result = candidate
		return nil
	})
	return result, err
}

type creditFreezeRequestWire struct {
	TaskID              string `json:"task_id"`
	ModelID             string `json:"model_id"`
	ClientTaskType      string `json:"client_task_type"`
	ClientEstimatedCost string `json:"client_estimated_cost"`
}

type releaseCreditFreezeRequestWire struct {
	TaskID string `json:"task_id"`
}

type creditFreezeResultWire struct {
	Freeze  creditFreezeWire `json:"freeze"`
	Balance BalanceSnapshot  `json:"balance"`
}

func (r creditFreezeResultWire) domain(scope BindingScope) FreezeResult {
	return FreezeResult{Freeze: r.Freeze.domain(scope), Balance: r.Balance}
}

type creditFreezeWire struct {
	FreezeID                 string       `json:"freeze_id"`
	TaskID                   string       `json:"task_id"`
	Status                   FreezeStatus `json:"status"`
	FreezeAmount             string       `json:"freeze_amount"`
	FreeFrozenAmount         string       `json:"free_frozen_amount"`
	SubscriptionFrozenAmount string       `json:"subscription_frozen_amount"`
	LowBalanceMode           bool         `json:"low_balance_mode"`
	ExpiresAt                time.Time    `json:"expires_at"`
	Version                  uint64       `json:"version"`
}

func (f creditFreezeWire) domain(scope BindingScope) Freeze {
	return Freeze{Scope: scope, FreezeID: f.FreezeID, TaskID: f.TaskID, Status: f.Status, FreezeAmount: f.FreezeAmount, FreeFrozenAmount: f.FreeFrozenAmount, SubscriptionFrozenAmount: f.SubscriptionFrozenAmount, LowBalanceMode: f.LowBalanceMode, ExpiresAt: f.ExpiresAt}
}

var _ CloudCreditClient = (*httpCreditClient)(nil)

type ConnectorOAuthStart struct {
	FlowID           string    `json:"flow_id"`
	AuthorizationURL string    `json:"authorization_url"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type ConnectorOAuthCompletion struct {
	Status      string `json:"status"`
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

type ConnectorOAuthClient interface {
	StartGitHub(context.Context, BindingScope) (ConnectorOAuthStart, error)
	CompleteGitHub(context.Context, BindingScope, string) (ConnectorOAuthCompletion, error)
}

type HTTPConnectorOAuthClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPConnectorOAuthClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (*HTTPConnectorOAuthClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: connector OAuth client is not configured")
	}
	return &HTTPConnectorOAuthClient{executor: executor, transport: transport}, nil
}

func (c *HTTPConnectorOAuthClient) StartGitHub(ctx context.Context, scope BindingScope) (ConnectorOAuthStart, error) {
	var result ConnectorOAuthStart
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, accessToken string) error {
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, githubConnectorOAuthStartPath, nil, accessToken, nil, &result)
	})
	if err != nil {
		return ConnectorOAuthStart{}, err
	}
	authorization, parseErr := url.Parse(result.AuthorizationURL)
	if strings.TrimSpace(result.FlowID) == "" || parseErr != nil || authorization.Scheme != "https" || authorization.Host == "" || result.ExpiresAt.IsZero() {
		return ConnectorOAuthStart{}, protocolError(fmt.Errorf("connector OAuth start response is incomplete"))
	}
	return result, nil
}

func (c *HTTPConnectorOAuthClient) CompleteGitHub(ctx context.Context, scope BindingScope, flowID string) (ConnectorOAuthCompletion, error) {
	flowID = strings.TrimSpace(flowID)
	if flowID == "" {
		return ConnectorOAuthCompletion{}, fmt.Errorf("cloud: connector OAuth flow_id is required")
	}
	var result ConnectorOAuthCompletion
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, accessToken string) error {
		path := "/api/v1/connectors/oauth/github/flows/" + url.PathEscape(flowID) + "/complete"
		return c.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, path, nil, accessToken, nil, &result)
	})
	if err != nil {
		return ConnectorOAuthCompletion{}, err
	}
	switch result.Status {
	case "pending":
		if result.AccessToken != "" || result.TokenType != "" || result.Scope != "" {
			return ConnectorOAuthCompletion{}, protocolError(fmt.Errorf("pending connector OAuth response contains credentials"))
		}
	case "completed":
		if strings.TrimSpace(result.AccessToken) == "" || !strings.EqualFold(result.TokenType, "bearer") {
			return ConnectorOAuthCompletion{}, protocolError(fmt.Errorf("completed connector OAuth response is incomplete"))
		}
		result.TokenType = "bearer"
	default:
		return ConnectorOAuthCompletion{}, protocolError(fmt.Errorf("connector OAuth response has invalid status"))
	}
	return result, nil
}

var _ ConnectorOAuthClient = (*HTTPConnectorOAuthClient)(nil)
