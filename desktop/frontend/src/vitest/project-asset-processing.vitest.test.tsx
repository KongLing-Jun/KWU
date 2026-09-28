import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import { ProjectAssetSelection } from "../features/projects/ui/ProjectAssetSelection";
import { ProjectAssetsPanel } from "../features/projects/ui/ProjectAssetsPanel";

const asset = {
  assetId: "asset-1",
  projectId: "project-1",
  filename: "evidence.txt",
  contentType: "text/plain",
  sizeBytes: 12,
  sha256: "a".repeat(64),
  version: 2,
  workItemIds: [],
  createdAt: "2026-08-10T00:00:00Z",
  updatedAt: "2026-08-10T01:00:00Z",
};

const directoryAsset = {
  kind: "asset" as const,
  id: "asset-1",
  name: "evidence.txt",
  folderId: null,
  contentType: "text/plain",
  sizeBytes: 12,
  version: 2,
  updatedAt: "2026-08-10T01:00:00Z",
};

const directoryFolder = {
  kind: "folder" as const,
  id: "folder-1",
  name: "Design",
  parentFolderId: null,
  version: 1,
  updatedAt: "2026-08-10T01:00:00Z",
};

function directoryPage(overrides: Record<string, unknown> = {}) {
  return {
    projectScopeKey: "scope-1",
    projectId: "project-1",
    breadcrumbs: [],
    items: [directoryFolder, directoryAsset],
    totalCount: 2,
    nextCursor: null,
    usageBytes: 12,
    quotaBytes: 5 * 1024 * 1024 * 1024,
    quotaPlan: "lite",
    ...overrides,
  };
}

function directoryGateway(list = vi.fn().mockResolvedValue(directoryPage())) {
  return {
    listCloudProjectAssetDirectory: list,
    listCloudProjectAssetProcessingStates: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [] }),
    listCloudProjectAssetOperationRecords: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [], limit: 100 }),
    uploadCloudProjectAsset: vi.fn(),
    downloadCloudProjectAsset: vi.fn(),
    deleteCloudProjectAsset: vi.fn(),
    createCloudProjectAssetFolder: vi.fn(),
    patchCloudProjectAssetFolder: vi.fn(),
    deleteCloudProjectAssetFolder: vi.fn(),
    patchCloudProjectAsset: vi.fn(),
    replaceCloudProjectAssetContent: vi.fn(),
  } as unknown as ProjectGateway;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

describe("project asset processing state", () => {
  it("keeps the legacy asset list visible when processing state is unavailable", async () => {
    const gateway = {
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [asset] }),
      listCloudProjectAssetProcessingStates: vi.fn().mockRejectedValue(new Error("not deployed")),
    } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);

    expect(await screen.findByText("evidence.txt")).toBeVisible();
    expect(await screen.findByText("处理状态暂不可用，文件列表仍可正常使用。")).toBeVisible();
  });

  it("shows the latest processing state in the asset actions menu", async () => {
    const gateway = {
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [asset] }),
      listCloudProjectAssetProcessingStates: vi.fn().mockResolvedValue({
        projectScopeKey: "scope-1",
        projectId: "project-1",
        items: [{
          assetId: "asset-1", assetVersionId: "asset-version-1", versionNumber: 1,
          scanStatus: "rejected", parseStatus: "not_started", indexStatus: "not_started", purgeStatus: "active",
          stateVersion: 2, scanAttempts: 1,
          createdAt: "2026-08-09T00:00:00Z", updatedAt: "2026-08-09T01:00:00Z",
          scanCompletedAt: null, purgedAt: null,
        }, {
          assetId: "asset-1", assetVersionId: "asset-version-2", versionNumber: 2,
          scanStatus: "clean", parseStatus: "ready", indexStatus: "processing", purgeStatus: "active",
          stateVersion: 3, scanAttempts: 1,
          createdAt: "2026-08-10T00:00:00Z", updatedAt: "2026-08-10T01:00:00Z",
          scanCompletedAt: "2026-08-10T00:30:00Z", purgedAt: null,
        }],
      }),
    } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);

    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("button", { name: "evidence.txt 操作" }));
    expect(screen.getByRole("status")).toHaveTextContent("扫描 安全 · 解析 就绪");
    expect(screen.queryByText(/已拒绝/)).not.toBeInTheDocument();
  });

  it("removes stale processing state when a refresh cannot read current state", async () => {
    const processing = {
      projectScopeKey: "scope-1",
      projectId: "project-1",
      items: [{
        assetId: "asset-1", assetVersionId: "asset-version-2", versionNumber: 2,
        scanStatus: "clean", parseStatus: "ready", indexStatus: "processing", purgeStatus: "active",
        stateVersion: 3, scanAttempts: 1,
        createdAt: "2026-08-10T00:00:00Z", updatedAt: "2026-08-10T01:00:00Z",
        scanCompletedAt: "2026-08-10T00:30:00Z", purgedAt: null,
      }],
    };
    const gateway = {
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [asset] }),
      listCloudProjectAssetProcessingStates: vi.fn().mockResolvedValueOnce(processing).mockRejectedValueOnce(new Error("offline")),
    } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("button", { name: "evidence.txt 操作" }));
    expect(screen.getByRole("status")).toHaveTextContent("扫描 安全 · 解析 就绪");

    await userEvent.click(screen.getByRole("columnheader", { name: /名称/ }));
    expect(await screen.findByText("处理状态暂不可用，文件列表仍可正常使用。")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "evidence.txt 操作" }));
    expect(screen.queryByText(/扫描 安全/)).not.toBeInTheDocument();
  });

  it("keeps archived project assets read-only while preserving downloads", async () => {
    const gateway = {
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [asset] }),
      listCloudProjectAssetProcessingStates: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [] }),
    } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly />);

    const row = (await screen.findByText("evidence.txt")).closest('[role="row"]') as HTMLElement | null;
    expect(row).not.toBeNull();
    expect(screen.getByRole("button", { name: "新建文件夹" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "上传文件" })).toBeDisabled();
    await userEvent.click(within(row!).getByRole("button", { name: "evidence.txt 操作" }));
    expect(within(row!).getByRole("menuitem", { name: "下载" })).toBeEnabled();
    expect(within(row!).getByRole("menuitem", { name: "重命名" })).toBeDisabled();
    expect(within(row!).getByRole("menuitem", { name: "移动" })).toBeDisabled();
    expect(within(row!).getByRole("menuitem", { name: "删除" })).toBeDisabled();
  });

  it("does not let an older project request replace the newly selected project", async () => {
    const firstAssets = deferred<{ projectScopeKey: string; projectId: string; items: typeof asset[] }>();
    const firstProcessing = deferred<{ projectScopeKey: string; projectId: string; items: [] }>();
    const secondAsset = { ...asset, assetId: "asset-2", projectId: "project-2", filename: "current-project.txt" };
    const gateway = {
      listCloudProjectAssets: vi.fn((projectId: string) => projectId === "project-1"
        ? firstAssets.promise
        : Promise.resolve({ projectScopeKey: "scope-2", projectId: "project-2", items: [secondAsset] })),
      listCloudProjectAssetProcessingStates: vi.fn((input: { projectId: string }) => input.projectId === "project-1"
        ? firstProcessing.promise
        : Promise.resolve({ projectScopeKey: "scope-2", projectId: "project-2", items: [] })),
    } as unknown as ProjectGateway;

    const view = render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await waitFor(() => expect(gateway.listCloudProjectAssets).toHaveBeenCalledWith("project-1"));

    view.rerender(<ProjectAssetsPanel projectId="project-2" projectScopeKey="scope-2" gateway={gateway} readOnly={false} />);
    expect(await screen.findByText("current-project.txt")).toBeVisible();

    await act(async () => {
      firstAssets.resolve({ projectScopeKey: "scope-1", projectId: "project-1", items: [asset] });
      firstProcessing.resolve({ projectScopeKey: "scope-1", projectId: "project-1", items: [] });
      await firstAssets.promise;
    });

    expect(screen.getByText("current-project.txt")).toBeVisible();
    expect(screen.queryByText("evidence.txt")).not.toBeInTheDocument();
  });

  it("gives compact asset actions stable accessible names", async () => {
    const gateway = {
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [asset] }),
      listCloudProjectAssetProcessingStates: vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [] }),
    } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);

    const row = (await screen.findByText("evidence.txt")).closest('[role="row"]') as HTMLElement | null;
    expect(row).not.toBeNull();
    expect(screen.getByRole("button", { name: "上传文件" })).toBeVisible();
    expect(screen.getByRole("searchbox", { name: "搜索文件或文件夹" })).toBeVisible();
    expect(screen.getByRole("button", { name: "文件类型：全部类型" })).toBeVisible();
    expect(within(row!).getByRole("checkbox", { name: "选择 evidence.txt" })).toBeVisible();
    expect(within(row!).getByRole("button", { name: "evidence.txt 操作" })).toBeVisible();
  });
});

describe("project asset directory interactions", () => {
  it("shows who changed an asset and filters operation records", async () => {
    const listRecords = vi.fn().mockResolvedValue({
      projectScopeKey: "scope-1",
      projectId: "project-1",
      limit: 100,
      items: [{
        operationId: "assetop-1", projectId: "project-1", resourceKind: "asset", resourceId: "asset-1",
        resourceName: "项目方案.pdf", operation: "update", operationDetail: "replace_content",
        actorUserId: "user-1", actorDisplayName: "张三", resourceVersion: 3, folderId: "folder-1",
        details: { asset_version_count: 3 }, createdAt: "2026-08-22T08:30:00Z",
      }],
    });
    const gateway = { ...directoryGateway(), listCloudProjectAssetOperationRecords: listRecords } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("button", { name: "操作记录" }));

    expect(await screen.findByRole("heading", { name: "资产操作记录" })).toBeVisible();
    expect(screen.getByText("张三")).toBeVisible();
    expect(screen.getByText("替换了文件内容")).toBeVisible();
    expect(screen.getByText("项目方案.pdf")).toBeVisible();
    expect(screen.getByText(/版本 3/)).toBeVisible();
    expect(listRecords).toHaveBeenLastCalledWith({ projectId: "project-1", operation: undefined, resourceKind: undefined, limit: 100 });

    await userEvent.click(screen.getByRole("button", { name: "操作类型：全部操作" }));
    await userEvent.click(screen.getByRole("option", { name: "删除" }));
    await waitFor(() => expect(listRecords).toHaveBeenLastCalledWith({ projectId: "project-1", operation: "delete", resourceKind: undefined, limit: 100 }));
  });

  it("downloads asset bytes into the global workspace and reports the saved path", async () => {
    const gateway = directoryGateway();
    const download = vi.mocked(gateway.downloadCloudProjectAsset);
    download.mockResolvedValue({
      projectScopeKey: "scope-1",
      projectId: "project-1",
      assetId: "asset-1",
      filename: "evidence.txt",
      url: "https://assets.example/evidence.txt",
      expiresAt: "2026-08-10T02:00:00Z",
      savedPath: `C:\\Reasonix\\global-workspace\\evidence.txt`,
    });

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);

    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("button", { name: "evidence.txt 操作" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "下载" }));

    expect(download).toHaveBeenCalledWith("project-1", "asset-1");
    expect(await screen.findByText(`已下载到 C:\\Reasonix\\global-workspace\\evidence.txt`)).toHaveAttribute("role", "status");
  });

  it("replaces file content as a versioned mutation in the current project folder", async () => {
    const nestedAsset = { ...directoryAsset, id: "nested-1", name: "nested.txt", folderId: "folder-1" };
    const list = vi.fn((input: { folderId?: string }) => Promise.resolve(input.folderId === "folder-1"
      ? directoryPage({ breadcrumbs: [{ folderId: "folder-1", name: "Design" }], items: [nestedAsset], totalCount: 1 })
      : directoryPage()));
    const replace = vi.fn().mockResolvedValue(directoryPage());
    const gateway = { ...directoryGateway(list), replaceCloudProjectAssetContent: replace } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await screen.findByText("Design");
    await userEvent.click(screen.getByRole("cell", { name: "Design" }));
    await screen.findByText("nested.txt");
    await userEvent.click(screen.getByRole("button", { name: "nested.txt 操作" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "替换内容" }));

    expect(replace).toHaveBeenCalledWith({ projectId: "project-1", assetId: "nested-1", expectedVersion: 2, folderId: "folder-1" });
    await waitFor(() => expect(list.mock.calls.filter(([input]) => input.folderId === "folder-1").length).toBeGreaterThanOrEqual(2));
  });

  it("keeps pagination scoped to the current directory while navigating", async () => {
    const list = vi.fn((input: { folderId?: string; cursor?: string }) => {
      if (input.folderId === "folder-1" && input.cursor === "folder-next") {
        return Promise.resolve(directoryPage({
          breadcrumbs: [{ folderId: "folder-1", name: "Design" }],
          items: [{ ...directoryAsset, id: "nested-2", name: "nested-2.txt", folderId: "folder-1" }],
          totalCount: 2,
        }));
      }
      if (input.folderId === "folder-1") {
        return Promise.resolve(directoryPage({
          breadcrumbs: [{ folderId: "folder-1", name: "Design" }],
          items: [{ ...directoryAsset, id: "nested-1", name: "nested-1.txt", folderId: "folder-1" }],
          totalCount: 2,
          nextCursor: "folder-next",
        }));
      }
      if (input.cursor === "root-next") {
        return Promise.resolve(directoryPage({
          items: [{ ...directoryAsset, id: "root-2", name: "root-2.txt" }],
          totalCount: 3,
        }));
      }
      return Promise.resolve(directoryPage({ totalCount: 3, nextCursor: "root-next" }));
    });
    const gateway = directoryGateway(list);

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);

    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("cell", { name: "Design" }));
    expect(await screen.findByText("nested-1.txt")).toBeVisible();
    expect(screen.queryByText("evidence.txt")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "加载更多（1/2）" }));
    expect(await screen.findByText("nested-2.txt")).toBeVisible();
    expect(list).toHaveBeenCalledWith(expect.objectContaining({ folderId: "folder-1", cursor: "folder-next", limit: 50 }));

    await userEvent.click(screen.getByRole("button", { name: "资产" }));
    expect(await screen.findByText("evidence.txt")).toBeVisible();
    expect(screen.queryByText("nested-2.txt")).not.toBeInTheDocument();
    expect(screen.queryByText("root-2.txt")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "加载更多（2/3）" }));
    expect(await screen.findByText("root-2.txt")).toBeVisible();
    expect(list).toHaveBeenCalledWith(expect.objectContaining({ cursor: "root-next", limit: 50 }));
  });

  it("applies search only on blur or Enter", async () => {
    const list = vi.fn().mockResolvedValue(directoryPage({ items: [], totalCount: 0 }));
    const gateway = directoryGateway(list);
    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(1));

    const search = screen.getByRole("searchbox", { name: "搜索文件或文件夹" });
    await userEvent.type(search, "report");
    expect(list).toHaveBeenCalledTimes(1);

    fireEvent.blur(search);
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ search: "report" })));

    await userEvent.clear(search);
    await userEvent.type(search, "budget{Enter}");
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ search: "budget" })));
  });

  it("does not submit search when Enter is part of IME composition", async () => {
    const list = vi.fn().mockResolvedValue(directoryPage({ items: [], totalCount: 0 }));
    const gateway = directoryGateway(list);
    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(1));

    const search = screen.getByRole("searchbox", { name: "搜索文件或文件夹" });
    fireEvent.compositionStart(search);
    fireEvent.change(search, { target: { value: "设计" } });
    fireEvent.keyDown(search, { key: "Enter", code: "Enter", isComposing: true });
    await act(async () => Promise.resolve());
    expect(list).toHaveBeenCalledTimes(1);

    fireEvent.compositionEnd(search);
    fireEvent.blur(search);
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ search: "设计" })));
  });

  it("sends type and sort changes as directory queries", async () => {
    const list = vi.fn().mockResolvedValue(directoryPage({ items: [], totalCount: 0 }));
    const gateway = directoryGateway(list);
    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(1));

    await userEvent.click(screen.getByRole("button", { name: "文件类型：全部类型" }));
    await userEvent.click(screen.getByRole("option", { name: "PDF" }));
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ type: "pdf", sort: "updated_at", direction: "desc" })));

    await userEvent.click(screen.getByRole("columnheader", { name: /名称/ }));
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ type: "pdf", sort: "name", direction: "asc" })));
    await userEvent.click(screen.getByRole("columnheader", { name: /名称/ }));
    await waitFor(() => expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ type: "pdf", sort: "name", direction: "desc" })));
  });

  it("tracks individual and select-all state through accessible checkboxes", async () => {
    const gateway = directoryGateway();
    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await screen.findByText("evidence.txt");

    const first = screen.getByRole("checkbox", { name: "选择 evidence.txt" });
    const second = screen.getByRole("checkbox", { name: "选择 Design" });
    const selectAll = screen.getByRole("checkbox", { name: "选择全部文件" });
    await userEvent.click(first);
    expect(screen.getByRole("status")).toHaveTextContent("已选择 1 项");
    expect(first).toBeChecked();

    await userEvent.click(second);
    expect(screen.getByRole("status")).toHaveTextContent("已选择 2 项");
    expect(selectAll).toBeChecked();

    await userEvent.click(screen.getByRole("button", { name: "清除选择" }));
    expect(screen.queryByText(/已选择/)).not.toBeInTheDocument();
    expect(first).not.toBeChecked();
    expect(second).not.toBeChecked();
  });

  it("shows Cloud quota and exposes the upgrade callback", async () => {
    const onUpgrade = vi.fn();
    const list = vi.fn().mockResolvedValue(directoryPage({
      items: [],
      totalCount: 0,
      usageBytes: 1024 * 1024 * 1024,
      quotaBytes: 5 * 1024 * 1024 * 1024,
      quotaPlan: "pro",
    }));
    const gateway = directoryGateway(list);
    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} onUpgrade={onUpgrade} />);

    expect(await screen.findByText(/存储空间已用 1 GB \/ 5 GB/)).toBeVisible();
    expect(screen.getByLabelText("当前 PRO 套餐存储额度")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "升级存储空间" }));
    expect(onUpgrade).toHaveBeenCalledTimes(1);
  });

  it("does not invent a Lite quota when Cloud has no authoritative quota", async () => {
    const gateway = directoryGateway(vi.fn().mockResolvedValue(directoryPage({ items: [], totalCount: 0, quotaBytes: 0, quotaPlan: "" })));
    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);

    expect(await screen.findByText(/额度暂不可用/)).toBeVisible();
    expect(screen.queryByText(/5 GB/)).not.toBeInTheDocument();
  });

  it("requests processing state only for assets visible in the directory", async () => {
    const processing = vi.fn().mockResolvedValue({ projectScopeKey: "scope-1", projectId: "project-1", items: [] });
    const gateway = { ...directoryGateway(), listCloudProjectAssetProcessingStates: processing } as unknown as ProjectGateway;
    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);

    await screen.findByText("evidence.txt");
    await waitFor(() => expect(processing).toHaveBeenCalledWith({ projectId: "project-1", assetIds: ["asset-1"] }));
  });

  it("does not refresh a new project when an old mutation finishes late", async () => {
    const pending = deferred<unknown>();
    const list = vi.fn(async (input: { projectId: string }) => directoryPage({
      projectScopeKey: input.projectId === "project-1" ? "scope-1" : "scope-2",
      projectId: input.projectId,
      items: input.projectId === "project-1" ? [directoryAsset] : [{ ...directoryAsset, id: "asset-2", name: "current-project.txt" }],
      totalCount: 1,
    }));
    const gateway = {
      ...directoryGateway(list),
      createCloudProjectAssetFolder: vi.fn(() => pending.promise),
      listCloudProjectAssetProcessingStates: vi.fn(async (input: { projectId: string }) => ({ projectScopeKey: input.projectId === "project-1" ? "scope-1" : "scope-2", projectId: input.projectId, items: [] })),
    } as unknown as ProjectGateway;
    const view = render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("button", { name: "新建文件夹" }));
    const dialog = screen.getByRole("dialog", { name: "新建文件夹" });
    await userEvent.type(within(dialog).getByLabelText("名称"), "Late");
    await userEvent.click(within(dialog).getByRole("button", { name: "确定" }));

    view.rerender(<ProjectAssetsPanel projectId="project-2" projectScopeKey="scope-2" gateway={gateway} readOnly={false} />);
    expect(await screen.findByText("current-project.txt")).toBeVisible();
    const callsBeforeResolution = list.mock.calls.length;
    pending.resolve({});
    await act(async () => Promise.resolve());
    expect(list).toHaveBeenCalledTimes(callsBeforeResolution);
    expect(screen.getByText("current-project.txt")).toBeVisible();
  });

  it("reloads the authoritative asset after a version conflict", async () => {
    const authoritative = { ...directoryAsset, name: "cloud-authoritative.txt", version: 3 };
    const list = vi.fn()
      .mockResolvedValueOnce(directoryPage())
      .mockResolvedValue(directoryPage({ items: [directoryFolder, authoritative] }));
    const patchAsset = vi.fn().mockRejectedValue({ code: "CONFLICT", message: "version mismatch" });
    const gateway = {
      ...directoryGateway(list),
      patchCloudProjectAsset: patchAsset,
    } as unknown as ProjectGateway;

    render(<ProjectAssetsPanel projectId="project-1" projectScopeKey="scope-1" gateway={gateway} readOnly={false} />);
    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("button", { name: "evidence.txt 操作" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "重命名" }));
    const dialog = screen.getByRole("dialog", { name: "重命名" });
    const name = within(dialog).getByLabelText("名称");
    await userEvent.clear(name);
    await userEvent.type(name, "my-stale-name.txt");
    await userEvent.click(within(dialog).getByRole("button", { name: "确定" }));

    expect(patchAsset).toHaveBeenCalledWith({
      projectId: "project-1",
      assetId: "asset-1",
      expectedVersion: 2,
      name: "my-stale-name.txt",
    });
    expect(await screen.findByText("cloud-authoritative.txt")).toBeVisible();
    expect(screen.queryByText("evidence.txt")).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("资产已被其他成员更新，已刷新为云端最新版本。");
    expect(list).toHaveBeenCalledTimes(2);
  });
});

describe("project asset selection", () => {
  it("navigates directories and keeps later asset pages selectable", async () => {
    const nestedAsset = { ...directoryAsset, id: "nested-1", name: "nested.txt", folderId: "folder-1" };
    const laterAsset = { ...directoryAsset, id: "asset-2", name: "later.txt" };
    const list = vi.fn((input: { folderId?: string; cursor?: string }) => {
      if (input.folderId === "folder-1") {
        return Promise.resolve(directoryPage({
          breadcrumbs: [{ folderId: "folder-1", name: "Design" }],
          items: [nestedAsset],
          totalCount: 1,
        }));
      }
      if (input.cursor === "next-page") {
        return Promise.resolve(directoryPage({ items: [laterAsset], totalCount: 3 }));
      }
      return Promise.resolve(directoryPage({ nextCursor: "next-page", totalCount: 3 }));
    });
    const gateway = directoryGateway(list);
    const onChange = vi.fn();

    render(<ProjectAssetSelection
      projectId="project-1"
      projectScopeKey="scope-1"
      gateway={gateway}
      selectedIds={[]}
      onChange={onChange}
    />);

    await screen.findByText("evidence.txt");
    await userEvent.click(screen.getByRole("button", { name: "加载更多资产" }));
    const later = await screen.findByRole("checkbox", { name: "later.txt" });
    await userEvent.click(later);
    expect(onChange).toHaveBeenCalledWith(["asset-2"]);
    expect(list).toHaveBeenCalledWith(expect.objectContaining({ cursor: "next-page", limit: 50 }));

    await userEvent.click(screen.getByRole("button", { name: /Design/ }));
    expect(await screen.findByRole("checkbox", { name: "nested.txt" })).toBeVisible();
    expect(screen.queryByRole("checkbox", { name: "later.txt" })).not.toBeInTheDocument();
    expect(list).toHaveBeenCalledWith(expect.objectContaining({ folderId: "folder-1", limit: 50 }));
  });
});
