# Desktop 单人聊天与专家团聊天子代理修复进度

## 当前状态

- 阶段：Stage 3 的15项任务及当前只读、rewind、项目专家历史 follow-up 已完成实现、自动化与独立原生验收；下一关口为最终 PR Gate。
- worktree：`D:\Reasonix-worktrees\plan-agent-session-turn-runtime`。
- 范围：按 Serial-01 至 Serial-15 小步实现；本回执不记录 Git/PR 状态，实际状态以 Git 和 PR 平台为准。
- 旧的“Agent Task Runtime”或“parallel 持久成员”方向不再是本主题需求。

## 文档权威关系

| 文档 | 当前职责 |
| --- | --- |
| [requirements.md](requirements.md) | 唯一产品需求、状态边界与 R-01 至 R-34 验收 |
| [plan.md](plan.md) | 复用当前源码的实现方案、阶段、风险与验证 |
| [contracts.md](contracts.md) | 数据、工具、事件、UI、evidence 和权限合同 |
| [evidence.md](evidence.md) | Codegraph、源码、复现与 WorkBuddy 交互事实 |
| 历史 OpenSpec：显式持久 task 调度（规范目录已移除） | 任务015的规范、设计与实施清单 |
| `progress.md` | 当前阶段、阻塞项和下一关口 |

任何从属文档不得覆盖 requirements；产品变化必须先修改 requirements 并重新确认。

## 已冻结决策

- 产品只有单人聊天与专家团聊天；单专家是单人聊天中的父 Agent 身份。
- 专家团不是新 Runtime，只是父 Agent 追加主专家 prompt，并把固定成员绑定到现有 task child Session。
- 一个子代理对应一个持久 Session，Session 可有多个 turn；同一 Session 只运行一个 turn。
- task 在父 turn 内统一异步，模型用 `task / wait_task / stop_task`控制；普通按ref，专家按member_name；
  每次admitted turn复用持久job ID形成独立`task_run_id`，同一ref仍可有多个串行turn。
- running child turn 绑定拉起它的父 `ModelTask`，父 final/结算前必须全部 wait/stop；同一父 turn
  最多并发四个不同 ref，同一 ref 仍只运行一个 turn。Session 可在后续父 turn 复用，新 child turn
  归属新的父计费任务。
- completed、failed、interrupted 都允许继续同一 Session；running 必须先 wait 或 stop。
- child 增量持久化；切换会话不取消，真正退出进程收敛为 interrupted，关闭到后台不影响任务，
  崩溃后不自动重放。
- 普通 task 默认 fresh，只有明确连续意图才复用；普通 UI 始终隐藏 child 历史。
- read_only_task 保持普通/单专家聊天的一次性临时只读研究，不产生 ref 或历史；专家团不提供它。
- 专家团只用task调度持久成员；已有running任务时，另一任务只有显式`independent: true`才可并发，
  最多四个。有依赖任务由父Agent先wait、检查并整理后再创建，以`depends_on`记录同父Session成功
  `task_run_id`；宿主不自动排队或在child间传话。parallel_tasks仍只是不产生成员历史的普通/单专家
  临时批量执行器。
- parallel_tasks 只在普通/单专家聊天使用，是临时批量 fan-out；只保留父聚合结果，无 ref/历史页。
- 父 transcript 只显示父 Agent 与外层 task/parallel；child 事件在 projection 入口分区。
- 只有专家团头像能打开同一父任务壳内的连续只读 child Session。
- 团队退出后父身份恢复基础 Agent，团队标识与历史保留只读，同会话不得再次召唤专家。
- fork/rewind 只处理父历史；新时间线不复制或复用旧 child，下一次派发 fresh。
- child 工具按 ref/raw ID 查询；父 readiness 只信任 wait_task 的宿主结构化完成回执，running child
  同时阻止父 final 与计费结算。
- child 不直接与用户聊天、不能创建孙代理；Task Tree 只负责任务 DAG。
- 父会话移入回收站前停止 running child，父/child 文件一起移动和恢复；永久删除时一起清理。
- 不迁移、不推断、不补造旧 parallel 或旧成员历史。
- Desktop 非简单工作由顶层父 Agent 在关键阶段通过普通 assistant content 提供简短工作更新；不暴露
  私有思维链、不逐工具播报、不新增消息协议或定时心跳，child 提示词保持不变。
- 专家团与普通聊天共用同一个 task runtime。专家层只做可信 `member_name -> subagentRef`映射，并
  省略逐次 `tools`输入，使成员继承完整子代理安全工具；普通 task的工具白名单保持不变。
- 不迁移旧子代理资料；同版本同配置必须支持重启续写，真实工具/schema变化仍严格拒绝并保留历史只读。
- child task不设总时长；OpenAI兼容单请求采用120秒网络空闲、5分钟无进展、10分钟绝对上限和
  5秒finish收尾窗口，完全无输出时最多安全重试一次。
- 普通与专家`wait_task`统一由宿主事件等待到终态或父取消，模型不再选择轮询间隔。

## 当前验证

- 已在最新 Codegraph 索引核验 task、read_only_task、parallel、SubagentStore、jobs、expert wrapper、ExpertMeta、
  event/history projection、Transcript、AgentWorkspace、ToolCard、child history/tool 路由和 evidence。
- 已核验 Desktop 现有回收站通过 `trashSubagentArtifacts/restoreSubagentArtifacts`整体移动和恢复
  child，`PurgeTrashedSession`永久删除 trash item。
- 已用用户提供样本和真实 Desktop 复现核对普通 task 串页及专家 parallel 切换后消失。
- 已用 WorkBuddy v5.3.8 只读操作核对同壳历史页、清除身份、历史保留和召唤限制。
- 任务 001 已完成统一异步 task 生命周期、ref 控制、增量保存和终态续写；jobs、agent、boot
  受影响包全包测试通过，代码守卫和 diff 检查通过。完整命令见任务 001 回执。
- 任务 002 已完成父/持久 child/parallel 三路事件分区；普通 task child 不再进入父 transcript，
  parallel 只显示外层聚合卡和临时分支状态，实时/历史投影不再生成 child/parallel 专家页。
- 任务 003 已完成团队 active/archived sidecar、固定成员到持久 task ref绑定、宿主可信续写、
  按成员 wait/stop、归档停止 running turn、重载恢复与 fork/rewind活动绑定重置；聚焦测试和代码守卫通过。
- 任务 004 已完成sidecar成员条恢复、同壳只读child Session、父所有权校验、child归档工具详情路由、
  内部身份wrapper清洗与归档历史保留；UI聚焦回归、Frontend生产构建、父子回收站聚焦测试和代码守卫通过；
  后续包级Session API guard发现的legacy分页直接字段写入已另行修正。
- Desktop `go test ./...`当前被基线测试fixture的 `fixture/model_fixture_01`配置错误阻断；同类既有测试在
  未含本分支改动的主工作区可独立复现，详情见任务004回执。
- 任务005的结构化回执/evidence子切片已完成：completed `wait_task`输出宿主观察的artifact与validation，
  仅completed回执成为父执行进展，非终态/失败/中断不进入父完成证据；child ledger仍不复制。
- task生命周期UI已完成：实时与历史重载都把`wait_task/stop_task`回执归入对应task卡，支持同ref多turn
  和跨分页后合并；找不到对应task时控制项保持可见，父JSONL不删改真实调用。
- 受影响Root包、Desktop聚焦测试、两项Frontend聚焦测试、生产构建、Wails正式构建和代码守卫通过。
  Desktop全包和Frontend全量脚本各有一个已在主工作区独立复现的基线失败，详见任务005回执。
- 顶层线程的 Computer Use 已恢复并唯一定位本工作树原生窗口；使用开发环境和
  `gpt/gpt-5.6-luna`完成普通task、parallel、专家团双成员、同ref续写/并发拒绝、切换/重启、
  child历史与工具详情、团队归档、归档后普通task隔离、stop/interrupted和回收站restore验收。
- 真实请求暴露裸模型名可能被`Config.ResolveModel`选到首个同名Provider；已改为优先沿用父Provider，
  显式Provider引用保持不变。`go test ./internal/boot -count=1`通过。
- 后续真实验收发现严格计费把当前 Session 级异步 task 当成跨 turn background 而全面拒绝；已确认
  修复方向是保留异步工具合同，把 child job 收紧到父 turn 并绑定父 `ModelTask`；运行时修复及测试已完成。
- task 现在从父 turn 上下文派生，保留 `running/ref → wait_task/stop_task`合同；父取消级联、运行中
  child 占用父计费 barrier并阻止父 final。任务007移除过度保守的父级单 task 门禁，但不改变这些
  生命周期边界。工具超时包装器返回不会误取消 child。
- 专家 sidecar/view 已保存 lead 快照并只暴露当前父拥有的已绑定成员；前端首 tab 显示 lead 身份，
  未派发子专家不再预显示。真实复验又发现空成员 `null`导致 React 崩溃、子历史仍允许操作父
  composer，已分别修复。子历史沿用 WorkBuddy 式稳定 composer 外壳：保持原尺寸，禁用并弱化内容，
  中央覆盖“返回主会话继续对话”。Root 受影响包、完整 boot、Desktop history、Frontend 11/11 与
  生产构建通过。
- 真实 Desktop 使用 `gpt/gpt-5.6-luna`完成普通 task→wait、专家团 task→wait、父 transcript 隔离、
  lead 身份、成员按绑定出现、只读子历史、会话切换和完整应用重启恢复；最新构建还确认子历史
  composer 外壳不缩放、覆盖按钮可返回主会话；标记为
  `PARENT-NEW-1549/CHILD-NEW-1549`及`LEAD-FIX-1558/MEMBER-FIX-1558`。
- 上述限定 prompt 结果已降级为定向链路冒烟。自然 prompt“给我做一个贪吃蛇”进一步暴露：父页面
  缺少中间过程叙述、第二个 task在首个 running 时仍先申请批准后失败、同成员 tool scope 变化导致
  续用失败、sticky 成员栏层级异常和暗色状态点语义不明。详见
  [自然任务自主调度与专家成员栏](验收问题-自然任务自主调度与专家成员栏.md)。
- Composer 已把模型与 session-local 推理强度收敛为一个 Codex 式两级选择器，保留既有
  `auto/low/medium/high`运行时合同；一级和两个二级面板已在真实 Desktop 检查。面板高度变化时
  `AnchoredPopover`会在绘制前同步重测，未再观察到先向下出现再跳回上方的闪烁。
- 父 Agent 可见工作更新已按确认边界实现：仅 Desktop 可选场景顶层 prompt 注入缓存稳定策略，
  基础父 Agent、单专家和主专家共用；旧无场景入口与 child 默认 prompt 不变。聚焦测试和
  `internal/config`、`internal/boot`、`internal/agent`完整包测试通过，Wails 正式构建通过；自然任务
  验收矩阵已写入任务006，任务007已覆盖其中的独立并发与依赖串行场景，其余场景尚未全部执行。
- 任务007已把父 turn 的持久 task 上限从一个改为四个不同 ref，同一 ref 锁、父取消、计费 barrier、
  final readiness和多 ref wait合同保持不变；第五个调用在创建 child/job 前拒绝。团队通用运行规则
  只保留在 Root Go Session system prompt，Frontend每回合隐藏块只保留成员名和语言元数据。
- 真实 Desktop 独立自然场景在父 Session
  `20260807-121418.246063500-gpt-gpt-5.6-luna`同一 turn 成功并发代码审查、测试策略和发布风险三位
  child并统一等待；依赖自然场景在父 Session
  `20260807-122125.508752500-gpt-gpt-5.6-luna`先 wait代码审查完成，再启动测试策略。旧的第二 task
  拒绝未再出现。后续写入修复仍命中既有同成员 tool scope不一致，保持下一独立问题。
- fork/rewind因Computer Use无法可靠触发hover消息菜单，仅保留既有Desktop自动化验收；永久删除按
  安全约束未在没有即时确认时执行，R-26由既有父/child联合清理测试覆盖。详细矩阵与Session/ref见任务005。
- 任务010的OpenAI兼容stream watchdog、无输出一次安全重放、finish grace和普通/专家统一事件等待已完成；
  `internal/provider/openai`、`internal/provider`、`internal/agent`和`internal/boot`聚焦包测试通过。
- 任务010真实应用验收完成：自然专家团请求并发两位child后只调用一次`wait_task`，等待期间父JSONL
  不增长，两个child completed后父Agent只唤醒一次并完成整合；父页面不串child内部工具，成员只读历史、
  页面刷新和既有ref恢复通过。该自然turn按持久assistant记录为父10次、child共10次模型请求；会话累计
  telemetry为61次/1,676,933 raw token，因未保存turn前快照，不把累计值误写成本轮独立用量。
- Windows当前CGO未启用，Provider `-race`未执行；这是环境限制，不记录为通过。
- 任务011已把普通 Transcript 的 `@tanstack/react-virtual`行机制抽成共用组件，并接入兼容
  `AgentWorkspace`和当前持久`ExpertTeamSession`的主/子历史；长历史有界挂载、深滚动、短历史、
  成员/会话切换、问题导航偏移、滚动恢复聚焦测试和Frontend生产构建通过。首次真实回切仍崩溃，
  后续定位并修复未按会话scope隔离的团队sidecar和legacy→persisted重复重挂载；20项新回归测试通过。
  第二轮真实压力验收首轮通过、连续回切第2轮起仍崩溃；实际DOM/RAF/observer计数稳定。开发态隐私
  受限环形trace进一步证明同一稳定semantic view因每次新tabId而反复`team-legacy → team-persisted`。
  sidecar缓存现只按稳定semantic view归属，桥刷新仍走当前tabId；同view换tab立即复用、跨view隔离、
  迟到结果保护和tab待就绪轻量等待共28项聚焦测试通过。隔离Profile的旧schema与损坏active sidecar
  均恢复两位可信成员；5轮主/子及会话压力切换未再崩溃，真实开发Profile头像与历史入口已恢复。
- 任务012已增加持久task meta保存后的内容无关状态事件，Frontend用该事件触发既有团队sidecar权威
  重读；未知状态不画点，running黄色无外圈，completed绿色，failed/interrupted红色。同ref再次派发
  回到running，超快child不依赖父task卡，且未增加状态轮询或`parallel_tasks`状态。Root受影响包、
  Frontend 12/12、18/18、32/32、16/16聚焦测试和生产构建通过。真实Desktop中同一测试策略专家完成
  绿色→黄色→绿色、再次派发黄色→父停止红色；切换会话与完整重启后红色均从child meta恢复，
  WebView未崩溃。
- 任务013已保留成员栏位于 Transcript footer 的现有结构，把透明渐变改为全宽主题不透明底板，
  并移除常态成员按钮的独立 elevated背景与阴影；后续补充低对比度主题中性底色，避免未选中成员
  在浅色主题中与底板融为一体。最终选中和未选中态共用同一浅灰底色，选中仅增加主题前景色细边框，
  hover/focus加深灰色；头像和黄绿红状态保持。
  persisted/legacy/虚拟化聚焦测试与Frontend生产构建通过。真实Desktop在746px窄宽下完成长历史
  上下滚动、滚到底部、主/子专家、普通/专家会话切换；最后内容可达，TodoPanel/Composer顺序不变，
  未出现成员栏串页、布局跳变或WebView崩溃。
- 最新自然验收使用普通持久task、专家团双持久task和三个临时只读并行任务覆盖父子投影、结果交付、
  成员状态、子历史、用户中断、会话切换及完整应用重启。重启后又完成两轮普通/专家父/专家子往返，
  日志无`STATUS_BREAKPOINT`、panic、fatal或out-of-memory，诊断heap未随往返单调增长。真实模型在
  临时只读场景选择`read_only_task`而非`parallel_tasks`，因此只认定临时并行产品边界通过，不把
  `parallel_tasks`真实模型选择路径写成已覆盖。
- 后续开放目标“帮我写个贪吃蛇游戏，吃的是苹果，速度快一点的”证明仅靠模型隐式判断依赖仍不足：
  实现child实际写文件，测试/发布child在无成品时零工具返回通用清单，父Agent随后把清单混入实际验收
  进度。已确认任务015的通用修复：显式独立并发、run级依赖、父Agent交接和三类中性未派发结果；
  不按角色特判、不增加队列/Task Tree/第二Runtime，详见自然任务验收问题第9节。

## 任务与交付映射

| 任务 | 串行组 | 状态 | 独立交付 |
| --- | --- | --- | --- |
| [001-统一持久task生命周期](tasks/001-统一持久task生命周期.md) | Serial-01 | 已完成 | Root task Runtime |
| [002-隔离事件并收敛parallel展示](tasks/002-隔离事件并收敛parallel展示.md) | Serial-02 | 已完成 | 事件/Frontend隔离 |
| [003-持久专家团成员绑定](tasks/003-持久专家团成员绑定.md) | Serial-03 | 已完成 | Expert sidecar/绑定 |
| [004-恢复专家历史与工具详情](tasks/004-恢复专家历史与工具详情.md) | Serial-04 | 已完成 | Desktop历史/详情/回收站 |
| [005-闭合task回执与整体验收](tasks/005-闭合task回执与整体验收.md) | Serial-05 | 已完成 | Evidence、生命周期UI、真实Desktop验收与Provider修正 |
| [006-修复验收缺陷与父turn计费](tasks/006-修复验收缺陷与父turn计费.md) | Serial-06 | 部分真实复验通过 | 父turn计费、专家身份、按需成员与只读历史 |
| [007-恢复多task并发与精简专家团提示词](tasks/007-恢复多task并发与精简专家团提示词.md) | Serial-07 | 已完成 | 最多四个持久task并发与单一运行时提示合同 |
| [008-固定专家成员工具能力](tasks/008-固定专家成员工具能力.md) | Serial-08 | 已完成 | 专家层省略逐次tools并复用默认安全工具集 |
| [009-保留中断父回合历史](tasks/009-保留中断父回合历史.md) | Serial-09 | 已完成 | 保留中断历史、续聊安全与重载一致性 |
| [010-终止无进展子请求并事件等待](tasks/010-终止无进展子请求并事件等待.md) | Serial-10 | 已完成 | Provider终止保护与无模型轮询等待 |
| [011-虚拟化专家团长历史](tasks/011-虚拟化专家团长历史.md) | Serial-11 | 已完成 | 专家团主/子历史DOM上限与切换稳定性 |
| [012-修复专家成员状态点](tasks/012-修复专家成员状态点.md) | Serial-12 | 已完成 | task成员状态三态显示、实时更新与重载恢复 |
| [013-稳定专家成员栏布局](tasks/013-稳定专家成员栏布局.md) | Serial-13 | 已完成 | Transcript底部单层不透明成员栏与平面按钮 |
| [014-锁定子代理推理强度](tasks/014-锁定子代理推理强度.md) | Serial-14 | 已完成 | 设置页强度作为三个子代理工具唯一来源 |
| [015-显式约束持久task调度](tasks/015-显式约束持久task调度.md) | Serial-15 | 已完成 | task显式独立并发、run级依赖与父Agent交接 |

任务按表中顺序执行。每项先完成聚焦验证和任务回执，再创建一个 coherent intent commit；前置任务
暴露的缺陷使用单独 `fix:`提交。Git/PR 当前状态不在本回执维护，以 Git 和 PR 平台为准；不得自动合并。

## 阻塞项

- 父 Agent 可见工作更新、多 task 调度和专家团提示词重复均已完成实现与自然复验。
- 真实停止父 turn暴露旧 `stripTurnMessagesAfter`会永久删除整轮父历史；任务009已改为保留 transcript
  并追加持久中断边界。普通会话真实停止、切换、完整重启和无自动重放均通过。后续独立 OpenSpec
  `fix-live-turn-cancellation-notice`已修正实时投影：仅当用户主动停止且以标准取消错误结束时立即显示
  `本轮已中断`；未主动停止的取消错误和停止期间的真实错误仍保持可见。reducer回归、类型检查和生产
  构建通过。主会话已用 Computer Use 在真实 Wails dev应用中新建自然会话，待模型产生 assistant内容、
  发起技能调用并进入审批等待后主动停止：当前页面立即显示`本轮已中断`；切换到另一会话再返回后仍
  显示同一文案，且可访问树均未出现`context canceled`。该修复未混入任务008范围。
- 同成员 tool scope已通过真实自然任务：同ref从只读调查续写到写入与回读，两轮schema hash一致；
  成员栏视觉问题继续保持独立。
- 真实验收新发现供应商心跳可让OpenAI SSE idle watchdog持续续期，最终回执无进展超过十分钟；模型
  显式60/120秒`wait_task`返回running后会重新请求并放大成本，任务010按已确认合同处理，详见
  [供应商心跳导致最终回执无限等待](验收问题-供应商心跳导致最终回执无限等待.md)。
- 旧 Transcript 虚拟化显式排除了专家团路径；长子历史返回主历史后快速切换会话曾触发
  WebView `STATUS_BREAKPOINT`。任务011已补齐专家团DOM挂载上限、稳定sidecar归属与等待态，并恢复
  旧schema/损坏active sidecar的可信成员绑定；隔离Profile和真实开发Profile复验均通过。
  成员状态点延后独立修复，
  详见
  [专家团长历史切换触发WebView崩溃](验收问题-专家团长历史切换触发WebView崩溃.md)。
- 任务014已完成代码、自动化与真实Desktop验收：三个schema移除`effort`，旧参数无错误忽略，配置
  medium统一进入事件profile、Provider解析和持久identity，auto与模型覆盖合同保持不变。真实普通
  自然任务在父Session`20260808-121931.925323700-gpt-gpt-5.6-luna`自主派发child
  `sa_20260808_122159_000000000_8e8eb39a2fd1`；原始task参数无effort，child以
  `gpt/gpt-5.6-luna / medium`完成9次Provider请求、实际读写与验证，父页面隔离和最终完成态正常。
  账号区域仍显示“登录已失效”，但自定义Provider真实可选和可发送，未再把该文案当作应用阻塞。
  Root Agent包测试、OpenSpec严格校验和diff检查通过；`code_guard`仍报告本任务未触碰的既有复杂度项。
- 任务015已完成共享持久task调度合同、两条cache-stable父Agent策略装配、run级依赖、重载排序和中性
  未派发UI。真实自然专家团Session`20260808-135834.894783100-gpt-gpt-5.6-luna`先wait并检查task-1，
  再以`depends_on:["task-1"]`派发task-2；普通/受控Session还覆盖显式并发、4/5容量、全部依赖终态、
  同ref续写、task-10重载、详情UI、完整重启和`parallel_tasks`临时聚合。OpenSpec 16/16完成，strict
  validate、Root四个受影响包和Frontend生产构建通过。
- 前一轮统一原生回归证据位于
  `C:\Users\ai03\AppData\Local\Temp\pr344-native-fix-20260810-012234`：专家等待成员时停止在700 ms观察内
  揭示父中断提示，成员保持红色并以“已中断”区分真实错误；readiness候选在实时、复制、切换和重启后
  均不回流；完整重启后八轮父/成员切换、Composer两级选择器和三路临时只读并行均通过。工作区前后
  快照差异为零，日志无`STATUS_BREAKPOINT`、panic、fatal或out-of-memory。
- 最终当前版本复验位于`C:\Users\ai03\AppData\Local\Temp\pr344-d2a15-native-20260810-032335`：专家中断在
  约900 ms内揭示，成员保持红色且aria/title为“已中断”；两次离开/返回均恢复各自最新中部语义锚点，
  八轮普通/专家父/成员切换稳定；readiness重启后仍只有一条accepted final，原始JSONL保留类型化拒绝候选，
  用户JSON导出不含瞬态揭示字段，工作区零写入。保存对话框后Windows拒绝Computer Use的屏幕捕获和光标
  权限，因此仅未重复当前版本的专家中断会话导出、重启后专家/parallel UI打开和模型子菜单视觉冒烟；这是
  捕获基础设施限制而非产品FAIL，核心受影响链路均已有原生、可访问树或持久化证据并通过。
- 后续全量专家链发现accepted final已进入父items/JSONL但未出现在实时视口：trace显示overview由9行跨到
  10行虚拟化时，虚拟器测量把`scrollTop`从bottom归零并误清`stick`，最终row 18落在未挂载range 0–9之外。
  当前前端修复只在同一expert workspace的直接渲染→虚拟化边界保留既有bottom语义；程序化回零/最终
  挂载和真实用户滚顶负例均有自动化覆盖，仍需随最终原生矩阵复验。
- 同一全量回访还暴露了虚拟化连续回零与history revalidation消息身份替换边界：pinned保护现限定在五帧
  稳定窗口并覆盖连续两次程序化reset；同view中部anchor按问题语义/重复次序从`u*`重映射为`h*`并保留
  offset，暂时没有挂载问题节点不再清除最近有效位置。加强回归分别先红后转绿，仍待最终原生矩阵复验。
- 五帧guard活跃期的用户优先级也已补精确自动化：冒泡wheel后立即滚顶并追加第二次程序化reset仍保持
  顶部，无用户双reset继续跟随底部。生产行为已满足，新增回归67/67通过；此次不改生产代码。
- e514原生长final复验进一步证明：overview已经虚拟化后，同一assistant从streaming原地settle时itemCount
  保持42，TurnActions却令row 33→34；测量scroll在passive pin前清除`stick`。现将既有五帧guard扩展到
  same-view、已虚拟化且仍pinned的content commit；精确回归由71/72转为72/72，真实用户中部不被抢回，
  仍待原生复验。另一个“工具密集中部无已挂载question anchor”回访缺口已固证，作为下一独立follow-up处理。
- 主审补强后确认首版回归未覆盖独立live状态：固定items引用、仅增长`live.text`并延迟测量时旧提交为
  70/72。现同一`pinnedLayoutVersion`同时驱动既有content pin effect与五帧guard，包含live text/reasoning
  长度；强化回归72/72、用户中部负例通过。
- 工具密集的超高assistant行中部可能没有挂载任何question DOM，旧路径因此不更新仍为bottom的视图快照。
  现仅在真实用户滚动意图且专家overview已虚拟化时，从虚拟行测量选择最近前置问题并保存既有identity/
  offset；程序化空窗不覆盖第7节有效anchor。旧基线73/76，新集成精确恢复offset 3800并保留bottom负例。
  专家78/78、语义锚点26/26、恢复8/8、虚拟化13/13及Frontend类型检查/生产构建均通过。
- 高行虚拟锚点现按输入类型使用有界生命周期：pointer/touch持续到window级end/up/cancel/blur，键盘保持
  one-shot；单次wheel使用固定五帧窗口覆盖同批多个scroll/测量。权威原生trace显示旧路径返回时
  `seq=90992/91009`均为`anchorMode=bottom`且没有`anchor.*`，位置从未写入Controller；精确回归在
  `1b0adb77`为89/92。复审加强用例从bottom仅发一次wheel，第二至第四帧持续更新offset 4800并在第五帧
  到期，该路径在`f86e47c3`已绿；但layout-before-passive跨view用例令同基线为94/95，新view错误消费旧wheel。
  现以origin semantic key同步门控，不依赖passive cleanup顺序；精确回归95/95且重挂恢复4800。该状态仅存于
  Transcript组件生命期；语义锚点26/26、恢复8/8、虚拟化13/13、Frontend类型检查/构建、Code Guard及strict
  均通过，不改变wire、sidecar、虚拟化阈值或持久格式。
- 最终clean单wheel复验保留了semantic q5/`-606.2`，但返回时`team-legacy`与`team-persisted`各执行
  一套五帧恢复；第二套结束后程序化scroll又把offset写成`-579.8`、`-42.2`，视觉位置明显漂移。
  已证问题是固定完成条件过早清除pending，随后晚到程序化scroll重新开放写回；不是bottom fallback或
  identity丢失。单一restore owner只是后续设计候选，具体晚到scroll来源、StrictMode必要性及最终机制尚未证明。
  本交付范围不再修改该路径，完整证据、复现、根因边界和
  后续验收见[专家会话返回双恢复与锚点漂移](验收问题-专家会话返回双恢复与锚点漂移.md)。
- 后续受控跨Session依赖拒绝发现独立artifact readiness误判：用户只要求一句中文结果且禁止修改文件，
  普通连词“以及”却触发whole-intent additive fallback，从否定的“文件/JSON”重建两个artifact并诱导写入。
  `fix-readonly-artifact-readiness` follow-up现以共享结构化续句解析器修复capture与steer；精确拒绝链、普通连接词、
  礼貌前缀边界、tail removal、carried shorthand和source-readonly命名输出回归已转绿。复审补充的混合显式目标+
  shorthand合并、后续file-list不吞newline边界、ASCII/中文礼貌逗号也已由capture与steer回归覆盖。action-leading
  additive续句现只由普通action parser产生目标，单个XLSX/generic验证即可满足单输出，同时保留真正多输出的distinct
  cardinality。mixed shorthand+action现按首个未否定action分段，各目标只归属一次；replacement当前clause与后续
  strict action clauses分开解析，不丢后续输出，也不把更后的source file-list吸收为交付。最终原生验收在missing
  `task-999`与cross-parent `task-1`两条自然负路径均得到中性`dependency_not_satisfied`，没有child/job、readiness重试、
  writer或verifier，隔离用户工作区43文件聚合哈希前后完全一致。
- 同步最新`main-v3`后补齐项目会话入口的专家成员历史隔离：项目Transcript复用普通入口的focus状态，成员历史中
  项目Composer不可编辑/提交且仅提供返回操作；返回、普通/项目切换与完整重启均恢复正确。conversation rewind同时
  验证旧成员即时/切换/重启后不复活，旧child/job按合同保留，同名成员后续派发使用fresh ref。

## 下一关口

1. 在同步后的最终树运行完整 PR Gate，并保留分类、测试与构建结果作为 PR 更新依据。
2. 双恢复/offset漂移只在后续独立OpenSpec中比较“单一restore owner”等候选机制，并以稳定完成条件红测决定方案；本交付范围不得继续叠加滚动补丁。
3. OpenSpec尚未sync/archive；push、更新Draft PR、标记Ready与合并仍分别以显式指令为准，不得自动执行。
