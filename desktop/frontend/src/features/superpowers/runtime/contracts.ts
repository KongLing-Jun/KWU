export type WidgetSurface = "conversation-inline" | "research-canvas";
export type WidgetStatus = "queued" | "loading" | "partial" | "ready" | "stale" | "error";
export type WidgetType = "research.summary" | "trend.chart" | "comparison.bars" | "source.coverage" | "product.catalog" | "chart.bar" | "chart.donut" | "chart.histogram";

export type ResearchMetric = {
  label: string;
  value: string;
  detail: string;
  tone?: "positive" | "neutral" | "warning";
};

export type ResearchSummaryProps = {
  kind: "research.summary";
  eyebrow: string;
  conclusion: string;
  confidence: number;
  summary: string;
  metrics: ResearchMetric[];
  findings: Array<{ label: string; text: string }>;
};

export type TrendChartProps = {
  kind: "trend.chart";
  eyebrow: string;
  label: string;
  range: string;
  value: string;
  valueLabel: string;
  change?: string;
  variant?: "primary" | "anomaly";
  availability?: { label: string; detail: string };
};

export type ComparisonBarsProps = {
  kind: "comparison.bars";
  eyebrow: string;
  label: string;
  items: Array<{ label: string; value: number }>;
  footnote: string;
};

export type SourceCoverageProps = {
  kind: "source.coverage";
  eyebrow: string;
  label: string;
  sources: Array<{
    source: string;
    role: string;
    status: "available" | "restricted" | "unavailable" | "mock";
    detail: string;
  }>;
  unavailable: string[];
};

export type ProductCatalogProps = {
  kind: "product.catalog";
  eyebrow: string;
  label: string;
  store: string;
  products: Array<{
    id: string;
    title: string;
    url: string;
    imageUrl?: string;
    price: string;
    availableVariants: number;
    totalVariants: number;
  }>;
  footnote: string;
};

export type ChartDatum = {
  label: string;
  value: number;
  displayValue: string;
  color?: string;
};

export type BarChartProps = {
  kind: "chart.bar";
  eyebrow: string;
  label: string;
  unit: string;
  data: ChartDatum[];
  footnote: string;
};

export type DonutChartProps = {
  kind: "chart.donut";
  eyebrow: string;
  label: string;
  centerLabel: string;
  data: ChartDatum[];
  footnote: string;
};

export type HistogramChartProps = {
  kind: "chart.histogram";
  eyebrow: string;
  label: string;
  unit: string;
  data: ChartDatum[];
  footnote: string;
};

export type WidgetProps = ResearchSummaryProps | TrendChartProps | ComparisonBarsProps | SourceCoverageProps | ProductCatalogProps | BarChartProps | DonutChartProps | HistogramChartProps;

export type WidgetEnvelope = {
  id: string;
  type: WidgetType;
  version: "1.0.0";
  surface: WidgetSurface;
  status: WidgetStatus;
  title: string;
  props: WidgetProps;
  evidenceRefs: string[];
  revision: number;
};

export type DashboardDocument = {
  schemaVersion: 1;
  documentId: string;
  workspaceId: string;
  taskId: string;
  revision: number;
  updatedAt: string;
  widgets: WidgetEnvelope[];
  appliedOperationIds: string[];
};

type DashboardActionBase = {
  operationId: string;
  baseRevision: number;
  at: string;
};

export type DashboardAction =
  | (DashboardActionBase & { type: "dashboard.add_widget"; widget: Omit<WidgetEnvelope, "revision"> })
  | (DashboardActionBase & { type: "dashboard.update_widget"; widgetId: string; changes: Partial<Pick<WidgetEnvelope, "title" | "status" | "props">> })
  | (DashboardActionBase & { type: "dashboard.remove_widget"; widgetId: string })
  | (DashboardActionBase & { type: "dashboard.move_widget"; widgetId: string; targetSurface: WidgetSurface; beforeWidgetId?: string })
  | (DashboardActionBase & { type: "dashboard.attach_evidence"; widgetId: string; evidenceRefs: string[] });

export type DashboardActionErrorCode = "REVISION_CONFLICT" | "WIDGET_TYPE_UNKNOWN" | "WIDGET_NOT_FOUND" | "SURFACE_UNSUPPORTED" | "SCHEMA_INVALID";

export type DashboardActionResult =
  | { ok: true; document: DashboardDocument; widget?: WidgetEnvelope }
  | { ok: false; error: { code: DashboardActionErrorCode; message: string }; document: DashboardDocument };

export const DASHBOARD_SCOPE = {
  workspaceId: "workspace-commerce-research",
  taskId: "task-portable-blender-opportunity",
  documentId: "primary",
} as const;
