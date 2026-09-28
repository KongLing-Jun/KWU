#!/usr/bin/env node

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
export const govulncheckVersion = "v1.6.0";

export function securityToolPolicyFindings(ciWorkflow) {
  const findings = [];
  if (/golang\.org\/x\/vuln\/cmd\/govulncheck@latest\b/.test(ciWorkflow)) {
    findings.push("govulncheck uses an unbounded @latest version");
  }
  if (
    !ciWorkflow.includes(
      `go install golang.org/x/vuln/cmd/govulncheck@${govulncheckVersion}`,
    )
  ) {
    findings.push(`govulncheck is not pinned to ${govulncheckVersion}`);
  }
  return findings;
}

function main() {
  const workflow = readFileSync(resolve(repoRoot, ".github/workflows/ci.yml"), "utf8");
  const findings = securityToolPolicyFindings(workflow);
  if (findings.length > 0) {
    for (const finding of findings) {
      console.error(finding);
    }
    process.exit(1);
  }
  console.log(`security tool policy: govulncheck pinned to ${govulncheckVersion}`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
