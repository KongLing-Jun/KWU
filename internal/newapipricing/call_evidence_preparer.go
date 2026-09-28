package newapipricing

import (
	"context"
	"strings"
	"time"

	"kwu/internal/modelbilling"
)

const callEvidenceGroupUnavailable = "__kwu_unavailable__"

// CallEvidencePreparer adapts the cache-only B1 PrepareStart boundary to
// modelbilling's derived-call evidence contract. Task identity and the frozen
// pricing group always come from CallEvidencePreparationInput.
type CallEvidencePreparer struct {
	Preparation              *PricePreparationReader
	Source                   string
	CreditConfig             modelbilling.CreditConfigProvider
	CreditConfigAuthenticity SourceAuthenticity
	Now                      func() time.Time
}

var _ modelbilling.CallEvidencePreparer = CallEvidencePreparer{}

func (p CallEvidencePreparer) PrepareForCall(ctx context.Context, input modelbilling.CallEvidencePreparationInput) (modelbilling.PricingEvidence, error) {
	if err := ctx.Err(); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	if p.Preparation == nil {
		return modelbilling.PricingEvidence{}, contractError("call evidence price preparation is required")
	}
	if err := validateCallEvidencePreparationInput(input); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	start := StartPreparationInput{
		Source: p.Source, BillingSource: input.Task.BillingSource,
		EvidenceID: input.EvidenceID, TaskID: input.Task.TaskID, Scope: input.Task.Scope,
		ModelID: input.ModelID, CapturedAt: p.now(),
	}
	if err := p.applyFrozenPricingFacts(ctx, input, &start); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	result := p.Preparation.PrepareStart(start)
	if result.Decision.Action == StartBlock || result.Evidence == nil {
		return modelbilling.PricingEvidence{}, callEvidencePreparationError(result)
	}
	if result.Decision.Action != StartAllow && result.Decision.Action != StartAllowDegraded {
		return modelbilling.PricingEvidence{}, contractError("call evidence preparation returned an invalid decision")
	}
	if err := validatePreparedCallEvidence(input, *result.Evidence); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	return *result.Evidence, nil
}

func (p CallEvidencePreparer) applyFrozenPricingFacts(ctx context.Context, input modelbilling.CallEvidencePreparationInput, start *StartPreparationInput) error {
	switch input.Task.BillingSource {
	case modelbilling.BillingSubscriptionPool:
		if p.CreditConfig == nil || input.Task.Scope.CloudBinding == nil {
			return contractError("subscription call evidence requires credit config and cloud binding")
		}
		config, err := p.CreditConfig.GetCreditConfig(ctx, *input.Task.Scope.CloudBinding)
		if err != nil {
			return err
		}
		if err := config.Validate(); err != nil {
			return err
		}
		if config.PricingGroup != input.InitialEvidence.PricingGroup {
			return contractError("subscription credit config changed the task pricing group")
		}
		if !sameCallEvidenceVersion(config.Version, input.InitialEvidence.PricingVersion) {
			return contractError("subscription credit config changed the task pricing version")
		}
		start.CreditConfig = &CreditConfigInput{Config: config, Authenticity: p.CreditConfigAuthenticity}
	case modelbilling.BillingNewAPIByok:
		if input.InitialEvidence.PricingGroup != callEvidenceGroupUnavailable {
			start.BYOKPricingGroup = input.InitialEvidence.PricingGroup
		}
	default:
		return contractError("call evidence task has an invalid billing source")
	}
	return nil
}

func validateCallEvidencePreparationInput(input modelbilling.CallEvidencePreparationInput) error {
	if _, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: input.Task.TaskID, Scope: input.Task.Scope, BillingSource: input.Task.BillingSource,
		InitialModelID: input.Task.InitialModelID, PricingEvidenceID: input.Task.PricingEvidenceID,
	}); err != nil {
		return err
	}
	if err := validateCallEvidenceIdentity(input); err != nil {
		return err
	}
	return validateInitialCallEvidence(input)
}

func validateCallEvidenceIdentity(input modelbilling.CallEvidencePreparationInput) error {
	if strings.TrimSpace(input.EvidenceID) == "" || strings.TrimSpace(input.EvidenceID) != input.EvidenceID ||
		strings.TrimSpace(input.ModelID) == "" || strings.TrimSpace(input.ModelID) != input.ModelID {
		return contractError("call evidence identity is invalid")
	}
	return nil
}

func validateInitialCallEvidence(input modelbilling.CallEvidencePreparationInput) error {
	initial := input.InitialEvidence
	if err := initial.Validate(input.Task.BillingSource); err != nil {
		return err
	}
	if initial.TaskID != input.Task.TaskID || !initial.Scope.Equal(input.Task.Scope) {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "initial evidence does not belong to call task scope"}
	}
	if initial.EvidenceID != input.Task.PricingEvidenceID || initial.ModelID != input.Task.InitialModelID {
		return contractError("initial evidence does not match call task identity")
	}
	return nil
}

func validatePreparedCallEvidence(input modelbilling.CallEvidencePreparationInput, evidence modelbilling.PricingEvidence) error {
	if err := evidence.Validate(input.Task.BillingSource); err != nil {
		return err
	}
	if evidence.TaskID != input.Task.TaskID || !evidence.Scope.Equal(input.Task.Scope) {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "prepared evidence does not belong to call task scope"}
	}
	if evidence.EvidenceID != input.EvidenceID || evidence.ModelID != input.ModelID ||
		evidence.PricingGroup != input.InitialEvidence.PricingGroup {
		return contractError("prepared evidence does not match call identity and frozen pricing group")
	}
	return nil
}

func callEvidencePreparationError(result StartPreparationResult) error {
	code := modelbilling.ErrorContractInvalid
	message := "call evidence preparation did not produce usable evidence"
	if result.Failure != nil {
		if result.Failure.Code != "" {
			code = result.Failure.Code
		}
		if result.Failure.Reason != "" {
			message += ": " + string(result.Failure.Reason)
		}
	}
	return &modelbilling.Error{Code: code, Message: message}
}

func sameCallEvidenceVersion(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (p CallEvidencePreparer) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}
