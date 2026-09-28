package wecom

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"kwu/internal/bot"
)

const (
	officialWebhookEndpoint = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"
	maxTextBytes            = 2048
	maxResponseBytes        = 64 << 10
	webhookLimitPerMinute   = 20
)

var processWebhookLimiter = newWebhookRateLimiter(time.Now)

type webhookRateWindow struct {
	startedAt time.Time
	count     int
}

type webhookRateLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	windows map[[sha256.Size]byte]webhookRateWindow
}

func newWebhookRateLimiter(now func() time.Time) *webhookRateLimiter {
	if now == nil {
		now = time.Now
	}
	return &webhookRateLimiter{now: now, windows: make(map[[sha256.Size]byte]webhookRateWindow)}
}

func (l *webhookRateLimiter) reserve(webhookKey string) bool {
	if l == nil {
		return false
	}
	now := l.now().UTC()
	fingerprint := sha256.Sum256([]byte(webhookKey))
	l.mu.Lock()
	defer l.mu.Unlock()
	window := l.windows[fingerprint]
	if window.startedAt.IsZero() || now.Before(window.startedAt) || now.Sub(window.startedAt) >= time.Minute {
		window = webhookRateWindow{startedAt: now}
	}
	if window.count >= webhookLimitPerMinute {
		return false
	}
	window.count++
	l.windows[fingerprint] = window
	return true
}

type FailureKind string

const (
	FailureInvalidRequest FailureKind = "invalid_request"
	FailureRateLimited    FailureKind = "rate_limited"
	FailureTargetRevoked  FailureKind = "target_revoked"
	FailureTemporary      FailureKind = "temporary"
)

// DeliveryError exposes only a stable failure category and numeric provider
// code. It deliberately omits provider response text and webhook credentials.
type DeliveryError struct {
	Kind         FailureKind
	ProviderCode int
}

func (e *DeliveryError) Error() string {
	if e == nil {
		return "wecom: delivery failed"
	}
	switch e.Kind {
	case FailureInvalidRequest:
		return "wecom: request is invalid"
	case FailureRateLimited:
		return "wecom: delivery was rate limited"
	case FailureTargetRevoked:
		return "wecom: webhook is unavailable"
	default:
		return "wecom: delivery is temporarily unavailable"
	}
}

func FailureKindOf(err error) (FailureKind, bool) {
	var deliveryErr *DeliveryError
	if !errors.As(err, &deliveryErr) {
		return "", false
	}
	return deliveryErr.Kind, true
}

type textRequest struct {
	MessageType string      `json:"msgtype"`
	Text        textContent `json:"text"`
}

type textContent struct {
	Content string `json:"content"`
}

type webhookResponse struct {
	ErrorCode int `json:"errcode"`
}

// SendText sends one UTF-8 text message through an enterprise WeChat group
// robot. webhookKey is resolved by the caller from a profile credential store;
// it must never be persisted in bot configuration or automation task data.
func SendText(ctx context.Context, webhookKey, text string) (bot.SendResult, error) {
	if err := validateWebhookKey(webhookKey); err != nil {
		return bot.SendResult{}, err
	}
	if !processWebhookLimiter.reserve(webhookKey) {
		return bot.SendResult{}, &DeliveryError{Kind: FailureRateLimited}
	}
	return sendText(ctx, http.DefaultClient, officialWebhookEndpoint, webhookKey, text)
}

func sendText(ctx context.Context, client *http.Client, endpoint, webhookKey, text string) (bot.SendResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := newTextHTTPRequest(ctx, endpoint, webhookKey, text)
	if err != nil {
		return bot.SendResult{}, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return bot.SendResult{}, safeRequestError(ctx)
	}
	defer resp.Body.Close()
	return decodeWebhookResponse(resp)
}

func newTextHTTPRequest(ctx context.Context, endpoint, webhookKey, text string) (*http.Request, error) {
	if err := validateWebhookKey(webhookKey); err != nil {
		return nil, err
	}
	if text == "" || len([]byte(text)) > maxTextBytes {
		return nil, &DeliveryError{Kind: FailureInvalidRequest}
	}
	body, err := json.Marshal(textRequest{MessageType: "text", Text: textContent{Content: text}})
	if err != nil {
		return nil, &DeliveryError{Kind: FailureInvalidRequest}
	}
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, &DeliveryError{Kind: FailureInvalidRequest}
	}
	query := requestURL.Query()
	query.Set("key", webhookKey)
	requestURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, &DeliveryError{Kind: FailureInvalidRequest}
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return req, nil
}

func safeRequestError(ctx context.Context) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return &DeliveryError{Kind: FailureTemporary}
}

func decodeWebhookResponse(resp *http.Response) (bot.SendResult, error) {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		kind := FailureTemporary
		switch resp.StatusCode {
		case http.StatusBadRequest:
			kind = FailureInvalidRequest
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone:
			kind = FailureTargetRevoked
		case http.StatusTooManyRequests:
			kind = FailureRateLimited
		}
		return bot.SendResult{}, &DeliveryError{Kind: kind, ProviderCode: resp.StatusCode}
	}
	var decoded webhookResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&decoded); err != nil {
		return bot.SendResult{}, errors.New("wecom: invalid provider response")
	}
	if decoded.ErrorCode != 0 {
		kind := FailureTemporary
		switch decoded.ErrorCode {
		case 45009:
			kind = FailureRateLimited
		case 93000, 93004:
			kind = FailureTargetRevoked
		}
		return bot.SendResult{}, &DeliveryError{Kind: kind, ProviderCode: decoded.ErrorCode}
	}
	return bot.SendResult{}, nil
}

func validateWebhookKey(key string) error {
	if key == "" || strings.TrimSpace(key) != key || len(key) > 512 {
		return &DeliveryError{Kind: FailureInvalidRequest}
	}
	for _, r := range key {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return &DeliveryError{Kind: FailureInvalidRequest}
		}
	}
	return nil
}
