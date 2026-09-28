import type { Item, LiveStream, MessageActionState, ToolItem } from "./sessionItems";
import type { SessionViewSnapshot } from "./sessionViewSnapshot";
import { initialPromptProjection, type PromptProjectionState, type PromptReplyInput } from "./promptProjection";
import { initialOperationalProjection, type OperationalProjectionState } from "./operationalProjection";
import { initialTaskTreeState, type TaskTreeState } from "../../lib/taskTreeState";
import type {
  BalanceInfo,
  CheckpointMeta,
  ContextInfo,
  EffortInfo,
  HistoryMessage,
  JobView,
  Meta,
  WireApproval,
  WireAsk,
  WireEvent,
  WireUsage,
  TranscriptReplay,
} from "../../lib/types";

export type HydrateReason = "switch-tab" | "new-session" | "resume-session" | "open-topic" | "startup";
export type SessionSurfaceIntent = "blank" | "existing-session" | "unknown";
export type ComposerSelectionPhase = "pending" | "ready" | "degraded" | "updating-effort";

export type ComposerSelectionState = {
  phase: ComposerSelectionPhase;
  seq: number;
  modelRef: string;
  effortSnapshot?: string;
};

export interface State {
  transcriptJournalId?: string;
  transcriptThrough: number;
  transcriptSeen: Record<number, true>;
  items: Item[];
  running: boolean;
  turnActive: boolean;
  pendingPrompt: boolean;
  backgroundJobs: number;
  cancelRequested: boolean;
  cancellable: boolean;
  approval?: WireApproval;
  ask?: WireAsk;
  prompts: PromptProjectionState;
  operational: OperationalProjectionState;
  usage?: WireUsage;
  context: ContextInfo;
  meta?: Meta;
  balance?: BalanceInfo;
  effort?: EffortInfo;
  composerSelection: ComposerSelectionState;
  jobs: JobView[];
  checkpoints: CheckpointMeta[];
  hydrating: boolean;
  surfaceIntent: SessionSurfaceIntent;
  hydrateReason?: HydrateReason;
  hydrateError?: string;
  hydrateHistoryLoaded?: boolean;
  hydratePreview?: SessionViewSnapshot;
  historyFingerprint?: string;
  historyRevalidationSeq: number;
  historyBefore: number;
  historyGeneration: string;
  historyHasMore: boolean;
  historyLoadingOlder: boolean;
  hydratePlaceholderItems?: Item[];
  hydratePlaceholderTabId?: string;
  backendActivationPending: boolean;
  messageAction?: MessageActionState;
  currentAssistant?: string;
  rootFinalCandidateId?: string;
  runtimeTurnId?: string;
  rejectedRootCandidateKeys: Record<string, true>;
  childItemsByRef: Record<string, Item[]>;
  subagentStatusByRef: Record<string, string>;
  currentChildAssistantByRef: Record<string, string>;
  parallelItemsByCall: Record<string, ToolItem[]>;
  live?: LiveStream;
  pendingUser?: string;
  pendingLocalUser?: { id: string; text: string };
  pendingTranscriptUser?: { id: string; text: string; createdAt: number; journalId?: string; afterSequence: number };
  discardTurn?: boolean;
  turnStartAt: number;
  turnTokens: number;
  turnTotalTokens: number;
  turnCost: number;
  sessionTokens: number;
  sessionCost: number;
  sessionCurrency: string;
  retry?: { attempt: number; max: number };
  taskTrees: TaskTreeState;
  seq: number;
  sessionGen: number;
}

export type Action =
  | { type: "transcript_replay"; replay: TranscriptReplay; pending: WireEvent[] }
  | { type: "event"; e: WireEvent }
  | { type: "historical_turn_done"; e: WireEvent }
  | { type: "stream_batch"; events: WireEvent[] }
  | { type: "user"; text: string; submitText?: string; seq: number }
  | { type: "unsend" }
  | { type: "send_failed"; error: string }
  | { type: "backend_status"; running: boolean; pendingPrompt?: boolean; backgroundJobs?: number; cancelRequested?: boolean; cancellable?: boolean; activeTurnStartedAt?: number; activeTurnCompletionTokens?: number }
  | { type: "cancel_requested" }
  | { type: "meta"; meta: Meta }
  | { type: "optimistic_meta"; meta: Meta }
  | { type: "context"; context: ContextInfo }
  | { type: "balance"; balance: BalanceInfo }
  | { type: "turn_billing_settled"; taskId: string; chargedCredits: string }
  | { type: "turn_billing_results"; results: import("../../lib/types").TurnBillingResult[]; scopeKey: string; sessionPath: string; journalId?: string; sessionGen: number }
  | { type: "composer_selection_start"; seq: number; modelRef: string; invalidate: boolean; meta?: Meta }
  | { type: "composer_selection_ready"; seq: number; modelRef: string; meta: Meta; effort: EffortInfo }
  | { type: "composer_selection_degraded"; seq: number; modelRef: string; meta?: Meta }
  | { type: "composer_effort_update_start"; seq: number; modelRef: string; effort: EffortInfo }
  | { type: "composer_effort_update_done"; seq: number }
  | { type: "composer_selection_restore"; seq: number; meta?: Meta; effort?: EffortInfo; selection: ComposerSelectionState }
  | { type: "jobs"; jobs: JobView[] }
  | { type: "checkpoints"; checkpoints: CheckpointMeta[] }
  | { type: "hydrate_start"; reason: HydrateReason; surfaceIntent: SessionSurfaceIntent; placeholderItems?: Item[]; placeholderTabId?: string; preserveHistory?: boolean }
  | { type: "hydrate_done" }
  | { type: "hydrate_error"; reason: HydrateReason; error: string }
  | { type: "backend_activation_start" }
  | { type: "backend_activation_done" }
  | { type: "message_action_start"; action: MessageActionState }
  | { type: "message_action_done" }
  | { type: "history"; messages: HistoryMessage[]; before?: number; hasMore?: boolean; generation?: string }
  | { type: "history_prepend"; messages: HistoryMessage[]; before: number; hasMore: boolean; generation: string }
  | { type: "history_older_loading"; loading: boolean }
  | { type: "history_revalidated"; messages: HistoryMessage[]; fingerprint: string; before?: number; hasMore?: boolean; generation?: string }
  | { type: "history_with_runtime_tail"; messages: HistoryMessage[]; fingerprint: string; before?: number; hasMore?: boolean; generation?: string }
  | { type: "local_notice"; level: "info" | "warn"; text: string }
  | { type: "clearApproval" }
  | { type: "clearAsk" }
  | { type: "prompt_reply_started"; promptId: string; input: PromptReplyInput }
  | { type: "prompt_reply_receipt"; receipt: import("../../lib/types").PromptReplyReceipt }
  | { type: "prompt_reply_failed"; promptId: string; occurrenceId?: string }
  | { type: "reset"; surfaceIntent?: SessionSurfaceIntent }
  | { type: "hydrate_reset" };

export const initialState: State = {
  transcriptThrough: 0,
  transcriptSeen: {},
  items: [],
  running: false,
  turnActive: false,
  pendingPrompt: false,
  backgroundJobs: 0,
  cancelRequested: false,
  cancellable: false,
  prompts: initialPromptProjection(),
  operational: initialOperationalProjection(),
  context: { used: 0, window: 0, sessionTokens: 0 },
  composerSelection: { phase: "pending", seq: 0, modelRef: "" },
  jobs: [],
  checkpoints: [],
  hydrating: false,
  surfaceIntent: "unknown",
  backendActivationPending: false,
  historyRevalidationSeq: 0,
  historyBefore: 0,
  historyGeneration: "",
  historyHasMore: false,
  historyLoadingOlder: false,
  childItemsByRef: {},
  subagentStatusByRef: {},
  currentChildAssistantByRef: {},
  parallelItemsByCall: {},
  rejectedRootCandidateKeys: {},
  turnStartAt: 0,
  turnTokens: 0,
  turnTotalTokens: 0,
  turnCost: 0,
  sessionTokens: 0,
  sessionCost: 0,
  sessionCurrency: "¥",
  taskTrees: initialTaskTreeState,
  seq: 0,
  sessionGen: 0,
};
