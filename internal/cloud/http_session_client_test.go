package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPSessionClientLoginUsesProductionWireContract(t *testing.T) {
	t.Parallel()

	serverTime := "2026-07-13T03:00:00Z"
	accessExpiry := "2026-07-13T04:00:00Z"
	refreshExpiry := "2026-08-12T03:00:00Z"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Request-Id"); got != "req-test" {
			t.Fatalf("X-Request-Id = %q", got)
		}
		var body struct {
			Phone           string         `json:"phone"`
			Password        string         `json:"password"`
			CaptchaTicket   string         `json:"captcha_ticket"`
			CaptchaProvider string         `json:"captcha_provider"`
			Device          DeviceIdentity `json:"device"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Phone != "+8613800138000" || body.Password != "local-secret" {
			t.Fatalf("unexpected credentials: phone=%q password_length=%d", body.Phone, len(body.Password))
		}
		if body.Device.DeviceID != "device-test" || body.Device.DeviceName != "Windows PC" || body.Device.ClientVersion != "0.9.0" {
			t.Fatalf("unexpected device: %#v", body.Device)
		}
		if body.CaptchaTicket == "" {
			t.Fatal("login request omitted captcha_ticket")
		}
		if body.CaptchaProvider != LoginCaptchaProviderAliyunCaptchaV2 {
			t.Fatalf("captcha_provider = %q", body.CaptchaProvider)
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatal("login request included an authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_01","display_name":"Reasonix User","avatar_url":null,"status":"active","role":"user","created_at":"2026-07-13T03:00:00Z"},"session":{"access_token":"access-01","access_token_expires_at":"` + accessExpiry + `","refresh_token":"refresh-01","refresh_token_expires_at":"` + refreshExpiry + `"}},"error":null,"request_id":"req-test","server_time":"` + serverTime + `"}`))
	}))
	defer server.Close()

	client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{
		DeviceID: "device-test", DeviceName: "Windows PC", ClientVersion: "0.9.0",
	}, func() string { return "req-test" })
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Login(context.Background(), LoginRequest{
		Phone: "+8613800138000", Password: "local-secret",
		CaptchaTicket: "test-captcha-ticket", CaptchaProvider: LoginCaptchaProviderAliyunCaptchaV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User != (CloudUser{CloudUserID: "usr_01", DisplayName: "Reasonix User", Role: "user"}) {
		t.Fatalf("unexpected user: %#v", result.User)
	}
	if result.Session.AccessToken != "access-01" || result.Session.RefreshToken != "refresh-01" {
		t.Fatalf("unexpected session tokens")
	}
	if result.ServerTime != mustParseHTTPTestTime(t, serverTime) || result.Session.AccessTokenExpiresAt != mustParseHTTPTestTime(t, accessExpiry) || result.Session.RefreshTokenExpiresAt != mustParseHTTPTestTime(t, refreshExpiry) {
		t.Fatalf("unexpected timestamps: %#v", result)
	}
}

func TestHTTPSessionClientLoginRejectsMissingCaptchaBeforeHTTP(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer server.Close()

	client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{
		DeviceID: "device-test", ClientVersion: "0.9.0",
	}, func() string { return "req-test" })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Login(context.Background(), LoginRequest{Phone: "+8613800138000", Password: "local-secret"}); err == nil {
		t.Fatal("Login accepted a request without CAPTCHA verification")
	}
	if calls != 0 {
		t.Fatalf("HTTP login calls = %d, want 0", calls)
	}
}

func TestHTTPSessionClientRefreshLogoutAndCurrentUser(t *testing.T) {
	t.Parallel()

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["refresh_token"] != "refresh-01" || body["device_id"] != "device-test" {
				t.Fatalf("unexpected refresh body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"data":{"session":{"access_token":"access-02","access_token_expires_at":"2026-07-13T05:00:00Z","refresh_token":"refresh-02","refresh_token_expires_at":"2026-08-12T04:00:00Z"}},"error":null,"request_id":"req-refresh","server_time":"2026-07-13T04:00:00Z"}`))
		case "/api/v1/me":
			if r.Header.Get("Authorization") != "Bearer access-02" {
				t.Fatalf("unexpected authorization header")
			}
			_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_01","display_name":"Reasonix User","status":"active","role":"user","created_at":"2026-07-13T03:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"subscription_id":null,"plan":"free","plan_config_version":null,"status":"free","period_start":null,"period_end":null,"version":0}},"error":null,"request_id":"req-me","server_time":"2026-07-13T04:00:01Z"}`))
		case "/api/v1/auth/logout":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["refresh_token"] != "refresh-02" || body["device_id"] != "device-test" || body["scope"] != "current_device" {
				t.Fatalf("unexpected logout body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"data":{"revoked":true},"error":null,"request_id":"req-logout","server_time":"2026-07-13T04:00:02Z"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-test", DeviceName: "Windows PC", ClientVersion: "0.9.0"}, func() string { return "req-test" })
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := client.Refresh(context.Background(), "refresh-01")
	if err != nil || refreshed.RefreshToken != "refresh-02" {
		t.Fatalf("refresh = %#v, %v", refreshed, err)
	}
	snapshot, err := client.CurrentUser(context.Background(), refreshed.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.User.CloudUserID != "usr_01" || snapshot.User.Role != "user" || snapshot.Balance.TotalBalance != "1000.0000" || snapshot.Subscription.Plan != "free" || snapshot.Subscription.Version != 0 {
		t.Fatalf("unexpected current user: %#v", snapshot)
	}
	if err := client.Logout(context.Background(), refreshed.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(requests, ","); got != "POST /api/v1/auth/refresh,GET /api/v1/me,POST /api/v1/auth/logout" {
		t.Fatalf("requests = %s", got)
	}
}

func TestHTTPSessionClientCurrentUserProjectsV1Capability(t *testing.T) {
	t.Parallel()
	valid := `{"enabled":true,"contract_version":"projects-v1-20260805","contract_hash_algorithm":"sha256","contract_hash":"` + strings.Repeat("a", 64) + `","instructions":true,"work_items":true}`
	cases := []struct {
		name       string
		capability string
		want       string
		protocol   bool
	}{
		{name: "absent"},
		{name: "disabled", capability: `{"enabled":false}`, want: `{"enabled":false}`},
		{name: "supported raw", capability: valid, want: valid},
		{name: "null is invalid", capability: `null`, protocol: true},
		{name: "duplicate key is invalid", capability: `{"enabled":true,"enabled":false}`, protocol: true},
		{name: "oversized is invalid", capability: `{"padding":"` + strings.Repeat("x", currentUserProjectsV1CapabilityMaxBytes) + `"}`, protocol: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			capabilities := ""
			if test.capability != "" {
				capabilities = `,"capabilities":{"projects_v1":` + test.capability + `}`
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_01","display_name":"Reasonix User","status":"active","role":"user","created_at":"2026-07-13T03:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0}` + capabilities + `},"error":null,"request_id":"req-me","server_time":"2026-07-13T04:00:01Z"}`))
			}))
			defer server.Close()
			client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-test", ClientVersion: "dev"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := client.CurrentUser(context.Background(), "access-token")
			if test.protocol {
				if !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
					t.Fatalf("CurrentUser error = %v, want CLOUD_PROTOCOL_ERROR", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := string(snapshot.Capabilities.ProjectsV1); got != test.want {
				t.Fatalf("projects_v1 raw = %q, want %q", got, test.want)
			}
		})
	}
}

func TestHTTPSessionClientCurrentUserProjectsV2Capability(t *testing.T) {
	t.Parallel()
	capability := `{"enabled":true,"contract_version":"projects-v2-test","contract_hash_algorithm":"sha256","contract_hash":"` + strings.Repeat("b", 64) + `","slices":{"members":false,"activity":false,"bindings":true,"work_item_fields":false,"assets":true}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"user":{"user_id":"usr_01","display_name":"Reasonix User","status":"active","role":"user","created_at":"2026-07-13T03:00:00Z"},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":1},"subscription":{"plan":"free","status":"free","period_end":null,"version":0},"capabilities":{"projects_v2":` + capability + `}},"error":null,"request_id":"req-me","server_time":"2026-07-13T04:00:01Z"}`))
	}))
	defer server.Close()
	client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-test", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.CurrentUser(context.Background(), "access-token")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(snapshot.Capabilities.ProjectsV2); got != capability {
		t.Fatalf("projects_v2 raw = %q, want %q", got, capability)
	}
}

func TestHTTPSessionClientRejectsPublicPlainHTTP(t *testing.T) {
	t.Parallel()
	_, err := NewHTTPSessionClient("http://example.com:8080", http.DefaultClient, DeviceIdentity{DeviceID: "device-test", ClientVersion: "dev"}, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS validation error, got %v", err)
	}
}

func TestHTTPSessionClientMapsCloudErrorWithoutUsingHTTPStatusText(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"INVALID_CREDENTIALS","message":"invalid credentials","retryable":false},"request_id":"req-error","server_time":"2026-07-13T04:00:00Z"}`))
	}))
	defer server.Close()
	client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-test", ClientVersion: "dev"}, func() string { return "req-test" })
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Login(context.Background(), LoginRequest{
		Phone: "+8613800138000", Password: "local-secret",
		CaptchaTicket: "test-captcha-ticket", CaptchaProvider: LoginCaptchaProviderAliyunCaptchaV2,
	})
	if !isCloudCode(err, ErrorCode("INVALID_CREDENTIALS")) {
		t.Fatalf("Login() error = %v, want INVALID_CREDENTIALS", err)
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.RequestID != "req-error" {
		t.Fatalf("Login() request id = %#v", err)
	}
	if strings.Contains(err.Error(), "local-secret") {
		t.Fatal("Login() error leaked password")
	}
}

func TestHTTPSessionClientRejectsMalformedAndOversizedResponses(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{"data":`},
		{name: "oversized", body: strings.Repeat("x", cloudAuthResponseLimit+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-test", ClientVersion: "dev"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CurrentUser(context.Background(), "access-secret")
			if !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
				t.Fatalf("CurrentUser() error = %v, want CLOUD_PROTOCOL_ERROR", err)
			}
			if strings.Contains(err.Error(), "access-secret") {
				t.Fatal("CurrentUser() error leaked access token")
			}
		})
	}
}

func TestHTTPSessionClientMapsRequestTimeoutToNetworkUnavailable(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-test", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = client.CurrentUser(ctx, "access-secret")
	if !isCloudCode(err, ErrorNetworkUnavailable) {
		t.Fatalf("CurrentUser() error = %v, want NETWORK_UNAVAILABLE", err)
	}
}

func mustParseHTTPTestTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
