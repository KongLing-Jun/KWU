package capabilitypackage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/semver"
)

const (
	SchemaVersion       = 1
	MaxManifestBytes    = 64 << 10
	MaxInstructionsSize = 64 << 10
	MaxSkillBytes       = 32 << 10
	MaxSkills           = 16
)

var safeID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,79}$`)

type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	ID            string       `json:"id"`
	Version       string       `json:"version"`
	Entrypoints   Entrypoints  `json:"entrypoints"`
	MCPServers    []MCPServer  `json:"mcpServers,omitempty"`
	HostActions   []string     `json:"hostActions,omitempty"`
	Skills        []SkillEntry `json:"skills,omitempty"`
}

// SkillEntry declares an inline playbook, not an execution grant. This first
// contract deliberately accepts only an explicit empty business-tool list.
type SkillEntry struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	AllowedTools []string `json:"allowedTools"`
}

type Skill struct {
	Name        string
	Description string
	Body        string
}

type Entrypoints struct {
	Instructions string `json:"instructions"`
}

type MCPServer struct {
	ID            string   `json:"id"`
	Transport     string   `json:"transport"`
	URL           string   `json:"url"`
	ReadOnlyTools []string `json:"readOnlyTools,omitempty"`
}

type Package struct {
	Manifest        Manifest
	Instructions    string
	InstructionHash string
	ContentDigest   string
	Skills          []Skill
}

func Load(source fs.FS, manifestPath string) (Package, error) {
	manifestPath, err := cleanPackagePath(manifestPath)
	if err != nil {
		return Package{}, err
	}
	payload, err := readBounded(source, manifestPath, MaxManifestBytes)
	if err != nil {
		return Package{}, fmt.Errorf("read capability manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Package{}, fmt.Errorf("decode capability manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Package{}, errors.New("capability manifest contains trailing data")
	}
	if err := validateManifest(manifest); err != nil {
		return Package{}, err
	}
	base := path.Dir(manifestPath)
	instructionPath, err := cleanPackagePath(path.Join(base, manifest.Entrypoints.Instructions))
	if err != nil || instructionPath == base || !strings.HasPrefix(instructionPath, base+"/") {
		return Package{}, errors.New("capability instructions escape package root")
	}
	instructions, err := readBounded(source, instructionPath, MaxInstructionsSize)
	if err != nil {
		return Package{}, fmt.Errorf("read capability instructions: %w", err)
	}
	if !utf8.Valid(instructions) {
		return Package{}, errors.New("capability instructions are not valid UTF-8")
	}
	instructionSum := sha256.Sum256(instructions)
	contentInput := append(append([]byte(nil), payload...), 0)
	contentInput = append(contentInput, instructions...)
	skills, skillDigests, err := loadSkills(source, base, manifest.Skills)
	if err != nil {
		return Package{}, err
	}
	contentInput = append(contentInput, skillDigests...)
	contentSum := sha256.Sum256(contentInput)
	return Package{
		Manifest: manifest, Instructions: string(instructions),
		InstructionHash: hex.EncodeToString(instructionSum[:]),
		ContentDigest:   hex.EncodeToString(contentSum[:]),
		Skills:          skills,
	}, nil
}

func (p Package) AllowsHostAction(action string) bool {
	for _, allowed := range p.Manifest.HostActions {
		if allowed == action {
			return true
		}
	}
	return false
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return errors.New("unsupported capability manifest schema")
	}
	if !safeID.MatchString(manifest.ID) {
		return errors.New("invalid capability package id")
	}
	version := manifest.Version
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if !semver.IsValid(version) || semver.Prerelease(version) != "" {
		return errors.New("invalid capability package version")
	}
	if _, err := cleanRelativeEntrypoint(manifest.Entrypoints.Instructions); err != nil {
		return fmt.Errorf("invalid capability instruction entrypoint: %w", err)
	}
	seenServers := map[string]bool{}
	for _, server := range manifest.MCPServers {
		if !safeID.MatchString(server.ID) || seenServers[server.ID] {
			return errors.New("invalid or duplicate capability MCP server id")
		}
		seenServers[server.ID] = true
		if server.Transport != "streamable-http" || !strings.HasPrefix(server.URL, "https://") {
			return errors.New("capability MCP server must use HTTPS streamable-http")
		}
		seenTools := map[string]bool{}
		for _, toolName := range server.ReadOnlyTools {
			if strings.TrimSpace(toolName) == "" || seenTools[toolName] {
				return errors.New("invalid or duplicate capability read-only tool")
			}
			seenTools[toolName] = true
		}
	}
	seenActions := map[string]bool{}
	for _, action := range manifest.HostActions {
		if action != "dashboard.add_widget" || seenActions[action] {
			return errors.New("unknown or duplicate capability Host action")
		}
		seenActions[action] = true
	}
	if err := validateSkillEntries(manifest.Skills); err != nil {
		return err
	}
	return nil
}

func validateSkillEntries(entries []SkillEntry) error {
	if len(entries) > MaxSkills {
		return errors.New("too many capability skills")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if !safeID.MatchString(entry.Name) || len(entry.Name) > 63 || seen[entry.Name] {
			return errors.New("invalid or duplicate capability skill name")
		}
		seen[entry.Name] = true
		if entry.Path != "skills/"+entry.Name+"/SKILL.md" {
			return errors.New("capability skill path must be canonical and package-local")
		}
		if entry.AllowedTools == nil || len(entry.AllowedTools) != 0 {
			return errors.New("capability skill requires an explicit empty business-tool intent")
		}
	}
	return nil
}

func loadSkills(source fs.FS, base string, entries []SkillEntry) ([]Skill, []byte, error) {
	var skills []Skill
	var digests []byte
	for _, entry := range entries {
		name := path.Join(base, entry.Path)
		info, err := fs.Stat(source, name)
		if err != nil || !info.Mode().IsRegular() {
			return nil, nil, errors.New("capability skill is not a regular package file")
		}
		raw, err := readBounded(source, name, MaxSkillBytes)
		if err != nil {
			return nil, nil, errors.New("capability skill cannot be read within its size limit")
		}
		sk, err := parseSkill(raw, entry.Name)
		if err != nil {
			return nil, nil, err
		}
		skills = append(skills, sk)
		sum := sha256.Sum256(raw)
		digests = append(digests, 0)
		digests = append(digests, entry.Path...)
		digests = append(digests, 0)
		digests = append(digests, sum[:]...)
	}
	return skills, digests, nil
}

// parseSkill intentionally accepts a small, literal frontmatter grammar. It
// never evaluates YAML tags, templates, shell substitutions, or scripts.
func parseSkill(raw []byte, expectedName string) (Skill, error) {
	if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return Skill{}, errors.New("capability skill must be UTF-8 text")
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return Skill{}, errors.New("capability skill requires frontmatter")
	}
	metadata, end, err := parseSkillMetadata(lines)
	if err != nil {
		return Skill{}, err
	}
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	if metadata["name"] != expectedName || metadata["allowed-tools"] != "[]" || body == "" {
		return Skill{}, errors.New("invalid capability skill identity, mode, tool intent, or body")
	}
	description := metadata["description"]
	if description == "" || len(description) > 512 || strings.ContainsAny(description, "\r\t") {
		return Skill{}, errors.New("invalid capability skill description")
	}
	return Skill{Name: expectedName, Description: description, Body: body}, nil
}

func parseSkillMetadata(lines []string) (map[string]string, int, error) {
	metadata := map[string]string{}
	for index := 1; index < len(lines); index++ {
		line := lines[index]
		if line == "---" {
			if len(metadata) != 3 {
				break
			}
			return metadata, index, nil
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || (key != "name" && key != "description" && key != "allowed-tools") {
			break
		}
		if _, duplicate := metadata[key]; duplicate {
			break
		}
		metadata[key] = strings.TrimSpace(value)
	}
	return nil, 0, errors.New("invalid or unsupported capability skill frontmatter")
}

func cleanRelativeEntrypoint(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
		return "", errors.New("entrypoint must be a clean relative path")
	}
	return value, nil
}

func cleanPackagePath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
		return "", errors.New("package path must be clean and relative")
	}
	return value, nil
}

func readBounded(source fs.FS, name string, limit int) ([]byte, error) {
	file, err := source.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > limit {
		return nil, errors.New("capability package file exceeds size limit")
	}
	return payload, nil
}
