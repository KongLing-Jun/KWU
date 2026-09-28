#!/usr/bin/env node

import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

import {
  REQUIRED_EVIDENCE_ROLES,
  validateEvidenceBundle,
} from "./testing-release-evidence-policy.mjs";

const tempPrefix = resolve(tmpdir(), "reasonix-evidence-policy-");
const bundleRoot = mkdtempSync(tempPrefix);
const scriptPath = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "testing-release-evidence-policy.mjs",
);
if (!bundleRoot.startsWith(`${resolve(tmpdir())}${sep}`)) {
  throw new Error("temporary fixture escaped the system temp directory");
}

function digest(value) {
  return createHash("sha256").update(value).digest("hex");
}

try {
  const files = REQUIRED_EVIDENCE_ROLES.map((role) => {
    const path = `evidence/${role}.json`;
    const contents = JSON.stringify({ role, synthetic: true });
    mkdirSync(dirname(join(bundleRoot, path)), { recursive: true });
    writeFileSync(join(bundleRoot, path), contents);
    return { role, path, sha256: digest(contents) };
  });
  const manifest = {
    schemaVersion: 1,
    sourceCommit: "1".repeat(40),
    files,
  };

  const valid = validateEvidenceBundle(manifest, bundleRoot);
  assert.equal(valid.valid, true, valid.issues.join("\n"));
  assert.equal(valid.verifiedFiles, REQUIRED_EVIDENCE_ROLES.length);
  assert.match(valid.limitation, /Completeness and hashes only/);
  const manifestPath = join(bundleRoot, "bundle-manifest.json");
  writeFileSync(manifestPath, JSON.stringify(manifest));
  const validCLI = spawnSync(process.execPath, [scriptPath, manifestPath], {
    encoding: "utf8",
  });
  assert.equal(validCLI.status, 0, validCLI.stderr || validCLI.stdout);
  assert.equal(JSON.parse(validCLI.stdout).valid, true);

  const tampered = structuredClone(manifest);
  writeFileSync(join(bundleRoot, tampered.files[0].path), "tampered");
  const tamperedResult = validateEvidenceBundle(tampered, bundleRoot);
  assert.equal(tamperedResult.valid, false);
  assert(tamperedResult.issues.some((issue) => issue.includes("sha256 does not match")));
  const tamperedCLI = spawnSync(process.execPath, [scriptPath, manifestPath], {
    encoding: "utf8",
  });
  assert.equal(tamperedCLI.status, 1, tamperedCLI.stderr || tamperedCLI.stdout);
  assert.equal(JSON.parse(tamperedCLI.stdout).valid, false);
  writeFileSync(
    join(bundleRoot, tampered.files[0].path),
    readFileSync(join(bundleRoot, files[1].path), "utf8").replace(files[1].role, files[0].role),
  );
  writeFileSync(join(bundleRoot, files[0].path), JSON.stringify({ role: files[0].role, synthetic: true }));

  const unsafe = structuredClone(manifest);
  unsafe.files[0].path = "../outside.json";
  const unsafeResult = validateEvidenceBundle(unsafe, bundleRoot);
  assert.equal(unsafeResult.valid, false);
  assert(unsafeResult.issues.some((issue) => issue.includes("normalized relative path")));

  const missing = structuredClone(manifest);
  missing.files = missing.files.slice(1);
  const missingResult = validateEvidenceBundle(missing, bundleRoot);
  assert.equal(missingResult.valid, false);
  assert(
    missingResult.issues.some((issue) =>
      issue.includes(`required evidence role is missing: ${REQUIRED_EVIDENCE_ROLES[0]}`),
    ),
  );
} finally {
  rmSync(bundleRoot, { recursive: true, force: true });
}

process.stdout.write("release evidence policy: all checks passed\n");
