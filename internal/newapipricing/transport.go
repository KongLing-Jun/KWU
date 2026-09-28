package newapipricing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"kwu/internal/modelbilling"
)

const userAgent = "KWU-Desktop/ModelMarketplace"

// Transport only obtains the raw pricing document. Endpoint and timeout remain
// owned by the caller so the model marketplace keeps one source configuration.
type Transport interface {
	Fetch(ctx context.Context, endpoint string) ([]byte, error)
}

type HTTPTransport struct {
	client *http.Client
}

func NewHTTPTransport(client *http.Client) HTTPTransport {
	if client == nil {
		client = http.DefaultClient
	}
	return HTTPTransport{client: client}
}

func (t HTTPTransport) Fetch(ctx context.Context, endpoint string) ([]byte, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, contractError("newapi pricing endpoint is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, contractError("create newapi pricing request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.client.Do(req)
	if err != nil {
		if cancellation := contextFailure(ctx, "request newapi pricing", err); cancellation != nil {
			return nil, cancellation
		}
		return nil, pricingError(true, "request newapi pricing: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
		return nil, pricingError(retryable, "request newapi pricing: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if cancellation := contextFailure(ctx, "read newapi pricing response", err); cancellation != nil {
			return nil, cancellation
		}
		return nil, pricingError(true, "read newapi pricing response: %v", err)
	}
	return body, nil
}

func contextFailure(ctx context.Context, operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", operation, err)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return fmt.Errorf("%s: %w", operation, contextErr)
	}
	return nil
}

func contractError(format string, args ...any) error {
	return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: fmt.Sprintf(format, args...)}
}

func pricingError(retryable bool, format string, args ...any) error {
	return &modelbilling.Error{
		Code:      modelbilling.ErrorPricingUnavailable,
		Message:   fmt.Sprintf(format, args...),
		Retryable: retryable,
	}
}
