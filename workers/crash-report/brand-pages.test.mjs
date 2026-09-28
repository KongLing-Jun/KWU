import { readFileSync } from "node:fs";

const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");
const shell = read("./src/shell.ts");
const pages = [shell, read("./src/auth_pages.ts"), read("./src/admin.ts"), read("./src/stats.ts")].join("\n");

for (const oldBrand of ["KWU ·", ">KWU<", "https://kwu.example", ">S</text>"]) {
  if (pages.includes(oldBrand)) throw new Error(`visible legacy brand remains: ${oldBrand}`);
}

for (const currentBrand of ["Clouvia · Sign in", "Clouvia · Register", "Clouvia · Account", "Clouvia · Users", "Clouvia · Audit", "Clouvia · Crash & Telemetry"]) {
  if (!pages.includes(currentBrand)) throw new Error(`current brand missing: ${currentBrand}`);
}

if (!shell.includes('href="/"') || !shell.includes(">Clouvia</a>") || !shell.includes(">C</text>")) {
  throw new Error("Clouvia header brand is incomplete");
}
