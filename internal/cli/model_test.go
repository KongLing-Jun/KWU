package cli

import (
	"os"
	"strings"
	"testing"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

// TestModelRefsFromConfig verifies the /model picker enumerates configured
// provider/model refs (built-in defaults when no kwu.toml is present), and
// only those whose provider API key is set.
func TestModelRefsFromConfig(t *testing.T) {
	isolateUserConfig(t) // no kwu.toml -> built-in default providers
	// Only KWU keyed -> MiMo refs must be filtered out.
	if _, err := config.SetCredential("KWU_API_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	t.Setenv("MIMO_API_KEY", "")
	refs := modelRefs()
	if len(refs) == 0 {
		t.Fatal("expected default provider/model refs, got none")
	}
	for _, r := range refs {
		if !strings.Contains(r, "/") {
			t.Errorf("ref %q should be provider/model", r)
		}
		if strings.HasPrefix(r, "mimo") {
			t.Errorf("ref %q from a provider without an API key should be filtered out", r)
		}
	}
}

// TestModelRefsSkipsUnconfigured verifies that with no provider keys set, the
// picker offers nothing rather than listing models the user can't select.
func TestModelRefsSkipsUnconfigured(t *testing.T) {
	isolateUserConfig(t)
	t.Setenv("KWU_API_KEY", "")
	t.Setenv("KWU_API_KEY", "")
	t.Setenv("MIMO_API_KEY", "")
	if refs := modelRefs(); len(refs) != 0 {
		t.Errorf("no keys set → no refs, got %v", refs)
	}
}

func TestModelRefsUsesKWUProfile(t *testing.T) {
	isolateUserConfig(t)
	t.Setenv(cliProfileEnv, "work")
	paths := config.NewProfilePathResolver(defaultCLIActorContext())
	writeTestConfig(t, paths.UserConfigPath(), `default_model = "work-provider/work-model"

[[providers]]
name = "work-provider"
kind = "openai"
base_url = "http://localhost.invalid"
model = "work-model"
`)

	refs := modelRefs()
	if len(refs) != 1 || refs[0] != "work-provider/work-model" {
		t.Fatalf("modelRefs() = %v, want current profile provider", refs)
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("modelRefs should not touch default profile config, stat err=%v", err)
	}
}

// TestModelArgCompletion verifies "/model " completes to the configured refs
// through the shared completion path.
func TestModelArgCompletion(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("KWU_API_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	m := newTestChatTUI()
	items, _, ok := m.slashArgItems("/model ")
	if !ok || len(items) == 0 {
		t.Fatalf("/model arg completion should offer refs, ok=%v n=%d", ok, len(items))
	}
}

// TestPersistModelWritesDefaultModel verifies that calling persistModel with a
// "provider/model" ref writes default_model = "<ref>" to the user config file
// in TOML form. This is the fix for the "default model resets on every launch"
// regression: previously /model only mutated the in-memory controller and the
// next startup read the global default.
func TestPersistModelWritesDefaultModel(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("KWU_API_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	t.Setenv("MIMO_API_KEY", "")

	m := newTestChatTUI()
	m.persistModel("kwu/gpt-5.4")

	body, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(body), `default_model = "kwu/gpt-5.4"`) {
		t.Fatalf("saved config missing default_model ref:\n%s", body)
	}
}

func TestPersistModelWritesControllerProfileConfig(t *testing.T) {
	isolateUserConfig(t)
	actor := testCLIProfileActor(t, "work")
	paths := config.NewProfilePathResolver(actor)

	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{Actor: actor, WorkspaceRoot: actor.WorkspaceRoot})
	m.persistModel("kwu/gpt-5.4")

	body, err := os.ReadFile(paths.UserConfigPath())
	if err != nil {
		t.Fatalf("read profile config: %v", err)
	}
	if !strings.Contains(string(body), `default_model = "kwu/gpt-5.4"`) {
		t.Fatalf("profile config missing default_model ref:\n%s", body)
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("persistModel should not touch default profile config, stat err=%v", err)
	}
}

// TestPersistModelRejectsUnknownRef verifies that an unresolvable ref is
// silently dropped (logged to slog, not pushed to the TUI notice channel)
// and never lands in the config file. Reason: surface a "persist failed"
// notice on the input box would make /model feel broken to users whose
// stored config doesn't list the exact model ref they picked; the in-
// memory switch still goes through.
func TestPersistModelRejectsUnknownRef(t *testing.T) {
	isolateUserConfig(t)
	if _, err := config.SetCredential("KWU_API_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}

	m := newTestChatTUI()
	m.persistModel("ghost/never-existed")

	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("unknown ref must not create config file, stat err=%v", err)
	}
}

func TestModelSwitchBuilderReceivesCurrentActorContext(t *testing.T) {
	isolateUserConfig(t)

	want := userctx.ActorContext{
		ActorID:       "tui-actor",
		ProfileID:     "tui-profile",
		AuthMethod:    "cli-test",
		ParentActorID: "parent-actor",
	}.Normalize()
	m := newTestChatTUI()
	m.ctrl = control.New(control.Options{Label: "old-model", Actor: want})
	m.modelRef = "old/model"
	high := "high"
	m.effortOverride = &high

	var got userctx.ActorContext
	m.buildController = func(ref string, effortOverride *string, _ []provider.Message, _ string, actor userctx.ActorContext) (*control.Controller, error) {
		if ref != "new/model" {
			t.Fatalf("ref = %q, want new/model", ref)
		}
		if effortOverride != nil {
			t.Fatalf("model switch effort override = %q, want nil", *effortOverride)
		}
		got = actor
		return control.New(control.Options{Label: "new-model"}), nil
	}

	m.runModelSubcommand("/model new/model")
	if m.pendingModelSwitch == nil {
		t.Fatal("model switch should schedule a rebuild")
	}
	msg, ok := m.pendingModelSwitch().(modelSwitchMsg)
	if !ok || msg.err != nil {
		t.Fatalf("pending model switch = %#v, want successful modelSwitchMsg", msg)
	}
	updated, _ := m.Update(msg)
	if next := updated.(chatTUI); next.effortOverride != nil {
		t.Fatalf("model switch retained session effort override %q", *next.effortOverride)
	}

	if got.ActorID != want.ActorID ||
		got.ProfileID != want.ProfileID ||
		got.AuthMethod != want.AuthMethod ||
		got.ParentActorID != want.ParentActorID {
		t.Fatalf("builder actor = %+v, want %+v", got, want)
	}
}

func TestModelSwitchUpdatesCurrentControllerSnapshotTarget(t *testing.T) {
	oldCtrl := control.New(control.Options{Label: "old-model"})
	newCtrl := control.New(control.Options{Label: "new-model"})

	m := newTestChatTUI()
	m.ctrl = oldCtrl
	m.currentCtrl = newCurrentController(oldCtrl)

	updated, _ := m.Update(modelSwitchMsg{
		ref:     "new/model",
		ctrl:    newCtrl,
		oldCtrl: oldCtrl,
		label:   newCtrl.Label(),
	})
	next := updated.(chatTUI)

	if got := next.currentCtrl.Get(); got != newCtrl {
		t.Fatalf("current controller = %v, want new controller", got)
	}
}

func TestRunChatHangupSnapshotsCurrentController(t *testing.T) {
	src, err := os.ReadFile("cli.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	if !strings.Contains(text, "_ = m.currentCtrl.Snapshot()") {
		t.Fatal("hangup handler should snapshot the current controller holder")
	}
	if strings.Contains(text, "_ = ctrl.Snapshot()\n\t\t\tp.Quit()") {
		t.Fatal("hangup handler still snapshots the startup controller")
	}
}
