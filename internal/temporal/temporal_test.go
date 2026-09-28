package temporal

import (
	"strings"
	"testing"
	"time"
)

func fixedClock(value time.Time) Clock {
	return ClockFunc(func() time.Time { return value })
}

func TestSnapshotUsesConfiguredTimezoneAndComputesWeekday(t *testing.T) {
	now := time.Date(2026, 7, 19, 16, 30, 0, 0, time.UTC)
	s := New(Options{Configured: "Asia/Shanghai", Clock: fixedClock(now)})
	got := s.Snapshot()
	if got.Date != "2026-07-20" || got.ClockTime != "00:30:00" {
		t.Fatalf("local datetime = %s %s", got.Date, got.ClockTime)
	}
	if got.Weekday != time.Monday || got.WeekdayISO != 1 {
		t.Fatalf("weekday = %s/%d, want Monday/1", got.Weekday, got.WeekdayISO)
	}
	if got.Timezone != "Asia/Shanghai" || got.UTCOffset != "+08:00" {
		t.Fatalf("timezone = %s %s", got.Timezone, got.UTCOffset)
	}
}

func TestTimezoneResolutionPrecedenceAndOverrideUpdates(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	s := New(Options{Override: "America/New_York", Configured: "Asia/Shanghai", Clock: fixedClock(now)})
	if name, source := s.Timezone(); name != "America/New_York" || source != SourceOverride {
		t.Fatalf("timezone = %q/%q", name, source)
	}
	if got := s.Snapshot(); got.ClockTime != "07:00:00" || got.UTCOffset != "-05:00" {
		t.Fatalf("New York snapshot = %+v", got)
	}
	if err := s.SetOverride(""); err != nil {
		t.Fatal(err)
	}
	if name, source := s.Timezone(); name != "Asia/Shanghai" || source != SourceConfig {
		t.Fatalf("fallback timezone = %q/%q", name, source)
	}
}

func TestInvalidTimezoneWarnsAndFallsBack(t *testing.T) {
	var warnings []string
	s := New(Options{
		Override:   "Mars/Olympus",
		Configured: "UTC",
		Clock:      fixedClock(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)),
		Warn:       func(message string) { warnings = append(warnings, message) },
	})
	if name, source := s.Timezone(); name != "UTC" || source != SourceConfig {
		t.Fatalf("timezone = %q/%q", name, source)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Mars/Olympus") {
		t.Fatalf("warnings = %v", warnings)
	}
	if err := s.SetOverride("Not/AZone"); err == nil {
		t.Fatal("SetOverride accepted invalid timezone")
	}
	if name, _ := s.Timezone(); name != "UTC" {
		t.Fatalf("invalid update changed timezone to %q", name)
	}
}

func TestDSTOffsetComesFromLocationRules(t *testing.T) {
	s := New(Options{Configured: "America/New_York", Clock: fixedClock(time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC))})
	if got := s.Snapshot().UTCOffset; got != "-04:00" {
		t.Fatalf("summer offset = %q, want -04:00", got)
	}
}

func TestContextBlockContainsDateFactsButNotClockTime(t *testing.T) {
	s := New(Options{Configured: "Asia/Shanghai", Clock: fixedClock(time.Date(2026, 7, 20, 1, 2, 3, 0, time.FixedZone("test", 8*60*60)))})
	got := s.Snapshot().ContextBlock()
	want := `<temporal_context date="2026-07-20" weekday="Monday" weekday_iso="1" timezone="Asia/Shanghai" utc_offset="+08:00"/>`
	if got != want {
		t.Fatalf("ContextBlock = %q, want %q", got, want)
	}
	if strings.Contains(got, "01:02:03") {
		t.Fatalf("ContextBlock leaked exact clock time: %s", got)
	}
}
