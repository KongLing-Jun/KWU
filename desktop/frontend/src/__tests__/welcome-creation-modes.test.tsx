// Run: tsx src/__tests__/welcome-creation-modes.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React, { useState } from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { Welcome, type CreationScenarioSkill } from "../components/Welcome";
import { LocaleProvider } from "../lib/i18n";
import type { Scene } from "../lib/types";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) ok(true, label);
  else ok(false, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  Object.defineProperty(dom.window.navigator, "language", { configurable: true, value: "zh-CN" });
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  globalThis.Node = dom.window.Node;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.Event = dom.window.Event;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  return dom;
}

async function renderWelcome(
  onPrompt: (text: string) => void = () => {},
  onSelectScenario: (text: string, skill?: CreationScenarioSkill) => void = () => {},
  onSelectScene: (scene: Scene) => Promise<void> = async () => {},
) {
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  await act(async () => {
    function Harness() {
      const [scene, setScene] = useState<Scene>("office");
      return <LocaleProvider>
        <Welcome
          variant="creation"
          onPrompt={onPrompt}
          onSelectScenario={onSelectScenario}
          scene={scene}
          sceneState="draft"
          onSelectScene={async (next) => {
            await onSelectScene(next);
            setScene(next);
          }}
        />
      </LocaleProvider>;
    }
    root.render(<Harness />);
    await flush();
  });
  return root;
}

function modeButton(label: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__mode"))
    .find((button) => button.textContent?.includes(label));
}

function headlineText(): string {
  return document.querySelector<HTMLElement>(".welcome-creation__headline")?.textContent?.trim() ?? "";
}

function scenarioButton(label: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__scenario"))
    .find((button) => button.textContent?.includes(label));
}

console.log("\ncreation welcome modes");

const styles = ["../styles.css", "../styles/session.css", "../styles/theme-overrides.css"]
  .map((path) => readFileSync(new URL(path, import.meta.url), "utf8"))
  .join("\n");
const hiddenWelcomeBrandSelectors = Array.from(styles.matchAll(/([^{}]+)\{[^{}]*\bdisplay:\s*none;[^{}]*\}/g))
  .map((match) => match[1].trim())
  .filter((selector) => selector.split(",").some((part) => part.trim().endsWith(".welcome__brand")));
const compactCreationRule = styles.match(
  /@media\s*\(max-height:\s*430px\)[\s\S]*?\.app--creation\s+\.welcome--creation\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationBrandRule = styles.match(
  /\.app--creation\s+\.welcome--creation\s+\.welcome__brand\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationModesRule = styles.match(
  /\.app--creation\s+\.welcome-creation__modes\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationModeRule = styles.match(
  /\.app--creation\s+\.welcome-creation__mode\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationScenarioRule = styles.match(
  /\.app--creation\s+\.welcome-creation__scenario\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationWelcomeRule = styles.match(
  /\.app--creation\s+\.welcome\.welcome--creation,[\s\S]*?\{([^{}]*)\}/,
)?.[1] ?? "";
const creationScenariosRule = styles.match(
  /\.app--creation\s+\.welcome-creation__scenarios\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationScenarioListRule = styles.match(
  /\.app--creation\s+\.welcome-creation__scenario-list\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationScenarioStripRule = styles.match(
  /\.app--creation\s+\.welcome-creation__scenario-strip\s*\{([^{}]*)\}/,
)?.[1] ?? "";
const creationScenarioArrowRule = styles.match(
  /\.app--creation\s+\.welcome-creation__scenario-arrow\s*\{([^{}]*)\}/,
)?.[1] ?? "";

eq(hiddenWelcomeBrandSelectors.join(", "), "", "short layouts keep the welcome brand visible");
ok(compactCreationRule.includes("--welcome-brand-icon-size: 48px"), "short creation welcome keeps a compact brand icon");
ok(compactCreationRule.includes("--welcome-brand-font-size: 22px"), "short creation welcome keeps a readable brand name");
ok(creationBrandRule.includes("align-items: center"), "creation brand aligns its icon and name vertically");
ok(creationBrandRule.includes("gap: 12px"), "creation brand keeps stable horizontal spacing");
ok(!styles.includes("welcome-creation__mascot"), "creation welcome removes the absolute mascot styles");
ok(creationModesRule.includes("height: 36px"), "creation mode group matches the reference capsule height");
ok(creationModesRule.includes("padding: 2px"), "creation mode group matches the reference capsule inset");
ok(creationModesRule.includes("border-radius: 100px"), "creation mode group uses a full pill radius");
ok(creationModeRule.includes("height: 32px"), "creation mode buttons match the reference pill height");
ok(creationModeRule.includes("padding: 0 12px"), "creation mode buttons match the reference horizontal inset");
ok(creationModeRule.includes("border-radius: 100px"), "creation mode buttons use a full pill radius");
ok(creationScenarioRule.includes("height: 32px"), "creation scenario buttons match the reference pill height");
ok(creationScenarioRule.includes("padding: 0 12px"), "creation scenario buttons match the reference horizontal inset");
ok(creationScenarioRule.includes("border-radius: 100px"), "creation scenario buttons use a full pill radius");
ok(creationWelcomeRule.includes("justify-items: center"), "creation welcome centers its first three rows");
ok(creationScenariosRule.includes("width: min(calc(100% - 32px), 656px)"), "scenario strip matches the WorkBuddy visible-width proportion and remains responsive");
ok(creationScenarioStripRule.includes("grid-template-columns: 20px minmax(0, 1fr) 20px"), "scenario strip reserves independent arrow slots around the scrolling titles");
ok(creationScenarioStripRule.includes("gap: 8px"), "scenario arrows remain separated from the title viewport");
ok(creationScenarioArrowRule.includes("position: relative"), "scenario arrows no longer overlay the scrolling titles");
ok(styles.includes("welcome-creation__scenario-viewport--can-scroll-left::before"), "scenario fades are scoped to the title viewport");
ok(creationScenarioListRule.includes("flex-wrap: nowrap"), "scenario strip remains on one row");
ok(creationScenarioListRule.includes("padding-block: 2px"), "scenario strip preserves the full pill outline and shadow vertically");
ok(creationScenarioListRule.includes("overflow-x: auto"), "scenario strip scrolls horizontally when needed");
ok(styles.includes("width: 72px"), "scenario strip uses the reference fade width");
ok(styles.includes(".welcome-creation__scenario-arrow::before"), "scenario arrows keep the reference enlarged hit target");
ok(
  /@media\s*\(max-height:\s*380px\)[\s\S]*?\.app--creation\s+\.welcome-creation__modes\s*\{[^{}]*display:\s*none;/.test(styles),
  "very short creation welcome hides the mode switcher before it can overlap the composer",
);
ok(
  /@media\s*\(min-height:\s*634px\)[\s\S]*?--creation-empty-center-shift:\s*clamp\(-37px, calc\(7vh - 81px\), -23px\)/.test(styles),
  "creation layout uses a continuous height-based centering shift",
);
ok(
  creationBrandRule.includes("margin-bottom: clamp(8px, calc(1.05vh + 1.35px), 10px)"),
  "creation brand spacing scales continuously with window height",
);
ok(
  creationModesRule.includes("margin-top: clamp(16px, calc(1.05vh + 9.35px), 18px)"),
  "creation mode spacing scales continuously with window height",
);
ok(
  creationScenariosRule.includes("margin-top: clamp(38px, calc(5.21vh + 4.97px), 48px)"),
  "creation scenario spacing scales continuously with window height",
);
ok(
  /@media\s*\(min-height:\s*634px\)[\s\S]*?\.chat-pane--creation-empty\s+\.main[\s\S]*?margin-top:\s*auto[\s\S]*?translateY\(var\(--creation-empty-center-shift\)\)/.test(styles),
  "creation main content participates in continuous vertical centering",
);
ok(
  /@media\s*\(min-height:\s*634px\)[\s\S]*?\.chat-pane--creation-empty\s+\.footer[\s\S]*?margin-bottom:\s*auto[\s\S]*?translateY\(var\(--creation-empty-center-shift\)\)/.test(styles),
  "creation composer participates in the same vertical centering shift",
);

const dom = installDom();
let submittedPrompt = "";
let selectedScenarioPrompt = "";
let selectedScenarioSkill: CreationScenarioSkill | undefined;
const selectedScenes: Scene[] = [];
const root = await renderWelcome((prompt) => {
  submittedPrompt = prompt;
}, (prompt, skill) => {
  selectedScenarioPrompt = prompt;
  selectedScenarioSkill = skill;
}, async (scene) => {
  selectedScenes.push(scene);
});

eq(document.querySelector<HTMLElement>(".welcome__brand-name")?.textContent?.trim(), "云智企办", "creation welcome shows brand name");
const brand = document.querySelector<HTMLElement>(".welcome__brand");
const brandLogo = document.querySelector<HTMLImageElement>(".welcome__brand-logo");
ok(Boolean(brandLogo), "creation welcome shows brand icon");
ok(brandLogo?.src.includes("brand-symbol-hollow.png") ?? false, "creation welcome uses the hollow brand icon");
eq(document.querySelectorAll(".welcome__brand-logo").length, 1, "creation welcome renders one brand icon");
eq(brand?.firstElementChild, brandLogo, "creation brand places its icon first in the brand row");
ok(brandLogo?.nextElementSibling?.classList.contains("welcome__brand-name") ?? false, "creation brand places its name after the icon");
eq(document.querySelector(".welcome-creation__mascot"), null, "creation welcome removes the separate mascot node");
eq(headlineText(), "你的办公超能力", "creation welcome defaults to office title");
const creationRows = Array.from(document.querySelector<HTMLElement>(".welcome--creation")?.children ?? []);
ok(creationRows[0]?.classList.contains("welcome__brand") ?? false, "brand lockup remains the first creation row");
ok(creationRows[1]?.classList.contains("welcome-creation__headline") ?? false, "dynamic slogan remains the second creation row");
ok(creationRows[2]?.classList.contains("welcome-creation__modes") ?? false, "primary modes remain the third creation row");
ok(creationRows[3]?.classList.contains("welcome-creation__scenarios") ?? false, "secondary scenarios remain the fourth creation row");
ok(Boolean(modeButton("日常办公")), "creation welcome shows daily office mode");
ok(Boolean(modeButton("代码开发")), "creation welcome shows code development mode");
ok(Boolean(modeButton("设计创意")), "creation welcome shows design creative mode");
eq(modeButton("日常办公")?.getAttribute("aria-pressed"), "true", "office mode starts selected");
ok(Boolean(scenarioButton("文档处理")), "creation welcome exposes document processing");
ok(Boolean(scenarioButton("金融服务")), "creation welcome exposes financial services");
ok(Boolean(scenarioButton("数据分析及可视化")), "creation welcome exposes data analysis and visualization");
ok(Boolean(scenarioButton("会议纪要")), "office mode directly exposes meeting notes");
ok(Boolean(scenarioButton("项目规划")), "office mode directly exposes project planning");
ok(Boolean(scenarioButton("调研报告")), "office mode directly exposes research reports");
ok(Boolean(scenarioButton("流程自动化")), "office mode directly exposes workflow automation");
eq(scenarioButton("更多"), undefined, "creation welcome removes the more control");
eq(document.querySelectorAll(".welcome-creation__scenario-list .welcome-creation__scenario").length, 7, "office mode renders its complete secondary scenario set");
const scenarioList = document.querySelector<HTMLDivElement>(".welcome-creation__scenario-list");
let horizontalScroll: ScrollToOptions | undefined;
if (scenarioList) {
  Object.defineProperties(scenarioList, {
    clientWidth: { configurable: true, value: 320 },
    scrollWidth: { configurable: true, value: 760 },
    scrollLeft: { configurable: true, value: 0, writable: true },
    scrollBy: { configurable: true, value: (options: ScrollToOptions) => { horizontalScroll = options; } },
  });
  await act(async () => {
    scenarioList.dispatchEvent(new Event("scroll"));
    await flush();
  });
}
eq(document.querySelector(".welcome-creation__scenario-arrow--left"), null, "scenario strip hides the left arrow at its starting edge");
ok(Boolean(document.querySelector(".welcome-creation__scenario-arrow--right")), "scenario strip shows the right arrow while more content is available");
ok(document.querySelector(".welcome-creation__scenario-arrow--right")?.parentElement?.classList.contains("welcome-creation__scenario-arrow-slot--right") ?? false, "right scenario arrow renders in its reserved slot");
eq(document.querySelectorAll(".welcome-creation__scenario-arrow-slot").length, 2, "scenario strip preserves both arrow slots while one direction is hidden");
await act(async () => {
  document.querySelector<HTMLButtonElement>(".welcome-creation__scenario-arrow--right")?.click();
  await flush();
});
eq(horizontalScroll?.left, 200, "right scenario arrow uses the reference 200px scroll step");
eq(horizontalScroll?.behavior, "smooth", "scenario arrow scrolls smoothly");
if (scenarioList) {
  scenarioList.scrollLeft = 440;
  await act(async () => {
    scenarioList.dispatchEvent(new Event("scroll"));
    await flush();
  });
}
ok(Boolean(document.querySelector(".welcome-creation__scenario-arrow--left")), "scenario strip shows the left arrow after scrolling");
ok(document.querySelector(".welcome-creation__scenario-arrow--left")?.parentElement?.classList.contains("welcome-creation__scenario-arrow-slot--left") ?? false, "left scenario arrow renders in its reserved slot");
eq(document.querySelector(".welcome-creation__scenario-arrow--right"), null, "scenario strip hides the right arrow at its ending edge");

eq(document.querySelectorAll(".welcome-creation__scenario-list").length, 1, "office scenarios share one horizontal list");

await act(async () => {
  scenarioButton("会议纪要")?.click();
  await flush();
});
ok(selectedScenarioPrompt.length > 0, "a formerly additional scenario injects its dedicated prompt");

await act(async () => {
  scenarioButton("文档处理")?.click();
  await flush();
});
eq(submittedPrompt, "", "business scenario does not submit a chat turn immediately");
eq(selectedScenarioPrompt, "帮我处理这份文档：梳理结构、提炼重点，并给出可执行的修改建议", "business scenario fills the composer draft");
eq(selectedScenarioSkill?.name, "document", "business scenario provides a stable skill identifier");
eq(selectedScenarioSkill?.label, "文档处理", "business scenario provides its localized skill label");
ok(selectedScenarioSkill?.candidates.includes("documents") === true, "business scenario provides real skill lookup candidates");
eq(scenarioButton("文档处理")?.getAttribute("aria-pressed"), "true", "selected business scenario exposes its state");

await act(async () => {
  modeButton("代码开发")?.click();
  await flush();
});
eq(headlineText(), "你的代码超能力", "code mode updates title");
eq(selectedScenes.at(-1), "coding", "UI code mode maps to the persisted coding value");
eq(modeButton("代码开发")?.getAttribute("aria-pressed"), "true", "code mode becomes selected");
eq(scenarioList?.scrollLeft, 0, "mode changes reset the scenario strip to its starting edge");
eq(scenarioButton("文档处理"), undefined, "code mode hides office scenarios");
ok(Boolean(scenarioButton("日常开发")), "code mode exposes daily development");
ok(Boolean(scenarioButton("网站开发")), "code mode exposes website development");
ok(Boolean(scenarioButton("智能体应用")), "code mode exposes localized agent app development");
ok(Boolean(scenarioButton("Skill 开发")), "code mode directly exposes Skill development");
ok(Boolean(scenarioButton("CI/CD")), "code mode directly exposes CI/CD");
ok(Boolean(scenarioButton("文档")), "code mode directly exposes documentation");
eq(document.querySelectorAll(".welcome-creation__scenario-list .welcome-creation__scenario").length, 6, "code mode renders its complete secondary scenario set");

await act(async () => {
  scenarioButton("网站开发")?.click();
  await flush();
});
eq(selectedScenarioPrompt, "帮我开发这个网站：先明确页面目标与交互，再完成实现、响应式适配和验证。", "code scenario fills its dedicated prompt");

await act(async () => {
  modeButton("设计创意")?.click();
  await flush();
});
eq(headlineText(), "你的设计超能力", "design mode updates title");
eq(modeButton("设计创意")?.getAttribute("aria-pressed"), "true", "design mode becomes selected");
eq(selectedScenes.at(-1), "design", "design mode uses the strict backend value");
eq(scenarioButton("日常开发"), undefined, "design mode hides code scenarios");
ok(Boolean(scenarioButton("网站设计")), "design mode exposes website design");
ok(Boolean(scenarioButton("PPT设计")), "design mode exposes presentation design");
ok(Boolean(scenarioButton("视觉海报")), "design mode exposes poster design");
ok(Boolean(scenarioButton("移动端App")), "design mode directly exposes mobile app design");
ok(Boolean(scenarioButton("设计系统")), "design mode directly exposes design systems");
ok(Boolean(scenarioButton("Web App")), "design mode directly exposes Web App design");
eq(document.querySelectorAll(".welcome-creation__scenario-list .welcome-creation__scenario").length, 6, "design mode renders its complete secondary scenario set");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Welcome variant="creation" onPrompt={() => {}} scene="office" sceneState="draft" onSelectScene={async () => {}} />
    </LocaleProvider>,
  );
  await flush();
  modeButton("代码开发")?.click();
  await flush();
});
eq(modeButton("日常办公")?.getAttribute("aria-pressed"), "true", "failed scene persistence keeps the previous Draft selection");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Welcome variant="creation" onPrompt={() => {}} scene="design" sceneState="locked" onSelectScene={async () => {}} />
    </LocaleProvider>,
  );
  await flush();
});
eq(document.querySelectorAll(".welcome-creation__mode").length, 3, "locked zero-turn sessions preserve the scene selector nodes");
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__mode, .welcome-creation__scenario")).every((button) => button.disabled),
  "locked zero-turn sessions keep scene and secondary controls non-interactive",
);
eq(modeButton("设计创意")?.getAttribute("aria-pressed"), "true", "locked zero-turn sessions preserve the saved Design selection");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Welcome variant="creation" onPrompt={() => {}} sceneState="legacy" onSelectScene={async () => {}} />
    </LocaleProvider>,
  );
  await flush();
});
eq(document.querySelectorAll(".welcome-creation__mode").length, 3, "legacy zero-turn sessions retain the complete mode row");
eq(modeButton("日常办公")?.getAttribute("aria-pressed"), "true", "legacy zero-turn sessions fall back to Office");
ok(Boolean(scenarioButton("文档处理")), "legacy zero-turn sessions show Office secondary scenarios");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Welcome variant="creation" onPrompt={() => {}} scene="office" onSelectScene={async () => {}} />
    </LocaleProvider>,
  );
  await flush();
});
eq(document.querySelectorAll(".welcome-creation__mode").length, 3, "missing zero-turn scene metadata retains the complete mode row");
eq(modeButton("日常办公")?.getAttribute("aria-pressed"), "true", "missing zero-turn scene metadata falls back to Office");

await act(async () => {
  root.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
