package modelbilling

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/provider"
)

const (
	FixtureVersionB0V1          = "b0-v1"
	DependencyNewAPIPricing     = "NEWAPI-PRICING-01"
	DependencyNewAPIStatus      = "NEWAPI-STATUS-01"
	DependencyCloudGateway      = "CLOUD-GATEWAY-01"
	DependencyCloudUsage        = "CLOUD-USAGE-01"
	DependencyCloudSettlement   = "CLOUD-SETTLEMENT-01"
	DependencyCloudCreditConfig = "CLOUD-CREDIT-CONFIG-01"
)

type CreditConfigFixtureBundle struct {
	Version      string                    `json:"fixture_version"`
	DependencyID string                    `json:"dependency_id"`
	Authenticity string                    `json:"authenticity"`
	Cases        []CreditConfigFixtureCase `json:"cases"`
}
type CreditConfigFixtureCase struct {
	Name   string        `json:"name"`
	Config *CreditConfig `json:"config,omitempty"`
	Error  *FixtureError `json:"error,omitempty"`
}

func DecodeCreditConfigFixture(reader io.Reader) (CreditConfigFixtureBundle, error) {
	bundle, err := decodeStrict[CreditConfigFixtureBundle](reader)
	if err != nil {
		return bundle, err
	}
	if err := validateFixtureHeader(bundle.Version, bundle.DependencyID, DependencyCloudCreditConfig, len(bundle.Cases)); err != nil {
		return CreditConfigFixtureBundle{}, err
	}
	if bundle.Authenticity != "MOCK_ONLY" {
		return CreditConfigFixtureBundle{}, fmt.Errorf("modelbilling: invalid credit config fixture authenticity")
	}
	for i, c := range bundle.Cases {
		if err := validateFixtureOutcome(c.Name, c.Config != nil, c.Error); err != nil {
			return CreditConfigFixtureBundle{}, fixtureCaseError(i, err)
		}
		if c.Config != nil {
			if err := c.Config.Validate(); err != nil {
				return CreditConfigFixtureBundle{}, fixtureCaseError(i, err)
			}
		}
	}
	return bundle, nil
}

type FixtureCreditConfigProvider struct{ bundle CreditConfigFixtureBundle }

func NewFixtureCreditConfigProvider(bundle CreditConfigFixtureBundle) (*FixtureCreditConfigProvider, error) {
	if err := validateCreditBundle(bundle); err != nil {
		return nil, err
	}
	return &FixtureCreditConfigProvider{bundle: cloneCreditConfigFixtureBundle(bundle)}, nil
}
func validateCreditBundle(bundle CreditConfigFixtureBundle) error {
	payload, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	_, err = DecodeCreditConfigFixture(strings.NewReader(string(payload)))
	return err
}
func (p *FixtureCreditConfigProvider) GetCreditConfig(_ context.Context, scope cloud.BindingScope) (CreditConfig, error) {
	if p == nil || len(p.bundle.Cases) == 0 {
		return CreditConfig{}, newError(ErrorContractInvalid, "credit config fixture is unavailable")
	}
	if err := scope.Validate(); err != nil {
		return CreditConfig{}, newError(ErrorScopeMismatch, "credit config fixture scope is invalid")
	}
	c := p.bundle.Cases[0]
	if c.Error != nil {
		return CreditConfig{}, c.Error.domainError()
	}
	return c.ConfigClone(), nil
}
func (p *FixtureCreditConfigProvider) Authenticity() string {
	if p == nil {
		return ""
	}
	return p.bundle.Authenticity
}
func (c CreditConfigFixtureCase) ConfigClone() CreditConfig {
	out := *c.Config
	if c.Config.Version != nil {
		v := *c.Config.Version
		out.Version = &v
	}
	return out
}

func cloneCreditConfigFixtureBundle(in CreditConfigFixtureBundle) CreditConfigFixtureBundle {
	out := in
	out.Cases = make([]CreditConfigFixtureCase, len(in.Cases))
	for i, c := range in.Cases {
		out.Cases[i] = c
		if c.Config != nil {
			config := c.ConfigClone()
			out.Cases[i].Config = &config
		}
		if c.Error != nil {
			fixtureErr := *c.Error
			out.Cases[i].Error = &fixtureErr
		}
	}
	return out
}

type FixtureError struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable,omitempty"`
}

func (e *FixtureError) Validate() error {
	if e == nil {
		return nil
	}
	if err := e.Code.Validate(); err != nil {
		return err
	}
	return requireTrimmed("fixture error message", e.Message)
}

func (e *FixtureError) domainError() error {
	if e == nil {
		return nil
	}
	return &Error{Code: e.Code, Message: e.Message, Retryable: e.Retryable}
}

type PricingFixtureBundle struct {
	Version      string               `json:"fixture_version"`
	DependencyID string               `json:"dependency_id"`
	Cases        []PricingFixtureCase `json:"cases"`
}

type PricingFixtureCase struct {
	Name     string           `json:"name"`
	Snapshot *PricingSnapshot `json:"snapshot,omitempty"`
	Error    *FixtureError    `json:"error,omitempty"`
}

type StatusFixtureBundle struct {
	Version      string              `json:"fixture_version"`
	DependencyID string              `json:"dependency_id"`
	Cases        []StatusFixtureCase `json:"cases"`
}

type StatusFixtureCase struct {
	Name   string               `json:"name"`
	Status *PricingSourceStatus `json:"status,omitempty"`
	Error  *FixtureError        `json:"error,omitempty"`
}

type GatewayFixtureBundle struct {
	Version      string               `json:"fixture_version"`
	DependencyID string               `json:"dependency_id"`
	Scope        cloud.BindingScope   `json:"scope"`
	Cases        []GatewayFixtureCase `json:"cases"`
}

type GatewayResponseMode string

const (
	GatewayResponseStreaming    GatewayResponseMode = "streaming"
	GatewayResponseNonStreaming GatewayResponseMode = "non_streaming"
)

type GatewayFixtureCase struct {
	Name         string                `json:"name"`
	Endpoint     MaasEndpoint          `json:"endpoint"`
	ResponseMode GatewayResponseMode   `json:"response_mode"`
	Request      GatewayRequest        `json:"request"`
	Chunks       []GatewayFixtureChunk `json:"chunks,omitempty"`
	Error        *FixtureError         `json:"error,omitempty"`
}

type GatewayFixtureChunk struct {
	Type              string                   `json:"type"`
	Text              string                   `json:"text,omitempty"`
	Usage             *NormalizedUsage         `json:"usage,omitempty"`
	ProviderRequestID string                   `json:"provider_request_id,omitempty"`
	RawUsageRedacted  json.RawMessage          `json:"raw_usage_redacted,omitempty"`
	QuotaObservation  *GatewayQuotaObservation `json:"quota_observation,omitempty"`
	Error             *FixtureError            `json:"error,omitempty"`
}

// GatewayQuotaObservation is the normalized fake boundary for the raw Maas
// response facts used by quota settlement. It deliberately models HTTP facts,
// not a precomputed quota projection.
type GatewayQuotaObservation struct {
	Headers    map[string][]string `json:"headers"`
	FinalJSON  json.RawMessage     `json:"final_json"`
	ObservedAt time.Time           `json:"observed_at"`
}

type UsageFixtureBundle struct {
	Version      string             `json:"fixture_version"`
	DependencyID string             `json:"dependency_id"`
	Scope        cloud.BindingScope `json:"scope"`
	Cases        []UsageFixtureCase `json:"cases"`
}

type UsageFixtureCase struct {
	Name       string              `json:"name"`
	Partial    *PartialUsageReport `json:"partial,omitempty"`
	Final      *FinalUsageReport   `json:"final,omitempty"`
	Settlement *Settlement         `json:"settlement,omitempty"`
	Error      *FixtureError       `json:"error,omitempty"`
}

type SettlementFixtureBundle struct {
	Version      string                  `json:"fixture_version"`
	DependencyID string                  `json:"dependency_id"`
	Scope        cloud.BindingScope      `json:"scope"`
	Cases        []SettlementFixtureCase `json:"cases"`
}

type SettlementFixtureCase struct {
	Name       string        `json:"name"`
	TaskID     string        `json:"task_id"`
	Settlement *Settlement   `json:"settlement,omitempty"`
	Error      *FixtureError `json:"error,omitempty"`
}

func DecodePricingFixture(reader io.Reader) (PricingFixtureBundle, error) {
	bundle, err := decodeStrict[PricingFixtureBundle](reader)
	if err != nil {
		return bundle, err
	}
	if err := validateFixtureHeader(bundle.Version, bundle.DependencyID, DependencyNewAPIPricing, len(bundle.Cases)); err != nil {
		return PricingFixtureBundle{}, err
	}
	for i, fixture := range bundle.Cases {
		if err := validateFixtureOutcome(fixture.Name, fixture.Snapshot != nil, fixture.Error); err != nil {
			return PricingFixtureBundle{}, fixtureCaseError(i, err)
		}
		if fixture.Snapshot != nil {
			if err := fixture.Snapshot.Validate(); err != nil {
				return PricingFixtureBundle{}, fixtureCaseError(i, err)
			}
		}
	}
	return bundle, nil
}

func DecodeStatusFixture(reader io.Reader) (StatusFixtureBundle, error) {
	bundle, err := decodeStrict[StatusFixtureBundle](reader)
	if err != nil {
		return bundle, err
	}
	if err := validateFixtureHeader(bundle.Version, bundle.DependencyID, DependencyNewAPIStatus, len(bundle.Cases)); err != nil {
		return StatusFixtureBundle{}, err
	}
	for i, fixture := range bundle.Cases {
		if err := validateFixtureOutcome(fixture.Name, fixture.Status != nil, fixture.Error); err != nil {
			return StatusFixtureBundle{}, fixtureCaseError(i, err)
		}
		if fixture.Status != nil {
			if err := fixture.Status.Validate(); err != nil {
				return StatusFixtureBundle{}, fixtureCaseError(i, err)
			}
		}
	}
	return bundle, nil
}

func DecodeGatewayFixture(reader io.Reader) (GatewayFixtureBundle, error) {
	bundle, err := decodeStrict[GatewayFixtureBundle](reader)
	if err != nil {
		return bundle, err
	}
	if err := validateGatewayFixtureBundle(bundle); err != nil {
		return GatewayFixtureBundle{}, err
	}
	return bundle, nil
}

func validateGatewayFixtureBundle(bundle GatewayFixtureBundle) error {
	if err := validateFixtureHeader(bundle.Version, bundle.DependencyID, DependencyCloudGateway, len(bundle.Cases)); err != nil {
		return err
	}
	if err := bundle.Scope.Validate(); err != nil {
		return err
	}
	seenNames := map[string]bool{}
	seenCalls := map[string]bool{}
	for i, fixture := range bundle.Cases {
		if err := validateGatewayFixtureCase(fixture, seenNames, seenCalls); err != nil {
			return fixtureCaseError(i, err)
		}
	}
	return nil
}

func validateGatewayFixtureCase(fixture GatewayFixtureCase, seenNames, seenCalls map[string]bool) error {
	if err := validateGatewayFixtureName(fixture.Name, seenNames); err != nil {
		return err
	}
	if err := fixture.Endpoint.Validate(); err != nil {
		return err
	}
	if fixture.ResponseMode != GatewayResponseStreaming && fixture.ResponseMode != GatewayResponseNonStreaming {
		return newError(ErrorContractInvalid, "gateway fixture response_mode is invalid")
	}
	if err := validateGatewayFixtureRequest(fixture.Request, seenCalls); err != nil {
		return err
	}
	if (fixture.Error == nil) == (len(fixture.Chunks) == 0) {
		return fmt.Errorf("define chunks or start error")
	}
	if err := fixture.Error.Validate(); err != nil {
		return err
	}
	return validateGatewayFixtureChunks(fixture.Endpoint, fixture.Chunks)
}

func validateGatewayFixtureName(name string, seen map[string]bool) error {
	if err := requireTrimmed("fixture case name", name); err != nil {
		return err
	}
	if seen[name] {
		return fmt.Errorf("duplicate fixture case name")
	}
	seen[name] = true
	return nil
}

func validateGatewayFixtureRequest(request GatewayRequest, seen map[string]bool) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if seen[request.CallID] {
		return fmt.Errorf("duplicate call_id")
	}
	seen[request.CallID] = true
	return nil
}

func validateGatewayFixtureChunks(endpoint MaasEndpoint, chunks []GatewayFixtureChunk) error {
	terminalChunks := 0
	for i, chunk := range chunks {
		if err := chunk.validateForEndpoint(endpoint); err != nil {
			return err
		}
		if gatewayFixtureChunkIsTerminal(chunk) {
			terminalChunks++
			if i != len(chunks)-1 {
				return newError(ErrorContractInvalid, "gateway fixture terminal chunk must be last")
			}
		}
	}
	if len(chunks) != 0 && terminalChunks != 1 {
		return newError(ErrorContractInvalid, "gateway fixture requires exactly one terminal chunk")
	}
	return nil
}

func gatewayFixtureChunkIsTerminal(chunk GatewayFixtureChunk) bool {
	return chunk.Type == "done" || chunk.Type == "error"
}

func DecodeUsageFixture(reader io.Reader) (UsageFixtureBundle, error) {
	bundle, err := decodeStrict[UsageFixtureBundle](reader)
	if err != nil {
		return bundle, err
	}
	if err := validateUsageFixtureBundle(bundle); err != nil {
		return UsageFixtureBundle{}, err
	}
	return bundle, nil
}

func validateUsageFixtureBundle(bundle UsageFixtureBundle) error {
	if err := validateFixtureHeader(bundle.Version, bundle.DependencyID, DependencyCloudUsage, len(bundle.Cases)); err != nil {
		return err
	}
	if err := bundle.Scope.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, fixture := range bundle.Cases {
		if err := validateUsageFixtureCase(fixture, bundle.Scope); err != nil {
			return fixtureCaseError(i, err)
		}
		key := usageFixtureKey(fixture)
		if seen[key] {
			return fixtureCaseError(i, fmt.Errorf("duplicate idempotency key"))
		}
		seen[key] = true
	}
	return nil
}

func usageFixtureKey(fixture UsageFixtureCase) string {
	if fixture.Partial != nil {
		return fixture.Partial.IdempotencyKey
	}
	return fixture.Final.IdempotencyKey
}

func validateUsageFixtureCase(fixture UsageFixtureCase, scope cloud.BindingScope) error {
	if err := requireTrimmed("fixture case name", fixture.Name); err != nil {
		return err
	}
	if (fixture.Partial == nil) == (fixture.Final == nil) {
		return fmt.Errorf("define exactly one partial or final report")
	}
	if fixture.Partial != nil {
		return validatePartialFixtureCase(fixture, scope)
	}
	return validateFinalFixtureCase(fixture, scope)
}

func validatePartialFixtureCase(fixture UsageFixtureCase, scope cloud.BindingScope) error {
	if err := fixture.Partial.Validate(); err != nil {
		return err
	}
	if err := validateDeliverableUsageFixtureCalls(fixture.Partial.Calls); err != nil {
		return err
	}
	if fixture.Partial.Scope != scope || fixture.Settlement != nil {
		return fmt.Errorf("partial scope or settlement is invalid")
	}
	return fixture.Error.Validate()
}

func validateFinalFixtureCase(fixture UsageFixtureCase, scope cloud.BindingScope) error {
	if err := fixture.Final.Validate(); err != nil {
		return err
	}
	if err := validateDeliverableUsageFixtureCalls(fixture.Final.Calls); err != nil {
		return err
	}
	if fixture.Final.Scope != scope {
		return fmt.Errorf("final scope does not match bundle")
	}
	if (fixture.Settlement == nil) == (fixture.Error == nil) {
		return fmt.Errorf("final must define settlement or error")
	}
	if fixture.Settlement != nil {
		if fixture.Settlement.TaskID != fixture.Final.TaskID {
			return fmt.Errorf("settlement task_id does not match final task_id")
		}
		return fixture.Settlement.Validate()
	}
	return fixture.Error.Validate()
}

func validateDeliverableUsageFixtureCalls(calls []ModelCallUsage) error {
	const legacyFixtureCallLimit = 256
	if len(calls) == 0 || len(calls) > legacyFixtureCallLimit {
		return fmt.Errorf("legacy usage fixture report requires 1..%d calls", legacyFixtureCallLimit)
	}
	for _, call := range calls {
		if err := call.Usage.ValidateForReport(); err != nil {
			return err
		}
		if call.Usage.BreakdownStatus != UsageExact || call.Usage.RequestCount == nil || *call.Usage.RequestCount <= 0 {
			return fmt.Errorf("legacy usage fixture call is not deliverable")
		}
	}
	return nil
}

func DecodeSettlementFixture(reader io.Reader) (SettlementFixtureBundle, error) {
	bundle, err := decodeStrict[SettlementFixtureBundle](reader)
	if err != nil {
		return bundle, err
	}
	if err := validateSettlementFixtureBundle(bundle); err != nil {
		return SettlementFixtureBundle{}, err
	}
	return bundle, nil
}

func validateSettlementFixtureBundle(bundle SettlementFixtureBundle) error {
	if err := validateFixtureHeader(bundle.Version, bundle.DependencyID, DependencyCloudSettlement, len(bundle.Cases)); err != nil {
		return err
	}
	if err := bundle.Scope.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, fixture := range bundle.Cases {
		if err := validateFixtureOutcome(fixture.Name, fixture.Settlement != nil, fixture.Error); err != nil {
			return fixtureCaseError(i, err)
		}
		if err := requireTrimmed("task_id", fixture.TaskID); err != nil {
			return fixtureCaseError(i, err)
		}
		if seen[fixture.TaskID] {
			return fixtureCaseError(i, fmt.Errorf("duplicate settlement task_id"))
		}
		seen[fixture.TaskID] = true
		if fixture.Settlement != nil {
			if fixture.Settlement.TaskID != fixture.TaskID {
				return fixtureCaseError(i, fmt.Errorf("settlement task_id mismatch"))
			}
			if err := fixture.Settlement.Validate(); err != nil {
				return fixtureCaseError(i, err)
			}
		}
	}
	return nil
}

func (c GatewayFixtureChunk) Validate() error {
	if err := c.validateBody(); err != nil {
		return err
	}
	if c.ProviderRequestID != "" {
		if err := requireTrimmed("provider_request_id", c.ProviderRequestID); err != nil {
			return err
		}
	}
	return ValidateRawUsageRedacted(c.RawUsageRedacted)
}

func (c GatewayFixtureChunk) validateForEndpoint(endpoint MaasEndpoint) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.QuotaObservation == nil {
		return nil
	}
	if c.Type != "done" {
		return newError(ErrorContractInvalid, "quota observation requires the final done chunk")
	}
	revisions, err := c.normalizedQuotaEvidenceRevisions(endpoint)
	if err != nil {
		return err
	}
	for i := range revisions {
		if c.ProviderRequestID != "" && revisions[i].OneAPIRequestID != nil && c.ProviderRequestID != *revisions[i].OneAPIRequestID {
			return newError(ErrorContractInvalid, "gateway fixture request ID conflicts with quota evidence")
		}
	}
	return nil
}

func (c GatewayFixtureChunk) validateBody() error {
	switch c.Type {
	case "text", "reasoning":
		if c.Text == "" {
			return fmt.Errorf("%s chunk requires text", c.Type)
		}
	case "usage":
		if c.Usage == nil {
			return fmt.Errorf("usage chunk requires usage")
		}
		if err := c.Usage.ValidateForReport(); err != nil {
			return err
		}
	case "done":
	case "error":
		if c.Error == nil {
			return fmt.Errorf("error chunk requires error")
		}
		if err := c.Error.Validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported gateway chunk type %q", c.Type)
	}
	return nil
}

func (c GatewayFixtureChunk) gatewayChunk(endpoint MaasEndpoint) (GatewayChunk, error) {
	chunk := c.providerChunk()
	usage := cloneGatewayFixtureUsage(c.Usage)
	requestID, evidence, evidenceRevisions, err := c.gatewayQuotaEvidence(endpoint)
	if err != nil {
		return GatewayChunk{}, err
	}
	return GatewayChunk{
		Chunk: chunk, Usage: usage, ProviderRequestID: requestID, RawUsageRedacted: append([]byte(nil), c.RawUsageRedacted...),
		QuotaEvidence: evidence, QuotaEvidenceRevisions: evidenceRevisions,
	}, nil
}

func (c GatewayFixtureChunk) providerChunk() provider.Chunk {
	chunk := provider.Chunk{}
	switch c.Type {
	case "text":
		chunk.Type, chunk.Text = provider.ChunkText, c.Text
	case "reasoning":
		chunk.Type, chunk.Text = provider.ChunkReasoning, c.Text
	case "usage":
		chunk.Type, chunk.Usage = provider.ChunkUsage, normalizedProviderUsage(*c.Usage)
	case "done":
		chunk.Type = provider.ChunkDone
	case "error":
		chunk.Type, chunk.Err = provider.ChunkError, c.Error.domainError()
	}
	return chunk
}

func cloneGatewayFixtureUsage(usage *NormalizedUsage) *NormalizedUsage {
	if usage == nil {
		return nil
	}
	cloned := *usage
	return &cloned
}

func (c GatewayFixtureChunk) gatewayQuotaEvidence(endpoint MaasEndpoint) (string, *QuotaEvidenceRevision, []QuotaEvidenceRevision, error) {
	revisions, err := c.normalizedQuotaEvidenceRevisions(endpoint)
	if err != nil {
		return "", nil, nil, err
	}
	if len(revisions) == 0 {
		return c.ProviderRequestID, nil, nil, nil
	}
	if len(revisions) == 1 {
		return c.singleGatewayQuotaEvidence(revisions[0])
	}
	return c.multipleGatewayQuotaEvidence(revisions)
}

func (c GatewayFixtureChunk) singleGatewayQuotaEvidence(revision QuotaEvidenceRevision) (string, *QuotaEvidenceRevision, []QuotaEvidenceRevision, error) {
	evidence := cloneQuotaEvidence(&revision)
	if evidence.OneAPIRequestID == nil {
		return c.ProviderRequestID, evidence, nil, nil
	}
	if c.ProviderRequestID != "" && c.ProviderRequestID != *evidence.OneAPIRequestID {
		return "", nil, nil, newError(ErrorContractInvalid, "gateway fixture request ID conflicts with quota evidence")
	}
	return *evidence.OneAPIRequestID, evidence, nil, nil
}

func (c GatewayFixtureChunk) multipleGatewayQuotaEvidence(revisions []QuotaEvidenceRevision) (string, *QuotaEvidenceRevision, []QuotaEvidenceRevision, error) {
	if c.ProviderRequestID != "" {
		return "", nil, nil, newError(ErrorContractInvalid, "gateway fixture request ID conflicts with quota evidence")
	}
	return "", nil, cloneQuotaEvidenceRevisions(revisions), nil
}

func (c GatewayFixtureChunk) normalizedQuotaEvidenceRevisions(endpoint MaasEndpoint) ([]QuotaEvidenceRevision, error) {
	if c.QuotaObservation == nil {
		return nil, nil
	}
	revisions, err := NormalizeQuotaEvidenceRevisions(QuotaObservationInput{
		Endpoint: endpoint, Headers: cloneHeaderValues(c.QuotaObservation.Headers),
		FinalJSON:  append(json.RawMessage(nil), c.QuotaObservation.FinalJSON...),
		ObservedAt: c.QuotaObservation.ObservedAt,
	})
	if err != nil {
		return nil, err
	}
	return cloneQuotaEvidenceRevisions(revisions), nil
}

func cloneHeaderValues(input map[string][]string) map[string][]string {
	if input == nil {
		return nil
	}
	output := make(map[string][]string, len(input))
	for name, values := range input {
		output[name] = append([]string(nil), values...)
	}
	return output
}

func normalizedProviderUsage(usage NormalizedUsage) *provider.Usage {
	return &provider.Usage{
		PromptTokens:     int(valueOrZero(usage.InputTokens)),
		CompletionTokens: int(valueOrZero(usage.OutputTokens)),
		TotalTokens:      int(valueOrZero(usage.ProviderTotalTokens)),
		CacheHitTokens:   int(valueOrZero(usage.CacheReadTokens)),
		CacheMissTokens:  int(valueOrZero(usage.NormalInputTokens)),
	}
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func decodeStrict[T any](reader io.Reader) (T, error) {
	var value T
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("modelbilling: decode fixture: %w", err)
	}
	var trailer any
	if err := decoder.Decode(&trailer); err != io.EOF {
		if err == nil {
			return value, fmt.Errorf("modelbilling: fixture contains multiple JSON values")
		}
		return value, fmt.Errorf("modelbilling: decode fixture trailer: %w", err)
	}
	return value, nil
}

func validateFixtureHeader(version, dependencyID, expected string, count int) error {
	if version != FixtureVersionB0V1 {
		return fmt.Errorf("modelbilling: unsupported fixture_version %q", version)
	}
	if dependencyID != expected {
		return fmt.Errorf("modelbilling: fixture dependency_id %q, want %q", dependencyID, expected)
	}
	if count == 0 {
		return fmt.Errorf("modelbilling: fixture cases are required")
	}
	return nil
}

func validateFixtureOutcome(name string, hasValue bool, fixtureErr *FixtureError) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("fixture case name is required")
	}
	if hasValue == (fixtureErr != nil) {
		return fmt.Errorf("fixture case %q must define exactly one value or error", name)
	}
	return fixtureErr.Validate()
}

func fixtureCaseError(index int, err error) error {
	return fmt.Errorf("modelbilling: fixture case %d: %w", index, err)
}
