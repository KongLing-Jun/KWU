import { StrictMode, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { CloudProjectsSection } from "../../src/features/projects/ui/CloudProjectsSection";
import type { ProjectGateway } from "../../src/features/projects/data/projectGateway";
import type {
  CloudProject,
  CloudProjectInstruction,
  CloudProjectWorkItem,
  CloudProjectWorkItemStatus,
} from "../../src/features/projects/model/projectTypes";
import { LocaleProvider } from "../../src/lib/i18n";
import "../../src/styles.css";
import "./workbuddy-project.css";

type Scenario = "anonymous" | "unsupported" | "incompatible" | "loading" | "empty" | "detail" | "stale" | "conflict" | "archived" | "account-switch" | "board-move";

const params = new URLSearchParams(window.location.search);
const requestedScenario = params.get("scenario") || "detail";
const scenario: Scenario = ["anonymous", "unsupported", "incompatible", "loading", "empty", "detail", "stale", "conflict", "archived", "account-switch", "board-move"].includes(requestedScenario)
  ? requestedScenario as Scenario
  : "detail";
const theme = params.get("theme") === "dark" ? "dark" : "light";
const zoom = Math.min(200, Math.max(50, Number(params.get("zoom") || "100")));
document.documentElement.dataset.theme = theme;
document.documentElement.dataset.fixtureZoom = String(zoom);
document.documentElement.style.setProperty("--font-scale", String(zoom / 100));

const NOW = "2026-08-05T04:00:00.000Z";
const SCOPE_A = "opaque-fixture-scope-a";
const SCOPE_B = "opaque-fixture-scope-b";

function project(projectId: string, name: string, status: "active" | "archived" = "active", version = 1): CloudProject {
  return { projectId, name, description: `${name} deterministic description`, status, version, createdAt: NOW, updatedAt: NOW };
}

function instruction(projectId: string, projectScopeKey: string): CloudProjectInstruction {
  return {
    projectScopeKey,
    projectId,
    humanDescription: "Keep the fixture deterministic.",
    agentInstruction: "Use only synthetic fixture data.",
    contentHash: "fixture-content-hash",
    version: 3,
    createdAt: NOW,
    updatedAt: NOW,
  };
}

function codedError(code: string) {
  return Object.assign(new Error(`Synthetic ${code}`), { code });
}

function workItem(
  projectId: string,
  workItemId: string,
  title: string,
  status: CloudProjectWorkItemStatus,
  description: string | null,
  version: number,
): CloudProjectWorkItem {
  return {
    workItemId,
    projectId,
    title,
    description,
    status,
    rank: 100,
    version,
    createdAt: NOW,
    updatedAt: NOW,
    assigneeUserId: null,
    assigneeStatus: null,
    assigneeDisplayName: null,
    priority: "none",
    startAt: null,
    dueAt: null,
    labels: [],
  };
}

function createGateway(activeScenario: Scenario, accountScope: string, emit: (event: string) => void): ProjectGateway {
  const projectScopeKey = accountScope === "fixture-account-b" ? SCOPE_B : SCOPE_A;
  const baseProject = activeScenario === "archived"
    ? project("project-archived", "Archived Fixture", "archived", 4)
    : accountScope === "fixture-account-b"
      ? project("project-beta", "Beta Fixture")
      : project("project-alpha", "Alpha Fixture");
  let authoritative = baseProject;
  let workItems: CloudProjectWorkItem[] = [
    workItem(baseProject.projectId, "work-todo", "Prepare brief", "todo", "Synthetic todo", 1),
    workItem(baseProject.projectId, "work-active", "Review contract", "in_progress", null, 2),
    workItem(baseProject.projectId, "work-paused", "Await input", "paused", null, 1),
    workItem(baseProject.projectId, "work-done", "Freeze scope", "done", null, 5),
  ];
  const result = () => ({ projectScopeKey, project: authoritative, stale: activeScenario === "stale", cachedAt: activeScenario === "stale" ? NOW : null });
  const record = <T,>(name: string, value: T): T => {
    emit(name);
    return value;
  };

  const gateway: Partial<ProjectGateway> = {
    async listCloudProjects() {
      emit("listCloudProjects");
      if (activeScenario === "loading") return await new Promise(() => undefined);
      if (activeScenario === "unsupported") throw codedError("CAPABILITY_UNAVAILABLE");
      if (activeScenario === "incompatible") throw codedError("CLOUD_PROTOCOL_ERROR");
      return { projectScopeKey, items: activeScenario === "empty" ? [] : [authoritative], nextCursor: null, stale: activeScenario === "stale", cachedAt: activeScenario === "stale" ? NOW : null };
    },
    async getCloudProject() { return record("getCloudProject", result()); },
    async createCloudProject(input) {
      authoritative = project("project-created", input.name);
      return record("createCloudProject", result());
    },
    async patchCloudProject() {
      emit("patchCloudProject");
      if (activeScenario === "conflict") {
        authoritative = project(baseProject.projectId, "Authoritative Fixture", "active", baseProject.version + 1);
        throw codedError("CONFLICT");
      }
      return result();
    },
    async archiveCloudProject() {
      authoritative = { ...authoritative, status: "archived", version: authoritative.version + 1 };
      return record("archiveCloudProject", result());
    },
    async restoreCloudProject() {
      authoritative = { ...authoritative, status: "active", version: authoritative.version + 1 };
      return record("restoreCloudProject", result());
    },
    async getCloudProjectInstruction(projectId) { return record("getCloudProjectInstruction", instruction(projectId, projectScopeKey)); },
    async editCloudProjectInstruction(input) { return record("editCloudProjectInstruction", { ...instruction(input.projectId, projectScopeKey), humanDescription: input.humanDescription, agentInstruction: input.agentInstruction, version: input.expectedVersion + 1 }); },
    async clearCloudProjectInstruction(input) { return record("clearCloudProjectInstruction", { ...instruction(input.projectId, projectScopeKey), humanDescription: "", agentInstruction: "", version: input.expectedVersion + 1 }); },
    async newCloudProjectSession(input) { return record("newCloudProjectSession", { tabId: input.tabId, sessionPath: "fixture-session", projectId: input.projectId, instructionVersion: 3, contentHash: "fixture-content-hash", reused: false }); },
    async getCloudProjectFeatures() { return record("getCloudProjectFeatures", {
      projectScopeKey,
      instructionsAvailable: true,
      assetsAvailable: false,
      workItemsAvailable: activeScenario === "board-move" || activeScenario === "archived",
      membersAvailable: false,
      activityAvailable: false,
      workItemFieldsAvailable: true,
      bindingsAvailable: false,
    }); },
    async listProjectConnectorAvailability() {
      return [{
        registryRef: "tencent-docs",
        name: "腾讯文档",
        description: "协作文档",
        localServerName: "腾讯文档",
        available: true,
        availabilityReason: "",
      }];
    },
    async listCloudProjectCapabilities(projectId) { return record("listCloudProjectCapabilities", { projectScopeKey, projectId, projectVersion: authoritative.version, items: [] }); },
    async saveCloudProjectCapabilities(input) { return record("saveCloudProjectCapabilities", { projectScopeKey, projectId: input.projectId, projectVersion: input.expectedVersion + 1, items: [] }); },
    async listCloudProjectMembers(projectId) { return record("listCloudProjectMembers", { projectScopeKey, projectId, currentRole: "owner" as const, items: [] }); },
    async createCloudProjectInvite(input) { return record("createCloudProjectInvite", { projectScopeKey, projectId: input.projectId, inviteId: "fixture-invite", token: "fixture-token", expiresAt: NOW, committedWithoutSecret: false }); },
    async listCloudProjectInvites(projectId) { return record("listCloudProjectInvites", { projectScopeKey, projectId, items: [] }); },
    async revokeCloudProjectInvite() { emit("revokeCloudProjectInvite"); },
    async deactivateCloudProjectMember() { emit("deactivateCloudProjectMember"); },
    async acceptCloudProjectInvite() { return record("acceptCloudProjectInvite", { projectScopeKey, status: "accepted", projectId: authoritative.projectId, membershipVersion: 1 }); },
    async deleteCloudProject() { emit("deleteCloudProject"); },
    async listCloudProjectActivities(input) { return record("listCloudProjectActivities", { projectScopeKey, projectId: input.projectId, items: [], nextCursor: null, readCursor: null }); },
    async listCloudProjectMessages(input) { return record("listCloudProjectMessages", { projectScopeKey, projectId: input.projectId, items: [], nextCursor: null, readCursor: null }); },
    async postCloudProjectMessage(input) { return record("postCloudProjectMessage", { activityId: "fixture-activity", kind: "user_message" as const, actorUserId: "fixture-user", authorDisplayName: "Fixture User", clientMessageId: "fixture-message", messageText: input.text, mentionUserIds: input.mentionUserIds, assets: [], unread: false, createdAt: NOW }); },
    async markCloudProjectActivityRead() { emit("markCloudProjectActivityRead"); },
    async listCloudProjectWorkItems(input) {
      const matching = input.status ? workItems.filter((item) => item.status === input.status) : workItems;
      return record("listCloudProjectWorkItems", { projectScopeKey, projectId: authoritative.projectId, items: matching, nextCursor: null, totalCount: matching.length, undatedCount: matching.filter((item) => !item.startAt && !item.dueAt).length });
    },
    async createCloudProjectWorkItem(input) {
      const created = workItem(input.projectId, "work-created", input.title, input.status || "todo", input.description, 1);
      workItems = [...workItems, created];
      return record("createCloudProjectWorkItem", { projectScopeKey, projectId: input.projectId, workItem: created });
    },
    async patchCloudProjectWorkItem(input) {
      const current = workItems.find((item) => item.workItemId === input.workItemId) || workItems[0];
      const patched: CloudProjectWorkItem = {
        ...current,
        title: input.title || current.title,
        description: input.descriptionSet ? input.description ?? null : current.description,
        version: current.version + 1,
      };
      workItems = workItems.map((item) => item.workItemId === patched.workItemId ? patched : item);
      return record("patchCloudProjectWorkItem", { projectScopeKey, projectId: input.projectId, workItem: patched });
    },
    async moveCloudProjectWorkItem(input) {
      const current = workItems.find((item) => item.workItemId === input.workItemId) || workItems[0];
      const workItem = { ...current, status: input.targetStatus as CloudProjectWorkItemStatus, rank: 200, version: current.version + 1 };
      workItems = workItems.map((item) => item.workItemId === workItem.workItemId ? workItem : item);
      return record("moveCloudProjectWorkItem", { projectScopeKey, projectId: input.projectId, workItem });
    },
  };
  return gateway as ProjectGateway;
}

function Fixture() {
  const [accountScope, setAccountScope] = useState("fixture-account-a");
  const [events, setEvents] = useState<string[]>([]);
  const gateway = useMemo(() => createGateway(scenario, accountScope, (event) => setEvents((current) => [...current, event])), [accountScope]);
  const authenticated = scenario !== "anonymous";

  return (
    <main className="workbuddy-project-fixture" data-testid="workbuddy-project-fixture" data-scenario={scenario} data-theme={theme} data-zoom={zoom}>
      {scenario === "account-switch" ? (
        <div className="workbuddy-project-fixture__controls">
          <button type="button" data-testid="account-switch" onClick={() => setAccountScope("fixture-account-b")}>切换测试账号</button>
        </div>
      ) : null}
      <LocaleProvider>
        <CloudProjectsSection key={accountScope} accountScopeKey={accountScope} authenticated={authenticated} gateway={gateway} />
      </LocaleProvider>
      <output className="workbuddy-project-fixture__events" data-testid="fixture-events" aria-hidden="true">{events.join(" ")}</output>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<StrictMode><Fixture /></StrictMode>);
