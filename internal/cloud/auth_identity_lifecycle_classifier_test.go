package cloud

import (
	"fmt"
	"testing"
)

func TestIsSessionTerminalError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "authentication required", err: &Error{Code: ErrorAuthRequired}, want: true},
		{name: "refresh token invalid", err: &Error{Code: ErrorCode("REFRESH_TOKEN_INVALID")}, want: true},
		{name: "wrapped account disabled", err: fmt.Errorf("request failed: %w", &Error{Code: ErrorCode("ACCOUNT_DISABLED")}), want: true},
		{name: "retryable transport error", err: fmt.Errorf("connection reset")},
		{name: "nil", err: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsSessionTerminalError(tt.err); got != tt.want {
				t.Fatalf("IsSessionTerminalError(%v) = %v, want %v", tt.err, got, tt.want)
			}
			if got := isSessionTerminalError(tt.err); got != tt.want {
				t.Fatalf("internal terminal classifier(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
