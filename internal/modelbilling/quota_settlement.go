package modelbilling

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"kwu/internal/cloud"
)

type QuotaSettlementState string

const (
	QuotaSettlementPending          QuotaSettlementState = "settlement_pending"
	QuotaSettlementSettled          QuotaSettlementState = "settled"
	QuotaSettlementSettledUncovered QuotaSettlementState = "settled_with_uncovered_amount"
	QuotaSettlementManualRequired   QuotaSettlementState = "failed_manual_required"
)

func (s QuotaSettlementState) terminal() bool {
	return s == QuotaSettlementSettled || s == QuotaSettlementSettledUncovered ||
		s == QuotaSettlementManualRequired
}

const QuotaSettlementCreditPerCNY DecimalString = "2000"

// AggregateTaskCredits adds paid tool charges to the authoritative model-task
// charge and rounds only the task-level total to four decimal places.
func AggregateTaskCredits(
	modelCredits string,
	toolMaaSQuota string,
	toolChargedCredits string,
	quotaPerUnit string,
	usdExchangeRate string,
	creditPerCNY string,
) (string, error) {
	total, err := nonNegativeCreditRat("model charged_credits", modelCredits)
	if err != nil {
		return "", err
	}
	if toolChargedCredits != "" {
		direct, parseErr := nonNegativeCreditRat("tool charged_credits", toolChargedCredits)
		if parseErr != nil {
			return "", parseErr
		}
		total.Add(total, direct)
	}
	if toolMaaSQuota != "" {
		quota, parseErr := nonNegativeCreditRat("tool MaaS quota", toolMaaSQuota)
		if parseErr != nil {
			return "", parseErr
		}
		quotaUnit, parseErr := positiveCreditRat("quota_per_unit", quotaPerUnit)
		if parseErr != nil {
			return "", parseErr
		}
		exchangeRate, parseErr := positiveCreditRat("usd_exchange_rate", usdExchangeRate)
		if parseErr != nil {
			return "", parseErr
		}
		creditRate, parseErr := positiveCreditRat("credit_per_cny", creditPerCNY)
		if parseErr != nil {
			return "", parseErr
		}
		converted := new(big.Rat).Mul(quota, exchangeRate)
		converted.Mul(converted, creditRate)
		converted.Quo(converted, quotaUnit)
		total.Add(total, converted)
	}
	return total.FloatString(4), nil
}

func nonNegativeCreditRat(name, value string) (*big.Rat, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "/eE+") {
		return nil, fmt.Errorf("modelbilling: %s must be a non-negative decimal", name)
	}
	parsed, ok := new(big.Rat).SetString(value)
	if !ok || parsed.Sign() < 0 {
		return nil, fmt.Errorf("modelbilling: %s must be a non-negative decimal", name)
	}
	return parsed, nil
}

func positiveCreditRat(name, value string) (*big.Rat, error) {
	parsed, err := nonNegativeCreditRat(name, value)
	if err != nil || parsed.Sign() <= 0 {
		return nil, fmt.Errorf("modelbilling: %s must be a positive decimal", name)
	}
	return parsed, nil
}

// QuotaSettlement is the normalized authoritative cloud ledger result. All
// amount pointers deliberately distinguish an absent fact from a legal zero.
type QuotaSettlement struct {
	SettlementID                        string               `json:"settlement_id"`
	TaskID                              string               `json:"task_id"`
	Status                              QuotaSettlementState `json:"status"`
	TaskTotalQuota                      *DecimalString       `json:"task_total_quota"`
	QuotaPolicyVersion                  *string              `json:"quota_policy_version,omitempty"`
	QuotaPerUnit                        *DecimalString       `json:"quota_per_unit"`
	USDExchangeRate                     *DecimalString       `json:"usd_exchange_rate"`
	CreditPerCNY                        *DecimalString       `json:"credit_per_cny"`
	ChargedCredits                      *DecimalString       `json:"charged_credits"`
	ConsumedFrozenAmount                *DecimalString       `json:"consumed_frozen_amount"`
	ConsumedFrozenFreeAmount            *DecimalString       `json:"consumed_frozen_free_amount"`
	ConsumedFrozenSubscriptionAmount    *DecimalString       `json:"consumed_frozen_subscription_amount"`
	AdditionalChargedAmount             *DecimalString       `json:"additional_charged_amount"`
	AdditionalChargedFreeAmount         *DecimalString       `json:"additional_charged_free_amount"`
	AdditionalChargedSubscriptionAmount *DecimalString       `json:"additional_charged_subscription_amount"`
	ReleasedFrozenAmount                *DecimalString       `json:"released_frozen_amount"`
	ReleasedFrozenFreeAmount            *DecimalString       `json:"released_frozen_free_amount"`
	ReleasedFrozenSubscriptionAmount    *DecimalString       `json:"released_frozen_subscription_amount"`
	UncoveredAmount                     *DecimalString       `json:"uncovered_amount"`
	Balance                             *DecimalString       `json:"balance"`
	BalanceVersion                      *uint64              `json:"balance_version"`
	ErrorCode                           *string              `json:"error_code,omitempty"`
}

// QuotaSettlementResult keeps the settlement scalar balance and the
// authoritative balance object together so their equality can be checked.
type QuotaSettlementResult struct {
	Settlement QuotaSettlement        `json:"settlement"`
	Balance    *cloud.BalanceSnapshot `json:"balance"`
}

// QuotaSettlementStore is the durable local cache used before workflow/task
// state advances. Exact retries are idempotent and accepted state changes are
// retained as immutable revisions by the SQLite implementation.
type QuotaSettlementStore interface {
	SaveQuotaSettlement(context.Context, TaskScope, QuotaSettlementResult, time.Time) error
	CurrentQuotaSettlement(context.Context, TaskScope, string) (QuotaSettlementResult, bool, error)
}

func (s QuotaSettlement) Validate() error {
	if err := requireTrimmed("settlement_id", s.SettlementID); err != nil {
		return err
	}
	if err := requireTrimmed("task_id", s.TaskID); err != nil {
		return err
	}
	if err := validateQuotaSettlementDecimals(s); err != nil {
		return err
	}
	if s.QuotaPolicyVersion != nil {
		if err := requireTrimmed("quota_policy_version", *s.QuotaPolicyVersion); err != nil {
			return err
		}
	}
	if s.ErrorCode != nil {
		if err := requireTrimmed("error_code", *s.ErrorCode); err != nil {
			return err
		}
	}
	if s.BalanceVersion != nil && *s.BalanceVersion == 0 {
		return newError(ErrorContractInvalid, "balance_version must be positive when present")
	}
	return validateQuotaSettlementStatus(s)
}

func validateQuotaSettlementDecimals(settlement QuotaSettlement) error {
	for _, value := range settlement.decimals() {
		if value != nil {
			if err := value.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateQuotaSettlementStatus(settlement QuotaSettlement) error {
	switch settlement.Status {
	case QuotaSettlementPending:
	case QuotaSettlementManualRequired:
		if settlement.ErrorCode == nil {
			return newError(ErrorContractInvalid, "manual settlement requires error_code")
		}
	case QuotaSettlementSettled, QuotaSettlementSettledUncovered:
		return validateTerminalQuotaSettlement(settlement)
	default:
		return newError(ErrorContractInvalid, "unknown quota settlement status")
	}
	return nil
}

func validateTerminalQuotaSettlement(settlement QuotaSettlement) error {
	if settlement.TaskTotalQuota == nil || settlement.QuotaPolicyVersion == nil || !settlement.hasAllLedgerFacts() ||
		settlement.BalanceVersion == nil || *settlement.BalanceVersion == 0 {
		return newError(ErrorContractInvalid, "terminal settlement requires every normalized ledger field")
	}
	if !terminalQuotaSettlementParametersValid(settlement) {
		return newError(ErrorSettlementBalanceInconsistent, "terminal settlement parameter snapshot is invalid")
	}
	return settlement.validateTerminalAmounts()
}

func terminalQuotaSettlementParametersValid(settlement QuotaSettlement) bool {
	return decimalValuePositive(*settlement.QuotaPerUnit) &&
		decimalValuePositive(*settlement.USDExchangeRate) &&
		decimalValuesEqual(*settlement.CreditPerCNY, QuotaSettlementCreditPerCNY)
}

func decimalValuePositive(value DecimalString) bool {
	parsed, ok := new(big.Rat).SetString(string(value))
	return ok && parsed.Sign() > 0
}

func (r QuotaSettlementResult) Validate() error {
	if err := r.Settlement.Validate(); err != nil {
		return err
	}
	if r.Balance == nil {
		return nil
	}
	if err := r.Balance.Validate(); err != nil {
		return newError(ErrorSettlementBalanceInconsistent, err.Error())
	}
	if err := validateQuotaSettlementOuterBalance(r.Balance); err != nil {
		return err
	}
	return validateQuotaSettlementBalanceSnapshot(r)
}

func validateQuotaSettlementOuterBalance(balance *cloud.BalanceSnapshot) error {
	for _, value := range []string{
		balance.TotalBalance, balance.FreeBalance, balance.SubscriptionBalance,
		balance.AvailableBalance, balance.FreeAvailableBalance, balance.SubscriptionAvailableBalance,
		balance.FrozenBalance, balance.FreeFrozenBalance, balance.SubscriptionFrozenBalance,
	} {
		if _, err := NewDecimalString(value); err != nil {
			return newError(ErrorSettlementBalanceInconsistent, "outer balance contains an invalid amount")
		}
	}
	return nil
}

func validateQuotaSettlementBalanceSnapshot(result QuotaSettlementResult) error {
	if result.Settlement.Balance == nil || result.Settlement.BalanceVersion == nil {
		return nil
	}
	if result.Balance.Version > *result.Settlement.BalanceVersion {
		// The settlement scalar is the balance at settlement commit. The outer
		// object is the latest authoritative account snapshot and may already
		// include later credits or charges.
		return nil
	}
	outer, err := NewDecimalString(result.Balance.TotalBalance)
	if err != nil || !decimalValuesEqual(*result.Settlement.Balance, outer) ||
		result.Balance.Version != *result.Settlement.BalanceVersion {
		return newError(ErrorSettlementBalanceInconsistent, "settlement and balance snapshot disagree")
	}
	return nil
}

// ValidateTaskTotalQuota binds the cloud settlement to the accepted final
// report. Pending/manual responses may omit this B5-owned field, but a value
// that is present must never disagree with the accepted final summary.
func (s QuotaSettlement) ValidateTaskTotalQuota(expected DecimalString) error {
	if err := expected.Validate(); err != nil {
		return err
	}
	if s.TaskTotalQuota != nil && !decimalValuesEqual(*s.TaskTotalQuota, expected) {
		return newError(ErrorQuotaSummaryMismatch, "settlement task_total_quota does not match accepted final")
	}
	return nil
}

func (s QuotaSettlement) ValidateForTask(task ModelTask) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := validateQuotaSettlementTaskBinding(s, task); err != nil {
		return err
	}
	if err := task.Freeze.Validate(); err != nil {
		return newError(ErrorSettlementBalanceInconsistent, err.Error())
	}
	if !quotaSettlementRequiresConservation(s.Status) {
		return nil
	}
	freezeAmount, freeFrozen, subscriptionFrozen, err := quotaSettlementFrozenAmounts(task)
	if err != nil {
		return err
	}
	if err := validateQuotaSettlementConservation(s, freezeAmount, freeFrozen, subscriptionFrozen); err != nil {
		return err
	}
	return validateQuotaSettlementAllocation(s)
}

func quotaSettlementRequiresConservation(status QuotaSettlementState) bool {
	return status == QuotaSettlementSettled || status == QuotaSettlementSettledUncovered
}

func validateQuotaSettlementTaskBinding(settlement QuotaSettlement, task ModelTask) error {
	if task.BillingSource != BillingSubscriptionPool || task.TaskID != settlement.TaskID || task.Freeze == nil ||
		task.Scope.CloudBinding == nil || task.Freeze.TaskID != task.TaskID ||
		task.Freeze.Scope != *task.Scope.CloudBinding {
		return newError(ErrorScopeMismatch, "quota settlement does not match subscription task and freeze")
	}
	return nil
}

func quotaSettlementFrozenAmounts(task ModelTask) (DecimalString, DecimalString, DecimalString, error) {
	freezeAmount, err := NewDecimalString(task.Freeze.FreezeAmount)
	if err != nil {
		return "", "", "", newError(ErrorSettlementBalanceInconsistent, "invalid frozen amount")
	}
	freeFrozen, err := NewDecimalString(task.Freeze.FreeFrozenAmount)
	if err != nil {
		return "", "", "", newError(ErrorSettlementBalanceInconsistent, "invalid free frozen amount")
	}
	subscriptionFrozen, err := NewDecimalString(task.Freeze.SubscriptionFrozenAmount)
	if err != nil {
		return "", "", "", newError(ErrorSettlementBalanceInconsistent, "invalid subscription frozen amount")
	}
	return freezeAmount, freeFrozen, subscriptionFrozen, nil
}

func validateQuotaSettlementConservation(
	settlement QuotaSettlement,
	freezeAmount, freeFrozen, subscriptionFrozen DecimalString,
) error {
	checks := []struct {
		left  []*DecimalString
		right DecimalString
	}{
		{[]*DecimalString{settlement.ConsumedFrozenAmount, settlement.ReleasedFrozenAmount}, freezeAmount},
		{[]*DecimalString{settlement.ConsumedFrozenAmount, settlement.AdditionalChargedAmount, settlement.UncoveredAmount}, *settlement.ChargedCredits},
		{[]*DecimalString{settlement.ConsumedFrozenFreeAmount, settlement.ConsumedFrozenSubscriptionAmount}, *settlement.ConsumedFrozenAmount},
		{[]*DecimalString{settlement.AdditionalChargedFreeAmount, settlement.AdditionalChargedSubscriptionAmount}, *settlement.AdditionalChargedAmount},
		{[]*DecimalString{settlement.ReleasedFrozenFreeAmount, settlement.ReleasedFrozenSubscriptionAmount}, *settlement.ReleasedFrozenAmount},
		{[]*DecimalString{settlement.ConsumedFrozenFreeAmount, settlement.ReleasedFrozenFreeAmount}, freeFrozen},
		{[]*DecimalString{settlement.ConsumedFrozenSubscriptionAmount, settlement.ReleasedFrozenSubscriptionAmount}, subscriptionFrozen},
	}
	for _, check := range checks {
		if !decimalPointerSumEqual(check.left, check.right) {
			return newError(ErrorSettlementBalanceInconsistent, "settlement amount conservation failed")
		}
	}
	return nil
}

func validateQuotaSettlementAllocation(settlement QuotaSettlement) error {
	zero := DecimalString("0")
	if !decimalValuesEqual(*settlement.ReleasedFrozenAmount, zero) &&
		(!decimalValuesEqual(*settlement.AdditionalChargedAmount, zero) || !decimalValuesEqual(*settlement.UncoveredAmount, zero)) {
		return newError(ErrorSettlementBalanceInconsistent, "released freeze cannot coexist with additional charge or uncovered amount")
	}
	if !decimalValuesEqual(*settlement.ConsumedFrozenSubscriptionAmount, zero) &&
		!decimalValuesEqual(*settlement.ReleasedFrozenFreeAmount, zero) {
		return newError(ErrorSettlementBalanceInconsistent, "free frozen credits must be consumed before subscription credits")
	}
	return nil
}

func (s QuotaSettlement) decimals() []*DecimalString {
	return []*DecimalString{
		s.TaskTotalQuota, s.QuotaPerUnit, s.USDExchangeRate, s.CreditPerCNY, s.ChargedCredits,
		s.ConsumedFrozenAmount, s.ConsumedFrozenFreeAmount, s.ConsumedFrozenSubscriptionAmount,
		s.AdditionalChargedAmount, s.AdditionalChargedFreeAmount, s.AdditionalChargedSubscriptionAmount,
		s.ReleasedFrozenAmount, s.ReleasedFrozenFreeAmount, s.ReleasedFrozenSubscriptionAmount,
		s.UncoveredAmount, s.Balance,
	}
}

func (s QuotaSettlement) hasAllLedgerFacts() bool {
	for _, value := range s.decimals()[1:] {
		if value == nil {
			return false
		}
	}
	return true
}

func (s QuotaSettlement) validateTerminalAmounts() error {
	zero := DecimalString("0")
	if s.Status == QuotaSettlementSettled {
		if !decimalValuesEqual(*s.UncoveredAmount, zero) {
			return newError(ErrorSettlementBalanceInconsistent, "settled result must have zero uncovered amount")
		}
		return nil
	}
	uncovered, uncoveredOK := new(big.Rat).SetString(string(*s.UncoveredAmount))
	balance, balanceOK := new(big.Rat).SetString(string(*s.Balance))
	if !uncoveredOK || uncovered.Sign() <= 0 || !balanceOK || balance.Sign() < 0 {
		return newError(ErrorSettlementBalanceInconsistent, "uncovered settlement requires positive uncovered amount and non-negative balance")
	}
	return nil
}

func decimalPointerSumEqual(values []*DecimalString, expected DecimalString) bool {
	total := new(big.Rat)
	for _, value := range values {
		if value == nil {
			return false
		}
		part, ok := new(big.Rat).SetString(string(*value))
		if !ok {
			return false
		}
		total.Add(total, part)
	}
	right, ok := new(big.Rat).SetString(string(expected))
	return ok && total.Cmp(right) == 0
}

func decimalValuesEqual(left, right DecimalString) bool {
	if strings.TrimSpace(string(left)) == "" || strings.TrimSpace(string(right)) == "" {
		return false
	}
	l, lok := new(big.Rat).SetString(string(left))
	r, rok := new(big.Rat).SetString(string(right))
	return lok && rok && l.Cmp(r) == 0
}
