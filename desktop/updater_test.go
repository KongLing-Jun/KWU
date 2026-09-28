package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/desktop/internal/update"
)

func updaterManifestJSON(t *testing.T, version, assetURL string) string {
	t.Helper()
	manifest := update.Manifest{Version: version, Platforms: map[string]update.Asset{}}
	if assetURL != "" {
		manifest.Platforms[update.CurrentPlatform()] = update.Asset{
			URL: assetURL, Sig: assetURL + ".minisig", Size: 7, SHA256: sha256Hex([]byte("payload")),
		}
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal updater manifest: %v", err)
	}
	return string(raw)
}

func updaterHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"dev", "", false},
		{"", "", false},
		{"  ", "", false},
		{"1.2.3", "v1.2.3", true},
		{"v1.2.3", "v1.2.3", true},
		{"v1.2", "v1.2.0", true}, // semver.Canonical fills the patch
		{"garbage", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeVersion(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeVersion(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestEvaluate(t *testing.T) {
	mk := func(version string) *update.Manifest {
		return &update.Manifest{
			Version:   version,
			Notes:     "notes",
			Platforms: map[string]update.Asset{update.CurrentPlatform(): {Size: 999}},
		}
	}

	if got := evaluate("v1.0.0", mk("v1.1.0")); !got.Available {
		t.Error("v1.0.0 -> v1.1.0 should be available")
	}
	if got := evaluate("v1.1.0", mk("v1.1.0")); got.Available {
		t.Error("same version should not be available")
	}
	if got := evaluate("v1.2.0", mk("v1.1.0")); got.Available {
		t.Error("newer-than-manifest should not be available")
	}
	// A dev build must never auto-prompt, even against a real release.
	if got := evaluate("dev", mk("v1.1.0")); got.Available {
		t.Error("dev build should not prompt to update")
	}
	// An invalid manifest version is a check error, not an update.
	got := evaluate("v1.0.0", mk("not-a-version"))
	if got.Available || got.Err == "" {
		t.Errorf("invalid manifest version: got %+v", got)
	}
	// Metadata carries through.
	full := evaluate("v1.0.0", mk("v1.1.0"))
	if full.Latest != "v1.1.0" || full.Notes != "notes" || full.AssetSize != 999 {
		t.Errorf("metadata not carried: %+v", full)
	}
	if full.CanSelfUpdate {
		t.Errorf("test binary must stay on the manual update path on %s", runtime.GOOS)
	}
}

func TestUpdatePolicyFor(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		registered bool
		hasAsset   bool
		can        bool
		manualKind string
	}{
		{name: "installed windows", goos: "windows", registered: true, hasAsset: true, can: true},
		{name: "portable windows", goos: "windows", hasAsset: true, manualKind: "windows-portable"},
		{name: "windows asset missing", goos: "windows", registered: true},
		{name: "macos", goos: "darwin", registered: true, hasAsset: true, manualKind: "macos"},
		{name: "linux", goos: "linux", registered: true, hasAsset: true, manualKind: "linux"},
		{name: "unsupported", goos: "freebsd", hasAsset: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := updatePolicyFor(test.goos, test.registered, test.hasAsset)
			if got.CanSelfUpdate != test.can || got.ManualKind != test.manualKind {
				t.Fatalf("policy = %+v, want can=%v manualKind=%q", got, test.can, test.manualKind)
			}
		})
	}
}

func TestCloudReleasePlatform(t *testing.T) {
	tests := []struct {
		goos, goarch   string
		platform, arch string
		wantErr        bool
	}{
		{goos: "windows", goarch: "amd64", platform: "windows", arch: "amd64"},
		{goos: "darwin", goarch: "arm64", platform: "macos", arch: "arm64"},
		{goos: "linux", goarch: "amd64", platform: "linux", arch: "amd64"},
		{goos: "freebsd", goarch: "amd64", wantErr: true},
		{goos: "windows", goarch: "386", wantErr: true},
	}
	for _, test := range tests {
		platform, arch, err := cloudReleasePlatform(test.goos, test.goarch)
		if (err != nil) != test.wantErr || platform != test.platform || arch != test.arch {
			t.Errorf("cloudReleasePlatform(%q, %q) = (%q, %q, %v)", test.goos, test.goarch, platform, arch, err)
		}
	}
}

func TestFetchCloudReleaseMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/client-releases/latest" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("platform"); got != "macos" {
			t.Errorf("platform = %q", got)
		}
		if got := r.URL.Query().Get("architecture"); got != "arm64" {
			t.Errorf("architecture = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"release":{"platform":"macos","architecture":"arm64","version_name":"1.0.11","download_url":"https://example.com/KWU.dmg","release_notes":"# Fixed"}}}`)
	}))
	defer srv.Close()

	release, err := fetchCloudReleaseMetadata(context.Background(), srv.Client(), srv.URL, "darwin", "arm64")
	if err != nil {
		t.Fatalf("fetchCloudReleaseMetadata: %v", err)
	}
	if release.VersionName != "1.0.11" || release.ReleaseNotes != "# Fixed" {
		t.Fatalf("release = %+v", release)
	}
}

func TestFetchCloudReleaseMetadataFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "not found", status: http.StatusNotFound, body: `{}`},
		{name: "invalid json", status: http.StatusOK, body: `{`},
		{name: "missing release", status: http.StatusOK, body: `{"data":{}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer srv.Close()
			if _, err := fetchCloudReleaseMetadata(context.Background(), srv.Client(), srv.URL, "windows", "amd64"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestReconcileUpdateAvailabilityRequiresPlatformAsset(t *testing.T) {
	base := UpdateInfo{Available: true, Current: "v1.0.0", Latest: "v1.1.0", DownloadURL: downloadPage()}

	withoutRelease := base
	reconcileUpdateAvailability(&withoutRelease, "v1.0.0", false, nil)
	if withoutRelease.Available {
		t.Fatalf("missing platform release remained visible: %+v", withoutRelease)
	}

	manual := base
	release := &cloudReleaseMetadata{
		VersionName:  "v1.0.5",
		DownloadURL:  "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases/download/desktop-v1.0.5/manual.dmg",
		ReleaseNotes: "platform notes",
	}
	reconcileUpdateAvailability(&manual, "v1.0.0", false, release)
	if manual.Available {
		t.Fatalf("cloud-only platform release remained visible without a signed manifest asset: %+v", manual)
	}

	manifestAsset := base
	reconcileUpdateAvailability(&manifestAsset, "v1.0.0", true, nil)
	if !manifestAsset.Available {
		t.Fatalf("signed manifest asset was hidden when cloud metadata was unavailable: %+v", manifestAsset)
	}
}

func TestReleaseNotesPreferMatchingNonEmptyCloudValue(t *testing.T) {
	release := &cloudReleaseMetadata{
		VersionName:  "1.0.11",
		ReleaseNotes: "  release notes  ",
		DownloadURL:  "https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com/releases/desktop-v1.0.11/KWU.dmg",
	}
	if got := releaseNotes("manifest notes", "v1.0.11", release); got != "release notes" {
		t.Fatalf("matching cloud notes = %q", got)
	}
	for _, test := range []struct {
		name    string
		release *cloudReleaseMetadata
	}{
		{name: "request failed", release: nil},
		{name: "version mismatch", release: &cloudReleaseMetadata{VersionName: "v1.0.12", ReleaseNotes: "wrong"}},
		{name: "empty", release: &cloudReleaseMetadata{VersionName: "v1.0.11", ReleaseNotes: "  "}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := releaseNotes(" manifest notes ", "v1.0.11", test.release); got != "manifest notes" {
				t.Fatalf("fallback notes = %q", got)
			}
		})
	}
}

func TestCloudDownloadURLCannotReplaceAutomaticUpdateAsset(t *testing.T) {
	info := UpdateInfo{
		Available: true, Latest: "v1.0.11", Notes: "manifest notes",
		CanSelfUpdate: true, DownloadURL: "https://example.com/releases",
	}
	release := &cloudReleaseMetadata{
		VersionName: "v1.0.11", ReleaseNotes: "cloud notes",
		DownloadURL: "https://attacker.example/payload.exe",
	}
	reconcileUpdateAvailability(&info, "v1.0.10", true, release)
	if info.DownloadURL != "https://example.com/releases" {
		t.Fatalf("automatic update link was replaced by cloud metadata: %q", info.DownloadURL)
	}
	if info.Notes != "cloud notes" {
		t.Fatalf("cloud notes = %q", info.Notes)
	}
}

func TestSafeUpdateDownloadURL(t *testing.T) {
	want := "https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com/releases/desktop-v1.0.11/KWU.dmg"
	if got := safeUpdateDownloadURL(want); got != want {
		t.Fatalf("safe URL = %q", got)
	}
	for _, candidate := range []string{
		"",
		"/relative/KWU.dmg",
		"http://example.com/KWU.dmg",
		"https:///missing-host",
		"https://user:pass@example.com/KWU.dmg",
		"javascript:alert(1)",
	} {
		if got := safeUpdateDownloadURL(candidate); got != downloadPage() {
			t.Errorf("safeUpdateDownloadURL(%q) = %q", candidate, got)
		}
	}
}

func TestEnrichUpdateInfoFromCloudSkipsCurrentVersion(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()

	info := UpdateInfo{Available: false, Latest: "v1.0.11"}
	enrichUpdateInfoFromCloud(context.Background(), srv.Client(), srv.URL, "windows", "amd64", &info)
	if calls.Load() != 0 {
		t.Fatalf("cloud release endpoint called %d times", calls.Load())
	}
}

func TestEnrichUpdateInfoFromCloudIgnoresTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, `{"data":{"release":{"version_name":"1.0.11","release_notes":"late"}}}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	info := UpdateInfo{Available: true, Latest: "v1.0.11"}
	enrichUpdateInfoFromCloud(ctx, srv.Client(), srv.URL, "windows", "amd64", &info)
	if !info.Available || info.Latest != "v1.0.11" || info.Notes != "" {
		t.Fatalf("cloud timeout changed manifest result: %+v", info)
	}
}

func TestChannelSelectsDistinctPointers(t *testing.T) {
	orig := channel
	t.Cleanup(func() { channel = orig })

	channel = "stable"
	stable := manifestEndpoints()
	channel = "canary"
	canary := manifestEndpoints()

	if len(stable) != 3 {
		t.Fatalf("stable endpoints = %v, want target authority and frozen migration fallbacks", stable)
	}
	target, err := desktopReleaseTarget(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if stable[0] != ossReleasesBase+"/channels/stable/"+target+"/latest.json" {
		t.Errorf("stable primary endpoint = %q, want target OSS stable manifest", stable[0])
	}
	if stable[1] != ossReleasesBase+"/channels/stable/latest.json" ||
		stable[2] != ghReleasesBase+"/latest/download/latest.json" {
		t.Errorf("stable fallbacks = %v, want frozen global OSS and GitHub manifests", stable[1:])
	}
	if len(canary) != 0 {
		t.Errorf("canary endpoints = %v, want none until an owned download domain is configured", canary)
	}
	if downloadPage() == (ghReleasesBase + "/latest") {
		t.Error("canary download page should not be the stable latest releases page")
	}
}

func TestDesktopReleaseTargetRoutesSupportedPlatforms(t *testing.T) {
	tests := map[string]string{
		"windows/amd64": "windows-amd64",
		"windows/arm64": "windows-arm64",
		"darwin/amd64":  "macos-universal",
		"darwin/arm64":  "macos-universal",
	}
	for platform, want := range tests {
		parts := strings.Split(platform, "/")
		got, err := desktopReleaseTarget(parts[0], parts[1])
		if err != nil || got != want {
			t.Errorf("desktopReleaseTarget(%s) = %q, %v; want %q", platform, got, err, want)
		}
	}
	if _, err := desktopReleaseTarget("linux", "amd64"); err == nil {
		t.Fatal("linux unexpectedly received a selective desktop release target")
	}
}

func TestDownloadUsesVersionedTargetFallbackWithIdenticalIdentity(t *testing.T) {
	assetURL := "https://downloads.test/installer"
	body := updaterManifestJSON(t, "v1.2.3", assetURL)
	githubBody := updaterManifestJSON(t, "v1.2.3", "https://github.com/example/release/download/desktop-v1.2.3/installer")
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "oss.test":
			return updaterHTTPResponse(http.StatusOK, githubBody), nil
		case r.URL.Host == "github.com" && strings.HasSuffix(r.URL.Path, "/download/desktop-v1.2.3/latest-windows-amd64.json"):
			return updaterHTTPResponse(http.StatusOK, body), nil
		default:
			return nil, fmt.Errorf("unexpected request %s", r.URL)
		}
	})}
	attempts := 0
	verified, err := downloadUpdateFromSources(context.Background(), client, []releaseSource{{
		Name: "OSS target", ManifestURL: "https://oss.test/latest.json",
		Target: "windows-amd64", Authority: true,
	}}, time.Second, func(_ context.Context, _ update.Asset) ([]byte, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("primary download unavailable")
		}
		return []byte("verified"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if verified.Source.Name != "GitHub versioned target" || attempts != 2 {
		t.Fatalf("fallback result = %+v, attempts=%d", verified.Source, attempts)
	}
}

func TestTargetPointerUnavailableFallsBackToFrozenGlobalManifest(t *testing.T) {
	sources := releaseSourcesFor("windows", "arm64")
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/channels/stable/windows-arm64/latest.json":
			return nil, errors.New("target pointer unavailable")
		case "/channels/stable/latest.json":
			return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "v1.0.0", "https://downloads.test/installer")), nil
		default:
			return nil, fmt.Errorf("unexpected request %s", r.URL)
		}
	})}
	fetched, err := fetchManifestFromSources(context.Background(), client, sources, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Source.Name != "OSS migration fallback" || fetched.Manifest.Version != "v1.0.0" {
		t.Fatalf("frozen fallback = %+v", fetched)
	}
}

func TestCacheAuthorizationUsesOnlyCurrentTargetPointer(t *testing.T) {
	orig := channel
	channel = "stable"
	t.Cleanup(func() { channel = orig })
	sources := releaseAuthorizationSources()
	if len(sources) != 1 || !sources[0].Authority || sources[0].FrozenFallback {
		t.Fatalf("cache authorization sources = %+v", sources)
	}
	if !strings.Contains(sources[0].ManifestURL, "/channels/stable/") ||
		strings.HasSuffix(sources[0].ManifestURL, "/channels/stable/latest.json") {
		t.Fatalf("cache authorization did not select a target pointer: %s", sources[0].ManifestURL)
	}
}

func TestProductionCacheAuthorizationUnavailableRetainsCacheWithoutFallback(t *testing.T) {
	cacheDir, meta, _ := saveStableTestCache(t)
	sources := releaseAuthorizationSources()
	if len(sources) != 1 {
		t.Fatalf("production authorization sources = %+v, want one current-target pointer", sources)
	}
	var requested []string
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		requested = append(requested, r.URL.String())
		return updaterHTTPResponse(http.StatusOK, "{"), nil
	})}
	var applyCalls int
	_, err := applyCurrentCachedUpdate(
		context.Background(),
		func() (*http.Client, error) { return client, nil },
		sources,
		time.Second,
		func(*cachedUpdate, []byte) error {
			applyCalls++
			return nil
		},
	)
	if err == nil || applyCalls != 0 {
		t.Fatalf("unavailable production authorization err=%v applyCalls=%d", err, applyCalls)
	}
	if len(requested) != 1 || requested[0] != sources[0].ManifestURL {
		t.Fatalf("production authorization requests = %v, want only %s", requested, sources[0].ManifestURL)
	}
	assertCachedUpdatePresence(t, cacheDir, meta, true)
}

func TestFetchManifestUsesIndependentSourceDeadline(t *testing.T) {
	var fallbackCalls atomic.Int32
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "oss.test":
			<-r.Context().Done()
			return nil, r.Context().Err()
		case "github.test":
			fallbackCalls.Add(1)
			if err := r.Context().Err(); err != nil {
				t.Fatalf("GitHub source inherited the OSS deadline: %v", err)
			}
			return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "v1.0.2", "")), nil
		default:
			return nil, fmt.Errorf("unexpected host %s", r.URL.Host)
		}
	})}
	sources := []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}
	fetched, err := fetchManifestFromSources(context.Background(), client, sources, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("fetchManifestFromSources: %v", err)
	}
	if fetched.Source.Name != "GitHub" || fetched.Manifest.Version != "v1.0.2" || fallbackCalls.Load() != 1 {
		t.Fatalf("unexpected fallback result: source=%s manifest=%+v calls=%d", fetched.Source.Name, fetched.Manifest, fallbackCalls.Load())
	}
}

func TestFetchManifestFallsBackFromInvalidPrimary(t *testing.T) {
	fastRetry(t)
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "not found", status: http.StatusNotFound, body: `{}`},
		{name: "bad json", status: http.StatusOK, body: `{`},
		{name: "invalid version", status: http.StatusOK, body: updaterManifestJSON(t, "invalid", "")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var fallbackCalls atomic.Int32
			client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "oss.test" {
					return updaterHTTPResponse(test.status, test.body), nil
				}
				fallbackCalls.Add(1)
				return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "v1.0.2", "")), nil
			})}
			sources := []releaseSource{
				{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
				{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
			}
			fetched, err := fetchManifestFromSources(context.Background(), client, sources, time.Second)
			if err != nil || fetched.Source.Name != "GitHub" || fallbackCalls.Load() == 0 {
				t.Fatalf("fallback result = %+v, %v; GitHub calls=%d", fetched, err, fallbackCalls.Load())
			}
		})
	}
}

func TestFetchManifestAggregatesSourceErrors(t *testing.T) {
	fastRetry(t)
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return updaterHTTPResponse(http.StatusBadGateway, r.URL.Host), nil
	})}
	sources := []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}
	_, err := fetchManifestFromSources(context.Background(), client, sources, time.Second)
	if err == nil || !strings.Contains(err.Error(), "OSS manifest") || !strings.Contains(err.Error(), "GitHub manifest") {
		t.Fatalf("source errors were not aggregated: %v", err)
	}
}

func TestDownloadUpdateFallsBackAcrossVerificationStages(t *testing.T) {
	tests := []struct {
		name       string
		primaryURL string
		primaryErr error
	}{
		{name: "asset download", primaryURL: "https://oss.test/asset.exe", primaryErr: errors.New("download asset: 404")},
		{name: "signature fetch", primaryURL: "https://oss.test/asset.exe", primaryErr: errors.New("fetch signature: 404")},
		{name: "signature verification", primaryURL: "https://oss.test/asset.exe", primaryErr: errors.New("verify signature: invalid")},
		{name: "sha256 verification", primaryURL: "https://oss.test/asset.exe", primaryErr: errors.New("verify sha256: mismatch")},
		{name: "missing platform asset", primaryURL: "", primaryErr: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var githubManifestCalls atomic.Int32
			client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "oss.test" {
					return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "v1.0.2", test.primaryURL)), nil
				}
				githubManifestCalls.Add(1)
				return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "1.0.2", "https://github.test/asset.exe")), nil
			})}
			var primaryVerifies, fallbackVerifies int
			verified, err := downloadUpdateFromSources(context.Background(), client, []releaseSource{
				{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
				{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
			}, time.Second, func(_ context.Context, asset update.Asset) ([]byte, error) {
				if strings.Contains(asset.URL, "oss.test") {
					primaryVerifies++
					return nil, test.primaryErr
				}
				fallbackVerifies++
				return []byte("payload"), nil
			})
			if err != nil {
				t.Fatalf("downloadUpdateFromSources: %v", err)
			}
			if verified.Source.Name != "GitHub" || verified.Version != "v1.0.2" || string(verified.Data) != "payload" || githubManifestCalls.Load() != 1 || fallbackVerifies != 1 {
				t.Fatalf("unexpected verified fallback: %+v, GitHub manifests=%d fallback verifies=%d", verified, githubManifestCalls.Load(), fallbackVerifies)
			}
			wantPrimaryVerifies := 1
			if test.primaryURL == "" {
				wantPrimaryVerifies = 0
			}
			if primaryVerifies != wantPrimaryVerifies {
				t.Fatalf("primary verification calls = %d, want %d", primaryVerifies, wantPrimaryVerifies)
			}
		})
	}
}

func TestDownloadUpdateRejectsDifferentFallbackVersion(t *testing.T) {
	var fallbackVerifies atomic.Int32
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		version := "v1.0.2"
		assetURL := "https://oss.test/asset.exe"
		if r.URL.Host == "github.test" {
			version = "v1.0.1"
			assetURL = "https://github.test/asset.exe"
		}
		return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, version, assetURL)), nil
	})}
	_, err := downloadUpdateFromSources(context.Background(), client, []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}, time.Second, func(_ context.Context, asset update.Asset) ([]byte, error) {
		if strings.Contains(asset.URL, "github.test") {
			fallbackVerifies.Add(1)
			return []byte("must not download"), nil
		}
		return nil, errors.New("download asset: unavailable")
	})
	if err == nil || !strings.Contains(err.Error(), "got v1.0.1, want v1.0.2") {
		t.Fatalf("different fallback version was not rejected: %v", err)
	}
	if fallbackVerifies.Load() != 0 {
		t.Fatalf("different-version fallback asset was requested %d times", fallbackVerifies.Load())
	}
}

func TestDownloadUpdateDoesNotContactGitHubAfterOSSSuccess(t *testing.T) {
	var githubCalls atomic.Int32
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.test" {
			githubCalls.Add(1)
		}
		return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "v1.0.2", "https://oss.test/asset.exe")), nil
	})}
	verified, err := downloadUpdateFromSources(context.Background(), client, []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}, time.Second, func(context.Context, update.Asset) ([]byte, error) {
		return []byte("payload"), nil
	})
	if err != nil || verified.Source.Name != "OSS" || githubCalls.Load() != 0 {
		t.Fatalf("OSS success unexpectedly used GitHub: result=%+v err=%v calls=%d", verified, err, githubCalls.Load())
	}
}

func TestDownloadUpdateStopsFallbackWhenParentCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var githubCalls atomic.Int32
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.test" {
			githubCalls.Add(1)
		}
		return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "v1.0.2", "https://"+r.URL.Host+"/asset.exe")), nil
	})}
	_, err := downloadUpdateFromSources(ctx, client, []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}, time.Second, func(context.Context, update.Asset) ([]byte, error) {
		cancel()
		return nil, context.Canceled
	})
	if !errors.Is(err, context.Canceled) || githubCalls.Load() != 0 {
		t.Fatalf("cancelled update continued to fallback: err=%v GitHub calls=%d", err, githubCalls.Load())
	}
}

func TestDownloadUpdateAggregatesAssetFailures(t *testing.T) {
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, "v1.0.2", "https://"+r.URL.Host+"/asset.exe")), nil
	})}
	_, err := downloadUpdateFromSources(context.Background(), client, []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}, time.Second, func(_ context.Context, asset update.Asset) ([]byte, error) {
		return nil, fmt.Errorf("download asset %s failed", asset.URL)
	})
	if err == nil || !strings.Contains(err.Error(), "OSS asset") || !strings.Contains(err.Error(), "GitHub asset") {
		t.Fatalf("asset failures were not aggregated: %v", err)
	}
}

func TestFallbackCacheMatchesOptionalVersionPrefix(t *testing.T) {
	withUpdateCacheDir(t)
	oldChannel := channel
	channel = "stable"
	t.Cleanup(func() { channel = oldChannel })

	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		version := "v1.0.2"
		assetURL := "https://oss.test/asset.exe"
		if r.URL.Host == "github.test" {
			version = "1.0.2"
			assetURL = "https://github.test/asset.exe"
		}
		return updaterHTTPResponse(http.StatusOK, updaterManifestJSON(t, version, assetURL)), nil
	})}
	verified, err := downloadUpdateFromSources(context.Background(), client, []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}, time.Second, func(_ context.Context, asset update.Asset) ([]byte, error) {
		if strings.Contains(asset.URL, "oss.test") {
			return nil, errors.New("download asset: unavailable")
		}
		return []byte("payload"), nil
	})
	if err != nil {
		t.Fatalf("fallback download: %v", err)
	}
	if verified.Version != "v1.0.2" || verified.Manifest.Version != "1.0.2" {
		t.Fatalf("locked and source versions = %q / %q", verified.Version, verified.Manifest.Version)
	}
	if _, err := saveCachedUpdate(verified.Version, verified.Asset, verified.Data); err != nil {
		t.Fatalf("save fallback cache: %v", err)
	}
	ossManifest := &update.Manifest{
		Version: "v1.0.2",
		Platforms: map[string]update.Asset{
			update.CurrentPlatform(): {
				URL: "https://oss.test/asset.exe", Sig: "https://oss.test/asset.exe.minisig",
				Size: 7, SHA256: sha256Hex([]byte("payload")),
			},
		},
	}
	if info := evaluate("v1.0.0", ossManifest); !info.Downloaded {
		t.Fatalf("OSS manifest did not recognize the prefix-equivalent fallback cache: %+v", info)
	}
}

func withUpdateCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	restore := updateCacheBaseDir
	updateCacheBaseDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { updateCacheBaseDir = restore })
	return dir
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestSaveCachedUpdateMarksEvaluateDownloaded(t *testing.T) {
	withUpdateCacheDir(t)
	oldChannel := channel
	channel = "stable"
	t.Cleanup(func() { channel = oldChannel })

	data := []byte("verified artifact")
	asset := update.Asset{
		URL:    "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases/download/desktop-v9.9.9/%E4%BA%91%E6%99%BA%E4%BC%81%E5%8A%9E-linux-amd64.tar.gz",
		Size:   int64(len(data)),
		SHA256: sha256Hex(data),
	}
	manifest := &update.Manifest{
		Version:   "v9.9.9",
		Platforms: map[string]update.Asset{update.CurrentPlatform(): asset},
	}
	if got := evaluate("v1.0.0", manifest); got.Downloaded {
		t.Fatal("fresh cache should not report a downloaded update")
	}
	meta, err := saveCachedUpdate("v9.9.9", asset, data)
	if err != nil {
		t.Fatalf("saveCachedUpdate: %v", err)
	}
	if meta.Version != "v9.9.9" || meta.Channel != "stable" || meta.Platform != update.CurrentPlatform() {
		t.Fatalf("cached metadata mismatch: %+v", meta)
	}
	if got := evaluate("v1.0.0", manifest); !got.Downloaded {
		t.Fatalf("evaluate did not detect cached update: %+v", got)
	}
}

func TestDefaultUpdateCacheBaseDirUsesClouvia(t *testing.T) {
	for _, name := range []string{"CLOUVIA_CACHE_HOME", "KWU_CACHE_HOME", "SUIYUAN_CACHE_HOME", "REASONIX_CACHE_HOME"} {
		t.Setenv(name, "")
	}
	dir, err := defaultUpdateCacheBaseDir()
	if err != nil {
		t.Fatalf("defaultUpdateCacheBaseDir: %v", err)
	}
	if got := filepath.Base(filepath.Dir(dir)); got != "Clouvia" {
		t.Fatalf("cache app dir = %q, want Clouvia", got)
	}
}

func TestDefaultUpdateCacheBaseDirUsesConfiguredCacheRoot(t *testing.T) {
	aliases := []string{"CLOUVIA_CACHE_HOME", "KWU_CACHE_HOME", "SUIYUAN_CACHE_HOME", "REASONIX_CACHE_HOME"}
	for _, active := range aliases {
		t.Run(active, func(t *testing.T) {
			for _, name := range aliases {
				t.Setenv(name, "")
			}
			root := t.TempDir()
			t.Setenv(active, root)
			dir, err := defaultUpdateCacheBaseDir()
			if err != nil {
				t.Fatalf("defaultUpdateCacheBaseDir: %v", err)
			}
			if got, want := filepath.Clean(dir), filepath.Join(root, "updates"); got != want {
				t.Fatalf("cache dir = %q, want %q", got, want)
			}
		})
	}
}

func TestAssetFileNameFallbackUsesDisplayName(t *testing.T) {
	got := assetFileName(update.Asset{}, "v9.9.9")
	want := "云智企办-v9.9.9-" + update.CurrentPlatform() + ".update"
	if got != want {
		t.Fatalf("assetFileName fallback = %q, want %q", got, want)
	}
}

func TestCachedUpdateRejectsTamperedArtifact(t *testing.T) {
	cacheDir := withUpdateCacheDir(t)
	oldChannel := channel
	channel = "stable"
	t.Cleanup(func() { channel = oldChannel })

	data := []byte("verified artifact")
	asset := update.Asset{
		URL:    "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases/download/desktop-v9.9.9/%E4%BA%91%E6%99%BA%E4%BC%81%E5%8A%9E-linux-amd64.tar.gz",
		Size:   int64(len(data)),
		SHA256: sha256Hex(data),
	}
	meta, err := saveCachedUpdate("v9.9.9", asset, data)
	if err != nil {
		t.Fatalf("saveCachedUpdate: %v", err)
	}
	if err := os.WriteFile(meta.Path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if cachedUpdateMatches("v9.9.9", asset) {
		t.Fatal("tampered cached artifact should not match")
	}
	if _, _, err := readVerifiedCachedUpdate(); err == nil {
		t.Fatal("readVerifiedCachedUpdate should reject a tampered artifact")
	}
	var clientCalls int
	if _, _, err := readCurrentVerifiedCachedUpdate(context.Background(), func() (*http.Client, error) {
		clientCalls++
		return &http.Client{}, nil
	}, []releaseSource{{Name: "unused", ManifestURL: "https://unused.test/latest.json"}}, time.Second); err == nil {
		t.Fatal("readCurrentVerifiedCachedUpdate should reject a tampered artifact")
	}
	if clientCalls != 0 {
		t.Fatalf("tampered artifact accessed remote authorization %d times", clientCalls)
	}
	assertCachedUpdatePresence(t, cacheDir, meta, false)
}

func TestCachedUpdateRejectsDifferentChannel(t *testing.T) {
	withUpdateCacheDir(t)
	oldChannel := channel
	channel = "stable"
	t.Cleanup(func() { channel = oldChannel })

	data := []byte("verified artifact")
	asset := update.Asset{
		URL:    "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases/download/desktop-v9.9.9/%E4%BA%91%E6%99%BA%E4%BC%81%E5%8A%9E-linux-amd64.tar.gz",
		Size:   int64(len(data)),
		SHA256: sha256Hex(data),
	}
	if _, err := saveCachedUpdate("v9.9.9", asset, data); err != nil {
		t.Fatalf("saveCachedUpdate: %v", err)
	}
	channel = "canary"
	if _, _, err := readVerifiedCachedUpdate(); err == nil {
		t.Fatal("readVerifiedCachedUpdate should reject a cache from another channel")
	}
}

func saveStableTestCache(t *testing.T) (string, *cachedUpdate, []byte) {
	t.Helper()
	cacheDir := withUpdateCacheDir(t)
	oldChannel := channel
	channel = "stable"
	t.Cleanup(func() { channel = oldChannel })
	data := []byte("payload")
	meta, err := saveCachedUpdate("v9.9.9", update.Asset{
		URL: "https://oss.test/asset.exe", Sig: "https://oss.test/asset.exe.minisig",
		Size: int64(len(data)), SHA256: sha256Hex(data),
	}, data)
	if err != nil {
		t.Fatalf("saveCachedUpdate: %v", err)
	}
	return cacheDir, meta, data
}

func stableTestManifest(t *testing.T, version string, asset *update.Asset) string {
	t.Helper()
	manifest := update.Manifest{Version: version, Platforms: map[string]update.Asset{}}
	if asset != nil {
		manifest.Platforms[update.CurrentPlatform()] = *asset
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal stable manifest: %v", err)
	}
	return string(raw)
}

func assertCachedUpdatePresence(t *testing.T, cacheDir string, meta *cachedUpdate, want bool) {
	t.Helper()
	for name, path := range map[string]string{
		"artifact": meta.Path,
		"metadata": filepath.Join(cacheDir, "downloaded.json"),
	} {
		_, err := os.Stat(path)
		if want && err != nil {
			t.Fatalf("%s should be retained: %v", name, err)
		}
		if !want && !os.IsNotExist(err) {
			t.Fatalf("%s should be removed: %v", name, err)
		}
	}
}

func TestStableAuthorizationFailureMessage(t *testing.T) {
	tests := []struct {
		name   string
		result stableAuthorizationResult
		want   string
	}{
		{
			name:   "no source detail",
			result: stableAuthorizationResult{Status: stableAuthorizationUnavailable},
			want:   "更新：稳定版安装授权暂时不可用",
		},
		{
			name: "preserves source and cause",
			result: stableAuthorizationResult{
				Status: stableAuthorizationUnavailable,
				Failures: []stableSourceFailure{{
					Source: releaseSource{Name: "OSS"},
					Err:    errors.New("连接超时"),
				}},
			},
			want: "更新：稳定版安装授权暂时不可用：来源 OSS 的更新清单：连接超时",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.failure().Error(); got != tt.want {
				t.Fatalf("failure message = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentCachedUpdateUsesOrderedStableAuthorization(t *testing.T) {
	sources := []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}
	tests := []struct {
		name         string
		response     func(*testing.T, *http.Request, update.Asset) (*http.Response, error)
		wantHosts    []string
		wantApply    int
		wantRetained bool
		wantStatus   stableAuthorizationStatus
		wantSource   string
		timeout      time.Duration
	}{
		{
			name: "OSS match short-circuits unavailable GitHub",
			response: func(t *testing.T, r *http.Request, asset update.Asset) (*http.Response, error) {
				if r.URL.Host == "github.test" {
					return nil, context.DeadlineExceeded
				}
				return updaterHTTPResponse(http.StatusOK, stableTestManifest(t, "v9.9.9", &asset)), nil
			},
			wantHosts: []string{"oss.test"}, wantApply: 1, wantRetained: true,
			wantStatus: stableAuthorizationAuthorized, wantSource: "OSS",
		},
		{
			name: "unavailable OSS falls back to matching GitHub",
			response: func(t *testing.T, r *http.Request, asset update.Asset) (*http.Response, error) {
				if r.URL.Host == "oss.test" {
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				return updaterHTTPResponse(http.StatusOK, stableTestManifest(t, "v9.9.9", &asset)), nil
			},
			wantHosts: []string{"oss.test", "github.test"}, wantApply: 1, wantRetained: true,
			wantStatus: stableAuthorizationAuthorized, wantSource: "GitHub", timeout: 10 * time.Millisecond,
		},
		{
			name: "all sources unavailable retains cache",
			response: func(t *testing.T, r *http.Request, asset update.Asset) (*http.Response, error) {
				return updaterHTTPResponse(http.StatusOK, "{"), nil
			},
			wantHosts: []string{"oss.test", "github.test"}, wantApply: 0, wantRetained: true,
			wantStatus: stableAuthorizationUnavailable,
		},
		{
			name: "unavailable OSS and rejecting GitHub invalidates cache",
			response: func(t *testing.T, r *http.Request, asset update.Asset) (*http.Response, error) {
				if r.URL.Host == "oss.test" {
					return updaterHTTPResponse(http.StatusOK, "{"), nil
				}
				return updaterHTTPResponse(http.StatusOK, stableTestManifest(t, "v9.9.8", &asset)), nil
			},
			wantHosts: []string{"oss.test", "github.test"}, wantApply: 0, wantRetained: false,
			wantStatus: stableAuthorizationRejected, wantSource: "GitHub",
		},
		{
			name: "both matching sources stop after OSS",
			response: func(t *testing.T, r *http.Request, asset update.Asset) (*http.Response, error) {
				return updaterHTTPResponse(http.StatusOK, stableTestManifest(t, "v9.9.9", &asset)), nil
			},
			wantHosts: []string{"oss.test"}, wantApply: 1, wantRetained: true,
			wantStatus: stableAuthorizationAuthorized, wantSource: "OSS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheDir, meta, data := saveStableTestCache(t)
			asset := update.Asset{URL: "https://source.test/asset.exe", Sig: "https://source.test/asset.exe.minisig", Size: meta.Size, SHA256: meta.SHA256}
			var hosts []string
			client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
				hosts = append(hosts, r.URL.Host)
				return tt.response(t, r, asset)
			})}
			timeout := tt.timeout
			if timeout == 0 {
				timeout = time.Second
			}

			result := validateCachedUpdateAgainstStable(context.Background(), client, sources, timeout, meta)
			if result.Status != tt.wantStatus || result.Source.Name != tt.wantSource {
				t.Fatalf("authorization = status %v source %q, want %v/%q", result.Status, result.Source.Name, tt.wantStatus, tt.wantSource)
			}
			if tt.wantStatus == stableAuthorizationUnavailable && len(result.Failures) != len(sources) {
				t.Fatalf("unavailable failures=%+v, want one raw failure per source", result.Failures)
			}
			hosts = nil
			var applyCalls int
			_, err := applyCurrentCachedUpdate(context.Background(), func() (*http.Client, error) { return client, nil }, sources, timeout, func(gotMeta *cachedUpdate, got []byte) error {
				applyCalls++
				if gotMeta.Version != meta.Version || !bytes.Equal(got, data) {
					t.Fatalf("unexpected apply input: meta=%+v data=%q", gotMeta, got)
				}
				return nil
			})
			if (err == nil) != (tt.wantApply == 1) {
				t.Fatalf("apply result err=%v, wantApply=%d", err, tt.wantApply)
			}
			if applyCalls != tt.wantApply {
				t.Fatalf("applyCalls=%d, want %d", applyCalls, tt.wantApply)
			}
			if !slices.Equal(hosts, tt.wantHosts) {
				t.Fatalf("source hosts=%v, want %v", hosts, tt.wantHosts)
			}
			assertCachedUpdatePresence(t, cacheDir, meta, tt.wantRetained)
		})
	}
}

func TestOSSExplicitRejectionStopsFallbackAndInvalidatesCache(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*update.Manifest)
	}{
		{name: "version", mutate: func(m *update.Manifest) { m.Version = "v9.9.8" }},
		{name: "platform", mutate: func(m *update.Manifest) { delete(m.Platforms, update.CurrentPlatform()) }},
		{name: "SHA256", mutate: func(m *update.Manifest) {
			asset := m.Platforms[update.CurrentPlatform()]
			asset.SHA256 = sha256Hex([]byte("different"))
			m.Platforms[update.CurrentPlatform()] = asset
		}},
		{name: "size", mutate: func(m *update.Manifest) {
			asset := m.Platforms[update.CurrentPlatform()]
			asset.Size++
			m.Platforms[update.CurrentPlatform()] = asset
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheDir, meta, _ := saveStableTestCache(t)
			manifest := update.Manifest{
				Version: "v9.9.9",
				Platforms: map[string]update.Asset{update.CurrentPlatform(): {
					URL: "https://oss.test/asset.exe", Sig: "https://oss.test/asset.exe.minisig",
					Size: meta.Size, SHA256: meta.SHA256,
				}},
			}
			tt.mutate(&manifest)
			raw, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			var sourceCalls, applyCalls int
			client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
				sourceCalls++
				if r.URL.Host != "oss.test" {
					t.Fatalf("explicit OSS rejection must not access fallback source: %s", r.URL.Host)
				}
				return updaterHTTPResponse(http.StatusOK, string(raw)), nil
			})}
			result := validateCachedUpdateAgainstStable(context.Background(), client, []releaseSource{
				{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
				{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
			}, time.Second, meta)
			if result.Status != stableAuthorizationRejected || result.Source.Name != "OSS" || result.Err == nil {
				t.Fatalf("authorization = %+v, want structured OSS rejection", result)
			}
			sourceCalls = 0
			_, err = applyCurrentCachedUpdate(context.Background(), func() (*http.Client, error) { return client, nil }, []releaseSource{
				{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
				{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
			}, time.Second, func(*cachedUpdate, []byte) error {
				applyCalls++
				return nil
			})
			if err == nil || sourceCalls != 1 || applyCalls != 0 {
				t.Fatalf("rejection result err=%v sourceCalls=%d applyCalls=%d", err, sourceCalls, applyCalls)
			}
			assertCachedUpdatePresence(t, cacheDir, meta, false)
		})
	}
}

func TestCurrentCachedUpdateRetainsCacheWhenAuthorizationCannotStart(t *testing.T) {
	tests := []struct {
		name      string
		ctx       func() context.Context
		sources   []releaseSource
		newClient func(*int) func() (*http.Client, error)
		wantCalls int
	}{
		{
			name:    "HTTP client creation failure",
			ctx:     context.Background,
			sources: []releaseSource{{Name: "OSS", ManifestURL: "https://oss.test/latest.json"}},
			newClient: func(calls *int) func() (*http.Client, error) {
				return func() (*http.Client, error) {
					(*calls)++
					return nil, errors.New("proxy configuration unavailable")
				}
			},
			wantCalls: 1,
		},
		{
			name: "no configured sources",
			ctx:  context.Background,
			newClient: func(calls *int) func() (*http.Client, error) {
				return func() (*http.Client, error) {
					(*calls)++
					return &http.Client{}, nil
				}
			},
			wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheDir, meta, _ := saveStableTestCache(t)
			var clientCalls, applyCalls int
			_, err := applyCurrentCachedUpdate(tt.ctx(), tt.newClient(&clientCalls), tt.sources, time.Second, func(*cachedUpdate, []byte) error {
				applyCalls++
				return nil
			})
			if err == nil || clientCalls != tt.wantCalls || applyCalls != 0 {
				t.Fatalf("result err=%v clientCalls=%d applyCalls=%d", err, clientCalls, applyCalls)
			}
			assertCachedUpdatePresence(t, cacheDir, meta, true)
		})
	}
}

func TestCurrentCachedUpdateStopsFallbackWhenParentContextIsCanceled(t *testing.T) {
	cacheDir, meta, _ := saveStableTestCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	var sourceCalls, applyCalls int
	client := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		sourceCalls++
		cancel()
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	_, err := applyCurrentCachedUpdate(ctx, func() (*http.Client, error) { return client, nil }, []releaseSource{
		{Name: "OSS", ManifestURL: "https://oss.test/latest.json"},
		{Name: "GitHub", ManifestURL: "https://github.test/latest.json"},
	}, time.Second, func(*cachedUpdate, []byte) error {
		applyCalls++
		return nil
	})
	if err == nil || sourceCalls != 1 || applyCalls != 0 {
		t.Fatalf("canceled result err=%v sourceCalls=%d applyCalls=%d", err, sourceCalls, applyCalls)
	}
	assertCachedUpdatePresence(t, cacheDir, meta, true)
}

func TestCheckSHA256(t *testing.T) {
	data := []byte("hello world")
	// echo -n "hello world" | shasum -a 256
	const sum = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if err := checkSHA256(data, sum); err != nil {
		t.Errorf("matching digest should pass: %v", err)
	}
	if err := checkSHA256(data, "deadbeef"); err == nil {
		t.Error("mismatched digest should fail")
	}
	// Case-insensitive hex.
	if err := checkSHA256(data, "B94D27B9934D3E08A52E52D7DA7DABFAC484EFE37A5380EE9088F7ACE2EFCDE9"); err != nil {
		t.Errorf("uppercase digest should pass: %v", err)
	}
}

func TestExtractBinary(t *testing.T) {
	want := []byte("#!/bin/sh\necho kwu\n")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string][]byte{"README": []byte("ignore me"), "kwu-desktop": want}
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()

	got, err := extractBinary(buf.Bytes(), "kwu-desktop")
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extracted %q, want %q", got, want)
	}
	if _, err := extractBinary(buf.Bytes(), "missing"); err == nil {
		t.Error("missing entry should error")
	}
}

func fastRetry(t *testing.T) {
	t.Helper()
	restore := retryBackoff
	retryBackoff = func(int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { retryBackoff = restore })
}

func TestDownloadRecoversFromMidStreamReset(t *testing.T) {
	fastRetry(t)
	const body = "complete-installer-bytes"
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < int32(downloadAttempts) {
			// Mid-stream reset: promise 100 bytes, send a few, drop the socket 鈥?			// the client's body read fails with unexpected EOF, exactly the CN-IPv6
			// "forcibly closed" case the retry exists for.
			conn, bw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			bw.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\npartial")
			bw.Flush()
			conn.Close()
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	data, err := download(context.Background(), srv.Client(), nil, srv.URL, 0, nil)
	if err != nil {
		t.Fatalf("download should recover after %d resets: %v", downloadAttempts-1, err)
	}
	if string(data) != body {
		t.Fatalf("got %q, want %q", data, body)
	}
	if n := atomic.LoadInt32(&calls); n != int32(downloadAttempts) {
		t.Fatalf("made %d attempts, want %d", n, downloadAttempts)
	}
}

func TestDownloadGivesUpAfterCap(t *testing.T) {
	fastRetry(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close()
	}))
	defer srv.Close()

	if _, err := download(context.Background(), srv.Client(), nil, srv.URL, 0, nil); err == nil {
		t.Fatal("download should fail after exhausting retries")
	}
	if n := atomic.LoadInt32(&calls); n != int32(downloadAttempts) {
		t.Fatalf("made %d attempts, want %d", n, downloadAttempts)
	}
}

func TestRetryTransientStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	if err := retryTransient(ctx, func(int) error {
		calls++
		return errors.New("boom")
	}); err == nil {
		t.Fatal("cancelled retry should return the error")
	}
	if calls != 1 {
		t.Fatalf("cancelled retry made %d calls, want 1", calls)
	}
}

func TestDownloadResumesWithRange(t *testing.T) {
	fastRetry(t)
	full := bytes.Repeat([]byte("0123456789"), 50) // 500 bytes
	const cut = 200
	var calls int32
	rangeCh := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// First attempt: promise the whole file, send a prefix, drop the socket.
			conn, bw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			fmt.Fprintf(bw, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n", len(full))
			bw.Write(full[:cut])
			bw.Flush()
			conn.Close()
			return
		}
		// Resume attempt: honor the Range header with a 206 + Content-Range.
		rng := r.Header.Get("Range")
		rangeCh <- rng
		start := 0
		fmt.Sscanf(rng, "bytes=%d-", &start)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(full)-1, len(full)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(full[start:])
	}))
	defer srv.Close()

	data, err := download(context.Background(), srv.Client(), nil, srv.URL, 0, nil)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if !bytes.Equal(data, full) {
		t.Fatalf("assembled %d bytes, want %d (equal=%v)", len(data), len(full), bytes.Equal(data, full))
	}
	select {
	case rng := <-rangeCh:
		if rng != fmt.Sprintf("bytes=%d-", cut) {
			t.Fatalf("resume Range = %q, want bytes=%d-", rng, cut)
		}
	default:
		t.Fatal("resume attempt sent no Range header")
	}
}

func TestDownloadFallsBackToSecondClient(t *testing.T) {
	fastRetry(t)
	const body = "served-over-ipv4"
	primary := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection reset (ipv6)")
	})}
	var fbCalls int32
	fallback := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&fbCalls, 1)
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Header:        make(http.Header),
		}, nil
	})}

	data, err := download(context.Background(), primary, fallback, "http://example.invalid/x", 0, nil)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(data) != body {
		t.Fatalf("got %q, want %q", data, body)
	}
	if atomic.LoadInt32(&fbCalls) == 0 {
		t.Fatal("fallback client was never used after the primary failed")
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
