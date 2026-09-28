package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/automation"
	"kwu/internal/boot"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/i18n"
	"kwu/internal/modelbilling"
	"kwu/internal/project"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

const (
	automationRunnerTestModelRef      = desktopTestBYOKModelRef
	automationRunnerTestBillingSource = desktopTestBYOKBillingSource
)

func configureAutomationRunnerTestRoute(t *testing.T) {
	t.Helper()
	configureDesktopTestBYOKRoute(t)
}

func automationRunnerTestPrompt(prompt string) automation.PromptConfig {
	return automation.PromptConfig{
		Prompt:        prompt,
		Model:         automationRunnerTestModelRef,
		BillingSource: automationRunnerTestBillingSource,
	}
}

type automationSubscriptionGateway struct {
	requests []modelbilling.GatewayRequest
}

var errAutomationSynchronousGateway = &provider.APIError{
	Provider: "airclaude_subscription",
	Status:   403,
	Code:     "pre_consume_token_quota_failed",
	Message:  "token quota is not enough",
}

type automationSynchronousErrorGateway struct {
	requests int
}

func (*automationSynchronousErrorGateway) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (g *automationSynchronousErrorGateway) Stream(
	context.Context,
	cloud.BindingScope,
	modelbilling.GatewayRequest,
) (<-chan modelbilling.GatewayChunk, error) {
	g.requests++
	return nil, errAutomationSynchronousGateway
}

func (*automationSubscriptionGateway) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (g *automationSubscriptionGateway) Stream(
	_ context.Context,
	_ cloud.BindingScope,
	request modelbilling.GatewayRequest,
) (<-chan modelbilling.GatewayChunk, error) {
	g.requests = append(g.requests, request)
	evidence, err := modelbilling.NormalizeQuotaEvidence(modelbilling.QuotaObservationInput{
		Endpoint: modelbilling.MaasEndpointChatCompletions,
		Headers: map[string][]string{
			"X-Oneapi-Request-Id": {"automation-request"},
		},
		FinalJSON:  json.RawMessage(`{"usage":{"quota":1}}`),
		ObservedAt: desktopFixtureTimestamp(0, 10),
	})
	if err != nil {
		return nil, err
	}
	input, output, total, count := int64(3), int64(2), int64(5), int64(1)
	usage := modelbilling.NormalizedUsage{
		InputTokens: &input, OutputTokens: &output, ProviderTotalTokens: &total,
		RequestCount: &count, BreakdownStatus: modelbilling.UsageExact,
	}
	chunks := make(chan modelbilling.GatewayChunk, 3)
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "automation repaired"}}
	chunks <- modelbilling.GatewayChunk{
		Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
			PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5,
			Presence: provider.UsagePresence{Input: true, Output: true, ProviderTotal: true},
		}},
		Usage: &usage, ProviderRequestID: "automation-request",
	}
	chunks <- modelbilling.GatewayChunk{
		Chunk: provider.Chunk{Type: provider.ChunkDone}, QuotaEvidence: &evidence,
	}
	close(chunks)
	return chunks, nil
}

func TestBindAutomationTaskTopicCreatesGlobalTopicAndPersistsID(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureAutomationRunnerTestRoute(t)
	setAnonymousAutomationQuota(t, app)
	task := automation.Task{
		ID:       "task-global",
		Name:     "Daily AI News",
		Scope:    "global",
		Prompt:   automation.PromptConfig{Prompt: "summarize news"},
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	if err := app.AutomationSaveTasks([]automation.Task{task}); err != nil {
		t.Fatalf("AutomationSaveTasks: %v", err)
	}

	bound, tab, err := app.bindAutomationTaskTopic(task)
	if err != nil {
		t.Fatalf("bindAutomationTaskTopic: %v", err)
	}

	if bound.TopicID == "" {
		t.Fatal("bound topic ID is empty")
	}
	if tab.TopicID != bound.TopicID || tab.Scope != "global" {
		t.Fatalf("tab = %+v, want global topic %q", tab, bound.TopicID)
	}
	if topics := loadProjectsFile().GlobalTopics; len(topics) != 1 || topics[0] != bound.TopicID {
		t.Fatalf("global topics = %#v, want only %q", topics, bound.TopicID)
	}
	stored, err := app.AutomationReloadTasks()
	if err != nil {
		t.Fatalf("AutomationReloadTasks: %v", err)
	}
	if len(stored) != 1 || stored[0].TopicID != bound.TopicID {
		t.Fatalf("stored tasks = %+v, want topic %q persisted", stored, bound.TopicID)
	}
}

func TestPersistAutomationTaskTopicCanonicalizesManagedTargetForReuse(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	configureAutomationRunnerTestRoute(t)
	projectID := "project-managed-topic-reuse"
	managedRoot, err := ensureCloudProjectWorkspace(context.Background(), state.services, projectID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := state.services.Cache.StoreDetail(context.Background(), state.scope, project.Project{
		ProjectID: projectID, OwnerUserID: state.scope.Binding.CloudUserID, Name: "Managed topic reuse",
		Status: project.ProjectActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	app.ensureAutomationService()
	legacy := automation.Task{
		ID: "legacy-managed-topic", Name: "Legacy managed topic", Scope: "project", WorkspaceRoot: managedRoot,
		TopicID:  "legacy-local-topic",
		Prompt:   automation.PromptConfig{Prompt: "run"},
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	if err := app.automationStore.SaveTasks([]automation.Task{legacy}); err != nil {
		t.Fatal(err)
	}
	stored, err := app.automationStore.Task(legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	resolved, _, err := app.resolveAutomationTask(stored)
	if err != nil {
		t.Fatal(err)
	}
	resolved.TopicID = "topic-cloud-reuse"
	if err := app.persistAutomationTaskTopic(resolved); err != nil {
		t.Fatalf("persistAutomationTaskTopic: %v", err)
	}
	persisted, err := app.automationStore.Task(legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Scope != "project" || persisted.ProjectID != projectID || persisted.WorkspaceRoot != "" || persisted.TopicID != resolved.TopicID {
		t.Fatalf("persisted target = scope %q project %q workspace %q topic %q", persisted.Scope, persisted.ProjectID, persisted.WorkspaceRoot, persisted.TopicID)
	}
	next, _, err := app.resolveAutomationTask(persisted)
	if err != nil {
		t.Fatal(err)
	}
	if next.TopicID != resolved.TopicID {
		t.Fatalf("next run topic = %q, want reused %q", next.TopicID, resolved.TopicID)
	}
}

func TestBindAutomationTaskTopicPreservesHiddenProjectTask(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	setAnonymousAutomationQuota(t, app)
	app.ensureAutomationService()
	globalTask := automation.Task{
		ID:       "task-global",
		Name:     "Daily AI News",
		Scope:    "global",
		Prompt:   automation.PromptConfig{Prompt: "summarize news"},
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	hiddenProjectTask := automation.Task{
		ID:            "task-hidden-project",
		Name:          "Hidden project task",
		Scope:         "project",
		WorkspaceRoot: t.TempDir(),
		Prompt:        automation.PromptConfig{Prompt: "summarize project"},
		Schedule:      automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "1h"},
	}
	if err := app.automationStore.SaveTasks([]automation.Task{globalTask, hiddenProjectTask}); err != nil {
		t.Fatalf("SaveTasks fixture: %v", err)
	}
	globalTask, err := app.automationStore.Task(globalTask.ID)
	if err != nil {
		t.Fatalf("Task fixture: %v", err)
	}

	bound, _, err := app.bindAutomationTaskTopic(globalTask)
	if err != nil {
		t.Fatalf("bindAutomationTaskTopic: %v", err)
	}

	storedGlobalTask, err := app.automationStore.Task(globalTask.ID)
	if err != nil {
		t.Fatalf("stored global task: %v", err)
	}
	if storedGlobalTask.TopicID == "" || storedGlobalTask.TopicID != bound.TopicID {
		t.Fatalf("stored global topic = %q, want %q", storedGlobalTask.TopicID, bound.TopicID)
	}
	if _, err := app.automationStore.Task(hiddenProjectTask.ID); err != nil {
		t.Fatalf("hidden project task was not preserved: %v", err)
	}
}

func TestBindAutomationTaskTopicReusesProjectTopic(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()
	app := NewApp()
	configureAutomationRunnerTestRoute(t)
	setAnonymousAutomationQuota(t, app)
	topic, err := app.CreateTopic("project", projectRoot, "Daily AI News")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	task := automation.Task{
		ID:            "task-project",
		Name:          "Daily AI News",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       topic.ID,
		Prompt:        automation.PromptConfig{Prompt: "summarize news"},
		Schedule:      automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	if err := app.AutomationSaveTasks([]automation.Task{task}); err != nil {
		t.Fatalf("AutomationSaveTasks: %v", err)
	}

	bound, tab, err := app.bindAutomationTaskTopic(task)
	if err != nil {
		t.Fatalf("bindAutomationTaskTopic: %v", err)
	}

	if bound.TopicID != topic.ID {
		t.Fatalf("bound topic ID = %q, want reused %q", bound.TopicID, topic.ID)
	}
	if tab.TopicID != topic.ID || tab.Scope != "project" || tab.WorkspaceRoot != projectRoot {
		t.Fatalf("tab = %+v, want project topic %q under %s", tab, topic.ID, projectRoot)
	}
	for _, project := range loadProjectsFile().Projects {
		if project.Root == projectRoot && len(project.Topics) == 1 && project.Topics[0] == topic.ID {
			return
		}
	}
	t.Fatalf("project topic was not indexed once: %+v", loadProjectsFile().Projects)
}

func TestBindAutomationTaskTopicCreatesFreshTopicEachRun(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureAutomationRunnerTestRoute(t)
	setAnonymousAutomationQuota(t, app)
	task := automation.Task{
		ID:                     "task-fresh",
		Name:                   "Fresh task",
		Scope:                  "global",
		TopicID:                "topic-should-not-reuse",
		NewConversationEachRun: true,
		Prompt:                 automation.PromptConfig{Prompt: "fresh"},
		Schedule:               automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	if err := app.AutomationSaveTasks([]automation.Task{task}); err != nil {
		t.Fatalf("AutomationSaveTasks: %v", err)
	}

	first, _, err := app.bindAutomationTaskTopic(task)
	if err != nil {
		t.Fatalf("first bindAutomationTaskTopic: %v", err)
	}
	second, _, err := app.bindAutomationTaskTopic(task)
	if err != nil {
		t.Fatalf("second bindAutomationTaskTopic: %v", err)
	}

	if first.TopicID == "" || second.TopicID == "" || first.TopicID == second.TopicID {
		t.Fatalf("fresh topics = %q and %q, want two distinct topics", first.TopicID, second.TopicID)
	}
	stored, err := app.AutomationReloadTasks()
	if err != nil {
		t.Fatalf("AutomationReloadTasks: %v", err)
	}
	if len(stored) != 1 || stored[0].TopicID != "topic-should-not-reuse" {
		t.Fatalf("stored tasks = %+v, want original topic untouched for fresh mode", stored)
	}
	if topics := loadProjectsFile().GlobalTopics; len(topics) != 2 {
		t.Fatalf("global topics = %#v, want two fresh topics", topics)
	}
}

func TestDesktopAutomationRunnerRunsInBoundTopicSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureAutomationRunnerTestRoute(t)
	app := NewApp()
	topic, err := app.CreateTopic("global", "", "Daily AI News")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	dir := desktopSessionDir(globalTabWorkspaceRoot())
	sessionPath, err := createEmptySessionFile(dir, "")
	if err != nil {
		t.Fatalf("createEmptySessionFile: %v", err)
	}
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	fake := &appendingDesktopRunner{session: sess, started: make(chan string, 1)}
	ctrl := control.New(control.Options{
		Runner:      fake,
		Executor:    exec,
		Sink:        event.Discard,
		SessionDir:  dir,
		SessionPath: sessionPath,
		Label:       "automation-test",
	})
	defer ctrl.Close()
	app.tabRuntimes["auto"] = &WorkspaceTab{
		ID:            "auto",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		TopicID:       topic.ID,
		TopicTitle:    "Daily AI News",
		SessionPath:   sessionPath,
		Ctrl:          ctrl,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
		model:         automationRunnerTestModelRef,
		billingSource: automationRunnerTestBillingSource,
	}
	app.tabOrder = append(app.tabOrder, "auto")
	task := automation.Task{
		ID:       "task-global",
		Name:     "Daily AI News",
		Scope:    "global",
		TopicID:  topic.ID,
		Prompt:   automationRunnerTestPrompt("summarize news"),
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	runDir := t.TempDir()

	result, err := newDesktopAutomationRunner(app).Run(context.Background(), automation.RunRequest{
		Task:     task,
		Manifest: automation.RunManifest{RunID: "run-1", TaskID: task.ID, TaskName: task.Name, Trigger: automation.TriggerManual},
		RunDir:   runDir,
		Sink:     event.Discard,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	input := <-fake.started
	if !strings.Contains(input, "# Automation Task") || !strings.Contains(input, "summarize news") {
		t.Fatalf("automation input = %q, want compiled task prompt", input)
	}
	if filepath.Clean(result.SessionPath) != filepath.Clean(sessionPath) {
		t.Fatalf("result session path = %q, want topic session %q", result.SessionPath, sessionPath)
	}
	if result.Status != automation.RunStatusSucceeded || result.Summary != "ok" {
		t.Fatalf("result = %+v, want succeeded summary ok", result)
	}
	if result.ResultPath == "" {
		t.Fatal("result path is empty")
	}
	if _, err := os.Stat(result.ResultPath); err != nil {
		t.Fatalf("result file stat: %v", err)
	}
}

func TestBuildTabControllerWithExplicitRouteIgnoresSessionModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureAutomationRunnerTestRoute(t)
	root := t.TempDir()
	route := modelbilling.BillingRouteSelection{
		ModelRef:      automationRunnerTestModelRef,
		BillingSource: modelbilling.BillingSource(automationRunnerTestBillingSource),
	}
	dir := desktopSessionDir(root)
	sessionPath, err := createEmptySessionFile(dir, "session-model")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := agent.SetBranchModelPreserveUpdated(sessionPath, "custom/session-model"); err != nil {
		t.Fatalf("set session model: %v", err)
	}
	tab := &WorkspaceTab{
		ID:            "explicit-route-build",
		Scope:         "project",
		WorkspaceRoot: root,
		SessionPath:   sessionPath,
		Ready:         false,
		model:         route.ModelRef,
		billingSource: string(route.BillingSource),
		actor:         userctx.Default(root),
		disabledMCP:   map[string]ServerView{},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = append(app.tabOrder, tab.ID)

	var builtModel, builtSource string
	app.tabControllerBuilder = func(_ context.Context, options boot.Options, actor userctx.ActorContext, workspaceRoot, model, billingSource string) (*control.Controller, error) {
		builtModel = model
		builtSource = billingSource
		return control.New(control.Options{
			SessionDir:    options.SessionDir,
			SessionPath:   sessionPath,
			Label:         model,
			Actor:         actor,
			WorkspaceRoot: workspaceRoot,
		}), nil
	}

	if err := app.setModelRouteForTabWithPersistence(tab.ID, route.ModelRef, route.BillingSource, false); err != nil {
		t.Fatalf("setModelRouteForTabWithPersistence: %v", err)
	}
	if tab.Ctrl == nil {
		t.Fatalf("explicit route build controller = nil, startup error = %q", tab.StartupErr)
	}
	defer tab.Ctrl.Close()
	if builtModel != route.ModelRef || builtSource != string(route.BillingSource) {
		t.Fatalf("builder route = %q/%q, want %q/%q", builtModel, builtSource, route.ModelRef, route.BillingSource)
	}
	if tab.model != route.ModelRef || tab.billingSource != string(route.BillingSource) {
		t.Fatalf("tab route = %q/%q, want %q/%q", tab.model, tab.billingSource, route.ModelRef, route.BillingSource)
	}
	if got, ok := agent.LoadSessionModel(sessionPath); !ok || got != route.ModelRef {
		t.Fatalf("session model = %q/%v, want %q", got, ok, route.ModelRef)
	}
	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err != nil || !ok || meta.BillingSource != string(route.BillingSource) {
		t.Fatalf("session billing source = %q (ok=%v err=%v), want %q", meta.BillingSource, ok, err, route.BillingSource)
	}
}

func TestDesktopAutomationRunnerRepairsLegacyRouteOnFailedTopicController(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	app := NewApp()
	const (
		modelID       = "claude-fable-5"
		canonicalRef  = "kwu/" + modelID
		legacyRef     = ":" + canonicalRef
		automationID  = "legacy-failed-topic"
		automationRun = "legacy-failed-topic-run"
	)

	previousModels := append([]string(nil), desktopAirClaudeSubscriptionModels...)
	desktopAirClaudeSubscriptionModels = []string{modelID}
	t.Cleanup(func() { desktopAirClaudeSubscriptionModels = previousModels })

	actor := userctx.Default("")
	cfg := config.Default()
	applyDesktopAirClaudeSubscriptionCatalogModels(cfg, []string{modelID})
	cfg.DefaultModel = canonicalRef
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingSubscriptionPool)
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app.readyHook = func() {}
	app.setCurrentActorContext(actor)
	_, binding := activateDesktopSubscriptionTestSession(t, app, actor)
	gateway := &automationSubscriptionGateway{}
	app.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
		resolverConfig := config.Default()
		applyDesktopAirClaudeSubscriptionCatalogModels(resolverConfig, []string{modelID})
		runtime.initializer.inner.Routes = desktopBillingRouteResolver{
			cfg: resolverConfig,
			subscriptionModels: map[string]bool{
				modelID: true,
			},
			subscriptionProvider: "automation-subscription-test",
		}
		runtime.dispatcher.Gateway = gateway
	}

	topic, err := app.CreateTopic("global", "", "Legacy route repair")
	if err != nil {
		t.Fatal(err)
	}
	workspaceRoot := globalTabWorkspaceRoot()
	tabActor := actorContextForWorkspace(actor, workspaceRoot)
	// Keep the legacy route in tab/task metadata, not in a Windows filename:
	// the leading colon would create an alternate-data-stream path that the
	// strict session validator must reject.
	sessionPath, err := createEmptySessionFile(desktopSessionDirForActor(tabActor, workspaceRoot), canonicalRef)
	if err != nil {
		t.Fatal(err)
	}
	tab := &WorkspaceTab{
		ID:            "legacy-failed-tab",
		Scope:         "global",
		WorkspaceRoot: workspaceRoot,
		TopicID:       topic.ID,
		TopicTitle:    "Legacy route repair",
		SessionPath:   sessionPath,
		Ready:         true,
		StartupErr:    "previous controller build failed",
		disabledMCP:   map[string]ServerView{},
		model:         legacyRef,
		billingSource: string(modelbilling.BillingSubscriptionPool),
		actor:         tabActor,
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.mu.Lock()
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = append(app.tabOrder, tab.ID)
	app.saveTabsLocked()
	app.mu.Unlock()
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	})

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	task := automation.Task{
		ID: automationID, Name: "Legacy route repair", Enabled: true,
		Scope: "global", TopicID: topic.ID,
		Prompt: automation.PromptConfig{
			Prompt: "repair and run", Model: legacyRef,
			BillingSource: string(modelbilling.BillingSubscriptionPool),
		},
		Schedule:  automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now, UpdatedAt: now,
	}
	store := automation.NewStore(t.TempDir())
	if err := store.SaveTasks([]automation.Task{task}); err != nil {
		t.Fatal(err)
	}
	engine := automation.NewEngineWithChecker(store, "desktop:test", newDesktopAutomationRunner(app), automation.Checker{
		TaskResolver: func(task automation.Task) (automation.Task, error) {
			resolved, _, err := app.resolveAutomationTask(task)
			return resolved, err
		},
		ModelResolver: func(task automation.Task) error {
			_, err := app.resolveAutomationTaskRoute(task)
			return err
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	runID, err := engine.Trigger(ctx, automation.TriggerRequest{TaskID: automationID, RunID: automationRun})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if runID != automationRun {
		t.Fatalf("run id = %q, want %q", runID, automationRun)
	}
	manifest, err := store.ReadRunManifest(automationID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Model != canonicalRef || manifest.BillingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("manifest route = %q/%q, want %s/%s", manifest.BillingSource, manifest.Model, modelbilling.BillingSubscriptionPool, canonicalRef)
	}
	summaries, err := store.ListRuns(automationID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].Status != automation.RunStatusSucceeded ||
		summaries[0].Model != canonicalRef ||
		summaries[0].BillingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("summary = %+v, want succeeded canonical route", summaries)
	}
	if len(gateway.requests) != 1 || gateway.requests[0].ModelID != modelID {
		t.Fatalf("gateway requests = %+v, want one %q execution", gateway.requests, modelID)
	}
	if tab.Ctrl == nil || tab.StartupErr != "" || tab.model != canonicalRef ||
		tab.billingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("repaired tab = ctrl:%T startup:%q route:%q/%q", tab.Ctrl, tab.StartupErr, tab.billingSource, tab.model)
	}
	saved := app.loadTabsFile()
	var persisted *desktopTabEntry
	for i := range saved.Tabs {
		if saved.Tabs[i].ID == tab.ID {
			persisted = &saved.Tabs[i]
			break
		}
	}
	if persisted == nil || persisted.Model != canonicalRef ||
		persisted.BillingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("persisted tab = %+v, want canonical healthy route", persisted)
	}
	if binding.ProfileID != actor.ProfileID {
		t.Fatalf("test binding = %+v, actor profile %q", binding, actor.ProfileID)
	}
}

func TestDesktopAutomationEngineArchivesSynchronousGatewayErrorWithoutCancellation(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	const (
		modelID      = "claude-fable-5"
		modelRef     = "kwu/" + modelID
		automationID = "synchronous-gateway-error"
		runID        = "synchronous-gateway-error-run"
	)

	previousModels := append([]string(nil), desktopAirClaudeSubscriptionModels...)
	desktopAirClaudeSubscriptionModels = []string{modelID}
	t.Cleanup(func() { desktopAirClaudeSubscriptionModels = previousModels })

	app := NewApp()
	actor := userctx.Default("")
	cfg := config.Default()
	applyDesktopAirClaudeSubscriptionCatalogModels(cfg, []string{modelID})
	cfg.DefaultModel = modelRef
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingSubscriptionPool)
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app.readyHook = func() {}
	app.setCurrentActorContext(actor)
	activateDesktopSubscriptionTestSession(t, app, actor)
	gateway := &automationSynchronousErrorGateway{}
	app.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
		resolverConfig := config.Default()
		applyDesktopAirClaudeSubscriptionCatalogModels(resolverConfig, []string{modelID})
		runtime.initializer.inner.Routes = desktopBillingRouteResolver{
			cfg: resolverConfig,
			subscriptionModels: map[string]bool{
				modelID: true,
			},
			subscriptionProvider: "automation-synchronous-error-test",
		}
		runtime.dispatcher.Gateway = gateway
	}

	topic, err := app.CreateTopic("global", "", "Synchronous gateway error")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	task := automation.Task{
		ID: automationID, Name: "Synchronous gateway error", Enabled: true,
		Scope: "global", TopicID: topic.ID,
		Prompt: automation.PromptConfig{
			Prompt: "fail synchronously", Model: modelRef,
			BillingSource: string(modelbilling.BillingSubscriptionPool),
		},
		Schedule:  automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now, UpdatedAt: now,
	}
	store := automation.NewStore(t.TempDir())
	if err := store.SaveTasks([]automation.Task{task}); err != nil {
		t.Fatal(err)
	}
	engine := automation.NewEngineWithChecker(store, "desktop:test", newDesktopAutomationRunner(app), automation.Checker{
		TaskResolver: func(task automation.Task) (automation.Task, error) {
			resolved, _, err := app.resolveAutomationTask(task)
			return resolved, err
		},
		ModelResolver: func(task automation.Task) error {
			_, err := app.resolveAutomationTaskRoute(task)
			return err
		},
	})

	type triggerResult struct {
		runID string
		err   error
	}
	completed := make(chan triggerResult, 1)
	go func() {
		gotRunID, triggerErr := engine.Trigger(context.Background(), automation.TriggerRequest{
			TaskID: automationID, RunID: runID,
		})
		completed <- triggerResult{runID: gotRunID, err: triggerErr}
	}()

	select {
	case result := <-completed:
		if result.err != nil {
			t.Fatalf("Trigger: %v", result.err)
		}
		if result.runID != runID {
			t.Fatalf("run id = %q, want %q", result.runID, runID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("synchronous Gateway.Stream error left automation Trigger running")
	}

	manifest, err := store.ReadRunManifest(automationID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Status != automation.RunStatusFailed || manifest.FinishedAt.IsZero() ||
		manifest.ErrorMessage != i18n.M.ProviderErrInsufficientCredits ||
		!strings.Contains(manifest.Error, errAutomationSynchronousGateway.Error()) ||
		strings.Count(manifest.Error, errAutomationSynchronousGateway.Error()) != 1 {
		t.Fatalf("manifest = %+v, want one raw gateway error plus a friendly user message", manifest)
	}
	if gateway.requests != 1 {
		t.Fatalf("gateway requests = %d, want 1", gateway.requests)
	}

	app.mu.RLock()
	var ctrl control.SessionAPI
	for _, tab := range app.tabRuntimes {
		if tab != nil && tab.TopicID == topic.ID {
			ctrl = tab.Ctrl
			break
		}
	}
	app.mu.RUnlock()
	if ctrl != nil {
		ctrl.Close()
	}
}

func TestAutomationTopicRouteRepairStatePreservesPendingControllerWait(t *testing.T) {
	app := NewApp()
	route := modelbilling.BillingRouteSelection{
		ModelRef: "kwu/claude-fable-5", BillingSource: modelbilling.BillingSubscriptionPool,
	}
	tab := &WorkspaceTab{
		ID: "pending-topic", model: ":kwu/claude-fable-5",
		billingSource: string(modelbilling.BillingSubscriptionPool),
	}
	app.tabRuntimes[tab.ID] = tab

	rebuild, wait := app.automationTopicRouteRepairState(tab.ID, route)
	if rebuild || !wait {
		t.Fatalf("pending mismatched route state = rebuild:%v wait:%v, want false/true", rebuild, wait)
	}

	tab.model = route.ModelRef
	rebuild, wait = app.automationTopicRouteRepairState(tab.ID, route)
	if rebuild || !wait {
		t.Fatalf("pending canonical route state = rebuild:%v wait:%v, want false/true", rebuild, wait)
	}

	tab.model = ":kwu/claude-fable-5"
	tab.Ready = true
	tab.StartupErr = "previous build failed"
	rebuild, wait = app.automationTopicRouteRepairState(tab.ID, route)
	if !rebuild || wait {
		t.Fatalf("failed route state = rebuild:%v wait:%v, want true/false", rebuild, wait)
	}
}

func TestDesktopAutomationRunnerReportsAsyncTopicStartupFailure(t *testing.T) {
	app := NewApp()
	wantErr := errors.New("subscription controller build failed")
	tab := &WorkspaceTab{ID: "startup-failure"}
	app.tabRuntimes[tab.ID] = tab
	go func() {
		time.Sleep(50 * time.Millisecond)
		app.mu.Lock()
		tab.Ready = true
		tab.StartupErr = wantErr.Error()
		app.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctrl, err := app.waitAutomationTopicController(ctx, tab.ID)
	if ctrl != nil {
		t.Fatalf("controller = %T, want nil", ctrl)
	}
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("wait error = %v, want async startup failure %q", err, wantErr)
	}
}

func TestAutomationTopicControllerWaitRepairPolicyPreservesCancelAndPending(t *testing.T) {
	app := NewApp()
	tab := &WorkspaceTab{ID: "wait-repair-policy"}
	app.tabRuntimes[tab.ID] = tab

	if app.automationTopicControllerWaitNeedsRouteRepair(context.Background(), tab.ID, errors.New("startup failed")) {
		t.Fatalf("pure pending build requested route repair")
	}
	if app.automationTopicControllerWaitNeedsRouteRepair(context.Background(), tab.ID, context.Canceled) {
		t.Fatalf("canceled pending build requested route repair")
	}
	if app.automationTopicControllerWaitNeedsRouteRepair(context.Background(), tab.ID, context.DeadlineExceeded) {
		t.Fatalf("deadline pending build requested route repair")
	}

	tab.Ready = true
	if !app.automationTopicControllerWaitNeedsRouteRepair(context.Background(), tab.ID, errors.New("controller failed")) {
		t.Fatalf("ready controller-less tab did not request route repair")
	}
	tab.Ready = false
	tab.StartupErr = "controller failed"
	if !app.automationTopicControllerWaitNeedsRouteRepair(context.Background(), tab.ID, errors.New("controller failed")) {
		t.Fatalf("startup-failed tab did not request route repair")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if app.automationTopicControllerWaitNeedsRouteRepair(canceled, tab.ID, errors.New("controller failed")) {
		t.Fatalf("canceled startup-failed tab requested route repair")
	}
}

func TestVerifyAutomationTopicRouteRequiresExactModelAndSource(t *testing.T) {
	app := NewApp()
	tab := &WorkspaceTab{ID: "verify-route", model: "custom/model-a", billingSource: string(modelbilling.BillingNewAPIByok)}
	app.tabRuntimes[tab.ID] = tab
	if err := app.verifyAutomationTopicRoute(tab.ID, modelbilling.BillingRouteSelection{
		ModelRef:      "custom/model-a",
		BillingSource: modelbilling.BillingNewAPIByok,
	}); err != nil {
		t.Fatalf("matching route rejected: %v", err)
	}
	if err := app.verifyAutomationTopicRoute(tab.ID, modelbilling.BillingRouteSelection{
		ModelRef:      "custom/model-b",
		BillingSource: modelbilling.BillingNewAPIByok,
	}); err == nil {
		t.Fatal("same-source different-model route was accepted")
	}
}

func TestDesktopAutomationRunnerDisplaysOnlyTaskPrompt(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureAutomationRunnerTestRoute(t)
	app := NewApp()
	topic, err := app.CreateTopic("global", "", "Appointment reminder")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	dir := desktopSessionDir(globalTabWorkspaceRoot())
	sessionPath, err := createEmptySessionFile(dir, "")
	if err != nil {
		t.Fatalf("createEmptySessionFile: %v", err)
	}
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	fake := &appendingDesktopRunner{session: sess, started: make(chan string, 1)}
	ctrl := control.New(control.Options{
		Runner:      fake,
		Executor:    exec,
		Sink:        event.Discard,
		SessionDir:  dir,
		SessionPath: sessionPath,
		Label:       "automation-display-test",
	})
	defer ctrl.Close()
	app.bindControllerDisplayRecorder(ctrl)
	app.tabRuntimes["auto-display"] = &WorkspaceTab{
		ID:            "auto-display",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		TopicID:       topic.ID,
		TopicTitle:    "Appointment reminder",
		SessionPath:   sessionPath,
		Ctrl:          ctrl,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
		model:         automationRunnerTestModelRef,
		billingSource: automationRunnerTestBillingSource,
	}
	app.tabOrder = append(app.tabOrder, "auto-display")
	const taskPrompt = "请提醒我确认体检预约，并列出预约前需要准备的事项。"
	task := automation.Task{
		ID:       "task-display",
		Name:     "Appointment reminder",
		Scope:    "global",
		TopicID:  topic.ID,
		Prompt:   automationRunnerTestPrompt(taskPrompt),
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}

	_, err = newDesktopAutomationRunner(app).Run(context.Background(), automation.RunRequest{
		Task:     task,
		Manifest: automation.RunManifest{RunID: "run-display", TaskID: task.ID, TaskName: task.Name, Trigger: automation.TriggerManual},
		RunDir:   t.TempDir(),
		Sink:     event.Discard,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	modelInput := <-fake.started
	if !strings.Contains(modelInput, "# Automation Task") || !strings.Contains(modelInput, "Task id: "+task.ID) {
		t.Fatalf("model input = %q, want full automation context", modelInput)
	}
	history := mustHistoryForTab(t, app, "auto-display")
	var user HistoryMessage
	for _, msg := range history {
		if msg.Role == "user" {
			user = msg
			break
		}
	}
	if user.Content != taskPrompt {
		t.Fatalf("displayed user content = %q, want only task prompt %q", user.Content, taskPrompt)
	}
	if !strings.Contains(user.SubmitText, "# Automation Task") {
		t.Fatalf("submit text = %q, want full automation prompt preserved", user.SubmitText)
	}
}

func TestDesktopAutomationRunnerDisplaysOnlyTaskPromptWhileRunning(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureAutomationRunnerTestRoute(t)
	app := NewApp()
	topic, err := app.CreateTopic("global", "", "Appointment reminder")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	dir := desktopSessionDir(globalTabWorkspaceRoot())
	sessionPath, err := createEmptySessionFile(dir, "")
	if err != nil {
		t.Fatalf("createEmptySessionFile: %v", err)
	}
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	fake := &blockingAppendingDesktopRunner{
		session: sess,
		started: make(chan string, 1),
		release: make(chan struct{}),
	}
	ctrl := control.New(control.Options{
		Runner:      fake,
		Executor:    exec,
		Sink:        event.Discard,
		SessionDir:  dir,
		SessionPath: sessionPath,
		Label:       "automation-display-running-test",
	})
	defer ctrl.Close()
	app.bindControllerDisplayRecorder(ctrl)
	app.tabRuntimes["auto-display-running"] = &WorkspaceTab{
		ID:            "auto-display-running",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		TopicID:       topic.ID,
		TopicTitle:    "Appointment reminder",
		SessionPath:   sessionPath,
		Ctrl:          ctrl,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
		model:         automationRunnerTestModelRef,
		billingSource: automationRunnerTestBillingSource,
	}
	app.tabOrder = append(app.tabOrder, "auto-display-running")
	const taskPrompt = "请提醒我确认体检预约，并列出预约前需要准备的事项。"
	task := automation.Task{
		ID:       "task-display-running",
		Name:     "Appointment reminder",
		Scope:    "global",
		TopicID:  topic.ID,
		Prompt:   automationRunnerTestPrompt(taskPrompt),
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	done := make(chan error, 1)
	go func() {
		_, err := newDesktopAutomationRunner(app).Run(context.Background(), automation.RunRequest{
			Task:     task,
			Manifest: automation.RunManifest{RunID: "run-display-running", TaskID: task.ID, TaskName: task.Name, Trigger: automation.TriggerManual},
			RunDir:   t.TempDir(),
			Sink:     event.Discard,
		})
		done <- err
	}()

	modelInput := <-fake.started
	if !strings.Contains(modelInput, "# Automation Task") || !strings.Contains(modelInput, "Task id: "+task.ID) {
		t.Fatalf("model input = %q, want full automation context", modelInput)
	}
	history := mustHistoryForTab(t, app, "auto-display-running")
	var user HistoryMessage
	for _, msg := range history {
		if msg.Role == "user" {
			user = msg
			break
		}
	}
	if user.Content != taskPrompt {
		t.Fatalf("displayed running user content = %q, want only task prompt %q", user.Content, taskPrompt)
	}
	close(fake.release)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

type blockingAppendingDesktopRunner struct {
	session *agent.Session
	started chan string
	release chan struct{}
}

func (r *blockingAppendingDesktopRunner) Run(ctx context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	r.started <- input
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.release:
		r.session.Add(provider.Message{Role: provider.RoleAssistant, Content: "ok"})
		return nil
	}
}

func TestDesktopAutomationRunnerRunsBusinessDiagnosticsThroughTopicConversation(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureAutomationRunnerTestRoute(t)
	app := NewApp()
	setAnonymousAutomationQuota(t, app)
	topic, err := app.CreateTopic("global", "", "周度经营诊断")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	dir := desktopSessionDir(globalTabWorkspaceRoot())
	sessionPath, err := createEmptySessionFile(dir, "")
	if err != nil {
		t.Fatalf("createEmptySessionFile: %v", err)
	}
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	fake := &appendingDesktopRunner{session: sess, started: make(chan string, 1)}
	ctrl := control.New(control.Options{
		Runner:      fake,
		Executor:    exec,
		Sink:        event.Discard,
		SessionDir:  dir,
		SessionPath: sessionPath,
		Label:       "automation-business-test",
	})
	defer ctrl.Close()
	app.bindControllerDisplayRecorder(ctrl)
	app.tabRuntimes["auto-business"] = &WorkspaceTab{
		ID:            "auto-business",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		TopicID:       topic.ID,
		TopicTitle:    "周度经营诊断",
		SessionPath:   sessionPath,
		Ctrl:          ctrl,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
		model:         automationRunnerTestModelRef,
		billingSource: automationRunnerTestBillingSource,
	}
	app.tabOrder = append(app.tabOrder, "auto-business")

	task := automation.Task{
		ID:      "task-business",
		Name:    "周度经营诊断",
		Scope:   "global",
		TopicID: topic.ID,
		BusinessDiagnostics: &automation.BusinessDiagnosticsConfig{
			TargetStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			TargetEnd:   time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
			Sources: []automation.BusinessDataSource{{
				ID:   "sales",
				Kind: automation.BusinessDataSales,
				Type: automation.BusinessSourceMCP,
				Rows: []map[string]any{{
					"date":     "2026-06-01",
					"order_id": "order-1",
					"amount":   100,
				}},
			}},
		},
		Prompt:   automationRunnerTestPrompt("diagnose business"),
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	if err := app.AutomationSaveTasks([]automation.Task{task}); err != nil {
		t.Fatalf("AutomationSaveTasks: %v", err)
	}
	runDir := t.TempDir()

	result, err := newDesktopAutomationRunner(app).Run(context.Background(), automation.RunRequest{
		Task:     task,
		Manifest: automation.RunManifest{RunID: "run-business", TaskID: task.ID, TaskName: task.Name, Trigger: automation.TriggerManual},
		RunDir:   runDir,
		Sink:     event.Discard,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	modelInput := <-fake.started
	if !strings.Contains(modelInput, "# Automation Task") || !strings.Contains(modelInput, "## Business Diagnostics") || !strings.Contains(modelInput, "inlineRows=1") {
		t.Fatalf("model input = %q, want full automation and business diagnostics context", modelInput)
	}
	if result.SessionPath == "" {
		t.Fatal("result session path is empty")
	}
	if topics := loadProjectsFile().GlobalTopics; len(topics) != 1 {
		t.Fatalf("global topics = %#v, want one automation topic", topics)
	}
	stored, err := app.AutomationReloadTasks()
	if err != nil {
		t.Fatalf("AutomationReloadTasks: %v", err)
	}
	if len(stored) != 1 || stored[0].TopicID == "" {
		t.Fatalf("stored tasks = %+v, want persisted topic id", stored)
	}
}

func TestDesktopAutomationRunnerRestoresTopicApprovalMode(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureAutomationRunnerTestRoute(t)
	app := NewApp()
	sess := agent.NewSession("sys")
	fake := &appendingDesktopRunner{session: sess, started: make(chan string, 1)}
	topicID, _, ctrl := addAutomationRunnerTestTab(t, app, "auto-approval", "Approval task", sess, fake)
	ctrl.SetToolApprovalMode(control.ToolApprovalAsk)
	task := automation.Task{
		ID:        "task-approval",
		Name:      "Approval task",
		Scope:     "global",
		TopicID:   topicID,
		Prompt:    automationRunnerTestPrompt("run with yolo"),
		Schedule:  automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
		Execution: automation.ExecutionPolicy{ApprovalMode: automation.ApprovalModeYolo},
	}

	if _, err := newDesktopAutomationRunner(app).Run(context.Background(), automation.RunRequest{
		Task:     task,
		Manifest: automation.RunManifest{RunID: "run-approval", TaskID: task.ID, TaskName: task.Name, Trigger: automation.TriggerManual},
		RunDir:   t.TempDir(),
		Sink:     event.Discard,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := ctrl.ToolApprovalMode(); got != control.ToolApprovalAsk {
		t.Fatalf("tool approval mode = %q, want restored ask", got)
	}
}

func TestDesktopAutomationRunnerClassifiesCanceledAndDeadlineWithoutOldResult(t *testing.T) {
	tests := []struct {
		name     string
		runErr   error
		want     automation.RunStatus
		wantCode automation.RunErrorCode
	}{
		{name: "canceled", runErr: context.Canceled, want: automation.RunStatusCanceled, wantCode: automation.RunErrorCanceled},
		{name: "deadline", runErr: context.DeadlineExceeded, want: automation.RunStatusBlocked, wantCode: automation.RunErrorApprovalTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			configureAutomationRunnerTestRoute(t)
			app := NewApp()
			sess := agent.NewSession("sys")
			sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "old result"})
			fake := &failingDesktopRunner{session: sess, started: make(chan string, 1), err: tt.runErr}
			topicID, _, _ := addAutomationRunnerTestTab(t, app, "auto-"+tt.name, "Failing task", sess, fake)
			task := automation.Task{
				ID:       "task-" + tt.name,
				Name:     "Failing task",
				Scope:    "global",
				TopicID:  topicID,
				Prompt:   automationRunnerTestPrompt("run"),
				Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
			}
			runDir := t.TempDir()

			result, err := newDesktopAutomationRunner(app).Run(context.Background(), automation.RunRequest{
				Task:     task,
				Manifest: automation.RunManifest{RunID: "run-" + tt.name, TaskID: task.ID, TaskName: task.Name, Trigger: automation.TriggerManual},
				RunDir:   runDir,
				Sink:     event.Discard,
			})
			if !errors.Is(err, tt.runErr) {
				t.Fatalf("Run error = %v, want %v", err, tt.runErr)
			}
			if result.Status != tt.want || result.ErrorCode != tt.wantCode {
				t.Fatalf("result = %+v, want status %q code %q", result, tt.want, tt.wantCode)
			}
			if result.ResultPath != "" {
				t.Fatalf("result path = %q, want empty without current assistant result", result.ResultPath)
			}
			if _, statErr := os.Stat(filepath.Join(runDir, "result.md")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("result.md stat err = %v, want not exist", statErr)
			}
		})
	}
}

type failingDesktopRunner struct {
	session *agent.Session
	started chan string
	err     error
}

func (r *failingDesktopRunner) Run(_ context.Context, input string) error {
	r.started <- input
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	return r.err
}

func addAutomationRunnerTestTab(t *testing.T, app *App, tabID, title string, sess *agent.Session, runner agent.Runner) (string, string, *control.Controller) {
	t.Helper()
	topic, err := app.CreateTopic("global", "", title)
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	dir := desktopSessionDir(globalTabWorkspaceRoot())
	sessionPath, err := createEmptySessionFile(dir, "")
	if err != nil {
		t.Fatalf("createEmptySessionFile: %v", err)
	}
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Runner:      runner,
		Executor:    exec,
		Sink:        event.Discard,
		SessionDir:  dir,
		SessionPath: sessionPath,
		Label:       tabID,
	})
	t.Cleanup(ctrl.Close)
	app.tabRuntimes[tabID] = &WorkspaceTab{
		ID:            tabID,
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		TopicID:       topic.ID,
		TopicTitle:    title,
		SessionPath:   sessionPath,
		Ctrl:          ctrl,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
		model:         automationRunnerTestModelRef,
		billingSource: automationRunnerTestBillingSource,
	}
	app.tabOrder = append(app.tabOrder, tabID)
	return topic.ID, sessionPath, ctrl
}
