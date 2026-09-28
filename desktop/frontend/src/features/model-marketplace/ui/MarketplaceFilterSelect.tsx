import { useEffect, useId, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Check, ChevronDown } from "lucide-react";
import { AnchoredPopover } from "../../../components/AnchoredPopover";

const PORTAL_THEME_TOKENS = ["--accent", "--surface", "--border", "--fg", "--fg-faint"] as const;

export function MarketplaceFilterSelect({
  kind,
  label,
  allLabel,
  value,
  options,
  onChange,
}: {
  kind: "vendor" | "plan";
  label: string;
  allLabel: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [triggerWidth, setTriggerWidth] = useState<number>();
  const [popoverTheme, setPopoverTheme] = useState<Record<string, string>>({});
  const menuId = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const allOptions = [{ value: "", label: allLabel }, ...options];
  const selectedLabel = allOptions.find((option) => option.value === value)?.label ?? allLabel;
  const selectedIndex = Math.max(0, allOptions.findIndex((option) => option.value === value));

  useEffect(() => {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const measure = () => {
      const width = trigger.getBoundingClientRect().width;
      if (width > 0) setTriggerWidth(width);
    };
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(trigger);
    return () => observer.disconnect();
  }, []);

  const focusOption = (index: number) => {
    const normalized = (index + allOptions.length) % allOptions.length;
    optionRefs.current[normalized]?.focus();
  };
  const restoreTriggerFocus = () => {
    window.requestAnimationFrame(() => triggerRef.current?.focus());
  };
  const choose = (nextValue: string) => {
    onChange(nextValue);
    setOpen(false);
    restoreTriggerFocus();
  };
  const openMenu = () => {
    const trigger = triggerRef.current;
    if (trigger) {
      const computed = window.getComputedStyle(trigger);
      const theme: Record<string, string> = {};
      for (const token of PORTAL_THEME_TOKENS) {
        const value = computed.getPropertyValue(token).trim();
        if (value) theme[token] = value;
      }
      setPopoverTheme(theme);
    }
    setOpen(true);
  };

  return (
    <>
      <button
        ref={triggerRef}
        className={`model-marketplace-filter${open ? " model-marketplace-filter--open" : ""}`}
        type="button"
        data-model-marketplace-vendor-filter={kind === "vendor" ? true : undefined}
        data-model-marketplace-plan-filter={kind === "plan" ? true : undefined}
        aria-controls={open ? menuId : undefined}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`${label}筛选：${selectedLabel}`}
        onClick={() => {
          if (open) setOpen(false);
          else openMenu();
        }}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault();
            openMenu();
            const nextIndex = event.key === "ArrowUp" ? allOptions.length - 1 : selectedIndex;
            window.requestAnimationFrame(() => focusOption(nextIndex));
          } else if (event.key === "Escape" && open) {
            event.preventDefault();
            setOpen(false);
          }
        }}
      >
        <span className="model-marketplace-filter__label">{label}</span>
        <span className="model-marketplace-filter__value">{selectedLabel}</span>
        <ChevronDown className="model-marketplace-filter__chevron" size={14} aria-hidden="true" />
      </button>
      <AnchoredPopover
        open={open}
        anchorRef={triggerRef}
        onClose={() => setOpen(false)}
        className="model-marketplace-filter-menu"
        placement="bottom"
        offset={6}
        style={{ width: triggerWidth, ...popoverTheme } as CSSProperties}
      >
        <div id={menuId} className="model-marketplace-filter-menu__list" role="listbox" aria-label={`${label}筛选`}>
          {allOptions.map((option, index) => {
            const selected = option.value === value;
            return (
              <button
                key={option.value || "all"}
                ref={(node) => {
                  optionRefs.current[index] = node;
                }}
                className={`model-marketplace-filter-option${selected ? " model-marketplace-filter-option--selected" : ""}`}
                type="button"
                role="option"
                aria-selected={selected}
                data-model-marketplace-filter-option={option.value || "all"}
                onClick={() => choose(option.value)}
                onKeyDown={(event) => {
                  if (event.key === "ArrowDown") {
                    event.preventDefault();
                    focusOption(index + 1);
                  } else if (event.key === "ArrowUp") {
                    event.preventDefault();
                    focusOption(index - 1);
                  } else if (event.key === "Home") {
                    event.preventDefault();
                    focusOption(0);
                  } else if (event.key === "End") {
                    event.preventDefault();
                    focusOption(allOptions.length - 1);
                  } else if (event.key === "Escape") {
                    event.preventDefault();
                    event.stopPropagation();
                    setOpen(false);
                    restoreTriggerFocus();
                  } else if (event.key === "Tab") {
                    setOpen(false);
                  }
                }}
              >
                <span>{option.label}</span>
                {selected ? <Check size={14} aria-hidden="true" /> : null}
              </button>
            );
          })}
        </div>
      </AnchoredPopover>
    </>
  );
}
