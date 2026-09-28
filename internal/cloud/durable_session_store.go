package cloud

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"kwu/internal/userctx"
)

const (
	durableSessionsSchema  = "reasonix.cloud.sessions"
	durableSessionsVersion = 3
)

type durableSessionsDocument struct {
	Schema      string              `json:"schema"`
	Version     int                 `json:"version"`
	ProfileID   string              `json:"profile_id"`
	Environment EnvironmentIdentity `json:"environment"`
	Sessions    []Session           `json:"sessions"`
}

type DurableCloudSessionStore struct {
	paths       ProfilePathProvider
	environment EnvironmentIdentity
}

func NewDurableCloudSessionStore(paths ProfilePathProvider, environment EnvironmentIdentity) *DurableCloudSessionStore {
	return &DurableCloudSessionStore{paths: paths, environment: environment}
}

func (s *DurableCloudSessionStore) Save(ctx context.Context, session Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := session.Validate(); err != nil {
		return err
	}
	if err := validateDurableSession(session); err != nil {
		return err
	}
	path, err := s.path(session.Scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	sessions, err := readDurableSessions(path, session.Scope.ProfileID, s.environment)
	if err != nil {
		return err
	}
	sessions[session.Scope.key()] = session
	return writeDurableSessions(path, session.Scope.ProfileID, s.environment, sessions)
}

func (s *DurableCloudSessionStore) Load(ctx context.Context, scope BindingScope) (Session, bool, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, false, err
	}
	if err := scope.Validate(); err != nil {
		return Session{}, false, err
	}
	path, err := s.path(scope.ProfileID)
	if err != nil {
		return Session{}, false, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	sessions, err := readDurableSessions(path, scope.ProfileID, s.environment)
	if err != nil {
		return Session{}, false, err
	}
	session, ok := sessions[scope.key()]
	return session, ok, nil
}

func (s *DurableCloudSessionStore) Clear(ctx context.Context, scope BindingScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	path, err := s.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	sessions, err := readDurableSessions(path, scope.ProfileID, s.environment)
	if err != nil {
		return err
	}
	if _, ok := sessions[scope.key()]; !ok {
		return nil
	}
	delete(sessions, scope.key())
	return writeDurableSessions(path, scope.ProfileID, s.environment, sessions)
}

func (s *DurableCloudSessionStore) path(profileID string) (string, error) {
	actor := userctx.ActorContext{ProfileID: profileID}
	return durableCloudFile(s.paths, actor, "sessions.json")
}

func readDurableSessions(path, profileID string, environment EnvironmentIdentity) (map[string]Session, error) {
	if err := environment.Validate(); err != nil {
		return nil, err
	}
	var document durableSessionsDocument
	found, err := durableReadJSON(path, &document)
	if err != nil {
		return nil, classifyCredentialReadError(err)
	}
	if !found {
		return make(map[string]Session), nil
	}
	if err := validateDurableSessionsDocument(document, profileID, environment); err != nil {
		return nil, err
	}
	sessions, err := decodeDurableSessions(document)
	if err != nil {
		return nil, credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), errors.Join(ErrCredentialStateInvalid, err))
	}
	return sessions, nil
}

func validateDurableSessionsDocument(document durableSessionsDocument, profileID string, environment EnvironmentIdentity) error {
	if document.Schema == durableSessionsSchema && (document.Version == 1 || document.Version == 2) {
		return credentialStateError(ErrorCode("LEGACY_CREDENTIAL_STATE"), ErrLegacyCredentialState)
	}
	if document.Schema != durableSessionsSchema || document.Version != durableSessionsVersion {
		return credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), ErrCredentialStateInvalid)
	}
	if document.Environment != environment {
		return credentialStateError(ErrorCode("ENVIRONMENT_MISMATCH"), ErrEnvironmentMismatch)
	}
	if document.ProfileID != profileID {
		return credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), ErrCredentialStateInvalid)
	}
	return nil
}

func decodeDurableSessions(document durableSessionsDocument) (map[string]Session, error) {
	sessions := make(map[string]Session, len(document.Sessions))
	for _, session := range document.Sessions {
		if err := addDurableSession(sessions, document.ProfileID, session); err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

func addDurableSession(sessions map[string]Session, profileID string, session Session) error {
	if err := session.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable session: %w", err)
	}
	if err := validateDurableSession(session); err != nil {
		return err
	}
	if session.Scope.ProfileID != profileID {
		return fmt.Errorf("cloud: durable session belongs to another profile")
	}
	key := session.Scope.key()
	if _, exists := sessions[key]; exists {
		return fmt.Errorf("cloud: duplicate durable session scope")
	}
	sessions[key] = session
	return nil
}

func validateDurableSession(session Session) error {
	switch session.State {
	case SessionOffline:
		if err := session.validateAuthenticated(); err != nil {
			return fmt.Errorf("cloud: invalid durable offline session: %w", err)
		}
		return nil
	case SessionExpired:
		return validateDurableExpiredSession(session)
	default:
		return nil
	}
}

func validateDurableExpiredSession(session Session) error {
	if session.AccessToken != "" || session.RefreshToken != "" || session.DeviceID != "" ||
		!session.AccessTokenExpiresAt.IsZero() || !session.RefreshTokenExpiresAt.IsZero() {
		return fmt.Errorf("cloud: invalid durable expired session")
	}
	if !session.IssuedAt.IsZero() || !session.ExpiresAt.IsZero() {
		return fmt.Errorf("cloud: invalid durable expired session")
	}
	if session.TerminationReason == "" || session.TerminatedAt.IsZero() {
		return fmt.Errorf("cloud: invalid durable expired session")
	}
	return nil
}

func writeDurableSessions(path, profileID string, environment EnvironmentIdentity, sessions map[string]Session) error {
	if err := environment.Validate(); err != nil {
		return err
	}
	keys := make([]string, 0, len(sessions))
	for key := range sessions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([]Session, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, sessions[key])
	}
	document := durableSessionsDocument{Schema: durableSessionsSchema, Version: durableSessionsVersion, ProfileID: profileID, Environment: environment, Sessions: ordered}
	return durableWriteJSON(path, document)
}

func (s *DurableCloudSessionStore) ReplaceRejected(ctx context.Context, session Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := session.Validate(); err != nil {
		return err
	}
	if err := validateDurableSession(session); err != nil {
		return err
	}
	path, err := s.path(session.Scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	_, readErr := readDurableSessions(path, session.Scope.ProfileID, s.environment)
	if !isCredentialStateRejected(readErr) {
		if readErr != nil {
			return readErr
		}
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: session state is no longer rejected"}
	}
	return writeDurableSessions(path, session.Scope.ProfileID, s.environment, map[string]Session{session.Scope.key(): session})
}

var _ CloudSessionStore = (*DurableCloudSessionStore)(nil)
