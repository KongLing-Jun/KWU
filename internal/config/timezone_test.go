package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeTimezoneIsUserLevelOnly(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	project := t.TempDir()
	paths := defaultProfilePathResolver(project)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte("[agent]\ntimezone = \"Asia/Shanghai\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte("[agent]\ntimezone = \"America/New_York\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadForRootWithPaths(project, paths)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.Timezone != "Asia/Shanghai" {
		t.Fatalf("timezone = %q, want user-level value", cfg.Agent.Timezone)
	}
}

func TestTimezoneRenderingRespectsScope(t *testing.T) {
	cfg := Default()
	cfg.Agent.Timezone = "Asia/Shanghai"
	if got := RenderTOMLForScope(cfg, RenderScopeUser); !strings.Contains(got, `timezone = "Asia/Shanghai"`) {
		t.Fatalf("user TOML missing timezone:\n%s", got)
	}
	if got := RenderTOMLForScope(cfg, RenderScopeProject); strings.Contains(got, "timezone") {
		t.Fatalf("project TOML exposed user timezone:\n%s", got)
	}
	if got := RenderTOMLProjectDelta(cfg); strings.Contains(got, "timezone") {
		t.Fatalf("project delta exposed user timezone:\n%s", got)
	}
}

func TestTemporalPolicyKeepsDateOnlyQuestionsToolFree(t *testing.T) {
	for _, want := range []string{
		"directly from temporal_context",
		"do not call current_datetime for date-only or weekday-only questions",
		"current time of day, seconds, a Unix timestamp, or an exact instant",
	} {
		if !strings.Contains(TemporalPolicy, want) {
			t.Errorf("TemporalPolicy missing %q: %s", want, TemporalPolicy)
		}
	}
}
