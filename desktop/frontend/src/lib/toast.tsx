import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import { CheckCircle2 } from "lucide-react";
import { CopyButton } from "../components/CopyButton";
import { reportClientError } from "./clientError";

export interface Toast {
  id: number;
  text: string;
  level: "info" | "success" | "warn" | "error";
  actionLabel?: string;
  diagnosticId?: string;
}

export interface ToastContextValue {
  toasts: Toast[];
  showToast: (text: string, level?: Toast["level"], options?: ToastOptions) => void;
  showError: (scope: string, error: unknown, fallbackMessage?: string, options?: ToastOptions) => void;
}

const ToastContext = createContext<ToastContextValue>({ toasts: [], showToast: () => {}, showError: () => {} });

export interface ToastOptions {
  durationMs?: number;
  onDismiss?: () => void;
  actionLabel?: string;
  onAction?: () => void;
}

const DEFAULT_TOAST_DURATION_MS = 2500;

export function useToast() {
  return useContext(ToastContext);
}

let nextId = 1;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());
  const dismissCallbacks = useRef(new Map<number, () => void>());
  const actionCallbacks = useRef(new Map<number, () => void>());

  const dismissToast = useCallback((id: number) => {
    const timer = timers.current.get(id);
    if (timer) clearTimeout(timer);
    timers.current.delete(id);
    const onDismiss = dismissCallbacks.current.get(id);
    dismissCallbacks.current.delete(id);
    actionCallbacks.current.delete(id);
    setToasts((prev) => prev.filter((t) => t.id !== id));
    onDismiss?.();
  }, []);

  const enqueueToast = useCallback((text: string, level: Toast["level"], options: ToastOptions, diagnosticId?: string) => {
    const id = nextId++;
    if (options.onDismiss) dismissCallbacks.current.set(id, options.onDismiss);
    if (options.onAction) actionCallbacks.current.set(id, options.onAction);
    setToasts((prev) => [...prev, { id, text, level, actionLabel: options.actionLabel, diagnosticId }]);
    const timer = setTimeout(() => dismissToast(id), Math.max(0, options.durationMs ?? DEFAULT_TOAST_DURATION_MS));
    timers.current.set(id, timer);
  }, [dismissToast]);
  const showToast = useCallback((text: string, level: Toast["level"] = "info", options: ToastOptions = {}) => {
    enqueueToast(text, level, options);
  }, [enqueueToast]);
  const showError = useCallback((
    scope: string,
    error: unknown,
    fallbackMessage = "操作失败，请稍后重试。",
    options: ToastOptions = {},
  ) => {
    const presentation = reportClientError(scope, error, { fallbackMessage });
    enqueueToast(presentation.userMessage, "error", options, presentation.diagnosticId);
  }, [enqueueToast]);
  const activateToastAction = useCallback((id: number) => {
    const onAction = actionCallbacks.current.get(id);
    dismissToast(id);
    onAction?.();
  }, [dismissToast]);

  return (
    <ToastContext.Provider value={{ toasts, showToast, showError }}>
      {children}
      <div className="toast-container">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`toast toast--${t.level}`}
            role={t.level === "error" ? "alert" : "status"}
            aria-live={t.level === "error" ? "assertive" : "polite"}
            onClick={() => dismissToast(t.id)}
          >
            {t.level === "success" && <CheckCircle2 className="toast__icon" aria-hidden size={16} />}
            {t.level === "warn" && <span className="toast__icon">⚠️</span>}
            {t.level === "error" && <span className="toast__icon">❌</span>}
            <span className="toast__text">{t.text}</span>
            {t.diagnosticId ? (
              <span className="toast__diagnostic" translate="no" onClick={(event) => event.stopPropagation()}>
                <span>Log ID:</span>
                <code>{t.diagnosticId}</code>
                <CopyButton text={t.diagnosticId} showInlineLabel={false} />
              </span>
            ) : null}
            {t.actionLabel && (
              <button
                type="button"
                className="toast__action"
                onClick={(event) => {
                  event.stopPropagation();
                  activateToastAction(t.id);
                }}
              >
                {t.actionLabel}
              </button>
            )}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
