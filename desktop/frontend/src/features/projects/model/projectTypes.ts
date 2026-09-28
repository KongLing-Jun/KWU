export type CloudProjectStatus = "active" | "archived";
export type CloudProjectListStatus = CloudProjectStatus | "all";

export interface CloudProject {
  projectId: string;
  name: string;
  description: string | null;
  status: CloudProjectStatus;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface CloudProjectPage {
  projectScopeKey: string;
  items: CloudProject[];
  nextCursor: string | null;
  stale: boolean;
  cachedAt: string | null;
}

export interface CloudProjectResult {
  projectScopeKey: string;
  project: CloudProject;
  workspacePath?: string;
  stale: boolean;
  cachedAt: string | null;
}

export interface CloudProjectOnboarding {
  projectScopeKey: string;
  project: CloudProject | null;
  guideVersion: number;
  state: "active" | "completed" | "dismissed" | "deleted";
  created: boolean;
}

export interface CloudProjectListInput {
  pageSize?: number;
  cursor?: string;
  status?: CloudProjectListStatus;
  search?: string;
}

export interface CreateCloudProjectInput {
  name: string;
  descriptionSet?: boolean;
  description: string | null;
  humanDescription?: string;
  agentInstruction?: string;
  skillId?: string | null;
  expertId?: string | null;
  connectorId?: string | null;
  bindings?: Array<{ kind: CloudProjectCapabilityKind; registryRef: string; position: number; config: Record<string, unknown> }>;
}

export interface PatchCloudProjectInput {
  projectId: string;
  expectedVersion: number;
  name?: string;
  descriptionSet?: boolean;
  description: string | null;
}

export interface ProjectVersionMutationInput {
  projectId: string;
  expectedVersion: number;
}

export interface CloudProjectInstruction {
  projectScopeKey: string;
  projectId: string;
  humanDescription: string;
  agentInstruction: string;
  contentHash: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface EditCloudProjectInstructionInput {
  projectId: string;
  expectedVersion: number;
  humanDescription: string;
  agentInstruction: string;
}

export interface ProjectInstructionVersionMutationInput {
  projectId: string;
  expectedVersion: number;
}

export interface CloudProjectSession {
  tabId: string;
  sessionPath: string;
  projectId: string;
  instructionVersion: number;
  contentHash: string;
  reused: boolean;
}

export interface NewCloudProjectSessionInput {
  projectId: string;
  tabId: string;
  workspacePath?: string;
  executionTarget?: "local" | "cloud";
}

export interface CloudProjectFeatures {
  projectScopeKey: string;
  instructionsAvailable: boolean;
  workItemsAvailable: boolean;
  membersAvailable: boolean;
  activityAvailable: boolean;
  workItemFieldsAvailable: boolean;
	bindingsAvailable: boolean;
	assetsAvailable: boolean;
}

export interface CloudProjectAsset {
	assetId: string;
	projectId: string;
	filename: string;
	contentType: string;
	sizeBytes: number;
	sha256: string;
	version: number;
	workItemIds: string[];
	createdAt: string;
	updatedAt: string;
}
export interface CloudProjectAssetList { projectScopeKey: string; projectId: string; items: CloudProjectAsset[]; }
export interface CloudProjectAssetDownload { projectScopeKey: string; projectId: string; assetId: string; filename: string; url: string; expiresAt: string; savedPath: string; }
export type CloudProjectAssetDirectoryKind = "folder" | "asset";
export type CloudProjectAssetDirectoryType = "all" | "folder" | "image" | "video" | "pdf" | "audio" | "document" | "markdown" | "archive" | "other";
export type CloudProjectAssetDirectorySort = "name" | "updated_at";
export type CloudProjectAssetDirectoryDirection = "asc" | "desc";
export interface CloudProjectAssetDirectoryInput {
	projectId: string;
	folderId?: string;
	search?: string;
	type?: CloudProjectAssetDirectoryType;
	sort?: CloudProjectAssetDirectorySort;
	direction?: CloudProjectAssetDirectoryDirection;
	cursor?: string;
	limit?: number;
}
export interface CloudProjectAssetDirectoryBreadcrumb { folderId: string; name: string; }
export type CloudProjectAssetBreadcrumb = CloudProjectAssetDirectoryBreadcrumb;
export interface CloudProjectAssetDirectoryItem {
	kind: CloudProjectAssetDirectoryKind;
	id: string;
	name: string;
	folderId: string | null;
	parentFolderId?: string | null;
	contentType?: string | null;
	sizeBytes?: number | null;
	updatedBy?: { userId: string; displayName: string };
	version: number;
	updatedAt: string;
}
export interface CloudProjectAssetDirectoryPage {
	projectScopeKey: string;
	projectId: string;
	breadcrumbs: CloudProjectAssetDirectoryBreadcrumb[];
	items: CloudProjectAssetDirectoryItem[];
	totalCount: number;
	nextCursor: string | null;
	usageBytes: number;
	quotaBytes: number;
	quotaPlan: string;
}
export type CloudProjectAssetOperation = "create" | "read" | "update" | "delete";
export type CloudProjectAssetOperationResourceKind = "asset" | "folder" | "directory";
export type CloudProjectAssetOperationDetail = "upload" | "create_folder" | "list_directory" | "list_assets" | "download" | "replace_content" | "update_metadata" | "update_folder" | "delete_asset" | "delete_folder";
export interface CloudProjectAssetOperationRecord {
	operationId: string;
	projectId: string;
	resourceKind: CloudProjectAssetOperationResourceKind;
	resourceId: string;
	resourceName: string;
	operation: CloudProjectAssetOperation;
	operationDetail: CloudProjectAssetOperationDetail;
	actorUserId: string;
	actorDisplayName: string;
	resourceVersion: number | null;
	folderId: string | null;
	details: Record<string, unknown>;
	createdAt: string;
}
export interface CloudProjectAssetOperationRecordListInput {
	projectId: string;
	operation?: CloudProjectAssetOperation;
	resourceKind?: CloudProjectAssetOperationResourceKind;
	actorUserId?: string;
	limit?: number;
}
export interface CloudProjectAssetOperationRecordPage {
	projectScopeKey: string;
	projectId: string;
	items: CloudProjectAssetOperationRecord[];
	limit: number;
}
export interface CreateCloudProjectAssetFolderInput { projectId: string; name: string; parentFolderId?: string; }
export interface PatchCloudProjectAssetFolderInput {
	projectId: string;
	folderId: string;
	expectedVersion: number;
	name?: string;
	parentFolderIdSet?: boolean;
	parentFolderId?: string | null;
}
export interface PatchCloudProjectAssetInput {
	projectId: string;
	assetId: string;
	expectedVersion: number;
	name?: string;
	folderIdSet?: boolean;
	folderId?: string | null;
}
export interface DeleteCloudProjectAssetFolderInput { projectId: string; folderId: string; expectedVersion: number; }
export interface ReplaceCloudProjectAssetContentInput { projectId: string; assetId: string; expectedVersion: number; folderId?: string; }
export type CloudProjectAssetScanStatus = "pending" | "scanning" | "clean" | "rejected" | "failed_retryable";
export type CloudProjectAssetDerivedStatus = "not_started" | "not_applicable" | "processing" | "ready" | "failed_retryable";
export type CloudProjectAssetPurgeStatus = "active" | "pending" | "cleanup_pending" | "purged";
export interface CloudProjectAssetProcessingState {
	assetId: string;
	assetVersionId: string;
	versionNumber: number;
	scanStatus: CloudProjectAssetScanStatus;
	parseStatus: CloudProjectAssetDerivedStatus;
	indexStatus: CloudProjectAssetDerivedStatus;
	purgeStatus: CloudProjectAssetPurgeStatus;
	stateVersion: number;
	scanAttempts: number;
	createdAt: string;
	updatedAt: string;
	scanCompletedAt: string | null;
	purgedAt: string | null;
}
export interface CloudProjectAssetProcessingStateList { projectScopeKey: string; projectId: string; items: CloudProjectAssetProcessingState[]; }
export interface CloudProjectAssetProcessingStateListInput { projectId: string; assetIds?: string[]; }
export interface MutateCloudProjectWorkItemAssetInput { projectId: string; workItemId: string; assetId: string; }

export type CloudProjectCapabilityKind = "connector" | "expert" | "skill";
export interface CloudProjectCapabilityBinding {
	bindingId: string;
	kind: CloudProjectCapabilityKind;
	registryRef: string;
	position: number;
	config: Record<string, unknown>;
	publicDisabled: boolean;
	version: number;
}
export interface CloudProjectCapabilityList {
	projectScopeKey: string;
	projectId: string;
	projectVersion: number;
	items: CloudProjectCapabilityBinding[];
}
export interface SaveCloudProjectCapabilitiesInput {
	projectId: string;
	expectedVersion: number;
	bindings?: Array<{ kind: CloudProjectCapabilityKind; registryRef: string; position: number; config: Record<string, unknown> }>;
	skillIdSet?: boolean;
	skillId?: string | null;
	expertIdSet?: boolean;
	expertId?: string | null;
	connectorIdSet?: boolean;
	connectorId?: string | null;
}
export interface ProjectConnectorAvailability {
	registryRef: string;
	name: string;
	description: string;
	iconUrl?: string;
	compatible: boolean;
	localServerName: string;
	localStatus?: "not_installed" | "installing" | "authorization_required" | "authorizing" | "connected" | "repair_required" | "failed";
	available: boolean;
	availabilityReason: string;
}
export interface CloudProjectMember { userId: string; displayName: string; role: "owner" | "member"; status: "active" | "inactive"; version: number; }
export interface CloudProjectMemberList { projectScopeKey: string; projectId: string; currentRole: "owner" | "member"; items: CloudProjectMember[]; }
export interface JoinCloudProjectInput { projectId: string; }
export interface JoinCloudProjectResult extends CloudProjectMember { projectScopeKey: string; projectId: string; createdAt: string; updatedAt: string; workspacePath?: string; }
export interface CloudProjectInvite { inviteId: string; status: string; role: "member"; version: number; expiresAt: string; createdAt: string; }
export interface CloudProjectInviteList { projectScopeKey: string; projectId: string; items: CloudProjectInvite[]; }
export interface CloudProjectInviteCreateInput { projectId: string; }
export interface CloudProjectInviteCreateResult { projectScopeKey: string; projectId: string; inviteId: string; token?: string; expiresAt: string; committedWithoutSecret: boolean; }
export interface CloudProjectInviteMutationInput { projectId: string; inviteId: string; expectedVersion: number; }
export interface CloudProjectMemberMutationInput { projectId: string; userId: string; expectedVersion: number; }
export interface AcceptCloudProjectInviteInput { inviteId: string; token: string; }
export interface AcceptCloudProjectInviteResult { projectScopeKey: string; status: string; projectId?: string; membershipVersion?: number; workspacePath?: string; }
export interface DeleteCloudProjectInput { projectId: string; expectedVersion: number; }

export type CloudProjectActivityFilter = "related_to_me" | "member_activity" | "all_members";
export interface CloudProjectActivityAsset { assetId: string; filename: string; contentType: string; sizeBytes: number; }
export interface CloudProjectMessageMemberReadState { userId: string; displayName: string; status: "read" | "unread"; }
export interface CloudProjectActivity { activityId: string; kind: "user_message" | "system_event"; actorUserId: string | null; authorDisplayName: string; clientMessageId?: string; messageText?: string; eventName?: string; eventPayload?: Record<string, unknown>; mentionUserIds: string[]; assets: CloudProjectActivityAsset[]; unread: boolean; otherMembersUnread?: boolean; unreadOtherMemberCount?: number; otherMemberReadStates?: CloudProjectMessageMemberReadState[]; createdAt: string; }
export interface CloudProjectActivityListInput { projectId: string; pageSize?: number; cursor?: string; filter: CloudProjectActivityFilter; memberUserIds?: string[]; }
export interface CloudProjectMessageListInput { projectId: string; pageSize?: number; cursor?: string; }
export interface CloudProjectActivityPage { projectScopeKey: string; projectId: string; items: CloudProjectActivity[]; nextCursor: string | null; readCursor: string | null; }
export interface PostCloudProjectMessageInput { projectId: string; text: string; mentionUserIds: string[]; assetIds: string[]; }
export interface MarkCloudProjectActivityReadInput { projectId: string; throughCursor: string; }
export interface MarkCloudProjectMessageReadInput { projectId: string; throughCursor: string; }

export type CloudProjectWorkItemStatus = "todo" | "in_progress" | "paused" | "done";
export type CloudProjectWorkItemListStatus = CloudProjectWorkItemStatus | "all";

export interface CloudProjectWorkItem {
  workItemId: string;
  projectId: string;
  title: string;
  description: string | null;
  status: CloudProjectWorkItemStatus;
  rank: number;
  version: number;
  createdAt: string;
  updatedAt: string;
  assigneeUserId: string | null;
  assigneeStatus: string | null;
  assigneeDisplayName: string | null;
  priority: "" | "none" | "low" | "medium" | "high" | "urgent";
  startAt: string | null;
  dueAt: string | null;
  labels: CloudProjectLabel[];
}
export interface CloudProjectLabel { labelId: string; name: string; color: string; version: number; }
export interface CloudProjectLabelList { projectScopeKey: string; projectId: string; items: CloudProjectLabel[]; }
export interface CreateCloudProjectLabelInput { projectId: string; name: string; color: string; }
export interface MutateCloudProjectWorkItemLabelInput { projectId: string; workItemId: string; labelId: string; expectedVersion: number; }

export interface CloudProjectWorkItemPage {
  projectScopeKey: string;
  projectId: string;
  items: CloudProjectWorkItem[];
  nextCursor: string | null;
  totalCount: number;
  undatedCount: number;
}

export interface CloudProjectWorkItemResult {
  projectScopeKey: string;
  projectId: string;
  workItem: CloudProjectWorkItem;
}

export interface CloudProjectWorkItemDeleteResult {
  projectScopeKey: string;
  projectId: string;
  workItemId: string;
  committedVersion: number;
}

export interface CloudProjectWorkItemListInput {
  projectId: string;
  pageSize?: number;
  cursor?: string;
  status?: CloudProjectWorkItemListStatus;
  search?: string;
  assigneeUserId?: string;
  priority?: "none" | "low" | "medium" | "high" | "urgent";
  labelId?: string;
  scheduledFrom?: string;
  scheduledTo?: string;
}

export interface PatchCloudProjectLabelInput { projectId: string; labelId: string; expectedVersion: number; name?: string; color?: string; }
export interface DeleteCloudProjectLabelInput { projectId: string; labelId: string; expectedVersion: number; }

export interface CreateCloudProjectWorkItemInput {
  projectId: string;
  title: string;
  descriptionSet?: boolean;
  description: string | null;
  status?: CloudProjectWorkItemStatus;
  assigneeUserId?: string;
  priority?: "none" | "low" | "medium" | "high" | "urgent";
  startAt?: string;
  dueAt?: string;
}

export interface PatchCloudProjectWorkItemInput {
  projectId: string;
  workItemId: string;
  expectedVersion: number;
  title?: string;
  descriptionSet?: boolean;
  description?: string | null;
  assigneeSet?: boolean;
  assigneeUserId?: string | null;
  priority?: "none" | "low" | "medium" | "high" | "urgent";
  startAtSet?: boolean;
  startAt?: string | null;
  dueAtSet?: boolean;
  dueAt?: string | null;
}

export interface MoveCloudProjectWorkItemInput {
  projectId: string;
  workItemId: string;
  expectedVersion: number;
  targetStatus: CloudProjectWorkItemStatus;
  beforeWorkItemId?: string;
  afterWorkItemId?: string;
}

export interface DeleteCloudProjectWorkItemInput {
  projectId: string;
  workItemId: string;
  expectedVersion: number;
}
