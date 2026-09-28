package pythonruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestDefaultManagedRoot_Override(t *testing.T) {
	t.Setenv("KWU_PYTHON_MANAGED_ROOT", filepath.Join(t.TempDir(), "managed"))
	if got, want := DefaultManagedRoot(), filepath.Clean(os.Getenv("KWU_PYTHON_MANAGED_ROOT")); got != want {
		t.Fatalf("DefaultManagedRoot() = %q, want %q", got, want)
	}
}

func TestManager_Exact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*Manifest)
		omit    string
		wantErr string
	}{
		{name: "valid manifest and layout"},
		{name: "schema mismatch", mutate: func(m *Manifest) { m.Schema++ }, wantErr: "schema"},
		{name: "version mismatch", mutate: func(m *Manifest) { m.Version = "3.12.9" }, wantErr: "version"},
		{name: "architecture mismatch", mutate: func(m *Manifest) { m.Arch = "wrong" }, wantErr: "architecture"},
		{name: "invalid source hash", mutate: func(m *Manifest) { m.SourceSHA256 = "xyz" }, wantErr: "source_sha256"},
		{name: "source hash mismatch", mutate: func(m *Manifest) { m.SourceSHA256 = strings.Repeat("b", 64) }, wantErr: "pinned artifact"},
		{name: "invalid source size", mutate: func(m *Manifest) { m.SourceSize = 0 }, wantErr: "source_size"},
		{name: "source size mismatch", mutate: func(m *Manifest) { m.SourceSize++ }, wantErr: "pinned artifact"},
		{name: "payload hash mismatch", mutate: func(m *Manifest) { m.PayloadSHA256 = strings.Repeat("b", 64) }, wantErr: "pinned payload"},
		{name: "payload receipt tamper", mutate: func(m *Manifest) { m.Files[0].Size++ }, wantErr: "payload receipt SHA-256"},
		{name: "missing executable", omit: "executable", wantErr: "executable"},
		{name: "missing license", omit: "license", wantErr: "license"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			installed := writeRuntime(t, root, PinnedVersion, tt.mutate, tt.omit)
			got, err := fixtureManager(t, root, installed.Root).Exact()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Exact() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Exact() error = %v", err)
			}
			if !filepath.IsAbs(got.Executable) || got.Manifest.Version != PinnedVersion {
				t.Fatalf("Exact() = %+v, want absolute pinned runtime", got)
			}
		})
	}
}

func TestManager_ExactRejectsPayloadChanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		change  func(*testing.T, ManagedRuntime)
		wantErr string
	}{
		{
			name: "payload content tamper",
			change: func(t *testing.T, installed ManagedRuntime) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(installed.Root, "LICENSE.txt"), []byte("fixture licensf"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "SHA-256 is",
		},
		{
			name: "residual sitecustomize",
			change: func(t *testing.T, installed ManagedRuntime) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(installed.Root, "sitecustomize.py"), []byte("raise RuntimeError('executed')"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "unexpected file \"sitecustomize.py\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			installed := writeRuntime(t, root, PinnedVersion, nil, "")
			manager := fixtureManager(t, root, installed.Root)
			tt.change(t, installed)
			if _, err := manager.Exact(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Exact() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestManager_ExactDoesNotTrustOlderRuntime(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRuntime(t, root, "3.11.8", nil, "")
	if _, err := NewManager(root).Exact(); err == nil || !strings.Contains(err.Error(), PinnedVersion) {
		t.Fatalf("Exact() error = %v, want missing pinned runtime", err)
	}
}

func TestPinnedArtifactsMatchWindowsPackagingLock(t *testing.T) {
	type lockedArtifact struct {
		Bytes  int64  `json:"bytes"`
		SHA256 string `json:"sha256"`
	}
	var lock struct {
		Schema    int                       `json:"schema"`
		Version   string                    `json:"version"`
		Artifacts map[string]lockedArtifact `json:"artifacts"`
	}
	path := filepath.Join("..", "..", "desktop", "build", "windows", "python-runtime.lock.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	if lock.Schema != ManifestSchema || lock.Version != PinnedVersion {
		t.Fatalf("packaging lock schema/version = %d/%q, want %d/%q", lock.Schema, lock.Version, ManifestSchema, PinnedVersion)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		wantSHA, wantSize, ok := pinnedArtifact(arch)
		got, exists := lock.Artifacts[arch]
		if !ok || !exists || got.SHA256 != wantSHA || got.Bytes != wantSize {
			t.Fatalf("packaging lock %s = %+v, want sha256=%s bytes=%d", arch, got, wantSHA, wantSize)
		}
	}
}

func TestValidatePinnedManagedAliasReceipt(t *testing.T) {
	const arch = "amd64"
	root := t.TempDir()
	selected := filepath.Join(root, "python.exe")
	alias := filepath.Join(root, "python3.exe")
	for _, path := range []string{selected, alias} {
		if err := os.WriteFile(path, []byte("independent managed fixture"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	selectedInfo, err := os.Stat(selected)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(alias)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(selectedInfo, aliasInfo) {
		t.Fatal("fixture aliases unexpectedly share the same file identity")
	}

	files, payloadSHA := fixturePayloadReceipt(t, root)
	sourceSHA, sourceSize, ok := pinnedArtifact(arch)
	if !ok {
		t.Fatalf("missing pinned artifact for %s", arch)
	}
	manifest := Manifest{
		Schema:        ManifestSchema,
		Version:       PinnedVersion,
		Arch:          arch,
		SourceSHA256:  sourceSHA,
		SourceSize:    sourceSize,
		PayloadSHA256: payloadSHA,
		Files:         files,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ManifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := validatePinnedManagedAliasReceipt(selected, alias, arch, payloadSHA); err != nil {
		t.Fatalf("trusted distinct managed alias rejected: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "python3.exe")
	if err := os.WriteFile(outside, []byte("independent managed fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validatePinnedManagedAliasReceipt(selected, outside, arch, payloadSHA); err == nil || !strings.Contains(err.Error(), "pair") {
		t.Fatalf("outside same-content copy error = %v, want pinned pair rejection", err)
	}
	if err := os.WriteFile(alias, []byte("tampered managed fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validatePinnedManagedAliasReceipt(selected, alias, arch, payloadSHA); err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("tampered managed alias error = %v, want receipt rejection", err)
	}
}

func writeRuntime(t *testing.T, root, version string, mutate func(*Manifest), omit string) ManagedRuntime {
	t.Helper()
	dir := filepath.Join(root, version, runtime.GOARCH)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sha, size, ok := pinnedArtifact(runtime.GOARCH)
	if version != PinnedVersion || !ok {
		sha, size = strings.Repeat("a", 64), 1024
	}
	if omit != "executable" {
		if err := os.WriteFile(filepath.Join(dir, pythonExecutableName()), []byte("fixture"), 0o755); err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS == "windows" {
			if err := os.WriteFile(filepath.Join(dir, "python3.exe"), []byte("fixture"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	if omit != "license" {
		if err := os.WriteFile(filepath.Join(dir, "LICENSE.txt"), []byte("fixture license"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, payloadSHA := fixturePayloadReceipt(t, dir)
	manifest := Manifest{
		Schema:        ManifestSchema,
		Version:       version,
		Arch:          runtime.GOARCH,
		SourceSHA256:  sha,
		SourceSize:    size,
		PayloadSHA256: payloadSHA,
		Files:         files,
	}
	if mutate != nil {
		mutate(&manifest)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return ManagedRuntime{Root: dir, Executable: filepath.Join(dir, pythonExecutableName()), Manifest: manifest}
}

func fixtureManager(t *testing.T, root, runtimeRoot string) *Manager {
	t.Helper()
	_, payloadSHA := fixturePayloadReceipt(t, runtimeRoot)
	manager := NewManager(root)
	manager.pinnedPayloadSHA256 = func(string) (string, bool) { return payloadSHA, true }
	return manager
}

func fixturePayloadReceipt(t *testing.T, root string) ([]PayloadFile, string) {
	t.Helper()
	var files []PayloadFile
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == root || entry.IsDir() || entry.Name() == ManifestName {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		digest, err := hashFile(func(path string) (io.ReadCloser, error) { return os.Open(path) }, path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, PayloadFile{Path: filepath.ToSlash(relative), Size: info.Size(), SHA256: digest})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	h := sha256.New()
	for _, file := range files {
		writePayloadRecord(h, file)
	}
	return files, hex.EncodeToString(h.Sum(nil))
}
