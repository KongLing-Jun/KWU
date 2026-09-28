#!/usr/bin/env node

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function nearestRank(values, percentile) {
  if (values.length === 0) return null;
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.max(0, Math.ceil(percentile * sorted.length) - 1);
  return sorted[index];
}

export function summarizeBaseline(resultSets) {
  const results = resultSets.flat();
  const grouped = new Map();
  for (const result of results) {
    const values = grouped.get(result.suite) ?? [];
    values.push(result);
    grouped.set(result.suite, values);
  }

  return [...grouped.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([suite, entries]) => {
      const elapsed = entries.map((entry) => entry.elapsed_ms);
      return {
        suite,
        samples: entries.length,
        pass: entries.filter((entry) => entry.result === "pass").length,
        fail: entries.filter((entry) => entry.result === "fail").length,
        timeout: entries.filter((entry) => entry.result === "timeout").length,
        elapsed_ms: {
          min: Math.min(...elapsed),
          p50: nearestRank(elapsed, 0.5),
          p95: nearestRank(elapsed, 0.95),
          max: Math.max(...elapsed),
        },
        observed_peaks: {
          working_set_bytes: Math.max(...entries.map((entry) => entry.peak_working_set_bytes)),
          handles: Math.max(...entries.map((entry) => entry.peak_handles)),
          processes: Math.max(...entries.map((entry) => entry.peak_processes)),
        },
      };
    });
}

export function parseBaselineJSON(text) {
  return JSON.parse(text.replace(/^\uFEFF/, ""));
}

function loadInputs(paths) {
  const commits = new Set();
  const resultSets = paths.map((path) => {
    const absolutePath = resolve(path);
    const environmentPath = resolve(dirname(absolutePath), "environment.json");
    const environment = parseBaselineJSON(readFileSync(environmentPath, "utf8"));
    commits.add(environment.source_commit);
    const parsed = parseBaselineJSON(readFileSync(absolutePath, "utf8"));
    return Array.isArray(parsed) ? parsed : [parsed];
  });
  if (commits.size !== 1) {
    throw new Error(`baseline inputs span multiple source commits: ${[...commits].join(", ")}`);
  }
  return { sourceCommit: [...commits][0], resultSets };
}

function main() {
  const paths = process.argv.slice(2);
  if (paths.length === 0) {
    throw new Error("usage: node scripts/testing-baseline-summary.mjs <results.json>...");
  }
  const { sourceCommit, resultSets } = loadInputs(paths);
  process.stdout.write(
    `${JSON.stringify(
      {
        schemaVersion: 1,
        sourceCommit,
        percentileMethod: "nearest-rank",
        suites: summarizeBaseline(resultSets),
      },
      null,
      2,
    )}\n`,
  );
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
