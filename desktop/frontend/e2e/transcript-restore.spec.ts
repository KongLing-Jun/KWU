import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  page.on("pageerror", (error) => { throw error; });
});

for (const viewport of [{ width: 1280, height: 800 }, { width: 390, height: 844 }]) {
  test(`clamped short transcript stays readable and preserves its anchor ${viewport.width}`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.goto("/e2e/fixtures/transcript-restore.html");
    await expect(page.locator(".transcript__restore-status")).toBeVisible();
    await expect(page.locator(".msg--assistant").first()).toBeVisible();
    await expect(page.locator(".transcript-virtual-list")).toHaveCount(0);
    await expect(page.getByTestId("saved-anchor")).toContainText('"offset":10000');
    await page.locator(".transcript").evaluate((element) => { element.scrollTop = 0; element.dispatchEvent(new Event("scroll")); });
    await expect(page.getByTestId("saved-anchor")).toContainText('"offset":10000');
    await page.locator(".transcript__restore-status button").first().click();
    await expect(page.locator(".msg--assistant").first()).toBeVisible();
    await expect(page.locator(".transcript__restore-status")).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath(`readable-${viewport.width}.png`) });
  });
}

test("unsettled team metadata cannot hide content beyond the job budget or revive a finished job", async ({ page }) => {
  await page.goto("/e2e/fixtures/transcript-restore.html?pending=true");
  await expect(page.locator(".transcript__restore-status")).toBeVisible({ timeout: 6000 });
  await expect(page.locator(".msg--assistant").first()).toBeVisible();
  await page.getByRole("button", { name: "Resolve team", exact: true }).click();
  await expect(page.locator('[data-team-state="team_archived"]')).toBeVisible();
  await expect(page.locator('[data-testid="persisted-expert-presentation"][aria-hidden="true"]')).toHaveCount(0);
  await expect(page.getByTestId("saved-anchor")).toContainText('"offset":10000');
});

test("user input and session switching cancel pending restoration", async ({ page }) => {
  await page.goto("/e2e/fixtures/transcript-restore.html?pending=true");
  await page.locator(".transcript").dispatchEvent("wheel", { deltaY: -100 });
  await expect(page.locator(".msg--assistant").first()).toBeVisible();
  await page.getByRole("button", { name: "Switch session", exact: true }).click();
  await page.getByRole("button", { name: "Resolve team", exact: true }).click();
  await expect(page.locator(".transcript__restore-status")).toHaveCount(0);
  await expect(page.locator(".msg--assistant").first()).toBeVisible();
});

for (const turns of [2, 4, 5, 6]) {
  test(`sidecar preserves the actual parent DOM and geometry for ${turns} turns`, async ({ page }) => {
    await page.goto(`/e2e/fixtures/transcript-restore.html?pending=true&bottom=true&archived=true&turns=${turns}`);
    const answer = page.locator(".msg--assistant").last();
    await expect(answer).toBeVisible();
    await expect(answer.locator("p").last()).toContainText("A synthetic history paragraph.");
    await page.evaluate(() => document.fonts.ready);
    let previousY: number | undefined;
    let stableSince = Date.now();
    await expect.poll(async () => {
      const y = (await answer.boundingBox())!.y;
      if (previousY !== y) stableSince = Date.now();
      previousY = y;
      return Date.now() - stableSince;
    }).toBeGreaterThan(200);
    const node = await answer.elementHandle();
    const before = await answer.boundingBox();
    await page.getByRole("button", { name: "Resolve team", exact: true }).click();
    await expect(page.locator('[data-team-state="team_archived"]')).toBeAttached();
    await expect.poll(() => node!.evaluate((element) => element.isConnected)).toBe(true);
    await expect.poll(async () => Math.abs((await answer.boundingBox())!.y - before!.y)).toBeLessThan(2);
    await expect(page.locator('[aria-hidden="true"][data-testid$="presentation"]')).toHaveCount(0);
  });
}

test("interruption reaches current bottom before metadata settles", async ({ page }) => {
  await page.goto("/e2e/fixtures/transcript-restore.html?pending=true&turns=48");
  await page.getByRole("button", { name: "Interrupt", exact: true }).click();
  const bottomDistance = () => page.locator(".transcript").evaluate((element) => element.scrollHeight - element.clientHeight - element.scrollTop);
  await expect.poll(bottomDistance).toBeLessThan(2);
  await page.getByRole("button", { name: "Resolve team", exact: true }).click();
  await expect.poll(bottomDistance).toBeLessThan(2);
  await expect(page.locator(".transcript__restore-status")).toHaveCount(0);
});
