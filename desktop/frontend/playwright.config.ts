import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.PLAYWRIGHT_VITE_PORT || "5291");
const origin = `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  outputDir: "../../output/playwright/browser-smoke/results",
  reporter: [
    ["line"],
    ["json", { outputFile: "../../output/playwright/browser-smoke/report.json" }],
  ],
  use: {
    ...devices["Desktop Chrome"],
    baseURL: `${origin}/e2e/fixtures/browser-smoke.html`,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: `pnpm exec vite --host 127.0.0.1 --port ${port} --strictPort`,
    url: `${origin}/e2e/fixtures/browser-smoke.html`,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
