import { isTransientWailsConnectionError } from "../../lib/wailsDevRuntime";

interface WailsFileDropRuntime {
  BrowserOpenURL?(url: string): void;
  OnFileDrop?(cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean): void;
  OnFileDropOff?(): void;
}

const RECENT_NATIVE_FILE_DRAG_MS = 2000;
const WAILS_NON_FILE_DRAG_MESSAGE = "additional File object is not a file on the disk";
const UNCAUGHT_ERROR_PREFIX_RE = /^Uncaught(?:\s+\(in promise\))?(?:\s+\w*Error)?:\s*/i;

function fileDropRuntime(): WailsFileDropRuntime | undefined {
  if (typeof window === "undefined") return undefined;
  return (window as Window & { runtime?: WailsFileDropRuntime }).runtime;
}

function errorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    const msg = (err as { message?: unknown }).message;
    if (typeof msg === "string") return msg;
  }
  return String(err);
}

export function isWailsNonFileDragError(err: unknown, recentNativeFileDrag = false): boolean {
  const msg = errorMessage(err).trim().replace(UNCAUGHT_ERROR_PREFIX_RE, "");
  if (msg.includes(WAILS_NON_FILE_DRAG_MESSAGE)) return true;
  return recentNativeFileDrag && msg.toLowerCase() === "invalid argument";
}

export function isWailsNonFileDragErrorEvent(
  event: Pick<ErrorEvent, "error" | "message">,
  recentNativeFileDrag = false,
): boolean {
  if (isWailsNonFileDragError(event.error ?? event.message, recentNativeFileDrag)) return true;
  return event.error != null && isWailsNonFileDragError(event.message, recentNativeFileDrag);
}

function dataTransferLooksLikeFileDrag(dt: DataTransfer | null): boolean {
  if (!dt) return false;
  if (dt.files?.length > 0) return true;
  return Array.from(dt.types ?? []).includes("Files");
}

let wailsDragSuppressionRefs = 0;
let wailsDragSuppressionUninstall: (() => void) | null = null;
let lastNativeFileDragAt = 0;

export function installWailsNonFileDragErrorSuppression(): () => void {
  if (typeof window === "undefined") return () => {};

  wailsDragSuppressionRefs += 1;
  if (!wailsDragSuppressionUninstall) {
    const handleNativeFileDrag = (e: DragEvent) => {
      if (!dataTransferLooksLikeFileDrag(e.dataTransfer)) return;
      lastNativeFileDragAt = Date.now();
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = "copy";
    };
    const hasRecentNativeFileDrag = () => Date.now() - lastNativeFileDragAt <= RECENT_NATIVE_FILE_DRAG_MS;
    const suppressNonFileDragError = (e: ErrorEvent) => {
      if (isWailsNonFileDragErrorEvent(e, hasRecentNativeFileDrag())) {
        e.preventDefault();
      }
    };
    const suppressNonFileDragRejection = (e: PromiseRejectionEvent) => {
      if (isWailsNonFileDragError(e.reason, hasRecentNativeFileDrag())) {
        e.preventDefault();
      }
    };

    window.addEventListener("dragenter", handleNativeFileDrag, true);
    window.addEventListener("dragover", handleNativeFileDrag, true);
    window.addEventListener("drop", handleNativeFileDrag, true);
    window.addEventListener("error", suppressNonFileDragError);
    window.addEventListener("unhandledrejection", suppressNonFileDragRejection);
    wailsDragSuppressionUninstall = () => {
      window.removeEventListener("dragenter", handleNativeFileDrag, true);
      window.removeEventListener("dragover", handleNativeFileDrag, true);
      window.removeEventListener("drop", handleNativeFileDrag, true);
      window.removeEventListener("error", suppressNonFileDragError);
      window.removeEventListener("unhandledrejection", suppressNonFileDragRejection);
      lastNativeFileDragAt = 0;
    };
  }

  let disposed = false;
  return () => {
    if (disposed) return;
    disposed = true;
    wailsDragSuppressionRefs = Math.max(0, wailsDragSuppressionRefs - 1);
    if (wailsDragSuppressionRefs === 0 && wailsDragSuppressionUninstall) {
      wailsDragSuppressionUninstall();
      wailsDragSuppressionUninstall = null;
    }
  };
}

type FileDropPoint = { x: number; y: number };
type FileDropCallback = (paths: string[], point: FileDropPoint) => void;

const fileDropCallbacks = new Set<FileDropCallback>();
let fileDropRuntimeActive = false;
let fileDropSuppressionUninstall: (() => void) | null = null;

// onFilesDropped subscribes to native OS file drops. The callback gets the
// dropped files' absolute paths and drop point. No-op outside the Wails runtime.
export function onFilesDropped(cb: (paths: string[], point?: FileDropPoint) => void): () => void {
  const rt = fileDropRuntime();
  if (!rt?.OnFileDrop) return () => {};

  const wrapped: FileDropCallback = (paths, point) => cb(paths, point);
  fileDropCallbacks.add(wrapped);
  if (!fileDropRuntimeActive) {
    // Wails' internal ResolveFilePaths throws when a non-file object (e.g. the
    // window icon) is dragged onto the webview. The error is uncaught and crashes
    // the app. Intercept it here so only real file drops reach the callback.
    fileDropSuppressionUninstall = installWailsNonFileDragErrorSuppression();
    rt.OnFileDrop((x, y, paths) => {
      if (!Array.isArray(paths) || paths.length === 0) return;
      const point = { x, y };
      for (const listener of Array.from(fileDropCallbacks)) listener(paths, point);
    }, true);
    fileDropRuntimeActive = true;
  }

  let disposed = false;
  return () => {
    if (disposed) return;
    disposed = true;
    fileDropCallbacks.delete(wrapped);
    if (fileDropCallbacks.size === 0) {
      try {
        rt.OnFileDropOff?.();
      } catch (error) {
        if (!isTransientWailsConnectionError(error)) throw error;
      } finally {
        fileDropRuntimeActive = false;
        fileDropSuppressionUninstall?.();
        fileDropSuppressionUninstall = null;
      }
    }
  };
}

// openExternal opens a URL in the system browser (so links in rendered markdown
// don't navigate the webview away from the app). Falls back to window.open in the
// non-Wails browser contexts.
export function openExternal(url: string): void {
  if (typeof window !== "undefined" && window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(url);
  } else if (typeof window !== "undefined") {
    window.open(url, "_blank", "noopener");
  }
}
