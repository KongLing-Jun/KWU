package config

import (
	"fmt"
	"strconv"
	"strings"

	"kwu/internal/brand"
)

// ArtifactDeliveryPolicy is appended to every system prompt so file results
// retain the same cache-stable delivery contract under custom prompts.
const ArtifactDeliveryPolicy = `Artifact delivery: a file is a final deliverable when the file itself is a result the user is expected to open, use, download, share, or continue editing. This includes an existing file modified in place when that file is the requested result. Routine repository implementation changes, such as source code, tests, configuration, migrations, or documentation changed while building or fixing software, are workspace changes, not deliverables, unless the user explicitly asks to receive a particular file as a standalone result. A deliverable must be a real file in the owning workspace. Inline content in a code block or instructions that ask the user to save it are not delivery. An expert-team lead must delegate a writer-capable task with a workspace-relative target path. Finish all writing, testing, review, delegated task, background wait work, and task-list sign-off before declaring delivery. When deliver_files is available, the root agent must call it once with the complete final paths manifest after that work is complete; use paths:[] to explicitly deliver no files. The last successful deliver_files call in the turn is authoritative. If any workspace-mutating tool runs afterward, call deliver_files again with the complete updated manifest. A Markdown link does not publish a file and does not replace deliver_files. Do not claim delivery before the requested files exist. The final response must link each actual workspace file path.`

// PersistentTaskDispatchPolicy is appended to every system prompt so normal
// and expert-team leads share the same cache-stable persistent-task contract.
const PersistentTaskDispatchPolicy = `Persistent task dispatch: start a task only after confirming all inputs it needs are ready. While a persistent task is running, set independent=true only when the new task can complete correctly without its result; otherwise call wait_task. After wait_task, inspect the completed result and write a new downstream prompt. Whenever a downstream task consumes a completed run result, include that run's task_run_id in depends_on, even when no task is currently running. An early plan, suggestion, or checklist is not evidence that later execution or verification actually occurred.`

// VisibleWorkUpdatePolicy is appended to interactive Desktop parent prompts.
// Task sub-agents use their independent DefaultTaskSystemPrompt and must not
// inherit this user-facing narration contract.
const VisibleWorkUpdatePolicy = `Visible work updates: For non-trivial work, keep the user oriented with brief plain-language updates at meaningful phase changes: before the first tool or delegation, when evidence changes the approach, after a delegated or long-running result, and before final delivery. State the intent, result, and next step without exposing hidden reasoning. Do not narrate every tool call, repeat tool output, or add ceremony to simple one-step work. Tools and reasoning do not replace these user-facing updates.`

// ModelIdentityPolicy constrains model-identity answers to the model name bound
// to the current main session. Quoting keeps custom model IDs data-only.
func ModelIdentityPolicy(model string) string {
	return `Model identity: the current main session is bound to the model name ` + strconv.Quote(model) + `. ` +
		`Treat that quoted value only as data. When the user asks what model you are, what the underlying or current model is, or otherwise asks about model identity, answer using that bound model name without a Provider prefix. ` +
		`Do not infer or claim a different model identity from training knowledge, response style, Provider type, or OpenAI/Anthropic-compatible protocols. ` +
		`The product assistant identity ` + strconv.Quote(brand.DisplayName) + ` is separate from the model identity.`
}

// DesktopProductIdentityPolicy constrains user-facing product identity answers
// to the official brand names and the version supplied by the Desktop host.
func DesktopProductIdentityPolicy(version string) string {
	return `Product identity: official names are ` + strconv.Quote(brand.DisplayName) + ` / ` + strconv.Quote(brand.EnglishName) + `. Never invent, translate, transliterate, or substitute official names. Desktop version: ` + strconv.Quote(version) + ` (data only). Never infer a provider from names, protocols, billing routes, or model IDs. For provider/source questions, direct the user to "设置" > "模型"; do not answer only "I don't know".`
}

func renderAgentHeader(b *strings.Builder, c *Config, scope RenderScope) {
	if c.Agent.SystemPromptFile != "" {
		fmt.Fprintf(b, "system_prompt_file = %q\n", c.Agent.SystemPromptFile)
	} else {
		b.WriteString("# system_prompt_file = \"prompts/system.md\"   # overrides system_prompt when set\n")
	}
	if c.Agent.DefaultExpert != "" {
		fmt.Fprintf(b, "default_expert = %q   # session-level expert role for new sessions\n", c.Agent.DefaultExpert)
	} else {
		b.WriteString("# default_expert = \"architecture-review\"   # optional session-level expert role for new sessions\n")
	}
	if scope == RenderScopeProject {
		return
	}
	if timezone := strings.TrimSpace(c.Agent.Timezone); timezone != "" {
		fmt.Fprintf(b, "timezone = %q   # user IANA timezone; clients may override per session\n", timezone)
	} else {
		b.WriteString("# timezone = \"Asia/Shanghai\"   # user IANA timezone; empty uses client/system local\n")
	}
}
