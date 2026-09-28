package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cloud "kwu/internal/cloud"
	"kwu/internal/config"
)

func TestWorkBuddyProjectPrivacyCanariesStayOutOfDiagnosticPayloads(t *testing.T) {
	isolateDesktopUserDirs(t)
	canaries := []string{
		"WB_PRIV_INSTRUCTION_7C4F",
		"WB_PRIV_DESCRIPTION_7C4F",
		`D:\private\workbuddy-7C4F`,
		"wb_access_token_7C4F",
		"wb_idempotency_key_7C4F",
		"wb_server_digest_7C4F",
		"wb_replay_payload_7C4F",
	}
	joined := strings.Join(canaries, "|")

	feedbackClient := &recordingFeedbackClient{
		result: cloud.ErrorReportSubmission{ReportID: "errpt_privacy", Status: cloud.ErrorReportStatusSubmitted},
	}
	app, _ := newFeedbackTestApp(t, feedbackClient)
	seedProjectPrivacyCanaries(app, canaries)
	if _, err := app.SubmitFeedback(FeedbackSubmitRequest{
		Description: "Synthetic diagnostic capture without Project content",
		AttemptID:   "attempt-privacy-capture",
	}); err != nil {
		t.Fatal(err)
	}
	errorReportPayload, err := json.Marshal(feedbackClient.lastCall().input)
	if err != nil {
		t.Fatal(err)
	}

	var telemetryPayload []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		telemetryPayload, _ = io.ReadAll(request.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	oldVersion, oldEndpoint := version, pingEndpoint
	t.Cleanup(func() { version, pingEndpoint = oldVersion, oldEndpoint })
	version, pingEndpoint = "v-project-privacy-test", server.URL
	configPath := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[desktop]\ntelemetry = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	telemetryApp := &App{}
	seedProjectPrivacyCanaries(telemetryApp, canaries)
	telemetryApp.sendStartupPing()

	normalizedErrors := map[string]error{
		"validation": cloudWailsError("cloud.test", &cloud.Error{Code: cloud.ErrorCode("VALIDATION_FAILED"), Message: joined, RequestID: "req-validation"}),
		"protocol":   cloudWailsError("cloud.test", &cloud.Error{Code: cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"), Message: joined, RequestID: "req-protocol"}),
		"panic":      cloudWailsError("cloud.test", fmt.Errorf("recovered panic: %s", joined)),
	}
	captures := map[string]string{
		"error-report": string(errorReportPayload),
		"telemetry":    string(telemetryPayload),
	}
	for name, normalized := range normalizedErrors {
		if normalized == nil {
			t.Fatalf("%s normalized error is nil", name)
		}
		captures["normalized-"+name] = normalized.Error()
	}
	for name, payload := range captures {
		if strings.TrimSpace(payload) == "" {
			t.Fatalf("%s capture is empty", name)
		}
		for _, canary := range canaries {
			if strings.Contains(payload, canary) {
				t.Fatalf("%s leaked Project privacy canary %q", name, canary)
			}
		}
	}
}

func seedProjectPrivacyCanaries(app *App, canaries []string) {
	app.tabRuntimes = map[string]*WorkspaceTab{
		"project-privacy-canary": {
			ID:            canaries[4],
			Scope:         "project",
			WorkspaceRoot: canaries[2],
			TopicID:       canaries[5],
			TopicTitle:    canaries[1],
			SessionPath:   canaries[6],
			StartupErr:    canaries[0],
			Label:         canaries[3],
			goal:          strings.Join(canaries, "|"),
		},
	}
	app.activeTabID = "project-privacy-canary"
}
