import { Children, memo, useCallback, useEffect, useMemo, useState, type ReactNode, type Ref } from "react";

import { historyMessagesToItems } from "../features/session/historyProjection";
import { expertAvatarSrcForKey, expertAvatarSrcForName, fallbackExpertMemberAvatarSrc } from "../lib/expertAssets";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { ExpertTeamSession, ExpertTeamSessionMember } from "../lib/types";
import type { Item, ToolStatus } from "../lib/useController";
import { AssistantMessage, UserMessage } from "./Message";
import { ToolCard } from "./ToolCard";
import { TranscriptRows, transcriptVirtualRows, type TranscriptVirtualRow, type TranscriptVirtualRowsHandle } from "./TranscriptVirtualRows";

type ChildItems = Record<string, Item[]>;

function initialFor(value: string): string {
  return value.trim().slice(0, 1).toUpperCase() || "?";
}

function memberAvatar(member: ExpertTeamSessionMember): string | undefined {
  return expertAvatarSrcForKey(member.avatarKey)
    || expertAvatarSrcForKey(member.expertId)
    || expertAvatarSrcForName(member.name)
    || fallbackExpertMemberAvatarSrc();
}

function toolStatus(value?: string): ToolStatus | null {
  switch (value) {
    case "running": return "running";
    case "completed": return "done";
    case "failed":
    case "interrupted":
    case "cancelled":
    case "stopped":
    case "timeout": return "error";
    default: return null;
  }
}

function statusText(value: string | undefined, status: ToolStatus, t: ReturnType<typeof useT>): string {
  if (value === "interrupted" || value === "cancelled" || value === "stopped") {
    return t("agentWorkspace.interrupted");
  }
  return t(`agentWorkspace.${status}`);
}

function renderChildItem(item: Item, tabId: string, ref: string): ReactNode {
  switch (item.kind) {
    case "user":
      return <UserMessage key={item.id} id={item.id} text={item.text} submitText={item.submitText} createdAt={item.createdAt} editDisabled />;
    case "assistant":
      return <AssistantMessage key={item.id} item={item} tabId={tabId} />;
    case "tool":
      return <ToolCard key={item.id} item={item} tabId={tabId} childRef={ref} />;
    case "notice":
    case "phase":
      return <div key={item.id} className="notice notice--info">{item.text}</div>;
    default:
      return null;
  }
}

const ExpertMemberHistory = memo(function ExpertMemberHistory({
  tabId,
  member,
  liveItems,
  scrollElement,
}: {
  tabId: string;
  member: ExpertTeamSessionMember;
  liveItems: Item[];
  scrollElement: HTMLDivElement | null;
}) {
  const t = useT();
  const ref = member.subagentRef ?? "";
  const [restored, setRestored] = useState<Item[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  const load = useCallback(async () => {
    if (!ref) return;
    try {
      const page = await app.SubagentHistoryPageForTab?.(tabId, ref, 0, 500, "");
      if (!page) throw new Error("history bridge unavailable");
      setRestored(historyMessagesToItems(page.messages, `member:${ref}:`).items);
      setFailed(false);
    } catch {
      setFailed(true);
    } finally {
      setLoading(false);
    }
  }, [ref, tabId]);

  useEffect(() => {
    setRestored([]);
    setLoading(true);
    setFailed(false);
    void load();
  }, [load]);

  useEffect(() => {
    if (member.status !== "running") return;
    const timer = window.setInterval(() => void load(), 1000);
    return () => window.clearInterval(timer);
  }, [load, member.status]);

  const items = useMemo(() => {
    if (restored.length === 0) return liveItems;
    // Saved child history ends at completed model messages. Its poll must not
    // hide the in-progress message that still owns the live reasoning stream.
    const streaming = liveItems.filter((item) => item.kind === "assistant" && item.streaming);
    return streaming.length > 0 ? [...restored, ...streaming] : restored;
  }, [liveItems, restored]);
  if (items.length > 0) {
    const rows = transcriptVirtualRows(items.map((item) => renderChildItem(item, tabId, ref)));
    return <TranscriptRows key={ref} rows={rows} scrollElement={scrollElement} testId="expert-team-child-virtual-list" />;
  }
  if (loading) return <div className="primary-workspace__empty">{t("agentWorkspace.loadingHistory")}</div>;
  return <div className="primary-workspace__empty">{t(failed ? "agentWorkspace.historyUnavailable" : "agentWorkspace.notStarted")}</div>;
});

export const ExpertTeamSessionView = memo(function ExpertTeamSessionView({
  team,
  tabId,
  childItemsByRef,
  overviewContent,
  rows,
  selectionScopeKey,
  onMemberFocusChange,
  scrollElement = null,
  overviewVirtualRowsRef,
}: {
  team: ExpertTeamSession | null;
  tabId: string;
  childItemsByRef: ChildItems;
  overviewContent?: ReactNode;
  rows?: TranscriptVirtualRow[];
  selectionScopeKey: string;
  onMemberFocusChange?: (focused: boolean, returnToLead?: () => void) => void;
  scrollElement?: HTMLDivElement | null;
  overviewVirtualRowsRef?: Ref<TranscriptVirtualRowsHandle>;
}) {
  const t = useT();
  const [selection, setSelection] = useState({ scope: selectionScopeKey, memberKey: "" });
  const selectedKey = selection.scope === selectionScopeKey ? selection.memberKey : "";
  const visibleMembers = useMemo(() => (team?.members ?? []).filter((member) => Boolean(member.subagentRef)), [team?.members]);
  const selectedMember = visibleMembers.find((member) => member.memberKey === selectedKey) ?? null;
  const selectMember = useCallback((memberKey: string) => {
    onMemberFocusChange?.(Boolean(memberKey), memberKey ? () => selectMember("") : undefined);
    setSelection({ scope: selectionScopeKey, memberKey });
  }, [onMemberFocusChange, selectionScopeKey]);
  const returnToLead = useCallback(() => selectMember(""), [selectMember]);
  const leadName = team?.lead?.name || team?.name || "";
  const leadAvatar = team ? memberAvatar(team.lead ?? { memberKey: "lead", name: leadName }) : undefined;
  const selectedRef = selectedMember?.subagentRef ?? "";
  const liveItems = useMemo(() => childItemsByRef[selectedRef] ?? [], [childItemsByRef, selectedRef]);
  const overviewRows = useMemo(() => rows ?? transcriptVirtualRows(Children.toArray(overviewContent)), [rows, overviewContent]);

  useEffect(() => {
    if (selectedKey && !selectedMember) selectMember("");
  }, [selectedKey, selectedMember, selectionScopeKey]);

  useEffect(() => {
    onMemberFocusChange?.(Boolean(selectedMember), selectedMember ? returnToLead : undefined);
  }, [onMemberFocusChange, returnToLead, selectedMember]);
  useEffect(() => () => onMemberFocusChange?.(false), [onMemberFocusChange]);

  return (
    <div className={`${team ? "agent-workspace agent-workspace--session " : ""}transcript-parent-workspace${selectedMember ? " agent-workspace--focused" : ""}`} data-team-state={team?.state}>
      <div className="agent-workspace__body agent-workspace__session-body" role={team ? "tabpanel" : undefined} aria-live={team ? "polite" : undefined}>
        <div className="agent-workspace__view" data-view={selectedMember ? "focused" : "overview"}>
          {selectedMember && selectedRef
            ? <ExpertMemberHistory key={`${selectionScopeKey}:${selectedRef}`} tabId={tabId} member={selectedMember} liveItems={liveItems} scrollElement={scrollElement} />
            : (
                <TranscriptRows
                  key={`${selectionScopeKey}:overview`}
                  ref={overviewVirtualRowsRef}
                  rows={overviewRows}
                  scrollElement={scrollElement}
                  testId={team ? "expert-team-overview-virtual-list" : "transcript-virtual-list"}
                />
              )}
        </div>
      </div>
      <div className="transcript-parent-controls">
      {team && <div className="agent-workspace__footer">
        <div className="agent-workspace__tabs" role="tablist" aria-label={t("agentWorkspace.ariaLabel")}>
          <button
            type="button"
            className={`agent-workspace__tab${selectedMember ? "" : " agent-workspace__tab--active"}`}
            role="tab"
            aria-selected={!selectedMember}
            onClick={returnToLead}
          >
            <span className="agent-workspace__avatar agent-workspace__avatar--main">
              {leadAvatar ? <img src={leadAvatar} alt="" /> : <span>{initialFor(leadName)}</span>}
            </span>
            <span className="agent-workspace__label">{leadName}</span>
          </button>
          {visibleMembers.map((member) => {
            const status = toolStatus(member.status);
            const avatar = memberAvatar(member);
            return (
              <button
                key={member.memberKey}
                type="button"
                className={`agent-workspace__tab${selectedMember?.memberKey === member.memberKey ? " agent-workspace__tab--active" : ""}`}
                role="tab"
                aria-selected={selectedMember?.memberKey === member.memberKey}
                title={member.role}
                onClick={() => selectMember(member.memberKey)}
              >
                <span className={`agent-workspace__avatar${status ? ` agent-workspace__avatar--${status}` : ""}`} data-agent-workspace-member={member.name}>
                  {avatar ? <img src={avatar} alt="" /> : <span>{initialFor(member.name)}</span>}
                </span>
                <span className="agent-workspace__label">{member.name}</span>
                {status ? <span className={`agent-workspace__status agent-workspace__status--${status}`} aria-label={statusText(member.status, status, t)} title={statusText(member.status, status, t)} /> : null}
              </button>
            );
          })}
        </div>
      </div>}
      </div>
    </div>
  );
});
