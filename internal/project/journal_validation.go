package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func validateProjectReflectValue(value reflect.Value) error {
	value, done := dereferenceProjectValue(value)
	if done {
		return nil
	}
	switch value.Kind() {
	case reflect.String:
		return validateProjectReflectString(value.String())
	case reflect.Map:
		return validateProjectReflectMap(value)
	case reflect.Struct:
		return validateProjectReflectStruct(value)
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return validateProjectRawJSON(value.Bytes())
		}
		return validateProjectReflectSequence(value)
	case reflect.Array:
		return validateProjectReflectSequence(value)
	default:
		return nil
	}
}

func dereferenceProjectValue(value reflect.Value) (reflect.Value, bool) {
	if !value.IsValid() {
		return value, true
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return value, true
		}
		value = value.Elem()
	}
	return value, false
}

func validateProjectReflectString(value string) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return fmt.Errorf("projects-v1: canonical request contains invalid text")
	}
	return nil
}

func validateProjectRawJSON(value []byte) error {
	if !utf8.Valid(value) || bytes.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("projects-v1: canonical request contains invalid raw JSON")
	}
	return nil
}

func validateProjectReflectMap(value reflect.Value) error {
	iterator := value.MapRange()
	for iterator.Next() {
		if iterator.Key().Kind() == reflect.String {
			key := iterator.Key().String()
			if !utf8.ValidString(key) || strings.ContainsRune(key, 0) {
				return fmt.Errorf("projects-v1: canonical request contains invalid field name")
			}
		}
		if err := validateProjectReflectValue(iterator.Value()); err != nil {
			return err
		}
	}
	return nil
}

func validateProjectReflectStruct(value reflect.Value) error {
	for index := 0; index < value.NumField(); index++ {
		if value.Type().Field(index).PkgPath == "" {
			if err := validateProjectReflectValue(value.Field(index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateProjectReflectSequence(value reflect.Value) error {
	for index := 0; index < value.Len(); index++ {
		if err := validateProjectReflectValue(value.Index(index)); err != nil {
			return err
		}
	}
	return nil
}

func validateDecodedProjectValue(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		return validateDecodedProjectObject(typed)
	case []any:
		return validateDecodedProjectArray(typed)
	case string:
		return validateProjectReflectString(typed)
	case json.Number:
		if strings.ContainsAny(string(typed), "eE") {
			return fmt.Errorf("projects-v1: canonical request numbers must not use exponent notation")
		}
	}
	return nil
}

func validateDecodedProjectObject(object map[string]any) error {
	for key, child := range object {
		if forbiddenProjectJournalField(key) {
			return fmt.Errorf("projects-v1: canonical request must not contain transport or secret field %q", key)
		}
		if !validProjectText(key, 256) {
			return fmt.Errorf("projects-v1: canonical request contains invalid field name")
		}
		if err := validateDecodedProjectValue(child); err != nil {
			return err
		}
	}
	return nil
}

func validateDecodedProjectArray(array []any) error {
	for _, child := range array {
		if err := validateDecodedProjectValue(child); err != nil {
			return err
		}
	}
	return nil
}

type journalPayloadRule struct {
	allowed map[string]struct{}
	check   func(map[string]any) error
}

func validateDecodedCommandPayload(operation ProjectCommandOperation, object map[string]any) error {
	rule, err := journalRule(operation)
	if err != nil {
		return err
	}
	if err := rule.check(object); err != nil {
		return err
	}
	for field := range object {
		if _, ok := rule.allowed[field]; !ok {
			return fmt.Errorf("projects-v1: unknown command field %q", field)
		}
	}
	return nil
}

func journalRule(operation ProjectCommandOperation) (journalPayloadRule, error) {
	switch operation {
	case ProjectCommandCreate:
		return journalPayloadRule{journalFieldSet("name", "description", "human_description", "agent_instruction", "skill_id", "expert_id", "connector_id", "bindings"), validateProjectCreatePayload}, nil
	case ProjectCommandPatch:
		return journalPayloadRule{journalFieldSet("expected_version", "name", "description", "status"), validateProjectPatchPayload}, nil
	case ProjectCommandInstructionPut:
		return journalPayloadRule{journalFieldSet("expected_version", "human_description", "agent_instruction"), validateInstructionPutPayload}, nil
	case ProjectCommandWorkItemCreate:
		return journalPayloadRule{journalFieldSet("title", "description", "status", "assignee_user_id", "priority", "start_at", "due_at"), validateWorkItemCreatePayload}, nil
	case ProjectCommandWorkItemPatch:
		return journalPayloadRule{journalFieldSet("expected_version", "title", "description", "assignee_user_id", "priority", "start_at", "due_at"), validateWorkItemPatchPayload}, nil
	case ProjectCommandWorkItemMove:
		return journalPayloadRule{journalFieldSet("expected_version", "target_status", "before_work_item_id", "after_work_item_id"), validateWorkItemMovePayload}, nil
	case ProjectCommandWorkItemDelete:
		return journalPayloadRule{journalFieldSet("expected_version"), validateWorkItemDeletePayload}, nil
	default:
		return journalPayloadRule{}, fmt.Errorf("projects-v1: invalid command operation")
	}
}

func requireJournalFields(object map[string]any, names ...string) error {
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("projects-v1: command field %s is required", name)
		}
	}
	return nil
}

func validateProjectCreatePayload(object map[string]any) error {
	if err := requireJournalFields(object, "name"); err != nil {
		return err
	}
	if !journalRequiredText(object["name"], 200) || !journalNullableText(object, "description", 4_000) || !journalOptionalText(object, "human_description", 12_000) || !journalOptionalText(object, "agent_instruction", 48_000) {
		return fmt.Errorf("projects-v1: invalid project.create payload")
	}
	if err := validateProjectCreateBindingIDs(object); err != nil {
		return err
	}
	return validateProjectCreateBindings(object)
}

func validateProjectCreateBindingIDs(object map[string]any) error {
	_, hasBindings := object["bindings"]
	for _, field := range []string{"skill_id", "expert_id", "connector_id"} {
		value, present := object[field]
		if present && !journalRequiredText(value, 500) {
			return fmt.Errorf("projects-v1: invalid project.create %s", field)
		}
		if present && hasBindings {
			return fmt.Errorf("projects-v1: project.create binding formats cannot be mixed")
		}
	}
	return nil
}

func validateProjectCreateBindings(object map[string]any) error {
	value, present := object["bindings"]
	if !present {
		return nil
	}
	bindings, ok := value.([]any)
	if !ok || len(bindings) == 0 || len(bindings) > 100 {
		return fmt.Errorf("projects-v1: invalid project.create bindings")
	}
	positions := make(map[string]struct{}, len(bindings))
	for index, value := range bindings {
		kind, position, err := validateProjectCreateBinding(value, index)
		if err != nil {
			return err
		}
		key := kind + ":" + strconv.Itoa(position)
		if _, duplicate := positions[key]; duplicate {
			return fmt.Errorf("projects-v1: duplicate project.create binding position")
		}
		positions[key] = struct{}{}
	}
	return nil
}

func validateProjectCreateBinding(value any, index int) (string, int, error) {
	binding, ok := value.(map[string]any)
	if !ok || !journalExactFields(binding, "kind", "registry_ref", "position", "config") {
		return "", 0, fmt.Errorf("projects-v1: invalid project.create binding %d", index)
	}
	kind, ok := binding["kind"].(string)
	if !ok || !validProjectCreateBindingKind(kind) || !journalRequiredText(binding["registry_ref"], 500) {
		return "", 0, fmt.Errorf("projects-v1: invalid project.create binding %d", index)
	}
	position, ok := journalIntegerInRange(binding["position"], 0, 99)
	if !ok {
		return "", 0, fmt.Errorf("projects-v1: invalid project.create binding %d", index)
	}
	if config, ok := binding["config"].(map[string]any); !ok || config == nil {
		return "", 0, fmt.Errorf("projects-v1: invalid project.create binding %d", index)
	}
	return kind, position, nil
}

func validProjectCreateBindingKind(kind string) bool {
	return kind == "connector" || kind == "expert" || kind == "skill"
}

func journalExactFields(object map[string]any, fields ...string) bool {
	if len(object) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, present := object[field]; !present {
			return false
		}
	}
	return true
}

func journalIntegerInRange(value any, minimum, maximum int) (int, bool) {
	number, ok := value.(json.Number)
	if !ok || strings.ContainsAny(number.String(), ".eE+-") {
		return 0, false
	}
	parsed, err := strconv.Atoi(number.String())
	return parsed, err == nil && parsed >= minimum && parsed <= maximum
}

func validateProjectPatchPayload(object map[string]any) error {
	if err := requireJournalFields(object, "expected_version"); err != nil {
		return err
	}
	if !journalPositiveVersion(object["expected_version"]) {
		return fmt.Errorf("projects-v1: invalid project.patch expected_version")
	}
	if !validProjectPatchFields(object) {
		return fmt.Errorf("projects-v1: invalid project.patch payload")
	}
	return nil
}

func validProjectPatchFields(object map[string]any) bool {
	_, hasName := object["name"]
	_, hasDescription := object["description"]
	status, hasStatus := object["status"]
	if !validProjectPatchPresence(hasName, hasDescription, hasStatus) {
		return false
	}
	if hasName && !journalRequiredText(object["name"], 200) {
		return false
	}
	if !journalNullableText(object, "description", 4_000) {
		return false
	}
	return !hasStatus || status == string(ProjectActive) || status == string(ProjectArchived)
}

func validProjectPatchPresence(hasName, hasDescription, hasStatus bool) bool {
	return (hasName || hasDescription || hasStatus) && (!hasStatus || (!hasName && !hasDescription))
}

func validateInstructionPutPayload(object map[string]any) error {
	if err := requireJournalFields(object, "expected_version", "human_description", "agent_instruction"); err != nil {
		return err
	}
	if !journalPositiveVersion(object["expected_version"]) || !journalText(object["human_description"], 12_000, false) || !journalText(object["agent_instruction"], 48_000, false) {
		return fmt.Errorf("projects-v1: invalid instruction.put payload")
	}
	return nil
}

func validateWorkItemCreatePayload(object map[string]any) error {
	if err := requireJournalFields(object, "title"); err != nil {
		return err
	}
	if !journalRequiredText(object["title"], 300) || !journalNullableText(object, "description", 8_000) || !journalOptionalWorkItemStatus(object, "status") || !journalWorkItemV2Fields(object) {
		return fmt.Errorf("projects-v1: invalid work_item.create payload")
	}
	return nil
}

func validateWorkItemPatchPayload(object map[string]any) error {
	if err := requireJournalFields(object, "expected_version"); err != nil {
		return err
	}
	if !journalPositiveVersion(object["expected_version"]) || !journalWorkItemPatchHasMutation(object) || !journalWorkItemPatchFields(object) {
		return fmt.Errorf("projects-v1: invalid work_item.patch payload")
	}
	return nil
}

func journalWorkItemPatchHasMutation(object map[string]any) bool {
	_, hasTitle := object["title"]
	_, hasDescription := object["description"]
	_, hasAssignee := object["assignee_user_id"]
	_, hasPriority := object["priority"]
	_, hasStart := object["start_at"]
	_, hasDue := object["due_at"]
	return hasTitle || hasDescription || hasAssignee || hasPriority || hasStart || hasDue
}

func journalWorkItemPatchFields(object map[string]any) bool {
	title, hasTitle := object["title"]
	return (!hasTitle || journalRequiredText(title, 300)) && journalNullableText(object, "description", 8_000) && journalWorkItemV2Fields(object)
}

func journalWorkItemV2Fields(object map[string]any) bool {
	if !journalNullableID(object, "assignee_user_id") || !journalOptionalWorkItemPriority(object, "priority") {
		return false
	}
	start, startPresent, startValid := journalNullableTime(object, "start_at")
	due, duePresent, dueValid := journalNullableTime(object, "due_at")
	if !startValid || !dueValid {
		return false
	}
	return !startPresent || !duePresent || !start.After(due)
}

func journalOptionalWorkItemPriority(object map[string]any, field string) bool {
	value, present := object[field]
	if !present {
		return true
	}
	priority, ok := value.(string)
	return ok && (priority == "none" || priority == "low" || priority == "medium" || priority == "high" || priority == "urgent")
}

func journalNullableTime(object map[string]any, field string) (time.Time, bool, bool) {
	value, present := object[field]
	if !present || value == nil {
		return time.Time{}, false, true
	}
	text, ok := value.(string)
	if !ok {
		return time.Time{}, true, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	return parsed, true, err == nil && !parsed.IsZero()
}

func validateWorkItemMovePayload(object map[string]any) error {
	if err := requireJournalFields(object, "expected_version", "target_status"); err != nil {
		return err
	}
	hasBefore := journalNonNullIDPresent(object, "before_work_item_id")
	hasAfter := journalNonNullIDPresent(object, "after_work_item_id")
	if !journalPositiveVersion(object["expected_version"]) || !journalWorkItemStatus(object["target_status"]) || (hasBefore && hasAfter) || !journalNullableID(object, "before_work_item_id") || !journalNullableID(object, "after_work_item_id") {
		return fmt.Errorf("projects-v1: invalid work_item.move payload")
	}
	return nil
}

func validateWorkItemDeletePayload(object map[string]any) error {
	if err := requireJournalFields(object, "expected_version"); err != nil {
		return err
	}
	if !journalPositiveVersion(object["expected_version"]) {
		return fmt.Errorf("projects-v1: invalid work_item.delete expected_version")
	}
	return nil
}

func validateReplayInput(key string, operation ProjectCommandOperation, projectID, resourceID string, payload any) ([]byte, error) {
	if err := validateIdempotencyKey(key); err != nil {
		return nil, err
	}
	if err := validateProjectCommandIdentity(operation, projectID, resourceID); err != nil {
		return nil, err
	}
	canonical, err := CanonicalProjectRequestBytes(payload)
	if err != nil {
		return nil, err
	}
	if err := validateProjectCommandPayload(operation, canonical); err != nil {
		return nil, err
	}
	return canonical, nil
}

func findReplayCommand(commands []projectCommandWire, scope Scope, key string, operation ProjectCommandOperation, projectID, resourceID string, canonical []byte) (ProjectCommandRecord, error) {
	for _, command := range commands {
		if command.IdempotencyKey != key {
			continue
		}
		if command.Operation != operation || command.ProjectID != projectID || command.ResourceID != resourceID || !bytes.Equal(command.CanonicalPayload, canonical) {
			return ProjectCommandRecord{}, fmt.Errorf("projects-v1: payload substitution detected for idempotency key")
		}
		return command.public(scope), nil
	}
	return ProjectCommandRecord{}, fmt.Errorf("projects-v1: unresolved command not found")
}

func validateCommandReadback(scope Scope, result CommandResult, readback ProjectCommandReadback) error {
	if !validCommandResultProof(scope, result, readback) {
		return fmt.Errorf("projects-v1: command result does not prove project completion")
	}
	if result.ResourceKind != "project" && result.ResourceKind != "instruction" && result.ResourceKind != "work_item" {
		return fmt.Errorf("projects-v1: invalid command result resource kind")
	}
	if readback.ResourceKind != result.ResourceKind || readback.ResourceID != result.ResourceID || readback.ProjectID != result.ProjectID || readback.Version != result.CommittedVersion {
		return fmt.Errorf("projects-v1: authoritative readback does not match command result")
	}
	return nil
}

func validCommandResultProof(scope Scope, result CommandResult, readback ProjectCommandReadback) bool {
	return result.ProjectScopeKey == scope.Key && readback.ProjectScopeKey == scope.Key && validProjectText(result.OperationID, 200) && result.ResourceID != "" && result.ProjectID != "" && result.CommittedVersion > 0 && result.CommittedVersion <= MaxSafeJSONInteger && readback.Version <= MaxSafeJSONInteger
}

func removeMatchingCommand(commands []projectCommandWire, key string, result CommandResult) ([]projectCommandWire, bool, error) {
	kept := make([]projectCommandWire, 0, len(commands))
	found := false
	for _, command := range commands {
		if command.IdempotencyKey != key {
			kept = append(kept, command)
			continue
		}
		if !commandResultMatchesRoute(command, result) {
			return nil, false, fmt.Errorf("projects-v1: command result belongs to another route target")
		}
		found = true
	}
	return kept, found, nil
}

func (j *ProjectCommandJournal) readJournalPayload(path string) ([]byte, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("projects-v1: inspect command journal: %w", err)
	}
	if err := j.validateJournalFileInfo(path, info); err != nil {
		return nil, false, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("projects-v1: read command journal: %w", err)
	}
	if !utf8.Valid(payload) {
		return nil, false, errors.Join(fmt.Errorf("projects-v1: command journal is not valid UTF-8"), j.isolate(path))
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return nil, false, errors.Join(err, j.isolate(path))
	}
	if err := validateProjectCommandJournalShape(payload); err != nil {
		return nil, false, errors.Join(err, j.isolate(path))
	}
	return payload, true, nil
}

func (j *ProjectCommandJournal) validateJournalFileInfo(path string, info os.FileInfo) error {
	if info.Size() > projectCommandJournalMaxBytes {
		return errors.Join(fmt.Errorf("projects-v1: command journal exceeds 10 MiB"), j.isolate(path))
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		return errors.Join(fmt.Errorf("projects-v1: command journal permissions must be 0600"), j.isolate(path))
	}
	return nil
}

func validateJournalCommands(commands []projectCommandWire, now time.Time) error {
	seen := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		if err := validateJournalCommand(command, now); err != nil {
			return err
		}
		if _, duplicate := seen[command.IdempotencyKey]; duplicate {
			return fmt.Errorf("projects-v1: duplicate command idempotency key")
		}
		seen[command.IdempotencyKey] = struct{}{}
	}
	return nil
}

func validateJournalCommand(command projectCommandWire, now time.Time) error {
	if err := validateIdempotencyKey(command.IdempotencyKey); err != nil {
		return err
	}
	if err := validateProjectCommandIdentity(command.Operation, command.ProjectID, command.ResourceID); err != nil {
		return err
	}
	if command.State != ProjectCommandPending && command.State != ProjectCommandUnknown {
		return fmt.Errorf("projects-v1: invalid command journal state")
	}
	if !validJournalCommandTime(command, now) {
		return fmt.Errorf("projects-v1: invalid command journal timestamp")
	}
	if err := validateJournalCommandCanonical(command); err != nil {
		return err
	}
	sum := sha256.Sum256(command.CanonicalPayload)
	if command.LocalPayloadHash != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("projects-v1: local_payload_hash does not match canonical replay payload")
	}
	return nil
}

func validateJournalCommandCanonical(command projectCommandWire) error {
	canonical, err := canonicalProjectRequestRaw(command.CanonicalPayload)
	if err != nil || len(canonical) > projectCommandJournalMaxRequest || !bytes.Equal(canonical, command.CanonicalPayload) {
		return fmt.Errorf("projects-v1: invalid canonical replay payload")
	}
	return validateProjectCommandPayload(command.Operation, canonical)
}

func validJournalCommandTime(command projectCommandWire, now time.Time) bool {
	return !command.CreatedAt.IsZero() && command.CreatedAt.Location() == time.UTC && command.UpdatedAt.Location() == time.UTC && !command.UpdatedAt.Before(command.CreatedAt) && !command.CreatedAt.After(now) && !command.UpdatedAt.After(now)
}

func validateJournalCommandShapes(commands []json.RawMessage) error {
	fields := []string{"idempotency_key", "operation", "project_id", "resource_id", "canonical_payload", "local_payload_hash", "state", "created_at", "updated_at"}
	for index, raw := range commands {
		var command map[string]json.RawMessage
		if err := json.Unmarshal(raw, &command); err != nil {
			return fmt.Errorf("projects-v1: command journal command %d must be an object", index)
		}
		if err := requireExactProjectFields(command, fields...); err != nil {
			return fmt.Errorf("projects-v1: command journal command %d: %w", index, err)
		}
		if err := requireNonNullProjectFields(command, fields, fmt.Sprintf("command journal command %d", index)); err != nil {
			return err
		}
	}
	return nil
}

type commandResultMatchFunc func(projectCommandWire, CommandResult) bool

var commandResultMatchers = map[ProjectCommandOperation]commandResultMatchFunc{
	ProjectCommandCreate: func(_ projectCommandWire, result CommandResult) bool {
		return result.ResourceKind == "project" && result.ResourceID == result.ProjectID
	},
	ProjectCommandPatch: func(command projectCommandWire, result CommandResult) bool {
		return result.ResourceKind == "project" && result.ProjectID == command.ProjectID && result.ResourceID == command.ProjectID
	},
	ProjectCommandInstructionPut: func(command projectCommandWire, result CommandResult) bool {
		return result.ResourceKind == "instruction" && result.ProjectID == command.ProjectID && result.ResourceID == command.ProjectID
	},
	ProjectCommandWorkItemCreate: func(command projectCommandWire, result CommandResult) bool {
		return result.ResourceKind == "work_item" && result.ProjectID == command.ProjectID
	},
	ProjectCommandWorkItemPatch: func(command projectCommandWire, result CommandResult) bool {
		return result.ResourceKind == "work_item" && result.ProjectID == command.ProjectID && result.ResourceID == command.ResourceID
	},
	ProjectCommandWorkItemMove: func(command projectCommandWire, result CommandResult) bool {
		return result.ResourceKind == "work_item" && result.ProjectID == command.ProjectID && result.ResourceID == command.ResourceID
	},
	ProjectCommandWorkItemDelete: func(command projectCommandWire, result CommandResult) bool {
		return result.ResourceKind == "work_item" && result.ProjectID == command.ProjectID && result.ResourceID == command.ResourceID
	},
}

func commandResultMatcher(operation ProjectCommandOperation) commandResultMatchFunc {
	return commandResultMatchers[operation]
}
