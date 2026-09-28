#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const forbidden = [
  "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
  "KWU_TEST_WEBVIEW2_CDP_PORT",
  "--remote-debugging-port",
  "connectOverCDP",
];

export function productionDebugFlagFindings(files) {
  const findings = [];
  for (const file of files) {
    const path = file.path.replaceAll("\\", "/");
    const productionPath =
      /^desktop\/[^/]+\.go$/.test(path)
      || path === "desktop/wails.json"
      || path.startsWith("desktop/build/")
      || (
        path.startsWith("desktop/frontend/src/")
        && !path.includes("/__tests__/")
        && !path.includes("/vitest/")
      );
    if (!productionPath) continue;
    for (const marker of forbidden) {
      if (file.content.includes(marker)) {
        findings.push({ path, marker });
      }
    }
  }
  return findings;
}

function main() {
  const paths = execFileSync(
    "git",
    ["ls-files", "--cached", "--others", "--exclude-standard"],
    { cwd: repoRoot, encoding: "utf8" },
  ).split(/\r?\n/).filter(Boolean);
  const files = paths
    .map((path) => ({ path, absolutePath: resolve(repoRoot, path) }))
    .filter(({ absolutePath }) => existsSync(absolutePath))
    .map(({ path, absolutePath }) => ({
      path,
      content: readFileSync(absolutePath, "utf8"),
    }));
  const findings = productionDebugFlagFindings(files);
  if (findings.length > 0) {
    for (const finding of findings) {
      console.error(`${finding.path}: production WebView2 debug marker ${finding.marker}`);
    }
    process.exit(1);
  }
  console.log("production WebView2 debug guard: passed");
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
