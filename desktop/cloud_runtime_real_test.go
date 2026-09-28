package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	cloud "kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/netclient"
	"kwu/internal/userctx"
)

func desktopTestCloudEnvironment(baseURL string) cloud.EnvironmentIdentity {
	identity, err := cloud.NewEnvironmentIdentity(cloud.EnvironmentDevelopment, baseURL)
	if err != nil {
		panic(err)
	}
	return identity
}

func TestCloudEnvironmentIdentityUsesBuildClassAndNormalizedURL(t *testing.T) {
	t.Parallel()
	dev, err := cloudEnvironmentIdentity("dev", "HTTPS://EXAMPLE.COM:443/api/../")
	if err != nil {
		t.Fatal(err)
	}
	release, err := cloudEnvironmentIdentity("1.2.3-rc.1", "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if dev.CloudBaseURL != release.CloudBaseURL || dev.EnvironmentID == release.EnvironmentID {
		t.Fatalf("build identities = dev %#v release %#v", dev, release)
	}
	if _, err := newRealCloudRuntimeWithRegistration(
		"https://example.com/", desktopCloudTestProfilePathProvider(t.TempDir()),
		http.DefaultClient, cloud.DeviceIdentity{DeviceID: "device-01", ClientVersion: "dev"},
		cloudRegistrationUnavailable, cloud.EnvironmentIdentity{},
	); err == nil {
		t.Fatal("real runtime accepted an omitted environment identity")
	}
}

func TestFixtureRuntimeUsesStableNonProductionEnvironment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	provider := desktopCloudTestProfilePathProvider(root)
	runtime, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	if _, err := runtime.bindings.ConfirmBinding(context.Background(), actor, "usr_fixture"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "binding.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fixture, _ := cloud.NewEnvironmentIdentity(cloud.EnvironmentFixture, "https://fixture.reasonix.invalid/")
	production, _ := cloud.NewEnvironmentIdentity(cloud.EnvironmentProduction, "https://fixture.reasonix.invalid/")
	if !strings.Contains(string(contents), fixture.EnvironmentID) || strings.Contains(string(contents), production.EnvironmentID) {
		t.Fatalf("fixture binding environment = %s", contents)
	}
}

func TestCloudRuntimeModeSelectionFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, fake, base string
		want             cloudRuntimeMode
		wantErr          bool
	}{
		{name: "disabled", want: cloudRuntimeDisabled},
		{name: "fixture", fake: "1", want: cloudRuntimeFixture},
		{name: "real", base: "http://192.168.10.243:8080", want: cloudRuntimeReal},
		{name: "conflict", fake: "1", base: "http://192.168.10.243:8080", wantErr: true},
		{name: "invalid fake flag", fake: "yes", wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := cloudRuntimeModeFromValues(test.fake, test.base)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("mode = %q, err=%v; want %q err=%t", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestCloudRegistrationModeSelectionIgnoresLegacyCaptchaDevelopmentFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		runtime    cloudRuntimeMode
		baseURL    string
		captchaDev string
		want       cloudRegistrationMode
		wantErr    bool
	}{
		{name: "fixture", runtime: cloudRuntimeFixture, want: cloudRegistrationFixture},
		{name: "disabled", runtime: cloudRuntimeDisabled, want: cloudRegistrationUnavailable},
		{name: "private real", runtime: cloudRuntimeReal, baseURL: "http://192.168.10.243:8080", want: cloudRegistrationReal},
		{name: "private real ignores development flag", runtime: cloudRuntimeReal, baseURL: "http://192.168.10.243:8080", captchaDev: "1", want: cloudRegistrationReal},
		{name: "loopback real", runtime: cloudRuntimeReal, baseURL: "http://127.0.0.1:8080", want: cloudRegistrationReal},
		{name: "loopback real ignores development flag", runtime: cloudRuntimeReal, baseURL: "http://127.0.0.1:8080", captchaDev: "1", want: cloudRegistrationReal},
		{name: "public real", runtime: cloudRuntimeReal, baseURL: "https://cloud.example.com", want: cloudRegistrationReal},
		{name: "public real ignores development flag", runtime: cloudRuntimeReal, baseURL: "https://cloud.example.com", captchaDev: "1", want: cloudRegistrationReal},
		{name: "disabled ignores development flag", runtime: cloudRuntimeDisabled, captchaDev: "1", want: cloudRegistrationUnavailable},
		{name: "fixture ignores development flag", runtime: cloudRuntimeFixture, captchaDev: "1", want: cloudRegistrationFixture},
		{name: "invalid development flag cannot change real mode", runtime: cloudRuntimeReal, baseURL: "http://192.168.10.243:8080", captchaDev: "yes", want: cloudRegistrationReal},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := cloudRegistrationModeFromValues(test.runtime, test.baseURL, test.captchaDev)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("registration mode = %q, err=%v; want %q err=%t", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestCloudRuntimeEnvironmentRejectsInsecureOrPrivateProductionCloud(t *testing.T) {
	for _, baseURL := range []string{"http://127.0.0.1:8080", "https://127.0.0.1:8443", "https://localhost"} {
		t.Run(baseURL, func(t *testing.T) {
			if err := validateProductionCloudBaseURL(baseURL); err == nil {
				t.Fatalf("production validator accepted %q", baseURL)
			}
		})
	}
}

func TestResolveCloudBaseURLUsesDevelopmentDefault(t *testing.T) {
	got, err := resolveCloudBaseURL("dev", "", "http://192.168.10.243:8080", "https://cloud.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://192.168.10.243:8080" {
		t.Fatalf("base URL = %q, want development default", got)
	}
}

func TestCloudBaseURLConstantsResolveByBuildMode(t *testing.T) {
	development, err := resolveCloudBaseURL("dev", "", cloudDevelopmentBaseURL, cloudProductionBaseURL)
	if err != nil || development != "http://192.168.1.183:8080" {
		t.Fatalf("development URL = %q, %v", development, err)
	}
	production, err := resolveCloudBaseURL("v1.2.3", "http://192.168.10.244:8080", cloudDevelopmentBaseURL, cloudProductionBaseURL)
	if err != nil || production != "https://www.suiyuan-ai.com" {
		t.Fatalf("production URL = %q, %v", production, err)
	}
}

func TestCloudProductionBaseURLIsPublicHTTPS(t *testing.T) {
	if err := validateProductionCloudBaseURL(cloudProductionBaseURL); err != nil {
		t.Fatalf("production URL = %q, %v", cloudProductionBaseURL, err)
	}
}

func TestMediaUnderstandingHTTPClientUsesContextForLongResponseWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(40 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 10 * time.Millisecond
	base := &http.Client{Transport: transport, Timeout: 10 * time.Millisecond}
	client := newMediaUnderstandingHTTPClient(base)

	if client == base {
		t.Fatal("media understanding reused the shared HTTP client")
	}
	longRunningTransport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("media transport = %T, want *http.Transport", client.Transport)
	}
	if client.Timeout != 0 || longRunningTransport.ResponseHeaderTimeout != 0 {
		t.Fatalf(
			"media timeouts = client %s, response headers %s; want context-only deadlines",
			client.Timeout, longRunningTransport.ResponseHeaderTimeout,
		)
	}
	if base.Timeout != 10*time.Millisecond || transport.ResponseHeaderTimeout != 10*time.Millisecond {
		t.Fatal("media client mutated the shared cloud HTTP client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("delayed response failed before the context deadline: %v", err)
	}
	response.Body.Close()
}

func TestResolveCloudBaseURLAllowsDevelopmentOverride(t *testing.T) {
	const override = "http://192.168.10.244:8080"
	got, err := resolveCloudBaseURL("dev", override, "http://192.168.10.243:8080", "https://cloud.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != override {
		t.Fatalf("base URL = %q, want %q", got, override)
	}
}

func TestResolveCloudBaseURLUsesProductionDefault(t *testing.T) {
	got, err := resolveCloudBaseURL("v1.2.3", "http://192.168.10.244:8080", "http://192.168.10.243:8080", "https://cloud.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cloud.example.com" {
		t.Fatalf("base URL = %q, want production default", got)
	}
}

func TestResolveCloudBaseURLRejectsInvalidProductionDefault(t *testing.T) {
	for _, baseURL := range []string{"", "http://cloud.example.com", "https://127.0.0.1:8443", "https://localhost"} {
		t.Run(baseURL, func(t *testing.T) {
			if _, err := resolveCloudBaseURL("v1.2.3", "https://ignored.example.com", "http://192.168.10.243:8080", baseURL); err == nil {
				t.Fatalf("accepted invalid production default %q", baseURL)
			}
		})
	}
}

func TestCloudRuntimeEnvironmentUsesDevelopmentDefault(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "")
	t.Setenv(cloudCaptchaDevEnv, "")
	t.Setenv("KWU_STATE_HOME", t.TempDir())

	runtime := newCloudRuntimeFromEnvironment()
	if !runtime.available || runtime.initErr != nil || !runtime.capabilities.Registration {
		t.Fatalf("development cloud default is unavailable: %#v", runtime)
	}
}

func TestCloudRuntimeEnvironmentRejectsCaptchaWithoutRealBase(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "")
	t.Setenv(cloudCaptchaDevEnv, "1")
	runtime := newCloudRuntimeFromEnvironment()
	if runtime.available || runtime.initErr == nil || !strings.Contains(runtime.initErr.Error(), cloudCaptchaDevEnv) {
		t.Fatalf("orphan development captcha did not fail closed: %#v", runtime)
	}
}

func TestCloudRuntimeEnvironmentConflictIsUnavailable(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	t.Setenv(cloudBaseURLEnv, "http://192.168.10.243:8080")
	runtime := newCloudRuntimeFromEnvironment()
	if runtime.available || runtime.initErr == nil {
		t.Fatalf("conflicting environment did not fail closed: %#v", runtime)
	}
}

func TestRealCloudRuntimeExposesA12RegistrationCapabilities(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	runtime, err := newRealCloudRuntime(server.URL, desktopCloudTestProfilePathProvider(t.TempDir()), server.Client(), cloud.DeviceIdentity{DeviceID: "device-01", DeviceName: "Windows PC", ClientVersion: "dev"}, desktopTestCloudEnvironment(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	runtime.start(context.Background())
	if !runtime.available || runtime.remoteAccount == nil || runtime.auth == nil || runtime.errorReports == nil || runtime.credit == nil || runtime.quota == nil || runtime.planClient == nil || runtime.tasks == nil || runtime.taskStore == nil || runtime.payment == nil || runtime.paymentRecovery == nil || runtime.paymentOrders == nil || runtime.assetNotifier == nil {
		t.Fatalf("real runtime is incomplete: %#v", runtime)
	}
	if !runtime.capabilities.Registration || !runtime.capabilities.Payments || runtime.capabilities.Realtime {
		t.Fatalf("unexpected A1.2 capabilities: %#v", runtime.capabilities)
	}
	if runtime.capabilities.RegistrationMode != cloudRegistrationReal {
		t.Fatalf("real registration mode = %q", runtime.capabilities.RegistrationMode)
	}
	if runtime.accounts != nil || runtime.paymentClient != nil || runtime.eventSync != nil {
		t.Fatalf("real runtime initialized fixture-only services")
	}
}

func TestCloudDeviceIDIsStableProtectedAndDistinctFromTelemetryID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "cloud-device-id")
	first, err := loadOrCreateCloudDeviceID(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreateCloudDeviceID(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 36 || strings.Count(first, "-") != 4 {
		t.Fatalf("unstable or invalid device id: %q / %q", first, second)
	}
	if filepath.Base(path) == "install-id" {
		t.Fatal("cloud device identity reused telemetry identity")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("device id permissions are too broad: %o", info.Mode().Perm())
	}
}

func TestCloudDeviceIDReplacesInvalidExistingValue(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cloud-device-id")
	if err := os.WriteFile(path, []byte("invalid-legacy-id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := loadOrCreateCloudDeviceID(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cloudDeviceIDPattern.MatchString(id) {
		t.Fatalf("replacement device ID = %q", id)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(stored)) != id {
		t.Fatalf("stored replacement = %q, want %q", stored, id)
	}
}

func TestCloudProxySpecDirectsPrivateHostUnlessProxyIsExplicit(t *testing.T) {
	t.Parallel()
	auto, err := cloudProxySpecForBase(netclient.ProxySpec{Mode: netclient.ModeAuto, DirectHosts: []string{"existing.test"}}, "http://192.168.10.243:8080")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(auto.DirectHosts, ","); got != "existing.test,192.168.10.243" {
		t.Fatalf("auto direct hosts = %q", got)
	}
	custom, err := cloudProxySpecForBase(netclient.ProxySpec{Mode: netclient.ModeCustom, URL: "http://127.0.0.1:7890"}, "http://192.168.10.243:8080")
	if err != nil {
		t.Fatal(err)
	}
	if len(custom.DirectHosts) != 0 {
		t.Fatalf("custom proxy was bypassed: %#v", custom.DirectHosts)
	}
	public, err := cloudProxySpecForBase(netclient.ProxySpec{Mode: netclient.ModeAuto}, "https://cloud.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(public.DirectHosts) != 0 {
		t.Fatalf("public HTTPS endpoint unexpectedly bypassed automatic proxy: %#v", public.DirectHosts)
	}
}

func TestPublicCloudHTTPClientFallsBackWhenConfigIsMalformed(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLOUVIA_HOME", root)
	t.Setenv("KWU_HOME", "")
	t.Setenv("SUIYUAN_HOME", "")
	t.Setenv("REASONIX_HOME", "")
	configPath := config.NewProfilePathResolver(userctx.Default(".")).UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[bot.weixin]\ninvalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newCloudHTTPClient("https://cloud.example.com"); err == nil {
		t.Fatal("ordinary cloud client should report malformed configuration")
	}
	client, err := newPublicCloudHTTPClient("https://cloud.example.com")
	if err != nil {
		t.Fatalf("public cloud client should use the default proxy fallback: %v", err)
	}
	if client == nil || client.Transport == nil {
		t.Fatal("public cloud client fallback is incomplete")
	}
}
