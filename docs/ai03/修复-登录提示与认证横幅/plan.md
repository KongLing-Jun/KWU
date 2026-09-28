# 登录提示与认证横幅修复计划

## 背景与问题

在 ARM 虚拟机从 `1.0.5-apxxx` 更新到 `1.0.5` 正式版后，观察到以下登录体验问题：

1. 线上用户被删除后，旧账号登录仍显示 `phone or password is incorrect`，无法表达账号可能已不存在。
2. 登录成功后点击“新建会话”会复用已有空白会话，但顶部“官方登录”横幅仍存在；进入账户页再返回或再次新建会话后，横幅才消失。
3. 切换账户页的当前账号可能显示一串 UID，并要求用户手动切换。

本轮修复前两项。第三项中的 UID 展示、切换交互和重复验证仍暂不纳入；但 AMD64 + 开发云真实验收证明，“失效绑定下注册新账号”必然经过既有 `CloudSwitchAccount`，因此切换成功后的订阅标签页恢复必须纳入，否则第二项无法在真实路径上成立。

补充对照观察：在 AMD64 电脑上，应用启动后原登录已失效，完成重新登录后横幅会立即消失。源码对比表明，这不是 CPU 架构或登录、注册实现分叉导致，而是横幅可能来自两种不同状态：前端根据匿名账号临时合成的横幅，或当前标签页 `meta` 中真实保留的认证待处理状态。

## 当前行为与预期行为

### 登录错误提示

- 当前：云端返回 `INVALID_CREDENTIALS` 时，桌面端统一展示 `phone or password is incorrect`。
- 预期：保持错误码和登录流程不变，仅将安全提示改为 `account does not exist or password is incorrect`。

该提示仍不区分“账号不存在”和“密码错误”，避免引入账号枚举能力。

### 登录后认证横幅

- 当前：账号匿名且当前模型走订阅线路时，前端会临时合成认证横幅；此时若底层会话状态已经是 `ready`，登录后仅更新账号状态即可让横幅消失，对应 AMD64 对照场景。
- 当前：若当前标签页 `meta.sendAvailability` 本身为 `subscription_auth_required`，登录或注册后即使后端已恢复会话，旧前端快照仍可能继续显示横幅。
- 最新 `main-v3` 的 PR #241 已增加通用 `model-route:changed` 广播：路由恢复成功时会触发标签页元数据刷新。但该 PR 未以认证横幅为目标，也没有覆盖登录或注册到横幅消失的完整验收。
- 预期：在保留 #241 通用事件刷新机制的基础上，登录或注册成功后通过既有账户成功回调显式刷新当前标签页 `meta`；若认证产生候选账号并完成显式切换，`CloudSwitchAccount` 还必须执行与登录、注册一致的订阅标签页恢复。
- 保持：新建空白会话的幂等复用逻辑不变，不创建重复空白会话，也不强制跳转到新会话 ID。

## 目标

- 为 `INVALID_CREDENTIALS` 提供同时覆盖账号不存在和密码错误的安全提示。
- 登录或注册成功后，显式刷新当前标签页元数据。
- 切换账号成功后恢复当前 Profile 下等待认证的订阅标签页。
- 保持空白会话复用、认证成功语义、错误码和请求标识不变。
- 将改动限制在既有错误映射和既有账户成功回调内。

## 非目标

- 修改云端服务的错误分类或用户删除策略。
- 在客户端判断账号究竟不存在还是密码错误。
- 修改空白会话创建、选择或去重逻辑。
- 引入全局认证事件、跨标签页批量刷新或新的组件属性。
- 自动重新绑定本地资料、自动切换账户，或改变 UID 展示规则。
- 处理第三项“当前账号显示 UID 且要求切换”的体验问题。
- 修改账户切换协议、绑定确认语义、UID 展示、重复验证或自动切换策略。

## 调查基线与源码证据

- Codegraph 索引基线：`1628871f64dc13c43c9dbda35dd4134616bee293`。
- 初始计划基线：`5044ee9a`，同时为 `desktop-v1.0.5` 标签。
- 当前实施 worktree 基线：`1c39dbf1`，即同步 PR #241 后的最新 `origin/main-v3`。
- Codegraph 用于定位初始调用链；由于最新基线晚于索引提交，PR #241 的变化已在独立 worktree 中通过实际 Git 差异和最新基线源码重新确认。

### 错误提示链路

- `internal/cloud/auth_fixture_client.go` 的 `HTTPSessionClient.Login` 请求 `/api/v1/auth/login`，结构化云端错误由 `decodeCloudResponse` 返回。
- `desktop/cloud_api.go` 的 `cloudWailsError` 将 `INVALID_CREDENTIALS` 映射为 `phone or password is incorrect`。
- `desktop/frontend/src/components/AccountCenter.tsx` 直接展示登录异常的 `error.message`。
- 生产云端服务不在当前仓库，因此无法仅凭本仓库证明用户删除后服务端为何返回 `INVALID_CREDENTIALS`；客户端提示不完整的路径已由源码确认。

### 横幅刷新链路

- `desktop/frontend/src/App.tsx` 优先使用 `state.meta?.sendAvailability` 计算横幅状态。
- `desktop/frontend/src/lib/sendAvailability.ts` 在账号匿名且使用订阅线路时临时返回 `subscription_auth_required`；账号认证后则直接返回底层会话状态。
- 因此，底层状态为 `ready` 时只更新账号状态即可消除横幅；底层状态本身为 `subscription_auth_required` 时必须重新读取标签页 `meta`。
- `desktop/tabs.go` 将 `subscriptionAuthPending` 映射为 `subscription_auth_required`。
- `desktop/cloud_api.go` 的 `CloudLogin` 和 `CloudRegister` 在成功后执行相同的 `cloudBootstrapForActor`、模型刷新和 `restoreAuthenticatedSubscriptionTabs`，不存在注册少执行恢复步骤的分叉。
- AMD64 + `http://192.168.10.243:8080` 真实复现表明：失效绑定下注册后产生候选账号，用户确认切换后账户与持久会话均已认证，但当前标签页仍为 `subscription_auth_required`，官方模型目录为空；重启后 `CloudBootstrap` 才恢复。
- `CloudSwitchAccount` 成功后会清空并刷新订阅模型，但没有调用 `restoreAuthenticatedSubscriptionTabs`。前端随后执行的 `refreshMeta` 只能重新读取未恢复的后端标签页状态。
- `AccountCenter.tsx` 的登录和注册成功路径均执行 `commitView` 后调用同一个 `syncMarketplaceAfterAuth`，继而调用 `onMarketplaceChanged`。
- `App.tsx` 当前将该回调直接绑定为 `refreshModelCatalog`，未调用 `refreshMeta`。
- `desktop/frontend/src/features/session/useController.ts` 的 `refreshMeta` 会重新读取当前标签页的 `MetaForTab` 并更新前端状态。

### 最新主线的通用刷新

- PR #241 在成功执行 `setModelRouteForTabWithPersistence` 后发送 `model-route:changed`。
- `App.tsx` 监听该事件，刷新全部标签页元数据；事件属于当前标签页时还会调用 `refreshMeta`。
- 登录或注册恢复认证待处理会话时会设置模型路由，因此通常能够触发该事件。
- 该机制是跨客户端模型路由同步的通用能力，不是认证完成契约。认证成功后的显式刷新作为低频兜底，覆盖事件未产生、延迟或未命中当前标签页的情况。

### 空白会话幂等性

- `desktop/tabs.go` 的 `EnsureBlankTab` 会优先复用已有空白标签页。
- `desktop/app_session_dedup_test.go` 已覆盖复用既有空白会话的行为。

### 第三项问题

- 本地账号绑定持久化在资料目录的 `binding.json`。
- 切换账户页当前账号标签使用 `display_name || cloud_user_id`；用户删除后旧绑定无法补全展示名时，会回退显示 UID。
- 自动修复需要定义哪些终止原因允许自动重新绑定，以及旧资料和新云端身份之间的兼容边界。本轮不承担该风险。

## 选定方案

### 1. 最小修改登录错误映射

仅修改桌面端 `INVALID_CREDENTIALS` 的安全消息：

```text
account does not exist or password is incorrect
```

保留错误码、`request_id`、HTTP/云端协议和前端展示方式。

### 2. 在通用事件刷新上增加认证成功保障

在 `App.tsx` 给 `AccountCenter` 传入组合回调：

1. 保持调用 `refreshModelCatalog()`。
2. 随后调用 `refreshMeta()`，重新获取当前标签页元数据。

不改变 `AccountCenter` 的认证流程或属性契约，不新增全局事件，也不修改 `ModelMarketplace` 当前的目录刷新回调。PR #241 的 `model-route:changed` 继续作为通用同步机制；认证回调刷新作为明确兜底。

`AccountCenter` 的既有回调也用于确认切换账户成功后的前端同步。真实验收证明仅刷新前端不足，因此在 `CloudSwitchAccount` 完成绑定提交、模型目录刷新并释放账户操作锁后，调用既有 `restoreAuthenticatedSubscriptionTabs`。不改变切换协议、绑定、凭据、会话 ID 或 UI。

## 影响范围

预计修改：

- `desktop/cloud_api.go`：更新 `INVALID_CREDENTIALS` 安全消息。
- `desktop/cloud_api.go`：切换成功后释放账户操作锁并恢复等待认证的订阅标签页。
- `desktop/frontend/src/App.tsx`：组合账户认证成功后的目录和当前标签页刷新。
- `desktop/cloud_api_real_test.go`：验证错误映射消息、错误码和 `request_id`。
- `desktop/frontend/src/__tests__/account-workspace-contract.test.ts`：更新账户工作区回调连线契约。
- 直接相关 Account Center 测试：验证登录和注册共用成功同步回调。

明确禁止修改：

- 空白会话去重和创建逻辑。
- 云端认证协议、账户绑定持久化和切换账户策略。
- 与本问题无关的模型市场刷新行为。

## 兼容性与迁移

- 不改变 API、数据结构、错误码、持久化格式或会话标识。
- 无数据迁移。
- 旧服务端仍可继续返回 `INVALID_CREDENTIALS`；仅客户端文案更准确。
- 登录成功后的新增读取为现有 `MetaForTab` 调用，不影响后端认证提交。
- #241 事件和认证成功回调可能在同一次恢复中分别触发刷新；认证属于低频操作，允许该次冗余读取。

## 风险与控制

- 文案范围变宽：符合用户认知，同时继续模糊账号是否存在，避免账号枚举。
- 当前标签页闭包变化：认证页和会话页共用当前 active tab；通过前端测试确认组合回调读取的是认证完成时的当前标签页。
- 元数据刷新失败：`refreshMeta` 保持现有容错，不回滚已成功的认证；最坏情况下横幅等待后续正常刷新，行为不比当前更差。
- 重复刷新：#241 路由事件与认证回调可能各执行一次元数据读取；不引入写操作或循环触发，低频成本可接受。
- 锁顺序回归：账户切换持有 `activeTabTransitionMu`，而标签页恢复会进入正常路由重建；必须先结束云账户操作再恢复，并用测试证明调用完成、无重复释放。
- 切换已提交但标签页恢复失败：保持既有登录/注册语义，返回安全错误但不回滚已成功的账户切换；后续 `CloudBootstrap` 或资源刷新仍可恢复。
- 误改模型市场行为：只组合 `AccountCenter` 的回调，`ModelMarketplace` 继续使用原回调。
- 会话幂等性回归：不修改相关代码，并保留既有去重测试作为回归保护。

## 验收标准

### 自动化验收

- `INVALID_CREDENTIALS` 仍返回原错误码和 `request_id`，消息变为 `account does not exist or password is incorrect`。
- 登录和注册成功后，既有 `onMarketplaceChanged` 回调仍被调用。
- `App` 的账户成功回调依次触发模型目录刷新和当前标签页 `meta` 刷新；#241 的模型路由事件监听保持不变。
- 确认切换账户后，匹配当前 Profile 的认证待处理订阅标签页恢复；其他 Profile 不受影响。
- `EnsureBlankTab` 继续复用既有空白会话。
- 前端生产构建通过；Go 仅运行受影响的 Desktop 包测试。

### 应用验收

1. 使用已删除或不存在的账号登录，观察到新提示；错误码语义和登录失败状态不变。
2. 在底层会话已经 `ready`、仅因账号匿名而显示横幅的场景完成登录，确认横幅立即消失。
3. 打开底层 `meta` 为 `subscription_auth_required` 的订阅认证待处理会话，分别完成登录、注册以及“注册后确认切换账号”，确认返回会话或点击“新建会话”后横幅立即消失。
4. 若此前已有空白会话，确认仍复用同一会话，不额外创建空白会话。

实际 ARM 环境如需要验证码或用户凭据，由用户按上述步骤完成最终应用验收；可自动覆盖的前端状态转换由测试完成。

## 测试计划

- 为桌面端安全错误映射增加或调整聚焦测试。
- 在现有 Account Center 测试中覆盖登录和注册共用成功同步回调，并保持确认切换账户的既有行为。
- 更新并运行 `desktop/frontend/src/__tests__/account-workspace-contract.test.ts`，验证 `AccountCenter` 使用组合回调，同时 `ModelMarketplace` 仍直接使用 `refreshModelCatalog`。
- 保留并运行 #241 的浏览器/客户端同步契约测试，确认通用 `model-route:changed` 刷新没有被改坏。
- 运行既有 `TestEnsureBlankTabReusesExistingBlankTab`。
- 运行订阅认证恢复相关 Desktop 测试。
- 增加账户切换成功后恢复匹配订阅标签页、且不影响其他 Profile 的 Desktop 测试。
- 前端变更运行 `pnpm build`；最终 PR 前按仓库规则运行 PR Gate。

## 回滚

- 回退错误消息和 `AccountCenter` 回调组合即可。
- 无数据、协议或持久状态需要恢复。

## 待确认事项

- 无产品决策待确认。
- 任务拆解阶段需确定 Account Center 登录成功回调的现有测试落点，但不得因此扩大生产代码范围。
