// Wire contract — mirrors desktop/wire.go (itself mirroring internal/serve/wire.go).
// One event channel carries every kind; `kind` discriminates the payload.

export type EventKind =
  | "turn_started"
  | "reasoning"
  | "text"
  | "message"
  | "tool_dispatch"
  | "tool_result"
  | "tool_progress"
  | "usage"
  | "notice"
  | "phase"
  | "approval_request"
  | "ask_request"
  | "operational"
  | "turn_done"
  | "compaction_started"
  | "compaction_done"
  | "mcp_surface_ready"
  | "retrying"
  | "steer"
  | "guardian_assessment"
  | "audit"
  | "task_tree_created"
  | "task_node_updated"
  | "task_node_started"
  | "task_node_output"
  | "task_node_completed"
  | "task_node_failed"
  | "task_tree_completed"
  | "task_tree_summary"
  | "subagent_status_changed"
  | "final_candidate_rejected";

export interface WireCompaction {
  trigger?: string; // "auto" | "manual"
  messages?: number; // done: how many messages were folded into the summary
  summary?: string; // done: the briefing (empty on an aborted pass)
  archive?: string; // done: archive path, if any
}

export interface WireProfile {
  model?: string;
  effort?: string;
}

export interface WireTool {
  id?: string;
  name: string;
  args?: string;
  output?: string;
  err?: string;
  artifactPaths?: string[];
  readOnly: boolean;
  truncated?: boolean;
  durationMs?: number;
  partial?: boolean; // an early dispatch (name only) — a full one with args follows
  parentId?: string; // set on a sub-agent's calls — the parent `task` call's id
  subagentRef?: string; // persisted child session that owns the event
  sourceToolId?: string; // raw child tool id before parent transport namespacing
  taskTreeId?: string; // set when this tool activity belongs to a task-tree node
  nodeId?: string; // task-tree node that owns this tool activity
  diff?: string;
  added?: number;
  removed?: number;
  profile?: WireProfile; // subagent model/effort resolved for this call
}

export interface WireCacheDiagnostics {
  prefixHash: string;
  prefixChanged: boolean;
  prefixChangeReasons?: string[];
  systemHash: string;
  toolsHash: string;
  logRewriteVersion: number;
  toolSchemaTokens: number;
  cacheMissTokens: number;
  cacheHitTokens: number;
}

export interface WireUsage {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  cacheHitTokens: number;
  cacheMissTokens: number;
  reasoningTokens?: number;
  source?: string;
  cacheDiagnostics?: WireCacheDiagnostics;
  // Session-cumulative cache tokens — the status bar shows the aggregate
  // hit-rate (Σhit/Σ(hit+miss)), steadier than the single-turn cacheHitTokens.
  sessionCacheHitTokens: number;
  sessionCacheMissTokens: number;
  cost?: number;
  currency?: string;
  // Deprecated compatibility alias. Prefer cost + currency.
  costUsd?: number;
}

export interface WireApproval {
  id: string;
  tool: string;
  subject: string;
  reason?: string;
  audit?: WireAudit;
}

export interface WireAudit {
  actor_id?: string;
  profile_id?: string;
  session_id?: string;
  workspace_root?: string;
  tool?: string;
  decision?: string;
  session_grant?: boolean;
  persistent?: boolean;
  persisted_rule?: string;
  memory_mutation?: boolean;
}

export interface WireGuardian {
  id: string;
  tool: string;
  subject: string;
  outcome: string;
  risk_level?: string;
  user_authorization?: string;
  rationale?: string;
  duration_ms?: number;
  usage?: WireUsage;
}

export interface WireTaskTreeNode {
  nodeId: string;
  status?: string;
  title?: string;
  parentId?: string;
  subagentRef?: string;
  dependsOn?: string[];
  goal?: string;
  expertId?: string;
  expertName?: string;
  expertRole?: string;
  contextExpectation?: string;
  outputExpectation?: string;
  completionPercent?: number;
  confidencePercent?: number;
}

export interface WireTaskTree {
  treeId: string;
  nodeId?: string;
  status?: string;
  title?: string;
  parentId?: string;
  dependsOn?: string[];
  result?: string;
  error?: string;
  outputDelta?: string;
  summary?: string;
  rootIds?: string[];
  nodes?: WireTaskTreeNode[];
  goal?: string;
  expertId?: string;
  expertName?: string;
  expertRole?: string;
  contextExpectation?: string;
  outputExpectation?: string;
  completionPercent?: number;
  confidencePercent?: number;
}

export interface WireAskOption {
  label: string;
  description?: string;
}

export interface WireAskQuestion {
  id: string;
  header?: string;
  prompt: string;
  options: WireAskOption[];
  multi?: boolean;
}

export interface WireAsk {
  id: string;
  questions: WireAskQuestion[];
  audit?: WireAudit;
}

// QuestionAnswer is the reply for one question, sent back via AnswerQuestion.
export interface QuestionAnswer {
  questionId: string;
  selected: string[];
}

export type PromptLifecycle = "registered" | "pending" | "replying" | "resolved" | "reply_failed" | "cancelled";

export type PromptReplyStatus = "accepted" | "already_resolved" | "prompt_not_found" | "wrong_tab" | "cancelled" | "failed";

export interface PromptReplyReceipt {
  promptId: string;
  status: PromptReplyStatus;
  occurrenceId?: string;
  retryable?: boolean;
}

export interface WireOperationalPrompt {
  kind: "approval" | "ask";
  lifecycle: PromptLifecycle;
  approval?: WireApproval;
  ask?: WireAsk;
}

export type OperationalAggregateKind = "prompt" | "task" | "model_call" | "tool_call" | "session" | "dashboard_write" | "plan";

export interface WireOperationalLifecycle {
  state: string;
  previousState?: string;
  role?: string;
}

export interface WireOperationalEnvelope {
  schemaVersion: 1;
  eventId: string;
  operationId: string;
  occurredAt?: string;
  causationId?: string;
  occurrenceId?: string;
  aggregate: { kind: OperationalAggregateKind; id: string; sequence: number };
  scope: { tabId?: string; sessionId?: string; turnId?: string; candidateId?: string; promptId?: string; taskId?: string; callId?: string; toolCallId?: string; planId?: string; dashboardWriteId?: string };
  kind: string;
  stage?: string;
  outcome?: string;
  severity?: string;
  userImpact?: string;
  retryable?: boolean;
  privacyClass?: string;
  prompt?: WireOperationalPrompt;
  task?: WireOperationalLifecycle;
  modelCall?: WireOperationalLifecycle;
  toolCall?: WireOperationalLifecycle;
  session?: WireOperationalLifecycle;
  dashboardWrite?: WireOperationalLifecycle;
  plan?: WireOperationalLifecycle;
}

export interface MemoryCitation {
  id?: string;
  source: string;
  lineStart?: number;
  lineEnd?: number;
  note?: string;
  kind?: string;
}

export interface WireEvent {
  kind: EventKind;
  transcriptJournalId?: string;
  transcriptSequence?: number;
  transcriptUser?: HistoryMessage;
  // Content-free delivery identity assigned before the Wails bridge. These
  // fields scope idempotency to one tab/session turn and model candidate.
  eventId?: string;
  turnId?: string;
  candidateId?: string;
  eventSequence?: number;
  text?: string;
  reasoning?: string;
  parentId?: string;
  subagentRef?: string;
  subagentStatus?: string;
  memoryCitations?: MemoryCitation[];
  artifactDelivery?: ArtifactDelivery;
  turn?: TurnMetadata;
  level?: "info" | "warn";
  tool?: WireTool;
  usage?: WireUsage;
  approval?: WireApproval;
  ask?: WireAsk;
  operational?: WireOperationalEnvelope;
  audit?: WireAudit;
  compaction?: WireCompaction;
  guardian?: WireGuardian;
  taskTree?: WireTaskTree;
  err?: string;
  interrupted?: boolean;
  retryAttempt?: number;
  retryMax?: number;
  // Tab routing: set by the Go-side tabEventSink so multi-tab frontends
  // route each event to the correct per-tab reducer.
  tabId?: string;
  // Stable conversation routing remains available after a running runtime is
  // detached from the visible tab list.
  conversationScope?: string;
  conversationWorkspaceRoot?: string;
  conversationTopicId?: string;
  conversationSessionPath?: string;
  activeTurnStartedAt?: number;
  activeTurnCompletionTokens?: number;
  sessionHitTokens?: number;
  sessionMissTokens?: number;
  sessionCost?: number;
  sessionCurrency?: string;
  // Deprecated compatibility alias. Prefer sessionCost + sessionCurrency.
  sessionCostUsd?: number;
}

export interface TurnMetadata {
  taskId: string;
  modelRef: string;
  modelId?: string;
  billingSource: string;
  chargedCredits?: string;
  billingStatus?: "pending" | "settled" | "unavailable" | "error";
  billingScopeKey?: string;
}

export interface TurnBillingQuery {
  sessionPath: string;
  profileId: string;
  cloudUserId: string;
  bindingGeneration: number;
  taskIds: string[];
}

export interface TurnBillingResult {
  taskId: string;
  status: "pending" | "settled" | "unavailable" | "error";
  chargedCredits?: string;
}

export interface TurnBillingSettled {
  taskId: string;
  chargedCredits: string;
}

// Tab management types (desktop/tabs.go).
export type TabSendAvailabilityState =
  | "starting"
  | "ready"
  | "subscription_auth_required"
  | "subscription_account_action_required"
  | "subscription_balance_depleted"
  | "byok_configuration_required"
  | "service_unavailable"
  | "startup_failed";

export interface TabSendAvailability {
  state: TabSendAvailabilityState;
  retryable: boolean;
  detail?: string;
  action?: "login" | "account" | "configure_byok" | "retry" | "";
}

export type Scene = "office" | "coding" | "design";
export type SceneState = "draft" | "locked" | "legacy";

export interface TabMeta {
  id: string;
  tabType?: "session" | "file";
  scope: string;
  workspaceRoot: string;
  workspaceName: string;
  workspacePath?: string;
  gitBranch?: string;
  topicId: string;
  topicTitle: string;
  sessionPath?: string;
  sessionTurns?: number;
  scene?: Scene;
  sceneState?: SceneState;
  readOnly?: boolean;
  filePath?: string;
  projectColor?: string;
  label: string;
  readonly modelRef?: string;
  billingSource?: "subscription_pool" | "newapi_byok" | "";
  effort?: string;
  expertId?: string;
  expertName?: string;
  ready: boolean;
  running: boolean;
  pendingPrompt?: boolean;
  backgroundJobs?: number;
  cancelRequested?: boolean;
  cancellable?: boolean;
  activeTurnStartedAt?: number;
  activeTurnCompletionTokens?: number;
  mode: Mode;
  collaborationMode?: CollaborationMode;
  toolApprovalMode?: ToolApprovalMode;
  tokenMode?: TokenMode;
  goal?: string;
  goalStatus?: GoalStatus;
  startupErr?: string;
  sendAvailability?: TabSendAvailability;
  active: boolean;
  cwd: string;
}

export interface ProjectNode {
  key: string;
  kind: "project" | "cloud_project" | "topic" | "session" | "global_folder" | "global_topic" | "global_session";
  label: string;
  root?: string;
  cloudProjectId?: string;
  topicId?: string;
  sessionPath?: string;
  sessionTurns?: number;
  scene?: Scene;
  sceneState?: SceneState;
  modelRef?: string;
  billingSource?: "subscription_pool" | "newapi_byok" | "";
  effort?: string;
  expertId?: string;
  expertName?: string;
  collaborationMode?: CollaborationMode;
  toolApprovalMode?: ToolApprovalMode;
  tokenMode?: TokenMode;
  goal?: string;
  goalStatus?: GoalStatus;
  projectColor?: string;
  turns?: number;
  createdAt?: number;
  lastActivityAt?: number;
  open?: boolean;
  running?: boolean;
  status?: ProjectTopicStatus;
  pinned?: boolean;
  capability?: CapabilityTaskScope;
  children?: ProjectNode[];
}

export interface CapabilityTaskScope {
  capabilityId: string;
  workspaceId: string;
  taskId: string;
  documentId: string;
}

export type ProjectTopicStatus = "thinking" | "streaming" | "waiting_confirmation" | "background_job" | "paused" | "error";

export interface TopicMeta {
  id: string;
  title: string;
  createdAt: number;
}

export interface ContextPanelInfo {
  usedTokens: number;
  windowTokens: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  reasoningTokens: number;
  cacheHitTokens: number;
  cacheMissTokens: number;
  requestCount?: number;
  elapsedMs?: number;
  sessionCost?: number;
  sessionCurrency?: string;
  // Deprecated compatibility alias. Prefer sessionCost + sessionCurrency.
  sessionCostUsd?: number;
  sources?: Record<string, UsageSourceStats>;
  mock?: boolean;
  readFiles: ReadFileRecord[];
  changedFiles: ChangedFileInfo[];
}

export interface UsageSourceStats {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  reasoningTokens: number;
  cacheHitTokens: number;
  cacheMissTokens: number;
  requestCount: number;
  sessionCost?: number;
  sessionCurrency?: string;
  sessionCostUsd?: number;
}

export interface ReadFileRecord {
  path: string;
  turn: number;
  time: number;
  offset?: number;
  limit?: number;
  truncated?: boolean;
}

export interface ChangedFileInfo {
  path: string;
  oldPath?: string;
  sources: string[];
  gitStatus?: string;
  turns: number[];
  latestPrompt?: string;
  latestTime?: number;
}

// Bound-method payloads (desktop/app.go).
export interface HistoryMessage {
  role: string;
  content: string;
  finalAnswer?: boolean;
  submitText?: string;
  createdAt?: number;
  reasoning?: string;
  memoryCitations?: MemoryCitation[];
  artifactDelivery?: ArtifactDelivery;
  level?: "info" | "warn";
  toolCalls?: HistoryToolCall[];
  toolCallId?: string;
  toolName?: string;
  toolResultArchived?: boolean;
  toolResultError?: string;
  pending?: boolean;
  trigger?: string;
  messages?: number;
  summary?: string;
  archive?: string;
  turn?: TurnMetadata;
}

export interface HistoryToolCall {
  id: string;
  name: string;
  arguments: string;
  subject?: string;
  summary?: string;
  diff?: string;
  added?: number;
  removed?: number;
  argumentsArchived?: boolean;
  artifactPaths?: string[];
  subagentRef?: string;
  taskLifecycle?: TaskLifecycle;
  taskLifecycleRefs?: string[];
  taskLifecycleReceipts?: TaskLifecycle[];
  subagentMessages?: HistoryMessage[];
}

export interface TaskLifecycleArtifact {
  path: string;
  size: number;
  source?: "writer" | "snapshot";
}

export interface TaskLifecycleValidation {
  kind: string;
  success: boolean;
  path?: string;
  status?: string;
}

export interface TaskLifecycle {
  subagentRef: string;
  taskRunId?: string;
  dependsOn?: string[];
  memberName?: string;
  turnStatus: "running" | "completed" | "failed" | "interrupted" | string;
  finalAnswer?: string;
  artifacts?: TaskLifecycleArtifact[];
  validations?: TaskLifecycleValidation[];
  error?: string;
}

// ArtifactDelivery is the versioned host-committed deliverable set for one
// final assistant message. A supported contract, including an empty paths
// list, is authoritative over legacy text and task-receipt inference.
export interface ArtifactDelivery {
  version: number;
  paths: string[];
  deliveredAt?: number;
}

export interface HistoryPage {
  messages: HistoryMessage[];
  before: number;
  hasMore: boolean;
  generation: string;
}

export interface TranscriptPage {
  journalId: string;
  baseline?: HistoryMessage[];
  through: number;
  next: number;
  hasMore: boolean;
  entries: { sequence: number; payload: WireEvent }[];
}

export interface TranscriptReplay {
  journalId: string;
  // Present when this replay only fills events after an installed snapshot.
  after?: number;
  through: number;
  messages: HistoryMessage[];
  events: WireEvent[];
}

export interface PromptHistoryEntry {
  text: string;
  at: number;          // unix ms
  sessionPath: string;
  turn: number;
}

export interface PromptHistoryResult {
  entries: PromptHistoryEntry[] | null;
  nonce: string;
  olderCursor?: string;
  hasOlder?: boolean;
}

// CheckpointMeta is one rewind point (a user turn) for the rewind UI.
export interface CheckpointMeta {
  turn: number;
  prompt: string;
  files: string[];
  turnFileCount?: number;
  time: number; // unix ms
  canCode?: boolean;
  canConversation?: boolean;
}

// SessionMeta is one saved session for the history panel.
export interface SessionMeta {
  path: string;
  preview: string;
  title?: string; // user-chosen name; falls back to preview when empty
  turns: number;
  createdAt: number; // unix milliseconds
  lastActivityAt: number; // unix milliseconds
  modTime: number; // compatibility alias for lastActivityAt
  deletedAt?: number; // unix milliseconds, present for trashed sessions
  current: boolean;
  open: boolean;
  scope?: string;       // "project" | "global"; empty for legacy → treated as "global"
  workspaceRoot?: string;
  expertId?: string;
  expertName?: string;
  modelRef?: string;
  billingSource?: "subscription_pool" | "newapi_byok" | "";
  effort?: string;
  collaborationMode?: CollaborationMode;
  toolApprovalMode?: ToolApprovalMode;
  tokenMode?: TokenMode;
  goal?: string;
  goalStatus?: GoalStatus;
  topicId?: string;
  topicTitle?: string;
  kind?: "session" | "channel" | string;
  channel?: string;
  channelLabel?: string;
  remoteId?: string;
  chatType?: string;
  userId?: string;
  threadId?: string;
  sessionSource?: string;
}

// ArtifactView is a delivered workspace file together with the complete
// session metadata needed by the existing session-resume flow.
export interface ArtifactView {
  path: string;
  name: string;
  extension: string;
  type: string;
  size: number;
  modifiedAt: number;
  deliveredAt: number;
  sessionTitle?: string;
  workspaceName: string;
  session: SessionMeta;
}

export interface ComposerSessionIdentity {
  modelRef: string;
  billingSource: "subscription_pool" | "newapi_byok" | "";
  effort?: string;
  expertKnown: boolean;
  expertId: string;
  expertName: string;
  collaborationMode: CollaborationMode;
  toolApprovalMode: ToolApprovalMode;
  tokenMode: TokenMode;
  goal: string;
  goalStatus: GoalStatus;
}

// SessionReference is a session selected via @ past:chats for context injection.
export interface SessionReference {
  path: string;
  title: string;
  preview?: string;
  turns?: number;
  createdAt?: number;
  lastActivityAt?: number;
}

export interface ExpertInfo {
  id: string;
  source?: "official" | "user" | string;
  mode?: "expert" | "team" | string;
  name: string;
  role?: string;
  description: string;
  category?: string;
  categories?: string[];
  tags?: string[];
  iconKey?: string;
  tone?: string;
  avatarKey?: string;
  avatarUrl?: string;
  members?: ExpertMemberInfo[];
  prompts?: string[];
  language?: string;
  enabled: boolean;
  createdAt?: number;
  updatedAt?: number;
}

// The active controller exposes only the fields needed to select an expert.
// Editable user metadata is available from ListUserExperts instead.
export interface SessionExpertInfo {
  id: string;
  name: string;
  description: string;
  category?: string;
  language?: string;
  enabled: boolean;
}

export interface ExpertMemberInfo {
  kind?: string;
  expertId?: string;
  name: string;
  role: string;
  task: string;
  output: string;
  tone?: string;
  avatarKey?: string;
  avatarUrl?: string;
}

export interface UserExpertInput {
  name: string;
  mode?: "expert" | "team" | string;
  description: string;
  category?: string;
  tags?: string[];
  avatarKey?: string;
  avatarFileName?: string;
  members?: ExpertMemberInfo[];
  systemPrompt: string;
  prompts?: string[];
  displayPrompt?: string;
  language?: string;
  enabled?: boolean;
}

export interface UserExpertInfo extends UserExpertInput {
  id: string;
  source: "user";
  mode: "expert" | "team" | string;
  enabled: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface ExpertLibraryState {
  favoriteIds: string[];
  recentIds: string[];
  usageCounts?: Record<string, number>;
}

export interface ExpertResetResult {
  experts: ExpertInfo[];
  userExperts: UserExpertInfo[];
  libraryState: ExpertLibraryState;
}

export interface ExpertState {
  selectedId: string;
  selectedName: string;
  source: "session" | "request" | "config" | "none" | string;
  canSwitch: boolean;
  requiresNewSession: boolean;
}

export interface ExpertTeamSessionMember {
  memberKey: string;
  expertId?: string;
  name: string;
  role?: string;
  avatarKey?: string;
  subagentRef?: string;
  status?: string;
}

export interface ExpertTeamSession {
  state: "team_active" | "team_archived" | string;
  id: string;
  name: string;
  lead: ExpertTeamSessionMember;
  members: ExpertTeamSessionMember[];
}

export interface WorkspaceView {
  path: string;
  name: string;
  current: boolean;
}

export interface ModelMarketplaceModel {
  id: string;
  name: string;
  icon?: string;
  vendor?: string;
  vendorIcon?: string;
  summary?: string;
  tags?: string[];
  groups?: string[];
  endpoints?: string[];
  billingType?: string;
  quotaType?: number;
  prices?: ModelMarketplacePriceFact[];
  pricingTiers?: ModelMarketplacePricingTier[];
}

export interface ModelMarketplacePriceFact {
  key: string;
  value: string;
}

export interface ModelMarketplacePricingTier {
  maxTokens?: string;
  prices?: ModelMarketplacePriceFact[];
}

export interface ModelMarketplaceSource {
  id: string;
  label: string;
  sourceUrl: string;
  pricingUrl: string;
  pricingVersion?: string;
  pricingState: string;
  pricingAuthenticity?: string;
  statusAuthenticity?: string;
  fetchedAt?: string;
  statusFetchedAt?: string;
  exchangeRate?: string;
  rechargePrice?: string;
  metadataStatus: string;
  partial?: boolean;
  models: ModelMarketplaceModel[];
  errorCode?: string;
}

export interface ModelMarketplaceView {
  sources: ModelMarketplaceSource[];
}

export type SharedModelCatalogState =
  | { state: "loading" }
  | {
      state: "ready";
      routes: ModelInfo[];
      marketplace?: ModelMarketplaceView;
    }
  | { state: "unavailable" };

export interface ContextInfo {
  used: number;
  window: number;
  sessionTokens: number;
  compactRatio?: number;
  activeTurnStartedAt?: number;
  activeTurnCompletionTokens?: number;
}

export interface Meta {
  label: string;
  readonly modelRef?: string;
  ready: boolean;
  startupErr?: string;
  sendAvailability?: TabSendAvailability;
  eventChannel: string;
  cwd: string;
  workspaceRoot?: string;
  workspaceName?: string;
  workspacePath?: string;
  sessionPath?: string;
  gitBranch?: string;
  autoApproveTools?: boolean;
  bypass?: boolean; // legacy JSON key for YOLO/full-access tool auto-approval
  collaborationMode?: CollaborationMode;
  toolApprovalMode?: ToolApprovalMode;
  tokenMode?: TokenMode;
  goal?: string;
  goalStatus?: GoalStatus;
}

export type CollaborationMode = "normal" | "plan" | "goal";
export type ToolApprovalMode = "ask" | "auto" | "yolo";
export type TokenMode = "full" | "economy";
export type GoalStatus = "running" | "complete" | "blocked" | "stopped";

export function normalizeCollaborationMode(mode?: string, goal?: string, legacyMode?: Mode): CollaborationMode {
  if (mode === "plan" || mode === "goal" || mode === "normal") return mode;
  if (legacyMode && modeHasPlan(legacyMode)) return "plan";
  if ((goal ?? "").trim()) return "goal";
  return "normal";
}

export function normalizeToolApprovalMode(
  mode?: string,
  legacyMode?: Mode,
  legacyAutoApproveTools?: boolean,
  fallbackMode?: ToolApprovalMode,
): ToolApprovalMode {
  const normalized = typeof mode === "string" ? mode.trim().toLowerCase() : "";
  if (normalized === "auto" || normalized === "yolo" || normalized === "ask") return normalized as ToolApprovalMode;
  if (legacyAutoApproveTools || (legacyMode && modeHasAutoApproveTools(legacyMode))) return "yolo";
  if (fallbackMode === "auto" && normalized === "") return "auto";
  return "ask";
}

export function normalizeTokenMode(mode?: string): TokenMode {
  if (mode === "economy") return "economy";
  return "full";
}

// Mode is the compatibility string for two independent composer axes:
// plan (read-only/user-plan gate) and yolo/full access (tool auto-approval).
export type Mode = "normal" | "plan" | "yolo" | "plan-yolo";

export function normalizeMode(mode?: string): Mode {
  if (mode === "plan" || mode === "yolo" || mode === "plan-yolo" || mode === "yolo-plan") {
    return mode === "yolo-plan" ? "plan-yolo" : mode;
  }
  return "normal";
}

export function modeHasPlan(mode: Mode): boolean {
  return mode === "plan" || mode === "plan-yolo";
}

export function modeHasAutoApproveTools(mode: Mode): boolean {
  return mode === "yolo" || mode === "plan-yolo";
}

export function modeFromAxes(plan: boolean, autoApproveTools: boolean): Mode {
  if (plan && autoApproveTools) return "plan-yolo";
  if (plan) return "plan";
  if (autoApproveTools) return "yolo";
  return "normal";
}

export function modeWithPlan(mode: Mode, plan: boolean): Mode {
  return modeFromAxes(plan, modeHasAutoApproveTools(mode));
}

export function modeWithAutoApproveTools(mode: Mode, autoApproveTools: boolean): Mode {
  return modeFromAxes(modeHasPlan(mode), autoApproveTools);
}

export interface CommandInfo {
  name: string; // without the leading slash
  description: string;
  hint?: string;
  kind: "builtin" | "custom" | "mcp" | "skill";
}

export interface DirEntry {
  name: string;
  isDir: boolean;
}

export interface DroppedItem {
  kind: "workspace" | "attachment";
  path: string;
  isDir?: boolean;
  previewUrl?: string;
}

export interface FilePreview {
  path: string;
  body: string;
  size: number;
  truncated: boolean;
  binary: boolean;
  kind?: "image" | "video" | "pdf";
  mime?: string;
  url?: string;
  err?: string;
}

export interface WorkspaceChangeView {
  path: string;
  oldPath?: string;
  sources: string[];
  gitStatus?: string;
  turns?: number[];
  latestPrompt?: string;
  latestTime?: number;
}

export interface WorkspaceChangesView {
  files: WorkspaceChangeView[];
  gitAvailable: boolean;
  gitErr?: string;
  gitBranch?: string;
}

export interface GitCommitView {
  hash: string;
  author: string;
  date: string;
  message: string;
}

export interface GitCommitDetailView {
  diff?: string;
  files?: string[];
}

export interface ComposerInsertRequest {
  id: number;
  text: string;
  mode?: "insert" | "replace";
  kind?: "skill";
  skillName?: string;
  skillLabel?: string;
}

// MCP & Skills drawer (desktop/app.go Capabilities) — the GUI counterpart to
// /mcp + /skill: connected/failed servers and discoverable skills.
export interface ServerView {
  name: string;
  transport: string;
  status: "connected" | "deferred" | "failed" | "initializing" | "disabled";
  startIntent?: "off" | "automatic" | string;
  runtimeState?: "idle" | "connecting" | "ready" | "issue" | string;
  builtIn?: boolean;
  configured?: boolean;
  autoStart: boolean;
  tier?: "background" | "eager" | string;
  command?: string;
  args?: string[];
  url?: string;
  envKeys?: string[];
  headerKeys?: string[];
  tools: number;
  prompts: number;
  resources: number;
  error?: string;
  toolList?: MCPToolView[];
  trustedReadOnlyTools?: string[];
  authStatus?: "none" | "possible" | "required" | string;
  authUrl?: string;
  authConfigured?: boolean;
}
export interface MCPToolView {
  name: string;
  description: string;
  readOnlyHint?: boolean;
}
export interface SkillView {
  name: string;
  displayName?: string;
  description: string;
  scope: string;
  runAs: string;
  enabled: boolean;
  iconUrl?: string;
  uninstallId?: string;
  uninstallable?: boolean;
}
export interface SkillRootSkillView {
  name: string;
  description: string;
  scope: string;
  runAs: string;
}
export interface SkillRootView {
  dir: string;
  scope: string;
  priority: number;
  status: string;
  configured: boolean;
  removable: boolean;
  skills: number;
  skillItems?: SkillRootSkillView[];
  warning?: string;
}
export interface CapabilitiesView {
  servers: ServerView[];
  skills: SkillView[];
  skillRoots: SkillRootView[];
}
export interface SkillsSettingsView {
  skills: SkillView[];
  skillRoots: SkillRootView[];
}
export interface PublicSkillView {
  category: string;
  categories: string[];
  type: "system" | "user";
  claim_state: string;
  claimable: boolean;
  claimed_user_handle?: string | null;
  created_at: string;
  description: string;
  downloads: number;
  homepage: string;
  iconUrl?: string | null;
  installs: number;
  install_count: number;
  labels: string[];
  name: string;
  namespace: string;
  ownerName: string;
  publisher: string;
  score: number;
  slug: string;
  source: string;
  stars: number;
  subCategories: string[];
  tags: string[];
  updated_at: string;
  verified: boolean;
  version: string;
  download_url: string;
  download_filename: string;
  download_size_bytes: number;
  download_sha256: string;
}
export interface PublicSkillsPageView {
  items: PublicSkillView[];
  next_cursor: string;
  has_more: boolean;
}
export interface PublicSkillDetailView {
  skill: PublicSkillView;
}
export interface PublicSkillInstallResult {
  name: string;
  namespace: string;
  slug: string;
  version: string;
  path: string;
  reload_required: boolean;
}
export interface MCPServerInput {
  name: string;
  catalogId?: string;
  authMode?: "none" | "credentials" | "oauth";
  transport: string; // stdio | http | sse
  command: string;
  args: string[];
  workDir?: string;
  url: string;
  env?: Record<string, string> | null;
  headers?: Record<string, string> | null;
  secretEnv?: Record<string, string> | null;
  secretHeaders?: Record<string, string> | null;
  requiredEnvKeys?: string[];
  requiredHeaderKeys?: string[];
  trustedReadOnlyTools?: string[];
}

export interface MCPConnectionTestResult {
  name: string;
  status: "connected" | "failed";
  phase: string;
  latencyMs: number;
  tools: number;
  resources: number;
  prompts: number;
  error?: string;
  repairAction?: string;
  alreadyConfigured?: boolean;
  sessionAttached?: boolean;
}

export interface ComposerCapabilitySubmit {
  text: string;
  title_text?: string;
  skill_names: string[];
  mcp_server_names: string[];
}

export interface ModelInfo {
  ref: string; // "provider/model" — pass to SetModel
  provider: string;
  model: string;
  billingSource?: "subscription_pool" | "newapi_byok";
  displayName?: string;
  status?: string;
  disabled?: boolean;
  disabledReason?: string;
  current: boolean;
}

export interface EffortInfo {
  supported: boolean;
  current: string; // "auto" | "low" | "medium" | "high" | "xhigh" | "max"
  default: string;
  levels: string[];
}

// Slash sub-command / argument completion (desktop/app.go SlashArgs). Mirrors the
// CLI's arg hints so the composer can suggest e.g. /skill → list/show/new/paths.
export interface SlashArgItem {
  label: string;
  insert: string; // token to place at the current position
  hint: string;
  descend: boolean; // re-open the menu one level deeper after accepting
}
export interface SlashArgsResult {
  items: SlashArgItem[];
  from: number; // byte offset where the current token begins
}

// Memory panel payloads (desktop/app.go MemoryView).
export interface MemoryDoc {
  path: string;
  scope: string; // "user" | "ancestor" | "project" | "local"
  body: string;
}

export interface MemoryFact {
  name: string;
  title?: string;
  description: string;
  type: string; // "user" | "feedback" | "project" | "reference"
  body: string;
}

export interface MemoryArchive extends MemoryFact {
  path: string;
  archivedAt?: string;
}

export interface MemoryScope {
  scope: string; // "user" | "project" | "local"
  path: string;
}

export interface MemorySuggestion {
  id: string;
  name: string;
  title: string;
  description: string;
  type: string;
  body: string;
  reason: string;
  evidence: string[];
}

export interface SkillSuggestion {
  id: string;
  name: string;
  description: string;
  scope: string;
  body: string;
  reason: string;
  evidence: string[];
}

export interface MemorySuggestionsView {
  memories: MemorySuggestion[];
  skills: SkillSuggestion[];
  generatedAt: string;
  available: boolean;
  source: string;
}

export interface MemoryView {
  docs: MemoryDoc[];
  facts: MemoryFact[];
  archives: MemoryArchive[];
  scopes: MemoryScope[];
  storeDir: string;
  storeGlobalDir?: string;
  available: boolean;
}

// SettingsTab is the top-level navigation item in the Settings Centre modal.
export type SettingsTab = "general" | "models" | "providers" | "bots" | "mcp" | "skills" | "memory" | "shortcuts" | "permissions" | "sandbox" | "appearance" | "experimental" | "updates" | "feedback";

export interface FeedbackImageView {
  token: string;
  filename: string;
  content_type: "image/png" | "image/jpeg";
  thumbnail_data_url: string;
  size_bytes: number;
}

export interface FeedbackImageSelectionView {
  images: FeedbackImageView[];
}

export interface FeedbackSubmitRequest {
  description: string;
  attempt_id: string;
  image_tokens: string[];
}

export interface FeedbackOperationErrorView {
  code: string;
  message: string;
  retryable: boolean;
  request_id?: string;
  diagnostic_id?: string;
}

export interface FeedbackSubmitView {
  report_id?: string;
  replayed: boolean;
  operation_error?: FeedbackOperationErrorView | null;
}

// Settings panel payloads (desktop/settings_app.go).
export interface ProviderView {
  name: string;
  builtIn: boolean;
  added: boolean;
  kind: string;
  baseUrl: string;
  models: string[];
  chatModels?: string[]; // chat-capable subset used by conversation/task selectors
  visionModels: string[]; // subset of models that accepts image input
  visionModelsConfigured: boolean; // true when an empty list is an explicit choice
  modelsUrl: string; // optional override for model discovery; empty derives from baseUrl
  default: string;
  apiKeyEnv: string;
  keySet: boolean; // the env var currently resolves to a value
  requiresKey?: boolean; // false for explicit no-auth providers
  configured?: boolean; // selectable: key is set or no key is required
  keySource?: string;
  keySourcePath?: string;
  balanceUrl: string; // optional wallet-balance endpoint; "" disables the readout
  contextWindow: number;
  reasoningProtocol: string; // auto|deepseek|openai|none; empty = auto/model registry
  supportedEfforts: string[]; // custom /effort levels; empty = use built-in Kind/BaseURL default
  defaultEffort: string; // /effort level when user picks "auto" or unset; "" = supportedEfforts[0]
  billingSource?: string; // subscription_pool|newapi_byok
  locked?: boolean; // account-managed provider: no key/config/remove controls
  availability?: string;
}

// BalanceInfo is the wallet-balance readout (desktop/app.go Balance). available
// is false when the provider declares no balanceUrl or a fetch failed; display is
// the formatted amount (e.g. "¥110.00").
export interface BalanceInfo {
  available: boolean;
  display: string;
  err?: string;
}

// JobView is one running background job (desktop/app.go Jobs) for the status bar.
export interface JobView {
  id: string;
  kind: string; // "bash" | "task"
  label: string;
  status: string; // "running"
  startedAt: number; // unix milliseconds
}

export interface PermissionsView {
  mode: string; // "ask" | "allow" | "deny"
  allow: string[];
  ask: string[];
  deny: string[];
}

export interface SandboxView {
  bash: string; // "enforce" | "off"
  network: boolean;
  workspaceRoot: string;
  allowWrite: string[];
  shell: string; // "auto" | "bash" | "powershell" | "pwsh"
}

export interface AgentView {
  temperature: number;
  maxSteps: number;
  plannerMaxSteps: number;
  systemPrompt: string;
  coldResumePrune: boolean;
  reasoningLanguage: string; // "auto" | "zh" | "en"
}

export interface BotAllowlistView {
  enabled: boolean;
  allowAll: boolean;
  qqUsers: string[];
  feishuUsers: string[];
  weixinUsers: string[];
  qqGroups: string[];
  feishuGroups: string[];
  weixinGroups: string[];
}

export interface QQBotView {
  enabled: boolean;
  appId: string;
  appSecretEnv: string;
  secretSet: boolean;
  sandbox: boolean;
}

export interface FeishuBotView {
  enabled: boolean;
  domain: string;
  appId: string;
  appSecretEnv: string;
  secretSet: boolean;
  verificationToken: string;
  mode: string;
  webhookPort: number;
  requireMention: boolean;
}

export interface WeixinBotView {
  enabled: boolean;
  accountId: string;
  tokenEnv: string;
  tokenSet: boolean;
  apiBase: string;
}

export interface BotConnectionCredentialView {
  appId: string;
  appSecretEnv: string;
  accountId: string;
  tokenEnv: string;
  secretSet: boolean;
}

export interface BotConnectionSessionMappingView {
  remoteId: string;
  sessionId: string;
  sessionSource: string;
  chatType: string;
  userId: string;
  threadId: string;
  scope: "global" | "project" | string;
  workspaceRoot: string;
  updatedAt: string;
}

export interface BotConnectionView {
  id: string;
  provider: "qq" | "feishu" | "weixin" | string;
  domain: "qq" | "feishu" | "lark" | "weixin" | string;
  label: string;
  enabled: boolean;
  status: "disconnected" | "pending" | "connected" | "error" | string;
  model: string;
  toolApprovalMode: ToolApprovalMode | "" | string;
  workspaceRoot: string;
  credential: BotConnectionCredentialView;
  sessionMappings: BotConnectionSessionMappingView[];
  lastError: string;
  createdAt: string;
  updatedAt: string;
}

export interface BotSettingsView {
  enabled: boolean;
  model: string;
  toolApprovalMode: ToolApprovalMode | "" | string;
  maxSteps: number;
  debounceMs: number;
  allowlist: BotAllowlistView;
  qq: QQBotView;
  feishu: FeishuBotView;
  weixin: WeixinBotView;
  connections: BotConnectionView[];
}

export interface BotRuntimeStatusView {
  running: boolean;
  status: string;
  message: string;
  connections: number;
  startedAt: string;
}

export interface BotInstallStartResult {
  ok: boolean;
  provider: string;
  domain: string;
  installId: string;
  url: string;
  deviceCode: string;
  userCode: string;
  interval: number;
  expireIn: number;
  message: string;
}

export interface BotInstallPollResult {
  done: boolean;
  connection: BotConnectionView;
  status: string;
  message: string;
  error: string;
}

export interface BotConnectionDiagnostic {
  id: string;
  label: string;
  status: string;
  message: string;
  messageId: string;
  phase: string;
  code: string;
  reportKind: string;
  reportDetail: string;
  occurredAt: string;
}

export interface SettingsView {
  defaultModel: string;
  defaultBillingSource?: string;
  selectedModelSummary?: SelectedModelSummaryView;
  plannerModel: string;
  subagentModel: string;
  subagentEffort: string;
  autoPlan: string;
  providers: ProviderView[];
  officialProviders: ProviderView[];
  permissions: PermissionsView;
  sandbox: SandboxView;
  agent: AgentView;
  bot: BotSettingsView;
  desktopLanguage: string; // "" | "en" | "zh"; empty = auto
  desktopLayoutStyle: string; // "classic" | "workbench" | "creation"
  desktopTheme: string; // "auto" | "dark" | "light"
  desktopThemeStyle: string;
  closeBehavior: string; // "background" | "quit"
  displayMode: string;   // "standard" | "compact"
  statusBarStyle: string; // "icon" | "text"
  statusBarItems: string[]; // ordered visible status bar item ids
  defaultToolApprovalMode: ToolApprovalMode | string; // default for newly-created sessions
  checkUpdates: boolean; // check for new versions on startup
  telemetry: boolean; // anonymous launch ping (install id + version + OS)
  metrics: boolean; // aggregate desktop metrics (anonymous signal/bucket counts)
  configPath: string;
  providerKinds: string[]; // provider implementations the kernel registered (for the kind picker)
  autoApproveTools: boolean;
  bypass: boolean; // legacy JSON key for live YOLO/full-access tool auto-approval
}

export interface SelectedModelSummaryView {
  ref: string;
  provider: string;
  model: string;
  displayName: string;
  billingSource: string;
  availability: "ready" | "unavailable" | "unverified" | string;
  accountState: "ready" | "signed_out" | "not_applicable" | "unknown" | string;
}

export type SettingsCoreView = Omit<SettingsView, "providers" | "officialProviders" | "providerKinds"> & {
  selectedModelSummary: SelectedModelSummaryView;
};

export interface SettingsModelDetailsView {
  providers: ProviderView[];
  officialProviders: ProviderView[];
  providerKinds: string[];
}

export interface DesktopStartupSettingsView {
  bot: BotSettingsView;
  desktopLanguage: string; // "" | "en" | "zh"; empty = auto
  desktopLayoutStyle: string; // "classic" | "workbench"
  desktopTheme: string; // "auto" | "dark" | "light"
  desktopThemeStyle: string;
  displayMode: string;   // "standard" | "compact"
  statusBarStyle: string; // "icon" | "text"
  statusBarItems: string[]; // ordered visible status bar item ids
  checkUpdates: boolean; // check for new versions on startup
}

// Auto-updater payloads (desktop/updater.go). UpdateInfo drives the update banner;
// UpdateProgress streams on the "updater:progress" event during download/install.
export interface UpdateInfo {
  available: boolean;
  current: string;
  latest: string;
  notes: string;
  channel: string;
  canSelfUpdate: boolean; // only a registered Windows installation updates in place
  manualOnly?: boolean;
  manualReason?: string;
  manualKind?: "windows-portable" | "macos" | "linux";
  downloaded: boolean;
  downloadUrl: string; // human-facing releases page (macOS path / fallback link)
  assetSize: number; // running platform's artifact size, for the progress bar
  err?: string; // set when the check itself failed (both endpoints down)
}

export interface UpdateDownloadResult {
  version: string;
  channel: string;
  path: string;
  size: number;
  sha256: string;
}

export interface UpdateProgress {
  phase: "downloading" | "verifying" | "downloaded" | "installing" | "done" | "error";
  received: number;
  total: number;
  err?: string;
}
