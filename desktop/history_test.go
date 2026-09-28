package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

func mustHistoryForTab(t testing.TB, app *App, tabID string) []HistoryMessage {
	t.Helper()
	history, err := app.HistoryForTab(tabID)
	if err != nil {
		t.Fatalf("HistoryForTab(%q): %v", tabID, err)
	}
	return history
}

func TestHistoryProjectsOnlyTypedFinalAnswerMarkers(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind provider.MessageKind
		want bool
	}{
		{name: "legacy ordinary", want: false},
		{name: "accepted final", kind: provider.MessageKindFinalAnswer, want: true},
	} {
		t.Run("controller "+tc.name, func(t *testing.T) {
			got := historyMessages([]provider.Message{{Role: provider.RoleAssistant, Content: tc.name, MessageKind: tc.kind}}, agent.UserPreviewText)
			if len(got) != 1 || got[0].FinalAnswer != tc.want {
				t.Fatalf("history = %+v, want finalAnswer=%v", got, tc.want)
			}
		})
	}

	path := filepath.Join(t.TempDir(), "events.jsonl")
	raw := strings.Join([]string{
		`{"kind":"model.final","content":"legacy ordinary"}`,
		`{"kind":"model.final","content":"accepted final","message_kind":"final_answer"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, ok, err := previewEventSessionMessages(path)
	if err != nil || !ok {
		t.Fatalf("previewEventSessionMessages = (%+v, %v, %v)", got, ok, err)
	}
	if len(got) != 2 || got[0].FinalAnswer || !got[1].FinalAnswer {
		t.Fatalf("preview history = %+v, want only typed record marked final", got)
	}
}

func TestHistoryProjectsInterruptedTurnBoundaryAsNotice(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "inspect this"},
		{Role: provider.RoleAssistant, Content: "working", ToolCalls: []provider.ToolCall{{
			ID: "call-1", Name: "read_file", Arguments: `{"path":"a.txt"}`,
		}}},
		{Role: provider.RoleTool, ToolCallID: "call-1", Name: "read_file", Content: "contents"},
		{Role: provider.RoleUser, Content: agent.TurnInterruptedMessage()},
	}
	history := historyMessages(msgs, agent.UserPreviewText)
	if len(history) != 4 {
		t.Fatalf("history messages = %d, want 4: %+v", len(history), history)
	}
	last := history[len(history)-1]
	if last.Role != "notice" || last.Level != "warn" || last.Content != "本轮已中断" {
		t.Fatalf("interruption projection = %+v", last)
	}
	for _, message := range history {
		if message.Role == "user" && agent.IsTurnInterruptedMessage(message.Content) {
			t.Fatalf("interruption marker leaked as user message: %+v", message)
		}
	}
}

func TestHistoryFiltersTypedReadinessRejectedCandidates(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "create the artifact"},
		{Role: provider.RoleAssistant, Content: "premature candidate", MessageKind: provider.MessageKindReadinessRejectedCandidate},
		{Role: provider.RoleUser, Content: "Host final-answer readiness check failed. Run the required tool calls."},
		{Role: provider.RoleUser, Content: "Host final-answer readiness check: one-time file delivery review. Give a complete final answer."},
		{Role: provider.RoleAssistant, Content: "accepted final"},
	}
	history := historyMessages(msgs, agent.UserPreviewText)
	if len(history) != 2 || history[0].Content != "create the artifact" || history[1].Content != "accepted final" {
		t.Fatalf("history = %+v, want user and accepted final only", history)
	}
	visible, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(visible), "premature candidate") {
		t.Fatalf("copy/export history leaked rejected candidate: %s", visible)
	}
}

func TestPreviewEventSessionFiltersTypedReadinessRejectedCandidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	raw := strings.Join([]string{
		`{"kind":"user.message","text":"create the artifact"}`,
		`{"kind":"model.final","message_kind":"readiness_rejected_candidate","content":"premature candidate"}`,
		`{"kind":"model.final","content":"accepted final"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	history, ok, err := previewEventSessionMessages(path)
	if err != nil || !ok {
		t.Fatalf("previewEventSessionMessages = ok:%v err:%v", ok, err)
	}
	if len(history) != 2 || history[0].Content != "create the artifact" || history[1].Content != "accepted final" {
		t.Fatalf("preview history = %+v, want user and accepted final only", history)
	}
	loadedRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loadedRaw), `"message_kind":"readiness_rejected_candidate"`) || !strings.Contains(string(loadedRaw), "premature candidate") {
		t.Fatalf("raw event session lost typed candidate: %s", loadedRaw)
	}
}

func TestHistoryMessagesIncludeAssistantReasoning(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "expanded prompt"},
		{Role: provider.RoleAssistant, Content: "answer", ReasoningContent: "thinking trace", ToolCalls: []provider.ToolCall{{
			ID: "call_1", Name: "bash", Arguments: `{"command":"pwd"}`,
		}}, MemoryCitations: []provider.MemoryCitation{{
			ID: "mem-1", Source: "Memory v5", Note: "use previous bash failure", Kind: "constraint",
		}}},
		{Role: provider.RoleTool, Name: "bash", ToolCallID: "call_1", Content: "tool output", ReasoningContent: "ignored by frontend filter"},
		{Role: provider.RoleAssistant, ReasoningContent: "tool-call-only thinking"},
	}

	got := historyMessages(msgs, func(content string) string {
		if content != "expanded prompt" {
			t.Fatalf("unexpected user content passed to resolver: %q", content)
		}
		return "display prompt"
	})

	if len(got) != len(msgs) {
		t.Fatalf("history length = %d, want %d", len(got), len(msgs))
	}
	if got[0].Content != "display prompt" {
		t.Fatalf("user display content = %q, want display prompt", got[0].Content)
	}
	if got[0].SubmitText != "expanded prompt" {
		t.Fatalf("user submit text = %q, want expanded prompt", got[0].SubmitText)
	}
	if got[1].Reasoning != "thinking trace" {
		t.Fatalf("assistant reasoning = %q, want thinking trace", got[1].Reasoning)
	}
	if len(got[1].MemoryCitations) != 1 || got[1].MemoryCitations[0].Note != "use previous bash failure" {
		t.Fatalf("assistant memory citations not preserved: %+v", got[1].MemoryCitations)
	}
	if len(got[1].ToolCalls) != 1 || got[1].ToolCalls[0].ID != "call_1" || got[1].ToolCalls[0].Name != "bash" {
		t.Fatalf("assistant tool calls not preserved: %+v", got[1].ToolCalls)
	}
	if !got[1].ToolCalls[0].ArgumentsArchived || got[1].ToolCalls[0].Arguments != "" || got[1].ToolCalls[0].Subject != "pwd" {
		t.Fatalf("assistant tool call was not restored as lightweight metadata: %+v", got[1].ToolCalls[0])
	}
	if got[2].ToolCallID != "call_1" || got[2].ToolName != "bash" || got[2].Content != "" || !got[2].ToolResultArchived {
		t.Fatalf("tool result details not preserved: %+v", got[2])
	}
	if got[2].Reasoning != "" {
		t.Fatalf("non-assistant reasoning should stay hidden, got %q", got[2].Reasoning)
	}
	if got[3].Reasoning != "tool-call-only thinking" {
		t.Fatalf("empty-content assistant reasoning = %q, want tool-call-only thinking", got[3].Reasoning)
	}
}

func TestHistoryMessagesPreserveDeliverFilesPathsWhenOutputIsArchived(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "report.md"), filepath.Join(root, "chart.png")}
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "create files"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "deliver-1", Name: deliverFilesToolName, Arguments: `{"paths":["report.md","chart.png"]}`}}},
		{Role: provider.RoleTool, Name: deliverFilesToolName, ToolCallID: "deliver-1", Content: mustDeliverFilesResultJSON(t, paths...)},
		{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "done"},
	}

	got := historyMessages(msgs, func(content string) string { return content })
	if len(got) != len(msgs) || len(got[1].ToolCalls) != 1 {
		t.Fatalf("history = %+v", got)
	}
	call := got[1].ToolCalls[0]
	if !slices.Equal(call.ArtifactPaths, paths) {
		t.Fatalf("history artifact paths = %v, want %v", call.ArtifactPaths, paths)
	}
	if got[2].Content != "" || !got[2].ToolResultArchived {
		t.Fatalf("deliver_files output was not archived: %+v", got[2])
	}

	cloned := cloneHistoryToolCalls([]HistoryToolCall{call})
	cloned[0].ArtifactPaths[0] = "changed"
	if call.ArtifactPaths[0] != paths[0] {
		t.Fatalf("clone shared artifact paths with source: %v", call.ArtifactPaths)
	}
}

func TestHistoryForTabRestoresPlannerDisplayAfterReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	handoff := strings.Join([]string{
		"# KWU executor handoff",
		"",
		"You are the executor now.",
		"",
		"Original task:",
		"fix the sandbox reload bug",
		"",
		"Planner output:",
		"inspect settings rebuild and preserve planner display",
		"",
		"Executor instructions:",
		"- apply the fix",
	}, "\n")

	sess := agent.NewSession("system")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: handoff})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "executor kept working"})
	ag := agent.New(stubProvider{}, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: ag, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	if err := recordSessionDisplay(dir, path, handoff, "fix the sandbox reload bug"); err != nil {
		t.Fatalf("recordSessionDisplay: %v", err)
	}

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{},
		activeTabID: "planner_tab",
	}
	tab := &WorkspaceTab{ID: "planner_tab", Scope: "global", Ctrl: ctrl, Ready: true, disabledMCP: map[string]ServerView{}}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab

	tab.sink.Emit(event.Event{Kind: event.TurnStarted})
	tab.sink.Emit(event.Event{Kind: event.Phase, Text: "deepseek-v4-pro · planning", Source: event.UsageSourcePlanner})
	tab.sink.Emit(event.Event{Kind: event.Reasoning, Text: "planner thinking\n", Source: event.UsageSourcePlanner})
	tab.sink.Emit(event.Event{Kind: event.Text, Text: "planner visible plan", Source: event.UsageSourcePlanner})
	tab.sink.Emit(event.Event{Kind: event.Message, Text: "planner visible plan", Reasoning: "planner thinking\n", Source: event.UsageSourcePlanner})
	tab.sink.Emit(event.Event{Kind: event.TurnStarted})
	tab.sink.Emit(event.Event{Kind: event.TurnDone})
	waitForAutosaveIdle(t, tab)

	got := mustHistoryForTab(t, app, tab.ID)
	if len(got) != 5 {
		t.Fatalf("history length = %d, want user + planner phase + planner answer + executor answer (plus system skipped later by UI): %+v", len(got), got)
	}
	if got[1].Content != "fix the sandbox reload bug" {
		t.Fatalf("user display content = %q, want original prompt", got[1].Content)
	}
	if got[2].Role != "phase" || !strings.Contains(got[2].Content, "planning") {
		t.Fatalf("planner phase missing after reload: %+v", got)
	}
	if got[3].Role != "assistant" || got[3].Content != "planner visible plan" || got[3].Reasoning != "planner thinking\n" {
		t.Fatalf("planner assistant display missing after reload: %+v", got[3])
	}
	if got[4].Role != "assistant" || got[4].Content != "executor kept working" {
		t.Fatalf("executor answer missing after reload: %+v", got[4])
	}
}

func TestHistoryMessagesArchiveCompletedToolPayloads(t *testing.T) {
	largeArgs := `{"command":"` + strings.Repeat("printf x;", 300) + `"}`
	largeOutput := strings.Repeat("line of output\n", 600)
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			ID: "call_large", Name: "bash", Arguments: largeArgs,
		}}},
		{Role: provider.RoleTool, Name: "bash", ToolCallID: "call_large", Content: largeOutput},
	}

	got := historyMessages(msgs, func(content string) string { return content })
	if len(got) != 2 {
		t.Fatalf("history length = %d, want 2", len(got))
	}
	call := got[0].ToolCalls[0]
	if !call.ArgumentsArchived {
		t.Fatalf("tool arguments were not marked archived: %+v", call)
	}
	if call.Arguments != "" {
		t.Fatalf("archived tool arguments should be omitted from initial history, got %d bytes", len(call.Arguments))
	}
	if call.Subject == "" {
		t.Fatalf("archived tool call should keep a collapsed subject: %+v", call)
	}
	if call.Summary == "" {
		t.Fatalf("archived tool call should keep a collapsed summary: %+v", call)
	}
	result := got[1]
	if !result.ToolResultArchived {
		t.Fatalf("tool result was not marked archived: %+v", result)
	}
	if result.Content != "" {
		t.Fatalf("archived successful tool output should be omitted from initial history, got %d bytes", len(result.Content))
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), largeArgs) || strings.Contains(string(encoded), largeOutput) {
		t.Fatalf("initial history JSON still contains large args/output: %d bytes", len(encoded))
	}
}

func TestToolResultForTabLoadsCompletedExpertTaskBeforeControllerReady(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	tab := &WorkspaceTab{
		ID:            "completed-tab",
		Scope:         "global",
		WorkspaceRoot: globalWorkspaceRoot(),
		Ready:         false,
		disabledMCP:   map[string]ServerView{},
	}
	dir := tabSessionDir(tab)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "completed-expert-team.jsonl")
	args := `{"tasks":[{"member_name":"Research Expert","prompt":"research"},{"member_name":"Review Expert","prompt":"review"}]}`
	output := "Completed 2 parallel tasks:\n-- task-1 --\nresearch answer\n-- task-2 --\nreview answer"
	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "parallel-completed", Name: "parallel_tasks", Arguments: args,
	}}})
	session.Add(provider.Message{Role: provider.RoleTool, Name: "parallel_tasks", ToolCallID: "parallel-completed", Content: output})
	if err := session.Save(path); err != nil {
		t.Fatalf("save completed session: %v", err)
	}
	tab.SessionPath = path

	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	if _, validPath, err := resolveTabSessionPath(tab, path); err != nil {
		t.Fatalf("resolve persisted completed session: %v", err)
	} else if loaded, loadErr := agent.LoadSession(validPath); loadErr != nil {
		t.Fatalf("load persisted completed session: %v", loadErr)
	} else if direct := control.ToolResultFromMessages(loaded.Snapshot(), "parallel-completed"); direct == nil {
		t.Fatalf("direct persisted tool lookup returned nil")
	}

	got := app.ToolResultForTab(tab.ID, "parallel-completed")
	if got == nil || got.Args != args || got.Output != output {
		t.Fatalf("persisted completed tool result = %+v, want full args and output", got)
	}
}

func TestExpertTeamForTabRestoresOwnedMemberBindings(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "expert-team.jsonl")
	writeHistoryTestSession(t, path, "team task")
	ownedRef := saveCompletedHistorySubagent(t, dir, agent.BranchID(path), "task_owned", []provider.Message{{
		Role: provider.RoleAssistant, Content: "owned history",
	}})
	foreignRef := saveCompletedHistorySubagent(t, dir, "other-parent", "task_foreign", []provider.Message{{
		Role: provider.RoleAssistant, Content: "foreign history",
	}})
	meta := agent.ExpertMeta{
		ExpertID: "code-delivery-team", ExpertName: "代码交付专家团", TeamState: agent.ExpertTeamActive,
		Lead: &agent.ExpertTeamMemberMeta{MemberKey: "expert:exp_architect", ExpertID: "exp_architect", Name: "架构顾问", Role: "系统设计", AvatarKey: "architect"},
		Members: []agent.ExpertTeamMemberMeta{
			{MemberKey: "expert:exp_review", ExpertID: "exp_review", Name: "代码审查专家", Role: "实现质量", SubagentRef: ownedRef},
			{MemberKey: "expert:exp_testing", ExpertID: "exp_testing", Name: "测试策略专家", Role: "验证覆盖", SubagentRef: foreignRef},
			{MemberKey: "expert:exp_release", ExpertID: "exp_release", Name: "发布风险顾问", Role: "上线控制"},
		},
	}
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatal(err)
	}
	app, tab := newControllerlessHistoryTab(root, path)
	app.replaceCloudExpertDefinitions([]expert.Definition{{
		ID: "code-delivery-team", Source: expert.SourceOfficial, Mode: expert.ModeTeam,
		Members: []expert.Member{
			{Kind: expert.MemberKindLead, ExpertID: "exp_architect", Name: "架构顾问", AvatarKey: "https://assets.example.com/lead.png"},
			{Kind: expert.MemberKindSub, ExpertID: "exp_review", Name: "代码审查专家", AvatarKey: "https://assets.example.com/review.png"},
			{Kind: expert.MemberKindSub, ExpertID: "exp_testing", Name: "测试策略专家", AvatarKey: "https://assets.example.com/testing.png"},
			{Kind: expert.MemberKindSub, ExpertID: "exp_release", Name: "发布风险顾问", AvatarKey: "https://assets.example.com/release.png"},
		},
	}})

	view, err := app.ExpertTeamForTab(tab.ID)
	if err != nil {
		t.Fatalf("ExpertTeamForTab: %v", err)
	}
	if view == nil || view.State != agent.ExpertTeamActive || view.Lead.Name != "架构顾问" || view.Lead.AvatarKey != "https://assets.example.com/lead.png" || len(view.Members) != 1 {
		t.Fatalf("expert team view = %+v", view)
	}
	if view.Members[0].SubagentRef != ownedRef || view.Members[0].Status != string(agent.SubagentCompleted) {
		t.Fatalf("owned member = %+v, want completed owned ref", view.Members[0])
	}
	if view.Members[0].AvatarKey != "https://assets.example.com/review.png" {
		t.Fatalf("owned member avatar = %q, want current service avatar URL", view.Members[0].AvatarKey)
	}

	meta.TeamState = agent.ExpertTeamArchived
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatal(err)
	}
	view, err = app.ExpertTeamForTab(tab.ID)
	if err != nil || view == nil || view.State != agent.ExpertTeamArchived || view.Members[0].SubagentRef != ownedRef {
		t.Fatalf("archived expert team view = %+v / %v", view, err)
	}

	meta.Members = nil
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatal(err)
	}
	view, err = app.ExpertTeamForTab(tab.ID)
	if err != nil || view == nil || view.Members == nil || len(view.Members) != 0 {
		t.Fatalf("expert team without bound members = %+v / %v, want non-nil empty members", view, err)
	}
}

func TestExpertTeamForTabDoesNotRestoreRewoundMemberBinding(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rewound-expert-team.jsonl")
	writeHistoryTestSession(t, path, "parent history after rewind")
	parentSession := agent.BranchID(path)
	ownedRef := saveCompletedHistorySubagent(t, dir, parentSession, "task-rewound", []provider.Message{{
		Role: provider.RoleAssistant, Content: "preserved child history",
	}})
	artifacts, err := agent.ListSubagentsByParent(dir, parentSession)
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("ListSubagentsByParent before rewind projection: artifacts=%+v err=%v", artifacts, err)
	}
	childSessionBefore, err := os.ReadFile(artifacts[0].SessionPath)
	if err != nil {
		t.Fatal(err)
	}
	childMetaBefore, err := os.ReadFile(artifacts[0].MetaPath)
	if err != nil {
		t.Fatal(err)
	}
	meta := agent.ExpertMeta{
		ExpertID: "code-delivery-team", ExpertName: "代码交付专家团", TeamState: agent.ExpertTeamArchived,
		TeamBindingsReset: true,
		Lead:              &agent.ExpertTeamMemberMeta{MemberKey: "member:0", Name: "架构顾问", Role: "系统设计"},
		Members: []agent.ExpertTeamMemberMeta{{
			MemberKey: "member:1", Name: "代码审查专家", Role: "实现质量", SubagentRef: ownedRef,
		}},
	}
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatal(err)
	}
	app, tab := newControllerlessHistoryTab(root, path)
	loadedSession, err := agent.LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	executor := agent.New(nil, tool.NewRegistry(), loadedSession, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: executor, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	defer ctrl.Close()
	tab.Ctrl = ctrl
	view, err := app.ExpertTeamForTab(tab.ID)
	if err != nil || view == nil || len(view.Members) != 1 {
		t.Fatalf("expert team before binding reset = %+v / %v", view, err)
	}

	meta.Members[0].SubagentRef = ""
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatal(err)
	}
	// App.Rewind invokes this existing refresh after Controller.Rewind succeeds.
	// The expert projection itself is intentionally uncached and re-reads sidecar state.
	app.refreshTabSessionTurns(tab.ID)
	view, err = app.ExpertTeamForTab(tab.ID)
	if err != nil || view == nil || view.State != agent.ExpertTeamArchived || view.Lead.Name != "架构顾问" || len(view.Members) != 0 {
		t.Fatalf("expert team immediately after binding reset = %+v / %v", view, err)
	}

	reloaded, reloadedTab := newControllerlessHistoryTab(root, path)
	view, err = reloaded.ExpertTeamForTab(reloadedTab.ID)
	if err != nil || view == nil || len(view.Members) != 0 {
		t.Fatalf("expert team after tab reload = %+v / %v", view, err)
	}
	artifacts, err = agent.ListSubagentsByParent(dir, parentSession)
	if err != nil || len(artifacts) != 1 || artifacts[0].Ref != ownedRef || artifacts[0].Meta.ParentSession != parentSession {
		t.Fatalf("owned child after binding reset = %+v / %v", artifacts, err)
	}
	childSessionAfter, err := os.ReadFile(artifacts[0].SessionPath)
	if err != nil {
		t.Fatal(err)
	}
	childMetaAfter, err := os.ReadFile(artifacts[0].MetaPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(childSessionAfter) != string(childSessionBefore) || string(childMetaAfter) != string(childMetaBefore) {
		t.Fatal("binding reset rewrote the owned child session or metadata")
	}
}

func TestExpertTeamForTabOrdersMembersByFirstDispatch(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "expert-team-dispatch-order.jsonl")
	writeHistoryTestSession(t, path, "team task")
	parentSession := agent.BranchID(path)
	refs := map[string]string{}
	for _, name := range []string{"later", "tie-a", "zero-a", "earlier", "tie-b", "zero-b"} {
		refs[name] = saveCompletedHistorySubagent(t, dir, parentSession, "task_"+name, []provider.Message{{
			Role: provider.RoleAssistant, Content: name + " history",
		}})
	}
	earlier := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	setHistorySubagentCreatedAt(t, dir, parentSession, refs["earlier"], earlier)
	setHistorySubagentCreatedAt(t, dir, parentSession, refs["later"], earlier.Add(time.Hour))
	setHistorySubagentCreatedAt(t, dir, parentSession, refs["tie-a"], earlier.Add(2*time.Hour))
	setHistorySubagentCreatedAt(t, dir, parentSession, refs["tie-b"], earlier.Add(2*time.Hour))
	setHistorySubagentCreatedAt(t, dir, parentSession, refs["zero-a"], time.Time{})
	setHistorySubagentCreatedAt(t, dir, parentSession, refs["zero-b"], time.Time{})

	configuredOrder := []string{"later", "tie-a", "zero-a", "earlier", "tie-b", "zero-b"}
	members := make([]agent.ExpertTeamMemberMeta, 0, len(configuredOrder))
	for _, name := range configuredOrder {
		members = append(members, agent.ExpertTeamMemberMeta{
			MemberKey:   "member:" + name,
			Name:        name,
			SubagentRef: refs[name],
		})
	}
	meta := agent.ExpertMeta{
		ExpertID: "code-delivery-team", ExpertName: "代码交付专家团", TeamState: agent.ExpertTeamActive,
		Lead:    &agent.ExpertTeamMemberMeta{MemberKey: "member:lead", Name: "lead"},
		Members: members,
	}
	if err := agent.SaveSessionExpert(path, meta); err != nil {
		t.Fatal(err)
	}
	app, tab := newControllerlessHistoryTab(root, path)

	view, err := app.ExpertTeamForTab(tab.ID)
	if err != nil {
		t.Fatalf("ExpertTeamForTab: %v", err)
	}
	got := make([]string, 0, len(view.Members))
	for _, member := range view.Members {
		got = append(got, member.Name)
	}
	want := []string{"earlier", "later", "tie-a", "tie-b", "zero-a", "zero-b"}
	if !slices.Equal(got, want) {
		t.Fatalf("expert member order = %v, want first-dispatch order %v", got, want)
	}
}

func TestSubagentToolResultForTabUsesOwnedChildTranscript(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "child-tool.jsonl")
	writeHistoryTestSession(t, path, "parent")
	const childArgs = `{"path":"child.txt"}`
	const childOutput = "child tool output"
	ref := saveCompletedHistorySubagent(t, dir, agent.BranchID(path), "task_child", []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "child_tool_1", Name: "read_file", Arguments: childArgs}}},
		{Role: provider.RoleTool, ToolCallID: "child_tool_1", Name: "read_file", Content: childOutput},
	})
	foreignRef := saveCompletedHistorySubagent(t, dir, "other-parent", "task_foreign", []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "foreign_tool", Name: "read_file", Arguments: `{}`}}},
		{Role: provider.RoleTool, ToolCallID: "foreign_tool", Name: "read_file", Content: "foreign"},
	})
	app, tab := newControllerlessHistoryTab(root, path)

	got, err := app.SubagentToolResultForTab(tab.ID, ref, "child_tool_1")
	if err != nil || got == nil || got.Args != childArgs || got.Output != childOutput {
		t.Fatalf("SubagentToolResultForTab = %+v / %v", got, err)
	}
	if _, err := app.SubagentToolResultForTab(tab.ID, foreignRef, "foreign_tool"); err == nil || !strings.Contains(err.Error(), "subagent reference not found") {
		t.Fatalf("cross-parent child lookup error = %v", err)
	}
	if _, err := app.SubagentToolResultForTab(tab.ID, ref, "missing_tool"); err == nil || !strings.Contains(err.Error(), "child tool result not found") {
		t.Fatalf("missing child tool lookup error = %v", err)
	}
}

func TestSanitizeSubagentHistoryHidesOnlyExpertIdentityWrapper(t *testing.T) {
	wrapped := `You are the configured expert-team member "Research Expert". Work only in that identity.

Compare the two implementations.`
	messages := []HistoryMessage{
		{Role: string(provider.RoleUser), Content: wrapped},
		{Role: string(provider.RoleAssistant), Content: "comparison"},
		{Role: string(provider.RoleUser), Content: "ordinary delegated prompt"},
	}

	got := sanitizeSubagentHistory(messages)
	if got[0].Content != "Compare the two implementations." {
		t.Fatalf("sanitized expert prompt = %q", got[0].Content)
	}
	if got[1].Content != "comparison" || got[2].Content != "ordinary delegated prompt" {
		t.Fatalf("sanitizer changed ordinary child history: %+v", got)
	}
	if messages[0].Content != wrapped {
		t.Fatalf("sanitizer mutated source messages: %+v", messages)
	}
}

func TestHistoryProjectsTaskWaitStopAsPerTurnLifecycles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent.jsonl")
	const ref = "sa_20260806_010203_000000000_aabbccddeeff"
	messages := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "task-1", Name: "task", Arguments: `{}`}}},
		{Role: provider.RoleTool, ToolCallID: "task-1", Name: "task", Content: "Subagent reference: " + ref + "\nTask run ID: task-11\nTurn status: running"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "wait-1", Name: "wait_task", Arguments: `{}`}}},
		{Role: provider.RoleTool, ToolCallID: "wait-1", Name: "wait_task", Content: `{"subagentRef":"` + ref + `","taskRunId":"task-11","turnStatus":"completed","finalAnswer":"first answer","artifacts":[{"path":"C:\\workspace\\.github\\workflows\\release.yml","size":11,"source":"writer"}]}`},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "task-2", Name: "task", Arguments: `{}`}}},
		{Role: provider.RoleTool, ToolCallID: "task-2", Name: "task", Content: "Subagent reference: " + ref + "\nTask run ID: task-12\nDependencies: task-11\nTurn status: running"},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "stop-2", Name: "stop_task", Arguments: `{}`}}},
		{Role: provider.RoleTool, ToolCallID: "stop-2", Name: "stop_task", Content: `{"subagentRef":"` + ref + `","taskRunId":"task-12","dependsOn":["task-11"],"turnStatus":"interrupted"}`},
	}

	got := historyMessagesForSession(dir, path, messages)
	first := requireHistoryToolCall(t, got, "task-1")
	second := requireHistoryToolCall(t, got, "task-2")
	wait := requireHistoryToolCall(t, got, "wait-1")
	stop := requireHistoryToolCall(t, got, "stop-2")
	if first.SubagentRef != ref || first.TaskLifecycle == nil || first.TaskLifecycle.TurnStatus != agent.SubagentCompleted || first.TaskLifecycle.FinalAnswer != "first answer" {
		t.Fatalf("first task lifecycle = %+v", first)
	}
	if second.SubagentRef != ref || second.TaskLifecycle == nil || second.TaskLifecycle.TurnStatus != agent.SubagentInterrupted {
		t.Fatalf("second task lifecycle = %+v", second)
	}
	if first.TaskLifecycle.TaskRunID != "task-11" || second.TaskLifecycle.TaskRunID != "task-12" || !slices.Equal(second.TaskLifecycle.DependsOn, []string{"task-11"}) {
		t.Fatalf("task run metadata: first=%+v second=%+v", first.TaskLifecycle, second.TaskLifecycle)
	}
	if len(wait.TaskLifecycleReceipts) != 1 || len(stop.TaskLifecycleReceipts) != 1 ||
		!slices.Equal([]string{wait.TaskLifecycleReceipts[0].SubagentRef, stop.TaskLifecycleReceipts[0].SubagentRef}, []string{ref, ref}) {
		t.Fatalf("control receipts: wait=%v stop=%v", wait.TaskLifecycleReceipts, stop.TaskLifecycleReceipts)
	}
	if artifacts := wait.TaskLifecycleReceipts[0].Artifacts; len(artifacts) != 1 || artifacts[0].Source != agent.SubagentArtifactSourceWriter {
		t.Fatalf("writer artifact provenance = %+v", artifacts)
	}
}

func TestHistoryKeepsUnmatchedTaskReceiptForFrontendPaginationMerge(t *testing.T) {
	const ref = "sa_20260806_010203_000000000_aabbccddeeff"
	messages := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "wait-1", Name: "wait_task", Arguments: `{}`}}},
		{Role: provider.RoleTool, ToolCallID: "wait-1", Name: "wait_task", Content: `{"subagentRef":"` + ref + `","turnStatus":"completed","finalAnswer":"page result"}`},
	}
	got := historyMessagesForSession(t.TempDir(), "parent.jsonl", messages)
	wait := requireHistoryToolCall(t, got, "wait-1")
	if len(wait.TaskLifecycleReceipts) != 1 || wait.TaskLifecycleReceipts[0].FinalAnswer != "page result" {
		t.Fatalf("wait lifecycle receipts = %+v", wait.TaskLifecycleReceipts)
	}
	if len(wait.TaskLifecycleRefs) != 0 {
		t.Fatalf("unmatched wait must remain visible, refs = %v", wait.TaskLifecycleRefs)
	}
}

func TestHistoryMessagesKeepToolFileDiffMetadata(t *testing.T) {
	diff := "@@ -27 +27 @@\n-func save():\n+func save_file():\n"
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			ID:        "edit",
			Name:      "edit_file",
			Arguments: `{"path":"settings/settings_IO.gd","old_string":"func save():","new_string":"func save_file():"}`,
			Diff:      diff,
			Added:     1,
			Removed:   1,
		}}},
		{Role: provider.RoleTool, Name: "edit_file", ToolCallID: "edit", Content: "edited settings/settings_IO.gd"},
	}

	got := historyMessages(msgs, func(content string) string { return content })
	call := got[0].ToolCalls[0]
	if call.Diff != diff || call.Added != 1 || call.Removed != 1 {
		t.Fatalf("history tool diff metadata = diff:%q +%d -%d", call.Diff, call.Added, call.Removed)
	}
	if !call.ArgumentsArchived || call.Arguments != "" {
		t.Fatalf("tool arguments should still be archived: %+v", call)
	}
}

func TestHistoryMessagesKeepBoundedToolErrors(t *testing.T) {
	largeError := "error: " + strings.Repeat("permission denied ", 400)
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			ID: "call_error", Name: "bash", Arguments: `{"command":"rm protected"}`,
		}}},
		{Role: provider.RoleTool, Name: "bash", ToolCallID: "call_error", Content: largeError},
	}

	got := historyMessages(msgs, func(content string) string { return content })
	result := got[1]
	if result.ToolResultError == "" {
		t.Fatalf("failed tool result should keep an error preview: %+v", result)
	}
	if result.Content != result.ToolResultError {
		t.Fatalf("tool result content and error preview diverged: content=%q error=%q", result.Content, result.ToolResultError)
	}
	if len(result.Content) >= len(largeError) {
		t.Fatalf("failed tool result preview was not bounded: got %d want < %d", len(result.Content), len(largeError))
	}
	if !strings.HasPrefix(result.Content, "error: permission denied") {
		t.Fatalf("failed tool result preview lost useful prefix: %q", result.Content[:min(len(result.Content), 80)])
	}
	if !result.ToolResultArchived {
		t.Fatalf("bounded failed tool result should still be marked archived for on-demand full data: %+v", result)
	}
}

func TestHistoryMessagesClipToolErrorsAtUTF8Boundary(t *testing.T) {
	largeError := "error: " + strings.Repeat("权限不足", 1000)
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			ID: "call_unicode_error", Name: "bash", Arguments: `{"command":"rm 受保护文件"}`,
		}}},
		{Role: provider.RoleTool, Name: "bash", ToolCallID: "call_unicode_error", Content: largeError},
	}

	got := historyMessages(msgs, func(content string) string { return content })
	result := got[1]
	if result.ToolResultError == "" {
		t.Fatalf("failed tool result should keep an error preview: %+v", result)
	}
	if !utf8.ValidString(result.ToolResultError) {
		t.Fatalf("failed tool result preview is not valid UTF-8: %q", result.ToolResultError)
	}
	if len(result.ToolResultError) >= len(largeError) {
		t.Fatalf("failed tool result preview was not bounded: got %d want < %d", len(result.ToolResultError), len(largeError))
	}
}

func TestHistoryMessagesKeepTodoWriteArguments(t *testing.T) {
	args := `{"todos":[{"content":"A","status":"in_progress"}]}`
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			ID: "todo_1", Name: "todo_write", Arguments: args,
		}}},
		{Role: provider.RoleTool, Name: "todo_write", ToolCallID: "todo_1", Content: "Todos updated"},
	}

	got := historyMessages(msgs, func(content string) string { return content })
	call := got[0].ToolCalls[0]
	if call.ArgumentsArchived {
		t.Fatalf("todo_write arguments must remain available for restored todo panel: %+v", call)
	}
	if call.Arguments != args {
		t.Fatalf("todo_write arguments = %q, want %q", call.Arguments, args)
	}
}

func TestHistoryMessagesPreserveUnaddressableToolPayloads(t *testing.T) {
	args := `{"command":"legacy"}`
	output := "legacy output\n" + strings.Repeat("detail\n", 8)
	msgs := []provider.Message{
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
			Name: "bash", Arguments: args,
		}}},
		{Role: provider.RoleTool, Name: "bash", Content: output},
	}

	got := historyMessages(msgs, func(content string) string { return content })
	call := got[0].ToolCalls[0]
	if call.ArgumentsArchived {
		t.Fatalf("tool call without an id cannot be archived for later lookup: %+v", call)
	}
	if call.Arguments != args {
		t.Fatalf("tool call without an id should keep args, got %q", call.Arguments)
	}
	result := got[1]
	if result.ToolResultArchived {
		t.Fatalf("tool result without an id cannot be archived for later lookup: %+v", result)
	}
	if result.Content != output {
		t.Fatalf("tool result without an id should keep output, got %q", result.Content)
	}
}

func TestPreviewSessionMessagesLoadsWithoutResuming(t *testing.T) {
	dir := t.TempDir()
	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "show history"})
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "answer", ReasoningContent: "saved reasoning"})
	path := filepath.Join(dir, "session.jsonl")
	if err := session.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := previewSessionMessages(dir, path)
	if err != nil {
		t.Fatalf("previewSessionMessages: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("preview history length = %d, want 2", len(got))
	}
	if got[1].Reasoning != "saved reasoning" {
		t.Fatalf("preview reasoning = %q, want saved reasoning", got[1].Reasoning)
	}
}

func TestHistoryForTabDefersCompletedTaskSubagentMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent.jsonl")
	parentSession := agent.BranchID(path)
	taskCallID := "task_call_1"

	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "ask the research expert"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: taskCallID, Name: "task", Arguments: `{"description":"Research Expert"}`,
	}}})
	session.Add(provider.Message{Role: provider.RoleTool, Name: "task", ToolCallID: taskCallID, Content: "research summary"})

	ref := saveCompletedHistorySubagent(t, dir, parentSession, taskCallID, []provider.Message{
		{Role: provider.RoleUser, Content: "subagent prompt"},
		{Role: provider.RoleAssistant, Content: "subagent answer", ReasoningContent: "subagent thinking"},
	})

	ag := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: ag, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	defer ctrl.Close()

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{},
		activeTabID: "tab",
	}
	tab := &WorkspaceTab{ID: "tab", Scope: "global", Ctrl: ctrl, Ready: true, disabledMCP: map[string]ServerView{}}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab

	got := mustHistoryForTab(t, app, tab.ID)
	call := requireHistoryToolCall(t, got, taskCallID)
	if call.SubagentRef != ref {
		t.Fatalf("subagent ref = %q, want %q", call.SubagentRef, ref)
	}
	if len(call.SubagentMessages) != 0 {
		t.Fatalf("subagent messages should be lazy, got %+v", call.SubagentMessages)
	}
	subagent, err := app.SubagentHistoryForTab(tab.ID, ref)
	if err != nil {
		t.Fatalf("SubagentHistoryForTab: %v", err)
	}
	if len(subagent) != 2 || subagent[0].Content != "subagent prompt" || subagent[1].Content != "subagent answer" || subagent[1].Reasoning != "subagent thinking" {
		t.Fatalf("lazy subagent history = %+v", subagent)
	}
}

func TestHistoryPageForTabReturnsCursorWindows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paged.jsonl")
	session := agent.NewSession("")
	for i := 0; i < 450; i++ {
		role := provider.RoleAssistant
		if i%2 == 0 {
			role = provider.RoleUser
		}
		session.Add(provider.Message{Role: role, Content: fmt.Sprintf("message-%03d", i)})
	}
	ag := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: ag, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	defer ctrl.Close()
	app := &App{tabRuntimes: map[string]*WorkspaceTab{}, activeTabID: "tab"}
	app.tabRuntimes["tab"] = &WorkspaceTab{ID: "tab", Ctrl: ctrl, Ready: true, disabledMCP: map[string]ServerView{}}

	first, err := app.HistoryPageForTab("tab", 0, 100, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Messages) != 100 || first.Before != 350 || !first.HasMore ||
		first.Messages[0].Content != "message-350" || first.Messages[99].Content != "message-449" {
		t.Fatalf("first page = before:%d more:%v messages:%d first:%q last:%q",
			first.Before, first.HasMore, len(first.Messages), first.Messages[0].Content, first.Messages[len(first.Messages)-1].Content)
	}
	second, err := app.HistoryPageForTab("tab", first.Before, 100, first.Generation)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second.Messages) != 100 || second.Before != 250 || !second.HasMore ||
		second.Messages[0].Content != "message-250" || second.Messages[99].Content != "message-349" {
		t.Fatalf("second page = before:%d more:%v messages:%d", second.Before, second.HasMore, len(second.Messages))
	}
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "new tail"})
	if _, err := app.HistoryPageForTab("tab", second.Before, 100, second.Generation); !errors.Is(err, agent.ErrSessionGenerationChanged) {
		t.Fatalf("stale generation error = %v", err)
	}
}

func TestPreviewSessionMessagesDefersCompletedTaskSubagentMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent.jsonl")
	parentSession := agent.BranchID(path)
	taskCallID := "task_call_1"

	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "ask the research expert"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: taskCallID, Name: "task", Arguments: `{"description":"Research Expert"}`,
	}}})
	if err := session.Save(path); err != nil {
		t.Fatalf("Save parent: %v", err)
	}
	ref := saveCompletedHistorySubagent(t, dir, parentSession, taskCallID, []provider.Message{
		{Role: provider.RoleUser, Content: "subagent prompt"},
		{Role: provider.RoleAssistant, Content: "subagent answer"},
	})

	got, err := previewSessionMessages(dir, path)
	if err != nil {
		t.Fatalf("previewSessionMessages: %v", err)
	}
	call := requireHistoryToolCall(t, got, taskCallID)
	if call.SubagentRef != ref || len(call.SubagentMessages) != 0 {
		t.Fatalf("preview should expose only lazy subagent ref: %+v", call)
	}
}

func TestHistoryForTabLeavesTaskSubagentEmptyWithoutArtifact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent.jsonl")
	taskCallID := "task_call_1"

	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "ask the research expert"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: taskCallID, Name: "task", Arguments: `{"description":"Research Expert"}`,
	}}})

	ag := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: ag, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	defer ctrl.Close()

	app := &App{tabRuntimes: map[string]*WorkspaceTab{}, activeTabID: "tab"}
	tab := &WorkspaceTab{ID: "tab", Scope: "global", Ctrl: ctrl, Ready: true, disabledMCP: map[string]ServerView{}}
	app.tabRuntimes[tab.ID] = tab

	call := requireHistoryToolCall(t, mustHistoryForTab(t, app, tab.ID), taskCallID)
	if call.SubagentRef != "" || len(call.SubagentMessages) != 0 {
		t.Fatalf("attached missing subagent artifact: %+v", call)
	}
}

func TestHistoryForTabDoesNotAttachSubagentFromOtherParentSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent.jsonl")
	taskCallID := "task_call_1"

	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "ask the research expert"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: taskCallID, Name: "task", Arguments: `{"description":"Research Expert"}`,
	}}})
	saveCompletedHistorySubagent(t, dir, "other-parent", taskCallID, []provider.Message{
		{Role: provider.RoleAssistant, Content: "wrong subagent answer"},
	})

	ag := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: ag, SessionDir: dir, SessionPath: path, Sink: event.Discard})
	defer ctrl.Close()

	app := &App{tabRuntimes: map[string]*WorkspaceTab{}, activeTabID: "tab"}
	tab := &WorkspaceTab{ID: "tab", Scope: "global", Ctrl: ctrl, Ready: true, disabledMCP: map[string]ServerView{}}
	app.tabRuntimes[tab.ID] = tab

	call := requireHistoryToolCall(t, mustHistoryForTab(t, app, tab.ID), taskCallID)
	if call.SubagentRef != "" || len(call.SubagentMessages) != 0 {
		t.Fatalf("attached subagent from another parent: %+v", call)
	}
}

func TestPreviewSessionMessagesIncludesProcessEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	body := strings.Join([]string{
		`{"kind":"phase","text":"Preparing context"}`,
		`{"kind":"notice","level":"warn","text":"Network changed"}`,
		`{"kind":"compaction_started","compaction":{"trigger":"manual"}}`,
		`{"kind":"compaction_done","compaction":{"trigger":"manual","messages":6,"summary":"Kept the current task.","archive":"/tmp/archive.jsonl"}}`,
		`{"type":"user.message","text":"hello"}`,
		`{"type":"model.final","content":"hi","reasoningContent":"thinking"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := previewSessionMessages(dir, path)
	if err != nil {
		t.Fatalf("previewSessionMessages: %v", err)
	}
	if len(got) != 6 {
		t.Fatalf("preview history length = %d, want 6: %+v", len(got), got)
	}
	if got[0].Role != "phase" || got[0].Content != "Preparing context" {
		t.Fatalf("phase event not preserved: %+v", got[0])
	}
	if got[1].Role != "notice" || got[1].Level != "warn" || got[1].Content != "Network changed" {
		t.Fatalf("notice event not preserved: %+v", got[1])
	}
	if got[2].Role != "compaction" || !got[2].Pending || got[2].Trigger != "manual" {
		t.Fatalf("pending compaction event not preserved: %+v", got[2])
	}
	if got[3].Role != "compaction" || got[3].Pending || got[3].Messages != 6 || got[3].Summary != "Kept the current task." || got[3].Archive != "/tmp/archive.jsonl" {
		t.Fatalf("finished compaction event not preserved: %+v", got[3])
	}
	if got[4].Role != "user" || got[5].Reasoning != "thinking" {
		t.Fatalf("conversation events not preserved: %+v", got[4:])
	}
}

func TestPreviewEventSessionMessagesStripsHiddenExpertPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	submitText := `<system-prompt source="expert" hidden="true" id="delivery-team" name="代码交付专家团" mode="team">
Coordinate the expert team.
</system-prompt>

修复专家团回显问题`
	line, err := json.Marshal(map[string]string{
		"type": "user.message",
		"text": submitText,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := previewSessionMessages(dir, path)
	if err != nil {
		t.Fatalf("previewSessionMessages: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("preview history length = %d, want 1: %+v", len(got), got)
	}
	if got[0].Content != "修复专家团回显问题" {
		t.Fatalf("event user content = %q, want visible prompt only", got[0].Content)
	}
	if got[0].SubmitText != submitText {
		t.Fatalf("event user submit text = %q, want hidden expert submit text preserved", got[0].SubmitText)
	}
}

func TestResumeSessionForTabTargetsSpecifiedTab(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := desktopSessionDir(globalTabWorkspaceRoot())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}

	activePath := filepath.Join(dir, "active.jsonl")
	inactivePath := filepath.Join(dir, "inactive.jsonl")
	targetPath := filepath.Join(dir, "target.jsonl")
	writeHistoryTestSession(t, activePath, "active prompt")
	writeHistoryTestSession(t, inactivePath, "inactive prompt")
	writeHistoryTestSession(t, targetPath, "target prompt")

	activeExec := agent.New(nil, nil, agent.NewSession(""), agent.Options{}, event.Discard)
	inactiveExec := agent.New(nil, nil, agent.NewSession(""), agent.Options{}, event.Discard)
	activeCtrl := control.New(control.Options{Executor: activeExec, SessionDir: dir, SessionPath: activePath, Label: "active"})
	inactiveCtrl := control.New(control.Options{Executor: inactiveExec, SessionDir: dir, SessionPath: inactivePath, Label: "inactive"})
	defer activeCtrl.Close()
	defer inactiveCtrl.Close()

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"active": {
				ID:            "active",
				Scope:         "global",
				WorkspaceRoot: globalTabWorkspaceRoot(),
				Ctrl:          activeCtrl,
				Ready:         true,
				sink:          &tabEventSink{tabID: "active"},
				disabledMCP:   map[string]ServerView{},
			},
			"inactive": {
				ID:            "inactive",
				Scope:         "global",
				WorkspaceRoot: globalTabWorkspaceRoot(),
				SessionPath:   inactivePath,
				Ctrl:          inactiveCtrl,
				Ready:         true,
				sink:          &tabEventSink{tabID: "inactive"},
				disabledMCP:   map[string]ServerView{},
			},
		},
		tabOrder:    []string{"active", "inactive"},
		activeTabID: "active",
	}

	got, err := app.ResumeSessionForTab("inactive", targetPath)
	if err != nil {
		t.Fatalf("ResumeSessionForTab: %v", err)
	}
	if activeCtrl.SessionPath() != activePath {
		t.Fatalf("active tab session path = %q, want %q", activeCtrl.SessionPath(), activePath)
	}
	if inactiveCtrl.SessionPath() != inactivePath {
		t.Fatalf("original inactive controller session path = %q, want %q", inactiveCtrl.SessionPath(), inactivePath)
	}
	if app.tabRuntimes["inactive"].Ctrl == inactiveCtrl {
		t.Fatal("resume to a different sessionPath mutated the existing controller in place")
	}
	if app.tabRuntimes["inactive"].Ctrl.SessionPath() != targetPath {
		t.Fatalf("inactive tab session path = %q, want %q", app.tabRuntimes["inactive"].Ctrl.SessionPath(), targetPath)
	}
	f := loadTabsFile()
	var savedInactive string
	for _, entry := range f.Tabs {
		if entry.ID == "inactive" {
			savedInactive = entry.SessionPath
			break
		}
	}
	if filepath.Clean(savedInactive) != filepath.Clean(targetPath) {
		t.Fatalf("saved inactive session path = %q, want %q", savedInactive, targetPath)
	}
	if len(got) != 1 || got[0].Content != "target prompt" {
		t.Fatalf("resumed history = %+v, want target prompt", got)
	}
}

func TestResumeSessionForTabRejectsOtherProfileSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actorA := userctx.ActorContext{ActorID: "actor", ProfileID: "alpha", AuthMethod: "test", WorkspaceRoot: root}
	actorB := userctx.ActorContext{ActorID: "actor", ProfileID: "beta", AuthMethod: "test", WorkspaceRoot: root}
	dirA := desktopSessionDirForActor(actorA, root)
	dirB := desktopSessionDirForActor(actorB, root)
	if filepath.Clean(dirA) == filepath.Clean(dirB) {
		t.Fatalf("test setup error: profile session dirs should differ: %q", dirA)
	}
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}

	activePath := filepath.Join(dirA, "active.jsonl")
	otherProfilePath := filepath.Join(dirA, "other-profile.jsonl")
	writeHistoryTestSession(t, activePath, "active prompt")
	writeHistoryTestSession(t, otherProfilePath, "other profile prompt")

	ctrl := control.New(control.Options{SessionDir: dirA, SessionPath: activePath, WorkspaceRoot: root, Actor: actorA, Label: "active"})
	defer ctrl.Close()
	app := NewApp()
	tab := &WorkspaceTab{
		ID:            "profile-tab",
		Scope:         "project",
		WorkspaceRoot: root,
		SessionPath:   activePath,
		Ctrl:          ctrl,
		Ready:         true,
		actor:         actorB,
		sink:          &tabEventSink{tabID: "profile-tab", app: app},
		disabledMCP:   map[string]ServerView{},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	_, err := app.ResumeSessionForTab(tab.ID, otherProfilePath)
	if err == nil || !strings.Contains(err.Error(), "outside session dir") {
		t.Fatalf("ResumeSessionForTab error = %v, want profile session-dir boundary error", err)
	}
	if got := filepath.Clean(ctrl.SessionPath()); got != filepath.Clean(activePath) {
		t.Fatalf("session path after rejected resume = %q, want %q", got, activePath)
	}
	if got := filepath.Clean(tabSessionDir(tab)); got != filepath.Clean(dirB) {
		t.Fatalf("tabSessionDir after rejected resume = %q, want current profile dir %q", got, dirB)
	}
}

func TestResumeSessionForTabDetachesRunningRuntimeForDifferentSessionPath(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := desktopSessionDir(globalTabWorkspaceRoot())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}

	topicID := "topic_same"
	sessionA := filepath.Join(dir, "session-a.jsonl")
	sessionB := filepath.Join(dir, "session-b.jsonl")
	writeHistoryTestSession(t, sessionA, "session A prompt")
	writeHistoryTestSession(t, sessionB, "session B prompt")

	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrlA := control.New(control.Options{
		Runner:      runner,
		SessionDir:  dir,
		SessionPath: sessionA,
		Label:       "session-a",
		Sink:        event.Discard,
	})
	defer ctrlA.Close()

	app := NewApp()
	tab := &WorkspaceTab{
		ID:                  "topic-tab",
		Scope:               "global",
		WorkspaceRoot:       globalTabWorkspaceRoot(),
		TopicID:             topicID,
		TopicTitle:          "Same topic",
		SessionPath:         sessionA,
		Ctrl:                ctrlA,
		Ready:               true,
		sink:                &tabEventSink{tabID: "topic-tab", app: app},
		disabledMCP:         map[string]ServerView{},
		plannerDisplay:      []HistoryMessage{{Role: "assistant", Content: "session A planning"}},
		plannerDisplayTools: map[string]string{"call-a": "read_file"},
		usageTelemetry: sessionUsageStats{Sources: map[string]usageSourceStats{
			event.UsageSourcePlanner: {PromptTokens: 17},
		}},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	ctrlA.Submit("keep running")
	<-runner.started

	got, err := app.ResumeSessionForTab(tab.ID, sessionB)
	if err != nil {
		t.Fatalf("ResumeSessionForTab: %v", err)
	}
	if !ctrlA.Running() {
		t.Fatal("session A controller was cancelled while resuming session B")
	}
	if ctrlA.SessionPath() != sessionA {
		t.Fatalf("session A controller path = %q, want %q", ctrlA.SessionPath(), sessionA)
	}
	detached := app.detachedSessions[sessionRuntimeKey(sessionA)]
	if detached == nil || detached.Ctrl != ctrlA {
		t.Fatalf("session A runtime was not detached: %+v", detached)
	}
	if detached.ID == tab.ID {
		t.Fatalf("detached runtime kept visible tab id %q", detached.ID)
	}
	if detached.sink == nil {
		t.Fatal("detached runtime lost its event sink")
	}
	if detached.sink.currentTabID() == tab.ID {
		t.Fatalf("detached sink tab id = %q, want non-visible id", detached.sink.currentTabID())
	}
	if len(detached.plannerDisplay) != 1 || detached.plannerDisplay[0].Content != "session A planning" {
		t.Fatalf("detached planner display = %+v, want session A planning", detached.plannerDisplay)
	}
	if detached.usageTelemetry.Sources[event.UsageSourcePlanner].PromptTokens != 17 {
		t.Fatalf("detached usage = %+v, want session A planner usage", detached.usageTelemetry)
	}
	if app.tabRuntimes[tab.ID].Ctrl == ctrlA {
		t.Fatal("visible tab still points at session A runtime after resuming session B")
	}
	if len(app.tabRuntimes[tab.ID].plannerDisplay) != 0 || len(app.tabRuntimes[tab.ID].usageTelemetry.Sources) != 0 {
		t.Fatalf("session B inherited session A sidecars: planner=%+v usage=%+v", app.tabRuntimes[tab.ID].plannerDisplay, app.tabRuntimes[tab.ID].usageTelemetry)
	}
	if gotPath := app.tabRuntimes[tab.ID].Ctrl.SessionPath(); gotPath != sessionB {
		t.Fatalf("visible tab session path = %q, want %q", gotPath, sessionB)
	}
	if len(got) != 1 || got[0].Content != "session B prompt" {
		t.Fatalf("resumed history = %+v, want session B prompt", got)
	}

	visible := app.tabRuntimes[tab.ID]
	detached.sink.Emit(event.Event{Kind: event.TurnStarted})
	if visible.ActivityStatus != "" {
		t.Fatalf("detached runtime event changed visible tab status to %q", visible.ActivityStatus)
	}
	detached.sink.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{
		Name:   "read_file",
		Args:   `{"path":"detached.go","offset":3,"limit":7}`,
		Output: "package main",
	}})
	if got := detached.telemetrySnapshot().ReadFiles; len(got) != 1 || got[0].Path != "detached.go" || got[0].Offset != 3 || got[0].Limit != 7 {
		t.Fatalf("detached runtime read telemetry = %+v", got)
	}
	if got := visible.telemetrySnapshot().ReadFiles; len(got) != 0 {
		t.Fatalf("detached runtime read telemetry was recorded on visible tab: %+v", got)
	}
	detached.sink.Emit(event.Event{Kind: event.Usage, Usage: &provider.Usage{PromptTokens: 42}})
	detached.sink.Emit(event.Event{Kind: event.TurnDone})
	if detached.usageTelemetry.PromptTokens != 42 {
		t.Fatalf("detached runtime usage was not recorded on detached tab: %+v", detached.usageTelemetry)
	}
	if detached.usageTelemetry.RequestCount != 1 {
		t.Fatalf("detached runtime request count = %d, want 1", detached.usageTelemetry.RequestCount)
	}
	if visible.usageTelemetry.PromptTokens != 0 {
		t.Fatalf("detached runtime usage was recorded on visible tab: %+v", visible.usageTelemetry)
	}
	if visible.saveAgain || visible.saving {
		t.Fatalf("detached runtime scheduled visible tab snapshot: saving=%v saveAgain=%v", visible.saving, visible.saveAgain)
	}

	close(runner.release)
	waitNotRunning(t, ctrlA)
}

func TestRebindTabToLoadedSessionReusesPreloadedTranscript(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}

	currentPath := filepath.Join(dir, "current.jsonl")
	targetPath := filepath.Join(dir, "target.jsonl")
	writeHistoryTestSession(t, currentPath, "current prompt")
	writeHistoryTestSession(t, targetPath, "target prompt")

	loaded, err := agent.LoadSession(targetPath)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if err := os.Remove(targetPath); err != nil {
		t.Fatalf("remove target session: %v", err)
	}

	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: currentPath, Label: "current", Sink: event.Discard})
	defer ctrl.Close()

	app := NewApp()
	tab := &WorkspaceTab{
		ID:            "tab",
		Scope:         "global",
		WorkspaceRoot: root,
		SessionPath:   currentPath,
		Ctrl:          ctrl,
		Ready:         true,
		sink:          &tabEventSink{tabID: "tab", app: app},
		disabledMCP:   map[string]ServerView{},
	}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.rebindTabToLoadedSessionPath(tab, targetPath, loaded); err != nil {
		t.Fatalf("rebindTabToLoadedSessionPath: %v", err)
	}
	got := mustHistoryForTab(t, app, tab.ID)
	if len(got) != 1 || got[0].Content != "target prompt" {
		t.Fatalf("rebound history = %+v, want target prompt", got)
	}
	if gotPath := app.tabRuntimes[tab.ID].Ctrl.SessionPath(); gotPath != targetPath {
		t.Fatalf("rebound session path = %q, want %q", gotPath, targetPath)
	}
}

func writeHistoryTestSession(t *testing.T, path, prompt string) {
	t.Helper()
	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: prompt})
	if err := session.Save(path); err != nil {
		t.Fatalf("Save %s: %v", path, err)
	}
}

func saveCompletedHistorySubagent(t *testing.T, sessionDir, parentSession, parentToolCallID string, messages []provider.Message) string {
	t.Helper()
	store := agent.NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	run, err := store.PrepareFresh(agent.SubagentSpec{
		Kind:             "task",
		Name:             "Research Expert",
		WorkspaceRoot:    t.TempDir(),
		ParentSession:    parentSession,
		ParentToolCallID: parentToolCallID,
		SystemPrompt:     "",
		Registry:         tool.NewRegistry(),
		Model:            "deepseek",
		Effort:           "max",
	})
	if err != nil {
		t.Fatalf("PrepareFresh subagent: %v", err)
	}
	defer run.Release()
	for _, msg := range messages {
		run.Session.Add(msg)
	}
	if err := store.SaveCompleted(run); err != nil {
		t.Fatalf("SaveCompleted subagent: %v", err)
	}
	return run.Ref
}

func setHistorySubagentCreatedAt(t *testing.T, sessionDir, parentSession, ref string, createdAt time.Time) {
	t.Helper()
	artifacts, err := agent.ListSubagentsByParent(sessionDir, parentSession)
	if err != nil {
		t.Fatalf("ListSubagentsByParent: %v", err)
	}
	for _, artifact := range artifacts {
		if artifact.Ref != ref {
			continue
		}
		artifact.Meta.CreatedAt = createdAt
		data, err := json.MarshalIndent(artifact.Meta, "", "  ")
		if err != nil {
			t.Fatalf("marshal subagent metadata: %v", err)
		}
		if err := os.WriteFile(artifact.MetaPath, data, 0o600); err != nil {
			t.Fatalf("write subagent metadata: %v", err)
		}
		return
	}
	t.Fatalf("subagent metadata %q not found", ref)
}

func requireHistoryToolCall(t *testing.T, messages []HistoryMessage, id string) HistoryToolCall {
	t.Helper()
	for _, msg := range messages {
		for _, call := range msg.ToolCalls {
			if call.ID == id {
				return call
			}
		}
	}
	t.Fatalf("history tool call %q not found in %+v", id, messages)
	return HistoryToolCall{}
}
