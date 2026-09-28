import { memo, useEffect, useMemo, useState } from "react";
import { uniqueStrings } from "../../../lib/array";
import { BRAND_PROVIDER_ID, LEGACY_PROVIDER_ID } from "../../../lib/brand";
import { app } from "../../../lib/bridge";
import { reportClientError } from "../../../lib/clientError";
import { useT } from "../../../lib/i18n";
import {
  apiKeyEnvFromProviderName,
  inferredVisionModels,
  isProviderApiKeyEnvName,
  mergedFetchedProviderModels,
  newCustomProviderKind,
  providerApiKeyEnvForSave,
  providerDefaultModel,
  providerIsConfigured,
  providerModelCandidates,
  providerRequiresKey,
} from "../../../lib/providerModels";
import {
  modelProviderLabel,
  providerGroupDescription,
  providerGroupID,
  providerGroupLabel,
  providerKeyStatusLabel,
} from "../../../lib/providerDisplay";
import type { ProviderView, SettingsView } from "../../../lib/types";
import type { SubscriptionAccountState } from "../../../lib/subscriptionModelAccess";
import { InlineConfirmButton } from "../../../components/InlineConfirmButton";
import { Tooltip } from "../../../components/Tooltip";
import { ModelPicker, toRef, type DerivedModelSelectionState } from "./ModelPicker";
import { SettingsField, SettingsSection } from "./SettingsPrimitives";

type SectionProps = {
  s: SettingsView;
  busy: boolean;
  apply: (fn: () => Promise<unknown>) => Promise<void>;
};

type ModelsSettingsSectionProps = SectionProps & {
  modelProviders: ProviderView[];
  modelRoutes: SettingsModelRoute[];
  modelOptionsLoading: boolean;
  subscriptionAccountState: SubscriptionAccountState;
};

export type SettingsModelRoute = {
  ref: string;
  billingSource: "subscription_pool" | "newapi_byok";
};

export function derivedModelSelectionState(input: {
  loading: boolean;
  inheritsDefault: boolean;
  selectedRef: string;
  defaultRef: string;
  availableRefs: readonly string[];
}): DerivedModelSelectionState {
  if (input.loading) return "loading";
  if (input.inheritsDefault) {
    return input.defaultRef && input.availableRefs.includes(input.defaultRef)
      ? "inherited_available"
      : "inherited_unavailable";
  }
  return input.selectedRef && input.availableRefs.includes(input.selectedRef)
    ? "explicit_available"
    : "explicit_unavailable";
}

export function modelRoutesForSettings(s: SettingsView, providers: readonly ProviderView[] = s.providers): SettingsModelRoute[] {
  const out: SettingsModelRoute[] = [];
  const seen = new Set<string>();
  for (const p of providers) {
    if (!p.added || !providerIsConfigured(p)) continue;
    const billingSource = p.billingSource || (p.builtIn ? "subscription_pool" : "newapi_byok");
    if (billingSource !== "subscription_pool" && billingSource !== "newapi_byok") continue;
    for (const m of p.chatModels ?? p.models) {
      const ref = `${p.name}/${m}`;
      const identity = `${ref}\u0000${billingSource}`;
      if (seen.has(identity)) continue;
      seen.add(identity);
      out.push({ ref, billingSource });
    }
  }
  return out;
}

// allRefs flattens providers into "provider/model" refs for legacy callers.
export function modelRefsForSettings(s: SettingsView, providers: readonly ProviderView[] = s.providers): string[] {
  return modelRoutesForSettings(s, providers).map((route) => route.ref);
}

function refsForBillingSource(routes: readonly SettingsModelRoute[], billingSource: string): string[] {
  return routes.filter((route) => route.billingSource === billingSource).map((route) => route.ref);
}

export function billingSourceForRef(s: SettingsView, ref: string, providers: readonly ProviderView[] = s.providers): string {
  const separator = ref.indexOf("/");
  if (separator <= 0 || !ref.slice(separator + 1)) return "";
  const providerName = ref.slice(0, separator);
  const sources = new Set(
    providers
      .filter((item) => item.name === providerName)
      .map((item) => item.billingSource || (item.builtIn ? "subscription_pool" : "newapi_byok"))
      .filter((source) => source === "subscription_pool" || source === "newapi_byok"),
  );
  return sources.size === 1 ? [...sources][0] : "";
}

// EFFORT_PRESETS is the canonical union of /effort levels the kernel
// recognises. The settings UI exposes these as toggleable checkboxes; users
// can additionally add arbitrary custom names via the "Add" input. The order
// here is what the user sees in the dropdown.
const EFFORT_PRESETS: readonly string[] = ["low", "medium", "high", "xhigh", "max"];
const REASONING_PROTOCOLS: readonly string[] = ["", "deepseek", "openai", "none"];
export function normalizeReasoningProtocol(protocol: string | undefined): string {
  return REASONING_PROTOCOLS.includes(protocol ?? "") ? protocol ?? "" : "";
}

export function normalizeReasoningLanguage(lang: string | undefined): string {
  const v = String(lang ?? "").trim().toLowerCase();
  return v === "zh" || v === "en" ? v : "auto";
}

function reasoningProtocolLabel(protocol: string, t: ReturnType<typeof useT>): string {
  switch (protocol) {
    case "deepseek":
      return t("settings.reasoningProtocol.deepseek");
    case "openai":
      return t("settings.reasoningProtocol.openai");
    case "none":
      return t("settings.reasoningProtocol.none");
    default:
      return t("settings.reasoningProtocol.auto");
  }
}


function StepLimitControl({
  value,
  presets,
  busy,
  onChange,
}: {
  value: number;
  presets: number[];
  busy: boolean;
  onChange: (value: number) => void;
}) {
  const t = useT();
  const normalized = normalizeStepLimit(value);
  const presetSet = new Set(presets.map(normalizeStepLimit));
  const [custom, setCustom] = useState(String(normalized));
  useEffect(() => setCustom(String(normalized)), [normalized]);
  const isCustom = !presetSet.has(normalized);
  const commitCustom = () => {
    const next = normalizeStepLimit(Number(custom));
    setCustom(String(next));
    if (next !== normalized) onChange(next);
  };
  return (
    <div className="step-limit-control">
      <div className="set-seg">
        {presets.map((preset) => {
          const n = normalizeStepLimit(preset);
          return (
            <button
              key={n}
              type="button"
              className={`set-seg__btn${normalized === n ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => n !== normalized && onChange(n)}
            >
              {stepLimitLabel(n, t)}
            </button>
          );
        })}
        <button
          type="button"
          className={`set-seg__btn${isCustom ? " set-seg__btn--on" : ""}`}
          disabled={busy}
          onClick={() => {
            if (!isCustom) setCustom(String(normalized || 12));
          }}
        >
          {t("settings.stepLimit.custom")}
        </button>
      </div>
      <input
        className="mem-input step-limit-control__custom"
        value={custom}
        disabled={busy}
        inputMode="numeric"
        aria-label={t("settings.stepLimit.custom")}
        onChange={(e) => setCustom(e.target.value.replace(/[^\d]/g, ""))}
        onBlur={commitCustom}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
        }}
      />
    </div>
  );
}
function normalizeStepLimit(value: number): number {
  return Number.isFinite(value) && value > 0 ? Math.trunc(value) : 0;
}

function stepLimitLabel(value: number, t: ReturnType<typeof useT>): string {
  return value === 0 ? t("settings.stepLimit.unlimited") : String(value);
}


export function ModelsSettingsSection({
  s,
  modelProviders,
  busy,
  apply,
  modelRoutes,
  modelOptionsLoading,
  subscriptionAccountState,
}: ModelsSettingsSectionProps) {
  const t = useT();
  const [subtab, setSubtab] = useState<"usage" | "access">("usage");
  const refs = useMemo(() => modelRoutes.map((route) => route.ref), [modelRoutes]);
  const modelSettings = useMemo(() => ({ ...s, providers: modelProviders }), [modelProviders, s]);
  const routeSourcesForRef = (ref: string): string[] => modelRoutes.filter((route) => route.ref === ref).map((route) => route.billingSource);
  const derivedRefs = useMemo(
    () => refsForBillingSource(modelRoutes, s.defaultBillingSource || billingSourceForRef(modelSettings, s.defaultModel)),
    [modelRoutes, modelSettings, s.defaultBillingSource, s.defaultModel],
  );
  const derivedRoutes = useMemo(
    () => modelRoutes.filter((route) => route.billingSource === (s.defaultBillingSource || billingSourceForRef(modelSettings, s.defaultModel))),
    [modelRoutes, modelSettings, s.defaultBillingSource, s.defaultModel],
  );
  const defaultRef = toRef(s.defaultModel, modelSettings);
  const plannerRef = toRef(s.plannerModel, modelSettings);
  const subagentRef = toRef(s.subagentModel, modelSettings);
  const plannerSelectRef = s.plannerModel.trim() === "" ? "" : plannerRef;
  const defaultBillingSource = s.defaultBillingSource || billingSourceForRef(modelSettings, defaultRef) || routeSourcesForRef(defaultRef)[0] || "";
  const anonymousSubscriptionRef = (ref: string) => (
    subscriptionAccountState === "anonymous"
    && (
      (ref === defaultRef && defaultBillingSource === "subscription_pool")
      || (routeSourcesForRef(ref).length === 1 && routeSourcesForRef(ref)[0] === "subscription_pool")
    )
  );
  const anonymousDefault = anonymousSubscriptionRef(defaultRef);
  const plannerSelectionState = derivedModelSelectionState({
    loading: modelOptionsLoading,
    inheritsDefault: plannerSelectRef === "",
    selectedRef: plannerRef,
    defaultRef,
    availableRefs: derivedRefs,
  });
  const subagentSelectionState = derivedModelSelectionState({
    loading: modelOptionsLoading,
    inheritsDefault: subagentRef === "",
    selectedRef: subagentRef,
    defaultRef,
    availableRefs: derivedRefs,
  });
  const [defaultProvider] = defaultRef.split("/");
  const defaultProviderView = modelSettings.providers.find((p) => (
    p.name === defaultProvider
    && (p.billingSource || (p.builtIn ? "subscription_pool" : "newapi_byok")) === defaultBillingSource
  )) ?? modelSettings.providers.find((p) => p.name === defaultProvider);
  const modelIssue = (() => {
    if (modelOptionsLoading || anonymousDefault) return "";
    if (defaultBillingSource === "subscription_pool") {
      const selectedSummary = s.selectedModelSummary;
      const summaryMatches = (
        selectedSummary?.ref === defaultRef
        && selectedSummary.billingSource === "subscription_pool"
      );
      const ready = (
        routeSourcesForRef(defaultRef).includes("subscription_pool")
        || (
          summaryMatches
          && selectedSummary.accountState === "ready"
          && selectedSummary.availability === "ready"
        )
      );
      const unavailable = (
        summaryMatches
        && selectedSummary.accountState === "ready"
        && selectedSummary.availability === "unavailable"
      );
      return !ready && unavailable
        ? t("settings.modelUnavailable", { ref: defaultRef || t("common.none") })
        : "";
    }
    if (!defaultProviderView) return t("settings.modelUnavailable", { ref: defaultRef || t("common.none") });
    return providerIsConfigured(defaultProviderView)
      ? ""
      : t("settings.modelNeedsKey", { provider: modelProviderLabel(defaultProvider, defaultProviderView, t) });
  })();
  const agent = s.agent ?? { temperature: 0, maxSteps: 0, plannerMaxSteps: 0, systemPrompt: "", coldResumePrune: true, reasoningLanguage: "auto" };
  const setAgentSteps = (maxSteps: number, plannerMaxSteps: number) => (
    app.SetAgentParams(agent.temperature, maxSteps, plannerMaxSteps, agent.systemPrompt)
  );

  return (
    <>
      <div className="settings-subtabs">
        <button
          type="button"
          className={`settings-subtab${subtab === "usage" ? " settings-subtab--active" : ""}`}
          aria-selected={subtab === "usage"}
          onClick={() => setSubtab("usage")}
        >
          {t("settings.modelTab.usage")}
        </button>
        <button
          type="button"
          className={`settings-subtab${subtab === "access" ? " settings-subtab--active" : ""}`}
          aria-selected={subtab === "access"}
          onClick={() => setSubtab("access")}
        >
          {t("settings.modelTab.access")}
        </button>
      </div>

      {subtab === "usage" ? (
        <>
          <SettingsSection title={t("settings.modelUsage")}>
            <SettingsField label={t("settings.defaultModel")} hint={t("settings.defaultModelHint")}>
              <ModelPicker
                s={modelSettings}
                refs={refs}
                routes={modelRoutes}
                value={defaultRef}
                selectedBillingSource={defaultBillingSource}
                disabled={busy}
                optionsLoading={modelOptionsLoading}
                unavailableValueLabel={anonymousDefault ? t("settings.anonymousDefaultModel") : undefined}
                unavailableValueMeta={anonymousDefault ? t("settings.anonymousSubscriptionModelHint") : undefined}
                onPick={(ref, billingSource) => {
                  if (!billingSource) return;
                  void apply(() => app.SetDefaultModelRoute(ref, billingSource));
                }}
              />
            </SettingsField>

            <SettingsField label={t("settings.plannerModel")}>
              <ModelPicker
                s={modelSettings}
                refs={derivedRefs}
                routes={derivedRoutes}
                value={plannerSelectRef}
                disabled={busy}
                optionsLoading={modelOptionsLoading}
                includeSameDefault
                selectionState={plannerSelectionState}
                unavailableValueLabel={anonymousSubscriptionRef(plannerSelectRef) ? t("settings.anonymousSubscriptionModel") : undefined}
                unavailableValueMeta={anonymousSubscriptionRef(plannerSelectRef) ? t("settings.anonymousSubscriptionModelHint") : undefined}
                onPick={(ref) => void apply(() => app.SetPlannerModel(ref))}
              />
            </SettingsField>

            <SettingsField label={t("settings.subagentModel")}>
              <ModelPicker
                s={modelSettings}
                refs={derivedRefs}
                routes={derivedRoutes}
                value={subagentRef}
                disabled={busy}
                optionsLoading={modelOptionsLoading}
                emptyOptionLabel={t("settings.subagentModelDefault")}
                emptyOptionHint={t("common.auto")}
                selectionState={subagentSelectionState}
                unavailableValueLabel={anonymousSubscriptionRef(subagentRef) ? t("settings.anonymousSubscriptionModel") : undefined}
                unavailableValueMeta={anonymousSubscriptionRef(subagentRef) ? t("settings.anonymousSubscriptionModelHint") : undefined}
                onPick={(ref) => void apply(() => app.SetSubagentModel(ref))}
              />
            </SettingsField>

            <SettingsField label={t("settings.subagentEffort")} hint={t("settings.subagentHint")}>
              <div className="set-seg subagent-effort-control" role="group" aria-label={t("settings.subagentEffort")}>
                {(["", ...EFFORT_PRESETS] as const).map((level) => {
                  const selected = (s.subagentEffort || "") === level;
                  return (
                    <button
                      key={level || "auto"}
                      type="button"
                      className={`set-seg__btn${selected ? " set-seg__btn--on" : ""}`}
                      aria-pressed={selected}
                      disabled={busy}
                      onClick={() => !selected && void apply(() => app.SetSubagentEffort(level))}
                    >
                      {level || t("settings.subagentEffortDefault")}
                    </button>
                  );
                })}
              </div>
            </SettingsField>

            {modelIssue && <div className="provider-fetch-banner provider-fetch-banner--warn">{modelIssue}</div>}
          </SettingsSection>
          <SettingsSection title={t("settings.agentRuntime")} description={t("settings.agentRuntimeHint")}>
            <SettingsField label={t("settings.executorMaxSteps")} hint={t("settings.executorMaxStepsHint")}>
              <StepLimitControl
                value={agent.maxSteps}
                presets={[10, 25, 50, 0]}
                busy={busy}
                onChange={(next) => void apply(() => setAgentSteps(next, agent.plannerMaxSteps))}
              />
            </SettingsField>
            <SettingsField label={t("settings.plannerMaxSteps")} hint={plannerSelectRef ? t("settings.plannerMaxStepsHint") : t("settings.plannerMaxStepsDisabledHint")}>
              <StepLimitControl
                value={agent.plannerMaxSteps}
                presets={[6, 12, 25, 0]}
                busy={busy}
                onChange={(next) => void apply(() => setAgentSteps(agent.maxSteps, next))}
              />
            </SettingsField>
            <SettingsField label={t("settings.coldResumePrune")} hint={t("settings.coldResumePruneHint")}>
              <div className="set-seg">
                {([true, false] as const).map((on) => (
                  <button
                    key={on ? "on" : "off"}
                    className={`set-seg__btn${agent.coldResumePrune === on ? " set-seg__btn--on" : ""}`}
                    disabled={busy}
                    onClick={() => void apply(() => app.SetColdResumePrune(on))}
                  >
                    {on ? t("settings.coldResumePrune.on") : t("settings.coldResumePrune.off")}
                  </button>
                ))}
              </div>
            </SettingsField>
            <SettingsField label={t("settings.reasoningLanguage")} hint={t("settings.reasoningLanguageHint")}>
              <div className="set-seg">
                {(["auto", "zh", "en"] as const).map((lang) => (
                  <button
                    key={lang}
                    className={`set-seg__btn${agent.reasoningLanguage === lang ? " set-seg__btn--on" : ""}`}
                    disabled={busy}
                    onClick={() => void apply(() => app.SetReasoningLanguage(lang))}
                  >
                    {t(`settings.reasoningLanguage.${lang}`)}
                  </button>
                ))}
              </div>
            </SettingsField>
          </SettingsSection>
        </>
      ) : (
        <ProvidersSection
          s={s}
          modelProviders={modelProviders}
          busy={busy}
          apply={apply}
          modelRoutes={modelRoutes}
          modelOptionsLoading={modelOptionsLoading}
        />
      )}
    </>
  );
}

function providerApiKeyEnvError(apiKeyEnv: string, t: ReturnType<typeof useT>): string {
  const key = apiKeyEnv.trim();
  if (!key || isProviderApiKeyEnvName(key)) return "";
  return t("settings.providerApiKeyEnvInvalid");
}

function providerRefreshReplacesCurrent(p: ProviderView): boolean {
  return p.builtIn && (p.name === BRAND_PROVIDER_ID || p.name === LEGACY_PROVIDER_ID);
}


function ProvidersSection({
  s,
  modelProviders,
  busy,
  apply,
  modelRoutes,
  modelOptionsLoading,
}: SectionProps & {
  modelProviders: ProviderView[];
  modelRoutes: SettingsModelRoute[];
  modelOptionsLoading: boolean;
}) {
  const t = useT();
  const defaultProvider = toRef(s.defaultModel, s).split("/")[0];
  const [editing, setEditing] = useState<string | null>(null);
  const [adding, setAdding] = useState<AddProviderMode>(null);
  const [fetchingProvider, setFetchingProvider] = useState<string | null>(null);
  const [fetchResults, setFetchResults] = useState<Record<string, ProviderFetchResult>>({});
  const [modelDrafts, setModelDrafts] = useState<Record<string, ProviderModelDraft>>({});
  const groups = useMemo(() => providerAccessGroups(modelProviders.filter((p) => p.added), t), [modelProviders, t]);
  const allowedSubscriptionModels = useMemo(() => new Set(
    modelRoutes
      .filter((route) => route.billingSource === "subscription_pool")
      .map((route) => {
        const separator = route.ref.indexOf("/");
        return separator > 0 ? route.ref.slice(separator + 1) : "";
      })
      .filter(Boolean),
  ), [modelRoutes]);

  const setGroupFetchResult = (groupID: string, result: ProviderFetchResult | null) => {
    setFetchResults((prev) => {
      const next = { ...prev };
      if (result) next[groupID] = result;
      else delete next[groupID];
      return next;
    });
  };

  const setGroupModelDraft = (groupID: string, draft: ProviderModelDraft | null) => {
    setModelDrafts((prev) => {
      const next = { ...prev };
      if (draft) next[groupID] = draft;
      else delete next[groupID];
      return next;
    });
  };

  const modelDraftForFetch = (p: ProviderView, fetched: string[]): ProviderModelDraft => {
    const replaceCurrent = providerRefreshReplacesCurrent(p);
    const candidates = providerModelCandidates(p.models, fetched, { replaceCurrent });
    const selected = mergedFetchedProviderModels(p.models, fetched, { preserveCurated: true, replaceCurrent });
    const visionSource = p.visionModelsConfigured ? p.visionModels : inferredVisionModels(candidates);
    return {
      providerName: p.name,
      candidates,
      selected: candidates.filter((model) => selected.includes(model)),
      visionModels: candidates.filter((model) => visionSource.includes(model)),
    };
  };

  const updateModelDraftSelection = (groupID: string, nextSelected: (draft: ProviderModelDraft) => string[]) => {
    setModelDrafts((prev) => {
      const draft = prev[groupID];
      if (!draft) return prev;
      const selectedSet = new Set(nextSelected(draft));
      return {
        ...prev,
        [groupID]: {
          ...draft,
          selected: draft.candidates.filter((model) => selectedSet.has(model)),
        },
      };
    });
  };

  const toggleModelDraftVision = (groupID: string, model: string) => {
    setModelDrafts((prev) => {
      const draft = prev[groupID];
      if (!draft) return prev;
      return {
        ...prev,
        [groupID]: {
          ...draft,
          visionModels: draft.visionModels.includes(model)
            ? draft.visionModels.filter((candidate) => candidate !== model)
            : draft.candidates.filter((candidate) => candidate === model || draft.visionModels.includes(candidate)),
        },
      };
    });
  };

  const refreshModels = async (group: ProviderAccessGroup, p: ProviderView) => {
    setFetchingProvider(group.id);
    setGroupFetchResult(group.id, null);
    setGroupModelDraft(group.id, null);
    try {
      let fetched: string[];
      try {
        fetched = await app.FetchProviderModels(p);
      } catch (e) {
        setGroupFetchResult(group.id, {
          kind: "warn",
          text: t("settings.fetchModelsFailedForProvider", {
            provider: group.label,
            err: reportClientError("models.fetch", e, {
              fallbackMessage: "请检查模型服务配置后重试。",
            }).userMessage,
          }),
        });
        return;
      }
      if (fetched.length === 0) {
        setGroupFetchResult(group.id, {
          kind: "warn",
          text: t("settings.fetchModelsEmptyForProvider", { provider: group.label }),
        });
        return;
      }
      const draft = modelDraftForFetch(p, fetched);
      setGroupModelDraft(group.id, draft);
      setGroupFetchResult(group.id, {
        kind: "ok",
        text: t("settings.fetchModelsReadyForProvider", { provider: group.label, n: draft.candidates.length }),
      });
    } finally {
      setFetchingProvider(null);
    }
  };

  const refreshGroup = async (group: ProviderAccessGroup) => {
    const probe = group.providers[0];
    if (!probe) return;
    await refreshModels(group, probe);
  };

  const saveKeyEnvAndAutoRefresh = async (group: ProviderAccessGroup, apiKeyEnv: string, value: string) => {
    const probe = group.providers[0];
    if (!probe || !apiKeyEnv) return;
    setFetchingProvider(group.id);
    setGroupFetchResult(group.id, null);
    setGroupModelDraft(group.id, null);
    try {
      await apply(async () => {
        await app.SetProviderKeyConfigOnly(apiKeyEnv, value);
        try {
          const fetched = await app.FetchProviderModels({ ...probe, apiKeyEnv });
          if (fetched.length > 0) {
            const draft = modelDraftForFetch({ ...probe, apiKeyEnv }, fetched);
            setGroupModelDraft(group.id, draft);
            setGroupFetchResult(group.id, {
              kind: "ok",
              text: t("settings.fetchModelsReadyForProvider", { provider: group.label, n: draft.candidates.length }),
            });
            return;
          }
          setGroupFetchResult(group.id, {
            kind: "warn",
            text: t("settings.fetchModelsEmptyForProvider", { provider: group.label }),
          });
        } catch (e) {
          setGroupFetchResult(group.id, {
            kind: "warn",
            text: t("settings.fetchModelsAfterKeyFailedForProvider", {
              provider: group.label,
              err: reportClientError("models.fetchAfterKey", e, {
                fallbackMessage: "密钥已保存，但模型列表加载失败，请稍后重试。",
              }).userMessage,
            }),
          });
        }
      });
    } finally {
      setFetchingProvider(null);
    }
  };

  const saveProviderKey = async (group: ProviderAccessGroup, apiKeyEnv: string, value: string) => {
    if (!apiKeyEnv) return;
    setGroupFetchResult(group.id, null);
    setGroupModelDraft(group.id, null);
    await apply(() => app.SetProviderKey(apiKeyEnv, value));
  };

  const clearProviderKey = async (apiKeyEnv: string) => {
    if (!apiKeyEnv) return;
    await apply(() => app.ClearProviderKey(apiKeyEnv));
  };

  const clearProviderKeyConfigOnly = async (apiKeyEnv: string) => {
    if (!apiKeyEnv) return;
    await apply(() => app.ClearProviderKeyConfigOnly(apiKeyEnv));
  };

  const saveModelDraft = async (group: ProviderAccessGroup) => {
    const draft = modelDrafts[group.id];
    const provider = draft ? group.providers.find((p) => p.name === draft.providerName) : null;
    const models = uniqueStrings(draft?.selected ?? []);
    const visionModels = uniqueStrings(draft?.visionModels ?? []).filter((model) => models.includes(model));
    if (!draft || !provider || models.length === 0) return;
    let saved = false;
    await apply(async () => {
      await app.SaveProviderConfigOnly({
        ...provider,
        models,
        visionModels,
        visionModelsConfigured: true,
        default: providerDefaultModel(provider.default, models),
      });
      saved = true;
    });
    if (!saved) return;
    setGroupModelDraft(group.id, null);
    setGroupFetchResult(group.id, {
      kind: "ok",
      text: t("settings.enabledModelsSavedForProvider", { provider: group.label, n: models.length }),
    });
  };

  return (
    <SettingsSection
      title={t("settings.providerAccess")}
      description={t("settings.providerAccessHint")}
      actions={
        <button className="btn btn--small" disabled={busy || adding !== null} onClick={() => setAdding("custom")}>
          {t("settings.addProvider")}
        </button>
      }
    >
      <div className="provider-access-grid">
        {groups.length === 0 && adding === null && (
          <div className="provider-empty">
            <strong>{t("settings.providerAccessEmptyTitle")}</strong>
            <span>{t("settings.providerAccessEmptyHint")}</span>
            <div className="provider-empty__actions">
              <button type="button" className="btn btn--small" disabled={busy} onClick={() => setAdding("custom")}>
                {t("settings.addProvider.customChoice")}
              </button>
            </div>
          </div>
        )}
        {adding !== null && (
          <AddProviderPanel
            kinds={s.providerKinds}
            existingProviderNames={s.providers.map((p) => p.name)}
            busy={busy}
            onCancel={() => setAdding(null)}
            onAddCustom={(pv) => apply(() => app.SaveProviderConfigOnly(pv)).then(() => setAdding(null))}
          />
        )}
        {adding === null && groups.map((group) => (
          <ProviderAccessCard
            key={group.id}
            group={group}
            busy={busy}
            fetching={fetchingProvider === group.id || group.providers.some((p) => fetchingProvider === p.name)}
            fetchResult={fetchResults[group.id]}
            modelDraft={modelDrafts[group.id]}
            allowedSubscriptionModels={allowedSubscriptionModels}
            subscriptionModelsLoading={modelOptionsLoading}
            defaultProvider={defaultProvider}
            editing={editing}
            kinds={s.providerKinds}
            onEdit={setEditing}
            onCancelEdit={() => setEditing(null)}
            onSave={(pv) => apply(() => app.SaveProviderConfigOnly(pv)).then(() => {
              setEditing(null);
              setGroupModelDraft(group.id, null);
            })}
            onRefresh={() => {
              if (group.locked) {
                void apply(() => app.RefreshOfficialModels());
                return;
              }
              void refreshGroup(group);
            }}
            onToggleDraftModel={(model) => updateModelDraftSelection(group.id, (draft) => (
              draft.selected.includes(model)
                ? draft.selected.filter((candidate) => candidate !== model)
                : [...draft.selected, model]
            ))}
            onToggleDraftVision={(model) => toggleModelDraftVision(group.id, model)}
            onSelectAllDraftModels={() => updateModelDraftSelection(group.id, (draft) => draft.candidates)}
            onClearDraftModels={() => updateModelDraftSelection(group.id, () => [])}
            onCancelDraftModels={() => setGroupModelDraft(group.id, null)}
            onSaveDraftModels={() => void saveModelDraft(group)}
            onSaveEditorKey={(env, value) => group.builtIn ? saveProviderKey(group, env, value) : saveKeyEnvAndAutoRefresh(group, env, value)}
            onClearEditorKey={group.builtIn ? clearProviderKey : clearProviderKeyConfigOnly}
            onDelete={(p) => apply(() => app.RemoveProviderAccess(p.name))}
          />
        ))}
      </div>
    </SettingsSection>
  );
}

type ProviderAccessGroup = {
  id: string;
  label: string;
  description: string;
  builtIn: boolean;
  providers: ProviderView[];
  apiKeyEnv: string;
  keySet: boolean;
  requiresKey: boolean;
  configured: boolean;
  keySource?: string;
  keySourcePath?: string;
  baseUrl: string;
  kind: string;
  models: string[];
  locked: boolean;
  availability?: string;
};

type ProviderFetchResult = {
  kind: "ok" | "warn";
  text: string;
};

type ProviderModelDraft = {
  providerName: string;
  candidates: string[];
  selected: string[];
  visionModels: string[];
};

type AddProviderMode = null | "custom";

function AddProviderPanel({
  kinds,
  existingProviderNames,
  busy,
  onCancel,
  onAddCustom,
}: {
  kinds: string[];
  existingProviderNames: string[];
  busy: boolean;
  onCancel: () => void;
  onAddCustom: (p: ProviderView) => void | Promise<void>;
}) {
  const t = useT();
  return (
    <div className="provider-add-panel">
      <div className="provider-add-panel__head">
        <div>
          <strong>{t("settings.addProvider.chooseTitle")}</strong>
          <span>{t("settings.addProvider.chooseHint")}</span>
        </div>
        <button type="button" className="btn btn--small" disabled={busy} onClick={onCancel}>
          {t("common.cancel")}
        </button>
      </div>
      <div className="provider-add-panel__hint">{t("settings.addProvider.customHint")}</div>
      <ProviderEditor
        kinds={kinds}
        existingProviderNames={existingProviderNames}
        busy={busy}
        onCancel={onCancel}
        onSave={onAddCustom}
      />
    </div>
  );
}

function ProviderAccessCard({
  group,
  busy,
  fetching,
  fetchResult,
  modelDraft,
  allowedSubscriptionModels,
  subscriptionModelsLoading,
  defaultProvider,
  editing,
  kinds,
  onEdit,
  onCancelEdit,
  onSave,
  onRefresh,
  onToggleDraftModel,
  onToggleDraftVision,
  onSelectAllDraftModels,
  onClearDraftModels,
  onCancelDraftModels,
  onSaveDraftModels,
  onSaveEditorKey,
  onClearEditorKey,
  onDelete,
}: {
  group: ProviderAccessGroup;
  busy: boolean;
  fetching: boolean;
  fetchResult?: ProviderFetchResult;
  modelDraft?: ProviderModelDraft;
  allowedSubscriptionModels: ReadonlySet<string>;
  subscriptionModelsLoading: boolean;
  defaultProvider: string;
  editing: string | null;
  kinds: string[];
  onEdit: (name: string) => void;
  onCancelEdit: () => void;
  onSave: (p: ProviderView) => void | Promise<void>;
  onRefresh: () => void;
  onToggleDraftModel: (model: string) => void;
  onToggleDraftVision: (model: string) => void;
  onSelectAllDraftModels: () => void;
  onClearDraftModels: () => void;
  onCancelDraftModels: () => void;
  onSaveDraftModels: () => void;
  onSaveEditorKey: (apiKeyEnv: string, value: string) => Promise<void>;
  onClearEditorKey?: (apiKeyEnv: string) => Promise<void>;
  onDelete?: (p: ProviderView) => Promise<void>;
}) {
  const t = useT();
  const editableProvider = group.providers[0];
  const isDefault = group.providers.some((p) => p.name === defaultProvider);
  const editingProvider = group.providers.find((p) => editing === p.name);
  const primaryProviderExpanded = Boolean(editableProvider && editing === editableProvider.name);
  const subscriptionGroup = group.providers.some((provider) => provider.billingSource === "subscription_pool");
  const displayedModels = subscriptionGroup
    ? group.models.filter((model) => allowedSubscriptionModels.has(model))
    : group.models;
  const settledDisplayedModels = subscriptionGroup && subscriptionModelsLoading ? [] : displayedModels;
  const visibleModels = settledDisplayedModels.slice(0, 6);
  const hiddenModelCount = Math.max(0, settledDisplayedModels.length - visibleModels.length);
  const modelListLabel = subscriptionGroup
    ? t("settings.currentPlanModels")
    : t(group.configured ? "settings.enabledModels" : "settings.modelList");
  const emptyModelListLabel = subscriptionGroup
    ? subscriptionModelsLoading
      ? t("settings.subscriptionModelsLoading")
      : t("settings.subscriptionModelsUnavailable")
    : t("settings.noModelsConfigured");
  return (
    <article className={`provider-access-card${group.builtIn ? " provider-access-card--builtin" : ""}`}>
      <div className="provider-access-card__head">
        <div className="provider-access-card__identity">
          <div className="provider-access-card__title">
            {group.label}
            <span className={`badge ${group.builtIn ? "badge--project" : "badge--neutral"}`}>
              {group.builtIn ? t("settings.builtinProviderBadge") : t("settings.customProviderBadge")}
            </span>
            <span className={`badge ${group.keySet ? "badge--project" : "badge--feedback"}`}>
              {providerKeyStatusLabel(group, t)}
            </span>
          </div>
          <div className="provider-access-card__desc">{group.description}</div>
        </div>
        <div className="provider-access-card__actions">
          {editableProvider && !group.locked && (
            <button
              className="btn btn--small"
              disabled={busy}
              aria-expanded={primaryProviderExpanded}
              onClick={() => primaryProviderExpanded ? onCancelEdit() : onEdit(editableProvider.name)}
            >
              {primaryProviderExpanded ? t("common.collapse") : t("settings.configureProvider")}
            </button>
          )}
          <button
            className="btn btn--small"
            disabled={busy || fetching || (!group.locked && !group.baseUrl) || (!group.locked && !group.configured)}
            onClick={onRefresh}
          >
            {fetching ? t("settings.fetchingModels") : t("settings.fetchModels")}
          </button>
          {editableProvider && onDelete && !group.locked && (
            isDefault && !group.builtIn ? (
              <Tooltip label={t("settings.cantDeleteDefault")}>
                <button className="btn btn--small" disabled>{t("settings.removeProviderAccess")}</button>
              </Tooltip>
            ) : (
              <InlineConfirmButton
                label={t("settings.removeProviderAccess")}
                confirmLabel={group.builtIn ? t("settings.confirmRemoveProviderAccess") : t("settings.confirmDeleteProvider")}
                cancelLabel={t("common.cancel")}
                disabled={busy}
                danger={!group.builtIn}
                onConfirm={() => onDelete(editableProvider)}
              />
            )
          )}
        </div>
      </div>

      <div className="provider-access-meta">
        <span>{group.kind}</span>
        {!group.builtIn && group.baseUrl && <span>{group.baseUrl}</span>}
        <span>{group.locked ? t("settings.pointsChannel") : (group.apiKeyEnv || t("common.none"))}</span>
        {group.keySource && <span title={group.keySourcePath || undefined}>{t("settings.keySource", { source: group.keySource })}</span>}
      </div>

      <div className="provider-card-block">
        <div className="provider-card-block__label">{modelListLabel}</div>
        <div className="provider-model-chips" aria-label={modelListLabel}>
          {visibleModels.length > 0 ? visibleModels.map((model) => (
            <span className="provider-model-chip" key={model}>
              {model}
            </span>
          )) : <span className="provider-model-chip provider-model-chip--empty">{emptyModelListLabel}</span>}
          {hiddenModelCount > 0 && (
            <span className="provider-model-chip provider-model-chip--more">
              {t("settings.moreModels", { n: hiddenModelCount })}
            </span>
          )}
        </div>
        {!group.configured && group.requiresKey && (
          <div className="provider-card-status provider-card-status--warn">
            {t("settings.modelsRequireKey")}
          </div>
        )}
        {group.locked && !group.configured && (
          <div className="provider-card-status provider-card-status--warn">
            {t("settings.pointsChannelUnavailable")}
          </div>
        )}
        {fetchResult && (
          <div className={`provider-card-status provider-card-status--${fetchResult.kind}`}>
            {fetchResult.text}
          </div>
        )}
      </div>

      {modelDraft && (
        <ProviderModelDraftPicker
          draft={modelDraft}
          busy={busy}
          fetching={fetching}
          onToggle={onToggleDraftModel}
          onToggleVision={onToggleDraftVision}
          onSelectAll={onSelectAllDraftModels}
          onClear={onClearDraftModels}
          onCancel={onCancelDraftModels}
          onSave={onSaveDraftModels}
        />
      )}

      {group.providers.length > 1 && !group.locked && (
        <div className="provider-profiles">
          {group.providers.map((p) => {
            const profileExpanded = editing === p.name;
            return (
              <div className="provider-profile-row" key={p.name}>
                <span>{p.name}</span>
                <span>{p.models.join(", ") || t("common.none")}</span>
                <button
                  className="btn btn--small"
                  disabled={busy}
                  aria-expanded={profileExpanded}
                  onClick={() => profileExpanded ? onCancelEdit() : onEdit(p.name)}
                >
                  {profileExpanded ? t("common.collapse") : t("settings.configureProfile")}
                </button>
              </div>
            );
          })}
        </div>
      )}

      {editingProvider && !group.locked && (
        <ProviderEditor
          initial={editingProvider}
          kinds={kinds}
          busy={busy}
          onCancel={onCancelEdit}
          onSave={onSave}
          onSaveKey={onSaveEditorKey}
          onClearKey={onClearEditorKey}
        />
      )}
    </article>
  );
}

function ProviderModelDraftPicker({
  draft,
  busy,
  fetching,
  onToggle,
  onToggleVision,
  onSelectAll,
  onClear,
  onCancel,
  onSave,
}: {
  draft: ProviderModelDraft;
  busy: boolean;
  fetching: boolean;
  onToggle: (model: string) => void;
  onToggleVision: (model: string) => void;
  onSelectAll: () => void;
  onClear: () => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  const t = useT();
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  // Debounce search to avoid expensive filtering on every keystroke
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 150);
    return () => clearTimeout(timer);
  }, [query]);
  const selected = new Set(draft.selected);
  const vision = new Set(draft.visionModels);
  const q = debouncedQuery.trim().toLowerCase();
  const visibleCandidates = q
    ? draft.candidates.filter((model) => model.toLowerCase().includes(q))
    : draft.candidates;
  const disabled = busy || fetching;

  return (
    <div className="provider-model-draft">
      <div className="provider-model-draft__head">
        <div>
          <div className="provider-card-block__label">{t("settings.modelCandidates")}</div>
          <span>{t("settings.modelCandidatesSelected", { n: draft.selected.length })}</span>
        </div>
        <div className="provider-model-draft__tools">
          <button type="button" className="btn btn--small" disabled={disabled || draft.selected.length === draft.candidates.length} onClick={onSelectAll}>
            {t("settings.selectAllModels")}
          </button>
          <button type="button" className="btn btn--small" disabled={disabled || draft.selected.length === 0} onClick={onClear}>
            {t("settings.clearModelSelection")}
          </button>
        </div>
      </div>
      <input
        className="mem-input provider-model-draft__search"
        placeholder={t("settings.modelCandidateSearch")}
        value={query}
        disabled={disabled}
        onChange={(e) => setQuery(e.target.value)}
      />
      <div className="provider-model-draft__list" role="list" aria-label={t("settings.modelCandidates")}>
        {visibleCandidates.length > 0 ? visibleCandidates.map((model) => {
          const enabled = selected.has(model);
          return (
            <div className="provider-model-draft__option" key={model}>
              <label className="provider-model-draft__model">
                <input
                  type="checkbox"
                  checked={enabled}
                  disabled={disabled}
                  onChange={() => onToggle(model)}
                />
                <span>{model}</span>
              </label>
              <label className="provider-model-draft__vision">
                <input
                  type="checkbox"
                  checked={enabled && vision.has(model)}
                  disabled={disabled || !enabled}
                  onChange={() => onToggleVision(model)}
                />
                <span>{t("settings.visionModel")}</span>
              </label>
            </div>
          );
        }) : (
          <div className="provider-model-draft__empty">{t("settings.noMatchingCandidateModels")}</div>
        )}
      </div>
      <div className="provider-model-draft__actions">
        <button type="button" className="btn btn--small" disabled={disabled} onClick={onCancel}>
          {t("common.cancel")}
        </button>
        <button type="button" className="btn btn--primary btn--small" disabled={disabled || draft.selected.length === 0} onClick={onSave}>
          {t("settings.saveEnabledModels")}
        </button>
      </div>
    </div>
  );
}

function providerAccessGroups(providers: ProviderView[], t: ReturnType<typeof useT>): ProviderAccessGroup[] {
  const groups = new Map<string, ProviderAccessGroup>();
  for (const p of providers) {
    const id = providerGroupID(p);
    const builtIn = id.startsWith("builtin:");
    const existing = groups.get(id);
    if (existing) {
      existing.providers.push(p);
      existing.keySet = existing.keySet || p.keySet;
      existing.requiresKey = existing.requiresKey && providerRequiresKey(p);
      existing.configured = existing.configured || providerIsConfigured(p);
      if (!existing.keySource && p.keySource) existing.keySource = p.keySource;
      if (!existing.keySourcePath && p.keySourcePath) existing.keySourcePath = p.keySourcePath;
      existing.models = uniqueStrings([...existing.models, ...p.models]);
      existing.locked = existing.locked || Boolean(p.locked);
      if (!existing.availability && p.availability) existing.availability = p.availability;
      continue;
    }
    groups.set(id, {
      id,
      label: providerGroupLabel(p, t),
      description: providerGroupDescription(p, t),
      builtIn,
      providers: [p],
      apiKeyEnv: p.apiKeyEnv,
      keySet: p.keySet,
      requiresKey: providerRequiresKey(p),
      configured: providerIsConfigured(p),
      keySource: p.keySource,
      keySourcePath: p.keySourcePath,
      baseUrl: p.baseUrl,
      kind: p.kind,
      models: uniqueStrings(p.models),
      locked: Boolean(p.locked),
      availability: p.availability,
    });
  }
  return Array.from(groups.values());
}

function parseProviderListInput(value: string): string[] {
  return uniqueStrings(value
    .split(/[,，]/)
    .map((entry) => entry.trim())
    .filter(Boolean));
}

// Memoized model chips for ProviderEditor — prevents re-render when typing
// in name/key/baseUrl fields.
const ModelChips = memo(function ModelChips({ modelNames }: { modelNames: string[] }) {
  const t = useT();
  if (modelNames.length === 0) return null;
  return (
    <div className="provider-model-chips">
      {modelNames.slice(0, 8).map((model) => (
        <span className="provider-model-chip" key={model}>{model}</span>
      ))}
      {modelNames.length > 8 && (
        <span className="provider-model-chip provider-model-chip--more">{t("settings.moreModels", { n: modelNames.length - 8 })}</span>
      )}
    </div>
  );
});

function ProviderEditor({
  initial,
  kinds,
  existingProviderNames = [],
  busy,
  onCancel,
  onSave,
  onSaveKey,
  onClearKey,
}: {
  initial?: ProviderView;
  kinds: string[];
  existingProviderNames?: string[];
  busy: boolean;
  onCancel: () => void;
  onSave: (p: ProviderView) => void;
  onSaveKey?: (apiKeyEnv: string, value: string) => Promise<void>;
  onClearKey?: (apiKeyEnv: string) => Promise<void>;
}) {
  const t = useT();
  const [name, setName] = useState(initial?.name ?? "");
  const defaultKind = newCustomProviderKind(kinds);
  const [kind, setKind] = useState(initial?.kind ?? defaultKind);
  const [baseUrl, setBaseUrl] = useState(initial?.baseUrl ?? "");
  const [models, setModels] = useState((initial?.models ?? []).join(", "));
  const [visionModels, setVisionModels] = useState((initial?.visionModels ?? []).join(", "));
  const [visionModelsConfigured, setVisionModelsConfigured] = useState(
    Boolean(initial?.visionModelsConfigured ?? ((initial?.visionModels ?? []).length > 0)),
  );
  const [modelsUrl] = useState(initial?.modelsUrl ?? "");
  const [apiKeyEnv, setApiKeyEnv] = useState(initial?.apiKeyEnv ?? "");
  const [keyDraft, setKeyDraft] = useState("");
  const [balanceUrl, setBalanceUrl] = useState(initial?.balanceUrl ?? "");
  // Empty when unset so the placeholder (and its "0 = default" hint) reads instead
  // of a bare "0"; saved back as 0.
  const [ctx, setCtx] = useState(initial?.contextWindow ? String(initial.contextWindow) : "");
  const [reasoningProtocol, setReasoningProtocol] = useState(normalizeReasoningProtocol(initial?.reasoningProtocol));
  const [supportedEfforts, setSupportedEfforts] = useState<string[]>(initial?.supportedEfforts ?? []);
  const [customEffortDraft, setCustomEffortDraft] = useState("");
  const [defaultEffort, setDefaultEffort] = useState(initial?.defaultEffort ?? "");
  const [fetchingModels, setFetchingModels] = useState(false);
  const [fetchStatus, setFetchStatus] = useState<string | null>(null);
  const [fetchErr, setFetchErr] = useState<string | null>(null);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const builtIn = initial?.builtIn ?? false;
  const isNewCustomProvider = !initial;

  // Offer the kinds the kernel actually registered; if the stored kind is a
  // legacy/unknown one, keep it as an option so editing doesn't silently change it.
  const kindOptions = kind && !kinds.includes(kind) ? [kind, ...kinds] : kinds;

  // Split supportedEfforts into the 5 canonical presets (for checkbox UI) and
  // any user-added custom names (rendered as removable chips). The preset order
  // is fixed; custom names keep insertion order.
  const presetEfforts = supportedEfforts.filter((e) => EFFORT_PRESETS.includes(e));
  const customEfforts = supportedEfforts.filter((e) => !EFFORT_PRESETS.includes(e));

  const togglePreset = (level: string) => {
    const has = presetEfforts.includes(level);
    const nextPresets = has ? presetEfforts.filter((e) => e !== level) : [...presetEfforts, level];
    setSupportedEfforts([...nextPresets, ...customEfforts]);
    // If the removed preset was the default, fall back to "auto" (empty string).
    if (has && defaultEffort === level) setDefaultEffort("");
  };

  const addCustomEffort = () => {
    const v = customEffortDraft.trim().toLowerCase();
    if (!v || supportedEfforts.includes(v)) {
      setCustomEffortDraft("");
      return;
    }
    setSupportedEfforts([...presetEfforts, ...customEfforts, v]);
    setCustomEffortDraft("");
  };

  const removeCustomEffort = (level: string) => {
    setSupportedEfforts(supportedEfforts.filter((e) => e !== level));
    if (defaultEffort === level) setDefaultEffort("");
  };

  const reportApiKeyEnvError = (effectiveApiKeyEnv: string): boolean => {
    const envError = providerApiKeyEnvError(effectiveApiKeyEnv, t);
    if (!envError) return false;
    setFetchStatus(null);
    setFetchErr(envError);
    setAdvancedOpen(true);
    return true;
  };

  const fetchModels = async () => {
    setFetchingModels(true);
    setFetchStatus(null);
    setFetchErr(null);
    try {
      const effectiveApiKeyEnv = providerApiKeyEnvForSave(name, apiKeyEnv, keyDraft);
      if (!apiKeyEnv.trim()) setApiKeyEnv(effectiveApiKeyEnv);
      if (reportApiKeyEnvError(effectiveApiKeyEnv)) return;
      if (keyDraft.trim()) await app.SetProviderKeyConfigOnly(effectiveApiKeyEnv, keyDraft.trim());
      const fetched = await app.FetchProviderModels({
        name: name.trim() || t("settings.newProviderDraftName"),
        builtIn: initial?.builtIn ?? false,
        added: initial?.added ?? true,
        kind: kind.trim() || defaultKind,
        baseUrl: baseUrl.trim(),
        modelsUrl,
        models: [],
        visionModels: [],
        visionModelsConfigured: false,
        default: "",
        apiKeyEnv: effectiveApiKeyEnv,
        keySet: Boolean(keyDraft.trim()) || (initial?.keySet ?? false),
        balanceUrl: balanceUrl.trim(),
        contextWindow: Number(ctx) || 0,
        reasoningProtocol,
        supportedEfforts,
        defaultEffort,
      });
      if (fetched.length === 0) throw new Error(t("settings.fetchModelsEmpty"));
      setModels(fetched.join(", "));
      setVisionModels((current) => {
        const existing = parseProviderListInput(current).filter((model) => fetched.includes(model));
        return uniqueStrings([...existing, ...inferredVisionModels(fetched)]).filter((model) => fetched.includes(model)).join(", ");
      });
      setVisionModelsConfigured(true);
      if (keyDraft.trim()) setKeyDraft("");
      setDefaultEffort((v) => v);
      setFetchStatus(t("settings.fetchModelsSuccess", { n: fetched.length }));
    } catch (e) {
      setFetchErr(reportClientError("models.load", e, {
        fallbackMessage: "加载模型设置失败，请稍后重试。",
      }).userMessage);
    } finally {
      setFetchingModels(false);
    }
  };

  const save = async () => {
    const trimmedName = name.trim();
    if (!initial && existingProviderNames.some((existing) => existing.trim().toLowerCase() === trimmedName.toLowerCase())) {
      setFetchStatus(null);
      setFetchErr(t("settings.providerNameExists", { name: trimmedName }));
      return;
    }
    const ms = parseProviderListInput(models);
    const vms = parseProviderListInput(visionModels).filter((model) => ms.includes(model));
    const effectiveApiKeyEnv = providerApiKeyEnvForSave(trimmedName, apiKeyEnv, keyDraft);
    if (reportApiKeyEnvError(effectiveApiKeyEnv)) return;
    if (keyDraft.trim()) await app.SetProviderKeyConfigOnly(effectiveApiKeyEnv, keyDraft.trim());
    onSave({
      name: trimmedName,
      builtIn: initial?.builtIn ?? false,
      added: initial?.added ?? true,
      kind: kind.trim() || defaultKind,
      baseUrl: baseUrl.trim(),
      models: ms,
      visionModels: vms,
      visionModelsConfigured: visionModelsConfigured || vms.length > 0,
      default: ms[0] ?? "",
      apiKeyEnv: effectiveApiKeyEnv,
      modelsUrl,
      keySet: Boolean(keyDraft.trim()) || (initial?.keySet ?? false),
      balanceUrl: balanceUrl.trim(),
      contextWindow: Number(ctx) || 0,
      reasoningProtocol,
      supportedEfforts,
      // Clear the stored default if no levels are selected; the backend's
      // NormalizeEffort would otherwise silently ignore an unsupported value.
      defaultEffort: supportedEfforts.length > 0 ? defaultEffort : "",
    });
  };

  if (builtIn) {
    const keyEnv = initial?.apiKeyEnv.trim() ?? "";
    return (
      <div className="provider-editor provider-editor--builtin provider-editor--key-only">
        {initial && onSaveKey && keyEnv && (
          <>
            <div className="provider-key-status provider-key-status--managed provider-key-status--compact">
              <span title={initial.keySourcePath || undefined}>
                {initial.keySet ? t("settings.configuredKey", { env: keyEnv }) : t("settings.notConfiguredKey", { env: keyEnv })}
                {initial.keySource ? ` · ${t("settings.keySource", { source: initial.keySource })}` : ""}
              </span>
              {initial.keySet && onClearKey && (
                <InlineConfirmButton
                  label={t("settings.clearKey")}
                  confirmLabel={t("settings.confirmClearKey")}
                  cancelLabel={t("common.cancel")}
                  disabled={busy}
                  danger
                  onConfirm={() => onClearKey(keyEnv)}
                />
              )}
            </div>
            <KeyField
              apiKeyEnv={keyEnv}
              busy={busy}
              keySet={initial.keySet}
              onSet={(env, value) => onSaveKey(env, value)}
            />
          </>
        )}
      </div>
    );
  }

  const modelNames = useMemo(
    () => models.split(",").map((m) => m.trim()).filter(Boolean),
    [models],
  );
  const canFetch = Boolean(name.trim() && baseUrl.trim());
  const apiKeyEnvInputError = providerApiKeyEnvError(apiKeyEnv, t);

  const protocolField = initial ? (
    <select className="mem-select" value={kind} onChange={(e) => setKind(e.target.value)}>
      {kindOptions.map((k) => (
        <option key={k} value={k}>
          {k === "openai" ? t("settings.providerProtocolOpenAI") : k}
        </option>
      ))}
    </select>
  ) : (
    <div className="provider-readonly-field provider-readonly-field--stacked" aria-readonly="true">
      <strong>{t("settings.providerProtocolOpenAI")}</strong>
      <span>{t("settings.providerProtocolOpenAIHint")}</span>
    </div>
  );

  const advancedFields = (
    <details className="provider-editor-advanced" open={advancedOpen} onToggle={(e) => setAdvancedOpen(e.currentTarget.open)}>
      <summary>{t("settings.providerAdvancedSettings")}</summary>
      <div className="provider-editor-advanced__body">
        <label className="set-label">{t("settings.providerApiKeyEnv")}</label>
        <input
          className="mem-input"
          placeholder={apiKeyEnvFromProviderName(name)}
          value={apiKeyEnv}
          onChange={(e) => setApiKeyEnv(e.target.value)}
        />
        {apiKeyEnvInputError && <div className="provider-fetch-status provider-fetch-status--error">{apiKeyEnvInputError}</div>}
        <div className="mem-hint">{t("settings.providerApiKeyEnvHint")}</div>
        <label className="set-label">{t("settings.providerBalanceUrl")}</label>
        <input className="mem-input" placeholder={t("settings.balanceUrlPlaceholder")} value={balanceUrl} onChange={(e) => setBalanceUrl(e.target.value)} />
        <div className="mem-hint">{t("settings.balanceUrlHint")}</div>
        <label className="set-label">{t("settings.providerContextWindow")}</label>
        <input className="mem-input" placeholder={t("settings.contextWindowPlaceholder")} value={ctx} onChange={(e) => setCtx(e.target.value)} inputMode="numeric" />
        <div className="mem-hint">{t("settings.contextWindowHint")}</div>
        <label className="set-label">{t("settings.visionModels")}</label>
        <input
          className="mem-input"
          placeholder={t("settings.providerModels")}
          value={visionModels}
          onChange={(e) => {
            setVisionModelsConfigured(true);
            setVisionModels(e.target.value);
          }}
        />
        <div className="mem-hint">{t("settings.visionModelsHint")}</div>
        <label className="set-label">{t("settings.reasoningProtocol")}</label>
        <select className="mem-select" value={reasoningProtocol} onChange={(e) => setReasoningProtocol(e.target.value)}>
          {REASONING_PROTOCOLS.map((protocol) => (
            <option key={protocol || "auto"} value={protocol}>
              {reasoningProtocolLabel(protocol, t)}
            </option>
          ))}
        </select>
        <div className="mem-hint">{t("settings.reasoningProtocolHint")}</div>
        <label className="set-label">{t("settings.supportedEfforts")}</label>
        {EFFORT_PRESETS.map((level) => (
          <label key={level} className="set-check">
            <input
              type="checkbox"
              checked={presetEfforts.includes(level)}
              onChange={() => togglePreset(level)}
            />
            {level}
          </label>
        ))}
        <div className="set-row">
          <input
            className="mem-input set-grow"
            placeholder={t("settings.supportedEffortsCustomPlaceholder")}
            value={customEffortDraft}
            onChange={(e) => setCustomEffortDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                addCustomEffort();
              }
            }}
          />
          <button
            type="button"
            className="btn btn--small"
            disabled={
              !customEffortDraft.trim() || supportedEfforts.includes(customEffortDraft.trim().toLowerCase())
            }
            onClick={addCustomEffort}
          >
            {t("common.add")}
          </button>
        </div>
        {customEfforts.length > 0 && (
          <div className="set-rules__chips">
            {customEfforts.map((level) => (
              <span className="set-rule" key={level}>
                {level}
                <Tooltip label={t("common.delete")}>
                  <button
                    type="button"
                    className="set-rule__x"
                    disabled={busy}
                    onClick={() => removeCustomEffort(level)}
                  >
                    ×
                  </button>
                </Tooltip>
              </span>
            ))}
          </div>
        )}
        <div className="mem-hint">{t("settings.supportedEffortsHint")}</div>
        <label className="set-label">{t("settings.defaultEffort")}</label>
        {supportedEfforts.length > 0 ? (
          <select
            className="mem-select"
            value={defaultEffort}
            onChange={(e) => setDefaultEffort(e.target.value)}
          >
            <option value="">{t("settings.defaultEffortAuto")}</option>
            {supportedEfforts.map((level) => (
              <option key={level} value={level}>
                {level}
              </option>
            ))}
          </select>
        ) : (
          <select className="mem-select" value="" disabled>
            <option value="">{t("settings.defaultEffortAuto")}</option>
          </select>
        )}
        <div className="mem-hint">{t("settings.defaultEffortHint")}</div>
      </div>
    </details>
  );

  return (
    <div className={`provider-editor${isNewCustomProvider ? " provider-editor--wizard" : ""}`}>
      <label className="set-label">{t("settings.customProviderName")}</label>
      <input className="mem-input" placeholder={t("settings.customProviderNamePlaceholder")} value={name} onChange={(e) => setName(e.target.value)} disabled={!!initial} />
      <label className="set-label">{t("settings.providerProtocol")}</label>
      {protocolField}
      <label className="set-label">{t("settings.providerBaseUrlLabel")}</label>
      <input className="mem-input" placeholder={t("settings.providerBaseUrl")} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} />
      {!initial && (
        <>
          <label className="set-label">{t("settings.providerKey")}</label>
          <input
            className="mem-input"
            type="password"
            placeholder={t("settings.providerKeyPlaceholder")}
            value={keyDraft}
            onChange={(e) => setKeyDraft(e.target.value)}
          />
        </>
      )}
      {initial && onSaveKey && apiKeyEnv.trim() && (
        <>
          <label className="set-label">{t("settings.providerKey")}</label>
          {initial.keySource && (
            <div className="mem-hint" title={initial.keySourcePath || undefined}>
              {t("settings.keySource", { source: initial.keySource })}
            </div>
          )}
          <KeyField
            apiKeyEnv={apiKeyEnv.trim()}
            busy={busy || fetchingModels}
            keySet={initial.keySet}
            onSet={(env, value) => onSaveKey(env, value)}
          />
        </>
      )}
      <div className="provider-model-fetch-row">
        <button
          type="button"
          className="btn btn--small"
          disabled={busy || fetchingModels || !canFetch}
          onClick={() => void fetchModels()}
        >
          {fetchingModels ? t("settings.fetchingModels") : t("settings.testFetchModels")}
        </button>
        <span>{t("settings.testFetchModelsHint")}</span>
      </div>
      {fetchStatus && <div className="provider-fetch-status provider-fetch-status--ok">{fetchStatus}</div>}
      {fetchErr && <div className="provider-fetch-status provider-fetch-status--error">{fetchErr}</div>}
      {modelNames.length > 0 && (
        <div className="provider-card-block">
          <div className="provider-card-block__label">{t("settings.availableModels")}</div>
          <ModelChips modelNames={modelNames} />
        </div>
      )}
      <label className="set-label">{t("settings.manualModels")}</label>
      <input className="mem-input" placeholder={t("settings.providerModels")} value={models} onChange={(e) => setModels(e.target.value)} />
      <div className="mem-hint">{t("settings.manualModelsHint")}</div>
      {advancedFields}
      <div className="prov-card__actions">
        <button className="btn btn--small" onClick={onCancel} disabled={busy}>
          {t("common.cancel")}
        </button>
        <button className="btn btn--primary btn--small" onClick={() => void save()} disabled={busy || !name.trim() || !baseUrl.trim() || !models.trim()}>
          {t("common.save")}
        </button>
      </div>
    </div>
  );
}

function KeyField({
  apiKeyEnv,
  busy,
  keySet = false,
  onSet,
}: {
  apiKeyEnv: string;
  busy: boolean;
  keySet?: boolean;
  onSet: (apiKeyEnv: string, value: string) => Promise<void>;
}) {
  const t = useT();
  const [val, setVal] = useState("");
  if (!apiKeyEnv) return null;
  const apiKeyEnvError = providerApiKeyEnvError(apiKeyEnv, t);
  return (
    <div className="set-key">
      <input
        className="mem-input"
        type="password"
        placeholder={t(keySet ? "settings.updateKey" : "settings.setKey", { env: apiKeyEnv })}
        value={val}
        onChange={(e) => setVal(e.target.value)}
      />
      <button
        className="btn btn--small"
        disabled={busy || !val.trim() || Boolean(apiKeyEnvError)}
        onClick={() => {
          void onSet(apiKeyEnv, val.trim());
          setVal("");
        }}
      >
        {t(keySet ? "settings.updateKeyAction" : "settings.saveKey")}
      </button>
      {apiKeyEnvError && <div className="provider-fetch-status provider-fetch-status--error">{apiKeyEnvError}</div>}
    </div>
  );
}
