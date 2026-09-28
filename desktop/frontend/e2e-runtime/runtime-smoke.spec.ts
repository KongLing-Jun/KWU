import AxeBuilder from "@axe-core/playwright";

import { test, expect } from "./runtime-evidence";

test.beforeEach(async ({ page }) => {
  await page.goto("/", { waitUntil: "domcontentloaded" });
  await expect(page).toHaveTitle("云智企办");
  await expect(page.getByRole("heading", { name: "你的办公超能力" })).toBeVisible();
});

test("renders the primary user entry points without browser errors", async ({ page }) => {
  const pageErrors: string[] = [];
  const consoleErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });

  await expect(page.getByRole("button", { name: "新建对话" })).toBeVisible();
  await expect(page.getByRole("button", { name: "自动化" })).toBeVisible();
  await expect(page.getByRole("button", { name: "模型广场" })).toBeVisible();

  const composer = page.getByRole("textbox");
  await expect(composer).toHaveCount(1);
  await expect(composer).toBeEditable();

  expect(pageErrors).toEqual([]);
  expect(consoleErrors).toEqual([]);
});

test("@audit has no serious or critical automated accessibility violations", async ({ page }, testInfo) => {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();
  const blockingViolations = results.violations.filter(
    (violation) => violation.impact === "serious" || violation.impact === "critical",
  );

  await testInfo.attach("runtime-axe-results.json", {
    body: Buffer.from(JSON.stringify(results, null, 2)),
    contentType: "application/json",
  });

  if (blockingViolations.length > 0) {
    const summary = blockingViolations
      .map((violation) => `${violation.id}: ${violation.nodes.length} affected node(s)`)
      .join(", ");
    throw new Error(`Serious or critical accessibility violations found: ${summary}`);
  }
});
