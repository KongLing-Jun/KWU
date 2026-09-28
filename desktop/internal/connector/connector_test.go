package connector

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeClientVersionSharedByCatalogAndInstaller(t *testing.T) {
	tests := map[string]string{"dev": "dev", "v1.2.3": "1.2.3", "1.2.3": "1.2.3", "v1.2.3+build.4": "1.2.3"}
	for input, want := range tests {
		got, err := NormalizeClientVersion(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeClientVersion(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	policy := RuntimePolicy{OS: "windows", Architecture: "x64", ClientVersion: "v1.2.3"}
	catalogURL, err := CatalogURL("https://cloud.example.com", policy)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(catalogURL)
	if parsed.Query().Get("client_version") != "1.2.3" {
		t.Fatalf("catalog client_version = %q", parsed.Query().Get("client_version"))
	}
	item, manifest := skillsManifest("unknown-lab-connector", "1.0.0", "1.2.3")
	if err := validateConnectorManifest(manifest, item, policy); err != nil {
		t.Fatalf("installer compatibility did not reuse canonical policy: %v", err)
	}
	policy.ClientVersion = "v1.2.2"
	if err := validateConnectorManifest(manifest, item, policy); err == nil {
		t.Fatal("older canonicalized client version was accepted")
	}
}

func TestStateStoreTreatsPriorBindingOwnerAsMissing(t *testing.T) {
	root := t.TempDir()
	store := StateStore{}
	previous := OwnerScope{ProfileID: "profile", CloudUserID: "user", BindingGeneration: 17, StateRoot: root}
	active := ActiveVersion{Version: "1.0.0", SHA256: "sha256"}
	if err := store.WriteOwner(previous, "tencent-docs", active, StatusConnected); err != nil {
		t.Fatal(err)
	}

	current := previous
	current.BindingGeneration = 19
	state, found, err := store.LoadOwner(current, "tencent-docs")
	if err != nil {
		t.Fatalf("LoadOwner returned stale binding error: %v", err)
	}
	if found || state != (ownerState{}) {
		t.Fatalf("LoadOwner stale binding = %+v, %v; want missing", state, found)
	}

	state, found, err = store.LoadOwner(previous, "tencent-docs")
	if err != nil || !found || state.Status != StatusConnected {
		t.Fatalf("LoadOwner matching binding = %+v, %v, %v", state, found, err)
	}
}

func TestHTTPSConnectorManifestIgnoresRuntimePlatformAndArchitecture(t *testing.T) {
	item, manifest := skillsManifest("architecture-neutral-connector", "1.0.0", "0.9.0")
	for _, policy := range []RuntimePolicy{
		{OS: "windows", Architecture: "arm64", ClientVersion: "1.2.0"},
		{OS: "macos", Architecture: "arm64", ClientVersion: "1.2.0"},
	} {
		if err := validateConnectorManifest(manifest, item, policy); err != nil {
			t.Fatalf("validateConnectorManifest(%+v) = %v", policy, err)
		}
	}
}

func TestUnknownConnectorUsesRawCatalogParserDownloaderAndInstaller(t *testing.T) {
	const connectorID = "unknown-lab-connector"
	packageBytes := skillsPackage(t, connectorID, "1.0.0", "0.1.0", "content-v1")
	hash := sha256Hex(packageBytes)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/packages/%s/1.0.0/%s/package.zip", connectorID, hash)
	rawCatalog := rawCatalog(t, listener.Addr().String(), connectorID, "1.0.0", "0.1.0", packageBytes, hash)
	server, err := StartDevFixedServer(listener, DevFixedFixture{Catalog: rawCatalog, Packages: map[string][]byte{path: packageBytes}})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	root := t.TempDir()
	scope := OwnerScope{ProfileID: "profile", CloudUserID: "user", BindingGeneration: 1, StateRoot: filepath.Join(root, "state")}
	service, err := NewService(ServiceOptions{
		Mode: CatalogModeDevFixed, Source: server.Source(), Downloader: Downloader{},
		Installer:  Installer{Root: filepath.Join(root, "packages"), Policy: testPolicy("dev")},
		StateStore: StateStore{SharedRoot: filepath.Join(root, "packages")}, TempRoot: filepath.Join(root, "tmp"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Install(context.Background(), connectorID, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != StatusAuthorizationNeeded || snapshot.Version != "1.0.0" || snapshot.SHA256 != hash {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	_, packageRoot, manifest, err := service.Manifest(connectorID)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildExecutionPlan(manifest, packageRoot, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if plan.ConnectorID != connectorID || plan.Skills == nil || len(plan.MCP) != 1 || plan.MCP[0].Transport != "streamable-http" {
		t.Fatalf("execution plan = %+v", plan)
	}
}

func TestServiceUpgradeChangesActiveContentAndPreservesConnectedOwner(t *testing.T) {
	const connectorID = "unknown-startup-upgrade"
	packages := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := packages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(raw)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	source := &changingCatalogSource{}
	root := t.TempDir()
	scope := OwnerScope{ProfileID: "profile", CloudUserID: "user", BindingGeneration: 1, StateRoot: filepath.Join(root, "state")}
	service, err := NewService(ServiceOptions{
		Mode: CatalogModeDevFixed, Source: source, Downloader: Downloader{},
		Installer:  Installer{Root: filepath.Join(root, "packages"), Policy: testPolicy("dev")},
		StateStore: StateStore{SharedRoot: filepath.Join(root, "packages")}, TempRoot: filepath.Join(root, "tmp"),
	})
	if err != nil {
		t.Fatal(err)
	}
	publish := func(version, content string) {
		t.Helper()
		raw := skillsPackage(t, connectorID, version, "0.1.0", content)
		hash := sha256Hex(raw)
		path := fmt.Sprintf("/packages/%s/%s/%s/package.zip", connectorID, version, hash)
		packages[path] = raw
		source.raw = rawCatalog(t, serverURL.Host, connectorID, version, "0.1.0", raw, hash)
	}
	publish("1.0.0", "startup-content-v1")
	if _, err := service.RefreshCatalog(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, err := service.Install(context.Background(), connectorID, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetOwnerStatus(scope, connectorID, StatusConnected, nil); err != nil {
		t.Fatal(err)
	}
	publish("1.1.0", "startup-content-v2")
	if _, err := service.RefreshCatalog(context.Background()); err != nil {
		t.Fatal(err)
	}
	upgraded, err := service.Install(context.Background(), connectorID, scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.Status != StatusConnected || upgraded.Version != "1.1.0" || upgraded.SHA256 == first.SHA256 {
		t.Fatalf("upgraded snapshot = %+v, first = %+v", upgraded, first)
	}
	content, err := os.ReadFile(filepath.Join(upgraded.PackagePath, "skills", "sample", "SKILL.md"))
	if err != nil || string(content) != "startup-content-v2" {
		t.Fatalf("upgraded active content = %q, %v", content, err)
	}
}

func TestUpgradeChangesActivePackageContentAndFailureKeepsOldActive(t *testing.T) {
	const connectorID = "unknown-upgrade-connector"
	root := t.TempDir()
	installer := Installer{Root: filepath.Join(root, "packages"), Policy: testPolicy("dev")}
	install := func(version, content string) InstallResult {
		t.Helper()
		raw := skillsPackage(t, connectorID, version, "0.1.0", content)
		path := filepath.Join(root, version+".zip")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		hash := sha256Hex(raw)
		item := catalogItem(connectorID, version, "0.1.0", int64(len(raw)), hash, "http://127.0.0.1:1/package.zip")
		result, err := installer.Install(context.Background(), item, DownloadedPackage{Path: path, Size: int64(len(raw)), SHA256: hash})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := install("1.0.0", "content-v1")
	second := install("1.1.0", "content-v2")
	if first.Active.SHA256 == second.Active.SHA256 || first.Path == second.Path {
		t.Fatalf("upgrade did not change active content: first=%+v second=%+v", first, second)
	}
	content, err := os.ReadFile(filepath.Join(second.Path, "skills", "sample", "SKILL.md"))
	if err != nil || string(content) != "content-v2" {
		t.Fatalf("active package content = %q, %v", content, err)
	}
	active, activePath, found, err := installer.Active(connectorID)
	if err != nil || !found || active.Version != "1.1.0" || activePath != second.Path {
		t.Fatalf("active after upgrade = %+v %q %v %v", active, activePath, found, err)
	}
	brokenPath := filepath.Join(root, "broken.zip")
	if err := os.WriteFile(brokenPath, []byte("not-a-zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	brokenHash := sha256Hex([]byte("not-a-zip"))
	brokenItem := catalogItem(connectorID, "1.2.0", "0.1.0", int64(len("not-a-zip")), brokenHash, "http://127.0.0.1:1/package.zip")
	if _, err := installer.Install(context.Background(), brokenItem, DownloadedPackage{Path: brokenPath, Size: int64(len("not-a-zip")), SHA256: brokenHash}); err == nil {
		t.Fatal("invalid upgrade package was accepted")
	}
	active, activePath, found, err = installer.Active(connectorID)
	if err != nil || !found || active.Version != "1.1.0" || activePath != second.Path {
		t.Fatalf("failed upgrade changed active = %+v %q %v %v", active, activePath, found, err)
	}
}

func TestActivePackageContentDamageIsRejectedBeforeRegistration(t *testing.T) {
	const connectorID = "unknown-damage-connector"
	root := t.TempDir()
	raw := skillsPackage(t, connectorID, "1.0.0", "0.1.0", "trusted-content")
	packagePath := filepath.Join(root, "package.zip")
	if err := os.WriteFile(packagePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := sha256Hex(raw)
	installer := Installer{Root: filepath.Join(root, "packages"), Policy: testPolicy("dev")}
	item := catalogItem(connectorID, "1.0.0", "0.1.0", int64(len(raw)), hash, "http://127.0.0.1:1/package.zip")
	installed, err := installer.Install(context.Background(), item, DownloadedPackage{Path: packagePath, Size: int64(len(raw)), SHA256: hash})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed.Path, "skills", "sample", "SKILL.md"), []byte("damaged-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, found, err := installer.ActiveManifest(connectorID); err == nil || found {
		t.Fatalf("damaged active package was accepted: found=%v err=%v", found, err)
	}
}

func TestExecutionPlanCoversHTTPSMCPAndSkills(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "sample"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		Connector: ConnectorManifest{ID: "unknown-composite", Kind: "composite", Entrypoints: ManifestEntrypoints{MCP: "mcp.json", Skills: "skills"}},
		MCP: &MCPManifest{Servers: []MCPServer{
			{ID: "remote", Transport: "streamable-http", URL: "https://mcp.example.com", StaticHeaders: map[string]string{"X-Client": "desktop"}, Auth: MCPAuth{Type: "none"}},
		}},
	}
	plan, err := BuildExecutionPlan(manifest, root, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.MCP) != 1 || plan.MCP[0].RuntimeName != "unknown-composite--remote" || plan.MCP[0].Headers["X-Client"] != "desktop" ||
		plan.MCP[0].AuthType != "none" || plan.MCP[0].AuthProvider != "" || plan.Skills == nil {
		t.Fatalf("execution plan = %+v", plan)
	}
	manifest.MCP.Servers[0].Transport = "stdio"
	if _, err := BuildExecutionPlan(manifest, root, "windows"); err == nil {
		t.Fatal("defensive execution plan accepted managed stdio")
	}
}

func TestSchema1PackageIsHistoricalOnly(t *testing.T) {
	item, manifest := skillsManifest("unknown-schema-one", "1.0.0", "0.1.0")
	manifest.SchemaVersion = 1
	if err := validateConnectorManifest(manifest, item, testPolicy("dev")); err == nil {
		t.Fatal("schema 1 package entered the schema 2 runtime")
	}
}

func TestCatalogRefreshFailureKeepsOnlyInMemoryLastGoodCatalog(t *testing.T) {
	raw := rawCatalog(t, "127.0.0.1:12345", "unknown-cache-connector", "1.0.0", "0.1.0", []byte("zip"), fmt.Sprintf("%064d", 2))
	source := &changingCatalogSource{raw: raw}
	root := t.TempDir()
	service, err := NewService(ServiceOptions{
		Mode: CatalogModeDevFixed, Source: source, Installer: Installer{Root: filepath.Join(root, "packages"), Policy: testPolicy("dev")},
		StateStore: StateStore{SharedRoot: filepath.Join(root, "packages")}, TempRoot: filepath.Join(root, "tmp"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RefreshCatalog(context.Background()); err != nil {
		t.Fatal(err)
	}
	source.err = errors.New("catalog offline")
	if _, err := service.RefreshCatalog(context.Background()); err == nil {
		t.Fatal("offline refresh unexpectedly succeeded")
	}
	catalog, err := service.Catalog()
	if err != nil || len(catalog.Items) != 1 || catalog.Items[0].ID != "unknown-cache-connector" {
		t.Fatalf("last good in-memory catalog = %+v, %v", catalog, err)
	}
	if _, err := os.Stat(filepath.Join(root, "catalog.json")); !os.IsNotExist(err) {
		t.Fatalf("P2 unexpectedly persisted a Catalog cache: %v", err)
	}
}

type changingCatalogSource struct {
	raw []byte
	err error
}

type catalogSourceFunc func(context.Context) ([]byte, error)

func (f catalogSourceFunc) Fetch(ctx context.Context) ([]byte, error) { return f(ctx) }

func TestCatalogRecoveryClearsStartupFailure(t *testing.T) {
	raw := rawCatalog(t, "127.0.0.1:12345", "recovery-test", "1.0.0", "0.1.0", []byte("zip"), fmt.Sprintf("%064d", 2))
	for _, parseFailure := range []bool{false, true} {
		calls := 0
		service := &Service{mode: CatalogModeDevFixed, source: catalogSourceFunc(func(context.Context) ([]byte, error) {
			calls++
			if calls == 1 {
				if parseFailure {
					return []byte("invalid json"), nil
				}
				return nil, errors.New("fixture offline")
			}
			return raw, nil
		})}
		if err := service.Start(context.Background()); err == nil {
			t.Fatal("startup failure was lost")
		}
		if _, err := service.Catalog(); err == nil {
			t.Fatal("failed startup published a catalog")
		}
		if _, err := service.RecoverCatalog(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Catalog(); err != nil {
			t.Fatal(err)
		}
		if _, err := service.RecoverCatalog(context.Background()); err != nil || calls != 2 {
			t.Fatalf("ready recovery fetched again: calls=%d, err=%v", calls, err)
		}
	}
}

func TestCatalogRecoveryLimitsConcurrentRefreshAndReleasesOnCancel(t *testing.T) {
	entered := make(chan struct{})
	var calls atomic.Int32
	service := &Service{source: catalogSourceFunc(func(ctx context.Context) ([]byte, error) {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- service.Start(ctx) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("fetch did not start")
	}
	for _, refresh := range []func(context.Context) (Catalog, error){service.RefreshCatalog, service.RecoverCatalog} {
		if _, err := refresh(context.Background()); !errors.Is(err, ErrCatalogRefreshInProgress) {
			t.Fatalf("concurrent refresh = %v", err)
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
	if calls.Load() != 1 || service.refreshing.Load() {
		t.Fatal("refresh gate was not released")
	}
	service.source = catalogSourceFunc(func(ctx context.Context) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("missing refresh deadline")
		}
		return nil, errors.New("recoverable failure")
	})
	if _, err := service.RecoverCatalog(context.Background()); err == nil || errors.Is(err, ErrCatalogRefreshInProgress) {
		t.Fatalf("retry after cancellation = %v", err)
	}
}

func (s *changingCatalogSource) Fetch(context.Context) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]byte(nil), s.raw...), nil
}

func skillsManifest(id, version, minimum string) (CatalogItem, ConnectorManifest) {
	item := catalogItem(id, version, minimum, 1, fmt.Sprintf("%064d", 0), "http://127.0.0.1:1/package.zip")
	manifest := ConnectorManifest{
		SchemaVersion: ConnectorSchemaVersion, ID: id, Version: version, Name: "Unknown Lab", Provider: "Test Provider",
		Description: "Non-product schema 2 fixture.", Category: "test", Examples: []string{"Use the fixture"}, Kind: "composite",
		Capabilities: []string{"remote_mcp", "skills"}, MinClientVersion: minimum,
		Platforms: []ManifestPlatform{{OS: "windows", Architectures: []string{"x64"}}}, Entrypoints: ManifestEntrypoints{MCP: "mcp.json", Skills: "skills"},
	}
	return item, manifest
}

func skillsPackage(t *testing.T, id, version, minimum, content string) []byte {
	t.Helper()
	_, manifest := skillsManifest(id, version, minimum)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	files := map[string][]byte{
		"connector.json":         MarshalManifest(manifest),
		"mcp.json":               []byte(`{"schema_version":2,"servers":[{"id":"remote","transport":"streamable-http","url":"https://mcp.example.com/mcp","auth":{"type":"none"}}]}`),
		"skills/sample/SKILL.md": []byte(content),
	}
	for name, raw := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func rawCatalog(t *testing.T, address, id, version, minimum string, packageBytes []byte, hash string) []byte {
	t.Helper()
	path := fmt.Sprintf("http://%s/packages/%s/%s/%s/package.zip", address, id, version, hash)
	item := catalogItem(id, version, minimum, int64(len(packageBytes)), hash, path)
	blocked := any(nil)
	wire := map[string]any{
		"data": map[string]any{"catalog_version": "sha256:" + fmt.Sprintf("%064d", 1), "generated_at": time.Now().UTC().Format(time.RFC3339), "items": []any{map[string]any{
			"id": item.ID, "name": item.Name, "provider": item.Provider, "description": item.Description, "category": item.Category,
			"icon_url": nil, "examples": []string{"Use the fixture"}, "kind": item.Kind, "version": item.Version,
			"min_client_version": item.MinClientVersion, "compatible": true, "blocked_reason": blocked,
			"package": map[string]any{"url": path, "size_bytes": len(packageBytes), "sha256": hash}, "capabilities": item.Capabilities,
		}}},
		"error": nil, "request_id": "request-1", "server_time": time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func catalogItem(id, version, minimum string, size int64, hash, packageURL string) CatalogItem {
	return CatalogItem{ID: id, Name: "Unknown Lab", Provider: "Test Provider", Description: "Non-product schema 2 fixture.", Category: "test", Examples: []string{"Use the fixture"}, Kind: "composite", Version: version, MinClientVersion: minimum, Compatible: true, Package: CatalogPackage{URL: packageURL, SizeBytes: size, SHA256: hash}, Capabilities: []string{"remote_mcp", "skills"}}
}

func testPolicy(clientVersion string) RuntimePolicy {
	return RuntimePolicy{OS: "windows", Architecture: "x64", ClientVersion: clientVersion}
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
