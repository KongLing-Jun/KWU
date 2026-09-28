package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/expert"
	"kwu/internal/plugin"
	"kwu/internal/skill"
	"kwu/internal/userctx"
)

func TestServeAutomationCheckerResolvesRuntimeCapabilities(t *testing.T) {
	isolateCLIConfigHome(t)
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "kwu.toml"), []byte(`
default_model = "serve-test"

[[providers]]
name = "serve-test"
kind = "openai"
base_url = "https://example.invalid"
model = "test-model"
api_key_env = "KWU_SERVE_TEST_KEY_UNSET"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "serve-user", ProfileID: "work"}.Normalize()
	principal := actor.ActorID + "/" + actor.ProfileID
	catalog := serveAutomationRuntimeCatalog(
		[]skill.Skill{{Name: "review", Description: "Review changes", Body: "private skill instructions"}},
		func(name string) bool { return name == "review" },
		[]expert.Info{{ID: "architect", Name: "Architect", Enabled: true, Prompts: []string{"private starter"}}},
		[]plugin.ServerStatus{{Name: "github", ToolList: []plugin.ToolInfo{{Name: "search"}, {Name: "create_issue"}}}},
		nil,
		principal,
	)
	checker := newServeAutomationChecker(actor, func(root string) (automation.CapabilityCatalog, error) {
		if root != workspace {
			t.Fatalf("capability source root = %q, want %q", root, workspace)
		}
		return catalog, nil
	})
	task := automation.Task{
		WorkspaceRoot: workspace,
		Prompt:        automation.PromptConfig{Prompt: "review this change"},
		Capabilities: automation.CapabilitySelection{
			SkillIDs:     []string{"review"},
			ExpertID:     "architect",
			ConnectorIDs: []string{"github"},
		},
	}

	snapshot, err := checker.CapabilityResolver(task, time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CapabilityResolver: %v", err)
	}
	if len(snapshot.Skills) != 1 || snapshot.Skills[0].ID != "review" || snapshot.Skills[0].ContentHash == "" {
		t.Fatalf("skills = %+v", snapshot.Skills)
	}
	if snapshot.Expert == nil || snapshot.Expert.ID != "architect" || snapshot.Expert.ContentHash == "" {
		t.Fatalf("expert = %+v", snapshot.Expert)
	}
	wantTools := []string{"mcp__github__search", "mcp__github__create_issue"}
	if len(snapshot.ToolAllowlist) != len(wantTools) {
		t.Fatalf("tool allowlist = %v, want %v", snapshot.ToolAllowlist, wantTools)
	}
	for index, want := range wantTools {
		if snapshot.ToolAllowlist[index] != want {
			t.Fatalf("tool allowlist = %v, want %v", snapshot.ToolAllowlist, wantTools)
		}
	}
	if len(snapshot.ConnectorGrants) != 1 || snapshot.ConnectorGrants[0].ConnectorID != "github" || snapshot.ConnectorGrants[0].Principal != principal {
		t.Fatalf("connector grants = %+v", snapshot.ConnectorGrants)
	}
}

func TestServeAutomationCheckerCapturesStrictCapabilitySnapshot(t *testing.T) {
	isolateCLIConfigHome(t)
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "kwu.toml"), []byte(`
default_model = "serve-test"

[[providers]]
name = "serve-test"
kind = "openai"
base_url = "https://example.invalid"
model = "test-model"
api_key_env = "KWU_SERVE_TEST_KEY_UNSET"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "serve-user", ProfileID: "work"}.Normalize()
	checker := newServeAutomationChecker(actor, nil)
	if checker.CapabilityResolver == nil {
		t.Fatal("serve automation capability resolver is nil")
	}
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	task := automation.Task{WorkspaceRoot: workspace, Prompt: automation.PromptConfig{Prompt: "sensitive serve prompt"}}

	snapshot, err := checker.CapabilityResolver(task, now)
	if err != nil {
		t.Fatalf("CapabilityResolver: %v", err)
	}
	if snapshot.PromptHash == "" {
		t.Fatal("PromptHash is empty")
	}
	if snapshot.ModelRef != "serve-test" {
		t.Fatalf("ModelRef = %q, want serve-test", snapshot.ModelRef)
	}
	wantPrincipal := actor.ActorID + "/" + actor.ProfileID
	if snapshot.Principal != wantPrincipal {
		t.Fatalf("Principal = %q, want %q", snapshot.Principal, wantPrincipal)
	}
	if !snapshot.CapturedAt.Equal(now) {
		t.Fatalf("CapturedAt = %v, want %v", snapshot.CapturedAt, now)
	}
}

func TestServeAutomationCheckerFailsClosedForUnresolvedArtifacts(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "serve-user", ProfileID: "work"}.Normalize()
	checker := newServeAutomationChecker(actor, nil)
	task := automation.Task{
		Prompt:       automation.PromptConfig{Prompt: "review this"},
		Capabilities: automation.CapabilitySelection{SkillIDs: []string{"missing-skill"}},
	}

	_, err := checker.CapabilityResolver(task, time.Now().UTC())
	if !errors.Is(err, automation.ErrCapabilityUnavailable) {
		t.Fatalf("CapabilityResolver error = %v, want ErrCapabilityUnavailable", err)
	}
}

func TestServeAutomationModelRefPriorityAndFailClosed(t *testing.T) {
	tests := []struct {
		name      string
		selection string
		prompt    string
		fallback  string
		want      string
		wantErr   bool
	}{
		{name: "selection wins", selection: " selected ", prompt: "prompt", fallback: "default", want: "selected"},
		{name: "prompt wins", selection: " ", prompt: " prompt ", fallback: "default", want: "prompt"},
		{name: "default fallback", selection: "", prompt: "\t", fallback: " default ", want: "default"},
		{name: "all empty", selection: " ", prompt: "\t", fallback: "\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serveAutomationModelRef(automation.CapabilitySelection{ModelRef: tt.selection}, tt.prompt, tt.fallback)
			if tt.wantErr {
				if !errors.Is(err, automation.ErrCapabilityUnavailable) {
					t.Fatalf("serveAutomationModelRef error = %v, want ErrCapabilityUnavailable", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("serveAutomationModelRef = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
