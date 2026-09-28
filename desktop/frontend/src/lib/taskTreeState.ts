import type { EventKind, WireEvent, WireTaskTree, WireTaskTreeNode } from "./types";

export interface TaskTreeNodeView {
  nodeId: string;
  title: string;
  status: string;
  parentId?: string;
  dependsOn: string[];
  childIds: string[];
  output: string;
  outputDeltas: string[];
  lastOutputDelta?: string;
  error?: string;
  result?: string;
  goal?: string;
  expertId?: string;
  expertName?: string;
  expertRole?: string;
  contextExpectation?: string;
  outputExpectation?: string;
  completionPercent?: number;
  confidencePercent?: number;
  toolIds: string[];
  updatedAt: number;
}

export interface TaskTreeView {
  treeId: string;
  title: string;
  status: string;
  rootIds: string[];
  nodes: Record<string, TaskTreeNodeView>;
  currentNodeIds: string[];
  summary?: string;
  result?: string;
  updatedAt: number;
}

export interface TaskTreeState {
  trees: Record<string, TaskTreeView>;
  activeTreeId?: string;
  seq: number;
}

export interface TaskTreeDisplayRow {
  treeId: string;
  nodeId: string;
  depth: number;
  title: string;
  status: string;
  running: boolean;
  output: string;
  lastOutputDelta?: string;
  error?: string;
  result?: string;
  goal?: string;
  expertId?: string;
  expertName?: string;
  expertRole?: string;
  contextExpectation?: string;
  outputExpectation?: string;
  completionPercent?: number;
  confidencePercent?: number;
  toolIds: string[];
}

export interface TaskTreeDisplayModel {
  treeId: string;
  title: string;
  status: string;
  currentNodeIds: string[];
  rows: TaskTreeDisplayRow[];
  summary?: string;
  result?: string;
}

const TASK_TREE_EVENT_KINDS = new Set<EventKind>([
  "task_tree_created",
  "task_node_updated",
  "task_node_started",
  "task_node_output",
  "task_node_completed",
  "task_node_failed",
  "task_tree_completed",
  "task_tree_summary",
]);

export const initialTaskTreeState: TaskTreeState = {
  trees: {},
  seq: 0,
};

export function isTaskTreePayloadEvent(event: Pick<WireEvent, "kind">): boolean {
  return TASK_TREE_EVENT_KINDS.has(event.kind);
}

export function applyTaskTreeEvent(state: TaskTreeState, event: WireEvent): TaskTreeState {
  if (isTaskTreePayloadEvent(event)) {
    return applyTaskTreePayloadEvent(state, event.kind, event.taskTree);
  }
  if (event.kind === "tool_dispatch" || event.kind === "tool_progress" || event.kind === "tool_result") {
    return applyTaskTreeToolEvent(state, event);
  }
  return state;
}

export function taskTreeDisplayModel(tree: TaskTreeView): TaskTreeDisplayModel {
  return {
    treeId: tree.treeId,
    title: tree.title,
    status: tree.status,
    currentNodeIds: tree.currentNodeIds,
    rows: taskTreeDisplayRows(tree),
    summary: tree.summary,
    result: tree.result,
  };
}

export function taskTreeDisplayRows(tree: TaskTreeView): TaskTreeDisplayRow[] {
  const rows: TaskTreeDisplayRow[] = [];
  const visited = new Set<string>();
  const visit = (nodeId: string, depth: number) => {
    if (visited.has(nodeId)) return;
    const node = tree.nodes[nodeId];
    if (!node) return;
    visited.add(nodeId);
    rows.push({
      treeId: tree.treeId,
      nodeId,
      depth,
      title: node.title,
      status: node.status,
      running: isRunningStatus(node.status),
      output: node.output,
      lastOutputDelta: node.lastOutputDelta,
      error: node.error,
      result: node.result,
      goal: node.goal,
      expertId: node.expertId,
      expertName: node.expertName,
      expertRole: node.expertRole,
      contextExpectation: node.contextExpectation,
      outputExpectation: node.outputExpectation,
      completionPercent: node.completionPercent,
      confidencePercent: node.confidencePercent,
      toolIds: node.toolIds,
    });
    for (const childId of node.childIds) visit(childId, depth + 1);
  };
  for (const rootId of tree.rootIds) visit(rootId, 0);
  for (const nodeId of Object.keys(tree.nodes)) visit(nodeId, 0);
  return rows;
}

function applyTaskTreePayloadEvent(state: TaskTreeState, kind: EventKind, payload?: WireTaskTree): TaskTreeState {
  if (!payload?.treeId) return state;
  const seq = state.seq + 1;
  let tree = cloneTree(state.trees[payload.treeId] ?? createTree(payload.treeId, seq), seq);
  switch (kind) {
    case "task_tree_created":
      tree = applyTreeCreated(tree, payload, seq);
      break;
    case "task_node_updated":
      tree = applyNodePayload(tree, payload, seq);
      break;
    case "task_node_started":
      tree = applyNodePayload(tree, { ...payload, status: payload.status || "running" }, seq);
      break;
    case "task_node_output":
      tree = applyNodeOutput(tree, payload, seq);
      break;
    case "task_node_completed":
      tree = applyNodePayload(tree, { ...payload, status: payload.status || "completed" }, seq);
      break;
    case "task_node_failed":
      tree = applyNodePayload(tree, { ...payload, status: payload.status || "failed" }, seq);
      break;
    case "task_tree_completed":
      tree = {
        ...tree,
        status: payload.status || "completed",
        summary: payload.summary ?? tree.summary,
        result: payload.result ?? tree.result,
        updatedAt: seq,
      };
      break;
    case "task_tree_summary":
      tree = {
        ...tree,
        status: payload.status ?? tree.status,
        summary: payload.summary ?? payload.result ?? tree.summary,
        result: payload.result ?? tree.result,
        updatedAt: seq,
      };
      break;
    default:
      return state;
  }
  tree = reindexTree(tree, payload.rootIds);
  return {
    ...state,
    seq,
    activeTreeId: payload.treeId,
    trees: { ...state.trees, [payload.treeId]: tree },
  };
}

function applyTaskTreeToolEvent(state: TaskTreeState, event: WireEvent): TaskTreeState {
  const tool = event.tool;
  if (!tool?.taskTreeId || !tool.nodeId || !tool.id) return state;
  const seq = state.seq + 1;
  let tree = cloneTree(state.trees[tool.taskTreeId] ?? createTree(tool.taskTreeId, seq), seq);
  const node = tree.nodes[tool.nodeId] ?? createNode(tool.nodeId, seq);
  const toolIds = node.toolIds.includes(tool.id) ? node.toolIds : [...node.toolIds, tool.id];
  tree.nodes[tool.nodeId] = { ...node, toolIds, updatedAt: seq };
  tree = reindexTree(tree);
  return {
    ...state,
    seq,
    activeTreeId: tool.taskTreeId,
    trees: { ...state.trees, [tool.taskTreeId]: tree },
  };
}

function applyTreeCreated(tree: TaskTreeView, payload: WireTaskTree, seq: number): TaskTreeView {
  const nodes = { ...tree.nodes };
  for (const wireNode of payload.nodes ?? []) {
    const node = nodes[wireNode.nodeId] ?? createNode(wireNode.nodeId, seq);
    nodes[wireNode.nodeId] = applyNodeFields(node, wireNode, seq);
  }
  if (payload.nodeId) {
    const rootNode = nodes[payload.nodeId] ?? createNode(payload.nodeId, seq);
    nodes[payload.nodeId] = applyNodeFields(rootNode, payload, seq);
  }
  return {
    ...tree,
    title: payload.title ?? tree.title,
    status: payload.status ?? tree.status,
    summary: payload.summary ?? tree.summary,
    result: payload.result ?? tree.result,
    nodes,
    updatedAt: seq,
  };
}

function applyNodePayload(tree: TaskTreeView, payload: WireTaskTree, seq: number): TaskTreeView {
  if (!payload.nodeId) {
    return {
      ...tree,
      title: payload.title ?? tree.title,
      status: payload.status ?? tree.status,
      summary: payload.summary ?? tree.summary,
      result: payload.result ?? tree.result,
      updatedAt: seq,
    };
  }
  const node = tree.nodes[payload.nodeId] ?? createNode(payload.nodeId, seq);
  const nextNode = applyNodeFields(node, payload, seq);
  return {
    ...tree,
    title: payload.title && !tree.title ? payload.title : tree.title,
    nodes: { ...tree.nodes, [payload.nodeId]: nextNode },
    updatedAt: seq,
  };
}

function applyNodeOutput(tree: TaskTreeView, payload: WireTaskTree, seq: number): TaskTreeView {
  if (!payload.nodeId) return tree;
  const node = tree.nodes[payload.nodeId] ?? createNode(payload.nodeId, seq);
  const outputDelta = payload.outputDelta ?? "";
  const nextNode = applyNodeFields(node, payload, seq);
  const withOutput = outputDelta
    ? {
        ...nextNode,
        output: nextNode.output + outputDelta,
        outputDeltas: [...nextNode.outputDeltas, outputDelta],
        lastOutputDelta: outputDelta,
      }
    : nextNode;
  return {
    ...tree,
    nodes: { ...tree.nodes, [payload.nodeId]: withOutput },
    updatedAt: seq,
  };
}

function applyNodeFields(node: TaskTreeNodeView, payload: WireTaskTreeNode | WireTaskTree, seq: number): TaskTreeNodeView {
  return {
    ...node,
    title: payload.title ?? node.title,
    status: payload.status ?? node.status,
    parentId: payload.parentId ?? node.parentId,
    dependsOn: payload.dependsOn ? [...payload.dependsOn] : node.dependsOn,
    error: "error" in payload ? payload.error ?? node.error : node.error,
    result: "result" in payload ? payload.result ?? node.result : node.result,
    goal: payload.goal ?? node.goal,
    expertId: payload.expertId ?? node.expertId,
    expertName: payload.expertName ?? node.expertName,
    expertRole: payload.expertRole ?? node.expertRole,
    contextExpectation: payload.contextExpectation ?? node.contextExpectation,
    outputExpectation: payload.outputExpectation ?? node.outputExpectation,
    completionPercent: payload.completionPercent ?? node.completionPercent,
    confidencePercent: payload.confidencePercent ?? node.confidencePercent,
    updatedAt: seq,
  };
}

function reindexTree(tree: TaskTreeView, explicitRootIds?: string[]): TaskTreeView {
  const nodes: Record<string, TaskTreeNodeView> = {};
  for (const [nodeId, node] of Object.entries(tree.nodes)) {
    nodes[nodeId] = { ...node, childIds: [] };
  }
  for (const node of Object.values(nodes)) {
    if (node.parentId && nodes[node.parentId]) {
      nodes[node.parentId] = { ...nodes[node.parentId], childIds: [...nodes[node.parentId].childIds, node.nodeId] };
    }
  }
  const preferredRoots = explicitRootIds ?? tree.rootIds;
  const rootIds = unique([
    ...preferredRoots.filter((nodeId) => Boolean(nodes[nodeId]) && !nodes[nodeId].parentId),
    ...Object.values(nodes)
      .filter((node) => !node.parentId || !nodes[node.parentId])
      .map((node) => node.nodeId),
  ]);
  return {
    ...tree,
    nodes,
    rootIds,
    currentNodeIds: Object.values(nodes)
      .filter((node) => isRunningStatus(node.status))
      .map((node) => node.nodeId),
  };
}

function cloneTree(tree: TaskTreeView, seq: number): TaskTreeView {
  const nodes: Record<string, TaskTreeNodeView> = {};
  for (const [nodeId, node] of Object.entries(tree.nodes)) {
    nodes[nodeId] = {
      ...node,
      dependsOn: [...node.dependsOn],
      childIds: [...node.childIds],
      outputDeltas: [...node.outputDeltas],
      toolIds: [...node.toolIds],
    };
  }
  return { ...tree, nodes, updatedAt: seq };
}

function createTree(treeId: string, seq: number): TaskTreeView {
  return {
    treeId,
    title: treeId,
    status: "created",
    rootIds: [],
    nodes: {},
    currentNodeIds: [],
    updatedAt: seq,
  };
}

function createNode(nodeId: string, seq: number): TaskTreeNodeView {
  return {
    nodeId,
    title: nodeId,
    status: "pending",
    dependsOn: [],
    childIds: [],
    output: "",
    outputDeltas: [],
    toolIds: [],
    updatedAt: seq,
  };
}

function isRunningStatus(status: string): boolean {
  return status === "running" || status === "started" || status === "in_progress";
}

function unique(values: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const value of values) {
    if (seen.has(value)) continue;
    seen.add(value);
    out.push(value);
  }
  return out;
}
