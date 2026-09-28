package newapipricing

import (
	"strings"
	"time"

	"kwu/internal/modelbilling"
)

const unavailableSnapshotIDPrefix = "pricing-unavailable-v1:"
const groupUnavailableSnapshotIDPrefix = "pricing-unavailable-v2:"

type PricingEvidenceRequest struct {
	EvidenceID    string
	TaskID        string
	Scope         modelbilling.TaskScope
	BillingSource modelbilling.BillingSource
	ModelID       string
	PricingGroup  string
	CapturedAt    time.Time
}

func BuildGroupUnavailablePricingEvidence(source string, request PricingEvidenceRequest) (modelbilling.PricingEvidence, error) {
	if request.BillingSource != modelbilling.BillingNewAPIByok || !sourceIDPattern.MatchString(source) {
		return modelbilling.PricingEvidence{}, contractError("group-unavailable evidence requires a valid BYOK source")
	}
	evidence := modelbilling.PricingEvidence{EvidenceID: request.EvidenceID, SnapshotID: groupUnavailableSnapshotIDPrefix + source + ":group_unavailable", Scope: cloneTaskScope(request.Scope), TaskID: request.TaskID, ModelID: request.ModelID, PricingGroup: "__kwu_unavailable__", State: modelbilling.PricingUnavailable, CapturedAt: request.CapturedAt}
	if err := evidence.Validate(request.BillingSource); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	return evidence, nil
}

func BuildPricingEvidence(cached CachedSnapshot, request PricingEvidenceRequest) (modelbilling.PricingEvidence, error) {
	if err := cached.Snapshot.Validate(); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	model, ok := findModel(cached.Snapshot.Models, request.ModelID)
	if !ok {
		return modelbilling.PricingEvidence{}, contractError("pricing evidence model %q is not in snapshot", request.ModelID)
	}
	if _, ok := validGroup(cached.Snapshot, model, request.PricingGroup); !ok {
		return modelbilling.PricingEvidence{}, contractError("pricing evidence group %q is invalid for model %q", request.PricingGroup, request.ModelID)
	}
	hash := strings.TrimSpace(cached.Snapshot.SourceHash)
	evidence := modelbilling.PricingEvidence{
		EvidenceID: request.EvidenceID, SnapshotID: cached.Snapshot.SnapshotID,
		Scope: cloneTaskScope(request.Scope), TaskID: request.TaskID, ModelID: request.ModelID,
		PricingGroup: request.PricingGroup, PricingVersion: cloneString(cached.Snapshot.SourceVersion),
		PricingHash: &hash, State: cached.Snapshot.State, CapturedAt: request.CapturedAt,
	}
	if err := evidence.Validate(request.BillingSource); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	return evidence, nil
}

func BuildUnavailablePricingEvidence(source string, readErr error, request PricingEvidenceRequest) (modelbilling.PricingEvidence, error) {
	if request.BillingSource != modelbilling.BillingNewAPIByok {
		return modelbilling.PricingEvidence{}, &modelbilling.Error{
			Code: modelbilling.ErrorBillingSourceMismatch, Message: "unavailable pricing evidence is BYOK-only",
		}
	}
	if modelbilling.ErrorCodeOf(readErr) != modelbilling.ErrorPricingUnavailable {
		return modelbilling.PricingEvidence{}, contractError("unavailable pricing evidence requires a pricing_unavailable cache read")
	}
	if !sourceIDPattern.MatchString(source) {
		return modelbilling.PricingEvidence{}, contractError("pricing source must be a public stable identifier")
	}
	evidence := modelbilling.PricingEvidence{
		EvidenceID: request.EvidenceID, SnapshotID: unavailableSnapshotIDPrefix + source,
		Scope: cloneTaskScope(request.Scope), TaskID: request.TaskID, ModelID: request.ModelID,
		PricingGroup: request.PricingGroup, State: modelbilling.PricingUnavailable,
		CapturedAt: request.CapturedAt,
	}
	if err := evidence.Validate(modelbilling.BillingNewAPIByok); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	return evidence, nil
}

func cloneTaskScope(scope modelbilling.TaskScope) modelbilling.TaskScope {
	cloned := scope
	if scope.CloudBinding != nil {
		binding := *scope.CloudBinding
		cloned.CloudBinding = &binding
	}
	return cloned
}
