import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { patchOverlay, workspaceText } from "./prepare-wails-dev.mjs";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");

test("patches the verified dependency entry point without changing transport code", () => {
  const module = JSON.parse(execFileSync("go", ["list", "-m", "-json", "github.com/wailsapp/wails/v2"], { cwd: join(root, "desktop"), encoding: "utf8" }));
  const source = readFileSync(join(module.Dir, "internal/frontend/runtime/dev/main.js"));
  const patched = patchOverlay(source);
  assert.ok(patched.includes('import {mount} from "svelte";'));
  assert.ok(patched.includes("components.overlay = mount(Overlay, {"));
  assert.equal(patched.slice(patched.indexOf("let websocket")), source.toString().slice(source.toString().indexOf("let websocket")));
  assert.throws(() => patchOverlay(Buffer.from(patched)), /Unexpected Wails overlay source/);
});

test("rejects changed upstream source instead of silently emitting unpatched IPC", () => {
  assert.throws(() => patchOverlay(Buffer.from("components.overlay = new Overlay({});")), /Unexpected Wails overlay source/);
});

test("workspace keeps spaces quoted and does not replace the application module", () => {
  const text = workspaceText(join(root, "desktop space"), join(root, "patched space"), "1.25.0");
  assert.match(text, /use "[^\n]+desktop space"/);
  assert.match(text, /replace github\.com\/wailsapp\/wails\/v2 => "[^\n]+patched space"/);
  assert.equal(text.includes("replace kwu"), false);
  assert.throws(() => workspaceText(root, root, "1.25.0\nuse /other"), /Unexpected Go/);
});
