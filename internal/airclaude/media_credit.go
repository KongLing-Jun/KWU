package airclaude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

type imageTokenUsage struct {
	InputTokens   *int64                 `json:"input_tokens"`
	OutputTokens  *int64                 `json:"output_tokens"`
	InputDetails  *imageTokenUsageDetail `json:"input_tokens_details"`
	OutputDetails *imageTokenUsageDetail `json:"output_tokens_details"`
}

type imageTokenUsageDetail struct {
	TextTokens  *int64 `json:"text_tokens"`
	ImageTokens *int64 `json:"image_tokens"`
}

func (p ImageQuotaPricing) Validate() error {
	input, err := positiveMediaCreditRat("input price", p.InputPerMillion)
	if err != nil {
		return err
	}
	output, err := positiveMediaCreditRat("output price", p.OutputPerMillion)
	if err != nil {
		return err
	}
	if _, err := positiveMediaCreditRat("quota per USD", p.QuotaPerUSD); err != nil {
		return err
	}
	if _, err := optionalMediaCreditRat("image input price", p.ImageInputPerMillion, input); err != nil {
		return err
	}
	_, err = optionalMediaCreditRat("image output price", p.ImageOutputPerMillion, output)
	return err
}

func estimateImageMaaSQuota(payload []byte, pricing ImageQuotaPricing) (string, error) {
	if err := pricing.Validate(); err != nil {
		return "", err
	}
	var envelope struct {
		Usage *imageTokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", err
	}
	if envelope.Usage == nil {
		return "", fmt.Errorf("usage is required when billing.quota is absent")
	}
	inputPrice, err := positiveMediaCreditRat("input price", pricing.InputPerMillion)
	if err != nil {
		return "", err
	}
	outputPrice, err := positiveMediaCreditRat("output price", pricing.OutputPerMillion)
	if err != nil {
		return "", err
	}
	quotaPerUSD, err := positiveMediaCreditRat("quota per USD", pricing.QuotaPerUSD)
	if err != nil {
		return "", err
	}
	imageInputPrice, err := optionalMediaCreditRat("image input price", pricing.ImageInputPerMillion, inputPrice)
	if err != nil {
		return "", err
	}
	imageOutputPrice, err := optionalMediaCreditRat("image output price", pricing.ImageOutputPerMillion, outputPrice)
	if err != nil {
		return "", err
	}
	weighted, err := weightedImageUsage(*envelope.Usage, inputPrice, outputPrice, imageInputPrice, imageOutputPrice)
	if err != nil {
		return "", err
	}
	quota := new(big.Rat).Mul(weighted, quotaPerUSD)
	quota.Quo(quota, big.NewRat(1_000_000, 1))
	return roundNonNegativeRat(quota).String(), nil
}

func weightedImageUsage(
	usage imageTokenUsage,
	inputPrice, outputPrice, imageInputPrice, imageOutputPrice *big.Rat,
) (*big.Rat, error) {
	if err := validateImageUsageSide("input", usage.InputTokens, usage.InputDetails); err != nil {
		return nil, err
	}
	if err := validateImageUsageSide("output", usage.OutputTokens, usage.OutputDetails); err != nil {
		return nil, err
	}
	total := new(big.Rat)
	if usage.InputDetails != nil {
		addTokenCost(total, usage.InputDetails.TextTokens, inputPrice)
		addTokenCost(total, usage.InputDetails.ImageTokens, imageInputPrice)
	} else {
		addTokenCost(total, usage.InputTokens, inputPrice)
	}
	if usage.OutputDetails != nil {
		addTokenCost(total, usage.OutputDetails.TextTokens, outputPrice)
		addTokenCost(total, usage.OutputDetails.ImageTokens, imageOutputPrice)
	} else {
		addTokenCost(total, usage.OutputTokens, outputPrice)
	}
	if total.Sign() <= 0 {
		return nil, fmt.Errorf("usage must contain a positive billed token count")
	}
	return total, nil
}

func validateImageUsageSide(name string, aggregate *int64, detail *imageTokenUsageDetail) error {
	if aggregate != nil && *aggregate < 0 {
		return fmt.Errorf("%s_tokens must be non-negative", name)
	}
	if detail == nil {
		if aggregate == nil {
			return fmt.Errorf("%s token usage is missing", name)
		}
		return nil
	}
	sum := int64(0)
	for _, value := range []*int64{detail.TextTokens, detail.ImageTokens} {
		if value == nil {
			continue
		}
		if *value < 0 {
			return fmt.Errorf("%s token detail must be non-negative", name)
		}
		sum += *value
	}
	if aggregate != nil && sum != *aggregate {
		return fmt.Errorf("%s token detail conflicts with aggregate", name)
	}
	return nil
}

func addTokenCost(total *big.Rat, tokens *int64, price *big.Rat) {
	if tokens == nil || *tokens == 0 {
		return
	}
	total.Add(total, new(big.Rat).Mul(new(big.Rat).SetInt64(*tokens), price))
}

func positiveMediaCreditRat(name, value string) (*big.Rat, error) {
	value = strings.TrimSpace(value)
	parsed, ok := new(big.Rat).SetString(value)
	if !ok || value == "" || strings.ContainsAny(value, "/eE+") || parsed.Sign() <= 0 {
		return nil, fmt.Errorf("%s must be a positive decimal", name)
	}
	return parsed, nil
}

func optionalMediaCreditRat(name, value string, fallback *big.Rat) (*big.Rat, error) {
	if strings.TrimSpace(value) == "" {
		return new(big.Rat).Set(fallback), nil
	}
	return positiveMediaCreditRat(name, value)
}

func roundNonNegativeRat(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(value.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

func mediaResponseQuota(payload []byte) (string, bool, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(payload, &root); err != nil {
		return "", false, err
	}
	billing, billingOK, err := nestedDecimal(root, "billing", "quota")
	if err != nil {
		return "", false, err
	}
	usage, usageOK, err := nestedDecimal(root, "usage", "quota")
	if err != nil {
		return "", false, err
	}
	if billingOK && usageOK {
		left, _ := new(big.Rat).SetString(billing)
		right, _ := new(big.Rat).SetString(usage)
		if left.Cmp(right) != 0 {
			return "", false, fmt.Errorf("billing.quota and usage.quota conflict")
		}
		return billing, true, nil
	}
	if billingOK {
		return billing, true, nil
	}
	return usage, usageOK, nil
}

func nestedDecimal(root map[string]json.RawMessage, parent, field string) (string, bool, error) {
	rawParent, ok := root[parent]
	if !ok {
		return "", false, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(rawParent, &object); err != nil || object == nil {
		return "", false, nil
	}
	raw, ok := object[field]
	if !ok {
		return "", false, nil
	}
	text, err := canonicalNonNegativeDecimal(raw, parent+"."+field)
	return text, true, err
}

func canonicalNonNegativeDecimal(raw json.RawMessage, name string) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var text string
	switch typed := value.(type) {
	case json.Number:
		text = typed.String()
	case string:
		text = typed
	default:
		return "", fmt.Errorf("%s must be a decimal", name)
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsAny(text, "/eE+") {
		return "", fmt.Errorf("%s must be a canonical decimal", name)
	}
	parsed, valid := new(big.Rat).SetString(text)
	if !valid || parsed.Sign() < 0 {
		return "", fmt.Errorf("%s must be a non-negative decimal", name)
	}
	return text, nil
}
