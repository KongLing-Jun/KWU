# 专家团结构化产物交付修复计划

## 问题与证据

用户通常停留在主专家界面，并以主专家最终回复判断整个专家团任务是否结束。子专家本轮实际生成且经宿主
验证的最终文件，应随主专家最终回复显示文件卡片；子专家完成、工具回执或主专家进度消息都不等于最终
交付。

原始缺卡不是子专家没有写文件，也不是缺少“做”“写”“贪吃蛇”等意图词：

- 普通提示词“帮我写个贪吃蛇”的 completed 子任务回执含本轮 `snake.html`，并带同路径成功的
  `artifact_observed` validation。
- 主会话已持久化 assistant；但前端只从主专家自然语言正文提取路径。正文不重复可解析路径时，结构化
  回执无法生成卡片。
- 首轮用自然语言正则收紧子任务完成门禁后，既出现“一个 HTML 被解析成两个目标”的误杀，也漏掉“实现
  单文件贪吃蛇浏览器游戏”，证明意图正则不是交付链路的可靠依据；该门禁已撤销。
- 后续复核又确认一个独立上游漏口：子专家允许使用 `bash` 或不在参数中返回输出路径的生成器，但工作区
  增量当前只在子任务 prompt 命中产物正则时启用。于是“未命中正则 + 无路径写工具”可以实际写出文件却
  不产生 `TurnArtifacts`。该路径在源码和确定性 opaque writer 中可达，但既有应用验收的两个成功样本均
  使用带路径的 `writer`，尚未覆盖该组合。

随后实现把宿主验证路径投影到同一用户轮次中回执后的最后一个主 assistant。实际 A/B 验收发现该实现也
不能交付：未修复版因为主模型恰好在终稿中写出路径，原缺卡没有稳定复现；修复版则在根任务尚未结束的
进度消息上稳定提前显示卡片。

源码证据解释了这一现象：

- `Agent.Run` 的每次模型流结束都会发根级 `message`；带工具的中间轮、readiness 尚未接受的候选和最终
  回答都会经过该事件，只有成功返回后才由 Controller 发无错误 `turn_done`。
- 当前补丁在根级 `message` 和 `turn_done` 两处运行 `projectVerifiedTaskArtifactPaths`，所以 receipt 之后的
  第一个进度候选即可拿到卡片。
- 历史只持久化 provider messages，不持久化 `turn_done`。现有 `MessageKind` 只标记 readiness 已拒绝候选；
  普通中间候选与真正接受的最终回答没有稳定区别，因此“每个用户轮次最后一个 assistant”无法严格排除
  失败、恢复中断或无最终回答的会话。

Codegraph 基于只读主工作区 `main-v3@33c926eb` 确认了
`Agent.Run -> Message/TurnDone -> event/history projection -> Item -> AssistantMessage -> ReadFile` 的数据流、
调用方和影响面。修复工作树已包含晚于索引基线的多轮修正，远端也已继续推进，因此索引只作为导航证据；
所有结论均以工作树源码、实际运行、测试和 Git diff 复核，进入 PR 阶段前再同步最新 `origin/main-v3`。

## 产品合同

主消息区的结构化文件卡片满足以下条件才发布：

`artifact_ready AND root_final_accepted AND root_turn_completed_successfully`

- `artifact_ready`：completed lifecycle 的 `artifacts` 与成功 `artifact_observed` validation 存在同路径交集。
- `root_final_accepted`：目标 assistant 是宿主明确标记的根任务最终回答，不由正文、顺序或提示词猜测。
- `root_turn_completed_successfully`：根回合无错误、未中断，且不是等待 plan approval 的阶段终点。

子专家侧的 `artifact_ready` 来源也必须是结构化且可归属的：带路径写工具以成功 writer receipt 为准；
不返回路径的写工具，只接受“本次子任务工作区增量”与“子专家最终答复中明确 Markdown 链接”的交集。
工作区增量本身不是交付声明，测试日志、覆盖率文件和其他未声明文件不能仅因发生变化就进入回执。

子专家完成后，文件可以立即出现在文件侧栏和子专家历史中；主消息区只在上述终态发布。失败、中断、仍在
运行、只有进度消息或没有最终回答时，不挂主消息卡片，也不伪造最终回复。

## 目标与非目标

### 目标

1. 保留已完成的自然语言门禁撤销，产物交付不依赖动作词、产品词或 prompt 白名单。
2. 保留宿主结构化证据交集、工作区 `ReadFile` 验证和验证后等价去重。
3. 把结构化路径只绑定到宿主接受且根回合成功结束的最终 assistant；根级中间 `message` 不发布卡片。
4. 用持久化的本地类型标记让 live、历史重载和漏失 `turn_done` 后的历史重建识别同一条最终回答。
5. 用全新 `wails dev` 专家团任务验收“中途无卡、最终有卡、重载仍在”。
6. 去除无路径写工具产物发现对 prompt 正则的依赖，同时要求子专家终稿明确声明该类交付路径。

### 非目标

- 不解析主专家正文或用户 prompt 来判断最终性或产物所有权；子专家终稿只用于声明无路径写工具产生的
  workspace-delta 候选，不改变根任务终稿身份。
- 不新增第二条 artifact 数据通道，不修改 task receipt/readiness schema，不扩大文件读取权限。
- 不自动切换窗口，不改变子专家历史永久可查看规则、文件侧栏时机或卡片视觉。
- 不把失败、运行中、已中断、plan approval 或无最终回答的根回合当作完成。
- 不迁移或猜测旧会话中未标记的最终回答；旧消息正文已有文件链接时，现有文本卡片行为继续有效。
- 不处理“历史子专家永久可查看”的独立问题。

## 选定方案

### 1. 明确持久化根任务最终回答

在既有 `provider.MessageKind` 中增加本地 `final_answer` 类型。`Agent.Run` 只在以下检查全部通过、即将成功
返回时给无 tool calls 的 assistant 设置该类型：final readiness、非空最终回答、executor handoff、待消费
steer。被拒绝或仍需继续执行的候选继续使用既有类型或普通类型，不能标成终稿。

该字段是现有本地 transcript discriminator，不发送给模型 API。Desktop 历史投影把它映射成
`HistoryMessage.finalAnswer`；controller-backed history 和只读 JSONL preview 使用相同规则。旧记录字段为空，
按未确认终稿处理，不做结构化补挂。

### 2. live 只在成功 `turn_done` 发布

前端在当前根回合内记录最后一个根级 assistant candidate id，但收到普通根级 `message` 时只完成消息展示，
不投影 artifact。`final_candidate_rejected` 清除对应候选。

收到无错误、未中断、且不处于 plan approval 的 `turn_done` 时，才把本回合最后候选标记为 `finalAnswer`，
再执行结构化产物投影。错误、中断、无候选或 plan approval 均 fail closed。若 Wails 漏失 `turn_done`，既有
terminal reconcile 会重载历史，由持久化 `final_answer` 恢复同一结果。

### 3. 投影只认显式终稿

`projectVerifiedTaskArtifactPaths` 继续只接受 completed lifecycle 中 `artifacts` 与成功
`artifact_observed` 的路径交集，但目标从“receipt 后最后一个 assistant”收紧为“同一可确认用户轮次内、
receipt 之后显式标记 `finalAnswer` 的根 assistant”。没有标记时不挂，不跨 user 边界，不伪造消息。

`AssistantMessage` 继续把结构化路径与正文候选逐一交给所属 tab 的 `ReadFile`；验证成功后按现有绝对/相对
等价规则去重，同一文件最终只显示一张卡片。

### 4. 无路径写工具采用“实际增量与终稿声明交集”

只要子专家注册表包含写工具，就在子任务开始时取得受限工作区快照，不再用
`artifactRequirementFromInput(prompt)` 决定是否采集。成功 writer receipt 中已有路径的文件继续直接进入
handoff，并保留 `writer` 来源优先级。

对仅由前后快照发现的 `snapshot` 候选，宿主只保留子专家成功最终答复中以 Markdown 文件链接明确引用的
路径；链接目标必须解析为本工作区内、且确属本轮新增或修改的普通文件。失败子任务、未引用的日志/YML、
隐藏工具目录和工作区外路径均不交付。子专家系统提示统一要求任何“作为交付物”的本地文件在终稿中给出
规范 Markdown 链接，不再枚举 create/generate 等动作词。

## 影响范围与交付切片

- 保留既有提交：错误自然语言门禁撤销，以及文件候选“先验证、后等价去重”。
- 修正终稿身份与历史投影：`internal/provider/provider.go`、`internal/agent/agent.go`、`desktop/app.go` 及相邻
  Go 测试。
- 修正 live/history 卡片发布：frontend `types.ts`、`sessionItems.ts`、`sessionState.ts`、
  `eventProjection.ts`、`historyProjection.ts` 及相邻回归测试；`Message.tsx` 只保留既有验证和去重行为。
- 补全无路径写工具发现：`internal/agent/subagent_tool_contract.go`、`task.go`、`parallel_tasks.go` 及相邻
  handoff 测试；不修改 artifact lifecycle、Desktop 或前端 schema。
- 任务 005 复开，形成一个新的 `fix:` 修正提交和独立完成回执；任务 003 随后重新执行应用验收。
- 新增任务 006，先做同一确定性输入的旧提交/修复提交 A/B，再重新执行任务 003 的全新应用验收。
- 未经用户另行授权不 push、不创建或更新 PR。

## 验收标准

- completed + `artifacts` + 同路径成功 `artifact_observed` 后，根任务仍运行或只发进度消息时，主消息区无卡。
- 根任务成功结束且最终回答正文不含路径时，最终回答显示一张可打开的结构化文件卡片。
- 最终正文也引用同一文件时，结构化绝对路径与文本相对路径经验证后只显示一张卡片。
- 根回合失败、已中断、等待 plan approval、最终候选被 readiness 拒绝或没有最终回答时，主消息区无结构化
  卡片；文件侧栏和子专家历史不受影响。
- 历史重载只在持久化 `final_answer` 上恢复卡片；未标记旧记录不猜测，正文链接仍按原逻辑显示。
- receipt 之前的 assistant、下一用户轮次以及 child assistant 均不接收路径。
- 未命中产物正则的普通实现提示中，无路径写工具新增文件且子专家终稿链接该文件时，completed receipt 含
  唯一 `snapshot` 产物；同轮未链接的日志/YML不进入 receipt。
- 相同输入下，未修复提交稳定缺少该产物，修复提交稳定包含该产物；A/B 使用确定性 provider/tool fixture，
  不把两次随机模型选择不同工具当作修复证据。
- 用户无需切换到子专家窗口，也不会自动切换。

## 验证与应用验收

实施阶段先建立旧红，再运行受影响范围：

```powershell
go test ./internal/agent
go test .
pnpm exec tsx src/__tests__/child-event-isolation.test.ts
pnpm exec tsx src/__tests__/assistant-file-links.test.tsx
pnpm exec tsx src/__tests__/attachment-display.test.ts
pnpm build
```

Go 检查按实际改动包和依赖关系收敛；第二条命令从 `desktop` 模块执行。应用验收从修复工作树精确 HEAD
启动全新 `wails dev`，分别执行一个带唯一文件名的针对性提示词和普通用户原话“帮我写个贪吃蛇”。全程
停留在主专家界面，记录以下观察点：子专家完成/主专家进度阶段无主消息卡片；根任务成功最终回复出现且仅
出现一张卡片；点击打开正确文件；切换任务并重载后仍只挂在同一终稿。另做确定性失败或中断负例。

不得连接 `9090`、不得使用发布版或云端未更新代码、不得复用旧历史任务。验收后精确停止本次开发进程。
同步最新 `main-v3` 后重跑受影响检查与关键应用路径，再按仓库规则运行最终 Gate。

任务 006 的 A/B 先使用同一个未命中正则的子任务 prompt、同一个无路径写 fixture、同一个子专家终稿链接
和同一个噪声文件集合：A 侧在父回执缺少目标文件，B 侧只包含终稿声明的目标文件。随后真实应用验收使用
全新 `wails dev` 会话；若模型实际选择带路径 writer，只能证明主链路未回归，不能冒充无路径分支的 A/B。

## 风险与回滚

- 最终类型若在 retry/steer/readiness 完成前写入，会把候选误当终稿；通过 Agent 分支级测试锁定只在成功
  返回路径设置。
- live 候选 id 若跨回合残留，会错挂前文；在 turn start、rejection、error/interruption 和终态统一清理，并
  用无本轮 assistant 对照测试约束。
- 旧会话没有 `final_answer`，结构化回填会保守缺失；这是避免历史误挂的兼容取舍，原正文文件链接、文件
  侧栏和子专家历史仍可用。
- 新修正可作为独立 `fix:` 回滚；不修改 artifact schema、外部 API、依赖或数据文件。
- 对所有 writer-capable 子任务取一次受限快照会增加目录遍历成本；既有隐藏目录、依赖目录跳过规则继续
  生效，并由聚焦基准/实际应用观察启动与完成时延。快照候选必须被终稿链接，避免把遍历结果直接当产物。
- 子专家若用无路径工具写文件却违反系统合同、终稿不声明路径，宿主会保守不交付；这是避免误挂日志和
  并发改动的明确取舍，不再用 prompt 正则或猜测补偿。
