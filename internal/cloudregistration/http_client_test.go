package cloudregistration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestHTTPClientRegistrationFlowUsesDocumentedWireContract(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		if request.Header.Get("X-Request-Id") != "req-a12" {
			t.Fatalf("X-Request-Id = %q", request.Header.Get("X-Request-Id"))
		}
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Fatalf("registration request sent Authorization = %q", authorization)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/auth/captcha-config":
			if request.Method != http.MethodGet {
				t.Fatalf("captcha config method = %s", request.Method)
			}
			_, _ = io.WriteString(w, `{"data":{"provider":"aliyun_captcha_v2","region":"cn","prefix":"public-prefix","scene_id":"register-scene"},"error":null,"request_id":"req-a12","server_time":"2026-07-14T04:00:00Z"}`)
		case "/api/v1/auth/register-password-crypto":
			if request.Method != http.MethodGet {
				t.Fatalf("password crypto method = %s", request.Method)
			}
			_, _ = io.WriteString(w, `{"data":{"algorithm":"AES","key_size_bits":128,"mode":"CBC","padding":"PKCS7Padding","output":"Base64","ciphertext_encoding":"base64(iv || ciphertext)","key_version":"auth-key-v1","key_base64":"MDEyMzQ1Njc4OWFiY2RlZg==","expires_at":"2099-07-14T04:05:00Z"},"error":null,"request_id":"req-a12","server_time":"2026-07-14T04:00:00Z"}`)
		case "/api/v1/auth/sms/register/send":
			if request.Method != http.MethodPost {
				t.Fatalf("send SMS method = %s", request.Method)
			}
			var body struct {
				Phone           string               `json:"phone"`
				CaptchaTicket   string               `json:"captcha_ticket"`
				CaptchaProvider string               `json:"captcha_provider"`
				Device          cloud.DeviceIdentity `json:"device"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Phone != "+8613900000002" || body.CaptchaTicket != "opaque-captcha-a12" || body.CaptchaProvider != "aliyun_captcha_v2" {
				t.Fatalf("SMS body = %#v", body)
			}
			assertRegistrationDevice(t, body.Device)
			_, _ = io.WriteString(w, `{"data":{"phone":"+8613900000002","expires_in_seconds":300},"error":null,"request_id":"req-a12","server_time":"2026-07-14T04:00:01Z"}`)
		case "/api/v1/auth/sms/password-reset/send":
			if request.Method != http.MethodPost {
				t.Fatalf("send password reset SMS method = %s", request.Method)
			}
			var body struct {
				Phone           string               `json:"phone"`
				CaptchaTicket   string               `json:"captcha_ticket"`
				CaptchaProvider string               `json:"captcha_provider"`
				Device          cloud.DeviceIdentity `json:"device"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Phone != "+8613900000002" || body.CaptchaTicket != "opaque-reset-captcha-a12" || body.CaptchaProvider != "aliyun_captcha_v2" {
				t.Fatalf("password reset SMS body = %#v", body)
			}
			assertRegistrationDevice(t, body.Device)
			_, _ = io.WriteString(w, `{"data":{"phone":"13900000002","expires_in_seconds":300},"error":null,"request_id":"req-a12","server_time":"2026-07-14T04:00:01Z"}`)
		case "/api/v1/auth/password/reset":
			if request.Method != http.MethodPost {
				t.Fatalf("reset password method = %s", request.Method)
			}
			var body struct {
				Phone       string          `json:"phone"`
				NewPassword string          `json:"new_password"`
				SMSCode     string          `json:"sms_code"`
				Device      json.RawMessage `json:"device"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Phone != "+8613900000002" || body.NewPassword != "new-password-a12" || body.SMSCode != "654321" || len(body.Device) != 0 {
				t.Fatalf("password reset body = %#v", body)
			}
			_, _ = io.WriteString(w, `{"data":{"reset":true},"error":null,"request_id":"req-a12","server_time":"2026-07-14T04:00:02Z"}`)
		case "/api/v1/auth/register":
			if request.Method != http.MethodPost {
				t.Fatalf("register method = %s", request.Method)
			}
			var body struct {
				Phone             string `json:"phone"`
				EncryptedPassword string `json:"encrypted_password"`
				PasswordCrypto    struct {
					Algorithm   string `json:"algorithm"`
					KeySize     int    `json:"key_size"`
					KeySizeBits int    `json:"key_size_bits"`
					Mode        string `json:"mode"`
					Padding     string `json:"padding"`
					Output      string `json:"output"`
					KeyVersion  string `json:"key_version"`
				} `json:"password_crypto"`
				SMSCode string               `json:"sms_code"`
				Device  cloud.DeviceIdentity `json:"device"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Phone != "+8613900000002" || body.EncryptedPassword != "encrypted-a12" || body.SMSCode != "123456" {
				t.Fatalf("register body = %#v", body)
			}
			if body.PasswordCrypto.Algorithm != "AES" || body.PasswordCrypto.KeySize != 128 || body.PasswordCrypto.KeySizeBits != 0 || body.PasswordCrypto.KeyVersion != "auth-key-v1" {
				t.Fatalf("password_crypto = %#v", body.PasswordCrypto)
			}
			assertRegistrationDevice(t, body.Device)
			_, _ = io.WriteString(w, validRegistrationEnvelope())
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, server.Client(), registrationDevice(), func() string { return "req-a12" })
	if err != nil {
		t.Fatal(err)
	}
	captchaConfig, err := client.CaptchaConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if captchaConfig.Prefix != "public-prefix" || captchaConfig.SceneID != "register-scene" {
		t.Fatalf("captcha config = %#v", captchaConfig)
	}
	cryptoConfig, err := client.PasswordCrypto(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cryptoConfig.KeySizeBits != 128 || cryptoConfig.KeyVersion != "auth-key-v1" {
		t.Fatalf("password crypto = %#v", cryptoConfig)
	}
	sms, err := client.SendRegistrationSMS(context.Background(), cloud.SendRegistrationSMSRequest{Phone: "+8613900000002", CaptchaTicket: "opaque-captcha-a12", CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2})
	if err != nil {
		t.Fatal(err)
	}
	if sms.Phone != "+8613900000002" || sms.ExpiresInSeconds != 300 {
		t.Fatalf("SMS result = %#v", sms)
	}
	resetSMS, err := client.SendPasswordResetSMS(context.Background(), cloud.SendPasswordResetSMSRequest{
		Phone: "+8613900000002", CaptchaTicket: "opaque-reset-captcha-a12", CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resetSMS.Phone != "13900000002" || resetSMS.ExpiresInSeconds != 300 {
		t.Fatalf("password reset SMS result = %#v", resetSMS)
	}
	reset, err := client.ResetPassword(context.Background(), cloud.PasswordResetRequest{
		Phone: "+8613900000002", NewPassword: "new-password-a12", SMSCode: "654321",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reset.Reset {
		t.Fatalf("password reset result = %#v", reset)
	}
	result, err := client.Register(context.Background(), cloud.RegistrationRequest{
		Phone: "+8613900000002", EncryptedPassword: "encrypted-a12", SMSCode: "123456",
		PasswordCrypto: cryptoConfig.Metadata(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authentication.User.CloudUserID != "usr_a12" || result.Authentication.User.Role != "user" || result.Authentication.Session.DeviceID != "device-a12" || result.Authentication.ServerTime != mustTime(t, "2026-07-14T04:00:02Z") {
		t.Fatalf("registration authentication = %#v", result.Authentication)
	}
	if !result.SignupReward.Granted || result.ThirdPartySync.Status != cloud.ThirdPartySyncPending || result.Balance.TotalBalance != "1000.0000" {
		t.Fatalf("registration result = %#v", result)
	}
	if got := strings.Join(requests, ","); got != "GET /api/v1/auth/captcha-config,GET /api/v1/auth/register-password-crypto,POST /api/v1/auth/sms/register/send,POST /api/v1/auth/sms/password-reset/send,POST /api/v1/auth/password/reset,POST /api/v1/auth/register" {
		t.Fatalf("requests = %s", got)
	}
}

func TestHTTPClientMapsRegistrationNotReadyAndCloudErrors(t *testing.T) {
	t.Run("registration dependencies not ready", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"data":null,"error":{"code":"NOT_FOUND","message":"not found"},"request_id":"req-404","server_time":"2026-07-14T04:00:00Z"}`)
		}))
		defer server.Close()
		client := mustHTTPClient(t, server)
		_, err := client.Register(context.Background(), validRegistrationRequest())
		if !hasCloudCode(err, cloud.ErrorCode("CAPABILITY_UNAVAILABLE")) || !strings.Contains(err.Error(), "dependencies") {
			t.Fatalf("Register error = %v", err)
		}
		assertNoRegistrationSecrets(t, err)
	})

	t.Run("documented cloud error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"data":null,"error":{"code":"SMS_RATE_LIMITED","message":"try later","retryable":true},"request_id":"req-rate","server_time":"2026-07-14T04:00:00Z"}`)
		}))
		defer server.Close()
		client := mustHTTPClient(t, server)
		_, err := client.SendRegistrationSMS(context.Background(), cloud.SendRegistrationSMSRequest{Phone: "+8613900000002", CaptchaTicket: "opaque-captcha-a12-secret", CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2})
		if !hasCloudCode(err, cloud.ErrorCode("SMS_RATE_LIMITED")) {
			t.Fatalf("SendRegistrationSMS error = %v", err)
		}
		var typed *cloud.Error
		if !errors.As(err, &typed) || !typed.Retryable || typed.RequestID != "req-rate" {
			t.Fatalf("SendRegistrationSMS error = %#v, want retryable req-rate", err)
		}
		assertNoRegistrationSecrets(t, err)
	})

	t.Run("documented retryable service error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"data":null,"error":{"code":"SERVICE_UNAVAILABLE","message":"try later","retryable":true},"request_id":"req-503","server_time":"2026-07-14T04:00:00Z"}`)
		}))
		defer server.Close()
		client := mustHTTPClient(t, server)
		_, err := client.SendRegistrationSMS(context.Background(), cloud.SendRegistrationSMSRequest{Phone: "+8613900000002", CaptchaTicket: "opaque-captcha-a12-secret", CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2})
		var typed *cloud.Error
		if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("SERVICE_UNAVAILABLE") || !typed.Retryable || typed.RequestID != "req-503" {
			t.Fatalf("SendRegistrationSMS error = %#v, want retryable SERVICE_UNAVAILABLE req-503", err)
		}
		assertNoRegistrationSecrets(t, err)
	})
}

func TestHTTPClientRejectsMalformedOversizedAndOversizedRequest(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{"data":`},
		{name: "oversized", body: strings.Repeat("x", (1<<20)+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			client := mustHTTPClient(t, server)
			_, err := client.PasswordCrypto(context.Background())
			if !hasCloudCode(err, cloud.ErrorCode("CLOUD_PROTOCOL_ERROR")) {
				t.Fatalf("PasswordCrypto error = %v", err)
			}
		})
	}

	calls := 0
	client, err := NewHTTPClient("http://127.0.0.1:1", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("must not send")
	})}, registrationDevice(), func() string { return "req-a12" })
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SendRegistrationSMS(context.Background(), cloud.SendRegistrationSMSRequest{
		Phone: strings.Repeat("1", (16<<10)+1), CaptchaTicket: "opaque-captcha-a12", CaptchaProvider: cloud.LoginCaptchaProviderAliyunCaptchaV2,
	})
	if err == nil || !strings.Contains(err.Error(), "16 KiB") {
		t.Fatalf("oversized request error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("oversized request reached transport %d times", calls)
	}
}

func TestHTTPClientMapsTimeoutToNetworkUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-request.Context().Done()
	}))
	defer server.Close()
	client := mustHTTPClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := client.PasswordCrypto(ctx)
	if !hasCloudCode(err, cloud.ErrorNetworkUnavailable) {
		t.Fatalf("PasswordCrypto error = %v", err)
	}
}

func TestHTTPClientMapsDisconnectWithoutLeakingRegistrationSecrets(t *testing.T) {
	client, err := NewHTTPClient("http://127.0.0.1:1", &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		}),
	}, registrationDevice(), func() string { return "req-disconnect" })
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Register(context.Background(), validRegistrationRequest())
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorNetworkUnavailable || !typed.Retryable || typed.RequestID != "req-disconnect" {
		t.Fatalf("Register error = %#v, want retryable disconnect network error", err)
	}
	assertNoRegistrationSecrets(t, err)
}

func TestHTTPClientSupportsExplicitCustomCA(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"provider":"aliyun_captcha_v2","region":"cn","prefix":"public-prefix","scene_id":"register-scene"},"error":null,"request_id":"req-ca","server_time":"2026-07-14T04:00:00Z"}`)
	}))
	defer server.Close()

	untrusted, err := NewHTTPClient(server.URL, &http.Client{}, registrationDevice(), func() string { return "req-untrusted-ca" })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := untrusted.CaptchaConfig(context.Background()); !hasCloudCode(err, cloud.ErrorNetworkUnavailable) {
		t.Fatalf("CaptchaConfig with untrusted CA error = %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	trusted, err := NewHTTPClient(server.URL, &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
	}}, registrationDevice(), func() string { return "req-ca" })
	if err != nil {
		t.Fatal(err)
	}
	config, err := trusted.CaptchaConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if config.Prefix != "public-prefix" || config.SceneID != "register-scene" {
		t.Fatalf("captcha config = %#v", config)
	}
}

func TestHTTPClientRejectsPublicPlainHTTP(t *testing.T) {
	_, err := NewHTTPClient("http://example.com:8080", http.DefaultClient, registrationDevice(), nil)
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("NewHTTPClient error = %v", err)
	}
}

func mustHTTPClient(t *testing.T, server *httptest.Server) *HTTPClient {
	t.Helper()
	client, err := NewHTTPClient(server.URL, server.Client(), registrationDevice(), func() string { return "req-a12" })
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func registrationDevice() cloud.DeviceIdentity {
	return cloud.DeviceIdentity{DeviceID: "device-a12", DeviceName: "Windows PC", ClientVersion: "0.9.0"}
}

func assertRegistrationDevice(t *testing.T, device cloud.DeviceIdentity) {
	t.Helper()
	if device != registrationDevice() {
		t.Fatalf("device = %#v", device)
	}
}

func validRegistrationRequest() cloud.RegistrationRequest {
	return cloud.RegistrationRequest{
		Phone: "+8613900000002", EncryptedPassword: "encrypted-a12-secret", SMSCode: "123456",
		PasswordCrypto: cloud.RegistrationPasswordMetadata{
			Algorithm: "AES", KeySize: 128, Mode: "CBC", Padding: "PKCS7Padding", Output: "Base64", KeyVersion: "auth-key-v1",
		},
	}
}

func validRegistrationEnvelope() string {
	return `{"data":{"user":{"user_id":"usr_a12","display_name":"Reasonix User","avatar_url":null,"status":"active","role":"user","created_at":"2026-07-14T04:00:00Z"},"session":{"access_token":"access-a12","access_token_expires_at":"2026-07-14T05:00:00Z","refresh_token":"refresh-a12","refresh_token_expires_at":"2026-08-13T04:00:00Z"},"signup_reward":{"granted":true,"amount":"1000.0000","ledger_id":"led_a12"},"third_party_sync":{"sync_id":"tps_a12","provider":"new-api","status":"pending","version":1,"updated_at":"2026-07-14T04:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":2}},"error":null,"request_id":"req-a12","server_time":"2026-07-14T04:00:02Z"}`
}

func hasCloudCode(err error, code cloud.ErrorCode) bool {
	var cloudErr *cloud.Error
	return errors.As(err, &cloudErr) && cloudErr.Code == code
}

func assertNoRegistrationSecrets(t *testing.T, err error) {
	t.Helper()
	for _, secret := range []string{"encrypted-a12-secret", "dev-captcha-a12-secret", "opaque-captcha-a12-secret", "+8613900000002", "123456"} {
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked registration secret %q", secret)
		}
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
