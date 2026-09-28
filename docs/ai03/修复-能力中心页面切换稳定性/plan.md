# 能力中心页面切换稳定性修复计划

## 状态

Ready。2026-08-18 已完成源码、现有测试和真实 Wails 基线复现核对；用户已确认目标布局与首次加载表现，没有待定产品或架构决策。

## 基线与故障证据

- 环境：Windows，`main-v3` 提交 `2b61a651a6fde1ebc2c06f884b09a3205a1a2433`，从 `D:\Reasonix\desktop` 使用现有 dev 配置与 profile 执行 `wails dev`；应用进程为 `D:\Reasonix\desktop\build\bin\kwu-desktop-dev.exe`。
- 窗口：Go 端最小外窗为 `1200x800`（`desktop/main.go` 的 `minimumWindowWidth` / `minimumWindowHeight`）；Computer Use 截得 WebView 内容为 `1186x793`。
- 前置条件：创作布局、最小窗口、窄侧栏已自动折叠；专家、技能、连接器均为本次进程首次进入。
- 复现步骤：展开侧栏，进入“专家 · 技能 · 连接器”；在专家页首次点击“技能”，待其完成；再首次点击“连接器”。
- 实际行为：首次点击技能后先出现只有目标导航选中态和“加载中”的中间画面，随后完整页面及顶部操作区出现，导航/内容发生可见位移；首次点击连接器后先出现连接器选中态与上一技能内容，下一次观察才替换为连接器目录。连接器最终页面的壳层实现也与专家/技能不同。
- 期望行为：目标导航选中后立即显示该目标页的稳定占位；上一页内容不再残留；加载前后导航位置、顶栏高度和内容起点不变；三个页面使用一致的壳层、顶栏和内边距。页面业务内容、操作能力和数据加载行为保持不变。

现有自动化没有证明上述行为：`desktop/frontend/src/__tests__/capability-workspace-stability.test.ts` 只检查源码字符串，且只禁止技能和专家依赖懒加载子树；`capabilities-panel-actions.test.ts` 单独挂载页面验证最终 Portal 位置，不经过 App 级路由、Suspense 或最小窗口布局。

## 根因与实现依据

1. `desktop/frontend/src/App.tsx` 以 `workspaceView` 同步选择能力外壳，但三个 lazy 页面各有自己的 Suspense 分支和通用空状态；当前内容边界没有按目标路由显式重置，也没有可识别的目标页占位。因此目标导航、旧内容、lazy fallback 和目标内容可能形成多个可见阶段。
2. `desktop/frontend/src/styles/theme-overrides.css` 中专家和技能已通过同步存在的 `capability-workspace-shell--experts` / `--skills` 确定壳层尺寸，连接器仍通过 `:has(.primary-workspace--connectors)` 选择壳、顶栏、导航和操作区。连接器样式生效依赖子树，与另两页不一致。提交 `5a404fab` 曾以同一原因将专家从 `:has(...)` 改为外层类，但现有回归未覆盖连接器。
3. 专家、技能、连接器的顶部操作分别由 `ExpertDirectory.tsx` 和 `CapabilitiesPanel.tsx` 在子组件挂载后 Portal 到 `#capability-workspace-header-actions`。技能还会在业务快照到达前提前返回内部加载态。若顶栏或内容流随操作区是否存在而改变，首次加载必然有重排机会。
4. 最小外窗的 WebView 宽度是 `1186px`，同时低于创作布局的 `1200px` 窄侧栏阈值。能力导航会关闭窄侧栏，这会改变横向可用空间并放大头部换行或子树选择器切换造成的视觉变化，但它不是纵向位移的独立根因。

## 目标与非目标

### 目标

- 专家、技能、连接器共享同一套 creation 能力壳几何；连接器不再依赖 lazy 子树选择器。
- 在最小窗口 `1200x800`（WebView `1186x793`）下，三页导航顶栏固定，操作区使用预留的内容工具带，不得在 Portal 到达时撑高导航。
- 目标页面 lazy 模块或业务数据未就绪时，显示带目标身份的稳定占位；目标切换提交后不展示上一页内容。
- 保留三页现有搜索、管理、召唤、连接、尝试、项目返回及数据刷新行为。
- 用聚焦自动回归锁定外层类、连接器禁用 `:has(...)`、路由重置边界和稳定占位，再用真实 Wails A/B 验收用户可见行为。

### 非目标

- 不改变专家、技能、连接器的后端接口、数据结构、缓存或授权流程。
- 不取消 lazy loading，不把三个大型页面常驻 DOM，不扩大初始 bundle。
- 不重做卡片、目录内容、筛选逻辑、侧栏策略或设置中心页面。
- 不修改 Wails 最小窗口尺寸，也不引入新的响应式框架或通用布局抽象。

## 验收标准

AC1. 在全新进程、最小窗口首次进入能力中心并按“专家 -> 技能 -> 连接器 -> 专家”切换时，选中态立即对应目标页；加载期间只出现目标页稳定占位，不出现上一页内容。

AC2. 上述每次切换在占位前后，能力导航的左上位置和 `52px` 顶栏高度保持一致；最小窗口操作区在预留工具带内出现，不能改变导航或内容起点。

AC3. 专家、技能、连接器使用相同的外层 route class 驱动壳背景、顶栏、导航按钮和响应式工具带；CSS 中不得再以连接器内容子树 `:has(.primary-workspace--connectors)` 决定这些属性。

AC4. 三页加载完成后原有搜索和主操作仍可见可用；专家召唤/管理、技能查看/安装管理、连接器连接/自定义/尝试及项目返回合同不变。设置中心以 `workspace=false` 使用技能和连接器页面的行为不变。

AC5. 聚焦回归、`pnpm build`、`git diff --check origin/main-v3...HEAD` 和仓库 PR gate 全部通过；仅前端文件变更，不运行 Go 测试。

## 方案

1. 在 `App.tsx` 保留现有 lazy import 和能力页面回调，只把能力内容区域改为按 `workspaceView` keyed 的切换边界，并为每个目标页使用可识别、尺寸稳定的 loading presentation。外壳和 header 不随 key 重建，使导航在目标页解析期间保持稳定。
2. 在 `theme-overrides.css` 将专家、技能、连接器共同的 creation 壳、`52px` header、导航按钮和 active/hover 状态合并到由三个同步外层 route class 驱动的规则。将所有连接器壳层 `:has(.primary-workspace--connectors)` 改为 `capability-workspace-shell--connectors` 或公共规则。
3. 为 WebView 不大于 `1200px` 的最小窗口模式预留固定操作工具带：导航 header 仍为 `52px`，Portal host 脱离 header 的高度计算并占用固定内容带，目标页面加载前后都保留相同空间。更窄的浏览器/测试视口可使用更高的预留带容纳技能操作换行，但仍不改变导航 header。
4. 不改三个子页面的业务状态与 Portal 合同；只在确有必要时调整它们的 workspace 顶栏 fallback，确保 workspace 模式不会把同一操作区先画在内容流、再移动到 Portal。设置中心的非 workspace 分支保持原样。
5. 扩展 `capability-workspace-stability.test.ts`，至少锁定 connector outer-class 规则、禁止连接器壳 `:has(...)`、keyed 内容边界、目标 loading 标识和最小窗口固定几何。复用 `capabilities-panel-actions.test.ts` 验证完成态 Portal；不为本次行为搭建新的通用组件或测试框架。

## 影响文件与合同

- `desktop/frontend/src/App.tsx`：能力中心唯一 App 级切换入口；保持 `CapabilityWorkspaceView`、导航回调和三页 props 不变。
- `desktop/frontend/src/styles/theme-overrides.css`：creation 能力壳和三页特定样式；统一公共几何，保留页面内容视觉规则。
- `desktop/frontend/src/styles/features.css`：仅当公共基础规则必须配合固定工具带或 loading presentation 时修改；避免无关 CSS 整理。
- `desktop/frontend/src/__tests__/capability-workspace-stability.test.ts`：聚焦壳层/切换稳定性回归。
- `desktop/frontend/src/__tests__/capabilities-panel-actions.test.ts`：仅在 Portal workspace/fallback 合同需要行为断言时修改。
- `desktop/frontend/src/components/CapabilitiesPanel.tsx`、`desktop/frontend/src/features/experts/ui/ExpertDirectory.tsx`：默认不改；若实现证明其首次 workspace render 会把顶栏先插入内容流，则只收紧 workspace 顶栏宿主行为，不触碰数据或操作逻辑。

设置中心是 `CapabilitiesPanel` 的现有消费者，继续使用 `workspace=false`。项目来源的“返回项目”按钮与能力 header 共存，不能被最小窗口工具带覆盖。没有公共 API、持久化数据、迁移、依赖、权限或兼容格式变更。

## 方案挑战与取舍

- 仅把连接器 `:has(...)` 改为外层类：能修复已知不一致，但不能约束旧内容、通用 fallback 和 Portal 到达的多阶段呈现，不满足 AC1/AC2。
- eager import 或进入能力中心即加载全部页面：可缩短首次 fallback，但扩大初始 bundle，仍不能单独保证业务数据/Portal 到达时不重排，超出必要范围。
- 三页常驻并用 CSS 隐藏：会保留各页请求、定时刷新和本地状态，增加后台副作用及内存，占用不受验收支持。
- 只增加过渡动画：会掩盖而非消除错误几何，且 reduced-motion 下仍失败。
- 抽取新的通用页面框架：当前唯一消费者就是 `App.tsx` 的一处能力分支；新增抽象不能减少足够复杂度，暂不采用。

选定方案只改变 App 切换边界和现有 CSS 所有权，既保留按需加载，又让 loading/Portal 阶段共享固定几何。

## 风险、回滚与偏差门槛

- 风险：固定工具带可能裁剪技能多按钮、连接器搜索或项目返回按钮。验证 `1186x793`、默认窗口和更窄浏览器视口的可见性与不重叠；只为真实溢出调整带高和换行。
- 风险：keyed 内容会在能力页之间明确卸载目标页面，这是当前互斥分支本就具有的语义；不得额外重置同一页面内部筛选或对话选择。
- 风险：CSS import 顺序为 `theme-overrides.css` 后接 `features.css`，需以真实最终级联和应用截图验证，不能只依赖字符串包含测试。
- 回滚：代码与样式为一个独立 `fix:` 提交，回滚即可恢复；无数据恢复步骤。
- 偏差门槛：若必须改变页面数据加载、公开 props、设置中心行为、Wails 窗口约束或 lazy bundle 策略，停止实现并回到计划确认。

## 验证

1. 基线自动证据：修复前运行现有 `capability-workspace-stability.test.ts`，记录其通过但未覆盖真实故障；新增断言应在旧基线失败、实现后通过。
2. 聚焦前端：从 `desktop/frontend` 运行新增/修改的精确 tsx 测试，并运行受影响的 `capabilities-panel-actions.test.ts`（若触碰 Portal 子页面）。
3. 构建：从 `desktop/frontend` 运行 `pnpm build`。前端变更不运行 Go 测试或 `pnpm test:all`。
4. 真实 A/B：A 使用未改 `D:\Reasonix\desktop` 的相同 dev profile、最小窗口和步骤，保留上述失败证据后正常停止并确认进程/端口/单例锁释放；B 从修复工作树 `desktop/` 以相同 dev 配置和 profile 串行启动 `wails dev`，恢复相同最小窗口与页面起点，首次执行“专家 -> 技能 -> 连接器 -> 专家”。记录 loading/完成态截图或 WebView DOM 矩形，验证 AC1-AC4。B 通过后保留修复版 dev 应用运行供用户验收。
5. 仓库门禁：按根 `AGENTS.md` 运行 Code Guard、baseline 文件保护、diff check 与 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`，检查分类为 frontend，并记录所有跳过项及原因。
