import { app } from "../../../lib/bridge";
import type { PublicSkillInstallResult, PublicSkillsPageView, SkillsSettingsView } from "../../../lib/types";
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
  CloudProjectMemberList,
  CloudProjectMemberMutationInput,
  JoinCloudProjectInput,
  JoinCloudProjectResult,
  DeleteCloudProjectInput,
  PostCloudProjectMessageInput,
  MarkCloudProjectActivityReadInput,
  MarkCloudProjectMessageReadInput,
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
	DeleteCloudProjectAssetFolderInput,
	ReplaceCloudProjectAssetContentInput,
	CloudProjectAssetProcessingStateList,
	CloudProjectAssetProcessingStateListInput,
	MutateCloudProjectWorkItemAssetInput,
	CloudProjectCapabilityList,
	SaveCloudProjectCapabilitiesInput,
  CloudProjectInstruction,
  CloudProjectPage,
  CloudProjectOnboarding,
  CloudProjectResult,
  CloudProjectSession,
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
  EditCloudProjectInstructionInput,
  PatchCloudProjectInput,
  PatchCloudProjectWorkItemInput,
  MoveCloudProjectWorkItemInput,
	DeleteCloudProjectWorkItemInput,
  MutateCloudProjectWorkItemLabelInput,
  NewCloudProjectSessionInput,
  ProjectInstructionVersionMutationInput,
  ProjectVersionMutationInput,
} from "../model/projectTypes";

export interface ProjectGateway {
  listCloudProjects(input: CloudProjectListInput): Promise<CloudProjectPage>;
  listJoinedCloudProjects(input: CloudProjectListInput): Promise<CloudProjectPage>;
  claimCloudProjectOnboarding?(): Promise<CloudProjectOnboarding>;
  getCloudProject(projectId: string): Promise<CloudProjectResult>;
  createCloudProject(input: CreateCloudProjectInput): Promise<CloudProjectResult>;
  patchCloudProject(input: PatchCloudProjectInput): Promise<CloudProjectResult>;
  archiveCloudProject(input: ProjectVersionMutationInput): Promise<CloudProjectResult>;
  restoreCloudProject(input: ProjectVersionMutationInput): Promise<CloudProjectResult>;
  getCloudProjectInstruction(projectId: string): Promise<CloudProjectInstruction>;
  editCloudProjectInstruction(input: EditCloudProjectInstructionInput): Promise<CloudProjectInstruction>;
  clearCloudProjectInstruction(input: ProjectInstructionVersionMutationInput): Promise<CloudProjectInstruction>;
  newCloudProjectSession(input: NewCloudProjectSessionInput): Promise<CloudProjectSession>;
  getCloudProjectFeatures(): Promise<CloudProjectFeatures>;
  listProjectConnectorAvailability(): ReturnType<typeof app.ListProjectConnectorAvailability>;
  installProjectConnector?(registryRef: string): ReturnType<typeof app.InstallProjectConnector>;
  listProjectSkillsSettings(): Promise<SkillsSettingsView>;
  listProjectPublicSkills(limit: number, cursor: string): Promise<PublicSkillsPageView>;
  installProjectPublicSkill(namespace: string, slug: string): Promise<PublicSkillInstallResult>;
	listCloudProjectAssets(projectId: string): Promise<CloudProjectAssetList>;
	listCloudProjectAssetProcessingStates(input: CloudProjectAssetProcessingStateListInput): Promise<CloudProjectAssetProcessingStateList>;
	listCloudProjectAssetDirectory?(input: CloudProjectAssetDirectoryInput): Promise<CloudProjectAssetDirectoryPage>;
	listCloudProjectAssetOperationRecords?(input: CloudProjectAssetOperationRecordListInput): Promise<CloudProjectAssetOperationRecordPage>;
	createCloudProjectAssetFolder?(input: CreateCloudProjectAssetFolderInput): Promise<CloudProjectAssetDirectoryPage>;
	patchCloudProjectAssetFolder?(input: PatchCloudProjectAssetFolderInput): Promise<CloudProjectAssetDirectoryPage>;
	deleteCloudProjectAssetFolder?(input: DeleteCloudProjectAssetFolderInput): Promise<CloudProjectAssetDirectoryPage>;
	patchCloudProjectAsset?(input: PatchCloudProjectAssetInput): Promise<CloudProjectAssetDirectoryPage>;
	replaceCloudProjectAssetContent?(input: ReplaceCloudProjectAssetContentInput): Promise<CloudProjectAssetDirectoryPage>;
	uploadCloudProjectAsset(projectId: string): Promise<CloudProjectAssetList>;
	uploadCloudProjectAsset(projectId: string, folderId: string | undefined): Promise<CloudProjectAssetDirectoryPage>;
	downloadCloudProjectAsset(projectId: string, assetId: string): Promise<CloudProjectAssetDownload>;
	deleteCloudProjectAsset(projectId: string, assetId: string): Promise<CloudProjectAssetList>;
	putCloudProjectWorkItemAsset(input: MutateCloudProjectWorkItemAssetInput): Promise<CloudProjectAsset>;
	deleteCloudProjectWorkItemAsset(input: MutateCloudProjectWorkItemAssetInput): Promise<CloudProjectAsset>;
	listCloudProjectCapabilities(projectId: string): Promise<CloudProjectCapabilityList>;
  saveCloudProjectCapabilities(input: SaveCloudProjectCapabilitiesInput): Promise<CloudProjectCapabilityList>;
  listCloudProjectWorkItems(input: CloudProjectWorkItemListInput): Promise<CloudProjectWorkItemPage>;
  createCloudProjectWorkItem(input: CreateCloudProjectWorkItemInput): Promise<CloudProjectWorkItemResult>;
  patchCloudProjectWorkItem(input: PatchCloudProjectWorkItemInput): Promise<CloudProjectWorkItemResult>;
  moveCloudProjectWorkItem(input: MoveCloudProjectWorkItemInput): Promise<CloudProjectWorkItemResult>;
  deleteCloudProjectWorkItem(input: DeleteCloudProjectWorkItemInput): Promise<CloudProjectWorkItemDeleteResult>;
  listCloudProjectLabels?(projectId: string): Promise<CloudProjectLabelList>;
  createCloudProjectLabel?(input: CreateCloudProjectLabelInput): Promise<CloudProjectLabelList>;
  patchCloudProjectLabel?(input: PatchCloudProjectLabelInput): Promise<CloudProjectLabelList>;
  deleteCloudProjectLabel?(input: DeleteCloudProjectLabelInput): Promise<CloudProjectLabelList>;
  putCloudProjectWorkItemLabel?(input: MutateCloudProjectWorkItemLabelInput): Promise<CloudProjectWorkItemResult>;
  deleteCloudProjectWorkItemLabel?(input: MutateCloudProjectWorkItemLabelInput): Promise<CloudProjectWorkItemResult>;
  listCloudProjectMembers(projectId: string): Promise<CloudProjectMemberList>;
  joinCloudProject(input: JoinCloudProjectInput): Promise<JoinCloudProjectResult>;
  createCloudProjectInvite(input: CloudProjectInviteCreateInput): Promise<CloudProjectInviteCreateResult>;
  listCloudProjectInvites(projectId: string): Promise<CloudProjectInviteList>;
  revokeCloudProjectInvite(input: CloudProjectInviteMutationInput): Promise<void>;
  deactivateCloudProjectMember(input: CloudProjectMemberMutationInput): Promise<void>;
  acceptCloudProjectInvite(input: AcceptCloudProjectInviteInput): Promise<AcceptCloudProjectInviteResult>;
  deleteCloudProject(input: DeleteCloudProjectInput): Promise<void>;
  listCloudProjectActivities(input: CloudProjectActivityListInput): Promise<CloudProjectActivityPage>;
  listCloudProjectMessages(input: CloudProjectMessageListInput): Promise<CloudProjectActivityPage>;
  postCloudProjectMessage(input: PostCloudProjectMessageInput): Promise<CloudProjectActivity>;
  markCloudProjectActivityRead?(input: MarkCloudProjectActivityReadInput): Promise<void>;
  markCloudProjectMessageRead?(input: MarkCloudProjectMessageReadInput): Promise<void>;
}

export const projectGateway: ProjectGateway = {
  listCloudProjects: (input) => app.ListCloudProjects(input),
  listJoinedCloudProjects: (input) => app.ListJoinedCloudProjects(input),
  claimCloudProjectOnboarding: () => app.ClaimCloudProjectOnboarding(),
  getCloudProject: (projectId) => app.GetCloudProject(projectId),
  createCloudProject: (input) => app.CreateCloudProject(input),
  patchCloudProject: (input) => app.PatchCloudProject(input),
  archiveCloudProject: (input) => app.ArchiveCloudProject(input),
  restoreCloudProject: (input) => app.RestoreCloudProject(input),
  getCloudProjectInstruction: (projectId) => app.GetCloudProjectInstruction(projectId),
  editCloudProjectInstruction: (input) => app.EditCloudProjectInstruction(input),
  clearCloudProjectInstruction: (input) => app.ClearCloudProjectInstruction(input),
  newCloudProjectSession: (input) => app.NewCloudProjectSession(input),
  getCloudProjectFeatures: () => app.GetCloudProjectFeatures(),
  listProjectConnectorAvailability: () => app.ListProjectConnectorAvailability(),
  installProjectConnector: (registryRef) => app.InstallProjectConnector(registryRef),
  listProjectSkillsSettings: () => app.SkillsSettings(),
  listProjectPublicSkills: (limit, cursor) => app.PublicSkills(limit, cursor),
  installProjectPublicSkill: (namespace, slug) => app.InstallPublicSkill(namespace, slug),
	listCloudProjectAssets: (projectId) => app.ListCloudProjectAssets(projectId),
	listCloudProjectAssetProcessingStates: (input) => app.ListCloudProjectAssetProcessingStates(input),
	listCloudProjectAssetDirectory: (input) => app.ListCloudProjectAssetDirectory(input),
	listCloudProjectAssetOperationRecords: (input) => app.ListCloudProjectAssetOperationRecords(input),
	createCloudProjectAssetFolder: (input) => app.CreateCloudProjectAssetFolder(input),
	patchCloudProjectAssetFolder: (input) => app.PatchCloudProjectAssetFolder({ ...input, parentFolderIdSet: Object.prototype.hasOwnProperty.call(input, "parentFolderId") }),
	deleteCloudProjectAssetFolder: (input) => app.DeleteCloudProjectAssetFolder(input),
	patchCloudProjectAsset: (input) => app.PatchCloudProjectAsset({ ...input, folderIdSet: Object.prototype.hasOwnProperty.call(input, "folderId") }),
	replaceCloudProjectAssetContent: (input) => app.ReplaceCloudProjectAssetContent(input),
	uploadCloudProjectAsset: ((projectId: string, ...folder: [string?]) => folder.length === 0
		? app.UploadCloudProjectAsset(projectId)
		: app.UploadCloudProjectAssetToFolder({ projectId, folderId: folder[0] })) as ProjectGateway["uploadCloudProjectAsset"],
	downloadCloudProjectAsset: (projectId, assetId) => app.DownloadCloudProjectAsset(projectId, assetId),
	deleteCloudProjectAsset: (projectId, assetId) => app.DeleteCloudProjectAsset(projectId, assetId),
	putCloudProjectWorkItemAsset: (input) => app.PutCloudProjectWorkItemAsset(input),
	deleteCloudProjectWorkItemAsset: (input) => app.DeleteCloudProjectWorkItemAsset(input),
	listCloudProjectCapabilities: (projectId) => app.ListCloudProjectCapabilities(projectId),
	saveCloudProjectCapabilities: (input) => app.SaveCloudProjectCapabilities(input),
  listCloudProjectWorkItems: (input) => app.ListCloudProjectWorkItems(input),
  createCloudProjectWorkItem: (input) => app.CreateCloudProjectWorkItem(input),
  patchCloudProjectWorkItem: (input) => app.PatchCloudProjectWorkItem(input),
  moveCloudProjectWorkItem: (input) => app.MoveCloudProjectWorkItem(input),
  deleteCloudProjectWorkItem: (input) => app.DeleteCloudProjectWorkItem(input),
  listCloudProjectLabels: (projectId) => app.ListCloudProjectLabels(projectId),
  createCloudProjectLabel: (input) => app.CreateCloudProjectLabel(input),
  patchCloudProjectLabel: (input) => app.PatchCloudProjectLabel(input),
  deleteCloudProjectLabel: (input) => app.DeleteCloudProjectLabel(input),
  putCloudProjectWorkItemLabel: (input) => app.PutCloudProjectWorkItemLabel(input),
  deleteCloudProjectWorkItemLabel: (input) => app.DeleteCloudProjectWorkItemLabel(input),
  listCloudProjectMembers: (projectId) => app.ListCloudProjectMembers(projectId),
  joinCloudProject: (input) => app.JoinCloudProject(input),
  createCloudProjectInvite: (input) => app.CreateCloudProjectInvite(input),
  listCloudProjectInvites: (projectId) => app.ListCloudProjectInvites(projectId),
  revokeCloudProjectInvite: (input) => app.RevokeCloudProjectInvite(input),
  deactivateCloudProjectMember: (input) => app.DeactivateCloudProjectMember(input),
  acceptCloudProjectInvite: (input) => app.AcceptCloudProjectInvite(input),
  deleteCloudProject: (input) => app.DeleteCloudProject(input),
  listCloudProjectActivities: (input) => app.ListCloudProjectActivities(input),
  listCloudProjectMessages: (input) => app.ListCloudProjectMessages(input),
  postCloudProjectMessage: (input) => app.PostCloudProjectMessage(input),
  markCloudProjectActivityRead: (input) => app.MarkCloudProjectActivityRead(input),
  markCloudProjectMessageRead: (input) => app.MarkCloudProjectMessageRead(input),
};
