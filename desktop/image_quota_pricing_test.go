package main

import (
	"context"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

type imageQuotaBalanceClient struct {
	balance cloud.MaaSKeyBalance
}

func (c imageQuotaBalanceClient) Get(context.Context, cloud.BindingScope) (cloud.MaaSKeyBalance, error) {
	return c.balance, nil
}

func TestDesktopImageQuotaPricingUsesCachedModelPricesAndAuthoritativeConversion(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	actor := userctx.ActorContext{ActorID: "image-pricing-actor", ProfileID: "image-pricing-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "image-pricing-user", BindingGeneration: 1}
	done := make(chan struct{})
	manager := &desktopBillingWorkerManager{
		bindingActor: actor, bindingScope: binding, hasBinding: true,
		bindingStatus: cloud.BindingBound, authState: cloud.AccountAuthenticated,
		authorized: true, authorizedActor: actor, authorizedScope: binding,
		active: &desktopBillingWorkerProcess{
			scope: modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding},
			done:  done,
		},
	}
	catalog := &desktopSubscriptionModelCatalog{}
	catalog.replace(binding, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: binding.CloudUserID, Key: "maas-image-key",
	}, []string{"gpt-image-2"})
	cache, err := newModelMarketplaceDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	config := defaultModelMarketplaceSources[0]
	if err := cache.Write(modelMarketplaceCacheFile{
		Version: modelMarketplaceCacheVersion, SourceID: config.ID, SourceURL: config.Endpoint,
		FetchedAt: time.Now().UTC(), Models: []ModelMarketplaceModel{{
			ID: "gpt-image-2", Name: "gpt-image-2", Prices: []ModelMarketplacePriceFact{
				{Key: "input_price", Value: "5"}, {Key: "output_price", Value: "30"}, {Key: "image_price", Value: "8"},
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	app := &App{
		billingWorkerRuntime: manager,
		subscriptionModels:   catalog,
		modelMarketplace:     &modelMarketplaceService{cache: cache, sources: []modelMarketplaceSourceConfig{config}},
		cloudRuntime: &cloudRuntime{maasBalance: imageQuotaBalanceClient{balance: cloud.MaaSKeyBalance{
			AvailableBalance: "100.0000", RemainQuota: 1000, Source: "maas_api_key",
			ConversionRuleVersion: "maas-quota-credit-v1", QuotaPerUSD: "500000",
			CNYPerUSD: "7.3", CreditPerCNY: "2000", QueriedAt: time.Now().UTC(),
		}}},
	}
	pricing, err := app.desktopImageQuotaPricingSource(actor)(context.Background(), "gpt-image-2")
	if err != nil {
		t.Fatal(err)
	}
	if pricing.InputPerMillion != "5" || pricing.OutputPerMillion != "30" ||
		pricing.ImageInputPerMillion != "8" || pricing.QuotaPerUSD != "500000" {
		t.Fatalf("pricing = %+v", pricing)
	}
	close(done)
}

var _ cloud.MaaSKeyBalanceClient = imageQuotaBalanceClient{}
