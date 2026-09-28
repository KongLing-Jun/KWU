# Desktop 单人聊天与专家团聊天的子代理需求

## 1. 文档地位

本文是本主题唯一的产品需求权威，冻结用户可见行为、运行边界、持久化语义和验收结果。
`plan.md`、`contracts.md`、`evidence.md`、`progress.md`以及后续任务文档只能解释如何满足本文，
不得另行发明 Agent Runtime、成员模型或历史模型。

本文中的“必须”和“不得”是验收约束。任何产品语义变化必须先修改本文并重新确认。

## 2. 核心产品模型

Desktop 只有两种聊天形态：

1. **单人聊天**：用户只与一个父 Agent 对话。父 Agent 可以是基础身份或一个已加载的单专家身份。
2. **专家团聊天**：仍由一个父 Agent 与用户对话；父 Agent 追加主专家身份，并用现有 `task`
   调度固定子专家。

专家团不是第二套 Agent Runtime。它只在单人聊天已有能力上增加：团队提示词、固定成员与
`subagentRef`绑定、成员状态、头像入口和只读子会话历史。

统一边界如下：

```text
task
  -> 一名可续写子代理
  -> 一份持久 child Session
  -> Session 内可有多个 turn

read_only_task
  -> 一次性只读研究代理
  -> 只返回最终答案
  -> 不持久化、不续写、不提供历史页

parallel_tasks
  -> 一次临时批量 fan-out
  -> 只向父 Agent 返回聚合结果
  -> 不形成可续写、可浏览的 child Session
```

Task Tree 继续只表达任务 DAG、依赖、状态、进度和结果关联；它不是会话树、成员表或历史存储。

## 3. 权威对象与术语

### 3.1 父 Session

用户在 Desktop 侧栏打开的顶层会话。它保存用户消息、父 Agent 的 reasoning 摘要与消息、父工具
调用/结果，以及 `task`、`wait_task`、`stop_task`、`parallel_tasks`的父级回执。

### 3.2 child Session

`task`创建的独立子代理会话。一名子代理对应一份 Session；同一 Session 可以连续执行多个 turn。
child Session 由父 Session 所有，不进入 Desktop 顶层会话列表或标签页。

### 3.3 task turn

child Session 中一次从任务输入到 `completed`、`failed`或`interrupted`的执行。终态描述本次 turn，
不表示 Session 永久关闭。每次成功启动的持久 task turn 都有独立 `task_run_id`；它复用现有持久
job 身份，用于精确记录本次执行及其依赖，不替代标识连续 child Session 的 `subagentRef`。

### 3.4 专家团工作空间

父 Session 上持久化的团队元数据：团队状态、成员显示快照、成员到当前 `subagentRef`的绑定。
它只负责身份、恢复和 UI 导航，不替代 Session、SubagentStore 或 Task Tree。

## 4. 会话与身份状态

父 Session 必须持久化以下聊天状态，不得靠当前输入、旧消息文本或前端缓存推断：

| 状态 | 当前父 Agent 身份 | 团队入口 | 原会话能否召唤专家 |
| --- | --- | --- | --- |
| `normal` | 基础 Agent 或单专家 | 无 | 可以 |
| `team_active` | 团队主专家 | 成员可见，已有历史可打开 | 不可切换团队/单专家 |
| `team_archived` | 基础 Agent | 标识与历史保留、只读 | 不可；必须新建会话 |

- 单专家属于 `normal` 单人聊天，只改变父 Agent 当前身份，不创建 child Session。
- 专家身份和团队主专家身份必须作为既有基础 system prompt 的附加指令，不得替换基础 prompt。
- 清除单专家后恢复基础身份，保留旧回复，并允许原会话再次召唤专家或专家团。
- 退出专家团后进入 `team_archived`：移除主专家附加指令，但保留团队标识、成员及历史入口；
  原会话不能再次启用任何专家或专家团。

## 5. 召唤交互

- 从当前 Composer 召唤单专家或专家团时，原地应用到当前 `normal` 父 Session。
- 从全局专家目录召唤时，进入新的会话草稿，不静默修改当前已有会话。
- `team_active`或`team_archived`会话再次召唤专家时，必须提示新建会话。
- 退出专家团后父 Agent 可以继续普通聊天，新的普通 `task`不得写入旧团队成员 Session。

## 6. task 的统一运行语义

### 6.1 父 turn 内统一异步

所有 `task`都按同一种父 turn 内异步机制运行，不再向模型暴露“前台 task / 跨 turn 后台 task”
两套产品语义：

1. `task`启动一个 child turn，立即返回启动回执。
2. `wait_task`由宿主等待该 turn 进入最终状态并返回结果；模型不选择轮询间隔，也不靠重复模型请求
   查询running状态。
3. `stop_task`中断正在运行的 turn，但保留 child Session。
4. 只有与 running child 完全独立的新任务可以显式声明 `independent: true`并行启动；依赖任务必须先
   `wait_task`收口前置 turn，由父 Agent检查并整理结果后再显式创建。
5. 父 Agent 产生 final 前必须用 `wait_task`或 `stop_task`收口所有必要 child turn。

现有内部 job/cancel 机制可以复用，但 job 必须继承拉起它的父 turn 上下文和计费身份，不能使用可
跨父 turn 存活的 Session 后台上下文。模型仍不得用内部 job 控制接口操作子代理，但 `task`必须把
该次持久 job 的不透明身份作为 `task_run_id`返回，以便声明后续依赖。

### 6.2 控制身份

- 普通聊天的 `task`、`wait_task`、`stop_task`以 `subagentRef`定位子代理。
- 专家团中模型只使用白名单 `member_name`；宿主负责解析该成员的 `subagentRef`和当前父 turn 内 job。
- 专家团 schema 不得向模型公开任意 `continue_from`或内部 job 控制接口；可以与普通 task 一样使用
  不透明 `task_run_id`声明 `depends_on`。
- `stop_task`应复用现有 `KillForSession`等取消能力，只停止当前 turn，不删除历史。
- 现有 `kill_shell`面向通用后台 job，不得作为子代理的产品控制接口。

### 6.3 生命周期

| 状态 | 含义 | 能否继续同一 Session |
| --- | --- | --- |
| `running` | 当前 turn 正在执行 | 否；先等待或停止 |
| `completed` | 当前 turn 正常完成 | 能 |
| `failed` | Provider、工具、超时或步数等失败 | 能 |
| `interrupted` | 用户/父 Agent 停止、父取消或应用退出 | 能 |

- 同一 child Session 同时最多一个 running turn。
- 一个父 Session 同时最多运行四个不同 child Session 的持久 turn；达到上限时返回
  `capacity_wait_required`且不创建第五个 child/job，直到已有 turn 通过 `wait_task`收口或被
  `stop_task`停止。
- 已有持久 task running 时，未显式声明 `independent: true`的新 task返回
  `dispatch_decision_required`；该结果不是 child失败，且不得创建 child Session、job、成员绑定或状态。
- `depends_on`只接受同一父 Session 中已成功完成的 `task_run_id`，可以引用此前父 turn 的历史交付。
  running依赖返回`dispatch_decision_required`；failed、interrupted、缺失、非task或跨父Session依赖返回
  `dependency_not_satisfied`。这些结果均不得创建 child。
- 父 Agent是唯一交接者：它等待并检查上游回执，整理必要信息写入新的下游 prompt，再创建后续task；
  宿主不得自动排队、自动启动、复制child历史或在child之间传话。
- 本轮不增加 running turn 中途注入消息或 inbox。
- `failed`和`interrupted`不得被当成 Session 死亡；下一次任务可继续已有历史。
- 整个child task不设固定总时长；OpenAI兼容模型请求分别受120秒网络空闲、5分钟无有效进展和
  10分钟单请求绝对上限保护。完全无输出的无进展请求最多安全重试一次，已有输出或tool call不得
  自动重放。达到上限只结束当前turn为failed，历史与已完成工具记录必须保留。
- 切换 Desktop 会话不得取消仍由活动父 turn 持有的 child turn。
- 真正退出应用进程时应尽量保存已产生历史并将 running turn 收敛为 `interrupted`；关闭窗口但按
  用户设置转入后台且父 turn 仍活动时不得中断任务。崩溃重启通过
  stale cleanup 收敛，不自动重放模型请求。
- 父 turn 取消必须级联取消 child；父 turn 不得在 child 仍 running 时 final 或完成计费结算。
- child turn 的每次模型调用绑定拉起它的父 `ModelTask`；后续父 turn 复用同一 child Session 时，
  新 child turn 绑定新的父 `ModelTask`。父/child 不跨 Provider。
- 父 Session 移入回收站前必须停止 running child，并把父/child 文件一起移入回收站；恢复父
  Session 时必须一起恢复 child；从回收站永久删除时必须一起删除父/child 数据。

### 6.4 增量持久化

child reasoning 摘要、assistant 消息、工具调用/结果和状态必须随执行增量写入 child JSONL/meta，
不得只在终态保存。切换会话、释放前端缓存或重启应用后，已落盘部分必须可恢复。

## 7. 普通聊天的子代理

- 普通 `task`默认创建新的 child Session。
- 只有用户明确要求继续同一个子代理上下文，或模型持有明确可用的 `subagentRef`时才复用；不得因
  描述相似或名字相同自动复用。
- `read_only_task`保持现有一次性临时语义：使用只读 Registry，只返回最终答案，不生成
  `subagentRef`，不进入 SubagentStore，也不支持 continue、wait、stop 或历史页。
- 普通 child 历史用于模型续写和故障恢复，但用户界面不提供头像或历史入口。
- child 不得获得 `task`、`read_only_task`、`parallel_tasks`等创建代理的工具；最大委派深度为一层。

## 8. 专家团的子专家

- 主专家始终使用父 Session，不为主专家再创建 child Session。
- 专家团成员调度只使用现有 `task`机制；专家团不得向模型提供 `read_only_task`或
  `parallel_tasks`。专家团 `task`不向模型公开逐次 `tools`白名单；宿主省略该参数，使成员从首次
  创建起继承同一套完整的子代理安全工具。只读调查由 prompt约束用途，不缩窄持久 Session的能力。
- 每个配置子专家在当前父分支内自动绑定一份稳定 child Session：首次任务 fresh，后续 turn 自动
  续写同一 `subagentRef`。
- 同一成员任务必须串行；不同成员只有在主专家显式声明 `independent: true`时才能并发。依赖成员必须
  先等待前置回执，由主专家整理后再派发，并以 `depends_on`记录实际消费的成功 `task_run_id`。
- 模型只选择 `member_name`；成员绑定由宿主持久化和维护。
- 普通 `task`继续保留现有逐任务 `tools`能力；专家团只增加 `member_name -> subagentRef`适配，不建立
  第二套模型、推理强度、工具 Registry、调度或持久化机制。
- 不迁移本改动前的成员资料。同版本且工具配置不变时，关闭应用再启动必须可续写；应用版本、MCP、
  插件或工具 schema真实变化时，严格能力校验可以拒绝续写，但旧成员历史必须保持只读可查看。
- 普通会话升级为专家团时，升级前的普通 child 不得被猜测或提升为团队成员。
- 退出团队前先停止仍在运行的成员 turn，再归档团队；归档后成员 Session 只读且不得接收新任务。

## 9. parallel_tasks 的边界

`parallel_tasks`只用于普通/单专家聊天中的临时批量任务：

- 每个分支使用隔离的临时 Session 运行，但不生成可续写 `subagentRef`，不写 child JSONL/meta。
- 父 Session 只持久化外层调用、分支状态/摘要和最终聚合结果。
- UI 只显示一个外层 `parallel_tasks`卡片，卡内显示各分支状态和最终摘要。
- 不提供分支头像、AgentWorkspace 标签、reasoning、内部工具详情或历史页。
- 切换会话或重启后，可由父工具结果恢复外层聚合，不恢复分支内部过程。
- 分支内部过程丢失是明确产品边界，不得伪造 ref 或历史。

## 10. 页面与事件隔离

### 10.1 父页面

父页面只允许显示：用户消息、父 Agent reasoning 摘要/消息、父 Agent 工具，以及 task/parallel 的
外层生命周期卡和返回结果。child Text、Reasoning、Message、Tool、final 不得成为父 transcript
顶层 item，也不得先混入后再靠 Transcript 隐藏。

对于需要调用工具或委派的非简单工作，父 Agent 必须通过既有普通 assistant `content`在有意义的
阶段变化处给出简短、面向用户的工作更新：首次工具/委派前说明目的；证据改变方案时说明调整；
委派或长耗时结果返回后说明结论与下一步；最终交付前说明收口状态。该要求同时适用于基础父 Agent、
单专家父 Agent 和专家团主专家，不适用于 child Agent。

工作更新不得暴露私有思维链、逐个复述工具调用、重复工具输出，或给简单的一步问答增加仪式性文本。
工具或 task 运行期间继续使用已有卡片和状态反馈，不新增定时心跳或合成消息。工作更新作为普通父
assistant 消息实时显示并持久化，切换会话或重启后按既有历史链恢复。

`task`、`wait_task`、`stop_task`虽然分别持久化为父工具调用/结果，UI 必须合并为同一张 task
生命周期卡；不得展示三张互不相关的工具卡。

`dispatch_decision_required`、`dependency_not_satisfied`和`capacity_wait_required`必须显示为紧凑、中性
的“任务未派发”调度结果，不显示原始JSON、红色child失败或成员头像。只有真正创建后失败的child才显示
失败状态。已启动task的详情可以显示`task_run_id`和已接受依赖，但不得新增task tree或等待队列页面。

### 10.2 专家历史页

只有 `team_active`和`team_archived`提供子专家入口。点击头像后，在用户观感上切换到一页新的
只读聊天历史，但仍位于原父任务壳内：

- 侧栏仍选中父会话，不创建顶层会话或标签；
- 主内容区加载该成员唯一 child Session 的连续多 turn 历史；
- 显示经隐私清洗的任务输入、reasoning 摘要、assistant 消息、工具调用和结果；
- 不显示输入框、发送按钮或用户直接回复 child 的能力；
- 提供明确的“返回主会话”；
- `team_archived`下标识与历史仍可查看。

### 10.3 child 工具详情

- child 工具卡展开必须按父 tab + `subagentRef` + child raw tool ID 查询 child Controller/JSONL。
- `compactArchivedToolItems`归档后仍须保留正确路由信息。
- 查询失败必须显示明确错误，不得回退父 `ToolResultForTab`后呈现空 args/output。
- 外层 task prompt 的隐私红线与 child 内部工具详情是两个边界，前者保持，后者必须可用。

## 11. fork 与 rewind

- fork/rewind 只处理父 Session 的历史、检查点和外层 task 回执。
- 不复制、截断、回退或拼接 child transcript。
- fork 后的新父分支、rewind 后继续的新时间线不得复用旧 child 绑定；下一次普通 task 或成员任务
  创建新的 child Session。
- 旧 child 数据按原父所有权保留到父删除，但不进入新时间线的活动成员历史。
- 父 Agent 依靠已保留的外层 task 结果/摘要获得分支上下文；本轮不设计 child 分段复制。

## 12. 结果交付与 evidence

- `wait_task`产生的结构化完成回执，是持久 child 向父 Session 交付结果的正式边界。
- 启动/终态回执至少包含 `subagentRef`、`task_run_id`、可选 `member_name`、turn 状态、final answer、
  artifact列表和必要验证；同一child Session续写产生新的`task_run_id`。
- child 内部工具 receipt 只属于 child ledger，不复制到父 evidence ledger。
- 只有 `completed`且由宿主实际核验的 artifact/验证事实，才能进入父 receipt 并满足 readiness。
- `failed`、`interrupted`或仍 `running`的 task 不得被父 final readiness 当成已完成。
- `parallel_tasks`只以外层聚合 receipt 交付；同样不得把临时分支内部 ledger 复制给父。

## 13. 所有权、安全与隐私

- child 历史/工具详情必须验证 ref 属于当前父 Session；不得跨无关父会话读取。
- `continue`继续执行现有 workspace、actor、profile、model、tool schema 等兼容校验。
- 系统 prompt、A2A 控制信封、除公开不透明`task_run_id`外的内部 job 控制字段不得暴露在用户历史页。
- 成员快照只保存恢复 UI 所需的非敏感身份字段，不保存私有 prompt。

## 14. 兼容与迁移

- 只保证新版本产生的数据满足本文，不迁移、不推断、不补造旧 parallel 历史或旧成员绑定。
- 不重写旧 Session 已持久化的 system prompt；可见工作更新策略只随新建 Desktop 场景会话生效。
- 旧父 Session 的外层工具结果继续可读；没有 child ref 时不得显示伪造历史入口。
- 优先扩展现有父 sidecar、SubagentStore 和 JSONL；不得另建 Agent/Invocation/TaskRun 存储体系。
- 不要求兼容未发布或未采用的旧方案文档与实验数据。

## 15. 非目标

- 用户与 child 直接聊天；
- 多级子代理或子专家再拉起代理；
- 每次 task run 的独立页面或运行选择器；
- 把 child Session 放入 Desktop 顶层会话目录；
- 把 Task Tree 改造成 Session/成员所有权模型；
- 专家团专用模型协议、Provider 协议或另一套 Runtime；
- 暴露模型私有思维链、为工作更新新增消息协议，或在运行期间生成定时心跳；
- 旧数据迁移、文本推断绑定、自动恢复崩溃前模型请求。

## 16. 验收场景

| ID | 场景 | 必须结果 |
| --- | --- | --- |
| R-01 | 普通聊天调用 task | 主页面只有一张 task 卡、父结果和父回复，无 child 串页 |
| R-02 | task 启动后立即 wait | child 在父 turn 内异步运行；wait 收口后父 Agent 才能继续 |
| R-03 | running task 被 stop | 当前 turn 为 interrupted，历史保留且可继续新 turn |
| R-04 | task completed/failed 后继续 | 同一明确 ref 可开始下一 turn；failed 不杀死 Session |
| R-05 | 活动父 turn 中切换会话再返回 | child 仍受原父 turn 持有并继续，父页面不出现 child item |
| R-06 | running 时关闭并重启 | 已有历史恢复，turn 为 interrupted，不自动重放 |
| R-07 | 普通聊天明确要求复用 | 续写指定 child Session；默认新任务仍 fresh |
| R-08 | 普通 parallel_tasks | 一张聚合卡显示分支状态/摘要，无 ref、历史或子工具页 |
| R-09 | parallel 重载 | 只恢复父聚合结果，不伪造分支历史 |
| R-10 | 单专家清除 | 历史保留，下一轮恢复基础身份，仍可再次召唤 |
| R-11 | 普通会话启用专家团 | 父历史保留，旧普通 child 不进入成员绑定 |
| R-12 | 专家成员首次 task | 建立该成员唯一 child Session 和历史入口 |
| R-13 | 同成员再次 task | 自动续写同一 Session 的下一 turn，不要求模型传 ref |
| R-14 | 多成员持久任务 | 独立成员显式声明`independent: true`后可分别启动task并统一wait；有依赖的成员必须等待前置结果再启动 |
| R-15 | 父 turn 内再次派发持久 task | 不同child最多四个并发；同一ref的第二个turn保持拒绝，第五个返回中性容量等待结果 |
| R-16 | 专家团切换会话/重启 | 团队、成员绑定、状态和已落盘历史恢复 |
| R-17 | 点击成员头像 | 原父任务壳显示连续只读 child 历史，并可返回主页面 |
| R-18 | child 归档工具展开 | 从正确 child JSONL 恢复完整 args/output/error |
| R-19 | 退出专家团 | 父身份恢复普通；团队标识和历史保留只读，原会话不可再召唤 |
| R-20 | 归档团队后普通 task | 创建普通隐藏 child，不续写旧成员 Session |
| R-21 | fork/rewind 后派发 | 父历史按现有规则变化；新时间线创建新 child，不复制旧 transcript |
| R-22 | child 尝试创建孙代理 | Registry 无代理创建工具，调用不可发生 |
| R-23 | readiness/计费收口 | running child 阻止父 final/结算；只承认 completed 宿主回执，不承认 child ledger |
| R-24 | 普通聊天调用 read_only_task | 只返回一次性只读研究结论，无 ref、续写或历史入口 |
| R-25 | 父会话移入回收站后恢复 | running child 先 interrupted；父/child 文件一起移动并一起恢复 |
| R-26 | 从回收站永久删除父会话 | 父 Session、全部 child Session 与关联元数据一起删除 |
| R-27 | 普通复杂任务调用工具或 task | 父 Agent 在关键阶段用简短自然语言说明目的、重要结果和下一步；child 内容仍不串页 |
| R-28 | 专家团主专家委派成员 | 主专家说明委派目的并在结果返回后整合结论；子专家过程只在其只读历史页显示 |
| R-29 | 简单问答、连续工具运行及重载 | 简单问答不强加过程文本；运行期间由现有卡片反馈；已有父工作更新在切换/重启后恢复 |
| R-30 | running时再次调用task且未声明独立 | 返回中性`dispatch_decision_required`；不创建child/job/成员状态，父Agent自行wait或显式重试并行 |
| R-31 | 显式独立的多task | `independent: true`允许最多四个不同child并发；达到上限返回`capacity_wait_required`且无红色失败 |
| R-32 | 主专家执行依赖任务 | 先wait并检查上游，由主专家整理新prompt后再派发；下游`depends_on`引用成功`task_run_id`，不自动传child历史 |
| R-33 | 同一child多turn与历史依赖 | `subagentRef`保持不变、每次turn的`task_run_id`不同；同父Session跨turn完成依赖在切换/重启后仍可验证 |
| R-34 | 无效依赖 | running依赖要求调度决策；失败/中断/缺失/跨父Session依赖返回`dependency_not_satisfied`且不泄露、不创建child |

## 17. 完成定义

只有以下条件全部成立，才能认为本需求完成：

1. R-01 至 R-34 的适用场景通过自动化或真实 Desktop 验收。
2. 单人、`team_active`、`team_archived`的实时与重载行为一致。
3. 父/child transcript 与 evidence 在数据路由层隔离，不依赖视觉隐藏或文本推断。
4. 实现只复用和扩展现有机制，没有引入第 15 节非目标。
