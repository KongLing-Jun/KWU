package automation

import (
	"errors"
	"testing"
	"time"
)

func TestDueOnce(t *testing.T) {
	runAt := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	task := Task{Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: runAt}}

	before := Due(task, runAt.Add(-time.Minute))
	if before.Due || before.SkipReason != "not due yet" || !before.NextRunAt.Equal(runAt) {
		t.Fatalf("before runAt = %+v, want not due until %s", before, runAt)
	}
	at := Due(task, runAt)
	if !at.Due || !at.NextRunAt.Equal(runAt) {
		t.Fatalf("at runAt = %+v, want due", at)
	}
	task.LastRunAt = runAt
	afterRun := Due(task, runAt.Add(time.Hour))
	if afterRun.Due || afterRun.SkipReason != "already ran" {
		t.Fatalf("after run = %+v, want already ran", afterRun)
	}
}

func TestDueInterval(t *testing.T) {
	createdAt := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	task := Task{
		CreatedAt: createdAt,
		Schedule:  Schedule{Kind: ScheduleKindInterval, Every: "30m"},
	}

	before := Due(task, createdAt.Add(29*time.Minute))
	if before.Due || before.SkipReason != "not due yet" || !before.NextRunAt.Equal(createdAt.Add(30*time.Minute)) {
		t.Fatalf("first interval before due = %+v", before)
	}
	first := Due(task, createdAt.Add(30*time.Minute))
	if !first.Due || !first.NextRunAt.Equal(createdAt.Add(30*time.Minute)) {
		t.Fatalf("first interval due = %+v", first)
	}

	task.LastRunAt = createdAt.Add(30 * time.Minute)
	later := Due(task, createdAt.Add(59*time.Minute))
	if later.Due || !later.NextRunAt.Equal(createdAt.Add(time.Hour)) {
		t.Fatalf("subsequent interval before due = %+v", later)
	}
	next := Due(task, createdAt.Add(time.Hour))
	if !next.Due || !next.NextRunAt.Equal(createdAt.Add(time.Hour)) {
		t.Fatalf("subsequent interval due = %+v", next)
	}
}

func TestDueCalendar(t *testing.T) {
	tests := []struct {
		name      string
		calendar  CalendarSchedule
		createdAt time.Time
		before    time.Time
		at        time.Time
	}{
		{
			name:      "daily",
			calendar:  CalendarSchedule{Frequency: "daily", TimeOfDay: "09:00"},
			createdAt: time.Date(2026, 6, 27, 8, 30, 0, 0, time.UTC),
			before:    time.Date(2026, 6, 27, 8, 59, 0, 0, time.UTC),
			at:        time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC),
		},
		{
			name:      "weekly",
			calendar:  CalendarSchedule{Frequency: "weekly", TimeOfDay: "09:00", Weekdays: []string{"fri"}},
			createdAt: time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC),
			before:    time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
			at:        time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC),
		},
		{
			name:      "monthly",
			calendar:  CalendarSchedule{Frequency: "monthly", TimeOfDay: "09:00", MonthDays: []int{15}},
			createdAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			before:    time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
			at:        time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC),
		},
		{
			name:      "yearly",
			calendar:  CalendarSchedule{Frequency: "yearly", TimeOfDay: "09:00", Months: []int{6}, MonthDays: []int{27}},
			createdAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			before:    time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
			at:        time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{
				CreatedAt: tt.createdAt,
				Schedule:  Schedule{Kind: ScheduleKindCalendar, Calendar: tt.calendar},
			}
			before := Due(task, tt.before)
			if before.Due || before.SkipReason != "not due yet" {
				t.Fatalf("before = %+v, want not due yet", before)
			}
			at := Due(task, tt.at)
			if !at.Due || !at.NextRunAt.Equal(tt.at) {
				t.Fatalf("at = %+v, want due at %s", at, tt.at)
			}
			task.LastRunAt = tt.at
			again := Due(task, tt.at.Add(time.Minute))
			if again.Due || again.SkipReason != "already ran" {
				t.Fatalf("again = %+v, want already ran", again)
			}
		})
	}
}

func TestDueCalendarClampsMonthDay(t *testing.T) {
	task := Task{
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Schedule: Schedule{
			Kind:     ScheduleKindCalendar,
			Calendar: CalendarSchedule{Frequency: "monthly", TimeOfDay: "09:00", MonthDays: []int{31}},
		},
	}
	now := time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)
	got := Due(task, now)
	if !got.Due || !got.NextRunAt.Equal(now) {
		t.Fatalf("monthly clamp = %+v, want due on Feb 28", got)
	}
}

func TestDueCalendarBiweeklyUsesCreatedWeekAsAnchor(t *testing.T) {
	createdAt := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC) // Monday.
	task := Task{
		CreatedAt: createdAt,
		Schedule: Schedule{
			Kind: ScheduleKindCalendar,
			Calendar: CalendarSchedule{
				Frequency: "biweekly",
				TimeOfDay: "09:00",
				Weekdays:  []string{"mon"},
			},
		},
	}

	first := Due(task, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))
	if !first.Due {
		t.Fatalf("first anchored week = %+v, want due", first)
	}
	task.LastRunAt = first.NextRunAt
	if skippedWeek := Due(task, time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)); skippedWeek.Due {
		t.Fatalf("alternating week = %+v, want not due", skippedWeek)
	}
	second := Due(task, time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC))
	if !second.Due || !second.NextRunAt.Equal(time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("second anchored week = %+v, want due on June 15", second)
	}
}

func TestDueTimeWindow(t *testing.T) {
	lastRun := time.Date(2026, 6, 27, 16, 0, 0, 0, time.UTC)
	task := Task{
		LastRunAt: lastRun,
		Schedule: Schedule{
			Kind:   ScheduleKindInterval,
			Every:  "30m",
			Window: TimeWindow{Start: "09:00", End: "17:00"},
		},
	}
	if got := Due(task, lastRun.Add(30*time.Minute)); !got.Due {
		t.Fatalf("inside window = %+v, want due", got)
	}
	if got := Due(task, time.Date(2026, 6, 27, 17, 0, 0, 0, time.UTC)); got.Due || got.SkipReason != "outside time window" {
		t.Fatalf("at exclusive end = %+v, want outside window", got)
	}

	task.Schedule.Window = TimeWindow{Start: "22:00", End: "06:00"}
	if got := Due(task, time.Date(2026, 6, 27, 23, 0, 0, 0, time.UTC)); !got.Due {
		t.Fatalf("cross-midnight inside = %+v, want due", got)
	}
	if got := Due(task, time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)); got.Due || got.SkipReason != "outside time window" {
		t.Fatalf("cross-midnight outside = %+v, want outside window", got)
	}
}

func TestDueUsesScheduleTimezoneForWindow(t *testing.T) {
	task := Task{
		LastRunAt: time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC),
		Schedule: Schedule{
			Kind:     ScheduleKindInterval,
			Every:    "30m",
			Timezone: "Asia/Shanghai",
			Window:   TimeWindow{Start: "09:00", End: "10:00"},
		},
	}
	if got := Due(task, time.Date(2026, 6, 27, 1, 30, 0, 0, time.UTC)); !got.Due {
		t.Fatalf("timezone window = %+v, want due in Shanghai window", got)
	}
	if got := Due(task, time.Date(2026, 6, 27, 2, 30, 0, 0, time.UTC)); got.Due || got.SkipReason != "outside time window" {
		t.Fatalf("timezone outside = %+v, want outside Shanghai window", got)
	}
}

func TestDueInvalidInputsReturnReason(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		task Task
		want string
	}{
		{
			name: "invalid interval",
			task: Task{Schedule: Schedule{Kind: ScheduleKindInterval, Every: "soon"}},
			want: "invalid interval",
		},
		{
			name: "invalid timezone",
			task: Task{Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now, Timezone: "Mars/Base"}},
			want: "invalid timezone",
		},
		{
			name: "invalid window",
			task: Task{Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now, Window: TimeWindow{Start: "25:00"}}},
			want: "invalid time window",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Due(tt.task, now)
			if got.Due || got.SkipReason != tt.want {
				t.Fatalf("Due = %+v, want reason %q", got, tt.want)
			}
		})
	}
}

func TestValidateScheduleRejectsInvalidKindSpecificContent(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		schedule Schedule
	}{
		{name: "once requires runAt", schedule: Schedule{Kind: ScheduleKindOnce}},
		{name: "once rejects interval", schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now, Every: "1h"}},
		{name: "interval requires positive duration", schedule: Schedule{Kind: ScheduleKindInterval, Every: "0s"}},
		{name: "interval rejects runAt", schedule: Schedule{Kind: ScheduleKindInterval, Every: "1h", RunAt: now}},
		{name: "calendar requires exact clock", schedule: Schedule{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "daily", TimeOfDay: "9:00"}}},
		{name: "weekly requires weekdays", schedule: Schedule{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "weekly", TimeOfDay: "09:00"}}},
		{name: "weekday must be recognized", schedule: Schedule{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "weekly", TimeOfDay: "09:00", Weekdays: []string{"funday"}}}},
		{name: "monthly day must be in range", schedule: Schedule{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "monthly", TimeOfDay: "09:00", MonthDays: []int{32}}}},
		{name: "yearly month must be in range", schedule: Schedule{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "yearly", TimeOfDay: "09:00", Months: []int{13}, MonthDays: []int{1}}}},
		{name: "timezone must exist", schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now, Timezone: "Mars/Base"}},
		{name: "window requires exact clock", schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now, Window: TimeWindow{Start: "9:00", End: "17:00"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateSchedule(tt.schedule); !errors.Is(err, ErrInvalidSchedule) {
				t.Fatalf("ValidateSchedule() error = %v, want %v", err, ErrInvalidSchedule)
			}
		})
	}
}

func TestValidateScheduleAcceptsSupportedDefinitions(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	schedules := []Schedule{
		{Kind: ScheduleKindOnce, RunAt: now, Timezone: "Asia/Shanghai"},
		{Kind: ScheduleKindInterval, Every: "30m", Window: TimeWindow{Start: "09:00", End: "17:00"}},
		{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "daily", TimeOfDay: "09:00"}},
		{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "weekly", TimeOfDay: "09:00", Weekdays: []string{"mon", "fri"}}},
		{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "biweekly", TimeOfDay: "09:00", Weekdays: []string{"mon"}}},
		{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "monthly", TimeOfDay: "09:00", MonthDays: []int{1, 31}}},
		{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "yearly", TimeOfDay: "09:00", Months: []int{1, 12}, MonthDays: []int{1, 31}}},
	}
	for _, schedule := range schedules {
		if err := ValidateSchedule(schedule); err != nil {
			t.Errorf("ValidateSchedule(%+v) = %v", schedule, err)
		}
	}
}

func TestDueHonorsEffectiveRange(t *testing.T) {
	from := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 6, 4, 23, 59, 0, 0, time.UTC)
	task := Task{
		CreatedAt:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		EffectiveRange: EffectiveRange{From: from, Until: until},
		Schedule: Schedule{
			Kind:     ScheduleKindCalendar,
			Calendar: CalendarSchedule{Frequency: "daily", TimeOfDay: "09:00"},
		},
	}
	if before := Due(task, from.Add(-time.Minute)); before.Due || before.SkipReason != "before effective range" {
		t.Fatalf("before range = %+v", before)
	}
	at := Due(task, time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC))
	if !at.Due || !at.NextRunAt.Equal(time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("inside range = %+v", at)
	}
	if after := Due(task, until.Add(time.Minute)); after.Due || after.SkipReason != "after effective range" {
		t.Fatalf("after range = %+v", after)
	}
}

func TestDueDoesNotRunOccurrenceScheduledBeforeEffectiveRange(t *testing.T) {
	from := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	task := Task{
		EffectiveRange: EffectiveRange{From: from},
		Schedule: Schedule{
			Kind:  ScheduleKindOnce,
			RunAt: from.Add(-time.Hour),
		},
	}
	got := Due(task, from)
	if got.Due || got.SkipReason != "scheduled outside effective range" {
		t.Fatalf("Due() = %+v, want occurrence outside effective range", got)
	}
}

func TestPreviewNextRunsUsesScheduleSemanticsAndEffectiveRange(t *testing.T) {
	task := Task{
		CreatedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
		EffectiveRange: EffectiveRange{
			From:  time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			Until: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		Schedule: Schedule{
			Kind: ScheduleKindCalendar,
			Calendar: CalendarSchedule{
				Frequency: "biweekly",
				TimeOfDay: "09:00",
				Weekdays:  []string{"mon"},
			},
		},
	}
	runs, err := PreviewNextRuns(task, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), 3)
	if err != nil {
		t.Fatalf("PreviewNextRuns() error = %v", err)
	}
	want := []time.Time{
		time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC),
	}
	if len(runs) != len(want) {
		t.Fatalf("PreviewNextRuns() = %v, want %v", runs, want)
	}
	for i := range want {
		if !runs[i].Equal(want[i]) {
			t.Errorf("run[%d] = %s, want %s", i, runs[i], want[i])
		}
	}
}

func TestPreviewNextRunsBiweeklyKeepsLocalWeekAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	task := Task{
		CreatedAt: time.Date(2026, 3, 2, 8, 0, 0, 0, loc),
		Schedule: Schedule{
			Kind:     ScheduleKindCalendar,
			Timezone: "America/New_York",
			Calendar: CalendarSchedule{Frequency: "biweekly", TimeOfDay: "09:00", Weekdays: []string{"mon"}},
		},
	}
	runs, err := PreviewNextRuns(task, time.Date(2026, 3, 2, 9, 0, 0, 0, loc), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 16, 9, 0, 0, 0, loc)
	if len(runs) != 1 || !runs[0].Equal(want) {
		t.Fatalf("PreviewNextRuns() = %v, want [%s]", runs, want)
	}
}

func TestIntervalScheduleWeekdaysFiltersDueAndPreview(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 8, 7, 9, 0, 0, 0, loc) // Friday
	task := Task{
		ID: "weekday-interval", Name: "weekday interval", Enabled: true,
		Prompt: PromptConfig{Prompt: "run"}, CreatedAt: created,
		Schedule: Schedule{Kind: ScheduleKindInterval, Every: "24h", Weekdays: []string{"mon", "tue", "wed", "thu", "fri"}, Timezone: "Asia/Shanghai"},
	}
	if err := ValidateSchedule(task.Schedule); err != nil {
		t.Fatalf("ValidateSchedule() error = %v", err)
	}
	preview, err := PreviewNextRuns(task, created, 2)
	if err != nil {
		t.Fatalf("PreviewNextRuns() error = %v", err)
	}
	wantMonday := time.Date(2026, 8, 10, 9, 0, 0, 0, loc)
	if len(preview) != 2 || !preview[0].Equal(wantMonday) {
		t.Fatalf("PreviewNextRuns() = %v, want Monday first", preview)
	}
	sunday := time.Date(2026, 8, 9, 10, 0, 0, 0, loc)
	due := Due(task, sunday)
	if due.Due || !due.NextRunAt.Equal(wantMonday) {
		t.Fatalf("Due() on Sunday = %+v, want next Monday", due)
	}
}

func TestIntervalScheduleRejectsInvalidWeekdays(t *testing.T) {
	schedule := Schedule{Kind: ScheduleKindInterval, Every: "1h", Weekdays: []string{"monday", "monday"}}
	if err := ValidateSchedule(schedule); !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("ValidateSchedule() error = %v, want ErrInvalidSchedule", err)
	}
}
