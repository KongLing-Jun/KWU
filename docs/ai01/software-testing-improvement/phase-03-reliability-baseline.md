# Phase 3 可靠性、性能与长稳基线

> 执行日期：2026-07-23
> 源码提交：`15a98177251bd024b499206e9d90b979fbc48436`
> 限制：按用户要求，Root Go/race/coverage 不再执行第 2、3 轮；本页不计算
> P50/P95，也不把短时采样写成 8 小时长稳。

## 1. Race 责任审计

| 层 | 当前事实 | 缺口 |
|---|---|---|
| PR | `ci.yml` 仅在 `root_go_mode == full` 时执行 `go test -race ./...` | 定向并发 owner 变化没有独立 race 映射 |
| Nightly | Root/Desktop 只执行 `go vet` 和普通 `go test` | 与计划中的 nightly full root race 不一致 |
| Desktop | PR 和 nightly 都运行普通 Desktop Go gate | 没有冻结可稳定执行的 Desktop race 包 |
| Release | Draft/Publish workflow 未发现 `go test -race` | 发布候选缺少目标 Windows 架构的 race 替代证据 |

### 未关闭问题 REL-TEST-001

- 严重度：Medium。
- 影响：full root race 依赖“广泛变更”触发，nightly 与发布候选不会补跑；
  Desktop 并发路径也没有明确 race 责任。
- 建议：先在 nightly 增加 full root race；再通过独立试跑冻结 Desktop
  可稳定包集合。CI 成本和 Windows/CGO 限制需单独评审，本任务未修改 workflow。

本轮已有的 full root race 单轮结果为通过，耗时 27 分 26 秒；该结果只证明
本次实际执行到的路径，不能替代 nightly/release 责任。

## 2. 资源 owner 清单

当前 `goleak.VerifyTestMain` 覆盖 5 个 package：
`internal/agent`、`internal/control`、`internal/jobs`、`internal/lsp` 和
`internal/plugin`。

| Owner | 主要资源 | 创建点/入口 | 回收路径 | 当前证据与缺口 |
|---|---|---|---|---|
| `jobs.Manager` | job goroutine、文件和子进程 | `internal/jobs/jobs.go` | `Close`、`CloseWithGrace`、`WaitTeardown` | goleak；有非协作 job 超时测试 |
| `plugin.Host` | MCP transport、子进程、后台 writer | `internal/plugin/plugin.go`、`transport_stdio.go` | `Host.Close` 取消、等待并关闭 client | goleak；关闭顺序明确 |
| `lsp.Manager` | language server 子进程和 stdio | `internal/lsp/client.go` | `Manager.Close`、client `cmd.Wait` | goleak；需补 Windows handle 趋势 |
| `control.Controller` | autosave、ticker、tool 子进程、jobs/plugin | `internal/control/controller.go` | `Close`、`CloseAfterDestroy` | goleak；异步 destroy 已有显式模式 |
| `agent` turn | timeout、parallel task goroutine | `internal/agent` | operation wait group/context | goleak；仅覆盖执行到的调度 |
| `desktop.App` | cloud caller、billing worker/SQLite、automation、heartbeat、bot、tray、shared host、tab controller | `desktop/app.go` | `App.shutdown` 按 owner 顺序关闭 | 无 Desktop 全局 goleak；需进程级趋势 |
| WebView2 | 原生窗口、WebView2 子进程、user-data files | Wails runtime | Wails shutdown/进程退出 | 本轮只读原生 smoke；CDP 仍阻塞 |

### 未关闭问题 REL-TEST-002

- 严重度：Medium。
- 影响：Desktop 的 handles、threads、WebView2 子进程和 SQLite 文件锁没有
  长时阈值；包级 goleak 不能覆盖这些 OS 资源。
- 当前措施：新增 `scripts/testing-process-soak.ps1`，按精确 PID/可执行文件
  只读采样 working set、private bytes、handles、threads、后代进程和 CPU。

10 秒脚本 smoke 对现有桌面进程执行成功：

```text
samples: 4
terminatedEarly: false
peak working set: 117428224 bytes
peak private bytes: 89714688 bytes
peak handles: 1221
peak threads: 40
peak descendant processes: 6
```

这只证明采集器可工作，不是稳定性结论，也不用于设阈值。

采集器归档格式已升级为 schema v2：运行前仍用绝对路径核验 PID 与目标
可执行文件，但 `report.json` 不再保存绝对路径，只记录文件名、仓库相对路径
（目标在仓库内时）和 SHA-256。新增
`scripts/testing-process-soak-test.ps1`，用隐藏的临时 PowerShell 进程验证：

- PID/可执行文件不匹配必须失败；
- JSON/CSV schema、样本数量、来源 SHA 和可执行文件 SHA 一致；
- 外部 fixture 不归档主机绝对路径、命令行、用户名或用户目录字段；
- fixture 结束后清理精确的测试进程和仓库内临时输出目录。

对当前集成分支桌面进程另执行 3 秒真实 schema smoke，得到 2 个样本、
`terminatedEarly=false`，归档路径为
`desktop/build/bin/kwu-desktop-dev.exe`，未出现工作台绝对路径。该结果只证明
采集与脱敏契约，不是 8 小时长稳结果。

## 3. 原生 fuzz

新增三个 `_test.go` fuzz target：

| Target | 风险边界 | Seeds/不变量 |
|---|---|---|
| `FuzzParseTreeJSON` | Planner TaskTree JSON、wrapper、截断内容 | 合法对象、fenced wrapper、截断、空输入；确定性、非空成功结果、可重新编码 |
| `FuzzDecodeCloudResponse` | 云响应 envelope、状态码、错误与 data | 200/400/500、错误 envelope、null、截断；确定性、非 200 不得成功、成功 data 非空 |
| `FuzzParseRelationships` | Office relationship XML | 合法、空属性、截断、空输入；确定性、数量有界、空 ID/target 不得逸出 |

Seed 回归：

```text
go test ./internal/tasktree ./internal/cloud ./internal/fileextract/office -count=1
PASS
```

逐个固定 50 次输入执行通过。最初并行执行三个 fuzz worker 超过 60 秒外层限时，
该次被记录为超时而不是通过；随后改为逐个执行，没有修改产品超时或断言。

## 4. 单轮 benchmark 基线

环境：

```text
OS: Windows 11 专业版 10.0.26200 build 26200
CPU: AMD EPYC 7B12, 16 cores / 16 logical processors
RAM: 48 GiB
Go: go1.26.4 windows/amd64
SHA: 15a98177251bd024b499206e9d90b979fbc48436
```

代表性单轮结果：

| Benchmark | 结果 | 分配 |
|---|---:|---:|
| `BenchmarkPlannerToolRegistry` | 736014 ns/op | 280372 B/op, 4302 allocs/op |
| `BenchmarkDesktopSettingsPayloads/Settings` | 326050 ns/op | 30595 B/op, 253 allocs/op |
| `BenchmarkDesktopStartupSettings` | 284326 ns/op | 21341 B/op, 185 allocs/op |
| `BenchmarkDesktopListSessionsScoped` | 2088164500 ns/op | 1961816 B/op, 11717 allocs/op |
| `BenchmarkDesktopSessionDir` | 11165 ns/op | 4128 B/op, 37 allocs/op |
| `BenchmarkHistoryMessagesSynthetic/1000x10KB-bash-success` | 8504956 ns/op | 2810664 B/op, 37052 allocs/op |

`DesktopListSessionsScoped` 的单次约 2.09 秒值得作为后续会话恢复优化观察点，
但当前只有一个样本，不能判定回归。现有 benchmark 没有覆盖真实新 profile
到首屏的冷启动，也没有 benchstat 多样本置信区间，因此 T3.4 尚未完成。

补充执行了一轮 `-count=5 -benchtime=1x` 定向采样；这是现有 benchmark 的
五个独立进程内样本，不是 Root Go/coverage 重跑，也不等同于 benchstat
基线。代表性中位数与范围如下：

| Benchmark | 中位数 | 最小值–最大值 |
|---|---:|---:|
| `BenchmarkPlannerToolRegistry` | 1.840 ms | 1.703–2.077 ms |
| `BenchmarkDesktopSettingsPayloads/Settings` | 0.451 ms | 0.375–0.490 ms |
| `BenchmarkDesktopStartupSettings` | 0.365 ms | 0.295–0.469 ms |
| `BenchmarkDesktopListSessionsScoped` | 568.849 ms | 489.408–1468.513 ms |
| `BenchmarkHistoryMessagesSynthetic/1000x10KB-bash-success` | 6.989 ms | 6.870–7.428 ms |
| `BenchmarkDesktopSessionDir` | 1.165 ms | 0.994–1.541 ms |

`DesktopListSessionsScoped` 的首样本明显高于其余四个样本，当前只能记录为
待复核的冷缓存/初始化候选，不能归因为产品回归。环境未安装 `benchstat`，
且现有 `DesktopStartupSettings` 只测设置负载，不是 Wails/WebView2 真实冷启动，
所以 T3.4 仍保持未完成。

## 5. 8/24/72 小时触发条件

- Nightly 8h：agent/jobs/plugin/LSP/control 生命周期、云事件循环、SQLite、
  updater 或 WebView2 依赖发生变更时；使用 fake/local fixture，不访问生产。
- Release 24h：每个 Windows 发布候选，或启动、安装、升级、休眠恢复、网络
  重连路径发生变化时；必须使用目标架构和候选制品。
- 重点 72h：重大版本、曾出现资源增长/崩溃，或客户承诺涉及 EDR、代理、
  自定义 CA、目标硬件时；需要客户环境 owner 与脱敏方案。

首次基线完成前不设拍脑袋阈值。至少三次同环境稳定样本后，才能评审 RSS、
private bytes、handles、threads、子进程、SQLite/日志增长以及操作成功率阈值。

## 6. 测试专用 pprof 诊断入口

新增 `internal/testdiag` 包，并通过 `//go:build testdiag` 与默认生产构建隔离。
入口只监听系统分配的 `127.0.0.1` 临时端口，使用独立 `http.ServeMux` 注册
pprof handler，不修改 `http.DefaultServeMux`，也不接受外部网络地址。测试覆盖：

- pprof 首页和 goroutine profile 可访问且响应非空；
- listener 必须是 loopback；
- `Close` 可重复调用，关闭后端点不可访问；
- 默认 `go list ./...` 不包含 `kwu/internal/testdiag`，显式
  `-tags=testdiag` 时才包含该包。

确定性复核命令：

```text
node scripts/testing-testdiag-guard.test.mjs
node scripts/testing-testdiag-guard.mjs
```

本轮两条命令均通过；后者会重新执行
`go test -tags=testdiag ./internal/testdiag -count=1`。该入口是可供后续真实 App
测试夹具显式启动的诊断组件，生产默认构建中不可见。

## 7. 尚未完成

- 没有执行 8 小时长稳，T3.6 与 Checkpoint M3 保持未完成。
- 没有 benchstat 置信比较、真实冷启动或 P95/P99；仅有一轮五样本原始采样。
- 没有修改 CI/nightly/release workflow；`REL-TEST-001` 仍需评审。
