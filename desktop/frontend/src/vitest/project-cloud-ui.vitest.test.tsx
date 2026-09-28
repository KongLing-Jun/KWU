import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "../lib/i18n";
import { CloudProjectsSection } from "../features/projects/ui/CloudProjectsSection";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import type { CloudProjectCapabilityBinding, CloudProjectSession } from "../features/projects/model/projectTypes";
import type { ProjectCapabilityKind } from "../features/projects/ui/CloudProjectsSectionImpl";
const bridgeMock = vi.hoisted(() => ({
  ListProjectConnectorAvailability: vi.fn(),
  SkillsSettings: vi.fn(),
  PublicSkills: vi.fn(),
}));

vi.mock("../lib/bridge", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/bridge")>();
  return { ...actual, app: { ...actual.app, ...bridgeMock } };
});

function project(id: string, name: string, version = 1, status: "active" | "archived" = "active") {
  return {
    projectId: id,
    name,
    description: null,
    status,
    version,
    createdAt: "2026-08-05T04:00:00Z",
    updatedAt: "2026-08-05T04:00:00Z",
  } as const;
}

function instruction(projectId = "p-1", version = 3, humanDescription = "Project context", agentInstruction = "Follow the project rules") {
  return {
    projectScopeKey: "opaque-a",
    projectId,
    humanDescription,
    agentInstruction,
    contentHash: "a".repeat(64),
    version,
    createdAt: "2026-08-05T04:00:00Z",
    updatedAt: "2026-08-05T04:00:00Z",
  };
}

function renderCloudProjects(gateway: ProjectGateway, accountScopeKey = "scope-a", onWorkspaceChange?: (open: boolean) => void, renderComposer?: (context: { onCreateSession: (tabId: string, workspacePath?: string) => Promise<CloudProjectSession> }) => React.ReactNode, onOpenCapability?: (kind: ProjectCapabilityKind, trigger: HTMLButtonElement, context: { sessionKey: string; projectId: string; onCapabilitiesChanged: (items: CloudProjectCapabilityBinding[]) => void; onCapabilityCatalogChanged: (kind: ProjectCapabilityKind, items: Array<{ id: string; name: string; iconUrl?: string }>) => void }) => void) {
  const verifiedGateway = {
    ...gateway,
    listJoinedCloudProjects: gateway.listJoinedCloudProjects ?? vi.fn().mockResolvedValue({ projectScopeKey: accountScopeKey === "scope-b" ? "opaque-b" : "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
    listProjectConnectorAvailability: gateway.listProjectConnectorAvailability ?? (() => bridgeMock.ListProjectConnectorAvailability()),
    listProjectSkillsSettings: gateway.listProjectSkillsSettings ?? (() => bridgeMock.SkillsSettings()),
    listProjectPublicSkills: gateway.listProjectPublicSkills ?? ((limit, cursor) => bridgeMock.PublicSkills(limit, cursor)),
    getCloudProjectFeatures: gateway.getCloudProjectFeatures ?? vi.fn().mockResolvedValue({ projectScopeKey: accountScopeKey === "scope-b" ? "opaque-b" : "opaque-a", instructionsAvailable: true, workItemsAvailable: false }),
  } as ProjectGateway;
  return render(
    <LocaleProvider>
      <CloudProjectsSection authenticated accountScopeKey={accountScopeKey} gateway={verifiedGateway} experts={[
        { id: "research-expert", name: "深度研究专家", meta: "专家库", iconUrl: "data:image/png;base64,expert-avatar" },
        { id: "review-expert", name: "评审专家", meta: "专家库" },
      ]} onWorkspaceChange={onWorkspaceChange} renderComposer={renderComposer} onOpenCapability={onOpenCapability} />
    </LocaleProvider>,
  );
}

async function openWorkspaceLifecycleMenu() {
  const trigger = await screen.findByRole("button", { name: "项目操作" });
  await userEvent.click(trigger);
  return screen.getByRole("menu");
}

describe("WorkBuddy CloudProject production surface", () => {

  it("joins a cloud project, opens its bindings, and installs a missing bound skill", async () => {
    const listCloudProjects = vi.fn()
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null })
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null });
    const listJoinedCloudProjects = vi.fn()
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null })
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("prj_shared", "共享项目")], nextCursor: null, stale: false, cachedAt: null });
    const joinCloudProject = vi.fn().mockResolvedValue({
      projectScopeKey: "opaque-a",
      projectId: "prj_shared",
      userId: "usr_current",
      role: "member",
      status: "active",
      displayName: "张三",
      version: 1,
      createdAt: "2026-08-20T10:30:00Z",
      updatedAt: "2026-08-20T10:30:00Z",
    });
    let skillInstalled = false;
    const installProjectPublicSkill = vi.fn().mockImplementation(async () => {
      skillInstalled = true;
      return { name: "planning", namespace: "reasonix", slug: "planning", version: "1.0.0", path: "C:/skills/planning/SKILL.md", reload_required: false };
    });
    const gateway = {
      listCloudProjects,
      listJoinedCloudProjects,
      joinCloudProject,
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false, membersAvailable: false, bindingsAvailable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("prj_shared", "共享项目"), stale: false, cachedAt: null }),
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({
        projectScopeKey: "opaque-a",
        projectId: "prj_shared",
        projectVersion: 1,
        items: [
          { bindingId: "bind-connector", kind: "connector", registryRef: "github", position: 0, config: {}, publicDisabled: false, version: 1 },
          { bindingId: "bind-expert", kind: "expert", registryRef: "research-expert", position: 0, config: {}, publicDisabled: false, version: 1 },
          { bindingId: "bind-skill", kind: "skill", registryRef: "@reasonix/planning", position: 0, config: {}, publicDisabled: false, version: 1 },
        ],
      }),
      listProjectConnectorAvailability: vi.fn().mockResolvedValue([{ registryRef: "github", name: "GitHub", description: "GitHub", localServerName: "GitHub MCP", available: true, availabilityReason: "", compatible: true }]),
      listProjectSkillsSettings: vi.fn().mockImplementation(async () => ({ skills: skillInstalled ? [{ name: "planning", uninstallId: "planning", enabled: true }] : [], skillRoots: [] })),
      listProjectPublicSkills: vi.fn().mockResolvedValue({ items: [{ name: "Planning", namespace: "reasonix", slug: "planning", description: "Plan" }], next_cursor: "", has_more: false }),
      installProjectPublicSkill,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway, "scope-a", undefined, undefined, vi.fn());

    await screen.findByRole("button", { name: /Alpha/ });
    await userEvent.click(screen.getByRole("button", { name: "加入云项目" }));
    const dialog = screen.getByRole("dialog", { name: "加入云项目" });
    await userEvent.click(within(dialog).getByRole("button", { name: "加入" }));
    expect(joinCloudProject).not.toHaveBeenCalled();
    expect(within(dialog).getByRole("alert")).toHaveTextContent("请输入项目 ID。");

    await userEvent.type(within(dialog).getByRole("textbox", { name: "项目 ID" }), "prj_shared");
    await userEvent.click(within(dialog).getByRole("button", { name: "加入" }));

    await waitFor(() => expect(joinCloudProject).toHaveBeenCalledWith({ projectId: "prj_shared" }));
    expect(await screen.findByRole("heading", { name: "共享项目" })).toBeVisible();
    expect(installProjectPublicSkill).toHaveBeenCalledWith("reasonix", "planning");
    const rail = screen.getByRole("complementary", { name: "项目配置" });
    expect(within(rail).getByLabelText("连接器 GitHub")).toBeVisible();
    expect(within(rail).getByLabelText("专家 深度研究专家")).toBeVisible();
    expect(within(rail).getByLabelText("技能 Planning")).toBeVisible();
    expect(screen.queryByRole("button", { name: "永久删除" })).not.toBeInTheDocument();
    expect(listCloudProjects).toHaveBeenCalledTimes(2);
    expect(listJoinedCloudProjects).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("dialog", { name: "加入云项目" })).not.toBeInTheDocument();
  });

  it("opens an existing joined project with the same capability bindings as an owned project", async () => {
    let skillInstalled = false;
    const installProjectPublicSkill = vi.fn().mockImplementation(async () => {
      skillInstalled = true;
      return { name: "planning", namespace: "reasonix", slug: "planning", version: "1.0.0", path: "C:/skills/planning/SKILL.md", reload_required: false };
    });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-owned", "我的项目")], nextCursor: null, stale: false, cachedAt: null }),
      listJoinedCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-shared", "团队共享项目")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false, membersAvailable: false, bindingsAvailable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-shared", "团队共享项目"), stale: false, cachedAt: null }),
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({
        projectScopeKey: "opaque-a",
        projectId: "p-shared",
        projectVersion: 2,
        items: [
          { bindingId: "shared-connector", kind: "connector", registryRef: "github", position: 0, config: {}, publicDisabled: false, version: 1 },
          { bindingId: "shared-expert", kind: "expert", registryRef: "research-expert", position: 0, config: {}, publicDisabled: false, version: 1 },
          { bindingId: "shared-skill", kind: "skill", registryRef: "@reasonix/planning", position: 0, config: {}, publicDisabled: false, version: 1 },
        ],
      }),
      listProjectConnectorAvailability: vi.fn().mockResolvedValue([{ registryRef: "github", name: "GitHub", description: "GitHub", localServerName: "GitHub MCP", available: true, availabilityReason: "", compatible: true }]),
      listProjectSkillsSettings: vi.fn().mockImplementation(async () => ({ skills: skillInstalled ? [{ name: "planning", uninstallId: "planning", enabled: true }] : [], skillRoots: [] })),
      listProjectPublicSkills: vi.fn().mockResolvedValue({ items: [{ name: "Planning", namespace: "reasonix", slug: "planning", description: "Plan" }], next_cursor: "", has_more: false }),
      installProjectPublicSkill,
    } as unknown as ProjectGateway;

    renderCloudProjects(gateway, "scope-a", undefined, undefined, vi.fn());

    expect(await screen.findByRole("heading", { name: "我已加入的项目" })).toBeVisible();
    const joinedList = screen.getByRole("list", { name: "我已加入的云项目列表" });
    expect(within(joinedList).queryByRole("button", { name: /我的项目/ })).not.toBeInTheDocument();
    await userEvent.click(within(joinedList).getByRole("button", { name: /团队共享项目/ }));
    expect(await screen.findByRole("heading", { name: "团队共享项目" })).toBeVisible();
    expect(gateway.listCloudProjectCapabilities).toHaveBeenCalledWith("p-shared");
    expect(installProjectPublicSkill).toHaveBeenCalledWith("reasonix", "planning");
    const rail = screen.getByRole("complementary", { name: "项目配置" });
    expect(within(rail).getByLabelText("连接器 GitHub")).toBeVisible();
    expect(within(rail).getByLabelText("专家 深度研究专家")).toBeVisible();
    expect(within(rail).getByLabelText("技能 Planning")).toBeVisible();
  });

  it("refreshes the accepted invite directory and opens the shared project", async () => {
    window.history.replaceState(null, "", "/#project-invite=invite-1&token=invite-token");
    const acceptCloudProjectInvite = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", status: "accepted", projectId: "p-shared", membershipVersion: 1 });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-shared", "共享项目")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false, membersAvailable: false }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-shared", "共享项目"), stale: false, cachedAt: null }),
      acceptCloudProjectInvite,
    } as unknown as ProjectGateway;

    renderCloudProjects(gateway);

    await waitFor(() => expect(acceptCloudProjectInvite).toHaveBeenCalledWith({ inviteId: "invite-1", token: "invite-token" }));
    expect(await screen.findByRole("heading", { name: "共享项目" })).toBeVisible();
    expect(window.location.hash).toBe("");
    expect(gateway.listCloudProjects).toHaveBeenCalledTimes(1);
  });

  it("lets active joined members edit instructions and capabilities while gating owner-only controls", async () => {
    const openCapability = vi.fn();
    const editCloudProjectInstruction = vi.fn().mockResolvedValue(instruction("p-shared", 4, "Project context", "Member project rule"));
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-shared", "共享项目")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: false, membersAvailable: true, activityAvailable: false, bindingsAvailable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-shared", "共享项目"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction("p-shared")),
      editCloudProjectInstruction,
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-shared", projectVersion: 1, items: [] }),
      listCloudProjectMembers: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-shared", currentRole: "member", items: [] }),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway, "scope-a", undefined, undefined, openCapability);

    await userEvent.click(await screen.findByRole("button", { name: /共享项目/ }));
    expect(await screen.findByRole("heading", { name: "共享项目" })).toBeVisible();
    await waitFor(() => expect(gateway.listCloudProjectMembers).toHaveBeenCalledWith("p-shared"));

    const editInstruction = screen.getByRole("button", { name: "编辑项目指令" });
    expect(editInstruction).toBeEnabled();
    fireEvent.click(editInstruction);
    const instructionDialog = screen.getByRole("dialog", { name: "项目指令" });
    fireEvent.change(within(instructionDialog).getByRole("textbox"), { target: { value: "Member project rule" } });
    fireEvent.click(within(instructionDialog).getByRole("button", { name: "保存项目指令" }));
    await waitFor(() => expect(editCloudProjectInstruction).toHaveBeenCalledWith({
      projectId: "p-shared",
      expectedVersion: 3,
      humanDescription: "Project context",
      agentInstruction: "Member project rule",
    }));
    const addConnector = screen.getByRole("button", { name: "添加连接器" });
    expect(addConnector).toBeEnabled();
    fireEvent.click(addConnector);
    expect(openCapability).toHaveBeenCalledWith("connector", addConnector, expect.objectContaining({ projectId: "p-shared" }));
    expect(screen.queryByRole("button", { name: "邀请" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "项目操作" })).not.toBeInTheDocument();
  });

  it("keeps a newer project search when an older load-more request fails", async () => {
    let rejectOlderPage!: (reason?: unknown) => void;
    const olderPage = new Promise((_resolve, reject) => { rejectOlderPage = reject; });
    const listCloudProjects = vi.fn((input: { search?: string; cursor?: string }) => {
      if (input.cursor === "page-2") return olderPage;
      if (input.search === "Beta") return Promise.resolve({ projectScopeKey: "opaque-a", items: [project("p-2", "Beta")], nextCursor: null, stale: false, cachedAt: null });
      return Promise.resolve({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: "page-2", stale: false, cachedAt: null });
    });
    const gateway = { listCloudProjects } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    expect(await screen.findByRole("button", { name: /Alpha/ })).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "加载更多云项目" }));
    const search = screen.getByRole("searchbox", { name: "搜索云项目" });
    await userEvent.type(search, "Beta");
    await userEvent.keyboard("{Enter}");
    expect(await screen.findByRole("button", { name: /Beta/ })).toBeVisible();
    await act(async () => rejectOlderPage(new Error("late load-more failure")));

    expect(screen.getByRole("button", { name: /Beta/ })).toBeVisible();
    expect(screen.queryByText("云项目加载失败，请稍后重试。")).not.toBeInTheDocument();
  });

  it("keeps the newest selected project when an older detail request fails", async () => {
    let rejectAlpha!: (reason?: unknown) => void;
    const alphaDetail = new Promise((_resolve, reject) => { rejectAlpha = reject; });
    const getCloudProject = vi.fn((projectId: string) => projectId === "p-1" ? alphaDetail : Promise.resolve({ projectScopeKey: "opaque-a", project: project("p-2", "Beta"), stale: false, cachedAt: null }));
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha"), project("p-2", "Beta")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false, bindingsAvailable: true }),
      getCloudProject,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));
    await userEvent.click(screen.getByRole("button", { name: /Beta/ }));
    expect(await screen.findByRole("heading", { name: "Beta" })).toBeVisible();
    await act(async () => rejectAlpha(new Error("late detail failure")));

    expect(screen.getByRole("heading", { name: "Beta" })).toBeVisible();
    expect(screen.queryByText("云项目详情加载失败，请重试。")).not.toBeInTheDocument();
  });

  it("keeps loaded project detail usable when optional capability hydration fails", async () => {
    const listCloudProjectCapabilities = vi.fn().mockRejectedValue(new Error("capability hydration unavailable"));
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false, bindingsAvailable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      listCloudProjectCapabilities,
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);

    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));
    expect(await screen.findByRole("heading", { name: "Alpha" })).toBeVisible();
    await waitFor(() => expect(listCloudProjectCapabilities).toHaveBeenCalledTimes(1));
    await userEvent.click(view.container.querySelector<HTMLButtonElement>(".cloud-projects__rail-toggle")!);

    expect(screen.getByRole("heading", { name: "Alpha" })).toBeVisible();
    expect(screen.queryByText("云项目详情加载失败，请刷新后重试。")).not.toBeInTheDocument();
  });

  it("does not let an older member response replace the newer project members", async () => {
    let resolveAlphaMembers!: (value: any) => void;
    const alphaMembers = new Promise((resolve) => { resolveAlphaMembers = resolve; });
    const listCloudProjectMembers = vi.fn((projectId: string) => projectId === "p-1" ? alphaMembers : Promise.resolve({
      projectScopeKey: "opaque-a", projectId: "p-2", currentRole: "owner", items: [{ userId: "beta-user", displayName: "Beta 成员", role: "member", status: "active", version: 1 }],
    }));
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha"), project("p-2", "Beta")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false, membersAvailable: true, activityAvailable: true }),
      getCloudProject: vi.fn((projectId: string) => Promise.resolve({ projectScopeKey: "opaque-a", project: project(projectId, projectId === "p-1" ? "Alpha" : "Beta"), stale: false, cachedAt: null })),
      listCloudProjectMembers,
      listCloudProjectMessages: vi.fn((input: { projectId: string }) => Promise.resolve({ projectScopeKey: "opaque-a", projectId: input.projectId, items: [], nextCursor: null, readCursor: null })),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));
    expect(await screen.findByRole("heading", { name: "Alpha" })).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "项目" }));
    await userEvent.click(await screen.findByRole("button", { name: /Beta/ }));
    await userEvent.click(await screen.findByRole("button", { name: "发布留言" }));
    await userEvent.type(screen.getByRole("textbox", { name: "发布项目留言" }), "@");
    expect(screen.getByRole("option", { name: "@Beta 成员" })).toBeVisible();
    await act(async () => resolveAlphaMembers({ projectScopeKey: "opaque-a", projectId: "p-1", currentRole: "owner", items: [{ userId: "alpha-user", displayName: "Alpha 成员", role: "member", status: "active", version: 1 }] }));

    expect(screen.getByRole("option", { name: "@Beta 成员" })).toBeVisible();
    expect(screen.queryByRole("option", { name: "@Alpha 成员" })).not.toBeInTheDocument();
  });

  it("creates a project with a 15-character name and an optional instruction template", async () => {
    const createCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "产品发布"), stale: false, cachedAt: null });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject,
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "产品发布"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction("p-2", 1)),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    await userEvent.click(await screen.findByRole("button", { name: "新建云项目" }));
    const dialog = screen.getByRole("dialog", { name: "新建云项目" });
    const name = within(dialog).getByLabelText("项目名称");
    expect(name).toHaveAttribute("maxlength", "15");
    expect(within(dialog).getByText("0/15")).toBeVisible();
    await userEvent.type(name, "产品发布");
    expect(within(dialog).getByText("4/15")).toBeVisible();

    const template = within(dialog).getByLabelText("指令模板");
    const agentInstruction = within(dialog).getByLabelText("项目指令");
    expect(template).toHaveAttribute("data-value", "");
    expect(template).toHaveTextContent("选择模板");
    expect(agentInstruction).toHaveValue("");
    await userEvent.click(template);
    await userEvent.click(within(dialog).getByRole("option", { name: "产品需求全流程" }));
    expect(agentInstruction).not.toHaveValue("");
    const selectedInstruction = (agentInstruction as HTMLTextAreaElement).value;

    await userEvent.click(within(dialog).getByRole("button", { name: "创建" }));
    expect(createCloudProject).toHaveBeenCalledWith({
      name: "产品发布",
      descriptionSet: false,
      description: null,
      humanDescription: "从需求澄清、范围冻结到验收和复盘。",
      agentInstruction: selectedInstruction,
    });
  });

  it("constrains the template list to the dialog body instead of scrolling the dialog", async () => {
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    await userEvent.click(await screen.findByRole("button", { name: "新建云项目" }));
    const dialog = screen.getByRole("dialog", { name: "新建云项目" });
    const body = dialog.querySelector<HTMLElement>(".cloud-project-dialog__body");
    const selectWrap = dialog.querySelector<HTMLElement>(".cloud-project-dialog__select-wrap");
    expect(body).not.toBeNull();
    expect(selectWrap).not.toBeNull();
    vi.spyOn(body!, "getBoundingClientRect").mockReturnValue({ top: 200, right: 640, bottom: 500, left: 0, width: 640, height: 300, x: 0, y: 200, toJSON: () => ({}) });
    vi.spyOn(selectWrap!, "getBoundingClientRect").mockReturnValue({ top: 320, right: 480, bottom: 356, left: 240, width: 240, height: 36, x: 240, y: 320, toJSON: () => ({}) });

    await userEvent.click(within(dialog).getByLabelText("指令模板"));

    const menu = within(dialog).getByRole("listbox", { name: "指令模板" });
    expect(menu).toHaveAttribute("data-placement", "bottom");
    expect(menu).toHaveStyle({ maxHeight: "134px" });
  });

  it("creates a project with every selected expert binding", async () => {
    const createCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "协作评审"), stale: false, cachedAt: null });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject,
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "协作评审"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction("p-2", 1)),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    await userEvent.click(await screen.findByRole("button", { name: "新建云项目" }));
    const dialog = screen.getByRole("dialog", { name: "新建云项目" });
    await userEvent.type(within(dialog).getByLabelText("项目名称"), "协作评审");
    await userEvent.click(within(dialog).getByText("项目能力（可选）"));
    const addExpert = within(dialog).getByRole("button", { name: "添加专家" });
    await userEvent.click(addExpert);

    const capabilityDialog = await screen.findByRole("dialog", { name: "选择专家" });
    await userEvent.click(within(capabilityDialog).getByRole("button", { name: /深度研究专家/ }));
    await userEvent.click(within(capabilityDialog).getByRole("button", { name: /评审专家/ }));
    expect(within(capabilityDialog).getByText("已选择 2 项")).toBeVisible();
    await userEvent.click(within(capabilityDialog).getByRole("button", { name: "确定" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "创建" }));

    expect(createCloudProject).toHaveBeenCalledWith({
      name: "协作评审",
      descriptionSet: false,
      description: null,
      bindings: [
        { kind: "expert", registryRef: "research-expert", position: 0, config: {} },
        { kind: "expert", registryRef: "review-expert", position: 1, config: {} },
      ],
    });
  });

  it("claims and deduplicates the onboarding project without replacing its normal workspace", async () => {
    const guide = project("p-guide", "项目指引");
    const regular = project("p-1", "Alpha");
    const claimCloudProjectOnboarding = vi.fn().mockResolvedValue({
      projectScopeKey: "opaque-a",
      project: guide,
      guideVersion: 1,
      state: "active",
      created: true,
    });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [regular, guide], nextCursor: null, stale: false, cachedAt: null }),
      claimCloudProjectOnboarding,
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: guide, stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction("p-guide", 1)),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);

    await waitFor(() => expect(claimCloudProjectOnboarding).toHaveBeenCalledTimes(1));
    const cards = view.container.querySelectorAll<HTMLButtonElement>(".cloud-projects__card");
    expect(cards).toHaveLength(2);
    expect(cards[0]).toHaveTextContent("项目指引");
    expect(screen.queryByText("集中管理云端项目、计划项与协作上下文")).not.toBeInTheDocument();
    expect(screen.getByText("多人协同，打造超级团队")).toBeVisible();
    expect(screen.queryByText("选择常用工作流，在创建项目之前继续调整已经准备好的指令。")).not.toBeInTheDocument();
    expect(screen.queryByText("2 个云项目")).not.toBeInTheDocument();
    await userEvent.click(cards[0]);
    expect(await screen.findByText("动态能力暂未开放")).toBeVisible();
    expect(screen.queryByRole("heading", { name: "从这里开始使用项目" })).not.toBeInTheDocument();
    expect(view.container.querySelector("[data-project-onboarding-guide]")).toBeNull();
    expect(gateway.getCloudProject).toHaveBeenCalledWith("p-guide");
  });

  it("Goal 12 prepares a CloudProject from a template instead of starting a conversation", async () => {
    bridgeMock.ListProjectConnectorAvailability.mockResolvedValue([{ registryRef: "tencent-docs", name: "腾讯文档", description: "协作文档", compatible: true, localServerName: "腾讯文档", available: true, availabilityReason: "" }]);
    const createCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "市场分析"), stale: false, cachedAt: null });
    const createdBinding = { bindingId: "binding-1", kind: "connector", registryRef: "tencent-docs", position: 0, config: {}, publicDisabled: false, version: 1 };
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject,
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "市场分析"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction("p-2", 1, "搜集可靠来源，将结论整理成简明报告。", "edited template instruction")),
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-2", projectVersion: 1, items: [createdBinding] }),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);

    await screen.findByText("从模板创建");
    const templateCards = view.container.querySelectorAll<HTMLButtonElement>(".project-directory__starter");
    expect(templateCards).toHaveLength(5);
    await userEvent.click(templateCards[0]);
    const dialog = screen.getByRole("dialog", { name: "新建项目" });
    const name = within(dialog).getByLabelText("项目名称");
    expect(name).toHaveFocus();
    await userEvent.click(within(dialog).getByRole("button", { name: "放大弹窗" }));
    expect(dialog).toHaveClass("project-modal-surface--maximized");
    await userEvent.click(within(dialog).getByRole("button", { name: "还原弹窗" }));
    const selector = within(dialog).getByLabelText("指令模板");
    const agentInstruction = within(dialog).getByLabelText("项目指令");
    expect(selector).toHaveAttribute("data-value", "report");
    expect(selector).toHaveTextContent("产品需求全流程");
    expect((agentInstruction as HTMLTextAreaElement).value).toContain("EARS 原则");
    expect(agentInstruction).not.toBeVisible();
    await userEvent.click(within(dialog).getByText("项目指令"));
    expect(within(dialog).getByText("切换模板将覆盖当前编辑内容")).toBeVisible();
    await userEvent.click(within(dialog).getByText("项目能力（可选）"));
    expect(within(dialog).getByText("专家")).toBeVisible();

    const addConnector = within(dialog).getByRole("button", { name: "添加连接器" });
    await userEvent.click(addConnector);
    const capabilityDialog = await screen.findByRole("dialog", { name: "选择连接器" });
    expect(dialog).toHaveAttribute("aria-hidden", "true");
    expect(dialog).toHaveAttribute("inert");
    await userEvent.click(within(capabilityDialog).getByRole("tab", { name: "全部连接器" }));
    await userEvent.click(within(capabilityDialog).getByRole("button", { name: /腾讯文档/ }));
    await userEvent.click(within(capabilityDialog).getByRole("button", { name: "确定" }));
    expect(await screen.findByRole("dialog", { name: "新建项目" })).toBe(dialog);
    await waitFor(() => expect(addConnector).toHaveFocus());
    expect(within(dialog).getByText("腾讯文档")).toBeVisible();

    await userEvent.clear(agentInstruction);
    await userEvent.type(agentInstruction, "edited template instruction");
    await userEvent.click(selector);
    await userEvent.click(within(dialog).getByRole("option", { name: "市场调研与竞品分析" }));
    expect(agentInstruction).not.toHaveValue("edited template instruction");
    const switched = (agentInstruction as HTMLTextAreaElement).value;
    await userEvent.type(agentInstruction, " retained");
    await userEvent.click(selector);
    await userEvent.click(within(dialog).getByRole("option", { name: "市场调研与竞品分析" }));
    expect(agentInstruction).toHaveValue(`${switched} retained`);

    await userEvent.type(name, "市场分析");
    await userEvent.click(within(dialog).getByRole("button", { name: "确定" }));
    expect(createCloudProject).toHaveBeenCalledWith({
      name: "市场分析",
      descriptionSet: false,
      description: null,
      humanDescription: "定义课题、核验来源并形成竞品与机会分析。",
      agentInstruction: switched + " retained",
      bindings: [{ kind: "connector", registryRef: "tencent-docs", position: 0, config: {} }],
    });
    expect(await screen.findByRole("heading", { name: "市场分析" })).toBeVisible();
  });

  it("renames and deletes the chosen directory card without opening the wrong workspace", async () => {
    const alpha = project("p-1", "Alpha");
    const beta = project("p-2", "Beta");
    const renamed = project("p-1", "Alpha Next", 2);
    const patchCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: renamed, stale: false, cachedAt: null });
    const deleteCloudProject = vi.fn().mockResolvedValue(undefined);
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [alpha, beta], nextCursor: null, stale: false, cachedAt: null }),
      patchCloudProject,
      deleteCloudProject,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    let menuTriggers = await screen.findAllByRole("button", { name: "项目操作" });
    await userEvent.click(menuTriggers[0]);
    const renameMenu = screen.getByRole("menu", { name: "Alpha 操作菜单" });
    await userEvent.click(within(renameMenu).getByRole("menuitem", { name: "重命名" }));
    const editDialog = screen.getByRole("dialog", { name: "编辑云项目" });
    const name = within(editDialog).getByLabelText("项目名称");
    await userEvent.clear(name);
    await userEvent.type(name, "Alpha Next");
    await userEvent.click(within(editDialog).getByRole("button", { name: "保存" }));

    expect(patchCloudProject).toHaveBeenCalledWith({ projectId: "p-1", expectedVersion: 1, name: "Alpha Next", descriptionSet: true, description: null });
    expect(await screen.findByRole("heading", { name: "我的项目" })).toBeVisible();
    expect(screen.getByRole("button", { name: "查看云项目Alpha Next" })).toBeVisible();

    menuTriggers = screen.getAllByRole("button", { name: "项目操作" });
    await userEvent.click(menuTriggers[1]);
    const deleteMenu = screen.getByRole("menu", { name: "Beta 操作菜单" });
    await userEvent.click(within(deleteMenu).getByRole("menuitem", { name: "删除" }));
    const deleteDialog = screen.getByRole("dialog", { name: "永久删除云项目" });
    await userEvent.click(within(deleteDialog).getByRole("button", { name: "永久删除" }));

    expect(deleteCloudProject).toHaveBeenCalledWith({ projectId: "p-2", expectedVersion: 1 });
    await waitFor(() => expect(screen.queryByRole("button", { name: "查看云项目Beta" })).not.toBeInTheDocument());
    expect(screen.getByRole("heading", { name: "我的项目" })).toBeVisible();
  });

  it("closes the workspace lifecycle menu on outside pointerdown, Escape, and before opening an action dialog", async () => {
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false, membersAvailable: false }),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);
    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));

    const trigger = await screen.findByRole("button", { name: "项目操作" });
    await userEvent.click(trigger);
    expect(screen.getByRole("menu", { name: "Alpha 项目操作菜单" })).toBeVisible();

    const planTab = screen.getByRole("button", { name: /^事项$/ });
    fireEvent.pointerDown(planTab);
    expect(screen.queryByRole("menu", { name: "Alpha 项目操作菜单" })).not.toBeInTheDocument();
    fireEvent.click(planTab);
    expect(planTab).toHaveAttribute("aria-current", "page");

    await userEvent.click(trigger);
    expect(screen.getByRole("menu", { name: "Alpha 项目操作菜单" })).toBeVisible();
    await userEvent.click(trigger);
    expect(screen.queryByRole("menu", { name: "Alpha 项目操作菜单" })).not.toBeInTheDocument();
    await userEvent.click(trigger);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("menu", { name: "Alpha 项目操作菜单" })).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();

    const menu = await openWorkspaceLifecycleMenu();
    await userEvent.click(within(menu).getByRole("menuitem", { name: "编辑云项目" }));
    expect(screen.queryByRole("menu", { name: "Alpha 项目操作菜单" })).not.toBeInTheDocument();
    const dialog = screen.getByRole("dialog", { name: "编辑云项目" });
    await userEvent.click(within(dialog).getByRole("button", { name: "取消" }));
    await waitFor(() => expect(trigger).toHaveFocus());
  });

  it("Goal 12 discards a cancelled template draft without mutation and restores focus", async () => {
    const createCloudProject = vi.fn();
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject,
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);
    await screen.findByText("从模板创建");
    const card = view.container.querySelector<HTMLButtonElement>(".project-directory__starter") as HTMLButtonElement;
    await userEvent.click(card);
    const dialog = screen.getByRole("dialog", { name: "新建项目" });
    await userEvent.type(within(dialog).getByLabelText("项目名称"), "不会创建");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "新建项目" })).not.toBeInTheDocument();
    expect(createCloudProject).not.toHaveBeenCalled();
    await waitFor(() => expect(card).toHaveFocus());
  });

  it("Goal 12 keeps template creation behind the cloud write gate", async () => {
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: true, cachedAt: "2026-08-06T12:00:00Z" }),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);
    await screen.findByText("从模板创建");
    const cards = view.container.querySelectorAll<HTMLButtonElement>(".project-directory__starter");
    expect(cards).toHaveLength(5);
    for (const card of cards) expect(card).toBeDisabled();
  });

  it("Goal 12 announces mutation failures without blaming a valid project name", async () => {
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject: vi.fn().mockRejectedValue(new Error("network unavailable")),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);
    await screen.findByText("从模板创建");
    await userEvent.click(view.container.querySelector<HTMLButtonElement>(".project-directory__starter") as HTMLButtonElement);
    const dialog = screen.getByRole("dialog", { name: "新建项目" });
    const name = within(dialog).getByLabelText("项目名称");
    await userEvent.click(within(dialog).getByRole("button", { name: "确定" }));
    expect(name).toHaveAttribute("aria-invalid", "true");
    await userEvent.type(name, "有效项目名");
    expect(name).not.toHaveAttribute("aria-invalid", "true");
    await userEvent.click(within(dialog).getByRole("button", { name: "确定" }));
    expect(await within(dialog).findByRole("alert")).toBeVisible();
    expect(name).not.toHaveAttribute("aria-invalid", "true");
  });

  it("shows a specific project quota message while preserving the new-project draft", async () => {
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject: vi.fn().mockRejectedValue({ code: "QUOTA_EXCEEDED", message: "project quota is exceeded" }),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);
    await screen.findByText("从模板创建");
    await userEvent.click(view.container.querySelector<HTMLButtonElement>(".project-directory__starter") as HTMLButtonElement);
    const dialog = screen.getByRole("dialog", { name: "新建项目" });
    const name = within(dialog).getByLabelText("项目名称");
    await userEvent.type(name, "保留的项目名");
    await userEvent.click(within(dialog).getByRole("button", { name: "确定" }));

    expect(await within(dialog).findByText("当前账号已达到可创建项目数量上限。请删除不需要的项目后重试，或查看套餐权益。")).toBeVisible();
    expect(name).toHaveValue("保留的项目名");
    expect(name).not.toHaveAttribute("aria-invalid", "true");
  });
  it("Goal 11 renders truthful Activity/Assets shells and a scope-bound owner-relation drawer", async () => {
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction()),
      listCloudProjectWorkItems: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [], nextCursor: null }),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);
    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));

    expect(view.container.querySelector("[data-project-activity-shell]")).not.toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "资产" }));
    expect(view.container.querySelector("[data-project-assets-shell]")).not.toBeNull();

    await userEvent.click(view.container.querySelector<HTMLButtonElement>(".cloud-projects__invite") as HTMLButtonElement);
    const drawer = view.container.querySelector("[data-project-member-drawer]");
    expect(drawer).not.toBeNull();
    expect(drawer).toHaveAttribute("data-owner-relation", "current-account");
    expect(drawer?.textContent).not.toMatch(/opaque-a|scope-a/);
    const close = within(drawer as HTMLElement).getByRole("button", { name: "关闭" });
    await waitFor(() => expect(close).toHaveFocus());
    fireEvent.keyDown(drawer as HTMLElement, { key: "Tab" });
    expect(close).toHaveFocus();
    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(view.container.querySelector("[data-project-member-drawer]")).toBeNull());
    await waitFor(() => expect(view.container.querySelector(".cloud-projects__invite")).toHaveFocus());

    await userEvent.click(view.container.querySelector<HTMLButtonElement>(".cloud-projects__invite") as HTMLButtonElement);

    view.rerender(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="scope-b" gateway={gateway} />
      </LocaleProvider>,
    );
    expect(view.container.querySelector("[data-project-member-drawer]")).toBeNull();
  });

  it("switches from the WorkBuddy-style directory to a real-data workspace without a nested sidebar or fake future facts", async () => {
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction()),
      listCloudProjectWorkItems: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [], nextCursor: null }),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway);

    expect(view.container.querySelector('[data-project-view="directory"]')).not.toBeNull();
    expect(view.container.querySelector('[data-project-hero]')).not.toBeNull();
    expect(screen.getByRole("heading", { name: "我的项目" })).toBeVisible();
    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));

    expect(view.container.querySelector('[data-project-view="workspace"]')).not.toBeNull();
    expect(view.container.querySelector('[data-project-view="directory"]')).toBeNull();
    expect(view.container.querySelector("[data-project-sidebar]")).toBeNull();
    expect(screen.getByRole("navigation", { name: "项目视图" })).toBeVisible();
    for (const tab of ["动态", "事项", "资产"]) {
      expect(screen.getByRole("button", { name: tab })).toBeVisible();
    }
    expect(screen.queryByRole("button", { name: "执行会话" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "邀请" })).not.toHaveAttribute("aria-disabled", "true");
    const workspaceHeader = view.container.querySelector(".cloud-projects__workspace-header") as HTMLElement;
    expect(within(workspaceHeader).queryByRole("button", { name: /项目配置/ })).not.toBeInTheDocument();
    expect(screen.getByText("成员协作暂未开放")).toBeVisible();
    const configurationRail = screen.getByRole("complementary", { name: "项目配置" });
    expect(configurationRail).toBeVisible();
    const collapseConfiguration = within(configurationRail).getByRole("button", { name: "收起项目配置" });
    await userEvent.click(collapseConfiguration);
    expect(view.container.querySelector(".cloud-projects__workspace-grid--configuration-hidden")).not.toBeNull();
    expect(screen.queryByRole("complementary", { name: "项目配置" })).not.toBeInTheDocument();
    const expandConfiguration = screen.getByRole("button", { name: "展开项目配置" });
    expect(expandConfiguration).toHaveAttribute("aria-expanded", "false");
    expect(expandConfiguration.parentElement).toHaveClass("cloud-projects__workspace-grid--configuration-hidden");
    await waitFor(() => expect(expandConfiguration).toHaveFocus());
    await userEvent.click(expandConfiguration);
    const reopenedConfiguration = screen.getByRole("complementary", { name: "项目配置" });
    await waitFor(() => expect(within(reopenedConfiguration).getByRole("button", { name: "收起项目配置" })).toHaveFocus());
    expect(within(reopenedConfiguration).queryByText("Alpha")).not.toBeInTheDocument();
    expect(within(reopenedConfiguration).queryByText("同步云端项目")).not.toBeInTheDocument();
    expect(within(reopenedConfiguration).queryByText(/指令版本/)).not.toBeInTheDocument();
    expect(within(reopenedConfiguration).queryByText(/保存或清空只会用于/)).not.toBeInTheDocument();
    expect(view.container.querySelector("[data-project-composer]")).not.toBeNull();
    expect(screen.getByText("动态能力暂未开放")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "事项" }));
    await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(4));
    expect(view.container.querySelector('.work-item-board[data-work-item-view="plan"]')).not.toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "资产" }));
    expect(screen.getByText("资产能力暂未开放")).toBeVisible();
  });

  it("renders saved project capabilities as icon chips in the configuration rail", async () => {
    const openCapability = vi.fn();
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: false, membersAvailable: false, activityAvailable: false, workItemFieldsAvailable: true, bindingsAvailable: true, assetsAvailable: false }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction()),
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({
        projectScopeKey: "opaque-a",
        projectId: "p-1",
        projectVersion: 1,
        items: [
          { bindingId: "b-1", kind: "connector", registryRef: "github", position: 0, config: {}, publicDisabled: false, version: 1 },
          { bindingId: "b-2", kind: "skill", registryRef: "@reasonix/planning", position: 0, config: {}, publicDisabled: false, version: 1 },
          { bindingId: "b-3", kind: "expert", registryRef: "research-expert", position: 0, config: {}, publicDisabled: false, version: 1 },
        ],
      }),
      listProjectConnectorAvailability: vi.fn().mockResolvedValue([{ registryRef: "github", name: "GitHub", description: "GitHub", localServerName: "GitHub MCP", available: true, availabilityReason: "" }]),
      listProjectSkillsSettings: vi.fn().mockResolvedValue({ skills: [{ name: "planning", uninstallId: "planning", enabled: true }], skillRoots: [] }),
      listProjectPublicSkills: vi.fn().mockResolvedValue({ items: [{ name: "Planning", namespace: "reasonix", slug: "planning", description: "Plan", iconUrl: "https://example.test/planning.png" }], next_cursor: "", has_more: false }),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway, "scope-a", undefined, undefined, openCapability);

    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));
    const rail = screen.getByRole("complementary", { name: "项目配置" });
    expect(within(rail).getByText("连接器 1")).toBeVisible();
    expect(await within(rail).findByLabelText("连接器 GitHub")).toBeVisible();
    expect(within(rail).getByText("技能 1")).toBeVisible();
    expect(within(rail).getByLabelText("技能 Planning")).toBeVisible();
    expect(within(rail).getByTitle("Planning").querySelector("img")).toHaveAttribute("src", "https://example.test/planning.png");
    expect(within(rail).getByText("专家 1")).toBeVisible();
    expect(within(rail).getByLabelText("专家 深度研究专家")).toBeVisible();
    expect(within(rail).getByTitle("深度研究专家").querySelector("img")).toHaveAttribute("src", "data:image/png;base64,expert-avatar");
    await userEvent.click(within(rail).getByRole("button", { name: "添加连接器" }));
    expect(openCapability).toHaveBeenCalledWith("connector", expect.any(HTMLButtonElement), expect.objectContaining({ projectId: "p-1", onCapabilitiesChanged: expect.any(Function), onCapabilityCatalogChanged: expect.any(Function) }));
    const capabilityContext = openCapability.mock.calls[0]?.[2];
    capabilityContext.onCapabilityCatalogChanged("connector", [{ id: "github", name: "GitHub", iconUrl: "https://example.test/github.png" }]);
    expect(await within(rail).findByLabelText("连接器 GitHub")).toBeVisible();
    expect(within(rail).getByTitle("GitHub").querySelector("img")).toHaveAttribute("src", "https://example.test/github.png");
    expect(within(rail).queryByText("使用现有能力")).not.toBeInTheDocument();
  });

  it("keeps base project detail usable without requesting or exposing Instruction actions when its verified slice is unavailable", async () => {
    const getCloudProjectInstruction = vi.fn();
    const newCloudProjectSession = vi.fn();
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "No instructions")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: false, workItemsAvailable: false }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "No instructions"), stale: false, cachedAt: null }),
      getCloudProjectInstruction,
      newCloudProjectSession,
    } as unknown as ProjectGateway;

    renderCloudProjects(gateway);
    await userEvent.click(await screen.findByRole("button", { name: /No instructions/ }));

    expect(await screen.findByRole("heading", { name: "No instructions" })).toBeVisible();
    expect(getCloudProjectInstruction).not.toHaveBeenCalled();
    expect(newCloudProjectSession).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "编辑项目指令" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "以当前版本新建任务" })).not.toBeInTheDocument();
  });

  it("keeps base project detail usable without mounting WorkItems when the verified slice is unavailable", async () => {
    const listCloudProjectWorkItems = vi.fn();
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Identity only")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: false }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Identity only"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction()),
      listCloudProjectWorkItems,
    } as unknown as ProjectGateway;

    renderCloudProjects(gateway);
    await userEvent.click(await screen.findByRole("button", { name: /Identity only/ }));

    expect(await screen.findByRole("heading", { name: "Identity only" })).toBeVisible();
    expect(gateway.getCloudProjectFeatures).toHaveBeenCalledTimes(1);
    expect(listCloudProjectWorkItems).not.toHaveBeenCalled();
    expect(screen.queryByRole("region", { name: /计划项/ })).not.toBeInTheDocument();
  });

  it("lists, server-searches, paginates, selects and refreshes without replacing LocalWorkspace semantics", async () => {
    const listCloudProjects = vi.fn()
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: "page-2", stale: false, cachedAt: null })
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-2", "Beta")], nextCursor: null, stale: false, cachedAt: null })
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-3", "Search result")], nextCursor: null, stale: false, cachedAt: null })
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-3", "Search result", 2)], nextCursor: null, stale: false, cachedAt: null });
    const getCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-3", "Search result", 2), stale: false, cachedAt: null });
    const gateway = {
      listCloudProjects,
      getCloudProject,
    } as unknown as ProjectGateway;

    renderCloudProjects(gateway);
    expect(await screen.findByRole("heading", { name: "我的项目" })).toBeVisible();
    expect(await screen.findByRole("button", { name: /Alpha/ })).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "加载更多云项目" }));
    expect(await screen.findByRole("button", { name: /Beta/ })).toBeVisible();
    expect(listCloudProjects).toHaveBeenNthCalledWith(2, expect.objectContaining({ cursor: "page-2" }));

    const searchbox = screen.getByRole("searchbox", { name: "搜索云项目" });
    expect(screen.queryByRole("button", { name: "搜索云项目" })).not.toBeInTheDocument();
    await userEvent.type(searchbox, "  Search  ");
    expect(listCloudProjects).toHaveBeenCalledTimes(2);
    await userEvent.tab();
    expect(await screen.findByRole("button", { name: /Search result/ })).toBeVisible();
    expect(listCloudProjects).toHaveBeenNthCalledWith(3, expect.objectContaining({ search: "Search" }));

    await userEvent.click(screen.getByRole("button", { name: /Search result/ }));
    expect(getCloudProject).toHaveBeenCalledWith("p-3");

    await userEvent.click(screen.getByRole("button", { name: "项目" }));
    await userEvent.click(screen.getByRole("button", { name: "刷新云项目" }));
    await waitFor(() => expect(listCloudProjects).toHaveBeenCalledTimes(4));
    expect(screen.getByRole("button", { name: /Search result/ })).toHaveTextContent("版本 2");
  });

  it("atomically hides old scoped state and rejects a late response", async () => {
    let resolveA!: (value: unknown) => void;
    const responseA = new Promise((resolve) => { resolveA = resolve; });
    const gateway = {
      listCloudProjects: vi.fn()
        .mockReturnValueOnce(responseA)
        .mockResolvedValueOnce({ projectScopeKey: "opaque-b", items: [project("p-b", "Scope B")], nextCursor: null, stale: false, cachedAt: null }),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway, "scope-a");

    view.rerender(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="scope-b" gateway={gateway} />
      </LocaleProvider>,
    );
    expect(await screen.findByRole("button", { name: /Scope B/ })).toBeVisible();
    resolveA({ projectScopeKey: "opaque-a", items: [project("p-a", "Late Scope A")], nextCursor: null, stale: false, cachedAt: null });
    await waitFor(() => expect(screen.queryByText("Late Scope A")).not.toBeInTheDocument());
    expect(screen.queryByDisplayValue(/.+/)).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps mutation drafts on conflict or unknown result without exposing local binding operations", async () => {
    const onWorkspaceChange = vi.fn();
    const createCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "Created"), stale: false, cachedAt: null });
    const patchCloudProject = vi.fn().mockRejectedValue({ code: "CONFLICT", message: "version mismatch" });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProject: vi.fn().mockImplementation((projectId: string) => Promise.resolve({
        projectScopeKey: "opaque-a",
        project: projectId === "p-2" ? project("p-2", "Created") : project("p-1", "Remote Alpha", 2),
        stale: false,
        cachedAt: null,
      })),
      createCloudProject,
      patchCloudProject,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway, "scope-a", onWorkspaceChange);
    await screen.findByRole("button", { name: /Alpha/ });

    const createTrigger = screen.getByRole("button", { name: "新建云项目" });
    await userEvent.click(createTrigger);
    const createDialog = screen.getByRole("dialog", { name: "新建云项目" });
    const nameInput = within(createDialog).getByLabelText("项目名称");
    expect(nameInput).toHaveFocus();
    const resizeButton = within(createDialog).getByRole("button", { name: "放大弹窗" });
    const createButton = within(createDialog).getByRole("button", { name: "创建" });
    await userEvent.click(createButton);
    expect(await within(createDialog).findByText("请输入项目名称。")).toBeVisible();
    expect(nameInput).toHaveFocus();
    createButton.focus();
    fireEvent.keyDown(createButton, { key: "Tab" });
    expect(resizeButton).toHaveFocus();
    nameInput.focus();
    await userEvent.type(nameInput, "Created");
    await userEvent.click(within(createDialog).getByRole("button", { name: "创建" }));
    expect(createCloudProject).toHaveBeenCalledWith(expect.not.objectContaining({ idempotencyKey: expect.anything() }));
    expect(await screen.findByRole("heading", { name: "Created" })).toBeVisible();
    expect(gateway.getCloudProject).toHaveBeenCalledWith("p-2");
    expect(onWorkspaceChange).toHaveBeenLastCalledWith(true);
    await userEvent.click(screen.getByRole("button", { name: "项目" }));

    await userEvent.click(screen.getByRole("button", { name: /Alpha/ }));
    await userEvent.click(within(await openWorkspaceLifecycleMenu()).getByRole("menuitem", { name: "编辑云项目" }));
    const editDialog = screen.getByRole("dialog", { name: "编辑云项目" });
    const editName = within(editDialog).getByLabelText("项目名称");
    await userEvent.clear(editName);
    await userEvent.type(editName, "My draft");
    await userEvent.click(within(editDialog).getByRole("button", { name: "保存" }));
    expect(await within(editDialog).findByRole("alert")).toHaveTextContent("已保留你的草稿");
    expect(editName).toHaveValue("My draft");

    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("button", { name: "绑定本地目录" })).not.toBeInTheDocument();
  });

  it("keeps a newly created project in the directory when authoritative hydration fails", async () => {
    const onWorkspaceChange = vi.fn();
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-2", "Created"), stale: false, cachedAt: null }),
      getCloudProject: vi.fn().mockRejectedValue(new Error("detail unavailable")),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway, "scope-a", onWorkspaceChange);
    await waitFor(() => expect(view.container.querySelector('[data-project-view="directory"]')).not.toBeNull());

    await userEvent.click(view.container.querySelector<HTMLButtonElement>(".cloud-projects__primary-action")!);
    const dialog = screen.getByRole("dialog");
    await userEvent.type(dialog.querySelector<HTMLInputElement>("input")!, "Created");
    await userEvent.click(dialog.querySelector<HTMLButtonElement>('button[type="submit"]')!);

    await waitFor(() => expect(gateway.getCloudProject).toHaveBeenCalledWith("p-2"));
    expect(view.container.querySelector('[data-project-view="directory"]')).not.toBeNull();
    expect(view.container.querySelector('[data-project-view="workspace"]')).toBeNull();
    expect(screen.getByRole("button", { name: /Created/ })).toBeVisible();
    expect(onWorkspaceChange).not.toHaveBeenCalledWith(true);
  });

  it("does not carry an in-flight Instruction mutation into the next selected project", async () => {
    let resolveEdit!: (value: ReturnType<typeof instruction>) => void;
    const delayedEdit = new Promise<ReturnType<typeof instruction>>((resolve) => { resolveEdit = resolve; });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha"), project("p-2", "Beta")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: false }),
      getCloudProject: vi.fn().mockImplementation((projectId: string) => Promise.resolve({ projectScopeKey: "opaque-a", project: project(projectId, projectId === "p-1" ? "Alpha" : "Beta"), stale: false, cachedAt: null })),
      getCloudProjectInstruction: vi.fn().mockImplementation((projectId: string) => Promise.resolve(instruction(projectId))),
      editCloudProjectInstruction: vi.fn().mockReturnValue(delayedEdit),
    } as unknown as ProjectGateway;

    renderCloudProjects(gateway);
    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));
    await userEvent.click(await screen.findByRole("button", { name: "编辑项目指令" }));
    await userEvent.type(within(screen.getByRole("dialog", { name: "项目指令" })).getByRole("textbox"), " updated");
    await userEvent.click(screen.getByRole("button", { name: "保存项目指令" }));
    await userEvent.click(screen.getByRole("button", { name: "项目" }));
    await userEvent.click(screen.getByRole("button", { name: /Beta/ }));
    expect(await screen.findByRole("button", { name: "编辑项目指令" })).toBeEnabled();

    await act(async () => {
      resolveEdit(instruction("p-1", 4, "Old Alpha response", "Old Alpha agent response"));
      await delayedEdit;
    });
    expect(screen.queryByText("Old Alpha response")).not.toBeInTheDocument();
    expect(screen.getByText("Follow the project rules")).toBeVisible();
    expect(screen.getByRole("button", { name: "编辑项目指令" })).toBeEnabled();
  });

  it("treats timeout as unknown and uses authoritative archive and restore results", async () => {
    const patchCloudProject = vi.fn().mockRejectedValue({ code: "NETWORK_UNAVAILABLE", message: "timeout", retryable: true });
    const archiveCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha", 2, "archived"), stale: false, cachedAt: null });
    const restoreCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha", 3), stale: false, cachedAt: null });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      patchCloudProject,
      archiveCloudProject,
      restoreCloudProject,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);
    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));

    await userEvent.click(within(await openWorkspaceLifecycleMenu()).getByRole("menuitem", { name: "编辑云项目" }));
    const editDialog = screen.getByRole("dialog", { name: "编辑云项目" });
    const draft = within(editDialog).getByLabelText("项目名称");
    await userEvent.clear(draft);
    await userEvent.type(draft, "Unknown draft");
    await userEvent.click(within(editDialog).getByRole("button", { name: "保存" }));
    expect(await within(editDialog).findByRole("alert")).toHaveTextContent("请求结果未知");
    expect(draft).toHaveValue("Unknown draft");
    await userEvent.keyboard("{Escape}");

    await userEvent.click(within(await openWorkspaceLifecycleMenu()).getByRole("menuitem", { name: "归档云项目" }));
    await userEvent.click(within(screen.getByRole("dialog", { name: "归档云项目" })).getByRole("button", { name: "归档" }));
    expect(archiveCloudProject).toHaveBeenCalledWith({ projectId: "p-1", expectedVersion: 1 });
    await userEvent.click(within(await openWorkspaceLifecycleMenu()).getByRole("menuitem", { name: "恢复云项目" }));
    await userEvent.click(within(screen.getByRole("dialog", { name: "恢复云项目" })).getByRole("button", { name: "恢复" }));
    expect(restoreCloudProject).toHaveBeenCalledWith({ projectId: "p-1", expectedVersion: 2 });

    expect(screen.queryByText("绑定本地目录")).not.toBeInTheDocument();
  });

  it("hides a pending dialog, query and draft immediately when the account scope changes", async () => {
    let resolveCreate!: (value: unknown) => void;
    const createResult = new Promise((resolve) => { resolveCreate = resolve; });
    const gateway = {
      listCloudProjects: vi.fn()
        .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-a", "Scope A")], nextCursor: null, stale: false, cachedAt: null })
        .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-a", "Scope A")], nextCursor: null, stale: false, cachedAt: null })
        .mockResolvedValueOnce({ projectScopeKey: "opaque-b", items: [project("p-b", "Scope B")], nextCursor: null, stale: false, cachedAt: null }),
      createCloudProject: vi.fn().mockReturnValue(createResult),
    } as unknown as ProjectGateway;
    const view = renderCloudProjects(gateway, "scope-a");
    await screen.findByRole("button", { name: /Scope A/ });
    await userEvent.type(screen.getByRole("searchbox", { name: "搜索云项目" }), "private query");
    await userEvent.keyboard("{Enter}");
    await waitFor(() => expect(gateway.listCloudProjects).toHaveBeenCalledTimes(2));
    await userEvent.click(screen.getByRole("button", { name: "新建云项目" }));
    const dialog = screen.getByRole("dialog", { name: "新建云项目" });
    await userEvent.type(within(dialog).getByLabelText("项目名称"), "Private draft");
    await userEvent.click(within(dialog).getByRole("button", { name: "创建" }));

    view.rerender(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="scope-b" gateway={gateway} />
      </LocaleProvider>,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue("private query")).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue("Private draft")).not.toBeInTheDocument();
    expect(await screen.findByRole("button", { name: /Scope B/ })).toBeVisible();
    resolveCreate({ projectScopeKey: "opaque-a", project: project("p-a-new", "Late A"), stale: false, cachedAt: null });
    await waitFor(() => expect(screen.queryByText("Late A")).not.toBeInTheDocument());
  });

  it("keeps cached project detail read-only without exposing local binding operations", async () => {
    const cachedAt = "2026-08-05T04:00:00Z";
    const listCloudProjectActivities = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [], nextCursor: null, readCursor: null });
    const listCloudProjectWorkItems = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [], nextCursor: null });
    const listCloudProjectAssets = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [] });
    const listCloudProjectCapabilities = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", projectVersion: 1, items: [] });
    const listCloudProjectMembers = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", currentRole: "owner", items: [] });
    const getCloudProjectInstruction = vi.fn().mockResolvedValue(instruction());
    const getCloudProjectFeatures = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", instructionsAvailable: true, workItemsAvailable: true, membersAvailable: true, activityAvailable: true, workItemFieldsAvailable: true, bindingsAvailable: true, assetsAvailable: true });
    const gateway = {
      listCloudProjects: vi.fn()
        .mockResolvedValueOnce({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha"), project("p-2", "Beta")], nextCursor: null, stale: true, cachedAt })
        .mockRejectedValueOnce({ code: "NETWORK_UNAVAILABLE", message: "offline", retryable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: true, cachedAt }),
      getCloudProjectFeatures,
      getCloudProjectInstruction,
      listCloudProjectCapabilities,
      listCloudProjectMembers,
      listCloudProjectActivities,
      listCloudProjectWorkItems,
      listCloudProjectAssets,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway, "scope-a", undefined, undefined, vi.fn());

    expect(await screen.findByText(/当前显示离线缓存/)).toHaveTextContent("仅供查看");
    expect(screen.getByRole("button", { name: "新建云项目" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: /Alpha/ }));
    const lifecycleMenu = await openWorkspaceLifecycleMenu();
    expect(within(lifecycleMenu).getByRole("menuitem", { name: "编辑云项目" })).toBeDisabled();
    expect(within(lifecycleMenu).getByRole("menuitem", { name: "归档云项目" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "绑定本地目录" })).not.toBeInTheDocument();
    expect(screen.getByText("项目指令必须联网")).toBeVisible();
    expect(screen.getByText("项目动态必须联网")).toBeVisible();
    expect(screen.getByRole("button", { name: "邀请" })).toBeDisabled();
    expect(screen.getByText("成员管理必须联网")).toBeVisible();
    expect(screen.getByRole("button", { name: "添加连接器" })).toBeDisabled();
    expect(getCloudProjectInstruction).not.toHaveBeenCalled();
    expect(listCloudProjectCapabilities).not.toHaveBeenCalled();
    expect(listCloudProjectMembers).not.toHaveBeenCalled();
    expect(listCloudProjectActivities).not.toHaveBeenCalled();
    expect(getCloudProjectFeatures).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "事项" }));
    expect(screen.getByText("事项必须联网")).toBeVisible();
    expect(listCloudProjectWorkItems).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "资产" }));
    expect(screen.getByText("项目资产必须联网")).toBeVisible();
    expect(listCloudProjectAssets).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "项目" }));
    await userEvent.type(screen.getByRole("searchbox", { name: "搜索云项目" }), "Beta");
    await userEvent.keyboard("{Enter}");
    expect(await screen.findByText("云项目加载失败，请稍后重试。")).toBeVisible();
    expect(screen.queryByRole("button", { name: /Beta/ })).not.toBeInTheDocument();
    expect(gateway.listCloudProjects).toHaveBeenLastCalledWith(expect.objectContaining({ search: "Beta" }));
  });

  it("distinguishes unavailable capability from request errors without hiding LocalWorkspace guidance", async () => {
    const unavailable = { listCloudProjects: vi.fn().mockRejectedValue({ code: "CAPABILITY_UNAVAILABLE" }) } as unknown as ProjectGateway;
    const view = renderCloudProjects(unavailable);
    expect(await screen.findByText(/当前云服务版本尚未提供云项目/)).toBeVisible();
    expect(screen.getByText(/本地目录不受影响/)).toBeVisible();

    view.unmount();
    const invalidResponse = { listCloudProjects: vi.fn().mockRejectedValue({ code: "CLOUD_PROTOCOL_ERROR" }) } as unknown as ProjectGateway;
    renderCloudProjects(invalidResponse);
    expect(await screen.findByText("云项目加载失败，请稍后重试。")).toBeVisible();
  });

  it("keeps instruction metadata out of the workspace while archived projects stay read-only", async () => {
    const hostile = `<img src=x onerror=alert(1)> [remote](https://example.invalid/x) javascript:alert(1)`;
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Archived", 2, "archived")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Archived", 2, "archived"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction("p-1", 7, hostile, "tool: shell")),
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);

    await userEvent.click(await screen.findByRole("button", { name: /Archived/ }));
    expect(await screen.findByText("tool: shell")).toBeVisible();
    expect(screen.queryByText("来源：云端项目指令")).not.toBeInTheDocument();
    expect(screen.queryByText("指令版本 7")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "编辑项目指令" }));
    const dialog = screen.getByRole("dialog", { name: "项目指令" });
    expect(within(dialog).queryByText("为之后创建的项目任务提供背景、边界和输出要求。")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("来源：云端项目指令")).not.toBeInTheDocument();
    expect(dialog.querySelector(".cloud-project-dialog__field--instruction")).not.toBeNull();
    expect(within(dialog).getByText(/\d+ \/ 10,000/)).toBeVisible();
    expect(screen.getByLabelText("项目说明")).toHaveTextContent(hostile);
    expect(within(dialog).getByRole("textbox")).toHaveValue("tool: shell");
    expect(within(dialog).getByRole("textbox")).toHaveAttribute("readonly");
    expect(within(dialog).getAllByRole("textbox")).toHaveLength(1);
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText(/项目已归档，指令仅供查看/)).toBeVisible();
    expect(screen.getByRole("button", { name: "保存项目指令" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "以当前版本新建任务" })).toBeDisabled();
  });

  it("edits, explicitly clears and starts a new session without claiming empty instruction injection", async () => {
    const editCloudProjectInstruction = vi.fn().mockResolvedValue(instruction("p-1", 4, "Updated", "New agent rule"));
    const clearCloudProjectInstruction = vi.fn().mockResolvedValue(instruction("p-1", 5, "", ""));
    const newCloudProjectSession = vi.fn().mockResolvedValue({ tabId: "tab-prepared", sessionPath: "session.jsonl", projectId: "p-1", instructionVersion: 5, contentHash: "a".repeat(64), reused: false });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue(instruction()),
      editCloudProjectInstruction,
      clearCloudProjectInstruction,
      newCloudProjectSession,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway, "scope-a", undefined, ({ onCreateSession }) => <button type="button" aria-label="以当前版本新建任务" onClick={() => void onCreateSession("tab-prepared", "D:\\workspace")}>start</button>);
    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));
    await waitFor(() => expect(gateway.getCloudProjectInstruction).toHaveBeenCalledWith("p-1"));

    await userEvent.click(screen.getByRole("button", { name: "编辑项目指令" }));
    const dialog = screen.getByRole("dialog", { name: "项目指令" });
    const summary = screen.getByLabelText("项目说明");
    const agentText = within(dialog).getByRole("textbox");
    expect(summary).toHaveTextContent("Project context");
    expect(within(dialog).getAllByRole("textbox")).toHaveLength(1);
    await userEvent.clear(agentText);
    await userEvent.type(agentText, "New agent rule");
    await userEvent.click(screen.getByRole("button", { name: "保存项目指令" }));
    expect(editCloudProjectInstruction).toHaveBeenCalledWith({ projectId: "p-1", expectedVersion: 3, humanDescription: "Project context", agentInstruction: "New agent rule" });
    expect(await screen.findByText("项目指令已保存并完成权威回读。")).toBeVisible();
    expect(screen.queryByText(/只会用于之后新建的任务/)).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "编辑项目指令" }));
    await userEvent.click(screen.getByRole("button", { name: "清空项目指令" }));
    expect(clearCloudProjectInstruction).not.toHaveBeenCalled();
    expect(screen.queryByText(/清空后当前页面无法恢复原内容/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "确认清空项目指令" }));
    expect(clearCloudProjectInstruction).toHaveBeenCalledWith({ projectId: "p-1", expectedVersion: 4 });
    await userEvent.click(screen.getByRole("button", { name: "以当前版本新建任务" }));
    expect(newCloudProjectSession).toHaveBeenCalledWith({ projectId: "p-1", tabId: "tab-prepared", workspacePath: "D:\\workspace" });
    expect(await screen.findByText(/指令为空，未添加项目指令上下文/)).toBeVisible();
  });

  it("keeps the instruction draft on conflict and refreshes the authoritative version", async () => {
    const getCloudProjectInstruction = vi.fn()
      .mockResolvedValueOnce(instruction("p-1", 3, "Original", "Original rule"))
      .mockResolvedValueOnce(instruction("p-1", 4, "Remote", "Remote rule"));
    const editCloudProjectInstruction = vi.fn()
      .mockRejectedValueOnce({ code: "CONFLICT", message: "version mismatch" })
      .mockResolvedValueOnce(instruction("p-1", 5, "Original", "My draft rule"));
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", items: [project("p-1", "Alpha")], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", project: project("p-1", "Alpha"), stale: false, cachedAt: null }),
      getCloudProjectInstruction,
      editCloudProjectInstruction,
    } as unknown as ProjectGateway;
    renderCloudProjects(gateway);
    await userEvent.click(await screen.findByRole("button", { name: /Alpha/ }));
    await userEvent.click(await screen.findByRole("button", { name: "编辑项目指令" }));
    const dialog = screen.getByRole("dialog", { name: "项目指令" });
    const summary = screen.getByLabelText("项目说明");
    const agentText = within(dialog).getByRole("textbox");
    expect(summary).toHaveTextContent("Original");
    await userEvent.clear(agentText);
    await userEvent.type(agentText, "My draft rule");
    await userEvent.click(screen.getByRole("button", { name: "保存项目指令" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("已保留你的草稿");
    expect(summary).toHaveTextContent("Original");
    expect(agentText).toHaveValue("My draft rule");
    expect(screen.queryByText("指令版本 4")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "保存项目指令" }));
    expect(editCloudProjectInstruction).toHaveBeenLastCalledWith(expect.objectContaining({ expectedVersion: 4, humanDescription: "Original", agentInstruction: "My draft rule" }));
  });
});
