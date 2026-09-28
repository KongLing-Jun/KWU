import { asArray } from "./array";
import type { SettingsView } from "./types";

export function settingsModelCatalogFingerprint(
  settings: SettingsView | null | undefined,
): string | null {
  if (!settings) return null;
  const providers = [...asArray(settings.providers), ...asArray(settings.officialProviders)]
    .map((provider) => ({
      name: provider.name,
      kind: provider.kind,
      added: provider.added,
      configured: provider.configured,
      baseUrl: provider.baseUrl,
      models: asArray(provider.models),
      chatModels: asArray(provider.chatModels),
      billingSource: provider.billingSource,
      availability: provider.availability,
    }))
    .sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
  return JSON.stringify({
    providerKinds: [...asArray(settings.providerKinds)].sort(),
    providers,
  });
}
