import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.REASONIX_TEST_BASE_URL || "http://127.0.0.1:5292";

export default defineConfig({
  testDir: "./e2e-runtime",
  testMatch: "**/*.spec.ts",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  outputDir: "../../output/playwright/runtime/results",
  reporter: [
    ["line"],
    ["json", { outputFile: "../../output/playwright/runtime/report.json" }],
  ],
  use: {
    ...devices["Desktop Chrome"],
    baseURL,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    video: "retain-on-failure",
  },
});
