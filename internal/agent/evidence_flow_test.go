package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/instruction"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

// scriptedProvider replays a distinct chunk set per Stream call, so a multi-turn
// Run() sees tool calls on turn 1 and a plain final answer on turn 2.
type scriptedProvider struct {
	name         string
	turns        [][]provider.Chunk
	call         int
	requests     []provider.Request
	beforeStream func(int)
}

func (s *scriptedProvider) Name() string { return s.name }

func (s *scriptedProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	s.requests = append(s.requests, req)
	i := s.call
	if s.beforeStream != nil {
		s.beforeStream(i)
	}
	if i >= len(s.turns) {
		i = len(s.turns) - 1
	}
	s.call++
	ch := make(chan provider.Chunk, len(s.turns[i]))
	for _, c := range s.turns[i] {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func toolCallChunk(id, name, args string) provider.Chunk {
	return provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: id, Name: name, Arguments: args}}
}

func toolResult(s *Session, name string) string {
	for _, m := range s.Messages {
		if m.Role == provider.RoleTool && m.Name == name {
			return m.Content
		}
	}
	return ""
}

func lastToolResult(s *Session, name string) string {
	var result string
	for _, m := range s.Messages {
		if m.Role == provider.RoleTool && m.Name == name {
			result = m.Content
		}
	}
	return result
}

func toolResults(s *Session, name string) []string {
	var results []string
	for _, m := range s.Messages {
		if m.Role == provider.RoleTool && m.Name == name {
			results = append(results, m.Content)
		}
	}
	return results
}

func sessionHasUserMessageContaining(s *Session, needle string) bool {
	for _, m := range s.Messages {
		if m.Role == provider.RoleUser && strings.Contains(m.Content, needle) {
			return true
		}
	}
	return false
}

type readinessAuditSink struct {
	events []evidence.ReadinessAudit
}

func (s *readinessAuditSink) Emit(event.Event) {}

func (s *readinessAuditSink) RecordReadinessAudit(a evidence.ReadinessAudit) {
	s.events = append(s.events, a)
}

// TestEvidenceFlowEndToEnd drives a full Run(): turn 1 runs bash then signs the
// step off citing that exact command; complete_step must see the host receipt
// recorded earlier in the same batch and report it host-verified.
func TestEvidenceFlowEndToEnd(t *testing.T) {
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "bash", `{"command":"go test ./..."}`),
			toolCallChunk("c2", "complete_step", `{
				"step":"Run the suite",
				"result":"tests pass",
				"evidence":[{"kind":"verification","summary":"go test ./... passed","command":"go test ./..."}]
			}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "run the suite and sign the step off"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := toolResult(a.session, "complete_step"); !strings.Contains(got, "host-verified 1") {
		t.Fatalf("complete_step result = %q, want it host-verified from the bash receipt", got)
	}
}

func TestEvidenceFlowEnforcesProjectChecksAfterWrite(t *testing.T) {
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			toolCallChunk("c2", "bash", `{"command":"go test ./..."}`),
			toolCallChunk("c3", "complete_step", `{
				"step":"Edit code",
				"result":"changed.go updated",
				"evidence":[{"kind":"diff","summary":"updated code","paths":["changed.go"]}]
			}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{
		ProjectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
	}, event.Discard)
	if err := a.Run(context.Background(), "edit and verify"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := toolResult(a.session, "complete_step")
	if !strings.Contains(got, "project checks 1") {
		t.Fatalf("complete_step result = %q, want project check verified from same batch", got)
	}
}

func TestFinalReadinessAllowsFinalAnswerWithoutWriter(t *testing.T) {
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("")
	a := New(prov, tool.NewRegistry(), session, Options{
		ProjectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
	}, event.Discard)

	if err := a.Run(context.Background(), "inspect only"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.call != 1 {
		t.Fatalf("provider calls = %d, want 1", prov.call)
	}
	if !sessionHasTypedMessage(session, "done", provider.MessageKindFinalAnswer) {
		t.Fatalf("successful final answer was not typed: %#v", session.Messages)
	}
}

func TestPendingSteerLeavesPriorCandidateOrdinary(t *testing.T) {
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "candidate before steer"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "answer after steer"}, {Type: provider.ChunkDone}},
	}}
	session := NewSession("")
	var a *Agent
	prov.beforeStream = func(call int) {
		if call == 0 {
			a.Steer("include the requested detail")
		}
	}
	a = New(prov, tool.NewRegistry(), session, Options{}, event.Discard)

	if err := a.Run(context.Background(), "answer me"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sessionHasTypedMessage(session, "candidate before steer", provider.MessageKindFinalAnswer) {
		t.Fatal("candidate before pending steer was marked final")
	}
	if !sessionHasTypedMessage(session, "answer after steer", provider.MessageKindFinalAnswer) {
		t.Fatalf("answer after steer was not marked final: %#v", session.Messages)
	}
}

func TestFinalReadinessAllowsWriterWithoutChecksOrTodos(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	if err := a.Run(context.Background(), "simple edit"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.call != 2 {
		t.Fatalf("provider calls = %d, want 2", prov.call)
	}
}

func TestFinalReadinessAuditSkipsWhenGateDoesNotApply(t *testing.T) {
	t.Run("no writer", func(t *testing.T) {
		prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
			{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
		}}
		sink := &readinessAuditSink{}
		a := New(prov, tool.NewRegistry(), NewSession(""), Options{
			ProjectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
		}, sink)

		if err := a.Run(context.Background(), "inspect only"); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(sink.events) != 0 {
			t.Fatalf("readiness audit events = %d, want 0: %+v", len(sink.events), sink.events)
		}
	})

	t.Run("writer without checks or todo", func(t *testing.T) {
		reg := tool.NewRegistry()
		reg.Add(fakeTool{name: "write_file", readOnly: false})
		prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
			{
				toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
				{Type: provider.ChunkDone},
			},
			{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
		}}
		sink := &readinessAuditSink{}
		a := New(prov, reg, NewSession(""), Options{}, sink)

		if err := a.Run(context.Background(), "simple edit"); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(sink.events) != 0 {
			t.Fatalf("readiness audit events = %d, want 0: %+v", len(sink.events), sink.events)
		}
	})
}

func TestFinalReadinessBlocksUntilProjectCheckRunsAfterWriter(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(fakeTool{name: "bash", readOnly: false})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature"}, {Type: provider.ChunkDone}},
		{
			toolCallChunk("c2", "bash", `{"command":"go test ./..."}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "verified done"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{
		ProjectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
	}, event.Discard)

	if err := a.Run(context.Background(), "edit and finish"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.call != 4 {
		t.Fatalf("provider calls = %d, want final answer to be retried after readiness block", prov.call)
	}
	if !sessionHasUserMessageContaining(a.session, "final-answer readiness") {
		t.Fatal("missing synthetic readiness retry message")
	}
	if got := lastToolResult(a.session, "bash"); !strings.Contains(got, "bash done") {
		t.Fatalf("bash tool result = %q, want command rerun after block", got)
	}
}

func TestFinalReadinessRejectedCandidateRemainsTypedModelContextAndEmitsRetraction(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(fakeTool{name: "bash", readOnly: false})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature candidate"}, {Type: provider.ChunkDone}},
		{
			toolCallChunk("c2", "bash", `{"command":"go test ./..."}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "accepted final"}, {Type: provider.ChunkDone}},
	}}
	sink := &recordSink{}
	session := NewSession("")
	a := New(prov, reg, session, Options{
		ProjectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
	}, sink)

	if err := a.Run(context.Background(), "edit and finish"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	candidateIndex := -1
	acceptedIndex := -1
	for i, message := range session.Messages {
		switch message.Content {
		case "premature candidate":
			candidateIndex = i
			if message.MessageKind != provider.MessageKindReadinessRejectedCandidate {
				t.Fatalf("candidate kind = %q, want typed readiness rejection", message.MessageKind)
			}
		case "accepted final":
			acceptedIndex = i
			if message.MessageKind != provider.MessageKindFinalAnswer {
				t.Fatalf("accepted final kind = %q, want final answer", message.MessageKind)
			}
		}
	}
	if candidateIndex < 0 || acceptedIndex <= candidateIndex+1 {
		t.Fatalf("candidate/final indexes = %d/%d in %#v", candidateIndex, acceptedIndex, session.Messages)
	}
	retry := session.Messages[candidateIndex+1]
	if retry.Role != provider.RoleUser || !strings.Contains(retry.Content, "Host final-answer readiness check failed") {
		t.Fatalf("message after candidate = %#v, want existing readiness retry", retry)
	}
	for _, want := range []string{"retracted from the user-visible record", "complete, standalone final answer", "substantive changes", "validation", "limitations", "receipt/todo bookkeeping"} {
		if !strings.Contains(retry.Content, want) {
			t.Fatalf("readiness retry %q missing standalone delivery contract %q", retry.Content, want)
		}
	}
	if len(prov.requests) < 3 || !requestHasTypedMessage(prov.requests[2], "premature candidate", provider.MessageKindReadinessRejectedCandidate) {
		t.Fatalf("third model request lost typed rejected candidate: %#v", prov.requests)
	}

	events := sink.kinds(event.FinalCandidateRejected)
	if len(events) != 1 {
		t.Fatalf("candidate retraction events = %d, want 1", len(events))
	}
	allEvents := sink.kinds(event.Notice)
	if len(allEvents) == 0 || !strings.Contains(allEvents[0].Text, "final-answer readiness blocked") {
		t.Fatalf("readiness notice missing after retraction: %#v", allEvents)
	}
	sink.mu.Lock()
	retractionIndex, noticeIndex := -1, -1
	for i, emitted := range sink.evs {
		if emitted.Kind == event.FinalCandidateRejected && retractionIndex < 0 {
			retractionIndex = i
		}
		if emitted.Kind == event.Notice && strings.Contains(emitted.Text, "final-answer readiness blocked") && noticeIndex < 0 {
			noticeIndex = i
		}
	}
	sink.mu.Unlock()
	if retractionIndex < 0 || noticeIndex <= retractionIndex {
		t.Fatalf("retraction/notice event order = %d/%d, want retraction first", retractionIndex, noticeIndex)
	}

	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := session.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), `"message_kind":"readiness_rejected_candidate"`) {
		t.Fatalf("raw session lost rejected candidate type:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"message_kind":"final_answer"`) {
		t.Fatalf("raw session lost accepted final-answer type:\n%s", raw)
	}
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if !sessionHasTypedMessage(loaded, "premature candidate", provider.MessageKindReadinessRejectedCandidate) {
		t.Fatalf("loaded session lost typed candidate: %#v", loaded.Messages)
	}
	if !sessionHasTypedMessage(loaded, "accepted final", provider.MessageKindFinalAnswer) {
		t.Fatalf("loaded session lost typed final answer: %#v", loaded.Messages)
	}
}

func requestHasTypedMessage(req provider.Request, content string, kind provider.MessageKind) bool {
	for _, message := range req.Messages {
		if message.Content == content && message.MessageKind == kind {
			return true
		}
	}
	return false
}

func sessionHasTypedMessage(session *Session, content string, kind provider.MessageKind) bool {
	for _, message := range session.Messages {
		if message.Content == content && message.MessageKind == kind {
			return true
		}
	}
	return false
}

func TestFinalReadinessAuditRecordsBlockAndRecovery(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(fakeTool{name: "bash", readOnly: false})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature"}, {Type: provider.ChunkDone}},
		{
			toolCallChunk("c2", "bash", `{"command":"go test ./..."}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "verified done"}, {Type: provider.ChunkDone}},
	}}
	sink := &readinessAuditSink{}
	a := New(prov, reg, NewSession(""), Options{
		ProjectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
	}, sink)

	if err := a.Run(context.Background(), "edit and finish"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("readiness audit events = %d, want 2: %+v", len(sink.events), sink.events)
	}
	blocked := sink.events[0]
	if blocked.Result != evidence.ReadinessBlocked || blocked.MissingProjectChecks != 1 || blocked.CommandMismatchMissing != 1 {
		t.Fatalf("blocked audit = %+v, want missing project check command", blocked)
	}
	recovered := sink.events[1]
	if recovered.Result != evidence.ReadinessAllowed || !recovered.Recovered {
		t.Fatalf("recovery audit = %+v, want allowed recovered", recovered)
	}
}

func TestFinalReadinessRejectsProjectCheckBeforeWriter(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "bash", `{"command":"go test ./..."}`),
			toolCallChunk("c2", "write_file", `{"path":"changed.go","content":"package main"}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature"}, {Type: provider.ChunkDone}},
		{
			toolCallChunk("c3", "bash", `{"command":"go test ./..."}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "verified done"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{
		ProjectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
	}, event.Discard)

	if err := a.Run(context.Background(), "verify before edit, then finish"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.call != 4 {
		t.Fatalf("provider calls = %d, want pre-write check rejected and retried", prov.call)
	}
}

func TestFinalReadinessRequiresCompleteStepAfterWriterWhenTodoSeen(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(todoWrite)
	reg.Add(completeStep)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			toolCallChunk("c2", "todo_write", `{"todos":[{"content":"Edit code","status":"in_progress"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature"}, {Type: provider.ChunkDone}},
		{
			toolCallChunk("c3", "complete_step", `{
				"step":"Edit code",
				"result":"changed.go updated",
				"evidence":[{"kind":"diff","summary":"updated code","paths":["changed.go"]}]
			}`),
			toolCallChunk("c4", "todo_write", `{"todos":[{"content":"Edit code","status":"completed"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "signed off done"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	if err := a.Run(context.Background(), "edit with todo and finish"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.call != 4 {
		t.Fatalf("provider calls = %d, want final answer to wait for complete_step", prov.call)
	}
	if got := lastToolResult(a.session, "complete_step"); !strings.Contains(got, "signed off") {
		t.Fatalf("complete_step result = %q, want successful sign-off", got)
	}
}

func TestFinalReadinessStopsAfterRepeatedBlocks(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(todoWrite)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			toolCallChunk("c2", "todo_write", `{"todos":[{"content":"Edit code","status":"in_progress"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature 1"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "premature 2"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "premature 3"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	err := a.Run(context.Background(), "edit with todo and never sign off")
	if err == nil {
		t.Fatal("expected repeated readiness blocks to stop the run")
	}
	if !strings.Contains(err.Error(), "final-answer readiness") {
		t.Fatalf("error = %v, want final-answer readiness", err)
	}
	if prov.call != 4 {
		t.Fatalf("provider calls = %d, want three blocked final answers after writer turn", prov.call)
	}
}

func TestTodoWriteOnlyTurnMayEndWithIncompleteTodos(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[{"content":"Draft plan","status":"in_progress"},{"content":"Implement","status":"pending"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "here is the task list"}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	if err := a.Run(context.Background(), "create a todo list only"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.call != 2 {
		t.Fatalf("provider calls = %d, want 2 without readiness retry", prov.call)
	}
	if got := lastToolResult(a.session, "todo_write"); !strings.Contains(got, "Todos updated") {
		t.Fatalf("todo_write result = %q, want successful todo update", got)
	}
}

func TestReadOnlyContextAndTodoTurnMayEndWithIncompleteTodos(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	reg.Add(todoWrite)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "read_file", `{"path":"README.md"}`),
			toolCallChunk("c2", "todo_write", `{"todos":[{"content":"Draft plan","status":"in_progress"},{"content":"Implement","status":"pending"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "I reviewed the context and wrote the list."}, {Type: provider.ChunkDone}},
	}}
	a := New(prov, reg, NewSession(""), Options{}, event.Discard)

	if err := a.Run(context.Background(), "read context and only draft a todo list"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.call != 2 {
		t.Fatalf("provider calls = %d, want 2 without readiness retry", prov.call)
	}
	if got := lastToolResult(a.session, "todo_write"); !strings.Contains(got, "Todos updated") {
		t.Fatalf("todo_write result = %q, want successful todo update", got)
	}
}

func TestFinalReadinessAuditRecordsTerminalError(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(todoWrite)
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "write_file", `{"path":"changed.go","content":"package main"}`),
			toolCallChunk("c2", "todo_write", `{"todos":[{"content":"Edit code","status":"in_progress"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature 1"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "premature 2"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "premature 3"}, {Type: provider.ChunkDone}},
	}}
	sink := &readinessAuditSink{}
	a := New(prov, reg, NewSession(""), Options{}, sink)

	err := a.Run(context.Background(), "edit with todo and never sign off")
	if err == nil {
		t.Fatal("expected repeated readiness blocks to stop the run")
	}
	if len(sink.events) != 3 {
		t.Fatalf("readiness audit events = %d, want 3: %+v", len(sink.events), sink.events)
	}
	last := sink.events[len(sink.events)-1]
	if last.Result != evidence.ReadinessErrored || last.IncompleteTodos == 0 {
		t.Fatalf("terminal audit = %+v, want errored with incomplete todos", last)
	}
}

// TestEvidenceFlowRejectsUncitedCommand proves the loop rejects a sign-off whose
// cited command was never run: bash ran "go test", complete_step cites "go vet".
func TestEvidenceFlowRejectsUncitedCommand(t *testing.T) {
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "bash", `{"command":"go test ./..."}`),
			toolCallChunk("c2", "complete_step", `{
				"step":"Vet the tree",
				"result":"vet is clean",
				"evidence":[{"kind":"verification","summary":"go vet passed","command":"go vet ./..."}]
			}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "vet the tree and sign off"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := toolResult(a.session, "complete_step")
	if !strings.Contains(got, "has no matching successful receipt") {
		t.Fatalf("complete_step result = %q, want the uncited command rejected", got)
	}
	if strings.Contains(got, "host-verified") {
		t.Fatalf("uncited command should not verify, got %q", got)
	}
}

func TestEvidenceFlowRejectsStepMissingFromTodoWrite(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[{"content":"Add parser","status":"in_progress"}]}`),
			toolCallChunk("c2", "complete_step", `{
				"step":"Ship parser",
				"result":"step is complete",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c3", "complete_step", `{
				"step":"Add parser",
				"result":"parser added",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c4", "todo_write", `{"todos":[{"content":"Add parser","status":"completed"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "update todos then sign off the wrong step"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := toolResult(a.session, "complete_step")
	if !strings.Contains(got, "matching todo_write item") {
		t.Fatalf("complete_step result = %q, want todo-backed rejection", got)
	}
}

func TestEvidenceFlowAcceptsTodoCompletionAfterCompleteStep(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[{"content":"Add parser","status":"in_progress"}]}`),
			toolCallChunk("c2", "complete_step", `{
				"step":"Add parser",
				"result":"parser added",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c3", "todo_write", `{"todos":[{"content":"Add parser","status":"completed"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "complete the todo with a sign-off first"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := lastToolResult(a.session, "todo_write"); !strings.Contains(got, "Todos updated") {
		t.Fatalf("final todo_write result = %q, want update accepted", got)
	}
}

func TestEvidenceFlowRejectsTodoCompletionWithoutCompleteStep(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[{"content":"Add parser","status":"in_progress"}]}`),
			toolCallChunk("c2", "todo_write", `{"todos":[{"content":"Add parser","status":"completed"}]}`),
			toolCallChunk("c3", "complete_step", `{
				"step":"Add parser",
				"result":"parser added",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c4", "todo_write", `{"todos":[{"content":"Add parser","status":"completed"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "complete the todo without a sign-off"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	results := toolResults(a.session, "todo_write")
	if len(results) < 2 {
		t.Fatalf("todo_write results = %v, want the rejected completion result", results)
	}
	got := results[1]
	if !strings.Contains(got, "complete_step") {
		t.Fatalf("todo_write result = %q, want completion rejected until complete_step", got)
	}
}

func TestEvidenceFlowRecoversTodoCompletionAfterFailedCompleteStepWithProgress(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[{"content":"Run project script","status":"in_progress"}]}`),
			toolCallChunk("c2", "bash", `{"command":"python \"script.py\""}`),
			toolCallChunk("c3", "complete_step", `{
				"step":"Run project script",
				"result":"script ran",
				"evidence":[{"kind":"verification","summary":"script completed","command":"python other.py"}]
			}`),
			toolCallChunk("c4", "todo_write", `{"todos":[{"content":"Run project script","status":"completed"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "recover after a failed complete_step"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	stepResult := lastToolResult(a.session, "complete_step")
	if !strings.Contains(stepResult, "no matching successful receipt") {
		t.Fatalf("complete_step result = %q, want the sign-off attempt to fail first", stepResult)
	}
	if !strings.Contains(stepResult, `python \"script.py\"`) {
		t.Fatalf("complete_step result = %q, want the self-correction hint to include the real command", stepResult)
	}
	if strings.Contains(stepResult, "todo_write") {
		t.Fatalf("complete_step result = %q, want command hints without todo tool noise", stepResult)
	}
	if got := lastToolResult(a.session, "todo_write"); !strings.Contains(got, "1 completed") {
		t.Fatalf("todo_write result = %q, want completion recovery accepted", got)
	}
}

func TestEvidenceFlowRecoversAfterBatchTodoCompletionRejection(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[
				{"content":"Port entity imports","status":"in_progress"},
				{"content":"Run build and tests","status":"pending"}
			]}`),
			toolCallChunk("c2", "todo_write", `{"todos":[
				{"content":"Port entity imports","status":"completed"},
				{"content":"Run build and tests","status":"completed"}
			]}`),
			{Type: provider.ChunkDone},
		},
		{
			toolCallChunk("c3", "complete_step", `{
				"step":"Port entity imports",
				"result":"entity imports ported",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c4", "complete_step", `{
				"step":"Run build and tests",
				"result":"build and tests ran",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c5", "todo_write", `{"todos":[
				{"content":"Port entity imports","status":"completed"},
				{"content":"Run build and tests","status":"completed"}
			]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "recover from a rejected batch todo update"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	stepResults := toolResults(a.session, "complete_step")
	if len(stepResults) < 2 {
		t.Fatalf("complete_step results = %v, want two sign-offs", stepResults)
	}
	if got := stepResults[1]; !strings.Contains(got, "signed off") {
		t.Fatalf("pending todo complete_step result = %q, want successful sign-off", got)
	}
	if got := lastToolResult(a.session, "todo_write"); !strings.Contains(got, "2 completed") {
		t.Fatalf("final todo_write result = %q, want all todos completed", got)
	}
}

func TestEvidenceFlowL02AtomicRejectionThenHostAdvancesCanonicalTodos(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("t1", "todo_write", `{"todos":[{"content":"S1","status":"in_progress"},{"content":"S2","status":"pending"},{"content":"S3","status":"pending"}]}`),
			toolCallChunk("b1", "bash", `{"command":"go test ./internal/..."}`),
			toolCallChunk("f1", "complete_step", `{"step":"S1","result":"done","evidence":[{"kind":"verification","summary":"tests passed","command":"go test ./wrong/..."}]}`),
			toolCallChunk("t2", "todo_write", `{"todos":[{"content":"S1","status":"completed"},{"content":"S2","status":"completed"},{"content":"S3","status":"in_progress"}]}`),
			{Type: provider.ChunkDone},
		},
		{
			toolCallChunk("s2", "complete_step", `{"step":"S2","result":"done","evidence":[{"kind":"manual","summary":"checked"}]}`),
			toolCallChunk("s1", "complete_step", `{"step":"S1","result":"done","evidence":[{"kind":"manual","summary":"checked"}]}`),
			{Type: provider.ChunkDone},
		},
		{
			toolCallChunk("s3", "complete_step", `{"step":"S3","result":"done","evidence":[{"kind":"manual","summary":"checked"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	var a *Agent
	var afterRecovery []evidence.TodoItem
	prov.beforeStream = func(call int) {
		if call == 2 && a != nil {
			afterRecovery = a.CanonicalTodoState()
		}
	}
	a = New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "replay L02 todo recovery"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	results := toolResults(a.session, "todo_write")
	if len(results) != 2 {
		t.Fatalf("model todo_write calls = %d, want baseline plus rejected batch only: %v", len(results), results)
	}
	rejected := results[1]
	for _, want := range []string{"complete update was not applied", `1 "S1"=in_progress`, `2 "S2"=pending`, `Missing successful complete_step receipt(s): 2 "S2"`} {
		if !strings.Contains(rejected, want) {
			t.Fatalf("L02 rejection %q missing %q", rejected, want)
		}
	}
	if len(afterRecovery) != 3 || afterRecovery[0].Status != "completed" || afterRecovery[1].Status != "completed" || afterRecovery[2].Status != "in_progress" {
		t.Fatalf("canonical state after S2 then S1 sign-off = %+v, want [completed completed in_progress]", afterRecovery)
	}
}

func TestEvidenceFlowFailedCompleteStepDoesNotAuthorizeTodoCompletion(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[{"content":"Add parser","status":"in_progress"}]}`),
			toolCallChunk("c2", "complete_step", `{
				"step":"Ship parser",
				"result":"parser shipped",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c3", "todo_write", `{"todos":[{"content":"Add parser","status":"completed"}]}`),
			toolCallChunk("c4", "complete_step", `{
				"step":"Add parser",
				"result":"parser added",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c5", "todo_write", `{"todos":[{"content":"Add parser","status":"completed"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "attempt completion after a failed sign-off"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	results := toolResults(a.session, "todo_write")
	if len(results) < 2 {
		t.Fatalf("todo_write results = %v, want the rejected completion result", results)
	}
	got := results[1]
	if !strings.Contains(got, "complete_step") {
		t.Fatalf("todo_write result = %q, want failed complete_step not to authorize completion", got)
	}
}

func TestEvidenceFlowRejectsReplacedTodoAfterNumericCompleteStep(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[{"content":"Add parser","status":"in_progress"}]}`),
			toolCallChunk("c2", "complete_step", `{
				"step":"1",
				"result":"parser added",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c3", "todo_write", `{"todos":[{"content":"Ship parser","status":"completed"}]}`),
			toolCallChunk("c4", "todo_write", `{"todos":[{"content":"Add parser","status":"completed"}]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "try to reuse a numeric sign-off for another todo"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	results := toolResults(a.session, "todo_write")
	if len(results) < 2 {
		t.Fatalf("todo_write results = %v, want the rejected replacement result", results)
	}
	got := results[1]
	if !strings.Contains(got, "Ship parser") || !strings.Contains(got, "complete_step") {
		t.Fatalf("todo_write result = %q, want replaced todo rejected", got)
	}
}

func TestEvidenceFlowAcceptsReorderedTodoAfterNumericCompleteStep(t *testing.T) {
	todoWrite, ok := tool.LookupBuiltin("todo_write")
	if !ok {
		t.Fatal("todo_write builtin not registered")
	}
	completeStep, ok := tool.LookupBuiltin("complete_step")
	if !ok {
		t.Fatal("complete_step builtin not registered")
	}
	reg := tool.NewRegistry()
	reg.Add(todoWrite)
	reg.Add(completeStep)

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("c1", "todo_write", `{"todos":[
				{"content":"Add parser","status":"in_progress"},
				{"content":"Write tests","status":"pending"}
			]}`),
			toolCallChunk("c2", "complete_step", `{
				"step":"1",
				"result":"parser added",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c3", "todo_write", `{"todos":[
				{"content":"Write tests","status":"pending"},
				{"content":"Add parser","status":"completed"}
			]}`),
			toolCallChunk("c4", "todo_write", `{"todos":[
				{"content":"Write tests","status":"in_progress"},
				{"content":"Add parser","status":"completed"}
			]}`),
			toolCallChunk("c5", "complete_step", `{
				"step":"Write tests",
				"result":"tests written",
				"evidence":[{"kind":"manual","summary":"checked manually"}]
			}`),
			toolCallChunk("c6", "todo_write", `{"todos":[
				{"content":"Write tests","status":"completed"},
				{"content":"Add parser","status":"completed"}
			]}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{}, event.Discard)
	if err := a.Run(context.Background(), "complete the signed todo after reordering it"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := lastToolResult(a.session, "todo_write"); !strings.Contains(got, "Todos updated") {
		t.Fatalf("final todo_write result = %q, want reordered signed todo accepted", got)
	}
}
