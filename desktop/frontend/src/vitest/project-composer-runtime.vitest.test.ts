import { describe, expect, it, vi } from "vitest";
import {
  asProjectComposerSubmitError,
  classifyProjectComposerFirstSend,
  ensureProjectComposerAdapter,
  projectComposerFailureMessage,
  ProjectComposerSubmitError,
  submitProjectComposerTurn,
  type ProjectComposerAdapter,
} from "../features/projects/model/projectComposerAdapter";
import type { TabMeta } from "../lib/types";

const hash = "a".repeat(64);

function adapter(): ProjectComposerAdapter {
  const registry = new Map<string, ProjectComposerAdapter>();
  const value = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "project-a" })!;
  value.tabId = "tab-project";
  return value;
}

function tab(sessionPath = "session.jsonl"): TabMeta {
  return {
    id: "tab-project", scope: "project", workspaceRoot: "/repo", workspaceName: "repo",
    topicId: "topic", topicTitle: "Project task", sessionPath, label: "Project task",
    ready: true, running: false, mode: "normal", active: true, cwd: "/repo",
  };
}

function harness(send = vi.fn().mockResolvedValue(undefined)) {
  const calls: string[] = [];
  const createSession = vi.fn(async () => {
    calls.push("create");
    return { tabId: "tab-project", sessionPath: "session.jsonl", projectId: "project-a", instructionVersion: 2, contentHash: hash, reused: false };
  });
  return {
    calls,
    createSession,
    dependencies: {
      createSession,
      refreshTabs: vi.fn(async () => { calls.push("refresh"); return [tab()]; }),
      activateTab: vi.fn(async () => { calls.push("activate"); }),
      setCollaborationMode: vi.fn(async () => { calls.push("profile:collaboration"); }),
      setToolApprovalMode: vi.fn(async () => { calls.push("profile:approval"); }),
      setTokenMode: vi.fn(async (_tabId, mode) => { calls.push(`profile:token-${mode}`); }),
      setGoal: vi.fn(async () => { calls.push("profile:goal"); }),
      send: vi.fn(async () => { calls.push("send"); await send(); }),
      promote: vi.fn(async () => { calls.push("promote"); }),
    },
  };
}

describe("project Composer App integration runtime", () => {
  it("submits a first turn whose visible text is empty but session context exists in submitText", async () => {
    const target = adapter();
    const runtime = harness();
    const submitText = "以下是用户引用的历史会话上下文：\nuser: prior question";
    expect(classifyProjectComposerFirstSend("", submitText)).toBe("submitted");
    await expect(submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies)).resolves.toBe("submitted");
    expect(runtime.dependencies.send).toHaveBeenCalledTimes(1);
  });

  it("runs DTO -> refresh -> activate/path verification -> profile/token -> send -> promotion", async () => {
    const target = adapter();
    const runtime = harness();
    await expect(submitProjectComposerTurn(target, {
      projectId: "project-a", instructionVersion: 2, contentHash: hash,
    }, runtime.dependencies)).resolves.toBe("submitted");
    expect(runtime.calls).toEqual(["create", "refresh", "activate", "profile:collaboration", "profile:approval", "profile:token-full", "profile:goal", "send", "promote"]);
    expect(runtime.dependencies.setTokenMode).toHaveBeenCalledWith("tab-project", "full");
    expect(target.phase).toBe("sent");
  });

  it("keeps a pinned draft after send failure and retries without a second DTO call", async () => {
    const target = adapter();
    const send = vi.fn().mockRejectedValueOnce(new Error("send failed")).mockResolvedValueOnce(undefined);
    const runtime = harness(send);
    await expect(submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies)).rejects.toThrow("send failed");
    expect(target.phase).toBe("pinned-unsent");
    await expect(submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies)).resolves.toBe("submitted");
    expect(runtime.createSession).toHaveBeenCalledTimes(1);
  });

  it("classifies first-send failures by stage and exposes only a safe diagnostic message", async () => {
    const target = adapter();
    const runtime = harness();
    runtime.createSession.mockRejectedValue(new Error("D:\\private\\workspace token=secret"));
    const failure = await submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies).catch((error) => error);
    expect(failure).toBeInstanceOf(ProjectComposerSubmitError);
    expect(failure.stage).toBe("create-session");
    expect(failure.code).toBe("PROJECT_COMPOSER_SESSION_CREATE_FAILED");
    expect(projectComposerFailureMessage(failure)).toMatch(/^项目任务创建失败，草稿已保留，请重试。（诊断编号：[A-Z0-9]{10}）$/);
    expect(projectComposerFailureMessage(failure)).not.toContain("private");
    expect(target.phase).toBe("draft");
  });

  it("preserves a narrower capability stage when the send dependency classifies its own failure", async () => {
    const target = adapter();
    const runtime = harness();
    runtime.dependencies.send.mockRejectedValue(asProjectComposerSubmitError("resolve-capabilities", new Error("catalog failed")));
    const failure = await submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies).catch((error) => error);
    expect(failure).toBeInstanceOf(ProjectComposerSubmitError);
    expect(failure.stage).toBe("resolve-capabilities");
    expect(target.phase).toBe("pinned-unsent");
    expect(runtime.createSession).toHaveBeenCalledTimes(1);
  });

  it("rejects a concurrent duplicate while one first send is in flight", async () => {
    let release!: () => void;
    const waiting = new Promise<void>((resolve) => { release = resolve; });
    const target = adapter();
    const runtime = harness(vi.fn(() => waiting));
    const first = submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies);
    await vi.waitFor(() => expect(target.phase).toBe("sending"));
    await expect(submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies)).resolves.toBe("blocked");
    release();
    await expect(first).resolves.toBe("submitted");
    expect(runtime.createSession).toHaveBeenCalledTimes(1);
  });

  it("fails closed when refreshed metadata does not own the pinned path", async () => {
    const target = adapter();
    const runtime = harness();
    runtime.dependencies.refreshTabs.mockResolvedValue([tab("other.jsonl")]);
    await expect(submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies)).rejects.toThrow(/path reconciliation/);
    expect(runtime.dependencies.send).not.toHaveBeenCalled();
    expect(target.phase).toBe("pinned-unsent");
  });

  it("never makes an accepted send retryable when local promotion cleanup fails", async () => {
    const target = adapter();
    const runtime = harness();
    runtime.dependencies.promote.mockRejectedValue(new Error("local navigation failed"));
    await expect(submitProjectComposerTurn(target, { projectId: "project-a", instructionVersion: 2, contentHash: hash }, runtime.dependencies)).resolves.toBe("submitted");
    expect(target.phase).toBe("sent");
    expect(runtime.dependencies.send).toHaveBeenCalledTimes(1);
  });
});
