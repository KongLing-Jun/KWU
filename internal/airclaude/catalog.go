package airclaude

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kwu/internal/config"
	"kwu/internal/netclient"
	"kwu/internal/provider/openai"
)

const modelCatalogTimeout = 10 * time.Second

// FetchChatModels reads the product-owned AirClaude catalog and returns only
// models that explicitly support the OpenAI chat endpoint used by Desktop.
// Specialized or capability-unknown models remain available to other product
// surfaces but are not exposed by the home-page chat switcher.
func FetchChatModels(ctx context.Context, proxy netclient.ProxySpec, apiKey string) ([]string, error) {
	apiKey, err := validateAPIKey(apiKey)
	if err != nil {
		return nil, err
	}
	client, err := netclient.NewHTTPClient(proxy, netclient.TransportOptions{})
	if err != nil {
		return nil, fmt.Errorf("airclaude model catalog: %w", err)
	}
	client.Timeout = modelCatalogTimeout
	return fetchChatModels(ctx, client, config.OfficialProviderBaseURL, apiKey)
}

func fetchChatModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]string, error) {
	catalog, err := openai.FetchModelCatalogWithClient(ctx, client, baseURL, apiKey)
	if err != nil {
		return nil, fmt.Errorf("airclaude model catalog: %w", err)
	}
	seen := make(map[string]struct{}, len(catalog))
	normalized := make([]string, 0, len(catalog))
	for _, entry := range catalog {
		if !supportsEndpoint(entry.SupportedEndpointTypes, "openai") {
			continue
		}
		model := strings.TrimSpace(entry.ID)
		if model == "" {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		normalized = append(normalized, model)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("airclaude model catalog: response contains no OpenAI chat models")
	}
	return normalized, nil
}

func supportsEndpoint(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
