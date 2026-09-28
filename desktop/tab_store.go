package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	tabsDatabaseName              = "desktop-tabs.sqlite"
	legacyTabsFileName            = "desktop-tabs.json"
	toolApprovalModePreferenceKey = "default_tool_approval_mode"
)

type desktopTabStore struct {
	dir string
}

type desktopTabActivation struct {
	PreviousID string
	ActiveID   string
	Version    uint64
}

func newDesktopTabStore(dir string) desktopTabStore {
	return desktopTabStore{dir: strings.TrimSpace(dir)}
}

func (s desktopTabStore) path() string {
	return filepath.Join(s.dir, tabsDatabaseName)
}

func (s desktopTabStore) open() (*sql.DB, error) {
	if s.dir == "" {
		return nil, fmt.Errorf("desktop tab store directory is empty")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create desktop tab store directory: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(s.path()) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open desktop tab store: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := ensureDesktopTabSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ensureDesktopTabSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS desktop_tab_state (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			active_tab_id TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT OR IGNORE INTO desktop_tab_state(singleton, active_tab_id, version) VALUES (1, '', 0)`,
		`CREATE TABLE IF NOT EXISTS desktop_tabs (
			id TEXT PRIMARY KEY,
			position INTEGER NOT NULL UNIQUE,
			scope TEXT NOT NULL,
			workspace_root TEXT NOT NULL,
			topic_id TEXT NOT NULL,
			session_path TEXT NOT NULL,
			scene TEXT NOT NULL,
			scene_state TEXT NOT NULL,
			read_only INTEGER NOT NULL,
			model TEXT NOT NULL,
			billing_source TEXT NOT NULL,
			effort TEXT,
			token_mode TEXT NOT NULL,
			mode TEXT NOT NULL,
			goal TEXT NOT NULL,
			tool_approval_mode TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_preferences (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("initialize desktop tab store: %w", err)
		}
	}
	return nil
}

func (s desktopTabStore) defaultToolApprovalMode() (string, bool, error) {
	db, err := s.open()
	if err != nil {
		return "", false, err
	}
	defer db.Close()

	var mode string
	err = db.QueryRow(
		`SELECT value FROM desktop_preferences WHERE key = ?`,
		toolApprovalModePreferenceKey,
	).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load desktop default tool approval mode: %w", err)
	}
	return normalizeToolApprovalMode(mode), true, nil
}

func (s desktopTabStore) setDefaultToolApprovalMode(mode string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(
		`INSERT INTO desktop_preferences(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		toolApprovalModePreferenceKey,
		normalizeToolApprovalMode(mode),
	); err != nil {
		return fmt.Errorf("save desktop default tool approval mode: %w", err)
	}
	return nil
}

func (s desktopTabStore) load() (desktopTabsFile, error) {
	db, err := s.open()
	if err != nil {
		return desktopTabsFile{}, err
	}
	defer db.Close()

	var state desktopTabsFile
	if err := db.QueryRow(`SELECT active_tab_id, version FROM desktop_tab_state WHERE singleton = 1`).Scan(&state.ActiveTab, &state.Version); err != nil {
		return desktopTabsFile{}, fmt.Errorf("load desktop tab state: %w", err)
	}
	rows, err := db.Query(`SELECT id, scope, workspace_root, topic_id, session_path,
		scene, scene_state, read_only, model, billing_source, effort, token_mode,
		mode, goal, tool_approval_mode
		FROM desktop_tabs ORDER BY position`)
	if err != nil {
		return desktopTabsFile{}, fmt.Errorf("load desktop tabRuntimes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entry desktopTabEntry
		var effort sql.NullString
		if err := rows.Scan(
			&entry.ID, &entry.Scope, &entry.WorkspaceRoot, &entry.TopicID,
			&entry.SessionPath, &entry.Scene, &entry.SceneState, &entry.ReadOnly,
			&entry.Model, &entry.BillingSource, &effort, &entry.TokenMode,
			&entry.Mode, &entry.Goal, &entry.ToolApprovalMode,
		); err != nil {
			return desktopTabsFile{}, fmt.Errorf("scan desktop tab: %w", err)
		}
		if effort.Valid {
			entry.Effort = &effort.String
		}
		state.Tabs = append(state.Tabs, entry)
	}
	if err := rows.Err(); err != nil {
		return desktopTabsFile{}, fmt.Errorf("iterate desktop tabRuntimes: %w", err)
	}
	return state, nil
}

func (s desktopTabStore) activeTabID() (string, uint64, error) {
	db, err := s.open()
	if err != nil {
		return "", 0, err
	}
	defer db.Close()
	var id string
	var version uint64
	if err := db.QueryRow(`SELECT active_tab_id, version FROM desktop_tab_state WHERE singleton = 1`).Scan(&id, &version); err != nil {
		return "", 0, fmt.Errorf("load active desktop tab: %w", err)
	}
	return id, version, nil
}

// activateTab atomically verifies the target and advances active_tab_id from
// the caller's observed value. The compare-and-swap boundary replaces the
// former App.mu critical section for activity switching: concurrent close,
// restore, or switch operations cannot silently overwrite one another.
func (s desktopTabStore) activateTab(expectedActiveID, targetID string) (desktopTabActivation, error) {
	expectedActiveID = strings.TrimSpace(expectedActiveID)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return desktopTabActivation{}, fmt.Errorf("desktop target tab id is empty")
	}
	db, err := s.open()
	if err != nil {
		return desktopTabActivation{}, err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return desktopTabActivation{}, fmt.Errorf("begin desktop tab activation: %w", err)
	}
	defer tx.Rollback()

	var currentID string
	var version uint64
	if err := tx.QueryRow(`SELECT active_tab_id, version FROM desktop_tab_state WHERE singleton = 1`).Scan(&currentID, &version); err != nil {
		return desktopTabActivation{}, fmt.Errorf("load desktop tab activation state: %w", err)
	}
	if currentID != expectedActiveID {
		return desktopTabActivation{}, fmt.Errorf("active tab changed during profile handoff: have %q, expected %q", currentID, expectedActiveID)
	}
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM desktop_tabs WHERE id = ?`, targetID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return desktopTabActivation{}, fmt.Errorf("tab %q not found", targetID)
		}
		return desktopTabActivation{}, fmt.Errorf("verify desktop target tab: %w", err)
	}
	nextVersion := version + 1
	result, err := tx.Exec(
		`UPDATE desktop_tab_state SET active_tab_id = ?, version = ? WHERE singleton = 1 AND active_tab_id = ? AND version = ?`,
		targetID, nextVersion, expectedActiveID, version,
	)
	if err != nil {
		return desktopTabActivation{}, fmt.Errorf("activate desktop tab: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return desktopTabActivation{}, fmt.Errorf("inspect desktop tab activation: %w", err)
	}
	if rows != 1 {
		return desktopTabActivation{}, fmt.Errorf("active tab changed during database commit")
	}
	if err := tx.Commit(); err != nil {
		return desktopTabActivation{}, fmt.Errorf("commit desktop tab activation: %w", err)
	}
	return desktopTabActivation{PreviousID: currentID, ActiveID: targetID, Version: nextVersion}, nil
}

func (s desktopTabStore) replace(state desktopTabsFile) (bool, error) {
	db, err := s.open()
	if err != nil {
		return false, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin desktop tab transaction: %w", err)
	}
	defer tx.Rollback()

	var currentVersion uint64
	if err := tx.QueryRow(`SELECT version FROM desktop_tab_state WHERE singleton = 1`).Scan(&currentVersion); err != nil {
		return false, fmt.Errorf("load desktop tab version: %w", err)
	}
	if state.Version == 0 {
		state.Version = currentVersion + 1
	}
	if state.Version <= currentVersion {
		return false, nil
	}
	if _, err := tx.Exec(`DELETE FROM desktop_tabs`); err != nil {
		return false, fmt.Errorf("clear desktop tabRuntimes: %w", err)
	}
	for position, entry := range state.Tabs {
		var effort any
		if entry.Effort != nil {
			effort = *entry.Effort
		}
		if _, err := tx.Exec(`INSERT INTO desktop_tabs(
			id, position, scope, workspace_root, topic_id, session_path, scene,
			scene_state, read_only, model, billing_source, effort, token_mode,
			mode, goal, tool_approval_mode
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entry.ID, position, entry.Scope, entry.WorkspaceRoot, entry.TopicID,
			entry.SessionPath, entry.Scene, entry.SceneState, entry.ReadOnly,
			entry.Model, entry.BillingSource, effort, entry.TokenMode, entry.Mode,
			entry.Goal, entry.ToolApprovalMode,
		); err != nil {
			return false, fmt.Errorf("store desktop tab %q: %w", entry.ID, err)
		}
	}
	if _, err := tx.Exec(`UPDATE desktop_tab_state SET active_tab_id = ?, version = ? WHERE singleton = 1`, state.ActiveTab, state.Version); err != nil {
		return false, fmt.Errorf("store active desktop tab: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit desktop tab transaction: %w", err)
	}
	return true, nil
}

func loadDesktopTabsFromDir(dir string) desktopTabsFile {
	store := newDesktopTabStore(dir)
	databaseExisted := false
	if _, err := os.Stat(store.path()); err == nil {
		databaseExisted = true
	}
	state, err := store.load()
	if err != nil {
		slog.Warn("load desktop tab database failed", "path", store.path(), "err", err)
		return desktopTabsFile{}
	}
	if databaseExisted || len(state.Tabs) > 0 || state.ActiveTab != "" {
		return state
	}

	legacyPath := filepath.Join(dir, legacyTabsFileName)
	body, err := os.ReadFile(legacyPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("read legacy desktop tabs failed", "path", legacyPath, "err", err)
		}
		return state
	}
	var legacy desktopTabsFile
	if err := json.Unmarshal(body, &legacy); err != nil {
		slog.Warn("decode legacy desktop tabs failed", "path", legacyPath, "err", err)
		return state
	}
	legacy.Version = state.Version + 1
	written, err := store.replace(legacy)
	if err != nil {
		slog.Warn("migrate legacy desktop tabs failed", "path", legacyPath, "err", err)
		return state
	}
	if written {
		if err := os.Remove(legacyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("remove migrated legacy desktop tabs failed", "path", legacyPath, "err", err)
		}
	}
	return legacy
}
