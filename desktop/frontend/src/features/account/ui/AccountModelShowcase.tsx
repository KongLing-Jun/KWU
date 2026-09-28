import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  BarChart3,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  FileText,
  Layers3,
  Mail,
  Network,
  PieChart,
  Route,
  ShieldCheck,
  Sparkles,
  Workflow,
  X,
} from "lucide-react";

import type { ModelMarketplaceModel, SharedModelCatalogState } from "../../../lib/types";

interface AccountModelShowcaseProps {
  modelCatalog?: SharedModelCatalogState;
}

interface ProviderDefinition {
  id: string;
  label: string;
  description: string;
  className: string;
  aliases: string[];
}

interface ProviderView extends ProviderDefinition {
  models: ModelMarketplaceModel[];
}

const KNOWN_PROVIDERS: ProviderDefinition[] = [
  { id: "openai", label: "OpenAI", description: "通用推理与专业创作", className: "openai", aliases: ["openai", "gpt", "codex", "o1", "o3", "o4"] },
  { id: "anthropic", label: "Anthropic", description: "长文本理解与严谨分析", className: "anthropic", aliases: ["anthropic", "claude"] },
  { id: "google", label: "Google", description: "多模态理解与内容生成", className: "google", aliases: ["google", "gemini"] },
  { id: "deepseek", label: "DeepSeek", description: "推理、代码与复杂任务", className: "deepseek", aliases: ["deepseek"] },
  { id: "glm", label: "GLM 智谱", description: "中文办公与多模态能力", className: "glm", aliases: ["glm", "zhipu", "chatglm", "智谱"] },
  { id: "minimax", label: "MiniMax", description: "语言、语音与多模态", className: "minimax", aliases: ["minimax", "abab"] },
];

const WORKFLOWS = [
  { icon: Mail, title: "读邮件", copy: "提取要点，识别待办" },
  { icon: BarChart3, title: "分析数据", copy: "理解表格，归纳趋势" },
  { icon: FileText, title: "生成汇报", copy: "整理结论，生成文档" },
  { icon: Workflow, title: "自动执行", copy: "串联工具，持续完成" },
];

const CAPABILITIES = [
  { icon: Layers3, title: "多模态支持", copy: "文本、图像、表格、语音" },
  { icon: Route, title: "智能路由", copy: "按任务匹配合适模型" },
  { icon: ShieldCheck, title: "企业级安全", copy: "数据隔离与权限控制" },
  { icon: Network, title: "多模型生态", copy: "接入国内外主流模型" },
];

function PaperPlaneVisual({ accent = false }: { accent?: boolean }) {
  const edge = accent ? "#ff806e" : "#277fef";
  const body = accent ? "#fff0ea" : "#e5f2ff";
  const upper = accent ? "#ffd5ca" : "#b9dcff";
  const fold = accent ? "#ff9d87" : "#68adf8";
  return (
    <svg viewBox="0 0 34 30" fill="none" aria-hidden="true">
      <path d="M2.3 13.6 31.2 2.2 20.1 27.5l-5.3-8.8-12.5-5.1Z" fill={body} stroke={edge} strokeWidth="1.15" strokeLinejoin="round" />
      <path d="M2.3 13.6 31.2 2.2 14.8 18.7 2.3 13.6Z" fill={upper} stroke={edge} strokeWidth=".7" strokeLinejoin="round" />
      <path d="m14.8 18.7 16.4-16.5-11.1 25.3-5.3-8.8Z" fill={fold} stroke={edge} strokeWidth=".7" strokeLinejoin="round" />
      <path d="m14.8 18.7-2.2 6 4.7-2.1" stroke={edge} strokeWidth="1" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function normalizeProviderText(model: ModelMarketplaceModel): string {
  return [model.vendor, model.vendorIcon, model.name, model.id].filter(Boolean).join(" ").toLowerCase();
}

function deduplicateModels(models: ModelMarketplaceModel[]): ModelMarketplaceModel[] {
  const seen = new Set<string>();
  return models.filter((model) => {
    const key = (model.id || model.name).trim().toLowerCase();
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function buildProviderViews(modelCatalog?: SharedModelCatalogState): ProviderView[] {
  const models = modelCatalog?.state === "ready"
    ? deduplicateModels(modelCatalog.marketplace?.sources.flatMap((source) => source.models) ?? [])
    : [];
  const known = KNOWN_PROVIDERS.map((provider) => ({
    ...provider,
    models: models.filter((model) => provider.aliases.some((alias) => normalizeProviderText(model).includes(alias))),
  }));
  const claimed = new Set(known.flatMap((provider) => provider.models.map((model) => model.id)));
  const unknownByVendor = new Map<string, ModelMarketplaceModel[]>();
  models.forEach((model) => {
    if (claimed.has(model.id)) return;
    const label = model.vendor?.trim() || "其他模型";
    const current = unknownByVendor.get(label) ?? [];
    current.push(model);
    unknownByVendor.set(label, current);
  });
  const unknown = Array.from(unknownByVendor.entries())
    .sort(([left], [right]) => left.localeCompare(right, "zh-CN"))
    .map(([label, providerModels], index): ProviderView => ({
      id: `other-${index}-${label.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`,
      label,
      description: "更多专业模型能力",
      className: "other",
      aliases: [],
      models: providerModels,
    }));
  return [...known, ...unknown];
}

function ProviderLogo({ provider }: { provider: ProviderView }) {
  return (
    <span className={`account-showcase__provider-logo account-showcase__provider-logo--${provider.className}`} aria-hidden="true">
      {provider.className === "other" ? provider.label.slice(0, 1).toUpperCase() : null}
    </span>
  );
}

function OfficeWorkflowVisual() {
  return (
    <div className="account-showcase__office-scene" aria-hidden="true">
      <svg className="account-showcase__office-connectors" viewBox="0 0 660 330" fill="none">
        <path d="M89 121C45 121 45 174 86 174H150" />
        <path d="M329 78H416C451 78 449 109 449 135" />
        <path d="M294 157V205C294 229 314 229 341 229H382" />
        <path d="M514 174H572C611 174 611 215 584 225" />
      </svg>
      <span className="account-showcase__office-plane"><PaperPlaneVisual /></span>
      <span className="account-showcase__office-plane account-showcase__office-plane--accent"><PaperPlaneVisual accent /></span>
      <span className="account-showcase__office-spark account-showcase__office-spark--coral" />
      <span className="account-showcase__office-spark account-showcase__office-spark--blue" />
      <span className="account-showcase__office-badge account-showcase__office-badge--chart"><BarChart3 size={20} /></span>
      <span className="account-showcase__office-badge account-showcase__office-badge--check"><CheckCircle2 size={22} /></span>
      <span className="account-showcase__office-detail account-showcase__office-detail--mail"><Mail size={17} /></span>
      <span className="account-showcase__office-badge account-showcase__office-badge--pie"><PieChart size={19} /></span>

      <div className="account-showcase__office-window account-showcase__office-window--mail">
        <span className="account-showcase__office-window-bar"><i /><i /><i /></span>
        <div className="account-showcase__office-window-title"><Mail size={13} /><strong>Inbox</strong></div>
        <span className="account-showcase__office-mail-row"><i /><b /><em /></span>
        <span className="account-showcase__office-mail-row"><i /><b /><em /></span>
        <span className="account-showcase__office-mail-row account-showcase__office-mail-row--active"><i /><b /><em /></span>
        <span className="account-showcase__office-mail-row"><i /><b /><em /></span>
      </div>

      <div className="account-showcase__office-window account-showcase__office-window--sheet">
        <span className="account-showcase__office-app-mark">X</span>
        <span className="account-showcase__office-sheet-grid" />
      </div>

      <div className="account-showcase__office-window account-showcase__office-window--document">
        <span className="account-showcase__office-app-mark account-showcase__office-app-mark--word">W</span>
        <span className="account-showcase__office-document-title" />
        <span className="account-showcase__office-document-line" />
        <span className="account-showcase__office-document-line" />
        <span className="account-showcase__office-document-line account-showcase__office-document-line--short" />
        <span className="account-showcase__office-document-block" />
      </div>

      <div className="account-showcase__office-window account-showcase__office-window--report">
        <span className="account-showcase__office-report-title">工作汇报</span>
        <span className="account-showcase__office-report-lines"><i /><i /><i /></span>
        <span className="account-showcase__office-pie" />
        <span className="account-showcase__office-bars"><i /><i /><i /><i /></span>
      </div>
    </div>
  );
}

export function AccountModelShowcase({ modelCatalog }: AccountModelShowcaseProps) {
  const providers = useMemo(() => buildProviderViews(modelCatalog), [modelCatalog]);
  const [activeProvider, setActiveProvider] = useState<ProviderView | null>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const [providerOverflow, setProviderOverflow] = useState({ before: false, after: false });

  const updateProviderOverflow = useCallback(() => {
    const track = trackRef.current;
    if (!track) return;
    setProviderOverflow({
      before: track.scrollLeft > 2,
      after: track.scrollLeft + track.clientWidth < track.scrollWidth - 2,
    });
  }, []);

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    updateProviderOverflow();
    track.addEventListener("scroll", updateProviderOverflow, { passive: true });
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(updateProviderOverflow);
    observer?.observe(track);
    return () => {
      track.removeEventListener("scroll", updateProviderOverflow);
      observer?.disconnect();
    };
  }, [providers, updateProviderOverflow]);

  useEffect(() => {
    if (!activeProvider) return;
    closeButtonRef.current?.focus();
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setActiveProvider(null);
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [activeProvider]);

  const scrollProviders = (direction: -1 | 1) => {
    const track = trackRef.current;
    if (!track) return;
    track.scrollBy({ left: direction * Math.max(220, track.clientWidth * 0.72), behavior: "smooth" });
  };

  return (
    <div className="account-showcase">
      <section className="account-showcase__hero" aria-labelledby="account-showcase-title">
        <div className="account-showcase__hero-copy">
          <h1 id="account-showcase-title">让每项办公任务，都有<span>合适的模型</span></h1>
          <p>一个办公 Agent，协同多家领先模型持续完成工作</p>
          <div className="account-showcase__workflow-list">
            {WORKFLOWS.map(({ icon: Icon, title, copy }, index) => (
              <div className="account-showcase__workflow-item" key={title}>
                <span className="account-showcase__workflow-index">{String(index + 1).padStart(2, "0")}</span>
                <span className="account-showcase__workflow-icon"><Icon size={17} aria-hidden="true" /></span>
                <span><strong>{title}</strong><small>{copy}</small></span>
              </div>
            ))}
          </div>
        </div>
        <div className="account-showcase__hero-visual" aria-hidden="true">
          <OfficeWorkflowVisual />
        </div>
      </section>

      <section className="account-showcase__providers" aria-labelledby="account-providers-title">
        <h2 className="sr-only" id="account-providers-title">支持的模型供应商</h2>
        <div className={`account-showcase__provider-carousel${providerOverflow.before || providerOverflow.after ? " account-showcase__provider-carousel--overflow" : ""}`}>
          {providerOverflow.before ? <button type="button" className="account-showcase__carousel-arrow account-showcase__carousel-arrow--before" aria-label="查看前一组供应商" onClick={() => scrollProviders(-1)}>
            <ChevronLeft size={18} aria-hidden="true" />
          </button> : null}
          <div className="account-showcase__provider-track" ref={trackRef}>
            {providers.map((provider) => (
              <button
                className={`account-showcase__provider-card account-showcase__provider-card--${provider.className}`}
                type="button"
                key={provider.id}
                onClick={() => setActiveProvider(provider)}
                data-account-provider={provider.id}
              >
                <span className="account-showcase__provider-head">
                  <ProviderLogo provider={provider} />
                  <span><strong>{provider.label}</strong><small>{provider.description}</small></span>
                </span>
                <span className="account-showcase__model-preview">
                  {provider.models.length > 0 ? provider.models.slice(0, 3).map((model) => (
                    <span key={model.id}>{model.name}</span>
                  )) : (
                    <span className="account-showcase__model-empty">
                      {modelCatalog?.state === "loading" ? "模型信息同步中" : "暂无模型信息"}
                    </span>
                  )}
                </span>
                <span className="account-showcase__provider-browse">浏览模型 <ChevronRight size={12} aria-hidden="true" /></span>
              </button>
            ))}
          </div>
          {providerOverflow.after ? <button type="button" className="account-showcase__carousel-arrow account-showcase__carousel-arrow--after" aria-label="查看后一组供应商" onClick={() => scrollProviders(1)}>
            <ChevronRight size={18} aria-hidden="true" />
          </button> : null}
        </div>
      </section>

      <section className="account-showcase__capabilities" aria-label="产品能力">
        {CAPABILITIES.map(({ icon: Icon, title, copy }) => (
          <div className="account-showcase__capability" key={title}>
            <span><Icon size={20} aria-hidden="true" /></span>
            <div><strong>{title}</strong><small>{copy}</small></div>
          </div>
        ))}
      </section>

      {activeProvider ? (
        <div
          className="account-showcase__dialog-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setActiveProvider(null);
          }}
        >
          <section className="account-showcase__dialog" role="dialog" aria-modal="true" aria-labelledby="account-provider-dialog-title" data-account-provider-dialog={activeProvider.id}>
            <header className="account-showcase__dialog-header">
              <ProviderLogo provider={activeProvider} />
              <div>
                <h2 id="account-provider-dialog-title">{activeProvider.label}</h2>
                <p>{activeProvider.description} · {activeProvider.models.length} 个模型</p>
              </div>
              <button ref={closeButtonRef} type="button" aria-label="关闭模型浏览" onClick={() => setActiveProvider(null)}>
                <X size={18} aria-hidden="true" />
              </button>
            </header>
            <div className="account-showcase__dialog-body">
              {activeProvider.models.length > 0 ? activeProvider.models.map((model) => (
                <article className="account-showcase__dialog-model" key={model.id}>
                  <span className="account-showcase__dialog-model-mark" aria-hidden="true"><Sparkles size={15} /></span>
                  <div>
                    <strong>{model.name}</strong>
                    <p>{model.summary || "适用于多种办公 Agent 任务"}</p>
                    {model.tags?.length ? <span>{model.tags.slice(0, 3).join(" · ")}</span> : null}
                  </div>
                </article>
              )) : (
                <div className="account-showcase__dialog-empty">模型信息暂未同步，请稍后刷新资产。</div>
              )}
            </div>
            <footer>模型信息由云端接口同步，仅用于浏览与能力展示</footer>
          </section>
        </div>
      ) : null}
    </div>
  );
}
