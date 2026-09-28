import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { ConnectorQuickMenu } from "../features/composer/ui/ConnectorQuickMenu";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
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

window.go = {
  main: {
    App: {
      MCPServers: async () => [],
      ConnectorCatalog: async () => ({
        catalog_version: "switch-popup-test",
        items: [
          { id: "canva", name: "Canva可画", provider: "Canva", description: "设计", category: "design", examples: [], kind: "mcp", version: "0.1.0", compatible: true, capabilities: ["mcp_tools"] },
          { id: "notion", name: "Notion", provider: "Notion", description: "文档", category: "productivity", examples: [], kind: "mcp", version: "0.1.0", compatible: true, capabilities: ["mcp_tools"] },
        ],
      }),
      ConnectorStates: async () => [
        { connector_id: "canva", status: "connected", tools: 4 },
        { connector_id: "notion", status: "authorization_required", tools: 0 },
      ],
    } as Partial<AppBindings> as AppBindings,
  },
};

const toggled: string[] = [];
const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
await act(async () => {
  root.render(
    <LocaleProvider>
      <ConnectorQuickMenu
        open
        tabId="switch-popup-tab"
        selectedNames={["canva"]}
        onToggle={(name) => toggled.push(name)}
        onMore={() => {}}
        onRequestClose={() => {}}
        variant="connected-switches"
      />
    </LocaleProvider>,
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
});

for (let attempt = 0; attempt < 20 && document.querySelectorAll(".composer-connector-menu__row").length === 0; attempt += 1) {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

ok(document.querySelectorAll(".composer-connector-menu__row").length === 1, "switch popup only shows connected connectors");
ok(document.body.textContent?.includes("Canva可画"), "switch popup shows the connected connector");
ok(!document.body.textContent?.includes("Notion"), "switch popup hides connectors that still require authorization");
ok(document.querySelector(".composer-connector-menu__head") === null, "switch popup has no catalog header");
ok(document.querySelector(".composer-connector-menu__more") === null, "switch popup has no catalog navigation");

await act(async () => {
  document.querySelector<HTMLInputElement>('.composer-connector-menu__switch input')?.click();
});
ok(toggled.join(",") === "canva", "switch popup toggles the connected connector");

await act(async () => root.unmount());
dom.window.close();

console.log("connector switch popover tests passed");
