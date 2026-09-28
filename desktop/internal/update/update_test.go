package update

import (
	"crypto/rand"
	"fmt"
	"testing"

	"aead.dev/minisign"
)

// TestEmbeddedPublicKeyParses guards the hard-coded publicKey constant: it must
// parse and carry the expected key ID, so a copy-paste slip is caught here rather
// than silently failing every signature check in the field.
func TestEmbeddedPublicKeyParses(t *testing.T) {
	var key minisign.PublicKey
	if err := key.UnmarshalText([]byte(publicKey)); err != nil {
		t.Fatalf("embedded public key does not parse: %v", err)
	}
	if got := fmt.Sprintf("%016X", key.ID()); got != "E33B7107401DB17B" {
		t.Fatalf("embedded public key ID = %s, want E33B7107401DB17B", got)
	}
}

// TestVerifyWith exercises the verify path end-to-end with a throwaway key pair:
// a genuine signature passes, tampered data fails, and a wrong-key signature fails.
func TestVerifyWith(t *testing.T) {
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubText, err := pub.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("the quick brown fox")
	sig := minisign.Sign(priv, data)

	if err := verifyWith(string(pubText), data, sig); err != nil {
		t.Fatalf("genuine signature should verify, got: %v", err)
	}
	if err := verifyWith(string(pubText), []byte("tampered payload"), sig); err == nil {
		t.Fatal("tampered data should fail verification")
	}

	otherPub, _, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherText, _ := otherPub.MarshalText()
	if err := verifyWith(string(otherText), data, sig); err == nil {
		t.Fatal("signature under a different key should fail verification")
	}
}

// TestVerifyWithKeyRotationHasNoCompatibilityWindow records the current
// rotation contract with throwaway identities. A client embedding the old key
// cannot accept artifacts signed by the new key, while a client embedding the
// new key cannot accept artifacts signed by the old key. This test deliberately
// does not weaken verification; it makes the missing overlap strategy explicit.
func TestVerifyWithKeyRotationHasNoCompatibilityWindow(t *testing.T) {
	oldPublic, oldPrivate, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	newPublic, newPrivate, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	oldPublicText, err := oldPublic.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	newPublicText, err := newPublic.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte("release candidate")
	oldSignature := minisign.Sign(oldPrivate, payload)
	newSignature := minisign.Sign(newPrivate, payload)

	if err := verifyWith(string(oldPublicText), payload, oldSignature); err != nil {
		t.Fatalf("old client rejected old signature: %v", err)
	}
	if err := verifyWith(string(oldPublicText), payload, newSignature); err == nil {
		t.Fatal("old client accepted signature from rotated key")
	}
	if err := verifyWith(string(newPublicText), payload, newSignature); err != nil {
		t.Fatalf("rotated client rejected new signature: %v", err)
	}
	if err := verifyWith(string(newPublicText), payload, oldSignature); err == nil {
		t.Fatal("rotated client accepted signature from retired key")
	}
}

// TestPlatformKey pins the key format the manifest generator and the updater both
// rely on; if these drift, lookups silently miss.
func TestPlatformKey(t *testing.T) {
	if got := PlatformKey("darwin", "arm64"); got != "darwin-arm64" {
		t.Fatalf("PlatformKey = %q, want darwin-arm64", got)
	}
}

// TestManifestAsset checks the running-platform lookup returns the listed asset
// and reports absence cleanly.
func TestManifestAsset(t *testing.T) {
	want := Asset{URL: "https://example/app", SHA256: "abc", Size: 42}
	m := Manifest{Platforms: map[string]Asset{CurrentPlatform(): want}}
	got, ok := m.Asset()
	if !ok || got != want {
		t.Fatalf("Asset() = %+v, %v; want %+v, true", got, ok, want)
	}
	if _, ok := (Manifest{Platforms: map[string]Asset{}}).Asset(); ok {
		t.Fatal("Asset() should report absence for an empty manifest")
	}
}
