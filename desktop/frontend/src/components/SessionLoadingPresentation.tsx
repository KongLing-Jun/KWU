import { LoaderCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { useT } from "../lib/i18n";

export const SESSION_LOADING_FEEDBACK_DELAY_MS = 180;

export function useSessionLoadingFeedback(active: boolean, identity: string): boolean {
  const [feedback, setFeedback] = useState({ identity, visible: false });
  useEffect(() => {
    if (!active) {
      setFeedback((current) => current.visible || current.identity !== identity
        ? { identity, visible: false }
        : current);
      return;
    }
    setFeedback((current) => current.visible || current.identity !== identity
      ? { identity, visible: false }
      : current);
    const timer = window.setTimeout(() => {
      setFeedback({ identity, visible: true });
    }, SESSION_LOADING_FEEDBACK_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [active, identity]);
  return active && feedback.identity === identity && feedback.visible;
}

export function SessionLoadingPresentation({
  feedbackVisible,
  overlay = false,
  className = "",
}: {
  feedbackVisible: boolean;
  overlay?: boolean;
  className?: string;
}) {
  const t = useT();
  return (
    <div
      className={[
        "transcript__session-loading",
        overlay ? "transcript__session-loading--overlay" : "",
        className,
      ].filter(Boolean).join(" ")}
      role={feedbackVisible ? "status" : undefined}
      aria-live={feedbackVisible ? "polite" : undefined}
    >
      {feedbackVisible && (
        <div className="transcript__session-loading-label">
          <LoaderCircle aria-hidden="true" size={16} />
          <span>{t("transcript.loadingSession")}</span>
        </div>
      )}
    </div>
  );
}
