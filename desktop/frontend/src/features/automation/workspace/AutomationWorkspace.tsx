import { useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import {
  AlarmClock,
  BarChart3,
  BriefcaseBusiness,
  CalendarDays,
  CircleHelp,
  ClipboardList,
  FileText,
  Film,
  HeartPulse,
  Languages,
  ListChecks,
  MessageCircle,
  Moon,
  Newspaper,
  Plus,
  type LucideIcon,
} from "lucide-react";
import type {
  AutomationRunSummary,
  AutomationTaskTemplate,
  HeartbeatTask,
} from "../heartbeat/heartbeat.types";

export type AutomationWorkspaceTab = "scheduled" | "runs";

export interface AutomationWorkspaceProps {
  templates: readonly AutomationTaskTemplate[];
  tasks: readonly HeartbeatTask[];
  runs: readonly AutomationRunSummary[];
  activeTab?: AutomationWorkspaceTab;
  onTabChange?: (tab: AutomationWorkspaceTab) => void;
  onAddTask: () => void;
  onSelectTemplate: (template: AutomationTaskTemplate) => void;
	canCreate?: boolean;
	createDisabledTitle?: string;
  renderTask?: (task: HeartbeatTask) => ReactNode;
  renderRun?: (run: AutomationRunSummary) => ReactNode;
}

const TEMPLATE_ICONS: Record<string, LucideIcon> = {
  news: Newspaper,
  language: Languages,
  moon: Moon,
  clipboard: ClipboardList,
  film: Film,
  calendar: CalendarDays,
  question: CircleHelp,
  message: MessageCircle,
  health: HeartPulse,
  chat: MessageCircle,
  checklist: ListChecks,
  business: BarChart3,
  task: FileText,
};

function compareTaskIdentity(left: HeartbeatTask, right: HeartbeatTask): number {
  const titleOrder = left.title.localeCompare(right.title, "zh-CN", { sensitivity: "base" });
  if (titleOrder !== 0) return titleOrder;
  if (left.id === right.id) return 0;
  return left.id < right.id ? -1 : 1;
}

function validNextRunAt(task: HeartbeatTask): number | null {
  if (!task.enabled || !task.nextRunAt) return null;
  const timestamp = Date.parse(task.nextRunAt);
  return Number.isFinite(timestamp) ? timestamp : null;
}

export function orderAutomationWorkspaceTasks(tasks: readonly HeartbeatTask[]): HeartbeatTask[] {
  return [...tasks].sort((left, right) => {
    if (left.enabled !== right.enabled) return left.enabled ? -1 : 1;
    if (!left.enabled) return compareTaskIdentity(left, right);

    const leftNextRunAt = validNextRunAt(left);
    const rightNextRunAt = validNextRunAt(right);
    if (leftNextRunAt !== null && rightNextRunAt !== null && leftNextRunAt !== rightNextRunAt) {
      return leftNextRunAt - rightNextRunAt;
    }
    if (leftNextRunAt !== rightNextRunAt) return leftNextRunAt === null ? 1 : -1;
    return compareTaskIdentity(left, right);
  });
}

function TemplateCard({
  template,
  onSelect,
	disabled,
}: {
  template: AutomationTaskTemplate;
  onSelect: (template: AutomationTaskTemplate) => void;
	disabled?: boolean;
}) {
  const Icon = TEMPLATE_ICONS[template.icon ?? ""] ?? FileText;
  return (
    <button
      className="automation-workspace__template-card"
      data-testid="automation-template-card"
      type="button"
      onClick={() => onSelect(template)}
	  disabled={disabled}
      aria-label={`使用模板：${template.title}`}
    >
      <span className="automation-workspace__template-icon" aria-hidden="true">
        <Icon size={21} strokeWidth={1.8} />
      </span>
      <span className="automation-workspace__template-copy">
        <strong>{template.title}</strong>
        <span>{template.description}</span>
      </span>
    </button>
  );
}

function DefaultTaskCard({ task }: { task: HeartbeatTask }) {
  return (
    <article className="automation-workspace__item-card">
      <span className="automation-workspace__item-icon" aria-hidden="true">
        <AlarmClock size={18} />
      </span>
      <span className="automation-workspace__item-copy">
        <strong>{task.title}</strong>
        <span>{task.prompt}</span>
      </span>
    </article>
  );
}

function DefaultRunCard({ run }: { run: AutomationRunSummary }) {
  return (
    <article className="automation-workspace__item-card">
      <span className="automation-workspace__item-icon" aria-hidden="true">
        <BriefcaseBusiness size={18} />
      </span>
      <span className="automation-workspace__item-copy">
        <strong>{run.taskName || run.taskId}</strong>
        <span>{run.summary || run.errorMessage || run.status}</span>
      </span>
      <span className={`automation-workspace__status automation-workspace__status--${run.status}`}>
        {run.status}
      </span>
    </article>
  );
}

export function AutomationWorkspace({
  templates,
  tasks,
  runs,
  activeTab,
  onTabChange,
  onAddTask,
  onSelectTemplate,
  renderTask,
  renderRun,
	canCreate = true,
	createDisabledTitle,
}: AutomationWorkspaceProps) {
  const [internalTab, setInternalTab] = useState<AutomationWorkspaceTab>("scheduled");
	const scheduledTabRef = useRef<HTMLButtonElement>(null);
	const runsTabRef = useRef<HTMLButtonElement>(null);
  const selectedTab = activeTab ?? internalTab;
  const orderedTasks = useMemo(() => orderAutomationWorkspaceTasks(tasks), [tasks]);

  const selectTab = (tab: AutomationWorkspaceTab) => {
    if (activeTab === undefined) setInternalTab(tab);
    onTabChange?.(tab);
  };
	const handleTabKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
		let next: AutomationWorkspaceTab | undefined;
		if (event.key === "ArrowLeft" || event.key === "ArrowRight") next = selectedTab === "scheduled" ? "runs" : "scheduled";
		if (event.key === "Home") next = "scheduled";
		if (event.key === "End") next = "runs";
		if (!next) return;
		event.preventDefault();
		selectTab(next);
		(next === "scheduled" ? scheduledTabRef : runsTabRef).current?.focus();
	};

  return (
    <section className="automation-workspace" aria-label="自动化工作区">
      <div className="automation-workspace__tab-bar" role="tablist" aria-label="自动化视图">
        <button
		  ref={scheduledTabRef}
          id="automation-scheduled-tab"
          className={`automation-workspace__tab${selectedTab === "scheduled" ? " automation-workspace__tab--active" : ""}`}
          type="button"
          role="tab"
          aria-selected={selectedTab === "scheduled"}
          aria-controls="automation-scheduled-panel"
          tabIndex={selectedTab === "scheduled" ? 0 : -1}
          onClick={() => selectTab("scheduled")}
		  onKeyDown={handleTabKeyDown}
        >
          定时任务
        </button>
        <button
		  ref={runsTabRef}
          id="automation-runs-tab"
          className={`automation-workspace__tab${selectedTab === "runs" ? " automation-workspace__tab--active" : ""}`}
          type="button"
          role="tab"
          aria-selected={selectedTab === "runs"}
          aria-controls="automation-runs-panel"
          tabIndex={selectedTab === "runs" ? 0 : -1}
          onClick={() => selectTab("runs")}
		  onKeyDown={handleTabKeyDown}
        >
          运行记录
        </button>
      </div>

      <div className="automation-workspace__scroll">
        {selectedTab === "scheduled" ? (
          <div
            id="automation-scheduled-panel"
            className="automation-workspace__panel"
            role="tabpanel"
            aria-labelledby="automation-scheduled-tab"
          >
            {tasks.length === 0 ? (
              <div className="automation-workspace__empty" data-testid="automation-empty-state">
                <AlarmClock size={46} strokeWidth={1.5} aria-hidden="true" />
                <p>开启你的第一个自动化任务吧</p>
                <button
                  className="automation-workspace__add-button heartbeat-page-heading__add"
                  data-testid="automation-add-task"
                  type="button"
                  onClick={onAddTask}
				  disabled={!canCreate}
				  title={!canCreate ? createDisabledTitle : undefined}
                >
                  <Plus size={17} strokeWidth={2} aria-hidden="true" />
                  添加自动化
                </button>
              </div>
            ) : (
              <section className="automation-workspace__task-section" aria-labelledby="automation-task-list-title">
                <div className="automation-workspace__section-heading">
                  <h2 id="automation-task-list-title">我的自动化</h2>
                  <button className="heartbeat-page-heading__add" type="button" onClick={onAddTask} disabled={!canCreate} title={!canCreate ? createDisabledTitle : undefined}>
                    <Plus size={15} aria-hidden="true" />
                    添加自动化
                  </button>
                </div>
                <div className="automation-workspace__item-list">
                  {orderedTasks.map((task) => (
                    <div key={task.id}>{renderTask ? renderTask(task) : <DefaultTaskCard task={task} />}</div>
                  ))}
                </div>
              </section>
            )}

            <section className="automation-workspace__template-section" aria-labelledby="automation-template-title">
              <h2 id="automation-template-title">自动化任务模板</h2>
              <div className="automation-workspace__template-grid">
                {templates.map((template) => (
                  <TemplateCard key={template.id} template={template} onSelect={onSelectTemplate} disabled={!canCreate} />
                ))}
              </div>
            </section>
          </div>
        ) : (
          <div
            id="automation-runs-panel"
            className="automation-workspace__panel automation-workspace__panel--runs"
            data-testid="automation-run-history"
            role="tabpanel"
            aria-labelledby="automation-runs-tab"
          >
            {runs.length === 0 ? (
              <div className="automation-workspace__empty automation-workspace__empty--runs">
                <ClipboardList size={46} strokeWidth={1.5} aria-hidden="true" />
                <p>暂无运行记录</p>
                <span>自动化任务运行后，记录会显示在这里。</span>
              </div>
            ) : (
              <section className="automation-workspace__run-section" aria-labelledby="automation-run-list-title">
                <h2 id="automation-run-list-title">运行记录</h2>
                <div className="automation-workspace__item-list">
                  {runs.map((run) => (
                    <div key={`${run.taskId}:${run.runId}`}>
                      {renderRun ? renderRun(run) : <DefaultRunCard run={run} />}
                    </div>
                  ))}
                </div>
              </section>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
