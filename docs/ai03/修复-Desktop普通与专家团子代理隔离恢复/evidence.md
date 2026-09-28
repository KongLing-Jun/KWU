# Desktop 单人聊天与专家团聊天子代理调查证据

## 1. 范围与基线

- 产品结论见 [requirements.md](requirements.md)；本文只记录可复查事实和直接推论。
- Codegraph 索引工作区：`D:\Reasonix`。
- 已索引并复核的基线：`origin/main-v3@b64ede81b89cd77a0cdd946a26fbf45abdd77e77`。
- 文档 worktree：`D:\Reasonix-worktrees\plan-agent-session-turn-runtime`。
- Codegraph 只用于主工作区；worktree 在同步基线后用直接源码读取、Git diff 和检查复核。
- 用户提供的截图、父/child JSONL 和真实应用复现只用于核验现象；不复制私有 prompt 或长日志。

## 2. 实际现象证据

### 2.1 普通 task 串页与重复内容

用户提供的父 Session、child Session 和截图共同证明：

- `task`确实创建了独立 child JSONL；其中有自己的 reasoning、assistant 和内部工具调用。
- 父 JSONL 只持久化外层 task call/result，没有保存 child 内部工具事件。
- 运行时 child 内容通过事件流出现在父页面，切换/重载后又不能从父 JSONL 恢复。
- 截图中看似重复的内容不是三份相同持久消息：可能依次是 child final、readiness 拒绝前的父候选
  final、恢复后的父 final；`turn_started`空 assistant 占位和工具轮次会进一步打乱视觉顺序。

直接结论：普通 task 已有独立存储，缺陷主要在实时投影和父候选回复显示，不是“父 Session 没有
保存 child 内部历史”。

### 2.2 专家团 parallel 切换后消失

真实 Desktop 复现：专家团用 `parallel_tasks`实时拉起多个成员后，当前页面能看到成员/分支；切换
父会话再返回，刚才的入口和历史消失。重启也没有可加载的 child JSONL。

直接结论：这与 parallel 的内存 Session 路径一致。它不是已实现但漏加载的持久历史；当前前台
parallel 根本没有 ref/JSONL 可恢复。

### 2.3 WorkBuddy 交互观察

此前通过真实应用只读操作确认：

- 子专家历史在同一父任务壳内切换主内容区，侧栏仍选中父会话；
- 子专家页只读，可返回主页面；
- 清除专家团后基础身份恢复，但团队标识、头像和历史入口保留；
- 已清除团队的会话不能重新召唤专家/团队，需要新会话；
- 清除单专家后可恢复基础身份并在原会话重新召唤；
- Composer 内召唤作用于当前会话，全局目录召唤进入新会话草稿。

这些事实只支持 UI 与状态需求。Reasonix 内部继续使用自己的 task/SubagentStore，不推断
WorkBuddy 的 Runtime 或存储。

## 3. task 的当前持久链路

Codegraph 与源码核验 `internal/agent/task.go`：

- `TaskTool.Execute`解析 fresh/continue 和 `run_in_background`。
- foreground 调用 `prepareTranscriptRun`，通过 `SubagentStore.PrepareFresh/PrepareContinue`获得
  ref 与 Session，执行 `RunSubAgentWithSession`，再 `SaveCompleted/SaveFailed`。
- background 走 jobs manager，调用 `MarkRunning`并异步保存终态。
- `FormatSubagentRunResult`把 ref 与 final answer 返回父工具。

核验 `internal/agent/subagent_store.go`：

- child 使用现有 `sa_*.jsonl + sa_*.meta.json`。
- meta 保存 ParentSession、ParentToolCallID、model/tool schema 等校验信息和状态。
- store 提供 fresh/continue、List/Delete、stale cleanup 和 ref 锁。
- 当前 `SaveCompleted/SaveFailed`主要在终态保存；正常 foreground 并不先 MarkRunning。
- 当前 continuation 校验拒绝 running、failed、interrupted，和“终态可继续”的需求不一致。

直接结论：Session、ref、所有权和 continue 已存在；需要统一生命周期与增量保存，不需要新 Runtime。

### 3.1 read_only_task 的既有边界

核验 `internal/agent/task.go:ReadOnlyTaskTool`：

- 描述明确称其为 isolated、ephemeral Session，只返回 final answer；
- schema 不包含 `continue_from`、`fork_from`或 `run_in_background`；
- `Execute`调用 `runReadOnlySubSessionWithA2A`，不进入 `PrepareFresh/PrepareContinue`或 SubagentStore；
- 使用 `ReadOnlySubagentToolRegistry`并继续禁止写入、后台 shell 和代理委派。

直接结论：把 read_only_task 改造成持久 Session 会推翻一项已正常工作的安全/存储边界。它应保留
为普通聊天的一次性研究工具；专家成员只使用持久 task，需要只读时限制 task 的工具白名单。

## 4. 基线前台/后台 task 分裂与后续身份复用

`TaskTool.Execute`的 foreground 分支阻塞到 child 结束，background 分支立即返回 job ID。通用
`bash_output`、`kill_shell`、`wait`同时服务 bash 与 task：

- `internal/tool/builtin/bgjobs.go`的公开 schema 以 `job_id`为控制身份；
- `kill_shell`调用 `jobs.Manager.KillForSession`；
- `wait`按 job ID 等待并返回最终输出；
- `internal/jobs/jobs.go`已具备按父 Session 限定的取消机制。

直接结论：底层取消/等待能力可以复用，但job ID不能替代“子代理身份”；统一
`task/wait_task/stop_task`应继续以`subagentRef`控制Session。任务001已完成该收敛。任务015进一步
复用每次持久job ID作为不透明`task_run_id`，只表达一次turn的依赖和审计；它不替代ref、不开放通用
job控制，因此不重新引入这里否定的Session割裂。

## 5. parallel_tasks 是临时批量执行器

Codegraph 与源码核验 `internal/agent/parallel_tasks.go`：

- 有 Desktop event sink 时，每个分支直接 `NewSession("")`并调用 `RunSubAgentWithSession`。
- 该路径没有 `PrepareFresh`、SubagentMeta、ref 或 child JSONL。
- 分支 synthetic ParentID 用于实时嵌套和卡片，并不提供持久身份。
- 无 event sink 的 fallback 会改走 background `TaskTool`，反而产生持久 child；同一工具因运行环境不同
  出现两种数据语义。

直接结论：把 parallel 定义为临时 fan-out 与主路径一致；需要消除 fallback 的持久化歧义，并把
父聚合结果做完整，而不是给每分支补 ref。

## 6. 专家团当前为何依赖 parallel

核验 `internal/agent/expert_team_tools.go`：

- expert orchestration wrapper 当前注册 task、read_only_task、parallel_tasks 和 wait。
- `withExpertTeamTaskMemberSchema`删除 `continue_from`并增加白名单 `member_name`。
- parallel 的每个 item 也要求 `member_name`。
- 当前成员身份主要注入 prompt/A2A；没有持久的 member-to-ref 绑定。

删除 `continue_from`本身是合理信任边界：模型不应选择任意 ref。缺口是宿主没有根据 member_name
保存和解析可信 ref。后续计费源码复核确认，持久 child turn 必须由拉起它的父 turn 持有；因此专家
成员持久任务按 `task -> wait/stop`顺序执行，临时并行才使用不产生 ref 的 `parallel_tasks`。
同理，read_only_task 不产生 ref，无法成为连续成员历史；专家工具集应只保留持久 task 及其
wait/stop 控制。

## 7. 专家身份与团队状态当前无法恢复

核验 `internal/agent/save.go`和 Desktop Controller：

- 当前 `ExpertMeta`主要保存正在使用的 ExpertID/Name/Source。
- `SaveSessionExpert`在 ExpertID 为空时删除 sidecar。
- `ClearExpert`重建基础 prompt 后清空当前选择。
- sidecar 没有 `team_active/team_archived`、成员显示快照或成员 ref 绑定。

直接结论：“基础身份 + 历史团队仍可查看”不能由当前 ExpertID 单字段表达。最小改动是扩展既有
sidecar，把当前身份和团队工作空间分开，而不是创建成员服务。

## 8. child 消息串入父页面的源码路径

核验 `internal/agent/task.go:subSinkForContext`、事件 wire 和前端 projection：

- child Text、Reasoning、Message、Tool 被转发到父 Desktop sink，并带 ParentID。
- child tool ID会被父调用 ID 命名空间化。
- `eventProjection.ts`把父/child assistant 维护在同一 `items/currentAssistantByParent`。
- `Transcript.tsx`会把 parented tools 嵌套，但普通 parented assistant 仍可能按顶层 assistant 渲染。
- 专家 `AgentWorkspace`只是对同一批 items 进行另一层分组/隐藏。

直接结论：当前普通和专家页面的本质都是“先把父/child 全放进父 state，再按视图筛”。正确隔离点
是投影入口的来源路由，而不是继续增加 `parentId`隐藏条件。

## 9. parallel 的当前前端显示与重载不一致

核验 `ToolCard.tsx`、`AgentWorkspace.tsx`和 `historyProjection.ts`：

- 实时时外层 parallel 卡可以用 synthetic child task 打开成员/分支 tab。
- deeper child 数据并没有稳定 ref，部分 subcalls 也未完整传入。
- 重载时父 JSONL 只有外层 parallel 结果，无法恢复这些临时 tabs。

直接结论：若 parallel 保持临时，成熟的稳定 UI 应是一张可从父结果恢复的聚合卡；运行中 tab/history
只是不可恢复的错觉，应移除。

## 10. child 工具详情为空的源码路径

核验 `compactArchivedToolItems`、`ToolCard.tsx`和 `desktop/app.go`：

- compact 会截断 args、清除 output，并标记 `dataArchived`。
- 展开时 `ToolCard`调用 `ToolResultForTab(tabId, toolId)`。
- `ToolResultForTab`只查询父 tab 的 Controller/Session。
- child raw tool ID实际位于 child JSONL；实时 ID又被 namespace。
- Desktop 已有 `SubagentHistoryForTab/SubagentHistoryPageForTab`的父所有权与 child 加载路径。

直接结论：需要 child 专用的 ref + raw ID 路由并复用现有授权。外层 task prompt 因隐私被隐藏与
child 内部工具详情查错 Session 是两个不同问题。

## 11. final/readiness 与三次相似回复

核验 `internal/agent/agent.go`、`stream_dispatcher.go`、`completestep.go`和父/child样本：

- child final 会作为 child Message 通过 sub sink 实时投影，但只作为外层 task result 持久化到父。
- 父 Agent 可能先形成候选 final，随后被 final readiness 拒绝并继续工具轮次。
- 恢复后父 Agent 再形成最终 final。
- 当前 UI 会创建 turn_started assistant 占位，排序/配对依赖工具轮次，因此候选和 child final 容易
  看成主 Agent 连续回复。

直接结论：readiness 拒绝是正确保护；缺陷是 child/候选的展示边界和完成回执不清晰。

## 12. Evidence 信任边界

核验 evidence ledger 与 `subagentArtifactHandoff`：

- 父与 child 各自维护 ledger；child 工具在 Desktop 可见不会自动进入父 ledger。
- 宿主已有观察 child artifact 的 handoff 能力。
- final readiness只读取父侧 receipt。

直接结论：父不信任 child 内部 receipt 是合理边界。产品缺口是 `wait_task`缺少统一结构化回执；
解决方案是附加宿主观察事实，而不是复制 child ledger 或解析 child final。

## 13. fork / rewind 的现有边界

核验 Controller：

- rewind 替换父 Session 到检查点，不遍历 child JSONL。
- fork 创建新父 Session并复制选定父消息/元数据。
- SubagentStore 当前支持祖先 ref 的 continuation 懒复制，但这是能力，不是专家团必须采用的产品语义。

父 task result 已经包含子任务总结。用户确认本轮只要求父历史 fork/rewind，因此无需设计 child
分段复制；新时间线 fresh child 是更简单且不制造历史连续性错觉的边界。

## 14. 回收站已有父子文件生命周期

核验 `desktop/app.go`和 `desktop/sessions.go`：

- `DeleteSession`的实际产品语义是移入本地回收站；移动前通过 `BeginDestroySession`取消父会话
  background jobs，等待收口后再移动文件；
- `trashSessionArtifacts`调用 `trashSubagentArtifacts`，把属于该 ParentSession 的 child JSONL/meta
  一起移入该父会话的 trash item；
- `RestoreSession`经 `restoreTrashedSessionFile/restoreSubagentArtifacts`一起恢复父/child；
- `PurgeTrashedSession`删除整个 trash item，因此永久删除父文件及其中全部 child 文件。

直接结论：需求无需新增回收站存储模型，只需让新的统一 task job/状态接入现有 teardown，并测试
running child 被标 interrupted 后与父文件一起移动、恢复或永久清理。

## 15. Task Tree 与单层代理

- Task Tree 当前管理 depends_on、DAG 校验、节点状态、进度和输出关联，没有 Session 身份合同。
- `SubagentMetaTools()`及 child Registry 已排除 task、read_only_task、parallel_tasks 等 Agent 创建
  工具；后台 shell schema 也在 child 中受限。

直接结论：Task Tree 有价值但不参与成员绑定；最大委派深度一层应继续保持和测试。

## 16. 总结

当前源码已经具备可复用的持久 child Session、continue、所有权校验、jobs 取消、历史分页、专家
选择、Task Tree、artifact handoff 和单层 Registry。需要修复的不是“专家团本身”，而是五个接口：

1. task 的异步控制与增量持久化；
2. 父/child/parallel 的事件投影隔离；
3. 专家团状态与 member-to-ref 恢复；
4. child 工具详情的正确 Controller 路由；
5. child 结果到父 readiness 的结构化回执。

同时保留两条已有边界：read_only_task 是一次性只读研究，回收站负责父/child 文件整体移动、恢复
与永久删除。

因此 [plan.md](plan.md) 只扩展现有机制，并明确保留 `parallel_tasks`的临时批量边界。
