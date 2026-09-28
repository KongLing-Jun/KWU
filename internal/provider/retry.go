package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// MaxRetries is the number of times SendWithRetry re-attempts the connection +
// header phase after the initial try (so up to MaxRetries+1 total attempts).
const MaxRetries = 10

const (
	maxBackoff     = 15 * time.Second
	maxRetryBudget = 45 * time.Second
)

// ErrRetryBudgetExhausted reports that retries stopped at the local elapsed
// safety limit. It is distinct from a caller cancellation or deadline.
var ErrRetryBudgetExhausted = errors.New("provider retry budget exhausted")

// ErrStreamStalled marks a started response body that stopped producing bytes.
// Providers wrap it with their name and idle window so callers retain useful
// diagnostics while IsConnReset can classify the failure as transport-level and
// safely retry or continue the interrupted turn.
var ErrStreamStalled = errors.New("stream stalled")

// maxAuthRetries bounds how many times a 401/403 is retried for a key that has
// authenticated before: a transient server-side rejection (quota/gateway/rate)
// usually clears in a couple of attempts, whereas a key that never worked is a
// real config error and fails fast.
const maxAuthRetries = 2

// SendOptions carries the per-request context SendWithRetry needs to label
// errors and decide whether a 401 is worth retrying.
type SendOptions struct {
	Provider   string // provider instance name, surfaced in errors
	KeyEnv     string // api_key_env the key is read from, when known
	KeySource  string // human-readable source of KeyEnv, when known
	KeyOwner   string // human-readable credential owner, when known
	KeyPresent bool   // a non-empty key is being sent — separates "rejected" from "missing"
	RetryAuth  bool   // the key has authenticated before — retry transient 401s instead of failing fast
}

// RetryInfo describes a backoff about to happen: Attempt is the 1-based retry
// number (of Max) and Delay is how long SendWithRetry will wait before it.
type RetryInfo struct {
	Attempt int
	Max     int
	Delay   time.Duration
	Err     error
}

type RetryNotify func(RetryInfo)

type retryNotifyKey struct{}

// BeforeDispatch is request-scoped write-ahead work that must complete before
// every physical HTTP attempt. It lives in context so shared providers never
// retain task or call identity across concurrent tasks.
type BeforeDispatch func() error

type beforeDispatchKey struct{}

func WithBeforeDispatch(ctx context.Context, fn BeforeDispatch) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, beforeDispatchKey{}, fn)
}

func beforeDispatchFromContext(ctx context.Context) BeforeDispatch {
	fn, _ := ctx.Value(beforeDispatchKey{}).(BeforeDispatch)
	return fn
}

// WithRetryNotify attaches a callback that SendWithRetry invokes before each
// backoff sleep, so the agent can surface a transient "retrying (n/m)" status.
func WithRetryNotify(ctx context.Context, fn RetryNotify) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, retryNotifyKey{}, fn)
}

func retryNotifyFromContext(ctx context.Context) RetryNotify {
	fn, _ := ctx.Value(retryNotifyKey{}).(RetryNotify)
	return fn
}

// APIError reports a provider error. HTTP failures carry Status and Body;
// OpenAI-compatible JSON errors also retain their stable Code and Message so
// the display layer does not have to match provider prose.
type APIError struct {
	Provider string
	Status   int
	Body     string
	Code     string
	Message  string
}

func (e *APIError) Error() string {
	prefix := e.Provider
	if e.Status != 0 {
		prefix = fmt.Sprintf("%s: status %d", prefix, e.Status)
	}
	if e.Code != "" {
		prefix += " [" + e.Code + "]"
	}
	if e.Message != "" {
		return prefix + ": " + e.Message
	}
	if e.Body != "" {
		return prefix + ": " + e.Body
	}
	return prefix
}

func newAPIError(providerName string, status int, body string) *APIError {
	apiErr := &APIError{Provider: providerName, Status: status, Body: body}
	var parsed struct {
		Error struct {
			Code    string `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(body), &parsed) == nil {
		apiErr.Code = strings.TrimSpace(parsed.Error.Code)
		if apiErr.Code == "" {
			apiErr.Code = strings.TrimSpace(parsed.Error.Type)
		}
		apiErr.Message = strings.TrimSpace(parsed.Error.Message)
	}
	return apiErr
}

// RetryableStatus reports whether a backoff can plausibly recover from status s:
// 408 (request timeout), 429 (rate limit) and 5xx (incl. Anthropic's 529). Other
// 4xx (400/401/402/422, …) are caller/config problems retrying can't fix.
func RetryableStatus(s int) bool {
	return s == http.StatusRequestTimeout || s == http.StatusTooManyRequests || (s >= 500 && s <= 599)
}

func transientErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

// IsConnReset reports whether err is a connection-level drop (peer reset,
// truncated body, closed socket) as opposed to a protocol or caller error. A
// stream cut this way mid-body can be replayed from scratch, unlike a decode or
// 4xx error. The common trigger is a local proxy (v2rayN/sing-box) idle-closing
// the long-lived SSE connection during a reasoner's first-token gap.
func IsConnReset(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrStreamStalled) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

func backoffDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > maxBackoff {
			return maxBackoff
		}
		return retryAfter
	}
	d := time.Duration(1<<(attempt-1)) * 500 * time.Millisecond
	if d > maxBackoff {
		d = maxBackoff
	}
	return d + time.Duration(rand.Intn(250))*time.Millisecond
}

func parseRetryAfter(resp *http.Response) time.Duration {
	v := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// SendWithRetry POSTs a streaming request built by newReq and returns the OK
// response. It retries the connection+header phase up to MaxRetries times on
// transient network errors and retryable statuses with capped exponential
// backoff + jitter, honoring Retry-After. A 401/403 becomes *AuthError: it
// fails fast for a key that has never authenticated (opts.RetryAuth false), but
// for a previously-good key it backs off and retries up to maxAuthRetries —
// MiMo and similar gateways return a transient 401 under load. Other non-OK
// statuses become *APIError. A RetryNotify in ctx fires before each sleep.
// Retries cover only the header phase — once the body streams, mid-stream
// failures are not retried (the model has already emitted tokens).
func SendWithRetry(ctx context.Context, httpClient *http.Client, opts SendOptions, newReq func(context.Context) (*http.Request, error)) (*http.Response, error) {
	return sendWithRetryBudget(ctx, httpClient, opts, newReq, maxRetryBudget)
}

func sendWithRetryBudget(ctx context.Context, httpClient *http.Client, opts SendOptions, newReq func(context.Context) (*http.Request, error), budget time.Duration) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(budget)
	notify := retryNotifyFromContext(ctx)
	beforeDispatch := beforeDispatchFromContext(ctx)
	state := retryAttemptState{}

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if err := waitForRetryAttempt(ctx, attempt, deadline, budget, notify, state); err != nil {
			return nil, err
		}
		result := runRetryAttempt(ctx, httpClient, opts, newReq, beforeDispatch, deadline, budget, state)
		if result.response != nil {
			return result.response, nil
		}
		if !result.retry {
			return nil, result.err
		}
		state.lastErr = result.err
		state.retryAfter = result.retryAfter
		state.authRetries += result.authRetry
	}
	return nil, state.lastErr
}

type retryAttemptState struct {
	lastErr     error
	retryAfter  time.Duration
	authRetries int
}

type retryAttemptResult struct {
	response   *http.Response
	err        error
	retryAfter time.Duration
	retry      bool
	authRetry  int
}

func waitForRetryAttempt(ctx context.Context, attempt int, deadline time.Time, budget time.Duration, notify RetryNotify, state retryAttemptState) error {
	if attempt == 0 {
		return nil
	}
	delay := backoffDelay(attempt, state.retryAfter)
	if notify != nil {
		notify(RetryInfo{Attempt: attempt, Max: MaxRetries, Delay: delay, Err: state.lastErr})
	}
	timer := time.NewTimer(delay)
	budgetWait := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	defer budgetWait.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-budgetWait.C:
		return retryBudgetOrContextError(ctx, budget, state.lastErr)
	case <-timer.C:
		return nil
	}
}

func runRetryAttempt(ctx context.Context, httpClient *http.Client, opts SendOptions, newReq func(context.Context) (*http.Request, error), beforeDispatch BeforeDispatch, deadline time.Time, budget time.Duration, state retryAttemptState) retryAttemptResult {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return retryAttemptResult{err: retryBudgetError(budget, state.lastErr)}
	}
	attemptCtx, cancelAttempt := context.WithCancel(ctx)
	budgetTimer := time.AfterFunc(remaining, cancelAttempt)
	resp, dispatched, err := dispatchAttempt(attemptCtx, httpClient, opts.Provider, beforeDispatch, newReq)
	if err != nil {
		return finishFailedDispatch(ctx, opts.Provider, err, dispatched, cancelAttempt, budgetTimer, deadline, budget, state.lastErr)
	}
	if resp.StatusCode == http.StatusOK {
		return finishSuccessfulDispatch(ctx, resp, cancelAttempt, budgetTimer, deadline, budget, state.lastErr)
	}
	return finishErrorResponse(ctx, attemptCtx, resp, opts, cancelAttempt, budgetTimer, deadline, budget, state)
}

func finishFailedDispatch(ctx context.Context, providerName string, dispatchErr error, dispatched bool, cancel context.CancelFunc, timer *time.Timer, deadline time.Time, budget time.Duration, lastErr error) retryAttemptResult {
	budgetExpired := !timer.Stop()
	cancel()
	if err := ctx.Err(); err != nil {
		return retryAttemptResult{err: err}
	}
	if budgetExpired || time.Until(deadline) <= 0 {
		return retryAttemptResult{err: retryBudgetError(budget, lastErr)}
	}
	wrapped := fmt.Errorf("%s: request failed: %w", providerName, dispatchErr)
	return retryAttemptResult{err: wrapped, retry: dispatched && transientErr(dispatchErr)}
}

func finishSuccessfulDispatch(ctx context.Context, resp *http.Response, cancel context.CancelFunc, timer *time.Timer, deadline time.Time, budget time.Duration, lastErr error) retryAttemptResult {
	if timer.Stop() {
		resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
		return retryAttemptResult{response: resp}
	}
	resp.Body.Close()
	cancel()
	return retryAttemptResult{err: retryBudgetOrContextError(ctx, budget, lastErr)}
}

func finishErrorResponse(ctx, attemptCtx context.Context, resp *http.Response, opts SendOptions, cancel context.CancelFunc, timer *time.Timer, deadline time.Time, budget time.Duration, state retryAttemptState) retryAttemptResult {
	stopBodyCancel := closeBodyOnContext(attemptCtx, resp.Body)
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	retryAfter := parseRetryAfter(resp)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	stopBodyCancel()
	budgetExpired := !timer.Stop()
	cancel()
	if err := ctx.Err(); err != nil {
		return retryAttemptResult{err: err}
	}
	if budgetExpired || time.Until(deadline) <= 0 {
		return retryAttemptResult{err: retryBudgetError(budget, state.lastErr)}
	}
	return classifyErrorResponse(resp.StatusCode, strings.TrimSpace(string(msg)), opts, retryAfter, state.authRetries)
}

func classifyErrorResponse(status int, body string, opts SendOptions, retryAfter time.Duration, authRetries int) retryAttemptResult {
	apiErr := newAPIError(opts.Provider, status, body)
	if status == http.StatusForbidden && apiErr.Code != "" {
		retry, authRetry := authRetryDecision(opts, authRetries)
		return retryAttemptResult{err: apiErr, retryAfter: retryAfter, retry: retry, authRetry: authRetry}
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		authErr := &AuthError{Provider: opts.Provider, KeyEnv: opts.KeyEnv, KeySource: opts.KeySource, KeyOwner: opts.KeyOwner, Status: status, HasKey: opts.KeyPresent}
		retry, authRetry := authRetryDecision(opts, authRetries)
		return retryAttemptResult{err: authErr, retryAfter: retryAfter, retry: retry, authRetry: authRetry}
	}
	return retryAttemptResult{err: apiErr, retryAfter: retryAfter, retry: RetryableStatus(status)}
}

func authRetryDecision(opts SendOptions, authRetries int) (bool, int) {
	retry := opts.RetryAuth && authRetries < maxAuthRetries
	if retry {
		return true, 1
	}
	return false, 0
}

func retryBudgetOrContextError(ctx context.Context, budget time.Duration, lastErr error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return retryBudgetError(budget, lastErr)
}

func closeBodyOnContext(ctx context.Context, body io.Closer) func() {
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	go func() {
		select {
		case <-ctx.Done():
			_ = body.Close()
		case <-done:
		}
	}()
	return stop
}

type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body *cancelOnCloseBody) Close() error {
	err := body.ReadCloser.Close()
	body.cancel()
	return err
}

func retryBudgetError(budget time.Duration, lastErr error) error {
	if lastErr == nil {
		return fmt.Errorf("%w after %s", ErrRetryBudgetExhausted, budget)
	}
	return fmt.Errorf("%w after %s: %w", ErrRetryBudgetExhausted, budget, lastErr)
}

func dispatchAttempt(ctx context.Context, httpClient *http.Client, providerName string, beforeDispatch BeforeDispatch, newReq func(context.Context) (*http.Request, error)) (*http.Response, bool, error) {
	req, err := newReq(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("%s: build request: %w", providerName, err)
	}
	if err := runBeforeDispatch(beforeDispatch); err != nil {
		return nil, false, err
	}
	resp, err := httpClient.Do(req)
	return resp, true, err
}

func runBeforeDispatch(beforeDispatch BeforeDispatch) error {
	if beforeDispatch == nil {
		return nil
	}
	return beforeDispatch()
}
