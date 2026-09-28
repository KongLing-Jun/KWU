#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { basename, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const SURFACES = [
  {
    id: "root-go",
    owner: "@SivanCola",
    command: "go test ./...",
    gate: "proportional PR; full on core or broad changes",
    matches: (path) => path.endsWith("_test.go") && !path.startsWith("desktop/"),
  },
  {
    id: "desktop-go",
    owner: "@SivanCola",
    command: "go test ./...",
    workingDirectory: "desktop",
    gate: "proportional PR; full on desktop core or release changes",
    matches: (path) => path.startsWith("desktop/") && path.endsWith("_test.go"),
  },
  {
    id: "frontend",
    owner: "@SivanCola",
    command: "pnpm test:all",
    workingDirectory: "desktop/frontend",
    gate: "not in PR gate; frontend PR changes run pnpm build only",
    matches: (path) =>
      path.startsWith("desktop/frontend/") &&
      (path.endsWith(".test.ts") || path.endsWith(".test.tsx")),
  },
  {
    id: "release-contract",
    owner: "@SivanCola",
    command: "node scripts/pr-gate.mjs --base origin/main-v3 --run frontend",
    gate: "PR when release-contract paths change",
    matches: (path) => {
      if (!path.startsWith("scripts/")) return false;
      const name = basename(path);
      return (
        name.endsWith("-test.ps1") ||
        name.endsWith("-test.sh") ||
        name.endsWith(".test.mjs")
      );
    },
  },
];

function normalizePath(path) {
  return path.replaceAll("\\", "/").replace(/^\.?\//, "");
}

export function classifyTestAssetPath(inputPath) {
  const path = normalizePath(inputPath);
  return SURFACES.find((surface) => surface.matches(path))?.id ?? null;
}

function repositoryFiles() {
  const output = execFileSync(
    "git",
    ["ls-files", "--cached", "--others", "--exclude-standard", "-z"],
    { cwd: repoRoot, encoding: "utf8" },
  );
  return output.split("\0").filter(Boolean).map(normalizePath);
}

function sourceCommit() {
  return execFileSync("git", ["rev-parse", "HEAD"], {
    cwd: repoRoot,
    encoding: "utf8",
  }).trim();
}

function readRepositoryFile(path) {
  return readFileSync(resolve(repoRoot, path), "utf8");
}

function countMatches(paths, pattern) {
  let count = 0;
  for (const path of paths) {
    if (pattern.test(readRepositoryFile(path))) count += 1;
  }
  return count;
}

export function collectTestingInventory(files = repositoryFiles()) {
  const normalizedFiles = files.map(normalizePath);
  const testFiles = normalizedFiles.filter((path) => classifyTestAssetPath(path));
  const goTestFiles = testFiles.filter((path) => path.endsWith("_test.go"));

  return {
    schemaVersion: 1,
    sourceCommit: sourceCommit(),
    surfaces: SURFACES.map(({ matches, ...surface }) => {
      const paths = normalizedFiles.filter(matches);
      return {
        ...surface,
        fileCount: paths.length,
        packageOrDirectoryCount: new Set(paths.map((path) => dirname(path))).size,
      };
    }),
    capabilities: {
      benchmarkFiles: countMatches(goTestFiles, /\bfunc\s+Benchmark\w+\s*\(/),
      fuzzTargetFiles: countMatches(goTestFiles, /\bfunc\s+Fuzz\w+\s*\(/),
      httptestFiles: countMatches(goTestFiles, /\bhttptest\./),
      goleakTestMainFiles: countMatches(goTestFiles, /\bgoleak\.VerifyTestMain\s*\(/),
      buildTaggedTestFiles: countMatches(goTestFiles, /^\/\/go:build\s+/m),
    },
    workflows: normalizedFiles
      .filter((path) => path.startsWith(".github/workflows/") && path.endsWith(".yml"))
      .sort(),
  };
}

function main() {
  process.stdout.write(`${JSON.stringify(collectTestingInventory(), null, 2)}\n`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
