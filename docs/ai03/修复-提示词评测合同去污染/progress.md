# 提示词评测合同去污染进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 阶段 | 实现、Desktop 验收与最终 PR Gate 完成 |
| 基线 | `origin/main-v3@bff9cff87fc5d5c3fcfbda9a038ffb07592663c2` |
| 分支 | `codex/fix-prompt-eval-contracts` |
| worktree | `D:\Reasonix-worktrees\fix-prompt-eval-contracts` |
| 稳定计划 | [`plan.md`](./plan.md) |
| 执行任务 | [`tasks/001-收口评测合同并完成RF003后Desktop复验.md`](./tasks/001-收口评测合同并完成RF003后Desktop复验.md) |
| PR | 未创建；实际状态以 Git/GitHub 为准 |

## 已锁定决策

- RF-001 接受 0、拒绝负数。
- RF-002 使用公开固定内容和结构化 focus 检查。
- RF-005 接受 `ValueError` / `TypeError`，其他异常失败。
- RF-007 公开 SVG 引用要求，画布接受正尺寸或正尺寸 `viewBox`。
- RF-004 已在现有源码修复；RF-006 非产品缺陷；不重复改动。
- 本 PR 不修改生产提示词或运行时。
- 自动化稳定后执行三场景各两次真实 Wails Desktop 复验。

## 当前验证

- Codegraph 已从 clean 索引追踪 prompt-eval 参数、任务与 grader 影响面。
- 索引基线到实现基线的 `benchmarks/prompt-eval` 文件无差异。
- RF-001、RF-002、RF-005、RF-007 合同修复及正反测试已完成；focused Go test、vet、
  code guard 和 diff check 已通过。
- 真实 Wails Desktop 六轮端到端 5/6：Office 1/2、Coding 2/2、Design 2/2。
- 六轮 active 平均 131.427 s、平均 328,705 tokens；可测审批等待合计 3.130 s，不足以解释
  场景间效率差异。一次 67.7 s 审批污染的探索轮次已排除。
- RF-003 的产物状态陈旧路径未复现；Office R1 的一次 readiness 拒绝对应未完成任务项。
- Office R2 遗漏 `contractor` 且错误声称已验证，是下一轮生产提示词的首要定向问题。
- 已确认 `origin/main-v3` 仍为实现基线；最终 PR Gate 按 Root Go targeted 模式通过。

## 下一门禁

等待创建 Draft PR 的明确授权。生产提示词改动留给后续独立切片。
