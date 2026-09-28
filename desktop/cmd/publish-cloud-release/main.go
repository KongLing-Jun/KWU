// Command publish-cloud-release publishes only the selected stable desktop
// release rows after immutable GitHub and OSS assets are live. It never uploads
// artifacts and does not modify the cloud repository.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"kwu/desktop/internal/update"
)

const (
	cloudReleaseTokenEnv = "KWU_CLOUD_RELEASE_TOKEN"
	maxReleaseNotesBytes = 64 * 1024
)

var (
	versionPattern   = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	sha256Pattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	markdownImageRE  = regexp.MustCompile(`!\[[^\]]*\]\s*(?:\(|\[)`)
	htmlTagRE        = regexp.MustCompile(`(?i)<!--|<\/?[a-z][^>]*>`)
	readReleaseToken = terminalReleaseToken
)

type commandOptions struct {
	ManifestPath string
	DMGPath      string
	NotesPath    string
	BaseURL      string
	Target       string
	DryRun       bool
	Preflight    bool
}

type releaseRecord struct {
	Platform      string `json:"platform"`
	Architecture  string `json:"architecture"`
	VersionName   string `json:"version_name"`
	VersionCode   int64  `json:"version_code"`
	DownloadURL   string `json:"download_url"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	SHA256        string `json:"sha256"`
	ReleaseNotes  string `json:"release_notes"`
}

type releaseEnvelope struct {
	Data struct {
		Release releaseRecord `json:"release"`
	} `json:"data"`
}

type historyEnvelope struct {
	Data *struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor json.RawMessage   `json:"next_cursor"`
		HasMore    *bool             `json:"has_more"`
	} `json:"data"`
}

type httpStatusError struct {
	statusCode int
}

func (err *httpStatusError) Error() string {
	if err.statusCode == http.StatusConflict {
		return "HTTP 409 Conflict; refusing to modify the existing release"
	}
	return fmt.Sprintf("HTTP %d: %s", err.statusCode, http.StatusText(err.statusCode))
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "publish-cloud-release:", err)
		fmt.Fprintln(os.Stderr, "No artifact was rolled back. Correct the error and rerun the same command.")
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("publish-cloud-release", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commandOptions
	flags.StringVar(&options.ManifestPath, "manifest", "", "OSS latest.json path")
	flags.StringVar(&options.DMGPath, "dmg", "", "macOS universal DMG path")
	flags.StringVar(&options.NotesPath, "notes", "", "approved Markdown release notes path")
	flags.StringVar(&options.BaseURL, "base-url", "", "cloud API origin")
	flags.StringVar(&options.Target, "target", "all", "release target: windows-amd64, windows-arm64, macos-universal, or all")
	flags.BoolVar(&options.DryRun, "dry-run", false, "validate and print without network writes")
	flags.BoolVar(&options.Preflight, "preflight", false, "validate cloud deployment and administrator access without publishing")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if strings.TrimSpace(options.BaseURL) == "" {
		return fmt.Errorf("-base-url is required")
	}
	baseURL, err := validateCloudBaseURL(options.BaseURL)
	if err != nil {
		return err
	}
	if options.Preflight {
		if options.DryRun {
			return fmt.Errorf("-preflight cannot be combined with -dry-run")
		}
		token, err := cloudReleaseToken()
		if err != nil {
			return err
		}
		client := &http.Client{Timeout: 15 * time.Second}
		if err := preflightCloudRelease(context.Background(), client, baseURL, token); err != nil {
			return err
		}
		if strings.TrimSpace(options.ManifestPath) != "" {
			if strings.TrimSpace(options.NotesPath) == "" {
				return fmt.Errorf("-notes is required when -preflight includes -manifest")
			}
			switch options.Target {
			case "windows-amd64", "windows-arm64", "macos-universal", "all":
			default:
				return fmt.Errorf("-target must be windows-amd64, windows-arm64, macos-universal, or all")
			}
			records, err := buildReleaseRecordsForTarget(
				options.ManifestPath, options.DMGPath, options.NotesPath, options.Target,
			)
			if err != nil {
				return err
			}
			if err := preflightReleaseRecords(context.Background(), client, baseURL, records); err != nil {
				return err
			}
		}
		fmt.Fprintln(output, "cloud release API preflight passed")
		return nil
	}
	if strings.TrimSpace(options.ManifestPath) == "" || strings.TrimSpace(options.NotesPath) == "" {
		return fmt.Errorf("-manifest and -notes are required")
	}
	switch options.Target {
	case "windows-amd64", "windows-arm64", "macos-universal", "all":
	default:
		return fmt.Errorf("-target must be windows-amd64, windows-arm64, macos-universal, or all")
	}
	records, err := buildReleaseRecordsForTarget(options.ManifestPath, options.DMGPath, options.NotesPath, options.Target)
	if err != nil {
		return err
	}
	if options.DryRun {
		for _, record := range records {
			fmt.Fprintf(output, "dry-run: %s/%s %s (%d bytes) -> %s\n", record.Platform, record.Architecture, record.VersionName, record.FileSizeBytes, record.DownloadURL)
		}
		return nil
	}
	token, err := cloudReleaseToken()
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	if err := preflightCloudRelease(context.Background(), client, baseURL, token); err != nil {
		return err
	}
	if err := publishReleaseRecords(context.Background(), client, baseURL, token, records); err != nil {
		return err
	}
	for _, record := range records {
		fmt.Fprintf(output, "published and verified: %s/%s %s\n", record.Platform, record.Architecture, record.VersionName)
	}
	return nil
}

func buildReleaseRecords(manifestPath, dmgPath, notesPath string) ([]releaseRecord, error) {
	return buildReleaseRecordsForTarget(manifestPath, dmgPath, notesPath, "all")
}

func buildReleaseRecordsForTarget(manifestPath, dmgPath, notesPath, target string) ([]releaseRecord, error) {
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read OSS manifest: %w", err)
	}
	var manifest update.Manifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		return nil, fmt.Errorf("decode OSS manifest: %w", err)
	}
	versionName, versionCode, err := releaseVersion(manifest.Version)
	if err != nil {
		return nil, err
	}
	notesBytes, err := os.ReadFile(notesPath)
	if err != nil {
		return nil, fmt.Errorf("read release notes: %w", err)
	}
	notes, err := validateReleaseNotes(string(notesBytes))
	if err != nil {
		return nil, err
	}

	buildTarget := func(selected string) ([]releaseRecord, error) {
		switch selected {
		case "windows-amd64":
			record, err := manifestReleaseRecord(manifest, "windows-amd64", "windows", "amd64", versionName, versionCode, notes)
			return []releaseRecord{record}, err
		case "windows-arm64":
			record, err := manifestReleaseRecord(manifest, "windows-arm64", "windows", "arm64", versionName, versionCode, notes)
			return []releaseRecord{record}, err
		case "macos-universal":
			amd64, err := manifestReleaseRecord(manifest, "darwin-amd64", "macos", "amd64", versionName, versionCode, notes)
			if err != nil {
				return nil, err
			}
			arm64, err := manifestReleaseRecord(manifest, "darwin-arm64", "macos", "arm64", versionName, versionCode, notes)
			if err != nil {
				return nil, err
			}
			if amd64.DownloadURL != arm64.DownloadURL || amd64.FileSizeBytes != arm64.FileSizeBytes || amd64.SHA256 != arm64.SHA256 {
				return nil, fmt.Errorf("macos-universal manifest entries must identify the same DMG")
			}
			return []releaseRecord{amd64, arm64}, nil
		default:
			return nil, fmt.Errorf("unknown release target %q", selected)
		}
	}
	if target != "all" {
		return buildTarget(target)
	}
	var records []releaseRecord
	for _, selected := range []string{"windows-amd64", "windows-arm64", "macos-universal"} {
		targetRecords, err := buildTarget(selected)
		if err != nil {
			return nil, err
		}
		records = append(records, targetRecords...)
	}
	return records, nil
}

func releaseVersion(raw string) (string, int64, error) {
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if !versionPattern.MatchString(name) {
		return "", 0, fmt.Errorf("version must be stable X.Y.Z or vX.Y.Z")
	}
	parts := strings.Split(name, ".")
	values := make([]int64, len(parts))
	for i, part := range parts {
		value, err := strconv.ParseInt(part, 10, 64)
		if err != nil || value < 0 || value > 999 {
			return "", 0, fmt.Errorf("version segment %q must be between 0 and 999", part)
		}
		values[i] = value
	}
	code := values[0]*1_000_000 + values[1]*1_000 + values[2]
	if code <= 0 {
		return "", 0, fmt.Errorf("version_code must be greater than zero")
	}
	return name, code, nil
}

func validateReleaseNotes(raw string) (string, error) {
	notes := strings.TrimSpace(raw)
	if notes == "" {
		return "", fmt.Errorf("release notes are empty")
	}
	if len([]byte(notes)) > maxReleaseNotesBytes {
		return "", fmt.Errorf("release notes exceed %d bytes", maxReleaseNotesBytes)
	}
	if markdownImageRE.MatchString(notes) {
		return "", fmt.Errorf("release notes images are not supported")
	}
	if htmlTagRE.MatchString(notes) {
		return "", fmt.Errorf("release notes HTML is not supported")
	}
	return notes, nil
}

func manifestReleaseRecord(manifest update.Manifest, key, platform, architecture, versionName string, versionCode int64, notes string) (releaseRecord, error) {
	asset, ok := manifest.Platforms[key]
	if !ok {
		return releaseRecord{}, fmt.Errorf("OSS manifest is missing %s", key)
	}
	normalizedSHA := strings.ToLower(strings.TrimSpace(asset.SHA256))
	if asset.Size <= 0 || !sha256Pattern.MatchString(normalizedSHA) {
		return releaseRecord{}, fmt.Errorf("OSS manifest %s has invalid size or SHA256", key)
	}
	downloadURL, err := validateArtifactURL(asset.URL)
	if err != nil {
		return releaseRecord{}, fmt.Errorf("OSS manifest %s URL: %w", key, err)
	}
	expectedName := "Clouvia-" + key + "-installer.exe"
	if strings.HasPrefix(key, "darwin-") {
		expectedName = "Clouvia-darwin-universal.dmg"
	}
	parsed, _ := url.Parse(downloadURL)
	if path.Base(parsed.Path) != expectedName {
		return releaseRecord{}, fmt.Errorf("OSS manifest %s must point to %s", key, expectedName)
	}
	return releaseRecord{
		Platform: platform, Architecture: architecture, VersionName: versionName, VersionCode: versionCode,
		DownloadURL: downloadURL, FileSizeBytes: asset.Size, SHA256: normalizedSHA, ReleaseNotes: notes,
	}, nil
}

func siblingArtifactURL(first, second, filename string) (string, error) {
	left, err := url.Parse(first)
	if err != nil {
		return "", err
	}
	right, err := url.Parse(second)
	if err != nil {
		return "", err
	}
	leftDir := path.Dir(left.Path)
	if left.Scheme != right.Scheme || left.Host != right.Host || leftDir != path.Dir(right.Path) {
		return "", fmt.Errorf("Windows OSS assets do not share one version directory")
	}
	left.Path = path.Join(leftDir, filename)
	left.RawPath = ""
	left.RawQuery = ""
	left.Fragment = ""
	return left.String(), nil
}

func validateArtifactURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("must be an absolute HTTPS URL without credentials")
	}
	return value, nil
}

func validateCloudBaseURL(raw string) (string, error) {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("cloud base URL is invalid")
	}
	if parsed.Scheme == "https" {
		return value, nil
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if parsed.Scheme == "http" && (strings.EqualFold(host, "localhost") || (ip != nil && (ip.IsLoopback() || ip.IsPrivate()))) {
		return value, nil
	}
	return "", fmt.Errorf("cloud base URL must use HTTPS unless it is a private integration endpoint")
}

func hashFile(filePath string) (int64, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	if size <= 0 {
		return 0, "", fmt.Errorf("file is empty")
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func cloudReleaseToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv(cloudReleaseTokenEnv)); token != "" {
		return token, nil
	}
	token, err := readReleaseToken()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("%s is empty", cloudReleaseTokenEnv)
	}
	return strings.TrimSpace(token), nil
}

func terminalReleaseToken() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("%s is empty and stdin is not a terminal", cloudReleaseTokenEnv)
	}
	fmt.Fprint(os.Stderr, "Cloud administrator access token: ")
	token, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read cloud release token: %w", err)
	}
	return string(token), nil
}

func publishReleaseRecords(ctx context.Context, client *http.Client, baseURL, token string, records []releaseRecord) error {
	if err := preflightReleaseRecords(ctx, client, baseURL, records); err != nil {
		return err
	}
	for _, record := range records {
		latest, err := fetchLatestRelease(ctx, client, baseURL, record.Platform, record.Architecture)
		switch {
		case err == nil:
			if latest.Platform != record.Platform {
				return fmt.Errorf("inspect latest %s/%s: latest release does not match platform", record.Platform, record.Architecture)
			}
			if latest.Architecture != record.Architecture {
				return fmt.Errorf("inspect latest %s/%s: latest release does not match architecture", record.Platform, record.Architecture)
			}
			if latest.VersionCode > record.VersionCode {
				return fmt.Errorf("publish %s/%s: latest version_code %d is newer than requested version_code %d", record.Platform, record.Architecture, latest.VersionCode, record.VersionCode)
			}
			if latest.VersionCode == record.VersionCode || latest.VersionName == record.VersionName {
				if err := verifyReleaseRecord(latest, record); err != nil {
					return fmt.Errorf("publish %s/%s: latest release has the same version identity but %w; refusing to modify it", record.Platform, record.Architecture, err)
				}
				continue
			}
		case isHTTPStatus(err, http.StatusNotFound):
			// No release exists for this target yet; publishing may proceed.
		case err != nil:
			return fmt.Errorf("inspect latest %s/%s: %w", record.Platform, record.Architecture, err)
		}
		if err := publishReleaseRecord(ctx, client, baseURL, token, record); err != nil {
			return fmt.Errorf("publish %s/%s: %w", record.Platform, record.Architecture, err)
		}
		latest, err = fetchLatestRelease(ctx, client, baseURL, record.Platform, record.Architecture)
		if err != nil {
			return fmt.Errorf("verify %s/%s: %w", record.Platform, record.Architecture, err)
		}
		if err := verifyReleaseRecord(latest, record); err != nil {
			return fmt.Errorf("verify %s/%s: latest release %w", record.Platform, record.Architecture, err)
		}
	}
	return nil
}

func preflightReleaseRecords(ctx context.Context, client *http.Client, baseURL string, records []releaseRecord) error {
	for _, record := range records {
		latest, err := fetchLatestRelease(ctx, client, baseURL, record.Platform, record.Architecture)
		switch {
		case err == nil:
			if latest.Platform != record.Platform {
				return fmt.Errorf("inspect latest %s/%s: latest release does not match platform", record.Platform, record.Architecture)
			}
			if latest.Architecture != record.Architecture {
				return fmt.Errorf("inspect latest %s/%s: latest release does not match architecture", record.Platform, record.Architecture)
			}
			if latest.VersionCode > record.VersionCode {
				return fmt.Errorf("publish %s/%s: latest version_code %d is newer than requested version_code %d", record.Platform, record.Architecture, latest.VersionCode, record.VersionCode)
			}
			if latest.VersionCode == record.VersionCode || latest.VersionName == record.VersionName {
				if err := verifyReleaseRecord(latest, record); err != nil {
					return fmt.Errorf("publish %s/%s: latest release has the same version identity but %w; refusing to modify it", record.Platform, record.Architecture, err)
				}
			}
		case isHTTPStatus(err, http.StatusNotFound):
			// No release exists for this target yet; publishing may continue.
		case err != nil:
			return fmt.Errorf("inspect latest %s/%s: %w", record.Platform, record.Architecture, err)
		}
	}
	return nil
}

func preflightCloudRelease(ctx context.Context, client *http.Client, baseURL, token string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/admin/client-releases/history?limit=1", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	var envelope historyEnvelope
	if err := doJSON(client, request, &envelope); err != nil {
		return fmt.Errorf("cloud release API preflight: %w", err)
	}
	if envelope.Data == nil || envelope.Data.Items == nil || envelope.Data.HasMore == nil || len(envelope.Data.NextCursor) == 0 {
		return fmt.Errorf("cloud release API preflight: history response does not match the expected schema")
	}
	return nil
}

func publishReleaseRecord(ctx context.Context, client *http.Client, baseURL, token string, record releaseRecord) error {
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/admin/client-releases", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	var envelope releaseEnvelope
	if err := doJSON(client, request, &envelope); err != nil {
		return err
	}
	if err := verifyReleaseRecord(envelope.Data.Release, record); err != nil {
		return fmt.Errorf("publish response %w", err)
	}
	return nil
}

func verifyReleaseRecord(actual, expected releaseRecord) error {
	switch {
	case actual.Platform != expected.Platform:
		return fmt.Errorf("does not match platform")
	case actual.Architecture != expected.Architecture:
		return fmt.Errorf("does not match architecture")
	case actual.VersionName != expected.VersionName:
		return fmt.Errorf("does not match version_name")
	case actual.VersionCode != expected.VersionCode:
		return fmt.Errorf("does not match version_code")
	case actual.DownloadURL != expected.DownloadURL:
		return fmt.Errorf("does not match download_url")
	case actual.FileSizeBytes != expected.FileSizeBytes:
		return fmt.Errorf("does not match file_size_bytes")
	case actual.SHA256 != expected.SHA256:
		return fmt.Errorf("does not match sha256")
	case actual.ReleaseNotes != expected.ReleaseNotes:
		return fmt.Errorf("does not match release_notes")
	default:
		return nil
	}
}

func fetchLatestRelease(ctx context.Context, client *http.Client, baseURL, platform, architecture string) (releaseRecord, error) {
	endpoint, err := url.Parse(baseURL + "/api/v1/client-releases/latest")
	if err != nil {
		return releaseRecord{}, err
	}
	query := endpoint.Query()
	query.Set("platform", platform)
	query.Set("architecture", architecture)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return releaseRecord{}, err
	}
	var envelope releaseEnvelope
	if err := doJSON(client, request, &envelope); err != nil {
		return releaseRecord{}, err
	}
	return envelope.Data.Release, nil
}

func doJSON(client *http.Client, request *http.Request, output any) error {
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 128*1024))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &httpStatusError{statusCode: response.StatusCode}
	}
	if err := json.Unmarshal(body, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func isHTTPStatus(err error, statusCode int) bool {
	var statusErr *httpStatusError
	return errors.As(err, &statusErr) && statusErr.statusCode == statusCode
}
