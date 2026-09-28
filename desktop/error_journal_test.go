package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	managedconnector "kwu/desktop/internal/connector"
	"kwu/desktop/internal/diagnostics"
	"kwu/internal/cloud"
	"kwu/internal/userctx"
)

type cloudDiagnosticTestError struct{ message string }

func (e cloudDiagnosticTestError) Error() string { return e.message }

func TestCloudWailsErrorRecordsOneSanitizedDiagnostic(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(closeDesktopErrorJournal)
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})

	secret := `Bearer cloud-secret-token`
	safe := cloudWailsError("cloud.test", errors.Join(errors.New(secret), &cloud.Error{
		Code:       "UPSTREAM_ERROR",
		Message:    secret,
		RequestID:  "req-cloud-1",
		HTTPStatus: 503,
		Retryable:  true,
	}))
	if safe == nil || !strings.Contains(safe.Error(), "[diagnostic_id=") {
		t.Fatalf("safe error did not expose a diagnostic id: %v", safe)
	}
	closeDesktopErrorJournal()

	body := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
	if strings.Count(body, " cloud.test ") != 1 {
		t.Fatalf("cloud diagnostic count mismatch: %s", body)
	}
	diagnosticID := cloudErrorDiagnosticID(safe)
	for _, expected := range []string{" " + diagnosticID + " cloud.test [UPSTREAM_ERROR]", "request_id=req-cloud-1", "status=503", "retryable=true", "*errors.joinError:"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("cloud diagnostic missing %s: %s", expected, body)
		}
	}
	if strings.Contains(body, secret) {
		t.Fatalf("cloud diagnostic leaked secret: %s", body)
	}
}

func TestCloudSanitizersRecordExactlyOnceAndNilDoesNotRecord(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(closeDesktopErrorJournal)
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})

	if cloudWailsError("cloud.test", nil) != nil || cloudPaymentWailsError("cloud.test", nil) != nil {
		t.Fatal("nil cloud errors must remain nil")
	}
	payment := cloudPaymentWailsError("cloud.test", &cloud.Error{
		Code:       "UPSTREAM_ERROR",
		Message:    `password=payment-secret response body must stay private`,
		Details:    json.RawMessage(`{"credential":"private"}`),
		RequestID:  "req-payment-2",
		HTTPStatus: 502,
		Retryable:  true,
	})
	unsafeSummary := `upstream dial failed password=payment-secret 13800138000 owner@example.com Bearer abcdefghijklmnop C:\Users\alice\private\prompt.txt prompt=user-prompt-content tool_input=user-tool-content response_body=user-file-content`
	operation := cloudOperationError("cloud.test", cloudDiagnosticTestError{message: unsafeSummary})
	if payment == nil || operation == nil || operation.DiagnosticID == "" {
		t.Fatalf("sanitized errors lost diagnostics: payment=%v operation=%#v", payment, operation)
	}
	if strings.Contains(payment.Error(), "payment-secret") || strings.Contains(operation.Message, "upstream dial failed") {
		t.Fatalf("safe renderer errors exposed diagnostic summaries: payment=%q operation=%#v", payment.Error(), operation)
	}
	if paymentID := cloudErrorDiagnosticID(payment); paymentID == "" || paymentID == "req-payment-2" || paymentID == operation.DiagnosticID {
		t.Fatalf("diagnostic ids are missing, reused, or not independent: payment=%q operation=%q", paymentID, operation.DiagnosticID)
	}
	closeDesktopErrorJournal()

	body := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
	if count := strings.Count(body, " cloud.test "); count != 2 {
		t.Fatalf("cloud diagnostic count = %d, want payment plus operation: %s", count, body)
	}
	if !strings.Contains(body, "upstream dial failed") || !strings.Contains(body, "main.cloudDiagnosticTestError") {
		t.Fatalf("unknown concrete cause summary/type missing: %s", body)
	}
	for _, forbidden := range []string{"payment-secret", `"credential":"private"`, "13800138000", "owner@example.com", "abcdefghijklmnop", "user-prompt-content", "user-tool-content", "user-file-content"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("cloud diagnostic leaked %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `C:\Users\alice\private\prompt.txt`) {
		t.Fatalf("local file path was redacted: %s", body)
	}
}

func TestCloudSanitizerDoesNotDedupeFreshDiagnosticIDs(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(closeDesktopErrorJournal)
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})

	cause := &cloud.Error{Code: "UPSTREAM_ERROR", Message: "stable upstream failure", RequestID: "req-stable", HTTPStatus: 503, Retryable: true}
	first := cloudWailsError("cloud.test", cause)
	second := cloudWailsError("cloud.test", cause)
	firstID, secondID := cloudErrorDiagnosticID(first), cloudErrorDiagnosticID(second)
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatalf("fresh diagnostic ids = %q and %q", firstID, secondID)
	}
	closeDesktopErrorJournal()

	body := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
	if count := strings.Count(body, " cloud.test "); count != 2 {
		t.Fatalf("cloud diagnostic count = %d, want 2: %s", count, body)
	}
	for _, diagnosticID := range []string{firstID, secondID} {
		if !strings.Contains(body, " "+diagnosticID+" cloud.test ") {
			t.Fatalf("cloud diagnostic %q was deduplicated: %s", diagnosticID, body)
		}
	}
}

func TestDesktopErrorBoundariesKeepOriginalCauseAndAcceptedLogID(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(closeDesktopErrorJournal)
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})

	feedbackCause := errors.Join(errors.New("feedback transport failed"), &cloud.Error{
		Code: "UPSTREAM_ERROR", Message: "password=feedback-secret", RequestID: "req-feedback-1", HTTPStatus: 502, Retryable: true,
	})
	feedback := feedbackOperationError("feedback.submit", feedbackCause)
	if feedback == nil || feedback.DiagnosticID == "" || feedback.Message == "password=feedback-secret" {
		t.Fatalf("feedback error did not preserve a safe log id: %#v", feedback)
	}

	projectCause := errors.Join(errors.New("project protocol failed"), &cloud.Error{
		Code: "UPSTREAM_ERROR", Message: "Bearer project-secret", RequestID: "req-project-1", HTTPStatus: 503, Retryable: true,
	})
	projectSafe := projectProtocolWailsError(projectCause)
	projectID := cloudErrorDiagnosticID(projectSafe)
	if projectID == "" || strings.Contains(projectSafe.Error(), "project-secret") {
		t.Fatalf("project error did not preserve a safe log id: %v", projectSafe)
	}

	backgroundCause := errors.New("snapshot write failed password=snapshot-secret")
	recordDesktopBackgroundFailure("session.snapshot", backgroundCause)
	recordDesktopBackgroundFailure("session.snapshot", backgroundCause)
	connectorCause := newConnectorOAuthFailure(connectorOAuthStageKeyring, connectorOAuthClassKeyringRead, true, false, errors.New("credential vault read failed password=connector-secret"))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	(&App{}).recordConnectorOAuthRuntimeFailure(canceled, userctx.ActorContext{}, managedconnector.OwnerScope{}, "", "", connectorCause)
	closeDesktopErrorJournal()

	body := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
	for _, expected := range []string{
		" " + feedback.DiagnosticID + " feedback.submit [UPSTREAM_ERROR]",
		" " + projectID + " project.protocol [UPSTREAM_ERROR]",
		" session.snapshot | *errors.errorString: snapshot write failed",
		" connector.oauth.runtime | *main.connectorOAuthFailure:",
		"request_id=req-feedback-1", "request_id=req-project-1",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("boundary diagnostic missing %q: %s", expected, body)
		}
	}
	if count := strings.Count(body, " project.protocol "); count != 1 {
		t.Fatalf("project sanitizer recorded %d diagnostics, want 1: %s", count, body)
	}
	if count := strings.Count(body, " session.snapshot "); count != 1 {
		t.Fatalf("background duplicate count = %d, want 1: %s", count, body)
	}
	if count := strings.Count(body, " connector.oauth.runtime "); count != 1 {
		t.Fatalf("connector runtime diagnostic count = %d, want 1: %s", count, body)
	}
	for _, forbidden := range []string{"feedback-secret", "project-secret", "snapshot-secret", "connector-secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("boundary diagnostic leaked %q: %s", forbidden, body)
		}
	}
}

func TestInstallDesktopErrorJournalUsesProfileCacheAndPersistsPanic(t *testing.T) {
	root := t.TempDir()
	oldVersion, oldChannel := version, channel
	version, channel = "test-version", "canary"
	t.Cleanup(func() {
		version, channel = oldVersion, oldChannel
		closeDesktopErrorJournal()
	})
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})
	slog.Error("raw panic canary", "message", "do-not-persist")
	recordGoPanicFailure("test-panic")
	closeDesktopErrorJournal()

	dir := filepath.Join(root, "diagnostics", "errors")
	body := readDesktopJournal(t, dir)
	if !strings.Contains(body, " desktop.panic [GO_PANIC] | Panic") {
		t.Fatalf("panic event missing: %s", body)
	}
	if !strings.Contains(body, "go.error | raw panic canary") || strings.Contains(body, "do-not-persist") {
		t.Fatalf("raw slog content leaked: %s", body)
	}
}

func TestMarkedDesktopWarningUsesProductionCacheWithoutRecordingOrdinaryWarning(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KWU_CACHE_HOME", root)
	oldVersion := version
	version = "release-test"
	t.Cleanup(func() {
		version = oldVersion
		closeDesktopErrorJournal()
	})
	installDesktopErrorJournal(desktopProfile{})
	slog.Warn("ordinary warning", "scope", "desktop.startup")
	slog.Warn("selected warning", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "desktop.shutdown", "path", `C:\Users\alice\secret.txt`, "err", cloudDiagnosticTestError{message: "shutdown flush failed"})
	closeDesktopErrorJournal()

	dir := filepath.Join(root, "diagnostics", "errors")
	body := readDesktopJournal(t, dir)
	if strings.Count(strings.TrimSpace(body), "\n") != 0 || !strings.Contains(body, " desktop.shutdown | selected warning: main.cloudDiagnosticTestError: shutdown flush failed") {
		t.Fatalf("warning projection = %s", body)
	}
	if strings.Contains(body, "secret.txt") || strings.Contains(body, "alice") {
		t.Fatalf("warning path leaked: %s", body)
	}
}

func TestRecordClientErrorUsesFixedBoundaryAndDropsRendererText(t *testing.T) {
	root := t.TempDir()
	oldVersion, oldChannel := version, channel
	version, channel = "test-version", "canary"
	t.Cleanup(func() {
		version, channel = oldVersion, oldChannel
		closeDesktopErrorJournal()
	})
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})
	(&App{}).RecordClientError(ClientErrorRecord{
		Scope:      "workspace.preview",
		Code:       "NETWORK_ERROR",
		ErrorType:  "function=private RendererError",
		OccurredAt: "2026-08-25T10:00:00.123Z",
		StackFrames: []ClientErrorFrame{{
			Category: "function=/Users/alice/private.ts",
			Ordinal:  1,
			Line:     12,
			Column:   3,
		}},
	})
	closeDesktopErrorJournal()

	body := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
	if !strings.Contains(body, " workspace.preview [NETWORK_ERROR] | renderer failure") {
		t.Fatalf("frontend event projection = %s", body)
	}
	for _, forbidden := range []string{"function=private", "RendererError", "private.ts", "Users", "alice"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer canary leaked %q: %s", forbidden, body)
		}
	}
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})
	closeDesktopErrorJournal()
	if restarted := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors")); !strings.Contains(restarted, " workspace.preview [NETWORK_ERROR] | renderer failure") {
		t.Fatalf("frontend event was not readable after journal restart: %s", restarted)
	}
}

func TestRecordClientErrorWithoutJournalIsNoOp(t *testing.T) {
	closeDesktopErrorJournal()
	(&App{}).RecordClientError(ClientErrorRecord{Scope: "workspace.preview", Code: "NETWORK_ERROR", ErrorType: "Error"})
}

func TestRecordClientErrorDoesNotReenqueueBackendDiagnosticID(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(closeDesktopErrorJournal)
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})
	original := projectDiagnosticWailsError("project.renderer.test", errors.New("original project gateway cause"), &cloud.Error{Code: "PROJECT_ERROR", Message: "project operation failed"})
	diagnosticID := cloudErrorDiagnosticID(original)
	journal := currentDesktopErrorJournal()
	if journal == nil || !journal.OwnsLogID(diagnosticID) {
		t.Fatalf("backend diagnostic id = %q", diagnosticID)
	}
	sessionPrefix, _, ok := strings.Cut(diagnosticID, "-")
	if !ok {
		t.Fatalf("backend diagnostic id has no session prefix: %q", diagnosticID)
	}
	unissuedID := sessionPrefix + "-zzzzzz"
	if journal.OwnsLogID(unissuedID) {
		t.Fatalf("journal owns never-issued current-session ID %q", unissuedID)
	}
	(&App{}).RecordClientError(ClientErrorRecord{
		Scope: "account.operation", Code: "CLOUD_ERROR", ErrorType: "Error",
		DiagnosticID: diagnosticID,
	})
	(&App{}).RecordClientError(ClientErrorRecord{
		Scope: "workspace.preview", Code: "NETWORK_ERROR", ErrorType: "Error",
		DiagnosticID: "abcdef-0001",
	})
	(&App{}).RecordClientError(ClientErrorRecord{
		Scope: "workspace.same-session-unissued", Code: "NETWORK_ERROR", ErrorType: "Error",
		DiagnosticID: unissuedID,
	})
	closeDesktopErrorJournal()
	body := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
	if count := strings.Count(body, " "+diagnosticID+" "); count != 1 {
		t.Fatalf("backend diagnostic id count = %d, want original cause only: %s", count, body)
	}
	if !strings.Contains(body, " project.renderer.test | *errors.errorString: original project gateway cause") {
		t.Fatalf("original backend cause missing: %s", body)
	}
	if !strings.Contains(body, " workspace.preview [NETWORK_ERROR] | renderer failure") {
		t.Fatalf("ordinary renderer-only error was suppressed: %s", body)
	}
	if !strings.Contains(body, " workspace.same-session-unissued [NETWORK_ERROR] | renderer failure") {
		t.Fatalf("same-session unissued renderer error was suppressed: %s", body)
	}
	for _, unownedID := range []string{"abcdef-0001", unissuedID} {
		if strings.Contains(body, " "+unownedID+" ") {
			t.Fatalf("unowned diagnostic id %q was reused instead of assigning a fresh local id: %s", unownedID, body)
		}
	}
}

func TestConfirmedPersistenceLossBoundariesRecordDiagnostics(t *testing.T) {
	expectations := map[string][]string{
		"settings_app.go": {
			`recordDesktopBackgroundFailure("session.snapshot.settings.rebuild", err)`,
			`recordDesktopBackgroundFailure("session.snapshot.provider.access.remove", err)`,
			`recordDesktopBackgroundFailure("session.snapshot.provider.delete", err)`,
		},
		"app.go": {
			`recordDesktopBackgroundFailure("topic.model.selection.save", err)`,
		},
		"tabs.go": {
			`recordDesktopBackgroundFailure("topic.created_at.delete.load", err)`,
			`recordDesktopBackgroundFailure("topic.created_at.delete.save", err)`,
			`recordDesktopBackgroundFailure("topic.model.delete.load", err)`,
			`recordDesktopBackgroundFailure("topic.model.delete.save", err)`,
			`recordDesktopBackgroundFailure("topic.migration.titles.save", err)`,
			`recordDesktopBackgroundFailure("topic.migration.sources.save", err)`,
		},
	}
	for path, snippets := range expectations {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, snippet := range snippets {
			if !strings.Contains(string(body), snippet) {
				t.Errorf("%s does not retain diagnostic seam %s", path, snippet)
			}
		}
	}
}

func readDesktopJournal(t *testing.T, dir string) string {
	t.Helper()
	var body strings.Builder
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		body.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("read journal directory %q: %v", dir, err)
	}
	return body.String()
}
