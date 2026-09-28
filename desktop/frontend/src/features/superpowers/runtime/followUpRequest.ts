import type { DashboardDocument, WidgetEnvelope } from "./contracts";

export const RESEARCH_FOLLOW_UP_LIMITS = {
  questionChars: 4_000,
  citations: 6,
  evidenceRefsPerCitation: 4,
  evidenceRefChars: 200,
} as const;

export type ResearchFollowUpCitation = {
  order: number;
  widgetId: string;
  widgetType: WidgetEnvelope["type"];
  widgetVersion: WidgetEnvelope["version"];
  widgetRevision: number;
  surface: WidgetEnvelope["surface"];
  title: string;
  evidenceRefs: string[];
};

export type ResearchFollowUpRequest = {
  schemaVersion: 1;
  requestId: string;
  createdAt: string;
  capabilityId: string;
  workspaceId: string;
  taskId: string;
  documentId: string;
  documentRevision: number;
  researchMode: string;
  question: string;
  citations: ResearchFollowUpCitation[];
  omissions: {
    questionChars: number;
    citations: number;
    evidenceRefs: number;
    missingWidgets: number;
  };
};

type BuildResearchFollowUpRequestInput = {
  requestId: string;
  createdAt: string;
  capabilityId: string;
  researchMode: string;
  question: string;
  selectedWidgetIds: string[];
  document: DashboardDocument;
};

export function buildResearchFollowUpRequest(input: BuildResearchFollowUpRequestInput): ResearchFollowUpRequest {
  const normalizedQuestion = input.question.trim();
  const question = normalizedQuestion.slice(0, RESEARCH_FOLLOW_UP_LIMITS.questionChars);
  const uniqueSelectedIds = Array.from(new Set(input.selectedWidgetIds));
  const boundedIds = uniqueSelectedIds.slice(0, RESEARCH_FOLLOW_UP_LIMITS.citations);
  const widgetsById = new Map(input.document.widgets.map((widget) => [widget.id, widget]));
  let missingWidgets = 0;
  let omittedEvidenceRefs = 0;

  const citations = boundedIds.flatMap((widgetId) => {
    const widget = widgetsById.get(widgetId);
    if (!widget) {
      missingWidgets += 1;
      return [];
    }
    const validEvidenceRefs = Array.from(new Set(widget.evidenceRefs.map((reference) => reference.trim())))
      .filter((reference) => reference.length > 0 && reference.length <= RESEARCH_FOLLOW_UP_LIMITS.evidenceRefChars);
    omittedEvidenceRefs += widget.evidenceRefs.length - Math.min(validEvidenceRefs.length, RESEARCH_FOLLOW_UP_LIMITS.evidenceRefsPerCitation);
    return [{
      order: 0,
      widgetId: widget.id,
      widgetType: widget.type,
      widgetVersion: widget.version,
      widgetRevision: widget.revision,
      surface: widget.surface,
      title: widget.title.slice(0, 160),
      evidenceRefs: validEvidenceRefs.slice(0, RESEARCH_FOLLOW_UP_LIMITS.evidenceRefsPerCitation),
    } satisfies ResearchFollowUpCitation];
  }).map((citation, index) => ({ ...citation, order: index + 1 }));

  return {
    schemaVersion: 1,
    requestId: input.requestId,
    createdAt: input.createdAt,
    capabilityId: input.capabilityId,
    workspaceId: input.document.workspaceId,
    taskId: input.document.taskId,
    documentId: input.document.documentId,
    documentRevision: input.document.revision,
    researchMode: input.researchMode,
    question,
    citations,
    omissions: {
      questionChars: normalizedQuestion.length - question.length,
      citations: Math.max(0, uniqueSelectedIds.length - RESEARCH_FOLLOW_UP_LIMITS.citations),
      evidenceRefs: Math.max(0, omittedEvidenceRefs),
      missingWidgets,
    },
  };
}
