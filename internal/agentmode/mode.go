package agentmode

import (
	"fmt"
	"strings"
)

// Mode identifies the high-level agent handling path for a request or session.
type Mode string

const (
	// ModeNormal is the existing default agent behavior.
	ModeNormal Mode = "normal"
	// ModeTaskTree enables the task-tree orchestration path.
	ModeTaskTree Mode = "task_tree"
)

// ModeSource lists every place that may contribute an agent mode for one
// controller/session build. Resolution order is Session, Request, then Config.
type ModeSource struct {
	// Session is persisted mode metadata loaded from an existing session. Resume
	// must prefer this so a config change cannot rewrite old session semantics.
	Session string
	// Request is a one-shot or session-creation override from an entry surface:
	// CLI/TUI flags, Desktop, Serve, ACP, Bot, or a similar frontend.
	Request string
	// Config is the configured default for newly created sessions.
	Config string
}

// ModeSourceKind identifies which source supplied the final mode.
type ModeSourceKind string

const (
	ModeSourceSession ModeSourceKind = "session"
	ModeSourceRequest ModeSourceKind = "request"
	ModeSourceConfig  ModeSourceKind = "config"
	ModeSourceDefault ModeSourceKind = "default"
)

// ResolvedMode carries the canonical mode together with the winning source.
type ResolvedMode struct {
	Mode   Mode
	Source ModeSourceKind
	Raw    string
}

type modeSpec struct {
	mode    Mode
	aliases []string
}

var modeSpecs = []modeSpec{
	{mode: ModeNormal, aliases: []string{"", "default", string(ModeNormal)}},
	{mode: ModeTaskTree, aliases: []string{string(ModeTaskTree)}},
}

// Normalize resolves a raw mode value into a canonical Mode.
//
// Empty and "default" intentionally resolve to ModeNormal so missing config keeps
// the pre-existing default behavior. Unknown values fail closed instead of
// silently falling back to normal mode.
func Normalize(raw string) (Mode, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	for _, spec := range modeSpecs {
		for _, alias := range spec.aliases {
			if value == alias {
				return spec.mode, nil
			}
		}
	}
	return "", fmt.Errorf("unknown agent mode %q (supported: %s)", raw, strings.Join(SupportedStrings(), ", "))
}

// ResolveMode resolves the effective agent mode using session > request >
// config precedence. When no source is explicit, it returns ModeNormal.
func ResolveMode(src ModeSource) (Mode, error) {
	resolved, err := ResolveModeSource(src)
	if err != nil {
		return "", err
	}
	return resolved.Mode, nil
}

// ResolveModeSource resolves the effective agent mode and records which source
// won. Empty strings are treated as absent sources; the explicit value "default"
// is still a real source and normalizes to ModeNormal.
func ResolveModeSource(src ModeSource) (ResolvedMode, error) {
	for _, candidate := range []struct {
		kind ModeSourceKind
		raw  string
	}{
		{kind: ModeSourceSession, raw: src.Session},
		{kind: ModeSourceRequest, raw: src.Request},
		{kind: ModeSourceConfig, raw: src.Config},
	} {
		if strings.TrimSpace(candidate.raw) == "" {
			continue
		}
		mode, err := Normalize(candidate.raw)
		if err != nil {
			return ResolvedMode{}, fmt.Errorf("%s agent mode: %w", candidate.kind, err)
		}
		return ResolvedMode{Mode: mode, Source: candidate.kind, Raw: candidate.raw}, nil
	}
	return ResolvedMode{Mode: ModeNormal, Source: ModeSourceDefault}, nil
}

// Supported returns the canonical mode values supported by this build.
func Supported() []Mode {
	out := make([]Mode, 0, len(modeSpecs))
	for _, spec := range modeSpecs {
		out = append(out, spec.mode)
	}
	return out
}

// SupportedStrings returns the canonical mode values as strings for diagnostics.
func SupportedStrings() []string {
	modes := Supported()
	out := make([]string, 0, len(modes))
	for _, mode := range modes {
		out = append(out, string(mode))
	}
	return out
}

func (m Mode) String() string {
	return string(m)
}
