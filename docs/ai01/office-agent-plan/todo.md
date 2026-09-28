# Office Agent 分阶段执行清单

> 使用方法：一次只批准一个阶段；任务 PR/提交标题携带任务 ID；勾选前必须满足任务文档验收条件和 [跨阶段完成定义](cross-cutting-contracts.md#8-每个任务的-definition-of-done)。项目 ID 解释见 [参考项目矩阵](reference-projects.md)。

## Phase 0：基线、契约与 ADR

- [ ] P0-1 冻结用户旅程与 MVP 范围 — 借鉴 `REF-OFFICEDEX`、`REF-ROWBOAT`；实现真源 `REF-REASONIX`
- [ ] P0-2 建立当前行为、缺口和证据基线 — `REF-REASONIX`
- [ ] P0-3 冻结 API v2、事件和状态机 — 借鉴 `REF-WORD-MCP`、`REF-UNIVER`
- [ ] P0-4 Runtime/Sidecar/Artifact IR/SQLite/GUI fallback ADR — 评估 `REF-UNIVER`、`REF-OFFICE-AI-AGENT`、`REF-ONLYOFFICE`、`REF-UFO`
- [ ] P0-5 建立 12 组业务与 Office 文件样本 — 借鉴 `REF-WORLDGUI` 的非默认初态
- [ ] P0-6 完成 Runtime PoC 和许可证门 — 对照 `REF-OFFICEDEX`、`REF-UNIVER`、`REF-ONLYOFFICE`
- [ ] Gate 0：范围、合同、ADR、样本和许可证评审通过

## Phase 1：运行可靠性

- [ ] P1-1 租约、heartbeat 与 fencing 合同 — `REF-REASONIX`
- [ ] P1-2 Engine heartbeat 与 stale writer 拒写 — `REF-REASONIX`、`REF-OPENADAPT`
- [ ] P1-3 异步 StartRun、幂等与 typed conflict — `REF-REASONIX`
- [ ] P1-4 取消、唯一终态与竞态处理 — `REF-OPENADAPT`
- [ ] P1-5 重启恢复与 crash marker — `REF-OPENADAPT`、`REF-OFFICE-AI-AGENT`
- [ ] P1-6 分页、错误映射、归档保留与 tombstone — `REF-OFFICE-AI-AGENT`
- [ ] Gate 1：多实例、长运行、取消、崩溃、重启故障注入通过

## Phase 2：SQLite 控制平面

- [ ] P2-1 Repository 合同、schema 与 migration runner — `REF-REASONIX`
- [ ] P2-2 Task 导入、version、软删除和兼容读取 — `REF-REASONIX`
- [ ] P2-3 Run/Event/Artifact 元数据索引 — `REF-OFFICE-AI-AGENT`、`REF-REASONIX`
- [ ] P2-4 增量 CRUD/CAS 与 409 恢复 — `REF-REASONIX`
- [ ] P2-5 SQLite 主读、游标分页和查询计划 — `REF-REASONIX`
- [ ] P2-6 shadow write/read-compare/回滚窗口 — `REF-REASONIX`
- [ ] Gate 2：历史数据、重复导入、迁移崩溃、回滚和容量基准通过

## Phase 3：经营诊断 Pipeline

- [ ] P3-1 SourceReader/Normalizer/QualityGate — 借鉴 `REF-TASKWEAVER`
- [ ] P3-2 MetricCalculator 与 trace 合同 — `REF-REASONIX`
- [ ] P3-3 Desktop/serve 统一 Orchestrator — 借鉴 `REF-TASKWEAVER`，实现 `REF-REASONIX`
- [ ] P3-4 HTTP/MCP 信任边界与注入防护 — `REF-WORD-MCP`、`REF-UFO`、`REF-FINEPRINT`
- [ ] P3-5 真实 MCP 数据源与 inline rows 清晰区分 — `REF-WORD-MCP`、`REF-ROWBOAT`
- [ ] P3-6 Trace/质量/指标 UI 与 golden — `REF-TASKWEAVER`、`REF-REASONIX`
- [ ] Gate 3：双宿主 facts 一致、blocked 数据不调用模型、12 组 golden 通过

## Phase 4：Artifact Engine

- [ ] P4-1 Runtime Adapter 与安全执行边界 — `REF-OFFICE-AI-AGENT`、`REF-UNIVER`、`REF-ONLYOFFICE`、`REF-OFFICEDEX`
- [ ] P4-2 Canonical Artifact IR 与版本模型 — `REF-UNIVER`、`REF-OFFICE-AI-AGENT`
- [ ] P4-3 Excel 月度经营报告纵向切片 — `REF-UNIVER`、`REF-OFFICE-AI-AGENT`、`REF-OPENADAPT`
- [ ] P4-4 DOCX Renderer 与局部编辑边界 — `REF-WORD-MCP`、`REF-OFFICE-AI-AGENT`、`REF-ONLYOFFICE`
- [ ] P4-5 PPTX Renderer 与视觉 QA — `REF-OFFICE-AI-AGENT`、`REF-UNIVER`、`REF-ONLYOFFICE`
- [ ] P4-6 模板、缓存和 Artifact 生命周期 — `REF-OFFICE-AI-AGENT`、`REF-UNIVER`、`REF-OFFICEDEX`
- [ ] Gate 4：先通过 Excel 真实文件/回滚/验证，再批准 Word/PPT 扩展

## Phase 5：审批与交付

- [ ] P5-1 Approval Domain 与 Policy Engine — `REF-FINEPRINT`、`REF-REASONIX`
- [ ] P5-2 Delivery Outbox 与幂等执行 — `REF-ROWBOAT`、`REF-REASONIX`
- [ ] P5-3 首个本地/邮件 Adapter — `REF-ROWBOAT`、`REF-FINEPRINT`
- [ ] P5-4 多渠道扩展与数据分类 — `REF-ROWBOAT`
- [ ] P5-5 审计、撤销和保留策略 — `REF-OPENADAPT`、`REF-REASONIX`
- [ ] Gate 5：真实失败、重复发送、凭据撤销和跨租户安全测试通过

## Phase 6：Office Workbench

- [ ] P6-1 前端模块拆分与状态边界 — `REF-OFFICEDEX`、`REF-REASONIX`
- [ ] P6-2 任务向导 — `REF-OFFICEDEX`、`REF-ROWBOAT`
- [ ] P6-3 运行中心与事件恢复 — `REF-OFFICEDEX`、`REF-REASONIX`
- [ ] P6-4 Artifact 预览、对比与版本恢复 — `REF-UNIVER`、`REF-OFFICEDEX`、`REF-OFFICE-AI-AGENT`
- [ ] P6-5 审批与交付中心 — `REF-ROWBOAT`、`REF-FINEPRINT`
- [ ] P6-6 Wails/HTTP 合同一致性 — `REF-REASONIX`、`REF-UNIVER`
- [ ] P6-7 可访问性、i18n 和非默认状态 — `REF-WORLDGUI`、`REF-OFFICEDEX`
- [ ] Gate 6：桌面 E2E、断线恢复、大文件与键盘旅程通过

## Phase 7：安全、观测、性能与发布

- [ ] P7-1 威胁模型与攻击测试 — `REF-FINEPRINT`、`REF-WORLDGUI`、`REF-UFO`、`REF-OFFICE-AI-AGENT`
- [ ] P7-2 GUI fallback 隔离与门控 — `REF-UFO`、`REF-OPENADAPT`、`REF-WORLDGUI`
- [ ] P7-3 可观测性与运维 — `REF-OPENADAPT`、`REF-REASONIX`
- [ ] P7-4 性能、容量与稳定性 — `REF-WORLDGUI`、`REF-UFO`、`REF-UNIVER`、`REF-ONLYOFFICE`
- [ ] P7-5 迁移、特性开关与回滚演练 — `REF-REASONIX`、`REF-OPENADAPT`
- [ ] P7-6 许可证、供应链、打包与发布门 — `REF-OFFICEDEX`、`REF-ONLYOFFICE`、`REF-UNIVER`
- [ ] P7-7 最终质量门与发布验收 — 以 `REF-REASONIX` 为唯一准入依据
- [ ] Gate 7：安全、容量、迁移、安装、许可证和项目规定检查全部通过

## 全局未决策项

- [ ] Runtime 采用内嵌、sidecar 还是混合方案（ADR）
- [ ] Univer 开源版能否覆盖 Excel 首个切片的导入/导出要求（PoC）
- [ ] DOCX/PPTX 保真支持矩阵与明确降级策略
- [ ] ONLYOFFICE 的分发、网络使用和商业许可可行性
- [ ] GUI fallback 是否进入产品、默认关闭策略和资源预算
- [ ] 首个远端交付渠道、OAuth scope 与租户模型
- [ ] 审计 hash chain/签名需求和保留期限

以上项目在证据不足时保持“候选/计划”，不得提前标记为已实现或已选型。
