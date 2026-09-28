import { useCallback, useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from "react";

import { useT } from "../../../lib/i18n";
import {
  expertActivationIds,
  pendingExpertActivationIdForTab,
  removeDeletedExpertSelection,
  resolveExpertRefreshSelection,
  type PendingExpertActivation,
} from "../../../lib/expertSelection";
import { useToast } from "../../../lib/toast";
import {
  currentComposerExpertTraceId,
  recordComposerExpertTrace,
} from "../../../lib/composerExpertTrace";
import type { ComposerExpertResolution } from "../../../lib/composerExpertHandoff";
import type { ExpertInfo, ExpertLibraryState, UserExpertInfo, UserExpertInput } from "../../../lib/types";
import { expertCatalogBridge } from "../bridge/expertBridge";
import { expertSelectionFromState, expertSelectionsFromRecentIds } from "../model/expertSelection";
import type { ExpertSelection } from "../ui/ExpertDirectory";

interface ExpertCatalogOptions {
  activeTabId: string | null;
  viewKey: string;
  canClearSelection: boolean;
  setRecentExperts: Dispatch<SetStateAction<ExpertSelection[]>>;
}

function normalizedUsageCounts(value: unknown): Record<string, number> {
  if (!value || typeof value !== "object") return {};
  return Object.fromEntries(Object.entries(value).filter(([id, count]) => (
    id.trim() !== "" && typeof count === "number" && Number.isFinite(count) && count > 0
  )));
}

export function useExpertCatalog({ activeTabId, viewKey, canClearSelection, setRecentExperts }: ExpertCatalogOptions) {
  const t = useT();
  const { showToast, showError } = useToast();
  const [experts, setExperts] = useState<ExpertInfo[]>([]);
  const [userExperts, setUserExperts] = useState<UserExpertInfo[]>([]);
  const [libraryState, setLibraryState] = useState<ExpertLibraryState>({ favoriteIds: [], recentIds: [], usageCounts: {} });
  const [activeExpert, setActiveExpert] = useState<ExpertSelection | null>(null);
  const [activeExpertResolution, setActiveExpertResolution] = useState<ComposerExpertResolution | null>(null);
  const pendingActivationRef = useRef<PendingExpertActivation | null>(null);
  const activationSequenceRef = useRef(0);
  const refreshSequenceRef = useRef(0);
  const activeTabRef = useRef(activeTabId);
  activeTabRef.current = activeTabId;
  const activeViewKeyRef = useRef(viewKey);
  activeViewKeyRef.current = viewKey;

  const activatableExpertIds = useMemo(() => expertActivationIds(experts, userExperts), [experts, userExperts]);

  const publishExpertSelection = useCallback((tabId: string, selection: ExpertSelection | null) => {
    ++refreshSequenceRef.current;
    if (activeTabRef.current !== tabId) return;
    setActiveExpert(selection);
    setActiveExpertResolution({
      tabId,
      viewKey: activeViewKeyRef.current,
      refreshSequence: refreshSequenceRef.current,
      authoritative: true,
    });
  }, []);

  const refreshExpertState = useCallback(async (requestedTabId?: string) => {
    const sequence = ++refreshSequenceRef.current;
    const targetTabId = requestedTabId ?? activeTabId;
    const activationSequence = activationSequenceRef.current;
    const startedDuringClear = pendingExpertActivationIdForTab(pendingActivationRef.current, targetTabId) === "";
    const targetViewKey = requestedTabId && activeTabRef.current === requestedTabId
      ? activeViewKeyRef.current
      : viewKey;
    const expertTraceId = currentComposerExpertTraceId();
    recordComposerExpertTrace("expert-refresh.start", {
      refreshSequence: sequence,
      targetTabId: targetTabId ?? "",
      targetViewKey,
      activeTabRef: activeTabRef.current ?? "",
      canClearSelection,
    }, expertTraceId);
    const isCurrent = () => (
      sequence === refreshSequenceRef.current
      && activationSequence === activationSequenceRef.current
      && !startedDuringClear
      && activeTabRef.current === targetTabId
      && activeViewKeyRef.current === targetViewKey
    );
    try {
      const catalogPromise = expertCatalogBridge.ListExperts();
      if (!targetTabId) {
        const [catalog, nextUserExperts] = await Promise.all([
          catalogPromise,
          expertCatalogBridge.ListUserExperts().catch(() => []),
        ]);
        if (!isCurrent()) {
          recordComposerExpertTrace("expert-refresh.ignored", {
            refreshSequence: sequence,
            targetTabId: "",
            targetViewKey,
            phase: "catalog-only",
            activeTabRef: activeTabRef.current ?? "",
          }, expertTraceId);
          return;
        }
        setUserExperts(Array.isArray(nextUserExperts) ? nextUserExperts : []);
        setExperts(Array.isArray(catalog) ? catalog : []);
        return;
      }
      const [catalog, expertState] = await Promise.all([
        catalogPromise,
        expertCatalogBridge.CurrentExpertForTab(targetTabId),
      ]);
      recordComposerExpertTrace("expert-refresh.current-expert-resolved", {
        refreshSequence: sequence,
        targetTabId,
        targetViewKey,
        backendExpertId: expertState?.selectedId ?? "",
        backendExpertName: expertState?.selectedName ?? "",
      }, expertTraceId);
      const normalizedExperts = Array.isArray(catalog) ? catalog : [];
      let normalizedUserExperts: UserExpertInfo[] = [];
      let normalizedLibraryState: ExpertLibraryState = { favoriteIds: [], recentIds: [], usageCounts: {} };
      const [nextUserExperts, nextLibraryState] = await Promise.all([
        expertCatalogBridge.ListUserExpertsForTab(targetTabId).catch(() => []),
        expertCatalogBridge.ExpertLibraryStateForTab(targetTabId).catch(() => ({ favoriteIds: [], recentIds: [], usageCounts: {} })),
      ]);
      normalizedUserExperts = Array.isArray(nextUserExperts) ? nextUserExperts : [];
      normalizedLibraryState = {
        favoriteIds: Array.isArray(nextLibraryState?.favoriteIds) ? nextLibraryState.favoriteIds : [],
        recentIds: Array.isArray(nextLibraryState?.recentIds) ? nextLibraryState.recentIds : [],
        usageCounts: normalizedUsageCounts(nextLibraryState?.usageCounts),
      };
      if (!isCurrent()) {
        recordComposerExpertTrace("expert-refresh.ignored", {
          refreshSequence: sequence,
          targetTabId,
          targetViewKey,
          phase: "full",
          activeTabRef: activeTabRef.current ?? "",
        }, expertTraceId);
        return;
      }
      setUserExperts(normalizedUserExperts);
      setLibraryState(normalizedLibraryState);
      setExperts(normalizedExperts);
      setRecentExperts(expertSelectionsFromRecentIds(
        normalizedLibraryState.recentIds,
        normalizedExperts,
        normalizedUserExperts,
      ));
      const nextActiveExpert = expertSelectionFromState(expertState, normalizedExperts, normalizedUserExperts);
      const backendExpertIds = expertActivationIds(normalizedExperts, normalizedUserExperts);
      const pendingActivation = pendingActivationRef.current;
      const pendingActivationId = pendingExpertActivationIdForTab(pendingActivation, targetTabId);
      const pendingClear = pendingActivationId === "";
      if ((nextActiveExpert?.id && pendingActivationId === nextActiveExpert.id) || (pendingClear && !nextActiveExpert)) {
        pendingActivationRef.current = null;
      }
      setActiveExpert((previous) => {
        const applied = resolveExpertRefreshSelection({
          previous,
          next: nextActiveExpert,
          canClear: canClearSelection,
          backendExpertIds,
          pendingActivationId,
          pendingClear,
        });
        recordComposerExpertTrace("expert-refresh.apply", {
          refreshSequence: sequence,
          targetTabId,
          targetViewKey,
          previousExpertId: previous?.id ?? "",
          previousExpertName: previous?.name ?? "",
          backendExpertId: nextActiveExpert?.id ?? "",
          backendExpertName: nextActiveExpert?.name ?? "",
          appliedExpertId: applied?.id ?? "",
          appliedExpertName: applied?.name ?? "",
          canClearSelection,
          pendingActivationId: pendingActivationId ?? "",
          pendingActivationTabId: pendingActivation?.tabId ?? "",
          pendingActivationSequence: pendingActivation?.requestSequence ?? 0,
          pendingClear,
        }, expertTraceId);
        return applied;
      });
      if (canClearSelection) {
        setActiveExpertResolution({
          viewKey: targetViewKey,
          tabId: targetTabId,
          refreshSequence: sequence,
          authoritative: true,
        });
        recordComposerExpertTrace("expert-refresh.authoritative", {
          refreshSequence: sequence,
          targetTabId,
          targetViewKey,
        }, expertTraceId);
      }
    } catch (error) {
      if (!isCurrent()) return;
      showError("experts.refresh", error, "加载专家列表失败，请稍后重试。");
    }
  }, [activeTabId, canClearSelection, setRecentExperts, showError, viewKey]);

  useEffect(() => {
    void refreshExpertState();
  }, [refreshExpertState]);

  const withErrorToast = useCallback(async <T,>(operation: () => Promise<T>): Promise<T> => {
    try {
      return await operation();
    } catch (error) {
      showError("experts.save", error, "保存专家设置失败，请稍后重试。");
      throw error;
    }
  }, [showError]);

  const saveUserExpert = useCallback((input: UserExpertInput, id?: string) => withErrorToast(async () => {
    const saved = id
      ? await expertCatalogBridge.UpdateUserExpert(id, input)
      : await expertCatalogBridge.CreateUserExpert(input);
    await refreshExpertState();
    return saved;
  }), [refreshExpertState, withErrorToast]);
  const saveCatalogExpert = useCallback((id: string, input: ExpertInfo) => withErrorToast(async () => {
    const saved = await expertCatalogBridge.UpdateExpert(id, input);
    await refreshExpertState();
    return saved;
  }), [refreshExpertState, withErrorToast]);
  const duplicateExpert = useCallback((id: string) => withErrorToast(async () => {
    const saved = await expertCatalogBridge.DuplicateExpert(id);
    await refreshExpertState();
    return saved;
  }), [refreshExpertState, withErrorToast]);
  const toggleExpertFavorite = useCallback((id: string, favorite: boolean) => withErrorToast(async () => {
    const next = await expertCatalogBridge.SetExpertFavorite(id, favorite);
    setLibraryState({
      favoriteIds: Array.isArray(next?.favoriteIds) ? next.favoriteIds : [],
      recentIds: Array.isArray(next?.recentIds) ? next.recentIds : [],
      usageCounts: normalizedUsageCounts(next?.usageCounts),
    });
    return next;
  }), [withErrorToast]);
  const importExpertConfig = useCallback((payload: string) => withErrorToast(
    () => expertCatalogBridge.ImportExpertConfig(payload),
  ), [withErrorToast]);
  const deleteExpert = useCallback((id: string, userOwned: boolean) => withErrorToast(async () => {
    if (pendingActivationRef.current?.expertId === id) {
      activationSequenceRef.current += 1;
      pendingActivationRef.current = null;
    }
    if (userOwned) await expertCatalogBridge.DeleteUserExpert(id);
    else await expertCatalogBridge.DeleteExpert(id);
    setActiveExpert((previous) => removeDeletedExpertSelection(previous, id));
    setRecentExperts((previous) => previous.filter((expert) => expert.id !== id));
    await refreshExpertState();
  }), [refreshExpertState, setRecentExperts, withErrorToast]);
  const deleteUserExpert = useCallback((id: string) => deleteExpert(id, true), [deleteExpert]);
  const deleteCatalogExpert = useCallback((id: string) => deleteExpert(id, false), [deleteExpert]);
  const resetExperts = useCallback(() => withErrorToast(async () => {
    activationSequenceRef.current += 1;
    pendingActivationRef.current = null;
    const result = await expertCatalogBridge.ResetExperts();
    setExperts(Array.isArray(result?.experts) ? result.experts : []);
    setUserExperts(Array.isArray(result?.userExperts) ? result.userExperts : []);
    setLibraryState({
      favoriteIds: Array.isArray(result?.libraryState?.favoriteIds) ? result.libraryState.favoriteIds : [],
      recentIds: Array.isArray(result?.libraryState?.recentIds) ? result.libraryState.recentIds : [],
    });
    setRecentExperts([]);
    setActiveExpert(null);
    showToast(t("experts.resetDone"));
  }), [setRecentExperts, showToast, t, withErrorToast]);

  return {
    expertCatalog: experts,
    userExpertCatalog: userExperts,
    expertLibraryState: libraryState,
    activeExpert,
    activeExpertResolution,
    setActiveExpert,
    publishExpertSelection,
    pendingExpertActivationRef: pendingActivationRef,
    expertActivationSequenceRef: activationSequenceRef,
    activatableExpertIds,
    refreshExpertState,
    saveUserExpert,
    saveCatalogExpert,
    duplicateExpert,
    toggleExpertFavorite,
    importExpertConfig,
    deleteUserExpert,
    deleteCatalogExpert,
    resetExperts,
  };
}
