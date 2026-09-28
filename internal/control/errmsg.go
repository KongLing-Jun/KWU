package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kwu/internal/brand"
	"kwu/internal/i18n"
	"kwu/internal/provider"
)

// ExplainError maps a provider failure to an actionable, localized message
// so the turn-done error the UI shows is never a bare status code or silent
// failure. Unknown errors (and nil) pass through unchanged.
func ExplainError(err error) error {
	if err == nil {
		return nil
	}
	if provider.IsStreamInterrupted(err) {
		return fmt.Errorf("model stream interrupted after recovery attempts: %s. The partial response was kept; retry or ask %s to continue", err.Error(), brand.DisplayName)
	}
	if provider.IsConnReset(err) {
		return fmt.Errorf("model stream disconnected before completion after retry attempts: %s. Check the provider/proxy connection, then retry or ask %s to continue", err.Error(), brand.DisplayName)
	}
	var apiErr *provider.APIError
	if errors.As(err, &apiErr) {
		return explainAPIError(err, apiErr)
	}
	var authErr *provider.AuthError
	if errors.As(err, &authErr) {
		return explainAuthError(authErr)
	}
	return err
}

func explainError(err error) error {
	return ExplainError(err)
}

func explainAPIError(original error, apiErr *provider.APIError) error {
	msg := providerAPIErrorMessage(apiErr)
	if msg == "" {
		return original
	}
	if reason := requestErrorReason(apiErr); reason != "" {
		return fmt.Errorf("%s\n%s", msg, reason)
	}
	return errors.New(msg)
}

func explainAuthError(authErr *provider.AuthError) error {
	if authErr.Status == 403 {
		return errors.New(i18n.M.ProviderErrForbidden)
	}
	if strings.EqualFold(authErr.Provider, "airclaude_subscription") {
		return errors.New(i18n.M.ProviderErrOfficialAuth)
	}
	msg := i18n.M.ProviderErrAuth
	if authErr.HasKey {
		msg = i18n.M.ProviderErrAuthRejected
	}
	if authErr.KeyEnv == "" {
		return errors.New(msg)
	}
	if authErr.KeySource != "" {
		return fmt.Errorf("%s (%s from %s)", msg, authErr.KeyEnv, authErr.KeySource)
	}
	if authErr.KeyOwner != "" {
		return fmt.Errorf("%s (%s for %s)", msg, authErr.KeyEnv, authErr.KeyOwner)
	}
	return fmt.Errorf("%s (%s)", msg, authErr.KeyEnv)
}

func providerAPIErrorMessage(apiErr *provider.APIError) string {
	switch strings.ToLower(strings.TrimSpace(apiErr.Code)) {
	case "pre_consume_token_quota_failed", "insufficient_user_quota":
		return i18n.M.ProviderErrInsufficientCredits
	case "access_denied":
		return i18n.M.ProviderErrForbidden
	case "model_not_found":
		return i18n.M.ProviderErrModelUnavailable
	}
	if msg := i18n.M.ProviderStatusMessage(apiErr.Status); msg != "" {
		return msg
	}
	if apiErr.Code != "" || apiErr.Message != "" {
		return i18n.M.ProviderErrRequestFailed
	}
	return ""
}

// requestErrorReason returns the provider's verbatim reason for request-shaped
// 4xx (400/422) — the localized line names the category, the body names the
// actual cause (context-length exceeded, unpaired tool_calls). Empty otherwise.
func requestErrorReason(e *provider.APIError) string {
	if e.Status != 400 && e.Status != 422 {
		return ""
	}
	return providerBodyReason(e.Body)
}

// providerBodyReason pulls the human reason from an OpenAI/Anthropic-shaped error
// body ({"error":{"message":…}}), falling back to the trimmed raw body.
func providerBodyReason(body string) string {
	if body == "" {
		return ""
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(body), &parsed) == nil && parsed.Error.Message != "" {
		return clampRunes(parsed.Error.Message, 800)
	}
	return clampRunes(body, 800)
}

func clampRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
