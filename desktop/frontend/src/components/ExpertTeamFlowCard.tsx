import { AlertTriangle, CheckCircle2, CircleDashed, Gauge, ListTree, PlayCircle, Users } from "lucide-react";
import type { CSSProperties } from "react";
import type { ExpertTeamFlowModel, ExpertTeamStage, ExpertTeamTaskNode } from "../lib/expertTeamFlow";

export function ExpertTeamFlowCard({ model }: { model: ExpertTeamFlowModel }) {
  const visibleNodes = model.nodes.slice(0, 6);
  return (
    <section className="expert-team-flow-card" data-stage={model.stage} aria-label="专家团协作">
      <header className="expert-team-flow-card__head">
        <div className="expert-team-flow-card__icon" aria-hidden="true">
          <Users size={17} />
        </div>
        <div className="expert-team-flow-card__title">
          <span>创建团队</span>
          <strong>{model.teamName}</strong>
          <small>{model.title}</small>
        </div>
        <StageBadge stage={model.stage} />
      </header>

      <div className="expert-team-flow-card__stages" aria-label="团队工作流">
        <StageStep label="目标拆解" active={model.stage === "planning"} done={model.stage !== "planning"} />
        <StageStep label="专家分派" active={model.stage === "executing"} done={model.stage === "completed" || model.stage === "blocked"} />
        <StageStep label="执行校验" active={model.stage === "executing"} done={model.stage === "completed"} />
        <StageStep label="汇总输出" active={model.stage === "completed"} done={model.stage === "completed"} />
      </div>

      <div className="expert-team-flow-card__body">
        <div className="expert-team-flow-card__section-title">
          <ListTree size={14} aria-hidden="true" />
          <span>任务树</span>
        </div>
        <div className="expert-team-flow-card__nodes">
          {visibleNodes.map((node) => (
            <TaskNodeRow key={node.nodeId} node={node} />
          ))}
        </div>
        {model.nodes.length > visibleNodes.length && (
          <div className="expert-team-flow-card__more">还有 {model.nodes.length - visibleNodes.length} 个任务</div>
        )}
      </div>

      {(model.summary || model.result) && (
        <footer className="expert-team-flow-card__summary">
          <strong>最终输出</strong>
          <span>{model.summary || model.result}</span>
        </footer>
      )}
    </section>
  );
}

function StageBadge({ stage }: { stage: ExpertTeamStage }) {
  const icon =
    stage === "completed" ? <CheckCircle2 size={14} /> :
    stage === "blocked" ? <AlertTriangle size={14} /> :
    stage === "executing" ? <PlayCircle size={14} /> :
    <CircleDashed size={14} />;
  return (
    <span className="expert-team-flow-card__badge" data-stage={stage}>
      {icon}
      {stageLabel(stage)}
    </span>
  );
}

function StageStep({ label, active, done }: { label: string; active: boolean; done: boolean }) {
  return (
    <span className="expert-team-flow-card__stage" data-active={active || undefined} data-done={done || undefined}>
      <i aria-hidden="true">{done ? <CheckCircle2 size={12} /> : active ? <PlayCircle size={12} /> : <CircleDashed size={12} />}</i>
      {label}
    </span>
  );
}

function TaskNodeRow({ node }: { node: ExpertTeamTaskNode }) {
  return (
    <article className="expert-team-flow-card__node" data-active={node.active || undefined} style={{ "--node-depth": node.depth } as CSSProperties & Record<"--node-depth", number>}>
      <div className="expert-team-flow-card__node-dot" aria-hidden="true" />
      <div className="expert-team-flow-card__node-main">
        <div className="expert-team-flow-card__node-title">
          <strong>{node.title}</strong>
          <span>{nodeStatusLabel(node.status)}</span>
        </div>
        <div className="expert-team-flow-card__node-meta">
          {node.expertName && <span>专家：{node.expertName}</span>}
          {node.expertRole && <span>角色：{node.expertRole}</span>}
          {node.tools.length > 0 && <span>工具：{node.tools.length}</span>}
          <span>置信度：{node.verification.confidencePercent == null ? "待后端接入" : `${node.verification.confidencePercent}%`}</span>
        </div>
        {(node.goal !== node.title || node.contextExpectation || node.outputExpectation) && (
          <div className="expert-team-flow-card__node-detail">
            {node.goal !== node.title && <span>目标：{node.goal}</span>}
            {node.contextExpectation && <span>上下文：{node.contextExpectation}</span>}
            {node.outputExpectation && <span>输出：{node.outputExpectation}</span>}
          </div>
        )}
        <div className="expert-team-flow-card__progress" aria-label={`完成度 ${node.verification.completionPercent}%`}>
          <i style={{ width: `${node.verification.completionPercent}%` }} />
        </div>
      </div>
      <div className="expert-team-flow-card__score" title="完成度">
        <Gauge size={13} aria-hidden="true" />
        {node.verification.completionPercent}%
      </div>
    </article>
  );
}

function stageLabel(stage: ExpertTeamStage): string {
  switch (stage) {
    case "completed": return "已完成";
    case "blocked": return "已阻塞";
    case "executing": return "执行中";
    default: return "规划中";
  }
}

function nodeStatusLabel(status: string): string {
  if (status === "completed" || status === "done" || status === "succeeded") return "完成";
  if (status === "running" || status === "started" || status === "in_progress") return "执行中";
  if (status === "failed" || status === "error" || status === "blocked") return "阻塞";
  return "等待";
}
