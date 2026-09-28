import { useEffect, useId, useRef, useState, type ReactNode, type RefObject } from "react";
import { Check, ChevronDown } from "lucide-react";
import { AnchoredPopover } from "./AnchoredPopover";

export interface ProjectSelectOption<T extends string> {
  value: T;
  label: string;
  prefix?: ReactNode;
  className?: string;
}

export function ProjectSelect<T extends string>({
  id,
  ariaLabel,
  value,
  options,
  disabled = false,
  prefix,
  className = "",
  menuClassName = "",
  triggerLabel,
  showSelectedCheck = true,
  showChevron = true,
  open: controlledOpen,
  onOpenChange,
  presentation = "popover",
  popoverPlacement = "bottom",
  popoverMinWidth,
  triggerRef: externalTriggerRef,
  onChange,
}: {
  id?: string;
  ariaLabel: string;
  value: T;
  options: Array<ProjectSelectOption<T>>;
  disabled?: boolean;
  prefix?: ReactNode;
  className?: string;
  menuClassName?: string;
  triggerLabel?: ReactNode;
  showSelectedCheck?: boolean;
  showChevron?: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  presentation?: "popover" | "attached-tray";
  popoverPlacement?: "auto" | "bottom";
  popoverMinWidth?: number;
  triggerRef?: RefObject<HTMLButtonElement | null>;
  onChange: (value: T) => void;
}) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const open = controlledOpen ?? uncontrolledOpen;
  const menuId = useId();
  const internalTriggerRef = useRef<HTMLButtonElement>(null);
  const triggerRef = externalTriggerRef ?? internalTriggerRef;
  const attachedMenuRef = useRef<HTMLDivElement>(null);
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const selectedIndex = Math.max(0, options.findIndex((option) => option.value === value));
  const selected = options[selectedIndex];

  const setOpen = (next: boolean | ((current: boolean) => boolean)) => {
    const resolved = typeof next === "function" ? next(open) : next;
    if (controlledOpen === undefined) setUncontrolledOpen(resolved);
    onOpenChange?.(resolved);
  };
  const focusOption = (index: number) => {
    if (!options.length) return;
    const normalized = (index + options.length) % options.length;
    optionRefs.current[normalized]?.focus();
  };
  const close = (restoreFocus = false) => {
    setOpen(false);
    if (restoreFocus) window.requestAnimationFrame(() => triggerRef.current?.focus());
  };
  const choose = (nextValue: T) => {
    onChange(nextValue);
    close(true);
  };

  useEffect(() => {
    if (!open || presentation !== "attached-tray") return;
    const closeOnOutsideClick = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (triggerRef.current?.contains(target) || attachedMenuRef.current?.contains(target)) return;
      close();
    };
    document.addEventListener("click", closeOnOutsideClick);
    return () => document.removeEventListener("click", closeOnOutsideClick);
  }, [open, presentation]);

  const menu = <div id={menuId} className="project-select-menu__list" role="listbox" aria-label={ariaLabel}>
    {options.map((option, index) => {
      const isSelected = option.value === value;
      return <button
        key={option.value}
        ref={(node) => { optionRefs.current[index] = node; }}
        type="button"
        role="option"
        aria-selected={isSelected}
        className={`${isSelected ? "is-selected" : ""}${option.className ? `${isSelected ? " " : ""}${option.className}` : ""}` || undefined}
        onClick={() => choose(option.value)}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown") { event.preventDefault(); focusOption(index + 1); }
          else if (event.key === "ArrowUp") { event.preventDefault(); focusOption(index - 1); }
          else if (event.key === "Home") { event.preventDefault(); focusOption(0); }
          else if (event.key === "End") { event.preventDefault(); focusOption(options.length - 1); }
          else if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); close(true); }
          else if (event.key === "Tab") close();
        }}
      >
        {option.prefix}
        <span>{option.label}</span>
        {isSelected && showSelectedCheck ? <Check size={14} aria-hidden="true" /> : null}
      </button>;
    })}
  </div>;

  return <>
    <button
      ref={triggerRef}
      id={id}
      type="button"
      className={`project-select${className ? ` ${className}` : ""}`}
      aria-label={`${ariaLabel}：${selected?.label ?? ""}`}
      aria-controls={open ? menuId : undefined}
      aria-expanded={open}
      aria-haspopup="listbox"
      disabled={disabled}
      onClick={() => setOpen((current) => !current)}
      onKeyDown={(event) => {
        if (event.key === "ArrowDown" || event.key === "ArrowUp") {
          event.preventDefault();
          setOpen(true);
          window.requestAnimationFrame(() => focusOption(event.key === "ArrowUp" ? options.length - 1 : selectedIndex));
        } else if (event.key === "Escape" && open) {
          event.preventDefault();
          close();
        }
      }}
    >
      {prefix}
      <span>{triggerLabel ?? selected?.label ?? ""}</span>
      {showChevron ? <ChevronDown size={14} aria-hidden="true" /> : null}
    </button>
    {presentation === "attached-tray"
      ? open ? <div ref={attachedMenuRef} className={`project-select-menu project-select-menu--attached${menuClassName ? ` ${menuClassName}` : ""}`}>{menu}</div> : null
      : <AnchoredPopover
          open={open}
          anchorRef={triggerRef}
          onClose={() => close()}
          className={`project-select-menu${menuClassName ? ` ${menuClassName}` : ""}`}
          placement={popoverPlacement}
          offset={6}
          style={{ minWidth: Math.max(popoverMinWidth ?? 0, triggerRef.current?.getBoundingClientRect().width ?? 0) }}
        >
          {menu}
        </AnchoredPopover>}
  </>;
}
