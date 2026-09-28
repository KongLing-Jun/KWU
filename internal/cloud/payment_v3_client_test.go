package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type paymentV3TestExecutor struct{}

func (paymentV3TestExecutor) Do(ctx context.Context, scope BindingScope, call func(context.Context, string) error) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	return call(ctx, "access-v3")
}

func TestHTTPPaymentV3ClientCreatesAndQueriesAuthoritativeOrder(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-v3", BindingGeneration: 2}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer access-v3" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == paymentV3OrdersPath:
			if r.Header.Get("X-Idempotency-Key") != "purchase-attempt-1" {
				t.Fatalf("idempotency key = %q", r.Header.Get("X-Idempotency-Key"))
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 2 || body["plan_id"] != "plan-plus-v1" || body["payment_method"] != "wechat" {
				t.Fatalf("body = %#v", body)
			}
			_, _ = w.Write([]byte(paymentV3Envelope(`"status":"payment_pending","paid":false,"granted":false,"version":2`)))
		case r.Method == http.MethodGet && r.URL.Path == paymentV3OrdersPath+"/payv3-01":
			if r.Header.Get("X-Idempotency-Key") != "" {
				t.Fatalf("query idempotency key = %q", r.Header.Get("X-Idempotency-Key"))
			}
			_, _ = w.Write([]byte(paymentV3Envelope(`"status":"granted","paid":true,"granted":true,"version":6`)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-v3", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := NewHTTPPaymentV3Client(paymentV3TestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	created, err := raw.CreateSubscriptionOrder(context.Background(), CreateSubscriptionOrderRequest{
		Scope: scope, PlanID: "plan-plus-v1", PaymentChannel: PaymentChannelWeChat, IdempotencyKey: "purchase-attempt-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Order.PlanID != "plan-plus-v1" || created.Order.Plan != "plus" || created.Order.AmountUnit != "" || created.Entry.QRCodeURL == "" || created.Order.Status != PaymentPending {
		t.Fatalf("created = %#v", created)
	}
	queriedResult, err := raw.(*httpPaymentV3Client).getPaymentOrderResultFor(context.Background(), scope, created.Order)
	if err != nil {
		t.Fatal(err)
	}
	queried := queriedResult.Order
	if queried.Status != PaymentGranted || queried.GrantStatus != GrantGranted || queried.Version != 6 || requests != 2 {
		t.Fatalf("queried = %#v requests=%d", queried, requests)
	}
}

func TestHTTPPaymentV3ClientCreatesUpgradeAndQueriesCommonOrder(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-upgrade", BindingGeneration: 3}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer access-v3" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == subscriptionUpgradeOrdersPath:
			if r.Header.Get("X-Idempotency-Key") != "upgrade-attempt-1" {
				t.Fatalf("idempotency key = %q", r.Header.Get("X-Idempotency-Key"))
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 2 || body["target_plan"] != "plus" || body["payment_channel"] != "alipay" {
				t.Fatalf("body = %#v", body)
			}
			_, _ = w.Write([]byte(paymentOrderEnvelope("subscription_upgrade", "plus", "lite", "payment_pending", "not_granted", 2)))
		case r.Method == http.MethodPost && r.URL.Path == paymentOrdersPath+"/pay-common-01/verify":
			_, _ = w.Write([]byte(paymentOrderEnvelope("subscription_upgrade", "plus", "lite", "granted", "granted", 7)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-upgrade", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := NewHTTPPaymentV3Client(paymentV3TestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	created, err := raw.CreateUpgradeOrder(context.Background(), CreateUpgradeOrderRequest{
		Scope: scope, TargetPlan: "plus", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "upgrade-attempt-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Order.OrderType != PaymentOrderSubscriptionUpgrade || created.Order.FromPlan != "lite" ||
		created.Order.Plan != "plus" || created.Order.Status != PaymentPending || created.Entry.QRCode == "" {
		t.Fatalf("created = %#v", created)
	}
	queriedResult, err := raw.(*httpPaymentV3Client).getPaymentOrderResultFor(context.Background(), scope, created.Order)
	if err != nil {
		t.Fatal(err)
	}
	queried := queriedResult.Order
	if queried.OrderType != PaymentOrderSubscriptionUpgrade || queried.FromPlan != "lite" ||
		queried.Status != PaymentGranted || queried.GrantStatus != GrantGranted || queried.Version != 7 || requests != 2 {
		t.Fatalf("queried = %#v requests=%d", queried, requests)
	}
}

func TestNormalizePaymentV3StatusSupportsCloudOrderStates(t *testing.T) {
	tests := []struct {
		wireStatus string
		paid       bool
		granted    bool
		want       PaymentStatus
	}{
		{wireStatus: "creating", want: PaymentCreated},
		{wireStatus: "payment_pending", want: PaymentPending},
		{wireStatus: "polling", want: PaymentPending},
		{wireStatus: "paid_verified", paid: true, want: PaymentPaidVerified},
		{wireStatus: "granting", paid: true, want: PaymentGranting},
		{wireStatus: "granted", paid: true, granted: true, want: PaymentGranted},
		{wireStatus: "create_failed", want: PaymentFailed},
		{wireStatus: "payment_failed", want: PaymentFailed},
		{wireStatus: "timeout", want: PaymentTimeout},
		{wireStatus: "manual_required", want: PaymentVerifyFailedManualRequired},
		{wireStatus: "manual_required", paid: true, want: PaymentVerifyFailedManualRequired},
	}
	for _, test := range tests {
		t.Run(test.wireStatus, func(t *testing.T) {
			got, err := normalizePaymentV3Status(paymentV3Order{
				Status: test.wireStatus, Paid: test.paid, Granted: test.granted, Version: 1,
			})
			if err != nil || got != test.want {
				t.Fatalf("normalizePaymentV3Status() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestNormalizePaymentV3StatusRejectsUnknownAndConflictingFacts(t *testing.T) {
	tests := []paymentV3Order{
		{Status: "unknown", Version: 1},
		{Status: "granted", Paid: true, Version: 1},
		{Status: "payment_pending", Paid: true, Version: 1},
		{Status: "granting", Version: 1},
		{Status: "paid_verified", Paid: true, Granted: true, Version: 1},
	}
	for _, wire := range tests {
		if _, err := normalizePaymentV3Status(wire); err == nil || !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
			t.Fatalf("normalizePaymentV3Status(%#v) error = %v, want CLOUD_PROTOCOL_ERROR", wire, err)
		}
	}
}

func TestNormalizePaymentOrderStatusHandlesVerificationAndGrantFailures(t *testing.T) {
	for _, test := range []struct {
		status      string
		grantStatus string
		want        PaymentStatus
	}{
		{status: "verify_failed_manual_required", grantStatus: "not_granted", want: PaymentVerifyFailedManualRequired},
		{status: "paid_verified", grantStatus: "failed", want: PaymentVerifyFailedManualRequired},
		{status: "granting", grantStatus: "granting", want: PaymentGranting},
	} {
		got, err := normalizePaymentOrderStatus(test.status, test.grantStatus)
		if err != nil || got != test.want {
			t.Fatalf("normalizePaymentOrderStatus(%q, %q) = %q, %v; want %q", test.status, test.grantStatus, got, err, test.want)
		}
	}
}

func TestNormalizePaymentV3OrderAcceptsEntrylessCreatingAndCreateFailed(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-v3", BindingGeneration: 2}
	for _, test := range []struct {
		wireStatus string
		want       PaymentStatus
	}{
		{wireStatus: "creating", want: PaymentCreated},
		{wireStatus: "create_failed", want: PaymentFailed},
	} {
		t.Run(test.wireStatus, func(t *testing.T) {
			result, err := normalizePaymentV3Order(scope, paymentV3TestOrder(test.wireStatus))
			if err != nil {
				t.Fatal(err)
			}
			if result.Order.Status != test.want || result.Entry != (PaymentEntry{}) {
				t.Fatalf("normalizePaymentV3Order() = %#v, want status %q without entry", result, test.want)
			}
			if err := result.Validate(); err != nil {
				t.Fatalf("normalized result validation failed: %v", err)
			}
		})
	}
}

func TestNormalizePaymentV3OrderRequiresEntryForPayableAndPostCreateStates(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-v3", BindingGeneration: 2}
	for _, test := range []struct {
		wireStatus string
		paid       bool
		granted    bool
	}{
		{wireStatus: "payment_pending"},
		{wireStatus: "polling"},
		{wireStatus: "paid_verified", paid: true},
		{wireStatus: "granting", paid: true},
		{wireStatus: "granted", paid: true, granted: true},
		{wireStatus: "payment_failed"},
		{wireStatus: "timeout"},
		{wireStatus: "manual_required"},
	} {
		t.Run(test.wireStatus, func(t *testing.T) {
			wire := paymentV3TestOrder(test.wireStatus)
			wire.Paid, wire.Granted = test.paid, test.granted
			if _, err := normalizePaymentV3Order(scope, wire); err == nil || !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
				t.Fatalf("normalizePaymentV3Order(%#v) error = %v, want CLOUD_PROTOCOL_ERROR", wire, err)
			}
		})
	}
}

func TestNormalizePaymentV3OrderRejectsEntryForCreatingAndCreateFailed(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-v3", BindingGeneration: 2}
	for _, status := range []string{"creating", "create_failed"} {
		t.Run(status, func(t *testing.T) {
			wire := paymentV3TestOrder(status)
			wire.QRCodeURL = "https://pay.example/qr"
			if _, err := normalizePaymentV3Order(scope, wire); err == nil || !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
				t.Fatalf("normalizePaymentV3Order(%#v) error = %v, want CLOUD_PROTOCOL_ERROR", wire, err)
			}
		})
	}
}

func paymentV3TestOrder(status string) paymentV3Order {
	return paymentV3Order{
		OrderID: "payv3-01", PlanID: "plan-plus-v1", Plan: "plus", PaymentMethod: "alipay",
		Amount: "29.9000", Currency: "CNY", Status: status,
		ExpiresAt: time.Date(2030, 7, 20, 4, 0, 0, 0, time.UTC), Version: 1,
	}
}

func TestNormalizePaymentV3OrderAcceptsObservedGrantedAlipayShape(t *testing.T) {
	var wire paymentV3Order
	if err := json.Unmarshal([]byte(`{
		"order_id":"payv3-observed","plan_id":"plan-lite-v1","plan":"lite",
		"payment_method":"alipay","amount":"1.0000","currency":"CNY",
		"status":"granted","qr_code_url":"https://pay.example/observed",
		"expires_at":"2030-07-22T08:30:00Z","paid":true,"granted":true,
		"paid_at":"2026-07-22T08:00:00Z","granted_at":"2026-07-22T08:00:01Z","version":15
	}`), &wire); err != nil {
		t.Fatal(err)
	}
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-observed", BindingGeneration: 1}
	result, err := normalizePaymentV3Order(scope, wire)
	if err != nil {
		t.Fatal(err)
	}
	if result.Order.Status != PaymentGranted || result.Order.GrantStatus != GrantGranted ||
		result.Order.Plan != "lite" || result.Order.PaymentChannel != PaymentChannelAlipay ||
		result.Order.Amount != "1.00" || result.Order.Version != 15 {
		t.Fatalf("normalized observed order = %#v", result)
	}
}

func paymentV3Envelope(state string) string {
	return `{"data":{"order_id":"payv3-01","plan_id":"plan-plus-v1","plan":"plus","payment_method":"wechat","amount":"29.9000","currency":"CNY",` +
		state + `,"qr_code_url":"https://pay.example/qr","expires_at":"2030-07-20T04:00:00Z"},"error":null,"request_id":"req-v3","server_time":"2026-07-20T03:00:00Z"}`
}

func paymentOrderEnvelope(orderType, plan, fromPlan, status, grantStatus string, version uint64) string {
	fromPlanJSON := "null"
	amount := "0.02"
	if orderType == "subscription_purchase" {
		amount = "29.90"
	}
	if fromPlan != "" {
		encoded, _ := json.Marshal(fromPlan)
		fromPlanJSON = string(encoded)
	}
	return `{"data":{"payment_order":{"order_id":"pay-common-01","order_type":"` + orderType +
		`","plan":"` + plan + `","from_plan":` + fromPlanJSON +
		`,"plan_config_version":"catalog-v1","amount":"` + amount + `","amount_unit":"yuan","currency":"CNY",` +
		`"payment_channel":"alipay","status":"` + status + `","grant_status":"` + grantStatus +
		`","credit_grant_amount":"40000.0000","expires_at":"2030-07-20T04:00:00Z","version":` +
		fmt.Sprint(version) + `},"payment_entry":{"channel":"alipay","entry_type":"qr_code",` +
		`"qr_code":"https://pay.example/alipay","expires_at":"2030-07-20T04:00:00Z"}},` +
		`"error":null,"request_id":"req-payment","server_time":"2026-07-20T03:00:00Z"}`
}

func TestHTTPMaaSRechargeClientCreatesAndQueriesOrder(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-recharge", BindingGeneration: 2}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer access-v3" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == maasRechargeOrdersPath:
			if r.Header.Get("X-Idempotency-Key") != "maas-recharge-attempt-1" {
				t.Fatalf("idempotency key = %q", r.Header.Get("X-Idempotency-Key"))
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 2 || body["amount"] != "100.00" || body["payment_method"] != "alipay" {
				t.Fatalf("body = %#v", body)
			}
			_, _ = w.Write([]byte(maasRechargeEnvelope(`"status":"payment_pending","paid":false,"credited":false,"version":2`)))
		case r.Method == http.MethodGet && r.URL.Path == maasRechargeOrdersPath+"/maasr-01":
			if r.Header.Get("X-Idempotency-Key") != "" {
				t.Fatalf("query idempotency key = %q", r.Header.Get("X-Idempotency-Key"))
			}
			_, _ = w.Write([]byte(maasRechargeEnvelope(`"status":"credited","paid":true,"credited":true,"paid_at":"2026-08-17T12:01:12Z","credited_at":"2026-08-17T12:01:13Z","remote_quota_after":7849316,"version":7`)))
		case strings.HasPrefix(r.URL.Path, maasRechargeOrdersPath):
			t.Fatalf("unexpected MaaS recharge route %s %s; query must use GET %s/{order_id}", r.Method, r.URL.Path, maasRechargeOrdersPath)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-v3", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPMaaSRechargeClient(paymentV3TestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	created, err := client.Create(context.Background(), scope, "100.00", "", "maas-recharge-attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != MaaSRechargePaymentPending || created.QuotaDelta != 6849316 || created.QRCodeURL == "" {
		t.Fatalf("created = %#v", created)
	}
	queried, err := client.Get(context.Background(), scope, created.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if queried.Status != MaaSRechargeCredited || !queried.Credited || queried.RemoteQuotaAfter == nil || *queried.RemoteQuotaAfter != 7849316 || requests != 2 {
		t.Fatalf("queried = %#v requests=%d", queried, requests)
	}
}

func TestHTTPMaaSRechargeClientRejectsInvalidAmountAndMismatchedOrder(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr-recharge", BindingGeneration: 2}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(maasRechargeEnvelope(`"order_id":"another-order","status":"payment_pending","paid":false,"credited":false,"version":2`)))
	}))
	defer server.Close()
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-v3", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPMaaSRechargeClient(paymentV3TestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	for _, amount := range []string{"0", "-1", "1.001", "01.00"} {
		if _, err := client.Create(context.Background(), scope, amount, "alipay", "attempt"); err == nil {
			t.Fatalf("amount %q was accepted", amount)
		}
	}
	if _, err := client.Get(context.Background(), scope, "maasr-01"); err == nil {
		t.Fatal("mismatched order was accepted")
	}
}

func TestMaaSRechargeOrderAcceptsProviderQRCodeContent(t *testing.T) {
	var order MaaSRechargeOrder
	if err := json.Unmarshal([]byte(maasRechargeEnvelope(`"status":"payment_pending","paid":false,"credited":false,"qr_code_url":"alipays://platformapi/startapp?appId=20000067&url=provider-payload","version":2`)), &struct {
		Data *MaaSRechargeOrder `json:"data"`
	}{Data: &order}); err != nil {
		t.Fatal(err)
	}
	if err := order.Validate(); err != nil {
		t.Fatalf("provider QR content was rejected: %v", err)
	}
}

func maasRechargeEnvelope(overrides string) string {
	return `{"data":{"order_id":"maasr-01","amount":"100.00","amount_unit":"yuan","currency":"CNY","payment_method":"alipay","recharge_key_name":"usr-rechargecz","conversion_rule_version":"maas-quota-credit-v1","quota_per_usd":"500000.00000000","cny_per_usd":"7.30000000","quota_delta":6849316,"qr_code_url":"https://pay.example/maasr-01","expires_at":"2026-08-17T12:10:00Z",` + overrides + `},"error":null,"request_id":"req-recharge","server_time":"2026-08-17T12:00:00Z"}`
}
