#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const isWindows = process.platform === "win32";

const executable = {
  go: isWindows ? "go.exe" : "go",
  node: isWindows ? "node.exe" : "node",
  pnpm: isWindows ? "pnpm.cmd" : "pnpm",
  powershell: isWindows ? "powershell.exe" : "pwsh",
};

export const PHASE_GATES = Object.freeze([
  {
    phase: "phase1",
    id: "go-core-client-contracts",
    command: executable.go,
    args: [
      "test",
      "./internal/agent",
      "./internal/modelbilling",
      "./internal/automation",
      "./internal/jobs",
      "./internal/cloud",
      "./internal/netclient",
      "./internal/cloudregistration",
      "./internal/installsource",
      "-count=1",
    ],
  },
  {
    phase: "phase1",
    id: "desktop-updater-contracts",
    command: executable.go,
    workingDirectory: "desktop",
    args: [
      "test",
      ".",
      "-run",
      "Test(NormalizeVersion|Evaluate|UpdatePolicyFor|FetchManifest|DownloadUpdate|SaveCachedUpdate|CachedUpdate|CheckSHA256|ExtractBinary|DownloadRecovers|DownloadGivesUp|RetryTransient|DownloadResumes|DownloadFallsBack)",
      "-count=1",
    ],
  },
  {
    phase: "phase1",
    id: "desktop-update-package",
    command: executable.go,
    workingDirectory: "desktop",
    args: ["test", "./internal/update", "-count=1"],
  },
  {
    phase: "phase2",
    id: "react-contracts",
    command: executable.pnpm,
    args: ["--dir", "desktop/frontend", "test:vitest"],
  },
  {
    phase: "phase2",
    id: "browser-smoke",
    command: executable.pnpm,
    args: ["--dir", "desktop/frontend", "test:browser"],
  },
  {
    phase: "phase2",
    id: "webview2-build-contract",
    command: executable.powershell,
    args: [
      "-NoProfile",
      "-ExecutionPolicy",
      "Bypass",
      "-File",
      "scripts/testing-webview2-cdp-build-test.ps1",
    ],
  },
  {
    phase: "phase2",
    id: "webview2-production-source-guard",
    command: executable.node,
    args: ["scripts/testing-webview2-production-guard.test.mjs"],
  },
  {
    phase: "phase2",
    id: "webview2-production-source-scan",
    command: executable.node,
    args: ["scripts/testing-webview2-production-guard.mjs"],
  },
  {
    phase: "phase2",
    id: "windows-uia-report-contract",
    command: executable.node,
    args: ["scripts/testing-windows-uia-report.test.mjs"],
  },
  {
    phase: "phase6",
    id: "testing-inventory-contract",
    command: executable.node,
    args: ["scripts/testing-inventory.test.mjs"],
  },
  {
    phase: "phase6",
    id: "phase-gate-contract",
    command: executable.node,
    args: ["scripts/testing-phase-gates.test.mjs"],
  },
  {
    phase: "phase6",
    id: "pr-gate-routing-contract",
    command: executable.node,
    args: ["scripts/pr-gate.test.mjs"],
  },
]);

export const QUALIFIED_GATES = Object.freeze([
  {
    phase: "phase2",
    id: "live-webview2-cdp",
    requiredInputs: ["test-only Wails executable produced by testing-webview2-cdp-build.ps1"],
    command:
      "powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/testing-webview2-cdp.ps1 -Executable <test-executable>",
  },
  {
    phase: "phase2",
    id: "live-windows-uia",
    requiredInputs: ["running desktop PID", "matching desktop executable"],
    command:
      "powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/testing-windows-uia-tree.ps1 -ProcessId <pid> -Executable <desktop-executable>",
  },
  {
    phase: "phase2",
    id: "final-installer-diagnostic-isolation",
    requiredInputs: ["final NSIS installer built from the source commit", "NSIS makensis tool"],
    command:
      "build installer, install in an isolated test profile, launch it, then run the production guard and loopback diagnostic-port observation",
  },
]);

export function gateManifest() {
  return {
    schemaVersion: 1,
    sourceCommitCommand: "git rev-parse HEAD",
    automatic: PHASE_GATES,
    environmentQualified: QUALIFIED_GATES,
  };
}

function parseArguments(args) {
  if (args.length === 0 || args[0] === "--list") return { list: true };
  if (args[0] !== "--run" || args.length !== 2) {
    throw new Error(
      "usage: node scripts/testing-phase-gates.mjs [--list | --run phase1|phase2|phase6|all]",
    );
  }
  const phase = args[1].toLowerCase();
  if (!["phase1", "phase2", "phase6", "all"].includes(phase)) {
    throw new Error(`unknown phase gate: ${args[1]}`);
  }
  return { list: false, phase };
}

function quoteCmdArgument(value) {
  const text = String(value);
  return /^[A-Za-z0-9_./:\\-]+$/.test(text) ? text : `"${text.replaceAll('"', '""')}"`;
}

export function commandInvocation(
  gate,
  platform = process.platform,
  comspec = process.env.ComSpec ?? "cmd.exe",
) {
  if (platform === "win32" && gate.command.toLowerCase().endsWith(".cmd")) {
    return {
      command: comspec,
      args: ["/d", "/s", "/c", [gate.command, ...gate.args].map(quoteCmdArgument).join(" ")],
    };
  }
  return { command: gate.command, args: gate.args };
}

function runGate(gate) {
  process.stdout.write(`\n[${gate.phase}] ${gate.id}\n`);
  const invocation = commandInvocation(gate);
  const result = spawnSync(invocation.command, invocation.args, {
    cwd: gate.workingDirectory ? resolve(repoRoot, gate.workingDirectory) : repoRoot,
    env: process.env,
    stdio: "inherit",
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`${gate.id} failed with exit code ${result.status ?? "unknown"}`);
  }
}

function main() {
  const options = parseArguments(process.argv.slice(2));
  if (options.list) {
    process.stdout.write(`${JSON.stringify(gateManifest(), null, 2)}\n`);
    return;
  }
  const selected = PHASE_GATES.filter(
    (gate) => options.phase === "all" || gate.phase === options.phase,
  );
  for (const gate of selected) runGate(gate);
  process.stdout.write(`\n${options.phase} automatic gates: passed (${selected.length})\n`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    main();
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}
