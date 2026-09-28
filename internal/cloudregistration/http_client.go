package cloudregistration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kwu/internal/cloud"
)

const (
	maxRegistrationRequestBytes = 16 << 10
	registrationResponseLimit   = 1 << 20
	registrationRequestTimeout  = 20 * time.Second
)

type HTTPClient struct {
	baseURL   *url.URL
	http      *http.Client
	device    cloud.DeviceIdentity
	requestID func() string
}

func (c *HTTPClient) CaptchaConfig(ctx context.Context) (cloud.CaptchaConfig, error) {
	var data cloud.CaptchaConfig
	envelope, err := c.do(ctx, http.MethodGet, "/api/v1/auth/captcha-config", nil, &data)
	if err != nil {
		return cloud.CaptchaConfig{}, err
	}
	if err := data.Validate(); err != nil {
		return cloud.CaptchaConfig{}, cloud.AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return data, nil
}

func NewHTTPClient(baseURL string, client *http.Client, device cloud.DeviceIdentity, requestID func() string) (*HTTPClient, error) {
	parsed, err := validateBaseURL(baseURL)
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
		requestID = newRequestID
	}
	return &HTTPClient{baseURL: parsed, http: client, device: device, requestID: requestID}, nil
}

func (c *HTTPClient) PasswordCrypto(ctx context.Context) (cloud.RegistrationPasswordCrypto, error) {
	var data cloud.RegistrationPasswordCrypto
	envelope, err := c.do(ctx, http.MethodGet, "/api/v1/auth/register-password-crypto", nil, &data)
	if err != nil {
		return cloud.RegistrationPasswordCrypto{}, err
	}
	if err := data.Validate(time.Now().UTC()); err != nil {
		return cloud.RegistrationPasswordCrypto{}, cloud.AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return data, nil
}

func (c *HTTPClient) SendRegistrationSMS(ctx context.Context, input cloud.SendRegistrationSMSRequest) (cloud.RegistrationSMSResult, error) {
	if err := input.Validate(); err != nil {
		return cloud.RegistrationSMSResult{}, err
	}
	var data cloud.RegistrationSMSResult
	envelope, err := c.do(ctx, http.MethodPost, "/api/v1/auth/sms/register/send", registrationSMSWire{
		Phone: input.Phone, CaptchaTicket: input.CaptchaTicket,
		CaptchaProvider: input.CaptchaProvider, Device: c.device,
	}, &data)
	if err != nil {
		return cloud.RegistrationSMSResult{}, err
	}
	if err := data.Validate(); err != nil {
		return cloud.RegistrationSMSResult{}, cloud.AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return data, nil
}

func (c *HTTPClient) SendPasswordResetSMS(ctx context.Context, input cloud.SendPasswordResetSMSRequest) (cloud.PasswordResetSMSResult, error) {
	if err := input.Validate(); err != nil {
		return cloud.PasswordResetSMSResult{}, err
	}
	var data cloud.PasswordResetSMSResult
	envelope, err := c.do(ctx, http.MethodPost, "/api/v1/auth/sms/password-reset/send", passwordResetSMSWire{
		Phone: input.Phone, CaptchaTicket: input.CaptchaTicket,
		CaptchaProvider: input.CaptchaProvider, Device: c.device,
	}, &data)
	if err != nil {
		return cloud.PasswordResetSMSResult{}, err
	}
	if err := data.Validate(); err != nil {
		return cloud.PasswordResetSMSResult{}, cloud.AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return data, nil
}

func (c *HTTPClient) ResetPassword(ctx context.Context, input cloud.PasswordResetRequest) (cloud.PasswordResetResult, error) {
	if err := input.Validate(); err != nil {
		return cloud.PasswordResetResult{}, err
	}
	var data cloud.PasswordResetResult
	envelope, err := c.do(ctx, http.MethodPost, "/api/v1/auth/password/reset", passwordResetWire{
		Phone: input.Phone, NewPassword: input.NewPassword, SMSCode: input.SMSCode,
	}, &data)
	if err != nil {
		return cloud.PasswordResetResult{}, err
	}
	if err := data.Validate(); err != nil {
		return cloud.PasswordResetResult{}, cloud.AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return data, nil
}

func (c *HTTPClient) Register(ctx context.Context, input cloud.RegistrationRequest) (cloud.RegistrationResult, error) {
	if err := input.Validate(); err != nil {
		return cloud.RegistrationResult{}, err
	}
	var data registrationDataWire
	envelope, err := c.do(ctx, http.MethodPost, "/api/v1/auth/register", registrationWire{
		Phone: input.Phone, EncryptedPassword: input.EncryptedPassword,
		PasswordCrypto: input.PasswordCrypto, SMSCode: input.SMSCode, Device: c.device,
	}, &data)
	if err != nil {
		return cloud.RegistrationResult{}, err
	}
	result := data.domain(c.device.DeviceID, envelope.ServerTime)
	if err := result.Validate(); err != nil {
		return cloud.RegistrationResult{}, cloud.AttachRequestID(protocolError(err), envelope.RequestID)
	}
	return result, nil
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body, destination any) (responseEnvelope, error) {
	payload, err := requestPayload(body)
	if err != nil {
		return responseEnvelope{}, err
	}
	if len(payload) > maxRegistrationRequestBytes {
		return responseEnvelope{}, fmt.Errorf("cloud: authentication request exceeds 16 KiB")
	}
	ctx, cancel := requestContext(ctx)
	defer cancel()
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: path})
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return responseEnvelope{}, err
	}
	request.Header.Set("Accept", "application/json")
	requestID := c.requestID()
	request.Header.Set("X-Request-Id", requestID)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return responseEnvelope{RequestID: requestID}, cloud.AttachRequestID(networkError("cloud network is unavailable"), requestID)
	}
	defer response.Body.Close()
	responsePayload, err := readResponse(response.Body)
	if err != nil {
		return responseEnvelope{RequestID: requestID}, cloud.AttachRequestID(err, requestID)
	}
	envelope, err := decodeResponse(responsePayload, response.StatusCode, destination)
	if envelope.RequestID == "" {
		envelope.RequestID = requestID
	}
	return envelope, cloud.AttachRequestID(err, envelope.RequestID)
}

func requestPayload(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

func requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, registrationRequestTimeout)
}

func readResponse(body io.Reader) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(body, registrationResponseLimit+1))
	if err != nil {
		return nil, networkError("cloud response could not be read")
	}
	if len(payload) > registrationResponseLimit {
		return nil, protocolError(fmt.Errorf("response exceeds safety limit"))
	}
	return payload, nil
}

type responseEnvelope struct {
	RequestID  string
	ServerTime time.Time
}

type rawResponse struct {
	Data       json.RawMessage `json:"data"`
	Error      *cloud.Error    `json:"error"`
	RequestID  string          `json:"request_id"`
	ServerTime time.Time       `json:"server_time"`
}

func decodeResponse(payload []byte, statusCode int, destination any) (responseEnvelope, error) {
	var raw rawResponse
	if err := json.Unmarshal(payload, &raw); err != nil {
		return responseEnvelope{}, protocolError(err)
	}
	envelope := responseEnvelope{RequestID: raw.RequestID, ServerTime: raw.ServerTime}
	if statusCode == http.StatusNotFound && raw.Error != nil && raw.Error.Code == cloud.ErrorCode("NOT_FOUND") {
		err := &cloud.Error{Code: cloud.ErrorCode("CAPABILITY_UNAVAILABLE"), Message: "cloud authentication dependencies are not ready"}
		return envelope, cloud.AttachRequestID(err, raw.RequestID)
	}
	if raw.Error != nil {
		return envelope, cloud.AttachRequestID(raw.Error, raw.RequestID)
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

func networkError(message string) error {
	return &cloud.Error{Code: cloud.ErrorNetworkUnavailable, Message: message, Retryable: true}
}

func protocolError(cause error) error {
	return &cloud.Error{Code: cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"), Message: "cloud response is invalid: " + cause.Error()}
}

type registrationSMSWire struct {
	Phone           string               `json:"phone"`
	CaptchaTicket   string               `json:"captcha_ticket"`
	CaptchaProvider string               `json:"captcha_provider"`
	Device          cloud.DeviceIdentity `json:"device"`
}

type passwordResetSMSWire struct {
	Phone           string               `json:"phone"`
	CaptchaTicket   string               `json:"captcha_ticket"`
	CaptchaProvider string               `json:"captcha_provider"`
	Device          cloud.DeviceIdentity `json:"device"`
}

type passwordResetWire struct {
	Phone       string `json:"phone"`
	NewPassword string `json:"new_password"`
	SMSCode     string `json:"sms_code"`
}

type registrationWire struct {
	Phone             string                             `json:"phone"`
	EncryptedPassword string                             `json:"encrypted_password"`
	PasswordCrypto    cloud.RegistrationPasswordMetadata `json:"password_crypto"`
	SMSCode           string                             `json:"sms_code"`
	Device            cloud.DeviceIdentity               `json:"device"`
}

type authUserWire struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

func (u authUserWire) domain() cloud.CloudUser {
	return cloud.CloudUser{CloudUserID: u.UserID, DisplayName: u.DisplayName, Role: u.Role}
}

type authSessionWire struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func (s authSessionWire) domain(deviceID string) cloud.RemoteSession {
	return cloud.RemoteSession{
		AccessToken: s.AccessToken, AccessTokenExpiresAt: s.AccessTokenExpiresAt,
		RefreshToken: s.RefreshToken, RefreshTokenExpiresAt: s.RefreshTokenExpiresAt, DeviceID: deviceID,
	}
}

type registrationDataWire struct {
	User           authUserWire                     `json:"user"`
	Session        authSessionWire                  `json:"session"`
	SignupReward   cloud.RegistrationSignupReward   `json:"signup_reward"`
	ThirdPartySync cloud.RegistrationThirdPartySync `json:"third_party_sync"`
	Balance        cloud.BalanceSnapshot            `json:"balance"`
}

func (w registrationDataWire) domain(deviceID string, serverTime time.Time) cloud.RegistrationResult {
	return cloud.RegistrationResult{
		Authentication: cloud.AuthenticationResult{
			User: w.User.domain(), Session: w.Session.domain(deviceID), ServerTime: serverTime,
		},
		SignupReward: w.SignupReward, ThirdPartySync: w.ThirdPartySync, Balance: w.Balance,
	}
}

func validateBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("cloud: base URL must be absolute")
	}
	if err := validateURLShape(parsed); err != nil {
		return nil, err
	}
	if err := validateURLScheme(parsed); err != nil {
		return nil, err
	}
	parsed.Path, parsed.RawPath = "", ""
	return parsed, nil
}

func validateURLShape(parsed *url.URL) error {
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

func validateURLScheme(parsed *url.URL) error {
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		if !privateHost(parsed.Hostname()) {
			return fmt.Errorf("cloud: HTTPS is required for non-private cloud endpoints")
		}
		return nil
	default:
		return fmt.Errorf("cloud: base URL must use HTTP or HTTPS")
	}
}

func privateHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return "req-" + hex.EncodeToString(value[:])
}

var _ cloud.CloudRegistrationClient = (*HTTPClient)(nil)
var _ cloud.CloudPasswordResetClient = (*HTTPClient)(nil)
