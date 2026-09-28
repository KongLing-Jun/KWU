package automation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kwu/internal/config"
)

const (
	legacyHeartbeatFileName          = "heartbeat-tasks.json"
	legacyHeartbeatTaskIDPrefix      = "heartbeat-"
	legacyApprovalModeExplicit       = "legacy_explicit"
	legacyApprovalModeEmptyDefault   = "legacy_empty_default_yolo"
	legacyApprovalModeInvalidDefault = "legacy_invalid_default_yolo"
	legacyHeartbeatScheduleTimezone  = "Local"
)

// HeartbeatMigrationResult is the in-memory conversion of the legacy Desktop
// heartbeat file. It is intentionally read-only; marker writes happen later.
type HeartbeatMigrationResult struct {
	Tasks    []Task                          `json:"tasks"`
	Metadata []HeartbeatMigrationMetadata    `json:"metadata,omitempty"`
	Skipped  []HeartbeatMigrationSkippedTask `json:"skipped,omitempty"`
}

// HeartbeatMigrationMetadata keeps legacy-only fields out of Task.
type HeartbeatMigrationMetadata struct {
	LegacyID           string `json:"legacyId,omitempty"`
	TaskID             string `json:"taskId"`
	TopicID            string `json:"topicId,omitempty"`
	ApprovalModeSource string `json:"approvalModeSource,omitempty"`
}

type HeartbeatMigrationSkippedTask struct {
	LegacyID string `json:"legacyId,omitempty"`
	Reason   string `json:"reason"`
}

type legacyHeartbeatConfig struct {
	Tasks []legacyHeartbeatTask `json:"tasks"`
}

type legacyHeartbeatTask struct {
	ID                     string `json:"id"`
	Title                  string `json:"title"`
	Prompt                 string `json:"prompt"`
	Interval               string `json:"interval"`
	Enabled                bool   `json:"enabled"`
	Scope                  string `json:"scope,omitempty"`
	WorkspaceRoot          string `json:"workspaceRoot,omitempty"`
	TopicID                string `json:"topicId,omitempty"`
	LastRunAt              int64  `json:"lastRunAt,omitempty"`
	NewConversationEachRun bool   `json:"newConversationEachRun,omitempty"`
	CreatedAt              int64  `json:"createdAt,omitempty"`
	ApprovalMode           string `json:"approvalMode"`
	TimeWindowStart        string `json:"timeWindowStart,omitempty"`
	TimeWindowEnd          string `json:"timeWindowEnd,omitempty"`
}

// ReadLegacyHeartbeatMigration reads the old Desktop heartbeat config path.
func ReadLegacyHeartbeatMigration() (HeartbeatMigrationResult, error) {
	return readLegacyHeartbeatMigration(filepath.Join(config.MemoryUserDir(), legacyHeartbeatFileName))
}

func readLegacyHeartbeatMigration(path string) (HeartbeatMigrationResult, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return HeartbeatMigrationResult{Tasks: []Task{}}, nil
	}
	if err != nil {
		return HeartbeatMigrationResult{}, fmt.Errorf("read legacy heartbeat tasks: %w", err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return HeartbeatMigrationResult{Tasks: []Task{}}, nil
	}
	return decodeLegacyHeartbeatMigration(path, b)
}

func decodeLegacyHeartbeatMigration(path string, b []byte) (HeartbeatMigrationResult, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return HeartbeatMigrationResult{Tasks: []Task{}}, nil
	}
	var file legacyHeartbeatConfig
	if err := json.Unmarshal(b, &file); err != nil {
		return HeartbeatMigrationResult{}, fmt.Errorf("read legacy heartbeat tasks: decode %s: %w", path, err)
	}

	result := HeartbeatMigrationResult{
		Tasks:    make([]Task, 0, len(file.Tasks)),
		Metadata: make([]HeartbeatMigrationMetadata, 0, len(file.Tasks)),
	}
	for i, legacy := range file.Tasks {
		task, source, err := convertHeartbeatTask(legacy)
		if err != nil {
			result.Skipped = append(result.Skipped, HeartbeatMigrationSkippedTask{
				LegacyID: strings.TrimSpace(legacy.ID),
				Reason:   fmt.Sprintf("legacy heartbeat task %d: %v", i, err),
			})
			continue
		}
		result.Tasks = append(result.Tasks, task)
		result.Metadata = append(result.Metadata, HeartbeatMigrationMetadata{
			LegacyID:           strings.TrimSpace(legacy.ID),
			TaskID:             task.ID,
			TopicID:            strings.TrimSpace(legacy.TopicID),
			ApprovalModeSource: source,
		})
	}
	return result, nil
}

func convertHeartbeatTask(h legacyHeartbeatTask) (Task, string, error) {
	schedule, err := ConvertHeartbeatSchedule(h.Interval, h.TimeWindowStart, h.TimeWindowEnd)
	if err != nil {
		return Task{}, "", err
	}
	mode, source := normalizeLegacyHeartbeatApprovalMode(h.ApprovalMode)
	task := Task{
		ID:            heartbeatTaskID(h.ID),
		Name:          strings.TrimSpace(h.Title),
		Enabled:       h.Enabled,
		Scope:         strings.TrimSpace(h.Scope),
		WorkspaceRoot: strings.TrimSpace(h.WorkspaceRoot),
		Prompt:        PromptConfig{Prompt: h.Prompt},
		Schedule:      schedule,
		Execution:     ExecutionPolicy{ApprovalMode: mode},
		Archive:       ArchivePolicy{KeepSession: h.NewConversationEachRun},
		CreatedAt:     unixMillisTime(h.CreatedAt),
		UpdatedAt:     unixMillisTime(h.CreatedAt),
		LastRunAt:     unixMillisTime(h.LastRunAt),
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.LastRunAt
	}
	if _, err := NormalizeTask(task); err != nil {
		return Task{}, "", err
	}
	return task, source, nil
}

// ConvertHeartbeatSchedule maps the legacy heartbeat interval string into an automation schedule.
func ConvertHeartbeatSchedule(interval, windowStart, windowEnd string) (Schedule, error) {
	window, err := legacyHeartbeatTimeWindow(windowStart, windowEnd)
	if err != nil {
		return Schedule{}, err
	}
	if schedule, ok := legacyHeartbeatCalendarSchedule(interval, window); ok {
		return schedule, nil
	}
	every, err := legacyHeartbeatInterval(interval)
	if err != nil {
		return Schedule{}, err
	}
	return Schedule{
		Kind:     ScheduleKindInterval,
		Every:    every,
		Timezone: legacyHeartbeatScheduleTimezone,
		Window:   window,
	}, nil
}

func legacyHeartbeatCalendarSchedule(interval string, window TimeWindow) (Schedule, bool) {
	idx := strings.Index(interval, "|")
	if idx < 0 {
		return Schedule{}, false
	}
	raw := strings.TrimSpace(interval[idx+1:])
	if raw == "" {
		return Schedule{}, false
	}
	at := "09:00"
	if parts := strings.SplitN(raw, "@", 2); len(parts) == 2 {
		raw = strings.TrimSpace(parts[0])
		at = strings.TrimSpace(parts[1])
	}
	if _, _, ok := parseScheduleClock(at); !ok {
		return Schedule{}, false
	}
	kind := raw
	rule := ""
	if parts := strings.SplitN(raw, ":", 2); len(parts) == 2 {
		kind = strings.TrimSpace(parts[0])
		rule = strings.TrimSpace(parts[1])
	}

	cal := CalendarSchedule{Frequency: kind, TimeOfDay: at}
	switch kind {
	case "daily":
	case "weekly":
		weekdays := legacyHeartbeatWeekdays(rule)
		if len(weekdays) == 0 {
			return Schedule{}, false
		}
		cal.Weekdays = weekdays
	case "monthly":
		cal.MonthDays = []int{legacyPositiveInt(rule, 1)}
	case "yearly":
		month, day := legacyHeartbeatYearly(rule)
		cal.Months = []int{month}
		cal.MonthDays = []int{day}
	default:
		return Schedule{}, false
	}

	return Schedule{
		Kind:     ScheduleKindCalendar,
		Calendar: cal,
		Timezone: legacyHeartbeatScheduleTimezone,
		Window:   window,
	}, true
}

func legacyHeartbeatInterval(interval string) (string, error) {
	base := strings.TrimSpace(interval)
	if idx := strings.Index(base, "|"); idx >= 0 {
		base = strings.TrimSpace(base[:idx])
	}
	if base == "" {
		return "", fmt.Errorf("%w: missing legacy heartbeat interval", ErrInvalidSchedule)
	}
	candidate := base
	switch base[len(base)-1] {
	case 's', 'm', 'h':
	default:
		candidate = base + "m"
	}
	d, err := time.ParseDuration(candidate)
	if err != nil || d <= 0 {
		return "", fmt.Errorf("%w: invalid legacy heartbeat interval %q", ErrInvalidSchedule, interval)
	}
	return candidate, nil
}

func legacyHeartbeatTimeWindow(start, end string) (TimeWindow, error) {
	window := TimeWindow{Start: strings.TrimSpace(start), End: strings.TrimSpace(end)}
	if _, _, err := parseOptionalClock(window.Start); err != nil {
		return TimeWindow{}, fmt.Errorf("%w: invalid legacy heartbeat time window start %q", ErrInvalidSchedule, start)
	}
	if _, _, err := parseOptionalClock(window.End); err != nil {
		return TimeWindow{}, fmt.Errorf("%w: invalid legacy heartbeat time window end %q", ErrInvalidSchedule, end)
	}
	return window, nil
}

func legacyHeartbeatWeekdays(rule string) []string {
	var out []string
	for _, part := range strings.Split(rule, ",") {
		if weekday, ok := legacyHeartbeatWeekday(part); ok {
			out = append(out, weekday)
		}
	}
	return out
}

func legacyHeartbeatWeekday(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sun", "sunday":
		return "sun", true
	case "mon", "monday":
		return "mon", true
	case "tue", "tuesday":
		return "tue", true
	case "wed", "wednesday":
		return "wed", true
	case "thu", "thursday":
		return "thu", true
	case "fri", "friday":
		return "fri", true
	case "sat", "saturday":
		return "sat", true
	default:
		return "", false
	}
}

func legacyHeartbeatYearly(rule string) (int, int) {
	parts := strings.SplitN(rule, "-", 2)
	month := legacyPositiveInt(firstLegacyPart(parts), 1)
	day := 1
	if len(parts) == 2 {
		day = legacyPositiveInt(parts[1], 1)
	}
	if month < 1 {
		month = 1
	}
	if month > 12 {
		month = 12
	}
	return month, day
}

func legacyPositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func firstLegacyPart(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func heartbeatTaskID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, legacyHeartbeatTaskIDPrefix) {
		return id
	}
	return legacyHeartbeatTaskIDPrefix + id
}

// HeartbeatCompatTaskID maps a legacy HeartbeatTask id to the automation task id.
func HeartbeatCompatTaskID(id string) string {
	return heartbeatTaskID(id)
}

// IsHeartbeatCompatTaskID reports whether an automation task came through the
// legacy heartbeat compatibility namespace.
func IsHeartbeatCompatTaskID(id string) bool {
	return strings.HasPrefix(strings.TrimSpace(id), legacyHeartbeatTaskIDPrefix)
}

func normalizeLegacyHeartbeatApprovalMode(mode string) (ApprovalMode, string) {
	normalized := ApprovalMode(strings.ToLower(strings.TrimSpace(mode)))
	switch normalized {
	case "":
		return ApprovalModeYolo, legacyApprovalModeEmptyDefault
	case ApprovalModeAsk, ApprovalModeAuto, ApprovalModeYolo:
		return normalized, legacyApprovalModeExplicit
	default:
		return ApprovalModeYolo, legacyApprovalModeInvalidDefault
	}
}

func unixMillisTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}
