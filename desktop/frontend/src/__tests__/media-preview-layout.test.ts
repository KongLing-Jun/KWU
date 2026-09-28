// Run: tsx src/__tests__/media-preview-layout.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const css = readFileSync(resolve(here, "../styles/session.css"), "utf8");

let failed = 0;
function ok(condition: boolean, label: string) {
  process.stdout.write(`  ${condition ? "PASS" : "FAIL"}  ${label}\n`);
  if (!condition) failed += 1;
}

console.log("\nmedia preview intrinsic sizing");

ok(
  /\.tool-media\s*\{[^}]*display:\s*flex;[^}]*flex-wrap:\s*wrap;/s.test(css),
  "media collection uses content-sized flex items",
);
ok(
  /\.tool-media__item img,\s*\.tool-media__item video\s*\{[^}]*width:\s*auto;[^}]*max-width:\s*100%;[^}]*height:\s*auto;/s.test(css),
  "image and video preserve intrinsic dimensions until the chat width is exceeded",
);
ok(
  /\.tool-media__item--video video\s*\{[^}]*aspect-ratio:\s*auto;/s.test(css),
  "video does not use a forced aspect ratio",
);
ok(
  !/\.tool-media__item img,\s*\.tool-media__item video\s*\{[^}]*max-height:/s.test(css),
  "media height is not capped by an arbitrary viewport limit",
);

console.log(`\n${4 - failed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
