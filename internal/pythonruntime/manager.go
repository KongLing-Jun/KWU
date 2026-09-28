package pythonruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	PinnedVersion            = "3.12.10"
	ManifestSchema           = 1
	ManifestName             = ".kwu-runtime.json"
	pinnedAMD64SHA256        = "4acbed6dd1c744b0376e3b1cf57ce906f9dc9e95e68824584c8099a63025a3c3"
	pinnedAMD64Size          = int64(11133606)
	pinnedARM64SHA256        = "3065efc3d382d1cda66757ac71ade11904fa6e350f5a97eb74811acd71ba5532"
	pinnedARM64Size          = int64(10413299)
	pinnedAMD64PayloadSHA256 = "37d3f0b2de372d2cebf88012cbed9eca64c9a59e3fe15c75787805067e253629"
	pinnedARM64PayloadSHA256 = "ad17c601d2ca8359e07a6e80790dfd00c4b9b913bad667fe8d0558d81fc99515"
)

// PayloadFile identifies one file installed from the pinned runtime archive.
type PayloadFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest describes the archive from which a managed runtime was installed.
type Manifest struct {
	Schema        int           `json:"schema"`
	Version       string        `json:"version"`
	Arch          string        `json:"arch"`
	SourceSHA256  string        `json:"source_sha256"`
	SourceSize    int64         `json:"source_size"`
	PayloadSHA256 string        `json:"payload_sha256"`
	Files         []PayloadFile `json:"files"`
}

// ManagedRuntime is a validated, installed Python runtime.
type ManagedRuntime struct {
	Root       string
	Executable string
	Manifest   Manifest
}

type managerFS struct {
	readFile func(string) ([]byte, error)
	stat     func(string) (fs.FileInfo, error)
	open     func(string) (io.ReadCloser, error)
	walkDir  func(string, fs.WalkDirFunc) error
}

// Manager locates managed runtimes without mutating their installation root.
// Its immutable configuration makes it safe for concurrent use.
type Manager struct {
	root                string
	arch                string
	fs                  managerFS
	pinnedPayloadSHA256 func(string) (string, bool)
}

// NewManager creates a manager for root. An empty root uses the platform default.
func NewManager(root string) *Manager {
	if root == "" {
		root = DefaultManagedRoot()
	}
	if absolute, err := filepath.Abs(root); err == nil {
		root = absolute
	} else {
		root = filepath.Clean(root)
	}
	return &Manager{
		root: root,
		arch: runtime.GOARCH,
		fs: managerFS{
			readFile: os.ReadFile,
			stat:     os.Stat,
			open: func(path string) (io.ReadCloser, error) {
				return os.Open(path)
			},
			walkDir: filepath.WalkDir,
		},
		pinnedPayloadSHA256: pinnedPayloadArtifact,
	}
}

// DefaultManagedRoot returns the versioned runtime root next to the application.
// KWU_PYTHON_MANAGED_ROOT overrides that location for managed deployments and tests.
func DefaultManagedRoot() string {
	if root := strings.TrimSpace(os.Getenv("KWU_PYTHON_MANAGED_ROOT")); root != "" {
		return filepath.Clean(root)
	}
	executable, err := os.Executable()
	if err != nil {
		return filepath.Join("runtimes", "python")
	}
	return filepath.Join(filepath.Dir(executable), "runtimes", "python")
}

// Exact returns the pinned runtime or an error explaining why it is unusable.
func (m *Manager) Exact() (ManagedRuntime, error) {
	return m.validate(PinnedVersion)
}

func (m *Manager) validate(version string) (ManagedRuntime, error) {
	root := filepath.Join(m.root, version, m.arch)
	manifestPath := filepath.Join(root, ManifestName)
	data, err := m.fs.readFile(manifestPath)
	if err != nil {
		return ManagedRuntime{}, fmt.Errorf("read managed Python manifest %q: %w", manifestPath, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ManagedRuntime{}, fmt.Errorf("decode managed Python manifest %q: %w", manifestPath, err)
	}
	if err := validateManifest(manifest, version, m.arch); err != nil {
		return ManagedRuntime{}, fmt.Errorf("validate managed Python manifest %q: %w", manifestPath, err)
	}
	if err := m.validatePayload(root, manifest); err != nil {
		return ManagedRuntime{}, fmt.Errorf("validate managed Python payload: %w", err)
	}
	executable := filepath.Join(root, pythonExecutableName())
	if err := requireRegularFile(m.fs.stat, executable); err != nil {
		return ManagedRuntime{}, fmt.Errorf("validate managed Python executable: %w", err)
	}
	if runtime.GOOS == "windows" {
		if err := requireRegularFile(m.fs.stat, filepath.Join(root, "python3.exe")); err != nil {
			return ManagedRuntime{}, fmt.Errorf("validate managed Python 3 alias: %w", err)
		}
	}
	if err := requireRegularFile(m.fs.stat, filepath.Join(root, "LICENSE.txt")); err != nil {
		return ManagedRuntime{}, fmt.Errorf("validate managed Python license: %w", err)
	}
	absExecutable, err := filepath.Abs(executable)
	if err != nil {
		return ManagedRuntime{}, fmt.Errorf("make managed Python path absolute: %w", err)
	}
	return ManagedRuntime{Root: root, Executable: absExecutable, Manifest: manifest}, nil
}

func validateManifest(manifest Manifest, version, arch string) error {
	if manifest.Schema != ManifestSchema {
		return fmt.Errorf("schema is %d, want %d", manifest.Schema, ManifestSchema)
	}
	if manifest.Version != version {
		return fmt.Errorf("version is %q, want %q", manifest.Version, version)
	}
	if manifest.Arch != arch {
		return fmt.Errorf("architecture is %q, want %q", manifest.Arch, arch)
	}
	sha, err := hex.DecodeString(manifest.SourceSHA256)
	if err != nil || len(sha) != 32 {
		return errors.New("source_sha256 must be a 64-character hexadecimal SHA-256")
	}
	if manifest.SourceSize <= 0 {
		return errors.New("source_size must be positive")
	}
	if version != PinnedVersion {
		return nil
	}
	return validatePinnedManifest(manifest, arch)
}

func validatePinnedManifest(manifest Manifest, arch string) error {
	expectedSHA, expectedSize, ok := pinnedArtifact(arch)
	if !ok {
		return fmt.Errorf("pinned Python %s does not support architecture %q", PinnedVersion, arch)
	}
	if !strings.EqualFold(manifest.SourceSHA256, expectedSHA) {
		return fmt.Errorf("source_sha256 is %q, want pinned artifact %q", manifest.SourceSHA256, expectedSHA)
	}
	if manifest.SourceSize != expectedSize {
		return fmt.Errorf("source_size is %d, want pinned artifact %d", manifest.SourceSize, expectedSize)
	}
	return nil
}

func pinnedArtifact(arch string) (string, int64, bool) {
	switch arch {
	case "amd64":
		return pinnedAMD64SHA256, pinnedAMD64Size, true
	case "arm64":
		return pinnedARM64SHA256, pinnedARM64Size, true
	default:
		return "", 0, false
	}
}

func pinnedPayloadArtifact(arch string) (string, bool) {
	switch arch {
	case "amd64":
		return pinnedAMD64PayloadSHA256, true
	case "arm64":
		return pinnedARM64PayloadSHA256, true
	default:
		return "", false
	}
}

func (m *Manager) validatePayload(root string, manifest Manifest) error {
	expectedReceiptSHA, ok := m.pinnedPayloadSHA256(m.arch)
	if !ok {
		return fmt.Errorf("pinned Python %s does not support architecture %q", PinnedVersion, m.arch)
	}
	if err := validatePayloadReceipt(manifest, expectedReceiptSHA); err != nil {
		return err
	}

	validation := newPayloadValidation(root, manifest.Files, m.fs.open)
	if err := m.fs.walkDir(root, validation.visit); err != nil {
		return err
	}
	return validation.validateComplete()
}

type payloadValidation struct {
	root        string
	expected    map[string]PayloadFile
	directories map[string]struct{}
	seen        map[string]struct{}
	open        func(string) (io.ReadCloser, error)
}

func newPayloadValidation(root string, files []PayloadFile, open func(string) (io.ReadCloser, error)) *payloadValidation {
	expected := make(map[string]PayloadFile, len(files))
	directories := make(map[string]struct{})
	for _, file := range files {
		native := filepath.Clean(filepath.FromSlash(file.Path))
		expected[normalizePayloadPath(native)] = file
		for dir := filepath.Dir(native); dir != "."; dir = filepath.Dir(dir) {
			directories[normalizePayloadPath(dir)] = struct{}{}
		}
	}
	return &payloadValidation{
		root:        root,
		expected:    expected,
		directories: directories,
		seen:        make(map[string]struct{}, len(expected)),
		open:        open,
	}
}

func (v *payloadValidation) visit(path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if path == v.root {
		return nil
	}
	relative, err := filepath.Rel(v.root, path)
	if err != nil {
		return err
	}
	if entry.IsDir() {
		return v.validateDirectory(relative)
	}
	if relative == ManifestName {
		return validateManifestEntry(entry)
	}
	return v.validateFile(path, relative, entry)
}

func (v *payloadValidation) validateDirectory(relative string) error {
	if _, ok := v.directories[normalizePayloadPath(relative)]; !ok {
		return fmt.Errorf("unexpected directory %q", filepath.ToSlash(relative))
	}
	return nil
}

func validateManifestEntry(entry fs.DirEntry) error {
	if entry.Type().IsRegular() {
		return nil
	}
	return fmt.Errorf("manifest %q is not a regular file", ManifestName)
}

func (v *payloadValidation) validateFile(path, relative string, entry fs.DirEntry) error {
	key := normalizePayloadPath(relative)
	file, ok := v.expected[key]
	if !ok {
		return fmt.Errorf("unexpected file %q", filepath.ToSlash(relative))
	}
	if err := validatePayloadFileEntry(entry, file); err != nil {
		return err
	}
	actualSHA, err := hashFile(v.open, path)
	if err != nil {
		return fmt.Errorf("hash payload file %q: %w", file.Path, err)
	}
	if actualSHA != file.SHA256 {
		return fmt.Errorf("payload file %q SHA-256 is %s, want %s", file.Path, actualSHA, file.SHA256)
	}
	v.seen[key] = struct{}{}
	return nil
}

func validatePayloadFileEntry(entry fs.DirEntry, file PayloadFile) error {
	if !entry.Type().IsRegular() {
		return fmt.Errorf("payload file %q is not a regular file", file.Path)
	}
	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("stat payload file %q: %w", file.Path, err)
	}
	if info.Size() != file.Size {
		return fmt.Errorf("payload file %q size is %d, want %d", file.Path, info.Size(), file.Size)
	}
	return nil
}

func (v *payloadValidation) validateComplete() error {
	if len(v.seen) == len(v.expected) {
		return nil
	}
	for key, file := range v.expected {
		if _, ok := v.seen[key]; !ok {
			return fmt.Errorf("payload file %q is missing", file.Path)
		}
	}
	return nil
}

func validatePayloadReceipt(manifest Manifest, expectedReceiptSHA string) error {
	if len(manifest.Files) == 0 {
		return errors.New("payload receipt has no files")
	}
	if manifest.PayloadSHA256 != expectedReceiptSHA {
		return fmt.Errorf("payload_sha256 is %q, want pinned payload %q", manifest.PayloadSHA256, expectedReceiptSHA)
	}
	h := sha256.New()
	previousPath := ""
	seenFolded := make(map[string]struct{}, len(manifest.Files))
	for _, file := range manifest.Files {
		if err := validatePayloadFile(file, previousPath, seenFolded); err != nil {
			return err
		}
		writePayloadRecord(h, file)
		previousPath = file.Path
	}
	actualReceiptSHA := hex.EncodeToString(h.Sum(nil))
	if actualReceiptSHA != manifest.PayloadSHA256 {
		return fmt.Errorf("payload receipt SHA-256 is %s, want %s", actualReceiptSHA, manifest.PayloadSHA256)
	}
	return nil
}

func validatePayloadFile(file PayloadFile, previousPath string, seenFolded map[string]struct{}) error {
	if !isCanonicalPayloadPath(file.Path) {
		return fmt.Errorf("payload path %q is not a canonical relative slash path", file.Path)
	}
	if previousPath != "" && file.Path <= previousPath {
		return errors.New("payload files must be sorted by path with no duplicates")
	}
	folded := strings.ToLower(file.Path)
	if _, ok := seenFolded[folded]; ok {
		return fmt.Errorf("payload path %q collides case-insensitively", file.Path)
	}
	seenFolded[folded] = struct{}{}
	return validatePayloadFileIdentity(file)
}

func isCanonicalPayloadPath(path string) bool {
	if path == "" || path == "." || path == ManifestName {
		return false
	}
	if strings.ContainsAny(path, "\\\r\n") || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != ".." && !strings.HasPrefix(clean, "../")
}

func validatePayloadFileIdentity(file PayloadFile) error {
	if file.Size < 0 {
		return fmt.Errorf("payload file %q has negative size", file.Path)
	}
	sha, err := hex.DecodeString(file.SHA256)
	if err != nil || len(sha) != sha256.Size || file.SHA256 != strings.ToLower(file.SHA256) {
		return fmt.Errorf("payload file %q SHA-256 must be 64 lowercase hexadecimal characters", file.Path)
	}
	return nil
}

func writePayloadRecord(h hash.Hash, file PayloadFile) {
	_, _ = io.WriteString(h, file.SHA256)
	_, _ = io.WriteString(h, "  ")
	_, _ = io.WriteString(h, strconv.FormatInt(file.Size, 10))
	_, _ = io.WriteString(h, "  ")
	_, _ = io.WriteString(h, file.Path)
	_, _ = io.WriteString(h, "\n")
}

func hashFile(open func(string) (io.ReadCloser, error), path string) (string, error) {
	file, err := open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func normalizePayloadPath(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

func requireRegularFile(stat func(string) (fs.FileInfo, error), path string) error {
	info, err := stat(path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	return nil
}

func validatePinnedManagedAlias(selected, resolved string) error {
	if !isPinnedManagedAliasPair(selected, resolved) {
		return errors.New("paths are not the pinned managed python.exe/python3.exe pair")
	}
	expectedReceipt, ok := pinnedPayloadArtifact(runtime.GOARCH)
	if !ok {
		return fmt.Errorf("unsupported managed alias architecture %q", runtime.GOARCH)
	}
	return validatePinnedManagedAliasReceipt(selected, resolved, runtime.GOARCH, expectedReceipt)
}

func validatePinnedManagedAliasReceipt(selected, resolved, arch, expectedReceipt string) error {
	if !isPinnedManagedAliasPathPair(selected, resolved) {
		return errors.New("paths are not the pinned managed python.exe/python3.exe pair")
	}
	root := filepath.Dir(selected)
	data, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		return fmt.Errorf("read managed alias manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decode managed alias manifest: %w", err)
	}
	if err := validateManifest(manifest, PinnedVersion, arch); err != nil {
		return err
	}
	if err := validatePayloadReceipt(manifest, expectedReceipt); err != nil {
		return err
	}
	for _, name := range []string{"python.exe", "python3.exe"} {
		if err := validateManifestPayloadFile(root, name, manifest.Files); err != nil {
			return err
		}
	}
	return nil
}

func isPinnedManagedAliasPair(selected, resolved string) bool {
	return runtime.GOOS == "windows" && isPinnedManagedAliasPathPair(selected, resolved)
}

func isPinnedManagedAliasPathPair(selected, resolved string) bool {
	return strings.EqualFold(filepath.Base(selected), "python.exe") &&
		strings.EqualFold(filepath.Base(resolved), "python3.exe") &&
		sameExecutablePath(filepath.Dir(selected), filepath.Dir(resolved))
}

func validateManifestPayloadFile(root, name string, files []PayloadFile) error {
	for _, file := range files {
		if file.Path != name {
			continue
		}
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() != file.Size {
			return fmt.Errorf("managed alias file %q does not match its receipt", name)
		}
		actual, err := hashFile(func(path string) (io.ReadCloser, error) { return os.Open(path) }, path)
		if err != nil || actual != file.SHA256 {
			return fmt.Errorf("managed alias file %q SHA-256 does not match its receipt", name)
		}
		return nil
	}
	return fmt.Errorf("managed alias receipt is missing %q", name)
}

func pythonExecutableName() string {
	if runtime.GOOS == "windows" {
		return "python.exe"
	}
	return "python"
}
