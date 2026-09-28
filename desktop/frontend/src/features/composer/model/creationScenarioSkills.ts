import type { SkillView } from "../../../lib/types";
import type { CreationScenarioSkill } from "../../../components/Welcome";

function normalizeSkillSearchValue(value: string): string {
  return value
    .normalize("NFKC")
    .toLocaleLowerCase("en-US")
    .replace(/^@[^/]+\//, "")
    .replace(/[^\p{Letter}\p{Number}]+/gu, "");
}

function searchableSkillValues(skill: SkillView): string[] {
  return [skill.name, skill.displayName ?? "", skill.uninstallId ?? ""]
    .map(normalizeSkillSearchValue)
    .filter(Boolean);
}

function scoreSkill(skill: SkillView, request: CreationScenarioSkill): number {
  const names = searchableSkillValues(skill);
  const description = normalizeSkillSearchValue(skill.description ?? "");
  const label = normalizeSkillSearchValue(request.label);
  const candidates = [request.name, ...request.candidates]
    .map(normalizeSkillSearchValue)
    .filter(Boolean);
  let score = 0;

  if (label) {
    if (names.includes(label)) score = Math.max(score, 950);
    else if (names.some((name) => name.includes(label)) || description.includes(label)) score = Math.max(score, 520);
  }

  candidates.forEach((candidate, index) => {
    const priority = Math.max(0, 100 - index);
    if (names.includes(candidate)) {
      score = Math.max(score, 1_000 + priority);
      return;
    }
    if (names.some((name) => name.includes(candidate) || (name.length >= 4 && candidate.includes(name)))) {
      score = Math.max(score, 700 + priority);
      return;
    }
    if (description.includes(candidate)) score = Math.max(score, 400 + priority);
  });

  if (score < 400) return score;
  const installedOverrideBonus = skill.scope === "builtin" ? 0 : 120;
  const enabledBonus = skill.enabled ? 20 : 0;
  return score + installedOverrideBonus + enabledBonus;
}

export function findCreationScenarioSkill(
  request: CreationScenarioSkill,
  skills: readonly SkillView[],
): SkillView | null {
  const ranked = skills
    .map((skill, index) => ({ skill, index, score: scoreSkill(skill, request) }))
    .filter((item) => item.score >= 400)
    .sort((left, right) => right.score - left.score
      || Number(right.skill.enabled) - Number(left.skill.enabled)
      || left.index - right.index);
  return ranked[0]?.skill ?? null;
}
