// Run: tsx src/__tests__/automation-editor-multiselect.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { HeartbeatPanel } from "../features/automation/heartbeat/HeartbeatPanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (value) passed += 1;
  else failed += 1;
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    await act(flush);
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function editorField(host: HTMLElement, label: string) {
  return [...host.querySelectorAll<HTMLElement>(".heartbeat-editor__field")]
    .find((field) => field.querySelector(":scope > label")?.textContent?.startsWith(label));
}

function activeOption(label: string) {
  return [...document.querySelectorAll<HTMLLabelElement>('[data-state="open"] .heartbeat-editor-multiselect__option')]
    .find((option) => option.textContent?.includes(label));
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
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.CustomEvent = dom.window.CustomEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};
Object.defineProperty(globalThis.window, "matchMedia", {
  configurable: true,
  value: () => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} }),
});
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });

const longConnectorName = "企业知识库连接器（名称很长用于验证完整标题）";
let validateDraftCalls = 0;
window.runtime = { EventsOn: () => () => {}, BrowserOpenURL: () => {} };
window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => [{
        id: "task-multiselect",
        revision: 1,
        name: "多选测试",
        enabled: true,
        scope: "global",
        prompt: { prompt: "run" },
        schedule: { kind: "interval", every: "30m" },
        execution: { approvalMode: "auto" },
        capabilities: { connectorIds: ["github"] },
        delivery: { enabled: true, targets: [{ type: "weixin_ilink", ref: "bot_target_v1_revoked" }] },
        createdAt: "2026-08-01T00:00:00Z",
      }],
      AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 1, limit: 99, canCreate: true, pausedTaskIds: [] }),
      AutomationListRuns: async () => [],
      AutomationListTemplates: async () => [],
      AutomationGetScheduleCatalog: async () => [],
      AutomationListDeliveryTargets: async () => [
        { type: "feishu", ref: "bot_target_v1_ops", label: "运营群", provider: "飞书", available: true, scope: "profile" },
        { type: "feishu", ref: "bot_target_v1_archive", label: "归档群", provider: "飞书", available: true, scope: "workspace" },
        { type: "weixin_ilink", ref: "bot_target_v1_revoked", label: "已撤销群", provider: "微信", available: false, unavailableReason: "授权已撤销", scope: "workspace" },
      ],
      AutomationListCapabilities: async () => ({
        models: [],
        skills: [],
        experts: [],
        connectors: [
          { id: "github", name: "GitHub", available: true },
          { id: "knowledge", name: longConnectorName, available: true },
          { id: "revoked", name: "已撤销连接器", available: false, unavailableReason: "授权已撤销" },
        ],
      }),
      AutomationValidateDraft: async () => {
        validateDraftCalls += 1;
        return { task: {}, nextRuns: [] };
      },
      AutomationModels: async () => [],
      ListWorkspaces: async () => [],
      SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
      ListExperts: async () => [],
      ModelMarketplace: async () => ({ sources: [] }),
    } as Partial<AppBindings> as AppBindings,
  },
};

console.log("\nautomation editor multi-select");
const host = document.getElementById("root")!;
const root = createRoot(host);
const access: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" };
await act(async () => {
  root.render(
    <LocaleProvider>
      <HeartbeatPanel open embedded authenticated subscriptionAccess={access} onClose={() => {}} onOpenTopic={() => {}} />
    </LocaleProvider>,
  );
});

await waitFor("automation task", () => host.querySelector(".heartbeat-card__title-btn") !== null);
await act(async () => {
  host.querySelector<HTMLButtonElement>(".heartbeat-card__title-btn")?.click();
  await flush();
});
await waitFor("automation editor", () => editorField(host, "连接器")?.querySelector(".heartbeat-editor-multiselect__trigger") !== null);

const connectorField = editorField(host, "连接器");
const connectorTrigger = connectorField?.querySelector<HTMLButtonElement>(".heartbeat-editor-multiselect__trigger");
if (!connectorTrigger) throw new Error("missing connector multi-select");
ok(connectorTrigger.textContent?.includes("GitHub") === true, "connector trigger renders the selected label instead of a count");
await act(async () => { connectorTrigger.click(); await flush(); });
const revokedConnector = activeOption("已撤销连接器")?.querySelector<HTMLInputElement>('input[type="checkbox"]');
ok(revokedConnector?.disabled === true, "an unavailable unselected connector cannot be added");
const knowledgeConnector = activeOption(longConnectorName)?.querySelector<HTMLInputElement>('input[type="checkbox"]');
await act(async () => { knowledgeConnector?.click(); await flush(); });
ok(
  connectorTrigger.querySelectorAll(".heartbeat-editor-multiselect__tag").length === 2 &&
    connectorTrigger.querySelector(`[title="${longConnectorName}"]`) !== null,
  "connector multi-select keeps every selected label and exposes the full long name",
);
const githubConnector = activeOption("GitHub")?.querySelector<HTMLInputElement>('input[type="checkbox"]');
await act(async () => { githubConnector?.click(); knowledgeConnector?.click(); await flush(); });
ok(connectorTrigger.textContent?.includes("不使用连接器") === true, "clearing connectors restores the explicit empty state");

const deliveryField = editorField(host, "结果通知");
const deliveryToggle = deliveryField?.querySelector<HTMLInputElement>('.heartbeat-editor__template-toggle input[type="checkbox"]');
let deliveryTrigger = deliveryField?.querySelector<HTMLButtonElement>(".heartbeat-editor-multiselect__trigger");
if (!deliveryToggle || !deliveryTrigger) throw new Error("missing delivery controls");
await act(async () => { deliveryTrigger?.click(); await flush(); });
const revokedTarget = activeOption("已撤销群")?.querySelector<HTMLInputElement>('input[type="checkbox"]');
ok(revokedTarget?.checked === true && revokedTarget.disabled === false, "a saved unavailable target remains selectable for removal");
await act(async () => { revokedTarget?.click(); await flush(); });
ok(deliveryTrigger.textContent?.includes("请选择通知目标") === true, "enabled delivery with no targets shows its required placeholder");
const opsTarget = activeOption("运营群")?.querySelector<HTMLInputElement>('input[type="checkbox"]');
const archiveTarget = activeOption("归档群")?.querySelector<HTMLInputElement>('input[type="checkbox"]');
ok(document.activeElement === opsTarget, "removing an unavailable saved target moves focus to an available option");
await act(async () => { opsTarget?.click(); archiveTarget?.click(); await flush(); });
ok(deliveryTrigger.querySelectorAll(".heartbeat-editor-multiselect__tag").length === 2, "delivery target multi-select renders all selected labels");

await act(async () => { deliveryToggle.click(); await flush(); });
ok(deliveryField?.querySelector(".heartbeat-editor-multiselect__trigger") === null, "disabling result delivery hides the target selector");
await act(async () => { deliveryToggle.click(); await flush(); });
deliveryTrigger = deliveryField?.querySelector<HTMLButtonElement>(".heartbeat-editor-multiselect__trigger");
ok(deliveryTrigger?.querySelectorAll(".heartbeat-editor-multiselect__tag").length === 2, "re-enabling delivery preserves the selected target labels");
await act(async () => { await new Promise((resolve) => setTimeout(resolve, 220)); });
ok(
  validateDraftCalls === 0 && !host.textContent?.includes("下三次运行预览"),
  "editing the draft neither renders nor requests the next-run preview",
);

await act(async () => root.unmount());
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
