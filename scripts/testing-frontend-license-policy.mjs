#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const frontendRoot = resolve(repoRoot, "desktop/frontend");

export const allowedLicenses = new Set([
  "MIT",
  "ISC",
  "BSD-2-Clause",
  "BSD-3-Clause",
]);

const deniedLicensePattern = /^(?:A?GPL-|SSPL-|BUSL-|UNLICENSED$)|Commons Clause/i;

export function classifyLicenseGroups(groups) {
  return Object.entries(groups)
    .map(([license, packages]) => {
      let decision = "review";
      if (allowedLicenses.has(license)) {
        decision = "allow";
      } else if (deniedLicensePattern.test(license)) {
        decision = "deny";
      }
      return {
        license,
        decision,
        packages: packages.map((entry) => entry.name).filter(Boolean).sort(),
      };
    })
    .sort((left, right) => left.license.localeCompare(right.license));
}

function loadFrontendLicenseGroups() {
  const windows = process.platform === "win32";
  const command = windows ? (process.env.ComSpec || "cmd.exe") : "pnpm";
  const args = windows
    ? ["/d", "/s", "/c", "pnpm licenses list --prod --json"]
    : ["licenses", "list", "--prod", "--json"];
  return JSON.parse(execFileSync(
    command,
    args,
    {
      cwd: frontendRoot,
      encoding: "utf8",
    },
  ));
}

function main() {
  const releaseMode = process.argv.includes("--release");
  const classifications = classifyLicenseGroups(loadFrontendLicenseGroups());
  const summary = {
    mode: releaseMode ? "release" : "report",
    allow: classifications.filter((entry) => entry.decision === "allow"),
    review: classifications.filter((entry) => entry.decision === "review"),
    deny: classifications.filter((entry) => entry.decision === "deny"),
  };
  process.stdout.write(`${JSON.stringify(summary, null, 2)}\n`);

  if (releaseMode && (summary.review.length > 0 || summary.deny.length > 0)) {
    console.error("frontend license policy: release blocked by review/deny entries");
    process.exit(1);
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
