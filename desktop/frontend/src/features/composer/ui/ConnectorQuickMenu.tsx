import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertCircle, Cable, ChevronRight, ExternalLink, LoaderCircle, RefreshCw } from "lucide-react";
import { app, openExternal } from "../../../lib/bridge";
import { reportClientError } from "../../../lib/clientError";
import { useT } from "../../../lib/i18n";
import type { ServerView } from "../../../lib/types";
import { connectorDisplayName, registerConnectorCatalog } from "../../connectors/catalog";
import { isConnectorTargetUnavailable } from "../../connectors/target";
import { ConnectorBrandIcon } from "../../connectors/ui/ConnectorBrandIcon";
import type { ConnectorCatalogItemView } from "../../../lib/bridge";

type ConnectorQuickMenuProps = {
  open: boolean;
  tabId?: string;
  selectedNames: readonly string[];
  onToggle: (name: string) => void;
  onMore: () => void;
  onRequestClose: () => void;
  variant?: "full" | "connected-switches";
};

function usable(server: ServerView): boolean {
  return server.status === "connected" && server.tools > 0;
}

function connecting(server: ServerView): boolean {
  return server.status === "initializing" || server.status === "deferred";
}

function authRequired(server: ServerView): boolean {
  return server.authStatus === "required" && /^https?:\/\//i.test((server.authUrl || "").trim());
}

function statusRank(server: ServerView): number {
  if (usable(server)) return 0;
  if (connecting(server)) return 1;
  if (authRequired(server)) return 2;
  if (server.status === "failed") return 3;
  return 4;
}

function statusLabel(server: ServerView, selected: boolean, t: ReturnType<typeof useT>): string {
  if (usable(server)) {
    if (selected) return t("composer.connectorQuick.enabledForTask");
    return t("composer.connectorQuick.connected", { count: server.tools });
  }
  if (connecting(server)) return t("composer.connectorQuick.connecting");
  if (authRequired(server)) return t("composer.connectorQuick.authRequired");
  if (server.status === "failed") return server.error || t("composer.connectorQuick.failed");
  return t("composer.connectorQuick.disconnected");
}

export function ConnectorQuickMenu({
  open,
  tabId,
  selectedNames,
  onToggle,
  onMore,
  onRequestClose,
  variant = "full",
}: ConnectorQuickMenuProps) {
  const t = useT();
  const [servers, setServers] = useState<ServerView[]>([]);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshFeedback, setRefreshFeedback] = useState("");
  const [busyName, setBusyName] = useState("");
  const [error, setError] = useState("");
	const [managedNames, setManagedNames] = useState<Set<string>>(() => new Set());
  const [catalogItems, setCatalogItems] = useState<Map<string, ConnectorCatalogItemView>>(() => new Map());
  const selected = useMemo(() => new Set(selectedNames), [selectedNames]);

  const reload = useCallback(async () => {
    const [configured, catalog, managedStates] = await Promise.all([
      app.MCPServers(),
		tabId ? app.ConnectorCatalog(tabId) : Promise.resolve({ catalog_version: "", items: [] }),
      tabId ? app.ConnectorStates(tabId) : Promise.resolve([]),
    ]);
	registerConnectorCatalog(catalog.items);
	setCatalogItems(new Map(catalog.items.map((item) => [item.id, item])));
	const ids = new Set(catalog.items.map((item) => item.id));
	setManagedNames(ids);
	const states = new Map(managedStates.map((state) => [state.connector_id, state]));
	const ordinary = configured.filter((server) => !catalog.items.some((item) => server.name.startsWith(`${item.id}--`)));
	const managed = catalog.items.map((item): ServerView => {
		const state = states.get(item.id);
		const connected = state?.status === "connected";
		const status: ServerView["status"] = connected ? "connected"
			: state?.status === "authorizing" || state?.status === "installing" ? "initializing"
			: state?.status === "failed" || state?.status === "repair_required" ? "failed" : "disabled";
		return { name: item.id, transport: item.kind, status, configured: true, autoStart: true,
			tools: connected ? Math.max(1, state?.tools ?? 0) : 0, prompts: 0, resources: 0, error: state?.error?.message };
	});
	const next = [...ordinary, ...managed].sort((left, right) => statusRank(left) - statusRank(right) || left.name.localeCompare(right.name));
    setServers(next);
    return next;
  }, [tabId]);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    setRefreshFeedback("");
    setError("");
    try {
      const next = await reload();
      setRefreshFeedback(t("composer.connectorQuick.refreshed", { count: next.length }));
    } catch (cause) {
      setError(reportClientError("connectors.quick.load", cause, {
        fallbackMessage: "加载连接器失败，请稍后重试。",
      }).userMessage);
    } finally {
      setRefreshing(false);
    }
  }, [reload, t]);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoading(true);
    setRefreshFeedback("");
    setError("");
    void reload()
      .catch((cause) => {
        if (!cancelled) setError(reportClientError("connectors.quick.load", cause, {
          fallbackMessage: "加载连接器失败，请稍后重试。",
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
    if (!refreshFeedback) return;
    const id = window.setTimeout(() => setRefreshFeedback(""), 1800);
    return () => window.clearTimeout(id);
  }, [refreshFeedback]);

  useEffect(() => {
    if (!open || !servers.some(connecting)) return;
		const id = window.setInterval(() => {
			void reload().catch((cause) => {
				if (isConnectorTargetUnavailable(cause)) {
					window.clearInterval(id);
					return;
				}
				setError(reportClientError("connectors.quick.poll", cause, {
					fallbackMessage: "加载连接器状态失败，请稍后重试。",
				}).userMessage);
			});
		}, 2000);
    return () => window.clearInterval(id);
  }, [open, reload, servers]);

  const connect = async (server: ServerView) => {
    setBusyName(server.name);
    setError("");
    try {
		if (managedNames.has(server.name)) {
			onRequestClose();
			onMore();
			return;
		}
      if (server.configured === false) {
		throw new Error(t("composer.connectorQuick.notUsable"));
      } else if (server.status === "disabled") {
        await app.SetMCPServerEnabled(server.name, true);
      } else {
        await app.ReconnectMCPServer(server.name);
      }
      const next = await reload();
      const connected = next.find((candidate) => candidate.name === server.name);
      if (connected && usable(connected)) {
        setRefreshFeedback(t("composer.connectorQuick.connected", { count: connected.tools }));
        return;
      }
      if (connected && authRequired(connected)) return;
      throw new Error(connected?.error || t("composer.connectorQuick.notUsable"));
    } catch (cause) {
      setError(reportClientError("connectors.quick.retry", cause, {
        fallbackMessage: "重试连接器失败，请稍后重试。",
      }).userMessage);
    } finally {
      setBusyName("");
    }
  };

  const connectedSwitches = variant === "connected-switches";
  const visibleServers = connectedSwitches ? servers.filter(usable) : servers;
  const missingSelections = connectedSwitches
    ? []
    : selectedNames.filter((name) => !servers.some((server) => server.name === name));

  return (
    <div className={`composer-connector-menu${connectedSwitches ? " composer-connector-menu--switches" : ""}`} role="menu" aria-label={t("composer.connectorQuick.title")}>
      {!connectedSwitches && <div className="composer-connector-menu__head">
        <div>
          <strong>{t("composer.connectorQuick.title")}</strong>
          <span aria-live="polite">
            {refreshing
              ? t("composer.connectorQuick.refreshing")
              : refreshFeedback || t("composer.connectorQuick.subtitle")}
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
      </div>}

      {error && (
        <div className="composer-connector-menu__error" role="alert">
          <AlertCircle size={14} aria-hidden />
          <span>{error}</span>
        </div>
      )}

      <div className="composer-connector-menu__list">
        {loading && servers.length === 0 ? (
          <div className="composer-connector-menu__empty">
            <LoaderCircle className="is-spinning" size={16} aria-hidden />
            {t("common.loading")}
          </div>
        ) : null}

        {!loading && visibleServers.length === 0 && missingSelections.length === 0 ? (
          <div className="composer-connector-menu__empty">
            {connectedSwitches ? "当前没有已连接的连接器" : t("composer.connectorQuick.empty")}
          </div>
        ) : null}

        {visibleServers.map((server) => {
          const enabled = selected.has(server.name);
          const isBusy = busyName === server.name;
          const isConnecting = connecting(server) || isBusy;
          const canToggle = usable(server);
          return (
            <div
              className={`composer-connector-menu__row${enabled ? " composer-connector-menu__row--selected" : ""}`}
              key={server.name}
            >
              <button
                type="button"
                className="composer-connector-menu__main"
                disabled={!canToggle || isBusy}
                onClick={() => onToggle(server.name)}
              >
                <ConnectorBrandIcon item={catalogItems.get(server.name)} name={connectorDisplayName(server.name)} size={32} />
                <span className="composer-connector-menu__copy">
                  <strong>{connectorDisplayName(server.name)}</strong>
                  {!connectedSwitches && <span title={server.error}>{statusLabel(server, enabled, t)}</span>}
                </span>
              </button>

              {canToggle ? (
                <label className="composer-connector-menu__switch">
                  <input
                    type="checkbox"
                    checked={enabled}
                    aria-label={t("composer.connectorQuick.toggleTask", { name: connectorDisplayName(server.name) })}
                    onChange={() => onToggle(server.name)}
                  />
                  <span aria-hidden />
                </label>
              ) : authRequired(server) ? (
                <button
                  type="button"
                  className="composer-connector-menu__action"
                  onClick={() => void openExternal((server.authUrl || "").trim())}
                >
                  <ExternalLink size={13} aria-hidden />
                  {t("composer.connectorQuick.authorize")}
                </button>
              ) : (
                <button
                  type="button"
                  className="composer-connector-menu__action"
                  disabled={isConnecting}
                  onClick={() => void connect(server)}
                >
                  {isConnecting && <LoaderCircle className="is-spinning" size={13} aria-hidden />}
                  {server.status === "failed"
                    ? t("composer.connectorQuick.retry")
                    : t("composer.connectorQuick.connect")}
                </button>
              )}
            </div>
          );
        })}

        {missingSelections.map((name) => (
          <div className="composer-connector-menu__row composer-connector-menu__row--missing" key={`missing:${name}`}>
            <button type="button" className="composer-connector-menu__main" onClick={() => onToggle(name)}>
              <AlertCircle size={14} aria-hidden />
              <span className="composer-connector-menu__copy">
                <strong>{connectorDisplayName(name)}</strong>
                <span>{t("composer.connectorQuick.removed")}</span>
              </span>
            </button>
            <button type="button" className="composer-connector-menu__action" onClick={() => onToggle(name)}>
              {t("common.delete")}
            </button>
          </div>
        ))}
      </div>

      {!connectedSwitches && <button
        type="button"
        className="composer-connector-menu__more"
        onClick={() => {
          onRequestClose();
          onMore();
        }}
      >
        <Cable size={15} aria-hidden />
        <span>{t("composer.connectorQuick.more")}</span>
        <ChevronRight size={14} aria-hidden />
      </button>}
    </div>
  );
}
