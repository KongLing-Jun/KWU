import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { CircleAlert, CircleCheckBig, Download, RefreshCw } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import { manualUpdateHintKey, useUpdater, type UpdateStatus } from "../lib/useUpdater";
import { ModalCloseButton } from "./ModalCloseButton";
import { UpdateReleaseNotesDialog } from "./UpdateReleaseNotesDialog";

const MB = 1024 * 1024;
const mb = (bytes: number) => (bytes / MB).toFixed(1);

export function UpdateCenterDialog({ onClose }: { onClose: () => void }) {
  const t = useT();
  const titleID = useId();
  const onCloseRef = useRef(onClose);
  const checkedRef = useRef(false);
  const restoreFocusRef = useRef<HTMLElement | null>(
    document.activeElement instanceof HTMLElement ? document.activeElement : null,
  );
  const [version, setVersion] = useState("");
  const [notesOpen, setNotesOpen] = useState(false);
  const { status, check, download, install } = useUpdater();
  onCloseRef.current = onClose;

  useEffect(() => {
    if (checkedRef.current) return;
    checkedRef.current = true;
    void app.Version().then(setVersion).catch(() => setVersion("—"));
    void check();
  }, [check]);

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || notesOpen) return;
      event.preventDefault();
      event.stopPropagation();
      onCloseRef.current();
    };
    window.addEventListener("keydown", closeOnEscape, true);
    return () => {
      window.removeEventListener("keydown", closeOnEscape, true);
      const restoreFocus = restoreFocusRef.current;
      if (!restoreFocus) return;
      window.requestAnimationFrame(() => {
        if (document.getElementById(titleID)?.isConnected) return;
        restoreFocusRef.current = null;
        if (restoreFocus.isConnected) restoreFocus.focus();
      });
    };
  }, [notesOpen, titleID]);

  const busy = status.kind === "checking"
    || status.kind === "downloading"
    || status.kind === "verifying"
    || status.kind === "installing";

  return createPortal(
    <div
      className="management-modal-backdrop update-center-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      <section
        className="management-modal update-center"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="management-modal__head update-center__head">
          <div className="update-center__heading">
            <div id={titleID} className="management-modal__title">{t("updater.title")}</div>
            <div className="management-modal__summary">{t("updater.currentVersion", { v: version || "—" })}</div>
          </div>
          <div className="management-modal__actions">
            <ModalCloseButton
              autoFocus
              showTooltipOnFocus={false}
              label={t("common.close")}
              onClick={onClose}
              disabled={busy}
            />
          </div>
        </header>

        <div className="update-center__body">
          <div
            className={`update-center__status update-center__status--${status.kind}`}
            role="status"
            aria-busy={busy}
          >
            <span className="update-center__status-icon" aria-hidden="true">
              {status.kind === "upToDate" || status.kind === "done"
                ? <CircleCheckBig size={22} strokeWidth={1.9} />
                : status.kind === "available" || status.kind === "downloaded"
                  ? <Download size={22} strokeWidth={1.9} />
                  : status.kind === "error"
                    ? <CircleAlert size={22} strokeWidth={1.9} />
                    : <RefreshCw className={busy ? "update-center__spin" : undefined} size={22} strokeWidth={1.9} />}
            </span>
            <div className="update-center__status-copy">
              <strong>{updateStatusTitle(status, t)}</strong>
              {status.kind === "available" && (
                <span>{t("updater.channelLabel", { channel: status.info.channel || "stable" })}</span>
              )}
              {status.kind === "error" && <span>{t("updater.failed", { msg: status.message })}</span>}
              {status.kind === "available" && !status.info.canSelfUpdate && (
                <span>{status.info.manualReason || t(manualUpdateHintKey(status.info.manualKind))}</span>
              )}
              {status.kind === "downloading" && (
                <div className="update-center__progress">
                  <progress value={status.received} max={status.total || undefined} />
                  <span>{t("updater.downloading", {
                    done: mb(status.received),
                    total: mb(status.total),
                    pct: status.total > 0 ? Math.round((status.received / status.total) * 100) : 0,
                  })}</span>
                </div>
              )}
            </div>
          </div>

          <div className="update-center__actions">
            {status.kind === "available" && (
              <>
                <button className="btn btn--small" type="button" onClick={() => setNotesOpen(true)}>
                  {t("updater.viewReleaseNotes")}
                </button>
                <button className="btn btn--small btn--primary" type="button" onClick={() => download(status.info)}>
                  <Download size={14} strokeWidth={2} aria-hidden="true" />
                  {status.info.canSelfUpdate ? t("updater.downloadUpdate") : t("updater.goToDownload")}
                </button>
              </>
            )}
            {status.kind === "downloaded" && (
              <button className="btn btn--small btn--primary" type="button" onClick={install}>
                {t("updater.restartInstall")}
              </button>
            )}
            {!busy && status.kind !== "downloaded" && (
              <button className="btn btn--small" type="button" onClick={() => void check()}>
                <RefreshCw size={14} strokeWidth={2} aria-hidden="true" />
                {status.kind === "error" ? t("updater.retry") : t("updater.checkButton")}
              </button>
            )}
          </div>
        </div>

        {notesOpen && status.kind === "available" && (
          <UpdateReleaseNotesDialog
            version={status.info.latest}
            notes={status.info.notes}
            onClose={() => setNotesOpen(false)}
          />
        )}
      </section>
    </div>,
    document.body,
  );
}

function updateStatusTitle(status: UpdateStatus, t: ReturnType<typeof useT>): string {
  switch (status.kind) {
    case "checking": return t("updater.checking");
    case "upToDate": return t("updater.upToDate");
    case "available": return t("updater.available", { v: status.info.latest });
    case "downloading": return t("updater.downloading", { done: mb(status.received), total: mb(status.total), pct: status.total > 0 ? Math.round((status.received / status.total) * 100) : 0 });
    case "verifying": return t("updater.verifying");
    case "downloaded": return t("updater.downloaded", { v: status.info.latest });
    case "installing": return t("updater.installing");
    case "done": return t("updater.done");
    case "error": return t("updater.retry");
    default: return t("updater.checkButton");
  }
}
