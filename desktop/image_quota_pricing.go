package main

import (
	"context"
	"fmt"
	"strings"

	"kwu/internal/airclaude"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func (a *App) desktopImageQuotaPricingSource(actor userctx.ActorContext) airclaude.ImageQuotaPricingSource {
	actor = actor.Normalize()
	return func(ctx context.Context, modelID string) (airclaude.ImageQuotaPricing, error) {
		if a == nil || a.cloudRuntime == nil || a.cloudRuntime.maasBalance == nil || a.modelMarketplace == nil {
			return airclaude.ImageQuotaPricing{}, fmt.Errorf("desktop image pricing is unavailable")
		}
		binding, _, authorized := a.desktopAuthorizedSubscriptionCatalogForActor(actor)
		if !authorized || a.billingWorkerRuntime.authorizeSubscription(actor, binding) != nil {
			return airclaude.ImageQuotaPricing{}, &modelbilling.Error{
				Code: modelbilling.ErrorAuthRequired, Message: "AirClaude image pricing authorization is unavailable",
			}
		}
		balance, err := a.cloudRuntime.maasBalance.Get(ctx, binding)
		if err != nil {
			return airclaude.ImageQuotaPricing{}, err
		}
		model, ok := marketplaceImageModel(a.modelMarketplace.Read(), modelID)
		if !ok {
			return airclaude.ImageQuotaPricing{}, fmt.Errorf("image model %q has no cached pricing", strings.TrimSpace(modelID))
		}
		prices := make(map[string]string, len(model.Prices))
		for _, fact := range model.Prices {
			prices[strings.TrimSpace(fact.Key)] = strings.TrimSpace(fact.Value)
		}
		pricing := airclaude.ImageQuotaPricing{
			InputPerMillion:       prices["input_price"],
			OutputPerMillion:      prices["output_price"],
			ImageInputPerMillion:  prices["image_price"],
			ImageOutputPerMillion: prices["output_image_price"],
			QuotaPerUSD:           balance.QuotaPerUSD,
		}
		if err := pricing.Validate(); err != nil {
			return airclaude.ImageQuotaPricing{}, fmt.Errorf("invalid cached image pricing: %w", err)
		}
		return pricing, nil
	}
}

func marketplaceImageModel(view ModelMarketplaceView, modelID string) (ModelMarketplaceModel, bool) {
	modelID = strings.TrimSpace(modelID)
	for _, source := range view.Sources {
		for _, model := range source.Models {
			if strings.TrimSpace(model.ID) == modelID {
				return model, true
			}
		}
	}
	return ModelMarketplaceModel{}, false
}
