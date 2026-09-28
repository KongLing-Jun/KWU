import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  Check,
  ChevronRight,
  Hammer,
  LoaderCircle,
  Plus,
  RefreshCw,
  Search,
} from "lucide-react";
import { app } from "../../../lib/bridge";
import { reportClientError } from "../../../lib/clientError";
import { useT } from "../../../lib/i18n";
import type { SkillView } from "../../../lib/types";

type SkillQuickMenuProps = {
  open: boolean;
  tokenNames: readonly string[];
  onInsert: (name: string) => void;
  onRemove: (name: string) => void;
  onMore: () => void;
  onRequestClose: () => void;
};

export function SkillQuickMenu({
  open,
  tokenNames,
  onInsert,
  onRemove,
  onMore,
  onRequestClose,
}: SkillQuickMenuProps) {
  const t = useT();
  const [skills, setSkills] = useState<SkillView[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [busyName, setBusyName] = useState("");
  const [feedback, setFeedback] = useState("");
  const [error, setError] = useState("");
  const selected = useMemo(() => new Set(tokenNames), [tokenNames]);

  const reload = useCallback(async () => {
    const view = await app.SkillsSettings();
    const next = [...(view.skills ?? [])].sort((left, right) => {
      const enabledDelta = Number(right.enabled) - Number(left.enabled);
      return enabledDelta || left.name.localeCompare(right.name);
    });
    setSkills(next);
    return next;
  }, []);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoading(true);
    setQuery("");
    setFeedback("");
    setError("");
    void reload()
      .catch((cause) => {
        if (!cancelled) setError(reportClientError("skills.quick.load", cause, {
          fallbackMessage: "加载技能失败，请稍后重试。",
        }).userMessage);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, reload]);

  useEffect(() => {
    if (!feedback) return;
    const id = window.setTimeout(() => setFeedback(""), 1800);
    return () => window.clearTimeout(id);
  }, [feedback]);

  const refresh = async () => {
    setRefreshing(true);
    setFeedback("");
    setError("");
    try {
      await app.RefreshSkills();
      const next = await reload();
      setFeedback(t("composer.skillQuick.refreshed", { count: next.length }));
    } catch (cause) {
      setError(reportClientError("skills.quick.enable", cause, {
        fallbackMessage: "启用技能失败，请稍后重试。",
      }).userMessage);
    } finally {
      setRefreshing(false);
    }
  };

  const enable = async (skill: SkillView) => {
    setBusyName(skill.name);
    setError("");
    try {
      await app.SetSkillEnabled(skill.name, true);
      await app.RefreshSkills();
      await reload();
      onInsert(skill.name);
    } catch (cause) {
      setError(reportClientError("skills.quick.disable", cause, {
        fallbackMessage: "停用技能失败，请稍后重试。",
      }).userMessage);
    } finally {
      setBusyName("");
    }
  };

  const normalizedQuery = query.trim().toLowerCase();
  const filtered = normalizedQuery
    ? skills.filter((skill) =>
        skill.name.toLowerCase().includes(normalizedQuery)
        || String(skill.description ?? "").toLowerCase().includes(normalizedQuery))
    : skills;
  const updateQuery = (value: string) => setQuery(value);

  return (
    <div className="composer-skill-menu" role="menu" aria-label={t("composer.skillQuick.title")}>
      <div className="composer-connector-menu__head">
        <div>
          <strong>{t("composer.skillQuick.title")}</strong>
          <span aria-live="polite">
            {refreshing
              ? t("composer.skillQuick.refreshing")
              : feedback || t("composer.skillQuick.subtitle")}
          </span>
        </div>
        <button
          type="button"
          className="composer-connector-menu__refresh"
          aria-label={t("caps.refresh")}
          disabled={loading || refreshing || Boolean(busyName)}
          onClick={() => void refresh()}
        >
          <RefreshCw className={refreshing ? "is-spinning" : undefined} size={14} aria-hidden />
        </button>
      </div>

      {skills.length > 6 && (
        <label className="composer-skill-menu__search">
          <Search size={14} aria-hidden />
          <input
            type="search"
            value={query}
            onInput={(event) => updateQuery(event.currentTarget.value)}
            onChange={(event) => updateQuery(event.currentTarget.value)}
            placeholder={t("composer.skillQuick.search")}
            aria-label={t("composer.skillQuick.search")}
            autoComplete="off"
            spellCheck={false}
          />
        </label>
      )}

      {error && (
        <div className="composer-connector-menu__error" role="alert">
          <AlertCircle size={14} aria-hidden />
          <span>{error}</span>
        </div>
      )}

      <div className="composer-connector-menu__list">
        {loading && skills.length === 0 && (
          <div className="composer-connector-menu__empty">
            <LoaderCircle className="is-spinning" size={16} aria-hidden />
            {t("common.loading")}
          </div>
        )}
        {!loading && filtered.length === 0 && (
          <div className="composer-connector-menu__empty">{t("composer.skillQuick.empty")}</div>
        )}

        {filtered.map((skill) => {
          const inserted = selected.has(skill.name);
          const busy = busyName === skill.name;
          const iconUrl = skill.iconUrl?.trim();
          const iconFallback = (skill.displayName?.trim() || skill.name).charAt(0).toUpperCase() || "/";
          return (
            <div
              className={`composer-connector-menu__row${inserted ? " composer-connector-menu__row--selected" : ""}${!skill.enabled ? " composer-connector-menu__row--missing" : ""}`}
              key={skill.name}
            >
              <button
                type="button"
                className="composer-connector-menu__main"
                disabled={!skill.enabled || busy}
                onClick={() => {
                  if (inserted) onRemove(skill.name);
                  else onInsert(skill.name);
                }}
              >
                <span className="composer-skill-menu__icon" aria-hidden="true">
                  <span>{iconFallback}</span>
                  {iconUrl && (
                    <img
                      src={iconUrl}
                      alt=""
                      onError={(event) => { event.currentTarget.remove(); }}
                    />
                  )}
                </span>
                <span className="composer-connector-menu__copy">
                  <strong>{skill.name}</strong>
                  <span title={skill.description}>
                    {skill.description}
                  </span>
                </span>
              </button>
              {skill.enabled ? (
                <button
                  type="button"
                  className={`composer-skill-menu__insert${inserted ? " composer-skill-menu__insert--selected" : ""}`}
                  aria-label={inserted
                    ? t("composer.skillQuick.remove", { name: skill.name })
                    : t("composer.skillQuick.insert", { name: skill.name })}
                  onClick={() => {
                    if (inserted) onRemove(skill.name);
                    else onInsert(skill.name);
                  }}
                >
                  {inserted ? <Check size={14} aria-hidden /> : <Plus size={15} aria-hidden />}
                </button>
              ) : (
                <button
                  type="button"
                  className="composer-connector-menu__action"
                  disabled={busy}
                  onClick={() => void enable(skill)}
                >
                  {busy && <LoaderCircle className="is-spinning" size={13} aria-hidden />}
                  {t("composer.skillQuick.enable")}
                </button>
              )}
            </div>
          );
        })}
      </div>

      <button
        type="button"
        className="composer-connector-menu__more"
        onClick={() => {
          onRequestClose();
          onMore();
        }}
      >
        <Hammer size={15} aria-hidden />
        <span>{t("composer.skillQuick.more")}</span>
        <ChevronRight size={14} aria-hidden />
      </button>
    </div>
  );
}
