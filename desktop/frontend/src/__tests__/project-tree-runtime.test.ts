// Run: tsx src/__tests__/project-tree-runtime.test.ts

import {
  projectTreeFolderDisclosure,
  defaultExpandedProjectTreeKeys,
  activeSessionAncestorKeys,
  projectTreeTopicPresentationClass,
  projectTreeTopicOpenRequest,
  projectTreeShouldSuppressOpenForRename,
  projectTreeNodeHasCapability,
  splitProjectTreeTaskWorkspace,
} from "../components/ProjectTree";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { ProjectNode } from "../lib/types";

let passed = 0;
let failed = 0;
const testDir = dirname(fileURLToPath(import.meta.url));
const projectTreeSource = readFileSync(resolve(testDir, "../features/workspace/ui/ProjectTree.tsx"), "utf8");

function eq(a: unknown, b: unknown, label: string) {
  if (JSON.stringify(a) === JSON.stringify(b)) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

console.log("\nproject tree runtime sessions");

const defaultComposerIdentity = {
  modelRef: "",
  billingSource: "",
  expertKnown: true,
  expertId: "",
  expertName: "",
  collaborationMode: "normal",
  toolApprovalMode: "ask",
  tokenMode: "full",
  goal: "",
  goalStatus: "stopped",
};

const cloudProjectLocalSession: ProjectNode = {
  key: "cloud_session_local",
  kind: "session",
  label: "Cloud project local session",
  cloudProjectId: "project-cloud",
  root: "/repo",
  topicId: "topic-cloud-local",
  sessionPath: "/repo/cloud-local.jsonl",
  sessionTurns: 3,
};

const cloudProjectGlobalSession: ProjectNode = {
  key: "cloud_session_global",
  kind: "global_session",
  label: "Cloud project global session",
  cloudProjectId: "project-cloud",
  topicId: "topic-cloud-global",
  sessionPath: "/tmp/cloud-global.jsonl",
  sessionTurns: 4,
};

const superpowerTopic: ProjectNode = {
  key: "superpower-topic",
  kind: "global_topic",
  label: "Superpower research",
  topicId: "topic-superpower",
  children: [{
    key: "superpower-session",
    kind: "global_session",
    label: "Superpower research session",
    topicId: "topic-superpower",
    capability: {
      capabilityId: "crossborder-commerce",
      workspaceId: "workspace-1",
      taskId: "task-1",
      documentId: "document-1",
    },
  }],
};

eq(
  projectTreeNodeHasCapability(superpowerTopic, "crossborder-commerce"),
  true,
  "superpower capability is detected when it is stored on a nested runtime session",
);

eq(
  projectTreeNodeHasCapability(superpowerTopic, "another-capability"),
  false,
  "unrelated capability ids do not classify a topic as a superpower session",
);

const tree: ProjectNode[] = [
  {
    key: "global_folder",
    kind: "global_folder",
    label: "Global",
    children: [
      {
        key: "global_topic_topic-a",
        kind: "global_topic",
        label: "Topic A",
        topicId: "topic-a",
        children: [
          {
            key: "global_session_a",
            kind: "global_session",
            label: "Session A",
            topicId: "topic-a",
            sessionPath: "/tmp/a.jsonl",
            sessionTurns: 0,
            scene: "coding",
            sceneState: "draft",
          },
          {
            key: "global_session_b",
            kind: "global_session",
            label: "Session B",
            topicId: "topic-a",
            sessionPath: "/tmp/b.jsonl",
            sessionTurns: 2,
            scene: "design",
            sceneState: "locked",
          },
        ],
      },
      {
        key: "global_topic_topic-b",
        kind: "global_topic",
        label: "Topic B",
        topicId: "topic-b",
      },
    ],
  },
];

eq(
  defaultExpandedProjectTreeKeys(tree),
  [],
  "without an active tab, no folders default to expanded",
);

eq(
  defaultExpandedProjectTreeKeys(tree, "global", "", "topic-a", "/tmp/b.jsonl"),
  ["global_folder", "global_topic_topic-a"],
  "active session path expands only ancestor folders",
);

eq(
  activeSessionAncestorKeys(tree, "global", "", "topic-a", "/tmp/b.jsonl"),
  ["global_folder", "global_topic_topic-a"],
  "activeSessionAncestorKeys matches defaultExpandedProjectTreeKeys for active session",
);

eq(
  activeSessionAncestorKeys(tree, "global", "", "topic-b"),
  ["global_folder"],
  "active topic without runtime session rows expands only parent folders",
);

eq(
  projectTreeTopicOpenRequest(tree[0].children?.[0].children?.[1] as ProjectNode),
  {
    scope: "global",
    workspaceRoot: "",
    topicId: "topic-a",
    sessionPath: "/tmp/b.jsonl",
    sessionTurns: 2,
    scene: "design",
    sceneState: "locked",
    composerIdentity: defaultComposerIdentity,
  },
  "global runtime session row opens the concrete path with its exact turns and scene",
);

eq(
  projectTreeTopicOpenRequest({
    key: "topic_project",
    kind: "topic",
    label: "Project topic",
    root: "/repo",
    topicId: "topic-project",
    sessionPath: "/repo/latest.jsonl",
    sessionTurns: 3,
    scene: "coding",
    sceneState: "draft",
  }),
  {
    scope: "project",
    workspaceRoot: "/repo",
    topicId: "topic-project",
    sessionPath: "/repo/latest.jsonl",
    sessionTurns: 3,
    scene: "coding",
    sceneState: "draft",
    composerIdentity: defaultComposerIdentity,
  },
  "regular project topic forwards the concrete path, exact turns, and scene",
);

eq(
  projectTreeTopicOpenRequest({
    key: "global_topic_existing",
    kind: "global_topic",
    label: "Global existing",
    topicId: "topic-global-existing",
    sessionPath: "/tmp/global-latest.jsonl",
    sessionTurns: 0,
    scene: "office",
    sceneState: "draft",
  }),
  {
    scope: "global",
    workspaceRoot: "",
    topicId: "topic-global-existing",
    sessionPath: "/tmp/global-latest.jsonl",
    sessionTurns: 0,
    scene: "office",
    sceneState: "draft",
    composerIdentity: defaultComposerIdentity,
  },
  "regular global topic preserves an exact zero-turn target and fallback scene",
);

eq(
  projectTreeTopicOpenRequest({
    key: "session_project_zero",
    kind: "session",
    label: "Project blank runtime",
    root: "/repo",
    topicId: "topic-project",
    sessionPath: "/repo/blank.jsonl",
    sessionTurns: 0,
    scene: "design",
    sceneState: "draft",
  }),
  {
    scope: "project",
    workspaceRoot: "/repo",
    topicId: "topic-project",
    sessionPath: "/repo/blank.jsonl",
    sessionTurns: 0,
    scene: "design",
    sceneState: "draft",
    composerIdentity: defaultComposerIdentity,
  },
  "project runtime session row preserves an exact zero-turn target and scene",
);

eq(
  [
    projectTreeTopicPresentationClass(cloudProjectLocalSession),
    projectTreeTopicPresentationClass(cloudProjectGlobalSession),
  ],
  ["project-tree__topic--project", "project-tree__topic--project"],
  "local and global sessions under the same cloud project share project presentation styling",
);

eq(
  projectTreeTopicOpenRequest(cloudProjectLocalSession),
  {
    scope: "project",
    workspaceRoot: "/repo",
    topicId: "topic-cloud-local",
    sessionPath: "/repo/cloud-local.jsonl",
    sessionTurns: 3,
    composerIdentity: defaultComposerIdentity,
  },
  "cloud project local session preserves its project execution target and workspace root",
);

eq(
  projectTreeTopicOpenRequest(cloudProjectGlobalSession),
  {
    scope: "global",
    workspaceRoot: "",
    topicId: "topic-cloud-global",
    sessionPath: "/tmp/cloud-global.jsonl",
    sessionTurns: 4,
    composerIdentity: defaultComposerIdentity,
  },
  "cloud project session preserves its real global execution target",
);

eq(
  projectTreeTopicPresentationClass(tree[0].children?.[0].children?.[0] as ProjectNode),
  "project-tree__topic--global",
  "global sessions outside cloud projects retain global presentation styling",
);

eq(
  projectTreeTopicOpenRequest({
    key: "topic_blank",
    kind: "topic",
    label: "Blank project topic",
    root: "/repo",
    topicId: "topic-blank",
  }),
  { scope: "project", workspaceRoot: "/repo", topicId: "topic-blank", sessionPath: undefined, composerIdentity: defaultComposerIdentity },
  "genuinely blank project topic remains pathless and keeps blank-surface intent",
);

eq(
  projectTreeShouldSuppressOpenForRename(
    { rowKey: "topic-a", canRename: true },
    { rowKey: "topic-a", canRename: true },
  ),
  true,
  "second click on the same renameable topic suppresses open for inline rename",
);

eq(
  projectTreeShouldSuppressOpenForRename(
    { rowKey: "session-a", canRename: false },
    { rowKey: "session-a", canRename: false },
  ),
  false,
  "runtime session double-click still allows the session row to open",
);

eq(
  projectTreeShouldSuppressOpenForRename(
    { rowKey: "topic-a", canRename: true },
    { rowKey: "topic-b", canRename: true },
  ),
  false,
  "quickly clicking a different topic still opens the new target",
);

const taskWorkspaceSections = splitProjectTreeTaskWorkspace([
  tree[0],
  {
    key: "project_repo",
    kind: "project",
    label: "Repo",
    root: "/repo",
    children: [
      {
        key: "topic_project",
        kind: "topic",
        label: "Project topic",
        root: "/repo",
        topicId: "topic-project",
      },
    ],
  },
  {
    key: "cloud_project_project-cloud",
    kind: "cloud_project",
    label: "Shared cloud project",
    cloudProjectId: "project-cloud",
  },
]);

eq(
  {
    task: taskWorkspaceSections.taskFolder?.kind,
    workspaces: taskWorkspaceSections.workspaces.map((node) => ({ kind: node.kind, root: node.root, cloudProjectId: node.cloudProjectId })),
    others: taskWorkspaceSections.others.length,
  },
  { task: "global_folder", workspaces: [{ kind: "project", root: "/repo" }, { kind: "cloud_project", cloudProjectId: "project-cloud" }], others: 0 },
  "global folder stays in tasks while local and logical cloud projects render as workspaces",
);

eq(
  projectTreeFolderDisclosure(false, true),
  {
    canExpand: false,
    isOpen: false,
    ariaExpanded: undefined,
    iconStackClassName: "project-tree__icon-stack",
  },
  "empty project folders are not exposed as expandable disclosure rows",
);

eq(
  projectTreeFolderDisclosure(true, false),
  {
    canExpand: true,
    isOpen: false,
    ariaExpanded: false,
    iconStackClassName: "project-tree__icon-stack project-tree__icon-stack--expandable",
  },
  "collapsed project folders keep disclosure semantics when children exist",
);

eq(
  projectTreeFolderDisclosure(true, true),
  {
    canExpand: true,
    isOpen: true,
    ariaExpanded: true,
    iconStackClassName: "project-tree__icon-stack project-tree__icon-stack--expandable",
  },
  "expanded project folders can show the open-folder state only when children exist",
);

eq(
  /project-tree__topic-action--archive[\s\S]*?confirmAction\?\.topicId === topicId[\s\S]*?trashTopic\(topicId\)[\s\S]*?setConfirmAction\(\{ topicId, action: "trash" \}\)/.test(projectTreeSource),
  true,
  "compact topic archive button uses the same second-click confirmation as the topic menu",
);

eq(
  /PROJECT_COLOR_OPTIONS|SetProjectColor|color-separator/.test(projectTreeSource),
  false,
  "workspace context menu omits project color controls",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
