package cloud

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var errFixtureAuth = errors.New("cloud: fixture authentication outcome")

// FixtureAccountScenarios selects only named A0 auth outcomes; it never maps
// a future cloud wire request to a fixture.
type FixtureAccountScenarios struct{ Register, Login, Refresh string }

// FixtureAccountClient is the deterministic CLOUD-01 L1 fake. It preserves no
// registration payload, credentials, plaintext passwords, or ciphertext.
type FixtureAccountClient struct {
	authority                   *FixtureCloudAuthority
	byName                      map[string]AuthFixture
	loginByPhone                map[string]AuthFixture
	bindingsByIdentity          map[fixtureAccountIdentity]ProfileBinding
	accountSnapshots            map[fixtureAccountIdentity]AccountSnapshotFixture
	accountsByCloudUser         map[string]fixtureCanonicalAccount
	registerScenario            string
	loginScenario               string
	refreshScenario             string
	mu                          sync.Mutex
	sendSMSCalls, registerCalls int
}

type fixtureAccountIdentity struct {
	CloudUserID       string
	BindingGeneration uint64
}

type fixtureCanonicalAccount struct {
	Binding  ProfileBinding
	Snapshot AccountSnapshotFixture
}

func fixtureIdentity(scope BindingScope) fixtureAccountIdentity {
	return fixtureAccountIdentity{CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration}
}

func NewFixtureAccountClient(bundle FixtureBundle, scenarios FixtureAccountScenarios) (*FixtureAccountClient, error) {
	authority, err := NewFixtureCloudAuthority(bundle, NewMemoryPaymentOrderRepository())
	if err != nil {
		return nil, err
	}
	return NewFixtureAccountClientWithAuthority(bundle, scenarios, authority)
}

func NewFixtureAccountClientWithAuthority(bundle FixtureBundle, scenarios FixtureAccountScenarios, authority *FixtureCloudAuthority) (*FixtureAccountClient, error) {
	if bundle.Version != A0FixtureVersion {
		return nil, fmt.Errorf("cloud: fixture account client requires %s", A0FixtureVersion)
	}
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	if authority == nil || authority.fixtureVersion != bundle.Version {
		return nil, fmt.Errorf("cloud: fixture account client requires the canonical authority")
	}
	scenarios = normalizeFixtureAccountScenarios(scenarios)
	client := newFixtureAccountClient(bundle, scenarios, authority)
	bindingsByCloudUser, err := client.indexAuthFixtures(bundle.AuthCases)
	if err != nil {
		return nil, err
	}
	if err := client.indexAccountSnapshots(bundle.AccountSnapshots, bindingsByCloudUser); err != nil {
		return nil, err
	}
	if err := validateFixtureAccountScenarios(client.byName, scenarios); err != nil {
		return nil, err
	}
	return client, nil
}

func normalizeFixtureAccountScenarios(scenarios FixtureAccountScenarios) FixtureAccountScenarios {
	if scenarios.Register == "" {
		scenarios.Register = "auth_success"
	}
	if scenarios.Login == "" {
		scenarios.Login = "auth_success"
	}
	if scenarios.Refresh == "" {
		scenarios.Refresh = "refresh_success"
	}
	return scenarios
}

func newFixtureAccountClient(bundle FixtureBundle, scenarios FixtureAccountScenarios, authority *FixtureCloudAuthority) *FixtureAccountClient {
	return &FixtureAccountClient{
		authority:           authority,
		byName:              make(map[string]AuthFixture, len(bundle.AuthCases)),
		loginByPhone:        make(map[string]AuthFixture),
		bindingsByIdentity:  make(map[fixtureAccountIdentity]ProfileBinding, len(bundle.AccountSnapshots)),
		accountSnapshots:    make(map[fixtureAccountIdentity]AccountSnapshotFixture, len(bundle.AccountSnapshots)),
		accountsByCloudUser: make(map[string]fixtureCanonicalAccount, len(bundle.AccountSnapshots)),
		registerScenario:    scenarios.Register,
		loginScenario:       scenarios.Login,
		refreshScenario:     scenarios.Refresh,
	}
}

func (c *FixtureAccountClient) indexAuthFixtures(fixtures []AuthFixture) (map[string]ProfileBinding, error) {
	bindingsByCloudUser := make(map[string]ProfileBinding, len(fixtures))
	for _, fixture := range fixtures {
		c.byName[fixture.Name] = fixture
		if fixture.LoginCredential != nil {
			c.loginByPhone[fixture.LoginCredential.Phone] = fixture
			scope, err := fixture.Binding.Scope()
			if err != nil {
				return nil, err
			}
			identity := fixtureIdentity(scope)
			if _, exists := c.bindingsByIdentity[identity]; exists {
				return nil, fmt.Errorf("cloud: duplicate projected account fixture identity")
			}
			if _, exists := bindingsByCloudUser[scope.CloudUserID]; exists {
				return nil, fmt.Errorf("cloud: duplicate canonical cloud account identity")
			}
			c.bindingsByIdentity[identity] = fixture.Binding
			bindingsByCloudUser[scope.CloudUserID] = fixture.Binding
		}
	}
	return bindingsByCloudUser, nil
}

func (c *FixtureAccountClient) indexAccountSnapshots(snapshots []AccountSnapshotFixture, bindingsByCloudUser map[string]ProfileBinding) error {
	for _, snapshot := range snapshots {
		identity := fixtureIdentity(snapshot.Scope)
		if _, exists := c.accountSnapshots[identity]; exists {
			return fmt.Errorf("cloud: duplicate projected account snapshot identity")
		}
		binding, ok := bindingsByCloudUser[snapshot.Scope.CloudUserID]
		if !ok {
			return fmt.Errorf("cloud: account snapshot has no canonical cloud account")
		}
		if _, exists := c.accountsByCloudUser[snapshot.Scope.CloudUserID]; exists {
			return fmt.Errorf("cloud: duplicate canonical cloud account snapshot")
		}
		c.accountSnapshots[identity] = snapshot
		c.accountsByCloudUser[snapshot.Scope.CloudUserID] = fixtureCanonicalAccount{Binding: binding, Snapshot: snapshot}
	}
	return nil
}

func validateFixtureAccountScenarios(byName map[string]AuthFixture, scenarios FixtureAccountScenarios) error {
	for _, scenario := range []string{scenarios.Register, scenarios.Login, scenarios.Refresh} {
		if _, ok := byName[scenario]; !ok {
			return fmt.Errorf("cloud: unknown A0 auth fixture %q", scenario)
		}
	}
	return nil
}

func (c *FixtureAccountClient) SendRegisterSMS(ctx context.Context, request AccountFixtureRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if request.Operation != AccountOperationSendRegisterSMS {
		return fmt.Errorf("cloud: fixture request operation is not send_register_sms")
	}
	c.mu.Lock()
	c.sendSMSCalls++
	c.mu.Unlock()
	return nil
}
func (c *FixtureAccountClient) Register(ctx context.Context, request AccountFixtureRequest) (AccountState, Session, error) {
	c.mu.Lock()
	c.registerCalls++
	c.mu.Unlock()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(request.Payload, &payload); err != nil {
		return AccountState{}, Session{}, err
	}
	if _, hasPlaintext := payload["password"]; hasPlaintext || len(payload["encrypted_password"]) == 0 || len(payload["password_crypto"]) == 0 {
		return AccountState{}, Session{}, fmt.Errorf("cloud: registration fixture payload requires encrypted_password only")
	}
	return c.accountOutcome(ctx, request, AccountOperationRegister, c.registerScenario)
}
func (c *FixtureAccountClient) Login(ctx context.Context, request AccountFixtureRequest) (AccountState, Session, error) {
	scenario, err := c.authenticateLogin(request)
	if err != nil {
		return AccountState{}, Session{}, err
	}
	return c.accountOutcome(ctx, request, AccountOperationLogin, scenario)
}

func (c *FixtureAccountClient) authenticateLogin(request AccountFixtureRequest) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	if request.Operation != AccountOperationLogin {
		return "", fmt.Errorf("cloud: fixture request operation is not login")
	}
	var payload struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(request.Payload, &payload); err != nil {
		return "", fmt.Errorf("cloud: decode fixture login payload: %w", err)
	}
	matched, ok := c.loginByPhone[payload.Phone]
	if !ok || matched.LoginCredential == nil {
		return "", &Error{Code: ErrorAuthRequired, Message: "cloud: invalid phone or password"}
	}
	expected, err := hex.DecodeString(matched.LoginCredential.PasswordDigest)
	if err != nil {
		return "", fmt.Errorf("cloud: login fixture has an invalid credential digest")
	}
	actual := sha256.Sum256([]byte(payload.Password))
	if subtle.ConstantTimeCompare(actual[:], expected) != 1 {
		return "", &Error{Code: ErrorAuthRequired, Message: "cloud: invalid phone or password"}
	}
	configured := c.byName[c.loginScenario]
	if configured.Error != nil {
		return configured.Name, nil
	}
	return matched.Name, nil
}
func (c *FixtureAccountClient) RefreshSession(ctx context.Context, session Session) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if err := session.Validate(); err != nil {
		return Session{}, err
	}
	if _, ok := c.accountsByCloudUser[session.Scope.CloudUserID]; !ok {
		return Session{}, fixtureMismatch("refresh session belongs to another fixture binding")
	}
	fixture := c.byName[c.refreshScenario]
	if fixture.Error != nil {
		return Session{}, fixtureError(*fixture.Error)
	}
	if fixture.Session == nil {
		return Session{}, fmt.Errorf("cloud: refresh fixture %q has no session", fixture.Name)
	}
	refreshed := *fixture.Session
	refreshed.Scope = session.Scope
	return refreshed, refreshed.Validate()
}
func (c *FixtureAccountClient) Logout(ctx context.Context, scope BindingScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return scope.Validate()
}
func (c *FixtureAccountClient) GetCurrentUser(ctx context.Context, scope BindingScope) (AccountState, error) {
	if err := ctx.Err(); err != nil {
		return AccountState{}, err
	}
	if err := scope.Validate(); err != nil {
		return AccountState{}, err
	}
	account, ok := c.accountsByCloudUser[scope.CloudUserID]
	if !ok {
		return AccountState{}, fixtureMismatch("account fixture belongs to another binding")
	}
	snapshot, err := c.authority.currentAccount(ctx, scope)
	if err != nil {
		return AccountState{}, err
	}
	binding := account.Binding
	binding.ProfileID = scope.ProfileID
	binding.BindingGeneration = scope.BindingGeneration
	state := AccountState{Binding: binding, State: AccountAuthenticated, User: CloudUser{CloudUserID: scope.CloudUserID}, Balance: snapshot.Balance, Subscription: snapshot.Subscription, LastSyncAt: snapshot.Subscription.UpdatedAt}
	return state, state.Validate()
}
func (c *FixtureAccountClient) SendRegisterSMSCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendSMSCalls
}
func (c *FixtureAccountClient) RegisterCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.registerCalls
}

func (c *FixtureAccountClient) accountOutcome(ctx context.Context, request AccountFixtureRequest, operation AccountOperation, scenario string) (AccountState, Session, error) {
	if err := ctx.Err(); err != nil {
		return AccountState{}, Session{}, err
	}
	if err := request.Validate(); err != nil {
		return AccountState{}, Session{}, err
	}
	if request.Operation != operation {
		return AccountState{}, Session{}, fmt.Errorf("cloud: fixture request operation mismatch")
	}
	fixture := c.byName[scenario]
	if fixture.Error != nil {
		return AccountState{}, Session{}, fixtureError(*fixture.Error)
	}
	canonicalScope, err := fixture.Binding.Scope()
	if err != nil {
		return AccountState{}, Session{}, err
	}
	projectedBinding := fixture.Binding
	projectedBinding.ActorID = request.Binding.ActorID
	projectedBinding.ProfileID = request.Binding.ProfileID
	projectedScope := canonicalScope
	projectedScope.ProfileID = request.Binding.ProfileID
	state, err := c.stateFor(projectedBinding, scenario)
	if err != nil {
		return AccountState{}, Session{}, err
	}
	if fixture.Session == nil {
		return AccountState{}, Session{}, fmt.Errorf("cloud: account fixture %q has no session", scenario)
	}
	session := *fixture.Session
	if fixtureIdentity(session.Scope) != fixtureIdentity(canonicalScope) {
		return AccountState{}, Session{}, fixtureMismatch("account session belongs to another fixture binding")
	}
	session.Scope = projectedScope
	if err := session.Validate(); err != nil {
		return AccountState{}, Session{}, err
	}
	return state, session, nil
}
func (c *FixtureAccountClient) stateFor(binding ProfileBinding, scenario string) (AccountState, error) {
	fixture := c.byName[scenario]
	fixtureScope, err := fixture.Binding.Scope()
	if err != nil {
		return AccountState{}, err
	}
	scope, err := binding.Scope()
	if err != nil {
		return AccountState{}, err
	}
	if fixture.Binding.Status != BindingBound || fixtureIdentity(fixtureScope) != fixtureIdentity(scope) {
		return AccountState{}, fixtureMismatch("account fixture belongs to another binding")
	}
	snapshot, ok := c.accountSnapshots[fixtureIdentity(scope)]
	if !ok {
		return AccountState{}, fixtureMismatch("account snapshot belongs to another binding")
	}
	subscription := snapshot.Subscription
	subscription.Scope = scope
	state := AccountState{Binding: binding, State: AccountAuthenticated, User: CloudUser{CloudUserID: scope.CloudUserID}, Balance: snapshot.Balance, Subscription: subscription, LastSyncAt: subscription.UpdatedAt}
	return state, state.Validate()
}
func fixtureError(value Error) error {
	value.Details = append([]byte(nil), value.Details...)
	return fmt.Errorf("%w: %w", errFixtureAuth, &value)
}
func fixtureMismatch(message string) error {
	return fmt.Errorf("%w: %w", errFixtureAuth, &Error{Code: ErrorProfileScopeMismatch, Message: message})
}

var _ CloudAccountClient = (*FixtureAccountClient)(nil)

const (
	cloudAuthResponseLimit  = 1 << 20
	cloudAuthRequestTimeout = 20 * time.Second
)

type HTTPSessionClient struct {
	baseURL   *url.URL
	http      *http.Client
	device    DeviceIdentity
	requestID func() string
}

func NewHTTPSessionClient(baseURL string, client *http.Client, device DeviceIdentity, requestID func() string) (*HTTPSessionClient, error) {
	parsed, err := validateCloudBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("cloud: HTTP client is required")
	}
	if err := device.Validate(); err != nil {
		return nil, err
	}
	if requestID == nil {
		requestID = newCloudRequestID
	}
	return &HTTPSessionClient{baseURL: parsed, http: client, device: device, requestID: requestID}, nil
}

func (c *HTTPSessionClient) Login(ctx context.Context, request LoginRequest) (AuthenticationResult, error) {
	if err := request.Validate(); err != nil {
		return AuthenticationResult{}, err
	}
	var response authLoginData
	envelope, err := c.do(ctx, http.MethodPost, "/api/v1/auth/login", struct {
		Phone           string         `json:"phone"`
		Password        string         `json:"password"`
		CaptchaTicket   string         `json:"captcha_ticket"`
		CaptchaProvider string         `json:"captcha_provider"`
		Device          DeviceIdentity `json:"device"`
	}{
		Phone: request.Phone, Password: request.Password,
		CaptchaTicket: request.CaptchaTicket, CaptchaProvider: request.CaptchaProvider,
		Device: c.device,
	}, "", &response)
	if err != nil {
		return AuthenticationResult{}, err
	}
	result := AuthenticationResult{User: response.User.domain(), Session: response.Session.domain(c.device.DeviceID), ServerTime: envelope.ServerTime}
	if err := result.User.Validate(); err != nil {
		return AuthenticationResult{}, AttachRequestID(protocolError(err), envelope.RequestID)
	}
	if err := result.Session.Validate(); err != nil {
		return AuthenticationResult{}, AttachRequestID(protocolError(err), envelope.RequestID)
	}
	if result.ServerTime.IsZero() {
		return AuthenticationResult{}, AttachRequestID(protocolError(fmt.Errorf("server_time is required")), envelope.RequestID)
	}
	return result, nil
}

func (c *HTTPSessionClient) Refresh(ctx context.Context, refreshToken string) (RemoteSession, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return RemoteSession{}, fmt.Errorf("cloud: refresh_token is required")
	}
	var response struct {
		Session authSessionWire `json:"session"`
	}
	envelope, err := c.do(ctx, http.MethodPost, "/api/v1/auth/refresh", struct {
		RefreshToken string `json:"refresh_token"`
		DeviceID     string `json:"device_id"`
	}{RefreshToken: refreshToken, DeviceID: c.device.DeviceID}, "", &response)
	if err != nil {
		return RemoteSession{}, err
	}
	session := response.Session.domain(c.device.DeviceID)
	if err := session.Validate(); err != nil {
		return RemoteSession{}, AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return session, nil
}

func (c *HTTPSessionClient) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return fmt.Errorf("cloud: refresh_token is required")
	}
	var response struct {
		Revoked bool `json:"revoked"`
	}
	_, err := c.do(ctx, http.MethodPost, "/api/v1/auth/logout", struct {
		RefreshToken string `json:"refresh_token"`
		DeviceID     string `json:"device_id"`
		Scope        string `json:"scope"`
	}{RefreshToken: refreshToken, DeviceID: c.device.DeviceID, Scope: "current_device"}, "", &response)
	return err
}

func (c *HTTPSessionClient) CurrentUser(ctx context.Context, accessToken string) (CurrentUserSnapshot, error) {
	if strings.TrimSpace(accessToken) == "" {
		return CurrentUserSnapshot{}, &Error{Code: ErrorAuthRequired, Message: "authentication is required"}
	}
	var response currentUserData
	envelope, err := c.do(ctx, http.MethodGet, "/api/v1/me", nil, accessToken, &response)
	if err != nil {
		return CurrentUserSnapshot{}, err
	}
	snapshot := CurrentUserSnapshot{
		User: response.User.domain(), Balance: response.Balance,
		Subscription: CurrentSubscription{Plan: response.Subscription.Plan, Status: response.Subscription.Status, Version: response.Subscription.Version},
		Capabilities: CurrentUserCapabilities{
			ProjectsV1: append(json.RawMessage(nil), response.Capabilities.ProjectsV1...),
			ProjectsV2: append(json.RawMessage(nil), response.Capabilities.ProjectsV2...),
		},
		ServerTime: envelope.ServerTime,
	}
	if response.Subscription.PeriodEnd != nil {
		snapshot.Subscription.PeriodEnd = *response.Subscription.PeriodEnd
	}
	if err := snapshot.User.Validate(); err != nil {
		return CurrentUserSnapshot{}, AttachRequestID(protocolError(err), envelope.RequestID)
	}
	if err := snapshot.Balance.Validate(); err != nil {
		return CurrentUserSnapshot{}, AttachRequestID(protocolError(err), envelope.RequestID)
	}
	if snapshot.Subscription.Plan == "" || snapshot.ServerTime.IsZero() {
		return CurrentUserSnapshot{}, AttachRequestID(protocolError(fmt.Errorf("current user response is incomplete")), envelope.RequestID)
	}
	if err := validateCurrentUserCapabilities(snapshot.Capabilities); err != nil {
		return CurrentUserSnapshot{}, AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return snapshot, nil
}

type cloudResponseEnvelope struct {
	Error      *Error    `json:"error"`
	RequestID  string    `json:"request_id"`
	ServerTime time.Time `json:"server_time"`
	StatusCode int       `json:"-"`
}

func (c *HTTPSessionClient) do(ctx context.Context, method, path string, body any, accessToken string, destination any) (cloudResponseEnvelope, error) {
	return c.doWithHeaders(ctx, method, path, body, accessToken, nil, destination)
}

// DoAuthenticatedJSON executes one protected JSON request with shared headers.
// The cloud envelope remains an internal transport detail.
func (c *HTTPSessionClient) DoAuthenticatedJSON(ctx context.Context, method, path string, body any, accessToken string, headers http.Header, destination any) error {
	_, err := c.doWithHeaders(ctx, method, path, body, accessToken, headers, destination)
	return err
}

// DoAuthenticatedJSONQuery executes one protected JSON request with encoded
// query parameters. Keeping query values separate from path prevents opaque
// cursor values from being interpreted as URL syntax.
func (c *HTTPSessionClient) DoAuthenticatedJSONQuery(ctx context.Context, method, path string, query url.Values, body any, accessToken string, headers http.Header, destination any) error {
	_, err := c.doWithHeadersAndQuery(ctx, method, path, query, body, accessToken, headers, destination)
	return err
}

// DoAuthenticatedMultipartJSON executes one protected multipart request and
// decodes the standard cloud JSON envelope.
func (c *HTTPSessionClient) DoAuthenticatedMultipartJSON(ctx context.Context, method, path string, payload []byte, contentType, accessToken string, headers http.Header, destination any) error {
	_, err := c.doPayloadWithHeadersAndQuery(ctx, method, path, nil, bytes.NewReader(payload), contentType, accessToken, headers, destination)
	return err
}

// DoAuthenticatedMultipartStreamJSON consumes a fresh multipart stream. The
// factory is invoked once per authenticated attempt so retries can reopen the
// source without retaining the full file in memory.
func (c *HTTPSessionClient) DoAuthenticatedMultipartStreamJSON(ctx context.Context, method, path string, stream func(context.Context) (io.ReadCloser, string, error), accessToken string, headers http.Header, destination any) error {
	body, contentType, err := stream(ctx)
	if err != nil {
		return err
	}
	defer body.Close()
	_, err = c.doPayloadWithHeadersAndQuery(ctx, method, path, nil, body, contentType, accessToken, headers, destination)
	return err
}

func (c *HTTPSessionClient) doWithHeaders(ctx context.Context, method, path string, body any, accessToken string, headers http.Header, destination any) (cloudResponseEnvelope, error) {
	return c.doWithHeadersAndQuery(ctx, method, path, nil, body, accessToken, headers, destination)
}

func (c *HTTPSessionClient) doWithHeadersAndQuery(ctx context.Context, method, path string, query url.Values, body any, accessToken string, headers http.Header, destination any) (cloudResponseEnvelope, error) {
	payload, err := cloudRequestPayload(body)
	if err != nil {
		return cloudResponseEnvelope{}, err
	}
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	return c.doPayloadWithHeadersAndQuery(ctx, method, path, query, payload, contentType, accessToken, headers, destination)
}

func (c *HTTPSessionClient) doPayloadWithHeadersAndQuery(ctx context.Context, method, path string, query url.Values, requestPayload io.Reader, contentType, accessToken string, headers http.Header, destination any) (cloudResponseEnvelope, error) {
	ctx, cancel := cloudRequestContext(ctx)
	defer cancel()
	request, err := c.newRequest(ctx, method, path, requestPayload, contentType, accessToken)
	if err != nil {
		return cloudResponseEnvelope{}, err
	}
	if len(query) > 0 {
		request.URL.RawQuery = query.Encode()
	}
	for name, values := range headers {
		request.Header.Del(name)
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	requestID := request.Header.Get("X-Request-Id")
	response, err := c.http.Do(request)
	if err != nil {
		return cloudResponseEnvelope{RequestID: requestID}, AttachRequestID(cloudNetworkError("cloud network is unavailable"), requestID)
	}
	defer response.Body.Close()
	responsePayload, err := readCloudResponse(response.Body)
	if err != nil {
		return cloudResponseEnvelope{RequestID: requestID}, AttachRequestID(err, requestID)
	}
	envelope, err := decodeCloudResponse(responsePayload, response.StatusCode, destination)
	envelope.StatusCode = response.StatusCode
	var typed *Error
	if errors.As(err, &typed) && typed.HTTPStatus == 0 {
		clone := *typed
		clone.HTTPStatus = response.StatusCode
		err = &clone
	}
	if envelope.RequestID == "" {
		envelope.RequestID = requestID
	}
	return envelope, AttachRequestID(err, envelope.RequestID)
}

func cloudRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, cloudAuthRequestTimeout)
}

func (c *HTTPSessionClient) newRequest(ctx context.Context, method, path string, payload io.Reader, contentType, accessToken string) (*http.Request, error) {
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: path})
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), payload)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Request-Id", c.requestID())
	request.Header.Set("X-Device-Id", c.device.DeviceID)
	request.Header.Set("X-Client-Version", c.device.ClientVersion)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	return request, nil
}

func cloudRequestPayload(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(encoded), nil
}

func readCloudResponse(body io.Reader) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(body, cloudAuthResponseLimit+1))
	if err != nil {
		return nil, cloudNetworkError("cloud response could not be read")
	}
	if len(payload) > cloudAuthResponseLimit {
		return nil, protocolError(fmt.Errorf("response exceeds safety limit"))
	}
	return payload, nil
}

type cloudRawResponse struct {
	Data       json.RawMessage `json:"data"`
	Error      *Error          `json:"error"`
	RequestID  string          `json:"request_id"`
	ServerTime time.Time       `json:"server_time"`
}

func decodeCloudResponse(payload []byte, statusCode int, destination any) (cloudResponseEnvelope, error) {
	var raw cloudRawResponse
	if err := json.Unmarshal(payload, &raw); err != nil {
		return cloudResponseEnvelope{}, protocolError(err)
	}
	envelope := cloudResponseEnvelope{Error: raw.Error, RequestID: raw.RequestID, ServerTime: raw.ServerTime}
	if raw.Error != nil {
		cloudErr := *raw.Error
		cloudErr.HTTPStatus = statusCode
		envelope.Error = &cloudErr
		return envelope, AttachRequestID(&cloudErr, raw.RequestID)
	}
	if statusCode != http.StatusOK {
		return envelope, protocolError(fmt.Errorf("unexpected HTTP status %d", statusCode))
	}
	if len(raw.Data) == 0 || string(raw.Data) == "null" {
		return envelope, protocolError(fmt.Errorf("response data is required"))
	}
	if err := json.Unmarshal(raw.Data, destination); err != nil {
		return envelope, protocolError(err)
	}
	return envelope, nil
}

func cloudNetworkError(message string) error {
	return &Error{Code: ErrorNetworkUnavailable, Message: message, Retryable: true}
}

type authUserWire struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

func (u authUserWire) domain() CloudUser {
	return CloudUser{CloudUserID: u.UserID, DisplayName: u.DisplayName, Role: u.Role}
}

type authSessionWire struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func (s authSessionWire) domain(deviceID string) RemoteSession {
	return RemoteSession{AccessToken: s.AccessToken, AccessTokenExpiresAt: s.AccessTokenExpiresAt, RefreshToken: s.RefreshToken, RefreshTokenExpiresAt: s.RefreshTokenExpiresAt, DeviceID: deviceID}
}

type authLoginData struct {
	User    authUserWire    `json:"user"`
	Session authSessionWire `json:"session"`
}

type currentUserData struct {
	User         authUserWire    `json:"user"`
	Balance      BalanceSnapshot `json:"balance"`
	Subscription struct {
		Plan      string             `json:"plan"`
		Status    SubscriptionStatus `json:"status"`
		PeriodEnd *time.Time         `json:"period_end"`
		Version   uint64             `json:"version"`
	} `json:"subscription"`
	Capabilities CurrentUserCapabilities `json:"capabilities"`
}

func validateCloudBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("cloud: base URL must be absolute")
	}
	if err := validateCloudURLShape(parsed); err != nil {
		return nil, err
	}
	if err := validateCloudURLScheme(parsed); err != nil {
		return nil, err
	}
	parsed.Path, parsed.RawPath = "", ""
	return parsed, nil
}

func validateCloudURLShape(parsed *url.URL) error {
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("cloud: base URL must be absolute")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("cloud: base URL must not contain credentials, path, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("cloud: base URL must not contain credentials, path, query, or fragment")
	}
	return nil
}

func validateCloudURLScheme(parsed *url.URL) error {
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		if !privateCloudHost(parsed.Hostname()) {
			return fmt.Errorf("cloud: HTTPS is required for non-private cloud endpoints")
		}
		return nil
	default:
		return fmt.Errorf("cloud: base URL must use HTTP or HTTPS")
	}
}

func privateCloudHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func protocolError(cause error) error {
	return &Error{Code: ErrorCode("CLOUD_PROTOCOL_ERROR"), Message: "cloud response is invalid: " + cause.Error()}
}

func newCloudRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return "req-" + hex.EncodeToString(value[:])
}

var _ CloudSessionClient = (*HTTPSessionClient)(nil)
