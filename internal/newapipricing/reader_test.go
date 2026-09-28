package newapipricing

import (
	"context"
	"sync/atomic"
	"testing"

	"kwu/internal/modelbilling"
)

func TestReaderReadIsCacheOnlyAndReturnsDeepCopy(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	seeded := mustRefreshCache(t, cache, request)
	reader, err := NewReader(cache)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	first, err := reader.Read("domestic")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	first.Snapshot.GroupRatios["default"] = "9"
	*first.Snapshot.Models[0].ModelRatio = "9"
	second, err := reader.Read("domestic")
	if err != nil {
		t.Fatalf("Read again: %v", err)
	}
	if second.Snapshot.SnapshotID != seeded.Snapshot.SnapshotID || second.Snapshot.GroupRatios["default"] != "1" ||
		second.Snapshot.Models[0].ModelRatio == nil || *second.Snapshot.Models[0].ModelRatio != "1" {
		t.Fatalf("reader returned aliased cache: %+v", second)
	}
}

func TestReaderRefreshUsesSnapshotCacheRefreshWithCatalogOnce(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	reader, err := NewReader(cache)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var transportCalls atomic.Int32
	var statusCalls atomic.Int32
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		transportCalls.Add(1)
		return []byte(cacheTestPricing("1")), nil
	})
	request.StatusSource = statusSourceFunc(func(context.Context) (modelbilling.PricingSourceStatus, error) {
		statusCalls.Add(1)
		return cacheTestStatus(), nil
	})

	result, err := reader.Refresh(context.Background(), request)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if result.Catalog == nil || result.CachedSnapshot.Snapshot.SnapshotID == "" {
		t.Fatalf("Refresh result = %+v", result)
	}
	if transportCalls.Load() != 1 || statusCalls.Load() != 1 {
		t.Fatalf("calls = transport %d, status %d", transportCalls.Load(), statusCalls.Load())
	}
}
