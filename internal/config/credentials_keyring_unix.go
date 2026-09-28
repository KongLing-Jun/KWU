//go:build (dragonfly && cgo) || (freebsd && cgo) || linux || netbsd || openbsd

package config

import (
	"errors"
	"strings"

	ss "github.com/zalando/go-keyring/secret_service"
)

var errConnectorKeyringOnlyUnsupported = errors.New("keyring-only connector credentials are unavailable on this platform")

func legacyKeyringCredentialValue(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}
	return secretServiceCredentialValue(key)
}

func keyringCredentialValue(owner CredentialOwner, key string) (string, bool) {
	key = strings.TrimSpace(key)
	name := CredentialKey(owner, key)
	if name == "" {
		return "", false
	}
	if value, ok := secretServiceCredentialValue(name); ok {
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
	return secretServiceCredentialDelete(key)
}

func keyringCredentialDelete(owner CredentialOwner, key string) error {
	key = strings.TrimSpace(key)
	name := CredentialKey(owner, key)
	if name == "" {
		return nil
	}
	if err := secretServiceCredentialDelete(name); err != nil {
		return err
	}
	if owner.isDefaultProfile() && !canonicalUserDataOnlyEnabled() {
		return legacyKeyringCredentialDelete(key)
	}
	return nil
}

func secretServiceCredentialValue(key string) (string, bool) {
	svc, err := ss.NewSecretService()
	if err != nil {
		return "", false
	}
	defer svc.Conn.Close()

	collection := svc.GetLoginCollection()
	search := map[string]string{
		"username": key,
		"service":  credentialsKeyringService,
	}
	if err := svc.Unlock(collection.Path()); err != nil {
		return "", false
	}
	results, err := svc.SearchItems(collection, search)
	if err != nil || len(results) == 0 {
		return "", false
	}

	session, err := svc.OpenSession()
	if err != nil {
		return "", false
	}
	defer svc.Close(session)

	if err := svc.Unlock(results[0]); err != nil {
		return "", false
	}
	secret, err := svc.GetSecret(results[0], session.Path())
	if err != nil || secret == nil || len(secret.Value) == 0 {
		return "", false
	}
	return string(secret.Value), true
}

func secretServiceCredentialDelete(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	svc, err := ss.NewSecretService()
	if err != nil {
		return err
	}
	defer svc.Conn.Close()

	collection := svc.GetLoginCollection()
	search := map[string]string{
		"username": key,
		"service":  credentialsKeyringService,
	}
	if err := svc.Unlock(collection.Path()); err != nil {
		return err
	}
	results, err := svc.SearchItems(collection, search)
	if err != nil {
		return err
	}
	for _, item := range results {
		if err := svc.Delete(item); err != nil {
			return err
		}
	}
	return nil
}

func keyringOnlySet(CredentialOwner, string, string) error {
	return errConnectorKeyringOnlyUnsupported
}

func keyringOnlyGet(CredentialOwner, string) (string, error) {
	return "", errConnectorKeyringOnlyUnsupported
}

func keyringOnlyDelete(CredentialOwner, string) error {
	return errConnectorKeyringOnlyUnsupported
}
