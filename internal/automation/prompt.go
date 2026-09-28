package automation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kwu/internal/brand"
)

// PromptCompiler builds the one-turn input for an automation run.
type PromptCompiler struct{}

type PromptCompileOptions struct {
	AssistPath string
	Assist     string
}

func (PromptCompiler) Compile(task Task, run RunManifest) string {
	return PromptCompiler{}.CompileWithOptions(task, run, PromptCompileOptions{})
}

func (PromptCompiler) CompileWithOptions(task Task, run RunManifest, opts PromptCompileOptions) string {
	var b strings.Builder
	writeLine(&b, "# Automation Task")
	writeLine(&b, "")
	writeLine(&b, "You are running a "+brand.DisplayName+" automation task in the background.")
	writeLine(&b, "This is the one-time input for this run. Do not save this task prompt to memory, configuration, skill storage, or a system prompt.")
	writeLine(&b, "")
	writeLine(&b, "## Run Context")
	writeField(&b, "Task name", firstNonEmpty(run.TaskName, task.Name))
	writeField(&b, "Task id", firstNonEmpty(run.TaskID, task.ID))
	writeField(&b, "Run id", run.RunID)
	writeField(&b, "Trigger", string(run.Trigger))
	writeField(&b, "Host owner", run.HostOwner)
	writeField(&b, "Workspace", firstNonEmpty(run.WorkspaceRoot, task.WorkspaceRoot))

	if role := strings.TrimSpace(task.Prompt.Role); role != "" {
		writeLine(&b, "")
		writeLine(&b, "## Role / Expert")
		writeLine(&b, role)
	}
	if len(task.Prompt.Skills) > 0 {
		writeLine(&b, "")
		writeLine(&b, "## Required Skills")
		for _, skill := range task.Prompt.Skills {
			skill = strings.TrimSpace(skill)
			if skill != "" {
				writeLine(&b, "- "+skill)
			}
		}
		writeLine(&b, "")
		writeLine(&b, "Use these skills when relevant. If a named skill is unavailable, report it clearly.")
	}
	if assist := strings.TrimSpace(opts.Assist); assist != "" {
		writeLine(&b, "")
		writeLine(&b, "## Automation Assist")
		writeField(&b, "Assist path", opts.AssistPath)
		writeLine(&b, "")
		writeLine(&b, assist)
	}
	if task.BusinessDiagnostics != nil {
		writeBusinessDiagnosticsPromptContext(&b, *task.BusinessDiagnostics)
	}

	writeLine(&b, "")
	writeLine(&b, "## Task Prompt")
	writeLine(&b, strings.TrimSpace(task.Prompt.Prompt))
	writeLine(&b, "")
	writeLine(&b, "## Output")
	writeLine(&b, "Return the final result in Markdown so it can be archived as result.md.")
	return strings.TrimSpace(b.String()) + "\n"
}

func writeBusinessDiagnosticsPromptContext(b *strings.Builder, cfg BusinessDiagnosticsConfig) {
	writeLine(b, "")
	writeLine(b, "## Business Diagnostics")
	writeField(b, "Target start", formatPromptTime(cfg.TargetStart))
	writeField(b, "Target end", formatPromptTime(cfg.TargetEnd))
	writeField(b, "Region", cfg.Region)
	if len(cfg.Recipients) > 0 {
		writeField(b, "Recipients", strings.Join(cfg.Recipients, ", "))
	}
	writeLine(b, "")
	writeLine(b, "Configured data sources:")
	for _, source := range cfg.Sources {
		var parts []string
		if path := strings.TrimSpace(source.Path); path != "" {
			parts = append(parts, "path="+path)
		}
		if url := strings.TrimSpace(source.URL); url != "" {
			parts = append(parts, "url="+url)
		}
		if format := strings.TrimSpace(source.Format); format != "" {
			parts = append(parts, "format="+format)
		}
		if len(source.Rows) > 0 {
			parts = append(parts, fmt.Sprintf("inlineRows=%d", len(source.Rows)))
		}
		line := fmt.Sprintf("- %s (%s/%s)", source.ID, source.Kind, source.Type)
		if len(parts) > 0 {
			line += ": " + strings.Join(parts, ", ")
		}
		writeLine(b, line)
		if len(source.Rows) > 0 {
			if rows, err := json.Marshal(source.Rows); err == nil {
				writeLine(b, "  rows: "+string(rows))
			}
		}
	}
	writeLine(b, "Use the listed files, URLs, or inline rows as the source of truth for the analysis.")
}

func formatPromptTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func writeField(b *strings.Builder, label, value string) {
	if value = strings.TrimSpace(value); value != "" {
		writeLine(b, fmt.Sprintf("%s: %s", label, value))
	}
}

func writeLine(b *strings.Builder, line string) {
	b.WriteString(line)
	b.WriteByte('\n')
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
