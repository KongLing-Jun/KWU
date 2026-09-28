package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

type turnBillingReadStore struct {
	modelbilling.QuotaSettlementStore
	result modelbilling.QuotaSettlementResult
	err    error
	found  bool
	calls  int
}

func TestTurnBillingForTabRestoresWithoutTranscriptEvents(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	actor := userctx.Default(root)
	dir := desktopSessionDirForActor(actor, root)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "billing.jsonl")
	message := provider.Message{Role: provider.RoleAssistant, Content: "completed", Turn: &provider.TurnMetadata{
		TaskID: "task-1", BillingSource: "subscription_pool", ToolChargedCredits: "0.25",
	}}
	payload, _ := json.Marshal(message)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "billing-test-user", BindingGeneration: 1}
	dbPath := filepath.Join(config.NewProfilePathResolver(actor).MemoryUserDir(), "model-billing.sqlite")
	db, err := modelbilling.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`CREATE TABLE model_tasks(task_id TEXT PRIMARY KEY,actor_id TEXT,profile_id TEXT,cloud_user_id TEXT,binding_generation INTEGER,billing_source TEXT)`,
		`CREATE TABLE model_task_quota_settlement_current(task_id TEXT PRIMARY KEY,settlement_revision INTEGER,settlement_id TEXT,status TEXT,normalized_payload BLOB,payload_hash TEXT,observed_at TEXT)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO model_tasks VALUES(?,?,?,?,?,?)`, "task-1", actor.ActorID, actor.ProfileID, binding.CloudUserID, 1, "subscription_pool"); err != nil {
		t.Fatal(err)
	}
	newApp := func() *App {
		a := &App{tabRuntimes: make(map[string]*WorkspaceTab)}
		a.tabRuntimes["tab"] = &WorkspaceTab{ID: "tab", Scope: "global", WorkspaceRoot: root, SessionPath: path, actor: actor}
		a.billingWorkerRuntime = &desktopBillingWorkerManager{hasBinding: true, bindingActor: actor, bindingScope: binding}
		return a
	}
	query := TurnBillingQuery{SessionPath: path, ProfileID: actor.ProfileID, CloudUserID: binding.CloudUserID, BindingGeneration: 1, TaskIDs: []string{"task-1"}}
	a := newApp()
	got, err := a.TurnBillingForTab("tab", query)
	if err != nil || len(got) != 1 || got[0].Status != "pending" {
		t.Fatalf("before settlement: %+v,%v", got, err)
	}
	result := testPersistedTurnSettlement()
	payload, err = modelbilling.CanonicalJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(payload)
	if _, err := db.Exec(`INSERT INTO model_task_quota_settlement_current VALUES(?,?,?,?,?,?,?)`, "task-1", 1, "set-1", "settled", []byte(payload), hex.EncodeToString(hash[:]), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	for _, app := range []*App{a, newApp()} {
		got, err = app.TurnBillingForTab("tab", query)
		if err != nil || len(got) != 1 || got[0].ChargedCredits == nil || *got[0].ChargedCredits != "1.7100" {
			t.Fatalf("local recovery: %+v,%v", got, err)
		}
	}
	query.BindingGeneration++
	if _, err := a.TurnBillingForTab("tab", query); err == nil {
		t.Fatal("wrong binding accepted")
	}
	query.BindingGeneration--
	query.SessionPath = filepath.Join(dir, "other.jsonl")
	if _, err := a.TurnBillingForTab("tab", query); err == nil {
		t.Fatal("wrong session accepted")
	}
}

func testPersistedTurnSettlement() modelbilling.QuotaSettlementResult {
	d := func(v string) *modelbilling.DecimalString { value := modelbilling.DecimalString(v); return &value }
	version := uint64(1)
	policy := "test"
	return modelbilling.QuotaSettlementResult{Settlement: modelbilling.QuotaSettlement{
		SettlementID: "set-1", TaskID: "task-1", Status: modelbilling.QuotaSettlementSettled,
		TaskTotalQuota: d("50"), QuotaPolicyVersion: &policy, QuotaPerUnit: d("500000"), USDExchangeRate: d("7.3"), CreditPerCNY: d("2000"),
		ChargedCredits: d("1.4600"), ConsumedFrozenAmount: d("1.46"), ConsumedFrozenFreeAmount: d("1.46"), ConsumedFrozenSubscriptionAmount: d("0"),
		AdditionalChargedAmount: d("0"), AdditionalChargedFreeAmount: d("0"), AdditionalChargedSubscriptionAmount: d("0"),
		ReleasedFrozenAmount: d("298.54"), ReleasedFrozenFreeAmount: d("298.54"), ReleasedFrozenSubscriptionAmount: d("0"),
		UncoveredAmount: d("0"), Balance: d("20998.54"), BalanceVersion: &version,
	}}
}

func (s *turnBillingReadStore) CurrentQuotaSettlements(context.Context, modelbilling.TaskScope, []string) (map[string]*modelbilling.QuotaSettlementResult, error) {
	s.calls++
	if !s.found {
		return map[string]*modelbilling.QuotaSettlementResult{"task": nil}, s.err
	}
	return map[string]*modelbilling.QuotaSettlementResult{"task": &s.result}, s.err
}

func TestReadSessionBillingTurns(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	data := `{"role":"user","content":"test"}
{"role":"assistant","turn":{"task_id":"wanted","billing_source":"subscription_pool","tool_maas_quota":"500000","tool_charged_credits":"0.25"}}
{"role":"assistant","turn":{"task_id":"other","billing_source":"subscription_pool"}}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	turns, err := readSessionBillingTurns(context.Background(), path, []string{"wanted", "unrelated"})
	if err != nil || len(turns) != 1 || turns["wanted"].ToolMaaSQuota != "500000" || turns["wanted"].ToolChargedCredits != "0.25" {
		t.Fatalf("read persisted metadata: %+v, %v", turns, err)
	}
}

func TestReadTurnBillingResults(t *testing.T) {
	t.Parallel()
	decimal := func(value string) *modelbilling.DecimalString { v := modelbilling.DecimalString(value); return &v }
	for _, test := range []struct {
		name                    string
		status                  modelbilling.QuotaSettlementState
		credits                 *modelbilling.DecimalString
		toolQuota, toolCredits  string
		err                     error
		found, belongs          bool
		wantStatus, wantCredits string
	}{
		{name: "settled without notification", status: modelbilling.QuotaSettlementSettled, credits: decimal("7.9"), found: true, belongs: true, wantStatus: "settled", wantCredits: "7.9000"},
		{name: "legal zero", status: modelbilling.QuotaSettlementSettled, credits: decimal("0"), found: true, belongs: true, wantStatus: "settled", wantCredits: "0.0000"},
		{name: "model and tools", status: modelbilling.QuotaSettlementSettled, credits: decimal("7.9"), toolQuota: "500000", toolCredits: "0.25", found: true, belongs: true, wantStatus: "settled", wantCredits: "14608.1500"},
		{name: "pending", status: modelbilling.QuotaSettlementPending, found: true, belongs: true, wantStatus: "pending"},
		{name: "not observed yet", belongs: true, wantStatus: "pending"},
		{name: "missing amount", status: modelbilling.QuotaSettlementSettled, found: true, belongs: true, wantStatus: "error"},
		{name: "read failure", belongs: true, err: errors.New("read failed"), wantStatus: "error"},
		{name: "wrong binding", belongs: true, err: &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch}, wantStatus: "unavailable"},
		{name: "other session", found: true, wantStatus: "unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &turnBillingReadStore{found: test.found, err: test.err, result: modelbilling.QuotaSettlementResult{Settlement: modelbilling.QuotaSettlement{
				Status: test.status, ChargedCredits: test.credits, QuotaPerUnit: decimal("500000"), USDExchangeRate: decimal("7.3"), CreditPerCNY: decimal("2000"),
			}}}
			turns := map[string]provider.TurnMetadata{}
			if test.belongs {
				turns["task"] = provider.TurnMetadata{TaskID: "task", BillingSource: "subscription_pool", ToolMaaSQuota: test.toolQuota, ToolChargedCredits: test.toolCredits}
			}
			got := readTurnBillingResults(context.Background(), store, modelbilling.TaskScope{}, []string{"task", "task"}, turns)
			if len(got) != 1 || got[0].Status != test.wantStatus {
				t.Fatalf("result: %+v", got)
			}
			if test.wantStatus == "settled" {
				if got[0].ChargedCredits == nil || *got[0].ChargedCredits != test.wantCredits {
					t.Fatalf("credits: %+v", got[0])
				}
			} else if got[0].ChargedCredits != nil {
				t.Fatal("failure exposed credits")
			}
			if store.calls > 1 || (!test.belongs && store.calls != 0) {
				t.Fatalf("unexpected reads: %d", store.calls)
			}
		})
	}
}
