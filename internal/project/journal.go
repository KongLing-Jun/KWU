package project

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"kwu/internal/cloud"
	"kwu/internal/fileutil"
)

const (
	projectCommandJournalSchema        = "reasonix.project-command-journal"
	projectCommandJournalSchemaVersion = 1
	projectCommandJournalMaxUnresolved = 1000
	projectCommandJournalMaxBytes      = 10 * 1024 * 1024
	projectCommandJournalMaxRequest    = 64 * 1024
	projectCommandJournalRetention     = 30 * 24 * time.Hour
)

type ProjectCommandOperation string

const (
	ProjectCommandCreate         ProjectCommandOperation = "project.create"
	ProjectCommandPatch          ProjectCommandOperation = "project.patch"
	ProjectCommandInstructionPut ProjectCommandOperation = "instruction.put"
	ProjectCommandWorkItemCreate ProjectCommandOperation = "work_item.create"
	ProjectCommandWorkItemPatch  ProjectCommandOperation = "work_item.patch"
	ProjectCommandWorkItemMove   ProjectCommandOperation = "work_item.move"
	ProjectCommandWorkItemDelete ProjectCommandOperation = "work_item.delete"
)

type ProjectCommandState string

const (
	ProjectCommandPending ProjectCommandState = "pending"
	ProjectCommandUnknown ProjectCommandState = "unknown"
)

type PrepareProjectCommandInput struct {
	Operation  ProjectCommandOperation
	ProjectID  string
	ResourceID string
	Payload    any
}

type ProjectCommandRecord struct {
	ProjectScopeKey  string
	IdempotencyKey   string
	Operation        ProjectCommandOperation
	ProjectID        string
	ResourceID       string
	CanonicalPayload json.RawMessage
	LocalPayloadHash string
	State            ProjectCommandState
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ProjectCommandJournalOptions struct {
	Root   string
	Scopes ProjectScopeResolver
	Now    func() time.Time
	Random io.Reader
}

type ProjectCommandJournal struct {
	root         string
	scopes       ProjectScopeResolver
	now          func() time.Time
	random       io.Reader
	randomMutex  sync.Mutex
	beforeCommit func()
}

// ProjectCommandReadback is the minimal identity/version proof obtained from
// an authoritative GET after a mutation command completes.
type ProjectCommandReadback struct {
	ProjectScopeKey string
	ResourceKind    string
	ResourceID      string
	ProjectID       string
	Version         uint64
}

// ProjectCommandDeletionReadback is an authoritative GET proof that the
// resource named by a successful permanent-delete command no longer exists.
type ProjectCommandDeletionReadback struct {
	ProjectScopeKey string
	ResourceKind    string
	ResourceID      string
	ProjectID       string
	Absent          bool
}

type projectCommandJournalDocument struct {
	Schema          string                    `json:"schema"`
	SchemaVersion   int                       `json:"schema_version"`
	Environment     cloud.EnvironmentIdentity `json:"environment"`
	Binding         cloud.BindingScope        `json:"binding"`
	ScopeKey        string                    `json:"scope_key"`
	ContractVersion string                    `json:"contract_version"`
	ContractHash    string                    `json:"contract_hash"`
	Commands        []projectCommandWire      `json:"commands"`
}

type projectCommandWire struct {
	IdempotencyKey   string                  `json:"idempotency_key"`
	Operation        ProjectCommandOperation `json:"operation"`
	ProjectID        string                  `json:"project_id"`
	ResourceID       string                  `json:"resource_id"`
	CanonicalPayload json.RawMessage         `json:"canonical_payload"`
	LocalPayloadHash string                  `json:"local_payload_hash"`
	State            ProjectCommandState     `json:"state"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

type projectCommandJournalLockEntry struct {
	mutex sync.Mutex
	refs  int
}

var projectCommandJournalLocks = struct {
	sync.Mutex
	entries map[string]*projectCommandJournalLockEntry
}{entries: make(map[string]*projectCommandJournalLockEntry)}

func NewProjectCommandJournal(options ProjectCommandJournalOptions) (*ProjectCommandJournal, error) {
	if strings.TrimSpace(options.Root) == "" {
		return nil, fmt.Errorf("projects-v1: command journal root is not configured")
	}
	if nilGatewayDependency(options.Scopes) {
		return nil, fmt.Errorf("projects-v1: command journal scope resolver is not configured")
	}
	if options.Now == nil {
		return nil, fmt.Errorf("projects-v1: command journal clock is not configured")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, fmt.Errorf("projects-v1: normalize command journal root: %w", err)
	}
	random := options.Random
	if random == nil {
		random = rand.Reader
	}
	return &ProjectCommandJournal{root: filepath.Clean(root), scopes: options.Scopes, now: options.Now, random: random}, nil
}

func CanonicalProjectRequestBytes(value any) ([]byte, error) {
	if err := validateProjectRequestValue(reflect.ValueOf(value)); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("projects-v1: encode canonical request: %w", err)
	}
	canonical, err := canonicalProjectRequestRaw(payload)
	if err != nil {
		return nil, err
	}
	if len(canonical) > projectCommandJournalMaxRequest {
		return nil, fmt.Errorf("projects-v1: canonical request exceeds 64 KiB")
	}
	return canonical, nil
}

func canonicalProjectRequestRaw(payload []byte) ([]byte, error) {
	if !utf8.Valid(payload) || bytes.IndexByte(payload, 0) >= 0 {
		return nil, fmt.Errorf("projects-v1: canonical request is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("projects-v1: decode canonical request: %w", err)
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("projects-v1: canonical request must be a JSON object")
	}
	if err := validateDecodedProjectRequest(object); err != nil {
		return nil, err
	}
	return marshalCanonicalProjectJSON(object)
}

func marshalCanonicalProjectJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("projects-v1: encode canonical request: %w", err)
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

func validateProjectRequestValue(value reflect.Value) error {
	return validateProjectReflectValue(value)
}

func validateDecodedProjectRequest(value any) error {
	return validateDecodedProjectValue(value)
}

func forbiddenProjectJournalField(key string) bool {
	switch strings.ToLower(key) {
	case "authorization", "access_token", "refresh_token", "token", "x-idempotency-key", "x-request-id", "server_request_digest", "digest_key", "digest_key_version":
		return true
	default:
		return false
	}
}

func validateProjectCommandPayload(operation ProjectCommandOperation, canonical []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return fmt.Errorf("projects-v1: canonical command payload must be an object")
	}
	return validateDecodedCommandPayload(operation, object)
}

func journalFieldSet(fields ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		set[field] = struct{}{}
	}
	return set
}

func journalText(value any, maximum int, required bool) bool {
	text, ok := value.(string)
	return ok && validProjectText(text, maximum) && (!required || strings.TrimSpace(text) != "")
}

func journalRequiredText(value any, maximum int) bool { return journalText(value, maximum, true) }

func journalNullableText(object map[string]any, field string, maximum int) bool {
	value, present := object[field]
	return !present || value == nil || journalText(value, maximum, false)
}

func journalOptionalText(object map[string]any, field string, maximum int) bool {
	value, present := object[field]
	return !present || journalText(value, maximum, false)
}

func journalPositiveVersion(value any) bool {
	number, ok := value.(json.Number)
	if !ok || strings.ContainsAny(number.String(), ".eE+-") {
		return false
	}
	parsed, err := strconv.ParseUint(number.String(), 10, 64)
	return err == nil && parsed > 0 && parsed <= MaxExpectedVersion
}

func journalWorkItemStatus(value any) bool {
	status, ok := value.(string)
	return ok && (status == "todo" || status == "in_progress" || status == "paused" || status == "done")
}

func journalOptionalWorkItemStatus(object map[string]any, field string) bool {
	value, present := object[field]
	return !present || journalWorkItemStatus(value)
}

func journalNullableID(object map[string]any, field string) bool {
	value, present := object[field]
	if !present || value == nil {
		return true
	}
	id, ok := value.(string)
	return ok && validateOpaqueProjectID(id) == nil
}

func journalNonNullIDPresent(object map[string]any, field string) bool {
	value, present := object[field]
	return present && value != nil
}

func (j *ProjectCommandJournal) Prepare(ctx context.Context, scope Scope, input PrepareProjectCommandInput) (ProjectCommandRecord, error) {
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return ProjectCommandRecord{}, err
	}
	canonical, payloadHash, err := prepareProjectCommandPayload(input)
	if err != nil {
		return ProjectCommandRecord{}, err
	}
	path := j.path(scope)
	release := acquireProjectCommandJournalLock(path)
	defer release()
	return j.prepareLocked(ctx, path, scope, input, canonical, payloadHash)
}

func prepareProjectCommandPayload(input PrepareProjectCommandInput) ([]byte, string, error) {
	if err := validateProjectCommandIdentity(input.Operation, input.ProjectID, input.ResourceID); err != nil {
		return nil, "", err
	}
	canonical, err := CanonicalProjectRequestBytes(input.Payload)
	if err != nil {
		return nil, "", err
	}
	if err := validateProjectCommandPayload(input.Operation, canonical); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(sum[:]), nil
}

func (j *ProjectCommandJournal) prepareLocked(ctx context.Context, path string, scope Scope, input PrepareProjectCommandInput, canonical []byte, payloadHash string) (ProjectCommandRecord, error) {
	document, found, err := j.loadLocked(path, scope)
	if err != nil {
		return ProjectCommandRecord{}, err
	}
	if !found {
		document = newProjectCommandJournalDocument(scope)
	}
	document.Commands, _ = j.retainCurrent(document.Commands)
	if len(document.Commands) >= projectCommandJournalMaxUnresolved {
		return ProjectCommandRecord{}, fmt.Errorf("projects-v1: command journal has 1000 unresolved commands")
	}
	key, err := j.newIdempotencyKey(document.Commands)
	if err != nil {
		return ProjectCommandRecord{}, err
	}
	now := j.now().UTC()
	wire := projectCommandWire{IdempotencyKey: key, Operation: input.Operation, ProjectID: input.ProjectID, ResourceID: input.ResourceID,
		CanonicalPayload: append(json.RawMessage(nil), canonical...), LocalPayloadHash: payloadHash,
		State: ProjectCommandPending, CreatedAt: now, UpdatedAt: now}
	document.Commands = append(document.Commands, wire)
	if err := j.commitLocked(ctx, path, scope, document); err != nil {
		return ProjectCommandRecord{}, err
	}
	return wire.public(scope), nil
}

func (j *ProjectCommandJournal) Pending(ctx context.Context, scope Scope) ([]ProjectCommandRecord, error) {
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return nil, err
	}
	path := j.path(scope)
	release := acquireProjectCommandJournalLock(path)
	defer release()
	document, found, err := j.loadLocked(path, scope)
	if err != nil || !found {
		return []ProjectCommandRecord{}, err
	}
	var changed bool
	document.Commands, changed = j.retainCurrent(document.Commands)
	if changed {
		if err := j.commitLocked(ctx, path, scope, document); err != nil {
			return nil, err
		}
	}
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return nil, err
	}
	records := make([]ProjectCommandRecord, len(document.Commands))
	for index, command := range document.Commands {
		records[index] = command.public(scope)
	}
	sort.Slice(records, func(i, k int) bool {
		if records[i].CreatedAt.Equal(records[k].CreatedAt) {
			return records[i].IdempotencyKey < records[k].IdempotencyKey
		}
		return records[i].CreatedAt.Before(records[k].CreatedAt)
	})
	return records, nil
}

func (j *ProjectCommandJournal) VerifyReplay(ctx context.Context, scope Scope, key string, operation ProjectCommandOperation, projectID, resourceID string, payload any) (ProjectCommandRecord, error) {
	canonical, err := validateReplayInput(key, operation, projectID, resourceID, payload)
	if err != nil {
		return ProjectCommandRecord{}, err
	}
	path := j.path(scope)
	release := acquireProjectCommandJournalLock(path)
	defer release()
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return ProjectCommandRecord{}, err
	}
	document, found, err := j.loadLocked(path, scope)
	if err != nil {
		return ProjectCommandRecord{}, err
	}
	if !found {
		return ProjectCommandRecord{}, fmt.Errorf("projects-v1: unresolved command not found")
	}
	if err := j.commitRetainedCommands(ctx, path, scope, &document); err != nil {
		return ProjectCommandRecord{}, err
	}
	return findReplayCommand(document.Commands, scope, key, operation, projectID, resourceID, canonical)
}

func (j *ProjectCommandJournal) commitRetainedCommands(ctx context.Context, path string, scope Scope, document *projectCommandJournalDocument) error {
	var changed bool
	document.Commands, changed = j.retainCurrent(document.Commands)
	if changed {
		return j.commitLocked(ctx, path, scope, *document)
	}
	return nil
}

func (j *ProjectCommandJournal) MarkUnknown(ctx context.Context, scope Scope, key string) error {
	return j.update(ctx, scope, key, func(command *projectCommandWire) error {
		command.State = ProjectCommandUnknown
		command.UpdatedAt = j.now().UTC()
		return nil
	})
}

// DiscardRejected removes a command only after Cloud returned a conclusive
// rejection proving that no mutation receipt/readback can complete it. The UI
// owns any draft retained for a later, newly keyed retry.
func (j *ProjectCommandJournal) DiscardRejected(ctx context.Context, scope Scope, key string) error {
	if err := validateIdempotencyKey(key); err != nil {
		return err
	}
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	path := j.path(scope)
	release := acquireProjectCommandJournalLock(path)
	defer release()
	document, found, err := j.loadLocked(path, scope)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("projects-v1: unresolved command not found")
	}
	document.Commands, _ = j.retainCurrent(document.Commands)
	kept := make([]projectCommandWire, 0, len(document.Commands))
	found = false
	for _, command := range document.Commands {
		if command.IdempotencyKey == key {
			found = true
			continue
		}
		kept = append(kept, command)
	}
	if !found {
		return fmt.Errorf("projects-v1: unresolved command not found")
	}
	document.Commands = kept
	return j.commitLocked(ctx, path, scope, document)
}

func (j *ProjectCommandJournal) CompleteAfterReadback(ctx context.Context, scope Scope, key string, result CommandResult, readback ProjectCommandReadback) error {
	if err := validateCommandReadback(scope, result, readback); err != nil {
		return err
	}
	return j.remove(ctx, scope, key, result)
}

func (j *ProjectCommandJournal) CompleteAfterDeletion(ctx context.Context, scope Scope, key string, result CommandResult, readback ProjectCommandDeletionReadback) error {
	if !deletionReadbackMatches(scope, result, readback) {
		return fmt.Errorf("projects-v1: authoritative deletion readback does not match command result")
	}
	if !deletionResultProvesCompletion(scope, result) {
		return fmt.Errorf("projects-v1: command result does not prove project completion")
	}
	return j.remove(ctx, scope, key, result)
}

func deletionReadbackMatches(scope Scope, result CommandResult, readback ProjectCommandDeletionReadback) bool {
	return readback.Absent &&
		readback.ProjectScopeKey == scope.Key &&
		readback.ResourceKind == result.ResourceKind &&
		readback.ResourceID == result.ResourceID &&
		readback.ProjectID == result.ProjectID
}

func deletionResultProvesCompletion(scope Scope, result CommandResult) bool {
	return result.ProjectScopeKey == scope.Key &&
		result.ResourceKind == "work_item" &&
		result.ResourceID != "" &&
		result.ProjectID != "" &&
		result.CommittedVersion > 0 &&
		result.CommittedVersion <= MaxSafeJSONInteger &&
		validProjectText(result.OperationID, 200)
}

func (j *ProjectCommandJournal) update(ctx context.Context, scope Scope, key string, mutate func(*projectCommandWire) error) error {
	if err := validateIdempotencyKey(key); err != nil {
		return err
	}
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	path := j.path(scope)
	release := acquireProjectCommandJournalLock(path)
	defer release()
	document, found, err := j.loadLocked(path, scope)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("projects-v1: unresolved command not found")
	}
	var changed bool
	document.Commands, changed = j.retainCurrent(document.Commands)
	for index := range document.Commands {
		if document.Commands[index].IdempotencyKey == key {
			if err := mutate(&document.Commands[index]); err != nil {
				return err
			}
			return j.commitLocked(ctx, path, scope, document)
		}
	}
	if changed {
		if err := j.commitLocked(ctx, path, scope, document); err != nil {
			return err
		}
	}
	return fmt.Errorf("projects-v1: unresolved command not found")
}

func (j *ProjectCommandJournal) remove(ctx context.Context, scope Scope, key string, result CommandResult) error {
	if err := validateIdempotencyKey(key); err != nil {
		return err
	}
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	path := j.path(scope)
	release := acquireProjectCommandJournalLock(path)
	defer release()
	document, found, err := j.loadLocked(path, scope)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("projects-v1: unresolved command not found")
	}
	var expiredChanged bool
	document.Commands, expiredChanged = j.retainCurrent(document.Commands)
	kept, found, err := removeMatchingCommand(document.Commands, key, result)
	if err != nil {
		return err
	}
	if !found {
		if expiredChanged {
			if err := j.commitLocked(ctx, path, scope, document); err != nil {
				return err
			}
		}
		return fmt.Errorf("projects-v1: unresolved command not found")
	}
	document.Commands = kept
	return j.commitLocked(ctx, path, scope, document)
}

func (j *ProjectCommandJournal) commitLocked(ctx context.Context, path string, scope Scope, document projectCommandJournalDocument) error {
	if j.beforeCommit != nil {
		j.beforeCommit()
	}
	if err := j.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("projects-v1: encode command journal: %w", err)
	}
	payload = append(payload, '\n')
	if len(payload) > projectCommandJournalMaxBytes {
		return fmt.Errorf("projects-v1: command journal exceeds 10 MiB")
	}
	if err := fileutil.AtomicWriteFileStrict(path, payload, 0o600); err != nil {
		return fmt.Errorf("projects-v1: atomically replace command journal: %w", err)
	}
	return syncProjectCommandJournalDirectory(filepath.Dir(path))
}

func (j *ProjectCommandJournal) loadLocked(path string, scope Scope) (document projectCommandJournalDocument, found bool, err error) {
	payload, found, err := j.readJournalPayload(path)
	if err != nil || !found {
		return document, found, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return document, false, errors.Join(fmt.Errorf("projects-v1: decode command journal: %w", err), j.isolate(path))
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return document, false, errors.Join(err, j.isolate(path))
	}
	if err := validateProjectCommandJournalDocument(document, scope, j.now().UTC()); err != nil {
		return document, false, errors.Join(err, j.isolate(path))
	}
	return document, true, nil
}

func validateProjectCommandJournalDocument(document projectCommandJournalDocument, scope Scope, now time.Time) error {
	if document.Schema != projectCommandJournalSchema || document.SchemaVersion != projectCommandJournalSchemaVersion {
		return fmt.Errorf("projects-v1: unsupported command journal schema")
	}
	if document.Environment != scope.Environment || document.Binding != scope.Binding || document.ScopeKey != scope.Key {
		return fmt.Errorf("projects-v1: command journal identity mismatch")
	}
	if document.ContractVersion != ContractVersion || document.ContractHash != CanonicalContractSHA256() {
		return fmt.Errorf("projects-v1: command journal contract mismatch")
	}
	if document.Commands == nil || len(document.Commands) > projectCommandJournalMaxUnresolved {
		return fmt.Errorf("projects-v1: invalid command journal count")
	}
	return validateJournalCommands(document.Commands, now)
}

func validateProjectCommandJournalShape(payload []byte) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		return fmt.Errorf("projects-v1: decode command journal shape: %w", err)
	}
	documentFields := [...]string{"schema", "schema_version", "environment", "binding", "scope_key", "contract_version", "contract_hash", "commands"}
	if err := requireExactProjectFields(document, documentFields[:]...); err != nil {
		return fmt.Errorf("projects-v1: command journal: %w", err)
	}
	if err := requireNonNullProjectFields(document, documentFields[:], "command journal"); err != nil {
		return err
	}
	if err := validateNestedProjectFields(document, "environment", []string{"environment_id", "cloud_base_url"}, "command journal"); err != nil {
		return err
	}
	if err := validateNestedProjectFields(document, "binding", []string{"profile_id", "cloud_user_id", "binding_generation"}, "command journal"); err != nil {
		return err
	}
	var commands []json.RawMessage
	if err := json.Unmarshal(document["commands"], &commands); err != nil {
		return fmt.Errorf("projects-v1: command journal commands must be an array")
	}
	return validateJournalCommandShapes(commands)
}

func commandResultMatchesRoute(command projectCommandWire, result CommandResult) bool {
	if result.ProjectID == "" || result.ResourceID == "" {
		return false
	}
	matcher := commandResultMatcher(command.Operation)
	return matcher != nil && matcher(command, result)
}

func validateProjectCommandIdentity(operation ProjectCommandOperation, projectID, resourceID string) error {
	switch operation {
	case ProjectCommandCreate:
		if projectID != "" || resourceID != "" {
			return fmt.Errorf("projects-v1: project.create must not predeclare a route resource")
		}
	case ProjectCommandPatch, ProjectCommandInstructionPut, ProjectCommandWorkItemCreate:
		if err := validateOpaqueProjectID(projectID); err != nil {
			return err
		}
		if resourceID != "" {
			return fmt.Errorf("projects-v1: command must not predeclare a nested route resource")
		}
	case ProjectCommandWorkItemPatch, ProjectCommandWorkItemMove, ProjectCommandWorkItemDelete:
		if err := validateOpaqueProjectID(projectID); err != nil {
			return err
		}
		if err := validateOpaqueProjectID(resourceID); err != nil {
			return fmt.Errorf("projects-v1: invalid work_item_id")
		}
	default:
		return fmt.Errorf("projects-v1: invalid command operation")
	}
	return nil
}

func (j *ProjectCommandJournal) retainCurrent(commands []projectCommandWire) ([]projectCommandWire, bool) {
	cutoff := j.now().UTC().Add(-projectCommandJournalRetention)
	kept := make([]projectCommandWire, 0, len(commands))
	for _, command := range commands {
		if command.CreatedAt.Before(cutoff) {
			continue
		}
		kept = append(kept, command)
	}
	return kept, len(kept) != len(commands)
}

func (j *ProjectCommandJournal) newIdempotencyKey(commands []projectCommandWire) (string, error) {
	existing := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		existing[command.IdempotencyKey] = struct{}{}
	}
	for attempt := 0; attempt < 8; attempt++ {
		value := make([]byte, 32)
		j.randomMutex.Lock()
		_, err := io.ReadFull(j.random, value)
		j.randomMutex.Unlock()
		if err != nil {
			return "", fmt.Errorf("projects-v1: generate idempotency key: %w", err)
		}
		key := hex.EncodeToString(value)
		if _, duplicate := existing[key]; !duplicate {
			return key, nil
		}
	}
	return "", fmt.Errorf("projects-v1: generate unique idempotency key")
}

func (j *ProjectCommandJournal) validateCurrentScope(ctx context.Context, expected Scope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := expected.Validate(); err != nil {
		return err
	}
	current, err := j.scopes.CurrentProjectScope(ctx)
	if err != nil {
		return err
	}
	if err := current.Validate(); err != nil {
		return err
	}
	if current.Key != expected.Key {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "command journal operation belongs to a stale scope"}
	}
	return nil
}

func (j *ProjectCommandJournal) path(scope Scope) string {
	return filepath.Join(j.root, "project-commands", "v1", scope.Key, "project_commands.json")
}

func (j *ProjectCommandJournal) isolate(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("projects-v1: inspect command journal before isolation: %w", err)
	}
	var lastErr error
	for sequence := 0; sequence < 1000; sequence++ {
		target := fmt.Sprintf("%s.invalid-%s-%03d", path, j.now().UTC().Format("20060102T150405.000000000Z"), sequence)
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := os.Rename(path, target); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("projects-v1: isolate command journal: %w", lastErr)
}

func newProjectCommandJournalDocument(scope Scope) projectCommandJournalDocument {
	return projectCommandJournalDocument{Schema: projectCommandJournalSchema, SchemaVersion: projectCommandJournalSchemaVersion,
		Environment: scope.Environment, Binding: scope.Binding, ScopeKey: scope.Key,
		ContractVersion: ContractVersion, ContractHash: CanonicalContractSHA256(), Commands: []projectCommandWire{}}
}

func (wire projectCommandWire) public(scope Scope) ProjectCommandRecord {
	record := ProjectCommandRecord{ProjectScopeKey: scope.Key, IdempotencyKey: wire.IdempotencyKey, Operation: wire.Operation,
		ProjectID: wire.ProjectID, ResourceID: wire.ResourceID, CanonicalPayload: append(json.RawMessage(nil), wire.CanonicalPayload...),
		LocalPayloadHash: wire.LocalPayloadHash, State: wire.State, CreatedAt: wire.CreatedAt, UpdatedAt: wire.UpdatedAt}
	return record
}

func acquireProjectCommandJournalLock(path string) func() {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	projectCommandJournalLocks.Lock()
	entry := projectCommandJournalLocks.entries[key]
	if entry == nil {
		entry = &projectCommandJournalLockEntry{}
		projectCommandJournalLocks.entries[key] = entry
	}
	entry.refs++
	projectCommandJournalLocks.Unlock()
	entry.mutex.Lock()
	return func() {
		entry.mutex.Unlock()
		projectCommandJournalLocks.Lock()
		entry.refs--
		if entry.refs == 0 && projectCommandJournalLocks.entries[key] == entry {
			delete(projectCommandJournalLocks.entries, key)
		}
		projectCommandJournalLocks.Unlock()
	}
}

func syncProjectCommandJournalDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("projects-v1: open command journal directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("projects-v1: sync command journal directory: %w", err)
	}
	return nil
}
