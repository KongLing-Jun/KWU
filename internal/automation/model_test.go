package automation

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"kwu/internal/provider"
)

func TestNormalizeTaskDefaultsApprovalMode(t *testing.T) {
	task, err := NormalizeTask(Task{
		ID:       " task-1 ",
		Name:     " Daily check ",
		Schedule: Schedule{Kind: " ONCE "},
	})
	if err != nil {
		t.Fatalf("NormalizeTask returned error: %v", err)
	}
	if task.ID != "task-1" || task.Name != "Daily check" {
		t.Fatalf("task names not trimmed: %+v", task)
	}
	if task.Schedule.Kind != ScheduleKindOnce {
		t.Fatalf("schedule kind = %q, want %q", task.Schedule.Kind, ScheduleKindOnce)
	}
	if task.Execution.ApprovalMode != ApprovalModeAuto {
		t.Fatalf("approval mode = %q, want %q", task.Execution.ApprovalMode, ApprovalModeAuto)
	}
	if task.SchemaVersion != CurrentTaskSchemaVersion || task.Revision != 1 {
		t.Fatalf("schema/revision = %d/%d, want %d/1", task.SchemaVersion, task.Revision, CurrentTaskSchemaVersion)
	}
	if task.MissedRunPolicy != MissedRunLatestOnce {
		t.Fatalf("missed-run policy = %q, want %q", task.MissedRunPolicy, MissedRunLatestOnce)
	}
}

func TestNormalizeTaskV2CompatibilityFields(t *testing.T) {
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	until := from.Add(24 * time.Hour)
	task, err := NormalizeTask(Task{
		ID:              " task-v2 ",
		Name:            " Task V2 ",
		Scope:           " global ",
		ProjectID:       " cloud-project-1 ",
		Schedule:        Schedule{Kind: ScheduleKindOnce},
		EffectiveRange:  EffectiveRange{From: from, Until: until},
		MissedRunPolicy: " LATEST_ONCE ",
		Capabilities: CapabilitySelection{
			ModelRef:     " model-route ",
			SkillIDs:     []string{" review ", "review", ""},
			ExpertID:     " expert-1 ",
			ConnectorIDs: []string{" connector-1 ", "connector-1", ""},
		},
		Delivery: DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{
			{Type: " wecom ", Ref: " bot_target_v1_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32)) + " "},
		}},
	})
	if err != nil {
		t.Fatalf("NormalizeTask returned error: %v", err)
	}
	if task.Scope != "project" || task.ProjectID != "cloud-project-1" || task.WorkspaceRoot != "" || task.Capabilities.ModelRef != "model-route" || task.Capabilities.ExpertID != "expert-1" {
		t.Fatalf("normalized v2 fields = %+v", task)
	}
	if !reflect.DeepEqual(task.Capabilities.SkillIDs, []string{"review"}) || !reflect.DeepEqual(task.Capabilities.ConnectorIDs, []string{"connector-1"}) {
		t.Fatalf("normalized capability ids = %+v", task.Capabilities)
	}
	if len(task.Delivery.Targets) != 1 || task.Delivery.Targets[0].Type != "wecom" || !validDesktopBotDeliveryTargetRef(task.Delivery.Targets[0].Ref) {
		t.Fatalf("normalized delivery = %+v", task.Delivery)
	}
}

func TestValidateTaskRejectsInvalidV2Fields(t *testing.T) {
	base := Task{ID: "task-v2", Name: "Task V2", Schedule: Schedule{Kind: ScheduleKindOnce}}

	reversed := base
	reversed.EffectiveRange = EffectiveRange{
		From:  time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := ValidateTask(reversed); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("reversed effective range error = %v, want %v", err, ErrInvalidTask)
	}

	rawWeComTarget := base
	rawWeComTarget.Delivery = DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{{Type: "wecom", Ref: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=secret"}}}
	if err := ValidateTask(rawWeComTarget); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("raw wecom target error = %v, want %v", err, ErrInvalidTask)
	}

	badPolicy := base
	badPolicy.MissedRunPolicy = "all"
	if err := ValidateTask(badPolicy); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("invalid missed-run policy error = %v, want %v", err, ErrInvalidTask)
	}

	badDelivery := base
	badDelivery.Delivery = DeliveryPolicy{Enabled: true}
	if err := ValidateTask(badDelivery); !errors.Is(err, ErrInvalidTask) {
		t.Fatalf("empty enabled delivery error = %v, want %v", err, ErrInvalidTask)
	}
}

func TestValidateTaskRequiresOpaqueProductionDeliveryRefs(t *testing.T) {
	base := Task{ID: "task-delivery-ref", Name: "Delivery ref", Schedule: Schedule{Kind: ScheduleKindOnce}}
	opaqueRef := "bot_target_v1_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))

	for _, targetType := range []string{"feishu", "lark", "weixin_ilink", "weixin", "ilink", "wecom", "custom"} {
		t.Run(targetType+" rejects raw ref", func(t *testing.T) {
			task := base
			task.Delivery = DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{{Type: targetType, Ref: "raw-remote-id"}}}
			if err := ValidateTask(task); !errors.Is(err, ErrInvalidTask) {
				t.Fatalf("ValidateTask error = %v, want %v", err, ErrInvalidTask)
			}
		})
		t.Run(targetType+" accepts opaque ref", func(t *testing.T) {
			task := base
			task.Delivery = DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{{Type: targetType, Ref: opaqueRef}}}
			if err := ValidateTask(task); err != nil {
				t.Fatalf("ValidateTask error = %v", err)
			}
		})
	}

	for _, targetType := range []string{"local_test", "retry_test"} {
		t.Run(targetType+" permits explicit test ref", func(t *testing.T) {
			task := base
			task.Delivery = DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{{Type: targetType, Ref: "test-inbox"}}}
			if err := ValidateTask(task); err != nil {
				t.Fatalf("ValidateTask error = %v", err)
			}
		})
	}
}

func TestTaskJSONPreservesUnknownFields(t *testing.T) {
	raw := []byte(`{
		"id":"legacy-task",
		"name":"Legacy task",
		"enabled":true,
		"schedule":{"kind":"once"},
		"futureOwner":{"mode":"keep"}
	}`)
	var task Task
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	normalized, err := NormalizeTask(task)
	if err != nil {
		t.Fatalf("NormalizeTask returned error: %v", err)
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatalf("Unmarshal fields returned error: %v", err)
	}
	if string(fields["futureOwner"]) != `{"mode":"keep"}` {
		t.Fatalf("futureOwner = %s, want preserved object", fields["futureOwner"])
	}
}

func TestNewTaskDefaultDisabled(t *testing.T) {
	task, err := NormalizeTask(Task{
		ID:       "task-1",
		Name:     "Daily check",
		Schedule: Schedule{Kind: ScheduleKindOnce},
	})
	if err != nil {
		t.Fatalf("NormalizeTask returned error: %v", err)
	}
	if task.Enabled {
		t.Fatal("new task default Enabled = true, want false")
	}
}

func TestYoloRequiresExplicitSelection(t *testing.T) {
	implicit, err := NormalizeTask(Task{
		ID:       "task-1",
		Name:     "Implicit approval",
		Schedule: Schedule{Kind: ScheduleKindOnce},
	})
	if err != nil {
		t.Fatalf("NormalizeTask implicit returned error: %v", err)
	}
	if implicit.Execution.ApprovalMode == ApprovalModeYolo {
		t.Fatal("implicit approval mode normalized to yolo")
	}

	explicit, err := NormalizeTask(Task{
		ID:       "task-2",
		Name:     "Explicit approval",
		Schedule: Schedule{Kind: ScheduleKindOnce},
		Execution: ExecutionPolicy{
			ApprovalMode: ApprovalModeYolo,
		},
	})
	if err != nil {
		t.Fatalf("NormalizeTask explicit returned error: %v", err)
	}
	if explicit.Execution.ApprovalMode != ApprovalModeYolo {
		t.Fatalf("explicit approval mode = %q, want yolo", explicit.Execution.ApprovalMode)
	}
}

func TestValidateTaskRejectsInvalidInputs(t *testing.T) {
	valid := Task{
		ID:       "task-1",
		Name:     "Daily check",
		Schedule: Schedule{Kind: ScheduleKindInterval},
		Execution: ExecutionPolicy{
			ApprovalMode: ApprovalModeAsk,
		},
	}

	tests := []struct {
		name string
		task Task
		want error
	}{
		{
			name: "empty id",
			task: func() Task {
				t := valid
				t.ID = " "
				return t
			}(),
			want: ErrInvalidTask,
		},
		{
			name: "empty name",
			task: func() Task {
				t := valid
				t.Name = ""
				return t
			}(),
			want: ErrInvalidTask,
		},
		{
			name: "bad schedule",
			task: func() Task {
				t := valid
				t.Schedule.Kind = "cron"
				return t
			}(),
			want: ErrInvalidSchedule,
		},
		{
			name: "bad approval mode",
			task: func() Task {
				t := valid
				t.Execution.ApprovalMode = "always"
				return t
			}(),
			want: ErrInvalidApprovalMode,
		},
		{
			name: "bad approval timeout",
			task: func() Task {
				t := valid
				t.Execution.ApprovalTimeout = "0"
				return t
			}(),
			want: ErrInvalidApprovalTimeout,
		},
		{
			name: "unsupported max concurrent runs",
			task: func() Task {
				t := valid
				t.Execution.MaxConcurrentRuns = 2
				return t
			}(),
			want: ErrInvalidTask,
		},
		{
			name: "unknown billing source",
			task: func() Task {
				t := valid
				t.Prompt.BillingSource = "another_source"
				return t
			}(),
			want: ErrInvalidTask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateTask(tt.task); !errors.Is(err, tt.want) {
				t.Fatalf("ValidateTask error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestTaskJSONRoundTrip(t *testing.T) {
	start := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	task := Task{
		ID:            "weekly-review",
		Name:          "Weekly review",
		Enabled:       true,
		Scope:         "project",
		WorkspaceRoot: "D:/project/KWU",
		Prompt: PromptConfig{
			Model:          "test-model",
			BillingSource:  "subscription_pool",
			Skills:         []string{"daily-git-worktree-integration-workflow"},
			Role:           "reviewer",
			Prompt:         "Summarize project risks.",
			ConditionModel: "condition-model",
		},
		Schedule: Schedule{
			Kind:     ScheduleKindCalendar,
			Timezone: "Asia/Shanghai",
			Calendar: CalendarSchedule{
				Frequency: "weekly",
				TimeOfDay: "09:00",
				Weekdays:  []string{"monday"},
			},
			Window: TimeWindow{
				Start: "09:00",
				End:   "18:00",
			},
		},
		Conditions: Conditions{
			ConditionPrompt: "Run only when there are open todos.",
			FailureCooldown: "30m",
		},
		Execution: ExecutionPolicy{
			ApprovalMode:      ApprovalModeAuto,
			ApprovalTimeout:   "5m",
			MaxConcurrentRuns: 1,
		},
		Archive: ArchivePolicy{
			Visible:     true,
			KeepSession: true,
			KeepEvents:  true,
		},
		CreatedAt: start,
		UpdatedAt: start,
	}

	b, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var got Task
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got.ID != task.ID || got.Prompt.BillingSource != task.Prompt.BillingSource ||
		got.Schedule.Kind != ScheduleKindCalendar || got.Execution.ApprovalMode != ApprovalModeAuto {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestNormalizeTaskAcceptsLegacyAndCanonicalBillingSources(t *testing.T) {
	legacy, err := NormalizeTask(Task{
		ID:       "legacy",
		Name:     "Legacy task",
		Schedule: Schedule{Kind: ScheduleKindOnce},
	})
	if err != nil {
		t.Fatalf("NormalizeTask legacy returned error: %v", err)
	}
	if legacy.Prompt.BillingSource != "" {
		t.Fatalf("legacy billing source = %q, want empty", legacy.Prompt.BillingSource)
	}

	for _, source := range []string{"subscription_pool", "newapi_byok"} {
		t.Run(source, func(t *testing.T) {
			task, err := NormalizeTask(Task{
				ID:       source,
				Name:     source,
				Prompt:   PromptConfig{BillingSource: " " + source + " "},
				Schedule: Schedule{Kind: ScheduleKindOnce},
			})
			if err != nil {
				t.Fatalf("NormalizeTask returned error: %v", err)
			}
			if task.Prompt.BillingSource != source {
				t.Fatalf("billing source = %q, want %q", task.Prompt.BillingSource, source)
			}
		})
	}
}

func TestNormalizeTaskTrimsModelWithoutGuessingRoute(t *testing.T) {
	task, err := NormalizeTask(Task{
		ID:       "task-route",
		Name:     "Route task",
		Schedule: Schedule{Kind: ScheduleKindOnce},
		Prompt: PromptConfig{
			Model:         " :kwu/claude-opus-4-6 ",
			BillingSource: "subscription_pool",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Prompt.Model != ":kwu/claude-opus-4-6" {
		t.Fatalf("model = %q, want trimmed but otherwise unchanged", task.Prompt.Model)
	}

	doubleColon, err := NormalizeTask(Task{
		ID:       "task-double-colon",
		Name:     "Double colon task",
		Schedule: Schedule{Kind: ScheduleKindOnce},
		Prompt: PromptConfig{
			Model:         "::custom/model",
			BillingSource: "newapi_byok",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if doubleColon.Prompt.Model != "::custom/model" {
		t.Fatalf("double-colon model = %q, want unchanged", doubleColon.Prompt.Model)
	}
}

func TestScheduleJSONRoundTrip(t *testing.T) {
	schedule := Schedule{
		Kind:     ScheduleKindCalendar,
		RunAt:    time.Date(2026, 6, 27, 9, 30, 0, 0, time.UTC),
		Every:    "24h",
		Timezone: "Asia/Shanghai",
		Calendar: CalendarSchedule{
			Frequency: "monthly",
			TimeOfDay: "09:30",
			MonthDays: []int{1, 15},
			Months:    []int{1, 6, 12},
		},
		Window: TimeWindow{Start: "09:00", End: "18:00"},
	}
	b, err := json.Marshal(schedule)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var got Schedule
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if !reflect.DeepEqual(got, schedule) {
		t.Fatalf("Schedule round trip = %+v, want %+v", got, schedule)
	}
}

func TestRunManifestJSONRoundTrip(t *testing.T) {
	manifest := RunManifest{
		RunID:           "run-1",
		TaskID:          "task-1",
		TaskName:        "Daily check",
		Status:          RunStatusSucceeded,
		Trigger:         TriggerManual,
		TriggerReason:   "manual test",
		ConditionReason: "manual",
		Owner:           "desktop:123",
		HostOwner:       "desktop:123",
		LeaseID:         "run-1",
		StartedAt:       time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC),
		FinishedAt:      time.Date(2026, 6, 27, 9, 5, 0, 0, time.UTC),
		DurationMs:      300000,
		Condition:       ConditionResult{ShouldRun: true, Reason: "manual", CheckedAt: time.Date(2026, 6, 27, 8, 59, 0, 0, time.UTC)},
		Model:           "test-model",
		BillingSource:   "subscription_pool",
		WorkspaceRoot:   "D:/project/KWU",
		SessionPath:     "runs/task-1/run-1/session.jsonl",
		EventsPath:      "runs/task-1/run-1/events.jsonl",
		ResultPath:      "runs/task-1/run-1/result.md",
		Summary:         "ok",
		Usage:           &provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var got RunManifest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if !reflect.DeepEqual(got, manifest) {
		t.Fatalf("RunManifest round trip = %+v, want %+v", got, manifest)
	}
}

func TestRunManifestTransitions(t *testing.T) {
	start := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	run := RunManifest{RunID: "run-1", Status: RunStatusQueued, StartedAt: start}
	if err := run.Transition(RunStatusCheckingCondition, "", start.Add(time.Second)); err != nil {
		t.Fatalf("Transition checking: %v", err)
	}
	if err := run.Transition(RunStatusRunning, "", start.Add(2*time.Second)); err != nil {
		t.Fatalf("Transition running: %v", err)
	}
	if err := run.Transition(RunStatusSucceeded, "", start.Add(3*time.Second)); err != nil {
		t.Fatalf("Transition succeeded: %v", err)
	}
	if run.FinishedAt.IsZero() || run.DurationMs != 3000 {
		t.Fatalf("finished=%s duration=%d, want finished and 3000ms", run.FinishedAt, run.DurationMs)
	}
	if err := run.Transition(RunStatusRunning, "", start.Add(4*time.Second)); !errors.Is(err, ErrInvalidRunTransition) {
		t.Fatalf("terminal transition error = %v, want %v", err, ErrInvalidRunTransition)
	}
}

func TestTaskModelDoesNotExposeSensitiveFields(t *testing.T) {
	var names []string
	collectFieldNames(reflect.TypeOf(Task{}), &names)
	for _, name := range names {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "key") || strings.Contains(lower, "api_key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			t.Fatalf("Task model should not expose sensitive field %q", name)
		}
	}
}

func collectFieldNames(typ reflect.Type, names *[]string) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || typ == reflect.TypeOf(time.Time{}) {
		return
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		*names = append(*names, field.Name)
		collectFieldNames(field.Type, names)
	}
}
