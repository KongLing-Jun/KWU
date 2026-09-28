package config

import (
	"fmt"
	"strings"
)

func renderToolsConfig(b *strings.Builder, c *Config) {
	b.WriteString("[tools]\n")
	if len(c.Tools.Enabled) == 0 {
		b.WriteString("enabled = []   # empty = all built-in tools\n")
	} else {
		b.WriteString("enabled = [")
		for i, t := range c.Tools.Enabled {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(b, "%q", t)
		}
		b.WriteString("]\n")
	}
	fmt.Fprintf(b, "tool_timeout_seconds = %d   # mandatory timeout for every AI tool call; must be > 0\n", c.ToolTimeoutSeconds())
	fmt.Fprintf(b, "bash_timeout_seconds = %d   # foreground safety cap; set 0 for no tool-local cap\n\n", c.BashTimeoutSeconds())
}

func renderToolsDelta(b *strings.Builder, c *Config) {
	if !hasToolsDelta(c) {
		return
	}
	b.WriteString("[tools]\n")
	if len(c.Tools.Enabled) > 0 {
		fmt.Fprintf(b, "enabled = %s\n", renderStringArray(c.Tools.Enabled))
	}
	if c.Tools.ToolTimeoutSeconds != nil && *c.Tools.ToolTimeoutSeconds > 0 {
		fmt.Fprintf(b, "tool_timeout_seconds = %d\n", *c.Tools.ToolTimeoutSeconds)
	}
	if c.Tools.BashTimeoutSeconds != nil && *c.Tools.BashTimeoutSeconds != 0 {
		fmt.Fprintf(b, "bash_timeout_seconds = %d\n", *c.Tools.BashTimeoutSeconds)
	}
	b.WriteString("\n")
}

func hasToolsDelta(c *Config) bool {
	return len(c.Tools.Enabled) > 0 ||
		(c.Tools.ToolTimeoutSeconds != nil && *c.Tools.ToolTimeoutSeconds > 0) ||
		(c.Tools.BashTimeoutSeconds != nil && *c.Tools.BashTimeoutSeconds != 0)
}
