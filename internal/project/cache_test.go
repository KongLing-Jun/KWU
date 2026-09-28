package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
)

const projectCacheTestHash = "d6e32811441ed5a71c544e7db8e0ee82e4497bfdf5849e71162279e32a3967b3"

func TestProjectCachePersistsVersionedExactScopeAtomically(t *testing.T) {
	now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}
	cache := newProjectCacheForTest(t, t.TempDir(), resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	page := cacheTestPage(t, scope, 2)
	if err := cache.StoreSummaries(context.Background(), scope, page); err != nil {
		t.Fatal(err)
	}
	if err := cache.StoreDetail(context.Background(), scope, page.Items[1]); err != nil {
		t.Fatal(err)
	}

	snapshot, found, err := cache.Load(context.Background(), scope)
	if err != nil || !found || snapshot.CachedAt != now || len(snapshot.Summaries.Items) != 2 || len(snapshot.Details) != 1 || snapshot.Details[0].ProjectID != "project-002" {
		t.Fatalf("Load = %#v, found=%t, err=%v", snapshot, found, err)
	}
	path := cache.path(scope)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{`"schema":"reasonix.project-cache"`, `"environment":`, `"binding":`, `"contract_version":"` + ContractVersion + `"`, `"contract_hash":"` + projectCacheTestHash + `"`, `"cached_at":"2026-08-05T11:00:00Z"`} {
		if !strings.Contains(text, required) {
			t.Errorf("cache document missing %s", required)
		}
	}
	for _, forbidden := range []string{`"instruction"`, `"token"`, `"path"`, `"workspace"`} {
		if strings.Contains(text, forbidden) {
			t.Errorf("cache document contains forbidden field %s", forbidden)
		}
	}
	temporary, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".project-cache-*.tmp"))
	if err != nil || len(temporary) != 0 {
		t.Fatalf("temporary files = %v, err=%v", temporary, err)
	}

	resolver.Set(gatewayTestScope(t, "user-2", 8))
	if _, found, err := cache.Load(context.Background(), resolver.scope); err != nil || found {
		t.Fatalf("other scope Load found=%t err=%v", found, err)
	}
	otherEnvironment, err := cloud.NewEnvironmentIdentity(cloud.EnvironmentFixture, "https://other.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	otherEnvironmentScope, err := NewScope(otherEnvironment, scope.Binding)
	if err != nil {
		t.Fatal(err)
	}
	resolver.Set(otherEnvironmentScope)
	if _, found, err := cache.Load(context.Background(), otherEnvironmentScope); err != nil || found {
		t.Fatalf("other environment Load found=%t err=%v", found, err)
	}
	resolver.Set(scope)
	otherContract := newProjectCacheForTest(t, cache.root, resolver, func() time.Time { return now }, "projects-v2", strings.Repeat("1", 64))
	if _, found, err := otherContract.Load(context.Background(), scope); err != nil || found {
		t.Fatalf("other contract Load found=%t err=%v", found, err)
	}
}

func TestProjectCacheStoreCreatedProjectKeepsServerHeadOrderStable(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	cache := newProjectCacheForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	existing := cacheTestPage(t, scope, 3)
	if err := cache.StoreSummaries(context.Background(), scope, existing); err != nil {
		t.Fatal(err)
	}
	created := Project{
		ProjectID: "project-new", OwnerUserID: scope.Binding.CloudUserID, Name: "New Project",
		Status: ProjectActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := cache.StoreCreatedProject(context.Background(), scope, created); err != nil {
		t.Fatal(err)
	}

	first, found, err := cache.Load(context.Background(), scope)
	if err != nil || !found {
		t.Fatalf("Load after create found=%t err=%v", found, err)
	}
	wantIDs := []string{"project-new", "project-001", "project-002", "project-003"}
	if got := projectIDs(first.Summaries.Items); !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("first snapshot order = %v, want %v", got, wantIDs)
	}
	if len(first.Details) != 1 || first.Details[0].ProjectID != created.ProjectID {
		t.Fatalf("created detail cache = %#v", first.Details)
	}

	authoritative := ProjectPage{Items: append([]Project{created}, existing.Items...), scopeKey: scope.Key}
	if err := cache.StoreSummaries(context.Background(), scope, authoritative); err != nil {
		t.Fatal(err)
	}
	refreshed, found, err := cache.Load(context.Background(), scope)
	if err != nil || !found {
		t.Fatalf("Load after refresh found=%t err=%v", found, err)
	}
	if got := projectIDs(refreshed.Summaries.Items); !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("refreshed snapshot order = %v, want %v", got, wantIDs)
	}
}

func TestProjectCacheJoinedDirectoryReplacesClearsAndKeepsOwnershipSeparate(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	cache := newProjectCacheForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	owned := cacheTestPage(t, scope, 1)
	joinedProject := owned.Items[0]
	joinedProject.ProjectID = "project-joined"
	joinedProject.OwnerUserID = "user-other"
	joinedProject.Name = "Joined"
	joined := ProjectPage{Items: []Project{joinedProject}, scopeKey: scope.Key}

	if err := cache.StoreSummaries(context.Background(), scope, owned); err != nil {
		t.Fatal(err)
	}
	if err := cache.StoreJoinedSummaries(context.Background(), scope, joined); err != nil {
		t.Fatal(err)
	}
	if err := cache.StoreDetail(context.Background(), scope, joinedProject); err != nil {
		t.Fatal(err)
	}
	snapshot, found, err := cache.Load(context.Background(), scope)
	if err != nil || !found {
		t.Fatalf("Load found=%t err=%v", found, err)
	}
	if !snapshot.OwnedDirectoryRefreshed || !snapshot.JoinedDirectoryRefreshed {
		t.Fatalf("directory refresh markers = owned:%t joined:%t", snapshot.OwnedDirectoryRefreshed, snapshot.JoinedDirectoryRefreshed)
	}
	if got := projectIDs(snapshot.Summaries.Items); !reflect.DeepEqual(got, []string{"project-001"}) {
		t.Fatalf("owned summaries = %v", got)
	}
	if got := projectIDs(snapshot.JoinedSummaries.Items); !reflect.DeepEqual(got, []string{"project-joined"}) {
		t.Fatalf("joined summaries = %v", got)
	}

	emptyJoined := ProjectPage{Items: []Project{}, scopeKey: scope.Key}
	if err := cache.StoreJoinedSummaries(context.Background(), scope, emptyJoined); err != nil {
		t.Fatal(err)
	}
	cleared, found, err := cache.Load(context.Background(), scope)
	if err != nil || !found {
		t.Fatalf("Load after empty joined refresh found=%t err=%v", found, err)
	}
	if cleared.JoinedSummaries.Items == nil || len(cleared.JoinedSummaries.Items) != 0 {
		t.Fatalf("joined summaries after clear = %#v", cleared.JoinedSummaries.Items)
	}
	if len(cleared.Details) != 0 {
		t.Fatalf("revoked joined detail remains cached = %#v", cleared.Details)
	}
}

func TestProjectCacheSafelyInvalidatesOlderSchema(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	cache := newProjectCacheForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	if err := cache.StoreSummaries(context.Background(), scope, cacheTestPage(t, scope, 1)); err != nil {
		t.Fatal(err)
	}
	path := cache.path(scope)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), `"schema_version":2`, `"schema_version":1`, 1))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Load(context.Background(), scope); err == nil || found {
		t.Fatalf("older schema Load found=%t err=%v", found, err)
	}
	if matches, _ := filepath.Glob(path + ".invalid-*"); len(matches) != 1 {
		t.Fatalf("older schema isolation files = %v", matches)
	}
}

func projectIDs(projects []Project) []string {
	ids := make([]string, len(projects))
	for index, item := range projects {
		ids[index] = item.ProjectID
	}
	return ids
}

func TestProjectCacheRemoveProjectEvictsSummaryAndDetail(t *testing.T) {
	now := time.Date(2026, 8, 17, 17, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}
	cache := newProjectCacheForTest(t, t.TempDir(), resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	page := cacheTestPage(t, scope, 2)
	if err := cache.StoreSummaries(context.Background(), scope, page); err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if err := cache.StoreDetail(context.Background(), scope, item); err != nil {
			t.Fatal(err)
		}
	}

	if err := cache.RemoveProject(context.Background(), scope, "project-001"); err != nil {
		t.Fatal(err)
	}
	snapshot, found, err := cache.Load(context.Background(), scope)
	if err != nil || !found {
		t.Fatalf("Load after remove found=%t err=%v", found, err)
	}
	if len(snapshot.Summaries.Items) != 1 || snapshot.Summaries.Items[0].ProjectID != "project-002" {
		t.Fatalf("summaries after remove = %#v", snapshot.Summaries.Items)
	}
	if len(snapshot.Details) != 1 || snapshot.Details[0].ProjectID != "project-002" {
		t.Fatalf("details after remove = %#v", snapshot.Details)
	}
}

func TestProjectCacheRemoveProjectKeepsSchemaValidWithoutCachedDetails(t *testing.T) {
	now := time.Date(2026, 8, 17, 17, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	cache := newProjectCacheForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	page := cacheTestPage(t, scope, 2)
	if err := cache.StoreSummaries(context.Background(), scope, page); err != nil {
		t.Fatal(err)
	}
	if err := cache.RemoveProject(context.Background(), scope, "project-001"); err != nil {
		t.Fatal(err)
	}
	snapshot, found, err := cache.Load(context.Background(), scope)
	if err != nil || !found {
		t.Fatalf("Load after summary-only remove found=%t err=%v", found, err)
	}
	if len(snapshot.Summaries.Items) != 1 || snapshot.Summaries.Items[0].ProjectID != "project-002" || snapshot.Details == nil || len(snapshot.Details) != 0 {
		t.Fatalf("summary-only snapshot after remove = %#v", snapshot)
	}
}

func TestProjectCacheBoundsCorruptionAndTTL(t *testing.T) {
	now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}
	cache := newProjectCacheForTest(t, t.TempDir(), resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)

	t.Run("summary and detail bounds", func(t *testing.T) {
		page := cacheTestPage(t, scope, projectCacheMaxSummaries+1)
		if err := cache.StoreSummaries(context.Background(), scope, page); err != nil {
			t.Fatal(err)
		}
		for index := 0; index < projectCacheMaxDetails+1; index++ {
			if err := cache.StoreDetail(context.Background(), scope, page.Items[index]); err != nil {
				t.Fatal(err)
			}
		}
		snapshot, found, err := cache.Load(context.Background(), scope)
		if err != nil || !found || len(snapshot.Summaries.Items) != projectCacheMaxSummaries || len(snapshot.Details) != projectCacheMaxDetails {
			t.Fatalf("bounded Load summaries=%d details=%d found=%t err=%v", len(snapshot.Summaries.Items), len(snapshot.Details), found, err)
		}
		if snapshot.Details[0].ProjectID == "project-001" {
			t.Fatal("detail LRU retained the oldest entry")
		}
	})

	t.Run("TTL evicts", func(t *testing.T) {
		now = now.Add(projectCacheStaleTTL + time.Second)
		if _, found, err := cache.Load(context.Background(), scope); err != nil || found {
			t.Fatalf("expired Load found=%t err=%v", found, err)
		}
		if _, err := os.Stat(cache.path(scope)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired cache still exists: %v", err)
		}
	})

	t.Run("oversize and corrupt files are isolated", func(t *testing.T) {
		now = time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
		path := cache.path(scope)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, projectCacheMaxBytes+1), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, found, err := cache.Load(context.Background(), scope); err == nil || found {
			t.Fatalf("oversize Load found=%t err=%v", found, err)
		}
		if matches, _ := filepath.Glob(path + ".invalid-*"); len(matches) != 1 {
			t.Fatalf("oversize isolation files = %v", matches)
		}
		if err := os.WriteFile(path, []byte(`{"schema":`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, found, err := cache.Load(context.Background(), scope); err == nil || found {
			t.Fatalf("corrupt Load found=%t err=%v", found, err)
		}
		if matches, _ := filepath.Glob(path + ".invalid-*"); len(matches) != 2 {
			t.Fatalf("corrupt isolation files = %v", matches)
		}
		if err := cache.StoreSummaries(context.Background(), scope, cacheTestPage(t, scope, 1)); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(raw), `"details":[]`, `"details":null`, 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, found, err := cache.Load(context.Background(), scope); err == nil || found {
			t.Fatalf("null details Load found=%t err=%v", found, err)
		}
	})

	t.Run("case aliases invalid UTF-8 and future timestamps are isolated", func(t *testing.T) {
		now = time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
		path := cache.path(scope)
		writeValid := func() []byte {
			t.Helper()
			if err := cache.StoreSummaries(context.Background(), scope, cacheTestPage(t, scope, 1)); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
		for name, mutate := range map[string]func([]byte) []byte{
			"case alias": func(raw []byte) []byte {
				return []byte(strings.Replace(string(raw), `"schema":"reasonix.project-cache"`, `"schema":"reasonix.project-cache","Schema":"reasonix.project-cache"`, 1))
			},
			"invalid UTF-8": func(raw []byte) []byte {
				index := strings.Index(string(raw), `Project 001`)
				if index < 0 {
					t.Fatal("cached project name not found")
				}
				raw[index] = 0xff
				return raw
			},
			"future cached_at": func(raw []byte) []byte {
				return []byte(strings.Replace(string(raw), `"cached_at":"2026-08-05T11:00:00Z"`, `"cached_at":"2026-08-06T11:00:00Z"`, 1))
			},
			"unsafe project version": func(raw []byte) []byte {
				return []byte(strings.Replace(string(raw), `"version":1`, `"version":9007199254740992`, 1))
			},
		} {
			t.Run(name, func(t *testing.T) {
				raw := mutate(writeValid())
				if err := os.WriteFile(path, raw, 0o600); err != nil {
					t.Fatal(err)
				}
				if _, found, err := cache.Load(context.Background(), scope); err == nil || found {
					t.Fatalf("invalid Load found=%t err=%v", found, err)
				}
			})
		}
	})
}

func TestProjectCacheFallbackMatrix(t *testing.T) {
	tests := []struct {
		name         string
		remoteErr    error
		wantFallback bool
		wantIsolated bool
	}{
		{name: "network unreachable", remoteErr: &cloud.Error{Code: cloud.ErrorNetworkUnavailable, Message: "offline"}, wantFallback: true},
		{name: "retryable server", remoteErr: &cloud.Error{Code: cloud.ErrorCode("SERVER_BUSY"), Message: "busy", Retryable: true}, wantFallback: true},
		{name: "authentication", remoteErr: &cloud.Error{Code: cloud.ErrorAuthRequired, Message: "expired", Retryable: true}, wantIsolated: true},
		{name: "not found", remoteErr: &cloud.Error{Code: cloud.ErrorCode("NOT_FOUND"), Message: "missing"}, wantIsolated: true},
		{name: "permission", remoteErr: &cloud.Error{Code: cloud.ErrorCode("PERMISSION_DENIED"), Message: "denied"}, wantIsolated: true},
		{name: "protocol", remoteErr: &cloud.Error{Code: cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"), Message: "drift"}, wantIsolated: true},
		{name: "owner mismatch", remoteErr: errors.New("project owner does not match current scope"), wantIsolated: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
			scope := gatewayTestScope(t, "user-1", 7)
			resolver := &gatewayTestScopeResolver{scope: scope}
			cache := newProjectCacheForTest(t, t.TempDir(), resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
			if err := cache.StoreSummaries(context.Background(), scope, cacheTestPage(t, scope, 1)); err != nil {
				t.Fatal(err)
			}
			snapshot, fallback, err := cache.ResolveFallback(context.Background(), scope, testCase.remoteErr)
			if testCase.wantFallback {
				if err != nil || !fallback || !snapshot.Stale || snapshot.CachedAt != now {
					t.Fatalf("ResolveFallback = %#v fallback=%t err=%v", snapshot, fallback, err)
				}
				return
			}
			if !errors.Is(err, testCase.remoteErr) || fallback {
				t.Fatalf("ResolveFallback fallback=%t err=%v", fallback, err)
			}
			if testCase.wantIsolated {
				if _, statErr := os.Stat(cache.path(scope)); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("unsafe cache was not isolated: %v", statErr)
				}
				if matches, _ := filepath.Glob(cache.path(scope) + ".isolated-*"); len(matches) != 1 {
					t.Fatalf("isolation files = %v", matches)
				}
			}
		})
	}

	t.Run("failed isolation remains durably denied until online refresh", func(t *testing.T) {
		now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
		scope := gatewayTestScope(t, "user-1", 7)
		resolver := &gatewayTestScopeResolver{scope: scope}
		cache := newProjectCacheForTest(t, t.TempDir(), resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
		page := cacheTestPage(t, scope, 1)
		if err := cache.StoreSummaries(context.Background(), scope, page); err != nil {
			t.Fatal(err)
		}
		cache.renameFile = func(string, string) error { return errors.New("simulated locked cache") }
		authErr := &cloud.Error{Code: cloud.ErrorAuthRequired, Message: "expired"}
		if _, fallback, err := cache.ResolveFallback(context.Background(), scope, authErr); !errors.Is(err, authErr) || fallback {
			t.Fatalf("auth fallback=%t err=%v", fallback, err)
		}
		if _, err := os.Stat(cache.denyPath(scope)); err != nil {
			t.Fatalf("durable deny marker: %v", err)
		}
		retryable := &cloud.Error{Code: cloud.ErrorCode("SERVER_BUSY"), Message: "busy", Retryable: true}
		if _, fallback, err := cache.ResolveFallback(context.Background(), scope, retryable); !errors.Is(err, retryable) || fallback {
			t.Fatalf("denied retryable fallback=%t err=%v", fallback, err)
		}
		if err := cache.StoreSummaries(context.Background(), scope, page); err != nil {
			t.Fatalf("online refresh: %v", err)
		}
		if _, err := os.Stat(cache.denyPath(scope)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("deny marker after refresh: %v", err)
		}
		if _, fallback, err := cache.ResolveFallback(context.Background(), scope, retryable); err != nil || !fallback {
			t.Fatalf("refreshed retryable fallback=%t err=%v", fallback, err)
		}
	})

	t.Run("sibling deny and isolation failure use restart-safe root deny", func(t *testing.T) {
		now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
		scope := gatewayTestScope(t, "user-1", 7)
		resolver := &gatewayTestScopeResolver{scope: scope}
		root := t.TempDir()
		cache := newProjectCacheForTest(t, root, resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
		page := cacheTestPage(t, scope, 1)
		if err := cache.StoreSummaries(context.Background(), scope, page); err != nil {
			t.Fatal(err)
		}
		cache.renameFile = func(string, string) error { return errors.New("simulated locked cache") }
		writeStrict := cache.atomicWriteFile
		cache.atomicWriteFile = func(path string, payload []byte, mode os.FileMode) error {
			if path == cache.denyPath(scope) {
				return errors.New("simulated unwritable cache directory")
			}
			return writeStrict(path, payload, mode)
		}
		authErr := &cloud.Error{Code: cloud.ErrorAuthRequired, Message: "expired"}
		if _, fallback, err := cache.ResolveFallback(context.Background(), scope, authErr); !errors.Is(err, authErr) || fallback {
			t.Fatalf("auth fallback=%t err=%v", fallback, err)
		}

		restarted := newProjectCacheForTest(t, root, resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
		retryable := &cloud.Error{Code: cloud.ErrorCode("SERVER_BUSY"), Message: "busy", Retryable: true}
		if _, fallback, err := restarted.ResolveFallback(context.Background(), scope, retryable); !errors.Is(err, retryable) || fallback {
			t.Fatalf("restart-safe denied fallback=%t err=%v", fallback, err)
		}
		if err := restarted.StoreSummaries(context.Background(), scope, page); err != nil {
			t.Fatalf("online refresh: %v", err)
		}
		if _, fallback, err := restarted.ResolveFallback(context.Background(), scope, retryable); err != nil || !fallback {
			t.Fatalf("refreshed fallback=%t err=%v", fallback, err)
		}
	})
}

func TestProjectCacheRejectsNULCursor(t *testing.T) {
	now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	cache := newProjectCacheForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	page := cacheTestPage(t, scope, 1)
	cursor := "opaque\x00cursor"
	page.NextCursor = &cursor
	if err := cache.StoreSummaries(context.Background(), scope, page); err == nil {
		t.Fatal("StoreSummaries accepted a NUL cursor")
	}
}

func TestProjectCacheScopeRaceAndConcurrentDetails(t *testing.T) {
	now := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
	scopeA := gatewayTestScope(t, "user-1", 7)
	scopeB := gatewayTestScope(t, "user-2", 8)
	resolver := &gatewayTestScopeResolver{scope: scopeA}
	cache := newProjectCacheForTest(t, t.TempDir(), resolver, func() time.Time { return now }, ContractVersion, projectCacheTestHash)
	entered := make(chan struct{})
	release := make(chan struct{})
	cache.beforeCommit = func() {
		close(entered)
		<-release
	}
	result := make(chan error, 1)
	go func() { result <- cache.StoreSummaries(context.Background(), scopeA, cacheTestPage(t, scopeA, 1)) }()
	<-entered
	resolver.Set(scopeB)
	close(release)
	if err := <-result; err == nil || !strings.Contains(err.Error(), string(cloud.ErrorProfileScopeMismatch)) {
		t.Fatalf("late StoreSummaries error = %v", err)
	}
	if _, err := os.Stat(cache.path(scopeA)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("late response wrote scope A cache: %v", err)
	}
	cache.beforeCommit = nil
	resolver.Set(scopeA)

	page := cacheTestPage(t, scopeA, 64)
	var wait sync.WaitGroup
	for _, project := range page.Items {
		project := project
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := cache.StoreDetail(context.Background(), scopeA, project); err != nil {
				t.Errorf("StoreDetail(%s): %v", project.ProjectID, err)
			}
		}()
	}
	wait.Wait()
	snapshot, found, err := cache.Load(context.Background(), scopeA)
	if err != nil || !found || len(snapshot.Details) != projectCacheMaxDetails {
		t.Fatalf("concurrent detail Load details=%d found=%t err=%v", len(snapshot.Details), found, err)
	}
}

func newProjectCacheForTest(t *testing.T, root string, resolver ProjectScopeResolver, now func() time.Time, version, hash string) *ProjectCache {
	t.Helper()
	cache, err := NewProjectCache(ProjectCacheOptions{Root: root, Scopes: resolver, Now: now, ContractVersion: version, ContractHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

func cacheTestPage(t *testing.T, scope Scope, count int) ProjectPage {
	t.Helper()
	items := make([]Project, count)
	for index := range items {
		items[index] = Project{
			ProjectID: fmt.Sprintf("project-%03d", index+1), OwnerUserID: scope.Binding.CloudUserID,
			Name: fmt.Sprintf("Project %03d", index+1), Status: ProjectActive, Version: 1,
			CreatedAt: time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC),
		}
	}
	return ProjectPage{Items: items, scopeKey: scope.Key}
}
