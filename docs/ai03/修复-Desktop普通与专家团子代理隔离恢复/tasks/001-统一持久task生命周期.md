# 任务 001：统一持久 task 生命周期

## 状态与交付

- 状态：已完成。
- 后续更正：验收发现这里实现的 job 使用 Session 级上下文，会被严格计费识别为跨 turn background。
  任务006负责在不推翻异步工具合同的前提下，把它收紧为父 turn 内 job 并绑定父 `ModelTask`。
- 执行组：Serial-01；由当前主 Agent 执行，不委派子代理。
- 依赖：已批准的 [`requirements.md`](../requirements.md)、[`plan.md`](../plan.md)与
  [`contracts.md`](../contracts.md)。
- 交付：一个可独立评审、测试和回滚的 Root Go 提交，包含源码、测试和本任务完成回执。
- 用户已授权任务拆解后连续实现和小步提交；未授权 push 或 PR。

## 目标

把现有 foreground/background `task`收敛为一个始终异步的持久 child-turn Runtime，提供按
`subagentRef`控制的 `task / wait_task / stop_task`，并保证 child Session 增量保存、终态可续写和
进程退出可恢复。

## 预期产物

- `task`立即返回 `{subagentRef, running}`，不再公开 `run_in_background`。
- `wait_task`按 ref 等待/读取当前或最近 turn 的结构化状态。
- `stop_task`按 ref 取消当前 turn，保留 Session 并标记 interrupted。
- completed、failed、interrupted 均可 PrepareContinue；running 明确拒绝新 turn。
- child 消息/工具结果按安全保存边界增量写入现有 JSONL/meta。

## 允许范围

- `internal/agent/task.go`
- `internal/agent/subagent_store.go`
- `internal/jobs/jobs.go`及最小必要的 task job 适配
- Root tool 注册/boot wiring
- 上述包的直接测试
- 本任务完成回执与 `progress.md`

## 禁止范围

- frontend、Desktop 专家 UI、ExpertMeta/team workspace。
- `parallel_tasks`产品语义和 AgentWorkspace。
- Provider 协议、新 Session/TaskRun Store、新数据库或新依赖。
- read_only_task 的一次性临时存储语义。
- child 对用户输入、嵌套代理或 running turn 消息注入。

## 源码依据

- `TaskTool.Execute`当前在 `task.go`内分成同步 foreground 与 jobs background。
- `SubagentStore`已有 fresh/continue、ref 锁、running/completed/failed/interrupted 和 stale cleanup。
- 当前 foreground 不先 MarkRunning，终态保存才写完整 Session。
- 当前 `validateMeta`拒绝 failed/interrupted continuation，需要按需求放开而保持身份校验。
- `jobs.Manager.KillForSession`和 wait 已提供取消/等待底层能力，但公开身份是临时 job ID。

## 实现约束

1. 不新建独立 turn ID；使用父 tool call、ref 和内部 job 关联本次 turn。
2. 任务001交付时，job ID只在宿主内部；模型 schema 和父工具结果以 ref 为身份。
3. 同一 ref 的锁覆盖整个 running turn，禁止并发写同一 child JSONL。
4. task 启动、增量保存、终态保存任一步失败都不得报告虚假成功。
5. stop/父取消/真正进程退出为 interrupted；Provider/tool/timeout/max-step 为 failed。
6. 关闭窗口转后台不停止 task；切换 tab 不停止 task。
7. read_only_task 保持当前 ephemeral、blocking、plan-mode-safe 路径。
8. child Registry 继续排除 task、read_only_task、parallel_tasks、wait/background shell。

> 后续合同：第2条是任务001交付时的控制身份边界。任务015保留ref作为wait/stop/continue身份，但复用
> 每次持久job ID作为不透明`task_run_id`返回，只用于依赖与审计，不开放job控制接口。

## 验收映射

- R-01 至 R-07：普通 task 启动、等待、停止、续写、切换、退出恢复。
- R-15：同一成员未来复用同一 ref 时的并发保护基础。
- R-20、R-22、R-24：普通隐藏 child 的生命周期基础、单层 Registry、read_only 既有边界。
- R-25、R-26 的停止/清理接口基础由本任务提供，文件移动由任务 004 验证。

## 聚焦验证

```powershell
go test ./internal/agent/...
go test ./internal/jobs/...
go test ./internal/tool/builtin/...
go test ./internal/boot/...
go run ./scripts/code_guard
git diff --check
```

按实际改包缩小或扩展，但必须覆盖：schema 不再公开 background、立即返回、wait、stop、状态转换、
增量保存、failed/interrupted 续写、同 ref 并发、stale cleanup、read_only 不变和单层 Registry。

## 完成回执

- 状态：已完成，待本地提交。
- 变更文件：`internal/agent/task.go`、`tool_receipts.go`、`subagent_store.go`、
  `subagent_tool_contract.go`及直接测试；`internal/jobs/jobs.go`、`artifacts.go`及直接测试；
  `internal/boot/boot.go`、`build_agent_tools.go`及装配/审计测试。
- 自动化结果：
  - `go test ./internal/jobs/... -count=1 -timeout=120s`通过；
  - `go test ./internal/agent/... -count=1 -timeout=180s`通过；
  - 在空临时 `APPDATA/LOCALAPPDATA`下执行
    `go test ./internal/boot/... -count=1 -timeout=180s`通过；
  - `go run ./scripts/code_guard`与 `git diff --check`通过，未修改 guard baseline。
- 偏差与风险：Desktop 持久父 Session 已统一异步；无持久父 Session 的 headless/direct 调用继续使用
  既有阻塞临时路径。旧专家 wrapper 私有传入的 `run_in_background`仅保留隐藏兼容分支，公开 schema
  已删除该参数；任务 003 将把专家 wrapper 迁移到正式持久 task 后删除兼容分支。
- 交接：`task / wait_task / stop_task`、ref 到内部 job 的持久关联、running 增量保存、
  interrupted 终态和终态续写基础已就绪；任务 002 可据此隔离 child 事件并收敛 parallel 展示。
