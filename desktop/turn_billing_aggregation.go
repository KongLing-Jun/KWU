package main

import (
	"log/slog"

	"kwu/internal/modelbilling"
	"kwu/internal/provider"
)

type turnBillingAggregationEntry struct {
	turnObserved       bool
	toolMaaSQuota      string
	toolChargedCredits string
	settlement         *TurnBillingSettledView
}

type turnBillingAggregationSnapshot struct {
	entries map[string]turnBillingAggregationEntry
}

func (a *App) observeTurnToolCharges(turn provider.TurnMetadata) {
	if a == nil || turn.TaskID == "" || turn.BillingSource != string(modelbilling.BillingSubscriptionPool) {
		return
	}
	settled := a.mergeTurnBilling(turn.TaskID, func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
		entry.turnObserved = true
		entry.toolMaaSQuota = turn.ToolMaaSQuota
		entry.toolChargedCredits = turn.ToolChargedCredits
		return entry
	})
	if settled != nil {
		a.emitRuntimeEvent(turnBillingSettledEvent, *settled)
	}
}

func (a *App) observeModelTurnSettlement(view TurnBillingSettledView) {
	if a == nil || view.TaskID == "" {
		return
	}
	settled := a.mergeTurnBilling(view.TaskID, func(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
		copy := view
		entry.settlement = &copy
		return entry
	})
	if settled != nil {
		a.emitRuntimeEvent(turnBillingSettledEvent, *settled)
	}
}

func (a *App) mergeTurnBilling(
	taskID string,
	update func(turnBillingAggregationEntry) turnBillingAggregationEntry,
) *TurnBillingSettledView {
	for {
		current := a.turnBillingAggregation.Load()
		if current == nil {
			empty := &turnBillingAggregationSnapshot{entries: map[string]turnBillingAggregationEntry{}}
			if !a.turnBillingAggregation.CompareAndSwap(nil, empty) {
				continue
			}
			current = empty
		}
		entry := cloneTurnBillingEntry(current.entries[taskID])
		entry = update(entry)
		next := &turnBillingAggregationSnapshot{entries: make(map[string]turnBillingAggregationEntry, len(current.entries)+1)}
		for id, existing := range current.entries {
			next.entries[id] = existing
		}
		var delivery *TurnBillingSettledView
		if entry.turnObserved && entry.settlement != nil {
			delete(next.entries, taskID)
			view := *entry.settlement
			credits, err := modelbilling.AggregateTaskCredits(
				view.ChargedCredits, entry.toolMaaSQuota, entry.toolChargedCredits,
				view.quotaPerUnit, view.usdExchangeRate, view.creditPerCNY,
			)
			if err != nil {
				slog.Warn("desktop: aggregate paid tool credits", "task_id", taskID, "err", err)
			} else {
				view.ChargedCredits = credits
			}
			delivery = &view
		} else {
			next.entries[taskID] = entry
		}
		if a.turnBillingAggregation.CompareAndSwap(current, next) {
			return delivery
		}
	}
}

func cloneTurnBillingEntry(entry turnBillingAggregationEntry) turnBillingAggregationEntry {
	if entry.settlement != nil {
		copy := *entry.settlement
		entry.settlement = &copy
	}
	return entry
}
