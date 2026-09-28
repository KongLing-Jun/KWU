import { AlertTriangle, Check, Database, LockKeyhole, Pin, X } from "lucide-react";
import type { MouseEvent, ReactNode } from "react";
import type { BarChartProps, ComparisonBarsProps, DonutChartProps, HistogramChartProps, ProductCatalogProps, ResearchSummaryProps, SourceCoverageProps, TrendChartProps, WidgetEnvelope, WidgetType } from "../runtime/contracts";

type WidgetRendererProps = {
  widget: WidgetEnvelope;
  onCite?: (widget: WidgetEnvelope) => void;
  selected?: boolean;
  focused?: boolean;
  onPromote?: (widget: WidgetEnvelope) => void;
  promoted?: boolean;
  onOpen?: (widget: WidgetEnvelope) => void;
  onRemove?: (widget: WidgetEnvelope) => void;
};

function MiniTrend({ variant = "primary" }: { variant?: "primary" | "anomaly" }) {
  const path = variant === "anomaly"
    ? "M4 53 C26 55 35 40 50 45 S74 49 91 43 L104 43 L110 8 L117 45 C138 50 151 36 169 42 S199 48 220 34 S246 43 260 40"
    : "M4 42 C23 29 39 54 57 47 S86 24 103 35 S129 45 148 39 S180 51 195 31 S228 27 260 43";
  return (
    <svg className="superpower-mini-trend" viewBox="0 0 264 62" preserveAspectRatio="none" aria-hidden="true">
      <path className="superpower-mini-trend__grid" d="M2 16h260M2 34h260M2 52h260" />
      <path className="superpower-mini-trend__area" d={`${path} L260 60 L4 60Z`} />
      <path className="superpower-mini-trend__line" d={path} />
      {variant === "anomaly" ? <circle cx="110" cy="8" r="3.5" /> : null}
    </svg>
  );
}

function WidgetLoading({ inline = false }: { inline?: boolean }) {
  return (
    <div className={`superpower-widget-loading${inline ? " superpower-widget-loading--inline" : ""}`} role="status" aria-label="组件加载中">
      <i /><i /><i />
    </div>
  );
}

function isResearchSummaryProps(value: unknown): value is ResearchSummaryProps {
  if (!value || typeof value !== "object") return false;
  const props = value as Partial<ResearchSummaryProps>;
  return props.kind === "research.summary"
    && typeof props.eyebrow === "string"
    && typeof props.conclusion === "string"
    && typeof props.confidence === "number"
    && typeof props.summary === "string"
    && Array.isArray(props.metrics)
    && Array.isArray(props.findings);
}

function ResearchSummaryWidget({ widget }: WidgetRendererProps) {
  if (!isResearchSummaryProps(widget.props)) {
    return <section className="superpower-research-summary" data-widget-invalid="true" role="alert">组件数据格式无效，请重新生成。</section>;
  }
  const props = widget.props;
  return (
    <section className="superpower-research-summary">
      {widget.status === "loading" ? <WidgetLoading inline /> : (
        <>
          <div className="superpower-research-summary__header">
            <div><small>{props.eyebrow}</small><h2>{props.conclusion}</h2></div>
            <span className="superpower-confidence"><span />可信度 {props.confidence}%</span>
          </div>
          <p>{props.summary}</p>
          <div className="superpower-summary-metrics">
            {props.metrics.map((metric) => <div key={metric.label}><small>{metric.label}</small><strong>{metric.value}</strong><span data-tone={metric.tone ?? "neutral"}>{metric.detail}</span></div>)}
          </div>
          <ul>{props.findings.map((finding) => <li key={finding.label}><span>{finding.label}</span>{finding.text}</li>)}</ul>
        </>
      )}
    </section>
  );
}

function TrendChartWidget({ widget }: WidgetRendererProps) {
  const props = widget.props as TrendChartProps;
  return (
    <section className="superpower-evidence-card superpower-evidence-card--wide">
      <header><div><small>{props.eyebrow}</small><strong>{props.label}</strong></div><span>{props.range}</span></header>
      {widget.status === "loading" ? <WidgetLoading /> : props.availability ? (
        <div className="superpower-evidence-unavailable"><Database size={18} /><span>{props.availability.label}</span><small>{props.availability.detail}</small></div>
      ) : (
        <>
          <div className="superpower-evidence-card__metric"><strong>{props.value}</strong><span>{props.valueLabel}</span>{props.change ? <em>{props.change}</em> : null}</div>
          <MiniTrend variant={props.variant} />
        </>
      )}
    </section>
  );
}

function ComparisonBarsWidget({ widget }: WidgetRendererProps) {
  const props = widget.props as ComparisonBarsProps;
  return (
    <section className="superpower-evidence-card">
      <header><div><small>{props.eyebrow}</small><strong>{props.label}</strong></div></header>
      {widget.status === "loading" ? <WidgetLoading /> : <>
        <div className="superpower-brand-bars">{props.items.map((item) => <div key={item.label}><span>{item.label}</span><i><b style={{ width: `${item.value}%` }} /></i><strong>{item.value}%</strong></div>)}</div>
        <p className="superpower-widget-footnote">{props.footnote}</p>
      </>}
    </section>
  );
}

const coverageIcons = {
  available: Check,
  restricted: LockKeyhole,
  unavailable: AlertTriangle,
  mock: Database,
};

function SourceCoverageWidget({ widget }: WidgetRendererProps) {
  const props = widget.props as SourceCoverageProps;
  return (
    <section className="superpower-evidence-card superpower-source-coverage">
      <header><div><small>{props.eyebrow}</small><strong>{props.label}</strong></div><span>数据口径</span></header>
      {widget.status === "loading" ? <WidgetLoading /> : <>
        <div className="superpower-source-coverage__list">{props.sources.map((source) => {
          const Icon = coverageIcons[source.status];
          return <div key={source.source} data-coverage-status={source.status}><Icon size={13} /><span><strong>{source.source}</strong><small>{source.role} · {source.detail}</small></span></div>;
        })}</div>
        <div className="superpower-source-coverage__missing"><strong>当前不可得</strong><span>{props.unavailable.join(" · ")}</span></div>
      </>}
    </section>
  );
}

function ProductCatalogWidget({ widget }: WidgetRendererProps) {
  const props = widget.props as ProductCatalogProps;
  return (
    <section className="superpower-evidence-card superpower-product-catalog">
      <header><div><small>{props.eyebrow}</small><strong>{props.label}</strong></div><span>{props.store}</span></header>
      {props.products.length ? <div className="superpower-product-catalog__list">{props.products.map((product) => (
        <a key={product.id} href={product.url} target="_blank" rel="noreferrer">
          {product.imageUrl ? <img src={product.imageUrl} alt="" loading="lazy" /> : <span className="superpower-product-catalog__image"><Database size={16} /></span>}
          <span><strong>{product.title}</strong><small>{product.price} · 可售规格 {product.availableVariants}/{product.totalVariants}</small></span>
        </a>
      ))}</div> : <div className="superpower-evidence-unavailable"><Database size={18} /><span>未找到匹配商品</span><small>可调整关键词后重新查询</small></div>}
      <p className="superpower-widget-footnote">{props.footnote}</p>
    </section>
  );
}

function ChartHeader({ eyebrow, label, unit }: { eyebrow: string; label: string; unit?: string }) {
  return <header><div><small>{eyebrow}</small><strong>{label}</strong></div>{unit ? <span>{unit}</span> : null}</header>;
}

function BarChartWidget({ widget }: WidgetRendererProps) {
  const props = widget.props as BarChartProps;
  const max = Math.max(1, ...props.data.map((item) => item.value));
  return <section className="superpower-evidence-card superpower-derived-chart superpower-derived-chart--wide">
    <ChartHeader eyebrow={props.eyebrow} label={props.label} unit={props.unit} />
    <div className="superpower-bar-chart">{props.data.map((item) => <div key={item.label} title={`${item.label}: ${item.displayValue}`}><strong>{item.displayValue}</strong><i><b style={{ height: `${Math.max(4, item.value / max * 100)}%`, background: item.color }} /></i><span>{item.label}</span></div>)}</div>
    <p className="superpower-widget-footnote">{props.footnote}</p>
  </section>;
}

function HistogramChartWidget({ widget }: WidgetRendererProps) {
  const props = widget.props as HistogramChartProps;
  const max = Math.max(1, ...props.data.map((item) => item.value));
  return <section className="superpower-evidence-card superpower-derived-chart">
    <ChartHeader eyebrow={props.eyebrow} label={props.label} unit={props.unit} />
    <div className="superpower-histogram">{props.data.map((item) => <div key={item.label}><strong>{item.displayValue}</strong><i><b style={{ height: `${Math.max(4, item.value / max * 100)}%`, background: item.color }} /></i><span>{item.label}</span></div>)}</div>
    <p className="superpower-widget-footnote">{props.footnote}</p>
  </section>;
}

function DonutChartWidget({ widget }: WidgetRendererProps) {
  const props = widget.props as DonutChartProps;
  const total = Math.max(1, props.data.reduce((sum, item) => sum + item.value, 0));
  let cursor = 0;
  const gradient = props.data.map((item) => {
    const start = cursor;
    cursor += item.value / total * 100;
    return `${item.color ?? "#6f86df"} ${start}% ${cursor}%`;
  }).join(", ");
  return <section className="superpower-evidence-card superpower-derived-chart">
    <ChartHeader eyebrow={props.eyebrow} label={props.label} />
    <div className="superpower-donut-chart"><div className="superpower-donut-chart__plot" style={{ background: `conic-gradient(${gradient})` }}><span>{props.centerLabel}</span></div><div className="superpower-donut-chart__legend">{props.data.map((item) => <div key={item.label}><i style={{ background: item.color }} /><span>{item.label}</span><strong>{item.displayValue}</strong></div>)}</div></div>
    <p className="superpower-widget-footnote">{props.footnote}</p>
  </section>;
}

const widgetRegistry: Record<WidgetType, (props: WidgetRendererProps) => ReactNode> = {
  "research.summary": ResearchSummaryWidget,
  "trend.chart": TrendChartWidget,
  "comparison.bars": ComparisonBarsWidget,
  "source.coverage": SourceCoverageWidget,
  "product.catalog": ProductCatalogWidget,
  "chart.bar": BarChartWidget,
  "chart.donut": DonutChartWidget,
  "chart.histogram": HistogramChartWidget,
};

export function WidgetRenderer({ widget, onCite, selected = false, focused = false, onPromote, promoted = false, onOpen, onRemove }: WidgetRendererProps) {
  const Renderer = widgetRegistry[widget.type];
  const selectable = Boolean(onCite) && widget.status !== "loading";
  const selectFromContextMenu = (event: MouseEvent<HTMLDivElement>) => {
    if (!selectable || !onCite) return;
    event.preventDefault();
    onCite(widget);
  };
  const openDetails = (event: MouseEvent<HTMLDivElement>) => {
    if (!onOpen || (event.target as HTMLElement).closest("a, button, input, textarea, select")) return;
    onOpen(widget);
  };
  return (
    <div
      className="superpower-generated-widget"
      data-generated-widget={widget.type}
      data-widget-id={widget.id}
      data-widget-version={widget.version}
      data-widget-surface={widget.surface}
      data-widget-status={widget.status}
      data-citation-selectable={selectable}
      data-citation-selected={selected}
      data-source-focused={focused}
      data-citation-hint={selected ? "已引用 · 右键取消" : "右键引用"}
      onContextMenu={selectFromContextMenu}
      onClick={openDetails}
      tabIndex={-1}
    >
      <Renderer widget={widget} onCite={onCite} />
      {onRemove ? <button
        type="button"
        className="superpower-widget-remove"
        aria-label={`关闭卡片：${widget.title}`}
        title="关闭卡片"
        onClick={(event) => {
          event.stopPropagation();
          onRemove(widget);
        }}
      ><X size={12} /></button> : null}
      {onPromote ? <button
        type="button"
        className="superpower-widget-promote"
        aria-label={promoted ? `${widget.title} 已加入聚合看板` : `将 ${widget.title} 加入聚合看板`}
        title={promoted ? "已加入聚合看板" : "加入聚合看板"}
        disabled={promoted}
        onClick={(event) => {
          event.stopPropagation();
          onPromote(widget);
        }}
      ><Pin size={11} /><span>{promoted ? "已聚合" : "加入聚合"}</span></button> : null}
    </div>
  );
}
