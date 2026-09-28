# 验收问题：专家团长历史切换触发 WebView 崩溃

## 现象

真实 Desktop 验收中，从包含大量消息和工具卡的子专家历史返回主专家，再快速切换到其他会话并返回时，
WebView 曾显示 `STATUS_BREAKPOINT`。宿主进程仍在，刷新可以恢复，父子 Session 持久数据没有丢失。

## 已确认边界

- 旧提交 `a60dc05f` 已通过 `@tanstack/react-virtual`限制普通 Transcript 同时挂载的 DOM 行数。
- 当前 `Transcript.tsx` 的 `virtualizeRegularRows`显式要求 `!expertWorkspaceActive`，专家团
  `AgentWorkspace`不走该保护。
- `AgentWorkspace`在主专家视图直接挂载全部 `overviewContent`；子专家视图最多恢复 200 条历史消息，
  再直接逐条渲染 `AssistantMessage`和 `ToolCard`。
- 本次复现恰好经过“长子历史 → 长主历史 → 其他会话 → 返回”的未覆盖路径。没有 crash dump，
  因而不把 Chromium 的内部断点原因表述为已证明；源码已证明专家团路径缺少旧修复的 DOM 上限。
- Computer Use 截图会增加渲染压力，但产品在该交互下崩溃仍属于 Desktop 稳定性缺陷。

## 期望行为

专家团主专家和子专家历史都复用现有 Transcript 虚拟化机制，只挂载视口附近的消息行；切换成员、返回主
会话、切换会话和返回时不得崩溃，也不得改变父子历史、成员栏、只读 composer 或滚动恢复语义。

## 修复边界

- 只处理专家团长历史的渲染数量、挂载和卸载压力。
- 不修改 task Runtime、事件投影、持久化、Provider、计费或成员状态。
- 子专家状态点实时不变绿和成员栏视觉作为下一独立问题处理。

## 验收路径

1. 打开已有长专家团会话，进入包含大量 assistant/tool 项的子专家历史。
2. 滚动历史，返回主专家，再切换到另一长会话并返回。
3. 连续重复成员切换、主/子返回和会话切换，确认 WebView 不出现 `STATUS_BREAKPOINT`。
4. 确认子历史仍只读、工具详情可打开、成员栏和 composer 不跳位，刷新后历史一致。

## 修复进展

- 已抽取普通 Transcript 既有虚拟行实现，并接入兼容 `AgentWorkspace`与当前持久
  `ExpertTeamSession`的 overview/focused child；仍只使用 Transcript 外层 scroll element。
- 长主/子历史有界挂载、深滚动非空、短历史直渲染、成员返回、会话scope重置、问题导航偏移、
  Transcript滚动恢复和生产构建均通过自动化验证。
- 首次真实验收仍在“中断历史验收…”与目标专家团会话回切时复现崩溃。进一步源码核验排除隐藏
  Transcript常驻，定位到团队sidecar未绑定会话scope：异步加载窗口会投影旧团队，返回目标会话又会
  先挂legacy workspace再替换persisted workspace。
- 后续修复先按`semanticViewKey + tabId`隔离并缓存sidecar，且短列表不再创建零行虚拟器；这能阻止旧
  会话污染，却仍会因Desktop每次侧栏切换生成新tab而让同一会话cache miss。
- 修后压力复验的首轮完整主/子/主路径通过，但连续短会话↔目标主专家回切从第2轮起仍出现
  `STATUS_BREAKPOINT`。真实74条父历史最终只有51个Item、13个顶层行；视口实挂4行、345个DOM元素，
  8轮组件回切的DOM、在途RAF和有效ResizeObserver数量稳定。因此剩余问题不能继续归因为未虚拟化
  的大DOM或数量线性泄漏。
- 已增加仅开发态的受限Transcript trace，记录opaque会话标识、normal/team路径、行数/挂载范围、
  scroll尺寸、可用时的JS heap、anchor/stick、virtual measure、ResizeObserver和受管RAF先后；不记录
  正文、prompt、reasoning或tool args/output。
- 三轮真实trace已经得到一致证据：目标会话的opaque view token始终为`v83ab7c86`，六次
  `active-tab.apply`的tabId均不同，每轮都发生`team-legacy → team-persisted`；与此同时DOM、
  ResizeObserver和RAF计数没有增长。剩余双挂载来自sidecar缓存把临时tabId错误当成归属的一部分，
  不是scroll/virtualizer计数泄漏。
- 缓存归属已收敛为稳定`semanticViewKey`（没有viewKey时仍回退当前tabId），桥读取和refresh继续使用
  当前tabId。回归测试证明同一semantic view换tab会立即persisted并向新tab后台刷新，不同semantic
  view不复用，迟到旧tab结果不污染。未改scroll、virtualizer、observer或动画策略；待第三轮真实压力
  复验确认WebView崩溃是否关闭。
- 第三轮真实压力验收中，WebView尚未再次崩溃，但发现新的阻塞问题：会话
  `20260807-134232.558038800-gpt-gpt-5.6-luna`的专家sidecar于`2026-08-08 12:06:50`
  被原子重写。仅凭Windows`CreationTime`不能证明此前无文件；隔离复现已经确认其故障前形态是只记录
  `expert_id/name/source`的旧schema。重写后的sidecar列出3名子专家，却全部缺少`subagent_ref`；
  `ExpertTeamForTab`过滤无所有权绑定成员后，页面只剩主专家“架构顾问”。
- 两个归属该父会话的持久化子Session仍在磁盘，分别对应“代码审查专家”和“测试策略专家”；因此当前
  证据是成员绑定/legacy恢复被覆盖，不是子会话历史被删除。真实样本已经被本次验收改变，不用于证明
  可重复性。

## 子专家头像消失复现

使用独立`KWU_DEV_PROFILE_ROOT`复制父JSONL、两个owned子Session和旧schema sidecar；未复制真实sidecar，
未修改真实会话目录。

1. 基线sidecar只含`expert_id/name/source`，SHA-256为
   `EAFB79992C246CEADFE34E02B9B368D2FEE9A8068EECCB1E47B70961CB025BC9`。
2. 父会话meta设置为现场相同的`scene=coding`、`scene_state=locked`。打开后UI走legacy恢复，明确显示
   主专家、代码审查专家和测试策略专家。
3. 切换到另一会话后，父JSONL与sidecar在同一秒`2026-08-08 12:26:25`被重写；sidecar SHA-256
   变为`6EF8EE2A30333871FFF641189CDAAA266EAE52111B952414CDDE11825EE0C7BF`，状态变为
   `team_active`，三个member均无`subagent_ref`。
4. 返回会话后UI稳定只显示主专家；两个子Session文件和ownership meta仍在。

源码链路与复现一致：`Controller.Resume`在locked scene重建system prompt，dirty autosave经
`persistSnapshot`调用`saveExpertMeta`；`priorExpertTeamRefs`要求previous已经是同一`team_active`，旧
schema无法保留绑定。前端`ExpertTeamForTab`又过滤无owned ref的member。据此先增加失败回归并确认最小
可信恢复边界，再继续WebView验收。

## 子专家绑定恢复实现

- Go回归已先在旧实现上稳定失败：旧schema在`Resume`后仍无`team_state`；已损坏的
  `team_active`不会补回唯一可信的owned child。
- 控制层现在于`Resume`内即时规范化旧schema，并修复已存在active sidecar的空绑定，不再等待切走或
  autosave。恢复候选必须同时满足：父JSONL中`task`调用含显式非空`member_name`、child meta的
  `ParentToolCallID`连接该调用、`ListSubagentsByParent`确认`ParentSession`且`Kind=task`；child JSONL
  还必须实际存在。普通无`member_name`任务、跨父child、非task child和无法验证的调用均不参与。
- 同一成员出现多个不同owned ref时视为歧义并保持空绑定；已有非空ref始终保持，不由历史候选替换。
  恢复只原子更新父`.expert.json`；测试逐字节验证所有child JSONL和meta均未创建、复制、删除或改写。
- `Resume`即时保存失败时只发warning notice，并保留磁盘与内存中的原meta，不宣称尚未持久化的
  `team_active`或ref。`saveExpertMeta`同样先尝试可信恢复，避免后续autosave重新覆盖旧schema。
- 聚焦恢复用例和`go test ./internal/control`通过；隔离Profile成员历史与WebView压力验收均已完成，
  结果见下节。

## 最终真实验收

- 在独立Profile先验证已损坏的`team_active` sidecar：首次`Resume`即恢复“代码审查专家”和
  “测试策略专家”的原`subagent_ref`；第三位没有唯一可信child的成员保持空绑定。
- 将sidecar重置为旧三字段schema，校验SHA-256仍为
  `EAFB79992C246CEADFE34E02B9B368D2FEE9A8068EECCB1E47B70961CB025BC9`后冷启动；首次
  `Resume`同样直接恢复两位成员和历史入口。两个child JSONL及ownership meta均未改写。
- 冷启动后连续完成5轮普通会话↔目标长专家团会话切换，进入并退出代码审查、测试策略两份长子历史，
  再执行父会话回切；renderer始终存活，未出现`STATUS_BREAKPOINT`、panic或fatal。
- 稳定会话在首次冷加载后均走`team-pending → team-persisted`，没有再次出现
  `team-legacy → team-persisted`。persisted主视图实挂6行、87个DOM元素和2个ToolCard；JS heap
  采样为38.37MB至71.49MB，最终52.41MB，没有逐轮单调增长。
- 真实开发Profile已使用同一构建恢复；目标会话重新显示主专家、代码审查专家和测试策略专家，sidecar
  持有上述两个可信ref，开发日志未发现崩溃或恢复错误。

据此，本验收问题中的“成员绑定消失”和“长历史切换触发WebView崩溃”路径已关闭。头像状态点实时颜色、
成员栏视觉和“登录已失效”属于独立问题，不纳入本修复结论。

## 诊断读取

- 开发终端筛选`[transcript-performance-trace]`可得到完整高频时序。
- 低频结构事件同步保存在`kwu:transcript-performance-trace:v1`的64KB受限环中；应用重启后可从
  DevTools执行`window.__transcriptPerformanceTrace?.export()`导出，`clear()`可清空。
- 生产构建不读写该存储、不输出日志，也不安装window API。
