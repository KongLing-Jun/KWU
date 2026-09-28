package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestEnvironmentIdentityFixedVectorsAndEquivalentURLs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		class   EnvironmentClass
		url     string
		wantURL string
		wantID  string
	}{
		{"production", EnvironmentProduction, "HTTPS://WWW.SUIYUAN-AI.COM:443", "https://www.suiyuan-ai.com/", "62bc6c3e4dc70d60e583372f5e86fb07bc5d327dcc49e18d3283dbcac35dee33"},
		{"development", EnvironmentDevelopment, "http://192.168.10.243:8080", "http://192.168.10.243:8080/", "09fa1c536599ef31f96de97f6c55d10e262593cce6b7f4950ec707f263c09ad7"},
		{"fixture", EnvironmentFixture, "https://fixture.reasonix.invalid", "https://fixture.reasonix.invalid/", "859814bae527e03d0b1313ef917ac1ea402a791371ef1ea70f454387f33eafa1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewEnvironmentIdentity(test.class, test.url)
			if err != nil {
				t.Fatal(err)
			}
			if got.CloudBaseURL != test.wantURL || got.EnvironmentID != test.wantID {
				t.Fatalf("identity = %#v", got)
			}
		})
	}
	prod, _ := NewEnvironmentIdentity(EnvironmentProduction, "https://www.suiyuan-ai.com/")
	dev, _ := NewEnvironmentIdentity(EnvironmentDevelopment, "https://www.suiyuan-ai.com/")
	devA, _ := NewEnvironmentIdentity(EnvironmentDevelopment, "http://dev-a.example/")
	devB, _ := NewEnvironmentIdentity(EnvironmentDevelopment, "http://dev-b.example/")
	if prod == dev || devA == devB {
		t.Fatal("build class or development URL did not isolate environment identity")
	}
}

func TestDurableCredentialsRejectLegacyMismatchAndInvalidWithoutReturningTokens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	scope, _ := NewBindingScope(actor, "usr-a", 1)
	production, _ := NewEnvironmentIdentity(EnvironmentProduction, "https://prod.example/")
	development, _ := NewEnvironmentIdentity(EnvironmentDevelopment, "https://dev.example/")

	for _, test := range []struct {
		name string
		data string
		want error
	}{
		{"legacy", `{"schema":"reasonix.cloud.sessions","version":2,"profile_id":"profile-a","sessions":[{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"state":"authenticated","access_token":"legacy-access","refresh_token":"legacy-refresh"}]}`, ErrLegacyCredentialState},
		{"unknown", `{"schema":"reasonix.cloud.sessions","version":99,"profile_id":"profile-a","sessions":[]}`, ErrCredentialStateInvalid},
		{"corrupt", `{"schema":`, ErrCredentialStateInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			provider := testProfilePathProvider(root)
			path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "sessions.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			session, found, err := NewDurableCloudSessionStore(provider, production).Load(ctx, scope)
			if !errors.Is(err, test.want) || found || session.AccessToken != "" || session.RefreshToken != "" {
				t.Fatalf("load = %#v, %t, %v", session, found, err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != test.data {
				t.Fatal("rejected credential document was rewritten")
			}
		})
	}

	root := t.TempDir()
	provider := testProfilePathProvider(root)
	store := NewDurableCloudSessionStore(provider, development)
	session := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "dev-access", RefreshToken: "dev-refresh"}
	if err := store.Save(ctx, session); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := NewDurableCloudSessionStore(provider, production).Load(ctx, scope)
	if !errors.Is(err, ErrEnvironmentMismatch) || found || loaded.AccessToken != "" {
		t.Fatalf("cross-environment load = %#v, %t, %v", loaded, found, err)
	}
}

func TestDurableSessionV3RejectsInvalidTerminationMetadataAndIncompleteOfflineCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	scope, _ := NewBindingScope(actor, "usr-a", 1)
	environment := testEnvironmentIdentity()
	now := time.Date(2026, 7, 27, 4, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		session Session
	}{
		{
			name: "authenticated termination residue",
			session: Session{
				Scope: scope, State: SessionAuthenticated, AccessToken: "access",
				TerminationReason: ErrorAuthRequired, TerminatedAt: now,
			},
		},
		{
			name: "offline termination residue",
			session: Session{
				Scope: scope, State: SessionOffline, AccessToken: "access",
				TerminationReason: ErrorAuthRequired, TerminatedAt: now,
			},
		},
		{
			name: "offline incomplete production metadata",
			session: Session{
				Scope: scope, State: SessionOffline, AccessToken: "must-not-leave", DeviceID: "device-01",
				AccessTokenExpiresAt: now.Add(time.Hour),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			provider := testProfilePathProvider(root)
			path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "sessions.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			document, err := json.Marshal(durableSessionsDocument{
				Schema: durableSessionsSchema, Version: durableSessionsVersion,
				ProfileID: actor.ProfileID, Environment: environment, Sessions: []Session{test.session},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, document, 0o600); err != nil {
				t.Fatal(err)
			}
			loaded, found, err := NewDurableCloudSessionStore(provider, environment).Load(ctx, scope)
			if !errors.Is(err, ErrCredentialStateInvalid) || found || loaded.AccessToken != "" {
				t.Fatalf("invalid v3 load = %#v, %t, %v", loaded, found, err)
			}
			var rejected *CredentialStateError
			if !errors.As(err, &rejected) || rejected.Code != ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID") {
				t.Fatalf("typed rejection = %#v, %v", rejected, err)
			}
			if test.name == "offline incomplete production metadata" {
				executor, newErr := NewAuthenticatedRequestExecutor(
					NewDurableCloudSessionStore(provider, environment), &concurrentSessionClient{}, func() time.Time { return now },
				)
				if newErr != nil {
					t.Fatal(newErr)
				}
				requestCalls := 0
				doErr := executor.Do(ctx, scope, func(context.Context, string) error {
					requestCalls++
					return nil
				})
				if !errors.Is(doErr, ErrCredentialStateInvalid) || requestCalls != 0 {
					t.Fatalf("executor error=%v requestCalls=%d", doErr, requestCalls)
				}
			}
		})
	}
}

func TestDurableBindingV2RejectsActorMismatchAndNonBoundStatesWithoutIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	environment := testEnvironmentIdentity()
	tests := []struct {
		name    string
		binding ProfileBinding
	}{
		{
			name: "actor mismatch",
			binding: ProfileBinding{
				ActorID: "actor-other", ProfileID: actor.ProfileID, CloudUserID: "secret-user",
				Status: BindingBound, BindingGeneration: 1,
			},
		},
		{
			name: "expired",
			binding: ProfileBinding{
				ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: "secret-user",
				Status: BindingExpired, BindingGeneration: 1,
			},
		},
		{
			name: "mismatch",
			binding: ProfileBinding{
				ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: "secret-user",
				Status: BindingMismatch, BindingGeneration: 1,
			},
		},
		{
			name:    "unbound",
			binding: ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: BindingUnbound},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := testProfilePathProvider(t.TempDir())
			path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "binding.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			document, err := json.Marshal(durableBindingDocument{
				Schema: durableBindingSchema, Version: durableBindingVersion,
				ProfileID: actor.ProfileID, Environment: environment, Binding: test.binding,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, document, 0o600); err != nil {
				t.Fatal(err)
			}
			binding, err := NewDurableBindingRepository(provider, environment).ResolveBinding(ctx, actor)
			if !errors.Is(err, ErrCredentialStateInvalid) || binding.Status != BindingExpired ||
				binding.CloudUserID != "" {
				t.Fatalf("invalid v2 binding = %#v, %v", binding, err)
			}
			var rejected *CredentialStateError
			if !errors.As(err, &rejected) || rejected.Code != ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID") {
				t.Fatalf("typed rejection = %#v, %v", rejected, err)
			}
		})
	}
}

func TestRejectedDurableCredentialsCanOnlyBeReplacedBySuccessfulAuthentication(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	environment := testEnvironmentIdentity()
	cloudDir := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1")
	if err := os.MkdirAll(cloudDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyBinding := `{"schema":"reasonix.cloud.binding","version":1,"profile_id":"default","binding":{"actor_id":"local","profile_id":"default","cloud_user_id":"usr_old","binding_status":"bound","binding_generation":7}}`
	legacySessions := `{"schema":"reasonix.cloud.sessions","version":2,"profile_id":"default","sessions":[{"scope":{"profile_id":"default","cloud_user_id":"usr_old","binding_generation":7},"state":"authenticated","access_token":"old-access","refresh_token":"old-refresh"}]}`
	if err := os.WriteFile(filepath.Join(cloudDir, "binding.json"), []byte(legacyBinding), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloudDir, "sessions.json"), []byte(legacySessions), 0o600); err != nil {
		t.Fatal(err)
	}

	bindings := NewDurableBindingRepository(provider, environment)
	sessionStore := NewDurableCloudSessionStore(provider, environment)
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	captcha, _ := NewMockCaptchaGateway("credential-replacement", time.Now().Add(time.Hour))
	app, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions,
		Cleanup: NewIdentityCleanup(sessionStore, noopScopedStore{}, noopScopedStore{}),
		Captcha: captcha, Encryptor: NewTestPasswordEncryptor(),
		Client: client, FixtureVersion: A0FixtureVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, recoverErr := app.Recover(ctx, actor)
	if !errors.Is(recoverErr, ErrLegacyCredentialState) || view.State != AccountExpired || view.Binding.CloudUserID != "" {
		t.Fatalf("rejected recovery = %#v, %v", view, recoverErr)
	}
	bindingBefore, _ := os.ReadFile(filepath.Join(cloudDir, "binding.json"))
	sessionsBefore, _ := os.ReadFile(filepath.Join(cloudDir, "sessions.json"))
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "wrong-password"}); err == nil {
		t.Fatal("failed authentication unexpectedly succeeded")
	}
	bindingAfter, _ := os.ReadFile(filepath.Join(cloudDir, "binding.json"))
	sessionsAfter, _ := os.ReadFile(filepath.Join(cloudDir, "sessions.json"))
	if string(bindingAfter) != string(bindingBefore) || string(sessionsAfter) != string(sessionsBefore) {
		t.Fatal("failed authentication replaced rejected credentials")
	}
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	binding, err := bindings.ResolveBinding(ctx, actor)
	if err != nil || binding.Status != BindingBound || binding.BindingGeneration != 8 {
		t.Fatalf("replacement binding = %#v, %v", binding, err)
	}
	session, found, err := sessions.Load(ctx, actor)
	if err != nil || !found || session.State != SessionAuthenticated || strings.Contains(session.AccessToken, "old") {
		t.Fatalf("replacement session = %#v, %t, %v", session, found, err)
	}
	contents, _ := os.ReadFile(filepath.Join(cloudDir, "sessions.json"))
	if strings.Contains(string(contents), "old-access") || !strings.Contains(string(contents), `"version":3`) {
		t.Fatalf("replacement document = %s", contents)
	}
}

func TestCorruptDurableCredentialsRecoverOnlyAfterSuccessfulAuthentication(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	environment := testEnvironmentIdentity()
	cloudDir := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1")
	if err := os.MkdirAll(cloudDir, 0o700); err != nil {
		t.Fatal(err)
	}
	corruptBinding := `{"schema":"reasonix.cloud.binding"`
	corruptSessions := `{"schema":"reasonix.cloud.sessions"`
	bindingPath := filepath.Join(cloudDir, "binding.json")
	sessionPath := filepath.Join(cloudDir, "sessions.json")
	if err := os.WriteFile(bindingPath, []byte(corruptBinding), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte(corruptSessions), 0o600); err != nil {
		t.Fatal(err)
	}

	bindings := NewDurableBindingRepository(provider, environment)
	sessionStore := NewDurableCloudSessionStore(provider, environment)
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	captcha, _ := NewMockCaptchaGateway("corrupt-credential-replacement", time.Now().Add(time.Hour))
	app, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions,
		Cleanup: NewIdentityCleanup(sessionStore, noopScopedStore{}, noopScopedStore{}),
		Captcha: captcha, Encryptor: NewTestPasswordEncryptor(),
		Client: client, FixtureVersion: A0FixtureVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, recoverErr := app.Recover(ctx, actor)
	if !errors.Is(recoverErr, ErrCredentialStateInvalid) || view.State != AccountExpired ||
		view.Binding.CloudUserID != "" {
		t.Fatalf("corrupt recovery = %#v, %v", view, recoverErr)
	}
	if _, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "wrong-password"}); err == nil {
		t.Fatal("failed authentication unexpectedly succeeded")
	}
	afterFailedBinding, _ := os.ReadFile(bindingPath)
	afterFailedSessions, _ := os.ReadFile(sessionPath)
	if string(afterFailedBinding) != corruptBinding || string(afterFailedSessions) != corruptSessions {
		t.Fatal("failed authentication rewrote corrupt credentials")
	}
	loginView, err := app.Login(ctx, actor, LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil || loginView.State != AccountAuthenticated {
		t.Fatalf("successful recovery login = %#v, %v", loginView, err)
	}
	binding, err := bindings.ResolveBinding(ctx, actor)
	if err != nil || binding.Status != BindingBound || binding.CloudUserID != "usr_01" {
		t.Fatalf("replacement binding = %#v, %v", binding, err)
	}
	session, found, err := sessions.Load(ctx, actor)
	if err != nil || !found || session.State != SessionAuthenticated ||
		session.AccessToken == "" || session.RefreshToken == "" {
		t.Fatalf("replacement session = %#v, %t, %v", session, found, err)
	}
	bindingContents, _ := os.ReadFile(bindingPath)
	sessionContents, _ := os.ReadFile(sessionPath)
	if !strings.Contains(string(bindingContents), `"version":2`) ||
		!strings.Contains(string(sessionContents), `"version":3`) {
		t.Fatalf("replacement documents = binding %s sessions %s", bindingContents, sessionContents)
	}
}

func TestRejectedBindingWithoutTrustedGenerationUsesIsolatedReplacementScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "binding.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	unknown := `{"schema":"reasonix.cloud.binding","version":99,"profile_id":"profile-a","environment":{"environment_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","cloud_base_url":"https://old.example/"},"binding":{"actor_id":"actor-a","profile_id":"profile-a","cloud_user_id":"usr-a","binding_status":"bound","binding_generation":1}}`
	if err := os.WriteFile(path, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	expected, err := repository.PrepareRejectedBinding(ctx, actor, "usr-a")
	if err != nil {
		t.Fatal(err)
	}
	if expected.BindingGeneration == 1 || expected.BindingGeneration == 0 {
		t.Fatalf("untrusted generation reused old scope: %#v", expected)
	}
	again, err := repository.PrepareRejectedBinding(ctx, actor, "usr-a")
	if err != nil || again != expected {
		t.Fatalf("replacement scope was not stable: %#v, %v", again, err)
	}
}

func TestRejectedSessionPreventsAuthenticatedRequestFromSendingToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	scope, _ := NewBindingScope(actor, "usr-a", 1)
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "sessions.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	document := `{"schema":"reasonix.cloud.sessions","version":2,"profile_id":"profile-a","sessions":[{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"state":"authenticated","access_token":"must-not-leave","refresh_token":"must-not-leave"}]}`
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &concurrentSessionClient{}
	executor, err := NewAuthenticatedRequestExecutor(NewDurableCloudSessionStore(provider, testEnvironmentIdentity()), client, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	requestCalls := 0
	err = executor.Do(ctx, scope, func(context.Context, string) error {
		requestCalls++
		return nil
	})
	if !errors.Is(err, ErrLegacyCredentialState) || requestCalls != 0 || client.RefreshCalls() != 0 {
		t.Fatalf("executor error=%v requestCalls=%d refreshCalls=%d", err, requestCalls, client.RefreshCalls())
	}
}
