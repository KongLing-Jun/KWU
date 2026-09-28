import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { collectMarkdownIssues } from "./check-markdown-links.mjs";

const root = mkdtempSync(join(tmpdir(), "clouvia-markdown-links-"));
try {
  mkdirSync(join(root, "docs", "guide"), { recursive: true });
  writeFileSync(join(root, "docs", "target.md"), "# Target\n");
  writeFileSync(
    join(root, "docs", "guide", "index.md"),
    [
      "[valid](../target.md#target)",
      "[external](https://example.com/docs)",
      "[missing](../missing.md)",
      "[machine](D:/private/worktree/file.go#L10)",
    ].join("\n"),
  );
  mkdirSync(join(root, "docs", "external-snapshot", "nested"), { recursive: true });
  writeFileSync(join(root, "docs", "external-snapshot", ".markdown-link-check-ignore"), "\n");
  writeFileSync(
    join(root, "docs", "external-snapshot", "nested", "index.md"),
    "[missing](references/not-exported.md)\n",
  );

  assert.deepEqual(
    collectMarkdownIssues(root, [
      "docs/guide/index.md",
      "docs/external-snapshot/nested/index.md",
    ]),
    [
      {
        file: "docs/guide/index.md",
        line: 3,
        target: "../missing.md",
        reason: "missing local target",
      },
      {
        file: "docs/guide/index.md",
        line: 4,
        target: "D:/private/worktree/file.go#L10",
        reason: "absolute local path",
      },
    ],
  );
} finally {
  rmSync(root, { recursive: true, force: true });
}

process.stdout.write("check-markdown-links tests: OK\n");
