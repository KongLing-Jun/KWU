import { app } from "../../../lib/bridge";
import type { CapabilityTaskScope as HostCapabilityTaskScope } from "../../../lib/types";
import { getWailsRuntime } from "../../../platform/wails/wailsRuntime";
import type { DashboardAction, DashboardDocument } from "./contracts";
import { DASHBOARD_SCOPE } from "./contracts";
import type { EvidenceDocument } from "./researchExecution";

export const CAPABILITY_ID = "crossborder-commerce";

export type CapabilityTaskScope = HostCapabilityTaskScope & { capabilityId: typeof CAPABILITY_ID };

const requestedScope: CapabilityTaskScope = {
  capabilityId: CAPABILITY_ID,
  ...DASHBOARD_SCOPE,
};

function desktopMethods() {
  if (typeof window === "undefined") return null;
  const bound = window.go?.main?.App;
  if (
    typeof bound?.EnsureCapabilityTaskBinding !== "function" ||
    typeof bound.EnsureCapabilityRuntime !== "function" ||
    typeof bound.EnsureCapabilityDashboard !== "function" ||
    typeof bound.LoadCapabilityDashboard !== "function" ||
    typeof bound.ApplyCapabilityDashboardAction !== "function"
  ) return null;
  return bound;
}

export function hasDesktopDashboardBridge(): boolean {
  return desktopMethods() !== null;
}

function parseDocument(value: string): DashboardDocument {
  const document = JSON.parse(value) as DashboardDocument;
  if (document.schemaVersion !== 1 || !Array.isArray(document.widgets) || typeof document.revision !== "number") {
    throw new Error("Desktop returned an invalid DashboardDocument");
  }
  return document;
}

function parseEvidenceDocument(value: string): EvidenceDocument {
  const document = JSON.parse(value) as EvidenceDocument;
  if (document.schemaVersion !== 1 || !Array.isArray(document.artifacts) || typeof document.revision !== "number") {
    throw new Error("Desktop returned an invalid EvidenceDocument");
  }
  return document;
}

function seedForScope(seed: DashboardDocument, scope: CapabilityTaskScope): DashboardDocument {
  return {
    ...seed,
    documentId: scope.documentId,
    workspaceId: scope.workspaceId,
    taskId: scope.taskId,
  };
}

export async function ensureDesktopDashboards(seed: DashboardDocument): Promise<{
  scopes: { session: CapabilityTaskScope; aggregate: CapabilityTaskScope };
  documents: { session: DashboardDocument; aggregate: DashboardDocument };
}> {
  if (!desktopMethods()) throw new Error("Desktop dashboard bridge is unavailable");
  const sessionScope = await app.EnsureCapabilityTaskBinding(requestedScope) as CapabilityTaskScope;
  const aggregateScope = requestedScope;
  const [sessionValue, aggregateValue] = await Promise.all([
    app.EnsureCapabilityDashboard(sessionScope, JSON.stringify(seedForScope(seed, sessionScope))),
    app.EnsureCapabilityDashboard(aggregateScope, JSON.stringify(seedForScope(seed, aggregateScope))),
  ]);
  // Dashboard history is a local sidecar and must remain available even when a
  // remote MCP runtime cannot be prepared. Runtime readiness is best-effort;
  // the next research turn can surface connector errors independently.
  void app.EnsureCapabilityRuntime(sessionScope.capabilityId).catch(() => undefined);
  return {
    scopes: { session: sessionScope, aggregate: aggregateScope },
    documents: { session: parseDocument(sessionValue), aggregate: parseDocument(aggregateValue) },
  };
}

export async function applyDesktopDashboardAction(scope: CapabilityTaskScope, action: DashboardAction): Promise<DashboardDocument> {
  if (!desktopMethods()) throw new Error("Desktop dashboard bridge is unavailable");
  return parseDocument(await app.ApplyCapabilityDashboardAction(scope, JSON.stringify(action)));
}

export async function ensureDesktopEvidence(scope: CapabilityTaskScope, seed: EvidenceDocument): Promise<EvidenceDocument> {
  if (!desktopMethods() || typeof window.go?.main?.App?.EnsureCapabilityEvidence !== "function") throw new Error("Desktop evidence bridge is unavailable");
  return parseEvidenceDocument(await app.EnsureCapabilityEvidence(scope, JSON.stringify(seed)));
}

export async function loadDesktopEvidence(scope: CapabilityTaskScope): Promise<EvidenceDocument> {
  if (!desktopMethods() || typeof window.go?.main?.App?.LoadCapabilityEvidence !== "function") throw new Error("Desktop evidence bridge is unavailable");
  return parseEvidenceDocument(await app.LoadCapabilityEvidence(scope));
}

export async function loadDesktopDashboard(scope: CapabilityTaskScope): Promise<DashboardDocument> {
  if (!desktopMethods()) throw new Error("Desktop dashboard bridge is unavailable");
  return parseDocument(await app.LoadCapabilityDashboard(scope));
}

export async function runEbayBrowseResearch(scope: CapabilityTaskScope, query: string): Promise<{ document: DashboardDocument; evidence: EvidenceDocument; capturedAt: string }> {
  if (typeof window.go?.main?.App?.RunCapabilityEbayBrowseResearch !== "function") throw new Error("Desktop eBay data adapter is unavailable");
  const result = await app.RunCapabilityEbayBrowseResearch(scope, { query, marketplace: "EBAY_US", limit: 30 });
  return {
    document: parseDocument(result.dashboardJson),
    evidence: parseEvidenceDocument(result.evidenceJson),
    capturedAt: result.capturedAt,
  };
}

export function subscribeDesktopDashboard(scope: CapabilityTaskScope, onDocument: (document: DashboardDocument) => void, onEvidence?: (document: EvidenceDocument) => void): () => void {
  const runtime = getWailsRuntime();
  if (!runtime) return () => {};
  return runtime.EventsOn("capability:dashboard-changed", (payload) => {
    if (!payload || typeof payload !== "object") return;
    const changed = payload as Partial<CapabilityTaskScope>;
    if (
      changed.capabilityId !== scope.capabilityId ||
      changed.workspaceId !== scope.workspaceId ||
      changed.taskId !== scope.taskId ||
      changed.documentId !== scope.documentId
    ) return;
    void loadDesktopDashboard(scope).then(onDocument).catch(() => undefined);
    if (onEvidence) void loadDesktopEvidence(scope).then(onEvidence).catch(() => undefined);
  });
}
