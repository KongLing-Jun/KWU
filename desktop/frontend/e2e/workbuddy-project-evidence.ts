import { expect, type Page, type TestInfo } from "@playwright/test";

export interface ProjectBrowserEvidence {
  consoleErrors: string[];
  pageErrors: string[];
  failedRequests: string[];
}

export function collectProjectBrowserEvidence(page: Page): ProjectBrowserEvidence {
  const evidence: ProjectBrowserEvidence = { consoleErrors: [], pageErrors: [], failedRequests: [] };
  page.on("console", (message) => {
    if (message.type() === "error") evidence.consoleErrors.push(message.text());
  });
  page.on("pageerror", (error) => evidence.pageErrors.push(error.message));
  page.on("requestfailed", (request) => evidence.failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText || "unknown"}`));
  return evidence;
}

export async function openProjectScenario(page: Page, scenario: string, options: { theme?: "light" | "dark"; zoom?: number } = {}) {
  await page.addInitScript(() => {
    Date.now = () => new Date("2026-08-14T10:00:00.000Z").getTime();
  });
  const query = new URLSearchParams({ scenario, theme: options.theme || "light", zoom: String(options.zoom || 100) });
  await page.goto(`/e2e/fixtures/workbuddy-project.html?${query}`);
  const fixture = page.locator("[data-testid=workbuddy-project-fixture]");
  await expect(fixture).toBeVisible({ timeout: 3_000 });
  await page.evaluate(() => document.fonts.ready);
  await page.addStyleTag({ content: "*, *::before, *::after { animation-duration: 0s !important; transition-duration: 0s !important; caret-color: transparent !important; }" });
  return fixture;
}

export async function attachProjectBrowserEvidence(testInfo: TestInfo, evidence: ProjectBrowserEvidence) {
  await testInfo.attach("workbuddy-project-browser-evidence", {
    body: Buffer.from(JSON.stringify(evidence, null, 2)),
    contentType: "application/json",
  });
  expect(evidence.pageErrors, "uncaught page errors").toEqual([]);
  expect(evidence.consoleErrors, "browser console errors").toEqual([]);
  expect(evidence.failedRequests, "failed browser requests").toEqual([]);
}
