import {
  BRAND_DISPLAY_NAME,
  BRAND_PROVIDER_ID,
  LEGACY_PROVIDER_ID,
} from "./brand";
import { useT } from "./i18n";
import { providerRequiresKey } from "./providerModels";
import type { ProviderView } from "./types";

export function officialProviderKind(p: ProviderView): string {
  if (!p.builtIn) return "";
  const name = p.name.trim();
  if (name === "deepseek" || name === "deepseek-flash" || name === "deepseek-pro") return "deepseek";
  return name === BRAND_PROVIDER_ID || name === LEGACY_PROVIDER_ID ? BRAND_PROVIDER_ID : "";
}

export function providerGroupID(p: ProviderView): string {
  const official = officialProviderKind(p);
  if (official) return `builtin:${official}`;
  return `custom:${p.name}`;
}

export function providerGroupLabel(p: ProviderView, t?: ReturnType<typeof useT>): string {
  const id = providerGroupID(p);
  if (id === "builtin:deepseek") return t ? t("settings.providerLabel.deepseek") : "DeepSeek";
  if (id === `builtin:${BRAND_PROVIDER_ID}`) return t ? t("settings.providerLabel.kwu") : BRAND_DISPLAY_NAME;
  return p.name;
}

export function providerGroupDescription(p: ProviderView, t: ReturnType<typeof useT>): string {
  const id = providerGroupID(p);
  if (id === "builtin:deepseek") return t("settings.providerDesc.deepseek");
  if (id === `builtin:${BRAND_PROVIDER_ID}`) return t("settings.providerDesc.kwu");
  return p.baseUrl;
}

export function providerKeyStatusLabel(
  provider: { keySet: boolean; requiresKey?: boolean; apiKeyEnv?: string },
  t: ReturnType<typeof useT>,
): string {
  if (!providerRequiresKey(provider)) return t("settings.noKeyRequired");
  return provider.keySet ? t("settings.keySet") : t("settings.noKey");
}

export function modelProviderLabel(
  provider: string,
  providerView: ProviderView | undefined,
  t: ReturnType<typeof useT>,
): string {
  if (providerView) return providerGroupLabel(providerView, t);
  const name = provider.trim();
  if (name === BRAND_PROVIDER_ID || name === LEGACY_PROVIDER_ID) {
    return t("settings.providerLabel.kwu");
  }
  return name;
}
