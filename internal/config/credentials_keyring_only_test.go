package config

import (
	"errors"
	"testing"
)

func TestKeyringOnlyCredentialUsesExactOwnerQualifiedSlot(t *testing.T) {
	oldSet, oldGet, oldDelete := keyringOnlySetLookup, keyringOnlyGetLookup, keyringOnlyDeleteLookup
	t.Cleanup(func() {
		keyringOnlySetLookup, keyringOnlyGetLookup, keyringOnlyDeleteLookup = oldSet, oldGet, oldDelete
	})
	owner := CredentialOwner{Kind: CredentialOwnerProfile, ID: "profile-cloud"}
	key := "connector:tencent-docs:oauth:v1"
	value := "versioned-json"
	var storedOwner CredentialOwner
	var storedKey, storedValue string
	keyringOnlySetLookup = func(gotOwner CredentialOwner, gotKey, gotValue string) error {
		storedOwner, storedKey, storedValue = gotOwner, gotKey, gotValue
		return nil
	}
	keyringOnlyGetLookup = func(gotOwner CredentialOwner, gotKey string) (string, error) {
		if gotOwner != storedOwner || gotKey != storedKey {
			t.Fatalf("unexpected keyring read owner=%+v key=%q", gotOwner, gotKey)
		}
		return storedValue, nil
	}
	keyringOnlyDeleteLookup = func(gotOwner CredentialOwner, gotKey string) error {
		if gotOwner != storedOwner || gotKey != storedKey {
			t.Fatalf("unexpected keyring delete owner=%+v key=%q", gotOwner, gotKey)
		}
		return nil
	}
	if err := SetKeyringOnlyCredential(owner, key, value); err != nil {
		t.Fatal(err)
	}
	if got, found, err := KeyringOnlyCredential(owner, key); err != nil || !found || got != value {
		t.Fatalf("read = %q, %v, %v", got, found, err)
	}
	if err := DeleteKeyringOnlyCredential(owner, key); err != nil {
		t.Fatal(err)
	}
}

func TestKeyringOnlyCredentialDoesNotFallback(t *testing.T) {
	oldGet := keyringOnlyGetLookup
	t.Cleanup(func() { keyringOnlyGetLookup = oldGet })
	keyringOnlyGetLookup = func(CredentialOwner, string) (string, error) {
		return "", errKeyringOnlyNotFound
	}
	got, found, err := KeyringOnlyCredential(CredentialOwnerForProfile("work"), "connector:tencent-docs:oauth:v1")
	if err != nil || found || got != "" {
		t.Fatalf("missing keyring credential = %q, %v, %v", got, found, err)
	}
	keyringOnlyGetLookup = func(CredentialOwner, string) (string, error) {
		return "", errors.New("keyring unavailable")
	}
	if _, _, err := KeyringOnlyCredential(CredentialOwnerForProfile("work"), "connector:tencent-docs:oauth:v1"); err == nil {
		t.Fatal("keyring error must not fall back")
	}
}
