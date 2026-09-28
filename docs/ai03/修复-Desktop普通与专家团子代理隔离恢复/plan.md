# Desktop 单人聊天与专家团聊天子代理修复计划

## 1. 文档定位

[requirements.md](requirements.md) 是唯一产品需求权威。本文只规定如何在现有 Reasonix 源码上实现，
不重新定义产品行为。

配套文档：

- [contracts.md](contracts.md)：最小数据、工具、事件、UI 与安全合同；
- [evidence.md](evidence.md)：当前源码、复现和交互事实；
- [progress.md](progress.md)：阶段、验证和下一关口的当前快照。

## 2. 当前问题

当前源码已经有可用的 `task + SubagentStore + child JSONL`，但多条路径没有共用同一边界：

1. child 事件通过父 sink 实时投影，普通 Transcript 因而出现 child reasoning、消息或 final；
2. `task`同时存在阻塞执行与 `run_in_background`两套模型可见语义，等待/停止又借用通用 job 工具；
3. `parallel_tasks`前台路径使用临时 Session，专家团却把它当成员调度和历史 UI 的来源；
4. 专家团没有持久的状态、成员快照和成员到 `subagentRef`绑定，切换或重载后依赖实时 item 的成员消失；
5. child 工具详情仍通过父 Controller 查询，归档后可能只得到空数据；
6. child ledger 与父 ledger 正确隔离，但父侧缺少统一的 task 完成回执和 readiness 闭环；
7. fork/rewind 与成员复用被旧方案过度绑定，超出了父历史分支本身需要承担的范围。

根因不是缺少“专家团 Runtime”，而是持久 child Session、临时批量执行、父 transcript 和专家 UI
四类职责混在了一起。

## 3. 实现原则

- 复用现有 `Session`、`SubagentStore`、jobs manager、专家 sidecar、事件 wire 和历史分页能力。
- `task`是持久、可续写的单个子代理；`parallel_tasks`是临时批量执行器，二者不得互相模拟。
- 专家团只是 `task`的固定成员绑定和 UI 导航，不建立 Agent/Invocation/TaskRun Store。
- 父/child 事件在投影入口分流，不把 Transcript 过滤当成隔离边界。
- 所有实现都可追溯到 R-01 至 R-34；旧数据不迁移、不猜测、不补造。

### 3.1 Desktop 父 Agent 可见工作更新

在 Desktop 可选择场景的顶层 prompt 中追加一段缓存稳定的本地策略，要求父 Agent 仅在非简单工作
的关键阶段通过既有 assistant `content`向用户说明目的、结果与下一步。策略位于专家/场景片段之后，
因此基础父 Agent、单专家和专家团主专家共用同一合同；`task`继续使用独立
`DefaultTaskSystemPrompt`，不继承该策略。

本项不新增事件、消息类型、Provider 字段或云端专家资源；普通 assistant 消息继续
沿现有实时投影、Session 增量保存和历史恢复链路工作。简单问答可直接作答，连续工具运行期间由已有
工具/task 卡片反馈，不生成合成心跳。

## 4. 选定方案

### 4.1 统一 task 为父 turn 内异步 child turn

保留 `TaskTool`创建/续写 child Session 的核心流程，把阻塞与跨 turn background 分支收敛成一个
受父 turn 持有的内部执行器：

```text
task
  -> PrepareFresh / PrepareContinue
  -> 注册继承父 TaskContext 的 turn-scoped job 与 cancel
  -> 将 child turn 标记 running
  -> 立即返回 {subagentRef, status=running}

后台执行
  -> RunSubAgentWithSession
  -> 增量保存 Session
  -> completed / failed / interrupted

wait_task
  -> 按 ref 或 member_name 等待/读取最终回执

stop_task
  -> 按 ref 或 member_name 定位内部 job 并取消当前 turn
```

具体约束：

- 从公开 `task` schema 移除 `run_in_background`；调用始终立即返回。
- 新增/整理 `wait_task`和 `stop_task`语义层，内部可以复用 jobs manager 的 wait 与
  `KillForSession`；控制仍使用ref/member，但把现有持久job ID作为不透明`task_run_id`写入启动/终态
  回执，用于精确依赖而不新增TaskRun Store。
- `wait_task`不向模型暴露轮询超时；宿主阻塞到目标job终态或父context取消，避免父模型携带完整
  上下文反复查询running状态。
- 同一父 Session 最多四个不同 ref 的 running 持久 task；同一 ref 仍最多一个 running turn。已有任务
  运行时，第二个task必须显式`independent: true`；未声明、依赖仍running或达到上限时返回结构化中性
  调度结果且不创建child。父 final必须等待所有必需任务收口。
- child job 注册父任务生命周期占用，所有 child ModelCall 归属父 `ModelTask`；父取消级联中断，
  child 终态后父任务才可结算。
- 普通聊天返回/接受 `subagentRef`；专家 wrapper 把 `member_name`解析成 ref。
- 同一 ref 的进程内锁覆盖整个 running turn；已 running 时新 turn 明确失败，不排队写同一 JSONL。
- `completed`、`failed`、`interrupted`都允许再次 PrepareContinue；只有 running 阻止新 turn。
- 父取消、stop、真正退出应用进程均收敛为 interrupted；关闭窗口但转入后台不改变任务状态；
  执行错误收敛为 failed。
- 不实现 turn 中途消息注入。

### 4.2 child 增量持久化与恢复

扩展现有 SubagentStore，而不是新增存储：

- turn 启动时写 running meta；
- child Session 每产生可持久事件后按现有 Session 保存边界增量落盘；
- 终态更新 meta，并保存最终 Session；
- stale running 在启动时标 interrupted；
- interrupted/failed Session 可被后续 turn 加载；
- 保存错误不能被外层工具报告为成功。

切换 tab 只释放或重建前端缓存，不影响活动父 turn 持有的 child。Desktop 真正退出进程时先取消相关 job，并
尽力保存已产生历史。父 Session 移入回收站时复用现有 `BeginDestroySession`停止 job，再通过
`trashSubagentArtifacts`把 child 与父文件一起移动；恢复复用 `restoreSubagentArtifacts`，永久删除
复用现有 trash item 清理。

### 4.3 保持 read_only_task 一次性

`read_only_task`继续使用 `task.go`中现有的临时只读路径，不接入 SubagentStore：

- 只在普通/单专家聊天提供，阻塞到一次性研究结论返回；
- 不生成 ref，不支持 continue、wait_task、stop_task 或历史页；
- 专家团 Registry 移除它；子专家继续使用同一个持久 `task`，并始终继承完整的子代理安全工具集；
- 专家 wrapper只做 `member_name -> subagentRef`可信映射并移除逐次 `tools`输入，不复制 TaskTool、
  Registry、模型设置或 SubagentStore；只读调查由 prompt约束用途，不改变成员 Session能力；
- 保持现有只读 Registry、plan-mode safe 与禁止后台 shell/代理委派的边界。

### 4.4 保持 parallel_tasks 临时化

保留当前前台批量 fan-out 的内存 Session 方向，删除或禁止它落入 background task 持久化 fallback：

- 每个分支只存在于该次 parallel 调用期间；
- 运行时事件用于更新外层 parallel 卡的分支状态，不建立 `subagentRef`或 child JSONL；
- 外层结果保存每支状态、摘要和聚合结果，使重载后仍能恢复一张完整卡；
- 移除 synthetic task tabs、AgentWorkspace 分支历史和 deeper child tool UI；
- 普通/单专家 Registry 保留 parallel；专家团 Registry 与 prompt 不提供 parallel。

该改动不是“丢数据修复”，而是把现有临时执行边界做一致：需要长期上下文时必须使用多个 `task`。

### 4.5 持久专家团工作空间

最小扩展父 Session 现有 expert sidecar，保存：

```text
chatMode: normal | team_active | team_archived
currentExpert?                 // 现有基础/单专家/主专家身份
teamWorkspace?
  teamId / teamName
  state
  members[]
    memberKey
    expertId?
    name / role / avatarKey / order
    subagentRef?
```

- `currentExpert`与历史团队工作空间分离；归档团队时可清空当前专家但不得删除 sidecar。
- 成员显示快照不保存 system prompt；名称仍供模型白名单使用，稳定绑定键优先复用 ExpertID。
- 主专家只使用父 Session；只有配置子成员建立绑定。
- 首次成员 task fresh 并写入 ref；后续由宿主自动 continue。
- 专家 wrapper 保留隐藏 `continue_from`的安全边界，并移除 `read_only_task`和 `parallel_tasks`。
- 主专家按依赖关系调度持久成员：独立工作以`independent: true`启动多个`task`后统一wait；有依赖的
  工作先wait前置成员，由主专家检查并整理回执后重新编写下游prompt，再以`depends_on`记录已成功
  `task_run_id`。宿主不自动排队、启动或复制child历史。`parallel_tasks`仍只承担普通/单专家的一次性
  临时fan-out，不形成成员历史。

专家团的通用运行合同由 Root Go 在 Session system prompt 中一次生成，云端目录只提供团队/成员资产与
成员专属提示。该合同保持简洁：主专家负责理解、按需委派、整合；具体工作至少委派一名合适成员；
独立任务需显式并发、依赖任务由主专家交接后串行；最多四个成员 task、同成员最多一个 running turn；用
`wait_task/stop_task`收口，完成前不得 final，并保持现有 `complete_step/verify_artifact`闭环。
Frontend 每回合隐藏块只保留成员白名单、语言等路由元数据，不重复注入整套编排规则。
- 团队归档前停止运行成员；之后绑定只用于历史导航，不再参与调度。

### 4.6 父/child 事件在入口分区

在现有事件合同增加足以确定来源的可选字段，例如：

```text
subagentRef?: string
sourceToolId?: string
```

- 持久 task 的 Text、Reasoning、Message、Tool、final 进入 `childItemsByRef[ref]`；
- 父事件进入 `mainItems`；
- parallel 临时分支事件只进入对应外层卡片的临时 branch state；
- `ParentID`继续表示嵌套关系，不能充当 Session 身份；
- 历史重载使用同样的分类结果，避免实时和重载显示不同。

### 4.7 task 卡与专家历史 UI

- 父 transcript 中 `task`、`wait_task`、`stop_task`按 `subagentRef`或成员绑定合并成一张 lifecycle 卡。
- 普通卡显示描述、当前状态和完成结论，不提供 child 入口。
- 专家卡增加成员身份；点击成员头像在现有父任务壳内装载该 ref 的只读历史。
- `team_active`和`team_archived`的成员条都由 sidecar 恢复，不依赖实时 tool item。
- 子历史页复用现有 SubagentHistory 分页和清洗；移除 composer，并提供返回主会话入口。
- 退出团队后恢复基础 prompt，但保留成员条和只读历史；同会话的召唤入口返回“请新建会话”。

### 4.8 child 工具详情路由

新增或整理 child 专用查询：

```text
ToolResultForSubagentForTab(parentTab, subagentRef, sourceToolId)
```

它复用现有父所有权校验，加载 child Session/JSONL，并按 raw tool ID 查找。归档 item 必须保留 ref
与 raw ID；找不到时返回明确错误，不回退父 `ToolResultForTab`。

### 4.9 fork / rewind

保持父历史能力简单：

- fork 复制父消息、外层工具回执和必要团队显示状态，不复制 child JSONL；
- rewind 只截断父历史，不截断 child；
- 新父分支或 rewind 后的新时间线清空活动成员 ref 绑定；下一次成员任务 fresh；
- 旧 child 仍由原父所有权管理，但不作为新时间线的活动历史；
- 不使用现有 lineage 懒复制去制造“同一成员历史已随父分支复制”的产品错觉。

### 4.10 结构化结果与 evidence

把 `wait_task`终态结果定义为父 Session 的正式完成回执：

```text
subagentRef
task_run_id
memberName?
turnStatus
finalAnswer
artifacts[]
validations[]
```

- 宿主只附加实际观察到的 artifact/read/write/verification；
- 非派发的调度回执不进入父完成证据；
- child ledger 不复制到父 ledger，child 自由文本不被解析为验证事实；
- running/failed/interrupted 回执不能满足完成 readiness；
- running child 同时阻止父 final 和父 `ModelTask`结算；
- 父 UI 可以合并工具卡，但父 JSONL 仍保留真实 task/wait/stop 调用与结果。

## 5. 交付阶段

### 阶段 A：task 生命周期与持久化

- 统一父 turn 内异步 task、wait_task、stop_task；
- 增量保存、状态恢复、failed/interrupted 可续写；
- 回收站移动/恢复/永久删除、进程退出和同 ref 并发边界；
- 保持 read_only Registry 与单层代理限制。

### 阶段 B：实时投影隔离与 parallel 收敛

- `mainItems`、`childItemsByRef`、parallel branch state 分区；
- 普通 task 不串页；
- parallel 只保留一张聚合卡，无持久 child/AgentWorkspace 分支页；
- 实时和重载一致。

### 阶段 C：专家团状态与历史恢复

- `normal / team_active / team_archived`持久化；
- 成员快照、member-to-ref 绑定与串行约束；
- 专家只用 task，多成员通过多个 task 并行；
- 头像只读历史、退出团队保留入口、同会话禁止再召唤；
- fork/rewind 重置活动绑定。

### 阶段 D：详情与证据闭环

- child raw tool 路由与归档详情；
- task lifecycle 卡合并；
- wait_task 结构化完成回执；
- readiness 正反验证和全链路 Desktop 验收。

### 阶段 E：持久 task 显式调度

- 复用现有job ID形成`task_run_id`，同ref多turn仍有独立run身份；
- running期间并发必须显式`independent: true`，四任务容量保持不变；
- `depends_on`只接受同父Session成功run，可跨父turn引用历史交付但不改变计费归属；
- 主Agent负责wait、检查、整理和重新派发，宿主不建立自动队列或child直连；
- 三种未派发状态使用中性结构化回执和紧凑UI，不创建成员状态。

各阶段可以按独立可审查切片实现，但最终启用顺序必须避免以下半成品：持久 child 仍串入父页面、
专家 UI 依赖尚未持久的绑定，或 readiness 接受尚未核验的 child 事实。

## 6. 预计影响范围

最终路径以 Stage 2 源码复核为准，预计涉及：

- Root Agent：`task.go`、`subagent_store.go`、`parallel_tasks.go`、
  `expert_team_tools.go`、jobs 适配、事件 wire、artifact/evidence；
- Control/Desktop Go：expert sidecar 保存加载、fork/rewind、父删除/退出、child 历史和工具查询；
- Frontend：session state、实时/历史 projection、Transcript、ToolCard、AgentWorkspace/成员导航；
- 测试：Root Go、Desktop Go、frontend build 与真实 Desktop 场景。

不修改 Provider线协议字段、顶层 Session 目录、Task Tree 数据模型或专家目录资产格式。OpenAI兼容
客户端增加120秒transport idle、5分钟progress idle、10分钟单请求上限、5秒finish收尾窗口，并只在
完全无有效输出时安全重试一次；这些保护不限制整个child task。

## 7. 验证计划

### 自动化

- Root Go：父 turn 内异步启动/等待/停止；状态机；计费归属；增量保存；失败/中断续写；进程退出/回收站；普通显式
  复用；read_only 一次性边界；parallel 临时化；专家成员绑定；同成员并发拒绝；Registry 单层边界；
  结构化 handoff；显式独立并发、run ID、同Session历史依赖、无效依赖和容量中性回执。
- Desktop Go：chatMode/sidecar 恢复；父所有权；child 历史分页；child 工具详情；fork/rewind 重置绑定；
  team archive；旧数据无绑定降级。
- Frontend：首个 child 事件即隔离；task 卡合并；parallel 聚合卡；三种 chatMode；成员历史只读；
  compact 详情；切换/重载一致。
- 按仓库规则对实际变更包运行比例化 Go 测试，所有 frontend 变更运行 `pnpm build`。

### 真实 Desktop

逐项执行 requirements R-01 至 R-34，重点复现原始两个问题：普通 task 不再出现相似 child 对话，
专家成员切换会话/重启后仍存在；同时验证 parallel 重载只保留聚合结果是预期行为。

## 8. 风险与回滚

- **turn-scoped 上下文**：job 必须同时继承父计费身份和取消生命周期；测试覆盖取消、final/结算门禁及
  后续父 turn 复用同 Session 时切换到新 `ModelTask`。
- **增量写并发**：复用 ref 锁并限制同 Session 单 turn；不得并发写 JSONL。
- **退出/崩溃窗口**：终态以 stale interrupted 收敛，禁止自动重放避免重复副作用。
- **sidecar 扩展**：字段可选、旧数据缺省为 normal；旧缺失绑定不补造。
- **parallel 误用**：专家 Registry 直接移除 parallel，并在 prompt 说明多成员并行方式。
- **错误并行**：并行必须显式声明，但宿主不猜业务语义；自然验收覆盖真正独立、实现后验证和跨领域
  依赖，非派发回执不得成为child失败或完成证据。
- **evidence 放宽**：只接受宿主结构化事实；任何解析 child final 的方案都应被拒绝。

回滚代码不得删除已经写入的 child JSONL；新增 sidecar 字段必须允许旧版本忽略。

## 9. 进入实现前关口

1. 本 Stage 1 文档集通过用户评审且无相互矛盾。
2. 获得单独授权后，从最新 `origin/main-v3`创建 Stage 2 任务拆解。
3. 任务文档逐项映射 R-01 至 R-34、允许范围、依赖和验证。
4. 任务拆解获批并再次获得实现授权后，才修改代码。

当前只定稿文档；不进入实现，不推送，不创建或修改 PR。
