package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"kwu/internal/cloud"
	"kwu/internal/fileutil"
)

const (
	localWorkspaceBindingSchema        = "reasonix.local-workspace-bindings"
	localWorkspaceBindingSchemaVersion = 1
)

// LocalWorkspaceBindingStatus reports whether a saved local directory remains
// accessible without deleting its durable association.
type LocalWorkspaceBindingStatus string

const (
	// LocalWorkspaceAvailable means the canonical directory is currently accessible.
	LocalWorkspaceAvailable LocalWorkspaceBindingStatus = "available"
	// LocalWorkspaceUnavailable preserves a binding whose directory cannot currently be accessed.
	LocalWorkspaceUnavailable LocalWorkspaceBindingStatus = "unavailable"
)

// LocalWorkspaceBinding binds one CloudProject identity to one local directory
// inside an exact environment/account generation.
type LocalWorkspaceBinding struct {
	ProjectScopeKey string                      `json:"project_scope_key"`
	ProjectID       string                      `json:"project_id"`
	DisplayPath     string                      `json:"display_path"`
	CanonicalPath   string                      `json:"canonical_path"`
	Status          LocalWorkspaceBindingStatus `json:"status"`
}

// LocalWorkspaceBindingRepositoryOptions configures independent durable
// binding storage below one profile state root.
type LocalWorkspaceBindingRepositoryOptions struct {
	Root   string
	Scopes ProjectScopeResolver
}

// LocalWorkspaceBindingRepository owns exact-scope local workspace bindings.
type LocalWorkspaceBindingRepository struct {
	root         string
	scopes       ProjectScopeResolver
	beforeCommit func()
}

type localWorkspaceBindingDocument struct {
	Schema        string                      `json:"schema"`
	SchemaVersion int                         `json:"schema_version"`
	Environment   cloud.EnvironmentIdentity   `json:"environment"`
	Binding       cloud.BindingScope          `json:"binding"`
	ScopeKey      string                      `json:"scope_key"`
	Bindings      []localWorkspaceBindingWire `json:"bindings"`
}

type localWorkspaceBindingWire struct {
	ProjectID     string                      `json:"project_id"`
	DisplayPath   string                      `json:"display_path"`
	CanonicalPath string                      `json:"canonical_path"`
	PathIdentity  string                      `json:"path_identity"`
	Status        LocalWorkspaceBindingStatus `json:"status"`
}

type localWorkspaceBindingLockEntry struct {
	mutex sync.Mutex
	refs  int
}

var localWorkspaceBindingLocks = struct {
	sync.Mutex
	entries map[string]*localWorkspaceBindingLockEntry
}{entries: make(map[string]*localWorkspaceBindingLockEntry)}

// NewLocalWorkspaceBindingRepository creates an independent binding store.
func NewLocalWorkspaceBindingRepository(options LocalWorkspaceBindingRepositoryOptions) (*LocalWorkspaceBindingRepository, error) {
	if strings.TrimSpace(options.Root) == "" {
		return nil, fmt.Errorf("projects-v1: local workspace binding root is not configured")
	}
	if nilGatewayDependency(options.Scopes) {
		return nil, fmt.Errorf("projects-v1: local workspace binding scope resolver is not configured")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, fmt.Errorf("projects-v1: normalize local workspace binding root: %w", err)
	}
	return &LocalWorkspaceBindingRepository{root: filepath.Clean(root), scopes: options.Scopes}, nil
}

// Bind creates or replaces one Project's binding after proving the directory's
// canonical identity. Multiple projects may use the same local workspace; each
// task remains isolated by its session-level ProjectContext pin.
func (r *LocalWorkspaceBindingRepository) Bind(ctx context.Context, scope Scope, projectID, displayPath string) (LocalWorkspaceBinding, error) {
	if err := r.validateCurrentScope(ctx, scope); err != nil {
		return LocalWorkspaceBinding{}, err
	}
	if err := validateOpaqueProjectID(projectID); err != nil {
		return LocalWorkspaceBinding{}, err
	}
	display, canonical, identity, err := resolveLocalWorkspacePath(displayPath)
	if err != nil {
		return LocalWorkspaceBinding{}, err
	}
	path := r.path(scope)
	release := acquireLocalWorkspaceBindingLock(path)
	defer release()
	document, found, err := r.loadLocked(path, scope)
	if err != nil {
		return LocalWorkspaceBinding{}, err
	}
	if !found {
		document = newLocalWorkspaceBindingDocument(scope)
	}
	replacement := localWorkspaceBindingWire{
		ProjectID: projectID, DisplayPath: display, CanonicalPath: canonical,
		PathIdentity: identity, Status: LocalWorkspaceAvailable,
	}
	document.Bindings = upsertLocalWorkspaceBinding(document.Bindings, replacement)
	if err := r.commitLocked(ctx, path, scope, document); err != nil {
		return LocalWorkspaceBinding{}, err
	}
	return replacement.public(scope), nil
}

// Get returns one retained binding and refreshes its current availability.
func (r *LocalWorkspaceBindingRepository) Get(ctx context.Context, scope Scope, projectID string) (LocalWorkspaceBinding, bool, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return LocalWorkspaceBinding{}, false, err
	}
	bindings, err := r.List(ctx, scope)
	if err != nil {
		return LocalWorkspaceBinding{}, false, err
	}
	for _, binding := range bindings {
		if binding.ProjectID == projectID {
			return binding, true, nil
		}
	}
	return LocalWorkspaceBinding{}, false, nil
}

// List returns all exact-scope bindings and durably marks inaccessible paths
// unavailable without deleting them.
func (r *LocalWorkspaceBindingRepository) List(ctx context.Context, scope Scope) ([]LocalWorkspaceBinding, error) {
	if err := r.validateCurrentScope(ctx, scope); err != nil {
		return nil, err
	}
	path := r.path(scope)
	release := acquireLocalWorkspaceBindingLock(path)
	defer release()
	document, found, err := r.loadLocked(path, scope)
	if err != nil || !found {
		return []LocalWorkspaceBinding{}, err
	}
	changed := refreshLocalWorkspaceBindingStatuses(document.Bindings)
	if changed {
		if err := r.commitLocked(ctx, path, scope, document); err != nil {
			return nil, err
		}
	}
	if err := r.validateCurrentScope(ctx, scope); err != nil {
		return nil, err
	}
	return publicLocalWorkspaceBindings(document.Bindings, scope), nil
}

func upsertLocalWorkspaceBinding(bindings []localWorkspaceBindingWire, replacement localWorkspaceBindingWire) []localWorkspaceBindingWire {
	for index := range bindings {
		if bindings[index].ProjectID == replacement.ProjectID {
			bindings[index] = replacement
			return bindings
		}
	}
	return append(bindings, replacement)
}

func refreshLocalWorkspaceBindingStatuses(bindings []localWorkspaceBindingWire) bool {
	changed := false
	for index := range bindings {
		status := LocalWorkspaceUnavailable
		if localWorkspaceDirectoryAccessible(bindings[index].CanonicalPath) {
			status = LocalWorkspaceAvailable
		}
		if bindings[index].Status != status {
			bindings[index].Status = status
			changed = true
		}
	}
	return changed
}

func publicLocalWorkspaceBindings(wires []localWorkspaceBindingWire, scope Scope) []LocalWorkspaceBinding {
	bindings := make([]LocalWorkspaceBinding, len(wires))
	for index, binding := range wires {
		bindings[index] = binding.public(scope)
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].ProjectID < bindings[j].ProjectID })
	return bindings
}

// Unbind removes only the local association; it does not touch either the
// directory or the CloudProject.
func (r *LocalWorkspaceBindingRepository) Unbind(ctx context.Context, scope Scope, projectID string) error {
	if err := r.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	if err := validateOpaqueProjectID(projectID); err != nil {
		return err
	}
	path := r.path(scope)
	release := acquireLocalWorkspaceBindingLock(path)
	defer release()
	document, found, err := r.loadLocked(path, scope)
	if err != nil || !found {
		return err
	}
	bindings := document.Bindings[:0]
	for _, binding := range document.Bindings {
		if binding.ProjectID != projectID {
			bindings = append(bindings, binding)
		}
	}
	if len(bindings) == len(document.Bindings) {
		return nil
	}
	document.Bindings = append([]localWorkspaceBindingWire(nil), bindings...)
	if document.Bindings == nil {
		document.Bindings = []localWorkspaceBindingWire{}
	}
	return r.commitLocked(ctx, path, scope, document)
}

func (r *LocalWorkspaceBindingRepository) loadLocked(path string, scope Scope) (localWorkspaceBindingDocument, bool, error) {
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return localWorkspaceBindingDocument{}, false, nil
	}
	if err != nil {
		return localWorkspaceBindingDocument{}, false, fmt.Errorf("projects-v1: read local workspace bindings: %w", err)
	}
	if !utf8.Valid(payload) {
		r.isolate(path)
		return localWorkspaceBindingDocument{}, false, fmt.Errorf("projects-v1: local workspace bindings are not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		r.isolate(path)
		return localWorkspaceBindingDocument{}, false, fmt.Errorf("projects-v1: invalid local workspace bindings: %w", err)
	}
	if err := validateLocalWorkspaceBindingShape(payload); err != nil {
		r.isolate(path)
		return localWorkspaceBindingDocument{}, false, err
	}
	var document localWorkspaceBindingDocument
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		r.isolate(path)
		return localWorkspaceBindingDocument{}, false, fmt.Errorf("projects-v1: decode local workspace bindings: %w", err)
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		r.isolate(path)
		return localWorkspaceBindingDocument{}, false, err
	}
	if err := validateLocalWorkspaceBindingDocument(document, scope); err != nil {
		r.isolate(path)
		return localWorkspaceBindingDocument{}, false, err
	}
	return document, true, nil
}

func (r *LocalWorkspaceBindingRepository) commitLocked(ctx context.Context, path string, scope Scope, document localWorkspaceBindingDocument) error {
	if r.beforeCommit != nil {
		r.beforeCommit()
	}
	if err := r.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("projects-v1: encode local workspace bindings: %w", err)
	}
	payload = append(payload, '\n')
	if err := fileutil.AtomicWriteFileStrict(path, payload, 0o600); err != nil {
		return fmt.Errorf("projects-v1: atomically replace local workspace bindings: %w", err)
	}
	return syncLocalWorkspaceBindingDirectory(filepath.Dir(path))
}

func (r *LocalWorkspaceBindingRepository) validateCurrentScope(ctx context.Context, expected Scope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := expected.Validate(); err != nil {
		return err
	}
	current, err := r.scopes.CurrentProjectScope(ctx)
	if err != nil {
		return err
	}
	if err := current.Validate(); err != nil {
		return err
	}
	if current.Key != expected.Key {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "local workspace binding operation belongs to a stale scope"}
	}
	return nil
}

func (r *LocalWorkspaceBindingRepository) path(scope Scope) string {
	return filepath.Join(r.root, "local-workspace-bindings", "v1", scope.Key+".json")
}

func (r *LocalWorkspaceBindingRepository) isolate(path string) {
	for sequence := 0; sequence < 1000; sequence++ {
		target := fmt.Sprintf("%s.invalid-%s-%03d", path, time.Now().UTC().Format("20060102T150405.000000000Z"), sequence)
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if os.Rename(path, target) == nil {
			return
		}
	}
}

func newLocalWorkspaceBindingDocument(scope Scope) localWorkspaceBindingDocument {
	return localWorkspaceBindingDocument{
		Schema: localWorkspaceBindingSchema, SchemaVersion: localWorkspaceBindingSchemaVersion,
		Environment: scope.Environment, Binding: scope.Binding, ScopeKey: scope.Key,
		Bindings: []localWorkspaceBindingWire{},
	}
}

func validateLocalWorkspaceBindingDocument(document localWorkspaceBindingDocument, scope Scope) error {
	if document.Schema != localWorkspaceBindingSchema || document.SchemaVersion != localWorkspaceBindingSchemaVersion {
		return fmt.Errorf("projects-v1: unsupported local workspace binding schema")
	}
	if document.Environment != scope.Environment || document.Binding != scope.Binding || document.ScopeKey != scope.Key {
		return fmt.Errorf("projects-v1: local workspace binding identity mismatch")
	}
	projectIDs := make(map[string]struct{}, len(document.Bindings))
	for _, binding := range document.Bindings {
		if err := validateLocalWorkspaceBindingWire(binding); err != nil {
			return err
		}
		if err := addUniqueLocalWorkspaceProject(projectIDs, binding); err != nil {
			return err
		}
	}
	return nil
}

func validateLocalWorkspaceBindingWire(binding localWorkspaceBindingWire) error {
	if err := validateOpaqueProjectID(binding.ProjectID); err != nil {
		return err
	}
	if binding.DisplayPath == "" || strings.TrimSpace(binding.DisplayPath) != binding.DisplayPath || strings.ContainsRune(binding.DisplayPath, '\x00') {
		return fmt.Errorf("projects-v1: invalid local workspace display path")
	}
	if !validLocalWorkspaceCanonicalPath(binding.CanonicalPath, binding.PathIdentity) {
		return fmt.Errorf("projects-v1: invalid canonical local workspace path")
	}
	if binding.Status != LocalWorkspaceAvailable && binding.Status != LocalWorkspaceUnavailable {
		return fmt.Errorf("projects-v1: invalid local workspace binding status")
	}
	return nil
}

func validLocalWorkspaceCanonicalPath(canonicalPath, pathIdentity string) bool {
	return !strings.ContainsRune(canonicalPath, '\x00') && !strings.ContainsRune(pathIdentity, '\x00') &&
		filepath.IsAbs(canonicalPath) && canonicalPath == filepath.Clean(canonicalPath) &&
		pathIdentity == workspacePathIdentity(canonicalPath)
}

func addUniqueLocalWorkspaceProject(projectIDs map[string]struct{}, binding localWorkspaceBindingWire) error {
	if _, duplicate := projectIDs[binding.ProjectID]; duplicate {
		return fmt.Errorf("projects-v1: duplicate local workspace project_id")
	}
	projectIDs[binding.ProjectID] = struct{}{}
	return nil
}

func validateLocalWorkspaceBindingShape(payload []byte) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		return fmt.Errorf("projects-v1: decode local workspace binding shape: %w", err)
	}
	documentFields := [...]string{"schema", "schema_version", "environment", "binding", "scope_key", "bindings"}
	if err := requireExactProjectFields(document, documentFields[:]...); err != nil {
		return fmt.Errorf("projects-v1: local workspace bindings: %w", err)
	}
	if err := requireNonNullProjectFields(document, documentFields[:], "local workspace binding"); err != nil {
		return err
	}
	if err := validateNestedProjectFields(document, "environment", []string{"environment_id", "cloud_base_url"}, "local workspace binding"); err != nil {
		return err
	}
	if err := validateNestedProjectFields(document, "binding", []string{"profile_id", "cloud_user_id", "binding_generation"}, "local workspace binding"); err != nil {
		return err
	}
	var bindings []json.RawMessage
	if err := json.Unmarshal(document["bindings"], &bindings); err != nil {
		return fmt.Errorf("projects-v1: local workspace bindings must be an array")
	}
	return validateLocalWorkspaceBindingShapes(bindings)
}

func validateLocalWorkspaceBindingShapes(bindings []json.RawMessage) error {
	for index, raw := range bindings {
		var binding map[string]json.RawMessage
		if err := json.Unmarshal(raw, &binding); err != nil {
			return fmt.Errorf("projects-v1: local workspace binding %d must be an object", index)
		}
		if err := requireExactProjectFields(binding, "project_id", "display_path", "canonical_path", "path_identity", "status"); err != nil {
			return fmt.Errorf("projects-v1: local workspace binding %d: %w", index, err)
		}
	}
	return nil
}

func resolveLocalWorkspacePath(value string) (display, canonical, identity string, err error) {
	display = strings.TrimSpace(value)
	if display == "" {
		return "", "", "", fmt.Errorf("projects-v1: local workspace path is empty")
	}
	absolute, err := filepath.Abs(display)
	if err != nil {
		return "", "", "", fmt.Errorf("projects-v1: normalize local workspace path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if err := requireAccessibleLocalWorkspaceDirectory(absolute); err != nil {
		return "", "", "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", "", "", fmt.Errorf("projects-v1: cannot prove local workspace path identity: %w", err)
	}
	canonical, err = filepath.Abs(resolved)
	if err != nil {
		return "", "", "", fmt.Errorf("projects-v1: canonicalize local workspace path: %w", err)
	}
	canonical = filepath.Clean(canonical)
	if err := requireAccessibleLocalWorkspaceDirectory(canonical); err != nil {
		return "", "", "", err
	}
	return display, canonical, workspacePathIdentity(canonical), nil
}

func requireAccessibleLocalWorkspaceDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("projects-v1: local workspace directory is unavailable: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("projects-v1: local workspace path is not a directory")
	}
	handle, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("projects-v1: local workspace directory is inaccessible: %w", err)
	}
	_, readErr := handle.Readdirnames(1)
	closeErr := handle.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return fmt.Errorf("projects-v1: local workspace directory is inaccessible: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("projects-v1: close local workspace directory: %w", closeErr)
	}
	return nil
}

func localWorkspaceDirectoryAccessible(path string) bool {
	return requireAccessibleLocalWorkspaceDirectory(path) == nil
}

func workspacePathIdentity(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS != "windows" {
		return clean
	}
	volume := filepath.VolumeName(clean)
	remainder := strings.TrimPrefix(clean, volume)
	return strings.ToLower(volume) + "|" + strings.ToLower(remainder)
}

func (binding localWorkspaceBindingWire) public(scope Scope) LocalWorkspaceBinding {
	return LocalWorkspaceBinding{
		ProjectScopeKey: scope.Key, ProjectID: binding.ProjectID, DisplayPath: binding.DisplayPath,
		CanonicalPath: binding.CanonicalPath, Status: binding.Status,
	}
}

func acquireLocalWorkspaceBindingLock(path string) func() {
	key := workspacePathIdentity(path)
	localWorkspaceBindingLocks.Lock()
	entry := localWorkspaceBindingLocks.entries[key]
	if entry == nil {
		entry = &localWorkspaceBindingLockEntry{}
		localWorkspaceBindingLocks.entries[key] = entry
	}
	entry.refs++
	localWorkspaceBindingLocks.Unlock()
	entry.mutex.Lock()
	return func() {
		entry.mutex.Unlock()
		localWorkspaceBindingLocks.Lock()
		entry.refs--
		if entry.refs == 0 && localWorkspaceBindingLocks.entries[key] == entry {
			delete(localWorkspaceBindingLocks.entries, key)
		}
		localWorkspaceBindingLocks.Unlock()
	}
}

func syncLocalWorkspaceBindingDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("projects-v1: open local workspace binding directory for sync: %w", err)
	}
	syncErr := handle.Sync()
	closeErr := handle.Close()
	if syncErr != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("projects-v1: sync local workspace binding directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("projects-v1: close local workspace binding directory: %w", closeErr)
	}
	return nil
}
