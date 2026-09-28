package newapipricing

import (
	"slices"
	"testing"
	"time"

	"kwu/internal/modelbilling"
)

func TestEstimateTokenRatioUsesExactSplitDimensions(t *testing.T) {
	cached := estimateTestSnapshot()
	cached.Provenance.Status.Authenticity = SourceReady
	config := modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}
	result := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "vip",
		Usage: modelbilling.NormalizedUsage{
			InputTokens: ptrInt64(3_000_000), NormalInputTokens: ptrInt64(1_000_000),
			CachedInputTokens: ptrInt64(2_000_000), CacheReadTokens: ptrInt64(1_000_000),
			CacheWriteTokens: ptrInt64(1_000_000), OutputTokens: ptrInt64(1_000_000),
			CachedOutputTokens: ptrInt64(250_000), ImageTokens: ptrInt64(1_000_000),
			AudioInputTokens: ptrInt64(1_000_000), AudioOutputTokens: ptrInt64(1_000_000),
			BreakdownStatus: modelbilling.UsageExact,
		},
		CreditConfig: &CreditConfigInput{Config: config, Authenticity: SourceReady},
	})

	if result.State != EstimateExact || !result.Complete {
		t.Fatalf("Estimate state = %q complete=%v reasons=%v", result.State, result.Complete, result.Reasons)
	}
	assertAmount(t, "USD", result.USD, "31.35", false)
	assertAmount(t, "CNY", result.CNY, "225.72", false)
	assertAmount(t, "credits", result.Credits, "451440", false)
	if !slices.Contains(result.Fallbacks, FallbackCachedOutputStandardRate) {
		t.Fatalf("fallbacks = %v", result.Fallbacks)
	}
}

func TestEstimatePartialCalculatesOnlyKnownDimensions(t *testing.T) {
	cached := estimateTestSnapshot()
	result := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default",
		Usage: modelbilling.NormalizedUsage{
			NormalInputTokens: ptrInt64(1_000_000), OutputTokens: ptrInt64(1_000_000),
			BreakdownStatus: modelbilling.UsagePartial,
		},
	})

	if result.State != EstimateDegraded || result.Complete {
		t.Fatalf("Estimate state = %q complete=%v", result.State, result.Complete)
	}
	assertAmount(t, "partial USD", result.USD, "6", false)
	if !slices.Contains(result.Reasons, ReasonUsagePartial) ||
		!slices.Contains(result.MissingUsage, UsageCacheRead) ||
		!slices.Contains(result.MissingUsage, UsageCacheWrite) {
		t.Fatalf("partial reasons=%v missing=%v", result.Reasons, result.MissingUsage)
	}
}

func TestEstimateTextModelExactDoesNotRequireMediaUsage(t *testing.T) {
	cached := textEstimateTestSnapshot()
	cached.Provenance.Status.Authenticity = SourceReady
	usage := exactTextUsage()
	result := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: usage,
		CreditConfig: &CreditConfigInput{
			Config:       modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart},
			Authenticity: SourceReady,
		},
	})

	if result.State != EstimateExact || !result.Complete || len(result.MissingUsage) != 0 {
		t.Fatalf("text estimate = %+v", result)
	}
}

func TestEstimateTextModelPartialOmitsInapplicableMediaMissingUsage(t *testing.T) {
	usage := exactTextUsage()
	usage.CacheWriteTokens = nil
	usage.BreakdownStatus = modelbilling.UsagePartial
	result := Estimate(EstimateInput{
		CachedSnapshot: textEstimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default", Usage: usage,
	})

	if result.State != EstimateDegraded || result.Complete || !slices.Equal(result.MissingUsage, []UsageDimension{UsageCacheWrite}) {
		t.Fatalf("text partial estimate = %+v", result)
	}
}

func TestEstimateExactRequiresApplicableMediaUsage(t *testing.T) {
	result := Estimate(EstimateInput{
		CachedSnapshot: estimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default", Usage: exactTextUsage(),
	})

	if result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonUsageAmbiguous) ||
		!slices.Equal(result.MissingUsage, []UsageDimension{UsageImage, UsageAudioInput, UsageAudioOutput}) {
		t.Fatalf("media-backed exact estimate = %+v", result)
	}
}

func TestEstimateMediaUsageWithoutPriceReturnsPricingMissing(t *testing.T) {
	for name, setUsage := range map[string]func(*modelbilling.NormalizedUsage){
		"image":        func(usage *modelbilling.NormalizedUsage) { usage.ImageTokens = ptrInt64(1) },
		"audio_input":  func(usage *modelbilling.NormalizedUsage) { usage.AudioInputTokens = ptrInt64(1) },
		"audio_output": func(usage *modelbilling.NormalizedUsage) { usage.AudioOutputTokens = ptrInt64(1) },
	} {
		t.Run(name, func(t *testing.T) {
			usage := exactTextUsage()
			setUsage(&usage)
			result := Estimate(EstimateInput{
				CachedSnapshot: textEstimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default", Usage: usage,
			})

			if result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonPricingMissing) {
				t.Fatalf("unpriced media estimate = %+v", result)
			}
		})
	}
}

func TestEstimatePartialWithoutKnownDimensionsDoesNotReturnZeroAmount(t *testing.T) {
	result := Estimate(EstimateInput{
		CachedSnapshot: estimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default",
		Usage: modelbilling.NormalizedUsage{BreakdownStatus: modelbilling.UsagePartial},
	})
	if result.State != EstimateDegraded || result.Complete || result.USD.Value != nil || len(result.MissingUsage) != 7 {
		t.Fatalf("empty token partial = %+v", result)
	}

	result = Estimate(EstimateInput{
		CachedSnapshot: estimateTestSnapshot(), ModelID: "request-model", PricingGroup: "default",
		Usage: modelbilling.NormalizedUsage{BreakdownStatus: modelbilling.UsagePartial},
	})
	if result.State != EstimateDegraded || result.Complete || result.USD.Value != nil ||
		!slices.Contains(result.MissingUsage, UsageRequest) {
		t.Fatalf("empty request partial = %+v", result)
	}
}

func TestEstimateRejectsAmbiguousUsageTotals(t *testing.T) {
	base := EstimateInput{
		CachedSnapshot: estimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default",
		Usage: exactZeroUsage(),
	}
	base.Usage.InputTokens = ptrInt64(2)
	base.Usage.NormalInputTokens = ptrInt64(1)
	if result := Estimate(base); result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonUsageAmbiguous) {
		t.Fatalf("input conflict result = %+v", result)
	}

	base.Usage = exactZeroUsage()
	base.Usage.OutputTokens = ptrInt64(1)
	base.Usage.CachedOutputTokens = ptrInt64(2)
	if result := Estimate(base); result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonUsageAmbiguous) {
		t.Fatalf("cached output conflict result = %+v", result)
	}
}

func TestEstimateRejectsUnconfirmedDirectPriceContracts(t *testing.T) {
	cached := estimateTestSnapshot()
	direct := modelbilling.DecimalString("1")
	cached.Snapshot.Models[0].InputPerMillion = &direct
	result := Estimate(EstimateInput{CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage()})
	if result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonPricingContractAmbiguous) {
		t.Fatalf("mixed direct price result = %+v", result)
	}

	cached = estimateTestSnapshot()
	cached.Snapshot.Models[0].ModelRatio = nil
	cached.Snapshot.Models[0].CompletionRatio = nil
	cached.Snapshot.Models[0].CacheRatio = nil
	cached.Snapshot.Models[0].CreateCacheRatio = nil
	cached.Snapshot.Models[0].ImageRatio = nil
	cached.Snapshot.Models[0].AudioRatio = nil
	cached.Snapshot.Models[0].AudioCompletionRatio = nil
	cached.Snapshot.Models[0].InputPerMillion = &direct
	result = Estimate(EstimateInput{CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage()})
	if result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonDirectPriceContractUnconfirmed) {
		t.Fatalf("direct-only result = %+v", result)
	}

	cached = estimateTestSnapshot()
	cached.Snapshot.Models[1].ModelPrice = nil
	cached.Snapshot.Models[1].RequestPerUnit = &direct
	result = Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "request-model", PricingGroup: "default",
		Usage: modelbilling.NormalizedUsage{RequestCount: ptrInt64(1), BreakdownStatus: modelbilling.UsageExact},
	})
	if result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonDirectPriceContractUnconfirmed) {
		t.Fatalf("request direct-only result = %+v", result)
	}
}

func TestEstimatePerRequestAndMockSources(t *testing.T) {
	cached := estimateTestSnapshot()
	config := modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}
	result := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "request-model", PricingGroup: "vip",
		Usage:        modelbilling.NormalizedUsage{RequestCount: ptrInt64(3), BreakdownStatus: modelbilling.UsageExact},
		CreditConfig: &CreditConfigInput{Config: config, Authenticity: SourceMockOnly},
	})
	if result.State != EstimateDegraded || !result.Complete {
		t.Fatalf("mock result state=%q complete=%v reasons=%v", result.State, result.Complete, result.Reasons)
	}
	assertAmount(t, "request USD", result.USD, "0.75", false)
	assertAmount(t, "request CNY", result.CNY, "5.4", true)
	assertAmount(t, "request credits", result.Credits, "10800", true)
	if !slices.Contains(result.Reasons, ReasonMockStatusSource) || !slices.Contains(result.Reasons, ReasonMockCreditConfig) {
		t.Fatalf("mock reasons = %v", result.Reasons)
	}
}

func TestEstimatePreservesVerySmallFiniteDecimalExactly(t *testing.T) {
	cached := estimateTestSnapshot()
	tiny := modelbilling.DecimalString("0.000000000000000001")
	cached.Snapshot.Models[0].ModelRatio = &tiny
	cached.Snapshot.GroupRatios["default"] = tiny
	usage := exactZeroUsage()
	usage.NormalInputTokens = ptrInt64(1)
	usage.InputTokens = ptrInt64(1)
	result := Estimate(EstimateInput{CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: usage})
	assertAmount(t, "tiny USD", result.USD, "0.000000000000000000000000000000000000000002", false)
}

func TestEstimateRejectsUnavailableUsageWithoutZeroPrice(t *testing.T) {
	for _, test := range []struct {
		status modelbilling.UsageBreakdownStatus
		reason EstimateReason
	}{
		{status: modelbilling.UsageUnavailable, reason: ReasonUsageUnavailable},
		{status: modelbilling.UsageMissing, reason: ReasonUsageMissing},
	} {
		result := Estimate(EstimateInput{
			CachedSnapshot: estimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default",
			Usage: modelbilling.NormalizedUsage{BreakdownStatus: test.status},
		})
		if result.State != EstimateUnavailable || result.USD.Value != nil || !slices.Contains(result.Reasons, test.reason) {
			t.Fatalf("status %q result = %+v", test.status, result)
		}
	}
}

func TestEstimateHandlesLargeIntegersAndLegalZero(t *testing.T) {
	usage := exactZeroUsage()
	usage.NormalInputTokens = ptrInt64(9_223_372_036_854_775_807)
	usage.InputTokens = usage.NormalInputTokens
	result := Estimate(EstimateInput{CachedSnapshot: estimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default", Usage: usage})
	assertAmount(t, "large USD", result.USD, "18446744073709.551614", false)

	zero := Estimate(EstimateInput{CachedSnapshot: estimateTestSnapshot(), ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage()})
	assertAmount(t, "zero USD", zero.USD, "0", false)
}

func TestEstimateRejectsMissingPriceAndInvalidGroup(t *testing.T) {
	cached := estimateTestSnapshot()
	cached.Snapshot.Models[0].CompletionRatio = nil
	usage := exactZeroUsage()
	usage.OutputTokens = ptrInt64(1)
	result := Estimate(EstimateInput{CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: usage})
	if result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonPricingMissing) {
		t.Fatalf("missing completion price = %+v", result)
	}

	result = Estimate(EstimateInput{CachedSnapshot: estimateTestSnapshot(), ModelID: "model-a", PricingGroup: "missing", Usage: exactZeroUsage()})
	if result.State != EstimateUnavailable || !slices.Contains(result.Reasons, ReasonPricingGroupInvalid) {
		t.Fatalf("invalid group = %+v", result)
	}
}

func estimateTestSnapshot() CachedSnapshot {
	one := modelbilling.DecimalString("1")
	two := modelbilling.DecimalString("2")
	cacheRead := modelbilling.DecimalString("0.1")
	cacheWrite := modelbilling.DecimalString("1.25")
	image := modelbilling.DecimalString("3")
	audio := modelbilling.DecimalString("4")
	audioOutput := modelbilling.DecimalString("5")
	request := modelbilling.DecimalString("0.5")
	exchange := modelbilling.DecimalString("7.2")
	version := "pricing-v1"
	statusVersion := "status-v1"
	return CachedSnapshot{
		Snapshot: modelbilling.PricingSnapshot{
			SnapshotID: "pricing-v1:domestic:test", Source: "domestic", SourceVersion: &version,
			SourceHash: "sha256:test", Currency: "CNY", ExchangeRate: &exchange,
			GroupRatios:  map[string]modelbilling.DecimalString{"default": "1", "vip": "0.5"},
			UsableGroups: []string{"default", "vip"}, FetchedAt: cacheTestStart,
			FreshUntil: cacheTestStart.Add(12 * time.Hour), StaleUntil: cacheTestStart.Add(24 * time.Hour),
			State: modelbilling.PricingFresh,
			Models: []modelbilling.ModelPrice{
				{ModelID: "model-a", ProviderModelID: "model-a", ModelName: "Model A", ModelType: "chat",
					QuotaType: modelbilling.QuotaTypeToken, BillingMode: modelbilling.BillingModeTokenRatio,
					ModelRatio: &one, CompletionRatio: &two, CacheRatio: &cacheRead,
					CreateCacheRatio: &cacheWrite, ImageRatio: &image, AudioRatio: &audio,
					AudioCompletionRatio: &audioOutput, EnableGroups: []string{"default", "vip"}},
				{ModelID: "request-model", ProviderModelID: "request-model", ModelName: "Request", ModelType: "image",
					QuotaType: modelbilling.QuotaTypePerRequest, BillingMode: modelbilling.BillingModePerRequest,
					ModelPrice: &request, EnableGroups: []string{"default", "vip"}},
			},
		},
		Provenance: SnapshotProvenance{
			Pricing: SourceEvidence{DependencyID: pricingDependency, Authenticity: SourceReady, SourceVersion: &version, FetchedAt: cacheTestStart},
			Status:  SourceEvidence{DependencyID: statusDependency, Authenticity: SourceMockOnly, SourceVersion: &statusVersion, FetchedAt: cacheTestStart},
		},
	}
}

func textEstimateTestSnapshot() CachedSnapshot {
	cached := estimateTestSnapshot()
	cached.Snapshot.Models[0].ImageRatio = nil
	cached.Snapshot.Models[0].AudioRatio = nil
	cached.Snapshot.Models[0].AudioCompletionRatio = nil
	return cached
}

func exactTextUsage() modelbilling.NormalizedUsage {
	return modelbilling.NormalizedUsage{
		InputTokens: ptrInt64(10), NormalInputTokens: ptrInt64(8), CachedInputTokens: ptrInt64(2),
		CacheReadTokens: ptrInt64(2), CacheWriteTokens: ptrInt64(0), OutputTokens: ptrInt64(4),
		BreakdownStatus: modelbilling.UsageExact,
	}
}

func exactZeroUsage() modelbilling.NormalizedUsage {
	return modelbilling.NormalizedUsage{
		InputTokens: ptrInt64(0), NormalInputTokens: ptrInt64(0), CachedInputTokens: ptrInt64(0),
		CacheReadTokens: ptrInt64(0), CacheWriteTokens: ptrInt64(0), OutputTokens: ptrInt64(0),
		ImageTokens: ptrInt64(0), AudioInputTokens: ptrInt64(0), AudioOutputTokens: ptrInt64(0),
		BreakdownStatus: modelbilling.UsageExact,
	}
}

func ptrInt64(value int64) *int64 { return &value }

func assertAmount(t *testing.T, name string, got EstimatedAmount, want string, mock bool) {
	t.Helper()
	if got.Value == nil || string(*got.Value) != want || got.Mock != mock {
		t.Fatalf("%s = %+v, want value %q mock=%v", name, got, want, mock)
	}
}
