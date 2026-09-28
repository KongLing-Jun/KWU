package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	cloud "kwu/internal/cloud"
)

func TestValidateDevelopmentRuntimeManifestAcceptsMatchingCloud(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/environmentz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"schema_version":1,"environment_id":"00000000-0000-4000-8000-000000000001","environment_name":"test"}}`))
	}))
	t.Cleanup(server.Close)

	path := writeDevelopmentRuntimeManifest(t, server.URL+"/", "00000000-0000-4000-8000-000000000001")
	profile := desktopProfile{Development: true, NormalizedCloudBaseURL: server.URL + "/"}
	if err := validateDevelopmentRuntimeManifest(profile, path, server.Client()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDevelopmentRuntimeManifestRejectsBeforeAuthenticatedStartup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"schema_version":1,"environment_id":"00000000-0000-4000-8000-000000000002","environment_name":"other"}}`))
	}))
	t.Cleanup(server.Close)

	path := writeDevelopmentRuntimeManifest(t, server.URL+"/", "00000000-0000-4000-8000-000000000001")
	profile := desktopProfile{Development: true, NormalizedCloudBaseURL: server.URL + "/"}
	if err := validateDevelopmentRuntimeManifest(profile, path, server.Client()); !errors.Is(err, cloud.ErrEnvironmentMismatch) {
		t.Fatalf("error = %v, want environment mismatch", err)
	}
}

func TestValidateDevelopmentRuntimeManifestRejectsCloudURLMismatch(t *testing.T) {
	path := writeDevelopmentRuntimeManifest(t, "http://127.0.0.1:9090/", "00000000-0000-4000-8000-000000000001")
	profile := desktopProfile{Development: true, NormalizedCloudBaseURL: "http://127.0.0.1:9091/"}
	if err := validateDevelopmentRuntimeManifest(profile, path, http.DefaultClient); !errors.Is(err, cloud.ErrEnvironmentMismatch) {
		t.Fatalf("error = %v, want environment mismatch", err)
	}
}

func writeDevelopmentRuntimeManifest(t *testing.T, baseURL, environmentID string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.json")
	contents := `{"schema_version":1,"cloud_base_url":"` + baseURL + `","cloud_environment_id":"` + environmentID + `","cloud_environment_name":"test","created_at":"2026-08-11T00:00:00Z","repositories":{},"services":{}}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
