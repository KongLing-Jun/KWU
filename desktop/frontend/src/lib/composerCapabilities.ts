import type { ComposerCapabilitySubmit, ServerView } from "./types";

export type ComposerCapabilitySelection = {
  skills?: string[];
  connectors?: string[];
};

export type ComposerCapabilitySelections = Record<string, ComposerCapabilitySelection>;
export type EnabledConnectorSelections = Record<string, string[]>;

const COMPOSER_CAPABILITIES_STORAGE_KEY = "kwu.composerCapabilities.v1";
const ENABLED_CONNECTORS_STORAGE_KEY = "kwu.enabledConnectors.v1";
export const MCP_SERVER_REMOVED_EVENT = "kwu:mcp-server-removed";

function safeCapabilityNames(names: readonly string[] | undefined): string[] {
  const unique = new Set<string>();
  for (const name of names ?? []) {
    const safeName = name.replace(/[<>&/\\\r\n]+/g, " ").replace(/\s+/g, " ").trim().slice(0, 120);
    if (safeName) unique.add(safeName);
  }
  return [...unique];
}

export function buildComposerCapabilitySubmit(
  userText: string,
  selection: ComposerCapabilitySelection,
  titleText?: string,
): ComposerCapabilitySubmit {
  return {
    text: userText,
    ...(titleText !== undefined ? { title_text: titleText } : {}),
    skill_names: safeCapabilityNames(selection.skills),
    mcp_server_names: safeCapabilityNames(selection.connectors),
  };
}

// Kept for call sites that only need the model-visible text. Capability names
// are deliberately not encoded into text; they travel through the structured
// bridge contract and are validated by the backend.
export function buildComposerCapabilitySubmitText(
  userText: string,
  _selection: ComposerCapabilitySelection,
): string {
  return userText;
}

export function isMCPPromptInvocation(userText: string): boolean {
  return userText.trimStart().startsWith("/mcp__");
}

export function mcpConnectTarget(userText: string): string | null {
  return /^\/mcp\s+connect\s+(\S+)\s*$/i.exec(userText.trim())?.[1] ?? null;
}

export async function ensureUsableMCPServer(
  name: string,
  listServers: () => Promise<ServerView[]>,
  reconnect: (name: string) => Promise<void>,
): Promise<ServerView> {
  const target = name.trim();
  if (!target) throw new Error("MCP connector name is required");
  let servers = await listServers();
  let server = servers.find((candidate) => candidate.name === target);
  if (server?.status !== "connected" || server.tools <= 0) {
    await reconnect(target);
    servers = await listServers();
    server = servers.find((candidate) => candidate.name === target);
  }
  if (!server || server.status !== "connected" || server.tools <= 0) {
    throw new Error(server?.error || `MCP connector ${target} did not expose any usable tools`);
  }
  return server;
}

export function isMCPManagementCommand(userText: string): boolean {
  const command = userText.trim();
  return /^\/mcp(?:\s+(?:list|show)(?:\s+\S+)?)?$/i.test(command) || mcpConnectTarget(command) !== null;
}

function normalizeCapabilitySelection(value: unknown): ComposerCapabilitySelection {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  const selection = value as { skills?: unknown; connectors?: unknown };
  return {
    connectors: safeCapabilityNames(Array.isArray(selection.connectors) ? selection.connectors.filter((name): name is string => typeof name === "string") : []),
  };
}

export function parseComposerCapabilitySelections(raw: string | null): ComposerCapabilitySelections {
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    const selections: ComposerCapabilitySelections = {};
    for (const [key, value] of Object.entries(parsed)) {
      if (!key) continue;
      const selection = normalizeCapabilitySelection(value);
      if ((selection.connectors?.length ?? 0) > 0) {
        selections[key] = selection;
      }
    }
    return selections;
  } catch {
    return {};
  }
}

export function loadComposerCapabilitySelections(): ComposerCapabilitySelections {
  if (typeof localStorage === "undefined") return {};
  try {
    return parseComposerCapabilitySelections(localStorage.getItem(COMPOSER_CAPABILITIES_STORAGE_KEY));
  } catch {
    return {};
  }
}

export function saveComposerCapabilitySelections(selections: ComposerCapabilitySelections): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(COMPOSER_CAPABILITIES_STORAGE_KEY, JSON.stringify(selections));
  } catch {
    /* ignore unavailable or full storage */
  }
}

export function removeConnectorFromCapabilitySelections(
  selections: ComposerCapabilitySelections,
  name: string,
): ComposerCapabilitySelections {
  const target = name.trim();
  if (!target) return selections;
  let changed = false;
  const next = Object.fromEntries(
    Object.entries(selections).map(([key, selection]) => {
      const connectors = selection.connectors ?? [];
      if (!connectors.includes(target)) return [key, selection];
      changed = true;
      return [key, { ...selection, connectors: connectors.filter((connector) => connector !== target) }];
    }),
  );
  return changed ? next : selections;
}

export function parseEnabledConnectorSelections(raw: string | null): EnabledConnectorSelections {
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    return Object.fromEntries(Object.entries(parsed)
      .filter(([scope]) => Boolean(scope))
      .map(([scope, names]) => [scope, safeCapabilityNames(Array.isArray(names)
        ? names.filter((name): name is string => typeof name === "string")
        : [])]));
  } catch {
    return {};
  }
}

export function loadEnabledConnectorSelections(): EnabledConnectorSelections {
  if (typeof localStorage === "undefined") return {};
  try {
    return parseEnabledConnectorSelections(localStorage.getItem(ENABLED_CONNECTORS_STORAGE_KEY));
  } catch {
    return {};
  }
}

export function saveEnabledConnectorSelections(selections: EnabledConnectorSelections): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(ENABLED_CONNECTORS_STORAGE_KEY, JSON.stringify(selections));
  } catch {
    /* ignore unavailable or full storage */
  }
}

export function removeConnectorFromEnabledSelections(
  selections: EnabledConnectorSelections,
  name: string,
): EnabledConnectorSelections {
  const target = name.trim();
  if (!target) return selections;
  let changed = false;
  const next = Object.fromEntries(Object.entries(selections).map(([scope, names]) => {
    if (!names.includes(target)) return [scope, names];
    changed = true;
    return [scope, names.filter((connector) => connector !== target)];
  }));
  return changed ? next : selections;
}

export function announceMCPServerRemoved(name: string): void {
  const trimmed = name.trim();
  if (!trimmed || typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent<string>(MCP_SERVER_REMOVED_EVENT, { detail: trimmed }));
}
