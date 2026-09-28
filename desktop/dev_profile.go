package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"kwu/internal/brand"
	cloud "kwu/internal/cloud"
)

const (
	devProfileRootEnv      = "KWU_DEV_PROFILE_ROOT"
	devEnvironmentFileName = "dev-environment.json"
	devEnvironmentSchema   = 1
)

type desktopProfile struct {
	Development            bool
	EnvironmentID          string
	NormalizedCloudBaseURL string
	HomeRoot               string
	StateRoot              string
	CacheRoot              string
	WebViewRoot            string
}

type devEnvironmentMarker struct {
	SchemaVersion int    `json:"schemaVersion"`
	EnvironmentID string `json:"environmentId"`
	CloudBaseURL  string `json:"cloudBaseUrl"`
}

type desktopProfileLocations struct {
	GOOS          string
	UserHomeDir   string
	UserConfigDir string
	UserCacheDir  string
}

var (
	desktopProfileInitOnce sync.Once
	initializedProfile     desktopProfile
	desktopProfileInitErr  error
	desktopCanonicalPaths  atomic.Bool
)

func ensureDesktopProfileInitialized() (desktopProfile, error) {
	desktopProfileInitOnce.Do(func() {
		initializedProfile, desktopProfileInitErr = initializeDesktopProfile(
			version,
			os.Getenv,
			os.Setenv,
			currentDesktopProfileLocations(),
		)
	})
	return initializedProfile, desktopProfileInitErr
}

func validDevEnvironmentID(environmentID string) bool {
	if len(environmentID) != 16 {
		return false
	}
	for _, value := range environmentID {
		if (value < '0' || value > '9') && (value < 'a' || value > 'f') {
			return false
		}
	}
	return true
}

func currentDesktopProfileLocations() desktopProfileLocations {
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	cacheDir, _ := os.UserCacheDir()
	return desktopProfileLocations{
		GOOS:          runtime.GOOS,
		UserHomeDir:   home,
		UserConfigDir: configDir,
		UserCacheDir:  cacheDir,
	}
}

func initializeDesktopProfile(
	buildVersion string,
	getenv func(string) string,
	setenv func(string, string) error,
	locations desktopProfileLocations,
) (desktopProfile, error) {
	if strings.TrimSpace(buildVersion) != "dev" {
		return desktopProfile{}, nil
	}
	if getenv == nil || setenv == nil {
		return desktopProfile{}, fmt.Errorf("desktop profile: environment access is unavailable")
	}

	resolvedBaseURL, err := resolveCloudBaseURL(buildVersion, getenv(cloudBaseURLEnv), cloudDevelopmentBaseURL, cloudProductionBaseURL)
	if err != nil {
		return desktopProfile{}, fmt.Errorf("desktop profile: resolve cloud base URL: %w", err)
	}
	normalizedBaseURL, err := normalizeDevCloudBaseURL(resolvedBaseURL)
	if err != nil {
		return desktopProfile{}, err
	}
	sum := sha256.Sum256([]byte(normalizedBaseURL))
	environmentID := hex.EncodeToString(sum[:])[:16]

	profile, err := resolveDevProfileRoots(environmentID, strings.TrimSpace(getenv(devProfileRootEnv)), locations)
	if err != nil {
		return desktopProfile{}, err
	}
	profile.Development = true
	profile.EnvironmentID = environmentID
	profile.NormalizedCloudBaseURL = normalizedBaseURL

	marker := devEnvironmentMarker{
		SchemaVersion: devEnvironmentSchema,
		EnvironmentID: environmentID,
		CloudBaseURL:  normalizedBaseURL,
	}
	if err := ensureDevEnvironmentMarker(profile.StateRoot, marker); err != nil {
		return desktopProfile{}, err
	}
	if err := setDesktopProfileEnvironment(getenv, setenv, profile, desktopCanonicalPaths.Load()); err != nil {
		return desktopProfile{}, err
	}
	return profile, nil
}

func normalizeDevCloudBaseURL(raw string) (string, error) {
	normalized, err := cloud.NormalizeEnvironmentBaseURL(raw)
	if err != nil {
		return "", fmt.Errorf("desktop profile: %w", err)
	}
	return normalized, nil
}

func resolveDevProfileRoots(environmentID, override string, locations desktopProfileLocations) (desktopProfile, error) {
	var profile desktopProfile
	if override != "" {
		if !filepath.IsAbs(override) {
			return profile, fmt.Errorf("desktop profile: %s must be an absolute path", devProfileRootEnv)
		}
		root := filepath.Clean(override)
		for _, productionRoot := range knownProductionProfileRoots(locations) {
			if pathEqualOrWithin(root, productionRoot, locations.GOOS) {
				return profile, fmt.Errorf("desktop profile: %s must not equal or be inside production data root %q", devProfileRootEnv, productionRoot)
			}
		}
		profile.HomeRoot = root
		profile.StateRoot = root
		profile.CacheRoot = filepath.Join(root, "cache")
		if locations.GOOS == "windows" {
			profile.WebViewRoot = filepath.Join(root, "webview")
		}
		return profile, nil
	}

	switch locations.GOOS {
	case "windows":
		if strings.TrimSpace(locations.UserConfigDir) == "" || strings.TrimSpace(locations.UserCacheDir) == "" {
			return profile, fmt.Errorf("desktop profile: Windows AppData and LocalAppData directories are required")
		}
		root := filepath.Join(locations.UserConfigDir, "kwu-dev", "environments", environmentID)
		cacheEnvironmentRoot := filepath.Join(locations.UserCacheDir, "kwu-dev", "environments", environmentID)
		profile.HomeRoot = root
		profile.StateRoot = root
		profile.CacheRoot = filepath.Join(cacheEnvironmentRoot, "cache")
		profile.WebViewRoot = filepath.Join(cacheEnvironmentRoot, "webview")
	default:
		if strings.TrimSpace(locations.UserHomeDir) == "" || strings.TrimSpace(locations.UserCacheDir) == "" {
			return profile, fmt.Errorf("desktop profile: user home and cache directories are required")
		}
		root := filepath.Join(locations.UserHomeDir, ".kwu-dev", "environments", environmentID)
		profile.HomeRoot = root
		profile.StateRoot = root
		profile.CacheRoot = filepath.Join(locations.UserCacheDir, "kwu-dev", "environments", environmentID, "cache")
	}
	return profile, nil
}

func knownProductionProfileRoots(locations desktopProfileLocations) []string {
	var roots []string
	add := func(path string) {
		if strings.TrimSpace(path) != "" {
			roots = append(roots, filepath.Clean(path))
		}
	}
	if locations.GOOS == "windows" {
		if locations.UserConfigDir != "" {
			add(filepath.Join(locations.UserConfigDir, "Clouvia"))
			add(filepath.Join(locations.UserConfigDir, "kwu"))
			// Wails v2 defaults WebView2 data to %APPDATA%/<executable>.
			add(filepath.Join(locations.UserConfigDir, "clouvia-desktop.exe"))
			add(filepath.Join(locations.UserConfigDir, "kwu-desktop.exe"))
		}
		if locations.UserCacheDir != "" {
			add(filepath.Join(locations.UserCacheDir, "kwu"))
		}
		return roots
	}
	if locations.UserHomeDir != "" {
		add(filepath.Join(locations.UserHomeDir, ".kwu"))
	}
	if locations.UserCacheDir != "" {
		add(filepath.Join(locations.UserCacheDir, "kwu"))
	}
	return roots
}

func pathEqualOrWithin(candidate, root, goos string) bool {
	if candidate == "" || root == "" {
		return false
	}
	candidate = filepath.Clean(candidate)
	root = filepath.Clean(root)
	if goos == "windows" {
		candidate = strings.ToLower(candidate)
		root = strings.ToLower(root)
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func ensureDevEnvironmentMarker(stateRoot string, want devEnvironmentMarker) error {
	if strings.TrimSpace(stateRoot) == "" {
		return fmt.Errorf("desktop profile: development state root is unavailable")
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return fmt.Errorf("desktop profile: create development state root: %w", err)
	}
	if err := os.Chmod(stateRoot, 0o700); err != nil {
		return fmt.Errorf("desktop profile: protect development state root: %w", err)
	}
	markerPath := filepath.Join(stateRoot, devEnvironmentFileName)
	if found, err := readDevEnvironmentMarker(markerPath); err != nil {
		return err
	} else if found != nil {
		return validateDevEnvironmentMarker(markerPath, *found, want)
	}

	data, err := json.Marshal(want)
	if err != nil {
		return fmt.Errorf("desktop profile: encode environment marker: %w", err)
	}
	file, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		found, readErr := readDevEnvironmentMarker(markerPath)
		if readErr != nil {
			return readErr
		}
		if found == nil {
			return fmt.Errorf("desktop profile: environment marker disappeared during initialization")
		}
		return validateDevEnvironmentMarker(markerPath, *found, want)
	}
	if err != nil {
		return fmt.Errorf("desktop profile: create environment marker: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("desktop profile: write environment marker: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("desktop profile: sync environment marker: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("desktop profile: close environment marker: %w", err)
	}
	return nil
}

func readDevEnvironmentMarker(path string) (*devEnvironmentMarker, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("desktop profile: open environment marker: %w", err)
	}
	defer file.Close()
	var marker devEnvironmentMarker
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&marker); err != nil {
		return nil, fmt.Errorf("desktop profile: decode environment marker: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("desktop profile: environment marker has trailing content")
	}
	return &marker, nil
}

func validateDevEnvironmentMarker(path string, got, want devEnvironmentMarker) error {
	if got != want {
		return fmt.Errorf(
			"desktop profile: environment marker %q conflicts with cloud environment (schemaVersion=%d environmentId=%q cloudBaseUrl=%q)",
			path,
			got.SchemaVersion,
			got.EnvironmentID,
			got.CloudBaseURL,
		)
	}
	return nil
}

func setDesktopProfileEnvironment(
	getenv func(string) string,
	setenv func(string, string) error,
	profile desktopProfile,
	includeCanonical bool,
) error {
	type environmentValue struct {
		name  string
		value string
	}
	values := []environmentValue{}
	if includeCanonical {
		values = append(values,
			environmentValue{name: brand.PathEnvPrefix + "_HOME", value: profile.HomeRoot},
			environmentValue{name: brand.PathEnvPrefix + "_STATE_HOME", value: profile.StateRoot},
			environmentValue{name: brand.PathEnvPrefix + "_CACHE_HOME", value: profile.CacheRoot},
		)
	}
	values = append(values,
		environmentValue{name: "KWU_HOME", value: profile.HomeRoot},
		environmentValue{name: "KWU_STATE_HOME", value: profile.StateRoot},
		environmentValue{name: "KWU_CACHE_HOME", value: profile.CacheRoot},
	)
	originals := make(map[string]string, len(values))
	for _, value := range values {
		originals[value.name] = getenv(value.name)
	}
	for index, value := range values {
		if err := setenv(value.name, value.value); err != nil {
			for rollback := index - 1; rollback >= 0; rollback-- {
				_ = setenv(values[rollback].name, originals[values[rollback].name])
			}
			return fmt.Errorf("desktop profile: set %s: %w", value.name, err)
		}
	}
	return nil
}

func desktopWebViewUserDataPath() string {
	if strings.TrimSpace(version) != "dev" {
		return strings.TrimSpace(os.Getenv("KWU_WEBVIEW_USER_DATA_PATH"))
	}
	profile, err := ensureDesktopProfileInitialized()
	if err != nil {
		return ""
	}
	return profile.WebViewRoot
}
