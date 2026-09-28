import { test, expect } from "@playwright/test";
import { attachProjectBrowserEvidence, collectProjectBrowserEvidence, openProjectScenario } from "./workbuddy-project-evidence";

const cases: ReadonlyArray<{
  scenario: "empty" | "detail" | "stale" | "archived" | "board-move";
  theme: "light" | "dark";
  width: number;
  height: number;
  zoom?: number;
}> = [
  { scenario: "empty", theme: "light", width: 1280, height: 900 },
  { scenario: "detail", theme: "light", width: 1280, height: 1000 },
  { scenario: "detail", theme: "light", width: 1132, height: 900 },
  { scenario: "stale", theme: "dark", width: 1280, height: 1000 },
  { scenario: "archived", theme: "dark", width: 820, height: 1100 },
  { scenario: "board-move", theme: "light", width: 1440, height: 1200 },
  { scenario: "detail", theme: "dark", width: 720, height: 1100, zoom: 125 },
  { scenario: "detail", theme: "light", width: 390, height: 844 },
];

for (const entry of cases) {
  test(`Project ${entry.scenario} ${entry.theme} ${entry.width}px${entry.zoom ? ` at ${entry.zoom}%` : ""}`, async ({ page }, testInfo) => {
    const evidence = collectProjectBrowserEvidence(page);
    await page.setViewportSize({ width: entry.width, height: entry.height });
    await openProjectScenario(page, entry.scenario, { theme: entry.theme, zoom: entry.zoom });
    if (["detail", "archived", "board-move"].includes(entry.scenario)) await page.locator(".cloud-projects__card").click();
    if (entry.scenario === "board-move") {
      await page.getByRole("button", { name: "事项", exact: true }).click();
      await page.locator('[data-work-item-view="plan"] .work-item-board__columns').waitFor();
      await page.getByRole("button", { name: "移动 Prepare brief" }).click();
      await page.getByRole("dialog", { name: "移动事项" }).waitFor();
    }
    await expect(page.locator(".cloud-projects")).toHaveScreenshot(`${entry.scenario}-${entry.theme}-${entry.width}${entry.zoom ? `-zoom-${entry.zoom}` : ""}.png`, {
      animations: "disabled",
      caret: "hide",
      mask: [page.locator("time")],
      scale: "css",
    });
    await attachProjectBrowserEvidence(testInfo, evidence);
  });
}
