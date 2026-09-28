// Run: tsx src/__tests__/agent-workspace-switcher.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { AgentWorkspace, type AgentWorkspaceExpertMember } from "../components/AgentWorkspace";
import { Transcript } from "../components/Transcript";
import { ToolCard } from "../components/ToolCard";
import { LocaleProvider } from "../lib/i18n";
import { historyMessagesToItems, initialState, reducer } from "../lib/useController";
import type { HistoryMessage } from "../lib/types";
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

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
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
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.Node = dom.window.Node;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", { configurable: true, value: () => {} });
  return dom;
}

function task(id: string, description: string, status: ToolItem["status"] = "done"): ToolItem {
  return {
    kind: "tool",
    id,
    name: "task",
    args: JSON.stringify({ description, prompt: `${description} prompt` }),
    readOnly: false,
    status,
    output: `${description} final answer`,
  };
}

function parallelTasks(id: string): ToolItem {
  return {
    kind: "tool",
    id,
    name: "parallel_tasks",
    args: JSON.stringify({
      tasks: [
        { description: "Worker Agent", prompt: "Worker Agent prompt" },
        { description: "Review Agent", prompt: "Review Agent prompt" },
      ],
    }),
    readOnly: false,
    status: "running",
  };
}

function child(id: string, parentId: string, output: string): ToolItem {
  return {
    kind: "tool",
    id,
    parentId,
    name: "bash",
    args: JSON.stringify({ command: `printf ${JSON.stringify(output)}` }),
    readOnly: false,
    status: "done",
    output,
  };
}

function childAssistant(id: string, parentId: string, text: string): Item {
  return {
    kind: "assistant",
    id,
    parentId,
    text,
    reasoning: "",
    streaming: false,
  };
}

console.log("\nagent workspace switcher");

const dom = installDom();
const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);

const items = [
  task("agent-worker", "Worker Agent"),
  task("agent-reviewer", "Review Agent"),
];
const subcalls = new Map<string, ToolItem[]>([
  ["agent-worker", [child("worker-shell", "agent-worker", "worker evidence")]],
  ["agent-reviewer", [child("review-shell", "agent-reviewer", "review evidence")]],
]);

await act(async () => {
  root.render(
    <LocaleProvider>
      <AgentWorkspace items={items} subcalls={subcalls} />
    </LocaleProvider>,
  );
  await flush();
});

ok(document.querySelector(".agent-workspace") !== null, "delegate group renders an agent workspace");
ok(document.body.textContent?.includes("主专家") === true, "agent workspace has a lead expert tab");
const leadExpertTab = document.querySelector<HTMLButtonElement>('[data-agent-workspace-tab-id="overview"]');
ok(leadExpertTab?.textContent?.includes("主专家") === true, "expert workspace labels the main conversation as the lead expert");
ok(document.body.textContent?.includes("Overview") === false, "expert workspace no longer exposes the overview label");
ok(document.body.textContent?.includes("Worker Agent") === true, "agent workspace has first subagent tab");
ok(document.body.textContent?.includes("Review Agent") === true, "agent workspace has second subagent tab");
const sessionBody = document.querySelector<HTMLElement>(".agent-workspace__session-body");
const sessionTabs = document.querySelector<HTMLElement>(".agent-workspace__tabs");
ok(sessionBody !== null, "agent workspace renders a large session body");
const overviewView = document.querySelector<HTMLElement>(".agent-workspace__view");
ok(overviewView?.dataset.view === "overview", "agent workspace renders overview inside a stable switch view");
ok(overviewView?.dataset.agentWorkspaceActive === "overview", "agent workspace marks the overview as the active view");
ok(
  Boolean(sessionBody && sessionTabs && (sessionBody.compareDocumentPosition(sessionTabs) & Node.DOCUMENT_POSITION_FOLLOWING)),
  "agent workspace keeps the session switcher after the large body",
);

{
  const fallbackHost = document.createElement("div");
  document.body.appendChild(fallbackHost);
  const fallbackRoot = createRoot(fallbackHost);
  const unnamedTask: ToolItem = {
    kind: "tool",
    id: "unnamed-expert-task",
    name: "task",
    args: JSON.stringify({ prompt: "Identify audit anomalies" }),
    readOnly: false,
    status: "running",
  };
  await act(async () => {
    fallbackRoot.render(
      <LocaleProvider>
        <AgentWorkspace items={[unnamedTask]} subcalls={new Map()} overviewContent={<div>Lead conversation</div>} />
      </LocaleProvider>,
    );
    await flush();
  });
  const assignmentRole = fallbackHost.querySelector<HTMLElement>(".agent-workspace__assignment-role");
  ok(assignmentRole?.textContent?.includes("子专家") === true, "unmatched delegation uses the generic sub-expert label");
  ok(assignmentRole?.textContent?.endsWith("task") === false, "unmatched delegation never exposes the task tool name as an expert");
  const visibleLabels = Array.from(fallbackHost.querySelectorAll<HTMLElement>(".agent-workspace__label, .agent-workspace__assignment-title"))
    .map((node) => node.textContent?.trim().toLocaleLowerCase());
  ok(visibleLabels.includes("task") === false, "unmatched delegation never exposes the task tool name in workspace tabs or titles");
  await act(async () => {
    fallbackRoot.unmount();
  });
  fallbackHost.remove();
}

{
  const explicitMemberHost = document.createElement("div");
  document.body.appendChild(explicitMemberHost);
  const explicitMemberRoot = createRoot(explicitMemberHost);
  const configuredTeam = {
    name: "Delivery Team",
    members: [
      { kind: "lead", name: "Lead Expert" },
      { kind: "sub", name: "Code Review Expert", avatarSrc: "file:///avatars/review.png" },
      { kind: "sub", name: "Test Strategy Expert", avatarSrc: "file:///avatars/test.png" },
    ],
  } as const;
  const delegatedTask: ToolItem = {
    kind: "tool",
    id: "continue-file-task",
    name: "task",
    args: JSON.stringify({ member_name: "Code Review Expert", description: "Continue generating files", prompt: "Continue." }),
    readOnly: false,
    status: "running",
  };
  const rejectedTask: ToolItem = {
    kind: "tool",
    id: "unknown-member-task",
    name: "task",
    args: JSON.stringify({ member_name: "File Generator", description: "Continue generating files", prompt: "Continue." }),
    readOnly: false,
    status: "error",
  };
  await act(async () => {
    explicitMemberRoot.render(
      <LocaleProvider>
        <AgentWorkspace items={[delegatedTask, rejectedTask]} subcalls={new Map()} expertTeam={configuredTeam} />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(
    explicitMemberHost.querySelector('[data-agent-workspace-member="Code Review Expert"]') !== null,
    "explicit member_name binds the conversation to the configured sub-expert",
  );
  ok(
    Array.from(explicitMemberHost.querySelectorAll<HTMLElement>(".agent-workspace__label")).every((node) => node.textContent?.trim() !== "Continue generating files"),
    "free-form task descriptions never become expert tabs when member_name is present",
  );
  ok(
    explicitMemberHost.textContent?.includes("File Generator") === false,
    "an unconfigured member_name never appears as an expert conversation",
  );
  ok(
    explicitMemberHost.textContent?.includes("主专家") === true,
    "the main expert remains visible only as the overview conversation",
  );
  await act(async () => {
    explicitMemberRoot.unmount();
  });
  explicitMemberHost.remove();
}

{
  const malformedTeamHost = document.createElement("div");
  document.body.appendChild(malformedTeamHost);
  const malformedTeamRoot = createRoot(malformedTeamHost);
  const historicalTask = task("historical-stranger", "Continue generating files", "running");
  await act(async () => {
    malformedTeamRoot.render(
      <LocaleProvider>
        <AgentWorkspace
          items={[historicalTask]}
          subcalls={new Map()}
          expertTeam={{ name: "Malformed Team", members: [{ kind: "lead", name: "Lead Expert" }] }}
        />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(malformedTeamHost.querySelector(".agent-workspace") === null, "a team without valid sub-experts renders no historical stranger conversations");
  await act(async () => {
    malformedTeamRoot.unmount();
  });
  malformedTeamHost.remove();
}

{
  const restoredLeadHost = document.createElement("div");
  document.body.appendChild(restoredLeadHost);
  const restoredLeadRoot = createRoot(restoredLeadHost);
  const historicalTask = {
    ...task("historical-unmatched", "Legacy specialist assignment", "done"),
    args: JSON.stringify({
      member_name: "Legacy Specialist",
      description: "Legacy specialist assignment",
      prompt: "Legacy specialist assignment prompt",
    }),
  };
  await act(async () => {
    restoredLeadRoot.render(
      <LocaleProvider>
        <AgentWorkspace
          items={[historicalTask]}
          subcalls={new Map()}
          expertTeam={{
            name: "Current Team",
            members: [
              { kind: "lead", name: "Lead Expert" },
              { kind: "sub", name: "Current Specialist" },
            ],
          }}
          overviewContent={<div data-restored-lead-history>restored lead conversation</div>}
        />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(
    restoredLeadHost.textContent?.includes("restored lead conversation") === true,
    "restored lead conversation remains visible when every legacy delegation is unmatched",
  );
  ok(
    restoredLeadHost.querySelector("[data-agent-workspace-member]") === null,
    "unmatched legacy delegations do not create stranger expert tabs",
  );
  await act(async () => {
    restoredLeadRoot.unmount();
  });
  restoredLeadHost.remove();
}

const workerTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
  .find((button) => button.textContent?.includes("Worker Agent"));
await act(async () => {
  workerTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
  await flush();
});

const focusedBody = document.querySelector<HTMLElement>(".agent-workspace__body");
const focusedView = document.querySelector<HTMLElement>(".agent-workspace__view");
ok(focusedView?.dataset.view === "focused", "agent workspace marks focused subagent panes as a focused view");
ok(focusedView?.dataset.agentWorkspaceActive === "agent-worker", "focused subagent view tracks the active child session");
ok(focusedBody?.textContent?.includes("worker evidence") === true, "focused subagent pane shows selected subagent output");
ok(focusedBody?.textContent?.includes("review evidence") === false, "focused subagent pane hides other subagent output");
const returnButton = document.querySelector<HTMLButtonElement>(".agent-workspace__return");
ok(returnButton !== undefined, "focused subagent session offers a return-to-main action");

const overviewTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
  .find((button) => button.textContent?.includes("主专家"));
await act(async () => {
  overviewTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
  await flush();
});

ok(document.querySelector<HTMLElement>(".agent-workspace__view")?.dataset.agentWorkspaceActive === "overview", "overview tab restores the full team view");
ok(document.querySelector<HTMLElement>(".agent-workspace__footer")?.textContent?.includes("Review Agent") === true, "overview keeps the complete expert switcher available");

await act(async () => {
  root.unmount();
});
dom.window.close();

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const completedTeam = {
    name: "Completion Team",
    members: [
      { kind: "lead", name: "Lead Expert" },
      { kind: "sub", name: "Completion Expert", role: "Handoff" },
    ],
  } as const;

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={[task("completion-live-id", "Completion Expert", "done")]}
          subcalls={new Map()}
          expertTeam={completedTeam}
          overviewContent={<div>Lead completion summary</div>}
          selectionScopeKey="completed-task-a"
        />
      </LocaleProvider>,
    );
    await flush();
  });
  const completionTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Completion Expert"));
  await act(async () => {
    completionTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
  });
  ok(
    document.querySelector<HTMLElement>(".agent-workspace__view")?.dataset.agentWorkspaceActive === "completion-live-id",
    "completed expert task can be selected before history replacement",
  );

  const restoredTask = {
    ...task("completion-restored-id", "Completion Expert", "done"),
    output: "restored completion answer",
    dataArchived: true,
  } satisfies ToolItem;
  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={[restoredTask]}
          subcalls={new Map()}
          expertTeam={completedTeam}
          overviewContent={<div>Lead completion summary</div>}
          selectionScopeKey="completed-task-a"
        />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(
    document.querySelector<HTMLElement>(".agent-workspace__view")?.dataset.agentWorkspaceActive === "completion-restored-id",
    "completed expert selection follows the same expert across archived item replacement",
  );
  ok(
    document.querySelector<HTMLElement>(".agent-workspace__body")?.textContent?.includes("restored completion answer") === true,
    "completed expert switch updates to the restored result without jumping to the lead expert",
  );

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={[task("next-task-id", "Completion Expert", "done")]}
          subcalls={new Map()}
          expertTeam={completedTeam}
          overviewContent={<div>Next task lead summary</div>}
          selectionScopeKey="completed-task-b"
        />
      </LocaleProvider>,
    );
    await flush();
  });
  ok(
    document.querySelector<HTMLElement>(".agent-workspace__view")?.dataset.agentWorkspaceActive === "overview",
    "switching away from a completed task does not leak its selected child expert into the next task",
  );

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
  const expertTeam = {
    name: "Market Team",
    avatarSrc: "file:///avatars/team.png",
    members: [
      { kind: "lead", name: "Lead Analyst", role: "Lead", task: "Coordinate market analysis.", output: "Summary", tone: "blue", avatarSrc: "file:///avatars/lead.png" },
      { kind: "sub", name: "Narrative Expert", role: "Narrative", task: "Write the narrative.", output: "Narrative", tone: "green", avatarSrc: "file:///avatars/narrative.png" },
      { kind: "sub", name: "Data Expert", role: "Data", task: "Check data quality.", output: "Data", tone: "amber", avatarSrc: "file:///avatars/data.png" },
    ],
  } as const;
  const items: Item[] = [
    { kind: "user", id: "user-market", text: "Analyze SPCX", createdAt: Date.now() },
    { kind: "assistant", id: "lead-market", text: "I will coordinate the analysis", reasoning: "", streaming: false },
    parallelTasks("market-parallel"),
    { ...task("market-parallel/narrative", "Write the narrative", "running"), parentId: "market-parallel", taskTreeId: "market-tree", nodeId: "narrative" },
    { ...task("market-parallel/data", "Check data quality", "running"), parentId: "market-parallel" },
    childAssistant("market-child-answer", "market-parallel/narrative", "child narrative answer"),
    child("market-child-shell", "market-parallel/narrative", "child shell evidence"),
  ];
  const taskTrees = {
    trees: {
      "market-tree": {
        treeId: "market-tree",
        title: "Market analysis",
        status: "running",
        rootIds: ["narrative"],
        nodes: {
          narrative: {
            nodeId: "narrative",
            title: "Write the narrative",
            status: "running",
            dependsOn: [],
            childIds: [],
            output: "child narrative answer",
            outputDeltas: [],
            goal: "Write the narrative",
            expertName: "Narrative Expert",
            toolIds: ["market-parallel/narrative"],
            updatedAt: 1,
          },
        },
        currentNodeIds: ["narrative"],
        updatedAt: 1,
      },
    },
    activeTreeId: "market-tree",
    seq: 1,
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <Transcript
          items={items}
          onPrompt={() => {}}
          questionNavigator={false}
          activeExpert={{ id: "market-team", name: "Market Team", mode: "team", avatarSrc: expertTeam.avatarSrc, members: expertTeam.members }}
          taskTrees={taskTrees}
        />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.querySelectorAll(".agent-workspace").length === 0, "legacy parallel branches do not create an expert workspace");
  ok(document.querySelectorAll(".agent-workspace__tabs").length === 0, "legacy parallel branches create no expert tabs");
  ok(document.body.textContent?.includes("Analyze SPCX") === true, "main transcript keeps the user message");
  ok(document.body.textContent?.includes("I will coordinate the analysis") === true, "main transcript keeps the lead expert reply");
  ok(document.querySelector(".parallel-task-list") !== null, "legacy live parallel branches render in the aggregate card");
  ok(document.querySelectorAll(".parallel-task-list__row").length === 2, "parallel aggregate shows both branch states");
  ok(document.body.textContent?.includes("child narrative answer") === false, "main transcript hides child replies and task-tree output");
  ok(document.body.textContent?.includes("child shell evidence") === false, "main transcript hides child tools");
  ok(document.querySelector(".expert-team-flow-card") === null, "parallel aggregate omits the detailed team flow card");

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
  const expertTeam = {
    name: "Metric Team",
    members: [
      { kind: "sub", name: "Metric Owner", role: "Metrics", task: "Collect metrics.", output: "Metric notes", tone: "amber" },
      { kind: "sub", name: "Quality Owner", role: "Quality", task: "Check quality.", output: "Quality notes", tone: "amber" },
    ],
  } as const;
  const items = [
    task("metric-task", "Collect metrics.", "running"),
    task("quality-task", "Check quality.", "running"),
  ];

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={items} subcalls={new Map()} expertTeam={expertTeam} />
      </LocaleProvider>,
    );
    await flush();
  });

  const tabImages = Array.from(document.querySelectorAll<HTMLImageElement>(".agent-workspace__tab img")).map((img) => img.getAttribute("src") ?? "");
  ok(tabImages.length === new Set(tabImages).size, "expert workspace does not render duplicate tab avatar images");
  ok(
    Boolean(document.querySelector<HTMLImageElement>("[data-agent-workspace-member=\"Quality Owner\"] img")),
    "expert workspace keeps fallback avatars in the shared illustrated style",
  );

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
  const expertTeam = {
    name: "Reporting Team",
    avatarSrc: "file:///avatars/team.png",
    members: [
      { kind: "lead", name: "Lead Analyst", role: "Lead", task: "Coordinate the work.", output: "Plan", tone: "blue", avatarSrc: "file:///avatars/lead.png" },
      { kind: "sub", name: "Report Narrative Expert", role: "Narrative", task: "Write report narrative.", output: "Narrative", tone: "green", avatarSrc: "file:///avatars/report.png" },
      { kind: "sub", name: "Metric Expert", role: "Metrics", task: "Check metrics.", output: "Metrics", tone: "amber", avatarSrc: "file:///avatars/metrics.png" },
    ],
  } as const;
  const items = [
    task("narrative-first", "Write report narrative", "done"),
    task("metric-task", "Check metrics", "done"),
    task("narrative-second", "Write report narrative follow-up", "running"),
  ];
  const memberByItemId = new Map<string, AgentWorkspaceExpertMember>([
    ["narrative-first", expertTeam.members[1]],
    ["metric-task", expertTeam.members[2]],
    ["narrative-second", expertTeam.members[1]],
  ]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={items} subcalls={new Map()} expertTeam={expertTeam} memberByItemId={memberByItemId} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(
    document.querySelectorAll("[data-agent-workspace-member=\"Report Narrative Expert\"]").length === 1,
    "expert workspace deduplicates repeated member tabs",
  );

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
  const items = [task("agent-planner", "Draft the implementation plan")];
  const planner: AgentWorkspaceExpertMember = { kind: "sub", name: "Planning Expert", role: "Planning" };
  const subcalls = new Map<string, Item[]>([
    ["agent-planner", [
      childAssistant("agent-planner-answer", "agent-planner", "private child final answer"),
      child("agent-planner-shell", "agent-planner", "private child tool output"),
    ]],
  ]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={items}
          subcalls={subcalls}
          memberByItemId={new Map([["agent-planner", planner]])}
          overviewContent={<div data-testid="main-session-output">normal lead expert reply</div>}
        />
      </LocaleProvider>,
    );
    await flush();
  });

  const leadView = document.querySelector<HTMLElement>(".agent-workspace__body");
  ok(document.querySelector<HTMLElement>("[data-testid=\"main-session-output\"]")?.textContent === "normal lead expert reply", "lead expert tab keeps the normal main conversation");
  ok(leadView?.textContent?.includes("Planning Expert") === true, "lead expert tab shows the assignment target");
  ok(leadView?.textContent?.includes("Draft the implementation plan") === true, "lead expert tab shows the outbound assignment text");
  ok(leadView?.textContent?.includes("private child final answer") === false, "lead expert tab hides the child final answer");
  ok(leadView?.textContent?.includes("private child tool output") === false, "lead expert tab hides child tool output");
  ok(leadView?.textContent?.includes("Draft the implementation plan final answer") === false, "lead expert tab hides task execution output");
  const plannerTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Planning Expert"));
  await act(async () => {
    plannerTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
  });
  const focusedChildView = document.querySelector<HTMLElement>(".agent-workspace__body");
  ok(focusedChildView?.textContent?.includes("private child final answer") === true, "focused child tab still renders its own reply");
  ok(focusedChildView?.textContent?.includes("private child tool output") === true, "focused child tab still renders its own tool output");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  let state = reducer(initialState, { type: "event", e: { kind: "turn_started" } } as never);
  state = reducer(state, { type: "event", e: { kind: "tool_dispatch", tool: { id: "parallel-2", name: "parallel_tasks", args: "{}", readOnly: false } } } as never);
  state = reducer(state, { type: "event", e: { kind: "tool_dispatch", tool: { id: "parallel-2/sub-1", parentId: "parallel-2", name: "task", args: JSON.stringify({ description: "Research Agent" }), readOnly: true } } } as never);
  state = reducer(state, { type: "event", e: { kind: "text", text: "subagent streaming answer", parentId: "parallel-2/sub-1" } } as never);
  state = reducer(state, { type: "event", e: { kind: "message", text: "subagent final answer", parentId: "parallel-2/sub-1" } } as never);

  const mainAssistant = state.items.find((item) => item.kind === "assistant" && !(item as { parentId?: string }).parentId && item.text.includes("subagent"));
  ok(mainAssistant === undefined, "subagent chat text does not leak into the main assistant session");

  ok(state.items.every((item) => item.kind !== "assistant"), "unidentified parallel child chat is discarded instead of stored in main items");
  ok(state.parallelItemsByCall["parallel-2"]?.length === 1, "parallel branch state remains attached to the outer aggregate call");
}

{
  const messages: HistoryMessage[] = [
    {
      role: "assistant",
      content: "",
      toolCalls: [
        {
          id: "task-history",
          name: "task",
          arguments: JSON.stringify({ description: "Research Agent" }),
          subagentRef: "sa_history",
          subagentMessages: [
            { role: "user", content: "subagent saved prompt" },
            { role: "assistant", content: "subagent saved answer", reasoning: "saved reasoning" },
          ],
        },
      ],
    },
    { role: "tool", content: "task summary", toolCallId: "task-history", toolName: "task" },
  ];
  const { items } = historyMessagesToItems(messages, "h");
  const taskItems = items.filter((item): item is ToolItem => item.kind === "tool" && item.id === "task-history");
  const childItems = items.filter((item) => (item as { parentId?: string }).parentId === "task-history");

  ok(childItems.length === 0, "parent history projection does not restore embedded child messages");

  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={taskItems} subcalls={new Map([["task-history", childItems]])} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.querySelector<HTMLElement>(".agent-workspace__body")?.textContent?.includes("subagent saved answer") === false, "history overview keeps restored subagent chat hidden");
  const researchTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Research Agent"));
  await act(async () => {
    researchTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
  });
  ok(document.querySelector<HTMLElement>(".agent-workspace__body")?.textContent?.includes("subagent saved answer") === false, "parent history alone cannot reveal embedded child history");

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
  (dom.window as unknown as { go: unknown }).go = {
    main: {
      App: {
        SubagentHistoryPageForTab: async () => ({
          messages: [
            { role: "user", content: "restored child prompt" },
            { role: "assistant", content: "restored child answer" },
          ],
          before: 0,
          hasMore: false,
          generation: "history-generation",
        }),
      },
    },
  };
  const restoredTask: ToolItem = {
    kind: "tool",
    id: "restored-task",
    name: "task",
    args: JSON.stringify({ member_name: "Research Expert", description: "Research" }),
    readOnly: false,
    status: "done",
    dataArchived: true,
    subagentRef: "sa_restored",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={[restoredTask]}
          subcalls={new Map()}
          tabId="restored-tab"
          expertTeam={{ members: [{ kind: "sub", name: "Research Expert", avatarSrc: "file:///avatars/research.png" }] }}
        />
      </LocaleProvider>,
    );
    await flush();
  });

  const restoredTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Research Expert"));
  await act(async () => {
    restoredTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();
  });
  ok(
    document.querySelector<HTMLElement>(".agent-workspace__body")?.textContent?.includes("restored child answer") === true,
    "switching back to a completed expert task lazily restores the child expert message",
  );

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
  let restoredParallelRequests = 0;
  (dom.window as unknown as { go: unknown }).go = {
    main: {
      App: {
        ToolResultForTab: async () => {
          restoredParallelRequests += 1;
          if (restoredParallelRequests === 1) return null;
          return {
          args: JSON.stringify({
            tasks: [
              { member_name: "Research Expert", description: "Collect evidence", prompt: "private research prompt" },
              { member_name: "Review Expert", description: "Review evidence", prompt: "private review prompt" },
            ],
          }),
          output: [
            "Completed 2 parallel tasks:",
            "── task-1 ──",
            "restored research answer",
            "── task-2 ──",
            "restored review answer",
          ].join("\n"),
          };
        },
      },
    },
  };
  const restoredParallel: ToolItem = {
    kind: "tool",
    id: "parallel-restored",
    name: "parallel_tasks",
    args: "",
    readOnly: false,
    status: "done",
    dataArchived: true,
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <Transcript
          items={[
            { kind: "user", id: "restored-user", text: "Restore the completed team" },
            restoredParallel,
            { kind: "assistant", id: "restored-lead", text: "Lead expert summary", reasoning: "", streaming: false },
          ]}
          tabId="restored-parallel-tab"
          viewKey="restored-parallel-view"
          onPrompt={() => {}}
          questionNavigator={false}
          activeExpert={{
            id: "restored-team",
            name: "Restored Team",
            mode: "team",
            avatarSrc: "file:///avatars/team.png",
            members: [
              { kind: "lead", name: "Lead Expert", avatarSrc: "file:///avatars/lead.png" },
              { kind: "sub", name: "Research Expert", avatarSrc: "file:///avatars/research.png" },
              { kind: "sub", name: "Review Expert", avatarSrc: "file:///avatars/review.png" },
            ],
          }}
        />
      </LocaleProvider>,
    );
    await flush();
    await flush();
    await flush();
  });
  await act(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 300));
    await flush();
  });

  ok(restoredParallelRequests === 0, "completed parallel history is not reinterpreted as expert child history");
  ok(document.querySelector(".agent-workspace") === null, "switching back keeps completed parallel as one outer card");
  ok(document.querySelector('[data-agent-workspace-tab-id="overview"]') === null, "completed parallel creates no lead expert tab");
  ok(document.querySelector('[data-agent-workspace-member="Research Expert"]') === null, "completed parallel creates no child expert avatar");
  ok(document.body.textContent?.includes("restored research answer") === false, "completed parallel does not restore a synthetic child conversation");

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
  const expertTeam = {
    name: "财务专家团",
    members: [
      { kind: "sub", name: "会计专家", role: "数据采集", task: "采集并统一经营数据口径。", output: "经营数据底稿", tone: "amber", avatarSrc: "file:///avatars/accounting.png" },
      { kind: "sub", name: "税务专家", role: "政策匹配与税务筹划", task: "匹配税务政策并输出筹划建议。", output: "税务建议", tone: "green", avatarSrc: "file:///avatars/tax.png" },
    ],
  } as const;
  const items = [
    task("accounting-tax-task", "会计专家-数据采集", "running"),
  ];
  const memberByItemId = new Map([["accounting-tax-task", expertTeam.members[1]]]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={items}
          subcalls={new Map()}
          expertTeam={expertTeam}
          memberByItemId={memberByItemId}
        />
      </LocaleProvider>,
    );
    await flush();
  });

  const accountingTab = document.querySelector<HTMLElement>("[data-agent-workspace-member=\"会计专家\"]");
  ok(
    accountingTab === null,
    "a lead-member task is not rendered as a sub-expert conversation",
  );
  ok(
    document.querySelector("[data-agent-workspace-member=\"税务专家\"]") === null,
    "accounting task is not mislabeled as the tax expert from task-tree mapping",
  );

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
  const expertTeam = {
    name: "财务专家团",
    avatarSrc: "file:///avatars/team.png",
    members: [
      { kind: "sub", name: "会计专家", role: "全维度数据采集", task: "汇聚跨部门经营数据并统一账务口径。", output: "经营数据底稿", tone: "amber", avatarSrc: "file:///avatars/accounting.png" },
      { kind: "sub", name: "经营决策专家", role: "诊断报告与方案推演", task: "输出季度经营诊断与决策方案。", output: "决策底稿", tone: "blue", avatarSrc: "file:///avatars/decision.png" },
    ],
  } as const;
  const items = [
    task("accounting-session", "会计专家-全维度数据采集", "running"),
    task("decision-session", "经营决策专家-诊断报告与方案推演", "running"),
  ];
  const subcalls = new Map<string, Item[]>([
    ["accounting-session", [childAssistant("accounting-answer", "accounting-session", "会计专家采集页面")]],
    ["decision-session", [childAssistant("decision-answer", "decision-session", "经营决策专家聊天页面")]],
  ]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={items}
          subcalls={subcalls}
          expertTeam={{ ...expertTeam, members: expertTeam.members.map((member) => ({ ...member, kind: "sub" as const })) }}
        />
      </LocaleProvider>,
    );
    await flush();
  });

  const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"));
  const accountingTab = tabs.find((button) => button.textContent?.includes("会计专家"));
  const decisionTab = tabs.find((button) => button.textContent?.includes("经营决策专家"));
  Object.defineProperty(accountingTab, "getBoundingClientRect", {
    configurable: true,
    value: () => ({ left: 80, right: 215, top: 300, bottom: 340, width: 135, height: 40, x: 80, y: 300, toJSON: () => {} }),
  });
  Object.defineProperty(decisionTab, "getBoundingClientRect", {
    configurable: true,
    value: () => ({ left: 224, right: 410, top: 300, bottom: 340, width: 186, height: 40, x: 224, y: 300, toJSON: () => {} }),
  });

  await act(async () => {
    accountingTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true, clientX: 260, clientY: 320 }));
    await flush();
  });

  ok(document.querySelector<HTMLElement>(".agent-workspace__view")?.dataset.agentWorkspaceActive === "decision-session", "coordinate hit testing chooses the expert tab under the pointer, not a stale overlapping event target");
  ok(document.querySelector<HTMLElement>(".agent-workspace__body")?.textContent?.includes("经营决策专家聊天页面") === true, "clicking the decision expert avatar by coordinate shows the decision expert chat page");
  ok(document.querySelector<HTMLElement>(".agent-workspace__body")?.textContent?.includes("会计专家采集页面") === false, "coordinate-selected decision expert page does not show the accounting expert chat page");

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
  const expertTeam = {
    name: "财务专家团",
    members: [
      { kind: "sub", name: "会计专家", role: "全维度数据采集", task: "汇聚跨部门经营数据并统一账务口径。", output: "经营数据底稿", tone: "amber", avatarSrc: "file:///avatars/accounting.png" },
      { kind: "sub", name: "经营决策专家", role: "诊断报告与方案推演", task: "输出季度经营诊断与决策方案。", output: "决策底稿", tone: "blue", avatarSrc: "file:///avatars/decision.png" },
    ],
  } as const;
  const items = [
    task("accounting-task", "会计专家：全维度数据采集与口径统一 — 诊断报告与方案推演", "running"),
  ];

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={items} subcalls={new Map()} expertTeam={expertTeam} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(
    document.querySelector("[data-agent-workspace-member=\"会计专家\"]") === null,
    "expert workspace never renders the configured lead member as a sub-expert tab",
  );
  ok(
    document.querySelector("[data-agent-workspace-member=\"经营决策专家\"]") === null,
    "finance lead task is not mislabeled as the decision expert",
  );

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
  const expertTeam = {
    name: "财务专家团",
    members: [
      { kind: "sub", name: "审计专家", role: "穿透核查", task: "排查发票合规风险。", output: "异常清单", tone: "violet" },
    ],
  } as const;
  const items = [task("audit-task", "排查发票合规风险", "running")];
  const memberByItemId = new Map([["audit-task", expertTeam.members[0]]]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={items}
          subcalls={new Map()}
          expertTeam={expertTeam}
          memberByItemId={memberByItemId}
        />
      </LocaleProvider>,
    );
    await flush();
  });

  const auditAvatar = document.querySelector<HTMLElement>("[data-agent-workspace-member=\"审计专家\"]");
  ok(
    auditAvatar?.querySelector<HTMLImageElement>("img")?.getAttribute("src")?.includes("research") === true,
    "finance expert member without uploaded avatar uses a static PNG avatar",
  );
  ok(auditAvatar?.textContent?.trim() !== "审", "finance expert member avatar does not fall back to text initials");

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
  const expertTeam = {
    name: "Trading Expert Team",
    avatarSrc: "file:///avatars/team.png",
    members: [
      { kind: "lead", name: "Lead Strategist", role: "Leader", task: "Coordinate the work.", output: "Plan", tone: "blue", avatarSrc: "file:///avatars/lead.png" },
      { kind: "sub", name: "Fundamentals Analyst", role: "Finance", task: "Check revenue and profit.", output: "Report", tone: "amber", avatarSrc: "file:///avatars/fundamentals.png" },
      { kind: "sub", name: "Risk Analyst", role: "Risk", task: "Check downside risk.", output: "Risks", tone: "green", avatarSrc: "file:///avatars/risk.png" },
    ],
  };
  const items = [
    task("fundamentals-task", "Check revenue and profit."),
    task("risk-task", "Check downside risk."),
  ];
  const subcalls = new Map<string, Item[]>([
    ["fundamentals-task", [childAssistant("fundamentals-answer", "fundamentals-task", "fundamentals report")]],
    ["risk-task", [childAssistant("risk-answer", "risk-task", "risk report")]],
  ]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={items} subcalls={subcalls} expertTeam={expertTeam} />
      </LocaleProvider>,
    );
    await flush();
  });

  const expertOverviewTab = document.querySelector<HTMLButtonElement>(".agent-workspace__tab");
  ok(expertOverviewTab?.textContent?.includes("主专家") === true, "expert team main tab uses the lead expert label");
  ok(expertOverviewTab?.textContent?.includes("Lead Strategist") === false, "expert team overview tab does not duplicate the lead agent name");
  ok(document.querySelector<HTMLImageElement>(".agent-workspace__avatar--main img")?.getAttribute("src") === "file:///avatars/team.png", "expert team overview tab uses the team avatar image");
  ok(document.body.textContent?.includes("Fundamentals Analyst") === true, "subagent tab shows matched expert member name");
  ok(document.querySelector<HTMLImageElement>("[data-agent-workspace-member=\"Fundamentals Analyst\"] img")?.getAttribute("src") === "file:///avatars/fundamentals.png", "subagent tab uses the matched member avatar image");

  const fundamentalsTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Fundamentals Analyst"));
  await act(async () => {
    fundamentalsTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
  });

  const focusedBody = document.querySelector<HTMLElement>(".agent-workspace__body");
  ok(focusedBody?.textContent?.includes("Check revenue and profit.") === false, "focused member conversation omits the delegated task heading");
  ok(document.querySelector<HTMLElement>(".agent-workspace__footer")?.textContent?.includes("Fundamentals Analyst") === true, "focused member conversation keeps the member label in the bottom tab");
  ok(focusedBody?.textContent?.includes("fundamentals report") === true, "clicking a member avatar shows that subagent conversation");
  ok(focusedBody?.textContent?.includes("risk report") === false, "focused member conversation hides sibling subagent output");
  const expertReturnButton = document.querySelector<HTMLButtonElement>(".agent-workspace__return");
  ok(expertReturnButton !== undefined, "focused expert member session exposes a return-to-main action");
  ok(expertReturnButton?.parentElement?.classList.contains("agent-workspace__return-row") === true, "focused expert member session keeps the return action in a dedicated footer row");
  await act(async () => {
    expertReturnButton?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
  });
  ok(document.querySelector<HTMLElement>(".agent-workspace__view")?.dataset.view === "overview", "return action switches the expert team back to the main conversation");
  ok(document.querySelector<HTMLElement>(".agent-workspace__view")?.dataset.agentWorkspaceActive === "overview", "return action marks the main expert conversation as active");

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
  const expertTeam = {
    name: "Research Team",
    members: [
      { kind: "lead", name: "Lead Strategist", role: "Lead", task: "Coordinate the work.", output: "Plan", tone: "blue", avatarSrc: "file:///avatars/lead.png" },
      { kind: "sub", name: "Research Specialist", role: "Research", task: "Map constraints.", output: "Research notes", tone: "green", avatarSrc: "file:///avatars/research.png" },
    ],
  } as const;
  const items = [task("research-delegate", "Explore implementation context", "running")];
  const memberByItemId = new Map([["research-delegate", expertTeam.members[1]]]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace
          items={items}
          subcalls={new Map()}
          expertTeam={expertTeam}
          memberByItemId={memberByItemId}
        />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(
    document.querySelector<HTMLImageElement>("[data-agent-workspace-member=\"Research Specialist\"] img")?.getAttribute("src") === "file:///avatars/research.png",
    "expert workspace restores member avatar from task-tree expert assignment",
  );

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
  const expertTeam = {
    name: "Code Delivery Team",
    avatarSrc: "file:///avatars/code-delivery-team.png",
    members: [
      { kind: "lead", name: "Solution Architect", role: "Architecture", task: "Break down module boundaries and data flow.", output: "Architecture notes", tone: "blue", avatarSrc: "file:///avatars/architect.png" },
      { kind: "sub", name: "Code Review Expert", role: "Implementation quality", task: "Inspect key paths, exception branches, maintainability, and possible regressions introduced by changes.", output: "Risk list", tone: "violet", avatarSrc: "file:///avatars/code-review.png" },
      { kind: "sub", name: "Release Risk Expert", role: "Release control", task: "Evaluate configuration, compatibility, rollback, and launch observation metrics.", output: "Release checklist", tone: "stone", avatarSrc: "file:///avatars/release.png" },
    ],
  } as const;
  const items = [task("git-diff-review", "Check git diff and file changes", "running")];

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={items} subcalls={new Map()} expertTeam={expertTeam} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(
    document.querySelector<HTMLImageElement>("[data-agent-workspace-member=\"Code Review Expert\"] img")?.getAttribute("src") === "file:///avatars/code-review.png",
    "expert workspace matches terse review task text to the code-review member avatar",
  );
  ok(document.body.textContent?.includes("Code Review Expert") === true, "matched expert member name replaces the terse delegated task title");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  localStorage.setItem("kwu-display-mode", "compact");
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const items: Item[] = [
    parallelTasks("parallel-1"),
    { ...task("parallel-1/sub-1", "Worker Agent", "running"), output: "worker evidence" },
    { ...task("parallel-1/sub-2", "Review Agent"), output: "review evidence" },
  ];
  const [parallel, ...nested] = items as ToolItem[];

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={parallel} subcalls={nested} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.querySelector(".agent-workspace") === null, "parallel_tasks does not render an agent workspace");
  ok(document.querySelectorAll(".parallel-task-list__row").length === 2, "parallel_tasks renders aggregate branch rows");
  ok(document.querySelector<HTMLElement>(".tool__body")?.textContent?.includes("\"prompt\"") === false, "parallel task arguments hide prompt fields from users");
  ok(document.querySelector<HTMLElement>(".tool__body")?.textContent?.includes("Worker Agent prompt") === false, "parallel task arguments hide subagent prompt text from users");
  ok(document.querySelectorAll(".agent-workspace__avatar").length === 0, "parallel branches expose no expert avatars");
  ok(document.body.textContent?.includes("worker evidence") === false, "parallel aggregate hides branch internal output");
  ok(document.body.textContent?.includes("review evidence") === false, "parallel aggregate hides every branch internal output");
  ok(document.querySelector(".agent-workspace__return") === null, "parallel aggregate exposes no child-session navigation");

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
  const promptOnlyTask: ToolItem = {
    kind: "tool",
    id: "prompt-only-task",
    name: "task",
    args: JSON.stringify({ prompt: "internal prompt should not be visible" }),
    readOnly: false,
    status: "done",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={promptOnlyTask} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.body.textContent?.includes("internal prompt should not be visible") === false, "prompt-only subagent tool does not expose prompt text");
  ok(document.querySelector<HTMLElement>(".tool__body")?.textContent?.includes("\"prompt\"") === false, "prompt-only subagent tool does not expose prompt key");

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
  const items = [
    task("agent-renderer", "Renderer Agent"),
  ];
  const childText = "**subagent final answer**\n\n```json\n{\"ok\":true}\n```";
  const subcalls = new Map<string, Item[]>([
    ["agent-renderer", [childAssistant("agent-renderer-answer", "agent-renderer", childText)]],
  ]);

  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={items} subcalls={subcalls} />
      </LocaleProvider>,
    );
    await flush();
  });

  const rendererTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Renderer Agent"));
  await act(async () => {
    rendererTab?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
    await flush();
  });

  const focusedBody = document.querySelector<HTMLElement>(".agent-workspace__body");
  ok(focusedBody?.textContent?.includes("Renderer Agent") === false, "focused subagent session omits the delegated task heading");
  ok(document.querySelector<HTMLElement>(".agent-workspace__footer")?.textContent?.includes("Renderer Agent") === true, "focused subagent session keeps the delegated task label in the bottom tab");
  ok(focusedBody?.querySelector(".msg--assistant .md") !== null, "focused subagent session reuses the standard assistant markdown renderer");
  ok(focusedBody?.textContent?.includes("Renderer Agent prompt") === false, "focused subagent session does not expose the parent task prompt");

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
  const streamingTask: ToolItem = {
    kind: "tool",
    id: "streaming-prompt-task",
    name: "task",
    args: '{"description":"Worker Agent","prompt":"streaming internal prompt',
    readOnly: false,
    status: "running",
  };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToolCard item={streamingTask} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.body.textContent?.includes("streaming internal prompt") === false, "partial subagent arguments never expose a streaming prompt");

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
  const hidden = '<system-prompt source="expert" hidden="true">chat-only secret</system-prompt>\n\n';
  const transcriptItems: Item[] = [
    { kind: "user", id: "hidden-user", text: `${hidden}visible question` },
    { kind: "phase", id: "hidden-phase", text: `${hidden}visible phase` },
    { kind: "notice", id: "hidden-notice", level: "info", text: `${hidden}visible notice` },
    {
      kind: "assistant",
      id: "hidden-reasoning",
      text: "visible answer",
      reasoning: `${hidden}visible reasoning`,
      streaming: true,
    },
  ];

  await act(async () => {
    root.render(
      <LocaleProvider>
        <Transcript items={transcriptItems} onPrompt={() => {}} questionNavigator={false} />
      </LocaleProvider>,
    );
    await flush();
  });

  ok(document.body.textContent?.includes("chat-only secret") === false, "chat presentation boundaries hide expert prompts from every message surface");
  ok(document.body.textContent?.includes("visible phase") === true, "phase text remains visible after hidden prompt removal");
  ok(document.body.textContent?.includes("visible notice") === true, "notice text remains visible after hidden prompt removal");
  ok(document.body.textContent?.includes("visible reasoning") === true, "reasoning text remains visible after hidden prompt removal");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
