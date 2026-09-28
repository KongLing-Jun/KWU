import { readFileSync } from "node:fs";
import { projectGateway } from "../features/projects/data/projectGateway";
import type { CloudProjectListInput, NewCloudProjectSessionInput } from "../features/projects/model/projectTypes";
import type { AppBindings } from "../lib/bridge";

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

const calls: Array<{ method: string; payload: unknown }> = [];
const page = { projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null };
const project = {
  projectScopeKey: "opaque-a",
  project: { projectId: "p-1", name: "Alpha", description: null, status: "active" as const, version: 1, createdAt: "2026-08-05T04:00:00Z", updatedAt: "2026-08-05T04:00:00Z" },
  stale: false,
  cachedAt: null,
};
const instruction = {
  projectScopeKey: "opaque-a", projectId: "p-1", humanDescription: "Human", agentInstruction: "Agent",
  contentHash: "a".repeat(64), version: 3, createdAt: "2026-08-05T04:00:00Z", updatedAt: "2026-08-05T04:00:00Z",
};
const workItem = {
  workItemId: "w-1", projectId: "p-1", title: "Plan", description: null, status: "todo" as const,
  rank: 100, version: 1, createdAt: "2026-08-05T04:00:00Z", updatedAt: "2026-08-05T04:00:00Z",
  assigneeUserId: null, assigneeStatus: null, assigneeDisplayName: null, priority: "none" as const,
  startAt: null, dueAt: null, labels: [],
};
const assetDirectoryPage = {
  projectScopeKey: "opaque-a", projectId: "p-1", breadcrumbs: [], items: [], totalCount: 0,
  nextCursor: null, usageBytes: 12, quotaBytes: 1024, quotaPlan: "free",
};
const assetOperationRecordPage = { projectScopeKey: "opaque-a", projectId: "p-1", items: [], limit: 20 };

const appBindings: Partial<AppBindings> = {
        ListCloudProjects: async (input) => { calls.push({ method: "list", payload: input }); return page; },
        ListJoinedCloudProjects: async (input) => { calls.push({ method: "list-joined", payload: input }); return page; },
        ClaimCloudProjectOnboarding: async () => {
          calls.push({ method: "onboarding-claim", payload: {} });
          return { projectScopeKey: "opaque-a", project: project.project, guideVersion: 1, state: "active" as const, created: false };
        },
        GetCloudProject: async (id) => { calls.push({ method: "get", payload: id }); return project; },
        CreateCloudProject: async (input) => { calls.push({ method: "create", payload: input }); return project; },
        GetCloudProjectInstruction: async (id) => { calls.push({ method: "instruction-get", payload: id }); return instruction; },
        EditCloudProjectInstruction: async (input) => { calls.push({ method: "instruction-edit", payload: input }); return instruction; },
        ClearCloudProjectInstruction: async (input) => { calls.push({ method: "instruction-clear", payload: input }); return { ...instruction, humanDescription: "", agentInstruction: "", version: 4 }; },
        NewCloudProjectSession: async (input: unknown) => {
          const value = input as NewCloudProjectSessionInput;
          calls.push({ method: "session-new", payload: value });
          return { tabId: value.tabId, sessionPath: "session.json", projectId: value.projectId, instructionVersion: 4, contentHash: "b".repeat(64), reused: false };
        },
        GetCloudProjectFeatures: async () => { calls.push({ method: "project-features", payload: null }); return {
          projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: true,
          membersAvailable: true, activityAvailable: true, workItemFieldsAvailable: true,
          bindingsAvailable: true, assetsAvailable: true,
        }; },
        ListCloudProjectAssetDirectory: async (input) => { calls.push({ method: "asset-directory-list", payload: input }); return assetDirectoryPage; },
        ListCloudProjectAssetOperationRecords: async (input) => { calls.push({ method: "asset-operation-record-list", payload: input }); return assetOperationRecordPage; },
        CreateCloudProjectAssetFolder: async (input) => { calls.push({ method: "asset-folder-create", payload: input }); return assetDirectoryPage; },
        PatchCloudProjectAssetFolder: async (input) => { calls.push({ method: "asset-folder-patch", payload: input }); return assetDirectoryPage; },
        DeleteCloudProjectAssetFolder: async (input) => { calls.push({ method: "asset-folder-delete", payload: input }); return assetDirectoryPage; },
        PatchCloudProjectAsset: async (input) => { calls.push({ method: "asset-patch", payload: input }); return assetDirectoryPage; },
        ReplaceCloudProjectAssetContent: async (input) => { calls.push({ method: "asset-replace", payload: input }); return assetDirectoryPage; },
        UploadCloudProjectAssetToFolder: async (input) => { calls.push({ method: "asset-upload", payload: input }); return assetDirectoryPage; },
        ListCloudProjectWorkItems: async (input) => { calls.push({ method: "work-item-list", payload: input }); return { projectScopeKey: "opaque-a", projectId: input.projectId, items: [workItem], totalCount: 1, undatedCount: 1, nextCursor: null }; },
        CreateCloudProjectWorkItem: async (input) => { calls.push({ method: "work-item-create", payload: input }); return { projectScopeKey: "opaque-a", projectId: input.projectId, workItem }; },
        PatchCloudProjectWorkItem: async (input) => { calls.push({ method: "work-item-patch", payload: input }); return { projectScopeKey: "opaque-a", projectId: input.projectId, workItem }; },
        MoveCloudProjectWorkItem: async (input) => { calls.push({ method: "work-item-move", payload: input }); return { projectScopeKey: "opaque-a", projectId: input.projectId, workItem }; },
  DeleteCloudProjectWorkItem: async (input) => {
    calls.push({ method: "work-item-delete", payload: input });
    return { projectScopeKey: "opaque-a", projectId: input.projectId, workItemId: input.workItemId, committedVersion: input.expectedVersion + 1 };
  },
};

globalThis.window = {
  go: {
    main: {
      App: appBindings as AppBindings,
    },
  },
} as unknown as Window & typeof globalThis;

const listAllInput = { search: "server search", cursor: "opaque-cursor", status: "all" } as const satisfies CloudProjectListInput;
await projectGateway.listCloudProjects(listAllInput);
await projectGateway.listJoinedCloudProjects(listAllInput);
await projectGateway.claimCloudProjectOnboarding?.();
await projectGateway.getCloudProject("p-1");
await projectGateway.createCloudProject({ name: "Alpha", descriptionSet: false, description: null, agentInstruction: "先给出可执行的下一步" });
await projectGateway.getCloudProjectInstruction("p-1");
await projectGateway.editCloudProjectInstruction({ projectId: "p-1", expectedVersion: 3, humanDescription: "Updated", agentInstruction: "Rule" });
await projectGateway.clearCloudProjectInstruction({ projectId: "p-1", expectedVersion: 4 });
await projectGateway.newCloudProjectSession({ projectId: "p-1", tabId: "tab-1" });
await projectGateway.getCloudProjectFeatures();
await projectGateway.listCloudProjectAssetDirectory?.({ projectId: "p-1", folderId: "folder-1", search: "report", type: "pdf", sort: "updated_at", direction: "desc", cursor: "cursor-1", limit: 25 });
await projectGateway.listCloudProjectAssetOperationRecords?.({ projectId: "p-1", operation: "delete", resourceKind: "asset", actorUserId: "user-1", limit: 20 });
await projectGateway.createCloudProjectAssetFolder?.({ projectId: "p-1", name: "Reports", parentFolderId: "folder-1" });
await projectGateway.patchCloudProjectAssetFolder?.({ projectId: "p-1", folderId: "folder-2", expectedVersion: 2, parentFolderIdSet: true, parentFolderId: null });
await projectGateway.deleteCloudProjectAssetFolder?.({ projectId: "p-1", folderId: "folder-2", expectedVersion: 2 });
await projectGateway.patchCloudProjectAsset?.({ projectId: "p-1", assetId: "asset-1", expectedVersion: 3, folderIdSet: true, folderId: null });
await projectGateway.replaceCloudProjectAssetContent?.({ projectId: "p-1", assetId: "asset-1", expectedVersion: 3, folderId: "folder-1" });
await projectGateway.uploadCloudProjectAsset("p-1", "folder-1");
await projectGateway.listCloudProjectWorkItems({ projectId: "p-1", pageSize: 100, status: "all", scheduledFrom: "2026-08-01", scheduledTo: "2026-09-11" });
await projectGateway.createCloudProjectWorkItem({ projectId: "p-1", title: "Plan", descriptionSet: false, description: null, status: "todo" });
await projectGateway.patchCloudProjectWorkItem({ projectId: "p-1", workItemId: "w-1", expectedVersion: 1, title: "Updated", descriptionSet: true, description: null });
await projectGateway.moveCloudProjectWorkItem({ projectId: "p-1", workItemId: "w-1", expectedVersion: 1, targetStatus: "in_progress", beforeWorkItemId: "w-2" });
const deletedWorkItem = await projectGateway.deleteCloudProjectWorkItem({ projectId: "p-1", workItemId: "w-1", expectedVersion: 1 });

const create = calls.find((call) => call.method === "create")?.payload as Record<string, unknown>;
ok(create && !("idempotencyKey" in create), "React project gateway must not accept or generate an idempotency key");
ok(create.agentInstruction === "先给出可执行的下一步", "initial agent_instruction reaches the create App bridge in the same request");
ok(!JSON.stringify(create).includes("C:\\private\\alpha"), "local path must not enter a CloudProject mutation");
ok(!("ListCloudProjectBindings" in (window.go?.main?.App ?? {})), "public local workspace binding methods are absent from the Wails surface");
ok(page.stale === false && page.cachedAt === null, "page contract requires explicit cache metadata");
const projectTypesSource = readFileSync(new URL("../features/projects/model/projectTypes.ts", import.meta.url), "utf8");
ok(/export type CloudProjectListStatus = CloudProjectStatus \| "all";/.test(projectTypesSource), "React list status includes canonical all");
const list = calls.find((call) => call.method === "list")?.payload as CloudProjectListInput | undefined;
ok(list?.status === "all", "canonical all status reaches the Wails adapter unchanged");
const joinedList = calls.find((call) => call.method === "list-joined")?.payload as CloudProjectListInput | undefined;
ok(joinedList?.status === "all" && joinedList.search === "server search", "joined project filters reach the dedicated Cloud App bridge unchanged");
ok(calls.some((call) => call.method === "onboarding-claim"), "onboarding claim reaches the server-owned App bridge without UI input");
ok(calls.some((call) => call.method === "instruction-get" && call.payload === "p-1"), "instruction GET reaches the exact App bridge method");
ok(JSON.stringify(calls.find((call) => call.method === "instruction-edit")?.payload) === JSON.stringify({ projectId: "p-1", expectedVersion: 3, humanDescription: "Updated", agentInstruction: "Rule" }), "instruction edit preserves the optimistic version and both plain-text fields");
ok(JSON.stringify(calls.find((call) => call.method === "instruction-clear")?.payload) === JSON.stringify({ projectId: "p-1", expectedVersion: 4 }), "explicit instruction clear preserves the optimistic version");
ok(JSON.stringify(calls.find((call) => call.method === "session-new")?.payload) === JSON.stringify({ projectId: "p-1", tabId: "tab-1" }), "new project session reaches the exact App bridge method with the prepared tab identity");
ok(calls.some((call) => call.method === "project-features"), "verified Project feature availability reaches the exact App bridge method");
ok(JSON.stringify(calls.find((call) => call.method === "asset-directory-list")?.payload) === JSON.stringify({ projectId: "p-1", folderId: "folder-1", search: "report", type: "pdf", sort: "updated_at", direction: "desc", cursor: "cursor-1", limit: 25 }), "asset directory query reaches the App bridge unchanged");
ok(JSON.stringify(calls.find((call) => call.method === "asset-operation-record-list")?.payload) === JSON.stringify({ projectId: "p-1", operation: "delete", resourceKind: "asset", actorUserId: "user-1", limit: 20 }), "asset operation record filters reach the App bridge unchanged");
ok(JSON.stringify(calls.find((call) => call.method === "asset-folder-create")?.payload) === JSON.stringify({ projectId: "p-1", name: "Reports", parentFolderId: "folder-1" }), "folder create preserves its target parent");
ok(JSON.stringify(calls.find((call) => call.method === "asset-folder-patch")?.payload) === JSON.stringify({ projectId: "p-1", folderId: "folder-2", expectedVersion: 2, parentFolderIdSet: true, parentFolderId: null }), "folder move preserves optimistic version and explicit root target");
ok(JSON.stringify(calls.find((call) => call.method === "asset-folder-delete")?.payload) === JSON.stringify({ projectId: "p-1", folderId: "folder-2", expectedVersion: 2 }), "folder delete preserves project, folder identity, and optimistic version");
ok(JSON.stringify(calls.find((call) => call.method === "asset-patch")?.payload) === JSON.stringify({ projectId: "p-1", assetId: "asset-1", expectedVersion: 3, folderIdSet: true, folderId: null }), "asset move preserves optimistic version and explicit root target");
ok(JSON.stringify(calls.find((call) => call.method === "asset-replace")?.payload) === JSON.stringify({ projectId: "p-1", assetId: "asset-1", expectedVersion: 3, folderId: "folder-1" }), "asset replacement preserves project, asset, directory, and optimistic version identities");
ok(JSON.stringify(calls.find((call) => call.method === "asset-upload")?.payload) === JSON.stringify({ projectId: "p-1", folderId: "folder-1" }), "asset upload sends only cloud identities to the native picker bridge");
ok(JSON.stringify(calls.find((call) => call.method === "work-item-list")?.payload) === JSON.stringify({ projectId: "p-1", pageSize: 100, status: "all", scheduledFrom: "2026-08-01", scheduledTo: "2026-09-11" }), "WorkItem calendar range reaches the App bridge unchanged");
ok(!JSON.stringify(calls.find((call) => call.method === "work-item-create")?.payload).includes("rank"), "WorkItem create never accepts a client rank");
ok(JSON.stringify(calls.find((call) => call.method === "work-item-patch")?.payload) === JSON.stringify({ projectId: "p-1", workItemId: "w-1", expectedVersion: 1, title: "Updated", descriptionSet: true, description: null }), "WorkItem patch carries explicit project identity and no status/rank");
ok(JSON.stringify(calls.find((call) => call.method === "work-item-move")?.payload) === JSON.stringify({ projectId: "p-1", workItemId: "w-1", expectedVersion: 1, targetStatus: "in_progress", beforeWorkItemId: "w-2" }), "one anchored WorkItem move carries explicit project identity and no client rank");
ok(JSON.stringify(calls.find((call) => call.method === "work-item-delete")?.payload) === JSON.stringify({ projectId: "p-1", workItemId: "w-1", expectedVersion: 1 }), "WorkItem delete carries explicit project identity and optimistic version");
ok(deletedWorkItem.workItemId === "w-1" && deletedWorkItem.committedVersion === 2, "WorkItem delete returns the committed deletion receipt");
ok(["work-item-create", "work-item-patch", "work-item-move", "work-item-delete"].every((method) => !JSON.stringify(calls.find((call) => call.method === method)?.payload).includes("idempotencyKey")), "WorkItem mutations never accept or generate a UI idempotency key");

console.log("project gateway CSS-free contract passed");
