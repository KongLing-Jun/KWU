# 模型身份回答约束修复进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | Stage 3：最终 PR Gate |
| 计划 | [`plan.md`](./plan.md) |
| 基线 | `origin/main-v3@2a49def2a5f066215b890de6d4505c76c4c31c8a` |
| 当前改动 | 实现与主会话代码复核完成；等待最终 Gate 与凭据环境应用验收 |
| 执行拓扑 | Serial-01；一个原子任务映射一个快速交付 PR |
| 外部 Git/PR 状态 | 以 Git 与 GitHub 为唯一事实源 |

## 已确认决策

- 使用当前会话经 `ResolveModel` 解析后的裸模型名 `entry.Model`。
- 不向用户暴露 `provider/model` 完整引用中的 Provider 前缀。
- “云智企办”是产品助手身份，不是模型名称。
- 模型身份问题必须回答绑定名称，不根据训练认知、风格或兼容协议猜测其他模型。
- 绑定名称不等于远程网关实际路由的可验证证明。
- 采用固定 system prompt 约束，不新增前端逻辑、查询工具或路由探测。
- 用户明确要求小改动快速上线，授权跳过等待独立计划/任务 PR 合并。

## 调查快照

| 检查 | 结论 |
| --- | --- |
| 主工作区 | 基线干净且保持只读 |
| 计划 worktree | 从最新 `origin/main-v3` 创建，checkout 使用 `core.autocrlf=false` |
| 模型事实 | `boot.Build` 在组装 system prompt 前已获得 `entry.Model` |
| 活动注入点 | `internal/boot/boot.go` 的 `buildSystemPrompt` 闭包 |
| 会话首消息 | `agent.NewSession` 将组装结果保存为 system 消息 |
| 模型切换 | Desktop 重建 Controller、刷新 system prompt，并携带其余历史 |
| 现有测试 | 已有切换模型刷新 system prompt 的端到端测试可扩展 |
| 主要风险 | 模型 ID 编码、Provider 前缀泄露、旧身份随历史残留、把绑定误称为实际路由 |

## 当前任务

- [`tasks/001-注入会话绑定模型身份.md`](./tasks/001-注入会话绑定模型身份.md)
- 实现已使用解析后的 `entry.Model` 在活动 `boot.Build` 与重构候选组装路径注入身份策略。
- 已覆盖模型名安全编码、三种模型引用解析、自定义 prompt 和 Desktop 切换刷新。
- `internal/config`、`internal/boot` 与 Desktop 直接受影响 package 完整测试均通过。
- common checks 发现的 config 文件数与 `boot.go` 行数守卫恶化已通过文件内合并修复；
  三组聚焦测试与 `go run ./scripts/code_guard` 通过。
- 已按 Draft PR #236 用户反馈移除路由免责声明；核心模型身份约束保持不变。
- 主会话已审查实际 commit/diff，并独立重跑三组关键测试。
- Computer Use 确认当前没有本分支 Desktop 实例；真实模型回复需在已认证 Provider
  环境验收，现有其他 worktree 的开发进程未作为本次证据。
- 详细变更、命令与剩余风险见任务完成回执。

## 阻塞与下一关口

- 当前无未决产品、兼容性、迁移、安全或外部 API 决策。
- 当前无实现、代码复核或自动化测试阻塞。
- 分支已确认包含最新 `origin/main-v3`；下一关口是最终 PR Gate。
- Gate 通过后可推送 Draft PR；用户完成凭据环境模型回复验收前不得标记 Ready。
- 不自动合并。
