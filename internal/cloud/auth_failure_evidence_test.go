package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestLogAuthFailureUsesTypedAllowlistAndSessionPresenceOnly(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	observed := Session{
		State:                 SessionAuthenticated,
		AccessToken:           "access-secret",
		RefreshToken:          "refresh-secret",
		AccessTokenExpiresAt:  time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC),
		RefreshTokenExpiresAt: time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC),
	}
	cause := &Error{
		Code:       ErrorCode("SERVICE_INTERNAL_PRIVATE"),
		Message:    "server body canary bearer secret",
		Details:    json.RawMessage(`{"token":"details-secret"}`),
		RequestID:  "bad/request-id",
		HTTPStatus: 503,
		Retryable:  true,
	}
	logAuthFailure("explicit_refresh", observed, string(observed.State), string(SessionOffline), cause, "state_transition", "")

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatal(err)
	}
	if record["journal_source"] != "cloud" || record["journal_event"] != "auth.failure" {
		t.Fatalf("journal identity = %#v", record)
	}
	if record["code"] != "UNKNOWN_CLOUD_ERROR" || record["request_id"] != "" || record["http_status"] != float64(503) || record["retryable"] != true {
		t.Fatalf("safe auth fields = %#v", record)
	}
	if record["access_present"] != true || record["refresh_present"] != true || record["state_before"] != "authenticated" || record["state_after"] != "offline" {
		t.Fatalf("session transition fields = %#v", record)
	}
	serialized := output.String()
	for _, forbidden := range []string{"access-secret", "refresh-secret", "server body canary", "details-secret", "SERVICE_INTERNAL_PRIVATE", "bad/request-id"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("auth evidence leaked %q: %s", forbidden, serialized)
		}
	}
	if !strings.Contains(serialized, "cloud auth failure") {
		t.Fatal("structured auth event did not retain fixed diagnostic message")
	}
}

func TestAuthenticatedFailureEnqueuesEvidenceBeforeTerminalSave(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	scope, err := NewBindingScope(actor, "usr-auth", 3)
	if err != nil {
		t.Fatal(err)
	}
	session := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh"}
	store := &evidenceOrderingSessionStore{session: session, found: true, output: &output, checkSave: true}
	executor := &authenticatedRequestExecutor{sessions: store, now: func() time.Time { return time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC) }}
	cause := &Error{Code: ErrorAuthRequired, Message: "terminal message canary", RequestID: "req-terminal"}
	if err := executor.persistFailure(context.Background(), scope, session, cause); err != cause {
		t.Fatalf("persistFailure error = %v, want original cause", err)
	}
	if !store.evidenceBeforeSave {
		t.Fatal("terminal session save ran before auth failure evidence was enqueued")
	}
	if store.session.State != SessionExpired || store.session.AccessToken != "" || store.session.RefreshToken != "" {
		t.Fatalf("terminal session = %#v", store.session)
	}
	if strings.Contains(output.String(), "terminal message canary") {
		t.Fatalf("raw auth message leaked: %s", output.String())
	}
}

func TestAuthenticatedFailureEnqueuesEvidenceBeforeOfflineSaveAndPreservesCredential(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	scope, err := NewBindingScope(actor, "usr-auth", 4)
	if err != nil {
		t.Fatal(err)
	}
	session := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh"}
	store := &evidenceOrderingSessionStore{session: session, found: true, output: &output, checkSave: true}
	executor := &authenticatedRequestExecutor{sessions: store, now: time.Now}
	cause := &Error{Code: ErrorNetworkUnavailable, Message: "offline message canary", Retryable: true, RequestID: "req-offline"}
	if err := executor.persistFailure(context.Background(), scope, session, cause); err != cause {
		t.Fatalf("persistFailure error = %v, want original cause", err)
	}
	if !store.evidenceBeforeSave {
		t.Fatal("offline session save ran before auth failure evidence was enqueued")
	}
	if store.session.State != SessionOffline || store.session.AccessToken != "access" || store.session.RefreshToken != "refresh" {
		t.Fatalf("offline session = %#v", store.session)
	}
	if strings.Contains(output.String(), "offline message canary") {
		t.Fatalf("raw auth message leaked: %s", output.String())
	}
}

func TestAuthApplicationExplicitRefreshFailureEnqueuesBeforeSessionSave(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	app := newAuthApplication(t, fixtureErrorClient{err: &Error{Code: ErrorAuthRequired}}, NewTestPasswordEncryptor())
	if _, err := app.bindings.ConfirmBinding(ctx, actor, "usr-auth"); err != nil {
		t.Fatal(err)
	}
	scope, err := NewBindingScope(actor, "usr-auth", 1)
	if err != nil {
		t.Fatal(err)
	}
	session := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh"}
	store := &evidenceOrderingSessionStore{session: session, found: true, output: &output}
	app.sessions = NewProfileSessionRepository(app.bindings, store)
	if err := app.sessions.Save(ctx, actor, session); err != nil {
		t.Fatal(err)
	}
	store.checkSave = true
	cause := &Error{Code: ErrorAuthRequired, Message: "explicit refresh message canary", RequestID: "req-explicit"}
	if _, err := app.persistFailure(ctx, actor, session, cause); err != cause {
		t.Fatalf("persistFailure error = %v, want original cause", err)
	}
	if !store.evidenceBeforeSave {
		t.Fatal("explicit refresh session save ran before auth failure evidence was enqueued")
	}
	if store.session.State != SessionExpired || store.session.AccessToken != "" || store.session.RefreshToken != "" {
		t.Fatalf("explicit refresh session = %#v", store.session)
	}
}

func TestAuthApplicationSessionSaveFailureEmitsSafeFailureEvidence(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	app := newAuthApplication(t, fixtureErrorClient{err: &Error{Code: ErrorAuthRequired}}, NewTestPasswordEncryptor())
	if _, err := app.bindings.ConfirmBinding(ctx, actor, "usr-auth"); err != nil {
		t.Fatal(err)
	}
	scope, err := NewBindingScope(actor, "usr-auth", 1)
	if err != nil {
		t.Fatal(err)
	}
	session := Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh"}
	saveErr := errors.New("disk save message canary")
	store := &evidenceOrderingSessionStore{session: session, found: true, output: &output}
	app.sessions = NewProfileSessionRepository(app.bindings, store)
	if err := app.sessions.Save(ctx, actor, session); err != nil {
		t.Fatal(err)
	}
	store.checkSave = true
	store.saveErr = saveErr
	cause := &Error{Code: ErrorAuthRequired, Message: "terminal message canary", RequestID: "req-save-failure"}
	if _, err := app.persistFailure(ctx, actor, session, cause); err != saveErr {
		t.Fatalf("persistFailure error = %v, want save error", err)
	}
	serialized := output.String()
	if !store.evidenceBeforeSave || !strings.Contains(serialized, `"code":"PERSISTENCE_FAILURE"`) {
		t.Fatalf("save failure evidence = %s", serialized)
	}
	if strings.Contains(serialized, "disk save message canary") {
		t.Fatalf("raw save error leaked: %s", serialized)
	}
}

type evidenceOrderingSessionStore struct {
	mu                 sync.Mutex
	session            Session
	found              bool
	output             *bytes.Buffer
	checkSave          bool
	evidenceBeforeSave bool
	saveErr            error
}

func (s *evidenceOrderingSessionStore) Save(_ context.Context, session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checkSave {
		s.evidenceBeforeSave = strings.Contains(s.output.String(), `"journal_event":"auth.failure"`)
	}
	s.session = session
	s.found = true
	return s.saveErr
}

func (s *evidenceOrderingSessionStore) Load(context.Context, BindingScope) (Session, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session, s.found, nil
}

func (s *evidenceOrderingSessionStore) Clear(context.Context, BindingScope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.found = false
	return nil
}
