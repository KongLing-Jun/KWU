import AxeBuilder from "@axe-core/playwright";
import { test, expect } from "@playwright/test";
import { attachProjectBrowserEvidence, collectProjectBrowserEvidence, openProjectScenario } from "./workbuddy-project-evidence";

for (const scenario of ["anonymous", "empty", "detail", "stale", "archived", "board-move"] as const) {
  test(`Project ${scenario} has no serious or critical accessibility violations`, async ({ page }, testInfo) => {
    const evidence = collectProjectBrowserEvidence(page);
    await openProjectScenario(page, scenario);
    if (["detail", "archived", "board-move"].includes(scenario)) await page.locator(".cloud-projects__card").click();
    const results = await new AxeBuilder({ page }).include("[data-testid=workbuddy-project-fixture]").withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    await testInfo.attach(`axe-${scenario}`, { body: Buffer.from(JSON.stringify(results.violations, null, 2)), contentType: "application/json" });
    const blocking = results.violations.filter((violation) => violation.impact === "serious" || violation.impact === "critical");
    expect(blocking, "serious or critical axe violations").toEqual([]);
    await attachProjectBrowserEvidence(testInfo, evidence);
  });
}
