#!/usr/bin/env node

import { createHash } from "node:crypto";
import { existsSync, readFileSync, statSync } from "node:fs";
import { dirname, isAbsolute, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

export const REQUIRED_EVIDENCE_ROLES = Object.freeze([
  "installer-windows-amd64",
  "installer-windows-arm64",
  "authenticode-report-windows-amd64",
  "authenticode-report-windows-arm64",
  "minisign-windows-amd64",
  "minisign-windows-arm64",
  "sbom-installer-windows-amd64",
  "sbom-installer-windows-arm64",
  "sbom-app-windows-amd64",
  "sbom-app-windows-arm64",
  "vulnerability-report",
  "license-report",
  "test-report",
  "approval-record",
]);

function safeRelativePath(value) {
  if (
    typeof value !== "string" ||
    value.length === 0 ||
    isAbsolute(value) ||
    value.includes("\\") ||
    value.includes("?") ||
    value.includes("#")
  ) {
    return false;
  }
  const segments = value.split("/");
  return segments.every((segment) => segment !== "" && segment !== "." && segment !== "..");
}

function sha256(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

export function validateEvidenceBundle(manifest, bundleRoot) {
  const root = resolve(bundleRoot);
  const rootPrefix = `${root}${sep}`;
  const issues = [];
  const roles = new Set();
  let verifiedFiles = 0;

  if (!manifest || typeof manifest !== "object" || Array.isArray(manifest)) {
    return {
      valid: false,
      issues: ["manifest must be a JSON object"],
      verifiedFiles,
      requiredRoles: REQUIRED_EVIDENCE_ROLES,
    };
  }
  if (manifest.schemaVersion !== 1) {
    issues.push("schemaVersion must equal 1");
  }
  if (typeof manifest.sourceCommit !== "string" || !/^[0-9a-f]{40}$/.test(manifest.sourceCommit)) {
    issues.push("sourceCommit must be a lowercase 40-character Git SHA");
  }
  if (!Array.isArray(manifest.files)) {
    issues.push("files must be an array");
  } else {
    const paths = new Set();
    for (const [index, file] of manifest.files.entries()) {
      const label = `files[${index}]`;
      if (!file || typeof file !== "object" || Array.isArray(file)) {
        issues.push(`${label} must be an object`);
        continue;
      }
      if (typeof file.role !== "string" || file.role.length === 0) {
        issues.push(`${label}.role must be non-empty`);
      } else if (roles.has(file.role)) {
        issues.push(`${label}.role duplicates ${file.role}`);
      } else {
        roles.add(file.role);
      }
      if (!safeRelativePath(file.path)) {
        issues.push(`${label}.path must be a normalized relative path`);
        continue;
      }
      if (paths.has(file.path)) {
        issues.push(`${label}.path duplicates ${file.path}`);
        continue;
      }
      paths.add(file.path);
      if (typeof file.sha256 !== "string" || !/^[0-9a-f]{64}$/.test(file.sha256)) {
        issues.push(`${label}.sha256 must be lowercase SHA-256`);
        continue;
      }
      const resolvedPath = resolve(root, file.path);
      if (!resolvedPath.startsWith(rootPrefix)) {
        issues.push(`${label}.path escapes the bundle root`);
        continue;
      }
      if (!existsSync(resolvedPath) || !statSync(resolvedPath).isFile()) {
        issues.push(`${label}.path is missing or is not a file`);
        continue;
      }
      if (sha256(resolvedPath) !== file.sha256) {
        issues.push(`${label}.sha256 does not match ${file.path}`);
        continue;
      }
      verifiedFiles += 1;
    }
  }

  for (const role of REQUIRED_EVIDENCE_ROLES) {
    if (!roles.has(role)) {
      issues.push(`required evidence role is missing: ${role}`);
    }
  }

  return {
    valid: issues.length === 0,
    issues,
    verifiedFiles,
    requiredRoles: REQUIRED_EVIDENCE_ROLES,
    limitation:
      "Completeness and hashes only; cryptographic validity, vulnerability disposition, license approval, and reviewer authorization require their dedicated gates.",
  };
}

function main() {
  const manifestPath = process.argv[2];
  if (!manifestPath) {
    process.stderr.write("usage: node scripts/testing-release-evidence-policy.mjs <bundle-manifest.json>\n");
    process.exitCode = 2;
    return;
  }
  const absoluteManifest = resolve(manifestPath);
  let manifest;
  try {
    manifest = JSON.parse(readFileSync(absoluteManifest, "utf8"));
  } catch (error) {
    process.stderr.write(`release evidence policy: ${error.message}\n`);
    process.exitCode = 2;
    return;
  }
  const result = validateEvidenceBundle(manifest, dirname(absoluteManifest));
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  if (!result.valid) process.exitCode = 1;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
