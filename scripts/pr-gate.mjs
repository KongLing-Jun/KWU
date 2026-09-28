#!/usr/bin/env node

import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const ROOT_CORE_PREFIXES = [
  "internal/agent/",
  "internal/billing/",
  "internal/boot/",
  "internal/config/",
  "internal/control/",
  "internal/provider/",
  "internal/store/",
  "internal/userctx/",
];

const MAX_TARGETED_ROOT_CORE_PACKAGES = 4;
const MAX_TARGETED_ROOT_CORE_FILES = 16;
const MAX_TARGETED_ROOT_PACKAGES = 12;
const MAX_TARGETED_ROOT_FILES = 30;
const MAX_TARGETED_DESKTOP_PACKAGES = 5;
const MAX_TARGETED_DESKTOP_FILES = 20;

const GATE_FILES = new Set([
  "AGENTS.md",
  "Makefile",
  ".github/workflows/ci.yml",
  ".github/workflows/nightly-go-regression.yml",
  "scripts/pr-gate.mjs",
  "scripts/pr-gate.test.mjs",
  "scripts/check-markdown-links.mjs",
  "scripts/check-markdown-links.test.mjs",
  "scripts/testing-baseline-command.ps1",
  "scripts/testing-baseline-summary.mjs",
  "scripts/testing-baseline-summary.test.mjs",
  "scripts/testing-baseline.ps1",
  "scripts/testing-baseline-test.ps1",
  "scripts/testing-frontend-license-policy.mjs",
  "scripts/testing-frontend-license-policy.test.mjs",
  "scripts/testing-inventory.mjs",
  "scripts/testing-inventory.test.mjs",
  "scripts/testing-phase-gates.mjs",
  "scripts/testing-phase-gates.test.mjs",
  "scripts/testing-process-soak.ps1",
  "scripts/testing-process-soak-test.ps1",
  "scripts/testing-release-evidence-policy.mjs",
  "scripts/testing-release-evidence-policy.test.mjs",
  "scripts/testing-security-tool-policy.mjs",
  "scripts/testing-security-tool-policy.test.mjs",
  "scripts/testing-testdiag-guard.mjs",
  "scripts/testing-testdiag-guard.test.mjs",
  "scripts/testing-webview2-cdp-build.ps1",
  "scripts/testing-webview2-cdp-build-test.ps1",
  "scripts/testing-webview2-cdp.ps1",
  "scripts/testing-webview2-production-guard.mjs",
  "scripts/testing-webview2-production-guard.test.mjs",
  "scripts/testing-windows-uia-report.mjs",
  "scripts/testing-windows-uia-report.test.mjs",
  "scripts/testing-windows-native-smoke.ps1",
  "scripts/testing-windows-uia-tree.ps1",
]);

function normalizePath(path) {
  return path.replaceAll("\\", "/").replace(/^\.?\//, "");
}

function uniqueSorted(values) {
  return [...new Set(values)].sort();
}

function packagePattern(path, modulePrefix = "") {
  const relative = modulePrefix ? path.slice(modulePrefix.length) : path;
  const dir = dirname(relative).replaceAll("\\", "/");
  return dir === "." ? "." : `./${dir}`;
}

export function classifyFiles(inputFiles) {
  const files = uniqueSorted(inputFiles.map(normalizePath).filter(Boolean));
  const result = {
    files,
    rootGo: { mode: "none", packages: [] },
    desktopGo: { mode: "none", packages: [] },
    frontend: { mode: "none", tests: [], build: false },
    releaseContract: false,
    gateContract: false,
    reasons: [],
  };

  if (files.some((path) => GATE_FILES.has(path))) {
    result.gateContract = true;
    result.reasons.push("gate infrastructure changed; routing contract required");
  }

  const rootGoFiles = files.filter(
    (path) => path.endsWith(".go") && !path.startsWith("desktop/"),
  );
  const rootModuleFiles = files.filter((path) => path === "go.mod" || path === "go.sum");
  const rootPackages = uniqueSorted(rootGoFiles.map((path) => packagePattern(path)));
  const rootCoreFiles = rootGoFiles.filter((path) =>
    ROOT_CORE_PREFIXES.some((prefix) => path.startsWith(prefix)),
  );
  const rootCorePackages = uniqueSorted(rootCoreFiles.map((path) => packagePattern(path)));
  const rootCoreBroad =
    rootCorePackages.length > MAX_TARGETED_ROOT_CORE_PACKAGES ||
    rootCoreFiles.length > MAX_TARGETED_ROOT_CORE_FILES;

  if (
    rootModuleFiles.length > 0 ||
    rootCoreBroad ||
    rootPackages.length > MAX_TARGETED_ROOT_PACKAGES ||
    rootGoFiles.length > MAX_TARGETED_ROOT_FILES
  ) {
    result.rootGo.mode = "full";
    result.reasons.push("root Go dependency or broad change");
  } else if (rootPackages.length > 0 && result.rootGo.mode !== "full") {
    result.rootGo.mode = "targeted";
    result.rootGo.packages = rootPackages;
  }

  const desktopGoFiles = files.filter((path) => path.startsWith("desktop/") && path.endsWith(".go"));
  const desktopModuleChanged = files.some(
    (path) => path === "desktop/go.mod" || path === "desktop/go.sum",
  );
  const desktopPackages = uniqueSorted(
    desktopGoFiles.map((path) => packagePattern(path, "desktop/")),
  );

  if (
    desktopModuleChanged ||
    desktopPackages.length > MAX_TARGETED_DESKTOP_PACKAGES ||
    desktopGoFiles.length > MAX_TARGETED_DESKTOP_FILES
  ) {
    result.desktopGo.mode = "full";
    result.reasons.push("desktop Go dependency or broad change");
  } else if (desktopPackages.length > 0 && result.desktopGo.mode !== "full") {
    result.desktopGo.mode = "targeted";
    result.desktopGo.packages = desktopPackages;
  }

  const frontendFiles = files.filter((path) => path.startsWith("desktop/frontend/"));
  if (frontendFiles.length > 0) {
    result.frontend.mode = "build";
    result.frontend.build = true;
    result.reasons.push("frontend changes require production build only");
  }

  if (
    files.some(
      (path) =>
        path.startsWith("desktop/build/") ||
        path.startsWith("scripts/desktop-") ||
        path.startsWith("scripts/assert-windows-"),
    )
  ) {
    result.releaseContract = true;
  }

  return result;
}

function parseArgs(argv) {
  const options = {
    base: "origin/main-v3",
    head: "HEAD",
    filesFile: "",
    githubOutput: "",
    run: "",
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--base") options.base = argv[++index];
    else if (arg === "--head") options.head = argv[++index];
    else if (arg === "--files-file") options.filesFile = argv[++index];
    else if (arg === "--github-output") options.githubOutput = argv[++index];
    else if (arg === "--run") options.run = argv[++index];
    else if (arg === "--help") options.help = true;
    else throw new Error(`unknown argument: ${arg}`);
  }
  return options;
}

function changedFiles(options) {
  if (options.filesFile) {
    const data = readFileSync(options.filesFile);
    return data.toString("utf8").split(data.includes(0) ? "\0" : /\r?\n/);
  }
  const output = execFileSync(
    "git",
    ["diff", "--name-only", "-z", `${options.base}...${options.head}`],
    { cwd: repoRoot },
  );
  return output.toString("utf8").split("\0");
}

function run(command, args, cwd = repoRoot) {
  process.stdout.write(`\n> ${command} ${args.join(" ")}\n`);
  let executable = command;
  let executableArgs = args;
  if (process.platform === "win32" && command === "pnpm") {
    const commandParts = [command, ...args];
    if (commandParts.some((part) => !/^[A-Za-z0-9_./:@-]+$/.test(part))) {
      throw new Error("refusing unsafe pnpm argument on Windows");
    }
    executable = process.env.ComSpec || "cmd.exe";
    executableArgs = ["/d", "/s", "/c", commandParts.join(" ")];
  }
  const result = spawnSync(executable, executableArgs, {
    cwd,
    stdio: "inherit",
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

function runCommon(options, result) {
  run("go", ["run", "./scripts/code_guard"]);
  run("node", ["scripts/check-markdown-links.mjs"]);
  run("git", ["ls-files", "--error-unmatch", "scripts/code_guard_baseline.json"]);
  run("git", [
    "diff",
    "--exit-code",
    `${options.base}...${options.head}`,
    "--",
    "scripts/code_guard_baseline.json",
  ]);
  run("git", ["diff", "--exit-code", "--", "scripts/code_guard_baseline.json"]);
  run("git", ["diff", "--cached", "--exit-code", "--", "scripts/code_guard_baseline.json"]);
  run("git", ["diff", "--check", `${options.base}...${options.head}`]);
  if (result.gateContract) {
    run("node", ["scripts/pr-gate.test.mjs"]);
    run("node", ["scripts/check-markdown-links.test.mjs"]);
  }
}

function runGoScope(scope, cwd) {
  if (scope.mode === "none") return;
  const packages = scope.mode === "full" ? ["./..."] : scope.packages;
  run("go", ["test", ...packages, "-count=1"], cwd);
}

function runFrontend(scope, releaseContract) {
  if (scope.mode === "none" && !releaseContract) return;
  const cwd = resolve(repoRoot, "desktop/frontend");
  if (releaseContract) {
    run("pnpm", ["exec", "tsx", "src/__tests__/installer-contract.test.ts"], cwd);
  }
  if (scope.build) run("pnpm", ["build"], cwd);
}

function writeGithubOutput(path, result) {
  const output = [
    `root_go_mode=${result.rootGo.mode}`,
    `desktop_go_mode=${result.desktopGo.mode}`,
    `frontend_mode=${result.frontend.mode}`,
    `release_contract=${result.releaseContract}`,
  ].join("\n");
  writeFileSync(path, `${output}\n`, { flag: "a" });
}

function printSummary(result) {
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
}

function usage() {
  process.stdout.write(`Usage: node scripts/pr-gate.mjs [options]

Options:
  --base <ref>          Diff base (default: origin/main-v3)
  --head <ref>          Diff head (default: HEAD)
  --files-file <path>   Read NUL- or newline-delimited paths instead of Git
  --github-output <p>   Append classification outputs for GitHub Actions
  --run <scope>         common | root-go | desktop-go | frontend | all
`);
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.help) {
    usage();
    return;
  }
  const result = classifyFiles(changedFiles(options));
  printSummary(result);
  if (options.githubOutput) writeGithubOutput(options.githubOutput, result);
  if (!options.run) return;
  if (options.run === "common" || options.run === "all") runCommon(options, result);
  if (options.run === "root-go" || options.run === "all") runGoScope(result.rootGo, repoRoot);
  if (options.run === "desktop-go" || options.run === "all") {
    runGoScope(result.desktopGo, resolve(repoRoot, "desktop"));
  }
  if (options.run === "frontend" || options.run === "all") {
    runFrontend(result.frontend, result.releaseContract);
  }
}

const isMain = process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (isMain) {
  main().catch((error) => {
    process.stderr.write(`pr-gate: ${error.message}\n`);
    process.exit(1);
  });
}
