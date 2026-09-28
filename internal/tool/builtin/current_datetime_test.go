package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kwu/internal/temporal"
	"kwu/internal/tool"
)

func TestCurrentDatetimeUsesSharedTemporalService(t *testing.T) {
	now := time.Date(2026, 7, 19, 16, 0, 1, 0, time.UTC)
	times := temporal.New(temporal.Options{
		Configured: "Asia/Shanghai",
		Clock:      temporal.ClockFunc(func() time.Time { return now }),
	})
	task := NewCurrentDatetime(times)

	out, err := task.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"local_datetime":"2026-07-20T00:00:01+08:00"`,
		`"utc_datetime":"2026-07-19T16:00:01Z"`,
		`"weekday":"Monday"`,
		`"weekday_iso":1`,
		`"timezone":"Asia/Shanghai"`,
		`"utc_offset":"+08:00"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %s: %s", want, out)
		}
	}
	if !task.ReadOnly() {
		t.Fatal("current_datetime must remain read-only")
	}
	classifier, ok := any(task).(tool.PlanModeClassifier)
	if !ok || !classifier.PlanModeSafe() {
		t.Fatal("current_datetime must explicitly remain plan-safe")
	}
}

func TestCurrentDatetimeSchemaIsFixedAndEmpty(t *testing.T) {
	task := NewCurrentDatetime(temporal.New(temporal.Options{}))
	before := string(task.Schema())
	if before != `{"type":"object","properties":{},"additionalProperties":false}` {
		t.Fatalf("schema = %s", before)
	}
	if _, err := task.Execute(context.Background(), json.RawMessage(`{"timezone":"UTC"}`)); err == nil {
		t.Fatal("unexpected arguments should be rejected")
	}
	if after := string(task.Schema()); after != before {
		t.Fatalf("schema changed: before=%s after=%s", before, after)
	}
}

func TestCurrentDatetimeDescriptionExcludesDateOnlyQuestions(t *testing.T) {
	description := NewCurrentDatetime(temporal.New(temporal.Options{})).Description()
	for _, want := range []string{
		"time-of-day, seconds, Unix timestamp, or exact-instant requests",
		"do not use for date-only or weekday-only questions",
		"temporal_context already provides those facts",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("description missing %q: %s", want, description)
		}
	}
}
