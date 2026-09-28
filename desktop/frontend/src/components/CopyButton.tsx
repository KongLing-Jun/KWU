import { useEffect, useRef, useState } from "react";
import { Check, Copy } from "lucide-react";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";

function fallbackCopyText(value: string): boolean {
  const activeElement = document.activeElement;
  const selection = document.getSelection();
  const ranges: Range[] = [];
  if (selection) {
    for (let index = 0; index < selection.rangeCount; index += 1) {
      ranges.push(selection.getRangeAt(index));
    }
  }
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.inset = "0 auto auto 0";
  textarea.style.width = "1px";
  textarea.style.height = "1px";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  let ok = false;
  try {
    ok = document.execCommand("copy");
  } finally {
    textarea.remove();
    if (selection) {
      selection.removeAllRanges();
      for (const range of ranges) selection.addRange(range);
    }
    if (activeElement instanceof HTMLElement) activeElement.focus();
  }
  return ok;
}

async function writeClipboardText(value: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(value);
    return;
  } catch {
    /* try the desktop runtime below */
  }
  try {
    if (typeof window !== "undefined" && (await window.runtime?.ClipboardSetText?.(value))) return;
  } catch {
    /* runtime unavailable in browser dev */
  }
  if (fallbackCopyText(value)) return;
  throw new Error("clipboard unavailable");
}

// CopyButton copies text to the clipboard and only acknowledges a completed write.
export function CopyButton({
  text,
  getText,
  className,
  label,
  showInlineLabel = true,
}: {
  text?: string;
  getText?: () => string | Promise<string>;
  className?: string;
  label?: string;
  showInlineLabel?: boolean;
}) {
  const t = useT();
  const { showError } = useToast();
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<number | null>(null);
  const actionLabel = label ?? t("msg.copy");
  const stateLabel = copied ? t("msg.copied") : actionLabel;

  useEffect(() => {
    return () => {
      if (timerRef.current != null) window.clearTimeout(timerRef.current);
    };
  }, []);

  const copy = async () => {
    try {
      const value = getText ? await getText() : text ?? "";
      await writeClipboardText(value);
      setCopied(true);
      if (timerRef.current != null) window.clearTimeout(timerRef.current);
      timerRef.current = window.setTimeout(() => {
        setCopied(false);
        timerRef.current = null;
      }, 1200);
    } catch (error) {
      showError("clipboard.copy", error, "复制失败，请稍后重试。");
    }
  };
  return (
    <button
      className={[
        "copybtn",
        copied ? "copybtn--copied" : "",
        className ?? "",
      ].filter(Boolean).join(" ")}
      onClick={copy}
      aria-label={stateLabel}
      title={actionLabel}
      type="button"
    >
      {copied ? <Check size={13} /> : <Copy size={13} />}
      {showInlineLabel && (
        <span className="copybtn__label-inline">{stateLabel}</span>
      )}
    </button>
  );
}
