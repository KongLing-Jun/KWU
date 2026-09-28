package capabilitypackage

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

const validSkill = "---\nname: research-plan\ndescription: Draft an evidence plan.\nallowed-tools: []\n---\nLiteral playbook body.\n"

func skillFixture(t *testing.T, entries []SkillEntry, body []byte) fstest.MapFS {
	t.Helper()
	var manifest Manifest
	if err := json.Unmarshal([]byte(validManifest), &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Skills = entries
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{
		"crossborder/capability.json":               {Data: raw},
		"crossborder/AGENTS.md":                     {Data: []byte("scope")},
		"crossborder/skills/research-plan/SKILL.md": {Data: body},
	}
}

func TestSkillPackageDigestIncludesRawBodyAndMetadata(t *testing.T) {
	entry := SkillEntry{Name: "research-plan", Path: "skills/research-plan/SKILL.md", AllowedTools: []string{}}
	base, err := Load(skillFixture(t, []SkillEntry{entry}, []byte(validSkill)), "crossborder/capability.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Skills) != 1 || base.Skills[0].Name != entry.Name {
		t.Fatal("missing Skill")
	}
	for _, body := range []string{validSkill + "changed", strings.ReplaceAll(validSkill, "\n", "\r\n"), strings.Replace(validSkill, "Draft an evidence plan.", "New description.", 1)} {
		changed, err := Load(skillFixture(t, []SkillEntry{entry}, []byte(body)), "crossborder/capability.json")
		if err != nil {
			t.Fatal(err)
		}
		if changed.ContentDigest == base.ContentDigest || changed.InstructionHash != base.InstructionHash {
			t.Fatal("raw Skill bytes must change only the package digest")
		}
	}
}

func TestSkillPackageRejectsInvalidDeclarations(t *testing.T) {
	entry := SkillEntry{Name: "research-plan", Path: "skills/research-plan/SKILL.md", AllowedTools: []string{}}
	cases := map[string][]SkillEntry{
		"duplicate":     {entry, entry},
		"traversal":     {{Name: entry.Name, Path: "skills/../SKILL.md", AllowedTools: []string{}}},
		"absolute":      {{Name: entry.Name, Path: "C:/SKILL.md", AllowedTools: []string{}}},
		"backslash":     {{Name: entry.Name, Path: `skills\research-plan\SKILL.md`, AllowedTools: []string{}}},
		"missing tools": {{Name: entry.Name, Path: entry.Path}},
		"tool grant":    {{Name: entry.Name, Path: entry.Path, AllowedTools: []string{"bash"}}},
		"invalid name":  {{Name: "Research-Plan", Path: entry.Path, AllowedTools: []string{}}},
		"too many":      make([]SkillEntry, MaxSkills+1),
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(skillFixture(t, entries, []byte(validSkill)), "crossborder/capability.json"); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestSkillPackageRejectsMalformedOrMissingBody(t *testing.T) {
	entry := SkillEntry{Name: "research-plan", Path: "skills/research-plan/SKILL.md", AllowedTools: []string{}}
	cases := map[string][]byte{
		"utf8": {0xff}, "oversize": []byte(strings.Repeat("x", MaxSkillBytes+1)),
		"no frontmatter": []byte("hello"),
		"duplicate":      []byte(strings.Replace(validSkill, "allowed-tools: []", "allowed-tools: []\nallowed-tools: []", 1)),
		"unknown":        []byte(strings.Replace(validSkill, "allowed-tools: []", "allowed-tools: []\nscript: run.sh", 1)),
		"name mismatch":  []byte(strings.Replace(validSkill, "name: research-plan", "name: other", 1)),
		"subagent":       []byte(strings.Replace(validSkill, "allowed-tools: []", "allowed-tools: []\nrunAs: subagent", 1)),
		"tools":          []byte(strings.Replace(validSkill, "allowed-tools: []", "allowed-tools: [bash]", 1)),
		"missing field":  []byte(strings.Replace(validSkill, "allowed-tools: []\n", "", 1)),
		"empty body":     []byte(strings.Replace(validSkill, "Literal playbook body.", "", 1)),
		"nul":            []byte(validSkill + "\x00"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(skillFixture(t, []SkillEntry{entry}, body), "crossborder/capability.json"); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	source := skillFixture(t, []SkillEntry{entry}, []byte(validSkill))
	delete(source, "crossborder/skills/research-plan/SKILL.md")
	if _, err := Load(source, "crossborder/capability.json"); err == nil {
		t.Fatal("missing file accepted")
	}
}

const validManifest = `{
  "schemaVersion": 1,
  "id": "crossborder-commerce",
  "version": "1.0.0",
  "entrypoints": {"instructions": "AGENTS.md"},
  "mcpServers": [{"id":"shopify-global-catalog","transport":"streamable-http","url":"https://catalog.shopify.com/api/ucp/mcp","readOnlyTools":["search_catalog"]}],
  "hostActions": ["dashboard.add_widget"]
}`

func TestLoadPackageProducesStableIdentityAndInstructions(t *testing.T) {
	source := fstest.MapFS{
		"crossborder/capability.json": {Data: []byte(validManifest)},
		"crossborder/AGENTS.md":       {Data: []byte("Use catalog facts as snapshots.\n")},
	}
	first, err := Load(source, "crossborder/capability.json")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(source, "crossborder/capability.json")
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.ID != "crossborder-commerce" || first.InstructionHash == "" || first.ContentDigest == "" {
		t.Fatalf("incomplete package: %#v", first)
	}
	if first.InstructionHash != second.InstructionHash || first.ContentDigest != second.ContentDigest {
		t.Fatal("package identity is not deterministic")
	}
	if !first.AllowsHostAction("dashboard.add_widget") || first.AllowsHostAction("dashboard.remove_widget") {
		t.Fatal("Host action allowlist is incorrect")
	}
}

func TestLoadPackageFailsClosed(t *testing.T) {
	tests := map[string]string{
		"unknown field":  strings.Replace(validManifest, `"schemaVersion": 1,`, `"schemaVersion": 1, "trusted": true,`, 1),
		"unknown action": strings.Replace(validManifest, "dashboard.add_widget", "dashboard.remove_widget", 1),
		"unsafe path":    strings.Replace(validManifest, "AGENTS.md", "../AGENTS.md", 1),
		"bad version":    strings.Replace(validManifest, "1.0.0", "latest", 1),
	}
	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			source := fstest.MapFS{
				"crossborder/capability.json": {Data: []byte(manifest)},
				"crossborder/AGENTS.md":       {Data: []byte("safe")},
			}
			if _, err := Load(source, "crossborder/capability.json"); err == nil {
				t.Fatal("expected package rejection")
			}
		})
	}
}

func TestLoadPackageRejectsInvalidOrOversizedInstructions(t *testing.T) {
	for name, instructions := range map[string][]byte{
		"invalid utf8": {0xff, 0xfe},
		"oversized":    []byte(strings.Repeat("x", MaxInstructionsSize+1)),
	} {
		t.Run(name, func(t *testing.T) {
			source := fstest.MapFS{
				"crossborder/capability.json": {Data: []byte(validManifest)},
				"crossborder/AGENTS.md":       {Data: instructions},
			}
			if _, err := Load(source, "crossborder/capability.json"); err == nil {
				t.Fatal("expected instruction rejection")
			}
		})
	}
}
