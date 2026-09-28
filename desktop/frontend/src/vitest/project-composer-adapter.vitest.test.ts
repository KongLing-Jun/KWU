import { describe, expect, it } from "vitest";
import {
  classifyProjectComposerFirstSend,
  ensureProjectComposerAdapter,
  ensureNewProjectConversationAdapter,
  parseProjectComposerKey,
  projectComposerBlankSessionTarget,
  projectComposerKey,
  reconcileProjectSession,
  releaseProjectComposerAdapter,
  planProjectComposerScopeChange,
  planProjectComposerLeave,
  applyProjectCapabilityBindings,
  availableProjectCapabilitySelection,
  projectComposerCapabilitySubmit,
  projectComposerAdapterForTab,
  projectComposerCapabilitiesForTab,
  initializeProjectComposerDefaults,
  type ProjectComposerAdapter,
} from "../features/projects/model/projectComposerAdapter";
import { projectExpertSelections, projectExpertTeamId } from "../features/projects/model/projectExpertSelection";
import type { ExpertInfo } from "../lib/types";

describe("project composer adapter", () => {
  it("initializes a project draft from the same model and approval defaults as a new conversation", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    const adapter = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "project-1" })!;
    initializeProjectComposerDefaults(adapter, {
      defaultModel: " cloud/glm-5.1 ",
      defaultBillingSource: "cloud_credits",
      defaultToolApprovalMode: "yolo",
    });
    expect(adapter).toMatchObject({
      executionTarget: "cloud",
      modelRef: "cloud/glm-5.1",
      modelBillingSource: "cloud_credits",
      effortRouteKey: "",
      defaultsInitialized: true,
      profile: { toolApprovalMode: "yolo" },
    });

    initializeProjectComposerDefaults(adapter, {
      defaultModel: "other/model",
      defaultBillingSource: "subscription_pool",
      defaultToolApprovalMode: "ask",
    });
    expect(adapter.modelRef).toBe("cloud/glm-5.1");
    expect(adapter.profile.toolApprovalMode).toBe("yolo");
  });

  it("starts a fresh project draft after the previous project conversation was sent", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    const identity = { accountScopeKey: "account-a", projectId: "project-1" };
    const sent = ensureProjectComposerAdapter(registry, identity)!;
    const canonicalKey = sent.key;
    sent.phase = "sent";
    sent.tabId = "tab-sent";
    sent.sessionPath = "C:\\sessions\\sent.jsonl";
    sent.capabilities = { skills: ["planning"], connectors: ["github"] };
    sent.availableExperts = [{ id: "testing", name: "测试专家", role: "测试", mode: "expert", prompt: "" }];

    const result = ensureNewProjectConversationAdapter(registry, identity);
    expect(result.replacedSentKey).toBe(canonicalKey);
    expect(result.adapter).not.toBe(sent);
    expect(result.adapter).toMatchObject({ phase: "draft", tabId: "", sessionPath: "" });
    expect(sent.key).not.toBe(canonicalKey);
    expect(registry.get(sent.key)).toBe(sent);
    expect(registry.get(canonicalKey)).toBe(result.adapter);
    expect(projectComposerCapabilitiesForTab(registry, "tab-sent")).toEqual({ skills: ["planning"], connectors: ["github"] });
    expect(projectComposerAdapterForTab(registry, "tab-sent")).toBe(sent);
    expect(projectComposerAdapterForTab(registry, "tab-sent")?.availableExperts.map((expert) => expert.id)).toEqual(["testing"]);

    const unsent = result.adapter!;
    expect(ensureNewProjectConversationAdapter(registry, identity)).toEqual({ adapter: unsent });
  });

  it("uses a project tab when bound and a workspace-independent tab when unbound", () => {
    expect(projectComposerBlankSessionTarget(" C:/repo ")).toEqual({ scope: "project", workspaceRoot: "C:/repo" });
    expect(projectComposerBlankSessionTarget("  ")).toEqual({ scope: "global", workspaceRoot: "" });
    expect(projectComposerBlankSessionTarget(" C:/repo ", "cloud")).toEqual({ scope: "global", workspaceRoot: "" });
  });

  it("uses an exact account and project identity without borrowing an active tab", () => {
    const identity = { accountScopeKey: "account-a", projectId: "project-1" };
    expect(parseProjectComposerKey(projectComposerKey(identity))).toEqual(identity);
  });

  it("keeps project bindings available without enabling skills or connectors", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    const adapter = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "project-1" })!;
    applyProjectCapabilityBindings(adapter, [
      { bindingId: "b1", kind: "skill", registryRef: "@reasonix/planning", position: 0, config: {}, publicDisabled: false, version: 1 },
      { bindingId: "b2", kind: "connector", registryRef: "github", position: 0, config: {}, publicDisabled: false, version: 1 },
    ]);
    expect(adapter.capabilities).toEqual({ skills: ["planning"], connectors: [] });
    expect(projectComposerCapabilitySubmit("ship it", adapter.capabilities)).toBeUndefined();
    expect(projectComposerCapabilitySubmit("ship it", adapter.capabilities, {
      text: "ship it",
      skill_names: ["planning"],
      mcp_server_names: [],
    }, adapter.capabilities)).toEqual({
      text: "ship it",
      skill_names: ["planning"],
      mcp_server_names: [],
    });
    expect(projectComposerCapabilitySubmit("ship it", adapter.capabilities, {
      text: "ship it",
      skill_names: [],
      mcp_server_names: ["github"],
    }, adapter.capabilities)).toEqual({
      text: "ship it",
      skill_names: [],
      mcp_server_names: ["github"],
    });
  });

  it("keeps the project capability scope for later turns on the promoted tab", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    const adapter = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "project-1" })!;
    adapter.phase = "sent";
    adapter.tabId = "project-tab";
    adapter.sessionPath = "sessions/project.jsonl";
    adapter.capabilities = { skills: ["planning"], connectors: ["github"] };

    expect(projectComposerCapabilitiesForTab(registry, "project-tab")).toEqual({
      skills: ["planning"],
      connectors: ["github"],
    });
    expect(projectComposerCapabilitiesForTab(registry, "ordinary-tab")).toBeUndefined();
  });

  it("does not let a stale cloud skill binding block the local project turn", () => {
    expect(availableProjectCapabilitySelection(
      { skills: ["superpowers-skill", "cloudbase", "stale-skill"], connectors: ["github"] },
      [
        { name: "superpowers", uninstallId: "@user_14606527/superpowers-skill", enabled: true },
        { name: "cloudbase", uninstallId: "@tencent-adm/cloudbase", enabled: true },
        { name: "stale-skill", uninstallId: "stale-skill", enabled: false },
      ],
    )).toEqual({ skills: ["superpowers", "cloudbase"], connectors: ["github"] });
  });

  it("drops unavailable project connectors without reintroducing them from the Composer turn", () => {
    const configured = { skills: ["planning"], connectors: ["github", "cloudbase"] };
    const available = availableProjectCapabilitySelection(
      configured,
      [{ name: "planning", uninstallId: "planning", enabled: true }],
      [
        { registryRef: "github", localServerName: "github", available: true },
        { registryRef: "cloudbase", localServerName: "", available: false },
      ],
    );
    expect(available).toEqual({ skills: ["planning"], connectors: ["github"] });
    expect(projectComposerCapabilitySubmit("ship it", available, {
      text: "ship it",
      skill_names: [],
      mcp_server_names: ["github", "cloudbase", "turn-only"],
    }, configured)).toEqual({
      text: "ship it",
      skill_names: [],
      mcp_server_names: ["github", "turn-only"],
    });
    expect(projectComposerCapabilitySubmit("ship it", { skills: [], connectors: [] }, {
      text: "ship it",
      skill_names: [],
      mcp_server_names: ["cloudbase"],
    }, configured)).toEqual({
      text: "ship it",
      skill_names: [],
      mcp_server_names: [],
    });
  });

  it("keeps every project expert binding selectable without treating a binding as enabled", () => {
    expect(projectExpertTeamId(["testing", "architecture"])).toBe("project-expert-team-116fb3dc1ea757d6");
    const bindings = [
      { bindingId: "e2", kind: "expert", registryRef: "architecture", position: 2, config: {}, publicDisabled: false, version: 1 },
      { bindingId: "e1", kind: "expert", registryRef: "testing", position: 1, config: {}, publicDisabled: false, version: 1 },
      { bindingId: "e3", kind: "expert", registryRef: "disabled", position: 3, config: {}, publicDisabled: true, version: 1 },
    ] as const;
    const experts: ExpertInfo[] = [
      { id: "testing", name: "测试专家", description: "测试", role: "质量", enabled: true, mode: "expert", prompts: ["测试建议"] },
      { id: "architecture", name: "架构专家", description: "架构", role: "架构", enabled: true, mode: "expert", prompts: ["架构建议"] },
      { id: "disabled", name: "停用专家", description: "停用", enabled: true, mode: "expert" },
    ];
    const selections = projectExpertSelections(bindings, experts);

    expect(selections.map((expert) => expert.id)).toEqual(["testing", "architecture"]);
    expect(selections.every((expert) => expert.mode === "expert")).toBe(true);
  });

  it("keeps an existing expert team selectable without enabling it from the binding", () => {
    const [selection] = projectExpertSelections([
      { bindingId: "team", kind: "expert", registryRef: "delivery-team", position: 0, config: {}, publicDisabled: false, version: 1 },
    ], [{
      id: "delivery-team",
      name: "交付专家团",
      description: "协作交付",
      enabled: true,
      mode: "team",
      members: [{ name: "负责人", role: "交付", task: "统筹", output: "结果" }],
    }]);

    expect(selection?.id).toBe("delivery-team");
    expect(selection?.mode).toBe("team");
    expect(selection).not.toHaveProperty("projectExpertIds");
  });

  it("keeps at most eight adapters per account scope and removes stale scopes", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    for (let index = 0; index < 8; index += 1) {
      expect(ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: `p-${index}` })).not.toBeNull();
    }
    expect(ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "p-9" })).toBeNull();
    expect(ensureProjectComposerAdapter(registry, { accountScopeKey: "account-b", projectId: "p-9" })).not.toBeNull();
    const plan = planProjectComposerScopeChange(registry, "account-b");
    expect(plan.removedKeys).toHaveLength(8);
    expect(plan.closeTabIds).toEqual([]);
    expect([...registry.values()].map((adapter) => adapter.accountScopeKey)).toEqual(["account-b"]);
  });

  it("closes every old-scope project tab, including pinned and sent sessions", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    const draft = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "draft" })!;
    draft.tabId = "tab-draft";
    const pinned = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "pinned" })!;
    pinned.tabId = "tab-pinned";
    pinned.sessionPath = "sessions/pinned.jsonl";
    pinned.phase = "pinned-unsent";
    const sent = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "sent" })!;
    sent.tabId = "tab-sent";
    sent.sessionPath = "sessions/sent.jsonl";
    sent.phase = "sent";

    const plan = planProjectComposerScopeChange(registry, "account-b");

    expect(plan).toEqual({
      removedKeys: [draft.key, pinned.key, sent.key],
      closeTabIds: ["tab-draft", "tab-pinned", "tab-sent"],
      retainedKeys: [],
    });
    expect(registry.size).toBe(0);
  });

  it("blocks local or management commands before creating the first project turn", () => {
    expect(classifyProjectComposerFirstSend("/memory")).toBe("handled-no-turn");
    expect(classifyProjectComposerFirstSend("/clear")).toBe("handled-no-turn");
    expect(classifyProjectComposerFirstSend("/mcp connect docs")).toBe("handled-no-turn");
    expect(classifyProjectComposerFirstSend("!pwd")).toBe("handled-no-turn");
    expect(classifyProjectComposerFirstSend("   ")).toBe("blocked");
    expect(classifyProjectComposerFirstSend("/goal ship the project")).toBe("submitted");
    expect(classifyProjectComposerFirstSend("/mcp__docs__summarize topic")).toBe("submitted");
    expect(classifyProjectComposerFirstSend("/custom-command @workspace:/repo")).toBe("submitted");
    expect(classifyProjectComposerFirstSend("", "historical session context only")).toBe("submitted");
    expect(classifyProjectComposerFirstSend("write the report")).toBe("submitted");
  });

  it("pins only the returned prepared tab and non-empty session path", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    const adapter = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "p-1" })!;
    expect(adapter).toMatchObject({ modelRef: "", workspacePath: "", executionTarget: "cloud" });
    const session = { tabId: "tab-1", sessionPath: "sessions/one.jsonl", projectId: "p-1", instructionVersion: 2, contentHash: "a".repeat(64), reused: false };
    expect(reconcileProjectSession(adapter, session, { projectId: "p-1", tabId: "tab-1", instructionVersion: 2, contentHash: "a".repeat(64) })).toMatchObject({ phase: "pinned-unsent", tabId: "tab-1", sessionPath: "sessions/one.jsonl" });
    expect(() => reconcileProjectSession(adapter, { ...session, tabId: "tab-other" }, { projectId: "p-1", tabId: "tab-1", instructionVersion: 2, contentHash: "a".repeat(64) })).toThrow(/prepared tab/);
  });

  it("releases capacity after a promoted or discarded adapter is removed", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    for (let index = 0; index < 8; index += 1) ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: `p-${index}` });
    const released = [...registry.values()][0];
    expect(releaseProjectComposerAdapter(registry, released.key)).toBe(released);
    expect(ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "p-9" })).not.toBeNull();
  });

  it("closes an empty prepared Tab on project leave but retains a non-empty or pinned draft", () => {
    const registry = new Map<string, ProjectComposerAdapter>();
    const prepared = ensureProjectComposerAdapter(registry, { accountScopeKey: "account-a", projectId: "prepared" })!;
    prepared.tabId = "tab-prepared";
    expect(planProjectComposerLeave(prepared, undefined)).toEqual({ removeKey: prepared.key, closeTabId: "tab-prepared" });
    expect(planProjectComposerLeave(prepared, { text: "keep", attachments: [], workspaceRefs: [], pastedBlocks: [], sessionRefs: [] })).toBeNull();
    prepared.sessionPath = "session.jsonl";
    prepared.phase = "pinned-unsent";
    expect(planProjectComposerLeave(prepared, undefined)).toBeNull();
  });
});
