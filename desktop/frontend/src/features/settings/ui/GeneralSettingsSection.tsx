import { useEffect, useId, useRef, useState, type MouseEvent as ReactMouseEvent, type PointerEvent } from "react";
import { Check, ChevronDown, ChevronUp, GripVertical, Play } from "lucide-react";
import { app } from "../../../lib/bridge";
import { getDisplayMode, onDisplayModeChange, setDisplayMode as setLocalDisplayMode } from "../../../lib/displayMode";
import { getGenerativePreset, setGenerativePreset, generativeMusic, type GenerativePreset } from "../../../lib/generative-music";
import { useT, type DictKey } from "../../../lib/i18n";
import { getAttentionPreference, getSuccessPreference, playAttentionChime, playSuccessChime, setAttentionPreference, setSuccessPreference, type SoundWavPref } from "../../../lib/sound";
import { DEFAULT_STATUS_BAR_ITEMS, normalizeStatusBarItems, type StatusBarItemId } from "../../../lib/statusBarItems";
import { normalizeToolApprovalMode, type SettingsView } from "../../../lib/types";
import { AnchoredPopover } from "../../../components/AnchoredPopover";
import { SoundSelect } from "../../../components/SoundSelect";
import { Tooltip } from "../../../components/Tooltip";
import { SettingsField, SettingsSection } from "./SettingsPrimitives";

const AUTO_PLAN_MODES = ["off", "on"] as const;
const TOOL_APPROVAL_MODES = ["ask", "auto", "yolo"] as const;

export type AutoPlanMode = (typeof AUTO_PLAN_MODES)[number];

export function normalizeAutoPlan(mode: string | undefined): AutoPlanMode {
  return mode === "ask" || mode === "on" ? "on" : "off";
}

type CloseBehavior = "background" | "quit";

export function normalizeCloseBehavior(mode: string | undefined): CloseBehavior {
  return mode === "quit" ? "quit" : "background";
}

type DisplayMode = "standard" | "compact";

export function normalizeDisplayMode(mode: string | undefined): DisplayMode {
  return mode === "standard" || mode === "compact" ? mode : "standard";
}

type DesktopLayoutStyle = "classic" | "workbench" | "creation";

export function normalizeDesktopLayoutStyle(style: string | undefined): DesktopLayoutStyle {
  void style;
  return "creation";
}

type StatusBarStyle = "icon" | "text";
type StatusBarDropPlacement = "before" | "after";
type StatusBarDragTarget = {
  id: StatusBarItemId;
  placement: StatusBarDropPlacement;
};

export function normalizeStatusBarStyle(style: string | undefined): StatusBarStyle {
  return style === "icon" ? "icon" : "text";
}

function statusBarItemLabel(id: StatusBarItemId, t: ReturnType<typeof useT>): string {
  switch (id) {
    case "model":
      return t("settings.statusBarItem.model");
    case "workspace":
      return t("settings.statusBarItem.workspace");
    case "git_branch":
      return t("settings.statusBarItem.gitBranch");
    case "cache":
      return t("status.cacheLabel");
    case "cache_avg":
      return t("status.cacheAvgLabel");
    case "session_tokens":
      return t("status.sessionTokensLabel");
    case "turn_tokens":
      return t("status.turnTokensLabel");
    case "turn_cost":
      return t("status.turnCostLabel");
    case "session_turns":
      return t("status.sessionTurnsLabel");
    case "context":
      return t("status.ctxLabel");
    case "compact":
      return t("status.compactLabel");
    case "cost":
      return t("status.costLabel");
    case "balance":
      return t("status.balanceLabel");
  }
}

export function closeBehaviorLabel(mode: CloseBehavior, t: ReturnType<typeof useT>): string {
  return mode === "quit" ? t("settings.closeBehavior.quit") : t("settings.closeBehavior.background");
}

export function GeneralSettingsSection({ s, busy, apply, agentRunning }: { s: SettingsView; busy: boolean; apply: (fn: () => Promise<unknown>) => Promise<void>; agentRunning: boolean }) {
  const t = useT();
  const closeBehavior = normalizeCloseBehavior(s.closeBehavior);
  const [displayMode, setDisplayMode] = useState<DisplayMode>(() => normalizeDisplayMode(getDisplayMode()));
  const [statusBarItemsExpanded, setStatusBarItemsExpanded] = useState(false);
  const [draggingStatusBarItem, setDraggingStatusBarItem] = useState<StatusBarItemId | null>(null);
  const [statusBarDragTarget, setStatusBarDragTargetState] = useState<StatusBarDragTarget | null>(null);
  const draggingStatusBarItemRef = useRef<StatusBarItemId | null>(null);
  const statusBarDragTargetRef = useRef<StatusBarDragTarget | null>(null);
  const mouseDragCleanupRef = useRef<(() => void) | null>(null);
  const soundPanelId = useId();
  const statusBarItemsPanelId = useId();
  useEffect(() => onDisplayModeChange((mode) => setDisplayMode(mode)), []);
  useEffect(() => () => mouseDragCleanupRef.current?.(), []);
  const autoPlan = normalizeAutoPlan(s.autoPlan);
  const defaultToolApprovalMode = normalizeToolApprovalMode(s.defaultToolApprovalMode);
  const [genMusicPreset, setGenMusicPreset] = useState<GenerativePreset>(getGenerativePreset());
  const [soundPref, setSoundPref] = useState<SoundWavPref>(getSuccessPreference());
  const [attentionPref, setAttentionPref] = useState<SoundWavPref>(getAttentionPreference());
  const [soundExpanded, setSoundExpanded] = useState(false);
  const statusBarStyle = normalizeStatusBarStyle(s.statusBarStyle);
  const statusBarItems = normalizeStatusBarItems(s.statusBarItems);
  const soundStatus = summarizeSoundStatus(genMusicPreset, soundPref, attentionPref);
  const visibleStatusItems = new Set<StatusBarItemId>(statusBarItems);
  const orderedStatusItems = [
    ...statusBarItems,
    ...DEFAULT_STATUS_BAR_ITEMS.filter((id) => !visibleStatusItems.has(id)),
  ];
  const applyStatusBarItems = (items: StatusBarItemId[]) => {
    const contentScrollTop = document.querySelector<HTMLElement>(".settings-center__content")?.scrollTop ?? 0;
    const navScrollTop = document.querySelector<HTMLElement>(".settings-center__nav")?.scrollTop ?? 0;
    const active = document.activeElement;
    if (active instanceof HTMLElement && active.closest(".status-bar-items-editor")) active.blur();
    void apply(() => app.SetStatusBarItems(items)).finally(() => {
      window.scrollTo(0, 0);
      requestAnimationFrame(() => {
        window.scrollTo(0, 0);
        const content = document.querySelector<HTMLElement>(".settings-center__content");
        const nav = document.querySelector<HTMLElement>(".settings-center__nav");
        if (content) content.scrollTop = Math.min(contentScrollTop, Math.max(0, content.scrollHeight - content.clientHeight));
        if (nav) nav.scrollTop = navScrollTop;
      });
    });
  };
  const toggleStatusBarItem = (id: StatusBarItemId) => {
    if (visibleStatusItems.has(id)) {
      if (statusBarItems.length <= 1) return;
      applyStatusBarItems(statusBarItems.filter((item) => item !== id));
      return;
    }
    applyStatusBarItems([...statusBarItems, id]);
  };
  const moveStatusBarItem = (id: StatusBarItemId, direction: -1 | 1) => {
    const idx = statusBarItems.indexOf(id);
    const nextIdx = idx + direction;
    if (idx < 0 || nextIdx < 0 || nextIdx >= statusBarItems.length) return;
    const next = [...statusBarItems];
    [next[idx], next[nextIdx]] = [next[nextIdx], next[idx]];
    applyStatusBarItems(next);
  };
  const reorderStatusBarItem = (fromId: StatusBarItemId, toId: StatusBarItemId, placement: StatusBarDropPlacement) => {
    const fromIdx = statusBarItems.indexOf(fromId);
    const toIdx = statusBarItems.indexOf(toId);
    if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) return;
    const next = statusBarItems.filter((item) => item !== fromId);
    const insertAt = next.indexOf(toId);
    if (insertAt < 0) return;
    next.splice(placement === "after" ? insertAt + 1 : insertAt, 0, fromId);
    if (next.every((item, index) => item === statusBarItems[index])) return;
    applyStatusBarItems(next);
  };
  const statusBarItemFromPoint = (x: number, y: number): StatusBarDragTarget | null => {
    const row = document.elementFromPoint(x, y)?.closest<HTMLElement>("[data-statusbar-setting-item]");
    const id = row?.dataset.statusbarSettingItem as StatusBarItemId | undefined;
    if (!row || !id || !statusBarItems.includes(id)) return null;
    const rect = row.getBoundingClientRect();
    return { id, placement: y < rect.top + rect.height / 2 ? "before" : "after" };
  };
  const setStatusBarDragTarget = (target: StatusBarDragTarget | null) => {
    const current = statusBarDragTargetRef.current;
    if (current?.id === target?.id && current?.placement === target?.placement) return;
    statusBarDragTargetRef.current = target;
    setStatusBarDragTargetState(target);
  };
  const beginStatusBarDrag = (id: StatusBarItemId, visible: boolean): boolean => {
    if (busy || !visible) return false;
    mouseDragCleanupRef.current?.();
    mouseDragCleanupRef.current = null;
    draggingStatusBarItemRef.current = id;
    statusBarDragTargetRef.current = null;
    setDraggingStatusBarItem(id);
    setStatusBarDragTargetState(null);
    return true;
  };
  const updateStatusBarDrag = (clientX: number, clientY: number) => {
    const draggingId = draggingStatusBarItemRef.current;
    if (!draggingId) return;
    const target = statusBarItemFromPoint(clientX, clientY);
    setStatusBarDragTarget(target && target.id !== draggingId ? target : null);
  };
  const finishStatusBarDrag = (clientX?: number, clientY?: number) => {
    const draggingId = draggingStatusBarItemRef.current;
    let target = statusBarDragTargetRef.current;
    if (draggingId && clientX !== undefined && clientY !== undefined) {
      const pointerTarget = statusBarItemFromPoint(clientX, clientY);
      if (pointerTarget && pointerTarget.id !== draggingId) target = pointerTarget;
    }
    if (draggingId && target) reorderStatusBarItem(draggingId, target.id, target.placement);
    draggingStatusBarItemRef.current = null;
    statusBarDragTargetRef.current = null;
    setDraggingStatusBarItem(null);
    setStatusBarDragTargetState(null);
  };
  const cancelStatusBarDrag = () => {
    mouseDragCleanupRef.current?.();
    mouseDragCleanupRef.current = null;
    draggingStatusBarItemRef.current = null;
    statusBarDragTargetRef.current = null;
    setDraggingStatusBarItem(null);
    setStatusBarDragTargetState(null);
  };
  const startStatusBarPointerDrag = (event: PointerEvent<HTMLElement>, id: StatusBarItemId, visible: boolean) => {
    if (event.button !== 0 || !beginStatusBarDrag(id, visible)) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
  };
  const moveStatusBarPointerDrag = (event: PointerEvent<HTMLElement>) => {
    if (!draggingStatusBarItemRef.current) return;
    event.preventDefault();
    updateStatusBarDrag(event.clientX, event.clientY);
  };
  const endStatusBarPointerDrag = (event: PointerEvent<HTMLElement>) => {
    if (!draggingStatusBarItemRef.current) return;
    event.preventDefault();
    try {
      event.currentTarget.releasePointerCapture(event.pointerId);
    } catch {
      // Pointer capture may already be released by the browser.
    }
    finishStatusBarDrag(event.clientX, event.clientY);
  };
  const cancelStatusBarPointerDrag = (event: PointerEvent<HTMLElement>) => {
    event.preventDefault();
    cancelStatusBarDrag();
  };
  const startStatusBarMouseDrag = (event: ReactMouseEvent<HTMLElement>, id: StatusBarItemId, visible: boolean) => {
    if (event.button !== 0 || !beginStatusBarDrag(id, visible)) return;
    event.preventDefault();
    const handleMove = (moveEvent: MouseEvent) => {
      moveEvent.preventDefault();
      updateStatusBarDrag(moveEvent.clientX, moveEvent.clientY);
    };
    const cleanup = () => {
      window.removeEventListener("mousemove", handleMove);
      window.removeEventListener("mouseup", handleUp);
    };
    const handleUp = (upEvent: MouseEvent) => {
      upEvent.preventDefault();
      cleanup();
      mouseDragCleanupRef.current = null;
      finishStatusBarDrag(upEvent.clientX, upEvent.clientY);
    };
    window.addEventListener("mousemove", handleMove);
    window.addEventListener("mouseup", handleUp);
    mouseDragCleanupRef.current = cleanup;
  };
  return (
    <SettingsSection>
      <SettingsField label={t("settings.language")}>
        <div className="set-seg">
          <span className="set-seg__btn set-seg__btn--on">中文</span>
        </div>
      </SettingsField>
      <SettingsField label={t("settings.closeBehavior")}>
        <div className="set-seg">
          {(["background", "quit"] as const).map((mode) => (
            <button
              key={mode}
              className={`set-seg__btn${closeBehavior === mode ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => void apply(() => app.SetCloseBehavior(mode))}
            >
              {closeBehaviorLabel(mode, t)}
            </button>
          ))}
        </div>
      </SettingsField>
      <SettingsField label={t("settings.displayMode")}>
        <div className="set-seg">
          {(["standard", "compact"] as const).map((mode) => (
            <button
              key={mode}
              className={`set-seg__btn${displayMode === mode ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => {
                setLocalDisplayMode(mode);
                void apply(() => app.SetDisplayMode(mode));
              }}
            >
              {t(`settings.displayMode.${mode}`)}
            </button>
          ))}
        </div>
      </SettingsField>
      <SettingsField label={t("settings.defaultToolApprovalMode")} hint={t("settings.defaultToolApprovalModeHint")}>
        <div className="set-seg">
          {TOOL_APPROVAL_MODES.map((mode) => (
            <button
              key={mode}
              className={`set-seg__btn${defaultToolApprovalMode === mode ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => void apply(() => app.SetDefaultToolApprovalMode(mode))}
            >
              {t(`settings.defaultToolApprovalMode.${mode}`)}
            </button>
          ))}
        </div>
      </SettingsField>
      <SettingsField label={t("settings.autoPlan")}>
        <div className="set-seg">
          {AUTO_PLAN_MODES.map((mode) => (
            <button
              key={mode}
              className={`set-seg__btn${autoPlan === mode ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => void apply(() => app.SetAutoPlan(mode))}
            >
              {t(`settings.autoPlan.${mode}`)}
            </button>
          ))}
        </div>
      </SettingsField>
      <SettingsField label={t("settings.sound")} hint={t("settings.soundHint")} stacked>
        <div className={`settings-sound-editor${soundExpanded ? " settings-sound-editor--expanded" : ""}`}>
          <div className="settings-sound-editor__summary">
            <span className={`settings-sound-editor__status settings-sound-editor__status--${soundStatus}`}>
              {t(`settings.soundStatus.${soundStatus}`)}
            </span>
            <Tooltip label={t(soundExpanded ? "settings.soundCollapse" : "settings.soundExpand")}>
              <button
                type="button"
                className="settings-sound-editor__toggle"
                aria-expanded={soundExpanded}
                aria-controls={soundPanelId}
                aria-label={t(soundExpanded ? "settings.soundCollapse" : "settings.soundExpand")}
                onClick={() => setSoundExpanded((open) => !open)}
              >
                {soundExpanded ? <ChevronUp size={15} aria-hidden="true" /> : <ChevronDown size={15} aria-hidden="true" />}
              </button>
            </Tooltip>
          </div>
          {soundExpanded && (
            <div className="settings-sound-editor__list" id={soundPanelId}>
              <div className="settings-sound-row">
                <span className="settings-sound-row__label">{t("settings.generativeMusic")}</span>
                <GenMusicSelect
                  value={genMusicPreset}
                  onChange={(next) => {
                    setGenMusicPreset(next);
                    setGenerativePreset(next);
                    if (next === "off") {
                      generativeMusic.stop();
                    } else {
                      if (generativeMusic.isRunning) {
                        generativeMusic.setPreset(next);
                      } else if (agentRunning) {
                        generativeMusic.start(next);
                      }
                      generativeMusic.playPreview(next);
                    }
                  }}
                  onPreview={() => { if (genMusicPreset !== "off") generativeMusic.playPreview(genMusicPreset); }}
                  previewDisabled={genMusicPreset === "off"}
                />
              </div>
              <div className="settings-sound-row">
                <span className="settings-sound-row__label">{t("settings.notificationSoundSuccess")}</span>
                <SoundSelect
                  value={soundPref}
                  onChange={(next) => {
                    setSoundPref(next);
                    setSuccessPreference(next);
                    playSuccessChime();
                  }}
                  onPreview={playSuccessChime}
                  previewDisabled={soundPref === "off"}
                />
              </div>
              <div className="settings-sound-row">
                <span className="settings-sound-row__label">{t("settings.notificationSoundAttention")}</span>
                <SoundSelect
                  value={attentionPref}
                  onChange={(next) => {
                    setAttentionPref(next);
                    setAttentionPreference(next);
                    playAttentionChime();
                  }}
                  onPreview={playAttentionChime}
                  previewDisabled={attentionPref === "off"}
                />
              </div>
            </div>
          )}
        </div>
      </SettingsField>
      <SettingsField label={t("settings.statusBarStyle")}>
        <div className="set-seg">
          {(["icon", "text"] as const).map((style) => (
            <button
              key={style}
              className={`set-seg__btn${statusBarStyle === style ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => void apply(() => app.SetStatusBarStyle(style))}
            >
              {t(`settings.statusBarStyle.${style}`)}
            </button>
          ))}
        </div>
      </SettingsField>
      <SettingsField label={t("settings.statusBarItems")} hint={t("settings.statusBarItemsHint")} stacked>
        <div className={`status-bar-items-editor${statusBarItemsExpanded ? " status-bar-items-editor--expanded" : ""}`}>
          <div className="status-bar-items-editor__summary">
            <span className="status-bar-items-editor__summary-text">
              {t("settings.statusBarItemsSummary", { visible: statusBarItems.length, total: DEFAULT_STATUS_BAR_ITEMS.length })}
            </span>
            <Tooltip label={t(statusBarItemsExpanded ? "settings.statusBarItemsCollapse" : "settings.statusBarItemsExpand")}>
              <button
                type="button"
                className="status-bar-items-editor__toggle"
                aria-expanded={statusBarItemsExpanded}
                aria-controls={statusBarItemsPanelId}
                aria-label={t(statusBarItemsExpanded ? "settings.statusBarItemsCollapse" : "settings.statusBarItemsExpand")}
                onClick={() => setStatusBarItemsExpanded((open) => !open)}
              >
                {statusBarItemsExpanded ? <ChevronUp size={15} aria-hidden="true" /> : <ChevronDown size={15} aria-hidden="true" />}
              </button>
            </Tooltip>
          </div>
          {statusBarItemsExpanded && (
            <div className="status-bar-items-editor__list" id={statusBarItemsPanelId}>
              {orderedStatusItems.map((id) => {
                const label = statusBarItemLabel(id, t);
                const visible = visibleStatusItems.has(id);
                const visibleIndex = statusBarItems.indexOf(id);
                const disableHide = visible && statusBarItems.length <= 1;
                const dragLabel = t("settings.statusBarItem.drag", { label });
                const moveUpLabel = t("settings.statusBarItem.moveUp", { label });
                const moveDownLabel = t("settings.statusBarItem.moveDown", { label });
                const dropPlacement = statusBarDragTarget?.id === id ? statusBarDragTarget.placement : null;
                return (
                  <div
                    className={[
                      "status-bar-item-row",
                      visible ? "" : "status-bar-item-row--hidden",
                      draggingStatusBarItem === id ? "status-bar-item-row--dragging" : "",
                      dropPlacement ? "status-bar-item-row--drag-over" : "",
                      dropPlacement === "before" ? "status-bar-item-row--drop-before" : "",
                      dropPlacement === "after" ? "status-bar-item-row--drop-after" : "",
                    ].filter(Boolean).join(" ")}
                    data-statusbar-setting-item={id}
                    key={id}
                  >
                    <Tooltip label={dragLabel}>
                      <button
                        type="button"
                        className="status-bar-item-row__drag"
                        disabled={!visible || busy}
                        aria-label={dragLabel}
                        title={dragLabel}
                        onPointerDown={(event) => startStatusBarPointerDrag(event, id, visible)}
                        onPointerMove={moveStatusBarPointerDrag}
                        onPointerUp={endStatusBarPointerDrag}
                        onPointerCancel={cancelStatusBarPointerDrag}
                        onMouseDown={(event) => startStatusBarMouseDrag(event, id, visible)}
                      >
                        <GripVertical size={14} aria-hidden="true" />
                      </button>
                    </Tooltip>
                    <label className="status-bar-item-row__toggle">
                      <input
                        type="checkbox"
                        checked={visible}
                        disabled={busy || disableHide}
                        onChange={() => toggleStatusBarItem(id)}
                      />
                      <span className="status-bar-item-row__check" aria-hidden="true">
                        {visible && <Check size={12} />}
                      </span>
                      <span className="status-bar-item-row__label">{label}</span>
                    </label>
                    <div className="status-bar-item-row__actions">
                      <Tooltip label={moveUpLabel}>
                        <button
                          type="button"
                          className="status-bar-item-row__order"
                          disabled={busy || !visible || visibleIndex <= 0}
                          onClick={() => moveStatusBarItem(id, -1)}
                          aria-label={moveUpLabel}
                        >
                          <ChevronUp size={14} aria-hidden="true" />
                        </button>
                      </Tooltip>
                      <Tooltip label={moveDownLabel}>
                        <button
                          type="button"
                          className="status-bar-item-row__order"
                          disabled={busy || !visible || visibleIndex < 0 || visibleIndex >= statusBarItems.length - 1}
                          onClick={() => moveStatusBarItem(id, 1)}
                          aria-label={moveDownLabel}
                        >
                          <ChevronDown size={14} aria-hidden="true" />
                        </button>
                      </Tooltip>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </SettingsField>
    </SettingsSection>
  );
}

const GENRE_OPTIONS: { value: GenerativePreset; labelKey: DictKey }[] = [
  { value: "off", labelKey: "settings.generativeMusic.off" },
  { value: "ethereal", labelKey: "settings.generativeMusic.presets.ethereal" },
  { value: "classic", labelKey: "settings.generativeMusic.presets.classic" },
  { value: "digital", labelKey: "settings.generativeMusic.presets.digital" },
  { value: "retro", labelKey: "settings.generativeMusic.presets.retro" },
];

function summarizeSoundStatus(
  music: GenerativePreset,
  success: SoundWavPref,
  attention: SoundWavPref,
): "allOff" | "enabled" | "custom" {
  const enabledCount = [music !== "off", success !== "off", attention !== "off"].filter(Boolean).length;
  if (enabledCount === 0) return "allOff";
  if (enabledCount === 1) return "enabled";
  return "custom";
}

function GenMusicSelect({
  value,
  onChange,
  onPreview,
  previewDisabled,
}: {
  value: GenerativePreset;
  onChange: (v: GenerativePreset) => void;
  onPreview: () => void;
  previewDisabled?: boolean;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const selected = GENRE_OPTIONS.find((o) => o.value === value) ?? GENRE_OPTIONS[0];

  return (
    <div className="sound-select">
      <button
        ref={triggerRef}
        className="sound-select__trigger"
        type="button"
        onClick={() => setOpen((v) => !v)}
      >
        <span className="sound-select__label">{t(selected.labelKey)}</span>
        <ChevronDown
          size={16}
          className={`sound-select__chev${open ? " sound-select__chev--open" : ""}`}
        />
      </button>
      {!previewDisabled && (
        <button className="chip chip--icon" type="button" title={t("settings.generativeMusicPreview")} aria-label={t("settings.generativeMusicPreview")} onClick={onPreview}>
          <Play size={13} aria-hidden="true" />
        </button>
      )}
      <AnchoredPopover
        open={open}
        anchorRef={triggerRef}
        onClose={() => setOpen(false)}
        className="sound-select__menu"
        placement="bottom"
      >
        <div className="sound-select__list" role="listbox">
          {GENRE_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              className={`sound-select__option${opt.value === value ? " sound-select__option--selected" : ""}`}
              role="option"
              aria-selected={opt.value === value}
              type="button"
              onClick={() => {
                onChange(opt.value);
                setOpen(false);
              }}
            >
              <span>{t(opt.labelKey)}</span>
              {opt.value === value && <Check size={14} className="sound-select__check" />}
            </button>
          ))}
        </div>
      </AnchoredPopover>
    </div>
  );
}
