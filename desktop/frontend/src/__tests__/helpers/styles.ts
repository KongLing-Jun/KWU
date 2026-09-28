import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const helperDir = dirname(fileURLToPath(import.meta.url));
const entryPath = resolve(helperDir, "../../styles.css");
const importPattern = /@import\s+["']([^"']+)["'];/g;

export function loadStylesSource(): string {
  const entry = readFileSync(entryPath, "utf8");
  const sources: string[] = [];
  for (const match of entry.matchAll(importPattern)) {
    sources.push(readFileSync(resolve(dirname(entryPath), match[1]), "utf8"));
  }
  return sources.join("\n");
}
