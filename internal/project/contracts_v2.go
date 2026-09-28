package project

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	V2ContractVersion = "projects-v2-20260822.1"
	v2ContractHash    = "1f246eaea4c3b715d79cfea035669beaa64f9f56dd413cc20d0d097f3cab592c"
)

//go:embed testdata/projects-v2/contracts.json
var projectsV2ContractBytes []byte

type V2CapabilityFeatures struct {
	Members        bool
	Activity       bool
	Bindings       bool
	WorkItemFields bool
	Assets         bool
}

type projectsV2Capability struct {
	Enabled bool
	Slices  map[string]json.RawMessage
}

func CanonicalV2ContractSHA256() string {
	sum := sha256.Sum256(projectsV2ContractBytes)
	return hex.EncodeToString(sum[:])
}

func EvaluateV2Capability(raw json.RawMessage) (CapabilityStatus, V2CapabilityFeatures, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return CapabilityUnavailable, V2CapabilityFeatures{}, nil
	}
	capability, err := decodeProjectsV2Capability(trimmed)
	if err != nil {
		return CapabilityUnavailable, V2CapabilityFeatures{}, err
	}
	if !capability.Enabled {
		return CapabilityUnavailable, V2CapabilityFeatures{}, nil
	}
	features, err := projectsV2CapabilityFeatures(capability.Slices)
	if err != nil {
		return CapabilityUnavailable, V2CapabilityFeatures{}, err
	}
	return CapabilityAvailable, features, nil
}

func decodeProjectsV2Capability(raw []byte) (projectsV2Capability, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return projectsV2Capability{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return projectsV2Capability{}, fmt.Errorf("projects-v2: decode capability: %w", err)
	}
	enabled, err := requiredCapabilityBoolean(fields, "projects-v2", "enabled")
	if err != nil {
		return projectsV2Capability{}, err
	}
	if !enabled {
		return projectsV2Capability{}, nil
	}
	var slices map[string]json.RawMessage
	if rawSlices, present := fields["slices"]; present {
		if err := json.Unmarshal(rawSlices, &slices); err != nil {
			return projectsV2Capability{}, fmt.Errorf("projects-v2: decode capability slices: %w", err)
		}
	}
	return projectsV2Capability{Enabled: enabled, Slices: slices}, nil
}

func projectsV2CapabilityFeatures(slices map[string]json.RawMessage) (V2CapabilityFeatures, error) {
	want := []string{"members", "activity", "bindings", "work_item_fields", "assets"}
	values := make(map[string]bool, len(want))
	for _, name := range want {
		rawValue, ok := slices[name]
		if !ok {
			continue
		}
		value, err := parseCapabilityBoolean(rawValue, "projects-v2", "slices."+name)
		if err != nil {
			return V2CapabilityFeatures{}, fmt.Errorf("projects-v2: invalid slice %s", name)
		}
		values[name] = value
	}
	return V2CapabilityFeatures{
		Members: values["members"], Activity: values["activity"], Bindings: values["bindings"],
		WorkItemFields: values["work_item_fields"], Assets: values["assets"],
	}, nil
}
