import { createContext, memo, type CSSProperties, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent as ReactMouseEvent, type PointerEvent as ReactPointerEvent, type ReactNode, type WheelEvent as ReactWheelEvent, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { Item, LiveStream, SessionScrollAnchor } from "../lib/useController";
import type { CheckpointMeta, ExpertTeamSession, Scene, SceneState, TurnMetadata } from "../lib/types";
import { useT } from "../lib/i18n";
import { AssistantMessage, TurnActions, UserMessage } from "./Message";
import { ProcessCompactIcon, ProcessPhaseIcon } from "./ProcessCard";
import { ToolCard } from "./ToolCard";
import { ArrowDown, ChevronRight } from "lucide-react";
import { Welcome, type CapabilityWelcomePresentation, type CreationScenarioSkill, type SceneSelectionAvailability } from "./Welcome";
import { ReadOnlyBatch } from "./ReadOnlyBatch";
import { ToolGroup, isCreationGroupableTool, toolGroupKind, type ToolGroupKind } from "./ToolGroup";
import { getDisplayMode, onDisplayModeChange, type DisplayMode } from "../lib/displayMode";
import { buildExpertTeamFlowModel, isExpertDelegationTool, type ExpertTeamActiveSelection, type ExpertTeamFlowModel } from "../lib/expertTeamFlow";
import { isRestorableExpertTool, restoreArchivedExpertTool, type RestoredExpertToolData } from "../lib/expertWorkspaceHistory";
import { stripHiddenSystemPrompt } from "../lib/expertPrompt";
import { app } from "../lib/bridge";
import type { TaskTreeState } from "../lib/taskTreeState";
import { isReadOnlyTool } from "../lib/useController";
import { useGSAPCollapse } from "../lib/useGSAPCollapse";
import { useEntranceAnimation } from "../lib/useEntranceAnimation";
import { useScrollManager } from "../lib/useScrollManager";
import { captureTranscriptAnchor, captureVirtualTranscriptAnchor, consumePresentationRestoreSnapshot, findTranscriptAnchorNode, remapTranscriptMiddleAnchor, restoreTranscriptAnchor, scrollTopForTranscriptNode, shouldAutoScrollTranscriptQuestions, stagePresentationRestoreSnapshot, syncPresentationRestoreSnapshot, type TranscriptCommittedPresentation, type TranscriptPresentationRestoreSnapshot, type TranscriptSemanticPosition } from "../lib/transcriptScrollAnchor";
import { TranscriptRestoreWindow } from "../lib/transcriptRestoreWindow";
import { buildTurnGroups, compactQuestionText, questionAnchorId, scrollVersion, warmUserPreview, type QuestionAnchor, type TurnGroup } from "../lib/transcriptGrouping";
import { recordTranscriptTrace, TRANSCRIPT_TRACE_ENABLED, transcriptTraceMemorySnapshot, transcriptTraceOpaqueToken, transcriptTraceViewToken } from "../lib/transcriptPerformanceTrace";
import { ExpertTeamFlowCard } from "./ExpertTeamFlowCard";
import { AgentWorkspace, type AgentWorkspaceExpertMember, type AgentWorkspaceExpertTeam, type AgentWorkspaceMemberMap } from "./AgentWorkspace";
import { generatedMediaByAssistant, isGeneratedMediaTool, type GeneratedMediaReference } from "../lib/mediaToolResult";
import { ExpertTeamSessionView } from "./ExpertTeamSession";
import { shouldVirtualizeTranscriptRows, TranscriptTraceViewContext, transcriptVirtualRows, type TranscriptVirtualRow, type TranscriptVirtualRowsHandle } from "./TranscriptVirtualRows";
import { presentCompactionTrigger, presentRuntimeEventText } from "../features/session/runtimeNoticePresentation";
import { consumeNoticeReveal, isSilentMCPAvailabilityNotice, noticeNeedsReveal } from "../features/session/sessionItems";
import { SessionLoadingPresentation, useSessionLoadingFeedback } from "./SessionLoadingPresentation";
import { ToolDisclosureProvider } from "../lib/toolDisclosure";
import { selectTaskNoticeItems, taskFailureCount } from "../features/session/taskNoticePresentation";

export { TRANSCRIPT_VIRTUALIZE_MIN_ROWS, shouldVirtualizeTranscriptRows } from "./TranscriptVirtualRows";

type ToolItem = Extract<Item, { kind: "tool" }>;
type AssistantItem = Extract<Item, { kind: "assistant" }>;
type UserItem = Extract<Item, { kind: "user" }>;
type OpenTurnAction = { turn: number; menu: "summary" | "rewind" };
type CompactTurn = {
  user?: UserItem;
  processItems: Item[];
  finalAssistant?: AssistantItem;
};
type TranscriptActiveExpert = ExpertTeamActiveSelection & {
  avatarSrc?: string;
  members?: readonly AgentWorkspaceExpertMember[];
};

let transcriptTraceInstanceSequence = 0;
let transcriptTraceWheelBatchSequence = 0;
type TranscriptWheelTraceState = {
  batch: number;
  view: string;
  startedAt: number;
  expiredAt: number | null;
  frame: number;
};
const transcriptWheelTraceStateByInstance = TRANSCRIPT_TRACE_ENABLED
  ? new Map<number, TranscriptWheelTraceState>()
  : null;

function forceRevealNotice(items: Item[]): Extract<Item, { kind: "notice" }> | undefined {
  for (let index = items.length - 1; index >= 0; index--) {
    const item = items[index];
    if (noticeNeedsReveal(item)) return item;
  }
  return undefined;
}

function normalizedExpertMemberName(value?: string): string {
  return value?.trim().toLocaleLowerCase() ?? "";
}

const QUESTION_NAV_MIN_COUNT = 2;
const SCROLL_ANCHOR_RESTORE_FRAMES = 5;
const EXPERT_TEAM_LOAD_RETRY_DELAY_MS = 250;
const EXPERT_TEAM_LOAD_MAX_ATTEMPTS = 3;
const GENERATED_MEDIA_LOAD_RETRY_DELAY_MS = 250;
const GENERATED_MEDIA_LOAD_MAX_ATTEMPTS = 3;
const noArchivedGeneratedMediaOutputs: ReadonlyMap<string, string> = new Map();
const LiveStreamContext = createContext<LiveStream | undefined>(undefined);

function useArchivedGeneratedMediaOutputs(
  items: readonly Item[],
  tabId: string | undefined,
  viewKey: string,
): ReadonlyMap<string, string> {
  const [state, setState] = useState<{ viewKey: string; outputs: ReadonlyMap<string, string> }>(() => ({
    viewKey,
    outputs: new Map(),
  }));
  const stateRef = useRef(state);
  stateRef.current = state;
  const pendingRequestsRef = useRef(new Set<string>());

  useEffect(() => {
    pendingRequestsRef.current.clear();
  }, [viewKey]);

  useEffect(() => {
    if (!tabId) return;
    const currentOutputs = stateRef.current.viewKey === viewKey
      ? stateRef.current.outputs
      : noArchivedGeneratedMediaOutputs;
    const candidates = items.filter((item): item is ToolItem => (
      item.kind === "tool"
      && isGeneratedMediaTool(item.name)
      && item.dataArchived === true
      && !item.output
      && !item.parentId
      && !currentOutputs.has(item.id)
      && !pendingRequestsRef.current.has(`${viewKey}\u0000${item.id}`)
    ));
    if (candidates.length === 0) return;

    let cancelled = false;
    const retryTimers = new Set<number>();
    const requestKeys = new Set<string>();
    const load = (item: ToolItem, attempt: number) => {
      const requestKey = `${viewKey}\u0000${item.id}`;
      pendingRequestsRef.current.add(requestKey);
      requestKeys.add(requestKey);
      void app.ToolResultForTab(tabId, item.id).catch(() => null).then((data) => {
        if (cancelled) return;
        const output = data?.output?.trim() ? data.output : "";
        if (output) {
          setState((current) => {
            const outputs = current.viewKey === viewKey
              ? new Map(current.outputs)
              : new Map<string, string>();
            outputs.set(item.id, output);
            return { viewKey, outputs };
          });
          return;
        }
        if (attempt >= GENERATED_MEDIA_LOAD_MAX_ATTEMPTS) return;
        const timer = window.setTimeout(() => {
          retryTimers.delete(timer);
          if (!cancelled) load(item, attempt + 1);
        }, GENERATED_MEDIA_LOAD_RETRY_DELAY_MS);
        retryTimers.add(timer);
      });
    };
    for (const item of candidates) load(item, 1);
    return () => {
      cancelled = true;
      for (const timer of retryTimers) window.clearTimeout(timer);
      for (const requestKey of requestKeys) pendingRequestsRef.current.delete(requestKey);
    };
  }, [items, tabId, viewKey]);

  return state.viewKey === viewKey ? state.outputs : noArchivedGeneratedMediaOutputs;
}

export function transcriptVirtualHotRowOffset(expertWorkspaceActive: boolean, regularPrefixRows: number): number {
  return expertWorkspaceActive ? 0 : regularPrefixRows;
}

const LiveAssistantMessage = memo(function LiveAssistantMessage({
  item,
  defaultExpanded = false,
  expandWhileStreaming = true,
  truncateStreamingReasoning = false,
  creationMode = false,
  tabId,
  generatedMedia,
}: {
  item: AssistantItem;
  defaultExpanded?: boolean;
  expandWhileStreaming?: boolean;
  truncateStreamingReasoning?: boolean;
  creationMode?: boolean;
  tabId?: string;
  generatedMedia?: readonly GeneratedMediaReference[];
}) {
  const live = useContext(LiveStreamContext);
  const shown = useMemo(
    () =>
      live && live.id === item.id
        ? { ...item, text: live.text, reasoning: live.reasoning, streaming: true, reasoningComplete: live.reasoningComplete }
        : item,
    [item, live?.id, live?.text, live?.reasoning, live?.reasoningComplete],
  );
  return (
    <AssistantMessage
      item={shown}
      defaultExpanded={defaultExpanded}
      expandWhileStreaming={expandWhileStreaming}
      truncateStreamingReasoning={truncateStreamingReasoning}
      creationMode={creationMode}
      tabId={tabId}
      generatedMedia={generatedMedia}
    />
  );
});

// ── Layer budgets ─────────────────────────────────────────────────────────────
// Hot zone: the most recent N user turns are always fully rendered. All data
// stays in memory (items[]), so expanding a warm turn is instant — no API call.
// Cold zone: a "load more" button paginates the warm zone in batches.
//
//   items[0]  ─┐
//   ...        │ Cold zone  ───  paginated, shown on "load more"
//              ├────────────  warmTurnStart
//   ...        │ Warm zone  ───  collapsible summary cards (individual expand)
//              ├────────────  hotStartIdx
//   items[N]  ─┤ Hot zone   ───  fully rendered
//   ...        │
//   items[end] ┘

const HOT_TURNS = 30;
const WARM_PAGE_SIZE = 20; // cold-zone pagination batch
const PROJECT_DRAG_SCROLL_THRESHOLD = 6;
const PROJECT_DRAG_SCROLL_EXCLUSION_SELECTOR = "a,button,input,textarea,select,option,[role='button'],[contenteditable='true'],pre,code";
// ── Helpers ───────────────────────────────────────────────────────────────────

// ── Transcript component ──────────────────────────────────────────────────────

export function Transcript({
  items: sourceItems,
  parallelItemsByCall,
  live,
  tabId,
  viewKey,
  initialScrollAnchor,
  onScrollAnchorChange,
  historyRevalidationSignal = 0,
  hasOlderHistory = false,
  loadingOlderHistory = false,
  onLoadOlderHistory,
  footerHeight = 0,
  onPrompt,
  onSelectScenario,
  scene,
  sceneState,
  sceneSelectionAvailability,
  onSelectScene,
  onEditPrompt,
  onRewind,
  checkpoints = [],
  actionPending = false,
  rewindDisabled = false,
  running = false,
  questionNavigator = true,
  welcomeVariant = "default",
  capabilityWelcomePresentation,
  creationMode = false,
  actionHoverMenus = false,
  rewindSignal = 0,
  revealSignal = 0,
  hydrating = false,
  showWelcomeDuringHydration = false,
  existingSessionLoading = false,
  dragScroll = false,
  taskTrees,
  activeExpert,
  expertSelectionRevision = 0,
  childItemsByRef = {},
  subagentStatusByRef = {},
  onExpertMemberHistoryFocusChange,
}: {
  items: Item[];
  parallelItemsByCall?: Record<string, ToolItem[]>;
  live?: LiveStream;
  tabId?: string;
  viewKey?: string;
  initialScrollAnchor?: SessionScrollAnchor;
  onScrollAnchorChange?: (key: string, anchor: SessionScrollAnchor) => void;
  historyRevalidationSignal?: number;
  hasOlderHistory?: boolean;
  loadingOlderHistory?: boolean;
  onLoadOlderHistory?: () => void | Promise<void>;
  footerHeight?: number;
  onPrompt: (text: string) => void;
  onSelectScenario?: (text: string, skill?: CreationScenarioSkill) => void;
  scene?: Scene;
  sceneState?: SceneState;
  sceneSelectionAvailability?: SceneSelectionAvailability;
  onSelectScene?: (scene: Scene) => Promise<void>;
  onEditPrompt?: (turn: number, displayText: string, submitText?: string) => boolean | void | Promise<boolean | void>;
  onRewind?: (turn: number, scope: string) => void;
  checkpoints?: CheckpointMeta[];
  actionPending?: boolean;
  rewindDisabled?: boolean;
  running?: boolean;
  questionNavigator?: boolean;
  welcomeVariant?: "default" | "creation";
  capabilityWelcomePresentation?: CapabilityWelcomePresentation;
  creationMode?: boolean;
  actionHoverMenus?: boolean;
  rewindSignal?: number;
  revealSignal?: number;
  hydrating?: boolean;
  showWelcomeDuringHydration?: boolean;
  existingSessionLoading?: boolean;
  dragScroll?: boolean;
  taskTrees?: TaskTreeState;
  activeExpert?: TranscriptActiveExpert | null;
  expertSelectionRevision?: number;
  childItemsByRef?: Record<string, Item[]>;
  subagentStatusByRef?: Record<string, string>;
  onExpertMemberHistoryFocusChange?: (focused: boolean, returnToLead?: () => void) => void;
}) {
  // Legacy team workspaces can relocate task rows out of the lead timeline.
  // Keep their notices until the owner is available in the persisted view.
  const items = useMemo(() => activeExpert?.mode === "team" && !tabId
    ? sourceItems
    : selectTaskNoticeItems(sourceItems), [activeExpert?.mode, sourceItems, tabId]);
  const traceInstanceRef = useRef(0);
  if (traceInstanceRef.current === 0) {
    traceInstanceRef.current = ++transcriptTraceInstanceSequence;
  }
  const traceInstance = traceInstanceRef.current;
  const t = useT();
  const semanticViewKey = viewKey || tabId || "";
  const traceView = useMemo(() => transcriptTraceViewToken(semanticViewKey), [semanticViewKey]);
  useEffect(() => {
    recordTranscriptTrace("transcript.mount", { instance: traceInstance }, { persist: true });
    return () => recordTranscriptTrace("transcript.unmount", { instance: traceInstance }, { persist: true });
  }, [traceInstance]);
  const {
    scrollRef,
    stick,
    onScroll,
    isAtBottom,
    smoothScrollTo,
    scrollToBottomAfterLayout,
    pinToBottomAfterLayout,
    trackQuestions,
    repinIfWasPinned,
    resizeFrame,
    lastClientHeight,
    lastFooterHeight,
  } = useScrollManager(traceView, semanticViewKey);
  const [scrollElement, setScrollElement] = useState<HTMLDivElement | null>(null);
  const bindScrollElement = useCallback((node: HTMLDivElement | null) => {
    scrollRef.current = node;
    if (node) setScrollElement((current) => current === node ? current : node);
  }, [scrollRef]);
  const pendingRevealBottomScroll = useRef(false);
  const pendingScrollAnchor = useRef<Extract<SessionScrollAnchor, { atBottom: false }> | null>(null);
  const pendingScrollViewKey = useRef("");
  const [scrollAnchorRestoreGeneration, setScrollAnchorRestoreGeneration] = useState(0);
  const [revealedSemanticViewKey, setRevealedSemanticViewKey] = useState(
    initialScrollAnchor?.atBottom === false ? "" : semanticViewKey,
  );
  const [semanticRestoreFailureViewKey, setSemanticRestoreFailureViewKey] = useState("");
  const restoreWindow = useRef<TranscriptRestoreWindow | null>(null);
  const restoreFinishedViewKey = useRef<string | null>(null);
  const protectedRestorePosition = useRef<TranscriptSemanticPosition | null>(null);
  const suppressReplacementCaptureDuringReveal = useRef(false);
  const hasRevealedPresentation = useRef(initialScrollAnchor?.atBottom !== false);
  const revealSemanticPresentation = useCallback(() => {
    hasRevealedPresentation.current = true;
    suppressReplacementCaptureDuringReveal.current = true;
    const presentation = persistedPresentationRef.current;
    presentation?.removeAttribute("aria-hidden");
    if (presentation) {
      presentation.inert = false;
      presentation.style.visibility = "";
      presentation.style.pointerEvents = "";
    }
    setRevealedSemanticViewKey(semanticViewKey);
  }, [semanticViewKey]);
  const latestSemanticPosition = useRef<TranscriptSemanticPosition | null>(null);
  const presentationRestoreSnapshot = useRef<TranscriptPresentationRestoreSnapshot | null>(null);
  const committedPresentation = useRef<TranscriptCommittedPresentation>({ viewKey: semanticViewKey, path: "normal" });
  const expertReturnPresentationDeferred = useRef(false);
  const deferredExpertReturnInputOnly = useRef(false);
  const deferredExpertReturnRevealBottom = useRef(false);
  const committedRowsVirtualization = useRef({ viewKey: semanticViewKey, expertWorkspaceActive: false, expertVirtualized: false, ordinaryVirtualized: false, contentVersion: "", initialized: false });
  const preservePinnedVirtualizationReset = useRef(false);
  const preservePinnedVirtualizationResetFrame = useRef<number | null>(null);
  const userScrollIntent = useRef(false);
  const userScrollGestureActive = useRef(false);
  const userPresentationSelectionIntent = useRef(false);
  const expertMemberFocused = useRef(false);
  const projectDragScrollRef = useRef<{
    pointerId: number;
    startY: number;
    startScrollTop: number;
    dragging: boolean;
  } | null>(null);
  const suppressProjectDragClickRef = useRef(false);
  const wheelScrollGestureActive = useRef(false);
  const wheelScrollGestureFrame = useRef<number | null>(null);
  const wheelScrollGestureViewKey = useRef("");
  const wheelScrollGestureDeltaY = useRef(0);
  const wheelScrollGestureAnchor = useRef<TranscriptSemanticPosition | null>(null);
  const wheelScrollClampCandidate = useRef<TranscriptSemanticPosition | null>(null);
  const virtualRowsRef = useRef<TranscriptVirtualRowsHandle>(null);
  const virtualLayoutReplacementAnchor = useRef<TranscriptSemanticPosition | null>(null);
  const persistedPresentationRef = useRef<HTMLDivElement | null>(null);
  const replacementPresentationHidden = useRef(false);
  const transcriptMounted = useRef(true);
  const detachedVirtualPresentation = useRef<{
    scope: object;
    position: TranscriptSemanticPosition | null;
    imperativelyHidden: boolean;
    wasDeferred: boolean;
  } | null>(null);
  const bindVirtualRows = useCallback((handle: TranscriptVirtualRowsHandle | null) => {
    if (restoreFinishedViewKey.current === pendingScrollViewKey.current) {
      virtualRowsRef.current = handle;
      detachedVirtualPresentation.current = null;
      virtualLayoutReplacementAnchor.current = null;
      return;
    }
    if (!handle && !transcriptMounted.current) {
      virtualRowsRef.current = null;
      detachedVirtualPresentation.current = null;
      return;
    }
    const previous = virtualRowsRef.current;
    virtualRowsRef.current = handle;
    const detached = detachedVirtualPresentation.current;
    if (handle && detached) {
      detachedVirtualPresentation.current = null;
      if (handle.scope === detached.scope) {
        virtualLayoutReplacementAnchor.current = null;
        if (detached.imperativelyHidden) {
          replacementPresentationHidden.current = false;
          expertReturnPresentationDeferred.current = detached.wasDeferred;
          const presentation = persistedPresentationRef.current;
          presentation?.removeAttribute("aria-hidden");
          if (presentation) {
            presentation.inert = false;
            presentation.style.visibility = "";
            presentation.style.pointerEvents = "";
          }
        }
        return;
      }
      if (userPresentationSelectionIntent.current) {
        userPresentationSelectionIntent.current = false;
        virtualLayoutReplacementAnchor.current = null;
        if (detached.imperativelyHidden) {
          replacementPresentationHidden.current = false;
          expertReturnPresentationDeferred.current = detached.wasDeferred;
          const presentation = persistedPresentationRef.current;
          presentation?.removeAttribute("aria-hidden");
          if (presentation) {
            presentation.inert = false;
            presentation.style.visibility = "";
            presentation.style.pointerEvents = "";
          }
        }
        return;
      }
      const queuedPosition = detached.position;
      const presentation = persistedPresentationRef.current;
      if (queuedPosition && presentation) {
        pendingScrollViewKey.current = queuedPosition.viewKey;
        pendingScrollAnchor.current = queuedPosition.anchor;
        setSemanticRestoreFailureViewKey("");
        setScrollAnchorRestoreGeneration((generation) => generation + 1);
      }
      return;
    }
    if (previous && !handle) {
      if (suppressReplacementCaptureDuringReveal.current) {
        detachedVirtualPresentation.current = null;
        return;
      }
      if (userPresentationSelectionIntent.current) {
        userPresentationSelectionIntent.current = false;
        detachedVirtualPresentation.current = null;
        virtualLayoutReplacementAnchor.current = null;
        return;
      }
      const canonicalPosition = latestSemanticPosition.current;
      virtualLayoutReplacementAnchor.current = canonicalPosition;
      const presentation = persistedPresentationRef.current;
      const wasDeferred = expertReturnPresentationDeferred.current;
      const imperativelyHidden = Boolean(!hasRevealedPresentation.current && canonicalPosition && presentation && presentation.getAttribute("aria-hidden") !== "true");
      if (imperativelyHidden && presentation) {
        replacementPresentationHidden.current = true;
        expertReturnPresentationDeferred.current = true;
        presentation.setAttribute("aria-hidden", "true");
        presentation.inert = true;
        presentation.style.visibility = "hidden";
        presentation.style.pointerEvents = "none";
      }
      const detachedPresentation = {
        scope: previous.scope,
        position: canonicalPosition,
        imperativelyHidden,
        wasDeferred,
      };
      detachedVirtualPresentation.current = detachedPresentation;
      queueMicrotask(() => {
        if (!transcriptMounted.current || detachedVirtualPresentation.current !== detachedPresentation || virtualRowsRef.current) return;
        if (restoreFinishedViewKey.current === pendingScrollViewKey.current) return;
        detachedVirtualPresentation.current = null;
        if (userPresentationSelectionIntent.current) {
          userPresentationSelectionIntent.current = false;
          virtualLayoutReplacementAnchor.current = null;
          if (imperativelyHidden && presentation) {
            replacementPresentationHidden.current = false;
            expertReturnPresentationDeferred.current = wasDeferred;
            presentation.removeAttribute("aria-hidden");
            presentation.inert = false;
            presentation.style.visibility = "";
            presentation.style.pointerEvents = "";
          }
          return;
        }
        if (!canonicalPosition || !presentation) return;
        pendingScrollViewKey.current = canonicalPosition.viewKey;
        pendingScrollAnchor.current = canonicalPosition.anchor;
        setSemanticRestoreFailureViewKey("");
        setScrollAnchorRestoreGeneration((generation) => generation + 1);
      });
    }
  }, []);
  useLayoutEffect(() => {
    transcriptMounted.current = true;
    return () => {
      transcriptMounted.current = false;
      detachedVirtualPresentation.current = null;
    };
  }, []);
  const traceViewRef = useRef(traceView);
  traceViewRef.current = traceView;
  const virtualQuestionRowByAnchorIDRef = useRef(new Map<string, number>());
  const virtualHotRowOffsetRef = useRef(0);
  const lastTrackedViewKey = useRef<string | undefined>(undefined);
  const lastHistoryRevalidationSignal = useRef(historyRevalidationSignal);
  const previousQuestions = useRef({ viewKey: semanticViewKey, questions: [] as QuestionAnchor[] });
  const sessionKey = useMemo(() => `${items[0]?.id ?? ""}|${items[items.length - 1]?.id ?? ""}`, [items]);
  const entranceRef = useEntranceAnimation<HTMLDivElement>(sessionKey, items.length);

  const [displayMode, setDisplayMode] = useState<DisplayMode>(() => getDisplayMode());
  useEffect(() => onDisplayModeChange((mode) => setDisplayMode(mode)), []);

  const questions = useMemo<QuestionAnchor[]>(() => {
    const anchors: QuestionAnchor[] = [];
    let turn = 0;
    for (const it of items) {
      if (it.kind !== "user") continue;
      anchors.push({ id: it.id, text: compactQuestionText(it.text), turn });
      turn += 1;
    }
    return anchors;
  }, [items]);
  const questionsRef = useRef(questions);
  const questionIdentity = useMemo(() => questions.map((question) => question.id).join("\u0000"), [questions]);
  const hasItems = items.length > 0;
  useLayoutEffect(() => {
    questionsRef.current = questions;
  }, [questions]);
  const writeScrollAnchor = useCallback((anchor: SessionScrollAnchor, traceQuestions: readonly QuestionAnchor[]) => {
    if (protectedRestorePosition.current?.viewKey === semanticViewKey) return;
    if (!viewKey || !onScrollAnchorChange) return;
    onScrollAnchorChange(viewKey, anchor);
    if (TRANSCRIPT_TRACE_ENABLED) {
      const questionIndex = anchor.atBottom
        ? -1
        : traceQuestions.findIndex((question) => question.id === anchor.anchorMessageId);
      recordTranscriptTrace("anchor.cache.write", {
        view: traceView,
        mode: anchor.atBottom ? "bottom" : "semantic",
        anchor: anchor.atBottom ? undefined : transcriptTraceOpaqueToken(anchor.anchorMessageId, "a"),
        questionIndex,
        offset: anchor.atBottom ? 0 : anchor.offset,
      });
    }
  }, [onScrollAnchorChange, traceView, viewKey]);
  useLayoutEffect(() => {
    const previous = previousQuestions.current;
    previousQuestions.current = { viewKey: semanticViewKey, questions };
    if (previous.viewKey !== semanticViewKey || previous.questions.length === 0) return;
    const remap = (anchor: Extract<SessionScrollAnchor, { atBottom: false }>) => (
      remapTranscriptMiddleAnchor(previous.questions, questions, anchor)
    );
    if (pendingScrollAnchor.current) pendingScrollAnchor.current = remap(pendingScrollAnchor.current);
    if (protectedRestorePosition.current?.viewKey === semanticViewKey) {
      protectedRestorePosition.current = { viewKey: semanticViewKey, anchor: remap(protectedRestorePosition.current.anchor) };
    }
    if (latestSemanticPosition.current?.viewKey === semanticViewKey) {
      const anchor = remap(latestSemanticPosition.current.anchor);
      latestSemanticPosition.current = { viewKey: semanticViewKey, anchor };
      writeScrollAnchor(anchor, questions);
    }
    if (presentationRestoreSnapshot.current?.viewKey === semanticViewKey && presentationRestoreSnapshot.current.anchor) {
      presentationRestoreSnapshot.current = {
        viewKey: semanticViewKey,
        anchor: remap(presentationRestoreSnapshot.current.anchor),
      };
    }
    if (wheelScrollGestureAnchor.current?.viewKey === semanticViewKey) {
      wheelScrollGestureAnchor.current = {
        viewKey: semanticViewKey,
        anchor: remap(wheelScrollGestureAnchor.current.anchor),
      };
    }
    if (wheelScrollClampCandidate.current?.viewKey === semanticViewKey) {
      wheelScrollClampCandidate.current = {
        viewKey: semanticViewKey,
        anchor: remap(wheelScrollClampCandidate.current.anchor),
      };
    }
  }, [questions, semanticViewKey, writeScrollAnchor]);
  const showQuestionNav = questionNavigator && questions.length >= QUESTION_NAV_MIN_COUNT;

  // Track question count and auto-scroll on new messages.
  useEffect(() => {
    const autoScroll = shouldAutoScrollTranscriptQuestions(
      lastTrackedViewKey.current,
      semanticViewKey,
      lastHistoryRevalidationSignal.current,
      historyRevalidationSignal,
    );
    lastTrackedViewKey.current = semanticViewKey;
    lastHistoryRevalidationSignal.current = historyRevalidationSignal;
    trackQuestions(questions.length, autoScroll);
  }, [historyRevalidationSignal, questions.length, semanticViewKey, trackQuestions]);

  const clearWheelScrollGesture = useCallback(() => {
    wheelScrollGestureActive.current = false;
    wheelScrollGestureViewKey.current = "";
    wheelScrollGestureDeltaY.current = 0;
    wheelScrollGestureAnchor.current = null;
    wheelScrollClampCandidate.current = null;
    if (TRANSCRIPT_TRACE_ENABLED) transcriptWheelTraceStateByInstance?.delete(traceInstance);
    if (wheelScrollGestureFrame.current !== null) {
      cancelAnimationFrame(wheelScrollGestureFrame.current);
      wheelScrollGestureFrame.current = null;
    }
  }, [traceInstance]);
  const beginWheelScrollGesture = useCallback((event: ReactWheelEvent<HTMLDivElement>) => {
    restoreFinishedViewKey.current = pendingScrollViewKey.current;
    protectedRestorePosition.current = null;
    setSemanticRestoreFailureViewKey("");
    let restarting = false;
    const previousTraceState = TRANSCRIPT_TRACE_ENABLED
      ? transcriptWheelTraceStateByInstance?.get(traceInstance)
      : undefined;
    if (TRANSCRIPT_TRACE_ENABLED) restarting = wheelScrollGestureActive.current;
    clearWheelScrollGesture();
    pendingScrollAnchor.current = null;
    presentationRestoreSnapshot.current = null;
    virtualLayoutReplacementAnchor.current = null;
    detachedVirtualPresentation.current = null;
    if (expertReturnPresentationDeferred.current) {
      latestSemanticPosition.current = null;
      deferredExpertReturnInputOnly.current = true;
      revealSemanticPresentation();
    }
    wheelScrollGestureActive.current = true;
    wheelScrollGestureViewKey.current = semanticViewKey;
    wheelScrollGestureDeltaY.current = event.deltaY;
    let traceBatch: number | undefined;
    if (TRANSCRIPT_TRACE_ENABLED) {
      traceBatch = ++transcriptTraceWheelBatchSequence;
      transcriptWheelTraceStateByInstance?.set(traceInstance, {
        batch: traceBatch,
        view: traceView,
        startedAt: Date.now(),
        expiredAt: null,
        frame: 0,
      });
      recordTranscriptTrace(restarting ? "wheel.restart" : "wheel.begin", {
        view: traceView,
        batch: traceBatch,
        previousBatch: restarting ? previousTraceState?.batch ?? 0 : 0,
        frame: 0,
        elapsedMs: 0,
        deltaY: event.deltaY,
        deltaMode: event.deltaMode,
      });
    }
    let framesLeft = SCROLL_ANCHOR_RESTORE_FRAMES;
    const expire = () => {
      wheelScrollGestureFrame.current = requestAnimationFrame(() => {
        wheelScrollGestureFrame.current = null;
        framesLeft -= 1;
        if (TRANSCRIPT_TRACE_ENABLED) {
          const traceState = transcriptWheelTraceStateByInstance?.get(traceInstance);
          if (traceState && traceState.batch === traceBatch) traceState.frame += 1;
        }
        if (framesLeft > 0 && wheelScrollGestureActive.current) {
          expire();
          return;
        }
        if (TRANSCRIPT_TRACE_ENABLED) {
          const expiredAt = Date.now();
          const traceState = transcriptWheelTraceStateByInstance?.get(traceInstance);
          if (traceState && traceState.batch === traceBatch) {
            traceState.expiredAt = expiredAt;
            recordTranscriptTrace("wheel.expire", {
              view: traceState.view,
              batch: traceState.batch,
              frame: traceState.frame,
              elapsedMs: Math.max(0, expiredAt - traceState.startedAt),
            });
          }
        }
        const clampCandidate = wheelScrollClampCandidate.current;
        const committedViewKey = committedPresentation.current.viewKey;
        wheelScrollGestureActive.current = false;
        wheelScrollGestureViewKey.current = "";
        wheelScrollGestureDeltaY.current = 0;
        wheelScrollGestureAnchor.current = null;
        wheelScrollClampCandidate.current = null;
        if (clampCandidate?.viewKey === committedViewKey) {
          restoreFinishedViewKey.current = null;
          restoreWindow.current = new TranscriptRestoreWindow(performance.now(), document.visibilityState !== "hidden");
          protectedRestorePosition.current = clampCandidate;
          pendingScrollViewKey.current = clampCandidate.viewKey;
          pendingScrollAnchor.current = clampCandidate.anchor;
          latestSemanticPosition.current = clampCandidate;
          stick.current = false;
          setScrollAnchorRestoreGeneration((generation) => generation + 1);
        }
      });
    };
    expire();
  }, [clearWheelScrollGesture, revealSemanticPresentation, semanticViewKey, traceInstance, traceView]);
  const beginUserScrollGesture = useCallback(() => {
    restoreFinishedViewKey.current = pendingScrollViewKey.current;
    protectedRestorePosition.current = null;
    setSemanticRestoreFailureViewKey("");
    clearWheelScrollGesture();
    pendingScrollAnchor.current = null;
    presentationRestoreSnapshot.current = null;
    virtualLayoutReplacementAnchor.current = null;
    detachedVirtualPresentation.current = null;
    if (expertReturnPresentationDeferred.current) {
      latestSemanticPosition.current = null;
      deferredExpertReturnInputOnly.current = true;
      revealSemanticPresentation();
    }
    userScrollGestureActive.current = true;
    userScrollIntent.current = true;
  }, [clearWheelScrollGesture, revealSemanticPresentation]);
  const captureUserPresentationSelection = useCallback((event: ReactMouseEvent<HTMLDivElement>) => {
    const tab = (event.target as Element | null)?.closest(".agent-workspace__tab");
    if (!tab?.closest('[data-testid="persisted-expert-presentation"]')) return;
    if (tab.getAttribute("aria-selected") === "true") {
      userPresentationSelectionIntent.current = false;
      return;
    }
    userPresentationSelectionIntent.current = true;
  }, []);
  const handleExpertMemberFocusChange = useCallback((focused: boolean, returnToLead?: () => void) => {
    userPresentationSelectionIntent.current = false;
    const returning = expertMemberFocused.current && !focused;
    expertMemberFocused.current = focused;
    if (focused) {
      restoreFinishedViewKey.current = pendingScrollViewKey.current;
      pendingScrollAnchor.current = null;
    } else if (returning && latestSemanticPosition.current?.viewKey === pendingScrollViewKey.current) {
      const position = latestSemanticPosition.current;
      pendingScrollAnchor.current = position.anchor;
      protectedRestorePosition.current = position;
      restoreFinishedViewKey.current = null;
      restoreWindow.current = new TranscriptRestoreWindow(performance.now(), document.visibilityState !== "hidden");
      setScrollAnchorRestoreGeneration((generation) => generation + 1);
    }
    onExpertMemberHistoryFocusChange?.(focused, returnToLead);
  }, [onExpertMemberHistoryFocusChange]);
  const stopRecoveryControlPropagation = useCallback((event: { stopPropagation: () => void }) => {
    event.stopPropagation();
  }, []);
  const clearUserScrollIntent = useCallback(() => {
    userScrollGestureActive.current = false;
    userScrollIntent.current = false;
    clearWheelScrollGesture();
  }, [clearWheelScrollGesture]);
  const resetProjectDragScroll = useCallback(() => {
    const drag = projectDragScrollRef.current;
    const element = scrollRef.current;
    projectDragScrollRef.current = null;
    element?.classList.remove("transcript--drag-scrolling");
    if (drag && element && typeof element.hasPointerCapture === "function" && element.hasPointerCapture(drag.pointerId)) {
      element.releasePointerCapture(drag.pointerId);
    }
  }, [scrollRef]);
  const beginProjectDragScroll = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    beginUserScrollGesture();
    if (!dragScroll || event.pointerType !== "mouse" || event.button !== 0) return;
    const target = event.target as Element | null;
    if (target?.closest(PROJECT_DRAG_SCROLL_EXCLUSION_SELECTOR)) return;
    suppressProjectDragClickRef.current = false;
    projectDragScrollRef.current = {
      pointerId: event.pointerId,
      startY: event.clientY,
      startScrollTop: event.currentTarget.scrollTop,
      dragging: false,
    };
    if (typeof event.currentTarget.setPointerCapture === "function") {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
  }, [beginUserScrollGesture, dragScroll]);
  const moveProjectDragScroll = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = projectDragScrollRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    const deltaY = event.clientY - drag.startY;
    if (!drag.dragging && Math.abs(deltaY) < PROJECT_DRAG_SCROLL_THRESHOLD) return;
    if (!drag.dragging) {
      drag.dragging = true;
      suppressProjectDragClickRef.current = true;
      event.currentTarget.classList.add("transcript--drag-scrolling");
      window.getSelection()?.removeAllRanges();
    }
    event.preventDefault();
    event.currentTarget.scrollTop = drag.startScrollTop - deltaY;
  }, []);
  const finishProjectDragScroll = useCallback((event?: ReactPointerEvent<HTMLDivElement>) => {
    const drag = projectDragScrollRef.current;
    if (event && drag && drag.pointerId !== event.pointerId) return;
    resetProjectDragScroll();
    clearUserScrollIntent();
  }, [clearUserScrollIntent, resetProjectDragScroll]);
  const captureProjectDragClick = useCallback((event: ReactMouseEvent<HTMLDivElement>) => {
    if (suppressProjectDragClickRef.current) {
      suppressProjectDragClickRef.current = false;
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    captureUserPresentationSelection(event);
  }, [captureUserPresentationSelection]);
  useEffect(() => {
    window.addEventListener("pointerup", clearUserScrollIntent);
    window.addEventListener("pointercancel", clearUserScrollIntent);
    window.addEventListener("touchend", clearUserScrollIntent);
    window.addEventListener("touchcancel", clearUserScrollIntent);
    window.addEventListener("blur", clearUserScrollIntent);
    return () => {
      window.removeEventListener("pointerup", clearUserScrollIntent);
      window.removeEventListener("pointercancel", clearUserScrollIntent);
      window.removeEventListener("touchend", clearUserScrollIntent);
      window.removeEventListener("touchcancel", clearUserScrollIntent);
      window.removeEventListener("blur", clearUserScrollIntent);
      resetProjectDragScroll();
      clearUserScrollIntent();
    };
  }, [clearUserScrollIntent, resetProjectDragScroll]);
  const markKeyboardScrollIntent = useCallback((event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (["ArrowDown", "ArrowUp", "End", "Home", "PageDown", "PageUp", " "].includes(event.key)) {
      restoreFinishedViewKey.current = pendingScrollViewKey.current;
      protectedRestorePosition.current = null;
      setSemanticRestoreFailureViewKey("");
      clearWheelScrollGesture();
      pendingScrollAnchor.current = null;
      presentationRestoreSnapshot.current = null;
      virtualLayoutReplacementAnchor.current = null;
      detachedVirtualPresentation.current = null;
      if (expertReturnPresentationDeferred.current) {
        latestSemanticPosition.current = null;
        deferredExpertReturnInputOnly.current = true;
        revealSemanticPresentation();
      }
      userScrollIntent.current = true;
    }
  }, [clearWheelScrollGesture, revealSemanticPresentation]);
  // Restore semantic scroll state per session. A cold first view defaults to
  // bottom; cached views restore either the pinned state or a message anchor.
  useLayoutEffect(() => {
    const anchor = initialScrollAnchor;
    const el = scrollRef.current;
    recordTranscriptTrace("view.switch", {
      view: traceViewRef.current,
      itemCount: items.length,
      anchorMode: anchor?.atBottom === false ? "semantic" : "bottom",
      scrollTop: el?.scrollTop ?? -1,
      scrollHeight: el?.scrollHeight ?? -1,
      clientHeight: el?.clientHeight ?? -1,
    }, { persist: true });
    pendingScrollViewKey.current = semanticViewKey;
    hasRevealedPresentation.current = anchor?.atBottom !== false;
    restoreFinishedViewKey.current = null;
    restoreWindow.current = new TranscriptRestoreWindow(performance.now(), document.visibilityState !== "hidden");
    protectedRestorePosition.current = anchor?.atBottom === false ? { viewKey: semanticViewKey, anchor } : null;
    stick.current = anchor?.atBottom !== false;
    pendingRevealBottomScroll.current = anchor?.atBottom !== false;
    pendingScrollAnchor.current = anchor?.atBottom === false ? anchor : null;
    latestSemanticPosition.current = anchor?.atBottom === false
      ? { viewKey: semanticViewKey, anchor }
      : null;
    presentationRestoreSnapshot.current = null;
    virtualLayoutReplacementAnchor.current = null;
    detachedVirtualPresentation.current = null;
    deferredExpertReturnInputOnly.current = false;
    deferredExpertReturnRevealBottom.current = false;
    userPresentationSelectionIntent.current = false;
    expertMemberFocused.current = false;
    setSemanticRestoreFailureViewKey("");
    setRevealedSemanticViewKey(anchor?.atBottom === false ? "" : semanticViewKey);
    clearUserScrollIntent();
    if (el && anchor?.atBottom === false) {
      restoreTranscriptAnchor(el, anchor.anchorMessageId, anchor.offset);
    }
    // initialScrollAnchor is the snapshot captured for this identity; later
    // controller renders must not re-apply a moving scroll position.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clearUserScrollIntent, semanticViewKey]);

  useLayoutEffect(() => {
    if (!pendingRevealBottomScroll.current || items.length === 0) return;
    pendingRevealBottomScroll.current = false;
    // Position the target task before paint, then follow virtual row measures
    // without allowing a previous task's scroll offset to flash onscreen.
    pinToBottomAfterLayout(5);
  }, [items.length, pinToBottomAfterLayout, revealSignal, semanticViewKey]);

  const revealNotice = useMemo(() => forceRevealNotice(items), [items]);

  // Auto-scroll to bottom during streaming, then keep following for a few
  // layout frames so image/avatar loads and markdown reflow cannot push the
  // newest text below the viewport while the user is still pinned.
  const contentVersion = useMemo(() => `${scrollVersion(items)}|task-tree:${taskTrees?.seq ?? 0}`, [items, taskTrees?.seq]);
  const pinnedLayoutVersion = `${contentVersion}|live:${live?.text?.length ?? 0}:${live?.reasoning?.length ?? 0}`;
  useEffect(() => {
    if (items.length === 0) return;
    if (!stick.current) return;
    pinToBottomAfterLayout(3);
  }, [pinnedLayoutVersion, pinToBottomAfterLayout]);

  // ResizeObserver for container height changes.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const attachedView = traceViewRef.current;
    lastClientHeight.current = el.clientHeight;
    recordTranscriptTrace("resize.attach", {
      view: attachedView,
      itemCount: items.length,
      clientHeight: el.clientHeight,
    }, { persist: true });
    const observer = new ResizeObserver(() => {
      const previous = lastClientHeight.current ?? el.clientHeight;
      lastClientHeight.current = el.clientHeight;
      recordTranscriptTrace("resize.fire", {
        view: traceViewRef.current,
        itemCount: items.length,
        clientHeight: el.clientHeight,
        delta: el.clientHeight - previous,
        stick: stick.current,
      });
      if (items.length === 0) return;
      repinIfWasPinned(el.clientHeight - previous);
    });
    observer.observe(el);
    return () => {
      observer.disconnect();
      recordTranscriptTrace("resize.detach", { view: attachedView, itemCount: items.length }, { persist: true });
      if (resizeFrame.current !== null) {
        cancelAnimationFrame(resizeFrame.current);
        resizeFrame.current = null;
      }
    };
  }, [items.length]);

  // Footer height changes → smooth scroll repin with GSAP.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const previous = lastFooterHeight.current ?? footerHeight;
    lastFooterHeight.current = footerHeight;
    if (items.length === 0) return;
    repinIfWasPinned(previous - footerHeight);
  }, [footerHeight, items.length]);

  // After a non-fork rewind, scroll to the last user message (the
  // rewound-to point) so the user knows where they are.
  useEffect(() => {
    if (rewindSignal <= 0 || questions.length === 0) return;
    const lastQ = questions[questions.length - 1];
    const container = scrollRef.current;
    if (!container) return;
    const el = findTranscriptAnchorNode(container, lastQ.id);
    if (!el) return;
    stick.current = false;
    container.scrollTop = scrollTopForTranscriptNode(container, el, 12);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rewindSignal]);

  // Sub-agent calls carry a parentId; collect them under their parent `task`
  // call so the parent card can render them nested, and skip them at top level.
  const subcallsByParent = useMemo(() => {
    const m = new Map<string, ToolItem[]>();
    for (const [parentId, branches] of Object.entries(parallelItemsByCall ?? {})) {
      m.set(parentId, branches);
    }
    for (const it of items) {
      if (it.kind === "tool" && it.parentId) {
        const arr = m.get(it.parentId) ?? [];
        arr.push(it);
        m.set(it.parentId, arr);
      }
    }
    return m;
  }, [items, parallelItemsByCall]);

  const sessionItemsByParent = useMemo(() => {
    const m = new Map<string, Item[]>();
    for (const it of items) {
      const parentId = (it as { parentId?: string }).parentId;
      if (!parentId) continue;
      const arr = m.get(parentId) ?? [];
      arr.push(it);
      m.set(parentId, arr);
    }
    return m;
  }, [items]);

  const [restoredExpertToolData, setRestoredExpertToolData] = useState<ReadonlyMap<string, RestoredExpertToolData>>(new Map());
  const restoredExpertToolRequestsRef = useRef(new Set<string>());
  useEffect(() => {
    restoredExpertToolRequestsRef.current.clear();
    setRestoredExpertToolData(new Map());
  }, [semanticViewKey]);
  const expertTeamRefreshSignal = useMemo(() => {
    const childRefs = Object.keys(childItemsByRef).sort().join(",");
    const statusTransitions = Object.entries(subagentStatusByRef)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([ref, status]) => `${ref}:${status}`)
      .join(",");
    const taskStates = items
      .filter((item): item is ToolItem => item.kind === "tool" && item.name === "task" && !item.parentId)
      .map((item) => `${item.id}:${item.status}`)
      .join(",");
    return `${activeExpert?.mode ?? ""}:${activeExpert?.id ?? ""}|${childRefs}|${statusTransitions}|${taskStates}|${historyRevalidationSignal}|${rewindSignal}`;
  }, [activeExpert?.id, activeExpert?.mode, childItemsByRef, historyRevalidationSignal, items, rewindSignal, subagentStatusByRef]);
  const expertTeamScopeKey = `${semanticViewKey}:expert:${expertSelectionRevision}`;
  const expertTeamCacheRef = useRef(new Map<string, ExpertTeamSession | null>());
  const [loadedExpertTeam, setLoadedExpertTeam] = useState<{
    scopeKey: string;
    team: ExpertTeamSession | null;
  } | null>(null);
  const [expertTeamRetrySignal, setExpertTeamRetrySignal] = useState(0);
  const expertTeamLoadAttemptsRef = useRef(new Map<string, number>());
  const candidateSessionExpertTeam = loadedExpertTeam?.scopeKey === expertTeamScopeKey
    ? loadedExpertTeam.team
    : expertTeamCacheRef.current.get(expertTeamScopeKey) ?? null;
  const sessionExpertTeam = candidateSessionExpertTeam;
  const expertTeamResolutionSettled = Boolean(
    loadedExpertTeam?.scopeKey === expertTeamScopeKey
    || expertTeamCacheRef.current.has(expertTeamScopeKey)
  );
  const expertSemanticRestoreHidden = Boolean(sessionExpertTeam)
    && initialScrollAnchor?.atBottom === false
    && revealedSemanticViewKey !== semanticViewKey;
  // A selected team may already have a sidecar before the user sends the
  // first turn. Its workspace is intentionally not part of an empty session's
  // presentation, so a missing or temporarily unreadable sidecar must not
  // replace the welcome surface with a blocking error.
  // Persisted expert presentation is a tab-sidecar concern. A transient team
  // selection without a tab must keep rendering the ordinary/legacy transcript
  // instead of waiting forever for a sidecar that cannot be queried.
  const persistedExpertPresentationRequired = items.length > 0
    && activeExpert?.mode === "team"
    && (Boolean(tabId) || initialScrollAnchor?.atBottom === false);
  const expertTeamResolutionPending = Boolean(persistedExpertPresentationRequired && (
    !tabId
    || !expertTeamResolutionSettled
  ));
  const deferExpertReturnPresentation = expertTeamResolutionPending
    && initialScrollAnchor?.atBottom === false
    && revealedSemanticViewKey !== semanticViewKey;
  useEffect(() => {
    recordTranscriptTrace("expert-cache.lookup", {
      view: traceView,
      instance: traceInstance,
      hit: expertTeamCacheRef.current.has(expertTeamScopeKey),
      size: expertTeamCacheRef.current.size,
      loadedScopeMatch: loadedExpertTeam?.scopeKey === expertTeamScopeKey,
      hasTeam: Boolean(sessionExpertTeam),
    }, { persist: true });
  }, [expertTeamScopeKey, loadedExpertTeam?.scopeKey, sessionExpertTeam, traceInstance, traceView]);
  useEffect(() => {
    if (!tabId) {
      return;
    }
    const requestKey = expertTeamScopeKey;
    const loadExpertTeam = app.ExpertTeamForTab;
    if (typeof loadExpertTeam !== "function") {
      expertTeamCacheRef.current.set(expertTeamScopeKey, null);
      setLoadedExpertTeam({ scopeKey: expertTeamScopeKey, team: null });
      return;
    }
    let cancelled = false;
    let retryTimer: number | null = null;
    void Promise.resolve(loadExpertTeam(tabId))
      .then((team) => {
        if (cancelled) return;
        expertTeamLoadAttemptsRef.current.delete(requestKey);
        const stagedSnapshot = deferredExpertReturnInputOnly.current
          ? null
          : stagePresentationRestoreSnapshot(
            committedPresentation.current,
            semanticViewKey,
            latestSemanticPosition.current,
            Boolean(team),
            stick.current,
          );
        deferredExpertReturnInputOnly.current = false;
        if (stagedSnapshot) presentationRestoreSnapshot.current = stagedSnapshot;
        expertTeamCacheRef.current.set(expertTeamScopeKey, team);
        recordTranscriptTrace("expert-cache.write", {
          view: traceView,
          instance: traceInstance,
          size: expertTeamCacheRef.current.size,
          hasTeam: Boolean(team),
        }, { persist: true });
        setLoadedExpertTeam({ scopeKey: expertTeamScopeKey, team });
      })
      .catch(() => {
        if (cancelled) return;
        const attempts = (expertTeamLoadAttemptsRef.current.get(requestKey) ?? 0) + 1;
        expertTeamLoadAttemptsRef.current.set(requestKey, attempts);
        if (attempts < EXPERT_TEAM_LOAD_MAX_ATTEMPTS) {
          retryTimer = window.setTimeout(() => {
            if (!cancelled) setExpertTeamRetrySignal((signal) => signal + 1);
          }, EXPERT_TEAM_LOAD_RETRY_DELAY_MS);
          return;
        }
        expertTeamLoadAttemptsRef.current.delete(requestKey);
        deferredExpertReturnInputOnly.current = false;
        expertTeamCacheRef.current.set(expertTeamScopeKey, null);
        recordTranscriptTrace("expert-cache.write", {
          view: traceView,
          instance: traceInstance,
          size: expertTeamCacheRef.current.size,
          hasTeam: false,
        }, { persist: true });
        setLoadedExpertTeam({ scopeKey: expertTeamScopeKey, team: null });
      });
    return () => {
      cancelled = true;
      if (retryTimer !== null) window.clearTimeout(retryTimer);
    };
  }, [activeExpert?.id, expertTeamRefreshSignal, expertTeamRetrySignal, expertTeamScopeKey, semanticViewKey, stick, tabId, traceInstance, traceView]);

  useEffect(() => {
    if (!sessionExpertTeam || !tabId) return;
    let cancelled = false;
    const retryTimers = new Set<number>();
    const candidates = items.filter((item): item is ToolItem => (
      item.kind === "tool"
      && isRestorableExpertTool(item)
      && item.name !== "parallel_tasks"
      && !restoredExpertToolData.has(item.id)
      && !restoredExpertToolRequestsRef.current.has(`${semanticViewKey}\u0000${item.id}`)
    ));
    if (candidates.length === 0) return;
    const load = (item: ToolItem, attempt: number) => {
      void app.ToolResultForTab(tabId, item.id).catch(() => null).then((data) => {
        if (cancelled) return;
        if (data) {
          setRestoredExpertToolData((current) => {
            const next = new Map(current);
            next.set(item.id, data);
            return next;
          });
          return;
        }
        if (attempt >= 4) {
          restoredExpertToolRequestsRef.current.delete(`${semanticViewKey}\u0000${item.id}`);
          return;
        }
        const timer = window.setTimeout(() => {
          retryTimers.delete(timer);
          if (!cancelled) load(item, attempt + 1);
        }, 250);
        retryTimers.add(timer);
      });
    };
    for (const item of candidates) {
      const requestKey = `${semanticViewKey}\u0000${item.id}`;
      restoredExpertToolRequestsRef.current.add(requestKey);
      load(item, 0);
    }
    return () => {
      cancelled = true;
      for (const timer of retryTimers) window.clearTimeout(timer);
      for (const item of candidates) restoredExpertToolRequestsRef.current.delete(`${semanticViewKey}\u0000${item.id}`);
    };
  }, [items, semanticViewKey, sessionExpertTeam, tabId]);

  const expertTeamFlow = useMemo(
    () => taskTrees ? buildExpertTeamFlowModel({ taskTrees, items, activeExpert }) : null,
    [activeExpert, items, taskTrees],
  );

  const expertWorkspaceItems = useMemo(() => {
    if (activeExpert?.mode !== "team") return [];
    const out: ToolItem[] = [];
    const seen = new Set<string>();
    for (const item of items) {
      if (item.kind !== "tool") continue;
      if (item.name === "parallel_tasks") continue;
      if (!isExpertDelegationTool(item.name)) continue;
      if (item.parentId) continue;
      if (seen.has(item.id)) continue;
      seen.add(item.id);
      out.push(restoreArchivedExpertTool(item, restoredExpertToolData.get(item.id)));
    }
    return out;
  }, [activeExpert?.mode, items, restoredExpertToolData]);
  const displayedExpertTeamFlow = useMemo(() => {
    if (!expertTeamFlow) return null;
    const onlyParallelDelegation = expertWorkspaceItems.length === 0 && items.some((item) => item.kind === "tool" && item.name === "parallel_tasks" && !item.parentId);
    return onlyParallelDelegation ? null : expertTeamFlow;
  }, [expertTeamFlow, expertWorkspaceItems.length, items]);

  const expertWorkspaceTeam = useMemo<AgentWorkspaceExpertTeam | null>(() => {
    if (activeExpert?.mode !== "team") return null;
    return {
      name: activeExpert.name,
      avatarSrc: activeExpert.avatarSrc,
      members: activeExpert.members,
    };
  }, [activeExpert]);

  const expertWorkspaceMemberByItemId = useMemo<AgentWorkspaceMemberMap | undefined>(() => {
    if (activeExpert?.mode !== "team" || !activeExpert.members?.length || !expertTeamFlow) return undefined;
    const membersByName = new Map(
      activeExpert.members
        .map((member) => [normalizedExpertMemberName(member.name), member] as const)
        .filter(([name]) => Boolean(name)),
    );
    const byItemId = new Map<string, AgentWorkspaceExpertMember>();
    for (const node of expertTeamFlow.nodes) {
      const member = membersByName.get(normalizedExpertMemberName(node.expertName));
      if (!member) continue;
      for (const tool of node.tools) {
        if (isExpertDelegationTool(tool.name)) byItemId.set(tool.id, member);
      }
    }
    return byItemId.size > 0 ? byItemId : undefined;
  }, [activeExpert?.members, activeExpert?.mode, expertTeamFlow]);

  // Selecting a team in the composer can persist a sidecar before the first
  // user turn is sent. Keep that selection in the composer, but do not expose
  // the conversation workspace/member avatar until the session has content.
  const persistedExpertWorkspaceActive = items.length > 0 && Boolean(sessionExpertTeam && tabId);
  const persistedExpertWorkspacePending = Boolean(
    (sessionExpertTeam && !tabId)
    || (persistedExpertPresentationRequired && expertTeamResolutionPending)
  );
  // Sidecar loading must not cover readable history. Only a saved non-bottom
  // anchor needs protection until the matching expert presentation is ready.
  const persistedExpertWorkspaceBlocking = persistedExpertWorkspacePending
    && initialScrollAnchor?.atBottom === false
    && revealedSemanticViewKey !== semanticViewKey;
  const persistedExpertWorkspaceFallback = Boolean(
    persistedExpertPresentationRequired
    && expertTeamResolutionSettled
    && !sessionExpertTeam
  );
  // The sidecar only enriches the team presentation. If it remains unavailable,
  // keep the regular parent transcript usable instead of replacing an active
  // turn with an error panel. A later status/history update still re-reads it.
  const legacyExpertWorkspaceActive = !persistedExpertWorkspaceActive
    && !persistedExpertWorkspacePending
    && !persistedExpertPresentationRequired
    && activeExpert?.mode === "team"
    && expertWorkspaceItems.length > 0;
  const expertWorkspaceActive = persistedExpertWorkspaceActive || legacyExpertWorkspaceActive;
  const expertWorkspaceHiddenToolIds = useMemo(() => {
    if (!legacyExpertWorkspaceActive) return new Set<string>();
    const hidden = new Set(expertWorkspaceItems.map((item) => item.id));
    const expertParallelTaskIds = new Set(
      expertWorkspaceItems
        .map((item) => item.parentId)
        .filter((parentId): parentId is string => Boolean(parentId)),
    );
    for (const item of items) {
      if (item.kind === "tool" && item.name === "parallel_tasks" && !item.parentId && expertParallelTaskIds.has(item.id)) {
        hidden.add(item.id);
      }
    }
    return hidden;
  }, [expertWorkspaceItems, items, legacyExpertWorkspaceActive]);

  // ── Layer state ────────────────────────────────────────────────────────────
  const [expandedWarmTurns, setExpandedWarmTurns] = useState<Set<number>>(new Set());
  const [coldPage, setColdPage] = useState(0);

  // Compute turn groups from the structural item list. Streaming text updates
  // keep the same items[] reference, so this stays out of the token hot path.
  const turnGroups = useMemo(() => buildTurnGroups(items), [items]);

  // hotStartIdx: first index of the hot zone in items[].
  const hotStartIdx = useMemo(() => {
    let needed = HOT_TURNS;
    for (let i = items.length - 1; i >= 0; i--) {
      if (items[i].kind === "user") {
        needed--;
        if (needed <= 0) return i;
      }
    }
    return 0;
  }, [items]);

  // How many turns are in the cold zone (not yet shown).
  const warmTurnCount = turnGroups.length - Math.min(turnGroups.length, HOT_TURNS);
  const shownWarmStart = Math.max(0, warmTurnCount - coldPage * WARM_PAGE_SIZE);
  const coldTurnCount = shownWarmStart;

  // ── The turn action menu ──────────────────────────────────────────────────
  const [openAction, setOpenAction] = useState<OpenTurnAction | null>(null);
  useEffect(() => {
    if (openAction === null) return;
    const onDown = (e: MouseEvent) => {
      const el = e.target as Element | null;
      if (!el || !el.closest(".turn-actions")) setOpenAction(null);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [openAction]);

  const userTurn = useMemo(() => new Map(questions.map((question) => [question.id, question.turn])), [questions]);
  const checkpointsByTurn = useMemo(() => new Map(checkpoints.map((checkpoint) => [checkpoint.turn, checkpoint])), [checkpoints]);

  // ── JumpBar integration ───────────────────────────────────────────────────
  const jumpToQuestion = useCallback((question: QuestionAnchor) => {
    const container = scrollRef.current;
    if (!container) return;
    const node = findTranscriptAnchorNode(container, question.id);
    if (node) {
      stick.current = false;
      smoothScrollTo(node, 12);
      return;
    }
    const virtualRow = virtualQuestionRowByAnchorIDRef.current.get(questionAnchorId(question.id));
    if (virtualRow === undefined || !virtualRowsRef.current) return;
    stick.current = false;
    virtualRowsRef.current.scrollToIndex(virtualHotRowOffsetRef.current + virtualRow, "start");
    let framesLeft = SCROLL_ANCHOR_RESTORE_FRAMES;
    const alignMountedQuestion = () => {
      requestAnimationFrame(() => {
        if (!transcriptMounted.current || pendingScrollViewKey.current !== semanticViewKey) return;
        const mounted = scrollRef.current
          ? findTranscriptAnchorNode(scrollRef.current, question.id)
          : null;
        if (mounted) {
          smoothScrollTo(mounted, 12);
          return;
        }
        framesLeft -= 1;
        if (framesLeft > 0) alignMountedQuestion();
      });
    };
    alignMountedQuestion();
  }, [scrollRef, semanticViewKey, smoothScrollTo, stick]);

  const handleJumpToQuestion = useCallback((question: QuestionAnchor) => {
    restoreFinishedViewKey.current = semanticViewKey;
    protectedRestorePosition.current = null;
    pendingScrollAnchor.current = null;
    presentationRestoreSnapshot.current = null;
    virtualLayoutReplacementAnchor.current = null;
    detachedVirtualPresentation.current = null;
    setSemanticRestoreFailureViewKey("");
    revealSemanticPresentation();
    // Auto-expand the warm turn when jumping to an old question.
    const warmTurnStart = turnGroups.length - HOT_TURNS;
    if (question.turn < warmTurnStart) {
      setExpandedWarmTurns((prev) => {
        if (prev.has(question.turn)) return prev;
        return new Set([...prev, question.turn]);
      });
    }
    jumpToQuestion(question);
  }, [jumpToQuestion, revealSemanticPresentation, semanticViewKey, turnGroups.length]);

  // ── Hot zone: fully rendered from hotStartIdx to end ─────────────────────
  // Memoized separately from the assembly so streaming tokens don't rebuild
  // the warm/cold zone JSX trees. Uses LiveStreamContext for streaming data
  // (added by upstream PR #3423) instead of per-call renderSegments.
  const empty = items.length === 0;

  useLayoutEffect(() => {
    if (!empty) return;
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = 0;
    stick.current = false;
    const frame = requestAnimationFrame(() => {
      el.scrollTop = 0;
    });
    return () => cancelAnimationFrame(frame);
  }, [empty, scrollRef, stick, tabId]);

  // Compact mode presents one process fold per user turn. Intermediate prose,
  // reasoning, tools, phases and informational runtime events stay inspectable
  // inside that fold; only the accepted final answer remains in the main flow.
  const compactTurns = useMemo<CompactTurn[] | null>(() => {
    if (displayMode === "standard" && !creationMode) return null;
    const turns: Array<{ user?: UserItem; items: Item[] }> = [];
    let current: { user?: UserItem; items: Item[] } | null = null;

    for (let i = hotStartIdx; i < items.length; i++) {
      const it = items[i];
      if ((it as { parentId?: string }).parentId) continue;
      if (it.kind === "tool" && expertWorkspaceHiddenToolIds.has(it.id)) continue;
      if (it.kind === "user") {
        if (current && (current.user || current.items.length > 0)) turns.push(current);
        current = { user: it, items: [] };
        continue;
      }
      if (!current) current = { items: [] };
      current.items.push(it);
    }
    if (current && (current.user || current.items.length > 0)) turns.push(current);

    return turns.map((turn, turnIndex) => {
      const settled = turnIndex < turns.length - 1 || !running;
      let finalIndex = -1;
      for (let index = turn.items.length - 1; index >= 0; index--) {
        const item = turn.items[index];
        if (item.kind === "assistant" && item.finalAnswer === true && item.text.trim() !== "") {
          finalIndex = index;
          break;
        }
      }
      let mostSubstantiveIndex = -1;
      let mostSubstantiveLength = 0;
      for (let index = 0; index < turn.items.length; index++) {
        const item = turn.items[index];
        if (item.kind !== "assistant" || item.streaming) continue;
        const length = item.text.trim().length;
        if (length > mostSubstantiveLength) {
          mostSubstantiveIndex = index;
          mostSubstantiveLength = length;
        }
      }
      // Some providers accept a short post-Todo acknowledgement as the final
      // candidate after already emitting the real report. Keep that report in
      // the answer stream when it is clearly more substantial than the receipt.
      if (finalIndex >= 0 && mostSubstantiveIndex >= 0) {
        const acceptedLength = (turn.items[finalIndex] as AssistantItem).text.trim().length;
        if (mostSubstantiveLength >= 200 && mostSubstantiveLength > acceptedLength * 3) {
          finalIndex = mostSubstantiveIndex;
        }
      }
      // Older persisted sessions may not carry finalAnswer. A settled turn's
      // most substantial completed body is the safest visible-answer fallback.
      if (finalIndex < 0 && settled) finalIndex = mostSubstantiveIndex;

      const finalAssistant = finalIndex >= 0 ? turn.items[finalIndex] as AssistantItem : undefined;
      const processItems = finalIndex >= 0
        ? turn.items.filter((_, index) => index !== finalIndex)
        : [...turn.items];
      if (finalAssistant?.reasoning.trim()) {
        processItems.push({ ...finalAssistant, text: "", finalAnswer: undefined });
      }
      return { user: turn.user, processItems, finalAssistant };
    });
  }, [creationMode, displayMode, expertWorkspaceHiddenToolIds, hotStartIdx, items, running]);
  const restoredGeneratedMediaOutputs = useArchivedGeneratedMediaOutputs(items, tabId, semanticViewKey);
  const assistantGeneratedMedia = useMemo(
    () => generatedMediaByAssistant(items, restoredGeneratedMediaOutputs),
    [items, restoredGeneratedMediaOutputs],
  );

  const hotZoneNodes = useMemo<ReactNode[]>(() => {
    const out: ReactNode[] = [];
    let actionText = "";
    let actionReady = false;
    let actionTurnMeta: TurnMetadata | undefined;
    let activeTurn: number | undefined;
    const pushTurnActions = () => {
      if (activeTurn == null || !actionReady || actionText.trim() === "") return;
      const turn = activeTurn;
      const openMenu = openAction && openAction.turn === turn ? openAction.menu : null;
      out.push(
        <TurnActions
          key={`ta-${turn}`}
          text={actionText}
          turn={turn}
          openMenu={openMenu}
          onOpenMenu={(menu) => setOpenAction(menu ? { turn, menu } : null)}
          checkpoint={checkpointsByTurn.get(turn)}
          actionPending={actionPending}
          rewindDisabled={rewindDisabled}
          hoverMenus={actionHoverMenus}
          turnMeta={actionTurnMeta}
          onRewind={(targetTurn, scope) => {
            onRewind?.(targetTurn, scope);
            setOpenAction(null);
          }}
        />,
      );
      actionText = "";
      actionReady = false;
      actionTurnMeta = undefined;
    };

    // Compact mode: one process fold and one visible final answer per turn.
    // Standard mode: flat rendering (no step groups)
    if (compactTurns) {
      let expertFlowTurnIndex = -1;
      if (displayedExpertTeamFlow) {
        for (let index = compactTurns.length - 1; index >= 0; index--) {
          const ownsTree = compactTurns[index].processItems.some((item) => item.kind === "tool" && item.taskTreeId === displayedExpertTeamFlow.treeId);
          if (ownsTree) {
            expertFlowTurnIndex = index;
            break;
          }
        }
        if (expertFlowTurnIndex < 0) expertFlowTurnIndex = compactTurns.length - 1;
      }
      for (let turnIndex = 0; turnIndex < compactTurns.length; turnIndex++) {
        const turn = compactTurns[turnIndex];
        const isLastTurn = turnIndex === compactTurns.length - 1;
        if (turn.user) {
          pushTurnActions();
          const tn = userTurn.get(turn.user.id);
          const checkpoint = tn == null ? undefined : checkpointsByTurn.get(tn);
          activeTurn = tn;
          out.push(
            <UserMessage
              key={turn.user.id}
              id={turn.user.id}
              text={turn.user.text}
              submitText={turn.user.submitText}
              failed={turn.user.failed}
              createdAt={turn.user.createdAt}
              turn={tn}
              anchorId={questionAnchorId(turn.user.id)}
              onEdit={onEditPrompt}
              editDisabled={rewindDisabled || !checkpoint?.canConversation}
            />,
          );
        }

        const attentionItems = turn.processItems.filter((item) => item.kind === "notice" && item.level === "warn");
        const foldedItems = turn.processItems.filter((item) => item.kind !== "notice" || item.level !== "warn");
        const durationMs = foldedItems.reduce((ms, item) => ms + (item.kind === "tool" ? item.durationMs ?? 0 : 0), 0);
        const expertFlow = turnIndex === expertFlowTurnIndex ? displayedExpertTeamFlow : null;
        const active = isLastTurn && running;
        if (foldedItems.length > 0 || expertFlow || active) {
          out.push(
            <TurnCollapse
              key={`turn-process-${turn.user?.id ?? foldedItems[0]?.id ?? "orphan"}`}
              items={foldedItems}
              durationMs={durationMs}
              mode={displayMode}
              subcalls={subcallsByParent}
              tabId={tabId}
              creationMode={creationMode}
              expertTeam={expertWorkspaceTeam}
              memberByItemId={expertWorkspaceMemberByItemId}
              expertTeamFlow={expertFlow}
              active={active}
            />,
          );
        }
        for (const item of attentionItems) {
          if (item.kind === "notice") out.push(<NoticeCard key={item.id} level={item.level} text={item.text} />);
        }

        if (turn.finalAssistant) {
          const finalItem = turn.finalAssistant.reasoning
            ? { ...turn.finalAssistant, reasoning: "", reasoningComplete: true }
            : turn.finalAssistant;
          out.push(
            <LiveAssistantMessage
              key={finalItem.id}
              item={finalItem}
              defaultExpanded={false}
              expandWhileStreaming={false}
              truncateStreamingReasoning={true}
              creationMode={creationMode}
              tabId={tabId}
              generatedMedia={assistantGeneratedMedia.get(finalItem.id)}
            />,
          );
          if (!finalItem.streaming && finalItem.text.trim() !== "") {
            actionText = finalItem.text;
            actionReady = true;
            actionTurnMeta = finalItem.turn;
          }
        }
      }
      if (!running) pushTurnActions();
    } else {
      // Standard mode: flat rendering
      const roBatch: ToolItem[] = [];
      const toolBatch: ToolItem[] = [];
      let toolBatchKind: ToolGroupKind | null = null;
      const flushRO = () => {
        if (roBatch.length === 0) return;
        out.push(<ReadOnlyBatch key={`rob-${roBatch[0].id}`} items={[...roBatch]} subcalls={subcallsByParent} tabId={tabId} />);
        roBatch.length = 0;
      };
      const flushToolBatch = () => {
        if (!toolBatchKind || toolBatch.length === 0) return;
        out.push(<ToolGroup key={`tg-${toolBatch[0].id}`} kind={toolBatchKind} items={[...toolBatch]} subcalls={subcallsByParent} tabId={tabId} />);
        toolBatch.length = 0;
        toolBatchKind = null;
      };
      for (let i = hotStartIdx; i < items.length; i++) {
        const it = items[i];
        if ((it as { parentId?: string }).parentId) continue;
        if (it.kind === "tool" && expertWorkspaceHiddenToolIds.has(it.id)) continue;
        if (creationMode && it.kind === "tool" && isCreationGroupableTool(it as ToolItem)) {
          const kind = toolGroupKind(it as ToolItem);
          if (kind) {
            if (toolBatchKind && toolBatchKind !== kind) flushToolBatch();
            toolBatchKind = kind;
            toolBatch.push(it as ToolItem);
            continue;
          }
        }
        if (
          !creationMode &&
          it.kind === "tool" &&
          !it.parentId &&
          it.status !== "running" &&
          it.name !== "todo_write" &&
          it.name !== "exit_plan_mode" &&
          isReadOnlyTool(it.name)
        ) {
          roBatch.push(it as ToolItem);
          continue;
        }
        flushToolBatch();
        flushRO();
        switch (it.kind) {
          case "user": {
            pushTurnActions();
            const tn = userTurn.get(it.id);
            const checkpoint = tn == null ? undefined : checkpointsByTurn.get(tn);
            activeTurn = tn;
            out.push(
              <UserMessage
                key={it.id}
                id={it.id}
                text={it.text}
                submitText={it.submitText}
                failed={it.failed}
                createdAt={it.createdAt}
                turn={tn}
                anchorId={questionAnchorId(it.id)}
                onEdit={onEditPrompt}
                editDisabled={rewindDisabled || !checkpoint?.canConversation}
              />,
            );
            break;
          }
          case "assistant":
            out.push(<LiveAssistantMessage key={it.id} item={it as AssistantItem} defaultExpanded={false} creationMode={creationMode} tabId={tabId} generatedMedia={assistantGeneratedMedia.get(it.id)} />);
            if (!it.streaming && it.text.trim() !== "") {
              actionText = it.text;
              actionReady = true;
              actionTurnMeta = it.turn;
            }
            break;
          case "tool":
            if (it.parentId) break;
            if (it.name === "todo_write") break;
            if (it.name === "exit_plan_mode") break;
            out.push(<ToolCard key={it.id} item={it} subcalls={subcallsByParent.get(it.id)} tabId={tabId} expertTeam={expertWorkspaceTeam} memberByItemId={expertWorkspaceMemberByItemId} />);
            break;
          case "phase": out.push(<PhaseCard key={it.id} text={it.text} />); break;
          case "notice": out.push(<NoticeCard key={it.id} level={it.level} text={it.text} />); break;
          case "compaction": out.push(<CompactionCard key={it.id} item={it} />); break;
        }
      }
      flushToolBatch();
      flushRO();
      if (!running) pushTurnActions();
    }
    return out;
  }, [hotStartIdx, items, openAction, actionPending, rewindDisabled, running, onEditPrompt, onRewind, subcallsByParent, userTurn, checkpointsByTurn, displayMode, compactTurns, tabId, actionHoverMenus, creationMode, expertWorkspaceTeam, expertWorkspaceMemberByItemId, expertWorkspaceActive, expertWorkspaceHiddenToolIds, assistantGeneratedMedia, displayedExpertTeamFlow]);

  const hotZoneRows = useMemo(() => transcriptVirtualRows(hotZoneNodes), [hotZoneNodes]);
  const regularRows: TranscriptVirtualRow[] = [];
  if (hasOlderHistory) {
    regularRows.push({
      key: "load-older-history",
      node: (
        <button
          type="button"
          className="warm-collapse"
          disabled={loadingOlderHistory}
          onClick={() => void onLoadOlderHistory?.()}
        >
          {loadingOlderHistory ? t("transcript.loadingEarlierHistory") : t("transcript.loadEarlierHistory")}
        </button>
      ),
    });
  }
  if (turnGroups.length > HOT_TURNS) {
    regularRows.push({
      key: "warm-zone",
      node: (
        <WarmZone
          turnGroups={turnGroups}
          expandedWarmTurns={expandedWarmTurns}
          shownWarmStart={shownWarmStart}
          coldTurnCount={coldTurnCount}
          scrollRef={scrollRef}
          warmItems={items}
          warmSubcalls={subcallsByParent}
          warmUserTurn={userTurn}
          warmCheckpoints={checkpointsByTurn}
          warmOpenAction={openAction}
          warmActionPending={actionPending}
          warmRewindDisabled={rewindDisabled}
          warmActionHoverMenus={actionHoverMenus}
          warmOnRewind={onRewind}
          warmSetOpenAction={setOpenAction}
          warmOnEdit={onEditPrompt}
          tabId={tabId}
          creationMode={creationMode}
          assistantGeneratedMedia={assistantGeneratedMedia}
          onToggleColdPage={() => setColdPage((page) => page + 1)}
          onToggleWarmTurn={(group, expand) => {
            setExpandedWarmTurns((previous) => {
              const next = new Set(previous);
              if (expand) next.add(group); else next.delete(group);
              return next;
            });
          }}
        />
      ),
    });
  }
  regularRows.push(...hotZoneRows);
  if (displayedExpertTeamFlow && displayMode === "standard" && !creationMode) {
    regularRows.push({
      key: "expert-team-flow",
      node: <ExpertTeamFlowCard model={displayedExpertTeamFlow} />,
    });
  }
  const renderedParentRows = expertWorkspaceActive ? hotZoneRows : regularRows;
  useLayoutEffect(() => {
    const byAnchorID = new Map<string, number>();
    renderedParentRows.forEach((row, index) => {
      if (row.anchorId) byAnchorID.set(row.anchorId, index);
    });
    virtualQuestionRowByAnchorIDRef.current = byAnchorID;
    virtualHotRowOffsetRef.current = 0;
  }, [renderedParentRows]);
  const parentRowsVirtualized = shouldVirtualizeTranscriptRows(renderedParentRows.length);
  const virtualizeRegularRows = !expertWorkspaceActive
    && !empty
    && parentRowsVirtualized;
  const tracePath = persistedExpertWorkspaceActive
    ? "team-persisted"
    : persistedExpertWorkspaceBlocking
      ? "team-pending"
    : persistedExpertWorkspaceFallback && initialScrollAnchor?.atBottom === false && revealedSemanticViewKey !== semanticViewKey
      // Keep the pending presentation identity while the DOM swaps to the
      // regular transcript, so a settled fallback cannot replay an old anchor.
      ? "team-pending"
    : legacyExpertWorkspaceActive
      ? "team-legacy"
      : "normal";
  const persistedExpertWorkspaceHidden = persistedExpertWorkspaceActive
    && expertSemanticRestoreHidden;
  const ordinarySemanticRestoreHidden = !persistedExpertWorkspaceActive && !legacyExpertWorkspaceActive
    && initialScrollAnchor?.atBottom === false
    && revealedSemanticViewKey !== semanticViewKey;
  useLayoutEffect(() => {
    suppressReplacementCaptureDuringReveal.current = false;
  }, [revealedSemanticViewKey]);
  const semanticRestoreFailed = semanticRestoreFailureViewKey === semanticViewKey;
  const sessionLoadingActivity = existingSessionLoading
    || persistedExpertWorkspaceBlocking
    || (persistedExpertWorkspaceHidden && !semanticRestoreFailed)
    || (legacyExpertWorkspaceActive && deferExpertReturnPresentation)
    || (ordinarySemanticRestoreHidden && !semanticRestoreFailed);
  const sessionLoadingFeedbackVisible = useSessionLoadingFeedback(sessionLoadingActivity, semanticViewKey);
  const retrySemanticRestore = useCallback(() => {
    const position = protectedRestorePosition.current;
    if (!position || position.viewKey !== pendingScrollViewKey.current) return;
    pendingScrollAnchor.current = position.anchor;
    restoreFinishedViewKey.current = null;
    restoreWindow.current = new TranscriptRestoreWindow(performance.now(), document.visibilityState !== "hidden");
    setSemanticRestoreFailureViewKey("");
    setScrollAnchorRestoreGeneration((generation) => generation + 1);
  }, []);
  const showCurrentSemanticPresentation = useCallback(() => {
    restoreFinishedViewKey.current = pendingScrollViewKey.current;
    pendingScrollAnchor.current = null;
    presentationRestoreSnapshot.current = null;
    virtualLayoutReplacementAnchor.current = null;
    detachedVirtualPresentation.current = null;
    deferredExpertReturnRevealBottom.current = false;
    replacementPresentationHidden.current = false;
    setSemanticRestoreFailureViewKey("");
    revealSemanticPresentation();
  }, [revealSemanticPresentation]);
  useLayoutEffect(() => {
    expertReturnPresentationDeferred.current = deferExpertReturnPresentation
      || persistedExpertWorkspaceHidden
      || ordinarySemanticRestoreHidden;
  }, [deferExpertReturnPresentation, ordinarySemanticRestoreHidden, persistedExpertWorkspaceHidden]);
  useEffect(() => {
    if (
      (!persistedExpertWorkspaceActive && !persistedExpertWorkspaceFallback)
      || !deferredExpertReturnRevealBottom.current
    ) return;
    deferredExpertReturnRevealBottom.current = false;
    pendingScrollAnchor.current = null;
    latestSemanticPosition.current = null;
    revealSemanticPresentation();
    scrollToBottomAfterLayout(5);
  }, [persistedExpertWorkspaceActive, persistedExpertWorkspaceFallback, revealSemanticPresentation, scrollToBottomAfterLayout]);
  const expertRowsVirtualized = expertWorkspaceActive && parentRowsVirtualized;
  const virtualizationResetVersion = expertWorkspaceActive
    ? pinnedLayoutVersion
    : virtualizeRegularRows ? "ordinary-virtualized" : "ordinary-direct";
  const handleScroll = useCallback(() => {
    if (expertMemberFocused.current) return;
    const userInitiated = userScrollGestureActive.current
      || (wheelScrollGestureActive.current && wheelScrollGestureViewKey.current === semanticViewKey)
      || userScrollIntent.current;
    userScrollIntent.current = false;
    if (!userInitiated && protectedRestorePosition.current?.viewKey === semanticViewKey) {
      stick.current = false;
      return;
    }
    const wasPinned = stick.current;
    onScroll();
    const el = scrollRef.current;
    if (TRANSCRIPT_TRACE_ENABLED) {
      const pointerInitiated = userScrollGestureActive.current;
      const wheelInitiated = wheelScrollGestureActive.current && wheelScrollGestureViewKey.current === semanticViewKey;
      const keyInitiated = !pointerInitiated && !wheelInitiated && userInitiated;
      const traceState = transcriptWheelTraceStateByInstance?.get(traceInstance);
      const recentWheel = traceState?.view === traceView ? traceState : undefined;
      const now = Date.now();
      recordTranscriptTrace("scroll.handle", {
        view: traceView,
        source: pointerInitiated ? "pointer" : wheelInitiated ? "wheel" : keyInitiated ? "key" : "programmatic",
        scrollTop: el?.scrollTop ?? -1,
        max: el ? Math.max(0, el.scrollHeight - el.clientHeight) : -1,
        stickBefore: wasPinned,
        stickAfter: stick.current,
        batch: recentWheel?.batch ?? 0,
        wheelActive: wheelInitiated,
        frameAge: recentWheel?.frame ?? -1,
        sinceWheelMs: recentWheel ? Math.max(0, now - recentWheel.startedAt) : -1,
        sinceExpireMs: recentWheel?.expiredAt == null ? -1 : Math.max(0, now - recentWheel.expiredAt),
      });
    }
    if (!el || !viewKey) return;
    if (
      !userInitiated
      && pendingScrollAnchor.current !== null
      && pendingScrollViewKey.current === semanticViewKey
    ) return;
    if (userInitiated) preservePinnedVirtualizationReset.current = false;
    const canonicalPosition = virtualLayoutReplacementAnchor.current;
    if (
      !userInitiated
      && canonicalPosition?.viewKey === semanticViewKey
    ) {
      setSemanticRestoreFailureViewKey("");
      stick.current = false;
      pendingScrollViewKey.current = semanticViewKey;
      pendingScrollAnchor.current = canonicalPosition.anchor;
      setScrollAnchorRestoreGeneration((generation) => generation + 1);
      return;
    }
    if (userInitiated && replacementPresentationHidden.current) {
      replacementPresentationHidden.current = false;
      virtualLayoutReplacementAnchor.current = null;
      if (revealedSemanticViewKey === semanticViewKey) {
        const presentation = persistedPresentationRef.current;
        presentation?.removeAttribute("aria-hidden");
        if (presentation) {
          presentation.inert = false;
          presentation.style.visibility = "";
          presentation.style.pointerEvents = "";
        }
        expertReturnPresentationDeferred.current = false;
      } else {
        revealSemanticPresentation();
      }
    }
    const upwardWheelAnchor = wheelScrollGestureAnchor.current;
    if (
      expertWorkspaceActive
      && expertRowsVirtualized
      && wheelScrollGestureActive.current
      && wheelScrollGestureViewKey.current === semanticViewKey
      && wheelScrollGestureDeltaY.current < 0
      && upwardWheelAnchor?.viewKey === semanticViewKey
      && !wasPinned
      && stick.current
    ) {
      stick.current = false;
      wheelScrollClampCandidate.current = upwardWheelAnchor;
      return;
    }
    if (
      preservePinnedVirtualizationReset.current
      && wasPinned
      && !userInitiated
      && pendingScrollAnchor.current === null
      && !stick.current
    ) {
      stick.current = true;
      latestSemanticPosition.current = null;
      presentationRestoreSnapshot.current = syncPresentationRestoreSnapshot(
        presentationRestoreSnapshot.current,
        semanticViewKey,
        tracePath,
        null,
      );
      writeScrollAnchor({ atBottom: true }, questions);
      pinToBottomAfterLayout(3);
      return;
    }
    if (stick.current) {
      latestSemanticPosition.current = null;
      presentationRestoreSnapshot.current = syncPresentationRestoreSnapshot(
        presentationRestoreSnapshot.current,
        semanticViewKey,
        tracePath,
        null,
      );
      if (userInitiated) pendingScrollAnchor.current = null;
      writeScrollAnchor({ atBottom: true }, questions);
      return;
    }
    let anchor = captureTranscriptAnchor(el, questions);
    let captureSource: "dom" | "virtual" | "none" = "none";
    let virtualCaptureMetrics: { offset: number; listTop: number; rowStart: number } | undefined;
    if (TRANSCRIPT_TRACE_ENABLED && anchor) captureSource = "dom";
    if (!anchor && userInitiated && expertWorkspaceActive && expertRowsVirtualized) {
      anchor = captureVirtualTranscriptAnchor(
        questions,
        virtualQuestionRowByAnchorIDRef.current,
        (row) => virtualRowsRef.current?.visualOffsetForIndex(virtualHotRowOffsetRef.current + row),
      );
      if (TRANSCRIPT_TRACE_ENABLED && anchor) {
        captureSource = "virtual";
        const row = virtualQuestionRowByAnchorIDRef.current.get(questionAnchorId(anchor.anchorMessageId));
        if (row !== undefined) {
          virtualCaptureMetrics = virtualRowsRef.current?.visualMetricsForIndex(virtualHotRowOffsetRef.current + row);
        }
      }
    }
    if (TRANSCRIPT_TRACE_ENABLED) {
      const traceState = transcriptWheelTraceStateByInstance?.get(traceInstance);
      const recentWheel = traceState?.view === traceView ? traceState : undefined;
      const questionIndex = anchor
        ? questions.findIndex((question) => question.id === anchor.anchorMessageId)
        : -1;
      recordTranscriptTrace("anchor.capture", {
        view: traceView,
        source: captureSource,
        found: Boolean(anchor),
        anchor: anchor ? transcriptTraceOpaqueToken(anchor.anchorMessageId, "a") : undefined,
        questionIndex,
        offset: anchor?.offset,
        listTop: virtualCaptureMetrics?.listTop,
        rowStart: virtualCaptureMetrics?.rowStart,
        batch: recentWheel?.batch ?? 0,
        frameAge: recentWheel?.frame ?? -1,
      });
    }
    if (!anchor) {
      if (userInitiated) pendingScrollAnchor.current = null;
      return;
    }
    const nextSemanticPosition: TranscriptSemanticPosition = {
      viewKey: semanticViewKey,
      anchor: { atBottom: false, anchorMessageId: anchor.anchorMessageId, offset: anchor.offset },
    };
    latestSemanticPosition.current = nextSemanticPosition;
    if (userInitiated) deferredExpertReturnInputOnly.current = false;
    if (
      expertWorkspaceActive
      && expertRowsVirtualized
      && wheelScrollGestureActive.current
      && wheelScrollGestureViewKey.current === semanticViewKey
      && wheelScrollGestureDeltaY.current < 0
    ) {
      wheelScrollGestureAnchor.current = nextSemanticPosition;
      if (wheelScrollClampCandidate.current?.viewKey === semanticViewKey) {
        wheelScrollClampCandidate.current = nextSemanticPosition;
      }
    }
    presentationRestoreSnapshot.current = syncPresentationRestoreSnapshot(
      presentationRestoreSnapshot.current,
      semanticViewKey,
      tracePath,
      latestSemanticPosition.current.anchor,
    );
    if (userInitiated) pendingScrollAnchor.current = null;
    writeScrollAnchor({
      atBottom: false,
      anchorMessageId: anchor.anchorMessageId,
      offset: anchor.offset,
    }, questions);
  }, [expertRowsVirtualized, expertWorkspaceActive, onScroll, pinToBottomAfterLayout, questions, revealSemanticPresentation, revealedSemanticViewKey, scrollRef, semanticViewKey, stick, traceInstance, tracePath, traceView, viewKey, writeScrollAnchor]);

  useLayoutEffect(() => {
    if (committedPresentation.current.viewKey !== semanticViewKey) clearWheelScrollGesture();
    // A cached team can become renderable when tabId binds, without a new
    // sidecar response. Use the same layout handoff as the asynchronous path.
    if (tracePath === "team-persisted" && !presentationRestoreSnapshot.current
      && restoreFinishedViewKey.current !== semanticViewKey) {
      presentationRestoreSnapshot.current = stagePresentationRestoreSnapshot(
        committedPresentation.current,
        semanticViewKey,
        latestSemanticPosition.current,
        true,
        stick.current,
      );
    }
    committedPresentation.current = { viewKey: semanticViewKey, path: tracePath };
  }, [clearWheelScrollGesture, semanticViewKey, stick, tracePath]);

  useLayoutEffect(() => {
    const previous = committedRowsVirtualization.current;
    const samePresentation = previous.viewKey === semanticViewKey
      && previous.initialized
      && previous.expertWorkspaceActive === expertWorkspaceActive;
    const shouldPreserveExpert = samePresentation
      && expertWorkspaceActive
      && expertRowsVirtualized
      && (!previous.expertVirtualized || previous.contentVersion !== virtualizationResetVersion);
    const shouldPreserveOrdinary = samePresentation
      && !expertWorkspaceActive
      && virtualizeRegularRows
      && !previous.ordinaryVirtualized;
    const shouldPreserve = (shouldPreserveExpert || shouldPreserveOrdinary) && stick.current;
    preservePinnedVirtualizationReset.current = shouldPreserve;
    committedRowsVirtualization.current = {
      viewKey: semanticViewKey,
      expertWorkspaceActive,
      expertVirtualized: expertRowsVirtualized,
      ordinaryVirtualized: virtualizeRegularRows,
      contentVersion: virtualizationResetVersion,
      initialized: true,
    };
    if (!shouldPreserve) return;
    let framesLeft = SCROLL_ANCHOR_RESTORE_FRAMES;
    const expire = () => {
      preservePinnedVirtualizationResetFrame.current = requestAnimationFrame(() => {
        preservePinnedVirtualizationResetFrame.current = null;
        framesLeft -= 1;
        if (framesLeft > 0) expire();
        else preservePinnedVirtualizationReset.current = false;
      });
    };
    expire();
    return () => {
      if (preservePinnedVirtualizationResetFrame.current !== null) {
        cancelAnimationFrame(preservePinnedVirtualizationResetFrame.current);
        preservePinnedVirtualizationResetFrame.current = null;
      }
      preservePinnedVirtualizationReset.current = false;
    };
  }, [expertRowsVirtualized, expertWorkspaceActive, semanticViewKey, stick, virtualizeRegularRows, virtualizationResetVersion]);

  useLayoutEffect(() => {
    if (restoreFinishedViewKey.current === semanticViewKey) {
      presentationRestoreSnapshot.current = null;
      return;
    }
    const consumed = consumePresentationRestoreSnapshot(
      presentationRestoreSnapshot.current,
      semanticViewKey,
      tracePath,
    );
    if (!consumed) return;
    presentationRestoreSnapshot.current = null;
    virtualLayoutReplacementAnchor.current = null;
    if (consumed.anchor) {
      pendingScrollViewKey.current = semanticViewKey;
      pendingScrollAnchor.current = consumed.anchor;
      if (scrollRef.current) restoreTranscriptAnchor(scrollRef.current, consumed.anchor.anchorMessageId, consumed.anchor.offset);
      setScrollAnchorRestoreGeneration((generation) => generation + 1);
      return;
    }
    pinToBottomAfterLayout(5);
  }, [pinToBottomAfterLayout, scrollRef, semanticViewKey, tracePath]);

  useEffect(() => {
    const anchor = pendingScrollAnchor.current;
    const restoreViewKey = semanticViewKey;
    const el = scrollRef.current;
    if (
      !anchor
      || pendingScrollViewKey.current !== restoreViewKey
      || !el
    ) return;
    const budget = restoreWindow.current ?? new TranscriptRestoreWindow(performance.now(), document.visibilityState !== "hidden");
    restoreWindow.current = budget;
    budget.resetSamples();
    let frame: number | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let attempts = 0;
    let disposed = false;
    let previousObservedTop = el.scrollTop;
    let virtualSeekUsed = false;
    let visibleJumpCount = 0;
    let visibleJumpTotalAbsDelta = 0;
    let visibleJumpSummaryRecorded = false;
    const presentationVisible = (tracePath === "team-legacy" || tracePath === "team-persisted")
      && !expertSemanticRestoreHidden;
    const recordVisibleJump = (
      beforeTop: number,
      afterTop: number,
      reason: "layout-shift" | "virtual-seek" | "anchor-correction",
      phase: "before-frame" | "restore-frame",
    ) => {
      if (!TRANSCRIPT_TRACE_ENABLED || !presentationVisible) return;
      const delta = afterTop - beforeTop;
      if (Math.abs(delta) <= 1) return;
      visibleJumpCount += 1;
      visibleJumpTotalAbsDelta += Math.abs(delta);
      recordTranscriptTrace("viewport.visible-jump", {
        view: traceViewRef.current,
        instance: traceInstance,
        path: tracePath,
        restoreGeneration: scrollAnchorRestoreGeneration,
        jumpIndex: visibleJumpCount,
        beforeTop,
        afterTop,
        delta,
        totalAbsDelta: visibleJumpTotalAbsDelta,
        reason,
        phase,
        anchor: transcriptTraceOpaqueToken(anchor.anchorMessageId, "a"),
        questionIndex: questionsRef.current.findIndex((question) => question.id === anchor.anchorMessageId),
        pending: true,
        deferred: deferExpertReturnPresentation,
        visible: true,
      });
    };
    const recordVisibleJumpSummary = (completionReason: "stable" | "incomplete" | "cancelled") => {
      if (
        visibleJumpSummaryRecorded
        || !TRANSCRIPT_TRACE_ENABLED
        || !presentationVisible
        || visibleJumpCount === 0
      ) return;
      visibleJumpSummaryRecorded = true;
      recordTranscriptTrace("viewport.visible-jump.summary", {
        view: traceViewRef.current,
        instance: traceInstance,
        path: tracePath,
        restoreGeneration: scrollAnchorRestoreGeneration,
        jumpCount: visibleJumpCount,
        totalAbsDelta: visibleJumpTotalAbsDelta,
        completionReason,
      });
    };
    const current = () => !disposed && transcriptMounted.current
      && pendingScrollAnchor.current === anchor
      && pendingScrollViewKey.current === restoreViewKey;
    const finish = (result: "exact" | "clamped" | "unaddressable" | "exhausted") => {
      if (!current()) return;
      restoreFinishedViewKey.current = restoreViewKey;
      pendingScrollAnchor.current = null;
      presentationRestoreSnapshot.current = null;
      virtualLayoutReplacementAnchor.current = null;
      detachedVirtualPresentation.current = null;
      replacementPresentationHidden.current = false;
      protectedRestorePosition.current = result === "exact" ? null : { viewKey: restoreViewKey, anchor };
      stick.current = false;
      setSemanticRestoreFailureViewKey(result === "exact" ? "" : restoreViewKey);
      revealSemanticPresentation();
      recordTranscriptTrace("anchor.restore.finish", { view: traceView, result, elapsed: budget.elapsed, attempts });
      recordVisibleJumpSummary(result === "exact" || result === "clamped" ? "stable" : "incomplete");
    };
    const checkBudget = () => {
      budget.advance(performance.now(), document.visibilityState !== "hidden");
      if (current() && budget.expired) finish("exhausted");
    };
    const wake = () => {
      checkBudget();
      if (current()) timer = setTimeout(wake, 100);
    };
    document.addEventListener("visibilitychange", checkBudget);
    wake();
    const seek = () => {
      if (!current()) return;
      frame = requestAnimationFrame(() => {
        frame = null;
        checkBudget();
        if (!current()) return;
        if (document.visibilityState === "hidden" || !hasItems || deferExpertReturnPresentation) {
          budget.resetSamples();
          seek();
          return;
        }
        attempts += 1;
        const container = scrollRef.current;
        if (!container) return;
        const beforeTop = TRANSCRIPT_TRACE_ENABLED ? container.scrollTop : 0;
        recordVisibleJump(previousObservedTop, beforeTop, "layout-shift", "before-frame");
        const anchorNode = findTranscriptAnchorNode(container, anchor.anchorMessageId);
        const virtualRow = virtualQuestionRowByAnchorIDRef.current.get(questionAnchorId(anchor.anchorMessageId));
        const virtualRows = virtualRowsRef.current;
        const row = virtualRow === undefined ? undefined : virtualHotRowOffsetRef.current + virtualRow;
        const beforeAnchorCorrection = container.scrollTop;
        const domSuccess = anchorNode
          ? restoreTranscriptAnchor(container, anchor.anchorMessageId, anchor.offset)
          : false;
        if (!anchorNode && row !== undefined && virtualRows && !virtualSeekUsed) {
          virtualSeekUsed = true;
          recordTranscriptTrace("anchor.virtual-seek", { view: traceViewRef.current, row });
          const beforeVirtualSeek = container.scrollTop;
          virtualRows.scrollToIndex(row, "start");
          recordVisibleJump(beforeVirtualSeek, container.scrollTop, "virtual-seek", "restore-frame");
        }
        recordVisibleJump(beforeAnchorCorrection, container.scrollTop, "anchor-correction", "restore-frame");
        previousObservedTop = container.scrollTop;
        const restoredNode = findTranscriptAnchorNode(container, anchor.anchorMessageId);
        const measuredOffset = restoredNode
          ? -(restoredNode.getBoundingClientRect().top - container.getBoundingClientRect().top)
          : undefined;
        const error = measuredOffset === undefined ? Number.POSITIVE_INFINITY : Math.abs(measuredOffset - anchor.offset);
        const layoutRevision = virtualRows?.layoutRevision();
        const result = budget.sample(anchor.offset, measuredOffset === undefined ? undefined : {
          offset: measuredOffset,
          scrollTop: container.scrollTop,
          maxScrollTop: Math.max(0, container.scrollHeight - container.clientHeight),
          clientHeight: container.clientHeight,
        });
        if (TRANSCRIPT_TRACE_ENABLED) {
          recordTranscriptTrace("anchor.restore.attempt", {
            view: traceViewRef.current,
            anchor: transcriptTraceOpaqueToken(anchor.anchorMessageId, "a"),
            questionIndex: questionsRef.current.findIndex((question) => question.id === anchor.anchorMessageId),
            offset: anchor.offset,
            row: virtualRow === undefined ? -1 : virtualHotRowOffsetRef.current + virtualRow,
            domSuccess,
            error: Number.isFinite(error) ? error : -1,
            layoutRevision: layoutRevision ?? -1,
            beforeTop,
            afterTop: container.scrollTop,
            frame: attempts,
          });
        }
        if (result) {
          finish(result);
          return;
        }
        if (!anchorNode && row === undefined && !hydrating) {
          finish("unaddressable");
          return;
        }
        seek();
      });
    };
    seek();
    return () => {
      disposed = true;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", checkBudget);
      if (frame !== null) {
        cancelAnimationFrame(frame);
      }
      recordVisibleJumpSummary("cancelled");
    };
  }, [deferExpertReturnPresentation, expertSemanticRestoreHidden, hasItems, hydrating, ordinarySemanticRestoreHidden, questionIdentity, scrollAnchorRestoreGeneration, scrollRef, revealSemanticPresentation, semanticViewKey, stick, tracePath, traceView]);

  useEffect(() => {
    if (!revealNotice || !consumeNoticeReveal(revealNotice, semanticViewKey)) return;
    restoreFinishedViewKey.current = semanticViewKey;
    protectedRestorePosition.current = null;
    setSemanticRestoreFailureViewKey("");
    pendingScrollAnchor.current = null;
    latestSemanticPosition.current = null;
    presentationRestoreSnapshot.current = null;
    virtualLayoutReplacementAnchor.current = null;
    if (expertReturnPresentationDeferred.current) {
      deferredExpertReturnInputOnly.current = true;
      deferredExpertReturnRevealBottom.current = activeExpert?.mode === "team" || expertWorkspaceActive;
      revealSemanticPresentation();
    }
    clearUserScrollIntent();
    scrollToBottomAfterLayout(5);
  }, [activeExpert?.mode, clearUserScrollIntent, expertWorkspaceActive, revealNotice, revealSemanticPresentation, scrollToBottomAfterLayout, semanticViewKey]);

  const traceRenderSignature = `${traceView}|${tracePath}|${items.length}|${regularRows.length}|${hotZoneRows.length}|${displayMode}|${running}|${virtualizeRegularRows}`;
  const lastTraceRenderSignature = useRef("");
  useLayoutEffect(() => {
    if (!TRANSCRIPT_TRACE_ENABLED) return;
    if (lastTraceRenderSignature.current === traceRenderSignature) return;
    lastTraceRenderSignature.current = traceRenderSignature;
    const el = scrollRef.current;
    const snapshot = (event: string, persist: boolean) => recordTranscriptTrace(event, {
      ...transcriptTraceMemorySnapshot(),
      view: traceView,
      path: tracePath,
      mode: displayMode,
      itemCount: items.length,
      running,
      regularRowCount: regularRows.length,
      hotRowCount: hotZoneRows.length,
      virtualized: expertWorkspaceActive ? expertRowsVirtualized : virtualizeRegularRows,
      mountedRows: el?.querySelectorAll(".transcript-virtual-row").length ?? 0,
      toolCards: el?.querySelectorAll(".tool").length ?? 0,
      domNodes: el?.querySelectorAll("*").length ?? 0,
      scrollTop: el?.scrollTop ?? -1,
      scrollHeight: el?.scrollHeight ?? -1,
      clientHeight: el?.clientHeight ?? -1,
      stick: stick.current,
      pendingAnchor: pendingScrollAnchor.current !== null,
    }, { persist });
    snapshot("view.commit", true);
    recordTranscriptTrace("raf.schedule", { view: traceView, owner: "view-after-frame" });
    let fired = false;
    const frame = requestAnimationFrame(() => {
      fired = true;
      recordTranscriptTrace("raf.fire", { view: traceView, owner: "view-after-frame" });
      snapshot("view.after-frame", true);
    });
    return () => {
      cancelAnimationFrame(frame);
      if (!fired) recordTranscriptTrace("raf.cancel", { view: traceView, owner: "view-after-frame" });
    };
  }, [traceRenderSignature]);

  // ── Assemble rendered output ──────────────────────────────────────────────
  // Warm/cold zone is a separate memo'd WarmZone component so streaming tokens
  // don't rebuild it. The hot zone uses LiveAssistantMessage (reads live from
  // LiveStreamContext) so streaming updates are captured immediately.
  return (
    <div className="transcript-shell">
      {semanticRestoreFailed && (
        <div className="transcript__restore-status" role="status" onPointerDown={stopRecoveryControlPropagation} onTouchStart={stopRecoveryControlPropagation} onKeyDown={stopRecoveryControlPropagation}>
          <span>{t("transcript.restoreUnavailable")}</span>
          <button type="button" onClick={retrySemanticRestore}>{t("common.retry")}</button>
          <button type="button" onClick={showCurrentSemanticPresentation}>{t("transcript.showCurrentContent")}</button>
        </div>
      )}
      <div
        className={`transcript${empty ? " transcript--empty" : ""}`}
        data-drag-scroll={dragScroll || undefined}
        ref={bindScrollElement}
        onScroll={handleScroll}
        onClickCapture={captureProjectDragClick}
        onWheel={beginWheelScrollGesture}
        onTouchStart={beginUserScrollGesture}
        onTouchEnd={clearUserScrollIntent}
        onTouchCancel={clearUserScrollIntent}
        onPointerDown={beginProjectDragScroll}
        onPointerMove={moveProjectDragScroll}
        onPointerUp={finishProjectDragScroll}
        onPointerCancel={finishProjectDragScroll}
        onLostPointerCapture={finishProjectDragScroll}
        onKeyDown={markKeyboardScrollIntent}
      >
        {empty && (!hydrating || showWelcomeDuringHydration) && <Welcome onPrompt={onPrompt} onSelectScenario={onSelectScenario} scene={scene} sceneState={sceneState} sceneSelectionAvailability={sceneSelectionAvailability} onSelectScene={onSelectScene} variant={welcomeVariant} capabilityPresentation={capabilityWelcomePresentation} />}
        {empty && existingSessionLoading && <SessionLoadingPresentation feedbackVisible={sessionLoadingFeedbackVisible} />}
        <LiveStreamContext.Provider value={live}>
          <TranscriptTraceViewContext.Provider value={traceView}>
          <ToolDisclosureProvider key={semanticViewKey}>
          <div ref={entranceRef}>
            {legacyExpertWorkspaceActive ? (
              <>
                {deferExpertReturnPresentation && <SessionLoadingPresentation feedbackVisible={sessionLoadingFeedbackVisible} />}
                <div
                  hidden={deferExpertReturnPresentation}
                  aria-hidden={deferExpertReturnPresentation ? "true" : undefined}
                  style={deferExpertReturnPresentation ? undefined : { display: "contents" }}
                >
                  {hasOlderHistory && (
                    <button
                      type="button"
                      className="warm-collapse"
                      disabled={loadingOlderHistory}
                      onClick={() => void onLoadOlderHistory?.()}
                    >
                      {loadingOlderHistory ? t("transcript.loadingEarlierHistory") : t("transcript.loadEarlierHistory")}
                    </button>
                  )}
                  <AgentWorkspace
                    items={expertWorkspaceItems}
                    subcalls={sessionItemsByParent}
                    tabId={tabId}
                    expertTeam={expertWorkspaceTeam}
                    memberByItemId={expertWorkspaceMemberByItemId}
                    overviewContent={hotZoneNodes}
                    selectionScopeKey={semanticViewKey}
                    scrollElement={scrollElement}
                    overviewVirtualRowsRef={bindVirtualRows}
                  />
                </div>
              </>
            ) : !empty ? (
              <>
                {(persistedExpertWorkspaceHidden || ordinarySemanticRestoreHidden) && <SessionLoadingPresentation feedbackVisible={sessionLoadingFeedbackVisible} overlay />}
                <div
                  ref={persistedPresentationRef}
                  data-testid={persistedExpertWorkspaceActive ? "persisted-expert-presentation" : "ordinary-semantic-presentation"}
                  aria-hidden={persistedExpertWorkspaceHidden || ordinarySemanticRestoreHidden ? "true" : undefined}
                  inert={persistedExpertWorkspaceHidden || ordinarySemanticRestoreHidden ? true : undefined}
                  style={persistedExpertWorkspaceHidden || ordinarySemanticRestoreHidden
                    ? { display: "contents", visibility: "hidden", pointerEvents: "none" }
                    : { display: "contents" }}
                >
                  <ExpertTeamSessionView
                    team={persistedExpertWorkspaceActive ? sessionExpertTeam : null}
                    tabId={tabId ?? ""}
                    childItemsByRef={childItemsByRef}
                    rows={renderedParentRows}
                    selectionScopeKey={semanticViewKey}
                    onMemberFocusChange={handleExpertMemberFocusChange}
                    scrollElement={scrollElement}
                    overviewVirtualRowsRef={bindVirtualRows}
                  />
                </div>
              </>
            ) : null}
          </div>
          </ToolDisclosureProvider>
          </TranscriptTraceViewContext.Provider>
        </LiveStreamContext.Provider>
      </div>

      {!empty && showQuestionNav && (
        <QuestionJumpBar questions={questions} onJump={handleJumpToQuestion} />
      )}

      {!empty && !isAtBottom && (
        <button
          type="button"
          className="transcript__jump-bottom"
          onClick={() => {
            restoreFinishedViewKey.current = semanticViewKey;
            protectedRestorePosition.current = null;
            pendingScrollAnchor.current = null;
            latestSemanticPosition.current = null;
            presentationRestoreSnapshot.current = null;
            virtualLayoutReplacementAnchor.current = null;
            setSemanticRestoreFailureViewKey("");
            revealSemanticPresentation();
            scrollToBottomAfterLayout(2);
          }}
          aria-label={t("transcript.jumpToBottom")}
          title={t("transcript.jumpToBottom")}
        >
          <ArrowDown size={18} strokeWidth={2.2} aria-hidden="true" />
        </button>
      )}
    </div>
  );
}

// ── WarmZone sub-component (React.memo for streaming isolation) ────────────
// Receives structural props only; reads streaming state (items, live) via refs
// so it never invalidates on streaming token arrival.

const WarmZone = memo(function WarmZone({
  turnGroups,
  expandedWarmTurns,
  shownWarmStart,
  coldTurnCount,
  scrollRef,
  warmItems,
  warmSubcalls,
  warmUserTurn,
  warmCheckpoints,
  warmOpenAction,
  warmActionPending,
  warmRewindDisabled,
  warmActionHoverMenus,
  warmOnRewind,
  warmSetOpenAction,
  warmOnEdit,
  tabId,
  creationMode,
  assistantGeneratedMedia,
  onToggleColdPage,
  onToggleWarmTurn,
}: {
  turnGroups: TurnGroup[];
  expandedWarmTurns: ReadonlySet<number>;
  shownWarmStart: number;
  coldTurnCount: number;
  scrollRef: React.RefObject<HTMLDivElement | null>;
  warmItems: readonly Item[];
  warmSubcalls: ReadonlyMap<string, ToolItem[]>;
  warmUserTurn: ReadonlyMap<string, number>;
  warmCheckpoints: ReadonlyMap<number, CheckpointMeta>;
  warmOpenAction: OpenTurnAction | null;
  warmActionPending: boolean;
  warmRewindDisabled: boolean;
  warmActionHoverMenus: boolean;
  warmOnRewind: ((turn: number, scope: string) => void) | undefined;
  warmSetOpenAction: (action: OpenTurnAction | null) => void;
  warmOnEdit?: (turn: number, displayText: string, submitText?: string) => boolean | void | Promise<boolean | void>;
  tabId?: string;
  creationMode?: boolean;
  assistantGeneratedMedia: ReadonlyMap<string, GeneratedMediaReference[]>;
  onToggleColdPage: () => void;
  onToggleWarmTurn: (g: number, expand: boolean) => void;
}) {
  const t = useT();
  const out: React.ReactNode[] = [];

  // 1. Cold zone: paginated warm turns (show more button).
  if (coldTurnCount > 0) {
    out.push(
      <button
        key="cold-load-more"
        type="button"
        className="warm-collapse"
        onClick={onToggleColdPage}
      >
        {t("transcript.showEarlierHistory", { n: coldTurnCount })}
      </button>,
    );
  }

  // 2. Warm zone: collapsed/expanded warm turn cards.
  let warmStartTurn = 0;
  if (turnGroups.length > HOT_TURNS) {
    warmStartTurn = turnGroups.length - HOT_TURNS - shownWarmStart;
    for (let g = warmStartTurn; g < turnGroups.length - HOT_TURNS; g++) {
      const group = turnGroups[g];
      if (!group) continue;
      const expanded = expandedWarmTurns.has(g);

      if (expanded) {
        const userText = group.userItem.kind === "user" ? group.userItem.text : "";
        out.push(
          <WarmTurnCard
            key={`warm-${g}`}
            userText={warmUserPreview(userText)}
            assistantPreview={group.assistantPreview}
            toolCount={group.toolCount}
            failedTaskCount={taskFailureCount(warmItems.slice(group.startIdx, group.endIdx))}
            expanded={true}
            onToggle={() => onToggleWarmTurn(g, false)}
          >
            {/* Expanded warm turns render items that are stable (never the
                streaming turn), so this captures items/live via a ref. */}
            <WarmTurnItems
              startIdx={group.startIdx}
              endIdx={group.endIdx}
              items={warmItems}
              subcalls={warmSubcalls}
              userTurnMap={warmUserTurn}
              checkpoints={warmCheckpoints}
              openAction={warmOpenAction}
              actionPending={warmActionPending}
              rewindDisabled={warmRewindDisabled}
              actionHoverMenus={warmActionHoverMenus}
              onRewind={warmOnRewind}
              setOpenAction={warmSetOpenAction}
              onEdit={warmOnEdit}
              tabId={tabId}
              creationMode={creationMode}
              assistantGeneratedMedia={assistantGeneratedMedia}
            />
          </WarmTurnCard>,
        );
      } else {
        const userText = group.userItem.kind === "user" ? group.userItem.text : "";
        out.push(
          <WarmTurnCard
            key={`warm-${g}`}
            userText={warmUserPreview(userText)}
            assistantPreview={group.assistantPreview}
            toolCount={group.toolCount}
            failedTaskCount={taskFailureCount(warmItems.slice(group.startIdx, group.endIdx))}
            expanded={false}
            onToggle={() => {
              onToggleWarmTurn(g, true);
              const el = scrollRef.current;
              const node = el ? findTranscriptAnchorNode(el, group.userItem.id) : null;
              if (el && node) {
                requestAnimationFrame(() => {
                  el.scrollTo({ top: scrollTopForTranscriptNode(el, node, 80), behavior: "smooth" });
                });
              }
            }}
          />,
        );
      }
    }
  }

  return out;
});

function WarmTurnItems({
  startIdx,
  endIdx,
  items,
  subcalls,
  userTurnMap,
  checkpoints,
  openAction,
  actionPending,
  rewindDisabled,
  actionHoverMenus,
  onRewind,
  setOpenAction,
  onEdit,
  tabId,
  creationMode = false,
  assistantGeneratedMedia,
}: {
  startIdx: number;
  endIdx: number;
  items: readonly Item[];
  subcalls: ReadonlyMap<string, ToolItem[]>;
  userTurnMap: ReadonlyMap<string, number>;
  checkpoints: ReadonlyMap<number, CheckpointMeta>;
  openAction: OpenTurnAction | null;
  actionPending: boolean;
  rewindDisabled: boolean;
  actionHoverMenus: boolean;
  onRewind: ((turn: number, scope: string) => void) | undefined;
  setOpenAction: (action: OpenTurnAction | null) => void;
  onEdit?: (turn: number, displayText: string, submitText?: string) => boolean | void | Promise<boolean | void>;
  tabId?: string;
  creationMode?: boolean;
  assistantGeneratedMedia: ReadonlyMap<string, GeneratedMediaReference[]>;
}) {
  const nodes: React.ReactNode[] = [];
  let actionText = "";
  let actionReady = false;
  let actionTurnMeta: TurnMetadata | undefined;
  let activeTurn: number | undefined;
  const pushTurnActions = () => {
    if (activeTurn == null || !actionReady || actionText.trim() === "") return;
    const turn = activeTurn;
    const openMenu = openAction && openAction.turn === turn ? openAction.menu : null;
    nodes.push(
      <TurnActions
        key={`ta-${turn}`}
        text={actionText}
        turn={turn}
        openMenu={openMenu}
        onOpenMenu={(menu) => setOpenAction(menu ? { turn, menu } : null)}
        checkpoint={checkpoints.get(turn)}
        actionPending={actionPending}
        rewindDisabled={rewindDisabled}
        hoverMenus={actionHoverMenus}
        turnMeta={actionTurnMeta}
        onRewind={(targetTurn, scope) => {
          onRewind?.(targetTurn, scope);
          setOpenAction(null);
        }}
      />,
    );
    actionText = "";
    actionReady = false;
    actionTurnMeta = undefined;
  };

  // Group consecutive completed read-only tools into ReadOnlyBatch
  const roBatch: ToolItem[] = [];
  const toolBatch: ToolItem[] = [];
  let toolBatchKind: ToolGroupKind | null = null;
  const flushRO = () => {
    if (roBatch.length === 0) return;
    nodes.push(<ReadOnlyBatch key={`rob-${roBatch[0].id}`} items={[...roBatch]} subcalls={subcalls} tabId={tabId} />);
    roBatch.length = 0;
  };
  const flushToolBatch = () => {
    if (!toolBatchKind || toolBatch.length === 0) return;
    nodes.push(<ToolGroup key={`tg-${toolBatch[0].id}`} kind={toolBatchKind} items={[...toolBatch]} subcalls={subcalls} tabId={tabId} />);
    toolBatch.length = 0;
    toolBatchKind = null;
  };

  for (let i = startIdx; i < endIdx && i < items.length; i++) {
    const it = items[i];

    // Completed read-only tools → batch into ReadOnlyBatch
    if (creationMode && it.kind === "tool" && isCreationGroupableTool(it as ToolItem)) {
      const kind = toolGroupKind(it as ToolItem);
      if (kind) {
        if (toolBatchKind && toolBatchKind !== kind) flushToolBatch();
        toolBatchKind = kind;
        toolBatch.push(it as ToolItem);
        continue;
      }
    }
    if (!creationMode && it.kind === "tool" && !it.parentId && it.name !== "todo_write" && it.name !== "exit_plan_mode" && isReadOnlyTool(it.name)) {
      roBatch.push(it as ToolItem);
      continue;
    }
    flushToolBatch();
    flushRO();

    switch (it.kind) {
      case "user": {
        pushTurnActions();
        const tn = userTurnMap.get(it.id);
        const checkpoint = tn == null ? undefined : checkpoints.get(tn);
        activeTurn = tn;
        nodes.push(
          <UserMessage
            key={it.id}
            text={it.text}
            submitText={it.submitText}
            failed={it.failed}
            createdAt={it.createdAt}
            turn={tn}
            anchorId={questionAnchorId(it.id)}
            onEdit={onEdit}
            editDisabled={rewindDisabled || !checkpoint?.canConversation}
          />,
        );
        break;
      }
      case "assistant": {
        nodes.push(<AssistantMessage key={it.id} item={it} defaultExpanded={false} creationMode={creationMode} tabId={tabId} generatedMedia={assistantGeneratedMedia.get(it.id)} />);
        if (!it.streaming && it.text.trim() !== "") {
          actionText = it.text;
          actionReady = true;
          actionTurnMeta = it.turn;
        }
        break;
      }
      case "tool": {
        if (it.parentId) break;
        if (it.name === "todo_write") break;
        if (it.name === "exit_plan_mode") break;
        nodes.push(<ToolCard key={it.id} item={it} subcalls={subcalls.get(it.id)} tabId={tabId} />);
        break;
      }
      case "phase": nodes.push(<PhaseCard key={it.id} text={it.text} />); break;
      case "notice": nodes.push(<NoticeCard key={it.id} level={it.level} text={it.text} />); break;
      case "compaction": nodes.push(<CompactionCard key={it.id} item={it} />); break;
    }
  }
  flushToolBatch();
  flushRO();
  pushTurnActions();
  return nodes;
}

// ── Warm turn summary card ────────────────────────────────────────────────────

function WarmTurnCard({
  userText,
  assistantPreview,
  toolCount,
  failedTaskCount,
  expanded,
  onToggle,
  children,
}: {
  userText: string;
  assistantPreview: string;
  toolCount: number;
  failedTaskCount: number;
  expanded: boolean;
  onToggle: () => void;
  children?: React.ReactNode;
}) {
  const t = useT();
  const contentRef = useRef<HTMLDivElement>(null);
  const prevHeightRef = useRef(0);
  useGSAPCollapse(contentRef, expanded, { prevHeight: prevHeightRef.current });
  // Always render both children so the container's scrollHeight reflects
  // the correct content at all times.  The inactive one is display:none.
  return (
    <div className={`warm-turn${expanded ? " warm-turn--expanded" : ""}`}>
      <button
        type="button"
        className="warm-turn__head"
        onClick={() => {
          // Capture height before DOM swap so the collapse animation
          // starts from the correct (expanded) height.
          const el = contentRef.current;
          if (el) {
            el.style.height = "auto";
            prevHeightRef.current = el.scrollHeight;
          }
          onToggle();
        }}
        aria-expanded={expanded}
      >
        <span className="warm-turn__chevron">
          <ChevronRight className={expanded ? "warm-turn__chevron--open" : ""} size={13} />
        </span>
        <span className="warm-turn__preview">{userText}</span>
        <span className="warm-turn__meta">
          {toolCount > 0 && <span>{t("transcript.toolCount", { n: toolCount })}</span>}
          {failedTaskCount > 0 && <span>{t("runtimeNotice.taskFailedAttempts", { n: failedTaskCount })}</span>}
        </span>
      </button>
      <div ref={contentRef} className="warm-turn__content">
        <div className="warm-turn__body" style={{ display: expanded ? undefined : "none" }}>{children}</div>
        {assistantPreview && (
          <div className="warm-turn__assistant" style={{ display: expanded ? "none" : undefined }}>{assistantPreview}</div>
        )}
      </div>
    </div>
  );
}

// ── TurnCollapse: compact mode grouping ──────────────────────────────────────

type TurnCollapseProps = {
  items: Item[];       // intermediate items (tools, reasoning, phase)
  durationMs: number;  // summed tool execution time across the batch; 0 when unknown
  mode: DisplayMode;
  subcalls: Map<string, ToolItem[]>;
  tabId?: string;
  creationMode?: boolean;
  expertTeam?: AgentWorkspaceExpertTeam | null;
  memberByItemId?: AgentWorkspaceMemberMap;
  expertTeamFlow?: ExpertTeamFlowModel | null;
  active?: boolean;
};

function TurnCollapse({ items, durationMs, mode, subcalls, tabId, creationMode = false, expertTeam, memberByItemId, expertTeamFlow, active = false }: TurnCollapseProps) {
  const t = useT();
  const containsMediaTool = items.some((item) => item.kind === "tool" && isGeneratedMediaTool(item.name));
  const [open, setOpen] = useState(active || containsMediaTool);
  const previousActive = useRef(active);
  const bodyRef = useRef<HTMLDivElement>(null);
  useGSAPCollapse(bodyRef, open);
  useEffect(() => {
    if (containsMediaTool) setOpen(true);
  }, [containsMediaTool]);
  useEffect(() => {
    if (active) setOpen(true);
    else if (previousActive.current) setOpen(false);
    previousActive.current = active;
  }, [active]);

  // Keep only items the body will actually render — an expandable fold over
  // nothing is worse than no fold.
  const displayItems = useMemo(() => {
    return items.filter((it) => {
      if (it.kind === "assistant") {
        if (it.streaming) return true;
        if (it.text.trim() !== "") return true;
        return Boolean(it.reasoning);
      }
      if (it.kind === "phase") return true;
      if (it.kind === "notice" || it.kind === "compaction") return true;
      if (it.kind !== "tool") return false;
      if (it.parentId || it.name === "todo_write" || it.name === "exit_plan_mode") return false;
      return true;
    });
  }, [items, mode]);

  const seconds = Math.round(durationMs / 1000);
  const failedTaskCount = taskFailureCount(displayItems);
  const stepCount = displayItems.length + (expertTeamFlow ? 1 : 0);
  const label = active
    ? t("transcript.turnProcessRunning", { n: stepCount })
    : seconds > 0
      ? t("transcript.turnProcessCompletedDuration", { n: stepCount, s: seconds })
      : t("transcript.turnProcessCompleted", { n: stepCount });

  if (displayItems.length === 0 && !expertTeamFlow && !active) return null;

  // Pre-compute body: group consecutive completed read-only tools into ReadOnlyBatch
  const body: ReactNode[] = [];
  const roBatch: ToolItem[] = [];
  const toolBatch: ToolItem[] = [];
  let toolBatchKind: ToolGroupKind | null = null;
  const flushRO = () => {
    if (roBatch.length === 0) return;
    body.push(<ReadOnlyBatch key={`rob-${roBatch[0].id}`} items={[...roBatch]} subcalls={subcalls} tabId={tabId} />);
    roBatch.length = 0;
  };
  const flushToolBatch = () => {
    if (!toolBatchKind || toolBatch.length === 0) return;
    body.push(<ToolGroup key={`tg-${toolBatch[0].id}`} kind={toolBatchKind} items={[...toolBatch]} subcalls={subcalls} tabId={tabId} />);
    toolBatch.length = 0;
    toolBatchKind = null;
  };
  for (const it of displayItems) {
    if (creationMode && it.kind === "tool" && isCreationGroupableTool(it as ToolItem)) {
      const kind = toolGroupKind(it as ToolItem);
      if (kind) {
        if (toolBatchKind && toolBatchKind !== kind) flushToolBatch();
        toolBatchKind = kind;
        toolBatch.push(it as ToolItem);
        continue;
      }
    }
    if (!creationMode && it.kind === "tool" && !it.parentId && it.name !== "todo_write" && it.name !== "exit_plan_mode" && it.status !== "running" && isReadOnlyTool(it.name)) {
      roBatch.push(it as ToolItem);
      continue;
    }
    flushToolBatch();
    flushRO();
    switch (it.kind) {
      case "tool":
        if (it.parentId) break;
        if (it.name === "todo_write") break;
        if (it.name === "exit_plan_mode") break;
        body.push(<ToolCard key={it.id} item={it as ToolItem} subcalls={subcalls.get(it.id)} tabId={tabId} expertTeam={expertTeam} memberByItemId={memberByItemId} />);
        break;
      case "phase": body.push(<PhaseCard key={it.id} text={it.text} />); break;
      case "notice": body.push(<NoticeCard key={it.id} level={it.level} text={it.text} />); break;
      case "compaction": body.push(<CompactionCard key={it.id} item={it} />); break;
      case "assistant": {
        body.push(<LiveAssistantMessage key={it.id} item={it as AssistantItem} defaultExpanded={false} creationMode={creationMode} tabId={tabId} />);
        break;
      }
    }
  }
  flushToolBatch();
  flushRO();
  if (expertTeamFlow) body.push(<ExpertTeamFlowCard key="expert-team-flow" model={expertTeamFlow} />);

  return (
    <div className={`turn-collapse${open ? " turn-collapse--open" : ""}`} data-kind="process" data-state={active ? "running" : "completed"} data-entrance={displayItems[0]?.id || undefined}>
      <button
        type="button"
        className="reasoning__head"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        <ChevronRight className={`reasoning__chevron${open ? " reasoning__chevron--open" : ""}`} size={12} />
        <span className="turn-collapse__label">{label}{failedTaskCount > 0 ? ` · ${t("runtimeNotice.taskFailedAttempts", { n: failedTaskCount })}` : ""}</span>
      </button>
      <div
        ref={bodyRef}
        className="turn-collapse__body"
        style={open ? undefined : { height: 0 }}
        aria-hidden={!open}
      >
        {body}
      </div>
    </div>
  );
}

// ── JumpBar, PhaseCard, NoticeCard, CompactionCard ────────────────────────────

function QuestionJumpBar({ questions, onJump }: { questions: QuestionAnchor[]; onJump: (question: QuestionAnchor) => void }) {
  const t = useT();
  const [hovered, setHovered] = useState<number | null>(null);
  const [active, setActive] = useState<number | null>(null);
  const barRef = useRef<HTMLDivElement>(null);
  const previewTop = useRef(0);
  const [showPreview, setShowPreview] = useState(false);

  useEffect(() => {
    if (questions.length === 0) return;
    setActive(questions[questions.length - 1]?.turn ?? null);
  }, [questions]);

  useEffect(() => {
    if (active === null) return;
    const el = barRef.current?.querySelector(`[data-turn="${active}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const hoverIdx = hovered !== null ? questions.findIndex((question) => question.turn === hovered) : -1;
  const hoveredQuestion = hovered !== null ? questions.find((question) => question.turn === hovered) : undefined;

  const closestQuestionFromY = (clientY: number): { question: QuestionAnchor; previewY: number } | null => {
    const el = barRef.current;
    if (!el) return null;
    const markers = el.querySelectorAll<HTMLElement>(".jump-item");
    const barRect = el.getBoundingClientRect();
    let closest = -1;
    let closestDist = Infinity;
    let closestY = 0;
    markers.forEach((item, index) => {
      const rect = item.getBoundingClientRect();
      const midY = rect.top + rect.height / 2;
      const dist = Math.abs(clientY - midY);
      if (dist < closestDist) {
        closestDist = dist;
        closest = index;
        closestY = midY - barRect.top;
      }
    });
    const question = questions[closest];
    if (!question) return null;
    return { question, previewY: closestY };
  };

  const onMove = (e: ReactMouseEvent<HTMLDivElement>) => {
    const closest = closestQuestionFromY(e.clientY);
    if (!closest) return;
    previewTop.current = closest.previewY;
    setHovered(closest.question.turn);
    setShowPreview(true);
  };

  const scrollTo = (question: QuestionAnchor) => {
    setActive(question.turn);
    onJump(question);
  };

  const onRailMouseDown = (e: ReactMouseEvent<HTMLDivElement>) => {
    const closest = closestQuestionFromY(e.clientY);
    if (!closest) return;
    e.preventDefault();
    previewTop.current = closest.previewY;
    setHovered(closest.question.turn);
    setShowPreview(true);
    scrollTo(closest.question);
  };

  const onItemMouseDown = (e: ReactMouseEvent<HTMLButtonElement>, question: QuestionAnchor) => {
    e.preventDefault();
    scrollTo(question);
  };

  const dotProps = (
    idx: number,
    turn: number,
  ): { style: CSSProperties; "data-d"?: string } => {
    const isActive = active === turn;
    if (hoverIdx < 0) {
      return { style: { width: isActive ? 18 : 12, background: isActive ? "var(--accent)" : undefined } };
    }
    const d = Math.abs(idx - hoverIdx);
    const width = d === 0 ? 32 : d === 1 ? 20 : d === 2 ? 14 : isActive ? 18 : 12;
    const background = d <= 2 ? undefined : isActive ? "var(--accent)" : undefined;
    return {
      style: { width, transitionDelay: `${d * 20}ms`, background },
      "data-d": d <= 2 ? String(d) : undefined,
    };
  };

  return (
    <nav
      className="jump-bar"
      ref={barRef}
      aria-label={t("questionNav.label")}
      onMouseMove={onMove}
      onMouseLeave={() => {
        setHovered(null);
        setShowPreview(false);
      }}
    >
      <div className="jump-scroll" onMouseDown={onRailMouseDown} onClick={onRailMouseDown}>
        {questions.map((question, index) => (
          <button
            className="jump-item"
            key={question.id}
            type="button"
            data-turn={question.turn}
            aria-label={t("questionNav.jump", { n: question.turn + 1 })}
            onMouseDown={(e) => onItemMouseDown(e, question)}
            onClick={(e) => {
              e.stopPropagation();
              if (e.detail === 0) scrollTo(question);
            }}
          >
            <span className="jump-dot" {...dotProps(index, question.turn)} />
          </button>
        ))}
      </div>
      {showPreview && hoveredQuestion && (
        <div className="jump-preview" style={{ top: previewTop.current }} role="tooltip">
          <span className="jump-text">{hoveredQuestion.text}</span>
        </div>
      )}
    </nav>
  );
}

type CompactionItem = Extract<Item, { kind: "compaction" }>;
type NoticeItem = Extract<Item, { kind: "notice" }>;

function PhaseCard({ text }: { text: string }) {
  const t = useT();
  const visibleText = stripHiddenSystemPrompt(text);
  if (!visibleText.trim()) return null;
  return <div className="phase" data-entrance="true"><ProcessPhaseIcon size={12} /><span>{presentRuntimeEventText("phase", visibleText, t)}</span></div>;
}

function NoticeCard({ level, text }: { level: NoticeItem["level"]; text: string }) {
  const t = useT();
  const visibleText = stripHiddenSystemPrompt(text);
  if (!visibleText.trim() || isSilentMCPAvailabilityNotice(visibleText)) return null;
  return (
    <div className={`notice-line notice-line--${level}`} data-entrance="true">
      <span className="notice-line__icon">{level === "warn" ? "⚠ " : "ℹ "}</span>
      <span className="notice-line__text">{presentRuntimeEventText("notice", visibleText, t)}</span>
    </div>
  );
}

function CompactionCard({ item }: { item: CompactionItem }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const trigger = presentCompactionTrigger(item.trigger, t);
  if (item.pending) {
    return <div className="compaction compaction--pending" data-entrance={item.id}><ProcessCompactIcon size={12} /><span>{t("compaction.working")}</span></div>;
  }
  return (
    <div className="compaction" data-entrance={item.id}>
      <button type="button" className="compaction__head" onClick={() => setOpen((v) => !v)} aria-expanded={open}>
        <ProcessCompactIcon size={12} />
        <span>{t("compaction.title")}</span>
        <span className="compaction__meta">{t("compaction.messages", { n: item.messages })}{trigger ? ` · ${trigger}` : ""}</span>
        <ChevronRight className={open ? "compaction__chevron--open" : ""} size={12} />
      </button>
      {open && <pre className="compaction__body">{item.summary}</pre>}
    </div>
  );
}
