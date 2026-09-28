import {
  CONNECTOR_CATALOG,
  CONNECTOR_CATEGORIES,
  USER_VISIBLE_CONNECTOR_CATALOG,
  connectorDisplayName,
  connectorEntriesForCategory,
  registerConnectorCatalog,
} from "../features/connectors/catalog";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

console.log("connector catalog contracts");

ok(CONNECTOR_CATEGORIES.length === 0, "the frontend does not own managed connector categories");
ok(CONNECTOR_CATALOG.length === 0, "the frontend does not own managed connector metadata");
ok(USER_VISIBLE_CONNECTOR_CATALOG === CONNECTOR_CATALOG, "the legacy static export remains empty");
ok(connectorEntriesForCategory("research").length === 0, "legacy category lookup cannot synthesize entries");

registerConnectorCatalog([{
  id: "unknown-lab-connector",
  name: "Lab Documents",
  provider: "Example Labs",
  description: "Dynamic connector",
  category: "research",
  icon_url: "https://example.test/icon.svg",
  examples: ["Read a document"],
  kind: "mcp",
  version: "1.0.0",
  compatible: true,
  capabilities: ["mcp_tools"],
}]);
ok(connectorDisplayName("unknown-lab-connector") === "Lab Documents", "display names come from the parsed Cloud Catalog");
ok(connectorDisplayName("never-published") === "never-published", "unknown IDs retain a safe fallback label");

console.log("connector catalog contracts passed");
