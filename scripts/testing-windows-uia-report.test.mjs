import assert from "node:assert/strict";

import { uiaReportFindings } from "./testing-windows-uia-report.mjs";

const validReport = {
  schemaVersion: 1,
  processID: 100,
  executable: "C:\\fixture\\app.exe",
  namesIncluded: false,
  nodeCount: 3,
  maximumDepth: 2,
  processIDs: [100, 200],
  controlTypes: [
    { controlType: "Window", count: 1 },
    { controlType: "Pane", count: 2 },
  ],
  truncated: false,
  nodes: [
    {
      index: 0,
      parentIndex: null,
      depth: 0,
      processID: 100,
      controlType: "Window",
      namePresent: true,
    },
    {
      index: 1,
      parentIndex: 0,
      depth: 1,
      processID: 200,
      controlType: "Pane",
      namePresent: false,
    },
    {
      index: 2,
      parentIndex: 1,
      depth: 2,
      processID: 200,
      controlType: "Pane",
      namePresent: true,
    },
  ],
};

assert.deepEqual(
  uiaReportFindings(validReport, {
    expectedProcessID: 100,
    expectedExecutable: "C:\\fixture\\app.exe",
    requireComplete: true,
    requireRedacted: true,
  }),
  [],
);

const unsafeReport = structuredClone(validReport);
unsafeReport.namesIncluded = true;
unsafeReport.truncated = true;
unsafeReport.nodes[1].name = "sensitive page text";
unsafeReport.nodes[2].parentIndex = 9;

assert.deepEqual(
  uiaReportFindings(unsafeReport, {
    requireComplete: true,
    requireRedacted: true,
  }),
  [
    "UIA tree is truncated",
    "UIA report includes element names",
    "node 1 contains a name in a redacted report",
    "node 2 has an invalid parentIndex",
  ],
);

console.log("Windows UIA report tests: passed");
