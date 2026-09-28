package newapipricing

import (
	"testing"
	"time"

	"kwu/internal/modelbilling"
)

func TestContentHashNormalizesMapAndSliceOrder(t *testing.T) {
	left, provenance := cacheHashFixture()
	right := clonePricingSnapshot(left)
	right.GroupRatios = map[string]modelbilling.DecimalString{"vip": "0.5", "default": "1"}
	right.UsableGroups = []string{"vip", "default"}
	right.Models = []modelbilling.ModelPrice{right.Models[1], right.Models[0]}
	right.Models[0].Capabilities = reverseStrings(right.Models[0].Capabilities)
	right.Models[0].EnableGroups = reverseStrings(right.Models[0].EnableGroups)
	right.Models[1].Capabilities = reverseStrings(right.Models[1].Capabilities)
	right.Models[1].EnableGroups = reverseStrings(right.Models[1].EnableGroups)

	leftHash, err := ContentHash(left, provenance)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := ContentHash(right, provenance)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("semantic reorder changed hash: %q/%q", leftHash, rightHash)
	}
}

func TestContentHashChangesForPricingGroupsStatusAndAuthenticity(t *testing.T) {
	base, provenance := cacheHashFixture()
	baseHash, err := ContentHash(base, provenance)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*modelbilling.PricingSnapshot, *SnapshotProvenance){
		"pricing": func(snapshot *modelbilling.PricingSnapshot, _ *SnapshotProvenance) {
			value := modelbilling.DecimalString("9")
			snapshot.Models[0].ModelRatio = &value
		},
		"group ratio": func(snapshot *modelbilling.PricingSnapshot, _ *SnapshotProvenance) {
			snapshot.GroupRatios["default"] = "2"
		},
		"usable group": func(snapshot *modelbilling.PricingSnapshot, _ *SnapshotProvenance) {
			snapshot.UsableGroups = append(snapshot.UsableGroups, "partner")
		},
		"exchange rate": func(snapshot *modelbilling.PricingSnapshot, _ *SnapshotProvenance) {
			value := modelbilling.DecimalString("8")
			snapshot.ExchangeRate = &value
		},
		"recharge price": func(snapshot *modelbilling.PricingSnapshot, _ *SnapshotProvenance) {
			value := modelbilling.DecimalString("2")
			snapshot.RechargePrice = &value
		},
		"status authenticity": func(_ *modelbilling.PricingSnapshot, provenance *SnapshotProvenance) {
			provenance.Status.Authenticity = SourceReady
		},
		"status source hash": func(_ *modelbilling.PricingSnapshot, provenance *SnapshotProvenance) {
			value := "sha256:upstream-status"
			provenance.Status.SourceHash = &value
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			snapshot := clonePricingSnapshot(base)
			changedProvenance := cloneProvenance(provenance)
			mutate(&snapshot, &changedProvenance)
			got, err := ContentHash(snapshot, changedProvenance)
			if err != nil {
				t.Fatal(err)
			}
			if got == baseHash {
				t.Fatalf("change did not affect hash %q", got)
			}
		})
	}
}

func TestContentHashExcludesTimeStateAndSnapshotIdentity(t *testing.T) {
	base, provenance := cacheHashFixture()
	baseHash, err := ContentHash(base, provenance)
	if err != nil {
		t.Fatal(err)
	}
	changed := clonePricingSnapshot(base)
	changed.SnapshotID = "another-id"
	changed.SourceHash = "sha256:another"
	changed.FetchedAt = changed.FetchedAt.Add(100 * time.Hour)
	changed.FreshUntil = changed.FreshUntil.Add(100 * time.Hour)
	changed.StaleUntil = changed.StaleUntil.Add(100 * time.Hour)
	changed.State = modelbilling.PricingStaleBlocked
	changedProvenance := cloneProvenance(provenance)
	changedProvenance.Pricing.FetchedAt = changedProvenance.Pricing.FetchedAt.Add(time.Hour)
	changedProvenance.Status.FetchedAt = changedProvenance.Status.FetchedAt.Add(time.Hour)
	got, err := ContentHash(changed, changedProvenance)
	if err != nil {
		t.Fatal(err)
	}
	if got != baseHash {
		t.Fatalf("time/state/identity changed content hash: %q/%q", baseHash, got)
	}
	if BuildSnapshotID(base.Source, baseHash) != BuildSnapshotID(base.Source, got) {
		t.Fatal("stable content did not produce stable snapshot ID")
	}
}

func cacheHashFixture() (modelbilling.PricingSnapshot, SnapshotProvenance) {
	version := "pricing-v1"
	statusVersion := "fixture-v1"
	exchange := modelbilling.DecimalString("7.2")
	recharge := modelbilling.DecimalString("1.1")
	one := modelbilling.DecimalString("1")
	two := modelbilling.DecimalString("2")
	requestPrice := modelbilling.DecimalString("0.5")
	fetchedAt := cacheTestStart
	snapshot := modelbilling.PricingSnapshot{
		SnapshotID: "ignored", Source: "domestic", SourceVersion: &version, SourceHash: "ignored", Currency: "CNY",
		ExchangeRate: &exchange, RechargePrice: &recharge,
		GroupRatios:  map[string]modelbilling.DecimalString{"default": "1", "vip": "0.5"},
		UsableGroups: []string{"default", "vip"}, FetchedAt: fetchedAt,
		FreshUntil: fetchedAt.Add(12 * time.Hour), StaleUntil: fetchedAt.Add(24 * time.Hour), State: modelbilling.PricingFresh,
		Models: []modelbilling.ModelPrice{
			{
				ModelID: "model-a", ProviderModelID: "model-a", ModelName: "Model A", ModelType: "chat",
				Capabilities: []string{"vision", "text"}, QuotaType: modelbilling.QuotaTypeToken,
				ModelRatio: &one, CompletionRatio: &two, EnableGroups: []string{"vip", "default"},
				BillingMode: modelbilling.BillingModeTokenRatio,
			},
			{
				ModelID: "model-b", ProviderModelID: "model-b", ModelName: "Model B", ModelType: "image",
				Capabilities: []string{"image"}, QuotaType: modelbilling.QuotaTypePerRequest,
				ModelPrice: &requestPrice, EnableGroups: []string{"vip"}, BillingMode: modelbilling.BillingModePerRequest,
			},
		},
	}
	provenance := SnapshotProvenance{
		Pricing: SourceEvidence{
			DependencyID: pricingDependency, Authenticity: SourceReady, SourceVersion: &version, FetchedAt: fetchedAt,
		},
		Status: SourceEvidence{
			DependencyID: statusDependency, Authenticity: SourceMockOnly, SourceVersion: &statusVersion,
			FetchedAt: fetchedAt.Add(-time.Hour),
		},
	}
	return snapshot, provenance
}

func reverseStrings(values []string) []string {
	out := append([]string(nil), values...)
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}
