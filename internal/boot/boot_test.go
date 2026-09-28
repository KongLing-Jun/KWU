package boot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/agent/testutil"
	"kwu/internal/agentmode"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/memory"
	"kwu/internal/netclient"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/sandbox"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
	"kwu/internal/userctx"

	// Blank import registers the provider kind the same way cmd/kwu's main
	// does; importing builtin above registers the built-in tools.
	_ "kwu/internal/provider/openai"
)

func TestAgentKeepPolicyFromConfig(t *testing.T) {
	if got := agentKeepPolicy(nil); got != agent.KeepErrors {
		t.Fatalf("nil keep policy = %v, want KeepErrors", got)
	}
	if got := agentKeepPolicy([]string{}); got != 0 {
		t.Fatalf("empty keep policy = %v, want 0", got)
	}
	if got := agentKeepPolicy([]string{"errors", "user_marked"}); got != agent.KeepErrors|agent.KeepUserMarked {
		t.Fatalf("combined keep policy = %v, want errors|user_marked", got)
	}
}

func TestUnavailableSharedMCPStaysOutOfChat(t *testing.T) {
	var events []event.Event
	var stderr bytes.Buffer
	host := plugin.NewHost()
	t.Cleanup(host.Close)
	b := &buildAssembly{
		ctx:        context.Background(),
		sink:       event.FuncSink(func(e event.Event) { events = append(events, e) }),
		stderr:     &stderr,
		pluginHost: host,
		registry:   tool.NewRegistry(),
	}

	b.startSharedEagerPlugin(plugin.Spec{
		Name:    "offline-connector",
		Type:    "stdio",
		Command: filepath.Join(t.TempDir(), "missing-mcp-command"),
	})

	if len(events) != 0 {
		t.Fatalf("startup events = %+v, want no chat notice for unavailable MCP", events)
	}
	if names := b.registry.Names(); len(names) != 0 {
		t.Fatalf("registered tools = %v, want unavailable MCP omitted", names)
	}
	failures := host.Failures()
	if len(failures) != 1 || failures[0].Name != "offline-connector" {
		t.Fatalf("Host.Failures() = %+v, want offline connector status", failures)
	}
	if got := stderr.String(); !strings.Contains(got, `MCP server "offline-connector" unavailable`) {
		t.Fatalf("stderr = %q, want internal unavailable diagnostic", got)
	}
}

func TestResolveAgentModePriority(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Mode = "task_tree"

	tests := []struct {
		name       string
		opts       Options
		wantMode   agentmode.Mode
		wantSource agentmode.ModeSourceKind
	}{
		{
			name:       "config default for new session",
			opts:       Options{},
			wantMode:   agentmode.ModeTaskTree,
			wantSource: agentmode.ModeSourceConfig,
		},
		{
			name:       "request overrides config",
			opts:       Options{AgentMode: "normal"},
			wantMode:   agentmode.ModeNormal,
			wantSource: agentmode.ModeSourceRequest,
		},
		{
			name:       "session overrides request and config",
			opts:       Options{SessionAgentMode: "task_tree", AgentMode: "normal"},
			wantMode:   agentmode.ModeTaskTree,
			wantSource: agentmode.ModeSourceSession,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveAgentMode(tt.opts, cfg)
			if err != nil {
				t.Fatalf("ResolveAgentMode returned error: %v", err)
			}
			if got.Mode != tt.wantMode || got.Source != tt.wantSource {
				t.Fatalf("ResolveAgentMode = %+v, want mode=%q source=%q", got, tt.wantMode, tt.wantSource)
			}
		})
	}
}

func TestResolveAgentModeDefaultsToNormalWithoutExplicitSource(t *testing.T) {
	got, err := ResolveAgentMode(Options{}, config.Default())
	if err != nil {
		t.Fatalf("ResolveAgentMode returned error: %v", err)
	}
	if got.Mode != agentmode.ModeNormal || got.Source != agentmode.ModeSourceDefault {
		t.Fatalf("ResolveAgentMode = %+v, want normal/default", got)
	}
}

func TestResolveAgentModeRejectsUnknownRequest(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Mode = "task_tree"

	if got, err := ResolveAgentMode(Options{AgentMode: "workflow"}, cfg); err == nil {
		t.Fatalf("ResolveAgentMode = %+v, want error", got)
	} else if !strings.Contains(err.Error(), "request agent mode") {
		t.Fatalf("ResolveAgentMode error = %q, want request source context", err.Error())
	}
}

func TestBuildRecordsResolvedAgentMode(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("task-tree-planner",
		testutil.Turn{Text: bootTaskTreePlannerJSON()},
		testutil.Turn{Text: "node execution complete"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
mode = "task_tree"
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.AgentMode(); got != agentmode.ModeTaskTree {
		t.Fatalf("AgentMode() = %q, want %q", got, agentmode.ModeTaskTree)
	}
	if got := ctrl.AgentModeSource(); got != agentmode.ModeSourceConfig {
		t.Fatalf("AgentModeSource() = %q, want %q", got, agentmode.ModeSourceConfig)
	}
	if err := ctrl.Run(context.Background(), "build a task tree"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 2 {
		t.Fatalf("provider requests = %d, want planner + executor", len(reqs))
	}
	if len(reqs[0].Tools) != 0 {
		t.Fatalf("task_tree planner should not expose tools: %v", toolSchemaNames(reqs[0].Tools))
	}
	if len(reqs[1].Tools) == 0 {
		t.Fatal("task_tree executor request should use the normal executor tool surface")
	}
	if !strings.Contains(userMessage(reqs[1].Messages), `"node_id": "plan"`) {
		t.Fatalf("executor request should receive local task node context:\n%s", userMessage(reqs[1].Messages))
	}
}

func TestBuildNormalModeKeepsPlannerCoordinator(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "executor"

[agent]
mode = "normal"
planner_model = "planner"
system_prompt = "BASE"

[[providers]]
name = "executor"
kind = "openai"
base_url = "https://example.invalid"
model = "exec-model"
api_key_env = "KWU_TEST_KEY_UNSET"

[[providers]]
name = "planner"
kind = "openai"
base_url = "https://example.invalid"
model = "plan-model"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.AgentMode(); got != agentmode.ModeNormal {
		t.Fatalf("AgentMode() = %q, want %q", got, agentmode.ModeNormal)
	}
	if got := ctrl.Label(); got != "exec-model + planner plan-model" {
		t.Fatalf("Label() = %q, want Coordinator label", got)
	}
}

func TestBuildDefaultAndNormalModeKeepSameRequestShape(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	defaultReq, defaultMode, defaultSource := firstAgentModeRequest(t, Options{})
	normalReq, normalMode, normalSource := firstAgentModeRequest(t, Options{AgentMode: "normal"})

	if defaultMode != agentmode.ModeNormal || defaultSource != agentmode.ModeSourceDefault {
		t.Fatalf("default mode/source = %q/%q, want normal/default", defaultMode, defaultSource)
	}
	if normalMode != agentmode.ModeNormal || normalSource != agentmode.ModeSourceRequest {
		t.Fatalf("explicit normal mode/source = %q/%q, want normal/request", normalMode, normalSource)
	}
	if got, want := systemMessage(normalReq.Messages), systemMessage(defaultReq.Messages); got != want {
		t.Fatalf("explicit normal changed system prompt\n--- default ---\n%s\n--- normal ---\n%s", want, got)
	}
	if got, want := toolSchemaNames(normalReq.Tools), toolSchemaNames(defaultReq.Tools); !reflect.DeepEqual(got, want) {
		t.Fatalf("explicit normal changed tool schema order\nnormal=%v\ndefault=%v", got, want)
	}
	if !reflect.DeepEqual(normalReq.Tools, defaultReq.Tools) {
		t.Fatalf("explicit normal changed provider-visible tool schemas; names=%v", toolSchemaNames(normalReq.Tools))
	}
	for _, want := range []string{"task", "wait_task", "stop_task", "read_only_task", "parallel_tasks"} {
		if !requestHasTool(defaultReq, want) || !requestHasTool(normalReq, want) {
			t.Fatalf("normal/default request missing %q; default=%v normal=%v", want, toolSchemaNames(defaultReq.Tools), toolSchemaNames(normalReq.Tools))
		}
	}
}

func TestBuildExposesSplitMediaToolsWhenConfigured(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	req, _, _ := firstAgentModeRequest(t, Options{
		ModelBillingOptions: ModelBillingOptions{
			MediaToolCredential: func(context.Context) (string, error) {
				return "test-maas-key", nil
			},
			MediaVideoClient: bootFakeVideoGenerationClient{},
			MediaVideoScope: func(context.Context) (cloud.BindingScope, error) {
				return cloud.BindingScope{ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1}, nil
			},
		},
	})
	for _, name := range []string{"generate_image", "generate_video"} {
		if !requestHasTool(req, name) {
			t.Fatalf("provider request missing %s; tools=%v", name, toolSchemaNames(req.Tools))
		}
		if !requestToolSchemaContains(req, name, `"prompt"`) || requestToolSchemaContains(req, name, `"model"`) {
			t.Fatalf("%s schema must expose only prompt", name)
		}
	}
	if requestHasTool(req, "generate_media") {
		t.Fatal("provider request unexpectedly exposes removed generate_media")
	}
}

type bootFakeVideoGenerationClient struct{}

func (bootFakeVideoGenerationClient) Create(context.Context, cloud.BindingScope, string, cloud.VideoGenerationRequest) (cloud.VideoGenerationTask, error) {
	return cloud.VideoGenerationTask{}, nil
}
func (bootFakeVideoGenerationClient) Get(context.Context, cloud.BindingScope, string) (cloud.VideoGenerationTask, error) {
	return cloud.VideoGenerationTask{}, nil
}
func (bootFakeVideoGenerationClient) GetProgress(context.Context, cloud.BindingScope, string) (cloud.VideoGenerationProgress, error) {
	return cloud.VideoGenerationProgress{}, nil
}

func firstAgentModeRequest(t *testing.T, opts Options) (provider.Request, agentmode.Mode, agentmode.ModeSourceKind) {
	t.Helper()
	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("agent-mode-compat", testutil.Turn{Text: "done"})
	setBootTokenProfileTestProvider(t, prov)
	opts.Sink = event.Discard

	ctrl, err := Build(context.Background(), opts)
	if err != nil {
		t.Fatalf("Build(%+v): %v", opts, err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "capture request shape"); err != nil {
		t.Fatalf("Run(%+v): %v", opts, err)
	}
	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests(%+v) = %d, want 1", opts, len(reqs))
	}
	return reqs[0], ctrl.AgentMode(), ctrl.AgentModeSource()
}

// TestBuildFoldsProjectMemoryIntoSystemPrompt is the end-to-end proof of the
// cache-first wiring: a project KWU.md is discovered at boot and folded
// into the session's system message (the cached prefix), and the `remember`
// tool is registered. It builds a real Controller from a throwaway project dir.
func TestBuildFoldsProjectMemoryIntoSystemPrompt(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)
	writeFile(t, dir, "KWU.md", "Project rule: always run go vet before committing.")

	ctrl, err := Build(context.Background(), Options{}) // RequireKey false: no network/key needed
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	// The system message is the cached prefix; it must contain both the base
	// prompt and the discovered memory.
	sys := systemMessage(ctrl.History())
	if !strings.Contains(sys, "BASE SYSTEM PROMPT") {
		t.Fatalf("base prompt missing from system message:\n%s", sys)
	}
	if !strings.Contains(sys, "always run go vet before committing") {
		t.Fatalf("project KWU.md not folded into system message:\n%s", sys)
	}
	// Base must come first so it stays a valid cache prefix when memory changes.
	if strings.Index(sys, "BASE SYSTEM PROMPT") > strings.Index(sys, "always run go vet") {
		t.Fatalf("memory should follow the base prompt, not precede it:\n%s", sys)
	}

	if mem := ctrl.Memory(); mem == nil || len(mem.Docs) == 0 {
		t.Fatal("controller memory set is empty after discovering KWU.md")
	}
}

func TestBuildDoesNotInjectProfileIntoSystemPrompt(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[profile]
learning = true
loading = true

[profile.global]
learning = true
loading = true

[profile.project]
learning = true
loading = true

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sys := systemMessage(ctrl.History())
	if strings.Contains(sys, "来自用户画像") {
		t.Fatalf("profile context must not enter the cache-stable system prompt:\n%s", sys)
	}
}

func TestBuildAppliesExpertOnlyWhenSelected(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	expertStorePath := filepath.Join(dir, "experts.sqlite")

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	withoutExpert, err := Build(context.Background(), Options{ExpertStorePath: expertStorePath})
	if err != nil {
		t.Fatalf("Build without expert: %v", err)
	}
	defer withoutExpert.Close()
	sys := systemMessage(withoutExpert.History())
	if strings.Contains(sys, "Expert role:") {
		t.Fatalf("no-expert system prompt contains expert block:\n%s", sys)
	}

	withExpert, err := Build(context.Background(), Options{ExpertID: "testing", ExpertStorePath: expertStorePath})
	if err != nil {
		t.Fatalf("Build with expert: %v", err)
	}
	defer withExpert.Close()
	expertSys := systemMessage(withExpert.History())
	if !strings.Contains(expertSys, "Expert role: 测试策略专家") {
		t.Fatalf("expert system prompt missing testing role:\n%s", expertSys)
	}
	if strings.Index(expertSys, "BASE SYSTEM PROMPT") > strings.Index(expertSys, "Expert role:") {
		t.Fatalf("expert block should follow base prompt:\n%s", expertSys)
	}
}

func TestBuildLoadsUserExpertCatalog(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	userExpertPath := filepath.Join(dir, "experts.json")
	record, err := expert.NewUserExpert("custom-local-reviewer", expert.UserExpertInput{
		Name:          "Local Reviewer",
		Description:   "Reviews local code.",
		Category:      "engineering",
		SystemPrompt:  "Review the local code as a user-defined expert.",
		DisplayPrompt: "Review this change.",
		Enabled:       true,
	}, time.UnixMilli(1000))
	if err != nil {
		t.Fatalf("NewUserExpert: %v", err)
	}
	if err := expert.SaveUserExperts(userExpertPath, []expert.UserExpert{record}); err != nil {
		t.Fatalf("SaveUserExperts: %v", err)
	}

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{ExpertID: record.ID, UserExpertPath: userExpertPath})
	if err != nil {
		t.Fatalf("Build with user expert: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.CurrentExpert(); got.SelectedID != record.ID {
		t.Fatalf("current expert = %+v, want user expert", got)
	}
	if sys := systemMessage(ctrl.History()); !strings.Contains(sys, record.SystemPrompt) {
		t.Fatalf("system prompt missing user expert prompt:\n%s", sys)
	}
}

func TestBuildUsesInMemoryExpertCatalogLoaderWithoutSQLite(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)
	sqlitePath := filepath.Join(dir, "must-not-exist.sqlite")
	var loads int
	loader := func() (*expert.Catalog, error) {
		loads++
		return expert.NewCatalog([]expert.Expert{{
			ID: "memory-expert", Name: "Memory Expert", Prompt: "Use the in-memory expert.",
			Enabled: true, Source: expert.SourceOfficial, Mode: expert.ModeExpert,
		}})
	}

	ctrl, err := Build(context.Background(), Options{
		ExpertID: "memory-expert", ExpertStorePath: sqlitePath, ExpertCatalogLoader: loader,
	})
	if err != nil {
		t.Fatalf("Build with in-memory expert: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.CurrentExpert(); got.SelectedID != "memory-expert" {
		t.Fatalf("current expert = %+v, want memory-expert", got)
	}
	if sys := systemMessage(ctrl.History()); !strings.Contains(sys, "Use the in-memory expert.") {
		t.Fatalf("system prompt missing in-memory expert prompt:\n%s", sys)
	}
	if loads < 2 {
		t.Fatalf("in-memory expert loader calls = %d, want initial catalog and prompt resolution", loads)
	}
	if _, err := os.Stat(sqlitePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("SQLite expert store was accessed at %q: %v", sqlitePath, err)
	}
}

func TestBuildUsesSQLiteExpertStoreForSessionExpert(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	storePath := filepath.Join(dir, "experts.sqlite")
	store := expert.NewStore(storePath, nil)
	if err := store.Save(context.Background(), expert.Definition{
		ID:          "sqlite-team",
		Mode:        expert.ModeTeam,
		Name:        "SQLite Team",
		Role:        "SQLite orchestration",
		Description: "Loaded from the controllable SQLite expert catalog.",
		Categories:  []string{"SQLite"},
		Tags:        []string{"store"},
		IconKey:     "users",
		Tone:        "teal",
		Members: []expert.Member{
			{Kind: "lead", Name: "SQLite Lead", Role: "Coordinator", Task: "Coordinate the team.", Output: "Plan", Tone: "blue"},
			{Kind: "sub", Name: "SQLite Worker", Role: "Execution", Task: "Execute the delegated work.", Output: "Result", Tone: "green"},
		},
		Prompts: []string{
			"Coordinate with the SQLite-backed expert team.",
			"Review the SQLite-backed delivery plan.",
			"Identify risks in the SQLite-backed task.",
		},
	}); err != nil {
		t.Fatalf("save sqlite expert: %v", err)
	}

	ctrl, err := Build(context.Background(), Options{ExpertID: "sqlite-team", ExpertStorePath: storePath})
	if err != nil {
		t.Fatalf("Build with SQLite expert: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.CurrentExpert(); got.SelectedID != "sqlite-team" || got.SelectedName != "SQLite Team" {
		t.Fatalf("current expert = %+v, want sqlite-team from SQLite store", got)
	}
	if sys := systemMessage(ctrl.History()); !strings.Contains(sys, "Coordinate with the SQLite-backed expert team.") {
		t.Fatalf("system prompt missing SQLite expert prompt:\n%s", sys)
	}

	resumed, err := Build(context.Background(), Options{SessionExpertID: "sqlite-team", ExpertStorePath: storePath})
	if err != nil {
		t.Fatalf("Build with resumed SQLite expert: %v", err)
	}
	defer resumed.Close()
	if got := resumed.CurrentExpert(); got.SelectedID != "sqlite-team" || got.Source != expert.SourceSession {
		t.Fatalf("resumed expert = %+v, want sqlite-team from session metadata", got)
	}
	if sys := systemMessage(resumed.History()); !strings.Contains(sys, "Coordinate with the SQLite-backed expert team.") {
		t.Fatalf("resumed system prompt missing SQLite expert prompt:\n%s", sys)
	}
}

func TestBuildSessionExplicitNoneOverridesDefaultExpert(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"
default_expert = "architecture-review"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	defaultExpert, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build default expert: %v", err)
	}
	defer defaultExpert.Close()
	if got := defaultExpert.CurrentExpert(); got.SelectedID != "architecture-review" {
		t.Fatalf("default expert state = %+v, want architecture-review", got)
	}

	legacySession, err := Build(context.Background(), Options{SessionExpertID: expert.ExplicitNoneID})
	if err != nil {
		t.Fatalf("Build explicit none: %v", err)
	}
	defer legacySession.Close()
	if got := legacySession.CurrentExpert(); got.SelectedID != "" || got.Source != expert.SourceNone {
		t.Fatalf("explicit none state = %+v, want none", got)
	}
	if sys := systemMessage(legacySession.History()); strings.Contains(sys, "Expert role:") {
		t.Fatalf("explicit none should suppress config default expert:\n%s", sys)
	}
}

func TestBuildIgnoresUnknownRestoredSessionExpert(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE SYSTEM PROMPT"
default_expert = "architecture-review"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{SessionExpertID: "missing-restored-session-expert"})
	if err != nil {
		t.Fatalf("Build with unknown restored session expert should not fail: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.CurrentExpert(); got.SelectedID != "" || got.Source != expert.SourceNone {
		t.Fatalf("restored unknown expert state = %+v, want none", got)
	}
	if sys := systemMessage(ctrl.History()); strings.Contains(sys, "Expert role:") {
		t.Fatalf("restored unknown expert should suppress config default expert:\n%s", sys)
	}
}

func TestBuildRunsCleanupPendingReconciler(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)
	sessionDir := filepath.Join(t.TempDir(), "sessions")
	called := false
	ctrl, err := Build(context.Background(), Options{
		SessionDir: sessionDir,
		CleanupPendingReconciler: func(got string) error {
			called = true
			if filepath.Clean(got) != filepath.Clean(sessionDir) {
				t.Fatalf("reconciler dir = %q, want %q", got, sessionDir)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if !called {
		t.Fatal("cleanup-pending reconciler was not called")
	}
}

func TestBuildUsesActorProfileSessionDirFallback(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	actor := userctx.ActorContext{
		ActorID:       "profile-user",
		ProfileID:     "profile-a",
		AuthMethod:    "test",
		WorkspaceRoot: project,
	}
	want := config.NewProfilePathResolver(actor).SessionDir()
	if filepath.Clean(want) == filepath.Clean(config.SessionDir()) {
		t.Fatalf("test setup error: profile session dir %q should differ from default %q", want, config.SessionDir())
	}

	var reconciled string
	ctrl, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		Actor:         actor,
		CleanupPendingReconciler: func(got string) error {
			reconciled = got
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if got := ctrl.SessionDir(); filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("SessionDir = %q, want actor profile session dir %q", got, want)
	}
	if filepath.Clean(reconciled) != filepath.Clean(want) {
		t.Fatalf("cleanup reconciler dir = %q, want actor profile session dir %q", reconciled, want)
	}
}

func TestBuildLoadsMemoryFromActorProfileUserDir(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	actor := userctx.ActorContext{
		ActorID:       "profile-user",
		ProfileID:     "profile-a",
		AuthMethod:    "test",
		WorkspaceRoot: project,
	}
	profilePaths := config.NewProfilePathResolver(actor)
	profileStore := memory.StoreForProjectRoot(profilePaths.MemoryUserDir(), profilePaths.MemoryProjectStoreRoot(), project)
	if _, err := profileStore.Save(memory.Memory{Name: "alpha-user-memory", Description: "profile a memory", Type: memory.TypeUser, Body: "alpha profile memory body"}); err != nil {
		t.Fatal(err)
	}
	if _, err := profileStore.Save(memory.Memory{Name: "shared-project-memory", Description: "workspace project memory", Type: memory.TypeProject, Body: "shared project memory body"}); err != nil {
		t.Fatal(err)
	}
	defaultStore := memory.StoreFor(config.MemoryUserDir(), project)
	if _, err := defaultStore.Save(memory.Memory{Name: "default-user-memory", Description: "default memory", Type: memory.TypeUser, Body: "default profile should not load"}); err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(profileStore.GlobalDir) == filepath.Clean(defaultStore.GlobalDir) {
		t.Fatalf("test setup error: profile GlobalDir %q should differ from default %q", profileStore.GlobalDir, defaultStore.GlobalDir)
	}

	ctrl, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		Actor:         actor,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	mem := ctrl.Memory()
	if mem == nil {
		t.Fatal("controller memory is nil")
	}
	if filepath.Clean(mem.Store.GlobalDir) != filepath.Clean(profileStore.GlobalDir) {
		t.Fatalf("memory GlobalDir = %q, want actor profile GlobalDir %q", mem.Store.GlobalDir, profileStore.GlobalDir)
	}
	sys := systemMessage(ctrl.History())
	if !strings.Contains(sys, "alpha-user-memory.md") {
		t.Fatalf("system prompt missing actor profile memory index:\n%s", sys)
	}
	if !strings.Contains(sys, "shared-project-memory.md") {
		t.Fatalf("system prompt missing shared project memory index:\n%s", sys)
	}
	if strings.Contains(sys, "default-user-memory.md") {
		t.Fatalf("system prompt leaked default profile memory:\n%s", sys)
	}
}

func TestBuildActorContextDoesNotAffectSystemPromptWhenMemoryEmpty(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	actorA := userctx.ActorContext{ActorID: "actor-alpha", ProfileID: "profile-alpha", AuthMethod: "test", WorkspaceRoot: project}
	actorB := userctx.ActorContext{ActorID: "actor-beta", ProfileID: "profile-beta", AuthMethod: "oauth", WorkspaceRoot: project}
	ctrlA, err := Build(context.Background(), Options{Sink: event.Discard, WorkspaceRoot: project, Actor: actorA})
	if err != nil {
		t.Fatalf("Build actor A: %v", err)
	}
	defer ctrlA.Close()
	ctrlB, err := Build(context.Background(), Options{Sink: event.Discard, WorkspaceRoot: project, Actor: actorB})
	if err != nil {
		t.Fatalf("Build actor B: %v", err)
	}
	defer ctrlB.Close()

	sysA := systemMessage(ctrlA.History())
	sysB := systemMessage(ctrlB.History())
	if sysA != sysB {
		t.Fatalf("empty-memory system prompt changed across actors:\nA:\n%s\nB:\n%s", sysA, sysB)
	}
	for _, forbidden := range []string{"actor-alpha", "actor-beta", "profile-alpha", "profile-beta"} {
		if strings.Contains(sysA, forbidden) {
			t.Fatalf("system prompt leaked actor/profile identity %q:\n%s", forbidden, sysA)
		}
	}
}

func TestBuildActorContextDoesNotAffectProviderVisibleRequestWhenMemoryEmpty(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	actorA := userctx.ActorContext{ActorID: "actor-alpha", ProfileID: "profile-alpha", AuthMethod: "test", WorkspaceRoot: project}
	actorB := userctx.ActorContext{ActorID: "actor-beta", ProfileID: "profile-beta", AuthMethod: "oauth", WorkspaceRoot: project}
	reqA := firstActorRequest(t, project, actorA)
	reqB := firstActorRequest(t, project, actorB)
	if got, want := systemMessage(reqA.Messages), systemMessage(reqB.Messages); got != want {
		t.Fatalf("empty-memory system prompt changed across actors\nA:\n%s\nB:\n%s", got, want)
	}
	if !reflect.DeepEqual(reqA.Tools, reqB.Tools) {
		t.Fatalf("empty-memory provider-visible tool schemas changed across actors\nA=%v\nB=%v",
			toolSchemaNames(reqA.Tools), toolSchemaNames(reqB.Tools))
	}
	body, err := json.Marshal(reqA)
	if err != nil {
		t.Fatalf("marshal provider request: %v", err)
	}
	for _, forbidden := range []string{"actor-alpha", "actor-beta", "profile-alpha", "profile-beta", "ActorContext", "ProfileID", "CredentialOwner"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("provider-visible request leaked actor/profile identity %q:\n%s", forbidden, body)
		}
	}
}

func TestBuildProfileMemoryOnlyChangesSystemPromptByMemoryContent(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	actorA := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-alpha", AuthMethod: "test", WorkspaceRoot: project}
	actorB := userctx.ActorContext{ActorID: "actor-b", ProfileID: "profile-beta", AuthMethod: "test", WorkspaceRoot: project}
	storeA := memory.StoreFor(config.NewProfilePathResolver(actorA).MemoryUserDir(), project)
	storeB := memory.StoreFor(config.NewProfilePathResolver(actorB).MemoryUserDir(), project)
	if _, err := storeA.Save(memory.Memory{Name: "preferred-shell", Description: "prefers pwsh", Type: memory.TypeUser, Body: "Use pwsh."}); err != nil {
		t.Fatal(err)
	}
	if _, err := storeB.Save(memory.Memory{Name: "preferred-shell", Description: "prefers bash", Type: memory.TypeUser, Body: "Use bash."}); err != nil {
		t.Fatal(err)
	}

	ctrlA, err := Build(context.Background(), Options{Sink: event.Discard, WorkspaceRoot: project, Actor: actorA})
	if err != nil {
		t.Fatalf("Build actor A: %v", err)
	}
	defer ctrlA.Close()
	ctrlB, err := Build(context.Background(), Options{Sink: event.Discard, WorkspaceRoot: project, Actor: actorB})
	if err != nil {
		t.Fatalf("Build actor B: %v", err)
	}
	defer ctrlB.Close()

	sysA := systemMessage(ctrlA.History())
	sysB := systemMessage(ctrlB.History())
	if !strings.Contains(sysA, "prefers pwsh") || strings.Contains(sysA, "prefers bash") {
		t.Fatalf("profile A prompt has wrong memory content:\n%s", sysA)
	}
	if !strings.Contains(sysB, "prefers bash") || strings.Contains(sysB, "prefers pwsh") {
		t.Fatalf("profile B prompt has wrong memory content:\n%s", sysB)
	}
	if strings.Index(sysA, "BASE") > strings.Index(sysA, "# Memory") || strings.Index(sysB, "BASE") > strings.Index(sysB, "# Memory") {
		t.Fatalf("memory should follow the cache-stable base prompt:\nA:\n%s\nB:\n%s", sysA, sysB)
	}
	for _, forbidden := range []string{"actor-a", "actor-b", "profile-alpha", "profile-beta"} {
		if strings.Contains(sysA, forbidden) || strings.Contains(sysB, forbidden) {
			t.Fatalf("system prompt leaked actor/profile identity %q:\nA:\n%s\nB:\n%s", forbidden, sysA, sysB)
		}
	}
}

func TestBuildHistoryGlobalSearchUsesActorProfileRoots(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "boot-retrieval-tool-test"
model = "x"
`)

	actor := userctx.ActorContext{
		ActorID:       "profile-user",
		ProfileID:     "profile-a",
		AuthMethod:    "test",
		WorkspaceRoot: project,
	}
	profilePaths := config.NewProfilePathResolver(actor)
	currentSessionDir := filepath.Join(t.TempDir(), "current-sessions")
	writeBootSession(t, filepath.Join(currentSessionDir, "current.jsonl"), "stagefive orchid current session visible")
	writeBootSession(t, filepath.Join(profilePaths.SessionDir(), "profile-global.jsonl"), "stagefive orchid profile global visible")
	writeBootSession(t, filepath.Join(profilePaths.ArchiveDir(), "profile-archive.jsonl"), "stagefive orchid profile archive visible")

	defaultPaths := config.NewProfilePathResolver(userctx.Default(project))
	writeBootSession(t, filepath.Join(defaultPaths.SessionDir(), "default.jsonl"), "stagefive orchid default profile secret")
	writeBootSession(t, filepath.Join(defaultPaths.ArchiveDir(), "default-archive.jsonl"), "stagefive orchid default archive secret")
	otherActor := actor
	otherActor.ProfileID = "profile-b"
	otherPaths := config.NewProfilePathResolver(otherActor)
	writeBootSession(t, filepath.Join(otherPaths.SessionDir(), "other.jsonl"), "stagefive orchid profile bee secret")
	writeBootSession(t, filepath.Join(otherPaths.ArchiveDir(), "other-archive.jsonl"), "stagefive orchid profile bee archive secret")

	registerBootRetrievalToolTestProvider()
	prov := testutil.NewMock("boot-retrieval-tool-test",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "history-profile", Name: "history", Arguments: `{"operation":"search","query":"stagefive orchid","scope":"global","limit":20}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootRetrievalToolTestProvider(t, prov)

	ctrl, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		SessionDir:    currentSessionDir,
		Actor:         actor,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.Run(context.Background(), "recover profile-scoped history"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	toolOutput := historyToolOutput(ctrl.History(), "history")
	for _, want := range []string{
		"stagefive orchid current session visible",
		"stagefive orchid profile global visible",
		"stagefive orchid profile archive visible",
	} {
		if !strings.Contains(toolOutput, want) {
			t.Fatalf("history output missing %q:\n%s", want, toolOutput)
		}
	}
	for _, forbidden := range []string{
		"stagefive orchid default profile secret",
		"stagefive orchid default archive secret",
		"stagefive orchid profile bee secret",
		"stagefive orchid profile bee archive secret",
	} {
		if strings.Contains(toolOutput, forbidden) {
			t.Fatalf("history output leaked %q:\n%s", forbidden, toolOutput)
		}
	}
}

func TestBuildCompactArchivesToActorProfileArchiveDir(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "boot-retrieval-tool-test"
model = "x"
`)

	actor := userctx.ActorContext{
		ActorID:       "profile-user",
		ProfileID:     "profile-a",
		AuthMethod:    "test",
		WorkspaceRoot: project,
	}
	profileArchiveDir := config.NewProfilePathResolver(actor).ArchiveDir()
	defaultArchiveDir := config.ArchiveDir()
	if filepath.Clean(profileArchiveDir) == filepath.Clean(defaultArchiveDir) {
		t.Fatalf("test setup error: profile archive dir %q should differ from default %q", profileArchiveDir, defaultArchiveDir)
	}

	registerBootRetrievalToolTestProvider()
	prov := testutil.NewMock("boot-retrieval-tool-test", testutil.Turn{Text: "compact digest"})
	setBootRetrievalToolTestProvider(t, prov)

	ctrl, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		Actor:         actor,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	big := strings.Repeat("profile compact archive detail ", 120)
	ctrl.Resume(&agent.Session{Messages: []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: "task"},
		{Role: provider.RoleAssistant, Content: big},
		{Role: provider.RoleAssistant, Content: big},
		{Role: provider.RoleUser, Content: "next"},
		{Role: provider.RoleAssistant, Content: "ok"},
	}}, "")

	if err := ctrl.Compact(context.Background(), ""); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if got := jsonlEntryCount(t, profileArchiveDir); got != 1 {
		t.Fatalf("profile archive entries = %d, want 1 in %s", got, profileArchiveDir)
	}
	if got := jsonlEntryCount(t, defaultArchiveDir); got != 0 {
		t.Fatalf("default archive entries = %d, want 0 in %s", got, defaultArchiveDir)
	}
}

func TestBuildDefaultsActorContextFromResolvedWorkspaceRoot(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	actor := ctrl.ActorContext()
	if actor.ActorID != userctx.DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, userctx.DefaultActorID)
	}
	if actor.ProfileID != userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", actor.ProfileID, userctx.DefaultProfileID)
	}
	if actor.AuthMethod != userctx.DefaultAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, userctx.DefaultAuthMethod)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(dir) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, dir)
	}
}

func TestBuildOverridesActorWorkspaceRootWithResolvedRoot(t *testing.T) {
	isolateConfigHome(t)
	cwd := robustTempDir(t)
	t.Chdir(cwd)
	project := robustTempDir(t)

	writeFile(t, project, "kwu.toml", `
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		Actor: userctx.ActorContext{
			ActorID:       "actor-1",
			ProfileID:     "profile-1",
			AuthMethod:    "oauth",
			WorkspaceRoot: filepath.Join(cwd, "stale"),
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	actor := ctrl.ActorContext()
	if actor.ActorID != "actor-1" || actor.ProfileID != "profile-1" || actor.AuthMethod != "oauth" {
		t.Fatalf("ActorContext identity = %+v", actor)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(project) {
		t.Fatalf("WorkspaceRoot = %q, want resolved root %q", actor.WorkspaceRoot, project)
	}
}

func TestBuildUsesActorProfileConfigWithoutDefaultLegacyMigration(t *testing.T) {
	home := isolateBootProfilePaths(t)
	project := robustTempDir(t)
	writeFile(t, project, "kwu.toml", "")
	writeFile(t, filepath.Join(home, ".kwu"), "config.json", `{"apiKey":"sk-default-legacy"}`)

	actor := userctx.ActorContext{
		ActorID:       "actor-work",
		ProfileID:     "work",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(project, "stale"),
	}
	paths := config.NewProfilePathResolver(actor)
	writeFile(t, filepath.Dir(paths.UserConfigPath()), filepath.Base(paths.UserConfigPath()), `
default_model = "profile-only"

[agent]
system_prompt = "PROFILE SYSTEM"

[[providers]]
name = "profile-only"
kind = "openai"
base_url = "https://example.invalid"
model = "profile-model"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	var notices []string
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e.Text)
		}
	})

	ctrl, err := Build(context.Background(), Options{
		Sink:          sink,
		WorkspaceRoot: project,
		Actor:         actor,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if ctrl.Label() != "profile-model" {
		t.Fatalf("controller label = %q, want profile config model", ctrl.Label())
	}
	if sys := systemMessage(ctrl.History()); !strings.Contains(sys, "PROFILE SYSTEM") {
		t.Fatalf("system prompt did not come from profile config:\n%s", sys)
	}
	gotActor := ctrl.ActorContext()
	if gotActor.ActorID != "actor-work" || gotActor.ProfileID != "work" || gotActor.AuthMethod != "oauth" {
		t.Fatalf("ActorContext identity = %+v", gotActor)
	}
	if filepath.Clean(gotActor.WorkspaceRoot) != filepath.Clean(project) {
		t.Fatalf("WorkspaceRoot = %q, want resolved root %q", gotActor.WorkspaceRoot, project)
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("non-default profile should not migrate default legacy config, stat err=%v", err)
	}
	for _, n := range notices {
		if strings.Contains(n, "migrated your previous configuration") {
			t.Fatalf("non-default profile emitted default migration notice: %v", notices)
		}
	}
}

const bootCredentialOwnerProviderKind = "boot-credential-owner-test"

var (
	bootCredentialOwnerProviderOnce sync.Once
	bootCredentialOwnerProviderMu   sync.Mutex
	bootCredentialOwnerProviderCfgs []provider.Config
)

func registerBootCredentialOwnerProvider() {
	bootCredentialOwnerProviderOnce.Do(func() {
		provider.Register(bootCredentialOwnerProviderKind, func(cfg provider.Config) (provider.Provider, error) {
			bootCredentialOwnerProviderMu.Lock()
			bootCredentialOwnerProviderCfgs = append(bootCredentialOwnerProviderCfgs, cfg)
			bootCredentialOwnerProviderMu.Unlock()
			return bootCredentialOwnerProvider{}, nil
		})
	})
}

type bootCredentialOwnerProvider struct{}

func (bootCredentialOwnerProvider) Name() string { return "boot-credential-owner-test" }

func (bootCredentialOwnerProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk)
	close(ch)
	return ch, nil
}

func resetBootCredentialOwnerProviderCfgs() {
	bootCredentialOwnerProviderMu.Lock()
	defer bootCredentialOwnerProviderMu.Unlock()
	bootCredentialOwnerProviderCfgs = nil
}

func lastBootCredentialOwnerProviderConfig(t *testing.T) provider.Config {
	t.Helper()
	bootCredentialOwnerProviderMu.Lock()
	defer bootCredentialOwnerProviderMu.Unlock()
	if len(bootCredentialOwnerProviderCfgs) == 0 {
		t.Fatal("provider factory was not called")
	}
	return bootCredentialOwnerProviderCfgs[len(bootCredentialOwnerProviderCfgs)-1]
}

func TestBuildUsesSameProfileKeyForProviderAndBalance(t *testing.T) {
	registerBootCredentialOwnerProvider()
	resetBootCredentialOwnerProviderCfgs()
	isolateBootProfilePaths(t)
	project := robustTempDir(t)

	const keyEnv = "KWU_STAGE403_SHARED_KEY"
	var gotAuth string
	balanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"42","granted_balance":"0","topped_up_balance":"42"}]}`))
	}))
	defer balanceServer.Close()

	writeFile(t, project, "kwu.toml", `
default_model = "same/active"

[[providers]]
name = "same"
kind = "`+bootCredentialOwnerProviderKind+`"
base_url = "https://example.invalid"
model = "active"
api_key_env = "`+keyEnv+`"
balance_url = "`+balanceServer.URL+`"
`)

	alpha := userctx.ActorContext{ActorID: "alpha-user", ProfileID: "alpha", AuthMethod: "test", WorkspaceRoot: project}
	beta := userctx.ActorContext{ActorID: "beta-user", ProfileID: "beta", AuthMethod: "test", WorkspaceRoot: project}
	if _, err := config.StoreCredentialLinesWithPaths(config.NewProfilePathResolver(alpha), []string{keyEnv + "=alpha-key"}); err != nil {
		t.Fatalf("store alpha credential: %v", err)
	}
	if _, err := config.StoreCredentialLinesWithPaths(config.NewProfilePathResolver(beta), []string{keyEnv + "=beta-key"}); err != nil {
		t.Fatalf("store beta credential: %v", err)
	}

	ctrl, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		Actor:         alpha,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	cfg := lastBootCredentialOwnerProviderConfig(t)
	if cfg.APIKey != "alpha-key" {
		t.Fatalf("provider APIKey = %q, want alpha profile key", cfg.APIKey)
	}
	if got := cfg.Extra["credential_owner"]; got != "profile alpha" {
		t.Fatalf("credential_owner = %v, want profile alpha", got)
	}
	if got := cfg.Extra["api_key_source"]; !strings.Contains(fmt.Sprint(got), "profile alpha") {
		t.Fatalf("api_key_source = %v, want profile alpha", got)
	}

	bal, err := ctrl.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal == nil || bal.Display() != "$42" {
		t.Fatalf("Balance = %#v, want USD 42", bal)
	}
	if gotAuth != "Bearer alpha-key" {
		t.Fatalf("balance Authorization = %q, want alpha profile key", gotAuth)
	}
}

func TestBuildMissingAPIKeyMessageNamesActorProfileCredentials(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	const keyEnv = "KWU_STAGE403_MISSING_KEY"
	writeFile(t, project, "kwu.toml", `
default_model = "missing"

[[providers]]
name = "missing"
kind = "openai"
base_url = "https://example.invalid"
model = "m"
api_key_env = "`+keyEnv+`"
`)
	actor := userctx.ActorContext{ActorID: "work-user", ProfileID: "work", AuthMethod: "test", WorkspaceRoot: project}
	credentialsPath := config.NewProfilePathResolver(actor).UserCredentialsPath()

	var notices []string
	ctrl, err := Build(context.Background(), Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.Notice {
				notices = append(notices, e.Text)
			}
		}),
		WorkspaceRoot: project,
		Actor:         actor,
	})
	if err != nil {
		t.Fatalf("Build with RequireKey=false: %v", err)
	}
	defer ctrl.Close()
	if !containsAll(notices, keyEnv, "profile work", credentialsPath) {
		t.Fatalf("missing-key notice should name key env, owner, and credentials path; notices = %v", notices)
	}

	_, err = Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		Actor:         actor,
		RequireKey:    true,
	})
	if err == nil {
		t.Fatal("Build with RequireKey=true should fail for missing key")
	}
	msg := err.Error()
	for _, want := range []string{keyEnv, "profile work", credentialsPath} {
		if !strings.Contains(msg, want) {
			t.Fatalf("RequireKey error %q should contain %q", msg, want)
		}
	}
}

func TestBuildRequireKeyPreservesBaseURLErrorWhenAPIKeyMissing(t *testing.T) {
	isolateBootProfilePaths(t)
	project := robustTempDir(t)
	const keyEnv = "KWU_STAGE403_BASE_URL_AND_KEY_MISSING"
	writeFile(t, project, "kwu.toml", `
default_model = "broken"

[[providers]]
name = "broken"
kind = "openai"
model = "m"
api_key_env = "`+keyEnv+`"
`)
	actor := userctx.ActorContext{ActorID: "work-user", ProfileID: "work", AuthMethod: "test", WorkspaceRoot: project}

	_, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: project,
		Actor:         actor,
		RequireKey:    true,
	})
	if err == nil {
		t.Fatal("Build with RequireKey=true should fail for missing base_url")
	}
	msg := err.Error()
	if !strings.Contains(msg, `provider "broken": base_url is required`) {
		t.Fatalf("RequireKey error = %q, want base_url validation error", msg)
	}
	for _, masked := range []string{keyEnv, "profile work", "credentials:"} {
		if strings.Contains(msg, masked) {
			t.Fatalf("RequireKey error %q should not be replaced with missing-key detail %q", msg, masked)
		}
	}
}

func containsAll(values []string, wants ...string) bool {
	for _, value := range values {
		ok := true
		for _, want := range wants {
			if !strings.Contains(value, want) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func writeBootSession(t *testing.T, path, content string) {
	t.Helper()
	sess := agent.NewSession("")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: content})
	if err := sess.Save(path); err != nil {
		t.Fatalf("save session %s: %v", path, err)
	}
}

func historyToolOutput(msgs []provider.Message, name string) string {
	var out string
	for _, msg := range msgs {
		if msg.Role == provider.RoleTool && msg.Name == name {
			out += "\n" + msg.Content
		}
	}
	return out
}

func jsonlEntryCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
			count++
		}
	}
	return count
}

func TestBuildRegistersUsableHistoryAndMemoryRetrievalTools(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-retrieval-tool-test"
model = "x"
`)

	sessionDir := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	past := agent.NewSession("")
	past.Add(provider.Message{Role: provider.RoleUser, Content: "Should the history layer use vector embeddings?"})
	past.Add(provider.Message{Role: provider.RoleAssistant, Content: "Decision: port lightweight BM25 history retrieval without a vector database."})
	if err := past.Save(filepath.Join(sessionDir, "past.jsonl")); err != nil {
		t.Fatalf("save past session: %v", err)
	}

	store := memory.StoreFor(config.MemoryUserDir(), dir)
	if _, err := store.Save(memory.Memory{
		Name:        "synthesis-cache-policy",
		Description: "Stable conclusions should be reused from memory",
		Type:        memory.TypeFeedback,
		Body:        "Use a synthesis cache document when expensive retrieval produced a stable conclusion.",
	}); err != nil {
		t.Fatalf("save memory: %v", err)
	}

	registerBootRetrievalToolTestProvider()
	prov := testutil.NewMock("boot-retrieval-tool-test",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "history-1", Name: "history", Arguments: `{"operation":"search","query":"BM25 vector database","scope":"project","limit":5}`},
			{ID: "memory-1", Name: "memory", Arguments: `{"operation":"search","query":"synthesis cache stable conclusion","limit":5}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootRetrievalToolTestProvider(t, prov)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, SessionDir: sessionDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sys := systemMessage(ctrl.History())
	for _, forbidden := range []string{
		"Decision: port lightweight BM25 history retrieval without a vector database.",
		"Use a synthesis cache document when expensive retrieval produced a stable conclusion.",
	} {
		if strings.Contains(sys, forbidden) {
			t.Fatalf("retrieval content should stay behind on-demand tools, not enter the cache-stable system prompt:\n%s", sys)
		}
	}

	if err := ctrl.Run(context.Background(), "recover past context"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) == 0 {
		t.Fatal("provider received no requests")
	}
	for _, want := range []string{"history", "memory", "remember", "forget"} {
		if !requestHasTool(reqs[0], want) {
			t.Fatalf("first request missing tool %q; tools=%v", want, toolSchemaNames(reqs[0].Tools))
		}
	}
	assertToolOrder(t, reqs[0].Tools, []string{"forget", "history", "memory", "remember"})

	toolResults := map[string]string{}
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool {
			toolResults[msg.Name] += "\n" + msg.Content
		}
	}
	if !strings.Contains(toolResults["history"], "port lightweight BM25 history retrieval") {
		t.Fatalf("history tool result did not include saved session decision:\n%s", toolResults["history"])
	}
	if !strings.Contains(toolResults["memory"], "synthesis-cache-policy") ||
		!strings.Contains(toolResults["memory"], "stable conclusion") {
		t.Fatalf("memory tool result did not include saved memory:\n%s", toolResults["memory"])
	}
}

const bootRetrievalToolTestProviderKind = "boot-retrieval-tool-test"

var (
	bootRetrievalToolTestProviderOnce    sync.Once
	bootRetrievalToolTestProviderCurrent *testutil.MockProvider
	bootRetrievalToolTestProviderMu      sync.Mutex
)

func registerBootRetrievalToolTestProvider() {
	bootRetrievalToolTestProviderOnce.Do(func() {
		provider.Register(bootRetrievalToolTestProviderKind, func(provider.Config) (provider.Provider, error) {
			bootRetrievalToolTestProviderMu.Lock()
			defer bootRetrievalToolTestProviderMu.Unlock()
			if bootRetrievalToolTestProviderCurrent == nil {
				return nil, errors.New("boot retrieval tool test provider is not installed")
			}
			return bootRetrievalToolTestProviderCurrent, nil
		})
	})
}

func setBootRetrievalToolTestProvider(t *testing.T, p *testutil.MockProvider) {
	t.Helper()
	bootRetrievalToolTestProviderMu.Lock()
	bootRetrievalToolTestProviderCurrent = p
	bootRetrievalToolTestProviderMu.Unlock()
	t.Cleanup(func() {
		bootRetrievalToolTestProviderMu.Lock()
		if bootRetrievalToolTestProviderCurrent == p {
			bootRetrievalToolTestProviderCurrent = nil
		}
		bootRetrievalToolTestProviderMu.Unlock()
	})
}

const bootTokenProfileTestProviderKind = "boot-token-profile-test"

var (
	bootTokenProfileTestProviderOnce    sync.Once
	bootTokenProfileTestProviderCurrent *testutil.MockProvider
	bootTokenProfileTestProviderMu      sync.Mutex
)

func registerBootTokenProfileTestProvider() {
	bootTokenProfileTestProviderOnce.Do(func() {
		provider.Register(bootTokenProfileTestProviderKind, func(provider.Config) (provider.Provider, error) {
			bootTokenProfileTestProviderMu.Lock()
			defer bootTokenProfileTestProviderMu.Unlock()
			if bootTokenProfileTestProviderCurrent == nil {
				return nil, errors.New("boot token profile test provider is not installed")
			}
			return bootTokenProfileTestProviderCurrent, nil
		})
	})
}

func setBootTokenProfileTestProvider(t *testing.T, p *testutil.MockProvider) {
	t.Helper()
	bootTokenProfileTestProviderMu.Lock()
	bootTokenProfileTestProviderCurrent = p
	bootTokenProfileTestProviderMu.Unlock()
	t.Cleanup(func() {
		bootTokenProfileTestProviderMu.Lock()
		if bootTokenProfileTestProviderCurrent == p {
			bootTokenProfileTestProviderCurrent = nil
		}
		bootTokenProfileTestProviderMu.Unlock()
	})
}

func requestHasTool(req provider.Request, name string) bool {
	for _, schema := range req.Tools {
		if schema.Name == name {
			return true
		}
	}
	return false
}

func requestToolSchemaContains(req provider.Request, name, want string) bool {
	for _, schema := range req.Tools {
		if schema.Name == name {
			return strings.Contains(string(schema.Parameters), want)
		}
	}
	return false
}

func requestHasToolPrefix(req provider.Request, prefix string) bool {
	for _, schema := range req.Tools {
		if strings.HasPrefix(schema.Name, prefix) {
			return true
		}
	}
	return false
}

func toolSchemaNames(tools []provider.ToolSchema) []string {
	names := make([]string, 0, len(tools))
	for _, schema := range tools {
		names = append(names, schema.Name)
	}
	return names
}

func assertToolOrder(t *testing.T, tools []provider.ToolSchema, want []string) {
	t.Helper()
	names := toolSchemaNames(tools)
	next := 0
	for _, name := range names {
		if next < len(want) && name == want[next] {
			next++
		}
	}
	if next != len(want) {
		t.Fatalf("tool order changed; provider-visible tool schema order affects prompt-cache shape.\nwant subsequence: %v\n got: %v", want, names)
	}
}

func firstTokenProfileRequest(t *testing.T, tokenMode string) provider.Request {
	t.Helper()
	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-profile", testutil.Turn{Text: "done"})
	setBootTokenProfileTestProvider(t, prov)

	opts := Options{Sink: event.Discard}
	if tokenMode != "" {
		opts.TokenMode = tokenMode
	}
	ctrl, err := Build(context.Background(), opts)
	if err != nil {
		t.Fatalf("Build(%q): %v", tokenMode, err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "capture request prefix"); err != nil {
		t.Fatalf("Run(%q): %v", tokenMode, err)
	}
	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests(%q) = %d, want 1", tokenMode, len(reqs))
	}
	return reqs[0]
}

func firstActorRequest(t *testing.T, workspace string, actor userctx.ActorContext) provider.Request {
	t.Helper()
	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("actor-invariance", testutil.Turn{Text: "done"})
	setBootTokenProfileTestProvider(t, prov)
	ctrl, err := Build(context.Background(), Options{
		Sink:          event.Discard,
		WorkspaceRoot: workspace,
		Actor:         actor,
	})
	if err != nil {
		t.Fatalf("Build(%+v): %v", actor, err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "capture provider-visible request"); err != nil {
		t.Fatalf("Run(%+v): %v", actor, err)
	}
	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests(%+v) = %d, want 1", actor, len(reqs))
	}
	return reqs[0]
}

func bootTaskTreePlannerJSON() string {
	return `{
  "id": "tree-boot-test",
  "goal": "build a task tree",
  "root_ids": ["plan"],
  "status": "pending",
  "nodes": {
    "plan": {
      "id": "plan",
      "title": "Plan task tree",
      "status": "pending",
      "executor": {"kind": "main"},
      "expected_output": "A stored structured task tree"
    }
  }
}`
}

func TestBuildSubagentSkillFailedContinuationPersistsTranscript(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootSubagentTestProvider()
	prov := &bootSubagentTestProvider{}
	setBootSubagentTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-subagent-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	sessionPath := agent.NewSessionPath(ctrl.SessionDir(), ctrl.Label())
	ctrl.SetSessionPath(sessionPath)

	if err := ctrl.Run(context.Background(), "first review"); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	ref := subagentRefFromHistory(t, ctrl.History())
	prov.setContinueRef(ref)

	if err := ctrl.Run(context.Background(), "continue review"); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	store := agent.NewSubagentStore(filepath.Join(config.SessionDir(), "subagents"))
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.Status != agent.SubagentFailed {
		t.Fatalf("status = %q, want failed", meta.Status)
	}
	if meta.ParentSession != agent.BranchID(sessionPath) {
		t.Fatalf("parent session = %q, want %q", meta.ParentSession, agent.BranchID(sessionPath))
	}
	sess, err := agent.LoadSession(filepath.Join(config.SessionDir(), "subagents", ref+".jsonl"))
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	msgs := sess.Snapshot()
	if len(msgs) != 4 || msgs[1].Content != "first skill task" || msgs[2].Content != "first skill answer" || msgs[3].Content != "second skill task" {
		t.Fatalf("failed skill transcript = %+v, want first task/answer plus second task", msgs)
	}
}

func TestBuildSubagentStoreHonorsSessionDirOverride(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootSubagentTestProvider()
	prov := &bootSubagentTestProvider{}
	setBootSubagentTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-subagent-test"
model = "x"
`)

	sessionDir := filepath.Join(t.TempDir(), "desktop-workspace-sessions")
	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, SessionDir: sessionDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	sessionPath := agent.NewSessionPath(ctrl.SessionDir(), ctrl.Label())
	ctrl.SetSessionPath(sessionPath)

	if err := ctrl.Run(context.Background(), "first review"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	ref := subagentRefFromHistory(t, ctrl.History())

	overrideStore := agent.NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	meta, err := overrideStore.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta from override dir: %v", err)
	}
	if meta.ParentSession != agent.BranchID(sessionPath) {
		t.Fatalf("parent session = %q, want %q", meta.ParentSession, agent.BranchID(sessionPath))
	}
	if _, err := os.Stat(filepath.Join(config.SessionDir(), "subagents", ref+".meta.json")); !os.IsNotExist(err) {
		t.Fatalf("subagent metadata should not be written to global session dir, stat err = %v", err)
	}
}

func TestBuildSubagentSkillUsesLiveReasoningLanguage(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootSubagentTestProvider()
	prov := &bootSubagentTestProvider{}
	setBootSubagentTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"
reasoning_language = "zh"

[[providers]]
name = "test-model"
kind = "boot-subagent-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetReasoningLanguage("auto")

	if err := ctrl.Run(context.Background(), "first review"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.requestsSnapshot()
	if len(reqs) < 2 {
		t.Fatalf("provider requests = %d, want parent request plus skill subagent request", len(reqs))
	}
	if got := bootLastUser(reqs[1]); strings.Contains(got, "<reasoning-language>") {
		t.Fatalf("skill subagent kept stale boot-time reasoning language after live auto update: %q", got)
	}
	if got := bootLastUser(reqs[1]); got != "first skill task" {
		t.Fatalf("skill subagent user prompt = %q, want first skill task", got)
	}
}

func TestBuildUsesConfiguredLanguageForResponsePreference(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootSubagentTestProvider()
	prov := &bootSubagentTestProvider{}
	setBootSubagentTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"
language = "en"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-subagent-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.Run(context.Background(), "first review"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.requestsSnapshot()
	if len(reqs) == 0 {
		t.Fatal("provider requests = 0, want at least one")
	}
	if got := bootLastUser(reqs[0]); !strings.Contains(got, "<response-language>") || !strings.Contains(got, "use English") {
		t.Fatalf("first user turn = %q, want English response preference", got)
	}
}

func TestBuildSubagentSkillGetsForegroundOnlyBash(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootSubagentTestProvider()
	prov := &bootSubagentTestProvider{}
	setBootSubagentTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-subagent-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetSessionPath(agent.NewSessionPath(ctrl.SessionDir(), ctrl.Label()))

	if err := ctrl.Run(context.Background(), "first review"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.requestsSnapshot()
	if len(reqs) < 2 {
		t.Fatalf("provider requests = %d, want parent request plus skill subagent request", len(reqs))
	}
	parentReq, subReq := reqs[0], reqs[1]
	for _, want := range []string{"task", "bash", "wait", "bash_output", "kill_shell"} {
		if !requestHasTool(parentReq, want) {
			t.Fatalf("parent request missing %q; tools=%v", want, toolSchemaNames(parentReq.Tools))
		}
	}
	if !requestToolSchemaContains(parentReq, "bash", "run_in_background") {
		t.Fatalf("parent bash schema should include run_in_background")
	}
	for _, hidden := range []string{"task", "run_skill", "read_only_skill", "read_skill", "install_skill", "install_source", "explore", "research", "review", "security_review", "wait", "bash_output", "kill_shell"} {
		if requestHasTool(subReq, hidden) {
			t.Fatalf("skill subagent request should hide %q; tools=%v", hidden, toolSchemaNames(subReq.Tools))
		}
	}
	if !requestHasTool(subReq, "bash") {
		t.Fatalf("skill subagent request should keep foreground bash; tools=%v", toolSchemaNames(subReq.Tools))
	}
	if requestToolSchemaContains(subReq, "bash", "run_in_background") {
		t.Fatalf("skill subagent bash schema should not include run_in_background")
	}
}

func TestBuildTaskSubagentMetadataInheritsActorContext(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerHeadlessTaskTestProvider()
	prov := &headlessTaskTestProvider{}
	setHeadlessTaskTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"
planner_model = ""
auto_plan = "off"

[[providers]]
name = "test-model"
kind = "boot-headless-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{
		Sink: event.Discard,
		Actor: userctx.ActorContext{
			ActorID:       "parent-actor",
			ProfileID:     "profile-a",
			AuthMethod:    "oauth",
			WorkspaceRoot: filepath.Join(dir, "stale"),
			RemoteIdentity: &userctx.RemoteIdentity{
				Provider: "lark",
				UserID:   "remote-user",
			},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetSessionPath(agent.NewSessionPath(ctrl.SessionDir(), ctrl.Label()))

	if err := ctrl.Run(context.Background(), "use a task subagent"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	ref := subagentRefFromHistory(t, ctrl.History())
	store := agent.NewSubagentStore(filepath.Join(ctrl.SessionDir(), "subagents"))
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.ActorID != "parent-actor" || meta.ProfileID != "profile-a" || meta.AuthMethod != "oauth" {
		t.Fatalf("actor metadata = %+v, want inherited parent actor/profile/auth", meta)
	}
	if filepath.Clean(meta.ActorWorkspaceRoot) != filepath.Clean(dir) {
		t.Fatalf("actor workspace root = %q, want resolved root %q", meta.ActorWorkspaceRoot, dir)
	}
	if meta.ParentActorID != "parent-actor" {
		t.Fatalf("parent actor id = %q, want parent-actor", meta.ParentActorID)
	}
	if meta.RemoteIdentity == nil || meta.RemoteIdentity.UserID != "remote-user" {
		t.Fatalf("remote identity = %+v, want remote user", meta.RemoteIdentity)
	}
	if meta.ActorID == meta.RemoteIdentity.UserID {
		t.Fatalf("remote identity user %q was promoted to ActorID", meta.RemoteIdentity.UserID)
	}
}

const bootSubagentTestProviderKind = "boot-subagent-test"

var (
	bootSubagentTestProviderOnce    sync.Once
	bootSubagentTestProviderCurrent *bootSubagentTestProvider
	bootSubagentTestProviderMu      sync.Mutex
)

func registerBootSubagentTestProvider() {
	bootSubagentTestProviderOnce.Do(func() {
		provider.Register(bootSubagentTestProviderKind, func(provider.Config) (provider.Provider, error) {
			bootSubagentTestProviderMu.Lock()
			defer bootSubagentTestProviderMu.Unlock()
			if bootSubagentTestProviderCurrent == nil {
				return nil, errors.New("boot subagent test provider is not installed")
			}
			return bootSubagentTestProviderCurrent, nil
		})
	})
}

func setBootSubagentTestProvider(t *testing.T, p *bootSubagentTestProvider) {
	t.Helper()
	bootSubagentTestProviderMu.Lock()
	bootSubagentTestProviderCurrent = p
	bootSubagentTestProviderMu.Unlock()
	t.Cleanup(func() {
		bootSubagentTestProviderMu.Lock()
		if bootSubagentTestProviderCurrent == p {
			bootSubagentTestProviderCurrent = nil
		}
		bootSubagentTestProviderMu.Unlock()
	})
}

type bootSubagentTestProvider struct {
	mu          sync.Mutex
	calls       int
	continueRef string
	requests    []provider.Request
}

func (p *bootSubagentTestProvider) Name() string { return "boot-subagent-test" }

func (p *bootSubagentTestProvider) setContinueRef(ref string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.continueRef = ref
}

func (p *bootSubagentTestProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	p.mu.Lock()
	ref := p.continueRef
	p.requests = append(p.requests, req)
	readinessChunks, isReadinessRetry := bootArtifactReadinessFixtureChunks(req.Messages)
	call := p.calls
	if !isReadinessRetry {
		p.calls++
	}
	p.mu.Unlock()

	var chunks []provider.Chunk
	if isReadinessRetry {
		chunks = readinessChunks
	} else {
		switch call {
		case 0:
			chunks = []provider.Chunk{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "review-1", Name: "review", Arguments: `{"task":"first skill task"}`}}}
		case 1:
			chunks = []provider.Chunk{{Type: provider.ChunkText, Text: "first skill answer"}, {Type: provider.ChunkDone}}
		case 2:
			chunks = []provider.Chunk{{Type: provider.ChunkText, Text: "parent first done"}, {Type: provider.ChunkDone}}
		case 3:
			args, _ := json.Marshal(map[string]string{"task": "second skill task", "continue_from": ref})
			chunks = []provider.Chunk{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "review-2", Name: "review", Arguments: string(args)}}}
		case 4:
			chunks = []provider.Chunk{{Type: provider.ChunkError, Err: errors.New("subagent skill failed")}}
		case 5:
			chunks = []provider.Chunk{{Type: provider.ChunkText, Text: "parent second done"}, {Type: provider.ChunkDone}}
		default:
			chunks = []provider.Chunk{{Type: provider.ChunkError, Err: fmt.Errorf("unexpected provider call %d", call)}}
		}
	}
	ch := make(chan provider.Chunk, len(chunks))
	for _, chunk := range chunks {
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

func (p *bootSubagentTestProvider) requestsSnapshot() []provider.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]provider.Request, len(p.requests))
	copy(out, p.requests)
	return out
}

func bootLastUser(req provider.Request) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == provider.RoleUser {
			return req.Messages[i].Content
		}
	}
	return ""
}

func bootArtifactReadinessFixtureChunks(messages []provider.Message) ([]provider.Chunk, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role == provider.RoleTool && message.Name == "deliver_files" {
			for j := i - 1; j >= 0; j-- {
				if messages[j].Role == provider.RoleAssistant && messages[j].Content != "" {
					return []provider.Chunk{{Type: provider.ChunkText, Text: messages[j].Content}, {Type: provider.ChunkDone}}, true
				}
			}
			return []provider.Chunk{{Type: provider.ChunkText, Text: "parent done"}, {Type: provider.ChunkDone}}, true
		}
		if message.Role != provider.RoleUser {
			continue
		}
		if !strings.Contains(message.Content, "Host final-answer readiness check failed") {
			return nil, false
		}
		return []provider.Chunk{{
			Type: provider.ChunkToolCall,
			ToolCall: &provider.ToolCall{
				ID:        "deliver-files-empty",
				Name:      "deliver_files",
				Arguments: `{"paths":[]}`,
			},
		}}, true
	}
	return nil, false
}

func bootOpenAITestMessages(req map[string]any) []provider.Message {
	raw, _ := json.Marshal(req["messages"])
	var messages []provider.Message
	_ = json.Unmarshal(raw, &messages)
	return messages
}

func bootWriteOpenAITestChunks(w http.ResponseWriter, chunks []provider.Chunk) {
	if call := chunks[0].ToolCall; call != nil {
		fmt.Fprintf(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":%q,"type":"function","function":{"name":%q,"arguments":%q}}]}}]}`+"\n\n", call.ID, call.Name, call.Arguments)
		fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
	} else {
		fmt.Fprintf(w, `data: {"choices":[{"delta":{"content":%q}}]}`+"\n\n", chunks[0].Text)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func TestBootArtifactReadinessRetryChunks(t *testing.T) {
	chunks, ok := bootArtifactReadinessFixtureChunks([]provider.Message{{
		Role:    provider.RoleUser,
		Content: "Host final-answer readiness check failed. Publish a manifest.",
	}})
	if !ok || len(chunks) != 1 || chunks[0].Type != provider.ChunkToolCall || chunks[0].ToolCall == nil {
		t.Fatalf("readiness retry chunks = %+v, ok = %v; want exactly one tool call", chunks, ok)
	}
	if got := chunks[0].ToolCall; got.Name != "deliver_files" || got.Arguments != `{"paths":[]}` {
		t.Fatalf("readiness retry tool call = %+v, want deliver_files with an empty paths array", got)
	}
}

func subagentRefFromHistory(t *testing.T, msgs []provider.Message) string {
	t.Helper()
	for _, msg := range msgs {
		if msg.Role != provider.RoleTool {
			continue
		}
		for _, line := range strings.Split(msg.Content, "\n") {
			if strings.HasPrefix(line, "Subagent reference: ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "Subagent reference: "))
			}
		}
	}
	t.Fatalf("no subagent reference in history: %+v", msgs)
	return ""
}

// TestBuildHeadlessRunRunsTaskSubagentWithoutSessionPath reproduces headless
// `kwu run`: a controller built via Build with NO SetSessionPath (exactly
// what internal/cli.runAgent does) must still be able to run a `task` sub-agent.
// Before the ephemeral fallback this failed with "parent session is required".
func TestBuildHeadlessRunRunsTaskSubagentWithoutSessionPath(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerHeadlessTaskTestProvider()
	prov := &headlessTaskTestProvider{}
	setHeadlessTaskTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-headless-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	// Deliberately NOT calling SetSessionPath — this is the headless run path.
	if err := ctrl.Run(context.Background(), "use a task subagent"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := ctrl.SessionPath(); got != "" {
		t.Fatalf("headless run should keep an empty session path, got %q", got)
	}

	var toolContent string
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool {
			toolContent += "\n" + msg.Content
		}
	}
	if strings.Contains(toolContent, "parent session is required") {
		t.Fatalf("task subagent failed in headless run mode: %s", toolContent)
	}
	if !strings.Contains(toolContent, "subagent answer") {
		t.Fatalf("task tool result = %q, want sub-agent answer", toolContent)
	}
	if strings.Contains(toolContent, "Subagent reference") {
		t.Fatalf("ephemeral headless run should not persist a transcript reference: %s", toolContent)
	}
}

const headlessTaskTestProviderKind = "boot-headless-test"

var (
	headlessTaskTestProviderOnce    sync.Once
	headlessTaskTestProviderCurrent *headlessTaskTestProvider
	headlessTaskTestProviderMu      sync.Mutex
)

func registerHeadlessTaskTestProvider() {
	headlessTaskTestProviderOnce.Do(func() {
		provider.Register(headlessTaskTestProviderKind, func(provider.Config) (provider.Provider, error) {
			headlessTaskTestProviderMu.Lock()
			defer headlessTaskTestProviderMu.Unlock()
			if headlessTaskTestProviderCurrent == nil {
				return nil, errors.New("headless task test provider is not installed")
			}
			return headlessTaskTestProviderCurrent, nil
		})
	})
}

func setHeadlessTaskTestProvider(t *testing.T, p *headlessTaskTestProvider) {
	t.Helper()
	headlessTaskTestProviderMu.Lock()
	headlessTaskTestProviderCurrent = p
	headlessTaskTestProviderMu.Unlock()
	t.Cleanup(func() {
		headlessTaskTestProviderMu.Lock()
		if headlessTaskTestProviderCurrent == p {
			headlessTaskTestProviderCurrent = nil
		}
		headlessTaskTestProviderMu.Unlock()
	})
}

type headlessTaskTestProvider struct {
	mu    sync.Mutex
	calls int
}

func (p *headlessTaskTestProvider) Name() string { return "boot-headless-test" }

func (p *headlessTaskTestProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	p.mu.Lock()
	call := p.calls
	readinessChunks, isReadinessRetry := bootArtifactReadinessFixtureChunks(req.Messages)
	if !isReadinessRetry {
		p.calls++
	}
	p.mu.Unlock()

	var chunks []provider.Chunk
	if isReadinessRetry {
		chunks = readinessChunks
	} else {
		switch call {
		case 0:
			chunks = []provider.Chunk{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "task-1", Name: "task", Arguments: `{"prompt":"find callers"}`}}}
		case 1:
			chunks = []provider.Chunk{{Type: provider.ChunkText, Text: "subagent answer"}, {Type: provider.ChunkDone}}
		default:
			latestTool := ""
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if req.Messages[i].Role == provider.RoleTool {
					latestTool = req.Messages[i].Content
					break
				}
			}
			if strings.Contains(latestTool, "Turn status: running") {
				ref := ""
				for _, line := range strings.Split(latestTool, "\n") {
					if value, ok := strings.CutPrefix(line, "Subagent reference: "); ok {
						ref = strings.TrimSpace(value)
						break
					}
				}
				chunks = []provider.Chunk{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
					ID: "wait-task-1", Name: "wait_task", Arguments: `{"subagent_ref":"` + ref + `","timeout_seconds":5}`,
				}}}
			} else {
				chunks = []provider.Chunk{{Type: provider.ChunkText, Text: "parent done"}, {Type: provider.ChunkDone}}
			}
		}
	}
	ch := make(chan provider.Chunk, len(chunks))
	for _, chunk := range chunks {
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

func TestNewProviderAppliesConfiguredDefaultEffort(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name:             "custom",
		Kind:             "openai",
		BaseURL:          srv.URL,
		Model:            "m",
		SupportedEfforts: []string{"low", "medium", "high"},
		DefaultEffort:    "MEDIUM",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if got := gotReq["reasoning_effort"]; got != "medium" {
		t.Fatalf("reasoning_effort = %#v, want medium from default_effort", got)
	}
}

func TestNewProviderAppliesModelReasoningProtocol(t *testing.T) {
	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, err := NewProvider(&config.ProviderEntry{
		Name:    "deepseek-proxy",
		Kind:    "openai",
		BaseURL: srv.URL,
		Model:   "deepseek-v4-flash",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for chunk := range ch {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if got := gotReq["reasoning_effort"]; got != "high" {
		t.Fatalf("reasoning_effort = %#v, want high from DeepSeek model capability", got)
	}
	thinking, ok := gotReq["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("thinking = %#v, want enabled", gotReq["thinking"])
	}
}

func TestConfiguredSubagentEffortControlsChildRequestWithoutSchemaField(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	var mu sync.Mutex
	var requests []map[string]any
	businessCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gotReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, gotReq)
		readinessChunks, isReadinessRetry := bootArtifactReadinessFixtureChunks(bootOpenAITestMessages(gotReq))
		call := businessCalls
		if !isReadinessRetry {
			businessCalls++
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		if isReadinessRetry {
			bootWriteOpenAITestChunks(w, readinessChunks)
			return
		}
		switch call {
		case 0:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_task","type":"function","function":{"name":"task","arguments":"{\"prompt\":\"debug this\",\"effort\":\"low\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		case 1:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"child done"}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"parent done"}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
	}))
	defer srv.Close()

	writeFile(t, dir, "kwu.toml", fmt.Sprintf(`
default_model = "local-openai"

[agent]
system_prompt = "BASE"
subagent_effort = "high"

[[providers]]
name = "local-openai"
kind = "openai"
base_url = %q
model = "custom-reasoning-model"
`, srv.URL))

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.Run(context.Background(), "delegate with high effort"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) < 2 {
		t.Fatalf("captured %d provider requests, want at least parent and subagent", len(requests))
	}
	assertSubagentSchemasHideEffort(t, requests[0])
	if got := requests[1]["reasoning_effort"]; got != "high" {
		t.Fatalf("subagent reasoning_effort = %#v, want configured high despite legacy tool argument; request=%#v", got, requests[1])
	}
}

func TestLegacySubagentEffortDoesNotOverrideAutomaticProfile(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	var mu sync.Mutex
	var requests []map[string]any
	businessCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gotReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, gotReq)
		readinessChunks, isReadinessRetry := bootArtifactReadinessFixtureChunks(bootOpenAITestMessages(gotReq))
		call := businessCalls
		if !isReadinessRetry {
			businessCalls++
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		if isReadinessRetry {
			bootWriteOpenAITestChunks(w, readinessChunks)
			return
		}
		switch call {
		case 0:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_task","type":"function","function":{"name":"task","arguments":"{\"prompt\":\"debug this\",\"effort\":\"high\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		case 1:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"child done"}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"parent done"}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
	}))
	defer srv.Close()

	writeFile(t, dir, "kwu.toml", fmt.Sprintf(`
default_model = "local-openai"

[agent]
system_prompt = "BASE"

[[providers]]
name = "local-openai"
kind = "openai"
base_url = %q
model = "custom-reasoning-model"
`, srv.URL))

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.Run(context.Background(), "delegate with auto effort"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) < 2 {
		t.Fatalf("captured %d provider requests, want at least parent and subagent", len(requests))
	}
	assertSubagentSchemasHideEffort(t, requests[0])
	if got, ok := requests[1]["reasoning_effort"]; ok {
		t.Fatalf("legacy effort argument should not override automatic profile, got %#v; request=%#v", got, requests[1])
	}
}

func TestLegacySubagentEffortIsIgnoredWhenReasoningProtocolIsNone(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	var mu sync.Mutex
	var requests []map[string]any
	businessCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gotReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, gotReq)
		readinessChunks, isReadinessRetry := bootArtifactReadinessFixtureChunks(bootOpenAITestMessages(gotReq))
		call := businessCalls
		if !isReadinessRetry {
			businessCalls++
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		if isReadinessRetry {
			bootWriteOpenAITestChunks(w, readinessChunks)
			return
		}
		switch call {
		case 0:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_task","type":"function","function":{"name":"task","arguments":"{\"prompt\":\"debug this\",\"effort\":\"high\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"parent done"}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
	}))
	defer srv.Close()

	writeFile(t, dir, "kwu.toml", fmt.Sprintf(`
default_model = "local-openai"

[agent]
system_prompt = "BASE"

[[providers]]
name = "local-openai"
kind = "openai"
base_url = %q
model = "plain-chat-model"
reasoning_protocol = "none"
`, srv.URL))

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.Run(context.Background(), "delegate with disabled effort"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	if len(requests) < 2 {
		mu.Unlock()
		t.Fatalf("captured %d provider requests, want at least parent and subagent", len(requests))
	}
	first := requests[0]
	for _, req := range requests[1:] {
		if got, ok := req["reasoning_effort"]; ok {
			mu.Unlock()
			t.Fatalf("legacy effort argument should be ignored when reasoning is disabled, got %#v in request %#v", got, req)
		}
	}
	mu.Unlock()
	assertSubagentSchemasHideEffort(t, first)
}

func TestLegacySubagentEffortDoesNotAffectModelOverrideProfile(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	var mu sync.Mutex
	var requests []map[string]any
	businessCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var gotReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, gotReq)
		readinessChunks, isReadinessRetry := bootArtifactReadinessFixtureChunks(bootOpenAITestMessages(gotReq))
		call := businessCalls
		if !isReadinessRetry {
			businessCalls++
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		if isReadinessRetry {
			bootWriteOpenAITestChunks(w, readinessChunks)
			return
		}
		switch call {
		case 0:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_task","type":"function","function":{"name":"task","arguments":"{\"prompt\":\"debug this\",\"model\":\"plain-openai\",\"effort\":\"high\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"parent done"}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
	}))
	defer srv.Close()

	writeFile(t, dir, "kwu.toml", fmt.Sprintf(`
default_model = "local-openai"

[agent]
system_prompt = "BASE"

[[providers]]
name = "local-openai"
kind = "openai"
base_url = %q
model = "custom-reasoning-model"

[[providers]]
name = "plain-openai"
kind = "openai"
base_url = %q
model = "plain-chat-model"
reasoning_protocol = "none"
`, srv.URL, srv.URL))

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if err := ctrl.Run(context.Background(), "delegate with override effort"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	if len(requests) < 2 {
		mu.Unlock()
		t.Fatalf("captured %d provider requests, want at least parent and subagent", len(requests))
	}
	first := requests[0]
	for _, req := range requests[1:] {
		if got, ok := req["reasoning_effort"]; ok {
			mu.Unlock()
			t.Fatalf("unsupported override provider should not receive reasoning_effort, got %#v in request %#v", got, req)
		}
	}
	child := requests[1]
	mu.Unlock()

	assertSubagentSchemasHideEffort(t, first)
	if got := child["model"]; got != "plain-chat-model" {
		t.Fatalf("subagent model = %#v, want model override plain-chat-model; request=%#v", got, child)
	}
}

func assertSubagentSchemasHideEffort(t *testing.T, req map[string]any) {
	t.Helper()
	for _, name := range []string{"task", "read_only_task", "parallel_tasks"} {
		params := openAIToolParametersJSON(t, req, name)
		if strings.Contains(params, `"effort"`) {
			t.Fatalf("%s schema should not expose model-controlled effort, parameters=%s", name, params)
		}
	}
}

func openAIToolParametersJSON(t *testing.T, req map[string]any, name string) string {
	t.Helper()
	tools, ok := req["tools"].([]any)
	if !ok {
		t.Fatalf("request has no OpenAI tools array: %#v", req["tools"])
	}
	for _, rawTool := range tools {
		toolMap, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		fn, ok := toolMap["function"].(map[string]any)
		if !ok || fn["name"] != name {
			continue
		}
		params, err := json.Marshal(fn["parameters"])
		if err != nil {
			t.Fatalf("marshal %s parameters: %v", name, err)
		}
		return string(params)
	}
	t.Fatalf("missing OpenAI tool %q in request", name)
	return ""
}

func TestBuildHonorsSessionDirOverride(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	sessionDir := filepath.Join(t.TempDir(), "desktop-workspace-sessions")
	ctrl, err := Build(context.Background(), Options{SessionDir: sessionDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	if got := ctrl.SessionDir(); got != sessionDir {
		t.Fatalf("SessionDir() = %q, want override %q", got, sessionDir)
	}
}

// TestBuildDiscoversSkills proves the skill wiring end-to-end: a project skill
// is discovered at boot, surfaced via Controller.Skills(), and its name folds
// into the cache-stable system prompt's "# Skills" index alongside a built-in.
func TestBuildDiscoversSkills(t *testing.T) {
	dir := robustTempDir(t)
	isolateConfigHome(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)
	writeFile(t, dir, ".kwu/skills/projskill.md", "---\ndescription: a project skill\n---\nplaybook")

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	var hasProj, hasBuiltin bool
	for _, s := range ctrl.Skills() {
		switch s.Name {
		case "projskill":
			hasProj = true
		case "explore":
			hasBuiltin = true
		}
	}
	if !hasProj || !hasBuiltin {
		t.Fatalf("Skills() should include the project skill and a built-in; got %v", ctrl.Skills())
	}

	sys := systemMessage(ctrl.History())
	if !strings.Contains(sys, "# Skills") {
		t.Fatalf("skills index missing from system prompt:\n%s", sys)
	}
	if !strings.Contains(sys, "projskill") || !strings.Contains(sys, "explore") {
		t.Fatalf("skill names missing from index:\n%s", sys)
	}
}

func TestBuildTokenFullMatchesDefaultRequestPrefix(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)
	writeFile(t, dir, ".kwu/skills/projskill.md", "---\ndescription: a project skill\n---\nplaybook")

	defaultReq := firstTokenProfileRequest(t, "")
	fullReq := firstTokenProfileRequest(t, TokenModeFull)

	if got, want := systemMessage(defaultReq.Messages), systemMessage(fullReq.Messages); got != want {
		t.Fatalf("explicit full mode changed the system prompt\n--- default ---\n%s\n--- full ---\n%s", got, want)
	}
	if strings.Contains(systemMessage(fullReq.Messages), tokenEconomyPrompt) {
		t.Fatalf("full mode system prompt should not include token economy prompt:\n%s", systemMessage(fullReq.Messages))
	}
	if !strings.Contains(systemMessage(fullReq.Messages), "# Skills") || !strings.Contains(systemMessage(fullReq.Messages), "projskill") {
		t.Fatalf("full mode should preserve the skills index in the system prompt:\n%s", systemMessage(fullReq.Messages))
	}
	if got, want := toolSchemaNames(fullReq.Tools), toolSchemaNames(defaultReq.Tools); !reflect.DeepEqual(got, want) {
		t.Fatalf("explicit full mode changed tool schema order\nfull=%v\ndefault=%v", got, want)
	}
	if !reflect.DeepEqual(fullReq.Tools, defaultReq.Tools) {
		t.Fatalf("explicit full mode changed provider-visible tool schemas; names=%v", toolSchemaNames(fullReq.Tools))
	}
	if requestHasTool(fullReq, "connect_tool_source") {
		t.Fatalf("full mode should not expose economy connector; tools=%v", toolSchemaNames(fullReq.Tools))
	}
}

func TestBuildTokenEconomyStartsWithLeanToolSurface(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy", testutil.Turn{Text: "done"})
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"

[[plugins]]
name = "mockmcp"
command = "kwu-missing-mockmcp"
`)
	writeFile(t, dir, ".kwu/skills/projskill.md", "---\ndescription: a project skill\n---\nplaybook")

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "use the lean surface"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	req := reqs[0]
	wantTools := []string{
		"ask",
		"bash",
		"bash_output",
		"code_index",
		"complete_step",
		"connect_tool_source",
		"current_datetime",
		"deliver_files",
		"edit_file",
		"forget",
		"glob",
		"grep",
		"history",
		"kill_shell",
		"list_sessions",
		"ls",
		"memory",
		"move_file",
		"multi_edit",
		"read_file",
		"read_session",
		"remember",
		"slash_command",
		"todo_write",
		"wait",
		"write_file",
	}
	if got := toolSchemaNames(req.Tools); !reflect.DeepEqual(got, wantTools) {
		t.Fatalf("economy first request tool order changed\ngot  %v\nwant %v", got, wantTools)
	}
	for _, want := range []string{"connect_tool_source", "current_datetime", "read_file", "grep", "edit_file", "bash", "slash_command", "ask", "deliver_files"} {
		if !requestHasTool(req, want) {
			t.Fatalf("economy first request missing tool %q; tools=%v", want, toolSchemaNames(req.Tools))
		}
	}
	for _, forbidden := range []string{
		"web_fetch", "task", "read_only_task", "read_only_skill", "run_skill", "read_skill", "install_skill", "install_source",
		"explore", "research", "review", "security_review",
		"lsp_definition", "lsp_references", "lsp_hover", "lsp_diagnostics",
		"verify_artifact",
	} {
		if requestHasTool(req, forbidden) {
			t.Fatalf("economy first request should hide %q; tools=%v", forbidden, toolSchemaNames(req.Tools))
		}
	}
	if requestHasToolPrefix(req, "mcp__mockmcp") {
		t.Fatalf("economy first request should not expose MCP placeholders; tools=%v", toolSchemaNames(req.Tools))
	}
	sys := systemMessage(req.Messages)
	if !strings.Contains(sys, tokenEconomyPrompt) {
		t.Fatalf("token economy prompt missing from system message:\n%s", sys)
	}
	if strings.Contains(sys, "# Skills") || strings.Contains(sys, "projskill") {
		t.Fatalf("skills index should not be in economy system prompt:\n%s", sys)
	}
}

func TestBuildTokenEconomyConnectsWebFetchOnDemand(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"web_fetch"}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "fetch later"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if requestHasTool(reqs[0], "web_fetch") {
		t.Fatalf("first request should hide web_fetch; tools=%v", toolSchemaNames(reqs[0].Tools))
	}
	if !requestHasTool(reqs[1], "web_fetch") {
		t.Fatalf("second request should expose web_fetch after connect_tool_source; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
}

func TestBuildTokenEconomyPlanModeCanConnectWebFetch(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"web_fetch"}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetPlanMode(true)
	if err := ctrl.Run(context.Background(), "fetch later while planning"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if !requestHasTool(reqs[1], "web_fetch") {
		t.Fatalf("second request should expose web_fetch in plan economy mode; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" && strings.Contains(msg.Content, "blocked:") {
			t.Fatalf("connect_tool_source should not be blocked in plan mode, got:\n%s", msg.Content)
		}
	}
}

func TestBuildTokenEconomyPlanModeCanConnectReadOnlyTask(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"read_only_subagent"}`},
		}},
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "readonly-1", Name: "read_only_task", Arguments: `{"prompt":"inspect safely"}`},
		}},
		testutil.Turn{Text: "read-only findings"},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetPlanMode(true)
	if err := ctrl.Run(context.Background(), "connect read-only subagent while planning"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 4 {
		t.Fatalf("requests = %d, want 4", len(reqs))
	}
	if !requestHasTool(reqs[1], "read_only_task") {
		t.Fatalf("second request should expose read_only_task in plan economy mode; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
	if requestHasTool(reqs[1], "task") {
		t.Fatalf("read_only_task source should not expose writer-capable task; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
	subReq := reqs[2]
	if !requestHasTool(subReq, "bash") || !requestHasTool(subReq, "read_file") {
		t.Fatalf("read_only_task child request should keep read-only research tools; tools=%v", toolSchemaNames(subReq.Tools))
	}
	if requestToolSchemaContains(subReq, "bash", "run_in_background") {
		t.Fatalf("read_only_task child bash schema should not advertise run_in_background")
	}
	for _, forbidden := range []string{
		"connect_tool_source", "task", "read_only_task", "parallel_tasks",
		"install_source", "run_skill", "read_only_skill", "read_skill", "install_skill", "remember", "forget",
		"write_file", "edit_file", "multi_edit", "move_file", "complete_step", "deliver_files",
	} {
		if requestHasTool(subReq, forbidden) {
			t.Fatalf("read_only_task child request should hide %q; tools=%v", forbidden, toolSchemaNames(subReq.Tools))
		}
	}
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" && strings.Contains(msg.Content, "blocked:") {
			t.Fatalf("connect_tool_source should not block read_only_task in plan mode, got:\n%s", msg.Content)
		}
	}
}

func TestBuildTokenEconomyPlanModeCanConnectReadOnlySkill(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"read_only_skill"}`},
		}},
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "skill-1", Name: "read_only_skill", Arguments: `{"name":"readonlydig","arguments":"inspect safely"}`},
		}},
		testutil.Turn{Text: "skill findings"},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)
	writeFile(t, dir, ".kwu/skills/readonlydig/SKILL.md", `---
description: read-only dig
runAs: subagent
allowed-tools: read_file, bash, write_file, connect_tool_source, read_only_skill
---
READ ONLY SKILL BODY`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetPlanMode(true)
	if err := ctrl.Run(context.Background(), "connect read-only skill while planning"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 4 {
		t.Fatalf("requests = %d, want 4", len(reqs))
	}
	if !requestHasTool(reqs[1], "read_only_skill") {
		t.Fatalf("second request should expose read_only_skill in plan economy mode; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
	for _, forbidden := range []string{"run_skill", "read_skill", "install_skill", "task", "review", "install_source"} {
		if requestHasTool(reqs[1], forbidden) {
			t.Fatalf("read_only_skill source should not expose %q; tools=%v", forbidden, toolSchemaNames(reqs[1].Tools))
		}
	}
	subReq := reqs[2]
	if !strings.Contains(systemMessage(subReq.Messages), "READ ONLY SKILL BODY") {
		t.Fatalf("read_only_skill child should use the skill body as system prompt:\n%s", systemMessage(subReq.Messages))
	}
	if !requestHasTool(subReq, "bash") || !requestHasTool(subReq, "read_file") {
		t.Fatalf("read_only_skill child request should keep read-only research tools; tools=%v", toolSchemaNames(subReq.Tools))
	}
	if requestToolSchemaContains(subReq, "bash", "run_in_background") {
		t.Fatalf("read_only_skill child bash schema should not advertise run_in_background")
	}
	for _, forbidden := range []string{
		"connect_tool_source", "task", "read_only_task", "read_only_skill", "parallel_tasks",
		"install_source", "run_skill", "read_skill", "install_skill", "remember", "forget",
		"write_file", "edit_file", "multi_edit", "move_file", "complete_step", "deliver_files",
	} {
		if requestHasTool(subReq, forbidden) {
			t.Fatalf("read_only_skill child request should hide %q; tools=%v", forbidden, toolSchemaNames(subReq.Tools))
		}
	}
	var toolOutput string
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" {
			toolOutput += msg.Content
			if strings.Contains(msg.Content, "blocked:") {
				t.Fatalf("connect_tool_source should not block read_only_skill in plan mode, got:\n%s", msg.Content)
			}
		}
	}
	if !strings.Contains(toolOutput, "readonlydig") || !strings.Contains(toolOutput, "# Skills") {
		t.Fatalf("read_only_skill source result should include the skill index, got:\n%s", toolOutput)
	}
}

func TestBuildTokenEconomyPlanModeCanConnectAllowedMCPSource(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"mcp","name":"mockmcp"}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", fmt.Sprintf(`
default_model = "test-model"

[agent]
system_prompt = "BASE"
plan_mode_allowed_tools = ["mcp__mockmcp__echo"]

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"

[[plugins]]
name = "mockmcp"
command = %q
args = ["-test.run=TestHelperProcess", "--"]
env = { GO_WANT_HELPER_PROCESS = "1" }
`, os.Args[0]))

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetPlanMode(true)
	if err := ctrl.Run(context.Background(), "connect allowed mcp while planning"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	reqs := prov.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if !requestHasTool(reqs[1], "mcp__mockmcp__echo") {
		t.Fatalf("second request should expose allowed MCP source in plan economy mode; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" {
			if strings.Contains(msg.Content, "blocked:") {
				t.Fatalf("connect_tool_source should not block allowed MCP in plan mode, got:\n%s", msg.Content)
			}
			if !strings.Contains(msg.Content, `enabled MCP server "mockmcp" tools: mcp__mockmcp__echo`) {
				t.Fatalf("connect_tool_source should report enabled MCP tools, got:\n%s", msg.Content)
			}
		}
	}
}

func TestBuildTokenEconomyPlanModeCanConnectTrustedReadOnlyMCPSource(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"mcp","name":"mockmcp"}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", fmt.Sprintf(`
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"

[[plugins]]
name = "mockmcp"
command = %q
args = ["-test.run=TestHelperProcess", "--"]
env = { GO_WANT_HELPER_PROCESS = "1" }
trusted_read_only_tools = ["echo"]
`, os.Args[0]))

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	ctrl.SetPlanMode(true)
	if err := ctrl.Run(context.Background(), "connect trusted mcp while planning"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	reqs := prov.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if !requestHasTool(reqs[1], "mcp__mockmcp__echo") {
		t.Fatalf("second request should expose trusted MCP source in plan economy mode; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" && strings.Contains(msg.Content, "blocked:") {
			t.Fatalf("connect_tool_source should not block trusted MCP in plan mode, got:\n%s", msg.Content)
		}
	}
}

func TestPlanModeAllowsMCPServerRequiresConcreteToolName(t *testing.T) {
	if planModeAllowsMCPServer([]string{"mcp__mockmcp__"}, "mockmcp") {
		t.Fatal("bare MCP namespace prefix should not allow a server in plan mode")
	}
	if !planModeAllowsMCPServer([]string{"mcp__mockmcp__echo"}, "mockmcp") {
		t.Fatal("concrete MCP tool name should allow its server in plan mode")
	}
}

func TestBuildTokenEconomyPlanModeBlocksSourcesWithPolicy(t *testing.T) {
	tests := []struct {
		source          string
		args            string
		forbiddenTools  []string
		forbiddenPrefix string
	}{
		{
			source:         "task",
			args:           `{"source":"task"}`,
			forbiddenTools: []string{"task"},
		},
		{
			source:         "install_source",
			args:           `{"source":"install_source"}`,
			forbiddenTools: []string{"install_source"},
		},
		{
			source: "skills",
			args:   `{"source":"skills"}`,
			forbiddenTools: []string{
				"run_skill", "read_only_skill", "read_skill", "install_skill",
				"explore", "research", "review", "security_review",
			},
		},
		{
			source:          "mcp",
			args:            `{"source":"mcp","name":"mockmcp"}`,
			forbiddenPrefix: "mcp__mockmcp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			isolateConfigHome(t)
			dir := robustTempDir(t)
			t.Chdir(dir)

			registerBootTokenProfileTestProvider()
			prov := testutil.NewMock("token-economy",
				testutil.Turn{ToolCalls: []provider.ToolCall{
					{ID: "source-1", Name: "connect_tool_source", Arguments: tt.args},
				}},
				testutil.Turn{Text: "done"},
			)
			setBootTokenProfileTestProvider(t, prov)
			writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"

[[plugins]]
name = "mockmcp"
command = "kwu-missing-mockmcp"
`)

			ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			defer ctrl.Close()
			ctrl.SetPlanMode(true)
			if err := ctrl.Run(context.Background(), "connect blocked source while planning"); err != nil {
				t.Fatalf("Run: %v", err)
			}

			reqs := prov.Requests()
			if len(reqs) != 2 {
				t.Fatalf("requests = %d, want 2", len(reqs))
			}
			var toolOutput string
			for _, msg := range ctrl.History() {
				if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" {
					toolOutput += msg.Content
				}
			}
			if strings.TrimSpace(toolOutput) == "" {
				t.Fatalf("connect_tool_source(%s) returned empty tool output", tt.source)
			}
			if !strings.Contains(toolOutput, "blocked:") || !strings.Contains(toolOutput, "plan mode") {
				t.Fatalf("connect_tool_source(%s) output = %q, want visible plan-mode block", tt.source, toolOutput)
			}
			for _, forbidden := range tt.forbiddenTools {
				if requestHasTool(reqs[1], forbidden) {
					t.Fatalf("blocked source %s should not expose %q; tools=%v", tt.source, forbidden, toolSchemaNames(reqs[1].Tools))
				}
			}
			if tt.forbiddenPrefix != "" && requestHasToolPrefix(reqs[1], tt.forbiddenPrefix) {
				t.Fatalf("blocked source %s should not expose tools with prefix %q; tools=%v", tt.source, tt.forbiddenPrefix, toolSchemaNames(reqs[1].Tools))
			}
		})
	}
}

func TestBuildWarnsIgnoredPlanModeAllowedTools(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("plan-mode-allowed-tools", testutil.Turn{Text: "done"})
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"
plan_mode_allowed_tools = ["bash", "custom_reader"]

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	var notices []event.Event
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e)
		}
	})

	ctrl, err := Build(context.Background(), Options{Sink: sink})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	for _, notice := range notices {
		if notice.Level == event.LevelWarn && strings.Contains(notice.Text, "plan_mode_allowed_tools") && strings.Contains(notice.Text, "bash") {
			if strings.Contains(notice.Text, "custom_reader") {
				t.Fatalf("warning should name ignored entries only, got %q", notice.Text)
			}
			return
		}
	}
	t.Fatalf("missing ignored plan_mode_allowed_tools warning; got %+v", notices)
}

func TestBuildTokenEconomyWebFetchConnectorHonorsDisabledBuiltin(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"web_fetch"}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[tools]
enabled = ["read_file", "grep"]

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "fetch later"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if requestHasTool(reqs[1], "web_fetch") {
		t.Fatalf("disabled web_fetch should not be exposed after connect_tool_source; tools=%v", toolSchemaNames(reqs[1].Tools))
	}
	var toolOutput string
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" {
			toolOutput += msg.Content
		}
	}
	if !strings.Contains(toolOutput, "web_fetch is disabled by [tools].enabled") {
		t.Fatalf("connector should explain disabled web_fetch, got:\n%s", toolOutput)
	}
}

func TestBuildTokenEconomyConnectsSkillsOnDemand(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	registerBootTokenProfileTestProvider()
	prov := testutil.NewMock("token-economy",
		testutil.Turn{ToolCalls: []provider.ToolCall{
			{ID: "source-1", Name: "connect_tool_source", Arguments: `{"source":"skills"}`},
		}},
		testutil.Turn{Text: "done"},
	)
	setBootTokenProfileTestProvider(t, prov)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)
	writeFile(t, dir, ".kwu/skills/projskill.md", "---\ndescription: a project skill\n---\nplaybook")

	ctrl, err := Build(context.Background(), Options{Sink: event.Discard, TokenMode: TokenModeEconomy})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "use skills later"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	for _, name := range []string{"run_skill", "read_only_skill", "read_skill", "explore"} {
		if requestHasTool(reqs[0], name) {
			t.Fatalf("first request should hide %q; tools=%v", name, toolSchemaNames(reqs[0].Tools))
		}
		if !requestHasTool(reqs[1], name) {
			t.Fatalf("second request should expose %q after connect_tool_source; tools=%v", name, toolSchemaNames(reqs[1].Tools))
		}
	}
	var toolOutput string
	for _, msg := range ctrl.History() {
		if msg.Role == provider.RoleTool && msg.Name == "connect_tool_source" {
			toolOutput += msg.Content
		}
	}
	if !strings.Contains(toolOutput, "projskill") || !strings.Contains(toolOutput, "# Skills") {
		t.Fatalf("skills source result should include the skill index, got:\n%s", toolOutput)
	}
}

func TestAddBuiltinsWithWorkspaceRootKeepsSessionTools(t *testing.T) {
	reg := tool.NewRegistry()
	var stderr bytes.Buffer
	addBuiltins(reg, nil, []string{robustTempDir(t)}, sandbox.Spec{}, 120*time.Second, builtin.SearchSpec{}, &stderr, robustTempDir(t), netclient.ProxySpec{})
	for _, name := range []string{
		"todo_write",
		"complete_step",
		"deliver_files",
		"bash_output",
		"kill_shell",
		"wait",
		"move_file",
		"notebook_edit",
	} {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("workspace builtins missing %q; got %v", name, reg.Names())
		}
	}
	if _, ok := reg.Get("verify_artifact"); ok {
		t.Fatalf("workspace model registry exposed hidden verify_artifact; got %v", reg.Names())
	}
}

func TestAddBuiltinsExplicitListRetainsDeliveryAndHidesVerifyArtifact(t *testing.T) {
	root := robustTempDir(t)

	reg := tool.NewRegistry()
	var stderr bytes.Buffer
	addBuiltins(reg, []string{"read_file", "deliver_files", "unknown_builtin"}, []string{root}, sandbox.Spec{}, 120*time.Second, builtin.SearchSpec{}, &stderr, root, netclient.ProxySpec{})

	if _, ok := reg.Get("verify_artifact"); ok {
		t.Fatalf("workspace model registry exposed hidden verify_artifact; got %v", reg.Names())
	}
	if got := reg.Names(); !reflect.DeepEqual(got, []string{"deliver_files", "read_file"}) {
		t.Fatalf("registry order = %v, want configured model tools without hidden verifier", got)
	}
	if got := stderr.String(); !strings.Contains(got, `warning: unknown built-in tool "unknown_builtin"`) {
		t.Fatalf("unknown built-in warning = %q", got)
	}
}

func TestAddBuiltinsTokenEconomyWorkspaceIncludesDeliveryAndHidesVerifyArtifact(t *testing.T) {
	root := robustTempDir(t)
	reg := tool.NewRegistry()
	var stderr bytes.Buffer
	addBuiltins(reg, tokenEconomyBuiltins(nil), []string{root}, sandbox.Spec{}, 120*time.Second, builtin.SearchSpec{}, &stderr, root, netclient.ProxySpec{})

	if _, ok := reg.Get("deliver_files"); !ok {
		t.Fatalf("economy workspace registry missing deliver_files; got %v", reg.Names())
	}
	if _, ok := reg.Get("verify_artifact"); ok {
		t.Fatalf("economy workspace registry exposed hidden verify_artifact; got %v", reg.Names())
	}
}

func TestAddBuiltinsNormalizesExplicitEnabledNames(t *testing.T) {
	for _, tt := range []struct {
		name      string
		workspace bool
	}{
		{name: "workspace", workspace: true},
		{name: "no workdir", workspace: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := robustTempDir(t)
			workDir := ""
			if tt.workspace {
				workDir = root
			}
			reg := tool.NewRegistry()
			var stderr bytes.Buffer
			addBuiltins(
				reg,
				[]string{" read_file ", "", "read_file", " deliver_files ", " verify_artifact ", " unknown_builtin ", "unknown_builtin", "   "},
				[]string{root},
				sandbox.Spec{},
				120*time.Second,
				builtin.SearchSpec{},
				&stderr,
				workDir,
				netclient.ProxySpec{},
			)

			want := []string{"read_file", "deliver_files"}
			if tt.workspace {
				want = []string{"deliver_files", "read_file"}
			}
			if got := reg.Names(); !reflect.DeepEqual(got, want) {
				t.Fatalf("registry names = %v, want normalized deduplicated names %v", got, want)
			}
			if got, want := stderr.String(), "warning: unknown built-in tool \"unknown_builtin\"\n"; got != want {
				t.Fatalf("warnings = %q, want %q", got, want)
			}
		})
	}
}

func TestBuildOmitsDisabledSkillsFromPromptAndRuntimeList(t *testing.T) {
	dir := robustTempDir(t)
	home := robustTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[skills]
disabled_skills = ["projskill", "review"]

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)
	writeFile(t, dir, ".kwu/skills/projskill.md", "---\ndescription: a project skill\n---\nplaybook")

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	for _, s := range ctrl.Skills() {
		if s.Name == "projskill" || s.Name == "review" {
			t.Fatalf("disabled skill %q should not be executable: %v", s.Name, ctrl.Skills())
		}
	}
	var allHasProj bool
	for _, s := range ctrl.AllSkills() {
		if s.Name == "projskill" {
			allHasProj = true
		}
	}
	if !allHasProj {
		t.Fatalf("AllSkills should include disabled skills for management: %v", ctrl.AllSkills())
	}
	sys := systemMessage(ctrl.History())
	if strings.Contains(sys, "projskill") || strings.Contains(sys, "- review ") {
		t.Fatalf("disabled skill names should be omitted from system prompt:\n%s", sys)
	}
}

func TestBuildOmitsExcludedSkillRootsFromPromptAndRuntimeList(t *testing.T) {
	dir := robustTempDir(t)
	home := robustTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Chdir(dir)
	excluded := filepath.Join(home, ".agents", "skills")
	writeFile(t, home, ".kwu/skills/keep.md", "---\ndescription: keep\n---\nplaybook")
	writeFile(t, home, ".agents/skills/noisy.md", "---\ndescription: noisy\n---\nplaybook")
	writeFile(t, dir, "kwu.toml", fmt.Sprintf(`
default_model = "test-model"

[agent]
system_prompt = "BASE"

[skills]
excluded_paths = [%q]

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`, excluded))

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	for _, s := range ctrl.Skills() {
		if s.Name == "noisy" {
			t.Fatalf("excluded skill should not be executable: %v", ctrl.Skills())
		}
	}
	sys := systemMessage(ctrl.History())
	if strings.Contains(sys, "noisy") {
		t.Fatalf("excluded skill name should be omitted from system prompt:\n%s", sys)
	}
	if !strings.Contains(sys, "keep") {
		t.Fatalf("non-excluded skill should remain in system prompt:\n%s", sys)
	}
}

// TestBuildWithoutMemoryLeavesPromptUnchanged is the inverse invariant: with no
// memory files, the system prompt is exactly the configured base — the cache
// prefix is untouched by the memory feature.
func TestBuildWithoutMemoryLeavesPromptUnchanged(t *testing.T) {
	dir := robustTempDir(t)
	home := robustTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "JUST THE BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sys := systemMessage(ctrl.History())
	// The built-in skills always append a "# Skills" index to the prefix; this
	// test is about memory, so strip that and assert the remaining base is exactly
	// the configured prompt — i.e. no *project/ancestor* memory leaked in. (A
	// user-global KWU.md in the real config dir could append; the test
	// environment has none, so the base stands alone.)
	base := sys
	if i := strings.Index(sys, "\n\n# Skills"); i >= 0 {
		base = sys[:i]
	}
	// The language policy is always appended at boot; strip it so this assertion
	// is purely about whether project/ancestor memory leaked into the base. The
	// user-decision policy is another fixed boot policy and is stripped for the
	// same reason.
	base = stripLanguagePolicy(base)
	if base != "JUST THE BASE" {
		t.Fatalf("expected untouched base prompt, got:\n%s", sys)
	}
}

func TestBuildLanguagePolicyIsAppended(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sys := systemMessage(ctrl.History())
	if !strings.Contains(sys, config.LanguagePolicy) {
		t.Fatalf("language policy missing from system prompt:\n%s", sys)
	}
}

func TestBuildInjectsResolvedModelIdentity(t *testing.T) {
	isolateConfigHome(t)
	tests := []struct {
		name      string
		modelRef  string
		wantModel string
	}{
		{name: "full reference", modelRef: "vendor/selected-model", wantModel: "selected-model"},
		{name: "provider name", modelRef: "vendor", wantModel: "default-model"},
		{name: "bare model", modelRef: "selected-model", wantModel: "selected-model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := robustTempDir(t)
			t.Chdir(dir)
			writeFile(t, dir, "kwu.toml", `
default_model = "vendor"

[agent]
system_prompt = "CUSTOM BASE"

[[providers]]
name = "vendor"
kind = "openai"
base_url = "https://example.invalid"
model = "default-model"
models = ["default-model", "selected-model"]
default = "default-model"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

			ctrl, err := Build(context.Background(), Options{Model: tt.modelRef})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			defer ctrl.Close()

			sys := systemMessage(ctrl.History())
			if !strings.HasPrefix(sys, "CUSTOM BASE") {
				t.Fatalf("custom system prompt was not preserved:\n%s", sys)
			}
			if want := config.ModelIdentityPolicy(tt.wantModel); !strings.Contains(sys, want) {
				t.Fatalf("resolved model identity policy missing:\nwant %q\nprompt:\n%s", want, sys)
			}
			if leaked := config.ModelIdentityPolicy("vendor/" + tt.wantModel); strings.Contains(sys, leaked) {
				t.Fatalf("Provider-prefixed model identity leaked into system prompt:\n%s", sys)
			}
		})
	}
}

func TestBuildAppendsDesktopProductIdentityPolicyOnlyWhenVersionProvided(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	version := "release\"\\variant\nnext-line"
	for _, tt := range []struct {
		name           string
		desktopVersion string
		wantPolicy     bool
	}{
		{name: "desktop version", desktopVersion: version, wantPolicy: true},
		{name: "other host", wantPolicy: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, err := Build(context.Background(), Options{DesktopVersion: tt.desktopVersion})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			defer ctrl.Close()

			sys := systemMessage(ctrl.History())
			policy := config.DesktopProductIdentityPolicy(version)
			if tt.wantPolicy {
				if !strings.HasSuffix(sys, policy) {
					t.Fatalf("desktop product identity policy is not the final system prompt block:\n%s", sys)
				}
				if strings.Contains(sys, version) {
					t.Fatalf("raw desktop version entered system prompt:\n%s", sys)
				}
			} else if strings.Contains(sys, "Desktop product identity:") {
				t.Fatalf("non-Desktop host received product identity policy:\n%s", sys)
			}
		})
	}
}

func TestBuildAppendsUserDecisionPolicyToCustomSystemPrompt(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sys := systemMessage(ctrl.History())
	for _, want := range []string{
		"User-owned choices",
		"call the ask tool",
		"Do not ask in prose",
	} {
		if !strings.Contains(sys, want) {
			t.Fatalf("user decision policy missing %q from custom system prompt:\n%s", want, sys)
		}
	}
}

func TestBuildAppendsArtifactDeliveryPolicyToCustomSystemPrompt(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sys := systemMessage(ctrl.History())
	for _, want := range []string{
		"real file",
		"file itself is a result",
		"open, use, download, share, or continue editing",
		"existing file modified in place",
		"source code, tests, configuration, migrations",
		"workspace changes, not deliverables",
		"particular file as a standalone result",
		"writer-capable",
		"workspace-relative target path",
		"code block",
		"ask the user to save",
		"last successful deliver_files call in the turn is authoritative",
		"complete final paths manifest",
		"paths:[]",
		"Markdown link does not publish a file",
		"If any workspace-mutating tool runs afterward",
		"final response must link each actual workspace file path",
		"Markdown link",
		"Finish all writing, testing, review, delegated task, background wait work, and task-list sign-off before declaring delivery",
	} {
		if !strings.Contains(config.ArtifactDeliveryPolicy, want) {
			t.Fatalf("artifact delivery policy missing %q:\n%s", want, config.ArtifactDeliveryPolicy)
		}
	}
	if strings.Contains(strings.ToLower(config.ArtifactDeliveryPolicy), "host verifies") ||
		strings.Contains(strings.ToLower(config.ArtifactDeliveryPolicy), "final boundary") {
		t.Fatalf("artifact delivery policy retains host final verification guidance:\n%s", config.ArtifactDeliveryPolicy)
	}
	if strings.Contains(config.ArtifactDeliveryPolicy, "explicitly asks to create, generate, export, or save") {
		t.Fatalf("artifact delivery policy retains verb-triggered delivery guidance:\n%s", config.ArtifactDeliveryPolicy)
	}
	if !strings.Contains(sys, config.ArtifactDeliveryPolicy) {
		t.Fatalf("artifact delivery policy missing from custom system prompt:\n%s", sys)
	}
	if decision, artifact, language := strings.Index(sys, config.UserDecisionPolicy), strings.Index(sys, config.ArtifactDeliveryPolicy), strings.Index(sys, config.LanguagePolicy); !(decision < artifact && artifact < language) {
		t.Fatalf("cache-stable policy order = decision:%d artifact:%d language:%d", decision, artifact, language)
	}
}

func TestBuildAppendsPersistentTaskDispatchPolicyToCustomSystemPrompt(t *testing.T) {
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "CUSTOM ORDINARY PROMPT"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)

	ctrl, err := Build(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sys := systemMessage(ctrl.History())
	if !strings.HasPrefix(sys, "CUSTOM ORDINARY PROMPT") {
		t.Fatalf("custom system prompt was not preserved:\n%s", sys)
	}
	assertPersistentTaskDispatchPolicy(t, sys)
}

func TestBuildAssemblyAppendsArtifactDeliveryPolicyToCustomSystemPrompt(t *testing.T) {
	b := &buildAssembly{
		ctx:            context.Background(),
		cfg:            config.Default(),
		entry:          &config.ProviderEntry{Model: "assembly-model"},
		basePrompt:     "BASE",
		userExpertPath: filepath.Join(t.TempDir(), "experts.json"),
	}
	prompt, _, err := b.buildSystemPrompt("", "")
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}
	if !strings.Contains(prompt, config.ArtifactDeliveryPolicy) {
		t.Fatalf("artifact delivery policy missing from build assembly custom prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, config.ModelIdentityPolicy("assembly-model")) {
		t.Fatalf("model identity policy missing from build assembly custom prompt:\n%s", prompt)
	}
}

func TestBuildAssemblyAppendsDesktopProductIdentityPolicyAtFinalPromptTail(t *testing.T) {
	version := "desktop\"\\version"
	b := &buildAssembly{
		ctx:            context.Background(),
		cfg:            config.Default(),
		entry:          &config.ProviderEntry{Model: "assembly-model"},
		basePrompt:     "BASE",
		userExpertPath: filepath.Join(t.TempDir(), "experts.json"),
		opts:           Options{DesktopVersion: version},
	}
	prompt, _, err := b.buildSystemPrompt("", "")
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}
	if !strings.HasSuffix(prompt, config.DesktopProductIdentityPolicy(version)) {
		t.Fatalf("desktop product identity policy is not the final build assembly block:\n%s", prompt)
	}
}

func TestBuildAssemblyAppendsPersistentTaskDispatchPolicyToSelectedExpertPrompt(t *testing.T) {
	const expertPrompt = "SELECTED EXPERT PROMPT"
	b := &buildAssembly{
		ctx:        context.Background(),
		cfg:        config.Default(),
		entry:      &config.ProviderEntry{Model: "assembly-model"},
		basePrompt: "BASE",
		opts: Options{ExpertCatalogLoader: func() (*expert.Catalog, error) {
			return expert.NewCatalog([]expert.Expert{{
				ID:      "selected-expert",
				Name:    "Selected Expert",
				Prompt:  expertPrompt,
				Enabled: true,
				Source:  expert.SourceOfficial,
				Mode:    expert.ModeExpert,
			}})
		}},
	}
	prompt, selection, err := b.buildSystemPrompt("", "selected-expert")
	if err != nil {
		t.Fatalf("buildSystemPrompt: %v", err)
	}
	if !selection.Selected || selection.ID != "selected-expert" {
		t.Fatalf("selection = %+v, want selected expert", selection)
	}
	if !strings.Contains(prompt, expertPrompt) {
		t.Fatalf("selected expert prompt missing:\n%s", prompt)
	}
	assertPersistentTaskDispatchPolicy(t, prompt)
}

func assertPersistentTaskDispatchPolicy(t *testing.T, prompt string) {
	t.Helper()
	if !strings.Contains(prompt, config.PersistentTaskDispatchPolicy) {
		t.Fatalf("shared persistent task dispatch policy missing:\n%s", prompt)
	}
	for _, want := range []string{
		"Persistent task dispatch",
		"all inputs it needs are ready",
		"wait_task",
		"independent=true",
		"After wait_task, inspect the completed result and write a new downstream prompt",
		"task_run_id",
		"depends_on",
		"even when no task is currently running",
		"not evidence that later execution or verification actually occurred",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("persistent task dispatch policy missing %q:\n%s", want, prompt)
		}
	}
	artifact := strings.Index(prompt, config.ArtifactDeliveryPolicy)
	persistent := strings.Index(prompt, config.PersistentTaskDispatchPolicy)
	temporal := strings.Index(prompt, config.TemporalPolicy)
	if !(artifact >= 0 && artifact < persistent && persistent < temporal) {
		t.Fatalf("cache-stable policy order = artifact:%d persistent:%d temporal:%d", artifact, persistent, temporal)
	}
}

func systemMessage(msgs []provider.Message) string {
	for _, m := range msgs {
		if m.Role == provider.RoleSystem {
			return m.Content
		}
	}
	return ""
}

func userMessage(msgs []provider.Message) string {
	for _, m := range msgs {
		if m.Role == provider.RoleUser {
			return m.Content
		}
	}
	return ""
}

func stripLanguagePolicy(s string) string {
	s = strings.TrimSpace(s)
	for _, policy := range []string{
		config.ModelIdentityPolicy("x"),
		config.LanguagePolicy,
		config.TemporalPolicy,
		config.PersistentTaskDispatchPolicy,
		config.ArtifactDeliveryPolicy,
		config.UserDecisionPolicy,
	} {
		s = strings.TrimSpace(strings.TrimSuffix(s, policy))
	}
	return s
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := writeFileRaw(dir, name, body); err != nil {
		t.Fatal(err)
	}
}

func TestRememberPermissionRuleUsesActorProfileUserConfig(t *testing.T) {
	isolateBootProfilePaths(t)
	workspace := robustTempDir(t)
	actor := userctx.ActorContext{
		ActorID:       "actor-a",
		ProfileID:     "profile-a",
		AuthMethod:    "test",
		WorkspaceRoot: workspace,
	}
	paths := config.NewProfilePathResolver(actor)
	writeFile(t, filepath.Dir(paths.UserConfigPath()), filepath.Base(paths.UserConfigPath()), `
[permissions]
allow = ["Bash(profile*)"]
`)
	writeFile(t, workspace, "kwu.toml", `
[permissions]
allow = ["Bash(workspace*)"]
`)

	const rule = "Bash(go test ./...)"
	res := rememberPermissionRuleForActor(actor, rule)
	if !res.Saved || res.Path != paths.UserConfigPath() {
		t.Fatalf("remember result = %+v, want saved to profile config %s", res, paths.UserConfigPath())
	}

	profileCfg := config.LoadForEdit(paths.UserConfigPath())
	if !hasPermissionRule(profileCfg.Permissions.Allow, rule) {
		t.Fatalf("profile rule missing from profile config: %v", profileCfg.Permissions.Allow)
	}
	workspaceCfg := config.LoadForEdit(filepath.Join(workspace, "kwu.toml"))
	if hasPermissionRule(workspaceCfg.Permissions.Allow, rule) {
		t.Fatalf("remembered rule was written to workspace config: %v", workspaceCfg.Permissions.Allow)
	}
}

func TestRememberPermissionRuleDoesNotHydrateProviderCredential(t *testing.T) {
	isolateBootProfilePaths(t)
	workspace := robustTempDir(t)
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "subscription-profile", AuthMethod: "test", WorkspaceRoot: workspace}
	paths := config.NewProfilePathResolver(actor)
	const key = "B2B_REMEMBER_PERMISSION_KEY"
	t.Setenv(key, "profile-key")
	writeFile(t, filepath.Dir(paths.UserConfigPath()), filepath.Base(paths.UserConfigPath()), `
[[providers]]
name = "fixture"
kind = "openai"
base_url = "https://fixture.invalid/v1"
model = "fixture-model"
api_key_env = "B2B_REMEMBER_PERMISSION_KEY"
`)
	writeFile(t, filepath.Dir(paths.UserCredentialsPath()), filepath.Base(paths.UserCredentialsPath()), key+"=profile-key\n")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}

	const rule = "Bash(go test ./...)"
	res := rememberPermissionRuleForActor(actor, rule)
	if !res.Saved || res.Path != paths.UserConfigPath() {
		t.Fatalf("remember result = %+v, want saved profile permission", res)
	}
	if got := os.Getenv(key); got != "" {
		t.Fatalf("remember permission hydrated provider credential %q", got)
	}
	stored := config.LoadForEditWithoutCredentialsWithPaths(paths)
	if !hasPermissionRule(stored.Permissions.Allow, rule) {
		t.Fatalf("remembered permission missing: %v", stored.Permissions.Allow)
	}
}

func TestRememberPermissionRuleSeparatesProfiles(t *testing.T) {
	isolateBootProfilePaths(t)
	workspace := robustTempDir(t)
	actorA := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a", AuthMethod: "test", WorkspaceRoot: workspace}
	actorB := userctx.ActorContext{ActorID: "actor-b", ProfileID: "profile-b", AuthMethod: "test", WorkspaceRoot: workspace}
	pathsA := config.NewProfilePathResolver(actorA)
	pathsB := config.NewProfilePathResolver(actorB)

	const rule = "Edit(src/app.go)"
	res := rememberPermissionRuleForActor(actorA, rule)
	if !res.Saved || res.Path != pathsA.UserConfigPath() {
		t.Fatalf("remember result = %+v, want saved to profile A config", res)
	}

	cfgA := config.LoadForEdit(pathsA.UserConfigPath())
	if !hasPermissionRule(cfgA.Permissions.Allow, rule) {
		t.Fatalf("profile A rule missing: %v", cfgA.Permissions.Allow)
	}
	cfgB := config.LoadForEdit(pathsB.UserConfigPath())
	if hasPermissionRule(cfgB.Permissions.Allow, rule) {
		t.Fatalf("profile B should not see profile A rule: %v", cfgB.Permissions.Allow)
	}
}

func TestRememberPermissionRuleDefaultProfileUsesUserConfigPath(t *testing.T) {
	isolateBootProfilePaths(t)
	workspace := robustTempDir(t)
	writeFile(t, workspace, "kwu.toml", `
[permissions]
allow = ["Bash(workspace*)"]
`)

	const rule = "Bash(go env)"
	res := rememberPermissionRuleForActor(userctx.Default(workspace), rule)
	if !res.Saved || res.Path != config.UserConfigPath() {
		t.Fatalf("remember result = %+v, want saved to default user config %s", res, config.UserConfigPath())
	}

	userCfg := config.LoadForEdit(config.UserConfigPath())
	if !hasPermissionRule(userCfg.Permissions.Allow, rule) {
		t.Fatalf("default profile rule missing from user config: %v", userCfg.Permissions.Allow)
	}
	workspaceCfg := config.LoadForEdit(filepath.Join(workspace, "kwu.toml"))
	if hasPermissionRule(workspaceCfg.Permissions.Allow, rule) {
		t.Fatalf("default profile rule was written to workspace config: %v", workspaceCfg.Permissions.Allow)
	}
}

func TestRememberPermissionRuleSkipsRuleCoveredByExistingAllow(t *testing.T) {
	isolateBootProfilePaths(t)
	workspace := robustTempDir(t)
	paths := config.NewProfilePathResolver(userctx.Default(workspace))
	writeFile(t, filepath.Dir(paths.UserConfigPath()), filepath.Base(paths.UserConfigPath()), `
[permissions]
allow = ["Bash(go test:*)"]
`)

	res := rememberPermissionRuleForActor(userctx.Default(workspace), "Bash(go test ./...)")
	if res.Saved || res.CoveredBy != "Bash(go test:*)" {
		t.Fatalf("remember result = %+v, want already covered", res)
	}
	cfg := config.LoadForEdit(paths.UserConfigPath())
	if len(cfg.Permissions.Allow) != 1 || cfg.Permissions.Allow[0] != "Bash(go test:*)" {
		t.Fatalf("allow rules = %v, want only existing prefix", cfg.Permissions.Allow)
	}
}

func TestRememberPermissionRulePrunesNarrowRulesWhenSavingBroaderRule(t *testing.T) {
	isolateBootProfilePaths(t)
	workspace := robustTempDir(t)
	paths := config.NewProfilePathResolver(userctx.Default(workspace))
	writeFile(t, filepath.Dir(paths.UserConfigPath()), filepath.Base(paths.UserConfigPath()), `
[permissions]
allow = ["Bash(go test ./...)", "Bash(go build ./...)"]
`)

	res := rememberPermissionRuleForActor(userctx.Default(workspace), "Bash(go test:*)")
	if !res.Saved || res.CoveredBy != "" {
		t.Fatalf("remember result = %+v, want saved broader rule", res)
	}
	cfg := config.LoadForEdit(paths.UserConfigPath())
	if hasPermissionRule(cfg.Permissions.Allow, "Bash(go test ./...)") {
		t.Fatalf("narrow go test rule should be pruned: %v", cfg.Permissions.Allow)
	}
	if !hasPermissionRule(cfg.Permissions.Allow, "Bash(go build ./...)") || !hasPermissionRule(cfg.Permissions.Allow, "Bash(go test:*)") {
		t.Fatalf("allow rules = %v, want unrelated exact plus prefix", cfg.Permissions.Allow)
	}
}

func hasPermissionRule(rules []string, want string) bool {
	for _, rule := range rules {
		if rule == want {
			return true
		}
	}
	return false
}

// TestBuildSkipsLegacyConfigEndToEnd drives the real boot path: new KWU
// development builds do not import old product config/key automatically.
func TestBuildSkipsLegacyConfigEndToEnd(t *testing.T) {
	home := robustTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)                               // os.UserHomeDir on Windows
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // os.UserConfigDir on Linux
	t.Setenv("AppData", filepath.Join(home, "AppData"))         // os.UserConfigDir on Windows
	t.Setenv("KWU_CREDENTIALS_STORE", "file")
	t.Setenv("DEEPSEEK_API_KEY", "")

	proj := robustTempDir(t)
	t.Chdir(proj)
	writeFile(t, proj, "kwu.toml", "")
	writeFile(t, filepath.Join(home, ".kwu"), "config.json",
		`{"apiKey":"sk-e2e","lang":"zh","mcpServers":{"fs":{"command":"npx","args":["-y","server-fs"]}}}`)

	var notices []string
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e.Text)
		}
	})

	ctrl, err := Build(context.Background(), Options{Sink: sink})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	migrated := false
	for _, n := range notices {
		if strings.Contains(n, "migrated your previous configuration") {
			migrated = true
		}
	}
	if migrated {
		t.Fatalf("unexpected legacy config migration notice emitted; got %v", notices)
	}

	dest := config.UserConfigPath()
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("legacy config migration wrote %s, stat err=%v", dest, err)
	}

	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "" {
		t.Errorf("DEEPSEEK_API_KEY should not be imported from legacy config: %q", got)
	}

	if _, err := os.Stat(config.UserCredentialsPath()); !os.IsNotExist(err) {
		t.Errorf("legacy key migration wrote credentials store, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".env")); !os.IsNotExist(err) {
		t.Errorf("legacy key migration wrote the user's ~/.env, stat err=%v", err)
	}
}

func TestBuildMigratesLegacySessionsFromConfigSessionDir(t *testing.T) {
	home := robustTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	proj := robustTempDir(t)
	writeFile(t, proj, "kwu.toml", "")

	legacyConfig := config.LegacyUserConfigPath()
	if legacyConfig == "" {
		t.Skip("legacy OS config path matches primary path on this platform")
	}
	legacyDir := filepath.Join(filepath.Dir(legacyConfig), "sessions")
	writeFile(t, legacyDir, "custom-root.events.jsonl",
		`{"type":"user.message","id":1,"ts":"t","turn":0,"text":"hello from redirected config root"}`+"\n"+
			`{"type":"model.final","id":2,"ts":"t","turn":0,"content":"hi from redirected root","toolCalls":[],"usage":{},"costUsd":0}`+"\n")

	var notices []string
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e.Text)
		}
	})

	// Pass the project root via WorkspaceRoot instead of t.Chdir: changing the
	// process cwd into a t.TempDir makes Windows refuse to remove that dir during
	// test cleanup (the cwd counts as "in use"), which is the only thing this test
	// failed on. WorkspaceRoot loads the same config without touching the cwd.
	ctrl, err := Build(context.Background(), Options{Sink: sink, WorkspaceRoot: proj})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	sessionPath := filepath.Join(config.SessionDir(), "custom-root.jsonl")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("legacy config-root session not imported to %s: %v", sessionPath, err)
	}
	if !strings.Contains(string(data), "hello from redirected config root") {
		t.Fatalf("migrated session missing legacy content:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(config.SessionDir(), ".legacy-imported.v0-events-config")); err != nil {
		t.Fatalf("config-root legacy import marker missing: %v", err)
	}
	sessionImported := false
	for _, n := range notices {
		if strings.Contains(n, "imported") && strings.Contains(n, "past session") && strings.Contains(n, legacyDir) {
			sessionImported = true
		}
	}
	if !sessionImported {
		t.Errorf("no config-root session-import notice emitted; got %v", notices)
	}
}

func TestBuildMigratesLegacyHomeAndProjectSessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("legacy home paths are Unix-only")
	}
	home := robustTempDir(t)
	xdg := filepath.Join(home, "xdg-config")
	kwuHome := filepath.Join(home, "kwu-home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("KWU_HOME", kwuHome)

	proj := robustTempDir(t)
	writeFile(t, proj, "kwu.toml", "[codegraph]\nenabled = false\n")

	legacyRoot := filepath.Join(home, ".kwu")
	writeFile(t, filepath.Join(legacyRoot, "sessions"), "xdg-flat.events.jsonl",
		`{"type":"user.message","id":1,"ts":"t","turn":0,"text":"hello from xdg"}`+"\n"+
			`{"type":"model.final","id":2,"ts":"t","turn":0,"content":"hi from xdg","toolCalls":[],"usage":{},"costUsd":0}`+"\n")

	slug := config.WorkspaceSlug(proj)
	legacyProjectDir := filepath.Join(legacyRoot, "projects", slug, "sessions")
	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "hello from old project session"})
	if err := session.Save(filepath.Join(legacyProjectDir, "project-chat.jsonl")); err != nil {
		t.Fatalf("save legacy project session: %v", err)
	}

	ctrl, err := Build(context.Background(), Options{WorkspaceRoot: proj})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	flatData, err := os.ReadFile(filepath.Join(config.SessionDir(), "xdg-flat.jsonl"))
	if err != nil {
		t.Fatalf("legacy XDG flat session not imported: %v", err)
	}
	if !strings.Contains(string(flatData), "hello from xdg") {
		t.Fatalf("legacy XDG flat session missing content:\n%s", flatData)
	}
	projectPath := filepath.Join(config.MemoryUserDir(), "projects", slug, "sessions", "project-chat.jsonl")
	projectData, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatalf("legacy project session not imported to %s: %v", projectPath, err)
	}
	if !strings.Contains(string(projectData), "hello from old project session") {
		t.Fatalf("legacy project session missing content:\n%s", projectData)
	}
}

// isolateConfigHome redirects os.UserConfigDir() (and the cache subtree under
// it) at a per-test temp dir by overriding the env vars Go's stdlib reads —
// HOME on darwin, XDG_CONFIG_HOME on linux. Without this, Build's plugin path
// would persist startup stats and cached schemas into the developer's real
// ~/Library/Application Support tree and bleed state across tests. Mirrors the
// withTempCache helper in internal/plugin/stats_test.go.
func isolateConfigHome(t *testing.T) string {
	t.Helper()
	dir := robustTempDir(t)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
	t.Setenv("KWU_CREDENTIALS_STORE", "file")
	return dir
}

func isolateBootProfilePaths(t *testing.T) string {
	t.Helper()
	home := robustTempDir(t)
	state := filepath.Join(robustTempDir(t), "state")
	cache := filepath.Join(robustTempDir(t), "cache")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv("KWU_HOME", home)
	t.Setenv("KWU_STATE_HOME", state)
	t.Setenv("KWU_CACHE_HOME", cache)
	t.Setenv("KWU_CREDENTIALS_STORE", "file")
	return home
}

// TestPartitionByTier pins the bucket assignment contract that the rest of
// boot.go's plugin orchestration depends on: eager keeps its blocking startup
// slice, while empty, background, legacy lazy, and unknown tiers all warm up in
// the background.
func TestPartitionByTier(t *testing.T) {
	entries := []config.PluginEntry{
		{Name: "e1", Tier: "eager"},
		{Name: "l1", Tier: "lazy"},
		{Name: "b1", Tier: "background"},
		{Name: "default", Tier: ""}, // empty defaults to background
	}

	eager, bg := partitionByTier(entries)

	if len(eager) != 1 || eager[0].Name != "e1" {
		t.Fatalf("eager bucket = %+v, want [e1]", eager)
	}
	if len(bg) != 3 || bg[0].Name != "l1" || bg[1].Name != "b1" || bg[2].Name != "default" {
		t.Fatalf("background bucket = %+v, want [l1, b1, default] preserving input order", bg)
	}
}

func TestFilterProjectMCPAutoStartRequiresTrustForLocalStdio(t *testing.T) {
	entries := []config.PluginEntry{
		{Name: "project-local", Command: "node", ProjectMCP: true},
		{Name: "project-http", Type: "http", URL: "https://mcp.example.com", ProjectMCP: true},
		{Name: "user-local", Command: "node"},
	}

	got, blocked := filterProjectMCPAutoStart(entries, false)
	if len(blocked) != 1 || blocked[0] != "project-local" {
		t.Fatalf("blocked = %+v, want [project-local]", blocked)
	}
	if len(got) != 2 || got[0].Name != "project-http" || got[1].Name != "user-local" {
		t.Fatalf("filtered entries = %+v, want project-http + user-local", got)
	}

	trusted, blocked := filterProjectMCPAutoStart(entries, true)
	if len(blocked) != 0 {
		t.Fatalf("trusted project should not block entries, got %+v", blocked)
	}
	if len(trusted) != len(entries) {
		t.Fatalf("trusted entries = %+v, want all entries", trusted)
	}
}

func TestPluginSpecsTrustKnownCodeGraphReadTools(t *testing.T) {
	specs := PluginSpecs([]config.PluginEntry{{Name: "codegraph"}})
	if len(specs) != 1 {
		t.Fatalf("PluginSpecs returned %d specs, want 1", len(specs))
	}
	for _, name := range []string{"codegraph_context", "codegraph_search", "context", "search"} {
		if !specs[0].ReadOnlyToolNames[name] {
			t.Fatalf("codegraph spec missing read-only override for %q: %+v", name, specs[0].ReadOnlyToolNames)
		}
	}
}

func TestPluginSpecsTrustConfiguredReadOnlyTools(t *testing.T) {
	specs := PluginSpecs([]config.PluginEntry{{
		Name:                 "github",
		TrustedReadOnlyTools: []string{"issue_read", " pull_request_read ", ""},
	}})
	if len(specs) != 1 {
		t.Fatalf("PluginSpecs returned %d specs, want 1", len(specs))
	}
	for _, name := range []string{"issue_read", "pull_request_read"} {
		if !specs[0].ReadOnlyToolNames[name] {
			t.Fatalf("configured trusted read-only tool %q missing: %+v", name, specs[0].ReadOnlyToolNames)
		}
	}
	if specs[0].ReadOnlyToolNames[""] {
		t.Fatalf("empty trusted read-only tool name should be ignored: %+v", specs[0].ReadOnlyToolNames)
	}
}

func TestPluginSpecsTrustPlanModeAllowedMCPTools(t *testing.T) {
	specs := PluginSpecsForRootWithPlanModeAllowedTools(
		[]config.PluginEntry{{Name: "github"}, {Name: "linear"}},
		"",
		[]string{
			"mcp__github__issue_read",
			"mcp__linear__issue_read",
			"mcp__github__",
			"read_file",
			"mcp__other__issue_read",
		},
	)
	if len(specs) != 2 {
		t.Fatalf("PluginSpecsForRootWithPlanModeAllowedTools returned %d specs, want 2", len(specs))
	}
	if !specs[0].ReadOnlyModelToolNames["mcp__github__issue_read"] {
		t.Fatalf("github allowed MCP tool missing from model trust map: %+v", specs[0].ReadOnlyModelToolNames)
	}
	if specs[0].ReadOnlyModelToolNames["mcp__github__"] || specs[0].ReadOnlyModelToolNames["mcp__other__issue_read"] {
		t.Fatalf("github trust map accepted non-concrete or other-server tools: %+v", specs[0].ReadOnlyModelToolNames)
	}
	if !specs[1].ReadOnlyModelToolNames["mcp__linear__issue_read"] {
		t.Fatalf("linear allowed MCP tool missing from model trust map: %+v", specs[1].ReadOnlyModelToolNames)
	}
}

func TestPluginSpecsForRootPinsCodeGraphToWorkspace(t *testing.T) {
	specs := PluginSpecsForRoot([]config.PluginEntry{{Name: "codegraph"}}, "/workspace")
	if len(specs) != 1 {
		t.Fatalf("PluginSpecsForRoot returned %d specs, want 1", len(specs))
	}
	if specs[0].Dir != "/workspace" {
		t.Fatalf("codegraph Dir = %q, want workspace root", specs[0].Dir)
	}
}

func TestPluginSpecsForRootDoesNotPinHTTPCodeGraph(t *testing.T) {
	specs := PluginSpecsForRoot([]config.PluginEntry{{Name: "codegraph", Type: "http", URL: "https://example.com/mcp"}}, "/workspace")
	if len(specs) != 1 {
		t.Fatalf("PluginSpecsForRoot returned %d specs, want 1", len(specs))
	}
	if specs[0].Dir != "" {
		t.Fatalf("http codegraph Dir = %q, want empty", specs[0].Dir)
	}
}

func TestPluginSpecsDoNotTrustCodeGraphToolsForOtherServers(t *testing.T) {
	specs := PluginSpecs([]config.PluginEntry{{Name: "not-codegraph"}})
	if len(specs) != 1 {
		t.Fatalf("PluginSpecs returned %d specs, want 1", len(specs))
	}
	if specs[0].ReadOnlyToolNames["codegraph_context"] {
		t.Fatalf("non-codegraph spec should not receive codegraph read-only overrides: %+v", specs[0].ReadOnlyToolNames)
	}
}

func TestBuildMigratesLegacyEagerTierToBackground(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"

[[plugins]]
name = "legacy-eager"
command = "kwu-missing-legacy-eager-mcp"
tier = "eager"
`)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	failures := waitForMCPFailure(t, ctrl.Host(), "legacy-eager", 2*time.Second)
	if len(failures) != 1 || failures[0].Name != "legacy-eager" {
		t.Fatalf("failures = %+v, want background startup failure for migrated legacy eager plugin", failures)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "kwu.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\ntier") {
		t.Fatalf("legacy eager tier should be removed during load:\n%s", raw)
	}
}

func TestBuildMigratesLegacyLazyTierToBackground(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"

[[plugins]]
name = "legacy-lazy"
command = "kwu-missing-legacy-lazy-mcp"
tier = "lazy"
`)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	failures := waitForMCPFailure(t, ctrl.Host(), "legacy-lazy", 2*time.Second)
	if len(failures) != 1 || failures[0].Name != "legacy-lazy" {
		t.Fatalf("failures = %+v, want background startup failure for migrated legacy lazy plugin", failures)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "kwu.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\ntier") {
		t.Fatalf("legacy lazy tier should be removed during load:\n%s", raw)
	}
}

func TestBuildDefaultsToNearestGitRoot(t *testing.T) {
	isolateConfigHome(t)
	root := robustTempDir(t)
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(root, "cmd", "tool")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "kwu.toml", `
default_model = "root-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "root-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`)
	t.Chdir(subdir)

	ctrl, err := Build(context.Background(), Options{Model: "root-model"})
	if err != nil {
		t.Fatalf("Build should load config from nearest git root: %v", err)
	}
	defer ctrl.Close()
}

func TestBuildMigratesLegacyEagerBeforeStatsDemotion(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	// Three samples above 2*budget — the rule in stats.go's Recommend triggers
	// when the trailing window is entirely over the threshold. Use 30s so even
	// future budget bumps stay below the threshold.
	for i := 0; i < 3; i++ {
		if err := plugin.RecordStartup("slowserver", 30*time.Second); err != nil {
			t.Fatalf("RecordStartup #%d: %v", i, err)
		}
	}

	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"

[[plugins]]
name = "slowserver"
command = "kwu-missing-slow-mcp-binary"
tier = "eager"
`)

	var notices []event.Event
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ctrl, err := Build(ctx, Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.Notice {
				notices = append(notices, e)
			}
		}),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer ctrl.Close()

	failures := waitForMCPFailure(t, ctrl.Host(), "slowserver", 2*time.Second)
	if len(failures) != 1 || failures[0].Name != "slowserver" {
		t.Fatalf("Host.Failures() = %+v, want background startup failure for migrated plugin", failures)
	}

	foundDemoteNotice := false
	for _, n := range notices {
		if strings.Contains(n.Text, "lazy") {
			foundDemoteNotice = true
			break
		}
	}
	if foundDemoteNotice {
		t.Fatalf("demotion notice should not mention legacy lazy tier; got notices %+v", notices)
	}
}

func waitForMCPFailure(t *testing.T, h *plugin.Host, name string, timeout time.Duration) []plugin.Failure {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		failures := h.Failures()
		for _, f := range failures {
			if f.Name == name {
				return failures
			}
		}
		if time.Now().After(deadline) {
			return failures
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestHelperProcess is invoked as a subprocess by TestBuildEagerStartsAtBoot
// and TestBuildLazyDoesNotConnectAtBoot. It mirrors the minimal MCP stdio
// server in internal/plugin/plugin_test.go so the boot package can drive an
// end-to-end handshake without depending on the plugin package's test helper
// (Go's testing framework only re-invokes the binary of the test package
// currently running). The helper gates on GO_WANT_HELPER_PROCESS=1 so a
// normal `go test ./internal/boot/...` does not trip it.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	in := bufio.NewReader(os.Stdin)
	for {
		line, err := in.ReadBytes('\n')
		if err != nil {
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		if req.ID == nil {
			continue // notification: no response
		}

		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]any{"name": "mock", "version": "0"},
				"capabilities":    map[string]any{},
			}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name":        "echo",
				"description": "Echo back the message.",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"msg": map[string]any{"type": "string"}},
					"required":   []string{"msg"},
				},
			}}}
		}

		resp := map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result}
		b, _ := json.Marshal(resp)
		os.Stdout.Write(append(b, '\n'))
	}
}
