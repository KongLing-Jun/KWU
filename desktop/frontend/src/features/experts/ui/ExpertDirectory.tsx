import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { ChangeEvent, ReactNode } from "react";
import { createPortal } from "react-dom";
import type { LucideIcon } from "lucide-react";
import {
  BarChart3,
  Bot,
  BriefcaseBusiness,
  ArrowLeft,
  ChevronRight,
  Code2,
  Compass,
  Copy,
  FileCheck2,
  FileText,
  FlaskConical,
  Pencil,
  Plus,
  Search,
  ShieldCheck,
  Star,
  Trash2,
  Upload,
  Users,
  Workflow,
  X,
} from "lucide-react";
import { useT } from "../../../lib/i18n";
import { app } from "../../../lib/bridge";
import { expertAvatarSrcForInfo, expertMemberAvatarSrc } from "../../../lib/expertAssets";
import type { ExpertInfo, ExpertLibraryState, UserExpertInfo, UserExpertInput } from "../../../lib/types";
import { useHorizontalDragScroll } from "../../../hooks/useHorizontalDragScroll";

const architectAvatar = new URL("../../../assets/experts/architect.png", import.meta.url).href;
const automationFlowAvatar = new URL("../../../assets/experts/automation-flow.png", import.meta.url).href;
const codeDeliveryTeamAvatar = new URL("../../../assets/experts/code-delivery-team.png", import.meta.url).href;
const codeReviewAvatar = new URL("../../../assets/experts/code-review.png", import.meta.url).href;
const customerOpsTeamAvatar = new URL("../../../assets/experts/customer-ops-team.png", import.meta.url).href;
const dataAnalysisTeamAvatar = new URL("../../../assets/experts/data-analysis-team.png", import.meta.url).href;
const dataReportAvatar = new URL("../../../assets/experts/data-report.png", import.meta.url).href;
const docReportAvatar = new URL("../../../assets/experts/doc-report.png", import.meta.url).href;
const officeAutomationTeamAvatar = new URL("../../../assets/experts/office-automation-team.png", import.meta.url).href;
const opsGrowthAvatar = new URL("../../../assets/experts/ops-growth.png", import.meta.url).href;
const projectDeliveryTeamAvatar = new URL("../../../assets/experts/project-delivery-team.png", import.meta.url).href;
const researchAvatar = new URL("../../../assets/experts/research.png", import.meta.url).href;
const securityReviewAvatar = new URL("../../../assets/experts/security-review.png", import.meta.url).href;
const officeAutomationSceneBackground = new URL("../../../assets/experts/scenes/office-automation.webp", import.meta.url).href;
const dataAnalysisSceneBackground = new URL("../../../assets/experts/scenes/data-analysis.webp", import.meta.url).href;
const customerOperationsSceneBackground = new URL("../../../assets/experts/scenes/customer-operations.webp", import.meta.url).href;
const codeDeliverySceneBackground = new URL("../../../assets/experts/scenes/code-delivery.webp", import.meta.url).href;

const AVATAR_BY_EXPERT_NAME: Record<string, string> = {
  架构顾问: architectAvatar,
  代码审查专家: codeReviewAvatar,
  安全审查专家: securityReviewAvatar,
  自动化流程专家: automationFlowAvatar,
  数据分析报告师: dataReportAvatar,
  文档报告专家: docReportAvatar,
  探索专家: researchAvatar,
  运营增长顾问: opsGrowthAvatar,
  测试策略专家: codeReviewAvatar,
  发布风险顾问: securityReviewAvatar,
  流程编排专家: automationFlowAvatar,
  表格建模专家: dataReportAvatar,
  文档自动化专家: docReportAvatar,
  通知集成专家: automationFlowAvatar,
  指标体系专家: dataReportAvatar,
  数据质量专家: dataReportAvatar,
  分析建模专家: researchAvatar,
  报告叙事专家: docReportAvatar,
  客户洞察专家: opsGrowthAvatar,
  服务流程专家: automationFlowAvatar,
  复盘报告专家: docReportAvatar,
  项目拆解专家: architectAvatar,
  资源排期专家: automationFlowAvatar,
  风险控制顾问: securityReviewAvatar,
  交付报告专家: docReportAvatar,
  主理专家: architectAvatar,
  执行专家: automationFlowAvatar,
  质量专家: codeReviewAvatar,
  会计专家: dataReportAvatar,
  审计专家: researchAvatar,
  税务专家: securityReviewAvatar,
  经营决策专家: docReportAvatar,
  架构评审专家: architectAvatar,
  产品规划专家: opsGrowthAvatar,
  会计核算专家: dataReportAvatar,
  审计风控专家: researchAvatar,
  税务合规专家: securityReviewAvatar,
  成本管控专家: dataReportAvatar,
  经营分析顾问: docReportAvatar,
  季度经营决策专家团: docReportAvatar,
  "Architecture Review": architectAvatar,
  "Testing Expert": codeReviewAvatar,
  "Product Expert": opsGrowthAvatar,
  "Code Review Expert": codeReviewAvatar,
  "Documentation Expert": docReportAvatar,
};

export type ExpertMode = "expert" | "team";
type ExpertView = "catalog" | "mine";
type ExpertLibraryFilter = "all" | "created" | "favorite" | "recent";
type ExpertSort = "hot" | "latest";

export type ExpertSelection = {
  id: string;
  name: string;
  role: string;
  mode: ExpertMode;
  prompt: string;
  systemPrompt?: string;
  avatarKey?: string;
  tone?: ExpertItem["tone"];
  avatarSrc?: string;
  members?: ExpertMember[];
  selectedAt?: number;
};

export type ExpertMember = {
  kind?: string;
  expertId?: string;
  name: string;
  role: string;
  task: string;
  output: string;
  tone: ExpertItem["tone"];
  avatarKey?: string;
  avatarSrc?: string;
};

type ExpertItem = {
  id: string;
  mode: ExpertMode;
  name: string;
  role: string;
  description: string;
  category?: string;
  categories: string[];
  tags: string[];
  icon: LucideIcon;
  tone: "amber" | "blue" | "green" | "violet" | "orange" | "teal" | "stone";
  avatarKey?: string;
  avatarSrc?: string;
  members?: ExpertMember[];
  prompts: string[];
  source?: "official" | "user";
  systemPrompt?: string;
  createdAt?: number;
};

type ExpertScene = {
  id: string;
  title: string;
  category: string;
  categoryAliases: string[];
  featuredItemIds: string[][];
  backgroundSrc: string;
  backgroundPosition: string;
};

type ExpertDirectoryProps = {
  onSummon: (selection: ExpertSelection) => void;
  authenticated: boolean;
  workspaceNavigation?: ReactNode;
  workspaceHeaderActionsId?: string;
  backendExperts?: ExpertInfo[];
  userExperts?: UserExpertInfo[];
  expertLibraryState?: ExpertLibraryState;
  onSaveUserExpert?: (input: UserExpertInput, id?: string) => Promise<UserExpertInfo>;
  onSaveCatalogExpert?: (id: string, input: ExpertInfo) => Promise<ExpertInfo>;
  onDuplicateExpert?: (id: string) => Promise<UserExpertInfo>;
  onToggleFavorite?: (id: string, favorite: boolean) => Promise<ExpertLibraryState>;
  onImportExpertConfig?: (payload: string) => Promise<UserExpertInput>;
  onDeleteUserExpert?: (id: string) => Promise<void>;
  onDeleteCatalogExpert?: (id: string) => Promise<void>;
};

type ExpertDraft = {
  name: string;
  role: string;
  mode: ExpertMode;
  description: string;
  tags: string[];
  tagInput: string;
  avatarKey: string;
  avatarFileName: string;
  members: ExpertMember[];
  systemPrompt: string;
  prompts: string[];
  importPayload: string;
};

const CATEGORIES = [
  "全部",
  "技术工程",
  "办公自动化",
  "数据分析",
  "表格处理",
  "报告生成",
  "项目管理",
  "运营增长",
  "客户服务",
  "财务分析",
];

const MIN_EXPERT_PROMPTS = 3;
const MAX_EXPERT_PROMPTS = 5;
const MAX_EXPERT_AVATAR_BYTES = 5 * 1024 * 1024;
const MAX_EXPERT_TAGS = 3;
const EXPERT_AVATAR_TYPES = new Set(["image/png", "image/jpeg", "image/webp"]);
const USER_EXPERT_CATEGORY_SUGGESTIONS = ["办公自动化", "数据分析", "客户运营", "代码交付"] as const;

function formatExpertUsageCount(count: number): string {
  const normalized = Math.max(0, Math.floor(count));
  if (normalized < 10_000) return String(normalized);
  return `${(normalized / 10_000).toFixed(2).replace(/\.00$/, "")}万`;
}

const emptyExpertDraft = (): ExpertDraft => ({
  name: "",
  role: "",
  mode: "expert",
  description: "",
  tags: [],
  tagInput: "",
  avatarKey: "",
  avatarFileName: "",
  members: [],
  systemPrompt: "",
  prompts: [],
  importPayload: "",
});

const defaultDraftMembers = (): ExpertMember[] => [];

const officialExpertItems = (items: Array<Omit<ExpertItem, "source">>): ExpertItem[] =>
  items.map((item) => ({ ...item, source: "official" }));

const EXPERT_ITEMS = officialExpertItems([
  {
    id: "architect",
    mode: "expert",
    name: "架构顾问",
    role: "技术工程",
    description: "拆解复杂系统设计，给出模块边界、接口关系和演进路径。",
    categories: ["技术工程", "项目管理"],
    tags: ["架构设计", "技术方案", "边界梳理"],
    icon: Code2,
    tone: "blue",
    avatarSrc: architectAvatar,
    prompts: [
      "请帮我拆解这个功能的技术方案，说明模块边界、核心流程和风险点。",
      "请评估当前架构是否适合继续扩展，并给出最小改动建议。",
      "请把这个需求拆成前端、后端和数据三部分的交付清单。",
    ],
  },
  {
    id: "code-review",
    mode: "expert",
    name: "代码审查专家",
    role: "代码质量",
    description: "检查变更中的正确性风险、可维护性问题和必要测试。",
    categories: ["技术工程"],
    tags: ["Bug风险", "代码质量", "回归点"],
    icon: FileCheck2,
    tone: "violet",
    avatarSrc: codeReviewAvatar,
    prompts: [
      "请从代码审查角度检查这次改动，优先指出可能导致线上问题的风险。",
      "请帮我找出这个实现里最值得先修的三个问题。",
      "请评估这段代码是否需要补测试，并说明测试重点。",
    ],
  },
  {
    id: "security-review",
    mode: "expert",
    name: "安全审查专家",
    role: "安全质量",
    description: "聚焦注入、权限、密钥、外部输入和敏感操作风险。",
    categories: ["技术工程"],
    tags: ["权限", "注入", "密钥"],
    icon: ShieldCheck,
    tone: "green",
    avatarSrc: securityReviewAvatar,
    prompts: [
      "请帮我检查这个方案里的安全风险，重点看权限、输入和密钥处理。",
      "请评估这个接口是否存在越权或注入风险。",
      "请给我一份上线前安全检查清单。",
    ],
  },
  {
    id: "automation-flow",
    mode: "expert",
    name: "自动化流程专家",
    role: "办公自动化",
    description: "把重复任务整理成可执行步骤，适合定时、批量和跨工具流程。",
    categories: ["办公自动化", "项目管理"],
    tags: ["流程", "定时", "执行"],
    icon: Workflow,
    tone: "teal",
    avatarSrc: automationFlowAvatar,
    prompts: [
      "请把这个重复任务设计成自动化流程，列出触发条件、步骤和异常处理。",
      "请帮我把这件事拆成可以每天自动执行的任务清单。",
      "请设计一个适合团队复用的办公自动化流程。",
    ],
  },
  {
    id: "data-report",
    mode: "expert",
    name: "数据分析报告师",
    role: "数据智能",
    description: "把数据和日志整理成指标诊断、趋势判断和决策报告。",
    categories: ["数据分析", "报告生成", "财务分析"],
    tags: ["指标", "诊断", "报告"],
    icon: BarChart3,
    tone: "amber",
    avatarSrc: dataReportAvatar,
    prompts: [
      "请帮我设计一套数据分析报告框架，包含核心指标、诊断路径和结论结构。",
      "请把这些数据整理成一份面向管理层的分析报告。",
      "请帮我分析指标变化原因，并给出可执行建议。",
    ],
  },
  {
    id: "doc-report",
    mode: "expert",
    name: "文档报告专家",
    role: "报告生成",
    description: "把讨论、代码和资料整理成结构清晰的交付说明。",
    categories: ["报告生成", "办公自动化"],
    tags: ["文档", "摘要", "交付"],
    icon: FileText,
    tone: "stone",
    avatarSrc: docReportAvatar,
    prompts: [
      "请把当前内容整理成一份结构清晰的交付说明。",
      "请帮我生成一份项目进展报告，突出结论、风险和下一步。",
      "请把这些零散信息整理成可读性强的文档。",
    ],
  },
  {
    id: "research",
    mode: "expert",
    name: "探索专家",
    role: "深度研究",
    description: "快速阅读代码库和资料，整理关键路径、证据和结论摘要。",
    categories: ["技术工程", "数据分析"],
    tags: ["代码探索", "调用链", "结论摘要"],
    icon: Compass,
    tone: "violet",
    avatarSrc: researchAvatar,
    prompts: [
      "请帮我快速探索这个代码库，找出和当前需求最相关的文件和调用链。",
      "请先调研再给我结论，列出关键证据和不确定点。",
      "请帮我梳理这个模块的职责和关键路径。",
    ],
  },
  {
    id: "ops-growth",
    mode: "expert",
    name: "运营增长顾问",
    role: "运营增长",
    description: "围绕增长目标拆解人群、触点、内容和转化动作。",
    categories: ["运营增长", "客户服务", "数据分析"],
    tags: ["增长", "转化", "复盘"],
    icon: BriefcaseBusiness,
    tone: "orange",
    avatarSrc: opsGrowthAvatar,
    prompts: [
      "请帮我设计一个运营增长方案，包含目标人群、触点和复盘指标。",
      "请分析这次活动转化偏低的可能原因，并给出优化动作。",
      "请帮我把客户反馈整理成可执行的运营改进计划。",
    ],
  },
  {
    id: "code-delivery-team",
    mode: "team",
    name: "代码交付专家团",
    role: "技术工程",
    description: "围绕功能交付协同分析架构、代码质量、测试覆盖和上线风险。",
    categories: ["技术工程", "项目管理"],
    tags: ["方案", "质量", "测试"],
    icon: Users,
    tone: "blue",
    avatarSrc: codeDeliveryTeamAvatar,
    members: [
      { name: "架构顾问", role: "系统设计", task: "拆解模块边界、数据流和接口契约，明确最小可交付架构。", output: "技术方案与依赖关系", tone: "blue" },
      { name: "代码审查专家", role: "实现质量", task: "检查关键路径、异常分支、可维护性和可能引入的回归点。", output: "风险清单与修复优先级", tone: "violet" },
      { name: "测试策略专家", role: "验证覆盖", task: "设计单测、集成验证和手工验收路径，保证改动可验证。", output: "测试矩阵与验收步骤", tone: "green" },
      { name: "发布风险顾问", role: "上线控制", task: "评估配置、兼容性、回滚方案和上线观察指标。", output: "发布检查表", tone: "stone" },
    ],
    prompts: [
      "请帮我拆解这个功能的交付方案，包括架构、代码质量、测试和风险点。",
      "请从专家团视角评估这个实现能不能进入开发。",
      "请把这个需求拆成可交付的开发任务，并标出优先级。",
    ],
  },
  {
    id: "office-automation-team",
    mode: "team",
    name: "办公自动化专家团",
    role: "办公自动化",
    description: "把表格、文档、消息和定时任务串成可复用的办公流程。",
    categories: ["办公自动化", "表格处理", "报告生成"],
    tags: ["表格", "文档", "流程"],
    icon: Bot,
    tone: "teal",
    avatarSrc: officeAutomationTeamAvatar,
    members: [
      { name: "流程编排专家", role: "任务流设计", task: "定义触发条件、执行步骤、人工确认点和失败重试策略。", output: "自动化流程图", tone: "teal" },
      { name: "表格建模专家", role: "表格处理", task: "整理字段、公式、校验规则和跨表引用关系。", output: "表格模板与字段说明", tone: "amber" },
      { name: "文档自动化专家", role: "报告生成", task: "把数据结果转成固定模板，生成日报、周报和交付说明。", output: "报告模板结构", tone: "stone" },
      { name: "通知集成专家", role: "消息联动", task: "设计消息提醒、审批节点和团队协作通知。", output: "通知规则与异常提醒", tone: "blue" },
    ],
    prompts: [
      "请把这个办公任务设计成自动化流程，包含表格处理、文档生成和消息提醒。",
      "请帮我把日报流程改造成半自动化方案。",
      "请设计一个从数据收集到报告输出的办公自动化流程。",
    ],
  },
  {
    id: "data-analysis-team",
    mode: "team",
    name: "数据分析专家团",
    role: "数据分析",
    description: "从指标定义、数据清洗、诊断分析到报告输出形成闭环。",
    categories: ["数据分析", "表格处理", "财务分析", "报告生成"],
    tags: ["指标", "清洗", "洞察"],
    icon: BarChart3,
    tone: "amber",
    avatarSrc: dataAnalysisTeamAvatar,
    members: [
      { name: "指标体系专家", role: "指标口径", task: "定义核心指标、派生指标、维度拆分和口径边界。", output: "指标字典", tone: "amber" },
      { name: "数据质量专家", role: "数据校验", task: "检查缺失、异常、重复、分布偏移和口径冲突。", output: "数据质量报告", tone: "green" },
      { name: "分析建模专家", role: "原因诊断", task: "拆解趋势、分群、漏斗和贡献度，定位变化原因。", output: "诊断路径与证据", tone: "violet" },
      { name: "报告叙事专家", role: "结论表达", task: "把分析结果整理成面向业务决策的结论和行动建议。", output: "决策报告大纲", tone: "stone" },
    ],
    prompts: [
      "请用专家团方式帮我完成数据分析，先定义指标，再诊断原因，最后输出报告。",
      "请帮我检查这批数据能不能支撑结论，并指出质量问题。",
      "请把这些表格整理成一份业务分析报告。",
    ],
  },
  {
    id: "customer-ops-team",
    mode: "team",
    name: "客户运营专家团",
    role: "客户服务",
    description: "把客户问题、服务流程和增长动作整理成可执行方案。",
    categories: ["客户服务", "运营增长", "报告生成"],
    tags: ["客户", "服务", "复盘"],
    icon: Users,
    tone: "orange",
    avatarSrc: customerOpsTeamAvatar,
    members: [
      { name: "客户洞察专家", role: "需求归因", task: "归纳客户诉求、高频问题、情绪变化和关键阻塞点。", output: "客户问题分层", tone: "orange" },
      { name: "服务流程专家", role: "响应效率", task: "梳理响应路径、升级规则、交付标准和 SLA 风险。", output: "服务流程优化表", tone: "teal" },
      { name: "运营增长顾问", role: "转化留存", task: "设计触达策略、转化动作、留存路径和复盘指标。", output: "运营动作清单", tone: "amber" },
      { name: "复盘报告专家", role: "管理汇报", task: "沉淀客户问题、改进动作、责任人和下一轮跟踪计划。", output: "复盘报告", tone: "stone" },
    ],
    prompts: [
      "请帮我把客户反馈整理成服务改进和运营增长方案。",
      "请分析这些客户问题背后的流程短板。",
      "请给我一个客户服务复盘报告框架。",
    ],
  },
  {
    id: "project-delivery-team",
    mode: "team",
    name: "项目交付专家团",
    role: "项目管理",
    description: "围绕目标、排期、资源、风险和交付物推进项目落地。",
    categories: ["项目管理", "报告生成", "办公自动化"],
    tags: ["计划", "风险", "交付"],
    icon: FlaskConical,
    tone: "green",
    avatarSrc: projectDeliveryTeamAvatar,
    members: [
      { name: "项目拆解专家", role: "范围拆分", task: "拆分目标、里程碑、任务颗粒度和关键依赖关系。", output: "WBS 与里程碑", tone: "green" },
      { name: "资源排期专家", role: "计划管理", task: "评估人力、节奏、并行度和关键路径上的排期风险。", output: "排期与资源表", tone: "blue" },
      { name: "风险控制顾问", role: "风险治理", task: "识别延期、质量、沟通和外部依赖风险，并给出缓解动作。", output: "风险台账", tone: "orange" },
      { name: "交付报告专家", role: "进展汇报", task: "整理项目状态、阻塞事项、决策请求和下一步计划。", output: "周报与汇报材料", tone: "stone" },
    ],
    prompts: [
      "请帮我制定这个项目的交付计划，包含里程碑、风险和每周检查点。",
      "请从专家团角度评估这个项目计划是否可执行。",
      "请把当前项目状态整理成一份周报。",
    ],
  },
]);

const SCENES: ExpertScene[] = [
  {
    id: "automation",
    title: "办公自动化",
    category: "办公自动化",
    categoryAliases: [],
    featuredItemIds: [
      ["exp_workflow_orchestration", "automation-flow"],
      ["exp_document_automation", "doc-report"],
      ["exg_office_automation_team", "office-automation-team"],
    ],
    backgroundSrc: officeAutomationSceneBackground,
    backgroundPosition: "center 42%",
  },
  {
    id: "analytics",
    title: "数据分析",
    category: "数据分析",
    categoryAliases: [],
    featuredItemIds: [
      ["exp_metrics_system", "data-report"],
      ["exp_analysis_modeling", "research"],
      ["exg_data_analysis_team", "data-analysis-team"],
    ],
    backgroundSrc: dataAnalysisSceneBackground,
    backgroundPosition: "center 44%",
  },
  {
    id: "service",
    title: "客户运营",
    category: "客户运营",
    categoryAliases: ["客户服务"],
    featuredItemIds: [
      ["exp_customer_insight"],
      ["exp_ops_growth", "ops-growth"],
      ["exg_customer_ops_team", "customer-ops-team"],
    ],
    backgroundSrc: customerOperationsSceneBackground,
    backgroundPosition: "60% 42%",
  },
  {
    id: "engineering",
    title: "代码交付",
    category: "代码交付",
    categoryAliases: ["技术工程"],
    featuredItemIds: [
      ["exp_architect", "architect"],
      ["exp_code_review", "code-review"],
      ["exg_code_delivery_team", "code-delivery-team"],
    ],
    backgroundSrc: codeDeliverySceneBackground,
    backgroundPosition: "center 42%",
  },
];

const backendToneByIndex: ExpertItem["tone"][] = ["blue", "violet", "green", "teal", "amber"];

function expertSource(source?: string): ExpertItem["source"] {
  if (source === "official" || source === "user") return source;
  return undefined;
}

function cleanedPrompts(prompts: string[] | undefined, fallback: string): string[] {
  const values = (prompts ?? []).map((prompt) => prompt.trim()).filter(Boolean).slice(0, MAX_EXPERT_PROMPTS);
  for (const prompt of [
    fallback,
    "请分析这个任务的关键风险。",
    "请给出这个任务可执行的下一步计划。",
    "请整理这个任务的目标、约束和验收标准。",
  ]) {
    if (values.length >= MIN_EXPERT_PROMPTS) break;
    if (!values.includes(prompt)) values.push(prompt);
  }
  return values;
}

function draftPromptValues(prompts: string[] | undefined): string[] {
  return (prompts ?? []).slice(0, MAX_EXPERT_PROMPTS);
}

function uniqueExpertItems(items: ExpertItem[]): ExpertItem[] {
  const seen = new Set<string>();
  return items.filter((item) => {
    if (seen.has(item.id)) return false;
    seen.add(item.id);
    return true;
  });
}

function sceneCategories(scene: ExpertScene): string[] {
  return [scene.category, ...scene.categoryAliases];
}

function featuredSceneItems(itemsById: ReadonlyMap<string, ExpertItem>, scene: ExpertScene): ExpertItem[] {
  return scene.featuredItemIds.flatMap((candidateIds) => {
    const item = candidateIds.map((id) => itemsById.get(id)).find((candidate) => candidate !== undefined);
    return item ? [item] : [];
  });
}

function normalizeExpertTone(value: string | undefined, fallback: ExpertItem["tone"]): ExpertItem["tone"] {
  switch (value) {
    case "amber":
    case "blue":
    case "green":
    case "violet":
    case "orange":
    case "teal":
    case "stone":
      return value;
    default:
      return fallback;
  }
}

function uploadedAvatarSrc(value?: string): string | undefined {
  const key = value?.trim();
  return key?.startsWith("data:image/") ? key : undefined;
}

function avatarSrcForExpertInfo(info: Pick<ExpertInfo, "id" | "avatarKey" | "avatarUrl" | "name">): string | undefined {
  return uploadedAvatarSrc(info.avatarKey) || expertAvatarSrcForInfo(info);
}

function isCloudUserAssetID(id: string): boolean {
  return id.startsWith("exp_") || id.startsWith("exg_");
}

function expertMemberWithAvatar(member: { kind?: string; expertId?: string; name: string; role: string; task: string; output: string; tone?: string; avatarKey?: string; avatarUrl?: string; avatarSrc?: string }): ExpertMember {
  return {
    kind: member.kind,
    expertId: member.expertId,
    name: member.name,
    role: member.role,
    task: member.task,
    output: member.output,
    tone: (member.tone as ExpertItem["tone"]) || "stone",
    avatarKey: member.avatarKey,
    avatarSrc: member.avatarSrc || uploadedAvatarSrc(member.avatarKey) || expertMemberAvatarSrc(member),
  };
}

function expertMembersWithAvatars(members?: { kind?: string; expertId?: string; name: string; role: string; task: string; output: string; tone?: string; avatarKey?: string; avatarUrl?: string; avatarSrc?: string }[]): ExpertMember[] | undefined {
  return members?.map(expertMemberWithAvatar);
}

function backendExpertIcon(info: ExpertInfo): LucideIcon {
  const key = `${info.id} ${info.category ?? ""}`.toLowerCase();
  if (key.includes("test")) return FlaskConical;
  if (key.includes("review")) return FileCheck2;
  if (key.includes("product")) return BriefcaseBusiness;
  if (key.includes("doc")) return FileText;
  return Code2;
}

function backendExpertItem(info: ExpertInfo, index: number): ExpertItem {
  const category = info.category?.trim() || "builtin";
  const language = info.language?.trim();
  return {
    id: info.id,
    mode: "expert",
    name: info.name,
    role: category,
    description: info.description,
    category,
    categories: [category],
    tags: [category, language].filter(Boolean) as string[],
    icon: backendExpertIcon(info),
    tone: backendToneByIndex[index % backendToneByIndex.length],
    prompts: [`请以 ${info.name} 的视角处理当前任务。`],
    source: expertSource(info.source),
  };
}

function backendExpertItemWithRuntimeFields(
  info: ExpertInfo,
  index: number,
  avatarKeyByName?: Map<string, string>,
  relatedCategories: string[] = [],
): ExpertItem {
  const fallback = backendExpertItem(info, index);
  const mode: ExpertMode = info.mode === "team" ? "team" : "expert";
  const category = info.category?.trim() || info.role?.trim() || info.categories?.find((item) => item.trim())?.trim() || (mode === "team" ? "team" : "builtin");
  const language = info.language?.trim();
  const tags = (info.tags ?? []).map((tag) => tag.trim()).filter(Boolean);
  const prompts = cleanedPrompts(info.prompts, fallback.prompts[0]);
  const members = (info.members ?? [])
    .map((member) => {
      const avatarKey = member.avatarKey?.trim() || avatarKeyByName?.get(member.name.trim());
      return {
        kind: member.kind,
        expertId: member.expertId,
        name: member.name,
        role: member.role,
        task: member.task,
        output: member.output,
        tone: (member.tone as ExpertItem["tone"]) || "stone",
        avatarKey,
        avatarSrc: expertMemberAvatarSrc({ ...member, avatarKey }),
      };
    })
    .filter((member) => member.name || member.role || member.task || member.output);
  return {
    id: info.id,
    mode,
    name: info.name,
    role: info.role?.trim() || category,
    description: info.description,
    category,
    categories: Array.from(new Set([
      category,
      ...(info.categories ?? []).map((item) => item.trim()).filter(Boolean),
      ...relatedCategories,
    ])),
    tags: (tags.length ? tags : [category, language].filter(Boolean) as string[]).slice(0, MAX_EXPERT_TAGS),
    icon: mode === "team" ? Users : fallback.icon,
    tone: normalizeExpertTone(info.tone, fallback.tone),
    avatarKey: info.avatarKey,
    avatarSrc: avatarSrcForExpertInfo(info) || fallback.avatarSrc,
    members,
    prompts,
    source: expertSource(info.source),
    createdAt: info.createdAt,
  };
}

function userExpertItem(info: UserExpertInfo, index: number): ExpertItem {
  const mode: ExpertMode = info.mode === "team" ? "team" : "expert";
  const category = info.category?.trim() || (mode === "team" ? "team" : "custom");
  const tags = (info.tags ?? []).map((tag) => tag.trim()).filter(Boolean);
  return {
    id: info.id,
    mode,
    name: info.name,
    role: category,
    description: info.description,
    category,
    categories: [category, ...tags],
    tags: tags.length ? tags.slice(0, MAX_EXPERT_TAGS) : [category],
    icon: mode === "team" ? Users : Compass,
    tone: mode === "team" ? "teal" : backendToneByIndex[index % backendToneByIndex.length],
    avatarKey: info.avatarKey,
    avatarSrc: avatarSrcForExpertInfo(info),
    members: expertMembersWithAvatars(info.members),
    prompts: cleanedPrompts(info.prompts, info.displayPrompt?.trim() || `请以${info.name}的身份处理当前任务。`),
    source: "user",
    systemPrompt: info.systemPrompt,
    createdAt: info.createdAt,
  };
}

function mergeBackendExperts(backendExperts: ExpertInfo[] | undefined, hiddenIds?: Set<string>): ExpertItem[] {
  if (backendExperts === undefined) return EXPERT_ITEMS;
  if (backendExperts.length === 0) return [];
  const staticPresentationById = new Map(EXPERT_ITEMS.map((item) => [item.id, item]));
  const items = new Map<string, ExpertItem>();
  const avatarKeyByName = new Map<string, string>();
  const relatedCategoriesById = new Map<string, string[]>();
  const addRelatedCategory = (id: string | undefined, category: string) => {
    const normalizedID = id?.trim();
    if (!normalizedID || !category) return;
    const categories = relatedCategoriesById.get(normalizedID) ?? [];
    if (!categories.includes(category)) categories.push(category);
    relatedCategoriesById.set(normalizedID, categories);
  };
  backendExperts.forEach((info) => {
    const name = info.name.trim();
    const avatarKey = info.avatarKey?.trim();
    if (name && avatarKey) avatarKeyByName.set(name, avatarKey);
    if (info.source !== "official" || info.mode !== "team") return;
    const domainCategory = info.tags?.map((tag) => tag.trim()).find(Boolean) ?? "";
    addRelatedCategory(info.id, domainCategory);
    info.members?.forEach((member) => addRelatedCategory(member.expertId, domainCategory));
  });
  backendExperts.filter((info) => info.enabled !== false).forEach((info, index) => {
    if (hiddenIds?.has(info.id)) return;
    const staticPresentation = staticPresentationById.get(info.id);
    const next = backendExpertItemWithRuntimeFields(info, index, avatarKeyByName, relatedCategoriesById.get(info.id));
    items.set(info.id, {
      ...next,
      ...(staticPresentation ? {
        icon: staticPresentation.icon,
        tone: staticPresentation.tone,
        avatarSrc: next.avatarSrc || staticPresentation.avatarSrc,
      } : null),
      mode: next.mode,
      name: info.name,
      role: info.role?.trim() || info.category?.trim() || next.role,
      description: info.description,
      categories: next.categories,
      tags: next.tags,
      members: expertMembersWithAvatars(next.members),
      prompts: next.prompts,
    });
  });
  return Array.from(items.values());
}

function ExpertAvatar({
  tone,
  team = false,
  size = "md",
  icon: Icon,
  variant,
  src,
}: {
  tone: ExpertItem["tone"];
  team?: boolean;
  size?: "sm" | "md" | "lg";
  icon?: LucideIcon;
  variant?: string;
  src?: string;
}) {
  const variantClass = variant ? ` expert-avatar--${variant}` : "";
  const photoClass = src ? " expert-avatar--photo" : "";

  return (
    <span className={`expert-avatar expert-avatar--${tone} expert-avatar--${size}${team ? " expert-avatar--team" : ""}${variantClass}${photoClass}`} aria-hidden="true">
      {src ? (
        <img className="expert-avatar__image" src={src} alt="" loading="lazy" decoding="async" />
      ) : (
        <>
          <span className="expert-avatar__panel">
            <span />
            <span />
            <span />
          </span>
          {team && (
            <>
              <span className="expert-avatar__mini expert-avatar__mini--left">
                <span className="expert-avatar__mini-face" />
                <span className="expert-avatar__mini-body" />
              </span>
              <span className="expert-avatar__mini expert-avatar__mini--right">
                <span className="expert-avatar__mini-face" />
                <span className="expert-avatar__mini-body" />
              </span>
            </>
          )}
          <span className="expert-avatar__person">
            <span className="expert-avatar__hair" />
            <span className="expert-avatar__face" />
            <span className="expert-avatar__body" />
          </span>
        </>
      )}
      {!src && Icon && (
        <span className="expert-avatar__tool">
          <Icon />
        </span>
      )}
    </span>
  );
}

function tagsFromDraft(tags: string[], pendingTag = ""): string[] {
  return [...tags, pendingTag]
    .map((tag) => tag.trim())
    .filter((tag, index, values) => Boolean(tag) && values.indexOf(tag) === index)
    .slice(0, MAX_EXPERT_TAGS);
}

function draftMembersForSave(mode: ExpertMode, members: ExpertMember[]): ExpertMember[] {
  if (mode !== "team") return [];
  return members.map((member, index) => {
    const kind = index === 0 ? "lead" : member.kind === "lead" ? "sub" : member.kind || "sub";
    const toneFallback: ExpertItem["tone"] = index === 0 ? "blue" : "stone";
    return {
      kind,
      expertId: member.expertId?.trim() || undefined,
      name: member.name.trim(),
      role: member.role.trim(),
      task: member.task.trim(),
      output: member.output.trim(),
      tone: normalizeExpertTone(member.tone, toneFallback),
      avatarKey: member.avatarKey?.trim() || undefined,
      avatarSrc: member.avatarSrc || uploadedAvatarSrc(member.avatarKey),
    };
  }).filter((member) => member.expertId || member.name || member.role || member.task || member.output || member.avatarKey);
}

export function ExpertDirectory({
  onSummon,
  authenticated,
  workspaceNavigation,
  workspaceHeaderActionsId,
  backendExperts,
  userExperts,
  expertLibraryState,
  onSaveUserExpert,
  onSaveCatalogExpert,
  onDuplicateExpert,
  onImportExpertConfig,
  onDeleteUserExpert,
  onDeleteCatalogExpert,
}: ExpertDirectoryProps) {
  const t = useT();
  const [view, setView] = useState<ExpertView>("catalog");
  const [mode, setMode] = useState<ExpertMode>("expert");
  const [sort, setSort] = useState<ExpertSort>("hot");
  const [category, setCategory] = useState("全部");
  const [query, setQuery] = useState("");
  const [workspaceHeaderActionsHost, setWorkspaceHeaderActionsHost] = useState<HTMLElement | null>(null);
  const categoryScrollerRef = useHorizontalDragScroll(view);

  useLayoutEffect(() => {
    setWorkspaceHeaderActionsHost(workspaceHeaderActionsId ? document.getElementById(workspaceHeaderActionsId) : null);
  }, [workspaceHeaderActionsId]);
  useEffect(() => {
    if (!authenticated) setView("catalog");
  }, [authenticated]);
  const [mineFilter, setMineFilter] = useState<ExpertLibraryFilter>("all");
  const [mineCategory, setMineCategory] = useState<"all" | "expert" | "team" | string>("all");
  const [customExperts, setCustomExperts] = useState<ExpertItem[]>([]);
  const [selected, setSelected] = useState<ExpertItem | null>(null);
  const [draftOpen, setDraftOpen] = useState(false);
  const [draftMode, setDraftMode] = useState<"create" | "edit" | "import">("create");
  const [editingId, setEditingId] = useState("");
  const [draft, setDraft] = useState<ExpertDraft>(() => emptyExpertDraft());
  const [draftAvatarError, setDraftAvatarError] = useState("");
  const [draftSavePending, setDraftSavePending] = useState(false);
  const draftSavePendingRef = useRef(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState("");
  const [runtimeBackendExperts, setRuntimeBackendExperts] = useState<ExpertInfo[]>([]);
  const usesRuntimeBackendExperts = backendExperts === undefined;
  const effectiveBackendExperts = usesRuntimeBackendExperts ? runtimeBackendExperts : backendExperts;
  const refreshRuntimeBackendExperts = async () => {
    if (!usesRuntimeBackendExperts) return;
    try {
      const experts = await app.ListExperts();
      setRuntimeBackendExperts(Array.isArray(experts) ? experts : []);
    } catch {
      setRuntimeBackendExperts([]);
    }
  };
  useEffect(() => {
    void refreshRuntimeBackendExperts();
  }, [usesRuntimeBackendExperts]);
  const userExpertIds = useMemo(() => new Set((userExperts ?? []).map((item) => item.id)), [userExperts]);
  const favoriteExpertIds = useMemo(() => new Set(expertLibraryState?.favoriteIds ?? []), [expertLibraryState]);
  const recentExpertIds = useMemo(() => new Set(expertLibraryState?.recentIds ?? []), [expertLibraryState]);
  const usageCounts = useMemo(() => expertLibraryState?.usageCounts ?? {}, [expertLibraryState?.usageCounts]);
  const userItems = useMemo(() => (userExperts ?? []).filter((item) => item.enabled).map(userExpertItem), [userExperts]);
  const catalogItems = useMemo(() => mergeBackendExperts(effectiveBackendExperts, userExpertIds), [effectiveBackendExperts, userExpertIds]);
  const itemById = useMemo(() => new Map(catalogItems.map((item) => [item.id, item])), [catalogItems]);
  const sceneItems = useMemo(() => {
    const officialItemsById = new Map(catalogItems
      .filter((item) => item.source === "official")
      .map((item) => [item.id, item]));
    return new Map(SCENES.map((scene) => [scene.id, featuredSceneItems(officialItemsById, scene)]));
  }, [catalogItems]);
  const memberOptions = useMemo(
    () => uniqueExpertItems([...catalogItems, ...userItems]).filter((item) => item.mode === "expert"),
    [catalogItems, userItems],
  );
  const availableCategories = useMemo(() => {
    const supported = Array.from(new Set(
      catalogItems
        .filter((item) => item.source === "official")
        .flatMap((item) => item.categories)
        .map((item) => item.trim())
        .filter((item) => item && item !== "全部"),
    ));
    const configured = CATEGORIES.filter((item) => item === "全部" || supported.includes(item));
    const backendCategories = supported.filter((item) => !CATEGORIES.includes(item));
    return [...configured, ...backendCategories];
  }, [catalogItems]);
  useEffect(() => {
    if (!availableCategories.includes(category)) setCategory("全部");
  }, [availableCategories, category]);

  const filteredItems = useMemo(() => {
    const q = query.trim().toLowerCase();
    const items = catalogItems.filter((item) => {
      if (item.source !== "official") return false;
      if (item.mode !== mode) return false;
      if (category !== "全部" && !item.categories.includes(category)) return false;
      if (!q) return true;
      return [item.name, item.role, item.description, ...item.tags, ...item.categories].some((value) => value.toLowerCase().includes(q));
    });
    return items.sort((left, right) => {
      if (sort === "latest") {
        return (right.createdAt ?? 0) - (left.createdAt ?? 0);
      }
      return (usageCounts[right.id] ?? 0) - (usageCounts[left.id] ?? 0);
    });
  }, [catalogItems, category, mode, query, sort, usageCounts]);

  const myBaseItems = useMemo(() => {
    if (userExperts !== undefined) {
      return uniqueExpertItems([
        ...userItems,
        ...catalogItems.filter((item) => favoriteExpertIds.has(item.id) || recentExpertIds.has(item.id)),
      ]);
    }
    return uniqueExpertItems([
      ...customExperts,
      ...catalogItems.filter((item) => item.source === "user"),
    ]);
  }, [catalogItems, customExperts, favoriteExpertIds, recentExpertIds, userExperts, userItems]);

  const mineTeamCategories = useMemo(() => Array.from(new Set(
    myBaseItems
      .filter((item) => item.source === "user" && item.mode === "team")
      .map((item) => item.category?.trim())
      .filter((item): item is string => Boolean(item)),
  )), [myBaseItems]);
  useEffect(() => {
    if (
      mineCategory !== "all" &&
      mineCategory !== "expert" &&
      mineCategory !== "team" &&
      !mineTeamCategories.includes(mineCategory)
    ) {
      setMineCategory("all");
    }
  }, [mineCategory, mineTeamCategories]);

  const myItems = useMemo(() => {
    const q = query.trim().toLowerCase();
    return myBaseItems.filter((item) => {
      if (mineFilter === "created" && item.source !== "user") return false;
      if (mineFilter === "favorite" && !favoriteExpertIds.has(item.id)) return false;
      if (mineFilter === "recent" && !recentExpertIds.has(item.id)) return false;
      if (mineCategory === "expert" && item.mode !== "expert") return false;
      if (mineCategory === "team" && item.mode !== "team") return false;
      if (mineCategory !== "all" && mineCategory !== "expert" && mineCategory !== "team") {
        if (item.category !== mineCategory) return false;
      }
      if (!q) return true;
      return [item.name, item.role, item.description, ...item.tags, ...item.categories].some((value) => value.toLowerCase().includes(q));
    });
  }, [favoriteExpertIds, mineCategory, mineFilter, myBaseItems, query, recentExpertIds]);

  const summon = (item: ExpertItem, prompt = item.prompts[0]) => {
    onSummon({
      id: item.id,
      name: item.name,
      role: item.role,
      mode: item.mode,
      prompt,
      systemPrompt: item.systemPrompt,
      avatarKey: item.avatarKey,
      tone: item.tone,
      avatarSrc: item.avatarSrc,
      members: expertMembersWithAvatars(item.members),
    });
  };

  const openCreate = (mode: "create" | "import" = "create") => {
    setDraftMode(mode);
    setDraftAvatarError("");
    setEditingId("");
    setDraft(mode === "import"
      ? {
        ...emptyExpertDraft(),
        name: "导入的专家",
        description: "从外部配置整理出的专家。",
        importPayload: JSON.stringify({
          name: "Imported Expert",
          mode: "expert",
          description: "Describe what this expert handles.",
          category: "custom",
          tags: ["custom"],
          systemPrompt: "Act as this expert for the current session.",
          prompts: [
            "请以这位专家的身份处理当前任务。",
            "请分析当前任务的关键风险。",
            "请把当前任务整理成可执行计划。",
          ],
          displayPrompt: "请以这位专家的身份处理当前任务。",
          language: "auto",
          enabled: true,
        }, null, 2),
      }
      : emptyExpertDraft());
    setDraftOpen(true);
  };

  const openEdit = (item: ExpertItem) => {
    if (item.source !== "user" || isCloudUserAssetID(item.id)) return;
    setDraftMode("edit");
    setDraftAvatarError("");
    setEditingId(item.id);
    setDraft({
      name: item.name,
      role: item.role,
      mode: item.mode,
      description: item.description,
      avatarKey: item.avatarKey ?? "",
      avatarFileName: "",
      members: item.mode === "team" ? (item.members?.map((member) => ({ ...member })) ?? defaultDraftMembers()) : (item.members?.map((member) => ({ ...member })) ?? []),
      tags: item.tags.slice(0, MAX_EXPERT_TAGS),
      tagInput: "",
      systemPrompt: item.systemPrompt ?? "",
      prompts: draftPromptValues(item.prompts),
      importPayload: "",
    });
    setDraftOpen(true);
  };

  const parseImportPayload = async () => {
    if (!onImportExpertConfig) return;
    try {
      const input = await onImportExpertConfig(draft.importPayload);
      const importedMembers = input.members?.map((member) => ({
        kind: member.kind,
        expertId: member.expertId,
        name: member.name,
        role: member.role,
        task: member.task,
        output: member.output,
        tone: (member.tone as ExpertItem["tone"]) || "stone",
        avatarKey: member.avatarKey,
        avatarSrc: uploadedAvatarSrc(member.avatarKey) || expertMemberAvatarSrc(member),
      })) ?? [];
      setDraft((current) => ({
        ...current,
        name: input.name,
        role: input.category ?? "",
        mode: input.mode === "team" ? "team" : "expert",
        description: input.description,
        avatarKey: input.avatarKey ?? "",
        avatarFileName: "",
        members: importedMembers.length ? importedMembers : input.mode === "team" ? defaultDraftMembers() : [],
        tags: (input.tags ?? []).map((tag) => tag.trim()).filter(Boolean).slice(0, MAX_EXPERT_TAGS),
        tagInput: "",
        systemPrompt: input.systemPrompt,
        prompts: draftPromptValues(input.prompts?.length ? input.prompts : [input.displayPrompt ?? ""]),
      }));
    } catch {
      return;
    }
  };

  const handleDraftAvatarChange = (event: ChangeEvent<HTMLInputElement>) => {
    const input = event.currentTarget;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    if (!EXPERT_AVATAR_TYPES.has(file.type)) {
      setDraftAvatarError(t("experts.formAvatarInvalid"));
      return;
    }
    if (file.size > MAX_EXPERT_AVATAR_BYTES) {
      setDraftAvatarError(t("experts.formAvatarTooLarge"));
      return;
    }
    const reader = new FileReader();
    reader.onerror = () => setDraftAvatarError(t("experts.formAvatarInvalid"));
    reader.onload = () => {
      if (typeof reader.result !== "string") {
        setDraftAvatarError(t("experts.formAvatarInvalid"));
        return;
      }
      setDraft((current) => ({
        ...current,
        avatarKey: reader.result as string,
        avatarFileName: file.name,
      }));
      setDraftAvatarError("");
    };
    reader.readAsDataURL(file);
  };

  const clearDraftAvatar = () => {
    setDraft((current) => ({ ...current, avatarKey: "", avatarFileName: "" }));
    setDraftAvatarError("");
  };

  const updateDraftMember = (index: number, patch: Partial<ExpertMember>) => {
    setDraft((current) => ({
      ...current,
      members: current.members.map((member, memberIndex) => (memberIndex === index ? { ...member, ...patch } : member)),
    }));
  };

  const addDraftMember = () => {
    setDraft((current) => ({
      ...current,
      members: [...current.members, { kind: "sub", expertId: "", name: "", role: "", task: "", output: "", tone: "stone" }],
    }));
  };

  const selectDraftMember = (index: number, expertId: string) => {
    const selectedMember = memberOptions.find((item) => item.id === expertId);
    if (!selectedMember) {
      updateDraftMember(index, { expertId: "", name: "", role: "", task: "", output: "", avatarKey: undefined, avatarSrc: undefined });
      return;
    }
    updateDraftMember(index, {
      expertId: selectedMember.id,
      name: selectedMember.name,
      role: selectedMember.role,
      task: selectedMember.description,
      output: selectedMember.prompts[0] || selectedMember.description,
      tone: selectedMember.tone,
      avatarKey: selectedMember.avatarKey,
      avatarSrc: selectedMember.avatarSrc,
    });
  };

  const removeDraftMember = (index: number) => {
    setDraft((current) => ({
      ...current,
      members: current.members.filter((_, memberIndex) => memberIndex !== index),
    }));
  };

  const addDraftTag = () => {
    setDraft((current) => {
      const tag = current.tagInput.trim();
      if (!tag) return current;
      if (current.tags.includes(tag) || current.tags.length >= MAX_EXPERT_TAGS) {
        return { ...current, tagInput: "" };
      }
      return { ...current, tags: [...current.tags, tag], tagInput: "" };
    });
  };

  const removeDraftTag = (index: number) => {
    setDraft((current) => ({
      ...current,
      tags: current.tags.filter((_, tagIndex) => tagIndex !== index),
    }));
  };

  const handleDraftModeChange = (nextMode: ExpertMode) => {
    setDraft((current) => ({
      ...current,
      mode: nextMode,
      members: nextMode === "team" && current.members.length === 0 ? defaultDraftMembers() : current.members,
    }));
  };

  const saveDraftInternal = async () => {
    const name = draft.name.trim() || "未命名专家";
    const tags = tagsFromDraft(draft.tags, draft.tagInput);
    if (draftMode !== "edit" && tags.length !== 0 && tags.length !== MAX_EXPERT_TAGS) return;
    const prompts = cleanedPrompts(draft.prompts, `请以${name}的身份处理当前任务。`);
    const role = draft.role.trim();
    if (!role) return;
    const returnView = view;
    const avatarKey = draft.avatarKey.trim();
    const members = draftMembersForSave(draft.mode, draft.members);
    if (draft.mode === "team" && !draft.members.every((member) =>
      member.expertId?.trim() && member.name.trim() && member.role.trim() && member.task.trim() && member.output.trim()
    )) return;
    if (usesRuntimeBackendExperts) {
      const id = editingId.trim();
      const existingItem = id ? itemById.get(id) : undefined;
      if (existingItem && existingItem.source !== "user") return;
      const categories = Array.from(new Set([
        role,
        ...(existingItem?.categories ?? []),
        ...tags,
      ].map((item) => item.trim()).filter(Boolean)));
      const input: ExpertInfo = {
        id: id || `custom-${Date.now()}`,
        mode: draft.mode,
        name,
        role,
        description: draft.description.trim() || "处理自定义专家任务。",
        category: role,
        categories,
        tags,
        iconKey: draft.mode === "team" ? "users" : "bot",
        tone: draft.mode === "team" ? "teal" : "blue",
        avatarKey: avatarKey || undefined,
        members,
        prompts,
        source: "user",
        enabled: true,
      };
      try {
        if (id && itemById.has(id)) {
          await app.UpdateExpert(id, input);
          setView(returnView);
        } else {
          await app.CreateExpert(input);
          setMineFilter("created");
          setMineCategory("all");
          setView("mine");
        }
        await refreshRuntimeBackendExperts();
      } catch {
        return;
      }
      setDraftOpen(false);
      return;
    }
    const editingItem = editingId ? itemById.get(editingId) : undefined;
    if (editingItem?.source === "user" && !userExpertIds.has(editingId) && onSaveCatalogExpert) {
      const categories = Array.from(new Set([
        role,
        ...editingItem.categories,
        ...tags,
      ].map((item) => item.trim()).filter(Boolean)));
      try {
        await onSaveCatalogExpert(editingId, {
          id: editingId,
          mode: draft.mode,
          name,
          role,
          description: draft.description.trim() || "处理你定义的专属任务。",
          category: role,
          categories,
          tags,
          iconKey: draft.mode === "team" ? "users" : "bot",
          tone: draft.mode === "team" ? "teal" : "blue",
          avatarKey: avatarKey || undefined,
          members: draft.mode === "team" ? members : undefined,
          prompts,
          source: "user",
          enabled: true,
        });
      } catch {
        return;
      }
      setView(returnView);
      setDraftOpen(false);
      return;
    }
    if (onSaveUserExpert) {
      try {
        await onSaveUserExpert({
          name,
          mode: draft.mode,
          description: draft.description.trim() || "处理你定义的专属任务。",
          category: role,
          tags,
          avatarKey: avatarKey || undefined,
          avatarFileName: draft.avatarFileName || undefined,
          members: draft.mode === "team" ? members : undefined,
          systemPrompt: draft.systemPrompt.trim() || `请以${name}的身份作为会话级专家处理当前任务。`,
          prompts,
          displayPrompt: prompts[0],
          language: "auto",
          enabled: true,
        }, userExpertIds.has(editingId) ? editingId : undefined);
      } catch {
        return;
      }
      setMineFilter("created");
      setMineCategory("all");
      setView("mine");
      setDraftOpen(false);
      return;
    }
    const item: ExpertItem = {
      id: editingId.startsWith("custom-") ? editingId : `custom-${Date.now()}`,
      mode: draft.mode,
      name,
      role,
      description: draft.description.trim() || "处理你定义的专属任务，按默认提问方式进入对话。",
      category: role,
      categories: [role, ...tags],
      tags,
      icon: draft.mode === "team" ? Users : Compass,
      tone: draft.mode === "team" ? "teal" : "blue",
      avatarKey: avatarKey || undefined,
      avatarSrc: uploadedAvatarSrc(avatarKey) || avatarSrcForExpertInfo({ id: editingId, avatarKey, name }),
      members: draft.mode === "team" ? members : undefined,
      prompts,
      source: "user",
    };
    setCustomExperts((current) => {
      const withoutCurrent = current.filter((entry) => entry.id !== item.id);
      return [item, ...withoutCurrent];
    });
    setMineFilter("created");
    setMineCategory("all");
    setView("mine");
    setDraftOpen(false);
  };

  const saveDraft = async () => {
    if (draftSavePendingRef.current) return;
    draftSavePendingRef.current = true;
    setDraftSavePending(true);
    try {
      await saveDraftInternal();
    } finally {
      draftSavePendingRef.current = false;
      setDraftSavePending(false);
    }
  };

  const duplicateExpert = async (item: ExpertItem) => {
    if (item.source !== "user") return;
    if (onDuplicateExpert) {
      try {
        await onDuplicateExpert(item.id);
      } catch {
        return;
      }
      setMineFilter("created");
      setView("mine");
      return;
    }
    const next: ExpertItem = {
      ...item,
      id: `custom-${Date.now()}`,
      name: `${item.name} 副本`,
      categories: [...item.categories],
      tags: [...item.tags],
      prompts: [...item.prompts],
      members: item.members?.map((member) => ({ ...member })),
      source: "user",
    };
    setCustomExperts((current) => [next, ...current]);
    setMineFilter("created");
    setView("mine");
  };

  const deleteUserExpert = async (item: ExpertItem) => {
    if (item.source !== "user") return;
    if (deleteConfirmId !== item.id) {
      setDeleteConfirmId(item.id);
      return;
    }
    setDeleteConfirmId("");
    if (usesRuntimeBackendExperts) {
      try {
        await app.DeleteExpert(item.id);
        await refreshRuntimeBackendExperts();
        setMineFilter("all");
        setMineCategory("all");
        if (selected?.id === item.id) setSelected(null);
      } catch {
        return;
      }
      return;
    }
    if (!userExpertIds.has(item.id) && onDeleteCatalogExpert) {
      try {
        await onDeleteCatalogExpert(item.id);
        if (selected?.id === item.id) setSelected(null);
      } catch {
        return;
      }
      return;
    }
    if (!onDeleteUserExpert || item.source !== "user") return;
    try {
      await onDeleteUserExpert(item.id);
      if (selected?.id === item.id) setSelected(null);
    } catch {
      return;
    }
  };

  const renderDeleteExpertControl = (item: ExpertItem) => {
    if (item.source !== "user" || isCloudUserAssetID(item.id)) return null;
    const confirming = deleteConfirmId === item.id;
    return (
      <>
        <button
          type="button"
          data-action="delete"
          className={confirming ? "expert-delete-confirm-button" : undefined}
          onClick={() => void deleteUserExpert(item)}
        >
          <Trash2 size={13} aria-hidden="true" />
          {t("common.delete")}
        </button>
        {confirming && (
          <button type="button" data-action="delete-cancel" onClick={() => setDeleteConfirmId("")}>
            {t("common.cancel")}
          </button>
        )}
      </>
    );
  };

  const draftAvatarSrc = uploadedAvatarSrc(draft.avatarKey) || avatarSrcForExpertInfo({ id: editingId, avatarKey: draft.avatarKey, name: draft.name });
  const draftCategoryValid = Boolean(draft.role.trim());
  const draftTagCount = tagsFromDraft(draft.tags, draft.tagInput).length;
  const draftTagsValid = draftMode === "edit" || draftTagCount === 0 || draftTagCount === MAX_EXPERT_TAGS;
  const draftMembersValid = draft.mode !== "team" ||
    draft.members.every((member) => member.expertId?.trim() && member.name.trim() && member.role.trim() && member.task.trim() && member.output.trim());

  const modeLabel = mode === "expert" ? t("experts.modeExpert") : t("experts.modeTeam");
  const selectScene = (scene: ExpertScene) => {
    const nextCategory = sceneCategories(scene).find((item) => availableCategories.includes(item));
    setCategory(nextCategory ?? "全部");
  };
  const libraryFilterLabel = (filter: ExpertLibraryFilter) => {
    switch (filter) {
      case "created":
        return t("experts.filter.created");
      case "favorite":
        return t("experts.filter.favorite");
      case "recent":
        return t("experts.filter.recent");
      default:
        return t("experts.filter.all");
    }
  };
  const catalogHeaderActions = view === "catalog" ? (
    <div className="expert-page__actions">
      <label className="expert-search">
        <Search size={16} aria-hidden="true" />
        <input
          type="search"
          name="experts-search"
          autoComplete="off"
          aria-label={t("experts.searchPlaceholder")}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("experts.searchPlaceholder")}
        />
      </label>
      {authenticated ? (
        <button
          type="button"
          className="expert-my-button"
          onClick={() => setView("mine")}
        >
          <Users size={15} aria-hidden="true" />
          {t("experts.myExperts")}
        </button>
      ) : null}
    </div>
  ) : null;

  return (
    <div className="expert-page">
      {workspaceHeaderActionsHost && catalogHeaderActions
        ? createPortal(catalogHeaderActions, workspaceHeaderActionsHost)
        : null}
      {!workspaceHeaderActionsId && (workspaceNavigation || catalogHeaderActions) && (
        <header className="expert-page__workspace-bar">
          {workspaceNavigation}
          {catalogHeaderActions}
        </header>
      )}
      <div className="expert-page__content">
        {view === "mine" ? (
          <>
          <header className="expert-page__header expert-page__header--mine">
            <div>
              <button type="button" className="expert-back-button" onClick={() => setView("catalog")}>
                <ArrowLeft size={14} aria-hidden="true" />
                {t("experts.backToCatalog")}
              </button>
              <h1>{t("experts.myTitle")}</h1>
              <p>{t("experts.mySubtitle")}</p>
            </div>
            <div className="expert-page__actions expert-page__actions--mine">
              <label className="expert-search">
                <Search size={16} aria-hidden="true" />
                <input
                  type="search"
                  name="my-experts-search"
                  autoComplete="off"
                  aria-label={t("experts.mySearchPlaceholder")}
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t("experts.mySearchPlaceholder")}
                />
              </label>
              <button type="button" className="expert-my-button" onClick={() => openCreate("import")}>
                <Upload size={15} aria-hidden="true" />
                {t("experts.importConfig")}
              </button>
              <button type="button" className="expert-create-button" onClick={() => openCreate("create")}>
                <Plus size={16} aria-hidden="true" />
                {t("experts.createExpert")}
              </button>
            </div>
          </header>

          <section className="expert-manage-layout" aria-label={t("experts.myExperts")}>
            <div className="expert-manage-main">
              <nav className="expert-catalog-nav expert-catalog-nav--mine" aria-label={t("experts.myFilters")}>
                <div className="expert-catalog-nav__primary">
                  <div className="expert-mode-switch expert-mode-switch--library" role="tablist" aria-label={t("experts.myFilters")}>
                    {(["all", "created", "favorite", "recent"] as ExpertLibraryFilter[]).map((filter) => (
                      <button
                        key={filter}
                        type="button"
                        role="tab"
                        aria-selected={mineFilter === filter}
                        className={mineFilter === filter ? "expert-mode-switch__item expert-mode-switch__item--active" : "expert-mode-switch__item"}
                        onClick={() => setMineFilter(filter)}
                      >
                        {libraryFilterLabel(filter)}
                      </button>
                    ))}
                  </div>
                </div>

                <div ref={categoryScrollerRef} className="expert-category-row" aria-label={t("experts.categories")}>
                  <button type="button" className={mineCategory === "all" ? "expert-category expert-category--active" : "expert-category"} onClick={() => setMineCategory("all")}>{t("experts.filter.all")}</button>
                  <button type="button" className={mineCategory === "expert" ? "expert-category expert-category--active" : "expert-category"} onClick={() => setMineCategory("expert")}>{t("experts.modeExpert")}</button>
                  <button type="button" className={mineCategory === "team" ? "expert-category expert-category--active" : "expert-category"} onClick={() => setMineCategory("team")}>{t("experts.modeTeam")}</button>
                  {mineTeamCategories.map((item) => (
                    <button key={item} type="button" className={mineCategory === item ? "expert-category expert-category--active" : "expert-category"} onClick={() => setMineCategory(item)}>{item}</button>
                  ))}
                </div>
              </nav>

              <div className="expert-manage-grid">
                <button type="button" className="expert-create-card" onClick={() => openCreate("create")}>
                  <span><Plus size={22} aria-hidden="true" /></span>
                  <strong>{t("experts.createNewTitle")}</strong>
                  <p>{t("experts.createNewBody")}</p>
                  <em>{t("experts.startCreate")}</em>
                </button>

                {myItems.map((item) => (
                  <div key={item.id} className="expert-manage-card">
                    <button type="button" className="expert-manage-card__body" onClick={() => setSelected(item)}>
                      <div className="expert-card__identity">
                        <ExpertAvatar tone={item.tone} team={item.mode === "team"} size="lg" icon={item.icon} variant={item.id} src={item.avatarSrc} />
                        <div className="expert-card__title">
                          <strong>{item.name}</strong>
                          <small>{item.mode === "team" ? t("experts.memberCount", { count: item.members?.length ?? 3 }) : item.role}</small>
                        </div>
                      </div>
                      <p>{item.description}</p>
                      <div className="expert-tags">
                        {item.tags.slice(0, 3).map((tag) => <span key={tag}>{tag}</span>)}
                      </div>
                    </button>
                    <div className="expert-manage-card__actions">
                      {item.source === "user" && (
                        <>
                          {!isCloudUserAssetID(item.id) && (
                            <button type="button" onClick={() => openEdit(item)}><Pencil size={13} aria-hidden="true" />{t("experts.edit")}</button>
                          )}
                          <button type="button" onClick={() => void duplicateExpert(item)}><Copy size={13} aria-hidden="true" />{t("experts.duplicate")}</button>
                          {renderDeleteExpertControl(item)}
                        </>
                      )}
                      <button type="button" className="expert-manage-card__use" onClick={() => summon(item)}>{t("experts.use")}</button>
                    </div>
                  </div>
                ))}
              </div>

              {myItems.length === 0 && (
                <div className="expert-empty">
                  <strong>{t("experts.emptyTitle")}</strong>
                  <span>{t("experts.emptyBody")}</span>
                </div>
              )}
            </div>

          </section>
          </>
        ) : (
          <>
          <section className="expert-section" aria-labelledby="expert-scenes-title">
            <h2 id="expert-scenes-title">{t("experts.scenes")}</h2>
            <div className="expert-scenes">
              {SCENES.map((scene) => {
                return (
                  <div key={scene.id} className="expert-scene-card">
                    <img
                      className="expert-scene-card__background"
                      src={scene.backgroundSrc}
                      alt=""
                      aria-hidden="true"
                      style={{ objectPosition: scene.backgroundPosition }}
                    />
                    <button type="button" className="expert-scene-card__head" onClick={() => selectScene(scene)}>
                      <strong>{scene.title}</strong>
                    </button>
                    <div className="expert-scene-card__list">
                      {(sceneItems.get(scene.id) ?? []).map((item) => (
                        <button key={item.id} type="button" className="expert-scene-person" onClick={() => setSelected(item)}>
                          <ExpertAvatar tone={item.tone} team={item.mode === "team"} size="sm" icon={item.icon} variant={item.id} src={item.avatarSrc} />
                          <span>{item.name}</span>
                          <ChevronRight size={14} aria-hidden="true" />
                        </button>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          </section>

          <nav className="expert-catalog-nav" aria-label={t("experts.modeSwitch")}>
            <div className="expert-catalog-nav__primary">
              <div className="expert-mode-switch expert-mode-switch--catalog" role="tablist" aria-label={t("experts.modeSwitch")}>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mode === "expert"}
                  className={mode === "expert" ? "expert-mode-switch__item expert-mode-switch__item--active" : "expert-mode-switch__item"}
                  onClick={() => setMode("expert")}
                >
                  {t("experts.modeExpert")}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mode === "team"}
                  className={mode === "team" ? "expert-mode-switch__item expert-mode-switch__item--active" : "expert-mode-switch__item"}
                  onClick={() => setMode("team")}
                >
                  {t("experts.modeTeam")}
                </button>
              </div>

              <div className="expert-sort-switch" role="group" aria-label={t("experts.sort.label")}>
                <button
                  type="button"
                  aria-pressed={sort === "hot"}
                  className={sort === "hot" ? "expert-sort-switch__item expert-sort-switch__item--active" : "expert-sort-switch__item"}
                  onClick={() => setSort("hot")}
                >
                  {t("experts.sort.hot")}
                </button>
                <button
                  type="button"
                  aria-pressed={sort === "latest"}
                  className={sort === "latest" ? "expert-sort-switch__item expert-sort-switch__item--active" : "expert-sort-switch__item"}
                  onClick={() => setSort("latest")}
                >
                  {t("experts.sort.latest")}
                </button>
              </div>
            </div>

            <div ref={categoryScrollerRef} className="expert-category-row" aria-label={t("experts.categories")}>
              {availableCategories.map((item) => (
                <button
                  key={item}
                  type="button"
                  className={item === category ? "expert-category expert-category--active" : "expert-category"}
                  onClick={() => setCategory(item)}
                >
                  {item}
                </button>
              ))}
            </div>
          </nav>

          <section className="expert-section expert-section--catalog" aria-label={modeLabel}>
            <div className="expert-grid">
              {filteredItems.map((item) => (
                <div key={item.id} className="expert-card-shell">
                  <button type="button" className="expert-card" onClick={() => setSelected(item)}>
                    <div className="expert-card__identity">
                      <ExpertAvatar tone={item.tone} team={item.mode === "team"} size="lg" icon={item.icon} variant={item.id} src={item.avatarSrc} />
                      <div className="expert-card__title">
                        <strong>{item.name}</strong>
                        <small>{item.role}</small>
                      </div>
                    </div>
                    <p>{item.description}</p>
                    <div className="expert-tags">
                      {item.tags.slice(0, 3).map((tag) => <span key={tag}>{tag}</span>)}
                    </div>
                  </button>
                  <button type="button" className="expert-card__summon" aria-label={`${t("experts.summon")} ${item.name}`} onClick={() => summon(item)}>
                    {t("experts.summon")}
                  </button>
                </div>
              ))}
            </div>
            {filteredItems.length === 0 && (
              <div className="expert-empty">
                <strong>{t("experts.emptyTitle")}</strong>
                <span>{t("experts.emptyBody")}</span>
              </div>
            )}
          </section>
          </>
        )}
      </div>

      {selected && (
        <div className="expert-modal-backdrop" role="presentation" onMouseDown={() => setSelected(null)}>
          <div className="expert-modal expert-modal--detail" role="dialog" aria-modal="true" aria-labelledby="expert-modal-title" onMouseDown={(event) => event.stopPropagation()}>
            <div className="expert-modal__scroll">
              <button type="button" className="expert-modal__close" aria-label={t("common.close")} onClick={() => setSelected(null)}>
                <X size={17} aria-hidden="true" />
              </button>
              <div className="expert-modal__head">
                <ExpertAvatar tone={selected.tone} team={selected.mode === "team"} size="lg" icon={selected.icon} variant={selected.id} src={selected.avatarSrc} />
                <div className="expert-modal__title-block">
                  <h2 id="expert-modal-title">{selected.name}</h2>
                  <div className="expert-modal__meta">
                    <span className="expert-modal__category">{selected.role}</span>
                  </div>
                  <span className="expert-modal__usage">
                    {t("experts.usageCount", { count: formatExpertUsageCount(usageCounts[selected.id] ?? 0) })}
                  </span>
                </div>
              </div>

              <div className="expert-modal__body">
                <section className="expert-modal__section">
                  <h3>{t("experts.capability")}</h3>
                  <p>{selected.description}</p>
                </section>

              <section className="expert-modal__section">
                <h3>{t("experts.fields")}</h3>
                <div className="expert-tags expert-tags--modal">
                  {selected.tags.map((tag) => <span key={tag}>{tag}</span>)}
                </div>
              </section>

              {selected.mode === "team" && selected.members && (
                <section className="expert-modal__section">
                  <h3>{t("experts.members")}</h3>
                  <div className="expert-member-list">
                    {selected.members.map((member, memberIndex) => (
                      <div
                        key={`${selected.id}-${member.name}`}
                        className="expert-member"
                        title={[member.task, member.output].filter(Boolean).join(" · ")}
                      >
                        <ExpertAvatar tone={member.tone} size="sm" src={member.avatarSrc || uploadedAvatarSrc(member.avatarKey) || AVATAR_BY_EXPERT_NAME[member.name]} />
                        <div className="expert-member__content">
                          <div className="expert-member__head">
                            <strong>{member.name}</strong>
                            <span>{member.role}</span>
                            {(member.kind === "lead" || memberIndex === 0) && (
                              <span className="expert-member__lead">
                                <Star size={12} aria-hidden="true" fill="currentColor" />
                                {t("experts.lead")}
                              </span>
                            )}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </section>
              )}

                <section className="expert-modal__section">
                  <h3>{t("experts.tryAsking")}</h3>
                  <div className="expert-prompt-list">
                    {selected.prompts.slice(0, MAX_EXPERT_PROMPTS).map((prompt, index) => (
                      <button key={`${selected.id}-prompt-${index}`} type="button" onClick={() => summon(selected, prompt)}>
                        <span>“{prompt}”</span>
                        <ChevronRight size={14} aria-hidden="true" />
                      </button>
                    ))}
                  </div>
                </section>
              </div>
            </div>

            <div className="expert-modal__footer">
              <button type="button" className="expert-summon-button" onClick={() => summon(selected)}>
                {t("experts.summon")} {selected.name}
              </button>
            </div>
          </div>
        </div>
      )}

      {draftOpen && (
        <div className="expert-modal-backdrop" role="presentation" onMouseDown={() => setDraftOpen(false)}>
          <div className="expert-modal expert-modal--form" role="dialog" aria-modal="true" aria-labelledby="expert-draft-title" onMouseDown={(event) => event.stopPropagation()}>
            <button type="button" className="expert-modal__close" aria-label={t("common.close")} onClick={() => setDraftOpen(false)}>
              <X size={17} aria-hidden="true" />
            </button>
            <div className="expert-modal__head">
              <div className="expert-draft-avatar">
                <ExpertAvatar tone={draft.mode === "team" ? "teal" : "blue"} team={draft.mode === "team"} size="lg" icon={draft.mode === "team" ? Users : Bot} src={draftAvatarSrc} />
              </div>
              <div>
                <h2 id="expert-draft-title">
                  {draftMode === "edit"
                    ? t(draft.mode === "team" ? "experts.formEditTeamTitle" : "experts.formEditTitle")
                    : draftMode === "import"
                      ? t("experts.formImportTitle")
                      : t(draft.mode === "team" ? "experts.formCreateTeamTitle" : "experts.formCreateTitle")}
                </h2>
                <div className="expert-modal__meta">
                  <span>{draft.mode === "team" ? t("experts.modeTeam") : t("experts.modeExpert")}</span>
                  <span>{t("experts.formCloudAsset")}</span>
                </div>
              </div>
            </div>

            <div className="expert-modal__body expert-draft-form">
              {draftMode === "import" && <p className="expert-draft-form__hint">{t("experts.formImportHint")}</p>}
              {draftMode === "import" && (
                <label className="expert-draft-form__wide">
                  <span>导入 JSON</span>
                  <textarea value={draft.importPayload} onChange={(event) => setDraft((current) => ({ ...current, importPayload: event.target.value }))} placeholder='{"name":"Imported Expert","systemPrompt":"..."}' />
                  <button type="button" className="expert-form-secondary" onClick={() => void parseImportPayload()}>解析配置</button>
                </label>
              )}
              <section className="expert-draft-section expert-draft-form__wide" aria-labelledby="expert-draft-basic-title">
                <div className="expert-draft-section__head">
                  <div>
                    <h3 id="expert-draft-basic-title">{t("experts.formBasicSection")}</h3>
                    <p>{t("experts.formBasicSectionHint")}</p>
                  </div>
                </div>
                <div className="expert-draft-section__grid">
                  <label className="expert-draft-form__wide">
                    <span>{t("experts.formName")}</span>
                    <input value={draft.name} onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))} placeholder={t("experts.formNamePlaceholder")} />
                  </label>
                  <div className="expert-draft-form__wide expert-avatar-field">
                    <span className="expert-avatar-field__label">{t("experts.formAvatar")}</span>
                    <div className="expert-avatar-field__controls">
                      <label className="expert-avatar-upload">
                        <Upload size={14} aria-hidden="true" />
                        {t("experts.formAvatarChoose")}
                        <input
                          type="file"
                          accept="image/png,image/jpeg,image/webp"
                          aria-label={t("experts.formAvatarChoose")}
                          onChange={handleDraftAvatarChange}
                        />
                      </label>
                      {draft.avatarFileName && <span className="expert-avatar-field__filename">{draft.avatarFileName}</span>}
                      {draftAvatarSrc && (
                        <button type="button" className="expert-avatar-field__remove" onClick={clearDraftAvatar}>
                          {t("experts.formAvatarRemove")}
                        </button>
                      )}
                    </div>
                    <small className={draftAvatarError ? "expert-avatar-field__hint expert-avatar-field__hint--error" : "expert-avatar-field__hint"}>
                      {draftAvatarError || t("experts.formAvatarHint")}
                    </small>
                  </div>
                  <label>
                    <span>{t("experts.formMode")}</span>
                    <select
                      value={draft.mode}
                      onChange={(event) => handleDraftModeChange(event.target.value as ExpertMode)}
                    >
                      <option value="expert">{t("experts.modeExpert")}</option>
                      <option value="team">{t("experts.modeTeam")}</option>
                    </select>
                  </label>
                  <label>
                    <span>{t("experts.formRole")}</span>
                    <input
                      list="expert-category-suggestions"
                      value={draft.role}
                      onInput={(event) => {
                        const role = event.currentTarget.value;
                        setDraft((current) => ({ ...current, role }));
                      }}
                      placeholder={t("experts.formCategoryPlaceholder")}
                    />
                    <datalist id="expert-category-suggestions">
                      {USER_EXPERT_CATEGORY_SUGGESTIONS.map((category) => <option key={category} value={category} />)}
                    </datalist>
                  </label>
                  <div className="expert-draft-form__wide expert-tag-field">
                    <span className="expert-tag-field__label">{t("experts.formTags")}</span>
                    <div className="expert-tag-editor">
                      <div className="expert-tag-editor__input-row">
                        <input
                          value={draft.tagInput}
                          onInput={(event) => {
                            const tagInput = event.currentTarget.value;
                            setDraft((current) => ({ ...current, tagInput }));
                          }}
                          onKeyDown={(event) => {
                            if (event.key !== "Enter" || event.nativeEvent.isComposing) return;
                            event.preventDefault();
                            addDraftTag();
                          }}
                          placeholder={t("experts.formTagsPlaceholder")}
                          aria-label={t("experts.formTagsPlaceholder")}
                          disabled={draft.tags.length >= MAX_EXPERT_TAGS}
                        />
                        <button
                          type="button"
                          className="expert-form-secondary expert-tag-editor__add"
                          onClick={addDraftTag}
                          disabled={!draft.tagInput.trim() || draft.tags.length >= MAX_EXPERT_TAGS}
                        >
                          <Plus size={14} aria-hidden="true" />
                          {t("experts.formTagAdd")}
                        </button>
                      </div>
                      {draft.tags.length > 0 && (
                        <div className="expert-tag-editor__tags" aria-label={t("experts.formTagsList")}>
                          {draft.tags.map((tag, index) => (
                            <span className="expert-tag-editor__tag" key={`${tag}-${index}`}>
                              {tag}
                              <button type="button" onClick={() => removeDraftTag(index)} aria-label={t("experts.formTagRemove", { tag })}>
                                <X size={12} aria-hidden="true" />
                              </button>
                            </span>
                          ))}
                        </div>
                      )}
                      <small className="expert-tag-editor__hint">{t("experts.formTagsHint", { count: MAX_EXPERT_TAGS })}</small>
                    </div>
                  </div>
                  <label className="expert-draft-form__wide">
                    <span>{t("experts.formDescription")}</span>
                    <textarea value={draft.description} onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))} placeholder={t("experts.formDescriptionPlaceholder")} />
                  </label>
                </div>
              </section>
              {draft.mode === "team" && (
                <section className="expert-draft-agents expert-draft-form__wide" aria-label={t("experts.members")}>
                  <div className="expert-draft-agents__head">
                    <div>
                      <h3>{t("experts.members")}</h3>
                      <p>{t("experts.formTeamSectionHint")}</p>
                    </div>
                    <button type="button" className="expert-form-secondary" onClick={addDraftMember}>
                      <Plus size={13} aria-hidden="true" />
                      {t("experts.formAddMember")}
                    </button>
                  </div>
                  {draft.members.length === 0 && (
                    <div className="expert-draft-agents__empty">
                      <Users size={22} aria-hidden="true" />
                      <span>{t("experts.formNoMembers")}</span>
                    </div>
                  )}
                  {draft.members.map((member, index) => (
                    <div key={`${member.kind ?? "member"}-${index}`} className="expert-draft-agent">
                      <div className="expert-draft-agent__toolbar">
                        <label className="expert-draft-agent__picker">
                          <span>{t("experts.formSelectMember")}</span>
                          <select value={member.expertId ?? ""} onChange={(event) => selectDraftMember(index, event.target.value)}>
                            <option value="">{t("experts.formSelectMember")}</option>
                            {memberOptions.map((option) => (
                              <option key={option.id} value={option.id}>{option.name} ({option.id})</option>
                            ))}
                          </select>
                        </label>
                        <button type="button" className="expert-form-icon" aria-label={t("common.delete")} onClick={() => removeDraftMember(index)}>
                          <Trash2 size={15} aria-hidden="true" />
                        </button>
                      </div>
                      <div className="expert-draft-agent__details">
                        <label>
                          <span>{t("experts.formMemberName")}</span>
                          <input value={member.name} readOnly placeholder={t("experts.formMemberName")} />
                        </label>
                        <label>
                          <span>{t("experts.formMemberRole")}</span>
                          <input value={member.role} readOnly placeholder={t("experts.formMemberRole")} />
                        </label>
                        <label>
                          <span>{t("experts.formMemberTask")}</span>
                          <input value={member.task} readOnly placeholder={t("experts.formMemberTask")} />
                        </label>
                        <label>
                          <span>{t("experts.formMemberOutput")}</span>
                          <input value={member.output} readOnly placeholder={t("experts.formMemberOutput")} />
                        </label>
                      </div>
                    </div>
                  ))}
                </section>
              )}
              <section className="expert-draft-section expert-draft-form__wide" aria-labelledby="expert-draft-session-title">
                <div className="expert-draft-section__head">
                  <div>
                    <h3 id="expert-draft-session-title">{t("experts.formSessionSection")}</h3>
                    <p>{t("experts.formSessionSectionHint")}</p>
                  </div>
                </div>
                <label>
                  <span>{t("experts.formSystemPrompt")}</span>
                  <textarea value={draft.systemPrompt} onChange={(event) => setDraft((current) => ({ ...current, systemPrompt: event.target.value }))} placeholder={t("experts.formSystemPromptPlaceholder")} />
                </label>
              </section>
            </div>

            <div className="expert-modal__footer expert-draft-form__footer">
              <button type="button" className="expert-form-secondary" onClick={() => setDraftOpen(false)}>{t("common.cancel")}</button>
              <button type="button" className="expert-summon-button" onClick={() => void saveDraft()} disabled={draftSavePending || !draftCategoryValid || !draftTagsValid || !draftMembersValid}>
                {t(draft.mode === "team" ? "experts.formSaveTeam" : "experts.formSave")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
