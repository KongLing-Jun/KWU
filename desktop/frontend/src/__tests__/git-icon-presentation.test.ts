// Run: tsx src/__tests__/git-icon-presentation.test.ts

import { readFileSync, readdirSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testDir = dirname(fileURLToPath(import.meta.url));
const sourceRoot = resolve(testDir, "..");

function collectComponentFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === "__tests__") continue;
    const path = join(dir, entry.name);
    if (entry.isDirectory()) files.push(...collectComponentFiles(path));
    else if (entry.isFile() && entry.name.endsWith(".tsx")) files.push(path);
  }
  return files;
}

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\nGit icon presentation");

const gitIconFiles = collectComponentFiles(sourceRoot)
  .filter((path) => /\bGitBranch\b/.test(readFileSync(path, "utf8")))
  .map((path) => relative(sourceRoot, path).replaceAll("\\", "/"));

ok(
  gitIconFiles.length === 0,
  `client components do not render Git branch glyphs${gitIconFiles.length ? `: ${gitIconFiles.join(", ")}` : ""}`,
);

const appSource = readFileSync(resolve(sourceRoot, "App.tsx"), "utf8");
const statusBarSource = readFileSync(resolve(sourceRoot, "components/StatusBar.tsx"), "utf8");
const welcomeSource = readFileSync(resolve(sourceRoot, "components/Welcome.tsx"), "utf8");

ok(
  !appSource.includes('openRightDockMode("changed")') &&
    !appSource.includes('t("workspace.changedTab")') &&
    appSource.includes('workspacePanelState.mode === "changed" ? "files" : workspacePanelState.mode'),
  "client shell omits the changes view and normalizes stale change-mode state to files",
);
ok(
  statusBarSource.includes("branchLabel") && welcomeSource.includes('{ id: "code-cicd", icon: Workflow,'),
  "branch text remains available and CI/CD uses a neutral workflow glyph",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
