import type { SettingsCoreView, SettingsModelDetailsView } from "../../../lib/types";

type Loader<T> = () => Promise<T>;
type SettingsScopeTab = { id?: string; scope?: string; workspaceRoot?: string } | null | undefined;
type PrefetchScheduler = {
  setTimeout(handler: () => void, timeout: number): number;
  clearTimeout(id: number): void;
  requestIdleCallback?: (handler: () => void, options?: { timeout: number }) => number;
  cancelIdleCallback?: (id: number) => void;
};

type CacheEntry<T> = {
  generation: number;
  value: T;
};

const coreCache = new Map<string, CacheEntry<SettingsCoreView>>();
const detailsCache = new Map<string, CacheEntry<SettingsModelDetailsView>>();
const coreInflight = new Map<string, Promise<SettingsCoreView>>();
const detailsInflight = new Map<string, Promise<SettingsModelDetailsView>>();
const generations = new Map<string, number>();
const detailsGenerations = new Map<string, number>();
const requestEpochs = new Map<string, number>();
const detailsRequestEpochs = new Map<string, number>();

function generation(scopeKey: string): number {
  return generations.get(scopeKey) ?? 0;
}

function detailsGeneration(scopeKey: string): number {
  return detailsGenerations.get(scopeKey) ?? 0;
}

function requestEpoch(scopeKey: string): number {
  return requestEpochs.get(scopeKey) ?? 0;
}

function detailsRequestEpoch(scopeKey: string): number {
  return detailsRequestEpochs.get(scopeKey) ?? 0;
}

function readSnapshot<T>(cache: Map<string, CacheEntry<T>>, scopeKey: string, currentGeneration: number): T | null {
  const entry = cache.get(scopeKey);
  return entry?.generation === currentGeneration ? entry.value : null;
}

async function loadSnapshot<T>(
  cache: Map<string, CacheEntry<T>>,
  inflight: Map<string, Promise<T>>,
  scopeKey: string,
  loader: Loader<T>,
  force: boolean,
  getGeneration: (scopeKey: string) => number,
  getRequestEpoch: (scopeKey: string) => number,
): Promise<T> {
  const cached = readSnapshot(cache, scopeKey, getGeneration(scopeKey));
  if (!force && cached) return cached;
  const existing = inflight.get(scopeKey);
  if (existing) return existing;
  const requestGeneration = getGeneration(scopeKey);
  const requestStartEpoch = getRequestEpoch(scopeKey);
  const request = loader().then((value) => {
    if (getGeneration(scopeKey) === requestGeneration && getRequestEpoch(scopeKey) === requestStartEpoch) {
      cache.set(scopeKey, { generation: requestGeneration, value });
    }
    return value;
  }).finally(() => {
    if (inflight.get(scopeKey) === request) inflight.delete(scopeKey);
  });
  inflight.set(scopeKey, request);
  return request;
}

export function peekSettingsCore(scopeKey: string): SettingsCoreView | null {
  return readSnapshot(coreCache, scopeKey, generation(scopeKey));
}

export function settingsSnapshotScopeKey(accountScopeKey: string, tab: SettingsScopeTab): string {
  return [
    accountScopeKey,
    tab?.id ?? "",
    tab?.scope ?? "",
    tab?.workspaceRoot ?? "",
  ].join("\u0000");
}

export function scheduleSettingsCorePrefetch(
  scopeKey: string,
  loader: Loader<SettingsCoreView>,
  scheduler: PrefetchScheduler = window,
): () => void {
  let cancelled = false;
  let idleCallbackID: number | null = null;
  const prefetch = () => {
    void loadSettingsCore(scopeKey, async () => {
      const value = await loader();
      if (cancelled) throw new Error("settings prefetch cancelled");
      return value;
    }).catch(() => {});
  };
  const timerID = scheduler.setTimeout(() => {
    if (scheduler.requestIdleCallback) {
      idleCallbackID = scheduler.requestIdleCallback(prefetch, { timeout: 1000 });
    } else {
      prefetch();
    }
  }, 1500);
  return () => {
    cancelled = true;
    scheduler.clearTimeout(timerID);
    if (idleCallbackID !== null) scheduler.cancelIdleCallback?.(idleCallbackID);
  };
}

export function loadSettingsCore(scopeKey: string, loader: Loader<SettingsCoreView>, force = false): Promise<SettingsCoreView> {
  return loadSnapshot(coreCache, coreInflight, scopeKey, loader, force, generation, requestEpoch);
}

export function updateSettingsCore(scopeKey: string, value: SettingsCoreView): void {
  coreCache.set(scopeKey, { generation: generation(scopeKey), value });
}

export function peekSettingsDetails(scopeKey: string): SettingsModelDetailsView | null {
  return readSnapshot(detailsCache, scopeKey, detailsGeneration(scopeKey));
}

export function loadSettingsDetails(scopeKey: string, loader: Loader<SettingsModelDetailsView>, force = false): Promise<SettingsModelDetailsView> {
  return loadSnapshot(detailsCache, detailsInflight, scopeKey, loader, force, detailsGeneration, detailsRequestEpoch);
}

export function invalidateSettingsDetails(scopeKey: string): void {
  detailsGenerations.set(scopeKey, detailsGeneration(scopeKey) + 1);
  detailsRequestEpochs.set(scopeKey, detailsRequestEpoch(scopeKey) + 1);
  detailsCache.delete(scopeKey);
  detailsInflight.delete(scopeKey);
}

export function invalidateSettingsScope(scopeKey: string): void {
  generations.set(scopeKey, generation(scopeKey) + 1);
  detailsGenerations.set(scopeKey, detailsGeneration(scopeKey) + 1);
  requestEpochs.set(scopeKey, requestEpoch(scopeKey) + 1);
  detailsRequestEpochs.set(scopeKey, detailsRequestEpoch(scopeKey) + 1);
  coreCache.delete(scopeKey);
  detailsCache.delete(scopeKey);
  coreInflight.delete(scopeKey);
  detailsInflight.delete(scopeKey);
}

export function cancelSettingsScopeLoads(scopeKey: string): void {
  requestEpochs.set(scopeKey, requestEpoch(scopeKey) + 1);
  detailsRequestEpochs.set(scopeKey, detailsRequestEpoch(scopeKey) + 1);
  coreInflight.delete(scopeKey);
  detailsInflight.delete(scopeKey);
}

export function resetSettingsSnapshotCacheForTests(): void {
  coreCache.clear();
  detailsCache.clear();
  coreInflight.clear();
  detailsInflight.clear();
  generations.clear();
  detailsGenerations.clear();
  requestEpochs.clear();
  detailsRequestEpochs.clear();
}
