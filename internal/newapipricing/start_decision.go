package newapipricing

import "kwu/internal/modelbilling"

type StartAction string

const (
	StartAllow         StartAction = "allow"
	StartAllowDegraded StartAction = "allow_degraded"
	StartBlock         StartAction = "block"
)

type StartReason string

const (
	StartEvidenceMissing         StartReason = "evidence_missing"
	StartEvidenceMismatch        StartReason = "evidence_mismatch"
	StartEstimateUnavailable     StartReason = "estimate_unavailable"
	StartEstimateIncomplete      StartReason = "estimate_incomplete"
	StartCreditsUnavailable      StartReason = "credits_unavailable"
	StartPricingBlocked          StartReason = "pricing_blocked"
	StartBillingSourceInvalid    StartReason = "billing_source_invalid"
	StartPricingStale            StartReason = "pricing_stale"
	StartMockCreditConfig        StartReason = "mock_credit_config"
	StartPricingUnavailable      StartReason = "pricing_unavailable"
	StartPricingGroupUnavailable StartReason = "pricing_group_unavailable"
)

type StartDecisionInput struct {
	BillingSource  modelbilling.BillingSource
	TaskID         string
	Scope          modelbilling.TaskScope
	Estimate       EstimateResult
	Evidence       *modelbilling.PricingEvidence
	RequireCredits bool
}

type StartDecision struct {
	Action          StartAction
	BillingSource   modelbilling.BillingSource
	Reasons         []StartReason
	EstimateReasons []EstimateReason
}

func DecideModelStart(input StartDecisionInput) StartDecision {
	decision := StartDecision{
		BillingSource:   input.BillingSource,
		EstimateReasons: append([]EstimateReason(nil), input.Estimate.Reasons...),
	}
	switch input.BillingSource {
	case modelbilling.BillingNewAPIByok:
		if reason := byokBlockReason(input); reason != "" {
			decision.Action = StartBlock
			decision.Reasons = append(decision.Reasons, reason)
			return decision
		}
		if input.Estimate.State == EstimateExact {
			decision.Action = StartAllow
			return decision
		}
		decision.Action = StartAllowDegraded
		decision.Reasons = append(decision.Reasons, StartEstimateUnavailable)
		return decision
	case modelbilling.BillingSubscriptionPool:
		return decideSubscriptionStart(decision, input)
	default:
		decision.Action = StartBlock
		decision.Reasons = append(decision.Reasons, StartBillingSourceInvalid)
		return decision
	}
}

func byokBlockReason(input StartDecisionInput) StartReason {
	if input.Evidence == nil {
		return StartEvidenceMissing
	}
	if !evidenceMatchesEstimate(*input.Evidence, input.TaskID, input.Scope, input.Estimate, input.BillingSource) {
		return StartEvidenceMismatch
	}
	return ""
}

func decideSubscriptionStart(decision StartDecision, input StartDecisionInput) StartDecision {
	if reason := subscriptionBlockReason(input); reason != "" {
		decision.Action = StartBlock
		decision.Reasons = append(decision.Reasons, reason)
		return decision
	}
	if reliableSubscriptionStart(input) {
		decision.Action = StartAllow
		return decision
	}
	decision.Action = StartAllowDegraded
	return decision
}

func subscriptionBlockReason(input StartDecisionInput) StartReason {
	if input.Evidence == nil {
		return StartEvidenceMissing
	}
	if !evidenceMatchesEstimate(*input.Evidence, input.TaskID, input.Scope, input.Estimate, input.BillingSource) {
		return StartEvidenceMismatch
	}
	if input.Estimate.PricingState == modelbilling.PricingStaleBlocked || input.Estimate.PricingState == modelbilling.PricingUnavailable {
		return StartPricingBlocked
	}
	if input.Estimate.State == EstimateUnavailable {
		return StartEstimateUnavailable
	}
	if !input.Estimate.Complete {
		return StartEstimateIncomplete
	}
	if input.RequireCredits && input.Estimate.Credits.Value == nil {
		return StartCreditsUnavailable
	}
	return ""
}

func reliableSubscriptionStart(input StartDecisionInput) bool {
	return input.Estimate.State == EstimateExact && input.Evidence.State == modelbilling.PricingFresh &&
		!input.Estimate.CNY.Mock && !input.Estimate.Credits.Mock
}

func evidenceMatchesEstimate(evidence modelbilling.PricingEvidence, taskID string, scope modelbilling.TaskScope, estimate EstimateResult, source modelbilling.BillingSource) bool {
	if evidence.Validate(source) != nil {
		return false
	}
	return evidence.TaskID == taskID && evidence.Scope.Equal(scope) &&
		evidence.SnapshotID == estimate.SnapshotID && evidence.ModelID == estimate.ModelID &&
		evidence.PricingGroup == estimate.PricingGroup && evidence.State == estimate.PricingState
}
