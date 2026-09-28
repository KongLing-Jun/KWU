import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ArtifactLibraryPage } from "../features/artifacts/ArtifactLibraryPage";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { ArtifactView } from "../lib/types";

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function artifact(name: string): ArtifactView {
  return {
    name,
    path: `reports/${name}`,
    extension: ".xlsx",
    type: "spreadsheet",
    size: 100,
    modifiedAt: 1,
    deliveredAt: 1,
    workspaceName: name,
    session: {
      path: `sessions/${name}.jsonl`,
      preview: "来源会话",
      turns: 1,
      createdAt: 1,
      lastActivityAt: 1,
      modTime: 1,
      current: false,
      open: false,
      workspaceRoot: "C:/workspace",
    },
  };
}

function installBridge(listArtifacts: AppBindings["ListArtifacts"]) {
  window.go = {
    main: { App: { ListArtifacts: listArtifacts } as Partial<AppBindings> as AppBindings },
  };
}

function page(accountScopeKey = "account-a", refreshSignal = 0) {
  return (
    <LocaleProvider>
      <ArtifactLibraryPage accountScopeKey={accountScopeKey} refreshSignal={refreshSignal} onOpenSource={vi.fn()} />
    </LocaleProvider>
  );
}

describe("ArtifactLibraryPage visibility invalidation", () => {
  it("reloads project changes, clears search results immediately, and keeps the view and type filter", async () => {
    const local = artifact("本地文件.xlsx");
    const revoked = artifact("退出项目文件.xlsx");
    const reload = deferred<ArtifactView[]>();
    const listArtifacts = vi.fn<AppBindings["ListArtifacts"]>()
      .mockResolvedValueOnce([local, revoked])
      .mockReturnValueOnce(reload.promise);
    installBridge(listArtifacts);
    const { rerender } = render(page());
    await screen.findByRole("button", { name: revoked.name });
    fireEvent.click(screen.getByRole("button", { name: "本地产物" }));
    fireEvent.change(screen.getByLabelText("文件类型"), { target: { value: "spreadsheet" } });
    fireEvent.click(screen.getByRole("button", { name: "搜索" }));
    const search = screen.getByRole("dialog");
    expect(within(search).getByText(revoked.name)).toBeInTheDocument();

    rerender(page("account-a", 1));

    expect(listArtifacts).toHaveBeenCalledTimes(2);
    expect(screen.queryAllByText(revoked.name)).toHaveLength(0);
    expect(within(search).queryByText(local.name)).not.toBeInTheDocument();
    await act(async () => { reload.resolve([local]); });
    expect(screen.getAllByText(local.name).length).toBeGreaterThan(0);
    expect(screen.queryAllByText(revoked.name)).toHaveLength(0);
    expect(screen.getByRole("button", { name: "本地产物" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByLabelText("文件类型")).toHaveValue("spreadsheet");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("reloads account switches and logout without suppressing local artifacts", async () => {
    const local = artifact("本地文件.xlsx");
    const accountA = artifact("账号甲文件.xlsx");
    const accountB = artifact("账号乙文件.xlsx");
    const switched = deferred<ArtifactView[]>();
    const loggedOut = deferred<ArtifactView[]>();
    const listArtifacts = vi.fn<AppBindings["ListArtifacts"]>()
      .mockResolvedValueOnce([local, accountA])
      .mockReturnValueOnce(switched.promise)
      .mockReturnValueOnce(loggedOut.promise);
    installBridge(listArtifacts);
    const { rerender } = render(page());
    await screen.findByRole("button", { name: accountA.name });

    rerender(page("account-b"));
    expect(screen.queryByText(accountA.name)).not.toBeInTheDocument();
    await act(async () => { switched.resolve([local, accountB]); });
    expect(screen.getByRole("button", { name: accountB.name })).toBeInTheDocument();

    rerender(page(""));
    expect(screen.queryByText(accountB.name)).not.toBeInTheDocument();
    await act(async () => { loggedOut.resolve([local]); });
    expect(listArtifacts).toHaveBeenCalledTimes(3);
    expect(screen.getByRole("button", { name: local.name })).toBeInTheDocument();
    expect(screen.queryByText(accountA.name)).not.toBeInTheDocument();
    expect(screen.queryByText(accountB.name)).not.toBeInTheDocument();
  });

  it.each(["success", "failure"] as const)("ignores a late %s from the previous account", async (outcome) => {
    const stale = artifact("旧账号文件.xlsx");
    const current = artifact("当前账号文件.xlsx");
    const oldRequest = deferred<ArtifactView[]>();
    const newRequest = deferred<ArtifactView[]>();
    const listArtifacts = vi.fn<AppBindings["ListArtifacts"]>()
      .mockReturnValueOnce(oldRequest.promise)
      .mockReturnValueOnce(newRequest.promise);
    installBridge(listArtifacts);
    const { rerender } = render(page());

    rerender(page("account-b"));
    await act(async () => { newRequest.resolve([current]); });
    await act(async () => {
      if (outcome === "success") oldRequest.resolve([stale]);
      else oldRequest.reject(new Error("old account request failed"));
    });

    expect(listArtifacts).toHaveBeenCalledTimes(2);
    expect(screen.getByRole("button", { name: current.name })).toBeInTheDocument();
    expect(screen.queryByText(stale.name)).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "刷新" })).toBeEnabled();
  });
});
