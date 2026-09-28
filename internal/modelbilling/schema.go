package modelbilling

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	SchemaVersionV1 = 1
	SchemaVersionV2 = 2
	SchemaVersionV3 = 3
	SchemaVersionV4 = 4
	SchemaVersionV5 = 5
	SchemaVersionV6 = 6
)

// OpenSQLite configures every pooled connection with foreign key enforcement
// and immediate write transactions. Immediate locking serializes independent
// controller/worker handles before a transaction can enter a read-to-write
// upgrade, while explicit read-only transactions remain ordinary reads.
// Stores using the B0 schema must open their database through this function.
func OpenSQLite(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(path) != path {
		return nil, fmt.Errorf("modelbilling: sqlite path is required and must be trimmed")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("modelbilling: resolve sqlite path: %w", err)
	}
	uriPath := filepath.ToSlash(absPath)
	if volume := filepath.VolumeName(absPath); volume != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := (&url.URL{Scheme: "file", Path: uriPath}).String() +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("modelbilling: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("modelbilling: ping sqlite: %w", err)
	}
	return db, nil
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("modelbilling: database is required")
	}
	if err := verifyForeignKeys(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, migrationTableSQL); err != nil {
		return fmt.Errorf("modelbilling: create migration table: %w", err)
	}
	for _, migration := range []struct {
		version int
		apply   func(context.Context, *sql.DB) error
	}{{SchemaVersionV1, applySchemaV1}, {SchemaVersionV2, applySchemaV2}, {SchemaVersionV3, applySchemaV3}, {SchemaVersionV4, applySchemaV4}, {SchemaVersionV5, applySchemaV5}, {SchemaVersionV6, applySchemaV6}} {
		applied, err := migrationApplied(ctx, db, migration.version)
		if err != nil {
			return err
		}
		if !applied {
			if err := migration.apply(ctx, db); err != nil {
				return err
			}
		}
	}
	return nil
}

func applySchemaV6(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("modelbilling: begin migration v6: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schemaV6SQL); err != nil {
		return fmt.Errorf("modelbilling: apply migration v6: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("modelbilling: check migration v6 foreign keys: %w", err)
	}
	violation := false
	for rows.Next() {
		violation = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("modelbilling: check migration v6 foreign keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("modelbilling: close migration v6 foreign key check: %w", err)
	}
	if violation {
		return fmt.Errorf("modelbilling: migration v6 produced foreign key violations")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO model_billing_schema_migrations (version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, SchemaVersionV6); err != nil {
		return fmt.Errorf("modelbilling: record migration v6: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("modelbilling: commit migration v6: %w", err)
	}
	return nil
}

func applySchemaV5(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("modelbilling: begin migration v5: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schemaV5SQL); err != nil {
		return fmt.Errorf("modelbilling: apply migration v5: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("modelbilling: check migration v5 foreign keys: %w", err)
	}
	violation := false
	for rows.Next() {
		violation = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("modelbilling: check migration v5 foreign keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("modelbilling: close migration v5 foreign key check: %w", err)
	}
	if violation {
		return fmt.Errorf("modelbilling: migration v5 produced foreign key violations")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO model_billing_schema_migrations (version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, SchemaVersionV5); err != nil {
		return fmt.Errorf("modelbilling: record migration v5: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("modelbilling: commit migration v5: %w", err)
	}
	return nil
}

func applySchemaV4(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("modelbilling: begin migration v4: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schemaV4SQL); err != nil {
		return fmt.Errorf("modelbilling: apply migration v4: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("modelbilling: check migration v4 foreign keys: %w", err)
	}
	violation := false
	for rows.Next() {
		violation = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("modelbilling: check migration v4 foreign keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("modelbilling: close migration v4 foreign key check: %w", err)
	}
	if violation {
		return fmt.Errorf("modelbilling: migration v4 produced foreign key violations")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO model_billing_schema_migrations (version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, SchemaVersionV4); err != nil {
		return fmt.Errorf("modelbilling: record migration v4: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("modelbilling: commit migration v4: %w", err)
	}
	return nil
}

func verifyForeignKeys(ctx context.Context, db *sql.DB) error {
	var foreignKeys int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		return fmt.Errorf("modelbilling: inspect foreign key enforcement: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("modelbilling: foreign keys are disabled; open the database with OpenSQLite")
	}
	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version int) (bool, error) {
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM model_billing_schema_migrations WHERE version = ?`, version).Scan(&applied); err != nil {
		return false, fmt.Errorf("modelbilling: inspect migration v%d: %w", version, err)
	}
	return applied > 0, nil
}

func applySchemaV3(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("modelbilling: begin migration v3: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schemaV3SQL); err != nil {
		return fmt.Errorf("modelbilling: apply migration v3: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("modelbilling: check migration v3 foreign keys: %w", err)
	}
	defer rows.Close()
	violation := false
	for rows.Next() {
		violation = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("modelbilling: check migration v3 foreign keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("modelbilling: close migration v3 foreign key check: %w", err)
	}
	if violation {
		return fmt.Errorf("modelbilling: migration v3 produced foreign key violations")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO model_billing_schema_migrations (version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, SchemaVersionV3); err != nil {
		return fmt.Errorf("modelbilling: record migration v3: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("modelbilling: commit migration v3: %w", err)
	}
	return nil
}

func applySchemaV2(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("modelbilling: begin migration v2: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schemaV2SQL); err != nil {
		return fmt.Errorf("modelbilling: apply migration v2: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("modelbilling: check migration v2 foreign keys: %w", err)
	}
	defer rows.Close()
	violation := false
	for rows.Next() {
		violation = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("modelbilling: check migration v2 foreign keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("modelbilling: close migration v2 foreign key check: %w", err)
	}
	if violation {
		return fmt.Errorf("modelbilling: migration v2 produced foreign key violations")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO model_billing_schema_migrations (version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, SchemaVersionV2); err != nil {
		return fmt.Errorf("modelbilling: record migration v2: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("modelbilling: commit migration v2: %w", err)
	}
	return nil
}

func applySchemaV1(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("modelbilling: begin migration v1: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schemaV1SQL); err != nil {
		return fmt.Errorf("modelbilling: apply migration v1: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO model_billing_schema_migrations (version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`, SchemaVersionV1); err != nil {
		return fmt.Errorf("modelbilling: record migration v1: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("modelbilling: commit migration v1: %w", err)
	}
	return nil
}

const migrationTableSQL = `
CREATE TABLE IF NOT EXISTS model_billing_schema_migrations (
	version INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL
)`

const schemaV1SQL = `
CREATE TABLE model_tasks (
	task_id TEXT PRIMARY KEY,
	scope_key TEXT NOT NULL CHECK (
		length(scope_key) = 42 AND substr(scope_key, 1, 10) = 'kwu_scope_'
		AND substr(scope_key, 11) NOT GLOB '*[^0-9a-f]*'
	),
	actor_id TEXT NOT NULL,
	profile_id TEXT NOT NULL,
	cloud_user_id TEXT,
	binding_generation INTEGER,
	billing_source TEXT NOT NULL CHECK (billing_source IN ('subscription_pool', 'newapi_byok')),
	initial_model_id TEXT,
	client_task_type TEXT,
	client_estimated_cost TEXT,
	freeze_idempotency_key TEXT,
	freeze_id TEXT,
	freeze_status TEXT,
	freeze_amount TEXT,
	free_frozen_amount TEXT,
	subscription_frozen_amount TEXT,
	low_balance_mode INTEGER CHECK (low_balance_mode IS NULL OR low_balance_mode IN (0, 1)),
	freeze_expires_at TEXT,
	entitlement_snapshot_json BLOB,
	pricing_evidence_id TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN (
		'created', 'freeze_pending', 'ready', 'running', 'usage_pending', 'settling',
		'completed', 'failed', 'cancelled', 'recovery_required'
	)),
	task_result TEXT CHECK (task_result IS NULL OR task_result IN ('completed', 'failed', 'cancelled')),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	ended_at TEXT,
	last_error_code TEXT,
	CHECK (
		(billing_source = 'subscription_pool'
			AND cloud_user_id IS NOT NULL AND length(trim(cloud_user_id)) > 0
			AND binding_generation IS NOT NULL AND binding_generation > 0)
		OR
		(billing_source = 'newapi_byok' AND cloud_user_id IS NULL AND binding_generation IS NULL)
	),
	CHECK (
		(state IN ('created', 'freeze_pending', 'ready', 'running') AND task_result IS NULL)
		OR (state = 'failed' AND task_result IS NOT NULL AND task_result = 'failed')
		OR (state = 'cancelled' AND task_result IS NOT NULL AND task_result = 'cancelled')
		OR (state IN ('usage_pending', 'settling') AND task_result IS NOT NULL)
		OR (state = 'completed' AND task_result IS NOT NULL
			AND (billing_source = 'subscription_pool' OR task_result = 'completed'))
		OR state = 'recovery_required'
	),
	FOREIGN KEY(pricing_evidence_id, task_id)
		REFERENCES pricing_evidence(evidence_id, task_id)
		DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE model_calls (
	call_id TEXT PRIMARY KEY,
	task_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('main', 'planner', 'executor', 'subagent', 'compaction')),
	model_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	provider_request_id TEXT,
	pricing_evidence_id TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN (
		'prepared', 'running', 'completed', 'failed', 'cancelled', 'interrupted', 'usage_missing'
	)),
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	normalized_usage_json BLOB,
	raw_usage_redacted_json BLOB,
	started_at TEXT,
	finished_at TEXT,
	last_error_code TEXT,
	UNIQUE(call_id, task_id),
	FOREIGN KEY(task_id) REFERENCES model_tasks(task_id),
	FOREIGN KEY(pricing_evidence_id, task_id, model_id)
		REFERENCES pricing_evidence(evidence_id, task_id, model_id)
);

CREATE TABLE usage_reports (
	idempotency_key TEXT PRIMARY KEY,
	task_id TEXT NOT NULL REFERENCES model_tasks(task_id),
	report_kind TEXT NOT NULL CHECK (report_kind IN ('partial', 'final')),
	sequence INTEGER NOT NULL,
	canonical_payload BLOB NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('pending', 'delivering', 'accepted', 'retry', 'blocked')),
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	next_attempt_at TEXT,
	last_error_code TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	CHECK (
		(report_kind = 'partial' AND sequence > 0)
		OR
		(report_kind = 'final' AND sequence = 0)
	),
	CHECK (
		(state IN ('pending', 'delivering', 'retry') AND next_attempt_at IS NOT NULL)
		OR
		(state IN ('accepted', 'blocked') AND next_attempt_at IS NULL)
	),
	UNIQUE(task_id, report_kind, sequence),
	UNIQUE(idempotency_key, task_id)
);

CREATE TRIGGER usage_reports_subscription_only_insert
BEFORE INSERT ON usage_reports
FOR EACH ROW
WHEN COALESCE((SELECT billing_source FROM model_tasks WHERE task_id = NEW.task_id), '') <> 'subscription_pool'
BEGIN
	SELECT RAISE(ABORT, 'usage_reports require a subscription task');
END;

CREATE TRIGGER usage_reports_subscription_only_update
BEFORE UPDATE OF task_id ON usage_reports
FOR EACH ROW
WHEN COALESCE((SELECT billing_source FROM model_tasks WHERE task_id = NEW.task_id), '') <> 'subscription_pool'
BEGIN
	SELECT RAISE(ABORT, 'usage_reports require a subscription task');
END;

CREATE TRIGGER model_tasks_keep_subscription_reports
BEFORE UPDATE OF billing_source ON model_tasks
FOR EACH ROW
WHEN NEW.billing_source <> 'subscription_pool'
	AND EXISTS (SELECT 1 FROM usage_reports WHERE task_id = OLD.task_id)
BEGIN
	SELECT RAISE(ABORT, 'task with usage reports must remain subscription');
END;

CREATE TABLE usage_report_calls (
	idempotency_key TEXT NOT NULL,
	task_id TEXT NOT NULL,
	call_id TEXT NOT NULL,
	PRIMARY KEY(idempotency_key, call_id),
	FOREIGN KEY(idempotency_key, task_id)
		REFERENCES usage_reports(idempotency_key, task_id),
	FOREIGN KEY(call_id, task_id)
		REFERENCES model_calls(call_id, task_id)
);

CREATE TABLE pricing_evidence (
	evidence_id TEXT PRIMARY KEY,
	snapshot_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	model_id TEXT NOT NULL,
	pricing_group TEXT NOT NULL,
	pricing_version TEXT,
	pricing_hash TEXT,
	state TEXT NOT NULL CHECK (state IN ('fresh', 'stale_usable', 'stale_blocked', 'unavailable')),
	captured_at TEXT NOT NULL,
	UNIQUE(evidence_id, task_id),
	UNIQUE(evidence_id, task_id, model_id),
	FOREIGN KEY(task_id) REFERENCES model_tasks(task_id)
		DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX idx_model_tasks_recoverable
	ON model_tasks(scope_key, state, updated_at);
CREATE INDEX idx_model_calls_task
	ON model_calls(task_id, started_at);
CREATE INDEX idx_usage_reports_due
	ON usage_reports(task_id, state, next_attempt_at);
CREATE INDEX idx_pricing_evidence_task
	ON pricing_evidence(task_id, captured_at);
`

// V2 only widens the immutable call-role vocabulary. SQLite cannot alter a
// CHECK constraint, so it rebuilds the call table and its dependent join table
// in one transaction while preserving every persisted row.
const schemaV2SQL = `
CREATE TABLE model_calls_v2 (
	call_id TEXT PRIMARY KEY, task_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('main','planner','executor','subagent','compaction','classifier','tasktree_planner','title')),
	model_id TEXT NOT NULL, provider TEXT NOT NULL, provider_request_id TEXT,
	pricing_evidence_id TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('prepared','running','completed','failed','cancelled','interrupted','usage_missing')),
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	normalized_usage_json BLOB, raw_usage_redacted_json BLOB, started_at TEXT, finished_at TEXT, last_error_code TEXT,
	UNIQUE(call_id, task_id),
	FOREIGN KEY(task_id) REFERENCES model_tasks(task_id),
	FOREIGN KEY(pricing_evidence_id, task_id, model_id) REFERENCES pricing_evidence(evidence_id, task_id, model_id)
);
INSERT INTO model_calls_v2 SELECT * FROM model_calls;
CREATE TABLE usage_report_calls_v2 (
	idempotency_key TEXT NOT NULL, task_id TEXT NOT NULL, call_id TEXT NOT NULL,
	PRIMARY KEY(idempotency_key, call_id),
	FOREIGN KEY(idempotency_key, task_id) REFERENCES usage_reports(idempotency_key, task_id),
	FOREIGN KEY(call_id, task_id) REFERENCES model_calls_v2(call_id, task_id)
);
INSERT INTO usage_report_calls_v2 SELECT * FROM usage_report_calls;
DROP TABLE usage_report_calls;
DROP TABLE model_calls;
ALTER TABLE model_calls_v2 RENAME TO model_calls;
ALTER TABLE usage_report_calls_v2 RENAME TO usage_report_calls;
CREATE INDEX idx_model_calls_task ON model_calls(task_id, started_at);
`

// V3 adds guardian as a first-class immutable call role and backfills the
// execution end of legacy subscription usage-pending rows. The dependent
// report join is rebuilt in the same transaction so its composite call foreign
// key is preserved while SQLite replaces the CHECK-constrained call table.
const schemaV3SQL = `
UPDATE model_tasks
SET ended_at = updated_at
WHERE billing_source = 'subscription_pool'
	AND state = 'usage_pending'
	AND ended_at IS NULL;
CREATE TABLE model_calls_v3 (
	call_id TEXT PRIMARY KEY,
	task_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('main','planner','executor','subagent','compaction','classifier','tasktree_planner','title','guardian')),
	model_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	provider_request_id TEXT,
	pricing_evidence_id TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('prepared','running','completed','failed','cancelled','interrupted','usage_missing')),
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	normalized_usage_json BLOB,
	raw_usage_redacted_json BLOB,
	started_at TEXT,
	finished_at TEXT,
	last_error_code TEXT,
	UNIQUE(call_id, task_id),
	FOREIGN KEY(task_id) REFERENCES model_tasks(task_id),
	FOREIGN KEY(pricing_evidence_id, task_id, model_id)
		REFERENCES pricing_evidence(evidence_id, task_id, model_id)
);
INSERT INTO model_calls_v3 (
	call_id, task_id, role, model_id, provider, provider_request_id,
	pricing_evidence_id, state, attempt_count, normalized_usage_json,
	raw_usage_redacted_json, started_at, finished_at, last_error_code
)
SELECT
	call_id, task_id, role, model_id, provider, provider_request_id,
	pricing_evidence_id, state, attempt_count, normalized_usage_json,
	raw_usage_redacted_json, started_at, finished_at, last_error_code
FROM model_calls;
CREATE TABLE usage_report_calls_v3 (
	idempotency_key TEXT NOT NULL,
	task_id TEXT NOT NULL,
	call_id TEXT NOT NULL,
	PRIMARY KEY(idempotency_key, call_id),
	FOREIGN KEY(idempotency_key, task_id)
		REFERENCES usage_reports(idempotency_key, task_id),
	FOREIGN KEY(call_id, task_id)
		REFERENCES model_calls_v3(call_id, task_id)
);
INSERT INTO usage_report_calls_v3 (idempotency_key, task_id, call_id)
	SELECT idempotency_key, task_id, call_id FROM usage_report_calls;
DROP TABLE usage_report_calls;
DROP TABLE model_calls;
ALTER TABLE model_calls_v3 RENAME TO model_calls;
ALTER TABLE usage_report_calls_v3 RENAME TO usage_report_calls;
CREATE INDEX idx_model_calls_task ON model_calls(task_id, started_at);
`

// V4 moves subscription settlement onto immutable Maas quota evidence while
// retaining token usage and pricing rows as local observability/history. Old
// token reports keep their payload bytes but are removed from every due path.
const schemaV4SQL = `
ALTER TABLE model_calls ADD COLUMN endpoint TEXT;
ALTER TABLE model_calls ADD COLUMN occurred_at TEXT;
ALTER TABLE model_calls ADD COLUMN quota_report_status TEXT NOT NULL DEFAULT 'unreported'
	CHECK (quota_report_status IN ('unreported','partial_reported','final_pending','final_reported'));
ALTER TABLE model_calls ADD COLUMN quota_report_last_error_code TEXT;

ALTER TABLE usage_reports ADD COLUMN contract_version TEXT NOT NULL DEFAULT 'legacy-token-v1'
	CHECK (contract_version IN ('legacy-token-v1','quota-contract-v1'));
ALTER TABLE usage_reports ADD COLUMN payload_hash TEXT
	CHECK (payload_hash IS NULL OR (
		length(payload_hash) = 64 AND payload_hash NOT GLOB '*[^0-9a-f]*'
	));

CREATE TRIGGER usage_reports_quota_hash_required_insert
BEFORE INSERT ON usage_reports
FOR EACH ROW
WHEN NEW.contract_version='quota-contract-v1' AND NEW.payload_hash IS NULL
BEGIN
	SELECT RAISE(ABORT, 'quota report payload hash is required');
END;

CREATE TRIGGER usage_reports_quota_hash_required_update
BEFORE UPDATE OF contract_version,payload_hash,canonical_payload ON usage_reports
FOR EACH ROW
WHEN NEW.contract_version='quota-contract-v1' AND NEW.payload_hash IS NULL
BEGIN
	SELECT RAISE(ABORT, 'quota report payload hash is required');
END;

CREATE TRIGGER usage_reports_contract_version_immutable
BEFORE UPDATE OF contract_version ON usage_reports
FOR EACH ROW
WHEN NEW.contract_version IS NOT OLD.contract_version
BEGIN
	SELECT RAISE(ABORT, 'report contract version is immutable');
END;

CREATE TRIGGER usage_reports_accepted_immutable_update
BEFORE UPDATE ON usage_reports
FOR EACH ROW
WHEN OLD.state='accepted'
BEGIN
	SELECT RAISE(ABORT, 'accepted report is immutable');
END;

CREATE TRIGGER usage_reports_accepted_immutable_delete
BEFORE DELETE ON usage_reports
FOR EACH ROW
WHEN OLD.state='accepted'
BEGIN
	SELECT RAISE(ABORT, 'accepted report is immutable');
END;

CREATE TRIGGER usage_report_calls_accepted_immutable_insert
BEFORE INSERT ON usage_report_calls
FOR EACH ROW
WHEN EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=NEW.idempotency_key AND task_id=NEW.task_id AND state='accepted'
)
BEGIN
	SELECT RAISE(ABORT, 'accepted report call links are immutable');
END;

CREATE TRIGGER usage_report_calls_accepted_immutable_update
BEFORE UPDATE ON usage_report_calls
FOR EACH ROW
WHEN EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=OLD.idempotency_key AND task_id=OLD.task_id AND state='accepted'
) OR EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=NEW.idempotency_key AND task_id=NEW.task_id AND state='accepted'
)
BEGIN
	SELECT RAISE(ABORT, 'accepted report call links are immutable');
END;

CREATE TRIGGER usage_report_calls_accepted_immutable_delete
BEFORE DELETE ON usage_report_calls
FOR EACH ROW
WHEN EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=OLD.idempotency_key AND task_id=OLD.task_id AND state='accepted'
)
BEGIN
	SELECT RAISE(ABORT, 'accepted report call links are immutable');
END;

UPDATE usage_reports
SET state='blocked', next_attempt_at=NULL, last_error_code='legacy_token_report_contract'
WHERE state <> 'accepted';

UPDATE model_tasks
SET state='recovery_required', last_error_code='legacy_token_report_contract'
WHERE billing_source='subscription_pool'
	AND state IN ('usage_pending','settling')
	AND EXISTS (SELECT 1 FROM usage_reports r WHERE r.task_id=model_tasks.task_id);

CREATE TABLE model_call_quota_evidence_revisions (
	call_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	evidence_revision INTEGER NOT NULL CHECK (evidence_revision > 0),
	oneapi_request_id TEXT,
	billing_quota_present INTEGER NOT NULL CHECK (billing_quota_present IN (0,1)),
	billing_quota_type TEXT CHECK (billing_quota_type IS NULL OR billing_quota_type IN ('number','string','null','bool','object','array')),
	billing_quota_value BLOB,
	usage_quota_present INTEGER NOT NULL CHECK (usage_quota_present IN (0,1)),
	usage_quota_type TEXT CHECK (usage_quota_type IS NULL OR usage_quota_type IN ('number','string','null','bool','object','array')),
	usage_quota_value BLOB,
	quota TEXT,
	quota_source TEXT CHECK (quota_source IS NULL OR quota_source IN ('billing.quota','usage.quota','both_consistent')),
	evidence_status TEXT NOT NULL CHECK (evidence_status IN ('complete','missing','conflicting','invalid')),
	evidence_issues_json BLOB NOT NULL,
	evidence_observed_at TEXT NOT NULL,
	PRIMARY KEY(call_id,evidence_revision),
	FOREIGN KEY(call_id,task_id) REFERENCES model_calls(call_id,task_id),
	CHECK (
		(billing_quota_present=0 AND billing_quota_type IS NULL AND billing_quota_value IS NULL)
		OR (billing_quota_present=1 AND billing_quota_type IS NOT NULL AND billing_quota_value IS NOT NULL)
	),
	CHECK (
		(usage_quota_present=0 AND usage_quota_type IS NULL AND usage_quota_value IS NULL)
		OR (usage_quota_present=1 AND usage_quota_type IS NOT NULL AND usage_quota_value IS NOT NULL)
	)
);

CREATE TRIGGER model_call_quota_evidence_subscription_only
BEFORE INSERT ON model_call_quota_evidence_revisions
FOR EACH ROW
WHEN COALESCE((
	SELECT t.billing_source FROM model_calls c
	JOIN model_tasks t ON t.task_id=c.task_id
	WHERE c.call_id=NEW.call_id AND c.task_id=NEW.task_id
), '') <> 'subscription_pool'
BEGIN
	SELECT RAISE(ABORT, 'quota evidence requires a subscription call');
END;

CREATE TRIGGER model_call_quota_evidence_next_revision
BEFORE INSERT ON model_call_quota_evidence_revisions
FOR EACH ROW
WHEN NEW.evidence_revision <> COALESCE((
	SELECT MAX(evidence_revision)+1 FROM model_call_quota_evidence_revisions
	WHERE call_id=NEW.call_id
), 1)
BEGIN
	SELECT RAISE(ABORT, 'quota evidence revision must append by one');
END;

CREATE TRIGGER model_call_quota_evidence_no_complete_regression
BEFORE INSERT ON model_call_quota_evidence_revisions
FOR EACH ROW
WHEN COALESCE((
	SELECT evidence_status FROM model_call_quota_evidence_revisions
	WHERE call_id=NEW.call_id ORDER BY evidence_revision DESC LIMIT 1
), '') = 'complete' AND NEW.evidence_status <> 'complete'
BEGIN
	SELECT RAISE(ABORT, 'complete quota evidence cannot regress');
END;

CREATE TRIGGER model_call_quota_evidence_immutable_update
BEFORE UPDATE ON model_call_quota_evidence_revisions
BEGIN
	SELECT RAISE(ABORT, 'quota evidence revisions are immutable');
END;

CREATE TRIGGER model_call_quota_evidence_immutable_delete
BEFORE DELETE ON model_call_quota_evidence_revisions
BEGIN
	SELECT RAISE(ABORT, 'quota evidence revisions are immutable');
END;

CREATE VIEW model_call_quota_evidence_current AS
SELECT r.*
FROM model_call_quota_evidence_revisions r
WHERE r.evidence_revision=(
	SELECT MAX(latest.evidence_revision)
	FROM model_call_quota_evidence_revisions latest
	WHERE latest.call_id=r.call_id
);

CREATE INDEX idx_model_call_quota_evidence_task
	ON model_call_quota_evidence_revisions(task_id,call_id,evidence_revision);

CREATE TABLE model_task_quota_workflow (
	task_id TEXT PRIMARY KEY REFERENCES model_tasks(task_id),
	status TEXT NOT NULL CHECK (status IN (
		'collecting','evidence_pending','supplementing','final_upload_pending','final_uploading',
		'settlement_pending','settled','settled_with_uncovered_amount','retry_pending','failed_manual_required'
	)),
	retry_stage TEXT CHECK (retry_stage IS NULL OR retry_stage IN ('final_upload','settlement_query')),
	last_error_code TEXT,
	updated_at TEXT NOT NULL,
	CHECK (
		(status='retry_pending' AND retry_stage IS NOT NULL)
		OR (status<>'retry_pending' AND retry_stage IS NULL)
	)
);

INSERT INTO model_task_quota_workflow(task_id,status,retry_stage,last_error_code,updated_at)
SELECT task_id,
	CASE
		WHEN state IN ('usage_pending','settling','completed','recovery_required')
			OR EXISTS (SELECT 1 FROM usage_reports r WHERE r.task_id=model_tasks.task_id)
		THEN 'failed_manual_required'
		ELSE 'collecting'
	END,
	NULL,
	CASE
		WHEN state IN ('usage_pending','settling','completed','recovery_required')
			OR EXISTS (SELECT 1 FROM usage_reports r WHERE r.task_id=model_tasks.task_id)
		THEN 'legacy_token_report_contract'
		ELSE NULL
	END,
	updated_at
FROM model_tasks
WHERE billing_source='subscription_pool';

CREATE INDEX idx_model_task_quota_workflow_status
	ON model_task_quota_workflow(status,updated_at,task_id);

CREATE TABLE model_task_quota_settlement_revisions (
	task_id TEXT NOT NULL,
	settlement_revision INTEGER NOT NULL CHECK (settlement_revision > 0),
	settlement_id TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN (
		'settlement_pending','settled','settled_with_uncovered_amount','failed_manual_required'
	)),
	normalized_payload BLOB NOT NULL,
	payload_hash TEXT NOT NULL CHECK (
		length(payload_hash) = 64 AND payload_hash NOT GLOB '*[^0-9a-f]*'
	),
	observed_at TEXT NOT NULL,
	PRIMARY KEY(task_id,settlement_revision),
	FOREIGN KEY(task_id) REFERENCES model_tasks(task_id)
);

CREATE TRIGGER model_task_quota_settlement_subscription_only
BEFORE INSERT ON model_task_quota_settlement_revisions
FOR EACH ROW
WHEN COALESCE((
	SELECT billing_source FROM model_tasks WHERE task_id=NEW.task_id
), '') <> 'subscription_pool'
BEGIN
	SELECT RAISE(ABORT, 'quota settlement requires a subscription task');
END;

CREATE TRIGGER model_task_quota_settlement_next_revision
BEFORE INSERT ON model_task_quota_settlement_revisions
FOR EACH ROW
WHEN NEW.settlement_revision <> COALESCE((
	SELECT MAX(settlement_revision)+1 FROM model_task_quota_settlement_revisions
	WHERE task_id=NEW.task_id
), 1)
BEGIN
	SELECT RAISE(ABORT, 'quota settlement revision must append by one');
END;

CREATE TRIGGER model_task_quota_settlement_immutable_update
BEFORE UPDATE ON model_task_quota_settlement_revisions
BEGIN
	SELECT RAISE(ABORT, 'quota settlement revisions are immutable');
END;

CREATE TRIGGER model_task_quota_settlement_immutable_delete
BEFORE DELETE ON model_task_quota_settlement_revisions
BEGIN
	SELECT RAISE(ABORT, 'quota settlement revisions are immutable');
END;

CREATE VIEW model_task_quota_settlement_current AS
SELECT r.*
FROM model_task_quota_settlement_revisions r
WHERE r.settlement_revision=(
	SELECT MAX(latest.settlement_revision)
	FROM model_task_quota_settlement_revisions latest
	WHERE latest.task_id=r.task_id
);

CREATE INDEX idx_model_task_quota_settlement_status
	ON model_task_quota_settlement_revisions(status,observed_at,task_id);
`

// V5 records the first proven task execution start. Historical rows remain
// NULL because their prior updated_at values may describe recovery rather
// than the original Ready -> Running transition.
const schemaV5SQL = `
ALTER TABLE model_tasks ADD COLUMN started_at TEXT;

CREATE TRIGGER model_tasks_started_at_first_running
BEFORE UPDATE OF state,started_at ON model_tasks
FOR EACH ROW
WHEN OLD.started_at IS NULL AND NEW.started_at IS NOT NULL
	AND NOT (OLD.state='ready' AND NEW.state='running')
BEGIN
	SELECT RAISE(ABORT, 'task started_at requires the first ready to running transition');
END;

CREATE TRIGGER model_tasks_started_at_immutable
BEFORE UPDATE OF started_at ON model_tasks
FOR EACH ROW
WHEN OLD.started_at IS NOT NULL AND NEW.started_at IS NOT OLD.started_at
BEGIN
	SELECT RAISE(ABORT, 'task started_at is immutable');
END;
`

// V6 scopes immutable call identity to its owning task. SQLite cannot change
// a primary key in place, so the call table and both dependent call-link
// tables are rebuilt atomically while preserving every V5 row and column.
const schemaV6SQL = `
CREATE TABLE model_calls_v6 (
	call_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('main','planner','executor','subagent','compaction','classifier','tasktree_planner','title','guardian')),
	model_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	provider_request_id TEXT,
	pricing_evidence_id TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('prepared','running','completed','failed','cancelled','interrupted','usage_missing')),
	attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
	normalized_usage_json BLOB,
	raw_usage_redacted_json BLOB,
	started_at TEXT,
	finished_at TEXT,
	last_error_code TEXT,
	endpoint TEXT,
	occurred_at TEXT,
	quota_report_status TEXT NOT NULL DEFAULT 'unreported'
		CHECK (quota_report_status IN ('unreported','partial_reported','final_pending','final_reported')),
	quota_report_last_error_code TEXT,
	PRIMARY KEY(task_id,call_id),
	UNIQUE(call_id,task_id),
	FOREIGN KEY(task_id) REFERENCES model_tasks(task_id),
	FOREIGN KEY(pricing_evidence_id,task_id,model_id)
		REFERENCES pricing_evidence(evidence_id,task_id,model_id)
);

INSERT INTO model_calls_v6 (
	call_id,task_id,role,model_id,provider,provider_request_id,
	pricing_evidence_id,state,attempt_count,normalized_usage_json,
	raw_usage_redacted_json,started_at,finished_at,last_error_code,
	endpoint,occurred_at,quota_report_status,quota_report_last_error_code
)
SELECT
	call_id,task_id,role,model_id,provider,provider_request_id,
	pricing_evidence_id,state,attempt_count,normalized_usage_json,
	raw_usage_redacted_json,started_at,finished_at,last_error_code,
	endpoint,occurred_at,quota_report_status,quota_report_last_error_code
FROM model_calls;

CREATE TABLE usage_report_calls_v6 (
	idempotency_key TEXT NOT NULL,
	task_id TEXT NOT NULL,
	call_id TEXT NOT NULL,
	PRIMARY KEY(idempotency_key,call_id),
	FOREIGN KEY(idempotency_key,task_id)
		REFERENCES usage_reports(idempotency_key,task_id),
	FOREIGN KEY(task_id,call_id)
		REFERENCES model_calls_v6(task_id,call_id)
);

INSERT INTO usage_report_calls_v6 (idempotency_key,task_id,call_id)
	SELECT idempotency_key,task_id,call_id FROM usage_report_calls;

CREATE TABLE model_call_quota_evidence_revisions_v6 (
	call_id TEXT NOT NULL,
	task_id TEXT NOT NULL,
	evidence_revision INTEGER NOT NULL CHECK (evidence_revision > 0),
	oneapi_request_id TEXT,
	billing_quota_present INTEGER NOT NULL CHECK (billing_quota_present IN (0,1)),
	billing_quota_type TEXT CHECK (billing_quota_type IS NULL OR billing_quota_type IN ('number','string','null','bool','object','array')),
	billing_quota_value BLOB,
	usage_quota_present INTEGER NOT NULL CHECK (usage_quota_present IN (0,1)),
	usage_quota_type TEXT CHECK (usage_quota_type IS NULL OR usage_quota_type IN ('number','string','null','bool','object','array')),
	usage_quota_value BLOB,
	quota TEXT,
	quota_source TEXT CHECK (quota_source IS NULL OR quota_source IN ('billing.quota','usage.quota','both_consistent')),
	evidence_status TEXT NOT NULL CHECK (evidence_status IN ('complete','missing','conflicting','invalid')),
	evidence_issues_json BLOB NOT NULL,
	evidence_observed_at TEXT NOT NULL,
	PRIMARY KEY(task_id,call_id,evidence_revision),
	FOREIGN KEY(task_id,call_id) REFERENCES model_calls_v6(task_id,call_id),
	CHECK (
		(billing_quota_present=0 AND billing_quota_type IS NULL AND billing_quota_value IS NULL)
		OR (billing_quota_present=1 AND billing_quota_type IS NOT NULL AND billing_quota_value IS NOT NULL)
	),
	CHECK (
		(usage_quota_present=0 AND usage_quota_type IS NULL AND usage_quota_value IS NULL)
		OR (usage_quota_present=1 AND usage_quota_type IS NOT NULL AND usage_quota_value IS NOT NULL)
	)
);

INSERT INTO model_call_quota_evidence_revisions_v6 (
	call_id,task_id,evidence_revision,oneapi_request_id,
	billing_quota_present,billing_quota_type,billing_quota_value,
	usage_quota_present,usage_quota_type,usage_quota_value,
	quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at
)
SELECT
	call_id,task_id,evidence_revision,oneapi_request_id,
	billing_quota_present,billing_quota_type,billing_quota_value,
	usage_quota_present,usage_quota_type,usage_quota_value,
	quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at
FROM model_call_quota_evidence_revisions;

DROP VIEW model_call_quota_evidence_current;
DROP TRIGGER model_call_quota_evidence_subscription_only;
DROP TRIGGER model_call_quota_evidence_next_revision;
DROP TRIGGER model_call_quota_evidence_no_complete_regression;
DROP TRIGGER model_call_quota_evidence_immutable_update;
DROP TRIGGER model_call_quota_evidence_immutable_delete;
DROP TRIGGER usage_report_calls_accepted_immutable_insert;
DROP TRIGGER usage_report_calls_accepted_immutable_update;
DROP TRIGGER usage_report_calls_accepted_immutable_delete;

DROP TABLE usage_report_calls;
DROP TABLE model_call_quota_evidence_revisions;
DROP TABLE model_calls;

ALTER TABLE model_calls_v6 RENAME TO model_calls;
ALTER TABLE usage_report_calls_v6 RENAME TO usage_report_calls;
ALTER TABLE model_call_quota_evidence_revisions_v6 RENAME TO model_call_quota_evidence_revisions;

CREATE INDEX idx_model_calls_task
	ON model_calls(task_id,started_at);
CREATE INDEX idx_model_call_quota_evidence_task
	ON model_call_quota_evidence_revisions(task_id,call_id,evidence_revision);

CREATE TRIGGER usage_report_calls_accepted_immutable_insert
BEFORE INSERT ON usage_report_calls
FOR EACH ROW
WHEN EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=NEW.idempotency_key AND task_id=NEW.task_id AND state='accepted'
)
BEGIN
	SELECT RAISE(ABORT, 'accepted report call links are immutable');
END;

CREATE TRIGGER usage_report_calls_accepted_immutable_update
BEFORE UPDATE ON usage_report_calls
FOR EACH ROW
WHEN EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=OLD.idempotency_key AND task_id=OLD.task_id AND state='accepted'
) OR EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=NEW.idempotency_key AND task_id=NEW.task_id AND state='accepted'
)
BEGIN
	SELECT RAISE(ABORT, 'accepted report call links are immutable');
END;

CREATE TRIGGER usage_report_calls_accepted_immutable_delete
BEFORE DELETE ON usage_report_calls
FOR EACH ROW
WHEN EXISTS (
	SELECT 1 FROM usage_reports
	WHERE idempotency_key=OLD.idempotency_key AND task_id=OLD.task_id AND state='accepted'
)
BEGIN
	SELECT RAISE(ABORT, 'accepted report call links are immutable');
END;

CREATE TRIGGER model_call_quota_evidence_subscription_only
BEFORE INSERT ON model_call_quota_evidence_revisions
FOR EACH ROW
WHEN COALESCE((
	SELECT t.billing_source FROM model_calls c
	JOIN model_tasks t ON t.task_id=c.task_id
	WHERE c.task_id=NEW.task_id AND c.call_id=NEW.call_id
), '') <> 'subscription_pool'
BEGIN
	SELECT RAISE(ABORT, 'quota evidence requires a subscription call');
END;

CREATE TRIGGER model_call_quota_evidence_next_revision
BEFORE INSERT ON model_call_quota_evidence_revisions
FOR EACH ROW
WHEN NEW.evidence_revision <> COALESCE((
	SELECT MAX(evidence_revision)+1 FROM model_call_quota_evidence_revisions
	WHERE task_id=NEW.task_id AND call_id=NEW.call_id
), 1)
BEGIN
	SELECT RAISE(ABORT, 'quota evidence revision must append by one');
END;

CREATE TRIGGER model_call_quota_evidence_no_complete_regression
BEFORE INSERT ON model_call_quota_evidence_revisions
FOR EACH ROW
WHEN COALESCE((
	SELECT evidence_status FROM model_call_quota_evidence_revisions
	WHERE task_id=NEW.task_id AND call_id=NEW.call_id
	ORDER BY evidence_revision DESC LIMIT 1
), '') = 'complete' AND NEW.evidence_status <> 'complete'
BEGIN
	SELECT RAISE(ABORT, 'complete quota evidence cannot regress');
END;

CREATE TRIGGER model_call_quota_evidence_final_bound
BEFORE INSERT ON model_call_quota_evidence_revisions
FOR EACH ROW
WHEN COALESCE((
	SELECT quota_report_status FROM model_calls
	WHERE task_id=NEW.task_id AND call_id=NEW.call_id
), '') IN ('final_pending','final_reported')
BEGIN
	SELECT RAISE(ABORT, 'final-bound quota evidence is immutable');
END;

CREATE TRIGGER model_call_quota_evidence_immutable_update
BEFORE UPDATE ON model_call_quota_evidence_revisions
BEGIN
	SELECT RAISE(ABORT, 'quota evidence revisions are immutable');
END;

CREATE TRIGGER model_call_quota_evidence_immutable_delete
BEFORE DELETE ON model_call_quota_evidence_revisions
BEGIN
	SELECT RAISE(ABORT, 'quota evidence revisions are immutable');
END;

CREATE VIEW model_call_quota_evidence_current AS
SELECT r.*
FROM model_call_quota_evidence_revisions r
WHERE r.evidence_revision=(
	SELECT MAX(latest.evidence_revision)
	FROM model_call_quota_evidence_revisions latest
	WHERE latest.task_id=r.task_id AND latest.call_id=r.call_id
);
`
