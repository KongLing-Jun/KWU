package connector

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
	"golang.org/x/net/http/httpguts"
)

const (
	MaxPackageFiles             = 600
	MaxPackageUncompressedBytes = 30 << 20
	MaxManifestBytes            = 1 << 20
)

var (
	tokenFieldPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
	templateField     = regexp.MustCompile(`\{([^{}]+)\}`)
)

type inspectedZip struct {
	files map[string]*zip.File
}

func inspectPackageZip(reader *zip.Reader) (inspectedZip, error) {
	files := make(map[string]*zip.File, len(reader.File))
	var total uint64
	regular := 0
	for _, file := range reader.File {
		name, err := normalizeZipPath(file.Name)
		if err != nil {
			return inspectedZip{}, err
		}
		key := strings.ToLower(name)
		if _, exists := files[key]; exists {
			return inspectedZip{}, errors.New("connector package: duplicate normalized path")
		}
		mode := file.Mode()
		if mode&os.ModeSymlink != 0 || !file.FileInfo().IsDir() && (mode&0o111 != 0 || prohibitedExecutablePath(name)) {
			return inspectedZip{}, errors.New("connector package: prohibited file mode")
		}
		if !file.FileInfo().IsDir() && !file.FileInfo().Mode().IsRegular() {
			return inspectedZip{}, errors.New("connector package: non-regular file")
		}
		if !allowedPackagePath(name, file.FileInfo().IsDir()) {
			return inspectedZip{}, errors.New("connector package: unexpected package path")
		}
		files[key] = file
		if !file.FileInfo().IsDir() {
			regular++
			if regular > MaxPackageFiles {
				return inspectedZip{}, errors.New("connector package: too many files")
			}
			total += file.UncompressedSize64
			if total > MaxPackageUncompressedBytes {
				return inspectedZip{}, errors.New("connector package: uncompressed size limit exceeded")
			}
		}
	}
	return inspectedZip{files: files}, nil
}

func normalizeZipPath(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return "", errors.New("connector package: unsafe path")
	}
	if len(name) >= 2 && isASCIILetter(name[0]) && name[1] == ':' {
		return "", errors.New("connector package: unsafe path")
	}
	withoutSlash := strings.TrimSuffix(name, "/")
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != withoutSlash {
		return "", errors.New("connector package: unsafe path")
	}
	return clean, nil
}

func allowedPackagePath(name string, directory bool) bool {
	if isControlFile(name) {
		return !directory
	}
	parts := strings.Split(name, "/")
	if directory {
		return name == "skills" || len(parts) == 2 && parts[0] == "skills" && parts[1] != ""
	}
	return len(parts) == 3 && parts[0] == "skills" && parts[1] != "" && parts[2] == "SKILL.md"
}

func isControlFile(name string) bool {
	switch name {
	case "connector.json", "mcp.json", "token-schema.json":
		return true
	default:
		return false
	}
}

func prohibitedExecutablePath(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".exe", ".com", ".bat", ".cmd", ".ps1", ".sh", ".msi", ".dll", ".so", ".dylib", ".jar":
		return true
	default:
		return false
	}
}

func parseManifests(z inspectedZip, item CatalogItem, policy RuntimePolicy) (Manifest, error) {
	controls := map[string][]byte{}
	regular := map[string]struct{}{}
	for key, file := range z.files {
		if file.FileInfo().IsDir() {
			continue
		}
		name, err := normalizeZipPath(file.Name)
		if err != nil {
			return Manifest{}, err
		}
		regular[name] = struct{}{}
		if isControlFile(key) {
			raw, err := readZipControl(file)
			if err != nil {
				return Manifest{}, err
			}
			controls[key] = raw
		}
	}
	return parseControlFiles(controls, regular, item, policy)
}

func parseControlFiles(controls map[string][]byte, regular map[string]struct{}, item CatalogItem, policy RuntimePolicy) (Manifest, error) {
	raw, ok := controls["connector.json"]
	if !ok {
		return Manifest{}, errors.New("connector package: missing connector.json")
	}
	var connector ConnectorManifest
	if err := decodeStrict(bytes.TrimSpace(raw), &connector); err != nil {
		return Manifest{}, fmt.Errorf("connector package: connector.json: %w", err)
	}
	if err := validateConnectorManifest(connector, item, policy); err != nil {
		return Manifest{}, err
	}
	result := Manifest{Connector: connector}
	var mcp MCPManifest
	if err := decodeControl(controls, "mcp.json", &mcp); err != nil {
		return Manifest{}, err
	}
	tokenReferenced, err := validateMCPManifest(mcp)
	if err != nil {
		return Manifest{}, err
	}
	result.MCP = &mcp
	_, tokenExists := regular["token-schema.json"]
	if tokenExists != tokenReferenced {
		return Manifest{}, errors.New("connector package: token schema reference mismatch")
	}
	if tokenReferenced {
		var token TokenSchema
		if err := decodeControl(controls, "token-schema.json", &token); err != nil {
			return Manifest{}, err
		}
		if err := validateTokenSchema(token); err != nil {
			return Manifest{}, err
		}
		result.Token = &token
	}
	if err := validateReferencedRoots(connector.Entrypoints, regular); err != nil {
		return Manifest{}, err
	}
	return result, nil
}

func LoadInstalledManifest(packageRoot string, active ActiveVersion, policy RuntimePolicy) (Manifest, error) {
	controls := map[string][]byte{}
	regular := map[string]struct{}{}
	err := filepath.WalkDir(packageRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(packageRoot, filePath)
		if err != nil || rel == "." {
			return err
		}
		name := filepath.ToSlash(rel)
		if !allowedPackagePath(name, entry.IsDir()) {
			return errors.New("connector package: unexpected package path")
		}
		if entry.IsDir() {
			return nil
		}
		regular[name] = struct{}{}
		if isControlFile(name) {
			raw, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			controls[name] = raw
		}
		return nil
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("connector package: read active package: %w", err)
	}
	var connector ConnectorManifest
	if err := decodeStrict(controls["connector.json"], &connector); err != nil {
		return Manifest{}, errors.New("connector package: invalid active connector.json")
	}
	item := CatalogItem{
		ID: connector.ID, Name: connector.Name, Provider: connector.Provider, Description: connector.Description,
		Category: connector.Category, Kind: connector.Kind, Version: active.Version, MinClientVersion: connector.MinClientVersion,
		Capabilities: connector.Capabilities,
	}
	if connector.ID != active.ConnectorID || connector.Version != active.Version {
		return Manifest{}, errors.New("connector package: active pointer mismatch")
	}
	return parseControlFiles(controls, regular, item, policy)
}

func decodeControl(controls map[string][]byte, name string, target any) error {
	raw, ok := controls[name]
	if !ok {
		return fmt.Errorf("connector package: missing %s", name)
	}
	if err := decodeStrict(bytes.TrimSpace(raw), target); err != nil {
		return fmt.Errorf("connector package: %s: %w", name, err)
	}
	return nil
}

func readZipControl(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > MaxManifestBytes {
		return nil, errors.New("connector package: manifest too large")
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, MaxManifestBytes+1))
	if err != nil || int64(len(raw)) > MaxManifestBytes || uint64(len(raw)) != file.UncompressedSize64 {
		return nil, errors.New("connector package: unreadable manifest")
	}
	return raw, nil
}

func validateConnectorManifest(m ConnectorManifest, item CatalogItem, policy RuntimePolicy) error {
	policy, err := NormalizeRuntimePolicy(policy)
	if err != nil {
		return errors.New("connector package: invalid runtime client version")
	}
	if m.SchemaVersion != ConnectorSchemaVersion || !validConnectorID(m.ID) || m.ID != item.ID || m.Version != item.Version || m.MinClientVersion != item.MinClientVersion || m.Kind != item.Kind {
		return errors.New("connector package: connector manifest mismatch")
	}
	if !isCanonicalWireSemver(m.Version) || !isCanonicalWireSemver(m.MinClientVersion) || !nonEmpty(m.Name) || !nonEmpty(m.Provider) || !nonEmpty(m.Description) || !nonEmpty(m.Category) {
		return errors.New("connector package: invalid connector metadata")
	}
	if m.Name != item.Name || m.Provider != item.Provider || m.Description != item.Description || m.Category != item.Category || !equalStrings(m.Capabilities, item.Capabilities) {
		return errors.New("connector package: catalog metadata mismatch")
	}
	if m.IconURL != nil && !validHTTPSURL(*m.IconURL) || m.Examples == nil || len(m.Examples) > 8 || !nonEmptyStrings(m.Examples) || m.Capabilities == nil || !nonEmptyStrings(m.Capabilities) {
		return errors.New("connector package: invalid connector metadata")
	}
	if _, err := platformSet(m.Platforms); err != nil {
		return err
	}
	// Schema 2 only supports public HTTPS streamable MCP servers (and optional
	// prompt-only Skills), so a valid package has no OS- or CPU-specific runtime.
	if policy.ClientVersion != "dev" && semver.Compare("v"+policy.ClientVersion, "v"+m.MinClientVersion) < 0 {
		return errors.New("connector package: Desktop client version is unsupported")
	}
	if m.Entrypoints.MCP != "mcp.json" {
		return errors.New("connector package: entrypoint mismatch")
	}
	if m.Entrypoints.Skills != "" && m.Entrypoints.Skills != "skills" {
		return errors.New("connector package: entrypoint mismatch")
	}
	if m.Entrypoints.Skills == "" && m.Kind != "mcp" || m.Entrypoints.Skills != "" && m.Kind != "composite" {
		return errors.New("connector package: kind and entrypoints mismatch")
	}
	return nil
}

func platformSet(items []ManifestPlatform) (map[string]map[string]struct{}, error) {
	result := make(map[string]map[string]struct{}, len(items))
	for _, item := range items {
		if !allowedPlatform(item.OS) || len(item.Architectures) == 0 {
			return nil, errors.New("connector package: invalid platforms")
		}
		if _, exists := result[item.OS]; exists {
			return nil, errors.New("connector package: duplicate platform")
		}
		architectures := map[string]struct{}{}
		for _, architecture := range item.Architectures {
			if !allowedArchitecture(architecture) {
				return nil, errors.New("connector package: invalid architecture")
			}
			if _, exists := architectures[architecture]; exists {
				return nil, errors.New("connector package: duplicate architecture")
			}
			architectures[architecture] = struct{}{}
		}
		result[item.OS] = architectures
	}
	return result, nil
}

func validateReferencedRoots(entrypoints ManifestEntrypoints, files map[string]struct{}) error {
	_, hasMCP := files["mcp.json"]
	if !hasMCP || entrypoints.MCP != "mcp.json" {
		return errors.New("connector package: entrypoint reference mismatch")
	}
	hasSkillFile := false
	for name := range files {
		if !strings.HasPrefix(name, "skills/") {
			continue
		}
		hasSkillFile = true
		parts := strings.Split(name, "/")
		if len(parts) != 3 || parts[1] == "" || parts[2] != "SKILL.md" {
			return errors.New("connector package: invalid skill path")
		}
	}
	if hasSkillFile != (entrypoints.Skills != "") {
		return errors.New("connector package: skills reference mismatch")
	}
	return nil
}

func validateMCPManifest(mcp MCPManifest) (bool, error) {
	if mcp.SchemaVersion != MCPSchemaVersion || len(mcp.Servers) == 0 {
		return false, errors.New("connector package: invalid mcp manifest")
	}
	serverIDs := map[string]struct{}{}
	tokenReferenced := false
	for _, server := range mcp.Servers {
		if !nonEmpty(server.ID) {
			return false, errors.New("connector package: invalid mcp server id")
		}
		if _, exists := serverIDs[server.ID]; exists {
			return false, errors.New("connector package: duplicate mcp server id")
		}
		serverIDs[server.ID] = struct{}{}
		if err := validateStaticHeaders(server.StaticHeaders); err != nil {
			return false, err
		}
		if server.Transport != "streamable-http" || !validMCPHTTPSURL(server.URL) {
			return false, errors.New("connector package: unsupported MCP transport")
		}
		switch server.Auth.Type {
		case "none":
			if server.Auth.Provider != "" || server.Auth.Scopes != nil || server.Auth.Resource != "" || server.Auth.ClientID != "" || server.Auth.Schema != "" {
				return false, errors.New("connector package: invalid none auth")
			}
		case "oauth":
			if server.Auth.Provider != "" {
				if server.Auth.Provider != "github" || server.Auth.Scopes != nil || server.Auth.Resource != "" || server.Auth.ClientID != "" || server.Auth.Schema != "" {
					return false, errors.New("connector package: invalid oauth provider")
				}
				break
			}
			if server.Transport != "streamable-http" || server.Auth.Scopes == nil || !nonEmptyStrings(*server.Auth.Scopes) || server.Auth.Schema != "" {
				return false, errors.New("connector package: invalid oauth auth")
			}
			if server.Auth.Resource != "" && (!validHTTPSURL(server.Auth.Resource) || !sameHTTPSOrigin(server.URL, server.Auth.Resource)) {
				return false, errors.New("connector package: invalid oauth resource")
			}
		case "token":
			if server.Transport != "streamable-http" || server.Auth.Provider != "" || server.Auth.Schema != "token-schema.json" || server.Auth.Scopes != nil || server.Auth.Resource != "" || server.Auth.ClientID != "" {
				return false, errors.New("connector package: invalid token auth")
			}
			tokenReferenced = true
		default:
			return false, errors.New("connector package: unsupported MCP auth")
		}
	}
	return tokenReferenced, nil
}

func validateStaticHeaders(headers map[string]string) error {
	for name, value := range headers {
		normalized := strings.ToLower(name)
		if !httpguts.ValidHeaderFieldName(name) || containsLineBreakOrNUL(value) || normalized == "authorization" || normalized == "proxy-authorization" || normalized == "cookie" || normalized == "set-cookie" || strings.Contains(normalized, "token") || strings.Contains(normalized, "secret") || strings.Contains(normalized, "api-key") {
			return errors.New("connector package: invalid static header")
		}
	}
	return nil
}

func validateTokenSchema(token TokenSchema) error {
	if token.SchemaVersion != TokenSchemaVersion || !nonEmpty(token.DocumentationLabel) || !validPublicHTTPSURL(token.DocumentationURL) || len(token.Fields) == 0 || len(token.Fields) > 8 || len(token.Headers) == 0 {
		return errors.New("connector package: invalid token schema")
	}
	fields := map[string]struct{}{}
	for _, field := range token.Fields {
		if !tokenFieldPattern.MatchString(field.ID) || !nonEmpty(field.Label) || field.Type != "text" && field.Type != "secret" {
			return errors.New("connector package: invalid token field")
		}
		if _, exists := fields[field.ID]; exists {
			return errors.New("connector package: duplicate token field")
		}
		fields[field.ID] = struct{}{}
	}
	headerNames := map[string]struct{}{}
	for _, header := range token.Headers {
		name := strings.ToLower(header.Name)
		if !httpguts.ValidHeaderFieldName(header.Name) || containsLineBreakOrNUL(header.Template) || name == "cookie" || name == "set-cookie" {
			return errors.New("connector package: invalid token header")
		}
		if _, exists := headerNames[name]; exists {
			return errors.New("connector package: duplicate token header")
		}
		headerNames[name] = struct{}{}
		matches := templateField.FindAllStringSubmatch(header.Template, -1)
		if len(matches) == 0 || strings.ContainsAny(templateField.ReplaceAllString(header.Template, ""), "{}") {
			return errors.New("connector package: invalid token header template")
		}
		for _, match := range matches {
			if _, exists := fields[match[1]]; !exists {
				return errors.New("connector package: unknown token header field")
			}
		}
	}
	return nil
}

func allowedPlatform(value string) bool {
	return value == "windows" || value == "macos" || value == "linux"
}
func allowedArchitecture(value string) bool { return value == "x64" || value == "arm64" }
func validHTTPSURL(value string) bool {
	return validPublicHTTPSURL(value)
}
func sameHTTPSOrigin(left, right string) bool {
	leftURL, leftErr := url.Parse(left)
	rightURL, rightErr := url.Parse(right)
	return leftErr == nil && rightErr == nil &&
		strings.EqualFold(leftURL.Scheme, rightURL.Scheme) && strings.EqualFold(leftURL.Host, rightURL.Host)
}
func validPublicHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !ip.IsMulticast()
	}
	return true
}
func validMCPHTTPSURL(value string) bool {
	if !validPublicHTTPSURL(value) || strings.ContainsAny(value, "{}") {
		return false
	}
	parsed, _ := url.Parse(value)
	if parsed.Fragment != "" {
		return false
	}
	for name := range parsed.Query() {
		normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		if strings.Contains(normalized, "token") || strings.Contains(normalized, "secret") || strings.Contains(normalized, "api-key") || strings.Contains(normalized, "authorization") || strings.Contains(normalized, "credential") || strings.Contains(normalized, "password") {
			return false
		}
	}
	return true
}

// IsPublicMCPHTTPSURL is the shared Desktop runtime guard for URLs that have
// already crossed the package parser.
func IsPublicMCPHTTPSURL(value string) bool { return validMCPHTTPSURL(value) }
func nonEmpty(value string) bool            { return strings.TrimSpace(value) != "" }
func nonEmptyStrings(values []string) bool {
	for _, value := range values {
		if !nonEmpty(value) {
			return false
		}
	}
	return true
}
func containsLineBreakOrNUL(value string) bool { return strings.ContainsAny(value, "\r\n\x00") }
func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
func isASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

// MarshalManifest is used by focused fixtures without adding a second parser.
func MarshalManifest(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}
