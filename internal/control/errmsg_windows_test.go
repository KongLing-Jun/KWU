package control

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestExplainErrorPreservesWindowsFilesystemSharingViolation(t *testing.T) {
	pathErr := &os.PathError{Op: "open", Path: `C:\sessions\scene.jsonl.meta`, Err: syscall.Errno(32)}
	original := fmt.Errorf("save scene session: %w", pathErr)
	got := ExplainError(original)
	if got != original {
		t.Fatalf("ExplainError replaced local persistence error: %v", got)
	}
	message := strings.ToLower(got.Error())
	for _, misleading := range []string{"model stream", "recovery", "retry", "provider", "proxy"} {
		if strings.Contains(message, misleading) {
			t.Fatalf("local persistence error contains %q guidance: %q", misleading, got.Error())
		}
	}
	for _, local := range []string{"save scene session", "open", "scene.jsonl.meta"} {
		if !strings.Contains(message, local) {
			t.Fatalf("local persistence error lost %q: %q", local, got.Error())
		}
	}
}
