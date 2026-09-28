package automation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadLegacyHeartbeatMissingFile(t *testing.T) {
	t.Setenv("KWU_STATE_HOME", t.TempDir())

	got, err := ReadLegacyHeartbeatMigration()
	if err != nil {
		t.Fatalf("ReadLegacyHeartbeatMigration returned error: %v", err)
	}
	if len(got.Tasks) != 0 {
		t.Fatalf("tasks len = %d, want 0", len(got.Tasks))
	}
}

func TestReadLegacyHeartbeatMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, legacyHeartbeatFileName)
	writeLegacyHeartbeatFile(t, path, legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{
			ID:                     "daily",
			Title:                  "Daily check",
			Prompt:                 "Summarize the workspace.",
			Interval:               "24h|daily@09:00",
			Enabled:                true,
			Scope:                  "project",
			WorkspaceRoot:          "D:/project/KWU",
			TopicID:                "topic-1",
			CreatedAt:              1000,
			LastRunAt:              2000,
			NewConversationEachRun: true,
		},
	}})

	got, err := readLegacyHeartbeatMigration(path)
	if err != nil {
		t.Fatalf("readLegacyHeartbeatMigration returned error: %v", err)
	}
	if len(got.Tasks) != 1 || len(got.Metadata) != 1 {
		t.Fatalf("result = %+v, want one task and metadata", got)
	}
	task := got.Tasks[0]
	if task.ID != "heartbeat-daily" || task.Name != "Daily check" || task.Prompt.Prompt != "Summarize the workspace." {
		t.Fatalf("task fields = %+v", task)
	}
	if task.Execution.ApprovalMode != ApprovalModeYolo {
		t.Fatalf("approval mode = %q, want yolo", task.Execution.ApprovalMode)
	}
	if got.Metadata[0].ApprovalModeSource != legacyApprovalModeEmptyDefault || got.Metadata[0].TopicID != "topic-1" {
		t.Fatalf("metadata = %+v", got.Metadata[0])
	}
}

func TestConvertHeartbeatTask(t *testing.T) {
	legacy := legacyHeartbeatTask{
		ID:                     "abc",
		Title:                  "Project heartbeat",
		Prompt:                 "Check status.",
		Interval:               "30m",
		Enabled:                true,
		Scope:                  "project",
		WorkspaceRoot:          "D:/project/KWU",
		CreatedAt:              1000,
		LastRunAt:              2000,
		NewConversationEachRun: true,
		ApprovalMode:           "auto",
		TimeWindowStart:        "09:00",
		TimeWindowEnd:          "18:00",
	}

	got, source, err := convertHeartbeatTask(legacy)
	if err != nil {
		t.Fatalf("convertHeartbeatTask returned error: %v", err)
	}
	wantCreatedAt := time.UnixMilli(1000)
	wantLastRunAt := time.UnixMilli(2000)
	if got.ID != "heartbeat-abc" || got.Name != "Project heartbeat" || !got.Enabled {
		t.Fatalf("basic fields = %+v", got)
	}
	if got.Scope != "project" || got.WorkspaceRoot != "D:/project/KWU" || got.Prompt.Prompt != "Check status." {
		t.Fatalf("mapped fields = %+v", got)
	}
	if got.Schedule.Kind != ScheduleKindInterval || got.Schedule.Every != "30m" {
		t.Fatalf("schedule = %+v", got.Schedule)
	}
	if got.Schedule.Window != (TimeWindow{Start: "09:00", End: "18:00"}) {
		t.Fatalf("window = %+v", got.Schedule.Window)
	}
	if got.Execution.ApprovalMode != ApprovalModeAuto || source != legacyApprovalModeExplicit {
		t.Fatalf("approval = %q source %q", got.Execution.ApprovalMode, source)
	}
	if !got.Archive.KeepSession {
		t.Fatal("Archive.KeepSession = false, want true")
	}
	if !got.CreatedAt.Equal(wantCreatedAt) || !got.LastRunAt.Equal(wantLastRunAt) {
		t.Fatalf("times = created %s last %s", got.CreatedAt, got.LastRunAt)
	}
}

func TestHeartbeatCompatMapping(t *testing.T) {
	if got := HeartbeatCompatTaskID("daily"); got != "heartbeat-daily" {
		t.Fatalf("HeartbeatCompatTaskID = %q, want heartbeat-daily", got)
	}
	if got := HeartbeatCompatTaskID("heartbeat-daily"); got != "heartbeat-daily" {
		t.Fatalf("HeartbeatCompatTaskID prefixed = %q, want heartbeat-daily", got)
	}
	if !IsHeartbeatCompatTaskID("heartbeat-daily") {
		t.Fatal("IsHeartbeatCompatTaskID should accept heartbeat-prefixed ids")
	}
	if IsHeartbeatCompatTaskID("task-daily") {
		t.Fatal("IsHeartbeatCompatTaskID should reject native automation ids")
	}
}

func TestConvertHeartbeatSchedule(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		want     Schedule
	}{
		{
			name:     "duration",
			interval: "15",
			want: Schedule{
				Kind:     ScheduleKindInterval,
				Every:    "15m",
				Timezone: legacyHeartbeatScheduleTimezone,
			},
		},
		{
			name:     "daily",
			interval: "24h|daily@09:30",
			want: Schedule{
				Kind:     ScheduleKindCalendar,
				Timezone: legacyHeartbeatScheduleTimezone,
				Calendar: CalendarSchedule{Frequency: "daily", TimeOfDay: "09:30"},
			},
		},
		{
			name:     "weekly",
			interval: "168h|weekly:mon,wed@10:15",
			want: Schedule{
				Kind:     ScheduleKindCalendar,
				Timezone: legacyHeartbeatScheduleTimezone,
				Calendar: CalendarSchedule{Frequency: "weekly", TimeOfDay: "10:15", Weekdays: []string{"mon", "wed"}},
			},
		},
		{
			name:     "monthly",
			interval: "720h|monthly:31@08:00",
			want: Schedule{
				Kind:     ScheduleKindCalendar,
				Timezone: legacyHeartbeatScheduleTimezone,
				Calendar: CalendarSchedule{Frequency: "monthly", TimeOfDay: "08:00", MonthDays: []int{31}},
			},
		},
		{
			name:     "yearly",
			interval: "8760h|yearly:6-27@11:00",
			want: Schedule{
				Kind:     ScheduleKindCalendar,
				Timezone: legacyHeartbeatScheduleTimezone,
				Calendar: CalendarSchedule{Frequency: "yearly", TimeOfDay: "11:00", Months: []int{6}, MonthDays: []int{27}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertHeartbeatSchedule(tt.interval, "", "")
			if err != nil {
				t.Fatalf("ConvertHeartbeatSchedule returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("schedule = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestConvertHeartbeatScheduleIsDueCompatible(t *testing.T) {
	schedule, err := ConvertHeartbeatSchedule("24h|daily@09:00", "", "")
	if err != nil {
		t.Fatalf("ConvertHeartbeatSchedule returned error: %v", err)
	}
	task := Task{
		CreatedAt: time.Date(2026, 6, 27, 8, 0, 0, 0, time.Local),
		Schedule:  schedule,
	}
	got := Due(task, time.Date(2026, 6, 27, 9, 0, 0, 0, time.Local))
	if !got.Due {
		t.Fatalf("Due = %+v, want migrated daily schedule due", got)
	}
}

func TestLegacyApprovalMode(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		wantMode   ApprovalMode
		wantSource string
	}{
		{name: "empty", mode: "", wantMode: ApprovalModeYolo, wantSource: legacyApprovalModeEmptyDefault},
		{name: "explicit", mode: " ask ", wantMode: ApprovalModeAsk, wantSource: legacyApprovalModeExplicit},
		{name: "auto", mode: "auto", wantMode: ApprovalModeAuto, wantSource: legacyApprovalModeExplicit},
		{name: "yolo", mode: "yolo", wantMode: ApprovalModeYolo, wantSource: legacyApprovalModeExplicit},
		{name: "invalid", mode: "later", wantMode: ApprovalModeYolo, wantSource: legacyApprovalModeInvalidDefault},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotSource := normalizeLegacyHeartbeatApprovalMode(tt.mode)
			if gotMode != tt.wantMode || gotSource != tt.wantSource {
				t.Fatalf("approval = %q %q, want %q %q", gotMode, gotSource, tt.wantMode, tt.wantSource)
			}
		})
	}
}

func TestConvertHeartbeatScheduleRejectsInvalidInputs(t *testing.T) {
	if _, err := ConvertHeartbeatSchedule("soon", "", ""); !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("invalid interval error = %v, want ErrInvalidSchedule", err)
	}
	if _, err := ConvertHeartbeatSchedule("30m", "25:00", ""); !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("invalid window error = %v, want ErrInvalidSchedule", err)
	}
}

func TestHeartbeatMigrationDoesNotExposeProviderKeys(t *testing.T) {
	task, _, err := convertHeartbeatTask(legacyHeartbeatTask{
		ID:           "key-check",
		Title:        "Key check",
		Prompt:       "Do work",
		Interval:     "30m",
		ApprovalMode: "auto",
	})
	if err != nil {
		t.Fatalf("convertHeartbeatTask returned error: %v", err)
	}
	b, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if strings.Contains(strings.ToLower(string(b)), "api_key") || strings.Contains(strings.ToLower(string(b)), "provider") {
		t.Fatalf("task JSON contains provider secret-shaped data: %s", b)
	}
}

func writeLegacyHeartbeatFile(t *testing.T, path string, cfg legacyHeartbeatConfig) {
	t.Helper()
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal legacy config: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("WriteFile legacy config: %v", err)
	}
}
