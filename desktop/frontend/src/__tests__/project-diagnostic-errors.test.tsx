import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";

import { ProjectAssetSelection } from "../features/projects/ui/ProjectAssetSelection";
import { WorkItemBoard } from "../features/projects/ui/WorkItemBoard";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", { pretendToBeVisual: true, url: "http://localhost" });
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.IS_REACT_ACT_ENVIRONMENT = true;

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function assertDiagnosticPresentation(host: HTMLElement, inlineSelector: string, expectedMessage: string, diagnosticId: string) {
  const inline = host.querySelector<HTMLElement>(inlineSelector);
  if (!inline?.textContent?.includes(expectedMessage)) throw new Error(`missing safe inline message: ${inline?.textContent}`);
  if (inline.textContent.includes("[diagnostic_id=")) throw new Error(`inline message exposed transport metadata: ${inline.textContent}`);
  const toast = host.querySelector<HTMLElement>(".toast--error");
  if (toast?.querySelector(".toast__text")?.textContent !== expectedMessage) throw new Error(`toast changed safe message: ${toast?.textContent}`);
  if (toast.querySelector(".toast__diagnostic code")?.textContent !== diagnosticId) throw new Error(`toast did not render Log ID separately: ${toast.textContent}`);
  if (!toast.querySelector(".toast__diagnostic .copybtn")) throw new Error("toast did not render the Log ID copy control");
}

const host = document.getElementById("root");
if (!host) throw new Error("missing root");
const root = createRoot(host);

const workItemDiagnosticId = "abc123-0001";
const workItemGateway = {
  listCloudProjectWorkItems: async () => {
    throw new Error(`PROJECT_ERROR: work item transport detail [diagnostic_id=${workItemDiagnosticId}]`);
  },
} as Partial<ProjectGateway> as ProjectGateway;

await act(async () => {
  root.render(
    <LocaleProvider><ToastProvider><WorkItemBoard
      accountScopeKey="account-1"
      projectScopeKey="scope-1"
      projectId="project-1"
      projectName="Project"
      archived={false}
      stale={false}
      gateway={workItemGateway}
    /></ToastProvider></LocaleProvider>,
  );
  await flush();
  await flush();
});
assertDiagnosticPresentation(host, ".work-item-board__body [role=alert], .cloud-projects__inline-error", "事项加载失败，请稍后重试。", workItemDiagnosticId);

await act(async () => {
  root.render(<div />);
  await flush();
});

const assetDiagnosticId = "abc123-0002";
const assetGateway = {
  listCloudProjectAssetDirectory: async () => {
    throw new Error(`PROJECT_ERROR: project asset transport detail [diagnostic_id=${assetDiagnosticId}]`);
  },
} as Partial<ProjectGateway> as ProjectGateway;

await act(async () => {
  root.render(
    <LocaleProvider><ToastProvider><ProjectAssetSelection
      projectId="project-1"
      projectScopeKey="scope-1"
      gateway={assetGateway}
      selectedIds={[]}
      onChange={() => {}}
    /></ToastProvider></LocaleProvider>,
  );
  await flush();
  await flush();
});
assertDiagnosticPresentation(host, ".project-asset-selection [role=alert]", "项目资产加载失败", assetDiagnosticId);

await act(async () => root.unmount());
dom.window.close();
console.log("project gateway diagnostics render safe inline text and a separate copyable Log ID");
