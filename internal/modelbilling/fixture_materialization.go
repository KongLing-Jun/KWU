package modelbilling

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"

	"kwu/internal/provider"
)

const fixtureStatusSuccessCase = "status_success"

//go:embed testdata/CLOUD-CREDIT-CONFIG-01/normalized-v1.json
var embeddedCreditConfigFixture []byte

//go:embed testdata/CLOUD-GATEWAY-01/normalized-v1.json
var embeddedGatewayFixture []byte

//go:embed testdata/NEWAPI-STATUS-01/normalized-v1.json
var fixtureStatusBundleJSON []byte

func LoadEmbeddedCreditConfigFixture() (CreditConfigFixtureBundle, error) {
	return DecodeCreditConfigFixture(bytes.NewReader(embeddedCreditConfigFixture))
}

// LoadEmbeddedGatewayFixture is the explicit MOCK_ONLY B2-TMP-009 catalog and
// template source used only when the desktop fake composition is enabled.
func LoadEmbeddedGatewayFixture() (GatewayFixtureBundle, error) {
	return DecodeGatewayFixture(bytes.NewReader(embeddedGatewayFixture))
}

const gatewayFixtureRuntimeCallIDPlaceholder = "${runtime.call_id}"

// GatewayFixtureSelection selects one strictly-versioned gateway template.
// Runtime identity is injected only after the template has passed validation.
type GatewayFixtureSelection struct {
	FixtureVersion string
	CaseName       string
}

type GatewayFixtureIdentity struct {
	TaskID          string
	CallID          string
	ProviderRequest provider.Request
}

func MaterializeGatewayFixture(bundle GatewayFixtureBundle, selection GatewayFixtureSelection, identity GatewayFixtureIdentity) (GatewayFixtureCase, error) {
	if err := validateGatewayFixtureBundle(bundle); err != nil {
		return GatewayFixtureCase{}, err
	}
	if selection.FixtureVersion != bundle.Version {
		return GatewayFixtureCase{}, newError(ErrorContractInvalid, "gateway fixture version does not match selection")
	}
	if err := requireTrimmed("fixture case name", selection.CaseName); err != nil {
		return GatewayFixtureCase{}, err
	}
	if err := requireTrimmed("task_id", identity.TaskID); err != nil {
		return GatewayFixtureCase{}, err
	}
	if err := requireTrimmed("call_id", identity.CallID); err != nil {
		return GatewayFixtureCase{}, err
	}
	for _, candidate := range bundle.Cases {
		if candidate.Name != selection.CaseName {
			continue
		}
		materialized, err := cloneFixture(candidate)
		if err != nil {
			return GatewayFixtureCase{}, err
		}
		materialized.Request.TaskID = identity.TaskID
		materialized.Request.CallID = identity.CallID
		materialized.Request.ProviderRequest = identity.ProviderRequest
		materializeGatewayResponseRequestID(&materialized, identity.CallID)
		if err := materialized.Request.Validate(); err != nil {
			return GatewayFixtureCase{}, fmt.Errorf("materialized gateway fixture: %w", err)
		}
		return materialized, nil
	}
	return GatewayFixtureCase{}, newError(ErrorContractInvalid, "gateway fixture case not found")
}

func materializeGatewayResponseRequestID(fixture *GatewayFixtureCase, callID string) {
	for chunkIndex := range fixture.Chunks {
		chunk := &fixture.Chunks[chunkIndex]
		if chunk.ProviderRequestID == gatewayFixtureRuntimeCallIDPlaceholder {
			chunk.ProviderRequestID = callID
		}
		if chunk.QuotaObservation == nil {
			continue
		}
		for name, values := range chunk.QuotaObservation.Headers {
			for valueIndex := range values {
				if values[valueIndex] == gatewayFixtureRuntimeCallIDPlaceholder {
					values[valueIndex] = callID
				}
			}
			chunk.QuotaObservation.Headers[name] = values
		}
	}
}

// FixtureStatusSource is the explicit L0/mock source backed by the registered
// NEWAPI-STATUS-01 normalized fixture. It is not a production status adapter.
type FixtureStatusSource struct {
	status PricingSourceStatus
}

func NewFixtureStatusSource() (FixtureStatusSource, error) {
	bundle, err := DecodeStatusFixture(bytes.NewReader(fixtureStatusBundleJSON))
	if err != nil {
		return FixtureStatusSource{}, err
	}
	for _, fixture := range bundle.Cases {
		if fixture.Name == fixtureStatusSuccessCase && fixture.Status != nil {
			return FixtureStatusSource{status: clonePricingSourceStatus(*fixture.Status)}, nil
		}
	}
	return FixtureStatusSource{}, fmt.Errorf("modelbilling: status fixture case %q is required", fixtureStatusSuccessCase)
}

func (s FixtureStatusSource) FetchStatus(ctx context.Context) (PricingSourceStatus, error) {
	if err := ctx.Err(); err != nil {
		return PricingSourceStatus{}, err
	}
	return clonePricingSourceStatus(s.status), nil
}

func clonePricingSourceStatus(status PricingSourceStatus) PricingSourceStatus {
	status.SourceVersion = cloneOptionalString(status.SourceVersion)
	status.SourceHash = cloneOptionalString(status.SourceHash)
	return status
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
