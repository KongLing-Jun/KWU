package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/event"
	"kwu/internal/provider"
)

func TestExtractResultFromMessage(t *testing.T) {
	result := ExtractResult(
		[]event.Event{
			{Kind: event.Message, Text: "first"},
			{Kind: event.Message, Text: "final"},
		},
		[]provider.Message{{Role: provider.RoleAssistant, Content: "history"}},
	)
	if result.Markdown != "final" || result.Summary != "final" {
		t.Fatalf("result = %+v, want final event message", result)
	}
}

func TestExtractResultFallback(t *testing.T) {
	result := ExtractResult(nil, []provider.Message{
		{Role: provider.RoleUser, Content: "question"},
		{Role: provider.RoleAssistant, Content: "answer"},
	})
	if result.Markdown != "answer" || result.Summary != "answer" {
		t.Fatalf("result = %+v, want assistant history", result)
	}
}

func TestWriteEmptyResultExplanation(t *testing.T) {
	result := ExtractResult(nil, nil)
	path := filepath.Join(t.TempDir(), "result.md")
	if err := WriteResult(path, result); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(body), "No final assistant result") {
		t.Fatalf("result.md = %q, want explicit empty result explanation", string(body))
	}
}
