// Run: tsx src/__tests__/generated-media-paths.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { AssistantMessage } from "../components/Message";
import { Transcript } from "../components/Transcript";
import { generatedMediaByAssistant, type GeneratedMediaReference } from "../lib/mediaToolResult";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { Item } from "../lib/useController";

let failed = 0;

function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (!value) failed += 1;
}

const localPath = "D:\\workspace\\.clouvia\\generated-media\\image.png";
const markdownImagePath = ".clouvia/generated-media/image.png";
const localVideoPath = "D:\\workspace\\.clouvia\\generated-media\\clip.mp4";
const remoteURL = "https://cdn.example/generated.mp4";
const imageOutput = JSON.stringify({
  media: [{ type: "image", path: localPath, mime_type: "image/png" }],
});
const videoOutput = JSON.stringify({
  media: [
    { type: "video", path: localVideoPath, mime_type: "video/mp4" },
    { type: "video", url: remoteURL, mime_type: "video/mp4" },
  ],
});

const mapping = generatedMediaByAssistant([
  { kind: "user", id: "user-1" },
  { kind: "assistant", id: "tool-call-1", text: "" },
  { kind: "tool", id: "media-1", name: "generate_image", output: imageOutput },
  { kind: "assistant", id: "tool-call-2", text: "" },
  { kind: "tool", id: "media-2", name: "generate_video", output: videoOutput },
  { kind: "assistant", id: "answer-1", text: "Generated" },
]);
ok(mapping.get("answer-1")?.length === 3, "generated media is associated with the final assistant answer");
ok(!mapping.has("tool-call-1"), "tool-call assistant messages do not receive the path footer");

const archivedMapping = generatedMediaByAssistant([
  { kind: "user", id: "archived-user" },
  { kind: "tool", id: "archived-media", name: "generate_image" },
  { kind: "assistant", id: "archived-answer", text: "Generated from archived output" },
], new Map([["archived-media", imageOutput]]));
ok(
  archivedMapping.get("archived-answer")?.[0]?.path === localPath,
  "restored archived media is associated with the final assistant answer",
);

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
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
const openedPaths: string[] = [];
window.go = {
  main: {
    App: {
      OpenWorkspacePathForTab: async (_tabId: string, path: string) => { openedPaths.push(path); },
      ToolResultForTab: async (_tabId: string, toolId: string) => (
        toolId === "archived-media" ? { args: "{}", output: imageOutput } : null
      ),
      ReadFileForTab: async (_tabId: string, path: string) => ({
        path,
        body: "",
        size: 1,
        truncated: false,
        binary: true,
        kind: "image",
        mime: "image/png",
        url: "/__kwu_workspace_media/generated/image.png",
      }),
      ExpertTeamForTab: async () => null,
    } as Partial<AppBindings> as AppBindings,
  },
};

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
await act(async () => {
  root.render(
    <LocaleProvider>
      <AssistantMessage
        item={{ kind: "assistant", id: "answer-1", text: `![Generated fish](${markdownImagePath})\n\nGenerated`, reasoning: "", streaming: false }}
        generatedMedia={mapping.get("answer-1") as GeneratedMediaReference[]}
        tabId="media-tab"
      />
    </LocaleProvider>,
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
});

const message = document.querySelector<HTMLElement>(".msg--assistant");
const footer = document.querySelector<HTMLElement>(".msg-generated-media-paths");
ok(
  document.querySelector<HTMLImageElement>(".msg--assistant .md img")?.getAttribute("src") === "/__kwu_workspace_media/generated/image.png",
  "workspace-relative Markdown images render through the authenticated media endpoint",
);
ok(message?.lastElementChild === footer, "generated media paths render at the end of the assistant message");
ok(footer?.textContent?.includes(localPath) === true, "path footer shows the complete local file path");
ok(footer?.textContent?.includes(localVideoPath) === true, "path footer shows the complete local video path");
ok(footer?.textContent?.includes(remoteURL) === true, "path footer shows the complete remote media URL");
const links = Array.from(footer?.querySelectorAll<HTMLAnchorElement>("a") ?? []);
ok(links.length === 3, "local image, local video, and remote video all render as hyperlinks");
ok(links[2]?.href === remoteURL, "remote media URL is clickable");

await act(async () => {
  links[0]?.click();
  links[1]?.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
});
ok(openedPaths[0] === localPath, "clicking the generated image path opens the local artifact");
ok(openedPaths[1] === localVideoPath, "clicking the generated video path opens the local artifact");

await act(async () => root.unmount());

const archivedItems: Item[] = [
  { kind: "user", id: "archived-user", text: "Generate an image" },
  {
    kind: "tool",
    id: "archived-media",
    name: "generate_image",
    args: "{}",
    readOnly: false,
    status: "done",
    dataArchived: true,
  },
  {
    kind: "assistant",
    id: "archived-answer",
    text: "Generated from archived output",
    reasoning: "",
    streaming: false,
  },
];
const archivedRoot = createRoot(rootElement);
await act(async () => {
  archivedRoot.render(
    <LocaleProvider>
      <Transcript
        items={archivedItems}
        tabId="media-tab"
        viewKey="archived-media-view"
        onPrompt={() => {}}
        questionNavigator={false}
      />
    </LocaleProvider>,
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
});
ok(
  document.querySelector<HTMLElement>(".msg-generated-media-paths")?.textContent?.includes(localPath) === true,
  "transcript restores an archived media result before rendering the final artifact link",
);

await act(async () => archivedRoot.unmount());
dom.window.close();

if (failed > 0) process.exit(1);
