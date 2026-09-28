# 任务 015：显式约束持久 task 调度

## 状态与依赖

- 状态：已完成；实现、自动化、主/顶层真实 Desktop 验收与最终 OpenSpec 验证均通过。
- 执行组：Serial-15；在任务014之后执行，除非用户另行调整顺序。
- OpenSpec：`openspec/changes/enforce-explicit-persistent-task-dispatch/`。
- 依赖：任务001的统一异步task、任务005的结构化回执、任务006的父turn计费、任务007的四并发、
  任务010的事件等待、任务012的成员状态，以及任务014不得被本任务重新引入模型级effort覆盖。
- 指派：实现子Agent使用`gpt-5.6-sol`、medium reasoning；主Agent复核实际diff、回执与真实应用验收。
- 实现与验收已获授权并完成；未授权push、PR、合并、OpenSpec sync或archive。

## 目标与预期产物

1. 让主Agent在已有持久task运行时显式说明新任务是否独立，避免开放自然目标中无意识拉起全部成员。
2. 复用现有持久job ID形成每次turn独立的`task_run_id`，同时保持一个子代理一个`subagentRef`、Session
   内多turn的现有模型。
3. 允许新task用`depends_on`记录同父Session成功run；父Agent负责wait、检查、整理和重新派发。
4. 将未声明并发、无效依赖和容量不足表现为中性未派发结果，不创建child或红色失败状态。
5. 交付Root Go合同/实现/回归、必要的Frontend投影/UI回归、本任务完成回执和真实Desktop验收证据。

## 源码与验收依据

- Codegraph索引基线：`D:\Reasonix@1477a102d6d679ed540498cc4db54fda0a1e645c`。
- 规划时worktree基线：`01de1121814c50e4342905877c92fb8e6b05173c`；以下结论已从worktree源码复核。
- `internal/agent/task.go`在`persistentTaskCapacityError`后才准备child；新门禁必须更早完成，确保未派发
  调用不会`PrepareFresh/PrepareContinue`。
- `internal/agent/managed_task.go`已定义四并发上限，并在`startManagedTask`获得`jobs.Job`；当前
  `formatTaskStart`只返回ref/status，没有暴露该次job ID。
- `internal/agent/subagent_tool_contract.go`集中解码普通和专家共享task参数；专家wrapper在
  `internal/agent/expert_team_tools.go`转换schema并负责可信member绑定。
- `internal/jobs.Job`已有独立`ID`、稳定`ControlID=subagentRef`、SessionID、kind、status和持久artifact；
  `wait_task/stop_task`仍按ref控制，run ID无需替代它们。
- 真实父Session`20260808-102038.936426100-gpt-gpt-5.6-luna.jsonl`证明测试/发布建议会在成品前完成，
  随后被父Agent混入验收进度；详见自然任务验收问题第9节。

## 允许范围

- `internal/agent`的共享task schema/decoder、工具说明、managed admission、启动/终态回执、evidence保护、
  通用主Agent调度提示与专家wrapper透传。
- `internal/jobs`中按父Session和job ID读取状态、恢复已持久run及保存依赖元数据所需的最小扩展。
- Desktop frontend的task结果projection/compact摘要/详情，以及直接相关聚焦测试。
- 本任务回执、OpenSpec tasks、总体requirements/plan/contracts/progress和验收问题状态。

## 禁止范围

- 自动等待队列、依赖完成后自动启动、workflow engine、Planner或Task Tree接入。
- 角色名、团队类型、prompt文本、文件扩展名或artifact类型启发式推断依赖。
- child之间直接传消息、复制reasoning/messages/tools、自动拼接下游prompt或用户直接与child聊天。
- 修改`parallel_tasks`临时批量执行、持久化分支或把它重新用于专家成员历史。
- 新建TaskRun Store、第三套ID、顶层Session、专家团专用Runtime或云端专家资产修改。
- 改变Provider选择、父/child计费归属、同ref锁、四并发上限、成员绑定或子代理effort合同。

## 实现合同

### 1. admission顺序

在任何child准备、ref分配、member绑定、running meta/event或job创建之前完成：

1. 解析并校验`depends_on`；
2. 检查当前running持久task；
3. 在有running时要求`independent: true`；
4. 检查四并发容量；
5. 只有全部通过才进入现有Prepare/start路径。

非派发结果均以nil tool error返回稳定结构：

- `dispatch_decision_required`：缺少并发意图或依赖仍running；
- `dependency_not_satisfied`：依赖failed/interrupted/缺失/非task/跨父Session；
- `capacity_wait_required`：四个task正在运行。

### 2. run与Session身份

- admitted task启动结果同时返回`subagentRef`和现有`jobs.Job.ID`对应的`task_run_id`。
- continue保持effective ref，但新job产生新run ID；wait终态回执返回对应run ID。
- `depends_on`只引用run ID，并仅接受当前父Session内成功completed的持久task；允许来自此前父turn。
- 跨父Session查询只返回泛化unsatisfied，不泄露label、ref、status、output或artifact。
- 新run仍绑定启动它的当前父turn/ModelTask；引用历史run不重新计费、不移动usage、不重开旧child。

### 3. 主Agent交接

- 通用tool/system合同要求父Agent只派发输入已具备的task。
- 真正独立于所有running结果的任务才设置`independent: true`。
- 依赖任务先`wait_task`，父Agent检查回执并编写包含必要摘要/产物的新prompt，再调用task并填
  `depends_on`。
- 早期建议/清单不得被当作成品后的实际核验；是否合格仍由父Agent判断，宿主不做语义推断。

### 4. UI与evidence

- 三种非派发结果默认显示中性紧凑摘要，不显示raw JSON、红色child失败、成员头像或空历史入口。
- 只有admitted child参与现有running黄色、completed绿色、failed/interrupted红色状态。
- task详情可显示run ID和依赖；不增加task graph、等待成员或新页面。
- 非派发结果不得成为completed wait receipt、artifact validation或父readiness证据。

## 验收标准

1. 无running时旧task调用不提供`independent`仍正常启动。
2. 有running时，缺失或false independence返回`dispatch_decision_required`且所有child侧副作用为零；
   明确`independent: true`可在容量内并发。
3. 四个running时第五个返回`capacity_wait_required`；任一终态后重试可以启动。
4. fresh与同ref continue分别返回不同run ID；切换会话、完整重启后仍可按原ID验证状态。
5. 同父Session历史successful run可作为依赖；running要求先wait；failed/interrupted/missing/foreign均拒绝。
6. 主专家自然完成“先产出、再实际验证”任务时，后者只在父Agent收到并整理前者结果后创建；真正
   独立的跨领域调查仍能并发。
7. 普通和专家团执行同一合同；专家member_name/ref历史恢复、状态点、父child投影和计费无回归。
8. `parallel_tasks`的schema、临时Session、聚合结果与重载行为无变化。

## 聚焦验证

实现时至少执行并记录实际命令：

```powershell
go test ./internal/jobs ./internal/agent
pnpm build
npx --yes @fission-ai/openspec@1.8.0 validate enforce-explicit-persistent-task-dispatch --strict
go run ./scripts/code_guard
git diff --check
```

如果实际diff改变`internal/expert`、`internal/boot`或Desktop Go包，按仓库比例化规则扩展对应包测试；
Frontend聚焦测试可在实现迭代和真实验收前运行，但最终PR Gate仍以仓库脚本分类为准。

## 真实Desktop验收

- 普通聊天：开放目标自然选择一个持久task；运行中再出现独立调查时允许显式并发，依赖工作先wait。
- 专家团：使用不指定工具/成员/步骤的产出+核验目标，检查主专家没有在成品前把清单当成实际验收。
- 历史依赖：完成一轮、进入下一父turn并引用旧run；切换和完整重启后仍通过同父Session校验。
- 负路径：running、failed/interrupted、伪造/跨Session依赖和容量上限均不创建头像/session，不显示红色
  child失败，也不泄露外部会话信息。
- 回归：同成员continue、stop/interrupted、成员只读历史、父transcript隔离、应用重启与普通
  `parallel_tasks`聚合。

## 完成回执

- 状态：OpenSpec 1.1至4.4全部完成。
- changed files：
  - Root合同/准入/回执：`internal/agent/{task.go,managed_task.go,subagent_tool_contract.go,tool_receipts.go}`及聚焦测试；
  - cache-stable共享调度提示：`internal/config/render_agent_header.go`、`internal/boot/{boot.go,build_config.go,boot_test.go}`；
  - jobs身份/持久化：`internal/jobs/{jobs.go,artifacts.go,artifacts_test.go}`；
  - Desktop投影/UI：`desktop/task_lifecycle_history.go`、`desktop/history_test.go`、Frontend `ToolCard`、
    event projection、类型、本地化、`taskDispatch.ts`及两项聚焦测试；
  - OpenSpec tasks与本回执/总体progress。
- 自动化结果：
  - 红测先证实缺失schema、run ID、session lookup、结构化容量结果、Frontend parser与evidence保护；实现后
    `go test ./internal/jobs ./internal/agent`通过；
  - `npx tsx src/__tests__/child-event-isolation.test.ts`为27/27，
    `npx tsx src/__tests__/task-dispatch-presentation.test.ts`通过，`pnpm build`通过；
  - `cd desktop; go test . -run TestHistoryProjectsTaskWaitStopAsPerTurnLifecycles -count=1`通过，
    `cd desktop; go test .`通过（`ok kwu/desktop 92.041s`）；
  - `npx --yes @fission-ai/openspec@1.8.0 validate enforce-explicit-persistent-task-dispatch --strict`通过，
    `git diff --check`通过；
  - `go run ./scripts/code_guard`中本任务新增的三个复杂度项已拆分消除；命令仍因任务014已记录的
    `internal/control/selected_expert_refresh.go`两项new和`internal/agent/task.go:subSinkForContext`一项worsened
    返回1，本任务未触碰这些函数。
  - 主审后续发现重载排序候选并已由红测证实：同ref连续完成`task-1`至`task-10`后，旧实现因sidecar
    文件名字典序让`WaitControlledForSession`错误返回`task-9 / dependsOn task-8`。重载现按既有
    `maxJobSeq`稳定恢复创建顺序，并以`startedAt/ID`处理并列；回归转绿，`go test ./internal/jobs`与
    `go test ./internal/agent`再次通过。未改变artifact格式、全局调度或运行中job顺序。
  - 首轮真实专家团验收暴露阻断缺口：主专家自然串行等待、检查并总结task-1后启动task-2，但task-2原始
    参数和sidecar均未记录`depends_on`。Runtime、schema和wrapper均正常；根因是此前只扩展了task工具说明，
    两条cache-stable system prompt构建路径未追加design要求的共享主Agent调度策略。
  - 旧`5353ac97`上，普通自定义prompt的legacy `Build`与selected expert的`buildAssembly`回归均因缺少
    `Persistent task dispatch`真实失败。现两条路径统一追加`config.PersistentTaskDispatchPolicy`，明确输入
    就绪、running时独立性、`wait_task`后检查并重写prompt、消费completed run时即使当前无running也用
    `task_run_id`填写`depends_on`，以及早期建议/清单不构成后续实际核验证据；定向回归转绿。
  - `go test ./internal/config -count=1`、`go test ./internal/agent -count=1`和完整
    `go test ./internal/boot -count=1`通过。boot包内任务014后仍期待模型可控effort的四条陈旧测试已先在
    基线证实失败，再更新为现行合同：`task`、`read_only_task`、`parallel_tasks` schema不暴露`effort`，旧
    JSON字段不覆盖自动/配置/模型覆盖profile，而配置`subagent_effort`仍决定实际child request；未改生产
    effort实现。
- 真实应用结果：顶层使用原生 Computer Use、开发Profile和`gpt/gpt-5.6-luna`在`53db2157`完成独立验收：
  - 自然专家团父Session`20260808-135834.894783100-gpt-gpt-5.6-luna`先派发task-1给测试专家
    `sa_20260808_135959_000000000_062ee3db670e`产出并实跑；父Agent随后`wait_task`、检查回执并重写
    下游prompt，再派发task-2给代码审查专家`sa_20260808_140150_000000000_722890ffef4e`。task-2原始
    参数与sidecar均为`depends_on:["task-1"]`，没有在成品完成前派发；父页面只显示父内容、外层task
    生命周期和两个成员入口，子历史可读且没有串入父transcript。
  - 普通父Session`20260808-130641.387050600-gpt-gpt-5.6-luna`已覆盖首个task兼容、显式独立并发、
    同ref续写和临时`parallel_tasks`聚合；完整退出并重启后，受控task-7至task-10落盘序列由
    `wait_task`正确恢复为最新`task-10`而非`task-9`。随后同Session成功依赖被接纳为task-11，原始
    参数、回执和sidecar均记录`depends_on:["task-10"]`；展开生命周期卡可见`Task run ID: task-11`
    与`Dependencies: task-10`。task-11 child meta仍为`gpt/gpt-5.6-luna / medium`。
  - 受控负路径父Session`20260808-140450.170243700-gpt-gpt-5.6-luna`验证缺失并发意图和running依赖
    返回`dispatch_decision_required`；四个admitted run占满后第五个返回`capacity_wait_required`；停止
    任一run后可再次接纳。interrupted、missing、foreign及明确标注的failed sidecar fixture均返回
    泛化`dependency_not_satisfied`。所有未派发调用均无ref/run/job/头像/历史副作用，默认UI为中性
    紧凑摘要且不显示raw JSON。
  - 会话切换与完整应用重启后，上述父历史、task卡、专家成员状态和只读child历史均恢复；普通
    `parallel_tasks`仍只有临时聚合结果，没有持久ref、member或run依赖行为。
  - 接受警告：受控task-11的child把文件路径传给只接受目录的`ls`，因此在文件真实存在时错误回答
    “不存在”。宿主工具返回和落盘文件均正确；该模型工具选择错误不改变task-11已被正确准入、记录
    依赖并完成的调度证据，也不在本OpenSpec的语义产出范围内，未用提示词或运行时修改掩盖。
- OpenSpec验证结果：
  - 完整性：16/16 tasks完成，8/8规范要求均有实现、测试和/或真实应用证据；
  - 正确性：并发意图、run身份、依赖校验、父Agent交接、四并发容量、中性UI、普通/专家共享合同、
    `parallel_tasks`与计费边界均与spec一致；
  - 一致性：实现复用现有job/ref/runtime和父Agent交接，没有引入自动队列、第二调度器、child历史注入、
    专家专用规则或Provider/effort变化；
  - CRITICAL：0；WARNING：1项已接受，即上文受控child误用`ls`，不属于本调度合同；SUGGESTION：0；
  - 当前命令：strict validate通过；`go test ./internal/jobs ./internal/agent ./internal/config ./internal/boot -count=1`
    全部通过；Frontend `pnpm build`通过，仅保留Vite既有的大chunk提示。
- 偏差、风险与交接：未增加队列、Task Tree、第二存储/调度器、Provider/计费/effort变化或child历史注入；
  `parallel_tasks.go`及其schema未修改。非派发nil-error回执被显式标为只读调度反馈，不能晋升父readiness。
  自然专家团已验证模型会按通用合同先wait、检查并重写下游prompt；宿主仍只保证显式声明和血缘准入，
  不推断业务语义或child内部工具选择是否正确。
- 后续missing/cross-parent依赖原生负路径确认调度层仍正确返回中性`dependency_not_satisfied`且不创建child/job，
  但父Agent的一句话总结被独立artifact readiness误判拒绝：普通“以及”触发whole-intent additive fallback，
  从“不要修改文件/不要复述JSON”重建伪artifact并诱导写文件。该问题归入既有OpenSpec
  `fix-readonly-artifact-readiness` follow-up；修复只收紧Root Agent artifact intent的capture/steer共享解析，
  不改变本任务的依赖准入、task结果、Provider、计费或UI合同。自动化已覆盖完整结构化拒绝链；最终原生
  Desktop在missing `task-999`与cross-parent `task-1`两条自然负路径均返回中性`dependency_not_satisfied`，
  没有child/job、readiness重试、writer或verifier，隔离用户工作区43文件聚合哈希前后完全一致。复审补充
  边界现进一步覆盖同一输入的显式目标与additive
  shorthand合并、后续file-list wording不吞newline shorthand，以及ASCII/中文礼貌逗号；action-leading additive
  续句也不再被普通与结构化parser重复计数，单输出readiness与真正多输出cardinality均有回归。mixed shorthand+
  action与replacement后续action现各归属一次，严格clause边界同时阻止后续source file-list泄漏；仍未扩大本任务调度合同。
- 后续全量原生验收的自然专家链已正确完成task-1/task-2与`depends_on`，但父overview在第10行跨入
  虚拟化时把测量回零误判成用户滚顶；714字accepted final虽已进入items和JSONL，却因range固定0–9而
  未挂载。该展示缺陷归入`fix-readiness-candidate-and-live-interruption`：红测60/62精确复现，修复仅保护
  同一expert overview的既有pinned语义，自动化已转绿；不改变本任务调度、task、wire、history或sidecar合同。
- 对上述展示修复继续复审与原生回访后补齐两项同范围边界：同一次9→10虚拟化测量可能连续两次回零，
  旧一次性guard在加强回归中为61/63；现改为只在五帧稳定窗口内保留pinned语义并在用户意图时立即取消。
  另一次专家中部回访中，history revalidation替换`u*`/`h*`消息身份并叠加暂时无挂载问题锚点，旧路径会
  清空最近位置并在persisted sidecar到达后force-bottom；禁用新重映射的全链路回归为62/64。现按问题
  文本及重复次序重映射身份、保留offset，空采集不再等同bottom；专家64/64与语义锚点24/24转绿。
  两项均只改frontend滚动投影和既有OpenSpec，不改变任务015的调度、wire、history数据或sidecar协议。
- 独立复审继续补强五帧guard仍活跃时的用户优先级：真实可冒泡wheel后立即滚顶，再紧接一次无用户reset，
  两次均保持顶部；无用户双reset仍保持底部。首稿64/65来自JSDOM `WheelEvent`默认不冒泡、React未收到
  `onWheel`，改为真实传播语义后67/67；生产实现无需修改，本次仅增加测试与OpenSpec回执。
- e514全量原生复验继续发现已虚拟化长overview的settle边界：同一assistant完成六项结果后从streaming原地settle，
  itemCount保持42，但终态TurnActions使row 33→34；新测量先把旧bottom解释成中部并清除`stick`，正式final
  因而不在视口。`toolCards` 3→2仅是mounted window计数，不作为归档证据。精确旧实现回归71/72，失败值
  为旧bottom 2140而新bottom 3100；现仅在same-view、已虚拟化且仍pinned的content commit复用既有五帧
  guard，72/72转绿，真实用户中部负例保持通过。该修复不改变本任务调度、wire、sidecar或虚拟化阈值。
- 主审随后指出首版回归通过重建items驱动`contentVersion`，未覆盖生产流式delta只增长独立`live`状态的路径。
  强化回归固定同一个items引用、仅增长`live.text`，并在五帧窗口内延迟测量：`48b4e354`为70/72，
  live阶段与后续settle均丢失bottom。现content pin effect与guard共享包含live text/reasoning长度的
  `pinnedLayoutVersion`，72/72转绿，live-only期间真实用户中部仍不被抢回。
- 同一原生回访的工具密集中部位于超高assistant行内部，mounted window不含question，旧DOM-only采集未发
  anchor并沿用bottom。该展示缺口在`fix-readiness-candidate-and-live-interruption`内以独立follow-up修复：
  仅真实用户滚动可从专家虚拟行测量建立最近问题identity与精确offset，程序化空窗仍保留已有anchor；
  旧基线73/76，新专家回归78/78并精确恢复offset 3800，语义锚点26/26、恢复8/8、虚拟化13/13通过；
  bottom/普通视图合同不变，不影响本任务调度实现。
- 独立复审先补连续手势与外部释放边界：`aa94e6bc`在单次pointer start后的第二个scroll停留于A，精确
  回归77/80；容器外pointerup又暴露79/80。pointer/touch现由start持续至window级end/up/cancel/blur。
  后续权威原生trace `09-anchor-failure-full-transcript-performance-trace.log`又证明单次wheel后的滚动批次仍未写入
  Controller：返回`seq=90992/91009`均为`anchorMode=bottom`且全段无`anchor.*`。`1b0adb77`精确回归
  89/92，后续scroll停留首个offset且重挂错误；wheel现使用固定五帧生命周期覆盖同批scroll/测量，键盘仍
  one-shot。复审加强回归从pinned bottom开始，仅发一次wheel并跨第二至第四帧更新稳定offset 4800，
  第五帧后的程序化5000不覆盖；该边界在`f86e47c3`已通过。但同一基线的layout-before-passive新用例令
  专家套件94/95：新semantic view会消费旧view尚未清理的wheel ref。现wheel同时记录origin semantic key，
  只有当前view匹配才允许虚拟fallback，过期与既有cleanup同时清理active/origin；精确回归95/95且重挂
  恢复4800；语义锚点26/26、恢复8/8、虚拟化13/13、Frontend类型检查/构建、Code Guard及strict均通过。
  该修复只改frontend瞬态滚动意图与既有OpenSpec，不改变本任务调度、wire、sidecar、虚拟化阈值或持久格式。
- 后续同一滚动OpenSpec复审发现，active same-view恢复自身的程序化scroll仍会依次把缓存A/X回写为
  question0/offset40、bottom及A/错误offset100，使sidecar persisted五帧恢复1100而非正确1024；旧专家
  回归98/101。`Transcript.handleScroll`现保留`onScroll`几何处理，但在同view pending且无真实用户意图时
  早退，不触碰Session锚点、latest position或presentation snapshot；wheel输入仍可取消并建立用户新位置，
  第五帧结束后的程序化采集照旧。专家102/102、恢复9/9、语义锚点26/26和虚拟化13/13通过；未改变
  本任务调度、wheel五帧、sidecar/wire/persistence或虚拟化协议。
- `fedfbef`原生单滚轮诊断又定位到虚拟总高在第四帧收缩：向上批次已连续捕获同一q5的
  295.4、155.4与47.4偏移，但浏览器把物理`scrollTop`钳到新max后，旧实现按bottom覆盖Session缓存并
  在返回时走`anchorMode=bottom`。精确旧基线专家回归107/109，仅缓存与物理恢复两项失败。现仅在同view、
  已虚拟化专家、active负向wheel且本批已有非bottom anchor的false→true钳制边界暂存最后anchor；正常
  第五帧过期时单次消费并复用既有五帧语义恢复，新pointer/touch/key/相反wheel/reveal/view/unmount均同步
  取消。专家125/125、语义锚点26/26、恢复9/9、虚拟化13/13及DEV诊断1/1通过；未改变任务调度、全局
  虚拟测量、80px阈值、wire、sidecar或持久格式。
- 对上述clamp修复的独立复审又发现两个同一生命周期缺口：candidate建立后的后续有效capture只更新
  gesture anchor，且同view history revalidation只重映射pending/latest/snapshot，导致到期仍恢复旧offset
  或旧identity并回底。最终fixture在`2da37ee4`为126/129，仅A/20仍恢复A/47及旧A未映射为A'的三项失败；
  现仅在candidate已存在时同步为本批`nextSemanticPosition`，并在既有questions layout remap内原子重映射
  两个wheel ref，不新增状态或候选建立路径。专家129/129、语义锚点26/26、恢复9/9、虚拟化13/13、两套
  TypeScript检查、生产构建、Code Guard及strict均通过；调度、虚拟器、80px阈值、wire/sidecar/persistence不变。
- 最终clean原生单wheel没有再次触发上述高度收缩clamp，因此保留其自动化与独立复审证据，不虚称原生
  正向覆盖。该轮另行固证了独立P1：返回时`team-legacy`与`team-persisted`对同一q5/`-606.2`连续执行
  两套五帧恢复，pending清除后的程序化scroll又把offset写成`-579.8`、`-42.2`，视觉位置明显漂移。
  当前任务不再继续叠加滚动补丁；复现、已证根因边界、未知项与后续候选设计验收见
  [专家会话返回双恢复与锚点漂移](../验收问题-专家会话返回双恢复与锚点漂移.md)。
