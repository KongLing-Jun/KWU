package plugin

import (
	"context"

	"kwu/internal/tool"
)

type toolState struct {
	info         []ToolInfo
	runtime      []tool.Tool
	runtimeReady bool
}

func (c *Client) toolsFor(ctx context.Context) ([]tool.Tool, error) {
	if tools, ok := c.cachedRuntimeTools(); ok {
		return tools, nil
	}
	return c.listTools(ctx)
}

// cacheRuntimeTools runs while listTools holds toolsMu.
func (c *Client) cacheRuntimeTools(tools []tool.Tool) []tool.Tool {
	tools = sortToolsByName(tools)
	c.tools.runtime = append([]tool.Tool(nil), tools...)
	c.tools.runtimeReady = true
	return tools
}

func (c *Client) cachedRuntimeTools() ([]tool.Tool, bool) {
	c.toolsMu.Lock()
	defer c.toolsMu.Unlock()
	if !c.tools.runtimeReady {
		return nil, false
	}
	return append([]tool.Tool(nil), c.tools.runtime...), true
}
