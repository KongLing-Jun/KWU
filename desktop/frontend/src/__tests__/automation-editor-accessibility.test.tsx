// Run: tsx src/__tests__/automation-editor-accessibility.test.tsx

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
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    await act(flush);
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

async function press(target: Element, key: string) {
  await act(async () => {
    target.dispatchEvent(new window.KeyboardEvent("keydown", { key, bubbles: true }));
    await flush();
  });
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

window.runtime = { EventsOn: () => () => {}, BrowserOpenURL: () => {} };
window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => [{
        id: "task-accessible-select",
        revision: 1,
        name: "键盘选择器测试",
        enabled: true,
        scope: "project",
        workspaceRoot: "D:/project/reasonix",
        prompt: { prompt: "run", model: "byok/model-a", billingSource: "newapi_byok" },
        schedule: { kind: "interval", every: "30m" },
        execution: { approvalMode: "auto" },
        createdAt: "2026-08-01T00:00:00Z",
      }],
      AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 1, limit: 0, unlimited: true, canCreate: true, pausedTaskIds: [] }),
      AutomationListRuns: async () => [],
      AutomationListTemplates: async () => [],
      AutomationGetScheduleCatalog: async () => [],
      AutomationListDeliveryTargets: async () => [],
      AutomationListCapabilities: async () => ({ models: [], skills: [], experts: [], connectors: [] }),
      AutomationValidateDraft: async () => ({ task: {}, nextRuns: [] }),
      AutomationModels: async () => [
        { ref: "byok/model-a", provider: "byok", model: "model-a", billingSource: "newapi_byok", current: false },
        { ref: "byok/model-b", provider: "byok", model: "model-b", billingSource: "newapi_byok", current: false },
      ],
      ListWorkspaces: async () => [{ name: "Reasonix", path: "D:/project/reasonix", current: true }],
			ListCloudProjects: async () => ({
				projectScopeKey: "scope-1",
				items: [{ projectId: "cloud-project-1", name: "云端计划", description: null, status: "active", version: 1, createdAt: "2026-08-01T00:00:00Z", updatedAt: "2026-08-01T00:00:00Z" }],
				nextCursor: null, stale: false, cachedAt: null,
			}),
			ListJoinedCloudProjects: async () => ({ projectScopeKey: "scope-1", items: [], nextCursor: null, stale: false, cachedAt: null }),
      SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
      ListExperts: async () => [],
			ListUserExperts: async () => [
				{ id: "user-expert-1", source: "user", mode: "expert", name: "我的专家", description: "自定义专家", systemPrompt: "按自定义方法执行", enabled: true, createdAt: 1, updatedAt: 1 },
				{ id: "user-team-1", source: "user", mode: "team", name: "我的专家团", description: "自定义专家团", systemPrompt: "组织专家协作", members: [], enabled: true, createdAt: 1, updatedAt: 1 },
			],
      ModelMarketplace: async () => ({ sources: [] }),
    } as Partial<AppBindings> as AppBindings,
  },
};

console.log("\nautomation editor select accessibility");
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
await waitFor("automation editor", () => host.querySelectorAll(".heartbeat-editor-select__trigger").length >= 3);

const workspaceField = [...host.querySelectorAll<HTMLElement>(".heartbeat-editor__field")]
  .find((field) => field.querySelector("label")?.textContent?.startsWith("工作空间"));
const workspaceTrigger = workspaceField?.querySelector<HTMLButtonElement>(".heartbeat-scope-btn");
ok(
  workspaceTrigger?.getAttribute("aria-haspopup") === "listbox" && workspaceTrigger.getAttribute("aria-expanded") === "false",
  "the workspace scope is exposed as one selectable list",
);
if (!workspaceTrigger) throw new Error("missing workspace picker trigger");
await act(async () => {
  workspaceTrigger.click();
  await flush();
});
const globalWorkspaceOption = [...document.querySelectorAll<HTMLElement>('[role="option"]')]
  .find((option) => option.textContent?.trim() === "全局");
ok(globalWorkspaceOption !== undefined, "the workspace picker offers the global scope alongside local workspaces");
const cloudWorkspaceOption = [...document.querySelectorAll<HTMLElement>('[role="option"]')]
	.find((option) => option.textContent?.includes("云端计划"));
ok(cloudWorkspaceOption !== undefined, "the workspace picker includes active Cloud Projects");
await act(async () => {
	cloudWorkspaceOption?.click();
	await flush();
});
ok(workspaceTrigger.textContent?.includes("云端计划") === true, "a Cloud Project can be selected as the automation workspace");
await act(async () => {
	workspaceTrigger.click();
	await flush();
});
const reopenedGlobalWorkspaceOption = [...document.querySelectorAll<HTMLElement>('[role="option"]')]
	.find((option) => option.textContent?.trim() === "全局");
await act(async () => {
  reopenedGlobalWorkspaceOption?.click();
  await flush();
});
ok(
  workspaceTrigger.textContent?.includes("全局") === true && workspaceTrigger.getAttribute("aria-expanded") === "false",
  "selecting global clears the project workspace and closes the picker",
);

const expertTrigger = host.querySelector<HTMLButtonElement>('.heartbeat-editor-select__trigger[aria-label="专家"]');
if (!expertTrigger) throw new Error("missing expert picker trigger");
await act(async () => {
	expertTrigger.click();
	await flush();
});
const expertOptionElements = [...document.querySelectorAll<HTMLElement>('[role="option"]')];
ok(expertOptionElements.some((option) => option.textContent?.includes("我的专家")), "the expert picker includes custom experts");
ok(expertOptionElements.some((option) => option.textContent?.includes("我的专家团")), "the expert picker includes custom expert teams");
await act(async () => {
	expertOptionElements.find((option) => option.textContent?.includes("我的专家团"))?.click();
	await flush();
});

const modelField = [...host.querySelectorAll<HTMLElement>(".heartbeat-editor__field")]
  .find((field) => field.querySelector("label")?.textContent?.startsWith("模型"));
const trigger = modelField?.querySelector<HTMLButtonElement>(".heartbeat-editor-select__trigger");
ok(trigger?.getAttribute("aria-label") === "模型", "the select trigger exposes its field label");

if (!trigger) throw new Error("missing model picker trigger");
trigger.focus();
await press(trigger, "ArrowDown");
await waitFor("opened model listbox", () => trigger.getAttribute("aria-expanded") === "true");
ok(document.activeElement?.textContent?.includes("model-a") === true, "opening by keyboard focuses the selected option");

await press(document.activeElement!, "ArrowDown");
ok(document.activeElement?.textContent?.includes("model-b") === true, "ArrowDown moves to the next option");
await press(document.activeElement!, "Home");
ok(document.activeElement?.textContent?.includes("model-a") === false, "Home moves to the first option");
await press(document.activeElement!, "End");
ok(document.activeElement?.textContent?.includes("model-b") === true, "End moves to the last option");
await press(document.activeElement!, "ArrowUp");
ok(document.activeElement?.textContent?.includes("model-a") === true, "ArrowUp moves to the previous option");
await press(document.activeElement!, "ArrowDown");
await press(document.activeElement!, "Enter");
ok(
  trigger.getAttribute("aria-expanded") === "false" &&
    trigger.textContent?.includes("model-b") === true &&
    document.activeElement === trigger,
  "Enter selects the active option, closes the listbox, and restores trigger focus",
);

await press(trigger, "ArrowUp");
await waitFor("reopened model listbox", () => trigger.getAttribute("aria-expanded") === "true");
await press(document.activeElement!, "Escape");
ok(
  trigger.getAttribute("aria-expanded") === "false" && document.activeElement === trigger,
  "Escape closes the listbox and restores trigger focus",
);

await act(async () => root.unmount());
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
