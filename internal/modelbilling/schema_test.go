package modelbilling

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openSchemaTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenSQLite(t.TempDir() + "/model-billing.sqlite")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}
	return db
}

func TestOpenSQLiteSerializesWriteTransactionsAtBegin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-billing.sqlite")
	first, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if err := ApplyMigrations(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })

	blocker, err := first.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	if _, err := blocker.Exec(`UPDATE model_billing_schema_migrations SET applied_at=applied_at WHERE version=?`, SchemaVersionV1); err != nil {
		t.Fatal(err)
	}
	readOnly, err := second.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("read-only transaction was blocked by writer reservation: %v", err)
	}
	if err := readOnly.Rollback(); err != nil {
		t.Fatal(err)
	}

	attempting := make(chan struct{})
	began := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(attempting)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tx, err := second.BeginTx(ctx, nil)
		if err != nil {
			result <- err
			return
		}
		close(began)
		if _, err = tx.ExecContext(ctx, `UPDATE model_billing_schema_migrations SET applied_at=applied_at WHERE version=?`, SchemaVersionV1); err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		result <- err
	}()
	<-attempting
	beganBeforeRelease := false
	select {
	case <-began:
		beganBeforeRelease = true
	case <-time.After(100 * time.Millisecond):
	}
	if err := blocker.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("serialized writer failed: %v", err)
	}
	if beganBeforeRelease {
		t.Fatal("second write transaction began before the first writer released its reservation")
	}
}

func insertSchemaTask(t *testing.T, db *sql.DB, taskID string, source BillingSource) {
	t.Helper()
	scope := TaskScope{ActorID: "local", ProfileID: "default"}
	var cloudUser any
	var generation any
	if source == BillingSubscriptionPool {
		cloudUser = "usr_" + taskID
		generation = 1
		binding := testBindingScope()
		binding.CloudUserID = cloudUser.(string)
		scope.CloudBinding = &binding
	}
	scopeKey, err := BuildTaskScopeKey(scope, source)
	if err != nil {
		t.Fatalf("BuildTaskScopeKey: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin task/evidence transaction: %v", err)
	}
	defer tx.Rollback()
	evidenceID := schemaEvidenceID(taskID)
	_, err = tx.Exec(`INSERT INTO model_tasks (
		task_id, scope_key, actor_id, profile_id, cloud_user_id, binding_generation,
		billing_source, pricing_evidence_id, state, created_at, updated_at
	) VALUES (?, ?, 'local', 'default', ?, ?, ?, ?, 'created', ?, ?)`,
		taskID, scopeKey, cloudUser, generation, source, evidenceID,
		"2026-07-11T00:00:00Z", "2026-07-11T00:00:00Z")
	if err != nil {
		t.Fatalf("insert task %s: %v", taskID, err)
	}
	if _, err := tx.Exec(`INSERT INTO pricing_evidence
		(evidence_id, snapshot_id, task_id, model_id, pricing_group, state, captured_at)
		VALUES (?, 'snapshot_01', ?, 'model_01', 'default', 'fresh', ?)`,
		evidenceID, taskID, "2026-07-11T00:00:00Z"); err != nil {
		t.Fatalf("insert initial evidence %s: %v", taskID, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit task/evidence %s: %v", taskID, err)
	}
}

func schemaEvidenceID(taskID string) string { return "price_" + taskID }

func TestApplyMigrationsCreatesB0SchemaAndIsRepeatable(t *testing.T) {
	db := openSchemaTestDB(t)
	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations repeat: %v", err)
	}
	var version int
	if err := db.QueryRow(`SELECT max(version) FROM model_billing_schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if version != SchemaVersionV6 {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersionV6)
	}
	for _, table := range []string{
		"model_tasks", "model_calls", "usage_reports", "usage_report_calls", "pricing_evidence",
		"model_call_quota_evidence_revisions", "model_task_quota_workflow",
		"model_task_quota_settlement_revisions",
	} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s count = %d, err %v", table, count, err)
		}
	}
}

func TestSchemaRestrictsAndKeepsTaskStartedAtImmutable(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_started_at_not_running", BillingNewAPIByok)
	if _, err := db.Exec(`UPDATE model_tasks SET started_at=? WHERE task_id='task_started_at_not_running'`,
		stamp(storeNow().Add(time.Minute))); err == nil {
		t.Fatal("schema accepted started_at outside Ready -> Running")
	}

	task, evidence := storeTask(BillingNewAPIByok, "task_started_at_immutable")
	store := NewSQLiteTaskStore(db)
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskCreated, To: TaskReady, At: storeNow().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: storeNow().Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{
		"clear":   nil,
		"rewrite": stamp(storeNow().Add(3 * time.Minute)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.Exec(`UPDATE model_tasks SET started_at=? WHERE task_id=?`, value, task.TaskID); err == nil {
				t.Fatal("schema accepted immutable started_at change")
			}
		})
	}
}

func TestSchemaAcceptsGuardianRole(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_guardian", BillingSubscriptionPool)
	if _, err := db.Exec(`INSERT INTO model_calls
		(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
		VALUES ('call_guardian', 'task_guardian', ?, 'model_01', 'fixture', ?, 'prepared')`,
		CallRoleGuardian, schemaEvidenceID("task_guardian")); err != nil {
		t.Fatalf("insert guardian call: %v", err)
	}
}

func TestOpenSQLiteEnablesForeignKeysOnEveryConnection(t *testing.T) {
	db := openSchemaTestDB(t)
	first, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("first Conn: %v", err)
	}
	defer first.Close()
	second, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("second Conn: %v", err)
	}
	defer second.Close()
	for index, conn := range []*sql.Conn{first, second} {
		var enabled int
		if err := conn.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&enabled); err != nil || enabled != 1 {
			t.Fatalf("connection %d foreign_keys = %d, %v", index, enabled, err)
		}
	}
}

func TestSchemaDerivesChildScopeAndBillingSourceFromTask(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_sub", BillingSubscriptionPool)
	if _, err := db.Exec(`UPDATE model_tasks SET scope_key = 'readable_scope' WHERE task_id = 'task_sub'`); err == nil {
		t.Fatal("schema accepted a readable caller-defined scope_key")
	}

	for _, table := range []string{"model_calls", "usage_reports", "pricing_evidence"} {
		columns := schemaColumns(t, db, table)
		for _, redundant := range []string{"scope_key", "actor_id", "profile_id", "cloud_user_id", "binding_generation", "billing_source"} {
			if columns[redundant] {
				t.Fatalf("%s retains uncheckable child column %s", table, redundant)
			}
		}
	}
	if _, err := db.Exec(`INSERT INTO model_calls
		(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
		VALUES ('call_missing', 'missing_task', 'main', 'model_01', 'fixture', 'price_missing', 'prepared')`); err == nil {
		t.Fatal("schema accepted a call without its parent task")
	}
	if _, err := db.Exec(`INSERT INTO pricing_evidence
		(evidence_id, snapshot_id, task_id, model_id, pricing_group, state, captured_at)
		VALUES ('evidence_missing', 'snapshot_01', 'missing_task', 'model_01', 'default', 'fresh', ?)`,
		"2026-07-11T00:00:00Z"); err == nil {
		t.Fatal("schema accepted pricing evidence without its parent task/scope")
	}
}

func TestSchemaRejectsCrossTaskReportCallAndBYOKUsage(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_sub", BillingSubscriptionPool)
	insertSchemaTask(t, db, "task_other", BillingSubscriptionPool)
	insertSchemaTask(t, db, "task_byok", BillingNewAPIByok)
	for _, row := range []struct{ callID, taskID string }{
		{callID: "call_sub", taskID: "task_sub"},
		{callID: "call_other", taskID: "task_other"},
		{callID: "call_byok", taskID: "task_byok"},
	} {
		if _, err := db.Exec(`INSERT INTO model_calls
			(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
			VALUES (?, ?, 'main', 'model_01', 'fixture', ?, 'completed')`, row.callID, row.taskID, schemaEvidenceID(row.taskID)); err != nil {
			t.Fatalf("insert call %s: %v", row.callID, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO usage_reports
		(idempotency_key, task_id, report_kind, sequence, canonical_payload, state, next_attempt_at, created_at, updated_at)
		VALUES ('report_sub', 'task_sub', 'partial', 1, '{}', 'pending', ?, ?, ?)`,
		"2026-07-11T00:01:00Z", "2026-07-11T00:00:00Z", "2026-07-11T00:00:00Z"); err != nil {
		t.Fatalf("insert subscription report: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO usage_reports
		(idempotency_key, task_id, report_kind, sequence, canonical_payload, state, next_attempt_at, created_at, updated_at)
		VALUES ('report_byok', 'task_byok', 'partial', 1, '{}', 'pending', ?, ?, ?)`,
		"2026-07-11T00:01:00Z", "2026-07-11T00:00:00Z", "2026-07-11T00:00:00Z"); err == nil {
		t.Fatal("schema accepted BYOK task in subscription usage_reports")
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls (idempotency_key, task_id, call_id)
		VALUES ('report_sub', 'task_sub', 'call_sub')`); err != nil {
		t.Fatalf("insert matching report call: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls (idempotency_key, task_id, call_id)
		VALUES ('report_sub', 'task_sub', 'call_other')`); err == nil {
		t.Fatal("schema accepted report linked to a call from another task/scope")
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls (idempotency_key, task_id, call_id)
		VALUES ('report_sub', 'task_sub', 'call_byok')`); err == nil {
		t.Fatal("schema accepted BYOK call in subscription usage report")
	}
}

func TestSchemaEnforcesEnumsBindingsAndRetryTime(t *testing.T) {
	db := openSchemaTestDB(t)
	scopeKey, err := BuildTaskScopeKey(testTaskScope(BillingSubscriptionPool), BillingSubscriptionPool)
	if err != nil {
		t.Fatalf("BuildTaskScopeKey: %v", err)
	}
	badTask := `INSERT INTO model_tasks (
		task_id, scope_key, actor_id, profile_id, cloud_user_id, binding_generation,
		billing_source, low_balance_mode, pricing_evidence_id, state, created_at, updated_at
	) VALUES (?, ?, 'local', 'default', ?, ?, ?, ?, 'price_01', ?, ?, ?)`
	for name, args := range map[string][]any{
		"zero generation": {"bad_generation", "usr", 0, BillingSubscriptionPool, nil, TaskCreated},
		"low balance":     {"bad_low_balance", "usr", 1, BillingSubscriptionPool, 2, TaskCreated},
		"billing source":  {"bad_source", nil, nil, "other", nil, TaskCreated},
		"task state":      {"bad_state", "usr", 1, BillingSubscriptionPool, nil, "unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			values := append([]any{args[0], scopeKey}, args[1:]...)
			values = append(values, "2026-07-11T00:00:00Z", "2026-07-11T00:00:00Z")
			if _, err := db.Exec(badTask, values...); err == nil {
				t.Fatal("schema accepted invalid task")
			}
		})
	}
	insertSchemaTask(t, db, "task_sub", BillingSubscriptionPool)
	for _, item := range []struct{ name, role, state string }{
		{name: "role", role: "unknown", state: "prepared"},
		{name: "state", role: "main", state: "unknown"},
	} {
		t.Run("call "+item.name, func(t *testing.T) {
			if _, err := db.Exec(`INSERT INTO model_calls
				(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
				VALUES (?, 'task_sub', ?, 'model_01', 'fixture', ?, ?)`, "bad_call_"+item.name, item.role, schemaEvidenceID("task_sub"), item.state); err == nil {
				t.Fatal("schema accepted invalid call enum")
			}
		})
	}
	insertReport := `INSERT INTO usage_reports
		(idempotency_key, task_id, report_kind, sequence, canonical_payload, state, next_attempt_at, created_at, updated_at)
		VALUES (?, 'task_sub', 'partial', ?, '{}', ?, ?, ?, ?)`
	now := "2026-07-11T00:00:00Z"
	for _, row := range []struct {
		key      string
		sequence int
		state    string
		next     any
	}{
		{key: "bad_report_state", sequence: 1, state: "unknown", next: now},
		{key: "pending_without_next", sequence: 2, state: "pending", next: nil},
		{key: "accepted_with_next", sequence: 3, state: "accepted", next: now},
	} {
		if _, err := db.Exec(insertReport, row.key, row.sequence, row.state, row.next, now, now); err == nil {
			t.Fatalf("schema accepted invalid report %s", row.key)
		}
	}
	if _, err := db.Exec(`INSERT INTO pricing_evidence
		(evidence_id, snapshot_id, task_id, model_id, pricing_group, state, captured_at)
		VALUES ('bad_pricing', 'snapshot_01', 'task_sub', 'model_01', 'default', 'unknown', ?)`, now); err == nil {
		t.Fatal("schema accepted invalid pricing state")
	}
}

func TestSchemaPersistsTaskResultAcrossSubscriptionAndBYOK(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_sub", BillingSubscriptionPool)
	insertSchemaTask(t, db, "task_byok", BillingNewAPIByok)
	if _, err := db.Exec(`UPDATE model_tasks SET state = 'failed', task_result = 'failed' WHERE task_id = 'task_sub'`); err != nil {
		t.Fatalf("persist failed result: %v", err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET state = 'usage_pending' WHERE task_id = 'task_sub'`); err != nil {
		t.Fatalf("preserve failed result in usage_pending: %v", err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET state = 'settling' WHERE task_id = 'task_sub'`); err != nil {
		t.Fatalf("preserve failed result in settling: %v", err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET state = 'completed' WHERE task_id = 'task_sub'`); err != nil {
		t.Fatalf("preserve failed result in completed: %v", err)
	}
	var result string
	if err := db.QueryRow(`SELECT task_result FROM model_tasks WHERE task_id = 'task_sub'`).Scan(&result); err != nil || result != "failed" {
		t.Fatalf("persisted task_result = %q, %v", result, err)
	}
	for name, statement := range map[string]string{
		"active result":  `UPDATE model_tasks SET state = 'running', task_result = 'completed' WHERE task_id = 'task_byok'`,
		"failed missing": `UPDATE model_tasks SET state = 'failed', task_result = NULL WHERE task_id = 'task_byok'`,
		"BYOK mismatch":  `UPDATE model_tasks SET state = 'completed', task_result = 'failed' WHERE task_id = 'task_byok'`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.Exec(statement); err == nil {
				t.Fatal("schema accepted invalid task result/state")
			}
		})
	}
}

func TestSchemaBindsTaskAndCallsToPricingEvidence(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_a", BillingSubscriptionPool)
	insertSchemaTask(t, db, "task_b", BillingSubscriptionPool)
	scopeKey, err := BuildTaskScopeKey(testTaskScope(BillingSubscriptionPool), BillingSubscriptionPool)
	if err != nil {
		t.Fatalf("BuildTaskScopeKey: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin missing evidence transaction: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO model_tasks
		(task_id, scope_key, actor_id, profile_id, cloud_user_id, binding_generation, billing_source,
		 pricing_evidence_id, state, created_at, updated_at)
		VALUES ('task_missing_evidence', ?, 'local', 'default', 'usr_fixture', 1, 'subscription_pool',
		 'missing_evidence', 'created', ?, ?)`, scopeKey, "2026-07-11T00:00:00Z", "2026-07-11T00:00:00Z"); err != nil {
		t.Fatalf("deferred task insert: %v", err)
	}
	if err := tx.Commit(); err == nil {
		t.Fatal("schema committed task without initial pricing evidence")
	}

	if _, err := db.Exec(`UPDATE model_tasks SET pricing_evidence_id = 'missing_evidence' WHERE task_id = 'task_b'`); err == nil {
		t.Fatal("schema accepted task referencing missing pricing evidence")
	}
	if _, err := db.Exec(`UPDATE model_tasks SET pricing_evidence_id = ? WHERE task_id = 'task_b'`, schemaEvidenceID("task_a")); err == nil {
		t.Fatal("schema accepted task referencing another task/scope evidence")
	}
	insertCall := `INSERT INTO model_calls
		(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
		VALUES (?, 'task_b', 'main', 'model_01', 'fixture', ?, 'prepared')`
	if _, err := db.Exec(insertCall, "call_missing_evidence", "missing_evidence"); err == nil {
		t.Fatal("schema accepted call referencing missing pricing evidence")
	}
	if _, err := db.Exec(insertCall, "call_cross_task", schemaEvidenceID("task_a")); err == nil {
		t.Fatal("schema accepted call referencing another task/scope evidence")
	}
	if _, err := db.Exec(insertCall, "call_valid", schemaEvidenceID("task_b")); err != nil {
		t.Fatalf("insert call with same-task evidence: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pricing_evidence
		(evidence_id, snapshot_id, task_id, model_id, pricing_group, state, captured_at)
		VALUES ('price_task_b_model_02', 'snapshot_02', 'task_b', 'model_02', 'default', 'fresh', ?)`,
		"2026-07-11T00:01:00Z"); err != nil {
		t.Fatalf("insert second-model evidence: %v", err)
	}
	insertModelTwoCall := `INSERT INTO model_calls
		(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
		VALUES (?, 'task_b', 'subagent', 'model_02', 'fixture', ?, 'prepared')`
	if _, err := db.Exec(insertModelTwoCall, "call_model_02", "price_task_b_model_02"); err != nil {
		t.Fatalf("insert second-model call with matching evidence: %v", err)
	}
	if _, err := db.Exec(insertModelTwoCall, "call_model_02_wrong_evidence", schemaEvidenceID("task_b")); err == nil {
		t.Fatal("schema accepted same-task evidence for a different model")
	}
}

func TestSchemaEnforcesReportSequenceAndIdempotency(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_01", BillingSubscriptionPool)
	insertReport := `INSERT INTO usage_reports (
		idempotency_key, task_id, report_kind, sequence, canonical_payload,
		state, next_attempt_at, attempt_count, created_at, updated_at
	) VALUES (?, 'task_01', ?, ?, ?, 'pending', ?, 0, ?, ?)`
	now := "2026-07-11T00:00:00Z"
	if _, err := db.Exec(insertReport, "partial_1", ReportPartial, 1, []byte(`{"sequence":1}`), now, now, now); err != nil {
		t.Fatalf("insert partial: %v", err)
	}
	if _, err := db.Exec(insertReport, "partial_0", ReportPartial, 0, []byte(`{}`), now, now, now); err == nil {
		t.Fatal("schema accepted partial sequence 0")
	}
	if _, err := db.Exec(insertReport, "final_1", ReportFinal, 1, []byte(`{}`), now, now, now); err == nil {
		t.Fatal("schema accepted final sequence 1")
	}
	if _, err := db.Exec(insertReport, "partial_duplicate", ReportPartial, 1, []byte(`{"sequence":1}`), now, now, now); err == nil {
		t.Fatal("schema accepted duplicate task/report_kind/sequence")
	}
	if _, err := db.Exec(insertReport, "partial_1", ReportPartial, 2, []byte(`{"sequence":2}`), now, now, now); err == nil {
		t.Fatal("schema accepted duplicate idempotency key")
	}
}

func TestSchemaKeepsReportVersionAndAcceptedFactsImmutable(t *testing.T) {
	db := openSchemaTestDB(t)
	insertSchemaTask(t, db, "task_immutable_report", BillingSubscriptionPool)
	for _, callID := range []string{"call_immutable_a", "call_immutable_b"} {
		if _, err := db.Exec(`INSERT INTO model_calls
			(call_id,task_id,role,model_id,provider,pricing_evidence_id,state)
			VALUES (?,'task_immutable_report','main','model_01','fixture',?,'completed')`,
			callID, schemaEvidenceID("task_immutable_report")); err != nil {
			t.Fatal(err)
		}
	}
	now := "2026-07-15T00:00:00Z"
	if _, err := db.Exec(`INSERT INTO usage_reports
		(idempotency_key,task_id,report_kind,sequence,canonical_payload,state,next_attempt_at,created_at,updated_at)
		VALUES ('report_immutable','task_immutable_report','partial',1,'{}','pending',?,?,?)`, now, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE usage_reports SET contract_version='quota-contract-v1',payload_hash=?
		WHERE idempotency_key='report_immutable'`, strings.Repeat("0", 64)); err == nil {
		t.Fatal("schema accepted a report contract version rewrite")
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls(idempotency_key,task_id,call_id)
		VALUES ('report_immutable','task_immutable_report','call_immutable_a')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE usage_reports SET state='accepted',next_attempt_at=NULL
		WHERE idempotency_key='report_immutable'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE usage_reports SET canonical_payload='{"changed":true}'
		WHERE idempotency_key='report_immutable'`); err == nil {
		t.Fatal("schema rewrote an accepted report")
	}
	if _, err := db.Exec(`DELETE FROM usage_report_calls
		WHERE idempotency_key='report_immutable' AND call_id='call_immutable_a'`); err == nil {
		t.Fatal("schema deleted an accepted report call link")
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls(idempotency_key,task_id,call_id)
		VALUES ('report_immutable','task_immutable_report','call_immutable_b')`); err == nil {
		t.Fatal("schema added an accepted report call link")
	}
	if _, err := db.Exec(`DELETE FROM usage_reports WHERE idempotency_key='report_immutable'`); err == nil {
		t.Fatal("schema deleted an accepted report")
	}
}

func schemaColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table_info %s: %v", table, err)
		}
		columns[name] = true
	}
	return columns
}

func TestReportIdempotencyKeysAreStableScopedAndOpaque(t *testing.T) {
	scope := testBindingScope()
	first, err := BuildPartialIdempotencyKey(scope, "task_01", "frz_01", 1)
	if err != nil {
		t.Fatalf("BuildPartialIdempotencyKey: %v", err)
	}
	repeat, err := BuildPartialIdempotencyKey(scope, "task_01", "frz_01", 1)
	if err != nil || repeat != first {
		t.Fatalf("repeat key = %q, %v; want %q", repeat, err, first)
	}
	second, err := BuildPartialIdempotencyKey(scope, "task_01", "frz_01", 2)
	if err != nil || second == first {
		t.Fatalf("sequence key = %q, %v; want distinct", second, err)
	}
	final, err := BuildFinalIdempotencyKey(scope, "task_01", "frz_01")
	if err != nil || final == first {
		t.Fatalf("final key = %q, %v; want distinct", final, err)
	}
	for _, raw := range []string{scope.ProfileID, scope.CloudUserID, "task_01", "frz_01"} {
		if strings.Contains(first, raw) || strings.Contains(final, raw) {
			t.Fatalf("idempotency key exposes %q: %q / %q", raw, first, final)
		}
	}
}
