import { spawnSync } from "node:child_process";
import { readdirSync } from "node:fs";
import { join, relative } from "node:path";

const testsDir = join(process.cwd(), "src", "__tests__");
const tsxCli = join(process.cwd(), "node_modules", "tsx", "dist", "cli.mjs");
const tests = readdirSync(testsDir)
  .filter((name) => /\.test\.tsx?$/.test(name))
  .sort((a, b) => a.localeCompare(b));

if (tests.length === 0) {
  console.error("No frontend tests found.");
  process.exit(1);
}

for (const name of tests) {
  const testPath = relative(process.cwd(), join(testsDir, name)).replace(/\\/g, "/");
  console.log(`\n> tsx ${testPath}`);
  const result = spawnSync(process.execPath, [tsxCli, testPath], {
    stdio: "inherit",
  });
  if (result.error) {
    console.error(result.error.message);
    process.exit(1);
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}
