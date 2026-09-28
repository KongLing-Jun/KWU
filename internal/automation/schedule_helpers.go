package automation

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseScheduleWeekdays(values []string) (map[time.Weekday]bool, bool) {
	out := map[time.Weekday]bool{}
	for _, value := range values {
		weekday, ok := parseScheduleWeekday(value)
		if !ok {
			continue
		}
		out[weekday] = true
	}
	return out, len(out) > 0
}

func parseScheduleWeekday(value string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sun", "sunday":
		return time.Sunday, true
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tuesday":
		return time.Tuesday, true
	case "wed", "wednesday":
		return time.Wednesday, true
	case "thu", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	default:
		return time.Sunday, false
	}
}

func parseOptionalClock(value string) (int, bool, error) {
	if strings.TrimSpace(value) == "" {
		return 0, false, nil
	}
	hour, minute, ok := parseScheduleClock(value)
	if !ok {
		return 0, false, fmt.Errorf("invalid clock %q", value)
	}
	return hour*60 + minute, true, nil
}

func parseScheduleClock(value string) (int, int, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

func timeAtMinutes(day time.Time, minutes int) time.Time {
	return dateAtSchedule(day.Year(), day.Month(), day.Day(), minutes/60, minutes%60, day.Location())
}

func dateAtSchedule(year int, month time.Month, day, hour, minute int, loc *time.Location) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, loc)
}

func monthlyScheduleCandidate(year int, month time.Month, day, hour, minute int, loc *time.Location) time.Time {
	if max := daysInScheduleMonth(year, month, loc); day > max {
		day = max
	}
	return dateAtSchedule(year, month, day, hour, minute, loc)
}

func daysInScheduleMonth(year int, month time.Month, loc *time.Location) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
}

func calendarPeriodStart(frequency string, now time.Time) time.Time {
	switch frequency {
	case "daily":
		return dateAtSchedule(now.Year(), now.Month(), now.Day(), 0, 0, now.Location())
	case "weekly":
		dayOffset := (int(now.Weekday()) + 6) % 7
		return dateAtSchedule(now.Year(), now.Month(), now.Day(), 0, 0, now.Location()).AddDate(0, 0, -dayOffset)
	case "biweekly":
		dayOffset := (int(now.Weekday()) + 6) % 7
		return dateAtSchedule(now.Year(), now.Month(), now.Day(), 0, 0, now.Location()).AddDate(0, 0, -dayOffset)
	case "monthly":
		return dateAtSchedule(now.Year(), now.Month(), 1, 0, 0, now.Location())
	case "yearly":
		return dateAtSchedule(now.Year(), time.January, 1, 0, 0, now.Location())
	default:
		return now
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
