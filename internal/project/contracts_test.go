package project

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const frozenProjectsV1SHA256 = "e9c7d4bb5dca21d6c02ae9258d960537621214ae583efbe3d14394af01aee1bf"

func TestProjectsV1CanonicalConsumer(t *testing.T) {
	contract, err := LoadCanonicalContract()
	if err != nil {
		t.Fatalf("load canonical projects-v1 contract: %v", err)
	}

	t.Run("exact bytes and capability identity", func(t *testing.T) {
		if got := len(CanonicalContractBytes()); got != 21099 {
			t.Fatalf("canonical fixture bytes = %d, want 21099", got)
		}
		if got := CanonicalContractSHA256(); got != frozenProjectsV1SHA256 {
			t.Fatalf("canonical fixture sha256 = %q, want %q", got, frozenProjectsV1SHA256)
		}
		if contract.Version != "projects-v1-20260812.1" || contract.Capability.Version != contract.Version || contract.Capability.HashAlgorithm != "sha256" {
			t.Fatalf("contract identity = %#v", contract)
		}
	})

	t.Run("safe JSON integer boundaries", func(t *testing.T) {
		if contract.WireIntegerMinimum != -9007199254740991 || contract.WireIntegerMaximum != 9007199254740991 {
			t.Fatalf("wire integer range = %d..%d", contract.WireIntegerMinimum, contract.WireIntegerMaximum)
		}
		if contract.ProjectVersionMinimum != 1 || contract.ProjectVersionMaximum != 9007199254740991 {
			t.Fatalf("project version range = %d..%d", contract.ProjectVersionMinimum, contract.ProjectVersionMaximum)
		}
		if contract.WorkItemRankMinimum != -9007199254740991 || contract.WorkItemRankMaximum != 9007199254740991 {
			t.Fatalf("work item rank range = %d..%d", contract.WorkItemRankMinimum, contract.WorkItemRankMaximum)
		}
		if contract.MaximumExpectedVersion != 9007199254740990 {
			t.Fatalf("maximum expected version = %d", contract.MaximumExpectedVersion)
		}
	})

	t.Run("routes", func(t *testing.T) {
		for _, operation := range []string{
			"GET /api/v1/projects",
			"POST /api/v1/projects",
			"POST /api/v1/projects/onboarding/claim",
			"PATCH /api/v1/projects/{project_id}",
			"PUT /api/v1/projects/{project_id}/instructions",
			"POST /api/v1/work-items/{work_item_id}/move",
			"DELETE /api/v1/work-items/{work_item_id}",
		} {
			if !slices.Contains(contract.Routes, operation) {
				t.Errorf("canonical routes omitted %q: %v", operation, contract.Routes)
			}
		}
		if contract.ProjectDelete != "not_exposed" || contract.WorkItemDelete != "permanent_idempotent" {
			t.Fatalf("delete routes = project:%q work-item:%q", contract.ProjectDelete, contract.WorkItemDelete)
		}
	})

	t.Run("DTO null and omitted semantics", func(t *testing.T) {
		if got := contract.ProjectCreateName; got != (FieldRule{Presence: "required", Null: "forbidden"}) {
			t.Fatalf("project create name rule = %#v", got)
		}
		if got := contract.ProjectPatchDescription; got != (FieldRule{Presence: "optional", Omitted: "unchanged", Null: "clear"}) {
			t.Fatalf("project patch description rule = %#v", got)
		}
		if got := contract.InstructionHumanDescription; got != (FieldRule{Presence: "required", Null: "forbidden"}) {
			t.Fatalf("instruction human_description rule = %#v", got)
		}
		if got := contract.WorkItemMoveBefore; got != (FieldRule{Presence: "optional", Omitted: "no_anchor", Null: "no_anchor"}) {
			t.Fatalf("work item move before rule = %#v", got)
		}
		if contract.WorkItemMoveAnchorRule != "at_most_one_before_or_after" {
			t.Fatalf("work item move anchor rule = %q", contract.WorkItemMoveAnchorRule)
		}
	})

	t.Run("safe errors and command result", func(t *testing.T) {
		wantErrors := []string{"AUTH_REQUIRED", "CONFLICT", "IDEMPOTENCY_CONFLICT", "NOT_FOUND", "QUOTA_EXCEEDED", "VALIDATION_FAILED"}
		if !slices.Equal(contract.ErrorCodes, wantErrors) {
			t.Fatalf("error codes = %v, want %v", contract.ErrorCodes, wantErrors)
		}
		wantFields := []string{"operation_id", "resource_kind", "resource_id", "project_id", "committed_version"}
		if !slices.Equal(contract.CommandResultFields, wantFields) {
			t.Fatalf("command result fields = %v, want %v", contract.CommandResultFields, wantFields)
		}
	})

	t.Run("instruction content hash examples", func(t *testing.T) {
		if len(contract.InstructionHashExamples) < 2 {
			t.Fatalf("instruction hash examples = %d, want at least 2", len(contract.InstructionHashExamples))
		}
		for _, example := range contract.InstructionHashExamples {
			if got := InstructionContentSHA256(example.HumanDescription, example.AgentInstruction); got != example.SHA256 {
				t.Errorf("%s instruction hash = %q, want %q", example.Name, got, example.SHA256)
			}
		}
	})
}

func TestProjectsV1CapabilityCompatibility(t *testing.T) {
	valid := `{"enabled":true,"contract_version":"` + ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + frozenProjectsV1SHA256 + `","instructions":true,"work_items":true}`
	tests := []struct {
		name string
		raw  json.RawMessage
		want CapabilityStatus
		err  string
	}{
		{name: "absent", raw: nil, want: CapabilityUnavailable},
		{name: "disabled", raw: json.RawMessage(`{"enabled":false}`), want: CapabilityUnavailable},
		{name: "supported", raw: json.RawMessage(valid), want: CapabilityAvailable},
		{name: "different version", raw: json.RawMessage(strings.Replace(valid, ContractVersion, "projects-v2", 1)), want: CapabilityAvailable},
		{name: "different hash", raw: json.RawMessage(strings.Replace(valid, frozenProjectsV1SHA256, strings.Repeat("0", 64), 1)), want: CapabilityAvailable},
		{name: "unknown field", raw: json.RawMessage(strings.Replace(valid, `"enabled":true`, `"enabled":true,"future_flag":{"mode":"new"}`, 1)), want: CapabilityAvailable},
		{name: "duplicate enabled", raw: json.RawMessage(strings.Replace(valid, `"enabled":true`, `"enabled":true,"enabled":false`, 1)), want: CapabilityUnavailable, err: "duplicate"},
		{name: "null enabled", raw: json.RawMessage(strings.Replace(valid, `"enabled":true`, `"enabled":null`, 1)), want: CapabilityUnavailable, err: "boolean"},
		{name: "malformed json", raw: json.RawMessage(`{"enabled":true`), want: CapabilityUnavailable, err: "EOF"},
		{name: "null work items", raw: json.RawMessage(strings.Replace(valid, `"work_items":true`, `"work_items":null`, 1)), want: CapabilityUnavailable, err: "boolean"},
		{name: "wrong-type instructions", raw: json.RawMessage(strings.Replace(valid, `"instructions":true`, `"instructions":"true"`, 1)), want: CapabilityUnavailable, err: "boolean"},
		{name: "instructions slice disabled", raw: json.RawMessage(strings.Replace(valid, "\"instructions\":true", "\"instructions\":false", 1)), want: CapabilityAvailable},
		{name: "work items slice disabled", raw: json.RawMessage(strings.Replace(valid, "\"work_items\":true", "\"work_items\":false", 1)), want: CapabilityAvailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, err := EvaluateCapability(test.raw)
			if status != test.want {
				t.Fatalf("status = %q, want %q", status, test.want)
			}
			if test.err == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if test.err != "" && (err == nil || !strings.Contains(err.Error(), test.err)) {
				t.Fatalf("error = %v, want %q mismatch", err, test.err)
			}
		})
	}
	status, features, err := EvaluateCapabilityFeatures(json.RawMessage(strings.Replace(valid, "\"work_items\":true", "\"work_items\":false", 1)))
	if err != nil || status != CapabilityAvailable || !features.Instructions || features.WorkItems {
		t.Fatalf("partial slice capability = %q %#v, %v", status, features, err)
	}
	status, features, err = EvaluateCapabilityFeatures(json.RawMessage(`{"enabled":true}`))
	if err != nil || status != CapabilityAvailable || features.Instructions || features.WorkItems {
		t.Fatalf("missing optional slices = %q %#v, %v", status, features, err)
	}
}
