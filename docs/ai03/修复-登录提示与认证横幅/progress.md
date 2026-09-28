# 登录提示与认证横幅修复进度

## 当前阶段

- 阶段：Stage 3，实施 PR 收尾。
- 状态：`CloudSwitchAccount` 恢复链路、自动化验证和 AMD64 + 开发云应用验收均已完成，准备同步最新基线并运行最终 PR Gate。
- 工作分支：`codex/impl-login-auth-ux`。
- 基线：包含任务拆解 PR #242 的 `origin/main-v3`，提交 `ecfebd2d`。

## 已确认决策

- 本轮仅修复登录错误提示和登录后认证横幅。
- `INVALID_CREDENTIALS` 文案改为 `account does not exist or password is incorrect`。
- 保持空白会话幂等复用，不修改新建会话逻辑。
- 登录与注册后端恢复链路一致；失效绑定下注册会转入账户切换，而 `CloudSwitchAccount` 缺少相同恢复步骤。
- 区分账号匿名时的前端合成横幅和标签页 `meta` 本身的认证横幅。
- 保留 PR #241 的通用 `model-route:changed` 刷新，并复用 `AccountCenter` 现有成功回调，在刷新模型目录后显式刷新当前标签页 `meta`。
- 接受认证时可能发生一次低频冗余元数据读取。
- 不新增全局认证事件、组件属性或跨标签页批量刷新。
- 当前账号显示 UID、切换交互和重复验证延期处理；仅将切换成功后的订阅标签页恢复纳入本轮。

## 调查与验证

- `INVALID_CREDENTIALS` 现在返回新的合并安全提示，并由直接测试锁定错误码、消息、`request_id` 和服务端细节过滤。
- `AccountCenter` 现在使用命名的稳定组合回调，先刷新模型目录，再等待当前标签页 `meta`；`ModelMarketplace` 连线保持不变。
- 登录、注册、PR #241 同步、空白会话复用和认证后订阅恢复的聚焦回归均通过。
- Desktop 受影响包 `go test . -count=1` 通过。
- 前端生产构建 `pnpm build` 通过，仅有既有的大 chunk 警告。
- 主会话独立审查实际实现提交和差异，未发现缺陷。
- 主会话独立重跑 Desktop 五个相关测试、账户工作区 77 项、账户登录注册 51 项、browser/client sync 和 send availability 37 项，全部通过。
- 修复前基线在 AMD64 + 开发云真实复现：注册、确认切换账号、点击新建对话后横幅仍存在。
- 原修复版在相同路径仍失败：账户已切换且持久会话已认证，但当前标签页仍为 `subscription_auth_required`、模型目录为空；重启后 `CloudBootstrap` 才恢复。
- 源码确认 `CloudLogin`、`CloudRegister`、`CloudBootstrap` 和 `CloudRefreshAssets` 调用 `restoreAuthenticatedSubscriptionTabs`，`CloudSwitchAccount` 未调用。
- 返工后 `CloudSwitchAccount` 在模型刷新后先释放账户操作锁，再调用既有恢复函数；条件 defer 防止提前返回漏释放或成功路径双重释放。
- 新增直接测试证明切换成功后当前 Profile 的待认证订阅标签页恢复、其他 Profile 保持不变，且 `activeTabTransitionMu` 已释放。
- 两组 Go 聚焦回归、账户工作区 77 项、账户登录注册 51 项、browser/client sync、Desktop 受影响包和前端生产构建均通过。
- 最终 AMD64 应用验收使用隔离的 `APPDATA`、`LOCALAPPDATA`、WebView 数据目录和 acceptance 版本标识，并通过 IPv4 映射 IPv6 地址连接同一开发云平台，未干扰其他开发实例。
- 最终真实路径验收通过：从失效登录状态进入，完成注册并确认切换账号后，官方登录横幅立即消失；再次点击“新建对话”横幅未恢复，官方订阅模型可用，空白会话仍保持幂等。

## 交付状态

- `plan.md`：已编写。
- 计划 PR：#240 已合并。
- `tasks/001-修复登录提示与认证横幅.md`：已更新完成回执。
- 任务拆解 PR：#242 已合并。
- 返工代码、直接测试、主会话复核和 AMD64 + 开发云应用重验已完成。
- 实现 PR：未创建；用户已授权同步最新 `main-v3`、运行 PR Gate、推送并创建 Draft PR。

## 风险与阻塞

- 当前无技术阻塞；锁顺序、双重释放防护和 Profile 隔离已有直接测试证据。
- 生产云端服务不在仓库内，无法从本仓库证明删除用户后的服务端错误分类；本轮仅修正已确认的客户端安全提示。
- 真实线上删除账号和 ARM64 升级环境仍属于后续补充验证，不阻塞本轮已确认的 AMD64 + 开发云修复交付。
- 认证恢复时通用路由事件和显式回调可能各读取一次元数据，这是已接受的低频冗余。

## 下一道门

1. 同步最新 `origin/main-v3` 并复核冲突与相关调用链。
2. 重跑聚焦检查、关键应用路径和最终 PR Gate。
3. 推送分支并创建 Draft PR。

## 剩余工作

- 同步最终基线、重跑必要检查与 PR Gate。
- 推送分支并创建实现 Draft PR。
- 后续按需补充 ARM64 升级和真实线上删除账号验收。
