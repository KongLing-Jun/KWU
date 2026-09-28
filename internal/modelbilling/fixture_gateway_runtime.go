package modelbilling

import (
	"context"
	"reflect"
	"sort"

	"kwu/internal/cloud"
	"kwu/internal/provider"
)

// FixtureSubscriptionModels returns only models explicitly declared for main
// calls by the versioned CLOUD-GATEWAY-01 fixture. Pricing caches are not an
// eligibility source for subscription_pool.
func FixtureSubscriptionModels(bundle GatewayFixtureBundle) ([]string, error) {
	if err := validateGatewayFixtureBundle(bundle); err != nil {
		return nil, err
	}
	models := map[string]struct{}{}
	for _, fixture := range bundle.Cases {
		if fixture.Request.Role == CallRoleMain {
			models[fixture.Request.ModelID] = struct{}{}
		}
	}
	out := make([]string, 0, len(models))
	for model := range models {
		out = append(out, model)
	}
	sort.Strings(out)
	return out, nil
}

// MaterializedFixtureGateway applies B2-TMP-009 to one selected strict
// template on every logical call. It never accepts an arbitrary UUID: request
// identity is injected, while scope, model, role and provider request remain
// fixture-validated by FixtureGateway.
type MaterializedFixtureGateway struct {
	bundle    GatewayFixtureBundle
	selection GatewayFixtureSelection
}

func NewMaterializedFixtureGateway(bundle GatewayFixtureBundle, selection GatewayFixtureSelection) (*MaterializedFixtureGateway, error) {
	if err := validateGatewayFixtureBundle(bundle); err != nil {
		return nil, err
	}
	if selection.FixtureVersion != bundle.Version {
		return nil, newError(ErrorContractInvalid, "gateway fixture version does not match selection")
	}
	if err := requireTrimmed("fixture case name", selection.CaseName); err != nil {
		return nil, err
	}
	return &MaterializedFixtureGateway{bundle: bundle, selection: selection}, nil
}

func (g *MaterializedFixtureGateway) Stream(ctx context.Context, scope cloud.BindingScope, request GatewayRequest) (<-chan GatewayChunk, error) {
	if g == nil {
		return nil, newError(ErrorContractInvalid, "materialized gateway is required")
	}
	caseFixture, err := g.materialize(request)
	if err != nil {
		return nil, err
	}
	fixtureGateway, err := NewFixtureGateway(GatewayFixtureBundle{Version: g.bundle.Version, DependencyID: g.bundle.DependencyID, Scope: g.bundle.Scope, Cases: []GatewayFixtureCase{caseFixture}})
	if err != nil {
		return nil, err
	}
	return fixtureGateway.Stream(ctx, scope, request)
}

// EndpointFor resolves the selected normalized endpoint before the dispatcher
// persists the call intent. Runtime task/call identity is used only to
// materialize and validate the selected fixture.
func (g *MaterializedFixtureGateway) EndpointFor(request GatewayRequest) (MaasEndpoint, error) {
	if g == nil {
		return "", newError(ErrorContractInvalid, "materialized gateway is required")
	}
	fixture, err := g.materialize(request)
	if err != nil {
		return "", err
	}
	return fixture.Endpoint, nil
}

func (g *MaterializedFixtureGateway) materialize(request GatewayRequest) (GatewayFixtureCase, error) {
	fixture, err := MaterializeGatewayFixture(g.bundle, g.selection, GatewayFixtureIdentity{
		TaskID: request.TaskID, CallID: request.CallID, ProviderRequest: request.ProviderRequest,
	})
	if err != nil {
		return GatewayFixtureCase{}, err
	}
	if !reflect.DeepEqual(fixture.Request, request) {
		return GatewayFixtureCase{}, newError(ErrorContractInvalid, "gateway request does not match selected fixture")
	}
	return fixture, nil
}

func (r GatewayRequest) Validate() error {
	for name, value := range map[string]string{
		"task_id": r.TaskID, "call_id": r.CallID, "model_id": r.ModelID, "role": string(r.Role),
	} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	return r.Role.Validate()
}

func (c GatewayChunk) Validate() error {
	if err := validateGatewayChunkEnvelope(c); err != nil {
		return err
	}
	if err := validateGatewayChunkEvidence(c); err != nil {
		return err
	}
	return validateGatewayUsageChunk(c)
}

func validateGatewayChunkEnvelope(chunk GatewayChunk) error {
	if err := ValidateRawUsageRedacted(chunk.RawUsageRedacted); err != nil {
		return err
	}
	if chunk.ProviderRequestID != "" {
		return requireTrimmed("provider_request_id", chunk.ProviderRequestID)
	}
	return nil
}

func validateGatewayChunkEvidence(chunk GatewayChunk) error {
	hasQuotaEvidence := chunk.QuotaEvidence != nil || len(chunk.QuotaEvidenceRevisions) != 0
	if hasQuotaEvidence && chunk.Chunk.Type != provider.ChunkDone {
		return newError(ErrorContractInvalid, "quota evidence requires the final done chunk")
	}
	if chunk.QuotaEvidence != nil && len(chunk.QuotaEvidenceRevisions) != 0 {
		return newError(ErrorContractInvalid, "gateway chunk cannot contain singular and revision quota evidence together")
	}
	if err := validateSingularGatewayEvidence(chunk.QuotaEvidence); err != nil {
		return err
	}
	return validateGatewayEvidenceRevisions(chunk.QuotaEvidenceRevisions)
}

func validateSingularGatewayEvidence(evidence *QuotaEvidenceRevision) error {
	if evidence == nil {
		return nil
	}
	if evidence.Revision != 1 {
		return newError(ErrorContractInvalid, "gateway quota evidence must start at revision 1")
	}
	return evidence.Validate()
}

func validateGatewayEvidenceRevisions(revisions []QuotaEvidenceRevision) error {
	for i := range revisions {
		if err := validateGatewayEvidenceRevision(revisions, i); err != nil {
			return err
		}
		if err := rejectDuplicateGatewayEvidence(revisions, i); err != nil {
			return err
		}
	}
	return nil
}

func validateGatewayEvidenceRevision(revisions []QuotaEvidenceRevision, index int) error {
	revision := revisions[index]
	if revision.Revision != uint64(index+1) || revision.OneAPIRequestID == nil {
		return newError(ErrorContractInvalid, "gateway quota evidence revisions must be contiguous request ID facts")
	}
	if err := revision.Validate(); err != nil {
		return err
	}
	if index > 0 && revision.ObservedAt.Before(revisions[index-1].ObservedAt) {
		return newError(ErrorContractInvalid, "gateway quota evidence observed_at cannot move backwards")
	}
	return nil
}

func rejectDuplicateGatewayEvidence(revisions []QuotaEvidenceRevision, index int) error {
	for previous := 0; previous < index; previous++ {
		if quotaEvidenceProjectionEqual(&revisions[previous], &revisions[index]) {
			return newError(ErrorContractInvalid, "gateway quota evidence revisions contain a duplicate fact")
		}
	}
	return nil
}

func validateGatewayUsageChunk(chunk GatewayChunk) error {
	if chunk.Chunk.Type != provider.ChunkUsage {
		return nil
	}
	if chunk.Chunk.Usage == nil || chunk.Usage == nil {
		return newError(ErrorContractInvalid, "gateway usage chunk requires provider and normalized usage")
	}
	return chunk.Usage.ValidateForReport()
}

var (
	_ SubscriptionModelGateway            = (*MaterializedFixtureGateway)(nil)
	_ SubscriptionGatewayEndpointResolver = (*MaterializedFixtureGateway)(nil)
)
