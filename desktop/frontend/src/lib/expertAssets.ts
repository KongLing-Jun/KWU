import type { ExpertInfo, ExpertMemberInfo } from "./types";

const architectAvatar = new URL("../assets/experts/architect.png", import.meta.url).href;
const automationFlowAvatar = new URL("../assets/experts/automation-flow.png", import.meta.url).href;
const codeDeliveryTeamAvatar = new URL("../assets/experts/code-delivery-team.png", import.meta.url).href;
const codeReviewAvatar = new URL("../assets/experts/code-review.png", import.meta.url).href;
const customerOpsTeamAvatar = new URL("../assets/experts/customer-ops-team.png", import.meta.url).href;
const dataAnalysisTeamAvatar = new URL("../assets/experts/data-analysis-team.png", import.meta.url).href;
const dataReportAvatar = new URL("../assets/experts/data-report.png", import.meta.url).href;
const docReportAvatar = new URL("../assets/experts/doc-report.png", import.meta.url).href;
const officeAutomationTeamAvatar = new URL("../assets/experts/office-automation-team.png", import.meta.url).href;
const opsGrowthAvatar = new URL("../assets/experts/ops-growth.png", import.meta.url).href;
const projectDeliveryTeamAvatar = new URL("../assets/experts/project-delivery-team.png", import.meta.url).href;
const researchAvatar = new URL("../assets/experts/research.png", import.meta.url).href;
const securityReviewAvatar = new URL("../assets/experts/security-review.png", import.meta.url).href;

const avatarByKey: Record<string, string> = {
  architect: architectAvatar,
  "architecture-review": architectAvatar,
  "automation-flow": automationFlowAvatar,
  "code-delivery-team": codeDeliveryTeamAvatar,
  "code-review": codeReviewAvatar,
  "customer-ops-team": customerOpsTeamAvatar,
  "data-analysis-team": dataAnalysisTeamAvatar,
  "data-report": dataReportAvatar,
  "doc-report": docReportAvatar,
  documentation: docReportAvatar,
  "office-automation-team": officeAutomationTeamAvatar,
  "ops-growth": opsGrowthAvatar,
  product: opsGrowthAvatar,
  "project-delivery-team": projectDeliveryTeamAvatar,
  research: researchAvatar,
  "security-review": securityReviewAvatar,
  testing: codeReviewAvatar,
};

const avatarByExpertName: Record<string, string> = {
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

const fallbackAvatarByTone: Record<string, string> = {
  amber: dataReportAvatar,
  blue: docReportAvatar,
  green: securityReviewAvatar,
  orange: opsGrowthAvatar,
  stone: codeReviewAvatar,
  teal: automationFlowAvatar,
  violet: researchAvatar,
};

function normalizedKey(value?: string): string {
  return value?.trim().toLowerCase() ?? "";
}

export function expertAvatarSrcForKey(key?: string): string | undefined {
  const trimmed = key?.trim();
  if (!trimmed) return undefined;
  if (/^data:image\//i.test(trimmed)) return trimmed;
  if (/^https:\/\//i.test(trimmed)) return trimmed;
  return avatarByKey[normalizedKey(trimmed)];
}

export function expertAvatarSrcForName(name?: string): string | undefined {
  const trimmed = name?.trim();
  return trimmed ? avatarByExpertName[trimmed] : undefined;
}

export function expertAvatarSrcForInfo(info?: Pick<ExpertInfo, "id" | "avatarKey" | "avatarUrl" | "name"> | null): string | undefined {
  if (!info) return undefined;
  const avatarURL = info.avatarUrl?.trim();
  if (avatarURL?.startsWith("https://")) return avatarURL;
  return expertAvatarSrcForKey(info.avatarKey) || expertAvatarSrcForKey(info.id) || expertAvatarSrcForName(info.name);
}

export function expertMemberAvatarSrc(member?: Pick<ExpertMemberInfo, "avatarKey" | "avatarUrl" | "name"> | { avatarKey?: string; avatarUrl?: string; name?: string } | null): string | undefined {
  const avatarURL = member?.avatarUrl?.trim();
  if (avatarURL?.startsWith("https://")) return avatarURL;
  return expertAvatarSrcForKey(member?.avatarKey) || expertAvatarSrcForName(member?.name);
}

export function fallbackExpertMemberAvatarSrc(tone?: string): string {
  return fallbackAvatarByTone[normalizedKey(tone)] ?? researchAvatar;
}
