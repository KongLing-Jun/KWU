package project

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"kwu/internal/cloud"
	"kwu/internal/fileutil"
)

const (
	projectCacheSchema        = "reasonix.project-cache"
	projectCacheSchemaVersion = 2
	projectCacheMaxSummaries  = 1000
	projectCacheMaxDetails    = 50
	projectCacheMaxBytes      = 5 * 1024 * 1024
	projectCacheStaleTTL      = 24 * time.Hour
)

// ProjectCacheOptions configures one profile-root cache without storing local
// paths or credentials inside the cache document.
type ProjectCacheOptions struct {
	Root            string
	Scopes          ProjectScopeResolver
	Now             func() time.Time
	ContractVersion string
	ContractHash    string
}

// ProjectCacheSnapshot is a bounded, non-authoritative Project cache view.
// Stale is set only when ResolveFallback accepts the cache for display.
type ProjectCacheSnapshot struct {
	Summaries                ProjectPage
	JoinedSummaries          ProjectPage
	OwnedDirectoryRefreshed  bool
	JoinedDirectoryRefreshed bool
	Details                  []Project
	CachedAt                 time.Time
	Stale                    bool
}

// ProjectCache persists scope- and contract-isolated Project read data.
type ProjectCache struct {
	root            string
	scopes          ProjectScopeResolver
	now             func() time.Time
	contractVersion string
	contractHash    string
	contractKey     string
	beforeCommit    func()
	renameFile      func(string, string) error
	atomicWriteFile func(string, []byte, os.FileMode) error
}

type projectCacheDocument struct {
	Schema          string                    `json:"schema"`
	SchemaVersion   int                       `json:"schema_version"`
	Environment     cloud.EnvironmentIdentity `json:"environment"`
	Binding         cloud.BindingScope        `json:"binding"`
	ScopeKey        string                    `json:"scope_key"`
	ContractVersion string                    `json:"contract_version"`
	ContractHash    string                    `json:"contract_hash"`
	CachedAt        time.Time                 `json:"cached_at"`
	Summaries       projectCachePageWire      `json:"summaries"`
	JoinedSummaries projectCachePageWire      `json:"joined_summaries"`
	OwnedRefreshed  bool                      `json:"owned_directory_refreshed"`
	JoinedRefreshed bool                      `json:"joined_directory_refreshed"`
	Details         []projectCacheDetailWire  `json:"details"`
}

type projectCachePageWire struct {
	Items      []projectCacheProjectWire `json:"items"`
	NextCursor *string                   `json:"next_cursor"`
}

type projectCacheDetailWire struct {
	Project    projectCacheProjectWire `json:"project"`
	AccessedAt time.Time               `json:"accessed_at"`
}

type projectCacheProjectWire struct {
	ProjectID   string        `json:"project_id"`
	OwnerUserID string        `json:"owner_user_id"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	Status      ProjectStatus `json:"status"`
	Version     uint64        `json:"version"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type projectCacheLockEntry struct {
	mutex sync.Mutex
	refs  int
}

var projectCacheLocks = struct {
	sync.Mutex
	entries map[string]*projectCacheLockEntry
}{entries: make(map[string]*projectCacheLockEntry)}

// NewProjectCache creates a cache rooted below the supplied profile state
// directory and isolated by contract version/hash plus the full Scope key.
func NewProjectCache(options ProjectCacheOptions) (*ProjectCache, error) {
	if err := validateProjectCacheOptions(options); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, fmt.Errorf("projects-v1: normalize project cache root: %w", err)
	}
	contractSum := sha256.Sum256([]byte(options.ContractVersion + "\n" + options.ContractHash))
	return &ProjectCache{
		root: filepath.Clean(root), scopes: options.Scopes, now: options.Now,
		contractVersion: options.ContractVersion, contractHash: options.ContractHash,
		contractKey: hex.EncodeToString(contractSum[:]), renameFile: os.Rename,
		atomicWriteFile: fileutil.AtomicWriteFileStrict,
	}, nil
}

func validateProjectCacheOptions(options ProjectCacheOptions) error {
	if strings.TrimSpace(options.Root) == "" {
		return fmt.Errorf("projects-v1: project cache root is not configured")
	}
	if nilGatewayDependency(options.Scopes) {
		return fmt.Errorf("projects-v1: project cache scope resolver is not configured")
	}
	if options.Now == nil {
		return fmt.Errorf("projects-v1: project cache clock is not configured")
	}
	if options.ContractVersion == "" || strings.TrimSpace(options.ContractVersion) != options.ContractVersion || len([]byte(options.ContractVersion)) > 128 {
		return fmt.Errorf("projects-v1: invalid cache contract version")
	}
	if len(options.ContractHash) != sha256.Size*2 || options.ContractHash != strings.ToLower(options.ContractHash) {
		return fmt.Errorf("projects-v1: invalid cache contract hash")
	}
	if _, err := hex.DecodeString(options.ContractHash); err != nil {
		return fmt.Errorf("projects-v1: invalid cache contract hash")
	}
	return nil
}

// Load reads a valid, unexpired snapshot for the exact current scope.
func (c *ProjectCache) Load(ctx context.Context, scope Scope) (ProjectCacheSnapshot, bool, error) {
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return ProjectCacheSnapshot{}, false, err
	}
	path := c.path(scope)
	release := acquireProjectCacheLock(path)
	defer release()
	document, found, err := c.loadDocumentLocked(path, scope, true, false)
	if err != nil || !found {
		return ProjectCacheSnapshot{}, false, err
	}
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return ProjectCacheSnapshot{}, false, err
	}
	return cacheSnapshot(document, scope), true, nil
}

// ResolveFallback returns same-scope stale data only for network-unavailable
// or explicitly retryable server errors. Every other remote error is returned
// unchanged and its cache file is isolated from later fallback.
func (c *ProjectCache) ResolveFallback(ctx context.Context, scope Scope, remoteErr error) (ProjectCacheSnapshot, bool, error) {
	if remoteErr == nil {
		return ProjectCacheSnapshot{}, false, fmt.Errorf("projects-v1: cache fallback requires a remote error")
	}
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return ProjectCacheSnapshot{}, false, err
	}
	if !projectCacheFallbackAllowed(remoteErr) {
		if err := c.denyFallback(ctx, scope, remoteErr); err != nil {
			return ProjectCacheSnapshot{}, false, err
		}
		return ProjectCacheSnapshot{}, false, remoteErr
	}
	snapshot, found, err := c.Load(ctx, scope)
	if err != nil || !found {
		return ProjectCacheSnapshot{}, false, remoteErr
	}
	snapshot.Stale = true
	return snapshot, true, nil
}

func (c *ProjectCache) denyFallback(ctx context.Context, scope Scope, remoteErr error) error {
	if errors.Is(remoteErr, context.Canceled) || errors.Is(remoteErr, context.DeadlineExceeded) {
		return nil
	}
	path := c.path(scope)
	release := acquireProjectCacheLock(path)
	defer release()
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	denyErr := c.markDenied(path, scope)
	isolateErr := c.isolate(path, "isolated")
	if denyErr != nil && isolateErr != nil {
		return errors.Join(remoteErr, denyErr, isolateErr)
	}
	return nil
}

func (c *ProjectCache) commitLocked(ctx context.Context, path string, scope Scope, document projectCacheDocument) error {
	if c.beforeCommit != nil {
		c.beforeCommit()
	}
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("projects-v1: encode project cache: %w", err)
	}
	payload = append(payload, '\n')
	if len(payload) > projectCacheMaxBytes {
		return fmt.Errorf("projects-v1: project cache exceeds %d bytes", projectCacheMaxBytes)
	}
	if err := c.atomicWriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("projects-v1: atomically replace project cache: %w", err)
	}
	if err := syncProjectCacheDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	if err := c.clearDenyMarkers(path, scope); err != nil {
		return err
	}
	if err := syncProjectCacheDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	return c.syncRootDenyDirectory(scope)
}

func (c *ProjectCache) syncRootDenyDirectory(scope Scope) error {
	rootDenyDirectory := filepath.Dir(c.rootDenyPath(scope))
	if _, err := os.Stat(rootDenyDirectory); err == nil {
		return syncProjectCacheDirectory(rootDenyDirectory)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("projects-v1: inspect project cache root deny directory: %w", err)
	}
	return nil
}

func (c *ProjectCache) clearDenyMarkers(path string, scope Scope) error {
	if err := removeProjectCacheMarker(c.denyPathForCachePath(path), "project cache deny"); err != nil {
		return err
	}
	return removeProjectCacheMarker(c.rootDenyPath(scope), "project cache root deny")
}

func removeProjectCacheMarker(path, label string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("projects-v1: clear %s marker: %w", label, err)
	}
	return nil
}

func (c *ProjectCache) loadDocumentLocked(path string, scope Scope, enforceTTL, allowDeniedRefresh bool) (projectCacheDocument, bool, error) {
	denied, err := c.cacheDenied(path, scope)
	if err != nil {
		return projectCacheDocument{}, false, err
	}
	if denied {
		if allowDeniedRefresh {
			return projectCacheDocument{}, false, nil
		}
		return projectCacheDocument{}, false, fmt.Errorf("projects-v1: project cache is denied until online refresh")
	}
	payload, found, err := c.readCachePayload(path)
	if err != nil || !found {
		return projectCacheDocument{}, false, err
	}
	document, err := c.decodeCachePayload(path, scope, payload)
	if err != nil {
		return projectCacheDocument{}, false, err
	}
	if enforceTTL && c.now().UTC().Sub(document.CachedAt) > projectCacheStaleTTL {
		return c.evictExpiredCache(path)
	}
	return document, true, nil
}

func (c *ProjectCache) cacheDenied(path string, scope Scope) (bool, error) {
	for _, denyPath := range []string{c.denyPathForCachePath(path), c.rootDenyPath(scope)} {
		if _, err := os.Stat(denyPath); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("projects-v1: inspect project cache deny marker: %w", err)
		}
	}
	return false, nil
}

func (c *ProjectCache) readCachePayload(path string) ([]byte, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("projects-v1: stat project cache: %w", err)
	}
	if info.Size() > projectCacheMaxBytes {
		return nil, false, errors.Join(fmt.Errorf("projects-v1: project cache exceeds %d bytes", projectCacheMaxBytes), c.isolate(path, "invalid"))
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("projects-v1: read project cache: %w", err)
	}
	if !utf8.Valid(payload) {
		return nil, false, errors.Join(fmt.Errorf("projects-v1: project cache is not valid UTF-8"), c.isolate(path, "invalid"))
	}
	return payload, true, nil
}

func (c *ProjectCache) decodeCachePayload(path string, scope Scope, payload []byte) (projectCacheDocument, error) {
	fail := func(err error) (projectCacheDocument, error) {
		return projectCacheDocument{}, errors.Join(err, c.isolate(path, "invalid"))
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return fail(fmt.Errorf("projects-v1: invalid project cache: %w", err))
	}
	if err := validateProjectCacheShape(payload); err != nil {
		return fail(err)
	}
	var document projectCacheDocument
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return fail(fmt.Errorf("projects-v1: decode project cache: %w", err))
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return fail(err)
	}
	if err := c.validateDocument(document, scope); err != nil {
		return fail(err)
	}
	return document, nil
}

func (c *ProjectCache) evictExpiredCache(path string) (projectCacheDocument, bool, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return projectCacheDocument{}, false, fmt.Errorf("projects-v1: evict expired project cache: %w", err)
	}
	return projectCacheDocument{}, false, nil
}

func (c *ProjectCache) validateDocument(document projectCacheDocument, scope Scope) error {
	if document.Schema != projectCacheSchema || document.SchemaVersion != projectCacheSchemaVersion {
		return fmt.Errorf("projects-v1: unsupported project cache schema")
	}
	if document.Environment != scope.Environment || document.Binding != scope.Binding || document.ScopeKey != scope.Key {
		return fmt.Errorf("projects-v1: project cache identity mismatch")
	}
	if document.ContractVersion != c.contractVersion || document.ContractHash != c.contractHash {
		return fmt.Errorf("projects-v1: project cache contract mismatch")
	}
	if err := c.validateCacheDocumentBounds(document); err != nil {
		return err
	}
	return validateCacheDocumentEntries(document, scope)
}

func validateCacheDocumentEntries(document projectCacheDocument, scope Scope) error {
	if err := validateCacheSummaryWires(document.Summaries, scope); err != nil {
		return err
	}
	if err := validateCacheSummaryWires(document.JoinedSummaries, scope); err != nil {
		return err
	}
	return validateCacheDetailWires(document.Details, scope)
}

func (c *ProjectCache) validateCacheDocumentBounds(document projectCacheDocument) error {
	if document.CachedAt.IsZero() || document.CachedAt.Location() != time.UTC || document.CachedAt.After(c.now().UTC()) {
		return fmt.Errorf("projects-v1: invalid project cache timestamp")
	}
	if len(document.Summaries.Items) > projectCacheMaxSummaries || len(document.JoinedSummaries.Items) > projectCacheMaxSummaries || len(document.Details) > projectCacheMaxDetails {
		return fmt.Errorf("projects-v1: project cache entry count exceeds bounds")
	}
	return nil
}

func validateCacheSummaryWires(summaries projectCachePageWire, scope Scope) error {
	seen := make(map[string]struct{}, len(summaries.Items))
	for _, wire := range summaries.Items {
		project, err := wire.domain(scope)
		if err != nil {
			return err
		}
		if _, duplicate := seen[project.ProjectID]; duplicate {
			return fmt.Errorf("projects-v1: duplicate cached project_id")
		}
		seen[project.ProjectID] = struct{}{}
	}
	return validateCacheCursor(summaries.NextCursor)
}

func validateCacheDetailWires(details []projectCacheDetailWire, scope Scope) error {
	seen := make(map[string]struct{}, len(details))
	for _, detail := range details {
		project, err := detail.Project.domain(scope)
		if err != nil {
			return err
		}
		if detail.AccessedAt.IsZero() || detail.AccessedAt.Location() != time.UTC {
			return fmt.Errorf("projects-v1: invalid cached detail access timestamp")
		}
		if _, duplicate := seen[project.ProjectID]; duplicate {
			return fmt.Errorf("projects-v1: duplicate cached detail project_id")
		}
		seen[project.ProjectID] = struct{}{}
	}
	return nil
}

func (c *ProjectCache) newDocument(scope Scope) projectCacheDocument {
	return projectCacheDocument{
		Schema: projectCacheSchema, SchemaVersion: projectCacheSchemaVersion,
		Environment: scope.Environment, Binding: scope.Binding, ScopeKey: scope.Key,
		ContractVersion: c.contractVersion, ContractHash: c.contractHash,
		Summaries:       projectCachePageWire{Items: []projectCacheProjectWire{}},
		JoinedSummaries: projectCachePageWire{Items: []projectCacheProjectWire{}}, Details: []projectCacheDetailWire{},
	}
}

func (c *ProjectCache) validateCurrentScope(ctx context.Context, expected Scope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := expected.Validate(); err != nil {
		return err
	}
	current, err := c.scopes.CurrentProjectScope(ctx)
	if err != nil {
		return err
	}
	if err := current.Validate(); err != nil {
		return err
	}
	if current.Key != expected.Key {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "project cache operation belongs to a stale binding scope"}
	}
	return nil
}

func (c *ProjectCache) path(scope Scope) string {
	return filepath.Join(c.root, "project-cache", "v1", c.contractKey, scope.Key+".json")
}

func (c *ProjectCache) isolate(path, reason string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("projects-v1: inspect cache before isolation: %w", err)
	}
	var lastErr error
	for sequence := 0; sequence < 1000; sequence++ {
		target := fmt.Sprintf("%s.%s-%s-%03d", path, reason, c.now().UTC().Format("20060102T150405.000000000Z"), sequence)
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := c.renameFile(path, target); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("projects-v1: isolate project cache: %w", lastErr)
}

func (c *ProjectCache) markDenied(path string, scope Scope) error {
	marker := []byte("reasonix.project-cache.denied.v1\n")
	primaryPath := c.denyPathForCachePath(path)
	if err := c.atomicWriteFile(primaryPath, marker, 0o600); err == nil {
		return syncProjectCacheDirectory(filepath.Dir(primaryPath))
	} else {
		primaryErr := fmt.Errorf("projects-v1: persist project cache deny marker: %w", err)
		rootPath := c.rootDenyPath(scope)
		if rootErr := c.atomicWriteFile(rootPath, marker, 0o600); rootErr != nil {
			return errors.Join(primaryErr, fmt.Errorf("projects-v1: persist project cache root deny marker: %w", rootErr))
		}
		if syncErr := syncProjectCacheDirectory(filepath.Dir(rootPath)); syncErr != nil {
			return errors.Join(primaryErr, syncErr)
		}
		return nil
	}
}

func (c *ProjectCache) denyPath(scope Scope) string {
	return c.denyPathForCachePath(c.path(scope))
}

func (c *ProjectCache) denyPathForCachePath(path string) string { return path + ".denied" }

func (c *ProjectCache) rootDenyPath(scope Scope) string {
	return filepath.Join(c.root, "project-cache-denials", "v1", c.contractKey, scope.Key+".denied")
}

func acquireProjectCacheLock(path string) func() {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	projectCacheLocks.Lock()
	entry := projectCacheLocks.entries[key]
	if entry == nil {
		entry = &projectCacheLockEntry{}
		projectCacheLocks.entries[key] = entry
	}
	entry.refs++
	projectCacheLocks.Unlock()
	entry.mutex.Lock()
	return func() {
		entry.mutex.Unlock()
		projectCacheLocks.Lock()
		entry.refs--
		if entry.refs == 0 && projectCacheLocks.entries[key] == entry {
			delete(projectCacheLocks.entries, key)
		}
		projectCacheLocks.Unlock()
	}
}

func syncProjectCacheDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("projects-v1: open project cache directory for sync: %w", err)
	}
	syncErr := handle.Sync()
	closeErr := handle.Close()
	if syncErr != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("projects-v1: sync project cache directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("projects-v1: close project cache directory: %w", closeErr)
	}
	return nil
}

func projectCacheFallbackAllowed(err error) bool {
	var cloudErr *cloud.Error
	if !errors.As(err, &cloudErr) {
		return false
	}
	switch cloudErr.Code {
	case cloud.ErrorAuthRequired, cloud.ErrorProfileScopeMismatch,
		cloud.ErrorCode("NOT_FOUND"), cloud.ErrorCode("PERMISSION_DENIED"), cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"):
		return false
	case cloud.ErrorNetworkUnavailable:
		return true
	default:
		return cloudErr.Retryable
	}
}

func validateProjectCacheShape(payload []byte) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		return fmt.Errorf("projects-v1: decode project cache shape: %w", err)
	}
	documentFields := [...]string{
		"schema", "schema_version", "environment", "binding", "scope_key",
		"contract_version", "contract_hash", "cached_at", "summaries", "joined_summaries",
		"owned_directory_refreshed", "joined_directory_refreshed", "details",
	}
	if err := requireExactProjectFields(document, documentFields[:]...); err != nil {
		return fmt.Errorf("projects-v1: project cache: %w", err)
	}
	if err := requireNonNullProjectFields(document, documentFields[:], "project cache"); err != nil {
		return err
	}
	if err := validateProjectCacheSummaryShape(document["summaries"]); err != nil {
		return err
	}
	if err := validateProjectCacheSummaryShape(document["joined_summaries"]); err != nil {
		return err
	}
	projectFields := []string{"project_id", "owner_user_id", "name", "description", "status", "version", "created_at", "updated_at"}
	if err := validateProjectCacheNestedShape(document); err != nil {
		return err
	}
	return validateProjectCacheDetailsShape(document["details"], projectFields)
}

func validateProjectCacheSummaryShape(raw json.RawMessage) error {
	var summaries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &summaries); err != nil {
		return fmt.Errorf("projects-v1: cached summaries must be an object")
	}
	if err := requireExactProjectFields(summaries, "items", "next_cursor"); err != nil {
		return fmt.Errorf("projects-v1: cached summaries: %w", err)
	}
	if err := validateProjectCacheSummaryFields(summaries); err != nil {
		return err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(summaries["items"], &items); err != nil {
		return fmt.Errorf("projects-v1: cached summary items must be an array")
	}
	return validateProjectCacheSummaryItems(items)
}

func validateProjectCacheSummaryFields(summaries map[string]json.RawMessage) error {
	for _, field := range [...]string{"items", "next_cursor"} {
		value, ok := summaries[field]
		if !ok || (field == "items" && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return fmt.Errorf("projects-v1: cached summaries field %s must be present", field)
		}
	}
	return nil
}

func validateProjectCacheSummaryItems(items []json.RawMessage) error {
	for index, raw := range items {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			return fmt.Errorf("projects-v1: cached summary %d must be an object", index)
		}
		if err := requireExactProjectFields(item, "project_id", "owner_user_id", "name", "description", "status", "version", "created_at", "updated_at"); err != nil {
			return fmt.Errorf("projects-v1: cached summary %d: %w", index, err)
		}
	}
	return nil
}

func validateProjectCacheNestedShape(document map[string]json.RawMessage) error {
	if err := validateNestedProjectFields(document, "environment", []string{"environment_id", "cloud_base_url"}, "cached"); err != nil {
		return err
	}
	return validateNestedProjectFields(document, "binding", []string{"profile_id", "cloud_user_id", "binding_generation"}, "cached")
}

func validateProjectCacheDetailsShape(raw json.RawMessage, projectFields []string) error {
	var details []json.RawMessage
	if err := json.Unmarshal(raw, &details); err != nil {
		return fmt.Errorf("projects-v1: cached details must be an array")
	}
	for index, raw := range details {
		var detail map[string]json.RawMessage
		if err := json.Unmarshal(raw, &detail); err != nil {
			return fmt.Errorf("projects-v1: cached detail %d must be an object", index)
		}
		if err := requireExactProjectFields(detail, "project", "accessed_at"); err != nil {
			return fmt.Errorf("projects-v1: cached detail %d: %w", index, err)
		}
		var project map[string]json.RawMessage
		if err := json.Unmarshal(detail["project"], &project); err != nil {
			return fmt.Errorf("projects-v1: cached detail %d project must be an object", index)
		}
		if err := requireExactProjectFields(project, projectFields...); err != nil {
			return fmt.Errorf("projects-v1: cached detail %d project: %w", index, err)
		}
	}
	return nil
}

func validateCachePage(page ProjectPage, scope Scope) error {
	if len(page.Items) == 0 && page.Items == nil {
		return fmt.Errorf("projects-v1: cached summaries must use a non-nil item list")
	}
	seen := make(map[string]struct{}, len(page.Items))
	for _, project := range page.Items {
		if err := validateCacheProject(project, scope); err != nil {
			return err
		}
		if _, duplicate := seen[project.ProjectID]; duplicate {
			return fmt.Errorf("projects-v1: duplicate cached project_id")
		}
		seen[project.ProjectID] = struct{}{}
	}
	return validateCacheCursor(page.NextCursor)
}

func validateCacheCursor(nextCursor *string) error {
	if nextCursor == nil {
		return nil
	}
	cursor := *nextCursor
	if cursor == "" || strings.TrimSpace(cursor) != cursor || strings.ContainsRune(cursor, '\x00') || len([]byte(cursor)) > 2048 {
		return fmt.Errorf("projects-v1: invalid cached next cursor")
	}
	return nil
}

func validateCacheProject(project Project, scope Scope) error {
	_, err := cacheProjectWire(project).domain(scope)
	return err
}

func cacheProjectWire(project Project) projectCacheProjectWire {
	return projectCacheProjectWire{
		ProjectID: project.ProjectID, OwnerUserID: project.OwnerUserID, Name: project.Name,
		Description: cloneString(project.Description), Status: project.Status, Version: project.Version,
		CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt,
	}
}

func cacheProjectWires(projects []Project) []projectCacheProjectWire {
	wires := make([]projectCacheProjectWire, len(projects))
	for index, project := range projects {
		wires[index] = cacheProjectWire(project)
	}
	return wires
}

func (wire projectCacheProjectWire) domain(scope Scope) (Project, error) {
	return projectWire{
		ProjectID: wire.ProjectID, OwnerUserID: wire.OwnerUserID, Name: wire.Name,
		Description: cloneString(wire.Description), Status: string(wire.Status), Version: wire.Version,
		CreatedAt: wire.CreatedAt, UpdatedAt: wire.UpdatedAt,
	}.domain(scope)
}

func cacheSnapshot(document projectCacheDocument, scope Scope) ProjectCacheSnapshot {
	summaries := make([]Project, len(document.Summaries.Items))
	for index, wire := range document.Summaries.Items {
		summaries[index], _ = wire.domain(scope)
	}
	joinedSummaries := make([]Project, len(document.JoinedSummaries.Items))
	for index, wire := range document.JoinedSummaries.Items {
		joinedSummaries[index], _ = wire.domain(scope)
	}
	details := make([]Project, len(document.Details))
	for index, detail := range document.Details {
		details[index], _ = detail.Project.domain(scope)
	}
	return ProjectCacheSnapshot{
		Summaries:                ProjectPage{Items: summaries, NextCursor: cloneString(document.Summaries.NextCursor), scopeKey: scope.Key},
		JoinedSummaries:          ProjectPage{Items: joinedSummaries, NextCursor: cloneString(document.JoinedSummaries.NextCursor), scopeKey: scope.Key},
		OwnedDirectoryRefreshed:  document.OwnedRefreshed,
		JoinedDirectoryRefreshed: document.JoinedRefreshed,
		Details:                  details, CachedAt: document.CachedAt,
	}
}
