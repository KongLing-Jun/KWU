# 任务 007：恢复多 task 并发与精简专家团提示词

## 状态与依赖

- 状态：已完成实现、自动化验证和真实 Desktop 验收。
- 执行组：Serial-07；由当前主 Agent 执行，不委派子代理。
- 依赖：任务001至006、自然任务验收记录，以及用户确认的多 task 和提示词边界。
- 未授权 push、PR 或合并；按文档合同与运行时实现两个 coherent intent 小步提交。

## 目标

1. 移除父 turn 只能运行一个持久 `task`的过度保守门禁，允许最多四个不同 child Session 并发。
2. 保持同一 `subagentRef`最多一个 running turn，保留父取消、计费 barrier、final readiness 和回执闭环。
3. 专家团继续只复用现有 `task / wait_task / stop_task`；由模型判断独立任务并行或依赖任务串行。
4. 把通用专家团运行合同收敛到 Root Go 的 Session system prompt，移除 Frontend 每回合重复规则。

## 源码依据

- `internal/agent/managed_task.go`的父级 `HasRunningKindForSession(..., "task")`是第二个 task 被拒绝的
  直接原因；它不区分 ref，也没有并发计数。
- `internal/agent/subagent_store.go`已对同一 ref 的 running turn 加锁并明确拒绝并发续写。
- `internal/jobs`已按父 Session、kind 和 control ID 管理多个 job；`wait_task`和专家 wrapper 已支持
  一次等待多个 ref/member。
- 父 `foregroundCallBarrier`可登记多个 child lease，`runningTaskFinalReadiness`按任意 running task
  阻止 final；无需新建计费或任务 Runtime。
- `internal/expert/definition_catalog.go`生成通用团队 system prompt；云端目录只提供团队/成员资产。
- `desktop/frontend/src/lib/expertPrompt.ts`仍在每回合隐藏块重复 task/parallel/A2A 编排规则，但
  `member-names`属性仍是专家工具路由所需元数据。

## 允许范围

- `internal/agent`和`internal/jobs`中持久 task 并发计数/门禁及聚焦测试。
- `internal/expert/definition_catalog.go`中通用团队 system prompt 及测试。
- `desktop/frontend/src/lib/expertPrompt.ts`移除重复运行时正文，保留成员白名单与语言元数据，并更新测试。
- 本任务回执、需求/计划/合同/进度和自然验收问题状态。

## 禁止范围

- 接入 Task Tree、planner、运行时排队、批量批准 UI 或第二套子代理调度。
- 把 `parallel_tasks`作为专家成员调度或持久成员历史来源。
- 允许同一 ref 并发写 Session，允许 child 跨父 turn，或放宽父 final/计费收口。
- 修改云端专家资产、Provider/计费协议、同成员 tool scope、成员栏视觉和状态点。
- 用户直接与 child 对话、孙代理、迁移旧数据或新增顶层 Session 类型。

## 实现合同

- 一个父 turn 最多四个 running 持久 `task`；第五个调用返回可操作错误，提示先 wait/stop。
- 不同 ref 可以并发；同一 ref 继续由 SubagentStore 锁拒绝第二个 running turn。
- 每个 child 分别登记父生命周期 lease；父取消停止全部，父 final/结算等待全部终态。
- `wait_task`可一次等待多个 ref/member；模型根据依赖关系决定启动顺序，宿主不实现 DAG 或队列。
- 专家团 system prompt 简洁声明：主专家负责理解、按需委派、整合；具体工作至少委派一名成员；
  独立任务可并发、依赖任务串行；最多四个、同成员一个；用 wait/stop 收口；完成前不得 final，并
  保持 complete_step/verify_artifact 合同。
- Frontend 隐藏块保留成员白名单与语言路由，不再重复上述运行时规则，也不提
  `read_only_task`、`parallel_tasks`、A2A 或内部 ID。

## 验收标准

- 同一父 turn 启动两个至四个 fresh/different-ref task 均返回 running，能够一次 wait 并各自完成。
- 第五个并发 task 在创建 child/job 前被拒绝；任一任务收口后可以再启动一个。
- 同一 ref 的第二个 running turn仍被拒绝；父取消和 stop 不误伤其他父 Session。
- 任一 child running 时父 final/结算仍被阻止，全部收口后恢复；多个 child 仍归属同一父 ModelTask。
- 专家团模型只收到一份通用运行合同，Frontend 仍能按精确 member name 路由。
- 普通 task、parallel 临时聚合、专家历史恢复和 child transcript 隔离不回归。

## 聚焦验证

```powershell
go test ./internal/jobs ./internal/agent ./internal/expert
pnpm exec tsx src/__tests__/expert-prompt.test.ts
pnpm build
go run ./scripts/code_guard
git diff --check
```

真实 Desktop 使用自然目标验收两种自主路径：可独立的代码审查/测试任务应允许并发并统一等待；明确
依赖前置产物的任务应先等待前一成员，再启动后一成员。Prompt 不写工具名、成员名或固定步骤。

## 完成回执

- 状态：已完成。
- changed files：
  - `internal/jobs/turn_jobs.go`、`internal/jobs/jobs_extra_test.go`：增加按父 Session/kind 的 running
    job 计数，并验证父级、kind 和终态隔离。
  - `internal/agent/managed_task.go`、`internal/agent/task.go`、`internal/agent/task_test.go`：把父级单
    task 门禁改为最多四个；第五个在创建 child transcript/job 前拒绝；验证四个 child 可并发并由
    一次多 ref `wait_task`收口。
  - `internal/expert/definition_catalog.go`、`internal/expert/expert_test.go`：收敛通用团队 system
    prompt，保留委派、并发/依赖、wait/stop、final、complete_step 和 artifact 合同。
  - `desktop/frontend/src/lib/expertPrompt.ts`、`desktop/frontend/src/__tests__/expert-prompt.test.ts`：
    团队每回合隐藏块只保留激活标记、精确成员名和语言元数据，不再重复运行时提示词。
- 自动化验证：
  - `go test ./internal/jobs ./internal/agent ./internal/expert`通过；
  - `go test ./internal/boot -count=1`通过；
  - `go test ./internal/control -count=1`通过；
  - `pnpm exec tsx src/__tests__/expert-prompt.test.ts`通过，46/46；
  - `pnpm exec tsx src/__tests__/expert-contract.test.ts`通过，47/47；
  - `pnpm build`通过，仅保留既有 chunk size warning；
  - `wails build -m -nosyncgomod -nopackage`通过，产物
    `desktop/build/bin/kwu-desktop.exe`，时间 `2026-08-07 20:12:46`；
  - `go run ./scripts/code_guard`和`git diff --check`通过；全部 changed files 为 LF。
- 真实 Desktop 独立并发场景：
  - 使用上述新构建、`gpt/gpt-5.6-luna`和自然 prompt“这个贪吃蛇小游戏准备交付了，帮我从实现质量
    和测试可靠性两个角度把关，发现必要问题就处理。”；没有写工具名、成员名或步骤。
  - 父 Session `20260807-121418.246063500-gpt-gpt-5.6-luna`在同一父 turn 依次返回三个 running：
    `sa_20260807_121708_000000000_5d0f9a0e7e89`（代码审查）、
    `sa_20260807_121714_000000000_f67ff496cdf3`（测试策略）、
    `sa_20260807_121718_000000000_cfdea8cd7e48`（发布风险），随后用一个 `wait_task`等待三位成员；
    未出现旧的“已有一个 persistent task”错误。父页面只显示派发、工作更新、等待与结果，点击头像
    后才进入对应 child 历史。
- 真实 Desktop 依赖串行场景：
  - 自然 prompt“先检查这个贪吃蛇当前实现里最值得担心的质量问题，等审查结论出来后，再根据这些
    具体问题设计针对性的回归测试方案。只做检查，不要修改文件。”。
  - 父 Session `20260807-122125.508752500-gpt-gpt-5.6-luna`只先启动并等待
    `sa_20260807_122221_000000000_fde47c0d2acc`（代码审查）；其 completed 结果返回后才启动
    `sa_20260807_122421_000000000_6d13535d3377`（测试策略）。证明提示词允许模型按依赖选择串行，
    宿主没有强制并发。
- 偏差与剩余风险：第一条自然场景在三路只读任务完成后，模型尝试让既有成员使用写工具修复，仍被
  既有 `subagent reference ... uses a different tool scope`边界拒绝。本任务明确禁止修改同成员
  tool scope，因此已停止验收 turn并记录为下一独立问题；它不影响本任务的多 ref 并发和串行选择结论。

## 后续合同收紧

任务007的完成回执只证明“最多四个不同ref可以并发”以及“模型能够在明确prompt下选择串行”，不证明
模型在开放自然任务中总能正确声明依赖。后续真实验收已发现主专家会把尚无成品的测试/风险建议与实现
任务同时派发，并把建议完成误当成实际验收进度。

[任务015](015-显式约束持久task调度.md)在不撤销本任务并发能力、不增加队列/DAG/第二Runtime的前提下，
进一步要求并行显式`independent: true`，以现有job ID形成`task_run_id`，并由主专家wait、检查、整理后
使用`depends_on`派发下游任务。本文以上内容保留为历史实现与验收回执；当前行为以requirements、
contracts和任务015为准。
