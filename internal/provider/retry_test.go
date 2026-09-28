package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type blockingBody struct {
	closed chan struct{}
}

func (b *blockingBody) Read([]byte) (int, error) {
	<-b.closed
	return 0, io.EOF
}

func (b *blockingBody) Close() error {
	select {
	case <-b.closed:
	default:
		close(b.closed)
	}
	return nil
}

func statusResp(status int, hdr map[string]string) *http.Response {
	h := http.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("body")), Header: h}
}

func newDummyReq(ctx context.Context) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPost, "http://x/y", nil)
}

func TestRetryableStatus(t *testing.T) {
	for _, s := range []int{408, 429, 500, 502, 503, 504, 529, 599} {
		if !RetryableStatus(s) {
			t.Errorf("status %d should be retryable", s)
		}
	}
	for _, s := range []int{200, 400, 401, 402, 403, 404, 422} {
		if RetryableStatus(s) {
			t.Errorf("status %d should not be retryable", s)
		}
	}
}

func TestTransientErr(t *testing.T) {
	if transientErr(nil) {
		t.Error("nil should not be transient")
	}
	if transientErr(context.Canceled) || transientErr(context.DeadlineExceeded) {
		t.Error("ctx cancel/deadline should not be transient")
	}
	if !transientErr(errors.New("connection reset")) {
		t.Error("network-ish error should be transient")
	}
}

func TestIsConnReset(t *testing.T) {
	if IsConnReset(nil) {
		t.Error("nil is not a conn reset")
	}
	if IsConnReset(context.Canceled) || IsConnReset(context.DeadlineExceeded) {
		t.Error("ctx cancel/deadline must not look like a recoverable reset")
	}
	if IsConnReset(errors.New("decode stream: invalid character")) {
		t.Error("a plain protocol error must not be treated as a conn reset")
	}
	if IsConnReset(&net.DNSError{Err: "no such host", Name: "invalid.example"}) {
		t.Error("an arbitrary net.Error without a transport operation must not be treated as a conn reset")
	}
	for _, err := range []error{
		fmt.Errorf("provider: %w after 50ms", ErrStreamStalled),
		io.EOF,
		io.ErrUnexpectedEOF,
		net.ErrClosed,
		syscall.ECONNABORTED,
		fmt.Errorf("read stream: %w", syscall.ECONNRESET),
		&net.OpError{Op: "read", Err: syscall.ECONNRESET},
		fmt.Errorf("read stream: %w", &net.OpError{Op: "read", Err: errors.New("wsarecv: forcibly closed")}),
	} {
		if !IsConnReset(err) {
			t.Errorf("want conn reset for %v", err)
		}
	}
}

func TestBackoffDelay(t *testing.T) {
	if d := backoffDelay(1, 0); d < 500*time.Millisecond || d >= 750*time.Millisecond {
		t.Errorf("attempt 1 base delay = %v, want [500ms,750ms)", d)
	}
	if d := backoffDelay(20, 0); d > maxBackoff+250*time.Millisecond {
		t.Errorf("delay %v exceeds cap+jitter", d)
	}
	if d := backoffDelay(5, 3*time.Second); d != 3*time.Second {
		t.Errorf("Retry-After should win: %v", d)
	}
	if d := backoffDelay(1, time.Hour); d != maxBackoff {
		t.Errorf("Retry-After should be capped to %v, got %v", maxBackoff, d)
	}
}

func TestSendWithRetryFailsFastOnClientErrors(t *testing.T) {
	for _, status := range []int{400, 402, 422} {
		calls := 0
		cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return statusResp(status, nil), nil
		})}
		_, err := SendWithRetry(context.Background(), cl, SendOptions{Provider: "p", KeyEnv: "KEY"}, newDummyReq)
		if calls != 1 {
			t.Errorf("status %d retried (%d calls), should fail fast", status, calls)
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != status {
			t.Errorf("status %d: want *APIError with Status=%d, got %v", status, status, err)
		}
	}
}

func TestSendWithRetryAuthError(t *testing.T) {
	calls := 0
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return statusResp(401, nil), nil
	})}
	_, err := SendWithRetry(context.Background(), cl, SendOptions{Provider: "deepseek", KeyEnv: "DEEPSEEK_API_KEY", KeyPresent: true}, newDummyReq)
	if calls != 1 {
		t.Errorf("401 retried (%d calls), should fail fast for a never-authed key", calls)
	}
	var authErr *AuthError
	if !errors.As(err, &authErr) || authErr.KeyEnv != "DEEPSEEK_API_KEY" {
		t.Errorf("want *AuthError naming the key env, got %v", err)
	}
}

func TestSendWithRetryPreservesStructuredForbiddenErrorAndRetryPolicy(t *testing.T) {
	calls := 0
	body := `{"error":{"message":"token quota is not enough","type":"new_api_error","code":"pre_consume_token_quota_failed"}}`
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}, nil
	})}

	_, err := SendWithRetry(context.Background(), cl, SendOptions{
		Provider:  "airclaude_subscription",
		RetryAuth: true,
	}, newDummyReq)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusForbidden ||
		apiErr.Code != "pre_consume_token_quota_failed" ||
		apiErr.Message != "token quota is not enough" {
		t.Fatalf("structured error fields = %+v", apiErr)
	}
	if calls != 1+maxAuthRetries {
		t.Fatalf("structured 403 made %d requests, want existing auth policy of %d", calls, 1+maxAuthRetries)
	}
}

func TestSendWithRetryRetriesTransientAuthForKnownKey(t *testing.T) {
	calls := 0
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls <= 2 {
			return statusResp(401, nil), nil
		}
		return statusResp(200, nil), nil
	})}
	resp, err := SendWithRetry(context.Background(), cl,
		SendOptions{Provider: "mimo", KeyEnv: "MIMO_API_KEY", KeyPresent: true, RetryAuth: true}, newDummyReq)
	if err != nil {
		t.Fatalf("a previously-good key should recover from a transient 401: %v", err)
	}
	if resp.StatusCode != 200 || calls != 3 {
		t.Fatalf("status=%d calls=%d, want 200 after 3 calls", resp.StatusCode, calls)
	}
}

func TestSendWithRetryAuthGivesUpAfterMaxAuthRetries(t *testing.T) {
	calls := 0
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return statusResp(401, nil), nil
	})}
	_, err := SendWithRetry(context.Background(), cl,
		SendOptions{Provider: "mimo", KeyEnv: "MIMO_API_KEY", KeyPresent: true, RetryAuth: true}, newDummyReq)
	if calls != 1+maxAuthRetries {
		t.Errorf("persistent 401 made %d calls, want %d (initial + maxAuthRetries)", calls, 1+maxAuthRetries)
	}
	var authErr *AuthError
	if !errors.As(err, &authErr) || !authErr.HasKey {
		t.Fatalf("want *AuthError with HasKey=true, got %v", err)
	}
}

func TestSendWithRetryRecoversAndNotifies(t *testing.T) {
	calls := 0
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return statusResp(503, nil), nil
		}
		return statusResp(200, nil), nil
	})}
	var infos []RetryInfo
	ctx := WithRetryNotify(context.Background(), func(i RetryInfo) { infos = append(infos, i) })

	resp, err := SendWithRetry(ctx, cl, SendOptions{Provider: "p", KeyEnv: "KEY"}, newDummyReq)
	if err != nil {
		t.Fatalf("should recover after one retry: %v", err)
	}
	if resp.StatusCode != 200 || calls != 2 {
		t.Fatalf("status=%d calls=%d, want 200 after 2 calls", resp.StatusCode, calls)
	}
	if len(infos) != 1 || infos[0].Attempt != 1 || infos[0].Max != MaxRetries {
		t.Fatalf("retry notify = %#v, want one Attempt 1/%d", infos, MaxRetries)
	}
}

func TestSendWithRetryStopsAtTotalBudget(t *testing.T) {
	calls := 0
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return statusResp(http.StatusServiceUnavailable, map[string]string{"Retry-After": "15"}), nil
	})}

	started := time.Now()
	_, err := sendWithRetryBudget(
		context.Background(),
		cl,
		SendOptions{Provider: "p"},
		newDummyReq,
		20*time.Millisecond,
	)
	if !errors.Is(err, ErrRetryBudgetExhausted) {
		t.Fatalf("err = %v, want ErrRetryBudgetExhausted", err)
	}
	if calls != 1 {
		t.Fatalf("requests = %d, want one request before budget exhausted", calls)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("budget stop took %v", elapsed)
	}
}

func TestSendWithRetryCallerCancellationWinsBudget(t *testing.T) {
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return statusResp(http.StatusServiceUnavailable, nil), nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sendWithRetryBudget(ctx, cl, SendOptions{Provider: "p"}, newDummyReq, time.Second)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrRetryBudgetExhausted) {
		t.Fatalf("err = %v, want caller cancellation", err)
	}
}

func TestRetryBudgetPreservesLastAPIError(t *testing.T) {
	last := &APIError{Provider: "p", Status: http.StatusTooManyRequests, Code: "rate_limit"}
	err := retryBudgetError(time.Second, last)
	var apiErr *APIError
	if !errors.Is(err, ErrRetryBudgetExhausted) || !errors.As(err, &apiErr) || apiErr != last {
		t.Fatalf("retry budget error lost its provider cause: %v", err)
	}
}

func TestSendWithRetryBudgetClosesBlockingErrorBody(t *testing.T) {
	body := &blockingBody{closed: make(chan struct{})}
	cl := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: body, Header: http.Header{}}, nil
	})}
	started := time.Now()
	_, err := sendWithRetryBudget(context.Background(), cl, SendOptions{Provider: "p"}, newDummyReq, 20*time.Millisecond)
	if !errors.Is(err, ErrRetryBudgetExhausted) {
		t.Fatalf("err = %v, want ErrRetryBudgetExhausted", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("blocking response body escaped budget: %v", elapsed)
	}
}

func TestSendWithRetryRunsBeforeDispatchBeforeEveryHTTPAttempt(t *testing.T) {
	requests, before := 0, 0
	cl := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return statusResp(http.StatusServiceUnavailable, nil), nil
		}
		return statusResp(http.StatusOK, nil), nil
	})}
	ctx := WithBeforeDispatch(context.Background(), func() error { before++; return nil })
	resp, err := SendWithRetry(ctx, cl, SendOptions{Provider: "test"}, newDummyReq)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if before != 2 || requests != 2 {
		t.Fatalf("before/requests = %d/%d, want 2/2", before, requests)
	}
}

func TestSendWithRetryStopsBeforeNetworkWhenBeforeDispatchFails(t *testing.T) {
	requests := 0
	cl := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return statusResp(http.StatusOK, nil), nil
	})}
	want := errors.New("write ahead failed")
	ctx := WithBeforeDispatch(context.Background(), func() error { return want })
	_, err := SendWithRetry(ctx, cl, SendOptions{Provider: "test"}, newDummyReq)
	if !errors.Is(err, want) || requests != 0 {
		t.Fatalf("err/requests = %v/%d", err, requests)
	}
}

func TestSendWithRetryDoesNotBeginAttemptWhenRequestCannotBeBuilt(t *testing.T) {
	before := 0
	ctx := WithBeforeDispatch(context.Background(), func() error { before++; return nil })
	_, err := SendWithRetry(ctx, http.DefaultClient, SendOptions{Provider: "test"}, func(context.Context) (*http.Request, error) {
		return nil, errors.New("invalid request")
	})
	if err == nil || before != 0 {
		t.Fatalf("err/before = %v/%d, want request build error and no begin attempt", err, before)
	}
}
