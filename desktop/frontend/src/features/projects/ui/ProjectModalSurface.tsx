import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from "react";
import { Maximize2, Minimize2, X } from "lucide-react";

const focusableSelector = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
const inertBackgroundElements = new Map<HTMLElement, { count: number; wasInert: boolean }>();

function acquireBackgroundInert(element: HTMLElement) {
  const current = inertBackgroundElements.get(element);
  if (current) {
    current.count += 1;
    return;
  }
  inertBackgroundElements.set(element, { count: 1, wasInert: element.hasAttribute("inert") });
  element.setAttribute("inert", "");
}

function releaseBackgroundInert(element: HTMLElement) {
  const current = inertBackgroundElements.get(element);
  if (!current) return;
  if (current.count > 1) {
    current.count -= 1;
    return;
  }
  inertBackgroundElements.delete(element);
  if (!current.wasInert) element.removeAttribute("inert");
}

function visibleFocusableElements(root: HTMLElement) {
  return Array.from(root.querySelectorAll<HTMLElement>(focusableSelector)).filter((element) => {
    const style = window.getComputedStyle(element);
    return style.display !== "none" && style.visibility !== "hidden";
  });
}

export function ProjectModalSurface({
  backdropClassName,
  className,
  role = "dialog",
  titleId,
  descriptionId,
  pending = false,
  suspended = false,
  showClose = true,
  closeLabel = "关闭",
  initialFocus = "content",
  returnFocus,
  initialFocusRef,
  onClose,
  children,
}: {
  backdropClassName: string;
  className: string;
  role?: "dialog" | "alertdialog";
  titleId: string;
  descriptionId?: string;
  pending?: boolean;
  suspended?: boolean;
  showClose?: boolean;
  closeLabel?: string;
  initialFocus?: "content" | "close";
  returnFocus?: HTMLElement | null;
  initialFocusRef?: RefObject<HTMLElement | null>;
  onClose: () => void;
  children: ReactNode;
}) {
  const backdropRef = useRef<HTMLDivElement>(null);
  const surfaceRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const capturedFocusRef = useRef<HTMLElement | null>(null);
  const [maximized, setMaximized] = useState(false);

  useEffect(() => {
    if (suspended || !backdropRef.current) return;
    const background: HTMLElement[] = [];
    let modalBranch: HTMLElement = backdropRef.current;
    while (modalBranch.parentElement) {
      const parent = modalBranch.parentElement;
      for (const sibling of Array.from(parent.children)) {
        if (sibling !== modalBranch && sibling instanceof HTMLElement) {
          acquireBackgroundInert(sibling);
          background.push(sibling);
        }
      }
      if (parent === document.body) break;
      modalBranch = parent;
    }
    return () => {
      for (const element of background.reverse()) releaseBackgroundInert(element);
    };
  }, [suspended]);

  useEffect(() => {
    capturedFocusRef.current = returnFocus || (document.activeElement instanceof HTMLElement ? document.activeElement : null);
    const requestedFocus = initialFocusRef?.current;
    const focusTarget = initialFocus === "close"
      ? closeRef.current
      : requestedFocus && window.getComputedStyle(requestedFocus).display !== "none"
        ? requestedFocus
        : surfaceRef.current ? visibleFocusableElements(surfaceRef.current)[0] ?? surfaceRef.current : null;
    focusTarget?.focus();
    return () => capturedFocusRef.current?.focus();
  }, [initialFocus, initialFocusRef, returnFocus]);

  const handleKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (suspended) return;
    if (event.key === "Escape" && !pending) {
      if (surfaceRef.current?.querySelector('[aria-expanded="true"]')) return;
      event.preventDefault();
      onClose();
      return;
    }
    if (event.key !== "Tab" || !surfaceRef.current) return;
    const controls = visibleFocusableElements(surfaceRef.current);
    if (controls.length === 0) {
      event.preventDefault();
      surfaceRef.current.focus();
      return;
    }
    const first = controls[0]!;
    const last = controls[controls.length - 1]!;
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return <div ref={backdropRef} className={backdropClassName} role="presentation" data-project-modal-suspended={suspended || undefined} onMouseDown={(event) => {
    if (event.target === event.currentTarget && !pending && !suspended) onClose();
  }}>
    <section
      ref={surfaceRef}
      className={`${className} project-modal-surface${maximized ? " project-modal-surface--maximized" : ""}`}
      role={role}
      aria-modal={suspended ? undefined : "true"}
      aria-hidden={suspended || undefined}
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      aria-busy={pending || undefined}
      inert={suspended ? true : undefined}
      tabIndex={-1}
      onKeyDown={handleKeyDown}
      onMouseDown={(event) => event.stopPropagation()}
    >
      <div className="project-modal-surface__controls">
        <button
          type="button"
          className="project-modal-surface__resize"
          aria-label={maximized ? "还原弹窗" : "放大弹窗"}
          aria-pressed={maximized}
          title={maximized ? "还原弹窗" : "放大弹窗"}
          onClick={() => setMaximized((current) => !current)}
        >
          {maximized ? <Minimize2 size={17} aria-hidden="true" /> : <Maximize2 size={17} aria-hidden="true" />}
        </button>
        {showClose ? <button ref={closeRef} type="button" className="project-modal-surface__close" aria-label={closeLabel} title={closeLabel} disabled={pending} onClick={onClose}><X size={18} aria-hidden="true" /></button> : null}
      </div>
      {children}
    </section>
  </div>;
}
