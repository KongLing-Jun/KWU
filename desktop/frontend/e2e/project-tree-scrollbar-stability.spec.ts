import { test, expect } from "./browser-evidence";

type ProjectTreeGeometry = {
  clientWidth: number;
  clientHeight: number;
  scrollHeight: number;
  rowLeft: number;
  rowRight: number;
  rowWidth: number;
  scrollbarGutter: string;
};

test("keeps project-tree rows stable when a section crosses the vertical overflow threshold", async ({ page }) => {
  await page.goto("?scenario=ready");

  await page.evaluate(() => {
    document.body.innerHTML = `
      <div class="app app--creation">
        <aside style="width: 294px">
          <div class="project-tree">
            <div class="project-tree__list" style="flex: none; width: 294px; height: 240px">
              <div class="project-tree__task-section project-tree__task-section--tasks">
                <div class="project-tree__task-section-head">任务 (3)</div>
                <div class="project-tree__task-section-body">
                  <div class="project-tree__topic project-tree__topic--global">一条足够长且会触发省略号的任务标题</div>
                  <div class="project-tree__topic project-tree__topic--global">第二条足够长且会触发省略号的任务标题</div>
                  <div class="project-tree__topic project-tree__topic--global">第三条足够长且会触发省略号的任务标题</div>
                </div>
              </div>
              <div class="project-tree__task-section project-tree__task-section--workspaces">
                <div class="project-tree__task-section-head">工作空间 (8)</div>
              </div>
            </div>
          </div>
        </aside>
      </div>
    `;
  });

  const geometry = async (): Promise<ProjectTreeGeometry> => page.evaluate(() => {
    const list = document.querySelector<HTMLElement>(".project-tree__list");
    const row = document.querySelector<HTMLElement>(".project-tree__topic--global");
    if (!list || !row) throw new Error("missing synthetic project-tree elements");
    const rowRect = row.getBoundingClientRect();
    return {
      clientWidth: list.clientWidth,
      clientHeight: list.clientHeight,
      scrollHeight: list.scrollHeight,
      rowLeft: rowRect.left,
      rowRight: rowRect.right,
      rowWidth: rowRect.width,
      scrollbarGutter: getComputedStyle(list).scrollbarGutter,
    };
  });

  const collapsed = await geometry();
  expect(collapsed.scrollHeight).toBeLessThanOrEqual(collapsed.clientHeight);
  expect(collapsed.scrollbarGutter).toBe("stable");

  const expectStableWidth = (actual: ProjectTreeGeometry) => {
    expect(actual.clientWidth).toBe(collapsed.clientWidth);
    expect(actual.rowLeft).toBe(collapsed.rowLeft);
    expect(actual.rowRight).toBe(collapsed.rowRight);
    expect(actual.rowWidth).toBe(collapsed.rowWidth);
  };

  for (let cycle = 0; cycle < 10; cycle += 1) {
    await page.locator(".project-tree__task-section--workspaces").evaluate((section) => {
      const body = document.createElement("div");
      body.className = "project-tree__task-section-body project-tree__task-section-body--synthetic";
      for (let index = 0; index < 8; index += 1) {
        const row = document.createElement("div");
        row.className = "project-tree__topic";
        row.textContent = `工作空间 ${index + 1}`;
        body.appendChild(row);
      }
      section.appendChild(body);
    });

    const expanded = await geometry();
    expect(expanded.scrollHeight).toBeGreaterThan(expanded.clientHeight);
    expectStableWidth(expanded);

    await page.locator(".project-tree__task-section-body--synthetic").evaluate((body) => body.remove());
    const collapsedAgain = await geometry();
    expect(collapsedAgain.scrollHeight).toBeLessThanOrEqual(collapsedAgain.clientHeight);
    expectStableWidth(collapsedAgain);
  }
});
