package cloud

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kwu/internal/userctx"
)

const (
	crossExecutorTestBuild      = "test/source-under-test"
	crossExecutorTestVersion    = "test/source-under-test"
	crossExecutorTestCommit     = "not-recorded"
	crossExecutorWinnerID       = "project"
	crossExecutorLoserID        = "account"
	crossExecutorWinnerExecutor = "project-executor"
	crossExecutorLoserExecutor  = "account-executor"
	crossExecutorWinnerAttempt  = "project-refresh"
	crossExecutorLoserAttempt   = "account-refresh"
	crossExecutorOldAccess      = "access-cross-old"
	crossExecutorOldRefresh     = "refresh-cross-old"
	crossExecutorNewAccess      = "access-cross-new"
	crossExecutorNewRefresh     = "refresh-cross-new"
	crossExecutorReuseRequest   = "req-cross-refresh-reused"
	crossExecutorCurrentRequest = "req-cross-current-auth"
)

type crossExecutorRefreshGate struct {
	mu          sync.Mutex
	entered     int
	allEntered  chan struct{}
	winnerSaved chan struct{}
	savedOnce   sync.Once
}

func newCrossExecutorRefreshGate() *crossExecutorRefreshGate {
	return &crossExecutorRefreshGate{
		allEntered:  make(chan struct{}),
		winnerSaved: make(chan struct{}),
	}
}

func (g *crossExecutorRefreshGate) refresh(ctx context.Context, attemptID string, now time.Time) (RemoteSession, error) {
	g.mu.Lock()
	g.entered++
	if g.entered == 2 {
		close(g.allEntered)
	}
	g.mu.Unlock()

	select {
	case <-g.allEntered:
	case <-ctx.Done():
		return RemoteSession{}, ctx.Err()
	}
	if attemptID == crossExecutorWinnerID {
		return RemoteSession{
			AccessToken:           crossExecutorNewAccess,
			RefreshToken:          crossExecutorNewRefresh,
			DeviceID:              "device-cross",
			AccessTokenExpiresAt:  now.Add(time.Hour),
			RefreshTokenExpiresAt: now.Add(48 * time.Hour),
		}, nil
	}
	select {
	case <-g.winnerSaved:
		return RemoteSession{}, &Error{
			Code:      ErrorCode("REFRESH_TOKEN_REUSED"),
			Message:   "refresh token was reused",
			RequestID: crossExecutorReuseRequest,
		}
	case <-ctx.Done():
		return RemoteSession{}, ctx.Err()
	default:
	}
	select {
	case <-g.winnerSaved:
		return RemoteSession{}, &Error{
			Code:      ErrorCode("REFRESH_TOKEN_REUSED"),
			Message:   "refresh token was reused",
			RequestID: crossExecutorReuseRequest,
		}
	case <-ctx.Done():
		return RemoteSession{}, ctx.Err()
	}
}

func (g *crossExecutorRefreshGate) markWinnerSaved() {
	g.savedOnce.Do(func() { close(g.winnerSaved) })
}

type crossExecutorSessionClient struct {
	gate      *crossExecutorRefreshGate
	attemptID string
	now       time.Time
}

func (c *crossExecutorSessionClient) Login(context.Context, LoginRequest) (AuthenticationResult, error) {
	return AuthenticationResult{}, errors.New("cross-executor test client does not implement login")
}

func (c *crossExecutorSessionClient) Refresh(ctx context.Context, _ string) (RemoteSession, error) {
	return c.gate.refresh(ctx, c.attemptID, c.now)
}

func (c *crossExecutorSessionClient) Logout(context.Context, string) error { return nil }

func (c *crossExecutorSessionClient) CurrentUser(context.Context, string) (CurrentUserSnapshot, error) {
	return CurrentUserSnapshot{}, errors.New("cross-executor test client does not implement current user")
}

type crossExecutorSessionStore struct {
	CloudSessionStore
	gate *crossExecutorRefreshGate
}

func (s *crossExecutorSessionStore) Save(ctx context.Context, session Session) error {
	if err := s.CloudSessionStore.Save(ctx, session); err != nil {
		return err
	}
	if session.AccessToken == crossExecutorNewAccess {
		s.gate.markWinnerSaved()
	}
	return nil
}

func TestCrossExecutorRefreshReusePreservesReplacementThenScrubsCurrentSession(t *testing.T) {
	t.Parallel()
	diagnosticAt := time.Now()
	diagnosticAtUTC := diagnosticAt.UTC().Format(time.RFC3339Nano)
	localTimezoneName, localTimezoneOffsetSeconds := diagnosticAt.Zone()
	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	scope, err := NewBindingScope(userctxActorForCrossExecutorTest(), "usr_cross_executor", 7)
	if err != nil {
		t.Fatal(err)
	}
	old := Session{
		Scope:                 scope,
		State:                 SessionAuthenticated,
		AccessToken:           crossExecutorOldAccess,
		RefreshToken:          crossExecutorOldRefresh,
		DeviceID:              "device-cross",
		AccessTokenExpiresAt:  now.Add(-time.Minute),
		RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}
	baseStore := NewMemorySessionStore()
	gate := newCrossExecutorRefreshGate()
	store := &crossExecutorSessionStore{CloudSessionStore: baseStore, gate: gate}
	if err := store.Save(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	projectExecutor, err := NewAuthenticatedRequestExecutor(
		store,
		&crossExecutorSessionClient{gate: gate, attemptID: crossExecutorWinnerID, now: now},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	accountExecutor, err := NewAuthenticatedRequestExecutor(
		store,
		&crossExecutorSessionClient{gate: gate, attemptID: crossExecutorLoserID, now: now},
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatal(err)
	}
	if projectExecutor == accountExecutor {
		t.Fatal("cross-executor baseline requires two distinct executor instances")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	type result struct {
		executorID string
		err        error
	}
	results := make(chan result, 2)
	go func() {
		results <- result{executorID: crossExecutorWinnerID, err: projectExecutor.Do(ctx, scope, func(context.Context, string) error { return nil })}
	}()
	go func() {
		results <- result{executorID: crossExecutorLoserID, err: accountExecutor.Do(ctx, scope, func(context.Context, string) error { return nil })}
	}()

	var winnerErr, loserErr error
	for range 2 {
		result := <-results
		if result.executorID == crossExecutorWinnerID {
			winnerErr = result.err
		} else {
			loserErr = result.err
		}
	}
	if winnerErr != nil {
		t.Fatalf("project executor winner returned %v", winnerErr)
	}
	var loserCloudErr *Error
	if !errors.As(loserErr, &loserCloudErr) || loserCloudErr.Code != ErrorCode("REFRESH_TOKEN_REUSED") || loserCloudErr.RequestID != crossExecutorReuseRequest {
		t.Fatalf("account executor loser error code/request_id = %s/%s", cloudErrorCodeForTest(loserErr), cloudRequestIDForTest(loserErr))
	}

	replacement, found, err := store.Load(ctx, scope)
	if err != nil || !found {
		t.Fatalf("replacement session load failed: found=%t state=%s", found, replacement.State)
	}
	if replacement.State != SessionAuthenticated || replacement.AccessToken != crossExecutorNewAccess || replacement.RefreshToken != crossExecutorNewRefresh {
		t.Fatalf("replacement session state/presence = %s/%t/%t", replacement.State, replacement.AccessToken != "", replacement.RefreshToken != "")
	}
	if replacement.AccessTokenExpiresAt == old.AccessTokenExpiresAt || replacement.RefreshTokenExpiresAt == old.RefreshTokenExpiresAt {
		t.Fatalf("replacement expiry did not advance: access_changed=%t refresh_changed=%t", replacement.AccessTokenExpiresAt != old.AccessTokenExpiresAt, replacement.RefreshTokenExpiresAt != old.RefreshTokenExpiresAt)
	}
	if sameObservedSession(replacement, old) {
		t.Fatal("replacement is still equal to the loser observed session")
	}
	t.Logf("test_build=%s test_version=%s test_commit=%s observed_at_utc=%s local_timezone_name=%s local_timezone_offset_seconds=%d source=%s profile=%s generation=%d session=%s access_present=%t refresh_present=%t access_expiry_before=%s refresh_expiry_before=%s access_expiry_after=%s refresh_expiry_after=%s executor_test_id=%s refresh_attempt_id=%s trigger_type=local_expiry http_status=not_applicable error_code=none request_id=none sameObservedSession=n/a termination_skipped=n/a session_revision_supported=false", crossExecutorTestBuild, crossExecutorTestVersion, crossExecutorTestCommit, diagnosticAtUTC, localTimezoneName, localTimezoneOffsetSeconds, crossExecutorWinnerID, scope.ProfileID, scope.BindingGeneration, replacement.State, replacement.AccessToken != "", replacement.RefreshToken != "", old.AccessTokenExpiresAt.UTC().Format(time.RFC3339), old.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), replacement.AccessTokenExpiresAt.UTC().Format(time.RFC3339), replacement.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), crossExecutorWinnerExecutor, crossExecutorWinnerAttempt)
	t.Logf("test_build=%s test_version=%s test_commit=%s observed_at_utc=%s local_timezone_name=%s local_timezone_offset_seconds=%d source=%s profile=%s generation=%d session=%s access_present=%t refresh_present=%t access_expiry_before=%s refresh_expiry_before=%s access_expiry_after=%s refresh_expiry_after=%s executor_test_id=%s refresh_attempt_id=%s trigger_type=local_expiry http_status=not_applicable error_code=%s request_id=%s sameObservedSession=%t termination_skipped=true session_revision_supported=false", crossExecutorTestBuild, crossExecutorTestVersion, crossExecutorTestCommit, diagnosticAtUTC, localTimezoneName, localTimezoneOffsetSeconds, crossExecutorLoserID, scope.ProfileID, scope.BindingGeneration, replacement.State, replacement.AccessToken != "", replacement.RefreshToken != "", old.AccessTokenExpiresAt.UTC().Format(time.RFC3339), old.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), replacement.AccessTokenExpiresAt.UTC().Format(time.RFC3339), replacement.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), crossExecutorLoserExecutor, crossExecutorLoserAttempt, loserCloudErr.Code, loserCloudErr.RequestID, false)

	currentErr := accountExecutor.Do(ctx, scope, func(context.Context, string) error {
		return &Error{Code: ErrorAuthRequired, Message: "current session is no longer authorized", RequestID: crossExecutorCurrentRequest}
	})
	var currentCloudErr *Error
	if !errors.As(currentErr, &currentCloudErr) || currentCloudErr.Code != ErrorAuthRequired || currentCloudErr.RequestID != crossExecutorCurrentRequest {
		t.Fatalf("current replacement request error code/request_id = %s/%s", cloudErrorCodeForTest(currentErr), cloudRequestIDForTest(currentErr))
	}
	expired, found, err := store.Load(ctx, scope)
	if err != nil || !found {
		t.Fatalf("expired session load failed: found=%t state=%s", found, expired.State)
	}
	if expired.State != SessionExpired || expired.AccessToken != "" || expired.RefreshToken != "" || expired.DeviceID != "" || !expired.AccessTokenExpiresAt.IsZero() || !expired.RefreshTokenExpiresAt.IsZero() || expired.TerminationReason != ErrorAuthRequired || expired.TerminatedAt.IsZero() {
		t.Fatalf("expired session state/presence/reason = %s/%t/%t/%s/%t", expired.State, expired.AccessToken != "", expired.RefreshToken != "", expired.TerminationReason, !expired.TerminatedAt.IsZero())
	}
	t.Logf("test_build=%s test_version=%s test_commit=%s observed_at_utc=%s local_timezone_name=%s local_timezone_offset_seconds=%d source=%s profile=%s generation=%d session=%s access_present=%t refresh_present=%t access_expiry_before=%s refresh_expiry_before=%s access_expiry_after=%s refresh_expiry_after=%s executor_test_id=%s refresh_attempt_id=not_applicable trigger_type=protected_request http_status=not_applicable error_code=%s request_id=%s sameObservedSession=true termination_skipped=false session_revision_supported=false termination_time_present=%t", crossExecutorTestBuild, crossExecutorTestVersion, crossExecutorTestCommit, diagnosticAtUTC, localTimezoneName, localTimezoneOffsetSeconds, crossExecutorLoserID, scope.ProfileID, scope.BindingGeneration, expired.State, expired.AccessToken != "", expired.RefreshToken != "", replacement.AccessTokenExpiresAt.UTC().Format(time.RFC3339), replacement.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), expired.AccessTokenExpiresAt.UTC().Format(time.RFC3339), expired.RefreshTokenExpiresAt.UTC().Format(time.RFC3339), crossExecutorLoserExecutor, currentCloudErr.Code, currentCloudErr.RequestID, !expired.TerminatedAt.IsZero())
}

func userctxActorForCrossExecutorTest() (actor userctx.ActorContext) {
	return userctx.ActorContext{ActorID: "local", ProfileID: "cross-executor"}
}

func cloudErrorCodeForTest(err error) ErrorCode {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

func cloudRequestIDForTest(err error) string {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.RequestID
	}
	return ""
}
