package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kwu/internal/temporal"
)

var currentDatetimeSchema = json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)

// CurrentDatetime is a session-scoped clock tool. It is registered eagerly so
// the model-visible tool list stays stable even when no time question is asked.
type CurrentDatetime struct {
	temporal *temporal.Service
}

func NewCurrentDatetime(times *temporal.Service) CurrentDatetime {
	return CurrentDatetime{temporal: times}
}

func (CurrentDatetime) Name() string { return "current_datetime" }

func (CurrentDatetime) Description() string {
	return "Return the exact current wall-clock time and timestamp from the host system clock in the user's resolved timezone. Use only for time-of-day, seconds, Unix timestamp, or exact-instant requests; do not use for date-only or weekday-only questions because temporal_context already provides those facts. Takes no arguments and does not use IP geolocation."
}

func (CurrentDatetime) Schema() json.RawMessage { return currentDatetimeSchema }

func (CurrentDatetime) ReadOnly() bool { return true }

func (CurrentDatetime) PlanModeSafe() bool { return true }

func (t CurrentDatetime) Execute(_ context.Context, args json.RawMessage) (string, error) {
	if t.temporal == nil {
		return "", fmt.Errorf("current_datetime: temporal service is unavailable")
	}
	var params map[string]json.RawMessage
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if len(params) != 0 {
		return "", fmt.Errorf("current_datetime takes no arguments")
	}

	snapshot := t.temporal.Snapshot()
	result := struct {
		LocalDateTime string `json:"local_datetime"`
		UTCDateTime   string `json:"utc_datetime"`
		Date          string `json:"date"`
		Time          string `json:"time"`
		Weekday       string `json:"weekday"`
		WeekdayISO    int    `json:"weekday_iso"`
		Timezone      string `json:"timezone"`
		UTCOffset     string `json:"utc_offset"`
		UnixSeconds   int64  `json:"unix_seconds"`
	}{
		LocalDateTime: snapshot.Local.Format(time.RFC3339),
		UTCDateTime:   snapshot.UTC.Format(time.RFC3339),
		Date:          snapshot.Date,
		Time:          snapshot.ClockTime,
		Weekday:       snapshot.Weekday.String(),
		WeekdayISO:    snapshot.WeekdayISO,
		Timezone:      snapshot.Timezone,
		UTCOffset:     snapshot.UTCOffset,
		UnixSeconds:   snapshot.UnixSeconds,
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal current datetime: %w", err)
	}
	return string(out), nil
}
