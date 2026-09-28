// bridge is the single seam between the React app and the Go kernel. In the Wails
// shell it calls the bound App methods (window.go.main.App.*) and subscribes to
// the runtime event stream (window.runtime.EventsOn). Browser development uses
// the proxied Wails runtime too, so every surface reads the same Go-owned state.

// @ts-ignore `wails generate module` creates this locally; fresh checkouts keep
// typecheck green by falling back to a disabled drift check below.
import type * as GeneratedApp from "../../wailsjs/go/main/App";

import { addBreadcrumb } from "./breadcrumbs";
import type { ProjectComposerDiagnostic } from "./projectComposerDiagnostics";
import { getWailsRuntime, type WailsRuntime } from "../platform/wails/wailsRuntime";

import type {
  CloudBootstrapView,
  CloudCachedAssetsView,
  CloudPaymentOperationView,
  CloudPaymentView,
  CloudMaaSRechargeOrderView,
  CloudMaaSRechargeKeyBalanceView,
} from "../store/account";

import type {
  ArtifactView,
  BalanceInfo,
  BotConnectionDiagnostic,
  BotInstallPollResult,
  BotInstallStartResult,
  BotRuntimeStatusView,
  BotSettingsView,
  CapabilitiesView,
  CheckpointMeta,
  CommandInfo,
  ContextInfo,
  ContextPanelInfo,
  DirEntry,
  DesktopStartupSettingsView,
  DroppedItem,
  EffortInfo,
  ExpertLibraryState,
  ExpertInfo,
  ExpertResetResult,
  ExpertState,
  ExpertTeamSession,
  FilePreview,
  FeedbackImageSelectionView,
  FeedbackSubmitRequest,
  FeedbackSubmitView,
  HistoryMessage,
  HistoryPage,
  TranscriptPage,
  JobView,
  MCPServerInput,
  MCPConnectionTestResult,
  ComposerCapabilitySubmit,
  MemorySuggestion,
  MemorySuggestionsView,
  MemoryView,
  Meta,
  ModelMarketplaceView,
  ModelInfo,
  ProjectNode,
  PublicSkillDetailView,
  PublicSkillInstallResult,
  PublicSkillsPageView,
  PromptHistoryResult,
  PromptReplyReceipt,
  ProviderView,
  QuestionAnswer,
  ServerView,
  SessionExpertInfo,
  SessionMeta,
  SettingsView,
  SettingsCoreView,
  SettingsModelDetailsView,
  SkillsSettingsView,
  SkillSuggestion,
  SlashArgsResult,
  TabMeta,
  TopicMeta,
  TurnBillingSettled,
  TurnBillingQuery,
  TurnBillingResult,
  UpdateDownloadResult,
  UpdateInfo,
  UpdateProgress,
  UserExpertInfo,
  UserExpertInput,
  WireEvent,
  WorkspaceChangesView,
  GitCommitView,
  GitCommitDetailView,
  WorkspaceView,
} from "./types";

import type {
  AcceptCloudProjectInviteInput,
  AcceptCloudProjectInviteResult,
  CloudProjectActivity,
  CloudProjectActivityListInput,
  CloudProjectMessageListInput,
  CloudProjectActivityPage,
  CloudProjectInviteCreateInput,
  CloudProjectInviteCreateResult,
  CloudProjectInviteList,
  CloudProjectInviteMutationInput,
  CloudProjectListInput,
  CloudProjectFeatures,
	CloudProjectAsset,
	CloudProjectAssetDownload,
	CloudProjectAssetList,
	CloudProjectAssetDirectoryInput,
	CloudProjectAssetDirectoryPage,
	CloudProjectAssetOperationRecordListInput,
	CloudProjectAssetOperationRecordPage,
	CreateCloudProjectAssetFolderInput,
	PatchCloudProjectAssetFolderInput,
	PatchCloudProjectAssetInput,
	CloudProjectAssetProcessingStateList,
	CloudProjectAssetProcessingStateListInput,
	DeleteCloudProjectAssetFolderInput,
	ReplaceCloudProjectAssetContentInput,
	MutateCloudProjectWorkItemAssetInput,
	CloudProjectCapabilityList,
	SaveCloudProjectCapabilitiesInput,
	ProjectConnectorAvailability,
  CloudProjectInstruction,
  CloudProjectPage,
  CloudProjectOnboarding,
  CloudProjectMemberList,
  CloudProjectMemberMutationInput,
  JoinCloudProjectInput,
  JoinCloudProjectResult,
  CloudProjectResult,
  CloudProjectSession,
  NewCloudProjectSessionInput,
  CloudProjectWorkItemListInput,
  CloudProjectWorkItemPage,
  CloudProjectWorkItemResult,
	CloudProjectWorkItemDeleteResult,
  CloudProjectLabelList,
  CreateCloudProjectInput,
  CreateCloudProjectWorkItemInput,
  CreateCloudProjectLabelInput,
  PatchCloudProjectLabelInput,
  DeleteCloudProjectLabelInput,
  DeleteCloudProjectInput,
  EditCloudProjectInstructionInput,
  PatchCloudProjectInput,
  PatchCloudProjectWorkItemInput,
  MoveCloudProjectWorkItemInput,
	DeleteCloudProjectWorkItemInput,
  MutateCloudProjectWorkItemLabelInput,
  ProjectInstructionVersionMutationInput,
  ProjectVersionMutationInput,
  PostCloudProjectMessageInput,
  MarkCloudProjectActivityReadInput,
  MarkCloudProjectMessageReadInput,
} from "../features/projects/model/projectTypes";

export {
  installWailsNonFileDragErrorSuppression,
  isWailsNonFileDragError,
  isWailsNonFileDragErrorEvent,
  onFilesDropped,
  openExternal,
} from "../platform/wails/wailsFiles";

type ExpertListFilter = { mode?: string; category?: string; query?: string };

interface NativeConfirmRequest {
  title: string;
  message: string;
  detail: string;
  confirmLabel: string;
  cancelLabel: string;
  destructive: boolean;
}

interface DesktopWindowState {
  width: number;
  height: number;
  x: number;
  y: number;
  maximised: boolean;
}

export interface OnboardingModelsResult {
  models: string[];
  defaultModel: string;
  usedFallback: boolean;
  warning?: string;
}

export interface CloudRegisterRequest {
  phone: string;
  password: string;
  sms_code: string;
}

export interface CloudLoginRequest {
  phone: string;
  password: string;
  captcha_passed?: boolean;
  captcha_ticket?: string;
  captcha_provider?: string;
}

export interface CloudCaptchaConfig {
  provider: string;
  region: string;
  prefix: string;
  scene_id: string;
}

export interface CloudSendRegisterSMSRequest {
  phone: string;
  captcha_passed?: boolean;
  captcha_ticket?: string;
  captcha_provider?: string;
}

export interface CloudSendPasswordResetSMSRequest {
  phone: string;
  captcha_ticket: string;
  captcha_provider: string;
}

export interface CloudResetPasswordRequest {
  phone: string;
  new_password: string;
  sms_code: string;
}

export interface CloudAssetsChangedView {
  balanceChanged: boolean;
  subscriptionChanged: boolean;
}

export interface ConnectorCatalogItemView {
  id: string;
  name: string;
  provider: string;
  description: string;
  category: string;
  icon_url?: string;
  examples: string[];
  kind: string;
  version: string;
  compatible: boolean;
  blocked_reason?: string;
  capabilities: string[];
}

export interface ConnectorAuthFieldView {
  key: string;
  server_id: string;
  id: string;
  label: string;
  type: "text" | "secret";
  required: boolean;
  documentation_url: string;
  documentation_label: string;
}

export interface ConnectorCatalogView {
  catalog_version: string;
  items: ConnectorCatalogItemView[];
}

export interface ConnectorStateView {
  connector_id: string;
  status: "not_installed" | "installing" | "authorization_required" | "authorizing" | "connected" | "repair_required" | "failed";
  version?: string;
  sha256?: string;
  error?: { code: string; message: string; retryable: boolean };
  tools?: number;
}

export type ClientErrorFrameCategory = "app" | "framework" | "runtime" | "unknown";

export interface ClientErrorFrame {
  category: ClientErrorFrameCategory;
  ordinal: number;
  line: number;
  column: number;
}

export interface ClientErrorRecord {
  scope: string;
  code: string;
  errorType: string;
  occurredAt: string;
	diagnosticId?: string;
  stackFrames?: ClientErrorFrame[];
  componentFrames?: ClientErrorFrame[];
  projectComposer?: ProjectComposerDiagnostic;
}

// AppBindings is the hand-written contract between the React app and the Go
// kernel. It uses local types (types.ts) so components don't import generated
// model classes. _CheckGeneratedBindings catches drift: when a Go method is
// added or renamed, the generated types shift, and a key present in GeneratedApp
// but missing from AppBindings causes a type error here. Fix: add the new method
// to AppBindings, then run `pnpm typecheck` to verify.
export interface AppBindings {
  Platform(): Promise<string>;
  ClientTimezone(): Promise<string>;
  SetClientTimezone(timezone: string): Promise<void>;
  EnsureCapabilityTaskBinding(request: { capabilityId: string; workspaceId: string; taskId: string; documentId: string }): Promise<{ capabilityId: string; workspaceId: string; taskId: string; documentId: string }>;
  EnsureCapabilityRuntime(capabilityId: string): Promise<string[]>;
  EnsureCapabilityDashboard(scope: { capabilityId: string; workspaceId: string; taskId: string; documentId: string }, seedJSON: string): Promise<string>;
  LoadCapabilityDashboard(scope: { capabilityId: string; workspaceId: string; taskId: string; documentId: string }): Promise<string>;
  ApplyCapabilityDashboardAction(scope: { capabilityId: string; workspaceId: string; taskId: string; documentId: string }, actionJSON: string): Promise<string>;
  EnsureCapabilityEvidence(scope: { capabilityId: string; workspaceId: string; taskId: string; documentId: string }, seedJSON: string): Promise<string>;
  LoadCapabilityEvidence(scope: { capabilityId: string; workspaceId: string; taskId: string; documentId: string }): Promise<string>;
  RunCapabilityEbayBrowseResearch(scope: { capabilityId: string; workspaceId: string; taskId: string; documentId: string }, request: { query: string; marketplace: string; limit: number }): Promise<{ query: string; marketplace: string; capturedAt: string; evidenceJson: string; dashboardJson: string }>;
  CloudBootstrap(): Promise<CloudBootstrapView>;
  CloudCachedAssets(): Promise<CloudCachedAssetsView>;
  CloudCaptchaConfig(): Promise<CloudCaptchaConfig>;
  CloudSendRegisterSMS(request: CloudSendRegisterSMSRequest): Promise<void>;
  CloudSendPasswordResetSMS(request: CloudSendPasswordResetSMSRequest): Promise<void>;
  CloudResetPassword(request: CloudResetPasswordRequest): Promise<void>;
  CloudRegister(request: CloudRegisterRequest): Promise<CloudBootstrapView>;
  CloudLogin(request: CloudLoginRequest): Promise<CloudBootstrapView>;
  CloudSwitchAccount(request: CloudLoginRequest): Promise<CloudBootstrapView>;
  CloudLogout(): Promise<CloudBootstrapView>;
  CloudRefreshAssets(): Promise<CloudBootstrapView>;
  PickFeedbackImages(existingTokens: string[]): Promise<FeedbackImageSelectionView>;
  RemoveFeedbackImage(token: string): Promise<void>;
  DiscardFeedbackDraft(tokens: string[]): Promise<void>;
  SubmitFeedback(request: FeedbackSubmitRequest): Promise<FeedbackSubmitView>;
  CloudCreateSubscriptionOrder(plan: string, channel: CloudPaymentView["payment_channel"]): Promise<CloudPaymentOperationView>;
  CloudCreateUpgradeOrder(targetPlan: string, channel: CloudPaymentView["payment_channel"]): Promise<CloudPaymentOperationView>;
  CloudConfirmPayment(orderID: string): Promise<CloudPaymentOperationView>;
  CloudRefreshPayment(orderID: string): Promise<CloudPaymentOperationView>;
  CloudCreateMaaSRechargeOrder(amount: string, paymentMethod: "alipay", idempotencyKey: string): Promise<CloudMaaSRechargeOrderView>;
  CloudGetMaaSRechargeOrder(orderID: string): Promise<CloudMaaSRechargeOrderView>;
  CloudGetMaaSRechargeKeyBalance(): Promise<CloudMaaSRechargeKeyBalanceView>;
  // ── Heartbeat ──
  HeartbeatListTasks(): Promise<unknown>;
  HeartbeatReloadTasks(): Promise<unknown>;
  HeartbeatSaveTasks(tasks: unknown): Promise<void>;
  HeartbeatTriggerNow(id: string): Promise<void>;
  HeartbeatGenerateID(): Promise<string>;
  AutomationListTasks(): Promise<unknown>;
  AutomationReloadTasks(): Promise<unknown>;
  AutomationSaveTasks(tasks: unknown): Promise<void>;
	AutomationCreateTask(task: unknown): Promise<unknown>;
	AutomationUpdateTask(task: unknown, expectedRevision: number): Promise<unknown>;
	AutomationSetTaskEnabled(id: string, enabled: boolean, expectedRevision: number): Promise<unknown>;
	AutomationDeleteTask(id: string, expectedRevision: number): Promise<void>;
	AutomationGetScheduleCatalog(): Promise<unknown>;
	AutomationListDeliveryTargets(): Promise<unknown>;
	AutomationValidateDraft(task: unknown): Promise<unknown>;
	AutomationListCapabilities(): Promise<unknown>;
	AutomationResolveCapabilities(task: unknown): Promise<unknown>;
  AutomationTaskQuota(): Promise<unknown>;
  AutomationTriggerNow(id: string): Promise<unknown>;
  AutomationListRuns(taskID: string, limit: number): Promise<unknown>;
	AutomationListRunsV2(filter: { taskId?: string; scope?: string; limit?: number }, cursor: string): Promise<unknown>;
  AutomationReadRun(taskID: string, runID: string): Promise<unknown>;
  AutomationListDeliveries(taskID: string, runID: string): Promise<unknown>;
  AutomationRetryDelivery(taskID: string, runID: string, attemptID: string): Promise<unknown>;
  AutomationExportRunResult(taskID: string, runID: string, format: string): Promise<unknown>;
  AutomationCancelRun(taskID: string, runID: string): Promise<unknown>;
  AutomationListTemplates(): Promise<unknown>;
  AutomationSaveTaskTemplate(task: unknown): Promise<unknown>;
  AutomationDeleteTaskTemplate(id: string): Promise<void>;
  AutomationRestoreTaskTemplate(id: string): Promise<void>;
  AutomationGenerateID(): Promise<string>;
  AutomationHeartbeatMigrationStatus(): Promise<unknown>;
  AutomationModels(workspaceRoot: string): Promise<ModelInfo[]>;
  Submit(input: string): Promise<void>;
  SubmitToTab(tabID: string, input: string): Promise<void>;
  SetSceneForTab(tabID: string, scene: string): Promise<TabMeta>;
  SubmitDisplay(display: string, input: string): Promise<void>;
  SubmitDisplayToTab(tabID: string, display: string, input: string): Promise<void>;
  RunShell(command: string): Promise<void>;
  RunShellForTab(tabID: string, command: string): Promise<void>;
  Steer(text: string): Promise<void>;
  SteerForTab(tabID: string, text: string): Promise<void>;
  Cancel(): Promise<void>;
  CancelTab(tabID: string): Promise<void>;
  Approve(id: string, allow: boolean, session: boolean, persist: boolean): Promise<void>;
  ApproveTab(tabID: string, id: string, allow: boolean, session: boolean, persist: boolean): Promise<PromptReplyReceipt | void>;
  AnswerQuestion(id: string, answers: QuestionAnswer[]): Promise<void>;
  AnswerQuestionForTab(tabID: string, id: string, answers: QuestionAnswer[]): Promise<PromptReplyReceipt | void>;
  RecordPromptReplyFailureForTab(tabID: string, promptID: string): Promise<PromptReplyReceipt | void>;
  ReplayPendingPrompts(): Promise<void>;
  SetPlanMode(on: boolean): Promise<void>;
  SetMode(mode: string): Promise<void>;
  SetModeForTab(tabID: string, mode: string): Promise<void>;
  SetAutoApproveTools(on: boolean): Promise<void>;
  SetCollaborationMode(mode: string): Promise<void>;
  SetCollaborationModeForTab(tabID: string, mode: string): Promise<void>;
  SetToolApprovalMode(mode: string): Promise<void>;
  SetToolApprovalModeForTab(tabID: string, mode: string): Promise<void>;
  SyncToolApprovalModeForTab(tabID: string, mode: string): Promise<void>;
  SetGoal(goal: string): Promise<void>;
  SetGoalForTab(tabID: string, goal: string): Promise<void>;
  ClearGoal(): Promise<void>;
  ClearGoalForTab(tabID: string): Promise<void>;
  Compact(): Promise<void>;
  NewSession(): Promise<void>;
  ClearSession(): Promise<void>;
  ListExperts(): Promise<ExpertInfo[]>;
  ListExpertsByFilter(input: ExpertListFilter): Promise<ExpertInfo[]>;
  ListSessionExperts(): Promise<SessionExpertInfo[]>;
  CreateExpert(input: ExpertInfo): Promise<ExpertInfo>;
  UpdateExpert(id: string, input: ExpertInfo): Promise<ExpertInfo>;
  DeleteExpert(id: string): Promise<void>;
  ResetExperts(): Promise<ExpertResetResult>;
  ListUserExperts(): Promise<UserExpertInfo[]>;
  ListUserExpertsForTab(tabID: string): Promise<UserExpertInfo[]>;
  ExpertLibraryState(): Promise<ExpertLibraryState>;
  ExpertLibraryStateForTab(tabID: string): Promise<ExpertLibraryState>;
  CreateUserExpert(input: UserExpertInput): Promise<UserExpertInfo>;
  UpdateUserExpert(id: string, input: UserExpertInput): Promise<UserExpertInfo>;
  DeleteUserExpert(id: string): Promise<void>;
  DuplicateExpert(id: string): Promise<UserExpertInfo>;
  SetExpertFavorite(id: string, favorite: boolean): Promise<ExpertLibraryState>;
  ImportExpertConfig(payload: string): Promise<UserExpertInput>;
  CurrentExpert(): Promise<ExpertState>;
  CurrentExpertForTab(tabID: string): Promise<ExpertState>;
  ExpertTeamForTab(tabID: string): Promise<ExpertTeamSession | null>;
  SetExpert(id: string): Promise<ExpertState>;
  SetExpertForTab(tabID: string, id: string): Promise<ExpertState>;
  AttachExpertForTab(tabID: string, id: string): Promise<ExpertState>;
  SetExpertGroup(ids: string[]): Promise<ExpertState>;
  SetExpertGroupForTab(tabID: string, ids: string[]): Promise<ExpertState>;
  ClearExpert(): Promise<ExpertState>;
  ClearExpertForTab(tabID: string): Promise<ExpertState>;
  History(): Promise<HistoryMessage[]>;
  HistoryForTab(tabID: string): Promise<HistoryMessage[]>;
  HistoryPageForTab?(tabID: string, before: number, limit: number, generation: string): Promise<HistoryPage>;
  TranscriptPageForTab?(tabID: string, journalID: string, after: number, through: number): Promise<TranscriptPage | null>;
  TurnBillingForTab?(tabID: string, query: TurnBillingQuery): Promise<TurnBillingResult[]>;
  SubagentHistoryForTab?(tabID: string, ref: string): Promise<HistoryMessage[]>;
  SubagentHistoryPageForTab?(tabID: string, ref: string, before: number, limit: number, generation: string): Promise<HistoryPage>;
  SubagentToolResultForTab(tabID: string, ref: string, toolID: string): Promise<{ args: string; output: string }>;
  ActivateSession?(path: string): Promise<void>;
  ActivateSessionForTab?(tabID: string, path: string): Promise<void>;
  ActivateChannelSessionForTab?(tabID: string, path: string): Promise<void>;
  Checkpoints(): Promise<CheckpointMeta[]>;
  CheckpointsForTab(tabID: string): Promise<CheckpointMeta[]>;
  Rewind(turn: number, scope: string): Promise<void>;
  Fork(turn: number): Promise<TabMeta>;
  SummarizeFrom(turn: number): Promise<void>;
  SummarizeUpTo(turn: number): Promise<void>;
  ListSessions(): Promise<SessionMeta[]>;
  ListAllSessions(): Promise<SessionMeta[]>;
  ListArtifacts(): Promise<ArtifactView[]>;
  ListTrashedSessions(): Promise<SessionMeta[]>;
  ListAllTrashedSessions(): Promise<SessionMeta[]>;
  ResumeSession(path: string): Promise<HistoryMessage[]>;
  ResumeSessionForTab(tabID: string, path: string): Promise<HistoryMessage[]>;
  OpenChannelSessionForTab(tabID: string, path: string): Promise<HistoryMessage[]>;
  PreviewSession(path: string): Promise<HistoryMessage[]>;
  DeleteSession(path: string): Promise<void>;
  RestoreSession(path: string): Promise<void>;
  PurgeTrashedSession(path: string): Promise<void>;
  RenameSession(path: string, title: string): Promise<void>;
  ScanPromptHistory(nonce: string): Promise<PromptHistoryResult>;
  ListWorkspaces(): Promise<WorkspaceView[]>;
	ListCloudProjects(input: CloudProjectListInput): Promise<CloudProjectPage>;
	ListJoinedCloudProjects(input: CloudProjectListInput): Promise<CloudProjectPage>;
	ClaimCloudProjectOnboarding(): Promise<CloudProjectOnboarding>;
	GetCloudProject(projectId: string): Promise<CloudProjectResult>;
	CreateCloudProject(input: CreateCloudProjectInput): Promise<CloudProjectResult>;
	PatchCloudProject(input: PatchCloudProjectInput): Promise<CloudProjectResult>;
	ArchiveCloudProject(input: ProjectVersionMutationInput): Promise<CloudProjectResult>;
	RestoreCloudProject(input: ProjectVersionMutationInput): Promise<CloudProjectResult>;
	GetCloudProjectInstruction(projectId: string): Promise<CloudProjectInstruction>;
	EditCloudProjectInstruction(input: EditCloudProjectInstructionInput): Promise<CloudProjectInstruction>;
	ClearCloudProjectInstruction(input: ProjectInstructionVersionMutationInput): Promise<CloudProjectInstruction>;
	NewCloudProjectSession(input: NewCloudProjectSessionInput): Promise<CloudProjectSession>;
	GetCloudProjectFeatures(): Promise<CloudProjectFeatures>;
	ListCloudProjectAssets(projectId: string): Promise<CloudProjectAssetList>;
	ListCloudProjectAssetProcessingStates(input: CloudProjectAssetProcessingStateListInput): Promise<CloudProjectAssetProcessingStateList>;
	UploadCloudProjectAsset(projectId: string): Promise<CloudProjectAssetList>;
	ListCloudProjectAssetDirectory(input: CloudProjectAssetDirectoryInput): Promise<CloudProjectAssetDirectoryPage>;
	ListCloudProjectAssetOperationRecords(input: CloudProjectAssetOperationRecordListInput): Promise<CloudProjectAssetOperationRecordPage>;
	CreateCloudProjectAssetFolder(input: CreateCloudProjectAssetFolderInput): Promise<CloudProjectAssetDirectoryPage>;
	PatchCloudProjectAssetFolder(input: PatchCloudProjectAssetFolderInput): Promise<CloudProjectAssetDirectoryPage>;
	DeleteCloudProjectAssetFolder(input: DeleteCloudProjectAssetFolderInput): Promise<CloudProjectAssetDirectoryPage>;
	PatchCloudProjectAsset(input: PatchCloudProjectAssetInput): Promise<CloudProjectAssetDirectoryPage>;
	ReplaceCloudProjectAssetContent(input: ReplaceCloudProjectAssetContentInput): Promise<CloudProjectAssetDirectoryPage>;
	UploadCloudProjectAssetToFolder(input: { projectId: string; folderId?: string }): Promise<CloudProjectAssetDirectoryPage>;
	DownloadCloudProjectAsset(projectId: string, assetId: string): Promise<CloudProjectAssetDownload>;
	DeleteCloudProjectAsset(projectId: string, assetId: string): Promise<CloudProjectAssetList>;
	PutCloudProjectWorkItemAsset(input: MutateCloudProjectWorkItemAssetInput): Promise<CloudProjectAsset>;
	DeleteCloudProjectWorkItemAsset(input: MutateCloudProjectWorkItemAssetInput): Promise<CloudProjectAsset>;
	ListCloudProjectCapabilities(projectId: string): Promise<CloudProjectCapabilityList>;
	ListProjectConnectorAvailability(): Promise<ProjectConnectorAvailability[]>;
	InstallProjectConnector(registryRef: string): Promise<ConnectorStateView>;
	SaveCloudProjectCapabilities(input: SaveCloudProjectCapabilitiesInput): Promise<CloudProjectCapabilityList>;
	ListCloudProjectWorkItems(input: CloudProjectWorkItemListInput): Promise<CloudProjectWorkItemPage>;
	CreateCloudProjectWorkItem(input: CreateCloudProjectWorkItemInput): Promise<CloudProjectWorkItemResult>;
	PatchCloudProjectWorkItem(input: PatchCloudProjectWorkItemInput): Promise<CloudProjectWorkItemResult>;
	MoveCloudProjectWorkItem(input: MoveCloudProjectWorkItemInput): Promise<CloudProjectWorkItemResult>;
	DeleteCloudProjectWorkItem(input: DeleteCloudProjectWorkItemInput): Promise<CloudProjectWorkItemDeleteResult>;
	ListCloudProjectLabels(projectId: string): Promise<CloudProjectLabelList>;
	CreateCloudProjectLabel(input: CreateCloudProjectLabelInput): Promise<CloudProjectLabelList>;
	PatchCloudProjectLabel(input: PatchCloudProjectLabelInput): Promise<CloudProjectLabelList>;
	DeleteCloudProjectLabel(input: DeleteCloudProjectLabelInput): Promise<CloudProjectLabelList>;
	PutCloudProjectWorkItemLabel(input: MutateCloudProjectWorkItemLabelInput): Promise<CloudProjectWorkItemResult>;
	DeleteCloudProjectWorkItemLabel(input: MutateCloudProjectWorkItemLabelInput): Promise<CloudProjectWorkItemResult>;
	ListCloudProjectMembers(projectId: string): Promise<CloudProjectMemberList>;
	JoinCloudProject(input: JoinCloudProjectInput): Promise<JoinCloudProjectResult>;
	CreateCloudProjectInvite(input: CloudProjectInviteCreateInput): Promise<CloudProjectInviteCreateResult>;
	ListCloudProjectInvites(projectId: string): Promise<CloudProjectInviteList>;
	RevokeCloudProjectInvite(input: CloudProjectInviteMutationInput): Promise<void>;
	DeactivateCloudProjectMember(input: CloudProjectMemberMutationInput): Promise<void>;
	AcceptCloudProjectInvite(input: AcceptCloudProjectInviteInput): Promise<AcceptCloudProjectInviteResult>;
	DeleteCloudProject(input: DeleteCloudProjectInput): Promise<void>;
	ListCloudProjectActivities(input: CloudProjectActivityListInput): Promise<CloudProjectActivityPage>;
	ListCloudProjectMessages(input: CloudProjectMessageListInput): Promise<CloudProjectActivityPage>;
	PostCloudProjectMessage(input: PostCloudProjectMessageInput): Promise<CloudProjectActivity>;
	MarkCloudProjectActivityRead(input: MarkCloudProjectActivityReadInput): Promise<void>;
	MarkCloudProjectMessageRead(input: MarkCloudProjectMessageReadInput): Promise<void>;
  PickWorkspace(): Promise<string>;
  SwitchWorkspace(path: string): Promise<string>;
  RemoveWorkspace(path: string): Promise<void>;
  ContextUsage(): Promise<ContextInfo>;
  ContextUsageForTab(tabID: string): Promise<ContextInfo>;
  Balance(): Promise<BalanceInfo>;
  BalanceForTab(tabID: string): Promise<BalanceInfo>;
  Jobs(): Promise<JobView[]>;
  JobsForTab(tabID: string): Promise<JobView[]>;
  ToolResultForTab(tabID: string, toolID: string): Promise<{ args: string; output: string } | null>;
  Meta(): Promise<Meta>;
  MetaForTab(tabID: string): Promise<Meta>;
  Commands(): Promise<CommandInfo[]>;
  Capabilities(): Promise<CapabilitiesView>;
  MCPServers(): Promise<ServerView[]>;
  ConnectorCatalog(tabID: string): Promise<ConnectorCatalogView>;
  ConnectorStates(tabID: string): Promise<ConnectorStateView[]>;
  InstallConnector(tabID: string, connectorID: string): Promise<ConnectorStateView>;
  ConnectorAuthFields(tabID: string, connectorID: string): Promise<ConnectorAuthFieldView[]>;
  ConnectConnector(tabID: string, connectorID: string, tokenValues: Record<string, string>): Promise<ConnectorStateView>;
	CancelConnectorAuthorization(tabID: string, connectorID: string): Promise<void>;
  DisconnectConnector(tabID: string, connectorID: string): Promise<ConnectorStateView>;
  SkillsSettings(): Promise<SkillsSettingsView>;
  PublicSkills(limit: number, cursor: string): Promise<PublicSkillsPageView>;
  SearchPublicSkills(keyword: string, limit: number, cursor: string): Promise<PublicSkillsPageView>;
  PublicSkill(namespace: string, slug: string): Promise<PublicSkillDetailView>;
  InstallPublicSkill(namespace: string, slug: string): Promise<PublicSkillInstallResult>;
  UninstallPublicSkill(slug: string): Promise<void>;
  AddMCPServer(input: MCPServerInput): Promise<number>;
  InstallMCPServer(input: MCPServerInput): Promise<MCPConnectionTestResult>;
  TestMCPServer(name: string): Promise<MCPConnectionTestResult>;
  SubmitCapabilitiesToTab(tabID: string, display: string, input: ComposerCapabilitySubmit): Promise<void>;
  UpdateMCPServer(name: string, input: MCPServerInput): Promise<void>;
  RemoveMCPServer(name: string): Promise<void>;
  ReconnectMCPServer(name: string): Promise<void>;
  ClearMCPServerAuthentication(name: string): Promise<void>;
  TrustMCPServerTool(name: string, toolName: string): Promise<void>;
  TrustMCPServerTools(name: string, toolNames: string[]): Promise<void>;
  UntrustMCPServerTool(name: string, toolName: string): Promise<void>;
  PickSkillFolder(): Promise<string>;
  AddSkillPath(path: string): Promise<void>;
  RemoveSkillPath(path: string): Promise<void>;
  RefreshSkills(): Promise<void>;
  ReloadCommands(): Promise<void>;
  SetSkillEnabled(name: string, enabled: boolean): Promise<void>;
  SetMCPServerEnabled(name: string, enabled: boolean): Promise<void>;
  SetMCPServerTier(name: string, tier: string): Promise<void>;
  SlashArgs(input: string): Promise<SlashArgsResult>;
  ListDir(rel: string): Promise<DirEntry[]>;
  ListDirForTab(tabID: string, rel: string): Promise<DirEntry[]>;
  SearchFileRefs(query: string): Promise<DirEntry[]>;
  SearchFileRefsForTab(tabID: string, query: string): Promise<DirEntry[]>;
  ReadFile(rel: string): Promise<FilePreview>;
  ReadFileForTab(tabID: string, rel: string): Promise<FilePreview>;
  ReadFileForWorkspace(workspaceRoot: string, rel: string): Promise<FilePreview>;
  WorkspaceChanges(tabID: string): Promise<WorkspaceChangesView>;
  GitBranches(): Promise<string[]>;
  GitCheckout(branch: string): Promise<void>;
  WorkspaceGitHistory(tabID: string, path: string): Promise<GitCommitView[]>;
  WorkspaceGitCommitDetail(tabID: string, hash: string, path: string): Promise<GitCommitDetailView>;
  OpenWorkspacePath(rel: string): Promise<void>;
  OpenWorkspacePathForTab(tabID: string, rel: string): Promise<void>;
  OpenWorkspacePathForWorkspace(workspaceRoot: string, rel: string): Promise<void>;
  RevealWorkspacePath(rel: string): Promise<void>;
  RevealWorkspacePathForTab(tabID: string, rel: string): Promise<void>;
  RevealPath(path: string): Promise<void>;
  SavePastedImage(dataUrl: string): Promise<string>;
  SaveClipboardImage(): Promise<string>;
  SavePastedFile(name: string, dataUrl: string): Promise<string>;
  PickExportFile(defaultFilename: string, mimeType: string): Promise<string>;
  PickAutomationSourceFile(sourceType: string): Promise<string>;
  SaveExportFile(path: string, payload: string, base64Encoded: boolean): Promise<void>;
  AttachDropped(path: string): Promise<DroppedItem>;
  AttachmentDataURL(path: string): Promise<string>;
  ModelMarketplace(): Promise<ModelMarketplaceView>;
  RefreshModelMarketplace(): Promise<ModelMarketplaceView>;
  Models(): Promise<ModelInfo[]>;
  SetModel(name: string): Promise<void>;
  ModelsForTab(tabID: string): Promise<ModelInfo[]>;
  SetModelForTab(tabID: string, name: string): Promise<void>;
  SetModelRouteForTab(tabID: string, name: string, billingSource: string): Promise<void>;
  Effort(): Promise<EffortInfo>;
  SetEffort(level: string): Promise<void>;
  EffortForTab(tabID: string): Promise<EffortInfo>;
  EffortForDraftModel(scope: string, workspaceRoot: string, modelRef: string, billingSource: string, currentEffort: string): Promise<EffortInfo>;
  SetEffortForTab(tabID: string, level: string): Promise<void>;
  SetTokenMode(mode: string): Promise<void>;
  SetTokenModeForTab(tabID: string, mode: string): Promise<void>;
  Memory(): Promise<MemoryView>;
  MemorySuggestions(): Promise<MemorySuggestionsView>;
  AcceptMemorySuggestion(suggestion: MemorySuggestion): Promise<string>;
  AcceptSkillSuggestion(suggestion: SkillSuggestion): Promise<string>;
  MemoryForTab(tabID: string): Promise<MemoryView>;
  MemorySuggestionsForTab(tabID: string): Promise<MemorySuggestionsView>;
  AcceptMemorySuggestionForTab(tabID: string, suggestion: MemorySuggestion): Promise<string>;
  AcceptSkillSuggestionForTab(tabID: string, suggestion: SkillSuggestion): Promise<string>;
  Remember(scope: string, note: string): Promise<string>;
  RememberForTab(tabID: string, scope: string, note: string): Promise<string>;
  Forget(name: string): Promise<void>;
  ForgetForTab(tabID: string, name: string): Promise<void>;
  SaveDoc(path: string, body: string): Promise<string>;
  SaveDocForTab(tabID: string, path: string, body: string): Promise<string>;
  DesktopStartupSettings(): Promise<DesktopStartupSettingsView>;
  SettingsCore(): Promise<SettingsCoreView>;
  SettingsModelDetails(): Promise<SettingsModelDetailsView>;
  Settings(): Promise<SettingsView>;
  SetDefaultModel(ref: string): Promise<void>;
  SetDefaultModelRoute(ref: string, billingSource: string): Promise<void>;
  SetPlannerModel(ref: string): Promise<void>;
  SetSubagentModel(ref: string): Promise<void>;
  SetSubagentEffort(level: string): Promise<void>;
  SetAutoPlan(mode: string): Promise<void>;
  SetDefaultToolApprovalMode(mode: string): Promise<void>;
  SaveProvider(p: ProviderView): Promise<void>;
  SaveProviderConfigOnly(p: ProviderView): Promise<void>;
  AddOfficialProviderAccess(kind: string, key: string): Promise<string>;
  FetchProviderModels(p: ProviderView): Promise<string[]>;
  RefreshOfficialModels(): Promise<void>;
  DeleteProvider(name: string): Promise<void>;
  RemoveProviderAccess(name: string): Promise<void>;
  SetProviderKey(apiKeyEnv: string, value: string): Promise<string>;
  SetProviderKeyConfigOnly(apiKeyEnv: string, value: string): Promise<string>;
  ClearProviderKey(apiKeyEnv: string): Promise<void>;
  ClearProviderKeyConfigOnly(apiKeyEnv: string): Promise<void>;
  SetPermissionMode(mode: string): Promise<void>;
  AddPermissionRule(list: string, rule: string): Promise<void>;
  RemovePermissionRule(list: string, rule: string): Promise<void>;
  SetSandbox(bash: string, network: boolean, workspaceRoot: string, allowWrite: string[], shell: string): Promise<void>;
  SetBotSettings(b: BotSettingsView): Promise<void>;
  SetBotSecret(envName: string, value: string): Promise<void>;
  ClearBotSecret(envName: string): Promise<void>;
  StartBotConnectionInstall(provider: string, domain: string): Promise<BotInstallStartResult>;
  PollBotConnectionInstall(installID: string): Promise<BotInstallPollResult>;
  BotRuntimeStatus(): Promise<BotRuntimeStatusView>;
  DiagnoseBotConnection(id: string): Promise<BotConnectionDiagnostic>;
  TestBotConnection(id: string, target?: string): Promise<BotConnectionDiagnostic>;
  SetCloseBehavior(mode: string): Promise<void>;
  SetDisplayMode(mode: string): Promise<void>;
  SetStatusBarStyle(style: string): Promise<void>;
  SetStatusBarItems(items: string[]): Promise<void>;
  SetDesktopLanguage(lang: string): Promise<void>;
  SetDesktopAppearance(theme: string, style: string): Promise<void>;
  SetDesktopLayoutStyle(style: string): Promise<void>;
  SetDesktopCheckUpdates(enabled: boolean): Promise<void>;
  SetDesktopTelemetry(enabled: boolean): Promise<void>;
  SetDesktopMetrics(enabled: boolean): Promise<void>;
  SetExpandThinking(on: boolean): Promise<void>;
  MigrateDesktopPreferences(language: string, theme: string, style: string): Promise<void>;
  SetAgentParams(temperature: number, maxSteps: number, plannerMaxSteps: number, systemPrompt: string): Promise<void>;
  SetColdResumePrune(enabled: boolean): Promise<void>;
  SetReasoningLanguage(lang: string): Promise<void>;
  SetTrayLocale(locale: "en" | "zh" | "zh-TW"): Promise<void>;
  // SetBypass is the legacy Wails name for YOLO/full-access tool auto-approval
  // (ask questions and plan approvals still wait; deny rules still apply).
  // Runtime-only.
  SetBypass(on: boolean): Promise<void>;
  Version(): Promise<string>;
  CheckUpdate(): Promise<UpdateInfo | null>;
  DownloadUpdate(): Promise<UpdateDownloadResult | null>;
  InstallUpdate(): Promise<void>;
  ApplyUpdate(): Promise<void>;
  OpenDownloadPage(): Promise<void>;
  OpenUpdateDownload(url: string): Promise<void>;
  NeedsOnboarding(): Promise<boolean>;
  ConnectKey(apiKey: string): Promise<OnboardingModelsResult>;
  CompleteOnboardingModelSelection(defaultModel: string, models: string[]): Promise<void>;
  // Crash overlay "Send report" (desktop/crash_app.go): scrubs user paths, attaches
  // version/os/arch, POSTs to the collection endpoint. Only ever sent on user click.
  ReportCrash(kind: string, detail: string): Promise<void>;
  RecordClientError(record: ClientErrorRecord): Promise<void>;
  ListTabs(): Promise<TabMeta[]>;
  ExportOperationalEvidence(tabID: string): Promise<string>;
  ExportOperationalPromptEvidence(tabID: string): Promise<string>;
  RetryTabStartup(tabID: string): Promise<TabMeta>;
  OpenProjectTab(workspaceRoot: string, topicID: string): Promise<TabMeta>;
  OpenGlobalTab(topicID: string): Promise<TabMeta>;
  OpenTopicSession(scope: string, workspaceRoot: string, topicID: string, sessionPath: string): Promise<TabMeta>;
  EnsureBlankTab(scope: string, workspaceRoot: string): Promise<TabMeta>;
  ActivateTopic(scope: string, workspaceRoot: string, topicID: string, sessionPath: string): Promise<TabMeta>;
  EnsureBlankSurface(scope: string, workspaceRoot: string): Promise<TabMeta>;
  SetActiveTab(tabID: string): Promise<void>;
  ReorderTabs(tabIDs: string[]): Promise<void>;
  CloseTab(tabID: string): Promise<void>;
  ListProjectTree(): Promise<ProjectNode[]>;
  ProjectTreeRevision(): Promise<number>;
  RenameProject(workspaceRoot: string, title: string): Promise<void>;
  SetProjectColor(workspaceRoot: string, color: string): Promise<void>;
  SetProjectPinned(workspaceRoot: string, pinned: boolean): Promise<void>;
  ReorderProjects(workspaceRoots: string[]): Promise<void>;
  CreateTopic(scope: string, workspaceRoot: string, title: string): Promise<TopicMeta>;
  RenameTopic(topicID: string, title: string): Promise<void>;
  DeleteTopic(topicID: string): Promise<void>;
  TrashTopic(topicID: string): Promise<void>;
  SetTopicPinned(topicID: string, pinned: boolean): Promise<void>;
  SetCloudProjectSessionPinned(path: string, pinned: boolean): Promise<void>;
  ContextPanel(tabID: string): Promise<ContextPanelInfo>;
  ProjectContextForTab(tabID: string): Promise<unknown>;
  // New native-feel bindings (added with the desktop native-feel plan).
  ConfirmAction(req: NativeConfirmRequest): Promise<boolean>;
  SaveWindowState(state: DesktopWindowState): Promise<void>;
}

// Compile-time drift check. Exclude<A, B> extracts keys in A that are missing
// from B. If that set is non-empty, AssertNever<non-never> fails with
// "Type 'X' does not satisfy the constraint 'never'".
// _CheckGenToApp errors mean a generated Go method has no TS counterpart.
// These compare method *names* only; full signature checking isn't possible here
// because local types (types.ts) use plain interfaces while generated types
// (models.ts) use classes with a convertValues prototype method. The structural
// mismatch would produce false positives. Method-arity and parameter-order drift
// are caught at the call sites by tsc when components invoke app.<method>(...).
type AssertNever<T extends never> = T;
type GeneratedAppKeys = keyof typeof GeneratedApp;
type GeneratedAppMissing =
  string extends GeneratedAppKeys ? true :
  number extends GeneratedAppKeys ? true :
  symbol extends GeneratedAppKeys ? true :
  false;
export type _CheckGenToApp = AssertNever<
  GeneratedAppMissing extends true ? never : Exclude<GeneratedAppKeys, keyof AppBindings>
>;

declare global {
  interface Window {
    runtime?: WailsRuntime;
    go?: { main?: { App?: AppBindings } };
  }
}

// Must match desktop/app.go's eventChannel constant.
const EVENT_CHANNEL = "agent:event";
const CONVERSATION_ACTIVITY_EVENT_CHANNEL = "conversation:activity";

// Resolve the Wails binding at CALL time, not module-load time: in dev the Wails
// runtime can inject window.go AFTER this module first evaluates, so snapshotting
// once would miss the live desktop binding for the whole session.
function realApp(): AppBindings | undefined {
  return typeof window !== "undefined" ? window.go?.main?.App : undefined;
}

function missingWailsBindingError(): Error {
  return new Error("Wails desktop backend bindings are missing; browser mock data is disabled.");
}

// onEvent subscribes to the agent's typed event stream; returns an unsubscribe.
export function onEvent(cb: (e: WireEvent) => void): () => void {
  const rt = getWailsRuntime();
  if (rt) {
    return rt.EventsOn(EVENT_CHANNEL, (payload) => cb(payload as WireEvent));
  }
  throw missingWailsBindingError();
}

// App-scoped lifecycle events remain available when a running conversation is
// detached from the visible tab surface. They intentionally contain no stream
// output, so background completion cannot leak into the active transcript.
export function onConversationActivity(cb: (e: WireEvent) => void): () => void {
  const rt = getWailsRuntime();
  if (rt) {
    return rt.EventsOn(CONVERSATION_ACTIVITY_EVENT_CHANNEL, (payload) => cb(payload as WireEvent));
  }
  throw missingWailsBindingError();
}

export function onCloudAssetsChanged(cb: (view: CloudAssetsChangedView) => void): () => void {
  const rt = getWailsRuntime();
  if (!rt) return () => {};
  try {
    return rt.EventsOn("cloud-assets:changed", (payload) => {
      if (payload === null || typeof payload !== "object") return;
      const view = payload as Partial<CloudAssetsChangedView>;
      cb({
        balanceChanged: view.balanceChanged === true,
        subscriptionChanged: view.subscriptionChanged === true,
      });
    });
  } catch {
    return () => {};
  }
}

export function onTurnBillingSettled(cb: (view: TurnBillingSettled) => void): () => void {
  const rt = getWailsRuntime();
  if (!rt) return () => {};
  return rt.EventsOn("turn-billing:settled", (payload) => {
    if (payload === null || typeof payload !== "object") return;
    const view = payload as Partial<TurnBillingSettled>;
    if (typeof view.taskId !== "string" || typeof view.chargedCredits !== "string") return;
    cb({ taskId: view.taskId, chargedCredits: view.chargedCredits });
  });
}

// onUpdaterProgress subscribes to the auto-updater's progress events (a separate
// channel from the agent stream); returns an unsubscribe. Must match the event
// name emitted in desktop/updater_app.go.
export function onUpdaterProgress(cb: (p: UpdateProgress) => void): () => void {
  const rt = getWailsRuntime();
  if (rt) {
    return rt.EventsOn("updater:progress", (p) => cb(p as UpdateProgress));
  }
  throw missingWailsBindingError();
}


// onReady subscribes to the agent:ready event fired when boot.Build completes.
// The frontend re-fetches Meta/Context/History when this lands.
export function onReady(cb: () => void): () => void {
  const rt = getWailsRuntime();
  if (rt) {
    return rt.EventsOn("agent:ready", () => cb());
  }
  throw missingWailsBindingError();
}

// Wait for the next backend-ready notification, with a bounded timeout for
// older runtimes or a missed event. Callers re-check ListTabs after the signal.
function waitForReadySignal(timeoutMs = 500): Promise<void> {
  return new Promise((resolve) => {
    let settled = false;
    let off = () => {};
    const finish = () => {
      if (settled) return;
      settled = true;
      window.clearTimeout(timer);
      off();
      resolve();
    };
    const timer = window.setTimeout(finish, timeoutMs);
    try {
      off = onReady(finish);
      if (settled) off();
    } catch {
      // Compatibility fallback: the timeout still wakes callers so they can
      // re-check ListTabs against an older or partially initialized runtime.
    }
  });
}

// Prefer backend-ready events and use a bounded compatibility poll when an
// older runtime misses them. The check returns true once its target is ready,
// failed, or no longer exists.
export async function waitForBackendReady(check: () => Promise<boolean>): Promise<void> {
  for (let attempt = 0; attempt < 12; attempt += 1) {
    if (await check()) return;
    if (attempt < 11) await waitForReadySignal();
  }
}

export function onProjectTreeChanged(cb: (revision?: number) => void): () => void {
  const rt = getWailsRuntime();
  if (rt) {
    return rt.EventsOn("project-tree:changed", (revision) => cb(typeof revision === "number" ? revision : undefined));
  }
  throw missingWailsBindingError();
}

export function onModelRouteChanged(cb: (tabID: string) => void): () => void {
  const rt = getWailsRuntime();
  if (rt) {
    return rt.EventsOn("model-route:changed", (tabID) => cb(typeof tabID === "string" ? tabID : ""));
  }
  throw missingWailsBindingError();
}

// app proxies every call to the live desktop binding, so a late-injected
// window.go is picked up transparently without creating a second data owner.
function bridgeBreadcrumb(method: string): string {
  if (method === "ReportCrash" || method === "RecordClientError") return "";
  if (/^(Submit|SubmitDisplay|RunShell|Steer|Cancel|Approve|AnswerQuestion|ReplayPendingPrompts)/.test(method))
    return `turn ${method}`;
  if (/^(SetModel|SetEffort|SetTokenMode|SetDefaultModel|SetPlannerModel|SetSubagentModel|SetSubagentEffort)/.test(method))
    return `model ${method}`;
  if (/^(SetDesktop|SetCloseBehavior|SetDisplayMode|SetStatusBar|SetExpandThinking|SetAutoPlan|SetDefaultToolApprovalMode|SetReasoningLanguage)/.test(method))
    return `settings ${method}`;
  if (/^(SaveProvider|AddOfficialProviderAccess|RemoveProviderAccess|DeleteProvider|SetProviderKey|ClearProviderKey|FetchProviderModels|ConnectKey|CompleteOnboardingModelSelection)/.test(method))
    return `provider ${method}`;
  if (/^(CheckUpdate|DownloadUpdate|InstallUpdate|ApplyUpdate|OpenDownloadPage|OpenUpdateDownload)/.test(method)) return `update ${method}`;
  if (/^(AddMCPServer|InstallMCPServer|TestMCPServer|SubmitCapabilitiesToTab|UpdateMCPServer|RemoveMCPServer|ReconnectMCPServer|ClearMCPServerAuthentication|TrustMCPServerTool|TrustMCPServerTools|UntrustMCPServerTool|SetMCPServer|ConnectorCatalog|ConnectorStates|InstallConnector|ConnectorAuthFields|ConnectConnector|CancelConnectorAuthorization|DisconnectConnector)/.test(method))
    return `mcp ${method}`;
  if (/^(AddSkillPath|RemoveSkillPath|RefreshSkills|SetSkillEnabled|AcceptSkillSuggestion|PublicSkill|SearchPublicSkills|InstallPublicSkill|UninstallPublicSkill)/.test(method))
    return `skill ${method}`;
  if (/^(OpenProjectTab|OpenGlobalTab|OpenTopicSession|EnsureBlankTab|ActivateTopic|EnsureBlankSurface|SetActiveTab|CloseTab|ReorderTabs|CreateTopic|RenameTopic|DeleteTopic|TrashTopic|RenameProject|RemoveWorkspace|SwitchWorkspace|PickWorkspace)/.test(method))
    return `nav ${method}`;
  return "";
}

export const app: AppBindings = new Proxy({} as AppBindings, {
  get(_t, prop) {
    const initialBound = realApp();
    if (initialBound) {
      const initialValue = (initialBound as unknown as Record<string, unknown>)[String(prop)];
      if (typeof initialValue !== "function") return initialValue;
    }
    return (...args: unknown[]) => {
      const method = String(prop);
      const crumb = bridgeBreadcrumb(method);
      if (crumb) addBreadcrumb("bridge", crumb);
      const invoke = (target: AppBindings): unknown => {
        const value = (target as unknown as Record<string, unknown>)[method];
        if (typeof value !== "function") return value;
        return (value as (...methodArgs: unknown[]) => unknown).apply(target, args);
      };
      try {
        const bound = realApp();
        if (!bound) throw missingWailsBindingError();
        const result = invoke(bound);
        if (result && typeof (result as Promise<unknown>).then === "function") {
          return (result as Promise<unknown>).catch((err) => {
            if (crumb) addBreadcrumb("bridge.error", method);
            throw err;
          });
        }
        return result;
      } catch (err) {
        if (crumb) addBreadcrumb("bridge.error", method);
        throw err;
      }
    };
  },
});

// Local diagnostics are deliberately fire-and-forget. Missing or stale Wails
// bindings must never create a renderer rejection or recurse through reporting.
export function recordClientError(record: ClientErrorRecord): void {
  try {
    const result = app.RecordClientError(record);
    if (result && typeof result.then === "function") {
      void result.catch(() => {});
    }
  } catch {
    // Browser-only and partially initialized desktop runtimes are expected.
  }
}
