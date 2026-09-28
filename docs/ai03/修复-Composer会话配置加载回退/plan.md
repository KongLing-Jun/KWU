# Composer 会话配置加载回退修复计划

## 背景与问题

`main-v3` 在 PR #351（核心提交 `499321cd`）中为模型与推理强度增加了
`pending / ready / degraded / updating-effort` 原子组合状态。该改动解决了模型与强度
半更新，但也把已有会话的首次呈现和会话交接统一映射为固定 `pending` 加载壳。

结果是点击另一个已有会话时，模型选择器会短暂清空或显示加载态，并隐藏推理强度；
应用冷启动、历史会话重开以及订阅信息读取较慢时更明显。这是既有会话快照行为的功能
回退，不是新的产品能力。

基线为最新 `origin/main-v3` 的 `7802ac60`。主工作区仅用于只读调查；计划与后续工作均
在独立工作树 `codex/fix-composer-session-config-loading` 中进行。

## 当前与预期行为

当前行为：

- `composerIdentityView` 已把模型、计费来源、专家、权限、协作方式等目标会话身份传给
  `TabMeta`、`SessionMeta` 和 `ProjectNode`，但遗漏了已经持久化的 `effort`。
- `coldStateForSurfaceTarget` 和 `stateFromViewSnapshot` 只恢复模型引用，将
  `composerSelection` 留在 `pending` 且不恢复 `effort`。
- `shouldHoldComposerSelectionPresentation` 把 `surfaceTransitionActive` 直接判为应显示
  固定加载壳；订阅权限或目录处于 `loading` 时也会立即返回 `pending`。
- 会话切换后才通过 `MetaForTab + EffortForTab` 异步发布完整组合，因此即使读取只跨越
  少数渲染帧，用户也会看到闪烁。

预期行为：

- 已打开标签切换、历史会话重开和应用重启恢复三种已有会话入口，都先用目标会话的
  既有持久化/缓存快照一次呈现模型与推理强度。
- 后端激活和权威回读完成前，值保持可见但选择器禁用；回读完成后解除禁用或沿用既有
  原子替换、降级与竞态处理。
- 订阅权限尚无结论时保留目标会话快照；一旦现有权限体系确认未登录、套餐不匹配或无权，
  仍按现有逻辑显示“请选择模型”。

## 目标

- 恢复已有会话配置的首帧可见性和会话隔离。
- 补齐现有 Composer 会话身份/视图快照中的推理强度，不创建第二套状态或持久化系统。
- 保留 PR #351 的模型与强度原子发布、旧请求拒绝和主动切换回滚能力。
- 以自动化测试和真实应用操作证明没有空白加载帧、跨会话串值或权限绕过。

## 非目标

- 不改变模型失效 fallback、通知、目录过滤或订阅鉴权规则。
- 不改变未登录或无权时合法出现的“请选择模型”。
- 不改变全新空白会话、用户主动切换模型/强度、发送门禁或专家权威回读语义。
- 不新增本地存储文件、缓存服务、后端运行时状态机或数据迁移任务。
- 不处理与 Composer 会话身份无关的通用加载体验。

## 用户场景与验收标准

### 已打开标签切换

- 点击目标标签后的首个目标会话 Composer 帧显示目标会话的模型与强度。
- 激活完成前选择器禁用但内容不清空；不得显示上一会话模型与目标会话强度或反向混配。
- 快速连续切换时，旧会话的异步结果不能覆盖最终目标会话。

### 历史会话重开

- 从历史列表或项目树打开已保存会话时，使用该会话 branch meta 中的模型、计费来源和
  effort 作为首帧身份。
- 专家/专家团、权限和协作方式继续使用现有 `ComposerSessionIdentity` 路径恢复。
- 权威回读结果一致时无额外视觉替换。

### 应用重启恢复

- 后端返回已保存标签/会话元数据后，Composer 同一次呈现模型与强度，不再额外等待
  `EffortForTab` 才补出强度。
- 真正缺少旧版快照字段的兼容会话可沿用现有权威回读，不伪造强度能力。

### 权限与目录状态

- BYOK 会话不受订阅权限读取影响，立即显示其持久化组合。
- subscription 会话在权限/目录仅为 `loading` 时显示持久化组合并禁用。
- 权限最终为 unavailable、未登录、套餐不匹配或目标路由不允许时，继续由现有
  `resolveComposerModelEntitlementPresentation` 进入 `choose`，不展示无权模型。
- 不支持推理强度的模型只显示模型；不得为了首帧显示而伪造可选强度等级。

## 证据与根因

- `desktop/session_composer_state.go` 从 `agent.BranchMeta` 读取模型、billing source 和
  `Effort`；`desktopTabEntry` 也已经持久化 `Effort *string`。
- `desktop/composer_identity_view.go` 的 `composerIdentityView` 及其 Session/Tab/Project
  映射没有 effort 字段，造成持久化值在前端交接合同中丢失。
- `desktop/frontend/src/features/session/sessionViewSnapshot.ts` 只保存基本 meta，不保存
  `State.effort` 或已确认的 `composerSelection`。
- `coldStateForSurfaceTarget` 与 `stateFromViewSnapshot` 明确把组合初始化为默认 pending。
- `shouldHoldComposerSelectionPresentation` 对 pending、目录切换和 surface transition 一律
  返回 true；`App.tsx` 随即把它映射为 `modelSelectionPresentation="pending"`。
- `ModelSwitcher.tsx` 在 pending 时清空模型标签并不传 effort，因此用户看到加载壳。
- 每次 active tab 改变都会重新并发调用 `ModelsForTab`、`Settings`、`ModelMarketplace`；
  选择器没有使用已有的两秒延迟门槛，而是立即显示 pending。

Codegraph 调查基于主工作区当时的 `f98f9878`，用于确认既有持久化和调用关系；该基线
落后于目标。所有与 PR #351 及最终方案相关的结论均已在独立工作树的 `7802ac60` 源码、
Git diff 和测试合同中重新确认，不把旧索引当作实现依据。

## 方案

1. 扩展现有 Composer 会话身份合同，使运行中标签、保存会话和项目树节点携带“是否有
   已知 effort 快照”及其持久化值；直接复用 `WorkspaceTab.effort` 和 branch meta，保持
   字段可选以兼容旧数据。
2. 扩展现有前端 `ComposerSessionIdentity` 和内存 `SessionViewSnapshot`，保存目标会话的
   已确认模型/强度组合；不增加新的 store 或磁盘格式。
3. 冷切换、缓存切换和历史重开时，从目标身份/快照初始化可见组合。快照只负责激活期间
   的展示；`EffortForTab` 仍是 supported、default、levels 等能力信息的权威来源。
4. 调整呈现门禁：surface transition 或订阅证据 `loading` 不再自动清空一个完整的目标
   快照，而是保留快照并设置 busy/disabled；没有可靠快照时继续使用现有 pending。
5. 保留 `refreshComposerSelectionForTab` 的并发 meta/effort 回读、单调序列和 ready/degraded
   发布。主动模型切换继续使用现有旧组合保留与失败恢复路径。
6. 保留 entitlement 的最终 `choose` 决策；本修复只区分“未知、正在读取”和“已确认无权”。

实现时优先复用现有类型和 reducer action；如果需要一个纯展示转换函数，应放在现有
Composer identity 或 model-switcher presentation 模块并通过单元测试覆盖，不引入平行状态机。

## 可能影响的合同与文件

后端/桥接合同：

- `desktop/composer_identity_view.go`
- `desktop/session_composer_state.go`（仅在需要保留 nil/known 语义时）
- `desktop/tabs.go`：`TabMeta`、`ProjectNode` 及映射
- `desktop/app.go`：`SessionMeta`

前端状态与呈现：

- `desktop/frontend/src/lib/types.ts`
- `desktop/frontend/src/lib/composerSessionIdentity.ts`
- `desktop/frontend/src/features/session/sessionViewSnapshot.ts`
- `desktop/frontend/src/features/session/useController.ts`
- `desktop/frontend/src/lib/modelSwitcherTransition.ts`
- `desktop/frontend/src/App.tsx`

测试文件按实际最小变更选择，预计覆盖 Composer identity、会话快照、标签切换水合、
原子选择、entitlement 和 model switcher presentation。不得为满足测试而扩大生产范围。

## 兼容性与迁移

- 新增桥接字段为可选字段，旧 session、旧 `desktop-tabs.json` 和缺少 effort 的 branch meta
  不需要迁移。
- 缺少可靠快照时继续异步权威读取；不根据模型名称猜测 supported 或 levels。
- 现有 JSON 字段、Wails 方法名和 `EffortInfo` 合同保持兼容；如最终只需扩展 DTO，不修改
  `EffortForTab` 的语义。
- 账户、profile、workspace 和 session identity key 继续作为隔离边界，快照不得跨边界复用。

## 风险与控制

- 风险：把旧会话 effort 配到新模型。控制：模型引用和 effort 作为同一目标身份/快照发布，
  并保留序列与 identity key 校验。
- 风险：短暂展示无权 subscription 模型。控制：仅在现有会话的持久化身份且权限仍为未知时
  显示并禁用；最终 denial 立即走现有 choose，发送门禁不放开。
- 风险：旧数据缺少能力信息。控制：不伪造支持等级，等待现有 `EffortForTab` 权威结果。
- 风险：破坏主动模型/强度切换。控制：保留现有 transition 分支并增加针对性回归测试。
- 风险：扩大桥接合同影响。控制：字段可选、只读，并运行受影响 Desktop Go 与 frontend build。

## 回滚

修复不迁移或重写用户数据。若出现回归，可回退本实现提交，恢复 PR #351 当前 pending
呈现；新增可选 JSON 字段可被旧前端安全忽略。

## 应用验收计划

在隔离 Wails 开发应用中准备至少两个具有不同模型/强度/权限/协作或专家身份的已有会话：

1. 在两个已打开标签间反复切换，逐帧观察选择器文本和禁用状态。
2. 关闭其中一个会话后从历史/项目树重新打开，确认首帧身份来自目标会话。
3. 重启应用并恢复标签，确认后端元数据到达后模型与强度一次显示。
4. 对 BYOK 会话验证目录读取不清空选择器。
5. 对 subscription 会话分别模拟 loading、allowed 和 definitive denial；loading 保留禁用快照，
   denial 仍显示“请选择模型”。
6. 快速连续切换，确认旧异步结果不覆盖最终会话。
7. 主动切换模型和强度，确认 PR #351 的原子替换、乐观强度和失败恢复未回退。

优先使用 Codex 内置浏览器控制 Wails 可访问页面；若原生 WebView 行为无法覆盖，再使用
Computer Use。实施者的聚焦测试不替代独立应用验收。

## 自动化验证计划

迭代期运行最接近的测试：

- 后端 Composer identity/session meta 相关 Go package tests。
- `session-view-snapshot`、`tab-switch-hydration`、`composer-selection-atomic`、
  `composer-model-entitlement`、`model-switcher-transition` 等受影响前端测试。
- 根据实际改动补充：三种已有会话入口的首帧组合、legacy 缺失字段、subscription
  loading/denial、快速切换 stale result。

实现稳定后运行：

- 直接改动的 Desktop Go package 范围测试。
- `desktop/frontend` 的 `pnpm build`。
- 最终 PR 获授权且分支同步最新 `origin/main-v3` 后，执行仓库规定的完整 PR Gate：
  `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all` 及 AGENTS.md 中列出的公共检查。

实现阶段仅允许在上述既有语义内选择最小技术形态；若发现必须改变权限、fallback、
外部 API、数据兼容或发送门禁，停止实现并重新进入审批。
