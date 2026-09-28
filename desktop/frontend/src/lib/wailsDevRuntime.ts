import { t } from "./i18n";

type RuntimeWindow = Window & {
  runtime?: unknown;
  go?: { main?: { App?: object } };
  WailsInvoke?: (message: string) => void;
  __KWU_BROWSER_RUNTIME_READY_SUPPRESSED__?: boolean;
};

const BINDING_WAIT_MS = 3_000;
const TRANSPORT_RETRY_MS = 250;
const TRANSPORT_PROBE_MESSAGE = "LDkwu browser runtime connection probe";
const CONNECTION_OVERLAY_ID = "runtime-connection-overlay";

type WailsInvoke = (message: string) => void;
type TransportWaitOptions = {
  timeoutMs?: number;
  retryMs?: number;
  sleep?: (ms: number) => Promise<void>;
};

let baseBrowserInvoke: WailsInvoke | null = null;
let reconnecting: Promise<void> | null = null;
let pageUnloading = false;

function runtimeWindow(): RuntimeWindow {
  return window as RuntimeWindow;
}

function hasAppBinding(): boolean {
  return Boolean(runtimeWindow().go?.main?.App);
}

function errorMessage(error: unknown): string {
  if (error && typeof error === "object" && "message" in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string") return message;
  }
  return String(error);
}

export function isTransientWailsConnectionError(error: unknown): boolean {
  const message = errorMessage(error);
  return (
    /WebSocket.*(?:CONNECTING|CLOSING|CLOSED)|Still in CONNECTING state/i.test(message) ||
    /Cannot read properties of null \(reading ['"]send['"]\)/i.test(message) ||
    /websocket:\s*(?:close sent|close \d+)/i.test(message)
  );
}

export async function waitForWailsTransport(
  invoke: WailsInvoke,
  options: TransportWaitOptions = {},
): Promise<boolean> {
  const timeoutMs = options.timeoutMs ?? BINDING_WAIT_MS;
  const retryMs = options.retryMs ?? TRANSPORT_RETRY_MS;
  const sleep = options.sleep ?? ((ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms)));
  const deadline = Date.now() + timeoutMs;

  do {
    try {
      invoke(TRANSPORT_PROBE_MESSAGE);
      return true;
    } catch (error) {
      if (!isTransientWailsConnectionError(error)) throw error;
    }
    await sleep(retryMs);
  } while (Date.now() < deadline);

  return false;
}

function showConnectionOverlay(): void {
  if (typeof document === "undefined" || document.getElementById(CONNECTION_OVERLAY_ID)) return;
  const host = document.createElement("section");
  host.id = CONNECTION_OVERLAY_ID;
  host.setAttribute("role", "status");
  host.setAttribute("aria-live", "polite");
  host.setAttribute("aria-busy", "true");

  const title = document.createElement("h1");
  title.className = "runtime-connection-overlay__title";
  title.textContent = t("runtimeConnection.title");
  const body = document.createElement("p");
  body.className = "runtime-connection-overlay__body";
  body.textContent = t("runtimeConnection.body");
  host.append(title, body);
  document.body.appendChild(host);
}

function hideConnectionOverlay(): void {
  if (typeof document !== "undefined") document.getElementById(CONNECTION_OVERLAY_ID)?.remove();
}

function recoverBrowserTransport(invoke: WailsInvoke): void {
  if (pageUnloading || reconnecting) return;
  showConnectionOverlay();
  reconnecting = waitForWailsTransport(invoke, { timeoutMs: Number.POSITIVE_INFINITY })
    .then((connected) => {
      if (connected && !pageUnloading) window.location.reload();
    })
    .finally(() => {
      reconnecting = null;
    });
}

function suppressBrowserRuntimeReady(): void {
  const target = runtimeWindow();
  if (target.__KWU_BROWSER_RUNTIME_READY_SUPPRESSED__ || typeof target.WailsInvoke !== "function") return;
  const invoke = target.WailsInvoke.bind(target);
  baseBrowserInvoke = invoke;
  target.WailsInvoke = (message: string) => {
    if (message === "runtime:ready") return;
    try {
      invoke(message);
    } catch (error) {
      if (!isTransientWailsConnectionError(error)) throw error;
      recoverBrowserTransport(invoke);
    }
  };
  window.addEventListener("beforeunload", () => {
    pageUnloading = true;
  }, { once: true });
  target.__KWU_BROWSER_RUNTIME_READY_SUPPRESSED__ = true;
}

function loadScript(src: string): Promise<void> {
  const existing = document.querySelector<HTMLScriptElement>(`script[src="${src}"]`);
  if (existing) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = src;
    script.async = false;
    script.dataset.wailsBrowserSync = "true";
    script.addEventListener("load", () => resolve(), { once: true });
    script.addEventListener("error", () => reject(new Error(`failed to load ${src}`)), { once: true });
    document.head.appendChild(script);
  });
}

async function waitForAppBinding(): Promise<boolean> {
  const deadline = Date.now() + BINDING_WAIT_MS;
  while (!hasAppBinding() && Date.now() < deadline) {
    await new Promise<void>((resolve) => window.setTimeout(resolve, 25));
  }
  return hasAppBinding();
}

async function waitForBackendAndReload(): Promise<never> {
  showConnectionOverlay();
  if (!baseBrowserInvoke) throw backendRequiredError();
  await waitForWailsTransport(baseBrowserInvoke, { timeoutMs: Number.POSITIVE_INFINITY });
  if (!pageUnloading) window.location.reload();
  return new Promise<never>(() => undefined);
}

function backendRequiredError(cause?: unknown): Error {
  const error = new Error(
    "Wails desktop backend is required. Start the desktop application and open its Wails dev URL; browser mock data is disabled.",
  );
  return cause === undefined ? error : Object.assign(error, { cause });
}

// In development, a plain Vite tab must connect to the local Wails devserver
// before React mounts. There is deliberately no browser-data fallback: both
// browser and desktop surfaces use the same Go bindings and persisted state.
export async function connectWailsDevRuntime(): Promise<void> {
  if (typeof window === "undefined" || typeof document === "undefined") return;
  if (hasAppBinding() || !import.meta.env.DEV) return;

  try {
    await loadScript("/wails/ipc.js");
    suppressBrowserRuntimeReady();
    await loadScript("/wails/runtime.js");
    if (!(await waitForAppBinding())) return waitForBackendAndReload();
    if (!baseBrowserInvoke || !(await waitForWailsTransport(baseBrowserInvoke))) return waitForBackendAndReload();
    hideConnectionOverlay();
  } catch (error) {
    if (error instanceof Error && error.message.startsWith("Wails desktop backend is required.")) throw error;
    throw backendRequiredError(error);
  }
}
