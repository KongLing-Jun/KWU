package automation

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kwu/internal/config"
	"kwu/internal/fileutil"

	_ "modernc.org/sqlite"
)

const (
	tasksFileName         = "tasks.json"
	templatesFileName     = "templates.json"
	templatesDBFileName   = "templates.sqlite"
	templatesMigrationID  = "templates-json"
	tasksV2BackupFileName = "tasks.v1.backup.json"
	tasksV2MarkerFileName = "tasks-v2.migrated"
)

// Store reads and writes automation state below one automation root directory.
type Store struct {
	dir        string
	persistent bool
	ownerProbe OwnerProbe
	taskMu     sync.Mutex
	runIndexMu sync.Mutex
	dbMu       sync.Mutex
	db         *sql.DB
}

type tasksFile struct {
	Tasks []Task `json:"tasks"`
}

type templatesFile struct {
	Templates []TaskTemplate `json:"templates"`
}

// DefaultDir returns the default automation state directory.
func DefaultDir() string {
	return filepath.Join(config.KWUHomeDir(), "automation")
}

// NewStore creates a Store rooted at dir, or DefaultDir when dir is empty.
func NewStore(dir string) *Store {
	if dir == "" {
		dir = DefaultDir()
	}
	return &Store{dir: dir}
}

// NewPersistentStore creates a Store that reuses its templates database until
// Close is called.
func NewPersistentStore(dir string) *Store {
	store := NewStore(dir)
	store.persistent = true
	return store
}

// NewLocalPersistentStore enables conservative process recovery for a profile
// owned by this machine. The probe must never infer death from lease expiry.
func NewLocalPersistentStore(dir string, probe OwnerProbe) *Store {
	store := NewPersistentStore(dir)
	store.ownerProbe = probe
	return store
}

// Close releases the persistent templates database, if one was opened.
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

// LoadTasks reads tasks.json, returning an empty list when it is missing or empty.
func (s *Store) LoadTasks() ([]Task, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	return s.loadTasksUnlocked()
}

func (s *Store) loadTasksUnlocked() ([]Task, error) {
	b, err := os.ReadFile(s.tasksPath())
	if errors.Is(err, os.ErrNotExist) {
		return []Task{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load automation tasks: %w", err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return []Task{}, nil
	}

	var file tasksFile
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, fmt.Errorf("load automation tasks: decode %s: %w", s.tasksPath(), err)
	}
	filtered, removedProjectFields := removeLegacyProjectAutomationTasks(file.Tasks)
	normalized, err := normalizeTasks(filtered)
	if err != nil {
		return nil, err
	}
	if err := s.persistLoadedTaskRepairs(b, normalized, tasksNeedV2Migration(file.Tasks), removedProjectFields); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (s *Store) persistLoadedTaskRepairs(original []byte, tasks []Task, needsV2Migration, removedProjectFields bool) error {
	if needsV2Migration {
		return s.persistTasksV2Migration(original, tasks)
	}
	if err := s.repairTasksV2Marker(); err != nil {
		return err
	}
	if !removedProjectFields {
		return nil
	}
	if err := s.saveTasksUnlocked(tasks); err != nil {
		return fmt.Errorf("remove legacy project automation tasks: %w", err)
	}
	return nil
}

func removeLegacyProjectAutomationTasks(tasks []Task) ([]Task, bool) {
	filtered := make([]Task, 0, len(tasks))
	changed := false
	for _, task := range tasks {
		linked := false
		for _, field := range []string{"projectId", "cloudProject"} {
			raw, exists := task.UnknownFields[field]
			if !exists {
				continue
			}
			changed = true
			delete(task.UnknownFields, field)
			trimmed := bytes.TrimSpace(raw)
			if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) && !bytes.Equal(trimmed, []byte(`""`)) {
				linked = true
			}
		}
		if !linked {
			filtered = append(filtered, task)
		}
	}
	return filtered, changed
}

func (s *Store) repairTasksV2Marker() error {
	backupPath := filepath.Join(s.dir, tasksV2BackupFileName)
	markerPath := filepath.Join(s.dir, tasksV2MarkerFileName)
	if _, err := os.Stat(backupPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("migrate automation tasks v2: inspect backup: %w", err)
	}
	if _, err := os.Stat(markerPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("migrate automation tasks v2: inspect marker: %w", err)
	}
	marker := []byte(time.Now().UTC().Format(time.RFC3339Nano) + "\n")
	if err := fileutil.AtomicWriteFile(markerPath, marker, 0o600); err != nil {
		return fmt.Errorf("migrate automation tasks v2: repair marker: %w", err)
	}
	return nil
}

func tasksNeedV2Migration(tasks []Task) bool {
	for _, task := range tasks {
		if task.SchemaVersion < CurrentTaskSchemaVersion || task.Revision < 1 {
			return true
		}
	}
	return false
}

func (s *Store) persistTasksV2Migration(original []byte, normalized []Task) error {
	backupPath := filepath.Join(s.dir, tasksV2BackupFileName)
	if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
		if err := fileutil.AtomicWriteFile(backupPath, original, 0o600); err != nil {
			return fmt.Errorf("migrate automation tasks v2: backup: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("migrate automation tasks v2: inspect backup: %w", err)
	}
	if err := s.saveTasksUnlocked(normalized); err != nil {
		return fmt.Errorf("migrate automation tasks v2: %w", err)
	}
	marker := []byte(time.Now().UTC().Format(time.RFC3339Nano) + "\n")
	if err := fileutil.AtomicWriteFile(filepath.Join(s.dir, tasksV2MarkerFileName), marker, 0o600); err != nil {
		return fmt.Errorf("migrate automation tasks v2: marker: %w", err)
	}
	return nil
}

// SaveTasks validates and atomically replaces tasks.json.
func (s *Store) SaveTasks(tasks []Task) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	return s.saveTasksUnlocked(tasks)
}

// RecordTaskRun updates scheduler-owned metadata under the same lock used by
// revisioned task edits. It never replaces the task definition or revision, so
// a run finishing concurrently with an editor save cannot restore stale input.
func (s *Store) RecordTaskRun(taskID, runID string, at time.Time) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	tasks, err := s.loadTasksUnlocked()
	if err != nil {
		return err
	}
	index, task, err := findTaskIndex(tasks, strings.TrimSpace(taskID))
	if err != nil {
		return err
	}
	task.LastRunID = strings.TrimSpace(runID)
	task.LastRunAt = at.UTC()
	tasks[index] = task
	return s.saveTasksUnlocked(tasks)
}

func (s *Store) saveTasksUnlocked(tasks []Task) error {
	normalized, err := normalizeTasks(tasks)
	if err != nil {
		return err
	}
	if normalized == nil {
		normalized = []Task{}
	}

	b, err := json.MarshalIndent(tasksFile{Tasks: normalized}, "", "  ")
	if err != nil {
		return fmt.Errorf("save automation tasks: encode: %w", err)
	}
	b = append(b, '\n')
	if err := fileutil.AtomicWriteFile(s.tasksPath(), b, 0o600); err != nil {
		return fmt.Errorf("save automation tasks: %w", err)
	}
	return nil
}

// Task returns one task by ID.
func (s *Store) Task(id string) (Task, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	tasks, err := s.loadTasksUnlocked()
	if err != nil {
		return Task{}, err
	}
	return findTask(tasks, id)
}

// CreateTask atomically appends one task with revision one.
func (s *Store) CreateTask(task Task) (Task, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	tasks, err := s.loadTasksUnlocked()
	if err != nil {
		return Task{}, err
	}
	for _, existing := range tasks {
		if strings.TrimSpace(existing.ID) == strings.TrimSpace(task.ID) {
			return Task{}, fmt.Errorf("%w: %s", ErrDuplicateTaskID, strings.TrimSpace(task.ID))
		}
	}
	now := time.Now().UTC()
	task.SchemaVersion = CurrentTaskSchemaVersion
	task.Revision = 1
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	normalized, err := NormalizeTask(task)
	if err != nil {
		return Task{}, err
	}
	if err := s.saveTasksUnlocked(append(tasks, normalized)); err != nil {
		return Task{}, err
	}
	return normalized, nil
}

// UpdateTask atomically replaces one task when expectedRevision is current.
func (s *Store) UpdateTask(task Task, expectedRevision int64) (Task, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	tasks, err := s.loadTasksUnlocked()
	if err != nil {
		return Task{}, err
	}
	index, existing, err := findTaskIndex(tasks, task.ID)
	if err != nil {
		return Task{}, err
	}
	if existing.Revision != expectedRevision {
		return Task{}, taskRevisionConflict(existing.ID, expectedRevision, existing.Revision)
	}
	task.SchemaVersion = CurrentTaskSchemaVersion
	task.Revision = existing.Revision + 1
	task.CreatedAt = existing.CreatedAt
	task.UpdatedAt = time.Now().UTC()
	task.LastRunAt = existing.LastRunAt
	task.LastRunID = existing.LastRunID
	if len(task.UnknownFields) == 0 {
		task.UnknownFields = cloneRawFields(existing.UnknownFields)
	}
	normalized, err := NormalizeTask(task)
	if err != nil {
		return Task{}, err
	}
	tasks[index] = normalized
	if err := s.saveTasksUnlocked(tasks); err != nil {
		return Task{}, err
	}
	return normalized, nil
}

// SetTaskEnabled changes only the enabled flag under revision control.
func (s *Store) SetTaskEnabled(id string, enabled bool, expectedRevision int64) (Task, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	tasks, err := s.loadTasksUnlocked()
	if err != nil {
		return Task{}, err
	}
	index, task, err := findTaskIndex(tasks, id)
	if err != nil {
		return Task{}, err
	}
	if task.Revision != expectedRevision {
		return Task{}, taskRevisionConflict(task.ID, expectedRevision, task.Revision)
	}
	task.Enabled = enabled
	task.Revision++
	task.UpdatedAt = time.Now().UTC()
	tasks[index] = task
	if err := s.saveTasksUnlocked(tasks); err != nil {
		return Task{}, err
	}
	return task, nil
}

// DeleteTask atomically removes one task under revision control.
func (s *Store) DeleteTask(id string, expectedRevision int64) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	tasks, err := s.loadTasksUnlocked()
	if err != nil {
		return err
	}
	index, task, err := findTaskIndex(tasks, id)
	if err != nil {
		return err
	}
	if task.Revision != expectedRevision {
		return taskRevisionConflict(task.ID, expectedRevision, task.Revision)
	}
	tasks = append(tasks[:index], tasks[index+1:]...)
	return s.saveTasksUnlocked(tasks)
}

func findTask(tasks []Task, id string) (Task, error) {
	_, task, err := findTaskIndex(tasks, id)
	return task, err
}

func findTaskIndex(tasks []Task, id string) (int, Task, error) {
	id = strings.TrimSpace(id)
	for index, task := range tasks {
		if task.ID == id {
			return index, task, nil
		}
	}
	return -1, Task{}, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
}

func taskRevisionConflict(id string, expected, actual int64) error {
	return fmt.Errorf("%w: task %s expected %d, actual %d", ErrTaskRevisionConflict, id, expected, actual)
}

func cloneRawFields(fields map[string]json.RawMessage) map[string]json.RawMessage {
	if len(fields) == 0 {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(fields))
	for name, value := range fields {
		cloned[name] = append(json.RawMessage(nil), value...)
	}
	return cloned
}

// LoadTaskTemplates reads user-created templates, returning an empty list when none exist.
func (s *Store) LoadTaskTemplates() ([]TaskTemplate, error) {
	db, release, err := s.templatesDB()
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := db.Query(`
SELECT id, title, description, prompt, interval, icon, kind, source
FROM task_templates
WHERE deleted = 0 AND source = ?
ORDER BY created_at, id`, TemplateSourceUser)
	if err != nil {
		return nil, fmt.Errorf("load automation templates: query sqlite: %w", err)
	}
	defer rows.Close()

	var templates []TaskTemplate
	for rows.Next() {
		var template TaskTemplate
		if err := rows.Scan(&template.ID, &template.Title, &template.Description, &template.Prompt, &template.Interval, &template.Icon, &template.Kind, &template.Source); err != nil {
			return nil, fmt.Errorf("load automation templates: scan sqlite: %w", err)
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load automation templates: rows sqlite: %w", err)
	}
	return normalizeTaskTemplates(templates, TemplateSourceUser)
}

// SaveTaskTemplate upserts one user-created template.
func (s *Store) SaveTaskTemplate(template TaskTemplate) (TaskTemplate, error) {
	normalized, err := normalizeTaskTemplate(template, TemplateSourceUser)
	if err != nil {
		return TaskTemplate{}, err
	}
	db, release, err := s.templatesDB()
	if err != nil {
		return TaskTemplate{}, err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return TaskTemplate{}, fmt.Errorf("save automation template %q: begin sqlite: %w", normalized.ID, err)
	}
	defer tx.Rollback()
	if err := saveTaskTemplateRow(tx, normalized); err != nil {
		return TaskTemplate{}, err
	}
	if err := tx.Commit(); err != nil {
		return TaskTemplate{}, fmt.Errorf("save automation template %q: commit sqlite: %w", normalized.ID, err)
	}
	return normalized, nil
}

// DeleteTaskTemplate soft-deletes one user-created or built-in template.
func (s *Store) DeleteTaskTemplate(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidTask
	}
	db, release, err := s.templatesDB()
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("delete automation template %q: begin sqlite: %w", id, err)
	}
	defer tx.Rollback()

	var source string
	err = tx.QueryRow(`SELECT source FROM task_templates WHERE id = ? AND deleted = 0`, id).Scan(&source)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("delete automation template %q: lookup sqlite: %w", id, err)
	}
	if err == nil && source == TemplateSourceUser {
		if _, err := tx.Exec(`UPDATE task_templates SET deleted = 1, updated_at = ? WHERE id = ?`, time.Now().UnixMilli(), id); err != nil {
			return fmt.Errorf("delete automation template %q: sqlite: %w", id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("delete automation template %q: commit sqlite: %w", id, err)
		}
		return nil
	}
	if isDefaultTaskTemplateID(id) {
		now := time.Now().UnixMilli()
		if _, err := tx.Exec(`
INSERT INTO task_templates (
	id, title, description, prompt, interval, icon, kind, source, deleted, created_at, updated_at
) VALUES (?, '', '', '', '', '', ?, ?, 1, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	source = excluded.source,
	deleted = 1,
	updated_at = excluded.updated_at`, id, TemplateKindTask, TemplateSourceBuiltin, now, now); err != nil {
			return fmt.Errorf("delete automation built-in template %q: sqlite: %w", id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("delete automation built-in template %q: commit sqlite: %w", id, err)
		}
		return nil
	}
	return ErrTaskNotFound
}

// RestoreTaskTemplate restores one soft-deleted template, or all deleted templates when id is empty.
func (s *Store) RestoreTaskTemplate(id string) error {
	id = strings.TrimSpace(id)
	db, release, err := s.templatesDB()
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("restore automation template %q: begin sqlite: %w", id, err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	if id == "" {
		if _, err := tx.Exec(`UPDATE task_templates SET deleted = 0, updated_at = ? WHERE deleted <> 0`, now); err != nil {
			return fmt.Errorf("restore automation templates: sqlite: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("restore automation templates: commit sqlite: %w", err)
		}
		return nil
	}
	if isDefaultTaskTemplateID(id) {
		if _, err := tx.Exec(`UPDATE task_templates SET deleted = 0, updated_at = ? WHERE id = ?`, now, id); err != nil {
			return fmt.Errorf("restore automation built-in template %q: sqlite: %w", id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("restore automation built-in template %q: commit sqlite: %w", id, err)
		}
		return nil
	}
	res, err := tx.Exec(`UPDATE task_templates SET deleted = 0, updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return fmt.Errorf("restore automation template %q: sqlite: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("restore automation template %q: affected rows sqlite: %w", id, err)
	}
	if affected == 0 {
		return ErrTaskNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("restore automation template %q: commit sqlite: %w", id, err)
	}
	return nil
}

func (s *Store) DeletedTaskTemplateIDs() (map[string]bool, error) {
	db, release, err := s.templatesDB()
	if err != nil {
		return nil, err
	}
	defer release()
	rows, err := db.Query(`SELECT id FROM task_templates WHERE deleted <> 0`)
	if err != nil {
		return nil, fmt.Errorf("load deleted automation templates: query sqlite: %w", err)
	}
	defer rows.Close()
	deleted := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("load deleted automation templates: scan sqlite: %w", err)
		}
		deleted[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load deleted automation templates: rows sqlite: %w", err)
	}
	return deleted, nil
}

func (s *Store) tasksPath() string {
	return filepath.Join(s.dir, tasksFileName)
}

func (s *Store) templatesPath() string {
	return filepath.Join(s.dir, templatesFileName)
}

func (s *Store) templatesDBPath() string {
	return filepath.Join(s.dir, templatesDBFileName)
}

func normalizeTasks(tasks []Task) ([]Task, error) {
	out := make([]Task, 0, len(tasks))
	seen := map[string]bool{}
	for i, task := range tasks {
		normalized, err := NormalizeTask(task)
		if err != nil {
			return nil, fmt.Errorf("task %d: %w", i, err)
		}
		if seen[normalized.ID] {
			return nil, fmt.Errorf("task %d: %w %q", i, ErrDuplicateTaskID, normalized.ID)
		}
		seen[normalized.ID] = true
		out = append(out, normalized)
	}
	return out, nil
}

func (s *Store) templatesDB() (*sql.DB, func(), error) {
	if !s.persistent {
		db, err := s.openTemplatesDB()
		if err != nil {
			return nil, func() {}, err
		}
		if err := s.ensureTemplatesDB(db); err != nil {
			db.Close()
			return nil, func() {}, err
		}
		return db, func() { _ = db.Close() }, nil
	}

	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db != nil {
		return s.db, func() {}, nil
	}
	db, err := s.openTemplatesDB()
	if err != nil {
		return nil, func() {}, err
	}
	if err := s.ensureTemplatesDB(db); err != nil {
		db.Close()
		return nil, func() {}, err
	}
	s.db = db
	return db, func() {}, nil
}

func (s *Store) openTemplatesDB() (*sql.DB, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create automation template store dir: %w", err)
	}
	db, err := sql.Open("sqlite", s.templatesDBPath())
	if err != nil {
		return nil, fmt.Errorf("open automation template sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure automation template sqlite: %w", err)
	}
	return db, nil
}

func (s *Store) ensureTemplatesDB(db *sql.DB) error {
	if _, err := db.Exec(taskTemplatesSchemaSQL); err != nil {
		return fmt.Errorf("initialize automation templates sqlite: %w", err)
	}
	if _, err := db.Exec(taskTemplateMigrationsSchemaSQL); err != nil {
		return fmt.Errorf("initialize automation template migrations sqlite: %w", err)
	}
	return s.migrateLegacyTaskTemplates(db)
}

func (s *Store) migrateLegacyTaskTemplates(db *sql.DB) error {
	var done int
	if err := db.QueryRow(`SELECT count(*) FROM task_template_migrations WHERE name = ?`, templatesMigrationID).Scan(&done); err != nil {
		return fmt.Errorf("migrate legacy automation templates: check marker: %w", err)
	}
	if done > 0 {
		return nil
	}
	b, err := os.ReadFile(s.templatesPath())
	if errors.Is(err, os.ErrNotExist) {
		_, err := db.Exec(`INSERT OR IGNORE INTO task_template_migrations (name, migrated_at) VALUES (?, ?)`, templatesMigrationID, time.Now().UnixMilli())
		if err != nil {
			return fmt.Errorf("migrate legacy automation templates: mark complete: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate legacy automation templates: read %s: %w", s.templatesPath(), err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		_, err := db.Exec(`INSERT OR IGNORE INTO task_template_migrations (name, migrated_at) VALUES (?, ?)`, templatesMigrationID, time.Now().UnixMilli())
		if err != nil {
			return fmt.Errorf("migrate legacy automation templates: mark complete: %w", err)
		}
		return nil
	}
	var file templatesFile
	if err := json.Unmarshal(b, &file); err != nil {
		return fmt.Errorf("migrate legacy automation templates: decode %s: %w", s.templatesPath(), err)
	}
	templates, err := normalizeTaskTemplates(file.Templates, TemplateSourceUser)
	if err != nil {
		return fmt.Errorf("migrate legacy automation templates: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate legacy automation templates: begin sqlite: %w", err)
	}
	defer tx.Rollback()
	for _, template := range templates {
		if err := saveTaskTemplateRow(tx, template); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO task_template_migrations (name, migrated_at) VALUES (?, ?)`, templatesMigrationID, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("migrate legacy automation templates: mark complete: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate legacy automation templates: commit sqlite: %w", err)
	}
	return nil
}

func saveTaskTemplateRow(tx *sql.Tx, template TaskTemplate) error {
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(`
INSERT INTO task_templates (
	id, title, description, prompt, interval, icon, kind, source, deleted, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	title = excluded.title,
	description = excluded.description,
	prompt = excluded.prompt,
	interval = excluded.interval,
	icon = excluded.icon,
	kind = excluded.kind,
	source = excluded.source,
	deleted = excluded.deleted,
	updated_at = excluded.updated_at`,
		template.ID, template.Title, template.Description, template.Prompt, template.Interval, template.Icon, template.Kind, template.Source, 0, now, now,
	); err != nil {
		return fmt.Errorf("save automation template %q: sqlite: %w", template.ID, err)
	}
	return nil
}

func normalizeTaskTemplates(templates []TaskTemplate, source string) ([]TaskTemplate, error) {
	out := make([]TaskTemplate, 0, len(templates))
	seen := map[string]bool{}
	for i, template := range templates {
		normalized, err := normalizeTaskTemplate(template, source)
		if err != nil {
			return nil, fmt.Errorf("template %d: %w", i, err)
		}
		if seen[normalized.ID] {
			return nil, fmt.Errorf("template %d: %w %q", i, ErrDuplicateTaskID, normalized.ID)
		}
		seen[normalized.ID] = true
		out = append(out, normalized)
	}
	return out, nil
}

func isDefaultTaskTemplateID(id string) bool {
	for _, template := range DefaultTaskTemplates() {
		if template.ID == id {
			return true
		}
	}
	return false
}

const taskTemplatesSchemaSQL = `
CREATE TABLE IF NOT EXISTS task_templates (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	prompt TEXT NOT NULL,
	interval TEXT NOT NULL,
	icon TEXT NOT NULL,
	kind TEXT NOT NULL,
	source TEXT NOT NULL,
	deleted INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
)`

const taskTemplateMigrationsSchemaSQL = `
CREATE TABLE IF NOT EXISTS task_template_migrations (
	name TEXT PRIMARY KEY,
	migrated_at INTEGER NOT NULL
)`
