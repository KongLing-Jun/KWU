# 统一模型目录状态计划

## 背景与问题

会话页底部模型选择框与模型广场的“当前可切换模型”表达的是同一件事：当前标签页、当前账户范围和当前订阅权限下可切换的模型集合。但两处目前各自调用 `ModelsForTab` 和 `ModelMarketplace`、各自维护 loading 与结果。

模型广场通过 `workspaceView === "modelMarketplace"` 条件挂载。离开页面会卸载组件，再次进入时 `ModelMarketplace` 的挂载 effect 必定重新调用 `ModelsForTab`，于是即使会话页选择框已经取得相同目录，模型广场仍显示“正在读取模型配置…”。同一时刻 `App` 的订阅默认修复事实也会读取相同数据，形成重复读取和短暂状态不一致。

## 当前行为

- 会话页 `ModelSwitcher` 依据标签页、账户 scope 和刷新 revision 独立读取模型路由与价格目录。
- `ModelMarketplace` 每次挂载都独立读取可切换模型；页面切换导致重复 loading。
- `App` 的 `subscriptionRepairFacts` 已按标签页、账户、当前模型和 revision 读取 `ModelsForTab`、`Settings` 与 `ModelMarketplace`。
- 后端 `ModelsForTab` 不访问模型价格远端接口，但会加载并合并桌面、项目和凭据相关配置，再应用订阅授权模型。
- 模型广场下方价格目录在进入页面时通过 `ModelMarketplace` 只读磁盘缓存；只有 `RefreshModelMarketplace` 可以访问远端价格接口。

## 期望行为

- 会话页模型选择框和模型广场“当前可切换模型”消费同一份 App 级模型目录状态。
- 普通进入、离开模型广场不触发模型目录读取，也不产生新的 loading。
- 标签页、账户/订阅身份、模型配置、当前模型或显式刷新发生变化时，App 统一重新读取目录。
- 真实刷新期间不保留旧列表；两处一起 loading，防止展示旧标签页或旧权限的模型。
- 读取失败时两处显示明确的不可用状态，不伪装成“暂无可选模型”。
- 模型广场现有“刷新”按钮在刷新价格目录后触发共享模型目录重读，不增加新按钮。
- 下方价格目录的缓存与联网语义保持不变。

## 目标

1. 消除两个 UI 入口对 `ModelsForTab` 和用于权限过滤的 `ModelMarketplace` 的重复调用。
2. 保证两处模型集合、当前项、权限过滤、loading 和失败状态一致。
3. 保留现有事件驱动的失效机制与模型选择行为。
4. 用聚焦组件测试和静态契约覆盖页面重挂载、共享更新、错误状态和刷新连线。

## 非目标

- 不修改 Go 后端、模型授权规则、配置加载或模型切换接口。
- 不新增缓存、持久化文件、TTL、后台轮询或请求去重服务。
- 不修改模型广场价格卡、筛选、排序、链接和远端价格协议。
- 不改变选择模型后返回会话页的交互。
- 不在本次统一其他模型目录消费者，例如自动化面板。

## 用户场景与验收标准

### 页面切换

1. 当前标签页的共享模型目录已经 ready。
2. 用户从会话页进入模型广场、返回会话页、再次进入模型广场。
3. 两处始终展示相同模型集合和当前模型。
4. 页面切换本身不新增 `ModelsForTab` 或用于过滤的 `ModelMarketplace` 调用，不重新显示模型配置 loading。

### 有效失效事件

1. 标签页、账户 scope、配置目录、订阅身份或当前模型发生变化，或用户点击模型广场“刷新”。
2. App 将共享目录置为 loading 并清空旧结果。
3. 新读取完成后，两处同时展示新目录；不得混入旧 scope 的结果。

### 失败

1. `ModelsForTab` 读取失败。
2. 会话页选择框不可操作并表达目录不可用。
3. 模型广场红框表达“模型配置暂不可用”，而不是“当前套餐暂无可切换模型”。
4. 后续有效失效事件或模型广场刷新可以重新读取。

### 价格目录

1. 进入模型广场仍只读取本地价格缓存。
2. 点击现有刷新按钮仍调用远端价格刷新，并同时使共享模型目录失效重读。

## 证据与实现依据

基线：`origin/main-v3` / `06307e38bc8775cc4c28188d31a8bf5682d243db`。当前环境没有可调用的 Codegraph，因此结论由基线源码直接核对：

- `desktop/frontend/src/features/model-marketplace/ui/ModelMarketplace.tsx:69-102`：价格缓存与可切换模型使用两个挂载 effect；后者直接调用 `ModelsForTab/Models`。
- `desktop/frontend/src/components/ModelSwitcher.tsx:39-85`：选择框维护独立 catalog state，并再次读取 routes 与 marketplace。
- `desktop/frontend/src/App.tsx:1709-1750`：`subscriptionRepairFacts` 已读取同样的 routes 与 marketplace，并有 scope/revision/stale-request 保护。
- `desktop/frontend/src/App.tsx:4906-4922`：模型广场是条件挂载，切走即卸载。
- `desktop/model_marketplace_app.go:194-224`：普通 `ModelMarketplace` 是 cache-only；远端访问只在 `RefreshModelMarketplace`。
- `desktop/app.go:7052-7118`：`ModelsForTab` 从配置和授权状态构造目录，不是轻量常量读取。

## 方案

1. 定义窄的共享模型目录 UI 状态契约，区分 `loading`、`ready` 和 `unavailable`；ready 包含 routes 及用于订阅过滤的 marketplace snapshot。
2. 在 `App` 中从当前 `subscriptionRepairFacts` 投影该契约。保留现有 request sequence、key 和 revision 作为唯一读取与失效路径。
3. 将共享状态经 `Composer` 传给 `ModelSwitcher`；移除切换器内部 catalog effect、request key 和 bridge 调用。
4. 将同一状态传给 `ModelMarketplace`；移除可切换模型的挂载 effect、局部候选列表与 refresh signal。选择成功后不在子组件伪造目录快照，由 App 的既有模型变更事件重读权威状态。
5. 两处继续调用同一 `filterModelInfosForSubscriptionAccess`，输入完全相同，确保集合一致。
6. 价格目录继续由模型广场局部读取和展示。现有刷新成功后的 `onMarketplaceChanged` 继续触发 App 的 `refreshModelCatalog`，从而同时重读共享目录。

## 影响范围

预计修改：

- `desktop/frontend/src/lib/types.ts`：共享状态类型。
- `desktop/frontend/src/App.tsx`：状态投影与 props 连线。
- `desktop/frontend/src/features/composer/ui/Composer.tsx`：透传共享状态。
- `desktop/frontend/src/components/ModelSwitcher.tsx`：改为纯消费共享状态。
- `desktop/frontend/src/features/model-marketplace/ui/ModelMarketplace.tsx`：改为纯消费共享可切换目录。
- 相关前端组件与静态契约测试。
- 本目录的 `progress.md` 与任务收据。

禁止修改 Go 后端、价格缓存结构、授权算法和不相关页面。

## 兼容性与迁移

这是桌面前端内部 props 契约调整，没有外部 API、配置或数据迁移。Wails binding 保持不变。组件测试必须随必填共享状态 prop 更新；不保留独立请求 fallback，以防重新引入双源状态。

## 风险与控制

- **失效事件漏接：** 复用现有 `subscriptionRepairFactsRevision`、账户 key、标签页 key、当前模型 key及设置 fingerprint，不新建第二套 revision。
- **陈旧请求覆盖：** 保留 `subscriptionRepairRequestRef` 的序列保护。
- **错误与空列表混淆：** routes 请求失败映射为 `unavailable`；成功返回空数组映射为 `ready` 空列表。
- **模型切换期间闪烁：** 按已确认语义进入 loading，不保留或局部伪造旧列表。
- **价格刷新语义被误改：** 只移除可切换目录请求，不修改价格 `loadMarketplace/refreshMarketplace` 路径。

## 回滚

按意图分提交。若共享状态连线出现回归，可回退实现提交，恢复两个子组件的独立 effect；文档提交可独立保留为问题记录。没有数据迁移或不可逆副作用。

## 验证计划

### 聚焦自动化测试

- 更新并运行 `model-switcher` 相关测试，覆盖 ready/loading/unavailable 与权限过滤。
- 更新并运行 `model-marketplace.test.tsx`，覆盖共享目录展示、loading、unavailable、选择与价格刷新回调。
- 更新现有 browser/client sync 静态契约，确认子组件不再调用 `ModelsForTab`，App 仍持有唯一目录读取与刷新连线。

### 应用验证

- 启动桌面前端可访问环境后，验证会话页模型选择框与模型广场模型集合、当前模型一致。
- 往返模型广场至少两次，观察不出现新的“正在读取模型配置…”。
- 点击模型广场刷新，观察共享模型目录进入 loading 后两处恢复一致。
- 若当前环境无法可靠启动 Wails 桌面运行时，记录自动化替代证据并将真实桌面验收标为待用户验证。

### 最终检查

- 前端变更运行 `pnpm build`。
- 运行受影响的聚焦测试。
- 检查 `git diff --check`、EOL、实际 diff 和任务收据。
- 不创建 PR，因此不运行或发布 PR Gate；若后续获准创建 PR，再同步最新 `main-v3` 并执行完整 gate。
