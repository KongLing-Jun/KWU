package automation

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// DueResult is the pure scheduling decision for one task at one moment.
type DueResult struct {
	Due        bool
	NextRunAt  time.Time
	SkipReason string
}

// Due 按任务自己的时区、时间窗口和调度类型判断 now 是否到期。
// 这是纯函数，Engine、Desktop 和 serve 都应复用这里，避免各宿主重复实现调度规则。
func Due(task Task, now time.Time) DueResult {
	if result, stop := scheduleDuePrecheck(task, now); stop {
		return result
	}
	loc, err := scheduleLocation(task.Schedule.Timezone)
	if err != nil {
		return notDue("invalid timezone", time.Time{})
	}
	localNow := now.In(loc)
	if ok, next, err := withinTimeWindow(task.Schedule.Window, localNow); err != nil {
		return notDue("invalid time window", time.Time{})
	} else if !ok {
		return notDue("outside time window", next)
	}

	return applyEffectiveRange(task, dueByKind(task, localNow), localNow)
}

func scheduleDuePrecheck(task Task, now time.Time) (DueResult, bool) {
	if err := ValidateSchedule(task.Schedule); err != nil {
		if validation, ok := err.(*scheduleValidationError); ok {
			return notDue(validation.skipReason, time.Time{}), true
		}
		return notDue("invalid schedule", time.Time{}), true
	}
	if !task.EffectiveRange.From.IsZero() && now.Before(task.EffectiveRange.From) {
		return notDue("before effective range", task.EffectiveRange.From), true
	}
	if !task.EffectiveRange.Until.IsZero() && now.After(task.EffectiveRange.Until) {
		return notDue("after effective range", time.Time{}), true
	}
	return DueResult{}, false
}

func dueByKind(task Task, now time.Time) DueResult {
	switch normalizeScheduleKind(task.Schedule.Kind) {
	case ScheduleKindOnce:
		return dueOnce(task, now)
	case ScheduleKindInterval:
		return dueInterval(task, now)
	case ScheduleKindCalendar:
		return dueCalendar(task, now)
	default:
		return notDue("invalid schedule kind", time.Time{})
	}
}

func applyEffectiveRange(task Task, result DueResult, now time.Time) DueResult {
	occurrence := result.NextRunAt
	if occurrence.IsZero() {
		return result
	}
	from := task.EffectiveRange.From
	if !from.IsZero() && occurrence.Before(from) {
		next, ok := nextScheduledAfter(task, from.In(now.Location()).Add(-time.Nanosecond))
		if !ok {
			return notDue("scheduled outside effective range", time.Time{})
		}
		if until := task.EffectiveRange.Until; !until.IsZero() && next.After(until) {
			return notDue("scheduled outside effective range", time.Time{})
		}
		if next.After(now) {
			return notDue("not due yet", next)
		}
		return DueResult{Due: true, NextRunAt: next}
	}
	if until := task.EffectiveRange.Until; !until.IsZero() && occurrence.After(until) {
		return notDue("scheduled outside effective range", time.Time{})
	}
	return result
}

// PreviewNextRuns returns up to count occurrences strictly after after. The
// result is bounded by the task's effective range and never mutates the task.
func PreviewNextRuns(task Task, after time.Time, count int) ([]time.Time, error) {
	if err := validatePreviewRequest(task.Schedule, count); err != nil {
		return nil, err
	}
	loc, err := scheduleLocation(task.Schedule.Timezone)
	if err != nil {
		return nil, err
	}
	cursor := after.In(loc)
	if from := task.EffectiveRange.From; !from.IsZero() && !from.Before(cursor) {
		cursor = from.In(loc).Add(-time.Nanosecond)
	}
	return collectNextRuns(task, cursor, count), nil
}

func validatePreviewRequest(schedule Schedule, count int) error {
	if count < 1 {
		return fmt.Errorf("preview count must be positive")
	}
	if count > 100 {
		return fmt.Errorf("preview count must not exceed 100")
	}
	return ValidateSchedule(schedule)
}

func collectNextRuns(task Task, cursor time.Time, count int) []time.Time {
	runs := make([]time.Time, 0, count)
	for len(runs) < count {
		next, ok := nextScheduledAfter(task, cursor)
		if !ok {
			break
		}
		if until := task.EffectiveRange.Until; !until.IsZero() && next.After(until) {
			break
		}
		if from := task.EffectiveRange.From; !from.IsZero() && next.Before(from) {
			cursor = next
			continue
		}
		runs = append(runs, next)
		cursor = next
	}
	return runs
}

func nextScheduledAfter(task Task, after time.Time) (time.Time, bool) {
	schedule := task.Schedule
	switch normalizeScheduleKind(schedule.Kind) {
	case ScheduleKindOnce:
		runAt := schedule.RunAt.In(after.Location())
		return runAt, runAt.After(after)
	case ScheduleKindInterval:
		base := task.LastRunAt
		if base.IsZero() {
			base = task.CreatedAt
		}
		if base.IsZero() {
			base = after
		}
		return nextIntervalOccurrence(base.In(after.Location()), schedule, after)
	case ScheduleKindCalendar:
		calendar := schedule.Calendar
		hour, minute, _ := parseScheduleClock(calendar.TimeOfDay)
		return nextCalendarOccurrenceAfter(
			calendar,
			strings.ToLower(strings.TrimSpace(calendar.Frequency)),
			hour,
			minute,
			after,
			calendarScheduleAnchor(task, after),
		)
	default:
		return time.Time{}, false
	}
}

type scheduleValidationError struct {
	skipReason string
	detail     string
}

func (e *scheduleValidationError) Error() string {
	return fmt.Sprintf("%v: %s", ErrInvalidSchedule, e.detail)
}

func (e *scheduleValidationError) Unwrap() error { return ErrInvalidSchedule }

func invalidSchedule(skipReason, format string, args ...any) error {
	return &scheduleValidationError{skipReason: skipReason, detail: fmt.Sprintf(format, args...)}
}

// ValidateSchedule checks the complete schedule definition before it is stored
// or used. Empty timezone remains the documented UTC default.
func ValidateSchedule(schedule Schedule) error {
	kind := normalizeScheduleKind(schedule.Kind)
	if !validScheduleKind(kind) {
		return invalidSchedule("invalid schedule kind", "unsupported kind %q", schedule.Kind)
	}
	if _, err := scheduleLocation(schedule.Timezone); err != nil {
		return invalidSchedule("invalid timezone", "timezone %q does not exist", schedule.Timezone)
	}
	if err := validateTimeWindow(schedule.Window); err != nil {
		return err
	}

	return validateScheduleKindContent(kind, schedule)
}

func validateScheduleKindContent(kind ScheduleKind, schedule Schedule) error {
	switch kind {
	case ScheduleKindOnce:
		return validateOnceSchedule(schedule)
	case ScheduleKindInterval:
		return validateIntervalSchedule(schedule)
	case ScheduleKindCalendar:
		return validateCalendarDefinition(schedule)
	}
	return nil
}

func validateOnceSchedule(schedule Schedule) error {
	if schedule.RunAt.IsZero() {
		return invalidSchedule("missing run time", "once schedule requires runAt")
	}
	if strings.TrimSpace(schedule.Every) != "" || len(schedule.Weekdays) > 0 || !isZeroCalendarSchedule(schedule.Calendar) {
		return invalidSchedule("invalid schedule", "once schedule contains interval or calendar fields")
	}
	return nil
}

func validateIntervalSchedule(schedule Schedule) error {
	every, err := time.ParseDuration(strings.TrimSpace(schedule.Every))
	if err != nil || every <= 0 {
		return invalidSchedule("invalid interval", "interval every must be a positive duration")
	}
	if len(schedule.Weekdays) > 0 && !validWeekdayList(schedule.Weekdays) {
		return invalidSchedule("invalid interval", "interval weekdays must be unique valid weekdays")
	}
	if !schedule.RunAt.IsZero() || !isZeroCalendarSchedule(schedule.Calendar) {
		return invalidSchedule("invalid interval", "interval schedule contains once or calendar fields")
	}
	return nil
}

func validateCalendarDefinition(schedule Schedule) error {
	if !schedule.RunAt.IsZero() || strings.TrimSpace(schedule.Every) != "" || len(schedule.Weekdays) > 0 {
		return invalidSchedule("invalid calendar", "calendar schedule contains once or interval fields")
	}
	return validateCalendarSchedule(schedule.Calendar)
}

func validateTimeWindow(window TimeWindow) error {
	start := strings.TrimSpace(window.Start)
	end := strings.TrimSpace(window.End)
	if start != "" && !isExactScheduleClock(start) {
		return invalidSchedule("invalid time window", "window start must use HH:MM")
	}
	if end != "" && !isExactScheduleClock(end) {
		return invalidSchedule("invalid time window", "window end must use HH:MM")
	}
	if start != "" && start == end {
		return invalidSchedule("invalid time window", "window start and end must differ")
	}
	return nil
}

func validateCalendarSchedule(calendar CalendarSchedule) error {
	frequency := strings.ToLower(strings.TrimSpace(calendar.Frequency))
	if !isExactScheduleClock(strings.TrimSpace(calendar.TimeOfDay)) {
		return invalidSchedule("invalid calendar time", "calendar timeOfDay must use HH:MM")
	}
	switch frequency {
	case "daily":
		return validateDailyCalendar(calendar)
	case "weekly", "biweekly":
		return validateWeeklyCalendar(calendar, frequency)
	case "monthly":
		return validateMonthlyCalendar(calendar)
	case "yearly":
		return validateYearlyCalendar(calendar)
	default:
		return invalidSchedule("invalid calendar", "unsupported calendar frequency %q", calendar.Frequency)
	}
}

func validateDailyCalendar(calendar CalendarSchedule) error {
	if len(calendar.Weekdays) > 0 || len(calendar.MonthDays) > 0 || len(calendar.Months) > 0 {
		return invalidSchedule("invalid calendar", "daily schedule contains recurrence selectors")
	}
	return nil
}

func validateWeeklyCalendar(calendar CalendarSchedule, frequency string) error {
	if !validWeekdayList(calendar.Weekdays) {
		return invalidSchedule("invalid calendar", "%s schedule requires valid weekdays", frequency)
	}
	if len(calendar.MonthDays) > 0 || len(calendar.Months) > 0 {
		return invalidSchedule("invalid calendar", "%s schedule contains month selectors", frequency)
	}
	return nil
}

func validateMonthlyCalendar(calendar CalendarSchedule) error {
	if !validMonthDayList(calendar.MonthDays) {
		return invalidSchedule("invalid calendar", "monthly schedule requires monthDays from 1 through 31")
	}
	if len(calendar.Weekdays) > 0 || len(calendar.Months) > 0 {
		return invalidSchedule("invalid calendar", "monthly schedule contains weekday or month selectors")
	}
	return nil
}

func validateYearlyCalendar(calendar CalendarSchedule) error {
	if !validMonthList(calendar.Months) || !validMonthDayList(calendar.MonthDays) {
		return invalidSchedule("invalid calendar", "yearly schedule requires months 1 through 12 and monthDays 1 through 31")
	}
	if len(calendar.Weekdays) > 0 {
		return invalidSchedule("invalid calendar", "yearly schedule contains weekday selectors")
	}
	return nil
}

func isZeroCalendarSchedule(calendar CalendarSchedule) bool {
	return strings.TrimSpace(calendar.Frequency) == "" &&
		strings.TrimSpace(calendar.TimeOfDay) == "" &&
		len(calendar.Weekdays) == 0 && len(calendar.MonthDays) == 0 && len(calendar.Months) == 0
}

func validWeekdayList(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[time.Weekday]struct{}, len(values))
	for _, value := range values {
		weekday, ok := parseScheduleWeekday(value)
		if !ok {
			return false
		}
		if _, duplicate := seen[weekday]; duplicate {
			return false
		}
		seen[weekday] = struct{}{}
	}
	return true
}

func validMonthDayList(values []int) bool {
	return validUniqueInts(values, 1, 31)
}

func validMonthList(values []int) bool {
	return validUniqueInts(values, 1, 12)
}

func validUniqueInts(values []int, min, max int) bool {
	if len(values) == 0 {
		return false
	}
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value < min || value > max {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func isExactScheduleClock(value string) bool {
	if len(value) != len("00:00") || value[2] != ':' {
		return false
	}
	for _, index := range []int{0, 1, 3, 4} {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	_, _, ok := parseScheduleClock(value)
	return ok
}

func dueOnce(task Task, now time.Time) DueResult {
	runAt := task.Schedule.RunAt
	if runAt.IsZero() {
		return notDue("missing run time", time.Time{})
	}
	runAt = runAt.In(now.Location())
	if !task.LastRunAt.IsZero() {
		return notDue("already ran", time.Time{})
	}
	if runAt.After(now) {
		return notDue("not due yet", runAt)
	}
	return DueResult{Due: true, NextRunAt: runAt}
}

func dueInterval(task Task, now time.Time) DueResult {
	if _, err := time.ParseDuration(strings.TrimSpace(task.Schedule.Every)); err != nil {
		return notDue("invalid interval", time.Time{})
	}
	base := task.LastRunAt
	if base.IsZero() {
		base = task.CreatedAt
	}
	if base.IsZero() {
		if intervalWeekdayAllowed(task.Schedule, now) {
			return DueResult{Due: true, NextRunAt: now}
		}
		next, ok := nextIntervalOccurrence(now, task.Schedule, now)
		if !ok {
			return notDue("invalid interval weekdays", time.Time{})
		}
		return notDue("weekday filtered", next)
	}
	next, ok := nextIntervalOccurrence(base.In(now.Location()), task.Schedule, base.In(now.Location()))
	if !ok {
		return notDue("invalid interval weekdays", time.Time{})
	}
	if next.After(now) {
		return notDue("not due yet", next)
	}
	return DueResult{Due: true, NextRunAt: next}
}

func nextIntervalOccurrence(base time.Time, schedule Schedule, after time.Time) (time.Time, bool) {
	every, err := time.ParseDuration(strings.TrimSpace(schedule.Every))
	if err != nil || every <= 0 {
		return time.Time{}, false
	}
	next := base.Add(every)
	if !next.After(after) {
		steps := after.Sub(base)/every + 1
		next = base.Add(steps * every)
	}
	if len(schedule.Weekdays) == 0 {
		return next, true
	}
	for attempts := 0; attempts < 8; attempts++ {
		if intervalWeekdayAllowed(schedule, next) {
			return next, true
		}
		target := nextDayStart(next)
		steps := target.Sub(base) / every
		if base.Add(steps * every).Before(target) {
			steps++
		}
		next = base.Add(steps * every)
		if !next.After(after) {
			next = next.Add(every)
		}
	}
	return time.Time{}, false
}

func intervalWeekdayAllowed(schedule Schedule, at time.Time) bool {
	if len(schedule.Weekdays) == 0 {
		return true
	}
	weekdays, ok := parseScheduleWeekdays(schedule.Weekdays)
	return ok && weekdays[at.Weekday()]
}

func nextDayStart(at time.Time) time.Time {
	next := at.AddDate(0, 0, 1)
	return time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
}

func dueCalendar(task Task, now time.Time) DueResult {
	cal := task.Schedule.Calendar
	hour, minute, ok := parseScheduleClock(defaultString(cal.TimeOfDay, "09:00"))
	if !ok {
		return notDue("invalid calendar time", time.Time{})
	}
	frequency := strings.ToLower(strings.TrimSpace(cal.Frequency))
	anchor := calendarScheduleAnchor(task, now)
	prev, ok := previousCalendarOccurrence(cal, frequency, hour, minute, now, anchor)
	if !ok {
		return notDue("invalid calendar", time.Time{})
	}

	anchor = task.CreatedAt.In(now.Location())
	if task.CreatedAt.IsZero() && task.LastRunAt.IsZero() {
		anchor = calendarPeriodStart(frequency, now)
	}
	if !anchor.IsZero() && prev.Before(anchor) {
		next, _ := nextCalendarOccurrenceAfter(cal, frequency, hour, minute, anchor.Add(-time.Nanosecond), calendarScheduleAnchor(task, now))
		return notDue("not due yet", next)
	}
	if !task.LastRunAt.IsZero() && !task.LastRunAt.In(now.Location()).Before(prev) {
		next, _ := nextCalendarOccurrenceAfter(cal, frequency, hour, minute, prev, calendarScheduleAnchor(task, now))
		return notDue("already ran", next)
	}
	return DueResult{Due: true, NextRunAt: prev}
}

func notDue(reason string, next time.Time) DueResult {
	return DueResult{SkipReason: reason, NextRunAt: next}
}

func scheduleLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(name)
}

func withinTimeWindow(w TimeWindow, now time.Time) (bool, time.Time, error) {
	start, startOK, err := parseOptionalClock(w.Start)
	if err != nil {
		return false, time.Time{}, err
	}
	end, endOK, err := parseOptionalClock(w.End)
	if err != nil {
		return false, time.Time{}, err
	}
	if !startOK && !endOK {
		return true, time.Time{}, nil
	}
	minutes := now.Hour()*60 + now.Minute()
	if startOK && !endOK {
		if minutes >= start {
			return true, time.Time{}, nil
		}
		return false, timeAtMinutes(now, start), nil
	}
	if !startOK && endOK {
		if minutes < end {
			return true, time.Time{}, nil
		}
		return false, timeAtMinutes(now.AddDate(0, 0, 1), 0), nil
	}
	if start < end {
		if minutes >= start && minutes < end {
			return true, time.Time{}, nil
		}
		if minutes < start {
			return false, timeAtMinutes(now, start), nil
		}
		return false, timeAtMinutes(now.AddDate(0, 0, 1), start), nil
	}
	if minutes >= start || minutes < end {
		return true, time.Time{}, nil
	}
	return false, timeAtMinutes(now, start), nil
}

func previousCalendarOccurrence(cal CalendarSchedule, frequency string, hour, minute int, now, anchor time.Time) (time.Time, bool) {
	switch frequency {
	case "daily":
		candidate := dateAtSchedule(now.Year(), now.Month(), now.Day(), hour, minute, now.Location())
		if candidate.After(now) {
			candidate = candidate.AddDate(0, 0, -1)
		}
		return candidate, true
	case "weekly":
		return previousWeeklyOccurrence(cal, hour, minute, now, time.Time{})
	case "biweekly":
		return previousWeeklyOccurrence(cal, hour, minute, now, anchor)
	case "monthly":
		days, ok := parseMonthDays(cal.MonthDays)
		if !ok {
			return time.Time{}, false
		}
		return previousMonthlyOccurrence(days, hour, minute, now), true
	case "yearly":
		months, ok := parseMonths(cal.Months)
		if !ok {
			return time.Time{}, false
		}
		days, ok := parseMonthDays(cal.MonthDays)
		if !ok {
			return time.Time{}, false
		}
		return previousYearlyOccurrence(months, days, hour, minute, now), true
	default:
		return time.Time{}, false
	}
}

func previousWeeklyOccurrence(cal CalendarSchedule, hour, minute int, now, anchor time.Time) (time.Time, bool) {
	weekdays, ok := parseScheduleWeekdays(cal.Weekdays)
	if !ok {
		return time.Time{}, false
	}
	var best time.Time
	lookback := 7
	if !anchor.IsZero() {
		lookback = 14
	}
	for offset := 0; offset < lookback; offset++ {
		day := now.AddDate(0, 0, -offset)
		if !weekdays[day.Weekday()] {
			continue
		}
		candidate := dateAtSchedule(day.Year(), day.Month(), day.Day(), hour, minute, now.Location())
		if !anchor.IsZero() && !sameBiweeklyCycle(anchor, candidate) {
			continue
		}
		if !candidate.After(now) && (best.IsZero() || candidate.After(best)) {
			best = candidate
		}
	}
	return best, !best.IsZero()
}

func nextCalendarOccurrenceAfter(cal CalendarSchedule, frequency string, hour, minute int, after, anchor time.Time) (time.Time, bool) {
	switch frequency {
	case "daily":
		candidate := dateAtSchedule(after.Year(), after.Month(), after.Day(), hour, minute, after.Location())
		if !candidate.After(after) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		return candidate, true
	case "weekly":
		return nextWeeklyOccurrence(cal, hour, minute, after, time.Time{})
	case "biweekly":
		return nextWeeklyOccurrence(cal, hour, minute, after, anchor)
	case "monthly":
		days, ok := parseMonthDays(cal.MonthDays)
		if !ok {
			return time.Time{}, false
		}
		return nextMonthlyOccurrence(days, hour, minute, after), true
	case "yearly":
		months, ok := parseMonths(cal.Months)
		if !ok {
			return time.Time{}, false
		}
		days, ok := parseMonthDays(cal.MonthDays)
		if !ok {
			return time.Time{}, false
		}
		return nextYearlyOccurrence(months, days, hour, minute, after), true
	default:
		return time.Time{}, false
	}
}

func nextWeeklyOccurrence(cal CalendarSchedule, hour, minute int, after, anchor time.Time) (time.Time, bool) {
	weekdays, ok := parseScheduleWeekdays(cal.Weekdays)
	if !ok {
		return time.Time{}, false
	}
	var best time.Time
	lookahead := 7
	if !anchor.IsZero() {
		lookahead = 14
	}
	for offset := 0; offset <= lookahead; offset++ {
		day := after.AddDate(0, 0, offset)
		if !weekdays[day.Weekday()] {
			continue
		}
		candidate := dateAtSchedule(day.Year(), day.Month(), day.Day(), hour, minute, after.Location())
		if !anchor.IsZero() && !sameBiweeklyCycle(anchor, candidate) {
			continue
		}
		if candidate.After(after) && (best.IsZero() || candidate.Before(best)) {
			best = candidate
		}
	}
	return best, !best.IsZero()
}

func calendarScheduleAnchor(task Task, now time.Time) time.Time {
	if !task.CreatedAt.IsZero() {
		return task.CreatedAt.In(now.Location())
	}
	if !task.LastRunAt.IsZero() {
		return task.LastRunAt.In(now.Location())
	}
	return calendarPeriodStart("weekly", now)
}

func sameBiweeklyCycle(anchor, candidate time.Time) bool {
	anchorWeek := calendarPeriodStart("weekly", anchor)
	candidateWeek := calendarPeriodStart("weekly", candidate)
	anchorDate := time.Date(anchorWeek.Year(), anchorWeek.Month(), anchorWeek.Day(), 0, 0, 0, 0, time.UTC)
	candidateDate := time.Date(candidateWeek.Year(), candidateWeek.Month(), candidateWeek.Day(), 0, 0, 0, 0, time.UTC)
	weeks := int(candidateDate.Sub(anchorDate).Hours() / (24 * 7))
	return weeks%2 == 0
}

func previousMonthlyOccurrence(days []int, hour, minute int, now time.Time) time.Time {
	var best time.Time
	for offset := 0; offset <= 1; offset++ {
		month := now.AddDate(0, -offset, 0)
		for _, day := range days {
			candidate := monthlyScheduleCandidate(month.Year(), month.Month(), day, hour, minute, now.Location())
			if !candidate.After(now) && (best.IsZero() || candidate.After(best)) {
				best = candidate
			}
		}
	}
	return best
}

func nextMonthlyOccurrence(days []int, hour, minute int, after time.Time) time.Time {
	var best time.Time
	for offset := 0; offset <= 13; offset++ {
		month := after.AddDate(0, offset, 0)
		for _, day := range days {
			candidate := monthlyScheduleCandidate(month.Year(), month.Month(), day, hour, minute, after.Location())
			if candidate.After(after) && (best.IsZero() || candidate.Before(best)) {
				best = candidate
			}
		}
	}
	return best
}

func previousYearlyOccurrence(months, days []int, hour, minute int, now time.Time) time.Time {
	var best time.Time
	for year := now.Year() - 1; year <= now.Year(); year++ {
		for _, month := range months {
			for _, day := range days {
				candidate := monthlyScheduleCandidate(year, time.Month(month), day, hour, minute, now.Location())
				if !candidate.After(now) && (best.IsZero() || candidate.After(best)) {
					best = candidate
				}
			}
		}
	}
	return best
}

func nextYearlyOccurrence(months, days []int, hour, minute int, after time.Time) time.Time {
	var best time.Time
	for year := after.Year(); year <= after.Year()+1; year++ {
		for _, month := range months {
			for _, day := range days {
				candidate := monthlyScheduleCandidate(year, time.Month(month), day, hour, minute, after.Location())
				if candidate.After(after) && (best.IsZero() || candidate.Before(best)) {
					best = candidate
				}
			}
		}
	}
	return best
}

func parseMonthDays(values []int) ([]int, bool) {
	if len(values) == 0 {
		return []int{1}, true
	}
	days := append([]int(nil), values...)
	sort.Ints(days)
	for _, day := range days {
		if day < 1 || day > 31 {
			return nil, false
		}
	}
	return days, true
}

func parseMonths(values []int) ([]int, bool) {
	if len(values) == 0 {
		return []int{1}, true
	}
	months := append([]int(nil), values...)
	sort.Ints(months)
	for _, month := range months {
		if month < 1 || month > 12 {
			return nil, false
		}
	}
	return months, true
}
