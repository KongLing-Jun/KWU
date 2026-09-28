# Phase 1：运行可靠性与一致性

> 入口：[总索引](./README.md) · 前置：[Phase 0](./00-baseline-and-adrs.md) · 下一阶段：[SQLite 控制平面](./02-sqlite-control-plane.md)

## 1. 阶段目标

修复当前文件租约无续期、Trigger 同步等待、取消仅进程内、错误映射和归档保留未闭环等问题。

## 2. 任务

### P1-1：租约模型与 fencing 合同

| 方面 | 工作 |
|---|---|
| 前端 | 定义 lease_lost/taken_over/owner_unreachable 文案 |
| 后端 | 为 Lease 增加 generation/fencingToken，定义合法接管与 stale writer 规则 |
| 数据库/文件 | 先扩展 `leases/*.json`，Phase 2 再迁 SQLite |
| 测试 | Lease JSON 往返、token 单调递增、过期边界 |
| 参考项目 | `REF-REASONIX` 本地 `lease.go` 为主；`REF-OPENADAPT` 借鉴失败即停/postcondition |

**验收：** token 单调；旧 token 写入被拒绝；规则写入 ADR-004。
**依赖：** P0-2。**范围：** S，1～2 个模型/测试文件。

### P1-2：Engine heartbeat 生命周期

| 方面 | 工作 |
|---|---|
| 前端 | 无新页面，只消费新状态 |
| 后端 | 获租约后每 TTL/3 刷新；丢租约取消 Runner；Stop 等待 goroutine |
| 数据库/文件 | 原子刷新 lease 文件并保留 owner/run/token |
| 测试 | >2×TTL 长任务、刷新失败、崩溃接管、goleak/race |
| 参考项目 | `REF-REASONIX`；`REF-OPENADAPT` 的可观察后置条件 |

**验收：** 两 Engine 竞争时只有当前 fencing owner 可提交终态。
**依赖：** P1-1。**范围：** M，Engine/Lease/测试 3～5 文件。

### P1-3：异步 StartRun 与幂等键

| 方面 | 工作 |
|---|---|
| 前端 | 点击后立即获得 runId，插入 queued 卡片并进入详情 |
| 后端 | 新增 StartRun；先持久化 queued，再后台执行；clientRequestId 去重 |
| 数据库/文件 | Phase 1 写入 manifest；Phase 2 建唯一约束 |
| 测试 | 双击、超时重试、租约冲突、Wails/HTTP 202 合同 |
| 参考项目 | `REF-REASONIX` Automation Service；`REF-OFFICE-AI-AGENT` 的 request-level idempotency 思路 |

**验收：** 300ms 内返回 runId；重复请求不创建第二个 run。
**依赖：** P1-2。**范围：** M，Service/API/bridge/测试。

### P1-4：持久化取消与终态竞争

| 方面 | 工作 |
|---|---|
| 前端 | 显示 cancel_requested、not_cancelable、already_finished，不丢 CancelResponse |
| 后端 | 记录取消请求；唯一终态提交；取消/成功竞争使用期望旧状态 |
| 数据库/文件 | manifest 增加 cancelRequestedAt/By；Phase 2 映射 runs |
| 测试 | 模型、工具、审批等待时取消；取消与成功并发 |
| 参考项目 | `REF-REASONIX` 当前 registry；`REF-OPENADAPT` 的失败即停 |

**验收：** “已请求取消”不提前冒充 canceled；终态不可被迟到结果覆盖。
**依赖：** P1-3。**范围：** M。

### P1-5：重启恢复与 abandoned run

| 方面 | 工作 |
|---|---|
| 前端 | 详情显示 recovered/abandoned 原因和可重试动作 |
| 后端 | 启动扫描无有效 owner 的 queued/running；按策略取消、失败或重试 |
| 数据库/文件 | manifest/recovery marker 保存 recoveredAt/reason |
| 测试 | queued/running 各阶段强制退出并重启 |
| 参考项目 | `REF-OPENADAPT` 的可重放/受控修复；`REF-REASONIX` 当前 archive |

**验收：** 不存在永久 running 且无 owner 的运行。
**依赖：** P1-2、P1-4。**范围：** M。

### P1-6：结果分页、错误映射与归档策略

| 方面 | 工作 |
|---|---|
| 前端 | 完整结果分页/流式读取；刷新、导出、取消错误统一展示 |
| 后端 | 事件 cursor、结果分段、404/409/422 typed errors、retention dry-run |
| 数据库/文件 | mark → grace period → delete；保留 result/artifact 索引 |
| 测试 | 超长报告、大 JSONL、并发清理、文件缺失、非法 timeout |
| 参考项目 | `REF-REASONIX` 为主；`REF-OFFICE-AI-AGENT` 借鉴版本/原件保护思想 |

**验收：** 长报告无需导出即可看全；清理不影响活跃/审批中的数据。
**依赖：** P1-4、P1-5。**范围：** M。

## 3. Checkpoint P1

- [ ] 30 分钟运行不会被第二宿主双写。
- [ ] StartRun 幂等、取消、重启恢复 E2E 通过。
- [ ] 所有受影响 Go/desktop/frontend 定向测试通过。
