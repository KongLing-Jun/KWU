# 验收发现问题：B2B 模式下专家团 task 调度死循环

## 1. 文档地位

本文记录 Serial-01 至 Serial-05 修复完成后，真实 Desktop 验收中发现的一个实际缺陷。只记录证据和根因，
**不包含解决方案**。任何修复必须先回到 `requirements.md` / `contracts.md` 评审是否属于本主题范围，
或作为独立缺陷处理。

后续独立源码复核更正：这里的“B2B”是历史函数命名，严格 Desktop 计费路径同时覆盖
`subscription_pool`和`newapi_byok`。基线明确允许父任务内 foreground subagent，并只拒绝真正可能
跨父任务存活的 background subagent。用户已确认修复边界：保留父 turn 内异步 task，child ModelCall
绑定拉起它的父 `ModelTask`且不跨 Provider，父 final/结算前必须 wait/stop。

**修复状态（任务006）**：代码与自动化验证完成，自定义 Provider 的普通/专家 `task → wait_task`
真实 Desktop 复验已通过；subscription 与 stop 路径待补。普通持久 task 不再
走 legacy background gate；child job 继承父 turn 的 TaskContext/取消和计费 barrier，父 final 前必须
wait/stop。Root 受影响包与 boot 装配聚焦测试通过。

## 2. 问题描述

### 2.1 现象

用户在 Desktop 选择"代码交付专家团"后发送"帮我做一个贪吃蛇"。主专家（lead agent）尝试把文件写入
任务委派给子专家"代码审查专家"，调用 `task` 工具，但每次调用都立即返回错误：

```
contract_invalid: Desktop strict/fake background subagent is disabled
```

主专家调用 `task` 共 5 次，全部返回相同错误；中间穿插尝试 `write_file`（报 `unknown tool`）
和 `bash`（报 `unknown tool`）。loop guard 在第 3 次相同错误后给出警告，但模型未改变策略，
继续重试 2 次，最终 `context canceled`。

### 2.2 用户影响

- 用户无法得到交付物（贪吃蛇游戏代码未生成）。
- 主专家陷入死循环，消耗 turn 预算但不产生任何有效输出。
- 专家团头像图标正常显示，给用户"可以工作"的错误预期，实际无法调度任何子专家。

## 3. 复现方法

### 3.1 前提条件

- Desktop 使用严格计费路由（`applyDesktopB2BOptions`历史命名函数完成装配）。
  - `subscription_pool`和`newapi_byok`都可能进入该路径。
  - **反向确认方法**：如果 `task` 调用返回 `contract_invalid: Desktop strict/fake background
    subagent is disabled`，则当前 Controller 已装配 background gate（见 5.1.2-5.1.3）。
- 已配置"代码交付专家团"，成员含"代码审查专家""测试策略专家""发布风险顾问"。

### 3.2 复现步骤

1. 在 Desktop 使用严格计费路由时，选择"代码交付专家团"。
2. 向主专家发送一个需要写文件的任务，例如"帮我做一个贪吃蛇"。
3. 等待主专家开始 turn。

### 3.3 预期 vs 实际

- **预期**：主专家通过 `task` 委派子专家，子专家写文件，主专家汇总交付。
- **实际**：`task` 调用返回 `contract_invalid: Desktop strict/fake background subagent is disabled`，
  主专家无 `write_file` 等直接执行工具，反复重试 `task` 直到 turn 耗尽。

### 3.4 验证主 agent 工具集

对话导出确认专家团主 agent 在该模式下只有以下 7 个工具：

```
ask, complete_step, stop_task, task, todo_write, verify_artifact, wait_task
```

没有 `write_file`、`bash`、`read_file`、`edit_file` 等任何直接执行工具。这是专家团模式的预期约束
（主 agent 不直接执行具体任务），但在 `task` 被全面拒绝时，主 agent 没有任何 fallback 路径。

## 4. 证据

### 4.1 对话导出与环境

- 对话导出：`C:/Users/ai03/Downloads/Global - 帮我开发一个贪吃蛇.md`。
- 截图：`clipboard-2026-08-07T03-22-40-969Z-9d3bb27a.png`（专家头像图标已拉起）。
- worktree 基线：`origin/main-v3@b64ede81b89cd77a0cdd946a26fbf45abdd77e77`，分支
  `codex/plan-agent-session-turn-runtime`。
- 专家团配置：代码交付专家团，可调用成员含"代码审查专家""测试策略专家""发布风险顾问"。
- 模型：用户提到换了模型后开始 turn，具体型号未记录。

### 4.2 错误序列（摘自对话导出）

主专家在单个 turn 内的工具调用序列：

| 序号 | 工具 | 结果 |
| --- | --- | --- |
| 1 | `write_file` | `unknown tool "write_file"` |
| 2 | `task`（member_name=代码审查专家） | `contract_invalid: Desktop strict/fake background subagent is disabled` |
| 3 | `task`（member_name=代码审查专家） | 同上 |
| 4 | `bash` | `unknown tool "bash"` |
| 5 | `task`（member_name=代码审查专家） | 同上；此后 loop guard 触发警告 `task failed 3× the same way` |
| 6 | `todo_write` | 失败（2 todos completed 但无 complete_step 回执） |
| 7 | `task`（member_name=代码审查专家） | 同上 |
| 8 | `task`（member_name=代码审查专家） | 同上 |
| — | `ask` | 用户操作后 `context canceled` |

`task` 共调用 5 次，全部返回相同错误。loop guard 在第 3 次后触发，模型仍继续重试 2 次。

## 5. 根因分析

本节按确定性分为三部分：**已确认**（有代码行号或 diff 证据，可独立复查）、**推断**（基于已知信息
合理推导，但未代码确认）、**促成因素**（不是直接根因，但导致死循环的必要条件）。

### 5.1 已确认部分

#### 5.1.1 Serial-01 改变了 `validateExecution` 的 gate 条件

`git diff origin/main-v3...HEAD -- internal/agent/task.go`：

```diff
-func (t *TaskTool) validateExecution(ctx context.Context, prompt string, background bool) error {
+func (t *TaskTool) validateExecution(ctx context.Context, prompt string) error {
    if prompt == "" {
        return fmt.Errorf("prompt is required")
    }
-    if background && t.backgroundPolicy != nil {
+    if t.backgroundPolicy != nil {
        return t.backgroundPolicy(ctx)
    }
    return nil
```

调用点 diff：

```diff
-	if err := t.validateExecution(ctx, p.Prompt, p.RunInBackground); err != nil {
+	if err := t.validateExecution(ctx, p.Prompt); err != nil {
```

**确认结论**：Serial-01 统一异步化移除了 `background` 参数，gate 条件从"仅 `run_in_background=true`
时拒绝"变为"`backgroundPolicy` 非空时无条件拒绝"。当前代码 `internal/agent/task.go:446-454`。

#### 5.1.2 `RejectBackgroundSubagents` 在严格 Desktop 计费路由设置

`desktop/model_billing.go:1658-1690`：

```go
func (a *App) applyDesktopB2BOptions(options *boot.Options, ...) error {
    ...
    if !policy.enableB2B {
        return nil  // ← 非 B2B 模式直接返回，不设置 RejectBackgroundSubagents
    }
    ...
    options.RejectBackgroundSubagents = true  // ← 仅 B2B 模式执行到此
    ...
}
```

Grep 全仓库确认，`RejectBackgroundSubagents = true` 只在 `applyDesktopB2BOptions` 一处设置
（基于当前 worktree `codex/plan-agent-session-turn-runtime` 分支）。

**确认结论**：`subscription_pool`和`newapi_byok`启用严格 Desktop 计费时都会设置该门禁；它的原始
语义是拒绝跨 turn background，而不是拒绝父任务内 child 调用。

#### 5.1.3 gate 注入与错误消息来源

`internal/boot/boot.go:636-653`：

```go
newTaskTool := func() *agent.TaskTool {
    task := agent.NewTaskTool(execProv, entry.Price, reg, maxSteps,
        entry.ContextWindow, ..., headlessGate,  // ← NewTaskTool 已传入 headlessGate 作为初始 policy
        keepPolicy, taskModel, taskEffort, resolveSubagentProvider).
        WithToolTimeout(toolTimeout).
        WithTranscripts(...).
        ...
    if opts.RejectBackgroundSubagents {
        task.WithBackgroundPolicy(func(context.Context) error {  // ← 覆盖 headlessGate
            return &modelbilling.Error{
                Code:    modelbilling.ErrorContractInvalid,
                Message: "Desktop strict/fake background subagent is disabled",
            }
        })
    }
    return task
}
```

`WithBackgroundPolicy`（task.go:287-290）直接修改指针接收者的 `backgroundPolicy` 字段，返回值是否
接收不影响生效。

**确认结论**：用户看到的错误消息 `"Desktop strict/fake background subagent is disabled"` 来自
boot.go:650，只有在 `RejectBackgroundSubagents=true` 时注入。

### 5.2 已补充确认部分

#### 5.2.1 用户 Desktop 实际处于严格计费路径

**推断依据**：
1. 用户看到错误 `"Desktop strict/fake background subagent is disabled"`。
2. 该错误只在 `RejectBackgroundSubagents=true` 且 `backgroundPolicy != nil` 时产生（5.1.3 已确认）。
3. `RejectBackgroundSubagents=true` 只在 `applyDesktopB2BOptions` 中设置（5.1.2 已确认）。
4. 因此推断用户 Desktop 实际调用了 `applyDesktopB2BOptions`，即处于 B2B 模式。

错误文本足以确认 Controller 由严格计费路由装配；具体 BillingSource 仍应在复验时从计费记录核对，
但不影响无条件 background gate 是直接根因。

#### 5.2.2 基线 foreground task 在严格计费模式下是受支持路径

**推断依据**：origin/main-v3 的 `validateExecution` 有 `background` 参数，只有 `run_in_background=true`
时才触发 gate。foreground task（`run_in_background=false` 或缺省）不受影响。

基线测试 `TestDesktopForegroundSubagentPersistsIndependentCallInParentTask`确认 child call 与父 call
持久化在同一父计费任务；background 拒绝测试则确认跨 turn background 在发网前失败。

### 5.3 促成因素（非直接根因，但导致死循环）

以下因素不是 `contract_invalid` 错误的直接原因，但它们使错误升级为死循环：

1. **专家团主 agent 没有写文件工具**：专家团模式约束主 agent 只能调度子专家，不能直接执行。
   当 `task` 被全面拒绝时，主 agent 没有任何替代路径完成任务。
2. **模型缺乏 fallback 策略**：主专家在 `task` 失败后没有转向"告知用户当前模式不支持子代理调度"，
   而是反复重试相同调用。
3. **loop guard 未强制终止**：loop guard 在第 3 次相同错误后只给出警告（`nudging the model to change
   approach`），没有强制终止 turn 或阻止后续相同调用，模型继续重试 2 次。

## 6. 影响范围

### 6.1 直接受影响场景

- **B2B 模式 + 专家团**：所有 `task` 调用被拒，主 agent 无法委派，无法自行写文件，死循环。
- **B2B 模式 + 普通 chat 调用 task**：同样被拒，但普通 chat 主 agent 通常有 `write_file`
  等工具（未验证 Registry 配置），可以自行完成，不会死循环。

### 6.2 不受影响场景

- 非 B2B 模式：不经过 `applyDesktopB2BOptions`，`RejectBackgroundSubagents` 不设置，
  `backgroundPolicy` 为 nil（或保持 `headlessGate` 初始值），`validateExecution` 不返回
  `contract_invalid`。Serial-01~05 真实 Desktop 验收（任务 005 矩阵）覆盖的就是这一路径。
- headless / 非 Desktop 路径：不经过 Desktop 计费装配，不设置 `RejectBackgroundSubagents`。

### 6.3 与 R-01 至 R-26 的关系

本问题属于 R-02/R-14/R-23 的父 turn 生命周期和计费收口缺陷，已纳入任务006；严格计费下普通与专家
task 都必须是支持路径。

## 7. 既有测试为何未发现

### 7.1 单元测试

- `go test ./internal/agent/...` 在普通模式下运行，`RejectBackgroundSubagents` 默认 false，
  `backgroundPolicy` 为 nil，`validateExecution` 不触发拒绝路径。
- `go test ./internal/boot/...` 的 boot 测试不覆盖 `RejectBackgroundSubagents=true` + 专家团
  wrapper 的组合。

### 7.2 Desktop 计费测试

`desktop/model_billing_test.go:1973,2412` 验证了 `opts.RejectBackgroundSubagents` 为 true，
并断言错误消息包含 `contract_invalid` 和 `background subagent is disabled`，同时确认 spy 计数为零
（零网络）。但这些测试只验证 gate 在单次 task 调用上的拒绝行为，**没有验证专家团主 agent 在
gate 拒绝后是否能给出有效 fallback，也没有验证 loop guard 是否能终止死循环**。

### 7.3 真实 Desktop 验收

Serial-05 真实 Desktop 验收矩阵（任务 005 回执）全部在 `gpt/gpt-5.6-luna` 正常 Provider 模式下
执行，未覆盖 B2B strict/fake 模式与专家团的组合。

## 8. 待解决问题声明

本问题已按任务006实现 turn-scoped job、父 `TaskContext`、父取消和 final/结算门禁；未扩展为 loop
guard 重构，也未通过禁用专家团规避受支持的 task 调度。最终关闭问题仍以 subscription 与 stop
真实 Desktop 复验为准。
