# 任务 002：隔离父子事件并收敛 parallel 展示

## 状态与交付

- 状态：已完成。
- 执行组：Serial-02；由当前主 Agent 执行，不委派子代理。
- 依赖：任务 001 已完成并提交。
- 交付：一个可独立评审、测试和回滚的事件/前端提交，包含源码、测试和完成回执。

## 目标

在事件投影入口区分父事件、持久 task child 事件和临时 parallel branch 事件，使普通父 transcript
永远只显示父 Agent；同时把 parallel UI 收敛成一张可由父结果恢复的聚合卡。

## 预期产物

- 持久 child 事件携带稳定 ref 与 raw child tool ID。
- frontend state 分为 `mainItems`、`childItemsByRef`和 parallel call 内部 branch state。
- 普通 task 不再出现 child reasoning/message/tool/final 或空 assistant 占位。
- parallel 只显示外层卡和分支状态/摘要，不生成 AgentWorkspace 分支 tab 或历史入口。
- 实时投影和 historyProjection 重载结果一致。

## 允许范围

- Root event wire 与 `internal/agent/task.go:subSinkForContext`
- `internal/agent/parallel_tasks.go`
- `desktop/frontend/src/features/session/*Projection*`、session state/reducer/types
- `Transcript.tsx`、`ToolCard.tsx`、必要的 `AgentWorkspace.tsx`调整
- 直接相关 Root/Frontend 测试与本任务回执

## 禁止范围

- 专家 sidecar、成员 ref 绑定、专家身份状态机。
- child 工具归档详情 bridge；由任务 004 处理。
- task 运行时状态机；缺陷返回任务 001 修复，不在本任务重写。
- parallel child 持久化、ref、续写、历史页或单支 wait/stop。

## 源码依据

- `subSinkForContext`当前把 child Text/Reasoning/Message/Tool转发给父 sink，仅依赖 ParentID。
- `eventProjection.ts`当前把父/child assistant 放入同一 items/currentAssistantByParent。
- `Transcript.tsx`只跳过 parented tool，没有统一跳过 parented assistant。
- `parallel_tasks.go`前台分支使用 `NewSession("")`，事件以 synthetic task 嵌套。
- `ToolCard`当前把 parallel direct tasks交给 AgentWorkspace，重载后没有持久 ref 可恢复。

## 实现约束

1. 第一个 child 事件就必须带 ref并进入 child buffer；不得短暂污染 mainItems。
2. ParentID只表达嵌套，不能替代 Session 身份。
3. parallel 分支不获得 subagentRef，也不写 SubagentStore。
4. 无 Desktop sink 的 parallel fallback不得改走持久 task；复用临时执行器或明确失败。
5. 父 JSONL继续只保存外层 parallel call/result；聚合结果必须足以恢复卡片。
6. 不以额外 Transcript hide 条件替代 reducer 分区。

## 验收映射

- R-01、R-03、R-05：普通 task 不串页且切换/重载一致。
- R-08、R-09：parallel 一张聚合卡、重载无分支历史。
- R-14：专家多成员并行未来由多个 task 实现，不依赖 parallel UI。
- R-23：UI 可见性不改变 evidence 边界。

## 聚焦验证

```powershell
go test ./internal/agent/...
Push-Location desktop/frontend
pnpm build
Pop-Location
go run ./scripts/code_guard
git diff --check
```

新增聚焦 frontend 脚本必须覆盖：child 首事件隔离、空 assistant 不显示、task card parent 内容、parallel
实时分支状态、重载聚合和无 AgentWorkspace 分支 tab。

## 完成回执

- 状态：实现与聚焦验证完成，随本任务提交交付。
- 变更文件：Root 修改 `event`/`eventwire`、`task.go`、`parallel_tasks.go`及相邻测试；Frontend
  修改 wire/session types、event/history projection、state/reducer、stream batch、Transcript、ToolCard、
  App 接线、样式及相邻测试；本任务回执与 `progress.md`同步更新。
- 实现结果：持久 task child 的首个 Text/Reasoning/Message/Tool 事件携带 `subagentRef`，child tool
  同时保留 `sourceToolId`；reducer 将其写入 `childItemsByRef`，父 `items`不再接收 child 内容。
  `parallel_tasks`只转发分支聚合 usage，由 synthetic branch 状态写入 `parallelItemsByCall`；ToolCard
  以行列表展示实时分支，重载只剩父聚合结果，不再创建 AgentWorkspace、头像或历史入口。
- 自动化结果：`go test ./internal/agent/... ./internal/event ./internal/eventwire`通过；新增隔离测试
  16/16、stream batch 16/16、AgentWorkspace/Transcript 107/107、ToolCard 21/21、history 21/21、
  transcript grouping 16/16；`pnpm build`、`go run ./scripts/code_guard`和`git diff --check`通过。
- 已知基线：额外执行 `pnpm test:typecheck`时，仓库现有 `installer-contract.test.ts`重复变量与
  `model-marketplace.test.tsx`缺 `pricingUrl`导致失败；生产 `pnpm build`类型检查通过，本任务未修改
  这两个文件。
- 偏差与风险：未实现 child 归档工具详情查询或专家头像历史入口，按任务 004 保留；实时 child
  buffer只用于投影隔离，持久 child JSONL仍是重载来源。未改变 parallel 临时 Session或父聚合回执。
- 交接：任务 003可开始持久专家团状态和成员 ref绑定；任务 004随后消费本任务的
  `childItemsByRef/subagentRef/sourceToolId`契约。
