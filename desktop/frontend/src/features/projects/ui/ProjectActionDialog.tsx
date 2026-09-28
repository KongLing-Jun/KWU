import { useEffect, useId, useLayoutEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { Boxes, Check, ChevronDown, Plus, Sparkles, Users, X } from "lucide-react";
import { useT, type DictKey } from "../../../lib/i18n";
import { ProjectModalSurface } from "./ProjectModalSurface";

export type ProjectDialogKind = "create" | "template" | "edit" | "archive" | "restore" | "delete";

export interface ProjectTemplateOption {
  id: string;
  label: string;
  description: string;
  instruction: string;
}

export type ProjectDraftCapabilityKind = "connector" | "expert" | "skill";

export interface ProjectDraftCapabilityItem {
  id: string;
  name: string;
  iconUrl?: string;
}

export type ProjectDraftCapabilities = Record<ProjectDraftCapabilityKind, ProjectDraftCapabilityItem[]>;

interface ProjectActionDialogProps {
  kind: ProjectDialogKind;
  name: string;
  description: string;
  pending: boolean;
  suspended?: boolean;
  error: string;
  nameInvalid?: boolean;
  returnFocus: HTMLElement | null;
  templates?: ProjectTemplateOption[];
  templateId?: string;
  templateInstruction?: string;
  capabilities?: ProjectDraftCapabilities;
  onNameChange(value: string): void;
  onDescriptionChange(value: string): void;
  onTemplateChange?(value: string): void;
  onTemplateInstructionChange?(value: string): void;
  onOpenCapability?(kind: ProjectDraftCapabilityKind, trigger: HTMLButtonElement): void;
  onRemoveCapability?(kind: ProjectDraftCapabilityKind, id: string): void;
  onClose(): void;
  onSubmit(): void;
}

interface ProjectTemplateSelectProps {
  id: string;
  label: string;
  placeholder: string;
  value?: string;
  describedBy: string;
  templates: ProjectTemplateOption[];
  onChange?(value: string): void;
}

function ProjectTemplateSelect({ id, label, placeholder, value, describedBy, templates, onChange }: ProjectTemplateSelectProps) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const options = [{ id: "", label: placeholder }, ...templates];
  const selectedIndex = options.findIndex((template) => template.id === value);
  const selectedTemplate = options[selectedIndex >= 0 ? selectedIndex : 0];
  const menuId = `${id}-menu`;

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  useLayoutEffect(() => {
    if (!open) return;
    const root = rootRef.current;
    const menu = menuRef.current;
    const scrollBoundary = root?.closest<HTMLElement>(".cloud-project-dialog__body");
    if (!root || !menu || !scrollBoundary) return;

    const updateMenuGeometry = () => {
      const rootRect = root.getBoundingClientRect();
      const boundaryRect = scrollBoundary.getBoundingClientRect();
      if (rootRect.height <= 0 || boundaryRect.height <= 0) return;

      const menuGap = 6;
      const boundaryInset = 4;
      const preferredHeight = 252;
      const spaceBelow = Math.max(0, boundaryRect.bottom - rootRect.bottom - menuGap - boundaryInset);
      const spaceAbove = Math.max(0, rootRect.top - boundaryRect.top - menuGap - boundaryInset);
      const placement = spaceBelow >= preferredHeight || spaceBelow >= spaceAbove ? "bottom" : "top";
      const availableHeight = placement === "bottom" ? spaceBelow : spaceAbove;

      menu.dataset.placement = placement;
      menu.style.maxHeight = `${Math.min(preferredHeight, Math.floor(availableHeight))}px`;
    };

    updateMenuGeometry();
    window.addEventListener("resize", updateMenuGeometry);
    scrollBoundary.addEventListener("scroll", updateMenuGeometry, { passive: true });
    const resizeObserver = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(updateMenuGeometry);
    resizeObserver?.observe(scrollBoundary);
    return () => {
      window.removeEventListener("resize", updateMenuGeometry);
      scrollBoundary.removeEventListener("scroll", updateMenuGeometry);
      resizeObserver?.disconnect();
    };
  }, [open]);

  const openMenu = () => {
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : 0);
    setOpen(true);
  };

  const selectTemplate = (index: number) => {
    const template = options[index];
    if (!template) return;
    if (template.id !== value) onChange?.(template.id);
    setActiveIndex(index);
    setOpen(false);
    triggerRef.current?.focus();
  };

  const onTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (options.length === 0) return;
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!open) {
        openMenu();
        return;
      }
      const direction = event.key === "ArrowDown" ? 1 : -1;
      setActiveIndex((current) => (current + direction + options.length) % options.length);
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      if (!open) setOpen(true);
      setActiveIndex(event.key === "Home" ? 0 : options.length - 1);
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (open) selectTemplate(activeIndex);
      else openMenu();
      return;
    }
    if (event.key === "Escape" && open) {
      event.preventDefault();
      event.stopPropagation();
      setOpen(false);
      return;
    }
    if (event.key === "Tab") setOpen(false);
  };

  return (
    <div ref={rootRef} className={`cloud-project-dialog__select-wrap${open ? " is-open" : ""}`}>
      <button
        ref={triggerRef}
        id={id}
        type="button"
        className="cloud-project-dialog__select-trigger"
        role="combobox"
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-controls={menuId}
        aria-describedby={describedBy}
        aria-activedescendant={open ? `${id}-option-${activeIndex}` : undefined}
        data-value={selectedTemplate?.id || ""}
        disabled={templates.length === 0}
        onClick={() => open ? setOpen(false) : openMenu()}
        onKeyDown={onTriggerKeyDown}
      >
        <span>{selectedTemplate?.label || ""}</span>
        <ChevronDown size={15} strokeWidth={1.8} aria-hidden="true" />
      </button>
      {open ? (
        <div ref={menuRef} id={menuId} className="cloud-project-dialog__select-menu" role="listbox" aria-label={label}>
          {options.map((template, index) => (
            <button
              key={template.id}
              id={`${id}-option-${index}`}
              type="button"
              className="cloud-project-dialog__select-option"
              role="option"
              tabIndex={-1}
              aria-selected={template.id === selectedTemplate?.id}
              data-active={index === activeIndex || undefined}
              onMouseEnter={() => setActiveIndex(index)}
              onClick={() => selectTemplate(index)}
            >
              <span>{template.label}</span>
              {template.id === selectedTemplate?.id ? <Check size={14} strokeWidth={2} aria-hidden="true" /> : null}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

export function ProjectActionDialog(props: ProjectActionDialogProps) {
  const t = useT();
  const titleId = useId();
  const errorId = useId();
  const nameCounterId = useId();
  const firstFieldRef = useRef<HTMLInputElement>(null);
  const destructive = props.kind === "archive" || props.kind === "delete";
  const isProjectForm = props.kind === "create" || props.kind === "template" || props.kind === "edit";
  const hasInstructionEditor = props.kind === "create" || props.kind === "template";
  const nameLength = Array.from(props.name).length;
  const [instructionExpanded, setInstructionExpanded] = useState(false);
  const [capabilitiesExpanded, setCapabilitiesExpanded] = useState(false);

  useEffect(() => {
    if (!props.error) return;
    if (isProjectForm && props.nameInvalid) firstFieldRef.current?.focus();
  }, [isProjectForm, props.error, props.nameInvalid]);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!props.pending) props.onSubmit();
  };

  const title = t(`workspace.cloudProjects.dialog.${props.kind}.title` as DictKey);
  const confirm = t(`workspace.cloudProjects.dialog.${props.kind}.confirm` as DictKey);
  const capabilityRows: Array<{ kind: ProjectDraftCapabilityKind; label: string; Icon: typeof Boxes }> = [
    { kind: "connector", label: "连接器", Icon: Boxes },
    { kind: "expert", label: "专家", Icon: Users },
    { kind: "skill", label: "技能", Icon: Sparkles },
  ];
  const capabilityCount = capabilityRows.reduce((count, { kind }) => count + (props.capabilities?.[kind]?.length || 0), 0);
  const instructionLength = Array.from(props.templateInstruction || "").length;

  return (
    <ProjectModalSurface
      backdropClassName="cloud-project-dialog__backdrop"
      className={`cloud-project-dialog${hasInstructionEditor ? " cloud-project-dialog--template" : ""}`}
      titleId={titleId}
      descriptionId={props.error ? errorId : undefined}
      pending={props.pending}
      suspended={props.suspended}
      showClose
      closeLabel={t("common.close")}
      returnFocus={props.returnFocus}
      initialFocusRef={firstFieldRef}
      onClose={props.onClose}
    >
        <header><h3 id={titleId}>{title}</h3></header>
        <form onSubmit={submit}>
          <div className="cloud-project-dialog__body">
            {isProjectForm ? (
              <>
                <section className="cloud-project-dialog__field">
                  <div className="cloud-project-dialog__field-heading">
                    <label htmlFor={`${titleId}-name`}>{t("workspace.cloudProjects.field.name")}</label>
                    <span id={nameCounterId}>{nameLength}/15</span>
                  </div>
                  <input ref={firstFieldRef} id={`${titleId}-name`} value={props.name} maxLength={15} onChange={(event) => props.onNameChange(event.currentTarget.value)} aria-invalid={props.nameInvalid || undefined} aria-describedby={[nameCounterId, props.nameInvalid && props.error ? errorId : ""].filter(Boolean).join(" ")} />
                </section>
                {props.kind === "edit" ? (
                  <section className="cloud-project-dialog__field">
                    <label htmlFor={`${titleId}-description`}>{t("workspace.cloudProjects.field.description")}</label>
                    <textarea id={`${titleId}-description`} value={props.description} onChange={(event) => props.onDescriptionChange(event.currentTarget.value)} />
                  </section>
                ) : null}
                {hasInstructionEditor ? (
                  <>
                    <section className="cloud-project-dialog__field">
                      <label htmlFor={`${titleId}-template`}>{t("workspace.cloudProjects.template.field")}</label>
                      <ProjectTemplateSelect
                        id={`${titleId}-template`}
                        label={t("workspace.cloudProjects.template.field")}
                        placeholder={t("workspace.cloudProjects.template.select")}
                        value={props.templateId}
                        describedBy={`${titleId}-template-warning`}
                        templates={props.templates || []}
                        onChange={props.onTemplateChange}
                      />
                    </section>
                    <details className="cloud-project-dialog__disclosure" open={instructionExpanded} onToggle={(event) => setInstructionExpanded(event.currentTarget.open)}>
                      <summary><span>{t("workspace.cloudProjects.template.instruction")}</span><small>{instructionLength.toLocaleString("zh-CN")} / 10,000</small></summary>
                      <section className="cloud-project-dialog__field cloud-project-dialog__field--instruction">
                        <textarea id={`${titleId}-instruction`} className="cloud-project-dialog__instruction" maxLength={10000} value={props.templateInstruction} placeholder={t("workspace.cloudProjects.template.placeholder")} aria-label={t("workspace.cloudProjects.template.instruction")} aria-describedby={`${titleId}-template-warning`} onChange={(event) => props.onTemplateInstructionChange?.(event.currentTarget.value)} />
                        <p id={`${titleId}-template-warning`} className="cloud-project-dialog__hint">{t("workspace.cloudProjects.template.overwriteWarning")}</p>
                      </section>
                    </details>
                    <details className="cloud-project-dialog__disclosure" open={capabilitiesExpanded} onToggle={(event) => setCapabilitiesExpanded(event.currentTarget.open)}>
                      <summary><span>项目能力（可选）</span><small>已选 {capabilityCount} 项</small></summary>
                      <div className="cloud-project-dialog__capabilities" aria-label="项目能力">
                        {capabilityRows.map(({ kind, label, Icon }) => {
                          const items = props.capabilities?.[kind] || [];
                          return (
                            <section key={kind} className="cloud-project-dialog__capability" aria-labelledby={`${titleId}-${kind}-label`}>
                              <button type="button" className="cloud-project-dialog__capability-open" aria-label={`添加${label}`} disabled={props.pending} onClick={(event) => props.onOpenCapability?.(kind, event.currentTarget)}><span><Plus size={14} aria-hidden="true" />添加</span></button>
                              <header><h4 id={`${titleId}-${kind}-label`}>{label}<span>（可选）</span></h4></header>
                              <div className="cloud-project-dialog__capability-items">
                                {items.length === 0 ? <span className="cloud-project-dialog__capability-empty">尚未添加</span> : items.map((item) => (
                                  <span key={item.id} className="cloud-project-dialog__capability-chip">
                                    {item.iconUrl ? <img src={item.iconUrl} alt="" /> : <Icon size={15} aria-hidden="true" />}
                                    <span>{item.name}</span>
                                    <button type="button" aria-label={`移除${item.name}`} disabled={props.pending} onClick={() => props.onRemoveCapability?.(kind, item.id)}><X size={13} aria-hidden="true" /></button>
                                  </span>
                                ))}
                              </div>
                            </section>
                          );
                        })}
                      </div>
                    </details>
                  </>
                ) : null}
              </>
            ) : null}
            {!isProjectForm ? <p className="cloud-project-dialog__message">{t(`workspace.cloudProjects.dialog.${props.kind}.body` as DictKey)}</p> : null}
            {props.error ? <p id={errorId} className="cloud-project-dialog__error" role="alert">{props.error}</p> : null}
          </div>
          <div className="cloud-project-dialog__actions">
            <button type="button" onClick={props.onClose} disabled={props.pending}>{t("common.cancel")}</button>
            <button type="submit" className={destructive ? "cloud-project-dialog__danger" : "cloud-project-dialog__confirm"} disabled={props.pending} aria-busy={props.pending || undefined}>{props.pending ? t("workspace.cloudProjects.pending") : confirm}</button>
          </div>
        </form>
    </ProjectModalSurface>
  );
}
