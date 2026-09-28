import { useCallback, useRef, useState } from "react";
import logo from "../assets/brand-symbol.png";
import { BRAND_DISPLAY_NAME } from "../lib/brand";
import { useT } from "../lib/i18n";
import { app } from "../lib/bridge";
import { reportClientError } from "../lib/clientError";

type OnboardingStep = "key" | "models";
type OnboardingState = "idle" | "validating" | "saving" | "error";

function normalizeModels(models: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of models) {
    const model = raw.trim();
    if (!model || seen.has(model)) continue;
    seen.add(model);
    out.push(model);
  }
  return out;
}

// Full-window first-run gate: save the official provider key, fetch model choices,
// then persist the user's selected default before unmounting into the main UI.
export function OnboardingOverlay({ onComplete }: { onComplete: () => void }) {
  const t = useT();
  const [value, setValue] = useState("");
  const [step, setStep] = useState<OnboardingStep>("key");
  const [state, setState] = useState<OnboardingState>("idle");
  const [error, setError] = useState<string | null>(null);
  const [models, setModels] = useState<string[]>([]);
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [defaultModel, setDefaultModel] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const submitKey = useCallback(async () => {
    const key = value.trim();
    if (!key) {
      setError(t("onboarding.error.empty"));
      setState("error");
      inputRef.current?.focus();
      return;
    }
    setState("validating");
    setError(null);
    try {
      const result = await app.ConnectKey(key);
      const nextModels = normalizeModels(result.models);
      if (nextModels.length === 0) {
        throw new Error(t("onboarding.error.noModels"));
      }
      const nextDefault = nextModels.includes(result.defaultModel) ? result.defaultModel : nextModels[0];
      setModels(nextModels);
      setSelectedModels(nextModels);
      setDefaultModel(nextDefault);
      setStep("models");
      setState("idle");
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      reportClientError("onboarding.fetchModels", e, {
        fallbackMessage: t("onboarding.error.unknown"),
      });
      if (/status\s*401|status\s*403|invalid/i.test(msg)) {
        setError(t("onboarding.error.invalid"));
      } else if (/network|unreachable|timeout|dial/i.test(msg)) {
        setError(t("onboarding.error.network"));
      } else {
        setError(msg || t("onboarding.error.unknown"));
      }
      setState("error");
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [t, value]);

  const toggleModel = useCallback((model: string, checked: boolean) => {
    const next = checked
      ? selectedModels.includes(model) ? selectedModels : [...selectedModels, model]
      : selectedModels.filter((m) => m !== model);
    setSelectedModels(next);
    setDefaultModel((current) => next.includes(current) ? current : next[0] || "");
    if (state === "error") {
      setState("idle");
      setError(null);
    }
  }, [selectedModels, state]);

  const saveSelection = useCallback(async () => {
    if (selectedModels.length === 0) {
      setError(t("onboarding.error.noSelection"));
      setState("error");
      return;
    }
    const selectedDefault = selectedModels.includes(defaultModel) ? defaultModel : selectedModels[0];
    setState("saving");
    setError(null);
    try {
      await app.CompleteOnboardingModelSelection(selectedDefault, selectedModels);
      onComplete();
    } catch (e) {
      setError(reportClientError("onboarding.save", e, {
        fallbackMessage: t("onboarding.error.unknown"),
      }).userMessage);
      setState("error");
    }
  }, [defaultModel, onComplete, selectedModels, t]);

  const selectedSet = new Set(selectedModels);
  const busy = state === "validating" || state === "saving";

  return (
    <div className="onboarding">
      <div className="onboarding__card">
        <img src={logo} className="onboarding__logo" alt={BRAND_DISPLAY_NAME} draggable={false} />
        <div className="onboarding__title">{t("onboarding.title")}</div>
        <div className="onboarding__tag">{t("onboarding.tagline")}</div>

        {step === "key" ? (
          <>
            <label className="onboarding__label" htmlFor="onboarding-key">
              {t("onboarding.inputLabel")}
            </label>
            <input
              id="onboarding-key"
              ref={inputRef}
              className="onboarding__input"
              type="password"
              autoComplete="off"
              spellCheck={false}
              placeholder={t("onboarding.inputPlaceholder")}
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                if (state === "error") setState("idle");
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !busy) {
                  e.preventDefault();
                  void submitKey();
                }
              }}
              disabled={busy}
            />
          </>
        ) : (
          <>
            <div className="onboarding__label">{t("onboarding.modelsTitle")}</div>
            <div className="onboarding__model-list">
              {models.map((model) => (
                <label className="onboarding__model-option" key={model}>
                  <input
                    type="checkbox"
                    checked={selectedSet.has(model)}
                    disabled={busy}
                    onChange={(e) => toggleModel(model, e.target.checked)}
                  />
                  <span>{model}</span>
                </label>
              ))}
            </div>

            <label className="onboarding__label" htmlFor="onboarding-default-model">
              {t("onboarding.defaultModelLabel")}
            </label>
            <select
              id="onboarding-default-model"
              className="onboarding__input"
              value={defaultModel}
              disabled={busy || selectedModels.length === 0}
              onChange={(e) => setDefaultModel(e.target.value)}
            >
              {selectedModels.map((model) => (
                <option key={model} value={model}>{model}</option>
              ))}
            </select>

          </>
        )}

        {state === "error" && error && (
          <div className="onboarding__error" role="alert">
            {error}
          </div>
        )}

        <button
          className="onboarding__submit"
          onClick={() => step === "key" ? void submitKey() : void saveSelection()}
          disabled={busy}
        >
          {busy ? (
            <>
              <span className="onboarding__spinner" />
              {state === "saving" ? t("onboarding.saving") : t("onboarding.validating")}
            </>
          ) : (
            step === "key" ? t("onboarding.submit") : t("onboarding.save")
          )}
        </button>

        <div className="onboarding__links">
          <span className="onboarding__privacy">{t("onboarding.privacy")}</span>
        </div>

        <button
          type="button"
          className="onboarding__skip"
          onClick={onComplete}
          disabled={busy}
        >
          {t("onboarding.skip")}
        </button>
      </div>
    </div>
  );
}
