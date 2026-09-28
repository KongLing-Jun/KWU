// Run: tsx src/__tests__/expert-directory-backend.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { fireEvent } from "@testing-library/dom";
import { ExpertDirectory, type ExpertSelection } from "../components/ExpertDirectory";
import { LocaleProvider } from "../lib/i18n";
import type { ExpertInfo, UserExpertInfo } from "../lib/types";

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

async function waitFor(label: string, fn: () => boolean) {
  for (let i = 0; i < 20; i++) {
    if (fn()) return;
    await act(async () => {
      await flush();
    });
  }
  throw new Error(`timed out waiting for ${label}`);
}

function clonedExpert(expert: Record<string, unknown>): Record<string, unknown> {
  return {
    ...expert,
    categories: [...((expert.categories as string[] | undefined) ?? [])],
    tags: [...((expert.tags as string[] | undefined) ?? [])],
    members: [...((expert.members as unknown[] | undefined) ?? [])],
    prompts: [...((expert.prompts as string[] | undefined) ?? [])],
  };
}

function installDom() {
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
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.HTMLInputElement = dom.window.HTMLInputElement;
  globalThis.HTMLSelectElement = dom.window.HTMLSelectElement;
  globalThis.HTMLTextAreaElement = dom.window.HTMLTextAreaElement;
  globalThis.File = dom.window.File;
  globalThis.FileReader = dom.window.FileReader;
  globalThis.Event = dom.window.Event;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: () => ({
      matches: true,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    }),
  });
  return dom;
}

console.log("\nexpert directory backend catalog");

const dom = installDom();
let selected: ExpertSelection | null = null;
let listCalls = 0;
let createCalls = 0;
let updateCalls = 0;
let deleteCalls = 0;
let lastCreatedInput: Record<string, unknown> | null = null;
let lastUpdatedInput: Record<string, unknown> | null = null;
const selectedValue = (): ExpertSelection | null => selected;
const createdInputValue = (): Record<string, unknown> | null => lastCreatedInput;
const updatedInputValue = (): Record<string, unknown> | null => lastUpdatedInput;
let backendExperts: Record<string, unknown>[] = [{
  id: "backend-only",
  source: "official",
  mode: "expert",
  name: "Backend Only Expert",
  role: "代码交付",
  description: "Only exists in the mocked backend catalog.",
  categories: ["代码交付"],
  tags: ["backend"],
  iconKey: "bot",
  tone: "stone",
  avatarKey: "architect",
  members: [],
  prompts: ["Use the backend expert prompt."],
}, {
  id: "exp_architect",
  source: "official",
  mode: "expert",
  name: "Fixed Architect Recommendation",
  role: "代码交付",
  description: "Fixed recommendation backed by the system catalog.",
  categories: ["代码交付"],
  tags: ["fixed"],
  iconKey: "bot",
  tone: "blue",
  members: [],
  prompts: ["Review the architecture."],
}, {
  id: "exp_code_review",
  source: "official",
  mode: "expert",
  name: "Fixed Review Recommendation",
  role: "代码交付",
  description: "Second fixed recommendation backed by the system catalog.",
  categories: ["代码交付"],
  tags: ["fixed"],
  iconKey: "bot",
  tone: "violet",
  members: [],
  prompts: ["Review the code."],
}, {
  id: "exg_code_delivery_team",
  source: "official",
  mode: "team",
  name: "Fixed Delivery Team Recommendation",
  role: "代码交付",
  description: "Fixed expert-team recommendation backed by the system catalog.",
  categories: ["代码交付"],
  tags: ["fixed"],
  iconKey: "users",
  tone: "teal",
  members: [],
  prompts: ["Coordinate code delivery."],
}, {
  id: "worker-agent",
  source: "official",
  mode: "expert",
  name: "Worker Agent",
  role: "Execution",
  description: "Standalone worker agent with an avatar.",
  categories: ["Backend"],
  tags: ["worker"],
  iconKey: "bot",
  tone: "green",
  avatarKey: "research",
  members: [],
  prompts: ["Work as the standalone worker.", "Review the worker plan.", "List the worker next steps."],
}, {
  id: "backend-data-expert",
  source: "official",
  mode: "expert",
  name: "Backend Data Expert",
  role: "数据分析",
  description: "Only handles data analysis.",
  categories: ["数据分析"],
  tags: ["data"],
  iconKey: "bot",
  tone: "violet",
  members: [],
  prompts: ["Analyze the data.", "Review data quality.", "Plan the analysis."],
}, {
  id: "backend-agent-team",
  source: "official",
  mode: "team",
  name: "Backend Agent Team",
  role: "代码交付",
  description: "Coordinates one lead agent and one worker agent.",
  categories: ["代码交付"],
  tags: ["Team Domain Category", "agents"],
  iconKey: "users",
  tone: "teal",
  avatarKey: "office-automation-team",
  members: [
    { kind: "lead", expertId: "backend-only", name: "Lead Agent", role: "Coordinator", task: "Plan the work.", output: "Plan", tone: "blue" },
    { kind: "sub", name: "Worker Agent", role: "Execution", task: "Do the work.", output: "Result", tone: "green" },
  ],
  prompts: ["Coordinate the backend agent team.", "Review the agent team risks.", "Plan the agent team delivery."],
}, {
  id: "backend-data-team",
  source: "official",
  mode: "team",
  name: "Backend Data Team",
  role: "数据分析",
  description: "Analyzes backend data.",
  categories: ["数据分析", "Expert Team Only Category"],
  tags: ["data"],
  iconKey: "users",
  tone: "violet",
  members: [],
  prompts: ["Analyze the data.", "Review data quality.", "Plan the analysis."],
}];
(window as unknown as {
  go: { main: { App: {
    ListExperts: () => Promise<unknown[]>;
    CreateExpert: (input: Record<string, unknown>) => Promise<unknown>;
    UpdateExpert: (id: string, input: Record<string, unknown>) => Promise<unknown>;
    DeleteExpert: (id: string) => Promise<void>;
  } } };
}).go = {
  main: {
    App: {
      async ListExperts() {
        listCalls += 1;
        return backendExperts.map(clonedExpert);
      },
      async CreateExpert(input) {
        createCalls += 1;
        lastCreatedInput = input;
        const created = clonedExpert(input);
        backendExperts = [created, ...backendExperts.filter((item) => item.id !== created.id)];
        return clonedExpert(created);
      },
      async UpdateExpert(id, input) {
        updateCalls += 1;
        lastUpdatedInput = input;
        const updated = { ...clonedExpert(input), id };
        backendExperts = backendExperts.map((item) => item.id === id ? updated : item);
        return clonedExpert(updated);
      },
      async DeleteExpert(id) {
        deleteCalls += 1;
        backendExperts = backendExperts.filter((item) => item.id !== id);
      },
    },
  },
};

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);

await act(async () => {
  root.render(
    <LocaleProvider>
      <ExpertDirectory authenticated onSummon={(selection) => {
        selected = selection;
      }} />
    </LocaleProvider>,
  );
  await flush();
});

await waitFor("backend expert", () => document.body.textContent?.includes("Backend Only Expert") === true);
ok(listCalls === 1, "ExpertDirectory loads experts from backend once");
ok(document.querySelector(".expert-reset-button") === null, "expert directory omits the reset experts action");
ok(document.body.textContent?.includes("Only exists in the mocked backend catalog.") === true, "renders backend-provided expert details");
ok(document.querySelector(".expert-avatar--architect") === null, "backend catalog does not resurrect static experts that the backend omitted");
const backendExpertCategory = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-category"))
  .find((button) => button.textContent?.trim() === "代码交付");
ok(Boolean(backendExpertCategory), "expert categories include backend-provided category values");
const backendExpertDomainCategory = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-category"))
  .find((button) => button.textContent?.trim() === "Team Domain Category");
ok(Boolean(backendExpertDomainCategory), "expert categories include domains inherited from expert-team membership");
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-category"))
    .some((button) => button.textContent?.trim() === "Expert Team Only Category"),
  "expert mode category navigation includes categories found only on expert teams",
);
const categoryScroller = document.querySelector<HTMLDivElement>(".expert-catalog-nav .expert-category-row");
if (!categoryScroller) throw new Error("missing expert category scroller");
Object.defineProperties(categoryScroller, {
  clientWidth: { configurable: true, value: 200 },
  scrollWidth: { configurable: true, value: 600 },
  scrollLeft: { configurable: true, value: 0, writable: true },
});
let capturedCategoryPointer: number | null = null;
Object.defineProperties(categoryScroller, {
  setPointerCapture: { configurable: true, value: (pointerId: number) => { capturedCategoryPointer = pointerId; } },
  hasPointerCapture: { configurable: true, value: (pointerId: number) => capturedCategoryPointer === pointerId },
  releasePointerCapture: { configurable: true, value: (pointerId: number) => { if (capturedCategoryPointer === pointerId) capturedCategoryPointer = null; } },
});
const categoryWheel = new window.WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 80 });
categoryScroller.dispatchEvent(categoryWheel);
ok(categoryScroller.scrollLeft === 80 && categoryWheel.defaultPrevented, "vertical wheel input scrolls expert categories horizontally");
const categoryDragTarget = categoryScroller.querySelector<HTMLButtonElement>(".expert-category:nth-child(3)");
if (!categoryDragTarget) throw new Error("missing expert category drag target");
const dispatchCategoryPointer = (target: Element, type: string, clientX: number) => {
  const event = new window.Event(type, { bubbles: true, cancelable: true });
  Object.defineProperties(event, {
    button: { configurable: true, value: 0 },
    clientX: { configurable: true, value: clientX },
    pointerId: { configurable: true, value: 1 },
    pointerType: { configurable: true, value: "mouse" },
  });
  target.dispatchEvent(event);
  return event;
};
dispatchCategoryPointer(categoryDragTarget, "pointerdown", 160);
dispatchCategoryPointer(categoryDragTarget, "pointermove", 80);
dispatchCategoryPointer(categoryDragTarget, "pointerup", 80);
ok(categoryScroller.scrollLeft === 160, "dragging left reveals later expert categories");
categoryDragTarget.click();
ok(categoryDragTarget.classList.contains("expert-category--active") === false, "finishing an expert category drag does not select its button");
await act(async () => {
  backendExpertDomainCategory?.click();
  await flush();
});
ok(
  document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Only Expert") === true &&
    document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Data Expert") === false,
  "expert-team domain categories filter their member experts",
);
await act(async () => {
  backendExpertCategory?.click();
  await flush();
});
ok(
  document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Only Expert") === true &&
    document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Data Expert") === false,
  "experts are classified by the backend category field",
);
const sceneTitles = Array.from(document.querySelectorAll<HTMLElement>(".expert-scene-card__head strong"))
  .map((node) => node.textContent?.trim());
ok(sceneTitles[sceneTitles.length - 1] === "\u4ee3\u7801\u4ea4\u4ed8", "recommended scenes place code delivery last");
ok(document.querySelectorAll(".expert-scene-card__background").length === 4, "recommended scenes render one background image per card");
const codeDeliveryScene = Array.from(document.querySelectorAll<HTMLElement>(".expert-scene-card"))
  .find((scene) => scene.querySelector(".expert-scene-card__head")?.textContent?.trim() === "代码交付");
ok(
  codeDeliveryScene?.textContent?.includes("Fixed Architect Recommendation") === true &&
    codeDeliveryScene?.textContent?.includes("Fixed Review Recommendation") === true &&
    codeDeliveryScene?.textContent?.includes("Fixed Delivery Team Recommendation") === true &&
    codeDeliveryScene?.textContent?.includes("Backend Only Expert") === false &&
    codeDeliveryScene?.textContent?.includes("Backend Agent Team") === false,
  "recommended scenes use fixed system expert and expert-team ids",
);
ok(
  document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Only Expert") === true,
  "non-recommended backend experts remain visible in the complete catalog",
);

const card = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-card")).find((button) => button.textContent?.includes("Backend Only Expert"));
if (!card) throw new Error("missing backend expert card");
const summonButtonFor = (label: string) => Array.from(document.querySelectorAll<HTMLElement>(".expert-card-shell"))
  .find((entry) => entry.textContent?.includes(label))
  ?.querySelector<HTMLButtonElement>(".expert-card__summon");

selected = null;
await act(async () => {
  card.click();
  await flush();
});
ok(selectedValue() === null, "expert card click opens details without summoning");
ok(document.querySelector("#expert-modal-title")?.textContent === "Backend Only Expert", "expert card click opens its details modal");
ok(document.querySelector(".expert-modal--detail") !== null, "expert details use the marketplace detail sheet layout");
ok(document.querySelector(".expert-modal__scroll") !== null, "expert detail header and content share one scroll region");
ok(document.querySelector(".expert-modal__usage")?.textContent?.includes("0次使用") === true, "expert details render persisted usage metadata");
ok(document.querySelector(".expert-modal__body")?.textContent?.includes("试试这样问我") === true, "expert details use the reference prompt heading");
ok(document.querySelector(".expert-identity-panel") === null, "expert details omit the redundant identity summary");
let promptButtons = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-prompt-list button"));
ok(promptButtons.length === 3, "expert details render three preset questions");
ok(document.querySelector(".expert-modal__footer-actions") === null, "official expert details hide edit, copy, and delete actions");
await act(async () => {
  promptButtons[0]?.click();
  await flush();
});
ok(selectedValue()?.id === "backend-only", "expert preset question summons the selected expert");
ok(selectedValue()?.prompt === "Use the backend expert prompt.", "expert preset question carries the clicked prompt");
const selectedWithMeta = selectedValue() as (ExpertSelection & { avatarKey?: string; avatarSrc?: string; tone?: string }) | null;
ok(selectedWithMeta?.avatarKey === "architect", "expert prompt selection carries backend avatar key");
ok(typeof selectedWithMeta?.avatarSrc === "string" && selectedWithMeta.avatarSrc.includes("architect"), "expert prompt selection carries resolved avatar source");
ok(selectedWithMeta?.tone === "stone", "expert prompt selection carries expert tone");
await act(async () => {
  document.querySelector<HTMLButtonElement>(".expert-modal__close")?.click();
  await flush();
});

const catalogSummonButton = summonButtonFor("Backend Only Expert");
if (!catalogSummonButton) throw new Error("missing catalog summon button");
selected = null;
await act(async () => {
  catalogSummonButton.click();
  await flush();
});
ok(selectedValue()?.id === "backend-only", "catalog summon button directly summons the selected expert");
ok(document.querySelector("#expert-modal-title") === null, "catalog summon button does not open the details modal");

const catalogModeButton = (label: string) => Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-mode-switch button"))
  .find((button) => button.textContent?.trim() === label);
const catalogTeamMode = catalogModeButton("专家团") ?? catalogModeButton("Expert team");
if (!catalogTeamMode) throw new Error("missing catalog team mode");
await act(async () => {
  catalogTeamMode.click();
  await flush();
});
await waitFor("catalog team card", () => document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Agent Team") === true);
const backendTeamCategory = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-category"))
  .find((button) => button.textContent?.trim() === "代码交付");
ok(Boolean(backendTeamCategory), "expert-team categories include backend-provided category values");
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-category"))
    .some((button) => button.textContent?.trim() === "Backend"),
  "expert-team mode category navigation includes categories found only on individual experts",
);
await act(async () => {
  backendTeamCategory?.click();
  await flush();
});
ok(
  document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Agent Team") === true &&
    document.querySelector(".expert-section--catalog")?.textContent?.includes("Backend Data Team") === false,
  "expert teams are classified by the backend category field",
);
const catalogTeamCard = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-card"))
  .find((button) => button.textContent?.includes("Backend Agent Team"));
if (!catalogTeamCard) throw new Error("missing catalog team card");
selected = null;
await act(async () => {
  catalogTeamCard.click();
  await flush();
});
ok(selectedValue() === null, "expert-team card click opens details without summoning");
ok(document.querySelector("#expert-modal-title")?.textContent === "Backend Agent Team", "expert-team card opens its details modal");
ok(document.querySelector(".expert-member-list")?.textContent?.includes("Worker Agent") === true, "expert-team details render members");
ok(document.querySelector(".expert-member__lead")?.textContent?.includes("主理人") === true, "expert-team details mark the lead member");
ok(document.querySelector(".expert-member")?.textContent?.includes("Plan the work.") === false, "expert-team member rows keep the compact reference hierarchy");
ok(document.querySelector(".expert-summon-button")?.textContent?.includes("Backend Agent Team") === true, "expert-team call to action names the selected team");
promptButtons = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-prompt-list button"));
ok(promptButtons.length === 3, "expert-team details render three preset questions");
ok(document.querySelector(".expert-modal__footer-actions") === null, "official expert-team details hide edit, copy, and delete actions");
await act(async () => {
  promptButtons[1]?.click();
  await flush();
});
const selectedTeamWithMeta = selectedValue() as (ExpertSelection & { avatarKey?: string; avatarSrc?: string; tone?: string; members?: unknown[] }) | null;
ok(selectedTeamWithMeta?.id === "backend-agent-team", "expert-team preset question summons the team");
ok(selectedTeamWithMeta?.prompt === "Review the agent team risks.", "expert-team preset question carries the clicked prompt");
ok(selectedTeamWithMeta?.avatarKey === "office-automation-team", "expert-team prompt selection carries team avatar key");
ok(Array.isArray(selectedTeamWithMeta?.members) && selectedTeamWithMeta.members.length === 2, "expert-team prompt selection carries member metadata");
const selectedTeamMembers = (selectedTeamWithMeta?.members ?? []) as Array<{ name?: string; avatarKey?: string; avatarSrc?: string }>;
const workerMember = selectedTeamMembers.find((member) => member.name === "Worker Agent");
ok(workerMember?.avatarKey === "research", "expert-team prompt selection resolves subagent avatar key");
ok(typeof workerMember?.avatarSrc === "string" && workerMember.avatarSrc.includes("research"), "expert-team prompt selection resolves subagent avatar source");
await act(async () => {
  document.querySelector<HTMLButtonElement>(".expert-modal__close")?.click();
  await flush();
});

const myButton = document.querySelector<HTMLButtonElement>(".expert-my-button");
if (!myButton) throw new Error("missing my experts button");
await act(async () => {
  myButton.click();
  await flush();
});
ok(document.querySelectorAll(".expert-manage-card").length === 0, "new user My Experts has no preset official cards");

const createButton = document.querySelector<HTMLButtonElement>(".expert-create-button");
if (!createButton) throw new Error("missing create button");
await act(async () => {
  createButton.click();
  await flush();
});
const draftModeSelect = document.querySelector<HTMLSelectElement>(".expert-modal--form select");
if (!draftModeSelect) throw new Error("missing draft mode select");
await act(async () => {
  draftModeSelect.value = "team";
  draftModeSelect.dispatchEvent(new Event("change", { bubbles: true }));
  await flush();
});
ok(document.querySelector("#expert-draft-title")?.textContent?.trim() === "创建专家团", "create form title follows expert-team mode");
ok(document.querySelectorAll(".expert-draft-section").length === 2, "create form groups basic information and session settings into visible sections");
ok(Boolean(document.querySelector(".expert-draft-agents__empty")), "empty expert team explains how to add its first member");
const categoryInput = Array.from(document.querySelectorAll<HTMLLabelElement>(".expert-modal--form label"))
  .find((label) => label.querySelector(":scope > span")?.textContent?.trim() === "分类")
  ?.querySelector<HTMLInputElement>("input");
if (!categoryInput) throw new Error("missing expert category input");
ok(categoryInput.getAttribute("list") === "expert-category-suggestions", "create form offers preset category suggestions");
await act(async () => {
  fireEvent.input(categoryInput, { target: { value: "商业研究" } });
  await flush();
});
const tagEditor = document.querySelector<HTMLElement>(".expert-tag-editor");
const tagInput = tagEditor?.querySelector<HTMLInputElement>("input");
if (!tagEditor || !tagInput) throw new Error("missing expert tag editor");
await act(async () => {
  fireEvent.input(tagInput, { target: { value: "调研" } });
  await flush();
});
const addTagButton = tagEditor.querySelector<HTMLButtonElement>(".expert-tag-editor__add");
if (!addTagButton) throw new Error("missing add tag button");
await act(async () => {
  addTagButton.click();
  await flush();
});
const refreshedTagInput = tagEditor.querySelector<HTMLInputElement>("input");
if (!refreshedTagInput) throw new Error("missing refreshed expert tag editor");
await act(async () => {
  fireEvent.input(refreshedTagInput, { target: { value: "洞察" } });
  await flush();
});
const readyAddTagButton = tagEditor.querySelector<HTMLButtonElement>(".expert-tag-editor__add");
if (!readyAddTagButton) throw new Error("missing ready add tag button");
await act(async () => {
  readyAddTagButton.click();
  await flush();
});
const finalTagInput = tagEditor.querySelector<HTMLInputElement>("input");
if (!finalTagInput) throw new Error("missing final expert tag input");
await act(async () => {
  fireEvent.input(finalTagInput, { target: { value: "报告" } });
  await flush();
});
const finalAddTagButton = tagEditor.querySelector<HTMLButtonElement>(".expert-tag-editor__add");
if (!finalAddTagButton) throw new Error("missing final add tag button");
await act(async () => {
  finalAddTagButton.click();
  await flush();
});
ok(
  Array.from(tagEditor.querySelectorAll(".expert-tag-editor__tag")).map((tag) => tag.textContent?.trim()).join("|").includes("调研") &&
    Array.from(tagEditor.querySelectorAll(".expert-tag-editor__tag")).map((tag) => tag.textContent?.trim()).join("|").includes("洞察") &&
    Array.from(tagEditor.querySelectorAll(".expert-tag-editor__tag")).map((tag) => tag.textContent?.trim()).join("|").includes("报告"),
  "create form adds multiple tags one at a time without comma parsing",
);
const removeFirstTagButton = tagEditor.querySelector<HTMLButtonElement>(".expert-tag-editor__tag button");
await act(async () => {
  removeFirstTagButton?.click();
  await flush();
});
ok(tagEditor.querySelectorAll(".expert-tag-editor__tag").length === 2, "create form removes an added tag independently");
const replacementTagInput = tagEditor.querySelector<HTMLInputElement>("input");
if (!replacementTagInput) throw new Error("missing replacement expert tag input");
await act(async () => {
  fireEvent.input(replacementTagInput, { target: { value: "复盘" } });
  await flush();
});
const replacementAddTagButton = tagEditor.querySelector<HTMLButtonElement>(".expert-tag-editor__add");
if (!replacementAddTagButton) throw new Error("missing replacement add tag button");
await act(async () => {
  replacementAddTagButton.click();
  await flush();
});
ok(tagEditor.querySelectorAll(".expert-tag-editor__tag").length === 3, "create form restores the required three custom tags");
const addTeamMemberButton = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-draft-agents__head button"))
  .find((button) => button.textContent?.includes("添加") || button.textContent?.includes("Add"));
if (!addTeamMemberButton) throw new Error("missing add team member button");
await act(async () => {
  addTeamMemberButton.click();
  addTeamMemberButton.click();
  await flush();
});
const createdTeamMemberRows = Array.from(document.querySelectorAll<HTMLElement>(".expert-draft-agent"));
ok(createdTeamMemberRows.every((row) => row.querySelectorAll(".expert-draft-agent__details label").length === 4), "expert-team members render readable labeled detail cards");
const createdTeamMemberSelects = createdTeamMemberRows.map((row) => row.querySelector<HTMLSelectElement>("select"));
if (createdTeamMemberSelects.some((select) => !select)) throw new Error("missing team member selectors");
await act(async () => {
  createdTeamMemberSelects[0]!.value = "backend-only";
  createdTeamMemberSelects[0]!.dispatchEvent(new Event("change", { bubbles: true }));
  createdTeamMemberSelects[1]!.value = "worker-agent";
  createdTeamMemberSelects[1]!.dispatchEvent(new Event("change", { bubbles: true }));
  await flush();
});
const draftPromptInputs = Array.from(document.querySelectorAll<HTMLTextAreaElement>(".expert-draft-prompts textarea"));
ok(draftPromptInputs.length === 0, "create form does not render preset-question fields");
ok(document.querySelector(".expert-draft-prompts") === null, "create form hides the complete preset-question section");
const draftSaveButton = document.querySelector<HTMLButtonElement>(".expert-modal--form .expert-summon-button");
ok(draftSaveButton?.disabled === false, "create form allows saving without preset questions");
ok(draftSaveButton?.textContent?.trim() === "保存专家团", "create form save action follows expert-team mode");
await act(async () => {
  draftSaveButton?.click();
  draftSaveButton?.click();
  await flush();
});
await waitFor("created expert card", () => document.querySelectorAll(".expert-manage-card").length > 0);
ok(createCalls === 1, "create form serializes a double submit into one backend CreateExpert call");
ok(listCalls >= 2, "create refreshes experts from backend");
ok(createdInputValue()?.mode === "team", "create form can create expert teams");
ok(createdInputValue()?.category === "商业研究", "create form saves a custom expert-team category");
ok(createdInputValue()?.role === "商业研究", "custom expert-team category is used as the role");
ok(createdInputValue()?.tags?.join("|") === "洞察|报告|复盘", "create form saves all individually added tags");
const createdAvatarKey = createdInputValue()?.avatarKey;
ok(createdAvatarKey === undefined, "create form allows the backend to apply its default avatar");
const createdMembers = (createdInputValue()?.members as Record<string, unknown>[] | undefined) ?? [];
ok(
  createdMembers.some((member) => member.expertId === "backend-only") &&
    createdMembers.some((member) => member.expertId === "worker-agent"),
  "create form saves selected backend expert IDs in submission order",
);
const createdPrompts = createdInputValue()?.prompts;
ok(Array.isArray(createdPrompts) && createdPrompts.length === 3, "create form generates the required default preset questions");
ok(createdInputValue()?.source === "user", "create form marks a custom expert team as user-owned");

const manageCards = Array.from(document.querySelectorAll<HTMLElement>(".expert-manage-card"));
const createdCard = manageCards[0];
if (!createdCard) throw new Error("missing created expert card");
await act(async () => {
  createdCard.querySelector<HTMLButtonElement>(".expert-manage-card__body")?.click();
  await flush();
});
const detailTags = Array.from(document.querySelectorAll<HTMLElement>(".expert-tags--modal span")).map((tag) => tag.textContent?.trim());
ok(detailTags.join("|") === "洞察|报告|复盘", "created expert details display every saved tag");
await act(async () => {
  document.querySelector<HTMLButtonElement>(".expert-modal__close")?.click();
  await flush();
});
const actionButtons = Array.from(createdCard.querySelectorAll<HTMLButtonElement>(".expert-manage-card__actions button"));
const editButton = actionButtons.find((button) => button.textContent?.includes("编辑") || button.textContent?.includes("Edit"));
if (!editButton) throw new Error("missing edit button");
ok(actionButtons.some((button) => button.textContent?.includes("复制") || button.textContent?.includes("Copy")), "user expert team keeps the copy action");
ok(actionButtons.some((button) => button.dataset.action === "delete"), "user expert team keeps the delete action");
await act(async () => {
  editButton.click();
  await flush();
});
const draftInputValues = () => Array.from(document.querySelectorAll<HTMLInputElement>(".expert-draft-agent input"))
  .map((input) => input.value);
await waitFor("editable user team agents", () =>
  draftInputValues().includes("Backend Only Expert") &&
  draftInputValues().includes("Worker Agent")
);
await act(async () => {
  document.querySelector<HTMLButtonElement>(".expert-modal--form .expert-summon-button")?.click();
  await flush();
});
await waitFor("updated expert call", () => updateCalls === 1);
ok(updateCalls === 1, "edit form updates a user expert team once");

const updatedCard = document.querySelector<HTMLElement>(".expert-manage-card");
if (!updatedCard) throw new Error("missing updated expert card");
const deleteButton = Array.from(updatedCard.querySelectorAll<HTMLButtonElement>(".expert-manage-card__actions button"))
  .find((button) => button.dataset.action === "delete");
ok(Boolean(deleteButton), "manage card exposes a delete action");
let deleteNativeConfirmCalls = 0;
const originalDeleteConfirm = window.confirm;
window.confirm = () => {
  deleteNativeConfirmCalls += 1;
  return true;
};
if (deleteButton) {
  await act(async () => {
    deleteButton.click();
    await flush();
  });
  ok(deleteCalls === 0, "first delete click only opens themed confirmation");
  ok(deleteNativeConfirmCalls === 0, "delete action does not open the native browser confirm dialog");
  const cancelDeleteButton = updatedCard.querySelector<HTMLButtonElement>('[data-action="delete-cancel"]');
  ok(Boolean(cancelDeleteButton), "delete action renders a themed inline cancellation option");
  const confirmDeleteButton = updatedCard.querySelector<HTMLButtonElement>('[data-action="delete"]');
  if (!confirmDeleteButton) throw new Error("missing delete confirm button");
  await act(async () => {
    confirmDeleteButton.click();
    await flush();
  });
  await waitFor("deleted expert call", () => deleteCalls === 1);
}
window.confirm = originalDeleteConfirm;
ok(deleteCalls === 1, "delete action calls backend DeleteExpert once");
ok(document.querySelectorAll(".expert-manage-card").length === 0, "deleting the only user expert team restores an empty My Experts page");

await act(async () => {
  root.render(
    <LocaleProvider>
      <ExpertDirectory authenticated={false} onSummon={(selection) => {
        selected = selection;
      }} />
    </LocaleProvider>,
  );
  await flush();
});
ok(document.querySelector(".expert-my-button") === null, "signed-out users do not see the My Experts entry");
ok(document.querySelector(".expert-page__header--mine") === null, "signing out while viewing My Experts returns to the catalog");

await act(async () => {
  root.unmount();
  await flush();
});

rootEl.innerHTML = "";
const libraryRoot = createRoot(rootEl);
const libraryExperts: ExpertInfo[] = [{
  id: "official-library-expert",
  source: "official",
  mode: "expert",
  name: "Official Library Expert",
  role: "Official",
  description: "Official favorite and recent expert.",
  categories: ["Official"],
  tags: ["official"],
  prompts: ["Official question one.", "Official question two.", "Official question three."],
  enabled: true,
}, {
  id: "official-library-team",
  source: "official",
  mode: "team",
  name: "Official Library Team",
  role: "Official",
  description: "Official recent team.",
  categories: ["Official"],
  tags: ["official"],
  members: [],
  prompts: ["Team question one.", "Team question two.", "Team question three."],
  enabled: true,
}];
const libraryUserExperts: UserExpertInfo[] = [{
  id: "user-library-expert",
  source: "user",
  mode: "expert",
  name: "User Library Expert",
  description: "Created by the user.",
  category: "Expert Only Category",
  tags: ["Team Category"],
  systemPrompt: "Act as the user expert.",
  prompts: ["User question one.", "User question two.", "User question three."],
  displayPrompt: "User question one.",
  enabled: true,
  createdAt: 1,
  updatedAt: 1,
}, {
  id: "user-library-team",
  source: "user",
  mode: "team",
  name: "User Library Team",
  description: "Created by the user.",
  category: "Team Category",
  tags: ["custom"],
  members: [],
  systemPrompt: "Act as the user team.",
  prompts: ["User team question one.", "User team question two.", "User team question three."],
  displayPrompt: "User team question one.",
  enabled: true,
  createdAt: 2,
  updatedAt: 2,
}];
const libraryState = {
  favoriteIds: ["official-library-expert", "official-library-team"],
  recentIds: ["official-library-expert"],
};
await act(async () => {
  libraryRoot.render(
    <LocaleProvider>
      <ExpertDirectory
        authenticated
        onSummon={() => undefined}
        backendExperts={libraryExperts}
        userExperts={libraryUserExperts}
        expertLibraryState={libraryState}
        onToggleFavorite={async () => libraryState}
        onDeleteUserExpert={async () => undefined}
      />
    </LocaleProvider>,
  );
  await flush();
});
await act(async () => {
  document.querySelector<HTMLButtonElement>(".expert-my-button")?.click();
  await flush();
});
const libraryCards = Array.from(document.querySelectorAll<HTMLElement>(".expert-manage-card"));
ok(libraryCards.length === 4, "My Experts joins user-created assets with favorited and recent system assets");
const officialLibraryCard = libraryCards.find((entry) => entry.textContent?.includes("Official Library Expert"));
if (!officialLibraryCard) throw new Error("missing favorited system expert");
const officialTeamCard = libraryCards.find((entry) => entry.textContent?.includes("Official Library Team"));
ok(Boolean(officialTeamCard), "My Experts resolves a recent system expert team to its full catalog data");
const officialActionsText = officialLibraryCard.querySelector(".expert-manage-card__actions")?.textContent ?? "";
ok(
  !(officialActionsText.includes("编辑") || officialActionsText.includes("Edit")) &&
    !(officialActionsText.includes("删除") || officialActionsText.includes("Delete")),
  "system favorite cards remain read-only",
);
const userLibraryCard = libraryCards.find((entry) => entry.textContent?.includes("User Library Expert"));
if (!userLibraryCard) throw new Error("missing user library expert");
const userActionsText = userLibraryCard.querySelector(".expert-manage-card__actions")?.textContent ?? "";
ok(
  (userActionsText.includes("编辑") || userActionsText.includes("Edit")) &&
  (userActionsText.includes("复制") || userActionsText.includes("Copy")) &&
  (userActionsText.includes("删除") || userActionsText.includes("Delete")),
  "user My Experts card keeps edit, copy, and delete actions",
);
const createdFilter = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-mode-switch--library button"))
  .find((button) => button.textContent?.trim() === "已创建" || button.textContent?.trim() === "Created");
if (!createdFilter) throw new Error("missing created filter");
const favoriteFilter = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-mode-switch--library button"))[2];
if (!favoriteFilter) throw new Error("missing favorite filter");
await act(async () => {
  favoriteFilter.click();
  await flush();
});
const favoriteCards = Array.from(document.querySelectorAll<HTMLElement>(".expert-manage-card"));
ok(
  favoriteCards.length === 2 &&
    favoriteCards.some((entry) => entry.textContent?.includes("Official Library Expert")) &&
    favoriteCards.some((entry) => entry.textContent?.includes("Official Library Team")),
  "Favorite filter displays full system expert and expert-team data resolved from the current user's favorite IDs",
);
await act(async () => {
  createdFilter.click();
  await flush();
});
ok(document.querySelectorAll(".expert-manage-card").length === 2, "Created filter contains only the user's expert and expert team");
const myCategoryButtons = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-category-row .expert-category"));
const myCategoryLabels = myCategoryButtons.map((button) => button.textContent?.trim());
ok(myCategoryLabels.includes("Team Category"), "My Experts categories include the user-created expert team's category field");
ok(!myCategoryLabels.includes("Expert Only Category"), "My Experts categories exclude categories found only on individual experts");
ok(
  !["技术工程", "办公自动化", "数据分析", "报告生成"].some((category) => myCategoryLabels.includes(category)),
  "My Experts categories do not include hard-coded category values",
);
const teamCategoryButton = myCategoryButtons.find((button) => button.textContent?.trim() === "Team Category");
if (!teamCategoryButton) throw new Error("missing dynamic expert-team category");
await act(async () => {
  teamCategoryButton.click();
  await flush();
});
const categoryFilteredCards = Array.from(document.querySelectorAll<HTMLElement>(".expert-manage-card"));
ok(
  categoryFilteredCards.length === 1 && categoryFilteredCards[0]?.textContent?.includes("User Library Team") === true,
  "My Experts category filter matches the category field instead of tags",
);
await act(async () => {
  libraryRoot.unmount();
  await flush();
});

rootEl.innerHTML = "";
const sortRoot = createRoot(rootEl);
const sortableExperts: ExpertInfo[] = [
  {
    id: "most-used",
    source: "official",
    mode: "expert",
    name: "Most Used Expert",
    role: "Sort",
    description: "Used most often.",
    categories: ["Sort"],
    tags: ["sort"],
    prompts: ["One.", "Two.", "Three."],
    enabled: true,
    createdAt: 100,
  },
  {
    id: "newest",
    source: "official",
    mode: "expert",
    name: "Newest Expert",
    role: "Sort",
    description: "Created most recently.",
    categories: ["Sort"],
    tags: ["sort"],
    prompts: ["One.", "Two.", "Three."],
    enabled: true,
    createdAt: 300,
  },
  {
    id: "middle",
    source: "official",
    mode: "expert",
    name: "Middle Expert",
    role: "Sort",
    description: "Falls between the others.",
    categories: ["Sort"],
    tags: ["sort"],
    prompts: ["One.", "Two.", "Three."],
    enabled: true,
    createdAt: 200,
  },
];
await act(async () => {
  sortRoot.render(
    <LocaleProvider>
      <ExpertDirectory
        authenticated
        onSummon={() => undefined}
        backendExperts={sortableExperts}
        expertLibraryState={{
          favoriteIds: [],
          recentIds: [],
          usageCounts: { "most-used": 12, middle: 4, newest: 1 },
        }}
      />
    </LocaleProvider>,
  );
  await flush();
});
const catalogCardNames = () => Array.from(document.querySelectorAll<HTMLElement>(".expert-section--catalog .expert-card__title strong"))
  .map((node) => node.textContent?.trim());
ok(catalogCardNames()[0] === "Most Used Expert", "hot sort uses persisted expert invocation counts");
const hotSortButton = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-sort-switch button"))
  .find((button) => button.textContent?.trim() === "最热");
const latestSortButton = Array.from(document.querySelectorAll<HTMLButtonElement>(".expert-sort-switch button"))
  .find((button) => button.textContent?.trim() === "最新");
ok(hotSortButton?.getAttribute("aria-pressed") === "true", "hot sort is selected by default");
if (!latestSortButton) throw new Error("missing latest sort button");
await act(async () => {
  latestSortButton.click();
  await flush();
});
ok(catalogCardNames()[0] === "Newest Expert", "latest sort uses expert creation time");
ok(latestSortButton.getAttribute("aria-pressed") === "true", "latest sort exposes its selected state");
await act(async () => {
  sortRoot.unmount();
  await flush();
});

rootEl.innerHTML = "";
const provenanceRoot = createRoot(rootEl);
let catalogSaveCalls = 0;
let userStoreSaveCalls = 0;
let catalogDeleteCalls = 0;
let userStoreDeleteCalls = 0;
const legacyCatalogUser: ExpertInfo = {
  id: "legacy-catalog-user",
  source: "user",
  mode: "expert",
  name: "Legacy Catalog User",
  role: "Custom",
  description: "A user-created expert stored in the legacy catalog.",
  categories: ["Custom"],
  tags: ["custom"],
  prompts: ["Legacy question one.", "Legacy question two.", "Legacy question three."],
  enabled: true,
};
const sceneCollisionUser: ExpertInfo = {
  ...legacyCatalogUser,
  id: "architect",
  name: "User Architect Collision",
};
const unknownSourceExpert: ExpertInfo = {
  ...legacyCatalogUser,
  id: "unknown-source-expert",
  source: "legacy",
  name: "Unknown Source Expert",
};
await act(async () => {
  provenanceRoot.render(
    <LocaleProvider>
      <ExpertDirectory
        authenticated
        onSummon={() => undefined}
        backendExperts={[legacyCatalogUser, sceneCollisionUser, unknownSourceExpert]}
        expertLibraryState={{ favoriteIds: [], recentIds: [] }}
        onSaveUserExpert={async () => {
          userStoreSaveCalls += 1;
          throw new Error("legacy catalog expert must not use the user store");
        }}
        onSaveCatalogExpert={async (_id, input) => {
          catalogSaveCalls += 1;
          return input;
        }}
        onDeleteUserExpert={async () => {
          userStoreDeleteCalls += 1;
        }}
        onDeleteCatalogExpert={async () => {
          catalogDeleteCalls += 1;
        }}
      />
    </LocaleProvider>,
  );
  await flush();
});
ok(document.querySelectorAll(".expert-card").length === 0, "legacy user experts stay out of the official catalog");
ok(document.querySelectorAll(".expert-scene-person").length === 0, "user and unknown sources stay out of official recommendation scenes");
await act(async () => {
  document.querySelector<HTMLButtonElement>(".expert-my-button")?.click();
  await flush();
});
const legacyCard = Array.from(document.querySelectorAll<HTMLElement>(".expert-manage-card"))
  .find((entry) => entry.textContent?.includes("Legacy Catalog User"));
if (!legacyCard) throw new Error("missing legacy catalog user expert");
const legacyEditButton = Array.from(legacyCard.querySelectorAll<HTMLButtonElement>(".expert-manage-card__actions button"))
  .find((button) => button.textContent?.includes("编辑") || button.textContent?.includes("Edit"));
if (!legacyEditButton) throw new Error("missing legacy catalog edit action");
await act(async () => {
  legacyEditButton.click();
  await flush();
});
await act(async () => {
  document.querySelector<HTMLButtonElement>(".expert-modal--form .expert-summon-button")?.click();
  await flush();
});
ok(catalogSaveCalls === 1 && userStoreSaveCalls === 0, "legacy catalog user edit stays in the catalog store");
const legacyDeleteButton = legacyCard.querySelector<HTMLButtonElement>('[data-action="delete"]');
if (!legacyDeleteButton) throw new Error("missing legacy catalog delete action");
await act(async () => {
  legacyDeleteButton.click();
  await flush();
});
await act(async () => {
  legacyCard.querySelector<HTMLButtonElement>('[data-action="delete"]')?.click();
  await flush();
});
ok(catalogDeleteCalls === 1 && userStoreDeleteCalls === 0, "legacy catalog user delete stays in the catalog store");
await act(async () => {
  provenanceRoot.unmount();
  await flush();
});

dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
