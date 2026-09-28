// useWindowStatePersistence polls the Wails runtime for window geometry and
// persists it via SaveWindowState so the next launch restores the same size and
// position. No-op in browser dev (no window.runtime).
//
// The Go OnBeforeClose hook (app.saveWindowStateSync) provides the authoritative
// final save while the native window is still valid; this hook covers
// moves/resizes during the session.
//
// NOTE: navigator.sendBeacon or a sync XHR would let us block beforeunload,
// but Wails bindings are async (Go IPC). We accept that the very last resize
// event may not land; the 5s poll and the Go OnBeforeClose hook make this unlikely
// to matter.

import { useEffect, useRef } from "react";
import { app } from "./bridge";

type WailsWindowHost = {
  runtime?: {
    WindowGetSize?: unknown;
  };
  chrome?: {
    webview?: {
      postMessage?: unknown;
    };
  };
  webkit?: {
    messageHandlers?: {
      external?: {
        postMessage?: unknown;
      };
    };
  };
};

export function isNativeWailsWindow(target: unknown): boolean {
  if (!target || typeof target !== "object") return false;
  const host = target as WailsWindowHost;
  if (!host.runtime?.WindowGetSize) return false;
  return (
    typeof host.chrome?.webview?.postMessage === "function" ||
    typeof host.webkit?.messageHandlers?.external?.postMessage === "function"
  );
}

export function useWindowStatePersistence() {
  const lastState = useRef("");

  useEffect(() => {
    if (!isNativeWailsWindow(window)) return;
    const runtime = typeof window !== "undefined" ? window.runtime : undefined;
    if (!runtime?.WindowGetSize || !runtime.WindowGetPosition || !runtime.WindowIsMaximised) return;
    const { WindowGetSize, WindowGetPosition, WindowIsMaximised } = runtime;

    let timer: ReturnType<typeof setInterval>;
    let disposed = false;

    const save = async () => {
      if (disposed) return;
      try {
        const size = await WindowGetSize();
        if (disposed) return;
        const pos = await WindowGetPosition();
        if (disposed) return;
        const maximised = await WindowIsMaximised();
        if (disposed) return;
        const json = JSON.stringify({
          width: size.w,
          height: size.h,
          x: pos.x,
          y: pos.y,
          maximised,
        });
        if (json !== lastState.current) {
          await app.SaveWindowState({ width: size.w, height: size.h, x: pos.x, y: pos.y, maximised });
          lastState.current = json;
        }
      } catch {
        /* runtime not ready yet — poll will retry */
      }
    };

    // Debounced save on resize (500ms after the last resize event).
    let debounce: ReturnType<typeof setTimeout>;
    const onResize = () => {
      clearTimeout(debounce);
      debounce = setTimeout(save, 500);
    };
    window.addEventListener("resize", onResize);

    // Periodic poll every 5s for moves/maximise that don't trigger resize.
    timer = setInterval(save, 5000);

    return () => {
      disposed = true;
      clearInterval(timer);
      clearTimeout(debounce);
      window.removeEventListener("resize", onResize);
    };
  }, []);
}

export function useViewportHeightVar() {
  useEffect(() => {
    if (typeof window === "undefined" || typeof document === "undefined") return;

    let frame = 0;
    const root = document.documentElement;
    const setHeight = () => {
      frame = 0;
      const height = Math.round(window.visualViewport?.height ?? window.innerHeight);
      if (height > 0) root.style.setProperty("--app-viewport-height", `${height}px`);
    };
    const schedule = () => {
      if (frame) window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(setHeight);
    };

    schedule();
    window.addEventListener("resize", schedule);
    window.addEventListener("orientationchange", schedule);
    document.addEventListener("fullscreenchange", schedule);
    window.visualViewport?.addEventListener("resize", schedule);

    return () => {
      if (frame) window.cancelAnimationFrame(frame);
      window.removeEventListener("resize", schedule);
      window.removeEventListener("orientationchange", schedule);
      document.removeEventListener("fullscreenchange", schedule);
      window.visualViewport?.removeEventListener("resize", schedule);
    };
  }, []);
}
