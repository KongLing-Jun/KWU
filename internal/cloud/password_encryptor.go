package cloud

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrPasswordEncryptionNotReady = errors.New("cloud: password encryption is not ready until CLOUD-03 closes")

// PasswordEncryptor transforms a plaintext password only for a registration
// submission. Implementations must not retain either the plaintext or result.
type PasswordEncryptor interface {
	Encrypt(string) (EncryptedPassword, error)
}

// PasswordCrypto describes the frozen L1 test output, not a production key
// distribution or compatibility policy.
type PasswordCrypto struct {
	Algorithm  string `json:"algorithm"`
	KeySize    int    `json:"key_size"`
	Mode       string `json:"mode"`
	Padding    string `json:"padding"`
	Output     string `json:"output"`
	KeyVersion string `json:"key_version"`
}

// EncryptedPassword is the only password value suitable for a normalized
// registration fixture request.
type EncryptedPassword struct {
	EncryptedPassword string         `json:"encrypted_password"`
	Crypto            PasswordCrypto `json:"password_crypto"`
}

const TestPasswordCryptoKeyVersion = "test-register-pwd-v1"

var (
	testPasswordEncryptionKey = []byte("reasonix-testkey")
	testPasswordEncryptionIV  = []byte("reasonix-test-iv")
)

// TestPasswordEncryptor uses the explicitly versioned L1 test key and IV. It
// exists solely for fixture-driven tests and must never be used for production
// registration or published as a production configuration.
type TestPasswordEncryptor struct{}

func NewTestPasswordEncryptor() TestPasswordEncryptor {
	return TestPasswordEncryptor{}
}

func (TestPasswordEncryptor) Encrypt(plaintext string) (EncryptedPassword, error) {
	block, err := aes.NewCipher(testPasswordEncryptionKey[:])
	if err != nil {
		return EncryptedPassword{}, err
	}
	padded := pkcs7Pad([]byte(plaintext), block.BlockSize())
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, testPasswordEncryptionIV[:]).CryptBlocks(ciphertext, padded)
	return EncryptedPassword{
		EncryptedPassword: base64.StdEncoding.EncodeToString(ciphertext),
		Crypto: PasswordCrypto{
			Algorithm:  "AES",
			KeySize:    128,
			Mode:       "CBC",
			Padding:    "PKCS7Padding",
			Output:     "Base64",
			KeyVersion: TestPasswordCryptoKeyVersion,
		},
	}, nil
}

func pkcs7Pad(plaintext []byte, blockSize int) []byte {
	padding := blockSize - len(plaintext)%blockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for index := len(plaintext); index < len(padded); index++ {
		padded[index] = byte(padding)
	}
	return padded
}

// ProductionPasswordEncryptor rejects encryption until CLOUD-03 supplies the
// production key, IV, key version, and compatibility rules.
type ProductionPasswordEncryptor struct{}

func NewPasswordEncryptor() PasswordEncryptor {
	return ProductionPasswordEncryptor{}
}

func (ProductionPasswordEncryptor) Encrypt(string) (EncryptedPassword, error) {
	return EncryptedPassword{}, ErrPasswordEncryptionNotReady
}

var _ PasswordEncryptor = TestPasswordEncryptor{}
var _ PasswordEncryptor = ProductionPasswordEncryptor{}

const (
	registrationPasswordAlgorithm          = "AES"
	registrationPasswordKeySizeBits        = 128
	registrationPasswordMode               = "CBC"
	registrationPasswordPadding            = "PKCS7Padding"
	registrationPasswordOutput             = "Base64"
	registrationPasswordCiphertextEncoding = "base64(iv || ciphertext)"
	registrationPasswordKeyBytes           = 16
	maxRegistrationEncryptedPasswordBytes  = 4096
)

// RegistrationPasswordCrypto is the short-lived key configuration returned
// by GET /api/v1/auth/register-password-crypto. It must not be persisted.
type RegistrationPasswordCrypto struct {
	Algorithm          string    `json:"algorithm"`
	KeySizeBits        int       `json:"key_size_bits"`
	Mode               string    `json:"mode"`
	Padding            string    `json:"padding"`
	Output             string    `json:"output"`
	CiphertextEncoding string    `json:"ciphertext_encoding"`
	KeyVersion         string    `json:"key_version"`
	KeyBase64          string    `json:"key_base64"`
	ExpiresAt          time.Time `json:"expires_at"`
}

func (c RegistrationPasswordCrypto) Validate(now time.Time) error {
	if now.IsZero() {
		return fmt.Errorf("cloud: password crypto validation time is required")
	}
	expected := RegistrationPasswordMetadata{
		Algorithm: registrationPasswordAlgorithm, KeySize: registrationPasswordKeySizeBits,
		Mode: registrationPasswordMode, Padding: registrationPasswordPadding,
		Output: registrationPasswordOutput, KeyVersion: c.KeyVersion,
	}
	if c.Metadata() != expected || c.CiphertextEncoding != registrationPasswordCiphertextEncoding {
		return fmt.Errorf("cloud: unsupported registration password crypto contract")
	}
	if err := requireTrimmed("key_version", c.KeyVersion); err != nil {
		return err
	}
	if err := requireTrimmed("key_base64", c.KeyBase64); err != nil {
		return err
	}
	key, err := base64.StdEncoding.DecodeString(c.KeyBase64)
	if err != nil {
		return fmt.Errorf("cloud: key_base64 is invalid")
	}
	if len(key) != registrationPasswordKeyBytes {
		return fmt.Errorf("cloud: registration password key must be 16 bytes")
	}
	if c.ExpiresAt.IsZero() || !now.UTC().Before(c.ExpiresAt.UTC()) {
		return fmt.Errorf("cloud: registration password crypto is expired")
	}
	return nil
}

func (c RegistrationPasswordCrypto) Metadata() RegistrationPasswordMetadata {
	return RegistrationPasswordMetadata{
		Algorithm: c.Algorithm, KeySize: c.KeySizeBits, Mode: c.Mode,
		Padding: c.Padding, Output: c.Output, KeyVersion: c.KeyVersion,
	}
}

// RegistrationPasswordMetadata is the public, key-free crypto description
// sent back with POST /api/v1/auth/register.
type RegistrationPasswordMetadata struct {
	Algorithm  string `json:"algorithm"`
	KeySize    int    `json:"key_size"`
	Mode       string `json:"mode"`
	Padding    string `json:"padding"`
	Output     string `json:"output"`
	KeyVersion string `json:"key_version"`
}

func (m RegistrationPasswordMetadata) Validate() error {
	if m.Algorithm != registrationPasswordAlgorithm || m.KeySize != registrationPasswordKeySizeBits ||
		m.Mode != registrationPasswordMode || m.Padding != registrationPasswordPadding || m.Output != registrationPasswordOutput {
		return fmt.Errorf("cloud: unsupported registration password metadata")
	}
	return requireTrimmed("key_version", m.KeyVersion)
}

type SendRegistrationSMSRequest struct {
	Phone           string
	CaptchaTicket   string
	CaptchaProvider string
}

func (r SendRegistrationSMSRequest) Validate() error {
	if err := requireTrimmed("phone", r.Phone); err != nil {
		return err
	}
	return (CaptchaProof{Ticket: r.CaptchaTicket, Provider: r.CaptchaProvider}).Validate()
}

type RegistrationSMSResult struct {
	Phone            string `json:"phone"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

func (r RegistrationSMSResult) Validate() error {
	if err := requireTrimmed("phone", r.Phone); err != nil {
		return err
	}
	if r.ExpiresInSeconds <= 0 {
		return fmt.Errorf("cloud: registration SMS expiry must be positive")
	}
	return nil
}

type RegistrationRequest struct {
	Phone             string
	EncryptedPassword string
	PasswordCrypto    RegistrationPasswordMetadata
	SMSCode           string
}

func (r RegistrationRequest) Validate() error {
	if err := requireTrimmed("phone", r.Phone); err != nil {
		return err
	}
	if err := requireTrimmed("encrypted_password", r.EncryptedPassword); err != nil {
		return err
	}
	if len(r.EncryptedPassword) > maxRegistrationEncryptedPasswordBytes {
		return fmt.Errorf("cloud: encrypted_password exceeds 4096 bytes")
	}
	if !isSixASCIIDigits(r.SMSCode) {
		return fmt.Errorf("cloud: sms_code must be 6 ASCII digits")
	}
	return r.PasswordCrypto.Validate()
}

func isSixASCIIDigits(value string) bool {
	if len(value) != 6 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

type RegistrationSignupReward struct {
	Granted  bool   `json:"granted"`
	Amount   string `json:"amount"`
	LedgerID string `json:"ledger_id"`
}

func (r RegistrationSignupReward) Validate() error {
	if err := requireTrimmed("signup_reward.amount", r.Amount); err != nil {
		return err
	}
	if r.Granted {
		return requireTrimmed("signup_reward.ledger_id", r.LedgerID)
	}
	if r.LedgerID != "" && strings.TrimSpace(r.LedgerID) != r.LedgerID {
		return fmt.Errorf("cloud: signup_reward.ledger_id must be trimmed")
	}
	return nil
}

// RegistrationThirdPartySync is the registration response summary. Unlike
// ThirdPartySync it has no local BindingScope and is never persisted as one.
type RegistrationThirdPartySync struct {
	SyncID    string               `json:"sync_id"`
	Provider  string               `json:"provider"`
	Status    ThirdPartySyncStatus `json:"status"`
	Version   uint64               `json:"version"`
	UpdatedAt time.Time            `json:"updated_at"`
}

func (s RegistrationThirdPartySync) Validate() error {
	if err := requireTrimmed("third_party_sync.sync_id", s.SyncID); err != nil {
		return err
	}
	if err := requireTrimmed("third_party_sync.provider", s.Provider); err != nil {
		return err
	}
	switch s.Status {
	case ThirdPartySyncPending, ThirdPartySyncRunning, ThirdPartySyncSucceeded,
		ThirdPartySyncFailedRetryable, ThirdPartySyncFailedManualRequired, ThirdPartySyncSkipped:
	default:
		return fmt.Errorf("cloud: invalid registration third-party sync status %q", s.Status)
	}
	if s.Version == 0 {
		return fmt.Errorf("cloud: registration third-party sync version must be positive")
	}
	if s.UpdatedAt.IsZero() {
		return fmt.Errorf("cloud: registration third-party sync updated_at is required")
	}
	return nil
}

type RegistrationResult struct {
	Authentication AuthenticationResult
	SignupReward   RegistrationSignupReward
	ThirdPartySync RegistrationThirdPartySync
	Balance        BalanceSnapshot
}

func (r RegistrationResult) Validate() error {
	if err := r.Authentication.User.Validate(); err != nil {
		return err
	}
	if err := r.Authentication.Session.Validate(); err != nil {
		return err
	}
	if r.Authentication.ServerTime.IsZero() {
		return fmt.Errorf("cloud: registration server_time is required")
	}
	if err := r.SignupReward.Validate(); err != nil {
		return err
	}
	// The cloud omits third_party_sync when no synchronization provider is
	// configured. When present, keep validating the complete summary.
	if r.ThirdPartySync != (RegistrationThirdPartySync{}) {
		if err := r.ThirdPartySync.Validate(); err != nil {
			return err
		}
	}
	return r.Balance.Validate()
}

type CloudRegistrationClient interface {
	CaptchaConfig(context.Context) (CaptchaConfig, error)
	PasswordCrypto(context.Context) (RegistrationPasswordCrypto, error)
	SendRegistrationSMS(context.Context, SendRegistrationSMSRequest) (RegistrationSMSResult, error)
	Register(context.Context, RegistrationRequest) (RegistrationResult, error)
}

type SendPasswordResetSMSRequest struct {
	Phone           string
	CaptchaTicket   string
	CaptchaProvider string
}

func (r SendPasswordResetSMSRequest) Validate() error {
	if err := requireTrimmed("phone", r.Phone); err != nil {
		return err
	}
	return (CaptchaProof{Ticket: r.CaptchaTicket, Provider: r.CaptchaProvider}).Validate()
}

type PasswordResetSMSResult struct {
	Phone            string `json:"phone"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

func (r PasswordResetSMSResult) Validate() error {
	if err := requireTrimmed("phone", r.Phone); err != nil {
		return err
	}
	if r.ExpiresInSeconds <= 0 {
		return fmt.Errorf("cloud: password reset SMS expiry must be positive")
	}
	return nil
}

type PasswordResetRequest struct {
	Phone       string
	NewPassword string
	SMSCode     string
}

func (r PasswordResetRequest) Validate() error {
	if err := requireTrimmed("phone", r.Phone); err != nil {
		return err
	}
	if !validRegistrationPassword(r.NewPassword) {
		return &Error{Code: ErrorCode("PASSWORD_INVALID"), Message: "password must contain 8 to 20 UTF-8 characters"}
	}
	if !isSixASCIIDigits(r.SMSCode) {
		return fmt.Errorf("cloud: sms_code must be 6 ASCII digits")
	}
	return nil
}

type PasswordResetResult struct {
	Reset bool `json:"reset"`
}

func (r PasswordResetResult) Validate() error {
	if !r.Reset {
		return fmt.Errorf("cloud: password reset response must confirm reset")
	}
	return nil
}

type CloudPasswordResetClient interface {
	SendPasswordResetSMS(context.Context, SendPasswordResetSMSRequest) (PasswordResetSMSResult, error)
	ResetPassword(context.Context, PasswordResetRequest) (PasswordResetResult, error)
}

type EncryptedRegistrationPassword struct {
	EncryptedPassword string
	Crypto            RegistrationPasswordMetadata
}

// RegistrationPasswordEncryptor applies the short-lived server configuration
// to one registration password. It does not retain plaintext, ciphertext, or
// key material between calls.
type RegistrationPasswordEncryptor struct {
	random io.Reader
	now    func() time.Time
}

func NewRegistrationPasswordEncryptor(random io.Reader, now func() time.Time) *RegistrationPasswordEncryptor {
	if random == nil {
		random = rand.Reader
	}
	if now == nil {
		now = time.Now
	}
	return &RegistrationPasswordEncryptor{random: random, now: now}
}

func (e *RegistrationPasswordEncryptor) Encrypt(plaintext string, config RegistrationPasswordCrypto) (EncryptedRegistrationPassword, error) {
	if e == nil || e.random == nil || e.now == nil {
		return EncryptedRegistrationPassword{}, fmt.Errorf("cloud: registration password encryptor is not configured")
	}
	if !validRegistrationPassword(plaintext) {
		return EncryptedRegistrationPassword{}, &Error{Code: ErrorCode("PASSWORD_INVALID"), Message: "password must contain 8 to 20 UTF-8 characters"}
	}
	if err := config.Validate(e.now().UTC()); err != nil {
		return EncryptedRegistrationPassword{}, err
	}
	key, err := base64.StdEncoding.DecodeString(config.KeyBase64)
	if err != nil {
		return EncryptedRegistrationPassword{}, fmt.Errorf("cloud: decode registration password key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptedRegistrationPassword{}, fmt.Errorf("cloud: initialize registration password cipher: %w", err)
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(e.random, iv); err != nil {
		return EncryptedRegistrationPassword{}, fmt.Errorf("cloud: generate registration password IV: %w", err)
	}
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	payload := make([]byte, aes.BlockSize+len(padded))
	copy(payload, iv)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(payload[aes.BlockSize:], padded)
	return EncryptedRegistrationPassword{
		EncryptedPassword: base64.StdEncoding.EncodeToString(payload),
		Crypto:            config.Metadata(),
	}, nil
}

func validRegistrationPassword(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	count := utf8.RuneCountInString(value)
	return count >= 8 && count <= 20
}
