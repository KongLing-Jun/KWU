package project

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	ContractVersion       = "projects-v1-20260812.1"
	canonicalContractHash = "e9c7d4bb5dca21d6c02ae9258d960537621214ae583efbe3d14394af01aee1bf"
	MaxSafeJSONInteger    = uint64(9_007_199_254_740_991)
	MaxExpectedVersion    = MaxSafeJSONInteger - 1
	MinSafeJSONInteger    = int64(-9_007_199_254_740_991)
)

//go:embed testdata/projects-v1/contracts.json
var canonicalContractBytes []byte

type FieldRule struct {
	Presence string
	Omitted  string
	Null     string
}

type CapabilityContract struct {
	Version       string
	HashAlgorithm string
	Instructions  bool
	WorkItems     bool
}

type InstructionHashExample struct {
	Name             string
	HumanDescription string
	AgentInstruction string
	SHA256           string
}

type Contract struct {
	Version                       string
	Routes                        []string
	ProjectDelete                 string
	WorkItemDelete                string
	ProjectCreateName             FieldRule
	ProjectCreateHumanDescription FieldRule
	ProjectCreateAgentInstruction FieldRule
	ProjectPatchDescription       FieldRule
	InstructionHumanDescription   FieldRule
	WorkItemMoveBefore            FieldRule
	WorkItemMoveAnchorRule        string
	ErrorCodes                    []string
	CommandResultFields           []string
	InstructionHashExamples       []InstructionHashExample
	Capability                    CapabilityContract
	WireIntegerMinimum            int64
	WireIntegerMaximum            int64
	ProjectVersionMinimum         uint64
	ProjectVersionMaximum         uint64
	WorkItemRankMinimum           int64
	WorkItemRankMaximum           int64
	MaximumExpectedVersion        uint64
}

type contractFixture struct {
	Metadata struct {
		Version       string `json:"contract_version"`
		HashAlgorithm string `json:"contract_hash_algorithm"`
		FixtureBytes  struct {
			Encoding     string `json:"encoding"`
			BOM          bool   `json:"bom"`
			LineEndings  string `json:"line_endings"`
			FinalNewline bool   `json:"final_newline"`
			HashScope    string `json:"hash_scope"`
		} `json:"fixture_bytes"`
	} `json:"metadata"`
	Routes struct {
		Operations     []string `json:"operations"`
		ProjectDelete  string   `json:"project_delete"`
		WorkItemDelete string   `json:"work_item_delete"`
	} `json:"routes"`
	DTO struct {
		ProjectCreate struct {
			Name             fixtureFieldRule `json:"name"`
			HumanDescription fixtureFieldRule `json:"human_description"`
			AgentInstruction fixtureFieldRule `json:"agent_instruction"`
		} `json:"project_create"`
		ProjectPatch struct {
			ExpectedVersion fixtureIntegerRule `json:"expected_version"`
			Description     fixtureFieldRule   `json:"description"`
		} `json:"project_patch"`
		InstructionPut struct {
			ExpectedVersion  fixtureIntegerRule `json:"expected_version"`
			HumanDescription fixtureFieldRule   `json:"human_description"`
		} `json:"instruction_put"`
		ProjectRead struct {
			VersionRange string `json:"version_range"`
		} `json:"project_read"`
		InstructionRead struct {
			VersionRange string `json:"version_range"`
		} `json:"instruction_read"`
		WorkItemRead struct {
			RankRange    string `json:"rank_range"`
			VersionRange string `json:"version_range"`
		} `json:"work_item_read"`
		WorkItemPatch struct {
			ExpectedVersion fixtureIntegerRule `json:"expected_version"`
		} `json:"work_item_patch"`
		WorkItemMove struct {
			ExpectedVersion  fixtureIntegerRule `json:"expected_version"`
			BeforeWorkItemID fixtureFieldRule   `json:"before_work_item_id"`
			AnchorRule       string             `json:"anchor_rule"`
		} `json:"work_item_move"`
	} `json:"dto"`
	Limits struct {
		WireInteger struct {
			Minimum int64 `json:"minimum"`
			Maximum int64 `json:"maximum"`
		} `json:"wire_integer"`
	} `json:"limits"`
	Capability struct {
		Version       string `json:"contract_version"`
		HashAlgorithm string `json:"contract_hash_algorithm"`
		Slices        struct {
			Instructions bool `json:"instructions"`
			WorkItems    bool `json:"work_items"`
		} `json:"slices"`
	} `json:"capability"`
	InstructionContentHash struct {
		Algorithm   string `json:"algorithm"`
		Output      string `json:"output"`
		Encoding    string `json:"encoding"`
		LineEndings string `json:"line_endings"`
		Examples    []struct {
			Name             string `json:"name"`
			HumanDescription string `json:"human_description"`
			AgentInstruction string `json:"agent_instruction"`
			SHA256           string `json:"sha256"`
		} `json:"examples"`
	} `json:"instruction_content_hash"`
	Success struct {
		CommandResultFields []string `json:"command_result_fields"`
	} `json:"success"`
	Errors map[string]json.RawMessage `json:"errors"`
}

type fixtureFieldRule struct {
	Presence string `json:"presence"`
	Omitted  string `json:"omitted"`
	Null     string `json:"null"`
}

type fixtureIntegerRule struct {
	Minimum uint64 `json:"minimum"`
	Maximum uint64 `json:"maximum"`
}

func (r fixtureFieldRule) public() FieldRule {
	return FieldRule{Presence: r.Presence, Omitted: r.Omitted, Null: r.Null}
}

func CanonicalContractBytes() []byte {
	return bytes.Clone(canonicalContractBytes)
}

func CanonicalContractSHA256() string {
	sum := sha256.Sum256(canonicalContractBytes)
	return hex.EncodeToString(sum[:])
}

func LoadCanonicalContract() (Contract, error) {
	if got := CanonicalContractSHA256(); got != canonicalContractHash {
		return Contract{}, fmt.Errorf("projects-v1: canonical fixture hash %s does not match compiled hash %s", got, canonicalContractHash)
	}
	var fixture contractFixture
	decoder := json.NewDecoder(bytes.NewReader(canonicalContractBytes))
	if err := decoder.Decode(&fixture); err != nil {
		return Contract{}, fmt.Errorf("projects-v1: decode canonical fixture: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Contract{}, err
	}
	if err := validateFixtureIdentity(fixture); err != nil {
		return Contract{}, err
	}

	errorCodes := make([]string, 0, len(fixture.Errors))
	for code := range fixture.Errors {
		errorCodes = append(errorCodes, code)
	}
	sort.Strings(errorCodes)
	examples := make([]InstructionHashExample, len(fixture.InstructionContentHash.Examples))
	for i, example := range fixture.InstructionContentHash.Examples {
		examples[i] = InstructionHashExample{
			Name: example.Name, HumanDescription: example.HumanDescription,
			AgentInstruction: example.AgentInstruction, SHA256: example.SHA256,
		}
	}
	return Contract{
		Version: fixture.Metadata.Version, Routes: append([]string(nil), fixture.Routes.Operations...),
		ProjectDelete: fixture.Routes.ProjectDelete, WorkItemDelete: fixture.Routes.WorkItemDelete,
		ProjectCreateName:             fixture.DTO.ProjectCreate.Name.public(),
		ProjectCreateHumanDescription: fixture.DTO.ProjectCreate.HumanDescription.public(),
		ProjectCreateAgentInstruction: fixture.DTO.ProjectCreate.AgentInstruction.public(),
		ProjectPatchDescription:       fixture.DTO.ProjectPatch.Description.public(),
		InstructionHumanDescription:   fixture.DTO.InstructionPut.HumanDescription.public(),
		WorkItemMoveBefore:            fixture.DTO.WorkItemMove.BeforeWorkItemID.public(),
		WorkItemMoveAnchorRule:        fixture.DTO.WorkItemMove.AnchorRule,
		ErrorCodes:                    errorCodes, CommandResultFields: append([]string(nil), fixture.Success.CommandResultFields...),
		InstructionHashExamples: examples,
		Capability:              CapabilityContract{Version: fixture.Capability.Version, HashAlgorithm: fixture.Capability.HashAlgorithm, Instructions: fixture.Capability.Slices.Instructions, WorkItems: fixture.Capability.Slices.WorkItems},
		WireIntegerMinimum:      fixture.Limits.WireInteger.Minimum, WireIntegerMaximum: fixture.Limits.WireInteger.Maximum,
		ProjectVersionMinimum: 1, ProjectVersionMaximum: MaxSafeJSONInteger,
		WorkItemRankMinimum: MinSafeJSONInteger, WorkItemRankMaximum: int64(MaxSafeJSONInteger),
		MaximumExpectedVersion: fixture.DTO.ProjectPatch.ExpectedVersion.Maximum,
	}, nil
}

func validateFixtureIdentity(fixture contractFixture) error {
	for _, validate := range []func(contractFixture) error{
		validateFixtureMetadata,
		validateInstructionHashFixture,
		validateSafeIntegerFixture,
		validateExpectedVersionFixture,
	} {
		if err := validate(fixture); err != nil {
			return err
		}
	}
	return nil
}

func validateFixtureMetadata(fixture contractFixture) error {
	if fixture.Metadata.Version != ContractVersion || fixture.Capability.Version != ContractVersion {
		return fmt.Errorf("projects-v1: canonical fixture has an unsupported contract version")
	}
	if fixture.Metadata.HashAlgorithm != "sha256" || fixture.Capability.HashAlgorithm != "sha256" {
		return fmt.Errorf("projects-v1: canonical fixture requires sha256")
	}
	bytesMeta := fixture.Metadata.FixtureBytes
	if bytesMeta.Encoding != "utf-8" || bytesMeta.BOM || bytesMeta.LineEndings != "lf" || !bytesMeta.FinalNewline || bytesMeta.HashScope != "exact_file_bytes" {
		return fmt.Errorf("projects-v1: canonical fixture byte contract is invalid")
	}
	return nil
}

func validateInstructionHashFixture(fixture contractFixture) error {
	if fixture.InstructionContentHash.Algorithm != "sha256" || fixture.InstructionContentHash.Output != "lowercase_hex" || fixture.InstructionContentHash.Encoding != "utf-8" || fixture.InstructionContentHash.LineEndings != "lf" {
		return fmt.Errorf("projects-v1: instruction content hash contract is invalid")
	}
	return nil
}

func validateSafeIntegerFixture(fixture contractFixture) error {
	if fixture.Limits.WireInteger.Minimum != MinSafeJSONInteger || fixture.Limits.WireInteger.Maximum != int64(MaxSafeJSONInteger) ||
		fixture.DTO.ProjectRead.VersionRange != "1..9007199254740991" || fixture.DTO.InstructionRead.VersionRange != "1..9007199254740991" ||
		fixture.DTO.WorkItemRead.VersionRange != "1..9007199254740991" || fixture.DTO.WorkItemRead.RankRange != "-9007199254740991..9007199254740991" {
		return fmt.Errorf("projects-v1: safe JSON integer contract is invalid")
	}
	return nil
}

func validateExpectedVersionFixture(fixture contractFixture) error {
	for _, rule := range []fixtureIntegerRule{
		fixture.DTO.ProjectPatch.ExpectedVersion, fixture.DTO.InstructionPut.ExpectedVersion,
		fixture.DTO.WorkItemPatch.ExpectedVersion, fixture.DTO.WorkItemMove.ExpectedVersion,
	} {
		if rule.Minimum != 1 || rule.Maximum != MaxExpectedVersion {
			return fmt.Errorf("projects-v1: expected version range is invalid")
		}
	}
	return nil
}

func InstructionContentSHA256(humanDescription, agentInstruction string) string {
	humanDescription = normalizeContractLF(humanDescription)
	agentInstruction = normalizeContractLF(agentInstruction)
	framed := fmt.Sprintf("projects-v1:instruction-content:v1\nhuman_description:%d\n%s\nagent_instruction:%d\n%s", len([]byte(humanDescription)), humanDescription, len([]byte(agentInstruction)), agentInstruction)
	sum := sha256.Sum256([]byte(framed))
	return hex.EncodeToString(sum[:])
}

func normalizeContractLF(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

type CapabilityStatus string

const (
	CapabilityUnavailable  CapabilityStatus = "unavailable"
	CapabilityAvailable    CapabilityStatus = "available"
	CapabilityIncompatible CapabilityStatus = "incompatible"
)

type CapabilityFeatures struct {
	Instructions     bool
	WorkItems        bool
	Members          bool
	Activity         bool
	Bindings         bool
	WorkItemFields   bool
	Assets           bool
	V2Status         CapabilityStatus
}

func EvaluateCapability(raw json.RawMessage) (CapabilityStatus, error) {
	status, _, err := EvaluateCapabilityFeatures(raw)
	return status, err
}

func EvaluateCapabilityFeatures(raw json.RawMessage) (CapabilityStatus, CapabilityFeatures, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return CapabilityUnavailable, CapabilityFeatures{}, nil
	}
	fields, err := decodeCapabilityFields(trimmed)
	if err != nil {
		return CapabilityUnavailable, CapabilityFeatures{}, err
	}
	enabled, err := requiredCapabilityBoolean(fields, "projects-v1", "enabled")
	if err != nil {
		return CapabilityUnavailable, CapabilityFeatures{}, err
	}
	if !enabled {
		return CapabilityUnavailable, CapabilityFeatures{}, nil
	}
	instructions, err := optionalCapabilityBoolean(fields, "projects-v1", "instructions")
	if err != nil {
		return CapabilityUnavailable, CapabilityFeatures{}, err
	}
	workItems, err := optionalCapabilityBoolean(fields, "projects-v1", "work_items")
	if err != nil {
		return CapabilityUnavailable, CapabilityFeatures{}, err
	}
	return CapabilityAvailable, CapabilityFeatures{Instructions: instructions, WorkItems: workItems}, nil
}

func decodeCapabilityFields(payload []byte) (map[string]json.RawMessage, error) {
	if !utf8.Valid(payload) {
		return nil, fmt.Errorf("projects-v1: capability is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, fmt.Errorf("projects-v1: decode capability shape: %w", err)
	}
	return fields, nil
}

func requiredCapabilityBoolean(fields map[string]json.RawMessage, capability, field string) (bool, error) {
	raw, present := fields[field]
	if !present {
		return false, fmt.Errorf("%s: capability field %s must be present", capability, field)
	}
	return parseCapabilityBoolean(raw, capability, field)
}

func optionalCapabilityBoolean(fields map[string]json.RawMessage, capability, field string) (bool, error) {
	raw, present := fields[field]
	if !present {
		return false, nil
	}
	return parseCapabilityBoolean(raw, capability, field)
}

func parseCapabilityBoolean(raw json.RawMessage, capability, field string) (bool, error) {
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s: capability field %s must be a boolean", capability, field)
	}
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("projects-v1: multiple JSON values")
		}
		return fmt.Errorf("projects-v1: decode trailing JSON: %w", err)
	}
	return nil
}
