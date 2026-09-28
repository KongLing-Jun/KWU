package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPCreditClientCreateFreezeUsesProtectedWireContract(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	request := FreezeRequest{Scope: scope, TaskID: "task_01", ModelID: "reasonix-mock-chat", ClientTaskType: "chat", ClientEstimatedCost: "0.0000", IdempotencyKey: "idem_credit_01"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/credit-freezes" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-current" || r.Header.Get("X-Device-Id") != "device-01" || r.Header.Get("X-Client-Version") != "dev" || r.Header.Get("X-Request-Id") != "req-credit" || r.Header.Get("X-Idempotency-Key") != request.IdempotencyKey {
			t.Fatalf("protected headers = %#v", r.Header)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["task_id"] != request.TaskID || body["model_id"] != request.ModelID || body["client_task_type"] != request.ClientTaskType || body["client_estimated_cost"] != request.ClientEstimatedCost || len(body) != 4 {
			t.Fatalf("body = %#v", body)
		}
		writeCreditSuccess(w, "frz_01", request.TaskID)
	}))
	defer server.Close()

	client := newTestHTTPCreditClient(t, server, scope, nil)
	result, err := client.CreateFreeze(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Freeze.FreezeID != "frz_01" || result.Freeze.Scope != scope || result.Balance.TotalBalance != "1000.0000" || result.Balance.Version != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestHTTPCreditClientRefreshesAndReplaysOnce(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	request := FreezeRequest{Scope: scope, TaskID: "task_01", ModelID: "reasonix-mock-chat", ClientTaskType: "chat", ClientEstimatedCost: "0.0000", IdempotencyKey: "idem_credit_01"}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			if r.Header.Get("Authorization") != "Bearer access-current" {
				t.Fatal("first request did not use current token")
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null,"error":{"code":"TOKEN_EXPIRED","message":"expired"},"request_id":"req-first","server_time":"2026-07-15T04:00:00Z"}`))
			return
		}
		if calls != 2 || r.Header.Get("Authorization") != "Bearer access-rotated" {
			t.Fatalf("replay = %d / %q", calls, r.Header.Get("Authorization"))
		}
		writeCreditSuccess(w, "frz_01", request.TaskID)
	}))
	defer server.Close()
	refresher := &creditSessionClient{remote: creditRemoteSession()}
	if _, err := newTestHTTPCreditClient(t, server, scope, refresher).CreateFreeze(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || refresher.calls != 1 {
		t.Fatalf("calls/refreshes = %d/%d", calls, refresher.calls)
	}
}

func TestHTTPCreditClientGetFreezeRejectsUnsafePathAndProjectsScope(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/credit-freezes/frz_01" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Idempotency-Key") != "" || r.Header.Get("Content-Type") != "" {
			t.Fatalf("GET headers = %#v", r.Header)
		}
		writeCreditSuccess(w, "frz_01", "task_01")
	}))
	defer server.Close()
	client := newTestHTTPCreditClient(t, server, scope, nil)
	result, err := client.GetFreeze(context.Background(), scope, "frz_01")
	if err != nil || result.Freeze.Scope != scope {
		t.Fatalf("GetFreeze = %#v, %v", result, err)
	}
	for _, id := range []string{"", "../other", "frz/other", ".", ".."} {
		if _, err := client.GetFreeze(context.Background(), scope, id); err == nil {
			t.Fatalf("unsafe freeze ID accepted: %q", id)
		}
	}
}

func TestHTTPCreditClientReleaseFreezeUsesStableProtectedWireContract(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	key, err := BuildIdempotencyKey(IdempotencyActionReleaseFreeze, scope, "task_01")
	if err != nil {
		t.Fatal(err)
	}
	request := ReleaseFreezeRequest{Scope: scope, TaskID: "task_01", IdempotencyKey: key}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/credit-freezes/release" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-current" ||
			r.Header.Get("X-Request-Id") != "req-credit" ||
			r.Header.Get("X-Idempotency-Key") != key {
			t.Fatalf("protected headers = %#v", r.Header)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["task_id"] != request.TaskID || len(body) != 1 {
			t.Fatalf("body = %#v", body)
		}
		writeCreditReleaseSuccess(w, "frz_01", request.TaskID)
	}))
	defer server.Close()

	client := newTestHTTPCreditClient(t, server, scope, nil)
	releaser, ok := client.(CloudCreditFreezeReleaser)
	if !ok {
		t.Fatal("HTTP credit client does not implement freeze release")
	}
	for range 2 {
		result, err := releaser.ReleaseFreeze(context.Background(), request)
		if err != nil || result.Freeze.Status != FreezeReleased || result.Balance.FrozenBalance != "0.0000" {
			t.Fatalf("ReleaseFreeze = %#v, %v", result, err)
		}
	}
	if calls != 2 {
		t.Fatalf("release calls = %d, want 2 idempotent attempts", calls)
	}
}

func TestHTTPCreditClientReleaseFreezePreservesConflictCodes(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	for _, code := range []ErrorCode{ErrorTaskNotCancellable, ErrorIdempotencyConflict} {
		t.Run(string(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusConflict)
				_, _ = fmt.Fprintf(w, `{"data":null,"error":{"code":%q,"message":"expected","retryable":false},"request_id":"req-release-conflict","server_time":"2026-07-15T04:00:00Z"}`, code)
			}))
			defer server.Close()
			client := newTestHTTPCreditClient(t, server, scope, nil)
			releaser := client.(CloudCreditFreezeReleaser)
			_, err := releaser.ReleaseFreeze(context.Background(), ReleaseFreezeRequest{
				Scope: scope, TaskID: "task_01", IdempotencyKey: "release-task-01",
			})
			var cloudErr *Error
			if !errors.As(err, &cloudErr) || cloudErr.Code != code ||
				cloudErr.Retryable || cloudErr.RequestID != "req-release-conflict" {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestHTTPCreditClientPreservesCloudErrorsAndRejectsMalformedResponses(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	for _, code := range []ErrorCode{ErrorAuthRequired, ErrorInsufficientBalance, ErrorModelPricing, ErrorFreezeNotFound, ErrorFreezeExpired} {
		t.Run(string(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusConflict)
				_, _ = fmt.Fprintf(w, `{"data":null,"error":{"code":%q,"message":"expected"},"request_id":"req-error","server_time":"2026-07-15T04:00:00Z"}`, code)
			}))
			defer server.Close()
			_, err := newTestHTTPCreditClient(t, server, scope, nil).CreateFreeze(context.Background(), FreezeRequest{Scope: scope, TaskID: "task_01", ModelID: "reasonix-mock-chat", ClientTaskType: "chat", ClientEstimatedCost: "0.0000", IdempotencyKey: "idem_credit_01"})
			var cloudErr *Error
			if !errors.As(err, &cloudErr) || cloudErr.Code != code || cloudErr.RequestID != "req-error" {
				t.Fatalf("error = %#v", err)
			}
		})
	}
	for _, body := range []string{`not-json`, `{"data":{"freeze":{"freeze_id":"","task_id":"task_01","status":"active","freeze_amount":"1","free_frozen_amount":"1","subscription_frozen_amount":"0","expires_at":"2026-07-15T05:00:00Z"},"balance":{"version":0}},"error":null,"request_id":"req-bad","server_time":"2026-07-15T04:00:00Z"}`} {
		t.Run("protocol", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			_, err := newTestHTTPCreditClient(t, server, scope, nil).GetFreeze(context.Background(), scope, "frz_01")
			if !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestHTTPCreditClientMapsNetworkFailureAndRequiresSession(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := newTestHTTPCreditClient(t, server, scope, nil)
	server.Close()
	if _, err := client.GetFreeze(context.Background(), scope, "frz_01"); !isCloudCode(err, ErrorNetworkUnavailable) {
		t.Fatalf("network error = %v", err)
	}
	transport, err := NewHTTPSessionClient("http://127.0.0.1:1", http.DefaultClient, DeviceIdentity{DeviceID: "device-01", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewAuthenticatedRequestExecutor(NewMemorySessionStore(), &creditSessionClient{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := NewHTTPCreditClient(executor, transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missing.GetFreeze(context.Background(), scope, "frz_01"); !isCloudCode(err, ErrorAuthRequired) {
		t.Fatalf("missing session error = %v", err)
	}
}

func newTestHTTPCreditClient(t *testing.T, server *httptest.Server, scope BindingScope, sessionClient CloudSessionClient) CloudCreditClient {
	t.Helper()
	if sessionClient == nil {
		sessionClient = &creditSessionClient{}
	}
	store := NewMemorySessionStore()
	now := time.Date(2026, 7, 15, 4, 0, 0, 0, time.UTC)
	if err := store.Save(context.Background(), Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access-current", RefreshToken: "refresh-current", DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	executor, err := NewAuthenticatedRequestExecutor(store, sessionClient, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-01", ClientVersion: "dev"}, func() string { return "req-credit" })
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPCreditClient(executor, transport)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func writeCreditSuccess(w http.ResponseWriter, freezeID, taskID string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"data":{"freeze":{"freeze_id":%q,"task_id":%q,"status":"active","freeze_amount":"300.0000","free_frozen_amount":"300.0000","subscription_frozen_amount":"0.0000","low_balance_mode":false,"expires_at":"2026-07-15T05:00:00Z","version":1},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"700.0000","free_available_balance":"700.0000","subscription_available_balance":"0.0000","frozen_balance":"300.0000","free_frozen_balance":"300.0000","subscription_frozen_balance":"0.0000","version":2}},"error":null,"request_id":"req-credit","server_time":"2026-07-15T04:00:00Z"}`, freezeID, taskID)
}

func writeCreditReleaseSuccess(w http.ResponseWriter, freezeID, taskID string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"data":{"freeze":{"freeze_id":%q,"task_id":%q,"status":"released","freeze_amount":"300.0000","free_frozen_amount":"300.0000","subscription_frozen_amount":"0.0000","low_balance_mode":false,"expires_at":"2026-07-15T05:00:00Z","version":2},"balance":{"total_balance":"1000.0000","free_balance":"1000.0000","subscription_balance":"0.0000","available_balance":"1000.0000","free_available_balance":"1000.0000","subscription_available_balance":"0.0000","frozen_balance":"0.0000","free_frozen_balance":"0.0000","subscription_frozen_balance":"0.0000","version":3}},"error":null,"request_id":"req-credit","server_time":"2026-07-15T04:00:00Z"}`, freezeID, taskID)
}

type creditSessionClient struct {
	mu     sync.Mutex
	remote RemoteSession
	calls  int
}

func (*creditSessionClient) Login(context.Context, LoginRequest) (AuthenticationResult, error) {
	return AuthenticationResult{}, nil
}
func (c *creditSessionClient) Refresh(context.Context, string) (RemoteSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.remote, nil
}
func (*creditSessionClient) Logout(context.Context, string) error { return nil }
func (*creditSessionClient) CurrentUser(context.Context, string) (CurrentUserSnapshot, error) {
	return CurrentUserSnapshot{}, nil
}
func creditRemoteSession() RemoteSession {
	return RemoteSession{AccessToken: "access-rotated", RefreshToken: "refresh-rotated", DeviceID: "device-01", AccessTokenExpiresAt: time.Date(2026, 7, 15, 5, 0, 0, 0, time.UTC), RefreshTokenExpiresAt: time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC)}
}
