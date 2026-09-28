import { readFileSync } from "node:fs";

function eq<T>(actual: T, expected: T, message: string) {
  if (actual !== expected) {
    throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
  }
}

const source = readFileSync(new URL("../components/CapabilitiesPanel.tsx", import.meta.url), "utf8");
const managedPolls = source.match(/const poll = window\.setInterval/g) ?? [];
eq(managedPolls.length, 1, "all managed connectors share one authorization poll implementation");
eq(
  source.includes("reloadManagedConnectors(tabID).catch((error) =>")
    && source.includes("isConnectorTargetUnavailable(error)")
    && source.includes("window.clearInterval(poll)")
    && source.includes("}, 1000);"),
  true,
  "the managed connector poll captures its tab, consumes stale-target failures, and is always cancellable",
);

console.log("managed connector poll safety passed");
