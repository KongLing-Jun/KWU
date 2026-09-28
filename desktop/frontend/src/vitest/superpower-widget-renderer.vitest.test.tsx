import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WidgetEnvelope } from "../features/superpowers/runtime/contracts";
import { WidgetRenderer } from "../features/superpowers/widgets/WidgetRenderer";

function widget(type: WidgetEnvelope["type"], props: WidgetEnvelope["props"]): WidgetEnvelope {
  return {
    id: `fixture-${type.replaceAll(".", "-")}`,
    type,
    version: "1.0.0",
    surface: "research-canvas",
    status: "ready",
    title: `Fixture ${type}`,
    props,
    evidenceRefs: [],
    revision: 1,
  };
}

const allWidgetCases: Array<{
  type: WidgetEnvelope["type"];
  props: WidgetEnvelope["props"];
  expectedText: string;
}> = [
  {
    type: "research.summary",
    props: {
      kind: "research.summary",
      eyebrow: "OEB-U01",
      conclusion: "PASS",
      confidence: 100,
      summary: "验证通过",
      metrics: [{ label: "状态", value: "PASS", detail: "OEB-U01", tone: "positive" }],
      findings: [],
    },
    expectedText: "验证通过",
  },
  {
    type: "trend.chart",
    props: { kind: "trend.chart", eyebrow: "趋势", label: "搜索热度", range: "近 30 天", value: "128", valueLabel: "指数", change: "+12%", variant: "anomaly" },
    expectedText: "搜索热度",
  },
  {
    type: "comparison.bars",
    props: { kind: "comparison.bars", eyebrow: "竞争", label: "品牌份额", items: [{ label: "品牌 A", value: 64 }, { label: "品牌 B", value: 36 }], footnote: "确定性夹具" },
    expectedText: "品牌 A",
  },
  {
    type: "source.coverage",
    props: { kind: "source.coverage", eyebrow: "来源", label: "证据覆盖", sources: [{ source: "公开市场", role: "市场", status: "available", detail: "已连接" }], unavailable: ["店铺私域"] },
    expectedText: "公开市场",
  },
  {
    type: "product.catalog",
    props: { kind: "product.catalog", eyebrow: "商品", label: "候选商品", store: "Fixture Store", products: [{ id: "sku-1", title: "便携榨汁机", url: "https://example.invalid/sku-1", price: "¥199", availableVariants: 2, totalVariants: 3 }], footnote: "确定性夹具" },
    expectedText: "便携榨汁机",
  },
  {
    type: "chart.bar",
    props: { kind: "chart.bar", eyebrow: "对比", label: "渠道销量", unit: "件", data: [{ label: "搜索", value: 80, displayValue: "80", color: "#6f86df" }], footnote: "确定性夹具" },
    expectedText: "渠道销量",
  },
  {
    type: "chart.donut",
    props: { kind: "chart.donut", eyebrow: "结构", label: "渠道占比", centerLabel: "100%", data: [{ label: "自然流量", value: 70, displayValue: "70%", color: "#6f86df" }, { label: "付费流量", value: 30, displayValue: "30%", color: "#ef9a77" }], footnote: "确定性夹具" },
    expectedText: "自然流量",
  },
  {
    type: "chart.histogram",
    props: { kind: "chart.histogram", eyebrow: "分布", label: "价格区间", unit: "商品数", data: [{ label: "100-199", value: 12, displayValue: "12", color: "#81b29a" }], footnote: "确定性夹具" },
    expectedText: "价格区间",
  },
];

describe("WidgetRenderer complete registry contract", () => {
  it.each(allWidgetCases)("renders $type", ({ type, props, expectedText }) => {
    const fixture = widget(type, props);
    const onCite = vi.fn();
    const onOpen = vi.fn();
    const onPromote = vi.fn();
    const onRemove = vi.fn();
    const { container } = render(<WidgetRenderer widget={fixture} onCite={onCite} onOpen={onOpen} onPromote={onPromote} onRemove={onRemove} />);

    const generated = container.querySelector(`[data-generated-widget="${type}"]`);
    expect(generated).toBeInTheDocument();
    expect(screen.getByText(expectedText)).toBeVisible();

    fireEvent.click(generated!);
    fireEvent.contextMenu(generated!);
    fireEvent.click(screen.getByRole("button", { name: `将 ${fixture.title} 加入聚合看板` }));
    fireEvent.click(screen.getByRole("button", { name: `关闭卡片：${fixture.title}` }));
    expect(onOpen).toHaveBeenCalledWith(fixture);
    expect(onCite).toHaveBeenCalledWith(fixture);
    expect(onPromote).toHaveBeenCalledWith(fixture);
    expect(onRemove).toHaveBeenCalledWith(fixture);
  });

  it("renders the shared loading and deterministic empty states", () => {
    const { rerender } = render(<WidgetRenderer widget={{
      ...widget("trend.chart", allWidgetCases[1].props),
      status: "loading",
    }} />);
    expect(screen.getByRole("status", { name: "组件加载中" })).toBeVisible();

    rerender(<WidgetRenderer widget={widget("product.catalog", {
      kind: "product.catalog",
      eyebrow: "商品",
      label: "候选商品",
      store: "Fixture Store",
      products: [],
      footnote: "空态夹具",
    })} />);
    expect(screen.getByText("未找到匹配商品")).toBeVisible();
  });

  it("renders the trend availability state without chart data", () => {
    render(<WidgetRenderer widget={widget("trend.chart", {
      kind: "trend.chart",
      eyebrow: "趋势",
      label: "搜索热度",
      range: "近 30 天",
      value: "",
      valueLabel: "指数",
      availability: { label: "数据暂不可用", detail: "稍后重试" },
    })} />);
    expect(screen.getByText("数据暂不可用")).toBeVisible();
    expect(screen.getByText("稍后重试")).toBeVisible();
  });

  it("shows an explicit error instead of a blank card for malformed metrics", () => {
    render(<WidgetRenderer widget={widget("research.summary", {
      kind: "research.summary",
      eyebrow: "OEB-U01",
      conclusion: "PASS",
      confidence: 100,
      summary: "验证通过",
      metrics: { status: "PASS" },
      findings: [],
    } as unknown as WidgetEnvelope["props"])} />);

    expect(screen.getByRole("alert")).toHaveTextContent("组件数据格式无效，请重新生成。");
  });
});
