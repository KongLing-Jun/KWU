import { BRAND_PROVIDER_ID, LEGACY_PROVIDER_ID } from "./brand";
import type { ModelInfo } from "./types";

export type ModelProviderLabels = {
  deepseek: string;
  product: string;
};

export type ModelProviderGroup = {
  provider: string;
  label: string;
  items: ModelInfo[];
};

export function modelProviderLabel(provider: string, labels: ModelProviderLabels): string {
  switch (provider) {
    case "cloud":
      return "Cloud";
    case "deepseek":
    case "deepseek-flash":
    case "deepseek-pro":
      return labels.deepseek;
    case BRAND_PROVIDER_ID:
    case LEGACY_PROVIDER_ID:
    case "airclaude_subscription":
      return labels.product;
    default:
      return provider;
  }
}

// A provider containing subscription-pool routes is always first. All other
// groups use a label/provider tie-breaker while preserving route order in each group.
export function groupModelInfosByProvider(
  models: readonly ModelInfo[],
  labels: ModelProviderLabels,
): ModelProviderGroup[] {
  const map = new Map<string, ModelInfo[]>();
  for (const model of models) {
    const items = map.get(model.provider);
    if (items) items.push(model);
    else map.set(model.provider, [model]);
  }
  return [...map.entries()]
    .sort(([a, aItems], [b, bItems]) => {
      const aSubscription = aItems.some((item) => item.billingSource === "subscription_pool");
      const bSubscription = bItems.some((item) => item.billingSource === "subscription_pool");
      if (aSubscription !== bSubscription) return aSubscription ? -1 : 1;
      const labelOrder = modelProviderLabel(a, labels).localeCompare(modelProviderLabel(b, labels));
      return labelOrder || a.localeCompare(b);
    })
    .map(([provider, items]) => ({
      provider,
      label: modelProviderLabel(provider, labels),
      items,
    }));
}
