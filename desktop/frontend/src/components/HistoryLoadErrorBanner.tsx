import { useT } from "../lib/i18n";
import { RefreshCw } from "lucide-react";

export function HistoryLoadErrorBanner({ error, cached = false, onRetry }: { error?: string; cached?: boolean; onRetry?: () => void }) {
  const t = useT();
  if (!error) return null;
  return (
    <div className="banner banner--error banner--actionable" role="alert" data-history-load-error="true">
      <span className="banner__msg">{t(cached ? "history.syncFailed" : "history.loadFailed")}</span>
      <span className="banner__spacer" />
      {onRetry && <button type="button" className="btn btn--secondary btn--small" onClick={onRetry} title={t("common.retry")} aria-label={t("common.retry")}><RefreshCw size={14} /></button>}
      <details className="banner__details">
        <summary>{t("sendAvailability.details")}</summary>
        <code>{error}</code>
      </details>
    </div>
  );
}
