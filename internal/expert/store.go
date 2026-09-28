package expert

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists mutable expert definitions and merges them with built-ins.
type Store struct {
	path       string
	builtins   []Definition
	persistent bool
	dbMu       sync.Mutex
	db         *sql.DB
}

// NewStore creates a Store that opens its database per operation.
func NewStore(path string, builtins []Definition) *Store {
	return &Store{path: strings.TrimSpace(path), builtins: cloneDefinitions(builtins)}
}

// NewPersistentStore creates a Store that reuses its database until Close.
func NewPersistentStore(path string, builtins []Definition) *Store {
	store := NewStore(path, builtins)
	store.persistent = true
	return store
}

// Close releases the persistent database, if one was opened.
func (s *Store) Close() error {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *Store) List(ctx context.Context) ([]Definition, error) {
	db, release, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	return listDefinitions(ctx, db)
}

func (s *Store) ListFiltered(ctx context.Context, filter ListFilter) ([]Definition, error) {
	defs, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	return filterDefinitions(defs, filter), nil
}

func (s *Store) Save(ctx context.Context, def Definition) error {
	normalized, err := normalizeMutableDefinition(def)
	if err != nil {
		return err
	}
	db, release, err := s.database(ctx)
	if err != nil {
		return err
	}
	defer release()
	return saveNormalizedDefinition(ctx, db, normalized)
}

func normalizeMutableDefinition(def Definition) (Definition, error) {
	id := strings.TrimSpace(def.ID)
	if IsBuiltinID(id) {
		return Definition{}, fmt.Errorf("official expert %q is read-only", id)
	}
	def.Source = SourceUser
	return normalizeDefinition(def)
}

func saveNormalizedDefinition(ctx context.Context, db *sql.DB, def Definition) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin expert save: %w", err)
	}
	defer rollbackTx(tx)
	if err := prepareDefinitionForSave(ctx, tx, &def); err != nil {
		return err
	}
	if err := saveDefinition(ctx, tx, def, time.Now().UnixMilli()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit expert save: %w", err)
	}
	return nil
}

func prepareDefinitionForSave(ctx context.Context, tx *sql.Tx, def *Definition) error {
	sortOrder, err := resolveSortOrder(ctx, tx, *def)
	if err != nil {
		return err
	}
	def.SortOrder = sortOrder
	if def.SystemPrompt != "" {
		return nil
	}
	def.SystemPrompt, err = existingSystemPrompt(ctx, tx, def.ID)
	return err
}

func (s *Store) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("expert id is required")
	}
	if IsBuiltinID(id) {
		return fmt.Errorf("official expert %q is read-only", id)
	}
	db, release, err := s.database(ctx)
	if err != nil {
		return err
	}
	defer release()
	if _, err := db.ExecContext(ctx, `DELETE FROM experts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete expert %q: %w", id, err)
	}
	return nil
}

// Reset rebuilds the expert table from the built-in catalog.
func (s *Store) Reset(ctx context.Context) error {
	db, release, err := s.database(ctx)
	if err != nil {
		return err
	}
	defer release()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin expert store reset: %w", err)
	}
	defer rollbackTx(tx)
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS experts`); err != nil {
		return fmt.Errorf("drop experts table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("create experts table: %w", err)
	}
	if err := seedDefinitions(ctx, tx, s.builtins); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit expert store reset: %w", err)
	}
	return nil
}

// DeleteUserDefinitions atomically removes only user-owned definitions while
// preserving the packaged or remotely cached official catalog.
func (s *Store) DeleteUserDefinitions(ctx context.Context) error {
	db, release, err := s.database(ctx)
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user expert deletion: %w", err)
	}
	defer rollbackTx(tx)
	if _, err := tx.ExecContext(ctx, `DELETE FROM experts WHERE source = ?`, SourceUser); err != nil {
		return fmt.Errorf("delete user experts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user expert deletion: %w", err)
	}
	return nil
}

// ReplaceOfficial atomically replaces the packaged/remote system catalog while
// preserving user-owned definitions in the same store.
func (s *Store) ReplaceOfficial(ctx context.Context, definitions []Definition) error {
	normalized := make([]Definition, 0, len(definitions))
	for index, definition := range definitions {
		definition.Source = SourceOfficial
		definition.SortOrder = (index + 1) * 10
		next, err := normalizeDefinition(definition)
		if err != nil {
			return err
		}
		normalized = append(normalized, next)
	}
	db, release, err := s.database(ctx)
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin official expert replacement: %w", err)
	}
	defer rollbackTx(tx)
	existingCreatedAt, err := officialCreationTimes(ctx, tx, len(normalized))
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM experts WHERE source = ?`, SourceOfficial); err != nil {
		return fmt.Errorf("delete official experts: %w", err)
	}
	now := time.Now().UnixMilli()
	for _, definition := range normalized {
		definition = preserveOfficialCreationTime(definition, existingCreatedAt)
		if err := saveDefinition(ctx, tx, definition, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit official expert replacement: %w", err)
	}
	return nil
}

func preserveOfficialCreationTime(definition Definition, existing map[string]int64) Definition {
	if definition.CreatedAt <= 0 {
		definition.CreatedAt = existing[definition.ID]
	}
	return definition
}

func officialCreationTimes(ctx context.Context, tx *sql.Tx, capacity int) (map[string]int64, error) {
	createdByID := make(map[string]int64, capacity)
	rows, err := tx.QueryContext(ctx, `SELECT id, created_at FROM experts WHERE source = ?`, SourceOfficial)
	if err != nil {
		return nil, fmt.Errorf("list official expert creation times: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var createdAt int64
		if err := rows.Scan(&id, &createdAt); err != nil {
			return nil, fmt.Errorf("scan official expert creation time: %w", err)
		}
		createdByID[id] = createdAt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate official expert creation times: %w", err)
	}
	return createdByID, nil
}

func syncBuiltinDefinitions(ctx context.Context, db *sql.DB, builtins []Definition) error {
	current, err := listDefinitions(ctx, db)
	if err != nil {
		return err
	}
	pending, err := pendingBuiltinDefinitions(current, builtins)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	return saveBuiltinDefinitions(ctx, db, pending)
}

func pendingBuiltinDefinitions(current, builtins []Definition) ([]Definition, error) {
	byID := make(map[string]Definition, len(current))
	for _, def := range current {
		byID[def.ID] = def
	}
	pending := make([]Definition, 0, len(builtins))
	for _, def := range builtins {
		def.Source = SourceOfficial
		normalized, err := normalizeDefinition(def)
		if err != nil {
			return nil, err
		}
		if stored, ok := byID[normalized.ID]; ok && reflect.DeepEqual(stored, normalized) {
			continue
		} else if ok {
			normalized.CreatedAt = stored.CreatedAt
			normalized.UpdatedAt = stored.UpdatedAt
			if reflect.DeepEqual(stored, normalized) {
				continue
			}
		}
		pending = append(pending, normalized)
	}
	return pending, nil
}

func saveBuiltinDefinitions(ctx context.Context, db *sql.DB, pending []Definition) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin expert builtin sync: %w", err)
	}
	defer rollbackTx(tx)
	now := time.Now().UnixMilli()
	for _, def := range pending {
		if err := saveDefinition(ctx, tx, def, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit expert builtin sync: %w", err)
	}
	return nil
}

type storedUserDataRow struct {
	id          string
	rawMode     string
	name        string
	promptsJSON string
	membersJSON string
}

type userDataUpdate struct {
	id      string
	prompts string
	members string
}

func migrateStoredUserData(ctx context.Context, db *sql.DB) error {
	updates, err := loadStoredUserDataUpdates(ctx, db)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	return saveStoredUserDataUpdates(ctx, db, updates)
}

func loadStoredUserDataUpdates(ctx context.Context, db *sql.DB) ([]userDataUpdate, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, mode, name, prompts_json, members_json
FROM experts
WHERE source = ?`, SourceUser)
	if err != nil {
		return nil, fmt.Errorf("list user expert data for migration: %w", err)
	}
	defer rows.Close()
	var updates []userDataUpdate
	for rows.Next() {
		update, changed, err := scanStoredUserDataUpdate(rows)
		if err != nil {
			return nil, err
		}
		if changed {
			updates = append(updates, update)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close user expert data migration rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user expert data migration rows: %w", err)
	}
	return updates, nil
}

func scanStoredUserDataUpdate(rows *sql.Rows) (userDataUpdate, bool, error) {
	var row storedUserDataRow
	if err := rows.Scan(&row.id, &row.rawMode, &row.name, &row.promptsJSON, &row.membersJSON); err != nil {
		return userDataUpdate{}, false, fmt.Errorf("scan user expert data for migration: %w", err)
	}
	mode, err := ParseMode(row.rawMode)
	if err != nil {
		return userDataUpdate{}, false, fmt.Errorf("expert %q: %w", row.id, err)
	}
	prompts, nextPrompts, err := normalizedStoredPrompts(row, mode)
	if err != nil {
		return userDataUpdate{}, false, err
	}
	members, nextMembers, err := normalizedStoredMembers(row, mode)
	if err != nil {
		return userDataUpdate{}, false, err
	}
	if reflect.DeepEqual(prompts, nextPrompts) && reflect.DeepEqual(members, nextMembers) {
		return userDataUpdate{}, false, nil
	}
	update, err := encodeUserDataUpdate(row.id, nextPrompts, nextMembers)
	if err != nil {
		return userDataUpdate{}, false, err
	}
	return update, true, nil
}

func normalizedStoredPrompts(row storedUserDataRow, mode Mode) ([]string, []string, error) {
	var prompts []string
	if err := decodeColumn(row.id, "prompts_json", row.promptsJSON, &prompts); err != nil {
		return nil, nil, err
	}
	next := trimStrings(prompts)
	switch {
	case len(next) > 5:
		next = append([]string(nil), next[:5]...)
	case len(next) < 3:
		next = completeLegacyPrompts(string(mode), row.name, next)
	}
	return prompts, next, nil
}

func normalizedStoredMembers(row storedUserDataRow, mode Mode) ([]Member, []Member, error) {
	var members []Member
	if err := decodeColumn(row.id, "members_json", row.membersJSON, &members); err != nil {
		return nil, nil, err
	}
	next := normalizeMembers(mode, members)
	if mode == ModeTeam {
		next = completeLegacyTeamMembers(row.name, next)
	}
	next, err := normalizeAndValidateMembers(row.id, mode, next)
	if err != nil {
		return nil, nil, err
	}
	return members, next, nil
}

func encodeUserDataUpdate(id string, prompts []string, members []Member) (userDataUpdate, error) {
	encodedPrompts, err := marshalColumn(id, "prompts_json", prompts)
	if err != nil {
		return userDataUpdate{}, err
	}
	encodedMembers, err := marshalColumn(id, "members_json", members)
	if err != nil {
		return userDataUpdate{}, err
	}
	return userDataUpdate{id: id, prompts: encodedPrompts, members: encodedMembers}, nil
}

func saveStoredUserDataUpdates(ctx context.Context, db *sql.DB, updates []userDataUpdate) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user expert data migration: %w", err)
	}
	defer rollbackTx(tx)
	now := time.Now().UnixMilli()
	for _, update := range updates {
		if _, err := tx.ExecContext(ctx, `
UPDATE experts SET prompts_json = ?, members_json = ?, updated_at = ? WHERE id = ?`,
			update.prompts, update.members, now, update.id); err != nil {
			return fmt.Errorf("migrate expert %q data: %w", update.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user expert data migration: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'experts'`).Scan(&count); err != nil {
		return false, fmt.Errorf("check experts table: %w", err)
	}
	return count > 0, nil
}

func tableRowCount(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM experts`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count experts rows: %w", err)
	}
	return count, nil
}

func ensureSchemaColumns(ctx context.Context, db *sql.DB) error {
	hasSystemPrompt, err := tableHasColumn(ctx, db, "system_prompt")
	if err != nil {
		return err
	}
	if !hasSystemPrompt {
		if _, err := db.ExecContext(ctx, `ALTER TABLE experts ADD COLUMN system_prompt TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add experts.system_prompt column: %w", err)
		}
	}
	hasSource, err := tableHasColumn(ctx, db, "source")
	if err != nil {
		return err
	}
	if !hasSource {
		if _, err := db.ExecContext(ctx, `ALTER TABLE experts ADD COLUMN source TEXT NOT NULL DEFAULT 'user'`); err != nil {
			return fmt.Errorf("add experts.source column: %w", err)
		}
	}
	return nil
}

func tableHasColumn(ctx context.Context, db *sql.DB, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(experts)`)
	if err != nil {
		return false, fmt.Errorf("inspect experts schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan experts schema: %w", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate experts schema: %w", err)
	}
	return false, nil
}

func initialize(ctx context.Context, db *sql.DB, builtins []Definition) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin expert store init: %w", err)
	}
	defer rollbackTx(tx)
	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("create experts table: %w", err)
	}
	if err := seedDefinitions(ctx, tx, builtins); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit expert store init: %w", err)
	}
	return nil
}

func seedExistingTable(ctx context.Context, db *sql.DB, builtins []Definition) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin expert store seed: %w", err)
	}
	defer rollbackTx(tx)
	if err := seedDefinitions(ctx, tx, builtins); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit expert store seed: %w", err)
	}
	return nil
}

func seedDefinitions(ctx context.Context, tx *sql.Tx, builtins []Definition) error {
	now := time.Now().UnixMilli()
	for _, def := range builtins {
		def.Source = SourceOfficial
		normalized, err := normalizeDefinition(def)
		if err != nil {
			return err
		}
		if err := insertDefinition(ctx, tx, normalized, now); err != nil {
			return err
		}
	}
	return nil
}

func rollbackTx(tx *sql.Tx) {
	_ = tx.Rollback()
}

func insertDefinition(ctx context.Context, tx *sql.Tx, def Definition, now int64) error {
	return saveDefinition(ctx, tx, def, now)
}

func saveDefinition(ctx context.Context, tx *sql.Tx, def Definition, now int64) error {
	categories, err := marshalColumn(def.ID, "categories_json", nonNilStrings(def.Categories))
	if err != nil {
		return err
	}
	tags, err := marshalColumn(def.ID, "tags_json", nonNilStrings(def.Tags))
	if err != nil {
		return err
	}
	members, err := marshalColumn(def.ID, "members_json", nonNilMembers(def.Members))
	if err != nil {
		return err
	}
	prompts, err := marshalColumn(def.ID, "prompts_json", nonNilStrings(def.Prompts))
	if err != nil {
		return err
	}
	createdAt := def.CreatedAt
	if createdAt <= 0 {
		createdAt = now
	}
	updatedAt := def.UpdatedAt
	if updatedAt <= 0 {
		updatedAt = now
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO experts (
	id, source, mode, name, role, description, system_prompt, categories_json, tags_json,
	icon_key, tone, avatar_key, members_json, prompts_json, sort_order,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	source = excluded.source,
	mode = excluded.mode,
	name = excluded.name,
	role = excluded.role,
	description = excluded.description,
	system_prompt = excluded.system_prompt,
	categories_json = excluded.categories_json,
	tags_json = excluded.tags_json,
	icon_key = excluded.icon_key,
	tone = excluded.tone,
	avatar_key = excluded.avatar_key,
	members_json = excluded.members_json,
	prompts_json = excluded.prompts_json,
	sort_order = excluded.sort_order,
	updated_at = excluded.updated_at`,
		def.ID, def.Source, string(def.Mode), def.Name, def.Role, def.Description, def.SystemPrompt, categories, tags,
		def.IconKey, def.Tone, def.AvatarKey, members, prompts, def.SortOrder, createdAt, updatedAt,
	); err != nil {
		return fmt.Errorf("save expert %q: %w", def.ID, err)
	}
	return nil
}

func resolveSortOrder(ctx context.Context, tx *sql.Tx, def Definition) (int, error) {
	if def.SortOrder != 0 {
		return def.SortOrder, nil
	}
	var existing sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT sort_order FROM experts WHERE id = ?`, def.ID).Scan(&existing); err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("lookup expert %q sort_order: %w", def.ID, err)
	}
	if existing.Valid {
		return int(existing.Int64), nil
	}
	var maxOrder sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT max(sort_order) FROM experts`).Scan(&maxOrder); err != nil {
		return 0, fmt.Errorf("lookup next expert sort_order: %w", err)
	}
	if maxOrder.Valid {
		return int(maxOrder.Int64) + 10, nil
	}
	return 10, nil
}

func existingSystemPrompt(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var prompt sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT system_prompt FROM experts WHERE id = ?`, id).Scan(&prompt); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("lookup expert %q system_prompt: %w", id, err)
	}
	if prompt.Valid {
		return strings.TrimSpace(prompt.String), nil
	}
	return "", nil
}

func normalizeDefinition(def Definition) (Definition, error) {
	def, err := normalizeDefinitionIdentity(def)
	if err != nil {
		return Definition{}, err
	}
	def = normalizeDefinitionPresentation(def)
	def.Prompts = trimStrings(def.Prompts)
	if err := validatePrompts(def.ID, def.Prompts); err != nil {
		return Definition{}, err
	}
	def.Members, err = normalizeAndValidateMembers(def.ID, def.Mode, def.Members)
	if err != nil {
		return Definition{}, err
	}
	return def, nil
}

func normalizeDefinitionIdentity(def Definition) (Definition, error) {
	def.ID = strings.TrimSpace(def.ID)
	if def.ID == "" {
		return Definition{}, fmt.Errorf("expert id is required")
	}
	source, err := normalizeDefinitionSource(def.ID, def.Source)
	if err != nil {
		return Definition{}, err
	}
	def.Source = source
	def.Mode, err = ParseMode(string(def.Mode))
	if err != nil {
		return Definition{}, err
	}
	def.Name = strings.TrimSpace(def.Name)
	if def.Name == "" {
		return Definition{}, fmt.Errorf("expert %q name is required", def.ID)
	}
	def.Role = strings.TrimSpace(def.Role)
	if def.Role == "" {
		return Definition{}, fmt.Errorf("expert %q role is required", def.ID)
	}
	return def, nil
}

func normalizeDefinitionSource(id, source string) (string, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	if source == "" {
		source = SourceUser
	}
	if source != SourceOfficial && source != SourceUser {
		return "", fmt.Errorf("expert %q has invalid source %q", id, source)
	}
	return source, nil
}

func normalizeDefinitionPresentation(def Definition) Definition {
	def.Description = strings.TrimSpace(def.Description)
	def.SystemPrompt = strings.TrimSpace(def.SystemPrompt)
	def.IconKey = strings.TrimSpace(def.IconKey)
	if def.IconKey == "" {
		def.IconKey = defaultDefinitionIcon(def.Mode)
	}
	def.Tone = strings.TrimSpace(def.Tone)
	if def.Tone == "" {
		def.Tone = "stone"
	}
	def.AvatarKey = strings.TrimSpace(def.AvatarKey)
	def.Categories = trimStrings(def.Categories)
	def.Tags = trimStrings(def.Tags)
	return def
}

func defaultDefinitionIcon(mode Mode) string {
	if mode == ModeTeam {
		return "users"
	}
	return "bot"
}

func marshalColumn(id, column string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("expert %q %s: %w", id, column, err)
	}
	return string(data), nil
}

func listDefinitions(ctx context.Context, db *sql.DB) ([]Definition, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, source, mode, name, role, description, system_prompt, categories_json, tags_json,
	icon_key, tone, avatar_key, members_json, prompts_json, sort_order, created_at, updated_at
FROM experts
ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("list experts: %w", err)
	}
	defer rows.Close()

	var defs []Definition
	for rows.Next() {
		var def Definition
		var mode string
		var categoriesJSON, tagsJSON, membersJSON, promptsJSON string
		var avatar sql.NullString
		if err := rows.Scan(
			&def.ID, &def.Source, &mode, &def.Name, &def.Role, &def.Description, &def.SystemPrompt, &categoriesJSON, &tagsJSON,
			&def.IconKey, &def.Tone, &avatar, &membersJSON, &promptsJSON, &def.SortOrder, &def.CreatedAt, &def.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan expert row: %w", err)
		}
		parsedMode, err := ParseMode(mode)
		if err != nil {
			return nil, fmt.Errorf("expert %q: %w", def.ID, err)
		}
		def.Mode = parsedMode
		if avatar.Valid {
			def.AvatarKey = avatar.String
		}
		if err := decodeColumn(def.ID, "categories_json", categoriesJSON, &def.Categories); err != nil {
			return nil, err
		}
		if err := decodeColumn(def.ID, "tags_json", tagsJSON, &def.Tags); err != nil {
			return nil, err
		}
		if err := decodeColumn(def.ID, "members_json", membersJSON, &def.Members); err != nil {
			return nil, err
		}
		def.Members = normalizeMembers(def.Mode, def.Members)
		if err := decodeColumn(def.ID, "prompts_json", promptsJSON, &def.Prompts); err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experts: %w", err)
	}
	return defs, nil
}

func decodeColumn(id, column, raw string, dest any) error {
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		return fmt.Errorf("expert %q %s: %w", id, column, err)
	}
	return nil
}

func nonNilMembers(members []Member) []Member {
	if members == nil {
		return []Member{}
	}
	return members
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func trimStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func filterDefinitions(defs []Definition, filter ListFilter) []Definition {
	category := strings.TrimSpace(filter.Category)
	if category == "全部" || strings.EqualFold(category, "all") {
		category = ""
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	if filter.Mode == "" && category == "" && query == "" {
		return defs
	}
	out := make([]Definition, 0, len(defs))
	for _, def := range defs {
		if filter.Mode != "" && def.Mode != filter.Mode {
			continue
		}
		if category != "" && !containsFold(def.Categories, category) {
			continue
		}
		if query != "" && !definitionMatchesQuery(def, query) {
			continue
		}
		out = append(out, def)
	}
	return out
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func definitionMatchesQuery(def Definition, query string) bool {
	fields := []string{def.Name, def.Role, def.Description}
	fields = append(fields, def.Categories...)
	fields = append(fields, def.Tags...)
	fields = append(fields, def.Prompts...)
	for _, member := range def.Members {
		fields = append(fields, member.Name, member.Role, member.Task, member.Output)
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

const schemaSQL = `
CREATE TABLE experts (
	id TEXT PRIMARY KEY,
	source TEXT NOT NULL DEFAULT 'user',
	mode TEXT NOT NULL,
	name TEXT NOT NULL,
	role TEXT NOT NULL,
	description TEXT NOT NULL,
	system_prompt TEXT NOT NULL DEFAULT '',
	categories_json TEXT NOT NULL,
	tags_json TEXT NOT NULL,
	icon_key TEXT NOT NULL,
	tone TEXT NOT NULL,
	avatar_key TEXT,
	members_json TEXT NOT NULL,
	prompts_json TEXT NOT NULL,
	sort_order INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
)`
