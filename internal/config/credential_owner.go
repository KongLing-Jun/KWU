package config

import (
	"errors"
	"fmt"
	"strings"

	"kwu/internal/userctx"
)

const (
	CredentialOwnerProfile   = "profile"
	CredentialOwnerWorkspace = "workspace"
	CredentialOwnerTeam      = "team"
	CredentialOwnerRuntime   = "runtime"
)

type CredentialOwner struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func (o CredentialOwner) Normalize() CredentialOwner {
	kind := strings.ToLower(strings.TrimSpace(o.Kind))
	switch kind {
	case CredentialOwnerProfile, CredentialOwnerWorkspace, CredentialOwnerTeam, CredentialOwnerRuntime:
	default:
		kind = CredentialOwnerProfile
	}
	id := strings.TrimSpace(o.ID)
	if kind == CredentialOwnerProfile && id == "" {
		id = userctx.DefaultProfileID
	}
	return CredentialOwner{Kind: kind, ID: id}
}

func (o CredentialOwner) Description() string {
	o = o.Normalize()
	if o.ID == "" {
		return o.Kind
	}
	return o.Kind + " " + o.ID
}

func (o CredentialOwner) isDefaultProfile() bool {
	o = o.Normalize()
	return o.Kind == CredentialOwnerProfile && o.ID == userctx.DefaultProfileID
}

func CredentialOwnerForProfile(profileID string) CredentialOwner {
	return CredentialOwner{Kind: CredentialOwnerProfile, ID: profileID}.Normalize()
}

func CredentialOwnerForPaths(paths ProfilePathResolver) CredentialOwner {
	paths = paths.withWorkspaceRoot("")
	return CredentialOwnerForProfile(paths.ProfileID())
}

func CredentialKey(owner CredentialOwner, envName string) string {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return ""
	}
	owner = owner.Normalize()
	if owner.Kind == "" || owner.ID == "" {
		return ""
	}
	return owner.Kind + ":" + owner.ID + ":" + envName
}

var errKeyringOnlyNotFound = errors.New("keyring credential not found")

var (
	keyringOnlySetLookup    = keyringOnlySet
	keyringOnlyGetLookup    = keyringOnlyGet
	keyringOnlyDeleteLookup = keyringOnlyDelete
)

// SetKeyringOnlyCredential writes directly to the OS keyring without a file fallback.
func SetKeyringOnlyCredential(owner CredentialOwner, key, value string) error {
	owner = owner.Normalize()
	key = strings.TrimSpace(key)
	if CredentialKey(owner, key) == "" {
		return errors.New("keyring-only credential owner and key are required")
	}
	if value == "" {
		return errors.New("keyring-only credential value is required")
	}
	if err := keyringOnlySetLookup(owner, key, value); err != nil {
		return fmt.Errorf("store keyring-only credential: %w", err)
	}
	return nil
}

func KeyringOnlyCredential(owner CredentialOwner, key string) (string, bool, error) {
	owner = owner.Normalize()
	key = strings.TrimSpace(key)
	if CredentialKey(owner, key) == "" {
		return "", false, errors.New("keyring-only credential owner and key are required")
	}
	value, err := keyringOnlyGetLookup(owner, key)
	if errors.Is(err, errKeyringOnlyNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read keyring-only credential: %w", err)
	}
	return value, value != "", nil
}

func DeleteKeyringOnlyCredential(owner CredentialOwner, key string) error {
	owner = owner.Normalize()
	key = strings.TrimSpace(key)
	if CredentialKey(owner, key) == "" {
		return errors.New("keyring-only credential owner and key are required")
	}
	if err := keyringOnlyDeleteLookup(owner, key); err != nil && !errors.Is(err, errKeyringOnlyNotFound) {
		return fmt.Errorf("delete keyring-only credential: %w", err)
	}
	return nil
}
