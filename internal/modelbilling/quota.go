package modelbilling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	QuotaContractV1       = "quota-contract-v1"
	LegacyTokenContractV1 = "legacy-token-v1"
)

type MaasEndpoint string

const (
	MaasEndpointChatCompletions MaasEndpoint = "/v1/chat/completions"
	MaasEndpointResponses       MaasEndpoint = "/v1/responses"
)

func (e MaasEndpoint) Validate() error {
	switch e {
	case MaasEndpointChatCompletions, MaasEndpointResponses:
		return nil
	default:
		return newError(ErrorContractInvalid, "unsupported Maas endpoint")
	}
}

type QuotaJSONType string

const (
	QuotaJSONNumber QuotaJSONType = "number"
	QuotaJSONString QuotaJSONType = "string"
	QuotaJSONNull   QuotaJSONType = "null"
	QuotaJSONBool   QuotaJSONType = "bool"
	QuotaJSONObject QuotaJSONType = "object"
	QuotaJSONArray  QuotaJSONType = "array"
)

type QuotaRawValue struct {
	Present bool            `json:"present"`
	Type    QuotaJSONType   `json:"json_type"`
	Value   json.RawMessage `json:"value"`
}

func (v QuotaRawValue) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	type rawValue struct {
		Present  bool             `json:"present"`
		JSONType *QuotaJSONType   `json:"json_type"`
		Value    *json.RawMessage `json:"value"`
	}
	if !v.Present {
		return json.Marshal(rawValue{Present: false})
	}
	typeName := v.Type
	value := append(json.RawMessage(nil), v.Value...)
	return json.Marshal(rawValue{Present: true, JSONType: &typeName, Value: &value})
}

func (v *QuotaRawValue) UnmarshalJSON(data []byte) error {
	if v == nil {
		return newError(ErrorContractInvalid, "quota raw value target is required")
	}
	candidate, err := decodeQuotaRawValue(data)
	if err != nil {
		return err
	}
	*v = candidate
	return nil
}

func decodeQuotaRawValue(data []byte) (QuotaRawValue, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return QuotaRawValue{}, newError(ErrorContractInvalid, "quota raw value must be an object")
	}
	if len(fields) != 3 || fields["present"] == nil || fields["json_type"] == nil || fields["value"] == nil {
		return QuotaRawValue{}, newError(ErrorContractInvalid, "quota raw value requires present, json_type, and value")
	}
	return decodeQuotaRawFields(fields)
}

func decodeQuotaRawFields(fields map[string]json.RawMessage) (QuotaRawValue, error) {
	var present bool
	var jsonType QuotaJSONType
	if err := json.Unmarshal(fields["present"], &present); err != nil {
		return QuotaRawValue{}, newError(ErrorContractInvalid, "quota raw present must be a boolean")
	}
	if !bytes.Equal(bytes.TrimSpace(fields["json_type"]), []byte("null")) {
		if err := json.Unmarshal(fields["json_type"], &jsonType); err != nil {
			return QuotaRawValue{}, newError(ErrorContractInvalid, "quota raw json_type must be a string or null")
		}
	}
	value := append(json.RawMessage(nil), fields["value"]...)
	if !present && jsonType == "" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		value = nil
	}
	candidate := QuotaRawValue{Present: present, Type: jsonType, Value: value}
	if err := candidate.Validate(); err != nil {
		return QuotaRawValue{}, err
	}
	return candidate, nil
}

func (v QuotaRawValue) Validate() error {
	if !v.Present {
		return validateAbsentQuotaRawValue(v)
	}
	return validatePresentQuotaRawValue(v)
}

func validateAbsentQuotaRawValue(value QuotaRawValue) error {
	if value.Type != "" || len(value.Value) != 0 {
		return newError(ErrorContractInvalid, "absent quota raw value cannot contain type or value")
	}
	return nil
}

func validatePresentQuotaRawValue(value QuotaRawValue) error {
	if !validQuotaJSONType(value.Type) || len(value.Value) == 0 || !json.Valid(value.Value) {
		return newError(ErrorContractInvalid, "present quota raw value requires valid type and JSON value")
	}
	actual, err := quotaJSONType(value.Value)
	if err != nil || actual != value.Type {
		return newError(ErrorContractInvalid, "quota raw value type does not match its JSON value")
	}
	normalized, err := normalizeQuotaRawJSON(value.Value)
	if err != nil || !bytes.Equal(normalized.Value, value.Value) {
		return newError(ErrorContractInvalid, "quota raw JSON value is not normalized")
	}
	return nil
}

func validQuotaJSONType(value QuotaJSONType) bool {
	switch value {
	case QuotaJSONNumber, QuotaJSONString, QuotaJSONNull, QuotaJSONBool, QuotaJSONObject, QuotaJSONArray:
		return true
	default:
		return false
	}
}

type QuotaSource string

const (
	QuotaSourceBilling        QuotaSource = "billing.quota"
	QuotaSourceUsage          QuotaSource = "usage.quota"
	QuotaSourceBothConsistent QuotaSource = "both_consistent"
)

type QuotaEvidenceStatus string

const (
	QuotaEvidenceComplete    QuotaEvidenceStatus = "complete"
	QuotaEvidenceMissing     QuotaEvidenceStatus = "missing"
	QuotaEvidenceConflicting QuotaEvidenceStatus = "conflicting"
	QuotaEvidenceInvalid     QuotaEvidenceStatus = "invalid"
)

type QuotaEvidenceIssue string

const (
	QuotaIssueRequestIDMissing QuotaEvidenceIssue = "request_id_missing"
	QuotaIssueMissing          QuotaEvidenceIssue = "quota_missing"
	QuotaIssueInvalid          QuotaEvidenceIssue = "quota_invalid"
	QuotaIssueConflicting      QuotaEvidenceIssue = "quota_conflicting"
)

var quotaEvidenceIssueOrder = []QuotaEvidenceIssue{
	QuotaIssueRequestIDMissing,
	QuotaIssueMissing,
	QuotaIssueInvalid,
	QuotaIssueConflicting,
}

type QuotaEvidenceRevision struct {
	Revision        uint64               `json:"evidence_revision"`
	OneAPIRequestID *string              `json:"oneapi_request_id"`
	BillingQuotaRaw QuotaRawValue        `json:"billing_quota_raw"`
	UsageQuotaRaw   QuotaRawValue        `json:"usage_quota_raw"`
	Quota           *DecimalString       `json:"quota"`
	Source          *QuotaSource         `json:"quota_source"`
	Status          QuotaEvidenceStatus  `json:"evidence_status"`
	Issues          []QuotaEvidenceIssue `json:"evidence_issues"`
	ObservedAt      time.Time            `json:"evidence_observed_at"`
}

func (e QuotaEvidenceRevision) Validate() error {
	if err := validateQuotaEvidenceIdentity(e); err != nil {
		return err
	}
	if err := e.BillingQuotaRaw.Validate(); err != nil {
		return err
	}
	if err := e.UsageQuotaRaw.Validate(); err != nil {
		return err
	}
	if !quotaEvidenceProjectionMatches(e) {
		return newError(ErrorContractInvalid, "quota evidence projection does not match raw evidence")
	}
	return nil
}

func validateQuotaEvidenceIdentity(e QuotaEvidenceRevision) error {
	if e.Revision == 0 || e.ObservedAt.IsZero() || e.ObservedAt.Location() != time.UTC {
		return newError(ErrorContractInvalid, "quota evidence requires positive revision and UTC observed_at")
	}
	if e.OneAPIRequestID != nil {
		if err := requireTrimmed("oneapi_request_id", *e.OneAPIRequestID); err != nil {
			return err
		}
	}
	if e.Issues == nil {
		return newError(ErrorContractInvalid, "quota evidence issues must be an array")
	}
	return nil
}

func quotaEvidenceProjectionMatches(e QuotaEvidenceRevision) bool {
	expected := deriveQuotaEvidence(e.OneAPIRequestID, e.BillingQuotaRaw, e.UsageQuotaRaw)
	return optionalStringEqual(expected.OneAPIRequestID, e.OneAPIRequestID) &&
		optionalDecimalEqual(expected.Quota, e.Quota) && optionalQuotaSourceEqual(expected.Source, e.Source) &&
		expected.Status == e.Status && quotaEvidenceIssuesEqual(expected.Issues, e.Issues)
}

type QuotaReportStatus string

const (
	QuotaReportUnreported      QuotaReportStatus = "unreported"
	QuotaReportPartialReported QuotaReportStatus = "partial_reported"
	QuotaReportFinalPending    QuotaReportStatus = "final_pending"
	QuotaReportFinalReported   QuotaReportStatus = "final_reported"
)

func (s QuotaReportStatus) Validate() error {
	switch s {
	case QuotaReportUnreported, QuotaReportPartialReported, QuotaReportFinalPending, QuotaReportFinalReported:
		return nil
	default:
		return newError(ErrorContractInvalid, "invalid quota report status")
	}
}

type QuotaObservationInput struct {
	Endpoint   MaasEndpoint
	Headers    map[string][]string
	FinalJSON  json.RawMessage
	ObservedAt time.Time
}

// NormalizeQuotaEvidence converts one Maas HTTP response into revision 1.
// Stream adapters pass the final response object after the stream completes;
// the real event/wire mapping remains outside this normalized contract.
func NormalizeQuotaEvidence(input QuotaObservationInput) (QuotaEvidenceRevision, error) {
	revisions, err := NormalizeQuotaEvidenceRevisions(input)
	if err != nil {
		return QuotaEvidenceRevision{}, err
	}
	if len(revisions) != 1 {
		return QuotaEvidenceRevision{}, newError(ErrorContractInvalid, "multiple Maas request IDs require revision audit")
	}
	return revisions[0], nil
}

// NormalizeQuotaEvidenceRevisions preserves every distinct request ID observed
// on one response. A normal response yields one revision; conflicting header
// values yield consecutive revisions so the dispatcher can persist the full
// ambiguity instead of reducing it to an error string.
func NormalizeQuotaEvidenceRevisions(input QuotaObservationInput) ([]QuotaEvidenceRevision, error) {
	if err := input.Endpoint.Validate(); err != nil {
		return nil, err
	}
	if input.ObservedAt.IsZero() {
		return nil, newError(ErrorContractInvalid, "quota evidence observed_at is required")
	}
	requestIDs, err := oneAPIRequestIDs(input.Headers)
	if err != nil {
		return nil, err
	}
	billing, usage, err := extractQuotaRawValues(input.FinalJSON)
	if err != nil {
		return nil, err
	}
	if len(requestIDs) == 0 {
		requestIDs = []*string{nil}
	}
	revisions := make([]QuotaEvidenceRevision, len(requestIDs))
	for i, requestID := range requestIDs {
		evidence := deriveQuotaEvidence(requestID, billing, usage)
		evidence.Revision = uint64(i + 1)
		evidence.ObservedAt = input.ObservedAt.UTC()
		if err := evidence.Validate(); err != nil {
			return nil, err
		}
		revisions[i] = evidence
	}
	return revisions, nil
}

func oneAPIRequestIDs(headers map[string][]string) ([]*string, error) {
	names := make([]string, 0)
	for name := range headers {
		if strings.EqualFold(name, "X-Oneapi-Request-Id") {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		left, right := strings.ToLower(names[i]), strings.ToLower(names[j])
		if left == right {
			return names[i] < names[j]
		}
		return left < right
	})
	seen := make(map[string]bool)
	requestIDs := make([]*string, 0)
	for _, name := range names {
		for _, value := range headers[name] {
			if value == "" {
				continue
			}
			if strings.TrimSpace(value) != value {
				return nil, newError(ErrorContractInvalid, "Maas request ID must be canonical and non-empty")
			}
			if seen[value] {
				continue
			}
			seen[value] = true
			copy := value
			requestIDs = append(requestIDs, &copy)
		}
	}
	return requestIDs, nil
}

func extractQuotaRawValues(payload json.RawMessage) (QuotaRawValue, QuotaRawValue, error) {
	if len(payload) == 0 || !json.Valid(payload) {
		return QuotaRawValue{}, QuotaRawValue{}, newError(ErrorContractInvalid, "final Maas response must be valid JSON")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(payload, &root); err != nil || root == nil {
		return QuotaRawValue{}, QuotaRawValue{}, newError(ErrorContractInvalid, "final Maas response must be a JSON object")
	}
	billing, err := quotaRawAt(root, "billing")
	if err != nil {
		return QuotaRawValue{}, QuotaRawValue{}, err
	}
	usage, err := quotaRawAt(root, "usage")
	if err != nil {
		return QuotaRawValue{}, QuotaRawValue{}, err
	}
	return billing, usage, nil
}

func quotaRawAt(root map[string]json.RawMessage, parent string) (QuotaRawValue, error) {
	rawParent, ok := root[parent]
	if !ok {
		return QuotaRawValue{}, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(rawParent, &object); err != nil || object == nil {
		return QuotaRawValue{}, nil
	}
	raw, ok := object["quota"]
	if !ok {
		return QuotaRawValue{}, nil
	}
	return normalizeQuotaRawJSON(raw)
}

func normalizeQuotaRawJSON(raw json.RawMessage) (QuotaRawValue, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return QuotaRawValue{}, newError(ErrorContractInvalid, "quota raw value must be valid JSON")
	}
	typeName, err := quotaJSONType(raw)
	if err != nil {
		return QuotaRawValue{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return QuotaRawValue{}, newError(ErrorContractInvalid, "decode quota raw value")
	}
	value, err = normalizeJSONNumbers(value)
	if err != nil {
		return QuotaRawValue{}, err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return QuotaRawValue{}, newError(ErrorContractInvalid, "encode quota raw value")
	}
	return QuotaRawValue{Present: true, Type: typeName, Value: canonical}, nil
}

func quotaJSONType(raw json.RawMessage) (QuotaJSONType, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", newError(ErrorContractInvalid, "quota raw value is empty")
	}
	switch trimmed[0] {
	case '"':
		return QuotaJSONString, nil
	case 'n':
		return QuotaJSONNull, nil
	case 't', 'f':
		return QuotaJSONBool, nil
	case '{':
		return QuotaJSONObject, nil
	case '[':
		return QuotaJSONArray, nil
	default:
		return QuotaJSONNumber, nil
	}
}

func normalizeJSONNumbers(value any) (any, error) {
	switch typed := value.(type) {
	case json.Number:
		canonical, err := canonicalDecimalNumber(typed.String())
		if err != nil {
			return nil, err
		}
		return json.Number(canonical), nil
	case []any:
		for i := range typed {
			next, err := normalizeJSONNumbers(typed[i])
			if err != nil {
				return nil, err
			}
			typed[i] = next
		}
	case map[string]any:
		for key, child := range typed {
			next, err := normalizeJSONNumbers(child)
			if err != nil {
				return nil, err
			}
			typed[key] = next
		}
	}
	return value, nil
}

func canonicalDecimalNumber(raw string) (string, error) {
	negative, mantissa, exponent, err := parseDecimalNumber(raw)
	if err != nil {
		return "", err
	}
	digits, decimalPosition := decimalDigits(mantissa, exponent)
	digits, decimalPosition, zero := trimDecimalDigits(digits, decimalPosition)
	if zero {
		return "0", nil
	}
	canonical := formatDecimalDigits(digits, decimalPosition)
	if negative {
		canonical = "-" + canonical
	}
	return canonical, nil
}

func parseDecimalNumber(raw string) (bool, string, int, error) {
	negative := strings.HasPrefix(raw, "-")
	if negative {
		raw = raw[1:]
	}
	mantissa, exponentText := raw, ""
	if index := strings.IndexAny(raw, "eE"); index >= 0 {
		mantissa, exponentText = raw[:index], raw[index+1:]
	}
	exponent := 0
	var err error
	if exponentText != "" {
		exponent, err = strconv.Atoi(exponentText)
		if err != nil {
			return false, "", 0, newError(ErrorContractInvalid, "invalid quota number exponent")
		}
	}
	return negative, mantissa, exponent, nil
}

func decimalDigits(mantissa string, exponent int) (string, int) {
	integer, fraction := mantissa, ""
	if index := strings.IndexByte(mantissa, '.'); index >= 0 {
		integer, fraction = mantissa[:index], mantissa[index+1:]
	}
	return integer + fraction, len(integer) + exponent
}

func trimDecimalDigits(digits string, decimalPosition int) (string, int, bool) {
	first := 0
	for first < len(digits) && digits[first] == '0' {
		first++
	}
	if first == len(digits) {
		return "", 0, true
	}
	digits = digits[first:]
	decimalPosition -= first
	for len(digits) > 1 && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
	}
	return digits, decimalPosition, false
}

func formatDecimalDigits(digits string, decimalPosition int) string {
	switch {
	case decimalPosition <= 0:
		return "0." + strings.Repeat("0", -decimalPosition) + digits
	case decimalPosition >= len(digits):
		return digits + strings.Repeat("0", decimalPosition-len(digits))
	default:
		return digits[:decimalPosition] + "." + digits[decimalPosition:]
	}
}

func deriveQuotaEvidence(requestID *string, billing, usage QuotaRawValue) QuotaEvidenceRevision {
	evidence := QuotaEvidenceRevision{
		OneAPIRequestID: cloneOptionalString(requestID), BillingQuotaRaw: billing, UsageQuotaRaw: usage,
		Status: QuotaEvidenceComplete, Issues: make([]QuotaEvidenceIssue, 0),
	}
	applyQuotaRequestID(&evidence, requestID)
	billingQuota, billingValid := normalizedQuotaValue(billing)
	usageQuota, usageValid := normalizedQuotaValue(usage)
	applyQuotaProjection(&evidence, billing, usage, billingQuota, usageQuota, billingValid, usageValid)
	return evidence
}

func applyQuotaRequestID(evidence *QuotaEvidenceRevision, requestID *string) {
	if requestID == nil || strings.TrimSpace(*requestID) == "" {
		evidence.OneAPIRequestID = nil
		evidence.Issues = append(evidence.Issues, QuotaIssueRequestIDMissing)
		evidence.Status = QuotaEvidenceMissing
	}
}

func applyQuotaProjection(
	evidence *QuotaEvidenceRevision,
	billing, usage QuotaRawValue,
	billingQuota, usageQuota *DecimalString,
	billingValid, usageValid bool,
) {
	if !billing.Present && !usage.Present {
		evidence.Issues = append(evidence.Issues, QuotaIssueMissing)
		evidence.Status = maxQuotaEvidenceStatus(evidence.Status, QuotaEvidenceMissing)
		return
	}
	if quotaRawProjectionInvalid(billing, usage, billingValid, usageValid) {
		applyInvalidQuotaProjection(evidence, billing, usage)
		return
	}
	if billing.Present && usage.Present {
		applyDualQuotaProjection(evidence, billingQuota, usageQuota)
		return
	}
	applySingleQuotaProjection(evidence, billing, usage, billingQuota, usageQuota)
}

func quotaRawProjectionInvalid(billing, usage QuotaRawValue, billingValid, usageValid bool) bool {
	return billing.Present && !billingValid || usage.Present && !usageValid
}

func applyInvalidQuotaProjection(evidence *QuotaEvidenceRevision, billing, usage QuotaRawValue) {
	evidence.Issues = append(evidence.Issues, QuotaIssueInvalid)
	evidence.Status = QuotaEvidenceInvalid
	if billing.Present != usage.Present {
		if billing.Present {
			evidence.Source = quotaSourcePointer(QuotaSourceBilling)
		} else {
			evidence.Source = quotaSourcePointer(QuotaSourceUsage)
		}
	}
}

func applyDualQuotaProjection(evidence *QuotaEvidenceRevision, billingQuota, usageQuota *DecimalString) {
	if *billingQuota != *usageQuota {
		evidence.Issues = append(evidence.Issues, QuotaIssueConflicting)
		evidence.Status = maxQuotaEvidenceStatus(evidence.Status, QuotaEvidenceConflicting)
		return
	}
	evidence.Quota = billingQuota
	evidence.Source = quotaSourcePointer(QuotaSourceBothConsistent)
}

func applySingleQuotaProjection(
	evidence *QuotaEvidenceRevision,
	billing, usage QuotaRawValue,
	billingQuota, usageQuota *DecimalString,
) {
	if billing.Present {
		evidence.Quota = billingQuota
		evidence.Source = quotaSourcePointer(QuotaSourceBilling)
		return
	}
	if usage.Present {
		evidence.Quota = usageQuota
		evidence.Source = quotaSourcePointer(QuotaSourceUsage)
	}
}

func normalizedQuotaValue(raw QuotaRawValue) (*DecimalString, bool) {
	if !raw.Present || raw.Type != QuotaJSONNumber || len(raw.Value) == 0 {
		return nil, false
	}
	text := string(raw.Value)
	if strings.HasPrefix(text, "-") {
		return nil, false
	}
	value, err := NewDecimalString(text)
	if err != nil {
		return nil, false
	}
	return &value, true
}

func maxQuotaEvidenceStatus(left, right QuotaEvidenceStatus) QuotaEvidenceStatus {
	rank := map[QuotaEvidenceStatus]int{
		QuotaEvidenceComplete: 0, QuotaEvidenceMissing: 1, QuotaEvidenceConflicting: 2, QuotaEvidenceInvalid: 3,
	}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func quotaSourcePointer(value QuotaSource) *QuotaSource { return &value }

func optionalDecimalEqual(left, right *DecimalString) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func optionalQuotaSourceEqual(left, right *QuotaSource) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func quotaEvidenceIssuesEqual(left, right []QuotaEvidenceIssue) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func normalizeEvidenceIssues(issues []QuotaEvidenceIssue) ([]QuotaEvidenceIssue, error) {
	seen := make(map[QuotaEvidenceIssue]bool, len(issues))
	for _, issue := range issues {
		seen[issue] = true
	}
	out := make([]QuotaEvidenceIssue, 0, len(seen))
	for _, issue := range quotaEvidenceIssueOrder {
		if seen[issue] {
			out = append(out, issue)
			delete(seen, issue)
		}
	}
	if len(seen) != 0 {
		return nil, newError(ErrorContractInvalid, fmt.Sprintf("unknown quota evidence issue %q", firstQuotaIssue(seen)))
	}
	return out, nil
}

func firstQuotaIssue(values map[QuotaEvidenceIssue]bool) QuotaEvidenceIssue {
	for value := range values {
		return value
	}
	return ""
}

var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

// DecimalString is a canonical non-negative base-10 value. B0 stores values
// exactly and deliberately does not perform arithmetic.
type DecimalString string

func NewDecimalString(value string) (DecimalString, error) {
	decimal := DecimalString(value)
	if err := decimal.Validate(); err != nil {
		return "", err
	}
	return decimal, nil
}

func (d DecimalString) Validate() error {
	if !decimalPattern.MatchString(string(d)) {
		return newError(ErrorContractInvalid, "invalid non-negative decimal string")
	}
	return nil
}

func (d *DecimalString) UnmarshalJSON(data []byte) error {
	if d == nil {
		return fmt.Errorf("modelbilling: nil DecimalString receiver")
	}
	if len(data) == 0 || data[0] != '"' {
		return newError(ErrorContractInvalid, "decimal value must be a JSON string")
	}
	var value string
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&value); err != nil {
		return newError(ErrorContractInvalid, "decode decimal string")
	}
	next, err := NewDecimalString(value)
	if err != nil {
		return err
	}
	*d = next
	return nil
}
