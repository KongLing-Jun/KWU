import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { expertSelectionsFromRecentIds } from "../features/experts/model/expertSelection";
import type { ExpertInfo, UserExpertInfo } from "../lib/types";

const testDir = dirname(fileURLToPath(import.meta.url));
const controllerSource = readFileSync(resolve(testDir, "../features/experts/controller/useExpertCatalog.ts"), "utf8");
assert.match(
  controllerSource,
  /setRecentExperts\(expertSelectionsFromRecentIds\(\s*normalizedLibraryState\.recentIds,\s*normalizedExperts,\s*normalizedUserExperts,/,
  "expert refresh projects the persisted recent IDs into the composer menu",
);

const experts: ExpertInfo[] = Array.from({ length: 6 }, (_, index) => ({
  id: `expert-${index + 1}`,
  source: "official",
  mode: index === 0 ? "team" : "expert",
  name: `Expert ${index + 1}`,
  role: `Role ${index + 1}`,
  description: "Catalog expert",
  prompts: [`Prompt ${index + 1}`],
  enabled: true,
}));
const userExperts: UserExpertInfo[] = [{
  id: "expert-1",
  source: "user",
  mode: "team",
  name: "Expert 1",
  description: "Rich user expert",
  category: "User team",
  avatarKey: "https://assets.example.com/team.png",
  systemPrompt: "private",
  displayPrompt: "User prompt",
  enabled: true,
  createdAt: 1,
  updatedAt: 1,
}];

const recent = expertSelectionsFromRecentIds([
  "expert-3",
  "missing-expert",
  "expert-1",
  "expert-3",
  "expert-2",
  "expert-4",
  "expert-5",
  "expert-6",
], experts, userExperts);

assert.deepEqual(
  recent.map((expert) => expert.id),
  ["expert-3", "expert-1", "expert-2", "expert-4", "expert-5"],
  "the recent menu restores persisted order, skips stale IDs and duplicates, and keeps five entries",
);
assert.equal(recent[1]?.avatarSrc, "https://assets.example.com/team.png", "recent user experts keep rich avatar metadata");
assert.equal(recent[1]?.prompt, "User prompt", "recent user experts keep their display prompt");

console.log("expert recent menu history passed");
