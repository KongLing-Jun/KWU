package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	MaxPackageBytes int64 = 10 << 20
	MaxCatalogBytes int64 = 1 << 20
)

var connectorIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

type CatalogMode string

const (
	CatalogModeDevFixed CatalogMode = "dev-fixed"
	CatalogModeCloud    CatalogMode = "cloud"
)

type RuntimePolicy struct {
	OS            string
	Architecture  string
	ClientVersion string
}

// NormalizeClientVersion is the single release-version normalization boundary
// used by both the Catalog query and package compatibility checks.
func NormalizeClientVersion(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "dev" {
		return raw, nil
	}
	if raw == "" {
		return "", errors.New("empty client version")
	}
	v := raw
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return "", errors.New("invalid client version")
	}
	canonical := semver.Canonical(v)
	if canonical == "" {
		return "", errors.New("invalid client version")
	}
	return strings.TrimPrefix(canonical, "v"), nil
}

func NormalizeRuntimePolicy(policy RuntimePolicy) (RuntimePolicy, error) {
	version, err := NormalizeClientVersion(policy.ClientVersion)
	if err != nil {
		return RuntimePolicy{}, err
	}
	policy.ClientVersion = version
	return policy, nil
}

func CatalogURL(baseURL string, policy RuntimePolicy) (string, error) {
	policy, err := NormalizeRuntimePolicy(policy)
	if err != nil {
		return "", fmt.Errorf("catalog: client version: %w", err)
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("catalog: invalid cloud base URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/v1/connectors/catalog"
	query := parsed.Query()
	query.Set("platform", policy.OS)
	query.Set("architecture", policy.Architecture)
	query.Set("client_version", policy.ClientVersion)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type CatalogSource interface {
	Fetch(ctx context.Context) ([]byte, error)
}

type Catalog struct {
	Version     string
	GeneratedAt time.Time
	RequestID   string
	ServerTime  time.Time
	Items       []CatalogItem
}

type CatalogItem struct {
	ID               string
	Name             string
	Provider         string
	Description      string
	Category         string
	IconURL          string
	Examples         []string
	Kind             string
	Version          string
	MinClientVersion string
	Compatible       bool
	BlockedReason    string
	Package          CatalogPackage
	Capabilities     []string
}

type CatalogPackage struct {
	URL       string
	SizeBytes int64
	SHA256    string
}

type catalogEnvelopeWire struct {
	Data       catalogDataWire `json:"data"`
	Error      json.RawMessage `json:"error"`
	RequestID  string          `json:"request_id"`
	ServerTime string          `json:"server_time"`
}

type catalogDataWire struct {
	CatalogVersion string            `json:"catalog_version"`
	GeneratedAt    string            `json:"generated_at"`
	Items          []catalogItemWire `json:"items"`
}

type catalogItemWire struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Provider         string             `json:"provider"`
	Description      string             `json:"description"`
	Category         string             `json:"category"`
	IconURL          *string            `json:"icon_url,omitempty"`
	Examples         []string           `json:"examples"`
	Kind             string             `json:"kind"`
	Version          string             `json:"version"`
	MinClientVersion string             `json:"min_client_version"`
	Compatible       bool               `json:"compatible"`
	BlockedReason    *string            `json:"blocked_reason"`
	Package          catalogPackageWire `json:"package"`
	Capabilities     []string           `json:"capabilities"`
}

type catalogPackageWire struct {
	URL       string `json:"url"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func ParseCatalog(raw []byte, mode CatalogMode) (Catalog, error) {
	if mode != CatalogModeDevFixed && mode != CatalogModeCloud {
		return Catalog{}, fmt.Errorf("catalog: unsupported mode %q", mode)
	}
	var wire catalogEnvelopeWire
	if err := decodeStrict(raw, &wire); err != nil {
		return Catalog{}, fmt.Errorf("catalog: invalid response: %w", err)
	}
	if len(wire.Error) == 0 || string(bytes.TrimSpace(wire.Error)) != "null" {
		return Catalog{}, errors.New("catalog: success response error must be null")
	}
	if !strings.HasPrefix(wire.Data.CatalogVersion, "sha256:") || !validLowerSHA256(strings.TrimPrefix(wire.Data.CatalogVersion, "sha256:")) {
		return Catalog{}, errors.New("catalog: invalid catalog_version")
	}
	generatedAt, err := time.Parse(time.RFC3339, wire.Data.GeneratedAt)
	if err != nil {
		return Catalog{}, errors.New("catalog: invalid generated_at")
	}
	serverTime, err := time.Parse(time.RFC3339, wire.ServerTime)
	if err != nil {
		return Catalog{}, errors.New("catalog: invalid server_time")
	}
	if strings.TrimSpace(wire.RequestID) == "" {
		return Catalog{}, errors.New("catalog: missing request_id")
	}
	items := make([]CatalogItem, 0, len(wire.Data.Items))
	previousID := ""
	seen := map[string]bool{}
	for _, itemWire := range wire.Data.Items {
		item, err := parseCatalogItem(itemWire, mode)
		if err != nil {
			return Catalog{}, err
		}
		if seen[item.ID] || previousID != "" && item.ID < previousID {
			return Catalog{}, errors.New("catalog: items must be unique and sorted by id")
		}
		seen[item.ID] = true
		previousID = item.ID
		items = append(items, item)
	}
	return Catalog{Version: wire.Data.CatalogVersion, GeneratedAt: generatedAt, RequestID: wire.RequestID, ServerTime: serverTime, Items: items}, nil
}

func parseCatalogItem(w catalogItemWire, mode CatalogMode) (CatalogItem, error) {
	if !validConnectorID(w.ID) || !nonEmpty(w.Name) || !nonEmpty(w.Provider) || !nonEmpty(w.Description) || !nonEmpty(w.Category) {
		return CatalogItem{}, fmt.Errorf("catalog: invalid connector metadata for %q", w.ID)
	}
	if w.IconURL != nil && !validHTTPSURL(*w.IconURL) {
		return CatalogItem{}, fmt.Errorf("catalog: invalid icon_url for %q", w.ID)
	}
	if w.Examples == nil || len(w.Examples) > 8 || !nonEmptyStrings(w.Examples) || w.Capabilities == nil || !nonEmptyStrings(w.Capabilities) {
		return CatalogItem{}, fmt.Errorf("catalog: invalid examples or capabilities for %q", w.ID)
	}
	if w.Kind != "mcp" && w.Kind != "composite" {
		return CatalogItem{}, fmt.Errorf("catalog: invalid kind for %q", w.ID)
	}
	if !isCanonicalWireSemver(w.Version) || !isCanonicalWireSemver(w.MinClientVersion) {
		return CatalogItem{}, fmt.Errorf("catalog: non-canonical version for %q", w.ID)
	}
	blocked := ""
	if w.BlockedReason != nil {
		blocked = *w.BlockedReason
	}
	if w.Compatible {
		if w.BlockedReason != nil {
			return CatalogItem{}, fmt.Errorf("catalog: compatible item %q has blocked_reason", w.ID)
		}
	} else if blocked != "client_too_old" && blocked != "platform_unsupported" && blocked != "architecture_unsupported" {
		return CatalogItem{}, fmt.Errorf("catalog: invalid blocked_reason for %q", w.ID)
	}
	if w.Package.SizeBytes <= 0 || w.Package.SizeBytes > MaxPackageBytes || !validLowerSHA256(w.Package.SHA256) {
		return CatalogItem{}, fmt.Errorf("catalog: invalid package metadata for %q", w.ID)
	}
	if err := validatePackageURL(w.Package.URL, w.ID, w.Version, w.Package.SHA256, mode); err != nil {
		return CatalogItem{}, fmt.Errorf("catalog: package url for %q: %w", w.ID, err)
	}
	icon := ""
	if w.IconURL != nil {
		icon = *w.IconURL
	}
	return CatalogItem{
		ID: w.ID, Name: w.Name, Provider: w.Provider, Description: w.Description, Category: w.Category, IconURL: icon,
		Examples: append([]string(nil), w.Examples...), Kind: w.Kind, Version: w.Version, MinClientVersion: w.MinClientVersion,
		Compatible: w.Compatible, BlockedReason: blocked,
		Package:      CatalogPackage{URL: w.Package.URL, SizeBytes: w.Package.SizeBytes, SHA256: w.Package.SHA256},
		Capabilities: append([]string(nil), w.Capabilities...),
	}, nil
}

func validatePackageURL(raw, connectorID, version, sha string, mode CatalogMode) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("invalid absolute URL")
	}
	if mode == CatalogModeCloud {
		if u.Scheme != "https" {
			return errors.New("cloud package URL must use HTTPS")
		}
		return nil
	}
	if u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" {
		return errors.New("dev-fixed package URL must use 127.0.0.1 HTTP with an explicit port")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 || net.ParseIP(u.Hostname()) == nil {
		return errors.New("invalid dev-fixed port")
	}
	segments := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(segments) < 4 || segments[len(segments)-1] != "package.zip" || !containsString(segments, connectorID) || !containsString(segments, version) || !containsString(segments, sha) {
		return errors.New("dev-fixed URL must contain connector id, version, sha256, and package.zip")
	}
	return nil
}

func validConnectorID(value string) bool { return connectorIDPattern.MatchString(value) }

func isCanonicalWireSemver(raw string) bool {
	if strings.HasPrefix(raw, "v") || raw == "" || strings.TrimSpace(raw) != raw {
		return false
	}
	v := "v" + raw
	return semver.IsValid(v) && strings.TrimPrefix(semver.Canonical(v), "v") == raw
}

func normalizeSemver(raw string) string {
	value, _ := NormalizeClientVersion(raw)
	return value
}

func parseNormalizedSemver(raw string) (string, error) {
	if raw == "dev" {
		return "", errors.New("dev is not a release semver")
	}
	return NormalizeClientVersion(raw)
}

func compareVersions(left, right string) int { return semver.Compare("v"+left, "v"+right) }

func decodeStrict(raw []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("trailing JSON value")
}

func validLowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
