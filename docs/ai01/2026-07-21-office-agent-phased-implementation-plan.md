# Office Agent 阶段性实施计划（拆分版导航）

> 本文件保留原路径作为兼容入口。原单体执行计划已拆成可独立评审、实施和验收的阶段文档；请从下方总索引进入。

## 文档入口

- [拆分计划总索引](office-agent-plan/README.md)
- [参考项目与功能借鉴矩阵](office-agent-plan/reference-projects.md)
- [跨阶段合同、数据模型与完成定义](office-agent-plan/cross-cutting-contracts.md)
- [可勾选执行清单](office-agent-plan/todo.md)

## 阶段文档

1. [阶段 0：基线、契约、ADR 与验收样本](office-agent-plan/00-baseline-and-adrs.md)
2. [阶段 1：运行可靠性](office-agent-plan/01-runtime-reliability.md)
3. [阶段 2：SQLite 控制平面](office-agent-plan/02-sqlite-control-plane.md)
4. [阶段 3：经营诊断 Pipeline](office-agent-plan/03-business-diagnostics-pipeline.md)
5. [阶段 4：Artifact Engine 与 Office 输出](office-agent-plan/04-artifact-engine.md)
6. [阶段 5：审批与可靠交付](office-agent-plan/05-approval-and-delivery.md)
7. [阶段 6：Office Workbench](office-agent-plan/06-office-workbench.md)
8. [阶段 7：安全、观测、性能与发布](office-agent-plan/07-security-observability-release.md)

## 执行约束

- 一次只批准一个阶段，按任务 ID 开发，并通过阶段 Gate 后再进入下一阶段。
- 每个任务均需覆盖前端、后端、数据库/迁移、测试、依赖和验收；不适用项要显式说明。
- 外部项目仅按矩阵中列出的能力借鉴。GPL/AGPL 项目默认只做黑盒或许可证评估，未经 ADR 与法务结论不得复制或进入分发物。
- Reasonix 现有 `boot.Build → control.Controller → agent.Agent` 主链、Automation Engine、MCP/审批/凭据和事件体系是实现真源，不得平行新建第二套核心。
- 文档中的候选技术与目标 API 均属于实施计划，不表示已经实现或已经选型。

## 推荐的首个开发切片

依次完成阶段 0、阶段 1 必要可靠性、阶段 2 最小控制平面和阶段 3 facts pipeline，然后实施 `P4-3 Excel 月度经营报告`，打通：

`创建任务 → 异步运行 → 经营诊断 → XLSX Artifact → 验证/版本/回滚 → 审批 → 受控交付`

该切片通过真实文件和失败恢复验收后，再扩展 DOCX、PPTX、更多交付渠道及 GUI fallback。
