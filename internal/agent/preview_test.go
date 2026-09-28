package agent

import (
	"strings"
	"testing"

	"kwu/internal/provider"
)

// realContract mirrors the shape compileExecutionContract emits: the
// source_event lives under planner_ir, and the block replaces the whole user
// turn.
func realContract(sourceEvent string) string {
	return "<memory-compiler-execution>\n" +
		`{"type":"memory_v5_execution_contract","instruction":"Execute source_event through planner_ir.",` +
		`"ir_explanation":{},"planner_ir":{"version":5,"goal":"g","source_event":` + jsonString(sourceEvent) + `}}` +
		"\n</memory-compiler-execution>"
}

func jsonString(s string) string {
	// minimal JSON string quoting for test fixtures (no control chars used here)
	return `"` + s + `"`
}

func TestTurnInterruptedMessageDoesNotBecomeUserPreviewOrTurn(t *testing.T) {
	marker := TurnInterruptedMessage()
	if !IsTurnInterruptedMessage(marker) {
		t.Fatal("host interruption marker was not recognized")
	}
	if got := UserPreviewText(marker); got != "" {
		t.Fatalf("UserPreviewText(marker) = %q, want empty", got)
	}
	preview, turns := SessionPreviewFromMessages([]provider.Message{
		{Role: provider.RoleUser, Content: "keep this prompt"},
		{Role: provider.RoleUser, Content: marker},
	})
	if preview != "keep this prompt" || turns != 1 {
		t.Fatalf("preview/turns = %q/%d, want kept prompt/1", preview, turns)
	}
}

func TestSessionPreviewIgnoresPersistedTurnInterruptedMessage(t *testing.T) {
	session := NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "keep this prompt"})
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "partial work"})
	session.Add(provider.Message{Role: provider.RoleUser, Content: TurnInterruptedMessage()})
	path := NewSessionPath(t.TempDir(), "test-model")
	if err := session.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	preview, turns := SessionPreview(path)
	if preview != "keep this prompt" || turns != 1 {
		t.Fatalf("persisted preview/turns = %q/%d, want kept prompt/1", preview, turns)
	}
}

// TestStripTransientUserBlocksUnwrapsMemoryCompilerExecution guards the #5307
// contract: the Memory v5 <memory-compiler-execution> block REPLACES the user
// turn (the prompt survives only inside the contract's source_event), so the
// display/preview path must unwrap it to the original prompt — not drop it like
// a prepended transient block, which would blank out the turn.
func TestStripTransientUserBlocksUnwrapsMemoryCompilerExecution(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "block only (compiled contract replaced the whole turn)",
			in:   realContract("add a config loader"),
			want: "add a config loader",
		},
		{
			name: "language blocks before the compiler block",
			// Real composition order: withTurnPreferences wraps the compiled
			// contract, so the language blocks lead and the compiler block
			// follows. Both must resolve to the original prompt.
			in: "<reasoning-language>zh</reasoning-language>\n\n" +
				"<response-language>zh</response-language>\n\n" + realContract("do the thing"),
			want: "do the thing",
		},
		{
			name: "top-level source_event fallback shape",
			in:   "<memory-compiler-execution>\n{\"source_event\":\"older shape\"}\n</memory-compiler-execution>",
			want: "older shape",
		},
		{
			name: "unrecoverable contract falls back to empty",
			in:   "<memory-compiler-execution>\n{\"type\":\"memory_v5_execution_contract\"}\n</memory-compiler-execution>",
			want: "",
		},
		{
			name: "non-contract content is untouched",
			in:   "just a normal prompt",
			want: "just a normal prompt",
		},
		{
			name: "profile context before prompt",
			in:   "<user-profile-context>\n# 来自用户画像\n- Prefer targeted tests.\n</user-profile-context>\n\nfix profile loading",
			want: "fix profile loading",
		},
		{
			name: "temporal context and workflow advice before prompt",
			in: `<temporal_context date="2026-07-20" weekday="Monday" weekday_iso="1" timezone="Asia/Shanghai" utc_offset="+08:00"/>` + "\n\n" +
				`<memory_workflow_advice authority="advisory">verified flow</memory_workflow_advice>` + "\n\n今天星期几",
			want: "今天星期几",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripTransientUserBlocks(tc.in); got != tc.want {
				t.Fatalf("StripTransientUserBlocks(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestUserPreviewTextPreservesCompiledTurnPrompt is the regression for the bot
// finding: a session whose first turn was compiled must still show the user's
// prompt in history/sidebar previews, not a blank line.
func TestUserPreviewTextPreservesCompiledTurnPrompt(t *testing.T) {
	in := realContract("ship the refactor")
	if got := UserPreviewText(in); got != "ship the refactor" {
		t.Fatalf("UserPreviewText = %q, want %q (compiled turn must not blank the preview)", got, "ship the refactor")
	}
}

func TestUserPreviewTextStripsHiddenExpertPrompt(t *testing.T) {
	in := `<system-prompt source="expert" hidden="true" id="code-delivery-team" name="代码交付专家团" mode="team">
Coordinate the team.
</system-prompt>

修复登录页按钮问题`
	if got := UserPreviewText(in); got != "修复登录页按钮问题" {
		t.Fatalf("UserPreviewText = %q, want user-authored prompt only", got)
	}
	if got := StripTransientUserBlocks(in); got == "修复登录页按钮问题" {
		t.Fatalf("StripTransientUserBlocks stripped hidden prompt; expert team routing needs the prefix intact")
	}
}

func TestCapabilityBlocksAreTransientForIntentRouting(t *testing.T) {
	in := `<enabled-mcp-connectors>
Enabled for this turn: playwright.
</enabled-mcp-connectors>

<selected-skill name="cyber-ppt">
# Skill: cyber-ppt
Create and verify a presentation artifact.
</selected-skill>

<system-prompt source="expert" hidden="true" id="code-delivery-team" mode="team" member-names='["reviewer"]'>
Coordinate the team.
</system-prompt>

只告诉我当前启用了哪些能力`
	stripped := StripTransientUserBlocks(in)
	if !strings.HasPrefix(stripped, `<system-prompt source="expert"`) {
		t.Fatalf("StripTransientUserBlocks left capability blocks before the expert prompt: %q", stripped)
	}
	if got := UserPreviewText(in); got != "只告诉我当前启用了哪些能力" {
		t.Fatalf("UserPreviewText = %q, want only the user-authored request", got)
	}
	members, ok := expertTeamConfiguredMembers(in)
	if !ok || len(members) != 1 || members[0] != "reviewer" {
		t.Fatalf("expert team behind capability blocks = (%v, %v), want reviewer", members, ok)
	}
}

func TestUserPreviewTextPreservesIndentedBodyAfterHiddenExpertPrompt(t *testing.T) {
	in := `<system-prompt source="expert" hidden="true" id="code-delivery-team" mode="team">
Coordinate the team.
</system-prompt>` + " \t\n\t\n" + `    fmt.Println("hello")`
	want := `    fmt.Println("hello")`
	if got := UserPreviewText(in); got != want {
		t.Fatalf("UserPreviewText = %q, want %q (user Markdown code indentation must survive protocol stripping)", got, want)
	}
}

func TestUserPreviewTextRecoversLegacyExpertAutoResearchGoal(t *testing.T) {
	in := `<active-goal>
<system-prompt source="expert" hidden="true" id="code-delivery-team" mode="team">
Internal expert instructions that must not be displayed.
</system-prompt>

持续排查这个线上卡顿直到根因明确，并验证修复

Goal mode: pursue this goal autonomously. Keep working until complete.
</active-goal>

Start pursuing the active goal now.`
	want := "持续排查这个线上卡顿直到根因明确，并验证修复"
	if got := UserPreviewText(in); got != want {
		t.Fatalf("UserPreviewText = %q, want recovered legacy expert goal %q", got, want)
	}
}

// TestSessionPreviewFromMessagesPreservesCompiledFirstTurn proves the end-to-end
// preview path (used for the picker/sidebar) recovers the prompt when the first
// persisted user turn is a compiled contract.
func TestSessionPreviewFromMessagesPreservesCompiledFirstTurn(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "sys"},
		{Role: provider.RoleUser, Content: realContract("add pagination to the users endpoint")},
		{Role: provider.RoleAssistant, Content: "done"},
	}
	preview, turns := SessionPreviewFromMessages(msgs)
	if preview != "add pagination to the users endpoint" {
		t.Fatalf("preview = %q, want the compiled turn's source_event", preview)
	}
	if turns != 1 {
		t.Fatalf("user turns = %d, want 1", turns)
	}
}
