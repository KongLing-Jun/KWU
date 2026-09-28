package cloud

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestTestPasswordEncryptorUsesVersionedTestAESContract(t *testing.T) {
	plaintext := "reasonix temporary password"
	encryptor := NewTestPasswordEncryptor()
	first, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	second, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}
	if first.EncryptedPassword != second.EncryptedPassword {
		t.Fatal("versioned test encryptor must be deterministic for its fixed test key and IV")
	}
	if strings.Contains(first.EncryptedPassword, plaintext) {
		t.Fatal("encrypted password must not contain plaintext")
	}
	if first.Crypto != (PasswordCrypto{
		Algorithm:  "AES",
		KeySize:    128,
		Mode:       "CBC",
		Padding:    "PKCS7Padding",
		Output:     "Base64",
		KeyVersion: TestPasswordCryptoKeyVersion,
	}) {
		t.Fatalf("password crypto metadata = %#v", first.Crypto)
	}

	encoded, err := base64.StdEncoding.DecodeString(first.EncryptedPassword)
	if err != nil {
		t.Fatalf("ciphertext is not base64: %v", err)
	}
	block, err := aes.NewCipher(testPasswordEncryptionKey[:])
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	if len(encoded) == 0 || len(encoded)%block.BlockSize() != 0 {
		t.Fatalf("ciphertext length = %d, want positive AES block multiple", len(encoded))
	}
	plain := make([]byte, len(encoded))
	cipher.NewCBCDecrypter(block, testPasswordEncryptionIV[:]).CryptBlocks(plain, encoded)
	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > block.BlockSize() {
		t.Fatalf("PKCS7 padding = %d", pad)
	}
	for _, value := range plain[len(plain)-pad:] {
		if int(value) != pad {
			t.Fatalf("invalid PKCS7 padding byte %d, want %d", value, pad)
		}
	}
	if got := string(plain[:len(plain)-pad]); got != plaintext {
		t.Fatalf("decrypted plaintext = %q, want %q", got, plaintext)
	}
}

func TestDefaultPasswordEncryptorIsUnavailableUntilCloud03Closes(t *testing.T) {
	_, err := NewPasswordEncryptor().Encrypt("temporary password")
	if !errors.Is(err, ErrPasswordEncryptionNotReady) {
		t.Fatalf("default Encrypt error = %v, want ErrPasswordEncryptionNotReady", err)
	}
}

func TestRegistrationPasswordEncryptorUsesRandomIVAndDocumentedEnvelope(t *testing.T) {
	now := time.Date(2026, 7, 14, 4, 0, 0, 0, time.UTC)
	config := validRegistrationPasswordCrypto(now)
	random := append(bytes.Repeat([]byte{0x11}, aes.BlockSize), bytes.Repeat([]byte{0x22}, aes.BlockSize)...)
	encryptor := NewRegistrationPasswordEncryptor(bytes.NewReader(random), func() time.Time { return now })
	plaintext := "ReasonixDev#2026"

	first, err := encryptor.Encrypt(plaintext, config)
	if err != nil {
		t.Fatalf("first Encrypt: %v", err)
	}
	second, err := encryptor.Encrypt(plaintext, config)
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}
	if first.EncryptedPassword == second.EncryptedPassword {
		t.Fatal("registration ciphertext reused an IV")
	}
	if got := decryptRegistrationPassword(t, config, first.EncryptedPassword); got != plaintext {
		t.Fatalf("decrypted plaintext = %q, want %q", got, plaintext)
	}
	if first.Crypto != config.Metadata() {
		t.Fatalf("password crypto metadata = %#v, want %#v", first.Crypto, config.Metadata())
	}
	encoded, err := base64.StdEncoding.DecodeString(first.EncryptedPassword)
	if err != nil {
		t.Fatalf("encrypted password base64: %v", err)
	}
	if !bytes.Equal(encoded[:aes.BlockSize], bytes.Repeat([]byte{0x11}, aes.BlockSize)) {
		t.Fatalf("encoded IV = %x", encoded[:aes.BlockSize])
	}
}

func TestRegistrationPasswordEncryptorValidatesUTF8CharacterCount(t *testing.T) {
	now := time.Date(2026, 7, 14, 4, 0, 0, 0, time.UTC)
	config := validRegistrationPasswordCrypto(now)
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "seven characters", password: strings.Repeat("界", 7), wantErr: true},
		{name: "eight characters", password: strings.Repeat("界", 8)},
		{name: "twenty characters", password: strings.Repeat("界", 20)},
		{name: "twenty one characters", password: strings.Repeat("界", 21), wantErr: true},
		{name: "invalid utf8", password: string([]byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa, 0xf9, 0xf8}), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encryptor := NewRegistrationPasswordEncryptor(bytes.NewReader(bytes.Repeat([]byte{0x33}, aes.BlockSize)), func() time.Time { return now })
			_, err := encryptor.Encrypt(test.password, config)
			if test.wantErr && err == nil {
				t.Fatal("expected password validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
		})
	}
}

func TestRegistrationPasswordEncryptorRejectsExpiredConfigAndRandomFailure(t *testing.T) {
	now := time.Date(2026, 7, 14, 4, 0, 0, 0, time.UTC)
	config := validRegistrationPasswordCrypto(now)
	config.ExpiresAt = now
	encryptor := NewRegistrationPasswordEncryptor(bytes.NewReader(bytes.Repeat([]byte{0x44}, aes.BlockSize)), func() time.Time { return now })
	if _, err := encryptor.Encrypt("ReasonixDev#2026", config); err == nil {
		t.Fatal("expected expired crypto error")
	}
	config = validRegistrationPasswordCrypto(now)
	encryptor = NewRegistrationPasswordEncryptor(failingRegistrationRandom{}, func() time.Time { return now })
	if _, err := encryptor.Encrypt("ReasonixDev#2026", config); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("random error = %v, want io.ErrUnexpectedEOF", err)
	}
}

type failingRegistrationRandom struct{}

func (failingRegistrationRandom) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func decryptRegistrationPassword(t *testing.T, config RegistrationPasswordCrypto, encoded string) string {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(config.KeyBase64)
	if err != nil {
		t.Fatalf("key base64: %v", err)
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("encrypted password base64: %v", err)
	}
	if len(payload) <= aes.BlockSize || (len(payload)-aes.BlockSize)%aes.BlockSize != 0 {
		t.Fatalf("encrypted payload length = %d", len(payload))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	plaintext := make([]byte, len(payload)-aes.BlockSize)
	cipher.NewCBCDecrypter(block, payload[:aes.BlockSize]).CryptBlocks(plaintext, payload[aes.BlockSize:])
	padding := int(plaintext[len(plaintext)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(plaintext) {
		t.Fatalf("PKCS7 padding = %d", padding)
	}
	for _, value := range plaintext[len(plaintext)-padding:] {
		if int(value) != padding {
			t.Fatalf("invalid PKCS7 padding byte %d", value)
		}
	}
	return string(plaintext[:len(plaintext)-padding])
}
