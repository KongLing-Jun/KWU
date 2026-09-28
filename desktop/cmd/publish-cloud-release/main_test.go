package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kwu/desktop/internal/update"
)

func TestReleaseVersion(t *testing.T) {
	tests := []struct {
		input   string
		name    string
		code    int64
		wantErr bool
	}{
		{input: "v1.0.11", name: "1.0.11", code: 1_000_011},
		{input: "999.999.999", name: "999.999.999", code: 999_999_999},
		{input: "0.0.0", wantErr: true},
		{input: "1.1000.0", wantErr: true},
		{input: "1.0.0-beta", wantErr: true},
	}
	for _, test := range tests {
		name, code, err := releaseVersion(test.input)
		if (err != nil) != test.wantErr || name != test.name || code != test.code {
			t.Errorf("releaseVersion(%q) = (%q, %d, %v)", test.input, name, code, err)
		}
	}
}

func TestBuildReleaseRecords(t *testing.T) {
	dir := t.TempDir()
	manifestPath, dmgPath, notesPath := writeReleaseInputs(t, dir)
	records, err := buildReleaseRecords(manifestPath, dmgPath, notesPath)
	if err != nil {
		t.Fatalf("buildReleaseRecords: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("record count = %d", len(records))
	}
	wantTargets := []string{"windows/amd64", "windows/arm64", "macos/amd64", "macos/arm64"}
	for i, record := range records {
		if got := record.Platform + "/" + record.Architecture; got != wantTargets[i] {
			t.Errorf("record %d target = %q", i, got)
		}
		if record.VersionName != "1.0.11" || record.VersionCode != 1_000_011 || record.FileSizeBytes <= 0 || len(record.SHA256) != 64 {
			t.Errorf("record %d metadata = %+v", i, record)
		}
		if record.ReleaseNotes != "# Version 1.0.11\n\n- Faster startup" {
			t.Errorf("record %d notes = %q", i, record.ReleaseNotes)
		}
	}
	if records[2].DownloadURL != records[3].DownloadURL || !strings.HasSuffix(records[2].DownloadURL, "/Clouvia-darwin-universal.dmg") {
		t.Fatalf("macOS URLs = %q, %q", records[2].DownloadURL, records[3].DownloadURL)
	}
}

func TestBuildReleaseRecordsWindowsARM64Only(t *testing.T) {
	dir := t.TempDir()
	manifestPath, dmgPath, notesPath := writeReleaseInputs(t, dir)
	records, err := buildReleaseRecordsForTarget(manifestPath, dmgPath, notesPath, "windows-arm64")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	if record.Platform != "windows" || record.Architecture != "arm64" {
		t.Fatalf("record target = %s/%s, want windows/arm64", record.Platform, record.Architecture)
	}
}

func TestBuildReleaseRecordsForEachSelectiveTarget(t *testing.T) {
	dir := t.TempDir()
	manifestPath, dmgPath, notesPath := writeReleaseInputs(t, dir)
	tests := map[string][]string{
		"windows-amd64":   {"windows/amd64"},
		"windows-arm64":   {"windows/arm64"},
		"macos-universal": {"macos/amd64", "macos/arm64"},
	}
	for target, want := range tests {
		records, err := buildReleaseRecordsForTarget(manifestPath, dmgPath, notesPath, target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if len(records) != len(want) {
			t.Fatalf("%s record count = %d, want %d", target, len(records), len(want))
		}
		for i, record := range records {
			if got := record.Platform + "/" + record.Architecture; got != want[i] {
				t.Errorf("%s record %d = %s, want %s", target, i, got, want[i])
			}
		}
	}
}

func TestBuildReleaseRecordsRejectsUnsupportedNotes(t *testing.T) {
	dir := t.TempDir()
	manifestPath, dmgPath, notesPath := writeReleaseInputs(t, dir)
	for _, notes := range []string{"![screenshot](https://example.com/a.png)", "<img src=\"https://example.com/a.png\">"} {
		if err := os.WriteFile(notesPath, []byte(notes), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := buildReleaseRecords(manifestPath, dmgPath, notesPath); err == nil {
			t.Fatalf("notes %q should fail", notes)
		}
	}
}

func TestPublishReleaseRecordsSupportsPartialRetry(t *testing.T) {
	records := testRecords()
	const token = "top-secret-admin-token"
	var mu sync.Mutex
	stored := map[string]releaseRecord{}
	postCount := map[string]int{}
	failOnce := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/admin/client-releases" {
			if r.Header.Get("Authorization") != "Bearer "+token {
				t.Errorf("Authorization header was not set")
			}
			var record releaseRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Errorf("decode publish: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			key := record.Platform + "/" + record.Architecture
			mu.Lock()
			postCount[key]++
			if key == "macos/amd64" && failOnce {
				failOnce = false
				mu.Unlock()
				http.Error(w, "temporary failure", http.StatusInternalServerError)
				return
			}
			stored[key] = record
			mu.Unlock()
			writeReleaseEnvelope(w, record)
			return
		}
		if r.URL.Path == "/api/v1/client-releases/latest" {
			key := r.URL.Query().Get("platform") + "/" + r.URL.Query().Get("architecture")
			mu.Lock()
			record, ok := stored[key]
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeReleaseEnvelope(w, record)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	baseURL, err := validateCloudBaseURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	firstErr := publishReleaseRecords(context.Background(), server.Client(), baseURL, token, records)
	if firstErr == nil || !strings.Contains(firstErr.Error(), "macos/amd64") {
		t.Fatalf("first publish error = %v", firstErr)
	}
	if strings.Contains(firstErr.Error(), token) {
		t.Fatal("publish error leaked the administrator token")
	}
	if err := publishReleaseRecords(context.Background(), server.Client(), baseURL, token, records); err != nil {
		t.Fatalf("retry publish: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(stored) != 4 {
		t.Fatalf("stored targets = %d", len(stored))
	}
	for key, count := range postCount {
		if count != 1 && key != "macos/amd64" {
			t.Errorf("POST count for %s = %d, want 1", key, count)
		}
	}
	if postCount["macos/amd64"] != 2 {
		t.Errorf("POST count for macos/amd64 = %d, want failed attempt plus retry", postCount["macos/amd64"])
	}
}

func TestPublishReleaseRecordsSkipsMatchingLatest(t *testing.T) {
	expected := testRecords()[0]
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/client-releases/latest":
			writeReleaseEnvelope(w, expected)
		case "/api/v1/admin/client-releases":
			postCount++
			writeReleaseEnvelope(w, expected)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", []releaseRecord{expected}); err != nil {
		t.Fatalf("publishReleaseRecords: %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST count = %d, want 0 for an identical latest release", postCount)
	}
}

func TestPublishReleaseRecordsRejectsConflictingSameVersion(t *testing.T) {
	expected := testRecords()[0]
	latest := expected
	latest.ReleaseNotes = "different notes"
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/client-releases/latest" {
			writeReleaseEnvelope(w, latest)
			return
		}
		if r.Method == http.MethodPost {
			postCount++
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", []releaseRecord{expected})
	if err == nil || !strings.Contains(err.Error(), "release_notes") || !strings.Contains(err.Error(), "refusing to modify") {
		t.Fatalf("publishReleaseRecords error = %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST count = %d, want 0 for conflicting same version", postCount)
	}
}

func TestPublishReleaseRecordsRejectsNewerLatest(t *testing.T) {
	expected := testRecords()[0]
	latest := expected
	latest.VersionName = "1.0.12"
	latest.VersionCode++
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/client-releases/latest" {
			writeReleaseEnvelope(w, latest)
			return
		}
		if r.Method == http.MethodPost {
			postCount++
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", []releaseRecord{expected})
	if err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("publishReleaseRecords error = %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST count = %d, want 0 when latest is newer", postCount)
	}
}

func TestPublishReleaseRecordsPostsOnlyAfterLatestNotFound(t *testing.T) {
	expected := testRecords()[0]
	getCount := 0
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/client-releases/latest":
			getCount++
			if postCount == 0 {
				http.NotFound(w, r)
				return
			}
			writeReleaseEnvelope(w, expected)
		case "/api/v1/admin/client-releases":
			postCount++
			writeReleaseEnvelope(w, expected)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", []releaseRecord{expected}); err != nil {
		t.Fatalf("publishReleaseRecords: %v", err)
	}
	if getCount != 3 || postCount != 1 {
		t.Fatalf("GET count = %d, POST count = %d; want 3 and 1", getCount, postCount)
	}
}

func TestPublishReleaseRecordsPublishesOverOlderLatest(t *testing.T) {
	expected := testRecords()[0]
	older := expected
	older.VersionName = "1.0.10"
	older.VersionCode--
	getCount := 0
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/client-releases/latest":
			getCount++
			if postCount == 0 {
				writeReleaseEnvelope(w, older)
				return
			}
			writeReleaseEnvelope(w, expected)
		case "/api/v1/admin/client-releases":
			postCount++
			writeReleaseEnvelope(w, expected)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	if err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", []releaseRecord{expected}); err != nil {
		t.Fatalf("publishReleaseRecords: %v", err)
	}
	if getCount != 3 || postCount != 1 {
		t.Fatalf("GET count = %d, POST count = %d; want 3 and 1", getCount, postCount)
	}
}

func TestPublishReleaseRecordsPreflightsAllRecordsBeforeAnyPost(t *testing.T) {
	records := testRecords()[:2]
	newer := records[1]
	newer.VersionName = "1.0.12"
	newer.VersionCode++
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			http.Error(w, "unexpected write", http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/api/v1/client-releases/latest" {
			if r.URL.Query().Get("architecture") == "arm64" {
				writeReleaseEnvelope(w, newer)
				return
			}
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", records)
	if err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("publishReleaseRecords error = %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST count = %d, want 0 when a later record conflicts", postCount)
	}
}

func TestPublishReleaseRecordsPreflightsBothMacArchitecturesBeforeAnyPost(t *testing.T) {
	records := testRecords()[2:]
	conflict := records[1]
	conflict.SHA256 = strings.Repeat("b", 64)
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
			http.Error(w, "unexpected write", http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/api/v1/client-releases/latest" {
			if r.URL.Query().Get("architecture") == "arm64" {
				writeReleaseEnvelope(w, conflict)
				return
			}
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", records)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("publishReleaseRecords error = %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST count = %d, want 0 when one macOS architecture conflicts", postCount)
	}
}

func TestPublishReleaseRecordsDoesNotTreatOtherErrorsAsMissing(t *testing.T) {
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", testRecords()[:1])
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("publishReleaseRecords error = %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST count = %d, want 0 after latest lookup failure", postCount)
	}
}

func TestPublishReleaseRecordValidatesEveryResponseField(t *testing.T) {
	expected := testRecords()[0]
	for _, test := range releaseRecordMismatchTests() {
		t.Run(test.field, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s", r.Method)
				}
				actual := expected
				test.mutate(&actual)
				writeReleaseEnvelope(w, actual)
			}))
			defer server.Close()

			err := publishReleaseRecord(context.Background(), server.Client(), server.URL, "secret", expected)
			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("publishReleaseRecord error = %v, want field %q", err, test.field)
			}
		})
	}
}

func TestPublishReleaseRecordsValidatesEveryLatestField(t *testing.T) {
	expected := testRecords()[0]
	for _, test := range releaseRecordMismatchTests() {
		t.Run(test.field, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/admin/client-releases":
					writeReleaseEnvelope(w, expected)
				case "/api/v1/client-releases/latest":
					actual := expected
					test.mutate(&actual)
					writeReleaseEnvelope(w, actual)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			err := publishReleaseRecords(context.Background(), server.Client(), server.URL, "secret", []releaseRecord{expected})
			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("publishReleaseRecords error = %v, want field %q", err, test.field)
			}
		})
	}
}

func TestPublishReleaseRecordRejectsConflictWithoutPatch(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	err := publishReleaseRecord(context.Background(), server.Client(), server.URL, "secret", testRecords()[0])
	if err == nil || !strings.Contains(err.Error(), "409 Conflict") || !strings.Contains(err.Error(), "refusing to modify") {
		t.Fatalf("publishReleaseRecord error = %v", err)
	}
	if len(methods) != 1 || methods[0] != http.MethodPost {
		t.Fatalf("request methods = %v, want one POST and no PATCH", methods)
	}
}

func TestRunPreflightRequiresOnlyBaseURLAndToken(t *testing.T) {
	const token = "preflight-admin-token"
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/admin/client-releases/history" || r.URL.Query().Get("limit") != "1" {
			t.Errorf("preflight request = %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("Authorization header was not set")
		}
		writeHistoryEnvelope(w)
	}))
	defer server.Close()
	t.Setenv(cloudReleaseTokenEnv, token)

	var output strings.Builder
	if err := run([]string{"-base-url", server.URL, "-preflight"}, &output); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if requestCount != 1 || !strings.Contains(output.String(), "preflight passed") {
		t.Fatalf("request count = %d output = %q", requestCount, output.String())
	}
}

func TestRunRecordStatePreflightChecksAllMacRecordsWithoutPost(t *testing.T) {
	dir := t.TempDir()
	manifestPath, dmgPath, notesPath := writeReleaseInputs(t, dir)
	latestCount := 0
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/client-releases/history":
			writeHistoryEnvelope(w)
		case "/api/v1/client-releases/latest":
			latestCount++
			http.NotFound(w, r)
		default:
			if r.Method == http.MethodPost {
				postCount++
			}
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(cloudReleaseTokenEnv, "record-preflight-token")

	if err := run([]string{
		"-preflight",
		"-manifest", manifestPath,
		"-dmg", dmgPath,
		"-notes", notesPath,
		"-base-url", server.URL,
		"-target", "macos-universal",
	}, io.Discard); err != nil {
		t.Fatalf("record-state preflight: %v", err)
	}
	if latestCount != 2 || postCount != 0 {
		t.Fatalf("latest GET count = %d, POST count = %d; want 2 and 0", latestCount, postCount)
	}
}

func TestCloudPreflightRejectsStatusAndSchemaFailuresWithoutLeakingToken(t *testing.T) {
	const token = "must-not-leak-from-preflight"
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "route missing", status: http.StatusNotFound},
		{name: "token invalid", status: http.StatusUnauthorized},
		{name: "administrator required", status: http.StatusForbidden},
		{name: "migration unavailable", status: http.StatusInternalServerError},
		{name: "wrong schema", status: http.StatusOK, body: `{"data":{}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				if test.body != "" {
					_, _ = io.WriteString(w, test.body)
				} else {
					_, _ = io.WriteString(w, token)
				}
			}))
			defer server.Close()

			err := preflightCloudRelease(context.Background(), server.Client(), server.URL, token)
			if err == nil || strings.Contains(err.Error(), token) {
				t.Fatalf("preflight error = %v", err)
			}
		})
	}
}

func TestRunPreflightsBeforePublishing(t *testing.T) {
	dir := t.TempDir()
	manifestPath, dmgPath, notesPath := writeReleaseInputs(t, dir)
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv(cloudReleaseTokenEnv, "secret")

	err := run([]string{
		"-manifest", manifestPath,
		"-dmg", dmgPath,
		"-notes", notesPath,
		"-base-url", server.URL,
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("run error = %v", err)
	}
	if postCount != 0 {
		t.Fatalf("POST count = %d, want no writes after failed preflight", postCount)
	}
}

func TestDryRunDoesNotReadTokenOrCallCloud(t *testing.T) {
	dir := t.TempDir()
	manifestPath, dmgPath, notesPath := writeReleaseInputs(t, dir)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("dry-run made a cloud request")
	}))
	defer server.Close()
	t.Setenv(cloudReleaseTokenEnv, "")
	restore := readReleaseToken
	readReleaseToken = func() (string, error) {
		t.Fatal("dry-run prompted for a token")
		return "", nil
	}
	t.Cleanup(func() { readReleaseToken = restore })
	var output strings.Builder
	if err := run([]string{
		"-manifest", manifestPath,
		"-dmg", dmgPath,
		"-notes", notesPath,
		"-base-url", server.URL,
		"-dry-run",
	}, &output); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if strings.Count(output.String(), "dry-run:") != 4 {
		t.Fatalf("dry-run output = %q", output.String())
	}
}

func TestCloudReleaseTokenUsesHiddenPromptWhenEnvironmentIsEmpty(t *testing.T) {
	t.Setenv(cloudReleaseTokenEnv, "")
	restore := readReleaseToken
	readReleaseToken = func() (string, error) { return "prompted-token", nil }
	t.Cleanup(func() { readReleaseToken = restore })
	token, err := cloudReleaseToken()
	if err != nil || token != "prompted-token" {
		t.Fatalf("cloudReleaseToken = %q, %v", token, err)
	}
}

func TestPublishErrorDoesNotEchoTokenFromResponse(t *testing.T) {
	const token = "must-not-appear-in-errors"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, token, http.StatusInternalServerError)
	}))
	defer server.Close()
	err := publishReleaseRecords(context.Background(), server.Client(), server.URL, token, testRecords()[:1])
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatalf("redacted publish error = %v", err)
	}
}

func writeReleaseInputs(t *testing.T, dir string) (string, string, string) {
	t.Helper()
	base := "https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com/releases/desktop-v1.0.11"
	manifest := update.Manifest{
		Version: "v1.0.11",
		Platforms: map[string]update.Asset{
			"windows-amd64": testAsset(base+"/Clouvia-windows-amd64-installer.exe", "amd64 installer"),
			"windows-arm64": testAsset(base+"/Clouvia-windows-arm64-installer.exe", "arm64 installer"),
			"darwin-amd64":  testAsset(base+"/Clouvia-darwin-universal.dmg", "universal dmg"),
			"darwin-arm64":  testAsset(base+"/Clouvia-darwin-universal.dmg", "universal dmg"),
		},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "latest.json")
	if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	dmgPath := filepath.Join(dir, "Clouvia-darwin-universal.dmg")
	if err := os.WriteFile(dmgPath, []byte("universal dmg"), 0o644); err != nil {
		t.Fatal(err)
	}
	notesPath := filepath.Join(dir, "release-notes.md")
	if err := os.WriteFile(notesPath, []byte("# Version 1.0.11\n\n- Faster startup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifestPath, dmgPath, notesPath
}

func testAsset(url, payload string) update.Asset {
	sum := sha256.Sum256([]byte(payload))
	return update.Asset{URL: url, Size: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])}
}

func testRecords() []releaseRecord {
	records := make([]releaseRecord, 0, 4)
	for _, target := range [][2]string{{"windows", "amd64"}, {"windows", "arm64"}, {"macos", "amd64"}, {"macos", "arm64"}} {
		records = append(records, releaseRecord{
			Platform: target[0], Architecture: target[1], VersionName: "1.0.11", VersionCode: 1_000_011,
			DownloadURL: fmt.Sprintf("https://example.com/%s-%s", target[0], target[1]), FileSizeBytes: 10,
			SHA256: strings.Repeat("a", 64), ReleaseNotes: "notes",
		})
	}
	return records
}

type releaseRecordMismatchTest struct {
	field  string
	mutate func(*releaseRecord)
}

func releaseRecordMismatchTests() []releaseRecordMismatchTest {
	return []releaseRecordMismatchTest{
		{field: "platform", mutate: func(record *releaseRecord) { record.Platform = "different" }},
		{field: "architecture", mutate: func(record *releaseRecord) { record.Architecture = "different" }},
		{field: "version_name", mutate: func(record *releaseRecord) { record.VersionName = "2.0.0" }},
		{field: "version_code", mutate: func(record *releaseRecord) { record.VersionCode++ }},
		{field: "download_url", mutate: func(record *releaseRecord) { record.DownloadURL += "-different" }},
		{field: "file_size_bytes", mutate: func(record *releaseRecord) { record.FileSizeBytes++ }},
		{field: "sha256", mutate: func(record *releaseRecord) { record.SHA256 = strings.Repeat("b", 64) }},
		{field: "release_notes", mutate: func(record *releaseRecord) { record.ReleaseNotes += " different" }},
	}
}

func writeReleaseEnvelope(w http.ResponseWriter, record releaseRecord) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"release": record}})
}

func writeHistoryEnvelope(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{"items": []any{}, "next_cursor": nil, "has_more": false},
	})
}
