import { asArray } from "./array";
import type { ProjectNode, TabMeta } from "./types";

export const SUBSCRIPTION_MARKETPLACE_SOURCE_ID = "international";
export const SUBSCRIPTION_BILLING_SOURCE = "subscription_pool";
export const SUBSCRIPTION_DEFAULT_REPAIR_MODEL_ID = "deepseek-v4-flash";

const SUBSCRIPTION_RULES = {
  free: { status: "free", tag: "Subscribe_Free" },
  lite: { status: "active", tag: "Subscribe_Lite" },
  plus: { status: "active", tag: "Subscribe_Plus" },
  pro: { status: "active", tag: "Subscribe_Pro" },
} as const;

export type SubscriptionPlan = keyof typeof SUBSCRIPTION_RULES;
export type SubscriptionTag = (typeof SUBSCRIPTION_RULES)[SubscriptionPlan]["tag"];

export type SubscriptionAccess =
  | { state: "loading" }
  | { state: "unavailable" }
  | {
      state: "ready";
      plan: SubscriptionPlan;
      requiredTag: SubscriptionTag;
    };

export type SubscriptionAccessInput = {
  initialized: boolean;
  hasBinding: boolean;
  allowSubscriptionModels: boolean;
  plan?: string | null;
  status?: string | null;
};

export type SubscriptionModelCandidate = {
  ref?: string;
  model: string;
  billingSource?: string;
  disabled?: boolean;
  current?: boolean;
};

export type SubscriptionMarketplaceModel = {
  name: string;
  tags?: readonly string[];
};

export type SubscriptionMarketplaceSource = {
  id: string;
  pricingState?: string;
  metadataStatus?: string;
  models?: readonly SubscriptionMarketplaceModel[];
  errorCode?: string;
  partial?: boolean;
};

export type SubscriptionMarketplaceView = {
  sources?: readonly SubscriptionMarketplaceSource[];
};

export type SubscriptionMarketplaceRefreshScopeState =
  | { state: "running"; controller: AbortController }
  | { state: "settled" };

export type ModelRouteAccess =
  | { state: "allowed" }
  | { state: "loading" }
  | { state: "unavailable" };

export type TopicTurnEvidence = {
  key: string;
  state: "loading" | "ready" | "unavailable";
  turns?: number;
};

export type SubscriptionRepairSessionInput = {
  tabKey: string;
  turnEvidence: TopicTurnEvidence;
  historyLoaded: boolean;
  hydrating: boolean;
  hydrateError?: string;
  backendActivationPending: boolean;
  readOnly: boolean;
  running: boolean;
  hasHistory: boolean;
  hasCheckpoints: boolean;
  hasLiveTurn: boolean;
};

export type SubscriptionAccountState = "loading" | "anonymous" | "unavailable" | "ready";

export type SubscriptionDefaultSettings = {
  defaultModel: string;
  defaultBillingSource?: string;
  plannerModel?: string;
  subagentModel?: string;
};

export type SubscriptionDefaultRepairDecision<T extends SubscriptionModelCandidate> =
  | { state: "keep" }
  | { state: "blocked" }
  | { state: "repair"; candidate: T };

export type SubscriptionDefaultRepairRunResult = "applied" | "duplicate" | "failed" | "stale";
export type SubscriptionDefaultRepairRunState = {
  scopeKey: string;
  attemptKey: string;
  state: "running" | "failed";
};

export type SubscriptionModelInvocationAccess =
  | { state: "allowed" }
  | { state: "loading" }
  | { state: "blocked" };

export function resolveSubscriptionAccess(input: SubscriptionAccessInput): SubscriptionAccess {
  if (!input.initialized) return { state: "loading" };
  if (!input.hasBinding || !input.allowSubscriptionModels) return { state: "unavailable" };

  const plan = input.plan;
  if (!isSubscriptionPlan(plan)) return { state: "unavailable" };

  const rule = SUBSCRIPTION_RULES[plan];
  if (input.status !== rule.status) return { state: "unavailable" };

  return {
    state: "ready",
    plan,
    requiredTag: rule.tag,
  };
}

export function resolveSubscriptionAccountState(input: {
  initialized: boolean;
  available: boolean;
  accountStatus?: string | null;
  hasBinding: boolean;
  access: SubscriptionAccess;
}): SubscriptionAccountState {
  if (!input.initialized || input.accountStatus === "authenticating" || input.accountStatus === "refreshing") {
    return "loading";
  }
  if (!input.accountStatus || input.accountStatus === "anonymous") return "anonymous";
  if (
    !input.available
    || input.accountStatus !== "authenticated"
    || !input.hasBinding
    || input.access.state !== "ready"
  ) {
    return "unavailable";
  }
  return "ready";
}

export function filterSubscriptionModelCandidates<T>(
  candidates: readonly T[],
  marketplace: SubscriptionMarketplaceView | null | undefined,
  access: SubscriptionAccess,
  describe: (candidate: T) => SubscriptionModelCandidate,
): T[] {
  return candidates.filter((candidate) => resolveModelRouteAccess(
    describe(candidate),
    marketplace,
    access,
    false,
    true,
  ).state === "allowed");
}

export function filterModelInfosForSubscriptionAccess<T extends SubscriptionModelCandidate>(
  models: readonly T[],
  marketplace: SubscriptionMarketplaceView | null | undefined,
  access: SubscriptionAccess,
): T[] {
  return filterSubscriptionModelCandidates(models, marketplace, access, (model) => model);
}

export function resolveModelRouteAccess(
  route: SubscriptionModelCandidate | null | undefined,
  marketplace: SubscriptionMarketplaceView | null | undefined,
  access: SubscriptionAccess,
  evidenceLoading = false,
  allowUniversalDefault = false,
): ModelRouteAccess {
  if (!route) return evidenceLoading ? { state: "loading" } : { state: "unavailable" };
  if (route.disabled) return { state: "unavailable" };
  if (route.billingSource === "newapi_byok") return { state: "allowed" };
  if (route.billingSource !== SUBSCRIPTION_BILLING_SOURCE) return { state: "unavailable" };
  if (access.state === "loading" || evidenceLoading) return { state: "loading" };
  if (access.state !== "ready") return { state: "unavailable" };
  if (allowUniversalDefault && route.model === SUBSCRIPTION_DEFAULT_REPAIR_MODEL_ID) {
    return { state: "allowed" };
  }
  return authorizedSubscriptionModels(marketplace, access)?.has(route.model)
    ? { state: "allowed" }
    : { state: "unavailable" };
}

export function subscriptionMarketplaceNeedsRefresh(
  marketplace: SubscriptionMarketplaceView | null | undefined,
): boolean {
  return !subscriptionMarketplaceRefreshSucceeded(marketplace);
}

export function subscriptionMarketplaceRefreshSucceeded(
  marketplace: SubscriptionMarketplaceView | null | undefined,
): boolean {
  const sources = marketplace?.sources?.filter(
    (source) => source.id === SUBSCRIPTION_MARKETPLACE_SOURCE_ID,
  ) ?? [];
  if (sources.length !== 1) return false;

  const source = sources[0];
  return source.pricingState === "fresh"
    && source.metadataStatus === "ready"
    && !source.errorCode?.trim()
    && source.partial !== true
    && Array.isArray(source.models);
}

export function claimSubscriptionMarketplaceRefresh(
  accountScopeStates: Map<string, SubscriptionMarketplaceRefreshScopeState>,
  accountScopeKey: string,
  force = false,
): AbortController | null {
  const run = accountScopeStates.get(accountScopeKey);
  if (!accountScopeKey || run?.state === "running" || (run?.state === "settled" && !force)) return null;
  const controller = new AbortController();
  accountScopeStates.set(accountScopeKey, { state: "running", controller });
  return controller;
}

export function settleSubscriptionMarketplaceRefresh(
  accountScopeStates: Map<string, SubscriptionMarketplaceRefreshScopeState>,
  accountScopeKey: string,
  controller: AbortController,
): boolean {
  const run = accountScopeStates.get(accountScopeKey);
  if (controller.signal.aborted || run?.state !== "running" || run.controller !== controller) return false;
  accountScopeStates.set(accountScopeKey, { state: "settled" });
  return true;
}

export function cancelSubscriptionMarketplaceRefresh(
  accountScopeStates: Map<string, SubscriptionMarketplaceRefreshScopeState>,
): void {
  for (const run of accountScopeStates.values()) {
    if (run.state === "running") run.controller.abort();
  }
  accountScopeStates.clear();
}

export function uniqueCurrentModelRoute<T extends SubscriptionModelCandidate>(
  routes: readonly T[],
): T | null {
  const current = routes.filter((route) => route.current);
  return current.length === 1 ? current[0] : null;
}

export function resolveActiveModelRoute<T extends SubscriptionModelCandidate>(
  routes: readonly T[],
  activeLabel: string | null | undefined,
  activeBillingSource: string | null | undefined,
): T | null {
  const current = routes.filter((route) => route.current);
  if (current.length > 0) return current.length === 1 ? current[0] : null;

  const normalizedLabel = activeLabel?.trim() ?? "";
  const normalizedSource = activeBillingSource?.trim() ?? "";
  if (!normalizedLabel || !normalizedSource) return null;

  const matches = routes.filter((route) => (
    route.billingSource === normalizedSource
    && (route.ref === normalizedLabel || route.model === normalizedLabel)
  ));
  return matches.length === 1 ? matches[0] : null;
}

export function exactSavedModelRoute<T extends SubscriptionModelCandidate>(
  routes: readonly T[],
  ref: string | null | undefined,
  billingSource: string | null | undefined,
): T | null {
  const normalizedRef = ref?.trim() ?? "";
  const normalizedSource = billingSource?.trim() ?? "";
  if (!normalizedRef || !normalizedSource) return null;
  const matches = routes.filter((route) => route.ref === normalizedRef && route.billingSource === normalizedSource);
  return matches.length === 1 ? matches[0] : null;
}

export function findSubscriptionDefaultRepairCandidate<T extends SubscriptionModelCandidate>(
  routes: readonly T[],
  marketplace: SubscriptionMarketplaceView | null | undefined,
  access: SubscriptionAccess,
): T | null {
  if (access.state !== "ready") return null;
  const matches = routes.filter((route) => (
    Boolean(route.ref?.trim())
    && route.model === SUBSCRIPTION_DEFAULT_REPAIR_MODEL_ID
    && route.billingSource === SUBSCRIPTION_BILLING_SOURCE
    && !route.disabled
    && resolveModelRouteAccess(route, marketplace, access).state === "allowed"
  ));
  return matches.length === 1 ? matches[0] : null;
}

export function hasSubscriptionDerivedModelConflict<T extends SubscriptionModelCandidate>(
  settings: SubscriptionDefaultSettings,
  routes: readonly T[],
): boolean {
  return [settings.plannerModel, settings.subagentModel].some((value) => {
    const ref = value?.trim() ?? "";
    if (!ref) return false;
    const matches = routes.filter((route) => route.ref === ref && !route.disabled);
    return matches.length !== 1 || matches[0].billingSource !== SUBSCRIPTION_BILLING_SOURCE;
  });
}

export function resolveSubscriptionDefaultRepair<T extends SubscriptionModelCandidate>(
  settings: SubscriptionDefaultSettings | null | undefined,
  routes: readonly T[] | null | undefined,
  marketplace: SubscriptionMarketplaceView | null | undefined,
  access: SubscriptionAccess,
): SubscriptionDefaultRepairDecision<T> {
  if (!settings || !routes) return { state: "blocked" };

  const defaultModel = settings.defaultModel.trim();
  const defaultBillingSource = settings.defaultBillingSource?.trim() ?? "";
  const savedRoute = exactSavedModelRoute(routes, defaultModel, defaultBillingSource);
  if (savedRoute && resolveModelRouteAccess(savedRoute, marketplace, access).state === "allowed") {
    return { state: "keep" };
  }

  // An invalid BYOK/legacy default belongs to BYOK configuration, not to
  // subscription repair. Never overwrite it as a side effect of this flow.
  if (defaultModel && defaultBillingSource !== SUBSCRIPTION_BILLING_SOURCE) {
    return { state: "keep" };
  }
  if (access.state !== "ready" || hasSubscriptionDerivedModelConflict(settings, routes)) {
    return { state: "blocked" };
  }

  const candidate = findSubscriptionDefaultRepairCandidate(routes, marketplace, access);
  return candidate ? { state: "repair", candidate } : { state: "blocked" };
}

export function subscriptionDefaultRepairAttemptKey(input: {
  accountKey: string;
  tabKey: string;
  hydrationGeneration: number;
  settings: SubscriptionDefaultSettings;
  routes: readonly SubscriptionModelCandidate[];
  marketplace: SubscriptionMarketplaceView;
}): string {
  const routeFacts = input.routes.map((route) => [
    route.ref?.trim() ?? "",
    route.model,
    route.billingSource ?? "",
    route.disabled ? "disabled" : "enabled",
    route.current ? "current" : "",
  ].join(":")).sort().join("|");
  const marketplaceFacts = (input.marketplace.sources ?? []).map((source) => [
    source.id,
    ...(source.models ?? []).map((model) => [
      model.name,
      ...(model.tags ?? []).slice().sort(),
    ].join(":")).sort(),
  ].join("|")).sort().join("\u0002");
  return [
    input.accountKey,
    input.tabKey,
    String(input.hydrationGeneration),
    input.settings.defaultModel.trim(),
    input.settings.defaultBillingSource?.trim() ?? "",
    input.settings.plannerModel?.trim() ?? "",
    input.settings.subagentModel?.trim() ?? "",
    routeFacts,
    marketplaceFacts,
  ].join("\u0001");
}

export function subscriptionDefaultRepairScopeKey(input: {
  accountKey: string;
  tabKey: string;
  hydrationGeneration: number;
}): string {
  return [
    input.accountKey,
    input.tabKey,
    String(input.hydrationGeneration),
  ].join("\u0001");
}

export function subscriptionRepairFactsKey(input: {
  accountKey: string;
  tabID: string;
  revision: number;
  activeLabel?: string | null;
  activeBillingSource?: string | null;
}): string {
  return [
    input.accountKey,
    input.tabID,
    String(input.revision),
    input.activeLabel?.trim() ?? "",
    input.activeBillingSource?.trim() ?? "",
  ].join("\u0001");
}

export async function runSubscriptionDefaultRepair(input: {
  attemptedKeys: Set<string>;
  attemptKey: string;
  isTargetCurrent: () => boolean;
  isScopeCurrent: () => boolean;
  onClaim?: () => void;
  apply: (isScopeCurrent: () => boolean) => Promise<boolean>;
  onApplied?: () => void | Promise<void>;
}): Promise<SubscriptionDefaultRepairRunResult> {
  if (!input.isScopeCurrent() || !input.isTargetCurrent()) return "stale";
  if (input.attemptedKeys.has(input.attemptKey)) return "duplicate";
  input.attemptedKeys.add(input.attemptKey);
  input.onClaim?.();
  if (!input.isScopeCurrent() || !input.isTargetCurrent()) return "stale";
  try {
    if (!await input.apply(input.isScopeCurrent)) return "stale";
  } catch {
    return "failed";
  }
  // Applying the route naturally changes settings and route facts. Only a
  // real account/tab/session scope change may invalidate completion here.
  if (!input.isScopeCurrent()) return "stale";
  try {
    await input.onApplied?.();
  } catch {
    // Both setters already succeeded. A best-effort UI/facts refresh failure
    // must not turn a completed write into a failed repair or retry loop.
  }
  return "applied";
}

export function settleSubscriptionDefaultRepairRunState(
  current: SubscriptionDefaultRepairRunState | null,
  completed: {
    scopeKey: string;
    attemptKey: string;
    result: Exclude<SubscriptionDefaultRepairRunResult, "duplicate">;
    currentScopeKey: string;
  },
): SubscriptionDefaultRepairRunState | null {
  if (current?.scopeKey !== completed.scopeKey || current.attemptKey !== completed.attemptKey) {
    return current;
  }
  if (completed.result === "failed" && completed.currentScopeKey === completed.scopeKey) {
    return { ...current, state: "failed" };
  }
  return null;
}

export function shouldBlockSubscriptionDefaultRepair(_input: {
  hasByokRoute: boolean;
  relevant: boolean;
  classificationPending: boolean;
  blankEligible: boolean;
  accountState: SubscriptionAccountState;
  factsSettled: boolean;
  decisionState: SubscriptionDefaultRepairDecision<SubscriptionModelCandidate>["state"];
}): boolean {
  // Default-model repair may continue in the background, but it must never
  // turn account/model discovery into a client-side send permission gate.
  return false;
}

export function resolveSubscriptionModelInvocationAccess(_input: {
  route: SubscriptionModelCandidate | null;
  routeAccess: ModelRouteAccess;
  factsState: "loading" | "settled";
  auxiliaryBillingSource?: string | null;
}): SubscriptionModelInvocationAccess {
  // Model/account facts are presentation data, not authorization to invoke.
  // Let the request path report any real provider or server-side rejection.
  return { state: "allowed" };
}

export function topicTurnEvidenceKey(tab: {
  id?: string | null;
  scope?: string | null;
  workspaceRoot?: string | null;
  topicId?: string | null;
} | null | undefined): string {
  const id = tab?.id?.trim() ?? "";
  const scope = tab?.scope?.trim() ?? "";
  const topicID = tab?.topicId?.trim() ?? "";
  if (!id || !scope || !topicID) return "";
  return [id, scope, tab?.workspaceRoot?.trim() ?? "", topicID].join("\u0000");
}

export function activeTopicTurnsFromTree(
  tree: ProjectNode[],
  tab?: Pick<TabMeta, "scope" | "workspaceRoot" | "topicId">,
): number | undefined {
  if (!tab?.topicId) return undefined;
  const targetScope = tab.scope === "global" ? "global" : "project";
  const walk = (nodes: ProjectNode[]): number | undefined => {
    for (const node of nodes) {
      if (!node) continue;
      if (node.kind === "topic" || node.kind === "global_topic") {
        const scope = node.kind === "global_topic" ? "global" : "project";
        if (
          scope === targetScope
          && node.topicId === tab.topicId
          && (scope === "global" || node.root === tab.workspaceRoot)
        ) {
          return node.turns ?? 0;
        }
      }
      const found = walk(asArray(node.children));
      if (found !== undefined) return found;
    }
    return undefined;
  };
  return walk(tree);
}

export function isBlankSessionEligibleForSubscriptionRepair(
  input: SubscriptionRepairSessionInput,
): boolean {
  return Boolean(
    input.tabKey
    && input.turnEvidence.key === input.tabKey
    && input.turnEvidence.state === "ready"
    && input.turnEvidence.turns === 0
    && input.historyLoaded
    && !input.hydrating
    && !input.hydrateError
    && !input.backendActivationPending
    && !input.readOnly
    && !input.running
    && !input.hasHistory
    && !input.hasCheckpoints
    && !input.hasLiveTurn
  );
}

export function filterSettingsRefsForSubscriptionAccess<TProvider>(
  refs: readonly string[],
  providers: readonly TProvider[],
  marketplace: SubscriptionMarketplaceView | null | undefined,
  access: SubscriptionAccess,
  describe: (ref: string, providers: readonly TProvider[]) => SubscriptionModelCandidate,
): string[] {
  return filterSubscriptionModelCandidates(
    refs,
    marketplace,
    access,
    (ref) => describe(ref, providers),
  );
}

function isSubscriptionPlan(value: string | null | undefined): value is SubscriptionPlan {
  return value !== null && value !== undefined && Object.prototype.hasOwnProperty.call(SUBSCRIPTION_RULES, value);
}

function authorizedSubscriptionModels(
  marketplace: SubscriptionMarketplaceView | null | undefined,
  access: SubscriptionAccess,
): ReadonlySet<string> | null {
  if (access.state !== "ready") return null;

  const sources = marketplace?.sources?.filter((source) => source.id === SUBSCRIPTION_MARKETPLACE_SOURCE_ID) ?? [];
  if (sources.length !== 1) return null;

  const models = sources[0].models;
  if (!models) return null;

  const modelCounts = new Map<string, number>();
  for (const model of models) {
    if (!model.name) continue;
    modelCounts.set(model.name, (modelCounts.get(model.name) ?? 0) + 1);
  }

  const allowed = new Set<string>();
  for (const model of models) {
    if (
      model.name
      && modelCounts.get(model.name) === 1
      && model.tags?.includes(access.requiredTag)
    ) {
      allowed.add(model.name);
    }
  }
  return allowed;
}
