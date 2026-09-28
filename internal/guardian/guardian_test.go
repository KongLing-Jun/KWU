package guardian

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type scriptedProvider struct {
	mu        sync.Mutex
	responses []scriptedResponse
	requests  []provider.Request
}

type guardianTestDispatcher struct {
	mu    sync.Mutex
	calls int
	reply string
}

type guardianTerminalDispatcher struct{ err error }

func (d guardianTerminalDispatcher) Stream(ctx context.Context, base provider.Provider, request provider.Request) (<-chan provider.Chunk, error) {
	chunks, _, err := d.StreamWithTerminalResult(ctx, base, request)
	return chunks, err
}

func (d guardianTerminalDispatcher) StreamWithTerminalResult(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	chunks := make(chan provider.Chunk)
	close(chunks)
	terminal := make(chan error, 1)
	terminal <- d.err
	close(terminal)
	return chunks, terminal, nil
}

func (d *guardianTestDispatcher) Stream(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, error) {
	d.mu.Lock()
	d.calls++
	d.mu.Unlock()
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: d.reply}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (d *guardianTestDispatcher) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

func TestGuardianUsesDistinctPrimaryAndCompactionDispatchers(t *testing.T) {
	base := &scriptedProvider{}
	main := &guardianTestDispatcher{reply: `{"risk_level":"low","user_authorization":"high","outcome":"allow","rationale":"ok"}`}
	compaction := &guardianTestDispatcher{reply: "guardian summary"}
	gs := NewSessionWithDispatchers(base, tool.NewRegistry(), PolicyPrompt(), "guardian-test", 0, nil, event.Discard, main, compaction)
	parent := agent.NewSession("sys")
	if _, _, err := gs.Review(context.Background(), "read_file", json.RawMessage(`{"path":"x"}`), parent); err != nil {
		t.Fatal(err)
	}
	if main.count() != 1 || len(base.requestsSnapshot()) != 0 {
		t.Fatalf("main/base calls = %d/%d", main.count(), len(base.requestsSnapshot()))
	}
	gs.sess.Messages = []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: strings.Repeat("old ", 500)},
		{Role: provider.RoleAssistant, Content: "old answer"},
		{Role: provider.RoleUser, Content: "next"},
		{Role: provider.RoleAssistant, Content: "next answer"},
		{Role: provider.RoleUser, Content: "latest"},
		{Role: provider.RoleAssistant, Content: "latest answer"},
	}
	if err := gs.agent.CompactNow(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if compaction.count() != 1 || main.count() != 1 {
		t.Fatalf("compaction/main calls = %d/%d", compaction.count(), main.count())
	}
}

func TestGuardianPropagatesInternalCompactionTerminalFailure(t *testing.T) {
	base := &scriptedProvider{}
	main := &guardianTestDispatcher{reply: `{"risk_level":"low","user_authorization":"high","outcome":"allow","rationale":"ok"}`}
	compaction := guardianTerminalDispatcher{err: agent.ErrMainStreamDispatcherCloseTimeout}
	gs := NewSessionWithDispatchers(base, tool.NewRegistry(), PolicyPrompt(), "guardian-test", 0, nil, event.Discard, main, compaction)
	gs.reviewCount = compactEvery - 1
	gs.sess.Messages = []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: strings.Repeat("old ", 500)},
		{Role: provider.RoleAssistant, Content: "old answer"},
		{Role: provider.RoleUser, Content: "middle"},
		{Role: provider.RoleAssistant, Content: "middle answer"},
		{Role: provider.RoleUser, Content: "recent"},
		{Role: provider.RoleAssistant, Content: "recent answer"},
	}

	allow, _, err := gs.Review(context.Background(), "write_file", json.RawMessage(`{"file_path":"x"}`), agent.NewSession("parent"))
	if allow || !errors.Is(err, agent.ErrMainStreamDispatcherCloseTimeout) {
		t.Fatalf("guardian Review = allow %v err %v", allow, err)
	}
}

type scriptedResponse struct {
	text  string
	usage *provider.Usage
}

func (p *scriptedProvider) Name() string { return "guardian-test" }

func (p *scriptedProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	resp := scriptedResponse{text: `{"risk_level":"low","user_authorization":"high","outcome":"allow","rationale":"ok"}`}
	if len(p.responses) > 0 {
		resp = p.responses[0]
		p.responses = p.responses[1:]
	}
	p.mu.Unlock()

	ch := make(chan provider.Chunk, 3)
	if resp.text != "" {
		ch <- provider.Chunk{Type: provider.ChunkText, Text: resp.text}
	}
	if resp.usage != nil {
		ch <- provider.Chunk{Type: provider.ChunkUsage, Usage: resp.usage}
	}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (p *scriptedProvider) requestsSnapshot() []provider.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]provider.Request(nil), p.requests...)
}

type captureSink struct {
	mu     sync.Mutex
	events []event.Event
}

func (s *captureSink) Emit(e event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

func (s *captureSink) guardianEvents() []event.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []event.Event
	for _, e := range s.events {
		if e.Kind == event.GuardianAssessment {
			out = append(out, e)
		}
	}
	return out
}

func TestParseAssessmentEnforcesCriticalDeny(t *testing.T) {
	a, err := ParseAssessment(`{"risk_level":"critical","user_authorization":"high","outcome":"allow","rationale":"delete prod secrets"}`)
	if err != nil {
		t.Fatalf("ParseAssessment error: %v", err)
	}
	if a.Outcome != "deny" {
		t.Fatalf("critical outcome = %q, want deny", a.Outcome)
	}
}

func TestParseAssessmentRejectsUnknownEnum(t *testing.T) {
	if _, err := ParseAssessment(`{"risk_level":"spicy","user_authorization":"high","outcome":"allow","rationale":"x"}`); err == nil {
		t.Fatal("ParseAssessment accepted unknown risk_level")
	}
}

func TestTranscriptRenderKeepsFirstAndLastUserAnchors(t *testing.T) {
	entries := []TranscriptEntry{{Kind: "user", Text: "first task"}}
	for i := 0; i < maxRecentEntries+5; i++ {
		entries = append(entries, TranscriptEntry{Kind: "assistant", Text: "assistant detail"})
	}
	entries = append(entries, TranscriptEntry{Kind: "user", Text: "latest instruction"})
	rendered := FormatTranscript(entries)
	if !strings.Contains(rendered, "first task") || !strings.Contains(rendered, "latest instruction") {
		t.Fatalf("rendered transcript lost user anchors:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Some conversation entries were omitted.") {
		t.Fatalf("rendered transcript should mention omissions:\n%s", rendered)
	}
}

func TestGuardianSaveLoadRestoresCursorForDeltaTranscript(t *testing.T) {
	prov := &scriptedProvider{responses: []scriptedResponse{
		{text: `{"risk_level":"low","user_authorization":"high","outcome":"allow","rationale":"first ok"}`},
		{text: `{"risk_level":"low","user_authorization":"high","outcome":"allow","rationale":"second ok"}`},
	}}
	sink := &captureSink{}
	gs := NewSession(prov, tool.NewRegistry(), PolicyPrompt(), "guardian-test", 0, nil, sink)
	parent := agent.NewSession("sys")
	parent.Add(provider.Message{Role: provider.RoleUser, Content: "first user request"})

	if allow, _, err := gs.Review(context.Background(), "write_file", json.RawMessage(`{"file_path":"a.txt"}`), parent); err != nil || !allow {
		t.Fatalf("first Review = allow %v err %v, want allow nil", allow, err)
	}
	path := filepath.Join(t.TempDir(), "session.guardian.jsonl")
	if err := gs.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if data, err := os.ReadFile(cursorPathForGuardianPath(path)); err != nil || !strings.Contains(string(data), `"EntryCount":1`) {
		t.Fatalf("cursor sidecar = %q err %v, want EntryCount 1", data, err)
	}

	loaded := NewSession(prov, tool.NewRegistry(), PolicyPrompt(), "guardian-test", 0, nil, sink)
	if err := loaded.Load(path); err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.cursor.EntryCount != 1 {
		t.Fatalf("loaded cursor = %+v, want EntryCount 1", loaded.cursor)
	}
	parent.Add(provider.Message{Role: provider.RoleUser, Content: "second user request"})
	if allow, _, err := loaded.Review(context.Background(), "write_file", json.RawMessage(`{"file_path":"b.txt"}`), parent); err != nil || !allow {
		t.Fatalf("second Review = allow %v err %v, want allow nil", allow, err)
	}

	reqs := prov.requestsSnapshot()
	if len(reqs) < 2 {
		t.Fatalf("requests = %d, want >= 2", len(reqs))
	}
	var delta string
	for _, req := range reqs {
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "TRANSCRIPT DELTA") && strings.Contains(m.Content, "second user request") {
				delta = m.Content
				break
			}
		}
		if delta != "" {
			break
		}
	}
	if delta == "" {
		t.Fatalf("second request did not include a delta transcript")
	}
	if !strings.Contains(delta, "second user request") {
		t.Fatalf("delta transcript missing new parent entry:\n%s", delta)
	}
	if strings.Contains(delta, "first user request") {
		t.Fatalf("delta transcript repeated old parent entry:\n%s", delta)
	}
}

func TestGuardianUsageDoesNotLeakAcrossReviews(t *testing.T) {
	prov := &scriptedProvider{responses: []scriptedResponse{
		{
			text:  `{"risk_level":"low","user_authorization":"high","outcome":"allow","rationale":"first ok"}`,
			usage: &provider.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
		},
		{text: `{"risk_level":"low","user_authorization":"high","outcome":"allow","rationale":"second ok"}`},
	}}
	sink := &captureSink{}
	gs := NewSession(prov, tool.NewRegistry(), PolicyPrompt(), "guardian-test", 0, nil, sink)
	parent := agent.NewSession("sys")
	parent.Add(provider.Message{Role: provider.RoleUser, Content: "do it"})

	if allow, _, err := gs.Review(context.Background(), "write_file", json.RawMessage(`{"file_path":"a.txt"}`), parent); err != nil || !allow {
		t.Fatalf("first Review = allow %v err %v, want allow nil", allow, err)
	}
	if allow, _, err := gs.Review(context.Background(), "write_file", json.RawMessage(`{"file_path":"b.txt"}`), parent); err != nil || !allow {
		t.Fatalf("second Review = allow %v err %v, want allow nil", allow, err)
	}

	events := sink.guardianEvents()
	if len(events) != 2 {
		t.Fatalf("guardian events = %d, want 2", len(events))
	}
	if events[0].Guardian.Usage == nil || events[0].Guardian.Usage.TotalTokens != 12 {
		t.Fatalf("first usage = %+v, want total 12", events[0].Guardian.Usage)
	}
	if events[1].Guardian.Usage != nil {
		t.Fatalf("second usage leaked from first review: %+v", events[1].Guardian.Usage)
	}
}
