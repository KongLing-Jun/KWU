package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"

	"kwu/internal/userctx"
)

// AccountSwitchPreparation is an opaque, one-process account switch proof.
// Its authenticated session is deliberately kept in unexported state.
type AccountSwitchPreparation struct {
	state *accountSwitchPreparationState
}

type accountSwitchPreparationState struct {
	mu sync.Mutex

	owner                 *AuthApplication
	actor                 userctx.ActorContext
	expectedBinding       ProfileBinding
	expectedSession       Session
	expectedSessionExists bool
	candidateSession      Session
	view                  AuthView
	consumed              bool
}

// View returns the token-free identity that may be shown for confirmation.
func (p *AccountSwitchPreparation) View() AuthView {
	if p == nil || p.state == nil {
		return AuthView{}
	}
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	return p.state.view
}

// MarshalJSON intentionally serializes only the safe confirmation view.
func (p *AccountSwitchPreparation) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("null"), nil
	}
	return json.Marshal(p.View())
}

// PrepareAccountSwitch authenticates a different cloud account without
// changing the current binding, session, scoped state, or cached auth view.
func (a *AuthApplication) PrepareAccountSwitch(ctx context.Context, actor userctx.ActorContext, input LoginInput) (*AccountSwitchPreparation, error) {
	state, candidateSession, err := a.authenticateAccountSwitchCandidate(ctx, actor, input)
	if err != nil {
		return nil, err
	}
	actor = actor.Normalize()
	if err := validateAccountSwitchCandidateScope(actor, state, candidateSession); err != nil {
		return nil, err
	}
	current, currentSession, currentSessionExists, err := a.loadAccountSwitchSource(ctx, actor, state.User.CloudUserID)
	if err != nil {
		return nil, err
	}
	state, candidateSession, next, err := projectAccountSwitchCandidate(current, state, candidateSession)
	if err != nil {
		return nil, err
	}

	view := AuthView{State: AccountAuthenticated, Binding: next, User: state.User, LastSyncAt: state.LastSyncAt}
	return &AccountSwitchPreparation{state: &accountSwitchPreparationState{
		owner:                 a,
		actor:                 actor,
		expectedBinding:       current,
		expectedSession:       currentSession,
		expectedSessionExists: currentSessionExists,
		candidateSession:      candidateSession,
		view:                  view,
	}}, nil
}

func (a *AuthApplication) authenticateAccountSwitchCandidate(ctx context.Context, actor userctx.ActorContext, input LoginInput) (AccountState, Session, error) {
	if strings.TrimSpace(input.Phone) == "" || input.Password == "" {
		return AccountState{}, Session{}, fmt.Errorf("cloud: login phone and password are required")
	}
	if a.sessionClient != nil {
		return a.authenticateRemoteAccountSwitchCandidate(ctx, actor, input)
	}
	return a.authenticateFixtureAccountSwitchCandidate(ctx, actor, input)
}

func (a *AuthApplication) authenticateRemoteAccountSwitchCandidate(ctx context.Context, actor userctx.ActorContext, input LoginInput) (AccountState, Session, error) {
	result, err := a.sessionClient.Login(ctx, LoginRequest{
		Phone: input.Phone, Password: input.Password,
		CaptchaTicket: input.CaptchaTicket, CaptchaProvider: input.CaptchaProvider,
	})
	if err != nil {
		return AccountState{}, Session{}, err
	}
	if err := validateRemoteAuthenticationResult(result); err != nil {
		return AccountState{}, Session{}, err
	}
	actor = actor.Normalize()
	binding := ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: result.User.CloudUserID, Status: BindingBound, BindingGeneration: 1}
	scope, err := binding.Scope()
	if err != nil {
		return AccountState{}, Session{}, err
	}
	state := AccountState{Binding: binding, State: AccountAuthenticated, User: result.User, LastSyncAt: result.ServerTime}
	return state, bindRemoteSession(scope, result.Session), nil
}

func (a *AuthApplication) authenticateFixtureAccountSwitchCandidate(ctx context.Context, actor userctx.ActorContext, input LoginInput) (AccountState, Session, error) {
	payload, err := json.Marshal(struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}{Phone: input.Phone, Password: input.Password})
	if err != nil {
		return AccountState{}, Session{}, err
	}
	state, candidateSession, err := a.client.Login(ctx, a.fixtureRequest(actor, AccountOperationLogin, payload))
	if err != nil {
		return AccountState{}, Session{}, err
	}
	if err := state.Validate(); err != nil {
		return AccountState{}, Session{}, err
	}
	if err := candidateSession.Validate(); err != nil {
		return AccountState{}, Session{}, err
	}
	if state.State != AccountAuthenticated || candidateSession.State != SessionAuthenticated {
		return AccountState{}, Session{}, &Error{Code: ErrorAuthRequired, Message: "cloud: account switch requires an authenticated remote session"}
	}
	return state, candidateSession, nil
}

func validateAccountSwitchCandidateScope(actor userctx.ActorContext, state AccountState, candidateSession Session) error {
	remoteScope, err := state.Binding.Scope()
	if err != nil {
		return err
	}
	if state.Binding.ActorID != actor.ActorID || state.Binding.ProfileID != actor.ProfileID || candidateSession.Scope != remoteScope {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: account switch result belongs to another binding"}
	}
	return nil
}

func (a *AuthApplication) loadAccountSwitchSource(ctx context.Context, actor userctx.ActorContext, candidateCloudUserID string) (ProfileBinding, Session, bool, error) {
	current, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return ProfileBinding{}, Session{}, false, err
	}
	if current.Status != BindingBound || current.CloudUserID == candidateCloudUserID {
		return ProfileBinding{}, Session{}, false, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: account switch requires a different bound cloud user"}
	}
	currentSession, ok, err := a.sessions.Load(ctx, actor)
	if err != nil {
		return ProfileBinding{}, Session{}, false, err
	}
	return current, currentSession, ok, nil
}

func projectAccountSwitchCandidate(current ProfileBinding, state AccountState, candidateSession Session) (AccountState, Session, ProfileBinding, error) {
	next, err := comparedRebinding(current, state.User.CloudUserID)
	if err != nil {
		return AccountState{}, Session{}, ProfileBinding{}, err
	}
	nextScope, err := next.Scope()
	if err != nil {
		return AccountState{}, Session{}, ProfileBinding{}, err
	}
	state.Binding = next
	hasSubscription := state.Subscription.Plan != ""
	if hasSubscription {
		state.Subscription.Scope = nextScope
	}
	candidateSession.Scope = nextScope
	if err := validateProjectedAccountSwitchState(state, hasSubscription); err != nil {
		return AccountState{}, Session{}, ProfileBinding{}, err
	}
	if err := candidateSession.Validate(); err != nil {
		return AccountState{}, Session{}, ProfileBinding{}, err
	}
	return state, candidateSession, next, nil
}

func validateProjectedAccountSwitchState(state AccountState, hasSubscription bool) error {
	if hasSubscription {
		return state.Validate()
	}
	if err := state.Binding.Validate(); err != nil {
		return err
	}
	if err := state.User.Validate(); err != nil {
		return err
	}
	if state.State != AccountAuthenticated || state.LastSyncAt.IsZero() {
		return fmt.Errorf("cloud: account switch identity is incomplete")
	}
	return nil
}

// CommitAccountSwitch consumes a preparation and replaces the current cloud
// identity only if its binding and source-session snapshot are still unchanged.
func (a *AuthApplication) CommitAccountSwitch(ctx context.Context, preparation *AccountSwitchPreparation) (AuthView, error) {
	actor, expectedBinding, expectedSession, expectedSessionExists, candidateSession, candidateView, err := preparation.consume(a)
	if err != nil {
		return AuthView{}, err
	}

	a.identityMu.Lock()
	defer a.identityMu.Unlock()

	oldScope, err := a.validateAccountSwitchSource(ctx, actor, expectedBinding, expectedSession, expectedSessionExists)
	if err != nil {
		return AuthView{}, err
	}
	rebound, err := a.replaceAccountSwitchIdentity(ctx, actor, expectedBinding, oldScope, candidateSession, candidateView)
	if err != nil {
		return a.failClosedAccountSwitch(ctx, actor), err
	}
	candidateView.Binding = rebound
	return a.setView(actor, candidateView), nil
}

func (a *AuthApplication) validateAccountSwitchSource(ctx context.Context, actor userctx.ActorContext, expectedBinding ProfileBinding, expectedSession Session, expectedSessionExists bool) (BindingScope, error) {
	current, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	if current != expectedBinding {
		return BindingScope{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: binding changed after account switch preparation"}
	}
	currentSession, ok, err := a.sessions.Load(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	if ok != expectedSessionExists || (ok && currentSession != expectedSession) {
		return BindingScope{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: session changed after account switch preparation"}
	}
	return expectedBinding.Scope()
}

func (a *AuthApplication) replaceAccountSwitchIdentity(ctx context.Context, actor userctx.ActorContext, expectedBinding ProfileBinding, oldScope BindingScope, candidateSession Session, candidateView AuthView) (ProfileBinding, error) {
	if err := a.cleanup.Clear(ctx, oldScope); err != nil {
		return ProfileBinding{}, err
	}
	rebound, err := a.bindings.CompareAndRebind(ctx, actor, expectedBinding, candidateView.Binding.CloudUserID)
	if err != nil {
		return ProfileBinding{}, err
	}
	reboundScope, err := rebound.Scope()
	if err != nil {
		return ProfileBinding{}, err
	}
	if rebound != candidateView.Binding || candidateSession.Scope != reboundScope {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: prepared account switch no longer matches the rebound identity"}
	}
	if err := a.sessions.Save(ctx, actor, candidateSession); err != nil {
		return ProfileBinding{}, err
	}
	return rebound, nil
}

func (p *AccountSwitchPreparation) consume(owner *AuthApplication) (userctx.ActorContext, ProfileBinding, Session, bool, Session, AuthView, error) {
	if p == nil || p.state == nil {
		return userctx.ActorContext{}, ProfileBinding{}, Session{}, false, Session{}, AuthView{}, fmt.Errorf("cloud: account switch preparation is required")
	}
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	if p.state.consumed {
		return userctx.ActorContext{}, ProfileBinding{}, Session{}, false, Session{}, AuthView{}, fmt.Errorf("cloud: account switch preparation was already consumed")
	}
	p.state.consumed = true
	if p.state.owner != owner {
		return userctx.ActorContext{}, ProfileBinding{}, Session{}, false, Session{}, AuthView{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: account switch preparation belongs to another authentication runtime"}
	}
	return p.state.actor, p.state.expectedBinding, p.state.expectedSession, p.state.expectedSessionExists, p.state.candidateSession, p.state.view, nil
}

func (a *AuthApplication) failClosedAccountSwitch(ctx context.Context, actor userctx.ActorContext) AuthView {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	view := AuthView{State: AccountOffline}
	if current, err := a.bindings.ResolveBinding(recoveryCtx, actor); err == nil {
		view.Binding = current
	}
	return a.setView(actor, view)
}

// BindingScopedStateStore is the identity-cleanup boundary for state owned by
// another A-line stage, such as a WSS cursor or cloud-user snapshot.
type BindingScopedStateStore interface {
	Clear(context.Context, BindingScope) error
}

// IdentityCleanup clears authentication-owned state for exactly one binding.
// It clears session, cursor, then snapshot state and stops at the first error,
// so callers must treat an error as a partial cleanup. It does not inspect or
// rebind any A2-A4 records.
type IdentityCleanup struct {
	sessions  CloudSessionStore
	cursors   BindingScopedStateStore
	snapshots BindingScopedStateStore
}

func NewIdentityCleanup(sessions CloudSessionStore, cursors, snapshots BindingScopedStateStore) *IdentityCleanup {
	return &IdentityCleanup{sessions: sessions, cursors: cursors, snapshots: snapshots}
}

func (c *IdentityCleanup) Clear(ctx context.Context, scope BindingScope) error {
	if err := c.validateDependencies(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := c.sessions.Clear(ctx, scope); err != nil {
		return err
	}
	if err := c.cursors.Clear(ctx, scope); err != nil {
		return err
	}
	return c.snapshots.Clear(ctx, scope)
}

func (c *IdentityCleanup) validateDependencies() error {
	if c == nil || isNilDependency(c.sessions) || isNilDependency(c.cursors) || isNilDependency(c.snapshots) {
		return fmt.Errorf("cloud: identity cleanup is not configured")
	}
	return nil
}

func isNilDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// AuthenticatedRequestExecutor executes a protected operation for one full
// binding scope and refreshes/replays it at most once when its access token is
// rejected. It intentionally owns no user-visible state.
type AuthenticatedRequestExecutor interface {
	Do(context.Context, BindingScope, func(context.Context, string) error) error
}

type authenticatedRequestExecutor struct {
	sessions  CloudSessionStore
	client    CloudSessionClient
	now       func() time.Time
	refreshMu sync.Mutex
}

func NewAuthenticatedRequestExecutor(sessions CloudSessionStore, client CloudSessionClient, now func() time.Time) (AuthenticatedRequestExecutor, error) {
	if isNilDependency(sessions) || isNilDependency(client) {
		return nil, fmt.Errorf("cloud: authenticated request executor is not configured")
	}
	if now == nil {
		now = time.Now
	}
	return &authenticatedRequestExecutor{sessions: sessions, client: client, now: now}, nil
}

func (e *authenticatedRequestExecutor) Do(ctx context.Context, scope BindingScope, request func(context.Context, string) error) error {
	if err := validateAuthenticatedOperation(ctx, scope, request); err != nil {
		return err
	}
	session, err := e.load(ctx, scope)
	if err != nil {
		return err
	}
	session, err = e.refreshIfExpired(ctx, scope, session)
	if err != nil {
		return err
	}
	if err := e.executeOnceWithReplay(ctx, scope, session, request); err != nil {
		return e.persistFailure(ctx, scope, session, err)
	}
	return e.restoreOnline(ctx, scope)
}

func validateAuthenticatedOperation(ctx context.Context, scope BindingScope, request func(context.Context, string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if request == nil {
		return fmt.Errorf("cloud: authenticated request is required")
	}
	return nil
}
func (e *authenticatedRequestExecutor) refreshIfExpired(ctx context.Context, scope BindingScope, session Session) (Session, error) {
	if !session.AccessTokenExpiresAt.IsZero() && e.now().Before(session.AccessTokenExpiresAt) {
		return session, nil
	}
	return e.refresh(ctx, scope, session, "")
}
func (e *authenticatedRequestExecutor) executeOnceWithReplay(ctx context.Context, scope BindingScope, session Session, request func(context.Context, string) error) error {
	err := request(ctx, session.AccessToken)
	if !isCloudCode(err, ErrorCode("TOKEN_EXPIRED")) {
		return err
	}
	session, err = e.refresh(ctx, scope, session, session.AccessToken)
	if err != nil {
		return err
	}
	return request(ctx, session.AccessToken)
}

func (e *authenticatedRequestExecutor) load(ctx context.Context, scope BindingScope) (Session, error) {
	session, ok, err := e.sessions.Load(ctx, scope)
	if err != nil {
		return Session{}, err
	}
	if !ok || (session.State != SessionAuthenticated && session.State != SessionOffline) {
		return Session{}, &Error{Code: ErrorAuthRequired, Message: "authentication is required"}
	}
	return session, nil
}

func (e *authenticatedRequestExecutor) refresh(ctx context.Context, scope BindingScope, observed Session, expectedAccess string) (Session, error) {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()
	current, err := e.load(ctx, scope)
	if err != nil {
		return Session{}, err
	}
	if e.skipRefresh(current, observed, expectedAccess) {
		return current, nil
	}
	return e.rotateCurrentSession(ctx, scope, current)
}

func (e *authenticatedRequestExecutor) skipRefresh(current, observed Session, expectedAccess string) bool {
	if expectedAccess != "" {
		return current.AccessToken != expectedAccess
	}
	return current.AccessTokenExpiresAt != observed.AccessTokenExpiresAt && !current.AccessTokenExpiresAt.IsZero() && e.now().Before(current.AccessTokenExpiresAt)
}
func (e *authenticatedRequestExecutor) rotateCurrentSession(ctx context.Context, scope BindingScope, current Session) (Session, error) {
	if current.RefreshTokenExpiresAt.IsZero() || !e.now().Before(current.RefreshTokenExpiresAt) {
		return Session{}, e.persistFailureLocked(ctx, scope, current, &Error{Code: ErrorCode("REFRESH_TOKEN_EXPIRED"), Message: "refresh token expired"})
	}
	remote, err := e.client.Refresh(ctx, current.RefreshToken)
	if err != nil {
		return Session{}, e.persistFailureLocked(ctx, scope, current, err)
	}
	if err := remote.Validate(); err != nil {
		return Session{}, protocolError(err)
	}
	if remote.DeviceID != current.DeviceID {
		return Session{}, e.persistFailureLocked(ctx, scope, current, &Error{Code: ErrorCode("DEVICE_MISMATCH"), Message: "device mismatch"})
	}
	rotated := bindRemoteSession(scope, remote)
	if err := e.sessions.Save(ctx, rotated); err != nil {
		logAuthFailure("automatic_refresh", current, string(current.State), string(current.State), err, "save_failure", "PERSISTENCE_FAILURE")
		return Session{}, err
	}
	return rotated, nil
}

func (e *authenticatedRequestExecutor) persistFailure(ctx context.Context, scope BindingScope, observed Session, cause error) error {
	terminal := isSessionTerminalError(cause)
	retryable := isRetryableSessionFailure(cause)
	if !terminal && !retryable {
		return cause
	}
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()
	return e.persistFailureLockedPhase(ctx, scope, observed, cause, "protected_request")
}
func (e *authenticatedRequestExecutor) persistFailureLocked(ctx context.Context, scope BindingScope, observed Session, cause error) error {
	return e.persistFailureLockedPhase(ctx, scope, observed, cause, "automatic_refresh")
}

func (e *authenticatedRequestExecutor) persistFailureLockedPhase(ctx context.Context, scope BindingScope, observed Session, cause error, phase string) error {
	terminal := isSessionTerminalError(cause)
	current, found, err := e.sessions.Load(ctx, scope)
	if err != nil {
		logAuthFailure(phase, observed, string(observed.State), "unchanged", err, "load_failure", "PERSISTENCE_FAILURE")
		return err
	}
	if !found {
		logAuthFailure(phase, observed, string(observed.State), "unchanged", cause, "missing_observed", "")
		return cause
	}
	if current.State == SessionExpired && terminal {
		logAuthFailure(phase, current, string(current.State), string(current.State), cause, "already_terminal", "")
		return cause
	}
	if !sameObservedSession(current, observed) {
		logAuthFailure(phase, observed, string(current.State), "unchanged", cause, "stale_observed", "")
		return cause
	}
	before := current.State
	if terminal {
		current = terminateSession(current, sessionTerminationCode(cause), e.now().UTC())
	} else {
		current.State = SessionOffline
	}
	logAuthFailure(phase, observed, string(before), string(current.State), cause, "state_transition", "")
	if err := e.sessions.Save(ctx, current); err != nil {
		logAuthFailure(phase, observed, string(before), string(current.State), err, "save_failure", "PERSISTENCE_FAILURE")
		return err
	}
	return cause
}

func sameObservedSession(current, observed Session) bool {
	return current.Scope == observed.Scope &&
		current.AccessToken == observed.AccessToken &&
		current.RefreshToken == observed.RefreshToken &&
		current.DeviceID == observed.DeviceID &&
		current.AccessTokenExpiresAt.Equal(observed.AccessTokenExpiresAt) &&
		current.RefreshTokenExpiresAt.Equal(observed.RefreshTokenExpiresAt)
}

func sessionTerminationCode(cause error) ErrorCode {
	var cloudErr *Error
	if errors.As(cause, &cloudErr) && cloudErr.Code != "" {
		return cloudErr.Code
	}
	return ErrorAuthRequired
}

func terminateSession(session Session, reason ErrorCode, at time.Time) Session {
	session.State = SessionExpired
	session.AccessToken = ""
	session.RefreshToken = ""
	session.DeviceID = ""
	session.AccessTokenExpiresAt = time.Time{}
	session.RefreshTokenExpiresAt = time.Time{}
	session.IssuedAt = time.Time{}
	session.ExpiresAt = time.Time{}
	session.TerminationReason = reason
	session.TerminatedAt = at.UTC()
	return session
}
func (e *authenticatedRequestExecutor) restoreOnline(ctx context.Context, scope BindingScope) error {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()
	current, err := e.load(ctx, scope)
	if err != nil || current.State != SessionOffline {
		return err
	}
	current.State = SessionAuthenticated
	return e.sessions.Save(ctx, current)
}

// RemoteAccountApplication owns authenticated /me reads and refresh-token
// rotation for the real cloud runtime.
type RemoteAccountApplication struct {
	bindings BindingStore
	sessions *ProfileSessionRepository
	client   CloudSessionClient
	executor AuthenticatedRequestExecutor
	observer CurrentUserObserver
}

// CurrentUserObserver lets composition roots react to a validated authenticated
// bootstrap without teaching the generic Cloud layer product semantics.
type CurrentUserObserver func(context.Context, userctx.ActorContext, AccountState) error

func NewRemoteAccountApplication(bindings BindingStore, sessions *ProfileSessionRepository, client CloudSessionClient, executor AuthenticatedRequestExecutor, observers ...CurrentUserObserver) (*RemoteAccountApplication, error) {
	if isNilDependency(bindings) || sessions == nil || isNilDependency(client) || isNilDependency(executor) {
		return nil, fmt.Errorf("cloud: remote account application is not configured")
	}
	if len(observers) > 1 || (len(observers) == 1 && observers[0] == nil) {
		return nil, fmt.Errorf("cloud: remote account observer is invalid")
	}
	if !sameBindingDependency(sessions.bindings, bindings) {
		return nil, fmt.Errorf("cloud: remote account sessions must use its binding repository")
	}
	application := &RemoteAccountApplication{bindings: bindings, sessions: sessions, client: client, executor: executor}
	if len(observers) == 1 {
		application.observer = observers[0]
	}
	return application, nil
}

func (a *RemoteAccountApplication) CurrentUser(ctx context.Context, actor userctx.ActorContext) (AccountState, error) {
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return AccountState{}, err
	}
	if binding.Status != BindingBound {
		return AccountState{}, &Error{Code: ErrorAuthRequired, Message: "authentication is required"}
	}
	scope, err := binding.Scope()
	if err != nil {
		return AccountState{}, err
	}
	observed, observedFound, err := a.sessions.sessions.Load(ctx, scope)
	if err != nil {
		return AccountState{}, err
	}
	var state AccountState
	hydrate := func(ctx context.Context, token string) error {
		snapshot, requestErr := a.client.CurrentUser(ctx, token)
		if requestErr != nil {
			return requestErr
		}
		state, requestErr = remoteAccountState(binding, snapshot)
		return requestErr
	}
	operationErr := a.executor.Do(ctx, scope, hydrate)
	operationErr = a.retryCurrentUserAfterTerminal(ctx, actor, binding, scope, observed, observedFound, operationErr, hydrate)
	if operationErr != nil {
		return AccountState{}, operationErr
	}
	if a.observer != nil {
		if err := a.observer(ctx, actor.Normalize(), state); err != nil {
			return AccountState{}, err
		}
	}
	return state, nil
}

func (a *RemoteAccountApplication) retryCurrentUserAfterTerminal(ctx context.Context, actor userctx.ActorContext, binding ProfileBinding, scope BindingScope, observed Session, observedFound bool, operationErr error, hydrate func(context.Context, string) error) error {
	if operationErr == nil || !observedFound || !isSessionTerminalError(operationErr) {
		return operationErr
	}
	if !a.canRetryCurrentUser(ctx, actor, binding, scope, observed) {
		return operationErr
	}
	return a.executor.Do(ctx, scope, hydrate)
}

func (a *RemoteAccountApplication) canRetryCurrentUser(ctx context.Context, actor userctx.ActorContext, binding ProfileBinding, scope BindingScope, observed Session) bool {
	current, found, err := a.sessions.sessions.Load(ctx, scope)
	if err != nil || !found || current.Scope != scope || current.State != SessionAuthenticated || sameObservedSession(current, observed) {
		return false
	}
	currentBinding, err := a.bindings.ResolveBinding(ctx, actor)
	return err == nil && currentBinding == binding
}

func remoteAccountState(binding ProfileBinding, snapshot CurrentUserSnapshot) (AccountState, error) {
	if err := snapshot.User.Validate(); err != nil {
		return AccountState{}, protocolError(err)
	}
	if snapshot.User.CloudUserID != binding.CloudUserID {
		return AccountState{}, &Error{Code: ErrorProfileScopeMismatch, Message: "current user belongs to another binding"}
	}
	if err := snapshot.Balance.Validate(); err != nil {
		return AccountState{}, protocolError(err)
	}
	if snapshot.ServerTime.IsZero() {
		return AccountState{}, protocolError(fmt.Errorf("server_time is required"))
	}
	scope, err := binding.Scope()
	if err != nil {
		return AccountState{}, err
	}
	subscription := Subscription{Scope: scope, Plan: snapshot.Subscription.Plan, Status: snapshot.Subscription.Status, PeriodEnd: snapshot.Subscription.PeriodEnd, Version: snapshot.Subscription.Version}
	state := AccountState{
		Binding: binding, State: AccountAuthenticated, User: snapshot.User, Balance: snapshot.Balance,
		Subscription: subscription,
		Capabilities: CurrentUserCapabilities{
			ProjectsV1: append(json.RawMessage(nil), snapshot.Capabilities.ProjectsV1...),
			ProjectsV2: append(json.RawMessage(nil), snapshot.Capabilities.ProjectsV2...),
		},
		LastSyncAt: snapshot.ServerTime,
	}
	if err := state.Validate(); err != nil {
		return AccountState{}, protocolError(err)
	}
	return state, nil
}

func isCloudCode(err error, code ErrorCode) bool {
	var cloudErr *Error
	return errors.As(err, &cloudErr) && cloudErr.Code == code
}

func (a *AuthApplication) registerRemote(ctx context.Context, actor userctx.ActorContext, input RegisterInput, attempt authViewAttempt) (AuthView, error) {
	config, operationErr := a.registrationClient.PasswordCrypto(ctx)
	if operationErr != nil {
		return a.completeRemoteRegistration(ctx, actor, RegistrationResult{}, operationErr, attempt)
	}
	encrypted, operationErr := a.registrationEncryptor.Encrypt(input.Password, config)
	if operationErr != nil {
		return a.completeRemoteRegistration(ctx, actor, RegistrationResult{}, operationErr, attempt)
	}
	request := RegistrationRequest{
		Phone: input.Phone, SMSCode: input.SMSCode,
		EncryptedPassword: encrypted.EncryptedPassword, PasswordCrypto: encrypted.Crypto,
	}
	if operationErr := request.Validate(); operationErr != nil {
		return a.completeRemoteRegistration(ctx, actor, RegistrationResult{}, operationErr, attempt)
	}
	result, operationErr := a.registrationClient.Register(ctx, request)
	return a.completeRemoteRegistration(ctx, actor, result, operationErr, attempt)
}

func (a *AuthApplication) completeRemoteRegistration(ctx context.Context, actor userctx.ActorContext, result RegistrationResult, operationErr error, attempt authViewAttempt) (AuthView, error) {
	if operationErr == nil {
		if err := result.Validate(); err != nil {
			operationErr = protocolRegistrationError(err)
		} else {
			return a.completeRemoteAuthentication(ctx, actor, result.Authentication, nil, attempt)
		}
	}
	a.identityMu.Lock()
	defer a.identityMu.Unlock()
	if err := a.ensureIdentityAttemptCurrent(ctx, actor, attempt); err != nil {
		return a.stableView(actor), err
	}
	if cancelErr := authenticationCancellation(ctx, operationErr); cancelErr != nil {
		operationErr = cancelErr
	}
	return a.rollbackCanceledAttempt(attempt), operationErr
}

func protocolRegistrationError(cause error) error {
	return &Error{Code: ErrorCode("CLOUD_PROTOCOL_ERROR"), Message: "cloud registration response is invalid: " + cause.Error()}
}

// IsSessionTerminalError reports whether an authenticated cloud operation
// failed because the persisted session can no longer be used.
func IsSessionTerminalError(err error) bool {
	for _, code := range []ErrorCode{
		ErrorAuthRequired, ErrorCode("TOKEN_EXPIRED"), ErrorCode("REFRESH_TOKEN_INVALID"),
		ErrorCode("REFRESH_TOKEN_EXPIRED"), ErrorCode("REFRESH_TOKEN_REUSED"),
		ErrorCode("DEVICE_MISMATCH"), ErrorCode("ACCOUNT_DISABLED"),
	} {
		if isCloudCode(err, code) {
			return true
		}
	}
	return false
}

func isSessionTerminalError(err error) bool {
	return IsSessionTerminalError(err)
}

// AuthFailureEnqueuer is implemented structurally by the desktop diagnostic
// fan-out handler. Keeping this seam here avoids coupling the cloud package to
// the desktop module while allowing auth mutexes to enqueue without sync I/O.
type AuthFailureEnqueuer interface {
	EnqueueAuthFailure([]slog.Attr) bool
}

// logAuthFailure emits only fixed, credential-free fields. Desktop installs a
// local slog fan-out handler; other cloud hosts may continue observing this
// structured classification through their existing logger.
func logAuthFailure(phase string, observed Session, before, after string, cause error, outcome, codeOverride string) {
	code, requestID, httpStatus, retryable := authFailureFields(cause)
	if codeOverride != "" {
		code = codeOverride
	}
	attrs := []slog.Attr{
		slog.String("journal_source", "cloud"),
		slog.String("journal_event", "auth.failure"),
		slog.String("phase", phase),
		slog.String("code", code),
		slog.String("request_id", requestID),
		slog.Int("http_status", httpStatus),
		slog.Bool("retryable", retryable),
		slog.String("state_before", before),
		slog.String("state_after", after),
		slog.String("outcome", outcome),
		slog.Bool("access_present", observed.AccessToken != ""),
		slog.Bool("refresh_present", observed.RefreshToken != ""),
	}
	if !observed.AccessTokenExpiresAt.IsZero() {
		attrs = append(attrs, slog.Time("access_expiry", observed.AccessTokenExpiresAt.UTC()))
	}
	if !observed.RefreshTokenExpiresAt.IsZero() {
		attrs = append(attrs, slog.Time("refresh_expiry", observed.RefreshTokenExpiresAt.UTC()))
	}
	if enqueuer, ok := slog.Default().Handler().(AuthFailureEnqueuer); ok && enqueuer.EnqueueAuthFailure(attrs) {
		return
	}
	slog.LogAttrs(context.Background(), slog.LevelError, "cloud auth failure", attrs...)
}

func authFailureFields(cause error) (string, string, int, bool) {
	var cloudErr *Error
	if !errors.As(cause, &cloudErr) || cloudErr == nil {
		return "UNKNOWN_CLOUD_ERROR", "", 0, false
	}
	code := normalizeAuthFailureCode(cloudErr.Code)
	requestID := safeAuthFailureRequestID(cloudErr.RequestID)
	status := cloudErr.HTTPStatus
	if status < 100 || status > 599 {
		status = 0
	}
	return code, requestID, status, cloudErr.Retryable
}

func normalizeAuthFailureCode(code ErrorCode) string {
	switch code {
	case ErrorAuthRequired, ErrorCode("TOKEN_EXPIRED"), ErrorCode("REFRESH_TOKEN_INVALID"),
		ErrorCode("REFRESH_TOKEN_EXPIRED"), ErrorCode("REFRESH_TOKEN_REUSED"), ErrorCode("DEVICE_MISMATCH"),
		ErrorCode("ACCOUNT_DISABLED"), ErrorNetworkUnavailable, ErrorCode("RATE_LIMITED"),
		ErrorCode("SERVICE_UNAVAILABLE"), ErrorCode("UPSTREAM_UNAVAILABLE"):
		return string(code)
	default:
		return "UNKNOWN_CLOUD_ERROR"
	}
}

func safeAuthFailureRequestID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		return ""
	}
	for _, char := range value {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.:", char) {
			return ""
		}
	}
	return value
}

func (a *AuthApplication) persistRemoteAuthentication(ctx context.Context, actor userctx.ActorContext, current, expected ProfileBinding, session Session) (ProfileBinding, error) {
	staged, err := a.sessions.stage(ctx, session)
	if err != nil {
		return ProfileBinding{}, err
	}
	binding, err := a.confirmAuthenticationBinding(ctx, actor, current, expected)
	if err != nil {
		return ProfileBinding{}, errors.Join(err, staged.rollback(ctx))
	}
	if binding != expected {
		err := &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: account result belongs to another binding"}
		return ProfileBinding{}, errors.Join(err, staged.rollback(ctx))
	}
	return binding, nil
}

func (a *AuthApplication) confirmAuthenticationBinding(ctx context.Context, actor userctx.ActorContext, current, expected ProfileBinding) (ProfileBinding, error) {
	if current.Status == BindingExpired {
		replacer, ok := a.bindings.(rejectedBindingReplacer)
		if !ok {
			return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: expired binding cannot be replaced by this repository"}
		}
		return replacer.ReplaceRejectedBinding(ctx, actor, expected)
	}
	binding, err := a.bindings.ConfirmBinding(ctx, actor, expected.CloudUserID)
	if !isProfileMismatch(err) || current.Status != BindingBound || current.CloudUserID == expected.CloudUserID {
		return binding, err
	}
	_, currentSessionExists, loadErr := a.sessions.Load(ctx, actor)
	if loadErr != nil {
		return ProfileBinding{}, loadErr
	}
	if currentSessionExists {
		return binding, err
	}
	oldScope, scopeErr := current.Scope()
	if scopeErr != nil {
		return ProfileBinding{}, scopeErr
	}
	if cleanupErr := a.cleanup.Clear(ctx, oldScope); cleanupErr != nil {
		return ProfileBinding{}, cleanupErr
	}
	return a.bindings.CompareAndRebind(ctx, actor, current, expected.CloudUserID)
}
