package modelbilling

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestInsertPricingEvidenceCommitsWithTaskInCallerTransaction(t *testing.T) {
	db := openSchemaTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	owner, evidence := storeTestEvidence("task_01", "evidence_01")
	insertStoreTestTask(t, tx, owner, evidence.EvidenceID)
	if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); err != nil {
		t.Fatalf("InsertPricingEvidence: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var taskCount, evidenceCount int
	if err := db.QueryRow(`SELECT count(*) FROM model_tasks WHERE task_id = ?`, owner.TaskID).Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pricing_evidence WHERE evidence_id = ?`, evidence.EvidenceID).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 1 || evidenceCount != 1 {
		t.Fatalf("persisted task=%d evidence=%d", taskCount, evidenceCount)
	}
}

func TestInsertPricingEvidencePersistsBYOKUnavailableEvidenceAndReferences(t *testing.T) {
	db := openSchemaTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	owner, evidence := storeTestEvidence("task_unavailable", "evidence_unavailable")
	evidence.SnapshotID = "pricing-unavailable-v1:domestic"
	evidence.PricingVersion = nil
	evidence.PricingHash = nil
	evidence.State = PricingUnavailable
	insertStoreTestTask(t, tx, owner, evidence.EvidenceID)
	if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); err != nil {
		t.Fatalf("InsertPricingEvidence: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO model_calls
		(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
		VALUES ('call_unavailable', ?, 'main', ?, 'fixture', ?, 'prepared')`,
		owner.TaskID, evidence.ModelID, evidence.EvidenceID); err != nil {
		t.Fatalf("insert call referencing unavailable evidence: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit unavailable evidence: %v", err)
	}

	var version, hash sql.NullString
	var state, snapshotID string
	if err := db.QueryRow(`SELECT snapshot_id, pricing_version, pricing_hash, state
		FROM pricing_evidence WHERE evidence_id = ?`, evidence.EvidenceID).Scan(&snapshotID, &version, &hash, &state); err != nil {
		t.Fatal(err)
	}
	if snapshotID != evidence.SnapshotID || version.Valid || hash.Valid || state != string(PricingUnavailable) {
		t.Fatalf("persisted unavailable evidence snapshot=%q version=%+v hash=%+v state=%q", snapshotID, version, hash, state)
	}
	if _, err := db.Exec(`INSERT INTO model_calls
		(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
		VALUES ('call_cross_model', ?, 'main', 'other_model', 'fixture', ?, 'prepared')`,
		owner.TaskID, evidence.EvidenceID); err == nil {
		t.Fatal("schema accepted unavailable evidence for another model")
	}
	insertSchemaTask(t, db, "task_other", BillingNewAPIByok)
	if _, err := db.Exec(`INSERT INTO model_calls
		(call_id, task_id, role, model_id, provider, pricing_evidence_id, state)
		VALUES ('call_cross_task', 'task_other', 'main', ?, 'fixture', ?, 'prepared')`,
		evidence.ModelID, evidence.EvidenceID); err == nil {
		t.Fatal("schema accepted unavailable evidence for another task")
	}
}

func TestInsertPricingEvidenceIsImmutableAndIdempotent(t *testing.T) {
	db := openSchemaTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	owner, evidence := storeTestEvidence("task_01", "evidence_01")
	insertStoreTestTask(t, tx, owner, evidence.EvidenceID)
	if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); err != nil {
		t.Fatal(err)
	}
	if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); err != nil {
		t.Fatalf("identical insert: %v", err)
	}
	changed := evidence
	changed.PricingGroup = "other"
	if err := InsertPricingEvidence(context.Background(), tx, owner, changed); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("changed insert error = %v", err)
	}
}

func TestInsertPricingEvidenceRollbackLeavesNoRow(t *testing.T) {
	db := openSchemaTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	owner, evidence := storeTestEvidence("task_01", "evidence_01")
	insertStoreTestTask(t, tx, owner, evidence.EvidenceID)
	if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var taskCount, evidenceCount int
	if err := db.QueryRow(`SELECT count(*) FROM model_tasks WHERE task_id = ?`, owner.TaskID).Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pricing_evidence WHERE evidence_id = ?`, evidence.EvidenceID).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 0 || evidenceCount != 0 {
		t.Fatalf("rows after rollback: task=%d evidence=%d", taskCount, evidenceCount)
	}
}

func TestInsertPricingEvidenceRejectsNilTransaction(t *testing.T) {
	owner, evidence := storeTestEvidence("task_01", "evidence_01")
	if err := InsertPricingEvidence(context.Background(), nil, owner, evidence); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("nil transaction error = %v", err)
	}
}

func TestInsertPricingEvidenceRequiresTaskFirstInCallerTransaction(t *testing.T) {
	db := openSchemaTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	owner, evidence := storeTestEvidence("task_01", "evidence_01")
	if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("evidence-first error = %v", err)
	}
}

func TestInsertPricingEvidenceDerivesScopeAndBillingSourceFromTask(t *testing.T) {
	db := openSchemaTestDB(t)
	owner, evidence := storeTestEvidenceForSource("task_01", "evidence_01", BillingSubscriptionPool)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	insertStoreTestTask(t, tx, owner, evidence.EvidenceID)
	if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	t.Run("scope_mismatch", func(t *testing.T) {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		changedOwner := owner
		changedOwner.Scope.ActorID = "other_actor"
		changed := evidence
		changed.Scope = changedOwner.Scope
		if err := InsertPricingEvidence(context.Background(), tx, changedOwner, changed); ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("scope mismatch error = %v", err)
		}
	})

	t.Run("billing_source_mismatch", func(t *testing.T) {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		changedOwner := owner
		changedOwner.BillingSource = BillingNewAPIByok
		changedOwner.Scope = testTaskScope(BillingNewAPIByok)
		changed := evidence
		changed.Scope = changedOwner.Scope
		if err := InsertPricingEvidence(context.Background(), tx, changedOwner, changed); ErrorCodeOf(err) != ErrorBillingSourceMismatch {
			t.Fatalf("billing source mismatch error = %v", err)
		}
	})

	t.Run("identical", func(t *testing.T) {
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err := InsertPricingEvidence(context.Background(), tx, owner, evidence); err != nil {
			t.Fatalf("identical insert: %v", err)
		}
	})
}

func TestInsertPricingEvidenceDoesNotMaskUnrelatedSQLError(t *testing.T) {
	db := openSchemaTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	owner, evidence := storeTestEvidence("task_01", "evidence_01")
	err = InsertPricingEvidence(context.Background(), tx, owner, evidence)
	if err == nil || ErrorCodeOf(err) == ErrorIdempotencyConflict {
		t.Fatalf("closed transaction error = %v", err)
	}
}

func storeTestEvidence(taskID, evidenceID string) (PricingEvidenceOwner, PricingEvidence) {
	return storeTestEvidenceForSource(taskID, evidenceID, BillingNewAPIByok)
}

func storeTestEvidenceForSource(taskID, evidenceID string, source BillingSource) (PricingEvidenceOwner, PricingEvidence) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	version := "v1"
	hash := "sha256:test"
	scope := testTaskScope(source)
	owner := PricingEvidenceOwner{TaskID: taskID, Scope: scope, BillingSource: source}
	evidence := PricingEvidence{
		EvidenceID: evidenceID, SnapshotID: "snapshot_01", Scope: scope, TaskID: taskID,
		ModelID: "model_01", PricingGroup: "default", PricingVersion: &version,
		PricingHash: &hash, State: PricingFresh, CapturedAt: now,
	}
	return owner, evidence
}

func insertStoreTestTask(t *testing.T, tx *sql.Tx, owner PricingEvidenceOwner, evidenceID string) {
	t.Helper()
	scopeKey, err := BuildTaskScopeKey(owner.Scope, owner.BillingSource)
	if err != nil {
		t.Fatal(err)
	}
	var cloudUserID any
	var bindingGeneration any
	if owner.Scope.CloudBinding != nil {
		cloudUserID = owner.Scope.CloudBinding.CloudUserID
		bindingGeneration = owner.Scope.CloudBinding.BindingGeneration
	}
	if _, err := tx.Exec(`INSERT INTO model_tasks (
		task_id, scope_key, actor_id, profile_id, cloud_user_id, binding_generation, billing_source, pricing_evidence_id,
		state, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'created', ?, ?)`, owner.TaskID, scopeKey,
		owner.Scope.ActorID, owner.Scope.ProfileID, cloudUserID, bindingGeneration, owner.BillingSource, evidenceID,
		"2026-07-12T00:00:00Z", "2026-07-12T00:00:00Z"); err != nil {
		t.Fatalf("insert task: %v", err)
	}
}
