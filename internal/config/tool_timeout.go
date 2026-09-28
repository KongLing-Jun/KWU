package config

import "strings"

// ToolsConfig selects which built-in tools are enabled. Empty means all of them.
type ToolsConfig struct {
	Enabled            []string             `toml:"enabled"`
	ToolTimeoutSeconds *int                 `toml:"tool_timeout_seconds"`
	BashTimeoutSeconds *int                 `toml:"bash_timeout_seconds"`
	BackgroundJobs     BackgroundJobsConfig `toml:"background_jobs"`
	Search             SearchConfig         `toml:"search"`
	Shell              ShellConfig          `toml:"shell"`
	Python             PythonRuntimeConfig  `toml:"python"`
}

// PythonRuntimeConfig controls interpreter reuse for agent-issued commands.
// It is user/global runtime policy and project TOML cannot override it.
type PythonRuntimeConfig struct {
	Interpreter string `toml:"interpreter"`
	Mode        string `toml:"mode"`
}

func (c *Config) PythonInterpreter() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Tools.Python.Interpreter)
}

// PythonRuntimeMode normalizes the configured mode. Unknown non-empty values are
// preserved so the runtime resolver can fail closed instead of silently using a
// different interpreter policy after a typo.
func (c *Config) PythonRuntimeMode() string {
	if c == nil {
		return "auto"
	}
	mode := strings.ToLower(strings.TrimSpace(c.Tools.Python.Mode))
	if mode == "" {
		return "auto"
	}
	return mode
}

const defaultToolTimeoutSeconds = 120

// ToolTimeoutSeconds returns the mandatory deadline for every AI-issued tool
// call. Invalid non-positive values fall back to the default rather than
// disabling the safety boundary.
func (c *Config) ToolTimeoutSeconds() int {
	if c.Tools.ToolTimeoutSeconds == nil || *c.Tools.ToolTimeoutSeconds <= 0 {
		return defaultToolTimeoutSeconds
	}
	return *c.Tools.ToolTimeoutSeconds
}
