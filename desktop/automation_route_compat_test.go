package main

import (
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/automation"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestExactAutomationRoutePickerCompatibilityIsCatalogBound(t *testing.T) {
	candidates := []ModelInfo{
		{Ref: "kwu/official-chat", BillingSource: string(modelbilling.BillingSubscriptionPool)},
		{Ref: "custom/byok-chat", BillingSource: string(modelbilling.BillingNewAPIByok)},
		{Ref: ":literal/byok-chat", BillingSource: string(modelbilling.BillingNewAPIByok)},
	}
	tests := []struct {
		name      string
		model     string
		source    modelbilling.BillingSource
		wantModel string
		wantErr   bool
	}{
		{
			name:  "official picker artifact",
			model: ":kwu/official-chat", source: modelbilling.BillingSubscriptionPool,
			wantModel: "kwu/official-chat",
		},
		{
			name:  "BYOK picker artifact",
			model: ":custom/byok-chat", source: modelbilling.BillingNewAPIByok,
			wantModel: "custom/byok-chat",
		},
		{
			name:  "valid literal leading colon wins exact match",
			model: ":literal/byok-chat", source: modelbilling.BillingNewAPIByok,
			wantModel: ":literal/byok-chat",
		},
		{
			name:  "unavailable repaired model remains unavailable",
			model: ":kwu/retired-chat", source: modelbilling.BillingSubscriptionPool,
			wantErr: true,
		},
		{
			name:  "same model in another source cannot repair",
			model: ":kwu/official-chat", source: modelbilling.BillingNewAPIByok,
			wantErr: true,
		},
		{
			name:  "double colon is not swallowed",
			model: "::kwu/official-chat", source: modelbilling.BillingSubscriptionPool,
			wantErr: true,
		},
		{
			name:  "colon without provider separator is not swallowed",
			model: ":official-chat", source: modelbilling.BillingSubscriptionPool,
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := exactAutomationRouteWithPickerCompatibility(candidates, test.model, test.source)
			if test.wantErr {
				if err == nil {
					t.Fatalf("route = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.ModelRef != test.wantModel || got.BillingSource != test.source {
				t.Fatalf("route = %+v, want %s/%s", got, test.source, test.wantModel)
			}
		})
	}
}

func TestStoredPickerArtifactsResolveThroughCurrentDesktopCatalog(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")

	cfg := config.Default()
	cfg.Desktop.ProviderAccess = []string{"custom"}
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{
		Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
		Model: "byok-chat", APIKeyEnv: "CUSTOM_CHAT_KEY",
	})
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	actor := userctx.Default("")
	scope := cloud.BindingScope{
		ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1,
	}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		authState: cloud.AccountAuthenticated, bindingStatus: cloud.BindingBound,
		authorized: true, authorizedActor: actor, authorizedScope: scope,
		bindingActor: actor, bindingScope: scope, hasBinding: true,
		active: &desktopBillingWorkerProcess{
			scope: modelbilling.TaskScope{
				ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &scope,
			},
			done: make(chan struct{}),
		},
	}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"official-chat"})

	dir := t.TempDir()
	payload := `{"tasks":[` +
		`{"id":"official","name":"Official","prompt":{"model":":kwu/official-chat","billingSource":"subscription_pool","prompt":"run"},"schedule":{"kind":"once"}},` +
		`{"id":"byok","name":"BYOK","prompt":{"model":":custom/byok-chat","billingSource":"newapi_byok","prompt":"run"},"schedule":{"kind":"once"}}` +
		`]}`
	if err := os.WriteFile(filepath.Join(dir, "tasks.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	tasks, err := automation.NewStore(dir).LoadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 ||
		tasks[0].Prompt.Model != ":kwu/official-chat" ||
		tasks[1].Prompt.Model != ":custom/byok-chat" {
		t.Fatalf("stored artifacts were rewritten before catalog validation: %+v", tasks)
	}

	want := map[string]modelbilling.BillingRouteSelection{
		"official": {ModelRef: "kwu/official-chat", BillingSource: modelbilling.BillingSubscriptionPool},
		"byok":     {ModelRef: "custom/byok-chat", BillingSource: modelbilling.BillingNewAPIByok},
	}
	for _, task := range tasks {
		got, err := app.resolveAutomationTaskRoute(task)
		if err != nil {
			t.Fatalf("resolve %s: %v", task.ID, err)
		}
		if got != want[task.ID] {
			t.Fatalf("resolve %s = %+v, want %+v", task.ID, got, want[task.ID])
		}
	}
	app.automationService = &fakeAutomationService{tasks: []automation.TaskView{{Task: tasks[0]}, {Task: tasks[1]}}}
	views, err := app.AutomationReloadTasks()
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range views {
		if view.Prompt.Model != want[view.ID].ModelRef || view.Prompt.BillingSource != string(want[view.ID].BillingSource) {
			t.Fatalf("editor received noncanonical route: %+v", view.Prompt)
		}
	}
}

func TestLegacyAutomationRouteStillRequiresOneCandidate(t *testing.T) {
	multiple := []ModelInfo{
		{Ref: "shared/model", BillingSource: string(modelbilling.BillingSubscriptionPool)},
		{Ref: "shared/model", BillingSource: string(modelbilling.BillingNewAPIByok)},
	}
	if _, err := uniqueLegacyAutomationRoute(multiple, "shared/model"); err == nil {
		t.Fatal("legacy route with multiple billing sources was accepted")
	}
	if _, err := uniqueLegacyAutomationRoute(nil, "missing/model"); err == nil {
		t.Fatal("legacy route with no candidate was accepted")
	}
}

func TestLegacyAutomationRouteRepairsOnePickerColonOnlyWhenUnique(t *testing.T) {
	candidates := []ModelInfo{
		{Ref: "custom/chat", BillingSource: string(modelbilling.BillingNewAPIByok)},
		{Ref: ":literal/chat", BillingSource: string(modelbilling.BillingNewAPIByok)},
	}
	got, err := uniqueLegacyAutomationRoute(candidates, ":custom/chat")
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelRef != "custom/chat" || got.BillingSource != modelbilling.BillingNewAPIByok {
		t.Fatalf("repaired legacy route = %+v", got)
	}
	literal, err := uniqueLegacyAutomationRoute(candidates, ":literal/chat")
	if err != nil {
		t.Fatal(err)
	}
	if literal.ModelRef != ":literal/chat" {
		t.Fatalf("literal leading-colon route was rewritten: %+v", literal)
	}
	ambiguous := []ModelInfo{
		{Ref: "shared/chat", BillingSource: string(modelbilling.BillingSubscriptionPool)},
		{Ref: "shared/chat", BillingSource: string(modelbilling.BillingNewAPIByok)},
	}
	for _, model := range []string{":shared/chat", "::custom/chat", ":chat"} {
		if _, err := uniqueLegacyAutomationRoute(ambiguous, model); err == nil {
			t.Fatalf("unsafe legacy route %q was accepted", model)
		}
	}
}
