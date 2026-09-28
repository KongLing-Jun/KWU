import { taskTreeDisplayModel, type TaskTreeState } from "./taskTreeState";
import type { Item, ToolStatus } from "./useController";

type ToolItem = Extract<Item, { kind: "tool" }>;

export type ExpertTeamStage = "planning" | "executing" | "completed" | "blocked";

export interface ExpertTeamActiveSelection {
  id: string;
  name: string;
  role?: string;
  mode?: "expert" | "team" | string;
  selectedAt?: number;
}

export interface ExpertTeamToolActivity {
  id: string;
  name: string;
  status: ToolStatus;
  summary?: string;
  subject?: string;
  parentId?: string;
  model?: string;
  effort?: string;
  delegated: boolean;
}

export interface ExpertTeamTaskVerification {
  completionPercent: number;
  confidencePercent?: number;
  confidenceSource: "backend" | "not_available";
}

export interface ExpertTeamTaskNode {
  treeId: string;
  nodeId: string;
  depth: number;
  title: string;
  status: string;
  running: boolean;
  active: boolean;
  goal: string;
  expertName?: string;
  expertRole?: string;
  contextExpectation?: string;
  outputExpectation?: string;
  output: string;
  result?: string;
  error?: string;
  tools: ExpertTeamToolActivity[];
  verification: ExpertTeamTaskVerification;
}

export interface ExpertTeamFlowModel {
  treeId: string;
  title: string;
  status: string;
  stage: ExpertTeamStage;
  teamName: string;
  owner?: string;
  activeNodeIds: string[];
  progressPercent: number;
  nodes: ExpertTeamTaskNode[];
  summary?: string;
  result?: string;
}

export interface BuildExpertTeamFlowInput {
  taskTrees: TaskTreeState;
  items: Item[];
  activeExpert?: ExpertTeamActiveSelection | null;
}

const DELEGATION_TOOLS = new Set(["task", "read_only_task", "parallel_tasks", "run_skill", "explore", "research", "review", "security_review"]);

export function buildExpertTeamFlowModel(input: BuildExpertTeamFlowInput): ExpertTeamFlowModel | null {
  const tree = selectTree(input.taskTrees);
  if (!tree) return null;

  const display = taskTreeDisplayModel(tree);
  const toolsByNode = groupToolsByNode(input.items, display.treeId);
  const activeNodeIds = new Set(display.currentNodeIds);
  const nodes = display.rows.map((row): ExpertTeamTaskNode => {
    const tools = toolsByNode.get(row.nodeId) ?? [];
    const delegate = tools.find((tool) => tool.delegated);
    return {
      treeId: row.treeId,
      nodeId: row.nodeId,
      depth: row.depth,
      title: row.title,
      status: row.status,
      running: row.running,
      active: activeNodeIds.has(row.nodeId),
      goal: row.goal ?? row.title,
      expertName: row.expertName ?? delegate?.subject ?? input.activeExpert?.name,
      expertRole: row.expertRole ?? input.activeExpert?.role,
      contextExpectation: row.contextExpectation,
      outputExpectation: row.outputExpectation,
      output: row.output,
      result: row.result,
      error: row.error,
      tools,
      verification: verificationFor(row.status, row.completionPercent, row.confidencePercent),
    };
  });

  return {
    treeId: display.treeId,
    title: display.title,
    status: display.status,
    stage: stageFor(display.status, nodes),
    teamName: teamNameFor(input.activeExpert),
    owner: input.activeExpert?.name,
    activeNodeIds: display.currentNodeIds,
    progressPercent: progressFor(nodes),
    nodes,
    summary: display.summary,
    result: display.result,
  };
}

export function isExpertDelegationTool(name: string): boolean {
  return DELEGATION_TOOLS.has(name);
}

function selectTree(taskTrees: TaskTreeState) {
  if (taskTrees.activeTreeId && taskTrees.trees[taskTrees.activeTreeId]) {
    return taskTrees.trees[taskTrees.activeTreeId];
  }
  return Object.values(taskTrees.trees).sort((a, b) => b.updatedAt - a.updatedAt)[0] ?? null;
}

function groupToolsByNode(items: Item[], treeId: string): Map<string, ExpertTeamToolActivity[]> {
  const grouped = new Map<string, ExpertTeamToolActivity[]>();
  for (const item of items) {
    if (item.kind !== "tool" || item.taskTreeId !== treeId || !item.nodeId) continue;
    const activity = toolActivity(item);
    const list = grouped.get(item.nodeId) ?? [];
    grouped.set(item.nodeId, [...list, activity]);
  }
  return grouped;
}

function toolActivity(tool: ToolItem): ExpertTeamToolActivity {
  return {
    id: tool.id,
    name: tool.name,
    status: tool.status,
    summary: tool.summary,
    subject: tool.subject,
    parentId: tool.parentId,
    model: tool.profile?.model,
    effort: tool.profile?.effort,
    delegated: isExpertDelegationTool(tool.name),
  };
}

function teamNameFor(activeExpert?: ExpertTeamActiveSelection | null): string {
  if (!activeExpert) return "AI 专家团";
  if (activeExpert.mode === "team") return activeExpert.name;
  return `${activeExpert.name} team`;
}

function stageFor(status: string, nodes: ExpertTeamTaskNode[]): ExpertTeamStage {
  if (isFailedStatus(status) || nodes.some((node) => isFailedStatus(node.status))) return "blocked";
  if (isCompletedStatus(status)) return "completed";
  if (nodes.some((node) => node.running)) return "executing";
  return "planning";
}

function progressFor(nodes: ExpertTeamTaskNode[]): number {
  if (nodes.length === 0) return 0;
  const total = nodes.reduce((sum, node) => sum + node.verification.completionPercent, 0);
  return Math.round(total / nodes.length);
}

function verificationFor(status: string, completionPercent?: number, confidencePercent?: number): ExpertTeamTaskVerification {
  return {
    completionPercent: normalizePercent(completionPercent ?? completionForStatus(status)),
    confidencePercent: confidencePercent == null ? undefined : normalizePercent(confidencePercent),
    confidenceSource: confidencePercent == null ? "not_available" : "backend",
  };
}

function completionForStatus(status: string): number {
  if (isCompletedStatus(status)) return 100;
  if (isFailedStatus(status)) return 100;
  if (status === "running" || status === "started" || status === "in_progress") return 50;
  return 0;
}

function normalizePercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, Math.round(value)));
}

function isCompletedStatus(status: string): boolean {
  return status === "completed" || status === "done" || status === "succeeded";
}

function isFailedStatus(status: string): boolean {
  return status === "failed" || status === "error" || status === "blocked" || status === "stopped";
}
