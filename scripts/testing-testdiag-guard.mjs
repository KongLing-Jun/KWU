#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const testdiagPackage = "kwu/internal/testdiag";

function packageLines(output) {
  return output.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
}

export function testdiagIsolationFindings(defaultOutput, taggedOutput) {
  const defaultPackages = packageLines(defaultOutput);
  const taggedPackages = packageLines(taggedOutput);
  const findings = [];

  if (defaultPackages.includes(testdiagPackage)) {
    findings.push("default package graph includes kwu/internal/testdiag");
  }
  if (!taggedPackages.includes(testdiagPackage)) {
    findings.push("testdiag-tagged package graph excludes kwu/internal/testdiag");
  }

  return findings;
}

function goList(args) {
  return execFileSync("go", ["list", "-f", "{{.ImportPath}}", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function main() {
  const findings = testdiagIsolationFindings(
    goList(["./..."]),
    goList(["-tags=testdiag", "./internal/testdiag"]),
  );

  if (findings.length > 0) {
    for (const finding of findings) {
      console.error(finding);
    }
    process.exit(1);
  }

  execFileSync(
    "go",
    ["test", "-tags=testdiag", "./internal/testdiag", "-count=1"],
    { cwd: repoRoot, stdio: "inherit" },
  );
  console.log("testdiag isolation guard: passed");
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
