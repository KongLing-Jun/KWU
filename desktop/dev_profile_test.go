package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"kwu/internal/brand"
	"kwu/internal/config"
	"kwu/internal/userctx"
)

func TestNormalizeDevCloudBaseURL(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: " HTTPS://Example.COM:443//api///v1/ ", want: "https://example.com/api/v1"},
		{raw: "http://EXAMPLE.com:80", want: "http://example.com/"},
		{raw: "http://EXAMPLE.com:8080/a/../b", want: "http://example.com:8080/b"},
		{raw: "https://[2001:DB8::1]:443//v1", want: "https://[2001:db8::1]/v1"},
	}
	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			got, err := normalizeDevCloudBaseURL(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("normalized URL = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeDevCloudBaseURLRejectsUnsafeValues(t *testing.T) {
	for _, raw := range []string{
		"",
		"example.com",
		"ftp://example.com",
		"https://user@example.com",
		"https://example.com?token=secret",
		"https://example.com?",
		"https://example.com#fragment",
		"https://example.com#",
		"https:///missing-host",
	} {
		t.Run(raw, func(t *testing.T) {
			if got, err := normalizeDevCloudBaseURL(raw); err == nil {
				t.Fatalf("normalizeDevCloudBaseURL(%q) = %q, want error", raw, got)
			}
		})
	}
}

func TestDevProfileEnvironmentIDIsStableAcrossWorkingDirectories(t *testing.T) {
	base := t.TempDir()
	locations := testDesktopProfileLocations(base)
	firstEnv := map[string]string{
		cloudBaseURLEnv:   " HTTPS://Cloud.Example.COM:443//api/ ",
		devProfileRootEnv: filepath.Join(base, "first-profile"),
	}
	first, err := initializeDesktopProfile("dev", mapGetenv(firstEnv), mapSetenv(firstEnv), locations)
	if err != nil {
		t.Fatal(err)
	}

	firstCWD := filepath.Join(base, "worktree-a")
	secondCWD := filepath.Join(base, "worktree-b")
	if err := os.MkdirAll(firstCWD, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(secondCWD, 0o755); err != nil {
		t.Fatal(err)
	}
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCWD) })
	if err := os.Chdir(secondCWD); err != nil {
		t.Fatal(err)
	}
	secondEnv := map[string]string{
		cloudBaseURLEnv:   "https://cloud.example.com/api",
		devProfileRootEnv: filepath.Join(base, "second-profile"),
	}
	second, err := initializeDesktopProfile("dev", mapGetenv(secondEnv), mapSetenv(secondEnv), locations)
	if err != nil {
		t.Fatal(err)
	}
	if first.EnvironmentID != second.EnvironmentID {
		t.Fatalf("environment IDs differ by cwd/worktree: %q != %q", first.EnvironmentID, second.EnvironmentID)
	}
	if first.NormalizedCloudBaseURL != second.NormalizedCloudBaseURL {
		t.Fatalf("normalized URLs differ: %q != %q", first.NormalizedCloudBaseURL, second.NormalizedCloudBaseURL)
	}

	sum := sha256.Sum256([]byte("https://cloud.example.com/api"))
	if want := hex.EncodeToString(sum[:])[:16]; first.EnvironmentID != want {
		t.Fatalf("environment ID = %q, want %q", first.EnvironmentID, want)
	}
}

func TestDifferentCloudURLsUseDifferentDefaultProfiles(t *testing.T) {
	base := t.TempDir()
	locations := testDesktopProfileLocations(base)
	firstEnv := map[string]string{cloudBaseURLEnv: "https://one.example.com"}
	secondEnv := map[string]string{cloudBaseURLEnv: "https://two.example.com"}
	first, err := initializeDesktopProfile("dev", mapGetenv(firstEnv), mapSetenv(firstEnv), locations)
	if err != nil {
		t.Fatal(err)
	}
	second, err := initializeDesktopProfile("dev", mapGetenv(secondEnv), mapSetenv(secondEnv), locations)
	if err != nil {
		t.Fatal(err)
	}
	if first.EnvironmentID == second.EnvironmentID {
		t.Fatalf("different URLs shared environment ID %q", first.EnvironmentID)
	}
	if first.HomeRoot == second.HomeRoot || first.CacheRoot == second.CacheRoot {
		t.Fatalf("different URLs shared roots: first=%+v second=%+v", first, second)
	}
	if !strings.Contains(first.HomeRoot, filepath.Join("kwu-dev", "environments", first.EnvironmentID)) {
		t.Fatalf("home root = %q, want environment-scoped path", first.HomeRoot)
	}
	if !strings.Contains(first.CacheRoot, filepath.Join("kwu-dev", "environments", first.EnvironmentID, "cache")) {
		t.Fatalf("cache root = %q, want environment-scoped path", first.CacheRoot)
	}
}

func TestWindowsDefaultWebViewUsesLocalAppDataEnvironmentRoot(t *testing.T) {
	base := t.TempDir()
	locations := testDesktopProfileLocations(base)
	locations.GOOS = "windows"
	profile, err := resolveDevProfileRoots("0123456789abcdef", "", locations)
	if err != nil {
		t.Fatal(err)
	}
	environmentRoot := filepath.Join(locations.UserCacheDir, "kwu-dev", "environments", "0123456789abcdef")
	if got, want := profile.CacheRoot, filepath.Join(environmentRoot, "cache"); got != want {
		t.Fatalf("Windows cache root = %q, want %q", got, want)
	}
	if got, want := profile.WebViewRoot, filepath.Join(environmentRoot, "webview"); got != want {
		t.Fatalf("Windows WebView root = %q, want %q", got, want)
	}
	if pathEqualOrWithin(profile.WebViewRoot, locations.UserConfigDir, "windows") {
		t.Fatalf("Windows WebView root unexpectedly uses roaming AppData: %q", profile.WebViewRoot)
	}
}

func TestUnixDefaultRootsUseHomeAndOSCache(t *testing.T) {
	base := t.TempDir()
	locations := testDesktopProfileLocations(base)
	locations.GOOS = "linux"
	profile, err := resolveDevProfileRoots("0123456789abcdef", "", locations)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := profile.HomeRoot, filepath.Join(locations.UserHomeDir, ".kwu-dev", "environments", "0123456789abcdef"); got != want {
		t.Fatalf("Unix home root = %q, want %q", got, want)
	}
	if profile.StateRoot != profile.HomeRoot {
		t.Fatalf("Unix state root = %q, want home root %q", profile.StateRoot, profile.HomeRoot)
	}
	if got, want := profile.CacheRoot, filepath.Join(locations.UserCacheDir, "kwu-dev", "environments", "0123456789abcdef", "cache"); got != want {
		t.Fatalf("Unix cache root = %q, want %q", got, want)
	}
	if profile.WebViewRoot != "" {
		t.Fatalf("Unix WebView root = %q, want no unsupported override", profile.WebViewRoot)
	}
}

func TestDevProfileRootOverrideControlsEveryPrivateRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "integration-profile")
	environment := map[string]string{
		cloudBaseURLEnv:   "https://cloud.example.com",
		devProfileRootEnv: root,
	}
	profile, err := initializeDesktopProfile("dev", mapGetenv(environment), mapSetenv(environment), testDesktopProfileLocations(base))
	if err != nil {
		t.Fatal(err)
	}
	if profile.HomeRoot != root || profile.StateRoot != root || profile.CacheRoot != filepath.Join(root, "cache") {
		t.Fatalf("override roots = %+v", profile)
	}
	if runtime.GOOS == "windows" && profile.WebViewRoot != filepath.Join(root, "webview") {
		t.Fatalf("WebView root = %q", profile.WebViewRoot)
	}
	for name, want := range map[string]string{
		"KWU_HOME":       root,
		"KWU_STATE_HOME": root,
		"KWU_CACHE_HOME": filepath.Join(root, "cache"),
	} {
		if got := environment[name]; got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}

	relativeEnv := map[string]string{cloudBaseURLEnv: "https://cloud.example.com", devProfileRootEnv: "relative/profile"}
	if _, err := initializeDesktopProfile("dev", mapGetenv(relativeEnv), mapSetenv(relativeEnv), testDesktopProfileLocations(base)); err == nil {
		t.Fatal("relative KWU_DEV_PROFILE_ROOT was accepted")
	}
}

func TestDevProfileRootRejectsProductionDirectories(t *testing.T) {
	base := t.TempDir()
	locations := testDesktopProfileLocations(base)
	for _, root := range knownProductionProfileRoots(locations) {
		for _, candidate := range []string{root, filepath.Join(root, "dev-test")} {
			environment := map[string]string{
				cloudBaseURLEnv:   "https://cloud.example.com",
				devProfileRootEnv: candidate,
			}
			if _, err := initializeDesktopProfile("dev", mapGetenv(environment), mapSetenv(environment), locations); err == nil {
				t.Fatalf("production directory %q was accepted", candidate)
			}
		}
	}
}

func TestKnownProductionProfileRootsProtectsCanonicalAndLegacyWindowsData(t *testing.T) {
	base := t.TempDir()
	locations := desktopProfileLocations{
		GOOS:          "windows",
		UserConfigDir: filepath.Join(base, "AppData"),
		UserCacheDir:  filepath.Join(base, "LocalAppData"),
	}
	got := knownProductionProfileRoots(locations)
	want := []string{
		filepath.Join(locations.UserConfigDir, "Clouvia"),
		filepath.Join(locations.UserConfigDir, "kwu"),
		filepath.Join(locations.UserConfigDir, "clouvia-desktop.exe"),
		filepath.Join(locations.UserConfigDir, "kwu-desktop.exe"),
		filepath.Join(locations.UserCacheDir, "kwu"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("knownProductionProfileRoots() = %v, want %v", got, want)
	}
}

func TestDevEnvironmentMarkerCreateMatchAndConflict(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "profile")
	locations := testDesktopProfileLocations(base)
	environment := map[string]string{cloudBaseURLEnv: "https://cloud.example.com", devProfileRootEnv: root}
	profile, err := initializeDesktopProfile("dev", mapGetenv(environment), mapSetenv(environment), locations)
	if err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(root, devEnvironmentFileName)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	var marker map[string]any
	if err := json.Unmarshal(data, &marker); err != nil {
		t.Fatal(err)
	}
	if len(marker) != 3 || marker["schemaVersion"] != float64(1) || marker["environmentId"] != profile.EnvironmentID || marker["cloudBaseUrl"] != profile.NormalizedCloudBaseURL {
		t.Fatalf("marker = %#v", marker)
	}
	if _, err := initializeDesktopProfile("dev", mapGetenv(environment), mapSetenv(environment), locations); err != nil {
		t.Fatalf("matching marker rejected: %v", err)
	}

	conflicting := devEnvironmentMarker{SchemaVersion: 1, EnvironmentID: "other", CloudBaseURL: "https://other.example.com/"}
	conflictingData, err := json.Marshal(conflicting)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath, conflictingData, 0o600); err != nil {
		t.Fatal(err)
	}
	environment["KWU_HOME"] = "unchanged-home"
	if _, err := initializeDesktopProfile("dev", mapGetenv(environment), mapSetenv(environment), locations); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflicting marker error = %v", err)
	}
	if environment["KWU_HOME"] != "unchanged-home" {
		t.Fatalf("environment changed before marker validation: %q", environment["KWU_HOME"])
	}
}

func TestProductionProfileInitializationIsExactNoOp(t *testing.T) {
	base := t.TempDir()
	markerPath := filepath.Join(base, devEnvironmentFileName)
	if err := os.WriteFile(markerPath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{
		cloudBaseURLEnv:      "not a URL",
		devProfileRootEnv:    base,
		"CLOUVIA_HOME":       "existing-canonical-home",
		"CLOUVIA_STATE_HOME": "existing-canonical-state",
		"CLOUVIA_CACHE_HOME": "existing-canonical-cache",
		"KWU_HOME":           "existing-home",
		"KWU_STATE_HOME":     "existing-state",
		"KWU_CACHE_HOME":     "existing-cache",
	}
	setCalls := 0
	setenv := func(name, value string) error {
		setCalls++
		environment[name] = value
		return nil
	}
	profile, err := initializeDesktopProfile("v1.2.3", mapGetenv(environment), setenv, desktopProfileLocations{})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Development || setCalls != 0 {
		t.Fatalf("production initialization changed state: profile=%+v setCalls=%d", profile, setCalls)
	}
	if got := environment["KWU_HOME"]; got != "existing-home" {
		t.Fatalf("KWU_HOME = %q", got)
	}
	if got := environment["CLOUVIA_HOME"]; got != "existing-canonical-home" {
		t.Fatalf("CLOUVIA_HOME = %q", got)
	}
}

func TestDevProfileEnvironmentFailureRollsBackCanonicalAndCompatibilityAliases(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "profile")
	environment := map[string]string{
		cloudBaseURLEnv:      "https://cloud.example.com",
		devProfileRootEnv:    root,
		"CLOUVIA_HOME":       "old-canonical-home",
		"CLOUVIA_STATE_HOME": "old-canonical-state",
		"CLOUVIA_CACHE_HOME": "old-canonical-cache",
		"KWU_HOME":           "old-compat-home",
		"KWU_STATE_HOME":     "old-compat-state",
		"KWU_CACHE_HOME":     "old-compat-cache",
	}
	original := make(map[string]string, len(environment))
	for name, value := range environment {
		original[name] = value
	}
	setenv := func(name, value string) error {
		if name == "KWU_STATE_HOME" && value != original[name] {
			return os.ErrPermission
		}
		environment[name] = value
		return nil
	}
	profile := desktopProfile{HomeRoot: root, StateRoot: root, CacheRoot: filepath.Join(root, "cache")}
	if err := setDesktopProfileEnvironment(mapGetenv(environment), setenv, profile, true); err == nil {
		t.Fatal("environment write failure was accepted")
	}
	for _, name := range []string{
		"CLOUVIA_HOME", "CLOUVIA_STATE_HOME", "CLOUVIA_CACHE_HOME",
		"KWU_HOME", "KWU_STATE_HOME", "KWU_CACHE_HOME",
	} {
		if got := environment[name]; got != original[name] {
			t.Fatalf("%s after rollback = %q, want %q", name, got, original[name])
		}
	}
}

func TestCanonicalOnlyDesktopStartupUsesDevelopmentProfileRoots(t *testing.T) {
	if os.Getenv("KWU_TEST_CANONICAL_DEV_PROFILE_HELPER") == "1" {
		runCanonicalOnlyDesktopStartupDevelopmentProfileTest(t)
		return
	}
	base := t.TempDir()
	for _, goos := range []string{"windows", "darwin"} {
		root := filepath.Join(base, goos, "isolated-dev-profile")
		command := exec.Command(os.Args[0], "-test.run=^TestCanonicalOnlyDesktopStartupUsesDevelopmentProfileRoots$")
		command.Env = append(os.Environ(),
			"KWU_TEST_CANONICAL_DEV_PROFILE_HELPER=1",
			"KWU_TEST_CANONICAL_DEV_PROFILE_GOOS="+goos,
			"KWU_TEST_CANONICAL_DEV_PROFILE_ROOT="+root,
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("%s canonical-only startup helper failed: %v\n%s", goos, err, output)
		}
	}
}

func runCanonicalOnlyDesktopStartupDevelopmentProfileTest(t *testing.T) {
	root := os.Getenv("KWU_TEST_CANONICAL_DEV_PROFILE_ROOT")
	goos := os.Getenv("KWU_TEST_CANONICAL_DEV_PROFILE_GOOS")
	if root == "" || (goos != "windows" && goos != "darwin") {
		t.Fatalf("invalid canonical-only helper inputs: root=%q goos=%q", root, goos)
	}
	for _, name := range []string{
		"CLOUVIA_HOME", "CLOUVIA_STATE_HOME", "CLOUVIA_CACHE_HOME",
		"KWU_HOME", "KWU_STATE_HOME", "KWU_CACHE_HOME",
	} {
		t.Setenv(name, "")
	}
	t.Setenv(cloudBaseURLEnv, "http://127.0.0.1:18900")
	t.Setenv(devProfileRootEnv, root)
	configureDesktopUserDataPolicy(goos, config.UseCanonicalUserDataOnly)
	profile, err := initializeDesktopProfile("dev", os.Getenv, os.Setenv, desktopProfileLocations{
		GOOS:          goos,
		UserHomeDir:   filepath.Join(filepath.Dir(root), "production-home"),
		UserConfigDir: filepath.Join(filepath.Dir(root), "production-appdata"),
		UserCacheDir:  filepath.Join(filepath.Dir(root), "production-cache"),
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCache := filepath.Join(root, "cache")
	if profile.HomeRoot != root || profile.StateRoot != root || profile.CacheRoot != wantCache {
		t.Fatalf("development profile roots = %+v", profile)
	}
	if got := config.KWUHomeDir(); got != root {
		t.Fatalf("canonical-only config home = %q, want %q", got, root)
	}
	if got := config.MemoryUserDir(); got != root {
		t.Fatalf("canonical-only memory state = %q, want %q", got, root)
	}
	if got := config.CacheDir(); got != wantCache {
		t.Fatalf("canonical-only cache = %q, want %q", got, wantCache)
	}
	resolver := config.NewProfilePathResolver(userctx.Default(""))
	if got := resolver.KWUHomeDir(); got != root {
		t.Fatalf("canonical-only profile home = %q, want %q", got, root)
	}
	if got := resolver.MemoryUserDir(); got != root {
		t.Fatalf("canonical-only profile state = %q, want %q", got, root)
	}
	if got := resolver.CacheDir(); got != wantCache {
		t.Fatalf("canonical-only profile cache = %q, want %q", got, wantCache)
	}
	for name, want := range map[string]string{
		"CLOUVIA_HOME": root, "CLOUVIA_STATE_HOME": root, "CLOUVIA_CACHE_HOME": wantCache,
		"KWU_HOME": root, "KWU_STATE_HOME": root, "KWU_CACHE_HOME": wantCache,
	} {
		if got := os.Getenv(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestDesktopConfigDirUsesClouviaProductionDefaultWithoutOverride(t *testing.T) {
	t.Setenv("CLOUVIA_HOME", "")
	t.Setenv("KWU_HOME", "")
	var want string
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		if err == nil {
			want = filepath.Join(dir, brand.EnglishName)
		}
	} else {
		home, _ := os.UserHomeDir()
		want = filepath.Join(home, brand.ProjectDirName)
	}
	if want == "" {
		home, _ := os.UserHomeDir()
		want = filepath.Join(home, brand.ProjectDirName)
	}
	if got := desktopConfigDir(); got != want {
		t.Fatalf("production desktop config dir = %q, want Clouvia default %q", got, want)
	}
}

func TestDesktopConfigDirHonorsExplicitProductionKWUHome(t *testing.T) {
	root := filepath.Join(t.TempDir(), "production-override")
	t.Setenv("KWU_HOME", root)
	if got := desktopConfigDir(); got != root {
		t.Fatalf("production desktop config dir = %q, want KWU_HOME %q", got, root)
	}
}

func TestDesktopConfigDirHonorsInitializedDevelopmentRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "dev-profile")
	for _, name := range []string{"CLOUVIA_HOME", "CLOUVIA_STATE_HOME", "CLOUVIA_CACHE_HOME", "KWU_HOME", "KWU_STATE_HOME", "KWU_CACHE_HOME", cloudBaseURLEnv, devProfileRootEnv} {
		t.Setenv(name, "")
	}
	t.Setenv(cloudBaseURLEnv, "https://dev-cloud.example.com")
	t.Setenv(devProfileRootEnv, root)
	if _, err := initializeDesktopProfile("dev", os.Getenv, os.Setenv, testDesktopProfileLocations(base)); err != nil {
		t.Fatal(err)
	}
	if got := desktopConfigDir(); got != root {
		t.Fatalf("development desktop config dir = %q, want environment root %q", got, root)
	}
}

func TestNewAppWritesMarkerBeforeCloudAndBillingConstruction(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "profile")
	markerPath := filepath.Join(root, devEnvironmentFileName)
	environment := map[string]string{cloudBaseURLEnv: "https://cloud.example.com", devProfileRootEnv: root}
	order := make([]string, 0, 3)
	assertMarker := func(stage string) {
		t.Helper()
		data, err := os.ReadFile(markerPath)
		if err != nil {
			t.Fatalf("%s ran before marker write: %v", stage, err)
		}
		if !strings.Contains(string(data), `"cloudBaseUrl":"https://cloud.example.com/"`) {
			t.Fatalf("%s saw unexpected marker: %s", stage, data)
		}
	}
	app := newAppWithDependencies(appConstructionDependencies{
		initializeProfile: func() (desktopProfile, error) {
			profile, err := initializeDesktopProfile("dev", mapGetenv(environment), mapSetenv(environment), testDesktopProfileLocations(base))
			order = append(order, "profile")
			return profile, err
		},
		newCloudRuntime: func() *cloudRuntime {
			assertMarker("cloud construction")
			order = append(order, "cloud")
			return &cloudRuntime{}
		},
		newBillingRuntime: func(factory desktopBillingWorkerFactory) *desktopBillingWorkerManager {
			assertMarker("billing construction")
			order = append(order, "billing")
			return newDesktopBillingWorkerManager(factory)
		},
	})
	if app.cloudRuntime == nil || app.cloudRuntime.initErr != nil {
		t.Fatalf("app cloud runtime = %#v", app.cloudRuntime)
	}
	if got := strings.Join(order, ","); got != "profile,cloud,billing" {
		t.Fatalf("construction order = %q", got)
	}
}

func TestNewAppMarkerConflictBlocksCloudAndBillingConstruction(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "profile")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	conflicting := devEnvironmentMarker{SchemaVersion: 1, EnvironmentID: "wrong", CloudBaseURL: "https://wrong.example.com/"}
	data, err := json.Marshal(conflicting)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, devEnvironmentFileName), data, 0o600); err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{cloudBaseURLEnv: "https://cloud.example.com", devProfileRootEnv: root}
	cloudCalled, billingCalled := false, false
	app := newAppWithDependencies(appConstructionDependencies{
		initializeProfile: func() (desktopProfile, error) {
			return initializeDesktopProfile("dev", mapGetenv(environment), mapSetenv(environment), testDesktopProfileLocations(base))
		},
		newCloudRuntime: func() *cloudRuntime {
			cloudCalled = true
			return &cloudRuntime{}
		},
		newBillingRuntime: func(factory desktopBillingWorkerFactory) *desktopBillingWorkerManager {
			billingCalled = true
			return newDesktopBillingWorkerManager(factory)
		},
	})
	if app.cloudRuntime == nil || app.cloudRuntime.initErr == nil || !strings.Contains(app.cloudRuntime.initErr.Error(), "conflicts") {
		t.Fatalf("marker conflict was not surfaced: %#v", app.cloudRuntime)
	}
	if cloudCalled || billingCalled {
		t.Fatalf("marker conflict reached private state constructors: cloud=%t billing=%t", cloudCalled, billingCalled)
	}
}

func TestInitializedDevRootsReachDesktopCloudBillingAndCaches(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "profile")
	for _, name := range []string{"CLOUVIA_HOME", "CLOUVIA_STATE_HOME", "CLOUVIA_CACHE_HOME", "KWU_HOME", "KWU_STATE_HOME", "KWU_CACHE_HOME", cloudBaseURLEnv, devProfileRootEnv} {
		t.Setenv(name, "")
	}
	t.Setenv(cloudBaseURLEnv, "https://cloud.example.com")
	t.Setenv(devProfileRootEnv, root)
	profile, err := initializeDesktopProfile("dev", os.Getenv, os.Setenv, testDesktopProfileLocations(base))
	if err != nil {
		t.Fatal(err)
	}
	if got := config.KWUHomeDir(); got != root {
		t.Fatalf("config home = %q, want %q", got, root)
	}
	if got := config.MemoryUserDir(); got != root {
		t.Fatalf("config state = %q, want %q", got, root)
	}
	if got := config.CacheDir(); got != filepath.Join(root, "cache") {
		t.Fatalf("config cache = %q", got)
	}
	if got := config.NewProfilePathResolver(userctx.Default("")).MemoryUserDir(); got != root {
		t.Fatalf("cloud resolver state = %q", got)
	}
	if got := desktopConfigDir(); got != root {
		t.Fatalf("desktop config dir = %q", got)
	}
	if got := globalWorkspaceRoot(); got != filepath.Join(root, "global-workspace") {
		t.Fatalf("global workspace = %q", got)
	}
	if got := defaultDesktopAcceptedQuotaRequestRegistry().path; got != filepath.Join(root, "model-billing-accepted-requests.sqlite") {
		t.Fatalf("accepted-request database = %q", got)
	}
	if got, err := defaultUpdateCacheBaseDir(); err != nil || got != filepath.Join(root, "cache", "updates") {
		t.Fatalf("updater cache = %q, %v", got, err)
	}
	if got := config.DesktopWindowStatePath(); got != filepath.Join(root, "desktop-window.json") {
		t.Fatalf("window state = %q", got)
	}
	if profile.WebViewRoot != "" && profile.WebViewRoot != filepath.Join(root, "webview") {
		t.Fatalf("WebView root = %q", profile.WebViewRoot)
	}

	db, err := openDesktopBillingDB(userctx.Default(""))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "model-billing.sqlite")); err != nil {
		t.Fatalf("billing database did not use dev state root: %v", err)
	}

	cloudRuntime := newCloudRuntimeFromEnvironment()
	if cloudRuntime.initErr != nil || !cloudRuntime.available {
		t.Fatalf("cloud runtime = %#v", cloudRuntime)
	}
	if _, err := os.Stat(filepath.Join(root, "cloud", "device-id")); err != nil {
		t.Fatalf("cloud device state did not use dev state root: %v", err)
	}
}

func testDesktopProfileLocations(base string) desktopProfileLocations {
	return desktopProfileLocations{
		GOOS:          runtime.GOOS,
		UserHomeDir:   filepath.Join(base, "home"),
		UserConfigDir: filepath.Join(base, "AppData"),
		UserCacheDir:  filepath.Join(base, "LocalAppData"),
	}
}

func mapGetenv(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func mapSetenv(values map[string]string) func(string, string) error {
	return func(name, value string) error {
		values[name] = value
		return nil
	}
}
