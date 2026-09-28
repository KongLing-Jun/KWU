export interface MCPToolEvidence {
  server: string;
  tool: string;
  label: string;
}

export function mcpToolEvidence(name: string): MCPToolEvidence | null {
  const match = /^mcp__([^_].*?)__(.+)$/.exec(name);
  if (!match) return null;
  const server = match[1];
  const tool = match[2];
  const connector = server
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.toLowerCase() === "github" ? "GitHub" : `${part[0]?.toUpperCase() ?? ""}${part.slice(1)}`)
    .join(" ");
  return { server, tool, label: `${connector} MCP · ${tool}` };
}
