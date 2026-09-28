import { expect, test } from "@playwright/test";
import { writeFile } from "node:fs/promises";

// Characterization of current behavior, not acceptance of the stale running state.
// This uses the real controller and Transcript, but not App/ProjectTree or native IPC.
for (const rate of [1, 6]) {
  for (const scenario of ["project", "project-expert", "global", "project-terminal", "project-terminal-prompt", "project-visible-control"]) {
    test(`hydration geometry ${scenario} CPU ${rate}x`, async ({ page }, testInfo) => {
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      const cdp = await page.context().newCDPSession(page);
      await cdp.send("Emulation.setCPUThrottlingRate", { rate });
      await page.setViewportSize({ width: 1280, height: 800 });
      const expert = scenario.includes("expert");
      const terminal = scenario.includes("terminal");
      const prompt = scenario.includes("prompt");
      const visibleControl = scenario.includes("visible-control");
      const replayRunning = !terminal || prompt;
      await page.goto(`/e2e/fixtures/session-hydration.html?scope=${scenario === "global" ? "global" : "project"}${expert ? "&expert" : ""}${terminal ? "&terminal" : ""}${prompt ? "&prompt" : ""}${visibleControl ? "&visible-control" : ""}`);
      await expect.poll(() => page.evaluate(() => window.__hydrationFixture.state.ready)).toBe(true);
      // Isolate intrinsic-size effects in a synthetic page; this is not a proposed production CSS fix.
      if (visibleControl) await page.addStyleTag({ content: ".msg { content-visibility: visible !important; }" });
      expect(await page.evaluate(() => window.__hydrationFixture.export().critical)).toEqual([]);
      await page.getByRole("button", { name: "Start capture", exact: true }).click();
      await page.getByRole("button", { name: "Open session", exact: true }).click();
      await expect.poll(() => page.evaluate(() => window.__hydrationFixture.state.target)).toBe(true);
      await page.getByRole("button", { name: "Release history", exact: true }).click();
      await expect.poll(() => page.evaluate(() => window.__hydrationFixture.state.items)).toBeGreaterThan(1);
      await expect.poll(() => page.evaluate(() => window.__hydrationFixture.state.running)).toBe(replayRunning);
      const answer = page.locator(".msg--assistant:not(.turn-collapse .msg--assistant)");
      await expect(answer).toHaveCount(replayRunning ? 0 : 1);
      await page.screenshot({ path: testInfo.outputPath("history.png") });
      const held = await page.evaluate(() => window.__hydrationFixture.export());
      const runtimeCalls = held.critical.filter((event) => event.event === "request.start" && event.data.operation === "runtime").length;
      if (expert) {
        await page.getByRole("button", { name: "Release team", exact: true }).click();
        await expect(page.getByTestId("persisted-expert-presentation")).toBeAttached();
        await expect(answer).toHaveCount(0);
        await page.screenshot({ path: testInfo.outputPath("team.png") });
      }
      // A held request, not a wall-clock timeout, controls the runtime refresh boundary.
      expect(await page.evaluate(() => window.__hydrationFixture.export().critical.filter((event) => event.event === "request.start" && event.data.operation === "runtime").length)).toBe(runtimeCalls);
      await page.getByRole("button", { name: "Release jobs", exact: true }).click();
      await expect.poll(() => page.evaluate(() => window.__hydrationFixture.state.running)).toBe(false);
      await expect.poll(() => page.evaluate(() => window.__hydrationFixture.state.approval)).toBe(false);
      await expect(answer).toHaveCount(1);
      await expect(answer).toContainText("Synthetic completed report");
      await page.screenshot({ path: testInfo.outputPath("idle.png") });
      await page.getByRole("button", { name: "Stop capture", exact: true }).click();
      const trace = await page.evaluate(() => window.__hydrationFixture.export());
      expect(trace.droppedCritical).toBe(0);
      expect(trace.critical.some((event) => event.event === "commit" && event.data.target && event.data.loaded && event.data.running === replayRunning)).toBe(true);
      expect(trace.geometry.some((event) => event.event === "answer" && event.data.present)).toBe(true);
      expect(trace.geometry.filter((event) => event.event === "answer" && event.data.present).every((event) => event.data.layoutAuto === !visibleControl)).toBe(true);
      expect(trace.critical.length).toBeLessThanOrEqual(trace.capacity);
      expect(trace.geometry.length).toBeLessThanOrEqual(trace.capacity);
      // Only fixed enum strings and numeric/boolean metrics reach the artifact.
      expect(JSON.stringify(trace)).not.toMatch(/Synthetic|fixture-target|fixture-turn|fixture-prompt|\/fixture|read_file/);
      await page.locator(".transcript").evaluate((node) => { node.scrollTop = 0; node.dispatchEvent(new Event("scroll")); });
      expect(await page.evaluate(() => window.__hydrationFixture.export())).toEqual(trace);
      const tracePath = testInfo.outputPath("hydration.json");
      await writeFile(tracePath, JSON.stringify(trace, null, 2));
      await testInfo.attach("hydration.json", { path: tracePath, contentType: "application/json" });
      expect(errors).toEqual([]);
    });
  }
}
