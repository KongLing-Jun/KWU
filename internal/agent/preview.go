package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

const turnInterruptedMessage = `<system-prompt source="turn-interrupted" hidden="true">
The previous turn was interrupted by the user. Preserve completed tool side effects, do not assume unfinished work completed, and wait for the user's next instruction before resuming it.
</system-prompt>`

// TurnInterruptedMessage is persisted after a user explicitly stops a turn.
// It remains part of model context while display projections render it as an
// interrupted boundary rather than as a user-authored message.
func TurnInterruptedMessage() string { return turnInterruptedMessage }

// IsTurnInterruptedMessage reports whether content is the host-authored turn
// interruption boundary.
func IsTurnInterruptedMessage(content string) bool {
	return strings.TrimSpace(content) == turnInterruptedMessage
}

var reTransientUserBlock = regexp.MustCompile(`(?s)^\s*(?:<temporal_context\b[^>]*/>|<(?:response-language|reasoning-language|memory-update|background-jobs|user-profile-context|memory_workflow_advice|enabled-mcp-connectors|selected-skill)(?:\s[^>]*)?>.*?</(?:response-language|reasoning-language|memory-update|background-jobs|user-profile-context|memory_workflow_advice|enabled-mcp-connectors|selected-skill)>)`)

var reMemoryCompilerExecution = regexp.MustCompile(`(?s)<memory-compiler-execution>\s*(.*?)\s*</memory-compiler-execution>`)

// StripTransientUserBlocks removes controller-injected transient XML blocks
// from persisted user messages before deriving display text, previews, or
// titles. The blocks are sent in user turns so they never affect the stable
// prompt prefix, but they should not become user-facing text later.
//
// Legacy Memory v5 <memory-compiler-execution> blocks are handled differently from
// the prepended transient blocks: it does not prefix the user's prompt, it
// REPLACES the whole turn (Agent.Run swaps the compiled contract in for the
// original input, keeping the user's text only in the contract's source_event
// field). Dropping it like a prefix block would leave an empty string, so we
// unwrap it to the original prompt instead — otherwise sessions whose first
// turn was compiled would show a blank history/sidebar preview (#5307).
func StripTransientUserBlocks(content string) string {
	s := unwrapMemoryCompilerExecution(content)
	for {
		loc := reTransientUserBlock.FindStringIndex(s)
		if loc == nil {
			break
		}
		s = stripProtocolSeparators(s[loc[1]:])
	}
	return s
}

// StripHiddenSystemPromptBlocks removes hidden expert/system context wrappers
// from display text. It is intentionally separate from StripTransientUserBlocks
// because expert-team tool routing must still see the hidden prefix before the
// model turn is sent.
func StripHiddenSystemPromptBlocks(content string) string {
	s := content
	for {
		next, ok := stripHiddenSystemPromptPrefix(s)
		if !ok {
			break
		}
		s = next
	}
	return s
}

func stripHiddenSystemPromptPrefix(content string) (string, bool) {
	s := strings.TrimLeft(content, " \t\r\n")
	if !strings.HasPrefix(s, "<system-prompt") {
		return content, false
	}
	if _, _, ok := hiddenSystemPromptPrefixAttrs(s); !ok {
		return content, false
	}
	end := strings.Index(strings.ToLower(s), "</system-prompt>")
	if end < 0 {
		return content, false
	}
	return stripProtocolSeparators(s[end+len("</system-prompt>"):]), true
}

func stripProtocolSeparators(s string) string {
	for range 2 {
		lineEnd := 0
		for lineEnd < len(s) && (s[lineEnd] == ' ' || s[lineEnd] == '\t') {
			lineEnd++
		}
		switch {
		case strings.HasPrefix(s[lineEnd:], "\r\n"):
			s = s[lineEnd+2:]
		case strings.HasPrefix(s[lineEnd:], "\n"), strings.HasPrefix(s[lineEnd:], "\r"):
			s = s[lineEnd+1:]
		default:
			return s
		}
	}
	return s
}

// unwrapMemoryCompilerExecution replaces a <memory-compiler-execution> contract
// with the user prompt it was compiled from (the contract's source_event), so
// display text and previews show what the user typed rather than the raw IR
// JSON or an empty string. Non-contract content is returned unchanged; a
// contract without a recoverable source_event collapses to empty, matching the
// prior "strip the block" behavior only as a last resort.
func unwrapMemoryCompilerExecution(content string) string {
	if !strings.Contains(content, "<memory-compiler-execution>") {
		return content
	}
	return reMemoryCompilerExecution.ReplaceAllStringFunc(content, func(block string) string {
		m := reMemoryCompilerExecution.FindStringSubmatch(block)
		if len(m) < 2 {
			return ""
		}
		return memoryCompilerSourceEvent(m[1])
	})
}

// memoryCompilerSourceEvent pulls the original user prompt out of a compiled
// execution contract's JSON body. The source_event lives under planner_ir; an
// older/looser shape may carry it at the top level, so both are checked.
// Returns "" when the body is not the expected JSON or carries no source_event.
func memoryCompilerSourceEvent(body string) string {
	var contract struct {
		SourceEvent string `json:"source_event"`
		PlannerIR   struct {
			SourceEvent string `json:"source_event"`
		} `json:"planner_ir"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &contract); err != nil {
		return ""
	}
	if s := strings.TrimSpace(contract.PlannerIR.SourceEvent); s != "" {
		return s
	}
	return strings.TrimSpace(contract.SourceEvent)
}

// UserPreviewText returns the user-authored part of a persisted user message.
func UserPreviewText(content string) string {
	if IsTurnInterruptedMessage(content) {
		return ""
	}
	s := StripTransientUserBlocks(content)
	if preview, ok := legacyHiddenExpertGoalPreview(s); ok {
		return preview
	}
	s = StripHiddenSystemPromptBlocks(s)
	s = HandoffTask(s)
	s = StripHiddenSystemPromptBlocks(StripTransientUserBlocks(s))
	return strings.TrimRight(s, " \t\r\n")
}

// legacyHiddenExpertGoalPreview recovers sessions created before AutoResearch
// stripped hidden expert context before persisting its goal. Those sessions
// have this shape:
//
//	<active-goal>
//	<system-prompt hidden="true">...</system-prompt>
//	user text
//
//	Goal mode: ...
//	</active-goal>
//
// Returning only the user text keeps old transcripts usable without treating a
// normal active-goal block as display content.
func legacyHiddenExpertGoalPreview(content string) (string, bool) {
	s := strings.TrimLeft(content, " \t\r\n")
	const open = "<active-goal>"
	const close = "</active-goal>"
	if !strings.HasPrefix(s, open) {
		return "", false
	}
	end := strings.Index(s[len(open):], close)
	if end < 0 {
		return "", false
	}
	body := s[len(open) : len(open)+end]
	visible, ok := stripHiddenSystemPromptPrefix(body)
	if !ok {
		return "", false
	}
	const policy = "\n\nGoal mode:"
	policyStart := strings.Index(visible, policy)
	if policyStart < 0 {
		return "", false
	}
	visible = strings.TrimRight(visible[:policyStart], " \t\r\n")
	if strings.TrimSpace(visible) == "" {
		return "", false
	}
	return visible, true
}
