import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { chmodSync, cpSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const moduleName = "github.com/wailsapp/wails/v2";
const supportedVersion = "v2.15.0";
const inputs = {
  "main.js": "d1986f80f9228e88f9bf7f221abc68e6e223570daaa26a1a67204a49bbfe9ed2",
  "build.js": "091c5f1fe2fa43b774bd25dd4cacb5a11b196bbde8a0c3336b71c7289730a0d1",
  "package-lock.json": "b354b096cfcab8c4d0f8410f32b3edda76a0f0e92bf40b944535ee8720555054",
};
const nativeAssets = ["ipc.js", "runtime_debug_desktop.js", "runtime_prod_desktop.js"];
const sha256 = (value) => createHash("sha256").update(value).digest("hex");

export function patchOverlay(source) {
  if (sha256(source) !== inputs["main.js"]) throw new Error("Unexpected Wails overlay source; review the upstream change before updating this patch.");
  return source.toString("utf8")
    .replace('import Overlay from "./Overlay.svelte";', 'import Overlay from "./Overlay.svelte";\nimport {mount} from "svelte";')
    .replace("components.overlay = new Overlay({", "components.overlay = mount(Overlay, {");
}

export function workspaceText(desktopRoot, patchedModule, goVersion) {
  const quote = (path) => JSON.stringify(resolve(path).replaceAll("\\", "/"));
  if (!/^1\.\d+(\.\d+)?$/.test(goVersion)) throw new Error("Unexpected Go language version");
  return `go ${goVersion}\n\nuse ${quote(desktopRoot)}\n\nreplace ${moduleName} => ${quote(patchedModule)}\n`;
}

function makeWritable(directory) {
  chmodSync(directory, statSync(directory).mode | 0o700);
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) makeWritable(path);
    else if (entry.isFile()) chmodSync(path, statSync(path).mode | 0o200);
  }
}

export function prepareWailsDev(desktopRoot) {
  desktopRoot = resolve(desktopRoot);
  const run = (command, args, cwd = desktopRoot) => execFileSync(command, args, {
    cwd, encoding: "utf8", stdio: ["ignore", "pipe", "inherit"],
  }).trim();
  const existingWorkspace = run("go", ["env", "GOWORK"]);
  if (existingWorkspace && existingWorkspace !== "off") throw new Error("An existing Go workspace is active; run preparation with GOWORK=off to avoid replacing it implicitly.");
  const module = JSON.parse(run("go", ["list", "-m", "-json", moduleName]));
  if (module.Version !== supportedVersion || module.Replace) throw new Error(`Wails dev overlay patch requires unreplaced ${supportedVersion}; review the patch for the selected dependency.`);
  const relativeRuntime = "internal/frontend/runtime";
  const originalRuntime = join(module.Dir, relativeRuntime);
  for (const [file, expected] of Object.entries(inputs)) {
    if (sha256(readFileSync(join(originalRuntime, "dev", file))) !== expected) throw new Error(`Unexpected Wails dev/${file}; refusing to patch unverified input.`);
  }

  const output = join(desktopRoot, "..", ".codex-run", "wails-dev");
  mkdirSync(output, { recursive: true });
  // Each watcher owns its workspace and dependency copy, including hot rebuilds.
  const staging = mkdtempSync(join(output, "runtime-"));
  const patchedModule = join(staging, "wails");
  cpSync(module.Dir, patchedModule, { recursive: true });
  makeWritable(patchedModule);
  const runtime = join(patchedModule, relativeRuntime);
  const dev = join(runtime, "dev");
  writeFileSync(join(dev, "main.js"), patchOverlay(readFileSync(join(dev, "main.js"))));

  const npm = (args) => {
    // Arguments are fixed below; PowerShell handles npm.cmd on Windows.
    const command = process.platform === "win32" ? "pwsh.exe" : "npm";
    const commandArgs = process.platform === "win32" ? ["-NoProfile", "-Command", `npm.cmd ${args.join(" ")}`] : args;
    return run(command, commandArgs, dev);
  };
  const npmVersion = npm(["--version"]);
  process.stderr.write(`Preparing isolated Wails ${supportedVersion} browser IPC...\n`);
  process.stderr.write(npm(["ci", "--no-audit", "--no-fund"]) + "\n");
  process.stderr.write(npm(["run", "build:dev"]) + "\n");
  for (const file of [...nativeAssets, "dev/package-lock.json"]) {
    if (sha256(readFileSync(join(runtime, file))) !== sha256(readFileSync(join(originalRuntime, file)))) throw new Error(`Dev build unexpectedly changed ${file}`);
  }
  const goVersion = JSON.parse(run("go", ["list", "-m", "-json"])).GoVersion;
  const workFile = join(staging, "go.work");
  writeFileSync(workFile, workspaceText(desktopRoot, patchedModule, goVersion));
  const report = {
    version: supportedVersion, desktopRoot, sourceModule: module.Dir, patchedModule, workFile,
    nodeVersion: process.version, npmVersion, inputs,
    browserIPCSHA256: sha256(readFileSync(join(runtime, "ipc_websocket.js"))),
    nativeAssets: Object.fromEntries(nativeAssets.map((file) => [file, sha256(readFileSync(join(runtime, file)))])),
  };
  writeFileSync(join(staging, "build-report.json"), JSON.stringify(report, null, 2) + "\n");
  return workFile;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    console.log(prepareWailsDev(process.argv[2] ?? join(dirname(fileURLToPath(import.meta.url)), "..", "desktop")));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
