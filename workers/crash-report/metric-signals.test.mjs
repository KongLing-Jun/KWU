import { readFileSync } from "node:fs";

const desktop = readFileSync(new URL("../../desktop/metrics_app.go", import.meta.url), "utf8");
const worker = readFileSync(new URL("./src/index.ts", import.meta.url), "utf8");

const desktopSignals = new Set([...desktop.matchAll(/m\.inc\("([^"]+)"/g)].map((match) => match[1]));
const block = /const METRIC_SIGNALS = \[([\s\S]*?)\] as const;/.exec(worker)?.[1] ?? "";
const workerSignals = new Set([...block.matchAll(/"([^"]+)"/g)].map((match) => match[1]));

const missing = [...desktopSignals].filter((signal) => !workerSignals.has(signal)).sort();
if (missing.length > 0) {
  throw new Error(`worker METRIC_SIGNALS missing desktop metrics: ${missing.join(", ")}`);
}
