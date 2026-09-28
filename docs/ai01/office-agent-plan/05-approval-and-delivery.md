# 阶段 5：审批与可靠交付

> 阶段目标：任何对外副作用都必须绑定确定的 Artifact 版本，经独立策略判断、可撤销审批和可靠 Outbox 执行。

## P5-1 Approval Domain 与 Policy Engine

**目标**：把审批策略放在 Agent 外部，按动作、数据分类、目标和 Artifact hash 决策。

| 方面 | 实施内容 |
|---|---|
| 前端 | 展示动作、目标、数据分类、Artifact hash/预览、有效期和批准范围。 |
| 后端 | 定义 Policy Engine、ApprovalRequest/Decision；默认一次性批准；内容指令不能提升权限。 |
| 数据库 | `approval_requests`、`approval_decisions`，记录策略版本、hash、actor、expiry 和理由。 |
| 测试 | hash 变化失效、过期、拒绝、重复决策、越权 actor、prompt injection、并发批准。 |
| 借鉴项目 | `REF-FINEPRINT`：Agent 外策略与人工监督失效用例；`REF-REASONIX`：per-tab approval、MCP trust、DecisionAudit。 |

**验收**：未批准或 Artifact 已变化时无法创建交付；UI 隐藏按钮不能绕过后端策略。

## P5-2 Delivery Outbox 与幂等执行

**目标**：将交付从请求线程解耦，支持重试、幂等、取消、死信和审计。

| 方面 | 实施内容 |
|---|---|
| 前端 | 显示 queued/sending/delivered/failed/cancelled，提供受控重试。 |
| 后端 | Outbox worker、adapter SPI、idempotency key、指数退避、终态收据。 |
| 数据库 | `deliveries`、`delivery_attempts`；唯一幂等键；只保存 opaque destination ref。 |
| 测试 | crash-after-send、重复消费、429/5xx、永久失败、取消竞态、重启恢复。 |
| 借鉴项目 | `REF-ROWBOAT`：连接器和后台 Agent 闭环；`REF-REASONIX`：调度、事件、凭据与 MCP。 |

**验收**：相同幂等键最多产生一次逻辑交付；每次尝试有可查询收据。

## P5-3 首个本地/邮件交付 Adapter

**目标**：先实现受限本地导出，再按许可与凭据条件实现一个邮件渠道。

| 方面 | 实施内容 |
|---|---|
| 前端 | 目标选择、收件人确认、附件预览、分类提示和发送结果。 |
| 后端 | 本地 adapter 强制 workspace confinement；邮件 adapter 使用既有 credential store 和最小 OAuth scope。 |
| 数据库 | destination 仅保存逻辑引用和脱敏显示名；token 不进入业务表。 |
| 测试 | 路径穿越、无效地址、附件替换、凭据失效、重复发送和敏感日志扫描。 |
| 借鉴项目 | `REF-ROWBOAT`：邮件连接产品流；`REF-REASONIX`：credential/MCP/event；`REF-FINEPRINT`：内容与权限分离。 |

**验收**：发送前 Artifact hash 与批准 hash 一致；凭据和文档正文不进入普通日志。

## P5-4 多渠道扩展与数据分类

**目标**：以相同 SPI 扩展 MCP/企业连接器，并以数据分类决定允许渠道和目标。

| 方面 | 实施内容 |
|---|---|
| 前端 | 渠道能力、权限范围、数据分类冲突和管理员策略说明。 |
| 后端 | capability discovery、destination resolver、tenant boundary、DLP hook。 |
| 数据库 | 渠道配置版本、策略引用和审计元数据；秘密仍由凭据存储管理。 |
| 测试 | 跨租户、scope 扩大、渠道降级、连接器超时、撤销凭据。 |
| 借鉴项目 | `REF-ROWBOAT`：MCP/连接器组织方式；`REF-REASONIX`：MCP trust/credential store。 |

**验收**：新增渠道不修改核心 Approval/Outbox 状态机；跨租户引用被拒绝。

## P5-5 审计、撤销与保留策略

**目标**：把批准、交付、重试、取消和清理组成可验证审计链。

| 方面 | 实施内容 |
|---|---|
| 前端 | 时间线、收据下载、撤销入口和保留期说明。 |
| 后端 | append-only 决策事件、hash chain/签名选型 ADR、受控撤销与补偿。 |
| 数据库 | 审计事件只追加；业务正文与审计元数据分级保留。 |
| 测试 | 篡改检测、撤销窗口、保留期、合法清理和审计导出。 |
| 借鉴项目 | `REF-REASONIX`：DecisionAudit/eventwire；`REF-OPENADAPT`：步骤与 postcondition 记录。 |

**验收**：可从 Run → Artifact → Approval → Delivery receipt 完整追踪。

## 阶段检查点

- [ ] 所有远端副作用均经过后端 Policy Engine。
- [ ] Outbox 在重启、重试与重复消费下保持幂等。
- [ ] 凭据不进入任务、交付或普通日志。
- [ ] 至少一个本地和一个受控远端渠道完成 E2E。

**退出门**：完成安全审查和真实失败演练后，才允许增加更多渠道。
