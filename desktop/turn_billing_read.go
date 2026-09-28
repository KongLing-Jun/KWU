package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
)

const turnBillingBatchLimit = 64

type TurnBillingQuery struct {
	SessionPath       string   `json:"sessionPath"`
	ProfileID         string   `json:"profileId"`
	CloudUserID       string   `json:"cloudUserId"`
	BindingGeneration uint64   `json:"bindingGeneration"`
	TaskIDs           []string `json:"taskIds"`
}

type TurnBillingResult struct {
	TaskID         string  `json:"taskId"`
	Status         string  `json:"status"`
	ChargedCredits *string `json:"chargedCredits,omitempty"`
}

// TurnBillingForTab reads already persisted facts, independently of transcript
// watermarks. Session ownership and binding are checked again on every request.
func (a *App) TurnBillingForTab(tabID string, query TurnBillingQuery) ([]TurnBillingResult, error) {
	if len(query.TaskIDs) == 0 || len(query.TaskIDs) > turnBillingBatchLimit {
		return nil, fmt.Errorf("invalid billing batch size")
	}
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab == nil {
		return nil, fmt.Errorf("session is unavailable")
	}
	path := tab.SessionPath
	if ctrl != nil {
		path = ctrl.SessionPath()
	}
	if strings.TrimSpace(path) == "" || sessionRuntimeKey(path) != sessionRuntimeKey(query.SessionPath) {
		return nil, fmt.Errorf("session changed during billing read")
	}
	actor := tabRuntimeActorContext(tab).Normalize()
	if visible, _, _ := a.projectSessionListVisible(a.bootContext(), actor, path); !visible {
		return nil, fmt.Errorf("session is unavailable")
	}
	binding, err := currentDesktopSubscriptionBinding(actor, a.billingWorkerRuntime)
	if err != nil || binding.ProfileID != query.ProfileID || binding.CloudUserID != query.CloudUserID || binding.BindingGeneration != query.BindingGeneration {
		return nil, fmt.Errorf("billing scope is unavailable")
	}
	_, path, err = resolveTabSessionPath(tab, path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 5*time.Second)
	defer cancel()
	turns, err := readSessionBillingTurns(ctx, path, query.TaskIDs)
	if err != nil {
		return nil, err
	}
	// A missing database is unavailable, not an invitation to create or migrate it.
	dbPath := filepath.Join(config.NewProfilePathResolver(actor).MemoryUserDir(), "model-billing.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("local billing database is unavailable")
	}
	uriPath := filepath.ToSlash(dbPath)
	if filepath.VolumeName(dbPath) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath}
	db, err := sql.Open("sqlite", uri.String()+"?mode=ro&_pragma=busy_timeout(1000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
	results := readTurnBillingResults(ctx, modelbilling.NewSQLiteQuotaSettlementStore(db), scope, query.TaskIDs, turns)
	current, err := currentDesktopSubscriptionBinding(actor, a.billingWorkerRuntime)
	if err != nil || current != binding {
		return nil, fmt.Errorf("billing scope changed during read")
	}
	return results, nil
}

// Decode only turn metadata from the model history, not the much larger UI
// event log. One pass serves the entire batch; client-supplied costs are ignored.
func readSessionBillingTurns(ctx context.Context, path string, ids []string) (map[string]provider.TurnMetadata, error) {
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	turns := make(map[string]provider.TurnMetadata)
	decoder := json.NewDecoder(io.LimitReader(file, info.Size()))
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var message struct {
			Role provider.Role          `json:"role"`
			Turn *provider.TurnMetadata `json:"turn"`
		}
		if err := decoder.Decode(&message); errors.Is(err, io.EOF) {
			return turns, nil
		} else if err != nil {
			return nil, err
		}
		if message.Role == provider.RoleAssistant && message.Turn != nil && wanted[message.Turn.TaskID] {
			turns[message.Turn.TaskID] = *message.Turn
		}
	}
}

type turnBillingBatchReader interface {
	CurrentQuotaSettlements(context.Context, modelbilling.TaskScope, []string) (map[string]*modelbilling.QuotaSettlementResult, error)
}

func readTurnBillingResults(ctx context.Context, store turnBillingBatchReader, scope modelbilling.TaskScope, ids []string, turns map[string]provider.TurnMetadata) []TurnBillingResult {
	results := make([]TurnBillingResult, 0, len(ids))
	owned := make([]string, 0, len(ids))
	for _, id := range ids {
		if turn, ok := turns[id]; ok && turn.BillingSource == string(modelbilling.BillingSubscriptionPool) {
			owned = append(owned, id)
		}
	}
	var settlements map[string]*modelbilling.QuotaSettlementResult
	var readErr error
	if len(owned) > 0 {
		settlements, readErr = store.CurrentQuotaSettlements(ctx, scope, owned)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		view := TurnBillingResult{TaskID: id, Status: "unavailable"}
		turn, ok := turns[id]
		if ok && turn.BillingSource == string(modelbilling.BillingSubscriptionPool) {
			result, found := settlements[id]
			switch {
			case modelbilling.ErrorCodeOf(readErr) == modelbilling.ErrorScopeMismatch:
			case readErr != nil:
				view.Status = "error"
			case !found:
			case result == nil || result.Settlement.Status == modelbilling.QuotaSettlementPending:
				view.Status = "pending"
			case result.Settlement.Status == modelbilling.QuotaSettlementSettled || result.Settlement.Status == modelbilling.QuotaSettlementSettledUncovered:
				credits, err := settledTurnCredits(*result, turn.ToolMaaSQuota, turn.ToolChargedCredits)
				if err != nil {
					view.Status = "error"
				} else {
					view.Status, view.ChargedCredits = "settled", &credits
				}
			}
		}
		results = append(results, view)
	}
	return results
}

func settledTurnCredits(result modelbilling.QuotaSettlementResult, toolQuota, toolCredits string) (string, error) {
	if result.Settlement.ChargedCredits == nil {
		return "", fmt.Errorf("settled credits are missing")
	}
	return modelbilling.AggregateTaskCredits(string(*result.Settlement.ChargedCredits), toolQuota, toolCredits,
		optionalDesktopDecimal(result.Settlement.QuotaPerUnit), optionalDesktopDecimal(result.Settlement.USDExchangeRate), optionalDesktopDecimal(result.Settlement.CreditPerCNY))
}
