import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeftRight, RefreshCw, Search } from "lucide-react";
import { app } from "../../../lib/bridge";
import { useT } from "../../../lib/i18n";
import { groupModelInfosByProvider } from "../../../lib/modelCatalogOrdering";
import {
  filterModelInfosForSubscriptionAccess,
  subscriptionMarketplaceNeedsRefresh,
  type SubscriptionAccess,
} from "../../../lib/subscriptionModelAccess";
import type { ModelInfo, ModelMarketplaceModel, ModelMarketplaceSource, ModelMarketplaceView, SharedModelCatalogState } from "../../../lib/types";
import { MarketplaceFilterSelect } from "./MarketplaceFilterSelect";

const SUBSCRIPTION_PLANS = [
  { value: "Free", label: "Free", catalogTitle: "Free 套餐模型", tag: "Subscribe_Free" },
  { value: "experience", label: "体验套餐", catalogTitle: "体验套餐模型", tag: "Subscribe_Pro" },
] as const;

type ModelMarketplaceProps = {
  loadMarketplace?: () => Promise<ModelMarketplaceView>;
  refreshMarketplace?: () => Promise<ModelMarketplaceView>;
  claimAutoRefresh?: () => boolean;
  modelCatalog: SharedModelCatalogState;
  onSelectModel?: (ref: string, billingSource?: ModelInfo["billingSource"]) => Promise<void> | void;
  onConfigureModels?: () => void;
  onRequireAccount?: () => void;
  onModelSelected?: () => void;
  onMarketplaceChanged?: () => void;
  subscriptionAccess: SubscriptionAccess;
  activeTabId?: string | null;
};

type LoadState =
  | { status: "loading" }
  | { status: "success"; view: ModelMarketplaceView }
  | { status: "error" };

const defaultLoadMarketplace = () => app.ModelMarketplace();
const defaultRefreshMarketplace = () => app.RefreshModelMarketplace();

export function ModelMarketplace({
  loadMarketplace = defaultLoadMarketplace,
  refreshMarketplace = defaultRefreshMarketplace,
  claimAutoRefresh,
  modelCatalog,
  onSelectModel,
  onModelSelected,
  onMarketplaceChanged,
  subscriptionAccess,
  activeTabId,
}: ModelMarketplaceProps) {
  const t = useT();
  const [state, setState] = useState<LoadState>({ status: "loading" });
  const [refreshing, setRefreshing] = useState(false);
  const [activeSourceId, setActiveSourceId] = useState("");
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [vendorFilter, setVendorFilter] = useState("");
  const [planFilter, setPlanFilter] = useState("");
  const [selectingRef, setSelectingRef] = useState("");
  const refreshInFlightRef = useRef<Promise<void> | null>(null);

  useEffect(() => {
    let cancelled = false;
    loadMarketplace()
      .then((view) => {
        if (!cancelled) setState({ status: "success", view });
      })
      .catch(() => {
        if (!cancelled) setState({ status: "error" });
      });
    return () => {
      cancelled = true;
    };
  }, [loadMarketplace]);

  const selectableModels = useMemo(() => modelCatalog.state === "ready"
    ? filterModelInfosForSubscriptionAccess(
        modelCatalog.routes,
        modelCatalog.marketplace,
        subscriptionAccess,
      )
    : [], [modelCatalog, subscriptionAccess]);
  const orderedSelectableModels = useMemo(() => groupModelInfosByProvider(selectableModels, {
    deepseek: t("settings.providerLabel.deepseek"),
    product: t("settings.providerLabel.kwu"),
  }).flatMap((group) => group.items), [selectableModels, t]);

  const refresh = useCallback(() => {
    if (refreshInFlightRef.current) return refreshInFlightRef.current;
    setRefreshing(true);
    const request = Promise.resolve()
      .then(refreshMarketplace)
      .then((view) => {
        setState({ status: "success", view });
        onMarketplaceChanged?.();
      })
      .catch(() => {
        setState({ status: "error" });
      })
      .finally(() => {
        if (refreshInFlightRef.current === request) refreshInFlightRef.current = null;
        setRefreshing(false);
      });
    refreshInFlightRef.current = request;
    return request;
  }, [onMarketplaceChanged, refreshMarketplace]);

  useEffect(() => {
    if (state.status !== "success" || !subscriptionMarketplaceNeedsRefresh(state.view) || !claimAutoRefresh?.()) return;
    void refresh();
  }, [claimAutoRefresh, refresh, state]);

  const sources = state.status === "success" ? state.view.sources ?? [] : [];
  const activeSource = sources.find((source) => source.id === activeSourceId) ?? sources[0];
  const activeModels = activeSource?.models ?? [];
  const vendors = useMemo(() => uniqueModelValues(activeModels, (model) => (model.vendor ? [model.vendor] : [])), [activeModels]);
  const planOptions = useMemo(() => subscriptionPlanOptions(activeModels), [activeModels]);
  const filteredModels = useMemo(
    () => filterModels(activeModels, deferredQuery, vendorFilter, planFilter),
    [activeModels, deferredQuery, vendorFilter, planFilter],
  );
  const filteredVendorCount = useMemo(
    () => uniqueModelValues(filteredModels, (model) => (model.vendor ? [model.vendor] : [])).length,
    [filteredModels],
  );
  const catalogTitle = SUBSCRIPTION_PLANS.find((plan) => plan.value === planFilter)?.catalogTitle ?? "全部模型";
  const selectModel = async (model: ModelInfo) => {
    if (model.disabled || model.current || selectingRef) return;
    setSelectingRef(model.ref);
    try {
      if (onSelectModel) {
        await onSelectModel(model.ref, model.billingSource);
      } else if (activeTabId) {
        if (model.billingSource) await app.SetModelRouteForTab(activeTabId, model.ref, model.billingSource);
        else await app.SetModelForTab(activeTabId, model.ref);
      } else {
        await app.SetModel(model.ref);
      }
      onModelSelected?.();
    } finally {
      setSelectingRef("");
    }
  };
  const selectSource = (source: ModelMarketplaceSource) => {
    setActiveSourceId(source.id);
    setQuery("");
    setVendorFilter("");
    setPlanFilter("");
  };

  const selectableModelsLoading = modelCatalog.state === "loading";
  const selectableStatus = selectableModelsLoading
    ? "正在读取模型配置…"
    : modelCatalog.state === "unavailable"
      ? "模型配置暂不可用"
      : subscriptionAccess.state === "loading"
      ? t("modelSwitcher.subscriptionLoading")
      : subscriptionAccess.state === "unavailable"
        ? t("modelSwitcher.subscriptionUnavailable")
        : "当前套餐暂无可切换模型。";
  const hasSelectableModels = !selectableModelsLoading && selectableModels.length > 0;

  const selectableSection = (
    <section
      className={`model-marketplace-selectable${hasSelectableModels ? " model-marketplace-selectable--expanded" : ""}`}
      aria-labelledby="model-marketplace-selectable-title"
    >
      <div className="model-marketplace-selectable__heading">
        <div className="model-marketplace-selectable__heading-main">
          <span className="model-marketplace-selectable__icon" aria-hidden="true">
            <ArrowLeftRight size={19} />
          </span>
          <div>
            <h3 id="model-marketplace-selectable-title">当前可切换模型</h3>
            {hasSelectableModels ? (
              <p>来自当前桌面配置，与对话输入框中的模型列表保持一致。</p>
            ) : (
              <p className="model-marketplace-selectable__status" role="status">{selectableStatus}</p>
            )}
          </div>
        </div>
        <span>{selectableModelsLoading ? "读取中" : `${selectableModels.length} 个`}</span>
      </div>
      {hasSelectableModels ? (
        <div className="model-marketplace-selectable__scroller" aria-label="可选择模型列表">
          <div className="model-marketplace-selectable__grid">
            {orderedSelectableModels.map((model) => {
              const selected = model.current;
              const busy = selectingRef === model.ref;
              return (
                <button
                  key={`${model.ref}:${model.billingSource ?? ""}`}
                  className={`model-marketplace-selectable__card${selected ? " model-marketplace-selectable__card--current" : ""}`}
                  type="button"
                  data-selectable-model={model.ref}
                  aria-pressed={selected}
                  disabled={model.disabled || busy}
                  onClick={() => void selectModel(model)}
                >
                  <span>
                    <strong>{model.displayName || model.model}</strong>
                    <small>{selectableProviderLabel(model.provider)}{model.billingSource ? ` · ${model.billingSource === "subscription_pool" ? "平台积分" : "自备密钥"}` : ""}</small>
                  </span>
                  <em>{busy ? "切换中…" : selected ? "当前模型" : model.disabled ? model.disabledReason || "不可用" : "选择"}</em>
                </button>
              );
            })}
          </div>
        </div>
      ) : null}
    </section>
  );

  if (state.status === "loading") {
    return (
      <section className="model-marketplace" aria-label="模型广场">
        <ModelMarketplaceHeader onRefresh={refresh} refreshing={refreshing} />
        {selectableSection}
        <div className="model-marketplace-status" role="status">正在读取模型数据…</div>
      </section>
    );
  }

  if (state.status === "error" || !activeSource) {
    return (
      <section className="model-marketplace" aria-label="模型广场">
        <ModelMarketplaceHeader onRefresh={refresh} refreshing={refreshing} />
        {selectableSection}
        <ModelMarketplaceEmptyBody
          title="需要联网获取模型数据"
          detail="本地暂无可用缓存，请明确刷新后再查看模型。"
          actionLabel={refreshing ? "刷新中…" : "联网获取模型数据"}
          onAction={refresh}
        />
      </section>
    );
  }

  const unavailable = activeSource.pricingState === "unavailable" && activeModels.length === 0;

  return (
    <section className="model-marketplace" aria-label="模型广场">
      <ModelMarketplaceHeader
        sourceCount={sources.length}
        currentCount={filteredModels.length}
        vendorCount={filteredVendorCount}
        sourceLabel={activeSource.label}
        onRefresh={refresh}
        refreshing={refreshing}
      />

      {selectableSection}

      <div className="model-marketplace-controls">
        {sources.length > 1 || sourceBadges(activeSource).length > 0 ? (
          <div className="model-marketplace-sourcebar">
            {sources.length > 1 ? (
              <div className="model-marketplace-tabs" role="tablist" aria-label="模型数据源">
                {sources.map((source) => {
                  const selected = source.id === activeSource.id;
                  return (
                    <button
                      key={source.id}
                      className={`model-marketplace-tab${selected ? " model-marketplace-tab--active" : ""}`}
                      type="button"
                      role="tab"
                      aria-selected={selected}
                      data-source-id={source.id}
                      onClick={() => selectSource(source)}
                    >
                      <span>{source.label}</span>
                      <span>{source.models.length}</span>
                    </button>
                  );
                })}
              </div>
            ) : null}
            <div className="model-marketplace__actions" aria-label="数据状态">
              {sourceBadges(activeSource).map((badge) => (
                <span className="model-marketplace__count" key={badge}>{badge}</span>
              ))}
            </div>
          </div>
        ) : null}

        <div className="model-marketplace-toolbar" aria-label="模型筛选">
          <label className="model-marketplace-search">
            <Search size={15} aria-hidden="true" />
            <input
              data-model-marketplace-search
              aria-label="搜索模型或厂商"
              autoComplete="off"
              value={query}
              onInput={(event) => setQuery(event.currentTarget.value)}
              placeholder="搜索模型或厂商…"
            />
          </label>
          <MarketplaceFilterSelect
            kind="vendor"
            label="厂商"
            allLabel="全部厂商"
            value={vendorFilter}
            options={vendors.map((vendor) => ({ value: vendor, label: vendor }))}
            onChange={setVendorFilter}
          />
          <MarketplaceFilterSelect
            kind="plan"
            label="套餐"
            allLabel="全部"
            value={planFilter}
            options={planOptions}
            onChange={setPlanFilter}
          />
        </div>
      </div>

      <section className="model-marketplace-catalog" aria-labelledby="model-marketplace-catalog-title">
        <header className="model-marketplace-catalog__header">
          <h3 id="model-marketplace-catalog-title">{catalogTitle}</h3>
          <span
            data-model-marketplace-result-count
            role="status"
            aria-live="polite"
            aria-atomic="true"
          >
            {filteredModels.length} 个结果
          </span>
        </header>
        {unavailable ? (
          <ModelMarketplaceEmptyBody
            title="需要联网获取模型数据"
            detail="本地暂无该数据源的可用缓存，请明确刷新。"
            actionLabel={refreshing ? "刷新中…" : "联网获取模型数据"}
            onAction={refresh}
          />
        ) : filteredModels.length === 0 ? (
          <ModelMarketplaceEmptyBody
            title={activeModels.length === 0 ? "暂无模型数据" : "暂无匹配模型"}
            detail="请调整搜索或筛选条件。"
          />
        ) : (
          <div className="model-marketplace-grid">
            {filteredModels.map((model) => (
              <ModelMarketplaceCard
                key={model.id}
                model={model}
              />
            ))}
          </div>
        )}
      </section>
    </section>
  );
}

function ModelMarketplaceHeader({
  sourceCount,
  currentCount,
  vendorCount,
  sourceLabel,
  onRefresh,
  refreshing,
}: {
  sourceCount?: number;
  currentCount?: number;
  vendorCount?: number;
  sourceLabel?: string;
  onRefresh: () => void;
  refreshing: boolean;
}) {
  return (
    <header className="model-marketplace__header">
      <div>
        <h2 className="model-marketplace__title">模型广场</h2>
        <p className="model-marketplace__subtitle">浏览适合办公任务的国际模型。</p>
        <p className="model-marketplace__source-note">
          {sourceLabel ? `数据来自 ${sourceLabel} · ` : ""}本地缓存优先 · 无缓存时自动获取
        </p>
      </div>
      <div className="model-marketplace__actions">
        {typeof sourceCount === "number" ? <span className="model-marketplace__count">{sourceCount} 数据源</span> : null}
        {typeof currentCount === "number" ? <span className="model-marketplace__count">{currentCount} 个模型</span> : null}
        {typeof vendorCount === "number" ? <span className="model-marketplace__count">{vendorCount} 个厂商</span> : null}
        <button data-model-marketplace-refresh className="model-marketplace__refresh" type="button" onClick={onRefresh} disabled={refreshing}>
          <RefreshCw size={15} aria-hidden="true" />
          {refreshing ? "刷新中…" : "刷新"}
        </button>
      </div>
    </header>
  );
}

function ModelMarketplaceEmptyBody({
  title,
  detail,
  actionLabel,
  onAction,
}: {
  title: string;
  detail: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <div className="model-marketplace-empty">
      <h3>{title}</h3>
      <p>{detail}</p>
      {actionLabel && onAction ? (
        <button
          data-model-marketplace-refresh
          className="model-marketplace-empty__button"
          type="button"
          onClick={onAction}
        >
          <RefreshCw size={15} aria-hidden="true" />
          {actionLabel}
        </button>
      ) : null}
    </div>
  );
}

function ModelMarketplaceCard({
  model,
}: {
  model: ModelMarketplaceModel;
}) {
  const avatar = avatarText(model);
  const logo = modelLogo(model);
  return (
    <article
      className="model-marketplace-card"
      data-model-name={model.name}
    >
      <span className="model-marketplace-card__top">
        <span className={`model-marketplace-card__avatar${logo ? ` model-marketplace-card__avatar--${logo}` : ""}`} aria-hidden="true">{avatar}</span>
        <span className="model-marketplace-card__identity">
          <strong className="model-marketplace-card__name" title={model.name}>{model.name}</strong>
          {model.vendor ? <span className="model-marketplace-card__vendor">{model.vendor}</span> : null}
        </span>
      </span>
    </article>
  );
}

function sourceBadges(source: ModelMarketplaceSource): string[] {
  const badges: string[] = [];
  if (source.errorCode && source.models.length > 0) badges.push("刷新失败，已使用缓存");
  if (source.pricingState === "stale_usable") badges.push("模型数据缓存已过期，暂可使用");
  if (source.pricingState === "stale_blocked") badges.push("模型数据已过期");
  if (source.statusAuthenticity === "MOCK_ONLY") badges.push("MOCK_ONLY");
  if (source.partial) badges.push("部分模型未展示");
  if (source.metadataStatus === "metadata_write_failed") badges.push("元数据缓存写入失败");
  if (source.metadataStatus === "metadata_unavailable") badges.push("展示元数据不可用");
  return badges;
}

function uniqueModelValues(models: ModelMarketplaceModel[], pick: (model: ModelMarketplaceModel) => string[]): string[] {
  const values: string[] = [];
  for (const model of models) {
    for (const value of pick(model)) {
      const trimmed = value.trim();
      if (trimmed && !values.includes(trimmed)) values.push(trimmed);
    }
  }
  return values;
}

function subscriptionPlanOptions(models: ModelMarketplaceModel[]): Array<{ value: string; label: string }> {
  return SUBSCRIPTION_PLANS.map((plan) => {
    const count = models.filter((model) => model.tags?.includes(plan.tag)).length;
    return { value: plan.value, label: `${plan.label}（${count}）` };
  });
}

function modelAvailableInPlan(model: ModelMarketplaceModel, planFilter: string): boolean {
  if (!planFilter) return true;
  const plan = SUBSCRIPTION_PLANS.find((item) => item.value === planFilter);
  return plan ? model.tags?.includes(plan.tag) ?? false : false;
}

function filterModels(models: ModelMarketplaceModel[], query: string, vendorFilter: string, planFilter: string): ModelMarketplaceModel[] {
  const normalizedQuery = query.trim().toLowerCase();
  return models.filter((model) => {
    if (vendorFilter && model.vendor !== vendorFilter) return false;
    if (!modelAvailableInPlan(model, planFilter)) return false;
    if (!normalizedQuery) return true;
    return [model.name, model.vendor]
      .filter((value): value is string => typeof value === "string")
      .some((value) => value.toLowerCase().includes(normalizedQuery));
  });
}

function selectableProviderLabel(provider: string): string {
  switch (provider.trim().toLowerCase()) {
    case "airclaude_subscription":
    case "kwu":
      return "云智企办";
    case "deepseek":
    case "deepseek-flash":
    case "deepseek-pro":
      return "深度求索";
    default:
      return provider;
  }
}

type ModelLogo = "openai" | "claude" | "google" | "alibaba" | "deepseek" | "zhipu";

const MODEL_LOGO_RULES: Array<{ logo: ModelLogo; pattern: RegExp }> = [
  { logo: "openai", pattern: /(^|[^a-z])(openai|chatgpt|gpt|o1|o3|o4|codex|sora|dall-e)([^a-z]|$)/i },
  { logo: "claude", pattern: /(^|[^a-z])(anthropic|claude)([^a-z]|$)/i },
  { logo: "google", pattern: /(^|[^a-z])(google|gemini|imagen|veo|gemma)([^a-z]|$)/i },
  { logo: "alibaba", pattern: /(^|[^a-z])(alibaba|qwen|qwq|qvq|tongyi|wan)([^a-z]|$)/i },
  { logo: "deepseek", pattern: /(^|[^a-z])deepseek([^a-z]|$)/i },
  { logo: "zhipu", pattern: /(^|[^a-z])(zhipu|chatglm|glm)([^a-z]|$)/i },
];

function modelLogo(model: ModelMarketplaceModel): ModelLogo | undefined {
  const explicit = (model.icon || model.vendorIcon || "")
    .split(".", 1)[0]
    .replace(/[^a-z0-9]+/gi, "")
    .toLowerCase();
  const explicitLogos: Record<string, ModelLogo> = {
    openai: "openai",
    claude: "claude",
    anthropic: "claude",
    google: "google",
    gemini: "google",
    alibaba: "alibaba",
    deepseek: "deepseek",
    zhipu: "zhipu",
    chatglm: "zhipu",
    glm: "zhipu",
  };
  if (explicitLogos[explicit]) return explicitLogos[explicit];

  const identity = `${model.icon || ""} ${model.vendorIcon || ""} ${model.vendor || ""} ${model.name}`.trim();
  return MODEL_LOGO_RULES.find((rule) => rule.pattern.test(identity))?.logo;
}

function avatarText(model: ModelMarketplaceModel): string {
  const raw = (model.icon || model.vendorIcon || model.vendor || model.name).replace(/\.Color$/i, "").trim();
  if (!raw) return "";
  const compact = raw.replace(/[^a-z0-9\u4e00-\u9fa5]+/gi, "");
  return compact.slice(0, 2) || raw.slice(0, 2);
}
