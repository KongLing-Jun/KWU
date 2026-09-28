package scripts

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCacheImpactScriptCoversExpertPromptFiles(t *testing.T) {
	body := readCacheImpactScript(t)
	blocks := regexp.MustCompile(`(?s)case "\$file" in(.*?)esac`).FindAllStringSubmatch(body, -1)
	if len(blocks) < 2 {
		t.Fatalf("expected at least two changed-file case blocks, got %d", len(blocks))
	}
	if !strings.Contains(blocks[0][1], "internal/expert/*") {
		t.Fatalf("cache-sensitive case block does not include internal/expert/*:\n%s", blocks[0][1])
	}
	if !strings.Contains(blocks[1][1], "internal/expert/*") {
		t.Fatalf("system-prompt-sensitive case block does not include internal/expert/*:\n%s", blocks[1][1])
	}
}

func TestCacheImpactScriptRequiresPromptReviewMetadata(t *testing.T) {
	body := readCacheImpactScript(t)
	for _, want := range []string{
		`require_field "Cache-impact"`,
		`require_field "Cache-guard"`,
		`require_review_field "System-prompt-review"`,
		`System-prompt-review: <reviewer/approval note>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("check-cache-impact.sh missing %q", want)
		}
	}
}

func readCacheImpactScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("check-cache-impact.sh")
	if err != nil {
		t.Fatalf("read check-cache-impact.sh: %v", err)
	}
	return string(data)
}
