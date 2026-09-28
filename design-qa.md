# 自动化生效日期区间 UI QA（2026-08-15）

## 证据

- 参考图：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-0fdfa4ac-222a-4abe-9396-544515143e77.png`
- 浏览器实现截图（关闭态）：`D:/project/Reasonix/automation-date-range-after.png`
- 浏览器实现截图（展开态）：`D:/project/Reasonix/automation-date-range-open.png`
- 参考图像素：1552 × 105；实现图像素与 CSS 视口：1280 × 720，设备像素比 1
- 状态：浅色主题，非全屏添加自动化，生效日期为空

## 对照结果

- [P1 已修复] 原界面直接并排显示两个日期时间输入，与参考图的单行“选择生效日期”入口不一致，也增加了表单视觉密度。
- 关闭态现在为单个 44px 高、整行圆角选择框，标签、辅助说明、占位文案和橙色聚焦边框与参考一致。
- 由于上一轮明确要求新增页不全屏，实现宽度受 648px 居中表单约束；这是产品布局约束，不是日期控件比例漂移。
- 点击单行入口后，以浮层展示原有开始/结束时间输入，仍写入 `effectiveFrom` 与 `effectiveUntil`，没有降低日期区间能力。
- 两个空日期输入使用“开始日期和时间”“结束日期和时间”中文提示，避免浏览器默认的 `yyyy/mm/日 --:--` 混合语言占位。

## 必查视觉面

- 字体：标签、辅助说明与占位文案层级清晰，无截断。
- 间距：标签与控件间距、44px 控件高度、8px 圆角及浮层留白正常。
- 色彩：默认边框使用现有中性色，聚焦/展开态使用参考图的橙色提示。
- 图像：该区域没有栅格图片资产，也没有新增替代性图形。
- 文案：使用“生效日期区间（可选，留空表示始终生效。）”“选择生效日期”“开始日期和时间”“结束日期和时间”。

## 交互验证

- 单行日期入口可展开和收起。
- 展开后同时存在“生效开始时间”和“生效结束时间”两个原生日期时间输入。
- 空值关闭态恢复显示“选择生效日期”。
- 生产构建 `pnpm build` 通过。

## 比较历史

- 首轮发现 P1：两个日期输入持续外露，与参考单行入口不一致。
- 修复后关闭态匹配参考的单行结构；展开态保留原有区间编辑能力，无页面溢出或遮挡。

## 最终结果

passed

---

# 自动化权限菜单可读性 QA（2026-08-15）

## 证据

- 参考图：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-bbfcd05e-fd71-4c54-8dd5-b7f61a7db6db.png`
- 浏览器实现截图：`D:/project/Reasonix/automation-permission-menu-after.png`
- 参考图与实现图像素：743 × 157；浏览器 CSS 视口：743 × 157，设备像素比 1
- 状态：浅色主题，添加自动化任务，权限等级菜单展开，`Auto` 选中

## 对照结果

- [P1 已修复] 原权限等级菜单继承紧凑触发器宽度，`Auto` 和“完全访问权限”出现截断，选项含义无法完整识别。
- 权限菜单现在使用 180px 最小宽度，`Ask`、`Auto`、`完全访问权限` 均完整显示；工具栏触发器尺寸保持不变。
- 菜单圆角、阴影、选中背景、勾选图标和橙色权限提示继续复用现有设计系统。

## 必查视觉面

- 字体：三个权限名称不再截断，字号与相邻工具菜单一致。
- 间距：选项内边距和勾选留白正常，菜单未挤压提示词工具栏。
- 色彩：保持权限提示橙色以及既有选中、边框、背景变量。
- 图像：此区域没有栅格图片；警告与勾选使用现有图标库。
- 文案：权限名称和底层值均未更改。

## 交互验证

- 选择 `Ask` 后触发器正确更新。
- 再次展开并恢复 `Auto`，原有权限模式状态逻辑正常。
- 生产构建 `pnpm build` 通过。

## 比较历史

- 首轮参考图发现 P1 可读性问题：菜单宽度与触发器一致，后两个选项被截断。
- 修复后的同尺寸对照图中三个选项均完整显示，无文本溢出或水平裁切。

## 最终结果

passed

---

# 自动化模型菜单可读性 QA（2026-08-15）

## 证据

- 参考图：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-9a9aa24d-edb1-4787-b5a5-544628ca7836.png`
- 浏览器实现截图：`D:/project/Reasonix/automation-model-menu-after-focused.png`
- 参考图与实现图像素：944 × 359；浏览器 CSS 视口：944 × 359，设备像素比 1
- 状态：浅色主题，添加自动化任务，模型菜单展开，默认模型选中

## 对照结果

- [P1 已修复] 原模型菜单继承约 74px 的紧凑触发器宽度，模型名称全部被截断，无法区分不同模型和提供方。
- 模型触发器仍保持工具栏的紧凑尺寸；展开菜单独立设置 320px 最小宽度，并继续受 90vw / 420px 上限约束。
- 当前五个模型名称及提供方均完整可读，选中勾选、悬停样式、圆角和阴影沿用现有设计系统。

## 必查视觉面

- 字体：字号、字重与同组技能/专家菜单一致，模型名不再省略。
- 间距：菜单内边距、行高和选中标记留白正常，未挤压工具栏。
- 色彩：沿用既有背景、边框、悬停和强调色变量。
- 图像：此菜单没有栅格图像资产；现有 Activity 与 Check 图标保持原图标库实现。
- 文案：模型名和 provider 原样显示，没有改写或硬编码。

## 交互验证

- 展开模型菜单后选择 `deepseek-v4-flash · airclaude_subscription`，触发器正确更新。
- 再次展开并恢复 `Auto`，选择值和原有模型路由逻辑正常。
- 生产构建 `pnpm build` 通过。

## 比较历史

- 首轮参考图发现 P1 可读性问题：约 74px 菜单导致所有长模型名截断。
- 修复后同尺寸对照图显示菜单宽度充足，所有当前选项完整显示，无溢出或遮挡。

## 最终结果

passed

---

# 自动化下半段表单 UI QA（2026-08-15）

## 证据

- 参考图：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-60e9d557-d637-4042-9b16-fc173156cc56.png`
- 浏览器实现截图：`D:/project/Reasonix/automation-lower-form-after.png`
- 浏览器视口：1280 × 720，设备像素比 1
- 状态：浅色主题，添加自动化任务，执行频率为“周期”

## 对照结果

- 提示词工具栏下方改为参考图的单列信息流：连接器、执行频率、生效日期、结果通知、失败冷却。
- 连接器改为整行选择器外观；展开后仍使用原有连接器 ID 多选逻辑，没有改动授权快照或提交数据。
- 执行频率保留原有“周期 / 按间隔 / 单次”三种模式及其动态配置控件，仅调整层级、间距和控件尺寸。
- 结果通知改为右侧开关行；授权目标仍由后端动态提供，没有硬编码参考图中的 WorkBuddy 或企微目标。
- 生效日期保留产品原有的起止区间能力，因此实现中仍有两个日期输入，而不是退化为参考图的单日期字段。

## 交互验证

- 连接器选择器可正常展开、显示空状态并收起。
- 结果通知开关可打开、关闭，并已恢复到验证前状态。
- “按间隔”和“周期”可正常切换，最终恢复到截图所示“周期”状态。
- 两个生效日期输入、失败冷却及通知目标禁用态均正常渲染。
- 生产构建 `pnpm build` 通过。

## 发现

- P0：无。
- P1：已修复。原下半段字段层级分散，连接器呈静态标签区，通知使用普通复选框，与参考图的信息密度和扫读顺序不一致。
- P2：无。
- P3：开发预览中保留了一条既有 Vite 客户端错误日志；本轮交互没有触发新的页面崩溃或渲染异常。

## 最终结果

passed

---

# Skills Marketplace Design QA

## Evidence

- Source visual truth (marketplace card treatment): `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-63bcf4ba-20a3-47cc-a51b-8527b39d246a.png`
- Source visual truth (existing product navigation and skills layout): `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-237c7153-19a2-401d-8416-5e92dd984d33.png`
- Final browser-rendered catalog: `D:/project/Reasonix/artifacts/design-qa/skills-marketplace-final-1552x928.png`
- Browser-rendered detail dialog: `D:/project/Reasonix/artifacts/design-qa/skills-marketplace-detail-1552x928.png`
- Browser CSS viewport: 1552 x 928 at device pixel ratio 1.
- Source pixels: marketplace crop 1685 x 518; product layout 1552 x 928.
- State: light theme, public catalog selected, first page loaded with 20 live API records.

## Full-view comparison

The final browser capture was inspected together with the marketplace reference. It preserves the source's three-column, lightly bordered card grid, square skill artwork, compact metadata badges, two-line descriptions, and muted star/download/source footer. It also keeps Reasonix's existing left navigation and expert/skill/connector workspace header from the product-layout reference.

The implementation intentionally adds a catalog/installed segmented control, search, loaded-count feedback, explicit install buttons, and pagination because these are required for the live public API and local installation flow.

## Focused comparison

The supplied marketplace image is already a focused card-region reference. At 1552 x 928, the implementation's three cards measure approximately 379 px each and remain fully inside the content viewport (right edge about 1485 px). Card padding, icon scale, title hierarchy, badge radius, description density, footer alignment, and row/column gaps were checked against that reference.

## Interaction verification

- Opened the ability center and selected Skills.
- Confirmed the public catalog loads 20 records without login.
- Searched for `ima` and confirmed the filtered results.
- Loaded the next cursor page and confirmed the count grows from 20 to 24 without duplicates.
- Opened the `ima-skills` detail dialog and confirmed namespace, version, package size, SHA-256 preview, metrics, homepage, and install action.
- Confirmed the detail dialog closes with Escape.
- Confirmed the Installed tab and existing local skill management remain available.
- Did not trigger a real install during visual QA to avoid changing the user's active skill state; installer behavior is covered by Go tests.
- No new console errors appeared after the flexible API metadata decoder was applied.

## Comparison history

The first viewport-transition capture appeared to crop the third column. DOM measurements showed the grid was within bounds; recapturing after layout reflow produced the final evidence above. The final capture has no horizontal clipping, overlapping controls, or broken card bounds.

## Findings

- P0: none.
- P1: none.
- P2: none.
- P3: API-provided category and source strings remain in their original language, and unusually long skill titles are truncated to protect grid rhythm. Both are intentional.

## Final result

passed

---

# Skills Marketplace Category Filter Follow-up QA

## Evidence

- Source visual truth: `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-47647317-e1a5-434c-90b2-92aa55281e01.png`.
- Browser-rendered all-categories state: `D:/project/Reasonix/artifacts/design-qa/skills-categories-all-1555x800.png`.
- Browser-rendered filtered state: `D:/project/Reasonix/artifacts/design-qa/skills-categories-data-analysis-1555x800.png`.
- Combined comparison evidence: `D:/project/Reasonix/artifacts/design-qa/skills-categories-comparison.png`.
- Source pixels: 1555 x 352. Implementation pixels and CSS viewport: 1555 x 800 at device pixel ratio 1. The implementation was cropped to the category-and-card region for the combined focused comparison; no density conversion was required.
- State: light theme, public skill catalog loaded with 20 live API records, `全部` selected for the source comparison; `数据分析` and `表格处理` selected for interaction verification.

## Full-view comparison

The category navigation follows the reference's single horizontal row, compact text tabs, subdued unselected labels, rounded selected state, and immediate transition into the skill-card grid. The implementation keeps the existing Reasonix three-column content width and marketplace card treatment instead of replacing the product shell with the reference's four-column standalone crop.

## Focused comparison

The combined comparison image places the supplied reference and the browser-rendered category/card region in one artifact. Category spacing, one-line overflow behavior, active-state hierarchy, card alignment, typography, neutral colors, real API artwork, and visible copy were inspected together. The implementation uses existing Reasonix tokens and its established three-column grid; this is an intentional product constraint rather than design drift.

## Interaction verification

- Opened the ability center, selected Skills, and loaded 20 live public records.
- Confirmed `全部` plus unique `categories` values render in first-seen API order.
- Selected `数据分析`; only `Excel/WPS 表格自动化工具` remained visible and the tab exposed `aria-selected="true"`.
- Selected `表格处理`; the same skill remained visible, confirming that one skill can belong to multiple categories.
- Confirmed the horizontal category row scrolls the selected off-screen category into view.
- Checked browser console errors after both filters; none were present.

## Required fidelity surfaces

- Fonts and typography: existing Reasonix Chinese UI font stack, compact tab text, card-title weight, wrapping, and truncation remain consistent and readable.
- Spacing and layout rhythm: the tab row is single-line with 8 px gaps and compact selected pills; the existing three-column grid remains aligned beneath it without overflow.
- Colors and visual tokens: neutral tab labels and the existing surface/foreground tokens match the reference's restrained treatment while preserving the product theme.
- Image quality and asset fidelity: live API icon URLs render sharply; no placeholder, CSS-drawn, inline-SVG, or generated substitute assets were introduced.
- Copy and content: category labels come directly from each record's `categories` array, with `全部` as the only UI-added label.

## Findings and comparison history

- P0: none.
- P1: none.
- P2: none.
- P3: the reference shows four columns because it is a wider standalone crop; Reasonix intentionally retains its established three-column marketplace inside the application shell.
- First browser comparison passed for the requested category-navigation scope, so no P0/P1/P2 visual iteration was required.

final result: passed

---

# Project conversation context display QA (2026-08-31)

## Evidence and state

- Source visual truth: `C:/Users/admin/AppData/Local/Temp/codex-clipboard-10106852-890f-495f-a7d7-b75970204647.png` (346 x 60 px).
- Browser-rendered focused implementation: `D:/project/KWU/artifacts/project-contextbar-after-focused.png` (360 x 34 px).
- Combined comparison evidence: `D:/project/KWU/artifacts/project-contextbar-qa-comparison.png` (726 x 80 px).
- CSS viewport: 1280 x 800 at device pixel ratio 1; the focused implementation crop is pixel-identical to its CSS clip, so no density normalization was needed.
- State: light theme, cloud project `t3`, local execution context, project conversation composer visible.

## Full-view and focused comparison

The browser-rendered project view was opened to the same `本地任务 / t3` state shown in the reference. The focused crop is sufficient because the requested change is isolated to the 34 px project context row; the combined comparison places the supplied reference and final implementation in one artifact. The implementation retains the folder icon and text order while removing both dropdown chevrons and the permission-mode display.

## Required fidelity surfaces

- Fonts and typography: both labels use the existing product font stack and small-UI sizing; computed foreground color is `rgb(24, 24, 27)`, producing the requested black text without disabled-state opacity.
- Spacing and layout rhythm: the row remains 34 px high, vertically centered, with the established 18 px gap and ellipsis protection for long project names.
- Colors and visual tokens: both labels use `var(--fg)` on the existing white surface; no warning or disabled color is visible.
- Image quality and asset fidelity: the existing Lucide folder icon remains vector-sharp. No raster replacement, custom SVG, CSS drawing, or placeholder asset was introduced.
- Copy and content: `本地任务` and `t3` remain visible; the permission label such as `禁止` is absent.

## Interaction and console verification

- The project context row contains two `SPAN` elements, zero buttons, zero dropdown chevrons, and zero permission controls.
- Both labels report the default cursor and cannot open menus or change project state.
- The static display no longer requests project-tree data solely to build selector options.
- Browser console warnings/errors checked: none.
- Targeted responsive-toolbar test: 156 passed, 0 failed.
- Production frontend build: passed.

## Comparison history and findings

- Initial reference mismatch: task type and project name appeared as muted dropdown controls, and a permission state was visible at the right edge.
- Final implementation: both values are black display-only text, dropdown affordances are absent, and the permission state is removed from the project conversation row.
- P0: none.
- P1: none.
- P2: none.
- P3: none for the requested display-only scope.

final result: passed

---

# Featured Skill Grid Responsive Fill QA (2026-08-29)

## Evidence

- Source visual truth: `C:/Users/admin/AppData/Local/Temp/codex-clipboard-e800f91f-4700-447f-96f7-ad5f9fcb7fac.png`.
- Browser-rendered focused implementation: `D:/project/KWU/artifacts/design-qa/skill-featured-responsive-wide.png`.
- Source and implementation pixels: 2227 x 267. CSS viewport: 2227 x 900 at device pixel ratio 1; the implementation was cropped to the same 2227 x 267 region, so no density normalization was required.
- State: light theme, four featured skill cards rendered with the production skill-library selectors and stylesheet.

## Comparison

The source and implementation were opened together in the same visual comparison input. The source exposes a large unused region after the fourth card. In the final implementation, four equal columns span the full 2168.8 px content grid: the first card begins at x=24 and the last card ends at the grid's x=2192.8 right edge. No document-level horizontal overflow is present.

A separate focused crop was not needed because the requested change is the full-row track distribution; the equal card widths, row edge, and responsive wrapping are legible in the full-width evidence. Existing typography, spacing inside cards, colors, assets, and copy were not changed.

## Responsive verification

- At 2227 px, the featured grid renders four equal 536.2 px columns in one complete row.
- At 900 px, it renders two equal 416.8 px columns and two complete rows.
- At 500 px, it renders one 457.6 px column and four complete rows.
- At all three widths, the last card in every row reaches the row's right edge and `scrollWidth` equals the viewport width.
- The isolated browser-rendered layout produced no console errors or warnings.

## Required fidelity surfaces

- Fonts and typography: unchanged; the existing production font stack, title weight, line height, truncation, and description treatment remain in use.
- Spacing and layout rhythm: card padding, 8 px gaps, radii, shadows, and section spacing remain unchanged; only featured-grid track counts now follow 4/2/1 responsive tiers.
- Colors and visual tokens: unchanged; production light-theme tokens remain active.
- Image quality and asset fidelity: production catalog image loading and fallbacks were not modified; the focused layout harness used the component's existing text fallback so no replacement asset was added to application code.
- Copy and content: unchanged; `精选技能`, `换一换`, titles, and descriptions remain sourced through the existing component.

## Comparison history and findings

- Initial P2: the generic `auto-fill` grid reserved empty tracks at wide widths, leaving a large unused region after four featured cards.
- First fix: collapsing empty tracks filled the wide row, but a 900 px check revealed a 3+1 arrangement with two empty positions on the second row.
- Final fix: scoped the featured grid to four columns on wide containers, two columns at 1080 px and below, and one column at 560 px and below.
- Post-fix evidence: 4/2/1 layouts fill every row at 2227, 900, and 500 px with no horizontal overflow.
- P0: none.
- P1: none.
- P2: none.
- P3: none for the requested responsive-fill scope.

final result: passed
---

# Creation Skill Success Toast Removal QA (2026-08-17)

## Evidence and state

- Source visual truth: `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-c91cc22f-c2dc-4372-90c2-309a9a69b1c8.png` (560 x 164 px), identifying the success toast that must be absent.
- Implementation screenshot: `artifacts/design-qa/creation-skill-no-success-toast-1280x720.png` (1280 x 720 px).
- Combined comparison: `artifacts/design-qa/creation-skill-success-toast-removal-comparison.png` (1280 x 500 px).
- Browser viewport: 1280 x 720 CSS px at DPR 1; no density normalization was required.
- State: light theme, Daily Office mode, `金融服务` scenario selected, matching Skill token embedded in the composer, success toast absent.

## Comparison and findings

- Full-view evidence confirms the selected scenario and embedded Skill token remain visible while no floating success message obscures the canvas.
- The combined focused comparison places the unwanted reference toast beside the rendered post-click state; the referenced copy and surface are absent in the implementation.
- Fonts and typography, spacing and layout rhythm, colors and tokens, icon/image quality, and all surrounding copy are unchanged because the fix removes only the redundant feedback surface.
- No actionable P0, P1, or P2 difference remains. No additional focused crop was needed because the full 1280 x 720 capture keeps both the composer token and the empty toast region readable.

## Interaction and validation

- Clicked `金融服务`; the real Skill token was inserted and the send action became available.
- DOM snapshot contained no `已嵌入` status/toast content after the click.
- Browser console contained 0 errors and 0 warnings in the verified state.
- Production frontend build passed; the targeted success-toast regression assertion passed.

## Comparison history

1. P2: successful Skill insertion showed a redundant floating confirmation in addition to the persistent composer token.
2. Removed the success-toast call and its unused locale entry while retaining missing-Skill and load-failure warnings.
3. Recaptured the same interaction state; the token remains embedded and no success toast is rendered.

final result: passed

---

# Creation Scenario Skill Embedding QA (2026-08-17)

## Evidence and state

- Source visual truth: `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-825edaf7-df4d-4423-a798-c9033d327e9b.png` (808 x 172 px) and `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-80981925-f5ed-4009-b339-02c3ad4f451a.png` (293 x 163 px).
- Browser-rendered implementation: `artifacts/design-qa/creation-skill-shortcuts-full-1280x720.png` and `artifacts/design-qa/creation-skill-token-full-1280x720.png`.
- Focused implementation evidence: `artifacts/design-qa/creation-skill-shortcuts-final-696x155.png` and `artifacts/design-qa/creation-skill-token-final-280x80.png`.
- Combined comparison input: `artifacts/design-qa/creation-skill-comparison.png`.
- Viewport: 1280 x 720 CSS px; browser reported DPR 1.25, while screenshots were emitted at CSS-pixel dimensions, so no additional density scaling was applied.
- State: light theme, Creation layout, Office scenario row; the focused token capture uses the real installed `excel-auto-zh` skill with the localized `金融服务` label to verify the same token treatment shown for `文档处理` in the source.

## Full-view and focused comparison

- The mode selector retains the source's centered segmented pill, dark selected state, muted inactive states, Lucide icons, and single-line horizontal scenario strip.
- Scenario chips retain the source's white surface, subtle border/shadow, full radius, icon alignment, and compact Chinese typography.
- The embedded skill retains the source's hammer icon, localized label, neutral token background, top-left composer placement, rounded input border, and removable-token affordance.
- The source crop uses a slightly larger gap between the mode and scenario rows than the current responsive 1280 x 720 layout. This is existing responsive spacing outside this behavior change and is P3 only.

## Required fidelity surfaces

- Fonts and typography: existing Chinese system-font stack, weights, line heights, and compact chip labels are preserved; no wrapping or truncation occurs.
- Spacing and layout: pill heights, radii, horizontal insets, one-row overflow behavior, composer token placement, and input border alignment remain intact.
- Colors and tokens: selected black, neutral inactive background, white scenario chips, gray borders, and token background continue to use the existing Creation theme tokens.
- Image quality and assets: no raster asset was added or replaced; UI icons continue to use the established Lucide icon set.
- Copy and content: visible labels stay localized; the token displays the scenario label while its hidden executable identity is the resolved real skill name.

## Interaction and validation

- Clicking a scenario queries the current skill snapshot, prefers a real installed or built-in match, enables a disabled match, and only then inserts the executable token.
- Browser verification resolved `金融服务` to the installed `excel-auto-zh` skill; DOM evidence showed `data-skill-token="excel-auto-zh"` with visible label `金融服务`.
- Hovering the token exposed its remove control; removal reduced the token count to 0 and clicking the scenario again restored it to 1.
- Missing or failed lookup falls back to the scenario prompt with a visible warning instead of inserting a fake executable token.
- Browser console errors/warnings: none.
- Frontend scenario interaction test: 108 passed, 0 failed. Resolver test passed. `pnpm build` passed.
- Root Go skill package and affected Desktop package tests passed.

## Comparison history and findings

1. Initial implementation exposed static scenario IDs as skill tokens even when no executable skill existed. Fixed by resolving against authoritative skill settings and rejecting fake tokens.
2. A fresh installation could still lack a matching user skill. Fixed by shipping inline built-in skills for every static creation scenario except `research`, which reuses the existing built-in research skill; user and project skills retain override precedence.
3. Final visual pass found no actionable P0, P1, or P2 difference. The responsive row-gap variance remains P3 and predates this behavior-only change.

final result: passed

---

# 自动化技能菜单可读性 QA（2026-08-15）

## 对照证据与状态

- 问题截图：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-3833afa1-0de7-4524-999b-99b506773c4e.png`。
- 修复截图：`automation-skill-menu-after.png`。
- 浏览器状态：1280 x 720 CSS px，DPR 1，浅色主题，自动化添加任务页，技能菜单展开态。

## Findings 与修复

- [P1 已修复] 技能菜单继承了紧凑工具栏触发器宽度，列表只剩图标和单字，技能名称不可辨认。
- 修复：复用通用 `HeartbeatSelect.menuMinWidth`，仅为技能选择器设置 260 px 的弹出层最小宽度；工具栏触发器尺寸保持不变。
- 复验：菜单实际宽度为 260 px；包括 `lingyi-wx-viral-script-generator` 在内的所有技能名称，其 `clientWidth` 与 `scrollWidth` 一致，无文字截断。

## 必检表面

- 字体与排版：沿用现有菜单字体、字号和字重，短名称与长名称均清晰可读。
- 间距与布局：图标、名称、选中勾和滚动条保留原间距，菜单不会挤压其他工具栏项。
- 颜色与令牌：背景、边框、悬停及选中颜色未改动。
- 图像质量：继续使用原技能图标或原回退标识，没有替换资产。
- 文案与内容：技能名称和说明数据保持原样。

## 交互与验证

- 已验证菜单展开、滚动、选择长名称技能和清空选择；未保存测试草稿。
- 页面日志无 error 或 warning。
- `pnpm build` 通过，包括 CSS 语法、z-index token、TypeScript 与 Vite 生产构建。
- 最终无待处理 P0、P1 或 P2 差异。

final result: passed

---

# 自动化专家菜单可读性 QA（2026-08-15）

## 对照证据与状态

- 问题截图：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-5e172b2f-c48a-4ab7-aee2-99c6e769c4ea.png`。
- 修复截图：`automation-expert-menu-after.png`。
- 浏览器状态：1280 x 720 CSS px，DPR 1，浅色主题，自动化添加任务页，专家菜单展开态。

## Findings 与修复

- [P1 已修复] 专家菜单继承了约 106 px 的紧凑触发器宽度，头像、名称和选中图标挤在一起，名称只能显示单字加省略号。
- 修复：为通用 `HeartbeatSelect` 增加可选 `menuMinWidth`，仅在专家选择器传入 260 px；工具栏触发器仍保持原紧凑宽度，弹出层则获得独立可读宽度。
- 复验：菜单实际宽度为 260 px；可见专家名称的 `clientWidth` 与 `scrollWidth` 一致，没有文本截断，头像、勾选图标和滚动条均正常。

## 必检表面

- 字体与排版：沿用现有 12.5 px 菜单字号和系统字体，专家名称可完整显示。
- 间距与布局：选项内头像、文本、勾选状态保持原 8 px 间距；菜单不再被触发器宽度压缩。
- 颜色与令牌：未改变背景、悬停、选中及边框令牌。
- 图像质量：继续使用原专家头像资源和圆形裁切，无占位替换。
- 文案与内容：专家名称、专业领域和选择结果保持原数据。

## 交互与验证

- 已验证菜单展开、滚动、选择专家、清空选择；未保存测试草稿。
- 页面日志无 error 或 warning。
- `pnpm build` 通过，包括 CSS 语法、z-index token、TypeScript 与 Vite 生产构建。
- 最终无待处理 P0、P1 或 P2 差异。

final result: passed

---

# 自动化全宽添加任务 UI QA（2026-08-15）

## 对照证据与状态

- 视觉源：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-b4709d71-a6fd-4cac-9770-95655e178240.png`。
- 实现截图：`automation-after-v2.png`。
- 参考图与实现图均以 1566 x 887 CSS px、浅色主题、自动化“添加任务”初始态进行同尺寸并排对照。

## 视觉与布局

- 添加/编辑态隐藏应用侧栏并将工作区扩展到全窗口；取消返回列表后侧栏恢复，匹配参考中的沉浸式编辑页且不改变全局导航行为。
- 顶部面包屑、取消/保存操作、52 px 蓝色提示条、全宽名称和工作空间控件、229 px 提示词编辑区、底部工具栏及连接器/执行频率的纵向节奏与参考一致。
- 提示词工具栏使用现有 Lucide 图标，并复用原模型、技能、专家与权限选择器；没有替换状态管理或保存字段。
- 参考图中的深色保存按钮代表可提交态；实现继续遵守原校验，名称和提示词为空时显示禁用态，填写两项后顶部与底部保存按钮同时可用。
- 新任务继续沿用原来的“按间隔 / 30 分钟”默认调度，未为匹配截图而改动业务默认值；周期、按间隔、单次以及后续高级设置均保留。

## 交互与回归

- 已验证名称和提示词输入、工作空间菜单（全局 / work_03）、权限菜单（Ask / Auto / 完全访问权限）、提示条关闭以及取消返回列表。
- 测试过程中未点击保存，没有创建或修改自动化任务数据。
- 页面日志无 error 或 warning；仅有 Vite 热更新与既有性能调试日志。
- `pnpm build` 通过，包括 CSS 语法、z-index token、TypeScript 与 Vite 生产构建。
- 最终无待处理 P0、P1 或 P2 视觉差异。

final result: passed

---

# 自动化添加任务视觉精修 QA（2026-08-15）

## 对照证据与状态

- 视觉源：`C:/Users/ai04/AppData/Local/Temp/codex-clipboard-05329c56-bf32-46e6-93bc-3dcd3146f11b.png`。
- 实现截图：`automation-after.png`；并排聚焦对照：`automation-design-comparison.png`。
- 浏览器视口：1417 x 932 CSS px，DPR 1，浅色主题，自动化“添加任务”初始态。
- 参考图像：1417 x 932 px；根据 Windows 125% 显示密度，将参考表单区域从 738 x 825 px 归一化为 590 x 660 px，与实现的 590 x 660 px 表单区域对照。

## 全视图与聚焦对照

- 全视图检查覆盖 Reasonix 原有侧栏、56 px 编辑页头、表单滚动区域和底部固定操作区；产品自身导航保持不变。
- 聚焦并排图覆盖提示条、名称、工作空间、提示词、技能/专家、模型、失败冷却及连接器区域，重要文字和控件均可清晰读取，无需额外局部裁切。
- 字体与排版：沿用 Reasonix 中文系统字体；标签、输入文字、辅助文案和提示条分别保持 14/14/12–13 px 层级，参考中的常规字重和弱化信息层级得到保留。
- 间距与布局：表单首屏上移 8 px 后，提示条、输入框和提示词区域的纵向位置与归一化参考一致；控件宽度、32 px 高度、8 px 圆角和 16 px 字段节奏一致。
- 颜色与令牌：继续使用现有主题变量；浅蓝提示条、灰色边界、白色表面和弱化文字与参考接近，并保持深色主题兼容。
- 图像与图标：该聚焦状态无位图内容；返回、下拉等标准图标继续使用现有图标库，没有新增占位图或手绘 SVG。
- 文案与内容：参考中的“提示、名称、工作空间、提示词、技能、专家、模型、失败冷却、连接器”等核心文案保持一致；Reasonix 独有的权限和调度字段继续保留。

## 交互、控制台与迭代记录

- 验证了进入自动化、打开添加页、填写名称和提示词、切换权限、打开模型选择器以及返回列表；必填项完成后添加按钮可用，未提交或持久化测试草稿。
- 浏览器控制台未发现 error。
- P2：首屏内容相对归一化参考整体下移约 8 px。已将编辑器顶部内边距从 24 px 调整为 16 px，复拍后名称输入框位于 y=159.6 px、提示词容器位于 y=313.2 px，与参考归一化位置一致。
- P3：提示条正文层级偏弱，窄宽度下长文本缺少明确换行约束。已将正文调整为 13 px / 20 px，并补充最小宽度和断行规则；键盘聚焦滚动同时为底部操作区预留 88 px。
- 最终无待处理 P0、P1 或 P2 差异。

final result: passed

---

## Project Composer, configuration rail, instruction dialog, and continuous calendar QA (2026-08-12)

### Implementation evidence

- Composer attached tray at 1294 x 800: `docs/archive/audits/project-composer-attached-tray-1294x800.png`
- Continuous authoritative calendar at 1294 x 800: `docs/archive/audits/project-continuous-calendar-1294x800.png`
- Instruction dialog at 1294 x 800: `docs/archive/audits/project-instruction-dialog-1294x800.png`
- Mobile calendar at 390 x 844: `docs/archive/audits/project-calendar-mobile-390x844.png`
- Runtime: authenticated Desktop development instance connected to the verified `team-integration` Cloud environment.

### Interaction and responsive verification

- The three project-context selectors are mutually exclusive, render as a single tray attached to the Composer context bar, restore focus, and retain keyboard navigation. The mobile overflow found during live QA was corrected so the Composer and tray remain within the 390 px viewport.
- Clicking `Auto` opens the searchable model list directly; selecting a model does not require a second navigation step.
- The configuration rail collapses completely. Its reopen control remains in the workspace grid as a narrow attached edge strip, and the expanded close control remains part of the rail header.
- The instruction editor uses one modal surface and one editable textarea, with source metadata, character count, fixed actions, Escape handling, and no duplicate read-only editor.
- The calendar initially loads the previous, current, and next months. Repeated PageDown extended the verified window through January 2027; PageUp loaded earlier months while preserving a bounded window; Today restored July/August/September 2026 around the current month.
- At 1170 x 800 and 390 x 844, the seven-column month grid keeps its minimum width and scrolls horizontally instead of compressing cells. The mobile configuration rail remains an overlay and closes without leaving a content-width gap.

### Visual findings

- P0: none.
- P1: none after fixing the mobile Composer intrinsic-width overflow.
- P2: the continuous calendar intentionally keeps its minimum grid width on narrow screens, matching the approved horizontal-scroll behavior.

final result: passed

---

## Project plan projections and composer selectors QA (2026-08-12)

### Source and implementation evidence

- Table reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-7ee7121d-0d8e-4a48-ad27-d9bf835ed3fb.png`
- List reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-97c37efc-bab7-445e-86bc-f8f433ccfbea.png`
- Gantt reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-256f7694-8196-4c00-9a2f-27c284fc87cd.png`
- Implementation captures: `docs/archive/audits/project-plan-table-1284x800-final.png`, `docs/archive/audits/project-plan-list-1284x800-final.png`, `docs/archive/audits/project-plan-gantt-1284x800-final.png`
- Side-by-side comparisons: `docs/archive/audits/project-plan-table-comparison.png`, `docs/archive/audits/project-plan-list-comparison.png`, `docs/archive/audits/project-plan-gantt-comparison.png`
- Responsive capture: `docs/archive/audits/project-plan-table-390x844-collapsed-final.png`
- State: light theme, authenticated project workspace, real Cloud WorkItems, configuration expanded for desktop comparisons and collapsed for mobile verification.

### Comparison and interaction QA

- The table uses the requested compact data grid with loaded-row selection, sequence, title, priority, status, handler and created time. Its horizontal overflow remains inside the projection surface when the project configuration rail reduces available width.
- The list is grouped by the four Cloud statuses, preserves authoritative counts, provides per-status creation and empty-state entry points, and keeps assignee and relative-update metadata aligned at the row edge.
- The Gantt view uses a fixed task table and synchronized timeline. Week, month, quarter and year controls, previous/next navigation and Today were exercised in the real browser. A real WorkItem received temporary start/due dates to verify the scheduled bar path and was restored to its original undated state through the same public editor.
- The project composer exposes task target, workspace and approval mode through the same themed selector. Cloud task mode disabled local workspace selection; local task mode, the workspace catalog and the approval catalog were restored without changing the existing project draft.
- Expert and skill capabilities were added through the public configuration UI, saved, reopened with `aria-pressed=true`, and restored to the initial snapshot. Connector entries were present but unavailable because this device has no configured connector; automation had no saved local definitions. Those two empty states were verified without fabricating data.
- At 390 x 844, the configuration drawer remained an overlay, could be collapsed, and the plan/composer surfaces retained their horizontal overflow rather than compressing controls into unreadable columns.

### Iteration result

- P0: none.
- P1: none.
- P2: none.
- P3: exact row density varies with the Reasonix application shell and configuration rail, while the requested hierarchy and interactions remain equivalent to the references.

final result: passed

## WorkBuddy board, assets, and project composer QA (2026-08-12)

### Source and browser evidence

- Board reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-1c6f1da2-a416-4248-9289-a6b2679139a5.png`
- Real Chrome board capture: `output/playwright/board-final-1334x644.png`
- Same-canvas comparison: `output/playwright/board-reference-vs-final.png`
- Desktop assets capture: `output/playwright/assets-final-1259x800.png`
- Mobile assets capture: `output/playwright/assets-final-390x844.png`
- Mobile dropdown capture: `output/playwright/dropdown-theme-390x844.png`

### Interaction and responsive QA

- Created five pending and three completed WorkItems through the authenticated project UI; moved one item to In Progress and back to Pending, and verified the Cloud-backed counts and cards after each authoritative reload.
- Verified Board, Table, List, Gantt, and Calendar projections. The Calendar correctly reported seven undated records before the final fifth pending item was added; the other projections displayed both pending and completed Cloud records.
- Uploaded `workbuddy-board-qa.md` and `project-asset-checklist.txt` through the Desktop native file picker. The directory readback showed both files and authoritative usage increased from 0 B to 294 B.
- At 1259 x 800, the asset toolbar reflows into balanced primary and filter/search rows. At 390 x 844, the toolbar uses a single-column mobile grid, the filter and search stay in the viewport, and assets render as cards.
- Project task targeting exposes Local Task and Cloud Task through the shared themed selector; Cloud Task disables local workspace selection and returning to Local Task restores it.
- All changed dropdown surfaces use the existing project theme tokens and the shared anchored popover rather than native platform styling.

final result: passed

## Project header, configuration drawer, Composer, and calendar QA (2026-08-12)

### Source and runtime evidence

- Header reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-26c50b18-f62e-4851-b207-198f51ec29ee.png`
- Composer reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-57a4232c-64cb-4b88-aa2c-886861930a99.png`
- Calendar reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-d45f295f-3cf7-475e-b6f6-842fcf269d30.png`
- Captures and side-by-side comparisons: `C:/Users/ai01/.codex/visualizations/2026/08/11/019fef0c-1657-7ad2-a349-5c1d216cbe87/project-qa`
- Verified viewports: 1653 x 800, 1294 x 800, 1239 x 800, and 390 x 844.
- Runtime: authenticated Desktop against the verified `team-integration` Cloud environment; health and readiness returned HTTP 200.

### Comparison and interaction QA

- The top bar now follows the reference hierarchy: `项目 / 当前项目名`, with Invite and the existing actions menu on the right; no duplicated configuration button remains.
- The configuration control remains visible at the workspace edge after collapse and reopens the drawer. Expanded and collapsed states were both captured; the main surface regains the full width when collapsed.
- The drawer retains instruction and capability controls while omitting the static synchronization, instruction-version, and save/clear explanatory text.
- The shared project Composer spans the available content width, uses the approved prompt, exposes transient workspace and approval context, displays `Auto` before an explicit model choice, and retains microphone and send controls. The 390 px viewport keeps these controls usable without clipping.
- The calendar renders a Monday-first fixed six-week grid with adjacent-month dates, the current-day marker, Chinese-calendar secondary text, authoritative undated count, previous/next month navigation, and Today restoration. Collapsed configuration produces the intended full-width month view; narrow screens retain the month-grid minimum width and horizontal access.
- Side-by-side inspection found no remaining P0, P1, or P2 discrepancy in hierarchy, spacing, overflow, or responsive behavior. The implementation intentionally retains the product navigation and data-driven current month rather than cropping them out to imitate the isolated reference frames.

### Iteration history

1. Rebuilt Cloud before Desktop and verified runtime ownership, environment identity, readiness, and matching contracts.
2. Found that the live model catalog converted the project fallback from `Auto` to `请选择模型`; added an explicit project-only unmatched-model presentation and regression test.
3. Re-captured all four target viewports and re-ran drawer and month-navigation interactions after the live-state fix.

final result: passed

## Project activity, plan board, and automation runs QA (2026-08-12)

### Source and implementation evidence

- Activity reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-a17dd1fe-ccc2-4f1c-bcf2-e8bff95f1c02.png`
- Plan reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-04382674-d4da-4b6b-a295-b64b2653e196.png`
- Automation-run reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-8628f30f-2475-4a16-abbb-237c2cdc4214.png`
- Activity capture: `docs/archive/audits/project-activity-1170x800-final.png`
- Plan captures: `docs/archive/audits/project-plan-board-1247x800-final-v2.png`, `docs/archive/audits/project-plan-1653x657-final.png`, and `docs/archive/audits/project-plan-390x844-final.png`
- Automation captures: `docs/archive/audits/project-task-1247x800-final.png` and `docs/archive/audits/project-task-390x844-final.png`
- Runtime: the verified `team-integration` stack with the rebuilt Desktop Wails process and live Cloud project data.

### Comparison and interaction QA

- The expanded activity composer uses one bordered surface. The focused textarea no longer creates a second inner outline; attachment, cancel, and disabled publish actions align with the reference.
- The plan toolbar retains all five live projections and keeps the left view group horizontally scrollable. Filter, search, label settings, and Add stay grouped at the right on desktop.
- The board renders four equal semantic columns with the reference's pale neutral surface, independent vertical lists, Cloud totals, compact cards, and horizontal overflow at narrow widths.
- The Tasks tab renders project-scoped local automation archives rather than session-history records. The live project currently has no archived runs, so the browser correctly shows the real empty state instead of seeded rows.
- The configuration rail can be removed completely from layout. When expanded, it contains project instructions and capability sections only; the project description, version card, and public local-directory binding action are absent.
- The mobile checks confirmed a 390 x 844 layout with horizontally scrollable plan columns, stacked automation filters, visible empty state, and a composer pinned below the project viewport.

### Iteration history

1. Removed the activity textarea's inherited inner focus outline while preserving a focus cue on the containing surface.
2. Replaced undefined project-only background tokens with the repository's `--bg-elev-2` token so selected views and plan columns remain visible in every supported theme.
3. Removed legacy dark styling from the All control and kept the actual selected plan projection visually distinct.
4. Rebuilt and restarted the verified integration stack, then repeated the browser comparison at all requested viewports.

No unresolved P0, P1, or P2 visual discrepancy remains. Differences in run count and work-item content are authoritative-data differences and were not replaced with hardcoded fixtures.

final result: passed

---

# Project Asset Library QA (2026-08-11)

## Evidence and target

- Source visual truth: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-bb773cd8-7c26-4029-a00d-7c3de7b7ebf6.png` (1653 x 836 px).
- Target states: 1653 x 836 desktop, 1170 x 800 compact desktop, and 390 x 844 mobile.
- Runtime: rebuilt Desktop and Cloud integration worktrees, Cloud migration 91, environment `team-integration`.

## Implemented fidelity surfaces

- Desktop uses the reference's flat full-width asset surface, compact toolbar, quota treatment, type/search controls, sortable table columns, restrained separators, and bottom project composer.
- Mobile changes the table into readable cards, keeps 44 px action targets, stacks filters, and presents row actions as a bottom menu without introducing a second component system.
- Icons use the existing Lucide dependency and all colors, typography, focus rings, borders, and spacing use current Reasonix tokens.
- No reference files or folders were inserted. Empty, loading, error, processing, archived, pagination, selection, dialog, and menu states render from Cloud data.

## Interaction and accessibility verification

- Vitest covers directory navigation, per-folder pagination, blur/Enter search, IME Enter protection, type and sort queries, multi-select, quota/upgrade, archived read-only behavior, stale-request isolation, and accessible labels.
- Dialogs support Escape, focus trapping, and focus restoration. Folder navigation, sorting, selection, rename, move, delete, download, and upgrade use semantic controls.
- Cloud and Wails contract tests cover query binding, strict response validation, nullable root moves, version conflicts, and mutation readback.

## Browser comparison status

- The rebuilt Wails endpoint returned HTTP 200 and the integration launcher verified Cloud/Desktop listener ownership and environment identity.
- The Codex in-app browser rejected navigation to the local Wails URL under its URL safety policy. Per browser safety rules, no alternate browser or indirect automation was used to bypass the block.
- Because no fresh implementation screenshot could be captured, the mandatory side-by-side 1653 x 836 / 1170 x 800 / 390 x 844 visual comparison remains unverified. This is a QA evidence blocker, not a known application failure.

## Findings

- P0: none established by tests or runtime verification.
- P1: visual evidence incomplete because the required in-app browser could not access the local URL.
- P2: none established by source review and responsive contract tests.
- P3: the live project may have an empty asset directory; the implementation intentionally renders the real empty state instead of screenshot fixtures.

final result: blocked on in-app browser visual capture

---

# Project Workspace And Plan Views QA (2026-08-11)

## Evidence

- Source visual truth: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-e46154d9-bf1d-4a70-a6db-288ea79c3c2f.png` (1653 x 657 px).
- Final implementation at the matching viewport: `docs/archive/audits/project-plan-1653x657.png` (1653 x 657 CSS px at device pixel ratio 1).
- Native side-by-side comparison: `docs/archive/audits/project-plan-1653x657-comparison.png` (3306 x 657 px).
- Narrow viewport evidence: `docs/archive/audits/project-plan-1170x800-collapsed.png` (1170 x 800 CSS px at device pixel ratio 1).
- State: light theme, authenticated `WorkBuddy 视觉验收` project, plan tab selected, project configuration collapsed, real Cloud work items loaded.

## Full-view and focused comparison

- The final plan surface keeps the reference's four fixed workflow columns, quiet gray column backgrounds, compact white cards, top toolbar, column-level add controls, and dense status/priority/assignee/time metadata.
- The implementation deliberately retains the requested `看板 / 表格 / 列表 / 甘特 / 日历` switcher and real Cloud search/filter/label controls; the supplied reference only shows the board subset.
- At 1653 x 657 all four columns are visible inside the main work surface. At 1170 x 800 the same four-column board keeps usable card widths and scrolls horizontally inside the plan surface instead of compressing cards.
- The reference is a cropped center-workspace image; the implementation comparison includes the persistent Reasonix navigation and composer as existing product constraints.
- The project configuration panel is absent from the DOM when collapsed. The workspace expands to the available width without the former empty rail track.

## Interaction verification

- Confirmed all five view buttons render and switch to distinct live surfaces: `[data-plan-board]`, `[data-plan-table]`, `[data-plan-list]`, `[data-plan-gantt]`, and `[data-plan-calendar]`.
- Opened and closed the real label-management dialog without mutating data.
- Opened and cancelled the real create-plan-item dialog without mutating data.
- Confirmed each board card exposes title, status, priority, assignee, and relative update time from Cloud data.
- Confirmed the four columns show Cloud total counts and each column owns its create action.
- Browser warning/error log after the interaction pass: 0 entries.

## Iteration history and findings

- First P1: collapsing project configuration removed the panel but left the grid's 320 px track. Fix: apply the single-column workspace grid only while configuration is hidden.
- First P2: board cards omitted the status pill required by the reference and product plan. Fix: add compact semantic status pills while retaining priority, assignee, and relative time.
- Second P2: a legacy broad card-span rule stretched the new status and priority pills across the card. Fix: narrowly reset direct metadata and footer spans to intrinsic width.
- Post-fix browser measurements at 1170 x 800: workspace width 869 px, board content width 960 px with contained horizontal scrolling, and no configuration rail rendered.
- P0: none.
- P1: none.
- P2: none.
- P3: real integration data contains one work item per status and no priority or assignee, so the final capture truthfully shows `无优先级` and `未指派` rather than copying reference fixtures.

final result: passed

## Project live search, configuration rail, and activity feed QA (2026-08-11)

### Source and implementation

- Expanded configuration reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-31e0e192-c367-4900-80e9-7cec37ced162.png` (317 x 833 px).
- Collapsed configuration reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-3301c2f8-563d-41c0-8a3f-6e48c8a42a37.png` (248 x 830 px).
- Activity-feed reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-16ad6785-87e9-4608-9a3d-de3deb82dade.png` (1651 x 586 px).
- Expanded implementation: `docs/archive/audits/project-workspace-configuration-expanded.png` (1280 x 720 px).
- Collapsed implementation: `docs/archive/audits/project-workspace-configuration-collapsed.png` (1280 x 720 px).
- Browser viewport: 1280 x 720 CSS px at device pixel ratio 1, light theme, authenticated `WorkBuddy 视觉验收` project.
- Density normalization: screenshots were compared at their native DPR 1 sizes. The narrow reference crops were judged against the corresponding right-rail and activity regions in the full implementation captures.

### Full-view and focused comparisons

- The expanded state keeps the requested 320 px right configuration rail, independent scrolling, compact bordered cards, and a top-right collapse control. The center activity area remains the primary surface.
- The collapsed state reduces the rail to a 48 px right-side control strip and releases the remaining width to the activity feed, matching the supplied collapsed composition without removing project data.
- The activity feed now uses one continuous horizontal timeline: compact circular actor mark, bold masked account, inline action, short month/day date, quiet separators, and a right-aligned filter cluster.
- Focused comparison was required for the rail toggle, activity row rhythm, and timestamp treatment; all three were inspected in the combined source/implementation browser comparison.
- The project directory search exposes only the searchbox and refresh control. Typing `修改bug` automatically reduced the live cloud result set to that project after the 250 ms debounce, with no submit button.

### Required fidelity surfaces

- Fonts and typography: existing Reasonix UI typography is retained; actor names use 600 weight, action copy uses the secondary foreground, and dates use tabular numerals.
- Spacing and layout rhythm: 74 px activity rows and quiet 1 px separators reproduce the reference's dense feed; the configuration rail uses the existing 4 px spacing system and 12 px card radii.
- Colors and visual tokens: all surfaces, text, borders, focus rings, and unread treatment use existing product tokens; no reference-only brand color was introduced.
- Image quality and asset fidelity: these states contain standard Lucide interface icons and no raster content requiring substitution. Existing project capability icons remain cloud-provided.
- Copy and content: project names, actors, activities, dates, instructions, and capabilities come from the live Cloud data. No reference fixture text was fabricated.

### Interaction and accessibility verification

- Search updates without form submission and preserves the existing server-side search contract.
- `收起项目配置` and `展开项目配置` retain a 40 x 40 px hit target, `aria-expanded`, `aria-controls`, visible focus styling, and focus on the same toggle position.
- Expanded and collapsed states both keep the `项目配置` complementary landmark available to assistive technology.
- The real activity feed displayed eight cloud events with shortened dates, while publish, related/member filters, refresh, and the shared project composer remained usable.
- Browser console errors: 0.

### Comparison history and findings

- Earlier P1: project search required a separate submit action despite the desired direct filtering flow.
- Fix: removed the submit button and introduced a race-safe 250 ms debounced Cloud search.
- Earlier P1: the configuration rail permanently occupied 320 px and could not release space to project work.
- Fix: added an accessible expanded/collapsed rail state with a 48 px collapsed track.
- Earlier P2: activity data appeared as isolated rounded cards with full timestamps, diverging from the supplied compact timeline.
- Fix: converted the list to continuous separated rows and month/day timestamps while preserving authoritative event data.
- Post-fix evidence: both saved implementation captures and the live browser DOM show the corrected states; no actionable P0, P1, or P2 differences remain.
- P3: the implementation retains the existing project identity/binding card in the expanded rail and the Reasonix neutral avatar token, because those are functional product constraints not requested for removal.

final result: passed

---

## Project template capability configuration QA (2026-08-11)

### Source and implementation

- Reference: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-f33160ca-9e7c-42c4-8d8d-d5c550a968a4.png`
- Implementation capture: `docs/archive/audits/project-template-capabilities-selected.png`
- Combined comparison: `docs/archive/audits/project-template-capabilities-comparison.png`
- Browser viewport: 1280 x 720 at device pixel ratio 1.
- State: light theme, authenticated project directory, `市场调研与竞品分析` template open, one real expert and one installed skill selected in the draft.

### Comparison and interaction QA

- The create surface follows the reference hierarchy: instruction editor, optional connector, expert, and skill sections, selected capability chips, remove controls, and fixed footer actions.
- The existing required project-name field and template selector remain in place; these are intentional Reasonix workflow differences from the reference.
- Selecting `会计核算专家` and the installed `cloudbase` skill returned both items to the create dialog without writing a project or mutating cloud state.
- Opening the connector picker reached the real connector catalog. The current live profile had no personal connector configured, so the UI showed its truthful empty state instead of fabricated entries.
- The first browser pass exposed the outer dialog scrolling its header offscreen after picker focus restoration. The dialog was changed to a bounded flex column so only the body scrolls; header and footer now remain fixed.
- The compacted body leaves all three capability sections visible in a 720 px viewport while preserving a small internal overflow for longer content.
- Vitest covers a real connector selection and verifies the capability binding is saved only after project creation.
- No unresolved P0, P1, or P2 visual or interaction issue remains.

final result: passed
---

# Featured Skills And Header Actions Follow-up QA

## Evidence

- Source visual truth: `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-2c7b1be9-5632-49c7-acb3-89c9fb8c779c.png`.
- Final desktop-app implementation: `D:/project/Reasonix/artifacts/design-qa/skill-featured-final-desktop.png`.
- Combined focused comparison: `D:/project/Reasonix/artifacts/design-qa/skill-featured-comparison.png`.
- Source pixels: 1572 x 309. Final app capture: 1535 x 816. Focused implementation crop: 1260 x 427.
- Density normalization: the 1572 px-wide source was downsampled to the 1260 px app-content width; both captures use ordinary desktop density with no device-frame scaling.
- State: light theme, public catalog loaded, four featured skills visible, default `全部` category selected, one local skill installed.

## Full-view comparison

The final implementation keeps the existing Reasonix application sidebar and capability navigation, then places search, `我安装的`, its live count, and `添加技能` on the same header row as `专家 / 技能 / 连接器`. The new `精选技能` section begins immediately below that row and uses the same four-column compact cards as the reference. The standard `技能` heading, horizontal categories, and full catalog continue below it.

## Focused comparison

The combined artifact puts the supplied reference and the final app crop in one image. Header alignment, four-card density, title/description hierarchy, 20 px card radius, square artwork, top-right install control, section spacing, and `换一换` placement were inspected together. Reasonix intentionally retains its mint active-navigation token and persistent sidebar rather than copying the reference's black active tab.

## Interaction verification

- `换一换` rotates the first featured item from `爆款内容预检（付费版）` to `Agently Mail`.
- Search hides the featured rail while filtering and returns it when the query is cleared.
- `我安装的` opens the installed view, exposes the live installed count, and provides a `技能库` return action.
- `添加技能` remains wired to the existing local skill-folder picker; it was not invoked during QA to avoid changing user data.
- Direct card installation and installed-state behavior remain covered by the targeted capabilities-panel action test.
- The browser console contained no errors during the interaction pass before the local dev server hot-reloaded; the final desktop-app capture loaded without an error overlay after the dev server restarted.

## Required fidelity surfaces

- Fonts and typography: existing Reasonix Chinese system-font stack, 22 px section headings, strong 16 px card titles, two-line descriptions, ellipsis, and line height remain consistent with the reference.
- Spacing and layout rhythm: header controls share one row; the featured section begins at 108.8 px in the measured app viewport; cards use the established four-column, 10 px-gap, 136 px-high compact layout.
- Colors and visual tokens: neutral white/gray surfaces match the reference while existing mint focus, active, and installed semantics remain intact.
- Image quality and asset fidelity: all skill artwork comes from live catalog image URLs; Lucide is used only for standard search, add, refresh, installed, and back controls. No placeholder or custom-drawn image assets were introduced.
- Copy and content: `精选技能`, `换一换`, `技能`, `我安装的`, and `添加技能` match the requested Chinese UI; card content stays sourced from the live catalog.

## Comparison history and findings

- Initial P2: the right-side controls occupied a separate content row, leaving more vertical whitespace than the reference and weakening the relationship with the capability navigation.
- Fix: positioned the skill toolbar inside the capability header at desktop widths and restored the in-flow sticky toolbar below 1120 px to prevent narrow-layout overlap.
- Post-fix evidence: the measured navigation spans y=0–66.8, the toolbar y=14–54, and the featured section starts at y=108.8; the final desktop capture confirms no clipping or overlap.
- P0: none.
- P1: none.
- P2: none.
- P3: live catalog titles are longer than the reference examples and truncate earlier because the persistent Reasonix sidebar reduces content width. This is an intentional product constraint.

final result: passed

---

# Full-width Skill Workspace Follow-up QA

## Evidence

- Source visual truth: `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-8e0ec54e-76cc-4de6-ac48-5145e31c1bc8.png`.
- Browser-rendered implementation: `D:/project/Reasonix/artifacts/skills-full-width-1509x847.png`.
- Combined comparison evidence: `D:/project/Reasonix/artifacts/skills-full-width-comparison.png`.
- Source pixels: 1509 x 847. Implementation pixels and CSS viewport: 1510 x 847 at device pixel ratio 1. The one-pixel width difference comes from the browser viewport override rounding and did not require density normalization.
- State: light theme, public skill catalog selected, live records loaded, left application navigation expanded in the implementation.

## Full-view comparison

The source and implementation were placed in one vertical comparison artifact and inspected at the same desktop height. The requested scope is the skill workspace's horizontal frame. The implementation now removes the inherited 1440 px maximum and uses the complete available workspace width. Browser measurements confirm the workspace right edge is exactly the viewport right edge (1510 px), with no document-level horizontal overflow.

The source capture omits the persistent application sidebar while the implementation retains it as an existing product constraint. Internal content padding remains balanced so the fixed tab bar, category row, and card grid align consistently without touching the window edge.

## Focused comparison

The right-edge relationship was checked directly from the combined artifact and DOM measurements. At the reference-sized viewport, the application content shell and `.primary-workspace--skills` both end at x=1510; the skill library's inner right edge is x=1443 after its intentional 56 px content padding and scrollbar gutter. No extra centered max-width gap remains.

## Required fidelity surfaces

- Fonts and typography: unchanged; existing Reasonix font family, weights, wrapping, and truncation remain intact.
- Spacing and layout rhythm: only the outer width constraint changed. Internal padding, category rhythm, card gaps, radii, and sticky-header spacing remain unchanged and balanced.
- Colors and visual tokens: unchanged; the existing light theme and semantic state colors are preserved.
- Image quality and asset fidelity: live catalog artwork remains sharp and uses the original API assets; no substitute assets were introduced.
- Copy and content: unchanged; live catalog names, descriptions, categories, and installed count are preserved.

## Interaction and runtime verification

- Confirmed the public catalog remains visible and the skill/category tab semantics are intact.
- Confirmed the full-width workspace has no page-level horizontal overflow.
- Confirmed no browser console errors after rendering at the source-sized viewport.
- Ran the targeted capabilities-panel action test and the full frontend production build successfully.

## Comparison history and findings

- Initial P2: the skill workspace inherited a 1440 px maximum width and centered margin, which could leave an unused strip on the right when the available main area exceeded that cap.
- Fix: scoped the skill workspace to `width: 100%`, `max-width: none`, and zero inline auto margin; also made the skill library explicitly fill its parent.
- Post-fix evidence: the measured workspace spans x=264 through x=1510 in the application-shell state and its right edge exactly matches the viewport boundary.
- P0: none.
- P1: none.
- P2: none.
- P3: none for the requested full-width scope.

final result: passed

---

# Compact Skill Cards Follow-up QA

## Evidence

- Source visual truth: `D:/project/Reasonix/artifacts/design-qa/skill-library-reference.png` (copied from the user-provided screenshot).
- Final browser-rendered implementation: `D:/project/Reasonix/artifacts/design-qa/skill-library-implementation-final-1558x900.png`.
- Combined full-view comparison: `D:/project/Reasonix/artifacts/design-qa/skill-library-comparison-final.png`.
- Combined focused card comparison: `D:/project/Reasonix/artifacts/design-qa/skill-library-comparison-focused-final.png`.
- Source pixels: 1558 x 357. Implementation pixels and CSS viewport: 1558 x 900 at device pixel ratio 1. No density conversion was required.
- State: light theme, public catalog selected, `全部` selected, 20 live API records loaded, first skill already installed.

## Full-view comparison

The reference and final browser capture were reviewed together in a single comparison artifact. Both use a single horizontal category row followed immediately by a four-column grid. Cards match the reference's 136 px height, 20 px radius, quiet white surface, 10 px grid gap, square artwork, bold single-line title, two-line description, and 34 px top-right add control. Reasonix's persistent product shell remains visible around the component and is treated as an intentional product constraint.

## Focused comparison

The first reference card and first rendered card were combined at native card height for detailed inspection. Icon scale, title weight and truncation, description line height, top/right control alignment, radius, and neutral surface treatment are consistent. The reference's first card is shown in a darker hover/selected treatment; the implementation exposes the same darker surface on hover while the captured installed card correctly shows its green completed state.

## Required fidelity surfaces

- Fonts and typography: the existing Reasonix Chinese system font stack remains in use; titles render at 16 px with a strong optical weight, descriptions at 14 px / 1.55 line height, and long live titles truncate without wrapping into the action control.
- Spacing and layout rhythm: category controls are 36 px high; cards are 136 px high with 24 px left padding, 10 px internal gap, 10 px grid gaps, 20 px radii, and no heavy border or elevation.
- Colors and visual tokens: neutral foreground and surface tokens reproduce the restrained gray/white reference; hover, installed, focus, and disabled states continue to use semantic product tokens.
- Image quality and asset fidelity: live API artwork renders as the real image asset; installed marketplace icons persist and render after installation. Lucide is used only for standard UI add/completed controls. No custom SVG, CSS art, emoji, or placeholder imagery was introduced.
- Copy and content: titles, descriptions, categories, and artwork come from the live skill catalog. Extra card metadata was moved back into the existing detail dialog rather than duplicated on the compact card surface.

## Interaction verification

- Selected `内容创作`; the catalog filtered from 20 to 5 cards and exposed `aria-selected="true"`.
- Restored `全部`; all 20 cards returned.
- Opened the first skill detail dialog, confirmed live detail content, and closed it with the labelled close button.
- Verified the installed shortcut remains available and the installed card presents a completed state instead of an install action.
- At a 900 px viewport, verified the 535.6 px skill content region collapses to one full-width card instead of two cramped 262.8 px cards.
- At a 1558 px viewport, verified the 1172 px skill content region renders four equal 285.5 px columns with no page or section overflow.
- Checked browser console errors after filtering, opening, and closing the detail dialog; none were present.

## Comparison history

- Initial P2: the responsive grid used viewport-based breakpoints, so at a 900 px window the sidebar left only a 535.6 px content region containing two 262.8 px cards. This over-truncated live titles and diverged from the reference card proportions.
- Fix: changed the skill library to an inline-size container and routed 4/3/2/1-column breakpoints from the actual component width.
- Post-fix evidence: the same 900 px viewport now renders one 535.6 px card with no overflow; the 1558 px desktop state continues to render four columns. The final browser screenshot and combined comparison were recaptured after this fix.

## Findings

- P0: none.
- P1: none.
- P2: none.
- P3: Reasonix's fixed left navigation makes each desktop card narrower than the standalone reference crop, so long live catalog titles truncate earlier. The four-column density and component proportions remain faithful inside the available product content width.

final result: passed

---

# Top-right Skill Header Placement Follow-up QA

## Evidence

- Source visual truth: `C:/Users/ai04/AppData/Local/Temp/codex-clipboard-4ca188c6-6851-4d8b-8b75-ec25aef8b952.png` (1573 x 98 px).
- Browser-rendered implementation: `D:/project/Reasonix/artifacts/design-qa/skill-header-top-right-final.png` (1280 x 720 px, device pixel ratio 1).
- Focused comparison: `D:/project/Reasonix/artifacts/design-qa/skill-header-top-right-comparison.png` (1016 x 149 px).
- Compared implementation header crop: 1016 x 70 px. The reference was proportionally normalized to the same 1016 px comparison width; the source and implementation viewport widths are recorded separately to avoid treating scale differences as layout defects.
- State: light theme, skill workspace selected, public catalog loaded, one installed skill.

## Full-view and focused comparison

The full browser capture confirms that the skill actions are now children of the page's top capability navigation rather than visually overlaid from the scrolling skill content. The focused comparison verifies the requested composition: capability tabs on the left and search, installed count, and add-skill actions on the far right of the same top row.

Browser measurements at the 1280 x 720 viewport show the navigation at x=264–1280 and y=0–68.8, with the action group at x=743.8–1256 and y=14–54. The right inset is exactly 24 px, matching the reference's top-right intent. The action host's parent is `.capability-workspace-nav.capability-workspace-nav--skills`.

## Required fidelity surfaces

- Fonts and typography: existing Reasonix navigation and control typography remain unchanged and readable.
- Spacing and layout rhythm: left navigation and right actions share one 68.8 px header row; controls are vertically centered and end 24 px from the right edge.
- Colors and visual tokens: existing mint active state and neutral action surfaces are retained as product design-system constraints.
- Image quality and asset fidelity: this header contains standard Lucide interface icons only; no substitute raster or custom-drawn assets were introduced.
- Copy and content: `搜索技能`, `我安装的`, the live installed count, and `添加技能` match the supplied reference.

## Interaction verification

- Opened the skill workspace and confirmed all three action controls are inside the top navigation landmark.
- Opened `我安装的`; the portaled toolbar stayed inside the top navigation and exposed the `技能库` return action.
- Returned to the catalog and confirmed the featured and regular skill content remained intact below the header.
- Browser logs contained no errors; only normal Wails/Vite connection and hot-update messages were present.
- The targeted capabilities-panel action test and production frontend build passed after the structural header change.

## Comparison history and findings

- Earlier P1: the toolbar was positioned from the skill content shell, which looked aligned but was not actually owned by the top navigation requested by the user.
- Fix: added a dedicated action host to `CapabilityWorkspaceNav` and portaled the live skill toolbar into that host; removed the content-shell absolute positioning.
- Post-fix evidence: DOM inspection confirms `.skill-library__topbar` is a direct child of the top navigation action host, and browser measurement confirms a 24 px right inset with no overlap.
- P0: none.
- P1: none.
- P2: none.
- P3: the implementation keeps Reasonix's mint selected-tab treatment rather than the reference's black selected tab.

final result: passed

---

## WorkBuddy project capability modal QA (2026-08-08)

### Source and implementation

- Reference: `C:\Users\ai01\AppData\Local\Temp\codex-clipboard-66b33feb-5007-41b1-b84a-a87d16f11019.png`
- Implementation capture: `D:\project\Reasonix-worktrees\integration-main-v3-20260807\.codex-run\qa-skill-modal-current.png`
- Combined comparison: `D:\project\Reasonix-worktrees\integration-main-v3-20260807\.codex-run\design-qa-skill-comparison-final.png`
- Source viewport: 1047 x 776 px.
- Implementation viewport: 1163 x 800 CSS px at DPR 1; normalized to 1047 x 776 px in the combined comparison.
- State: light theme, authenticated project workspace, skill capability modal open, one persisted cloud skill binding selected.

### Comparison record

- Full-view comparison covered the modal header, search and category controls, three-column capability grid, selected-count feedback, scroll treatment, and footer actions.
- The first pass found a translucent modal surface, weak card boundaries, an overlapping tab/grid layout, and overly tall descriptions.
- The implementation was corrected to use the product theme tokens, an opaque elevated surface, bounded cards with line clamping, a non-overlapping flex layout, cloud-provided icons, and catalog-derived categories.
- The final comparison has no unresolved P0, P1, or P2 visual issues. Differences in exact catalog items and labels are data-driven and do not affect the interaction or hierarchy.

### Interaction evidence

- Opened the project skill selector without leaving the project workspace.
- Searched and selected a public cloud skill, saved it, closed the modal, reopened it, and confirmed the persisted selection was read back.
- Verified custom project selectors expose keyboard navigation and focus restoration through their existing component contract.
- Verified the project model selector prepares a project-scoped session without requiring a local workspace binding.

---

# 自动化工作台设计 QA

日期：2026-08-01
目标：对照用户提供的 WorkBuddy「定时任务」与「添加自动化任务」截图，验证 Reasonix 的自动化工作台布局、交互和响应式行为。

## 对照证据

- 参考图：用户提供的 WorkBuddy 定时任务页与添加任务页。
- 实现截图：[design-qa-automation-workspace.png](design-qa-automation-workspace.png)
- 运行环境：目标 worktree 的 Wails 开发实例，`http://127.0.0.1:34292`。

## 检查结果

- 默认进入「定时任务」，可切换「运行记录」；方向键、Home、End 可切换并移动焦点。
- 空状态、添加自动化入口、12 个固定模板均可见；模板区在桌面为三列，并在窄屏降为两列和一列。
- 内容在工作区内部滚动，不挤压左侧导航。
- 添加页包含顶部取消/保存、在线运行提示、名称、工作空间、提示词、模型、技能、专家、连接器、权限、频率、生效区间、错过执行和通知配置。
- 满配额时添加入口和模板入口不可操作，并展示配额提示。
- 定向 DOM/交互测试、类型检查和视觉对照均通过；开发实例交互过程中未观察到本次功能引入的可见运行错误。

## 差异说明

- 沿用 Reasonix 自身的侧边栏、主题变量、字体和品牌色，不复制 WorkBuddy 的品牌资产。
- 项目级 `catch_up` 在缺少项目管理员授权边界时被后端拒绝并从项目任务选项隐藏，避免静默扩大权限。
final result: passed

## Project directory annotation QA (2026-08-11)

### Source truth

- Annotation 4 artwork: `C:/Users/ai01/.codex/attachments/af2cb96d-f986-4168-9a88-a49af40b64a1/image-1.png`
- Annotation 7 project-card menu: `C:/Users/ai01/.codex/attachments/af2cb96d-f986-4168-9a88-a49af40b64a1/image-2.png`
- Product requirements and five complete prompts: `C:/Users/ai01/.codex/attachments/af2cb96d-f986-4168-9a88-a49af40b64a1/goal-objective.md`

### Implementation evidence

- Final page screenshot: `docs/archive/audits/project-directory-implementation-menu-open-v3.png`
- Artwork side-by-side comparison: `docs/archive/audits/project-directory-hero-comparison.png`
- Project-card menu side-by-side comparison: `docs/archive/audits/project-directory-menu-comparison.png`
- Browser viewport: 1170 x 800 at device pixel ratio 1
- Source artwork: 629 x 210; rendered artwork: 585 x 195.296875; browser natural dimensions: 629 x 210
- Source project-card reference: 759 x 190; implementation project list: 821 x 176
- State: light theme, authenticated against `team-integration`, three live projects visible, first project menu open

### Comparison and interaction QA

- The hero uses the supplied bitmap directly. Subject order, line art, white background, and full wide composition match the source without circular masking or cropping.
- The directory preserves the reference's two-column card composition. Each card owns a three-dot trigger; the popover is anchored to its card and presents Rename plus a red destructive Delete action.
- The five requested templates appear below the project list in the required product order.
- Rename opened `编辑云项目` with `WorkBuddy 项目验收 2026-08-06` prefilled; the dialog was closed without mutation.
- Delete opened `永久删除云项目` with irreversible-action copy; the dialog was closed without mutation.
- `产品需求全流程` opened `新建项目`; its instruction contained 1,689 characters and included EARS, `阶段六：上线复盘`, and the final confirmation reminder.
- A manual WebView reload produced one transient Wails IPC `send` error at 08:54:44Z while the bridge reconnected. The subsequent stable Rename, Delete, template, and menu interactions produced no new warnings or errors.
- No remaining P0, P1, or P2 visual mismatch was found in the final side-by-side comparisons.

### Iteration history

1. Corrected the CSS aspect ratio from `627 / 218` to the source's actual `629 / 210` dimensions.
2. Increased menu focus selector specificity so the intended subtle focused background replaced the inherited generic 3 px outline while retaining keyboard focus behavior.
3. Regenerated the final screenshot and both side-by-side comparisons after the fixes.

final result: passed

---

# Project Workbench Editor And Configuration QA (2026-08-14)

## Evidence and state

- Source visual truth: `C:/Users/ai01/AppData/Local/Temp/codex-clipboard-dbe2a81b-467a-40aa-83cd-f37894879fa0.png`, `codex-clipboard-ff2d29d0-f084-4466-a8d7-67a568f6f7e9.png`, `codex-clipboard-fa1eb693-305b-4591-b104-e27e97c3d0cb.png`, `codex-clipboard-9325e5b6-11b5-4863-830f-dd9ed5b2cac2.png`, and `codex-clipboard-e1e6ed8c-a288-4c85-9e36-90796aa5fc46.png`.
- Implementation: `http://127.0.0.1:5175/`, authenticated project `UX验收0814完成`, light theme, new-work-item editor.
- Browser viewport: 1280 x 720 CSS px at DPR 1. Editor source and implementation status captures are both 880 x 465 px without density scaling.
- Final captures: `reports/design-qa-assets/work-item-status-final.png`, `work-item-assignee-final.png`, `work-item-priority-final.png`, `project-configuration-inline.png`, and `table-assignee-selector.png`.
- Side-by-side comparisons: `reports/design-qa-assets/work-item-status-comparison-final.png`, `work-item-assignee-comparison-final.png`, and `work-item-priority-comparison-final.png`.
- Full findings and verification record: `reports/project-workbench-design-qa-2026-08-14.md`.

## Full-view and focused comparison

- The editor matches the reference's 880 px white surface, title/description hierarchy, bottom attribute row, divider, attachment action, and right-aligned cancel/create actions.
- Status and assignee open above their triggers; priority opens below. Attribute order, compact pill density, semantic status colors, priority badge colors, and menu widths were compared in the combined images.
- The reference full-screen icon remains intentionally absent because this product flow previously removed full-screen editing; close, keyboard dismissal, and focus restoration remain available.
- Date fields continue to use the browser-native date picker. Native browser chrome is not present in page screenshots; trigger styling and start-before-due validation are covered by the DOM tests.

## Required fidelity surfaces

- Fonts and typography: existing Reasonix Chinese system-font stack, weights, line height, wrapping, and muted hierarchy are retained.
- Spacing and layout: editor measures 880 x 465; all six properties share one bottom row and wrap only at narrow widths.
- Colors and tokens: gray/blue/orange/teal statuses and gray/pink/orange/blue/green priorities reproduce the supplied semantic palette.
- Image quality: the flow has no raster imagery; standard interface icons reuse Lucide and no custom SVG or placeholder asset was introduced.
- Copy and content: labels follow the supplied Chinese terminology while member names remain authoritative project data.
- Accessibility and responsiveness: listbox/option semantics, `aria-expanded`, Escape focus restoration, axe checks, and 820 px geometry checks pass. Expanded configuration is an in-flow right grid column; at narrow widths it becomes an in-flow lower grid row rather than an overlay.

## Comparison history and findings

1. P2: generic 100% width controls produced stacked mechanical bars, and compact configuration used an absolute overlay. Fixed intrinsic pill sizing and converted compact/narrow configuration to in-flow grid tracks.
2. P2: status icons lacked the reference colors, redundant chevrons widened status/priority pills, and the priority menu was too narrow. Added semantic colors, editor-specific chevron control, and measured 124/138/220 px menu widths.
3. Final pass: no actionable P0, P1, or P2 difference remains. P3 differences are the intentional full-screen omission and live member-count variance.

## Interaction and validation

- Real app smoke covered configuration expand/collapse, table assignee menu, all editor property menus, Escape focus restoration, date inputs, and cancel without mutating project data.
- Vitest: 24 files and 150 tests passed.
- Browser suite: 37/37 passed, including axe, visual snapshots, keyboard reachability, and narrow-layout containment.
- Production build passed CSS syntax, z-index tokens, TypeScript, and Vite build.

final result: passed

---

# Experience Plan Layout and Quota Feedback Design QA

**Comparison Target**

- Source visual truth: `C:\Users\ai02\.codex\generated_images\01a00e9d-9206-7800-9a4f-e040723762f8\exec-f0906105-22d4-40f5-84c9-963f08dcef8a.png`
- Implementation screenshot: `C:\Users\ai02\.codex\visualizations\2026\08\17\01a00e9d-9206-7800-9a4f-e040723762f8\implementation-final.png`
- Full comparison: `C:\Users\ai02\.codex\visualizations\2026\08\17\01a00e9d-9206-7800-9a4f-e040723762f8\design-comparison-final.png`
- Focused card comparison: `C:\Users\ai02\.codex\visualizations\2026\08\17\01a00e9d-9206-7800-9a4f-e040723762f8\card-comparison-final.png`
- Source pixels: 1487 × 1058. Implementation pixels/CSS viewport: 1280 × 911 at DPR 1. The implementation was proportionally normalized to 1487 × 1058 for the full-view comparison.
- State: authenticated Free account, no active orders, Alipay selected, experience plan available.

**Findings**

- No actionable P0/P1/P2 findings remain.
- Fonts and typography: hierarchy now matches the reference with a prominent plan title, price, total points, and stacked unlimited values; the existing Segoe UI/Microsoft YaHei product stack is intentionally retained.
- Spacing and layout rhythm: the selected split summary/2×2 benefits layout, dividers, CTA alignment, footer notes, radii, and monochrome elevation are preserved. The dark outer preview canvas differs from the generated mock only because the isolated harness retains the real desktop modal backdrop.
- Colors and visual tokens: implementation uses the existing foreground, border, elevated background, and payment-brand tokens and matches the reference's restrained monochrome palette.
- Image quality and asset fidelity: the screen contains no raster imagery. Existing Lucide product icons remain vector-sharp and consistent with the application design system.
- Copy and content: plan name, ¥3.00 cloud-sourced preview price, 180,000 monthly points, model access, unlimited display values, purchase CTA, and footer entitlement copy are all present.

**Comparison History**

- Iteration 1 — P2: the details row did not stretch to the full card height, leaving the CTA too high and producing dead space. Fixed by stretching the plan-content grid; the revised capture aligns the divider and CTA with the bottom rhythm.
- Iteration 2 — P2: benefit hierarchy was too small and automation/project values were single-line. Fixed by stacking label/value pairs and increasing title, price, icon, value, and CTA optical sizes. The focused comparison shows the same hierarchy as the reference.

**Open Questions**

- None.

**Implementation Checklist**

- [x] Reuse the existing cloud price and purchase flow.
- [x] Reuse the existing history and payment-channel controls.
- [x] Preserve the existing model entitlement copy and footer facts.
- [x] Verify the final DOM, responsive grid behavior, and browser console.

**Follow-up Polish**

- P3: the implementation keeps the application's native typeface and slightly tighter density instead of reproducing the generated mock's heavier display type exactly.

Primary interactions tested: payment channel state and purchase-button availability were inspected in the browser; purchase/order behavior remains covered by the existing integration-style payment test. Browser console warnings/errors checked: none.

final result: passed

---

# 资料库搜索与二级栏交互 Design QA

## 证据

- 参考图：
  - `C:\Users\admin\AppData\Local\Temp\codex-clipboard-66979ecb-5ea3-435e-970e-27701e5bd03c.png`（搜索弹窗）
  - `C:\Users\admin\AppData\Local\Temp\codex-clipboard-ef050243-ce26-479d-817f-5f7bf069368e.png`（资料栏展开）
  - `C:\Users\admin\AppData\Local\Temp\codex-clipboard-7dd0197f-14c2-475c-bd63-eaf79fc7f885.png`（资料栏折叠）
- 实现截图：
  - `C:\Users\admin\.codex\visualizations\2026\08\31\01a056bf-d3dc-7ef0-8c11-6454ce581046\artifact-library-qa\search-modal-final.png`
  - `C:\Users\admin\.codex\visualizations\2026\08\31\01a056bf-d3dc-7ef0-8c11-6454ce581046\artifact-library-qa\expanded.png`
  - `C:\Users\admin\.codex\visualizations\2026\08\31\01a056bf-d3dc-7ef0-8c11-6454ce581046\artifact-library-qa\collapsed.png`
- 运行环境：真实 Wails 开发运行态，`http://127.0.0.1:5173/`。
- 比较尺寸：参考图均为 1658×1019、96 DPI；实现截图均为 1658×970、96 DPI。按相同 1658 px 内容宽度比较，参考图额外包含 Windows 标题栏和任务栏。
- 状态：有两条真实资料库数据；分别检查搜索弹窗、展开、折叠，以及搜索过滤和关闭后焦点恢复。

## 对照结果

1. 搜索已改为资料库内部弹窗，弹窗位置和 880×490 px 尺寸符合参考；资料库二级栏可独立折叠，主侧栏保持不变。
2. 首轮截图发现资料库遮罩偏浅：实现背景采样约 RGB(194,195,197)，参考约 RGB(102,102,102)。
3. 调整资料库局部遮罩到 `rgba(0, 0, 0, 0.6)` 后复测，背景采样为 RGB(102,102,102)，与参考一致；外层主侧栏仍未被遮罩。
4. 搜索输入可按文件名、来源对话和工作区过滤；关闭弹窗后焦点回到“搜索”按钮。
5. 折叠只移除资料库二级栏，内容区扩展；再次点击可恢复二级栏和原有“最近产物 / 本地产物”状态。
6. 资料库搜索底部键盘提示使用“选择 / 打开 / 关闭”，共享命令面板的默认文案保持不变。

未发现阻塞当前目标的布局、交互或可访问性问题。参考产品中的云端资料、分享、教学卡片等内容不属于本次资料库最小实现。

final result: passed
