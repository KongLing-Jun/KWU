import type { ConnectorCatalogItemView } from "../../lib/bridge";
import type { MCPServerInput } from "../../lib/types";

// The public Cloud Catalog is the only product connector metadata source.
// These types remain for the separate custom-MCP catalog card implementation;
// they do not contain product entries.
export type ConnectorCategory = string;
export type ConnectorSetup = "direct" | "requires-config" | "guide-only";
export type ConnectorMaturity = "official" | "preview" | "community";
export type ConnectorAuthMode = "none" | "credentials" | "oauth";

export interface ConnectorCatalogEntry {
  id: string;
  category: ConnectorCategory;
  name: string;
  provider: string;
  description: string;
  authMode: ConnectorAuthMode;
  setup: ConnectorSetup;
  maturity: ConnectorMaturity;
  docsUrl: string;
  note: string;
  tags: string[];
  examplePrompt?: string;
  preset?: MCPServerInput;
  requiredEnvKeys?: string[];
  requiredPlainEnvKeys?: string[];
  requiredHeaderKeys?: string[];
  unresolvedCommandTokens?: string[];
  conflictGroup?: "browser-automation";
}

export const CONNECTOR_CATEGORIES: readonly string[] = [];
export const CONNECTOR_CATALOG: readonly ConnectorCatalogEntry[] = [];
export const USER_VISIBLE_CONNECTOR_CATALOG = CONNECTOR_CATALOG;

const displayNames = new Map<string, string>();
const catalogItems = new Map<string, ConnectorCatalogItemView>();

export function registerConnectorCatalog(items: readonly ConnectorCatalogItemView[]): void {
  displayNames.clear();
  catalogItems.clear();
  for (const item of items) {
    displayNames.set(item.id, item.name);
    catalogItems.set(item.id, item);
  }
}

export function connectorDisplayName(connectorID: string): string {
  return displayNames.get(connectorID) ?? connectorID;
}

export function connectorCatalogItem(connectorID: string): ConnectorCatalogItemView | undefined {
  return catalogItems.get(connectorID);
}

export function connectorEntriesForCategory(_category: ConnectorCategory): readonly ConnectorCatalogEntry[] {
  return [];
}
