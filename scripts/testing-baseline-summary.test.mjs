import assert from "node:assert/strict";

import { parseBaselineJSON, summarizeBaseline } from "./testing-baseline-summary.mjs";

assert.deepEqual(parseBaselineJSON('\uFEFF{"result":"pass"}'), { result: "pass" });

const summary = summarizeBaseline([
  [
    {
      suite: "frontend-test",
      elapsed_ms: 100,
      result: "pass",
      peak_working_set_bytes: 1000,
      peak_handles: 10,
      peak_processes: 2,
    },
    {
      suite: "frontend-test",
      elapsed_ms: 200,
      result: "pass",
      peak_working_set_bytes: 2000,
      peak_handles: 20,
      peak_processes: 3,
    },
  ],
  [
    {
      suite: "frontend-test",
      elapsed_ms: 300,
      result: "timeout",
      peak_working_set_bytes: 1500,
      peak_handles: 15,
      peak_processes: 4,
    },
  ],
]);

assert.deepEqual(summary, [
  {
    suite: "frontend-test",
    samples: 3,
    pass: 2,
    fail: 0,
    timeout: 1,
    elapsed_ms: { min: 100, p50: 200, p95: 300, max: 300 },
    observed_peaks: {
      working_set_bytes: 2000,
      handles: 20,
      processes: 4,
    },
  },
]);

console.log("testing baseline summary: all checks passed");
