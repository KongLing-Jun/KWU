package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"golang.org/x/mod/semver"

	"kwu/desktop/internal/update"
	"kwu/internal/brand"
	"kwu/internal/config"
	"kwu/internal/netclient"
	"kwu/internal/userctx"
)

// updater.go is the transport-free core of the desktop auto-updater: manifest
// fetch, version comparison, signed download, and per-platform apply/relaunch. It
// has no Wails dependency so the logic is unit-tested directly; updater_app.go is
// the thin Wails binding that wires these into App methods and progress events.

// Stable builds poll the owned OSS release bucket first and fall back to the same
// GitHub release when either its manifest or its verified asset chain fails.
// Canary builds still have no remote update pointer.
const (
	ossReleasesBase       = "https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com"
	ghReleasesBase        = "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases"
	manifestSourceTimeout = 7 * time.Second
	cloudReleaseTimeout   = 5 * time.Second
)

type releaseSource struct {
	Name           string
	ManifestURL    string
	Target         string
	Authority      bool
	FrozenFallback bool
}

type fetchedManifest struct {
	Source   releaseSource
	Manifest *update.Manifest
}

type verifiedDownload struct {
	Source   releaseSource
	Version  string
	Manifest *update.Manifest
	Asset    update.Asset
	Data     []byte
}

type cloudReleaseMetadata struct {
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	VersionName  string `json:"version_name"`
	DownloadURL  string `json:"download_url"`
	ReleaseNotes string `json:"release_notes"`
}

type cloudReleaseEnvelope struct {
	Data struct {
		Release *cloudReleaseMetadata `json:"release"`
	} `json:"data"`
}

// releaseSources returns ordered update sources for the running build's channel.
func desktopReleaseTarget(goos, goarch string) (string, error) {
	switch {
	case goos == "windows" && goarch == "amd64":
		return "windows-amd64", nil
	case goos == "windows" && goarch == "arm64":
		return "windows-arm64", nil
	case goos == "darwin" && (goarch == "amd64" || goarch == "arm64"):
		return "macos-universal", nil
	default:
		return "", fmt.Errorf("update: unsupported desktop release target %s/%s", goos, goarch)
	}
}

func releaseSourcesFor(goos, goarch string) []releaseSource {
	if channel == "canary" {
		return nil
	}
	target, err := desktopReleaseTarget(goos, goarch)
	if err != nil {
		return nil
	}
	return []releaseSource{
		{Name: "OSS target", ManifestURL: ossReleasesBase + "/channels/stable/" + target + "/latest.json", Target: target, Authority: true},
		{Name: "OSS migration fallback", ManifestURL: ossReleasesBase + "/channels/stable/latest.json", FrozenFallback: true},
		{Name: "GitHub migration fallback", ManifestURL: ghReleasesBase + "/latest/download/latest.json", FrozenFallback: true},
	}
}

func releaseSources() []releaseSource { return releaseSourcesFor(runtime.GOOS, runtime.GOARCH) }

func releaseAuthorizationSources() []releaseSource {
	sources := releaseSources()
	if len(sources) == 0 {
		return nil
	}
	return sources[:1]
}

func githubTargetManifestSource(version, target string) (releaseSource, error) {
	normalized, ok := normalizeVersion(version)
	if !ok {
		return releaseSource{}, fmt.Errorf("update: cannot locate GitHub manifest for invalid version %q", version)
	}
	if target == "" {
		return releaseSource{}, fmt.Errorf("update: target manifest source has no release target")
	}
	tag := "desktop-" + normalized
	return releaseSource{
		Name:        "GitHub versioned target",
		ManifestURL: ghReleasesBase + "/download/" + tag + "/latest-" + target + ".json",
		Target:      target,
	}, nil
}

func sameTargetAssetIdentity(left, right update.Asset) bool {
	baseName := func(raw string) string {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return ""
		}
		return path.Base(parsed.Path)
	}
	return left.Size == right.Size &&
		strings.EqualFold(left.SHA256, right.SHA256) &&
		baseName(left.URL) != "" && baseName(left.URL) == baseName(right.URL) &&
		baseName(left.Sig) != "" && baseName(left.Sig) == baseName(right.Sig)
}

// manifestEndpoints is retained for channel-selection tests and diagnostics.
func manifestEndpoints() []string {
	sources := releaseSources()
	endpoints := make([]string, 0, len(sources))
	for _, source := range sources {
		endpoints = append(endpoints, source.ManifestURL)
	}
	return endpoints
}

// downloadPage is the human-facing releases page shown when self-update is
// unavailable (macOS) or the manifest omits its own link.
func downloadPage() string {
	if channel == "canary" {
		return ghReleasesBase // lists pre-releases too
	}
	return ghReleasesBase + "/latest"
}

func cloudReleasePlatform(goos, goarch string) (string, string, error) {
	platform := goos
	if goos == "darwin" {
		platform = "macos"
	}
	if platform != "windows" && platform != "macos" && platform != "linux" {
		return "", "", fmt.Errorf("update: unsupported cloud release platform %q", goos)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", "", fmt.Errorf("update: unsupported cloud release architecture %q", goarch)
	}
	return platform, goarch, nil
}

func fetchCloudReleaseMetadata(ctx context.Context, c *http.Client, baseURL, goos, goarch string) (*cloudReleaseMetadata, error) {
	platform, architecture, err := cloudReleasePlatform(goos, goarch)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/v1/client-releases/latest")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("platform", platform)
	query.Set("architecture", architecture)
	endpoint.RawQuery = query.Encode()

	body, err := fetchBytesOnce(ctx, c, endpoint.String())
	if err != nil {
		return nil, err
	}
	var envelope cloudReleaseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("update: decode cloud release metadata: %w", err)
	}
	if envelope.Data.Release == nil || strings.TrimSpace(envelope.Data.Release.VersionName) == "" {
		return nil, fmt.Errorf("update: cloud release response has no release")
	}
	return envelope.Data.Release, nil
}

func sameReleaseVersion(left, right string) bool {
	trim := func(value string) string {
		value = strings.TrimSpace(value)
		return strings.TrimPrefix(value, "v")
	}
	return trim(left) != "" && trim(left) == trim(right)
}

func releaseNotes(manifestNotes, manifestVersion string, release *cloudReleaseMetadata) string {
	if release != nil && sameReleaseVersion(manifestVersion, release.VersionName) {
		if notes := strings.TrimSpace(release.ReleaseNotes); notes != "" {
			return notes
		}
	}
	return strings.TrimSpace(manifestNotes)
}

func enrichUpdateInfoFromCloud(ctx context.Context, c *http.Client, baseURL, goos, goarch string, info *UpdateInfo) {
	if info == nil || !info.Available {
		return
	}
	release, err := fetchCloudReleaseMetadata(ctx, c, baseURL, goos, goarch)
	if err != nil {
		return
	}
	info.Notes = releaseNotes(info.Notes, info.Latest, release)
}

func safeUpdateDownloadURL(candidate string) string {
	if !isSafeUpdateDownloadURL(candidate) {
		return downloadPage()
	}
	parsed, _ := url.Parse(strings.TrimSpace(candidate))
	return parsed.String()
}

func isSafeUpdateDownloadURL(candidate string) bool {
	parsed, err := url.Parse(strings.TrimSpace(candidate))
	return err == nil && parsed.IsAbs() && strings.EqualFold(parsed.Scheme, "https") && parsed.Host != "" && parsed.User == nil
}

// UpdateInfo is the CheckUpdate result that drives the frontend's update banner.
type UpdateInfo struct {
	Available     bool   `json:"available"`
	Current       string `json:"current"`
	Latest        string `json:"latest"`
	Notes         string `json:"notes"`
	Channel       string `json:"channel"`
	CanSelfUpdate bool   `json:"canSelfUpdate"`
	ManualOnly    bool   `json:"manualOnly,omitempty"`
	ManualReason  string `json:"manualReason,omitempty"`
	ManualKind    string `json:"manualKind,omitempty"`
	Downloaded    bool   `json:"downloaded"`
	DownloadURL   string `json:"downloadUrl"`   // human-facing releases page (macOS path / fallback link)
	AssetSize     int64  `json:"assetSize"`     // running platform's artifact size, for the progress bar
	Err           string `json:"err,omitempty"` // set when the check itself failed
}

// UpdateDownloadResult is returned after an artifact has been downloaded,
// verified, and stored in the local updater cache.
type UpdateDownloadResult struct {
	Version string `json:"version"`
	Channel string `json:"channel"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
}

// updateProgress is the payload of the "updater:progress" Wails event emitted
// throughout DownloadUpdate / InstallUpdate.
type updateProgress struct {
	Phase    string `json:"phase"` // downloading | verifying | downloaded | installing | done | error
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Err      string `json:"err,omitempty"`
}

func httpClient() (*http.Client, error) { return newHTTPClient(false) }

// httpClientIPv4 pins the dialer to IPv4 鈥?the download fallback when the default
// (often IPv6-first) route to Cloudflare keeps resetting mid-transfer.
func httpClientIPv4() (*http.Client, error) { return newHTTPClient(true) }

func newHTTPClient(forceIPv4 bool) (*http.Client, error) {
	// Proxy settings do not require a provider credential. Keep updater,
	// telemetry, metrics, and crash transport from hydrating a BYOK key.
	cfg, err := config.LoadForRootMetadataWithActor(".", userctx.Default("."))
	if err != nil {
		return nil, err
	}
	return netclient.NewHTTPClient(cfg.OutboundProxySpec(), netclient.TransportOptions{ForceIPv4: forceIPv4})
}

type updatePolicy struct {
	CanSelfUpdate bool
	ManualKind    string
}

// reconcileUpdateAvailability requires a concrete signed manifest asset before
// an update is shown. Cloud metadata can enrich release notes and a manual-only
// download link, but can never select or authorize an auto-update asset.
func reconcileUpdateAvailability(info *UpdateInfo, _ string, hasManifestAsset bool, release *cloudReleaseMetadata) {
	if info == nil || !info.Available {
		return
	}
	if !hasManifestAsset {
		info.Available = false
		return
	}
	info.Notes = releaseNotes(info.Notes, info.Latest, release)
	if info.ManualOnly && release != nil && sameReleaseVersion(info.Latest, release.VersionName) {
		if candidate := strings.TrimSpace(release.DownloadURL); isSafeUpdateDownloadURL(candidate) {
			info.DownloadURL = candidate
		}
	}
}

// updatePolicyFor keeps unsupported and portable installs on the manual path.
// Only a registered Windows installation with a matching signed installer asset
// can update in place. Linux and macOS remain manual downloads for now.
func updatePolicyFor(goos string, registeredWindowsInstall, hasAsset bool) updatePolicy {
	switch goos {
	case "windows":
		if !registeredWindowsInstall {
			return updatePolicy{ManualKind: "windows-portable"}
		}
		if !hasAsset {
			return updatePolicy{}
		}
		return updatePolicy{CanSelfUpdate: true}
	case "darwin":
		return updatePolicy{ManualKind: "macos"}
	case "linux":
		return updatePolicy{ManualKind: "linux"}
	default:
		return updatePolicy{}
	}
}

func currentUpdatePolicy(hasAsset bool) updatePolicy {
	return updatePolicyFor(runtime.GOOS, runningFromRegisteredInstall(), hasAsset)
}

func canSelfUpdate() bool {
	return currentUpdatePolicy(true).CanSelfUpdate
}

func manualUpdateKind() string {
	return currentUpdatePolicy(true).ManualKind
}

// normalizeVersion canonicalizes a version to semver "vX.Y.Z". It reports ok=false
// for the un-injected "dev" build (and anything not valid semver), so a dev build
// never prompts to update.
func normalizeVersion(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" {
		return "", false
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return "", false
	}
	return semver.Canonical(v), true
}

func fetchManifestFromSource(ctx context.Context, c *http.Client, source releaseSource, timeout time.Duration) (*update.Manifest, error) {
	sourceCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	b, err := fetchBytes(sourceCtx, c, source.ManifestURL)
	if err != nil {
		return nil, err
	}
	var manifest update.Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if _, ok := normalizeVersion(manifest.Version); !ok {
		return nil, fmt.Errorf("manifest has no valid version")
	}
	return &manifest, nil
}

func fetchManifestFromSources(ctx context.Context, c *http.Client, sources []releaseSource, timeout time.Duration) (*fetchedManifest, error) {
	var failures []string
	if len(sources) == 0 {
		return nil, fmt.Errorf("update: no manifest endpoint configured for %s channel", channel)
	}
	for _, source := range sources {
		manifest, err := fetchManifestFromSource(ctx, c, source, timeout)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s manifest: %v", source.Name, err))
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		return &fetchedManifest{Source: source, Manifest: manifest}, nil
	}
	return nil, fmt.Errorf("update: fetch manifest failed: %s", strings.Join(failures, "; "))
}

// fetchManifest pulls the first valid latest.json with an independent deadline
// for each source, so an OSS timeout cannot consume GitHub's fallback budget.
func fetchManifest(ctx context.Context, c *http.Client) (*update.Manifest, error) {
	fetched, err := fetchManifestFromSources(ctx, c, releaseSources(), manifestSourceTimeout)
	if err != nil {
		return nil, err
	}
	return fetched.Manifest, nil
}

func downloadUpdateFromSources(
	ctx context.Context,
	c *http.Client,
	sources []releaseSource,
	timeout time.Duration,
	verify func(context.Context, update.Asset) ([]byte, error),
) (*verifiedDownload, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("update: no manifest endpoint configured for %s channel", channel)
	}
	var targetVersion string
	var authoritativeAsset *update.Asset
	var failures []string
	for _, source := range sources {
		manifest, err := fetchManifestFromSource(ctx, c, source, timeout)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s manifest: %v", source.Name, err))
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if targetVersion == "" {
			targetVersion = manifest.Version
		} else if !sameReleaseVersion(targetVersion, manifest.Version) {
			failures = append(failures, fmt.Sprintf("%s manifest version: got %s, want %s", source.Name, manifest.Version, targetVersion))
			continue
		}
		asset, ok := manifest.Asset()
		if !ok {
			failures = append(failures, fmt.Sprintf("%s asset: no update artifact for %s", source.Name, update.CurrentPlatform()))
			continue
		}
		if source.Authority {
			assetCopy := asset
			authoritativeAsset = &assetCopy
		} else if authoritativeAsset != nil && !sameTargetAssetIdentity(*authoritativeAsset, asset) {
			failures = append(failures, fmt.Sprintf("%s asset: identity differs from the authoritative target pointer", source.Name))
			continue
		}
		data, err := verify(ctx, asset)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s asset: %v", source.Name, err))
			if source.Authority {
				fallback, fallbackErr := githubTargetManifestSource(manifest.Version, source.Target)
				if fallbackErr == nil {
					fallbackManifest, fetchErr := fetchManifestFromSource(ctx, c, fallback, timeout)
					if fetchErr == nil {
						fallbackAsset, ok := fallbackManifest.Asset()
						if !sameReleaseVersion(manifest.Version, fallbackManifest.Version) || !ok ||
							!sameTargetAssetIdentity(fallbackAsset, asset) {
							failures = append(failures, "GitHub versioned target manifest does not match the authoritative target identity")
						} else if fallbackData, verifyErr := verify(ctx, fallbackAsset); verifyErr == nil {
							return &verifiedDownload{Source: fallback, Version: manifest.Version, Manifest: fallbackManifest, Asset: fallbackAsset, Data: fallbackData}, nil
						} else {
							failures = append(failures, fmt.Sprintf("%s asset: %v", fallback.Name, verifyErr))
						}
					} else {
						failures = append(failures, fmt.Sprintf("%s manifest: %v", fallback.Name, fetchErr))
					}
				}
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		return &verifiedDownload{Source: source, Version: targetVersion, Manifest: manifest, Asset: asset, Data: data}, nil
	}
	return nil, fmt.Errorf("update: all release sources failed: %s", strings.Join(failures, "; "))
}

// evaluate compares the running version against the manifest and builds the
// frontend-facing result. Pure (no I/O) so the comparison is unit-tested.
func evaluate(current string, m *update.Manifest) UpdateInfo {
	page := m.DownloadPage
	if page == "" {
		page = downloadPage()
	}
	asset, hasAsset := m.Asset()
	policy := currentUpdatePolicy(hasAsset)
	info := UpdateInfo{
		Current:       current,
		Latest:        m.Version,
		Notes:         strings.TrimSpace(m.Notes),
		Channel:       channel,
		CanSelfUpdate: policy.CanSelfUpdate,
		ManualOnly:    !policy.CanSelfUpdate,
		ManualKind:    policy.ManualKind,
		DownloadURL:   page,
	}
	cur, okCur := normalizeVersion(current)
	latest, okLatest := normalizeVersion(m.Version)
	if !okLatest {
		info.Err = "manifest has no valid version"
		return info
	}
	// A dev/invalid running version never auto-prompts.
	if okCur && semver.Compare(latest, cur) > 0 {
		info.Available = true
	}
	if hasAsset {
		info.AssetSize = asset.Size
		info.Downloaded = cachedUpdateMatches(m.Version, asset)
	}
	return info
}

type cachedUpdate struct {
	Version      string `json:"version"`
	Channel      string `json:"channel"`
	Platform     string `json:"platform"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	DownloadedAt string `json:"downloadedAt"`
}

var updateCacheBaseDir = defaultUpdateCacheBaseDir

func defaultUpdateCacheBaseDir() (string, error) {
	if base := config.CacheDir(); base != "" {
		return filepath.Join(base, "updates"), nil
	}
	return filepath.Join(os.TempDir(), brand.EnglishName, "updates"), nil
}

func updateCacheDir() (string, error) {
	dir, err := updateCacheBaseDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func updateMetadataPath() (string, error) {
	dir, err := updateCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "downloaded.json"), nil
}

func assetFileName(asset update.Asset, version string) string {
	if u, err := url.Parse(asset.URL); err == nil {
		if base := filepath.Base(u.Path); base != "." && base != "/" {
			return base
		}
	}
	clean := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-").Replace(version)
	return brand.DisplayName + "-" + clean + "-" + update.CurrentPlatform() + ".update"
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func saveCachedUpdate(version string, asset update.Asset, data []byte) (*cachedUpdate, error) {
	if err := checkSHA256(data, asset.SHA256); err != nil {
		return nil, err
	}
	dir, err := updateCacheDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, assetFileName(asset, version))
	if err := writeAtomic(path, data, 0o600); err != nil {
		return nil, err
	}
	meta := &cachedUpdate{
		Version:      version,
		Channel:      channel,
		Platform:     update.CurrentPlatform(),
		Path:         path,
		Size:         int64(len(data)),
		SHA256:       asset.SHA256,
		DownloadedAt: time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	metadataPath, err := updateMetadataPath()
	if err != nil {
		return nil, err
	}
	if err := writeAtomic(metadataPath, append(raw, '\n'), 0o600); err != nil {
		return nil, err
	}
	return meta, nil
}

func loadCachedUpdate() (*cachedUpdate, error) {
	path, err := updateMetadataPath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta cachedUpdate
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	if meta.Version == "" || meta.Channel == "" || meta.Platform == "" || meta.Path == "" || meta.SHA256 == "" {
		return nil, fmt.Errorf("update: cached metadata is incomplete")
	}
	return &meta, nil
}

func cachedUpdateMatches(version string, asset update.Asset) bool {
	meta, err := loadCachedUpdate()
	if err != nil {
		return false
	}
	return sameReleaseVersion(meta.Version, version) &&
		meta.Channel == channel &&
		meta.Platform == update.CurrentPlatform() &&
		strings.EqualFold(meta.SHA256, asset.SHA256) &&
		meta.Size == asset.Size &&
		fileSHA256Matches(meta.Path, meta.SHA256)
}

func fileSHA256Matches(path, want string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), want)
}

func readVerifiedCachedUpdate() (*cachedUpdate, []byte, error) {
	meta, err := loadCachedUpdate()
	if err != nil {
		return nil, nil, err
	}
	if meta.Channel != channel {
		return nil, nil, fmt.Errorf("update: cached update is for %s channel, current channel is %s", meta.Channel, channel)
	}
	if meta.Platform != update.CurrentPlatform() {
		return nil, nil, fmt.Errorf("update: cached update is for %s, current platform is %s", meta.Platform, update.CurrentPlatform())
	}
	data, err := os.ReadFile(meta.Path)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSHA256(data, meta.SHA256); err != nil {
		return nil, nil, err
	}
	return meta, data, nil
}

type stableAuthorizationStatus uint8

const (
	stableAuthorizationAuthorized stableAuthorizationStatus = iota + 1
	stableAuthorizationUnavailable
	stableAuthorizationRejected
)

type stableSourceFailure struct {
	Source releaseSource
	Err    error
}

type stableAuthorizationResult struct {
	Status   stableAuthorizationStatus
	Source   releaseSource
	Err      error
	Failures []stableSourceFailure
}

func (r stableAuthorizationResult) failure() error {
	if r.Err != nil {
		return r.Err
	}
	if len(r.Failures) == 0 {
		return fmt.Errorf("更新：稳定版安装授权暂时不可用")
	}
	failures := make([]string, 0, len(r.Failures))
	for _, failure := range r.Failures {
		failures = append(failures, fmt.Sprintf("来源 %s 的更新清单：%v", failure.Source.Name, failure.Err))
	}
	return fmt.Errorf("更新：稳定版安装授权暂时不可用：%s", strings.Join(failures, "；"))
}

// validateCachedUpdateAgainstStable processes the supplied sources in order:
// unavailable sources fall through, while the first parsed manifest either
// authorizes the exact cache or rejects it. Production installation passes only
// releaseAuthorizationSources(), whose sole authority is the running target's
// OSS pointer; frozen global and GitHub fallbacks can never authorize install.
// Unavailable authorization fails closed without installation, and the caller's
// state machine decides to retain the cache for a later retry.
func validateCachedUpdateAgainstStable(
	ctx context.Context,
	c *http.Client,
	sources []releaseSource,
	timeout time.Duration,
	meta *cachedUpdate,
) stableAuthorizationResult {
	if meta == nil {
		return stableAuthorizationResult{
			Status: stableAuthorizationRejected,
			Err:    fmt.Errorf("update: cached metadata is missing"),
		}
	}
	if len(sources) == 0 {
		return stableAuthorizationResult{
			Status: stableAuthorizationUnavailable,
			Err:    fmt.Errorf("update: no stable manifest endpoint configured for %s channel", channel),
		}
	}
	failures := make([]stableSourceFailure, 0, len(sources))
	for _, source := range sources {
		manifest, err := fetchManifestFromSource(ctx, c, source, timeout)
		if err != nil {
			failures = append(failures, stableSourceFailure{Source: source, Err: err})
			if ctx.Err() != nil {
				return stableAuthorizationResult{
					Status: stableAuthorizationUnavailable, Source: source, Failures: failures,
				}
			}
			continue
		}
		asset, ok := manifest.Asset()
		if !ok {
			return stableAuthorizationResult{
				Status: stableAuthorizationRejected, Source: source,
				Err: fmt.Errorf("update: %s stable manifest has no asset for %s", source.Name, update.CurrentPlatform()),
			}
		}
		if !sameReleaseVersion(meta.Version, manifest.Version) {
			return stableAuthorizationResult{
				Status: stableAuthorizationRejected, Source: source,
				Err: fmt.Errorf("update: %s stable version is %s, cached version is %s", source.Name, manifest.Version, meta.Version),
			}
		}
		if !strings.EqualFold(meta.SHA256, asset.SHA256) {
			return stableAuthorizationResult{
				Status: stableAuthorizationRejected, Source: source,
				Err: fmt.Errorf("update: %s stable asset SHA256 no longer matches the cache", source.Name),
			}
		}
		if meta.Size != asset.Size {
			return stableAuthorizationResult{
				Status: stableAuthorizationRejected, Source: source,
				Err: fmt.Errorf("update: %s stable asset size is %d, cached size is %d", source.Name, asset.Size, meta.Size),
			}
		}
		return stableAuthorizationResult{Status: stableAuthorizationAuthorized, Source: source}
	}
	return stableAuthorizationResult{Status: stableAuthorizationUnavailable, Failures: failures}
}

func invalidateCachedUpdate(meta *cachedUpdate) error {
	metadataPath, err := updateMetadataPath()
	if err != nil {
		return err
	}
	var failures []string
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		failures = append(failures, fmt.Sprintf("remove metadata: %v", err))
	}
	if meta != nil && meta.Path != "" {
		cacheDir, cacheErr := updateCacheDir()
		if cacheErr != nil {
			failures = append(failures, fmt.Sprintf("resolve cache directory: %v", cacheErr))
		} else {
			relative, relativeErr := filepath.Rel(cacheDir, meta.Path)
			if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
				failures = append(failures, "refusing to remove cached artifact outside the update cache")
			} else if err := os.Remove(meta.Path); err != nil && !os.IsNotExist(err) {
				failures = append(failures, fmt.Sprintf("remove artifact: %v", err))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("update: invalidate cached update: %s", strings.Join(failures, "; "))
	}
	return nil
}

func readCurrentVerifiedCachedUpdate(
	ctx context.Context,
	newClient func() (*http.Client, error),
	sources []releaseSource,
	timeout time.Duration,
) (*cachedUpdate, []byte, error) {
	loadedMeta, _ := loadCachedUpdate()
	meta, data, err := readVerifiedCachedUpdate()
	if err != nil {
		invalidateErr := invalidateCachedUpdate(loadedMeta)
		if invalidateErr != nil {
			return nil, nil, fmt.Errorf("%w; %v", err, invalidateErr)
		}
		return nil, nil, err
	}
	c, err := newClient()
	if err != nil {
		return nil, nil, err
	}
	authorization := validateCachedUpdateAgainstStable(ctx, c, sources, timeout, meta)
	switch authorization.Status {
	case stableAuthorizationAuthorized:
		return meta, data, nil
	case stableAuthorizationUnavailable:
		return nil, nil, authorization.failure()
	case stableAuthorizationRejected:
		invalidateErr := invalidateCachedUpdate(meta)
		if invalidateErr != nil {
			return nil, nil, fmt.Errorf("%w; %v", authorization.failure(), invalidateErr)
		}
		return nil, nil, authorization.failure()
	default:
		return nil, nil, fmt.Errorf("更新：稳定版安装授权返回未知结果")
	}
}

func applyCurrentCachedUpdate(
	ctx context.Context,
	newClient func() (*http.Client, error),
	sources []releaseSource,
	timeout time.Duration,
	apply func(*cachedUpdate, []byte) error,
) (*cachedUpdate, error) {
	meta, data, err := readCurrentVerifiedCachedUpdate(ctx, newClient, sources, timeout)
	if err != nil {
		return nil, err
	}
	if err := apply(meta, data); err != nil {
		return nil, err
	}
	return meta, nil
}

// downloadAttempts caps how many times a transient transport failure (connection
// reset, read timeout, gateway 5xx) is retried before the update gives up. CN IPv6
// routes to Cloudflare reset mid-transfer often enough that a retry or two usually
// completes the download instead of surfacing a "forcibly closed" error.
const downloadAttempts = 3

// retryBackoff is the pause before the Nth retry; a package var so tests shrink it.
var retryBackoff = func(attempt int) time.Duration { return time.Duration(attempt) * 500 * time.Millisecond }

// retryTransient runs attempt 1..downloadAttempts of fetch, pausing between tries,
// until one succeeds. fetch receives the 1-based attempt number so a caller can
// switch transports on a retry. It stops early when ctx is cancelled (window closed
// / user cancelled). Only the transport is retried; the signature and sha256 checks
// run downstream in downloadVerify and are not retried.
func retryTransient(ctx context.Context, fetch func(attempt int) error) error {
	var err error
	for attempt := 1; attempt <= downloadAttempts; attempt++ {
		if err = fetch(attempt); err == nil {
			return nil
		}
		if ctx.Err() != nil || attempt == downloadAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryBackoff(attempt)):
		}
	}
	return err
}

// fetchBytes GETs a URL fully into memory, retrying transient transport failures.
func fetchBytes(ctx context.Context, c *http.Client, url string) ([]byte, error) {
	var data []byte
	err := retryTransient(ctx, func(int) error {
		var e error
		data, e = fetchBytesOnce(ctx, c, url)
		return e
	})
	return data, err
}

func fetchBytesOnce(ctx context.Context, c *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// download fetches url into memory, invoking onProgress as bytes arrive. A transient
// transport failure is retried; the retry resumes from the bytes already received
// via a Range request instead of restarting, and switches to the IPv4 fallback
// client (when provided) since a reset usually means the IPv6 route is the problem.
// total is the expected size for the progress denominator (refined from the response).
func download(ctx context.Context, c, fallback *http.Client, url string, total int64, onProgress func(received, total int64)) ([]byte, error) {
	var buf bytes.Buffer
	err := retryTransient(ctx, func(attempt int) error {
		client := c
		if attempt > 1 && fallback != nil {
			client = fallback
		}
		return downloadInto(ctx, client, url, &buf, &total, onProgress)
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// downloadInto appends url's body to buf, resuming from buf's current length via a
// Range request so a retry continues the partial download. A 206 carries the
// remaining bytes; a 200 means the server ignored Range, so buf is reset and the
// whole file re-downloaded. total is refined from the response for the progress
// denominator (Content-Length on 200, the size field of Content-Range on 206).
func downloadInto(ctx context.Context, c *http.Client, url string, buf *bytes.Buffer, total *int64, onProgress func(received, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if buf.Len() > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", buf.Len()))
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		buf.Reset()
		if resp.ContentLength > 0 {
			*total = resp.ContentLength
		}
	case http.StatusPartialContent:
		if t := totalFromContentRange(resp.Header.Get("Content-Range")); t > 0 {
			*total = t
		}
	default:
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	have := int64(buf.Len())
	pr := &progressReader{r: resp.Body, received: have, lastEmit: have, total: *total, onProgress: onProgress}
	_, err = io.Copy(buf, pr)
	return err
}

// totalFromContentRange parses the total size out of a "bytes 200-999/1000" header,
// returning 0 when it's absent or "*" (unknown).
func totalFromContentRange(v string) int64 {
	i := strings.LastIndex(v, "/")
	if i < 0 {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v[i+1:]), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// progressReader reports cumulative bytes read, throttled so the event channel
// isn't flooded.
type progressReader struct {
	r          io.Reader
	received   int64
	total      int64
	lastEmit   int64
	onProgress func(received, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.received += int64(n)
	// Emit roughly every 256 KiB, and always on the final read (io.EOF).
	if p.onProgress != nil && (p.received-p.lastEmit >= 256<<10 || err == io.EOF) {
		p.lastEmit = p.received
		p.onProgress(p.received, p.total)
	}
	return n, err
}

// checkSHA256 verifies data's digest matches the lowercase-hex want.
func checkSHA256(data []byte, want string) error {
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
		return fmt.Errorf("update: sha256 mismatch: got %s want %s", got, want)
	}
	return nil
}

// extractBinary pulls a single named regular file out of a .tar.gz blob.
func extractBinary(targz []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(targz))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && (h.Name == name || strings.HasSuffix(h.Name, "/"+name)) {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("update: %q not found in archive", name)
}

// applyLinux replaces the running binary with the one inside the downloaded
// tar.gz; the caller relaunches afterwards.
func applyLinux(targz []byte) error {
	bin, err := extractBinary(targz, "kwu-desktop")
	if err != nil {
		return err
	}
	return selfupdate.Apply(bytes.NewReader(bin), selfupdate.Options{})
}

func applyWindowsFile(path string) error {
	return installerCommand(path, currentInstallDir(), os.Getpid()).Start()
}

// currentInstallDir is the directory of the running executable 鈥?the location a
// Windows update must overwrite. Empty when it can't be resolved, in which case
// the installer falls back to its own InstallDir logic.
func currentInstallDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// relaunch starts a fresh copy of the (just-replaced) executable.
func relaunch() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Start()
}
