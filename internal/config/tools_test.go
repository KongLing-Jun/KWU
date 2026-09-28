package config

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestToolTimeoutSecondsDefaultsToMandatorySafetyCap(t *testing.T) {
	cfg := Default()
	if cfg.Tools.ToolTimeoutSeconds != nil {
		t.Fatalf("default raw tool timeout = %v, want nil", *cfg.Tools.ToolTimeoutSeconds)
	}
	if got := cfg.ToolTimeoutSeconds(); got != 120 {
		t.Fatalf("ToolTimeoutSeconds() = %d, want 120", got)
	}
}

func TestToolTimeoutSecondsCannotBeDisabled(t *testing.T) {
	for _, value := range []int{0, -1} {
		cfg := Default()
		cfg.Tools.ToolTimeoutSeconds = intPtr(value)
		if got := cfg.ToolTimeoutSeconds(); got != 120 {
			t.Fatalf("ToolTimeoutSeconds() with %d = %d, want 120", value, got)
		}
	}
}

func TestToolTimeoutSecondsAcceptsPositiveOverride(t *testing.T) {
	cfg := Default()
	if _, err := toml.Decode("[tools]\ntool_timeout_seconds = 45\n", cfg); err != nil {
		t.Fatalf("decode tool timeout: %v", err)
	}
	if got := cfg.ToolTimeoutSeconds(); got != 45 {
		t.Fatalf("ToolTimeoutSeconds() = %d, want 45", got)
	}
}

func TestBashTimeoutSecondsDefaultsToSafetyCap(t *testing.T) {
	cfg := Default()
	if cfg.Tools.BashTimeoutSeconds != nil {
		t.Fatalf("default raw bash timeout = %v, want nil", *cfg.Tools.BashTimeoutSeconds)
	}
	if got := cfg.BashTimeoutSeconds(); got != 120 {
		t.Fatalf("BashTimeoutSeconds() = %d, want 120", got)
	}
}

func TestBashTimeoutSecondsAllowsExplicitZero(t *testing.T) {
	cfg := Default()
	cfg.Tools.BashTimeoutSeconds = intPtr(0)
	if got := cfg.BashTimeoutSeconds(); got != 0 {
		t.Fatalf("BashTimeoutSeconds() = %d, want 0", got)
	}
}

func TestBashTimeoutSecondsParsesExplicitZero(t *testing.T) {
	cfg := Default()
	if _, err := toml.Decode("[tools]\nbash_timeout_seconds = 0\n", cfg); err != nil {
		t.Fatalf("decode explicit zero: %v", err)
	}
	if cfg.Tools.BashTimeoutSeconds == nil {
		t.Fatal("explicit zero decoded as nil")
	}
	if got := cfg.BashTimeoutSeconds(); got != 0 {
		t.Fatalf("BashTimeoutSeconds() = %d, want 0", got)
	}
}

func TestBashTimeoutSecondsFallsBackForNegative(t *testing.T) {
	cfg := Default()
	cfg.Tools.BashTimeoutSeconds = intPtr(-1)
	if got := cfg.BashTimeoutSeconds(); got != 120 {
		t.Fatalf("BashTimeoutSeconds() = %d, want 120", got)
	}
}

func TestPythonRuntimeConfigNormalizesModeAndInterpreter(t *testing.T) {
	cfg := Default()
	cfg.Tools.Python = PythonRuntimeConfig{Interpreter: "  C:/Python312/python.exe  ", Mode: " MANAGED "}
	if got := cfg.PythonInterpreter(); got != "C:/Python312/python.exe" {
		t.Fatalf("PythonInterpreter() = %q", got)
	}
	if got := cfg.PythonRuntimeMode(); got != "managed" {
		t.Fatalf("PythonRuntimeMode() = %q, want managed", got)
	}
	cfg.Tools.Python.Mode = "unsupported"
	if got := cfg.PythonRuntimeMode(); got != "unsupported" {
		t.Fatalf("PythonRuntimeMode() = %q, want fail-closed value", got)
	}
}

func TestBackgroundJobStalledWarningSecondsDefault(t *testing.T) {
	cfg := Default()
	if cfg.Tools.BackgroundJobs.StalledWarningSeconds != nil {
		t.Fatalf("default raw stalled warning = %v, want nil", *cfg.Tools.BackgroundJobs.StalledWarningSeconds)
	}
	if got := cfg.BackgroundJobStalledWarningSeconds(); got != 900 {
		t.Fatalf("BackgroundJobStalledWarningSeconds() = %d, want 900", got)
	}
}

func TestBackgroundJobStalledWarningSecondsAllowsExplicitZero(t *testing.T) {
	cfg := Default()
	cfg.Tools.BackgroundJobs.StalledWarningSeconds = intPtr(0)
	if got := cfg.BackgroundJobStalledWarningSeconds(); got != 0 {
		t.Fatalf("BackgroundJobStalledWarningSeconds() = %d, want 0", got)
	}
}

func TestBackgroundJobStalledWarningSecondsParsesExplicitZero(t *testing.T) {
	cfg := Default()
	if _, err := toml.Decode("[tools.background_jobs]\nstalled_warning_seconds = 0\n", cfg); err != nil {
		t.Fatalf("decode explicit zero: %v", err)
	}
	if cfg.Tools.BackgroundJobs.StalledWarningSeconds == nil {
		t.Fatal("explicit zero decoded as nil")
	}
	if got := cfg.BackgroundJobStalledWarningSeconds(); got != 0 {
		t.Fatalf("BackgroundJobStalledWarningSeconds() = %d, want 0", got)
	}
}

func TestBackgroundJobStalledWarningSecondsBounds(t *testing.T) {
	cfg := Default()
	cfg.Tools.BackgroundJobs.StalledWarningSeconds = intPtr(-1)
	if got := cfg.BackgroundJobStalledWarningSeconds(); got != 900 {
		t.Fatalf("negative BackgroundJobStalledWarningSeconds() = %d, want 900", got)
	}
	cfg.Tools.BackgroundJobs.StalledWarningSeconds = intPtr(90000)
	if got := cfg.BackgroundJobStalledWarningSeconds(); got != 86400 {
		t.Fatalf("oversized BackgroundJobStalledWarningSeconds() = %d, want 86400", got)
	}
}
