package modelbilling

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestSQLiteQuotaSettlementStorePersistsIdempotentRevisionHistory(t *testing.T) {
	db := openSchemaTestDB(t)
	task := settlementStoreTask(t, db, "task_settlement_store")
	store := NewSQLiteQuotaSettlementStore(db)
	pending := QuotaSettlementResult{Settlement: QuotaSettlement{
		SettlementID: "set-store", TaskID: task.TaskID, Status: QuotaSettlementPending,
	}}
	firstAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, pending, firstAt); err != nil {
		t.Fatalf("save pending: %v", err)
	}
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, pending, firstAt.Add(time.Minute)); err != nil {
		t.Fatalf("exact pending retry: %v", err)
	}
	changedPending := pending
	changedPending.Settlement.TaskTotalQuota = settlementDecimal("50")
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, changedPending, firstAt.Add(time.Minute)); err != nil {
		t.Fatalf("pending enrichment = %v", err)
	}

	terminal := QuotaSettlementResult{Settlement: validQuotaSettlement()}
	terminal.Settlement.TaskID = task.TaskID
	terminal.Settlement.SettlementID = "set-store"
	invalidTerminal := terminal
	invalidTerminal.Settlement.ReleasedFrozenFreeAmount = settlementDecimal("285.3")
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, invalidTerminal, firstAt.Add(2*time.Minute)); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("invalid task-bound settlement = %v", err)
	}
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, terminal, firstAt.Add(-time.Minute)); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("regressed observation time = %v", err)
	}
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, terminal, firstAt.Add(2*time.Minute)); err != nil {
		t.Fatalf("save terminal: %v", err)
	}
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, terminal, firstAt.Add(3*time.Minute)); err != nil {
		t.Fatalf("exact terminal retry: %v", err)
	}

	restarted := NewSQLiteQuotaSettlementStore(db)
	current, found, err := restarted.CurrentQuotaSettlement(context.Background(), task.Scope, task.TaskID)
	if err != nil || !found {
		t.Fatalf("current after restart = %+v, %v, %v", current, found, err)
	}
	if current.Settlement.Status != QuotaSettlementSettled || current.Settlement.SettlementID != "set-store" {
		t.Fatalf("current settlement = %+v", current.Settlement)
	}
	if current.Settlement.QuotaPerUnit == nil || *current.Settlement.QuotaPerUnit != "500000" ||
		current.Settlement.ConsumedFrozenFreeAmount == nil || *current.Settlement.ConsumedFrozenFreeAmount != "1.46" ||
		current.Settlement.Balance == nil || *current.Settlement.Balance != "20998.54" ||
		current.Settlement.BalanceVersion == nil || *current.Settlement.BalanceVersion != 14 {
		t.Fatalf("current settlement lost normalized ledger facts = %+v", current.Settlement)
	}
	history, err := restarted.ListQuotaSettlements(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 || history[0].Revision != 1 || history[1].Revision != 2 || history[2].Revision != 3 ||
		history[0].Result.Settlement.Status != QuotaSettlementPending ||
		history[1].Result.Settlement.Status != QuotaSettlementPending ||
		history[1].Result.Settlement.TaskTotalQuota == nil ||
		history[2].Result.Settlement.Status != QuotaSettlementSettled {
		t.Fatalf("history = %+v", history)
	}

	conflict := terminal
	conflict.Settlement.Balance = settlementDecimal("20985.5")
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, conflict, firstAt.Add(4*time.Minute)); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("changed terminal retry = %v", err)
	}
}

func TestSQLiteQuotaSettlementStoreRejectsMutationAndWrongScope(t *testing.T) {
	db := openSchemaTestDB(t)
	task := settlementStoreTask(t, db, "task_settlement_store_scope")
	store := NewSQLiteQuotaSettlementStore(db)
	result := QuotaSettlementResult{Settlement: QuotaSettlement{
		SettlementID: "set-scope", TaskID: task.TaskID, Status: QuotaSettlementManualRequired,
		ErrorCode: settlementString("MANUAL_REVIEW_REQUIRED"),
	}}
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, result, storeNow()); err != nil {
		t.Fatal(err)
	}
	wrong := task.Scope
	binding := *task.Scope.CloudBinding
	wrong.CloudBinding = &binding
	wrong.CloudBinding.BindingGeneration++
	if _, _, err := store.CurrentQuotaSettlement(context.Background(), wrong, task.TaskID); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("wrong scope = %v", err)
	}
	if _, err := db.Exec(`UPDATE model_task_quota_settlement_revisions SET status='settled' WHERE task_id=?`, task.TaskID); err == nil {
		t.Fatal("immutable settlement revision update succeeded")
	}
}

func TestSQLiteQuotaSettlementStorePendingBalanceVersionMustAdvance(t *testing.T) {
	db := openSchemaTestDB(t)
	task := settlementStoreTask(t, db, "task_settlement_pending_balance_version")
	store := NewSQLiteQuotaSettlementStore(db)
	firstAt := time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)
	pending := pendingSettlementWithBalance(task.TaskID, "set-pending-balance", "10", 14)
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, pending, firstAt); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, pending, firstAt.Add(-time.Minute)); err != nil {
		t.Fatalf("exact payload retry = %v", err)
	}

	sameVersion := pendingSettlementWithBalance(task.TaskID, "set-pending-balance", "11", 14)
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, sameVersion, firstAt.Add(time.Minute)); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("same-version changed balance = %v", err)
	}
	lowerVersion := pendingSettlementWithBalance(task.TaskID, "set-pending-balance", "9", 13)
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, lowerVersion, firstAt.Add(time.Minute)); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("lower balance version = %v", err)
	}

	higherVersion := pendingSettlementWithBalance(task.TaskID, "set-pending-balance", "12", 15)
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, higherVersion, firstAt.Add(time.Minute)); err != nil {
		t.Fatalf("higher balance version = %v", err)
	}
	omitted := QuotaSettlementResult{Settlement: QuotaSettlement{
		SettlementID: "set-pending-balance", TaskID: task.TaskID, Status: QuotaSettlementPending,
		TaskTotalQuota: settlementDecimal("50"),
	}}
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, omitted, firstAt.Add(2*time.Minute)); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("omitted confirmed balance = %v", err)
	}
}

func TestSQLiteQuotaSettlementStoreTerminalBalanceVersionCannotRegress(t *testing.T) {
	for _, test := range []struct {
		name         string
		version      uint64
		wantConflict bool
	}{
		{name: "same", version: 14, wantConflict: true},
		{name: "lower", version: 13, wantConflict: true},
		{name: "higher", version: 15},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openSchemaTestDB(t)
			task := settlementStoreTask(t, db, "task_settlement_terminal_balance_"+test.name)
			store := NewSQLiteQuotaSettlementStore(db)
			firstAt := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)
			pending := pendingSettlementWithBalance(task.TaskID, "set-terminal-balance", "20998.54", 14)
			if err := store.SaveQuotaSettlement(context.Background(), task.Scope, pending, firstAt); err != nil {
				t.Fatal(err)
			}
			terminal := QuotaSettlementResult{Settlement: validQuotaSettlement()}
			terminal.Settlement.TaskID = task.TaskID
			terminal.Settlement.SettlementID = "set-terminal-balance"
			terminal.Settlement.BalanceVersion = settlementVersion(test.version)
			terminal.Balance = withoutOuterBalanceSnapshot("20998.54", test.version)
			err := store.SaveQuotaSettlement(context.Background(), task.Scope, terminal, firstAt.Add(time.Minute))
			if test.wantConflict && ErrorCodeOf(err) != ErrorIdempotencyConflict {
				t.Fatalf("terminal %s balance version = %v", test.name, err)
			}
			if !test.wantConflict && err != nil {
				t.Fatalf("terminal higher balance version = %v", err)
			}
		})
	}
}

func pendingSettlementWithBalance(taskID, settlementID, total string, version uint64) QuotaSettlementResult {
	return QuotaSettlementResult{
		Settlement: QuotaSettlement{
			SettlementID: settlementID, TaskID: taskID, Status: QuotaSettlementPending,
			Balance: settlementDecimal(total), BalanceVersion: settlementVersion(version),
		},
		Balance: withoutOuterBalanceSnapshot(total, version),
	}
}

func settlementStoreTask(t *testing.T, db *sql.DB, taskID string) ModelTask {
	t.Helper()
	task, evidence := storeTask(BillingSubscriptionPool, taskID)
	tasks := NewSQLiteTaskStore(db)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	taskContext := storeContext(t, task)
	if err := tasks.RecordFreezeRequest(context.Background(), taskContext, freezeRequest(task), storeNow().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	freeze := &cloud.Freeze{
		Scope: *task.Scope.CloudBinding, FreezeID: "freeze_" + task.TaskID, TaskID: task.TaskID,
		Status: cloud.FreezeActive, FreezeAmount: "300", FreeFrozenAmount: "300", SubscriptionFrozenAmount: "0",
		ExpiresAt: storeNow().Add(time.Hour),
	}
	if err := tasks.RecordFreezeResult(context.Background(), taskContext, freeze,
		&TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}, storeNow().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	loaded, err := tasks.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}
