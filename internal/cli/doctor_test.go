package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/config"
)

func TestDoctorCommandPrintsJSON(t *testing.T) {
	out := captureStdout(t, func() {
		if rc := doctorCommand([]string{"--json"}, "test-version"); rc != 0 {
			t.Fatalf("doctorCommand rc = %d, want 0", rc)
		}
	})

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("doctor --json output is not JSON: %v\n%s", err, out)
	}
	if decoded["version"] != "test-version" {
		t.Fatalf("version = %v, want test-version", decoded["version"])
	}
}

func TestDoctorCommandJSONUsesCLIProfile(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "alpha")
	actor := defaultCLIActorContext()
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`
default_model = "alpha"

[[providers]]
name = "alpha"
kind = "doctor-test-provider"
base_url = "https://alpha.example.invalid/v1"
model = "profile-model"
api_key_env = "KWU_ALPHA_DOCTOR_KEY"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.StoreCredentialLinesWithPaths(paths, []string{"KWU_ALPHA_DOCTOR_KEY=alpha-key"}); err != nil {
		t.Fatalf("StoreCredentialLinesWithPaths: %v", err)
	}

	out := captureStdout(t, func() {
		if rc := doctorCommand([]string{"--json"}, "test-version"); rc != 0 {
			t.Fatalf("doctorCommand rc = %d, want 0", rc)
		}
	})

	var decoded struct {
		Config struct {
			SourcePath string `json:"source_path"`
			UserPath   string `json:"user_path"`
		} `json:"config"`
		Sessions struct {
			Dir string `json:"dir"`
		} `json:"sessions"`
		Providers []struct {
			Name       string `json:"name"`
			KeyPresent bool   `json:"key_present"`
		} `json:"providers"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("doctor --json output is not JSON: %v\n%s", err, out)
	}

	wantUserSuffix := filepath.Join("profiles", "alpha", "config.toml")
	if !strings.Contains(decoded.Config.UserPath, wantUserSuffix) {
		t.Fatalf("config.user_path = %q, want alpha profile suffix %q", decoded.Config.UserPath, wantUserSuffix)
	}
	if !strings.Contains(decoded.Config.SourcePath, wantUserSuffix) {
		t.Fatalf("config.source_path = %q, want alpha profile suffix %q", decoded.Config.SourcePath, wantUserSuffix)
	}
	wantSessionSuffix := filepath.Join("profiles", "alpha", "sessions")
	if !strings.Contains(decoded.Sessions.Dir, wantSessionSuffix) {
		t.Fatalf("sessions.dir = %q, want alpha profile suffix %q", decoded.Sessions.Dir, wantSessionSuffix)
	}
	for _, p := range decoded.Providers {
		if p.Name == "alpha" {
			if !p.KeyPresent {
				t.Fatal("alpha provider key_present = false, want true")
			}
			return
		}
	}
	t.Fatalf("alpha provider missing from doctor report: %+v", decoded.Providers)
}

func TestRunDispatchesDoctor(t *testing.T) {
	out := captureStdout(t, func() {
		if rc := Run([]string{"doctor"}, "dispatch-version"); rc != 0 {
			t.Fatalf("Run doctor rc = %d, want 0", rc)
		}
	})
	if !strings.Contains(out, "Clouvia dispatch-version doctor") {
		t.Fatalf("doctor output missing header:\n%s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
