import { useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";
import {
  modelProviderLabel,
  providerGroupID,
  providerGroupLabel,
  providerKeyStatusLabel,
} from "../../../lib/providerDisplay";
import { providerRequiresKey } from "../../../lib/providerModels";
import { useT } from "../../../lib/i18n";
import type { ProviderView, SettingsView } from "../../../lib/types";
import { AnchoredPopover } from "../../../components/AnchoredPopover";

export function toRef(model: string, s: SettingsView): string {
  if (!model) return "";
  if (model.includes("/")) return model;
  const byName = s.providers.find((p) => p.name === model);
  if (byName) return `${byName.name}/${byName.default || byName.models[0] || ""}`;
  const byModel = s.providers.find((p) => p.models.includes(model));
  if (byModel) return `${byModel.name}/${model}`;
  return model;
}

export function settingsModelMeta(s: SettingsView, t: ReturnType<typeof useT>): string {
  const ref = toRef(s.defaultModel, s);
  if (!ref) return t("common.none");
  if (!ref.includes("/")) return ref;
  const [provider, ...modelParts] = ref.split("/");
  const model = modelParts.join("/") || ref;
  const providerView = s.providers.find((p) => p.name === provider);
  return `${modelProviderLabel(provider, providerView, t)} · ${model}`;
}

type ModelPickerOption = {
  ref: string;
  provider: string;
  model: string;
  billingSource?: string;
  providerView?: ProviderView;
};

export type ModelPickerRoute = {
  ref: string;
  billingSource?: string;
};

export type DerivedModelSelectionState =
  | "loading"
  | "inherited_available"
  | "inherited_unavailable"
  | "explicit_available"
  | "explicit_unavailable";

export function ModelPicker({
  s,
  refs,
  routes,
  value,
  selectedBillingSource,
  disabled,
  optionsLoading = false,
  includeSameDefault = false,
  emptyOptionLabel,
  emptyOptionHint,
  selectionState,
  unavailableValueLabel,
  unavailableValueMeta,
  onPick,
}: {
  s: SettingsView;
  refs?: string[];
  routes?: readonly ModelPickerRoute[];
  value: string;
  selectedBillingSource?: string;
  disabled: boolean;
  optionsLoading?: boolean;
  includeSameDefault?: boolean;
  emptyOptionLabel?: string;
  emptyOptionHint?: string;
  selectionState?: DerivedModelSelectionState;
  unavailableValueLabel?: string;
  unavailableValueMeta?: string;
  onPick: (ref: string, billingSource?: string) => void;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [triggerWidth, setTriggerWidth] = useState<number>();
  const triggerRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const measure = () => setTriggerWidth(trigger.getBoundingClientRect().width);
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(trigger);
    return () => observer.disconnect();
  }, []);
  // Debounce search to avoid expensive filtering on every keystroke
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 150);
    return () => clearTimeout(timer);
  }, [query]);
  const q = debouncedQuery.trim().toLowerCase();
  const emptyLabel = includeSameDefault ? t("settings.plannerNone") : emptyOptionLabel;
  const emptyHint = includeSameDefault ? t("settings.plannerNoneHint") : emptyOptionHint;
  const emptyMeta = includeSameDefault ? t("settings.plannerNoneHintShort") : emptyOptionHint;
  const pickerRoutes: readonly ModelPickerRoute[] = routes ?? (refs ?? []).map((ref): ModelPickerRoute => ({ ref }));
  const routeRefs = pickerRoutes.map((route) => route.ref);
  const selected = pickerRoutes
    .filter((route) => route.ref === value && (!selectedBillingSource || route.billingSource === selectedBillingSource))
    .map((route) => modelOptionFromRef(route.ref, s, route.billingSource))
    .find((option): option is ModelPickerOption => Boolean(option)) ?? null;
  const maskUnavailableValue = Boolean(value && !selected && unavailableValueLabel);
  const pendingSelection = optionsLoading && value ? modelOptionFromRef(value, s, selectedBillingSource) : null;
  const selectedLabel = value === "" && emptyLabel
    ? emptyLabel
    : selected?.model || pendingSelection?.model || (maskUnavailableValue ? unavailableValueLabel : value) || t("common.none");
  const waitingForSelection = optionsLoading
    && (!selected || !selected.providerView)
    && (Boolean(value) || routeRefs.length === 0);
  const selectionStateMeta = selectionState === "loading"
    ? t("settings.subscriptionModelsLoading")
    : selectionState === "inherited_available"
      ? t("settings.derivedModelInheritedAvailable")
      : selectionState === "inherited_unavailable"
        ? t("settings.derivedModelInheritedUnavailable")
        : selectionState === "explicit_available"
          ? t("settings.derivedModelExplicitAvailable")
          : selectionState === "explicit_unavailable"
            ? t("settings.derivedModelExplicitUnavailable")
            : "";
  const selectedMeta = selectionStateMeta || (waitingForSelection
    ? t("settings.subscriptionModelsLoading")
    : value === "" && emptyLabel
    ? emptyMeta || ""
    : selected
    ? modelOptionMeta(selected, t)
    : maskUnavailableValue
    ? unavailableValueMeta || t("settings.currentModelUnavailable")
    : value
    ? t("settings.currentModelUnavailable")
    : t("settings.noModelsConfigured"));
  const emptyOptionVisible = Boolean(emptyLabel) && (!q || `${emptyLabel} ${emptyHint || ""}`.toLowerCase().includes(q));

  const groups = useMemo(() => {
    const providerOrder: string[] = [];
    const providerSeen = new Set<string>();
    for (const p of s.providers) {
      const id = providerGroupID(p);
      if (!providerSeen.has(id)) {
        providerOrder.push(id);
        providerSeen.add(id);
      }
    }
    const options = pickerRoutes
      .map((route) => modelOptionFromRef(route.ref, s, route.billingSource))
      .filter((opt): opt is ModelPickerOption => Boolean(opt))
      .filter((opt) => !q || `${opt.ref} ${opt.provider} ${modelProviderLabel(opt.provider, opt.providerView, t)} ${opt.model}`.toLowerCase().includes(q));
    for (const opt of options) {
      const groupID = modelOptionGroupID(opt);
      if (!providerSeen.has(groupID)) {
        providerOrder.push(groupID);
        providerSeen.add(groupID);
      }
    }
    return providerOrder
      .map((groupID) => {
        const providerViews = s.providers.filter((p) => providerGroupID(p) === groupID);
        const firstProvider = providerViews[0];
        return {
          groupID,
          label: firstProvider ? providerGroupLabel(firstProvider, t) : groupID,
          keySet: providerViews.some((p) => p.keySet),
          requiresKey: providerViews.every((p) => providerRequiresKey(p)),
          options: uniqueModelOptions(options.filter((opt) => modelOptionGroupID(opt) === groupID)),
        };
      })
      .filter((group) => group.options.length > 0);
  }, [pickerRoutes, q, s, t]);

  useEffect(() => {
    if (!open) setQuery("");
  }, [open]);

  const pick = (option: ModelPickerOption) => {
    setOpen(false);
    if (option.ref !== value || (selectedBillingSource && option.billingSource !== selectedBillingSource)) {
      onPick(option.ref, option.billingSource);
    }
  };

  return (
    <div className="settings-model-picker">
      <button
        ref={triggerRef}
        type="button"
        className="settings-model-picker__trigger"
        disabled={disabled || (!includeSameDefault && !emptyOptionLabel && routeRefs.length === 0)}
        aria-haspopup="listbox"
        aria-expanded={open}
        data-model-selection-state={selectionState}
        onClick={() => setOpen((next) => !next)}
      >
        <span className="settings-model-picker__selected">
          <span>{selectedLabel}</span>
          <small>{selectedMeta}</small>
        </span>
        <ChevronDown size={16} className={`settings-model-picker__chev${open ? " settings-model-picker__chev--open" : ""}`} />
      </button>
      <AnchoredPopover
        open={open && !disabled}
        anchorRef={triggerRef}
        onClose={() => setOpen(false)}
        className="settings-model-picker__menu"
        placement="bottom"
        style={{ width: triggerWidth }}
      >
        <div className="settings-model-picker__search">
          <input
            value={query}
            placeholder={t("settings.searchModels")}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
          />
        </div>
        <div className="settings-model-picker__list" role="listbox">
          {emptyOptionVisible && (
            <button
              type="button"
              role="option"
              aria-selected={value === ""}
              className={`settings-model-picker__option settings-model-picker__option--pinned${value === "" ? " settings-model-picker__option--selected" : ""}`}
              onClick={() => {
                setOpen(false);
                if (value !== "") onPick("");
              }}
            >
              <span>
                <strong>{emptyLabel}</strong>
                {emptyHint && <small>{emptyHint}</small>}
              </span>
              {value === "" && <Check size={14} />}
            </button>
          )}
          {groups.map((group) => (
            <div className="settings-model-picker__group" key={group.groupID}>
              <div className="settings-model-picker__group-title">
                <span>{group.label}</span>
                <small>{providerKeyStatusLabel(group, t)}</small>
              </div>
              {group.options.map((opt) => (
                <button
                  key={`${opt.ref}:${opt.billingSource ?? ""}`}
                  type="button"
                  role="option"
                  aria-selected={opt.ref === value && (!selectedBillingSource || opt.billingSource === selectedBillingSource)}
                  className={`settings-model-picker__option${opt.ref === value && (!selectedBillingSource || opt.billingSource === selectedBillingSource) ? " settings-model-picker__option--selected" : ""}`}
                  onClick={() => pick(opt)}
                >
                  <span>
                    <strong>{opt.model}</strong>
                    <small>{modelOptionMeta(opt, t)}</small>
                  </span>
                  {opt.ref === value && (!selectedBillingSource || opt.billingSource === selectedBillingSource) && <Check size={14} />}
                </button>
              ))}
            </div>
          ))}
          {!emptyOptionVisible && groups.length === 0 && <div className="settings-model-picker__empty">{t("settings.noMatchingModels")}</div>}
        </div>
      </AnchoredPopover>
    </div>
  );
}

function modelOptionFromRef(ref: string, s: SettingsView, billingSource?: string): ModelPickerOption | null {
  if (!ref) return null;
  const [provider, ...modelParts] = ref.split("/");
  const model = modelParts.join("/") || ref;
  return {
    ref,
    provider,
    model,
    billingSource,
    providerView: providerViewsForRoute(s, provider, billingSource)[0],
  };
}

function providerViewsForRoute(s: SettingsView, provider: string, billingSource?: string): ProviderView[] {
  const matches = s.providers.filter((p) => p.name === provider);
  if (!billingSource) return matches;
  return matches.filter((p) => (
    (p.billingSource || (p.builtIn ? "subscription_pool" : "newapi_byok")) === billingSource
  ));
}

function modelOptionMeta(option: ModelPickerOption, t: ReturnType<typeof useT>): string {
  const key = option.providerView ? providerKeyStatusLabel(option.providerView, t) : t("settings.noKey");
  return `${modelProviderLabel(option.provider, option.providerView, t)} · ${key}`;
}

function modelOptionGroupID(option: ModelPickerOption): string {
  return option.providerView ? providerGroupID(option.providerView) : `custom:${option.provider}`;
}

function uniqueModelOptions(options: ModelPickerOption[]): ModelPickerOption[] {
  const seen = new Set<string>();
  const out: ModelPickerOption[] = [];
  for (const option of options) {
    const identity = `${option.model}\u0000${option.billingSource ?? ""}`;
    if (seen.has(identity)) continue;
    seen.add(identity);
    out.push(option);
  }
  return out;
}
