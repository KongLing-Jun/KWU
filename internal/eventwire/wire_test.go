package eventwire

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kwu/internal/event"
	"kwu/internal/provider"
)

const retiredArchiveEventKind = event.Kind(18)

func TestToWireRetryingJSON(t *testing.T) {
	w := ToWire(event.Event{Kind: event.Retrying, RetryAttempt: 3, RetryMax: 10})
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{`"kind":"retrying"`, `"retryAttempt":3`, `"retryMax":10`} {
		if !strings.Contains(s, want) {
			t.Fatalf("retrying JSON = %s, want it to contain %s", s, want)
		}
	}
}

func TestToWirePreservesContentFreeDeliveryIdentity(t *testing.T) {
	w := ToWire(event.Event{
		Kind: event.Message, EventID: "e-1", TurnID: "t-1", CandidateID: "c-1", Sequence: 7,
		Text: "must not be used as identity",
	})
	if w.EventID != "e-1" || w.TurnID != "t-1" || w.CandidateID != "c-1" || w.EventSequence != 7 {
		t.Fatalf("delivery identity = %+v", w)
	}
}

func TestToWireTurnDoneMetadataJSON(t *testing.T) {
	w := ToWire(event.Event{
		Kind: event.TurnDone,
		Turn: &provider.TurnMetadata{
			TaskID: "task-1", ModelRef: "provider/glm-5.2", ModelID: "glm-5.2", BillingSource: "subscription_pool",
		},
	})
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	want := `{"kind":"turn_done","turn":{"taskId":"task-1","modelRef":"provider/glm-5.2","modelId":"glm-5.2","billingSource":"subscription_pool"}}`
	if got != want {
		t.Fatalf("turn metadata JSON = %s, want %s", got, want)
	}
}

func TestToWireArtifactDeliveryDeepCopiesContract(t *testing.T) {
	delivery := &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"reports/result.md"}, DeliveredAt: 123}
	w := ToWire(event.Event{Kind: event.TurnDone, ArtifactDelivery: delivery})
	if w.ArtifactDelivery == nil || w.ArtifactDelivery.Version != provider.ArtifactDeliveryVersion || len(w.ArtifactDelivery.Paths) != 1 || w.ArtifactDelivery.Paths[0] != "reports/result.md" {
		t.Fatalf("wire artifact delivery = %#v", w.ArtifactDelivery)
	}
	delivery.Paths[0] = "mutated-source.md"
	w.ArtifactDelivery.Paths[0] = "mutated-wire.md"
	if delivery.Paths[0] != "mutated-source.md" {
		t.Fatalf("wire mutation changed event delivery: %#v", delivery)
	}
	if fresh := ToWire(event.Event{Kind: event.TurnDone, ArtifactDelivery: delivery}); fresh.ArtifactDelivery == nil || fresh.ArtifactDelivery.Paths[0] != "mutated-source.md" {
		t.Fatalf("fresh wire delivery = %#v", fresh.ArtifactDelivery)
	}
	b, err := json.Marshal(ToWire(event.Event{
		Kind: event.TurnDone, ArtifactDelivery: &provider.ArtifactDelivery{Version: 1, Paths: []string{"report.md"}, DeliveredAt: 123},
		OriginSessionPath: `C:\private\session.jsonl`, OriginWorkspaceRoot: `C:\private\workspace`,
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `{"kind":"turn_done","artifactDelivery":{"version":1,"paths":["report.md"],"deliveredAt":123}}`; got != want {
		t.Fatalf("artifact delivery JSON = %s, want %s", got, want)
	}
}

func TestToWirePreservesInvalidArtifactDeliveryAsSuppressionMarker(t *testing.T) {
	tests := []struct {
		name     string
		delivery *provider.ArtifactDelivery
		wantJSON string
	}{
		{
			name: "unknown version",
			delivery: &provider.ArtifactDelivery{
				Version:     provider.ArtifactDeliveryVersion + 1,
				Paths:       []string{"future-report.pdf"},
				DeliveredAt: 123,
			},
			wantJSON: `{"kind":"turn_done","artifactDelivery":{"version":2,"paths":["future-report.pdf"],"deliveredAt":123}}`,
		},
		{
			name:     "missing paths",
			delivery: &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, DeliveredAt: 123},
			wantJSON: `{"kind":"turn_done","artifactDelivery":{"version":1,"paths":null,"deliveredAt":123}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ToWire(event.Event{Kind: event.TurnDone, ArtifactDelivery: tt.delivery})
			if w.ArtifactDelivery == nil {
				t.Fatal("wire artifact delivery is nil; the frontend could fall back to legacy inference")
			}
			b, err := json.Marshal(w)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if got := string(b); got != tt.wantJSON {
				t.Fatalf("artifact delivery JSON = %s, want %s", got, tt.wantJSON)
			}
		})
	}
}

func TestToWireSubagentStatusChangedJSONIsContentFree(t *testing.T) {
	w := ToWire(event.Event{
		Kind:           event.SubagentStatusChanged,
		SubagentRef:    "sa_status",
		SubagentStatus: "completed",
	})
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `{"kind":"subagent_status_changed","subagentRef":"sa_status","subagentStatus":"completed"}`; got != want {
		t.Fatalf("status JSON = %s, want exactly %s", got, want)
	}
}

func TestToWireReasoningDeltaUsesTextField(t *testing.T) {
	b, err := json.Marshal(ToWire(event.Event{Kind: event.Reasoning, Text: "delta"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(b), `{"kind":"reasoning","text":"delta"}`; got != want {
		t.Fatalf("reasoning JSON = %s, want exactly %s", got, want)
	}
}

func TestKindNamesComplete(t *testing.T) {
	for k := event.Kind(0); k < event.KindCount; k++ {
		if k == retiredArchiveEventKind {
			continue
		}
		if ToWire(event.Event{Kind: k}).Kind == "" {
			t.Fatalf("kind %d has no wire name", k)
		}
	}
	if got := ToWire(event.Event{Kind: retiredArchiveEventKind}).Kind; got != "" {
		t.Fatalf("retired archive kind has wire name %q", got)
	}
}

func TestDesktopWireEventKindTypeCoversSharedKinds(t *testing.T) {
	ts := readDesktopTypes(t)
	for k := event.Kind(0); k < event.KindCount; k++ {
		if k == retiredArchiveEventKind {
			continue
		}
		kind := ToWire(event.Event{Kind: k}).Kind
		if !strings.Contains(ts, `"`+kind+`"`) {
			t.Fatalf("desktop WireEvent EventKind is missing %q", kind)
		}
	}
}

func TestDesktopWireEventTypeCoversSharedPayloadFields(t *testing.T) {
	ts := readDesktopTypes(t)
	for _, want := range []string{
		"eventId?: string;",
		"turnId?: string;",
		"candidateId?: string;",
		"eventSequence?: number;",
		"retryAttempt?: number;",
		"retryMax?: number;",
		"memoryCitations?: MemoryCitation[];",
		"export interface MemoryCitation",
		"cacheDiagnostics?: WireCacheDiagnostics;",
		"export interface WireCacheDiagnostics",
		"prefixHash: string;",
		"prefixChanged: boolean;",
		"prefixChangeReasons?: string[];",
		"toolSchemaTokens: number;",
		"taskTree?: WireTaskTree;",
		"export interface WireTaskTree",
		"export interface WireTaskTreeNode",
		"treeId: string;",
		"nodeId?: string;",
		"outputDelta?: string;",
		"taskTreeId?: string;",
		"subagentRef?: string;",
		"subagentStatus?: string;",
		"sourceToolId?: string;",
		"turn?: TurnMetadata;",
		"export interface TurnMetadata",
	} {
		if !strings.Contains(ts, want) {
			t.Fatalf("desktop WireEvent types are missing %q", want)
		}
	}
}

func TestToWireTaskTreePayloadJSON(t *testing.T) {
	w := ToWire(event.Event{
		Kind: event.TaskNodeOutput,
		TaskTree: &event.TaskTreePayload{
			TreeID:      "tree-1",
			NodeID:      "node-1",
			Status:      event.TaskTreeStatusRunning,
			Title:       "Run checks",
			ParentID:    "root",
			OutputDelta: "go test ./internal/event\n",
		},
	})
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		`"kind":"task_node_output"`,
		`"taskTree":{"treeId":"tree-1"`,
		`"nodeId":"node-1"`,
		`"status":"running"`,
		`"title":"Run checks"`,
		`"parentId":"root"`,
		`"outputDelta":"go test ./internal/event\n"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("task tree JSON = %s, want it to contain %s", s, want)
		}
	}
}

func TestToWireMessageMemoryCitations(t *testing.T) {
	w := ToWire(event.Event{
		Kind: event.Message,
		Text: "done",
		MemoryCitations: []provider.MemoryCitation{{
			ID:        "mem-1",
			Source:    "MEMORY.md",
			LineStart: 116,
			LineEnd:   123,
			Note:      "kwu workflow",
			Kind:      "memory_reference",
		}},
	})
	if len(w.MemoryCitations) != 1 {
		t.Fatalf("memory citations = %+v, want one citation", w.MemoryCitations)
	}
	got := w.MemoryCitations[0]
	if got.Source != "MEMORY.md" || got.LineStart != 116 || got.LineEnd != 123 || got.Note != "kwu workflow" {
		t.Fatalf("citation = %+v, want source/line/note preserved", got)
	}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"memoryCitations":[`) {
		t.Fatalf("wire JSON missing memoryCitations: %s", string(b))
	}
}

func TestToWireAssistantParentID(t *testing.T) {
	w := ToWire(event.Event{Kind: event.Message, Text: "child done", ParentID: "task-1", SubagentRef: "child-ref"})
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{`"kind":"message"`, `"text":"child done"`, `"parentId":"task-1"`, `"subagentRef":"child-ref"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("parented message JSON = %s, want it to contain %s", s, want)
		}
	}
}

func readDesktopTypes(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "desktop", "frontend", "src", "lib", "types.ts")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read desktop types: %v", err)
	}
	return string(b)
}

func TestToWireToolPayloadJSON(t *testing.T) {
	w := ToWire(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{
		ID: "call-1", Name: "task", Args: `{"prompt":"x"}`, Output: "ignored",
		Err: "blocked", ReadOnly: true, Truncated: true, DurationMs: 522,
		Partial: true, ParentID: "parent-1", SubagentRef: "child-ref", SourceToolID: "raw-call-1", TaskTreeID: "tree-1", NodeID: "node-1",
		FileDiff: event.FileDiff{Diff: "@@ -1 +1 @@\n-old\n+new\n", Added: 1, Removed: 1},
		Profile:  &event.Profile{Model: "deepseek-pro", Effort: "max"},
	}})
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		`"kind":"tool_dispatch"`, `"id":"call-1"`, `"name":"task"`,
		`"args":"{\"prompt\":\"x\"}"`, `"output":"ignored"`, `"err":"blocked"`,
		`"readOnly":true`, `"truncated":true`, `"durationMs":522`, `"partial":true`,
		`"parentId":"parent-1"`, `"subagentRef":"child-ref"`, `"sourceToolId":"raw-call-1"`, `"taskTreeId":"tree-1"`, `"nodeId":"node-1"`,
		`"diff":"@@ -1 +1 @@\n-old\n+new\n"`,
		`"added":1`, `"removed":1`, `"profile":{"model":"deepseek-pro","effort":"max"}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("tool JSON = %s, want it to contain %s", s, want)
		}
	}
}

func TestWireToolArtifactPathsJSON(t *testing.T) {
	w := Event{Kind: "tool_result", Tool: &Tool{
		Name: "deliver_files", ArtifactPaths: []string{"C:/workspace/report.md", "C:/workspace/chart.png"},
	}}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"artifactPaths":["C:/workspace/report.md","C:/workspace/chart.png"]`) {
		t.Fatalf("tool artifact paths JSON = %s", s)
	}
}

func TestToWireUsagePayloadJSON(t *testing.T) {
	w := ToWire(event.Event{
		Kind: event.Usage,
		Usage: &provider.Usage{
			PromptTokens: 1000, CompletionTokens: 200, TotalTokens: 1200,
			CacheHitTokens: 900, CacheMissTokens: 100, ReasoningTokens: 33,
		},
		Pricing:     &provider.Pricing{CacheHit: 0.02, Input: 1, Output: 2},
		UsageSource: event.UsageSourceTitle,
		CacheDiagnostics: &event.CacheDiagnostics{
			PrefixHash: "p", PrefixChanged: true, PrefixChangeReasons: []string{"log_rewrite"},
			SystemHash: "s", ToolsHash: "t", LogRewriteVersion: 1, ToolSchemaTokens: 42,
			CacheMissTokens: 100, CacheHitTokens: 900,
		},
		SessionHit: 8000, SessionMiss: 2000,
	})
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		`"kind":"usage"`, `"promptTokens":1000`, `"completionTokens":200`, `"totalTokens":1200`,
		`"cacheHitTokens":900`, `"cacheMissTokens":100`, `"reasoningTokens":33`,
		`"source":"title"`, `"sessionCacheHitTokens":8000`, `"sessionCacheMissTokens":2000`,
		`"currency":"¥"`, `"costUsd":`, `"cacheDiagnostics":`, `"prefixHash":"p"`,
		`"prefixChanged":true`, `"prefixChangeReasons":["log_rewrite"]`, `"toolSchemaTokens":42`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("usage JSON = %s, want it to contain %s", s, want)
		}
	}
}

func TestToWireInteractionAndLifecyclePayloads(t *testing.T) {
	tests := []struct {
		name string
		in   event.Event
		want []string
	}{
		{
			name: "approval",
			in:   event.Event{Kind: event.ApprovalRequest, Approval: event.Approval{ID: "a1", Tool: "bash", Subject: "rm"}},
			want: []string{`"kind":"approval_request"`, `"approval":{"id":"a1","tool":"bash","subject":"rm"}`},
		},
		{
			name: "ask",
			in: event.Event{Kind: event.AskRequest, Ask: event.Ask{
				ID: "ask-1",
				Questions: []event.AskQuestion{{
					ID: "q1", Header: "Pick", Prompt: "Choose", Multi: true,
					Options: []event.AskOption{{Label: "A", Description: "Alpha"}, {Label: "B"}},
				}},
			}},
			want: []string{`"kind":"ask_request"`, `"ask":{"id":"ask-1"`, `"header":"Pick"`, `"description":"Alpha"`, `"multi":true`},
		},
		{
			name: "compaction",
			in: event.Event{Kind: event.CompactionDone, Compaction: event.Compaction{
				Trigger: "manual", Messages: 7, Summary: "brief", Archive: "/tmp/archive.jsonl",
			}},
			want: []string{`"kind":"compaction_done"`, `"trigger":"manual"`, `"messages":7`, `"summary":"brief"`, `"archive":"/tmp/archive.jsonl"`},
		},
		{
			name: "turn done error",
			in:   event.Event{Kind: event.TurnDone, Err: errors.New("boom")},
			want: []string{`"kind":"turn_done"`, `"err":"boom"`},
		},
		{
			name: "turn done interrupted",
			in:   event.Event{Kind: event.TurnDone, Interrupted: true},
			want: []string{`"kind":"turn_done"`, `"interrupted":true`},
		},
		{
			name: "final candidate rejected",
			in:   event.Event{Kind: event.FinalCandidateRejected},
			want: []string{`"kind":"final_candidate_rejected"`},
		},
		{
			name: "steer",
			in:   event.Event{Kind: event.Steer, Text: "mid-turn guidance"},
			want: []string{`"kind":"steer"`, `"text":"mid-turn guidance"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(ToWire(tt.in))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(b)
			for _, want := range tt.want {
				if !strings.Contains(s, want) {
					t.Fatalf("%s JSON = %s, want it to contain %s", tt.name, s, want)
				}
			}
		})
	}
}

func TestToWireAuditPayloads(t *testing.T) {
	audit := event.DecisionAudit{
		ActorID:        "actor-1",
		ProfileID:      "profile-1",
		SessionID:      "session-1",
		WorkspaceRoot:  "/repo",
		Tool:           "bash",
		Decision:       "allow",
		SessionGrant:   true,
		PersistedRule:  "Bash(go test:*)",
		MemoryMutation: true,
	}
	tests := []event.Event{
		{Kind: event.ApprovalRequest, Approval: event.Approval{ID: "a1", Tool: "bash", Subject: "go test", Audit: &audit}},
		{Kind: event.AuditEvent, Audit: audit},
	}
	for _, in := range tests {
		b, err := json.Marshal(ToWire(in))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		s := string(b)
		for _, want := range []string{
			`"actor_id":"actor-1"`,
			`"profile_id":"profile-1"`,
			`"session_id":"session-1"`,
			`"workspace_root":"/repo"`,
			`"tool":"bash"`,
			`"decision":"allow"`,
			`"session_grant":true`,
			`"persisted_rule":"Bash(go test:*)"`,
			`"memory_mutation":true`,
		} {
			if !strings.Contains(s, want) {
				t.Fatalf("audit JSON = %s, want it to contain %s", s, want)
			}
		}
	}
}
