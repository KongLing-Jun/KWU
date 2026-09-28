import { useCallback, useEffect, useState } from "react";
import { app, onUpdaterProgress } from "./bridge";
import { reportClientError } from "./clientError";
import type { DictKey } from "./i18n";
import type { UpdateInfo } from "./types";

// useUpdater drives the auto-update state machine shared by the top banner and the
// Settings panel: check, download/verify, then a separate restart/install action.

export type UpdateStatus =
  | { kind: "idle" }
  | { kind: "checking" }
  | { kind: "upToDate"; current: string }
  | { kind: "available"; info: UpdateInfo }
  | { kind: "downloading"; received: number; total: number; info: UpdateInfo }
  | { kind: "verifying"; info: UpdateInfo }
  | { kind: "downloaded"; info: UpdateInfo }
  | { kind: "installing"; info?: UpdateInfo }
  | { kind: "done" }
  | { kind: "error"; message: string };

export interface Updater {
  status: UpdateStatus;
  check: () => Promise<void>;
  download: (info: UpdateInfo) => void;
  install: () => void;
  openDownload: () => void;
  reset: () => void;
}

function errMsg(e: unknown): string {
  return reportClientError("updater.operation", e, {
    fallbackMessage: "更新操作失败，请稍后重试。",
  }).userMessage;
}

export function manualUpdateHintKey(kind?: UpdateInfo["manualKind"]): DictKey {
  switch (kind) {
    case "windows-portable":
      return "updater.windowsPortableHint";
    case "macos":
      return "updater.macHint";
    case "linux":
      return "updater.linuxHint";
    default:
      return "updater.manualHint";
  }
}

export function useUpdater(): Updater {
  const [status, setStatus] = useState<UpdateStatus>({ kind: "idle" });

  // A single long-lived subscription advances the state machine through the apply
  // phases. It reads the in-flight info from the current state so progress events
  // carry the version/notes forward without re-plumbing them.
  useEffect(() => {
    return onUpdaterProgress((p) => {
      setStatus((cur) => {
        const info = "info" in cur ? cur.info : undefined;
        switch (p.phase) {
          case "downloading":
            return info ? { kind: "downloading", received: p.received, total: p.total, info } : cur;
          case "verifying":
            return info ? { kind: "verifying", info } : cur;
          case "downloaded":
            return info ? { kind: "downloaded", info: { ...info, downloaded: true } } : cur;
          case "installing":
            return { kind: "installing", info };
          case "done":
            return { kind: "done" };
          case "error":
            return { kind: "error", message: p.err ?? "update failed" };
          default:
            return cur;
        }
      });
    });
  }, []);

  const check = useCallback(async () => {
    setStatus({ kind: "checking" });
    try {
      const info = await app.CheckUpdate();
      if (!info) {
        setStatus({ kind: "upToDate", current: "" });
        return;
      }
      if (info.err) {
        setStatus({ kind: "error", message: info.err });
        return;
      }
      if (!info.available) {
        setStatus({ kind: "upToDate", current: info.current });
        return;
      }
      setStatus(info.downloaded ? { kind: "downloaded", info } : { kind: "available", info });
    } catch (e) {
      setStatus({ kind: "error", message: errMsg(e) });
    }
  }, []);

  const download = useCallback((info: UpdateInfo) => {
    if (!info.canSelfUpdate) {
      void app.OpenUpdateDownload(info.downloadUrl);
      return;
    }
    setStatus({ kind: "downloading", received: 0, total: info.assetSize, info });
    void app.DownloadUpdate()
      .then((result) => {
        if (result) setStatus({ kind: "downloaded", info: { ...info, downloaded: true } });
      })
      .catch((e) => setStatus({ kind: "error", message: errMsg(e) }));
  }, []);

  const install = useCallback(() => {
    setStatus((cur) => ("info" in cur ? { kind: "installing", info: cur.info } : { kind: "installing" }));
    void app.InstallUpdate().catch((e) => setStatus({ kind: "error", message: errMsg(e) }));
  }, []);

  const openDownload = useCallback(() => {
    void app.OpenDownloadPage();
  }, []);

  const reset = useCallback(() => setStatus({ kind: "idle" }), []);

  return { status, check, download, install, openDownload, reset };
}
