# 模型广场无缓存自动获取修复计划

## 背景与问题

模型广场的价格目录采用本地缓存优先。当前页面挂载后调用
`ModelMarketplace`，该绑定严格只读磁盘缓存；只有用户点击刷新才会调用
`RefreshModelMarketplace` 访问 AirClaude `/api/pricing`。因此首次安装、缓存被清理、
缓存文件损坏或身份校验失败时，页面直接显示“需要联网获取模型数据”，不会自动完成
首次数据引导。

这不是偶发漏调，而是现有显式刷新契约。当前体验的问题是：用户已经主动进入需要远端
数据的模型广场，仍必须再点击一次才能看到目录。

## 当前行为与期望行为

当前行为：

1. 进入模型广场，前端执行一次 cache-only `ModelMarketplace`。
2. 无可用缓存时，后端返回 `pricingState=unavailable` 且模型列表为空。
3. 前端显示恢复提示，等待用户明确点击刷新。
4. 有可用缓存时直接展示，不访问网络。

期望行为：

1. 保留首次 cache-only 读取。
2. 读取结果明确表示没有任何可用价格缓存时，在本次应用生命周期内自动调用一次现有
   `RefreshModelMarketplace`。
3. 自动刷新成功后直接展示目录并沿用现有父级目录失效通知。
4. 自动刷新失败后不循环重试、不弹全局错误，保留现有提示与手动刷新入口。
5. 有有效缓存或仍可展示的旧缓存时不自动联网。

## 目标

- 消除模型广场首次无缓存时多余的手动点击。
- 自动联网严格限制为每次应用启动最多一次。
- 自动刷新与手动刷新共用同一防重和状态更新路径。
- 保持后端缓存、远端协议、价格投影及现有离线展示行为不变。

## 非目标与边界

- 不修改 `ModelMarketplace` 的 cache-only 后端契约。
- 不修改 `RefreshModelMarketplace`、AirClaude endpoint、8 秒超时、缓存格式或价格解析。
- 不为已有缓存增加后台刷新或定时刷新。
- 不增加自动重试、全局错误弹窗或网络状态探测。
- 不处理 `newDefaultPricingDependencies` 初始化错误被调用方忽略的诊断缺口；仅在风险中记录。
- 不修改模型可选择目录、订阅权限过滤、排序、价格卡或筛选行为。

## 用户场景与验收标准

### 首次无缓存

1. 用户在一次应用启动中首次进入模型广场。
2. cache-only 读取返回无来源，或所有来源均为 `unavailable` 且无模型。
3. 页面自动调用一次远端刷新。
4. 成功时展示模型目录，并通知父级刷新共享模型目录。

### 自动刷新失败

1. 自动刷新返回不可用结果或调用失败。
2. 页面显示现有“需要联网获取模型数据”与手动刷新入口。
3. 页面不自动发起第二次请求。
4. 用户点击手动刷新仍可再次请求。

### 同一应用内再次进入

1. 首次自动尝试失败后离开模型广场，再次进入。
2. 页面只读缓存，不再次自动联网。
3. 用户仍可手动刷新。

### 已有缓存

- 任一来源存在可展示模型时不自动刷新。
- `stale_usable` 等仍可展示的旧缓存不自动刷新。
- 正常进入仍保持一次 cache-only 读取。

## 证据与根因

Codegraph 对主工作区索引基线 `5c9a4ffcf3060c67c83b86454e35696431b116c3`
给出的调用路径为：

`ModelMarketplace` 组件挂载 → `app.ModelMarketplace` →
`App.ModelMarketplace` → `modelMarketplaceService.Read` → 磁盘缓存。

远端路径仅为：

刷新动作 → `app.RefreshModelMarketplace` →
`modelMarketplaceService.Refresh` → `transport.Fetch`。

工作树基线已同步到最新 `origin/main-v3`：
`02de3355b3ddd393c2ed0f056f5490c5499415c4`。由于该 worktree 不在 Codegraph
索引中，以下结论已用 worktree 实际源码重新确认：

- `desktop/frontend/src/features/model-marketplace/ui/ModelMarketplace.tsx` 的挂载 effect
  只调用 `loadMarketplace`，刷新只由 `refresh` 函数触发。
- `desktop/model_marketplace_app.go` 明确注释并实现 `ModelMarketplace` cache-only，
  `RefreshModelMarketplace` 是唯一可访问 AirClaude pricing endpoint 的绑定。
- `desktop/frontend/src/__tests__/model-marketplace.test.tsx` 当前测试明确要求初次进入
  只读缓存、无缓存后由显式刷新完成 bootstrap。
- `desktop/model_marketplace_app_test.go` 证明 cache-only `Read` 不访问 transport，刷新后
  才写入并复用缓存。

根因是产品行为契约停留在“必须显式刷新”的离线优先设计，没有为用户主动进入页面时的
冷启动增加一次性 bootstrap 协调。

## 方案选择

在 `App` 组件保存应用生命周期级的一次性领取器：第一次满足冷启动条件的模型广场实例
可以领取自动刷新资格，后续重新挂载不能再次领取。将领取函数作为 prop 传给
`ModelMarketplace`，避免模块级全局状态和组件卸载后计数重置。

`ModelMarketplace` 在 cache-only 结果落地后判断冷启动条件：

- `sources` 为空；或
- 所有来源均为 `pricingState=unavailable` 且模型列表为空。

满足条件且成功领取资格时，调用与手动按钮相同的刷新函数。刷新函数使用同步 in-flight
引用防重，避免自动 effect 与用户点击在同一渲染周期重复请求。刷新函数的成功、失败、
父级通知和 UI 状态沿用现有语义。

不选择后端 `ModelMarketplace` 自动访问网络，因为那会破坏已记录的 cache-only 契约、让
普通读取隐含最多 8 秒的网络阻塞，并扩大到本地价格准备等共享边界。

## 影响范围与契约

预计修改：

- `desktop/frontend/src/App.tsx`：持有应用生命周期自动尝试资格并传入模型广场。
- `desktop/frontend/src/features/model-marketplace/ui/ModelMarketplace.tsx`：冷启动判定、
  自动刷新 effect 和同步防重。
- `desktop/frontend/src/__tests__/model-marketplace.test.tsx`：更新首次无缓存契约，覆盖成功、
  失败、手动重试、再次挂载和已有缓存不联网。
- `desktop/frontend/src/__tests__/model-marketplace-app-contract.test.ts`：如现有静态契约需要，
  验证 App 级资格所有权和 prop 接线。
- 本主题任务与进度文档。

外部 API、Wails bridge 方法签名、Go 类型、缓存 JSON 和持久化路径均不变。

## 兼容性、迁移与回滚

无需数据迁移。旧缓存继续读取，新安装只多一次有边界的远端请求。回滚时删除 App 级领取器
和组件自动 effect，即恢复现有显式刷新行为；缓存文件仍兼容。

## 风险与控制

- **重复请求**：App 生命周期领取器限制自动尝试一次；组件同步 in-flight 引用合并自动与
  手动并发。
- **离线等待**：只在无可用缓存时发生一次，沿用现有 8 秒后端超时；失败后不循环。
- **React Strict Mode/effect 重放**：领取动作同步、幂等，第二次 effect 无法再次领取。
- **误刷新空但有效目录**：条件要求来源明确 `unavailable`；正常 fresh 空目录不自动刷新。
- **初始化失败不可诊断**：当前 App 构造丢弃定价依赖错误；本修复不扩大到该跨子系统契约，
  自动尝试后仍保留手动入口。后续可单独治理。
- **父级状态重复失效**：仅实际完成的刷新调用沿用一次 `onMarketplaceChanged`。

## 验证计划

### 自动化验证

- 运行模型广场组件测试，覆盖所有新增冷启动场景和原有 44 项行为。
- 运行模型广场 App 静态契约测试。
- 运行 `pnpm build`，验证 React/TypeScript 生产构建。
- 不运行 Go 测试：本修复不修改 Go 文件、Go module 或 Go 共享契约。

### 应用级验证

优先用 Codex 内置浏览器连接真实桌面前端/Wails 后端：

1. 隔离或指定空的模型广场缓存目录后启动应用。
2. 首次进入模型广场，观察一次自动刷新并出现真实模型。
3. 模拟远端失败或离线，确认提示保留且不循环。
4. 离开并再次进入，确认同一应用生命周期不再次自动请求。
5. 点击手动刷新，确认仍可请求。

若真实运行环境无法安全隔离缓存，则用组件集成测试作为最近真实环境，并明确记录应用验收
限制，不修改用户现有缓存。

### 最终检查

- 检查实际 Git diff、EOL 和任务收据。
- 运行受影响前端构建及仓库 `code_guard`。
- 未授权 PR，因此不运行或发布 PR Gate；如后续授权 PR，需先同步最新 `main-v3` 并执行
  `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`。
