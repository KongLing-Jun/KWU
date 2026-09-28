package modelbilling

import (
	"context"
	"testing"
	"time"
)

func TestCurrentQuotaSettlements(t *testing.T) {
	t.Parallel()
	db := openSchemaTestDB(t)
	task := settlementStoreTask(t, db, "batch-settled")
	pending := settlementStoreTask(t, db, "batch-pending")
	store := NewSQLiteQuotaSettlementStore(db)
	result := QuotaSettlementResult{Settlement: validQuotaSettlement()}
	result.Settlement.TaskID = task.TaskID
	result.Settlement.SettlementID = "set-batch"
	if err := store.SaveQuotaSettlement(context.Background(), task.Scope, result, time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	ids := []string{task.TaskID, pending.TaskID, "missing", task.TaskID}
	got, err := NewSQLiteQuotaSettlementStore(db).CurrentQuotaSettlements(context.Background(), task.Scope, ids)
	if err != nil || len(got) != 2 || got[task.TaskID] == nil || got[task.TaskID].Settlement.Status != QuotaSettlementSettled {
		t.Fatalf("batch: %+v, %v", got, err)
	}
	if value, ok := got[pending.TaskID]; !ok || value != nil {
		t.Fatalf("pending task: %+v", value)
	}
	if _, ok := got["missing"]; ok {
		t.Fatal("unknown task exposed")
	}
	for _, name := range []string{"actor", "profile", "binding"} {
		t.Run(name, func(t *testing.T) {
			scope := task.Scope
			binding := *scope.CloudBinding
			scope.CloudBinding = &binding
			switch name {
			case "actor":
				scope.ActorID = "other"
			case "profile":
				scope.ProfileID = "other"
				binding.ProfileID = "other"
			case "binding":
				binding.BindingGeneration++
			}
			got, err := store.CurrentQuotaSettlements(context.Background(), scope, ids)
			if err == nil && len(got) != 0 {
				t.Fatalf("cross-scope result: %+v", got)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.CurrentQuotaSettlements(ctx, task.Scope, ids); err == nil {
		t.Fatal("canceled batch succeeded")
	}
	if _, err := store.CurrentQuotaSettlements(context.Background(), task.Scope, make([]string, 65)); err == nil {
		t.Fatal("oversized batch accepted")
	}
}
