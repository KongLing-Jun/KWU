// Run: tsx src/__tests__/artifact-delivery-contract.test.tsx

import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { AssistantMessage } from "../components/Message";
import { historyMessagesToItems } from "../features/session/historyProjection";
import { reducer } from "../features/session/useController";
import { initialState } from "../features/session/sessionState";
import { MAX_ARTIFACT_DELIVERY_PATHS, normalizeArtifactDeliveryV1 } from "../lib/artifactDelivery";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { AppBindings } from "../lib/bridge";
import type { ArtifactDelivery } from "../lib/types";

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
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

function flushTimers(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function renderDelivery(delivery: ArtifactDelivery, reads: string[]) {
  document.body.innerHTML = "<div id=\"root\"></div>";
  window.go = {
    main: {
      App: {
        ReadFileForTab: async (_tabID: string, path: string) => {
          reads.push(path);
          return { path, body: "", size: 1, truncated: false, binary: false };
        },
        OpenWorkspacePathForTab: async () => {},
        AttachmentDataURL: async () => "",
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToastProvider>
          <AssistantMessage
            tabId="delivery-tab"
            item={{
              kind: "assistant",
              id: "delivery",
              text: "已交付 `joke.md`，另有 [legacy.xlsx](legacy.xlsx)。",
              reasoning: "",
              streaming: false,
              artifactPaths: ["legacy.xlsx"],
              artifactDelivery: delivery,
            }}
          />
        </ToastProvider>
      </LocaleProvider>,
    );
    await flushTimers();
  });
  return root;
}

async function renderLegacyCandidate(
  id: string,
  finalAnswer: boolean | undefined,
  streaming: boolean,
  reads: string[],
  artifactPaths: string[] = ["report.md"],
  text = "[report.md](report.md)",
) {
  document.body.innerHTML = "<div id=\"root\"></div>";
  window.go = {
    main: {
      App: {
        ReadFileForTab: async (_tabID: string, path: string) => {
          reads.push(path);
          return { path, body: "", size: 1, truncated: false, binary: false };
        },
        OpenWorkspacePathForTab: async () => {},
        AttachmentDataURL: async () => "",
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToastProvider>
          <AssistantMessage
            tabId="delivery-tab"
            item={{
              kind: "assistant",
              id,
              text,
              reasoning: "",
              streaming,
              artifactPaths,
              ...(finalAnswer === undefined ? {} : { finalAnswer }),
            }}
          />
        </ToastProvider>
      </LocaleProvider>,
    );
    await flushTimers();
  });
  return root;
}

const committedReads: string[] = [];
const committedRoot = await renderDelivery({ version: 1, paths: ["joke.md"] }, committedReads);
assert.deepEqual(committedReads, ["joke.md"]);
const cards = Array.from(document.querySelectorAll<HTMLButtonElement>("button.msg-attachment"));
assert.equal(cards.length, 1);
assert.equal(cards[0]?.getAttribute("title"), "joke.md");
await act(async () => committedRoot.unmount());

const multipleReads: string[] = [];
const multipleRoot = await renderDelivery({ version: 1, paths: ["reports/workbuddy.md", "reports/sources.pdf"] }, multipleReads);
assert.deepEqual(multipleReads, ["reports/workbuddy.md", "reports/sources.pdf"]);
const multipleCards = Array.from(document.querySelectorAll<HTMLButtonElement>("button.msg-attachment"));
assert.equal(multipleCards.length, 2);
assert.deepEqual(multipleCards.map((card) => card.getAttribute("title")), ["reports/workbuddy.md", "reports/sources.pdf"]);
await act(async () => multipleRoot.unmount());

const emptyReads: string[] = [];
const emptyRoot = await renderDelivery({ version: 1, paths: [] }, emptyReads);
assert.deepEqual(emptyReads, []);
assert.equal(document.querySelectorAll("button.msg-attachment").length, 0);
await act(async () => emptyRoot.unmount());

const futureReads: string[] = [];
const futureRoot = await renderDelivery({ version: 2, paths: ["future.pdf"] }, futureReads);
assert.deepEqual(futureReads, []);
assert.equal(document.querySelectorAll("button.msg-attachment").length, 0);
await act(async () => futureRoot.unmount());

const legacySuccessReads: string[] = [];
const legacySuccessRoot = await renderLegacyCandidate("history-legacy-final", true, false, legacySuccessReads);
assert.deepEqual(legacySuccessReads, ["report.md"]);
assert.equal(document.querySelectorAll("button.msg-attachment").length, 1);
await act(async () => legacySuccessRoot.unmount());

const legacyAliasReads: string[] = [];
const legacyAliasRoot = await renderLegacyCandidate(
  "history-legacy-aliases",
  true,
  false,
  legacyAliasReads,
  ["C:\\workspace\\WorkBuddy-report.md", "./WorkBuddy-report.md"],
  "[WorkBuddy-report.md](./WorkBuddy-report.md)",
);
assert.deepEqual(legacyAliasReads, ["C:\\workspace\\WorkBuddy-report.md", "./WorkBuddy-report.md"]);
const legacyAliasCards = Array.from(document.querySelectorAll<HTMLButtonElement>("button.msg-attachment"));
assert.equal(legacyAliasCards.length, 1);
assert.equal(legacyAliasCards[0]?.getAttribute("title"), "C:\\workspace\\WorkBuddy-report.md");
await act(async () => legacyAliasRoot.unmount());

for (const scenario of [
  { id: "streaming-candidate", finalAnswer: true, streaming: true },
  { id: "failed-candidate", finalAnswer: undefined, streaming: false },
  { id: "cancelled-candidate", finalAnswer: undefined, streaming: false },
  { id: "rejected-candidate", finalAnswer: undefined, streaming: false },
]) {
  const reads: string[] = [];
  const root = await renderLegacyCandidate(scenario.id, scenario.finalAnswer, scenario.streaming, reads);
  assert.deepEqual(reads, [], `${scenario.id} must not probe a legacy artifact`);
  assert.equal(document.querySelectorAll("button.msg-attachment").length, 0, `${scenario.id} Markdown link must not become a card`);
  await act(async () => root.unmount());
}

assert.equal(normalizeArtifactDeliveryV1({ version: 2, paths: ["future.pdf"] }), undefined);
for (const path of [
  "report.json",
  "report.yaml",
  "report.custom",
  "docs/workbuddy-report.md",
  "assets/chart.png",
  "public/final.pdf",
  "static/export.json",
  "README",
  "src/main.go",
  "logs/run.log",
  "build/output.json",
  ".hidden/cache/file.bin",
  ".customer-delivery/report.md",
  ".customer-delivery/docs/report.custom",
]) {
  assert.deepEqual(
    normalizeArtifactDeliveryV1({ version: 1, paths: [path], deliveredAt: 1234 }),
    { version: 1, paths: [path], deliveredAt: 1234 },
    `${path} is a valid explicit deliverable`,
  );
}
assert.deepEqual(normalizeArtifactDeliveryV1({ version: 1, paths: [] }), { version: 1, paths: [] });

const malformedContracts: Array<{ label: string; value: unknown }> = [
  { label: "leading whitespace", value: { version: 1, paths: [" report.pdf"] } },
  { label: "backslash", value: { version: 1, paths: ["reports\\report.pdf"] } },
  { label: "absolute path", value: { version: 1, paths: ["C:/reports/report.pdf"] } },
  { label: "parent traversal", value: { version: 1, paths: ["../report.pdf"] } },
  { label: "duplicate path", value: { version: 1, paths: ["report.pdf", "report.pdf"] } },
  {
    label: "too many paths",
    value: {
      version: 1,
      paths: Array.from({ length: MAX_ARTIFACT_DELIVERY_PATHS + 1 }, (_, index) => `report-${index}.md`),
    },
  },
];

for (const malformed of malformedContracts) {
  assert.equal(normalizeArtifactDeliveryV1(malformed.value), undefined, `${malformed.label} rejects the entire v1 contract`);
  const reads: string[] = [];
  const root = await renderDelivery(malformed.value as ArtifactDelivery, reads);
  assert.deepEqual(reads, [], `${malformed.label} must not probe contract or legacy paths`);
  assert.equal(
    document.querySelectorAll("button.msg-attachment").length,
    0,
    `${malformed.label} must suppress every attachment card`,
  );
  await act(async () => root.unmount());
}

const committedHistory = historyMessagesToItems([
  { role: "assistant", content: "", finalAnswer: true, artifactDelivery: { version: 1, paths: [] } },
], "history:");
assert.equal(committedHistory.items.length, 1);
assert.deepEqual(
  committedHistory.items[0]?.kind === "assistant" ? committedHistory.items[0].artifactDelivery : undefined,
  { version: 1, paths: [] },
);
const futureHistory = historyMessagesToItems([
  { role: "assistant", content: "", finalAnswer: true, artifactDelivery: { version: 2, paths: ["future.pdf"] } },
], "history:");
assert.equal(futureHistory.items.length, 1);
assert.deepEqual(
  futureHistory.items[0]?.kind === "assistant" ? futureHistory.items[0].artifactDelivery : undefined,
  { version: 2, paths: ["future.pdf"] },
);

let futureEventState = reducer(initialState, {
  type: "event",
  e: { kind: "turn_started", turnId: "turn-v2" },
});
futureEventState = reducer(futureEventState, {
  type: "event",
  e: { kind: "message", turnId: "turn-v2", candidateId: "candidate-v2", text: "[legacy.xlsx](legacy.xlsx)" },
});
futureEventState = reducer(futureEventState, {
  type: "event",
  e: { kind: "turn_done", turnId: "turn-v2", artifactDelivery: { version: 2, paths: ["future.pdf"] } },
});
const futureEventAssistant = futureEventState.items.find((item) => item.kind === "assistant");
assert.deepEqual(
  futureEventAssistant?.kind === "assistant" ? futureEventAssistant.artifactDelivery : undefined,
  { version: 2, paths: ["future.pdf"] },
);

console.log("artifact delivery contract: PASS");
