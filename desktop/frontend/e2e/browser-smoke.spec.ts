import { test, expect } from "./browser-evidence";

test("starts with accessible navigation and composer landmarks", async ({ page }) => {
  await page.goto("?scenario=ready");

  await expect(page.getByRole("heading", { level: 1, name: "Reasonix browser smoke" })).toBeVisible();
  const navigation = page.getByRole("navigation", { name: "Primary" });
  await expect(navigation.getByRole("button")).toHaveCount(3);
  await expect(page.getByRole("textbox", { name: "Message" })).toBeEnabled();

  await navigation.getByRole("button", { name: "Projects" }).click();
  await expect(page.getByTestId("last-action")).toHaveText("projects");
});

test("exposes login recovery and model switch actions", async ({ page }) => {
  await page.goto("?scenario=auth");

  const status = page.getByRole("status").filter({ hasText: "登录后使用官方订阅" });
  await expect(status).toContainText("登录后使用官方订阅");
  await expect(status).toHaveAttribute("aria-live", "polite");

  await page.getByRole("button", { name: "登录" }).click();
  await expect(page.getByTestId("last-action")).toHaveText("login");
  await expect(page.getByRole("button", { name: "重试" })).toHaveCount(0);
  await page.getByRole("button", { name: "切换模型" }).click();
  await expect(page.getByTestId("last-action")).toHaveText("switch-model");
});

test("keeps sending disabled while the model service is offline", async ({ page }) => {
  await page.goto("?scenario=offline");

  await expect(page.getByRole("status").filter({ hasText: "模型服务暂时不可用" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Message" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Send" })).toBeDisabled();
  await page.getByRole("button", { name: "重试" }).click();
  await expect(page.getByTestId("last-action")).toHaveText("retry");
});

test("restores the last test workspace after reload", async ({ page }) => {
  await page.goto("?scenario=ready");

  await page.getByRole("textbox", { name: "Workspace name" }).fill("Synthetic workspace");
  await page.getByRole("button", { name: "Save workspace" }).click();
  await page.reload();

  await expect(page.getByTestId("restored-workspace")).toHaveText("Synthetic workspace");
});

test("prepares a controlled file selection through its label", async ({ page }) => {
  await page.goto("?scenario=ready");

  await page.getByLabel("Choose a local file").setInputFiles({
    name: "synthetic.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("synthetic fixture"),
  });

  await expect(page.getByRole("status").filter({ hasText: "synthetic.txt" })).toBeVisible();
});

test("keeps the connector composer editable after the first send", async ({ page }) => {
  await page.goto("?scenario=composer-followup");

  const input = page.locator("#composer-input");
  await input.fill("first message");
  await input.press("Enter");
  await expect(page.getByTestId("followup-state")).toHaveText("sent");
  await expect(input).toHaveText("");

  const inputBox = await input.boundingBox();
  const cardBox = await page.locator(".composer-card").boundingBox();
  const toolbarBox = await page.locator(".composer-meta").boundingBox();
  expect(inputBox?.height ?? 0).toBeGreaterThanOrEqual(32);
  expect((toolbarBox?.y ?? 0) + (toolbarBox?.height ?? 0)).toBeLessThanOrEqual(
    (cardBox?.y ?? 0) + (cardBox?.height ?? 0) + 1,
  );

  await input.fill("second message");
  await expect(input).toHaveText("second message");
});

test("keeps a cloud-project composer geometric resize contract with connectors", async ({ page }) => {
  await page.setViewportSize({ width: 760, height: 720 });
  await page.goto("?scenario=composer-project-resize");

  const card = page.locator(".composer-card");
  const handle = page.getByRole("separator", { name: "拖动调整输入区高度，双击重置" });
  const input = page.locator(".composer__input");
  const dragHandleBy = async (deltaY: number) => {
    const handleBox = await handle.boundingBox();
    if (!handleBox) throw new Error("missing composer resize handle box");
    const x = handleBox.x + handleBox.width / 2;
    const y = handleBox.y + handleBox.height / 2;
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x, y + deltaY, { steps: 4 });
    await page.mouse.up();
  };
  const geometry = async () => page.evaluate(() => {
    const box = (selector: string) => {
      const element = document.querySelector<HTMLElement>(selector);
      if (!element) throw new Error(`missing ${selector}`);
      const rect = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return {
        top: rect.top,
        bottom: rect.bottom,
        height: rect.height,
        minHeight: style.minHeight,
        overflow: style.overflow,
      };
    };
    const editor = document.querySelector<HTMLElement>(".composer__input");
    if (!editor) throw new Error("missing composer input");
    const placeholder = getComputedStyle(editor, "::before");
    return {
      card: box(".composer-card"),
      chips: box(".composer-capability-chips"),
      composer: box(".composer"),
      input: box(".composer__input"),
      meta: box(".composer-meta"),
      placeholderHeight: Number.parseFloat(placeholder.height),
      placeholderLineHeight: Number.parseFloat(placeholder.lineHeight),
      inputClientHeight: editor.clientHeight,
    };
  });

  const initialHeight = (await card.boundingBox())?.height ?? 0;
  await dragHandleBy(-24);
  await expect.poll(async () => (await card.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(initialHeight + 21);
  const expandedWithoutConnector = (await card.boundingBox())?.height ?? 0;
  await dragHandleBy(12);
  await expect.poll(async () => (await card.boundingBox())?.height ?? 0).toBeLessThan(expandedWithoutConnector);
  await handle.press("Home");

  await page.getByRole("button", { name: "Add one connector" }).click();
  await expect(page.getByTestId("connector-count")).toHaveText("1");
  await expect(handle).toHaveAttribute("aria-valuemin", "148");
  const minimum = await geometry();
  expect(minimum.inputClientHeight).toBeGreaterThanOrEqual(minimum.placeholderLineHeight);
  expect(minimum.placeholderHeight).toBeLessThanOrEqual(minimum.inputClientHeight + 1);
  expect(minimum.chips.bottom).toBeLessThanOrEqual(minimum.composer.top + 1);
  expect(minimum.composer.bottom).toBeLessThanOrEqual(minimum.meta.top + 1);
  expect(minimum.meta.bottom).toBeLessThanOrEqual(minimum.card.bottom + 1);

  await dragHandleBy(-24);
  await expect.poll(async () => (await card.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(minimum.card.height + 23);

  const pointerHeight = (await card.boundingBox())?.height ?? 0;
  await handle.press("ArrowUp");
  await expect.poll(async () => (await card.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(pointerHeight + 15);
  await handle.press("ArrowDown");
  await expect.poll(async () => (await card.boundingBox())?.height ?? 0).toBeLessThanOrEqual(pointerHeight + 1);
  await handle.press("PageUp");
  await handle.press("PageDown");
  await handle.press("End");
  await expect(handle).toHaveAttribute("aria-valuenow", "288");
  await handle.press("Home");
  await expect(handle).toHaveAttribute("aria-valuenow", "148");

  await page.setViewportSize({ width: 520, height: 720 });
  await page.getByRole("button", { name: "Add many connectors" }).click();
  await expect(page.getByTestId("connector-count")).toHaveText("18");
  await expect.poll(async () => Number(await handle.getAttribute("aria-valuemin"))).toBeGreaterThan(148);
  const wrapped = await geometry();
  expect(wrapped.chips.height).toBeGreaterThan(46);
  expect(wrapped.inputClientHeight).toBeGreaterThanOrEqual(wrapped.placeholderLineHeight);
  expect(wrapped.meta.bottom).toBeLessThanOrEqual(wrapped.card.bottom + 1);

  await page.getByRole("button", { name: "Remove connectors" }).click();
  await expect(page.getByTestId("connector-count")).toHaveText("0");
  await expect.poll(async () => (await card.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(147);
  await expect.poll(async () => (await card.boundingBox())?.height ?? 0).toBeLessThanOrEqual(149);
  const persistedBeforeReset = await page.evaluate(() => JSON.parse(localStorage.getItem("kwu.layoutPreferences.v1") || "{}").sizes?.composerHeight);
  expect(persistedBeforeReset).toBe(148);
  await handle.dblclick();
  await expect(card).not.toHaveClass(/composer-card--resized/);
  const persistedAfterReset = await page.evaluate(() => JSON.parse(localStorage.getItem("kwu.layoutPreferences.v1") || "{}").sizes?.composerHeight);
  expect(persistedAfterReset).toBeUndefined();

  await input.fill("first line\nsecond line\nthird line");
  await expect(input).toContainText("third line");
  const textGeometry = await page.evaluate(() => {
    const editor = document.querySelector<HTMLElement>(".composer__input");
    if (!editor) throw new Error("missing composer input");
    return { clientHeight: editor.clientHeight, scrollHeight: editor.scrollHeight };
  });
  expect(textGeometry.clientHeight).toBeGreaterThan(0);
  expect(textGeometry.scrollHeight).toBeGreaterThanOrEqual(textGeometry.clientHeight);
});

test("keeps the active expert menu open until its trigger toggles it closed", async ({ page }) => {
  await page.goto("?scenario=composer-expert-menu");

  const expertTrigger = page.locator(".composer-action-trigger--expert");
  await expertTrigger.click();

  await expect(expertTrigger).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator(".composer-add-menu[data-state=\"open\"]")).toBeVisible();
  await expect(page.locator(".composer-recent-expert-menu")).toBeVisible();
  await expect(page.locator(".composer-recent-expert-menu__item--active")).toContainText("Finance Team");

  await expertTrigger.click();
  await expect(expertTrigger).toHaveAttribute("aria-expanded", "false");
  await expect(page.locator(".composer-add-menu")).toBeHidden();

  await page.getByRole("button", { name: "清除当前专家" }).click();
  await expect(page.getByTestId("expert-state")).toHaveText("cleared");
  await expect(page.locator(".composer-add-menu")).toHaveCount(0);
});

test("collaboration mode chip cancellation never mounts the closed menu", async ({ page }) => {
  await page.goto("?scenario=composer-collaboration-chip");
  await page.evaluate(() => {
    const body = document.body;
    body.dataset.intentMenuMounts = "0";
    const observer = new MutationObserver((records) => {
      const mounted = records.some((record) =>
        Array.from(record.addedNodes).some((node) =>
          node instanceof Element && (node.matches(".composer-add-menu") || Boolean(node.querySelector(".composer-add-menu"))),
        ),
      );
      if (mounted) body.dataset.intentMenuMounts = String(Number(body.dataset.intentMenuMounts) + 1);
    });
    observer.observe(body, { childList: true, subtree: true });
  });

  await page.locator(".composer-mode-chip--plan").click();
  await expect(page.getByTestId("collaboration-mode")).toHaveText("normal");
  await page.waitForTimeout(200);

  await expect(page.locator(".composer-add-menu")).toHaveCount(0);
  await expect(page.locator("body")).toHaveAttribute("data-intent-menu-mounts", "0");
});

test("keeps the expert menu compact in a short window", async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 620 });
  await page.goto("?scenario=composer-expert-menu");

  await page.locator(".composer-action-trigger--expert").click();
  const menu = page.locator(".composer-add-menu[data-state=\"open\"]");
  await expect(menu).toBeVisible();

  const menuBox = await menu.boundingBox();
  expect(menuBox?.width ?? Number.POSITIVE_INFINITY).toBeLessThanOrEqual(900 * 0.7 + 1);
  expect(menuBox?.height ?? Number.POSITIVE_INFINITY).toBeLessThanOrEqual(620 * 0.65 + 1);
  await expect(page.locator(".composer-add-menu__nav")).toHaveCSS("overflow-y", "auto");
  await expect(page.locator(".composer-add-menu__panel")).toHaveCSS("overflow-y", "auto");
});
