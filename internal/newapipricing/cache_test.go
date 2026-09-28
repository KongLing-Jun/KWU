package newapipricing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/modelbilling"
)

var cacheTestStart = time.Date(2026, 7, 11, 1, 2, 3, 0, time.UTC)

type cacheTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *cacheTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *cacheTestClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

type transportFunc func(context.Context, string) ([]byte, error)

func (f transportFunc) Fetch(ctx context.Context, endpoint string) ([]byte, error) {
	return f(ctx, endpoint)
}

type statusSourceFunc func(context.Context) (modelbilling.PricingSourceStatus, error)

func (f statusSourceFunc) FetchStatus(ctx context.Context) (modelbilling.PricingSourceStatus, error) {
	return f(ctx)
}

func newCacheTestCache(t *testing.T, dir string, clock *cacheTestClock) *SnapshotCache {
	t.Helper()
	cache, err := NewSnapshotCache(dir, clock)
	if err != nil {
		t.Fatalf("NewSnapshotCache: %v", err)
	}
	return cache
}

func cacheTestRequest(source, raw string) RefreshRequest {
	return RefreshRequest{
		Source: source, Endpoint: "https://public.example/api/pricing", Currency: "CNY",
		Transport: transportFunc(func(context.Context, string) ([]byte, error) { return []byte(raw), nil }),
		StatusSource: statusSourceFunc(func(context.Context) (modelbilling.PricingSourceStatus, error) {
			return cacheTestStatus(), nil
		}),
	}
}

func cacheTestStatus() modelbilling.PricingSourceStatus {
	version := "fixture-v1"
	return modelbilling.PricingSourceStatus{
		USDExchangeRate: "7.2", RechargePrice: "1.1", FetchedAt: cacheTestStart.Add(-time.Hour),
		SourceVersion: &version,
	}
}

func cacheTestPricing(ratio string) string {
	return fmt.Sprintf(`{
		"success":true,
		"pricing_version":"pricing-v1",
		"group_ratio":{"vip":0.5,"default":1},
		"usable_group":{"vip":"VIP","default":"Default"},
		"data":[{
			"model_name":"model-a","quota_type":0,"model_ratio":%s,"completion_ratio":2,
			"cache_ratio":0.1,"enable_groups":["vip","default"],
			"supported_endpoint_types":["anthropic","openai"]
		}]
	}`, ratio)
}

func mustRefreshCache(t *testing.T, cache *SnapshotCache, request RefreshRequest) CachedSnapshot {
	t.Helper()
	got, err := cache.Refresh(context.Background(), request)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return got
}

func cacheFilePath(t *testing.T, cache *SnapshotCache, source string) string {
	t.Helper()
	path, err := cache.path(source)
	if err != nil {
		t.Fatalf("cache path: %v", err)
	}
	return path
}

func TestSnapshotCacheWritesAndReopensCompleteSnapshot(t *testing.T) {
	dir := t.TempDir()
	clock := &cacheTestClock{now: cacheTestStart}
	first := newCacheTestCache(t, dir, clock)
	written := mustRefreshCache(t, first, cacheTestRequest("domestic", cacheTestPricing("1")))

	reopened := newCacheTestCache(t, dir, clock)
	got, err := reopened.Read("domestic")
	if err != nil {
		t.Fatalf("Read after reopen: %v", err)
	}
	if got.Snapshot.SnapshotID != written.Snapshot.SnapshotID || got.Snapshot.SourceHash != written.Snapshot.SourceHash {
		t.Fatalf("reopened identity = %q/%q", got.Snapshot.SnapshotID, got.Snapshot.SourceHash)
	}
	if got.Provenance.Pricing.Authenticity != SourceReady || got.Provenance.Status.Authenticity != SourceMockOnly {
		t.Fatalf("provenance = %+v", got.Provenance)
	}
	info, err := os.Stat(cacheFilePath(t, first, "domestic"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("cache mode = %o", info.Mode().Perm())
	}
}

func TestDefaultCacheDirUsesApplicationCacheRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KWU_CACHE_HOME", root)
	if got, want := DefaultCacheDir(), filepath.Join(root, "pricing"); got != want {
		t.Fatalf("DefaultCacheDir = %q, want %q", got, want)
	}
}

func TestSnapshotCacheRejectsUnknownVersionFieldsAndBrokenFiles(t *testing.T) {
	clock := &cacheTestClock{now: cacheTestStart}
	seedDir := t.TempDir()
	seed := newCacheTestCache(t, seedDir, clock)
	mustRefreshCache(t, seed, cacheTestRequest("domestic", cacheTestPricing("1")))
	valid, err := os.ReadFile(cacheFilePath(t, seed, "domestic"))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string][]byte{
		"unknown version": []byte(strings.Replace(string(valid), `"version": 1`, `"version": 99`, 1)),
		"unknown field":   []byte(strings.Replace(string(valid), `"version": 1,`, `"version": 1, "secret": true,`, 1)),
		"corrupt":         []byte(`not-json`),
		"truncated":       valid[:len(valid)/2],
		"empty":           {},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cache := newCacheTestCache(t, dir, clock)
			path := cacheFilePath(t, cache, "domestic")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := cache.Read("domestic")
			if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable {
				t.Fatalf("Read error = %v", err)
			}
		})
	}
}

func TestSnapshotCacheRejectsSnapshotContractViolation(t *testing.T) {
	dir := t.TempDir()
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, dir, clock)
	mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
	path := cacheFilePath(t, cache, "domestic")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	record, err := decodeCacheFile(data)
	if err != nil {
		t.Fatal(err)
	}
	record.Snapshot.Currency = ""
	data, err = encodeCacheFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	reopened := newCacheTestCache(t, dir, clock)
	if _, err := reopened.Read("domestic"); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable {
		t.Fatalf("Read invalid snapshot error = %v", err)
	}
}

func TestSnapshotCacheAtomicReplacementAndFailurePreserveOldCache(t *testing.T) {
	dir := t.TempDir()
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, dir, clock)
	first := mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
	path := cacheFilePath(t, cache, "domestic")
	oldBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	second := mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("3")))
	if second.Snapshot.SourceHash == first.Snapshot.SourceHash {
		t.Fatal("replacement did not change content hash")
	}
	newBytes, err := os.ReadFile(path)
	if err != nil || string(newBytes) == string(oldBytes) {
		t.Fatalf("replacement bytes unchanged: %v", err)
	}

	stableBytes := append([]byte(nil), newBytes...)
	stableHash := second.Snapshot.SourceHash
	cache.encode = func(cacheFile) ([]byte, error) { return nil, errors.New("encode failed") }
	got, err := cache.Refresh(context.Background(), cacheTestRequest("domestic", cacheTestPricing("4")))
	if err == nil || got.Snapshot.SourceHash != stableHash {
		t.Fatalf("encode fallback = hash %q, err %v", got.Snapshot.SourceHash, err)
	}
	assertCacheUnchanged(t, path, stableBytes)

	cache.encode = encodeCacheFile
	cache.writeFile = func(string, []byte, os.FileMode) error { return errors.New("write failed") }
	got, err = cache.Refresh(context.Background(), cacheTestRequest("domestic", cacheTestPricing("5")))
	if err == nil || got.Snapshot.SourceHash != stableHash {
		t.Fatalf("write fallback = hash %q, err %v", got.Snapshot.SourceHash, err)
	}
	assertCacheUnchanged(t, path, stableBytes)
	assertOnlyCacheFile(t, filepath.Dir(path), filepath.Base(path))
}

func assertCacheUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		t.Fatalf("cache changed after failed update: %v", err)
	}
}

func assertOnlyCacheFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		t.Fatalf("cache directory entries = %+v", entries)
	}
}

func TestSnapshotCacheRecomputesStateAtExactBoundaries(t *testing.T) {
	dir := t.TempDir()
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, dir, clock)
	mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
	tests := []struct {
		at   time.Time
		want modelbilling.PricingState
	}{
		{cacheTestStart.Add(11*time.Hour + 59*time.Minute + 59*time.Second), modelbilling.PricingFresh},
		{cacheTestStart.Add(12 * time.Hour), modelbilling.PricingStaleUsable},
		{cacheTestStart.Add(23*time.Hour + 59*time.Minute + 59*time.Second), modelbilling.PricingStaleUsable},
		{cacheTestStart.Add(24 * time.Hour), modelbilling.PricingStaleBlocked},
	}
	for _, test := range tests {
		clock.Set(test.at)
		got, err := cache.Read("domestic")
		if err != nil || got.Snapshot.State != test.want {
			t.Fatalf("Read at %v = %q, %v", test.at, got.Snapshot.State, err)
		}
		if shared := DefaultPricingStateAt(cacheTestStart, test.at); shared != test.want {
			t.Fatalf("DefaultPricingStateAt at %v = %q, want %q", test.at, shared, test.want)
		}
	}
}

func TestSnapshotCacheAllowsTestOnlyPolicyInjection(t *testing.T) {
	clock := &cacheTestClock{now: cacheTestStart}
	cache, err := newSnapshotCache(t.TempDir(), clock, refreshPolicy{FreshFor: 2 * time.Hour, StaleFor: 3 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	got := mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
	if !got.Snapshot.FreshUntil.Equal(cacheTestStart.Add(2*time.Hour)) ||
		!got.Snapshot.StaleUntil.Equal(cacheTestStart.Add(3*time.Hour)) {
		t.Fatalf("injected policy bounds = %v/%v", got.Snapshot.FreshUntil, got.Snapshot.StaleUntil)
	}
}

func TestSnapshotCacheReadReturnsDeepCopies(t *testing.T) {
	dir := t.TempDir()
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, dir, clock)
	first := mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
	*first.Snapshot.SourceVersion = "changed"
	*first.Snapshot.ExchangeRate = "9"
	first.Snapshot.GroupRatios["default"] = "9"
	first.Snapshot.UsableGroups[0] = "changed"
	*first.Snapshot.Models[0].ModelRatio = "9"
	first.Snapshot.Models[0].Capabilities[0] = "changed"
	first.Snapshot.Models[0].EnableGroups[0] = "changed"
	*first.Provenance.Status.SourceVersion = "changed"

	got, err := cache.Read("domestic")
	if err != nil {
		t.Fatal(err)
	}
	if *got.Snapshot.SourceVersion != "pricing-v1" || *got.Snapshot.ExchangeRate != "7.2" ||
		got.Snapshot.GroupRatios["default"] != "1" || got.Snapshot.UsableGroups[0] != "default" ||
		*got.Snapshot.Models[0].ModelRatio != "1" || got.Snapshot.Models[0].Capabilities[0] != "text" ||
		got.Snapshot.Models[0].EnableGroups[0] != "vip" || *got.Provenance.Status.SourceVersion != "fixture-v1" {
		t.Fatalf("cached value was aliased: %+v", got)
	}
}

func TestSnapshotCacheFileContainsNoSensitiveFields(t *testing.T) {
	dir := t.TempDir()
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, dir, clock)
	mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
	data, err := os.ReadFile(cacheFilePath(t, cache, "domestic"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	if !strings.Contains(lower, `"model_ratio": "1"`) {
		t.Fatal("decimal price was not persisted as a JSON string")
	}
	for _, forbidden := range []string{
		"access_token", "refresh_token", "api_key", "profile_id", "cloud_user_id",
		"binding_generation", "entitlement", "plan", "task_id", "call_id",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("cache contains forbidden field %q", forbidden)
		}
	}
}
