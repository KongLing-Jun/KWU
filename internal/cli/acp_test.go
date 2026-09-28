package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/acp"
	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/netclient"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"

	_ "kwu/internal/tool/builtin"
)

const acpTestProviderKind = "acp-test-provider"

func init() {
	provider.Register(acpTestProviderKind, func(cfg provider.Config) (provider.Provider, error) {
		return &acpTestProvider{cfg: cfg}, nil
	})
}

func TestACPBuiltinToolsKeepSessionLevelBuiltins(t *testing.T) {
	dir := t.TempDir()
	tools := toolMap(acpBuiltinTools(&config.Config{}, dir, []string{dir}))
	for _, name := range []string{
		"todo_write",
		"complete_step",
		"bash_output",
		"kill_shell",
		"wait",
		"move_file",
		"notebook_edit",
	} {
		if tools[name] == nil {
			t.Fatalf("ACP workspace tools missing %q; got %v", name, toolNames(tools))
		}
	}
}

func TestACPInitializesWithoutAPIKey(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv("DEEPSEEK_API_KEY", "")
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n")
	_ = w.Close()
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})

	out := captureStdout(t, func() {
		if rc := Run([]string{"--acp"}, "test-version"); rc != 0 {
			t.Fatalf("Run --acp initialize rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, `"protocolVersion":1`) || !strings.Contains(out, `"name":"kwu"`) {
		t.Fatalf("initialize output = %s", out)
	}
}

func TestACPFactoryLoadsSessionCwdProjectConfig(t *testing.T) {
	home := isolateCLIConfigHome(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
default_model = "local"

[[providers]]
name = "local"
kind = "acp-test-provider"
base_url = "http://example.invalid"
model = "fake-model"
api_key_env = "KWU_TEST_KEY"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(project, ".kwu", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "acp-only.md"), []byte("ACP project command"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(home); err != nil {
		t.Fatal(err)
	}

	actor := userctx.ActorContext{
		ActorID:       "cli-acp-actor",
		ProfileID:     "cli-acp-profile",
		AuthMethod:    "acp",
		WorkspaceRoot: project,
	}.Normalize()
	if _, err := config.StoreCredentialLinesWithPaths(config.NewProfilePathResolver(actor), []string{"KWU_TEST_KEY=test-key"}); err != nil {
		t.Fatalf("StoreCredentialLinesWithPaths: %v", err)
	}
	ctrl, err := (&acpFactory{}).NewSession(context.Background(), acp.SessionParams{
		Cwd:   project,
		Sink:  event.Discard,
		Actor: actor,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.ActorContext(); got.ActorID != actor.ActorID ||
		got.ProfileID != actor.ProfileID ||
		got.AuthMethod != actor.AuthMethod ||
		got.WorkspaceRoot != actor.WorkspaceRoot {
		t.Fatalf("ActorContext = %+v, want %+v", got, actor)
	}

	for _, cmd := range ctrl.Commands() {
		if cmd.Name == "acp-only" {
			return
		}
	}
	t.Fatalf("ACP session did not load project command from cwd; commands=%v", ctrl.Commands())
}

func TestACPFactorySessionDirUsesCLIProfile(t *testing.T) {
	isolateCLIConfigHome(t)

	defaultFactory := &acpFactory{actor: defaultCLIActorContext()}
	if got, want := filepath.Clean(defaultFactory.SessionDir()), filepath.Clean(config.SessionDir()); got != want {
		t.Fatalf("default ACP SessionDir = %q, want legacy default %q", got, want)
	}

	t.Setenv(cliProfileEnv, "work")
	actor := defaultCLIActorContext()
	factory := &acpFactory{actor: actor}
	got := filepath.Clean(factory.SessionDir())
	want := filepath.Clean(config.NewProfilePathResolver(actor).SessionDir())
	if got != want {
		t.Fatalf("profile ACP SessionDir = %q, want %q", got, want)
	}
	if got == filepath.Clean(config.SessionDir()) {
		t.Fatalf("profile ACP SessionDir fell back to global default %q", got)
	}
}

func TestACPFactorySessionConfigStateUsesCLIProfile(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "work")
	actor := defaultCLIActorContext()
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`
default_model = "work"

[[providers]]
name = "work"
kind = "acp-test-provider"
base_url = "http://example.invalid"
model = "profile-model"
api_key_env = "KWU_PROFILE_KEY"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.StoreCredentialLinesWithPaths(paths, []string{"KWU_PROFILE_KEY=test-key"}); err != nil {
		t.Fatalf("StoreCredentialLinesWithPaths: %v", err)
	}

	state, err := (&acpFactory{actor: actor}).SessionConfigState(context.Background(), acp.SessionConfigStateParams{})
	if err != nil {
		t.Fatalf("SessionConfigState: %v", err)
	}
	if state.Model != "work/profile-model" {
		t.Fatalf("SessionConfigState model = %q, want work/profile-model", state.Model)
	}
}

func TestACPFactoryNewSessionUsesDefaultCLIProfileActor(t *testing.T) {
	isolateCLIConfigHome(t)
	project := t.TempDir()
	t.Setenv(cliProfileEnv, "work")
	actor := defaultCLIActorContext()
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`
default_model = "work"

[[providers]]
name = "work"
kind = "acp-test-provider"
base_url = "http://example.invalid"
model = "profile-model"
api_key_env = "KWU_PROFILE_KEY"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.StoreCredentialLinesWithPaths(paths, []string{"KWU_PROFILE_KEY=test-key"}); err != nil {
		t.Fatalf("StoreCredentialLinesWithPaths: %v", err)
	}

	ctrl, err := (&acpFactory{actor: actor}).NewSession(context.Background(), acp.SessionParams{
		Cwd:  project,
		Sink: event.Discard,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer ctrl.Close()

	gotActor := ctrl.ActorContext()
	if gotActor.ProfileID != "work" {
		t.Fatalf("ActorContext ProfileID = %q, want work", gotActor.ProfileID)
	}
	if filepath.Clean(gotActor.WorkspaceRoot) != filepath.Clean(project) {
		t.Fatalf("ActorContext WorkspaceRoot = %q, want %q", gotActor.WorkspaceRoot, project)
	}
	if got := filepath.Clean(ctrl.SessionDir()); got != filepath.Clean(paths.SessionDir()) {
		t.Fatalf("SessionDir = %q, want profile session dir %q", got, paths.SessionDir())
	}
	if filepath.Clean(ctrl.SessionDir()) == filepath.Clean(config.SessionDir()) {
		t.Fatalf("NewSession fell back to global default session dir %q", ctrl.SessionDir())
	}
}

func TestACPFactoryClearsEffortOverrideForUnsupportedModel(t *testing.T) {
	isolateCLIConfigHome(t)
	if _, err := config.SetCredential("KWU_TEST_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
default_model = "reasoner/reasoning-model"

[[providers]]
name = "reasoner"
kind = "acp-test-provider"
base_url = "http://example.invalid"
model = "reasoning-model"
api_key_env = "KWU_TEST_KEY"
supported_efforts = ["low", "high"]

[[providers]]
name = "plain"
kind = "acp-test-provider"
base_url = "http://example.invalid"
model = "plain-model"
api_key_env = "KWU_TEST_KEY"
effort = "high"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	high := "high"
	state, err := (&acpFactory{}).SessionConfigState(context.Background(), acp.SessionConfigStateParams{
		Cwd:            project,
		Model:          "reasoner/reasoning-model",
		EffortOverride: &high,
	})
	if err != nil {
		t.Fatalf("reasoning SessionConfigState: %v", err)
	}
	if state.EffortOverride == nil || *state.EffortOverride != "high" {
		t.Fatalf("reasoning effort override = %v, want high", state.EffortOverride)
	}

	state, err = (&acpFactory{}).SessionConfigState(context.Background(), acp.SessionConfigStateParams{
		Cwd:            project,
		Model:          "plain/plain-model",
		EffortOverride: &high,
	})
	if err != nil {
		t.Fatalf("plain SessionConfigState: %v", err)
	}
	if _, ok := findACPConfigOption(state.ConfigOptions, "effort"); ok {
		t.Fatalf("plain model should not advertise effort option: %+v", state.ConfigOptions)
	}
	if state.EffortOverride == nil || *state.EffortOverride != "" {
		t.Fatalf("plain effort override = %v, want explicit empty override", state.EffortOverride)
	}
}

func TestACPTaskProfileDefaults(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.SubagentModel = "default-model"
	cfg.Agent.SubagentEffort = "high"
	cfg.Agent.SubagentModels = map[string]string{"task": "task-model"}
	cfg.Agent.SubagentEfforts = map[string]string{"task": "max"}

	model, effort := acpTaskProfileDefaults(cfg)
	if model != "task-model" || effort != "max" {
		t.Fatalf("task profile defaults = %q/%q, want task-model/max", model, effort)
	}

	cfg.Agent.SubagentModels = nil
	cfg.Agent.SubagentEfforts = nil
	model, effort = acpTaskProfileDefaults(cfg)
	if model != "default-model" || effort != "high" {
		t.Fatalf("fallback task profile defaults = %q/%q, want default-model/high", model, effort)
	}
}

func TestACPSubagentProviderResolverHonorsProfile(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = []config.ProviderEntry{
		{
			Name:             "parent",
			Kind:             acpTestProviderKind,
			Model:            "parent-model",
			ContextWindow:    111,
			SupportedEfforts: []string{"low", "high"},
		},
		{
			Name:             "sub",
			Kind:             acpTestProviderKind,
			Models:           []string{"sub-model"},
			Default:          "sub-model",
			ContextWindow:    222,
			SupportedEfforts: []string{"low", "high"},
		},
	}
	parent, ok := cfg.ResolveModel("parent")
	if !ok {
		t.Fatal("parent model did not resolve")
	}

	resolve := newACPSubagentProviderResolver(cfg, parent, netclient.ProxySpec{})
	prov, _, ctxWin, err := resolve("sub/sub-model", "HIGH")
	if err != nil {
		t.Fatalf("resolve sub profile: %v", err)
	}
	got := prov.(*acpTestProvider).cfg
	if got.Model != "sub-model" || got.Extra["effort"] != "high" || ctxWin != 222 {
		t.Fatalf("resolved profile = model:%q effort:%v ctx:%d, want sub-model/high/222", got.Model, got.Extra["effort"], ctxWin)
	}

	prov, _, ctxWin, err = resolve("", "low")
	if err != nil {
		t.Fatalf("resolve effort-only profile: %v", err)
	}
	got = prov.(*acpTestProvider).cfg
	if got.Model != "parent-model" || got.Extra["effort"] != "low" || ctxWin != 111 {
		t.Fatalf("effort-only profile = model:%q effort:%v ctx:%d, want parent-model/low/111", got.Model, got.Extra["effort"], ctxWin)
	}
}

func TestACPSubagentProviderResolverRejectsInvalidEffort(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = []config.ProviderEntry{{
		Name:             "parent",
		Kind:             acpTestProviderKind,
		Model:            "parent-model",
		SupportedEfforts: []string{"low", "high"},
	}}
	parent, ok := cfg.ResolveModel("parent")
	if !ok {
		t.Fatal("parent model did not resolve")
	}

	resolve := newACPSubagentProviderResolver(cfg, parent, netclient.ProxySpec{})
	if _, _, _, err := resolve("", "max"); err == nil {
		t.Fatal("invalid effort should fail before ACP task falls back to the parent profile")
	}
}

func findACPConfigOption(options []acp.SessionConfigOption, id string) (acp.SessionConfigOption, bool) {
	for _, opt := range options {
		if opt.ID == id {
			return opt, true
		}
	}
	return acp.SessionConfigOption{}, false
}

func toolMap(tools []tool.Tool) map[string]tool.Tool {
	out := make(map[string]tool.Tool, len(tools))
	for _, t := range tools {
		out[t.Name()] = t
	}
	return out
}

func toolNames(tools map[string]tool.Tool) []string {
	out := make([]string, 0, len(tools))
	for name := range tools {
		out = append(out, name)
	}
	return out
}

type acpTestProvider struct {
	cfg provider.Config
}

func (p *acpTestProvider) Name() string { return p.cfg.Name }

func (p *acpTestProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 1)
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}
