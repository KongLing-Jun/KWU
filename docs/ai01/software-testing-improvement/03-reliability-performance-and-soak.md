# Phase 3：可靠性、性能与长稳

## Goal

发现短时单元测试不易暴露的数据竞争、goroutine/句柄泄漏、性能退化、恢复失败和长时间运行问题，并形成可比较的基线。

## Task 3.1：race 分层

**Goal**：让 race 覆盖真实并发路径，同时控制 PR 成本。

- PR：修改并发 owner 时运行定向 `go test -race`；root broad change 保留现有 full race。
- Nightly：补齐 full root race，并评估 desktop module 可稳定运行的范围。
- Release candidate：在目标 Windows 架构对关键生命周期和更新路径执行定向 race/压力场景；若平台或 CGO 限制阻止执行，记录替代证据。

race 只发现实际执行到的路径。取消、重复 completion、stale result、关闭顺序和超时必须显式构造调度。

## Task 3.2：goleak 与资源 owner

列出现有 `goleak.VerifyTestMain` package，重点审计 agent、jobs、plugin、LSP、control、heartbeat、event loop 和更新下载。

- 每个长期 goroutine 有创建、取消、等待和 owner。
- 不使用宽泛 ignore 吞掉未知泄漏。
- 同时跟踪 Windows Handle Count、线程数、子进程、临时文件和 SQLite 连接。
- 关闭/退出测试要等待资源回收，而不是依赖任意 sleep。

## Task 3.3：Go 原生 fuzz

P0 候选：

- Session JSONL/meta、配置、manifest、事件和 provider payload 解析。
- 路径、编码、导入导出和命令参数。
- 更新清单、版本、URL、签名和下载元数据。
- 金额、幂等键、状态机和迁移输入。

每个 target 包含合法 seeds、边界 seeds 和历史缺陷；断言无 panic、资源有界和业务不变量。发现的 corpus 必须最小化并转为普通回归测试。

## Task 3.4：性能基线

**Goal**：围绕用户等待和资源占用，而不是只追求微基准数字。

| 指标 | 建议场景 |
|---|---|
| 冷/热启动 | 新 profile、已有大历史、离线、代理失败 |
| 会话列表/恢复 | 不同 JSONL 数量和大小、损坏 sidecar |
| 内存/GC | 长对话、附件、频繁切换工作区 |
| goroutine/句柄 | 空闲、任务运行、断网重连、重复打开文件 |
| 磁盘 | autosave、迁移、更新下载、日志轮转 |
| UI 响应 | 大列表、流式输出、窗口最小化/恢复 |

Go 热点使用 `testing.B`、`benchstat`；pprof 只在诊断/测试构建显式开启。基线必须记录硬件、OS、Go 版本、提交 SHA、样本数和置信信息。

## Task 3.5：8/24/72 小时长稳

分层执行：

- Nightly 8h：空闲、定时任务、断网/恢复、会话写入、模型 mock。
- Release 24h：真实桌面进程、重复打开/关闭工作区、休眠/唤醒、网络切换。
- 重大版本或重点客户 72h：目标硬件、EDR/代理/自定义 CA 环境。

每次采集：

- RSS/private bytes、GC、goroutine、线程、Windows handles。
- SQLite 文件/锁、临时目录、日志大小、子进程。
- 操作成功率、P95/P99 延迟、重试和未处理错误。
- 崩溃 dump 与脱敏诊断包。

阈值由三次稳定基线和产品 SLO 决定，不在首次执行前拍脑袋设定。

## Task 3.6：性能与稳定性回归处理

- 回归必须能定位到版本、场景和指标。
- 自动重跑用于确认和收集证据，不覆盖首次失败。
- 不以关闭监控、扩大泄漏 ignore 或放宽超时作为修复。
- 基线更新必须单独评审，并说明硬件、数据集或预期行为为何改变。

## 阶段退出门

- [x] race 责任在 PR/nightly/release 间清晰分层。
- [x] 关键 goroutine、句柄和子进程有 owner 与回收测试。
- [x] 至少 3 个高风险输入边界具备 fuzz target。
- [ ] 冷启动、会话恢复和长时间运行有可比较基线。
- [ ] 至少完成 8 小时长稳并能输出脱敏证据。
