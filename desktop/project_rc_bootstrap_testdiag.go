//go:build testdiag

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/cloud"
)

const workBuddyRCManifestEnv = "WORKBUDDY_RC_MANIFEST"

func workBuddyRCTestMode() bool {
	return strings.TrimSpace(os.Getenv(workBuddyRCManifestEnv)) != ""
}

type workBuddyRCTestManifest struct {
	BaseURL                string    `json:"base_url"`
	AccessTokenA           string    `json:"access_token_a"`
	RefreshTokenA          string    `json:"refresh_token_a"`
	AccessTokenExpiresAtA  time.Time `json:"access_token_expires_at_a"`
	RefreshTokenExpiresAtA time.Time `json:"refresh_token_expires_at_a"`
	DeviceIDA              string    `json:"device_id_a"`
	UserA                  string    `json:"user_a"`
}

// prepareWorkBuddyRCTest seeds only the output-only testdiag executable's
// isolated profile. The ordinary build has no manifest or token bootstrap.
func prepareWorkBuddyRCTest(ctx context.Context, app *App) (returnErr error) {
	path := strings.TrimSpace(os.Getenv(workBuddyRCManifestEnv))
	if path == "" {
		return nil
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s must be absolute", workBuddyRCManifestEnv)
	}
	stage := "read_manifest"
	defer func() {
		_ = writeWorkBuddyRCBootstrapReport(path+".desktop-bootstrap.json", stage, returnErr == nil)
	}()
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read WorkBuddy RC manifest: %w", err)
	}
	var manifest workBuddyRCTestManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return fmt.Errorf("decode WorkBuddy RC manifest: %w", err)
	}
	stage = "validate_runtime"
	if app == nil || app.cloudRuntime == nil || !app.cloudRuntime.available {
		return fmt.Errorf("WorkBuddy RC cloud runtime is unavailable")
	}
	if strings.TrimRight(manifest.BaseURL, "/") != strings.TrimRight(app.cloudRuntime.projectEnvironment.CloudBaseURL, "/") ||
		strings.TrimSpace(manifest.AccessTokenA) == "" || strings.TrimSpace(manifest.RefreshTokenA) == "" ||
		strings.TrimSpace(manifest.DeviceIDA) == "" || strings.TrimSpace(manifest.UserA) == "" ||
		!manifest.AccessTokenExpiresAtA.After(time.Now().UTC()) || !manifest.RefreshTokenExpiresAtA.After(time.Now().UTC()) {
		return fmt.Errorf("WorkBuddy RC manifest does not match the configured cloud")
	}
	stage = "confirm_binding"
	actor := app.cloudActorContext().Normalize()
	binding, err := app.cloudRuntime.bindings.ConfirmBinding(ctx, actor, manifest.UserA)
	if err != nil {
		return fmt.Errorf("prepare WorkBuddy RC binding: %w", err)
	}
	stage = "resolve_scope"
	scope, err := binding.Scope()
	if err != nil {
		return err
	}
	stage = "save_session"
	if err := app.cloudRuntime.sessions.Save(ctx, actor, cloud.Session{
		Scope: scope, State: cloud.SessionAuthenticated, AccessToken: manifest.AccessTokenA,
		RefreshToken: manifest.RefreshTokenA, DeviceID: manifest.DeviceIDA,
		AccessTokenExpiresAt: manifest.AccessTokenExpiresAtA, RefreshTokenExpiresAt: manifest.RefreshTokenExpiresAtA,
	}); err != nil {
		return fmt.Errorf("prepare WorkBuddy RC session: %w", err)
	}
	stage = "complete"
	return nil
}

func writeWorkBuddyRCBootstrapReport(path, stage string, ok bool) error {
	payload, err := json.Marshal(map[string]any{
		"schema": "reasonix.workbuddy-rc-bootstrap/v1",
		"stage":  stage,
		"ok":     ok,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o600)
}
