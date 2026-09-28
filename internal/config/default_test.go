package config

import (
	"testing"

	"kwu/internal/agentmode"
)

func TestDefaultAutoPlanOff(t *testing.T) {
	if got := Default().Agent.AutoPlan; got != "off" {
		t.Fatalf("default auto_plan = %q, want off", got)
	}
}

func TestDefaultAgentModeNormal(t *testing.T) {
	got, err := Default().AgentMode()
	if err != nil {
		t.Fatalf("default agent mode returned error: %v", err)
	}
	if got != agentmode.ModeNormal {
		t.Fatalf("default agent mode = %q, want %q", got, agentmode.ModeNormal)
	}
}

func TestAgentModeFromConfig(t *testing.T) {
	cfg := Default()
	cfg.Agent.Mode = "task_tree"

	got, err := cfg.AgentMode()
	if err != nil {
		t.Fatalf("agent mode returned error: %v", err)
	}
	if got != agentmode.ModeTaskTree {
		t.Fatalf("agent mode = %q, want %q", got, agentmode.ModeTaskTree)
	}
}

func TestAgentModeRejectsUnknown(t *testing.T) {
	cfg := Default()
	cfg.Agent.Mode = "workflow"

	if got, err := cfg.AgentMode(); err == nil {
		t.Fatalf("agent mode = %q, want error", got)
	}
}

func TestDefaultReasoningLanguageAuto(t *testing.T) {
	if got := Default().ReasoningLanguage(); got != "auto" {
		t.Fatalf("default reasoning_language = %q, want auto", got)
	}
}

func TestDefaultDesktopAppearanceLightGraphite(t *testing.T) {
	cfg := Default()
	if got := cfg.DesktopTheme(); got != "light" {
		t.Fatalf("default desktop theme = %q, want light", got)
	}
	if got := cfg.DesktopThemeStyle(); got != "" {
		t.Fatalf("default desktop theme style = %q, want empty so frontend resolves graphite", got)
	}
}

func TestDefaultDesktopReportingOff(t *testing.T) {
	cfg := Default()
	if cfg.DesktopTelemetry() {
		t.Fatal("default desktop telemetry = true, want false")
	}
	if cfg.DesktopMetrics() {
		t.Fatal("default desktop metrics = true, want false")
	}
	enabled := true
	cfg.Desktop.Telemetry = &enabled
	cfg.Desktop.Metrics = &enabled
	if !cfg.DesktopTelemetry() || !cfg.DesktopMetrics() {
		t.Fatal("explicitly enabled desktop reporting should remain enabled")
	}
}

func TestDefaultDesktopUpdateChecksOn(t *testing.T) {
	cfg := Default()
	if !cfg.DesktopCheckUpdates() {
		t.Fatal("default desktop update checks = false, want true")
	}
	disabled := false
	cfg.Desktop.CheckUpdates = &disabled
	if cfg.DesktopCheckUpdates() {
		t.Fatal("desktop update checks explicit false = true, want false")
	}
}
