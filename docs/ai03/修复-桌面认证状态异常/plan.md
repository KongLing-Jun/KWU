# 桌面认证状态异常修复计划

## 1. 修复边界

调查阶段已在未修改产品代码的
`origin/main-v3@768d8fa2711c7fa0b48460677aa78037806fd053` 固化自动化、Wails dev 和本机
production 证据。修复分支已合并最新
`origin/main-v3@663297b7f5c9929c1cca383111597e9e46be7323`；该基线仅在
`desktop/cloud_runtime.go` 增加项目 command replay 状态，没有改变认证 executor
所有权、session 合同或账户前端状态。工作目录继续使用独立 worktree
`D:\Reasonix-auth-investigation`，主工作区的未跟踪 `artifacts` 不读取、不修改、不清理。

目标：消除同一 cloud runtime 内跨 executor 的 refresh token 重用；对 `/me` 的迟到
terminal 错误有界验证当前 replacement；让真正终止的 session 与 bootstrap/UI 同步
进入 expired；hydration 失败时不把内部 cloud user ID 当正常用户名展示。

非目标：不修改云端 refresh 协议、token TTL、JWT、数据库、device 语义、durable
credential schema、binding generation、环境隔离、支付/项目请求的通用重放策略或跨进程
锁。桌面单例之外的多进程协调没有本次运行证据，不扩大为新的持久锁协议。

禁止记录 access token、refresh token、手机号、密码、完整 Authorization header，
也不记录 token 前后缀或可复用摘要。测试只允许输出 token 是否存在、过期时间、
session state、binding generation、错误 code/request_id 等不可复用信息。

## 2. 现场与版本证据

### 2.1 正在运行的生产程序

- 进程：`C:\Users\ai03\AppData\Local\Programs\云智企办\clouvia-desktop.exe`
- PID：`36616`
- 启动时间：`2026-08-19T18:30:56.965012+08:00`
- File/Product version：`1.2.1`
- Go build：`go1.26.4`、`GOOS=windows`、`GOARCH=amd64`
- ldflags：`main.version=v1.2.1`、`main.channel=stable`
- build tags：`production,devtools`
- 签名：有效，签名主体为广州鑫恒泰互联科技有限公司
- SHA-256：只用于区分本机安装产物，不写入用户日志
- cloud base URL：生产构建忽略开发 override，源码常量与安装二进制均包含
  `https://www.suiyuan-ai.com`

安装二进制没有嵌入 Git commit。`desktop-v1.2.1` 标签和本机保留的 v1.2.1
release source worktree 都指向 `58ed531d895a7af1f5ccd2718c9f427b2bd9b9b3`；
保留的 release exe 是 ARM64，当前安装 exe 是 AMD64，不能用二进制哈希直接证明
两者来自同一 commit。因此“当前安装包精确 commit”为待补发布 provenance，而不是
假设为当前源码或标签。

### 2.2 本机生产持久状态安全摘要

采样时间 `2026-08-20T23:00:39+08:00`，本机时区 `China Standard Time`：

| 项目 | 结果 |
| --- | --- |
| 生产根 | `%APPDATA%\Clouvia` |
| binding schema | `reasonix.cloud.binding` v2 |
| session schema | `reasonix.cloud.sessions` v3 |
| environment ID | 与 production + `https://www.suiyuan-ai.com/` 派生值一致 |
| profile / actor | `default` / `local` |
| binding | `bound`, generation 6, cloud user ID present |
| session | `authenticated`, access/refresh token present |
| access expiry | `2026-08-20T22:12:32+08:00`，采样时已本地过期约 48 分钟 |
| refresh expiry | `2026-09-18T11:14:36+08:00`，未自然到期 |

开发根 `%APPDATA%\kwu-dev\environments` 存在但没有环境实例。公开
`/environmentz` 采样的 HTTP Date 比请求结束本机 UTC 约快 24 秒，不足以解释
一小时边界异常；这只是单次粗粒度证据，不替代同一次故障的 server_time。

当前运行 UI 的只读观察显示正常账户身份且没有登录失效提示。没有点击账户、刷新或
触发远端请求，因此这只证明“本地 access 已过期时旧 UI 快照仍可存在”，不作为
故障根因证据。

## 3. 已证明的源码状态链

### 3.1 恢复与正常 hydration

1. `CloudBootstrap` 和 `CloudRefreshAssets` 都进入
   `cloudBootstrapForActor`。
2. `AuthApplication.Recover` 与 `State` 都在 `identityMu` 下从 durable binding /
   session 派生 `AuthView`。
3. bound + authenticated session 产生 `AuthView.State=authenticated`，保留
   binding status/generation/cloud user ID。
4. `AuthView.User` 只来自进程内 `views` map；durable binding/session 不保存
   display name。进程重启后第一次 recovery 的 `AuthView.User` 为空。
5. `RemoteAccountApplication.CurrentUser` 通过它内部的
   `AuthenticatedRequestExecutor` 刷新/重放 `/me`，成功后才产生带 display name
   的 `AccountState`。
6. `cloudAccountView` 优先以 `AccountState.User` 覆盖 `AuthView.User`；成功时
   account 为 authenticated/bound，display name 和 cloud user ID 都存在。
7. Zustand `applyBootstrap` 保存 account/assets，并把 `operation_error` 独立保存为
   `error`。AccountCenter 的显示名是
   `display_name || cloud_user_id || "—"`。

### 3.2 hydration 失败的混合状态

`RemoteAccountApplication.CurrentUser` 返回错误时，
`cloudBootstrapForActor` 重新调用 `runtime.auth.State`，用空 `AccountState` 调用
`cloudAccountView`，再把错误单独放入 `OperationError` 并以成功的 Wails 返回结束。

若错误处理没有终止当前 durable session，输出为：

| 字段 | 值 |
| --- | --- |
| Session.State | authenticated |
| AuthView.State | authenticated |
| binding | bound + 原 generation |
| account.status | authenticated |
| cloud_user_id | present |
| display_name | restart 后为空；同进程可保留内存旧值 |
| operation_error | terminal code + request_id |
| Zustand account | authenticated |
| Zustand error | operation_error |
| requiresReauthentication | false，只判断 `status === "expired"` |
| identityLabel | display name；为空时回退内部 cloud user ID |
| 登录失效 UI | 不出现 expired 专用重新登录提示 |

AccountCenter 打开时会渲染普通 error alert 及 code/request_id；侧栏和普通工作区不呈现
这个详细错误，因此用户可只看到退化后的 user ID，而看不到登录失效入口。

### 3.3 双 executor refresh 竞态

真实 runtime 创建两个独立 executor：

- `NewRemoteAccountApplication` 内部创建 executor A，服务 `/me`。
- `newRealCloudRuntimeWithRegistration` 又创建 executor B，服务 projects、credit、
  payment、MaaS、event、expert assets 等客户端。

两者共享同一个 `DurableCloudSessionStore`，但 `refreshMu` 是 executor 实例字段。
因此它们可同时读取同一个过期 session 和同一个旧 refresh token，并分别发起 rotation。

确定性时序：

1. A、B 都读取旧 authenticated session。
2. 服务端先让 B rotation 成功，B 保存 replacement。
3. A 收到 `REFRESH_TOKEN_REUSED`。
4. A 的 `persistFailureLocked` 重新读取 store，发现
   `sameObservedSession=false`，按迟到错误保护跳过终止，返回 terminal error。
5. bootstrap 重读 auth，得到 replacement 的 authenticated session；进程重启后
   display name 为空，于是输出 authenticated + user ID + operation error。
6. 服务端 reuse 规则已撤销整个 family，下一次 replacement 请求返回
   `AUTH_REQUIRED`。
7. 此次 observed/current 相同，executor 把 session scrub 为 expired；下一次
   bootstrap 才输出 expired，前端才进入登录失效 UI。

单 executor 内部的 `refreshMu` 可以避免重复 rotation，但不能协调上述两个实例。

## 4. 历史回归

| 提交 | 实际行为变化 |
| --- | --- |
| `3124f1d2` | 引入真实 `/me` hydration；失败时重读 auth，把 account status 与 `OperationError` 分离并成功返回。这一调用路径仍在最新 main-v3。 |
| `0fb4e602` | 引入实例级 `AuthenticatedRequestExecutor.refreshMu`。 |
| `0b1639ae` | runtime 增加第二个 executor，开始存在跨 executor 重复 refresh 风险。 |
| `5aac4cbc` | 修复开发/生产 durable 环境污染，并加入 observed/current CAS：迟到旧错误不得覆盖已轮换新 session。该保护本身正确，但使双 executor race 可短暂保留 authenticated replacement。 |
| `86bcb469` | 将真实 expired 从 recoverable identity 中移除，增加 `requiresReauthentication(status === expired)` 和登录失效 UI。 |
| `65e907cd` | 账户刷新改为 authoritative：hydration 失败不再回填旧资产，sidebar 资产事件改为远端 `CloudRefreshAssets`。没有改变 executor 所有权或 account/error 分离，但增加了该失败分支触发和可见机会。 |
| `62c99728` | 只调整带 operation error 时资产清空/合并规则；混合 account/error 合同仍保留。 |

因此：跨 executor 竞态从 `0b1639ae` 起存在；当前“迟到 terminal error 但 session
仍 authenticated”的必要保护从 `5aac4cbc` 起存在；用户可见的 user-ID 回退还依赖
重启后未持久化 display name 和 `3124f1d2` 的 account/error 分离。不能把单个提交
描述为完整故障的唯一成因。

## 5. 其他原因的证据等级

| 候选 | 当前结论 |
| --- | --- |
| 安装版本与源码不一致 | 版本/tag 高度对应，但 exe 不嵌 commit，不能精确排除；需发布 provenance。 |
| JWT signing key 多实例不一致 | 客户端源码无法证明或排除；需服务端按 request_id、instance/deploy revision 对齐日志。 |
| DEVICE_MISMATCH | 客户端有确定性终止分支；本机无该 code 运行证据。当前请求若命中同一 session 会 expired。 |
| refresh 30 天自然到期 | 本机当前 refresh expiry 尚有约 29 天，排除本次现场切片；历史故障仍需同次日志。 |
| durable environment/schema/profile mismatch | 本机 production 文档 schema、environment、profile 一致；不支持当前是旧污染。读取拒绝在源码中会投影 expired，而非 authenticated + user ID。 |
| 本机时钟偏差 | 单次公开 Date 粗测约 24 秒；可增加早刷新概率但不足以解释本次一小时级状态。需同次 server_time。 |
| 数据库错误统一映射 AUTH_REQUIRED | 仓库不含认证服务实现，客户端只传播 code；需服务端 request_id 和数据库错误分类。 |
| 旧 bootstrap 覆盖新状态 | 同 profile 低 generation/同 generation 异 UID 会被拒绝；同 generation 同 UID 没有全局 request epoch，源码风险可达但未获运行证据。 |
| contextKey/profile 旧快照回放 | snapshot 按 contextKey 缓存；切换会回放稳定快照。scope/generation 有校验，但请求排序仍不统一，属可达风险。 |
| sidebar / AccountCenter 不同 generation | 两者读同一 Zustand store，不会同时持有两份 store generation；但它们独立发请求，只有 AccountCenter 自身 operation epoch。 |
| operation_error 未呈现 | AccountCenter 会呈现；sidebar/普通工作区不会呈现详细 error，符合用户主观现象。 |
| 重启丢失 AuthView.User | 已由源码证明；display name 不持久化，第一次 `/me` hydration 失败即回退 cloud user ID。 |

## 6. 已固化的基线回归证据

调查阶段已经新增以下测试和安全诊断，产品代码仍保持基线行为：

1. 前端纯状态测试：注入 authenticated/bound、cloud user ID present、display name
   absent、`AUTH_REQUIRED` 或 `REFRESH_TOKEN_REUSED`，证明 Zustand 保存 account/error、
   identity 回退 user ID、`requiresReauthentication=false`、expired 提示不出现；再注入
   expired 对照，证明提示出现。
2. `internal/cloud` 双 executor 测试：一个 store、两个 executor、barrier 控制两个
   refresh 都读到旧 session；winner 保存 replacement，loser 返回 reused；断言 loser
   因 observed/current 不同不终止 replacement；再用 replacement 请求返回
   `AUTH_REQUIRED`，断言 scrub + expired。
3. desktop bootstrap 测试：使用真实 runtime composition 和受控 HTTP stub 重现
   account executor 与 shared executor 的竞态，断言第一次 bootstrap 为
   authenticated/bound/user ID/no display name/terminal operation error，第二次为 expired。
4. 测试日志只记录：测试 build/commit 参数、UTC/local timezone、调用来源、测试 executor
   ID、profile、generation、session state、expiry、refresh attempt ID、触发类型、HTTP
   code/request_id/status、前后 revision/expiry、sameObservedSession 结果、bootstrap account
   status/display-name-present/error code、前端 applyBootstrap 前后 account/error。
   token 只记录 present，不记录值、片段或摘要。

同步 `origin/main-v3@663297b7` 后的基线复核中，core 与两份前端证据继续通过；desktop
测试第一阶段仍精确得到两次 refresh 与混合 authenticated 状态，但第二阶段因 fixture
把 replacement access expiry 固定为 `2026-08-20T16:00:00Z`，在次日运行时意外触发
第三次 refresh，并命中 `req-cross-unexpected`。这是测试时钟夹具缺陷，不是根因变化；
实现任务必须把 replacement expiry 改为相对测试当前时间的确定性未来值，然后再把该
测试改为修复后的“一次 refresh + 正常 hydration/当前终止 expired”断言。

## 7. 修复验收、风险与回滚

### 验收

- runtime composition 只能创建一个供 account、project、credit、payment、event、MaaS
  等消费者共享的 `AuthenticatedRequestExecutor`；确定性并发测试中只允许一次 refresh。
- `/me` 执行前在首次解析出的固定 `BindingScope` 记录 observed session；第一次返回
  terminal error 后在同一 scope 重新读取 durable current，并确认 actor binding 仍等于
  首次 binding。仅当 current 为 authenticated、binding 未变化且 current 与 observed
  credential/expiry 不同时才允许再执行一次 `/me`：replacement 可用则返回完整
  `AccountState`，不得留下 terminal `operation_error`。
- replacement 也返回 `AUTH_REQUIRED` 或当前 session 本身终止时，durable session 必须
  scrub credential 并转 expired；同一次 bootstrap 必须输出 account.status=expired，
  Zustand 与 sidebar/AccountCenter 同步显示登录失效。
- retryable/network/offline 错误不得转 expired；迟到旧错误不得直接覆盖 replacement。
- 当前账户 display name 缺失时，sidebar 与 AccountCenter 显示既有
  `account.nameUnavailable` 安全文案，不显示 `usr_*`；账号切换 candidate 的身份区分合同
  不在本次修改范围。
- dev/prod environment identity、profile root、schema、binding generation、token TTL 和
  cloud base URL 行为保持不变，无数据迁移、无全量重新登录。
- 所有诊断继续禁止 credential、手机号、device ID、Authorization 和完整 cloud user ID。

### 风险

- 修改 `NewRemoteAccountApplication` 构造合同会影响现有测试与 desktop 组合根：全部调用者
  必须显式注入 executor，避免保留第二条隐式创建路径。
- 对任意 authenticated operation 通用重放可能重复支付或写操作；因此 C 只在幂等的
  `RemoteAccountApplication.CurrentUser` 内做一次有界重试，不改变 executor 的通用
  mutation replay 合同。
- replacement 在验证前可能已被服务端 family revocation 撤销；第二次 `/me` 必须仍走
  executor，让当前 terminal error 按既有路径 scrub/expire，不能只检查本地 state。
- 当前 session 已被第一次 terminal error 正确转为 expired、post-load 失败或 session 没有
  变化时不得重试，必须返回原始 terminal error，保留其安全 code/request_id。
- pre/post session 比较必须直接使用首次 binding 的固定 scope，不能通过 actor 重新解析成
  新 binding；并发 account switch/rebind 时不得拿新账户 session 重试旧账户 `/me`。
- 前端只改变可见标签，`cloud_user_id` 仍保留在 scope、binding、feedback 和请求校验中；
  不得删除内部身份字段或破坏 profile mismatch 恢复。
- 测试日志 helper 只接受布尔 presence、时间、枚举、generation、code、request_id；
  不接受 token/string credential 参数。

### 回滚

修复不迁移 durable 数据。回滚为按提交撤销 frontend 标签修复及 runtime/core executor
注入与 `/me` 有界恢复；现有 session 文件保持 schema v3。已经被旧版本撤销的 session
family 无法由客户端修复恢复，用户仍需重新登录一次。

## 8. 选定实现

- A：把 `NewRemoteAccountApplication` 改为要求调用者显式注入
  `AuthenticatedRequestExecutor`，以 executor 参数替换原 `now` 参数并删除构造器内部创建，
  不保留兼容形状。desktop composition 先创建唯一
  executor，再同时注入 remote account 与所有其他认证 client。实例内 `refreshMu` 因而
  成为整个 runtime 的 refresh single-flight 边界。
- C：把 remote account hydration 提取为一次执行 helper，并在首次执行前直接从现有
  `ProfileSessionRepository` 的 underlying store、使用首次 binding 的固定 scope 保存
  observed session。第一次 `/me` 返回 session terminal error 后在同一 scope 加载 current，
  并重新解析 actor binding；仅当 binding 与首次值相同、current 为 authenticated
  且 `sameObservedSession(current, observed)==false` 时，最多再调用一次同一共享 executor。
  replacement 可用则恢复用户详情；replacement 已撤销则第二次调用沿既有
  `persistFailureLocked` 路径 scrub/expire。current 已 expired、未变化、缺失或 post-load
  失败、binding 已切换时返回第一次原始 terminal error，不丢失 request_id。不得循环、
  sleep、直接读取或记录 token，也不得把该策略扩展到通用写请求。
- observer 只在最终 validated replacement hydration 成功后执行一次；首次迟到 terminal
  结果和所有最终失败路径均不得通知 observer。
- B：不在 `cloudAccountView` 里凭 error code 强制伪造 expired。以 durable session 为
  唯一真相：C 的第二次验证若失败，既有 executor 先 scrub/expire；
  `cloudBootstrapForActor` 随后重读 `AuthApplication.State`，同一响应自然输出 expired。
  这样保留迟到旧错误不得杀死有效 replacement 的合同。
- D：只修改当前账户的可见身份标签。`App.tsx` sidebar 与 `AccountCenter.tsx` 在
  recoverable account 缺少 display name 时使用 `account.nameUnavailable`；内部
  cloud user ID、scope key、feedback identity、candidate switch 数据保持不变。

修复设计必须同时保留：迟到旧错误不得覆盖新 rotation、offline/retryable 不转
expired、环境/schema/profile fail closed、generation 防倒退、token scrub 与重启后
expired、AccountCenter 的安全 code/request_id 呈现。

### 影响文件与合同

- `internal/cloud/auth_identity_lifecycle.go`：remote account 构造依赖与幂等 `/me` 有界
  恢复；`AuthenticatedRequestExecutor` 公共行为与 session schema 不变。
- `internal/cloud/remote_account_application_test.go`、
  `internal/cloud/auth_cross_executor_test.go`：显式注入与 replacement 成功/撤销两条路径。
- `desktop/cloud_runtime.go`：唯一 executor 的创建和注入顺序。
- `desktop/cloud_api_real_auth_state_test.go` 及最近 runtime 测试：真实 composition 只刷新
  一次，并验证 success/expired bootstrap；同时移除固定日期造成的跨日不稳定。
- `desktop/frontend/src/App.tsx`、
  `desktop/frontend/src/features/account/ui/AccountCenter.tsx` 与既有账户测试：当前账户安全
  标签；不新增 locale key。

### 兼容与验证

构造函数位于 Go `internal` 包，不是外部公共 API；所有仓库内调用点同步修改。无依赖、
module、wire DTO、Wails binding、JSON schema 或持久数据变化。验证按改动范围运行
`go test ./internal/cloud`、desktop 受影响 package、`pnpm build`，再执行仓库 PR gate。
真实应用验收串行使用同一显式 dev profile：A 为当前未修复基线的已记录 Wails 复现，
B 从修复 worktree 的 `desktop/` 运行 `wails dev`，确认并发 refresh 只发生一次、正常
display name 保持、当前 terminal session 显示登录失效、hydration 暂时失败不显示 user ID。
生产破坏性 A 已记录；B 不默认再次撤销真实账号，除非自动化与 dev A/B 通过后仍有必要。

## 9. Wails dev 真实窗口基线复现

### 9.1 运行条件

- 时间：`2026-08-21`，本机时区 `China Standard Time`（UTC+08:00）。
- 源码：独立 worktree `D:\Reasonix-auth-investigation`，产品源码相对
  `origin/main-v3@768d8fa2711c7fa0b48460677aa78037806fd053` 未修改；当前 HEAD 仅包含
  调查测试与文档提交。
- 应用：从该 worktree 的 `desktop/` 启动 `wails dev`，窗口版本标识为 `vdev`。
- cloud base URL：`http://127.0.0.1:19090/`；profile root 为一次性目录
  `%LOCALAPPDATA%\Temp\reasonix-auth-repro-20260821`，environment marker 记录 schema 1
  与该 loopback URL。
- 使用固定合成账户与合成 credential；日志只记录 token present、session state、
  generation、error code 和安全 request ID，不记录 credential 值、片段或摘要。
- 本机安装版 `clouvia-desktop.exe` PID 36616 与另一独立开发窗口保持运行且未操作；
  dev/prod singleton、profile root 和监听端口均不同。

### 9.2 可见基线 A

受控 localhost 服务在 `/me` 返回 `REFRESH_TOKEN_REUSED` 前，先原子写入同一
`DurableCloudSessionStore` 的 replacement session，等价触发
`persistFailureLocked` 的 `sameObservedSession=false` 路径。真实 Wails binding、
`CloudBootstrap`、Zustand 与 AccountCenter 的观察结果为：

| 观察点 | 结果 |
| --- | --- |
| durable session | authenticated，access/refresh present |
| binding | bound，generation 1 |
| `/me` operation error | `REFRESH_TOKEN_REUSED` |
| 侧栏 identity | `usr_auth_repro` |
| AccountCenter identity | `usr_auth_repro` |
| AccountCenter error | 普通“云请求失败” alert |
| 登录失效 UI | 不出现；无重新登录表单 |

重复打开账户中心后三次 `/me` 均保持同一组合：session authenticated、display name
缺失、cloud user ID present、terminal operation error present、termination skipped。
这在真实桌面窗口中复现了用户描述的“用户名退化为 userId，但没有登录失效提示”。

### 9.3 后续终止阶段

将同一受控服务切换为 replacement 请求返回 `AUTH_REQUIRED` 后，再从真实账户菜单
进入 AccountCenter。第四次 `/me` 的 observed/current session 相同，结果为：

- durable session 变为 `expired`；access/refresh present 均为 false；
- 侧栏文字立即变为“登录已失效”；
- AccountCenter 显示“登录状态已失效，请重新登录”与登录表单。

因此真实窗口证明了完整可见时间线：先出现 authenticated + userId + operation error，
后续当前 replacement 被拒绝时才变为 expired 和重新登录 UI。

### 9.4 证据边界与清理

Wails 窗口运行没有在两个真实 runtime executor 的 refresh HTTP handler 内注入 barrier；
为了不改产品代码，第一阶段由 localhost 服务在返回迟到错误前更新同一个 durable store。
两个真实 executor 共享 store 的确定性 refresh 竞态由已提交的
`desktop/cloud_api_real_auth_state_test.go` 与
`internal/cloud/auth_cross_executor_test.go` 单独证明。因此这是“真实 UI 呈现证据 +
真实 runtime 双 executor 自动化证据”的组合，不冒充同一次生产故障的 request_id 证据。

复现后已正常停止本 worktree 的 Wails 进程与 localhost 服务，确认 5175、34238、19090
无监听，并删除一次性 profile root 与临时 harness。主工作区、安装版 profile、真实账号
和 credential 均未读取、修改或清理。

## 10. 本机生产环境破坏性复现

用户于 `2026-08-21` 明确授权撤销当前真实 session family 并接受重新登录。测试前再次
确认安装进程路径为
`C:\Users\ai03\AppData\Local\Programs\云智企办\clouvia-desktop.exe`、版本 `1.2.1`，
cloud base URL 为 `https://www.suiyuan-ai.com/`，production durable binding 为
`default` / generation 6 / bound。测试期间停止安装进程，避免第三个 runtime 参与。

临时只读源码 harness 使用 production `DurableCloudSessionStore`、真实
`HTTPSessionClient` 和两个不同 `AuthenticatedRequestExecutor`。它仅把 access expiry
结构化改为本地已过期；access/refresh token 值保持不变且从未输出。channel barrier
保证两个 executor 在释放前都已读到同一个旧 refresh token，随后并发请求真实
`/api/v1/auth/refresh`。

第一阶段在 `2026-08-21T01:07:07+08:00` 得到同一次确定性证据：

| 字段 | 结果 |
| --- | --- |
| 两个 executor 观察同一 refresh | true |
| account executor | `REFRESH_TOKEN_REUSED`, request_id `req-1696420313b19cd9ca3b9a0224ab8e82` |
| project executor | success |
| durable session | authenticated，access/refresh present |
| replacement access expiry | `2026-08-21T02:07:31+08:00` |
| refresh expiry | `2026-09-18T11:14:36+08:00` |
| loser 终止是否跳过 | true；replacement 与 loser observed session 不同 |

第二阶段在 `2026-08-21T01:07:14+08:00` 用 durable replacement 请求真实 `/api/v1/me`，
服务端返回 `AUTH_REQUIRED`，request_id
`req-f93e84eb237e40d6700968ff9a493799`；executor 随即把 durable session 转为 expired，
access/refresh 均 scrub。重启同一安装版后，真实窗口左下角显示“登录已失效”。

这组运行证据把此前“源码可达风险”提升为已证明根因：生产服务的一次性 refresh
轮换语义与桌面端两个实例级 refresh 锁不共享组合后，确实会发生旧 token 重用、family
撤销、本地短暂 authenticated replacement、下一次受保护请求才 expired 的完整链。
临时 harness 源码与二进制已删除，独立 worktree clean；生产安装版保持运行，但该账号
已按授权退出，需要用户重新登录。日志未输出 token、device ID、手机号、Authorization
或完整 cloud user ID；仅保留 presence、expiry、状态、错误 code 和安全 request_id。

## 11. 修复版 Wails dev B 侧验收

### 11.1 等价运行条件

- 时间：`2026-08-21 12:24-12:35`，本机时区 `China Standard Time`（UTC+08:00）。
- 源码：独立 worktree `D:\Reasonix-auth-investigation`，修复版 HEAD
  `8a87445f042a20a60258994e7a21517651eaac8c`，已合并当时最新
  `origin/main-v3`；窗口版本标识为 `vdev`。
- 从修复 worktree 的 `desktop/` 启动 `wails dev`；为避开另一开发实例，只把本实例的
  browser binding 端口显式设为 `127.0.0.1:34116`，frontend 为
  `http://127.0.0.1:5174/`。
- cloud base URL 为 `http://127.0.0.1:19090/`，使用一次性合成 dev profile
  `%LOCALAPPDATA%\Temp\reasonix-auth-fix-b-20260821`、binding generation 1 和合成
  credential。生产安装版、生产 profile 与其他开发实例均未参与 B 侧请求。
- 临时桩与种子工具只输出 token presence、expiry、session state、generation、HTTP
  error code 和计数；未输出 credential 值、片段、摘要、device ID 或完整 cloud user ID。

### 11.2 共享刷新真实绑定验证

初始 bootstrap 成功显示合成名称 `Acceptance User`。随后把同一 durable session 的
access expiry 设为过去时间，桩服务用 barrier 阻塞第一次 refresh，并从同一真实 Wails
进程并发调用 `ListCloudProjects` 与 `CloudRefreshAssets`。两个调用都进入等待后释放
barrier，结果为：

| 观察点 | 结果 |
| --- | --- |
| 两个 Wails binding 调用 | 均 fulfilled |
| refresh HTTP 次数 | 1 |
| `/me` HTTP 次数 | 1 |
| project HTTP 次数 | 1 |
| refresh 结束后的侧栏 identity | `Acceptance User` |
| 登录失效 UI | 不出现 |

这证明修复版真实 runtime 已由唯一 executor 协调 account 与 project 两个入口；确定性
单元回归同时覆盖 replacement 成功、replacement terminal、fixed-scope binding switch、
post-load failure、expired/offline/unchanged session 和 observer 计数。

### 11.3 终止错误同步验证

在 replacement 仍为当前 session 时让 `/me` 返回 `AUTH_REQUIRED`，同一次
`CloudRefreshAssets` 的安全摘要为：`account.status=expired`、binding bound / generation 1、
display name absent、cloud user ID present、`operation_error.code=AUTH_REQUIRED`。重新加载
真实 frontend 后：

- 侧栏显示“登录已失效”；
- 账户菜单显示“登录状态已失效 / 点击登录、注册”；
- 不再存在 authenticated + internal user ID + terminal operation error 的混合可见状态。

### 11.4 可恢复 hydration 失败与保留状态

正常停止 B 进程并确认本实例端口释放后，以相同 profile 重新写入 authenticated 合成
session，再让 `/me` 返回 `NETWORK_UNAVAILABLE`。最终 bootstrap 安全摘要为：
`account.status=offline`、binding bound / generation 1、display name absent、cloud user ID
present、`operation_error.code=NETWORK_UNAVAILABLE`。真实 frontend 的三个观察点一致：

- sidebar 显示“账号名称暂不可用”；
- account menu 显示“账号名称暂不可用 / 账户”；
- AccountCenter 标题显示“账号名称暂不可用”，同时呈现可重试网络错误，没有显示内部
  `usr_*`，也没有误报登录失效。

修复版 Wails dev、loopback 桩和合成 profile 保持运行，供手工验收；临时 harness 源码已
从 worktree 删除。该保留状态不含真实账户 credential，不改变安装版仍需重新登录的事实。
