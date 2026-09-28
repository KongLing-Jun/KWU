package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kwu/internal/fileutil"
	"kwu/internal/modelbilling"
	"kwu/internal/newapipricing"
)

const (
	modelMarketplaceEndpoint   = "https://api.airclaudeai.com/api/pricing"
	modelMarketplacePricingURL = "https://www.airclaudeai.com/models"
	modelMarketplaceTimeout    = 8 * time.Second

	modelMarketplaceCacheVersion   = 1
	marketplaceMetadataReady       = "ready"
	marketplaceMetadataUnavailable = "metadata_unavailable"
	marketplaceMetadataWriteFailed = "metadata_write_failed"
)

var defaultModelMarketplaceSources = []modelMarketplaceSourceConfig{
	{ID: "international", Label: "国际模型", Endpoint: modelMarketplaceEndpoint, LandingURL: modelMarketplacePricingURL},
}

type ModelMarketplaceView struct {
	Sources []ModelMarketplaceSource `json:"sources"`
}

type ModelMarketplaceSource struct {
	ID                  string                  `json:"id"`
	Label               string                  `json:"label"`
	SourceURL           string                  `json:"sourceUrl"`
	PricingURL          string                  `json:"pricingUrl"`
	PricingVersion      string                  `json:"pricingVersion,omitempty"`
	PricingState        string                  `json:"pricingState"`
	PricingAuthenticity string                  `json:"pricingAuthenticity,omitempty"`
	StatusAuthenticity  string                  `json:"statusAuthenticity,omitempty"`
	FetchedAt           string                  `json:"fetchedAt,omitempty"`
	StatusFetchedAt     string                  `json:"statusFetchedAt,omitempty"`
	ExchangeRate        *string                 `json:"exchangeRate,omitempty"`
	RechargePrice       *string                 `json:"rechargePrice,omitempty"`
	MetadataStatus      string                  `json:"metadataStatus"`
	Partial             bool                    `json:"partial,omitempty"`
	Models              []ModelMarketplaceModel `json:"models"`
	ErrorCode           string                  `json:"errorCode,omitempty"`
}

type ModelMarketplaceModel struct {
	ID           string                        `json:"id"`
	Name         string                        `json:"name"`
	Icon         string                        `json:"icon,omitempty"`
	Vendor       string                        `json:"vendor,omitempty"`
	VendorIcon   string                        `json:"vendorIcon,omitempty"`
	Summary      string                        `json:"summary,omitempty"`
	Tags         []string                      `json:"tags,omitempty"`
	Groups       []string                      `json:"groups,omitempty"`
	Endpoints    []string                      `json:"endpoints,omitempty"`
	BillingType  string                        `json:"billingType,omitempty"`
	QuotaType    *int                          `json:"quotaType,omitempty"`
	Prices       []ModelMarketplacePriceFact   `json:"prices,omitempty"`
	PricingTiers []ModelMarketplacePricingTier `json:"pricingTiers,omitempty"`
}

type ModelMarketplacePriceFact struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ModelMarketplacePricingTier struct {
	MaxTokens *string                     `json:"maxTokens,omitempty"`
	Prices    []ModelMarketplacePriceFact `json:"prices,omitempty"`
}

type modelMarketplaceSourceConfig struct {
	ID         string
	Label      string
	Endpoint   string
	LandingURL string
}

type modelMarketplaceCache interface {
	Read(modelMarketplaceSourceConfig) (modelMarketplaceCacheFile, error)
	Write(modelMarketplaceCacheFile) error
}

type modelMarketplaceService struct {
	cache          modelMarketplaceCache
	transport      newapipricing.Transport
	sources        []modelMarketplaceSourceConfig
	refreshTimeout time.Duration
	now            func() time.Time
}

type modelMarketplaceDiskCache struct {
	dir       string
	writeFile func(string, []byte, os.FileMode) error
}

type modelMarketplaceCacheFile struct {
	Version        int                     `json:"version"`
	SourceID       string                  `json:"source_id"`
	SourceURL      string                  `json:"source_url"`
	PricingVersion string                  `json:"pricing_version,omitempty"`
	FetchedAt      time.Time               `json:"fetched_at"`
	Partial        bool                    `json:"partial,omitempty"`
	Models         []ModelMarketplaceModel `json:"models"`
}

type airClaudePricingEnvelope struct {
	Success        bool                     `json:"success"`
	PricingVersion string                   `json:"pricing_version"`
	Vendors        []airClaudePricingVendor `json:"vendors"`
	Data           []airClaudePricingModel  `json:"data"`
}

type airClaudePricingVendor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type airClaudePricingModel struct {
	ModelName              string                 `json:"model_name"`
	Description            string                 `json:"description"`
	Icon                   string                 `json:"icon"`
	VendorID               *int                   `json:"vendor_id"`
	BillingType            string                 `json:"billing_type"`
	QuotaType              *int                   `json:"quota_type"`
	Tags                   string                 `json:"tags"`
	EnableGroups           []string               `json:"enable_groups"`
	SupportedEndpointTypes []string               `json:"supported_endpoint_types"`
	InputPrice             *json.Number           `json:"input_price"`
	OutputPrice            *json.Number           `json:"output_price"`
	CacheReadPrice         *json.Number           `json:"cache_read_price"`
	CacheWrite5mPrice      *json.Number           `json:"cache_write_5m_price"`
	CacheWrite1hPrice      *json.Number           `json:"cache_write_1h_price"`
	RequestPrice           *json.Number           `json:"request_price"`
	ImagePrice             *json.Number           `json:"image_price"`
	OutputImagePrice       *json.Number           `json:"output_image_price"`
	AudioInputPrice        *json.Number           `json:"audio_input_price"`
	AudioOutputPrice       *json.Number           `json:"audio_output_price"`
	SearchPrice            *json.Number           `json:"search_price"`
	ContextTieredPrice     []airClaudePricingTier `json:"context_tiered_price"`
}

type airClaudePricingTier struct {
	MaxTokens         *json.Number `json:"max_tokens"`
	InputPrice        *json.Number `json:"input_price"`
	OutputPrice       *json.Number `json:"output_price"`
	CacheReadPrice    *json.Number `json:"cache_read_price"`
	CacheWrite5mPrice *json.Number `json:"cache_write_5m_price"`
	CacheWrite1hPrice *json.Number `json:"cache_write_1h_price"`
}

func newDefaultPricingDependencies() (*newapipricing.Reader, *newapipricing.PricePreparationReader, *modelMarketplaceService, error) {
	cache, err := newapipricing.NewDefaultCache()
	if err != nil {
		return nil, nil, nil, err
	}
	reader, err := newapipricing.NewReader(cache)
	if err != nil {
		return nil, nil, nil, err
	}
	preparation, err := newapipricing.NewPricePreparationReader(reader)
	if err != nil {
		return nil, nil, nil, err
	}
	marketplaceCache, err := newModelMarketplaceDiskCache(newapipricing.DefaultCacheDir())
	if err != nil {
		return nil, nil, nil, err
	}
	client := &http.Client{Timeout: modelMarketplaceTimeout}
	service := &modelMarketplaceService{
		cache: marketplaceCache, transport: newapipricing.NewHTTPTransport(client),
		sources:        append([]modelMarketplaceSourceConfig(nil), defaultModelMarketplaceSources...),
		refreshTimeout: modelMarketplaceTimeout, now: time.Now,
	}
	return reader, preparation, service, nil
}

// ModelMarketplace is cache-only. RefreshModelMarketplace is the sole bound
// method that can access the AirClaude pricing endpoint.
func (a *App) ModelMarketplace() (ModelMarketplaceView, error) {
	if a == nil || a.modelMarketplace == nil {
		return unavailableModelMarketplaceView(defaultModelMarketplaceSources), nil
	}
	return a.modelMarketplace.Read(), nil
}

func (a *App) RefreshModelMarketplace() (ModelMarketplaceView, error) {
	if a == nil || a.modelMarketplace == nil {
		return unavailableModelMarketplaceView(defaultModelMarketplaceSources), nil
	}
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return a.modelMarketplace.Refresh(ctx), nil
}

func (s *modelMarketplaceService) Read() ModelMarketplaceView {
	view := ModelMarketplaceView{Sources: make([]ModelMarketplaceSource, 0, len(s.sources))}
	for _, config := range s.sources {
		cached, err := s.cache.Read(config)
		if err != nil {
			view.Sources = append(view.Sources, modelMarketplaceUnavailableSource(config, err))
			continue
		}
		view.Sources = append(view.Sources, modelMarketplaceSourceFromCache(config, cached, s.currentTime()))
	}
	return view
}

func (s *modelMarketplaceService) Refresh(ctx context.Context) ModelMarketplaceView {
	view := ModelMarketplaceView{Sources: make([]ModelMarketplaceSource, len(s.sources))}
	var wait sync.WaitGroup
	for index, config := range s.sources {
		wait.Add(1)
		go func() {
			defer wait.Done()
			view.Sources[index] = s.refreshSource(ctx, config)
		}()
	}
	wait.Wait()
	return view
}

func (s *modelMarketplaceService) refreshSource(ctx context.Context, config modelMarketplaceSourceConfig) ModelMarketplaceSource {
	timeout := s.refreshTimeout
	if timeout <= 0 {
		timeout = modelMarketplaceTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body, err := s.transport.Fetch(ctx, config.Endpoint)
	if err != nil {
		return s.cachedOrUnavailable(config, err)
	}
	cached, err := buildAirClaudeMarketplaceCache(config, body, s.currentTime())
	if err != nil {
		return s.cachedOrUnavailable(config, err)
	}
	source := modelMarketplaceSourceFromCache(config, cached, s.currentTime())
	if err := s.cache.Write(cached); err != nil {
		source.MetadataStatus = marketplaceMetadataWriteFailed
	}
	return source
}

func (s *modelMarketplaceService) cachedOrUnavailable(config modelMarketplaceSourceConfig, cause error) ModelMarketplaceSource {
	cached, err := s.cache.Read(config)
	if err != nil {
		return modelMarketplaceUnavailableSource(config, cause)
	}
	source := modelMarketplaceSourceFromCache(config, cached, s.currentTime())
	source.ErrorCode = marketplaceErrorCode(cause)
	return source
}

func (s *modelMarketplaceService) currentTime() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func buildAirClaudeMarketplaceCache(config modelMarketplaceSourceConfig, data []byte, fetchedAt time.Time) (modelMarketplaceCacheFile, error) {
	envelope, err := decodeAirClaudePricing(data)
	if err != nil {
		return modelMarketplaceCacheFile{}, err
	}
	models, partial := projectAirClaudeModels(envelope)
	cached := modelMarketplaceCacheFile{
		Version: modelMarketplaceCacheVersion, SourceID: strings.TrimSpace(config.ID),
		SourceURL: strings.TrimSpace(config.Endpoint), PricingVersion: strings.TrimSpace(envelope.PricingVersion),
		FetchedAt: fetchedAt.UTC(), Partial: partial, Models: models,
	}
	if err := validateModelMarketplaceCache(config, cached); err != nil {
		return modelMarketplaceCacheFile{}, err
	}
	return cached, nil
}

func decodeAirClaudePricing(data []byte) (airClaudePricingEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var envelope airClaudePricingEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return airClaudePricingEnvelope{}, fmt.Errorf("decode AirClaude pricing: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return airClaudePricingEnvelope{}, errors.New("AirClaude pricing contains multiple JSON values")
		}
		return airClaudePricingEnvelope{}, fmt.Errorf("decode AirClaude pricing trailer: %w", err)
	}
	if !envelope.Success {
		return airClaudePricingEnvelope{}, errors.New("AirClaude pricing response was not successful")
	}
	return envelope, nil
}

func projectAirClaudeModels(envelope airClaudePricingEnvelope) ([]ModelMarketplaceModel, bool) {
	vendors := make(map[int]airClaudePricingVendor, len(envelope.Vendors))
	for _, vendor := range envelope.Vendors {
		vendors[vendor.ID] = vendor
	}
	models := make([]ModelMarketplaceModel, 0, len(envelope.Data))
	seen := make(map[string]bool, len(envelope.Data))
	partial := false
	for _, item := range envelope.Data {
		name := strings.TrimSpace(item.ModelName)
		if name == "" || seen[name] {
			partial = true
			continue
		}
		model := ModelMarketplaceModel{
			ID: name, Name: name, Icon: strings.TrimSpace(item.Icon), Summary: strings.TrimSpace(item.Description),
			Tags: splitAirClaudeTags(item.Tags), Groups: cleanModelMarketplaceStrings(item.EnableGroups),
			Endpoints:   cleanModelMarketplaceStrings(item.SupportedEndpointTypes),
			BillingType: strings.TrimSpace(item.BillingType), QuotaType: cloneInt(item.QuotaType),
			Prices: airClaudePriceFacts(item.InputPrice, item.OutputPrice, item.CacheReadPrice,
				item.CacheWrite5mPrice, item.CacheWrite1hPrice, item.RequestPrice, item.ImagePrice,
				item.OutputImagePrice, item.AudioInputPrice, item.AudioOutputPrice, item.SearchPrice),
			PricingTiers: projectAirClaudePricingTiers(item.ContextTieredPrice),
		}
		if item.VendorID != nil {
			if vendor, ok := vendors[*item.VendorID]; ok {
				model.Vendor = strings.TrimSpace(vendor.Name)
				model.VendorIcon = strings.TrimSpace(vendor.Icon)
			}
		}
		models = append(models, model)
		seen[name] = true
	}
	sortModelMarketplaceModels(models)
	return models, partial
}

func sortModelMarketplaceModels(models []ModelMarketplaceModel) {
	sort.Slice(models, func(i, j int) bool {
		left, right := strings.ToLower(models[i].Name), strings.ToLower(models[j].Name)
		if left == right {
			return models[i].Name < models[j].Name
		}
		return left < right
	})
}

func airClaudePriceFacts(values ...*json.Number) []ModelMarketplacePriceFact {
	keys := []string{
		"input_price", "output_price", "cache_read_price", "cache_write_5m_price", "cache_write_1h_price",
		"request_price", "image_price", "output_image_price", "audio_input_price", "audio_output_price", "search_price",
	}
	facts := make([]ModelMarketplacePriceFact, 0, len(values))
	for index, value := range values {
		if value == nil || index >= len(keys) {
			continue
		}
		facts = append(facts, ModelMarketplacePriceFact{Key: keys[index], Value: value.String()})
	}
	return facts
}

func projectAirClaudePricingTiers(values []airClaudePricingTier) []ModelMarketplacePricingTier {
	tiers := make([]ModelMarketplacePricingTier, 0, len(values))
	for _, item := range values {
		tier := ModelMarketplacePricingTier{
			MaxTokens: numberString(item.MaxTokens),
			Prices: airClaudePriceFacts(item.InputPrice, item.OutputPrice, item.CacheReadPrice,
				item.CacheWrite5mPrice, item.CacheWrite1hPrice),
		}
		if tier.MaxTokens != nil || len(tier.Prices) > 0 {
			tiers = append(tiers, tier)
		}
	}
	return tiers
}

func numberString(value *json.Number) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func splitAirClaudeTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return cleanModelMarketplaceStrings(strings.Split(value, ","))
}

func newModelMarketplaceDiskCache(dir string) (*modelMarketplaceDiskCache, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("model marketplace cache directory is required")
	}
	return &modelMarketplaceDiskCache{dir: dir, writeFile: fileutil.AtomicWriteFileStrict}, nil
}

func (c *modelMarketplaceDiskCache) Read(config modelMarketplaceSourceConfig) (modelMarketplaceCacheFile, error) {
	path, err := c.path(config.ID)
	if err != nil {
		return modelMarketplaceCacheFile{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return modelMarketplaceCacheFile{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cached modelMarketplaceCacheFile
	if err := decoder.Decode(&cached); err != nil {
		return modelMarketplaceCacheFile{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return modelMarketplaceCacheFile{}, errors.New("model marketplace cache contains multiple JSON values")
	}
	if err := validateModelMarketplaceCache(config, cached); err != nil {
		return modelMarketplaceCacheFile{}, err
	}
	return cached, nil
}

func (c *modelMarketplaceDiskCache) Write(cached modelMarketplaceCacheFile) error {
	config := modelMarketplaceSourceConfig{ID: cached.SourceID, Endpoint: cached.SourceURL}
	if err := validateModelMarketplaceCache(config, cached); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path, err := c.path(cached.SourceID)
	if err != nil {
		return err
	}
	return c.writeFile(path, data, 0o600)
}

func (c *modelMarketplaceDiskCache) path(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" || strings.ContainsAny(source, `/\\`) {
		return "", errors.New("model marketplace source is invalid")
	}
	return filepath.Join(c.dir, "marketplace-"+source+"-v1.json"), nil
}

func validateModelMarketplaceCache(config modelMarketplaceSourceConfig, cached modelMarketplaceCacheFile) error {
	if cached.Version != modelMarketplaceCacheVersion || cached.SourceID != strings.TrimSpace(config.ID) ||
		cached.SourceURL != strings.TrimSpace(config.Endpoint) || cached.FetchedAt.IsZero() {
		return errors.New("model marketplace cache identity is invalid")
	}
	for _, model := range cached.Models {
		if strings.TrimSpace(model.ID) == "" || strings.TrimSpace(model.Name) == "" {
			return errors.New("model marketplace cache model identity is invalid")
		}
		if err := validateMarketplacePriceFacts(model.Prices); err != nil {
			return err
		}
		for _, tier := range model.PricingTiers {
			if tier.MaxTokens != nil && strings.TrimSpace(*tier.MaxTokens) == "" {
				return errors.New("model marketplace tier max_tokens is invalid")
			}
			if err := validateMarketplacePriceFacts(tier.Prices); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMarketplacePriceFacts(facts []ModelMarketplacePriceFact) error {
	for _, fact := range facts {
		if strings.TrimSpace(fact.Key) == "" || strings.TrimSpace(fact.Value) == "" {
			return errors.New("model marketplace price fact is invalid")
		}
	}
	return nil
}

func modelMarketplaceSourceFromCache(config modelMarketplaceSourceConfig, cached modelMarketplaceCacheFile, now time.Time) ModelMarketplaceSource {
	source := modelMarketplaceSourceBase(config)
	source.PricingVersion = cached.PricingVersion
	source.PricingState = string(newapipricing.DefaultPricingStateAt(cached.FetchedAt, now))
	source.FetchedAt = cached.FetchedAt.UTC().Format(time.RFC3339Nano)
	source.MetadataStatus = marketplaceMetadataReady
	source.Partial = cached.Partial
	source.Models = append([]ModelMarketplaceModel(nil), cached.Models...)
	sortModelMarketplaceModels(source.Models)
	return source
}

func unavailableModelMarketplaceView(configs []modelMarketplaceSourceConfig) ModelMarketplaceView {
	view := ModelMarketplaceView{Sources: make([]ModelMarketplaceSource, 0, len(configs))}
	for _, config := range configs {
		view.Sources = append(view.Sources, modelMarketplaceUnavailableSource(config, nil))
	}
	return view
}

func modelMarketplaceUnavailableSource(config modelMarketplaceSourceConfig, err error) ModelMarketplaceSource {
	source := modelMarketplaceSourceBase(config)
	source.PricingState = string(modelbilling.PricingUnavailable)
	source.MetadataStatus = marketplaceMetadataUnavailable
	source.ErrorCode = marketplaceErrorCode(err)
	return source
}

func marketplaceErrorCode(err error) string {
	if code := modelbilling.ErrorCodeOf(err); code != "" {
		return string(code)
	}
	return string(modelbilling.ErrorPricingUnavailable)
}

func modelMarketplaceSourceBase(config modelMarketplaceSourceConfig) ModelMarketplaceSource {
	pricingURL := strings.TrimSpace(config.LandingURL)
	if pricingURL == "" {
		pricingURL = modelMarketplacePricingURL
	}
	return ModelMarketplaceSource{
		ID: strings.TrimSpace(config.ID), Label: strings.TrimSpace(config.Label),
		SourceURL: strings.TrimSpace(config.Endpoint), PricingURL: pricingURL,
		Models: []ModelMarketplaceModel{},
	}
}

func cleanModelMarketplaceStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
