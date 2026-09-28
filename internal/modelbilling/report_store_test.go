package modelbilling

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestSQLiteReportStoreFinalUsesQuotaVersionHashAndImmutablePayload(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_final", true, true, "request_final")
	store := NewSQLiteReportStore(db)
	build, first, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if build.Disposition != ReportBuildDeliverable || first == nil || first.ContractVersion != QuotaContractV1 ||
		first.PayloadHash != quotaPayloadHash(first.CanonicalPayload) {
		t.Fatalf("final enqueue = %+v / %+v", build, first)
	}
	var version, hash string
	if err := db.QueryRow(`SELECT contract_version,payload_hash FROM usage_reports WHERE idempotency_key=?`, first.IdempotencyKey).Scan(&version, &hash); err != nil {
		t.Fatal(err)
	}
	if version != QuotaContractV1 || hash != first.PayloadHash {
		t.Fatalf("persisted version/hash = %q/%q", version, hash)
	}

	// Token telemetry is no longer part of the subscription settlement payload.
	if _, err := db.Exec(`UPDATE model_calls SET normalized_usage_json=?,raw_usage_redacted_json=? WHERE task_id=?`,
		[]byte(`{"request_count":999,"breakdown_status":"exact"}`), []byte(`{"request_count":999}`), task.TaskID); err != nil {
		t.Fatal(err)
	}
	_, repeated, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil || repeated == nil || !bytes.Equal(repeated.CanonicalPayload, first.CanonicalPayload) || repeated.PayloadHash != first.PayloadHash {
		t.Fatalf("token-only retry changed final = %+v, %v", repeated, err)
	}

	evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint: MaasEndpointChatCompletions, Headers: map[string][]string{"X-Oneapi-Request-Id": {"request_final"}},
		FinalJSON: []byte(`{"billing":{"quota":9}}`), ObservedAt: storeNow().Add(6 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence.Revision = 2
	if err := NewSQLiteQuotaEvidenceStore(db).Append(context.Background(), task.Scope, task.TaskID, "call_"+task.TaskID, evidence); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("final-bound evidence mutation error = %v", err)
	}
}

func TestSQLiteReportStoreFinalEvidencePendingDoesNotInsertRow(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_evidence", true, false, "")
	finalEnqueueAt := storeNow().Add(13 * time.Minute)
	clockAt := task.UpdatedAt
	store := NewSQLiteReportStoreWithClock(db, func() time.Time { return clockAt })
	build, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil || build.Disposition != ReportBuildEvidencePending || pending != nil {
		t.Fatalf("evidence pending = %+v / %+v / %v", build, pending, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM usage_reports WHERE task_id=?`, task.TaskID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("evidence-pending rows = %d, %v", count, err)
	}
	workflow := quotaStoreWorkflow(t, db, task)
	if workflow.Status != QuotaWorkflowEvidencePending {
		t.Fatalf("workflow = %+v", workflow)
	}

	supplementingAt := storeNow().Add(11 * time.Minute)
	if err := NewSQLiteQuotaWorkflowStore(db).Update(context.Background(), QuotaWorkflowUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: QuotaWorkflowEvidencePending, To: QuotaWorkflowSupplementing, At: supplementingAt,
	}); err != nil {
		t.Fatal(err)
	}
	supplementQuotaEvidence(t, db, task, "call_"+task.TaskID, 2, "request_supplement", "3", storeNow().Add(12*time.Minute))
	clockAt = finalEnqueueAt
	build, pending, err = store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil || build.Disposition != ReportBuildDeliverable || pending == nil {
		t.Fatalf("supplemented final = %+v / %+v / %v", build, pending, err)
	}
	workflow = quotaStoreWorkflow(t, db, task)
	if workflow.Status != QuotaWorkflowFinalUploadPending || !workflow.UpdatedAt.Equal(finalEnqueueAt) ||
		!pending.CreatedAt.Equal(*task.EndedAt) {
		t.Fatalf("supplemented workflow = %+v", workflow)
	}
}

func TestSQLiteReportStorePartialIsCumulativeAndAcceptsIncompleteEvidence(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_partial", false, false, "")
	capturedAt := storeNow().Add(6 * time.Minute)
	store := NewSQLiteReportStoreWithClock(db, func() time.Time { return capturedAt })
	firstBuild, first, err := store.EnqueuePartial(context.Background(), task.Scope, task.TaskID)
	if err != nil || first == nil || first.Sequence != 1 || firstBuild.Partial == nil {
		t.Fatalf("first partial = %+v / %+v / %v", firstBuild, first, err)
	}
	if firstBuild.Partial.Calls[0].EvidenceStatus != QuotaEvidenceMissing {
		t.Fatalf("incomplete partial call = %+v", firstBuild.Partial.Calls[0])
	}
	now := first.CreatedAt.Add(time.Minute)
	claimed, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("claim first partial = %+v, %v, %v", claimed, ok, err)
	}
	if err := store.MarkAccepted(context.Background(), task.Scope, first.IdempotencyKey, claimed.AttemptCount, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if status := quotaStoreCallStatus(t, db, task.TaskID); status != QuotaReportPartialReported {
		t.Fatalf("partial accepted call status = %q", status)
	}

	supplementQuotaEvidence(t, db, task, "call_"+task.TaskID, 2, "request_partial", "4.5", now.Add(2*time.Minute))
	capturedAt = now.Add(3 * time.Minute)
	secondBuild, second, err := store.EnqueuePartial(context.Background(), task.Scope, task.TaskID)
	if err != nil || second == nil || second.Sequence != 2 || secondBuild.Partial == nil ||
		secondBuild.Partial.Calls[0].EvidenceRevision != 2 || secondBuild.Partial.Calls[0].EvidenceStatus != QuotaEvidenceComplete {
		t.Fatalf("second partial = %+v / %+v / %v", secondBuild, second, err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	removed := *secondBuild.Partial
	removed.Sequence = 3
	removed.CapturedAt = removed.CapturedAt.Add(time.Second)
	removed.Calls = nil
	if err := validateCumulativePartial(context.Background(), tx, task, removed); ErrorCodeOf(err) != ErrorQuotaPartialSnapshotConflict {
		t.Fatalf("removed call error = %v", err)
	}
	regressed := *secondBuild.Partial
	regressed.Sequence = 3
	regressed.CapturedAt = regressed.CapturedAt.Add(time.Second)
	regressed.Calls = append([]QuotaReportCall(nil), regressed.Calls...)
	regressed.Calls[0].EvidenceRevision = 1
	if err := validateCumulativePartial(context.Background(), tx, task, regressed); ErrorCodeOf(err) != ErrorQuotaPartialSnapshotConflict {
		t.Fatalf("revision regression error = %v", err)
	}
	changedComplete := *secondBuild.Partial
	changedComplete.Sequence = 3
	changedComplete.CapturedAt = changedComplete.CapturedAt.Add(time.Second)
	changedComplete.Calls = append([]QuotaReportCall(nil), changedComplete.Calls...)
	replacement := quotaReportBuilderEvidence(t, "request_partial", "5", changedComplete.CapturedAt)
	changedComplete.Calls[0].BillingQuotaRaw = replacement.BillingQuotaRaw
	changedComplete.Calls[0].UsageQuotaRaw = replacement.UsageQuotaRaw
	changedComplete.Calls[0].Quota = replacement.Quota
	changedComplete.Calls[0].QuotaSource = replacement.Source
	changedComplete.Calls[0].EvidenceStatus = replacement.Status
	changedComplete.Calls[0].EvidenceIssues = replacement.Issues
	changedComplete.Calls[0].EvidenceRevision++
	if err := validateCumulativePartial(context.Background(), tx, task, changedComplete); err != nil {
		t.Fatalf("higher complete evidence revision error = %v", err)
	}
}

func TestSQLiteReportStorePartialExcludesPersistedInFlightCall(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_in_flight", false, true, "request_completed")
	calls := NewSQLiteCallStore(db)
	taskContext := storeContext(t, task)
	running := ModelCall{
		CallID: "call_running", TaskID: task.TaskID, Scope: task.Scope, BillingSource: BillingSubscriptionPool,
		Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", Endpoint: MaasEndpointChatCompletions,
		PricingEvidenceID: task.PricingEvidenceID, State: CallPrepared,
	}
	if err := calls.Prepare(context.Background(), taskContext, running); err != nil {
		t.Fatal(err)
	}
	if err := calls.BeginAttempt(context.Background(), taskContext, running.CallID, storeNow().Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}

	capturedAt := storeNow().Add(7 * time.Minute)
	built, pending, err := NewSQLiteReportStoreWithClock(db, func() time.Time { return capturedAt }).
		EnqueuePartial(context.Background(), task.Scope, task.TaskID)
	if err != nil || pending == nil || built.Partial == nil {
		t.Fatalf("partial with persisted in-flight call = %+v / %+v / %v", built, pending, err)
	}
	if len(built.Partial.Calls) != 1 || built.Partial.Calls[0].CallID != "call_"+task.TaskID {
		t.Fatalf("partial calls = %+v", built.Partial.Calls)
	}
	var linked int
	if err := db.QueryRow(`SELECT COUNT(*) FROM usage_report_calls WHERE idempotency_key=?`, pending.IdempotencyKey).Scan(&linked); err != nil || linked != 1 {
		t.Fatalf("partial call links = %d, %v", linked, err)
	}
}

func TestSQLiteReportStoreFinalDeliveryMaintainsCallAndWorkflowState(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_delivery", true, true, "request_delivery")
	store := NewSQLiteReportStore(db)
	_, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if quotaStoreCallStatus(t, db, task.TaskID) != QuotaReportFinalPending ||
		quotaStoreWorkflow(t, db, task).Status != QuotaWorkflowFinalUploadPending {
		t.Fatal("final enqueue did not atomically mark call/workflow pending")
	}
	now := quotaStoreWorkflow(t, db, task).UpdatedAt.Add(time.Minute)
	first, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute))
	if err != nil || !ok || first.AttemptCount != 1 || quotaStoreWorkflow(t, db, task).Status != QuotaWorkflowFinalUploading {
		t.Fatalf("first claim = %+v, %v, %v", first, ok, err)
	}
	retryAt := now.Add(3 * time.Minute)
	if err := store.MarkRetry(context.Background(), task.Scope, pending.IdempotencyKey, 1, retryAt, "quota_retryable", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	workflow := quotaStoreWorkflow(t, db, task)
	if workflow.Status != QuotaWorkflowRetryPending || workflow.RetryStage == nil || *workflow.RetryStage != QuotaRetryFinalUpload ||
		quotaStoreCallError(t, db, task.TaskID) != "quota_retryable" {
		t.Fatalf("retry facts = %+v / %q", workflow, quotaStoreCallError(t, db, task.TaskID))
	}
	second, ok, err := store.ClaimDue(context.Background(), task.Scope, retryAt, retryAt.Add(time.Minute))
	if err != nil || !ok || second.AttemptCount != 2 {
		t.Fatalf("second claim = %+v, %v, %v", second, ok, err)
	}
	if err := store.MarkAccepted(context.Background(), task.Scope, pending.IdempotencyKey, 2, retryAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if quotaStoreCallStatus(t, db, task.TaskID) != QuotaReportFinalReported || quotaStoreCallError(t, db, task.TaskID) != "" ||
		quotaStoreWorkflow(t, db, task).Status != QuotaWorkflowSettlementPending {
		t.Fatal("final acceptance did not atomically mark call/workflow")
	}
}

func TestSQLiteReportStoreLegacyRowsAreAuditableButNeverDue(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_legacy", true, true, "request_legacy")
	key, err := BuildFinalIdempotencyKey(*task.Scope.CloudBinding, task.TaskID, task.Freeze.FreezeID)
	if err != nil {
		t.Fatal(err)
	}
	created := reportQueueStamp(*task.EndedAt)
	if _, err := db.Exec(`INSERT INTO usage_reports
		(idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,next_attempt_at,created_at,updated_at,contract_version)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, key, task.TaskID, ReportFinal, 0, []byte(`{"legacy":true}`),
		ReportPending, 0, created, created, created, LegacyTokenContractV1); err != nil {
		t.Fatal(err)
	}
	store := NewSQLiteReportStore(db)
	got, found, err := store.GetByIdempotencyKey(context.Background(), task.Scope, key)
	if err != nil || !found || got.ContractVersion != LegacyTokenContractV1 || got.PayloadHash != "" {
		t.Fatalf("legacy metadata = %+v, %v, %v", got, found, err)
	}
	if _, ok, err := store.ClaimDue(context.Background(), task.Scope, task.EndedAt.Add(time.Hour), task.EndedAt.Add(2*time.Hour)); err != nil || ok {
		t.Fatalf("legacy report became due = %v, %v", ok, err)
	}
}

func TestSQLiteReportStoreRejectsAcceptedRequestIDReuse(t *testing.T) {
	db := openSchemaTestDB(t)
	firstTask := createQuotaReportStoreTask(t, db, "task_quota_store_reuse_a", true, true, "shared_request")
	store := NewSQLiteReportStore(db)
	_, first, err := store.EnqueueFinal(context.Background(), firstTask.Scope, firstTask.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	now := quotaStoreWorkflow(t, db, firstTask).UpdatedAt.Add(time.Minute)
	claimed, ok, err := store.ClaimDue(context.Background(), firstTask.Scope, now, now.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("claim first = %+v, %v, %v", claimed, ok, err)
	}
	if err := store.MarkAccepted(context.Background(), firstTask.Scope, first.IdempotencyKey, claimed.AttemptCount, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	secondTask := createQuotaReportStoreTask(t, db, "task_quota_store_reuse_b", true, true, "shared_request")
	if _, _, err := store.EnqueueFinal(context.Background(), secondTask.Scope, secondTask.TaskID); ErrorCodeOf(err) != ErrorQuotaRequestIDReused {
		t.Fatalf("request id reuse error = %v", err)
	}
}

func TestSQLiteReportStoreBackfillsAcceptedQuotaRequestIDs(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_backfill", true, true, "request_backfill")
	store := NewSQLiteReportStore(db)
	_, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	now := quotaStoreWorkflow(t, db, task).UpdatedAt.Add(time.Minute)
	claimed, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("claim accepted final = %+v, %v, %v", claimed, ok, err)
	}
	if err := store.MarkAccepted(context.Background(), task.Scope, pending.IdempotencyKey, claimed.AttemptCount, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	registry := &fixtureAcceptedRequestRegistry{owners: make(map[string]string)}
	cloudUserID := task.Scope.CloudBinding.CloudUserID
	if err := store.BackfillAcceptedQuotaRequestIDs(context.Background(), cloudUserID, registry); err != nil {
		t.Fatal(err)
	}
	if owner := registry.owners[cloudUserID+"\x00request_backfill"]; owner != task.TaskID {
		t.Fatalf("backfilled owner = %q, want %q", owner, task.TaskID)
	}
	if err := store.BackfillAcceptedQuotaRequestIDs(context.Background(), cloudUserID, registry); err != nil {
		t.Fatalf("idempotent backfill = %v", err)
	}
}

func TestSQLiteReportStoreSkipsAcceptedQuotaBackfillBeforeV4(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy-v3.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(migrationTableSQL); err != nil {
		t.Fatal(err)
	}
	for version := SchemaVersionV1; version <= SchemaVersionV3; version++ {
		if _, err := db.Exec(`INSERT INTO model_billing_schema_migrations (version,applied_at) VALUES (?,?)`, version, "2026-07-15T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}

	registry := &fixtureAcceptedRequestRegistry{owners: make(map[string]string)}
	if err := NewSQLiteReportStore(db).BackfillAcceptedQuotaRequestIDs(context.Background(), "legacy-cloud-user", registry); err != nil {
		t.Fatalf("pre-V4 accepted quota backfill = %v, want skipped", err)
	}
	if len(registry.owners) != 0 {
		t.Fatalf("pre-V4 accepted quota backfill wrote registry entries: %+v", registry.owners)
	}
}

func TestSQLiteReportStoreScopesAcceptedRequestIDReuseToCloudUser(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteReportStore(db)
	firstScope := TaskScope{ActorID: "local", ProfileID: "profile-a", CloudBinding: &cloud.BindingScope{
		ProfileID: "profile-a", CloudUserID: "shared-user", BindingGeneration: 1,
	}}
	firstTask := createQuotaReportStoreTaskInScope(t, db, "task_quota_store_cloud_user_a", firstScope, true, true, "shared_request")
	_, first, err := store.EnqueueFinal(context.Background(), firstTask.Scope, firstTask.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	now := quotaStoreWorkflow(t, db, firstTask).UpdatedAt.Add(time.Minute)
	claimed, ok, err := store.ClaimDue(context.Background(), firstTask.Scope, now, now.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("claim first = %+v, %v, %v", claimed, ok, err)
	}
	if err := store.MarkAccepted(context.Background(), firstTask.Scope, first.IdempotencyKey, claimed.AttemptCount, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name      string
		taskID    string
		scope     TaskScope
		wantReuse bool
	}{
		{
			name: "different profile same cloud user", taskID: "task_quota_store_cloud_user_b",
			scope: TaskScope{ActorID: "local", ProfileID: "profile-b", CloudBinding: &cloud.BindingScope{
				ProfileID: "profile-b", CloudUserID: "shared-user", BindingGeneration: 1,
			}}, wantReuse: true,
		},
		{
			name: "same profile newer binding generation", taskID: "task_quota_store_cloud_user_generation",
			scope: TaskScope{ActorID: "local", ProfileID: "profile-a", CloudBinding: &cloud.BindingScope{
				ProfileID: "profile-a", CloudUserID: "shared-user", BindingGeneration: 2,
			}}, wantReuse: true,
		},
		{
			name: "different cloud user", taskID: "task_quota_store_other_cloud_user",
			scope: TaskScope{ActorID: "local", ProfileID: "profile-b", CloudBinding: &cloud.BindingScope{
				ProfileID: "profile-b", CloudUserID: "other-user", BindingGeneration: 1,
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			task := createQuotaReportStoreTaskInScope(t, db, test.taskID, test.scope, true, true, "shared_request")
			_, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
			if test.wantReuse {
				if ErrorCodeOf(err) != ErrorQuotaRequestIDReused || pending != nil {
					t.Fatalf("request id reuse = %+v, %v", pending, err)
				}
				return
			}
			if err != nil || pending == nil {
				t.Fatalf("different cloud user enqueue = %+v, %v", pending, err)
			}
		})
	}
}

func TestSQLiteFinalQuotaReportCorrectorRebuildsOnlyCurrentDeliveringAttempt(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_correct", true, true, "request_correct")
	store := NewSQLiteReportStore(db)
	_, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	now := quotaStoreWorkflow(t, db, task).UpdatedAt.Add(time.Minute)
	claimed, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("claim = %+v, %v, %v", claimed, ok, err)
	}
	var invalid FinalQuotaReport
	if err := json.Unmarshal(claimed.CanonicalPayload, &invalid); err != nil {
		t.Fatal(err)
	}
	invalid.ClientSummary.TaskTotalQuota = "999"
	invalidPayload, err := CanonicalJSON(invalid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE usage_reports SET canonical_payload=?,payload_hash=? WHERE idempotency_key=?`,
		[]byte(invalidPayload), quotaPayloadHash(invalidPayload), pending.IdempotencyKey); err != nil {
		t.Fatal(err)
	}
	corrector := NewSQLiteFinalQuotaReportCorrector(db)
	corrected, err := corrector.CorrectUnacceptedFinal(context.Background(), task.Scope, pending.IdempotencyKey,
		claimed.AttemptCount, ErrorQuotaSummaryMismatch, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if corrected.State != ReportPending || corrected.AttemptCount != claimed.AttemptCount ||
		corrected.PayloadHash != quotaPayloadHash(corrected.CanonicalPayload) ||
		quotaStoreWorkflow(t, db, task).Status != QuotaWorkflowFinalUploadPending {
		t.Fatalf("corrected final = %+v", corrected)
	}
	before := append([]byte(nil), corrected.CanonicalPayload...)
	if _, err := corrector.CorrectUnacceptedFinal(context.Background(), task.Scope, pending.IdempotencyKey,
		claimed.AttemptCount, ErrorQuotaSummaryMismatch, now.Add(2*time.Second)); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("stale correction error = %v", err)
	}
	after, found, err := store.GetByIdempotencyKey(context.Background(), task.Scope, pending.IdempotencyKey)
	if err != nil || !found || !bytes.Equal(before, after.CanonicalPayload) {
		t.Fatalf("stale correction mutated bytes = %+v, %v", after, err)
	}
}

func TestSQLiteReportStoreHashPoisonIsIsolatedAndConcurrentClaimIsUnique(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_poison", true, true, "request_poison")
	store := NewSQLiteReportStore(db)
	_, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE usage_reports SET canonical_payload=? WHERE idempotency_key=?`, []byte(`{}`), pending.IdempotencyKey); err != nil {
		t.Fatal(err)
	}
	now := quotaStoreWorkflow(t, db, task).UpdatedAt.Add(time.Minute)
	if _, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute)); !ok || ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("poison claim = %v, %v", ok, err)
	}
	var state ReportState
	if err := db.QueryRow(`SELECT state FROM usage_reports WHERE idempotency_key=?`, pending.IdempotencyKey).Scan(&state); err != nil || state != ReportBlocked {
		t.Fatalf("poison state = %q, %v", state, err)
	}

	other := createQuotaReportStoreTask(t, db, "task_quota_store_concurrent", true, true, "request_concurrent")
	_, _, err = store.EnqueueFinal(context.Background(), other.Scope, other.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		ok  bool
		err error
	}
	results := make(chan result, 2)
	otherNow := quotaStoreWorkflow(t, db, other).UpdatedAt.Add(time.Minute)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			_, ok, err := store.ClaimDue(context.Background(), other.Scope, otherNow, otherNow.Add(time.Minute))
			results <- result{ok: ok, err: err}
		}()
	}
	start.Done()
	claimedCount := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.ok {
			claimedCount++
		}
	}
	if claimedCount != 1 {
		t.Fatalf("concurrent claims = %d", claimedCount)
	}
}

func TestSQLiteReportStoreLatePartialCannotTouchFinalBoundCall(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_late_partial", true, true, "request_late_partial")
	store := NewSQLiteReportStoreWithClock(db, func() time.Time { return storeNow().Add(11 * time.Minute) })
	_, partial, err := store.EnqueuePartial(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	now := partial.CreatedAt.Add(time.Minute)
	claimed, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute))
	if err != nil || !ok || claimed.IdempotencyKey != partial.IdempotencyKey {
		t.Fatalf("claim partial = %+v, %v, %v", claimed, ok, err)
	}
	if _, _, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID); err != nil {
		t.Fatal(err)
	}
	finalClaim, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute))
	if err != nil || !ok || finalClaim.Kind != ReportFinal {
		t.Fatalf("claim final = %+v, %v, %v", finalClaim, ok, err)
	}
	if err := store.MarkBlocked(context.Background(), task.Scope, finalClaim.IdempotencyKey,
		finalClaim.AttemptCount, "final_error", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	retryAt := now.Add(2 * time.Minute)
	if err := store.MarkRetry(context.Background(), task.Scope, partial.IdempotencyKey, claimed.AttemptCount,
		retryAt, "partial_error", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if status, code := quotaStoreCallStatus(t, db, task.TaskID), quotaStoreCallError(t, db, task.TaskID); status != QuotaReportFinalPending || code != "final_error" {
		t.Fatalf("partial retry changed final-bound call = %q/%q", status, code)
	}
	claimed, ok, err = store.ClaimDue(context.Background(), task.Scope, retryAt, retryAt.Add(time.Minute))
	if err != nil || !ok || claimed.IdempotencyKey != partial.IdempotencyKey {
		t.Fatalf("reclaim partial = %+v, %v, %v", claimed, ok, err)
	}
	if err := store.MarkAccepted(context.Background(), task.Scope, partial.IdempotencyKey,
		claimed.AttemptCount, retryAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if status, code := quotaStoreCallStatus(t, db, task.TaskID), quotaStoreCallError(t, db, task.TaskID); status != QuotaReportFinalPending || code != "final_error" {
		t.Fatalf("partial acceptance changed final-bound call = %q/%q", status, code)
	}
}

func TestSQLiteReportStoreRejectsPartialCaptureBeforeLoadedFacts(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_early_capture", false, true, "request_early_capture")
	store := NewSQLiteReportStoreWithClock(db, func() time.Time { return storeNow().Add(4 * time.Minute) })

	if _, pending, err := store.EnqueuePartial(context.Background(), task.Scope, task.TaskID); ErrorCodeOf(err) != ErrorQuotaTimeInvalid || pending != nil {
		t.Fatalf("early partial capture = %+v, %v", pending, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM usage_reports WHERE task_id=?`, task.TaskID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("early capture report rows = %d, %v", count, err)
	}
}

func TestSQLiteReportStoreRetryVerifiesCallLinksAndUsesTaskStart(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_retry_integrity", true, true, "request_integrity")
	store := NewSQLiteReportStore(db)
	_, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM usage_report_calls WHERE idempotency_key=?`, pending.IdempotencyKey); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("missing report call link error = %v", err)
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls(idempotency_key,task_id,call_id) VALUES (?,?,?)`,
		pending.IdempotencyKey, task.TaskID, "call_"+task.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET created_at=? WHERE task_id=?`,
		stamp(task.CreatedAt.Add(-time.Minute)), task.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.GetByIdempotencyKey(context.Background(), task.Scope, pending.IdempotencyKey); err != nil || !found {
		t.Fatalf("historical created_at changed final identity = %v, %v", found, err)
	}
}

func TestSQLiteReportStorePoisonIsolationSurvivesWorkflowDrift(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_quota_store_workflow_drift", true, true, "request_workflow_drift")
	store := NewSQLiteReportStore(db)
	_, pending, err := store.EnqueueFinal(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE usage_reports SET canonical_payload='{}' WHERE idempotency_key=?`, pending.IdempotencyKey); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_task_quota_workflow SET status=?,retry_stage=NULL WHERE task_id=?`,
		QuotaWorkflowSettlementPending, task.TaskID); err != nil {
		t.Fatal(err)
	}
	now := quotaStoreWorkflow(t, db, task).UpdatedAt.Add(time.Minute)
	if _, ok, err := store.ClaimDue(context.Background(), task.Scope, now, now.Add(time.Minute)); !ok || ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("poison with workflow drift = %v, %v", ok, err)
	}
	var state ReportState
	if err := db.QueryRow(`SELECT state FROM usage_reports WHERE idempotency_key=?`, pending.IdempotencyKey).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != ReportBlocked {
		t.Fatalf("poison report state = %q", state)
	}
}

func createQuotaReportStoreTask(t *testing.T, db *sql.DB, taskID string, ended, complete bool, requestID string) ModelTask {
	return createQuotaReportStoreTaskInScope(t, db, taskID, testTaskScope(BillingSubscriptionPool), ended, complete, requestID)
}

func createQuotaReportStoreTaskInScope(t *testing.T, db *sql.DB, taskID string, scope TaskScope, ended, complete bool, requestID string) ModelTask {
	t.Helper()
	task, evidence := storeTask(BillingSubscriptionPool, taskID)
	task.Scope = scope
	evidence.Scope = scope
	tasks := NewSQLiteTaskStore(db)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	taskContext := storeContext(t, task)
	if err := tasks.RecordFreezeRequest(context.Background(), taskContext, freezeRequest(task), storeNow().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tasks.RecordFreezeResult(context.Background(), taskContext, activeFreeze(task),
		&TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}, storeNow().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tasks.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: storeNow().Add(3 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	addQuotaReportStoreCall(t, db, task, evidence, "call_"+taskID, complete, requestID, "1.25", 4)
	if ended {
		result := TaskResultCompleted
		if err := tasks.UpdateState(context.Background(), TaskStateUpdate{
			Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskUsagePending,
			Result: &result, At: storeNow().Add(10 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := tasks.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func addQuotaReportStoreCall(t *testing.T, db *sql.DB, task ModelTask, pricing PricingEvidence, callID string, complete bool, requestID, quota string, minute int) {
	t.Helper()
	store := NewSQLiteCallStore(db)
	taskContext := storeContext(t, task)
	call := ModelCall{
		CallID: callID, TaskID: task.TaskID, Scope: task.Scope, BillingSource: BillingSubscriptionPool,
		Role: CallRoleMain, ModelID: pricing.ModelID, Provider: "fixture", Endpoint: MaasEndpointChatCompletions,
		PricingEvidenceID: pricing.EvidenceID, State: CallPrepared,
	}
	if err := store.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	startedAt := storeNow().Add(time.Duration(minute) * time.Minute)
	if err := store.BeginAttempt(context.Background(), taskContext, callID, startedAt); err != nil {
		t.Fatal(err)
	}
	finishedAt := startedAt.Add(time.Minute)
	headers := map[string][]string(nil)
	payload := json.RawMessage(`{}`)
	if complete {
		headers = map[string][]string{"X-Oneapi-Request-Id": {requestID}}
		payload = json.RawMessage(fmt.Sprintf(`{"billing":{"quota":%s}}`, quota))
	}
	quotaEvidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint: MaasEndpointChatCompletions, Headers: headers, FinalJSON: payload, ObservedAt: finishedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	requestCount := int64(1)
	call.State, call.FinishedAt, call.QuotaEvidence = CallCompleted, &finishedAt, &quotaEvidence
	call.Usage = NormalizedUsage{RequestCount: &requestCount, BreakdownStatus: UsageExact}
	if err := store.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
}

func supplementQuotaEvidence(t *testing.T, db *sql.DB, task ModelTask, callID string, revision uint64, requestID, quota string, at time.Time) {
	t.Helper()
	evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:  MaasEndpointChatCompletions,
		Headers:   map[string][]string{"X-Oneapi-Request-Id": {requestID}},
		FinalJSON: json.RawMessage(fmt.Sprintf(`{"billing":{"quota":%s}}`, quota)), ObservedAt: at,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence.Revision = revision
	if err := NewSQLiteQuotaEvidenceStore(db).Append(context.Background(), task.Scope, task.TaskID, callID, evidence); err != nil {
		t.Fatal(err)
	}
}

func quotaStoreWorkflow(t *testing.T, db *sql.DB, task ModelTask) QuotaWorkflow {
	t.Helper()
	workflow, err := NewSQLiteQuotaWorkflowStore(db).Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	return workflow
}

func quotaStoreCallStatus(t *testing.T, db *sql.DB, taskID string) QuotaReportStatus {
	t.Helper()
	var status QuotaReportStatus
	if err := db.QueryRow(`SELECT quota_report_status FROM model_calls WHERE task_id=?`, taskID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func quotaStoreCallError(t *testing.T, db *sql.DB, taskID string) string {
	t.Helper()
	var value sql.NullString
	if err := db.QueryRow(`SELECT quota_report_last_error_code FROM model_calls WHERE task_id=?`, taskID).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value.String
}
