package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"

	"kwu/internal/config"
	"kwu/internal/userctx"
)

type EnvironmentClass string

const (
	EnvironmentProduction  EnvironmentClass = "production"
	EnvironmentDevelopment EnvironmentClass = "development"
	EnvironmentFixture     EnvironmentClass = "fixture"
)

// EnvironmentIdentity binds local authentication material to one build class
// and one normalized cloud endpoint.
type EnvironmentIdentity struct {
	EnvironmentID string `json:"environment_id"`
	CloudBaseURL  string `json:"cloud_base_url"`
}

func NewEnvironmentIdentity(class EnvironmentClass, rawBaseURL string) (EnvironmentIdentity, error) {
	switch class {
	case EnvironmentProduction, EnvironmentDevelopment, EnvironmentFixture:
	default:
		return EnvironmentIdentity{}, fmt.Errorf("cloud: unsupported environment class %q", class)
	}
	normalized, err := NormalizeEnvironmentBaseURL(rawBaseURL)
	if err != nil {
		return EnvironmentIdentity{}, err
	}
	sum := sha256.Sum256([]byte(string(class) + "\x00" + normalized))
	return EnvironmentIdentity{EnvironmentID: hex.EncodeToString(sum[:]), CloudBaseURL: normalized}, nil
}

func (e EnvironmentIdentity) Validate() error {
	if len(e.EnvironmentID) != sha256.Size*2 {
		return fmt.Errorf("cloud: environment_id must be a complete SHA-256 digest")
	}
	for _, value := range e.EnvironmentID {
		if (value < '0' || value > '9') && (value < 'a' || value > 'f') {
			return fmt.Errorf("cloud: environment_id must be lowercase hexadecimal")
		}
	}
	normalized, err := NormalizeEnvironmentBaseURL(e.CloudBaseURL)
	if err != nil {
		return err
	}
	if normalized != e.CloudBaseURL {
		return fmt.Errorf("cloud: environment cloud base URL is not normalized")
	}
	return nil
}

func NormalizeEnvironmentBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := parseEnvironmentBaseURL(raw)
	if err != nil {
		return "", err
	}
	scheme, err := normalizedEnvironmentURLScheme(parsed.Scheme)
	if err != nil {
		return "", err
	}
	if err := validateEnvironmentURLDecorations(raw, parsed); err != nil {
		return "", err
	}
	return (&url.URL{
		Scheme: scheme,
		Host:   normalizedEnvironmentURLHost(parsed, scheme),
		Path:   normalizedEnvironmentURLPath(parsed.Path),
	}).String(), nil
}

func parseEnvironmentBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Opaque != "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("cloud: cloud base URL must be an absolute HTTP or HTTPS URL")
	}
	return parsed, nil
}

func normalizedEnvironmentURLScheme(raw string) (string, error) {
	scheme := strings.ToLower(raw)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("cloud: cloud base URL must use HTTP or HTTPS")
	}
	return scheme, nil
}

func validateEnvironmentURLDecorations(raw string, parsed *url.URL) error {
	if parsed.User != nil {
		return fmt.Errorf("cloud: cloud base URL must not contain userinfo")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(raw, "?") {
		return fmt.Errorf("cloud: cloud base URL must not contain a query")
	}
	if parsed.Fragment != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("cloud: cloud base URL must not contain a fragment")
	}
	return nil
}

func normalizedEnvironmentURLHost(parsed *url.URL, scheme string) string {
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		return net.JoinHostPort(hostname, port)
	}
	if strings.Contains(hostname, ":") {
		return "[" + hostname + "]"
	}
	return hostname
}

func normalizedEnvironmentURLPath(raw string) string {
	if raw == "" {
		return "/"
	}
	cleaned := pathpkg.Clean("/" + strings.TrimLeft(raw, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

// ProfilePathProvider supplies the existing 04 profile-aware path resolver.
// Production composition uses config.NewProfilePathResolver; tests can inject
// a resolver whose roots point at a temporary directory.
type ProfilePathProvider func(userctx.ActorContext) config.ProfilePathResolver

var (
	ErrLegacyCredentialState  = errors.New("cloud: legacy credential state")
	ErrEnvironmentMismatch    = errors.New("cloud: credential environment mismatch")
	ErrCredentialStateInvalid = errors.New("cloud: durable credential state invalid")
)

type CredentialStateError struct {
	Code  ErrorCode
	cause error
}

func (e *CredentialStateError) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Code) + ": local authentication state was rejected"
}

func (e *CredentialStateError) Unwrap() error { return e.cause }

func credentialStateError(code ErrorCode, cause error) error {
	return &CredentialStateError{Code: code, cause: cause}
}

func isCredentialStateRejected(err error) bool {
	var rejected *CredentialStateError
	return errors.As(err, &rejected)
}

type durableContentError struct {
	message string
	cause   error
}

func (e *durableContentError) Error() string { return e.message }
func (e *durableContentError) Unwrap() error { return e.cause }

func classifyCredentialReadError(err error) error {
	var content *durableContentError
	if errors.As(err, &content) {
		return credentialStateError(ErrorCode("DURABLE_CREDENTIAL_STATE_INVALID"), errors.Join(ErrCredentialStateInvalid, err))
	}
	return err
}

func DefaultProfilePathProvider(actor userctx.ActorContext) config.ProfilePathResolver {
	return config.NewProfilePathResolver(actor)
}

var durablePathLocks sync.Map

func durableCloudFile(provider ProfilePathProvider, actor userctx.ActorContext, name string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("cloud: profile path provider is not configured")
	}
	actor = actor.Normalize()
	resolver := provider(actor)
	if resolver.ProfileID() != actor.ProfileID {
		return "", fmt.Errorf("cloud: profile path resolver returned another profile")
	}
	root := resolver.MemoryUserDir()
	if root == "" {
		return "", fmt.Errorf("cloud: profile state path is unavailable")
	}
	return filepath.Join(root, "cloud", "v1", name), nil
}

func durablePathLock(path string) *sync.Mutex {
	lock, _ := durablePathLocks.LoadOrStore(filepath.Clean(path), &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func durableReadJSON[T any](path string, destination *T) (bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cloud: open durable state: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return false, &durableContentError{message: "cloud: decode durable state: " + err.Error(), cause: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return false, &durableContentError{message: "cloud: durable state has trailing content"}
	}
	return true, nil
}

func durableWriteJSON(path string, value any) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("cloud: create durable state directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("cloud: protect durable state directory: %w", err)
	}
	temporaryPath, err := writeDurableTemporary(directory, value)
	if err != nil {
		return err
	}
	defer os.Remove(temporaryPath)
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("cloud: replace durable state: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("cloud: protect durable state file: %w", err)
	}
	return syncDurableDirectory(directory)
}

func writeDurableTemporary(directory string, value any) (string, error) {
	temporary, err := os.CreateTemp(directory, ".state-*.tmp")
	if err != nil {
		return "", fmt.Errorf("cloud: create durable state temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	remove := func(err error) (string, error) {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return "", err
	}
	if err := temporary.Chmod(0o600); err != nil {
		return remove(fmt.Errorf("cloud: protect durable state temporary file: %w", err))
	}
	encoder := json.NewEncoder(temporary)
	if err := encoder.Encode(value); err != nil {
		return remove(fmt.Errorf("cloud: encode durable state: %w", err))
	}
	if err := temporary.Sync(); err != nil {
		return remove(fmt.Errorf("cloud: sync durable state temporary file: %w", err))
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return "", fmt.Errorf("cloud: close durable state temporary file: %w", err)
	}
	return temporaryPath, nil
}

func syncDurableDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("cloud: open durable state directory for sync: %w", err)
	}
	syncErr := handle.Sync()
	closeErr := handle.Close()
	if syncErr != nil && !ignoreDurableDirectorySyncError(syncErr) {
		return fmt.Errorf("cloud: sync durable state directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("cloud: close durable state directory: %w", closeErr)
	}
	return nil
}
