package agent

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/provider"
)

func TestLoadSessionPageReadsTailWithoutSplittingToolGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paged.jsonl")
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "old"},
		{Role: provider.RoleAssistant, Content: "old answer"},
		{Role: provider.RoleUser, Content: "new"},
		{Role: provider.RoleAssistant, Content: "call", ToolCalls: []provider.ToolCall{{ID: "call-1", Name: "read"}}},
		{Role: provider.RoleTool, ToolCallID: "call-1", Content: "result"},
		{Role: provider.RoleAssistant, Content: "done"},
	}
	if err := NewSessionFromMessages(messages).Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	tail, before, more, generation, err := LoadSessionPage(path, 0, 2, "")
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(tail) != 3 || tail[0].Content != "call" || !more || before <= 0 || generation == "" {
		t.Fatalf("tail = %+v before=%d more=%v generation=%q", tail, before, more, generation)
	}
	older, before, more, nextGeneration, err := LoadSessionPage(path, before, 2, generation)
	if err != nil {
		t.Fatalf("older: %v", err)
	}
	if len(older) != 2 || older[0].Content != "old answer" || !more || nextGeneration != generation {
		t.Fatalf("older = %+v more=%v generation=%q", older, more, nextGeneration)
	}
	oldest, _, more, _, err := LoadSessionPage(path, before, 2, generation)
	if err != nil || len(oldest) != 1 || oldest[0].Content != "old" || more {
		t.Fatalf("oldest = %+v more=%v err=%v", oldest, more, err)
	}
}

func TestLoadSessionPageRejectsStaleGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rewritten.jsonl")
	session := NewSessionFromMessages([]provider.Message{{Role: provider.RoleUser, Content: "first"}})
	if err := session.Save(path); err != nil {
		t.Fatalf("save first: %v", err)
	}
	_, _, _, generation, err := LoadSessionPage(path, 0, 1, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "changed"})
	if err := session.Save(path); err != nil {
		t.Fatalf("save changed: %v", err)
	}
	_, _, _, _, err = LoadSessionPage(path, 1, 1, generation)
	if !errors.Is(err, ErrSessionGenerationChanged) {
		t.Fatalf("stale cursor error = %v", err)
	}
}

func TestLoadSessionPageSupportsMessageLargerThanReadBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.jsonl")
	content := strings.Repeat("large-output-", 100_000)
	if err := NewSessionFromMessages([]provider.Message{{Role: provider.RoleAssistant, Content: content}}).Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	messages, _, more, _, err := LoadSessionPage(path, 0, 1, "")
	if err != nil || more || len(messages) != 1 || messages[0].Content != content {
		t.Fatalf("large page len=%d more=%v err=%v", len(messages), more, err)
	}
}
