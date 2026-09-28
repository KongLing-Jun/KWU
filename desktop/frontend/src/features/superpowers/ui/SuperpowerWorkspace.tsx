import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  ArrowRight,
  BarChart3,
  MessageSquareText,
  Pin,
  Search,
  ShoppingBag,
  Sparkles,
  X,
  Zap,
} from "lucide-react";
import type { DashboardDocument, WidgetEnvelope } from "../runtime/contracts";
import { applyDesktopDashboardAction, CAPABILITY_ID, ensureDesktopDashboards, ensureDesktopEvidence, hasDesktopDashboardBridge, subscribeDesktopDashboard, type CapabilityTaskScope } from "../runtime/desktopDashboardBridge";
import { chartKindsFromPrompt, deriveChartWidget, type DerivedChartKind } from "../runtime/chartDerivation";
import { buildResearchFollowUpRequest, type ResearchFollowUpRequest } from "../runtime/followUpRequest";
import { buildResearchSessionSubmission, type EvidenceDocument, type ResearchExecutionResult } from "../runtime/researchExecution";
import { WidgetRenderer } from "../widgets/WidgetRenderer";
import type { ComposerCapabilitySubmit } from "../../../lib/types";
import type { CapabilityWelcomePresentation } from "../../../components/Welcome";
import "../superpowers.css";

type WorkspaceScreen = "catalog" | "ecommerce";
type DashboardViewScope = "session" | "aggregate";

const capabilities = [
  { title: "跨境电商超能力", description: "组合供应侧、目标市场与店铺经营证据，形成可追溯的验证判断。", icon: ShoppingBag, tone: "commerce", status: "可体验", available: true },
  { title: "经营分析超能力", description: "把分散经营数据整理为指标、异常与行动建议。", icon: BarChart3, tone: "analytics", status: "即将上线", available: false },
  { title: "客户洞察超能力", description: "归纳反馈主题、用户旅程与高价值需求机会。", icon: MessageSquareText, tone: "voice", status: "即将上线", available: false },
  { title: "知识研究超能力", description: "跨来源收集资料，保留引用并生成研究结论。", icon: Search, tone: "research", status: "规划中", available: false },
];

const ecommerceWelcomePresentation: CapabilityWelcomePresentation = {
  label: "跨境电商超能力",
  title: "从市场线索到可验证的经营判断",
  description: "围绕供应发现、目标市场、竞品和店铺经营证据开展研究，结论与来源会同步沉淀到右侧看板。",
  prompts: [
    { id: "supply", label: "发现潜力商品", prompt: "请从当前可用证据中发现有潜力的跨境商品，并说明筛选依据。" },
    { id: "market", label: "评估目标市场", prompt: "请评估适合进入的目标市场，比较需求、竞争和进入门槛。" },
    { id: "competitors", label: "分析竞品格局", prompt: "请分析主要竞品、价格带、卖点和可差异化机会。" },
    { id: "validation", label: "核验经营假设", prompt: "请核验当前经营假设，列出支持证据、反证和仍需补充的信息。" },
  ],
};

const createEmptyDashboardDocument = (): DashboardDocument => ({
  schemaVersion: 1,
  documentId: "primary",
  workspaceId: "workspace-commerce-research",
  taskId: "task-portable-blender-opportunity",
  revision: 0,
  updatedAt: new Date().toISOString(),
  widgets: [],
  appliedOperationIds: [],
});

const createEmptyEvidenceDocument = (): EvidenceDocument => ({
  schemaVersion: 1,
  capabilityId: CAPABILITY_ID,
  workspaceId: "workspace-commerce-research",
  taskId: "task-portable-blender-opportunity",
  revision: 1,
  updatedAt: new Date().toISOString(),
  artifacts: [],
});

type CitationReference = {
  widgetId: string;
  title: string;
  evidenceRef: string;
};

type FollowUp = {
  request: ResearchFollowUpRequest;
  status: "running" | "complete" | "error";
  executionMode: "session";
  sessionText?: string;
  sources?: ResearchExecutionResult["sources"];
  baselineAssistantIds?: string[];
};

export type SuperpowerComposerSend = (
  displayText: string,
  submitText?: string,
  capabilities?: ComposerCapabilitySubmit,
) => "submitted" | "handled-no-turn" | "blocked" | Promise<"submitted" | "handled-no-turn" | "blocked">;

export type SuperpowerSessionAdapter = {
  ready: boolean;
  running: boolean;
  unavailableReason?: string;
  messages: Array<{ id: string; role: "user" | "assistant"; text: string; streaming: boolean }>;
  assistantMessages: Array<{ id: string; text: string; streaming: boolean }>;
  submit: SuperpowerComposerSend;
};

export type SuperpowerWorkspaceProps = {
  sessionAdapter?: SuperpowerSessionAdapter;
  initialScreen?: WorkspaceScreen;
  navigationKey?: number;
  onCapabilityBindingChanged?: () => void;
  renderSessionTranscript?: (options?: { welcomePresentation?: CapabilityWelcomePresentation; onWelcomePrompt?: (text: string) => void }) => ReactNode;
  renderOperationalPrompt?: () => ReactNode;
  renderComposer?: (onSend: SuperpowerComposerSend) => ReactNode;
  renderSessionStatusBar?: () => ReactNode;
};

function SuperpowerCatalog({ onOpenEcommerce }: { onOpenEcommerce: () => void }) {
  return (
    <section className="superpower-catalog" aria-labelledby="superpower-catalog-title">
      <header className="superpower-catalog__hero">
        <div className="superpower-catalog__eyebrow"><Sparkles size={15} />超级工作流</div>
        <h1 id="superpower-catalog-title">选择你的超能力</h1>
        <p>把专家经验、数据工具和 Agent 工作流组合成一个开箱即用的业务能力。</p>
      </header>

      <div className="superpower-catalog__meta">
        <span>能力中心</span>
        <span>1 个可体验 · 3 个即将上线</span>
      </div>

      <div className="superpower-card-grid">
        {capabilities.map((capability) => {
          const Icon = capability.icon;
          return (
            <button
              key={capability.title}
              type="button"
              className={`superpower-card superpower-card--${capability.tone}${capability.available ? " superpower-card--available" : ""}`}
              disabled={!capability.available}
              onClick={capability.available ? onOpenEcommerce : undefined}
              aria-label={capability.available ? `打开${capability.title}` : `${capability.title}，${capability.status}`}
            >
              <span className="superpower-card__visual">
                <span className="superpower-card__icon"><Icon size={28} /></span>
                <span className="superpower-card__orb superpower-card__orb--one" />
                <span className="superpower-card__orb superpower-card__orb--two" />
              </span>
              <span className="superpower-card__body">
                <span className="superpower-card__topline">
                  <strong>{capability.title}</strong>
                  <small>{capability.status}</small>
                </span>
                <span className="superpower-card__description">{capability.description}</span>
                <span className="superpower-card__footer">
                  <span>{capability.available ? "供给发现 · 市场快照 · 经营验证" : "正在准备"}</span>
                  <ArrowRight size={16} />
                </span>
              </span>
            </button>
          );
        })}
      </div>

      <aside className="superpower-catalog__note">
        <Zap size={16} />
        <span><strong>什么是超能力？</strong> 它不是一条提示词，而是围绕业务目标组织好的数据、工具、过程和交付界面。</span>
      </aside>
    </section>
  );
}

function GeneratedResearchCanvas({ onCite, onPromote, onOpen, onRemove, promotedWidgetIds, document, selectedWidgetIds, focusedWidgetId }: { onCite: (widget: WidgetEnvelope) => void; onPromote?: (widget: WidgetEnvelope) => void; onOpen: (widget: WidgetEnvelope) => void; onRemove: (widget: WidgetEnvelope) => void; promotedWidgetIds: string[]; document: DashboardDocument; selectedWidgetIds: string[]; focusedWidgetId: string | null }) {
  const canvasWidgets = document.widgets.filter((widget) => widget.surface === "research-canvas");
  return (
    <div className="superpower-dashboard" aria-label="AI 生成研究组件" data-document-revision={document.revision}>
      {canvasWidgets.map((widget) => <WidgetRenderer key={widget.id} widget={widget} onCite={onCite} onPromote={onPromote} onOpen={onOpen} onRemove={onRemove} promoted={promotedWidgetIds.includes(widget.id)} selected={selectedWidgetIds.includes(widget.id)} focused={widget.id === focusedWidgetId} />)}
    </div>
  );
}

function EcommerceWorkspace({ sessionAdapter, renderSessionTranscript, renderOperationalPrompt, renderComposer, renderSessionStatusBar, navigationKey, onCapabilityBindingChanged }: { sessionAdapter?: SuperpowerSessionAdapter; renderSessionTranscript?: SuperpowerWorkspaceProps["renderSessionTranscript"]; renderOperationalPrompt?: () => ReactNode; renderComposer?: (onSend: SuperpowerComposerSend) => ReactNode; renderSessionStatusBar?: () => ReactNode; navigationKey: number; onCapabilityBindingChanged?: () => void }) {
  const [sessionDocument, setSessionDocument] = useState<DashboardDocument>(createEmptyDashboardDocument);
  const [aggregateDocument, setAggregateDocument] = useState<DashboardDocument>(createEmptyDashboardDocument);
  const [desktopScopes, setDesktopScopes] = useState<{ session: CapabilityTaskScope; aggregate: CapabilityTaskScope } | null>(null);
  const [evidenceDocument, setEvidenceDocument] = useState<EvidenceDocument>(createEmptyEvidenceDocument);
  const [persistenceMode, setPersistenceMode] = useState<"connecting" | "desktop" | "unavailable">("connecting");
  const [persistenceError, setPersistenceError] = useState("");
  const [dashboardView, setDashboardView] = useState<DashboardViewScope>("session");
  const dashboardDocument = dashboardView === "session" ? sessionDocument : aggregateDocument;
  const sessionCanvasWidgetCount = sessionDocument.widgets.filter((widget) => widget.surface === "research-canvas").length;
  const aggregateCanvasWidgetCount = aggregateDocument.widgets.filter((widget) => widget.surface === "research-canvas").length;
  const dashboardCanvasWidgetCount = dashboardView === "session" ? sessionCanvasWidgetCount : aggregateCanvasWidgetCount;
  const selectedResearchMode = "综合研究" as const;
  const [citations, setCitations] = useState<CitationReference[]>([]);
  const [followUps, setFollowUps] = useState<FollowUp[]>([]);
  const [focusedWidgetId] = useState<string | null>(null);
  const [detailWidget, setDetailWidget] = useState<WidgetEnvelope | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPersistenceMode("connecting");
    setPersistenceError("");
    setDesktopScopes(null);
    setSessionDocument(createEmptyDashboardDocument());
    setDashboardView("session");
    setCitations([]);
    if (!hasDesktopDashboardBridge()) {
      setPersistenceMode("unavailable");
      setPersistenceError("Desktop 看板桥接不可用。");
      return;
    }
    ensureDesktopDashboards(createEmptyDashboardDocument())
      .then(async ({ scopes, documents }) => ({ scopes, documents, evidence: await ensureDesktopEvidence(scopes.session, createEmptyEvidenceDocument()) }))
      .then(({ scopes, documents, evidence }) => {
        if (cancelled) return;
        setDesktopScopes(scopes);
        setSessionDocument(documents.session);
        setAggregateDocument(documents.aggregate);
        setEvidenceDocument(evidence);
        setPersistenceMode("desktop");
        onCapabilityBindingChanged?.();
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setPersistenceMode("unavailable");
        setPersistenceError(error instanceof Error ? error.message : String(error || "看板加载失败，请重新进入会话重试。"));
      });
    return () => { cancelled = true; };
  }, [navigationKey, onCapabilityBindingChanged]);

  useEffect(() => {
    if (persistenceMode !== "desktop" || !desktopScopes) return;
    const unsubscribeSession = subscribeDesktopDashboard(desktopScopes.session, setSessionDocument, setEvidenceDocument);
    const unsubscribeAggregate = subscribeDesktopDashboard(desktopScopes.aggregate, setAggregateDocument, setEvidenceDocument);
    return () => {
      unsubscribeSession();
      unsubscribeAggregate();
    };
  }, [desktopScopes, persistenceMode]);

  useEffect(() => {
    if (!detailWidget) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setDetailWidget(null);
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [detailWidget]);

  useEffect(() => {
    if (!sessionAdapter) return;
    const messageById = new Map(sessionAdapter.assistantMessages.map((message) => [message.id, message]));
    setFollowUps((current) => current.map((followUp) => {
      if (followUp.executionMode !== "session" || followUp.status !== "running") return followUp;
      const baseline = new Set(followUp.baselineAssistantIds ?? []);
      const candidates = sessionAdapter.assistantMessages.filter((message) => !baseline.has(message.id));
      const latest = candidates[candidates.length - 1];
      if (!latest) return followUp;
      const status = latest.streaming || sessionAdapter.running ? "running" : "complete";
      if (followUp.sessionText === latest.text && followUp.status === status && messageById.has(latest.id)) return followUp;
      return { ...followUp, status, sessionText: latest.text };
    }));
  }, [sessionAdapter?.assistantMessages, sessionAdapter?.running]);

  const selectedWidgetIds = useMemo(() => citations.map((citation) => citation.widgetId), [citations]);
  const promotedWidgetIds = useMemo(() => aggregateDocument.widgets.map((widget) => widget.id), [aggregateDocument.widgets]);
  const citeEvidence = (widget: WidgetEnvelope) => {
    const selected = citations.some((citation) => citation.widgetId === widget.id);
    if (selected) {
      setCitations((current) => current.filter((citation) => citation.widgetId !== widget.id));
      return;
    }
    setCitations((current) => current.some((citation) => citation.widgetId === widget.id) ? current : [...current, {
      widgetId: widget.id,
      title: widget.title,
      evidenceRef: widget.evidenceRefs[0] ?? `组件快照 r${widget.revision}`,
    }]);
  };
  const promoteWidget = async (widget: WidgetEnvelope) => {
    if (!desktopScopes || aggregateDocument.widgets.some((candidate) => candidate.id === widget.id)) return;
    const { revision: _revision, ...promotedWidget } = widget;
    try {
      const document = await applyDesktopDashboardAction(desktopScopes.aggregate, {
        type: "dashboard.add_widget",
        operationId: `ui:promote:${widget.id}:r${widget.revision}`,
        baseRevision: aggregateDocument.revision,
        at: new Date().toISOString(),
        widget: promotedWidget,
      });
      setAggregateDocument(document);
    } catch (error) {
      setPersistenceError(error instanceof Error ? error.message : "组件加入聚合看板失败。");
    }
  };
  const removeWidget = async (widget: WidgetEnvelope) => {
    if (!desktopScopes) return;
    try {
      setPersistenceError("");
      const document = await applyDesktopDashboardAction(desktopScopes[dashboardView], {
        type: "dashboard.remove_widget",
        operationId: `ui:remove:${widget.id}:r${widget.revision}:${Date.now()}`,
        baseRevision: dashboardDocument.revision,
        at: new Date().toISOString(),
        widgetId: widget.id,
      });
      if (dashboardView === "session") setSessionDocument(document);
      else setAggregateDocument(document);
      setCitations((current) => current.filter((citation) => citation.widgetId !== widget.id));
      setDetailWidget((current) => current?.id === widget.id ? null : current);
    } catch (error) {
      setPersistenceError(error instanceof Error ? error.message : "卡片关闭失败。");
    }
  };
  const createDerivedChart = async (kind: DerivedChartKind, sourceDocument = dashboardDocument) => {
    if (!desktopScopes) return null;
    const widget = deriveChartWidget(sourceDocument, kind);
    if (!widget) {
      setPersistenceError("当前看板没有可转换的商品目录数据。");
      return null;
    }
    try {
      setPersistenceError("");
      const document = await applyDesktopDashboardAction(desktopScopes[dashboardView], {
        type: "dashboard.add_widget",
        operationId: `ui:derive-chart:${kind}:r${sourceDocument.revision}`,
        baseRevision: sourceDocument.revision,
        at: new Date().toISOString(),
        widget,
      });
      if (dashboardView === "session") setSessionDocument(document);
      else setAggregateDocument(document);
      return document;
    } catch (error) {
      setPersistenceError(error instanceof Error ? error.message : "图表生成失败。");
      return null;
    }
  };
  const submitFollowUp: SuperpowerComposerSend = async (displayText, composerSubmitText = displayText, capabilities) => {
    const normalized = displayText.trim();
    if (!normalized || !sessionAdapter) return "blocked";
    let requestDocument = dashboardDocument;
    for (const kind of chartKindsFromPrompt(normalized)) {
      const generated = await createDerivedChart(kind, requestDocument);
      if (generated) requestDocument = generated;
    }
    const createdAt = new Date().toISOString();
    const request = buildResearchFollowUpRequest({
      requestId: `ui:follow-up:${createdAt}:${followUps.length + 1}`,
      createdAt,
      capabilityId: CAPABILITY_ID,
      researchMode: selectedResearchMode,
      question: normalized,
      selectedWidgetIds,
      document: requestDocument,
    });
    try {
      const submission = await buildResearchSessionSubmission(request, evidenceDocument.artifacts, composerSubmitText);
      const baselineAssistantIds = sessionAdapter.assistantMessages.map((message) => message.id);
      setFollowUps((current) => [...current, {
        request,
        status: "running",
        executionMode: "session",
        baselineAssistantIds,
        sources: submission.sources,
      }]);
      const outcome = await sessionAdapter.submit(submission.displayText, submission.submitText, capabilities);
      if (outcome !== "submitted") {
        setFollowUps((current) => current.map((followUp) => followUp.request.requestId === request.requestId ? { ...followUp, status: "error" } : followUp));
      }
      return outcome;
    } catch {
      setFollowUps((current) => current.map((followUp) => followUp.request.requestId === request.requestId ? { ...followUp, status: "error" } : followUp));
      return "blocked";
    }
  };
  return (
    <section className="superpower-ecommerce" aria-label="跨境电商超能力工作台">
      <div className="superpower-ecommerce__body">
        <section className="superpower-conversation" aria-label="研究过程">
          <div className="superpower-conversation__timeline">
            {renderSessionTranscript?.({
              welcomePresentation: ecommerceWelcomePresentation,
              onWelcomePrompt: (text) => { void submitFollowUp(text); },
            })}
          </div>
          <div className="superpower-composer-wrap">
            {renderOperationalPrompt?.()}
            {renderComposer && sessionAdapter
              ? <div className="superpower-shared-composer">
                  {renderComposer(submitFollowUp)}
                </div>
              : <div className="superpower-composer-unavailable">请在 Desktop 会话中使用研究输入框。</div>}
          </div>
          {renderSessionStatusBar?.()}
        </section>

        <aside className="superpower-evidence" aria-label="AI 研究组件画布">
            <header className="superpower-evidence__scope-header">
              <div className="superpower-evidence__scope-main">
                <div className="superpower-dashboard-scope" role="tablist" aria-label="看板范围">
                  <button type="button" role="tab" aria-selected={dashboardView === "session"} onClick={() => { setDashboardView("session"); setCitations([]); setDetailWidget(null); }}>本次会话 <span>{sessionCanvasWidgetCount}</span></button>
                  <button type="button" role="tab" aria-selected={dashboardView === "aggregate"} onClick={() => { setDashboardView("aggregate"); setCitations([]); setDetailWidget(null); }}>聚合看板 <span>{aggregateCanvasWidgetCount}</span></button>
                </div>
              </div>
              <small>{dashboardView === "session" ? "MCP 结果默认写入当前会话" : "跨会话沉淀的已选组件"}</small>
            </header>
            <div className="superpower-evidence__content">
              {persistenceMode === "connecting" ? <div className="superpower-dashboard-state" role="status"><i className="superpower-spinner" />正在恢复看板…</div> : null}
              {persistenceMode === "unavailable" ? <div className="superpower-dashboard-state superpower-dashboard-state--error" role="alert"><strong>看板暂不可用</strong><span>{persistenceError}</span></div> : null}
              {persistenceMode === "desktop" && persistenceError ? <div className="superpower-dashboard-state superpower-dashboard-state--error" role="alert"><span>{persistenceError}</span></div> : null}
              {persistenceMode === "desktop" && dashboardCanvasWidgetCount === 0 ? <div className="superpower-dashboard-state"><Pin size={17} /><strong>{dashboardView === "session" ? "本次会话还没有看板组件" : "聚合看板还是空的"}</strong><span>{dashboardView === "session" ? "开始研究后，MCP 生成的组件会保存在这里。" : "从会话看板将有价值的组件加入聚合。"}</span></div> : null}
              <GeneratedResearchCanvas onCite={citeEvidence} onPromote={dashboardView === "session" ? promoteWidget : undefined} onOpen={setDetailWidget} onRemove={removeWidget} promotedWidgetIds={promotedWidgetIds} document={dashboardDocument} selectedWidgetIds={selectedWidgetIds} focusedWidgetId={focusedWidgetId} />
            </div>
        </aside>
      </div>

      {detailWidget ? <div className="superpower-widget-dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setDetailWidget(null); }}>
        <section className="superpower-widget-dialog" role="dialog" aria-modal="true" aria-labelledby="superpower-widget-dialog-title">
          <header className="superpower-widget-dialog__header">
            <div><small>智能看板组件</small><strong id="superpower-widget-dialog-title">{detailWidget.title}</strong><span>{detailWidget.type} · r{detailWidget.revision}</span></div>
            <button type="button" aria-label="关闭卡片详情" onClick={() => setDetailWidget(null)}><X size={16} /></button>
          </header>
          <div className="superpower-widget-dialog__content"><WidgetRenderer widget={detailWidget} /></div>
          {detailWidget.evidenceRefs.length ? <footer><small>证据来源</small>{detailWidget.evidenceRefs.map((reference) => <span key={reference}>{reference}</span>)}</footer> : null}
        </section>
      </div> : null}

    </section>
  );
}

export function SuperpowerWorkspace({ sessionAdapter, initialScreen = "catalog", navigationKey = 0, onCapabilityBindingChanged, renderSessionTranscript, renderOperationalPrompt, renderComposer, renderSessionStatusBar }: SuperpowerWorkspaceProps) {
  const [screen, setScreen] = useState<WorkspaceScreen>(initialScreen);
  useEffect(() => {
    setScreen(initialScreen);
  }, [initialScreen, navigationKey]);
  return (
    <div className="superpower-workspace" data-screen={screen}>
      {screen === "catalog" ? <SuperpowerCatalog onOpenEcommerce={() => setScreen("ecommerce")} /> : <EcommerceWorkspace sessionAdapter={sessionAdapter} renderSessionTranscript={renderSessionTranscript} renderOperationalPrompt={renderOperationalPrompt} renderComposer={renderComposer} renderSessionStatusBar={renderSessionStatusBar} navigationKey={navigationKey} onCapabilityBindingChanged={onCapabilityBindingChanged} />}
    </div>
  );
}
