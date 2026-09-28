package mcpoauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"kwu/internal/brand"
)

const (
	callbackPath     = "/oauth/connector/callback"
	maxResponseBytes = 1 << 20
)

var resourceMetadataPattern = regexp.MustCompile(`(?i)resource_metadata="([^"]+)"`)

// ErrInvalidGrant reports that the authorization server rejected the refresh
// token. Callers use this sentinel to require a new interactive authorization.
var ErrInvalidGrant = errors.New("MCP OAuth refresh token is invalid")

type Config struct {
	ResourceURL string
	Resource    string
	Scopes      []string
	ClientID    string
}

type Metadata struct {
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	RegistrationEndpoint  string
	GrantTypesSupported   []string
}

type Registration struct {
	ClientID string
}

type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Scope        string
	ExpiresAt    time.Time
}

type Client struct{ HTTP *http.Client }

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Client{HTTP: httpClient}
}

func (c *Client) Discover(ctx context.Context, config Config) (Metadata, error) {
	if c == nil || c.HTTP == nil || !secureHTTPS(config.ResourceURL) {
		return Metadata{}, errors.New("MCP OAuth configuration is invalid")
	}
	resource := config.Resource
	if resource == "" {
		resource = config.ResourceURL
	}
	metadataURL, challengeErr := c.challengeMetadataURL(ctx, config.ResourceURL)
	candidates := make([]string, 0, 1+len(protectedMetadataCandidates(config.ResourceURL)))
	if challengeErr == nil {
		candidates = append(candidates, metadataURL)
	}
	for _, candidate := range protectedMetadataCandidates(config.ResourceURL) {
		if !contains(candidates, candidate) {
			candidates = append(candidates, candidate)
		}
	}
	var issuer string
	var lastErr error
	for _, candidate := range candidates {
		protectedRaw, getErr := c.getJSON(ctx, candidate)
		if getErr != nil {
			lastErr = getErr
			continue
		}
		parsedIssuer, parseErr := parseProtectedMetadata(protectedRaw, resource, config.Scopes)
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		issuer = parsedIssuer
		break
	}
	if issuer == "" {
		if lastErr != nil {
			return Metadata{}, lastErr
		}
		return Metadata{}, challengeErr
	}
	lastErr = nil
	for _, endpoint := range authorizationMetadataCandidates(issuer) {
		raw, getErr := c.getJSON(ctx, endpoint)
		if getErr != nil {
			lastErr = getErr
			continue
		}
		metadata, parseErr := parseAuthorizationMetadata(raw, issuer, config.Scopes, config.ClientID == "")
		if parseErr == nil {
			return metadata, nil
		}
		lastErr = parseErr
	}
	if lastErr == nil {
		lastErr = errors.New("authorization-server metadata is unavailable")
	}
	return Metadata{}, lastErr
}

func (c *Client) challengeMetadataURL(ctx context.Context, resourceURL string) (string, error) {
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"Reasonix","version":"dev"}}}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resourceURL, strings.NewReader(initialize))
	if err != nil {
		return "", errors.New("create MCP OAuth discovery request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", errors.New("request MCP OAuth discovery")
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		return "", fmt.Errorf("MCP OAuth discovery returned HTTP %d", resp.StatusCode)
	}
	match := resourceMetadataPattern.FindStringSubmatch(resp.Header.Get("WWW-Authenticate"))
	if len(match) != 2 || !secureHTTPS(match[1]) {
		return "", errors.New("MCP server did not advertise valid protected-resource metadata")
	}
	return match[1], nil
}

func parseProtectedMetadata(raw []byte, resource string, scopes []string) (string, error) {
	var metadata struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
		BearerMethods        []string `json:"bearer_methods_supported"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil || metadata.Resource != resource || len(metadata.AuthorizationServers) == 0 {
		return "", errors.New("protected-resource metadata does not match the connector")
	}
	if len(metadata.ScopesSupported) > 0 && !containsAll(metadata.ScopesSupported, scopes) {
		return "", errors.New("protected-resource metadata does not support required scopes")
	}
	if len(metadata.BearerMethods) > 0 && !contains(metadata.BearerMethods, "header") {
		return "", errors.New("protected-resource metadata does not support bearer headers")
	}
	for _, issuer := range metadata.AuthorizationServers {
		if secureHTTPS(issuer) {
			return issuer, nil
		}
	}
	return "", errors.New("protected-resource metadata has no secure authorization server")
}

func parseAuthorizationMetadata(raw []byte, issuer string, scopes []string, registrationRequired bool) (Metadata, error) {
	var server struct {
		Issuer                    string   `json:"issuer"`
		AuthorizationEndpoint     string   `json:"authorization_endpoint"`
		TokenEndpoint             string   `json:"token_endpoint"`
		RegistrationEndpoint      string   `json:"registration_endpoint"`
		ScopesSupported           []string `json:"scopes_supported"`
		ResponseTypesSupported    []string `json:"response_types_supported"`
		GrantTypesSupported       []string `json:"grant_types_supported"`
		AuthMethodsSupported      []string `json:"token_endpoint_auth_methods_supported"`
		ChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	}
	if err := json.Unmarshal(raw, &server); err != nil || !compatibleAuthorizationIssuer(issuer, server.Issuer) || !secureHTTPS(server.AuthorizationEndpoint) || !secureHTTPS(server.TokenEndpoint) || registrationRequired && !secureHTTPS(server.RegistrationEndpoint) {
		return Metadata{}, errors.New("authorization-server metadata is invalid")
	}
	if len(server.ScopesSupported) > 0 && !containsAll(server.ScopesSupported, scopes) || !contains(server.ResponseTypesSupported, "code") || !contains(server.GrantTypesSupported, "authorization_code") || len(server.AuthMethodsSupported) > 0 && !contains(server.AuthMethodsSupported, "none") || !contains(server.ChallengeMethodsSupported, "S256") {
		return Metadata{}, errors.New("authorization-server metadata does not support the connector flow")
	}
	return Metadata{
		Issuer: server.Issuer, AuthorizationEndpoint: server.AuthorizationEndpoint,
		TokenEndpoint: server.TokenEndpoint, RegistrationEndpoint: server.RegistrationEndpoint,
		GrantTypesSupported: append([]string(nil), server.GrantTypesSupported...),
	}, nil
}

func compatibleAuthorizationIssuer(advertised, metadataIssuer string) bool {
	if advertised == metadataIssuer {
		return true
	}
	left, leftErr := url.Parse(advertised)
	right, rightErr := url.Parse(metadataIssuer)
	return leftErr == nil && rightErr == nil && secureHTTPS(metadataIssuer) &&
		strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

func (c *Client) Register(ctx context.Context, config Config, metadata Metadata, redirectURI string) (Registration, error) {
	if !validLoopbackRedirect(redirectURI) {
		return Registration{}, errors.New("invalid loopback redirect URI")
	}
	if strings.TrimSpace(config.ClientID) != "" {
		return Registration{ClientID: strings.TrimSpace(config.ClientID)}, nil
	}
	if !secureHTTPS(metadata.RegistrationEndpoint) {
		return Registration{}, errors.New("invalid dynamic-registration endpoint or redirect URI")
	}
	grantTypes := []string{"authorization_code"}
	if contains(metadata.GrantTypesSupported, "refresh_token") {
		grantTypes = append(grantTypes, "refresh_token")
	}
	registrationRequest := map[string]any{
		"redirect_uris": []string{redirectURI}, "token_endpoint_auth_method": "none",
		"grant_types": grantTypes, "response_types": []string{"code"},
		"application_type": "native", "client_name": brand.DisplayName,
	}
	if len(config.Scopes) > 0 {
		registrationRequest["scope"] = strings.Join(config.Scopes, " ")
	}
	body, _ := json.Marshal(registrationRequest)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.RegistrationEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return Registration{}, errors.New("create dynamic-registration request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Registration{}, errors.New("request dynamic client registration")
	}
	defer resp.Body.Close()
	raw, err := readBounded(resp.Body)
	if err != nil || resp.StatusCode/100 != 2 {
		return Registration{}, fmt.Errorf("dynamic client registration returned HTTP %d", resp.StatusCode)
	}
	clientID, err := parseRegistration(raw, redirectURI, config.Scopes, grantTypes)
	if err != nil {
		return Registration{}, err
	}
	return Registration{ClientID: clientID}, nil
}

func parseRegistration(raw []byte, redirectURI string, scopes, grantTypes []string) (string, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields["client_secret"] != nil || fields["client_secret_expires_at"] != nil {
		return "", errors.New("dynamic registration returned confidential-client data")
	}
	var response struct {
		ClientID                string   `json:"client_id"`
		RedirectURIs            []string `json:"redirect_uris"`
		Scope                   string   `json:"scope"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return "", errors.New("dynamic-registration response is invalid")
	}
	clientID := strings.TrimSpace(response.ClientID)
	if clientID == "" || len(clientID) > 512 || len(response.RedirectURIs) != 1 || response.RedirectURIs[0] != redirectURI || response.TokenEndpointAuthMethod != "" && response.TokenEndpointAuthMethod != "none" || strings.TrimSpace(response.Scope) != "" && !containsAll(strings.Fields(response.Scope), scopes) {
		return "", errors.New("dynamic-registration response does not match the public client request")
	}
	if len(response.GrantTypes) > 0 && !containsAll(response.GrantTypes, grantTypes) || len(response.ResponseTypes) > 0 && !contains(response.ResponseTypes, "code") {
		return "", errors.New("dynamic-registration response conflicts with the public client request")
	}
	return clientID, nil
}

func AuthorizationURL(config Config, metadata Metadata, registration Registration, redirectURI, state, challenge string) (string, error) {
	if !secureHTTPS(metadata.AuthorizationEndpoint) || strings.TrimSpace(registration.ClientID) == "" || !validLoopbackRedirect(redirectURI) || state == "" || challenge == "" {
		return "", errors.New("incomplete authorization request")
	}
	u, _ := url.Parse(metadata.AuthorizationEndpoint)
	query := u.Query()
	query.Set("response_type", "code")
	query.Set("client_id", registration.ClientID)
	query.Set("redirect_uri", redirectURI)
	if len(config.Scopes) > 0 {
		query.Set("scope", strings.Join(config.Scopes, " "))
	}
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	if config.Resource != "" {
		query.Set("resource", config.Resource)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (c *Client) Exchange(ctx context.Context, config Config, metadata Metadata, registration Registration, redirectURI, code, verifier string) (Token, error) {
	if !secureHTTPS(metadata.TokenEndpoint) || strings.TrimSpace(registration.ClientID) == "" || !validLoopbackRedirect(redirectURI) || strings.TrimSpace(code) == "" || strings.TrimSpace(verifier) == "" {
		return Token{}, errors.New("incomplete token request")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {registration.ClientID}, "redirect_uri": {redirectURI}, "code_verifier": {verifier}}
	if config.Resource != "" {
		form.Set("resource", config.Resource)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, metadata.TokenEndpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Token{}, errors.New("request OAuth token")
	}
	defer resp.Body.Close()
	raw, err := readBounded(resp.Body)
	if err != nil || resp.StatusCode/100 != 2 {
		return Token{}, fmt.Errorf("token endpoint returned HTTP %d", resp.StatusCode)
	}
	return parseTokenResponse(raw, false)
}

// Refresh exchanges one public-client refresh token at the token endpoint
// advertised by OAuth metadata. It deliberately has no client-secret or HTTP
// Basic authentication path.
func (c *Client) Refresh(ctx context.Context, config Config, metadata Metadata, registration Registration, refreshToken string) (Token, error) {
	clientID := strings.TrimSpace(registration.ClientID)
	refreshToken = strings.TrimSpace(refreshToken)
	if c == nil || c.HTTP == nil || !secureHTTPS(metadata.TokenEndpoint) || clientID == "" || refreshToken == "" {
		return Token{}, errors.New("incomplete refresh token request")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	if config.Resource != "" {
		form.Set("resource", config.Resource)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, errors.New("create OAuth refresh request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Token{}, errors.New("request OAuth refresh token")
	}
	defer resp.Body.Close()
	raw, err := readBounded(resp.Body)
	if err != nil {
		return Token{}, errors.New("read OAuth refresh response")
	}
	if resp.StatusCode/100 != 2 {
		var oauthError struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &oauthError) == nil && oauthError.Error == "invalid_grant" {
			return Token{}, ErrInvalidGrant
		}
		return Token{}, fmt.Errorf("refresh token endpoint returned HTTP %d", resp.StatusCode)
	}
	return parseTokenResponse(raw, true)
}

func parseTokenResponse(raw []byte, requireExpiry bool) (Token, error) {
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if json.Unmarshal(raw, &response) != nil || strings.TrimSpace(response.AccessToken) == "" || response.TokenType != "" && !strings.EqualFold(response.TokenType, "Bearer") || requireExpiry && response.ExpiresIn <= 0 {
		return Token{}, errors.New("OAuth token response is incomplete")
	}
	if response.TokenType == "" {
		response.TokenType = "Bearer"
	}
	var expiresAt time.Time
	if response.ExpiresIn > 0 {
		expiresAt = time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)
	}
	return Token{
		AccessToken:  strings.TrimSpace(response.AccessToken),
		RefreshToken: strings.TrimSpace(response.RefreshToken),
		TokenType:    response.TokenType,
		Scope:        strings.TrimSpace(response.Scope),
		ExpiresAt:    expiresAt,
	}, nil
}

type CallbackListener struct {
	RedirectURI string
	listener    net.Listener
	server      *http.Server
	state       string
	result      chan callbackResult
	mu          sync.Mutex
	consumed    bool
}

type callbackResult struct {
	code string
	err  error
}

func StartCallbackListener(state string) (*CallbackListener, error) {
	if strings.TrimSpace(state) == "" {
		return nil, errors.New("OAuth state is required")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("start OAuth callback listener")
	}
	callback := &CallbackListener{RedirectURI: "http://" + listener.Addr().String() + callbackPath, listener: listener, state: state, result: make(chan callbackResult, 1)}
	callback.server = &http.Server{Handler: http.HandlerFunc(callback.handle), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = callback.server.Serve(listener) }()
	return callback, nil
}

func (c *CallbackListener) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet || r.URL.Path != callbackPath {
		http.NotFound(w, r)
		return
	}
	if r.URL.Query().Get("state") != c.state {
		http.Error(w, "Authorization state mismatch.", http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	if c.consumed {
		c.mu.Unlock()
		http.Error(w, "Authorization callback was already used.", http.StatusConflict)
		return
	}
	c.consumed = true
	c.mu.Unlock()
	result := callbackResult{code: strings.TrimSpace(r.URL.Query().Get("code"))}
	if strings.TrimSpace(r.URL.Query().Get("error")) != "" {
		result.err = errors.New("authorization was not completed")
	} else if result.code == "" {
		result.err = errors.New("authorization callback did not include a code")
	}
	c.result <- result
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><meta charset=utf-8><title>%s</title><p>授权已处理，可以返回 %s。</p>", brand.DisplayName, brand.DisplayName)
}

func (c *CallbackListener) Wait(ctx context.Context) (string, error) {
	if c == nil {
		return "", errors.New("OAuth callback listener is unavailable")
	}
	select {
	case result := <-c.result:
		return result.code, result.err
	case <-ctx.Done():
		return "", errors.New("authorization timed out or was cancelled")
	}
}

func (c *CallbackListener) Close() {
	if c == nil || c.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = c.server.Shutdown(ctx)
}

func NewPKCE() (state, verifier, challenge string, err error) {
	state, err = randomURLSafe(32)
	if err != nil {
		return "", "", "", err
	}
	verifier, err = randomURLSafe(64)
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(verifier))
	return state, verifier, base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func randomURLSafe(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string) ([]byte, error) {
	if !secureHTTPS(endpoint) {
		return nil, errors.New("OAuth metadata endpoint is not HTTPS")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("request OAuth metadata")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth metadata returned HTTP %d", resp.StatusCode)
	}
	return readBounded(resp.Body)
}

func protectedMetadataCandidates(resource string) []string {
	u, err := url.Parse(resource)
	if err != nil {
		return nil
	}
	origin := u.Scheme + "://" + u.Host
	pathCandidate := origin + "/.well-known/oauth-protected-resource" + strings.TrimRight(u.EscapedPath(), "/")
	rootCandidate := origin + "/.well-known/oauth-protected-resource"
	if pathCandidate == rootCandidate {
		return []string{rootCandidate}
	}
	return []string{pathCandidate, rootCandidate}
}

func authorizationMetadataCandidates(issuer string) []string {
	u, err := url.Parse(issuer)
	if err != nil {
		return nil
	}
	origin := u.Scheme + "://" + u.Host
	pathPart := strings.TrimRight(u.EscapedPath(), "/")
	return []string{origin + "/.well-known/oauth-authorization-server" + pathPart, origin + "/.well-known/openid-configuration" + pathPart}
}

func readBounded(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxResponseBytes {
		return nil, errors.New("OAuth response is too large")
	}
	return raw, nil
}
func secureHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == ""
}
func validLoopbackRedirect(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "http" && u.Hostname() == "127.0.0.1" && u.Port() != "" && u.Path == callbackPath && u.RawQuery == "" && u.Fragment == "" && u.User == nil
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func containsAll(values, targets []string) bool {
	for _, target := range targets {
		if !contains(values, target) {
			return false
		}
	}
	return true
}
