#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { dirname, isAbsolute, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const ignoreMarker = ".markdown-link-check-ignore";
const markdownLinkPattern = /!?\[[^\]]*]\(([^)]+)\)/g;
const externalTargetPattern = /^(?:https?:|mailto:|data:|tel:|#)/i;
const windowsAbsolutePattern = /^[A-Za-z]:[\\/]/;

function normalizeTarget(raw) {
  let target = raw.trim();
  if (target.startsWith("<") && target.includes(">")) {
    target = target.slice(1, target.indexOf(">"));
  } else {
    target = target.replace(/\s+(?:"[^"]*"|'[^']*')\s*$/, "");
  }
  return target.trim();
}

function localPathFromTarget(target) {
  const withoutFragment = target.split("#", 1)[0];
  const withoutQuery = withoutFragment.split("?", 1)[0];
  try {
    return decodeURIComponent(withoutQuery);
  } catch {
    return withoutQuery;
  }
}

function isIgnored(root, absoluteFile) {
  const absoluteRoot = resolve(root);
  let directory = dirname(absoluteFile);
  while (directory !== absoluteRoot) {
    if (existsSync(resolve(directory, ignoreMarker))) return true;
    const parent = dirname(directory);
    if (parent === directory) break;
    directory = parent;
  }
  return existsSync(resolve(absoluteRoot, ignoreMarker));
}

export function collectMarkdownIssues(root, files) {
  const issues = [];
  for (const file of files) {
    const absoluteFile = resolve(root, file);
    if (!existsSync(absoluteFile)) continue;
    if (isIgnored(root, absoluteFile)) continue;
    const source = readFileSync(absoluteFile, "utf8");
    for (const match of source.matchAll(markdownLinkPattern)) {
      const target = normalizeTarget(match[1]);
      if (!target || externalTargetPattern.test(target)) continue;
      const localPath = localPathFromTarget(target);
      if (!localPath) continue;
      const line = source.slice(0, match.index).split("\n").length;
      if (
        target.toLowerCase().startsWith("file://") ||
        windowsAbsolutePattern.test(localPath) ||
        isAbsolute(localPath)
      ) {
        issues.push({ file, line, target, reason: "absolute local path" });
        continue;
      }
      const resolvedTarget = resolve(dirname(absoluteFile), localPath);
      if (!existsSync(resolvedTarget)) {
        issues.push({ file, line, target, reason: "missing local target" });
      }
    }
  }
  return issues;
}

function repositoryMarkdownFiles() {
  return execFileSync(
    "git",
    ["ls-files", "-co", "--exclude-standard", "-z", "--", "*.md"],
    {
      cwd: repoRoot,
    },
  )
    .toString("utf8")
    .split("\0")
    .filter(Boolean);
}

function main() {
  const issues = collectMarkdownIssues(repoRoot, repositoryMarkdownFiles());
  if (issues.length === 0) {
    process.stdout.write("Markdown local links: OK\n");
    return;
  }
  for (const issue of issues) {
    process.stderr.write(
      `${issue.file}:${issue.line}: ${issue.reason}: ${issue.target}\n`,
    );
  }
  process.stderr.write(`Markdown local links: ${issues.length} issue(s)\n`);
  process.exitCode = 1;
}

const isMain = process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isMain) main();
