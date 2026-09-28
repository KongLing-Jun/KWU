# Desktop 子代理调度与专家团最小增量合同

## 1. 合同地位

本文把 [requirements.md](requirements.md) 转换为实现必须保持的数据和接口边界，不独立定义产品。
如果现有源码无法满足，应暂停并回到需求评审，不得用第二套 Runtime 绕开。

## 2. 对象合同

### 2.1 父 Session

- 唯一顶层聊天对象，继续承担侧栏、标签、fork 和 rewind。
- 保存父消息/reasoning、父工具调用/结果、task 生命周期回执和 parallel 聚合结果。
- 不保存 child 内部消息、reasoning 或工具事件。

### 2.2 child Session

- 由现有 `SubagentStore`创建、加载、保存和删除。
- 一个 `subagentRef`唯一标识一名 child 及其连续多 turn 历史。
- `ParentSession`继续表示所有权；移入回收站时父/child 文件一起移动，恢复时一起恢复，永久删除时
  级联清理。
- 不进入顶层 Session catalog。

### 2.3 task turn

最小逻辑状态：

```text
subagentRef
task_run_id          // 复用该次持久job ID；模型可见但只用于依赖
status: running | completed | failed | interrupted
parentToolCallId
depends_on[]?        // 同父Session的成功task_run_id
startedAt / endedAt?
lastError?
```

- 同一 ref 同时最多一个 running turn。
- terminal 状态不关闭 Session。
- `task_run_id`直接复用现有持久job ID，不新增 TaskRun Store或第三套身份；同ref续写产生新run ID。
- `subagentRef`仍是wait/stop/continue控制身份；`task_run_id`只用于启动/终态回执、依赖与详情审计。

### 2.4 专家团工作空间

最小逻辑结构：

```text
SessionExpertMeta
  chatMode: normal | team_active | team_archived
  currentExpert?                 // 基础身份为空；单专家或团队主专家为现有选择
  teamWorkspace?
    teamId / teamName
    state: active | archived
    members[]
      memberKey
      expertId?
      name / role / avatarKey / order
      subagentRef?
```

- `currentExpert`和 `teamWorkspace`独立；团队归档可清空当前专家但必须保留 workspace。
- snapshot 不得包含 system prompt、A2A 信封或其他私有配置。
- 旧 sidecar 没有 `chatMode/teamWorkspace`时按 `normal`读取。
- `memberKey`是父 Session 内稳定键，优先由现有 ExpertID 派生；`member_name`只用于显示和白名单。
- sub 成员规范化名称必须唯一，否则模型无法无歧义选择，应在激活前拒绝。

### 2.5 parallel invocation

- 只存在于一次 `parallel_tasks`调用与其父工具结果中。
- 分支可以有内部临时 Session/branch ID，但不得获得 `subagentRef`或写 SubagentStore。
- 重载来源只有父 Session 的外层聚合结果。

### 2.6 Task Tree

继续只表达 DAG、depends_on、节点状态、进度和结果关联。不得保存 child transcript、成员绑定、
subagentRef 或 tool detail 所有权。持久`task`新增的`depends_on=task_run_id[]`只是run级血缘与准入校验，
不创建或读取Task Tree节点，也不触发Task Tree调度。

## 3. 工具合同

### 3.1 task

公开语义：

```text
input:
  prompt / description / model / effort / max_steps ...
  continue_from?     // 只在普通聊天公开，且必须是明确复用
  independent?       // 已有running task时，显式确认新任务完全独立
  depends_on[]?      // 同父Session已成功完成的task_run_id

output:
  subagentRef
  task_run_id
  turnStatus=running
```

- 不公开 `run_in_background`；所有调用立即返回，但内部 job 只能活在当前父 turn 内。
- fresh 创建 child Session，continue 续写指定 Session。
- expert wrapper 不公开 `continue_from`，由 `member_name`绑定决定 fresh/continue。
- 启动失败不得返回 running；未派发结果不得创建child ref/job/成员状态。
- 已有running task且未声明`independent: true`时返回nil-error结构化
  `dispatch_decision_required`，提示父Agent wait或显式重试并行。
- `depends_on`在创建child前按当前父Session、kind=task和终态校验：running返回
  `dispatch_decision_required`；failed/interrupted/缺失/跨父Session返回`dependency_not_satisfied`；只有
  successful completed可通过。同父Session此前父turn的完成run允许引用。
- 同一父Session允许最多四个不同 ref 的 running 持久 child；第五个返回结构化
  `capacity_wait_required`，必须先 wait 或 stop。
- 同一 ref 仍只允许一个 running turn，由既有 SubagentStore 进程内锁拒绝并发续写。
- child job 继承父 `TaskContext`和生命周期占用，父取消时级联取消；父任务结算必须等待其终态。

### 3.2 read_only_task

- 保持现有一次性、临时、阻塞到结论返回的语义。
- 使用现有只读 Registry、prompt 和预算，不生成 `subagentRef`，不写 SubagentStore。
- 不支持 continue、wait_task、stop_task 或用户历史页。
- 只在普通/单专家 Registry 可见；专家团队 Registry 不注册。

### 3.3 wait_task

普通输入：一个或多个 `subagentRef`；专家输入：一个或多个 `member_name`。宿主解析到当前 turn。

- 模型schema不暴露`timeout_seconds`。调用后由jobs manager等待所有目标进入completed、failed或
  interrupted，或由父context取消；等待期间不得发起新的父模型请求。
- failed/interrupted必须返回结构化终态回执；不能把running状态伪装成完成，也不能删除child历史。
- 内部诊断可以读取running快照，但它不是模型轮询接口。

### 3.4 OpenAI兼容流终止

- 任意网络数据只刷新120秒transport idle；只有reasoning、content、tool、usage或明确终态刷新
  5分钟progress idle。
- 单次请求墙钟上限10分钟；`finish_reason`出现后给`[DONE]`和usage最多5秒收尾窗口。
- 无有效输出的progress timeout最多重试一次；已有有效输出时转成stream interrupted并停止自动重放。
- 这些上限只约束单次Provider请求，不限制child task总时长或工具执行时长。

终态输出至少包含：

```text
subagentRef
task_run_id
memberName?
turnStatus
finalAnswer?
artifacts[]
validations[]
error?
```

- 可等待 running turn，也可读取已终止 turn 的最后回执。
- 完成回执写入父 Session/evidence；内部 child receipt 不复制。
- 多 ref 等待继续以ref/member作为控制输入；结果同时返回对应不透明`task_run_id`，不开放任意job控制。

### 3.4 stop_task

- 普通输入 `subagentRef`；专家输入 `member_name`。
- 宿主定位当前内部 job 并调用现有取消能力。
- 已终止时为幂等状态返回；找不到/所有权不符时明确报错。
- 成功停止后 turn 为 interrupted，Session 和已落盘历史保留。

### 3.5 parallel_tasks

- 只在普通/单专家 Registry 可见，专家团队 Registry 不注册。
- 外层仍是一条父工具调用，按现有规则并发运行临时分支并返回输入顺序聚合。
- 分支不调用 `PrepareFresh/PrepareContinue`，不生成 ref，不支持 wait_task/stop_task 单支控制或续写。
- 未启动的 skipped 分支只写外层状态，不创建伪造历史。
- 没有 event sink 时也不得 fallback 到持久 background task；应使用同一临时执行器或明确失败。

### 3.6 专家 wrapper

- 模型只看白名单 `member_name`，不看任意 ref或`continue_from`；与普通task共用不透明
  `task_run_id/depends_on`调度合同，但不能用它代替member控制成员。
- `member_name -> memberKey -> subagentRef/current turn`由宿主解析。
- 首次调用 fresh 并原子写绑定；后续调用 continue。
- 专家 wrapper不公开也不接受逐次 `tools`白名单；转入同一个 `TaskTool`前省略该字段，使 fresh 和
  continue都使用现有“继承全部可用子代理安全工具”的默认路径。
- `TaskTool`、SubagentStore、模型/推理强度设置、工具 Registry与严格 identity校验均与普通 task共用；
  不为子专家复制第二套运行时。普通 task的 `tools`输入保持不变。
- 同版本且工具配置不变时必须支持重启续写；真实工具/schema变化仍由 `SubagentStore`拒绝，不扩张
  能力、不迁移旧 ref，历史保持只读。
- 不同成员最多四个 task 并发；同一成员仍最多一个 running turn。已有running成员时，模型必须以
  `independent: true`显式确认另一成员独立；依赖任务先wait并由父Agent整理后再创建，以`depends_on`
  记录成功run。宿主执行意图门禁、依赖校验、上限、同ref锁、父生命周期、取消、计费与final边界，
  但不推断业务关系、不排队或自动交接。
- `team_archived`或无 active workspace 时不得进入成员调度路径。

### 3.7 专家团提示词所有权

- 云端专家目录只提供团队/成员名称、说明和成员专属 prompt，不承载通用运行时调度合同。
- Root Go 生成一次简洁、缓存稳定的专家团 system prompt，作为调度合同唯一权威来源。
- Frontend 每回合隐藏块只携带 `member-names`和语言等路由元数据，不重复 task/wait/stop 编排规则。
- 通用合同只描述产品能力与限制；仅暴露任务所需的`independent`、不透明`task_run_id/depends_on`，不暴露
  任意ref、job控制接口、A2A信封或内部存储/计费实现。

## 4. 状态与恢复合同

| 事件 | task turn | child Session | team binding |
| --- | --- | --- | --- |
| task 启动 | running | 立即保存初始状态 | 首次成员写 ref |
| 增量事件 | running | 增量保存 | 不变 |
| 正常完成 | completed | 保存 final | 保持 |
| 执行错误 | failed | 保存错误前历史 | 保持，可续写 |
| stop/父取消/真正退出进程 | interrupted | 保存已有历史 | 保持，可续写 |
| 切换 tab | 不变 | 在活动父 turn 内继续 | 不变 |
| 团队归档 | running 先 stop | 历史保留只读 | 保留但停用 |
| 移入回收站 | running 先 stop 并标 interrupted | 父/child 文件一起移入 | 跟随父 sidecar 移入 |
| 从回收站恢复 | 不自动运行 | 父/child 文件一起恢复 | 跟随父 sidecar 恢复 |
| 从回收站永久删除 | 无 | 父/child 数据一起删除 | 删除 |

`CleanupStaleRunning`只把未正常收口的 running turn 标 interrupted，不自动重放。

## 5. fork / rewind 合同

- fork 复制父消息、父工具回执和必要团队显示元数据，不复制 child JSONL。
- rewind 只改变父消息历史和检查点，不修改 child JSONL。
- 新 fork 或 rewind 后继续的新时间线清空 active `subagentRef`绑定。
- 下一次成员 task 创建新 child；旧 child 不被懒复制为新时间线成员。
- 无论旧 child 是否仍在磁盘，父 Agent 只依赖当前父历史中保留的 task 回执。

## 6. 事件投影合同

持久 child 事件必须携带足以路由的来源，例如：

```text
subagentRef?: string
sourceToolId?: string
parentToolCallId?: string
```

投影顺序：

```text
父事件              -> mainItems
task child 事件      -> childItemsByRef[ref]
parallel branch 事件 -> parallelStateByCall[parentToolCallId]
```

- child 事件不得短暂进入 mainItems。
- `ParentID`只表示嵌套关系，不能替代 `subagentRef`。
- 历史 projection 必须产生与实时 projection 相同的分区。
- `turn_started`空 assistant 占位不得形成用户可见的重复 assistant 消息。

## 7. UI 合同

### 7.1 父 transcript

- 只渲染 mainItems。
- `task/wait_task/stop_task`按 child/member 合并成一张 task 卡。
- `dispatch_decision_required`、`dependency_not_satisfied`和`capacity_wait_required`默认折叠为中性
  “任务未派发”摘要，不创建成员入口、不显示红色child失败；详情可显示run/dependency引用。
- 普通 task 卡无 child 历史入口；专家 task 卡标识成员并可导航。
- parallel 只显示一张外层卡，卡内是分支状态和最终摘要，无 synthetic task tabs。

### 7.2 专家历史

- 只有 active/archived 团队显示成员条。
- 未建立 ref 的成员可显示未执行状态，但不可打开空历史。
- 点击已绑定成员在父任务壳内打开连续只读 Session；侧栏仍选中父会话。
- 页面显示清洗后的多 turn reasoning/messages/tools/results，无 composer，提供返回入口。
- 团队归档后仍可查看；任何新普通 child 不得进入旧成员历史。

## 8. child 工具详情合同

- 父工具详情继续通过 `ToolResultForTab(tabId, toolId)`。
- child 工具详情必须通过 parent tab + ref + raw tool ID 的专用查询。
- 查询先验证 ref 归属父 Session，再读 child Controller/JSONL。
- compact 归档保留 `subagentRef/sourceToolId`；child not-found 不得回退父查询。
- 外层 task prompt 隐私隐藏不影响 child 内部工具 args/output 的授权读取。

## 9. Evidence 合同

- 父、child ledger 保持隔离。
- `task`启动回执只证明已运行，不证明完成。
- 非派发调度回执不证明启动或完成，不得晋升父evidence。
- `wait_task`的 completed 回执是正式交付；宿主核验字段可进入父 ledger。
- failed/interrupted/running 不满足完成 readiness。
- running child 持有父任务生命周期；不得先完成父 final 或计费结算。
- child turn 沿用父 Provider 并归属当前父 `ModelTask`；同一 Session 的下一 child turn 归属下一父任务。
- 不复制 child tool receipt，不解析 child final 自由文本为验证事实。
- parallel 只产生一份父聚合 receipt，临时分支 ledger 不持久化到父。

## 10. 权限与隐私合同

- child history/tool detail 必须验证父所有权；跨父读取明确拒绝。
- continue 保持 workspace、actor、profile、model、tool schema 等现有校验。
- child Registry 不包含任何 Agent 创建工具，也不暴露 background shell。
- 用户历史页不显示system prompt、A2A信封或内部job控制字段；task详情可显示公开的不透明
  `task_run_id`与依赖引用。

## 11. 兼容合同

- 新增 meta/sidecar 字段必须可选；旧数据缺省为 normal。
- 不迁移旧 parallel 内部过程，不从名字、文本、Task Tree 推断 ref。
- 不改变 `read_only_task`的一次性临时存储边界。
- 不增加新的 Session ID、Invocation Store、TaskRun Store 或顶层 Session 类型。
- 不修改专家目录资产结构、Provider线协议字段或 Task Tree 模型；只补齐现有OpenAI兼容客户端的
  终止、超时和安全重放语义。

## 12. 变更停止条件

出现以下任一情况必须停止并重新确认 requirements：

- 需要第二套 Agent Runtime、Session 格式或运行记录页面；
- 需要用户直接与 child 聊天或支持多级子代理；
- 需要让专家重新使用 parallel_tasks 才能并行；
- 需要让专家使用 read_only_task 并因此产生第二类成员历史；
- 需要复制/截断 child transcript 才能实现 fork/rewind；
- 需要迁移或猜测旧成员历史；
- 需要把 child 自由文本或内部 ledger 直接当成父证据；
- 需要暴露任意 ref、可执行job控制接口、私有 prompt 或放宽父所有权校验。
