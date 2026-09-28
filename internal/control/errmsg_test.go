package control

import (
	"errors"
	"io"
	"strings"
	"testing"

	"kwu/internal/i18n"
	"kwu/internal/provider"
)

func TestExplainError(t *testing.T) {
	if explainError(nil) != nil {
		t.Error("nil should stay nil")
	}

	bal := explainError(&provider.APIError{Provider: "deepseek", Status: 402, Body: "Insufficient Balance"})
	if bal.Error() != i18n.M.ProviderErrInsufficientBalance {
		t.Errorf("402 = %q, want the insufficient-balance message", bal.Error())
	}

	auth := explainError(&provider.AuthError{Provider: "deepseek", KeyEnv: "DEEPSEEK_API_KEY", Status: 401})
	if !strings.Contains(auth.Error(), "DEEPSEEK_API_KEY") {
		t.Errorf("401 should name the key env: %q", auth.Error())
	}
	if !strings.Contains(auth.Error(), i18n.M.ProviderErrAuth) {
		t.Errorf("401 without a key should use the missing-key message: %q", auth.Error())
	}

	rejected := explainError(&provider.AuthError{Provider: "mimo", KeyEnv: "MIMO_API_KEY", Status: 401, HasKey: true})
	if !strings.Contains(rejected.Error(), i18n.M.ProviderErrAuthRejected) {
		t.Errorf("401 with a key present should use the server-rejected message: %q", rejected.Error())
	}
	if !strings.Contains(rejected.Error(), "MIMO_API_KEY") {
		t.Errorf("401 should still name the key env: %q", rejected.Error())
	}

	sourced := explainError(&provider.AuthError{Provider: "deepseek", KeyEnv: "DEEPSEEK_API_KEY", KeySource: "project .env", Status: 401, HasKey: true})
	if !strings.Contains(sourced.Error(), "DEEPSEEK_API_KEY from project .env") {
		t.Errorf("401 should name the key source: %q", sourced.Error())
	}

	owned := explainError(&provider.AuthError{Provider: "deepseek", KeyEnv: "DEEPSEEK_API_KEY", KeyOwner: "profile work", Status: 401, HasKey: true})
	if !strings.Contains(owned.Error(), "DEEPSEEK_API_KEY for profile work") {
		t.Errorf("401 should name the key owner when source is unavailable: %q", owned.Error())
	}

	officialAuth := explainError(&provider.AuthError{
		Provider:  "airclaude_subscription",
		KeySource: "cloud account MaaS credential",
		Status:    401,
		HasKey:    true,
	})
	if officialAuth.Error() != i18n.M.ProviderErrOfficialAuth ||
		strings.Contains(officialAuth.Error(), "MaaS") {
		t.Errorf("official 401 should ask the user to sign in again: %q", officialAuth.Error())
	}

	forbidden := explainError(&provider.AuthError{
		Provider: "airclaude_subscription",
		Status:   403,
		HasKey:   true,
	})
	if forbidden.Error() != i18n.M.ProviderErrForbidden {
		t.Errorf("generic 403 = %q, want access guidance", forbidden.Error())
	}

	credits := explainError(&provider.APIError{
		Provider: "airclaude_subscription",
		Status:   403,
		Code:     "pre_consume_token_quota_failed",
		Message:  "token quota is not enough",
	})
	if credits.Error() != i18n.M.ProviderErrInsufficientCredits {
		t.Errorf("quota error = %q, want credit guidance", credits.Error())
	}

	noModel := explainError(&provider.APIError{
		Provider: "airclaude_subscription",
		Status:   503,
		Code:     "model_not_found",
	})
	if noModel.Error() != i18n.M.ProviderErrModelUnavailable {
		t.Errorf("model-not-found error = %q, want model guidance", noModel.Error())
	}

	streamFailure := explainError(&provider.APIError{
		Provider: "airclaude_subscription",
		Code:     "new_api_error",
		Message:  "provider-specific prose",
	})
	if streamFailure.Error() != i18n.M.ProviderErrRequestFailed {
		t.Errorf("unknown structured stream error = %q, want generic guidance", streamFailure.Error())
	}

	for _, status := range []int{400, 404, 422, 429, 500, 502, 503, 504, 529} {
		got := explainError(&provider.APIError{Provider: "p", Status: status})
		if got.Error() == "" || got.Error() == (&provider.APIError{Provider: "p", Status: status}).Error() {
			t.Errorf("status %d should map to a localized message, got %q", status, got.Error())
		}
	}

	jsonBody := explainError(&provider.APIError{Provider: "deepseek", Status: 400, Body: `{"error":{"message":"This model's maximum context length is 65536 tokens.","type":"invalid_request_error"}}`})
	if !strings.Contains(jsonBody.Error(), i18n.M.ProviderErrBadRequest) || !strings.Contains(jsonBody.Error(), "maximum context length") {
		t.Errorf("400 should append the provider reason from a JSON body, got %q", jsonBody.Error())
	}

	rawBody := explainError(&provider.APIError{Provider: "deepseek", Status: 422, Body: "some unparseable detail"})
	if !strings.Contains(rawBody.Error(), "some unparseable detail") {
		t.Errorf("422 should fall back to the raw body, got %q", rawBody.Error())
	}

	noLeak := explainError(&provider.APIError{Provider: "deepseek", Status: 429, Body: `{"error":{"message":"slow down"}}`})
	if noLeak.Error() != i18n.M.ProviderErrRateLimited {
		t.Errorf("429 body must not leak into the message, got %q", noLeak.Error())
	}

	interrupted := explainError(&provider.StreamInterruptedError{Err: io.ErrUnexpectedEOF})
	if !strings.Contains(interrupted.Error(), "model stream interrupted") || !strings.Contains(interrupted.Error(), "continue") {
		t.Errorf("stream interruption should be actionable, got %q", interrupted.Error())
	}

	disconnected := explainError(io.ErrUnexpectedEOF)
	if !strings.Contains(disconnected.Error(), "model stream disconnected") || !strings.Contains(disconnected.Error(), "retry") {
		t.Errorf("connection reset should be actionable, got %q", disconnected.Error())
	}

	plain := errors.New("some other failure")
	if explainError(plain) != plain {
		t.Error("unknown errors should pass through unchanged")
	}
}
