import { useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";
import { useT } from "../lib/i18n";
import { Markdown } from "./Markdown";
import { ModalCloseButton } from "./ModalCloseButton";

export function UpdateReleaseNotesDialog({
  version,
  notes,
  onClose,
}: {
  version: string;
  notes: string;
  onClose: () => void;
}) {
  const t = useT();
  const titleID = useId();
  const onCloseRef = useRef(onClose);
  const restoreFocusRef = useRef<HTMLElement | null>(
    document.activeElement instanceof HTMLElement ? document.activeElement : null,
  );
  onCloseRef.current = onClose;

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      onCloseRef.current();
    };
    window.addEventListener("keydown", closeOnEscape, true);
    return () => {
      window.removeEventListener("keydown", closeOnEscape, true);
      const restoreFocus = restoreFocusRef.current;
      if (restoreFocus) {
        window.requestAnimationFrame(() => {
          // React StrictMode replays effects without removing the mounted dialog.
          if (document.getElementById(titleID)?.isConnected) return;
          restoreFocusRef.current = null;
          if (restoreFocus.isConnected) restoreFocus.focus();
        });
      }
    };
  }, []);

  return createPortal(
    <div
      className="management-modal-backdrop update-notes-modal-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        className="management-modal update-notes-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="management-modal__head">
          <div>
            <div id={titleID} className="management-modal__title">{t("updater.releaseNotes")}</div>
            <div className="management-modal__summary">{version}</div>
          </div>
          <div className="management-modal__actions">
            <ModalCloseButton
              autoFocus
              showTooltipOnFocus={false}
              label={t("common.close")}
              onClick={onClose}
            />
          </div>
        </header>
        <div className="update-notes-modal__body">
          <Markdown text={notes.trim() || t("updater.noReleaseNotes")} />
        </div>
      </section>
    </div>,
    document.body,
  );
}
