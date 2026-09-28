package airclaude

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	"kwu/internal/provider"
	"kwu/internal/provider/openai"
)

// Gateway is the native AirClaude subscription adapter. The desktop
// composition wraps it with the current cloud binding authorizer before this
// type can be reached.
type Gateway struct {
	proxy   netclient.ProxySpec
	apiKey  string
	now     func() time.Time
	mu      sync.Mutex
	byModel map[string]provider.Provider
}

func NewGateway(apiKey string, proxy netclient.ProxySpec, now func() time.Time) (*Gateway, error) {
	apiKey, err := validateAPIKey(apiKey)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &Gateway{proxy: proxy, apiKey: apiKey, now: now, byModel: make(map[string]provider.Provider)}, nil
}

func (g *Gateway) EndpointFor(request modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (g *Gateway) Stream(
	ctx context.Context,
	binding cloud.BindingScope,
	request modelbilling.GatewayRequest,
) (<-chan modelbilling.GatewayChunk, error) {
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	if _, err := g.EndpointFor(request); err != nil {
		return nil, err
	}
	model, err := g.provider(request.ModelID)
	if err != nil {
		return nil, err
	}
	chunks, err := model.Stream(ctx, request.ProviderRequest)
	if err != nil {
		return nil, err
	}
	out := make(chan modelbilling.GatewayChunk)
	go g.forward(ctx, chunks, out)
	return out, nil
}

func (g *Gateway) provider(modelID string) (provider.Provider, error) {
	if g == nil {
		return nil, fmt.Errorf("airclaude gateway is required")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if existing := g.byModel[modelID]; existing != nil {
		return existing, nil
	}
	created, err := openai.New(provider.Config{
		Name: "airclaude_subscription", BaseURL: config.OfficialProviderBaseURL,
		Model: modelID, APIKey: g.apiKey,
		Extra: map[string]any{
			"include_billing": true,
			"proxy_spec":      g.proxy,
			"api_key_source":  "cloud account MaaS credential",
		},
	})
	if err != nil {
		return nil, err
	}
	g.byModel[modelID] = created
	return created, nil
}

func (g *Gateway) forward(
	ctx context.Context,
	source <-chan provider.Chunk,
	destination chan<- modelbilling.GatewayChunk,
) {
	defer close(destination)
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-source:
			if !ok {
				return
			}
			adapted, err := g.adaptChunk(chunk)
			if err != nil {
				adapted = modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkError, Err: err}}
			}
			select {
			case <-ctx.Done():
				return
			case destination <- adapted:
			}
			if err != nil {
				return
			}
		}
	}
}

func (g *Gateway) adaptChunk(chunk provider.Chunk) (modelbilling.GatewayChunk, error) {
	adapted := modelbilling.GatewayChunk{Chunk: chunk}
	evidence := chunk.BillingEvidence
	adapted.Chunk.BillingEvidence = nil
	if chunk.Type == provider.ChunkUsage && chunk.Usage != nil {
		adapted.Usage = normalizeUsage(chunk.Usage)
	}
	if evidence == nil {
		return validateAdaptedChunk(adapted)
	}
	if chunk.Type != provider.ChunkDone {
		return modelbilling.GatewayChunk{}, fmt.Errorf("airclaude: billing evidence requires the final done chunk")
	}
	revisions, err := modelbilling.NormalizeQuotaEvidenceRevisions(modelbilling.QuotaObservationInput{
		Endpoint: modelbilling.MaasEndpointChatCompletions,
		Headers: http.Header{
			"X-Oneapi-Request-Id": append([]string(nil), evidence.ProviderRequestIDs...),
		},
		FinalJSON:  evidence.FinalJSON,
		ObservedAt: g.now().UTC(),
	})
	if err != nil {
		return modelbilling.GatewayChunk{}, err
	}
	if len(revisions) == 1 {
		adapted.QuotaEvidence = &revisions[0]
		if revisions[0].OneAPIRequestID != nil {
			adapted.ProviderRequestID = *revisions[0].OneAPIRequestID
		}
	} else {
		adapted.QuotaEvidenceRevisions = revisions
	}
	return validateAdaptedChunk(adapted)
}

func validateAdaptedChunk(chunk modelbilling.GatewayChunk) (modelbilling.GatewayChunk, error) {
	if err := chunk.Validate(); err != nil {
		return modelbilling.GatewayChunk{}, err
	}
	return chunk, nil
}

func normalizeUsage(usage *provider.Usage) *modelbilling.NormalizedUsage {
	value := func(present bool, tokens int) *int64 {
		if !present {
			return nil
		}
		result := int64(tokens)
		return &result
	}
	requestCount := int64(1)
	return &modelbilling.NormalizedUsage{
		InputTokens:         value(usage.Presence.Input, usage.PromptTokens),
		NormalInputTokens:   value(usage.Presence.NormalInput, usage.NormalInputTokens),
		CachedInputTokens:   value(usage.Presence.CachedInput, usage.CacheHitTokens),
		CacheReadTokens:     value(usage.Presence.CacheRead, usage.CacheReadTokens),
		CacheWriteTokens:    value(usage.Presence.CacheWrite, usage.CacheWriteTokens),
		OutputTokens:        value(usage.Presence.Output, usage.CompletionTokens),
		ProviderTotalTokens: value(usage.Presence.ProviderTotal, usage.TotalTokens),
		RequestCount:        &requestCount,
		BreakdownStatus:     modelbilling.UsagePartial,
	}
}

var _ modelbilling.SubscriptionModelGateway = (*Gateway)(nil)
var _ modelbilling.SubscriptionGatewayEndpointResolver = (*Gateway)(nil)
