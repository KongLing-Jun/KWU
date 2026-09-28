import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { FileText, Plus, RefreshCw, Send } from "lucide-react";
import { useToast } from "../../../lib/toast";
import type { ProjectGateway } from "../data/projectGateway";
import { formatProjectDateTime } from "../model/projectDateTime";
import type { CloudProjectActivity, CloudProjectMember } from "../model/projectTypes";

interface Props { projectId: string; projectScopeKey: string; gateway: ProjectGateway; members: CloudProjectMember[]; readOnly: boolean; }

interface MentionRange { start: number; end: number; query: string; }
interface SelectedMention { userId: string; text: string; start: number; end: number; }

function mentionRangeAtCaret(value: string, caret: number): MentionRange | null {
  const beforeCaret = value.slice(0, caret);
  const match = beforeCaret.match(/(^|\s)@([^\s@]*)$/);
  if (!match) return null;
  const query = match[2] ?? "";
  return { start: caret - query.length - 1, end: caret, query };
}

function reconcileSelectedMentions(previousText: string, nextText: string, mentions: SelectedMention[]) {
  let prefix = 0;
  while (prefix < previousText.length && prefix < nextText.length && previousText[prefix] === nextText[prefix]) prefix += 1;
  let suffix = 0;
  while (suffix < previousText.length - prefix && suffix < nextText.length - prefix && previousText[previousText.length - suffix - 1] === nextText[nextText.length - suffix - 1]) suffix += 1;
  const previousEditEnd = previousText.length - suffix;
  const nextEditEnd = nextText.length - suffix;
  const delta = nextEditEnd - previousEditEnd;
  return mentions.flatMap((mention) => {
    if (mention.end <= prefix) return [mention];
    if (mention.start >= previousEditEnd) {
      const shifted = { ...mention, start: mention.start + delta, end: mention.end + delta };
      return nextText.slice(shifted.start, shifted.end) === shifted.text ? [shifted] : [];
    }
    return [];
  });
}

export function ProjectActivityPanel({ projectId, projectScopeKey, gateway, members, readOnly }: Props) {
  const { showError } = useToast();
  const listRequestRef = useRef(0);
  const postRequestRef = useRef(0);
  const activeMembers = useMemo(() => members.filter((member) => member.status === "active"), [members]);
  const [items, setItems] = useState<CloudProjectActivity[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [text, setText] = useState("");
  const [selectedMentions, setSelectedMentions] = useState<SelectedMention[]>([]);
  const [mentionRange, setMentionRange] = useState<MentionRange | null>(null);
  const [mentionIndex, setMentionIndex] = useState(0);
  const [composeOpen, setComposeOpen] = useState(false);
  const [status, setStatus] = useState("");
  const [pending, setPending] = useState(false);
  const [openingAssetId, setOpeningAssetId] = useState("");
  const messageRef = useRef<HTMLTextAreaElement>(null);
  const pendingMentionCaretRef = useRef<number | null>(null);
  const assetOpenRequestRef = useRef(0);
  const mentionUserIds = useMemo(() => [...new Set(selectedMentions.map((mention) => mention.userId))], [selectedMentions]);

  const mentionSuggestions = useMemo(() => {
    if (!mentionRange) return [];
    const query = mentionRange.query.trim().toLocaleLowerCase("zh-CN");
    return activeMembers.filter((member) => !query || member.displayName.toLocaleLowerCase("zh-CN").includes(query));
  }, [activeMembers, mentionRange]);

  const load = useCallback(async (cursor = "", append = false) => {
    const request = ++listRequestRef.current;
    setPending(true);
    try {
      const page = await gateway.listCloudProjectMessages({ projectId, pageSize: 20, ...(cursor ? { cursor } : {}) });
      if (listRequestRef.current !== request || page.projectScopeKey !== projectScopeKey || page.projectId !== projectId) return;
      setItems((current) => append ? mergeActivities(current, page.items) : page.items);
      setNextCursor(page.nextCursor);
      setStatus("");
      const unreadActivityIds = new Set(page.items.filter((item) => item.unread).map((item) => item.activityId));
      if (page.readCursor && unreadActivityIds.size && gateway.markCloudProjectMessageRead) {
        try {
          await gateway.markCloudProjectMessageRead({ projectId, throughCursor: page.readCursor });
          if (listRequestRef.current !== request || page.projectScopeKey !== projectScopeKey || page.projectId !== projectId) return;
          setItems((current) => current.map((item) => unreadActivityIds.has(item.activityId) ? { ...item, unread: false } : item));
        } catch (reason) {
          if (listRequestRef.current === request && page.projectScopeKey === projectScopeKey && page.projectId === projectId) {
            setStatus("留言已加载，但标记已读失败");
            showError("projects.operation", reason, "留言已加载，但标记已读失败");
          }
        }
      }
    } catch (reason) {
      if (listRequestRef.current === request) {
        setStatus("留言加载失败");
        showError("projects.operation", reason, "留言加载失败");
      }
    }
    finally { if (listRequestRef.current === request) setPending(false); }
  }, [gateway, projectId, projectScopeKey, showError]);

  useEffect(() => {
    setItems([]);
    setNextCursor(null);
    void load();
    return () => {
      listRequestRef.current += 1;
    };
  }, [load]);
  useEffect(() => {
    setOpeningAssetId("");
    assetOpenRequestRef.current += 1;
    return () => { postRequestRef.current += 1; assetOpenRequestRef.current += 1; };
  }, [projectId, projectScopeKey]);
  useEffect(() => { if (composeOpen) messageRef.current?.focus(); }, [composeOpen]);
  useLayoutEffect(() => {
    const caret = pendingMentionCaretRef.current;
    if (caret === null) return;
    pendingMentionCaretRef.current = null;
    messageRef.current?.focus();
    messageRef.current?.setSelectionRange(caret, caret);
  }, [text]);

  const post = async () => {
    if (!text.trim() || readOnly) return;
    if (new TextEncoder().encode(text).byteLength > 20_480) {
      setStatus("留言不能超过 20,480 个 UTF-8 字节。");
      return;
    }
    const request = ++postRequestRef.current;
    const postedProjectId = projectId;
    const postedScopeKey = projectScopeKey;
    setPending(true); setStatus("");
    try {
      const created = await gateway.postCloudProjectMessage({ projectId, text, mentionUserIds, assetIds: [] });
      if (postRequestRef.current !== request || projectId !== postedProjectId || projectScopeKey !== postedScopeKey) return;
      setItems((current) => [created, ...current.filter((item) => item.activityId !== created.activityId)]);
      setText(""); setSelectedMentions([]); setMentionRange(null); setMentionIndex(0); setComposeOpen(false); setStatus("");
    } catch (reason) {
      if (postRequestRef.current === request && projectId === postedProjectId && projectScopeKey === postedScopeKey) {
        const message = "留言结果未知；Go 日志会在下次刷新时按 client_message_id 恢复。";
        setStatus(message);
        showError("projects.operation", reason, message);
      }
    }
    finally { if (postRequestRef.current === request && projectId === postedProjectId && projectScopeKey === postedScopeKey) setPending(false); }
  };

  const openAsset = async (assetId: string) => {
    if (openingAssetId) return;
    const request = ++assetOpenRequestRef.current;
    setOpeningAssetId(assetId);
    setStatus("");
    try {
      await gateway.downloadCloudProjectAsset(projectId, assetId);
    } catch (reason) {
      if (assetOpenRequestRef.current === request) {
        setStatus("附件打开失败，请稍后重试。");
        showError("projects.operation", reason, "附件打开失败，请稍后重试。");
      }
    } finally {
      if (assetOpenRequestRef.current === request) setOpeningAssetId("");
    }
  };

  const selectMention = (member: CloudProjectMember) => {
    if (!mentionRange) return;
    const mentionText = `@${member.displayName}`;
    const insertion = `${mentionText} `;
    const nextText = `${text.slice(0, mentionRange.start)}${insertion}${text.slice(mentionRange.end)}`;
    const nextCaret = mentionRange.start + insertion.length;
    pendingMentionCaretRef.current = nextCaret;
    setText(nextText);
    setSelectedMentions((current) => [
      ...reconcileSelectedMentions(text, nextText, current),
      { userId: member.userId, text: mentionText, start: mentionRange.start, end: mentionRange.start + mentionText.length },
    ]);
    setMentionRange(null);
    setMentionIndex(0);
  };

  const updateMentionRange = (value: string, caret: number) => {
    setMentionRange(mentionRangeAtCaret(value, caret));
    setMentionIndex(0);
  };

  return <section className="project-activity" aria-labelledby="project-activity-title">
    <h3 id="project-activity-title" className="sr-only">项目动态</h3>
    <div className="project-activity__toolbar">
      <button type="button" className="project-activity__publish" aria-expanded={composeOpen} aria-controls="project-activity-composer" disabled={readOnly} onClick={() => setComposeOpen((open) => !open)}><Plus size={15} aria-hidden="true" />发布留言</button>
      <div className="project-activity__toolbar-end">
        <button type="button" className="project-activity__refresh" onClick={() => void load()} aria-label="刷新留言"><RefreshCw size={15} aria-hidden="true" /></button>
      </div>
    </div>
    {composeOpen ? <form id="project-activity-composer" onSubmit={(event) => { event.preventDefault(); void post(); }}>
      <textarea
        ref={messageRef}
        aria-label="发布项目留言"
        aria-autocomplete="list"
        aria-controls={mentionSuggestions.length ? "project-activity-mention-suggestions" : undefined}
        aria-expanded={mentionSuggestions.length > 0}
        aria-activedescendant={mentionSuggestions.length ? `project-activity-mention-option-${mentionIndex}` : undefined}
        placeholder="发布留言同步给项目成员，输入 @ 可提及成员"
        value={text}
        onChange={(event) => {
          const nextText = event.currentTarget.value;
          setSelectedMentions((current) => reconcileSelectedMentions(text, nextText, current));
          setText(nextText);
          updateMentionRange(nextText, event.currentTarget.selectionStart);
        }}
        onKeyDown={(event) => {
          if (!mentionSuggestions.length) return;
          if (event.key === "Escape") {
            event.preventDefault();
            setMentionRange(null);
          } else if (event.key === "ArrowDown") {
            event.preventDefault();
            setMentionIndex((current) => (current + 1) % mentionSuggestions.length);
          } else if (event.key === "ArrowUp") {
            event.preventDefault();
            setMentionIndex((current) => (current - 1 + mentionSuggestions.length) % mentionSuggestions.length);
          } else if (event.key === "Enter") {
            event.preventDefault();
            const selected = mentionSuggestions[mentionIndex];
            if (selected) selectMention(selected);
          }
        }}
        disabled={readOnly || pending}
        maxLength={20480}
      />
      {mentionSuggestions.length ? <div id="project-activity-mention-suggestions" className="project-activity__mention-suggestions" role="listbox" aria-label="提及成员建议">{mentionSuggestions.map((member, index) => <button id={`project-activity-mention-option-${index}`} key={member.userId} type="button" role="option" aria-selected={index === mentionIndex} onMouseDown={(event) => event.preventDefault()} onClick={() => selectMention(member)}>@{member.displayName}</button>)}</div> : null}
      <div className="project-activity__composer-actions">
        <div><button type="button" onClick={() => { setComposeOpen(false); setMentionRange(null); }}>取消</button><button type="submit" disabled={readOnly || pending || !text.trim()}><Send size={14} aria-hidden="true" />确认发布</button></div>
      </div>
    </form> : null}
    {status ? <p role="status">{status}</p> : null}
    <ol>{items.map((item) => <li key={item.activityId} data-unread={item.unread || undefined}>
      <span className="project-activity__avatar" aria-hidden="true">{item.authorDisplayName.trim().slice(0, 1).toUpperCase() || "?"}</span><div><p><strong className="project-activity__actor">{item.authorDisplayName}</strong> <span className="project-activity__action">{item.messageText || "发布了留言"}</span></p>{(item.assets ?? []).length ? <ul className="project-activity__message-assets">{item.assets.map((asset) => <li key={asset.assetId}><button type="button" aria-label={`打开附件 ${asset.filename}`} disabled={Boolean(openingAssetId)} onClick={() => void openAsset(asset.assetId)}><FileText size={13} aria-hidden="true" /><span>{asset.filename}</span></button></li>)}</ul> : null}{item.otherMemberReadStates?.length ? <div className="project-activity__read-receipts" aria-label="其他成员阅读状态">{item.otherMemberReadStates.map((state) => <span key={state.userId} data-status={state.status}>{state.displayName} · {state.status === "read" ? "已读" : "未读"}</span>)}</div> : null}</div>{item.unread ? <span className="project-activity__unread" aria-label="未读">未读</span> : null}<time dateTime={item.createdAt}>留言时间：{formatProjectDateTime(item.createdAt)}</time>
    </li>)}</ol>
    {nextCursor ? <button type="button" className="cloud-projects__more" disabled={pending} onClick={() => void load(nextCursor, true)}>加载更多留言</button> : null}
  </section>;
}

function mergeActivities(current: CloudProjectActivity[], incoming: CloudProjectActivity[]) {
  const byID = new Map(current.map((item) => [item.activityId, item]));
  for (const item of incoming) byID.set(item.activityId, item);
  return [...byID.values()];
}
