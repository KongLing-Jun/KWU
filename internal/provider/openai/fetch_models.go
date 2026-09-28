package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type modelFetchStatusError struct {
	status int
	body   string
}

// ModelCatalogEntry is the capability-bearing subset of an OpenAI-compatible
// /models response used by product integrations that need to select a concrete
// endpoint before exposing a model.
type ModelCatalogEntry struct {
	ID                     string
	SupportedEndpointTypes []string
}

func (e modelFetchStatusError) Error() string {
	return fmt.Sprintf("fetch models: status %d: %s", e.status, strings.TrimSpace(e.body))
}

// IsModelFetchEndpointMiss reports whether a model-list request reached a
// plausible endpoint path that the provider does not implement.
func IsModelFetchEndpointMiss(err error) bool {
	var statusErr modelFetchStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.status == http.StatusNotFound || statusErr.status == http.StatusMethodNotAllowed
}

// FetchModels calls the OpenAI-compatible GET /models endpoint and returns the
// available model IDs.
func FetchModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	cli := &http.Client{Timeout: 10 * time.Second}
	return FetchModelsWithClient(ctx, cli, baseURL, apiKey)
}

// FetchModelsWithClient is FetchModels with a caller-owned HTTP client. It is
// used by native product integrations that must honor the desktop proxy policy.
func FetchModelsWithClient(ctx context.Context, cli *http.Client, baseURL, apiKey string) ([]string, error) {
	body, err := fetchModelsResponse(ctx, cli, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return decodeModelIDs(body)
}

// FetchModelCatalogWithClient calls GET /models and preserves endpoint
// capability metadata when the provider supplies it.
func FetchModelCatalogWithClient(ctx context.Context, cli *http.Client, baseURL, apiKey string) ([]ModelCatalogEntry, error) {
	body, err := fetchModelsResponse(ctx, cli, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return decodeModelCatalog(body)
}

func fetchModelsResponse(ctx context.Context, cli *http.Client, baseURL, apiKey string) ([]byte, error) {
	if cli == nil {
		return nil, fmt.Errorf("fetch models: HTTP client is required")
	}
	url := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(url, "/models") {
		url += "/models"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch models: build request: %w", err)
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("fetch models: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, modelFetchStatusError{status: resp.StatusCode, body: truncateFetchBody(string(body))}
	}

	return body, nil
}

func decodeModelIDs(body []byte) ([]string, error) {
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("fetch models: decode response: %w", err)
	}
	ids := make([]string, 0, len(result.Data))
	for _, model := range result.Data {
		if id := strings.TrimSpace(model.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func decodeModelCatalog(body []byte) ([]ModelCatalogEntry, error) {
	var result struct {
		Data []struct {
			ID                     string   `json:"id"`
			SupportedEndpointTypes []string `json:"supported_endpoint_types"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("fetch models: decode response: %w", err)
	}

	models := make([]ModelCatalogEntry, 0, len(result.Data))
	for _, m := range result.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			models = append(models, ModelCatalogEntry{
				ID:                     id,
				SupportedEndpointTypes: cleanModelEndpointTypes(m.SupportedEndpointTypes),
			})
		}
	}
	return models, nil
}

func cleanModelEndpointTypes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func truncateFetchBody(body string) string {
	body = strings.TrimSpace(body)
	const max = 512
	if len([]rune(body)) <= max {
		return body
	}
	r := []rune(body)
	return string(r[:max]) + "..."
}
