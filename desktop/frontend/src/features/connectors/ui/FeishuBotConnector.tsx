import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { app } from "../../../lib/bridge";
import { useT } from "../../../lib/i18n";
import type { SettingsView } from "../../../lib/types";
import { BotsSettingsSection } from "../../settings/ui/BotsSettingsSection";
import { modelRefsForSettings } from "../../settings/ui/ModelsSettingsSection";
import { ConnectorBrandIcon } from "./ConnectorBrandIcon";

const feishuIcon = new URL("../../../assets/channels/feishu-lark.png", import.meta.url).href;
const FEISHU_SEARCH_TEXT = "飞书 feishu 消息通道 机器人 个人智能体";

export function matchesFeishuConnectorQuery(query: string): boolean {
  return !query.trim() || FEISHU_SEARCH_TEXT.includes(query.trim().toLowerCase());
}

export function FeishuBotConnector({
  query,
  onChanged,
}: {
  query: string;
  onChanged?: () => void;
}) {
  const t = useT();
  const [settings, setSettings] = useState<SettingsView | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const visible = matchesFeishuConnectorQuery(query);

  const reload = useCallback(async () => {
    setError("");
    try {
      const next = await app.Settings();
      setSettings(next);
      return true;
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      return false;
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const modelRefs = useMemo(() => settings ? modelRefsForSettings(settings) : [], [settings]);
  const apply = useCallback(async (operation: () => Promise<unknown>) => {
    setBusy(true);
    setError("");
    try {
      await operation();
      if (await reload()) onChanged?.();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }, [onChanged, reload]);
  const refreshAfterInstall = useCallback(() => {
    void reload().then((ok) => {
      if (ok) onChanged?.();
    });
  }, [onChanged, reload]);

  if (!visible) return null;
  if (!settings) {
    const statusClass = error ? "configuration_error" : "connecting";
    const statusLabel = error || t("common.loading");
    return (
      <article
        className={`connector-catalog__card connector-catalog__card--${statusClass}`}
        data-connector-id="feishu-bot"
      >
        <button
          className="connector-catalog__card-main"
          type="button"
          disabled={!error}
          onClick={() => void reload()}
          aria-label={`${t("settings.botFeishu")} · ${statusLabel}`}
        >
          <ConnectorBrandIcon item={{ icon_url: feishuIcon }} name={t("settings.botFeishu")} size={40} />
          <span className="connector-catalog__card-copy">
            <span className="connector-catalog__card-title"><strong>{t("settings.botFeishu")}</strong></span>
            <span className="connector-catalog__description">{t("settings.botInstallFeishuHint")}</span>
            <span className={`connector-catalog__card-status connector-catalog__card-status--${statusClass}`} role={error ? "alert" : "status"}>
              {statusLabel}
            </span>
          </span>
          <span className={`connector-catalog__card-action connector-catalog__card-action--${statusClass}`} aria-hidden="true">
            {error ? <RefreshCw size={18} /> : <Loader2 className="connector-catalog__spinner" size={18} />}
          </span>
        </button>
      </article>
    );
  }

  return (
    <BotsSettingsSection
      s={settings}
      busy={busy}
      apply={apply}
      modelRefs={modelRefs}
      modelOptionsLoading={false}
      modelEditingDisabled={false}
      catalogTargets={["feishu"]}
      connectorCatalog
      onConnectionChanged={refreshAfterInstall}
    />
  );
}
