package automation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/event"
	"kwu/internal/fileutil"
	"kwu/internal/provider"
)

const emptyResultMarkdown = "No final assistant result was produced for this automation run.\n"

type Result struct {
	Markdown string
	Summary  string
}

func ExtractResult(events []event.Event, history []provider.Message) Result {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == event.Message && strings.TrimSpace(events[i].Text) != "" {
			return resultFromMarkdown(events[i].Text)
		}
	}
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == provider.RoleAssistant && strings.TrimSpace(history[i].Content) != "" {
			return resultFromMarkdown(history[i].Content)
		}
	}
	return resultFromMarkdown(emptyResultMarkdown)
}

func WriteResult(path string, result Result) error {
	body := strings.TrimSpace(result.Markdown)
	if body == "" {
		body = strings.TrimSpace(emptyResultMarkdown)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write run result: create dir: %w", err)
	}
	if err := fileutil.AtomicWriteFile(path, []byte(body+"\n"), 0o600); err != nil {
		return fmt.Errorf("write run result: %w", err)
	}
	return nil
}

func resultFromMarkdown(markdown string) Result {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		markdown = strings.TrimSpace(emptyResultMarkdown)
	}
	return Result{
		Markdown: markdown,
		Summary:  summarizeResult(markdown),
	}
}

func summarizeResult(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > 160 {
			return string(runes[:157]) + "..."
		}
		return line
	}
	return strings.TrimSpace(emptyResultMarkdown)
}
