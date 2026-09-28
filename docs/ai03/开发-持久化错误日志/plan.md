# Desktop 持久化错误日志开发计划

## 1. 背景与基线证据

用户在 `2026-08-25 10:45` 前观察到开发版 Desktop 显示“登录已失效”，随后手动重新登录。
调查确认重新登录在当前开发 profile 中原子覆盖了旧 `sessions.json`；旧会话只留下了被终止后的
业务状态，没有保留导致终止的错误码、`request_id`、请求阶段或客户端版本，因此无法区分
`AUTH_REQUIRED`、refresh token 拒绝、设备不匹配或服务端会话丢失。

实施基线为
`origin/main-v3@7499bc985db3f81f3ddf57133a0439798c569e16`，独立 worktree 为
`D:\Reasonix-worktrees\persistent-error-journal`。主工作区的未跟踪文件与本开发无关，本任务不
修改主工作区。

最新基线的可观察性现状为：

- `desktop/frontend/src/lib/clientError.ts:reportClientError` 已完成错误码提取、500 字符摘要脱敏、
  同对象及 2 秒 fingerprint 去重，但默认 sink 只是 `console.error`，进程退出后消失；
- `desktop/frontend/src/lib/breadcrumbs.ts` 只保留内存中的最近 30 条 breadcrumb；
- `desktop/frontend/src/lib/crash.ts:reportCrash` 和 `ErrorBoundary.componentDidCatch` 能构造前端栈与
  崩溃覆盖层，但不自动持久化；
- `desktop/crash_pending.go` 只为被 `goSafe` 捕获的 Go panic 保存一个待上报文件，不是普通失败
  日志；
- `desktop/main.go` 没有安装文件 `slog.Handler`，默认 `slog.Warn/Error` 只进入当前进程输出；
- `internal/cloud/auth_identity_lifecycle.go:persistFailureLocked` 与
  `internal/cloud/auth_application.go:persistFailure` 会把 terminal/retryable 认证失败持久化为
  `expired/offline`，但在覆盖会话状态前没有另存安全错误证据；
- 已有远端 `App.ReportCrash` 与意见反馈接口要求用户显式提交。本计划不改变其授权和上传语义。

这不是单个登录分支的日志缺失，而是 Desktop 的前端、Go 与认证状态机没有共同的本地失败事件
落点。

## 2. 已确认产品边界

用户已经确认：

- 建设 Desktop 统一失败事件日志底座，不做全量运行日志；
- 开发版与正式版均默认启用，不提供设置、开关、状态页、手动清理或导出 UI；
- 日志严格留在本机，不自动上传，也不自动附加到意见反馈或崩溃服务；
- Windows 开发环境写入
  `%LOCALAPPDATA%\kwu-dev\environments\<environment_id>\cache\diagnostics\errors\`，正式环境写入
  `%LOCALAPPDATA%\Clouvia\diagnostics\errors\`；其他平台沿用现有 profile `CacheDir()`；
- 单文件上限 `2 MiB`，所有日志总量上限 `10 MiB`，历史日志最多保留 `7 天`；
- 启动、写入前的节流检查和轮转时自动清理；清理或写入失败不得阻塞应用启动、登录或其他业务；
- 记录前端结构化失败、React/全局未处理异常、Go `ERROR`、明确标记的失败型 `WARN`、认证
  terminal/offline/refresh 失败；不记录成功请求、用户点击、页面切换、用户内容或普通性能轨迹；
- 保存稳定错误分类和仅含枚举/数值的调用栈元数据，但不保存任意 `Error.message`、服务端 message 或 stack 首行；
  所有字段仍必须脱敏、截断，并禁止 token、密码、Authorization、请求/响应正文、
  用户对话、文件内容、环境变量值、完整用户目录、device ID 和完整 cloud user ID。

## 3. 目标与非目标

### 目标

1. 建立一个 Desktop 本地 JSONL failure journal，跨应用重启、重新登录和账号切换保留证据。
2. 用 profile cache 隔离开发后端与正式环境，并以轮转、过期和总容量三层边界阻止无限增长。
3. 让 Go、前端和 cloud auth 只提交结构化白名单字段，由 journal 在最终落盘前统一再次脱敏。
4. 认证状态转为 `expired/offline` 前把安全原因、阶段、`request_id` 和前后状态摘要非阻塞放入
   有界队列，由单 writer 立即持久化；正常重新登录不得删除该证据。
5. 对短时间重复失败限流/聚合，避免故障循环快速消耗容量。
6. journal 自身在目录不可写、磁盘不足、文件损坏、清理失败或关闭竞态下保持 best-effort，不改变
   任何业务返回值或状态机分支。

### 非目标

- 不记录 INFO/DEBUG、正常启动、成功请求、用户操作轨迹、prompt、模型输出、工具参数或业务正文。
- 不把现有所有 `slog.Warn` 或任意 `error.Error()` 原样写入文件。
- 不新增设置页、通知、日志查看器、手动清理、导出、远端上报或服务端 API。
- 不修改现有 crash/feedback/telemetry 的用户授权、网络行为或持久化数据 schema。
- 不引入 SQLite、第三方日志轮转依赖、系统服务、注册表项或数据迁移。
- 不承诺捕获进程被强杀、系统断电、OS/WebView 原生崩溃之前尚未到达 journal 的事件。
- 不扩展到 CLI 或服务端进程的持久化日志；`internal/cloud` 新增的安全失败事件仍可沿现有
  `slog` 输出被其他宿主观察，但只有 Desktop 安装文件 handler。

## 4. 验收标准

| ID | 场景 | 预期 |
| --- | --- | --- |
| AC-01 | 开发版和正式版启动 | 无需配置即在各自 profile cache 的 `diagnostics/errors` 使用独立 journal |
| AC-02 | 普通前端业务失败 | `reportClientError` 的 allowlisted scope/code/error type 等稳定字段 best-effort 进入 JSONL；未知值归为固定 `unknown`，任意 message 不落盘，既有用户提示与 console 行为不变 |
| AC-03 | React、`window.error`、`unhandledrejection` | 每次故障只提交一条；frame 只保存 allowlisted category、ordinal、line/column 数值，不保存 message、函数名、文件名或 basename，崩溃覆盖层行为不变 |
| AC-04 | Go `ERROR` 或显式失败型 `WARN` | 既有 stderr handler 继续收到记录，journal 只保存允许字段，不保存任意 attr/raw error |
| AC-05 | terminal 认证失败 | 会话被 scrub/写为 expired 前先非阻塞入队；最终日志含 phase、code、request_id、HTTP status、retryable、前后状态、token presence 与过期时间，但无 token/device/user ID |
| AC-06 | retryable/network 认证失败 | 写入队列中的目标状态为 offline 并保留 credential；不得误记为 expired |
| AC-07 | 手动重新登录或重启 | 先前 failure journal 不随 auth/session cleanup 删除，仍可读取完整 JSON 行 |
| AC-08 | 重复错误 | journal 按脱敏稳定字段在 30 秒内抑制相同 fingerprint；窗口结束后同一 Error 对象再次发生也能保留下一条 |
| AC-09 | 文件超过 2 MiB | 写入前轮转；任一文件不超过 2 MiB（允许文件系统 metadata 误差，不允许单条突破） |
| AC-10 | 历史超过 7 天或总量将超过 10 MiB | 删除最旧非活动文件；若无法恢复预算则丢弃本次日志，不突破容量上限 |
| AC-11 | 队列满、并发关闭、目录/锁/文件/清理/同步失败 | 丢弃 journal 事件并保留原 stderr/console；无 panic，应用启动、认证锁和业务调用返回不等待磁盘 I/O，Close 最多等待 500 ms |
| AC-12 | 隐私样本 | 任意 message/body canary、token、Bearer、密码、手机号、邮箱、完整路径及路径尾段、长密钥/JWT 和 cloud identity 不出现在落盘字节中；目录/文件使用私有权限 |
| AC-13 | 范围 | 无 UI、配置、网络上报、module 依赖、持久数据迁移或 `code_guard_baseline.json` 变化 |

## 5. 选定设计

### 5.1 Desktop-owned journal

在 Desktop nested module 内新增小型 journal 包，使用标准库与仓库已有 `x/sys` 平台锁完成 JSONL 编码、互斥写入、同步、
轮转和清理。journal 由 Desktop 启动组合根创建；它不进入 root CLI 的默认路径，也不成为 cloud
业务状态的依赖。

启动组合在 dev profile 已解析并设置 `CLOUVIA/KWU_*_HOME` 之后、首次迁移 warning 与 `NewApp`
之前，只同步计算 journal 路径、创建内存队列并启动 writer goroutine：

- dev 使用 `desktopProfile.CacheRoot` 和 16 位 environment ID；
- production 使用 `config.CacheDir()` 和固定环境标签 `production`；
- `mkdir`、跨进程锁、目录扫描、清理、open/stat/write/sync/close 全部由 writer 异步执行；任何文件系统
  阻塞或失败都不得阻塞 `bootstrapDesktopLaunch`，迁移 warning 可先入队；
- 根路径不可解析时安装丢弃型 journal 并保留原默认 logger，继续启动；
- shutdown best-effort flush/close，关闭失败不覆盖 Wails 的原退出行为。

目录使用 `0700`、文件使用 `0600`（Windows 按平台能力 best-effort）。所有进程共享一个 current
JSONL；每次 cleanup、容量预留、轮转、append 和失败回滚都在 journal 专属、短时跨进程 advisory
file lock 内串行执行。锁使用已有 `x/sys` 平台能力、非阻塞重试且最多等待 `250 ms`；拿不到锁就
丢弃事件。取得锁后必须按路径重新 open/stat current；append/sync/rollback 后必须在解锁前 close，
禁止跨 lock 生命周期缓存文件 descriptor。该锁只保护 journal 目录，不复用 cloud 的进程内 durable
JSON 锁，也不参与业务状态。

### 5.2 容量与保留算法

- writer queue 固定为 256 条；enqueue 永不阻塞。Close 先原子切换为 discard，后续 enqueue 永远 no-op；
  事件 channel 永不 close，writer 由独立 stop signal 触发并最多等待 `500 ms` drain，超时即放弃。Close
  幂等，并发 enqueue/Close 不得 panic；
- JSONL 单条上限 `16 KiB`；scope/event/error type 各 `128 bytes`、code `80 bytes`、request ID
  `128 bytes`，frame 最多 `64` 条且每条只含 enum category、ordinal、line/column 数值；超限先删可选
  frames，仍超限则丢弃事件；
- current 文件在 `existing_size + event_size > 2 MiB` 或跨 UTC 日时轮转，确保长期低流量进程也能执行
  7 天清理；
- writer goroutine 启动后立即异步 housekeeping，之后即使没有新错误也至多每小时重试一次；constructor/
  `bootstrapDesktopLaunch` 不等待结果。首次事件写入前仍做容量检查，跨日和容量轮转时也扫描目录；
- 先删除修改时间早于 7 天的非活动文件，再按修改时间从旧到新删除，直到为本次写入保留空间且
  总量不超过 10 MiB；
- 所有容量判断与 append 在同一跨进程锁内；若无法腾出预算，本次记录被丢弃，因此并发进程也不能
  共同突破 10 MiB；archive 路径用 `O_EXCL` 预留并最多重试 16 次，避免 PID/时间碰撞；
- 每次 open existing current 后、append 前在锁内检查末尾换行；若存在 partial tail，则 truncate 到最后
  一个完整换行（无完整行则清空）。修复失败时关闭并以碰撞安全名称移入同目录 `.corrupt` quarantine；
  quarantine 计入 10 MiB 总量但不作为 JSONL 读取，随后创建新 current；
- append 前记录 offset；事件与换行编码成一个 byte slice。短写/失败时 truncate 回该 offset，rollback
  失败同样隔离 current，保证先前完整行与后续文件都可逐行解析；
- journal 内部错误不再写 journal，避免递归；原 stderr/console sink 保留。

### 5.3 结构化白名单与最终脱敏

统一事件 schema 只允许：

- `schema_version`、UTC `occurred_at`、`severity`、backend 固定的 `source/event`、allowlisted `scope`；
- 按来源 allowlist 的 `code`、受限 `request_id`、`http_status`、`retryable`；
- allowlisted `error_type`、只含 enum category/ordinal/line/column 的 `stack_frames`、
  `component_frames`；stack 第一行、函数名、文件名/basename 和任意 message 永不进入 DTO；
- auth 专用 `phase`、`state_before`、`state_after`、access/refresh presence 与 UTC expiry；
- app `version`、`channel`、build revision（可获取时）、environment ID、OS/arch 和 PID。

不提供任意 `map[string]any` 透传。普通 renderer/server `Error.message` 不属于可持久化输入；backend
忽略 renderer 提交的 source/event，并把 scope 映射到已知 scope allowlist、未知值映射到
`frontend.unknown`；frontend code 只接受仓库当前固定错误码表（`ERROR_CODE_MESSAGES` 与固定
`knownMessages` maps）的并集，其他合法格式字符串也映射到 `UNKNOWN`；error type 只接受内建分类
allowlist，未知值映射到 `Error`。frontend 在内存中
解析 raw stack 后只提交 frame category（`app/framework/runtime/unknown`）、ordinal、line/column；Go
再次验证 enum 和非负数值，函数名、文件名/basename 与其他原文不跨 Wails boundary。journal 对剩余
字符串再次应用 secret、JWT、Bearer、邮箱、手机号、绝对路径（包括路径尾段）和长 token 脱敏规则；
现有 `crash_app.go` 缺少手机号/完整路径清除能力，必须在提取公共 helper 时补齐，并用最终文件字节
断言验证，而不是声称旧规则已覆盖。
`cloud.Error.Details`、请求/响应 body、scope/profile/user/device 原值永不进入事件 DTO。
auth code 使用 auth lifecycle 当前识别的 typed code allowlist，未知服务端 code 映射为
`UNKNOWN_CLOUD_ERROR`；frontend 与 auth 不共享一个接受任意字符串的 code 通道。

### 5.4 Go 与 `slog` 接入

安装一个 fan-out `slog.Handler`：原 handler 永远先/同时保留，journal 分支只接受：

- `slog.LevelError`，只投影 severity、固定 callsite category/ID 和固定 `go.error` event，不读取或
  持久化 `slog.Source.File`、message/attrs；
- `slog.LevelWarn` 且带固定 failure marker 的记录；
- cloud auth 使用稳定事件名和安全字段，不传 raw `err`。

现有未标记 warning、独立 `slog.New(...)` logger、INFO/DEBUG 不进入 journal。本切片只给直接可达且
明确表示失败的 Desktop startup/shutdown/persistence warning 加 marker；不机械修改全部 32 个
`slog` 调用点。

`goSafe` 捕获的 panic 在写 `crash-pending.json` 的同时提交一条本地 crash failure；两条路径均
best-effort，现有 re-panic 行为不变。

### 5.5 认证状态转换证据

在两个真实持久化转换点提交安全事件：

- `AuthApplication.persistFailure`：显式 refresh lifecycle 失败；
- `authenticatedRequestExecutor.persistFailureLocked`：受保护请求、自动 refresh/replay 失败。

事件必须在覆盖 durable session 前使用非阻塞 enqueue 交给 journal writer，字段只从 typed
`cloud.Error` 与
session metadata 构造：phase、terminal/retryable、code、request ID、HTTP status、before/target
state、token presence 和 expiry。terminal 事件不得在 `terminateSession` 后才读取 presence/expiry；
stale observed session 被 guard 拒绝覆盖时记录稳定 outcome，不误称已 terminalize。session save
失败另发不含 raw error 的 persistence failure 事件。

enqueue 在 auth mutex 内只做固定字段复制和有界 channel send，不执行 file lock、cleanup、write 或
sync。队列满直接丢弃，业务继续；因此进程在入队后、writer 落盘前崩溃或断电时允许丢失最后事件，
这是用户明确批准的非阻塞取舍。该方案不增加第二套 auth 状态机；其他宿主只可能在既有 stderr
看到同一份安全失败分类。

### 5.6 前端接入

- 为 `AppBindings` 增加一个 best-effort `RecordClientError` Wails 方法，参数是明确 DTO，不使用
  `ReportCrash`，不触发网络；
- `reportClientError` 继续生成现有 console log 和 presentation；local sink 提交位于现有永久
  object filter 之外，由 Go journal 的 30 秒稳定 fingerprint 去重，无 Wails binding 或 bridge
  reject 时静默降级；
- `reportCrash` 先构造一次持久化用的枚举/数值 crash fields，再通过一次 `reportClientError` 调用完成 console、
  local sink 和 presentation，随后 paint 同一 payload；不得再直接第二次提交。普通业务 failure
  不附加 frames；每个 React/window/rejection 最终只产生一条 JSONL；
- Go binding 对所有前端字段重新校验、截断和脱敏。frontend 的脱敏只改善 console 与测试，不是
  安全边界。

## 6. 被拒绝的替代方案

| 方案 | 拒绝原因 |
| --- | --- |
| 只补 auth 日志 | 无法解决前端、Go 和后续其他 bug 的证据丢失，违背统一底座目标 |
| 捕获全部 stdout/stderr/console | 含正常轨迹、路径和任意文本，隐私与容量不可控 |
| 把所有 `slog.Warn` 原样落盘 | 当前 warning 含 `path/session_path/err` 等任意字段，违反白名单合同 |
| 复用 `ReportCrash` 或 telemetry | 会引入隐式网络上传和用户授权变化 |
| 使用 session/state Roaming 目录 | 日志会参与用户数据漫游或备份；cache 更符合七天可丢弃数据语义 |
| 无跨进程锁地共写 `errors.jsonl` | dev worktree/startup race 会造成竞争或突破容量；选定方案用短时 advisory lock 串行维护 |
| SQLite 或第三方 rolling logger | 当前只需顺序 append、按文件清理；新增依赖/数据库超过需求 |
| 只依赖 frontend 脱敏 | Go 是最终持久化边界，必须 distrust renderer 输入 |
| 只按总量、不按时间清理 | 低错误量会无限期保留陈旧敏感摘要 |
| 清理失败仍继续写 | 会突破用户确认的硬容量边界 |

## 7. 影响文件与合同

预计代码范围：

- `desktop/internal/diagnostics/`：journal、事件 schema、slog fan-out、轮转/清理/去重及测试；
- `desktop/error_journal.go`：Desktop profile 组合、Wails DTO/binding、panic/selected warning 映射；
- `desktop/main.go`、`desktop/crash_pending.go` 和少量直接失败 warning 所在文件：初始化、关闭与安全
  failure marker；
- `internal/cloud/auth_application.go`、`internal/cloud/auth_identity_lifecycle.go` 及相邻测试：安全 auth
  transition 事件；
- `desktop/frontend/src/lib/clientError.ts`、`crash.ts`、`bridge.ts`、`main.tsx` 及相邻测试：本地 sink；
- 本主题任务回执。

新增的是本地诊断 JSONL schema 和一个 Desktop 内部 Wails method。它不是服务端/public API，不被
其他应用消费；仍需用确定性测试锁定，避免后续误加敏感字段。无 config、cloud wire、session JSON、
database、migration、module 或 dependency 变化。

## 8. 风险、控制与回滚

| 风险 | 控制 |
| --- | --- |
| 敏感值经 raw error/stack/slog attr 泄漏 | backend 固定 source/event、scope/type allowlist、frame 无原文、禁止 arbitrary attrs/details、Go 最终脱敏、合成 canary 字节断言 |
| journal handler 递归或吞掉原日志 | 保存并 fan-out 原 handler；handler 只 enqueue，writer 内部不得调用 `slog`；测试 delegate 输出 |
| 全局 `slog.SetDefault` 污染测试 | 安装/恢复 helper，测试串行并 cleanup；生产只安装一次 |
| 多 goroutine/多进程竞争 | 单 writer + 跨进程 advisory lock 覆盖 open/stat/repair/cleanup/reserve/rotate/append/rollback/close；descriptor 不跨锁；250 ms 超时丢弃 |
| 进程终止遗留 partial tail | 下次持锁 open 后先 truncate 到最后完整换行；修复失败隔离为计入预算的 `.corrupt` 文件 |
| 启动文件系统阻塞 | 启动只计算路径、建内存队列和 goroutine；所有文件系统操作在 writer 异步执行 |
| shutdown 与全局 logger 并发发送 | Close 原子切 discard，事件 channel 永不 close，独立 stop signal drain；并发 enqueue 只 no-op/非阻塞 send |
| 启动后无错误或长期低流量不触发 7 天清理 | writer 启动后异步 housekeeping，并用独立小时 ticker 重试；不依赖错误事件 |
| 大栈或故障循环写满磁盘 | 单字段/单条上限、fingerprint 去重、2/10 MiB 硬限制 |
| 前端 sink 失败制造新错误 | fire-and-forget catch 且不调用 `reportClientError` 回报 journal 自身错误 |
| auth 日志改变状态机时序 | 持久化分支前只做非阻塞 enqueue；磁盘 I/O 不在 identity/refresh mutex 内；原 cause/save 顺序与返回值保持 |
| cache 被用户或 OS 清理 | cache 是明确可丢弃诊断数据；七天内尽力持久，不提升为业务状态 |

回滚按两个代码提交逆序完成：先移除 frontend bridge，再移除 auth/slog/journal 初始化。无需转换或
清理业务数据；遗留 `diagnostics/errors` 文件会在七天后由旧版本之外的 OS/用户缓存策略处理，或可
安全手工删除。回滚不恢复已经缺失的历史错误证据。

## 9. 验证计划

### 9.1 自动化

Root Go（直接修改 `internal/cloud`）：

```powershell
go test ./internal/cloud -count=1
```

Desktop nested module：

```powershell
cd desktop
go test . ./internal/diagnostics -count=1 -timeout 240s
```

聚焦 frontend 回归与生产 build：

```powershell
cd desktop/frontend
pnpm exec tsx src/__tests__/client-error.test.ts
pnpm exec tsx src/__tests__/crash-reporting.test.ts
pnpm build
```

测试必须覆盖 JSONL 可解析性、两个 helper 进程交替 append/轮转、descriptor 生命周期、2 MiB 轮转、
10 MiB/7 天清理、跨日轮转、无法清理时不突破预算、预置 partial tail 的启动修复、短写 rollback、
30 秒/1024 fingerprint 边界、预置过期文件且不 enqueue 时异步启动清理、阻塞文件系统下 constructor/
bootstrap 不等待、并发 enqueue/Close 无 panic且 500 ms 返回、delegate 保留、frontend sink 降级，
以及合法格式但不在 allowlist 的 code 与恶意
scope/error type/function/basename/message/body/路径尾段 canary 在最终文件字节中完全不存在。

认证测试用受控 session store/client 精确覆盖 terminal、offline、refresh、stale observed、save
failure；断言 enqueue 发生在 session save 前、磁盘 writer 不持有 auth mutex，重新登录/cleanup 后
日志文件仍存在。

### 9.2 真实 Desktop 路径

- A：保留本次 `2026-08-25` 真实 incident 作为 baseline evidence：重新登录覆盖了旧 session，当前
  基线没有 journal，无法恢复 code/request ID。不得声称已重新制造同一次服务端错误。
- B：从 patched worktree 的 `desktop/`、使用与 `D:\Reasonix` 相同 dev 服务/profile/data 配置
  串行运行 `wails dev`。创建临时 workspace 与一个临时文本文件，在 Workspace 文件预览列表出现后
  从文件系统删除该文件，再在真实 UI 点击该条目，走既有
  `reportClientError("workspace.preview", ...)`。预期既有预览失败 UI 不变，journal 新增唯一
  `source=frontend/scope=workspace.preview/event=client.failure` 且不含临时根、文件名或原始 message。
  正常关闭并重启后确认该 JSONL 行仍存在，再从应用移除临时 workspace 并删除临时目录。
- auth terminal 的破坏性真实账号测试默认不执行；由受控 HTTP/runtime integration 覆盖。只有另获
  明确授权并能等价恢复初始状态时才可改变真实 session。
- A/B 不并行运行；切换前正常关闭进程并确认 singleton/端口释放。B 最终通过后保留 patched
  `wails dev` 供用户手工验收。

### 9.3 仓库门禁

最终按 `AGENTS.md` 运行：

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code origin/main-v3...HEAD -- scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

使用 `git diff --name-only -z origin/main-v3...HEAD` 的完整 NUL path 集合核对 gate classification。

## 10. 计划挑战结果

- 统一底座不等于全量日志：只允许显式 failure event 和严格筛选的 native records。
- profile cache、七天与 10 MiB 是用户已确认的数据生命周期；设置/上传/导出均无验收依据。
- auth 最小扩展点是两个既有 session transition 分支，不需要记录正常请求或改造所有 HTTP client。
- frontend 已有统一 error funnel，增加 local sink 比逐页面接线更小；global crash 只需补 stack DTO。
- 共享 current 文件配合短时跨进程锁，是维持 10 MiB 硬预算所需的最小协调；每次锁内重新
  open/stat/repair 并在解锁前 close，避免跨进程轮转继续写旧 descriptor；不新增常驻协调服务。
- 一个任务会同时横跨 journal 算法、root cloud、desktop Go 和 frontend，难以独立审阅；拆成“Go/auth
  底座”和“frontend 接入”两个串行行为任务。并行 writer 无收益且会重叠 Desktop journal 文件。
- 本地持久化错误内容属于安全/隐私表面，计划与任务合同完成后必须进行一次高风险只读审查。
- 高风险审查发现并已修正：任意 message/stack 首行不可持久化、旧 sanitizer 不覆盖手机号与路径
  尾段、硬容量需要跨进程锁、auth mutex 内不能做磁盘 I/O、crash 必须单路径提交、frontend 永久
  object dedupe 不能控制 journal、短写必须 rollback，以及真实 Desktop 验收必须给出可执行路径。
- 二次审查进一步收紧：renderer 自由 scope/type/frame 原文必须在 Go 边界枚举化；current descriptor
  不得跨锁；已有 partial tail 必须先修复或隔离；启动组合不得同步执行任何 journal 文件系统操作。
- 三次审查进一步收紧：frontend/auth code 分别使用现有语义 allowlist，未知 code 固定化；Close 先
  原子进入 discard、事件 channel 永不关闭，避免全局 logger 与 shutdown 并发发送 panic。
- 四次审查修复异步初始化带来的保留期回归：writer 启动后自行做 housekeeping 并按小时重试，不依赖
  首条失败事件，同时 constructor 和 bootstrap 仍不等待文件系统。
- 用户明确批准 auth failure 在 session save 前非阻塞入队、后台立即写盘；接受进程在极短窗口内
  崩溃时最后事件可能丢失，以换取磁盘故障不阻塞认证和业务。

## 11. 阶段门

用户已确认所有产品边界与验收标准。`plan.md` 与任务合同分别提交，并完成高风险只读审查后，向
用户展示最终计划与任务映射并取得一次明确实施批准。批准后使用 fresh `gpt-5.6-luna`、reasoning
`max`、context fork `none` 的实现子代理串行执行；主 Agent 负责提交/diff/回执审阅、集成验证与
真实应用验收。
