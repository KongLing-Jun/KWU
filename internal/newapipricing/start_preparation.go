package newapipricing

import (
	"strings"
	"time"

	"kwu/internal/modelbilling"
)

type StartPreparationInput struct {
	Source           string
	BillingSource    modelbilling.BillingSource
	EvidenceID       string
	TaskID           string
	Scope            modelbilling.TaskScope
	ModelID          string
	CreditConfig     *CreditConfigInput
	BYOKPricingGroup string
	CapturedAt       time.Time
}
type StartPreparationResult struct {
	CachedSnapshot *CachedSnapshot
	Evidence       *modelbilling.PricingEvidence
	Decision       StartDecision
	Failure        *PreparationFailure
}

// PrepareStart is a cache-only start-fact gate. It deliberately never estimates usage.
func (r *PricePreparationReader) PrepareStart(input StartPreparationInput) StartPreparationResult {
	if err := validateStartInput(input); err != nil {
		decisionReason, failureReason := startInputFailure(input)
		return startBlocked(input, modelbilling.ErrorCodeOf(err), decisionReason, failureReason)
	}
	if result, ok := prepareGroupUnavailable(input); ok {
		return result
	}
	return r.prepareKnownGroup(input, startPricingGroup(input))
}

func prepareGroupUnavailable(input StartPreparationInput) (StartPreparationResult, bool) {
	if input.BillingSource != modelbilling.BillingNewAPIByok || strings.TrimSpace(input.BYOKPricingGroup) != "" {
		return StartPreparationResult{}, false
	}
	e, err := BuildGroupUnavailablePricingEvidence(input.Source, startEvidenceRequest(input, "__kwu_unavailable__"))
	if err != nil {
		return startBlocked(input, modelbilling.ErrorCodeOf(err), StartEvidenceMismatch, ReasonEvidenceMismatch), true
	}
	return StartPreparationResult{Evidence: &e, Decision: StartDecision{Action: StartAllowDegraded, BillingSource: input.BillingSource, Reasons: []StartReason{StartPricingGroupUnavailable}}}, true
}
func startPricingGroup(input StartPreparationInput) string {
	if input.BillingSource == modelbilling.BillingSubscriptionPool {
		return input.CreditConfig.Config.PricingGroup
	}
	return input.BYOKPricingGroup
}
func (r *PricePreparationReader) prepareKnownGroup(input StartPreparationInput, group string) StartPreparationResult {
	if r == nil || r.reader == nil {
		return startBlocked(input, modelbilling.ErrorContractInvalid, StartEvidenceMissing, ReasonPricingContractAmbiguous)
	}
	cached, err := r.reader.Read(input.Source)
	if err != nil {
		return r.startReadError(input, group, err)
	}
	return startCachedResult(input, group, cached)
}
func (r *PricePreparationReader) startReadError(input StartPreparationInput, group string, err error) StartPreparationResult {
	if input.BillingSource == modelbilling.BillingNewAPIByok && modelbilling.ErrorCodeOf(err) == modelbilling.ErrorPricingUnavailable {
		e, eerr := BuildUnavailablePricingEvidence(input.Source, err, startEvidenceRequest(input, group))
		if eerr != nil {
			return startBlocked(input, modelbilling.ErrorCodeOf(eerr), StartEvidenceMismatch, ReasonEvidenceMismatch)
		}
		return StartPreparationResult{Evidence: &e, Decision: StartDecision{Action: StartAllowDegraded, BillingSource: input.BillingSource, Reasons: []StartReason{StartPricingUnavailable}}, Failure: &PreparationFailure{Code: modelbilling.ErrorPricingUnavailable, Reason: ReasonPricingUnavailable}}
	}
	return startBlocked(input, modelbilling.ErrorCodeOf(err), StartPricingUnavailable, ReasonSnapshotUnavailable)
}
func startCachedResult(input StartPreparationInput, group string, cached CachedSnapshot) StartPreparationResult {
	model, ok := findModel(cached.Snapshot.Models, input.ModelID)
	if !ok {
		return startBlocked(input, modelbilling.ErrorContractInvalid, StartEvidenceMismatch, ReasonModelNotFound)
	}
	if _, ok := validGroup(cached.Snapshot, model, group); !ok {
		return startBlocked(input, modelbilling.ErrorContractInvalid, StartEvidenceMismatch, ReasonPricingGroupInvalid)
	}
	e, err := BuildPricingEvidence(cached, startEvidenceRequest(input, group))
	if err != nil {
		return startBlocked(input, modelbilling.ErrorCodeOf(err), StartEvidenceMismatch, ReasonEvidenceMismatch)
	}
	if cached.Snapshot.State == modelbilling.PricingStaleBlocked {
		return StartPreparationResult{CachedSnapshot: &cached, Evidence: &e, Decision: StartDecision{Action: StartBlock, BillingSource: input.BillingSource, Reasons: []StartReason{StartPricingBlocked}}, Failure: &PreparationFailure{Code: modelbilling.ErrorPricingUnavailable, Reason: ReasonStaleBlocked}}
	}
	if cached.Snapshot.State == modelbilling.PricingUnavailable {
		return StartPreparationResult{CachedSnapshot: &cached, Evidence: &e, Decision: StartDecision{Action: StartBlock, BillingSource: input.BillingSource, Reasons: []StartReason{StartPricingBlocked}}, Failure: &PreparationFailure{Code: modelbilling.ErrorPricingUnavailable, Reason: ReasonPricingUnavailable}}
	}
	decision := StartDecision{Action: StartAllow, BillingSource: input.BillingSource}
	if cached.Snapshot.State == modelbilling.PricingStaleUsable {
		decision.Action = StartAllowDegraded
		decision.Reasons = append(decision.Reasons, StartPricingStale)
	}
	if input.BillingSource == modelbilling.BillingSubscriptionPool && input.CreditConfig.Authenticity == SourceMockOnly {
		decision.Action = StartAllowDegraded
		decision.Reasons = append(decision.Reasons, StartMockCreditConfig)
	}
	return StartPreparationResult{CachedSnapshot: &cached, Evidence: &e, Decision: decision}
}
func validateStartInput(i StartPreparationInput) error {
	if err := validateStartIdentity(i); err != nil {
		return err
	}
	switch i.BillingSource {
	case modelbilling.BillingSubscriptionPool:
		if i.BYOKPricingGroup != "" {
			return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "subscription input cannot include a BYOK pricing group"}
		}
		return validateStartCreditConfig(i.CreditConfig)
	case modelbilling.BillingNewAPIByok:
		if i.CreditConfig != nil {
			return &modelbilling.Error{Code: modelbilling.ErrorBillingSourceMismatch, Message: "BYOK input cannot include a credit config"}
		}
		if i.BYOKPricingGroup != "" && strings.TrimSpace(i.BYOKPricingGroup) != i.BYOKPricingGroup {
			return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "BYOK pricing group must be trimmed"}
		}
		return nil
	}
	return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "invalid billing source"}
}
func validateStartIdentity(i StartPreparationInput) error {
	if err := i.BillingSource.Validate(); err != nil {
		return err
	}
	if err := i.Scope.Validate(i.BillingSource); err != nil {
		return err
	}
	if !sourceIDPattern.MatchString(i.Source) {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "invalid pricing source"}
	}
	for _, v := range []string{i.EvidenceID, i.TaskID, i.ModelID} {
		if strings.TrimSpace(v) == "" || strings.TrimSpace(v) != v {
			return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "invalid start preparation identity"}
		}
	}
	if i.CapturedAt.IsZero() {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "captured time is required"}
	}
	return nil
}
func startInputFailure(i StartPreparationInput) (StartReason, EstimateReason) {
	if i.BillingSource.Validate() != nil {
		return StartBillingSourceInvalid, ReasonBillingSourceInvalid
	}
	if strings.TrimSpace(i.ModelID) == "" || strings.TrimSpace(i.ModelID) != i.ModelID {
		return StartEvidenceMismatch, ReasonModelNotFound
	}
	if i.BillingSource == modelbilling.BillingSubscriptionPool && i.BYOKPricingGroup != "" {
		return StartEvidenceMismatch, ReasonPricingGroupInvalid
	}
	if i.BillingSource == modelbilling.BillingNewAPIByok && i.BYOKPricingGroup != "" && strings.TrimSpace(i.BYOKPricingGroup) != i.BYOKPricingGroup {
		return StartEvidenceMismatch, ReasonPricingGroupInvalid
	}
	return StartEvidenceMismatch, ReasonPricingContractAmbiguous
}
func validateStartCreditConfig(value *CreditConfigInput) error {
	if value == nil || value.Config.Validate() != nil || (value.Authenticity != SourceReady && value.Authenticity != SourceMockOnly) {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "subscription credit config is required"}
	}
	return nil
}
func startEvidenceRequest(i StartPreparationInput, group string) PricingEvidenceRequest {
	return PricingEvidenceRequest{EvidenceID: i.EvidenceID, TaskID: i.TaskID, Scope: i.Scope, BillingSource: i.BillingSource, ModelID: i.ModelID, PricingGroup: group, CapturedAt: i.CapturedAt}
}
func startBlocked(i StartPreparationInput, code modelbilling.ErrorCode, reason StartReason, failureReason EstimateReason) StartPreparationResult {
	if code == "" {
		code = modelbilling.ErrorContractInvalid
	}
	return StartPreparationResult{Decision: StartDecision{Action: StartBlock, BillingSource: i.BillingSource, Reasons: []StartReason{reason}}, Failure: &PreparationFailure{Code: code, Reason: failureReason}}
}
