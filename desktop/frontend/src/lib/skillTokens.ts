import { t } from "./i18n";
import type { DictKey } from "../locales/zh";

export const SKILL_TOKEN_PREFIX = "/";

const SKILL_NAME_SOURCE = "[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}";
const SLASH_COMMAND_RE = new RegExp(`(^|\\s)/(${SKILL_NAME_SOURCE})(?=$|\\s)`, "g");

const CREATION_SCENARIO_LABEL_KEYS: Partial<Record<string, DictKey>> = {
  document: "welcome.creation.scenarioDocument",
  finance: "welcome.creation.scenarioFinance",
  data: "welcome.creation.scenarioData",
  meeting: "welcome.creation.scenarioMeeting",
  project: "welcome.creation.scenarioProject",
  research: "welcome.creation.scenarioResearch",
  automation: "welcome.creation.scenarioAutomation",
  "code-daily": "welcome.creation.scenarioCodeDaily",
  "code-website": "welcome.creation.scenarioCodeWebsite",
  "code-agent": "welcome.creation.scenarioCodeAgent",
  "code-skill": "welcome.creation.scenarioCodeSkill",
  "code-cicd": "welcome.creation.scenarioCodeCiCd",
  "code-docs": "welcome.creation.scenarioCodeDocs",
  "design-website": "welcome.creation.scenarioDesignWebsite",
  "design-ppt": "welcome.creation.scenarioDesignPpt",
  "design-poster": "welcome.creation.scenarioDesignPoster",
  "design-mobile": "welcome.creation.scenarioDesignMobile",
  "design-system": "welcome.creation.scenarioDesignSystem",
  "design-webapp": "welcome.creation.scenarioDesignWebApp",
};

export function displaySkillTokenLabel(name: string): string {
  const key = CREATION_SCENARIO_LABEL_KEYS[name];
  return key ? t(key) : name;
}

export type SkillTokenPart =
  | { kind: "text"; text: string }
  | { kind: "skill"; name: string; raw: string };

export type ParsedSkillTokens = {
  parts: SkillTokenPart[];
  skillNames: string[];
};

export function formatSkillToken(name: string): string {
  return `${SKILL_TOKEN_PREFIX}${name}`;
}

function partsForSlashCommands(value: string, allowedNames?: ReadonlySet<string>): SkillTokenPart[] {
  const parts: SkillTokenPart[] = [];
  let cursor = 0;
  SLASH_COMMAND_RE.lastIndex = 0;
  for (let match = SLASH_COMMAND_RE.exec(value); match; match = SLASH_COMMAND_RE.exec(value)) {
    const name = match[2];
    if (allowedNames && !allowedNames.has(name)) continue;
    const tokenStart = match.index + match[1].length;
    if (tokenStart > cursor) parts.push({ kind: "text", text: value.slice(cursor, tokenStart) });
    const raw = `/${name}`;
    parts.push({ kind: "skill", name, raw });
    cursor = tokenStart + raw.length;
  }
  if (cursor < value.length) parts.push({ kind: "text", text: value.slice(cursor) });
  if (parts.length === 0 && value) parts.push({ kind: "text", text: value });
  return parts;
}

export function parseSkillTokenParts(
  value: string,
  skillNames: readonly string[],
): SkillTokenPart[] {
  return partsForSlashCommands(value, new Set(skillNames));
}

// Sent messages do not retain a skill catalogue snapshot. Render slash
// invocations as generic command tokens; known creation shortcuts can still
// use their localized labels, while execution depends on the controller's
// real command/skill registry.
export function parseSlashCommandParts(value: string): SkillTokenPart[] {
  return partsForSlashCommands(value);
}

export function parseSkillTokens(
  value: string,
  skillNames: readonly string[],
): ParsedSkillTokens {
  const parts = parseSkillTokenParts(value, skillNames);
  const uniqueNames = new Set<string>();
  for (const part of parts) {
    if (part.kind === "skill") uniqueNames.add(part.name);
  }
  return {
    parts,
    skillNames: [...uniqueNames],
  };
}

export function skillTokenNames(value: string, skillNames: readonly string[]): string[] {
  return parseSkillTokens(value, skillNames).skillNames;
}

function withoutMatchingSkillTokens(
  value: string,
  skillNames: readonly string[],
  shouldRemove: (name: string) => boolean,
): string {
  let output = "";
  let removed = false;
  for (const part of parseSkillTokenParts(value, skillNames)) {
    if (part.kind === "skill" && shouldRemove(part.name)) {
      removed = true;
      continue;
    }
    let next = part.kind === "skill" ? part.raw : part.text;
    if (removed && part.kind === "text") {
      if (!output || /\s$/.test(output)) next = next.replace(/^\s+/, "");
      else if (/^\s/.test(next)) next = ` ${next.replace(/^\s+/, "")}`;
      removed = false;
    }
    output += next;
  }
  return output.trim();
}

export function stripSkillTokens(value: string, skillNames: readonly string[]): string {
  return withoutMatchingSkillTokens(value, skillNames, () => true);
}

export function removeSkillToken(
  value: string,
  skillNames: readonly string[],
  name: string,
): string {
  return withoutMatchingSkillTokens(value, skillNames, (candidate) => candidate === name);
}
