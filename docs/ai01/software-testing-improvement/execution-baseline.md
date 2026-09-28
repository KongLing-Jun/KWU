# 测试执行基线

> 状态：Baseline accepted for phase entry
>
> 源码基线：`15a98177251bd024b499206e9d90b979fbc48436`
>
> 环境：Windows 10.0.26200、amd64、PowerShell 5.1.26100.8875、
> Go 1.26.4、Node 26.1.0、pnpm 11.1.3、16 logical processors
>
> 采集入口：`scripts/testing-baseline.ps1`

本文对应 T0.2。原始日志与逐秒资源采样保存在忽略目录
`output/testing-baseline/<timestamp>/`，不提交可能包含本机路径的运行日志。
提交到仓库的仅为脱敏汇总、失败分类和可复现命令。

## 1. 当前有效结果

| 时间 | 套件 | 结果 | 耗时 | 峰值工作集 | 峰值句柄 | 峰值进程 | 观测 CPU |
|---|---|---|---:|---:|---:|---:|---:|
| 2026-07-23 | `go test ./...` | pass | 516,710 ms | 477 MiB | 1,419 | 3 | 568.047 s |
| 2026-07-23 | `go test -race ./...` | fail | 639,461 ms | 6.69 GiB | 6,804 | 23 | 529.141 s |
| 2026-07-23 | `go test -race ./...`（修复后） | pass | 1,646,302 ms | 845 MiB | 1,901 | 6 | 1,612.906 s |
| 2026-07-23 | Root atomic coverage | pass | 628,439 ms | 2.05 GiB | 3,879 | 20 | 566.297 s |
| 2026-07-23 | Desktop `go test ./...` | pass | 179,651 ms | 2.24 GiB | 2,277 | 6 | 104.797 s |
| 2026-07-23 | Frontend `pnpm test:all` | pass | 164,925 ms | 843 MiB | 1,802 | 7 | 19.328 s |
| 2026-07-23 | Frontend `pnpm build` | pass | 25,842 ms | 2.20 GiB | 1,472 | 5 | 39.766 s |

Root coverage profile 为 2,396,840 bytes。资源是每秒采样到的进程树观测值，
不是 profiler 总量。首次成功 race
运行超过 27 分钟，因此 full race 适合 broad PR/nightly，不适合作为每个局部
改动的无条件短反馈门禁。

## 2. 已完成三轮的套件

统计由以下命令生成，使用 nearest-rank 百分位：

```powershell
node scripts/testing-baseline-summary.mjs `
  output/testing-baseline/20260723-174321/results.json `
  output/testing-baseline/20260723-180109/results.json
```

| 套件 | 样本 | 通过 | P50 | P95 | 范围 |
|---|---:|---:|---:|---:|---:|
| Desktop Go | 3 | 3 | 96,247 ms | 179,651 ms | 89,428～179,651 ms |
| Frontend test | 3 | 3 | 169,643 ms | 191,546 ms | 164,925～191,546 ms |
| Frontend build | 3 | 3 | 26,139 ms | 32,481 ms | 25,842～32,481 ms |

三样本 P95 等于最大值，只作为当前执行成本的初始观测，不作为性能 SLO。
汇总器会拒绝跨源码提交合并，并兼容 Windows PowerShell 5.1 生成的 UTF-8 BOM。

## 3. 首次 race 失败与修复

首次 full race 在
`TestParallelTasksCancellationDoesNotWaitForChildToolAfterModelCallSettles`
发现三个数据竞争报告。两个并行子任务同时调用测试共享
`mockProvider.Stream`，无锁写入 `lastReq` 与 `requests` 切片。

修复只在 `internal/agent/coordinator_test.go` 的测试 mock 中保护请求记录和
流选择临界区，不改变生产并行任务行为。

验证链：

```powershell
go test -race ./internal/agent `
  -run '^TestParallelTasksCancellationDoesNotWaitForChildToolAfterModelCallSettles$' `
  -count=10
go test -race ./internal/agent
go test -race ./...
```

结果依次为通过、通过、通过。首次失败被保留，不以最终重试成功改写。

## 4. 采集器自身修复

第一次采集时，Windows PowerShell 的 `Start-Process -PassThru` 未返回可用
`ExitCode`，使一个 stdout 全部为 `ok`、stderr 为空的 Root Go 套件被错误标为
fail。执行器现在通过隔离子进程写入 ASCII exit-code 文件，并用真实
`exit 7` 回归测试验证非零退出码。

执行器还提供每套件 30 分钟默认 timeout；超时会归档为 `timeout`，并按当前
测试父子树终止进程，避免留下孤儿 Go/Node 进程。

## 5. 单轮接受决定与尚缺证据

2026-07-23 用户明确要求停止 Root Go 与 coverage 的第 2、3 轮，单轮结果
足够作为本次工作进入下一阶段的基线。正在执行的第二轮 Root Go 因该决定被
终止；它是用户指示的取消，不计为测试失败，也不进入统计。原始目录为
`output/testing-baseline/20260723-181303/`。

- Root Go、root race、coverage 各保留一个成功样本，不计算 P50/P95，也不将
  单样本解释为稳定性或性能 SLO。
- goroutine、磁盘目录增量与产品日志大小需要诊断接口或场景级采集补充。
- 严格冷缓存需要显式、可恢复的缓存隔离方案；当前只标记首次观测与热重复，
  不声称清除了系统或工具链缓存。

T0.2 按本次明确调整后的接受条件完成；Checkpoint M0 仍受环境矩阵和决策
owner 评审约束。
