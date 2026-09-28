package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/expert"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestAutomationCapabilitiesSkipUnselectedCloudExperts(t *testing.T) {
	app := newAutomationCapabilityTestApp(t, userctx.Default(""))
	setAnonymousAutomationQuota(t, app)
	calls := 0
	app.cloudRuntime = &cloudRuntime{expertAssets: &expertCatalogTestClient{
		list: func(context.Context) (cloud.SystemExpertAssets, error) {
			calls++
			return cloud.SystemExpertAssets{}, errors.New("expert service offline")
		},
	}}
	t.Cleanup(func() { app.shutdownAutomation(context.Background()) })
	task := automation.Task{
		ID: "offline-catalog", Name: "Local task", Enabled: false,
		Prompt:   automation.PromptConfig{Prompt: "inert test"},
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	if _, err := app.AutomationResolveCapabilities(task); err != nil {
		t.Fatalf("preflight with no expert: %v", err)
	}
	created, err := app.AutomationCreateTask(task)
	if err != nil {
		t.Fatalf("create local task: %v", err)
	}
	created.Name = "Updated local task"
	updated, err := app.AutomationUpdateTask(created, created.Revision)
	if err != nil || updated.Revision <= created.Revision {
		t.Fatalf("update local task: %+v, %v", updated, err)
	}
	_, engine := app.ensureAutomationService()
	// Disabled tasks reach the real capability checker, then stop before the runner.
	runID, err := engine.Trigger(context.Background(), automation.TriggerRequest{TaskID: task.ID})
	if err != nil {
		t.Fatalf("check disabled fixture: %v", err)
	}
	manifest, err := app.automationStore.ReadRunManifest(task.ID, runID)
	if err != nil || manifest.Condition.Reason != "task disabled" || manifest.Capabilities.ModelRef != "custom/chat-model" {
		t.Fatalf("runtime capability check: %+v, %v", manifest, err)
	}
	if calls != 0 {
		t.Fatalf("unselected expert calls = %d, want 0", calls)
	}
	updated.Capabilities.ExpertID = "required-expert"
	if _, err := app.AutomationResolveCapabilities(updated); err == nil {
		t.Fatal("selected expert outage passed preflight")
	}
	if calls != 1 {
		t.Fatalf("selected expert calls = %d, want 1", calls)
	}
	if _, err := app.AutomationUpdateTask(updated, updated.Revision); err != nil {
		t.Fatal(err)
	}
	runID, err = engine.Trigger(context.Background(), automation.TriggerRequest{TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = app.automationStore.ReadRunManifest(task.ID, runID)
	if err != nil || manifest.Condition.Reason != "capability unavailable" || calls != 2 {
		t.Fatalf("runtime must recheck selected expert: reason=%q calls=%d err=%v", manifest.Condition.Reason, calls, err)
	}
	if _, err := app.AutomationListCapabilities(); err == nil || calls != 3 {
		t.Fatalf("full browsing must retain required fetch: calls=%d err=%v", calls, err)
	}
}

func TestAutomationCapabilitiesSelectedReferencesRemainStrict(t *testing.T) {
	app := newAutomationCapabilityTestApp(t, userctx.Default(""))
	for _, test := range []struct {
		name      string
		selection automation.CapabilitySelection
	}{
		{"missing skill", automation.CapabilitySelection{SkillIDs: []string{"missing-skill"}}},
		{"missing connector", automation.CapabilitySelection{ConnectorIDs: []string{"missing-connector"}}},
		{"missing expert", automation.CapabilitySelection{ExpertID: "missing-expert"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := app.AutomationResolveCapabilities(automation.Task{
				Prompt: automation.PromptConfig{Prompt: "inert test"}, Capabilities: test.selection,
			})
			if !errors.Is(err, automation.ErrCapabilityUnavailable) {
				t.Fatalf("selected reference error = %v", err)
			}
		})
	}
}

func TestAutomationCapabilitiesScopedSnapshotMatchesFullCatalog(t *testing.T) {
	app := newAutomationCapabilityTestApp(t, userctx.Default(""))
	app.cloudRuntime = &cloudRuntime{expertAssets: &expertCatalogTestClient{
		list: func(context.Context) (cloud.SystemExpertAssets, error) {
			return testSystemExpertAssets("exp_selected"), nil
		},
	}}
	task := automation.Task{
		Prompt:       automation.PromptConfig{Prompt: "inert test"},
		Capabilities: automation.CapabilitySelection{ExpertID: "exp_selected"},
	}
	resolved, _, err := app.resolveAutomationTask(task)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := app.AutomationListCapabilities()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	want, err := automation.ResolveCapabilitySnapshotWithContext(resolved.Capabilities, catalog,
		automation.CapabilitySnapshotContext{Prompt: task.Prompt.Prompt, Principal: automationActorPrincipal(app)}, at)
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.resolveAutomationCapabilities(task, at)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("scoped snapshot = %+v, want %+v, err=%v", got, want, err)
	}
}

func TestAutomationResolveCapabilitiesUsesRuntimeDefaultModelRoute(t *testing.T) {
	app := newAutomationCapabilityTestApp(t, userctx.Default(""))

	snapshot, err := app.AutomationResolveCapabilities(automation.Task{
		Prompt: automation.PromptConfig{Prompt: "use the configured default"},
	})
	if err != nil {
		t.Fatalf("AutomationResolveCapabilities: %v", err)
	}
	if snapshot.ModelRef != "custom/chat-model" {
		t.Fatalf("ModelRef = %q, want custom/chat-model", snapshot.ModelRef)
	}

	_, err = app.AutomationResolveCapabilities(automation.Task{
		Prompt: automation.PromptConfig{
			Prompt: "unavailable route", Model: "missing/chat-model",
			BillingSource: string(modelbilling.BillingNewAPIByok),
		},
	})
	if !errors.Is(err, errAutomationModelUnavailable) {
		t.Fatalf("AutomationResolveCapabilities route error = %v", err)
	}
}

func TestAutomationResolveCapabilitiesUsesCurrentPrincipalAndPrompt(t *testing.T) {
	actor := userctx.Default("").Normalize()
	app := newAutomationCapabilityTestApp(t, actor)
	before := time.Now().UTC()

	snapshot, err := app.AutomationResolveCapabilities(automation.Task{
		Prompt: automation.PromptConfig{Prompt: "private preflight prompt"},
	})
	if err != nil {
		t.Fatalf("AutomationResolveCapabilities: %v", err)
	}
	after := time.Now().UTC()
	wantPrincipal := actor.ActorID + "/" + actor.ProfileID
	if snapshot.Principal != wantPrincipal {
		t.Fatalf("Principal = %q, want %q", snapshot.Principal, wantPrincipal)
	}
	if snapshot.PromptHash == "" {
		t.Fatal("PromptHash is empty")
	}
	if snapshot.CapturedAt.Before(before) || snapshot.CapturedAt.After(after) {
		t.Fatalf("CapturedAt = %v, want between %v and %v", snapshot.CapturedAt, before, after)
	}
}

func TestAutomationResolveCapabilitiesFailsClosedForUnavailableCapability(t *testing.T) {
	app := newAutomationCapabilityTestApp(t, userctx.Default(""))
	_, err := app.AutomationResolveCapabilities(automation.Task{
		Prompt:       automation.PromptConfig{Prompt: "review this"},
		Capabilities: automation.CapabilitySelection{SkillIDs: []string{"missing-preflight-skill"}},
	})
	if !errors.Is(err, automation.ErrCapabilityUnavailable) {
		t.Fatalf("AutomationResolveCapabilities error = %v, want ErrCapabilityUnavailable", err)
	}
}

func newAutomationCapabilityTestApp(t *testing.T, actor userctx.ActorContext) *App {
	t.Helper()
	isolateDesktopUserDirs(t)
	actor = actor.Normalize()
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`
default_model = "custom/chat-model"

[desktop]
provider_access = ["custom"]
default_billing_source = "newapi_byok"

[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "chat-model"
api_key_env = "AUTOMATION_CAPABILITY_TEST_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SetCredentialWithPaths(paths, "AUTOMATION_CAPABILITY_TEST_KEY", "sk-test"); err != nil {
		t.Fatal(err)
	}
	app := &App{}
	app.setCurrentActorContext(actor)
	t.Cleanup(app.closeExpertStore)
	return app
}

func TestAutomationCapabilityCatalogMapsSafeExistingMetadata(t *testing.T) {
	catalog := automationCapabilityCatalog(
		[]ModelInfo{{Ref: "openai/gpt-5", Provider: "openai", Model: "gpt-5", Current: true}},
		[]SkillView{{Name: "review", Description: "Review changes", Scope: "project", RunAs: "inline", Enabled: true}},
		[]ExpertInfo{{ID: "architect", Name: "Architect", Description: "safe summary", Enabled: true}},
		[]ServerView{{
			Name: "github", Status: "connected", Command: "secret-command", URL: "https://secret.invalid",
			EnvKeys: []string{"GITHUB_TOKEN"}, HeaderKeys: []string{"Authorization"},
			TrustedReadOnlyTools: []string{"search"},
			ToolList: []ToolView{
				{Name: "search", ReadOnlyHint: true},
				{Name: "create_issue"},
			},
		}},
	)

	if len(catalog.Models) != 1 || catalog.Models[0].ID != "openai/gpt-5" || !catalog.Models[0].Available {
		t.Fatalf("models = %+v", catalog.Models)
	}
	if len(catalog.Skills) != 1 || catalog.Skills[0].ID != "review" || !catalog.Skills[0].Available {
		t.Fatalf("skills = %+v", catalog.Skills)
	}
	if len(catalog.Experts) != 1 || catalog.Experts[0].ID != "architect" || !catalog.Experts[0].Available {
		t.Fatalf("experts = %+v", catalog.Experts)
	}
	if len(catalog.Connectors) != 1 {
		t.Fatalf("connectors = %+v", catalog.Connectors)
	}
	connector := catalog.Connectors[0]
	if len(connector.ToolAllowlist) != 2 || connector.ToolAllowlist[0] != "mcp__github__search" || connector.ToolAllowlist[1] != "mcp__github__create_issue" {
		t.Fatalf("connector tools = %v", connector.ToolAllowlist)
	}
	if len(connector.ReadOnlyTools) != 1 || connector.ReadOnlyTools[0] != "mcp__github__search" {
		t.Fatalf("connector read-only tools = %v", connector.ReadOnlyTools)
	}

	body, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	serialized := string(body)
	for _, secretMetadata := range []string{"secret-command", "secret.invalid", "GITHUB_TOKEN", "Authorization", "safe summary"} {
		if strings.Contains(serialized, secretMetadata) {
			t.Fatalf("catalog leaked %q: %s", secretMetadata, serialized)
		}
	}
}

func TestAutomationCapabilityCatalogMarksUnavailableEntries(t *testing.T) {
	catalog := automationCapabilityCatalog(
		nil,
		[]SkillView{{Name: "disabled-skill", Enabled: false}},
		[]ExpertInfo{{ID: "disabled-expert", Name: "Disabled", Enabled: false}},
		[]ServerView{{Name: "offline", Status: "failed", Error: "credential text must not escape"}},
	)

	if catalog.Skills[0].Available || catalog.Skills[0].UnavailableReason != "disabled" {
		t.Fatalf("skill = %+v", catalog.Skills[0])
	}
	if catalog.Experts[0].Available || catalog.Experts[0].UnavailableReason != "disabled" {
		t.Fatalf("expert = %+v", catalog.Experts[0])
	}
	if catalog.Connectors[0].Available || catalog.Connectors[0].UnavailableReason != "failed" {
		t.Fatalf("connector = %+v", catalog.Connectors[0])
	}
	body, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(body), "credential text") {
		t.Fatalf("catalog leaked connector error: %s", body)
	}
}

func TestAutomationUserExpertsAndTeamsJoinCapabilityCatalog(t *testing.T) {
	catalog := automation.CapabilityCatalog{Experts: []automation.CapabilityOption{{ID: "official", Name: "Official", Available: true}}}
	catalog = appendAutomationUserExpertCapabilities(catalog, []expert.UserExpertInfo{
		{ID: "custom-expert", Name: "Custom Expert", Mode: string(expert.ModeExpert), Enabled: true, SystemPrompt: "custom instructions"},
		{ID: "custom-team", Name: "Custom Team", Mode: string(expert.ModeTeam), Enabled: true, Members: []expert.Member{{Name: "Lead"}}},
		{ID: "official", Name: "Duplicate", Enabled: true},
	})
	if len(catalog.Experts) != 3 {
		t.Fatalf("experts = %+v, want official plus custom expert and team", catalog.Experts)
	}
	for _, id := range []string{"custom-expert", "custom-team"} {
		found := false
		for _, option := range catalog.Experts {
			if option.ID == id {
				found = option.Available && option.ContentHash != ""
			}
		}
		if !found {
			t.Fatalf("custom capability %q missing or incomplete: %+v", id, catalog.Experts)
		}
	}
}
