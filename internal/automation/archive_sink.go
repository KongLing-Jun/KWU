package automation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
)

// ArchiveEventMeta identifies which automation run an archived event belongs to.
type ArchiveEventMeta struct {
	TaskID string
	RunID  string
}

// ArchivedEvent is one JSONL record in events.jsonl.
type ArchivedEvent struct {
	Time   time.Time       `json:"time"`
	TaskID string          `json:"taskId"`
	RunID  string          `json:"runId"`
	Kind   event.Kind      `json:"kind"`
	Level  event.Level     `json:"level"`
	Text   string          `json:"text,omitempty"`
	Tool   *event.Tool     `json:"tool,omitempty"`
	Usage  *provider.Usage `json:"usage,omitempty"`
	Error  string          `json:"error,omitempty"`
	Status RunStatus       `json:"status,omitempty"`
	Event  event.Event     `json:"event"`
}

// ArchiveSink serializes typed events to events.jsonl and optionally forwards them.
type ArchiveSink struct {
	mu      sync.Mutex
	enc     *json.Encoder
	file    *os.File
	forward event.Sink
	meta    ArchiveEventMeta
	now     func() time.Time
	err     error
	closed  bool
	last    string
}

func NewArchiveSink(path string, meta ArchiveEventMeta, forward event.Sink) (*ArchiveSink, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("archive events: create dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("archive events: open: %w", err)
	}
	return &ArchiveSink{
		enc:     json.NewEncoder(f),
		file:    f,
		forward: forward,
		meta:    meta,
		now:     time.Now,
	}, nil
}

func (s *ArchiveSink) Emit(ev event.Event) {
	if s == nil {
		return
	}
	if s.write(ev, "") && s.forward != nil {
		s.forward.Emit(ev)
	}
}

func (s *ArchiveSink) EmitRunStatus(status RunStatus) error {
	if s == nil {
		return nil
	}
	ev := event.Event{
		Kind:  event.Notice,
		Level: event.LevelInfo,
		Text:  "automation run " + string(status),
	}
	if s.write(ev, status) && s.forward != nil {
		s.forward.Emit(ev)
	}
	return s.Err()
}

func (s *ArchiveSink) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *ArchiveSink) LastMessage() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func (s *ArchiveSink) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.err
	}
	if err := s.file.Close(); err != nil {
		s.recordErrLocked(fmt.Errorf("archive events: close: %w", err))
	}
	s.closed = true
	return s.err
}

func (s *ArchiveSink) write(ev event.Event, status RunStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		s.recordErrLocked(fmt.Errorf("archive events: write after close"))
		return false
	}
	if err := s.enc.Encode(s.record(ev, status)); err != nil {
		s.recordErrLocked(fmt.Errorf("archive events: write: %w", err))
		return false
	}
	if ev.Kind == event.Message && strings.TrimSpace(ev.Text) != "" {
		s.last = ev.Text
	}
	return true
}

func (s *ArchiveSink) record(ev event.Event, status RunStatus) ArchivedEvent {
	stored := ev
	stored.Err = nil
	stored.Tool = redactedArchivedTool(ev.Tool)
	return ArchivedEvent{
		Time:   s.now().UTC(),
		TaskID: s.meta.TaskID,
		RunID:  s.meta.RunID,
		Kind:   ev.Kind,
		Level:  ev.Level,
		Text:   ev.Text,
		Tool:   archivedTool(ev),
		Usage:  ev.Usage,
		Error:  archivedError(ev),
		Status: status,
		Event:  stored,
	}
}

func (s *ArchiveSink) recordErrLocked(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

func archivedTool(ev event.Event) *event.Tool {
	switch ev.Kind {
	case event.ToolDispatch, event.ToolResult, event.ToolProgress:
		tool := redactedArchivedTool(ev.Tool)
		return &tool
	default:
		return nil
	}
}

func redactedArchivedTool(tool event.Tool) event.Tool {
	tool.Args = ""
	tool.Output = ""
	tool.Err = ""
	tool.Diff = ""
	return tool
}

func archivedError(ev event.Event) string {
	if ev.Err != nil {
		return ev.Err.Error()
	}
	if ev.Tool.Err != "" {
		return "tool execution failed"
	}
	return ""
}
