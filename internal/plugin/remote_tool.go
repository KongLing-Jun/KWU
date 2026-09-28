package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// remoteTool adapts an MCP server tool to the agent's common tool interface.
type remoteTool struct {
	client   *Client
	name     string // namespaced "mcp__<server>__<tool>"
	rawName  string // original name for tools/call
	desc     string
	schema   json.RawMessage
	readOnly bool // from MCP readOnlyHint or trusted first-party Spec override
	// readOnlyTrusted is true only when readOnly came from a first-party
	// Spec.ReadOnlyToolNames override, not the server's readOnlyHint. Plan mode
	// uses it to decide whether to trust ReadOnly() at face value.
	readOnlyTrusted bool
}

func (t *remoteTool) Name() string        { return t.name }
func (t *remoteTool) Description() string { return t.desc }

// ReadOnly reflects MCP readOnlyHint, plus trusted first-party Spec overrides.
// It defaults to false: opaque third-party tools must declare readOnlyHint
// before joining reader-default permission handling or plan-mode execution.
func (t *remoteTool) ReadOnly() bool { return t.readOnly }

// PlanModeUntrustedReadOnly reports true when ReadOnly() is true only because the
// MCP server self-reported readOnlyHint. A first-party ReadOnlyToolNames override
// is trusted, so it returns false. Plan mode treats an untrusted read-only tool
// like a writer unless it is declared in plan_mode_allowed_tools.
func (t *remoteTool) PlanModeUntrustedReadOnly() bool {
	return t.readOnly && !t.readOnlyTrusted
}

func (t *remoteTool) Schema() json.RawMessage {
	if len(t.schema) == 0 {
		return json.RawMessage(`{"type":"object"}`)
	}
	return canonicalizeSchema(t.schema)
}

func (t *remoteTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var argMap map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &argMap); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
	}
	res, err := t.client.call(ctx, "tools/call", map[string]any{
		"name":      t.rawName,
		"arguments": argMap,
	})
	if err != nil {
		return "", err
	}
	return parseToolResult(res)
}

// parseToolResult flattens an MCP tools/call result into plain text. MCP tools
// may return structuredContent without duplicating it into a text block, so the
// structured payload is retained as JSON when no text content is present.
func parseToolResult(res json.RawMessage) (string, error) {
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		IsError           bool            `json:"isError"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return "", fmt.Errorf("decode tool result: %w", err)
	}
	var sb strings.Builder
	for _, content := range out.Content {
		if content.Type == "text" {
			sb.WriteString(content.Text)
		}
	}
	text := sb.String()
	if text == "" && len(out.StructuredContent) > 0 && string(out.StructuredContent) != "null" {
		text = string(out.StructuredContent)
	}
	if out.IsError {
		return text, fmt.Errorf("plugin tool reported error: %s", text)
	}
	return text, nil
}
