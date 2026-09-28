// Run: tsx src/__tests__/transcript-virtualization.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { Virtualizer } from "@tanstack/react-virtual";
import { shouldVirtualizeTranscriptRows, transcriptVirtualHotRowOffset } from "../components/Transcript";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq(actual: unknown, expected: unknown, label: string) {
  ok(actual === expected, `${label}${actual === expected ? "" : `: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`}`);
}

console.log("\ntranscript virtualization");

eq(shouldVirtualizeTranscriptRows(9), false, "small transcripts keep direct DOM rendering");
eq(shouldVirtualizeTranscriptRows(10), true, "the virtualization boundary is inclusive");
eq(transcriptVirtualHotRowOffset(false, 2), 2, "regular question navigation includes warm and older-history prefix rows");
eq(transcriptVirtualHotRowOffset(true, 2), 0, "expert overview question navigation indexes its hot rows without regular prefixes");

{
  const keys = Array.from({ length: 48 }, (_, index) => `row-${index}`);
  const virtualizer = new Virtualizer({
    count: keys.length,
    getScrollElement: () => null,
    estimateSize: () => 180,
    getItemKey: (index) => keys[index],
    overscan: 3,
    initialRect: { width: 1_000, height: 800 },
    scrollToFn: () => {},
    observeElementRect: () => () => {},
    observeElementOffset: () => () => {},
  });
  const mountedRows = virtualizer.getVirtualItems();
  ok(mountedRows.length > 0 && mountedRows.length < keys.length, "the configured viewport mounts a bounded row subset");
  eq(virtualizer.getTotalSize(), 48 * 180, "the virtualizer preserves the complete estimated scroll range");
  eq(mountedRows[0]?.key, "row-0", "the virtual window preserves stable semantic row keys");
  virtualizer.resizeItem(0, 360);
  eq(virtualizer.getTotalSize(), (47 * 180) + 360, "dynamic row measurement updates the total scroll range");
}

{
  const here = dirname(fileURLToPath(import.meta.url));
  const appSource = readFileSync(resolve(here, "../App.tsx"), "utf8");
  const transcriptSource = readFileSync(resolve(here, "../components/Transcript.tsx"), "utf8");
  const virtualRowsSource = readFileSync(resolve(here, "../components/TranscriptVirtualRows.tsx"), "utf8");
  const sessionStyles = readFileSync(resolve(here, "../styles/session.css"), "utf8");
  eq(appSource.match(/<Transcript\b/g)?.length, 1, "App declares one ordinary chat Transcript surface");
  ok(
    appSource.includes('workspaceView === "projects" ? (')
      && !appSource.includes("renderProjectConversation={")
      && !appSource.includes('viewKey={`project:${sessionViewKey}`}'),
    "the Projects workspace does not own a second Transcript surface",
  );
  ok(!appSource.includes("PersistentViewCache"), "App does not retain hidden Transcript DOM trees");
  ok(
    virtualRowsSource.includes("getItemKey:")
      && virtualRowsSource.includes("measureElement")
      && virtualRowsSource.includes("overscan:")
      && transcriptSource.includes("scrollElement ? \"ready\" : \"pending\""),
    "Transcript wires stable keys, dynamic measurement, bounded overscan, and a ready scroll element",
  );
  ok(
    sessionStyles.includes(".transcript-virtual-list")
      && sessionStyles.includes(".transcript-virtual-row")
      && sessionStyles.includes("position: absolute"),
    "virtual rows use one relative height container with absolute viewport rows",
  );
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
