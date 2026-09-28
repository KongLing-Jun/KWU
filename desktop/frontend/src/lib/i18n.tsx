// The desktop ships a single Simplified Chinese UI. Keep the preference-shaped
// API so persisted settings and existing callers remain compatible, but always
// normalize legacy automatic, English, or Traditional Chinese values to `zh`.

import { createContext, useCallback, useContext, useEffect } from "react";
import type { ReactNode } from "react";
import { zh, type DictKey } from "../locales/zh";

export type Locale = "zh";
export type { DictKey };
export type LangPref = "" | "en" | "zh" | "zh-TW";

const STORAGE_KEY = "kwu-lang";
const CHINESE_LOCALE: Locale = "zh";

export const SPINNER_WORDS: Record<Locale, string[]> = {
  zh: [
    "畅游中", "沉思中", "鼓捣中", "酝酿中", "施法中", "苦思中",
    "渗滤中", "反刍中", "文火慢炖", "合成中", "修补中",
    "腌制入味", "嘎吱运算", "孵化中", "盘算中", "嗡嗡运转", "锻造中",
    "探索中", "摆弄中", "来感觉了",
  ],
};

export function detectLocale(_pref: LangPref): Locale {
  return CHINESE_LOCALE;
}

export function normalizeLangPref(_value: unknown): LangPref {
  return CHINESE_LOCALE;
}

export function readLegacyLangPref(): LangPref {
  if (typeof localStorage !== "undefined") localStorage.getItem(STORAGE_KEY);
  return CHINESE_LOCALE;
}

export function clearLegacyLangPref(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    /* private mode / no storage */
  }
}

function translate(key: DictKey, vars?: Record<string, string | number>): string {
  const value: string = zh[key] ?? key;
  if (!vars) return value;
  return value.replace(/\{(\w+)\}/g, (_, name) => (
    vars[name] !== undefined ? String(vars[name]) : `{${name}}`
  ));
}

export function t(key: DictKey, vars?: Record<string, string | number>): string {
  return translate(key, vars);
}

export function getLocale(): Locale {
  return CHINESE_LOCALE;
}

export type Translator = (key: DictKey, vars?: Record<string, string | number>) => string;

interface I18nValue {
  locale: Locale;
  pref: LangPref;
  setPref: (pref: LangPref) => void;
  t: Translator;
}

const I18nContext = createContext<I18nValue | null>(null);

export function LocaleProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    if (typeof document !== "undefined") document.documentElement.lang = "zh-CN";
  }, []);

  const setPref = useCallback((_next: LangPref) => {
    // The UI language is fixed to Simplified Chinese.
  }, []);
  const translateValue = useCallback<Translator>((key, vars) => translate(key, vars), []);

  return (
    <I18nContext.Provider
      value={{ locale: CHINESE_LOCALE, pref: CHINESE_LOCALE, setPref, t: translateValue }}
    >
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n(): I18nValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within a LocaleProvider");
  return ctx;
}

export function useT(): Translator {
  return useI18n().t;
}
