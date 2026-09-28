import { Children, memo, useCallback, useEffect, useMemo, useState, type MouseEvent as ReactMouseEvent, type ReactNode, type Ref } from "react";
import { ArrowLeft } from "lucide-react";
import { expertAvatarSrcForKey, expertAvatarSrcForName, fallbackExpertMemberAvatarSrc } from "../lib/expertAssets";
import { useT } from "../lib/i18n";
import { app } from "../lib/bridge";
import { historyMessagesToItems } from "../features/session/historyProjection";
import { subjectOf } from "../lib/tools";
import type { Item, ToolStatus } from "../lib/useController";
import { AssistantMessage } from "./Message";
import { ToolCard } from "./ToolCard";
import { TranscriptRows, transcriptVirtualRows, type TranscriptVirtualRowsHandle } from "./TranscriptVirtualRows";

type ToolItem = Extract<Item, { kind: "tool" }>;
type AssistantItem = Extract<Item, { kind: "assistant" }>;
type SessionItem = ToolItem | AssistantItem;

export type AgentWorkspaceExpertMember = {
  kind?: string;
  name: string;
  role?: string;
  task?: string;
  output?: string;
  tone?: string;
  avatarKey?: string;
  avatarSrc?: string;
};

export type AgentWorkspaceExpertTeam = {
  name?: string;
  avatarSrc?: string;
  members?: readonly AgentWorkspaceExpertMember[];
};

export type AgentWorkspaceMemberMap = ReadonlyMap<string, AgentWorkspaceExpertMember | null | undefined>;

const TAB_HIT_SLOP_PX = 2;
const FALLBACK_AVATAR_TONES = ["amber", "violet", "teal", "blue", "green", "orange", "stone"] as const;

function labelFor(item: ToolItem): string {
  return item.subject || subjectOf(item.name, item.args) || item.summary || item.name;
}

function tabKeyFor(item: ToolItem, member?: AgentWorkspaceExpertMember | null): string {
  const name = normalizedMatchText(member?.name ?? "");
  return name ? `member:${name}` : `item:${item.id}`;
}

function initialFor(label: string): string {
  const trimmed = label.trim();
  return trimmed ? trimmed.slice(0, 1).toUpperCase() : "?";
}

function staticAvatarSrcForMember(member?: AgentWorkspaceExpertMember | null): string | undefined {
  if (!member) return undefined;
  return member.avatarSrc || expertAvatarSrcForKey(member.avatarKey) || expertAvatarSrcForName(member.name) || fallbackExpertMemberAvatarSrc(member.tone);
}

function uniqueAvatarSrcForMember(member: AgentWorkspaceExpertMember | null | undefined, used: Set<string>): string | undefined {
  if (!member) return undefined;
  const preferred = staticAvatarSrcForMember(member);
  if (preferred && !used.has(preferred)) {
    used.add(preferred);
    return preferred;
  }
  for (const tone of FALLBACK_AVATAR_TONES) {
    const src = fallbackExpertMemberAvatarSrc(tone);
    if (!used.has(src)) {
      used.add(src);
      return src;
    }
  }
  return preferred;
}

function normalizedMatchText(value: string): string {
  return value.toLocaleLowerCase().replace(/[\s\p{P}\p{S}]+/gu, "");
}

function argsMatchText(args: string): string {
  if (!args.trim()) return "";
  try {
    const parsed = JSON.parse(args) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      const record = parsed as Record<string, unknown>;
      return ["member_name", "description", "prompt", "task", "name", "subject"]
        .map((key) => record[key])
        .filter((value): value is string => typeof value === "string")
        .join(" ");
    }
  } catch {
    return args;
  }
  return args;
}

function memberNameFromArgs(args: string): string {
  if (!args.trim()) return "";
  try {
    const parsed = JSON.parse(args) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      const memberName = (parsed as Record<string, unknown>).member_name;
      return typeof memberName === "string" ? memberName.trim() : "";
    }
  } catch {
    return "";
  }
  return "";
}

function descriptionFromArgs(args: string): string {
  if (!args.trim()) return "";
  try {
    const parsed = JSON.parse(args) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      const description = (parsed as Record<string, unknown>).description;
      return typeof description === "string" ? description.trim() : "";
    }
  } catch {
    return "";
  }
  return "";
}

function delegatedTaskLabel(item: ToolItem): string {
  return item.subject?.trim() || descriptionFromArgs(item.args) || item.summary?.trim() || item.name;
}

function assignmentMatchText(item: ToolItem): string {
  return [memberNameFromArgs(item.args), delegatedTaskLabel(item), item.subject, descriptionFromArgs(item.args), item.summary]
    .filter((value): value is string => Boolean(value && value.trim()))
    .join(" ");
}

function itemMatchText(item: ToolItem): string {
  return [labelFor(item), item.subject, item.summary, item.name, argsMatchText(item.args)]
    .filter((value): value is string => Boolean(value && value.trim()))
    .join(" ");
}

const shortAsciiTerms = new Set(["ai", "api", "ci", "db", "go", "js", "qa", "ts", "ui", "ux"]);
const termAliases: Record<string, readonly string[]> = {
  check: ["inspect", "review", "检查", "审查"],
  changes: ["change", "diff", "变更", "改动", "回归"],
  diff: ["code", "review", "代码", "审查", "变更", "改动"],
  file: ["code", "代码"],
  git: ["code", "review", "代码", "审查"],
  inspect: ["check", "review", "检查", "审查"],
  review: ["check", "inspect", "检查", "审查"],
  变更: ["changes", "diff", "改动"],
  改动: ["changes", "diff", "变更"],
  检查: ["check", "inspect", "review", "审查"],
  审查: ["check", "inspect", "review", "检查"],
};

function addMatchTerm(terms: Set<string>, value: string) {
  const term = value.trim().toLocaleLowerCase();
  if (!term) return;
  terms.add(term);
  for (const alias of termAliases[term] ?? []) terms.add(alias);
}

function significantTerms(value: string): Set<string> {
  const terms = new Set<string>();
  for (const token of value.toLocaleLowerCase().match(/[a-z0-9]+/g) ?? []) {
    if (token.length >= 3 || shortAsciiTerms.has(token)) addMatchTerm(terms, token);
  }
  for (const run of value.match(/\p{Script=Han}+/gu) ?? []) {
    if (run.length <= 2) {
      addMatchTerm(terms, run);
      continue;
    }
    for (let i = 0; i < run.length - 1; i += 1) {
      addMatchTerm(terms, run.slice(i, i + 2));
    }
  }
  return terms;
}

function memberMatchText(member: AgentWorkspaceExpertMember): string {
  return [member.name, member.role, member.task, member.output]
    .filter((value): value is string => Boolean(value && value.trim()))
    .join(" ");
}

function overlapScore(itemTerms: ReadonlySet<string>, member: AgentWorkspaceExpertMember): number {
  const memberTerms = significantTerms(memberMatchText(member));
  let score = 0;
  for (const term of itemTerms) {
    if (memberTerms.has(term)) score += term.length >= 4 ? 2 : 1;
  }
  return score;
}

function leadMemberFor(team?: AgentWorkspaceExpertTeam | null): AgentWorkspaceExpertMember | null {
  const members = team?.members ?? [];
  return members.find((member) => member.kind?.trim().toLowerCase() === "lead") ?? members[0] ?? null;
}

function configuredSubExpertMembers(team?: AgentWorkspaceExpertTeam | null): AgentWorkspaceExpertMember[] {
  const members: AgentWorkspaceExpertMember[] = [];
  const seen = new Set<string>();
  for (const member of team?.members ?? []) {
    const name = member.name.trim();
    if (member.kind?.trim().toLowerCase() !== "sub" || !name) continue;
    const key = name.toLocaleLowerCase();
    if (seen.has(key)) return [];
    seen.add(key);
    members.push(member);
  }
  return members;
}

function matchMemberInPool(item: ToolItem, pool: readonly AgentWorkspaceExpertMember[]): AgentWorkspaceExpertMember | null {
  const itemText = itemMatchText(item);
  const haystack = normalizedMatchText(itemText);
  if (!haystack) return null;
  const labelText = normalizedMatchText(labelFor(item));
  for (const member of pool) {
    const name = normalizedMatchText(member.name);
    if (name && haystack.includes(name)) return member;
  }
  for (const member of pool) {
    const task = normalizedMatchText(member.task ?? "");
    if (task && (haystack.includes(task) || (labelText && task.includes(labelText)))) return member;
  }
  const itemTerms = significantTerms(itemText);
  let best: AgentWorkspaceExpertMember | null = null;
  let bestScore = 0;
  for (const member of pool) {
    const score = overlapScore(itemTerms, member);
    if (score > bestScore) {
      best = member;
      bestScore = score;
    }
  }
  return bestScore >= 2 ? best : null;
}

function memberForItem(item: ToolItem, team?: AgentWorkspaceExpertTeam | null, mappedMember?: AgentWorkspaceExpertMember | null): AgentWorkspaceExpertMember | null {
  if (!team) return mappedMember ?? null;
  const members = configuredSubExpertMembers(team);
  if (members.length === 0) return null;
  const explicitName = normalizedMatchText(memberNameFromArgs(item.args));
  if (explicitName) {
    return members.find((member) => normalizedMatchText(member.name) === explicitName) ?? null;
  }
  const namedTeamMembers = matchMemberNamesInPool(item, team.members ?? []);
  if (namedTeamMembers.length > 1) return null;
  const namedTeamMember = namedTeamMembers[0];
  if (namedTeamMember) {
    const namedKey = normalizedMatchText(namedTeamMember.name);
    const configuredNamedMember = members.find((member) => normalizedMatchText(member.name) === namedKey) ?? null;
    if (!configuredNamedMember) return null;
    if (mappedMember && normalizedMatchText(mappedMember.name) !== namedKey) return null;
    const assignmentText = normalizedMatchText(assignmentMatchText(item));
    const hasCompetingAssignment = members.some((member) => {
      if (normalizedMatchText(member.name) === namedKey) return false;
      return [member.role, member.task].some((value) => {
        const signature = normalizedMatchText(value ?? "");
        return signature.length >= 4 && assignmentText.includes(signature);
      });
    });
    if (hasCompetingAssignment) return null;
    return configuredNamedMember;
  }
  if (mappedMember) {
    const mappedName = normalizedMatchText(mappedMember.name);
    const configuredMappedMember = members.find((member) => normalizedMatchText(member.name) === mappedName);
    if (configuredMappedMember) return configuredMappedMember;
  }
  return matchMemberInPool(item, members);
}

function matchMemberNamesInPool(item: ToolItem, pool: readonly AgentWorkspaceExpertMember[]): AgentWorkspaceExpertMember[] {
  const haystack = normalizedMatchText(assignmentMatchText(item));
  if (!haystack) return [];
  return pool.filter((member) => {
    const name = normalizedMatchText(member.name);
    return Boolean(name && haystack.includes(name));
  });
}

function toneClass(tone?: string): string {
  const normalized = tone?.trim().toLowerCase();
  if (!normalized) return "";
  if (!["amber", "blue", "green", "violet", "orange", "teal", "stone"].includes(normalized)) return "";
  return ` agent-workspace__avatar--tone-${normalized}`;
}

function statusLabel(status: ToolStatus, t: ReturnType<typeof useT>): string {
  switch (status) {
    case "running": return t("agentWorkspace.running");
    case "done": return t("agentWorkspace.done");
    case "error": return t("agentWorkspace.error");
    case "stopped": return t("agentWorkspace.stopped");
  }
}

function tabKeyAtPoint(tabList: HTMLElement, clientX: number, clientY: number): string | null {
  if (!Number.isFinite(clientX) || !Number.isFinite(clientY)) return null;
  const tabs = Array.from(tabList.querySelectorAll<HTMLElement>("[data-agent-workspace-tab-id]"));
  let nearestInRow: { id: string; distance: number } | null = null;
  for (const tab of tabs) {
    const id = tab.dataset.agentWorkspaceTabKey ?? tab.dataset.agentWorkspaceTabId;
    if (!id) continue;
    const rect = tab.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) continue;
    const insideY = clientY >= rect.top - TAB_HIT_SLOP_PX && clientY <= rect.bottom + TAB_HIT_SLOP_PX;
    if (!insideY) continue;
    const insideX = clientX >= rect.left - TAB_HIT_SLOP_PX && clientX <= rect.right + TAB_HIT_SLOP_PX;
    if (insideX) return id;
    const distance = clientX < rect.left ? rect.left - clientX : clientX - rect.right;
    if (!nearestInRow || distance < nearestInRow.distance) nearestInRow = { id, distance };
  }
  return nearestInRow && nearestInRow.distance <= TAB_HIT_SLOP_PX ? nearestInRow.id : null;
}

function sessionItems(items?: readonly Item[]): SessionItem[] {
  return (items ?? []).filter((item): item is SessionItem => item.kind === "tool" || item.kind === "assistant");
}

function toolItems(items?: readonly Item[]): ToolItem[] {
  return (items ?? []).filter((item): item is ToolItem => item.kind === "tool");
}

function renderSessionItem(
  item: SessionItem,
  subcalls: ReadonlyMap<string, Item[]>,
  tabId?: string,
  expertTeam?: AgentWorkspaceExpertTeam | null,
  memberByItemId?: AgentWorkspaceMemberMap,
  childRef?: string,
) {
  if (item.kind === "assistant") {
    return <AssistantMessage key={item.id} item={item} tabId={tabId} />;
  }
  return <ToolCard key={item.id} item={item} subcalls={toolItems(subcalls.get(item.id))} tabId={tabId} expertTeam={expertTeam} memberByItemId={memberByItemId} childRef={childRef} />;
}

function outputFallbackFor(item: ToolItem, restoredOutput?: string): AssistantItem | null {
  const text = (restoredOutput ?? item.output ?? item.error ?? "").trim();
  if (!text) return null;
  return {
    kind: "assistant",
    id: `${item.id}:session-output`,
    text,
    reasoning: "",
    streaming: item.status === "running",
  };
}

function focusedSessionItems(item: ToolItem, subcalls: ReadonlyMap<string, Item[]>, restoredOutput?: string): SessionItem[] {
  const items = sessionItems(subcalls.get(item.id));
  const hasAssistant = items.some((sessionItem) => sessionItem.kind === "assistant");
  const fallback = hasAssistant ? null : outputFallbackFor(item, restoredOutput);
  return fallback ? [...items, fallback] : items;
}

function FocusedAgentSession({
  item,
  subcalls,
  tabId,
  expertTeam,
  memberByItemId,
  scrollElement,
}: {
  item: ToolItem;
  subcalls: ReadonlyMap<string, Item[]>;
  tabId?: string;
  expertTeam?: AgentWorkspaceExpertTeam | null;
  memberByItemId?: AgentWorkspaceMemberMap;
  scrollElement: HTMLDivElement | null;
}) {
  const [restoredOutput, setRestoredOutput] = useState<string | undefined>();
  const [restoredSessionItems, setRestoredSessionItems] = useState<SessionItem[]>([]);
  const [subagentHistoryFailed, setSubagentHistoryFailed] = useState(false);
  const directItems = sessionItems(subcalls.get(item.id));
  const availableItems = directItems.length > 0 ? directItems : restoredSessionItems;
  const hasAssistant = availableItems.some((sessionItem) => sessionItem.kind === "assistant");
  const needsArchivedOutput = Boolean(item.dataArchived && tabId && !hasAssistant && !item.output && !item.error && (!item.subagentRef || subagentHistoryFailed));

  useEffect(() => {
    setRestoredOutput(undefined);
    setRestoredSessionItems([]);
    setSubagentHistoryFailed(false);
  }, [item.id]);

  useEffect(() => {
    if (!item.subagentRef || !tabId || directItems.length > 0) return;
    let cancelled = false;
    const request = typeof app.SubagentHistoryPageForTab === "function"
      ? app.SubagentHistoryPageForTab(tabId, item.subagentRef, 0, 200, "").then((page) => page.messages)
      : app.SubagentHistoryForTab?.(tabId, item.subagentRef);
    if (!request) {
      setSubagentHistoryFailed(true);
      return;
    }
    void request.then((messages) => {
      if (cancelled) return;
      const restored = sessionItems(historyMessagesToItems(messages, `${item.id}-restored-`).items);
      if (restored.length > 0) setRestoredSessionItems(restored);
      else setSubagentHistoryFailed(true);
    }).catch(() => {
      if (!cancelled) setSubagentHistoryFailed(true);
    });
    return () => { cancelled = true; };
  }, [directItems.length, item.id, item.subagentRef, tabId]);

  useEffect(() => {
    if (!needsArchivedOutput || !tabId) return;
    let cancelled = false;
    app.ToolResultForTab(tabId, item.id).then((data) => {
      if (!cancelled) setRestoredOutput(data?.output || undefined);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, [item.id, needsArchivedOutput, tabId]);

  const focusedItems = availableItems.length > 0
    ? availableItems
    : focusedSessionItems(item, subcalls, restoredOutput);
  if (focusedItems.length > 0) {
    const rows = transcriptVirtualRows(focusedItems.map((sessionItem) => renderSessionItem(sessionItem, subcalls, tabId, expertTeam, memberByItemId, item.subagentRef)));
    return <TranscriptRows key={item.id} rows={rows} scrollElement={scrollElement} testId="agent-workspace-focused-virtual-list" />;
  }

  return <ToolCard key={item.id} item={item} subcalls={toolItems(subcalls.get(item.id))} tabId={tabId} expertTeam={expertTeam} memberByItemId={memberByItemId} />;
}

function LeadExpertAssignments({
  items,
  memberById,
  sourceLabel,
  fallbackTargetLabel,
  fallbackTaskLabel,
}: {
  items: readonly ToolItem[];
  memberById: ReadonlyMap<string, AgentWorkspaceExpertMember | null>;
  sourceLabel: string;
  fallbackTargetLabel: string;
  fallbackTaskLabel: string;
}) {
  return items.map((item) => {
    const delegatedLabel = delegatedTaskLabel(item).trim();
    const task = delegatedLabel === item.name ? fallbackTaskLabel : delegatedLabel;
    if (!task) return null;
    const target = memberById.get(item.id)?.name.trim() || fallbackTargetLabel;
    return (
      <div className="agent-workspace__assignment" data-agent-workspace-assignment={item.id} key={item.id}>
        <span className="agent-workspace__assignment-role">{sourceLabel} → {target}</span>
        <span className="agent-workspace__assignment-title">{task}</span>
      </div>
    );
  });
}

export const AgentWorkspace = memo(function AgentWorkspace({
  items,
  subcalls,
  tabId,
  expertTeam,
  memberByItemId,
  overviewContent,
  selectionScopeKey = "",
  scrollElement = null,
  overviewVirtualRowsRef,
}: {
  items: ToolItem[];
  subcalls: ReadonlyMap<string, Item[]>;
  tabId?: string;
  expertTeam?: AgentWorkspaceExpertTeam | null;
  memberByItemId?: AgentWorkspaceMemberMap;
  overviewContent?: ReactNode;
  selectionScopeKey?: string;
  scrollElement?: HTMLDivElement | null;
  overviewVirtualRowsRef?: Ref<TranscriptVirtualRowsHandle>;
}) {
  const t = useT();
  const [selection, setSelection] = useState({ scopeKey: selectionScopeKey, tabKey: "overview" });
  const selectedTabKey = selection.scopeKey === selectionScopeKey ? selection.tabKey : "overview";
  const selectTab = useCallback((tabKey: string) => {
    setSelection({ scopeKey: selectionScopeKey, tabKey });
  }, [selectionScopeKey]);
  const memberById = useMemo(() => new Map(items.map((item) => [item.id, memberForItem(item, expertTeam, memberByItemId?.get(item.id))])), [items, expertTeam, memberByItemId]);
  const visibleItems = useMemo(
    () => (expertTeam ? items.filter((item) => memberById.get(item.id) != null) : items),
    [items, expertTeam, memberById],
  );
  const tabs = useMemo(() => {
    const out: Array<{ key: string; item: ToolItem }> = [];
    const indexByKey = new Map<string, number>();
    for (const item of visibleItems) {
      const key = tabKeyFor(item, memberById.get(item.id));
      const index = indexByKey.get(key);
      if (index === undefined) {
        indexByKey.set(key, out.length);
        out.push({ key, item });
      } else {
        out[index] = { key, item };
      }
    }
    return out;
  }, [visibleItems, memberById]);
  const tabByKey = useMemo(() => new Map(tabs.map((tab) => [tab.key, tab.item])), [tabs]);
  const leadMember = useMemo(() => leadMemberFor(expertTeam), [expertTeam]);
  const activeTabKey = selectedTabKey === "overview" || tabByKey.has(selectedTabKey) ? selectedTabKey : "overview";
  const activeItem = activeTabKey === "overview" ? null : tabByKey.get(activeTabKey) ?? null;
  const activeId = activeItem?.id ?? "overview";
  const overviewLabel = t("agentWorkspace.overview");
  const overviewAvatarSrc = expertTeam?.avatarSrc;
  const viewMode = activeItem ? "focused" : "overview";
  const usedTabAvatarSrc = new Set<string>();
  if (overviewAvatarSrc) usedTabAvatarSrc.add(overviewAvatarSrc);
  const handleTabsClick = useCallback((event: ReactMouseEvent<HTMLDivElement>) => {
    if (event.detail === 0 && event.clientX === 0 && event.clientY === 0) return;
    const tabKey = tabKeyAtPoint(event.currentTarget, event.clientX, event.clientY);
    if (tabKey) selectTab(tabKey);
  }, [selectTab]);

  useEffect(() => {
    if (selectedTabKey !== "overview" && !tabByKey.has(selectedTabKey)) selectTab("overview");
  }, [selectedTabKey, selectTab, tabByKey]);

  const overviewRows = useMemo(() => {
    const nodes = overviewContent !== undefined
      ? Children.toArray(overviewContent)
      : visibleItems.map((item) => (
          <ToolCard key={item.id} item={item} subcalls={toolItems(subcalls.get(item.id))} tabId={tabId} expertTeam={expertTeam} memberByItemId={memberByItemId} />
        ));
    if (overviewContent !== undefined) {
      nodes.push(
        <LeadExpertAssignments
          key="lead-expert-assignments"
          items={visibleItems}
          memberById={memberById}
          sourceLabel={overviewLabel}
          fallbackTargetLabel={t("agentWorkspace.subExpert")}
          fallbackTaskLabel={t("agentWorkspace.delegatedTask")}
        />,
      );
    }
    return transcriptVirtualRows(nodes);
  }, [expertTeam, memberById, memberByItemId, overviewContent, overviewLabel, subcalls, t, tabId, visibleItems]);

  if (visibleItems.length === 0 && overviewContent === undefined) return null;

  return (
    <div className={`agent-workspace agent-workspace--session${activeItem ? " agent-workspace--focused" : ""}`}>
      <div className="agent-workspace__body agent-workspace__session-body" role="tabpanel" aria-live="polite">
        <div
          key={activeId}
          className="agent-workspace__view"
          data-view={viewMode}
          data-agent-workspace-active={activeId}
        >
          {activeItem ? (
            <FocusedAgentSession key={activeItem.id} item={activeItem} subcalls={subcalls} tabId={tabId} expertTeam={expertTeam} memberByItemId={memberByItemId} scrollElement={scrollElement} />
          ) : (
            <TranscriptRows
              key={`${selectionScopeKey}:overview`}
              ref={overviewVirtualRowsRef}
              rows={overviewRows}
              scrollElement={scrollElement}
              testId="agent-workspace-overview-virtual-list"
            />
          )}
        </div>
      </div>
      <div className="agent-workspace__footer">
        <div className="agent-workspace__tabs" role="tablist" aria-label={t("agentWorkspace.ariaLabel")} onClick={handleTabsClick}>
          <button
            type="button"
            className={`agent-workspace__tab${activeId === "overview" ? " agent-workspace__tab--active" : ""}`}
            role="tab"
            aria-selected={activeId === "overview"}
            data-agent-workspace-tab-id="overview"
            data-agent-workspace-tab-key="overview"
            onClick={() => selectTab("overview")}
          >
            <span className={`agent-workspace__avatar agent-workspace__avatar--main${toneClass(leadMember?.tone)}`}>
              {overviewAvatarSrc ? <img src={overviewAvatarSrc} alt="" loading="lazy" decoding="async" /> : <span>{initialFor(overviewLabel) || t("agentWorkspace.mainInitial")}</span>}
            </span>
            <span className="agent-workspace__label">{overviewLabel}</span>
          </button>
          {tabs.map(({ key, item }) => {
            const member = memberById.get(item.id);
            const inferredLabel = labelFor(item);
            const label = member?.name.trim() || (inferredLabel === item.name ? t("agentWorkspace.subExpert") : inferredLabel);
            const statusText = statusLabel(item.status, t);
            const memberAvatarSrc = uniqueAvatarSrcForMember(member, usedTabAvatarSrc);
            return (
              <button
                key={key}
                type="button"
                className={`agent-workspace__tab${activeId === item.id ? " agent-workspace__tab--active" : ""}`}
                role="tab"
                aria-selected={activeId === item.id}
                data-agent-workspace-tab-id={item.id}
                data-agent-workspace-tab-key={key}
                onClick={() => selectTab(key)}
              >
                <span
                  className={`agent-workspace__avatar agent-workspace__avatar--${item.status}${toneClass(member?.tone)}`}
                  data-agent-workspace-member={member?.name}
                >
                  {memberAvatarSrc ? <img src={memberAvatarSrc} alt="" loading="lazy" decoding="async" /> : <span>{initialFor(label)}</span>}
                </span>
                <span className="agent-workspace__label">{label}</span>
                <span className={`agent-workspace__status agent-workspace__status--${item.status}`} aria-label={statusText} title={statusText} />
              </button>
            );
          })}
        </div>
        {activeItem ? (
          <div className="agent-workspace__return-row">
            <button type="button" className="agent-workspace__return" onClick={() => selectTab("overview")}>
              <ArrowLeft size={15} aria-hidden="true" />
              <span>{t("agentWorkspace.returnMain")}</span>
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
});
