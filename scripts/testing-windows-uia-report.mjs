#!/usr/bin/env node

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

function uniqueSortedNumbers(values) {
  return [...new Set(values)].sort((left, right) => left - right);
}

export function uiaReportFindings(report, options = {}) {
  const findings = [];
  const nodes = Array.isArray(report?.nodes) ? report.nodes : [];

  if (report?.schemaVersion !== 1) {
    findings.push("schemaVersion must be 1");
  }
  if (nodes.length === 0) {
    findings.push("nodes must be a non-empty array");
    return findings;
  }
  if (report.nodeCount !== nodes.length) {
    findings.push("nodeCount does not match nodes.length");
  }
  if (options.expectedProcessID !== undefined && report.processID !== options.expectedProcessID) {
    findings.push("processID does not match the expected process");
  }
  if (
    options.expectedExecutable !== undefined
    && resolve(report.executable) !== resolve(options.expectedExecutable)
  ) {
    findings.push("executable does not match the expected path");
  }
  if (options.requireComplete && report.truncated !== false) {
    findings.push("UIA tree is truncated");
  }
  if (options.requireRedacted && report.namesIncluded !== false) {
    findings.push("UIA report includes element names");
  }

  for (const [index, node] of nodes.entries()) {
    if (node.index !== index) {
      findings.push(`node ${index} has a non-sequential index`);
    }
    if (index === 0) {
      if (node.parentIndex !== null || node.depth !== 0) {
        findings.push("root node must have parentIndex null and depth 0");
      }
      if (node.processID !== report.processID) {
        findings.push("root node processID does not match report.processID");
      }
    } else {
      if (!Number.isInteger(node.parentIndex) || node.parentIndex < 0 || node.parentIndex >= index) {
        findings.push(`node ${index} has an invalid parentIndex`);
      } else if (node.depth !== nodes[node.parentIndex].depth + 1) {
        findings.push(`node ${index} depth does not follow its parent`);
      }
    }
    if (options.requireRedacted && Object.hasOwn(node, "name")) {
      findings.push(`node ${index} contains a name in a redacted report`);
    }
  }

  const maximumDepth = Math.max(...nodes.map((node) => node.depth));
  if (report.maximumDepth !== maximumDepth) {
    findings.push("maximumDepth does not match node depths");
  }

  const processIDs = uniqueSortedNumbers(nodes.map((node) => node.processID));
  if (JSON.stringify(report.processIDs) !== JSON.stringify(processIDs)) {
    findings.push("processIDs do not match node process IDs");
  }

  const controlTypeCount = Array.isArray(report.controlTypes)
    ? report.controlTypes.reduce((total, entry) => total + entry.count, 0)
    : 0;
  if (controlTypeCount !== nodes.length) {
    findings.push("controlTypes counts do not add up to nodeCount");
  }

  return findings;
}

function main() {
  const [reportPath, expectedProcessID, expectedExecutable] = process.argv.slice(2);
  if (!reportPath || !expectedProcessID || !expectedExecutable) {
    console.error(
      "usage: node scripts/testing-windows-uia-report.mjs "
      + "<report.json> <expected-process-id> <expected-executable>",
    );
    process.exit(2);
  }

  const report = JSON.parse(readFileSync(resolve(reportPath), "utf8").replace(/^\uFEFF/, ""));
  const findings = uiaReportFindings(report, {
    expectedProcessID: Number(expectedProcessID),
    expectedExecutable,
    requireComplete: true,
    requireRedacted: true,
  });
  if (findings.length > 0) {
    for (const finding of findings) {
      console.error(finding);
    }
    process.exit(1);
  }
  console.log(
    `Windows UIA report: passed (${report.nodeCount} nodes, depth ${report.maximumDepth})`,
  );
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
