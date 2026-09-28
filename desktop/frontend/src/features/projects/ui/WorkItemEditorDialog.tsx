import { useId, useMemo, useRef, useState, type Dispatch, type MouseEvent as ReactMouseEvent, type RefObject, type SetStateAction } from "react";
import { CalendarDays, ChevronDown, Paperclip, Plus, Search, Tags, UserRound } from "lucide-react";
import { AnchoredPopover } from "../../../components/AnchoredPopover";
import { InlineConfirmButton } from "../../../components/InlineConfirmButton";
import { ProjectSelect } from "../../../components/ProjectSelect";
import { Tooltip } from "../../../components/Tooltip";
import type { ProjectGateway } from "../data/projectGateway";
import type { CloudProjectLabel, CloudProjectMember, CloudProjectWorkItem, CloudProjectWorkItemStatus } from "../model/projectTypes";
import { ProjectAssetSelection } from "./ProjectAssetSelection";
import { ProjectModalSurface } from "./ProjectModalSurface";
import { WORK_ITEM_STATUS_ICONS } from "./workItemStatusVisual";

export const WORK_ITEM_PRIORITIES = ["none", "low", "medium", "high", "urgent"] as const;
export type WorkItemPriority = (typeof WORK_ITEM_PRIORITIES)[number];

export interface WorkItemEditorState {
  kind: "create" | "edit";
  workItemId: string;
  title: string;
  description: string;
  status: CloudProjectWorkItemStatus;
  assigneeUserId: string;
  priority: WorkItemPriority;
  startAt: string;
  dueAt: string;
  labelIds: string[];
  assetIds: string[];
  persisted?: CloudProjectWorkItem;
  persistedDraft?: Pick<WorkItemEditorState, "title" | "description" | "assigneeUserId" | "priority" | "startAt" | "dueAt">;
}

const STATUS_OPTIONS: Array<{ value: CloudProjectWorkItemStatus; label: string }> = [
  { value: "todo", label: "待开始" },
  { value: "in_progress", label: "进行中" },
  { value: "paused", label: "已暂停" },
  { value: "done", label: "已完成" },
];

export const WORK_ITEM_PRIORITY_OPTIONS: Array<{ value: WorkItemPriority; label: string }> = [
  { value: "none", label: "无" },
  { value: "urgent", label: "紧急" },
  { value: "high", label: "高" },
  { value: "medium", label: "中" },
  { value: "low", label: "低" },
];

export function workItemPriorityLabel(priority: WorkItemPriority | null | undefined, noneLabel = "无") {
  if (!priority || priority === "none") return noneLabel;
  return WORK_ITEM_PRIORITY_OPTIONS.find((option) => option.value === priority)?.label ?? noneLabel;
}

const priorityOptionClass = (priority: WorkItemPriority) => `work-item-board__priority-option work-item-board__priority-option--${priority}`;

function showDatePicker(event: ReactMouseEvent<HTMLInputElement>) {
  const input = event.currentTarget;
  if (typeof input.showPicker !== "function") return;
  try {
    input.showPicker();
  } catch {
    input.focus();
  }
}

export function WorkItemEditorDialog({
  editor,
  setEditor,
  titleRef,
  members,
  labels,
  statusOptions,
  pending,
  readOnly,
  dateInvalid,
  dateErrorId,
  projectId,
  projectScopeKey,
  gateway,
  onCreateLabel,
  onSubmit,
  onDelete,
  onClose,
}: {
  editor: WorkItemEditorState;
  setEditor: Dispatch<SetStateAction<WorkItemEditorState | null>>;
  titleRef: RefObject<HTMLInputElement | null>;
  members: CloudProjectMember[];
  labels: CloudProjectLabel[];
  statusOptions: CloudProjectWorkItemStatus[];
  pending: boolean;
  readOnly: boolean;
  dateInvalid: boolean;
  dateErrorId: string;
  projectId: string;
  projectScopeKey: string;
  gateway: ProjectGateway;
  onCreateLabel?: (name: string, color: string) => Promise<string | null>;
  onSubmit: () => void;
  onDelete: () => void;
  onClose: () => void;
}) {
  const [openMenu, setOpenMenu] = useState<"assignee" | "labels" | null>(null);
  const [assigneeQuery, setAssigneeQuery] = useState("");
  const [labelQuery, setLabelQuery] = useState("");
  const [labelCreateOpen, setLabelCreateOpen] = useState(false);
  const [labelDraftName, setLabelDraftName] = useState("");
  const [labelDraftColor, setLabelDraftColor] = useState("#3366FF");
  const [creatingLabel, setCreatingLabel] = useState(false);
  const [assetsOpen, setAssetsOpen] = useState(false);
  const assigneeAnchorRef = useRef<HTMLButtonElement>(null);
  const labelAnchorRef = useRef<HTMLButtonElement>(null);
  const labelNameRef = useRef<HTMLInputElement>(null);
  const labelCreateTitleId = useId();
  const labelCreateFormId = useId();
  const StatusIcon = WORK_ITEM_STATUS_ICONS[editor.status];
  const statusSelectOptions = STATUS_OPTIONS.filter((option) => statusOptions.includes(option.value)).map((option) => {
    const OptionIcon = WORK_ITEM_STATUS_ICONS[option.value];
    return {
      ...option,
      prefix: <OptionIcon className={`work-item-board__property-status work-item-board__property-status--${option.value}`} size={15} aria-hidden="true" />,
      className: `work-item-board__status-option work-item-board__status-option--${option.value}`,
    };
  });
  const prioritySelectOptions = WORK_ITEM_PRIORITY_OPTIONS.map((option) => ({ ...option, className: priorityOptionClass(option.value) }));
  const currentMember = members.find((member) => member.userId === editor.assigneeUserId);
  const filteredMembers = useMemo(() => {
    const query = assigneeQuery.trim().toLocaleLowerCase();
    return query ? members.filter((member) => member.displayName.toLocaleLowerCase().includes(query)) : members;
  }, [assigneeQuery, members]);
  const filteredLabels = useMemo(() => {
    const query = labelQuery.trim().toLocaleLowerCase();
    return query ? labels.filter((label) => label.name.toLocaleLowerCase().includes(query)) : labels;
  }, [labelQuery, labels]);
  const normalizedLabelDraftName = labelDraftName.trim();
  const labelNameExists = Boolean(normalizedLabelDraftName) && labels.some((label) => label.name.localeCompare(normalizedLabelDraftName, undefined, { sensitivity: "accent" }) === 0);

  const patchEditor = (patch: Partial<WorkItemEditorState>) => setEditor((current) => current ? { ...current, ...patch } : current);
  const closeMenu = () => setOpenMenu(null);
  const openLabelCreateDialog = () => {
    setOpenMenu(null);
    setLabelDraftName("");
    setLabelDraftColor("#3366FF");
    setLabelCreateOpen(true);
  };
  const createLabel = async () => {
    if (!onCreateLabel || !normalizedLabelDraftName || labelNameExists || creatingLabel || pending || readOnly) return;
    setCreatingLabel(true);
    try {
      const labelId = await onCreateLabel(normalizedLabelDraftName, labelDraftColor);
      if (!labelId) return;
      setEditor((current) => current ? { ...current, labelIds: current.labelIds.includes(labelId) ? current.labelIds : [...current.labelIds, labelId] } : current);
      setLabelQuery("");
      setLabelCreateOpen(false);
    } finally {
      setCreatingLabel(false);
    }
  };

  return <><ProjectModalSurface backdropClassName="work-item-board__dialog-backdrop" className="work-item-board__dialog work-item-board__dialog--editor" titleId="work-item-editor-title" pending={pending} suspended={labelCreateOpen} initialFocusRef={titleRef} onClose={onClose}>
      <header>
        <h3 id="work-item-editor-title">{editor.kind === "create" ? "新建待办" : "编辑待办"}</h3>
      </header>

      <div className="work-item-board__dialog-body work-item-board__editor-body">
        <input
          ref={titleRef}
          className="work-item-board__editor-title"
          aria-label="标题"
          placeholder="请输入标题"
          maxLength={240}
          value={editor.title}
          onChange={(event) => patchEditor({ title: event.currentTarget.value })}
        />
        <textarea
          className="work-item-board__editor-description"
          aria-label="说明"
          placeholder="添加描述（可选）"
          value={editor.description}
          onChange={(event) => patchEditor({ description: event.currentTarget.value })}
        />
        {assetsOpen ? <div className="work-item-board__editor-assets"><ProjectAssetSelection projectId={projectId} projectScopeKey={projectScopeKey} gateway={gateway} selectedIds={editor.assetIds} onChange={(assetIds) => patchEditor({ assetIds })} /></div> : null}
        {dateInvalid ? <p id={dateErrorId} className="cloud-projects__inline-error">截止日期不能早于开始日期。</p> : null}
      </div>

      <div className="work-item-board__editor-properties" role="group" aria-label="待办属性">
        <ProjectSelect
          ariaLabel="状态"
          value={editor.status}
          options={statusSelectOptions}
          prefix={<StatusIcon className={`work-item-board__property-status work-item-board__property-status--${editor.status}`} size={15} aria-hidden="true" />}
          popoverPlacement="auto"
          popoverMinWidth={124}
          showSelectedCheck={false}
          showChevron={false}
          disabled={pending || readOnly}
          onChange={(status) => patchEditor({ status })}
        />

        <span className="work-item-board__property-popover">
          <button ref={assigneeAnchorRef} type="button" className="project-select" aria-label={`处理人：${currentMember?.displayName || "未指派"}`} aria-expanded={openMenu === "assignee"} disabled={pending || readOnly} onClick={() => setOpenMenu((menu) => menu === "assignee" ? null : "assignee")}>
            <UserRound size={15} aria-hidden="true" /><span>{currentMember?.displayName || "处理人"}</span><ChevronDown size={13} aria-hidden="true" />
          </button>
          <AnchoredPopover open={openMenu === "assignee"} anchorRef={assigneeAnchorRef} onClose={closeMenu} className="work-item-board__property-menu" placement="auto" offset={6}>
            <div role="listbox" aria-label="处理人" className="work-item-board__property-menu-list">
              <label className="work-item-board__property-search"><Search size={14} aria-hidden="true" /><input value={assigneeQuery} onChange={(event) => setAssigneeQuery(event.currentTarget.value)} placeholder="搜索处理人" aria-label="搜索处理人" /></label>
              <button type="button" role="option" aria-selected={!editor.assigneeUserId} onClick={() => { patchEditor({ assigneeUserId: "" }); closeMenu(); }}><span className="work-item-board__member-checkbox" aria-hidden="true" />未指派</button>
              {filteredMembers.map((member) => <button key={member.userId} type="button" role="option" aria-selected={editor.assigneeUserId === member.userId} onClick={() => { patchEditor({ assigneeUserId: member.userId }); closeMenu(); }}><span className="work-item-board__member-checkbox" aria-hidden="true" />{member.displayName}</button>)}
            </div>
          </AnchoredPopover>
        </span>

        <label className="work-item-board__date-chip"><CalendarDays size={14} aria-hidden="true" /><span>{editor.startAt || "开始日期"}</span><input type="date" value={editor.startAt} aria-label="开始日期" onClick={showDatePicker} onChange={(event) => patchEditor({ startAt: event.currentTarget.value })} disabled={pending || readOnly} /></label>
        <label className="work-item-board__date-chip"><CalendarDays size={14} aria-hidden="true" /><span>{editor.dueAt || "截止日期"}</span><input type="date" value={editor.dueAt} min={editor.startAt || undefined} aria-label="截止日期" aria-invalid={dateInvalid || undefined} aria-describedby={dateInvalid ? dateErrorId : undefined} onClick={showDatePicker} onChange={(event) => patchEditor({ dueAt: event.currentTarget.value })} disabled={pending || readOnly} /></label>

        <ProjectSelect ariaLabel="优先级" value={editor.priority} options={prioritySelectOptions} triggerLabel={editor.priority === "none" ? "优先级" : undefined} showSelectedCheck={false} showChevron={false} popoverMinWidth={138} className={`work-item-board__priority-select work-item-board__priority-select--${editor.priority}`} disabled={pending || readOnly} onChange={(priority) => patchEditor({ priority })} />

        <span className="work-item-board__property-popover">
          <button ref={labelAnchorRef} type="button" className="project-select" aria-label={`标签：已选择 ${editor.labelIds.length} 个`} aria-expanded={openMenu === "labels"} disabled={pending || readOnly} onClick={() => setOpenMenu((menu) => menu === "labels" ? null : "labels")}><Tags size={14} aria-hidden="true" /><span>{editor.labelIds.length ? `标签 ${editor.labelIds.length}` : "标签"}</span></button>
          <AnchoredPopover open={openMenu === "labels"} anchorRef={labelAnchorRef} onClose={closeMenu} className="work-item-board__property-menu" placement="auto" offset={6}>
            <div className="work-item-board__property-menu-list" aria-label="标签选择">
              <label className="work-item-board__property-search"><Search size={14} aria-hidden="true" /><input value={labelQuery} onChange={(event) => setLabelQuery(event.currentTarget.value)} placeholder="搜索标签" aria-label="搜索标签" /></label>
              {filteredLabels.length ? filteredLabels.map((label) => <label key={label.labelId} className="work-item-board__label-option"><input type="checkbox" checked={editor.labelIds.includes(label.labelId)} onChange={(event) => patchEditor({ labelIds: event.currentTarget.checked ? [...editor.labelIds, label.labelId] : editor.labelIds.filter((id) => id !== label.labelId) })} /><i style={{ background: label.color }} aria-hidden="true" />{label.name}</label>) : <p>暂无标签</p>}
              {onCreateLabel ? <button type="button" className="work-item-board__label-create-option" disabled={pending || readOnly} onClick={openLabelCreateDialog}><Plus size={14} aria-hidden="true" /><span>新增标签</span></button> : null}
            </div>
          </AnchoredPopover>
        </span>
      </div>

      <footer>
        <Tooltip label="添加附件" side="top">
          <button type="button" className="work-item-board__attachment-button" aria-pressed={assetsOpen} onClick={() => setAssetsOpen((value) => !value)} aria-label="添加附件"><Paperclip size={18} aria-hidden="true" /></button>
        </Tooltip>
        <span className="work-item-board__dialog-footer-spacer" />
        {editor.kind === "edit" ? <InlineConfirmButton label="删除" confirmLabel="确认删除" cancelLabel="取消" danger disabled={pending || readOnly} onConfirm={onDelete} /> : null}
        <button type="button" onClick={onClose}>取消</button>
        <button type="button" className="work-item-board__add" disabled={pending || readOnly || !editor.title.trim() || dateInvalid} onClick={onSubmit}>{pending ? "保存中…" : editor.kind === "create" ? "创建" : "保存"}</button>
      </footer>
  </ProjectModalSurface>
  {labelCreateOpen ? <ProjectModalSurface backdropClassName="work-item-board__dialog-backdrop" className="work-item-board__dialog work-item-board__dialog--create-label" titleId={labelCreateTitleId} pending={creatingLabel || pending} initialFocusRef={labelNameRef} returnFocus={labelAnchorRef.current} onClose={() => setLabelCreateOpen(false)}>
    <header><h3 id={labelCreateTitleId}>新建标签</h3></header>
    <form id={labelCreateFormId} className="work-item-board__dialog-body work-item-board__label-create-form" onSubmit={(event) => { event.preventDefault(); void createLabel(); }}>
      <label><span>标签名称</span><input ref={labelNameRef} value={labelDraftName} maxLength={80} aria-label="标签名称" placeholder="请输入标签名称" onChange={(event) => setLabelDraftName(event.currentTarget.value)} /></label>
      <label><span>标签颜色</span><input type="color" value={labelDraftColor} aria-label="标签颜色" onChange={(event) => setLabelDraftColor(event.currentTarget.value)} /></label>
      {labelNameExists ? <p className="cloud-projects__inline-error">已存在同名标签。</p> : null}
    </form>
    <footer><button type="button" disabled={creatingLabel || pending} onClick={() => setLabelCreateOpen(false)}>取消</button><button type="submit" form={labelCreateFormId} className="work-item-board__add" disabled={!normalizedLabelDraftName || labelNameExists || creatingLabel || pending}>{creatingLabel ? "创建中…" : "创建"}</button></footer>
  </ProjectModalSurface> : null}</>;
}
