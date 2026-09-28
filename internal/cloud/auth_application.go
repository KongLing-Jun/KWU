package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"kwu/internal/userctx"
)

// AuthView is the UI-safe current cloud identity. Session tokens are never
// copied into this view.
type AuthView struct {
	State      AccountStatus
	Binding    ProfileBinding
	User       CloudUser
	LastSyncAt time.Time
}

type RegisterInput struct {
	Phone, Password, SMSCode string
}

// LoginInput contains user-form fields for the desktop/domain boundary. It is
// not a frozen cloud wire contract.
type LoginInput struct {
	Phone, Password                string
	CaptchaPassed                  bool
	CaptchaTicket, CaptchaProvider string
}

type AuthApplicationDependencies struct {
	Bindings              BindingStore
	Sessions              *ProfileSessionRepository
	Cleanup               *IdentityCleanup
	Captcha               CaptchaGateway
	Encryptor             PasswordEncryptor
	Client                CloudAccountClient
	SessionClient         CloudSessionClient
	RegistrationClient    CloudRegistrationClient
	PasswordResetClient   CloudPasswordResetClient
	RegistrationEncryptor *RegistrationPasswordEncryptor
	FixtureVersion        string
}

// AuthApplication is the single L1 authentication state machine. Its only
// persistence path for tokens is ProfileSessionRepository.
type AuthApplication struct {
	bindings              BindingStore
	sessions              *ProfileSessionRepository
	cleanup               *IdentityCleanup
	captcha               CaptchaGateway
	encryptor             PasswordEncryptor
	client                CloudAccountClient
	sessionClient         CloudSessionClient
	registrationClient    CloudRegistrationClient
	passwordResetClient   CloudPasswordResetClient
	registrationEncryptor *RegistrationPasswordEncryptor
	fixtureVersion        string

	identityMu sync.Mutex
	mu         sync.RWMutex
	views      map[string]authViewEntry
}

type authViewEntry struct {
	view     AuthView
	stable   AuthView
	revision uint64
}

type authViewAttempt struct {
	key      string
	prior    AuthView
	revision uint64
	identity authIdentitySnapshot
}

type authIdentitySnapshot struct {
	binding       ProfileBinding
	session       Session
	sessionExists bool
}

type authenticationCommit struct {
	actor    userctx.ActorContext
	state    AccountState
	session  Session
	expected ProfileBinding
}

func NewAuthApplication(deps AuthApplicationDependencies) (*AuthApplication, error) {
	if isNilDependency(deps.Bindings) || deps.Sessions == nil || deps.Cleanup == nil {
		return nil, fmt.Errorf("cloud: auth application is not configured")
	}
	if err := validateAuthApplicationDependencies(deps); err != nil {
		return nil, err
	}
	if !sameBindingDependency(deps.Sessions.bindings, deps.Bindings) {
		return nil, fmt.Errorf("cloud: auth application sessions must use its binding repository")
	}
	return &AuthApplication{
		bindings: deps.Bindings, sessions: deps.Sessions, cleanup: deps.Cleanup,
		captcha: deps.Captcha, encryptor: deps.Encryptor, client: deps.Client, sessionClient: deps.SessionClient,
		registrationClient: deps.RegistrationClient, passwordResetClient: deps.PasswordResetClient,
		registrationEncryptor: deps.RegistrationEncryptor,
		fixtureVersion:        deps.FixtureVersion, views: make(map[string]authViewEntry),
	}, nil
}

func validateAuthApplicationDependencies(deps AuthApplicationDependencies) error {
	fixtureConfigured := !isNilDependency(deps.Client)
	productionConfigured := !isNilDependency(deps.SessionClient)
	if fixtureConfigured == productionConfigured {
		return fmt.Errorf("cloud: auth application requires exactly one account client")
	}
	if fixtureConfigured {
		return validateFixtureAuthDependencies(deps)
	}
	return validateRealRegistrationDependencies(deps)
}

func validateFixtureAuthDependencies(deps AuthApplicationDependencies) error {
	if !isNilDependency(deps.RegistrationClient) || !isNilDependency(deps.PasswordResetClient) || deps.RegistrationEncryptor != nil {
		return fmt.Errorf("cloud: fixture auth application cannot use real registration dependencies")
	}
	if isNilDependency(deps.Captcha) || isNilDependency(deps.Encryptor) || strings.TrimSpace(deps.FixtureVersion) == "" {
		return fmt.Errorf("cloud: fixture auth application is not configured")
	}
	return nil
}

func validateRealRegistrationDependencies(deps AuthApplicationDependencies) error {
	registrationConfigured := !isNilDependency(deps.RegistrationClient)
	encryptorConfigured := deps.RegistrationEncryptor != nil
	if registrationConfigured != encryptorConfigured {
		return fmt.Errorf("cloud: real registration requires client and encryptor dependencies")
	}
	return nil
}

func sameBindingDependency(resolver BindingResolver, store BindingStore) bool {
	left, right := reflect.ValueOf(resolver), reflect.ValueOf(store)
	return left.IsValid() && right.IsValid() && left.Type() == right.Type() && left.Kind() == reflect.Pointer && left.Pointer() == right.Pointer()
}

// SendRegisterSMS verifies fixture/development CAPTCHA before constructing a
// cloud request. Real mode uses SendRegisterSMSWithProof to validate the opaque
// provider proof without replacing or interpreting it.
func (a *AuthApplication) SendRegisterSMS(ctx context.Context, actor userctx.ActorContext, captchaPassed bool, phone string) error {
	return a.sendRegisterSMS(ctx, actor, phone, CaptchaProof{}, captchaPassed)
}

func (a *AuthApplication) SendRegisterSMSWithProof(ctx context.Context, actor userctx.ActorContext, phone string, proof CaptchaProof) error {
	return a.sendRegisterSMS(ctx, actor, phone, proof, false)
}

func (a *AuthApplication) sendRegisterSMS(ctx context.Context, actor userctx.ActorContext, phone string, proof CaptchaProof, captchaPassed bool) error {
	if a.client == nil && a.registrationClient == nil {
		return &Error{Code: ErrorCode("CAPABILITY_UNAVAILABLE"), Message: "cloud registration is unavailable"}
	}
	proof, err := a.prepareCaptchaProof(ctx, proof, captchaPassed)
	if err != nil {
		return err
	}
	if a.registrationClient != nil {
		_, err := a.registrationClient.SendRegistrationSMS(ctx, SendRegistrationSMSRequest{Phone: phone, CaptchaTicket: proof.Ticket, CaptchaProvider: proof.Provider})
		return err
	}
	payload, err := json.Marshal(struct {
		Phone  string `json:"phone"`
		Ticket string `json:"captcha_ticket"`
	}{Phone: phone, Ticket: proof.Ticket})
	if err != nil {
		return err
	}
	return a.client.SendRegisterSMS(ctx, a.fixtureRequest(actor, AccountOperationSendRegisterSMS, payload))
}

func (a *AuthApplication) CaptchaConfig(ctx context.Context) (CaptchaConfig, error) {
	if a.registrationClient == nil {
		return CaptchaConfig{}, &Error{Code: ErrorCode("CAPABILITY_UNAVAILABLE"), Message: "cloud captcha configuration is unavailable"}
	}
	return a.registrationClient.CaptchaConfig(ctx)
}

func (a *AuthApplication) SendPasswordResetSMSWithProof(ctx context.Context, phone string, proof CaptchaProof) (PasswordResetSMSResult, error) {
	if isNilDependency(a.passwordResetClient) {
		return PasswordResetSMSResult{}, &Error{Code: ErrorCode("CAPABILITY_UNAVAILABLE"), Message: "cloud password reset is unavailable"}
	}
	proof, err := a.prepareCaptchaProof(ctx, proof, false)
	if err != nil {
		return PasswordResetSMSResult{}, err
	}
	return a.passwordResetClient.SendPasswordResetSMS(ctx, SendPasswordResetSMSRequest{
		Phone: phone, CaptchaTicket: proof.Ticket, CaptchaProvider: proof.Provider,
	})
}

func (a *AuthApplication) ResetPassword(ctx context.Context, input PasswordResetRequest) (PasswordResetResult, error) {
	if isNilDependency(a.passwordResetClient) {
		return PasswordResetResult{}, &Error{Code: ErrorCode("CAPABILITY_UNAVAILABLE"), Message: "cloud password reset is unavailable"}
	}
	return a.passwordResetClient.ResetPassword(ctx, input)
}

func (a *AuthApplication) Register(ctx context.Context, actor userctx.ActorContext, input RegisterInput) (AuthView, error) {
	if a.client == nil && a.registrationClient == nil {
		return a.stableView(actor), &Error{Code: ErrorCode("CAPABILITY_UNAVAILABLE"), Message: "cloud registration is unavailable"}
	}
	if strings.TrimSpace(input.Phone) == "" || strings.TrimSpace(input.SMSCode) == "" || input.Password == "" {
		return AuthView{}, fmt.Errorf("cloud: registration phone, password, and SMS code are required")
	}
	attempt, err := a.beginIdentityAttempt(ctx, actor, AccountAuthenticating)
	if err != nil {
		return a.stableView(actor), err
	}
	if a.registrationClient != nil {
		return a.registerRemote(ctx, actor, input, attempt)
	}
	encrypted, err := a.encryptor.Encrypt(input.Password)
	if err != nil {
		return a.completeAuthentication(ctx, actor, AccountState{}, Session{}, err, attempt)
	}
	payload, err := json.Marshal(struct {
		Phone             string         `json:"phone"`
		SMSCode           string         `json:"sms_code"`
		EncryptedPassword string         `json:"encrypted_password"`
		PasswordCrypto    PasswordCrypto `json:"password_crypto"`
	}{Phone: input.Phone, SMSCode: input.SMSCode, EncryptedPassword: encrypted.EncryptedPassword, PasswordCrypto: encrypted.Crypto})
	if err != nil {
		return a.completeAuthentication(ctx, actor, AccountState{}, Session{}, err, attempt)
	}
	state, session, err := a.client.Register(ctx, a.fixtureRequest(actor, AccountOperationRegister, payload))
	return a.completeAuthentication(ctx, actor, state, session, err, attempt)
}

func (a *AuthApplication) Login(ctx context.Context, actor userctx.ActorContext, input LoginInput) (AuthView, error) {
	if strings.TrimSpace(input.Phone) == "" || input.Password == "" {
		return AuthView{}, fmt.Errorf("cloud: login phone and password are required")
	}
	input, err := a.PrepareLoginInput(ctx, input)
	if err != nil {
		return a.stableView(actor), err
	}
	attempt, err := a.beginIdentityAttempt(ctx, actor, AccountAuthenticating)
	if err != nil {
		return a.stableView(actor), err
	}
	if a.sessionClient != nil {
		result, err := a.sessionClient.Login(ctx, LoginRequest{
			Phone: input.Phone, Password: input.Password,
			CaptchaTicket: input.CaptchaTicket, CaptchaProvider: input.CaptchaProvider,
		})
		return a.completeRemoteAuthentication(ctx, actor, result, err, attempt)
	}
	payload, err := json.Marshal(struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}{Phone: input.Phone, Password: input.Password})
	if err != nil {
		return a.completeAuthentication(ctx, actor, AccountState{}, Session{}, err, attempt)
	}
	state, session, err := a.client.Login(ctx, a.fixtureRequest(actor, AccountOperationLogin, payload))
	return a.completeAuthentication(ctx, actor, state, session, err, attempt)
}

// PrepareLoginInput converts development verification or validates a real
// opaque provider proof before a cloud login request.
func (a *AuthApplication) PrepareLoginInput(ctx context.Context, input LoginInput) (LoginInput, error) {
	if a.sessionClient == nil {
		return input, nil
	}
	proof, err := a.prepareCaptchaProof(ctx, CaptchaProof{Ticket: input.CaptchaTicket, Provider: input.CaptchaProvider}, input.CaptchaPassed)
	if err != nil {
		return LoginInput{}, err
	}
	input.CaptchaTicket = proof.Ticket
	input.CaptchaProvider = proof.Provider
	return input, nil
}

func (a *AuthApplication) prepareCaptchaProof(ctx context.Context, proof CaptchaProof, captchaPassed bool) (CaptchaProof, error) {
	if !isNilDependency(a.captcha) {
		permit, err := a.captcha.Verify(ctx, captchaPassed)
		if err != nil {
			return CaptchaProof{}, err
		}
		ticket, err := permit.Ticket()
		if err != nil {
			return CaptchaProof{}, err
		}
		return CaptchaProof{Ticket: ticket, Provider: LoginCaptchaProviderAliyunCaptchaV2}, nil
	}
	if err := proof.Validate(); err != nil {
		return CaptchaProof{}, err
	}
	return proof, nil
}

func (a *AuthApplication) Refresh(ctx context.Context, actor userctx.ActorContext) (AuthView, error) {
	a.identityMu.Lock()
	identity, err := a.captureIdentity(ctx, actor)
	if err != nil {
		view := a.stableView(actor)
		a.identityMu.Unlock()
		return view, err
	}
	if !identity.sessionExists {
		view := a.setView(actor, AuthView{State: AccountAnonymous, Binding: identity.binding})
		a.identityMu.Unlock()
		return view, nil
	}
	attempt := a.beginTransient(actor, AccountRefreshing)
	attempt.identity = identity
	session := identity.session
	a.identityMu.Unlock()

	var refreshed Session
	if a.sessionClient != nil {
		remote, refreshErr := a.sessionClient.Refresh(ctx, session.RefreshToken)
		err = refreshErr
		if refreshErr == nil {
			refreshed = bindRemoteSession(session.Scope, remote)
		}
	} else {
		refreshed, err = a.client.RefreshSession(ctx, session)
	}
	a.identityMu.Lock()
	defer a.identityMu.Unlock()
	if err := a.ensureIdentityAttemptCurrent(ctx, actor, attempt); err != nil {
		return a.stableView(actor), err
	}
	if cancelErr := authenticationCancellation(ctx, err); cancelErr != nil {
		return a.rollbackCanceledAttempt(attempt), cancelErr
	}
	commitCtx, cancelCommit := localAuthCommitContext(ctx)
	defer cancelCommit()
	if err != nil {
		return a.persistFailure(commitCtx, actor, session, err)
	}
	if refreshed.Scope != session.Scope {
		err := &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: refreshed session belongs to another binding"}
		return a.persistFailure(commitCtx, actor, session, err)
	}
	if err := a.sessions.Save(commitCtx, actor, refreshed); err != nil {
		logAuthFailure("explicit_refresh", session, string(session.State), string(session.State), err, "save_failure", "PERSISTENCE_FAILURE")
		return a.failureView(actor, err), err
	}
	view := attempt.prior
	view.State = AccountAuthenticated
	view.Binding = attempt.identity.binding
	return a.setView(actor, view), nil
}

func (a *AuthApplication) Recover(ctx context.Context, actor userctx.ActorContext) (AuthView, error) {
	a.identityMu.Lock()
	defer a.identityMu.Unlock()
	return a.stateLocked(ctx, actor)
}

func (a *AuthApplication) Logout(ctx context.Context, actor userctx.ActorContext) error {
	a.identityMu.Lock()
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		a.identityMu.Unlock()
		return err
	}
	if binding.Status != BindingBound {
		err := a.setAnonymous(actor, binding)
		a.identityMu.Unlock()
		return err
	}
	scope, err := binding.Scope()
	if err != nil {
		a.identityMu.Unlock()
		return err
	}
	var refreshToken string
	if session, ok, loadErr := a.sessions.Load(ctx, actor); loadErr != nil {
		a.identityMu.Unlock()
		return loadErr
	} else if ok {
		refreshToken = session.RefreshToken
	}
	if err := a.cleanup.Clear(ctx, scope); err != nil {
		a.identityMu.Unlock()
		return err
	}
	if err := a.setAnonymous(actor, binding); err != nil {
		a.identityMu.Unlock()
		return err
	}
	a.identityMu.Unlock()
	if a.sessionClient != nil {
		if refreshToken == "" {
			return nil
		}
		return a.sessionClient.Logout(ctx, refreshToken)
	}
	return a.client.Logout(ctx, scope)
}

func (a *AuthApplication) completeRemoteAuthentication(ctx context.Context, actor userctx.ActorContext, result AuthenticationResult, operationErr error, attempt authViewAttempt) (AuthView, error) {
	a.identityMu.Lock()
	defer a.identityMu.Unlock()
	if view, err, handled := a.authenticationOperationResult(ctx, actor, operationErr, attempt); handled {
		return view, err
	}
	if err := validateRemoteAuthenticationResult(result); err != nil {
		return a.failureView(actor, err), err
	}
	commitCtx, cancelCommit := localAuthCommitContext(ctx)
	defer cancelCommit()
	return a.commitRemoteAuthentication(commitCtx, actor, result, attempt)
}

func validateRemoteAuthenticationResult(result AuthenticationResult) error {
	if err := result.User.Validate(); err != nil {
		return err
	}
	if err := result.Session.Validate(); err != nil {
		return err
	}
	if result.ServerTime.IsZero() {
		return fmt.Errorf("cloud: authentication server_time is required")
	}
	return nil
}

func (a *AuthApplication) commitRemoteAuthentication(ctx context.Context, actor userctx.ActorContext, result AuthenticationResult, attempt authViewAttempt) (AuthView, error) {
	actor = actor.Normalize()
	current, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil && !isCredentialStateRejected(err) {
		return a.failureView(actor, err), err
	}
	expected, err := a.expectedAuthenticationBinding(ctx, actor, current, result.User.CloudUserID)
	if err != nil {
		return a.failureView(actor, err), err
	}
	scope, err := expected.Scope()
	if err != nil {
		return a.failureView(actor, err), err
	}
	binding, err := a.persistRemoteAuthentication(ctx, actor, current, expected, bindRemoteSession(scope, result.Session))
	if err != nil {
		if isProfileMismatch(err) {
			a.rollbackView(attempt)
			return AuthView{State: AccountOffline, Binding: expected, User: result.User, LastSyncAt: result.ServerTime}, err
		}
		return a.failureView(actor, err), err
	}
	return a.setView(actor, AuthView{State: AccountAuthenticated, Binding: binding, User: result.User, LastSyncAt: result.ServerTime}), nil
}

func bindRemoteSession(scope BindingScope, remote RemoteSession) Session {
	return Session{
		Scope: scope, State: SessionAuthenticated,
		AccessToken: remote.AccessToken, RefreshToken: remote.RefreshToken, DeviceID: remote.DeviceID,
		AccessTokenExpiresAt: remote.AccessTokenExpiresAt, RefreshTokenExpiresAt: remote.RefreshTokenExpiresAt,
	}
}

// State derives the six UI states from the current actor's bound session.
func (a *AuthApplication) State(ctx context.Context, actor userctx.ActorContext) (AuthView, error) {
	a.identityMu.Lock()
	defer a.identityMu.Unlock()
	return a.stateLocked(ctx, actor)
}

func (a *AuthApplication) stateLocked(ctx context.Context, actor userctx.ActorContext) (AuthView, error) {
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return a.bindingStateFailure(actor, binding, err)
	}
	if transient, ok := a.transientView(actor, binding); ok {
		return transient, nil
	}
	return a.stateForResolvedBinding(ctx, actor, binding)
}

func (a *AuthApplication) bindingStateFailure(actor userctx.ActorContext, binding ProfileBinding, err error) (AuthView, error) {
	if !isCredentialStateRejected(err) {
		return AuthView{}, err
	}
	view := AuthView{State: AccountExpired, Binding: rejectedBinding(actor.Normalize(), binding.BindingGeneration)}
	return a.setView(actor, view), err
}

func (a *AuthApplication) stateForResolvedBinding(ctx context.Context, actor userctx.ActorContext, binding ProfileBinding) (AuthView, error) {
	if binding.Status == BindingUnbound {
		return a.setView(actor, AuthView{State: AccountAnonymous, Binding: binding}), nil
	}
	if binding.Status != BindingBound {
		return a.setView(actor, AuthView{State: AccountOffline, Binding: binding}), nil
	}
	scope, err := binding.Scope()
	if err != nil {
		return a.setView(actor, AuthView{State: AccountOffline, Binding: binding}), err
	}
	return a.stateFromBoundSession(ctx, actor, binding, scope)
}

func (a *AuthApplication) stateFromBoundSession(ctx context.Context, actor userctx.ActorContext, binding ProfileBinding, scope BindingScope) (AuthView, error) {
	session, ok, err := a.sessions.Load(ctx, actor)
	if err != nil {
		if isCredentialStateRejected(err) {
			return a.setView(actor, AuthView{
				State:   AccountExpired,
				Binding: rejectedBinding(actor.Normalize(), binding.BindingGeneration),
			}), err
		}
		return a.setView(actor, AuthView{State: AccountOffline, Binding: binding}), err
	}
	if !ok {
		return a.setView(actor, AuthView{State: AccountAnonymous, Binding: binding}), nil
	}
	if session.Scope != scope {
		err := &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: loaded session belongs to another resolved binding"}
		return a.setView(actor, AuthView{State: AccountOffline, Binding: binding}), err
	}
	state := accountStatusFromSessionState(session.State)
	view := a.cachedView(actor, binding)
	view.State, view.Binding = state, binding
	return a.setView(actor, view), nil
}

func accountStatusFromSessionState(state SessionState) AccountStatus {
	switch state {
	case SessionAuthenticated:
		return AccountAuthenticated
	case SessionExpired:
		return AccountExpired
	case SessionOffline:
		return AccountOffline
	default:
		return AccountAnonymous
	}
}

// ApplyThirdPartySync accepts only the current binding and intentionally does
// not alter the authentication fact for any of the six sync statuses.
func (a *AuthApplication) ApplyThirdPartySync(ctx context.Context, actor userctx.ActorContext, syncState ThirdPartySync) error {
	if err := syncState.Validate(); err != nil {
		return err
	}
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return err
	}
	scope, err := binding.Scope()
	if err != nil {
		return profileScopeMismatch(actor.Normalize(), binding)
	}
	if syncState.Scope != scope {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: third-party sync belongs to another binding"}
	}
	return nil
}

func (a *AuthApplication) completeAuthentication(ctx context.Context, actor userctx.ActorContext, state AccountState, session Session, operationErr error, attempt authViewAttempt) (AuthView, error) {
	a.identityMu.Lock()
	defer a.identityMu.Unlock()
	if view, err, handled := a.authenticationOperationResult(ctx, actor, operationErr, attempt); handled {
		return view, err
	}
	commitCtx, cancelCommit := localAuthCommitContext(ctx)
	defer cancelCommit()
	commit, failure, err := a.validateAuthenticationResult(actor, state, session, attempt)
	if err != nil {
		return failure, err
	}
	commit, failure, err = a.prepareAuthenticationCommit(commitCtx, commit, attempt)
	if err != nil {
		return failure, err
	}
	return a.commitAuthentication(commitCtx, commit, attempt)
}

func (a *AuthApplication) authenticationOperationResult(ctx context.Context, actor userctx.ActorContext, operationErr error, attempt authViewAttempt) (AuthView, error, bool) {
	if err := a.ensureIdentityAttemptCurrent(ctx, actor, attempt); err != nil {
		return a.stableView(actor), err, true
	}
	if cancelErr := authenticationCancellation(ctx, operationErr); cancelErr != nil {
		return a.rollbackCanceledAttempt(attempt), cancelErr, true
	}
	if operationErr == nil {
		return AuthView{}, nil, false
	}
	if isProfileMismatch(operationErr) {
		a.rollbackView(attempt)
		return attempt.prior, operationErr, true
	}
	return a.failureView(actor, operationErr), operationErr, true
}

func (a *AuthApplication) validateAuthenticationResult(actor userctx.ActorContext, state AccountState, session Session, attempt authViewAttempt) (authenticationCommit, AuthView, error) {
	if err := state.Validate(); err != nil {
		return authenticationCommit{}, a.failureView(actor, err), err
	}
	if err := session.Validate(); err != nil {
		return authenticationCommit{}, a.failureView(actor, err), err
	}
	actor = actor.Normalize()
	remoteScope, err := state.Binding.Scope()
	if err != nil {
		return authenticationCommit{}, a.failureView(actor, err), err
	}
	if state.Binding.ActorID != actor.ActorID || state.Binding.ProfileID != actor.ProfileID || session.Scope != remoteScope {
		err := &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: account result belongs to another binding"}
		a.rollbackView(attempt)
		return authenticationCommit{}, AuthView{State: AccountOffline, Binding: state.Binding, User: state.User, LastSyncAt: state.LastSyncAt}, err
	}
	return authenticationCommit{actor: actor, state: state, session: session}, AuthView{}, nil
}

func (a *AuthApplication) prepareAuthenticationCommit(ctx context.Context, commit authenticationCommit, attempt authViewAttempt) (authenticationCommit, AuthView, error) {
	current, err := a.bindings.ResolveBinding(ctx, commit.actor)
	if err != nil && !isCredentialStateRejected(err) {
		return authenticationCommit{}, a.failureView(commit.actor, err), err
	}
	expected, err := a.expectedAuthenticationBinding(ctx, commit.actor, current, commit.state.User.CloudUserID)
	if err != nil {
		if isProfileMismatch(err) {
			a.rollbackView(attempt)
		}
		return authenticationCommit{}, a.failureView(commit.actor, err), err
	}
	expectedScope, err := expected.Scope()
	if err != nil {
		return authenticationCommit{}, a.failureView(commit.actor, err), err
	}
	commit.state.Binding = expected
	commit.state.Subscription.Scope = expectedScope
	commit.session.Scope = expectedScope
	if err := commit.state.Validate(); err != nil {
		return authenticationCommit{}, a.failureView(commit.actor, err), err
	}
	if err := commit.session.Validate(); err != nil {
		return authenticationCommit{}, a.failureView(commit.actor, err), err
	}
	commit.expected = expected
	return commit, AuthView{}, nil
}

func (a *AuthApplication) commitAuthentication(ctx context.Context, commit authenticationCommit, attempt authViewAttempt) (AuthView, error) {
	staged, err := a.sessions.stage(ctx, commit.session)
	if err != nil {
		return a.failureView(commit.actor, err), err
	}
	binding, err := a.confirmPreparedAuthenticationBinding(ctx, commit.actor, commit.expected)
	if err != nil {
		if isProfileMismatch(err) {
			a.rollbackView(attempt)
			return AuthView{State: AccountOffline, Binding: commit.state.Binding, User: commit.state.User, LastSyncAt: commit.state.LastSyncAt}, errors.Join(err, staged.rollback(ctx))
		}
		return a.failureView(commit.actor, err), errors.Join(err, staged.rollback(ctx))
	}
	if binding != commit.expected {
		err := &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: account result belongs to another binding"}
		return a.failureView(commit.actor, err), errors.Join(err, staged.rollback(ctx))
	}
	return a.setView(commit.actor, AuthView{State: AccountAuthenticated, Binding: binding, User: commit.state.User, LastSyncAt: commit.state.LastSyncAt}), nil
}

func (a *AuthApplication) expectedAuthenticationBinding(ctx context.Context, actor userctx.ActorContext, current ProfileBinding, cloudUserID string) (ProfileBinding, error) {
	actor = actor.Normalize()
	if current.ActorID != actor.ActorID || current.ProfileID != actor.ProfileID {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: resolved binding belongs to another actor"}
	}
	switch current.Status {
	case BindingUnbound:
		return ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: cloudUserID, Status: BindingBound, BindingGeneration: 1}, nil
	case BindingBound:
		if current.CloudUserID == cloudUserID {
			return current, nil
		}
		if current.BindingGeneration == ^uint64(0) {
			return ProfileBinding{}, fmt.Errorf("cloud: binding_generation overflow")
		}
		return ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: cloudUserID, Status: BindingBound, BindingGeneration: current.BindingGeneration + 1}, nil
	case BindingExpired:
		replacer, ok := a.bindings.(rejectedBindingReplacer)
		if !ok {
			return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: expired binding cannot be replaced by this repository"}
		}
		return replacer.PrepareRejectedBinding(ctx, actor, cloudUserID)
	default:
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: authentication requires an unbound or bound profile"}
	}
}

func (a *AuthApplication) confirmPreparedAuthenticationBinding(ctx context.Context, actor userctx.ActorContext, expected ProfileBinding) (ProfileBinding, error) {
	current, err := a.bindings.ResolveBinding(ctx, actor)
	if isCredentialStateRejected(err) && current.Status == BindingExpired {
		replacer, ok := a.bindings.(rejectedBindingReplacer)
		if !ok {
			return ProfileBinding{}, err
		}
		return replacer.ReplaceRejectedBinding(ctx, actor, expected)
	}
	if err != nil {
		return ProfileBinding{}, err
	}
	return a.confirmAuthenticationBinding(ctx, actor, current, expected)
}

func (a *AuthApplication) persistFailure(ctx context.Context, actor userctx.ActorContext, observed Session, cause error) (AuthView, error) {
	terminal := isSessionTerminalError(cause)
	retryable := isRetryableSessionFailure(cause)
	if !terminal && !retryable {
		return a.stableView(actor), cause
	}
	prior, ok, err := a.sessions.Load(ctx, actor)
	if err != nil {
		logAuthFailure("explicit_refresh", observed, string(observed.State), "unchanged", err, "load_failure", "PERSISTENCE_FAILURE")
		return a.failureView(actor, err), err
	}
	if !ok {
		logAuthFailure("explicit_refresh", observed, string(observed.State), "unchanged", cause, "missing_observed", "")
		return a.stableView(actor), cause
	}
	if !sameObservedSession(prior, observed) {
		logAuthFailure("explicit_refresh", observed, string(prior.State), "unchanged", cause, "stale_observed", "")
		return a.stableView(actor), cause
	}
	failed := prior
	viewState := AccountOffline
	if terminal {
		failed = terminateSession(prior, sessionTerminationCode(cause), time.Now().UTC())
		viewState = AccountExpired
	} else {
		failed.State = SessionOffline
	}
	logAuthFailure("explicit_refresh", prior, string(prior.State), string(failed.State), cause, "state_transition", "")
	if err := a.sessions.Save(ctx, actor, failed); err != nil {
		logAuthFailure("explicit_refresh", prior, string(prior.State), string(failed.State), err, "save_failure", "PERSISTENCE_FAILURE")
		return a.failureView(actor, err), err
	}
	scope := observed.Scope
	view := a.cachedView(actor, ProfileBinding{ActorID: actor.Normalize().ActorID, ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, Status: BindingBound, BindingGeneration: scope.BindingGeneration})
	view.State = viewState
	return a.setView(actor, view), cause
}

func (a *AuthApplication) fixtureRequest(actor userctx.ActorContext, operation AccountOperation, payload []byte) AccountFixtureRequest {
	actor = actor.Normalize()
	return AccountFixtureRequest{Binding: ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: BindingBinding}, Operation: operation, FixtureVersion: a.fixtureVersion, Payload: payload}
}

func (a *AuthApplication) beginIdentityAttempt(ctx context.Context, actor userctx.ActorContext, state AccountStatus) (authViewAttempt, error) {
	a.identityMu.Lock()
	defer a.identityMu.Unlock()
	identity, err := a.captureIdentity(ctx, actor)
	if err != nil {
		return authViewAttempt{}, err
	}
	attempt := a.beginTransient(actor, state)
	attempt.identity = identity
	return attempt, nil
}

func (a *AuthApplication) captureIdentity(ctx context.Context, actor userctx.ActorContext) (authIdentitySnapshot, error) {
	actor = actor.Normalize()
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		if isCredentialStateRejected(err) {
			return authIdentitySnapshot{binding: rejectedBinding(actor.Normalize(), binding.BindingGeneration)}, nil
		}
		return authIdentitySnapshot{}, err
	}
	snapshot := authIdentitySnapshot{binding: binding}
	if binding.Status != BindingBound {
		return snapshot, nil
	}
	scope, err := binding.Scope()
	if err != nil {
		return authIdentitySnapshot{}, err
	}
	session, ok, err := a.sessions.Load(ctx, actor)
	if err != nil {
		if isCredentialStateRejected(err) {
			snapshot.binding = rejectedBinding(actor, binding.BindingGeneration)
			snapshot.session = Session{Scope: scope, State: SessionExpired}
			snapshot.sessionExists = true
			return snapshot, nil
		}
		return authIdentitySnapshot{}, err
	}
	if ok && session.Scope != scope {
		return authIdentitySnapshot{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: session does not match the captured binding"}
	}
	snapshot.session = session
	snapshot.sessionExists = ok
	return snapshot, nil
}

func (a *AuthApplication) ensureIdentityAttemptCurrent(ctx context.Context, actor userctx.ActorContext, attempt authViewAttempt) error {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	current, err := a.captureIdentity(recoveryCtx, actor)
	if err != nil {
		a.rollbackViewTo(attempt, AuthView{State: AccountOffline})
		return err
	}
	a.mu.RLock()
	revisionCurrent := a.views[attempt.key].revision == attempt.revision
	a.mu.RUnlock()
	if current != attempt.identity {
		if revisionCurrent {
			a.rollbackViewTo(attempt, safeViewFromIdentity(current))
		}
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: authentication response belongs to a stale identity attempt"}
	}
	if !revisionCurrent {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: authentication response belongs to a stale identity attempt"}
	}
	return nil
}

func localAuthCommitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(context.WithoutCancel(ctx))
}

func authenticationCancellation(ctx context.Context, operationErr error) error {
	if err := ctx.Err(); errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(operationErr, context.Canceled) || errors.Is(operationErr, context.DeadlineExceeded) {
		return operationErr
	}
	return nil
}

func isProfileMismatch(err error) bool {
	var cloudErr *Error
	return errors.As(err, &cloudErr) && cloudErr.Code == ErrorProfileScopeMismatch
}
func isNetworkUnavailable(err error) bool {
	var cloudErr *Error
	return errors.As(err, &cloudErr) && cloudErr.Code == ErrorNetworkUnavailable
}

func isRetryableSessionFailure(err error) bool {
	if isNetworkUnavailable(err) {
		return true
	}
	var cloudErr *Error
	if !errors.As(err, &cloudErr) {
		return false
	}
	switch cloudErr.Code {
	case ErrorCode("RATE_LIMITED"), ErrorCode("SERVICE_UNAVAILABLE"), ErrorCode("UPSTREAM_UNAVAILABLE"):
		return true
	default:
		return cloudErr.Retryable
	}
}
