package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aead.dev/minisign"

	"kwu/desktop/internal/update"
)

// TestSignFiles signs a file with a throwaway key pair (injected via env, exactly
// as CI passes the real key) and verifies the produced .minisig validates under the
// matching public key.
func TestSignFiles(t *testing.T) {
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := minisign.EncryptKey("pw", priv)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MINISIGN_PRIVATE_KEY", string(enc))
	t.Setenv("MINISIGN_PRIVATE_KEY_FILE", "")
	t.Setenv("MINISIGN_PASSWORD", "pw")
	useVerificationKey(t, pub)

	dir := t.TempDir()
	artifact := filepath.Join(dir, "Clouvia-linux-amd64.tar.gz")
	payload := []byte("pretend this is a release tarball")
	if err := os.WriteFile(artifact, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := signFiles([]string{artifact}); err != nil {
		t.Fatalf("signFiles: %v", err)
	}
	sig, err := os.ReadFile(artifact + ".minisig")
	if err != nil {
		t.Fatalf("read signature: %v", err)
	}
	if !minisign.Verify(pub, payload, sig) {
		t.Fatal("produced signature does not verify under the signing key")
	}
}

func TestSignFilesReadsEncryptedKeyFile(t *testing.T) {
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := minisign.EncryptKey("file-password", priv)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "kwu.key")
	if err := os.WriteFile(keyPath, enc, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(dir, "Clouvia-windows-amd64-installer.exe")
	payload := []byte("windows installer")
	if err := os.WriteFile(artifact, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MINISIGN_PRIVATE_KEY", "")
	t.Setenv("MINISIGN_PRIVATE_KEY_FILE", keyPath)
	t.Setenv("MINISIGN_PASSWORD", "file-password")
	useVerificationKey(t, pub)

	if err := signFiles([]string{artifact}); err != nil {
		t.Fatalf("signFiles with key file: %v", err)
	}
	sig, err := os.ReadFile(artifact + ".minisig")
	if err != nil {
		t.Fatal(err)
	}
	if !minisign.Verify(pub, payload, sig) {
		t.Fatal("key-file signature did not verify")
	}
}

func TestSignFilesRemovesSignatureWhenEmbeddedVerificationFails(t *testing.T) {
	_, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := minisign.EncryptKey("pw", priv)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MINISIGN_PRIVATE_KEY", string(enc))
	t.Setenv("MINISIGN_PRIVATE_KEY_FILE", "")
	t.Setenv("MINISIGN_PASSWORD", "pw")
	restore := verifySignedArtifact
	verifySignedArtifact = func([]byte, []byte) error { return errors.New("wrong public key") }
	t.Cleanup(func() { verifySignedArtifact = restore })

	artifact := filepath.Join(t.TempDir(), "Clouvia-windows-arm64-installer.exe")
	if err := os.WriteFile(artifact, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := signFiles([]string{artifact}); err == nil || !strings.Contains(err.Error(), "embedded public key") {
		t.Fatalf("signFiles error = %v", err)
	}
	if _, err := os.Stat(artifact + ".minisig"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid signature was not removed: %v", err)
	}
}

func TestSigningPasswordUsesHiddenPromptWhenEnvironmentIsEmpty(t *testing.T) {
	t.Setenv("MINISIGN_PASSWORD", "")
	restore := readSigningPassword
	readSigningPassword = func() (string, error) { return "prompted-password", nil }
	t.Cleanup(func() { readSigningPassword = restore })

	password, err := signingPassword()
	if err != nil || password != "prompted-password" {
		t.Fatalf("signingPassword = %q, %v", password, err)
	}
}

func TestSelfTestSigningKeyMatchesEmbeddedPublicKey(t *testing.T) {
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := minisign.EncryptKey("self-test-password", priv)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MINISIGN_PRIVATE_KEY", string(encrypted))
	t.Setenv("MINISIGN_PRIVATE_KEY_FILE", "")
	t.Setenv("MINISIGN_PASSWORD", "self-test-password")
	useVerificationKey(t, pub)

	if err := selfTestSigningKey(); err != nil {
		t.Fatalf("selfTestSigningKey: %v", err)
	}
}

func TestSelfTestSigningKeyRejectsDifferentEmbeddedPublicKey(t *testing.T) {
	_, privateKey, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	differentPublicKey, _, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := minisign.EncryptKey("self-test-password", privateKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MINISIGN_PRIVATE_KEY", string(encrypted))
	t.Setenv("MINISIGN_PRIVATE_KEY_FILE", "")
	t.Setenv("MINISIGN_PASSWORD", "self-test-password")
	useVerificationKey(t, differentPublicKey)

	if err := selfTestSigningKey(); err == nil || !strings.Contains(err.Error(), "does not match embedded public key") {
		t.Fatalf("selfTestSigningKey mismatch error = %v", err)
	}
}

func TestReencryptSigningKeyPreservesKeyAndRefusesOverwrite(t *testing.T) {
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := minisign.EncryptKey("old-password", priv)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MINISIGN_PRIVATE_KEY", string(encrypted))
	t.Setenv("MINISIGN_PRIVATE_KEY_FILE", "")
	t.Setenv("MINISIGN_PASSWORD", "old-password")
	t.Setenv("MINISIGN_NEW_PASSWORD", "new-password")
	useVerificationKey(t, pub)

	output := filepath.Join(t.TempDir(), "suiyuan.key")
	if err := reencryptSigningKey(output); err != nil {
		t.Fatalf("reencryptSigningKey: %v", err)
	}
	reencrypted, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := minisign.DecryptKey("old-password", reencrypted); err == nil {
		t.Fatal("old password unexpectedly decrypted the re-encrypted key")
	}
	reencryptedPrivate, err := minisign.DecryptKey("new-password", reencrypted)
	if err != nil {
		t.Fatalf("new password did not decrypt key: %v", err)
	}
	message := []byte("same signing identity")
	if !minisign.Verify(pub, message, minisign.Sign(reencryptedPrivate, message)) {
		t.Fatal("re-encrypted private key does not match the original public key")
	}
	if err := reencryptSigningKey(output); err == nil || !strings.Contains(err.Error(), "create output") {
		t.Fatalf("overwrite error = %v", err)
	}
}

func TestReencryptSigningKeyRejectsMissingOrUnchangedPassword(t *testing.T) {
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := minisign.EncryptKey("old-password", priv)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MINISIGN_PRIVATE_KEY", string(encrypted))
	t.Setenv("MINISIGN_PRIVATE_KEY_FILE", "")
	t.Setenv("MINISIGN_PASSWORD", "old-password")
	useVerificationKey(t, pub)

	t.Setenv("MINISIGN_NEW_PASSWORD", "")
	if err := reencryptSigningKey(filepath.Join(t.TempDir(), "missing.key")); err == nil || !strings.Contains(err.Error(), "MINISIGN_NEW_PASSWORD") {
		t.Fatalf("missing new password error = %v", err)
	}
	t.Setenv("MINISIGN_NEW_PASSWORD", "old-password")
	if err := reencryptSigningKey(filepath.Join(t.TempDir(), "same.key")); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("unchanged password error = %v", err)
	}
}

func useVerificationKey(t *testing.T, publicKey minisign.PublicKey) {
	t.Helper()
	restore := verifySignedArtifact
	verifySignedArtifact = func(message, signature []byte) error {
		if !minisign.Verify(publicKey, message, signature) {
			return errors.New("signature verification failed")
		}
		return nil
	}
	t.Cleanup(func() { verifySignedArtifact = restore })
}

// TestGenManifest builds a manifest from a directory of fake artifacts and checks
// both Windows platforms are listed with a download URL, a parallel .minisig URL,
// and a non-empty digest. Manual-download and generated files must be ignored.
func TestGenManifest(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"Clouvia-darwin-arm64.zip",
		"Clouvia-darwin-amd64.zip",
		"Clouvia-windows-amd64-installer.exe",
		"Clouvia-windows-amd64.zip", // portable download, not the updater channel
		"Clouvia-windows-arm64-installer.exe",
		"Clouvia-windows-arm64.zip", // portable download, not the updater channel
		"Clouvia-darwin-universal.dmg",
		"Clouvia-linux-amd64.tar.gz",
		"Clouvia-linux-amd64.deb",            // human download, not the updater channel
		"Clouvia-linux-amd64.tar.gz.minisig", // must be skipped
		"README.txt",                         // unmatched, must be skipped
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GITHUB_REPOSITORY", "xiaoxiaotest9527-bit/KWU")
	t.Setenv("RELEASE_REPOSITORY", "xiaoxiaotest9527-bit/KWU-release")

	if err := genManifest(dir, "v1.2.0", "desktop-v1.2.0"); err != nil {
		t.Fatalf("genManifest: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m update.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("latest.json is not valid: %v", err)
	}
	if m.Version != "v1.2.0" {
		t.Fatalf("version = %q, want v1.2.0", m.Version)
	}
	if len(m.Platforms) != 4 {
		t.Fatalf("want 4 stable platforms, got %d: %v", len(m.Platforms), m.Platforms)
	}
	win, ok := m.Platforms["windows-amd64"]
	if !ok {
		t.Fatal("windows-amd64 missing")
	}
	wantURL := "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases/download/desktop-v1.2.0/" + url.PathEscape("Clouvia-windows-amd64-installer.exe")
	if win.URL != wantURL {
		t.Fatalf("windows url = %q, want %q", win.URL, wantURL)
	}
	if win.Sig != wantURL+".minisig" {
		t.Fatalf("windows sig = %q, want %q.minisig", win.Sig, wantURL)
	}
	if win.SHA256 == "" || win.Size == 0 {
		t.Fatalf("windows asset missing digest/size: %+v", win)
	}
	// The Windows updater channel is the per-arch -installer.exe; the portable .zip
	// must not shadow the windows-arm64 key.
	arm, ok := m.Platforms["windows-arm64"]
	if !ok {
		t.Fatal("windows-arm64 missing")
	}
	if !strings.HasSuffix(arm.URL, "/"+url.PathEscape("Clouvia-windows-arm64-installer.exe")) {
		t.Fatalf("windows-arm64 url = %q, want the installer, not the portable zip", arm.URL)
	}
	darwinAMD64, ok := m.Platforms["darwin-amd64"]
	if !ok {
		t.Fatal("darwin-amd64 missing")
	}
	darwinARM64, ok := m.Platforms["darwin-arm64"]
	if !ok {
		t.Fatal("darwin-arm64 missing")
	}
	if darwinAMD64 != darwinARM64 || !strings.HasSuffix(darwinAMD64.URL, "/Clouvia-darwin-universal.dmg") {
		t.Fatalf("universal DMG mappings differ: amd64=%+v arm64=%+v", darwinAMD64, darwinARM64)
	}
	if _, ok := m.Platforms["linux-amd64"]; ok {
		t.Fatal("Linux is not published in the stable phase-one manifest")
	}
}

func TestGenManifestWindowsARM64Only(t *testing.T) {
	dir := t.TempDir()
	name := "Clouvia-windows-arm64-installer.exe"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("arm64"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "arm64-latest.json")
	if err := genManifestWithOptions(dir, "v1.0.3", "desktop-v1.0.3", manifestOptions{
		Output:    output,
		Platforms: []string{"windows-arm64"},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var manifest update.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Platforms) != 1 {
		t.Fatalf("platform count = %d, want 1", len(manifest.Platforms))
	}
	if _, ok := manifest.Platforms["windows-arm64"]; !ok {
		t.Fatal("windows-arm64 platform is missing")
	}
}

func TestReleaseRepositoryDoesNotFallBackToSourceRepository(t *testing.T) {
	t.Setenv("RELEASE_REPOSITORY", "")
	t.Setenv("GITHUB_REPOSITORY", "xiaoxiaotest9527-bit/KWU")
	if got := releaseRepository(); got != defaultReleaseRepository {
		t.Fatalf("releaseRepository = %q, want dedicated desktop repository %q", got, defaultReleaseRepository)
	}
}

func TestGenManifestWritesGitHubAndOSSVariants(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"Clouvia-windows-amd64-installer.exe",
		"Clouvia-windows-arm64-installer.exe",
		"Clouvia-darwin-universal.dmg",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	notesPath := filepath.Join(dir, "notes.md")
	const notes = "# Release notes\n\n- Fixed updater\n"
	if err := os.WriteFile(notesPath, []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELEASE_REPOSITORY", "xiaoxiaotest9527-bit/KWU-release")
	githubOutput := filepath.Join(dir, "github", "latest.json")
	if err := runManifest([]string{"--notes-file", notesPath, "--output", githubOutput, dir, "v1.0.11", "desktop-v1.0.11"}); err != nil {
		t.Fatalf("GitHub manifest: %v", err)
	}
	ossOutput := filepath.Join(dir, "oss", "latest.json")
	ossBase := "https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com/releases/desktop-v1.0.11"
	downloadPage := "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases/tag/desktop-v1.0.11"
	if err := runManifest([]string{
		"--base-url", ossBase,
		"--download-page", downloadPage,
		"--notes-file", notesPath,
		"--output", ossOutput,
		dir, "v1.0.11", "desktop-v1.0.11",
	}); err != nil {
		t.Fatalf("OSS manifest: %v", err)
	}

	readManifest := func(path string) update.Manifest {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var manifest update.Manifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Fatal(err)
		}
		return manifest
	}
	github := readManifest(githubOutput)
	oss := readManifest(ossOutput)
	if got := github.Platforms["windows-amd64"].URL; !strings.HasPrefix(got, "https://github.com/xiaoxiaotest9527-bit/KWU-release/releases/download/desktop-v1.0.11/") {
		t.Fatalf("GitHub asset URL = %q", got)
	}
	if got := oss.Platforms["windows-amd64"].URL; !strings.HasPrefix(got, ossBase+"/") {
		t.Fatalf("OSS asset URL = %q", got)
	}
	if oss.DownloadPage != downloadPage {
		t.Fatalf("OSS download page = %q", oss.DownloadPage)
	}
	if len(oss.Platforms) != 4 || len(github.Platforms) != 4 {
		t.Fatalf("unexpected platform counts: GitHub=%d OSS=%d", len(github.Platforms), len(oss.Platforms))
	}
	if oss.Notes != notes || github.Notes != notes {
		t.Fatalf("release notes snapshot mismatch: GitHub=%q OSS=%q", github.Notes, oss.Notes)
	}
}

func TestRunManifestRequiresNonEmptyReleaseNotes(t *testing.T) {
	dir := t.TempDir()
	blankNotes := filepath.Join(dir, "blank-notes.md")
	if err := os.WriteFile(blankNotes, []byte(" \n\t"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing flag",
			args: []string{dir, "v1.0.11", "desktop-v1.0.11"},
			want: "--notes-file is required",
		},
		{
			name: "missing file",
			args: []string{"--notes-file", filepath.Join(dir, "missing.md"), dir, "v1.0.11", "desktop-v1.0.11"},
			want: "read notes file",
		},
		{
			name: "blank file",
			args: []string{"--notes-file", blankNotes, dir, "v1.0.11", "desktop-v1.0.11"},
			want: "release notes are empty",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runManifest(test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runManifest error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGenManifestRejectsMissingArchitectureAndInsecureURL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Clouvia-windows-amd64-installer.exe"), []byte("amd64"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := genManifest(dir, "v1.0.11", "desktop-v1.0.11"); err == nil || !strings.Contains(err.Error(), "windows-arm64") {
		t.Fatalf("missing architecture error = %v", err)
	}
	notesPath := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(notesPath, []byte("# Release notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runManifest([]string{"--notes-file", notesPath, "--base-url", "http://example.com/releases", dir, "v1.0.11", "desktop-v1.0.11"}); err == nil {
		t.Fatal("insecure base URL should fail")
	}
}
