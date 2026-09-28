// Run: tsx src/__tests__/creation-scenario-skills.test.ts

import { strict as assert } from "node:assert";
import { findCreationScenarioSkill } from "../features/composer/model/creationScenarioSkills";
import type { SkillView } from "../lib/types";

function skill(name: string, options: Partial<SkillView> = {}): SkillView {
  return {
    name,
    description: "",
    scope: "user",
    runAs: "skill",
    enabled: true,
    ...options,
  };
}

const documentRequest = {
  name: "document",
  label: "文档处理",
  candidates: ["documents", "document-processing", "document", "word", "docx", "pdf"],
};

assert.equal(
  findCreationScenarioSkill(documentRequest, [
    skill("unrelated"),
    skill("documents", { displayName: "Documents" }),
  ])?.name,
  "documents",
  "the scenario resolves the real installed skill instead of using its presentation id",
);

assert.equal(
  findCreationScenarioSkill(documentRequest, [
    skill("document", { scope: "builtin" }),
    skill("documents"),
  ])?.name,
  "documents",
  "an installed matching skill overrides the generic scenario builtin",
);

assert.equal(
  findCreationScenarioSkill(documentRequest, [
    skill("documents", { enabled: false }),
    skill("generic-helper", { description: "Document processing and office assistance" }),
  ])?.name,
  "documents",
  "an exact disabled match wins over a vague enabled description so the caller can enable it",
);

assert.equal(
  findCreationScenarioSkill({
    name: "design-ppt",
    label: "PPT设计",
    candidates: ["presentations", "presentation", "ppt-design", "powerpoint"],
  }, [
    skill("documents"),
    skill("Presentations", { enabled: false }),
  ])?.name,
  "Presentations",
  "matching is case insensitive and supports the presentation skill name",
);

assert.equal(
  findCreationScenarioSkill(documentRequest, [
    skill("ai", { description: "A general assistant" }),
    skill("unrelated", { description: "A general assistant" }),
  ]),
  null,
  "an unrelated skill is never turned into a fake scenario token",
);

console.log("creation scenario skill resolution: passed");
