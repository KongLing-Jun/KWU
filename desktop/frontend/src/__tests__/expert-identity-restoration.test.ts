import assert from "node:assert/strict";

import { runtimeComposerSessionIdentity } from "../lib/composerSessionIdentity";
import { expertSelectionFromState } from "../features/experts/model/expertSelection";
import type { ComposerSessionIdentity, ExpertInfo, UserExpertInfo } from "../lib/types";

const previous: ComposerSessionIdentity = {
  modelRef: "provider/model",
  billingSource: "",
  expertKnown: false,
  expertId: "",
  expertName: "",
  collaborationMode: "normal",
  toolApprovalMode: "ask",
  tokenMode: "full",
  goal: "",
  goalStatus: "stopped",
};

const runtimeIdentity = runtimeComposerSessionIdentity({
  expertId: "custom-team",
  expertName: "Custom Team",
}, previous);
assert.equal(runtimeIdentity.expertKnown, true, "a runtime expert binding replaces the blank task identity cache");
assert.equal(runtimeIdentity.expertId, "custom-team");

const flattenedCatalog: ExpertInfo[] = [{
  id: "custom-team",
  source: "user",
  mode: "team",
  name: "Custom Team",
  description: "Flattened catalog record",
  enabled: true,
  members: [{ kind: "sub", name: "Research Expert", role: "Research", task: "Research", output: "Notes", tone: "green" }],
}];
const userCatalog: UserExpertInfo[] = [{
  id: "custom-team",
  source: "user",
  mode: "team",
  name: "Custom Team",
  description: "Rich user record",
  category: "Research",
  avatarKey: "https://assets.example.com/team.png",
  members: [{ kind: "sub", name: "Research Expert", role: "Research", task: "Research", output: "Notes", tone: "green", avatarKey: "https://assets.example.com/research.png" }],
  systemPrompt: "private",
  enabled: true,
  createdAt: 1,
  updatedAt: 1,
}];

const selection = expertSelectionFromState({ selectedId: "custom-team", selectedName: "Custom Team", source: "session", canSwitch: false, requiresNewSession: false }, flattenedCatalog, userCatalog);
assert.equal(selection?.mode, "team");
assert.equal(selection?.avatarSrc, "https://assets.example.com/team.png", "task restoration keeps the uploaded main expert avatar");
assert.equal(selection?.members?.[0]?.avatarSrc, "https://assets.example.com/research.png", "task restoration keeps uploaded child expert avatars");

const projectTeam = expertSelectionFromState({
  selectedId: "project-expert-team-116fb3dc1ea757d6",
  selectedName: "项目专家团",
  source: "session",
  canSwitch: false,
  requiresNewSession: false,
}, [], []);
assert.equal(projectTeam?.mode, "team", "project expert teams survive authoritative backend refreshes without a catalog record");
assert.equal(projectTeam?.name, "项目专家团");

console.log("expert identity restoration passed");
