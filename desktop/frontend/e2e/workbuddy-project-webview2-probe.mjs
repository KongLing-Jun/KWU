import AxeBuilder from "@axe-core/playwright";
import { chromium } from "@playwright/test";
import { mkdir, readFile } from "node:fs/promises";
import { isAbsolute, resolve } from "node:path";

function argument(name, fallback = "") {
  const index = process.argv.indexOf(`--${name}`);
  return index >= 0 && process.argv[index + 1] ? process.argv[index + 1] : fallback;
}

const endpoint = argument("endpoint");
const output = argument("output");
const workspace = argument("workspace");
const mode = argument("mode", "seed");
const projectName = argument("project-name", "WorkBuddy RC WebView2");
const supportedModes = new Set(["seed", "resume", "crash", "inspect", "collaboration", "unified-seed", "unified-resume"]);
if (!endpoint || !output || !supportedModes.has(mode)) {
  console.error("usage: node workbuddy-project-webview2-probe.mjs --endpoint <loopback> --output <dir> --mode <seed|resume|crash|inspect|collaboration|unified-seed|unified-resume> [--workspace <dir>] [--project-name <name>]");
  process.exit(2);
}

async function readRCAccountSwitch() {
  const manifestPath = process.env.WORKBUDDY_RC_MANIFEST || "";
  if (!isAbsolute(manifestPath)) throw new Error("WORKBUDDY_RC_MANIFEST must be absolute");
  const value = JSON.parse(await readFile(manifestPath, "utf8"));
  if (!value.phone_b || !value.login_password || !value.user_b) throw new Error("RC manifest has no account-switch fixture");
  return { phone: value.phone_b, password: value.login_password, userId: value.user_b };
}

async function readRCAccounts() {
  const manifestPath = process.env.WORKBUDDY_RC_MANIFEST || "";
  if (!isAbsolute(manifestPath)) throw new Error("WORKBUDDY_RC_MANIFEST must be absolute");
  const value = JSON.parse(await readFile(manifestPath, "utf8"));
  if (!value.phone_a || !value.phone_b || !value.login_password || !value.user_a || !value.user_b) {
    throw new Error("RC manifest has no two-account fixture");
  }
  return {
    password: value.login_password,
    owner: { phone: value.phone_a, userId: value.user_a },
    member: { phone: value.phone_b, userId: value.user_b },
  };
}

async function verifyProjectMemberCannotUseAdminAPI() {
  const manifestPath = process.env.WORKBUDDY_RC_MANIFEST || "";
  if (!isAbsolute(manifestPath)) throw new Error("WORKBUDDY_RC_MANIFEST must be absolute");
  const value = JSON.parse(await readFile(manifestPath, "utf8"));
  if (!value.base_url || !value.phone_b || !value.login_password) throw new Error("RC manifest has no live Admin-boundary fixture");
  const deviceId = "workbuddy-stage5-admin-boundary";
  const login = await fetch(`${value.base_url}/api/v1/auth/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Device-Id": deviceId,
      "X-Request-Id": `req-stage5-${Date.now()}`,
    },
    body: JSON.stringify({
      phone: value.phone_b,
      password: value.login_password,
      captcha_ticket: "workbuddy-rc-captcha",
      captcha_provider: "aliyun_captcha_v2",
      device: { device_id: deviceId, client_version: "stage5-e2e" },
    }),
  });
  if (login.status !== 200) throw new Error(`live Admin-boundary login returned ${login.status}`);
  const envelope = await login.json();
  const accessToken = envelope?.data?.session?.access_token;
  if (!accessToken) throw new Error("live Admin-boundary login omitted its access token");
  const response = await fetch(`${value.base_url}/api/v1/admin/projects?page_size=20`, {
    headers: { Authorization: `Bearer ${accessToken}`, "X-Request-Id": `req-stage5-admin-${Date.now()}` },
  });
  return response.status;
}

async function verifyAdminProjectProjection(projectId) {
  const manifestPath = process.env.WORKBUDDY_RC_MANIFEST || "";
  if (!isAbsolute(manifestPath)) throw new Error("WORKBUDDY_RC_MANIFEST must be absolute");
  const value = JSON.parse(await readFile(manifestPath, "utf8"));
  if (!value.base_url || !value.access_token_admin) throw new Error("RC manifest has no isolated live Admin projection fixture");
  const response = await fetch(`${value.base_url}/api/v1/admin/projects/${encodeURIComponent(projectId)}`, {
    headers: { Authorization: `Bearer ${value.access_token_admin}`, "X-Request-Id": `req-stage5-admin-detail-${Date.now()}` },
  });
  const raw = await response.text();
  if (response.status !== 200) throw new Error(`live Admin projection returned ${response.status}: ${raw.slice(0, 200)}`);
  if (response.headers.get("cache-control") !== "no-store") throw new Error("live Admin projection omitted Cache-Control: no-store");
  for (const forbidden of ["member_identity", "message_text", "instruction_body", "work_item_content", "binding_metadata", "invite_token", "local_session", "local_task", "desktop_path", "credential", "device_availability"]) {
    if (raw.includes(`"${forbidden}"`)) throw new Error(`live Admin projection leaked forbidden field ${forbidden}`);
  }
  const project = JSON.parse(raw)?.data?.project;
  const fields = ["member_count", "user_message_count", "system_event_count"];
  if (!project || fields.some((field) => !Number.isSafeInteger(project[field]) || project[field] < 0)) {
    throw new Error(`live Admin projection omitted safe v2 counts: ${raw.slice(0, 300)}`);
  }
  return Object.fromEntries(fields.map((field) => [field, project[field]]));
}

async function waitForSingleProject(page, name) {
  const deadline = Date.now() + 20_000;
  let lastResult;
  while (Date.now() < deadline) {
    const result = await page.evaluate(() => window.go.main.App.ListCloudProjects({ pageSize: 100, status: "all" }));
    const matches = Array.isArray(result.items) ? result.items.filter((item) => item.name === name) : [];
    lastResult = result;
    if (matches.length === 1) return { ...result, items: matches };
    await new Promise((resolveWait) => setTimeout(resolveWait, 200));
  }
  const safeItems = Array.isArray(lastResult?.items)
    ? lastResult.items.map(({ projectId, name: itemName, status }) => ({ projectId, name: itemName, status }))
    : null;
  throw new Error(`restart reconciliation did not leave exactly one Project fact: ${JSON.stringify({ items: safeItems, scopeKey: lastResult?.projectScopeKey || null })}`);
}

async function collectProjectShellEvidence(page, screenshotPath) {
  const featureButtons = page.locator("section.sidebar-feature-zone > div.sidebar-feature-zone__items > button.sidebar-feature-zone__item");
  if ((await featureButtons.count()) < 2) throw new Error("Project navigation button is unavailable in the real Wails shell");
  await featureButtons.nth(1).click();
  await page.locator("#cloud-projects-title, #cloud-project-workspace-title").first().waitFor({ state: "visible", timeout: 10_000 });
  const axe = await new AxeBuilder({ page })
    .include(".cloud-projects")
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();
  const blockingAxe = axe.violations.filter((violation) => violation.impact === "serious" || violation.impact === "critical");
  await page.screenshot({ path: screenshotPath, animations: "disabled", fullPage: true });
  return blockingAxe.map(({ id, impact, help, nodes }) => ({
    id,
    impact,
    help,
    nodes: nodes.map(({ target, failureSummary }) => ({ target, failureSummary })),
  }));
}

await mkdir(resolve(output), { recursive: true });
const browser = await chromium.connectOverCDP(endpoint);
try {
  const deadline = Date.now() + 20_000;
  let pages = browser.contexts().flatMap((context) => context.pages());
  while (pages.length === 0 && Date.now() < deadline) {
    await new Promise((resolveWait) => setTimeout(resolveWait, 100));
    pages = browser.contexts().flatMap((context) => context.pages());
  }
  if (pages.length === 0) throw new Error("WebView2 exposed no page");
  const page = pages[0];
  await page.waitForLoadState("domcontentloaded", { timeout: 15_000 }).catch(() => {});
  await page.waitForFunction(() => Boolean(window.go?.main?.App?.CloudBootstrap), null, { timeout: 20_000 });
  const bootstrap = await page.evaluate(() => window.go.main.App.CloudBootstrap());
  if (bootstrap?.account?.status !== "authenticated") {
    throw new Error(`testdiag profile did not recover an authenticated account (available=${Boolean(bootstrap?.available)}, status=${bootstrap?.account?.status || "missing"}, operation=${bootstrap?.operationError?.code || "none"})`);
  }
  const consoleTypes = [];
  let pageErrorCount = 0;
  page.on("console", (message) => {
    if (message.type() === "error" || message.type() === "warning") consoleTypes.push(message.type());
  });
  page.on("pageerror", () => { pageErrorCount += 1; });

  if (mode === "crash") {
    await page.evaluate((name) => window.go.main.App.CreateCloudProject({ name, descriptionSet: true, description: "Synthetic RC crash description" }), projectName);
    throw new Error("fault-injected mutation unexpectedly returned");
  }

  let flow;
  if (mode === "seed") {
    flow = await page.evaluate(async ({ name, workspacePath }) => {
      const app = window.go.main.App;
      const created = await app.CreateCloudProject({ name, descriptionSet: true, description: "Synthetic RC description" });
      const projectId = created.project.projectId;
      const readback = await app.GetCloudProject(projectId);
      const searched = await app.ListCloudProjects({ search: name });
      let workspaceOpened = false;
      if (workspacePath) {
        await app.SwitchWorkspace(workspacePath);
        workspaceOpened = true;
      }
      return {
        projectId,
        scopeKey: created.projectScopeKey,
        readbackMatches: readback.project.projectId === projectId && readback.project.name === name,
        searchMatches: searched.items.length === 1 && searched.items[0].projectId === projectId,
        workspaceOpened,
      };
    }, { name: projectName, workspacePath: workspace });
    if (!flow.readbackMatches || !flow.searchMatches || (workspace && !flow.workspaceOpened)) throw new Error("real Wails seed flow did not reach its authoritative state");
  } else if (mode === "resume") {
    const list = await waitForSingleProject(page, projectName);
    flow = await page.evaluate(async ({ projectId, workspacePath, scopeKey }) => {
      const app = window.go.main.App;
      let sessionTabId = "";
      if (workspacePath) {
        const normalizedWorkspace = await app.SwitchWorkspace(workspacePath);
        const readyDeadline = Date.now() + 20_000;
        let activeTab;
        while (Date.now() < readyDeadline) {
          const tabs = await app.ListTabs();
          activeTab = tabs.find((tab) => tab.active && tab.workspaceRoot === normalizedWorkspace);
          if (activeTab?.startupErr) throw new Error(`workspace controller startup failed: ${activeTab.startupErr}`);
          if (activeTab?.ready) break;
          await new Promise((resolveWait) => setTimeout(resolveWait, 100));
        }
        if (!activeTab?.ready) throw new Error("workspace controller did not become ready");
        sessionTabId = activeTab.id;
      }
      const current = await app.GetCloudProject(projectId);
      const instruction = await app.EditCloudProjectInstruction({
        projectId,
        expectedVersion: 1,
        humanDescription: "Synthetic instruction summary",
        agentInstruction: "Keep the synthetic RC task scoped.",
      });
      const session = await app.NewCloudProjectSession({ projectId, tabId: sessionTabId });
      const workItem = await app.CreateCloudProjectWorkItem({ projectId, title: "Synthetic RC item", status: "todo" });
      const moved = await app.MoveCloudProjectWorkItem({
        projectId,
        workItemId: workItem.workItem.workItemId,
        expectedVersion: 1,
        targetStatus: "in_progress",
      });
      const items = await app.ListCloudProjectWorkItems({ projectId });
      return {
        projectId,
        scopeKey,
        projectVersion: current.project.version,
        instructionVersion: instruction.version,
        sessionProjectMatches: session.projectId === projectId && session.instructionVersion === instruction.version,
        movedStatus: moved.workItem.status,
        workItemCount: items.items.length,
      };
    }, { projectId: list.items[0].projectId, workspacePath: workspace, scopeKey: list.projectScopeKey });
    if (flow.instructionVersion !== 2 || !flow.sessionProjectMatches || flow.movedStatus !== "in_progress" || flow.workItemCount !== 1) {
      throw new Error("real Wails resumed Project flow did not reach its authoritative state");
    }
  } else if (mode === "unified-seed") {
    const accounts = await readRCAccounts();
    flow = await page.evaluate(async ({ name, actors }) => {
      const app = window.go.main.App;
      const switchAccount = (phone) => app.CloudSwitchAccount({
        phone,
        password: actors.password,
        captcha_passed: true,
        captcha_ticket: "workbuddy-rc-captcha",
        captcha_provider: "aliyun_captcha_v2",
      });
      const created = await app.CreateCloudProject({ name, descriptionSet: true, description: "Unified approved-scope RC" });
      const projectId = created.project.projectId;
      const invite = await app.CreateCloudProjectInvite({ projectId });
      await switchAccount(actors.member.phone);
      const accepted = await app.AcceptCloudProjectInvite({ inviteId: invite.inviteId, token: invite.token });
      await switchAccount(actors.owner.phone);
      const message = await app.PostCloudProjectMessage({ projectId, text: "Unified owner mention", mentionUserIds: [actors.member.userId] });
      const item = await app.CreateCloudProjectWorkItem({
        projectId,
        title: "Unified shared Plan item",
        status: "todo",
        assigneeUserId: actors.member.userId,
        priority: "high",
      });
      const moved = await app.MoveCloudProjectWorkItem({
        projectId,
        workItemId: item.workItem.workItemId,
        expectedVersion: item.workItem.version,
        targetStatus: "in_progress",
      });
      const capabilities = await app.ListCloudProjectCapabilities(projectId);
      const saved = await app.SaveCloudProjectCapabilities({
        projectId,
        expectedVersion: capabilities.projectVersion,
        skillIdSet: true,
        skillId: "@reasonix/planning",
      });
      return {
        projectId,
        scopeKey: created.projectScopeKey,
        inviteAccepted: accepted.status === "accepted",
        messageId: message.activityId,
        workItemId: moved.workItem.workItemId,
        movedStatus: moved.workItem.status,
        bindingId: saved.items[0]?.bindingId || "",
      };
    }, { name: projectName, actors: accounts });
    if (!flow.inviteAccepted || !flow.messageId || flow.movedStatus !== "in_progress" || !flow.bindingId) {
      throw new Error(`unified seed did not converge: ${JSON.stringify(flow)}`);
    }
  } else if (mode === "unified-resume") {
    const accounts = await readRCAccounts();
    const list = await waitForSingleProject(page, projectName);
    const adminProjection = await verifyAdminProjectProjection(list.items[0].projectId);
    flow = await page.evaluate(async ({ projectId, actors }) => {
      const app = window.go.main.App;
      const switchAccount = (phone) => app.CloudSwitchAccount({
        phone,
        password: actors.password,
        captcha_passed: true,
        captcha_ticket: "workbuddy-rc-captcha",
        captcha_provider: "aliyun_captcha_v2",
      });
      const members = await app.ListCloudProjectMembers(projectId);
      const activities = await app.ListCloudProjectActivities({ projectId, pageSize: 100, filter: "all_members", memberUserIds: [] });
      const workItems = await app.ListCloudProjectWorkItems({ projectId });
      const capabilities = await app.ListCloudProjectCapabilities(projectId);
      await switchAccount(actors.member.phone);
      const memberPlan = await app.ListCloudProjectWorkItems({ projectId });
      await switchAccount(actors.owner.phone);
      const current = await app.GetCloudProject(projectId);
      const archived = await app.ArchiveCloudProject({ projectId, expectedVersion: current.project.version });
      const restored = await app.RestoreCloudProject({ projectId, expectedVersion: archived.project.version });
      await app.DeleteCloudProject({ projectId, expectedVersion: restored.project.version });
      let deletedReadRejected = false;
      try { await app.GetCloudProject(projectId); } catch { deletedReadRejected = true; }
      return {
        projectId,
        memberCount: members.items.filter((item) => item.status === "active").length,
        activityCount: activities.items.length,
        workItemCount: workItems.items.length,
        bindingCount: capabilities.items.length,
        memberSawAssignedPlan: memberPlan.items.some((item) => item.assigneeUserId === actors.member.userId && item.status === "in_progress"),
        archivedStatus: archived.project.status,
        restoredStatus: restored.project.status,
        deletedReadRejected,
      };
    }, { projectId: list.items[0].projectId, actors: accounts });
    flow.memberAdminAPIStatus = await verifyProjectMemberCannotUseAdminAPI();
    flow.adminProjection = adminProjection;
    if (flow.memberCount !== 2 || flow.activityCount < 1 || flow.workItemCount !== 1 || flow.bindingCount !== 1 ||
        !flow.memberSawAssignedPlan || flow.archivedStatus !== "archived" || flow.restoredStatus !== "active" ||
        !flow.deletedReadRejected || flow.memberAdminAPIStatus !== 403 || flow.adminProjection.member_count !== 2 ||
        flow.adminProjection.user_message_count !== 1) {
      throw new Error(`unified resume did not converge: ${JSON.stringify(flow)}`);
    }
  } else if (mode === "collaboration") {
    const accounts = await readRCAccounts();
    flow = await page.evaluate(async ({ name, accounts: actors }) => {
      const app = window.go.main.App;
      const switchAccount = (phone) => app.CloudSwitchAccount({
        phone,
        password: actors.password,
        captcha_passed: true,
        captcha_ticket: "workbuddy-rc-captcha",
        captcha_provider: "aliyun_captcha_v2",
      });
      const step = async (label, call) => {
        try { return await call(); } catch (error) { throw new Error(`${label}: ${String(error)}`); }
      };
      const created = await step("create project", () => app.CreateCloudProject({ name, descriptionSet: true, description: "Synthetic two-account collaboration RC" }));
      const projectId = created.project.projectId;
      const invite = await step("create invite", () => app.CreateCloudProjectInvite({ projectId }));
      if (!invite.inviteId || !invite.token || invite.committedWithoutSecret) throw new Error("owner invite did not return its one-time secret");

      const memberLogin = await step("switch to member", () => switchAccount(actors.member.phone));
      const accepted = await step("accept invite", () => app.AcceptCloudProjectInvite({ inviteId: invite.inviteId, token: invite.token }));
      const memberList = await step("list members as member", () => app.ListCloudProjectMembers(projectId));

      const ownerLogin = await step("switch to owner", () => switchAccount(actors.owner.phone));
      const posted = await step("post owner mention", () => app.PostCloudProjectMessage({ projectId, text: "Synthetic owner mention", mentionUserIds: [actors.member.userId] }));
      const workItem = await step("create assigned work item", () => app.CreateCloudProjectWorkItem({
        projectId,
        title: "Synthetic shared Plan item",
        status: "todo",
        assigneeUserId: actors.member.userId,
        priority: "high",
      }));
      const moved = await step("move assigned work item", () => app.MoveCloudProjectWorkItem({
        projectId,
        workItemId: workItem.workItem.workItemId,
        expectedVersion: workItem.workItem.version,
        targetStatus: "in_progress",
      }));
      const current = await step("read before archive", () => app.GetCloudProject(projectId));
      const archived = await step("archive", () => app.ArchiveCloudProject({ projectId, expectedVersion: current.project.version }));
      const restored = await step("restore", () => app.RestoreCloudProject({ projectId, expectedVersion: archived.project.version }));

      await step("switch to member for Activity", () => switchAccount(actors.member.phone));
      const activityBefore = await step("list unread Activity", () => app.ListCloudProjectActivities({ projectId, pageSize: 100, filter: "all_members", memberUserIds: [] }));
      if (!activityBefore.readCursor) throw new Error("member Activity read cursor is missing");
      await step("mark Activity read", () => app.MarkCloudProjectActivityRead({ projectId, throughCursor: activityBefore.readCursor }));
      const activityAfter = await step("read Activity after watermark", () => app.ListCloudProjectActivities({ projectId, pageSize: 100, filter: "all_members", memberUserIds: [] }));
      const reply = await step("post member reply", () => app.PostCloudProjectMessage({ projectId, text: "Synthetic member reply", mentionUserIds: [actors.owner.userId] }));
      const memberWorkItems = await step("list shared Plan as member", () => app.ListCloudProjectWorkItems({ projectId }));

      await step("switch to owner for delete", () => switchAccount(actors.owner.phone));
      const ownerActivity = await step("owner reply readback", () => app.ListCloudProjectActivities({ projectId, pageSize: 100, filter: "all_members", memberUserIds: [] }));
      const beforeDelete = await step("read before delete", () => app.GetCloudProject(projectId));
      await step("permanent delete", () => app.DeleteCloudProject({ projectId, expectedVersion: beforeDelete.project.version }));
      let deletedReadRejected = false;
      try { await app.GetCloudProject(projectId); } catch { deletedReadRejected = true; }

      return {
        projectId,
        ownerScopeKey: created.projectScopeKey,
        inviteAccepted: accepted.status === "accepted" && accepted.projectId === projectId,
        accountSwitchMatches: memberLogin.account?.cloud_user_id === actors.member.userId && ownerLogin.account?.cloud_user_id === actors.owner.userId,
        memberRole: memberList.currentRole,
        activeMemberCount: memberList.items.filter((item) => item.status === "active").length,
        ownerMessageId: posted.activityId,
        memberMentionUnread: activityBefore.items.some((item) => item.activityId === posted.activityId && item.unread && item.mentionUserIds.includes(actors.member.userId)),
        activityReadCleared: activityAfter.items.filter((item) => item.unread).length === 0,
        memberReplyId: reply.activityId,
        ownerSawReply: ownerActivity.items.some((item) => item.activityId === reply.activityId),
        movedStatus: moved.workItem.status,
        memberSawAssignedWorkItem: memberWorkItems.items.some((item) => item.workItemId === moved.workItem.workItemId && item.assigneeUserId === actors.member.userId && item.status === "in_progress"),
        archivedStatus: archived.project.status,
        restoredStatus: restored.project.status,
        deletedReadRejected,
      };
    }, { name: projectName, accounts });
    if (!flow.inviteAccepted || !flow.accountSwitchMatches || flow.memberRole !== "member" || flow.activeMemberCount !== 2 ||
        !flow.ownerMessageId || !flow.memberMentionUnread || !flow.activityReadCleared || !flow.memberReplyId || !flow.ownerSawReply ||
        flow.movedStatus !== "in_progress" || !flow.memberSawAssignedWorkItem || flow.archivedStatus !== "archived" || flow.restoredStatus !== "active" || !flow.deletedReadRejected) {
      throw new Error(`real Wails collaboration flow did not converge: ${JSON.stringify(flow)}`);
    }
    flow.memberAdminAPIStatus = await verifyProjectMemberCannotUseAdminAPI();
    if (flow.memberAdminAPIStatus !== 403) throw new Error(`Project member Admin API boundary returned ${flow.memberAdminAPIStatus}`);
  } else {
    const list = await waitForSingleProject(page, projectName);
    flow = { projectId: list.items[0].projectId, scopeKey: list.projectScopeKey, matchCount: list.items.length };
  }

  const axeFindings = await collectProjectShellEvidence(page, resolve(output, `workbuddy-project-${mode}.png`));
  const seriousOrCriticalAxeViolations = axeFindings.length;
  if (mode === "resume") {
    const archived = await page.evaluate(async (projectId) => {
      const current = await window.go.main.App.GetCloudProject(projectId);
      return window.go.main.App.ArchiveCloudProject({ projectId, expectedVersion: current.project.version });
    }, flow.projectId);
    flow.archivedStatus = archived.project.status;
    const accountB = await readRCAccountSwitch();
    const switched = await page.evaluate((input) => window.go.main.App.CloudSwitchAccount({
      phone: input.phone,
      password: input.password,
      captcha_passed: true,
      captcha_ticket: "workbuddy-rc-captcha",
      captcha_provider: "aliyun_captcha_v2",
    }), accountB);
    const accountBProjects = await page.evaluate((name) => window.go.main.App.ListCloudProjects({ search: name }), projectName);
    let foreignReadRejected = false;
    try {
      await page.evaluate((projectId) => window.go.main.App.GetCloudProject(projectId), flow.projectId);
    } catch {
      foreignReadRejected = true;
    }
    flow.accountSwitchMatches = switched.account?.cloud_user_id === accountB.userId;
    flow.accountBProjectCount = accountBProjects.items.length;
    flow.foreignReadRejected = foreignReadRejected;
    if (flow.archivedStatus !== "archived" || !flow.accountSwitchMatches || flow.accountBProjectCount !== 0 || !flow.foreignReadRejected) {
      throw new Error("real Wails archive/account-switch isolation failed");
    }
  }

  const result = {
    mode,
    page: { url: page.url(), title: await page.title(), language: await page.locator("html").getAttribute("lang"), hasWailsRuntime: await page.evaluate(() => typeof window.runtime !== "undefined") },
    authenticated: true,
    flow,
    consoleErrorCount: consoleTypes.filter((type) => type === "error").length,
    consoleWarningCount: consoleTypes.filter((type) => type === "warning").length,
    pageErrorCount,
    seriousOrCriticalAxeViolations,
    axeFindings,
  };
  process.stdout.write(`${JSON.stringify(result)}\n`);
  if (result.consoleErrorCount || pageErrorCount || seriousOrCriticalAxeViolations) process.exitCode = 1;
} finally {
  await browser.close();
}
