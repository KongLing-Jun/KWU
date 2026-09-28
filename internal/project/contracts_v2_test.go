package project

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectsV2CanonicalConsumerIsExactAndNonDeferred(t *testing.T) {
	const version = "projects-v2-20260822.1"
	const cloudSHA256 = "1f246eaea4c3b715d79cfea035669beaa64f9f56dd413cc20d0d097f3cab592c"
	gotHash := CanonicalV2ContractSHA256()
	if gotHash != cloudSHA256 {
		t.Fatalf("projects-v2 consumer hash = %s, want %s", gotHash, cloudSHA256)
	}
	var fixture struct {
		Metadata struct {
			ContractVersion string `json:"contract_version"`
		} `json:"metadata"`
		Capability struct {
			Slices map[string]any `json:"slices"`
		} `json:"capability"`
		Readiness struct {
			Default    string `json:"default"`
			OnMismatch string `json:"on_mismatch"`
		} `json:"readiness"`
	}
	if err := json.Unmarshal(projectsV2ContractBytes, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Metadata.ContractVersion != version {
		t.Fatalf("version = %q", fixture.Metadata.ContractVersion)
	}
	if fixture.Readiness.Default != "disabled" || !strings.Contains(fixture.Readiness.OnMismatch, "v1_stays_available") {
		t.Fatalf("consumer is not fail closed: %#v", fixture.Readiness)
	}
	for _, name := range []string{"members", "activity", "bindings", "work_item_fields", "assets"} {
		if _, ok := fixture.Capability.Slices[name]; !ok {
			t.Errorf("missing slice %q", name)
		}
	}
	raw := strings.ToLower(string(projectsV2ContractBytes))
	if !strings.Contains(raw, `"optional": ["description", "human_description", "agent_instruction"]`) {
		t.Fatal("aggregate create description is missing from the consumer contract")
	}
	if !strings.Contains(raw, `"patch /api/v1/projects/{project_id}/bindings"`) {
		t.Fatal("incremental binding patch is missing from the consumer contract")
	}
	if !strings.Contains(raw, `"put /api/v1/projects/{project_id}/message-read"`) || !strings.Contains(raw, `"item_read_fields": ["unread", "other_members_unread", "unread_other_member_count", "other_member_read_states"]`) {
		t.Fatal("isolated message read protocol is missing from the consumer contract")
	}
	if !strings.Contains(raw, `"get /api/v1/projects/{project_id}/asset-operation-records"`) || !strings.Contains(raw, `"asset_operation_records"`) {
		t.Fatal("asset operation record protocol is missing from the consumer contract")
	}
	for _, forbidden := range []string{"asset-uploads", "asset_folders", "asset_bytes", "members_per_project"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("deferred token %q leaked into consumer", forbidden)
		}
	}
}

func TestProjectsV2CapabilityUsesEnablementAndKnownSlicesOnly(t *testing.T) {
	valid := `{"enabled":true,"contract_version":"` + V2ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + v2ContractHash + `","slices":{"members":true,"activity":true,"bindings":true,"work_item_fields":true,"assets":true}}`
	tests := []struct {
		name string
		raw  json.RawMessage
		want CapabilityStatus
		err  string
	}{
		{name: "absent", raw: nil, want: CapabilityUnavailable},
		{name: "disabled", raw: json.RawMessage(`{"enabled":false}`), want: CapabilityUnavailable},
		{name: "disabled ignores slices", raw: json.RawMessage(`{"enabled":false,"slices":"invalid"}`), want: CapabilityUnavailable},
		{name: "supported", raw: json.RawMessage(valid), want: CapabilityAvailable},
		{name: "different version", raw: json.RawMessage(strings.Replace(valid, V2ContractVersion, "projects-v3", 1)), want: CapabilityAvailable},
		{name: "different hash", raw: json.RawMessage(strings.Replace(valid, v2ContractHash, strings.Repeat("0", 64), 1)), want: CapabilityAvailable},
		{name: "unknown field and slice", raw: json.RawMessage(strings.Replace(strings.Replace(valid, `"enabled":true`, `"enabled":true,"future_field":42`, 1), `"assets":true}`, `"assets":true,"future_slice":true}`, 1)), want: CapabilityAvailable},
		{name: "wrong enabled type", raw: json.RawMessage(`{"enabled":"true"}`), want: CapabilityUnavailable, err: "boolean"},
		{name: "malformed json", raw: json.RawMessage(`{"enabled":true`), want: CapabilityUnavailable, err: "EOF"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, _, err := EvaluateV2Capability(test.raw)
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

	status, features, err := EvaluateV2Capability(json.RawMessage(`{"enabled":true,"slices":{"members":true,"future_slice":true}}`))
	if err != nil || status != CapabilityAvailable || !features.Members || features.Activity || features.Bindings || features.WorkItemFields || features.Assets {
		t.Fatalf("partial v2 slices = %q %#v, %v", status, features, err)
	}
}
