import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { ProjectDirectory } from "../components/ProjectDirectory";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
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

let listWorkspacesCalls = 0;
window.go = {
  main: {
    App: {
      ListWorkspaces: async () => {
        listWorkspacesCalls += 1;
        return [{ path: "C:\\work\\reasonix", name: "Reasonix", current: true }];
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

const rootElement = document.getElementById("root");
ok(rootElement, "missing root element");
const root = createRoot(rootElement);

await act(async () => {
  root.render(
    <LocaleProvider>
      <ProjectDirectory />
    </LocaleProvider>,
  );
  await flush();
});

ok(document.querySelector(".project-directory__section--local") === null, "project directory should not render the local-directory section");
ok(listWorkspacesCalls === 0, "project directory should not load local workspaces when the local-directory section is hidden");

await act(async () => root.unmount());
dom.window.close();
console.log("project directory excludes local workspaces");
