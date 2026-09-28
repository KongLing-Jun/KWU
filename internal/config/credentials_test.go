package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/userctx"
)

func TestStoreCredentialLinesWithPathsDefaultProfileCompatible(t *testing.T) {
	isolateProfilePathEnv(t)
	key := "KEY_CREDENTIAL_DEFAULT_PROFILE"
	t.Setenv(key, "")
	os.Unsetenv(key)

	paths := NewProfilePathResolver(userctx.Default(""))
	target, err := StoreCredentialLinesWithPaths(paths, []string{key + "=from_default"})
	if err != nil {
		t.Fatalf("StoreCredentialLinesWithPaths: %v", err)
	}
	if filepath.Clean(target) != filepath.Clean(UserCredentialsPath()) {
		t.Fatalf("target = %q, want default credentials path %q", target, UserCredentialsPath())
	}
	data, err := os.ReadFile(UserCredentialsPath())
	if err != nil {
		t.Fatalf("read default credentials: %v", err)
	}
	if string(data) != key+"=from_default\n" {
		t.Fatalf("default credentials = %q", data)
	}
	if got := os.Getenv(key); got != "from_default" {
		t.Fatalf("process env = %q, want stored value", got)
	}
	if got := CredentialsTargetDescriptionWithPaths(paths); filepath.Clean(got) != filepath.Clean(UserCredentialsPath()) {
		t.Fatalf("target description = %q, want default credentials path", got)
	}
	if got := CredentialOwnerDescriptionWithPaths(paths); got != "profile default" {
		t.Fatalf("owner description = %q, want profile default", got)
	}
}

func TestSetCredentialWithPathsValidatesSingleKey(t *testing.T) {
	isolateProfilePathEnv(t)
	actor := userctx.Default("")
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)
	key := "KEY_CREDENTIAL_PROFILE_SET"
	t.Setenv(key, "")
	os.Unsetenv(key)

	target, err := SetCredentialWithPaths(paths, key, "from_work")
	if err != nil {
		t.Fatalf("SetCredentialWithPaths: %v", err)
	}
	if filepath.Clean(target) != filepath.Clean(paths.UserCredentialsPath()) {
		t.Fatalf("target = %q, want profile credentials path %q", target, paths.UserCredentialsPath())
	}
	if got := os.Getenv(key); got != "from_work" {
		t.Fatalf("process env = %q, want stored profile value", got)
	}
	if !CredentialStoredWithPaths(paths, key) {
		t.Fatal("profile credential should be stored")
	}

	for _, badKey := range []string{"bad-key", "123_API_KEY"} {
		if _, err := SetCredentialWithPaths(paths, badKey, "value"); err == nil || !strings.Contains(err.Error(), `invalid credential key "`+badKey+`"`) {
			t.Fatalf("invalid key error for %q = %v, want SetCredential-style invalid credential key", badKey, err)
		}
		if CredentialStoredWithPaths(paths, badKey) {
			t.Fatalf("invalid key %q should not be reported as stored", badKey)
		}
	}
	if _, err := SetCredentialWithPaths(paths, "KEY_WITH_NEWLINE", "bad\nvalue"); err == nil || !strings.Contains(err.Error(), "credential value for KEY_WITH_NEWLINE contains a newline") {
		t.Fatalf("newline value error = %v, want SetCredential-style newline error", err)
	}
}

func TestKWUOfficialCredentialFallsBackToLegacyKWUKey(t *testing.T) {
	isolateProfilePathEnv(t)
	paths := NewProfilePathResolver(userctx.Default(""))
	t.Setenv("KWU_API_KEY", "")
	t.Setenv("KWU_API_KEY", "")
	os.Unsetenv("KWU_API_KEY")
	os.Unsetenv("KWU_API_KEY")

	if _, err := StoreCredentialLinesWithPaths(paths, []string{"KWU_API_KEY=legacy-key"}); err != nil {
		t.Fatalf("StoreCredentialLinesWithPaths: %v", err)
	}

	entry := ProviderEntry{Name: "kwu", APIKeyEnv: "KWU_API_KEY"}
	entry.ResolveAPIKeyForRootWithPaths(".", paths)
	if got := entry.APIKey(); got != "legacy-key" {
		t.Fatalf("APIKey() = %q, want legacy fallback", got)
	}
	if !CredentialStoredWithPaths(paths, "KWU_API_KEY") {
		t.Fatal("KWU_API_KEY should be reported stored when legacy KWU_API_KEY exists")
	}
}

func TestProfileCredentialsDoNotReadDefaultEnv(t *testing.T) {
	isolateProfilePathEnv(t)
	project := t.TempDir()
	key := "KEY_PROFILE_ISOLATED"
	t.Setenv(key, "")
	os.Unsetenv(key)

	defaultPaths := NewProfilePathResolver(userctx.Default(project))
	actor := userctx.Default(project)
	actor.ProfileID = "work"
	workPaths := NewProfilePathResolver(actor)

	if err := os.MkdirAll(filepath.Dir(defaultPaths.UserCredentialsPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultPaths.UserCredentialsPath(), []byte(key+"=from_default\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := ResolveCredentialForRootGlobalFirstWithPaths(project, workPaths, key); got.Set {
		t.Fatalf("work profile read default credentials: %+v", got)
	}
	if CredentialStoredWithPaths(workPaths, key) {
		t.Fatal("work profile should not report default profile credential as stored")
	}

	if _, err := StoreCredentialLinesWithPaths(workPaths, []string{key + "=from_work"}); err != nil {
		t.Fatalf("StoreCredentialLinesWithPaths work: %v", err)
	}
	work := ResolveCredentialForRootGlobalFirstWithPaths(project, workPaths, key)
	if !work.Set || work.Value != "from_work" {
		t.Fatalf("work credential = %+v, want profile value", work)
	}
	if filepath.Clean(work.Source.Path) == filepath.Clean(defaultPaths.UserCredentialsPath()) {
		t.Fatalf("work credential source used default path %q", work.Source.Path)
	}
	if !strings.Contains(work.Source.Label, "profile work") {
		t.Fatalf("work source label = %q, want profile owner reference", work.Source.Label)
	}

	def := ResolveCredentialForRootGlobalFirstWithPaths(project, defaultPaths, key)
	if !def.Set || def.Value != "from_default" {
		t.Fatalf("default credential = %+v, want default value", def)
	}
}

func TestCredentialKeySeparatesProfilesForKeyringSlots(t *testing.T) {
	key := "DEEPSEEK_API_KEY"
	defaultSlot := CredentialKey(CredentialOwnerForProfile(userctx.DefaultProfileID), key)
	workSlot := CredentialKey(CredentialOwnerForProfile("work"), key)
	personalSlot := CredentialKey(CredentialOwnerForProfile("personal"), key)

	if defaultSlot == key {
		t.Fatalf("default keyring slot = %q, want owner-qualified slot with legacy fallback handled separately", defaultSlot)
	}
	if workSlot == personalSlot {
		t.Fatalf("profile keyring slots collide: %q", workSlot)
	}
	if !strings.Contains(workSlot, "profile:work:") {
		t.Fatalf("work keyring slot = %q, want profile owner prefix", workSlot)
	}
	if got := CredentialKey(CredentialOwner{Kind: CredentialOwnerWorkspace, ID: "ws"}, key); got != "workspace:ws:"+key {
		t.Fatalf("workspace credential slot = %q", got)
	}
}

func TestKeyringCredentialResolutionUsesProfileOwnerSlots(t *testing.T) {
	isolateProfilePathEnv(t)
	project := t.TempDir()
	key := "DEEPSEEK_API_KEY"
	t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreKeyring)
	t.Setenv(key, "")
	os.Unsetenv(key)

	defaultPaths := NewProfilePathResolver(userctx.Default(project))
	workActor := userctx.Default(project)
	workActor.ProfileID = "work"
	workPaths := NewProfilePathResolver(workActor)
	defaultOwner := CredentialOwnerForPaths(defaultPaths)
	workOwner := CredentialOwnerForPaths(workPaths)

	slots := map[string]string{
		CredentialKey(defaultOwner, key): "default-keyring-value",
		CredentialKey(workOwner, key):    "work-keyring-value",
		key:                              "legacy-keyring-value",
	}
	var calls []CredentialOwner
	oldLookup := keyringCredentialValueLookup
	keyringCredentialValueLookup = func(owner CredentialOwner, envName string) (string, bool) {
		owner = owner.Normalize()
		calls = append(calls, owner)
		if value, ok := slots[CredentialKey(owner, envName)]; ok {
			return value, true
		}
		if owner.isDefaultProfile() {
			value, ok := slots[envName]
			return value, ok
		}
		return "", false
	}
	t.Cleanup(func() { keyringCredentialValueLookup = oldLookup })

	work := ResolveCredentialForRootGlobalFirstWithPaths(project, workPaths, key)
	if !work.Set || work.Value != "work-keyring-value" {
		t.Fatalf("work profile keyring credential = %+v, want owner-qualified work value", work)
	}
	if got := credentialOwnerCallCount(calls, workOwner); got != 1 {
		t.Fatalf("work profile keyring lookup owner calls = %d, want 1", got)
	}
	if got := credentialOwnerCallCount(calls, defaultOwner); got != 0 {
		t.Fatalf("work profile unexpectedly looked up default owner %d time(s)", got)
	}
	if strings.Contains(work.Source.Label, work.Value) {
		t.Fatalf("source label leaked credential value: %q", work.Source.Label)
	}
	if !strings.Contains(work.Source.Label, "profile work") {
		t.Fatalf("source label = %q, want profile owner reference", work.Source.Label)
	}

	delete(slots, CredentialKey(workOwner, key))
	calls = nil
	isolated := ResolveCredentialForRootGlobalFirstWithPaths(project, workPaths, key)
	if isolated.Set {
		t.Fatalf("work profile read default or legacy keyring value: %+v", isolated)
	}
	if got := credentialOwnerCallCount(calls, workOwner); got != 1 {
		t.Fatalf("isolated work lookup owner calls = %d, want 1", got)
	}
	if got := credentialOwnerCallCount(calls, defaultOwner); got != 0 {
		t.Fatalf("isolated work lookup should not fallback to default owner, got %d call(s)", got)
	}

	calls = nil
	def := ResolveCredentialForRootGlobalFirstWithPaths(project, defaultPaths, key)
	if !def.Set || def.Value != "default-keyring-value" {
		t.Fatalf("default profile keyring credential = %+v, want default owner value", def)
	}
	if strings.Contains(def.Source.Label, def.Value) {
		t.Fatalf("default source label leaked credential value: %q", def.Source.Label)
	}

	delete(slots, CredentialKey(defaultOwner, key))
	calls = nil
	legacy := ResolveCredentialForRootGlobalFirstWithPaths(project, defaultPaths, key)
	if !legacy.Set || legacy.Value != "legacy-keyring-value" {
		t.Fatalf("default profile legacy keyring fallback = %+v, want legacy value", legacy)
	}
}

func TestRemoveCredentialWithPathsClearsKeyringOwnerSlot(t *testing.T) {
	isolateProfilePathEnv(t)
	project := t.TempDir()
	key := "KEYRING_REMOVE_PROFILE_SCOPED"
	t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreKeyring)
	t.Setenv(key, "process-value")

	defaultPaths := NewProfilePathResolver(userctx.Default(project))
	actorA := userctx.Default(project)
	actorA.ProfileID = "profile-a"
	pathsA := NewProfilePathResolver(actorA)
	actorB := userctx.Default(project)
	actorB.ProfileID = "profile-b"
	pathsB := NewProfilePathResolver(actorB)

	defaultOwner := CredentialOwnerForPaths(defaultPaths)
	ownerA := CredentialOwnerForPaths(pathsA)
	ownerB := CredentialOwnerForPaths(pathsB)
	slots := map[string]string{
		CredentialKey(defaultOwner, key): "default-owner-keyring-value",
		CredentialKey(ownerA, key):       "profile-a-keyring-value",
		CredentialKey(ownerB, key):       "profile-b-keyring-value",
		key:                              "legacy-keyring-value",
	}

	oldLookup := keyringCredentialValueLookup
	keyringCredentialValueLookup = func(owner CredentialOwner, envName string) (string, bool) {
		owner = owner.Normalize()
		if value, ok := slots[CredentialKey(owner, envName)]; ok {
			return value, true
		}
		if owner.isDefaultProfile() {
			value, ok := slots[envName]
			return value, ok
		}
		return "", false
	}
	oldDelete := keyringCredentialDeleteLookup
	keyringCredentialDeleteLookup = func(owner CredentialOwner, envName string) error {
		owner = owner.Normalize()
		delete(slots, CredentialKey(owner, envName))
		if owner.isDefaultProfile() {
			delete(slots, envName)
		}
		return nil
	}
	t.Cleanup(func() {
		keyringCredentialValueLookup = oldLookup
		keyringCredentialDeleteLookup = oldDelete
	})

	if err := os.MkdirAll(filepath.Dir(pathsA.UserCredentialsPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathsA.UserCredentialsPath(), []byte(key+"=profile-a-file-value\nOTHER_KEY=kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	beforeA := ResolveCredentialForRootGlobalFirstWithPaths(project, pathsA, key)
	if !beforeA.Set || beforeA.Value != "profile-a-keyring-value" {
		t.Fatalf("profile A before remove = %+v, want keyring value", beforeA)
	}

	if err := RemoveCredentialWithPaths(pathsA, key); err != nil {
		t.Fatalf("RemoveCredentialWithPaths profile A: %v", err)
	}
	afterA := ResolveCredentialForRootGlobalFirstWithPaths(project, pathsA, key)
	if afterA.Set {
		t.Fatalf("profile A after remove still resolved credential: %+v", afterA)
	}
	afterB := ResolveCredentialForRootGlobalFirstWithPaths(project, pathsB, key)
	if !afterB.Set || afterB.Value != "profile-b-keyring-value" {
		t.Fatalf("profile B after profile A remove = %+v, want untouched profile B keyring value", afterB)
	}
	if got := os.Getenv(key); got != "" {
		t.Fatalf("process env after remove = %q, want unset", got)
	}
	fileData, err := os.ReadFile(pathsA.UserCredentialsPath())
	if err != nil {
		t.Fatalf("read profile A credentials: %v", err)
	}
	if strings.Contains(string(fileData), key+"=") {
		t.Fatalf("profile A credentials still contains removed key: %q", fileData)
	}
	if !strings.Contains(string(fileData), "OTHER_KEY=kept") {
		t.Fatalf("profile A credentials lost unrelated key: %q", fileData)
	}

	if err := RemoveCredentialWithPaths(defaultPaths, key); err != nil {
		t.Fatalf("RemoveCredentialWithPaths default: %v", err)
	}
	afterDefault := ResolveCredentialForRootGlobalFirstWithPaths(project, defaultPaths, key)
	if afterDefault.Set {
		t.Fatalf("default profile after remove still resolved owner or legacy keyring value: %+v", afterDefault)
	}
}

func TestRemoveCredentialWithPathsFileStoreDoesNotDeleteKeyringSlot(t *testing.T) {
	isolateProfilePathEnv(t)
	project := t.TempDir()
	key := "FILE_STORE_REMOVE_ONLY"
	t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreFile)
	t.Setenv(key, "process-value")

	paths := NewProfilePathResolver(userctx.Default(project))
	if err := os.MkdirAll(filepath.Dir(paths.UserCredentialsPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserCredentialsPath(), []byte(key+"=file-value\nOTHER_KEY=kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldDelete := keyringCredentialDeleteLookup
	keyringCredentialDeleteLookup = func(owner CredentialOwner, envName string) error {
		t.Fatalf("file credentials store should not delete keyring slot for owner %+v key %q", owner, envName)
		return nil
	}
	t.Cleanup(func() { keyringCredentialDeleteLookup = oldDelete })

	if err := RemoveCredentialWithPaths(paths, key); err != nil {
		t.Fatalf("RemoveCredentialWithPaths: %v", err)
	}
	if got := os.Getenv(key); got != "" {
		t.Fatalf("process env after remove = %q, want unset", got)
	}
	data, err := os.ReadFile(paths.UserCredentialsPath())
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if strings.Contains(string(data), key+"=") {
		t.Fatalf("credentials file still contains removed key: %q", data)
	}
	if !strings.Contains(string(data), "OTHER_KEY=kept") {
		t.Fatalf("credentials file lost unrelated key: %q", data)
	}
}

func TestCredentialResolverWithPathsCachesSharedProviderKey(t *testing.T) {
	isolateProfilePathEnv(t)
	project := t.TempDir()
	key := "KEY_PROFILE_SHARED_PROVIDER"
	t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreKeyring)
	t.Setenv(key, "")
	os.Unsetenv(key)

	actor := userctx.Default(project)
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)
	wantOwner := CredentialOwnerForPaths(paths)

	calls := 0
	oldLookup := keyringCredentialValueLookup
	keyringCredentialValueLookup = func(owner CredentialOwner, envName string) (string, bool) {
		if owner.Normalize() != wantOwner {
			t.Fatalf("keyring lookup owner = %+v, want %+v", owner.Normalize(), wantOwner)
		}
		if envName != key {
			t.Fatalf("keyring lookup key = %q, want %q", envName, key)
		}
		calls++
		if calls == 1 {
			return "first-profile-value", true
		}
		return "changed-value", true
	}
	t.Cleanup(func() { keyringCredentialValueLookup = oldLookup })

	cfg := &Config{Providers: []ProviderEntry{
		{Name: "primary", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "m", APIKeyEnv: key},
		{Name: "secondary", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "m", APIKeyEnv: key},
	}}
	resolver := NewCredentialResolverForRootWithPaths(project, paths)
	resolveProviderCredentialsWithResolver(cfg, resolver)

	if calls != 1 {
		t.Fatalf("keyring lookups = %d, want 1 for shared api_key_env in one resolver", calls)
	}
	for _, provider := range cfg.Providers {
		if got := provider.APIKey(); got != "first-profile-value" {
			t.Fatalf("%s APIKey() = %q, want cached profile value", provider.Name, got)
		}
	}
}

func credentialOwnerCallCount(calls []CredentialOwner, want CredentialOwner) int {
	want = want.Normalize()
	count := 0
	for _, call := range calls {
		if call.Normalize() == want {
			count++
		}
	}
	return count
}
