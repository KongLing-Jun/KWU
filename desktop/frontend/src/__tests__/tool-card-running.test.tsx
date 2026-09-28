// Run: tsx src/__tests__/tool-card-running.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { ToolCard } from "../components/ToolCard";
import { shouldAutoExpandToolGroup, ToolGroup } from "../components/ToolGroup";
import { LocaleProvider } from "../lib/i18n";
import { ToolDisclosureProvider } from "../lib/toolDisclosure";
import { toolDisplayName } from "../lib/tools";
import type { AppBindings } from "../lib/bridge";
import type { Item } from "../lib/useController";

type ToolItem = Extract<Item, { kind: "tool" }>;

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

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: { ...dom.window.navigator, language: "en-US" } });
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.Node = dom.window.Node;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", { configurable: true, value: () => {} });
  return dom;
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

function mockApp(methods: Partial<AppBindings>) {
  window.go = {
    main: {
      App: methods as AppBindings,
    },
  };
}

console.log("\ntool card running state");

ok(toolDisplayName("write_file") === "写入文件", "built-in write tools use the Chinese interface vocabulary");
ok(toolDisplayName("verify_artifact") === "验证产物", "artifact verification uses the Chinese interface vocabulary");
ok(toolDisplayName("deliver_files") === "交付文件", "artifact delivery uses the Chinese interface vocabulary");
ok(toolDisplayName("complete_step") === "完成步骤", "step completion uses the Chinese interface vocabulary");
ok(toolDisplayName("task") === "委派任务", "delegated tasks use the Chinese interface vocabulary");
ok(toolDisplayName("custom_mcp_tool") === "custom_mcp_tool", "unknown extension tool names remain unchanged");

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const task: ToolItem = {
    kind: "tool",
    id: "task-lifecycle-card",
    name: "task",
    args: JSON.stringify({ description: "review implementation" }),
    readOnly: false,
    status: "done",
    dataArchived: true,
    taskLifecycle: {
      subagentRef: "child-lifecycle",
      turnStatus: "completed",
      finalAnswer: "review completed",
      artifacts: [{ path: "D:\\workspace\\report.md", size: 42 }],
    },
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={task} tabId="tab-parent" />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(document.querySelectorAll(".tool").length === 1, "completed task lifecycle renders as one card");
  const trigger = document.querySelector<HTMLButtonElement>(".tool__head");
  await act(async () => {
    trigger?.click();
    await flush();
  });
  ok(document.body.textContent?.includes("review completed") === true, "task lifecycle card exposes the child final answer");
  ok(document.body.textContent?.includes("D:\\workspace\\report.md") === true, "task lifecycle card exposes host-observed artifacts");

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard
          item={{
            kind: "tool", id: "wait-lifecycle-card", name: "wait_task", args: "{}", readOnly: false, status: "done",
            taskLifecycleRefs: ["child-lifecycle"],
          }}
        />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(document.querySelector(".tool") === null, "folded wait control does not render a duplicate card");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const item: ToolItem = {
    kind: "tool",
    id: "call-bash-1",
    name: "bash",
    args: JSON.stringify({ command: "python3 <<'PYEOF'\nwhile True:\n    pass\nPYEOF" }),
    readOnly: false,
    status: "running",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={item} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.querySelector(".tool--open") === null, "running shell card stays collapsed without user input");
  ok(document.body.textContent?.includes("命令仍在运行") === false, "collapsed running shell does not mount its waiting detail");
  const trigger = document.querySelector<HTMLButtonElement>(".tool__head");
  await act(async () => {
    trigger?.click();
    await flush();
  });
  ok(document.querySelector(".tool--open") !== null, "user click expands a running shell card");
  ok(document.body.textContent?.includes("命令仍在运行") === true, "expanded running shell shows its waiting detail");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const running: ToolItem = {
    kind: "tool",
    id: "task-user-controlled-disclosure",
    name: "task",
    args: JSON.stringify({ description: "inspect the implementation" }),
    readOnly: false,
    status: "running",
  };
  const nested: ToolItem = {
    kind: "tool",
    id: "nested-read",
    parentId: running.id,
    name: "read_file",
    args: JSON.stringify({ path: "D:\\workspace\\source.ts" }),
    readOnly: true,
    status: "running",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={running} />
      </LocaleProvider>,
    );
    await flush();
  });
  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={running} subcalls={[nested]} />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(document.querySelector(".tool--open") === null, "nested tool arrival cannot expand its parent card");

  const trigger = document.querySelector<HTMLButtonElement>(".tool__head");
  await act(async () => {
    trigger?.click();
    await flush();
  });
  ok(document.querySelector(".tool--open") !== null, "user click expands a nested tool card");

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={{ ...running, status: "done" }} subcalls={[{ ...nested, status: "done" }]} />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(document.querySelector(".tool--open") !== null, "completion preserves the user's expanded state");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const item: ToolItem = {
    kind: "tool",
    id: "call-completed-shell",
    name: "bash",
    args: JSON.stringify({ command: "printf output" }),
    output: Array.from({ length: 200 }, (_, index) => `line ${index + 1}`).join("\n"),
    readOnly: false,
    status: "done",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={item} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.querySelector(".tool__body .code-block") === null, "completed collapsed tools do not mount hidden code viewers");
  const trigger = document.querySelector<HTMLButtonElement>(".tool__head");
  await act(async () => {
    trigger?.click();
    await flush();
  });
  ok(document.querySelector(".tool__body .code-block") !== null, "opening a completed tool mounts its full details on demand");
  ok(document.querySelector(".tool__body")?.textContent?.includes("line 1") === true, "lazy-mounted details preserve tool output");
  await act(async () => {
    trigger?.click();
    await flush();
  });
  ok(document.querySelector(".tool__body .code-block") === null, "closing a completed tool releases its heavy details");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  mockApp({
    ToolResultForTab: async () => {
      throw new Error("archive unavailable");
    },
  });
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const item: ToolItem = {
    kind: "tool",
    id: "call-archived-1",
    name: "bash",
    args: "",
    readOnly: false,
    status: "running",
    dataArchived: true,
    subject: "echo hi",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={item} tabId="tab-1" />
      </LocaleProvider>,
    );
    await flush();
  });

  await act(async () => {
    document.querySelector<HTMLButtonElement>(".tool__head")?.click();
    await flush();
    await flush();
  });

  ok(document.body.textContent?.includes("无法加载工具详情。") === true, "user-opened archived tool load failures show a visible error");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const openedMediaPaths: string[] = [];
  mockApp({
    ReadFileForTab: async (_tabID, path) => ({
      path,
      body: "",
      size: 12,
      truncated: false,
      binary: false,
      kind: path.endsWith(".mp4") ? "video" : "image",
      mime: path.endsWith(".mp4") ? "video/mp4" : "image/png",
      url: path.endsWith(".mp4") ? "/media/clip.mp4" : "/media/image.png",
    }),
    OpenWorkspacePathForTab: async (_tabID, path) => { openedMediaPaths.push(path); },
  });
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const item: ToolItem = {
    kind: "tool",
    id: "call-media-1",
    name: "generate_video",
    args: JSON.stringify({ prompt: "A generated clip" }),
    readOnly: false,
    status: "done",
    output: JSON.stringify({
      action: "generate_video",
      media: [
        { type: "image", path: "D:\\workspace\\.clouvia\\generated-media\\image.png", mime_type: "image/png" },
        { type: "video", path: "D:\\workspace\\.clouvia\\generated-media\\clip.mp4", mime_type: "video/mp4" },
      ],
    }),
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={item} tabId="tab-media" />
      </LocaleProvider>,
    );
    await flush();
    await flush();
  });

  ok(document.querySelector<HTMLImageElement>(".tool-media img")?.getAttribute("src") === "/media/image.png", "media tool renders generated image");
  const video = document.querySelector<HTMLVideoElement>(".tool-media video");
  ok(video?.getAttribute("src") === "/media/clip.mp4", "media tool renders generated video");
  ok(video?.hasAttribute("controls") === true, "generated video exposes playback controls");
  const mediaPaths = Array.from(document.querySelectorAll<HTMLElement>(".tool-media figcaption")).map((caption) => caption.textContent);
  ok(mediaPaths.includes("D:\\workspace\\.clouvia\\generated-media\\image.png"), "image preview shows its full file path");
  ok(mediaPaths.includes("D:\\workspace\\.clouvia\\generated-media\\clip.mp4"), "video preview shows its full file path");
  const mediaLinks = Array.from(document.querySelectorAll<HTMLAnchorElement>(".tool-media figcaption a"));
  ok(mediaLinks.length === 2, "generated image and video captions render as artifact hyperlinks");
  await act(async () => {
    mediaLinks[0]?.click();
    mediaLinks[1]?.click();
    await flush();
  });
  ok(openedMediaPaths[0] === "D:\\workspace\\.clouvia\\generated-media\\image.png", "image caption opens its local artifact");
  ok(openedMediaPaths[1] === "D:\\workspace\\.clouvia\\generated-media\\clip.mp4", "video caption opens its local artifact");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const item: ToolItem = {
    kind: "tool",
    id: "call-media-progress",
    name: "generate_video",
    args: JSON.stringify({ prompt: "Ocean at sunrise" }),
    readOnly: false,
    status: "running",
    output: [
      JSON.stringify({ kind: "video_generation_progress", task_id: "vgen_01", status: "queued", progress_percent: 20 }),
      JSON.stringify({ kind: "video_generation_progress", task_id: "vgen_01", status: "running", progress_percent: 70 }),
    ].join("\n") + "\n",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={item} />
      </LocaleProvider>,
    );
    await flush();
  });

  const progress = document.querySelector<HTMLElement>(".tool-video-progress");
  const bar = progress?.querySelector<HTMLElement>('[role="progressbar"]');
  ok(progress?.textContent?.includes("视频生成进度") === true, "running video shows cloud progress");
  ok(progress?.textContent?.includes("vgen_01") === true, "video progress shows the cloud task ID");
  ok(bar?.getAttribute("aria-valuenow") === "70", "video progress uses the latest percentage");
  ok(bar?.querySelector<HTMLElement>("span")?.style.width === "70%", "video progress bar reflects the latest percentage");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const running: ToolItem = {
    kind: "tool",
    id: "call-shell-regroup",
    name: "bash",
    args: JSON.stringify({ command: "wait 30 seconds" }),
    readOnly: false,
    status: "running",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolDisclosureProvider>
          <ToolCard item={running} />
        </ToolDisclosureProvider>
      </LocaleProvider>,
    );
    await flush();
  });
  await act(async () => {
    document.querySelector<HTMLButtonElement>(".tool__head")?.click();
    await flush();
  });
  ok(document.querySelector(".tool--open") !== null, "user expands the running shell before regrouping");

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolDisclosureProvider>
          <ToolGroup
            kind="shell"
            items={[{ ...running, status: "done", output: "TOOL-DISCLOSURE-DONE" }]}
            subcalls={new Map()}
          />
        </ToolDisclosureProvider>
      </LocaleProvider>,
    );
    await flush();
    await flush();
  });
  ok(document.querySelector(".tool-group--open") !== null, "completed shell regroup inherits the user's disclosure choice");
  ok(document.querySelector(".tool--open") !== null, "regrouped shell card remains expanded after remount");
  ok(document.body.textContent?.includes("TOOL-DISCLOSURE-DONE") === true, "regrouped shell keeps its completed output visible");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const prior: ToolItem = {
    kind: "tool",
    id: "call-shell-prior",
    name: "bash",
    args: JSON.stringify({ command: "printf PRIOR" }),
    output: "PRIOR",
    readOnly: false,
    status: "done",
  };
  const running: ToolItem = {
    kind: "tool",
    id: "call-shell-joins-existing-group",
    name: "bash",
    args: JSON.stringify({ command: "wait 30 seconds" }),
    readOnly: false,
    status: "running",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolDisclosureProvider>
          <div>
            <ToolGroup key="stable-shell-group" kind="shell" items={[prior]} subcalls={new Map()} />
            <ToolCard key={running.id} item={running} />
          </div>
        </ToolDisclosureProvider>
      </LocaleProvider>,
    );
    await flush();
  });
  await act(async () => {
    const toolHeads = document.querySelectorAll<HTMLButtonElement>(".tool__head");
    toolHeads[toolHeads.length - 1]?.click();
    await flush();
  });

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolDisclosureProvider>
          <div>
            <ToolGroup
              key="stable-shell-group"
              kind="shell"
              items={[prior, { ...running, status: "done", output: "JOINED-GROUP-DONE" }]}
              subcalls={new Map()}
            />
          </div>
        </ToolDisclosureProvider>
      </LocaleProvider>,
    );
    await flush();
    await flush();
  });
  ok(document.querySelector(".tool-group--open") !== null, "existing shell group opens when it absorbs a user-opened running card");
  ok(Array.from(document.querySelectorAll(".tool--open")).length === 1, "only the user-opened card stays expanded inside the existing group");
  ok(document.body.textContent?.includes("JOINED-GROUP-DONE") === true, "existing group exposes the completed output inherited from the running card");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

ok(
  shouldAutoExpandToolGroup([{ name: "generate_image" }, { name: "edit_file" }]),
  "media tool automatically expands its tool group",
);
ok(
  !shouldAutoExpandToolGroup([{ name: "edit_file" }]),
  "ordinary tool groups remain collapsed",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
