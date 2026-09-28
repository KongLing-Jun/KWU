import { test, expect } from "@playwright/test";
import { attachProjectBrowserEvidence, collectProjectBrowserEvidence, openProjectScenario } from "./workbuddy-project-evidence";

test.describe("WorkBuddy Project browser contract", () => {
  test.afterEach(async ({ page }, testInfo) => {
    const evidence = (page as typeof page & { __projectEvidence?: ReturnType<typeof collectProjectBrowserEvidence> }).__projectEvidence;
    if (evidence) await attachProjectBrowserEvidence(testInfo, evidence);
  });

  test.beforeEach(async ({ page }) => {
    (page as typeof page & { __projectEvidence?: ReturnType<typeof collectProjectBrowserEvidence> }).__projectEvidence = collectProjectBrowserEvidence(page);
  });

  for (const [scenario, availability] of [
    ["anonymous", "anonymous"],
    ["unsupported", "unavailable"],
    ["incompatible", "error"],
    ["loading", "loading"],
    ["empty", "available"],
    ["stale", "available"],
  ] as const) {
    test(`${scenario} exposes the expected availability state`, async ({ page }) => {
      await openProjectScenario(page, scenario);
      await expect(page.locator(".cloud-projects")).toHaveAttribute("data-cloud-availability", availability);
      if (scenario === "stale") {
        await expect(page.locator(".cloud-projects__stale")).toBeVisible();
        await expect(page.locator(".cloud-projects__primary-action")).toBeDisabled();
      }
    });
  }

  test("account switch drops the previous scoped result", async ({ page }) => {
    await openProjectScenario(page, "account-switch");
    await expect(page.getByText("Alpha Fixture", { exact: true })).toBeVisible();
    await page.getByTestId("account-switch").click();
    await expect(page.getByText("Beta Fixture", { exact: true })).toBeVisible();
    await expect(page.getByText("Alpha Fixture", { exact: true })).toHaveCount(0);
  });

  test("project conflict preserves the draft and refreshes authoritative data", async ({ page }) => {
    await openProjectScenario(page, "conflict");
    await page.locator(".cloud-projects__card").click();
    await page.locator(".cloud-projects__lifecycle-menu summary").click();
    await page.locator(".cloud-projects__lifecycle-menu button").first().click();
    const dialog = page.getByRole("dialog", { name: "编辑云项目" });
    const name = dialog.getByRole("textbox", { name: "项目名称" });
    await expect(name).toBeFocused();
    await name.fill("Saved draft");
    await dialog.getByRole("button", { name: "保存" }).click();
    await expect(dialog.getByRole("alert")).toBeVisible();
    await expect(name).toHaveValue("Saved draft");
    await expect(page.locator("#cloud-project-workspace-title")).toHaveText("Authoritative Fixture");
  });

  test("project configuration no longer exposes obsolete workspace binding controls", async ({ page }) => {
    await openProjectScenario(page, "detail");
    await page.locator(".cloud-projects__card").click();
    const configurationToggle = page.getByRole("button", { name: "展开项目配置" });
    if (await configurationToggle.isVisible()) await configurationToggle.click();
    const configuration = page.getByRole("complementary", { name: "项目配置" });
    await expect(configuration).toBeVisible();
    await expect(configuration.getByText("工作空间绑定", { exact: true })).toHaveCount(0);
    await expect(configuration.getByText(/本地路径|选择工作空间/)).toHaveCount(0);
  });

  test("keyboard focus is trapped and narrow layout does not overflow the document", async ({ page }) => {
    await page.setViewportSize({ width: 720, height: 900 });
    await openProjectScenario(page, "detail", { zoom: 125 });
    await page.locator(".cloud-projects__primary-action").click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.locator("input")).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(dialog.locator(".project-modal-surface__close")).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(dialog.getByRole("button", { name: "放大弹窗" })).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(dialog.locator("button[type=submit]")).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await expect(page.locator(".cloud-projects__hero img")).toHaveCSS("display", "none");
  });

  test("new project capability picker receives real pointer input above its parent modal", async ({ page }) => {
    await openProjectScenario(page, "empty");
    await page.locator(".cloud-projects__primary-action").click();
    const projectDialog = page.locator(".cloud-project-dialog");
    const name = projectDialog.getByRole("textbox", { name: "项目名称" });
    await name.fill("Nested draft");

    await projectDialog.getByText("项目能力（可选）").click();
    const addConnector = projectDialog.getByRole("button", { name: "添加连接器" });
    await addConnector.locator("> span").click();
    const capabilityDialog = page.getByRole("dialog", { name: "选择连接器" });
    await expect(capabilityDialog).toBeVisible();
    await expect(projectDialog).toHaveAttribute("inert", "");
    await expect(projectDialog).toHaveAttribute("aria-hidden", "true");

    await capabilityDialog.getByRole("tab", { name: "全部连接器" }).click();
    await capabilityDialog.getByRole("button", { name: /腾讯文档/ }).click();
    await capabilityDialog.getByRole("button", { name: "确定" }).click();
    await expect(capabilityDialog).toHaveCount(0);
    await expect(projectDialog).toBeVisible();
    await expect(projectDialog).not.toHaveAttribute("inert", "");
    await expect(name).toHaveValue("Nested draft");
    await expect(projectDialog.getByText("腾讯文档", { exact: true })).toBeVisible();
    await expect(addConnector).toBeFocused();
  });

  test("selected-project geometry contains board overflow and keeps every column keyboard reachable", async ({ page }) => {
    await page.setViewportSize({ width: 820, height: 1000 });
    await openProjectScenario(page, "board-move");
    await page.locator(".cloud-projects__card").click();
    await page.getByRole("button", { name: "事项", exact: true }).click();
    await expect(page.locator(".work-item-board__columns")).toBeVisible();

    const geometry = await page.evaluate(() => {
      const root = document.querySelector<HTMLElement>(".cloud-projects")!;
      const center = document.querySelector<HTMLElement>(".cloud-projects__workspace-center")!;
      const view = document.querySelector<HTMLElement>(".cloud-projects__workspace-view")!;
      const board = document.querySelector<HTMLElement>(".work-item-board")!;
      const scroller = document.querySelector<HTMLElement>(".work-item-board__board-scroller")!;
      const rootRect = root.getBoundingClientRect();
      const centerRect = center.getBoundingClientRect();
      const viewRect = view.getBoundingClientRect();
      const boardRect = board.getBoundingClientRect();
      const scrollerRect = scroller.getBoundingClientRect();
      return {
        pageContained: document.documentElement.scrollWidth <= window.innerWidth,
        rootContained: rootRect.left >= 0 && rootRect.right <= window.innerWidth,
        centerWidth: centerRect.width,
        configurationCollapsed: !document.querySelector(".cloud-projects__rail") && Boolean(document.querySelector(".cloud-projects__configuration-edge-toggle")),
        boardContained: boardRect.left >= viewRect.left && boardRect.right <= viewRect.right,
        scrollerContained: scrollerRect.left >= boardRect.left && scrollerRect.right <= boardRect.right,
        boardOverflow: scroller.scrollWidth > scroller.clientWidth,
      };
    });
    expect(geometry.pageContained).toBe(true);
    expect(geometry.rootContained).toBe(true);
    expect(geometry.centerWidth).toBeGreaterThan(600);
    expect(geometry.configurationCollapsed).toBe(true);
    expect(geometry.boardContained).toBe(true);
    expect(geometry.scrollerContained).toBe(true);
    expect(geometry.boardOverflow).toBe(true);

	await page.locator(".cloud-projects__tabs button").first().focus();
	const actionCount = await page.locator(".work-item-board__card button:enabled").evaluateAll((actions) => {
		actions.forEach((action, index) => ((action as HTMLElement).dataset.focusProof = String(index)));
		return actions.length;
	});
	const reachedActions = new Set<string>();
	const reachedColumns = new Set<string>();
	for (let tabIndex = 0; tabIndex < 160 && reachedActions.size < actionCount; tabIndex += 1) {
		await page.keyboard.press("Tab");
		const focused = await page.evaluate(() => {
			const active = document.activeElement as HTMLElement | null;
			if (!active?.matches(".work-item-board__card button:enabled")) return null;
			const column = active.closest<HTMLElement>(".work-item-board__column")!;
			const scroller = active.closest<HTMLElement>(".work-item-board__board-scroller")!;
			const activeRect = active.getBoundingClientRect();
			const scrollerRect = scroller.getBoundingClientRect();
			return {
				action: active.dataset.focusProof,
				column: column.getAttribute("aria-labelledby"),
				visible: activeRect.left >= scrollerRect.left - 1 && activeRect.right <= scrollerRect.right + 1,
			};
		});
		if (focused) {
			expect(focused.visible).toBe(true);
			reachedActions.add(focused.action || "");
			reachedColumns.add(focused.column || "");
		}
	}
	expect(reachedActions.size).toBe(actionCount);
	expect([...reachedColumns]).toEqual([
      "work-item-status-todo",
      "work-item-status-in_progress",
      "work-item-status-paused",
      "work-item-status-done",
    ]);
    expect(await page.locator(".work-item-board__board-scroller").evaluate((node) => node.scrollLeft)).toBeGreaterThan(0);

    const centerWidthBeforeOpen = await page.locator(".cloud-projects__workspace-center").evaluate((node) => node.getBoundingClientRect().width);
    const centerHeightBeforeOpen = await page.locator(".cloud-projects__workspace-center").evaluate((node) => node.getBoundingClientRect().height);
    await page.getByRole("button", { name: "展开项目配置" }).click();
    await expect(page.getByRole("complementary", { name: "项目配置" })).toBeVisible();
    const centerWidthAfterOpen = await page.locator(".cloud-projects__workspace-center").evaluate((node) => node.getBoundingClientRect().width);
    const centerHeightAfterOpen = await page.locator(".cloud-projects__workspace-center").evaluate((node) => node.getBoundingClientRect().height);
    expect(centerWidthAfterOpen).toBe(centerWidthBeforeOpen);
    expect(centerHeightAfterOpen).toBeLessThan(centerHeightBeforeOpen);
    expect(await page.getByRole("complementary", { name: "项目配置" }).evaluate((node) => getComputedStyle(node).position)).toBe("relative");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(page.getByRole("button", { name: "展开项目配置" })).toBeFocused();
  });

  test("work item moves through the production board contract", async ({ page }) => {
    await openProjectScenario(page, "board-move");
    await page.locator(".cloud-projects__card").click();
    await page.getByRole("button", { name: "事项", exact: true }).click();
    await page.getByRole("button", { name: "移动 Prepare brief" }).click();
    const dialog = page.getByRole("dialog", { name: "移动事项" });
    await dialog.getByRole("combobox", { name: "目标状态" }).selectOption("in_progress");
    await dialog.getByRole("button", { name: "确认移动" }).click();
    await expect(page.getByRole("region", { name: "进行中" }).getByText("Prepare brief", { exact: true })).toBeVisible();
    await expect(page.getByTestId("fixture-events")).toContainText("moveCloudProjectWorkItem");
  });
});
