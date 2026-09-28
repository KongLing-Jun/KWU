package main

import (
	"context"
	"encoding/json"
	"errors"

	"kwu/desktop/internal/capabilitypackage"
	"kwu/internal/tool"
)

const capabilityLoadSkillToolName = "capability_load_skill"

type capabilitySkillTool struct {
	schema json.RawMessage
	load   func(string) (capabilitypackage.Skill, capabilitypackage.Package, error)
}

func newCapabilitySkillTool(stateRoot, identity string, pinned capabilitySessionBinding, pkg capabilitypackage.Package) tool.Tool {
	names := make([]string, 0, len(pkg.Skills))
	for _, sk := range pkg.Skills {
		names = append(names, sk.Name)
	}
	schema, _ := json.Marshal(map[string]any{
		"type": "object", "required": []string{"name"}, "additionalProperties": false,
		"properties": map[string]any{"name": map[string]any{"type": "string", "enum": names}},
	})
	return capabilitySkillTool{schema: schema, load: func(name string) (capabilitypackage.Skill, capabilitypackage.Package, error) {
		return loadPinnedCapabilitySkill(stateRoot, identity, pinned, name)
	}}
}

func (capabilitySkillTool) Name() string { return capabilityLoadSkillToolName }
func (capabilitySkillTool) Description() string {
	return "Load one declared playbook from this capability's pinned package into the current model context. This is a read-only static load, not workflow execution, data collection, or a permission grant. No business tools or Dashboard mutations are invoked by this loader."
}
func (capabilitySkillTool) ReadOnly() bool     { return true }
func (capabilitySkillTool) PlanModeSafe() bool { return true }
func (t capabilitySkillTool) Schema() json.RawMessage {
	return append(json.RawMessage(nil), t.schema...)
}

func (t capabilitySkillTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var args struct {
		Name string `json:"name"`
	}
	if len(raw) > 1024 || decodeCapabilityJSON(raw, &args) != nil || args.Name == "" || t.load == nil {
		return "", errors.New("capability_load_skill: invalid request")
	}
	sk, pkg, err := t.load(args.Name)
	if err != nil {
		return "", err
	}
	result, err := json.Marshal(struct {
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		PackageVersion   string   `json:"packageVersion"`
		PackageDigest    string   `json:"packageDigest"`
		Instructions     string   `json:"instructions"`
		AllowedTools     []string `json:"allowedTools"`
		PermissionNotice string   `json:"permissionNotice"`
	}{sk.Name, sk.Description, pkg.Manifest.Version, pkg.ContentDigest, sk.Body, []string{},
		"Empty allowedTools is playbook intent, not revocation of parent-turn permissions. This load is one tool call; no business/data tool or Dashboard action was executed. Plan readiness is not source read-readiness or execution authorization."})
	return string(result), err
}

func loadPinnedCapabilitySkill(stateRoot, identity string, pinned capabilitySessionBinding, name string) (capabilitypackage.Skill, capabilitypackage.Package, error) {
	var current capabilitySessionBinding
	if readCapabilityJSON(capabilityStableBindingPath(stateRoot, identity), &current) != nil ||
		validateCapabilitySessionBinding(current) != nil || current.SessionIdentity != identity ||
		current.SessionIdentity != pinned.SessionIdentity || current.Scope != pinned.Scope ||
		capabilitySessionScope(current) != capabilitySessionScope(pinned) ||
		current.PackageVersion != pinned.PackageVersion || current.PackageDigest != pinned.PackageDigest || current.InstructionHash != pinned.InstructionHash {
		return capabilitypackage.Skill{}, capabilitypackage.Package{}, errors.New("capability_load_skill: binding changed or unavailable")
	}
	pkg, found, err := resolveCapabilityPackage(current.Scope.CapabilityID)
	if err != nil || !found || current.PackageVersion != pkg.Manifest.Version || current.PackageDigest != pkg.ContentDigest || current.InstructionHash != pkg.InstructionHash {
		return capabilitypackage.Skill{}, capabilitypackage.Package{}, errors.New("capability_load_skill: package pin mismatch")
	}
	for _, sk := range pkg.Skills {
		if sk.Name == name {
			return sk, pkg, nil
		}
	}
	return capabilitypackage.Skill{}, capabilitypackage.Package{}, errors.New("capability_load_skill: undeclared skill")
}

var _ tool.Tool = capabilitySkillTool{}
