package cloud

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/config"
	"kwu/internal/userctx"
)

func TestDurableBindingRepositoryRestoresAndIsolatesProfiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	actorA := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	actorB := userctx.ActorContext{ActorID: "actor-b", ProfileID: "profile-b"}

	first := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	if _, err := first.ConfirmBinding(ctx, actorA, "usr-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.ConfirmBinding(ctx, actorA, "usr-other"); err == nil {
		t.Fatal("different cloud user did not require explicit rebind")
	} else {
		assertProfileScopeMismatch(t, err)
	}
	afterCandidate := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	unchanged, err := afterCandidate.ResolveBinding(ctx, actorA)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != BindingBound || unchanged.CloudUserID != "usr-a" || unchanged.BindingGeneration != 1 {
		t.Fatalf("binding changed after candidate = %+v", unchanged)
	}
	if _, err := afterCandidate.Rebind(ctx, actorA, "usr-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.ConfirmBinding(ctx, actorB, "usr-c"); err != nil {
		t.Fatal(err)
	}

	restarted := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	gotA, err := restarted.ResolveBinding(ctx, actorA)
	if err != nil {
		t.Fatal(err)
	}
	if gotA.CloudUserID != "usr-b" || gotA.BindingGeneration != 2 || gotA.Status != BindingBound {
		t.Fatalf("restored profile-a binding = %+v", gotA)
	}
	gotB, err := restarted.ResolveBinding(ctx, actorB)
	if err != nil {
		t.Fatal(err)
	}
	if gotB.CloudUserID != "usr-c" || gotB.BindingGeneration != 1 || gotB.Status != BindingBound {
		t.Fatalf("restored profile-b binding = %+v", gotB)
	}

	otherActor := userctx.ActorContext{ActorID: "actor-other", ProfileID: actorA.ProfileID}
	if binding, err := restarted.ResolveBinding(ctx, otherActor); !errors.Is(err, ErrCredentialStateInvalid) ||
		binding.Status != BindingExpired || binding.CloudUserID != "" {
		t.Fatal("another actor read profile binding")
	}
}

func TestDurableSessionStoreRestartsWithCompleteScopeIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	actorA := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	actorB := userctx.ActorContext{ActorID: "actor-b", ProfileID: "profile-b"}

	bindings := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	bindingA, err := bindings.ConfirmBinding(ctx, actorA, "usr-a")
	if err != nil {
		t.Fatal(err)
	}
	bindingB, err := bindings.ConfirmBinding(ctx, actorB, "usr-b")
	if err != nil {
		t.Fatal(err)
	}
	scopeA, _ := bindingA.Scope()
	scopeB, _ := bindingB.Scope()
	store := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
	repository := NewProfileSessionRepository(bindings, store)
	secretA := "access-A-DO-NOT-LOG"
	refreshA := "refresh-A-DO-NOT-LOG"
	if err := repository.Save(ctx, actorA, Session{Scope: scopeA, State: SessionAuthenticated, AccessToken: secretA, RefreshToken: refreshA}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, actorB, Session{Scope: scopeB, State: SessionAuthenticated, AccessToken: "access-b", RefreshToken: "refresh-b"}); err != nil {
		t.Fatal(err)
	}
	assertSecretOnlyInFile(t, root, secretA, filepath.Join(provider(actorA).MemoryUserDir(), "cloud", "v1", "sessions.json"))
	assertSecretOnlyInFile(t, root, refreshA, filepath.Join(provider(actorA).MemoryUserDir(), "cloud", "v1", "sessions.json"))

	restartedBindings := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	restarted := NewProfileSessionRepository(restartedBindings, NewDurableCloudSessionStore(provider, testEnvironmentIdentity()))
	got, ok, err := restarted.Load(ctx, actorA)
	if err != nil || !ok {
		t.Fatalf("restart load failed: ok=%t err=%v", ok, err)
	}
	if got.Scope != scopeA || got.AccessToken != secretA || got.RefreshToken != refreshA {
		t.Fatal("restart did not restore the exact profile-scoped session")
	}
	if _, err := restartedBindings.Rebind(ctx, actorA, "usr-new"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := restarted.Load(ctx, actorA); err != nil || ok {
		t.Fatalf("old generation visible after rebind: ok=%t err=%v", ok, err)
	}
	if gotB, ok, err := restarted.Load(ctx, actorB); err != nil || !ok || gotB.Scope != scopeB {
		t.Fatalf("profile-b session lost or crossed scope: ok=%t err=%v", ok, err)
	}
	if err := restarted.Clear(ctx, actorB); err != nil {
		t.Fatal(err)
	}
	afterClear := NewProfileSessionRepository(NewDurableBindingRepository(provider, testEnvironmentIdentity()), NewDurableCloudSessionStore(provider, testEnvironmentIdentity()))
	if _, ok, err := afterClear.Load(ctx, actorB); err != nil || ok {
		t.Fatalf("cleared profile-b session restored after restart: ok=%t err=%v", ok, err)
	}
}

func TestDurableSessionStoreRejectsLegacyTokensWithoutRewriting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	scope, _ := NewBindingScope(actor, "usr-a", 1)
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "sessions.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	v1 := `{"schema":"reasonix.cloud.sessions","version":1,"profile_id":"profile-a","sessions":[{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"state":"authenticated","access_token":"legacy-access","refresh_token":"legacy-refresh","expires_at":"2026-07-13T04:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}

	session, ok, err := NewDurableCloudSessionStore(provider, testEnvironmentIdentity()).Load(ctx, scope)
	if !errors.Is(err, ErrLegacyCredentialState) || ok {
		t.Fatalf("legacy load = %#v, %t, %v", session, ok, err)
	}
	if session.AccessToken != "" || session.RefreshToken != "" {
		t.Fatalf("legacy load exposed token: %#v", session)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != v1 {
		t.Fatalf("legacy document was rewritten: %s", contents)
	}
}

func TestDurableRepositoriesRejectCorruptAndUnknownJSON(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	resolver := provider(actor)
	cloudDir := filepath.Join(resolver.MemoryUserDir(), "cloud", "v1")
	if err := os.MkdirAll(cloudDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		path string
		data string
		load func() error
	}{
		{
			name: "binding empty",
			path: filepath.Join(cloudDir, "binding.json"),
			data: "",
			load: func() error {
				_, err := NewDurableBindingRepository(provider, testEnvironmentIdentity()).ResolveBinding(ctx, actor)
				return err
			},
		},
		{
			name: "binding unknown field",
			path: filepath.Join(cloudDir, "binding.json"),
			data: `{"schema":"reasonix.cloud.binding","version":1,"profile_id":"profile-a","binding":{"actor_id":"actor-a","profile_id":"profile-a","cloud_user_id":"usr-a","binding_status":"bound","binding_generation":1},"unexpected":true}`,
			load: func() error {
				_, err := NewDurableBindingRepository(provider, testEnvironmentIdentity()).ResolveBinding(ctx, actor)
				return err
			},
		},
		{
			name: "binding unknown version",
			path: filepath.Join(cloudDir, "binding.json"),
			data: `{"schema":"reasonix.cloud.binding","version":2,"profile_id":"profile-a","binding":{"actor_id":"actor-a","profile_id":"profile-a","cloud_user_id":"usr-a","binding_status":"bound","binding_generation":1}}`,
			load: func() error {
				_, err := NewDurableBindingRepository(provider, testEnvironmentIdentity()).ResolveBinding(ctx, actor)
				return err
			},
		},
		{
			name: "binding trailing JSON",
			path: filepath.Join(cloudDir, "binding.json"),
			data: `{"schema":"reasonix.cloud.binding","version":1,"profile_id":"profile-a","binding":{"actor_id":"actor-a","profile_id":"profile-a","cloud_user_id":"usr-a","binding_status":"bound","binding_generation":1}} {}`,
			load: func() error {
				_, err := NewDurableBindingRepository(provider, testEnvironmentIdentity()).ResolveBinding(ctx, actor)
				return err
			},
		},
		{
			name: "binding envelope profile mismatch",
			path: filepath.Join(cloudDir, "binding.json"),
			data: `{"schema":"reasonix.cloud.binding","version":1,"profile_id":"profile-b","binding":{"actor_id":"actor-a","profile_id":"profile-a","cloud_user_id":"usr-a","binding_status":"bound","binding_generation":1}}`,
			load: func() error {
				_, err := NewDurableBindingRepository(provider, testEnvironmentIdentity()).ResolveBinding(ctx, actor)
				return err
			},
		},
		{
			name: "binding entry profile mismatch",
			path: filepath.Join(cloudDir, "binding.json"),
			data: `{"schema":"reasonix.cloud.binding","version":1,"profile_id":"profile-a","binding":{"actor_id":"actor-a","profile_id":"profile-b","cloud_user_id":"usr-a","binding_status":"bound","binding_generation":1}}`,
			load: func() error {
				_, err := NewDurableBindingRepository(provider, testEnvironmentIdentity()).ResolveBinding(ctx, actor)
				return err
			},
		},
		{
			name: "binding entry actor mismatch",
			path: filepath.Join(cloudDir, "binding.json"),
			data: `{"schema":"reasonix.cloud.binding","version":1,"profile_id":"profile-a","binding":{"actor_id":"actor-other","profile_id":"profile-a","cloud_user_id":"usr-a","binding_status":"bound","binding_generation":1}}`,
			load: func() error {
				_, err := NewDurableBindingRepository(provider, testEnvironmentIdentity()).ResolveBinding(ctx, actor)
				return err
			},
		},
		{
			name: "session truncated",
			path: filepath.Join(cloudDir, "sessions.json"),
			data: `{"schema":"reasonix.cloud.sessions","version":1,"profile_id":"profile-a","sessions":[`,
			load: func() error {
				scope, _ := NewBindingScope(actor, "usr-a", 1)
				_, _, err := NewDurableCloudSessionStore(provider, testEnvironmentIdentity()).Load(ctx, scope)
				return err
			},
		},
		{
			name: "session entry scope mismatch",
			path: filepath.Join(cloudDir, "sessions.json"),
			data: `{"schema":"reasonix.cloud.sessions","version":1,"profile_id":"profile-a","sessions":[{"scope":{"profile_id":"profile-b","cloud_user_id":"usr-a","binding_generation":1},"state":"authenticated","access_token":"access","refresh_token":"refresh"}]}`,
			load: func() error {
				scope, _ := NewBindingScope(actor, "usr-a", 1)
				_, _, err := NewDurableCloudSessionStore(provider, testEnvironmentIdentity()).Load(ctx, scope)
				return err
			},
		},
		{
			name: "session envelope profile mismatch",
			path: filepath.Join(cloudDir, "sessions.json"),
			data: `{"schema":"reasonix.cloud.sessions","version":1,"profile_id":"profile-b","sessions":[]}`,
			load: func() error {
				scope, _ := NewBindingScope(actor, "usr-a", 1)
				_, _, err := NewDurableCloudSessionStore(provider, testEnvironmentIdentity()).Load(ctx, scope)
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(tc.path, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tc.load(); err == nil {
				t.Fatal("corrupt durable state was treated as missing")
			}
		})
	}
}

func TestDurableSessionStoreIOErrorDoesNotExposeTokens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	profileRoot := provider(actor).MemoryUserDir()
	if err := os.MkdirAll(filepath.Dir(profileRoot), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profileRoot, []byte("path collision"), 0o600); err != nil {
		t.Fatal(err)
	}
	scope, _ := NewBindingScope(actor, "usr-a", 1)
	store := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
	accessSecret := "ACCESS-TOKEN-THAT-MUST-NOT-APPEAR"
	refreshSecret := "REFRESH-TOKEN-THAT-MUST-NOT-APPEAR"

	err := store.Save(ctx, Session{Scope: scope, State: SessionAuthenticated, AccessToken: accessSecret, RefreshToken: refreshSecret})
	if err == nil {
		t.Fatal("session save unexpectedly succeeded through a file path")
	}
	if strings.Contains(err.Error(), accessSecret) || strings.Contains(err.Error(), refreshSecret) {
		t.Fatal("durable session I/O error exposed token")
	}
}

func TestDurableSessionStoreConcurrentScopesDoNotLoseUpdates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	store := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())

	const count = 12
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for i := 1; i <= count; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			scope, err := NewBindingScope(actor, "usr-"+string(rune('a'+i-1)), uint64(i))
			if err != nil {
				errCh <- err
				return
			}
			errCh <- store.Save(ctx, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh"})
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	restarted := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
	for i := 1; i <= count; i++ {
		scope, _ := NewBindingScope(actor, "usr-"+string(rune('a'+i-1)), uint64(i))
		if _, ok, err := restarted.Load(ctx, scope); err != nil || !ok {
			t.Fatalf("scope %d missing after concurrent saves: ok=%t err=%v", i, ok, err)
		}
	}
}

func testProfilePathProvider(root string) ProfilePathProvider {
	return func(actor userctx.ActorContext) config.ProfilePathResolver {
		return config.NewProfilePathResolverWithOptions(config.ProfilePathResolverOptions{
			Actor:     actor,
			HomeRoot:  filepath.Join(root, "home"),
			StateRoot: filepath.Join(root, "state"),
			CacheRoot: filepath.Join(root, "cache"),
		})
	}
}

func testEnvironmentIdentity() EnvironmentIdentity {
	identity, err := NewEnvironmentIdentity(EnvironmentFixture, "https://fixture.reasonix.invalid/")
	if err != nil {
		panic(err)
	}
	return identity
}

func TestDurableRepositoryCanceledContextDoesNotMutate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	repository := NewDurableBindingRepository(testProfilePathProvider(t.TempDir()), testEnvironmentIdentity())
	if _, err := repository.ConfirmBinding(ctx, actor, "usr-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ConfirmBinding error = %v", err)
	}
}

func TestAuthApplicationAcceptsDurableBindingStore(t *testing.T) {
	t.Parallel()
	provider := testProfilePathProvider(t.TempDir())
	bindings := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	store := NewDurableCloudSessionStore(provider, testEnvironmentIdentity())
	sessions := NewProfileSessionRepository(bindings, store)
	captcha, err := NewMockCaptchaGateway("durable-captcha", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions,
		Cleanup: NewIdentityCleanup(store, noopScopedStore{}, noopScopedStore{}),
		Captcha: captcha, Encryptor: NewTestPasswordEncryptor(),
		Client: fixtureErrorClient{err: errors.New("unused")}, FixtureVersion: A0FixtureVersion,
	})
	if err != nil {
		t.Fatalf("NewAuthApplication with durable binding store: %v", err)
	}
}

func TestAuthApplicationRejectsNonComparableBindingStoreWithoutPanic(t *testing.T) {
	t.Parallel()
	bindings := nonComparableBindingStore{}
	sessions := NewProfileSessionRepository(bindings, NewMemorySessionStore())
	captcha, err := NewMockCaptchaGateway("non-comparable", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("NewAuthApplication panicked for an injected BindingStore: %v", recovered)
		}
	}()
	_, err = NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions,
		Cleanup: NewIdentityCleanup(sessions.sessions, noopScopedStore{}, noopScopedStore{}),
		Captcha: captcha, Encryptor: NewTestPasswordEncryptor(),
		Client: fixtureErrorClient{err: errors.New("unused")}, FixtureVersion: A0FixtureVersion,
	})
	if err == nil {
		t.Fatal("non-identity-comparable binding store was accepted")
	}
}

type nonComparableBindingStore []ProfileBinding

func (nonComparableBindingStore) ResolveBinding(context.Context, userctx.ActorContext) (ProfileBinding, error) {
	return ProfileBinding{}, nil
}

func (nonComparableBindingStore) ConfirmBinding(context.Context, userctx.ActorContext, string) (ProfileBinding, error) {
	return ProfileBinding{}, nil
}

func (nonComparableBindingStore) Rebind(context.Context, userctx.ActorContext, string) (ProfileBinding, error) {
	return ProfileBinding{}, nil
}

func (nonComparableBindingStore) CompareAndRebind(context.Context, userctx.ActorContext, ProfileBinding, string) (ProfileBinding, error) {
	return ProfileBinding{}, nil
}

func assertSecretOnlyInFile(t *testing.T, root, secret, expectedPath string) {
	t.Helper()
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), secret) {
			matches = append(matches, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0] != filepath.Clean(expectedPath) {
		t.Fatalf("secret persisted outside the profile session file: paths=%v", matches)
	}
}
