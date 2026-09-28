package newapipricing

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"kwu/internal/modelbilling"
)

type SnapshotHeader struct {
	SnapshotID    string
	Source        string
	SourceHash    string
	Currency      string
	ExchangeRate  *modelbilling.DecimalString
	RechargePrice *modelbilling.DecimalString
	FetchedAt     time.Time
	FreshUntil    time.Time
	StaleUntil    time.Time
	State         modelbilling.PricingState
}

type Vendor struct {
	ID          int
	Name        string
	Description string
	Icon        string
}

type ModelMetadata struct {
	ModelID                string
	ModelName              string
	Description            string
	VendorID               *int
	Vendor                 *Vendor
	EnableGroups           []string
	SupportedEndpointTypes []string
	ModelRatio             *modelbilling.DecimalString
	CompletionRatio        *modelbilling.DecimalString
	CacheRatio             *modelbilling.DecimalString
	CreateCacheRatio       *modelbilling.DecimalString
	ImageRatio             *modelbilling.DecimalString
	AudioRatio             *modelbilling.DecimalString
	AudioCompletionRatio   *modelbilling.DecimalString
	ModelPrice             *modelbilling.DecimalString
}

type ModelIssue struct {
	Index     int
	ModelName string
	Err       error
}

type CatalogAvailability string

const (
	CatalogNoUsablePrices CatalogAvailability = "no_usable_prices"
	CatalogPartial        CatalogAvailability = "partial"
	CatalogComplete       CatalogAvailability = "complete"
)

type Catalog struct {
	SourceVersion *string
	GroupRatios   map[string]modelbilling.DecimalString
	UsableGroups  []string
	Models        []ModelMetadata
	Prices        []modelbilling.ModelPrice
	Issues        []ModelIssue
}

type Adapter struct{}

func (Adapter) Decode(raw []byte) (Catalog, error) {
	envelope, err := decodeWire(raw)
	if err != nil {
		return Catalog{}, err
	}
	if err := validateEnvelope(envelope); err != nil {
		return Catalog{}, err
	}
	return adaptCatalog(envelope)
}

func (a Adapter) Adapt(raw []byte, header SnapshotHeader) (Catalog, modelbilling.PricingSnapshot, error) {
	catalog, err := a.Decode(raw)
	if err != nil {
		return Catalog{}, modelbilling.PricingSnapshot{}, err
	}
	snapshot, err := catalog.Snapshot(header)
	if err != nil {
		return catalog, modelbilling.PricingSnapshot{}, err
	}
	return catalog, snapshot, nil
}

func (c Catalog) Availability() CatalogAvailability {
	if len(c.Prices) == 0 {
		return CatalogNoUsablePrices
	}
	if len(c.Issues) > 0 {
		return CatalogPartial
	}
	return CatalogComplete
}

func (c Catalog) Snapshot(header SnapshotHeader) (modelbilling.PricingSnapshot, error) {
	if c.Availability() == CatalogNoUsablePrices {
		return modelbilling.PricingSnapshot{}, pricingError(false, "newapi pricing response has no usable model prices")
	}
	snapshot := modelbilling.PricingSnapshot{
		SnapshotID: header.SnapshotID, Source: header.Source, SourceVersion: cloneString(c.SourceVersion),
		SourceHash: header.SourceHash, Currency: header.Currency,
		ExchangeRate: cloneDecimal(header.ExchangeRate), RechargePrice: cloneDecimal(header.RechargePrice),
		GroupRatios: cloneGroupRatios(c.GroupRatios), UsableGroups: append([]string(nil), c.UsableGroups...),
		FetchedAt: header.FetchedAt, FreshUntil: header.FreshUntil, StaleUntil: header.StaleUntil,
		State: header.State, Models: cloneModelPrices(c.Prices),
	}
	if err := snapshot.Validate(); err != nil {
		return modelbilling.PricingSnapshot{}, err
	}
	return snapshot, nil
}

func adaptCatalog(envelope pricingWireEnvelope) (Catalog, error) {
	groupRatios, err := adaptGroupRatios(envelope.GroupRatio)
	if err != nil {
		return Catalog{}, err
	}
	version := optionalTrimmed(envelope.PricingVersion)
	catalog := Catalog{
		SourceVersion: version,
		GroupRatios:   groupRatios,
		UsableGroups:  sortedMapKeys(envelope.UsableGroup),
		Models:        make([]ModelMetadata, 0, len(*envelope.Data)),
		Prices:        make([]modelbilling.ModelPrice, 0, len(*envelope.Data)),
	}
	vendors := adaptVendors(envelope.Vendors)
	for index, wireModel := range *envelope.Data {
		metadata, price, err := adaptModel(wireModel, vendors)
		if strings.TrimSpace(wireModel.ModelName) != "" {
			catalog.Models = append(catalog.Models, metadata)
		}
		if err != nil {
			catalog.Issues = append(catalog.Issues, ModelIssue{Index: index, ModelName: strings.TrimSpace(wireModel.ModelName), Err: err})
			continue
		}
		catalog.Prices = append(catalog.Prices, price)
	}
	return catalog, nil
}

func adaptModel(wireModel pricingWireModel, vendors map[int]Vendor) (ModelMetadata, modelbilling.ModelPrice, error) {
	metadata, err := adaptModelMetadata(wireModel, vendors)
	if err != nil {
		return metadata, modelbilling.ModelPrice{}, err
	}
	modelType, capabilities, err := classifyModel(wireModel.SupportedEndpointTypes)
	if err != nil {
		return metadata, modelbilling.ModelPrice{}, err
	}
	quotaType, billingMode, err := mapBillingMode(wireModel)
	if err != nil {
		return metadata, modelbilling.ModelPrice{}, err
	}
	price := modelbilling.ModelPrice{
		ModelID: metadata.ModelID, ProviderModelID: metadata.ModelID, ModelName: metadata.ModelName,
		ModelType: modelType, Capabilities: capabilities, QuotaType: quotaType,
		ModelRatio: metadata.ModelRatio, CompletionRatio: metadata.CompletionRatio,
		CacheRatio: metadata.CacheRatio, CreateCacheRatio: metadata.CreateCacheRatio,
		ImageRatio: metadata.ImageRatio, AudioRatio: metadata.AudioRatio,
		AudioCompletionRatio: metadata.AudioCompletionRatio, ModelPrice: metadata.ModelPrice,
		EnableGroups: append([]string(nil), metadata.EnableGroups...), BillingMode: billingMode,
	}
	if billingMode == modelbilling.BillingModeTieredExpr {
		expression := strings.TrimSpace(wireModel.BillingExpression)
		price.BillingExpression = &expression
	}
	if err := price.Validate(); err != nil {
		return metadata, modelbilling.ModelPrice{}, err
	}
	return metadata, price, nil
}

func adaptModelMetadata(wireModel pricingWireModel, vendors map[int]Vendor) (ModelMetadata, error) {
	name := strings.TrimSpace(wireModel.ModelName)
	metadata := ModelMetadata{
		ModelID: name, ModelName: name, Description: strings.TrimSpace(wireModel.Description),
		VendorID: cloneInt(wireModel.VendorID), EnableGroups: cleanStrings(wireModel.EnableGroups),
		SupportedEndpointTypes: cleanStrings(wireModel.SupportedEndpointTypes),
	}
	if name == "" {
		return metadata, contractError("newapi pricing model_name is required")
	}
	if wireModel.VendorID != nil {
		if vendor, ok := vendors[*wireModel.VendorID]; ok {
			cloned := vendor
			metadata.Vendor = &cloned
		}
	}
	return adaptModelDecimals(metadata, wireModel)
}

func adaptModelDecimals(metadata ModelMetadata, wireModel pricingWireModel) (ModelMetadata, error) {
	var err error
	metadata.ModelRatio, err = adaptDecimal(wireModel.ModelRatio)
	if err == nil {
		metadata.CompletionRatio, err = adaptDecimal(wireModel.CompletionRatio)
	}
	if err == nil {
		metadata.CacheRatio, err = adaptDecimal(wireModel.CacheRatio)
	}
	if err == nil {
		metadata.CreateCacheRatio, err = adaptDecimal(wireModel.CreateCacheRatio)
	}
	if err == nil {
		metadata.ImageRatio, err = adaptDecimal(wireModel.ImageRatio)
	}
	if err == nil {
		metadata.AudioRatio, err = adaptDecimal(wireModel.AudioRatio)
	}
	if err == nil {
		metadata.AudioCompletionRatio, err = adaptDecimal(wireModel.AudioCompletionRatio)
	}
	if err == nil {
		metadata.ModelPrice, err = adaptDecimal(wireModel.ModelPrice)
	}
	return metadata, err
}

func adaptDecimal(number *json.Number) (*modelbilling.DecimalString, error) {
	if number == nil {
		return nil, nil
	}
	canonical, err := canonicalDecimal(number.String())
	if err != nil {
		return nil, err
	}
	decimal, err := modelbilling.NewDecimalString(canonical)
	if err != nil {
		return nil, err
	}
	return &decimal, nil
}

const (
	maxWireDecimalLength = 32
	minWireExponent      = -324
	maxWireExponent      = 308
)

func canonicalDecimal(raw string) (string, error) {
	if err := validateWireDecimal(raw); err != nil {
		return "", err
	}
	mantissa, exponent, err := parseWireDecimalExponent(raw)
	if err != nil {
		return "", err
	}
	if err := validateWireExponent(exponent); err != nil {
		return "", err
	}
	digits, decimalPosition, zero, err := canonicalWireMantissa(raw, mantissa)
	if err != nil {
		return "", err
	}
	if zero {
		return "0", nil
	}
	decimalPosition += exponent
	if err := validateFloat64WireMagnitude(digits, decimalPosition-1); err != nil {
		return "", err
	}
	return renderCanonicalDecimal(digits, decimalPosition), nil
}

func validateWireDecimal(raw string) error {
	if raw == "" || len(raw) > maxWireDecimalLength {
		return contractError("invalid newapi pricing decimal")
	}
	if strings.HasPrefix(raw, "-") {
		return contractError("newapi pricing decimal must be non-negative")
	}
	return nil
}

func parseWireDecimalExponent(raw string) (string, int, error) {
	exponentIndex := strings.IndexAny(raw, "eE")
	if exponentIndex < 0 {
		return raw, 0, nil
	}
	exponentText := raw[exponentIndex+1:]
	if strings.IndexAny(exponentText, "eE") >= 0 {
		return "", 0, contractError("invalid newapi pricing decimal %q", raw)
	}
	exponent, err := strconv.Atoi(exponentText)
	if err != nil {
		return "", 0, contractError("invalid newapi pricing decimal %q", raw)
	}
	return raw[:exponentIndex], exponent, nil
}

func validateWireExponent(exponent int) error {
	if exponent < minWireExponent || exponent > maxWireExponent {
		return contractError("newapi pricing decimal exponent is outside float64 wire range")
	}
	return nil
}

func canonicalWireMantissa(raw, mantissa string) (string, int, bool, error) {
	integer, fraction, hasFraction := strings.Cut(mantissa, ".")
	if integer == "" || (hasFraction && fraction == "") {
		return "", 0, false, contractError("invalid newapi pricing decimal %q", raw)
	}
	digits := integer + fraction
	if strings.Trim(digits, "0123456789") != "" {
		return "", 0, false, contractError("invalid newapi pricing decimal %q", raw)
	}
	leadingZeros := len(digits) - len(strings.TrimLeft(digits, "0"))
	if leadingZeros == len(digits) {
		return "", 0, true, nil
	}
	digits = digits[leadingZeros:]
	return digits, len(integer) - leadingZeros, false, nil
}

func renderCanonicalDecimal(digits string, decimalPosition int) string {
	switch {
	case decimalPosition <= 0:
		return "0." + strings.Repeat("0", -decimalPosition) + digits
	case decimalPosition >= len(digits):
		return digits + strings.Repeat("0", decimalPosition-len(digits))
	default:
		return digits[:decimalPosition] + "." + digits[decimalPosition:]
	}
}

func validateFloat64WireMagnitude(significand string, scientificExponent int) error {
	const (
		minScientificExponent = -324
		maxScientificExponent = 308
		minFloatSignificand   = "49406564584124654"
		maxFloatSignificand   = "17976931348623157"
	)
	if scientificExponent < minScientificExponent || scientificExponent > maxScientificExponent {
		return contractError("newapi pricing decimal is outside float64 wire range")
	}
	if scientificExponent == minScientificExponent && compareSignificands(significand, minFloatSignificand) < 0 {
		return contractError("newapi pricing decimal is below the minimum float64 wire value")
	}
	if scientificExponent == maxScientificExponent && compareSignificands(significand, maxFloatSignificand) > 0 {
		return contractError("newapi pricing decimal exceeds the maximum float64 wire value")
	}
	return nil
}

func compareSignificands(left, right string) int {
	width := max(len(left), len(right))
	left += strings.Repeat("0", width-len(left))
	right += strings.Repeat("0", width-len(right))
	return strings.Compare(left, right)
}

func mapBillingMode(model pricingWireModel) (modelbilling.QuotaType, modelbilling.BillingMode, error) {
	if model.QuotaType == nil {
		return "", "", contractError("newapi pricing model %q quota_type is required", strings.TrimSpace(model.ModelName))
	}
	mode := strings.TrimSpace(model.BillingMode)
	switch *model.QuotaType {
	case wireQuotaToken:
		if mode == "" {
			return modelbilling.QuotaTypeToken, modelbilling.BillingModeTokenRatio, nil
		}
		if mode == string(modelbilling.BillingModeTieredExpr) && strings.TrimSpace(model.BillingExpression) != "" {
			return modelbilling.QuotaTypeToken, modelbilling.BillingModeTieredExpr, nil
		}
	case wireQuotaPerRequest:
		if mode == "" {
			return modelbilling.QuotaTypePerRequest, modelbilling.BillingModePerRequest, nil
		}
	}
	return "", "", contractError("newapi pricing model %q has unsupported quota_type/billing_mode %d/%q", strings.TrimSpace(model.ModelName), *model.QuotaType, mode)
}

func classifyModel(endpoints []string) (string, []string, error) {
	types := map[string]bool{}
	for _, endpoint := range cleanStrings(endpoints) {
		switch endpoint {
		case "openai", "openai-response", "openai-response-compact", "anthropic", "gemini":
			types["chat"] = true
		case "image-generation":
			types["image"] = true
		case "embeddings":
			types["embedding"] = true
		case "jina-rerank":
			types["rerank"] = true
		case "openai-video":
			types["video"] = true
		}
	}
	for _, specialized := range []string{"image", "embedding", "rerank", "video"} {
		if types[specialized] {
			return specialized, []string{specialized}, nil
		}
	}
	if types["chat"] {
		return "chat", []string{"text"}, nil
	}
	return "", nil, contractError("newapi pricing model type cannot be mapped from supported_endpoint_types")
}

func adaptGroupRatios(values map[string]json.Number) (map[string]modelbilling.DecimalString, error) {
	out := make(map[string]modelbilling.DecimalString, len(values))
	for group, number := range values {
		group = strings.TrimSpace(group)
		if group == "" {
			return nil, contractError("newapi pricing group_ratio contains an empty group")
		}
		decimal, err := adaptDecimal(&number)
		if err != nil {
			return nil, err
		}
		out[group] = *decimal
	}
	return out, nil
}

func adaptVendors(values []pricingWireVendor) map[int]Vendor {
	out := make(map[int]Vendor, len(values))
	for _, value := range values {
		out[value.ID] = Vendor{ID: value.ID, Name: strings.TrimSpace(value.Name), Description: strings.TrimSpace(value.Description), Icon: strings.TrimSpace(value.Icon)}
	}
	return out
}

func sortedMapKeys(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func optionalTrimmed(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func cleanRequiredMessage(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneDecimal(value *modelbilling.DecimalString) *modelbilling.DecimalString {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneModelPrices(values []modelbilling.ModelPrice) []modelbilling.ModelPrice {
	out := make([]modelbilling.ModelPrice, len(values))
	for index, value := range values {
		out[index] = cloneModelPrice(value)
	}
	return out
}

func cloneModelPrice(value modelbilling.ModelPrice) modelbilling.ModelPrice {
	value.Capabilities = append([]string(nil), value.Capabilities...)
	value.EnableGroups = append([]string(nil), value.EnableGroups...)
	value.ModelRatio = cloneDecimal(value.ModelRatio)
	value.CompletionRatio = cloneDecimal(value.CompletionRatio)
	value.CacheRatio = cloneDecimal(value.CacheRatio)
	value.CreateCacheRatio = cloneDecimal(value.CreateCacheRatio)
	value.ImageRatio = cloneDecimal(value.ImageRatio)
	value.AudioRatio = cloneDecimal(value.AudioRatio)
	value.AudioCompletionRatio = cloneDecimal(value.AudioCompletionRatio)
	value.ModelPrice = cloneDecimal(value.ModelPrice)
	value.BillingExpression = cloneString(value.BillingExpression)
	value.InputPerMillion = cloneDecimal(value.InputPerMillion)
	value.CacheReadPerMillion = cloneDecimal(value.CacheReadPerMillion)
	value.CacheWritePerMillion = cloneDecimal(value.CacheWritePerMillion)
	value.OutputPerMillion = cloneDecimal(value.OutputPerMillion)
	value.ImageInputPerMillion = cloneDecimal(value.ImageInputPerMillion)
	value.AudioInputPerMillion = cloneDecimal(value.AudioInputPerMillion)
	value.AudioOutputPerMillion = cloneDecimal(value.AudioOutputPerMillion)
	value.RequestPerUnit = cloneDecimal(value.RequestPerUnit)
	return value
}

func cloneGroupRatios(values map[string]modelbilling.DecimalString) map[string]modelbilling.DecimalString {
	out := make(map[string]modelbilling.DecimalString, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func (i ModelIssue) Error() string {
	if i.ModelName == "" {
		return fmt.Sprintf("model[%d]: %v", i.Index, i.Err)
	}
	return fmt.Sprintf("model[%d] %s: %v", i.Index, i.ModelName, i.Err)
}
