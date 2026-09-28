package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/fileutil"
)

const (
	runsIndexFileName      = "runs-index.sqlite"
	runsIndexDirtyFileName = "runs-index.dirty"
	runsIndexSchemaVersion = 3
)

const runsIndexSchemaSQL = `
CREATE TABLE run_index_meta (schema_version INTEGER NOT NULL);
CREATE TABLE run_summaries (
	task_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	task_name TEXT NOT NULL,
	display_name TEXT NOT NULL,
	scope TEXT NOT NULL,
	trigger TEXT NOT NULL,
	started_at_ns INTEGER NOT NULL,
	summary_json BLOB NOT NULL,
	PRIMARY KEY (task_id, run_id)
);
CREATE INDEX run_summaries_order_idx ON run_summaries(started_at_ns DESC, run_id DESC, task_id DESC);
CREATE INDEX run_summaries_scope_idx ON run_summaries(scope, started_at_ns DESC, run_id DESC, task_id DESC);
CREATE INDEX run_summaries_task_idx ON run_summaries(scope, task_id);
`

type runVisibility func(RunSummary) (bool, error)

func (s *Store) runsIndexPath() string      { return filepath.Join(s.dir, runsIndexFileName) }
func (s *Store) runsIndexDirtyPath() string { return filepath.Join(s.dir, runsIndexDirtyFileName) }

func (s *Store) indexedRunPage(ctx context.Context, filter RunFilter, cursor string, visible runVisibility) (RunPage, []RunTaskFilterOption, error) {
	if err := ctx.Err(); err != nil {
		return RunPage{}, nil, err
	}
	filter, err := normalizeRunFilter(filter)
	if err != nil {
		return RunPage{}, nil, err
	}
	s.runIndexMu.Lock()
	defer s.runIndexMu.Unlock()
	if err := s.ensureRunsIndexLocked(ctx); err != nil {
		return RunPage{}, nil, err
	}
	page, options, err := s.queryRunsIndexLocked(ctx, filter, cursor, visible)
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return page, options, err
	}
	// The index is disposable. A malformed row or database error is repaired
	// from authoritative manifests before the operation fails.
	if markErr := s.markRunsIndexDirtyLocked(); markErr != nil {
		return RunPage{}, nil, errors.Join(err, markErr)
	}
	if rebuildErr := s.rebuildRunsIndexLocked(ctx); rebuildErr != nil {
		return RunPage{}, nil, errors.Join(err, rebuildErr)
	}
	return s.queryRunsIndexLocked(ctx, filter, cursor, visible)
}

func (s *Store) ensureRunsIndexLocked(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	needsRebuild, err := s.runsIndexNeedsRebuildLocked()
	if err != nil {
		return err
	}
	if !needsRebuild && s.existingRunsIndexValidLocked() {
		return nil
	}
	if !needsRebuild {
		if err := s.markRunsIndexDirtyLocked(); err != nil {
			return err
		}
	}
	return s.rebuildRunsIndexLocked(ctx)
}

func (s *Store) existingRunsIndexValidLocked() bool {
	db, err := openRunsIndex(s.runsIndexPath())
	if err != nil {
		return false
	}
	defer db.Close()
	return validateRunsIndex(db) == nil
}

func (s *Store) runsIndexNeedsRebuildLocked() (bool, error) {
	if _, err := os.Stat(s.runsIndexDirtyPath()); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect automation run index marker: %w", err)
	}
	if _, err := os.Stat(s.runsIndexPath()); errors.Is(err, os.ErrNotExist) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("inspect automation run index: %w", err)
	}
	return false, nil
}

func (s *Store) markRunsIndexDirtyLocked() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create automation run index dir: %w", err)
	}
	if err := fileutil.AtomicWriteFile(s.runsIndexDirtyPath(), []byte(time.Now().UTC().Format(time.RFC3339Nano)+"\n"), 0o600); err != nil {
		return fmt.Errorf("mark automation run index dirty: %w", err)
	}
	return nil
}

func (s *Store) rebuildRunsIndexLocked(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.markRunsIndexDirtyLocked(); err != nil {
		return err
	}
	tmp := s.runsIndexPath() + ".rebuild"
	_ = os.Remove(tmp)
	db, err := createRunsIndexRebuildDB(ctx, tmp)
	if err != nil {
		return err
	}
	cleanup := func() { _ = db.Close(); _ = os.Remove(tmp) }
	if err = s.populateRunsIndex(ctx, db); err != nil {
		cleanup()
		return err
	}
	if err = db.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close automation run index: %w", err)
	}
	return s.installRunsIndexLocked(tmp)
}

func createRunsIndexRebuildDB(ctx context.Context, path string) (*sql.DB, error) {
	db, err := openRunsIndex(path)
	if err != nil {
		return nil, err
	}
	if _, err = db.ExecContext(ctx, runsIndexSchemaSQL); err == nil {
		_, err = db.ExecContext(ctx, `INSERT INTO run_index_meta(schema_version) VALUES (?)`, runsIndexSchemaVersion)
	}
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize automation run index: %w", err)
	}
	return db, nil
}

func (s *Store) populateRunsIndex(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("rebuild automation run index: begin: %w", err)
	}
	runs, err := s.loadRunsForIndex(ctx)
	for index := 0; err == nil && index < len(runs); index++ {
		err = upsertRunSummary(ctx, tx, runs[index])
	}
	if err == nil {
		err = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	if err != nil {
		return fmt.Errorf("rebuild automation run index: %w", err)
	}
	return nil
}

func (s *Store) installRunsIndexLocked(tmp string) error {
	_ = os.Remove(s.runsIndexPath())
	if err := os.Rename(tmp, s.runsIndexPath()); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install automation run index: %w", err)
	}
	if err := os.Remove(s.runsIndexDirtyPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear automation run index marker: %w", err)
	}
	return nil
}

func (s *Store) loadRunsForIndex(ctx context.Context) ([]RunSummary, error) {
	root := filepath.Join(s.dir, runsDirName)
	tasks, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []RunSummary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list run tasks: %w", err)
	}
	runs := make([]RunSummary, 0)
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !task.IsDir() {
			continue
		}
		taskRuns, err := loadTaskRunsForIndex(ctx, filepath.Join(root, task.Name()))
		if err != nil {
			return nil, err
		}
		runs = append(runs, taskRuns...)
	}
	return runs, nil
}

func loadTaskRunsForIndex(ctx context.Context, taskPath string) ([]RunSummary, error) {
	entries, err := os.ReadDir(taskPath)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	runs := make([]RunSummary, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		manifest, err := readRunManifest(filepath.Join(taskPath, entry.Name(), manifestFileName))
		if err != nil {
			return nil, err
		}
		runs = append(runs, runSummary(manifest))
	}
	return runs, nil
}

func openRunsIndex(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open automation run index: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure automation run index: %w", err)
	}
	return db, nil
}

func validateRunsIndex(db *sql.DB) error {
	var check string
	if err := db.QueryRow(`PRAGMA quick_check`).Scan(&check); err != nil || check != "ok" {
		if err == nil {
			err = fmt.Errorf("quick_check = %s", check)
		}
		return fmt.Errorf("validate automation run index: %w", err)
	}
	var version int
	if err := db.QueryRow(`SELECT schema_version FROM run_index_meta LIMIT 1`).Scan(&version); err != nil || version != runsIndexSchemaVersion {
		if err == nil {
			err = fmt.Errorf("schema version %d", version)
		}
		return fmt.Errorf("validate automation run index: %w", err)
	}
	return nil
}

func (s *Store) updateRunsIndexLocked(ctx context.Context, summary RunSummary) error {
	db, err := openRunsIndex(s.runsIndexPath())
	if err != nil {
		return err
	}
	defer db.Close()
	return upsertRunSummary(ctx, db, summary)
}

type runSummaryExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func upsertRunSummary(ctx context.Context, execer runSummaryExecer, summary RunSummary) error {
	payload, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = execer.ExecContext(ctx, `
INSERT INTO run_summaries(task_id, run_id, task_name, display_name, scope, trigger, started_at_ns, summary_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(task_id, run_id) DO UPDATE SET task_name=excluded.task_name, display_name=excluded.display_name, scope=excluded.scope,
trigger=excluded.trigger, started_at_ns=excluded.started_at_ns, summary_json=excluded.summary_json`,
		summary.TaskID, summary.RunID, summary.TaskName, summary.DisplayName, summary.Scope, string(summary.Trigger), summary.StartedAt.UTC().UnixNano(), payload)
	return err
}

func (s *Store) queryRunsIndexLocked(ctx context.Context, filter RunFilter, encodedCursor string, visible runVisibility) (RunPage, []RunTaskFilterOption, error) {
	db, err := openRunsIndex(s.runsIndexPath())
	if err != nil {
		return RunPage{}, nil, err
	}
	defer db.Close()
	options, err := queryRunTaskOptions(ctx, db, filter, visible)
	if err != nil {
		return RunPage{}, nil, err
	}
	where, args, err := runIndexWhere(filter, encodedCursor)
	if err != nil {
		return RunPage{}, nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT summary_json FROM run_summaries `+where+` ORDER BY started_at_ns DESC, run_id DESC, task_id DESC`, args...)
	if err != nil {
		return RunPage{}, nil, err
	}
	defer rows.Close()
	limit, _ := normalizedRunPageLimit(filter.Limit)
	items, err := scanIndexedRuns(ctx, rows, limit, visible)
	if err != nil {
		return RunPage{}, nil, err
	}
	page := RunPage{TaskFilterOptions: options}
	if len(items) <= limit {
		page.Items = items
		return page, options, nil
	}
	page.Items, page.HasMore = items[:limit], true
	page.NextCursor, err = encodeRunCursor(page.Items[len(page.Items)-1], filter)
	return page, options, err
}

func scanIndexedRuns(ctx context.Context, rows *sql.Rows, limit int, visible runVisibility) ([]RunSummary, error) {
	items := make([]RunSummary, 0, limit+1)
	for rows.Next() {
		summary, err := scanIndexedRun(ctx, rows)
		if err != nil {
			return nil, err
		}
		allowed := true
		if visible != nil {
			allowed, err = visible(summary)
		}
		if err != nil {
			return nil, err
		}
		if allowed {
			items = append(items, summary)
		}
		if len(items) == limit+1 {
			break
		}
	}
	return items, rows.Err()
}

func scanIndexedRun(ctx context.Context, rows *sql.Rows) (RunSummary, error) {
	if err := ctx.Err(); err != nil {
		return RunSummary{}, err
	}
	var payload []byte
	var summary RunSummary
	if err := rows.Scan(&payload); err != nil {
		return RunSummary{}, err
	}
	if err := json.Unmarshal(payload, &summary); err != nil {
		return RunSummary{}, err
	}
	return summary, nil
}

func queryRunTaskOptions(ctx context.Context, db *sql.DB, filter RunFilter, visible runVisibility) ([]RunTaskFilterOption, error) {
	where, args := runIndexScopeWhere(filter)
	rows, err := db.QueryContext(ctx, `SELECT task_id, task_name, scope FROM (
SELECT task_id, task_name, scope,
ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY started_at_ns DESC, run_id DESC) AS row_number
FROM run_summaries `+where+`) WHERE row_number = 1 ORDER BY lower(task_name), task_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]RunTaskFilterOption, 0)
	for rows.Next() {
		var summary RunSummary
		if err := rows.Scan(&summary.TaskID, &summary.TaskName, &summary.Scope); err != nil {
			return nil, err
		}
		allowed := true
		if visible != nil {
			allowed, err = visible(summary)
		}
		if err != nil {
			return nil, err
		}
		if allowed {
			options = append(options, RunTaskFilterOption{TaskID: summary.TaskID, TaskName: summary.TaskName})
		}
	}
	return options, rows.Err()
}

func runIndexScopeWhere(filter RunFilter) (string, []any) {
	clauses, args := []string{}, []any{}
	if filter.Scope != "" {
		clauses = append(clauses, `lower(scope) = lower(?)`)
		args = append(args, filter.Scope)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func runIndexWhere(filter RunFilter, encodedCursor string) (string, []any, error) {
	scopeWhere, args := runIndexScopeWhere(filter)
	clauses := []string{}
	if scopeWhere != "" {
		clauses = append(clauses, strings.TrimPrefix(scopeWhere, "WHERE "))
	}
	if filter.TaskID != "" {
		clauses = append(clauses, `task_id = ?`)
		args = append(args, filter.TaskID)
	}
	if filter.Trigger != "" {
		clauses = append(clauses, `trigger = ?`)
		args = append(args, string(filter.Trigger))
	}
	if filter.Search != "" {
		clauses = append(clauses, `(instr(lower(task_name), ?) > 0 OR instr(lower(display_name), ?) > 0)`)
		args = append(args, filter.Search, filter.Search)
	}
	if encodedCursor != "" {
		cursor, err := decodeRunCursor(encodedCursor, filter)
		if err != nil {
			return "", nil, err
		}
		ns := cursor.StartedAt.UTC().UnixNano()
		clauses = append(clauses, `(started_at_ns < ? OR (started_at_ns = ? AND run_id < ?) OR (started_at_ns = ? AND run_id = ? AND task_id < ?))`)
		args = append(args, ns, ns, cursor.RunID, ns, cursor.RunID, cursor.TaskID)
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, nil
}
