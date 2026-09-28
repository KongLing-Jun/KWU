package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kwu/internal/provider"

	_ "modernc.org/sqlite"
)

const (
	sessionCatalogFileName       = "session-catalog.sqlite"
	sessionCatalogDirName        = ".session-index"
	sessionCatalogVersion        = 1
	sessionCatalogBootstrapLimit = 128
	sessionCatalogRepairTimeout  = 5 * time.Minute
)

type sessionCatalogRow struct {
	SessionInfo
	SchemaVersion int
	FileSize      int64
	FileMTimeNS   int64
}

var (
	sessionCatalogLocks   sync.Map
	sessionCatalogRepairs sync.Map
	sessionCatalogDirty   sync.Map
	sessionCatalogChecks  sync.Map
	sessionCatalogStale   sync.Map
)

type sessionCatalogRepair struct {
	done chan struct{}
	err  error
}

func sessionCatalogEnabled() bool {
	return strings.TrimSpace(os.Getenv("KWU_DISABLE_SESSION_CATALOG")) != "1"
}

func sessionCatalogPath(dir string) string {
	return filepath.Join(filepath.Clean(dir), sessionCatalogDirName, sessionCatalogFileName)
}

func sessionCatalogLock(dir string) *sync.Mutex {
	value, _ := sessionCatalogLocks.LoadOrStore(filepath.Clean(dir), &sync.Mutex{})
	return value.(*sync.Mutex)
}

func loadSessionCatalogRows(ctx context.Context, dir string) ([]sessionCatalogRow, bool, error) {
	if !sessionCatalogEnabled() {
		rows, err := scanSessionCatalogRowsContext(ctx, dir)
		return rows, false, err
	}
	exists, err := sessionDirectoryExists(dir)
	if err != nil || !exists {
		return []sessionCatalogRow{}, false, err
	}
	catalogExists := pathExists(sessionCatalogPath(dir))
	rows, ready, readErr := readSessionCatalog(dir)
	if readErr == nil && rows != nil {
		return finishSessionCatalogRead(ctx, dir, rows, ready)
	}
	return bootstrapMissingSessionCatalog(ctx, dir, catalogExists)
}

func sessionDirectoryExists(dir string) (bool, error) {
	_, err := os.Stat(dir)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func finishSessionCatalogRead(ctx context.Context, dir string, rows []sessionCatalogRow, ready bool) ([]sessionCatalogRow, bool, error) {
	pending := !ready || sessionCatalogNeedsRepair(dir)
	if !pending {
		return rows, false, nil
	}
	startSessionCatalogRepair(ctx, dir)
	if !sessionCatalogDirectoryIsSmall(dir, sessionCatalogBootstrapLimit) {
		return rows, true, nil
	}
	if err := WaitSessionCatalogRepair(ctx, dir); err != nil {
		return nil, false, err
	}
	repaired, repairedReady, err := readSessionCatalog(dir)
	if err != nil {
		return nil, false, err
	}
	if repairedReady {
		return repaired, false, nil
	}
	return rows, true, nil
}

func bootstrapMissingSessionCatalog(ctx context.Context, dir string, catalogExists bool) ([]sessionCatalogRow, bool, error) {
	// Keep first paint bounded for legacy or corrupt catalogs. The partial rows
	// are metadata-only placeholders; an app-lifetime, cancellable repair fills
	// exact previews/counts off the request path.
	rows, exhausted, err := bootstrapSessionCatalogRows(ctx, dir, sessionCatalogBootstrapLimit)
	if err != nil {
		return nil, false, err
	}
	if exhausted && !catalogExists {
		rows, err = scanSessionCatalogRowsContext(ctx, dir)
		if err != nil {
			return nil, false, err
		}
		if err := rebuildSessionCatalog(dir, rows, true); err != nil {
			return rows, false, nil
		}
		backfillSessionCatalogSidecars(rows)
		return rows, false, nil
	}
	startSessionCatalogRepair(ctx, dir)
	return rows, true, nil
}

func scanSessionCatalogRows(dir string) ([]sessionCatalogRow, error) {
	return scanSessionCatalogRowsContext(context.Background(), dir)
}

func scanSessionCatalogRowsContext(ctx context.Context, dir string) ([]sessionCatalogRow, error) {
	ordered, err := scanSessionOrder(dir)
	if err != nil {
		return nil, err
	}
	rows := make([]sessionCatalogRow, 0, len(ordered))
	for _, session := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		preview, turns := session.Preview, session.Turns
		schemaVersion := session.SchemaVersion
		_, forceDecode := sessionCatalogStale.LoadAndDelete(sessionCatalogStaleKey(dir, session.Path))
		if schemaVersion < BranchMetaCountsVersion || forceDecode {
			preview, turns = previewSession(session.Path)
			schemaVersion = BranchMetaCountsVersion
		}
		expertID, expertName := "", ""
		if meta, ok, err := LoadSessionExpert(session.Path); err == nil && ok {
			expertID, expertName = meta.ExpertID, meta.ExpertName
		}
		rows = append(rows, sessionCatalogRow{
			SessionInfo: SessionInfo{
				Path: session.Path, CreatedAt: session.CreatedAt,
				LastActivityAt: session.LastActivityAt, ModTime: session.ModTime,
				Preview: preview, Turns: turns, ExpertID: expertID, ExpertName: expertName,
				Scope: session.Scope, WorkspaceRoot: session.WorkspaceRoot,
				TopicID: session.TopicID, TopicTitle: session.TopicTitle,
			},
			SchemaVersion: schemaVersion,
		})
	}
	return rows, nil
}

func bootstrapSessionCatalogRows(ctx context.Context, dir string, limit int) ([]sessionCatalogRow, bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []sessionCatalogRow{}, true, nil
		}
		return nil, false, err
	}
	defer f.Close()
	entries, exhausted, err := readBoundedDirectoryEntries(f, limit)
	if err != nil {
		return nil, false, err
	}
	rows := make([]sessionCatalogRow, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if row, ok := bootstrapSessionCatalogRow(dir, entry); ok {
			rows = append(rows, row)
		}
	}
	sortSessionCatalogRows(rows)
	return rows, exhausted, nil
}

func readBoundedDirectoryEntries(dir *os.File, limit int) ([]os.DirEntry, bool, error) {
	entries, err := dir.ReadDir(limit)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if errors.Is(err, io.EOF) {
		return entries, true, nil
	}
	more, nextErr := dir.ReadDir(1)
	return entries, len(more) == 0 && errors.Is(nextErr, io.EOF), nil
}

func bootstrapSessionCatalogRow(dir string, entry os.DirEntry) (sessionCatalogRow, bool) {
	if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
		return sessionCatalogRow{}, false
	}
	path := filepath.Join(dir, entry.Name())
	if !IsVisibleSession(path) {
		return sessionCatalogRow{}, false
	}
	info, err := entry.Info()
	if err != nil || info.Size() == 0 {
		return sessionCatalogRow{}, false
	}
	return sessionCatalogRow{SessionInfo: SessionInfo{
		Path: path, CreatedAt: info.ModTime(), LastActivityAt: info.ModTime(),
		ModTime: info.ModTime(), Turns: 1, Scope: "global",
	}}, true
}

func sessionCatalogDirectoryIsSmall(dir string, limit int) bool {
	f, err := os.Open(dir)
	if err != nil {
		return true
	}
	defer f.Close()
	entries, exhausted, err := readBoundedDirectoryEntries(f, limit+1)
	if err != nil {
		return false
	}
	if len(entries) > limit {
		return false
	}
	return exhausted
}

func sortSessionCatalogRows(rows []sessionCatalogRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].LastActivityAt.Equal(rows[j].LastActivityAt) {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].LastActivityAt.After(rows[j].LastActivityAt)
	})
}

func readSessionCatalog(dir string) ([]sessionCatalogRow, bool, error) {
	if _, err := os.Stat(sessionCatalogPath(dir)); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	db, err := openSessionCatalog(dir)
	if err != nil {
		return nil, false, err
	}
	defer db.Close()

	version, complete, recordedDirMTime, err := readSessionCatalogState(db)
	if err != nil {
		return nil, false, err
	}
	if version != sessionCatalogVersion {
		return nil, false, nil
	}
	out, err := querySessionCatalogRows(db, dir)
	if err != nil {
		return nil, false, err
	}
	return out, sessionCatalogIsReady(dir, out, complete, recordedDirMTime), nil
}

func sessionCatalogIsReady(dir string, rows []sessionCatalogRow, complete int, recordedDirMTime int64) bool {
	if complete != 1 || recordedDirMTime == 0 || recordedDirMTime != sessionCatalogDirectoryMTime(dir) {
		return false
	}
	return validateOneSessionCatalogRow(dir, rows)
}

func readSessionCatalogState(db *sql.DB) (version, complete int, recordedDirMTime int64, err error) {
	if err = db.QueryRow(`SELECT value FROM session_catalog_state WHERE key = 'schema_version'`).Scan(&version); err != nil {
		return
	}
	if err = db.QueryRow(`SELECT value FROM session_catalog_state WHERE key = 'backfill_complete'`).Scan(&complete); err != nil {
		return
	}
	_ = db.QueryRow(`SELECT value FROM session_catalog_state WHERE key = 'directory_mtime_ns'`).Scan(&recordedDirMTime)
	return
}

func querySessionCatalogRows(db *sql.DB, dir string) ([]sessionCatalogRow, error) {
	rows, err := db.Query(`
SELECT relative_path, created_at_ns, updated_at_ns, preview, turn_count,
       expert_id, expert_name, scope, workspace_root, topic_id, topic_title,
       metadata_version, file_size, file_mtime_ns
FROM session_catalog
WHERE cleanup_pending = 0
ORDER BY updated_at_ns DESC, session_key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]sessionCatalogRow, 0)
	for rows.Next() {
		row, err := scanSessionCatalogRow(rows, dir)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanSessionCatalogRow(rows *sql.Rows, dir string) (sessionCatalogRow, error) {
	var relative string
	var createdNS, updatedNS int64
	var row sessionCatalogRow
	if err := rows.Scan(
		&relative, &createdNS, &updatedNS, &row.Preview, &row.Turns,
		&row.ExpertID, &row.ExpertName, &row.Scope, &row.WorkspaceRoot,
		&row.TopicID, &row.TopicTitle, &row.SchemaVersion, &row.FileSize, &row.FileMTimeNS,
	); err != nil {
		return sessionCatalogRow{}, err
	}
	if filepath.Base(relative) != relative || filepath.Ext(relative) != ".jsonl" {
		return sessionCatalogRow{}, fmt.Errorf("invalid session catalog path %q", relative)
	}
	row.Path = filepath.Join(dir, relative)
	row.CreatedAt = time.Unix(0, createdNS).UTC()
	row.LastActivityAt = time.Unix(0, updatedNS).UTC()
	row.ModTime = row.LastActivityAt
	return row, nil
}

func validateOneSessionCatalogRow(dir string, rows []sessionCatalogRow) bool {
	if len(rows) == 0 {
		return true
	}
	value, _ := sessionCatalogChecks.LoadOrStore(filepath.Clean(dir), &atomic.Uint64{})
	index := int(value.(*atomic.Uint64).Add(1)-1) % len(rows)
	info, err := os.Stat(rows[index].Path)
	valid := err == nil && info.Size() == rows[index].FileSize && info.ModTime().UnixNano() == rows[index].FileMTimeNS
	if !valid {
		sessionCatalogStale.Store(sessionCatalogStaleKey(dir, rows[index].Path), struct{}{})
	}
	return valid
}

func sessionCatalogStaleKey(dir, path string) string {
	return filepath.Clean(dir) + "\x00" + filepath.Clean(path)
}

func rebuildSessionCatalog(dir string, rows []sessionCatalogRow, complete bool) error {
	lock := sessionCatalogLock(dir)
	lock.Lock()
	defer lock.Unlock()
	return rebuildSessionCatalogLocked(dir, rows, complete)
}

func rebuildSessionCatalogLocked(dir string, rows []sessionCatalogRow, complete bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	db, err := openRepairableSessionCatalog(dir)
	if err != nil {
		return err
	}
	defer db.Close()
	return writeSessionCatalog(db, dir, rows, complete)
}

func openRepairableSessionCatalog(dir string) (*sql.DB, error) {
	db, err := openSessionCatalog(dir)
	if err != nil {
		return nil, err
	}
	if err := ensureSessionCatalog(db); err == nil {
		return db, nil
	} else if !isSessionCatalogCorruption(err) {
		db.Close()
		return nil, err
	}
	db.Close()
	corruptPath := fmt.Sprintf("%s.corrupt-%d", sessionCatalogPath(dir), time.Now().UTC().UnixNano())
	if err := os.Rename(sessionCatalogPath(dir), corruptPath); err != nil {
		return nil, err
	}
	db, err = openSessionCatalog(dir)
	if err != nil {
		return nil, err
	}
	if err := ensureSessionCatalog(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func writeSessionCatalog(db *sql.DB, dir string, rows []sessionCatalogRow, complete bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM session_catalog`); err != nil {
		return err
	}
	for _, row := range rows {
		if err := upsertSessionCatalogRow(tx, row); err != nil {
			return err
		}
	}
	completeValue := 0
	if complete {
		completeValue = 1
	}
	dirMTime := sessionCatalogDirectoryMTime(dir)
	if _, err := tx.Exec(`
INSERT INTO session_catalog_state (key, value) VALUES
  ('schema_version', ?), ('backfill_complete', ?), ('last_reconciled_ns', ?),
  ('directory_mtime_ns', ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		sessionCatalogVersion, completeValue, time.Now().UTC().UnixNano(), dirMTime); err != nil {
		return err
	}
	return tx.Commit()
}

func sessionCatalogDirectoryMTime(dir string) int64 {
	if info, err := os.Stat(dir); err == nil {
		return info.ModTime().UnixNano()
	}
	return 0
}

func sessionCatalogNeedsRepair(dir string) bool {
	_, dirty := sessionCatalogDirty.Load(filepath.Clean(dir))
	return dirty
}

func startSessionCatalogRepair(parent context.Context, dir string) *sessionCatalogRepair {
	key := filepath.Clean(dir)
	if existing, ok := sessionCatalogRepairs.Load(key); ok {
		return existing.(*sessionCatalogRepair)
	}
	state := &sessionCatalogRepair{done: make(chan struct{})}
	actual, loaded := sessionCatalogRepairs.LoadOrStore(key, state)
	if loaded {
		return actual.(*sessionCatalogRepair)
	}
	go func() {
		defer close(state.done)
		defer sessionCatalogRepairs.Delete(key)
		ctx, cancel := context.WithTimeout(parent, sessionCatalogRepairTimeout)
		defer cancel()
		lock := sessionCatalogLock(key)
		for {
			if err := ctx.Err(); err != nil {
				state.err = err
				return
			}
			lock.Lock()
			sessionCatalogDirty.Delete(key)
			rows, err := scanSessionCatalogRowsContext(ctx, key)
			if err == nil {
				err = rebuildSessionCatalogLocked(key, rows, true)
			}
			lock.Unlock()
			if err != nil {
				sessionCatalogDirty.Store(key, struct{}{})
				state.err = err
				return
			}
			backfillSessionCatalogSidecars(rows)
			if _, dirty := sessionCatalogDirty.Load(key); !dirty {
				return
			}
		}
	}()
	return state
}

func backfillSessionCatalogSidecars(rows []sessionCatalogRow) {
	for _, row := range rows {
		meta, ok, err := LoadBranchMeta(row.Path)
		if err == nil && (!ok || meta.SchemaVersion < BranchMetaCountsVersion ||
			meta.Preview != row.Preview || meta.Turns != row.Turns) {
			_ = UpdateSessionMeta(row.Path, "", row.Preview, row.Turns, false)
		}
	}
}

// WaitSessionCatalogRepair waits for an in-flight derived-catalog repair. It
// returns immediately when no repair is active.
func WaitSessionCatalogRepair(ctx context.Context, dir string) error {
	value, ok := sessionCatalogRepairs.Load(filepath.Clean(dir))
	if !ok {
		return nil
	}
	state := value.(*sessionCatalogRepair)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-state.done:
		return state.err
	}
}

func isSessionCatalogCorruption(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "malformed") ||
		strings.Contains(message, "not a database") ||
		strings.Contains(message, "file is encrypted")
}

func refreshSessionCatalogEntry(path string, messages []provider.Message) {
	dir := filepath.Dir(path)
	if !sessionCatalogEnabled() {
		return
	}
	if _, err := os.Stat(sessionCatalogPath(dir)); err != nil {
		return
	}
	row, ok := sessionCatalogRowForPath(path, messages)
	if !ok {
		return
	}
	lock := sessionCatalogLock(dir)
	if !lock.TryLock() {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
		return
	}
	defer lock.Unlock()
	db, err := openSessionCatalog(dir)
	if err != nil {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
		return
	}
	defer db.Close()
	if err := ensureSessionCatalog(db); err != nil {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
		return
	}
	if err := upsertSessionCatalogRow(db, row); err != nil {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
		return
	}
	if _, err := db.Exec(`
INSERT INTO session_catalog_state (key, value) VALUES ('directory_mtime_ns', ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, sessionCatalogDirectoryMTime(dir)); err != nil {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
	}
}

func removeSessionCatalogEntry(path string) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(sessionCatalogPath(dir)); err != nil {
		return
	}
	lock := sessionCatalogLock(dir)
	if !lock.TryLock() {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
		return
	}
	defer lock.Unlock()
	db, err := openSessionCatalog(dir)
	if err != nil {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
		return
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM session_catalog WHERE session_key = ?`, BranchID(path)); err != nil {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
		return
	}
	if _, err := db.Exec(`
INSERT INTO session_catalog_state (key, value) VALUES ('directory_mtime_ns', ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, sessionCatalogDirectoryMTime(dir)); err != nil {
		sessionCatalogDirty.Store(filepath.Clean(dir), struct{}{})
	}
}

func sessionCatalogRowForPath(path string, messages []provider.Message) (sessionCatalogRow, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || filepath.Ext(path) != ".jsonl" {
		return sessionCatalogRow{}, false
	}
	row := sessionCatalogRow{SessionInfo: SessionInfo{
		Path: path, CreatedAt: info.ModTime(), LastActivityAt: info.ModTime(),
		ModTime: info.ModTime(), Scope: "global",
	}}
	if meta, ok, err := LoadBranchMeta(path); err == nil && ok {
		applyBranchMetaToSessionCatalogRow(&row, meta)
	}
	if messages != nil {
		row.Preview, row.Turns = SessionPreviewFromMessages(messages)
		row.SchemaVersion = BranchMetaCountsVersion
	} else if row.SchemaVersion < BranchMetaCountsVersion {
		backfillSessionCatalogRowContent(&row)
	}
	if meta, ok, err := LoadSessionExpert(path); err == nil && ok {
		row.ExpertID, row.ExpertName = meta.ExpertID, meta.ExpertName
	}
	return row, true
}

func applyBranchMetaToSessionCatalogRow(row *sessionCatalogRow, meta BranchMeta) {
	if !meta.CreatedAt.IsZero() {
		row.CreatedAt = meta.CreatedAt
	}
	if !meta.UpdatedAt.IsZero() {
		row.LastActivityAt = meta.UpdatedAt
		row.ModTime = meta.UpdatedAt
	}
	row.Scope = meta.DefaultScope()
	row.WorkspaceRoot = meta.WorkspaceRoot
	row.TopicID = meta.TopicID
	row.TopicTitle = meta.TopicTitle
	row.Preview = meta.Preview
	row.Turns = meta.Turns
	row.SchemaVersion = meta.SchemaVersion
}

func backfillSessionCatalogRowContent(row *sessionCatalogRow) {
	row.Preview, row.Turns = previewSession(row.Path)
	_ = UpdateSessionMeta(row.Path, "", row.Preview, row.Turns, false)
	row.SchemaVersion = BranchMetaCountsVersion
}

type sessionCatalogExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func upsertSessionCatalogRow(exec sessionCatalogExecer, row sessionCatalogRow) error {
	relative := filepath.Base(row.Path)
	if relative == "." || relative == "" || filepath.Ext(relative) != ".jsonl" {
		return fmt.Errorf("invalid session path %q", row.Path)
	}
	fileSize, fileMTime := int64(0), int64(0)
	if info, err := os.Stat(row.Path); err == nil {
		fileSize, fileMTime = info.Size(), info.ModTime().UnixNano()
	}
	_, err := exec.Exec(`
INSERT INTO session_catalog (
  session_key, relative_path, created_at_ns, updated_at_ns, preview, turn_count,
  expert_id, expert_name, scope, workspace_root, topic_id, topic_title,
  cleanup_pending, file_size, file_mtime_ns, indexed_at_ns, metadata_version
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)
ON CONFLICT(session_key) DO UPDATE SET
  relative_path = excluded.relative_path,
  created_at_ns = excluded.created_at_ns,
  updated_at_ns = excluded.updated_at_ns,
  preview = excluded.preview,
  turn_count = excluded.turn_count,
  expert_id = excluded.expert_id,
  expert_name = excluded.expert_name,
  scope = excluded.scope,
  workspace_root = excluded.workspace_root,
  topic_id = excluded.topic_id,
  topic_title = excluded.topic_title,
  cleanup_pending = 0,
  file_size = excluded.file_size,
  file_mtime_ns = excluded.file_mtime_ns,
  indexed_at_ns = excluded.indexed_at_ns,
  metadata_version = excluded.metadata_version`,
		BranchID(row.Path), relative, row.CreatedAt.UnixNano(), row.LastActivityAt.UnixNano(),
		row.Preview, row.Turns, row.ExpertID, row.ExpertName, row.Scope,
		row.WorkspaceRoot, row.TopicID, row.TopicTitle, fileSize, fileMTime,
		time.Now().UTC().UnixNano(), row.SchemaVersion,
	)
	return err
}

func openSessionCatalog(dir string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(sessionCatalogPath(dir)), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sessionCatalogPath(dir))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureSessionCatalog(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS session_catalog_state (
  key TEXT PRIMARY KEY,
  value INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS session_catalog (
  session_key TEXT PRIMARY KEY,
  relative_path TEXT NOT NULL,
  created_at_ns INTEGER NOT NULL,
  updated_at_ns INTEGER NOT NULL,
  preview TEXT NOT NULL DEFAULT '',
  turn_count INTEGER NOT NULL DEFAULT 0 CHECK (turn_count >= 0),
  expert_id TEXT NOT NULL DEFAULT '',
  expert_name TEXT NOT NULL DEFAULT '',
  scope TEXT NOT NULL DEFAULT 'global',
  workspace_root TEXT NOT NULL DEFAULT '',
  topic_id TEXT NOT NULL DEFAULT '',
  topic_title TEXT NOT NULL DEFAULT '',
  cleanup_pending INTEGER NOT NULL DEFAULT 0 CHECK (cleanup_pending IN (0, 1)),
  file_size INTEGER NOT NULL DEFAULT 0,
  file_mtime_ns INTEGER NOT NULL DEFAULT 0,
  indexed_at_ns INTEGER NOT NULL,
  metadata_version INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_session_catalog_updated
  ON session_catalog(updated_at_ns DESC, session_key ASC);
CREATE INDEX IF NOT EXISTS idx_session_catalog_scope_updated
  ON session_catalog(scope, workspace_root, updated_at_ns DESC, session_key ASC);
`)
	return err
}
