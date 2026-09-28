package boot

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/config"
	"kwu/internal/scene"
)

func TestBuildInjectsSceneFragmentInActivePromptSlot(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeScenePromptTestConfig(t, dir)
	expertStorePath := filepath.Join(dir, "experts.sqlite")

	tests := []struct {
		scene    scene.Scene
		fragment string
	}{
		{scene.Office, scene.OfficePromptFragment},
		{scene.Coding, scene.CodingPromptFragment},
		{scene.Design, scene.DesignPromptFragment},
	}
	allFragments := []string{scene.OfficePromptFragment, scene.CodingPromptFragment, scene.DesignPromptFragment}
	for _, tt := range tests {
		ctrl, err := Build(context.Background(), Options{
			Scene: tt.scene, ExpertID: "testing", ExpertStorePath: expertStorePath,
		})
		if err != nil {
			t.Fatalf("Build(scene=%q): %v", tt.scene, err)
		}
		system := systemMessage(ctrl.History())
		ctrl.Close()

		if count := strings.Count(system, tt.fragment); count != 1 {
			t.Fatalf("scene %q fragment count = %d, want 1\n%s", tt.scene, count, system)
		}
		for _, other := range allFragments {
			want := 0
			if other == tt.fragment {
				want = 1
			}
			if count := strings.Count(system, other); count != want {
				t.Fatalf("scene %q fragment %q count = %d, want %d", tt.scene, other, count, want)
			}
		}
		baseAt := strings.Index(system, "BASE SYSTEM PROMPT")
		expertAt := strings.Index(system, "Expert role: 测试策略专家")
		sceneAt := strings.Index(system, tt.fragment)
		workUpdateAt := strings.Index(system, config.VisibleWorkUpdatePolicy)
		policyAt := strings.Index(system, config.UserDecisionPolicy)
		if !(baseAt >= 0 && baseAt < expertAt && expertAt < sceneAt && sceneAt < workUpdateAt && workUpdateAt < policyAt) {
			t.Fatalf("scene %q prompt order base=%d expert=%d scene=%d work-update=%d policy=%d\n%s", tt.scene, baseAt, expertAt, sceneAt, workUpdateAt, policyAt, system)
		}
		if count := strings.Count(system, config.VisibleWorkUpdatePolicy); count != 1 {
			t.Fatalf("scene %q visible work update policy count = %d, want 1\n%s", tt.scene, count, system)
		}
	}
}

func TestBuildUnsetPreservesPromptWithoutSceneFragment(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeScenePromptTestConfig(t, dir)

	implicit, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build implicit unset: %v", err)
	}
	implicitPrompt := systemMessage(implicit.History())
	implicit.Close()
	explicit, err := Build(context.Background(), Options{Scene: scene.Unset})
	if err != nil {
		t.Fatalf("Build explicit unset: %v", err)
	}
	explicitPrompt := systemMessage(explicit.History())
	explicit.Close()

	if implicitPrompt != explicitPrompt {
		t.Fatalf("explicit unset changed prompt\nimplicit:\n%s\nexplicit:\n%s", implicitPrompt, explicitPrompt)
	}
	for _, fragment := range []string{scene.OfficePromptFragment, scene.CodingPromptFragment, scene.DesignPromptFragment} {
		if strings.Contains(implicitPrompt, fragment) {
			t.Fatalf("unset prompt contains scene fragment %q", fragment)
		}
	}
	if strings.Contains(implicitPrompt, config.VisibleWorkUpdatePolicy) {
		t.Fatalf("unset prompt contains Desktop visible work update policy:\n%s", implicitPrompt)
	}
}

func TestBuildRejectsInvalidScene(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeScenePromptTestConfig(t, dir)
	if _, err := Build(context.Background(), Options{Scene: scene.Scene("legacy")}); err == nil {
		t.Fatal("Build accepted invalid scene")
	}
}

func TestSceneDoesNotChangeProviderVisibleToolSchemas(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	baseline, _, _ := firstAgentModeRequest(t, Options{})
	for _, promptScene := range []scene.Scene{scene.Office, scene.Coding, scene.Design} {
		request, _, _ := firstAgentModeRequest(t, Options{Scene: promptScene})
		if !reflect.DeepEqual(request.Tools, baseline.Tools) {
			t.Fatalf("scene %q changed provider-visible tool schemas\nscene=%v\nbaseline=%v", promptScene, toolSchemaNames(request.Tools), toolSchemaNames(baseline.Tools))
		}
	}
}

func TestExpertPromptRebuildKeepsSceneFragmentUnique(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeScenePromptTestConfig(t, dir)
	ctrl, err := Build(context.Background(), Options{
		Scene:           scene.Design,
		ExpertStorePath: filepath.Join(dir, "experts.sqlite"),
		SessionDir:      filepath.Join(dir, "sessions"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	ctrl.EnsureSessionPath()
	for _, change := range []func() error{
		func() error { _, err := ctrl.SetExpert("testing"); return err },
		func() error { _, err := ctrl.ClearExpert(); return err },
	} {
		if err := change(); err != nil {
			t.Fatal(err)
		}
		prompt := systemMessage(ctrl.History())
		if count := strings.Count(prompt, scene.DesignPromptFragment); count != 1 {
			t.Fatalf("rebuilt Design fragment count = %d, want 1\n%s", count, prompt)
		}
		if strings.Contains(prompt, scene.OfficePromptFragment) || strings.Contains(prompt, scene.CodingPromptFragment) {
			t.Fatalf("rebuilt Design prompt contains another scene fragment\n%s", prompt)
		}
		if count := strings.Count(prompt, config.VisibleWorkUpdatePolicy); count != 1 {
			t.Fatalf("rebuilt visible work update policy count = %d, want 1\n%s", count, prompt)
		}
	}
}

func writeScenePromptTestConfig(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)
}
