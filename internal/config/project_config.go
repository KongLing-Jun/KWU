package config

import "fmt"

func renderMinimalProjectReasoningLanguage(lang string) string {
	return fmt.Sprintf(`# `+kwuDisplayName+` project configuration.
# Project-local overrides are merged over the user config.

[agent]
reasoning_language = %q
`, lang)
}
