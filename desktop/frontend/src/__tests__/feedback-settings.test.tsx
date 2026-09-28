// Run: tsx src/__tests__/feedback-settings.test.tsx

import { JSDOM } from "jsdom";
import React, { useState } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";

import {
  FeedbackSettingsSection,
  resolveFeedbackAccountState,
  useFeedbackDraft,
  type FeedbackAccountState,
} from "../features/settings/ui/FeedbackSettingsSection";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { FeedbackImageView, FeedbackSubmitRequest, FeedbackSubmitView } from "../lib/types";

let passed = 0;
let failed = 0;

function ok(value: unknown, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq(actual: unknown, expected: unknown, label: string) {
  ok(JSON.stringify(actual) === JSON.stringify(expected), `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function settle() {
  await act(async () => {
    await flushPromises();
  });
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    await settle();
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

const dom = new JSDOM("<!doctype html><html><body></body></html>", {
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
globalThis.CustomEvent = dom.window.CustomEvent;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
Object.defineProperty(globalThis.window, "matchMedia", {
  configurable: true,
  value: () => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} }),
});
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });

type HarnessProps = {
  accountState: FeedbackAccountState;
  accountScopeKey: string;
  onRequireAccount?: () => void;
  onClose?: () => void;
};

function Harness({
  accountState,
  accountScopeKey,
  onRequireAccount = () => {},
  onClose = () => {},
}: HarnessProps) {
  return (
    <LocaleProvider>
      <HarnessContent
        accountState={accountState}
        accountScopeKey={accountScopeKey}
        onRequireAccount={onRequireAccount}
        onClose={onClose}
      />
    </LocaleProvider>
  );
}

function HarnessContent({
  accountState,
  accountScopeKey,
  onRequireAccount = () => {},
  onClose = () => {},
}: HarnessProps) {
  const [visible, setVisible] = useState(true);
  const controller = useFeedbackDraft({ accountState, accountScopeKey, onRequireAccount });
  return (
    <>
      <button data-test="toggle" type="button" onClick={() => setVisible((current) => !current)}>切换页面</button>
      <button data-test="close" type="button" onClick={() => void controller.requestClose(onClose)}>关闭设置</button>
      {visible && <FeedbackSettingsSection controller={controller} />}
    </>
  );
}

function createScenario(props: HarnessProps) {
  const host = document.createElement("div");
  document.body.appendChild(host);
  const root = createRoot(host);
  const render = async (nextProps = props) => {
    await act(async () => {
      root.render(<Harness {...nextProps} />);
      await flushPromises();
    });
  };
  return { host, root, render };
}

function textButton(host: HTMLElement, label: string): HTMLButtonElement {
  const button = [...host.querySelectorAll<HTMLButtonElement>("button")]
    .find((candidate) => candidate.textContent?.trim() === label);
  if (!button) throw new Error(`missing button: ${label}`);
  return button;
}

async function click(button: HTMLButtonElement) {
  await act(async () => {
    button.click();
    await flushPromises();
  });
}

async function enterDescription(host: HTMLElement, value: string) {
  const textarea = host.querySelector<HTMLTextAreaElement>("#feedback-description");
  if (!textarea) throw new Error("missing feedback textarea");
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, "value")?.set;
    if (setter) setter.call(textarea, value);
    else textarea.value = value;
    const reactPropsKey = Object.keys(textarea).find((key) => key.startsWith("__reactProps$"));
    const reactProps = reactPropsKey
      ? (textarea as unknown as Record<string, {
        onChange?: (event: { currentTarget: HTMLTextAreaElement; target: HTMLTextAreaElement }) => void;
      }>)[reactPropsKey]
      : undefined;
    reactProps?.onChange?.({ currentTarget: textarea, target: textarea });
    await flushPromises();
  });
}

function installBindings(bindings: Partial<AppBindings>) {
  window.go = { main: { App: bindings as AppBindings } };
}

console.log("\nfeedback settings");

eq(
  resolveFeedbackAccountState({ initialized: false, available: true, status: "authenticated", hasRecoverableIdentity: true }),
  "loading",
  "account remains loading until initialization settles",
);
eq(
  resolveFeedbackAccountState({ initialized: true, available: true, status: "offline", hasRecoverableIdentity: true }),
  "ready",
  "recoverable offline identity can submit independently of subscription state",
);
eq(
  resolveFeedbackAccountState({ initialized: true, available: true, status: "expired", hasRecoverableIdentity: true }),
  "expired",
  "expired account requires reauthentication",
);
eq(
  resolveFeedbackAccountState({ initialized: true, available: false, status: "authenticated", hasRecoverableIdentity: true }),
  "unavailable",
  "unavailable cloud binding fails closed",
);

let loginCalls = 0;
installBindings({
  DiscardFeedbackDraft: async () => {},
  ConfirmAction: async () => true,
});
const anonymous = createScenario({
  accountState: "anonymous",
  accountScopeKey: "",
  onRequireAccount: () => { loginCalls += 1; },
});
await anonymous.render();
ok(anonymous.host.querySelector<HTMLTextAreaElement>("#feedback-description")?.disabled, "anonymous feedback form is disabled");
ok(textButton(anonymous.host, "提交反馈").disabled, "anonymous submission is disabled");
await click(textButton(anonymous.host, "去登录"));
eq(loginCalls, 1, "anonymous account action uses the existing login entry");
await act(async () => anonymous.root.unmount());
anonymous.host.remove();

const token = `feedback:${"a".repeat(48)}`;
const image: FeedbackImageView = {
  token,
  filename: "页面截图.png",
  content_type: "image/png",
  thumbnail_data_url: "data:image/jpeg;base64,/9j/2Q==",
  size_bytes: 1536,
};
const submitRequests: FeedbackSubmitRequest[] = [];
const submitResults: FeedbackSubmitView[] = [
  {
    replayed: false,
    operation_error: {
      code: "NETWORK_ERROR",
      message: "网络连接失败，请检查网络后重试。",
      retryable: true,
      request_id: "req_feedback_001",
      diagnostic_id: "log_feedback_001",
    },
  },
  {
    replayed: false,
    operation_error: {
      code: "NETWORK_ERROR",
      message: "网络连接失败，请检查网络后重试。",
      retryable: true,
      request_id: "req_feedback_002",
    },
  },
  { report_id: "errpt_feedback_001", replayed: false },
];
let pickerCalls = 0;
const discarded: string[][] = [];
installBindings({
  PickFeedbackImages: async () => {
    pickerCalls += 1;
    return { images: [image] };
  },
  RemoveFeedbackImage: async () => {},
  DiscardFeedbackDraft: async (tokens) => { discarded.push(tokens); },
  SubmitFeedback: async (request) => {
    submitRequests.push(request);
    return submitResults.shift() ?? { report_id: "errpt_fallback", replayed: false };
  },
  ConfirmAction: async () => true,
});
const ready = createScenario({ accountState: "ready", accountScopeKey: "profile-1\u0000user-1\u00001" });
await ready.render();
await enterDescription(ready.host, "导出文件时应用闪退");
await click(textButton(ready.host, "添加图片"));
await waitFor("selected feedback image", () => ready.host.textContent?.includes("页面截图.png") === true);
eq(pickerCalls, 1, "image button invokes the native picker exactly once");
ok(ready.host.textContent?.includes("C:\\") === false, "feedback DOM contains no local path");
ok(ready.host.innerHTML.includes("data:image/png;base64") === false, "feedback DOM contains no original image payload");

await click(textButton(ready.host, "切换页面"));
ok(ready.host.querySelector("#feedback-description") === null, "switching away unmounts only the feedback page");
await click(textButton(ready.host, "切换页面"));
eq(ready.host.querySelector<HTMLTextAreaElement>("#feedback-description")?.value, "导出文件时应用闪退", "draft text survives settings tab switches");
ok(ready.host.textContent?.includes("页面截图.png"), "draft image survives settings tab switches");

await click(textButton(ready.host, "提交反馈"));
await waitFor("first structured failure", () => ready.host.textContent?.includes("req_feedback_001") === true);
eq(submitRequests.length, 1, "one explicit submit performs one bridge call");
eq(submitRequests[0]?.description, "导出文件时应用闪退", "submission trims and forwards the description");
eq(submitRequests[0]?.image_tokens, [token], "submission forwards only opaque image tokens");
ok(ready.host.textContent?.includes("网络连接失败，请检查网络后重试。"), "structured Chinese cloud error is displayed");
ok(ready.host.textContent?.includes("NETWORK_ERROR"), "structured error code remains available for diagnosis");
ok(ready.host.textContent?.includes("log_feedback_001"), "structured feedback error renders its Log ID separately");
ok(ready.host.querySelector('[data-feedback-error-log-id] .copybtn') !== null, "structured feedback error renders a Log ID copy control");

await click(textButton(ready.host, "提交反馈"));
await waitFor("second structured failure", () => ready.host.textContent?.includes("req_feedback_002") === true);
eq(submitRequests[1]?.attempt_id, submitRequests[0]?.attempt_id, "unchanged manual retry reuses the same attempt id");
eq(submitRequests.length, 2, "failed submission is never retried automatically");

await enterDescription(ready.host, "导出图片时应用闪退");
await click(textButton(ready.host, "提交反馈"));
await waitFor("successful feedback", () => ready.host.textContent?.includes("errpt_feedback_001") === true);
ok(submitRequests[2]?.attempt_id !== submitRequests[1]?.attempt_id, "editing the draft creates a new attempt id");
eq(ready.host.querySelector<HTMLTextAreaElement>("#feedback-description")?.value, "", "successful submission clears the description");
ok(ready.host.textContent?.includes("页面截图.png") === false, "successful submission clears image metadata");
ok(ready.host.textContent?.includes("反馈已提交"), "successful submission displays a Chinese confirmation");
await act(async () => ready.root.unmount());
ready.host.remove();

let confirmResult = false;
let confirmCalls = 0;
let closeCalls = 0;
installBindings({
  PickFeedbackImages: async () => ({ images: [image] }),
  RemoveFeedbackImage: async () => {},
  SubmitFeedback: async () => new Promise<FeedbackSubmitView>(() => {}),
  DiscardFeedbackDraft: async (tokens) => { discarded.push(tokens); },
  ConfirmAction: async () => {
    confirmCalls += 1;
    return confirmResult;
  },
});
const guarded = createScenario({
  accountState: "ready",
  accountScopeKey: "profile-2\u0000user-2\u00001",
  onClose: () => { closeCalls += 1; },
});
await guarded.render();
await enterDescription(guarded.host, "尚未提交的反馈");
await click(guarded.host.querySelector<HTMLButtonElement>('[data-test="close"]')!);
eq(confirmCalls, 1, "dirty close asks for confirmation");
eq(closeCalls, 0, "cancelled confirmation keeps settings open");
confirmResult = true;
await click(guarded.host.querySelector<HTMLButtonElement>('[data-test="close"]')!);
eq(confirmCalls, 2, "confirmed close uses the same native guard");
eq(closeCalls, 1, "confirmed close continues to the settings close action");
eq(guarded.host.querySelector<HTMLTextAreaElement>("#feedback-description")?.value, "", "confirmed close clears the draft");
await act(async () => guarded.root.unmount());
guarded.host.remove();

dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
