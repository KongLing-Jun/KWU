package newapipricing

import (
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

func TestBuildPricingEvidenceBindsSnapshotTaskModelAndGroup(t *testing.T) {
	cached := estimateTestSnapshot()
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "user_01", BindingGeneration: 1}
	request := PricingEvidenceRequest{
		EvidenceID: "evidence_01", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default", CloudBinding: &binding},
		BillingSource: modelbilling.BillingSubscriptionPool,
		ModelID:       "model-a", PricingGroup: "vip", CapturedAt: cacheTestStart.Add(time.Minute),
	}

	evidence, err := BuildPricingEvidence(cached, request)
	if err != nil {
		t.Fatalf("BuildPricingEvidence: %v", err)
	}
	if evidence.EvidenceID != request.EvidenceID || evidence.TaskID != request.TaskID ||
		evidence.ModelID != request.ModelID || evidence.PricingGroup != request.PricingGroup ||
		evidence.SnapshotID != cached.Snapshot.SnapshotID || evidence.PricingHash == nil ||
		*evidence.PricingHash != cached.Snapshot.SourceHash || evidence.PricingVersion == nil ||
		*evidence.PricingVersion != *cached.Snapshot.SourceVersion || !evidence.Scope.Equal(request.Scope) {
		t.Fatalf("evidence = %+v", evidence)
	}
}

func TestBuildPricingEvidenceDeepCopiesTaskScope(t *testing.T) {
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "user_01", BindingGeneration: 1}
	request := PricingEvidenceRequest{
		EvidenceID: "evidence_01", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default", CloudBinding: &binding},
		BillingSource: modelbilling.BillingSubscriptionPool,
		ModelID:       "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	}

	evidence, err := BuildPricingEvidence(estimateTestSnapshot(), request)
	if err != nil {
		t.Fatalf("BuildPricingEvidence: %v", err)
	}
	request.Scope.CloudBinding.ProfileID = "request_changed"
	request.Scope.CloudBinding.CloudUserID = "request_user_changed"
	request.Scope.CloudBinding.BindingGeneration = 2
	if got := evidence.Scope.CloudBinding; got.ProfileID != "default" || got.CloudUserID != "user_01" || got.BindingGeneration != 1 {
		t.Fatalf("evidence binding changed with request = %+v", got)
	}

	evidence.Scope.CloudBinding.ProfileID = "evidence_changed"
	evidence.Scope.CloudBinding.CloudUserID = "evidence_user_changed"
	evidence.Scope.CloudBinding.BindingGeneration = 3
	if got := request.Scope.CloudBinding; got.ProfileID != "request_changed" || got.CloudUserID != "request_user_changed" || got.BindingGeneration != 2 {
		t.Fatalf("request binding changed with evidence = %+v", got)
	}
}

func TestBuildPricingEvidenceRejectsInvalidGroupAndModel(t *testing.T) {
	base := PricingEvidenceRequest{
		EvidenceID: "evidence_01", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"},
		BillingSource: modelbilling.BillingNewAPIByok,
		ModelID:       "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	}
	for name, mutate := range map[string]func(*PricingEvidenceRequest){
		"model": func(request *PricingEvidenceRequest) { request.ModelID = "missing" },
		"group": func(request *PricingEvidenceRequest) { request.PricingGroup = "missing" },
	} {
		t.Run(name, func(t *testing.T) {
			request := base
			mutate(&request)
			if _, err := BuildPricingEvidence(estimateTestSnapshot(), request); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestBuildPricingEvidenceDoesNotRewriteOlderSnapshotEvidence(t *testing.T) {
	request := PricingEvidenceRequest{
		EvidenceID: "evidence_old", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"},
		BillingSource: modelbilling.BillingNewAPIByok,
		ModelID:       "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	}
	oldSnapshot := estimateTestSnapshot()
	oldEvidence, err := BuildPricingEvidence(oldSnapshot, request)
	if err != nil {
		t.Fatal(err)
	}
	newSnapshot := estimateTestSnapshot()
	newSnapshot.Snapshot.SnapshotID = "pricing-v1:domestic:new"
	newSnapshot.Snapshot.SourceHash = "sha256:new"
	request.EvidenceID = "evidence_new"
	request.CapturedAt = request.CapturedAt.Add(time.Hour)
	newEvidence, err := BuildPricingEvidence(newSnapshot, request)
	if err != nil {
		t.Fatal(err)
	}
	if oldEvidence.SnapshotID != oldSnapshot.Snapshot.SnapshotID || *oldEvidence.PricingHash != oldSnapshot.Snapshot.SourceHash {
		t.Fatalf("old evidence changed = %+v", oldEvidence)
	}
	if newEvidence.SnapshotID == oldEvidence.SnapshotID || *newEvidence.PricingHash == *oldEvidence.PricingHash {
		t.Fatalf("new evidence did not bind new snapshot = %+v", newEvidence)
	}
}

func TestBuildUnavailablePricingEvidenceUsesStableSentinelIdentity(t *testing.T) {
	readErr := &modelbilling.Error{Code: modelbilling.ErrorPricingUnavailable}
	request := PricingEvidenceRequest{
		EvidenceID: "evidence_unavailable", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"},
		BillingSource: modelbilling.BillingNewAPIByok,
		ModelID:       "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	}

	first, err := BuildUnavailablePricingEvidence("domestic", readErr, request)
	if err != nil {
		t.Fatalf("BuildUnavailablePricingEvidence: %v", err)
	}
	second, err := BuildUnavailablePricingEvidence("domestic", readErr, request)
	if err != nil {
		t.Fatalf("second BuildUnavailablePricingEvidence: %v", err)
	}
	if first.EvidenceID != request.EvidenceID || first.SnapshotID != "pricing-unavailable-v1:domestic" ||
		first.TaskID != request.TaskID || first.ModelID != request.ModelID || first.PricingGroup != request.PricingGroup ||
		first.PricingVersion != nil || first.PricingHash != nil || first.State != modelbilling.PricingUnavailable ||
		!first.CapturedAt.Equal(request.CapturedAt) || !first.Scope.Equal(request.Scope) {
		t.Fatalf("unavailable evidence = %+v", first)
	}
	if first.SnapshotID != second.SnapshotID || first.SnapshotID == BuildSnapshotID("domestic", "sha256:test") {
		t.Fatalf("sentinel identity first=%q second=%q", first.SnapshotID, second.SnapshotID)
	}
	if err := first.Validate(modelbilling.BillingNewAPIByok); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	request.Scope.ActorID = "changed"
	if first.Scope.ActorID != "actor" {
		t.Fatalf("evidence scope changed with request = %+v", first.Scope)
	}
}

func TestBuildUnavailablePricingEvidenceRejectsWrongSourceOrCause(t *testing.T) {
	request := PricingEvidenceRequest{
		EvidenceID: "evidence_unavailable", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"},
		BillingSource: modelbilling.BillingNewAPIByok,
		ModelID:       "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	}
	tests := map[string]struct {
		source  string
		readErr error
	}{
		"invalid source": {source: "../domestic", readErr: &modelbilling.Error{Code: modelbilling.ErrorPricingUnavailable}},
		"contract error": {source: "domestic", readErr: &modelbilling.Error{Code: modelbilling.ErrorContractInvalid}},
		"scope mismatch": {source: "domestic", readErr: &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch}},
		"nil error":      {source: "domestic"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildUnavailablePricingEvidence(test.source, test.readErr, request); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestBuildUnavailablePricingEvidenceRejectsSubscriptionAndInvalidRequest(t *testing.T) {
	readErr := &modelbilling.Error{Code: modelbilling.ErrorPricingUnavailable}
	base := PricingEvidenceRequest{
		EvidenceID: "evidence_unavailable", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"},
		BillingSource: modelbilling.BillingNewAPIByok,
		ModelID:       "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	}
	tests := map[string]struct {
		mutate func(*PricingEvidenceRequest)
		code   modelbilling.ErrorCode
	}{
		"subscription": {
			mutate: func(request *PricingEvidenceRequest) { request.BillingSource = modelbilling.BillingSubscriptionPool },
			code:   modelbilling.ErrorBillingSourceMismatch,
		},
		"evidence id": {mutate: func(request *PricingEvidenceRequest) { request.EvidenceID = "" }, code: modelbilling.ErrorContractInvalid},
		"task id":     {mutate: func(request *PricingEvidenceRequest) { request.TaskID = " task_01" }, code: modelbilling.ErrorContractInvalid},
		"model id":    {mutate: func(request *PricingEvidenceRequest) { request.ModelID = "" }, code: modelbilling.ErrorContractInvalid},
		"group":       {mutate: func(request *PricingEvidenceRequest) { request.PricingGroup = "" }, code: modelbilling.ErrorContractInvalid},
		"scope": {
			mutate: func(request *PricingEvidenceRequest) {
				request.Scope.CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "user_01", BindingGeneration: 1}
			},
			code: modelbilling.ErrorBillingSourceMismatch,
		},
		"captured at": {mutate: func(request *PricingEvidenceRequest) { request.CapturedAt = time.Time{} }, code: modelbilling.ErrorContractInvalid},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			request := base
			test.mutate(&request)
			if _, err := BuildUnavailablePricingEvidence("domestic", readErr, request); modelbilling.ErrorCodeOf(err) != test.code {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
