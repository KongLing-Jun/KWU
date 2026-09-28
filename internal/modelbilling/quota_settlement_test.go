package modelbilling

import (
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestQuotaSettlementDoesNotFreezePendingOrManualFieldPresence(t *testing.T) {
	for _, settlement := range []QuotaSettlement{
		{SettlementID: "set-pending", TaskID: "task-1", Status: QuotaSettlementPending},
		{
			SettlementID: "set-manual", TaskID: "task-1", Status: QuotaSettlementManualRequired,
			TaskTotalQuota: settlementDecimal("50"), ChargedCredits: settlementDecimal("0"),
			ErrorCode: settlementString("MANUAL_REVIEW_REQUIRED"),
		},
	} {
		if err := settlement.Validate(); err != nil {
			t.Fatalf("Validate(%s) error = %v", settlement.Status, err)
		}
	}
}

func TestQuotaSettlementValidatesLedgerAndBalanceConservation(t *testing.T) {
	settlement := validQuotaSettlement()
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "user-1", BindingGeneration: 1}
	task := ModelTask{
		TaskID: "task-1", BillingSource: BillingSubscriptionPool,
		Scope: TaskScope{ActorID: "actor-1", ProfileID: "default", CloudBinding: &binding},
		Freeze: &cloud.Freeze{
			Scope: binding, FreezeID: "freeze-1", TaskID: "task-1", Status: cloud.FreezeActive,
			FreezeAmount: "300", FreeFrozenAmount: "300", SubscriptionFrozenAmount: "0",
			ExpiresAt: time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC),
		},
	}
	if err := settlement.ValidateForTask(task); err != nil {
		t.Fatalf("ValidateForTask() error = %v", err)
	}

	settlement.ReleasedFrozenFreeAmount = settlementDecimal("285.3")
	if err := settlement.ValidateForTask(task); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("ValidateForTask() conservation error = %v", err)
	}
}

func TestQuotaSettlementResultRequiresMatchingAuthoritativeBalance(t *testing.T) {
	settlement := validQuotaSettlement()
	result := QuotaSettlementResult{
		Settlement: settlement,
		Balance: &cloud.BalanceSnapshot{
			TotalBalance: "20998.54", FreeBalance: "998.54", SubscriptionBalance: "20000",
			AvailableBalance: "20998.54", FreeAvailableBalance: "998.54", SubscriptionAvailableBalance: "20000",
			FrozenBalance: "0", FreeFrozenBalance: "0", SubscriptionFrozenBalance: "0", Version: 14,
		},
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	result.Balance.Version--
	if err := result.Validate(); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("Validate() mismatch error = %v", err)
	}

	withoutOuter := QuotaSettlementResult{Settlement: settlement}
	if err := withoutOuter.Validate(); err != nil {
		t.Fatalf("terminal result without outer balance = %v", err)
	}
	pending := QuotaSettlementResult{
		Settlement: QuotaSettlement{
			SettlementID: "set-pending", TaskID: "task-1", Status: QuotaSettlementPending,
			Balance: settlementDecimal("20985.4"), BalanceVersion: settlementVersion(14),
		},
		Balance: withoutOuterBalanceSnapshot("20985.4", 14),
	}
	if err := pending.Validate(); err != nil {
		t.Fatalf("pending result with consistent outer balance = %v", err)
	}
	outerOnly := QuotaSettlementResult{
		Settlement: QuotaSettlement{SettlementID: "set-outer", TaskID: "task-1", Status: QuotaSettlementPending},
		Balance:    withoutOuterBalanceSnapshot("10", 15),
	}
	if err := outerOnly.Validate(); err != nil {
		t.Fatalf("pending result with outer-only B5 fields = %v", err)
	}
	outerOnly.Balance.TotalBalance = "-1"
	if err := outerOnly.Validate(); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("invalid outer balance amount = %v", err)
	}
}

func TestQuotaSettlementResultAcceptsNewerAuthoritativeBalanceSnapshot(t *testing.T) {
	settlement := validQuotaSettlement()
	settlementBalanceVersion := *settlement.BalanceVersion
	result := QuotaSettlementResult{
		Settlement: settlement,
		Balance: &cloud.BalanceSnapshot{
			TotalBalance: "1200.0000", FreeBalance: "1200.0000", SubscriptionBalance: "0.0000",
			AvailableBalance: "1200.0000", FreeAvailableBalance: "1200.0000",
			SubscriptionAvailableBalance: "0.0000", FrozenBalance: "0.0000",
			FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000",
			Version: settlementBalanceVersion + 1,
		},
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("newer authoritative balance rejected: %v", err)
	}

	result.Balance.Version = settlementBalanceVersion - 1
	if err := result.Validate(); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("older authoritative balance error = %v", err)
	}
}

func TestQuotaSettlementBindsAcceptedFinalAndPolicyParameters(t *testing.T) {
	settlement := validQuotaSettlement()
	if err := settlement.ValidateTaskTotalQuota("50.0"); err != nil {
		t.Fatalf("matching task total = %v", err)
	}
	if err := settlement.ValidateTaskTotalQuota("51"); ErrorCodeOf(err) != ErrorQuotaSummaryMismatch {
		t.Fatalf("mismatching task total = %v", err)
	}
	settlement.QuotaPerUnit = settlementDecimal("0")
	if err := settlement.Validate(); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("zero quota parameter = %v", err)
	}
	settlement = validQuotaSettlement()
	settlement.QuotaPerUnit = settlementDecimal("600000")
	settlement.USDExchangeRate = settlementDecimal("7.4")
	if err := settlement.Validate(); err != nil {
		t.Fatalf("valid updated policy snapshot = %v", err)
	}
	settlement.CreditPerCNY = settlementDecimal("1000")
	if err := settlement.Validate(); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("invalid fixed credit conversion = %v", err)
	}
	settlement = validQuotaSettlement()
	settlement.QuotaPolicyVersion = nil
	if err := settlement.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("missing quota policy version = %v", err)
	}
}

func TestQuotaSettlementAllowsUncoveredChargeWithOtherTasksFrozenBalance(t *testing.T) {
	settlement := validQuotaSettlement()
	settlement.Status = QuotaSettlementSettledUncovered
	settlement.TaskTotalQuota = settlementDecimal("79367")
	settlement.ChargedCredits = settlementDecimal("2317.5164")
	settlement.ConsumedFrozenAmount = settlementDecimal("260.1720")
	settlement.ConsumedFrozenFreeAmount = settlementDecimal("260.1720")
	settlement.ReleasedFrozenAmount = settlementDecimal("0")
	settlement.ReleasedFrozenFreeAmount = settlementDecimal("0")
	settlement.UncoveredAmount = settlementDecimal("2057.3444")
	settlement.Balance = settlementDecimal("300.0000")

	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "user-1", BindingGeneration: 1}
	task := ModelTask{
		TaskID: settlement.TaskID, BillingSource: BillingSubscriptionPool,
		Scope: TaskScope{ActorID: "actor-1", ProfileID: "default", CloudBinding: &binding},
		Freeze: &cloud.Freeze{
			Scope: binding, FreezeID: "freeze-1", TaskID: settlement.TaskID, Status: cloud.FreezeActive,
			FreezeAmount: "260.1720", FreeFrozenAmount: "260.1720", SubscriptionFrozenAmount: "0",
			ExpiresAt: time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC),
		},
	}
	if err := settlement.ValidateForTask(task); err != nil {
		t.Fatalf("uncovered settlement with unrelated frozen balance rejected: %v", err)
	}
}

func TestQuotaSettlementRejectsLedgerPriorityViolations(t *testing.T) {
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "user-1", BindingGeneration: 1}
	task := ModelTask{
		TaskID: "task-1", BillingSource: BillingSubscriptionPool,
		Scope: TaskScope{ActorID: "actor-1", ProfileID: "default", CloudBinding: &binding},
		Freeze: &cloud.Freeze{
			Scope: binding, FreezeID: "freeze-1", TaskID: "task-1", Status: cloud.FreezeActive,
			FreezeAmount: "300", FreeFrozenAmount: "300", SubscriptionFrozenAmount: "0",
			ExpiresAt: time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC),
		},
	}

	releaseAndAdditional := validQuotaSettlement()
	releaseAndAdditional.ChargedCredits = settlementDecimal("2.46")
	releaseAndAdditional.AdditionalChargedAmount = settlementDecimal("1")
	releaseAndAdditional.AdditionalChargedFreeAmount = settlementDecimal("1")
	if err := releaseAndAdditional.ValidateForTask(task); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("release/additional priority error = %v", err)
	}

	task.Freeze.FreeFrozenAmount = "200"
	task.Freeze.SubscriptionFrozenAmount = "100"
	freeFirst := validQuotaSettlement()
	freeFirst.ConsumedFrozenFreeAmount = settlementDecimal("0.46")
	freeFirst.ConsumedFrozenSubscriptionAmount = settlementDecimal("1")
	freeFirst.ReleasedFrozenFreeAmount = settlementDecimal("199.54")
	freeFirst.ReleasedFrozenSubscriptionAmount = settlementDecimal("99")
	if err := freeFirst.ValidateForTask(task); ErrorCodeOf(err) != ErrorSettlementBalanceInconsistent {
		t.Fatalf("free-first priority error = %v", err)
	}
}

func validQuotaSettlement() QuotaSettlement {
	version := uint64(14)
	policyVersion := "maas-quota-config-v1"
	return QuotaSettlement{
		SettlementID: "set-1", TaskID: "task-1", Status: QuotaSettlementSettled,
		TaskTotalQuota: settlementDecimal("50"), QuotaPolicyVersion: &policyVersion, QuotaPerUnit: settlementDecimal("500000"),
		USDExchangeRate: settlementDecimal("7.3"), CreditPerCNY: settlementDecimal("2000"),
		ChargedCredits: settlementDecimal("1.4600"), ConsumedFrozenAmount: settlementDecimal("1.46"),
		ConsumedFrozenFreeAmount: settlementDecimal("1.46"), ConsumedFrozenSubscriptionAmount: settlementDecimal("0"),
		AdditionalChargedAmount: settlementDecimal("0"), AdditionalChargedFreeAmount: settlementDecimal("0"),
		AdditionalChargedSubscriptionAmount: settlementDecimal("0"), ReleasedFrozenAmount: settlementDecimal("298.54"),
		ReleasedFrozenFreeAmount: settlementDecimal("298.54"), ReleasedFrozenSubscriptionAmount: settlementDecimal("0"),
		UncoveredAmount: settlementDecimal("0"), Balance: settlementDecimal("20998.54"), BalanceVersion: &version,
	}
}

func settlementDecimal(value string) *DecimalString {
	decimal := DecimalString(value)
	return &decimal
}

func settlementString(value string) *string { return &value }

func settlementVersion(value uint64) *uint64 { return &value }

func withoutOuterBalanceSnapshot(total string, version uint64) *cloud.BalanceSnapshot {
	return &cloud.BalanceSnapshot{
		TotalBalance: total, FreeBalance: "0", SubscriptionBalance: "0", AvailableBalance: "0",
		FreeAvailableBalance: "0", SubscriptionAvailableBalance: "0", FrozenBalance: "0",
		FreeFrozenBalance: "0", SubscriptionFrozenBalance: "0", Version: version,
	}
}
