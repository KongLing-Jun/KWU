package agentmode

import (
	"strings"
	"testing"
)

func TestSupportedModes(t *testing.T) {
	got := Supported()
	want := []Mode{ModeNormal, ModeTaskTree}
	if len(got) != len(want) {
		t.Fatalf("Supported() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Supported()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	got[0] = "mutated"
	if again := Supported(); again[0] != ModeNormal {
		t.Fatalf("Supported() returned mutable package storage: %v", again)
	}
}

func TestNormalizeDefaultAndKnownModes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Mode
	}{
		{name: "empty means normal", raw: "", want: ModeNormal},
		{name: "whitespace means normal", raw: " \t\n ", want: ModeNormal},
		{name: "default means normal", raw: "default", want: ModeNormal},
		{name: "normal", raw: "normal", want: ModeNormal},
		{name: "normal trims and folds case", raw: " Normal ", want: ModeNormal},
		{name: "task tree", raw: "task_tree", want: ModeTaskTree},
		{name: "task tree trims and folds case", raw: " TASK_TREE ", want: ModeTaskTree},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.raw)
			if err != nil {
				t.Fatalf("Normalize(%q) returned error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeRejectsUnknownMode(t *testing.T) {
	got, err := Normalize("workflow")
	if err == nil {
		t.Fatalf("Normalize(workflow) = %q, want error", got)
	}
	if got != "" {
		t.Fatalf("Normalize(workflow) mode = %q, want empty on error", got)
	}
	for _, want := range []string{"unknown agent mode", "workflow", "normal", "task_tree"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Normalize(workflow) error = %q, want it to mention %q", err.Error(), want)
		}
	}
}

func TestResolveModeSourcePriority(t *testing.T) {
	tests := []struct {
		name       string
		src        ModeSource
		wantMode   Mode
		wantSource ModeSourceKind
		wantRaw    string
	}{
		{
			name:       "no source defaults to normal",
			src:        ModeSource{},
			wantMode:   ModeNormal,
			wantSource: ModeSourceDefault,
		},
		{
			name:       "config supplies new session default",
			src:        ModeSource{Config: "task_tree"},
			wantMode:   ModeTaskTree,
			wantSource: ModeSourceConfig,
			wantRaw:    "task_tree",
		},
		{
			name:       "request overrides config",
			src:        ModeSource{Request: "normal", Config: "task_tree"},
			wantMode:   ModeNormal,
			wantSource: ModeSourceRequest,
			wantRaw:    "normal",
		},
		{
			name:       "session overrides request and config",
			src:        ModeSource{Session: "task_tree", Request: "normal", Config: "normal"},
			wantMode:   ModeTaskTree,
			wantSource: ModeSourceSession,
			wantRaw:    "task_tree",
		},
		{
			name:       "explicit default keeps winning source",
			src:        ModeSource{Request: "default", Config: "task_tree"},
			wantMode:   ModeNormal,
			wantSource: ModeSourceRequest,
			wantRaw:    "default",
		},
		{
			name:       "blank session is absent",
			src:        ModeSource{Session: " \t", Request: "task_tree", Config: "normal"},
			wantMode:   ModeTaskTree,
			wantSource: ModeSourceRequest,
			wantRaw:    "task_tree",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveModeSource(tt.src)
			if err != nil {
				t.Fatalf("ResolveModeSource(%+v) returned error: %v", tt.src, err)
			}
			if got.Mode != tt.wantMode || got.Source != tt.wantSource || got.Raw != tt.wantRaw {
				t.Fatalf("ResolveModeSource(%+v) = %+v, want mode=%q source=%q raw=%q", tt.src, got, tt.wantMode, tt.wantSource, tt.wantRaw)
			}
		})
	}
}

func TestResolveModeSourceRejectsFirstExplicitUnknown(t *testing.T) {
	got, err := ResolveModeSource(ModeSource{Request: "workflow", Config: "task_tree"})
	if err == nil {
		t.Fatalf("ResolveModeSource returned %+v, want error", got)
	}
	for _, want := range []string{"request agent mode", "workflow", "normal", "task_tree"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ResolveModeSource error = %q, want it to mention %q", err.Error(), want)
		}
	}
}

func TestResolveModeConvenience(t *testing.T) {
	got, err := ResolveMode(ModeSource{Config: "task_tree"})
	if err != nil {
		t.Fatalf("ResolveMode returned error: %v", err)
	}
	if got != ModeTaskTree {
		t.Fatalf("ResolveMode = %q, want %q", got, ModeTaskTree)
	}
}

func TestModeString(t *testing.T) {
	if got := ModeTaskTree.String(); got != "task_tree" {
		t.Fatalf("ModeTaskTree.String() = %q, want task_tree", got)
	}
}
