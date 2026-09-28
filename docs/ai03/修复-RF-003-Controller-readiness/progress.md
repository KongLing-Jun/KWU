# RF-003 Controller readiness 修复进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | 实现、独立复核、生产/Desktop 实际验收和最新基线最终 Gate 均已完成；进入外部合并评审阶段 |
| 当前同步基线 | `origin/main-v3@14decefcf7e81ef86ad461071153e1a72bd2a9da` |
| worktree | `D:\Reasonix-worktrees\fix-rf003-controller-readiness` |
| 分支 | `codex/fix-rf003-controller-readiness` |
| 稳定决策 | [`plan.md`](./plan.md) |
| 任务合同 | [`tasks/001-修复产物需求解析与readiness误判.md`](./tasks/001-修复产物需求解析与readiness误判.md) |
| 产品代码 | parser、matcher 诊断及相邻完成态回归修复已完成 |
| PR | Draft PR #321 为交付载体；实际 head/status 以 GitHub 为事实源 |

## 任务与 PR 映射

| 任务 | 依赖与执行关系 | 交付 |
| --- | --- | --- |
| 001 修复产物需求解析与 readiness 误判 | 实现、独立复核和实际验收完成 | 计划、任务、代码、测试和回执共用 Draft PR #321 |

只设置一个实现任务，因为句界、动作作用域、source exclusion、matcher 诊断和
端到端验收共同构成可评审、可测试、可回滚的最小闭环。拆分会产生 parser 已改变但
readiness 仍不可诊断，或测试不能证明完整失败路径的中间状态。

## 已锁定决策

- 根因是 artifact requirement parser 生成错误 targets，不是 verify 或 mutation。
- 采用 sentence-aware、verb-scoped 的确定性修复。
- 保留 source/output 隔离、目标基数、distinct matching、freshness 和 background
  settlement 条件。
- 诊断增强不得复制 matcher 或改变放行语义。
- 不修改 prompt、grader、runner、Desktop 前端、schema、依赖或存储。
- 合并一个 PR 是用户明确批准的流程例外；最终仍需应用验收、同步和 PR Gate。

## 当前验证

| 验证面 | 状态 |
| --- | --- |
| Codegraph | 已在 clean `D:\Reasonix` 索引追踪 parser → ledger → readiness → Controller |
| 真实会话 | Office、Coding、Design 三条父会话均证明 verify 成功后仍被拒绝 |
| 最小失败路径 | 同基线 overlay 诊断已证明错误 targets 导致 `fresh=false` |
| 现有相关测试 | 调查期全部通过，确认缺少真实输入回归覆盖 |
| 实现测试 | 相邻回归先失败复现；parser/readiness 聚焦测试、三个受影响包测试和 `code_guard` 通过 |
| Agent.Run 回归 | 真实 workspace writer/read/verifier 路径通过，Office 双产物验证后无 readiness retry |
| 主会话包测试 | `go test ./internal/agent ./internal/evidence ./internal/tool/builtin -count=1` 通过 |
| 生产 Controller | Office、Coding 通过；Design 的 Controller/readiness 通过，但 artifact grader 因 SVG canvas 0x0 仍有已知模型产物质量偏差 |
| Wails Desktop | 原始编号列表输入自然完成，UI 待办 3/3；最后一次产物操作同时验证两个目标，无 readiness 阻断或重试 |
| PR Gate | 最新同步基线上 common checks 与 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all` 通过；分类为 targeted `./internal/agent ./internal/evidence` |

## 风险与下一门禁

- 主要风险是句点、跨行列表和 `replace/change` 收紧造成 false negative。
- 实现不得用放宽 matcher 或忽略匿名 target 的方式掩盖 parser 错误。
- 首次冷启动 Office 验收曾被无关的 experts table 初始化超时阻塞；同场景重试及完整
  三场景重跑均通过，未观察到 RF-003 失败。
- 剩余外部门禁：交付分支推送后的合并评审；本文不声称 Draft PR #321 已合并。
