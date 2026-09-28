import type { DashboardAction, DashboardActionResult, DashboardDocument, WidgetEnvelope, WidgetSurface, WidgetType } from "./contracts";

const knownWidgetTypes = new Set<WidgetType>(["research.summary", "trend.chart", "comparison.bars", "source.coverage", "product.catalog", "chart.bar", "chart.donut", "chart.histogram"]);
const supportedSurfaces: Record<WidgetType, WidgetSurface[]> = {
  "research.summary": ["conversation-inline", "research-canvas"],
  "trend.chart": ["research-canvas"],
  "comparison.bars": ["research-canvas"],
  "source.coverage": ["conversation-inline", "research-canvas"],
  "product.catalog": ["research-canvas"],
  "chart.bar": ["research-canvas"],
  "chart.donut": ["research-canvas"],
  "chart.histogram": ["research-canvas"],
};

function validateWidget(widget: Omit<WidgetEnvelope, "revision"> | WidgetEnvelope): string | null {
  if (!knownWidgetTypes.has(widget.type)) return `Unknown widget type: ${widget.type}`;
  if (!supportedSurfaces[widget.type].includes(widget.surface)) return `${widget.type} does not support ${widget.surface}`;
  if (widget.props.kind !== widget.type) return `Widget props kind does not match ${widget.type}`;
  if (widget.version !== "1.0.0") return `Unsupported widget version: ${widget.version}`;
  return null;
}

function commit(document: DashboardDocument, action: DashboardAction, widgets: WidgetEnvelope[]): DashboardDocument {
  return {
    ...document,
    revision: document.revision + 1,
    updatedAt: action.at,
    widgets,
    appliedOperationIds: [...document.appliedOperationIds, action.operationId].slice(-100),
  };
}

export function applyDashboardAction(document: DashboardDocument, action: DashboardAction): DashboardActionResult {
  if (document.appliedOperationIds.includes(action.operationId)) return { ok: true, document };
  if (action.baseRevision !== document.revision) {
    return { ok: false, error: { code: "REVISION_CONFLICT", message: `Expected revision ${document.revision}, received ${action.baseRevision}` }, document };
  }

  if (action.type === "dashboard.add_widget") {
    const validationError = validateWidget(action.widget);
    if (validationError) return { ok: false, error: { code: "SCHEMA_INVALID", message: validationError }, document };
    if (document.widgets.some((widget) => widget.id === action.widget.id)) {
      return { ok: false, error: { code: "SCHEMA_INVALID", message: `Duplicate widget id: ${action.widget.id}` }, document };
    }
    const widget: WidgetEnvelope = { ...action.widget, revision: 1 };
    return { ok: true, document: commit(document, action, [...document.widgets, widget]), widget };
  }

  const targetIndex = document.widgets.findIndex((widget) => widget.id === action.widgetId);
  if (targetIndex < 0) return { ok: false, error: { code: "WIDGET_NOT_FOUND", message: `Widget not found: ${action.widgetId}` }, document };
  const target = document.widgets[targetIndex];

  if (action.type === "dashboard.remove_widget") {
    return { ok: true, document: commit(document, action, document.widgets.filter((widget) => widget.id !== action.widgetId)) };
  }

  if (action.type === "dashboard.update_widget") {
    const widget: WidgetEnvelope = { ...target, ...action.changes, revision: target.revision + 1 };
    const validationError = validateWidget(widget);
    if (validationError) return { ok: false, error: { code: "SCHEMA_INVALID", message: validationError }, document };
    const widgets = [...document.widgets];
    widgets[targetIndex] = widget;
    return { ok: true, document: commit(document, action, widgets), widget };
  }

  if (action.type === "dashboard.attach_evidence") {
    const widget = { ...target, evidenceRefs: Array.from(new Set([...target.evidenceRefs, ...action.evidenceRefs])), revision: target.revision + 1 };
    const widgets = [...document.widgets];
    widgets[targetIndex] = widget;
    return { ok: true, document: commit(document, action, widgets), widget };
  }

  if (!supportedSurfaces[target.type].includes(action.targetSurface)) {
    return { ok: false, error: { code: "SURFACE_UNSUPPORTED", message: `${target.type} does not support ${action.targetSurface}` }, document };
  }
  const withoutTarget = document.widgets.filter((widget) => widget.id !== action.widgetId);
  const beforeIndex = action.beforeWidgetId ? withoutTarget.findIndex((widget) => widget.id === action.beforeWidgetId) : -1;
  const widget = { ...target, surface: action.targetSurface, revision: target.revision + 1 };
  if (beforeIndex < 0) withoutTarget.push(widget);
  else withoutTarget.splice(beforeIndex, 0, widget);
  return { ok: true, document: commit(document, action, withoutTarget), widget };
}

export interface DashboardStore {
  load(): DashboardDocument | null;
  save(document: DashboardDocument): void;
  clear(): void;
}

export class BrowserLocalDashboardStore implements DashboardStore {
  constructor(private readonly key: string) {}

  load(): DashboardDocument | null {
    if (typeof window === "undefined") return null;
    const value = window.localStorage.getItem(this.key);
    if (!value) return null;
    try {
      const document = JSON.parse(value) as DashboardDocument;
      if (document.schemaVersion !== 1 || !Array.isArray(document.widgets) || typeof document.revision !== "number") return null;
      if (document.widgets.some((widget) => validateWidget(widget))) return null;
      return document;
    } catch {
      return null;
    }
  }

  save(document: DashboardDocument) {
    if (typeof window !== "undefined") window.localStorage.setItem(this.key, JSON.stringify(document));
  }

  clear() {
    if (typeof window !== "undefined") window.localStorage.removeItem(this.key);
  }
}
