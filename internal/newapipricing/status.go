package newapipricing

import (
	"context"

	"kwu/internal/modelbilling"
)

// StatusSource is deliberately normalized-only while NEWAPI-STATUS-01 is
// MOCK_ONLY. No endpoint or status wire contract belongs in B1A.
type StatusSource interface {
	FetchStatus(ctx context.Context) (modelbilling.PricingSourceStatus, error)
}

func LoadStatus(ctx context.Context, source StatusSource) (modelbilling.PricingSourceStatus, error) {
	if source == nil {
		return modelbilling.PricingSourceStatus{}, contractError("newapi status source is required")
	}
	status, err := source.FetchStatus(ctx)
	if err != nil {
		return modelbilling.PricingSourceStatus{}, err
	}
	if err := status.Validate(); err != nil {
		return modelbilling.PricingSourceStatus{}, err
	}
	return status, nil
}
