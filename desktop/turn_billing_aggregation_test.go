package main

import (
	"fmt"
	"sync"
	"testing"
)

func TestTurnBillingAggregationJoinsSettlementAndToolFactsInEitherOrder(t *testing.T) {
	for _, settlementFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "settlement_first", false: "turn_first"}[settlementFirst], func(t *testing.T) {
			app := &App{}
			settlement := TurnBillingSettledView{
				TaskID: "task-1", ChargedCredits: "10.0000",
				quotaPerUnit: "500000", usdExchangeRate: "7.3", creditPerCNY: "2000",
			}
			putSettlement := func() *TurnBillingSettledView {
				return app.mergeTurnBilling("task-1", func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
					copy := settlement
					entry.settlement = &copy
					return entry
				})
			}
			putTurn := func() *TurnBillingSettledView {
				return app.mergeTurnBilling("task-1", func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
					entry.turnObserved = true
					entry.toolMaaSQuota = "35800"
					entry.toolChargedCredits = "0.264"
					return entry
				})
			}
			var first, second *TurnBillingSettledView
			if settlementFirst {
				first, second = putSettlement(), putTurn()
			} else {
				first, second = putTurn(), putSettlement()
			}
			if first != nil {
				t.Fatalf("first fact unexpectedly completed aggregation: %+v", first)
			}
			if second == nil || second.TaskID != "task-1" || second.ChargedCredits != "1055.6240" {
				t.Fatalf("aggregated settlement = %+v", second)
			}
		})
	}
}

func TestTurnBillingAggregationAcceptsZeroToolCharges(t *testing.T) {
	app := &App{}
	if got := app.mergeTurnBilling("task-2", func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
		entry.turnObserved = true
		return entry
	}); got != nil {
		t.Fatalf("turn-only aggregation = %+v", got)
	}
	got := app.mergeTurnBilling("task-2", func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
		entry.settlement = &TurnBillingSettledView{TaskID: "task-2", ChargedCredits: "7.9"}
		return entry
	})
	if got == nil || got.ChargedCredits != "7.9000" {
		t.Fatalf("zero-tool settlement = %+v", got)
	}
}

func TestTurnBillingAggregationAddsObservedImageQuotaToModelCharge(t *testing.T) {
	app := &App{}
	if got := app.mergeTurnBilling("task-image", func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
		entry.turnObserved = true
		entry.toolMaaSQuota = "2898"
		return entry
	}); got != nil {
		t.Fatalf("tool-only aggregation = %+v", got)
	}
	got := app.mergeTurnBilling("task-image", func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
		entry.settlement = &TurnBillingSettledView{
			TaskID: "task-image", ChargedCredits: "164.9800",
			quotaPerUnit: "500000", usdExchangeRate: "7.3", creditPerCNY: "2000",
		}
		return entry
	})
	// Image quota 2898 converts to 84.6216 credits, so the displayed turn total
	// must be 164.9800 + 84.6216 rather than either component alone.
	if got == nil || got.ChargedCredits != "249.6016" {
		t.Fatalf("image + model settlement = %+v, want 249.6016", got)
	}
}

func TestTurnBillingAggregationConcurrentCASUpdates(t *testing.T) {
	type result struct {
		taskID string
		view   *TurnBillingSettledView
	}
	app := &App{}
	results := make(chan result, 200)
	var workers sync.WaitGroup
	for i := 0; i < 100; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		workers.Add(2)
		go func() {
			defer workers.Done()
			view := app.mergeTurnBilling(taskID, func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
				entry.turnObserved = true
				entry.toolChargedCredits = "0.25"
				return entry
			})
			results <- result{taskID: taskID, view: view}
		}()
		go func() {
			defer workers.Done()
			view := app.mergeTurnBilling(taskID, func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
				entry.settlement = &TurnBillingSettledView{TaskID: taskID, ChargedCredits: "1"}
				return entry
			})
			results <- result{taskID: taskID, view: view}
		}()
	}
	workers.Wait()
	close(results)

	settled := make(map[string]int, 100)
	for result := range results {
		if result.view == nil {
			continue
		}
		if result.view.TaskID != result.taskID || result.view.ChargedCredits != "1.2500" {
			t.Fatalf("aggregation result for %s = %+v", result.taskID, result.view)
		}
		settled[result.taskID]++
	}
	if len(settled) != 100 {
		t.Fatalf("settled tasks = %d, want 100", len(settled))
	}
	for taskID, count := range settled {
		if count != 1 {
			t.Fatalf("settlement deliveries for %s = %d, want 1", taskID, count)
		}
	}
}
