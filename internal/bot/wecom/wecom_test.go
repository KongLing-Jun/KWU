package wecom

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestSendTextPostsOfficialPayloadWithoutLeakingKey(t *testing.T) {
	const key = "explicit-invalid-test-key"
	var requestURL, requestBody string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestURL = req.URL.String()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll request: %v", err)
		}
		requestBody = string(body)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)), Header: make(http.Header)}, nil
	})}

	if _, err := sendText(context.Background(), client, officialWebhookEndpoint, key, "daily summary"); err != nil {
		t.Fatalf("sendText: %v", err)
	}
	if requestURL != officialWebhookEndpoint+"?key=explicit-invalid-test-key" {
		t.Fatalf("request URL = %q", requestURL)
	}
	if requestBody != `{"msgtype":"text","text":{"content":"daily summary"}}` {
		t.Fatalf("request body = %s", requestBody)
	}
}

func TestWebhookRateLimiterSharesQuotaByCredentialFingerprint(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	limiter := newWebhookRateLimiter(func() time.Time { return now })
	for i := 0; i < webhookLimitPerMinute; i++ {
		if !limiter.reserve("shared-webhook-key") {
			t.Fatalf("reservation %d unexpectedly rejected", i+1)
		}
	}
	if limiter.reserve("shared-webhook-key") {
		t.Fatal("21st reservation for the same webhook was accepted")
	}
	if !limiter.reserve("different-webhook-key") {
		t.Fatal("a different webhook did not receive an independent quota")
	}
	now = now.Add(time.Minute)
	if !limiter.reserve("shared-webhook-key") {
		t.Fatal("quota did not reset after one minute")
	}
}

func TestSendTextReturnsSafeErrors(t *testing.T) {
	const key = "explicit-invalid-test-key"
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "provider error", status: http.StatusOK, body: `{"errcode":93000,"errmsg":"key private-webhook-key provider-secret-detail"}`},
		{name: "http error", status: http.StatusBadGateway, body: `upstream echoed private-webhook-key`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
			})}
			_, err := sendText(context.Background(), client, officialWebhookEndpoint, key, "hello")
			if err == nil {
				t.Fatal("sendText error = nil")
			}
			if strings.Contains(err.Error(), key) || strings.Contains(err.Error(), "echoed") || strings.Contains(err.Error(), "provider-secret-detail") {
				t.Fatalf("unsafe error = %v", err)
			}
		})
	}
}

func TestSendTextClassifiesOfficialFailureCodes(t *testing.T) {
	tests := []struct {
		body string
		want FailureKind
	}{
		{body: `{"errcode":45009,"errmsg":"rate limited"}`, want: FailureRateLimited},
		{body: `{"errcode":93000,"errmsg":"webhook removed"}`, want: FailureTargetRevoked},
		{body: `{"errcode":93004,"errmsg":"webhook disabled"}`, want: FailureTargetRevoked},
	}
	for _, tt := range tests {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
		})}
		_, err := sendText(context.Background(), client, officialWebhookEndpoint, "private-key", "hello")
		kind, ok := FailureKindOf(err)
		if !ok || kind != tt.want {
			t.Errorf("failure kind = %q, %v; want %q", kind, err, tt.want)
		}
	}
}
