# 桌面失效账户与会话环境隔离修复计划

## 1. 问题与目标

当前桌面端会把已经不能使用的云会话继续显示成一个 UID 账户，账户页同时出现余额、
订阅横线且没有重新登录提示。用户无法理解这个状态，也无法顺畅恢复。

事故取证还证明，历史开发构建曾从正式数据根读取 device/session 并直连开发环境，
正式构建后来又把该开发 token 发给生产环境。详细证据见
[evidence.md](./evidence.md)。

本次只解决两件事：

1. 不能使用的账户明确进入“登录已失效”，复用现有登录/注册流程。
2. 本地认证文件绑定环境；旧 schema 或环境不匹配时拒绝使用其中的 token。

## 2. 用户体验

### 登录已失效

- 左下角折叠入口显示“登录已失效”，保留现有展开箭头。
- 点击仍展开现有账户菜单，不直接跳页。
- 菜单顶部显示“登录状态已失效”和“点击登录 / 注册”；点击后打开现有账户页。
- 菜单其余 DOM 与行为完全复用当前未登录状态：保留本地头像与头像上传、总余额和
  套餐支付入口，并显示既有未登录占位值；不显示旧 UID、云头像、云余额、订阅或支付
  数据，也不显示退出登录。
- 设置、更新、外观继续可用。
- 账户页直接显示现有登录/注册表单，并内联显示
  “登录状态已失效，请重新登录”。
- 登录原账号、其他账号或注册均可恢复。
- 不新增中间页、额外按钮、弹窗或 Toast。

### 保持不变

- 从未登录或主动退出仍显示“登录 / 注册”，没有失效提示。
- offline、timeout、5xx 和限流保持现有 offline/retry 语义。
- 正常登录、refresh、注册和账号切换流程保持不变。

## 3. 范围

### 必须完成

- binding/session 持久化文档记录并校验当前客户端环境。
- 旧 schema、环境不匹配、未知 schema 或损坏内容 fail closed，token 不得出站。
- 确定性认证终止只把当前 session 写成 expired，并清除其中的认证材料。
- 保存稳定的终止原因，供日志、测试和后续诊断使用。
- expired 不再被前端当作可恢复账户，不显示 UID 或空账户概览。
- expired 时撤销内存中的账户和订阅模型可用性，但不删除非认证磁盘缓存。
- 增加 Go、前端和桌面应用验收。

### 明确不做

- 不修改 Reasonix-Cloud、数据库、API 或部署。
- 不引入 binding/session 跨文件事务或通用 credential coordinator。
- 不修改 `BindingScope`，不为运行时 401 推进 binding generation。
- 不重构账号切换、refresh 并发、跨进程文件锁或生产单实例策略。
- 不级联删除余额、订阅、游标、快照等非认证磁盘缓存。
- 不保留旧 schema 或跨环境登录连续性；用户重新登录即可。

## 4. 验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 正式构建读取含开发 token 的旧 schema | token 不出站；显示“登录已失效” |
| AC-02 | prod/dev 或 dev A/dev B 环境不匹配 | 拒绝凭据；行为同 AC-01 |
| AC-03 | 同环境有效新 schema | 正常恢复账户 |
| AC-04 | cloud base URL 变化 | 视为环境变化，要求重新登录 |
| AC-05 | durable 内容损坏或 schema 未知 | fail closed，不显示 UID 或使用 token |
| AC-06 | durable 文件临时 I/O 错误 | 不改原文件；显示可重试本地错误 |
| AC-07 | 当前 session 收到 `AUTH_REQUIRED` | 清 token、持久化 expired；重启后仍需登录 |
| AC-08 | 旧 token 的终止响应晚于 rotation | 不覆盖 rotation 后的新 session |
| AC-09 | timeout、offline、5xx 或限流 | 保留 token 和 offline/retry |
| AC-10 | 展开失效菜单 | 除顶部失效文案外复用未登录菜单；仅显示本地头像和未登录占位值，不恢复旧云数据或退出登录 |
| AC-11 | 点击展开菜单顶部的失效入口 | 打开现有登录/注册页并显示内联说明 |
| AC-12 | 重新登录或注册成功 | 清失效原因，恢复正常账户 UI |
| AC-13 | 主动退出或从未登录 | 仍显示“登录 / 注册” |

## 5. 最小实现方案

### 5.1 环境身份与 durable 校验

增加内部 `EnvironmentIdentity`，由构建类别和规范化 cloud base URL 派生：

- `version == "dev"` 为 development；其他候选/发布版本为 production。
- 保存完整 `environment_id` 和规范化 URL；读取时两者都必须匹配。
- URL 规范化复用现有 dev profile 的等价规则。
- 现有 dev profile 目录 ID 和数据根布局不变。
- real runtime 必须显式传入环境身份；fixture/test runtime 显式传入稳定且非生产的
  测试环境身份，不允许构造函数静默省略。

binding 文档 v1 升为 v2，session 文档 v2 升为 v3。环境字段只放在 durable
envelope，不进入 `ProfileBinding`、`BindingScope` 或云端协议。

读取规则：

- 新 schema 且环境匹配：按现有逻辑加载。
- 旧/未知 schema、环境不匹配或内容损坏：返回可诊断的拒绝原因，但不返回 token；
  认证视图投影为 expired。
- 临时 I/O 错误：返回本地可重试错误，不改写原文件，也不误判成远端登录失效。
- 成功登录/注册且远端结果验证通过后，使用窄化的认证成功替换入口，在现有 path
  lock 下分别原子覆盖被拒绝的 binding/session 文档。该入口不用于启动恢复或普通
  session Save，不承诺跨文件原子性；任一步失败时仍保持 fail closed，旧 token
  不得重新可达。

替换入口允许 rejected/expired binding 转为 bound，并使用下一 binding generation；
session 替换直接丢弃被拒绝文档中的旧条目和 token。该规则只用于环境/旧 schema
拒绝后的首次成功登录。普通运行时 401 和同账号重新登录不改变 generation 规则。

本地拒绝原因固定为：

- `LEGACY_CREDENTIAL_STATE`
- `ENVIRONMENT_MISMATCH`
- `DURABLE_CREDENTIAL_STATE_INVALID`

### 5.2 运行时认证终止

`AuthApplication.persistFailure`（显式 refresh 路径）和 authenticated request
executor 的终止写入都必须遵循同一契约。`AUTH_REQUIRED`、token/refresh 失效、
device mismatch、account disabled 等现有确定性终止错误只修改请求所属的 session：

1. executor 记录发起请求时观察到的 session/token。
2. 在现有 `refreshMu` 内重新加载当前 session。
3. 如果 session/token 已经 rotation，则只返回本次操作错误。
4. 如果仍是同一 session，则写入 `SessionExpired`，清空 access token、refresh
   token、session device ID 和 expiry，并保存稳定 code/UTC 时间。

安装级 `cloud/device-id` 不删除。网络错误、5xx 和限流继续走 offline/retry。
两条路径可复用小型 session scrub/reason helper，但不得新增 credential
coordinator。本路径不修改 binding，不触碰账号切换，也不新增跨文件事务。

### 5.3 桌面与前端投影

- 桌面继续使用 `CloudAccountView.status = "expired"`。
- `hasRecoverableCloudIdentity` 只接受 authenticated，以及带稳定 ID 的 offline；
  expired 单独判断为需要重新认证。
- expired 不使用 UID 作为文案、云头像槽位或账户快照恢复依据；菜单改用未登录状态
  的本地头像槽位和占位值。
- 进入 expired 时清空余额、订阅和支付的内存视图。
- sidebar、菜单、AccountCenter 和中英文 locale 按第 2 节渲染。

桌面 billing 仅在 `AccountExpired` 或 binding expired/mismatch/unbound 时撤销旧
controller scope 和内存 subscription model catalog。`AccountOffline +
BindingBound` 保留现有 scope/catalog 和 retryable 行为。磁盘缓存不删除。

## 6. 影响范围

预计修改：

- `internal/cloud`：环境身份、durable envelope、认证视图和终止 session 清理。
- `desktop/cloud_runtime.go`、`desktop/cloud_api.go`：注入环境并投影失效。
- `desktop/model_billing.go`：expired 内存撤权。
- `desktop/frontend/src/App.tsx`、账户 store/hook/AccountCenter、locale 和聚焦测试。

仅当复用 URL 规范化确有必要时调整 `desktop/dev_profile.go`，不得改变目录 ID。
不得修改依赖、云平台或 `scripts/code_guard_baseline.json`。

## 7. 风险与防护

整体难度为中等，实施残余风险为中低。UI 收敛是低风险；durable schema 和运行时
迟到响应保护是主要技术工作。

- 正常凭据误判：同时比较 environment ID 和规范化 URL，并用固定向量测试。
- 迟到错误覆盖新 token：在现有 `refreshMu` 内比较 observed/current session。
- offline 被误判 expired：对错误分类和前端状态分别测试。
- 旧缓存回填：环境拒绝后首次登录使用下一 generation；expired 清内存视图。
- 降级读取不了新 schema：前向修复；必要时停止桌面后仅清认证文件并重新登录。
- 跨进程仍可能竞争同一生产数据根：安装包单实例降低概率；本次不增加文件锁。

若实施需要通用事务、账号切换重构、跨进程锁或云平台改动，必须停止并重新确认，
不能把“顺便硬化”加入本修复。

## 8. 验证

### 自动化

```powershell
go test ./internal/cloud ./internal/cloudquota ./internal/modelbilling ./internal/modelbillingworker -count=1
go vet ./internal/cloud ./internal/cloudquota ./internal/modelbilling ./internal/modelbillingworker

cd desktop
go test . -count=1
go vet .

cd frontend
pnpm test:typecheck
pnpm exec tsx src/__tests__/account-store.test.ts
pnpm exec tsx src/__tests__/account-sidebar-bootstrap.test.tsx
pnpm exec tsx src/__tests__/account-center-states.test.tsx
pnpm exec tsx src/__tests__/workbuddy-primary-navigation.test.ts
```

Go 测试覆盖 real/fixture 环境身份、环境矩阵、旧/损坏 schema、I/O、认证成功替换、
两条终止写入路径的 token scrub/reason 持久化、迟到终止、重启和重新登录。
前端/桌面测试覆盖 expired/offline/anonymous、菜单、登录页说明、旧 UID/云头像
隐藏、本地头像与未登录占位复用、内存数据撤权和 offline 缓存保持。

### 应用验收

- 正式构建：在隔离数据根预置开发/旧凭据，并用有记录的 OS 级网络阻断确认 token
  不出站、左下角显示“登录已失效”、现有登录/注册页显示内联说明。不执行真实生产登录。
- 受控 dev/test 构建：本地 stub 返回 `AUTH_REQUIRED`，验证 token 清理、重启和
  重新登录；再注入 timeout/5xx/I/O 错误，验证不会误判 expired。

应用稳定后同步最新 `origin/main-v3`，重跑聚焦检查和关键应用路径，再执行：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

任何必需检查失败时不得创建或更新 PR。

## 9. 基线与关口

- Codegraph 导航：`D:\Reasonix@345a695b28fae2b6f09916c745ffcd52e1885f24`
- 当前 worktree：
  `origin/main-v3@e4000e52d91214e16870163acbf10cd6ab553833`

Codegraph 只作旧基线导航；材料已在 worktree 直接复核，见
[evidence.md](./evidence.md)。

本计划没有待定产品问题。未经明确授权，不提交、不推送、不创建 PR，也不进入任务
拆解或实施。
