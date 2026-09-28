// Heartbeat task types — mirrors desktop/heartbeat.go.

export interface HeartbeatTask {
  id: string;
	revision?: number;
  title: string;
  prompt: string;
  interval: string;   // e.g. "5m", "1h", "30s"
  enabled: boolean;
  scope?: string;      // "global" or "project"
	projectId?: string;   // logical Cloud Project owner
  workspaceRoot?: string;
  topicId?: string;
  lastRunAt?: number;  // unix millis
  lastRunId?: string;
  nextRunAt?: string;  // RFC3339, computed by the backend
  newConversationEachRun?: boolean; // true = create new topic each run
  createdAt?: number;
  approvalMode?: "ask" | "auto" | "yolo"; // empty defaults to "auto"
  timeWindowStart?: string; // "HH:MM" — interval tasks only run after this time
  timeWindowEnd?: string;   // "HH:MM" — interval tasks only run before this time
	intervalWeekdays?: string[];
	timezone?: string;
  model?: string;
  billingSource?: "subscription_pool" | "newapi_byok";
  skills?: string[];
  role?: string;
	expertId?: string;
	connectorIds?: string[];
	effectiveFrom?: string;
	effectiveUntil?: string;
	missedRunPolicy?: "skip" | "latest_once" | "catch_up";
	deliveryEnabled?: boolean;
	deliveryTargets?: AutomationDeliveryTarget[];
  conditionPrompt?: string;
  failureCooldown?: string;
  businessDiagnostics?: BusinessDiagnosticsConfig;
}

export interface AutomationTaskTemplate {
  id: string;
  title: string;
  description: string;
  prompt: string;
  interval?: string;
  icon?: string;
  kind?: "task" | "business_diagnostics";
  source?: "builtin" | "user";
}

export interface AutomationTask {
	schemaVersion?: number;
	revision?: number;
  id: string;
  name: string;
  enabled: boolean;
  scope?: string;
	projectId?: string;
  workspaceRoot?: string;
  topicId?: string;
  newConversationEachRun?: boolean;
  prompt: AutomationPrompt;
  schedule: AutomationSchedule;
  conditions?: AutomationConditions;
  execution?: AutomationExecution;
  archive?: AutomationArchive;
	capabilities?: AutomationCapabilitySelection;
	effectiveRange?: AutomationEffectiveRange;
	missedRunPolicy?: "skip" | "latest_once" | "catch_up";
	delivery?: AutomationDeliveryPolicy;
  createdAt?: string;
  updatedAt?: string;
  lastRunAt?: string;
  lastRunId?: string;
  nextRunAt?: string;
  businessDiagnostics?: BusinessDiagnosticsConfig;
}

export interface AutomationCapabilitySelection {
	modelRef?: string;
	skillIds?: string[];
	expertId?: string;
	connectorIds?: string[];
}

export interface AutomationCapabilityArtifact {
	id: string;
	version?: string;
	contentHash?: string;
}

export interface AutomationConnectorGrantSnapshot {
	connectorId: string;
	serverId: string;
	toolAllowlist?: string[];
	grantId: string;
	principal: string;
	source: string;
	expiresAt?: string;
}

export interface AutomationCapabilitySnapshot extends AutomationCapabilitySelection {
	approvalMode?: "ask" | "auto" | "yolo";
	toolAllowlist?: string[];
	promptHash?: string;
	skills?: AutomationCapabilityArtifact[];
	expert?: AutomationCapabilityArtifact;
	connectorGrants?: AutomationConnectorGrantSnapshot[];
	principal?: string;
	riskSummary?: string;
	capturedAt?: string;
}

export interface AutomationEffectiveRange {
	from?: string;
	until?: string;
}

export interface AutomationDeliveryTarget {
	type: string;
	ref: string;
}

export interface AutomationDeliveryTargetCatalogEntry extends AutomationDeliveryTarget {
	label: string;
	provider: string;
	available: boolean;
	unavailableReason?: string;
	scope: string;
}

export interface AutomationDeliveryPolicy {
	enabled: boolean;
	targets?: AutomationDeliveryTarget[];
}

export interface AutomationDeliveryAttempt {
	id: string;
	runId: string;
	status: "queued" | "sending" | "succeeded" | "failed" | "canceled";
	retryable?: boolean;
	target: AutomationDeliveryTarget;
	failure?: { class?: string; code?: string; message?: string };
}

export interface AutomationTaskQuota {
  state: "loading" | "ready";
  plan?: "anonymous" | "free" | "lite" | "plus" | "pro";
  used: number;
  limit: number;
  unlimited: boolean;
  canCreate: boolean;
  pausedTaskIds: string[];
}

export type BusinessDataKind = "sales" | "inventory" | "purchase" | "finance" | "returns";

export type BusinessSourceType = "csv" | "xlsx" | "http" | "mcp";

export interface BusinessDataSource {
  id: string;
  kind: BusinessDataKind;
  type: BusinessSourceType;
  path?: string;
  url?: string;
  format?: string;
  rows?: Record<string, unknown>[];
}

export interface BusinessDiagnosticsConfig {
  sources: BusinessDataSource[];
  targetStart: string;
  targetEnd: string;
  region?: string;
  recipients?: string[];
}

export interface AutomationPrompt {
  model?: string;
  billingSource?: "subscription_pool" | "newapi_byok";
  skills?: string[];
  role?: string;
  prompt: string;
  conditionModel?: string;
}

export interface AutomationSchedule {
  kind: "once" | "interval" | "calendar";
  runAt?: string;
  every?: string;
	weekdays?: string[];
  calendar?: AutomationCalendarSchedule;
  timezone?: string;
  window?: AutomationTimeWindow;
}

export interface AutomationScheduleCatalogEntry {
	id: string;
	kind: AutomationSchedule["kind"];
	frequency?: string;
	requiresRunAt?: boolean;
	requiresEvery?: boolean;
	requiresWeekdays?: boolean;
	requiresMonthDays?: boolean;
	requiresMonths?: boolean;
}

export interface AutomationCalendarSchedule {
  frequency?: string;
  timeOfDay?: string;
  weekdays?: string[];
  monthDays?: number[];
  months?: number[];
}

export interface AutomationTimeWindow {
  start?: string;
  end?: string;
}

export interface AutomationConditions {
  conditionPrompt?: string;
  failureCooldown?: string;
}

export interface AutomationExecution {
  approvalMode?: "ask" | "auto" | "yolo";
  approvalTimeout?: string;
  maxConcurrentRuns?: number;
}

export interface AutomationArchive {
  visible?: boolean;
  retention?: string;
  keepSession?: boolean;
  keepEvents?: boolean;
}

export type AutomationRunStatus =
  | "queued"
  | "checking_condition"
  | "skipped"
  | "running"
  | "succeeded"
  | "failed"
  | "blocked"
  | "canceled";

export interface AutomationRunSummary {
  runId: string;
  taskId: string;
  taskName?: string;
  status: AutomationRunStatus;
  trigger?: "manual" | "schedule";
  triggerReason?: string;
  skipReason?: string;
  conditionReason?: string;
  owner?: string;
  scope?: "global" | "project" | string;
	projectId?: string;
  model?: string;
  billingSource?: "subscription_pool" | "newapi_byok";
  workspaceRoot?: string;
  startedAt?: string;
  finishedAt?: string;
  durationMs?: number;
  resultPath?: string;
  summary?: string;
  errorCode?: string;
  errorMessage?: string;
  error?: string;
}

export interface AutomationRunFilter {
	taskId?: string;
	scope?: string;
	limit?: number;
}

export interface AutomationRunPage {
	items: AutomationRunSummary[];
	nextCursor?: string;
	hasMore: boolean;
}

export interface AutomationRunManifest extends AutomationRunSummary {
	taskRevision?: number;
	capabilities?: AutomationCapabilitySnapshot;
	delivery?: AutomationDeliveryPolicy;
	schedule?: AutomationSchedule;
	effectiveRange?: AutomationEffectiveRange;
  hostOwner?: string;
  leaseId?: string;
  sessionPath?: string;
  eventsPath?: string;
  usage?: Record<string, unknown>;
  condition?: {
    shouldRun?: boolean;
    reason?: string;
    error?: string;
    checkedAt?: string;
  };
}

export interface AutomationRunEvent {
  time?: string;
  kind?: string | number;
  level?: string | number;
  text?: string;
  status?: AutomationRunStatus;
  error?: string;
}

export interface AutomationRunDetail {
  manifest?: AutomationRunManifest;
	deliveries?: AutomationDeliveryAttempt[];
  resultSummary?: string;
  resultTruncated?: boolean;
  availableResultFormats?: AutomationRunResultExportOption[];
  events?: AutomationRunEvent[];
}

export type AutomationRunResultExportFormat = "md" | "txt" | "json" | "docx" | "xlsx" | "pptx";

export interface AutomationRunResultExportOption {
  format: AutomationRunResultExportFormat;
  label: string;
  extension?: string;
  mimeType?: string;
}

export interface AutomationRunResultExport {
  filename: string;
  mimeType: string;
  payload: string;
  base64Encoded: boolean;
}

export interface AutomationCancelRunResponse {
  taskId: string;
  runId: string;
  canceled: boolean;
  status: string;
  reason?: string;
}
