package cloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPMaaSKeyClientUsesAuthenticatedCurrentUserContract(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != currentMaaSKeyPath || r.URL.RawQuery != "" {
			t.Fatalf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer access-current" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data":{"status":"ready","key_name":"usr_remote","key_id":9,"maas_key":"sk-user-key"},"error":null,"request_id":"req-maas","server_time":"2026-07-15T04:00:00Z"}`))
	}))
	defer server.Close()

	result, err := newTestMaaSKeyClient(t, server, scope).Get(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != MaaSKeyReady || result.KeyName != scope.CloudUserID || result.KeyID == nil || *result.KeyID != 9 || result.Key != "sk-user-key" {
		t.Fatalf("result = %#v", result)
	}
}

func TestHTTPMaaSKeyClientAcceptsNonReadySnapshotsWithoutCredential(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	for _, status := range []MaaSKeyStatus{MaaSKeyCreatePending, MaaSKeyCreating, MaaSKeyRetryWait, MaaSKeyManual} {
		t.Run(string(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"data":{"status":%q,"key_name":"usr_remote","key_id":null,"maas_key":null},"error":null,"request_id":"req-maas","server_time":"2026-07-15T04:00:00Z"}`, status)
			}))
			defer server.Close()
			result, err := newTestMaaSKeyClient(t, server, scope).Get(context.Background(), scope)
			if err != nil || result.Status != status || result.Key != "" {
				t.Fatalf("result/error = %#v / %v", result, err)
			}
		})
	}
}

func TestHTTPMaaSKeyClientRejectsWrongIdentityAndMalformedReadyKeyWithoutLeakingKey(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	for _, body := range []string{
		`{"data":{"status":"ready","key_name":"usr_other","key_id":9,"maas_key":"sk-sensitive-value"},"error":null,"request_id":"req-maas","server_time":"2026-07-15T04:00:00Z"}`,
		`{"data":{"status":"ready","key_name":"usr_remote","key_id":null,"maas_key":"sk-sensitive-value"},"error":null,"request_id":"req-maas","server_time":"2026-07-15T04:00:00Z"}`,
		`{"data":{"status":"creating","key_name":"usr_remote","key_id":null,"maas_key":"sk-sensitive-value"},"error":null,"request_id":"req-maas","server_time":"2026-07-15T04:00:00Z"}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		_, err := newTestMaaSKeyClient(t, server, scope).Get(context.Background(), scope)
		server.Close()
		if err == nil || !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
			t.Fatalf("error = %v", err)
		}
		if strings.Contains(err.Error(), "sk-sensitive-value") {
			t.Fatalf("error leaked MaaS key: %v", err)
		}
	}
}

func TestHTTPMaaSRechargeKeyBalanceUsesCurrentUserContract(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_recharge", BindingGeneration: 2}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != maasRechargeKeyBalancePath || r.URL.RawQuery != "" {
			t.Fatalf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer access-v3" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data":{"key_name":"usr_rechargecz","available_balance":"14600.0000","remain_quota":500000,"balance_cny":"7.3000","source":"maas_recharge_api_key","conversion_rule_version":"maas-quota-credit-v1","quota_per_usd":"500000","cny_per_usd":"7.3","credit_per_cny":"2000","queried_at":"2026-08-22T10:00:00Z"},"error":null,"request_id":"req-recharge-balance","server_time":"2026-08-22T10:00:00Z"}`))
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-v3", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPMaaSRechargeKeyBalanceClient(paymentV3TestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := client.Get(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if balance.KeyName != "usr_rechargecz" || balance.AvailableBalance != "14600.0000" || balance.RemainQuota != 500000 || balance.BalanceCNY != "7.3000" || balance.NotFound {
		t.Fatalf("balance = %#v", balance)
	}
}

func TestHTTPMaaSKeyBalanceUsesCurrentUserContract(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_subscription", BindingGeneration: 3}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != maasKeyBalancePath || r.ContentLength != 0 {
			t.Fatalf("request = %s %s content_length=%d", r.Method, r.URL.Path, r.ContentLength)
		}
		_, _ = w.Write([]byte(`{"data":{"available_balance":"146000.0000","remain_quota":5000000,"source":"maas_api_key","conversion_rule_version":"maas-quota-credit-v1","quota_per_usd":"500000","cny_per_usd":"7.3","credit_per_cny":"2000","queried_at":"2026-08-22T10:00:00Z"},"error":null,"request_id":"req-subscription-balance","server_time":"2026-08-22T10:00:00Z"}`))
	}))
	defer server.Close()

	transport := newTestBalanceTransport(t, server)
	client, err := NewHTTPMaaSKeyBalanceClient(paymentV3TestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := client.Get(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if balance.AvailableBalance != "146000.0000" || balance.Source != maasKeyBalanceSource || balance.CreditPerCNY != "2000" {
		t.Fatalf("balance = %#v", balance)
	}
}

func TestHTTPMaaSRechargeKeyBalanceNormalizesOnlyExplicitNotFound(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_no_recharge", BindingGeneration: 4}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"NOT_FOUND","message":"recharge key not found","retryable":false},"request_id":"req-no-recharge","server_time":"2026-08-22T10:00:00Z"}`))
	}))
	defer server.Close()
	client, err := NewHTTPMaaSRechargeKeyBalanceClient(paymentV3TestExecutor{}, newTestBalanceTransport(t, server))
	if err != nil {
		t.Fatal(err)
	}
	balance, err := client.Get(context.Background(), scope)
	if err != nil || !balance.NotFound || balance.AvailableBalance != "0.0000" {
		t.Fatalf("balance/error = %#v / %v", balance, err)
	}
}

func TestHTTPMaaSRechargeKeyBalancePreservesRealErrors(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_errors", BindingGeneration: 5}
	tests := []struct {
		name       string
		statusCode int
		code       ErrorCode
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, code: ErrorAuthRequired},
		{name: "forbidden", statusCode: http.StatusForbidden, code: ErrorCode("FORBIDDEN")},
		{name: "server", statusCode: http.StatusInternalServerError, code: ErrorCode("INTERNAL_ERROR")},
		{name: "not found code on success status", statusCode: http.StatusOK, code: ErrorCode("NOT_FOUND")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
				_, _ = fmt.Fprintf(w, `{"data":null,"error":{"code":%q,"message":"safe failure","retryable":false},"request_id":"req-real-error","server_time":"2026-08-22T10:00:00Z"}`, test.code)
			}))
			defer server.Close()
			client, err := NewHTTPMaaSRechargeKeyBalanceClient(paymentV3TestExecutor{}, newTestBalanceTransport(t, server))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Get(context.Background(), scope)
			if !isCloudCode(err, test.code) {
				t.Fatalf("error = %v, want %s", err, test.code)
			}
			var typed *Error
			if !errors.As(err, &typed) || typed.HTTPStatus != test.statusCode || typed.RequestID != "req-real-error" {
				t.Fatalf("typed error = %#v", err)
			}
		})
	}
}

func TestHTTPMaaSRechargeKeyBalancePreservesNetworkFailure(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_network", BindingGeneration: 6}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	transport := newTestBalanceTransport(t, server)
	server.Close()
	client, err := NewHTTPMaaSRechargeKeyBalanceClient(paymentV3TestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Get(context.Background(), scope)
	if !isCloudCode(err, ErrorNetworkUnavailable) {
		t.Fatalf("error = %v, want %s", err, ErrorNetworkUnavailable)
	}
}

func TestMaaSRechargeKeyBalanceRejectsMalformedResponse(t *testing.T) {
	valid := MaaSRechargeKeyBalance{
		KeyName: "usr_rechargecz", AvailableBalance: "14600.0000", RemainQuota: 500000, BalanceCNY: "7.3000",
		Source: maasRechargeKeyBalanceSource, ConversionRuleVersion: "maas-quota-credit-v1", QuotaPerUSD: "500000",
		CNYPerUSD: "7.3", CreditPerCNY: "2000", QueriedAt: time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC),
	}
	tests := []MaaSRechargeKeyBalance{
		func() MaaSRechargeKeyBalance { value := valid; value.AvailableBalance = "-1.0000"; return value }(),
		func() MaaSRechargeKeyBalance { value := valid; value.AvailableBalance = "invalid"; return value }(),
		func() MaaSRechargeKeyBalance { value := valid; value.RemainQuota = -1; return value }(),
		func() MaaSRechargeKeyBalance { value := valid; value.CNYPerUSD = "0"; return value }(),
		func() MaaSRechargeKeyBalance { value := valid; value.CNYPerUSD = " 7.3"; return value }(),
		func() MaaSRechargeKeyBalance { value := valid; value.Source = maasKeyBalanceSource; return value }(),
	}
	for _, invalid := range tests {
		if err := invalid.Validate(); err == nil || !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
			t.Fatalf("invalid recharge balance %#v error = %v", invalid, err)
		}
	}
}

func newTestBalanceTransport(t *testing.T, server *httptest.Server) *HTTPSessionClient {
	t.Helper()
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-balance", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return transport
}

func newTestMaaSKeyClient(t *testing.T, server *httptest.Server, scope BindingScope) MaaSKeyClient {
	t.Helper()
	credit := newTestHTTPCreditClient(t, server, scope, nil)
	httpCredit := credit.(*httpCreditClient)
	client, err := NewHTTPMaaSKeyClient(httpCredit.executor, httpCredit.transport)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
