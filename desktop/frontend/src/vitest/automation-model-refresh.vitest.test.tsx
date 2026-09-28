import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { HeartbeatPanel } from "../features/automation/heartbeat/HeartbeatPanel";
import { useAutomationModels } from "../features/automation/heartbeat/useAutomationModels";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { ModelInfo, SharedModelCatalogState } from "../lib/types";

const models: ModelInfo[] = [{
  ref: "kwu/deepseek-v4-flash", provider: "airclaude_subscription", model: "deepseek-v4-flash",
  billingSource: "subscription_pool", current: true,
}];
const access = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" } as const;
const catalog: SharedModelCatalogState = { state: "ready", routes: models, marketplace: { sources: [] } };
const loadingText = "正在加载模型…";
const unavailableText = "当前模型不可用，请选择其他模型或检查模型设置。";
const events = new Map<string, Set<(...args: unknown[]) => void>>();
const loadModels = vi.fn<(...args: string[]) => Promise<ModelInfo[]>>();
const resolveCapabilities = vi.fn(async (_task: unknown) => ({ riskSummary: "" }));
const createTask = vi.fn(async (task: unknown) => ({
  ...(task as object), id: "saved", revision: 1, createdAt: "2026-09-08T00:00:00Z",
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

beforeEach(() => {
  events.clear();
  loadModels.mockReset().mockResolvedValue(models);
  resolveCapabilities.mockReset().mockResolvedValue({ riskSummary: "" });
  createTask.mockClear();
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", { configurable: true, value: vi.fn() });
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: () => ({ matches: false, addEventListener() {}, removeEventListener() {} }),
  });
  window.runtime = {
    EventsOn: (name, callback) => {
      const listeners = events.get(name) ?? new Set();
      events.set(name, listeners);
      listeners.add(callback);
      return () => { listeners.delete(callback); };
    },
    BrowserOpenURL() {},
  };
  window.go = { main: { App: {
    AutomationModels: loadModels,
    AutomationResolveCapabilities: resolveCapabilities,
    AutomationCreateTask: createTask,
    AutomationReloadTasks: async () => [],
    AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 0, unlimited: true, canCreate: true, pausedTaskIds: [] }),
    AutomationListRuns: async () => [],
    AutomationListTemplates: async () => [],
    AutomationGenerateID: async () => "draft",
    AutomationGetScheduleCatalog: async () => [],
    AutomationListDeliveryTargets: async () => [],
    AutomationListCapabilities: async () => ({ models: [], skills: [], experts: [], connectors: [] }),
    ListWorkspaces: async () => [],
    SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
    ListExperts: async () => [],
    ListUserExperts: async () => [],
    ProjectTreeRevision: async () => 0,
    ListCloudProjects: async () => ({ items: [], projectScopeKey: "test", nextCursor: null, stale: false, cachedAt: null }),
    ListJoinedCloudProjects: async () => ({ items: [], projectScopeKey: "test", nextCursor: null, stale: false, cachedAt: null }),
  } as Partial<AppBindings> as AppBindings } };
});

function editor(tick = 0, account = "profile/account/1", settingsRevision = 0) {
  return <LocaleProvider>
    <span>Running task update {tick}</span>
    <HeartbeatPanel open embedded startNew authenticated
      modelCatalog={{ ...catalog }} subscriptionAccess={{ ...access }}
      modelAccountScopeKey={account} modelSettingsRevision={settingsRevision}
      onClose={() => {}} onOpenTopic={() => {}} />
  </LocaleProvider>;
}

async function openEditor() {
  await screen.findByRole("button", { name: "添加" });
  await waitFor(() => expect(screen.queryByText(loadingText)).not.toBeInTheDocument());
}

function fillDraft() {
  fireEvent.change(screen.getByLabelText("名称"), { target: { value: "Automation" } });
  fireEvent.change(screen.getByLabelText("提示词"), { target: { value: "Inspect status" } });
}

describe("automation model editor", () => {
  it.each([
    { name: "valid default", list: models },
    { name: "empty result", list: [] },
    { name: "missing default", list: models.map((model) => ({ ...model, current: false })) },
  ])("keeps model feedback stable across focus and trailing refresh: $name", async ({ list }) => {
    loadModels.mockResolvedValue(list);
    const view = render(editor());
    await openEditor();
    const picker = screen.getByRole("button", { name: "模型" });
    const label = picker.textContent;
    const feedback = screen.queryByText(unavailableText);
    const first = deferred<ModelInfo[]>();
    const trailing = deferred<ModelInfo[]>();
    loadModels.mockReturnValueOnce(first.promise).mockReturnValueOnce(trailing.promise);
    act(() => {
      window.dispatchEvent(new Event("blur"));
      window.dispatchEvent(new Event("focus"));
      window.dispatchEvent(new Event("focus"));
    });
    for (let tick = 1; tick <= 5; tick++) {
      await act(async () => view.rerender(editor(tick)));
    }
    const expectStable = () => {
      expect(picker.textContent).toBe(label);
      expect(screen.queryByText(loadingText)).not.toBeInTheDocument();
      expect(screen.queryByText(unavailableText)).toBe(feedback);
    };
    expectStable();
    fillDraft();
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    expect(resolveCapabilities).not.toHaveBeenCalled();
    await act(async () => first.resolve(list));
    expect(loadModels).toHaveBeenCalledTimes(3);
    expectStable();
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    expect(resolveCapabilities).not.toHaveBeenCalled();
    await act(async () => trailing.resolve(list));
    expectStable();
  });

  it("keeps a first-load error visible through focus retry until a successful empty result", async () => {
    loadModels.mockRejectedValueOnce(new Error("offline"));
    render(editor());
    const error = await screen.findByText("模型加载失败，请重试。");
    const retry = deferred<ModelInfo[]>();
    loadModels.mockReturnValueOnce(retry.promise);
    act(() => window.dispatchEvent(new Event("focus")));
    expect(screen.getByText("模型加载失败，请重试。")).toBe(error);
    expect(screen.queryByText(loadingText)).not.toBeInTheDocument();
    fillDraft();
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    expect(resolveCapabilities).not.toHaveBeenCalled();
    await act(async () => retry.resolve([]));
    expect(screen.queryByText("模型加载失败，请重试。")).not.toBeInTheDocument();
    expect(screen.getByText(unavailableText)).toBeInTheDocument();
  });

  it("preserves a refresh failure and validation styling until a valid retry completes", async () => {
    render(editor());
    await openEditor();
    fillDraft();
    loadModels.mockRejectedValueOnce(new Error("offline"));
    await act(async () => window.dispatchEvent(new Event("focus")));
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    const feedback = screen.getByText("模型加载失败，请重试。").parentElement!;
    const markup = feedback.outerHTML;
    expect(feedback).toHaveAttribute("role", "alert");
    expect(resolveCapabilities).not.toHaveBeenCalled();
    const retry = deferred<ModelInfo[]>();
    loadModels.mockReturnValueOnce(retry.promise);
    act(() => window.dispatchEvent(new Event("focus")));
    expect(feedback.outerHTML).toBe(markup);
    expect(screen.queryByText(loadingText)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "模型" })).toHaveAttribute("aria-busy", "true");
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    expect(resolveCapabilities).not.toHaveBeenCalled();
    await act(async () => retry.resolve(models));
    expect(screen.queryByText("模型加载失败，请重试。")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    await waitFor(() => expect(createTask).toHaveBeenCalledTimes(1));
  });

  it.each([true, false])("ignores running-task rerenders with a current default: %s", async (current) => {
    loadModels.mockResolvedValue(models.map((model) => ({ ...model, current })));
    const view = render(editor());
    await openEditor();
    expect(loadModels).toHaveBeenCalledTimes(1);
    const statuses: string[] = [];
    const observer = new MutationObserver(() => {
      if (screen.queryByText(loadingText)) statuses.push("loading");
    });
    observer.observe(view.container, { subtree: true, childList: true, characterData: true });
    for (let tick = 1; tick <= 20; tick++) {
      await act(async () => view.rerender(editor(tick)));
    }
    observer.disconnect();
    expect(loadModels).toHaveBeenCalledTimes(1);
    expect(statuses).toEqual([]);
    if (!current) {
      expect(screen.getByText(unavailableText)).toBeInTheDocument();
      expect(createTask).not.toHaveBeenCalled();
      return;
    }
    fillDraft();
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    await waitFor(() => expect(createTask).toHaveBeenCalledTimes(1));
    expect(createTask).toHaveBeenCalledWith(expect.objectContaining({
      prompt: expect.objectContaining({ model: models[0].ref, billingSource: "subscription_pool" }),
    }));
  });

  it("retains a backend model error across unrelated updates and focus refresh", async () => {
    resolveCapabilities.mockRejectedValue({ code: "AUTOMATION_MODEL_UNAVAILABLE" });
    const view = render(editor());
    await openEditor();
    fillDraft();
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    await screen.findByText(unavailableText);
    await waitFor(() => expect(loadModels).toHaveBeenCalledTimes(2));
    await act(async () => view.rerender(editor(1)));
    expect(loadModels).toHaveBeenCalledTimes(2);
    await act(async () => window.dispatchEvent(new Event("focus")));
    expect(screen.getByText(unavailableText)).toBeInTheDocument();
    expect(screen.getByLabelText("名称")).toHaveValue("Automation");
    expect(createTask).not.toHaveBeenCalled();
  });

  it("invalidates equal-plan account changes and real settings changes", async () => {
    const view = render(editor());
    await openEditor();
    await act(async () => view.rerender(editor(0, "profile/other-account/2")));
    expect(loadModels).toHaveBeenCalledTimes(2);
    await act(async () => view.rerender(editor(0, "profile/other-account/2", 1)));
    expect(loadModels).toHaveBeenCalledTimes(3);
  });

  it("does not submit cached models while refreshing or after the default is removed", async () => {
    render(editor());
    await openEditor();
    fillDraft();
    const refresh = deferred<ModelInfo[]>();
    loadModels.mockReturnValueOnce(refresh.promise);
    act(() => window.dispatchEvent(new Event("focus")));
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    expect(resolveCapabilities).not.toHaveBeenCalled();
    await act(async () => refresh.resolve([]));
    expect(screen.getByText(unavailableText)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "添加" }));
    expect(resolveCapabilities).not.toHaveBeenCalled();
    expect(createTask).not.toHaveBeenCalled();
  });
});

describe("automation model read lifecycle", () => {
  it("coalesces a focus burst and keeps the displayed list until the trailing read settles", async () => {
    const { result } = renderHook(() => useAutomationModels("", "account/global", 0, false, false));
    await waitFor(() => expect(result.current.loading).toBe(false));
    const first = deferred<ModelInfo[]>();
    const trailing = deferred<ModelInfo[]>();
    loadModels.mockReturnValueOnce(first.promise).mockReturnValueOnce(trailing.promise);
    act(() => {
      for (let i = 0; i < 10; i++) window.dispatchEvent(new Event("focus"));
    });
    expect(loadModels).toHaveBeenCalledTimes(2);
    expect(result.current.models).toEqual(models);
    expect(result.current.loading).toBe(true);
    expect(result.current.initialLoading).toBe(false);
    await act(async () => first.resolve([]));
    expect(loadModels).toHaveBeenCalledTimes(3);
    expect(result.current.models).toEqual(models);
    expect(result.current.loading).toBe(true);
    await act(async () => trailing.resolve([]));
    expect(result.current.models).toEqual([]);
    expect(result.current.loading).toBe(false);
  });

  it("rejects stale completion across account A to B to A, including while saving", async () => {
    const old = deferred<ModelInfo[]>();
    loadModels.mockReturnValueOnce(old.promise);
    const { result, rerender, unmount } = renderHook(
      ({ scope, saving }) => useAutomationModels("", scope, 0, saving, false),
      { initialProps: { scope: "A", saving: false } },
    );
    rerender({ scope: "B", saving: true });
    expect(result.current.models).toEqual([]);
    expect(result.current.loading).toBe(true);
    expect(result.current.initialLoading).toBe(true);
    rerender({ scope: "A", saving: false });
    await waitFor(() => expect(result.current.models).toEqual(models));
    await act(async () => old.resolve([]));
    expect(result.current.models).toEqual(models);
    expect(loadModels).toHaveBeenCalledTimes(2);
    unmount();
    expect(events.get("model-route:changed")?.size).toBe(0);
  });

  it("preserves display on failed refresh, exposes the error, and permits an explicit retry", async () => {
    const { result } = renderHook(() => useAutomationModels("", "A", 0, false, false));
    await waitFor(() => expect(result.current.loading).toBe(false));
    loadModels.mockRejectedValueOnce(new Error("offline"));
    await act(async () => result.current.refresh());
    expect(result.current.models).toEqual(models);
    expect(result.current.error).toBeTruthy();
    await act(async () => result.current.refresh());
    expect(result.current.error).toBeUndefined();
    expect(loadModels).toHaveBeenCalledTimes(3);
  });

  it("defers events during save and does not reload for an ordinary save phase change", async () => {
    const { result, rerender } = renderHook(
      ({ saving }) => useAutomationModels("", "A", 0, saving, false),
      { initialProps: { saving: false } },
    );
    await waitFor(() => expect(result.current.loading).toBe(false));
    rerender({ saving: true });
    rerender({ saving: false });
    expect(loadModels).toHaveBeenCalledTimes(1);
    rerender({ saving: true });
    act(() => {
      for (let i = 0; i < 5; i++) events.get("model-route:changed")?.forEach((cb) => cb("tab"));
    });
    expect(loadModels).toHaveBeenCalledTimes(1);
    rerender({ saving: false });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(loadModels).toHaveBeenCalledTimes(2);
  });
});
