package cloudquota

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

func TestUploadPartialQuotaUsesAuthenticatedStrictWire(t *testing.T) {
	report := testPartialReport(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != partialPath {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		assertProtectedHeaders(t, r, report.IdempotencyKey)
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 6 || body["calls"] == nil || body["partial_sequence"] == nil {
			t.Fatalf("body fields = %#v", body)
		}
		var calls []map[string]json.RawMessage
		if err := json.Unmarshal(body["calls"], &calls); err != nil {
			t.Fatal(err)
		}
		if len(calls) != 1 || calls[0]["billing_quota_raw"] == nil || calls[0]["usage_quota_raw"] != nil {
			t.Fatalf("calls = %#v", calls)
		}
		if got := strings.TrimSpace(string(calls[0]["evidence_issues"])); got != "[]" {
			t.Fatalf("evidence_issues = %s, want []", got)
		}
		writeSuccess(t, w, partialReceiptWire{
			UsageReportID: "urp_01", TaskID: report.TaskID, ReportType: "partial", Status: "validated",
			PartialSequence: int64(report.Sequence), AcceptedCallCount: 1,
			PayloadFingerprint: strings.Repeat("a", 64),
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	if err := client.UploadPartialQuota(context.Background(), report.Scope, report); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeQuotaMapsAuthoritativeSettlement(t *testing.T) {
	report := testFinalReport(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != finalPath {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		assertProtectedHeaders(t, r, report.IdempotencyKey)
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 4 || body["task_id"] == nil || body["freeze_id"] == nil ||
			body["client_summary"] == nil || body["calls"] == nil || body["contract_version"] != nil {
			t.Fatalf("body fields = %#v", body)
		}
		var calls []map[string]json.RawMessage
		if err := json.Unmarshal(body["calls"], &calls); err != nil {
			t.Fatal(err)
		}
		if len(calls) != 1 || calls[0]["usage_quota_raw"] != nil ||
			strings.TrimSpace(string(calls[0]["evidence_issues"])) != "[]" {
			t.Fatalf("calls = %#v", calls)
		}
		writeSuccess(t, w, terminalSettlementWire(report.TaskID))
	}))
	defer server.Close()

	result, err := newTestClient(t, server).FinalizeQuota(context.Background(), report.Scope, report)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settlement.QuotaPolicyVersion == nil || *result.Settlement.QuotaPolicyVersion != "maas-quota-config-v1" ||
		result.Settlement.QuotaPerUnit == nil || *result.Settlement.QuotaPerUnit != "500000" ||
		result.Settlement.ChargedCredits == nil || *result.Settlement.ChargedCredits != "1.4600" ||
		result.Balance == nil || result.Balance.Version != 14 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSettlementWireAcceptsOnlyConsistentLegacyAmountAliases(t *testing.T) {
	wire := terminalSettlementWire("task_01")
	charged, chargedFree, chargedSubscription := "1.4600", "1.4600", "0.0000"
	releasedFree, releasedSubscription := "298.5400", "0.0000"
	wire.Settlement.ChargedAmount = &charged
	wire.Settlement.ChargedFreeAmount = &chargedFree
	wire.Settlement.ChargedSubscriptionAmount = &chargedSubscription
	wire.Settlement.ReleasedFreeAmount = &releasedFree
	wire.Settlement.ReleasedSubscriptionAmount = &releasedSubscription
	if _, err := wire.domain("task_01"); err != nil {
		t.Fatalf("consistent aliases: %v", err)
	}
	wrong := "1.0000"
	wire.Settlement.ChargedAmount = &wrong
	if _, err := wire.domain("task_01"); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
		t.Fatalf("inconsistent alias error = %v", err)
	}
}

func TestSettlementWirePreservesManualFailureWithoutInventingPolicyFacts(t *testing.T) {
	code := "SETTLEMENT_MANUAL_REQUIRED"
	wire := settlementResultWire{Settlement: settlementWire{
		SettlementID: "set_manual", TaskID: "task_01", Status: "failed_manual_required", ErrorCode: &code,
	}}
	result, err := wire.domain("task_01")
	if err != nil {
		t.Fatal(err)
	}
	if result.Settlement.QuotaPolicyVersion != nil || result.Settlement.ErrorCode == nil ||
		*result.Settlement.ErrorCode != code {
		t.Fatalf("manual settlement = %#v", result.Settlement)
	}
}

func TestGetQuotaSettlementUsesSafePathAndRejectsUnknownResponseFields(t *testing.T) {
	scope := testScope()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/task-settlements/task_01" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		assertProtectedHeaders(t, r, "")
		_, _ = w.Write([]byte(`{"data":{"settlement":{"settlement_id":"set_01","task_id":"task_01","status":"settlement_pending"},"balance":null,"unknown":true},"error":null,"request_id":"req-cloud","server_time":"2026-07-16T00:00:00Z"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.GetQuotaSettlement(context.Background(), scope, "task_01")
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
		t.Fatalf("unknown response error = %v", err)
	}
	for _, taskID := range []string{"", "../task", "task/other", ".", ".."} {
		if _, err := client.GetQuotaSettlement(context.Background(), scope, taskID); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
			t.Fatalf("unsafe task ID %q error = %v", taskID, err)
		}
	}
}

func TestQuotaClientMapsCloudErrorsAndPreservesRemoteCause(t *testing.T) {
	report := testFinalReport(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"data":null,"error":{"code":"QUOTA_SUMMARY_MISMATCH","message":"mismatch","retryable":false},"request_id":"req-error","server_time":"2026-07-16T00:00:00Z"}`))
	}))
	defer server.Close()

	_, err := newTestClient(t, server).FinalizeQuota(context.Background(), report.Scope, report)
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorQuotaSummaryMismatch {
		t.Fatalf("mapped error = %v", err)
	}
	var remote *cloud.Error
	if !errors.As(err, &remote) || remote.Code != "QUOTA_SUMMARY_MISMATCH" || remote.RequestID != "req-error" {
		t.Fatalf("remote cause = %#v", remote)
	}
}

func TestQuotaClientRefreshesSessionOnceAndPreservesCancellation(t *testing.T) {
	report := testPartialReport(t)
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			if r.Header.Get("Authorization") != "Bearer access-current" {
				t.Fatalf("first authorization = %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null,"error":{"code":"TOKEN_EXPIRED","message":"expired","retryable":false},"request_id":"req-first","server_time":"2026-07-16T00:00:00Z"}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer access-rotated" {
			t.Fatalf("second authorization = %q", r.Header.Get("Authorization"))
		}
		writeSuccess(t, w, partialReceiptWire{
			UsageReportID: "urp_01", TaskID: report.TaskID, ReportType: "partial", Status: "validated",
			PartialSequence: 1, AcceptedCallCount: 1, PayloadFingerprint: strings.Repeat("b", 64),
		})
	}))
	defer server.Close()

	refresher := &testSessionClient{remote: testRemoteSession()}
	client := newTestClientWithSession(t, server, refresher)
	if err := client.UploadPartialQuota(context.Background(), report.Scope, report); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || refresher.calls != 1 {
		t.Fatalf("calls/refresh = %d/%d", calls, refresher.calls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.UploadPartialQuota(ctx, report.Scope, report); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
}

func TestQuotaClientRejectsRemovedUsageQuotaBeforeHTTP(t *testing.T) {
	report := testPartialReport(t)
	report.Calls[0].UsageQuotaRaw = report.Calls[0].BillingQuotaRaw
	source := modelbilling.QuotaSourceBothConsistent
	report.Calls[0].QuotaSource = &source
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()

	err := newTestClient(t, server).UploadPartialQuota(context.Background(), report.Scope, report)
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorUsageRejected || calls != 0 {
		t.Fatalf("error/calls = %v/%d", err, calls)
	}
}

func testPartialReport(t *testing.T) modelbilling.PartialQuotaReport {
	t.Helper()
	scope := testScope()
	key, err := modelbilling.BuildPartialIdempotencyKey(scope, "task_01", "frz_01", 1)
	if err != nil {
		t.Fatal(err)
	}
	return modelbilling.PartialQuotaReport{
		ContractVersion: modelbilling.QuotaContractV1, TaskID: "task_01", FreezeID: "frz_01",
		Sequence: 1, ReportMode: modelbilling.ReportModeCumulative,
		CapturedAt: time.Date(2026, 7, 16, 0, 1, 0, 0, time.UTC), Calls: []modelbilling.QuotaReportCall{testQuotaCall()},
		Scope: scope, IdempotencyKey: key,
	}
}

func testFinalReport(t *testing.T) modelbilling.FinalQuotaReport {
	t.Helper()
	scope := testScope()
	key, err := modelbilling.BuildFinalIdempotencyKey(scope, "task_01", "frz_01")
	if err != nil {
		t.Fatal(err)
	}
	return modelbilling.FinalQuotaReport{
		ContractVersion: modelbilling.QuotaContractV1, TaskID: "task_01", FreezeID: "frz_01",
		ClientSummary: modelbilling.QuotaTaskSummary{
			TaskTotalQuota: "50", ModelCallCount: 1,
			StartedAt:  time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
			FinishedAt: time.Date(2026, 7, 16, 0, 2, 0, 0, time.UTC),
		},
		Calls: []modelbilling.QuotaReportCall{testQuotaCall()}, Scope: scope, IdempotencyKey: key,
		TaskResult: modelbilling.TaskResultCompleted, CreatedAt: time.Date(2026, 7, 16, 0, 2, 0, 0, time.UTC),
	}
}

func testQuotaCall() modelbilling.QuotaReportCall {
	requestID := "oneapi-req-01"
	quota := modelbilling.DecimalString("50")
	source := modelbilling.QuotaSourceBilling
	return modelbilling.QuotaReportCall{
		CallID: "call_01", Endpoint: modelbilling.MaasEndpointChatCompletions, Model: "gpt-5.4-mini",
		OneAPIRequestID: &requestID,
		BillingQuotaRaw: modelbilling.QuotaRawValue{Present: true, Type: modelbilling.QuotaJSONNumber, Value: json.RawMessage("50")},
		Quota:           &quota, QuotaSource: &source, EvidenceStatus: modelbilling.QuotaEvidenceComplete,
		EvidenceIssues: []modelbilling.QuotaEvidenceIssue{}, EvidenceRevision: 1,
		OccurredAt: time.Date(2026, 7, 16, 0, 1, 0, 0, time.UTC),
	}
}

func testScope() cloud.BindingScope {
	return cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1}
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	return newTestClientWithSession(t, server, &testSessionClient{})
}

func newTestClientWithSession(t *testing.T, server *httptest.Server, sessionClient cloud.CloudSessionClient) *Client {
	t.Helper()
	scope := testScope()
	store := cloud.NewMemorySessionStore()
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	if err := store.Save(context.Background(), cloud.Session{
		Scope: scope, State: cloud.SessionAuthenticated, AccessToken: "access-current", RefreshToken: "refresh-current",
		DeviceID: "device-01", AccessTokenExpiresAt: now.Add(time.Hour), RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	executor, err := cloud.NewAuthenticatedRequestExecutor(store, sessionClient, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	transport, err := cloud.NewHTTPSessionClient(server.URL, server.Client(), cloud.DeviceIdentity{
		DeviceID: "device-01", ClientVersion: "0.9.0",
	}, func() string { return "req-quota" })
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(executor, transport)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func assertProtectedHeaders(t *testing.T, request *http.Request, key string) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer access-current" && request.Header.Get("Authorization") != "Bearer access-rotated" {
		t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
	}
	if request.Header.Get("X-Device-Id") != "device-01" || request.Header.Get("X-Client-Version") != "0.9.0" ||
		request.Header.Get("X-Request-Id") != "req-quota" || request.Header.Get("X-Idempotency-Key") != key {
		t.Fatalf("headers = %#v", request.Header)
	}
}

func writeSuccess(t *testing.T, response http.ResponseWriter, data any) {
	t.Helper()
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(map[string]any{
		"data": data, "error": nil, "request_id": "req-cloud", "server_time": "2026-07-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
}

func terminalSettlementWire(taskID string) settlementResultWire {
	str := func(value string) *string { return &value }
	version := int64(14)
	return settlementResultWire{
		Settlement: settlementWire{
			SettlementID: "set_01", TaskID: taskID, Status: "settled", TaskTotalQuota: str("50"),
			QuotaPolicyVersion: str("maas-quota-config-v1"), QuotaPerUnit: str("500000"),
			USDExchangeRate: str("7.3"), CreditPerCNY: str("2000"), ChargedCredits: str("1.4600"),
			ConsumedFrozenAmount: str("1.4600"), ConsumedFrozenFreeAmount: str("1.4600"),
			ConsumedFrozenSubscriptionAmount: str("0.0000"), AdditionalChargedAmount: str("0.0000"),
			AdditionalChargedFreeAmount: str("0.0000"), AdditionalChargedSubscriptionAmount: str("0.0000"),
			ReleasedFrozenAmount: str("298.5400"), ReleasedFrozenFreeAmount: str("298.5400"),
			ReleasedFrozenSubscriptionAmount: str("0.0000"), UncoveredAmount: str("0.0000"),
			Balance: str("998.5400"), BalanceVersion: &version, LedgerEntries: []ledgerEntryWire{},
		},
		Balance: &cloud.BalanceSnapshot{
			TotalBalance: "998.5400", FreeBalance: "998.5400", SubscriptionBalance: "0.0000",
			AvailableBalance: "998.5400", FreeAvailableBalance: "998.5400", SubscriptionAvailableBalance: "0.0000",
			FrozenBalance: "0.0000", FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000", Version: 14,
		},
	}
}

type testSessionClient struct {
	mu     sync.Mutex
	remote cloud.RemoteSession
	calls  int
}

func (*testSessionClient) Login(context.Context, cloud.LoginRequest) (cloud.AuthenticationResult, error) {
	return cloud.AuthenticationResult{}, nil
}

func (c *testSessionClient) Refresh(context.Context, string) (cloud.RemoteSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.remote, nil
}

func (*testSessionClient) Logout(context.Context, string) error { return nil }

func (*testSessionClient) CurrentUser(context.Context, string) (cloud.CurrentUserSnapshot, error) {
	return cloud.CurrentUserSnapshot{}, nil
}

func testRemoteSession() cloud.RemoteSession {
	return cloud.RemoteSession{
		AccessToken: "access-rotated", RefreshToken: "refresh-rotated", DeviceID: "device-01",
		AccessTokenExpiresAt:  time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC),
		RefreshTokenExpiresAt: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
	}
}
