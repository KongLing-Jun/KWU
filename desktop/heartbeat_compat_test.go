package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/config"
)

func TestHeartbeatCompatListTasksUsesAutomationService(t *testing.T) {
	app := NewApp()
	lastRun := time.Date(2026, 6, 27, 9, 30, 0, 0, time.UTC)
	created := time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)
	app.automationService = &fakeAutomationService{
		tasks: []automation.TaskView{
			{Task: automation.Task{
				ID:            "heartbeat-daily",
				Name:          "Daily check",
				Enabled:       true,
				Scope:         "project",
				WorkspaceRoot: "D:/project/KWU",
				Prompt:        automation.PromptConfig{Prompt: "Summarize status."},
				Schedule: automation.Schedule{
					Kind:     automation.ScheduleKindCalendar,
					Timezone: "Local",
					Calendar: automation.CalendarSchedule{Frequency: "daily", TimeOfDay: "09:30"},
					Window:   automation.TimeWindow{Start: "09:00", End: "18:00"},
				},
				Execution: automation.ExecutionPolicy{ApprovalMode: automation.ApprovalModeYolo},
				Archive:   automation.ArchivePolicy{KeepSession: true},
				CreatedAt: created,
				LastRunAt: lastRun,
			}},
			{Task: automation.Task{
				ID:       "task-not-heartbeat",
				Name:     "Native automation task",
				Enabled:  true,
				Prompt:   automation.PromptConfig{Prompt: "Do not show in old heartbeat API."},
				Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
			}},
		},
	}

	tasks := app.HeartbeatListTasks()

	if len(tasks) != 1 {
		t.Fatalf("HeartbeatListTasks len = %d, want 1: %+v", len(tasks), tasks)
	}
	task := tasks[0]
	if task.ID != "heartbeat-daily" || task.Title != "Daily check" || task.Prompt != "Summarize status." {
		t.Fatalf("task fields = %+v, want converted automation heartbeat task", task)
	}
	if task.Interval != "24h|daily@09:30" || task.TimeWindowStart != "09:00" || task.TimeWindowEnd != "18:00" {
		t.Fatalf("schedule fields = %+v, want legacy heartbeat schedule", task)
	}
	if task.ApprovalMode != "yolo" || !task.NewConversationEachRun {
		t.Fatalf("policy fields = %+v, want old heartbeat-compatible policy", task)
	}
	if task.CreatedAt != created.UnixMilli() || task.LastRunAt != lastRun.UnixMilli() {
		t.Fatalf("time fields = %+v, want unix millis", task)
	}
}

func TestHeartbeatCompatSaveTasksWritesAutomationStore(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	setAnonymousAutomationQuota(t, app)
	existing := automation.Task{
		ID:       "task-native",
		Name:     "Native automation task",
		Enabled:  true,
		Prompt:   automation.PromptConfig{Prompt: "Keep me."},
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "1h"},
	}
	if err := app.AutomationSaveTasks([]automation.Task{existing}); err != nil {
		t.Fatalf("AutomationSaveTasks: %v", err)
	}

	err := app.HeartbeatSaveTasks([]HeartbeatTask{{
		ID:                     "daily",
		Title:                  "Daily check",
		Prompt:                 "Summarize status.",
		Interval:               "24h|daily@09:30",
		Enabled:                true,
		Scope:                  "project",
		WorkspaceRoot:          "D:/project/KWU",
		NewConversationEachRun: true,
		CreatedAt:              time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC).UnixMilli(),
		ApprovalMode:           "ask",
		TimeWindowStart:        "09:00",
		TimeWindowEnd:          "18:00",
	}})
	if err != nil {
		t.Fatalf("HeartbeatSaveTasks: %v", err)
	}

	stored, err := automation.NewStore("").LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored tasks len = %d, want 2: %+v", len(stored), stored)
	}
	byID := map[string]automation.Task{}
	for _, task := range stored {
		byID[task.ID] = task
	}
	if _, ok := byID["task-native"]; !ok {
		t.Fatalf("native automation task was not preserved: %+v", stored)
	}
	heartbeatTask := byID["heartbeat-daily"]
	if heartbeatTask.Name != "Daily check" || heartbeatTask.Prompt.Prompt != "Summarize status." {
		t.Fatalf("heartbeat task = %+v, want converted old API task", heartbeatTask)
	}
	normalizedRoot, err := filepath.Abs("D:/project/KWU")
	if err != nil {
		t.Fatal(err)
	}
	assertMaterializedAutomationRoute(t, heartbeatTask, "project", normalizedRoot)
	if heartbeatTask.Execution.ApprovalMode != automation.ApprovalModeAsk || !heartbeatTask.NewConversationEachRun || heartbeatTask.Archive.KeepSession {
		t.Fatalf("heartbeat policy = %+v archive %+v fresh %v, want converted policy", heartbeatTask.Execution, heartbeatTask.Archive, heartbeatTask.NewConversationEachRun)
	}
	if _, err := os.Stat(filepath.Join(config.MemoryUserDir(), "heartbeat-tasks.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy heartbeat file stat error = %v, want not exists", err)
	}
}

func TestHeartbeatCompatBulkSaveRejectsBeforeServiceCall(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "custom/missing-model"

[desktop]
provider_access = ["custom"]
default_billing_source = "newapi_byok"

[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "chat-model"
api_key_env = "DESKTOP_TEST_BYOK_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &fakeAutomationService{}
	app.automationService = fake

	err := app.HeartbeatSaveTasks([]HeartbeatTask{{
		ID:        "daily",
		Title:     "Daily check",
		Prompt:    "Summarize status.",
		Interval:  "30m",
		Enabled:   true,
		CreatedAt: time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC).UnixMilli(),
	}})
	if err == nil {
		t.Fatal("HeartbeatSaveTasks accepted an unavailable default route")
	}
	if fake.saveCalls != 0 || fake.savedTasks != nil {
		t.Fatalf("heartbeat service state = calls %d tasks %+v, want no save call", fake.saveCalls, fake.savedTasks)
	}
}

func TestHeartbeatCompatPreservesConversationFields(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	setAnonymousAutomationQuota(t, app)

	err := app.HeartbeatSaveTasks([]HeartbeatTask{{
		ID:                     "daily",
		Title:                  "Daily check",
		Prompt:                 "Summarize status.",
		Interval:               "30m",
		Enabled:                true,
		Scope:                  "project",
		WorkspaceRoot:          "D:/project/KWU",
		TopicID:                "topic-existing",
		NewConversationEachRun: false,
		CreatedAt:              time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC).UnixMilli(),
		ApprovalMode:           "auto",
	}})
	if err != nil {
		t.Fatalf("HeartbeatSaveTasks: %v", err)
	}

	stored, err := automation.NewStore("").LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored tasks len = %d, want 1: %+v", len(stored), stored)
	}
	if stored[0].TopicID != "topic-existing" || stored[0].NewConversationEachRun {
		t.Fatalf("stored conversation fields = topic %q fresh %v, want topic-existing false", stored[0].TopicID, stored[0].NewConversationEachRun)
	}

	reloaded := app.HeartbeatReloadTasks()
	if len(reloaded) != 1 {
		t.Fatalf("reloaded tasks len = %d, want 1: %+v", len(reloaded), reloaded)
	}
	if reloaded[0].TopicID != "topic-existing" || reloaded[0].NewConversationEachRun {
		t.Fatalf("reloaded conversation fields = topic %q fresh %v, want topic-existing false", reloaded[0].TopicID, reloaded[0].NewConversationEachRun)
	}
}

func TestHeartbeatCompatTriggerNowUsesAutomationService(t *testing.T) {
	app := NewApp()
	app.activeTabID = "active"
	fake := &fakeAutomationService{
		trigger: automation.TriggerNowResponse{TaskID: "heartbeat-daily", RunID: "run-1", Status: automation.RunStatusQueued},
	}
	app.automationService = fake

	app.HeartbeatTriggerNow("daily")

	if fake.triggerTaskID != "heartbeat-daily" {
		t.Fatalf("trigger task ID = %q, want heartbeat-daily", fake.triggerTaskID)
	}
	if app.cachedActiveTabID() != "active" {
		t.Fatalf("activeTabID = %q, want unchanged", app.activeTabID)
	}
}

func TestHeartbeatCompatGenerateIDUsesAutomationNamespace(t *testing.T) {
	app := NewApp()

	id := app.HeartbeatGenerateID()

	if !strings.HasPrefix(id, "heartbeat-task-") {
		t.Fatalf("HeartbeatGenerateID = %q, want heartbeat task id", id)
	}
}

func TestHeartbeatCompatLegacyEngineStartDoesNotRunScheduler(t *testing.T) {
	engine := newHeartbeatEngine(NewApp())

	engine.Start()
	t.Cleanup(engine.Stop)

	if engine.running {
		t.Fatal("legacy heartbeat engine should not start its scheduler")
	}
}

func TestHeartbeatCompatStartAutomationDoesNotCreateLegacyEngine(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()

	app.startAutomation(context.Background())
	t.Cleanup(func() { app.shutdownAutomation(context.Background()) })

	if app.heartbeat != nil {
		t.Fatalf("legacy heartbeat engine = %#v, want nil", app.heartbeat)
	}
}
