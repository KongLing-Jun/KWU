import { memo, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { CodeViewer } from "./CodeViewer";
import { DiffView } from "./DiffView";
import { useT } from "../lib/i18n";
import { app } from "../lib/bridge";
import { diffsFor, languageForToolArgs, subjectOf, summarize, summarizeFileDiff, toolDisplayName } from "../lib/tools";
import { useShellExpand } from "../lib/shellExpand";
import { useGSAPCollapse } from "../lib/useGSAPCollapse";
import { mcpToolEvidence } from "../lib/mcpToolEvidence";
import type { Item, ToolStatus } from "../lib/useController";
import { isReadOnlyTool } from "../lib/useController";
import { ReadOnlyBatch } from "./ReadOnlyBatch";
import type { AgentWorkspaceExpertTeam, AgentWorkspaceMemberMap } from "./AgentWorkspace";
import { MediaToolPreview } from "./MediaToolPreview";
import { isGeneratedMediaTool, parseGeneratedMediaResult, parseVideoGenerationProgress } from "../lib/mediaToolResult";
import { parseTaskDispatchOutcome } from "../lib/taskDispatch";
import { recordTranscriptTrace, transcriptTraceOpaqueToken, TRANSCRIPT_TRACE_ENABLED } from "../lib/transcriptPerformanceTrace";
import { useToolDisclosureContext } from "../lib/toolDisclosure";

type ToolItem = Extract<Item, { kind: "tool" }>;

type ToolCardProps = {
  item: ToolItem;
  subcalls?: ToolItem[];
  tabId?: string;
  expertTeam?: AgentWorkspaceExpertTeam | null;
  memberByItemId?: AgentWorkspaceMemberMap;
  childRef?: string;
};

const SUBAGENT_TOOLS = new Set(["task", "parallel_tasks", "run_skill", "explore", "research", "review", "security_review"]);

/** Lines shown by default in a shell output block before the "show all" button. */
const SHELL_PREVIEW_LINES = 10;

function isPromptField(key: string): boolean {
  return key.toLowerCase().includes("prompt");
}

function isEmptyDisplayValue(value: unknown): boolean {
  if (value == null) return true;
  if (Array.isArray(value)) return value.length === 0 || value.every(isEmptyDisplayValue);
  if (typeof value === "object") return Object.keys(value as Record<string, unknown>).length === 0;
  return false;
}

function redactPromptFields(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(redactPromptFields).filter((entry) => !isEmptyDisplayValue(entry));
  }
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
      if (isPromptField(key)) continue;
      const redacted = redactPromptFields(child);
      if (!isEmptyDisplayValue(redacted)) out[key] = redacted;
    }
    return out;
  }
  return value;
}

function displayArgs(json: string, hideUnparsed = false): string {
  if (!json.trim()) return "";
  try {
    const redacted = redactPromptFields(JSON.parse(json) as unknown);
    return isEmptyDisplayValue(redacted) ? "" : JSON.stringify(redacted, null, 2);
  } catch {
    // Tool arguments arrive incrementally while a call is streaming. A partial
    // sub-agent payload cannot be parsed safely, so fail closed until the JSON
    // is complete instead of briefly exposing its internal prompt field.
    return hideUnparsed ? "" : json;
  }
}

function formatToolDuration(ms?: number): string {
  if (typeof ms !== "number" || !Number.isFinite(ms) || ms < 0) return "";
  return `${Math.round(ms)} ms`;
}

/** Returns the first n lines of text and the total line count. */
function splitPreview(text: string, n: number): { preview: string; total: number; hasMore: boolean } {
  const lines = text.split("\n");
  const total = lines.length;
  if (total <= n) return { preview: text, total, hasMore: false };
  return { preview: lines.slice(0, n).join("\n"), total, hasMore: true };
}

function taskLifecycleDisplayItem(item: ToolItem): ToolItem {
  const lifecycle = item.name === "task" ? item.taskLifecycle : undefined;
  if (!lifecycle) return item;
  const status: ToolStatus = lifecycle.turnStatus === "running"
    ? "running"
    : lifecycle.turnStatus === "completed"
      ? "done"
      : lifecycle.turnStatus === "failed"
        ? "error"
        : "stopped";
  const artifactLines = (lifecycle.artifacts ?? []).map((artifact) => `- ${artifact.path}`);
  const identityLines = [
    lifecycle.taskRunId ? `Task run ID: ${lifecycle.taskRunId}` : "",
    lifecycle.dependsOn?.length ? `Dependencies: ${lifecycle.dependsOn.join(", ")}` : "",
  ].filter(Boolean);
  const output = [identityLines.join("\n"), lifecycle.finalAnswer?.trim(), artifactLines.length > 0 ? `Artifacts:\n${artifactLines.join("\n")}` : ""]
    .filter(Boolean)
    .join("\n\n");
  return {
    ...item,
    status,
    output: output || undefined,
    error: lifecycle.error || (status === "error" ? "Subagent turn failed" : undefined),
    dataArchived: false,
  };
}

export const ToolCard = memo(function ToolCard(props: ToolCardProps) {
  if ((props.item.name === "wait_task" || props.item.name === "stop_task") && (props.item.taskLifecycleRefs?.length ?? 0) > 0) {
    return null;
  }
  return <ToolCardContent {...props} item={taskLifecycleDisplayItem(props.item)} />;
});

// ToolCardContent renders one user-visible tool call after lifecycle controls
// have been folded into their owning task card.
function ToolCardContent({
  item,
  subcalls,
  tabId,
  expertTeam,
  memberByItemId,
  childRef,
}: ToolCardProps) {
  const t = useT();
  const isPreparing = item.preparing === true && item.status === "running";
  const nested = subcalls ?? [];
  const hasNested = nested.length > 0;
  const isSubagent = SUBAGENT_TOOLS.has(item.name);
  const isShellTool = item.name === "bash" || Boolean(item.isShell);
  const mcpEvidence = mcpToolEvidence(item.name);
  const profileText =
    isSubagent && item.profile
      ? [item.profile.model, item.profile.effort ? `effort ${item.profile.effort}` : ""].filter(Boolean).join(" · ")
      : "";

  // Tool detail disclosure belongs exclusively to the user. Keep the choice in
  // the transcript's session scope so virtualization and completion-time tool
  // grouping cannot erase it when they remount this card.
  const disclosure = useToolDisclosureContext();
  const contextSetToolOpen = disclosure?.setToolOpen;
  const [localOpen, setLocalOpen] = useState(false);
  // A running parallel call is itself the live progress surface. Keep its
  // compact branch rows visible without requiring an extra disclosure click;
  // completed tool details remain exclusively user-controlled.
  const runningParallelOpen = item.name === "parallel_tasks" && item.status === "running" && hasNested;
  const open = runningParallelOpen || (disclosure ? disclosure.openToolIds.has(item.id) : localOpen);
  const setOpen = useCallback((nextOpen: boolean) => {
    if (contextSetToolOpen) contextSetToolOpen(item.id, nextOpen);
    else setLocalOpen(nextOpen);
  }, [contextSetToolOpen, item.id]);
  const traceTool = useMemo(() => transcriptTraceOpaqueToken(item.id, "t"), [item.id]);
  const [bodyMounted, setBodyMounted] = useState(open);
  const openRef = useRef(open);
  openRef.current = open;
  const [showAll, setShowAll] = useState(false);
  // Lazy-load full tool data from the backend when the card is expanded and
  // the in-memory copy was archived for memory efficiency.
  const [fullData, setFullData] = useState<{ args: string; output?: string } | null>(null);
  const [loadError, setLoadError] = useState(false);
  const archivedWithoutFullData = Boolean(item.dataArchived && !fullData);
  const effectiveArgs = archivedWithoutFullData ? "" : fullData?.args ?? item.args;
  const effectiveOutput = fullData?.output ?? item.output;
  const generatedMedia = useMemo(
    () => isGeneratedMediaTool(item.name) ? parseGeneratedMediaResult(effectiveOutput) : [],
    [effectiveOutput, item.name],
  );
  const videoProgress = useMemo(
    () => item.name === "generate_video" && item.status === "running"
      ? parseVideoGenerationProgress(effectiveOutput)
      : null,
    [effectiveOutput, item.name, item.status],
  );
  const displayOutput = videoProgress ? undefined : effectiveOutput;
  const taskDispatch = item.name === "task" ? parseTaskDispatchOutcome(displayOutput) : null;
  // A collapsed body must stay genuinely cheap. Previously every completed
  // card still parsed its full payload and mounted hidden code/diff trees at
  // height zero, which made tool-heavy session switches rebuild hundreds of
  // invisible nodes in WebView2.
  const visibleArgs = bodyMounted ? displayArgs(effectiveArgs, isSubagent) : "";
  const previewDiff = item.fileDiff?.diff ? item.fileDiff : undefined;
  const diffs = bodyMounted && !previewDiff && !archivedWithoutFullData ? diffsFor(item.name, effectiveArgs) : [];
  const subject = fullData ? subjectOf(item.name, effectiveArgs) : item.subject || subjectOf(item.name, effectiveArgs);
  const previousItemRef = useRef(item);
  useEffect(() => {
    if (previousItemRef.current === item) return;
    previousItemRef.current = item;
    setFullData(null);
  }, [item]);
  useEffect(() => {
    setLoadError(false);
  }, [item.id]);

  // edit diffs are the point of the card, so they're shown inline; everything
  // else folds its args/output away by default.  Open while running so the
  // user sees progress; closed by default once settled.
  const hasArchivedOnDemandBody = Boolean(item.dataArchived && tabId);
  const hasArgsOrOutput = !previewDiff && diffs.length === 0 && (!!visibleArgs || !!displayOutput || hasArchivedOnDemandBody);

  // Shell output: split into preview + "show all" toggle.
  const shellOutput = bodyMounted && isShellTool && displayOutput ? displayOutput : null;
  const shellPreview = shellOutput ? splitPreview(shellOutput, SHELL_PREVIEW_LINES) : null;
  const showRunningNoOutput = isShellTool && item.status === "running" && !isPreparing && !effectiveOutput;
  const hasBody = Boolean(
    previewDiff
    || hasNested
    || effectiveArgs.trim()
    || effectiveOutput
    || showRunningNoOutput
    || hasArchivedOnDemandBody
    || loadError
    || item.error,
  );
  useEffect(() => {
    if ((!open && !isGeneratedMediaTool(item.name)) || !item.dataArchived || fullData || !tabId) return;
    let cancelled = false;
    setLoadError(false);
    const request = childRef
      ? app.SubagentToolResultForTab(tabId, childRef, item.sourceToolId || item.id)
      : app.ToolResultForTab(tabId, item.id);
    request.then((d) => {
      if (!cancelled && d) {
        setFullData(d);
        setLoadError(false);
      }
    }).catch(() => {
      if (!cancelled) setLoadError(true);
    });
    return () => { cancelled = true; };
  }, [childRef, open, item.id, item.name, item.sourceToolId, item.dataArchived, fullData, tabId]);
  useEffect(() => {
    if (open) setBodyMounted(true);
  }, [open]);
  useEffect(() => {
    if (!TRANSCRIPT_TRACE_ENABLED) return;
    recordTranscriptTrace("tool.presentation", {
      tool: traceTool,
      status: item.status,
      open,
      disclosure: "user",
      bodyMounted,
      hasBody,
      nestedCount: nested.length,
      shell: isShellTool,
    }, { persist: true });
  }, [bodyMounted, hasBody, isShellTool, item.status, nested.length, open, traceTool]);
  useEffect(() => {
    if (!TRANSCRIPT_TRACE_ENABLED) return;
    recordTranscriptTrace("tool.mount", { tool: traceTool }, { persist: true });
    return () => recordTranscriptTrace("tool.unmount", { tool: traceTool }, { persist: true });
  }, [traceTool]);

  const toggleOpen = () => {
    if (!hasBody) return;
    const nextOpen = !openRef.current;
    if (nextOpen) setBodyMounted(true);
    recordTranscriptTrace("tool.toggle", { tool: traceTool, source: "pointer", open: nextOpen }, { persist: true });
    setOpen(nextOpen);
  };

  // Register this shell card's toggle with the global ShellExpand context so
  // Ctrl/Cmd+B can expand/collapse the most recent shell output. openRef keeps the
  // registered closure flipping the current state, not a stale one.
  const shellExpand = useShellExpand();
  useEffect(() => {
    if (!item.isShell || !shellExpand) return;
    return shellExpand.register(item.id, () => {
      const nextOpen = !openRef.current;
      if (nextOpen) setBodyMounted(true);
      recordTranscriptTrace("tool.toggle", { tool: traceTool, source: "shortcut", open: nextOpen }, { persist: true });
      setOpen(nextOpen);
    });
  }, [item.isShell, item.id, setOpen, shellExpand, traceTool]);

  // Read-only "research" calls (read/grep/ls/glob/web_fetch) are hidden after
  // completion so they don't clutter the transcript. During execution they still
  // render so the user sees progress.
  const quiet =
    !mcpEvidence && item.readOnly && !hasNested && item.status !== "error" && item.status !== "stopped";

  const duration = item.status === "running" ? "" : formatToolDuration(item.durationMs);
  const dispatchSummary = taskDispatch ? t(`tool.taskDispatch.${taskDispatch.status}`) : "";
  const summary = item.status === "running" ? "" : dispatchSummary || item.summary || summarizeFileDiff(item.fileDiff) || (archivedWithoutFullData ? "" : summarize(item.name, effectiveArgs, effectiveOutput, item.error));

  // GSAP-driven collapse/expand for tool body
  const toolBodyRef = useRef<HTMLDivElement>(null);
  useGSAPCollapse(toolBodyRef, open, {
    onCloseComplete: () => setBodyMounted(false),
  });
  useEffect(() => {
    if (!TRANSCRIPT_TRACE_ENABLED) return;
    const body = toolBodyRef.current;
    if (!body || typeof ResizeObserver === "undefined") return;
    let previousHeight = -1;
    const observer = new ResizeObserver((entries) => {
      const height = Math.round(entries[0]?.contentRect.height ?? body.getBoundingClientRect().height);
      if (height === previousHeight) return;
      previousHeight = height;
      recordTranscriptTrace("tool.body.resize", { tool: traceTool, height, open: openRef.current });
    });
    observer.observe(body);
    return () => observer.disconnect();
  }, [traceTool]);

  return (
    <div className={`tool${quiet ? " tool--quiet" : ""}${isSubagent ? " tool--subagent" : ""}${generatedMedia.length > 0 || videoProgress ? " tool--media" : ""}${open && hasBody ? " tool--open" : ""}`} data-entrance={item.id} data-preparing={isPreparing ? "" : undefined}>
      <button
        type="button"
        className="tool__head"
        data-running={item.status === "running" ? "" : undefined}
        onClick={toggleOpen}
        aria-expanded={hasBody ? open : undefined}
      >
        <span className="tool__label-group">
          {hasNested && <span className="tool__nested-count">⊞{nested.length}</span>}
          {taskDispatch && <span className="tool__status-icon">·</span>}
          {!taskDispatch && item.status === "error" && <span className="tool__status-icon tool__status-icon--err">✗</span>}
          {!taskDispatch && item.status === "done" && <span className="tool__status-icon tool__status-icon--ok">✓</span>}
          {item.status === "stopped" && <span className="tool__status-icon tool__status-icon--stopped">—</span>}
          <span className="tool__name">{mcpEvidence?.label ?? toolDisplayName(item.name)}</span>
          {mcpEvidence && item.status !== "running" && (
            <span className={`tool__mcp-status tool__mcp-status--${item.status}`}>
              状态：{item.status === "done" ? "成功" : item.status === "error" ? "失败" : "已停止"}
            </span>
          )}
          {subject && <span className="tool__subject">{subject}</span>}
          {isPreparing && <span className="tool__subject" role="status">{t("tool.preparing")}</span>}
        </span>
        {profileText && <span className="tool__profile">{profileText}</span>}
        {summary && <span className="tool__summary">{summary}</span>}
        {duration && <span className="tool__duration">{duration}</span>}
        {hasBody && (
          <span className={`tool__chevron${open ? " tool__chevron--open" : ""}`}>
            <ChevronRight size={12} />
          </span>
        )}
        {item.status !== "running" && (
          <span
            className={`tool__dot${taskDispatch ? " tool__dot--stopped" : item.status === "done" ? " tool__dot--ok" : ""}${!taskDispatch && item.status === "error" ? " tool__dot--err" : ""}${!taskDispatch && item.status === "stopped" ? " tool__dot--stopped" : ""}`}
            aria-hidden="true"
          />
        )}
      </button>

      {generatedMedia.length > 0 && <MediaToolPreview media={generatedMedia} tabId={tabId} />}
      {videoProgress && (
        <div className="tool-video-progress" role="status" aria-live="polite">
          <div className="tool-video-progress__head">
            <span>{t("tool.videoProgress")}</span>
            <strong>{videoProgress.progressPercent}%</strong>
          </div>
          <div
            className="tool-video-progress__track"
            role="progressbar"
            aria-label={t("tool.videoProgress")}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={videoProgress.progressPercent}
          >
            <span style={{ width: `${videoProgress.progressPercent}%` }} />
          </div>
          <div className="tool-video-progress__meta">
            <span>{t("tool.videoTaskId")}<code>{videoProgress.taskId}</code></span>
            {videoProgress.status && <span>{videoProgress.status}</span>}
          </div>
        </div>
      )}

      <div ref={toolBodyRef} className="tool__body">
        {bodyMounted && <>

        {previewDiff ? (
          <DiffView diff={previewDiff.diff} language={languageForToolArgs(fullData?.args ?? item.args)} maxHeight={260} />
        ) : (
          diffs.map((d, i) => (
            <div key={i}>
              {d.label && <div className="tool__difflabel">{d.label}</div>}
              <DiffView original={d.original} modified={d.modified} language={d.lang} maxHeight={260} />
            </div>
          ))
        )}

        {hasNested && (
          <div className="tool__nested">
            {item.name === "parallel_tasks" ? (
              <div className="parallel-task-list" data-testid="parallel-task-list">
                {nested.map((branch, index) => (
                  <div key={branch.id} className="parallel-task-list__row">
                    <span className={`tool__status-icon${branch.status === "error" ? " tool__status-icon--err" : branch.status === "done" ? " tool__status-icon--ok" : ""}`}>
                      {branch.status === "running" ? "…" : branch.status === "done" ? "✓" : branch.status === "error" ? "✗" : "—"}
                    </span>
                    <span className="parallel-task-list__label">
                      {branch.summary || summarize(branch.name, branch.args) || `Task ${index + 1}`}
                    </span>
                  </div>
                ))}
              </div>
            ) : (() => {
              const out: ReactNode[] = [];
              const roBatch: typeof nested = [];
              const flush = () => {
                if (roBatch.length === 0) return;
                out.push(<ReadOnlyBatch key={`rob-${roBatch[0].id}`} items={[...roBatch]} subcalls={new Map()} tabId={tabId} />);
                roBatch.length = 0;
              };
              for (const c of nested) {
                if (isReadOnlyTool(c.name) && c.name !== "todo_write") {
                  roBatch.push(c);
                  continue;
                }
                flush();
                out.push(<ToolCard key={c.id} item={c} tabId={tabId} expertTeam={expertTeam} memberByItemId={memberByItemId} />);
              }
              flush();
              return out;
            })()}
          </div>
        )}
        {shellPreview && (
          <>
            <CodeViewer value={showAll ? shellOutput! : shellPreview.preview} maxHeight={showAll ? 480 : 260} />
            {shellPreview.hasMore && !showAll && (
              <button className="tool__showall" onClick={() => setShowAll(true)}>
                {t("tool.showAllLines", { n: shellPreview.total })}
              </button>
            )}
            {item.truncated && <div className="tool__note">{t("tool.truncated")}</div>}
          </>
        )}

        {showRunningNoOutput && (
          <div className="tool__running-note">{t("tool.runningNoOutput")}</div>
        )}

        {!shellPreview && hasArgsOrOutput && (
          <>
            {visibleArgs && <CodeViewer value={visibleArgs} language="json" maxHeight={180} />}
            {displayOutput && (
              <>
                <CodeViewer value={displayOutput} maxHeight={280} />
                {item.truncated && <div className="tool__note">{t("tool.truncated")}</div>}
              </>
            )}
          </>
        )}

        {item.error && <div className="tool__err">{item.error}</div>}
        {loadError && (
          <div className="tool__err" role="alert">
            {t("tool.archivedLoadError")}
          </div>
        )}
        </>}
      </div>
    </div>
  );
}
