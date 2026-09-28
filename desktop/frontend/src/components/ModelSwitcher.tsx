import { useEffect, useMemo, useRef, useState } from "react";
import { Brain, Check, ChevronLeft, ChevronRight, ChevronsUpDown, Gauge, Search } from "lucide-react";
import { asArray } from "../lib/array";
import { BRAND_PROVIDER_ID, LEGACY_PROVIDER_ID } from "../lib/brand";
import { useT } from "../lib/i18n";
import { groupModelInfosByProvider } from "../lib/modelCatalogOrdering";
import {
  filterModelInfosForSubscriptionAccess,
  type SubscriptionAccess,
} from "../lib/subscriptionModelAccess";
import type { EffortInfo, ModelInfo, SharedModelCatalogState } from "../lib/types";
import { AnchoredPopover } from "./AnchoredPopover";
import { Tooltip } from "./Tooltip";

// ModelSwitcher opens an upward popover listing configured providers. Selecting
// one switches the active model while the current conversation continues.
export function ModelSwitcher({
  label,
  modelCatalog,
  currentBillingSource,
  unmatchedLabel,
  entryMode = "root",
  selectionPending = false,
  selectionPresentation = "auto",
  subscriptionAccess,
  effort,
  effortSnapshot,
  effortDisabled = false,
  onPick,
  onPickEffort,
}: {
  label: string;
  modelCatalog: SharedModelCatalogState;
  currentBillingSource?: ModelInfo["billingSource"];
  unmatchedLabel?: string;
  entryMode?: "root" | "models";
  selectionPending?: boolean;
  selectionPresentation?: "auto" | "choose" | "pending";
  subscriptionAccess: SubscriptionAccess;
  effort?: EffortInfo;
  effortSnapshot?: string;
  effortDisabled?: boolean;
  onPick: (ref: string, billingSource?: ModelInfo["billingSource"]) => void;
  onPickEffort?: (level: string) => void;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [panel, setPanel] = useState<"root" | "models" | "effort">("root");
  const [query, setQuery] = useState("");
  const triggerRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const subscriptionPending = subscriptionAccess.state === "loading";
  const models = useMemo(
    () => subscriptionPending || modelCatalog.state !== "ready"
      ? []
      : filterModelInfosForSubscriptionAccess(
          modelCatalog.routes,
          modelCatalog.marketplace,
          subscriptionAccess,
        ),
    [modelCatalog, subscriptionAccess, subscriptionPending],
  );

  useEffect(() => {
    if (open) {
      setPanel(entryMode);
      setQuery("");
    }
  }, [entryMode, open]);

  useEffect(() => {
    if (open && panel === "models") {
      window.requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open, panel]);

  const keyword = query.trim().toLowerCase();
  const filtered = useMemo(
    () => keyword
      ? models.filter((m) => (
        m.model.toLowerCase().includes(keyword) ||
        m.provider.toLowerCase().includes(keyword) ||
        (m.displayName ?? "").toLowerCase().includes(keyword)
      ))
      : models,
    [models, keyword],
  );

  const currentModel = useMemo(
    () => modelCatalog.state === "ready"
      ? resolveCurrentModel(modelCatalog.routes, label, currentBillingSource)
      : null,
    [currentBillingSource, label, modelCatalog],
  );
  const currentSubscriptionUnavailable = subscriptionAccess.state === "unavailable"
    && (currentBillingSource === "subscription_pool"
      || currentModel?.billingSource === "subscription_pool"
      || isOfficialSubscriptionRef(label));
  const effortLevels = asArray(effort?.levels);
  const hasEffort = Boolean(effort?.supported && effortLevels.length > 0 && onPickEffort);
  const hasEffortSnapshot = effort === undefined && effortSnapshot !== undefined && selectionPending;
  const currentEffort = effort ? effort.current || "auto" : effortSnapshot || "auto";
  const currentEffortLabel = effortLevelLabel(currentEffort, t);

  const providerLabels = {
    deepseek: t("settings.providerLabel.deepseek"),
    product: t("settings.providerLabel.kwu"),
  };
  const groups = useMemo(
    () => groupModelInfosByProvider(filtered, providerLabels),
    [filtered, providerLabels.deepseek, providerLabels.product],
  );

  const modelsPending = selectionPresentation === "pending"
    || modelCatalog.state === "loading"
    || subscriptionPending
    || selectionPending;
  const catalogUnavailable = modelCatalog.state === "unavailable";
  const triggerModelLabel = selectionPresentation === "pending"
    ? ""
    : selectionPresentation === "choose"
      ? t("modelSwitcher.selectModel")
      : currentModel && !currentSubscriptionUnavailable
    ? modelDisplayLabel(currentModel, t)
    : label.trim() && !currentSubscriptionUnavailable
      ? modelRefDisplayLabel(label, t)
    : unmatchedLabel?.trim() && !currentSubscriptionUnavailable
      ? unmatchedLabel.trim()
    : catalogUnavailable
      ? "模型配置暂不可用"
      : models.length > 0
      ? t("modelSwitcher.selectModel")
      : t("modelSwitcher.noModels");
  const modelTriggerLabel = selectionPresentation === "pending"
    ? t("common.loading")
    : triggerModelLabel;
  const triggerLabel = hasEffort || hasEffortSnapshot
    ? `${modelTriggerLabel} · ${t("modelSwitcher.effortTitle")}：${currentEffortLabel}`
    : modelTriggerLabel;

  const pick = (model: ModelInfo) => {
    setOpen(false);
    onPick(model.ref, model.billingSource);
  };
  const pickEffort = (level: string) => {
    setOpen(false);
    if (level !== currentEffort) onPickEffort?.(level);
  };

  return (
    <div className="modelsw modelsw--model">
      <Tooltip label={triggerLabel} fill>
        <button
          ref={triggerRef}
          type="button"
          className="modelsw__trigger modelsw__trigger--model"
          aria-label={triggerLabel}
          aria-expanded={open}
          aria-busy={modelsPending || undefined}
          disabled={modelsPending || catalogUnavailable || models.length === 0}
          onClick={() => setOpen((v) => !v)}
        >
          <Brain size={13} className="modelsw__kind" />
          <span
            className={`modelsw__label modelsw__selection ${selectionPresentation === "pending" ? "modelsw__label--pending" : ""}`}
            aria-live="polite"
          >
            <span className="modelsw__selection-model">{triggerModelLabel}</span>
            {(hasEffort || hasEffortSnapshot) && (
              <span className={`modelsw__selection-effort ${currentEffort !== "auto" ? "modelsw__selection-effort--explicit" : ""}`}>
                · {currentEffortLabel}
              </span>
            )}
          </span>
          <ChevronsUpDown size={11} />
        </button>
      </Tooltip>
      <AnchoredPopover
        open={open && !modelsPending}
        anchorRef={triggerRef}
        onClose={() => setOpen(false)}
        className="modelsw__menu modelsw__menu--portal modelsw__menu--model"
        contentKey={panel}
      >
        <div className="modelsw__panels" data-panel={panel}>
          {panel === "root" && (
            <div className="modelsw__root-panel">
              <button type="button" className="modelsw__nav-item" onClick={() => setPanel("models")}>
                <Brain size={13} />
                <span className="modelsw__nav-label">{t("modelSwitcher.modelTitle")}</span>
                <span className="modelsw__nav-value">{triggerModelLabel}</span>
                <ChevronRight size={13} />
              </button>
              {hasEffort && (
                <button
                  type="button"
                  className="modelsw__nav-item"
                  disabled={effortDisabled}
                  onClick={() => setPanel("effort")}
                >
                  <Gauge size={13} />
                  <span className="modelsw__nav-label">{t("modelSwitcher.effortTitle")}</span>
                  <span className="modelsw__nav-value">{currentEffortLabel}</span>
                  <ChevronRight size={13} />
                </button>
              )}
            </div>
          )}
          {panel === "models" && (
            <div className="modelsw__subpanel">
              {entryMode === "root" ? <button type="button" className="modelsw__panel-header" onClick={() => setPanel("root")}>
                <ChevronLeft size={14} />
                <span>{t("modelSwitcher.modelTitle")}</span>
              </button> : null}
              <div className="modelsw__search" role="presentation">
                <Search size={13} />
                <input
                  ref={inputRef}
                  type="text"
                  className="modelsw__search-input"
                  placeholder={t("modelSwitcher.searchPlaceholder")}
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Escape") {
                      if (entryMode === "models") setOpen(false);
                      else setPanel("root");
                    }
                    if (e.key === "Enter" && filtered.length === 1) pick(filtered[0]);
                  }}
                />
              </div>
              {entryMode === "models" && hasEffort ? <button
                type="button"
                className="modelsw__nav-item modelsw__nav-item--inline"
                disabled={effortDisabled}
                onClick={() => setPanel("effort")}
              >
                <Gauge size={13} />
                <span className="modelsw__nav-label">{t("modelSwitcher.effortTitle")}</span>
                <span className="modelsw__nav-value">{currentEffortLabel}</span>
                <ChevronRight size={13} />
              </button> : null}
              <div role="listbox">
                {models.length === 0 && <div className="modelsw__empty">{t("modelSwitcher.noModels")}</div>}
                {models.length > 0 && filtered.length === 0 && query && <div className="modelsw__empty">{t("modelSwitcher.noMatches")}</div>}
                {groups.map((g) => (
                  <div key={g.provider} role="group" aria-label={g.label} className="modelsw__group">
                    <div className="modelsw__group-label" role="presentation"><Brain size={11} />{g.label}</div>
                    {g.items.map((m) => (
                      <button
                        key={`${m.ref}:${m.billingSource || ""}`}
                        type="button"
                        role="option"
                        aria-label={modelDisplayLabel(m, t)}
                        aria-selected={currentModel === m}
                        aria-disabled={m.disabled || undefined}
                        disabled={m.disabled}
                        title={m.disabledReason ? `${modelDisplayLabel(m, t)} · ${m.disabledReason}` : modelDisplayLabel(m, t)}
                        className={`modelsw__item ${currentModel === m ? "modelsw__item--current" : ""} ${m.disabled ? "modelsw__item--disabled" : ""}`}
                        onClick={() => {
                          if (!m.disabled) pick(m);
                        }}
                      >
                        <span className="modelsw__copy">
                          <span className="modelsw__model">{modelDisplayLabel(m, t)}</span>
                          {m.billingSource && <span className="modelsw__meta">{m.billingSource === "subscription_pool" ? "平台积分" : "自定义接入"}</span>}
                          {(m.displayName || m.status) && (
                            <span className="modelsw__meta">
                              {m.displayName ? m.model : ""}
                              {m.displayName && m.status ? " · " : ""}
                              {m.status || ""}
                            </span>
                          )}
                        </span>
                        {currentModel === m && <Check size={13} className="modelsw__check" />}
                      </button>
                    ))}
                  </div>
                ))}
              </div>
            </div>
          )}
          {panel === "effort" && hasEffort && (
            <div className="modelsw__subpanel">
              <button type="button" className="modelsw__panel-header" onClick={() => setPanel(entryMode === "models" ? "models" : "root")}>
                <ChevronLeft size={14} />
                <span>{t("modelSwitcher.effortTitle")}</span>
              </button>
              <div className="modelsw__effort-list" role="group" aria-label={t("modelSwitcher.effortTitle")}>
                {effortLevels.map((level) => (
                  <button
                    key={level}
                    type="button"
                    className={`modelsw__item modelsw__effort-item ${level === currentEffort ? "modelsw__item--current" : ""}`}
                    disabled={effortDisabled}
                    aria-pressed={level === currentEffort}
                    title={level === "auto" ? t("modelSwitcher.effortAutoHint") : undefined}
                    onClick={() => pickEffort(level)}
                  >
                    <span className="modelsw__model">{effortLevelLabel(level, t)}</span>
                    {level === currentEffort && <Check size={13} className="modelsw__check" />}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </AnchoredPopover>
    </div>
  );
}

function effortLevelLabel(level: string, t: ReturnType<typeof useT>): string {
  switch (level) {
    case "auto": return t("modelSwitcher.effortAuto");
    case "low": return t("modelSwitcher.effortLow");
    case "medium": return t("modelSwitcher.effortMedium");
    case "high": return t("modelSwitcher.effortHigh");
    case "xhigh": return t("modelSwitcher.effortXHigh");
    case "max": return t("modelSwitcher.effortMax");
    default: return level;
  }
}

function resolveCurrentModel(
  models: readonly ModelInfo[],
  ref: string,
  billingSource?: ModelInfo["billingSource"],
): ModelInfo | null {
  const normalizedRef = ref.trim();
  if (!normalizedRef) return null;
  const matches = models.filter((model) => (
    (model.ref === normalizedRef || model.model === normalizedRef)
    && (!billingSource || model.billingSource === billingSource)
  ));
  if (matches.length === 1) return matches[0];
  if (billingSource) return null;
  const current = matches.filter((model) => model.current);
  return current.length === 1 ? current[0] : null;
}

function providerShortLabel(provider: string, t: ReturnType<typeof useT>): string {
  switch (provider) {
    case "deepseek":
    case "deepseek-flash":
    case "deepseek-pro":
      return t("settings.providerShortLabel.deepseek");
    case BRAND_PROVIDER_ID:
    case LEGACY_PROVIDER_ID:
    case "airclaude_subscription":
      return t("settings.providerShortLabel.kwu");
    default:
      return provider;
  }
}

function isOfficialSubscriptionRef(ref: string): boolean {
  const normalized = ref.trim();
  const separator = normalized.indexOf("/");
  if (separator <= 0 || separator === normalized.length - 1) return false;
  const provider = normalized.slice(0, separator);
  return provider === BRAND_PROVIDER_ID
    || provider === LEGACY_PROVIDER_ID
    || provider === "airclaude_subscription";
}

function modelDisplayLabel(model: ModelInfo, t: ReturnType<typeof useT>): string {
  return `${providerShortLabel(model.provider, t)}/${model.displayName || model.model}`;
}

function modelRefDisplayLabel(ref: string, t: ReturnType<typeof useT>): string {
  const normalized = ref.trim();
  const separator = normalized.indexOf("/");
  if (separator <= 0 || separator === normalized.length - 1) return normalized;
  return `${providerShortLabel(normalized.slice(0, separator), t)}/${normalized.slice(separator + 1)}`;
}
