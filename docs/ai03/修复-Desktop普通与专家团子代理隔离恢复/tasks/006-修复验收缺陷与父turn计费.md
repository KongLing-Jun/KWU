# 任务 006：修复验收缺陷与父 turn 计费

## 状态与依赖

- 状态：父 turn 计费与定向链路已完成；父 Agent 可见工作更新已实现并通过自动化/构建验证，等待按本文自然场景验收；其余调度、续用和成员栏问题保持待确认。
- 执行组：Serial-06；由当前主 Agent 执行，不委派子代理。
- 依赖：任务001至005、三份验收问题记录，以及用户确认的父 turn 计费边界。
- 未授权 push、PR 或合并；按独立修复意图小步提交。

## 目标

1. 保留 `task / wait_task / stop_task`异步合同，但 child job 只能活在拉起它的父 turn 内。
2. child ModelCall 归属父 `ModelTask`且沿用父 Provider；running child 阻止父 final/结算。
3. 专家团主 tab 使用配置的 lead 名字和头像快照，只显示已建立可信 ref 的子专家。
4. 重新验证普通聊天、专家团、严格订阅计费与 BYOK 路径没有回归。

## 允许范围

- Root task/jobs/modelbilling/control 中父 turn 上下文、生命周期占用、取消和并发门禁的最小改动。
- Desktop 专家 sidecar/view 及 `ExpertTeamSession`展示修正。
- 对应 Root、Desktop、Frontend 聚焦测试和本任务回执。
- 验收问题文档与 `progress.md`状态修正。
- Desktop 顶层场景 prompt 的可见工作更新策略及其聚焦测试。

## 禁止范围

- 新建第二套子代理调度、独立 child 计费任务或跨 Provider fallback。
- 允许 running child 跨已结束父 turn。
- 把 `parallel_tasks`分支持久化或作为专家成员历史。
- 用户直接与 child 对话、孙代理、旧数据迁移或新顶层 Session 类型。
- 无关 loop guard、Provider 协议和专家目录格式重构。
- 云端专家资源、child system prompt、新消息协议、合成心跳或运行时强制文本重试。

## 实现合同

- `task`立即返回 running/ref；本任务最初加入的父级单 task 门禁已被自然验收证明过度保守，后续由
  任务007替换为不同 ref 最多四个并发、同一 ref 仍串行。
- job 从当前父 turn 派生，继承 `TaskContext`和计费生命周期占用；父取消级联中断。
- 父任务生命周期等待 child turn 终态，终态后释放占用；后续父 turn 复用 Session 时使用新的父任务。
- strict background gate 只拒绝真正跨 turn background，不得拒绝上述 turn-scoped task。
- lead 身份作为非敏感快照随团队 sidecar 保存并恢复；`team.name`继续表示团队名，不冒充 lead。
- 子专家 tab 只渲染拥有当前父所有权可信 `subagentRef`的成员；active/archived 都保留已绑定历史。
- 非简单 Desktop 工作中，父 Agent 在关键阶段使用既有普通 assistant `content`说明目的、结果与下一步；
  简单任务、child Agent 和连续工具运行期间不被强制增加过程文本。

## 验收标准

- subscription pool 与 BYOK 下，普通/专家 `task`不再返回 background disabled，child call 与父 call
  写入同一 `ModelTask`，且 Provider 不跨越。
- running child 时父 final/结算仍被阻止；多 task 并发边界由任务007验收。
- 父取消使 child 为 interrupted；切换 tab 不取消仍活动的父 turn。
- 同一 child Session 后续 turn 绑定新的父 `ModelTask`。
- 专家团首 tab 显示配置 lead 的名字和头像；未派发成员不显示，派发后及团队归档后仍可查看历史。
- 原普通 child 事件隔离、parallel 聚合、child 工具详情和回收站行为不回归。
- 普通复杂任务和专家团委派都出现简短父工作更新；更新可随父 Session 重载，且 child 历史仍隔离。

## 聚焦验证

按实际改包运行比例化测试，至少覆盖：

```powershell
go test ./internal/agent/... ./internal/jobs/... ./internal/modelbilling/... ./internal/control/...
go test ./desktop/... # 从 desktop module 按实际包范围执行
pnpm build            # 从 desktop/frontend 执行
go run ./scripts/code_guard
git diff --check
```

真实 Desktop 使用开发环境同 Provider 的主/子模型，分别验收普通 task、专家成员 task、停止、续写、
切换/重启、主专家身份、成员按需出现及严格计费记录。

## 父 Agent 可见工作更新验收方案

### 环境与证据

- 使用包含本修复的新 Desktop 构建、开发环境既有自定义 Provider 和 `gpt/gpt-5.6-luna`；主/child
  保持同 Provider，不为通过验收临时改 system prompt 或模型参数。
- 全部使用新建的 Office/Coding/Design 场景 Session；旧 Session 不纳入策略生效判断。
- 复杂任务使用一次性小型 fixture workspace，预置可发现、可修复且可验证的问题。用户 prompt 不写
  `task`、工具名、成员名、步骤、固定回执或“先解释再操作”等路径限定。
- 每项保存界面截图/导出、父 Session JSONL；涉及委派时同时记录 child ref 和 child JSONL。除视觉
  判断外，还要核对非空父 assistant `content`与工具调用在持久历史中的真实顺序。

### 自然场景

| ID | 用户输入与模式 | 观察点 | 通过标准 |
| --- | --- | --- | --- |
| VU-01 | 普通聊天：“用一句话解释什么是单元测试。” | 简单一步问答 | 直接回答即可，不强加阶段播报或工具调用 |
| VU-02 | 普通聊天：“帮我检查这个小项目有没有明显问题，能修的就修一下。” | 自主调查、修改、验证 | 首次工具前有简短目的说明；关键证据返回后说明结论/下一步；最终交付前完成收口，不逐工具复述 |
| VU-03 | 普通聊天：“这个小项目准备交付了，帮我从代码质量和测试两个方面把关，有问题就处理。” | 模型是否自然委派及如何整合 | 若自主使用 `task`，父 Agent 在委派前说明目的、结果返回后整合结论；若未委派，不把它伪判为 task 失败，但仍满足普通复杂任务更新合同 |
| VU-04 | 专家团聊天：“这个小项目准备交付了，帮我把关并处理必要问题。” | 主专家选择成员、等待和整合 | 主专家在首次委派前说明安排，在成员结果返回后说明结论与下一步；父页没有 child 消息串入，头像历史仍只读 |
| VU-05 | VU-02/VU-04 中出现连续工具或长时间 task | 运行中反馈 | 运行期间由现有工具/task 卡反馈即可；不要求定时文本，也不应在每个工具之间生成同义套话 |
| VU-06 | 完成 VU-02/VU-04 后切换会话并完整重启 | 历史恢复 | 父工作更新按原顺序恢复；专家成员和 child 历史仍隔离，更新不会重复或消失 |

### 判定规则

- “简短且清楚”按用户能否理解正在做什么、重要结果和下一步判断，不要求固定中文措辞或泄露
  reasoning；工具卡内容本身不替代父 Agent 的关键阶段说明。
- 复杂任务若完全没有首次工具/委派前的父文本，或从用户输入到 final 仍只有工具卡，直接判失败。
- 证据确实没有改变方案时，不强求“方案改变”消息；没有发生委派时，不强求委派消息。
- 若单次模型输出偏离合同，在新的同模式 Session 用同一自然 prompt 复测一次；两次结果都如实保留，
  不用更强限定 prompt 覆盖失败，也不把定向链路冒烟当作自然验收通过。
- 自动化负责证明策略注入顺序、唯一性、旧无场景入口不变和 child prompt 隔离；真实 Desktop 负责
  判定模型行为、页面顺序、持久化与用户体感。

## 完成回执

- 状态：运行时和专家 UI 修复已完成并按意图分为两个本地提交；未 push、未创建 PR。
- 可见工作更新：新增缓存稳定的 Desktop 父提示策略，只在可选择场景中位于专家/场景片段之后注入；
  基础父 Agent、单专家和专家团主专家共用既有 assistant `content`链，旧无场景入口和
  `DefaultTaskSystemPrompt`保持不变；未修改云端资源、Provider、事件或前端协议。
- 本项已通过：`go test ./internal/config -count=1`、`go test ./internal/boot -count=1`、
  `go test ./internal/agent -count=1`、`wails build -m -nosyncgomod -nopackage`；真实自然任务验收按
  VU-01 至 VU-06 留待后续统一执行。
- 运行时：`jobs.Manager`新增父 turn 派生启动路径；`task`继承工具超时包装器保存的父上下文，
  获取父计费生命周期租约；父 final 继续被 running child 阻止，父级单 task 门禁由任务007移除。
  真正的 legacy
  `run_in_background`仍由严格后台门禁拒绝。
- 计费：`TaskLifecycleRunner`把派生工作纳入既有 foreground barrier；child 普通失败仍是父可处理的
  工具结果，child ModelCall 的持久化错误继续由各 call registration 交付。
- 专家 UI：sidecar 单独保存 `kind=lead`快照；Desktop view 只返回当前父拥有的已绑定成员；首 tab
  使用 lead 名字/头像，未派发成员不渲染，归档团队继续保留已绑定历史。
- 直接变更覆盖 `internal/agent`、`internal/jobs`、`internal/modelbilling`、`internal/boot`、
  `internal/control`、Desktop history bridge 和 `ExpertTeamSession`前端。
- 已通过：
  - `go test ./internal/jobs ./internal/agent ./internal/modelbilling ./internal/control`
  - `go test ./internal/boot -run "TestBuildTaskSubagentMetadataInheritsActorContext|TestBuildHeadlessRunRunsTaskSubagentWithoutSessionPath|TestDesktopStrictDerivedEntryCompositionKeepsEveryRoleOnBillingSeams" -count=1`
  - Desktop `TestExpertTeamForTabRestoresOwnedMemberBindings`
  - `pnpm exec tsx src/__tests__/expert-team-session.test.tsx`（11/11）
  - `pnpm build`
- 真实 Desktop：先确认旧进程 PID 62436/exe 均早于修复提交并作废该轮结果；随后三次完整构建、
  重启和核对 exe 时间戳。最终 UI 构建为 `2c81a7db`，exe 时间 2026-08-07 16:22:49，PID 60540。
- `gpt/gpt-5.6-luna`自定义 Provider 已通过：
  - 普通会话 `task → wait_task`返回 `CHILD-NEW-1549`，父页面仅一条
    `PARENT-NEW-1549 CHILD-NEW-1549`，无 child 消息串页或重复回复；
  - 专家团未派发时仅显示 lead“架构顾问”；绑定后才显示“代码审查专家”；最终返回
    `LEAD-FIX-1558 MEMBER-FIX-1558`；
  - 点击成员加载独立只读历史；切换父会话、完整关闭并重启应用后，成员、状态和历史仍恢复；
  - 子历史保留与主会话相同尺寸的 composer 外壳，原内容禁用并弱化，中央覆盖“返回主会话继续
    对话”；点击后主 transcript 和可用 composer 原位恢复，无大面积形变或跳动。
- 真实复验过程中发现并修复两项回归：空成员切片序列化为 `null`触发 `.filter()`崩溃；子历史聚焦
  未通知父壳，导致父 composer 仍可交互。两项均有聚焦测试并在新构建中复验通过；按用户确认，
  子历史不是移除 composer，而是保留稳定外壳并提供唯一返回操作。
- Desktop 两个既有严格计费集成测试仍在 `boot.Build`前因 `fixture/model_fixture_01`只解析到默认
  `kwu` Provider 而失败；相同失败已在未含本分支改动的 `D:\Reasonix`主工作区独立复现，不归因于
  本任务。真实严格计费会话仍是下一关口。
- 偏差：没有引入第二计费任务、跨 Provider fallback、parallel 持久化或跨父 turn child；符合合同。
- 剩余风险：自定义 Provider 主路径已经关闭；仍需真实 Desktop 补齐 `stop_task`和 subscription
  Provider 路径。此前高度限定 prompt 的结果只算定向冒烟；自然 prompt“给我做一个贪吃蛇”发现的
  四组问题记录在[自然任务自主调度与专家成员栏](../验收问题-自然任务自主调度与专家成员栏.md)。
  在产品边界确认和自然任务复验通过前，任务006不标为全部完成。
