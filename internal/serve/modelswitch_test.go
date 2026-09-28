package serve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

const serveSwitchActorProviderKind = "serve-switch-actor-test"
const serveTitleProfileProviderKind = "serve-title-profile-test"

var (
	serveSwitchActorProviderOnce  sync.Once
	serveTitleProfileProviderOnce sync.Once
)

type serveSwitchActorProvider struct{ name string }
type serveTitleProfileProvider struct{ cfg provider.Config }

func (p serveSwitchActorProvider) Name() string { return p.name }

func (p serveSwitchActorProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk)
	close(ch)
	return ch, nil
}

func (p serveTitleProfileProvider) Name() string { return p.cfg.Name }

func (p serveTitleProfileProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk)
	close(ch)
	return ch, nil
}

func registerServeSwitchActorProvider() {
	serveSwitchActorProviderOnce.Do(func() {
		provider.Register(serveSwitchActorProviderKind, func(cfg provider.Config) (provider.Provider, error) {
			return serveSwitchActorProvider{name: cfg.Name}, nil
		})
	})
}

func registerServeTitleProfileProvider() {
	serveTitleProfileProviderOnce.Do(func() {
		provider.Register(serveTitleProfileProviderKind, func(cfg provider.Config) (provider.Provider, error) {
			return serveTitleProfileProvider{cfg: cfg}, nil
		})
	})
}

func TestTitleProviderModelRefUsesConfigDefault(t *testing.T) {
	want := config.Default().DefaultModel
	if got := titleProviderModelRef(&config.Config{}); got != want {
		t.Fatalf("title provider default ref = %q, want %q", got, want)
	}
}

func isolateServeSwitchConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	xdg := filepath.Join(home, ".config")
	appData := filepath.Join(home, "AppData")
	for _, dir := range []string{xdg, appData} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("KWU_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("KWU_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("AppData", appData)
	return home
}

func TestInitTitleProviderUsesCurrentProfileAndWorkspace(t *testing.T) {
	registerServeTitleProfileProvider()
	isolateServeSwitchConfigHome(t)
	t.Setenv("KWU_CREDENTIALS_STORE", config.CredentialsStoreFile)
	root := t.TempDir()
	t.Chdir(t.TempDir())
	key := "KEY_SERVE_TITLE_PROFILE"
	t.Setenv(key, "")
	os.Unsetenv(key)

	actor := userctx.ActorContext{
		ActorID:       "local",
		ProfileID:     "profile-title",
		AuthMethod:    "local",
		WorkspaceRoot: root,
	}
	profilePaths := config.NewProfilePathResolver(actor)
	defaultPaths := config.NewProfilePathResolver(userctx.Default(root))
	writeFile := func(path, body string, perm os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), perm); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(defaultPaths.UserConfigPath(), `
default_model = "title-provider/default-title"

[[providers]]
name = "title-provider"
kind = "serve-title-profile-test"
base_url = "https://default.invalid/v1"
model = "default-title"
api_key_env = "`+key+`"
`, 0o644)
	writeFile(defaultPaths.UserCredentialsPath(), key+"=from_default\n", 0o600)
	writeFile(profilePaths.UserCredentialsPath(), key+"=from_profile\n", 0o600)
	writeFile(filepath.Join(root, "kwu.toml"), `
default_model = "title-provider/profile-title"

[[providers]]
name = "title-provider"
kind = "serve-title-profile-test"
base_url = "https://profile.invalid/v1"
model = "profile-title"
api_key_env = "`+key+`"
`, 0o644)

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{
		Sink:          bc,
		WorkspaceRoot: root,
		SessionDir:    profilePaths.ProjectSessionDir(root),
		Actor:         actor,
	})
	s := New(ctrl, bc, config.ServeConfig{AuthMode: "token", Token: "secret"})

	titleProv, ok := s.titleProv.(serveTitleProfileProvider)
	if !ok {
		t.Fatalf("title provider = %T, want serveTitleProfileProvider", s.titleProv)
	}
	if titleProv.cfg.BaseURL != "https://profile.invalid/v1" || titleProv.cfg.Model != "profile-title" {
		t.Fatalf("title provider config = %+v, want workspace provider config", titleProv.cfg)
	}
	if titleProv.cfg.APIKey != "from_profile" {
		t.Fatalf("title provider API key = %q, want current profile credential", titleProv.cfg.APIKey)
	}
	if got := os.Getenv(key); got != "from_profile" {
		t.Fatalf("process credential = %q, want current profile credential", got)
	}
}

func TestInitTitleProviderFallsBackToConfigDefaultWhenDefaultModelEmpty(t *testing.T) {
	registerServeTitleProfileProvider()
	isolateServeSwitchConfigHome(t)
	t.Setenv("KWU_CREDENTIALS_STORE", config.CredentialsStoreFile)
	root := t.TempDir()
	t.Chdir(t.TempDir())
	key := "KEY_SERVE_TITLE_KWU"
	t.Setenv(key, "")
	os.Unsetenv(key)
	defaultRef := config.Default().DefaultModel
	defaultProvider, defaultModel, ok := strings.Cut(defaultRef, "/")
	if !ok || defaultProvider == "" || defaultModel == "" {
		t.Fatalf("config default model ref = %q, want provider/model", defaultRef)
	}

	actor := userctx.ActorContext{
		ActorID:       "local",
		ProfileID:     "profile-title-fallback",
		AuthMethod:    "local",
		WorkspaceRoot: root,
	}
	profilePaths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(profilePaths.UserCredentialsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePaths.UserCredentialsPath(), []byte(key+"=from_profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(`
default_model = ""

[[providers]]
name = "`+defaultProvider+`"
kind = "serve-title-profile-test"
base_url = "https://kwu.invalid/v1"
model = "`+defaultModel+`"
api_key_env = "`+key+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{
		Sink:          bc,
		WorkspaceRoot: root,
		SessionDir:    profilePaths.ProjectSessionDir(root),
		Actor:         actor,
	})
	s := New(ctrl, bc, config.ServeConfig{AuthMode: "token", Token: "secret"})

	titleProv, ok := s.titleProv.(serveTitleProfileProvider)
	if !ok {
		t.Fatalf("title provider = %T, want serveTitleProfileProvider", s.titleProv)
	}
	if titleProv.cfg.Name != defaultProvider || titleProv.cfg.Model != defaultModel {
		t.Fatalf("title provider config = %+v, want %s", titleProv.cfg, defaultRef)
	}
	if titleProv.cfg.APIKey != "from_profile" {
		t.Fatalf("title provider API key = %q, want current profile credential", titleProv.cfg.APIKey)
	}
}

func TestInitTitleProviderCreatesDefaultKWUOpenAIProvider(t *testing.T) {
	isolateServeSwitchConfigHome(t)
	t.Setenv("KWU_CREDENTIALS_STORE", config.CredentialsStoreFile)
	root := t.TempDir()
	t.Chdir(t.TempDir())

	actor := userctx.ActorContext{
		ActorID:       "local",
		ProfileID:     "profile-title-kwu-openai",
		AuthMethod:    "local",
		WorkspaceRoot: root,
	}
	profilePaths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(profilePaths.UserCredentialsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePaths.UserCredentialsPath(), []byte("KWU_API_KEY=from_profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	bc := NewBroadcaster()
	ctrl := control.New(control.Options{
		Sink:          bc,
		WorkspaceRoot: root,
		SessionDir:    profilePaths.ProjectSessionDir(root),
		Actor:         actor,
	})
	s := New(ctrl, bc, config.ServeConfig{AuthMode: "token", Token: "secret"})

	if s.titleProv == nil {
		t.Fatalf("title provider was not created for default %s", config.Default().DefaultModel)
	}
	if got := s.titleProv.Name(); got != "kwu" {
		t.Fatalf("title provider name = %q, want kwu", got)
	}
}

func TestInitTitleProviderIsFailClosedInB2BFakeMode(t *testing.T) {
	registerServeTitleProfileProvider()
	isolateServeSwitchConfigHome(t)
	t.Setenv("KWU_CLOUD_FAKE", "1")
	root := t.TempDir()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-title-fake", AuthMethod: "local", WorkspaceRoot: root}
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`
default_model = "title-provider/title"

[[providers]]
name = "title-provider"
kind = "serve-title-profile-test"
base_url = "https://title.invalid/v1"
model = "title"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc, WorkspaceRoot: root, SessionDir: paths.ProjectSessionDir(root), Actor: actor})
	s := New(ctrl, bc, config.ServeConfig{})
	if _, ok := s.titleProv.(provider.FailClosedProvider); !ok {
		t.Fatalf("title provider = %T, want fail-closed provider", s.titleProv)
	}
	if _, err := s.titleProv.Stream(context.Background(), provider.Request{}); !provider.IsContractInvalid(err) {
		t.Fatalf("title provider error = %v", err)
	}
}

// TestControllerAccessorIsRaceSafe guards the switchModel concurrency contract:
// handlers read the controller through ctl() while a swap runs under the write
// lock. With the lock removed this fails under `go test -race` (the CI race job).
func TestControllerAccessorIsRaceSafe(t *testing.T) {
	a, b := &control.Controller{}, &control.Controller{}
	s := &Server{ctrl: a}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := s.ctl(); got != a && got != b {
				t.Errorf("ctl() returned a pointer that was never set")
			}
		}()
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.mu.Lock()
			if s.ctrl == a {
				s.ctrl = b
			} else {
				s.ctrl = a
			}
			s.mu.Unlock()
		}()
	}
	wg.Wait()
}

func TestSwitchModelPreservesActorContext(t *testing.T) {
	registerServeSwitchActorProvider()
	isolateServeSwitchConfigHome(t)
	root := t.TempDir()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(`
default_model = "old/old-model"

[[providers]]
name = "old"
kind = "serve-switch-actor-test"
base_url = "https://example.invalid"
model = "old-model"

[[providers]]
name = "new"
kind = "serve-switch-actor-test"
base_url = "https://example.invalid"
model = "new-model"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	bc := NewBroadcaster()
	actor := userctx.ActorContext{
		ActorID:    "actor-serve",
		ProfileID:  "profile-serve",
		AuthMethod: "oauth",
	}
	sessionDir := config.NewProfilePathResolver(actor).ProjectSessionDir(root)
	ctrl := control.New(control.Options{
		Sink:          bc,
		WorkspaceRoot: root,
		SessionDir:    sessionDir,
		Actor:         actor,
	})
	s := &Server{ctrl: ctrl, bc: bc}

	if err := s.switchModel(context.Background(), "new/new-model"); err != nil {
		t.Fatalf("switchModel: %v", err)
	}
	defer s.ctl().Close()

	gotActor := s.ctl().ActorContext()
	if gotActor.ActorID != "actor-serve" || gotActor.ProfileID != "profile-serve" || gotActor.AuthMethod != "oauth" {
		t.Fatalf("switched ActorContext() = %+v", gotActor)
	}
	if filepath.Clean(gotActor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", gotActor.WorkspaceRoot, root)
	}
	if filepath.Clean(s.ctl().SessionDir()) != filepath.Clean(sessionDir) {
		t.Fatalf("SessionDir = %q, want %q", s.ctl().SessionDir(), sessionDir)
	}
}

func TestSwitchEffortWritesCurrentProfileConfig(t *testing.T) {
	registerServeSwitchActorProvider()
	isolateServeSwitchConfigHome(t)
	root := t.TempDir()
	t.Chdir(t.TempDir())
	actor := userctx.ActorContext{
		ActorID:    "serve",
		ProfileID:  "profile-serve",
		AuthMethod: "serve-token",
	}
	profilePaths := config.NewProfilePathResolver(actor)
	defaultPaths := config.NewProfilePathResolver(userctx.Default(""))
	writeConfig := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte(`
default_model = "local/model"

[[providers]]
name = "local"
kind = "serve-switch-actor-test"
base_url = "https://example.invalid"
model = "model"
models = ["model"]
supported_efforts = ["low", "high"]
`)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig(profilePaths.UserConfigPath())
	writeConfig(defaultPaths.UserConfigPath())

	sessionDir := profilePaths.ProjectSessionDir(root)
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{
		Label:         "local/model",
		Sink:          bc,
		WorkspaceRoot: root,
		SessionDir:    sessionDir,
		Actor:         actor,
	})
	s := &Server{ctrl: ctrl, bc: bc}

	if err := s.switchEffort(context.Background(), "high"); err != nil {
		t.Fatalf("switchEffort: %v", err)
	}
	defer s.ctl().Close()

	profileCfg := config.LoadForEdit(profilePaths.UserConfigPath())
	profileProvider, ok := profileCfg.Provider("local")
	if !ok {
		t.Fatal("profile provider missing")
	}
	if profileProvider.Effort != "high" {
		t.Fatalf("profile effort = %q, want high", profileProvider.Effort)
	}
	defaultCfg := config.LoadForEdit(defaultPaths.UserConfigPath())
	defaultProvider, ok := defaultCfg.Provider("local")
	if !ok {
		t.Fatal("default provider missing")
	}
	if defaultProvider.Effort != "" {
		t.Fatalf("default profile effort = %q, want unchanged empty", defaultProvider.Effort)
	}
	gotActor := s.ctl().ActorContext()
	if gotActor.ActorID != "serve" || gotActor.ProfileID != "profile-serve" || gotActor.AuthMethod != "serve-token" {
		t.Fatalf("switched ActorContext() = %+v", gotActor)
	}
	if filepath.Clean(gotActor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", gotActor.WorkspaceRoot, root)
	}
	if filepath.Clean(s.ctl().SessionDir()) != filepath.Clean(sessionDir) {
		t.Fatalf("SessionDir = %q, want %q", s.ctl().SessionDir(), sessionDir)
	}
}
