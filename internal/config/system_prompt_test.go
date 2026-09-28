package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"kwu/internal/brand"
)

func TestDefaultSystemPromptFocusedExecutionContract(t *testing.T) {
	for _, want := range []string{
		brand.DisplayName,
		"## 1. Establish the outcome",
		"## 3. Execute and try to disprove correctness",
		"A successful tool call proves only that the call ran",
		"do not treat the existence of a file, a zero exit code, or the presence of an attribute as proof",
		"Before making another tool call",
		"If none remains, do not call another tool",
		"In plan mode, use only read-only exploration",
	} {
		if !strings.Contains(DefaultSystemPrompt, want) {
			t.Fatalf("DefaultSystemPrompt missing %q:\n%s", want, DefaultSystemPrompt)
		}
	}
	for _, scenePrompt := range []string{"Scene: Office", "Scene: Coding", "Scene: Design"} {
		if strings.Contains(DefaultSystemPrompt, scenePrompt) {
			t.Fatalf("DefaultSystemPrompt contains Scene-specific rule %q", scenePrompt)
		}
	}
	if got := utf8.RuneCountInString(DefaultSystemPrompt); got > 3700 {
		t.Fatalf("DefaultSystemPrompt length = %d, want <= 3700 runes", got)
	}
}

func TestModelIdentityPolicy(t *testing.T) {
	model := "custom\"model\\variant\nnext-line"
	got := ModelIdentityPolicy(model)

	for _, want := range []string{
		strconv.Quote(model),
		"current main session is bound",
		"without a Provider prefix",
		"training knowledge",
		"response style",
		"Provider type",
		"OpenAI/Anthropic-compatible protocols",
		"product assistant identity",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ModelIdentityPolicy() missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, model) {
		t.Fatalf("ModelIdentityPolicy() contains unencoded model ID:\n%s", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("ModelIdentityPolicy() contains a structural newline:\n%s", got)
	}
	if got != ModelIdentityPolicy(model) {
		t.Fatal("ModelIdentityPolicy() is not deterministic")
	}
}

func TestDesktopProductIdentityPolicyUsesOfficialNamesAndEscapesVersion(t *testing.T) {
	version := "release\"\\variant\nnext-line"
	got := DesktopProductIdentityPolicy(version)

	for _, want := range []string{
		strconv.Quote(brand.DisplayName),
		strconv.Quote(brand.EnglishName),
		strconv.Quote(version),
		"Never invent, translate, transliterate, or substitute official names",
		"names, protocols, billing routes, or model IDs",
		"\"设置\" > \"模型\"",
		"I don't know",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("DesktopProductIdentityPolicy() missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, version) {
		t.Fatalf("DesktopProductIdentityPolicy() contains unencoded version:\n%s", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("DesktopProductIdentityPolicy() contains a structural newline:\n%s", got)
	}
	for _, forbidden := range []string{"模型广场", "Model Marketplace"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("DesktopProductIdentityPolicy() contains obsolete UI entry %q:\n%s", forbidden, got)
		}
	}
	if got != DesktopProductIdentityPolicy(version) {
		t.Fatal("DesktopProductIdentityPolicy() is not deterministic")
	}
}

func TestDesktopProductIdentityPolicyStaysWithinPromptBudget(t *testing.T) {
	policy := DesktopProductIdentityPolicy("dev")
	if got := len(policy); got > 480 {
		t.Fatalf("DesktopProductIdentityPolicy(\"dev\") bytes = %d, want <= 480", got)
	}
	if got := len(policy) / 4; got > 120 {
		t.Fatalf("DesktopProductIdentityPolicy(\"dev\") byte estimate = %d, want <= 120", got)
	}
}

func TestResolveSystemPromptForRootRelativePath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prompts", "session.md"), []byte(" project session prompt \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	cfg := Default()
	cfg.Agent.SystemPromptFile = filepath.Join("prompts", "session.md")

	got, err := cfg.ResolveSystemPromptForRoot(root)
	if err != nil {
		t.Fatalf("ResolveSystemPromptForRoot: %v", err)
	}
	if got != "project session prompt" {
		t.Fatalf("system prompt = %q, want %q", got, "project session prompt")
	}
}

func TestResolveSystemPromptForRootAbsolutePath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.md")
	if err := os.WriteFile(path, []byte(" absolute session prompt \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	cfg := Default()
	cfg.Agent.SystemPromptFile = path

	got, err := cfg.ResolveSystemPromptForRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveSystemPromptForRoot: %v", err)
	}
	if got != "absolute session prompt" {
		t.Fatalf("system prompt = %q, want %q", got, "absolute session prompt")
	}
}
