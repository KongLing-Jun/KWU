package connector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/fileutil"
)

type Status string

const (
	StatusNotInstalled        Status = "not_installed"
	StatusInstalling          Status = "installing"
	StatusAuthorizationNeeded Status = "authorization_required"
	StatusAuthorizing         Status = "authorizing"
	StatusConnected           Status = "connected"
	StatusRepairNeeded        Status = "repair_required"
	StatusFailed              Status = "failed"
)

type SafeError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type OwnerScope struct {
	ProfileID         string `json:"profile_id"`
	CloudUserID       string `json:"cloud_user_id"`
	BindingGeneration uint64 `json:"binding_generation"`
	StateRoot         string `json:"-"`
}

func (s OwnerScope) Validate() error {
	if strings.TrimSpace(s.ProfileID) == "" || strings.TrimSpace(s.CloudUserID) == "" || s.BindingGeneration == 0 || strings.TrimSpace(s.StateRoot) == "" {
		return errors.New("connector state: invalid owner scope")
	}
	return nil
}

type Snapshot struct {
	ConnectorID string     `json:"connector_id"`
	Status      Status     `json:"status"`
	Version     string     `json:"version,omitempty"`
	SHA256      string     `json:"sha256,omitempty"`
	Error       *SafeError `json:"error,omitempty"`
	PackagePath string     `json:"-"`
}

type installState struct {
	ConnectorID string     `json:"connector_id"`
	Status      Status     `json:"status"`
	Error       *SafeError `json:"error,omitempty"`
}

type ownerState struct {
	ProfileID         string     `json:"profile_id"`
	CloudUserID       string     `json:"cloud_user_id"`
	BindingGeneration uint64     `json:"binding_generation"`
	ConnectorID       string     `json:"connector_id"`
	Status            Status     `json:"status"`
	Version           string     `json:"version"`
	SHA256            string     `json:"sha256"`
	Error             *SafeError `json:"error,omitempty"`
}

type StateStore struct{ SharedRoot string }

func (s StateStore) WriteInstall(connectorID string, status Status, safeErr *SafeError) error {
	if status != StatusInstalling && status != StatusFailed {
		return errors.New("connector state: invalid shared install status")
	}
	state := installState{ConnectorID: connectorID, Status: status, Error: cloneSafeError(safeErr)}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFileStrict(filepath.Join(s.SharedRoot, connectorID, "install-state.json"), raw, 0o600)
}

func (s StateStore) ClearInstall(connectorID string) error {
	err := os.Remove(filepath.Join(s.SharedRoot, connectorID, "install-state.json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s StateStore) LoadInstall(connectorID string) (installState, bool, error) {
	raw, err := os.ReadFile(filepath.Join(s.SharedRoot, connectorID, "install-state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return installState{}, false, nil
		}
		return installState{}, false, err
	}
	var state installState
	if err := decodeStrict(raw, &state); err != nil || state.ConnectorID != connectorID || (state.Status != StatusInstalling && state.Status != StatusFailed) {
		return installState{}, false, errors.New("connector state: invalid shared state")
	}
	return state, true, nil
}

func (s StateStore) WriteOwner(scope OwnerScope, connectorID string, active ActiveVersion, status Status) error {
	return s.WriteOwnerWithError(scope, connectorID, active, status, nil)
}

func (s StateStore) WriteOwnerWithError(scope OwnerScope, connectorID string, active ActiveVersion, status Status, safeErr *SafeError) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if status != StatusAuthorizationNeeded && status != StatusAuthorizing && status != StatusConnected && status != StatusRepairNeeded {
		return errors.New("connector state: invalid owner status")
	}
	state := ownerState{
		ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration,
		ConnectorID: connectorID, Status: status, Version: active.Version, SHA256: active.SHA256, Error: cloneSafeError(safeErr),
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFileStrict(ownerStatePath(scope, connectorID), raw, 0o600)
}

func (s StateStore) LoadOwner(scope OwnerScope, connectorID string) (ownerState, bool, error) {
	if err := scope.Validate(); err != nil {
		return ownerState{}, false, err
	}
	raw, err := os.ReadFile(ownerStatePath(scope, connectorID))
	if err != nil {
		if os.IsNotExist(err) {
			return ownerState{}, false, nil
		}
		return ownerState{}, false, err
	}
	var state ownerState
	if err := decodeStrict(raw, &state); err != nil {
		return ownerState{}, false, err
	}
	if state.ConnectorID != connectorID {
		return ownerState{}, false, errors.New("connector state: owner scope mismatch")
	}
	// A new Cloud binding generation must not reuse the previous binding's
	// authorization state. Treat it like no state for the current owner so one
	// stale connector cannot prevent the rest of the catalog from loading.
	if state.ProfileID != scope.ProfileID || state.CloudUserID != scope.CloudUserID || state.BindingGeneration != scope.BindingGeneration {
		return ownerState{}, false, nil
	}
	return state, true, nil
}

func ownerStatePath(scope OwnerScope, connectorID string) string {
	digest := sha256.Sum256([]byte(scope.CloudUserID))
	owner := hex.EncodeToString(digest[:])
	return filepath.Join(scope.StateRoot, "users", owner, connectorID+".json")
}

func cloneSafeError(value *SafeError) *SafeError {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func safeInstallError(err error) *SafeError {
	code := "CONNECTOR_PACKAGE_INVALID"
	message := "连接器包无效，请重试。"
	retryable := false
	text := err.Error()
	if strings.Contains(text, "too large") || strings.Contains(text, "size limit") {
		code = "CONNECTOR_PACKAGE_TOO_LARGE"
		message = "连接器包超过大小限制。"
	} else if strings.Contains(text, "request failed") || strings.Contains(text, "HTTP status") || strings.Contains(text, "create temporary") || strings.Contains(text, "write active") {
		code = "CONNECTOR_STORAGE_UNAVAILABLE"
		message = "连接器包暂时不可用，请重试。"
		retryable = true
	}
	return &SafeError{Code: code, Message: message, Retryable: retryable}
}

func (s Snapshot) ValidatePublic() error {
	if s.Error != nil && (strings.Contains(s.Error.Message, ":\\") || strings.Contains(s.Error.Message, "/Users/") || strings.Contains(s.Error.Message, "token")) {
		return fmt.Errorf("connector state: unsafe public error")
	}
	return nil
}
