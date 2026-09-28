package newapipricing

import (
	"slices"

	"kwu/internal/modelbilling"
)

type EstimateState string

const (
	EstimateExact       EstimateState = "exact"
	EstimateDegraded    EstimateState = "degraded"
	EstimateUnavailable EstimateState = "unavailable"
)

type EstimateReason string

const (
	ReasonSnapshotUnavailable            EstimateReason = "snapshot_unavailable"
	ReasonSnapshotStaleUsable            EstimateReason = "stale_usable"
	ReasonStaleBlocked                   EstimateReason = "stale_blocked"
	ReasonModelNotFound                  EstimateReason = "model_not_found"
	ReasonPricingGroupInvalid            EstimateReason = "pricing_group_invalid"
	ReasonPricingMissing                 EstimateReason = "pricing_missing"
	ReasonPricingUnavailable             EstimateReason = "pricing_unavailable"
	ReasonPricingGroupUnavailable        EstimateReason = "pricing_group_unavailable"
	ReasonBillingSourceInvalid           EstimateReason = "billing_source_invalid"
	ReasonEvidenceMismatch               EstimateReason = "evidence_mismatch"
	ReasonPricingContractAmbiguous       EstimateReason = "pricing_contract_ambiguous"
	ReasonDirectPriceContractUnconfirmed EstimateReason = "direct_price_contract_unconfirmed"
	ReasonDynamicExpressionUnsupported   EstimateReason = "dynamic_expression_unsupported"
	ReasonUsageMissing                   EstimateReason = "usage_missing"
	ReasonUsageUnavailable               EstimateReason = "usage_unavailable"
	ReasonUsagePartial                   EstimateReason = "usage_partial"
	ReasonUsageAmbiguous                 EstimateReason = "usage_ambiguous"
	ReasonCreditConfigUnavailable        EstimateReason = "credit_config_unavailable"
	ReasonMockStatusSource               EstimateReason = "mock_status_source"
	ReasonMockCreditConfig               EstimateReason = "mock_credit_config"
	ReasonExchangeRateUnavailable        EstimateReason = "exchange_rate_unavailable"
)

type EstimateFallback string

const FallbackCachedOutputStandardRate EstimateFallback = "cached_output_standard_rate"

type UsageDimension string

const (
	UsageNormalInput UsageDimension = "normal_input_tokens"
	UsageCacheRead   UsageDimension = "cache_read_tokens"
	UsageCacheWrite  UsageDimension = "cache_write_tokens"
	UsageOutput      UsageDimension = "output_tokens"
	UsageImage       UsageDimension = "image_tokens"
	UsageAudioInput  UsageDimension = "audio_input_tokens"
	UsageAudioOutput UsageDimension = "audio_output_tokens"
	UsageRequest     UsageDimension = "request_count"
)

type EstimatedAmount struct {
	Value *modelbilling.DecimalString `json:"value,omitempty"`
	Mock  bool                        `json:"mock,omitempty"`
}

type CreditConfigInput struct {
	Config       modelbilling.CreditConfig
	Authenticity SourceAuthenticity
}

type EstimateInput struct {
	CachedSnapshot CachedSnapshot
	ModelID        string
	PricingGroup   string
	Usage          modelbilling.NormalizedUsage
	CreditConfig   *CreditConfigInput
}

type EstimateResult struct {
	State        EstimateState
	Complete     bool
	SnapshotID   string
	ModelID      string
	PricingGroup string
	PricingState modelbilling.PricingState
	USD          EstimatedAmount
	CNY          EstimatedAmount
	Credits      EstimatedAmount
	Reasons      []EstimateReason
	MissingUsage []UsageDimension
	Fallbacks    []EstimateFallback
}

func Estimate(input EstimateInput) EstimateResult {
	result := newEstimateResult(input)
	snapshot := input.CachedSnapshot.Snapshot
	if reason := snapshotEstimateReason(snapshot, input.ModelID); reason != "" {
		return unavailableEstimate(result, reason)
	}
	applySnapshotDegradation(&result, snapshot.State)
	model, ok := findModel(snapshot.Models, input.ModelID)
	if !ok {
		return unavailableEstimate(result, ReasonModelNotFound)
	}
	groupRatio, ok := validGroup(snapshot, model, input.PricingGroup)
	if !ok {
		return unavailableEstimate(result, ReasonPricingGroupInvalid)
	}
	if err := input.Usage.ValidateOptional(); err != nil {
		return unavailableEstimate(result, ReasonUsageAmbiguous)
	}
	applyCachedOutputFallback(&result, input.Usage)
	total, success := estimateByMode(&result, model, groupRatio, input.Usage)
	if !success {
		return result
	}
	applyCurrencies(&result, total, input.CachedSnapshot.Provenance, snapshot.ExchangeRate, input.CreditConfig)
	return result
}

func newEstimateResult(input EstimateInput) EstimateResult {
	return EstimateResult{
		State: EstimateExact, Complete: true, SnapshotID: input.CachedSnapshot.Snapshot.SnapshotID,
		ModelID: input.ModelID, PricingGroup: input.PricingGroup,
		PricingState: input.CachedSnapshot.Snapshot.State,
	}
}

func snapshotEstimateReason(snapshot modelbilling.PricingSnapshot, modelID string) EstimateReason {
	if model, ok := findModel(snapshot.Models, modelID); ok {
		if reason := directPriceReason(model); reason != "" {
			return reason
		}
	}
	if err := snapshot.Validate(); err != nil || snapshot.State == modelbilling.PricingUnavailable {
		return ReasonSnapshotUnavailable
	}
	if snapshot.State == modelbilling.PricingStaleBlocked {
		return ReasonStaleBlocked
	}
	return ""
}

func applySnapshotDegradation(result *EstimateResult, state modelbilling.PricingState) {
	if state == modelbilling.PricingStaleUsable {
		degradeEstimate(result, ReasonSnapshotStaleUsable)
	}
}

func applyCachedOutputFallback(result *EstimateResult, usage modelbilling.NormalizedUsage) {
	if usage.CachedOutputTokens != nil && *usage.CachedOutputTokens > 0 {
		result.Fallbacks = append(result.Fallbacks, FallbackCachedOutputStandardRate)
	}
}

func estimateByMode(result *EstimateResult, model modelbilling.ModelPrice, group modelbilling.DecimalString, usage modelbilling.NormalizedUsage) (exactDecimal, bool) {
	switch model.BillingMode {
	case modelbilling.BillingModeTokenRatio:
		return estimateTokenRatio(result, model, group, usage)
	case modelbilling.BillingModePerRequest:
		return estimatePerRequest(result, model, group, usage)
	case modelbilling.BillingModeTieredExpr:
		*result = unavailableEstimate(*result, ReasonDynamicExpressionUnsupported)
		return exactDecimal{}, false
	}
	*result = unavailableEstimate(*result, ReasonPricingUnavailable)
	return exactDecimal{}, false
}

func estimateTokenRatio(result *EstimateResult, model modelbilling.ModelPrice, group modelbilling.DecimalString, usage modelbilling.NormalizedUsage) (exactDecimal, bool) {
	if !validateEstimableUsageStatus(result, usage.BreakdownStatus) {
		return exactDecimal{}, false
	}
	dimensions := []struct {
		name     UsageDimension
		value    *int64
		required bool
		unit     func(modelbilling.ModelPrice, modelbilling.DecimalString) (exactDecimal, bool)
	}{
		{UsageNormalInput, usage.NormalInputTokens, true, normalInputUnit},
		{UsageCacheRead, usage.CacheReadTokens, true, cacheReadUnit},
		{UsageCacheWrite, usage.CacheWriteTokens, true, cacheWriteUnit},
		{UsageOutput, usage.OutputTokens, true, outputUnit},
		{UsageImage, usage.ImageTokens, model.ImageRatio != nil, imageUnit},
		{UsageAudioInput, usage.AudioInputTokens, model.AudioRatio != nil, audioInputUnit},
		{UsageAudioOutput, usage.AudioOutputTokens, model.AudioRatio != nil && model.AudioCompletionRatio != nil, audioOutputUnit},
	}
	total := zeroExactDecimal()
	knownDimensions := 0
	for _, dimension := range dimensions {
		if dimension.value == nil {
			if dimension.required {
				result.MissingUsage = append(result.MissingUsage, dimension.name)
			}
			continue
		}
		knownDimensions++
		if *dimension.value == 0 {
			continue
		}
		unit, ok := dimension.unit(model, group)
		if !ok {
			*result = unavailableEstimate(*result, ReasonPricingMissing)
			return exactDecimal{}, false
		}
		total = total.add(unit.multiplyInt64(*dimension.value).divideMillion())
	}
	if !finalizeUsageCompleteness(result, usage.BreakdownStatus) {
		return exactDecimal{}, false
	}
	if knownDimensions == 0 {
		return exactDecimal{}, false
	}
	return total, true
}

func validateEstimableUsageStatus(result *EstimateResult, status modelbilling.UsageBreakdownStatus) bool {
	switch status {
	case modelbilling.UsageExact, modelbilling.UsagePartial:
		return true
	case modelbilling.UsageUnavailable:
		*result = unavailableEstimate(*result, ReasonUsageUnavailable)
	case modelbilling.UsageMissing:
		*result = unavailableEstimate(*result, ReasonUsageMissing)
	default:
		*result = unavailableEstimate(*result, ReasonUsageAmbiguous)
	}
	return false
}

func finalizeUsageCompleteness(result *EstimateResult, status modelbilling.UsageBreakdownStatus) bool {
	if len(result.MissingUsage) > 0 && status == modelbilling.UsageExact {
		*result = unavailableEstimate(*result, ReasonUsageAmbiguous)
		return false
	}
	if len(result.MissingUsage) > 0 || status == modelbilling.UsagePartial {
		result.Complete = false
		degradeEstimate(result, ReasonUsagePartial)
	}
	return true
}

func estimatePerRequest(result *EstimateResult, model modelbilling.ModelPrice, group modelbilling.DecimalString, usage modelbilling.NormalizedUsage) (exactDecimal, bool) {
	if usage.BreakdownStatus == modelbilling.UsageUnavailable {
		*result = unavailableEstimate(*result, ReasonUsageUnavailable)
		return exactDecimal{}, false
	}
	if usage.BreakdownStatus == modelbilling.UsageMissing {
		*result = unavailableEstimate(*result, ReasonUsageMissing)
		return exactDecimal{}, false
	}
	if usage.BreakdownStatus != modelbilling.UsageExact && usage.BreakdownStatus != modelbilling.UsagePartial {
		*result = unavailableEstimate(*result, ReasonUsageAmbiguous)
		return exactDecimal{}, false
	}
	if usage.RequestCount == nil {
		result.MissingUsage = append(result.MissingUsage, UsageRequest)
		if usage.BreakdownStatus == modelbilling.UsageExact {
			*result = unavailableEstimate(*result, ReasonUsageAmbiguous)
			return exactDecimal{}, false
		}
		result.Complete = false
		degradeEstimate(result, ReasonUsagePartial)
		return exactDecimal{}, false
	}
	if model.ModelPrice == nil {
		*result = unavailableEstimate(*result, ReasonPricingMissing)
		return exactDecimal{}, false
	}
	if usage.BreakdownStatus == modelbilling.UsagePartial {
		result.Complete = false
		degradeEstimate(result, ReasonUsagePartial)
	}
	return parseExactDecimal(*model.ModelPrice).multiply(parseExactDecimal(group)).multiplyInt64(*usage.RequestCount), true
}

func applyCurrencies(result *EstimateResult, total exactDecimal, provenance SnapshotProvenance, exchange *modelbilling.DecimalString, credit *CreditConfigInput) {
	usd := total.decimalString()
	result.USD.Value = &usd
	if exchange == nil {
		degradeEstimate(result, ReasonExchangeRateUnavailable)
		return
	}
	cnyValue := total.multiply(parseExactDecimal(*exchange)).decimalString()
	cnyMock := provenance.Status.Authenticity == SourceMockOnly
	result.CNY = EstimatedAmount{Value: &cnyValue, Mock: cnyMock}
	if cnyMock {
		degradeEstimate(result, ReasonMockStatusSource)
	}
	if credit == nil || credit.Config.Validate() != nil ||
		(credit.Authenticity != SourceReady && credit.Authenticity != SourceMockOnly) {
		degradeEstimate(result, ReasonCreditConfigUnavailable)
		return
	}
	creditsValue := parseExactDecimal(cnyValue).multiply(parseExactDecimal(credit.Config.CreditPerCNY)).decimalString()
	creditMock := cnyMock || credit.Authenticity == SourceMockOnly
	result.Credits = EstimatedAmount{Value: &creditsValue, Mock: creditMock}
	if credit.Authenticity == SourceMockOnly {
		degradeEstimate(result, ReasonMockCreditConfig)
	}
}

func normalInputUnit(model modelbilling.ModelPrice, group modelbilling.DecimalString) (exactDecimal, bool) {
	return multipliedUnit(group, model.ModelRatio)
}

func cacheReadUnit(model modelbilling.ModelPrice, group modelbilling.DecimalString) (exactDecimal, bool) {
	return multipliedUnit(group, model.ModelRatio, model.CacheRatio)
}

func cacheWriteUnit(model modelbilling.ModelPrice, group modelbilling.DecimalString) (exactDecimal, bool) {
	return multipliedUnit(group, model.ModelRatio, model.CreateCacheRatio)
}

func outputUnit(model modelbilling.ModelPrice, group modelbilling.DecimalString) (exactDecimal, bool) {
	return multipliedUnit(group, model.ModelRatio, model.CompletionRatio)
}

func imageUnit(model modelbilling.ModelPrice, group modelbilling.DecimalString) (exactDecimal, bool) {
	return multipliedUnit(group, model.ModelRatio, model.ImageRatio)
}

func audioInputUnit(model modelbilling.ModelPrice, group modelbilling.DecimalString) (exactDecimal, bool) {
	return multipliedUnit(group, model.ModelRatio, model.AudioRatio)
}

func audioOutputUnit(model modelbilling.ModelPrice, group modelbilling.DecimalString) (exactDecimal, bool) {
	return multipliedUnit(group, model.ModelRatio, model.AudioRatio, model.AudioCompletionRatio)
}

func multipliedUnit(group modelbilling.DecimalString, values ...*modelbilling.DecimalString) (exactDecimal, bool) {
	unit := parseExactDecimal("2").multiply(parseExactDecimal(group))
	for _, value := range values {
		if value == nil {
			return exactDecimal{}, false
		}
		unit = unit.multiply(parseExactDecimal(*value))
	}
	return unit, true
}

func findModel(models []modelbilling.ModelPrice, modelID string) (modelbilling.ModelPrice, bool) {
	for _, model := range models {
		if model.ModelID == modelID {
			return model, true
		}
	}
	return modelbilling.ModelPrice{}, false
}

func validGroup(snapshot modelbilling.PricingSnapshot, model modelbilling.ModelPrice, group string) (modelbilling.DecimalString, bool) {
	ratio, ok := snapshot.GroupRatios[group]
	return ratio, ok && slices.Contains(snapshot.UsableGroups, group) && slices.Contains(model.EnableGroups, group)
}

func directPriceReason(model modelbilling.ModelPrice) EstimateReason {
	if !containsPrice([]*modelbilling.DecimalString{
		model.InputPerMillion, model.CacheReadPerMillion, model.CacheWritePerMillion,
		model.OutputPerMillion, model.ImageInputPerMillion, model.AudioInputPerMillion,
		model.AudioOutputPerMillion, model.RequestPerUnit,
	}) {
		return ""
	}
	if containsPrice([]*modelbilling.DecimalString{
		model.ModelPrice, model.ModelRatio, model.CompletionRatio, model.CacheRatio,
		model.CreateCacheRatio, model.ImageRatio, model.AudioRatio, model.AudioCompletionRatio,
	}) {
		return ReasonPricingContractAmbiguous
	}
	return ReasonDirectPriceContractUnconfirmed
}

func containsPrice(values []*modelbilling.DecimalString) bool {
	for _, value := range values {
		if value != nil {
			return true
		}
	}
	return false
}

func unavailableEstimate(result EstimateResult, reason EstimateReason) EstimateResult {
	result.State = EstimateUnavailable
	result.Complete = false
	result.USD = EstimatedAmount{}
	result.CNY = EstimatedAmount{}
	result.Credits = EstimatedAmount{}
	result.Reasons = appendUniqueReason(result.Reasons, reason)
	return result
}

func degradeEstimate(result *EstimateResult, reason EstimateReason) {
	if result.State == EstimateExact {
		result.State = EstimateDegraded
	}
	result.Reasons = appendUniqueReason(result.Reasons, reason)
}

func appendUniqueReason(reasons []EstimateReason, reason EstimateReason) []EstimateReason {
	if !slices.Contains(reasons, reason) {
		return append(reasons, reason)
	}
	return reasons
}
