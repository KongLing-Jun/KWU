export const BRAND_DISPLAY_NAME = "云智企办";
export const BRAND_ENGLISH_NAME = "Clouvia";
export const BRAND_SLUG = "clouvia";

export function formatBrandVersion(version: string): string {
  const normalized = version.trim().replace(/^v/i, "");
  return normalized ? `v${normalized}` : "";
}

export const LEGACY_BRAND_DISPLAY_NAME = "快务AI";
export const LEGACY_BRAND_ENGLISH_NAME = "KWU";
export const LEGACY_BRAND_SLUG = "kwu";

export const BRAND_ENV_PREFIX = "KWU";
export const LEGACY_BRAND_ENV_PREFIX = "KWU";

export const OFFICIAL_API_KEY_ENV = `${BRAND_ENV_PREFIX}_API_KEY`;
export const LEGACY_OFFICIAL_API_KEY_ENV = `${LEGACY_BRAND_ENV_PREFIX}_API_KEY`;

export const PROJECT_CONFIG_NAME = "clouvia.toml";
export const LEGACY_PROJECT_CONFIG_NAME = "kwu.toml";

export const BRAND_PROVIDER_ID = "kwu";
export const LEGACY_PROVIDER_ID = BRAND_PROVIDER_ID;

export const DESKTOP_BINARY_NAME = "kwu-desktop";
export const LEGACY_DESKTOP_BINARY_NAME = DESKTOP_BINARY_NAME;

export const MEMORY_FILE_NAME = "CLOUVIA.md";
export const LEGACY_MEMORY_FILE_NAME = "KWU.md";
