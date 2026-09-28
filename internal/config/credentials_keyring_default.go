//go:build !linux && !netbsd && !openbsd && !(dragonfly && cgo) && !(freebsd && cgo)

package config

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

func legacyKeyringCredentialValue(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	value, err := keyring.Get(credentialsKeyringService, key)
	return value, err == nil && value != ""
}

func keyringCredentialValue(owner CredentialOwner, key string) (string, bool) {
	key = strings.TrimSpace(key)
	name := CredentialKey(owner, key)
	if name == "" {
		return "", false
	}
	value, err := keyring.Get(credentialsKeyringService, name)
	if err == nil && value != "" {
		return value, true
	}
	if owner.isDefaultProfile() {
		return legacyKeyringCredentialValueFallback(key)
	}
	return "", false
}

func legacyKeyringCredentialDelete(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if err := keyring.Delete(credentialsKeyringService, key); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

func keyringCredentialDelete(owner CredentialOwner, key string) error {
	key = strings.TrimSpace(key)
	name := CredentialKey(owner, key)
	if name == "" {
		return nil
	}
	if err := keyring.Delete(credentialsKeyringService, name); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	if owner.isDefaultProfile() && !canonicalUserDataOnlyEnabled() {
		return legacyKeyringCredentialDelete(key)
	}
	return nil
}

func keyringOnlySet(owner CredentialOwner, key, value string) error {
	return keyring.Set(credentialsKeyringService, CredentialKey(owner, key), value)
}

func keyringOnlyGet(owner CredentialOwner, key string) (string, error) {
	value, err := keyring.Get(credentialsKeyringService, CredentialKey(owner, key))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errKeyringOnlyNotFound
	}
	return value, err
}

func keyringOnlyDelete(owner CredentialOwner, key string) error {
	err := keyring.Delete(credentialsKeyringService, CredentialKey(owner, key))
	if errors.Is(err, keyring.ErrNotFound) {
		return errKeyringOnlyNotFound
	}
	return err
}
