package modelbilling

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"kwu/internal/cloud"
)

// The usage and settlement fixtures are embedded only for the explicit
// desktop strict/fake composition. They are not production cloud responses.
//
//go:embed testdata/CLOUD-USAGE-01/normalized-v1.json
var embeddedUsageFixture []byte

//go:embed testdata/CLOUD-SETTLEMENT-01/normalized-v1.json
var embeddedSettlementFixture []byte

func LoadEmbeddedUsageFixture() (UsageFixtureBundle, error) {
	return DecodeUsageFixture(bytes.NewReader(embeddedUsageFixture))
}

func LoadEmbeddedSettlementFixture() (SettlementFixtureBundle, error) {
	return DecodeSettlementFixture(bytes.NewReader(embeddedSettlementFixture))
}

type FixtureGateway struct {
	scope  cloud.BindingScope
	byCall map[string]GatewayFixtureCase
}

func NewFixtureGateway(bundle GatewayFixtureBundle) (*FixtureGateway, error) {
	if err := validateGatewayFixtureBundle(bundle); err != nil {
		return nil, err
	}
	byCall := make(map[string]GatewayFixtureCase, len(bundle.Cases))
	for _, fixture := range bundle.Cases {
		cloned, err := cloneFixture(fixture)
		if err != nil {
			return nil, err
		}
		byCall[fixture.Request.CallID] = cloned
	}
	return &FixtureGateway{scope: bundle.Scope, byCall: byCall}, nil
}

func (g *FixtureGateway) Stream(ctx context.Context, scope cloud.BindingScope, request GatewayRequest) (<-chan GatewayChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if scope != g.scope {
		return nil, newError(ErrorScopeMismatch, "gateway fixture belongs to another binding")
	}
	fixture, err := g.fixtureForRequest(request)
	if err != nil {
		return nil, err
	}
	if fixture.Error != nil {
		return nil, fixture.Error.domainError()
	}
	chunks := make([]GatewayChunk, len(fixture.Chunks))
	for i, fixtureChunk := range fixture.Chunks {
		chunk, err := fixtureChunk.gatewayChunk(fixture.Endpoint)
		if err != nil {
			return nil, err
		}
		chunks[i] = chunk
	}
	out := make(chan GatewayChunk)
	go func() {
		defer close(out)
		for _, chunk := range chunks {
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// EndpointFor exposes the endpoint declared by the normalized fixture before
// dispatcher write-ahead persistence. It never derives an endpoint from model
// or provider names.
func (g *FixtureGateway) EndpointFor(request GatewayRequest) (MaasEndpoint, error) {
	if g == nil {
		return "", newError(ErrorContractInvalid, "fixture gateway is required")
	}
	fixture, err := g.fixtureForRequest(request)
	if err != nil {
		return "", err
	}
	return fixture.Endpoint, nil
}

func (g *FixtureGateway) fixtureForRequest(request GatewayRequest) (GatewayFixtureCase, error) {
	if err := request.Validate(); err != nil {
		return GatewayFixtureCase{}, err
	}
	fixture, ok := g.byCall[request.CallID]
	if !ok {
		return GatewayFixtureCase{}, newError(ErrorContractInvalid, "gateway fixture call not found")
	}
	if !reflect.DeepEqual(fixture.Request, request) {
		return GatewayFixtureCase{}, newError(ErrorContractInvalid, "gateway request does not match fixture")
	}
	return fixture, nil
}

type FixtureCloudUsageClient struct {
	scope       cloud.BindingScope
	usageByKey  map[string]UsageFixtureCase
	settlements map[string]SettlementFixtureCase
	mu          sync.Mutex
	accepted    map[string][]byte
}

func NewFixtureCloudUsageClient(usage UsageFixtureBundle, settlements SettlementFixtureBundle) (*FixtureCloudUsageClient, error) {
	if err := validateUsageFixtureBundle(usage); err != nil {
		return nil, err
	}
	if err := validateSettlementFixtureBundle(settlements); err != nil {
		return nil, err
	}
	if usage.Scope != settlements.Scope {
		return nil, newError(ErrorScopeMismatch, "usage and settlement fixtures use different bindings")
	}
	client := &FixtureCloudUsageClient{
		scope: usage.Scope, usageByKey: make(map[string]UsageFixtureCase, len(usage.Cases)),
		settlements: make(map[string]SettlementFixtureCase, len(settlements.Cases)), accepted: make(map[string][]byte),
	}
	for _, fixture := range usage.Cases {
		key := ""
		if fixture.Partial != nil {
			key = fixture.Partial.IdempotencyKey
		} else {
			key = fixture.Final.IdempotencyKey
		}
		if _, exists := client.usageByKey[key]; exists {
			return nil, newError(ErrorContractInvalid, "duplicate usage fixture idempotency key")
		}
		cloned, err := cloneFixture(fixture)
		if err != nil {
			return nil, err
		}
		client.usageByKey[key] = cloned
	}
	for _, fixture := range settlements.Cases {
		cloned, err := cloneFixture(fixture)
		if err != nil {
			return nil, err
		}
		client.settlements[fixture.TaskID] = cloned
	}
	return client, nil
}

func cloneFixture[T any](value T) (T, error) {
	var cloned T
	payload, err := json.Marshal(value)
	if err != nil {
		return cloned, fmt.Errorf("modelbilling: clone fixture: %w", err)
	}
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return cloned, fmt.Errorf("modelbilling: clone fixture: %w", err)
	}
	return cloned, nil
}

func (c *FixtureCloudUsageClient) UploadPartial(ctx context.Context, scope cloud.BindingScope, report PartialUsageReport) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if scope != c.scope || report.Scope != scope {
		return newError(ErrorScopeMismatch, "partial report belongs to another binding")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	fixture, ok := c.usageByKey[report.IdempotencyKey]
	if !ok || fixture.Partial == nil {
		return newError(ErrorContractInvalid, "partial usage fixture not found")
	}
	return c.acceptUsage(report.IdempotencyKey, report, *fixture.Partial, fixture.Error)
}

func (c *FixtureCloudUsageClient) FinalizeUsage(ctx context.Context, scope cloud.BindingScope, report FinalUsageReport) (Settlement, error) {
	if err := ctx.Err(); err != nil {
		return Settlement{}, err
	}
	if scope != c.scope || report.Scope != scope {
		return Settlement{}, newError(ErrorScopeMismatch, "final report belongs to another binding")
	}
	if err := report.Validate(); err != nil {
		return Settlement{}, err
	}
	fixture, ok := c.usageByKey[report.IdempotencyKey]
	if !ok || fixture.Final == nil {
		return Settlement{}, newError(ErrorContractInvalid, "final usage fixture not found")
	}
	if err := c.acceptUsage(report.IdempotencyKey, report, *fixture.Final, fixture.Error); err != nil {
		return Settlement{}, err
	}
	if fixture.Settlement == nil {
		return Settlement{}, newError(ErrorContractInvalid, "final usage fixture has no settlement")
	}
	return *fixture.Settlement, nil
}

func (c *FixtureCloudUsageClient) GetSettlement(ctx context.Context, scope cloud.BindingScope, taskID string) (Settlement, error) {
	if err := ctx.Err(); err != nil {
		return Settlement{}, err
	}
	if scope != c.scope {
		return Settlement{}, newError(ErrorScopeMismatch, "settlement fixture belongs to another binding")
	}
	fixture, ok := c.settlements[taskID]
	if !ok {
		return Settlement{}, newError(ErrorContractInvalid, "settlement fixture not found")
	}
	if fixture.Error != nil {
		return Settlement{}, fixture.Error.domainError()
	}
	if fixture.Settlement == nil {
		return Settlement{}, newError(ErrorContractInvalid, "settlement fixture has no value")
	}
	return *fixture.Settlement, nil
}

func (c *FixtureCloudUsageClient) acceptUsage(key string, actual, expected any, fixtureErr *FixtureError) error {
	actualPayload, err := CanonicalJSON(actual)
	if err != nil {
		return err
	}
	expectedPayload, err := CanonicalJSON(expected)
	if err != nil {
		return err
	}
	if err := ValidateCanonicalPayload(expectedPayload, actualPayload); err != nil {
		return err
	}
	if fixtureErr != nil {
		return fixtureErr.domainError()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if prior, exists := c.accepted[key]; exists {
		return ValidateCanonicalPayload(prior, actualPayload)
	}
	c.accepted[key] = append([]byte(nil), actualPayload...)
	return nil
}

// MaterializedUsageFixtureSelection binds the runtime fake to named,
// versioned normalized cases. PartialCaseName is optional because B3C does not
// schedule automatic partial reports.
type MaterializedUsageFixtureSelection struct {
	FixtureVersion     string
	PartialCaseName    string
	FinalCaseName      string
	SettlementCaseName string
}

// MaterializedFixtureCloudUsageClient applies only runtime-owned identities to
// one selected strict usage/settlement template. Timestamps, model, pricing,
// usage, raw usage, result, sequence and report mode remain exact fixture
// semantics. It never performs HTTP.
type MaterializedFixtureCloudUsageClient struct {
	scope      cloud.BindingScope
	partial    *UsageFixtureCase
	final      UsageFixtureCase
	settlement SettlementFixtureCase

	mu       sync.Mutex
	accepted map[string][]byte
}

func NewMaterializedFixtureCloudUsageClient(
	usage UsageFixtureBundle,
	settlements SettlementFixtureBundle,
	runtimeScope cloud.BindingScope,
	selection MaterializedUsageFixtureSelection,
) (*MaterializedFixtureCloudUsageClient, error) {
	if err := validateMaterializedFixtureInputs(usage, settlements, runtimeScope, selection); err != nil {
		return nil, err
	}
	templates, err := selectMaterializedFixtureTemplates(usage, settlements, selection)
	if err != nil {
		return nil, err
	}
	return &MaterializedFixtureCloudUsageClient{
		scope:      runtimeScope,
		partial:    templates.partial,
		final:      templates.final,
		settlement: templates.settlement,
		accepted:   make(map[string][]byte),
	}, nil
}

type materializedFixtureTemplates struct {
	partial    *UsageFixtureCase
	final      UsageFixtureCase
	settlement SettlementFixtureCase
}

func validateMaterializedFixtureInputs(
	usage UsageFixtureBundle,
	settlements SettlementFixtureBundle,
	runtimeScope cloud.BindingScope,
	selection MaterializedUsageFixtureSelection,
) error {
	if err := validateUsageFixtureBundle(usage); err != nil {
		return err
	}
	if err := validateSettlementFixtureBundle(settlements); err != nil {
		return err
	}
	if usage.Scope != settlements.Scope {
		return newError(ErrorScopeMismatch, "usage and settlement fixtures use different bindings")
	}
	if err := runtimeScope.Validate(); err != nil {
		return newError(ErrorScopeMismatch, err.Error())
	}
	return validateMaterializedFixtureSelection(usage, settlements, selection)
}

func validateMaterializedFixtureSelection(
	usage UsageFixtureBundle,
	settlements SettlementFixtureBundle,
	selection MaterializedUsageFixtureSelection,
) error {
	if selection.FixtureVersion != usage.Version || selection.FixtureVersion != settlements.Version {
		return newError(ErrorContractInvalid, "fixture version does not match selection")
	}
	if err := requireTrimmed("final fixture case name", selection.FinalCaseName); err != nil {
		return err
	}
	if err := requireTrimmed("settlement fixture case name", selection.SettlementCaseName); err != nil {
		return err
	}
	if selection.PartialCaseName == "" {
		return nil
	}
	return requireTrimmed("partial fixture case name", selection.PartialCaseName)
}

func selectMaterializedFixtureTemplates(
	usage UsageFixtureBundle,
	settlements SettlementFixtureBundle,
	selection MaterializedUsageFixtureSelection,
) (materializedFixtureTemplates, error) {
	final, err := selectedUsageFixture(usage, selection.FinalCaseName, ReportFinal)
	if err != nil {
		return materializedFixtureTemplates{}, err
	}
	settlement, err := selectedSettlementFixture(settlements, selection.SettlementCaseName)
	if err != nil {
		return materializedFixtureTemplates{}, err
	}
	if err := validateSelectedSettlementFixtures(final, settlement); err != nil {
		return materializedFixtureTemplates{}, err
	}
	partial, err := selectedPartialFixture(usage, selection.PartialCaseName)
	if err != nil {
		return materializedFixtureTemplates{}, err
	}
	return materializedFixtureTemplates{partial: partial, final: final, settlement: settlement}, nil
}

func selectedPartialFixture(bundle UsageFixtureBundle, name string) (*UsageFixtureCase, error) {
	if name == "" {
		return nil, nil
	}
	fixture, err := selectedUsageFixture(bundle, name, ReportPartial)
	if err != nil {
		return nil, err
	}
	return &fixture, nil
}

func validateSelectedSettlementFixtures(final UsageFixtureCase, query SettlementFixtureCase) error {
	if final.Settlement == nil || query.Error != nil || query.Settlement == nil {
		return newError(ErrorContractInvalid, "materialized settlement selections must be successful")
	}
	if !compatibleSettlementFixtures(*final.Settlement, *query.Settlement) {
		return newError(ErrorContractInvalid, "usage and settlement fixture cases are incompatible")
	}
	return nil
}

func selectedUsageFixture(bundle UsageFixtureBundle, name string, kind ReportKind) (UsageFixtureCase, error) {
	for _, fixture := range bundle.Cases {
		if fixture.Name != name {
			continue
		}
		if kind == ReportPartial && fixture.Partial == nil || kind == ReportFinal && fixture.Final == nil {
			return UsageFixtureCase{}, newError(ErrorContractInvalid, "usage fixture case has the wrong report kind")
		}
		return cloneFixture(fixture)
	}
	return UsageFixtureCase{}, newError(ErrorContractInvalid, "usage fixture case not found")
}

func selectedSettlementFixture(bundle SettlementFixtureBundle, name string) (SettlementFixtureCase, error) {
	for _, fixture := range bundle.Cases {
		if fixture.Name == name {
			return cloneFixture(fixture)
		}
	}
	return SettlementFixtureCase{}, newError(ErrorContractInvalid, "settlement fixture case not found")
}

func compatibleSettlementFixtures(left, right Settlement) bool {
	if left.State != right.State || !left.ObservedAt.Equal(right.ObservedAt) {
		return false
	}
	if !compatibleSettlementAmounts(left, right) {
		return false
	}
	if !compatibleOptionalString(left.PricingStatus, right.PricingStatus) {
		return false
	}
	if !compatibleOptionalString(left.ErrorCode, right.ErrorCode) {
		return false
	}
	return compatibleOptionalUint64(left.BalanceVersion, right.BalanceVersion)
}

func compatibleSettlementAmounts(left, right Settlement) bool {
	pairs := [][2]*DecimalString{
		{left.ChargedAmount, right.ChargedAmount},
		{left.ChargedFreeAmount, right.ChargedFreeAmount},
		{left.ChargedSubscriptionAmount, right.ChargedSubscriptionAmount},
		{left.ReleasedFreeAmount, right.ReleasedFreeAmount},
		{left.ReleasedSubscriptionAmount, right.ReleasedSubscriptionAmount},
		{left.UncoveredAmount, right.UncoveredAmount},
	}
	for _, pair := range pairs {
		if !compatibleOptionalDecimal(pair[0], pair[1]) {
			return false
		}
	}
	return true
}

func compatibleOptionalDecimal(left, right *DecimalString) bool {
	return left == nil || right == nil || *left == *right
}

func compatibleOptionalString(left, right *string) bool {
	return left == nil || right == nil || *left == *right
}

func compatibleOptionalUint64(left, right *uint64) bool {
	return left == nil || right == nil || *left == *right
}

func (c *MaterializedFixtureCloudUsageClient) UploadPartial(
	ctx context.Context,
	scope cloud.BindingScope,
	report PartialUsageReport,
) error {
	if err := materializedUsageRequestValid(ctx, c, scope, report.Scope); err != nil {
		return err
	}
	if err := report.Validate(); err != nil {
		return err
	}
	if c.partial == nil {
		return newError(ErrorContractInvalid, "partial usage fixture is not selected")
	}
	expected, err := materializePartialUsageFixture(*c.partial.Partial, report)
	if err != nil {
		return err
	}
	return c.acceptMaterialized(report.IdempotencyKey, report, expected, c.partial.Error)
}

func (c *MaterializedFixtureCloudUsageClient) FinalizeUsage(
	ctx context.Context,
	scope cloud.BindingScope,
	report FinalUsageReport,
) (Settlement, error) {
	if err := materializedUsageRequestValid(ctx, c, scope, report.Scope); err != nil {
		return Settlement{}, err
	}
	if err := report.Validate(); err != nil {
		return Settlement{}, err
	}
	expected, err := materializeFinalUsageFixture(*c.final.Final, report)
	if err != nil {
		return Settlement{}, err
	}
	if err := c.acceptMaterialized(report.IdempotencyKey, report, expected, c.final.Error); err != nil {
		return Settlement{}, err
	}
	if c.final.Settlement == nil {
		return Settlement{}, newError(ErrorContractInvalid, "final usage fixture has no settlement")
	}
	return materializeSettlement(*c.final.Settlement, c.scope, report.TaskID)
}

func (c *MaterializedFixtureCloudUsageClient) GetSettlement(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID string,
) (Settlement, error) {
	if err := materializedUsageRequestValid(ctx, c, scope, scope); err != nil {
		return Settlement{}, err
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return Settlement{}, err
	}
	if c.settlement.Settlement == nil {
		return Settlement{}, newError(ErrorContractInvalid, "settlement fixture has no settlement")
	}
	return materializeSettlement(*c.settlement.Settlement, c.scope, taskID)
}

func materializedUsageRequestValid(
	ctx context.Context,
	client *MaterializedFixtureCloudUsageClient,
	scope cloud.BindingScope,
	reportScope cloud.BindingScope,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if client == nil {
		return newError(ErrorContractInvalid, "materialized usage fixture client is required")
	}
	if scope != client.scope || reportScope != scope {
		return newError(ErrorScopeMismatch, "usage report belongs to another binding")
	}
	return nil
}

func materializePartialUsageFixture(template PartialUsageReport, actual PartialUsageReport) (PartialUsageReport, error) {
	cloned, err := cloneFixture(template)
	if err != nil {
		return PartialUsageReport{}, err
	}
	template = cloned
	template.TaskID = actual.TaskID
	template.Scope = actual.Scope
	template.FreezeID = actual.FreezeID
	template.IdempotencyKey = actual.IdempotencyKey
	if err := materializeUsageCallIdentities(template.Calls, actual.Calls); err != nil {
		return PartialUsageReport{}, err
	}
	return template, template.Validate()
}

func materializeFinalUsageFixture(template FinalUsageReport, actual FinalUsageReport) (FinalUsageReport, error) {
	cloned, err := cloneFixture(template)
	if err != nil {
		return FinalUsageReport{}, err
	}
	template = cloned
	template.TaskID = actual.TaskID
	template.Scope = actual.Scope
	template.FreezeID = actual.FreezeID
	template.IdempotencyKey = actual.IdempotencyKey
	if err := materializeUsageCallIdentities(template.Calls, actual.Calls); err != nil {
		return FinalUsageReport{}, err
	}
	return template, template.Validate()
}

func materializeUsageCallIdentities(expected, actual []ModelCallUsage) error {
	if len(expected) != len(actual) {
		return newError(ErrorIdempotencyConflict, "usage report call count does not match fixture")
	}
	for i := range expected {
		expected[i].CallID = actual[i].CallID
		expected[i].PricingEvidenceID = actual[i].PricingEvidenceID
		expected[i].ProviderRequestID = cloneOptionalString(actual[i].ProviderRequestID)
	}
	return nil
}

func (c *MaterializedFixtureCloudUsageClient) acceptMaterialized(key string, actual, expected any, fixtureErr *FixtureError) error {
	actualPayload, err := CanonicalJSON(actual)
	if err != nil {
		return err
	}
	expectedPayload, err := CanonicalJSON(expected)
	if err != nil {
		return err
	}
	if err := ValidateCanonicalPayload(expectedPayload, actualPayload); err != nil {
		return err
	}
	c.mu.Lock()
	if prior, exists := c.accepted[key]; exists {
		err = ValidateCanonicalPayload(prior, actualPayload)
	} else {
		c.accepted[key] = append([]byte(nil), actualPayload...)
	}
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if fixtureErr != nil {
		return fixtureErr.domainError()
	}
	return nil
}

func materializeSettlement(template Settlement, scope cloud.BindingScope, taskID string) (Settlement, error) {
	cloned, err := cloneFixture(template)
	if err != nil {
		return Settlement{}, err
	}
	template = cloned
	settlementID, err := materializedSettlementID(scope, taskID)
	if err != nil {
		return Settlement{}, err
	}
	template.TaskID = taskID
	template.SettlementID = settlementID
	if err := template.Validate(); err != nil {
		return Settlement{}, err
	}
	return template, nil
}

func materializedSettlementID(scope cloud.BindingScope, taskID string) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", newError(ErrorScopeMismatch, err.Error())
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return "", err
	}
	payload := fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%s", scope.ProfileID, scope.CloudUserID,
		scope.BindingGeneration, FixtureVersionB0V1, taskID)
	sum := sha256.Sum256([]byte(payload))
	return "set_fixture_" + hex.EncodeToString(sum[:16]), nil
}

type FixtureSettlementTaskIdentity struct {
	Scope    cloud.BindingScope
	TaskID   string
	FreezeID string
}

type FixtureSettlementIdentityResolver func(
	context.Context,
	cloud.BindingScope,
	string,
) (FixtureSettlementTaskIdentity, error)

type FixtureSettlementObservation struct {
	Scope        cloud.BindingScope
	TaskID       string
	FreezeID     string
	SettlementID string
}

// FixtureSettlementObserver validates the persisted task/freeze identity and
// records observation evidence only. It never mutates balance state.
type FixtureSettlementObserver struct {
	scope   cloud.BindingScope
	resolve FixtureSettlementIdentityResolver

	mu           sync.Mutex
	observations []FixtureSettlementObservation
}

func NewFixtureSettlementObserver(
	scope cloud.BindingScope,
	resolve FixtureSettlementIdentityResolver,
) (*FixtureSettlementObserver, error) {
	if err := scope.Validate(); err != nil {
		return nil, newError(ErrorScopeMismatch, err.Error())
	}
	if resolve == nil {
		return nil, newError(ErrorContractInvalid, "settlement identity resolver is required")
	}
	return &FixtureSettlementObserver{scope: scope, resolve: resolve}, nil
}

func (o *FixtureSettlementObserver) CreditSettlementObserved(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID string,
	settlementID string,
) error {
	if err := validateFixtureSettlementObservationRequest(ctx, o, scope, taskID, settlementID); err != nil {
		return err
	}
	identity, err := o.resolve(ctx, scope, taskID)
	if err != nil {
		return err
	}
	if err := validateFixtureSettlementIdentity(identity, scope, taskID, settlementID); err != nil {
		return err
	}
	o.recordObservation(identity, settlementID)
	return nil
}

func validateFixtureSettlementObservationRequest(
	ctx context.Context,
	observer *FixtureSettlementObserver,
	scope cloud.BindingScope,
	taskID string,
	settlementID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if observer == nil || observer.resolve == nil {
		return newError(ErrorContractInvalid, "fixture settlement observer is required")
	}
	if scope != observer.scope {
		return newError(ErrorScopeMismatch, "settlement observation belongs to another binding")
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return err
	}
	return requireTrimmed("settlement_id", settlementID)
}

func validateFixtureSettlementIdentity(
	identity FixtureSettlementTaskIdentity,
	scope cloud.BindingScope,
	taskID string,
	settlementID string,
) error {
	if identity.Scope != scope || identity.TaskID != taskID {
		return newError(ErrorScopeMismatch, "settlement observer resolved another task identity")
	}
	if err := requireTrimmed("freeze_id", identity.FreezeID); err != nil {
		return err
	}
	expectedSettlementID, err := materializedSettlementID(scope, taskID)
	if err != nil {
		return err
	}
	if settlementID != expectedSettlementID {
		return newError(ErrorScopeMismatch, "settlement observer received another settlement identity")
	}
	return nil
}

func (o *FixtureSettlementObserver) recordObservation(
	identity FixtureSettlementTaskIdentity,
	settlementID string,
) {
	o.mu.Lock()
	o.observations = append(o.observations, FixtureSettlementObservation{
		Scope: identity.Scope, TaskID: identity.TaskID, FreezeID: identity.FreezeID, SettlementID: settlementID,
	})
	o.mu.Unlock()
}

func (o *FixtureSettlementObserver) Observations() []FixtureSettlementObservation {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]FixtureSettlementObservation(nil), o.observations...)
}

var (
	_ SubscriptionModelGateway            = (*FixtureGateway)(nil)
	_ SubscriptionGatewayEndpointResolver = (*FixtureGateway)(nil)
	_ CloudUsageClient                    = (*FixtureCloudUsageClient)(nil)
	_ CloudUsageClient                    = (*MaterializedFixtureCloudUsageClient)(nil)
	_ SettlementObserver                  = (*FixtureSettlementObserver)(nil)
)
