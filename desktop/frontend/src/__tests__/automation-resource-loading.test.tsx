// Run: tsx src/__tests__/automation-resource-loading.test.tsx

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
  for (let attempt = 0; attempt < 80; attempt += 1) {
    await act(() => flush());
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

const eventCallbacks = new Map<string, (...args: unknown[]) => void>();
let projectTreeRevision = 0;
function emitProjectTreeChanged(delayed = false) {
	projectTreeRevision += 1;
	const revision = projectTreeRevision;
	if (delayed) {
		setTimeout(() => eventCallbacks.get("project-tree:changed")?.(revision), 0);
		return;
	}
	eventCallbacks.get("project-tree:changed")?.(revision);
}
window.runtime = {
  EventsOn: (name: string, callback: (...args: unknown[]) => void) => {
    eventCallbacks.set(name, callback);
    return () => eventCallbacks.delete(name);
  },
  BrowserOpenURL: () => {},
};

let projectCalls = 0;
let templateCalls = 0;
let projectMode: "recovery" | "late" = "recovery";
let templateMode: "recovery" | "late" = "recovery";
const projectPage = (projectId: string, name: string) => ({
	projectScopeKey: "scope",
	items: [{ projectId, name }],
	nextCursor: "",
	stale: false,
	cachedAt: null,
});
let resolveLateProject: ((page: ReturnType<typeof projectPage>) => void) | undefined;
let resolveLateTemplates: ((templates: unknown[]) => void) | undefined;
window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => { throw new Error("tasks unavailable"); },
      AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 0, limit: 0, unlimited: true, canCreate: true, pausedTaskIds: [] }),
      AutomationListTemplates: async () => {
        templateCalls += 1;
				if (templateMode === "late") {
					if (templateCalls === 1) return new Promise<unknown[]>((resolve) => { resolveLateTemplates = resolve; });
					return [{ id: "newest-template", title: "最新模板", description: "newest", prompt: "newest", source: "builtin" }];
				}
        if (templateCalls === 1) throw new Error("templates initializing");
        return [{ id: "daily-summary", title: "每日总结", description: "整理当天进展", prompt: "总结", source: "builtin" }];
      },
      AutomationGetScheduleCatalog: async () => [],
      AutomationListDeliveryTargets: async () => [],
			AutomationGenerateID: async () => "task-new",
			AutomationModels: async () => [],
			AutomationListCapabilities: async () => ({ models: [], skills: [], experts: [], connectors: [] }),
			SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
			ListExperts: async () => [],
			ListUserExperts: async () => [],
			ModelMarketplace: async () => ({ sources: [] }),
      ListWorkspaces: async () => [{ name: "Local Workspace", path: "D:\\KWU", current: true }],
      ListCloudProjects: async () => {
        projectCalls += 1;
				if (projectMode === "late") {
					if (projectCalls === 1) return new Promise<ReturnType<typeof projectPage>>((resolve) => { resolveLateProject = resolve; });
					emitProjectTreeChanged();
					return projectPage("project-newest", "Newest 云项目");
				}
        if (projectCalls === 1) throw new Error("projects initializing");
        const projectId = projectCalls === 2 ? "project-alpha" : "project-beta";
        const name = projectCalls === 2 ? "Alpha 云项目" : "Beta 云项目";
				emitProjectTreeChanged();
        return projectPage(projectId, name);
      },
      ListJoinedCloudProjects: async () => {
				if (projectMode === "recovery" && projectCalls === 1) throw new Error("joined projects initializing");
				emitProjectTreeChanged(true);
				return { projectScopeKey: "scope", items: [], nextCursor: "", stale: false, cachedAt: null };
			},
			ProjectTreeRevision: async () => projectTreeRevision,
    } as unknown as Partial<AppBindings> as AppBindings,
  },
};

console.log("\nautomation resource loading");
const host = document.getElementById("root")!;
const root = createRoot(host);
const access: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" };
await act(async () => {
  root.render(
    <LocaleProvider>
      <HeartbeatPanel open authenticated subscriptionAccess={access} onClose={() => {}} onOpenTopic={() => {}} />
    </LocaleProvider>,
  );
});

await waitFor("independent resource errors", () => (
  host.querySelector("[data-automation-project-error]") !== null
  && host.querySelector("[data-automation-template-error]") !== null
));
ok(templateCalls === 1 && projectCalls === 1, "project and template failures are visible instead of becoming empty results");

const errorTemplateButton = [...host.querySelectorAll<HTMLButtonElement>("button")]
	.find((button) => button.textContent?.includes("模板使用"));
await act(async () => { errorTemplateButton?.click(); await flush(); });
ok(!host.textContent?.includes("暂无可用模板"), "template errors do not render the successful empty state");
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-modal__back")?.click(); await flush(); });

const addButton = [...host.querySelectorAll<HTMLButtonElement>("button")]
	.find((button) => button.textContent?.includes("添加任务"));
await act(async () => { addButton?.click(); await flush(); });
await waitFor("task editor", () => host.querySelector(".heartbeat-editor") !== null);
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-scope-btn")?.click(); await flush(); });
const editorRetry = [...host.querySelectorAll<HTMLButtonElement>(".heartbeat-project-menu__item")]
	.find((button) => button.textContent?.includes("云项目加载失败"));
ok(editorRetry !== undefined, "task editor reuses the recoverable cloud-project state");
await act(async () => { editorRetry?.click(); await flush(); });
await waitFor("editor project retry", () => projectCalls === 2 && host.querySelector("[data-automation-project-error]") === null);
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-modal__back")?.click(); await flush(); });

await act(async () => {
  host.querySelector<HTMLButtonElement>("[data-automation-template-error] button")?.click();
  await flush();
});
await waitFor("resource retries", () => (
  host.querySelector("[data-automation-project-error]") === null
  && host.querySelector("[data-automation-template-error]") === null
));
ok(projectCalls === 2 && templateCalls === 2, "failed project and template requests can be retried independently");

const selectButtons = host.querySelectorAll<HTMLButtonElement>(".heartbeat-toolbar__btn--select");
await act(async () => { selectButtons[selectButtons.length - 1]?.click(); await flush(); });
await waitFor("retried project option", () => document.body.textContent?.includes("Alpha 云项目") === true);
ok(document.body.textContent?.includes("Alpha 云项目") === true, "project retry publishes the successful project list");

await act(async () => { emitProjectTreeChanged(); await flush(); });
await waitFor("project tree refresh", () => document.body.textContent?.includes("Beta 云项目") === true);
ok(projectCalls === 3 && document.body.textContent?.includes("Beta 云项目") === true, "project-tree changes refresh cloud projects");
await act(async () => { await flush(); await flush(); });
ok(projectCalls === 3, "project-list cache events do not create a self-refresh loop");

const templateButton = [...host.querySelectorAll<HTMLButtonElement>("button")]
  .find((button) => button.textContent?.includes("模板使用"));
await act(async () => { templateButton?.click(); await flush(); });
await waitFor("template after unrelated failures", () => host.textContent?.includes("每日总结") === true);
ok(host.textContent?.includes("每日总结") === true, "task and workspace failures do not discard a successful template retry");

projectMode = "late";
templateMode = "late";
projectCalls = 0;
templateCalls = 0;
await act(async () => {
	root.render(<LocaleProvider><HeartbeatPanel key="late" open={false} authenticated subscriptionAccess={access} onClose={() => {}} onOpenTopic={() => {}} /></LocaleProvider>);
	await flush();
});
await act(async () => {
	root.render(<LocaleProvider><HeartbeatPanel key="late" open authenticated subscriptionAccess={access} onClose={() => {}} onOpenTopic={() => {}} /></LocaleProvider>);
	await flush();
});
await waitFor("late first requests", () => projectCalls === 1 && templateCalls === 1);
const loadingTemplateButton = [...host.querySelectorAll<HTMLButtonElement>("button")]
	.find((button) => button.textContent?.includes("模板使用"));
await act(async () => { loadingTemplateButton?.click(); await flush(); });
ok(
	host.querySelector("[data-automation-template-loading]") !== null && !host.textContent?.includes("暂无可用模板"),
	"template loading does not render the successful empty state",
);
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-modal__back")?.click(); await flush(); });
await act(async () => {
	root.render(<LocaleProvider><HeartbeatPanel key="late" open={false} authenticated subscriptionAccess={access} onClose={() => {}} onOpenTopic={() => {}} /></LocaleProvider>);
	await flush();
});
await act(async () => {
	root.render(<LocaleProvider><HeartbeatPanel key="late" open authenticated subscriptionAccess={access} onClose={() => {}} onOpenTopic={() => {}} /></LocaleProvider>);
	await flush();
});
await waitFor("fast second requests", () => projectCalls === 2 && templateCalls === 2);

const latestSelectButtons = host.querySelectorAll<HTMLButtonElement>(".heartbeat-toolbar__btn--select");
await act(async () => { latestSelectButtons[latestSelectButtons.length - 1]?.click(); await flush(); });
await waitFor("newest project", () => document.body.textContent?.includes("Newest 云项目") === true);
const latestTemplateButton = [...host.querySelectorAll<HTMLButtonElement>("button")]
	.find((button) => button.textContent?.includes("模板使用"));
await act(async () => { latestTemplateButton?.click(); await flush(); });
await waitFor("newest template", () => host.textContent?.includes("最新模板") === true);

await act(async () => {
	resolveLateProject?.(projectPage("project-stale", "Stale 云项目"));
	resolveLateTemplates?.([{ id: "stale-template", title: "过期模板", description: "stale", prompt: "stale", source: "builtin" }]);
	await flush();
});
ok(!host.textContent?.includes("过期模板"), "late template responses cannot overwrite a newer generation");
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-modal__back")?.click(); await flush(); });
const finalSelectButtons = host.querySelectorAll<HTMLButtonElement>(".heartbeat-toolbar__btn--select");
await act(async () => { finalSelectButtons[finalSelectButtons.length - 1]?.click(); await flush(); });
ok(!document.body.textContent?.includes("Stale 云项目"), "late project responses cannot overwrite a newer generation");

const callsBeforeAnonymousOpen = projectCalls;
await act(async () => {
	root.render(
		<LocaleProvider>
			<HeartbeatPanel key="anonymous" open authenticated={false} subscriptionAccess={{ state: "unavailable" }} onClose={() => {}} onOpenTopic={() => {}} />
		</LocaleProvider>,
	);
	await flush();
});
await waitFor("anonymous automation panel", () => host.textContent?.includes("添加任务") === true);
await act(async () => {
	emitProjectTreeChanged();
	await flush();
});
ok(projectCalls === callsBeforeAnonymousOpen, "anonymous panels skip cloud-project requests on open and project-tree changes");
ok(host.querySelector("[data-automation-project-error]") === null, "anonymous panels do not render cloud-project failures");

const anonymousAddButton = [...host.querySelectorAll<HTMLButtonElement>("button")]
	.find((button) => button.textContent?.includes("添加任务"));
await act(async () => { anonymousAddButton?.click(); await flush(); });
await waitFor("anonymous task editor", () => host.querySelector(".heartbeat-editor") !== null);
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-scope-btn")?.click(); await flush(); });
ok(host.textContent?.includes("Local Workspace") === true, "anonymous task editors keep local workspaces available");
ok(host.textContent?.includes("登录云账号后可查看云项目") === true, "anonymous task editors reuse the cloud-project sign-in guidance");

await act(async () => root.unmount());
ok(!eventCallbacks.has("project-tree:changed"), "project-tree listener is cleaned up on unmount");
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
