import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { app, onModelRouteChanged } from "../../../lib/bridge";
import type { ModelInfo } from "../../../lib/types";

type ModelResult = {
  scopeKey: string;
  requestKey: string;
  models: ModelInfo[];
  hasSuccessfulResult: boolean;
  loading: boolean;
  error?: string;
};

// Catalog reads depend on the target and binding, not the chat UI's catalog wrapper.
export function useAutomationModels(
  root: string,
  scopeKey: string,
  settingsRevision: number,
  saving: boolean,
  modelSaveError: boolean,
) {
  const requestKey = JSON.stringify([scopeKey, settingsRevision]);
  const [result, setResult] = useState<ModelResult>();
  const savingRef = useRef(saving);
  const refreshRef = useRef<() => void>(() => {});
  const resumeRef = useRef<() => void>(() => {});
  useLayoutEffect(() => { savingRef.current = saving; }, [saving]);

  useEffect(() => {
    let active = true;
    let inFlight = false;
    let queued = false;
    const reload = async () => {
      if (!active) return;
      queued = true;
      setResult((previous) => ({
        scopeKey, requestKey,
        models: previous?.scopeKey === scopeKey ? previous.models : [],
        hasSuccessfulResult: previous?.scopeKey === scopeKey && previous.hasSuccessfulResult,
        error: previous?.scopeKey === scopeKey ? previous.error : undefined,
        loading: true,
      }));
      if (savingRef.current || inFlight) return;
      queued = false;
      inFlight = true;
      try {
        const list = await app.AutomationModels(root);
        if (active && !queued) setResult({
          scopeKey, requestKey, models: Array.isArray(list) ? list : [],
          hasSuccessfulResult: true, loading: false,
        });
      } catch {
        if (active && !queued) setResult((previous) => ({
          scopeKey, requestKey,
          models: previous?.scopeKey === scopeKey ? previous.models : [],
          hasSuccessfulResult: previous?.scopeKey === scopeKey && previous.hasSuccessfulResult,
          loading: false, error: "模型加载失败，请重试。",
        }));
      } finally {
        inFlight = false;
        // Events during a read require at most one trailing read of the latest facts.
        if (active && queued && !savingRef.current) void reload();
      }
    };
    const refresh = () => { void reload(); };
    const resume = () => { if (queued) refresh(); };
    refreshRef.current = refresh;
    resumeRef.current = resume;
    refresh();
    // The legacy event does not distinguish tab choices from persisted defaults.
    const stop = onModelRouteChanged(refresh);
    window.addEventListener("focus", refresh);
    return () => {
      active = false;
      stop();
      window.removeEventListener("focus", refresh);
    };
  }, [root, scopeKey, requestKey]);

  useEffect(() => {
    if (saving) return;
    if (modelSaveError) refreshRef.current();
    else resumeRef.current();
  }, [saving, modelSaveError]);

  const refresh = useCallback(() => refreshRef.current(), []);
  const current = result?.scopeKey === scopeKey ? result : undefined;
  const loading = !current || current.requestKey !== requestKey || current.loading;
  return {
    models: current?.models ?? [],
    loading,
    // An accepted empty result is display evidence too; a failed read is not.
    initialLoading: loading && !current?.hasSuccessfulResult,
    error: current?.error,
    refresh,
  };
}
