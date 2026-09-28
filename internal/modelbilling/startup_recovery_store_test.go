package modelbilling

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestSQLiteTaskStoreStartupRecoveryIDsUseCompleteScope(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	base := TaskScope{ActorID: "local", ProfileID: "default"}
	first := base
	first.CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "usr-a", BindingGeneration: 1}
	otherGeneration := base
	otherGeneration.CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "usr-a", BindingGeneration: 2}
	otherUser := base
	otherUser.CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "usr-b", BindingGeneration: 1}
	byok := base

	for id, scope := range map[string]TaskScope{
		"task-first": first, "task-generation": otherGeneration, "task-user": otherUser, "task-byok": byok,
	} {
		persistStartupRecoveryTask(t, store, id, scope)
	}

	ids, err := store.ListStartupRecoveryIDs(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []string{"task-first"}) {
		t.Fatalf("subscription IDs = %v", ids)
	}
	ids, err = store.ListStartupRecoveryIDs(context.Background(), otherGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []string{"task-generation"}) {
		t.Fatalf("generation IDs = %v", ids)
	}
	ids, err = store.ListStartupRecoveryIDs(context.Background(), byok)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []string{"task-byok"}) {
		t.Fatalf("BYOK IDs = %v", ids)
	}
}

func TestSQLiteTaskStoreQuarantineStartupTaskIsScopeSafeAndPreservesStartedAt(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	scope := TaskScope{ActorID: "local", ProfileID: "default"}
	persistStartupRecoveryTask(t, store, "task-created-corrupt", scope)
	persistStartupRecoveryTask(t, store, "task-running-corrupt", scope)

	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: scope, TaskID: "task-running-corrupt", From: TaskCreated, To: TaskReady, At: storeNow().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: scope, TaskID: "task-running-corrupt", From: TaskReady, To: TaskRunning, At: storeNow().Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	var started string
	if err := db.QueryRow(`SELECT started_at FROM model_tasks WHERE task_id='task-running-corrupt'`).Scan(&started); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET actor_id='conflicting-actor' WHERE task_id IN ('task-created-corrupt','task-running-corrupt')`); err != nil {
		t.Fatal(err)
	}

	ids, err := store.ListStartupRecoveryIDs(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []string{"task-created-corrupt", "task-running-corrupt"}) {
		t.Fatalf("startup IDs = %v", ids)
	}
	now := storeNow().Add(3 * time.Minute)
	for _, id := range ids {
		if err := store.QuarantineStartupTask(context.Background(), scope, id, now, "local_restart_interrupted"); err != nil {
			t.Fatalf("quarantine %s: %v", id, err)
		}
	}

	assertRawStartupRecoveryTask(t, db, "task-created-corrupt", TaskFailed, sql.NullString{String: string(TaskResultFailed), Valid: true}, sql.NullString{}, sql.NullString{String: stamp(now), Valid: true})
	assertRawStartupRecoveryTask(t, db, "task-running-corrupt", TaskRecoveryRequired, sql.NullString{}, sql.NullString{String: started, Valid: true}, sql.NullString{})
	ids, err = store.ListStartupRecoveryIDs(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("second scan IDs = %v", ids)
	}

	other := TaskScope{ActorID: "other", ProfileID: "default"}
	if err := store.QuarantineStartupTask(context.Background(), other, "task-created-corrupt", now, "local_restart_interrupted"); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("cross-scope quarantine error = %v", err)
	}
}

func TestSQLiteTaskStoreStartupRecoveryFindsPersistedIdentityConflictsByScopeKey(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	subscription := TaskScope{ActorID: "local", ProfileID: "default"}
	subscription.CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "usr-conflict", BindingGeneration: 3}
	byok := TaskScope{ActorID: "local", ProfileID: "default"}
	otherGeneration := subscription
	otherGeneration.CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "usr-conflict", BindingGeneration: 4}
	for id, scope := range map[string]TaskScope{
		"task-sub-source-conflict":  subscription,
		"task-sub-cloud-conflict":   subscription,
		"task-byok-source-conflict": byok,
		"task-other-generation":     otherGeneration,
	} {
		persistStartupRecoveryTask(t, store, id, scope)
	}
	// Keep each row structurally valid while making its persisted identity
	// disagree with the immutable scope_key written at task creation.
	if _, err := db.Exec(`UPDATE model_tasks
		SET billing_source=?,cloud_user_id=NULL,binding_generation=NULL
		WHERE task_id='task-sub-source-conflict'`, BillingNewAPIByok); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks
		SET billing_source=?,cloud_user_id='usr-injected',binding_generation=9
		WHERE task_id='task-byok-source-conflict'`, BillingSubscriptionPool); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks
		SET cloud_user_id='usr-other',binding_generation=8
		WHERE task_id='task-sub-cloud-conflict'`); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		scope TaskScope
		ids   []string
	}{
		{scope: subscription, ids: []string{"task-sub-cloud-conflict", "task-sub-source-conflict"}},
		{scope: byok, ids: []string{"task-byok-source-conflict"}},
	} {
		ids, err := store.ListStartupRecoveryIDs(context.Background(), test.scope)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, test.ids) {
			t.Fatalf("scope %+v startup IDs = %v", test.scope, ids)
		}
		for _, id := range ids {
			if err := store.QuarantineStartupTask(context.Background(), test.scope, id, storeNow().Add(time.Minute), "local_restart_interrupted"); err != nil {
				t.Fatalf("quarantine identity conflict %s: %v", id, err)
			}
		}
	}
	var otherState TaskState
	if err := db.QueryRow(`SELECT state FROM model_tasks WHERE task_id='task-other-generation'`).Scan(&otherState); err != nil {
		t.Fatal(err)
	}
	if otherState != TaskCreated {
		t.Fatalf("other generation state = %s", otherState)
	}
}

func TestSQLiteTaskStoreQuarantineStartupTaskKeepsLifecycleTimeMonotonic(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	scope := TaskScope{ActorID: "local", ProfileID: "default"}
	persistStartupRecoveryTask(t, store, "task-clock-created", scope)
	persistStartupRecoveryTask(t, store, "task-clock-running", scope)
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: scope, TaskID: "task-clock-running", From: TaskCreated, To: TaskReady, At: storeNow().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	started := storeNow().Add(2 * time.Minute)
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: scope, TaskID: "task-clock-running", From: TaskReady, To: TaskRunning, At: started,
	}); err != nil {
		t.Fatal(err)
	}
	ended := storeNow().Add(3 * time.Minute)
	if _, err := db.Exec(`UPDATE model_tasks SET ended_at=? WHERE task_id='task-clock-running'`, stamp(ended)); err != nil {
		t.Fatal(err)
	}
	rolledBackClock := storeNow().Add(-time.Hour)
	for _, id := range []string{"task-clock-created", "task-clock-running"} {
		if err := store.QuarantineStartupTask(context.Background(), scope, id, rolledBackClock, "local_restart_interrupted"); err != nil {
			t.Fatalf("quarantine %s: %v", id, err)
		}
	}
	var createdUpdated, createdEnded, runningStarted, runningUpdated, runningEnded string
	if err := db.QueryRow(`SELECT updated_at,ended_at FROM model_tasks WHERE task_id='task-clock-created'`).Scan(&createdUpdated, &createdEnded); err != nil {
		t.Fatal(err)
	}
	if createdUpdated != stamp(storeNow()) || createdEnded != stamp(storeNow()) {
		t.Fatalf("created times = updated:%s ended:%s", createdUpdated, createdEnded)
	}
	if err := db.QueryRow(`SELECT started_at,updated_at,ended_at FROM model_tasks WHERE task_id='task-clock-running'`).Scan(&runningStarted, &runningUpdated, &runningEnded); err != nil {
		t.Fatal(err)
	}
	if runningStarted != stamp(started) || runningUpdated != stamp(ended) || runningEnded != stamp(ended) {
		t.Fatalf("running times = started:%s updated:%s ended:%s", runningStarted, runningUpdated, runningEnded)
	}
}

func persistStartupRecoveryTask(t *testing.T, store *SQLiteTaskStore, id string, scope TaskScope) {
	t.Helper()
	source := BillingNewAPIByok
	if scope.CloudBinding != nil {
		source = BillingSubscriptionPool
	}
	task, evidence := storeTask(source, id)
	task.Scope = scope
	evidence.Scope = scope
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatalf("persist %s: %v", id, err)
	}
}

func assertRawStartupRecoveryTask(t *testing.T, db *sql.DB, id string, wantState TaskState, wantResult, wantStarted, wantEnded sql.NullString) {
	t.Helper()
	var state TaskState
	var result, started, ended, code sql.NullString
	if err := db.QueryRow(`SELECT state,task_result,started_at,ended_at,last_error_code FROM model_tasks WHERE task_id=?`, id).Scan(&state, &result, &started, &ended, &code); err != nil {
		t.Fatal(err)
	}
	if state != wantState || result != wantResult || started != wantStarted || ended != wantEnded || !code.Valid || code.String != "local_restart_interrupted" {
		t.Fatalf("raw %s = state=%s result=%v started=%v ended=%v code=%v", id, state, result, started, ended, code)
	}
}
