import { CONNECTOR_CATALOG } from "../features/connectors/catalog";
import { resolveConnectorAvailability } from "../lib/connectorAvailability";
import { mcpToolEvidence } from "../lib/mcpToolEvidence";
import type { ServerView } from "../lib/types";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

function server(name: string, status: ServerView["status"], tools: number): ServerView {
  return {
    name,
    status,
    tools,
    prompts: 0,
    resources: 0,
    transport: "stdio",
    configured: true,
    autoStart: true,
  };
}

for (const entry of CONNECTOR_CATALOG.filter((item) => item.preset)) {
  const name = entry.preset!.name;
  ok(resolveConnectorAvailability(name, []).state === "uninstalled", `${entry.id} reports uninstalled without config`);
  ok(
    resolveConnectorAvailability(name, [server(name, "failed", 0)]).state === "configuration_error",
    `${entry.id} distinguishes configured failures`,
  );
  ok(
    !resolveConnectorAvailability(name, [server(name, "connected", 0)]).selectable,
    `${entry.id} cannot be selected without discovered tools`,
  );
  const connected = resolveConnectorAvailability(name, [server(name, "connected", 8)]);
  ok(connected.selectable && connected.label.includes("8"), `${entry.id} can be selected only after tool discovery`);
}

const evidence = mcpToolEvidence("mcp__github__get_pull_request");
ok(evidence?.label === "GitHub MCP · get_pull_request", "MCP calls expose connector and tool evidence");
ok(mcpToolEvidence("bash") === null, "ordinary tools are not mislabeled as MCP calls");

console.log("connector availability tests passed");
