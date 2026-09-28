import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { disconnectManagedConnectorSelectionNames, MCPServersSettingsPage } from "../components/CapabilitiesPanel";
import { useAccountStore } from "../features/account/store/accountStore";
import type { AppBindings } from "../lib/bridge";
import { MCP_SERVER_REMOVED_EVENT, removeConnectorFromCapabilitySelections } from "../lib/composerCapabilities";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { MCPServerInput, Meta, ServerView, TabMeta } from "../lib/types";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function setInputValue(input: HTMLInputElement, value: string) {
  const win = input.ownerDocument.defaultView;
  const valueSetter = win ? Object.getOwnPropertyDescriptor(win.HTMLInputElement.prototype, "value")?.set : undefined;
  if (valueSetter) valueSetter.call(input, value);
  else input.value = value;
  const reactPropsKey = Object.keys(input).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (input as unknown as Record<string, { onChange?: (event: { target: HTMLInputElement }) => void }>)[reactPropsKey]
    : undefined;
  reactProps?.onChange?.({ target: input });
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    await act(async () => {
      await flush();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><div id=\"capability-workspace-header-actions\"></div><div id=\"root\"></div></body></html>", {
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
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.CustomEvent = dom.window.CustomEvent;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: () => ({
      matches: true,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    }),
  });
  return dom;
}

console.log("connector catalog UI");

const dom = installDom();
const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
const meta: Meta = {
  label: "test",
  ready: true,
  eventChannel: "connector-catalog-channel",
  cwd: "/tmp/connector-catalog",
  workspaceRoot: "/tmp/connector-catalog",
};
const tabs: TabMeta[] = [{
  id: "connector-catalog-tab",
  scope: "project",
  workspaceRoot: "/tmp/connector-catalog",
  workspaceName: "connector-catalog",
  topicId: "connector-catalog-topic",
  topicTitle: "Connector catalog",
  label: "Connector catalog",
  ready: true,
  running: false,
  mode: "normal",
  toolApprovalMode: "auto",
  active: true,
  cwd: "/tmp/connector-catalog",
}];
const addCalls: MCPServerInput[] = [];
const selectedCalls: string[] = [];
const tryCalls: Array<{ name: string; prompt: string }> = [];
let connectorStateCalls = 0;
let connectorInstallCalls = 0;
const removedConnectorEvents: string[] = [];
window.addEventListener(MCP_SERVER_REMOVED_EVENT, (event) => {
  removedConnectorEvents.push((event as CustomEvent<string>).detail);
});
let dynamicSkillsConnected = false;
let dynamicMCPConnected = true;
let tokenConnectValues: Record<string, string> | null = null;
let tokenConnectAttempts = 0;
let tokenUnhandledRejections = 0;
window.addEventListener("unhandledrejection", () => { tokenUnhandledRejections += 1; });
let servers: ServerView[] = [{
  name: "antv-chart",
  status: "connected",
  tools: 8,
  prompts: 0,
  resources: 0,
  transport: "stdio",
  configured: true,
  autoStart: true,
}, {
  name: "custom-mcp",
  status: "connected",
  tools: 3,
  prompts: 0,
  resources: 0,
  transport: "stdio",
  configured: true,
  autoStart: true,
}, {
  name: "unknown-lab-mcp--server",
  status: "connected",
  tools: 2,
  prompts: 0,
  resources: 0,
  transport: "streamable-http",
  configured: true,
  autoStart: true,
}];

window.go = {
  main: {
    App: {
      Meta: async () => meta,
      ListTabs: async () => tabs,
      MCPServers: async () => servers,
      ConnectorCatalog: async () => ({
        catalog_version: "dynamic-test",
        items: [{
          id: "unknown-lab-skills",
          name: "Lab Skills",
          provider: "Example Labs",
          description: "Dynamic Skills connector",
          category: "research",
          icon_url: "https://example.test/skills.svg",
          examples: ["Summarize this document"],
          kind: "composite",
          version: "1.0.0",
          compatible: true,
          capabilities: ["remote_mcp", "skills"],
        }, {
          id: "unknown-lab-mcp",
          name: "Lab Documents",
          provider: "Example Labs",
          description: "Dynamic MCP connector",
          category: "research",
          examples: ["Read this document"],
          kind: "mcp",
          version: "1.0.0",
          compatible: true,
          capabilities: ["mcp_tools"],
        }],
      }),
      ConnectorStates: async () => {
        connectorStateCalls += 1;
        return [{
          connector_id: "unknown-lab-skills",
          status: dynamicSkillsConnected ? "connected" : "authorization_required",
        }, {
          connector_id: "unknown-lab-mcp",
          status: dynamicMCPConnected ? "connected" : "authorization_required",
          tools: dynamicMCPConnected ? 2 : 0,
        }];
      },
      InstallConnector: async (_tabID: string, connectorID: string) => {
        connectorInstallCalls += 1;
        return { connector_id: connectorID, status: "authorization_required" };
      },
      ConnectorAuthFields: async (_tabID: string, connectorID: string) => connectorID === "unknown-lab-skills" ? [{
        key: "remote.api_key",
        server_id: "remote",
        id: "api_key",
        label: "API key",
        type: "secret",
        required: true,
        documentation_url: "https://docs.example.test/tokens",
        documentation_label: "Get an API key",
      }] : [],
      ConnectConnector: async (_tabID: string, connectorID: string, values: Record<string, string>) => {
        if (connectorID === "unknown-lab-skills") {
          tokenConnectAttempts += 1;
          tokenConnectValues = { ...values };
          if (tokenConnectAttempts === 1) throw new Error("acceptance token connect failure");
          dynamicSkillsConnected = true;
        }
        return { connector_id: connectorID, status: "connected" };
      },
      DisconnectConnector: async (_tabID: string, connectorID: string) => {
        if (connectorID === "unknown-lab-skills") throw new Error("acceptance disconnect failure");
        dynamicMCPConnected = false;
        servers = servers.filter((server) => server.name !== "unknown-lab-mcp--server");
        return {
          connector_id: connectorID,
          status: "authorization_required",
        };
      },
			CancelConnectorAuthorization: async () => {},
      AddMCPServer: async () => 0,
      InstallMCPServer: async (input: MCPServerInput) => {
        addCalls.push(input);
        if (input.name === "fetch") {
          return {
            name: input.name,
            status: "failed",
            phase: "dependency",
            latencyMs: 0,
            tools: 0,
            resources: 0,
            prompts: 0,
            sessionAttached: false,
            error: "missing uvx runtime",
            repairAction: "install uvx and retry",
          };
        }
        servers = [
          ...servers.filter((server) => server.name !== input.name),
          {
            name: input.name,
            status: "connected",
            tools: 8,
            prompts: 1,
            resources: 2,
            transport: input.transport,
            configured: true,
            autoStart: true,
          },
        ];
        return {
          name: input.name,
          status: "connected",
          phase: "complete",
          latencyMs: 126,
          tools: 8,
          resources: 2,
          prompts: 1,
          sessionAttached: true,
        };
      },
      ReconnectMCPServer: async () => {},
      SetMCPServerEnabled: async () => {},
    } as Partial<AppBindings> as AppBindings,
  },
};

useAccountStore.setState({
  initialized: true,
  available: true,
  account: { status: "anonymous", profile_id: "default" },
});

await act(async () => {
  root.render(
    <LocaleProvider>
      <ToastProvider>
        <MCPServersSettingsPage
          workspace
          selectedServers={["unknown-lab-mcp", "unknown-lab-mcp--server"]}
          onSelectServer={(name) => selectedCalls.push(name)}
          onTryConnector={(name, prompt) => tryCalls.push({ name, prompt })}
        />
      </ToastProvider>
    </LocaleProvider>,
  );
  await flush();
});
await waitFor("connector catalog", () => Boolean(document.querySelector(".connector-catalog__grid")));

ok(Boolean(document.querySelector(".connector-workspace-add--catalog")), "catalog stays visible in the connector workspace");
ok(connectorStateCalls === 0, "anonymous connector catalog skips owner-scoped state lookup");
ok(document.body.textContent?.includes("未登录时可浏览连接器；登录后可连接和使用。"), "anonymous catalog explains the login boundary");
const anonymousCard = document.querySelector<HTMLElement>('[data-connector-id="unknown-lab-skills"]');
const anonymousOpen = anonymousCard?.querySelector<HTMLButtonElement>(".connector-catalog__card-main");
if (!anonymousOpen) throw new Error("missing anonymous connector card");
await act(async () => {
  anonymousOpen.click();
  await flush();
});
const anonymousConnect = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__actions .btn--primary");
if (!anonymousConnect) throw new Error("missing anonymous connector action");
await act(async () => {
  anonymousConnect.click();
  await flush();
});
ok(connectorInstallCalls === 0, "anonymous connect does not start installation");
ok(document.querySelector(".connector-catalog-dialog__error")?.textContent?.includes("请先登录云智企办"), "anonymous connect shows the login prompt");
await act(async () => {
  document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__close")?.click();
  useAccountStore.setState({
    initialized: true,
    available: true,
    account: { status: "authenticated", profile_id: "default", cloud_user_id: "catalog-user", binding_generation: 1 },
  });
  await flush();
});
await waitFor("connector state after login", () => connectorStateCalls > 0);
ok(!document.body.textContent?.includes("未登录时可浏览连接器"), "authenticated catalog removes the login notice");
await waitFor("authenticated connector toolbar", () => Boolean(document.querySelector('.connector-workspace-topbar__search input[type="search"]')));
const searchInput = document.querySelector<HTMLInputElement>('.connector-workspace-topbar__search input[type="search"]');
if (!searchInput) throw new Error("missing connector search input");
await act(async () => {
  setInputValue(searchInput, "Lab Documents");
  await flush();
});
ok(document.querySelectorAll(".connector-catalog__card").length === 1, "connector search filters the dynamic catalog");
await act(async () => {
  setInputValue(searchInput, "");
  await flush();
});
ok(!document.querySelector(".connector-workspace-topbar__action"), "connector workspace hides the custom MCP creation entry");
ok(!document.querySelector(".connector-workspace-add--custom"), "connector workspace does not render the custom MCP form");

ok(document.querySelectorAll(".connector-catalog__card").length === 3, "catalog renders Feishu beside both unknown non-product connectors");
ok(
  Boolean(document.querySelector('[data-connector-id="feishu-bot"] .connector-catalog__card-main .connector-brand-icon')),
  "Feishu uses the same native card and centered brand icon as the rest of the connector catalog",
);
ok(document.querySelector<HTMLElement>('[data-connector-id="unknown-lab-skills"]')?.textContent?.includes("Lab Skills"), "Skills card uses Cloud display metadata");
ok(document.querySelector<HTMLElement>('[data-connector-id="unknown-lab-mcp"]')?.textContent?.includes("Lab Documents"), "MCP card uses Cloud display metadata");
ok(!document.querySelector('[data-connector-id="antv-chart"]'), "removed catalog connectors are not rendered as managed cards");
ok(addCalls.length === 0, "opening the managed catalog does not install an MCP preset");
ok(
  Array.from(document.querySelectorAll(".cap-server-entry")).some((row) => row.textContent?.includes("custom-mcp")),
  "generic configured MCP entries remain generic until removed through their normal action",
);
ok(
  !Array.from(document.querySelectorAll(".cap-server-entry")).some((row) => row.textContent?.includes("unknown-lab-mcp--server")),
  "a package-managed MCP runtime stays represented by its single catalog card",
);

const tokenCard = document.querySelector<HTMLElement>('[data-connector-id="unknown-lab-skills"]');
const tokenOpen = tokenCard?.querySelector<HTMLButtonElement>(".connector-catalog__card-main");
if (!tokenOpen) throw new Error("missing token connector card action");
await act(async () => {
  tokenOpen.click();
  await flush();
});
const tokenDialog = document.querySelector<HTMLElement>(".connector-catalog-dialog");
const tokenClose = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__close");
ok(tokenClose?.getAttribute("aria-label") === "关闭", "connector details expose an accessible close control");
if (!tokenDialog || !tokenClose) throw new Error("missing connector detail close control");
await act(async () => {
  tokenDialog.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  await flush();
});
ok(Boolean(document.querySelector(".connector-catalog-dialog")), "clicking inside the dialog does not close connector details");
await act(async () => {
  tokenClose.click();
  await flush();
});
ok(!document.querySelector(".connector-catalog-dialog"), "the close control dismisses connector details");
await act(async () => {
  tokenOpen.click();
  await flush();
});
const tokenBackdrop = document.querySelector<HTMLElement>(".connector-catalog-dialog-backdrop");
if (!tokenBackdrop) throw new Error("missing connector detail backdrop");
await act(async () => {
  tokenBackdrop.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  await flush();
});
ok(!document.querySelector(".connector-catalog-dialog"), "clicking the backdrop closes connector details");
await act(async () => {
  tokenOpen.click();
  await flush();
});
const tokenConnect = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__actions .btn--primary");
if (!tokenConnect) throw new Error("missing token connector action");
await act(async () => {
  tokenConnect.click();
  await flush();
});
await waitFor("token credential form", () => Boolean(document.querySelector('[aria-label="Connector token credentials"]')));
const tokenForm = document.querySelector<HTMLElement>('[aria-label="Connector token credentials"]');
const tokenInput = tokenForm?.querySelector<HTMLInputElement>('input[type="password"]');
ok(tokenForm?.textContent?.includes("Get an API key"), "token form shows the declared documentation label");
ok(tokenForm?.querySelectorAll("input").length === 1, "token form renders only the declared minimum fields");
ok(tokenInput?.type === "password", "secret token field is masked by default");
if (!tokenInput) throw new Error("missing token secret input");
await act(async () => {
  setInputValue(tokenInput, "front-end-secret");
  await flush();
});
const tokenSubmit = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__actions .btn--primary");
if (!tokenSubmit) throw new Error("missing token submit action");
await act(async () => {
  tokenSubmit.click();
  await flush();
});
await waitFor("token connection failure", () => Boolean(document.querySelector(".toast--error")));
const retainedTokenForm = document.querySelector<HTMLElement>('[aria-label="Connector token credentials"]');
const retainedTokenInput = retainedTokenForm?.querySelector<HTMLInputElement>('input[type="password"]');
ok(Boolean(retainedTokenForm), "failed token connection keeps the credential form open");
ok(retainedTokenInput?.value === "front-end-secret", "failed token connection preserves the entered secret");
ok(
  document.querySelector(".toast--error")?.textContent?.includes("暂时无法连接"),
  "failed token connection shows one lightweight error toast",
);
ok(document.querySelectorAll(".toast--error").length === 1, "one failed connection produces one error presentation");
ok(!document.querySelector(".connector-workspace-add--catalog > .banner--error"), "connector failures do not occupy the catalog with a page banner");
ok(tokenUnhandledRejections === 0, "failed token connection does not emit unhandledrejection");
if (!retainedTokenInput) throw new Error("missing retained token secret input");
await act(async () => {
  setInputValue(retainedTokenInput, "front-end-secret-retry");
  await flush();
});
const retryTokenSubmit = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__actions .btn--primary");
if (!retryTokenSubmit) throw new Error("missing retained token submit action");
await act(async () => {
  retryTokenSubmit.click();
  await flush();
});
await waitFor("token form dismissal", () => !document.querySelector('[aria-label="Connector token credentials"]'));
ok(tokenConnectAttempts === 2, "token connection retries through the real catalog component chain");
ok(tokenConnectValues?.["remote.api_key"] === "front-end-secret-retry", "the successful retry sends the edited token value");
ok(!document.body.textContent?.includes("front-end-secret-retry"), "successful token value is not retained in rendered frontend state");
ok(document.querySelectorAll(".toast--success").length === 1, "one successful connection produces one lightweight success toast");
ok(document.querySelector(".toast--success")?.textContent?.includes("Lab Skills 已连接"), "success toast identifies the connected service without repeating guidance");

const dynamicCard = document.querySelector<HTMLElement>('[data-connector-id="unknown-lab-mcp"]');
const dynamicOpen = dynamicCard?.querySelector<HTMLButtonElement>(".connector-catalog__card-main");
if (!dynamicOpen) throw new Error("missing dynamic connector card action");
await act(async () => {
  dynamicOpen.click();
  await flush();
});
ok(!document.querySelector(".connector-catalog-dialog__connected-status"), "connected connector details do not repeat success in a persistent status card");
ok(document.querySelectorAll(".toast--success").length === 1, "opening an already connected connector does not repeat the success toast");
ok(!document.querySelector(".connector-catalog-dialog__title-line .connector-catalog__connected-dot"), "connected details avoid a redundant status dot beside the title");
const dynamicTry = Array.from(document.querySelectorAll<HTMLButtonElement>(".connector-catalog-dialog__actions button"))
  .find((button) => button.textContent?.includes("在对话中试用"));
ok(dynamicTry?.classList.contains("btn--primary"), "connected connector details emphasize the conversation action");
let dynamicDisconnect = Array.from(document.querySelectorAll<HTMLButtonElement>(".connector-catalog-dialog__actions button"))
  .find((button) => button.textContent?.includes("解绑"));
if (!dynamicDisconnect) throw new Error("missing dynamic connector disconnect action");
await act(async () => {
  dynamicDisconnect!.click();
  await flush();
});
dynamicDisconnect = Array.from(document.querySelectorAll<HTMLButtonElement>(".connector-catalog-dialog__actions button"))
  .find((button) => button.textContent?.includes("确认解绑"));
if (!dynamicDisconnect) throw new Error("missing dynamic connector disconnect confirmation");
await act(async () => {
  dynamicDisconnect!.click();
  await flush();
});
await waitFor("dynamic connector removal events", () => removedConnectorEvents.includes("unknown-lab-mcp--server"));
ok(removedConnectorEvents.filter((name) => name === "unknown-lab-mcp").length === 1, "dynamic disconnect clears the catalog selection once");
ok(removedConnectorEvents.filter((name) => name === "unknown-lab-mcp--server").length === 1, "dynamic disconnect clears the exact runtime selection once");

const initialSelections = {
  "session-a": { connectors: ["unknown-lab-mcp", "unknown-lab-mcp--server", "custom-mcp"] },
  "session-b": { connectors: ["unknown-lab-mcp--server"] },
  "session-c": { connectors: ["custom-mcp"] },
};
const clearedSelections = removedConnectorEvents.reduce(removeConnectorFromCapabilitySelections, initialSelections);
ok(clearedSelections["session-a"].connectors?.join(",") === "custom-mcp", "dynamic disconnect clears catalog and runtime selections from the current task");
ok(clearedSelections["session-b"].connectors?.length === 0, "dynamic disconnect clears the runtime selection from other tasks");
ok(clearedSelections["session-c"].connectors?.join(",") === "custom-mcp", "dynamic disconnect preserves unrelated MCP selections");

const eventCountBeforeFailure = removedConnectorEvents.length;
let disconnectFailed = false;
try {
  const removedNames = await disconnectManagedConnectorSelectionNames(
    "unknown-lab-skills",
    [{
      name: "unknown-lab-skills--server",
      status: "connected",
      tools: 1,
      prompts: 0,
      resources: 0,
      transport: "streamable-http",
      configured: true,
      autoStart: true,
    }],
    async () => { throw new Error("acceptance disconnect failure"); },
  );
  removedNames.forEach((name) => window.dispatchEvent(new CustomEvent<string>(MCP_SERVER_REMOVED_EVENT, { detail: name })));
} catch {
  disconnectFailed = true;
}
ok(disconnectFailed, "failed dynamic disconnect propagates its backend error");
ok(removedConnectorEvents.length === eventCountBeforeFailure, "failed dynamic disconnect does not clear task selections");

await act(async () => {
  root.unmount();
});
dom.window.close();

const concurrentDom = installDom();
const concurrentRootElement = document.getElementById("root");
if (!concurrentRootElement) throw new Error("missing concurrent root");
const concurrentRoot = createRoot(concurrentRootElement);
const concurrentStarted: string[] = [];
const concurrentCancelled: string[] = [];
const concurrentResolvers = new Map<string, { resolve: () => void; reject: (error: Error) => void }>();
const concurrentConnected = new Set<string>();
const concurrentAuthorizing = new Set<string>();
window.go = {
  main: {
    App: {
      Meta: async () => meta,
      ListTabs: async () => tabs,
      MCPServers: async () => [],
      ConnectorCatalog: async () => ({
        catalog_version: "concurrent-test",
        items: ["oauth-one", "oauth-two"].map((id, index) => ({
          id,
          name: `OAuth ${index + 1}`,
          provider: "Example Labs",
          description: `Concurrent browser authorization ${index + 1}`,
          category: "productivity",
          examples: [],
          kind: "mcp",
          version: "1.0.0",
          compatible: true,
          capabilities: ["mcp_tools"],
        })),
      }),
      ConnectorStates: async () => ["oauth-one", "oauth-two"].map((id) => ({
        connector_id: id,
        status: concurrentConnected.has(id) ? "connected" : concurrentAuthorizing.has(id) ? "authorizing" : "authorization_required",
      })),
      InstallConnector: async (_tabID: string, connectorID: string) => ({ connector_id: connectorID, status: "authorization_required" }),
      ConnectorAuthFields: async () => [],
      ConnectConnector: async (_tabID: string, connectorID: string) => {
        concurrentStarted.push(connectorID);
        concurrentAuthorizing.add(connectorID);
        await new Promise<void>((resolve, reject) => concurrentResolvers.set(connectorID, {
          resolve: () => { concurrentAuthorizing.delete(connectorID); concurrentConnected.add(connectorID); resolve(); },
          reject,
        }));
        return { connector_id: connectorID, status: "connected" };
      },
      CancelConnectorAuthorization: async (_tabID: string, connectorID: string) => {
        concurrentCancelled.push(connectorID);
        concurrentAuthorizing.delete(connectorID);
        concurrentResolvers.get(connectorID)?.reject(new Error("authorization canceled"));
      },
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => {
  concurrentRoot.render(<LocaleProvider><ToastProvider><MCPServersSettingsPage workspace /></ToastProvider></LocaleProvider>);
  await flush();
});
await waitFor("concurrent catalog", () => document.querySelectorAll(".connector-catalog__card").length === 3);
const openFirst = document.querySelector<HTMLButtonElement>('[data-connector-id="oauth-one"] .connector-catalog__card-main');
if (!openFirst) throw new Error("missing first connector card");
await act(async () => { openFirst.click(); await flush(); });
const connectFirst = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__actions .btn--primary");
if (!connectFirst) throw new Error("missing first connector connect action");
await act(async () => { connectFirst.click(); await flush(); });
await waitFor("first concurrent authorization", () => concurrentStarted.includes("oauth-one"));
ok(document.body.textContent?.includes("若页面被关闭，可取消连接，或直接连接其他服务"), "authorization dialog explains how to recover after closing the browser page");

await act(async () => {
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
  await flush();
});
ok(!document.querySelector(".connector-catalog-dialog"), "connector details can be closed without abandoning browser authorization");
ok(document.querySelector('[data-connector-id="oauth-two"]')?.textContent?.includes("点击连接将切换当前授权"), "other connector cards explain that starting them switches authorization");

const openSecond = document.querySelector<HTMLButtonElement>('[data-connector-id="oauth-two"] .connector-catalog__card-main');
if (!openSecond) throw new Error("missing second connector card");
await act(async () => { openSecond.click(); await flush(); });
const connectSecond = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__actions .btn--primary");
if (!connectSecond) throw new Error("missing second connector connect action");
await act(async () => { connectSecond.click(); await flush(); });
await waitFor("authorization replacement", () => concurrentCancelled.includes("oauth-one") && concurrentStarted.join(",") === "oauth-one,oauth-two");
ok(document.body.textContent?.includes("已结束 OAuth 1 的授权等待，正在打开 OAuth 2 的授权页面"), "replacement dialog identifies the canceled and newly opened authorizations");
ok(!document.body.textContent?.includes("连接失败，请重试"), "replaced authorization does not surface as a connection failure");

const cancelSecond = Array.from(document.querySelectorAll<HTMLButtonElement>(".connector-catalog-dialog__actions button"))
  .find((button) => button.textContent?.includes("取消连接"));
if (!cancelSecond) throw new Error("missing authorization cancel action");
await act(async () => { cancelSecond.click(); await flush(); });
await waitFor("second authorization cancellation", () => concurrentCancelled.includes("oauth-two"));
ok(!document.querySelector(".connector-catalog-dialog"), "successful cancellation closes the connector dialog");
ok(document.querySelector(".toast")?.textContent?.includes("已取消 OAuth 2 连接"), "successful cancellation uses a lightweight toast");
ok(!document.querySelector(".connector-catalog-dialog__notice"), "cancellation does not leave a persistent gray notice");
await act(async () => { concurrentRoot.unmount(); });
concurrentDom.window.close();

const recoveredDom = installDom();
const recoveredRootElement = document.getElementById("root");
if (!recoveredRootElement) throw new Error("missing recovered root");
const recoveredRoot = createRoot(recoveredRootElement);
let recoveredStatus: "authorizing" | "authorization_required" = "authorizing";
let recoveredCancelCalls = 0;
window.go = {
  main: {
    App: {
      Meta: async () => meta,
      ListTabs: async () => tabs,
      MCPServers: async () => [],
      ConnectorCatalog: async () => ({
        catalog_version: "recovered-authorization",
        items: [{
          id: "oauth-recovered",
          name: "Recovered OAuth",
          provider: "Example Labs",
          description: "Authorization restored from backend state",
          category: "productivity",
          examples: [],
          kind: "mcp",
          version: "1.0.0",
          compatible: true,
          capabilities: ["mcp_tools"],
        }, {
          id: "oauth-failed",
          name: "Failed OAuth",
          provider: "Example Labs",
          description: "Localized OAuth failure",
          category: "productivity",
          examples: [],
          kind: "mcp",
          version: "1.0.0",
          compatible: true,
          capabilities: ["mcp_tools"],
        }],
      }),
      ConnectorStates: async () => [{
        connector_id: "oauth-recovered",
        status: recoveredStatus,
      }, {
        connector_id: "oauth-failed",
        status: "authorization_required",
        error: { code: "CONNECTOR_OAUTH_FAILED", message: "connector OAuth authorization was not completed", retryable: true },
      }],
      CancelConnectorAuthorization: async () => {
        recoveredCancelCalls += 1;
        recoveredStatus = "authorization_required";
      },
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => {
  recoveredRoot.render(<LocaleProvider><ToastProvider><MCPServersSettingsPage workspace /></ToastProvider></LocaleProvider>);
  await flush();
});
await waitFor("recovered authorization catalog", () => document.querySelectorAll(".connector-catalog__card").length === 3);
ok(document.body.textContent?.includes("授权未完成"), "known OAuth failures use a short localized card status");
ok(!document.body.textContent?.includes("connector OAuth authorization was not completed"), "raw English OAuth failure is hidden");
const openRecovered = document.querySelector<HTMLButtonElement>('[data-connector-id="oauth-recovered"] .connector-catalog__card-main');
if (!openRecovered) throw new Error("missing recovered authorization card");
await act(async () => { openRecovered.click(); await flush(); });
const cancelRecovered = Array.from(document.querySelectorAll<HTMLButtonElement>(".connector-catalog-dialog__actions button"))
  .find((button) => button.textContent?.includes("取消连接"));
if (!cancelRecovered) throw new Error("missing recovered cancel action");
await act(async () => { cancelRecovered.click(); await flush(); });
await waitFor("recovered cancellation", () => recoveredCancelCalls === 1);
ok(!document.body.textContent?.includes("取消连接失败"), "authorization restored from backend state can be canceled without frontend busy state");
await act(async () => { recoveredRoot.unmount(); });
recoveredDom.window.close();

const noTaskDom = installDom();
const noTaskRootElement = document.getElementById("root");
if (!noTaskRootElement) throw new Error("missing no-task root");
const noTaskRoot = createRoot(noTaskRootElement);
let noTaskCatalogCalls = 0;
let noTaskStateCalls = 0;
window.go = {
  main: {
    App: {
      Meta: async () => meta,
      ListTabs: async () => [],
      MCPServers: async () => [],
      ConnectorCatalog: async () => {
        noTaskCatalogCalls += 1;
        return {
          catalog_version: "no-task-dynamic",
          items: [{
            id: "unknown-no-task",
            name: "No-task Dynamic Connector",
            provider: "Example Labs",
            description: "Visible before the first task exists",
            category: "research",
            examples: [],
            kind: "skills",
            version: "1.0.0",
            compatible: true,
            capabilities: ["skills"],
          }],
        };
      },
      ConnectorStates: async () => {
        noTaskStateCalls += 1;
        return [];
      },
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => {
  noTaskRoot.render(<LocaleProvider><MCPServersSettingsPage workspace /></LocaleProvider>);
  await flush();
});
await waitFor("no-task dynamic connector", () => document.body.textContent?.includes("No-task Dynamic Connector") === true);
ok(noTaskCatalogCalls === 1, "catalog loads before the first task exists");
ok(noTaskStateCalls === 0, "task-scoped states stay deferred until a task exists");
await act(async () => {
  noTaskRoot.unmount();
});
noTaskDom.window.close();

const staleTargetDom = installDom();
const staleTargetRootElement = document.getElementById("root");
if (!staleTargetRootElement) throw new Error("missing stale-target root");
const staleTargetRoot = createRoot(staleTargetRootElement);
let preparedTabID = "";
window.go = {
  main: {
    App: {
      Meta: async () => meta,
      ListTabs: async () => tabs,
      MCPServers: async () => [],
      ConnectorCatalog: async () => ({
        catalog_version: "stale-target",
        items: [{
          id: "stale-target-connector",
          name: "Stale target connector",
          provider: "Example Labs",
          description: "Uses the backend active tab",
          category: "productivity",
          examples: [],
          kind: "mcp",
          version: "1.0.0",
          compatible: true,
          capabilities: ["mcp_tools"],
        }],
      }),
      ConnectorStates: async () => [{ connector_id: "stale-target-connector", status: "authorization_required" }],
      InstallConnector: async (tabID: string, connectorID: string) => {
        preparedTabID = tabID;
        return { connector_id: connectorID, status: "authorization_required" };
      },
      ConnectorAuthFields: async () => [],
      ConnectConnector: async (_tabID: string, connectorID: string) => ({ connector_id: connectorID, status: "connected" }),
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => {
  staleTargetRoot.render(<LocaleProvider><MCPServersSettingsPage workspace targetTabID="stale-tab" /></LocaleProvider>);
  await flush();
});
await waitFor("stale target catalog", () => document.querySelectorAll(".connector-catalog__card").length === 2);
const staleTargetCard = document.querySelector<HTMLButtonElement>('[data-connector-id="stale-target-connector"] .connector-catalog__card-main');
if (!staleTargetCard) throw new Error("missing stale-target connector card");
await act(async () => { staleTargetCard.click(); await flush(); });
const staleTargetConnect = document.querySelector<HTMLButtonElement>(".connector-catalog-dialog__actions .btn--primary");
if (!staleTargetConnect) throw new Error("missing stale-target connect action");
await act(async () => { staleTargetConnect.click(); await flush(); });
await waitFor("stale target preparation", () => preparedTabID !== "");
ok(preparedTabID === "connector-catalog-tab", "stale frontend target falls back to the backend active tab");
await act(async () => { staleTargetRoot.unmount(); });
staleTargetDom.window.close();

const destroyedTargetDom = installDom();
const destroyedTargetRootElement = document.getElementById("root");
if (!destroyedTargetRootElement) throw new Error("missing destroyed-target root");
const destroyedTargetRoot = createRoot(destroyedTargetRootElement);
let rejectDestroyedState: ((reason?: unknown) => void) | null = null;
let destroyedStateCalls = 0;
let unhandledRejections = 0;
window.addEventListener("unhandledrejection", () => { unhandledRejections += 1; });
window.go = {
  main: {
    App: {
      Meta: async () => meta,
      ListTabs: async () => tabs,
      MCPServers: async () => [],
      ConnectorCatalog: async () => ({ catalog_version: "destroyed-target", items: [] }),
      ConnectorStates: async () => {
        destroyedStateCalls += 1;
        return await new Promise((_, reject) => { rejectDestroyedState = reject; });
      },
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => {
  destroyedTargetRoot.render(<LocaleProvider><MCPServersSettingsPage workspace targetTabID="connector-catalog-tab" /></LocaleProvider>);
  await flush();
});
await waitFor("destroyed target polling start", () => destroyedStateCalls === 1 && rejectDestroyedState !== null);
await act(async () => {
  destroyedTargetRoot.render(<LocaleProvider><MCPServersSettingsPage workspace targetTabID="" /></LocaleProvider>);
  rejectDestroyedState?.(new Error('connector target tab "connector-catalog-tab" is unavailable'));
  await flush();
});
await waitFor("destroyed target local completion", () => destroyedStateCalls === 1);
ok(unhandledRejections === 0, "destroyed target background state load does not emit unhandledrejection");
await act(async () => {
  destroyedTargetRoot.unmount();
});
destroyedTargetDom.window.close();

console.log("connector catalog UI passed");
