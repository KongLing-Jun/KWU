package modelbilling

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"

	"kwu/internal/cloud"
)

const DependencyCloudQuota = "CLOUD-QUOTA-01"

type QuotaFixtureBundle struct {
	Version      string                       `json:"fixture_version"`
	DependencyID string                       `json:"dependency_id"`
	Authenticity string                       `json:"authenticity"`
	Scope        cloud.BindingScope           `json:"scope"`
	Cases        []QuotaSettlementFixtureCase `json:"cases"`
}

type QuotaSettlementFixtureCase struct {
	Name   string                 `json:"name"`
	Result *QuotaSettlementResult `json:"result,omitempty"`
	Error  *FixtureError          `json:"error,omitempty"`
}

//go:embed testdata/CLOUD-QUOTA-01/normalized-v1.json
var embeddedQuotaFixture []byte

func LoadEmbeddedQuotaFixture() (QuotaFixtureBundle, error) {
	return DecodeQuotaFixture(bytes.NewReader(embeddedQuotaFixture))
}

func DecodeQuotaFixture(reader io.Reader) (QuotaFixtureBundle, error) {
	payload, err := io.ReadAll(reader)
	if err != nil {
		return QuotaFixtureBundle{}, err
	}
	bundle, err := decodeStrict[QuotaFixtureBundle](bytes.NewReader(payload))
	if err != nil {
		return bundle, err
	}
	if err := validateQuotaFixturePresence(payload); err != nil {
		return QuotaFixtureBundle{}, err
	}
	if err := validateQuotaFixtureHeader(bundle); err != nil {
		return QuotaFixtureBundle{}, err
	}
	if err := bundle.Scope.Validate(); err != nil {
		return QuotaFixtureBundle{}, err
	}
	if err := validateQuotaFixtureCases(bundle.Cases); err != nil {
		return QuotaFixtureBundle{}, err
	}
	return bundle, nil
}

func validateQuotaFixtureHeader(bundle QuotaFixtureBundle) error {
	if bundle.Version != QuotaContractV1 || bundle.DependencyID != DependencyCloudQuota ||
		bundle.Authenticity != "MOCK_ONLY" || len(bundle.Cases) == 0 {
		return fmt.Errorf("modelbilling: invalid quota fixture header")
	}
	return nil
}

func validateQuotaFixtureCases(fixtures []QuotaSettlementFixtureCase) error {
	seen := make(map[string]bool, len(fixtures))
	for index, fixture := range fixtures {
		if err := validateQuotaFixtureCase(index, fixture, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateQuotaFixtureCase(index int, fixture QuotaSettlementFixtureCase, seen map[string]bool) error {
	if invalidQuotaFixtureCase(fixture, seen) {
		return fixtureCaseError(index, fmt.Errorf("invalid quota fixture case"))
	}
	seen[fixture.Name] = true
	return validateQuotaFixtureOutcome(index, fixture)
}

func invalidQuotaFixtureCase(fixture QuotaSettlementFixtureCase, seen map[string]bool) bool {
	return strings.TrimSpace(fixture.Name) == "" || fixture.Name != strings.TrimSpace(fixture.Name) || seen[fixture.Name] ||
		(fixture.Result == nil) == (fixture.Error == nil)
}

func validateQuotaFixtureOutcome(index int, fixture QuotaSettlementFixtureCase) error {
	if fixture.Result == nil {
		if err := fixture.Error.Validate(); err != nil {
			return fixtureCaseError(index, err)
		}
		return nil
	}
	if err := fixture.Result.Validate(); err != nil {
		return fixtureCaseError(index, err)
	}
	if err := validateQuotaFixtureExactCharge(fixture.Result.Settlement); err != nil {
		return fixtureCaseError(index, err)
	}
	return nil
}

var quotaSettlementFixtureFields = []string{
	"settlement_id", "task_id", "status", "task_total_quota", "quota_per_unit",
	"usd_exchange_rate", "credit_per_cny", "charged_credits", "consumed_frozen_amount",
	"consumed_frozen_free_amount", "consumed_frozen_subscription_amount", "additional_charged_amount",
	"additional_charged_free_amount", "additional_charged_subscription_amount", "released_frozen_amount",
	"released_frozen_free_amount", "released_frozen_subscription_amount", "uncovered_amount", "balance", "balance_version",
}

func validateQuotaFixturePresence(payload []byte) error {
	var root struct {
		Cases []json.RawMessage `json:"cases"`
	}
	if err := json.Unmarshal(payload, &root); err != nil {
		return err
	}
	for index, rawCase := range root.Cases {
		if err := validateQuotaFixtureCasePresence(index, rawCase); err != nil {
			return err
		}
	}
	return nil
}

func validateQuotaFixtureCasePresence(index int, rawCase json.RawMessage) error {
	var caseObject map[string]json.RawMessage
	if err := json.Unmarshal(rawCase, &caseObject); err != nil {
		return fixtureCaseError(index, err)
	}
	resultRaw, hasResult := caseObject["result"]
	if !hasResult || bytes.Equal(bytes.TrimSpace(resultRaw), []byte("null")) {
		return nil
	}
	var resultObject map[string]json.RawMessage
	if err := json.Unmarshal(resultRaw, &resultObject); err != nil {
		return fixtureCaseError(index, err)
	}
	if err := requireQuotaFixtureFields(index, resultObject, []string{"settlement", "balance"}, "result"); err != nil {
		return err
	}
	var settlementObject map[string]json.RawMessage
	if err := json.Unmarshal(resultObject["settlement"], &settlementObject); err != nil {
		return fixtureCaseError(index, err)
	}
	return requireQuotaFixtureFields(index, settlementObject, quotaSettlementFixtureFields, "settlement")
}

func requireQuotaFixtureFields(index int, object map[string]json.RawMessage, fields []string, subject string) error {
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fixtureCaseError(index, fmt.Errorf("quota fixture %s is missing %s", subject, field))
		}
	}
	return nil
}

func validateQuotaFixtureExactCharge(settlement QuotaSettlement) error {
	if !terminalQuotaFixtureStatus(settlement.Status) {
		return nil
	}
	if err := requireQuotaFixtureChargeFields(settlement); err != nil {
		return err
	}
	expected, err := quotaFixtureExpectedCharge(settlement)
	if err != nil {
		return err
	}
	return validateQuotaFixtureChargedCredits(settlement, expected)
}

func terminalQuotaFixtureStatus(status QuotaSettlementState) bool {
	return status == QuotaSettlementSettled || status == QuotaSettlementSettledUncovered
}

func requireQuotaFixtureChargeFields(settlement QuotaSettlement) error {
	values := []*DecimalString{
		settlement.TaskTotalQuota, settlement.QuotaPerUnit, settlement.USDExchangeRate,
		settlement.CreditPerCNY, settlement.ChargedCredits,
	}
	for _, value := range values {
		if value == nil {
			return newError(ErrorContractInvalid, "terminal quota fixture requires charge formula fields")
		}
	}
	return nil
}

func quotaFixtureExpectedCharge(settlement QuotaSettlement) (*big.Rat, error) {
	expected, ok := new(big.Rat).SetString(string(*settlement.TaskTotalQuota))
	if !ok {
		return nil, newError(ErrorContractInvalid, "invalid quota fixture total")
	}
	for _, factor := range []*DecimalString{settlement.USDExchangeRate, settlement.CreditPerCNY} {
		value, parsed := new(big.Rat).SetString(string(*factor))
		if !parsed {
			return nil, newError(ErrorContractInvalid, "invalid quota fixture charge factor")
		}
		expected.Mul(expected, value)
	}
	perUnit, ok := new(big.Rat).SetString(string(*settlement.QuotaPerUnit))
	if !ok || perUnit.Sign() == 0 {
		return nil, newError(ErrorContractInvalid, "invalid quota fixture quota_per_unit")
	}
	expected.Quo(expected, perUnit)
	return expected, nil
}

func validateQuotaFixtureChargedCredits(settlement QuotaSettlement, expected *big.Rat) error {
	charged, ok := new(big.Rat).SetString(string(*settlement.ChargedCredits))
	if !ok || expected.Cmp(charged) != 0 || decimalScale(*settlement.ChargedCredits) > 4 {
		return newError(ErrorContractInvalid, "quota fixture charge must be exact without rounding")
	}
	return nil
}

func decimalScale(value DecimalString) int {
	text := string(value)
	dot := strings.IndexByte(text, '.')
	if dot < 0 {
		return 0
	}
	return len(text) - dot - 1
}

func quotaFixtureCase(bundle QuotaFixtureBundle, name string) (QuotaSettlementFixtureCase, error) {
	if err := requireTrimmed("quota fixture case name", name); err != nil {
		return QuotaSettlementFixtureCase{}, err
	}
	for _, fixture := range bundle.Cases {
		if fixture.Name == name {
			return fixture, nil
		}
	}
	return QuotaSettlementFixtureCase{}, newError(ErrorContractInvalid, "quota fixture case not found")
}
