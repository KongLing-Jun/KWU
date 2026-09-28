package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	cloud "kwu/internal/cloud"
)

const devRuntimeManifestEnv = "REASONIX_DEV_RUNTIME_MANIFEST"

var cloudEnvironmentUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type developmentRuntimeManifest struct {
	SchemaVersion        int             `json:"schema_version"`
	CloudBaseURL         string          `json:"cloud_base_url"`
	CloudEnvironmentID   string          `json:"cloud_environment_id"`
	CloudEnvironmentName string          `json:"cloud_environment_name"`
	CreatedAt            string          `json:"created_at"`
	Repositories         json.RawMessage `json:"repositories"`
	Services             json.RawMessage `json:"services"`
}

type cloudEnvironmentResponse struct {
	Data struct {
		SchemaVersion   int    `json:"schema_version"`
		EnvironmentID   string `json:"environment_id"`
		EnvironmentName string `json:"environment_name"`
	} `json:"data"`
	Error      json.RawMessage `json:"error"`
	RequestID  string          `json:"request_id"`
	ServerTime string          `json:"server_time"`
}

func preflightDevelopmentRuntime(profile desktopProfile) error {
	if version != "dev" || !profile.Development {
		return nil
	}
	manifestPath := strings.TrimSpace(os.Getenv(devRuntimeManifestEnv))
	if manifestPath == "" {
		return nil
	}
	return validateDevelopmentRuntimeManifest(profile, manifestPath, &http.Client{Timeout: 5 * time.Second})
}

func validateDevelopmentRuntimeManifest(profile desktopProfile, manifestPath string, client *http.Client) error {
	if !filepath.IsAbs(manifestPath) {
		return fmt.Errorf("desktop runtime manifest: %s must be an absolute path", devRuntimeManifestEnv)
	}
	file, err := os.Open(filepath.Clean(manifestPath))
	if err != nil {
		return fmt.Errorf("desktop runtime manifest: open: %w", err)
	}
	defer file.Close()

	var manifest developmentRuntimeManifest
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("desktop runtime manifest: decode: %w", err)
	}
	if manifest.SchemaVersion != 1 || !cloudEnvironmentUUIDPattern.MatchString(manifest.CloudEnvironmentID) {
		return fmt.Errorf("desktop runtime manifest: invalid schema or environment identity")
	}
	normalized, err := cloud.NormalizeEnvironmentBaseURL(manifest.CloudBaseURL)
	if err != nil || normalized != manifest.CloudBaseURL {
		return fmt.Errorf("desktop runtime manifest: cloud URL is not normalized")
	}
	if manifest.CloudBaseURL != profile.NormalizedCloudBaseURL {
		return fmt.Errorf("desktop runtime manifest: %w", cloud.ErrEnvironmentMismatch)
	}
	if client == nil {
		return fmt.Errorf("desktop runtime manifest: HTTP client is unavailable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(manifest.CloudBaseURL, "/")+"/environmentz", nil)
	if err != nil {
		return fmt.Errorf("desktop runtime manifest: build environment request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("desktop runtime manifest: verify Cloud environment: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("desktop runtime manifest: Cloud environment endpoint returned HTTP %d", response.StatusCode)
	}
	var actual cloudEnvironmentResponse
	responseDecoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	responseDecoder.DisallowUnknownFields()
	if err := responseDecoder.Decode(&actual); err != nil {
		return fmt.Errorf("desktop runtime manifest: decode Cloud environment: %w", err)
	}
	if actual.Data.SchemaVersion != 1 || actual.Data.EnvironmentID != manifest.CloudEnvironmentID {
		return fmt.Errorf("desktop runtime manifest: %w", cloud.ErrEnvironmentMismatch)
	}
	return nil
}
