import { useState } from "react";
import { CalendarDays, FileText, Pencil, UserRound } from "lucide-react";
import type { ProjectGateway } from "../data/projectGateway";
import { formatProjectDateTime } from "../model/projectDateTime";
import type { CloudProjectActivity, CloudProjectAsset, CloudProjectWorkItem } from "../model/projectTypes";
import { workItemPriorityLabel, type WorkItemPriority } from "./WorkItemEditorDialog";
import { ProjectModalSurface } from "./ProjectModalSurface";

const STATUS_LABELS = { todo: "待开始", in_progress: "进行中", paused: "已暂停", done: "已完成" } as const;
const ACTIVITY_LABELS: Record<string, string> = {
  "work_item.created": "创建了事项",
  "work_item.updated": "更新了事项",
  "work_item.moved": "移动了事项",
};

export function WorkItemDetailDialog({ item, assets, activities, activityLoading, gateway, returnFocus, onEdit, onClose }: {
  item: CloudProjectWorkItem;
  assets: CloudProjectAsset[];
  activities: CloudProjectActivity[];
  activityLoading: boolean;
  gateway: ProjectGateway;
  returnFocus?: HTMLElement | null;
  onEdit: () => void;
  onClose: () => void;
}) {
  const [assetPending, setAssetPending] = useState("");
  const [assetError, setAssetError] = useState("");
  const openAsset = async (asset: CloudProjectAsset) => {
    setAssetPending(asset.assetId);
    setAssetError("");
    try { await gateway.downloadCloudProjectAsset(item.projectId, asset.assetId); }
    catch { setAssetError("附件打开失败，请稍后重试。"); }
    finally { setAssetPending(""); }
  };

  return <ProjectModalSurface backdropClassName="work-item-board__dialog-backdrop" className="work-item-board__dialog work-item-board__dialog--detail" titleId="work-item-detail-title" returnFocus={returnFocus} onClose={onClose}>
    <header className="work-item-board__detail-header">
      <h3 id="work-item-detail-title">事项详情</h3>
      <button type="button" onClick={onEdit}><Pencil size={15} aria-hidden="true" />编辑</button>
    </header>
    <div className="work-item-board__detail-layout">
      <main>
        <h2>{item.title}</h2>
        <p className="work-item-board__detail-description">{item.description || "暂无描述"}</p>
        <section className="work-item-board__detail-attachments" aria-labelledby="work-item-detail-attachments-title">
          <h4 id="work-item-detail-attachments-title">附件 <span>{assets.length}</span></h4>
          {assets.length ? <ul>{assets.map((asset) => <li key={asset.assetId}><button type="button" disabled={assetPending === asset.assetId} onClick={() => void openAsset(asset)}><FileText size={16} aria-hidden="true" /><span>{asset.filename}</span></button></li>)}</ul> : <p>暂无附件</p>}
          {assetError ? <p role="alert">{assetError}</p> : null}
        </section>
        <section className="work-item-board__detail-activity" aria-labelledby="work-item-detail-activity-title">
          <h4 id="work-item-detail-activity-title">动态</h4>
          {activityLoading ? <p>动态加载中…</p> : null}
          {!activityLoading && activities.length ? <ol>{activities.map((activity) => <li key={activity.activityId}><span className="work-item-board__detail-activity-dot" aria-hidden="true" /><p><strong>{activity.authorDisplayName}</strong> · {ACTIVITY_LABELS[activity.eventName || ""] || "更新了事项"} · <time dateTime={activity.createdAt}>{formatProjectDateTime(activity.createdAt)}</time></p></li>)}</ol> : null}
          {!activityLoading && !activities.length ? <ol><li><span className="work-item-board__detail-activity-dot" aria-hidden="true" /><p>创建了事项 · <time dateTime={item.createdAt}>{formatProjectDateTime(item.createdAt)}</time></p></li></ol> : null}
        </section>
      </main>
      <aside aria-label="事项属性">
        <dl>
          <div><dt>状态</dt><dd>{STATUS_LABELS[item.status]}</dd></div>
          <div><dt>处理人</dt><dd><UserRound size={14} aria-hidden="true" />{item.assigneeDisplayName || "未分配"}</dd></div>
          <div><dt>开始日期</dt><dd><CalendarDays size={14} aria-hidden="true" />{item.startAt ? new Date(item.startAt).toLocaleDateString("zh-CN") : "未设置"}</dd></div>
          <div><dt>截止日期</dt><dd><CalendarDays size={14} aria-hidden="true" />{item.dueAt ? new Date(item.dueAt).toLocaleDateString("zh-CN") : "未设置"}</dd></div>
          <div><dt>优先级</dt><dd>{workItemPriorityLabel(item.priority as WorkItemPriority | null, "未设置")}</dd></div>
          <div><dt>标签</dt><dd>{item.labels.length ? item.labels.map((label) => label.name).join("、") : "未设置"}</dd></div>
        </dl>
      </aside>
    </div>
  </ProjectModalSurface>;
}
