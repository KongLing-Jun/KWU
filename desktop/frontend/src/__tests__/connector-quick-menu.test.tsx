import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { ConnectorQuickMenu } from "../features/composer/ui/ConnectorQuickMenu";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { ServerView } from "../lib/types";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    await act(async () => {
      await flush();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

const connected = (name: string, tools: number): ServerView => ({
  name,
  status: "connected",
  tools,
  prompts: 0,
  resources: 0,
  transport: "stdio",
  configured: true,
  autoStart: true,
});
let servers: ServerView[] = [
  connected("unknown-lab-mcp--documents", 2),
  connected("antv-chart", 8),
  connected("custom-mcp", 3),
  {
    name: "playwright",
    status: "failed",
    tools: 0,
    prompts: 0,
    resources: 0,
    transport: "stdio",
    configured: true,
    autoStart: true,
    error: "process exited",
  },
];
const reconnectCalls: string[] = [];
const installCalls: string[] = [];
const toggled: string[] = [];
let mcpServerReads = 0;
let moreCalls = 0;
let closeCalls = 0;

window.go = {
  main: {
    App: {
      MCPServers: async () => {
        mcpServerReads += 1;
        return servers;
      },
      ConnectorStates: async () => [{
        connector_id: "unknown-lab-skills",
        status: "connected",
      }],
      ConnectorCatalog: async () => ({
        catalog_version: "dynamic-test",
        items: [{
          id: "unknown-lab-skills", name: "Lab Skills", provider: "Example Labs", description: "Dynamic Skills",
          category: "research", examples: ["Summarize this"], kind: "skills", version: "1.0.0", compatible: true,
          capabilities: ["skills"],
        }, {
          id: "unknown-lab-mcp", name: "Lab Documents", provider: "Example Labs", description: "Dynamic MCP",
          category: "research", examples: ["Read this"], kind: "mcp", version: "1.0.0", compatible: true,
          capabilities: ["mcp_tools"],
        }],
      }),
      InstallMCPServer: async (input) => {
        installCalls.push(input.name);
        servers = [...servers.filter((server) => server.name !== input.name), connected(input.name, 9)];
        return {
          name: input.name,
          status: "connected",
          phase: "complete",
          latencyMs: 12,
          tools: 9,
          prompts: 0,
          resources: 0,
          sessionAttached: true,
        };
      },
      ReconnectMCPServer: async (name: string) => {
        reconnectCalls.push(name);
        servers = servers.map((server) => server.name === name ? connected(name, 4) : server);
      },
      SetMCPServerEnabled: async () => {},
    } as Partial<AppBindings> as AppBindings,
  },
};

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
await act(async () => {
  root.render(
    <LocaleProvider>
      <ConnectorQuickMenu
        open
        tabId="connector-quick-tab"
        selectedNames={["antv-chart", "removed-server"]}
        onToggle={(name) => toggled.push(name)}
        onMore={() => {
          moreCalls += 1;
        }}
        onRequestClose={() => {
          closeCalls += 1;
        }}
      />
    </LocaleProvider>,
  );
  await flush();
});
await waitFor(
  "configured connector list",
  () => document.querySelectorAll(".composer-connector-menu__row").length === servers.length + 2,
);
ok(document.querySelector(".composer-connector-menu")?.textContent?.includes("Lab Skills"), "quick list includes dynamic Skills connector");
ok(document.querySelector(".composer-connector-menu")?.textContent?.includes("Lab Documents"), "quick list includes dynamic MCP connector");
ok(
  !Array.from(document.querySelectorAll(".composer-connector-menu__row"))
    .some((row) => row.textContent?.includes("memory")),
  "quick list does not synthesize hidden unconfigured presets",
);
ok(
  Array.from(document.querySelectorAll(".composer-connector-menu__row"))
    .some((row) => row.textContent?.includes("custom-mcp")),
  "configured custom MCP remains visible in the quick list",
);
ok(!document.querySelector(".composer-connector-menu")?.textContent?.includes("unknown-lab-skills"), "dynamic connector ID is not shown to users");
ok(!document.querySelector(".composer-connector-menu")?.textContent?.includes("unknown-lab-mcp"), "dynamic MCP ID is not shown to users");
const skillsSwitch = document.querySelector<HTMLInputElement>(
  '.composer-connector-menu__row input[aria-label*="Lab Skills"]',
);
ok(skillsSwitch?.checked === false, "connected dynamic Skills connector is available to the current task");
await act(async () => {
  skillsSwitch?.click();
  await flush();
});
ok(toggled.includes("unknown-lab-skills"), "dynamic display name preserves its connector ID when selected");

const readsBeforeRefresh = mcpServerReads;
await act(async () => {
  document.querySelector<HTMLButtonElement>(".composer-connector-menu__refresh")?.click();
  await flush();
});
await waitFor(
  "observable real refresh",
  () => mcpServerReads > readsBeforeRefresh
    && document.querySelector(".composer-connector-menu__head")?.textContent?.includes(`已刷新 ${servers.length + 1} 个连接器`) === true,
);

const antVSwitch = document.querySelector<HTMLInputElement>(
  '.composer-connector-menu__row input[aria-label*="antv-chart"]',
);
ok(antVSwitch?.checked === true, "quick list exposes the current task selection");
await act(async () => {
  antVSwitch?.click();
  await flush();
});
ok(toggled.includes("antv-chart"), "quick list toggles a connected connector for the current task");

const customSwitch = document.querySelector<HTMLInputElement>(
  '.composer-connector-menu__row input[aria-label*="custom-mcp"]',
);
ok(customSwitch?.checked === false, "custom MCP starts available but disabled for the current task");
await act(async () => {
  customSwitch?.click();
  await flush();
});
ok(toggled.includes("custom-mcp"), "configured custom MCP can be enabled for the current task");
ok(installCalls.length === 0, "hidden presets cannot be installed from the quick list");

const playwrightRow = Array.from(document.querySelectorAll<HTMLElement>(".composer-connector-menu__row"))
  .find((row) => row.textContent?.includes("playwright"));
const retry = playwrightRow?.querySelector<HTMLButtonElement>(".composer-connector-menu__action");
if (!retry) throw new Error("missing retry action");
await act(async () => {
  retry.click();
  await flush();
});
await waitFor("real reconnect", () => reconnectCalls.includes("playwright"));
ok(!toggled.includes("playwright"), "successful connection does not automatically enable the connector");
ok(
  Array.from(document.querySelectorAll(".composer-connector-menu__row"))
    .find((row) => row.textContent?.includes("playwright"))
    ?.textContent?.includes("4") === true,
  "successful retry refreshes the list from MCPServers and exposes discovered tools",
);

const removedRow = Array.from(document.querySelectorAll<HTMLElement>(".composer-connector-menu__row"))
  .find((row) => row.textContent?.includes("removed-server"));
await act(async () => {
  removedRow?.querySelector<HTMLButtonElement>(".composer-connector-menu__action")?.click();
  await flush();
});
ok(toggled.includes("removed-server"), "stale task selection can be removed directly from the quick list");

await act(async () => {
  document.querySelector<HTMLButtonElement>(".composer-connector-menu__more")?.click();
  await flush();
});
ok(closeCalls === 1 && moreCalls === 1, "only More connectors closes the quick list and opens the full workspace");

await act(async () => root.unmount());
dom.window.close();

console.log("connector quick menu tests passed");
