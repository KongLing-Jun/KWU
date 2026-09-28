import { JSDOM } from "jsdom";
import React, { act, useState } from "react";
import { createRoot } from "react-dom/client";
import { useWorkspaceNavigation, type WorkspaceView } from "../app/useWorkspaceNavigation";

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.MouseEvent = dom.window.MouseEvent;

const navigationEvents: string[] = [];

function Harness() {
  const [workspaceView, setWorkspaceViewState] = useState<WorkspaceView>("chat");
  const [selectedCloudProjectId, setSelectedCloudProjectIdState] = useState("project-old");
  const navigation = useWorkspaceNavigation({
    closeTransientOverlays: () => {},
    closeNarrowSidebar: () => {},
    clearSidebarDetail: () => {},
    clearSettingsTarget: () => {},
    closeHeartbeatOverlay: () => {},
    setSelectedCloudProjectId: (projectId) => {
      navigationEvents.push(`project:${projectId}`);
      setSelectedCloudProjectIdState(projectId);
    },
    setWorkspaceView: (view) => {
      navigationEvents.push(`view:${view}`);
      setWorkspaceViewState(view);
    },
  });

  return (
    <div data-view={workspaceView} data-project-id={selectedCloudProjectId}>
      <button type="button" data-action="overview" onClick={navigation.openProjectDirectory}>项目</button>
      <button type="button" data-action="project-a" onClick={() => navigation.openCloudProject("project-a")}>项目 A</button>
      <button type="button" data-action="project-b" onClick={() => navigation.openCloudProject("project-b")}>项目 B</button>
      <button type="button" data-action="chat" onClick={() => setWorkspaceViewState("chat")}>新建对话</button>
    </div>
  );
}

const host = document.getElementById("root");
ok(host, "missing root element");
const root = createRoot(host);
await act(async () => root.render(<Harness />));

const shell = () => host.querySelector<HTMLElement>("div");
const click = async (action: string) => {
  const button = host.querySelector<HTMLButtonElement>(`[data-action="${action}"]`);
  ok(button, `missing ${action} action`);
  await act(async () => button.click());
};

await click("overview");
ok(shell()?.dataset.view === "projects", "the primary Projects entry opens the project directory");
ok(shell()?.dataset.projectId === "", "the primary Projects entry clears a stale cloud-project selection");
ok(
  navigationEvents.slice(-2).join("|") === "project:|view:projects",
  "the overview intent clears the selection before opening the project workspace",
);

await click("project-a");
ok(shell()?.dataset.view === "projects", "a workspace cloud-project entry opens the project workspace");
ok(shell()?.dataset.projectId === "project-a", "a workspace cloud-project entry preserves its project id");
ok(
  navigationEvents.slice(-2).join("|") === "project:project-a|view:projects",
  "the detail intent selects project A before opening the project workspace",
);

await click("project-b");
ok(shell()?.dataset.projectId === "project-b", "a later workspace entry replaces the previous project id");

await click("chat");
await click("overview");
ok(shell()?.dataset.view === "projects", "the Projects entry returns from another primary workspace");
ok(shell()?.dataset.projectId === "", "returning through the Projects entry does not restore project B");

await click("project-a");
await click("overview");
ok(shell()?.dataset.projectId === "", "clicking Projects while a detail is open returns to the overview");

await act(async () => root.unmount());
dom.window.close();
console.log("workspace project navigation intents passed");
