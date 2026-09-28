import { JSDOM } from "jsdom";
import React, { act, useState } from "react";
import { createRoot } from "react-dom/client";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import type { CloudProject, CloudProjectResult } from "../features/projects/model/projectTypes";
import { CloudProjectsSection } from "../features/projects/ui/CloudProjectsSectionImpl";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

const project = (projectId: string, name: string): CloudProject => ({
  projectId,
  name,
  description: null,
  status: "active",
  version: 1,
  createdAt: "2026-08-21T00:00:00Z",
  updatedAt: "2026-08-21T00:00:00Z",
});

const projects = [project("project-a", "项目 A"), project("project-b", "项目 B")];
const joinedProjects = [project("project-c", "项目 C")];
const detailRequests: Array<{ projectId: string; request: ReturnType<typeof deferred<CloudProjectResult>> }> = [];
const featureRequest = deferred<{ projectScopeKey: string; instructionsAvailable: boolean; workItemsAvailable: boolean }>();
let instructionRequests = 0;
const gateway = {
  listCloudProjects: async () => ({ projectScopeKey: "scope-1", items: projects, nextCursor: null, stale: false, cachedAt: null }),
  listJoinedCloudProjects: async () => ({ projectScopeKey: "scope-1", items: joinedProjects, nextCursor: null, stale: false, cachedAt: null }),
  getCloudProjectFeatures: () => featureRequest.promise,
  getCloudProject: (projectId: string) => {
    const request = deferred<CloudProjectResult>();
    detailRequests.push({ projectId, request });
    return request.promise;
  },
  getCloudProjectInstruction: async (projectId: string) => {
    instructionRequests += 1;
    return {
      projectScopeKey: "scope-1",
      projectId,
      humanDescription: "项目背景",
      agentInstruction: "遵循项目约定",
      contentHash: "a".repeat(64),
      version: 7,
      createdAt: "2026-08-21T00:00:00Z",
      updatedAt: "2026-08-21T00:00:00Z",
    };
  },
  listCloudProjectCapabilities: async (projectId: string) => ({ projectScopeKey: "scope-1", projectId, projectVersion: 1, items: [] }),
} as Partial<ProjectGateway> as ProjectGateway;

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
globalThis.MouseEvent = dom.window.MouseEvent;

let projectComposerLeaveCalls = 0;

function Harness() {
  const [selectedProjectId, setSelectedProjectId] = useState("project-c");
  return (
    <>
      <button type="button" data-action="overview" onClick={() => setSelectedProjectId("")}>项目总览</button>
      <button type="button" data-action="project-a" onClick={() => setSelectedProjectId("project-a")}>项目 A</button>
      <button type="button" data-action="project-b" onClick={() => setSelectedProjectId("project-b")}>项目 B</button>
      <button type="button" data-action="project-c" onClick={() => setSelectedProjectId("project-c")}>项目 C</button>
      <CloudProjectsSection
        accountScopeKey="account-1"
        authenticated
        gateway={gateway}
        selectedProjectId={selectedProjectId}
        onSelectedProjectChange={setSelectedProjectId}
        onProjectComposerLeave={() => { projectComposerLeaveCalls += 1; }}
        renderComposer={({ instructionVersion, contentHash }) => <div data-project-instruction-version={instructionVersion} data-project-content-hash={contentHash} />}
      />
    </>
  );
}

const flush = async () => {
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
};
const click = async (action: string) => {
  const button = document.querySelector<HTMLButtonElement>(`[data-action="${action}"]`);
  ok(button, `missing ${action} action`);
  await act(async () => button.click());
};
const resolveDetail = async (index: number, expectedProjectId: string) => {
  await flush();
  const detail = detailRequests[index];
  ok(detail?.projectId === expectedProjectId, `expected detail request for ${expectedProjectId}`);
  await act(async () => {
    detail.request.resolve({
      projectScopeKey: "scope-1",
      project: [...projects, ...joinedProjects].find((item) => item.projectId === expectedProjectId)!,
      stale: false,
      cachedAt: null,
    });
    await detail.request.promise;
  });
  await flush();
};

const host = document.getElementById("root");
ok(host, "missing root element");
const root = createRoot(host);
await act(async () => root.render(<LocaleProvider><ToastProvider><Harness /></ToastProvider></LocaleProvider>));
await flush();

ok(document.querySelector('[data-project-view="workspace-loading"]'), "a selected workspace project starts on a detail-loading surface");
ok(!document.querySelector("[data-project-hero]"), "a selected workspace project never flashes the directory overview");
await resolveDetail(0, "project-c");
ok(document.querySelector('[data-project-view="workspace"]'), "a joined project opens after its detail loads");
ok(document.querySelector("#cloud-project-workspace-title")?.textContent === "项目 C", "the initially selected joined-project detail is rendered");
ok(instructionRequests === 0, "joined-project detail can arrive before the project feature switch");
ok(document.querySelector("[data-project-instruction-version]")?.getAttribute("data-project-instruction-version") === "0", "the composer remains instruction-gated while feature discovery is pending");
await act(async () => {
  featureRequest.resolve({ projectScopeKey: "scope-1", instructionsAvailable: true, workItemsAvailable: false });
  await featureRequest.promise;
});
await flush();
await flush();
ok(instructionRequests === 1, "feature readiness retries the current joined-project instruction exactly once");
ok(document.querySelector("[data-project-instruction-version]")?.getAttribute("data-project-instruction-version") === "7", "the composer receives the authoritative instruction version after the retry");
ok(document.querySelector("[data-project-content-hash]")?.getAttribute("data-project-content-hash") === "a".repeat(64), "the composer receives the authoritative instruction hash after the retry");

await click("overview");
ok(document.querySelector('[data-project-view="directory"]'), "the primary Projects entry returns an already-mounted detail to the directory");
ok(document.querySelector("[data-project-hero]"), "the primary Projects entry stays on the overview");
ok(projectComposerLeaveCalls === 0, "primary navigation does not discard the existing project conversation draft");

await click("project-a");
ok(document.querySelector('[data-project-view="workspace-loading"]'), "reopening project A does not render the overview while loading");
await resolveDetail(1, "project-a");
await click("project-b");
ok(document.querySelector('[data-project-view="workspace-loading"]'), "switching projects hides the previous project immediately");
ok(!document.querySelector("#cloud-project-workspace-title"), "project A is not shown while project B loads");
ok(!document.querySelector("[data-project-hero]"), "switching to project B does not flash the overview");
await resolveDetail(2, "project-b");
ok(document.querySelector("#cloud-project-workspace-title")?.textContent === "项目 B", "project B replaces project A");

await click("overview");
await click("project-c");
ok(document.querySelector('[data-project-view="workspace-loading"]'), "opening a joined project starts its detail request");
await resolveDetail(3, "project-c");
ok(document.querySelector("#cloud-project-workspace-title")?.textContent === "项目 C", "a joined project opened from controlled navigation replaces the loading surface");

await act(async () => root.unmount());
dom.window.close();
console.log("controlled cloud-project navigation passed");
