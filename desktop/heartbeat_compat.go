package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"kwu/internal/automation"
)

func (a *App) heartbeatCompatListTasks() []HeartbeatTask {
	svc, _ := a.ensureAutomationService()
	views, err := svc.ListTasks(a.bootContext())
	if err != nil {
		return nil
	}
	tasks := make([]HeartbeatTask, 0, len(views))
	for _, view := range views {
		if !automation.IsHeartbeatCompatTaskID(view.ID) {
			continue
		}
		tasks = append(tasks, heartbeatCompatTaskFromAutomation(view.Task))
	}
	return tasks
}

func (a *App) heartbeatCompatSaveTasks(tasks []HeartbeatTask) error {
	svc, _ := a.ensureAutomationService()
	views, err := svc.ListTasks(a.bootContext())
	if err != nil {
		return err
	}
	next := make([]automation.Task, 0, len(views)+len(tasks))
	for _, view := range views {
		if automation.IsHeartbeatCompatTaskID(view.ID) {
			continue
		}
		next = append(next, view.Task)
	}
	for i, task := range tasks {
		converted, err := heartbeatCompatTaskToAutomation(task)
		if err != nil {
			return fmt.Errorf("heartbeat task %d: %w", i, err)
		}
		next = append(next, converted)
	}
	materialized, err := a.materializeAutomationTasks(next)
	if err != nil {
		return err
	}
	return a.saveAutomationTasks(materialized)
}

func (a *App) heartbeatCompatTriggerNow(id string) {
	svc, _ := a.ensureAutomationService()
	_, _ = svc.TriggerNow(a.bootContext(), automation.TriggerNowRequest{
		TaskID: automation.HeartbeatCompatTaskID(id),
		Reason: "heartbeat compatibility trigger",
	})
}

func heartbeatCompatTaskToAutomation(task HeartbeatTask) (automation.Task, error) {
	schedule, err := automation.ConvertHeartbeatSchedule(task.Interval, task.TimeWindowStart, task.TimeWindowEnd)
	if err != nil {
		return automation.Task{}, err
	}
	out := automation.Task{
		ID:                     automation.HeartbeatCompatTaskID(task.ID),
		Name:                   strings.TrimSpace(task.Title),
		Enabled:                task.Enabled,
		Scope:                  strings.TrimSpace(task.Scope),
		WorkspaceRoot:          strings.TrimSpace(task.WorkspaceRoot),
		TopicID:                strings.TrimSpace(task.TopicID),
		NewConversationEachRun: task.NewConversationEachRun,
		Prompt:                 automation.PromptConfig{Prompt: task.Prompt},
		Schedule:               schedule,
		Execution: automation.ExecutionPolicy{
			ApprovalMode: automation.ApprovalMode(normalizeHeartbeatApprovalMode(task.ApprovalMode)),
		},
		CreatedAt: heartbeatCompatUnixMillisTime(task.CreatedAt),
		UpdatedAt: heartbeatCompatUnixMillisTime(task.CreatedAt),
		LastRunAt: heartbeatCompatUnixMillisTime(task.LastRunAt),
	}
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = out.LastRunAt
	}
	if _, err := automation.NormalizeTask(out); err != nil {
		return automation.Task{}, err
	}
	return out, nil
}

func heartbeatCompatTaskFromAutomation(task automation.Task) HeartbeatTask {
	return HeartbeatTask{
		ID:                     task.ID,
		Title:                  task.Name,
		Prompt:                 task.Prompt.Prompt,
		Interval:               heartbeatCompatIntervalFromSchedule(task.Schedule),
		Enabled:                task.Enabled,
		Scope:                  task.Scope,
		WorkspaceRoot:          task.WorkspaceRoot,
		TopicID:                task.TopicID,
		LastRunAt:              heartbeatCompatUnixMillis(task.LastRunAt),
		NewConversationEachRun: task.NewConversationEachRun || task.Archive.KeepSession || task.Archive.Visible,
		CreatedAt:              heartbeatCompatUnixMillis(task.CreatedAt),
		ApprovalMode:           string(task.Execution.ApprovalMode),
		TimeWindowStart:        task.Schedule.Window.Start,
		TimeWindowEnd:          task.Schedule.Window.End,
	}
}

func heartbeatCompatIntervalFromSchedule(schedule automation.Schedule) string {
	switch schedule.Kind {
	case automation.ScheduleKindOnce:
		if schedule.RunAt.IsZero() {
			return "once@"
		}
		return "once@" + schedule.RunAt.Format(time.RFC3339)
	case automation.ScheduleKindCalendar:
		return heartbeatCompatCalendarInterval(schedule.Calendar)
	default:
		if strings.TrimSpace(schedule.Every) != "" {
			return strings.TrimSpace(schedule.Every)
		}
		return "30m"
	}
}

func heartbeatCompatCalendarInterval(cal automation.CalendarSchedule) string {
	at := strings.TrimSpace(cal.TimeOfDay)
	if at == "" {
		at = "09:00"
	}
	switch strings.ToLower(strings.TrimSpace(cal.Frequency)) {
	case "weekly":
		days := cal.Weekdays
		if len(days) == 0 {
			days = []string{"mon"}
		}
		return "168h|weekly:" + strings.Join(days, ",") + "@" + at
	case "monthly":
		return "720h|monthly:" + heartbeatCompatInts(cal.MonthDays, []int{1}) + "@" + at
	case "yearly":
		return "8760h|yearly:" + heartbeatCompatFirstInt(cal.Months, 1) + "-" + heartbeatCompatFirstInt(cal.MonthDays, 1) + "@" + at
	default:
		return "24h|daily@" + at
	}
}

func heartbeatCompatInts(values []int, fallback []int) string {
	if len(values) == 0 {
		values = fallback
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func heartbeatCompatFirstInt(values []int, fallback int) string {
	if len(values) == 0 {
		return strconv.Itoa(fallback)
	}
	return strconv.Itoa(values[0])
}

func heartbeatCompatUnixMillisTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func heartbeatCompatUnixMillis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}
