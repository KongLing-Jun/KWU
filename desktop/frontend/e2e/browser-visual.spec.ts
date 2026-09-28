import { test, expect } from "./browser-evidence";

test("keeps the ready-state shell visually stable", async ({ page }) => {
  await page.goto("?scenario=ready");
  await expect(page.getByRole("heading", { level: 1, name: "Reasonix browser smoke" })).toBeVisible();

  await expect(page).toHaveScreenshot("ready-state-shell.png", {
    animations: "disabled",
    fullPage: true,
  });
});
