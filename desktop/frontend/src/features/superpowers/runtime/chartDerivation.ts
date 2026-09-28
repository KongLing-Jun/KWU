import type { DashboardDocument, ProductCatalogProps, WidgetEnvelope } from "./contracts";

export type DerivedChartKind = "bar" | "histogram" | "donut";

type CatalogProduct = ProductCatalogProps["products"][number] & { sourceTitle: string; evidenceRefs: string[] };

const palette = ["#6f86df", "#9b72d4", "#52a888", "#df9364", "#d16f8e", "#7aa8c9"];

function priceValue(price: string) {
  const values = Array.from(price.matchAll(/\d[\d,]*(?:\.\d+)?/g), (match) => Number(match[0].replaceAll(",", ""))).filter(Number.isFinite);
  if (!values.length) return null;
  return values.length > 1 ? (values[0] + values[1]) / 2 : values[0];
}

function merchantName(url: string) {
  try {
    return new URL(url).hostname.replace(/^www\./, "") || "未知商家";
  } catch {
    return "未知商家";
  }
}

function catalogProducts(document: DashboardDocument) {
  const products = new Map<string, CatalogProduct>();
  document.widgets.filter((widget) => widget.type === "product.catalog").forEach((widget) => {
    const props = widget.props as ProductCatalogProps;
    props.products.forEach((product) => products.set(product.url || product.id, {
      ...product,
      sourceTitle: widget.title,
      evidenceRefs: widget.evidenceRefs,
    }));
  });
  return [...products.values()];
}

function compactLabel(value: string, max = 18) {
  return value.length > max ? `${value.slice(0, max - 1)}…` : value;
}

export function chartKindsFromPrompt(text: string): DerivedChartKind[] {
  const kinds: DerivedChartKind[] = [];
  if (/柱状|条形|价格对比|价格比较/.test(text)) kinds.push("bar");
  if (/直方|区间|价格分布/.test(text)) kinds.push("histogram");
  if (/饼图|环形|商家分布|来源分布/.test(text)) kinds.push("donut");
  return kinds;
}

export function deriveChartWidget(document: DashboardDocument, kind: DerivedChartKind): Omit<WidgetEnvelope, "revision"> | null {
  const products = catalogProducts(document);
  if (!products.length) return null;
  const evidenceRefs = [...new Set(products.flatMap((product) => product.evidenceRefs))];
  const suffix = `r${document.revision}`;

  if (kind === "bar") {
    const priced = products.map((product) => ({ product, value: priceValue(product.price) })).filter((item): item is { product: CatalogProduct; value: number } => item.value !== null).sort((a, b) => b.value - a.value).slice(0, 10);
    if (!priced.length) return null;
    return {
      id: `derived-price-comparison-${suffix}`,
      type: "chart.bar",
      version: "1.0.0",
      surface: "research-canvas",
      status: "ready",
      title: "商品价格对比",
      props: {
        kind: "chart.bar",
        eyebrow: "智能重组 · 商品目录",
        label: "商品价格对比",
        unit: "USD",
        data: priced.map(({ product, value }, index) => ({ label: compactLabel(product.title), value, displayValue: product.price, color: palette[index % palette.length] })),
        footnote: `按当前看板 ${priced.length} 个可解析价格的商品比较；区间价格按中点展示，不代表成交价。`,
      },
      evidenceRefs,
    };
  }

  if (kind === "histogram") {
    const prices = products.map((product) => priceValue(product.price)).filter((value): value is number => value !== null);
    if (!prices.length) return null;
    const min = Math.min(...prices);
    const max = Math.max(...prices);
    const bucketCount = Math.min(5, Math.max(3, Math.ceil(Math.sqrt(prices.length))));
    const width = max === min ? 1 : (max - min) / bucketCount;
    const counts = Array.from({ length: bucketCount }, () => 0);
    prices.forEach((price) => { counts[Math.min(bucketCount - 1, Math.floor((price - min) / width))] += 1; });
    return {
      id: `derived-price-histogram-${suffix}`,
      type: "chart.histogram",
      version: "1.0.0",
      surface: "research-canvas",
      status: "ready",
      title: "价格区间分布",
      props: {
        kind: "chart.histogram",
        eyebrow: "智能重组 · 商品目录",
        label: "价格区间分布",
        unit: "商品数",
        data: counts.map((count, index) => {
          const start = min + index * width;
          const end = index === bucketCount - 1 ? max : min + (index + 1) * width;
          return { label: `$${Math.round(start)}–${Math.round(end)}`, value: count, displayValue: `${count} 个`, color: palette[index % palette.length] };
        }),
        footnote: `基于当前看板 ${prices.length} 个商品价格自动分箱；区间价格按中点归类。`,
      },
      evidenceRefs,
    };
  }

  const counts = new Map<string, number>();
  products.forEach((product) => counts.set(merchantName(product.url), (counts.get(merchantName(product.url)) ?? 0) + 1));
  const entries = [...counts.entries()].sort((a, b) => b[1] - a[1]);
  return {
    id: `derived-merchant-donut-${suffix}`,
    type: "chart.donut",
    version: "1.0.0",
    surface: "research-canvas",
    status: "ready",
    title: "商家来源分布",
    props: {
      kind: "chart.donut",
      eyebrow: "智能重组 · 商品目录",
      label: "商家来源分布",
      centerLabel: `${products.length} 个商品`,
      data: entries.slice(0, 6).map(([label, value], index) => ({ label: compactLabel(label), value, displayValue: `${value} 个`, color: palette[index % palette.length] })),
      footnote: "按商品链接域名聚合，仅表示当前目录结果的来源构成。",
    },
    evidenceRefs,
  };
}
