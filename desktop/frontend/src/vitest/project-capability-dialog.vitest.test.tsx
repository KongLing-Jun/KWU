import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import { ProjectCapabilityDialog, projectCapabilityOptionDisabled } from "../features/projects/ui/ProjectCapabilityDialog";
import { ProjectCapabilityIcon } from "../features/projects/ui/ProjectCapabilityIcon";

const appMock = vi.hoisted(() => ({
  ListProjectConnectorAvailability: vi.fn(), SkillsSettings: vi.fn(), PublicSkills: vi.fn(),
  InstallPublicSkill: vi.fn(),
}));
vi.mock("../lib/bridge", () => ({ app: appMock }));

function capabilityGateway(overrides: Partial<ProjectGateway> = {}): ProjectGateway {
  return {
    listProjectConnectorAvailability: () => appMock.ListProjectConnectorAvailability(),
    listProjectSkillsSettings: () => appMock.SkillsSettings(),
    listProjectPublicSkills: (limit, cursor) => appMock.PublicSkills(limit, cursor),
    installProjectPublicSkill: (namespace, slug) => appMock.InstallPublicSkill(namespace, slug),
    ...overrides,
  } as ProjectGateway;
}

describe("ProjectCapabilityDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    appMock.InstallPublicSkill.mockResolvedValue({ installed: true });
    appMock.ListProjectConnectorAvailability.mockResolvedValue([{ registryRef: "github", name: "GitHub MCP", description: "GitHub 仓库", compatible: true, localServerName: "github", available: true, availabilityReason: "" }]);
  });

  it("loads existing cloud bindings and saves connector selection without leaving Project", async () => {
    const save = vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 5, items: [{ bindingId: "b-1", kind: "connector", registryRef: "GitHub MCP", position: 0, config: {}, publicDisabled: false, version: 1 }] });
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 4, items: [] }),
      saveCloudProjectCapabilities: save,
    });
    const close = vi.fn();
    render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={close} onApplied={vi.fn()} />);
    fireEvent.click(await screen.findByRole("tab", { name: "全部连接器" }));
    fireEvent.click(await screen.findByRole("button", { name: /GitHub MCP/ }));
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(save).toHaveBeenCalledWith({
      projectId: "p-1",
      expectedVersion: 4,
      bindings: [{ kind: "connector", registryRef: "github", position: 0, config: {} }],
    }));
    expect(close).toHaveBeenCalled();
  });

  it("saves an uninstalled connector to Cloud without installing or authorizing it", async () => {
    const saved = [{ bindingId: "b-connector", kind: "connector" as const, registryRef: "notion", position: 0, config: {}, publicDisabled: false, version: 1 }];
    const save = vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 5, items: saved });
    const install = vi.fn().mockResolvedValue({ connector_id: "notion", status: "authorization_required" });
    const onApplied = vi.fn();
    appMock.ListProjectConnectorAvailability.mockResolvedValue([{
      registryRef: "notion", name: "Notion", description: "Knowledge base", compatible: true,
      localServerName: "", localStatus: "not_installed", available: false, availabilityReason: "尚未连接",
    }]);
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 4, items: [] }),
      saveCloudProjectCapabilities: save,
      installProjectConnector: install,
    });

    render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={onApplied} />);
    fireEvent.click(await screen.findByRole("tab", { name: "全部连接器" }));
    const option = await screen.findByRole("button", { name: /Notion/ });
    expect(option).toBeEnabled();
    fireEvent.click(option);
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(install).not.toHaveBeenCalled();
    expect(onApplied).toHaveBeenCalledWith(saved);
  });

  it("echoes existing experts and saves multiple experts without dropping other binding kinds", async () => {
    const existing = [
      { bindingId: "b-expert-1", kind: "expert" as const, registryRef: "exp_architect", position: 0, config: { role: "lead" }, publicDisabled: false, version: 1 },
      { bindingId: "b-expert-2", kind: "expert" as const, registryRef: "exp_reviewer", position: 1, config: {}, publicDisabled: false, version: 1 },
      { bindingId: "b-skill", kind: "skill" as const, registryRef: "@reasonix/planning", position: 0, config: { enabled: true }, publicDisabled: false, version: 1 },
    ];
    const saved = [
      ...existing,
      { bindingId: "b-expert-3", kind: "expert" as const, registryRef: "exp_auditor", position: 2, config: {}, publicDisabled: false, version: 1 },
    ];
    const save = vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 8, items: saved });
    const onApplied = vi.fn();
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 7, items: existing }),
      saveCloudProjectCapabilities: save,
    });
    const experts = [
      { id: "exp_architect", name: "架构顾问", meta: "专家库" },
      { id: "exp_reviewer", name: "代码审查专家", meta: "专家库" },
      { id: "exp_auditor", name: "审计风控专家", meta: "专家库" },
    ];

    render(<ProjectCapabilityDialog kind="expert" projectId="p-1" gateway={gateway} experts={experts} onClose={vi.fn()} onApplied={onApplied} />);

    expect(await screen.findByText("已选择 2 项")).toBeVisible();
    expect(screen.getByRole("button", { name: /架构顾问/ })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: /代码审查专家/ })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: /审计风控专家/ }));
    expect(screen.getByText("已选择 3 项")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    await waitFor(() => expect(save).toHaveBeenCalledWith({
      projectId: "p-1",
      expectedVersion: 7,
      bindings: [
        { kind: "skill", registryRef: "@reasonix/planning", position: 0, config: { enabled: true } },
        { kind: "expert", registryRef: "exp_architect", position: 0, config: { role: "lead" } },
        { kind: "expert", registryRef: "exp_reviewer", position: 1, config: {} },
        { kind: "expert", registryRef: "exp_auditor", position: 2, config: {} },
      ],
    }));
    expect(onApplied).toHaveBeenCalledWith(saved);
  });

  it("keeps a failed save selection and explains missing project permission", async () => {
    const save = vi.fn().mockRejectedValue({ code: "PERMISSION_DENIED" });
    const close = vi.fn();
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 4, items: [] }),
      saveCloudProjectCapabilities: save,
    });
    render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={close} onApplied={vi.fn()} />);

    fireEvent.click(await screen.findByRole("tab", { name: "全部连接器" }));
    const option = await screen.findByRole("button", { name: /GitHub MCP/ });
    fireEvent.click(option);
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("你当前没有配置项目连接器的权限。当前选择已保留。");
    expect(option).toHaveAttribute("aria-pressed", "true");
    expect(close).not.toHaveBeenCalled();
  });

  it("retries an image after its icon URL changes", () => {
    const view = render(<ProjectCapabilityIcon kind="expert" iconUrl="https://example.test/old.png" />);
    fireEvent.error(view.container.querySelector("img")!);
    expect(view.container.querySelector("img")).toBeNull();

    view.rerender(<ProjectCapabilityIcon kind="expert" iconUrl="https://example.test/new.png" />);
    expect(view.container.querySelector("img")).toHaveAttribute("src", "https://example.test/new.png");
  });

  it("blocks unavailable additions while allowing an existing unavailable binding to be removed", async () => {
    const unavailable = { id: "connector-offline", name: "Offline connector", available: false };
    expect(projectCapabilityOptionDisabled(unavailable, false)).toBe(true);
    expect(projectCapabilityOptionDisabled({ ...unavailable, installable: true }, false)).toBe(false);
    expect(projectCapabilityOptionDisabled(unavailable, true)).toBe(false);

    appMock.ListProjectConnectorAvailability.mockResolvedValue([{
      registryRef: "connector-offline", name: "Offline connector", description: "Needs local setup",
      compatible: false, localServerName: "Offline connector", available: false, availabilityReason: "当前客户端版本不兼容",
    }]);
    const save = vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 5, items: [] });
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 4, items: [] }),
      saveCloudProjectCapabilities: save,
    });
    render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);
    fireEvent.click(await screen.findByRole("tab", { name: "全部连接器" }));
    const option = await screen.findByRole("button", { name: /Offline connector/ });
    expect(option).toBeDisabled();
    expect(option).toHaveAttribute("title", "当前客户端版本不兼容");
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(save).toHaveBeenCalledWith({ projectId: "p-1", expectedVersion: 4, bindings: [] }));
  });

	it("exposes public connectors, my experts, installed skills, and SkillHub as working source filters", async () => {
	  appMock.ListProjectConnectorAvailability.mockResolvedValue([
		{ registryRef: "github", name: "GitHub MCP", description: "GitHub", compatible: true, localServerName: "GitHub MCP", available: true, availabilityReason: "" },
		{ registryRef: "notion", name: "Notion", description: "Notion", compatible: true, localServerName: "", available: false, availabilityReason: "尚未连接" },
	  ]);
	  const capability = vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 1, items: [] });
	  const gateway = capabilityGateway({ listCloudProjectCapabilities: capability, saveCloudProjectCapabilities: vi.fn() });
	  const connectorView = render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);
	  expect(await screen.findByText("当前项目还没有配置连接器。可切换到“全部连接器”添加。")).toBeVisible();
	  fireEvent.click(screen.getByRole("tab", { name: "全部连接器" }));
	  expect(await screen.findByRole("button", { name: /GitHub MCP/ })).toBeVisible();
	  expect(screen.getByRole("button", { name: /Notion/ })).toBeEnabled();
	  connectorView.unmount();

	  const experts = [
		{ id: "expert-public", name: "Architect", meta: "专家库", iconUrl: "data:image/png;base64,avatar" },
		{ id: "expert-mine", name: "My reviewer", meta: "我的专家" },
	  ];
	  const expertView = render(<ProjectCapabilityDialog kind="expert" projectId="p-1" gateway={gateway} experts={experts} onClose={vi.fn()} onApplied={vi.fn()} />);
	  const architectOption = await screen.findByRole("button", { name: /Architect/ });
	  expect(architectOption).toBeVisible();
	  expect(architectOption.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,avatar");
	  expect(architectOption.querySelector("img")).toHaveClass("project-capability-icon__image");
	  expect(screen.queryByRole("button", { name: /My reviewer/ })).not.toBeInTheDocument();
	  fireEvent.click(screen.getByRole("tab", { name: "我的专家" }));
	  expect(await screen.findByRole("button", { name: /My reviewer/ })).toBeVisible();
	  expertView.unmount();

	  appMock.SkillsSettings.mockResolvedValue({ skills: [{ name: "installed", uninstallId: "installed", enabled: true }], skillRoots: [] });
	  appMock.PublicSkills.mockResolvedValue({ items: [
		{ name: "Installed", namespace: "reasonix", slug: "installed", description: "Installed skill" },
		{ name: "Remote", namespace: "reasonix", slug: "remote", description: "Remote skill" },
	  ], next_cursor: "", has_more: false });
	  render(<ProjectCapabilityDialog kind="skill" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);
	  await screen.findByRole("button", { name: /Installed/ });
	  fireEvent.click(screen.getByRole("tab", { name: "已安装" }));
	  expect(screen.getByRole("button", { name: /Installed/ })).toBeVisible();
	  expect(screen.queryByRole("button", { name: /Remote/ })).not.toBeInTheDocument();
	  fireEvent.click(screen.getByRole("tab", { name: "SkillHub" }));
	  expect(screen.getByRole("button", { name: /Remote/ })).toBeEnabled();
	});

  it("distinguishes a source with no items from a search with no matches", async () => {
    appMock.ListProjectConnectorAvailability.mockResolvedValue([]);
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 1, items: [] }),
      saveCloudProjectCapabilities: vi.fn(),
    });
    render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);

    expect(await screen.findByText("当前项目还没有配置连接器。可切换到“全部连接器”添加。")).toBeVisible();
    fireEvent.change(screen.getByPlaceholderText("搜索连接器"), { target: { value: "github" } });
    expect(screen.getByText("没有匹配“github”的连接器，请调整搜索关键词。")).toBeVisible();
    fireEvent.change(screen.getByPlaceholderText("搜索连接器"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("tab", { name: "全部连接器" }));
    expect(screen.getByText("Cloud Catalog 当前没有可用连接器，请稍后重试。")).toBeVisible();
  });

	it("saves an unconnected project connector without coupling connection management", async () => {
	  appMock.ListProjectConnectorAvailability.mockResolvedValue([{ registryRef: "notion", name: "Notion", description: "Notion", compatible: true, localServerName: "", available: false, availabilityReason: "尚未连接" }]);
	  const save = vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 2, items: [] });
	  const gateway = capabilityGateway({
		listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 1, items: [] }),
		saveCloudProjectCapabilities: save,
	  });
	  render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);
	  fireEvent.click(await screen.findByRole("tab", { name: "全部连接器" }));
	  const option = await screen.findByRole("button", { name: /Notion/ });
	  fireEvent.click(option);
	  expect(screen.queryByRole("button", { name: "保存并逐个连接" })).not.toBeInTheDocument();
	  expect(screen.getByText("连接器可以先保存到项目；无论是否已连接，都不会自动出现在对话输入框。")).toBeVisible();
	  fireEvent.click(screen.getByRole("button", { name: "保存" }));
	  await waitFor(() => expect(save).toHaveBeenCalledWith({
		projectId: "p-1",
		expectedVersion: 1,
		bindings: [{ kind: "connector", registryRef: "notion", position: 0, config: {} }],
	  }));
	});

	it("saves multiple project connectors while preserving other binding kinds", async () => {
	  appMock.ListProjectConnectorAvailability.mockResolvedValue([
		{ registryRef: "github", name: "GitHub MCP", description: "GitHub", compatible: true, localServerName: "github", available: true, availabilityReason: "" },
		{ registryRef: "notion", name: "Notion", description: "Notion", compatible: true, localServerName: "", available: false, availabilityReason: "尚未连接" },
	  ]);
	  const existing = [{ bindingId: "b-skill", kind: "skill" as const, registryRef: "planning", position: 0, config: { mode: "safe" }, publicDisabled: false, version: 1 }];
	  const save = vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 3, items: existing });
	  const gateway = capabilityGateway({
		listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 2, items: existing }),
		saveCloudProjectCapabilities: save,
	  });
	  render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);
	  fireEvent.click(await screen.findByRole("tab", { name: "全部连接器" }));
	  fireEvent.click(await screen.findByRole("button", { name: /GitHub MCP/ }));
	  fireEvent.click(screen.getByRole("button", { name: /Notion/ }));
	  expect(screen.getByText("已选择 2 项")).toBeVisible();
	  fireEvent.click(screen.getByRole("button", { name: "保存" }));
	  await waitFor(() => expect(save).toHaveBeenCalledWith({
		projectId: "p-1",
		expectedVersion: 2,
		bindings: [
		  { kind: "skill", registryRef: "planning", position: 0, config: { mode: "safe" } },
		  { kind: "connector", registryRef: "github", position: 0, config: {} },
		  { kind: "connector", registryRef: "notion", position: 1, config: {} },
		],
	  }));
	});

  it("shows an actionable permission error without also rendering a generic empty state", async () => {
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockRejectedValue({ code: "PERMISSION_DENIED" }),
      saveCloudProjectCapabilities: vi.fn(),
    });
    render(<ProjectCapabilityDialog kind="connector" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);

    expect(await screen.findByRole("alert")).toHaveTextContent("你没有查看或配置当前项目连接器的权限。");
    expect(screen.queryByText(/没有可用的连接器/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
  });

  it("explains empty installed-skill sources", async () => {
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 1, items: [] }),
      saveCloudProjectCapabilities: vi.fn(),
    });
    appMock.SkillsSettings.mockResolvedValue({ skills: [], skillRoots: [] });
    appMock.PublicSkills.mockResolvedValue({ items: [], next_cursor: "", has_more: false });
    const skillView = render(<ProjectCapabilityDialog kind="skill" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);

    await screen.findByText("当前没有推荐技能。请稍后重试。");
    fireEvent.click(screen.getByRole("tab", { name: "已安装" }));
    expect(screen.getByText("当前设备没有已安装的技能。请先安装技能，再返回此处选择。")).toBeVisible();
    fireEvent.click(screen.getByRole("tab", { name: "SkillHub" }));
    expect(screen.getByText("SkillHub 当前没有可用技能。请检查网络后重试。")).toBeVisible();
    skillView.unmount();

  });

  it("keeps installed skills available when SkillHub is temporarily unavailable", async () => {
    appMock.SkillsSettings.mockResolvedValue({
      skills: [{ name: "local-review", displayName: "本机评审", description: "本机已安装技能", uninstallId: "@reasonix/local-review", enabled: true }],
      skillRoots: [],
    });
    appMock.PublicSkills.mockRejectedValue(new Error("SkillHub unavailable"));
    render(<ProjectCapabilityDialog kind="skill" projectId="" gateway={capabilityGateway()} experts={[]} mode="draft" onClose={vi.fn()} onDraftApplied={vi.fn()} />);

    fireEvent.click(await screen.findByRole("tab", { name: "已安装" }));
    expect(await screen.findByRole("button", { name: /本机评审/ })).toBeVisible();
    expect(screen.queryByText("技能加载失败。请关闭窗口后重试。")).not.toBeInTheDocument();
  });

  it("keeps multiple experts selected while preparing a new project", async () => {
    const onDraftApplied = vi.fn();
    const experts = [
      { id: "exp_architect", name: "架构顾问", meta: "专家库" },
      { id: "exp_reviewer", name: "代码审查专家", meta: "专家库" },
    ];
    render(<ProjectCapabilityDialog kind="expert" projectId="" gateway={capabilityGateway()} experts={experts} mode="draft" onClose={vi.fn()} onDraftApplied={onDraftApplied} />);

    fireEvent.click(await screen.findByRole("button", { name: /架构顾问/ }));
    fireEvent.click(screen.getByRole("button", { name: /代码审查专家/ }));

    expect(screen.getByText("已选择 2 项")).toBeVisible();
    expect(screen.getByRole("button", { name: /架构顾问/ })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: /代码审查专家/ })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "确定" }));
    expect(onDraftApplied).toHaveBeenCalledWith([
      expect.objectContaining(experts[0]),
      expect.objectContaining(experts[1]),
    ]);
  });

  it("keeps multiple skills selected while preparing a new project", async () => {
    appMock.SkillsSettings.mockResolvedValue({ skills: [], skillRoots: [] });
    appMock.PublicSkills.mockResolvedValue({
      items: [
        { name: "Planning", namespace: "reasonix", slug: "planning", description: "Plan delivery" },
        { name: "Review", namespace: "reasonix", slug: "review", description: "Review delivery" },
      ],
      next_cursor: "",
      has_more: false,
    });
    const onDraftApplied = vi.fn();
    render(<ProjectCapabilityDialog kind="skill" projectId="" gateway={capabilityGateway()} experts={[]} mode="draft" onClose={vi.fn()} onDraftApplied={onDraftApplied} />);

    fireEvent.click(await screen.findByRole("button", { name: /Planning/ }));
    fireEvent.click(screen.getByRole("button", { name: /Review/ }));

    expect(screen.getByText("已选择 2 项")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "确定" }));
    expect(onDraftApplied).toHaveBeenCalledWith([
      expect.objectContaining({ id: "@reasonix/planning" }),
      expect.objectContaining({ id: "@reasonix/review" }),
    ]);
  });

  it("saves the cloud registry identity for a project skill instead of a local install name", async () => {
    appMock.SkillsSettings.mockResolvedValue({ skills: [], skillRoots: [] });
    appMock.PublicSkills.mockResolvedValue({ items: [{ name: "Planning", namespace: "reasonix", slug: "planning", description: "Plan delivery" }], next_cursor: "", has_more: false });
    const save = vi.fn().mockResolvedValue({
      projectScopeKey: "scope", projectId: "p-1", projectVersion: 3,
      items: [{ bindingId: "b-skill", kind: "skill", registryRef: "@reasonix/planning", position: 0, config: {}, publicDisabled: false, version: 1 }],
    });
    const gateway = capabilityGateway({
      listCloudProjectCapabilities: vi.fn().mockResolvedValue({ projectScopeKey: "scope", projectId: "p-1", projectVersion: 2, items: [] }),
      saveCloudProjectCapabilities: save,
    });
    render(<ProjectCapabilityDialog kind="skill" projectId="p-1" gateway={gateway} experts={[]} onClose={vi.fn()} onApplied={vi.fn()} />);
    const option = await screen.findByRole("button", { name: /Planning/ });
    expect(option).toBeEnabled();
    fireEvent.click(option);
    fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(save).toHaveBeenCalledWith({
      projectId: "p-1",
      expectedVersion: 2,
      bindings: [{ kind: "skill", registryRef: "@reasonix/planning", position: 0, config: {} }],
    }));
    expect(appMock.InstallPublicSkill).toHaveBeenCalledWith("reasonix", "planning");
  });
});
