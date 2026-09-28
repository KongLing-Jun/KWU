package config

import (
	"fmt"
	"strings"
)

// DesktopConfig controls desktop-only UI preferences, separately from top-level
// language and [ui], so they cannot affect CLI or provider-visible data.
type DesktopConfig struct {
	Language                string   `toml:"language"`                   // auto|en|zh; empty/auto = browser/OS auto-detect
	LayoutStyle             string   `toml:"layout_style"`               // classic|workbench|creation; desktop layout style
	Theme                   string   `toml:"theme"`                      // auto|dark|light; empty resolves to auto
	ThemeStyle              string   `toml:"theme_style"`                // graphite|aurora|slate|carbon|nocturne|amber and legacy aliases
	CloseBehavior           string   `toml:"close_behavior"`             // quit|background; desktop window close behavior
	DisplayMode             string   `toml:"display_mode"`               // standard|compact (legacy "minimal" maps to compact); transcript display mode
	StatusBarStyle          string   `toml:"status_bar_style"`           // icon|text; desktop status bar metric labels
	StatusBarItems          []string `toml:"status_bar_items"`           // ordered visible desktop status bar items
	DefaultToolApprovalMode string   `toml:"default_tool_approval_mode"` // ask|auto|yolo; default for newly-created desktop sessions
	CheckUpdates            *bool    `toml:"check_updates"`              // startup update checks; nil keeps the default enabled
	Telemetry               *bool    `toml:"telemetry"`                  // anonymous launch ping (install id + version + OS); nil keeps the default disabled
	Metrics                 *bool    `toml:"metrics"`                    // aggregate desktop metrics (anonymous signal/bucket counts; no content); nil keeps the default disabled
	ProviderAccess          []string `toml:"provider_access"`            // desktop-only list of provider entries shown in Settings > Model > Access
	DefaultBillingSource    string   `toml:"default_billing_source"`     // subscription_pool|newapi_byok; paired with top-level default_model
	ExpandThinking          bool     `toml:"expand_thinking"`            // true = show reasoning text expanded by default; false = collapsed
}

func renderDesktopModelPreferences(b *strings.Builder, c DesktopConfig) {
	if source := strings.TrimSpace(c.DefaultBillingSource); source != "" {
		fmt.Fprintf(b, "default_billing_source = %q   # desktop: route paired with default_model\n", source)
	}
	fmt.Fprintf(b, "expand_thinking = %v   # desktop: show reasoning text expanded by default; false = collapsed\n", c.ExpandThinking)
}
