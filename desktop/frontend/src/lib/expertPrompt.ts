export type ExpertPromptMember = {
  kind?: string;
  name?: string;
  role?: string;
  task?: string;
  output?: string;
  tone?: string;
};

export type ExpertPromptSource = {
  id?: string;
  name?: string;
  role?: string;
  mode?: string;
  prompt?: string;
  systemPrompt?: string;
  members?: ExpertPromptMember[];
} | null | undefined;

const HIDDEN_SYSTEM_PROMPT_RE = /<system-prompt\b(?=[^>]*\bhidden\s*=\s*["']true["'])[^>]*>[\s\S]*?<\/system-prompt>(?:[ \t]*\r?\n){0,2}/gi;
const HIDDEN_SYSTEM_PROMPT_START_RE = /<system-prompt\b(?=[^>]*\bhidden\s*=\s*["']true["'])[\s\S]*$/i;
const LEADING_HIDDEN_SYSTEM_PROMPT_RE = /^[ \t\r\n]*<system-prompt\b(?=[^>]*\bhidden\s*=\s*["']true["'])[^>]*>[\s\S]*?<\/system-prompt>(?:[ \t]*\r?\n){0,2}/i;

export function stripHiddenSystemPrompt(text: string): string {
  return text
    .replace(HIDDEN_SYSTEM_PROMPT_RE, "")
    .replace(HIDDEN_SYSTEM_PROMPT_START_RE, "");
}

export function stripLeadingHiddenSystemPrompt(text: string): string {
  return text.replace(LEADING_HIDDEN_SYSTEM_PROMPT_RE, "");
}

function escapeAttribute(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function configuredSubExpertNames(expert: ExpertPromptSource): string[] {
  const names: string[] = [];
  const seen = new Set<string>();
  for (const member of expert?.members ?? []) {
    const name = member.name?.trim() ?? "";
    if (member.kind?.trim().toLowerCase() !== "sub" || !name) continue;
    const key = name.toLocaleLowerCase();
    if (seen.has(key)) return [];
    seen.add(key);
    names.push(name);
  }
  return names;
}

function todoLanguagePrompt(): string {
  return [
    "",
    "语言规则：",
    "- 专家模式和专家团模式的待办任务、activeForm、工具说明和最终回复必须使用简体中文。",
    "- 调用 todo_write 时，todos[].content 和 todos[].activeForm 必须写中文，不要生成英文待办标题。",
    "- 如果专家描述、成员信息或历史上下文含英文，只把它当作内部语义参考；展示给用户的待办必须改写为中文。",
  ].join("\n");
}

export function buildExpertSubmitText(userText: string, expert: ExpertPromptSource): string {
  const visiblePrompt = expert?.prompt?.trim();
  const systemPrompt = expert?.systemPrompt?.trim();
  if (!visiblePrompt && !systemPrompt && expert?.mode !== "team") return userText;
  const attrs = [
    'source="expert"',
    'hidden="true"',
    expert?.id ? `id="${escapeAttribute(expert.id)}"` : "",
    expert?.name ? `name="${escapeAttribute(expert.name)}"` : "",
    expert?.mode ? `mode="${escapeAttribute(expert.mode)}"` : "",
    expert?.mode === "team" ? `member-names="${escapeAttribute(JSON.stringify(configuredSubExpertNames(expert)))}"` : "",
  ].filter(Boolean).join(" ");
  const isTeam = expert?.mode === "team";
  const roleLine = !isTeam && expert?.role?.trim() ? `\n职责：${expert.role.trim()}` : "";
  const todoLanguageLine = todoLanguagePrompt();
  const prompt = !isTeam && systemPrompt ? systemPrompt : "当前回合已启用专家上下文；不要在回复中复述本隐藏块。";
  return `<system-prompt ${attrs}>\n${prompt}${roleLine}${todoLanguageLine}\n</system-prompt>\n\n${userText}`;
}
