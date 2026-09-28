// Command sign is the CI-side signing and manifest tool for desktop releases. It
// is never shipped in any artifact 鈥?the release workflow invokes it via
// `go run ./cmd/sign`. It shares desktop/internal/update with the running updater
// so the sign path and the verify path use one definition of the manifest and one
// minisign implementation.
//
// Subcommands:
//
//	sign <file>...               Write <file>.minisig for each file, signing with the
//	                             encrypted minisign private key in $MINISIGN_PRIVATE_KEY
//	                             or the file at $MINISIGN_PRIVATE_KEY_FILE.
//
//	manifest [flags] <dir> <ver> <tag>
//	                             Scan <dir> for Windows installers, compute size +
//	                             sha256, and write a GitHub or OSS latest.json.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"aead.dev/minisign"
	"golang.org/x/term"

	"kwu/desktop/internal/update"
)

// platforms are the manifest keys we publish. A built artifact is matched to a key
// by substring (file names embed the key, e.g. Clouvia-windows-amd64-installer.exe), so the
// generator and the updater agree on update.PlatformKey output.
var platforms = []string{"windows-amd64", "windows-arm64", "darwin-amd64", "darwin-arm64"}

const defaultReleaseRepository = "xiaoxiaotest9527-bit/KWU-release"

var (
	readSigningPassword  = terminalSigningPassword
	verifySignedArtifact = update.Verify
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "sign":
		err = signFiles(os.Args[2:])
	case "manifest":
		err = runManifest(os.Args[2:])
	case "genkey":
		if len(os.Args) != 3 {
			usage()
		}
		err = genKey(os.Args[2])
	case "verify":
		if len(os.Args) != 3 {
			usage()
		}
		err = verifyFile(os.Args[2])
	case "selftest":
		if len(os.Args) != 2 {
			usage()
		}
		err = selfTestSigningKey()
	case "reencrypt":
		if len(os.Args) != 3 {
			usage()
		}
		err = reencryptSigningKey(os.Args[2])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sign:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:\n  sign <file>...\n  manifest --notes-file FILE [--base-url URL] [--download-page URL] [--output FILE] <dir> <version> <tag>\n  genkey <dir>\n  verify <file>\n  selftest\n  reencrypt <output-key-file>")
	os.Exit(2)
}

// verifyFile checks <file> against <file>.minisig using the embedded public key 鈥?// the same check the updater runs before applying. A self-test that the signing
// key matches what's compiled in. Returns an error (nonzero exit) on mismatch.
func verifyFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(path + ".minisig")
	if err != nil {
		return err
	}
	if err := update.Verify(data, sig); err != nil {
		return err
	}
	fmt.Printf("OK: %s verifies against the embedded public key\n", path)
	return nil
}

// genKey generates a fresh minisign key pair, writing the encrypted private key
// (kwu.key) and the public key (kwu.pub) into dir. The password comes
// from $MINISIGN_PASSWORD. The public key is printed 鈥?it's safe to publish; embed
// it in internal/update/verify.go. The private key never leaves dir.
func genKey(dir string) error {
	pw, err := signingPassword()
	if err != nil {
		return fmt.Errorf("genkey: %w", err)
	}
	pub, priv, err := minisign.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	enc, err := minisign.EncryptKey(pw, priv)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	keyPath := filepath.Join(dir, "kwu.key")
	pubPath := filepath.Join(dir, "kwu.pub")
	if err := os.WriteFile(keyPath, enc, 0o600); err != nil {
		return err
	}
	pubText, err := pub.MarshalText()
	if err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, pubText, 0o644); err != nil {
		return err
	}
	fmt.Printf("private key -> %s (keep secret; this is the MINISIGN_PRIVATE_KEY value)\n", keyPath)
	fmt.Printf("public key  -> %s\n\n", pubPath)
	fmt.Printf("public key (embed in internal/update/verify.go, key ID %016X):\n%s\n", pub.ID(), pubText)
	return nil
}

// signFiles writes a detached .minisig next to each input file. The encrypted
// private key may be supplied directly by environment or read from a local file;
// neither the key nor its password is accepted through argv.
func signFiles(files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("sign: no files given")
	}
	keyText, err := signingPrivateKey()
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	password, err := signingPassword()
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	priv, err := minisign.DecryptKey(password, keyText)
	if err != nil {
		return fmt.Errorf("sign: decrypt private key: %w", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		sig := minisign.SignWithComments(priv, data,
			"file:"+filepath.Base(f), "KWU desktop release")
		out := f + ".minisig"
		if err := os.WriteFile(out, sig, 0o644); err != nil {
			return err
		}
		if err := verifySignedArtifact(data, sig); err != nil {
			_ = os.Remove(out)
			return fmt.Errorf("sign: verify %s with embedded public key: %w", f, err)
		}
		fmt.Printf("signed %s -> %s\n", f, out)
	}
	return nil
}

// selfTestSigningKey proves that the configured encrypted private key can be
// decrypted and that it matches the public key embedded in the desktop updater.
// No private material, password, challenge, or signature is printed.
func selfTestSigningKey() error {
	keyText, err := signingPrivateKey()
	if err != nil {
		return fmt.Errorf("selftest: %w", err)
	}
	password, err := signingPassword()
	if err != nil {
		return fmt.Errorf("selftest: %w", err)
	}
	priv, err := minisign.DecryptKey(password, keyText)
	if err != nil {
		return fmt.Errorf("selftest: decrypt private key: %w", err)
	}
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return fmt.Errorf("selftest: generate challenge: %w", err)
	}
	signature := minisign.SignWithComments(priv, challenge, "KWU release key self-test", "temporary challenge")
	if err := verifySignedArtifact(challenge, signature); err != nil {
		return fmt.Errorf("selftest: private key does not match embedded public key: %w", err)
	}
	fmt.Println("OK: configured minisign private key matches the embedded public key")
	return nil
}

// reencryptSigningKey writes the same private key encrypted with
// MINISIGN_NEW_PASSWORD. The old password follows the normal hidden-input or
// MINISIGN_PASSWORD path. Passwords and private material are never accepted as
// command-line arguments or printed.
func reencryptSigningKey(output string) error {
	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("reencrypt: output path is empty")
	}
	keyText, err := signingPrivateKey()
	if err != nil {
		return fmt.Errorf("reencrypt: %w", err)
	}
	oldPassword, err := signingPassword()
	if err != nil {
		return fmt.Errorf("reencrypt: %w", err)
	}
	newPassword := os.Getenv("MINISIGN_NEW_PASSWORD")
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("reencrypt: MINISIGN_NEW_PASSWORD is empty")
	}
	if newPassword == oldPassword {
		return fmt.Errorf("reencrypt: new password must differ from the old password")
	}
	priv, err := minisign.DecryptKey(oldPassword, keyText)
	if err != nil {
		return fmt.Errorf("reencrypt: decrypt private key: %w", err)
	}

	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return fmt.Errorf("reencrypt: generate challenge: %w", err)
	}
	signature := minisign.SignWithComments(priv, challenge, "KWU release key re-encryption", "temporary challenge")
	if err := verifySignedArtifact(challenge, signature); err != nil {
		return fmt.Errorf("reencrypt: private key does not match embedded public key: %w", err)
	}

	encrypted, err := minisign.EncryptKey(newPassword, priv)
	if err != nil {
		return fmt.Errorf("reencrypt: encrypt private key: %w", err)
	}
	if _, err := minisign.DecryptKey(newPassword, encrypted); err != nil {
		return fmt.Errorf("reencrypt: verify encrypted private key: %w", err)
	}

	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("reencrypt: create output: %w", err)
	}
	wrote := false
	defer func() {
		_ = file.Close()
		if !wrote {
			_ = os.Remove(output)
		}
	}()
	if _, err := file.Write(encrypted); err != nil {
		return fmt.Errorf("reencrypt: write output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("reencrypt: close output: %w", err)
	}
	wrote = true
	fmt.Printf("re-encrypted private key -> %s\n", output)
	return nil
}

func signingPrivateKey() ([]byte, error) {
	if key := os.Getenv("MINISIGN_PRIVATE_KEY"); strings.TrimSpace(key) != "" {
		return []byte(key), nil
	}
	path := strings.TrimSpace(os.Getenv("MINISIGN_PRIVATE_KEY_FILE"))
	if path == "" {
		return nil, fmt.Errorf("MINISIGN_PRIVATE_KEY and MINISIGN_PRIVATE_KEY_FILE are empty")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read MINISIGN_PRIVATE_KEY_FILE: %w", err)
	}
	if len(strings.TrimSpace(string(key))) == 0 {
		return nil, fmt.Errorf("MINISIGN_PRIVATE_KEY_FILE is empty")
	}
	return key, nil
}

func signingPassword() (string, error) {
	if password := os.Getenv("MINISIGN_PASSWORD"); strings.TrimSpace(password) != "" {
		return password, nil
	}
	password, err := readSigningPassword()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(password) == "" {
		return "", fmt.Errorf("MINISIGN_PASSWORD is empty")
	}
	return password, nil
}

func terminalSigningPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("MINISIGN_PASSWORD is empty and stdin is not a terminal")
	}
	fmt.Fprint(os.Stderr, "Minisign private key password: ")
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read minisign password: %w", err)
	}
	return string(password), nil
}

type manifestOptions struct {
	BaseURL      string
	DownloadPage string
	Output       string
	NotesFile    string
	Platforms    []string
}

func runManifest(args []string) error {
	flags := flag.NewFlagSet("manifest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options manifestOptions
	flags.StringVar(&options.BaseURL, "base-url", "", "artifact URL prefix")
	flags.StringVar(&options.DownloadPage, "download-page", "", "human-facing release page")
	flags.StringVar(&options.Output, "output", "", "manifest output path")
	flags.StringVar(&options.NotesFile, "notes-file", "", "markdown release notes snapshot")
	flags.Func("platform", "required updater platform (repeatable)", func(value string) error {
		options.Platforms = append(options.Platforms, value)
		return nil
	})
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if strings.TrimSpace(options.NotesFile) == "" {
		return fmt.Errorf("manifest: --notes-file is required")
	}
	if flags.NArg() != 3 {
		return fmt.Errorf("manifest: expected <dir> <version> <tag>")
	}
	return genManifestWithOptions(flags.Arg(0), flags.Arg(1), flags.Arg(2), options)
}

// genManifest scans dir for the supported updater artifacts and writes
// dir/latest.json. This wrapper preserves the original positional API.
// version is the semver compared by the updater (e.g. "v1.1.0"); tag is the GitHub
// release tag used in download URLs (e.g. "desktop-v1.1.0").
func genManifest(dir, version, tag string) error {
	return genManifestWithOptions(dir, version, tag, manifestOptions{})
}

func genManifestWithOptions(dir, version, tag string, options manifestOptions) error {
	repo := releaseRepository()
	baseURL := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, tag)
	}
	if err := validateHTTPSURL(baseURL); err != nil {
		return fmt.Errorf("manifest: base URL: %w", err)
	}
	downloadPage := strings.TrimSpace(options.DownloadPage)
	if downloadPage == "" {
		downloadPage = fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	}
	if err := validateHTTPSURL(downloadPage); err != nil {
		return fmt.Errorf("manifest: download page: %w", err)
	}
	m := update.Manifest{
		Version:      version,
		DownloadPage: downloadPage,
		Platforms:    map[string]update.Asset{},
	}
	if strings.TrimSpace(options.NotesFile) != "" {
		notes, err := os.ReadFile(options.NotesFile)
		if err != nil {
			return fmt.Errorf("manifest: read notes file: %w", err)
		}
		if strings.TrimSpace(string(notes)) == "" {
			return fmt.Errorf("manifest: release notes are empty")
		}
		m.Notes = string(notes)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasSuffix(name, ".minisig") || name == "latest.json" {
			continue
		}
		keys := matchPlatforms(name)
		if len(keys) == 0 {
			continue
		}
		size, sum, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		artifactURL := baseURL + "/" + url.PathEscape(name)
		asset := update.Asset{URL: artifactURL, Sig: artifactURL + ".minisig", Size: size, SHA256: sum}
		for _, key := range keys {
			m.Platforms[key] = asset
			fmt.Printf("manifest: %s -> %s (%d bytes)\n", key, name, size)
		}
	}
	requiredPlatforms := options.Platforms
	if len(requiredPlatforms) == 0 {
		requiredPlatforms = platforms
	}
	seen := map[string]bool{}
	for _, platform := range requiredPlatforms {
		if !isSupportedPlatform(platform) {
			return fmt.Errorf("manifest: unsupported required platform %s", platform)
		}
		if seen[platform] {
			return fmt.Errorf("manifest: duplicate required platform %s", platform)
		}
		seen[platform] = true
		if _, ok := m.Platforms[platform]; !ok {
			return fmt.Errorf("manifest: required platform artifact %s is missing from %s", platform, dir)
		}
	}
	for platform := range m.Platforms {
		if !seen[platform] {
			delete(m.Platforms, platform)
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	output := strings.TrimSpace(options.Output)
	if output == "" {
		output = filepath.Join(dir, "latest.json")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, append(b, '\n'), 0o644)
}

func validateHTTPSURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("must be an absolute HTTPS URL")
	}
	return nil
}

func releaseRepository() string {
	if repo := strings.TrimSpace(os.Getenv("RELEASE_REPOSITORY")); repo != "" {
		return repo
	}
	return defaultReleaseRepository
}

// matchPlatforms returns the manifest keys represented by an artifact name.
func isSupportedPlatform(platform string) bool {
	for _, candidate := range platforms {
		if platform == candidate {
			return true
		}
	}
	return false
}

func matchPlatforms(name string) []string {
	// The .deb is a human-download package (like the macOS .dmg); the Linux updater
	// channel is the .tar.gz. Skip it so it doesn't shadow the tarball's linux-amd64 key.
	if strings.HasSuffix(name, ".deb") {
		return nil
	}
	// The Windows updater channel is the per-arch -installer.exe; the portable .zip
	// is a human download, so skip it or it would shadow the installer's key.
	if strings.Contains(name, "windows-") && !strings.HasSuffix(name, "-installer.exe") {
		return nil
	}
	if name == "Clouvia-darwin-universal.dmg" {
		return []string{"darwin-amd64", "darwin-arm64"}
	}
	for _, p := range platforms {
		if strings.Contains(name, p) {
			return []string{p}
		}
	}
	return nil
}

// hashFile returns the size and lowercase-hex SHA-256 of a file, streaming it so
// large artifacts don't have to fit in memory.
func hashFile(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}
