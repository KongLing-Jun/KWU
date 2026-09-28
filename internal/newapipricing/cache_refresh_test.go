package newapipricing

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/modelbilling"
)

func TestSnapshotCacheFirstOfflineReturnsPricingUnavailable(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		return nil, errors.New("offline")
	})
	got, err := cache.Refresh(context.Background(), request)
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable || got.Snapshot.State != "" {
		t.Fatalf("first offline refresh = %+v, %v", got, err)
	}
}

func TestSnapshotCacheRefreshWithCatalogReturnsPersistedAdaptResult(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
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

	got, err := cache.RefreshWithCatalog(context.Background(), request)
	if err != nil {
		t.Fatalf("RefreshWithCatalog: %v", err)
	}
	if got.Catalog == nil {
		t.Fatal("RefreshWithCatalog returned nil Catalog")
	}
	if got.CachedSnapshot.Snapshot.Source != request.Source ||
		got.CachedSnapshot.Snapshot.SourceVersion == nil || got.Catalog.SourceVersion == nil ||
		*got.CachedSnapshot.Snapshot.SourceVersion != *got.Catalog.SourceVersion ||
		len(got.CachedSnapshot.Snapshot.Models) != 1 || len(got.Catalog.Prices) != 1 ||
		got.CachedSnapshot.Snapshot.Models[0].ModelID != got.Catalog.Prices[0].ModelID {
		t.Fatalf("Catalog and snapshot differ: %+v / %+v", got.Catalog, got.CachedSnapshot.Snapshot)
	}
	if got.CachedSnapshot.Snapshot.SnapshotID == "" || got.CachedSnapshot.Snapshot.SourceHash == "" {
		t.Fatalf("snapshot identity is incomplete: %+v", got.CachedSnapshot.Snapshot)
	}
	if transportCalls.Load() != 1 || statusCalls.Load() != 1 {
		t.Fatalf("calls = transport %d, status %d", transportCalls.Load(), statusCalls.Load())
	}
	if persisted, readErr := cache.Read(request.Source); readErr != nil ||
		persisted.Snapshot.SnapshotID != got.CachedSnapshot.Snapshot.SnapshotID ||
		persisted.Snapshot.SourceHash != got.CachedSnapshot.Snapshot.SourceHash {
		t.Fatalf("persisted snapshot = %+v, %v", persisted, readErr)
	}
}

func TestSnapshotCacheRefreshFailureFallbackDoesNotMoveTimeBounds(t *testing.T) {
	dir := t.TempDir()
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, dir, clock)
	original := mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
	failing := cacheTestRequest("domestic", cacheTestPricing("1"))
	failing.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		return nil, errors.New("refresh failed")
	})
	tests := []struct {
		name string
		at   time.Time
		want modelbilling.PricingState
	}{
		{"fresh", cacheTestStart.Add(time.Hour), modelbilling.PricingFresh},
		{"stale", cacheTestStart.Add(12 * time.Hour), modelbilling.PricingStaleUsable},
		{"blocked", cacheTestStart.Add(24 * time.Hour), modelbilling.PricingStaleBlocked},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clock.Set(test.at)
			got, err := cache.Refresh(context.Background(), failing)
			if err == nil || got.Snapshot.State != test.want {
				t.Fatalf("fallback = state %q, err %v", got.Snapshot.State, err)
			}
			if !got.Snapshot.FetchedAt.Equal(original.Snapshot.FetchedAt) ||
				!got.Snapshot.FreshUntil.Equal(original.Snapshot.FreshUntil) ||
				!got.Snapshot.StaleUntil.Equal(original.Snapshot.StaleUntil) {
				t.Fatalf("fallback moved time bounds: %+v", got.Snapshot)
			}
		})
	}
}

func TestSnapshotCacheCoalescesConcurrentRefreshBySource(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return []byte(cacheTestPricing("1")), nil
	})

	const waiters = 12
	begin := make(chan struct{})
	results := make(chan CachedSnapshot, waiters)
	errorsFound := make(chan error, waiters)
	var ready sync.WaitGroup
	ready.Add(waiters)
	for range waiters {
		go func() {
			ready.Done()
			<-begin
			value, err := cache.Refresh(context.Background(), request)
			results <- value
			errorsFound <- err
		}()
	}
	ready.Wait()
	close(begin)
	<-started
	time.Sleep(30 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("transport calls while shared refresh is blocked = %d", got)
	}
	close(release)
	var snapshotID string
	for range waiters {
		if err := <-errorsFound; err != nil {
			t.Fatalf("shared refresh error: %v", err)
		}
		got := <-results
		if snapshotID == "" {
			snapshotID = got.Snapshot.SnapshotID
		}
		if got.Snapshot.SnapshotID != snapshotID {
			t.Fatalf("shared snapshot IDs differ: %q/%q", snapshotID, got.Snapshot.SnapshotID)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("transport calls = %d", got)
	}
}

func TestSnapshotCacheMixedRefreshMethodsShareOneFlight(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	started := make(chan struct{})
	release := make(chan struct{})
	var transportCalls atomic.Int32
	var statusCalls atomic.Int32
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		transportCalls.Add(1)
		close(started)
		<-release
		return []byte(cacheTestPricing("1")), nil
	})
	request.StatusSource = statusSourceFunc(func(context.Context) (modelbilling.PricingSourceStatus, error) {
		statusCalls.Add(1)
		return cacheTestStatus(), nil
	})

	detailedResult := make(chan RefreshResult, 1)
	detailedError := make(chan error, 1)
	go func() {
		value, err := cache.RefreshWithCatalog(context.Background(), request)
		detailedResult <- value
		detailedError <- err
	}()
	<-started

	legacyResult := make(chan CachedSnapshot, 1)
	legacyError := make(chan error, 1)
	go func() {
		value, err := cache.Refresh(context.Background(), request)
		legacyResult <- value
		legacyError <- err
	}()
	waitForRefreshWaiters(t, cache, request.Source, 2)
	close(release)

	detailed := <-detailedResult
	legacy := <-legacyResult
	if err := <-detailedError; err != nil {
		t.Fatalf("RefreshWithCatalog: %v", err)
	}
	if err := <-legacyError; err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if detailed.Catalog == nil || detailed.CachedSnapshot.Snapshot.SnapshotID != legacy.Snapshot.SnapshotID {
		t.Fatalf("mixed results = %+v / %+v", detailed, legacy)
	}
	if transportCalls.Load() != 1 || statusCalls.Load() != 1 {
		t.Fatalf("calls = transport %d, status %d", transportCalls.Load(), statusCalls.Load())
	}
}

func TestSnapshotCacheRefreshWithCatalogReturnsDeepCopiesPerWaiter(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	started := make(chan struct{})
	release := make(chan struct{})
	request := cacheTestRequest("domestic", cacheTestCatalogPricing())
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		close(started)
		<-release
		return []byte(cacheTestCatalogPricing()), nil
	})

	results := make(chan RefreshResult, 2)
	errorsFound := make(chan error, 2)
	go func() {
		value, err := cache.RefreshWithCatalog(context.Background(), request)
		results <- value
		errorsFound <- err
	}()
	<-started
	go func() {
		value, err := cache.RefreshWithCatalog(context.Background(), request)
		results <- value
		errorsFound <- err
	}()
	waitForRefreshWaiters(t, cache, request.Source, 2)
	close(release)

	first := <-results
	second := <-results
	for range 2 {
		if err := <-errorsFound; err != nil {
			t.Fatalf("RefreshWithCatalog: %v", err)
		}
	}
	if first.Catalog == nil || second.Catalog == nil {
		t.Fatalf("nil Catalog = first %v, second %v", first.Catalog == nil, second.Catalog == nil)
	}

	*first.Catalog.SourceVersion = "changed"
	first.Catalog.GroupRatios["default"] = "9"
	first.Catalog.UsableGroups[0] = "changed"
	*first.Catalog.Models[0].VendorID = 99
	first.Catalog.Models[0].Vendor.Name = "changed"
	first.Catalog.Models[0].EnableGroups[0] = "changed"
	first.Catalog.Models[0].SupportedEndpointTypes[0] = "changed"
	*first.Catalog.Models[0].ModelRatio = "9"
	first.Catalog.Prices[0].Capabilities[0] = "changed"
	first.Catalog.Prices[0].EnableGroups[0] = "changed"
	*first.Catalog.Prices[0].ModelRatio = "9"
	if issueErr, ok := first.Catalog.Issues[0].Err.(*modelbilling.Error); ok {
		issueErr.Message = "changed"
	} else {
		t.Fatalf("issue error type = %T", first.Catalog.Issues[0].Err)
	}
	*first.CachedSnapshot.Snapshot.SourceVersion = "changed"
	*first.CachedSnapshot.Snapshot.ExchangeRate = "9"
	first.CachedSnapshot.Snapshot.GroupRatios["default"] = "9"
	first.CachedSnapshot.Snapshot.UsableGroups[0] = "changed"
	first.CachedSnapshot.Snapshot.Models[0].Capabilities[0] = "changed"
	first.CachedSnapshot.Snapshot.Models[0].EnableGroups[0] = "changed"
	*first.CachedSnapshot.Snapshot.Models[0].ModelRatio = "9"
	*first.CachedSnapshot.Provenance.Status.SourceVersion = "changed"

	assertUnchangedRefreshResult(t, second)
	persisted, err := cache.Read(request.Source)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if *persisted.Snapshot.SourceVersion != "pricing-v1" || *persisted.Snapshot.ExchangeRate != "7.2" ||
		persisted.Snapshot.GroupRatios["default"] != "1" || persisted.Snapshot.UsableGroups[0] != "default" ||
		persisted.Snapshot.Models[0].Capabilities[0] != "text" || persisted.Snapshot.Models[0].EnableGroups[0] != "vip" ||
		*persisted.Snapshot.Models[0].ModelRatio != "1" || *persisted.Provenance.Status.SourceVersion != "fixture-v1" {
		t.Fatalf("cached snapshot was aliased: %+v", persisted)
	}
}

func TestSnapshotCacheRefreshWithCatalogFailuresNeverExposeCatalog(t *testing.T) {
	tests := map[string]func(*SnapshotCache, *RefreshRequest) context.Context{
		"transport": func(_ *SnapshotCache, request *RefreshRequest) context.Context {
			request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
				return nil, errors.New("transport failed")
			})
			return context.Background()
		},
		"status": func(_ *SnapshotCache, request *RefreshRequest) context.Context {
			request.StatusSource = statusSourceFunc(func(context.Context) (modelbilling.PricingSourceStatus, error) {
				return modelbilling.PricingSourceStatus{}, errors.New("status failed")
			})
			return context.Background()
		},
		"adapt": func(_ *SnapshotCache, request *RefreshRequest) context.Context {
			request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
				return []byte(`{"success":true,"data":`), nil
			})
			return context.Background()
		},
		"persist": func(cache *SnapshotCache, _ *RefreshRequest) context.Context {
			cache.writeFile = func(string, []byte, os.FileMode) error { return errors.New("persist failed") }
			return context.Background()
		},
		"context": func(_ *SnapshotCache, _ *RefreshRequest) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		},
	}
	for name, arrange := range tests {
		t.Run(name, func(t *testing.T) {
			cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
			request := cacheTestRequest("domestic", cacheTestPricing("1"))
			ctx := arrange(cache, &request)
			result, err := cache.RefreshWithCatalog(ctx, request)
			if err == nil {
				t.Fatalf("RefreshWithCatalog succeeded: %+v", result)
			}
			if result.Catalog != nil {
				t.Fatalf("failure exposed Catalog: %+v", result.Catalog)
			}
		})
	}
}

func TestSnapshotCacheRefreshWithCatalogContextFailureAfterAdaptDoesNotPersistCatalog(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	ctx, cancel := context.WithCancel(context.Background())
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.StatusSource = statusSourceFunc(func(sharedContext context.Context) (modelbilling.PricingSourceStatus, error) {
		cancel()
		<-sharedContext.Done()
		return cacheTestStatus(), nil
	})

	result, err := cache.RefreshWithCatalog(ctx, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RefreshWithCatalog error = %v", err)
	}
	if result.Catalog != nil {
		t.Fatalf("cancelled refresh exposed Catalog: %+v", result.Catalog)
	}
	waitForNoFlight(t, cache, request.Source)
	if cached, readErr := cache.Read(request.Source); modelbilling.ErrorCodeOf(readErr) != modelbilling.ErrorPricingUnavailable {
		t.Fatalf("cancelled refresh persisted cache: %+v, %v", cached, readErr)
	}
}

func TestSnapshotCacheRefreshWithCatalogFallbackPreservesErrorWithoutCatalog(t *testing.T) {
	tests := map[string]func(*SnapshotCache, *RefreshRequest, error){
		"transport": func(_ *SnapshotCache, request *RefreshRequest, wantErr error) {
			request.Transport = transportFunc(func(context.Context, string) ([]byte, error) { return nil, wantErr })
		},
		"persist": func(cache *SnapshotCache, _ *RefreshRequest, wantErr error) {
			cache.writeFile = func(string, []byte, os.FileMode) error { return wantErr }
		},
	}
	for name, arrange := range tests {
		t.Run(name, func(t *testing.T) {
			cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
			original := mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricing("1")))
			wantErr := errors.New("refresh failed")
			request := cacheTestRequest("domestic", cacheTestPricing("2"))
			arrange(cache, &request, wantErr)

			result, err := cache.RefreshWithCatalog(context.Background(), request)
			if !errors.Is(err, wantErr) {
				t.Fatalf("fallback error = %v, want %v", err, wantErr)
			}
			if result.Catalog != nil {
				t.Fatalf("fallback exposed Catalog: %+v", result.Catalog)
			}
			if result.CachedSnapshot.Snapshot.SnapshotID != original.Snapshot.SnapshotID ||
				result.CachedSnapshot.Snapshot.SourceHash != original.Snapshot.SourceHash {
				t.Fatalf("fallback snapshot = %+v, want %+v", result.CachedSnapshot.Snapshot, original.Snapshot)
			}
		})
	}
}

func waitForRefreshWaiters(t *testing.T, cache *SnapshotCache, source string, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		cache.mu.Lock()
		flight := cache.flights[source]
		got := 0
		if flight != nil {
			got = flight.waiters
		}
		cache.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("refresh waiters did not reach %d", want)
}

func waitForNoFlight(t *testing.T, cache *SnapshotCache, source string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		cache.mu.Lock()
		_, exists := cache.flights[source]
		cache.mu.Unlock()
		if !exists {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("refresh remained permanently in flight")
}

func cacheTestCatalogPricing() string {
	return `{
		"success":true,
		"pricing_version":"pricing-v1",
		"vendors":[{"id":7,"name":"Vendor","description":"Vendor description","icon":"vendor.svg"}],
		"group_ratio":{"vip":0.5,"default":1},
		"usable_group":{"vip":"VIP","default":"Default"},
		"data":[
			{
				"model_name":"model-a","description":"Model A","vendor_id":7,
				"quota_type":0,"model_ratio":1,"completion_ratio":2,"cache_ratio":0.1,
				"enable_groups":["vip","default"],"supported_endpoint_types":["anthropic","openai"]
			},
			{
				"model_name":"model-invalid","description":"Invalid model","vendor_id":7,
				"quota_type":0,"model_ratio":3,"enable_groups":["default"],
				"supported_endpoint_types":["unknown"]
			}
		]
	}`
}

func assertUnchangedRefreshResult(t *testing.T, result RefreshResult) {
	t.Helper()
	issueErr, ok := result.Catalog.Issues[0].Err.(*modelbilling.Error)
	if !ok {
		t.Fatalf("issue error type = %T", result.Catalog.Issues[0].Err)
	}
	if *result.Catalog.SourceVersion != "pricing-v1" || result.Catalog.GroupRatios["default"] != "1" ||
		result.Catalog.UsableGroups[0] != "default" || *result.Catalog.Models[0].VendorID != 7 ||
		result.Catalog.Models[0].Vendor.Name != "Vendor" || result.Catalog.Models[0].EnableGroups[0] != "vip" ||
		result.Catalog.Models[0].SupportedEndpointTypes[0] != "anthropic" || *result.Catalog.Models[0].ModelRatio != "1" ||
		result.Catalog.Prices[0].Capabilities[0] != "text" || result.Catalog.Prices[0].EnableGroups[0] != "vip" ||
		*result.Catalog.Prices[0].ModelRatio != "1" || issueErr.Message == "changed" {
		t.Fatalf("Catalog was aliased: %+v", result.Catalog)
	}
	if *result.CachedSnapshot.Snapshot.SourceVersion != "pricing-v1" ||
		*result.CachedSnapshot.Snapshot.ExchangeRate != "7.2" ||
		result.CachedSnapshot.Snapshot.GroupRatios["default"] != "1" ||
		result.CachedSnapshot.Snapshot.UsableGroups[0] != "default" ||
		result.CachedSnapshot.Snapshot.Models[0].Capabilities[0] != "text" ||
		result.CachedSnapshot.Snapshot.Models[0].EnableGroups[0] != "vip" ||
		*result.CachedSnapshot.Snapshot.Models[0].ModelRatio != "1" ||
		*result.CachedSnapshot.Provenance.Status.SourceVersion != "fixture-v1" {
		t.Fatalf("CachedSnapshot was aliased: %+v", result.CachedSnapshot)
	}
}

func TestSnapshotCacheDifferentSourcesRefreshIndependently(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	startedA, startedB := make(chan struct{}), make(chan struct{})
	releaseA, releaseB := make(chan struct{}), make(chan struct{})
	requestA := blockingCacheRequest("domestic", startedA, releaseA)
	requestB := blockingCacheRequest("international", startedB, releaseB)

	result := make(chan error, 2)
	go func() { _, err := cache.Refresh(context.Background(), requestA); result <- err }()
	go func() { _, err := cache.Refresh(context.Background(), requestB); result <- err }()
	select {
	case <-startedA:
	case <-time.After(time.Second):
		t.Fatal("domestic refresh did not start")
	}
	select {
	case <-startedB:
	case <-time.After(time.Second):
		t.Fatal("international refresh was blocked by domestic refresh")
	}
	close(releaseA)
	close(releaseB)
	for range 2 {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	for _, source := range []string{"domestic", "international"} {
		if got, err := cache.Read(source); err != nil || got.Snapshot.Source != source {
			t.Fatalf("Read %s = %+v, %v", source, got, err)
		}
	}
}

func blockingCacheRequest(source string, started, release chan struct{}) RefreshRequest {
	request := cacheTestRequest(source, cacheTestPricing("1"))
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		close(started)
		<-release
		return []byte(cacheTestPricing("1")), nil
	})
	return request
}

func TestSnapshotCacheCancelledWaiterDoesNotCancelSharedRefresh(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	started := make(chan struct{})
	release := make(chan struct{})
	request := blockingCacheRequest("domestic", started, release)

	firstResult := make(chan error, 1)
	go func() {
		_, err := cache.Refresh(context.Background(), request)
		firstResult <- err
	}()
	<-started
	cancelled, cancel := context.WithCancel(context.Background())
	secondResult := make(chan error, 1)
	go func() {
		_, err := cache.Refresh(cancelled, request)
		secondResult <- err
	}()
	cancel()
	select {
	case err := <-secondResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled waiter error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter did not return promptly")
	}
	close(release)
	if err := <-firstResult; err != nil {
		t.Fatalf("uncancelled waiter was affected: %v", err)
	}
	if _, err := cache.Read("domestic"); err != nil {
		t.Fatalf("shared refresh did not populate cache: %v", err)
	}
}

func TestSnapshotCacheAllCancelledWaitersReleaseInflightRefresh(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	started := make(chan struct{})
	transportCancelled := make(chan struct{})
	releaseOldTransport := make(chan struct{})
	secondTransportStarted := make(chan struct{})
	closingFlightWait := make(chan struct{})
	cache.onClosingFlightWait = func() { close(closingFlightWait) }
	var calls atomic.Int32
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.Transport = transportFunc(func(ctx context.Context, _ string) ([]byte, error) {
		switch calls.Add(1) {
		case 1:
			close(started)
			<-ctx.Done()
			close(transportCancelled)
			<-releaseOldTransport
			return nil, ctx.Err()
		case 2:
			close(secondTransportStarted)
			return []byte(cacheTestPricing("2")), nil
		default:
			return nil, errors.New("unexpected extra transport call")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() {
		_, err := cache.Refresh(ctx, request)
		firstResult <- err
	}()
	<-started
	cancel()
	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled refresh error = %v", err)
	}
	<-transportCancelled

	cache.mu.Lock()
	oldFlight := cache.flights["domestic"]
	if oldFlight == nil || oldFlight.accepting || oldFlight.waiters != 0 {
		cache.mu.Unlock()
		t.Fatalf("old flight after last waiter = %+v", oldFlight)
	}
	cache.mu.Unlock()

	secondResult := make(chan struct {
		value RefreshResult
		err   error
	}, 1)
	go func() {
		value, err := cache.RefreshWithCatalog(context.Background(), request)
		secondResult <- struct {
			value RefreshResult
			err   error
		}{value: value, err: err}
	}()
	<-closingFlightWait
	if calls.Load() != 1 {
		t.Fatalf("second caller started transport before old flight completed: %d", calls.Load())
	}
	select {
	case result := <-secondResult:
		t.Fatalf("second caller inherited old flight result: %+v", result)
	default:
	}

	close(releaseOldTransport)
	<-secondTransportStarted
	result := <-secondResult
	if result.err != nil {
		t.Fatalf("second refresh error = %v", result.err)
	}
	if calls.Load() != 2 || result.value.Catalog == nil ||
		result.value.CachedSnapshot.Snapshot.Models[0].ModelRatio == nil ||
		*result.value.CachedSnapshot.Snapshot.Models[0].ModelRatio != "2" {
		t.Fatalf("second refresh result = calls %d, result %+v", calls.Load(), result.value)
	}
	got, err := cache.Read("domestic")
	if err != nil || got.Snapshot.SourceHash != result.value.CachedSnapshot.Snapshot.SourceHash {
		t.Fatalf("cached second refresh = %+v, %v", got, err)
	}
	assertNoFlight(t, cache, "domestic")
}

func TestSnapshotCacheWaitingForAbandonedFlightHonorsContext(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	started := make(chan struct{})
	transportCancelled := make(chan struct{})
	releaseTransport := make(chan struct{})
	closingFlightWait := make(chan struct{})
	cache.onClosingFlightWait = func() { close(closingFlightWait) }
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.Transport = transportFunc(func(ctx context.Context, _ string) ([]byte, error) {
		close(started)
		<-ctx.Done()
		close(transportCancelled)
		<-releaseTransport
		return nil, ctx.Err()
	})

	firstContext, cancelFirst := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() {
		_, err := cache.Refresh(firstContext, request)
		firstResult <- err
	}()
	<-started
	cancelFirst()
	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("first cancellation error = %v", err)
	}
	<-transportCancelled
	cache.mu.Lock()
	oldFlight := cache.flights["domestic"]
	cache.mu.Unlock()
	if oldFlight == nil {
		t.Fatal("old flight missing while its transport is held")
	}

	secondContext, cancelSecond := context.WithCancel(context.Background())
	secondResult := make(chan error, 1)
	go func() {
		result, err := cache.RefreshWithCatalog(secondContext, request)
		if result.Catalog != nil {
			secondResult <- errors.New("cancelled closing-flight waiter received Catalog")
			return
		}
		secondResult <- err
	}()
	<-closingFlightWait
	cancelSecond()
	if err := <-secondResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("closing-flight wait cancellation error = %v", err)
	}
	close(releaseTransport)
	<-oldFlight.done
	assertNoFlight(t, cache, "domestic")
}

func assertNoFlight(t *testing.T, cache *SnapshotCache, source string) {
	t.Helper()
	cache.mu.Lock()
	_, exists := cache.flights[source]
	cache.mu.Unlock()
	if exists {
		t.Fatal("refresh remained permanently in flight")
	}
}

func TestSnapshotCacheRefreshFailureAllowsNextAttempt(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	var calls atomic.Int32
	request := cacheTestRequest("domestic", cacheTestPricing("1"))
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("temporary failure")
		}
		return []byte(cacheTestPricing("1")), nil
	})
	if _, err := cache.Refresh(context.Background(), request); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable {
		t.Fatalf("first refresh error = %v", err)
	}
	if _, err := cache.Refresh(context.Background(), request); err != nil {
		t.Fatalf("second refresh error = %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("transport calls = %d", calls.Load())
	}
}
