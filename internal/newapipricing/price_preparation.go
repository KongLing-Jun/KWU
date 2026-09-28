package newapipricing

import (
	"slices"
	"time"

	"kwu/internal/modelbilling"
)

type PricePreparationInput struct {
	Source        string
	BillingSource modelbilling.BillingSource
	EvidenceID    string
	TaskID        string
	Scope         modelbilling.TaskScope
	ModelID       string
	PricingGroup  string
	Usage         modelbilling.NormalizedUsage
	CreditConfig  *CreditConfigInput
	CapturedAt    time.Time
}

type PreparationFailure struct {
	Code   modelbilling.ErrorCode
	Reason EstimateReason
}

type PricePreparationResult struct {
	CachedSnapshot *CachedSnapshot
	Estimate       EstimateResult
	Evidence       *modelbilling.PricingEvidence
	Decision       StartDecision
	Failure        *PreparationFailure
}

// PricePreparationReader is B2's cache-only B1 boundary. It has no network,
// persistence writes, task, freeze, balance, or billing-source switching behavior.
type PricePreparationReader struct {
	reader *Reader
}

func NewPricePreparationReader(reader *Reader) (*PricePreparationReader, error) {
	if reader == nil {
		return nil, contractError("pricing reader is required")
	}
	return &PricePreparationReader{reader: reader}, nil
}

func (r *PricePreparationReader) Prepare(input PricePreparationInput) PricePreparationResult {
	if err := input.BillingSource.Validate(); err != nil {
		return rejectedPreparation(input, modelbilling.ErrorCodeOf(err), StartBillingSourceInvalid)
	}
	if err := input.Scope.Validate(input.BillingSource); err != nil {
		return rejectedPreparation(input, modelbilling.ErrorCodeOf(err), StartEvidenceMismatch)
	}
	if r == nil || r.reader == nil {
		return rejectedPreparation(input, modelbilling.ErrorContractInvalid, StartEvidenceMissing)
	}

	cached, err := r.reader.Read(input.Source)
	if err != nil {
		return preparationFromReadError(input, err)
	}

	result := newPreparationResult(input, cached)
	result.CachedSnapshot = &cached

	evidence, evidenceErr := BuildPricingEvidence(cached, pricingEvidenceRequest(input))
	if evidenceErr != nil {
		return rejectedPreparationWithSnapshot(result, input, modelbilling.ErrorCodeOf(evidenceErr))
	}
	result.Evidence = &evidence
	if result.Estimate.State == EstimateUnavailable {
		reason := firstEstimateReason(result.Estimate)
		result.Failure = &PreparationFailure{Code: modelbilling.ErrorPricingUnavailable, Reason: reason}
	}

	result.Decision = DecideModelStart(StartDecisionInput{
		BillingSource:  input.BillingSource,
		TaskID:         input.TaskID,
		Scope:          input.Scope,
		Estimate:       result.Estimate,
		Evidence:       result.Evidence,
		RequireCredits: input.BillingSource == modelbilling.BillingSubscriptionPool,
	})
	return result
}

func preparationFromReadError(input PricePreparationInput, readErr error) PricePreparationResult {
	result := newPreparationResult(input, CachedSnapshot{})
	code := modelbilling.ErrorCodeOf(readErr)
	if code == "" {
		code = modelbilling.ErrorContractInvalid
	}
	result.Failure = &PreparationFailure{Code: code, Reason: ReasonSnapshotUnavailable}
	if code != modelbilling.ErrorPricingUnavailable || input.BillingSource != modelbilling.BillingNewAPIByok {
		result.Decision = blockedPreparationDecision(input.BillingSource, result.Estimate, StartEvidenceMissing)
		return result
	}

	evidence, err := BuildUnavailablePricingEvidence(input.Source, readErr, pricingEvidenceRequest(input))
	if err != nil {
		result.Failure.Code = modelbilling.ErrorCodeOf(err)
		result.Decision = blockedPreparationDecision(input.BillingSource, result.Estimate, StartEvidenceMismatch)
		return result
	}
	result.Evidence = &evidence
	result.Estimate.SnapshotID = evidence.SnapshotID
	result.Estimate.PricingState = modelbilling.PricingUnavailable
	result.Decision = DecideModelStart(StartDecisionInput{
		BillingSource: input.BillingSource, TaskID: input.TaskID, Scope: input.Scope,
		Estimate: result.Estimate, Evidence: result.Evidence,
	})
	return result
}

func pricingEvidenceRequest(input PricePreparationInput) PricingEvidenceRequest {
	return PricingEvidenceRequest{
		EvidenceID: input.EvidenceID, TaskID: input.TaskID, Scope: input.Scope,
		BillingSource: input.BillingSource, ModelID: input.ModelID,
		PricingGroup: input.PricingGroup, CapturedAt: input.CapturedAt,
	}
}

func newPreparationResult(input PricePreparationInput, cached CachedSnapshot) PricePreparationResult {
	estimate := Estimate(EstimateInput{
		CachedSnapshot: cached,
		ModelID:        input.ModelID,
		PricingGroup:   input.PricingGroup,
		Usage:          input.Usage,
		CreditConfig:   input.CreditConfig,
	})
	return PricePreparationResult{
		Estimate: estimate,
		Decision: DecideModelStart(StartDecisionInput{
			BillingSource:  input.BillingSource,
			TaskID:         input.TaskID,
			Scope:          input.Scope,
			Estimate:       estimate,
			RequireCredits: input.BillingSource == modelbilling.BillingSubscriptionPool,
		}),
	}
}

func rejectedPreparation(input PricePreparationInput, code modelbilling.ErrorCode, reason StartReason) PricePreparationResult {
	result := newPreparationResult(input, CachedSnapshot{})
	result.Decision = blockedPreparationDecision(input.BillingSource, result.Estimate, reason)
	result.Failure = &PreparationFailure{Code: code, Reason: firstEstimateReason(result.Estimate)}
	return result
}

func rejectedPreparationWithSnapshot(result PricePreparationResult, input PricePreparationInput, code modelbilling.ErrorCode) PricePreparationResult {
	result.Evidence = nil
	result.Decision = blockedPreparationDecision(input.BillingSource, result.Estimate, StartEvidenceMismatch)
	result.Failure = &PreparationFailure{Code: code, Reason: firstEstimateReason(result.Estimate)}
	return result
}

func blockedPreparationDecision(source modelbilling.BillingSource, estimate EstimateResult, reason StartReason) StartDecision {
	return StartDecision{
		Action:          StartBlock,
		BillingSource:   source,
		Reasons:         []StartReason{reason},
		EstimateReasons: append([]EstimateReason(nil), estimate.Reasons...),
	}
}

func firstEstimateReason(estimate EstimateResult) EstimateReason {
	if slices.Contains(estimate.Reasons, ReasonModelNotFound) {
		return ReasonModelNotFound
	}
	if slices.Contains(estimate.Reasons, ReasonPricingGroupInvalid) {
		return ReasonPricingGroupInvalid
	}
	if len(estimate.Reasons) != 0 {
		return estimate.Reasons[0]
	}
	return ReasonPricingUnavailable
}
