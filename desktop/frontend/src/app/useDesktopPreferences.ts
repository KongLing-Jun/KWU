import { useCallback, useEffect, useState } from "react";

import { app } from "../lib/bridge";
import { DEFAULT_STATUS_BAR_ITEMS, normalizeStatusBarItems, type StatusBarItemId } from "../lib/statusBarItems";
import { applyTheme, normalizeThemePreference, normalizeThemeStyleForTheme } from "../lib/theme";
import type { SettingsView } from "../lib/types";

export type DesktopLayoutStyle = "classic" | "workbench" | "creation";
export type DesktopPlatform = "darwin" | "windows" | "linux";

export function normalizeDesktopLayoutStyle(style: string | undefined): DesktopLayoutStyle {
  void style;
  return "creation";
}

function normalizeDesktopPlatform(value: string): DesktopPlatform {
  if (value === "darwin" || value === "windows") return value;
  return "linux";
}

function browserPlatformOverride(): DesktopPlatform | null {
  if (typeof window === "undefined" || window.runtime) return null;
  const value = new URLSearchParams(window.location.search).get("platform");
  if (value === "darwin" || value === "windows" || value === "linux") return value;
  return null;
}

function detectBrowserPlatform(): DesktopPlatform {
  const override = browserPlatformOverride();
  if (override) return override;
  if (typeof navigator === "undefined") return "linux";
  const marker = `${navigator.platform} ${navigator.userAgent}`;
  if (/Win/i.test(marker)) return "windows";
  if (/Mac/i.test(marker)) return "darwin";
  return "linux";
}

type DesktopPreferenceSettings = Pick<
  SettingsView,
  "desktopTheme" | "desktopThemeStyle" | "desktopLayoutStyle" | "desktopLanguage" | "checkUpdates" | "statusBarStyle" | "statusBarItems"
>;

export function useDesktopPreferences(setLocalePreference: (locale: "zh") => void) {
  const [desktopLayoutStyle, setDesktopLayoutStyle] = useState<DesktopLayoutStyle>("creation");
  const [startupUpdateChecksEnabled, setStartupUpdateChecksEnabled] = useState<boolean | null>(null);
  const [desktopPlatform, setDesktopPlatform] = useState<DesktopPlatform>(detectBrowserPlatform);
  const [statusBarStyle, setStatusBarStyle] = useState<"icon" | "text">("text");
  const [statusBarItems, setStatusBarItems] = useState<StatusBarItemId[]>(() => [...DEFAULT_STATUS_BAR_ITEMS]);

  useEffect(() => {
    document.documentElement.setAttribute("data-platform", desktopPlatform);
  }, [desktopPlatform]);

  useEffect(() => {
    let cancelled = false;
    const override = browserPlatformOverride();
    if (override) {
      setDesktopPlatform(override);
      return () => {
        cancelled = true;
      };
    }
    void app.Platform()
      .then((value) => {
        if (!cancelled) setDesktopPlatform(normalizeDesktopPlatform(value));
      })
      .catch((error) => console.warn("platform probe failed", error));
    return () => {
      cancelled = true;
    };
  }, []);

  const applyDesktopPreferences = useCallback((settings: DesktopPreferenceSettings) => {
    const nextTheme = normalizeThemePreference(settings.desktopTheme);
    const nextStyle = normalizeThemeStyleForTheme(settings.desktopThemeStyle, nextTheme);
    applyTheme(nextTheme, nextStyle, { persist: false });
    setDesktopLayoutStyle(normalizeDesktopLayoutStyle(settings.desktopLayoutStyle));
    setLocalePreference("zh");
    setStartupUpdateChecksEnabled(settings.checkUpdates !== false);
    setStatusBarStyle(settings.statusBarStyle === "text" ? "text" : "icon");
    setStatusBarItems(normalizeStatusBarItems(settings.statusBarItems));
  }, [setLocalePreference]);

  const enableStartupUpdateChecks = useCallback(() => setStartupUpdateChecksEnabled(true), []);

  return {
    desktopLayoutStyle,
    singleSurfaceLayout: desktopLayoutStyle === "workbench" || desktopLayoutStyle === "creation",
    startupUpdateChecksEnabled,
    desktopPlatform,
    statusBarStyle,
    statusBarItems,
    applyDesktopPreferences,
    enableStartupUpdateChecks,
  };
}
