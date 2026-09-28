import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "../lib/i18n";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import { CloudProjectsSection } from "../features/projects/ui/CloudProjectsSection";
const project = {
  projectId: "p-13",
  name: "Goal 13",
  description: null,
  status: "active" as const,
  version: 1,
  createdAt: "2026-08-06T00:00:00Z",
  updatedAt: "2026-08-06T00:00:00Z",
};

function gateway(): ProjectGateway {
  return {
    listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "scope-13", items: [project], nextCursor: null, stale: false, cachedAt: null }),
    getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "scope-13", project, stale: false, cachedAt: null }),
    getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "scope-13", instructionsAvailable: true, workItemsAvailable: false }),
    getCloudProjectInstruction: vi.fn().mockResolvedValue({ projectScopeKey: "scope-13", projectId: project.projectId, humanDescription: "", agentInstruction: "Use project context", contentHash: "a".repeat(64), version: 1, createdAt: project.createdAt, updatedAt: project.updatedAt }),
    newCloudProjectSession: vi.fn().mockResolvedValue({ tabId: "tab-prepared", sessionPath: "goal-13.json", projectId: project.projectId, instructionVersion: 1, contentHash: "a".repeat(64), reused: false }),
  } as unknown as ProjectGateway;
}

describe("Goal 13 project Composer integration", () => {
  it("renders the supplied production Composer and routes three capability cards without cloud mutation", async () => {
    const projectGateway = gateway();
    const onOpenCapability = vi.fn();
    const onRestoreFocusConsumed = vi.fn();
    const onProjectComposerLeave = vi.fn();
    const renderComposer = vi.fn(({ sessionKey, projectName }: { sessionKey: string; projectName: string }) => <textarea id="composer-input" data-session-key={sessionKey} data-project-name={projectName} />);
    const view = render(
      <LocaleProvider>
        <CloudProjectsSection
          authenticated
          accountScopeKey="account-13"
          gateway={projectGateway}
          renderComposer={renderComposer}
          onOpenCapability={onOpenCapability}
          onRestoreFocusConsumed={onRestoreFocusConsumed}
          onProjectComposerLeave={onProjectComposerLeave}
        />
      </LocaleProvider>,
    );

    await waitFor(() => expect(view.container.querySelector(".cloud-projects__card")).toBeTruthy());
    await userEvent.click(view.container.querySelector(".cloud-projects__card") as HTMLButtonElement);

    expect(await waitFor(() => view.container.querySelector("#composer-input"))).toHaveAttribute("data-session-key", JSON.stringify(["project-composer-v1", "account-13", "p-13"]));
    expect(view.container.querySelector("#composer-input")).toHaveAttribute("data-project-name", "Goal 13");
    expect(view.container.querySelector(".cloud-projects__composer")).toHaveAttribute("data-project-composer-reused", "true");
    const actions = view.container.querySelectorAll<HTMLButtonElement>("[data-project-capability]");
    expect(actions).toHaveLength(3);
    await userEvent.click(actions[0]);
    expect(onOpenCapability).toHaveBeenCalledWith("connector", expect.anything(), expect.objectContaining({ sessionKey: JSON.stringify(["project-composer-v1", "account-13", "p-13"]), projectId: "p-13" }));
    expect(projectGateway.newCloudProjectSession).not.toHaveBeenCalled();

    view.rerender(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="account-13" gateway={projectGateway} renderComposer={renderComposer} onOpenCapability={onOpenCapability} restoreFocusCapability="connector" onRestoreFocusConsumed={onRestoreFocusConsumed} onProjectComposerLeave={onProjectComposerLeave} />
      </LocaleProvider>,
    );
    await waitFor(() => expect(onRestoreFocusConsumed).toHaveBeenCalled());
    expect(view.container.querySelector<HTMLButtonElement>('[data-project-capability="connector"]')).toHaveFocus();
    expect(view.container.querySelector(".cloud-projects__back")).toBeNull();
    await userEvent.click(view.container.querySelector<HTMLButtonElement>(".cloud-projects__project-link")!);
    expect(onProjectComposerLeave).toHaveBeenCalledWith(JSON.stringify(["project-composer-v1", "account-13", "p-13"]));
  });

  it("pins the authoritative project session once before the shared Composer send", async () => {
    const projectGateway = gateway();
    const renderComposer = ({ onCreateSession }: { onCreateSession: (tabId: string, workspacePath?: string) => Promise<unknown> }) => (
      <button type="button" data-project-send onClick={() => void onCreateSession("tab-prepared")}>send</button>
    );
    const view = render(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="account-13" gateway={projectGateway} renderComposer={renderComposer} />
      </LocaleProvider>,
    );
    await waitFor(() => expect(view.container.querySelector(".cloud-projects__card")).toBeTruthy());
    await userEvent.click(view.container.querySelector(".cloud-projects__card") as HTMLButtonElement);
    await userEvent.click(await waitFor(() => view.container.querySelector("[data-project-send]") as HTMLButtonElement));
    await userEvent.click(view.container.querySelector("[data-project-send]") as HTMLButtonElement);
    await waitFor(() => expect(projectGateway.newCloudProjectSession).toHaveBeenCalledTimes(1));
    expect(projectGateway.newCloudProjectSession).toHaveBeenCalledWith({ projectId: "p-13", tabId: "tab-prepared", workspacePath: "" });
  });

  it("refreshes the Composer project name from the authoritative rename result", async () => {
    const projectGateway = gateway();
    const renamed = { ...project, name: "Goal 13 Next", version: 2 };
    projectGateway.patchCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "scope-13", project: renamed, stale: false, cachedAt: null });
    const renderComposer = vi.fn(({ projectName }: { projectName: string }) => <span data-current-project-name>{projectName}</span>);
    const view = render(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="account-13" gateway={projectGateway} renderComposer={renderComposer} />
      </LocaleProvider>,
    );

    await waitFor(() => expect(view.container.querySelector(".cloud-projects__card")).toBeTruthy());
    await userEvent.click(view.container.querySelector(".cloud-projects__card") as HTMLButtonElement);
    expect(await screen.findByText("Goal 13", { selector: "[data-current-project-name]" })).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "项目操作" }));
    await userEvent.click(within(screen.getByRole("menu")).getByRole("menuitem", { name: "编辑云项目" }));
    const dialog = screen.getByRole("dialog", { name: "编辑云项目" });
    const name = within(dialog).getByLabelText("项目名称");
    await userEvent.clear(name);
    await userEvent.type(name, renamed.name);
    await userEvent.click(within(dialog).getByRole("button", { name: "保存" }));

    expect(await screen.findByText(renamed.name, { selector: "[data-current-project-name]" })).toBeVisible();
    expect(renderComposer).toHaveBeenLastCalledWith(expect.objectContaining({ projectId: project.projectId, projectName: renamed.name }));
  });

  it("keeps project sessions out of the project workspace navigation", async () => {
    const view = render(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="account-13" gateway={gateway()} renderComposer={() => <textarea aria-label="项目输入" />} />
      </LocaleProvider>,
    );

    await waitFor(() => expect(view.container.querySelector(".cloud-projects__card")).toBeTruthy());
    await userEvent.click(view.container.querySelector(".cloud-projects__card") as HTMLButtonElement);
    expect(screen.queryByRole("button", { name: "执行会话" })).not.toBeInTheDocument();
    expect(view.container.querySelector("[data-project-conversation]")).toBeNull();
  });

  it("disables the production Composer contract before an archived project can create a session", async () => {
    const archived = { ...project, status: "archived" as const };
    const projectGateway = gateway();
    projectGateway.listCloudProjects = vi.fn().mockResolvedValue({ projectScopeKey: "scope-13", items: [archived], nextCursor: null, stale: false, cachedAt: null });
    projectGateway.getCloudProject = vi.fn().mockResolvedValue({ projectScopeKey: "scope-13", project: archived, stale: false, cachedAt: null });
    const renderComposer = vi.fn(({ projectWritable, projectDisabledReason }: { projectWritable: boolean; projectDisabledReason?: string }) => (
      <button type="button" data-project-send disabled={!projectWritable} title={projectDisabledReason}>send</button>
    ));
    const view = render(
      <LocaleProvider>
        <CloudProjectsSection authenticated accountScopeKey="account-13" gateway={projectGateway} renderComposer={renderComposer} />
      </LocaleProvider>,
    );

    await waitFor(() => expect(view.container.querySelector(".cloud-projects__card")).toBeTruthy());
    await userEvent.click(view.container.querySelector(".cloud-projects__card") as HTMLButtonElement);
    await waitFor(() => expect(view.container.querySelector("[data-project-send]")).toBeTruthy());
    const send = view.container.querySelector<HTMLButtonElement>("[data-project-send]");
    expect(send).toBeDisabled();
    expect(send?.title).toBeTruthy();
    expect(projectGateway.newCloudProjectSession).not.toHaveBeenCalled();
  });
});
