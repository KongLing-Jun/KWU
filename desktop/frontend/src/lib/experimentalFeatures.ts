const SUPERPOWERS_ENABLED_KEY = "kwu-experimental-superpowers-enabled";

export function getSuperpowersEnabled(): boolean {
  if (typeof localStorage === "undefined") return false;
  try {
    return localStorage.getItem(SUPERPOWERS_ENABLED_KEY) === "true";
  } catch {
    return false;
  }
}

export function setSuperpowersEnabled(enabled: boolean): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(SUPERPOWERS_ENABLED_KEY, String(enabled));
  } catch {
    /* The in-memory setting still applies when storage is unavailable. */
  }
}
