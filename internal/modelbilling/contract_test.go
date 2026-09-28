package modelbilling

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/provider"
)

func containsJSONKey(data []byte, key string) bool {
	return bytes.Contains(data, []byte(`"`+key+`"`))
}

func testBindingScope() cloud.BindingScope {
	return cloud.BindingScope{
		ProfileID:         "default",
		CloudUserID:       "usr_fixture",
		BindingGeneration: 1,
	}
}

func testTaskScope(source BillingSource) TaskScope {
	scope := TaskScope{ActorID: "local", ProfileID: "default"}
	if source == BillingSubscriptionPool {
		binding := testBindingScope()
		scope.CloudBinding = &binding
	}
	return scope
}

func TestDecimalStringRejectsNonCanonicalOrNegativeValues(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    DecimalString
		wantErr bool
	}{
		{name: "zero", raw: `"0"`, want: "0"},
		{name: "fraction", raw: `"2000.0000"`, want: "2000.0000"},
		{name: "negative", raw: `"-1"`, wantErr: true},
		{name: "leading zero", raw: `"01.0"`, wantErr: true},
		{name: "exponent", raw: `"1e3"`, wantErr: true},
		{name: "number", raw: `1.5`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got DecimalString
			err := json.Unmarshal([]byte(tt.raw), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) accepted invalid decimal %q", tt.raw, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("Unmarshal(%s) = %q, %v; want %q", tt.raw, got, err, tt.want)
			}
		})
	}
}

func TestModelTaskValidatesBillingScopeAndFreezeOwnership(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	binding := testBindingScope()
	task := ModelTask{
		TaskID:         "task_01",
		Scope:          testTaskScope(BillingSubscriptionPool),
		BillingSource:  BillingSubscriptionPool,
		InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "1.0",
		FreezeRequest: &cloud.FreezeRequest{
			Scope: binding, TaskID: "task_01", ModelID: "model_01",
			ClientTaskType: "chat", ClientEstimatedCost: "1.0", IdempotencyKey: "freeze_key",
		},
		Freeze: &cloud.Freeze{
			Scope: binding, FreezeID: "frz_01", TaskID: "task_01", Status: cloud.FreezeActive,
			FreezeAmount: "1.0", FreeFrozenAmount: "1.0", SubscriptionFrozenAmount: "0",
			ExpiresAt: now.Add(time.Hour),
		},
		Entitlement:       &TaskEntitlementSnapshot{Plan: "starter", CapturedAt: now},
		PricingEvidenceID: "price_01",
		State:             TaskReady,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := task.Validate(); err != nil {
		t.Fatalf("valid subscription task: %v", err)
	}
	for name, change := range map[string]func(*cloud.FreezeRequest){
		"model":          func(v *cloud.FreezeRequest) { v.ModelID = "other_model" },
		"task type":      func(v *cloud.FreezeRequest) { v.ClientTaskType = "other_task" },
		"estimated cost": func(v *cloud.FreezeRequest) { v.ClientEstimatedCost = "2.0" },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := task
			request := *task.FreezeRequest
			change(&request)
			invalid.FreezeRequest = &request
			if ErrorCodeOf(invalid.Validate()) != ErrorContractInvalid {
				t.Fatalf("freeze request mismatch error = %v", invalid.Validate())
			}
		})
	}
	emptyVersion, spacedVersion := "", " "
	for _, version := range []*string{&emptyVersion, &spacedVersion} {
		invalid := task
		entitlement := *task.Entitlement
		entitlement.EntitlementVersion = version
		invalid.Entitlement = &entitlement
		if ErrorCodeOf(invalid.Validate()) != ErrorContractInvalid {
			t.Fatalf("entitlement version error = %v", invalid.Validate())
		}
	}
	expired := task
	expiredFreeze := *task.Freeze
	expiredFreeze.Status = cloud.FreezeExpiredReleased
	expired.Freeze = &expiredFreeze
	if err := expired.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("ready task with expired freeze error = %v, want %s", err, ErrorContractInvalid)
	}

	byok := task
	byok.TaskID = "task_byok"
	byok.Scope = testTaskScope(BillingNewAPIByok)
	byok.BillingSource = BillingNewAPIByok
	byok.FreezeRequest = nil
	byok.Freeze = nil
	byok.Entitlement = nil
	byok.State = TaskReady
	if err := byok.Validate(); err != nil {
		t.Fatalf("valid BYOK task: %v", err)
	}

	byok.Freeze = task.Freeze
	if err := byok.Validate(); ErrorCodeOf(err) != ErrorBillingSourceMismatch {
		t.Fatalf("BYOK freeze error = %v, want %s", err, ErrorBillingSourceMismatch)
	}
}

func TestModelCallMustInheritTaskBillingSourceAndScope(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	task := ModelTask{
		TaskID: "task_byok", Scope: testTaskScope(BillingNewAPIByok),
		BillingSource: BillingNewAPIByok, PricingEvidenceID: "price_01",
		State: TaskRunning, CreatedAt: now, UpdatedAt: now,
	}
	call := ModelCall{
		CallID: "call_01", TaskID: task.TaskID, Scope: task.Scope,
		BillingSource: BillingNewAPIByok, Role: "planner", ModelID: "model_01",
		Provider: "newapi", PricingEvidenceID: "price_01", State: CallPrepared,
	}
	if err := call.ValidateForTask(task); err != nil {
		t.Fatalf("valid call: %v", err)
	}
	subscriptionTask := task
	subscriptionTask.Scope = testTaskScope(BillingSubscriptionPool)
	subscriptionTask.BillingSource = BillingSubscriptionPool
	subscriptionCall := call
	subscriptionCall.Scope = testTaskScope(BillingSubscriptionPool)
	subscriptionCall.BillingSource = BillingSubscriptionPool
	if err := subscriptionCall.ValidateForTask(subscriptionTask); err != nil {
		t.Fatalf("call with value-equal scope: %v", err)
	}
	call.RawUsageRedacted = json.RawMessage(`{"content":"model output"}`)
	if err := call.ValidateForTask(task); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("unredacted usage error = %v, want %s", err, ErrorContractInvalid)
	}
	call.RawUsageRedacted = nil
	call.BillingSource = BillingSubscriptionPool
	if err := call.ValidateForTask(task); ErrorCodeOf(err) != ErrorBillingSourceMismatch {
		t.Fatalf("billing mismatch error = %v, want %s", err, ErrorBillingSourceMismatch)
	}
}

func TestGatewayChunkRejectsModelContentInRawUsage(t *testing.T) {
	chunk := GatewayChunk{RawUsageRedacted: json.RawMessage(`{"input_tokens":1}`)}
	if err := chunk.Validate(); err != nil {
		t.Fatalf("valid raw usage: %v", err)
	}
	chunk.RawUsageRedacted = json.RawMessage(`{"messages":[{"content":"secret"}]}`)
	if err := chunk.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("unredacted gateway usage error = %v, want %s", err, ErrorContractInvalid)
	}
}

func TestRawUsageRedactedCanonicalizesSensitiveKeySeparatorsRecursively(t *testing.T) {
	capitalize := func(word string) string {
		return strings.ToUpper(word[:1]) + word[1:]
	}
	camelCase := func(words []string) string {
		key := words[0]
		for _, word := range words[1:] {
			key += capitalize(word)
		}
		return key
	}
	keyForms := []struct {
		name string
		key  func([]string) string
	}{
		{"snake", func(words []string) string { return strings.Join(words, "_") }},
		{"kebab", func(words []string) string { return strings.Join(words, "-") }},
		{"camel", camelCase},
		{"flat", func(words []string) string { return strings.Join(words, "") }},
		{"case", func(words []string) string { return strings.ToUpper(strings.Join(words, "_")) }},
	}
	locations := []struct {
		name string
		wrap func(string) string
	}{
		{"top", func(key string) string { return `{"` + key + `":"value"}` }},
		{"nested", func(key string) string { return `{"nested":{"` + key + `":"value"}}` }},
		{"array", func(key string) string { return `[{"nested":{"` + key + `":"value"}}]` }},
	}
	for _, sensitive := range []struct {
		name  string
		words []string
	}{
		{"x api key", []string{"x", "api", "key"}},
		{"set cookie", []string{"set", "cookie"}},
		{"access token", []string{"access", "token"}},
		{"refresh token", []string{"refresh", "token"}},
		{"api token", []string{"api", "token"}},
		{"authorization", []string{"authorization"}},
		{"bearer", []string{"bearer"}},
		{"cookie", []string{"cookie"}},
		{"token", []string{"token"}},
		{"key", []string{"key"}},
		{"credential", []string{"credential"}},
		{"secret", []string{"secret"}},
		{"password", []string{"password"}},
	} {
		for _, form := range keyForms {
			for _, location := range locations {
				t.Run(sensitive.name+"/"+form.name+"/"+location.name, func(t *testing.T) {
					raw := location.wrap(" " + form.key(sensitive.words) + " ")
					if err := ValidateRawUsageRedacted(json.RawMessage(raw)); ErrorCodeOf(err) != ErrorContractInvalid {
						t.Fatalf("raw usage %s error = %v, want %s", raw, err, ErrorContractInvalid)
					}
				})
			}
		}
	}
	if err := ValidateRawUsageRedacted(json.RawMessage(`{"input_tokens":8,"output_tokens":2,"total_tokens":10}`)); err != nil {
		t.Fatalf("usage token fields rejected: %v", err)
	}
}

func TestGatewayRequestCarriesProviderRequestWithoutCredentials(t *testing.T) {
	req := GatewayRequest{
		TaskID: "task_01", CallID: "call_01", ModelID: "model_01", Role: "main",
		ProviderRequest: provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hello"}}},
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("GatewayRequest.Validate: %v", err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal GatewayRequest: %v", err)
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "api_key", "short_lived_token"} {
		if json.Valid(data) && containsJSONKey(data, forbidden) {
			t.Fatalf("GatewayRequest JSON contains forbidden key %q: %s", forbidden, data)
		}
	}
}

func TestPendingReportRequiresImmutableCanonicalSubscriptionPayload(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	key, err := BuildFinalIdempotencyKey(testBindingScope(), "task_01", "frz_01")
	if err != nil {
		t.Fatalf("BuildFinalIdempotencyKey: %v", err)
	}
	report := PendingReport{
		Scope: testBindingScope(), TaskID: "task_01", FreezeID: "frz_01",
		ContractVersion: LegacyTokenContractV1,
		Kind:            ReportFinal, Sequence: 0, IdempotencyKey: key,
		CanonicalPayload: json.RawMessage(`{"task_id":"task_01"}`), State: ReportPending,
		NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("valid final report: %v", err)
	}
	report.Sequence = 1
	if err := report.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("final sequence error = %v, want %s", err, ErrorContractInvalid)
	}
	if err := ValidateCanonicalPayload(report.CanonicalPayload, json.RawMessage(`{"task_id":"other"}`)); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("payload conflict error = %v, want %s", err, ErrorIdempotencyConflict)
	}
}

func validPartialReport(t *testing.T, sequence uint64) PartialUsageReport {
	t.Helper()
	scope := testBindingScope()
	key, err := BuildPartialIdempotencyKey(scope, "task_01", "frz_01", sequence)
	if err != nil {
		t.Fatalf("BuildPartialIdempotencyKey: %v", err)
	}
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	count := int64(1)
	return PartialUsageReport{
		TaskID: "task_01", Scope: scope, FreezeID: "frz_01", Sequence: sequence,
		ReportMode: ReportModeCumulative,
		Calls: []ModelCallUsage{{
			CallID: "call_01", ModelID: "model_01", PricingGroup: "default",
			PricingEvidenceID: "price_01",
			OccurredAt:        now,
			Usage:             NormalizedUsage{RequestCount: &count, BreakdownStatus: UsageExact},
		}},
		IdempotencyKey: key, CreatedAt: now,
	}
}

func validFinalReport(t *testing.T, result TaskResult) FinalUsageReport {
	t.Helper()
	scope := testBindingScope()
	key, err := BuildFinalIdempotencyKey(scope, "task_01", "frz_01")
	if err != nil {
		t.Fatalf("BuildFinalIdempotencyKey: %v", err)
	}
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	count := int64(1)
	return FinalUsageReport{
		TaskID: "task_01", Scope: scope, FreezeID: "frz_01", Calls: []ModelCallUsage{{
			CallID: "call_01", ModelID: "model_01", PricingGroup: "default", PricingEvidenceID: "price_01",
			OccurredAt: now, Usage: NormalizedUsage{RequestCount: &count, BreakdownStatus: UsageExact},
		}},
		ClientSummary: TaskUsageSummary{ModelCallCount: 1, StartedAt: now, FinishedAt: now},
		TaskResult:    result, IdempotencyKey: key, CreatedAt: now,
	}
}

func TestUsageReportsRequireCalls(t *testing.T) {
	partial := validPartialReport(t, 1)
	partial.Calls = nil
	if err := partial.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("empty partial error = %v, want %s", err, ErrorContractInvalid)
	}
	for _, result := range []TaskResult{TaskResultCompleted, TaskResultFailed, TaskResultCancelled} {
		final := validFinalReport(t, result)
		final.Calls = nil
		final.ClientSummary.ModelCallCount = 0
		if err := final.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
			t.Fatalf("zero-call final result %s error = %v, want %s", result, err, ErrorContractInvalid)
		}
	}
	final := validFinalReport(t, TaskResult("running"))
	if err := final.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("invalid task result error = %v, want %s", err, ErrorContractInvalid)
	}
}

func TestUsageReportsVerifyOpaqueIdempotencyKeys(t *testing.T) {
	if _, err := BuildFinalIdempotencyKey(testBindingScope(), " task_01", "frz_01"); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("builder accepted untrimmed task_id: %v", err)
	}
	if _, err := BuildFinalIdempotencyKey(testBindingScope(), "task_01", "frz_01 "); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("builder accepted untrimmed freeze_id: %v", err)
	}
	partial := validPartialReport(t, 1)
	partial.IdempotencyKey = "readable_partial_key"
	if err := partial.Validate(); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("partial wrong key error = %v, want %s", err, ErrorIdempotencyConflict)
	}
	partial = validPartialReport(t, 1)
	partial.TaskID = " task_01"
	if err := partial.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("partial untrimmed task error = %v, want %s", err, ErrorContractInvalid)
	}
	partial = validPartialReport(t, 1)
	partial.FreezeID = "frz_01 "
	if err := partial.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("partial untrimmed freeze error = %v, want %s", err, ErrorContractInvalid)
	}
	partial = validPartialReport(t, 1)
	partial.Sequence = 2
	if err := partial.Validate(); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("partial changed sequence error = %v, want %s", err, ErrorIdempotencyConflict)
	}
	partial = validPartialReport(t, 1)
	partial.Scope.BindingGeneration++
	if err := partial.Validate(); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("partial changed scope error = %v, want %s", err, ErrorIdempotencyConflict)
	}

	final := validFinalReport(t, TaskResultCancelled)
	final.IdempotencyKey = "readable_final_key"
	if err := final.Validate(); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("final wrong key error = %v, want %s", err, ErrorIdempotencyConflict)
	}

	scope := testBindingScope()
	sequenceOne, _ := BuildPartialIdempotencyKey(scope, "task_01", "frz_01", 1)
	sequenceTwo, _ := BuildPartialIdempotencyKey(scope, "task_01", "frz_01", 2)
	otherScope := scope
	otherScope.BindingGeneration++
	otherScopeKey, _ := BuildPartialIdempotencyKey(otherScope, "task_01", "frz_01", 1)
	if sequenceOne == sequenceTwo || sequenceOne == otherScopeKey {
		t.Fatalf("idempotency key did not vary by sequence/scope: %q %q %q", sequenceOne, sequenceTwo, otherScopeKey)
	}
}

func TestPendingReportNextAttemptAtFollowsState(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	key, err := BuildFinalIdempotencyKey(testBindingScope(), "task_01", "frz_01")
	if err != nil {
		t.Fatalf("BuildFinalIdempotencyKey: %v", err)
	}
	base := PendingReport{
		Scope: testBindingScope(), TaskID: "task_01", FreezeID: "frz_01", Kind: ReportFinal,
		ContractVersion: LegacyTokenContractV1,
		Sequence:        0, IdempotencyKey: key, CanonicalPayload: json.RawMessage(`{"task_id":"task_01"}`),
		CreatedAt: now, UpdatedAt: now,
	}
	for _, state := range []ReportState{ReportPending, ReportRetry, ReportDelivering} {
		report := base
		report.State = state
		if err := report.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
			t.Fatalf("state %s without next_attempt_at error = %v", state, err)
		}
		report.NextAttemptAt = &now
		if err := report.Validate(); err != nil {
			t.Fatalf("state %s with next_attempt_at: %v", state, err)
		}
	}
	for _, state := range []ReportState{ReportAccepted, ReportBlocked} {
		report := base
		report.State = state
		if err := report.Validate(); err != nil {
			t.Fatalf("state %s without next_attempt_at: %v", state, err)
		}
		report.NextAttemptAt = &now
		if err := report.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
			t.Fatalf("state %s with fake next_attempt_at error = %v", state, err)
		}
	}
}

func TestPendingReportVerifiesIdempotencyIdentity(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	report := PendingReport{
		Scope: testBindingScope(), TaskID: "task_01", FreezeID: "frz_01", Kind: ReportPartial,
		ContractVersion: LegacyTokenContractV1,
		Sequence:        1, CanonicalPayload: json.RawMessage(`{"sequence":1}`), State: ReportPending,
		NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	key, err := BuildPartialIdempotencyKey(report.Scope, report.TaskID, report.FreezeID, report.Sequence)
	if err != nil {
		t.Fatalf("BuildPartialIdempotencyKey: %v", err)
	}
	report.IdempotencyKey = key
	if err := report.Validate(); err != nil {
		t.Fatalf("valid pending report: %v", err)
	}
	for name, mutate := range map[string]func(*PendingReport){
		"key":      func(r *PendingReport) { r.IdempotencyKey = "kwu_00000000000000000000000000000000" },
		"sequence": func(r *PendingReport) { r.Sequence++ },
		"scope":    func(r *PendingReport) { r.Scope.BindingGeneration++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := report
			mutate(&changed)
			if err := changed.Validate(); ErrorCodeOf(err) != ErrorIdempotencyConflict {
				t.Fatalf("changed pending report error = %v, want %s", err, ErrorIdempotencyConflict)
			}
		})
	}
}

func TestTaskScopeKeysAreStableScopedAndOpaque(t *testing.T) {
	subscription := testTaskScope(BillingSubscriptionPool)
	first, err := BuildTaskScopeKey(subscription, BillingSubscriptionPool)
	if err != nil {
		t.Fatalf("BuildTaskScopeKey: %v", err)
	}
	repeat, _ := BuildTaskScopeKey(subscription, BillingSubscriptionPool)
	if first != repeat {
		t.Fatalf("scope key unstable: %q != %q", first, repeat)
	}
	variants := []TaskScope{subscription, subscription, subscription, subscription}
	variants[0].ProfileID = "work"
	variants[0].CloudBinding = &cloud.BindingScope{ProfileID: "work", CloudUserID: "usr_fixture", BindingGeneration: 1}
	variants[1].CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "other_user", BindingGeneration: 1}
	variants[2].CloudBinding = &cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_fixture", BindingGeneration: 2}
	variants[3].ActorID = "other_actor"
	for _, variant := range variants {
		key, err := BuildTaskScopeKey(variant, BillingSubscriptionPool)
		if err != nil || key == first {
			t.Fatalf("variant scope key = %q, %v; want distinct from %q", key, err, first)
		}
	}
	byok := testTaskScope(BillingNewAPIByok)
	byokKey, err := BuildTaskScopeKey(byok, BillingNewAPIByok)
	if err != nil || byokKey == first {
		t.Fatalf("BYOK scope key = %q, %v", byokKey, err)
	}
	for _, readable := range []string{subscription.ActorID, subscription.ProfileID, subscription.CloudBinding.CloudUserID} {
		if strings.Contains(first, readable) || strings.Contains(byokKey, readable) {
			t.Fatalf("scope key exposes %q: %q / %q", readable, first, byokKey)
		}
	}
}

func TestReportAndCompletedCallRequireUsageBreakdownStatus(t *testing.T) {
	partial := validPartialReport(t, 1)
	partial.Calls[0].Usage.BreakdownStatus = ""
	if err := partial.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("partial empty breakdown error = %v", err)
	}
	usage := NormalizedUsage{}
	if err := usage.ValidateOptional(); err != nil {
		t.Fatalf("optional usage rejected empty status: %v", err)
	}
	if err := usage.ValidateForReport(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("strict usage empty breakdown error = %v", err)
	}
	gateway := GatewayChunk{
		Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{}},
		Usage: &NormalizedUsage{},
	}
	if err := gateway.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("gateway usage empty breakdown error = %v", err)
	}
	fixtureChunk := GatewayFixtureChunk{Type: "usage", Usage: &NormalizedUsage{}}
	if err := fixtureChunk.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("fixture gateway usage empty breakdown error = %v", err)
	}
	task := ModelTask{
		TaskID: "task_byok", Scope: testTaskScope(BillingNewAPIByok), BillingSource: BillingNewAPIByok,
		PricingEvidenceID: "price_01", State: TaskRunning,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	call := ModelCall{
		CallID: "call_01", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		Role: CallRoleMain, ModelID: "model_01", Provider: "fixture", PricingEvidenceID: "price_01",
		State: CallCompleted,
	}
	if err := call.ValidateForTask(task); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("completed call empty breakdown error = %v", err)
	}
}

func TestNormalizedUsageRejectsConflictingAggregateEvidence(t *testing.T) {
	base := NormalizedUsage{
		InputTokens: int64Pointer(10), NormalInputTokens: int64Pointer(8),
		CachedInputTokens: int64Pointer(2), CacheReadTokens: int64Pointer(2),
		CacheWriteTokens: int64Pointer(0), OutputTokens: int64Pointer(4),
		CachedOutputTokens: int64Pointer(1), BreakdownStatus: UsageExact,
	}
	for name, mutate := range map[string]func(*NormalizedUsage){
		"input total":   func(usage *NormalizedUsage) { usage.InputTokens = int64Pointer(11) },
		"cached input":  func(usage *NormalizedUsage) { usage.CachedInputTokens = int64Pointer(3) },
		"cached output": func(usage *NormalizedUsage) { usage.CachedOutputTokens = int64Pointer(5) },
	} {
		t.Run(name, func(t *testing.T) {
			usage := base
			mutate(&usage)
			if err := usage.ValidateForReport(); ErrorCodeOf(err) != ErrorContractInvalid {
				t.Fatalf("ValidateForReport error = %v", err)
			}
		})
	}
}

func int64Pointer(value int64) *int64 { return &value }

func TestModelPriceValidatesQuotaAndBillingModeMatrix(t *testing.T) {
	decimal := DecimalString("1")
	expression := "usage.input_tokens * 2"
	base := ModelPrice{
		ModelID: "model_01", ProviderModelID: "provider_01", ModelName: "Model", ModelType: "chat",
	}
	tests := []struct {
		name    string
		price   ModelPrice
		wantErr bool
	}{
		{name: "token ratio", price: withPriceMode(base, QuotaTypeToken, BillingModeTokenRatio, &decimal, nil, nil)},
		{name: "per request", price: withPriceMode(base, QuotaTypePerRequest, BillingModePerRequest, nil, &decimal, nil)},
		{name: "tiered expression", price: withPriceMode(base, QuotaTypeToken, BillingModeTieredExpr, nil, nil, &expression)},
		{name: "token unavailable", price: withPriceMode(base, QuotaTypeToken, BillingModeUnavailable, nil, nil, nil)},
		{name: "request unavailable", price: withPriceMode(base, QuotaTypePerRequest, BillingModeUnavailable, nil, nil, nil)},
		{name: "token ratio missing basis", price: withPriceMode(base, QuotaTypeToken, BillingModeTokenRatio, nil, nil, nil), wantErr: true},
		{name: "request missing price", price: withPriceMode(base, QuotaTypePerRequest, BillingModePerRequest, nil, nil, nil), wantErr: true},
		{name: "tiered blank expression", price: withPriceMode(base, QuotaTypeToken, BillingModeTieredExpr, nil, nil, new(string)), wantErr: true},
		{name: "token per request mismatch", price: withPriceMode(base, QuotaTypeToken, BillingModePerRequest, nil, &decimal, nil), wantErr: true},
		{name: "request token ratio mismatch", price: withPriceMode(base, QuotaTypePerRequest, BillingModeTokenRatio, &decimal, nil, nil), wantErr: true},
		{name: "request tiered mismatch", price: withPriceMode(base, QuotaTypePerRequest, BillingModeTieredExpr, nil, nil, &expression), wantErr: true},
		{name: "unavailable with price", price: withPriceMode(base, QuotaTypeToken, BillingModeUnavailable, &decimal, nil, nil), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.price.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPricingEvidenceValidatesTaskAndCallModelIdentity(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	task := ModelTask{
		TaskID: "task_01", Scope: testTaskScope(BillingNewAPIByok), BillingSource: BillingNewAPIByok,
		PricingEvidenceID: "price_main", State: TaskRunning, CreatedAt: now, UpdatedAt: now,
	}
	evidence := PricingEvidence{
		EvidenceID: "price_main", SnapshotID: "snapshot_01", Scope: task.Scope, TaskID: task.TaskID,
		ModelID: "model_main", PricingGroup: "default", State: PricingFresh, CapturedAt: now,
	}
	if err := evidence.Validate(task.BillingSource); err != nil {
		t.Fatalf("PricingEvidence.Validate: %v", err)
	}
	if err := evidence.ValidateForTask(task); err != nil {
		t.Fatalf("PricingEvidence.ValidateForTask: %v", err)
	}
	call := ModelCall{
		CallID: "call_01", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		Role: CallRoleMain, ModelID: evidence.ModelID, Provider: "fixture",
		PricingEvidenceID: evidence.EvidenceID, State: CallPrepared,
	}
	if err := evidence.ValidateForCall(task, call); err != nil {
		t.Fatalf("PricingEvidence.ValidateForCall: %v", err)
	}
	otherModel := call
	otherModel.ModelID = "model_other"
	if err := evidence.ValidateForCall(task, otherModel); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("cross-model evidence error = %v", err)
	}
	otherTask := task
	otherTask.TaskID = "task_other"
	otherTask.PricingEvidenceID = evidence.EvidenceID
	if err := evidence.ValidateForTask(otherTask); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("cross-task evidence error = %v", err)
	}
	wrongPrimary := task
	wrongPrimary.PricingEvidenceID = "price_other"
	if err := evidence.ValidateForTask(wrongPrimary); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("wrong primary evidence error = %v", err)
	}
}

func withPriceMode(base ModelPrice, quota QuotaType, mode BillingMode, ratio, request *DecimalString, expression *string) ModelPrice {
	base.QuotaType = quota
	base.BillingMode = mode
	base.ModelRatio = ratio
	base.ModelPrice = request
	base.BillingExpression = expression
	return base
}

func TestFrozenEnumsRejectUnknownValues(t *testing.T) {
	for name, validate := range map[string]func() error{
		"call role":    func() error { return CallRole("other").Validate() },
		"task result":  func() error { return TaskResult("running").Validate() },
		"report mode":  func() error { return ReportMode("delta").Validate() },
		"quota type":   func() error { return QuotaType("0").Validate() },
		"billing mode": func() error { return BillingMode("ratio").Validate() },
	} {
		if err := validate(); ErrorCodeOf(err) != ErrorContractInvalid {
			t.Fatalf("%s unknown value error = %v", name, err)
		}
	}
}

func TestCallRoleGuardianIsValid(t *testing.T) {
	if err := CallRoleGuardian.Validate(); err != nil {
		t.Fatalf("guardian role validation: %v", err)
	}
}
