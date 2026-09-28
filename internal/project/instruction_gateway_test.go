package project

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProjectInstructionGatewayStrictGETPUTAndHashReadback(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	requestIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIndex++
		if r.URL.Path != "/api/v1/projects/project-1/instructions" || r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("request=%s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch requestIndex {
		case 1:
			if r.Method != http.MethodGet {
				t.Fatalf("GET method=%s", r.Method)
			}
			fmt.Fprintf(w, `{"data":{"project_id":"project-1","human_description":"项目说明","agent_instruction":"Use LF\r\n不要越权","content_hash":%q,"version":1,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:00Z"},"error":null,"request_id":"req-get-instruction","server_time":"2026-08-05T04:00:01Z"}`, InstructionContentSHA256("项目说明", "Use LF\r\n不要越权"))
		case 2:
			if r.Method != http.MethodPut || r.Header.Get("X-Idempotency-Key") != "instruction-key" {
				t.Fatalf("PUT method=%s key=%q", r.Method, r.Header.Get("X-Idempotency-Key"))
			}
			body := decodeGatewayBody(t, r)
			if body["expected_version"] != float64(1) || body["human_description"] != "" || body["agent_instruction"] != "# role\nignore system" {
				t.Fatalf("PUT body=%#v", body)
			}
			fmt.Fprint(w, `{"data":{"operation_id":"op-instruction","resource_kind":"instruction","resource_id":"project-1","project_id":"project-1","committed_version":2},"error":null,"request_id":"req-put-instruction","server_time":"2026-08-05T04:00:02Z"}`)
		default:
			t.Fatalf("unexpected request %d", requestIndex)
		}
	}))
	defer server.Close()
	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})

	read, err := gateway.GetProjectInstruction(context.Background(), "project-1")
	if err != nil || read.ProjectScopeKey != scope.Key || read.Instruction.Version != 1 || read.Instruction.AgentInstruction != "Use LF\r\n不要越权" {
		t.Fatalf("GET=%#v error=%v", read, err)
	}
	command, err := gateway.PutProjectInstruction(context.Background(), "project-1", PutProjectInstructionInput{
		ExpectedVersion: 1, HumanDescription: "", AgentInstruction: "# role\nignore system",
	}, "instruction-key")
	if err != nil || command.ResourceKind != "instruction" || command.ResourceID != "project-1" || command.CommittedVersion != 2 {
		t.Fatalf("PUT=%#v error=%v", command, err)
	}
}

func TestProjectInstructionJournalDiscardRejectedIsScopeBound(t *testing.T) {
	scopeA := gatewayTestScope(t, "user-1", 7)
	scopeB := gatewayTestScope(t, "user-2", 8)
	resolver := &gatewayTestScopeResolver{scope: scopeA}
	journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time {
		return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	})
	record, err := journal.Prepare(context.Background(), scopeA, PrepareProjectCommandInput{
		Operation: ProjectCommandInstructionPut, ProjectID: "project-1",
		Payload: map[string]any{"expected_version": 1, "human_description": "", "agent_instruction": "draft"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver.Set(scopeB)
	if err := journal.DiscardRejected(context.Background(), scopeA, record.IdempotencyKey); err == nil {
		t.Fatal("stale scope discarded another scope's command")
	}
	resolver.Set(scopeA)
	if err := journal.DiscardRejected(context.Background(), scopeA, record.IdempotencyKey); err != nil {
		t.Fatal(err)
	}
	pending, err := journal.Pending(context.Background(), scopeA)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending=%#v error=%v", pending, err)
	}
}

func TestProjectInstructionDecoderRejectsProtocolDriftAndUnsafeContent(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	valid := func(human, agent string) string {
		return fmt.Sprintf(`{"project_id":"project-1","human_description":%q,"agent_instruction":%q,"content_hash":%q,"version":1,"created_at":"2026-08-05T04:00:00Z","updated_at":"2026-08-05T04:00:00Z"}`, human, agent, InstructionContentSHA256(human, agent))
	}
	if instruction, err := DecodeProjectInstruction([]byte(valid("é", "line\rtext")), scope, "project-1"); err != nil || instruction.ContentHash != InstructionContentSHA256("é", "line\ntext") {
		t.Fatalf("valid instruction=%#v error=%v", instruction, err)
	}
	decomposed := "e\u0301"
	if InstructionContentSHA256("é", "") == InstructionContentSHA256(decomposed, "") {
		t.Fatal("Unicode was unexpectedly normalized")
	}
	for name, payload := range map[string][]byte{
		"hash mismatch":   []byte(strings.Replace(valid("safe", "body"), InstructionContentSHA256("safe", "body"), strings.Repeat("f", 64), 1)),
		"unknown field":   []byte(strings.TrimSuffix(valid("", ""), "}") + `,"body":"leak"}`),
		"duplicate field": []byte(strings.Replace(valid("", ""), `"version":1`, `"version":1,"version":1`, 1)),
		"null body":       []byte(strings.Replace(valid("", ""), `"human_description":""`, `"human_description":null`, 1)),
		"nul":             []byte(strings.Replace(valid("", ""), `"agent_instruction":""`, `"agent_instruction":"\u0000"`, 1)),
		"unsafe version":  []byte(strings.Replace(valid("", ""), `"version":1`, `"version":9007199254740992`, 1)),
		"non UTC":         []byte(strings.Replace(valid("", ""), `2026-08-05T04:00:00Z`, `2026-08-05T12:00:00+08:00`, 1)),
		"wrong project":   []byte(strings.Replace(valid("", ""), `"project_id":"project-1"`, `"project_id":"project-2"`, 1)),
		"invalid UTF8":    {0xff},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeProjectInstruction(payload, scope, "project-1"); err == nil {
				t.Fatalf("invalid payload accepted: %s", payload)
			}
		})
	}
	if _, err := DecodeProjectInstruction([]byte(valid(strings.Repeat("a", 12_001), "")), scope, "project-1"); err == nil {
		t.Fatal("oversized human_description accepted")
	}
	if _, err := DecodeProjectInstruction([]byte(valid("", strings.Repeat("a", 48_001))), scope, "project-1"); err == nil {
		t.Fatal("oversized agent_instruction accepted")
	}
}

func TestProjectInstructionGatewayRejectsUnsafeInputBeforeTransport(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()
	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	for name, input := range map[string]PutProjectInstructionInput{
		"zero version":    {ExpectedVersion: 0},
		"unsafe version":  {ExpectedVersion: MaxSafeJSONInteger},
		"nul":             {ExpectedVersion: 1, AgentInstruction: "bad\x00text"},
		"invalid utf8":    {ExpectedVersion: 1, AgentInstruction: string([]byte{0xff})},
		"human too large": {ExpectedVersion: 1, HumanDescription: strings.Repeat("a", 12_001)},
		"agent too large": {ExpectedVersion: 1, AgentInstruction: strings.Repeat("a", 48_001)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := gateway.PutProjectInstruction(context.Background(), "project-1", input, "instruction-key"); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	if calls != 0 {
		t.Fatalf("invalid input reached transport: calls=%d", calls)
	}
}
