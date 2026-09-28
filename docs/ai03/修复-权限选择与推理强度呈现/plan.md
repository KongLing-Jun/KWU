# 权限选择与推理强度呈现修复计划

## 背景与问题

Composer 底部的模型选择器和权限选择器相邻，但当前视觉与状态反馈不一致：

- 模型选择器在创建页使用浅色细边框，弹层使用统一的主题浮层表面；权限入口和权限弹层没有加入同一组主题覆盖，边框、圆角与阴影观感不同。
- 权限菜单的三项分别使用 `Shield`、`ShieldCheck`、`ShieldAlert`，入口却始终写死为 `Shield`。切换后只有文字变化，图标不会对应当前权限。
- “完全访问”菜单项已有危险色，入口没有对应警示色；活动项还会被通用选中样式覆盖，状态层级不完整。
- 模型身份随会话元数据先呈现，推理强度仍被归为第二阶段附属数据，可能等待 Controller 就绪及 context、balance、jobs、checkpoints 等同批查询。冷状态和 `optimistic_meta` 也只携带模型，不携带强度；`ModelSwitcher` 的 `selectionPending` 只禁用按钮，不隐藏模型文案。因此单纯提前查询 effort 仍会出现“模型先显示，推理强度稍后补出”的布局变化。
- 直接切换模型和设置强度的路径没有复用 `refreshMetaForTab`，而是各自按 meta → context → effort 串行刷新。即使修正会话水合，用户点击模型后仍可能先看到新模型搭配旧或缺失的强度。

预期行为是：权限控件与模型控件属于同一套选择器视觉；权限入口的图标、文字和颜色始终反映当前状态；模型和其推理强度作为同一选择身份在快速水合阶段呈现，不再等待无关附属数据。

## 已确认的产品决策

- 仅修正视觉、状态映射与呈现时序，不改变三种权限的含义、默认值、顺序或后端行为。
- 权限入口和权限弹层复用模型选择器的浅色边框、主题表面、圆角与浮层阴影语言。
- ask、auto、yolo 分别显示 `Shield`、`ShieldCheck`、`ShieldAlert` 及现有对应文案。
- 选中“完全访问”时，入口和菜单项的图标、文字使用警示红色；按钮边框与背景保持中性浅色。菜单活动项可使用轻微危险色底，不渲染重色警示块。
- ask、auto 的活动项沿用普通强调色；点击后图标、文字、颜色和无障碍状态同步更新。
- 模型与推理强度采用显式的组合就绪状态和单次 reducer 更新；支持推理强度的模型应与当前强度一起出现，不显示占位文案，也不借用上一会话或上一模型的强度。
- 推理强度读取失败时保持模型选择器可用，不让控件永久停留在加载态。

## 目标与非目标

### 目标

- 统一相邻模型与权限选择器的入口及弹层视觉。
- 建立权限值到入口图标、文案和状态色的单一映射，避免触发器和菜单分叉。
- 消除支持推理强度的模型在首次加载或会话切换时先显示模型、后补强度的可见跳变。
- 保持窄窗口、键盘操作、焦点回归和辅助技术语义稳定。

### 非目标

- 不修改权限策略、审批流程、快捷键、配置默认值或 Wails 权限桥接。
- 不修改模型路由、订阅鉴权、推理强度能力判断、可选等级或默认强度。
- 不重做通用 `AnchoredPopover` 或其它 Composer 控件。
- 不把权限警示扩展为确认弹窗或阻断流程。

## 用户场景与验收标准

1. 当前权限为“需要批准”时，入口显示普通盾牌和对应文案；打开面板后该项使用普通活动样式。
2. 选择“自动批准”后，入口立即显示带勾盾牌和“自动批准”，菜单的 `aria-checked` 与活动样式同步。
3. 选择“完全访问”后，入口立即显示警示盾牌和“完全访问”，图标与文字为警示红色，但浅色边框和中性背景不变；弹层活动项使用警示红和轻微危险色底。
4. 权限入口的默认、悬浮、焦点、打开和禁用状态与模型入口使用同一浅色边框语言；权限弹层与模型弹层使用同一主题边框、圆角、表面和阴影。
5. 权限弹层标题与外层权限入口使用相同的常规字重；活动状态和完全访问仍只通过既定状态色表达，不通过加粗制造额外层级。
6. 三项选择仍关闭菜单并把焦点交回输入区域；上下方向键、Home、End 和 `aria-checked` 行为保持。
7. 首次进入会话、打开历史会话、切换会话和直接切换模型时，支持推理强度的当前模型以“模型 · 强度”一次稳定呈现；模型与强度文字使用相同前景色，强度不再等待 context、balance、jobs 或 checkpoints。
8. 首次/会话加载时，新的模型身份尚未与 effort 对齐则使用现有加载呈现；用户显式切换模型时保留并短暂禁用上一组已确认的“模型 · 强度”，新组合完整就绪后一次替换。两类路径都不得发布新模型配旧强度，显式切换不得清空正常可见的旧组合。
9. 不支持推理强度的模型仍只显示模型；强度查询失败时以降级就绪状态显示模型并保持可用，不出现旧会话强度或永久加载。
10. 用户手动切换同一模型的推理强度时，模型文案始终可见，所选强度立即乐观呈现；后台确认期间只短暂禁用控件，失败则恢复旧强度并给出提示，不得把整个模型选择框切成空白 pending。
11. 现有窄窗口断点不溢出，模型和权限控件仍可点击和辨识。

## Codegraph 与源代码依据

调查基线：`origin/main-v3` 的 `f98f9878`。Codegraph 索引与主工作区源代码共同确认：

- `desktop/frontend/src/features/composer/ui/Composer.tsx` 的权限菜单按 ask/auto/yolo 映射 `Shield`、`ShieldCheck`、`ShieldAlert`，但入口 JSX 固定渲染 `Shield`；文案由 `approvalModeLabel` 动态计算。
- 同文件复用 `AnchoredPopover`、`chooseCompactApprovalMode`、`menuitemradio` 和焦点回归，说明本次不需要改变权限回调或交互架构。
- `desktop/frontend/src/styles/session.css` 中模型与权限入口各自声明边框；权限弹层基于 `.composer-access-menu`，模型弹层基于 `.modelsw__menu`，两组圆角与阴影不同。
- `desktop/frontend/src/styles/theme-overrides.css` 的创建页浅色表面选择器包含 `.modelsw__trigger`，不包含 `.composer-approval-trigger`。
- `desktop/frontend/src/styles/base.css` 的主题浮层覆盖包含 `.modelsw__menu`，不包含 `.composer-approval-menu`；模型活动项使用 accent/soft-accent，权限活动项仍使用通用前景色。
- `desktop/frontend/src/components/ModelSwitcher.tsx` 仅在 `effort.supported`、levels 非空且存在回调时渲染强度。
- `desktop/frontend/src/features/session/useController.ts` 的 `loadSessionDataForTab` 第一阶段读取 meta/history，第二阶段才在 Controller readiness 之后与 jobs/checkpoints/context/balance 一起读取 effort；`refreshMetaForTab` 还按 meta、context、effort 顺序串行读取。
- 同文件的 `coldStateForSurfaceTarget`、`stateFromViewSnapshot` 和 `switchTab` 乐观路径可以先提供 `modelRef` 而没有 effort；`ModelSwitcher` 只有 `selectionPresentation="pending"` 才隐藏选择文案，`selectionPending` 不会阻止模型单独显示。
- 同文件的 `setModel` 与 `setEffort` 直接顺序调用 `MetaForTab`、`ContextUsageForTab`、`EffortForTab`，不会经过 `refreshMetaForTab`；这是直接切换后强度晚出现并可能短暂复用旧值的独立路径。
- `desktop/frontend/src/features/session/sessionReducer.ts` 的 reset/hydrate_reset 会保留现有 effort；请求序列门禁只能拦截晚到响应，不能自动清除已存在但属于旧模型身份的 effort。
- `desktop/app.go` 的 `EffortForTab` 根据标签页模型和本地配置计算 `EffortInfo`，不依赖已启动的 Controller；因此可以安全地与模型元数据在快速阶段并发读取。

## 根因与实施方案

### 权限呈现

1. 在 `Composer.tsx` 建立当前权限的统一呈现对象，包含图标、文案和模式 class；触发器与菜单共用同一映射，避免入口再次写死图标。
2. 给触发器增加模式状态 class。ask/auto 使用普通状态色，yolo 只将图标和文字提升为危险色；悬浮、焦点和打开态不能覆盖 yolo 的语义色。
3. 让 `.composer-approval-trigger` 加入模型入口使用的创建页浅色表面覆盖，让 `.composer-approval-menu` 加入模型弹层的主题表面覆盖；在权限自身样式中对齐必要的边框、圆角、背景和阴影 token，而不耦合组件 DOM。
4. 为 ask/auto 活动项使用普通 accent 状态，为 yolo 活动项使用 danger 状态和轻微 danger-soft 混色；保留 `aria-checked`。
5. 权限弹层标题使用与外层入口一致的常规字重；保留活动项颜色和 yolo 危险色，不额外加粗。

### 模型与推理强度时序

1. 在会话状态中增加显式的 Composer 选择身份阶段（pending、ready、degraded、updating-effort）和每标签页选择序列，并增加同时携带 meta 与 effort 的组合 action，以及只更新 effort 的乐观 action。只有匹配当前序列的 action 可以发布；不依赖两个 dispatch 恰好被 React 批处理。
2. 冷状态、无 effort 的缓存快照，以及任意 `meta`/`optimistic_meta` 引入不同 `modelRef` 时在状态层将组合标记为 pending。首次/会话加载由 App 向 `ModelSwitcher` 传递现有的空白 pending；显式模型切换则保存旧模型、billing source 和 effort，在状态 pending 期间继续呈现该旧快照，直到新目录身份和组合状态同时就绪。
3. 建立唯一的 `refreshComposerSelectionForTab` 辅助路径，并发读取 `MetaForTab` 与 `EffortForTab`，校验请求仍属于目标标签页/身份后，通过组合 action 一次提交。meta 成功而 effort 失败时提交 degraded：清除旧 effort、显示模型并保持选择器可用。
4. `loadSessionDataForTab` 在第一阶段启动组合刷新，并从第二阶段移除 effort；history 与选择身份并发，context、balance、jobs、checkpoints 继续独立加载，不阻塞组合身份。
5. `refreshMetaForTab`、直接 `setModel`、启动重试及其它需要重新确认模型身份的路径复用该辅助函数。模型切换开始时状态进入 pending，但 UI 保留旧确认组合并短暂禁用；成功时一次替换，失败恢复旧组合。`setEffort` 不改变模型身份，改用 updating-effort 乐观更新当前强度并保持模型文案；后台确认后回到 ready，失败恢复旧组合并提示。
6. 保留现有会话加载序列和 `stillCurrent` 门禁，并在组合提交前同时核对标签页 ID、选择序列与返回的 `modelRef`；晚到的旧会话或旧模型结果不得覆盖当前选择。
7. 不改变 Go 结构、Wails 方法签名或 `EffortInfo` 合同。
8. 模型选择器中的 effort 文案继承组合文字颜色，显式强度不再单独使用 accent 色。

## 预计影响范围

- `desktop/frontend/src/features/composer/ui/Composer.tsx`
- `desktop/frontend/src/App.tsx`
- `desktop/frontend/src/styles/session.css`
- `desktop/frontend/src/styles/theme-overrides.css`
- `desktop/frontend/src/styles/base.css`（若主题浮层覆盖的权威规则仍位于此处）
- `desktop/frontend/src/features/session/useController.ts`
- `desktop/frontend/src/features/session/sessionState.ts`
- `desktop/frontend/src/features/session/sessionReducer.ts`
- `desktop/frontend/src/features/session/sessionViewSnapshot.ts`（若缓存快照需要保存已对齐的组合状态）
- `desktop/frontend/src/__tests__/composer-responsive-toolbar.test.tsx`
- `desktop/frontend/src/__tests__/tab-switch-hydration.test.tsx` 或最近的会话水合测试
- 必要时补充聚焦的 ModelSwitcher 呈现测试；不修改后端文件。

## 兼容性、风险与回滚

- 权限值、回调、桥接合同和持久化均不变，无数据迁移。
- 主要视觉风险是主题覆盖优先级导致 yolo 危险色或浅色边框被 hover/active 规则覆盖；通过三种模式和主题样式断言及应用验收控制。
- 主要时序风险是组合门禁造成永久加载、提前读取 effort 阻塞历史，或旧请求污染新标签页/新模型；通过显式 degraded 状态、并发加载、身份校验和延迟 promise 测试控制。
- 手动强度更新若复用模型 pending 会造成整框消失；通过独立 updating-effort 阶段、可见乐观值、后台期间禁用和失败回滚控制。
- 显式模型切换若把内部 pending 直接映射为空白壳会破坏原有可见性；通过保存旧确认组合，并等待新目录身份与新 effort 同时就绪后一次替换来控制。
- 若 effort 查询失败，组合 action 清除旧强度并以 degraded 状态发布当前模型；不会继续等待或复用旧值。
- 回滚前端提交即可恢复现状，不涉及配置或会话数据回滚。

## 应用测试计划

- 在真实 Wails 窗口检查默认主题及当前主题样式下，权限入口与模型入口的边框、背景、圆角和弹层阴影一致。
- 依次选择需要批准、自动批准、完全访问，观察入口图标、文案、颜色、弹层活动项、关闭和焦点回归。
- 首次打开一个支持推理强度的会话，确认模型与强度一次出现；在支持/不支持强度的会话间切换。直接切换模型时确认旧组合持续可见且短暂禁用，新组合就绪后一次替换，无空白或半更新状态闪现。
- 在同一模型下依次切换多个强度，确认模型始终可见、新强度立即出现、后台确认期间没有空白帧；模拟失败时恢复旧强度并出现提示。
- 在最窄受支持窗口复核工具栏不溢出，两个选择器仍可操作。

## 自动化测试计划

- 扩展 `composer-responsive-toolbar.test.tsx`：覆盖三种模式的触发器图标/class/文案、yolo 颜色契约、面板主题样式及原有选择/键盘行为。
- 扩展 reducer/ModelSwitcher 测试：证明新 `modelRef` 会使组合进入 pending、清除旧 effort，且只有组合 action 或 degraded 结果会结束加载并发布文案。
- 扩展会话水合测试：用可控 promise 延迟 context、balance、jobs、checkpoints，证明组合身份不等待附属数据；切换后旧 effort 不得写入新会话。
- 覆盖直接模型切换：内部 pending 期间保留旧确认模型与 effort；延迟 context 时新模型与 effort 仍一次提交；支持与不支持强度的模型互切会清除旧值；effort 失败进入 degraded 而非永久 pending。
- 覆盖直接强度切换：updating-effort 立即保留模型并显示新强度，成功回到 ready，失败恢复旧强度和 ready 状态。
- 在 `desktop/frontend` 运行聚焦测试与 `pnpm build`。
- 实施 PR 前按仓库规则运行 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all` 及通用检查；本计划不改 Go 文件，因此不额外运行 Go 测试。
