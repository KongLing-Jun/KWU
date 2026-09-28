import AxeBuilder from "@axe-core/playwright";

import { test, expect } from "./browser-evidence";

test("has no automatically detectable WCAG A or AA violations", async ({ page }, testInfo) => {
  await page.goto("?scenario=ready");

  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();

  await testInfo.attach("axe-results.json", {
    body: Buffer.from(JSON.stringify(results, null, 2)),
    contentType: "application/json",
  });

  expect(results.violations).toEqual([]);
});
