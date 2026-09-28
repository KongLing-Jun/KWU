# Phase 3：统一确定性经营诊断 Pipeline

> 入口：[总索引](./README.md) · 前置：[Phase 2](./02-sqlite-control-plane.md) · 下一阶段：[Artifact Engine](./04-artifact-engine.md)

## 1. 阶段目标

Desktop 与 serve 统一执行“采集 → 标准化 → 质量门禁 → 指标/trace → 可选 Agent 解读”，AI 不得重算或覆盖确定性事实。

## 2. 任务

### P3-1：拆 SourceReader

| 方面 | 工作 |
|---|---|
| 前端 | 数据源显示读取状态、行数、hash、fetchedAt |
| 后端 | CSV/XLSX/HTTP/inline rows 实现统一 SourceReader |
| 数据库/文件 | source config 入 task_sources；原始摘要入 diagnostics artifact |
| 测试 | 坏编码、坏 XLSX、超大响应、取消、各格式 golden |
| 参考项目 | `REF-REASONIX` 当前 `business.go`；`REF-TASKWEAVER` 只参考插件式数据任务边界 |

**验收：** 读取阶段不做指标计算；错误有 sourceId 和 typed code。
**依赖：** P2-3。**范围：** M。

### P3-2：拆 Normalizer 与 QualityGate

| 方面 | 工作 |
|---|---|
| 前端 | 列级错误、warning/blocking、源行定位 |
| 后端 | 字段别名、类型、region、窗口 warning、必需列/基线/重复检查分离 |
| 数据库/文件 | quality issues 和 normalized dataset 写 diagnostics JSON |
| 测试 | 缺列、坏日期/数值、重复、无当前期、无环比/同比基线 |
| 参考项目 | `REF-REASONIX` 现有确定性规则；`REF-OPENADAPT` 的 postcondition/失败即停 |

**验收：** blocking 时后续 Metric/Agent 不执行。
**依赖：** P3-1。**范围：** M。

### P3-3：MetricCalculator 与 provenance trace

| 方面 | 工作 |
|---|---|
| 前端 | 指标卡可展开 sourceId/kind/rows |
| 后端 | 指标公式保持现有行为；每个 metric 绑定 trace/citation id |
| 数据库/文件 | metrics/trace 留 diagnostics JSON；DB 只存 count/index |
| 测试 | 重构前后 golden equivalence、边界值、浮点和时间窗口 |
| 参考项目 | `REF-REASONIX` 当前 metrics；`REF-TASKWEAVER` 的分析计划/解释器思想 |

**验收：** 任一关键指标都能回到原始行；相同输入 hash 得到相同 facts。
**依赖：** P3-2。**范围：** M。

### P3-4：Desktop/serve 统一 Orchestrator

| 方面 | 工作 |
|---|---|
| 前端 | 任务配置“仅事实/事实+AI 解读”；页面分区展示 |
| 后端 | 两宿主共用 Pipeline；成功后才调用各自 Agent Runner |
| 数据库/文件 | `diagnostics.json` 和 `narrative.md` 独立 artifact |
| 测试 | 同输入跨宿主 facts hash 相同；blocked 不调用模型 |
| 参考项目 | `REF-REASONIX` 共享 Service/Controller；`REF-TASKWEAVER` 只参考 facts→解释分层 |

**验收：** Agent 不可用时事实仍可导出；宿主差异只在 Session 承载。
**依赖：** P3-3、P1-5。**范围：** M。

### P3-5：HTTP source 安全

| 方面 | 工作 |
|---|---|
| 前端 | 显示域名、格式、大小、连接测试和策略拒绝原因 |
| 后端 | 限制协议/重定向/私网/DNS rebinding/大小/超时/content-type |
| 数据库 | 只存 URL 和凭据引用，不存 token/password |
| 测试 | localhost、link-local、私网、重定向、DNS 切换、zip bomb |
| 参考项目 | `REF-FINEPRINT` 的外部策略原则；`REF-REASONIX` 现有网络/工具权限 |

**验收：** SSRF 负向用例全拒绝且有审计。
**依赖：** P3-1。**范围：** M，可与 P3-2/3 并行。

### P3-6：真实 MCP source 与 inline_rows 拆分

| 方面 | 工作 |
|---|---|
| 前端 | MCP 选择 server/tool；粘贴数据改名 inline_rows |
| 后端 | MCP 调用复用 trust/approval/schema；返回仍走 Normalizer |
| 数据库 | source_type=mcp_tool/inline_rows；MCP 只存引用 |
| 测试 | tool 不可信、schema mismatch、超时、撤销授权 |
| 参考项目 | `REF-REASONIX` MCP trust；`REF-WORD-MCP` 的资源/工具分离；`REF-ROWBOAT` 的 MCP 产品连接方式 |

**验收：** 配置中的 rows 不再被误解为实时 MCP；秘密不落任务表。
**依赖：** P3-1、P3-5。**范围：** M。

## 3. Checkpoint P3

- [ ] Desktop/serve facts、issues、metrics、trace 完全一致。
- [ ] blocked 数据 0 次模型调用。
- [ ] 每个指标最多三次点击可定位来源行。
