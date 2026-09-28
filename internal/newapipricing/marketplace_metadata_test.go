package newapipricing

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMarketplaceMetadataCacheWritesStrictMetadataOnlySidecar(t *testing.T) {
	dir := t.TempDir()
	snapshots := newCacheTestCache(t, dir, &cacheTestClock{now: cacheTestStart})
	result, err := snapshots.RefreshWithCatalog(context.Background(), cacheTestRequest("domestic", cacheTestCatalogPricing()))
	if err != nil || result.Catalog == nil {
		t.Fatalf("RefreshWithCatalog = %+v, %v", result, err)
	}
	metadata, err := ProjectMarketplaceMetadata(result.CachedSnapshot, *result.Catalog)
	if err != nil {
		t.Fatalf("ProjectMarketplaceMetadata: %v", err)
	}
	cache, err := NewMarketplaceMetadataCache(dir)
	if err != nil {
		t.Fatalf("NewMarketplaceMetadataCache: %v", err)
	}
	if err := cache.Write(metadata); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := cache.Read("domestic", result.CachedSnapshot)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.SourceID != result.CachedSnapshot.Snapshot.Source ||
		got.SnapshotID != result.CachedSnapshot.Snapshot.SnapshotID ||
		got.SourceHash != result.CachedSnapshot.Snapshot.SourceHash {
		t.Fatalf("metadata identity = %+v", got)
	}
	if len(got.Models) != 2 || got.Models[0].Vendor != "Vendor" || got.Models[0].VendorIcon != "vendor.svg" {
		t.Fatalf("metadata models = %+v", got.Models)
	}
	if len(got.Issues) != 1 || got.Issues[0].Code != "contract_invalid" || got.Issues[0].ModelID != "model-invalid" {
		t.Fatalf("metadata issues = %+v", got.Issues)
	}

	data, err := os.ReadFile(cacheFilePathForMetadata(t, cache, "domestic"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{
		"model_ratio", "completion_ratio", "cache_ratio", "group_ratio", "exchange_rate",
		"recharge_price", "currency", "model_price", "pricing_evidence", "credit_config",
		"raw", "token", "api_key", "authorization", "bearer", "response_body", `"index"`,
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("sidecar contains forbidden field %q: %s", forbidden, data)
		}
	}
	if strings.Contains(lower, "model type cannot be mapped") {
		t.Fatalf("sidecar contains raw mapping error: %s", data)
	}
}

func TestProjectMarketplaceMetadataKeepsAnonymousMappingIssueAsStableCode(t *testing.T) {
	dir := t.TempDir()
	snapshots := newCacheTestCache(t, dir, &cacheTestClock{now: cacheTestStart})
	raw := `{
		"success":true,
		"pricing_version":"pricing-v1",
		"group_ratio":{"default":1},
		"usable_group":{"default":"Default"},
		"data":[
			{"model_name":" ","quota_type":0},
			{"model_name":"model-a","quota_type":0,"model_ratio":1,"completion_ratio":2,"enable_groups":["default"],"supported_endpoint_types":["openai"]}
		]
	}`
	result, err := snapshots.RefreshWithCatalog(context.Background(), cacheTestRequest("domestic", raw))
	if err != nil || result.Catalog == nil {
		t.Fatalf("RefreshWithCatalog = %+v, %v", result, err)
	}

	metadata, err := ProjectMarketplaceMetadata(result.CachedSnapshot, *result.Catalog)
	if err != nil {
		t.Fatalf("ProjectMarketplaceMetadata: %v", err)
	}
	if len(metadata.Issues) != 1 || metadata.Issues[0].ModelID != "" ||
		metadata.Issues[0].Code != "contract_invalid" {
		t.Fatalf("anonymous issue = %+v", metadata.Issues)
	}
}

func TestMarketplaceMetadataCacheRejectsStrictDecodeAndIdentityMismatch(t *testing.T) {
	dir := t.TempDir()
	snapshots := newCacheTestCache(t, dir, &cacheTestClock{now: cacheTestStart})
	result, err := snapshots.RefreshWithCatalog(context.Background(), cacheTestRequest("domestic", cacheTestCatalogPricing()))
	if err != nil || result.Catalog == nil {
		t.Fatalf("RefreshWithCatalog = %+v, %v", result, err)
	}
	metadata, err := ProjectMarketplaceMetadata(result.CachedSnapshot, *result.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := NewMarketplaceMetadataCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := encodeMarketplaceMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string][]byte{
		"unknown version":   []byte(strings.Replace(string(data), `"version": 1`, `"version": 2`, 1)),
		"unknown field":     []byte(strings.Replace(string(data), `"version": 1,`, `"version": 1, "price": "1",`, 1)),
		"multiple values":   append(append([]byte(nil), data...), []byte(`{"version":1}`)...),
		"source mismatch":   []byte(strings.Replace(string(data), `"source_id": "domestic"`, `"source_id": "international"`, 1)),
		"snapshot mismatch": []byte(strings.Replace(string(data), metadata.SnapshotID, "pricing-v1:domestic:old", 1)),
		"hash mismatch":     []byte(strings.Replace(string(data), metadata.SourceHash, "sha256:old", 1)),
	}
	for name, invalid := range tests {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(cacheFilePathForMetadata(t, cache, "domestic"), invalid, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := cache.Read("domestic", result.CachedSnapshot); MarketplaceMetadataErrorCodeOf(err) != MetadataUnavailable {
				t.Fatalf("Read error = %v", err)
			}
		})
	}
}

func TestMarketplaceMetadataCacheAtomicReplacementAndFailurePreserveOldSidecar(t *testing.T) {
	dir := t.TempDir()
	snapshots := newCacheTestCache(t, dir, &cacheTestClock{now: cacheTestStart})
	result, err := snapshots.RefreshWithCatalog(context.Background(), cacheTestRequest("domestic", cacheTestCatalogPricing()))
	if err != nil || result.Catalog == nil {
		t.Fatalf("RefreshWithCatalog = %+v, %v", result, err)
	}
	metadata, err := ProjectMarketplaceMetadata(result.CachedSnapshot, *result.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := NewMarketplaceMetadataCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Write(metadata); err != nil {
		t.Fatal(err)
	}
	path := cacheFilePathForMetadata(t, cache, "domestic")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	metadata.Models[0].Vendor = "Updated Vendor"
	if err := cache.Write(metadata); err != nil {
		t.Fatalf("replace sidecar: %v", err)
	}
	updated, err := cache.Read("domestic", result.CachedSnapshot)
	if err != nil || updated.Models[0].Vendor != "Updated Vendor" {
		t.Fatalf("updated metadata = %+v, %v", updated, err)
	}

	stable, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(stable) == string(first) {
		t.Fatal("atomic replacement did not change sidecar")
	}
	cache.writeFile = func(string, []byte, os.FileMode) error { return errors.New("write failed") }
	metadata.Models[0].Vendor = "Must Not Persist"
	if err := cache.Write(metadata); MarketplaceMetadataErrorCodeOf(err) != MetadataWriteFailed {
		t.Fatalf("Write error = %v", err)
	}
	afterFailure, err := os.ReadFile(path)
	if err != nil || string(afterFailure) != string(stable) {
		t.Fatalf("sidecar changed after failed write: %v", err)
	}
}

func cacheFilePathForMetadata(t *testing.T, cache *MarketplaceMetadataCache, source string) string {
	t.Helper()
	path, err := cache.path(source)
	if err != nil {
		t.Fatalf("metadata path: %v", err)
	}
	return path
}
