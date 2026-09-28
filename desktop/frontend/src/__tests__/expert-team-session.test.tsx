// Run: tsx src/__tests__/expert-team-session.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React, { act } from "react";
import { createRoot } from "react-dom/client";

import { ExpertTeamSessionView } from "../components/ExpertTeamSession";
import { LocaleProvider } from "../lib/i18n";
import type { ExpertTeamSession } from "../lib/types";

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

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.Node = dom.window.Node;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

let childToolRequest = "";
let memberFocused = false;
let returnToLeadAction: (() => void) | undefined;
const onMemberFocusChange = (focused: boolean, returnToLead?: () => void) => {
  memberFocused = focused;
  returnToLeadAction = returnToLead;
};
(dom.window as unknown as { go: unknown }).go = {
  main: {
    App: {
      SubagentHistoryPageForTab: async () => ({
        messages: [
          { role: "user", content: "delegated review" },
          {
            role: "assistant",
            content: "child answer",
            toolCalls: [{ id: "child_tool_1", name: "read_file", arguments: "", argumentsArchived: true }],
          },
          { role: "tool", toolCallId: "child_tool_1", toolName: "read_file", content: "", toolResultArchived: true },
        ],
        before: 0,
        hasMore: false,
        generation: "child-generation",
      }),
      SubagentToolResultForTab: async (tabID: string, ref: string, toolID: string) => {
        childToolRequest = `${tabID}|${ref}|${toolID}`;
        return { args: `{"path":"child.go"}`, output: "package child" };
      },
    },
  },
};

const team: ExpertTeamSession = {
  state: "team_active",
  id: "review-team",
  name: "Review Team",
  lead: { memberKey: "lead", name: "Architecture Lead", role: "coordination", avatarKey: "architect" },
  members: [
    { memberKey: "reviewer", expertId: "code-review", name: "Review Expert", role: "review", avatarKey: "https://assets.example.com/reviewer.png", subagentRef: "sa_owned", status: "completed" },
    { memberKey: "runner", expertId: "security-review", name: "Running Expert", role: "implementation", avatarKey: "https://assets.example.com/runner.png", subagentRef: "sa_running", status: "running" },
    { memberKey: "failed", name: "Failed Expert", role: "testing", subagentRef: "sa_failed", status: "failed" },
    { memberKey: "interrupted", name: "Interrupted Expert", role: "testing", subagentRef: "sa_interrupted", status: "interrupted" },
    { memberKey: "unknown", name: "Unknown Expert", role: "review", subagentRef: "sa_unknown", status: "queued" },
    { memberKey: "tester", name: "Test Expert", role: "test" },
  ],
};

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
await act(async () => {
  root.render(
    <LocaleProvider>
      <ExpertTeamSessionView
        team={team}
        tabId="parent-tab"
        childItemsByRef={{}}
        overviewContent={<div>parent transcript</div>}
        selectionScopeKey="parent-session"
        onMemberFocusChange={onMemberFocusChange}
      />
    </LocaleProvider>,
  );
  await flush();
});

ok(document.body.textContent?.includes("parent transcript") === true, "overview keeps the parent transcript visible");
const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"));
const reviewTab = tabs.find((button) => button.textContent?.includes("Review Expert"));
const testTab = tabs.find((button) => button.textContent?.includes("Test Expert"));
ok(tabs[0]?.textContent?.includes("Architecture Lead") === true, "overview uses the configured lead identity");
const reviewAvatar = reviewTab?.querySelector<HTMLImageElement>(".agent-workspace__avatar img")?.src ?? "";
const runningAvatar = tabs.find((button) => button.textContent?.includes("Running Expert"))?.querySelector<HTMLImageElement>(".agent-workspace__avatar img")?.src ?? "";
ok(reviewAvatar === "https://assets.example.com/reviewer.png" && runningAvatar === "https://assets.example.com/runner.png", "service avatar URLs win over local expert-id and name fallbacks");
ok(
  tabs.map((button) => button.textContent?.trim()).join("|") ===
    "Architecture Lead|Review Expert|Running Expert|Failed Expert|Interrupted Expert|Unknown Expert",
  "member bar preserves the authoritative backend order after the fixed lead",
);
ok(testTab === undefined, "member without a task session is hidden until a binding exists");
ok(reviewTab?.querySelector(".agent-workspace__status--done") !== null, "completed member uses the green terminal status class");
ok(tabs.find((button) => button.textContent?.includes("Running Expert"))?.querySelector(".agent-workspace__status--running") !== null, "running member uses the yellow status class");
ok(tabs.find((button) => button.textContent?.includes("Failed Expert"))?.querySelector(".agent-workspace__status--error") !== null, "failed member uses the red terminal status class");
ok(tabs.find((button) => button.textContent?.includes("Interrupted Expert"))?.querySelector(".agent-workspace__status--error") !== null, "interrupted member uses the red terminal status class");
ok(tabs.find((button) => button.textContent?.includes("Failed Expert"))?.querySelector(".agent-workspace__status")?.getAttribute("aria-label") === "错误", "failed member keeps the error label");
ok(tabs.find((button) => button.textContent?.includes("Interrupted Expert"))?.querySelector(".agent-workspace__status")?.getAttribute("aria-label") === "已中断", "interrupted member distinguishes user cancellation from failure");
ok(tabs.find((button) => button.textContent?.includes("Interrupted Expert"))?.querySelector(".agent-workspace__status")?.getAttribute("title") === "已中断", "interrupted member exposes the localized interruption title");
ok(tabs.find((button) => button.textContent?.includes("Unknown Expert"))?.querySelector(".agent-workspace__status") === null, "unknown member status does not render a dot");
const statusStyles = readFileSync(new URL("../styles/session.css", import.meta.url), "utf8");
const runningStatusRule = statusStyles.match(/\.agent-workspace__status--running\s*\{([^}]*)\}/)?.[1] ?? "";
ok(runningStatusRule.includes("background: var(--warn)"), "running status uses the warning color token");
ok(!runningStatusRule.includes("box-shadow"), "running status has no halo");
const memberFooterRule = statusStyles.match(/\.agent-workspace__footer\s*\{([^}]*)\}/)?.[1] ?? "";
const memberTabsRule = statusStyles.match(/\.agent-workspace__tabs\s*\{([^}]*)\}/)?.[1] ?? "";
const memberTabRule = statusStyles.match(/\.agent-workspace__tab\s*\{([^}]*)\}/)?.[1] ?? "";
const memberTabInteractiveRule = statusStyles.match(/\.agent-workspace__tab:hover,[\s\S]*?\.agent-workspace__tab:focus-visible\s*\{([^}]*)\}/)?.[1] ?? "";
const memberTabActiveRule = statusStyles.match(/\.agent-workspace__tab--active\s*\{([^}]*)\}/)?.[1] ?? "";
const memberTabActiveInteractiveRule = statusStyles.match(/\.agent-workspace__tab--active:hover,[\s\S]*?\.agent-workspace__tab--active:focus-visible\s*\{([^}]*)\}/)?.[1] ?? "";
ok(memberFooterRule.includes("position: sticky") && memberFooterRule.includes("bottom: 0"), "member bar remains pinned at the transcript footer");
ok(memberFooterRule.includes("background: transparent"), "member bar leaves transcript content visible through its surface");
ok(!memberFooterRule.includes("var(--chat-bg)") && !memberFooterRule.includes("linear-gradient"), "member bar does not add an opaque theme or gradient backdrop");
ok(memberTabsRule.includes("background: transparent") && memberTabsRule.includes("overflow-x: auto"), "member tab strip adds no nested surface and keeps horizontal access");
ok(memberTabRule.includes("background: color-mix(in srgb, var(--fg) 4%, var(--chat-bg))") && memberTabRule.includes("box-shadow: none"), "unselected member buttons stay distinct and flat on the shared surface");
ok(memberTabInteractiveRule.includes("color-mix(in srgb, var(--fg) 7%, var(--chat-bg))") && memberTabActiveRule.includes("background: color-mix(in srgb, var(--fg) 4%, var(--chat-bg))") && memberTabActiveRule.includes("border-color: var(--fg)") && memberTabActiveInteractiveRule.includes("color-mix(in srgb, var(--fg) 7%, var(--chat-bg))"), "selected and unselected member buttons share a neutral fill while retaining hover, focus, and active feedback");

await act(async () => {
  root.render(
    <LocaleProvider>
      <ExpertTeamSessionView
        team={{ ...team, members: null } as unknown as ExpertTeamSession}
        tabId="parent-tab"
        childItemsByRef={{}}
        overviewContent={<div>parent transcript without bound members</div>}
        selectionScopeKey="parent-session"
        onMemberFocusChange={onMemberFocusChange}
      />
    </LocaleProvider>,
  );
  await flush();
});
ok(document.body.textContent?.includes("parent transcript without bound members") === true, "null member payload keeps the lead transcript usable");

await act(async () => {
  root.render(
    <LocaleProvider>
      <ExpertTeamSessionView
        team={team}
        tabId="parent-tab"
        childItemsByRef={{}}
        overviewContent={<div>parent transcript</div>}
        selectionScopeKey="parent-session"
        onMemberFocusChange={onMemberFocusChange}
      />
    </LocaleProvider>,
  );
  await flush();
});

await act(async () => {
  Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Review Expert"))
    ?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
  await flush();
  await flush();
});
ok(document.body.textContent?.includes("child answer") === true, "member tab lazily restores its child session history");
ok(document.querySelector("textarea") === null, "child history view has no user composer");
ok(memberFocused, "member history focus is reported so the parent shell can lock its composer");
ok(typeof returnToLeadAction === "function", "member history exposes a return action for the stable composer shell");

await act(async () => {
  document.querySelector<HTMLButtonElement>(".tool__head")?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
  await flush();
});
ok(childToolRequest === "parent-tab|sa_owned|child_tool_1", "archived child tool data uses the owned child route");

await act(async () => {
  returnToLeadAction?.();
  await flush();
});
ok(!memberFocused, "the composer-shell return action restores parent focus");

await act(async () => {
  root.render(
    <LocaleProvider>
      <ExpertTeamSessionView
        team={{ ...team, state: "team_archived" }}
        tabId="parent-tab"
        childItemsByRef={{}}
        overviewContent={<div>parent transcript</div>}
        selectionScopeKey="parent-session"
        onMemberFocusChange={onMemberFocusChange}
      />
    </LocaleProvider>,
  );
  await flush();
});
ok(document.querySelector(".agent-workspace")?.getAttribute("data-team-state") === "team_archived", "archived team keeps its history switcher");

await act(async () => {
  root.render(
    <LocaleProvider>
      <ExpertTeamSessionView
        team={team}
        tabId="parent-tab"
        childItemsByRef={{
          sa_running: [{ kind: "assistant", id: "child:sa_running:live", text: "", reasoning: "Live investigation continues", streaming: true }],
        }}
        overviewContent={<div>parent transcript</div>}
        selectionScopeKey="parent-session"
        onMemberFocusChange={onMemberFocusChange}
      />
    </LocaleProvider>,
  );
  await flush();
});
for (let visit = 0; visit < 2; visit += 1) {
  await act(async () => {
    Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Running Expert"))
      ?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();
  });
  ok(document.querySelector(".reasoning__body")?.textContent?.includes("Live investigation continues") === true, `running member reasoning survives restored history on visit ${visit + 1}`);
  ok(document.body.textContent?.includes("delegated review") === true && document.body.textContent?.includes("child answer") === true, `running member retains saved conversation context on visit ${visit + 1}`);
  ok(document.querySelectorAll('[data-entrance="child:sa_running:live"]').length === 1, `running member renders one live reasoning message on visit ${visit + 1}`);
  await act(async () => {
    returnToLeadAction?.();
    await flush();
  });
}

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
