package scene

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseStrictSelectableValues(t *testing.T) {
	t.Parallel()
	for _, want := range []Scene{Office, Coding, Design} {
		got, err := Parse(string(want))
		if err != nil || got != want {
			t.Fatalf("Parse(%q) = %q, %v; want %q, nil", want, got, err, want)
		}
	}
	for _, value := range []string{"", "code", "unset", "legacy", "Office", "CODING", "design ", " office", "unknown"} {
		if got, err := Parse(value); err == nil || got != Unset {
			t.Fatalf("Parse(%q) = %q, %v; want unset plus error", value, got, err)
		}
	}
}

func TestFrozenPromptFragments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		scene    Scene
		want     string
		keywords []string
	}{
		{Office, "Scene: Office. Produce concise, directly usable documents, spreadsheets, communications, and organized information with minimal disruption. Preserve source facts, reconcile conflicts, distinguish facts from assumptions, verify calculations and labels, and confirm the requested file contents and paths before delivery.", []string{"directly usable", "source facts", "reconcile conflicts", "verify calculations"}},
		{Coding, "Scene: Coding. Understand the relevant code, contracts, and behavior that must be preserved before editing. Make the smallest coherent change. Validate the requested behavior, not merely command success, with focused tests that cover the critical normal path, boundaries, and failure cases; read the actual results and inspect regression risk before delivery.", []string{"behavior that must be preserved", "requested behavior", "boundaries", "actual results", "regression risk"}},
		{Design, "Scene: Design. Clarify the audience and goal, then produce a concrete, locally inspectable design artifact using capabilities actually available. Verify file validity, render-relevant structure, information hierarchy, responsive behavior, and requested accessibility states such as visible keyboard focus; the presence of an attribute alone is not proof that a state is visible. Use a real preview or render when available, otherwise state the validation limit. Never claim access to canvases, layers, cloud sync, exports, or generated files unless that action actually occurred.", []string{"locally inspectable", "render-relevant structure", "visible keyboard focus", "attribute alone", "validation limit"}},
	}

	seen := make(map[string]Scene, len(tests))
	for _, tt := range tests {
		got, err := Fragment(tt.scene)
		if err != nil {
			t.Fatalf("Fragment(%q): %v", tt.scene, err)
		}
		if got != tt.want {
			t.Fatalf("Fragment(%q) changed\n got: %q\nwant: %q", tt.scene, got, tt.want)
		}
		if utf8.RuneCountInString(got) > 640 {
			t.Fatalf("Fragment(%q) length = %d, want <= 640", tt.scene, utf8.RuneCountInString(got))
		}
		for _, keyword := range tt.keywords {
			if !strings.Contains(got, keyword) {
				t.Fatalf("Fragment(%q) missing keyword %q", tt.scene, keyword)
			}
		}
		if prior, ok := seen[got]; ok {
			t.Fatalf("Fragment(%q) duplicates Fragment(%q)", tt.scene, prior)
		}
		seen[got] = tt.scene
	}

	if got, err := Fragment(Unset); err != nil || got != "" {
		t.Fatalf("Fragment(Unset) = %q, %v; want empty, nil", got, err)
	}
	if _, err := Fragment(Scene("legacy")); err == nil {
		t.Fatal("Fragment(legacy) succeeded; want strict rejection")
	}
}

func TestRestoreStatus(t *testing.T) {
	t.Parallel()
	for _, want := range []Status{
		{Scene: Office, State: Draft},
		{Scene: Coding, State: Locked},
		{Scene: Design, State: Draft},
	} {
		got, err := RestoreStatus(string(want.Scene), string(want.State))
		if err != nil || got != want {
			t.Fatalf("RestoreStatus(%q, %q) = %+v, %v; want %+v", want.Scene, want.State, got, err, want)
		}
		if sceneValue, stateValue, ok := got.Persisted(); !ok || sceneValue != string(want.Scene) || stateValue != string(want.State) {
			t.Fatalf("Persisted(%+v) = %q, %q, %v", got, sceneValue, stateValue, ok)
		}
	}

	legacy, err := RestoreStatus("", "")
	if err != nil || !legacy.IsLegacy() {
		t.Fatalf("missing metadata = %+v, %v; want legacy", legacy, err)
	}
	for _, values := range [][2]string{{"office", ""}, {"", "draft"}, {"code", "draft"}, {"office", "legacy"}, {"Office", "locked"}, {"design", "LOCKED"}} {
		got, err := RestoreStatus(values[0], values[1])
		if err == nil || !got.IsLegacy() {
			t.Fatalf("RestoreStatus(%q, %q) = %+v, %v; want diagnostic legacy", values[0], values[1], got, err)
		}
	}
	if _, _, ok := legacy.Persisted(); ok {
		t.Fatal("legacy status must not be persisted as a public scene")
	}
}
