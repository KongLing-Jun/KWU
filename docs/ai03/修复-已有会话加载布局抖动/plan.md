# 已有会话加载布局抖动修复计划

本计划超过约 200 行审阅触发值，是因为同一原子过渡同时约束布局、草稿、导航竞态、精确摘要和应用验收；执行结果与 Git/PR 状态留在任务回执和外部事实源，不在此重复。

## 1. 背景与问题

在 creation 布局中，打开尚未缓存的已有会话时，页面会先把空的加载态误判为新建会话，使 Composer 出现在中部；历史返回后才切回普通会话布局，Composer 随之落到底部。

应用验收还证明了两个同源边界缺陷：

- “新建对话”创建的零消息会话已有持久化 `sessionPath`；从左侧全局任务或工作空间项目树重开同一会话时，当前实现仅凭非空路径把它误判成已有内容会话，导致同一会话在不同入口呈现不同布局。
- 应用重启时，前端可能在 tab Controller 尚未就绪、项目树缓存尚未加载前先收到 `ListTabs`；若恢复元数据没有同样的精确 turn 摘要，当前零消息 tab 的首帧仍会因非空 `sessionPath` 退化为未知/已有内容布局。
- 新建空白会话水合期间，Transcript 先隐藏 Welcome、只居中 Composer；空历史返回后才挂载品牌、标题和场景入口，Composer 随组合高度变化向下移动。

顶部“当前模型不可用”提示由独立权限请求驱动；已有会话的专家团标识也由独立的 tab 级专家状态请求恢复。这两项已确认留作后续独立优化。

## 2. 基线与证据

- Codegraph 主工作区索引提交：`345a695b28fae2b6f09916c745ffcd52e1885f24`。
- 当前实际 `git merge-base HEAD origin/main-v3`：`26ecc3c8b392e6a02598994ae9198eabdac2b574`。
- Codegraph 只作主工作区调用路径导航；独立 worktree 未建立索引，所有物质结论均以 worktree 源码、测试和实际 diff 复核。

| 源码证据 | 当前行为 |
| --- | --- |
| `App.tsx` 的 `sessionHasContent` 与 `chat-pane--creation-empty` | 布局只看已经到达的 `items/live`，不能表达“已有会话正在加载” |
| `useController.beginSurfaceTransition` | 冷目标先以 `items=[]`、`hydrating=true`、`backendActivationPending=true` 呈现 |
| `loadSessionDataForTab`、`resumeSession`、`openChannelSession` | 先分发 `hydrate_start`，异步历史成功后才写入 `history` |
| `Transcript.tsx` | `hydrating` 只抑制 Welcome，不阻止 App 使用 creation 居中几何布局 |
| `theme-overrides.css` | `chat-pane--creation-empty` 通过 auto margin 把 main/footer 居中 |
| `hydratePlaceholderItems` | 部分非单表面路径会短暂显示上一会话内容 |
| `surfaceLoadingHintVisible` | 单表面路径已有 120ms 静默阈值，但提示位于 StatusBar，且未统一覆盖全部入口 |
| `ListProjectTree` / `runtimeSessionNodes` | 普通 `topic/global_topic` 需要携带确定 `sessionPath`；内部目标索引已有精确 `turns`，但返回对象没有把“目标为零 turn”与“无目标”区分给前端 |
| `ActivateTopic` / `findTopicSessionForTargetForActor` | 点击无路径普通行后才按 actor 目录、scope/root、更新时间与 path tie-break 解析实际会话 |
| `ProjectTree` topic click / `App.handleOpenTopic` | 为保留双击重命名，第一次 click 后 200ms 才调用 `handleOpenTopic`；此前不会启动 surface transition |
| `surfaceIntentForSessionPath` | 把所有非空路径判成 `existing-session`，混淆“持久化运行时身份”和“是否已有用户 turn” |
| `Transcript` / `ensureBlankSurface` | 明确 blank 的水合阶段仍用 `hydrating` 抑制 Welcome，先渲染 Composer、后挂载完整 creation 内容 |
| `topicSessionIndexForDir` / `topicSessionSelection` / `ProjectNode` | 索引已读取 BranchMeta 的 `scene/scene_state`，但 selection、缓存和树节点只传播 path/turns，场景摘要在后端丢失 |
| `ProjectTree` / `SurfaceTarget` / `App` | 点击意图只携带 turns；App 先以未知场景强制 Office，同时隐藏标题/场景入口，待 `ActivateTopic` 返回后才恢复真实场景 |

### 根因

根因有五层：前端原本把“真正的新建空白会话”和“已有会话尚未返回历史”压缩成同一个 `!sessionHasContent` 条件；补齐普通行路径后又把“存在运行时身份”错误等同于“已有用户 turn”；topic 第一次 click 为双击重命名延迟 200ms 才调用 `handleOpenTopic`；blank 水合期间抑制 Welcome；最后，持久化场景在“索引读取 → selection/树节点 → 点击目标”三拍传播中被丢弃，App 只能先猜 Office、隐藏场景内容，再等待激活返回。CSS 按输入语义正常工作；必须同步建立可取消目标意图，并让后端精确目标摘要决定首帧布局和场景。

### 已确认的关联风险：过渡期草稿归属

该风险真实可达：

1. creation 布局使用 single-surface 模式。
2. `beginSurfaceTransition` 立即替换可见 state，但 `activeTabId` 要等 `ActivateTopic` 返回后才更新。
3. Composer 的 `sessionKey` 此时仍由旧 `activeTab/activeTabId` 生成。
4. textarea 的 `disabled` 不包含水合或 `backendActivationPending`，因此用户可以输入。
5. 目标激活后 draft key 改变，Composer 会把当前文本保存到旧 key，再恢复目标 key；用户刚输入的内容可能从当前输入框消失并归入旧会话。

既然本次已确认“加载期间允许编辑”，实现必须同时建立过渡目标的草稿身份，不能只禁用发送。

## 3. 目标与边界

### 目标

- 已有会话冷加载从第一帧使用普通会话布局，Composer 始终位于底部。
- 不显示 Welcome，不显示上一会话内容。
- textarea 可编辑；历史和目标运行时就绪前禁止发送。
- 加载超过约 120ms 时在历史区显示“正在加载会话…”；快速加载不闪提示。
- 输入内容从 transition 开始就归属于目标会话，激活完成后不丢失。
- 会话中心、项目树、命令面板、tab/链接和恢复路径使用同一语义。
- 新建空白会话、目标缓存、滚动恢复、错误提示和草稿隔离保持兼容。
- 同一个零消息会话从“新建对话”、全局任务、工作空间项目会话及具体会话子项进入时使用相同 creation 布局。
- blank 水合首帧直接挂载完整 Welcome + Composer 组合，不因空历史返回而重排。
- 从已有内容或非 blank 表面显式“新建对话”时，首个可见空白帧固定为完整 Office/draft；对当前同 scope/root、已绑定且精确零轮的可复用 blank 重复点击则保留当前场景。
- 零轮目标绑定前保持完整模式行、标题和场景入口，所有模式及二级场景不可交互；绑定后在原节点上启用允许的控件。
- 缺失、损坏或 legacy 场景元数据只在展示层回退为完整 Office/draft，不静默迁移磁盘内容。

### 非目标

- 不调整模型不可用 Banner。
- 不改变历史格式、持久化 schema 或 Wails 方法签名；仅允许给 `ProjectNode` 增加可选目标摘要（`sessionTurns/scene/sceneState`），并保留既有 `TabMeta.sessionTurns`，不得劣化项目树或 `ListTabs` 重复读取性能。
- 不新增历史失败重试入口或错误页。
- 不重新设计 Welcome、Composer 外观、历史列表或模型权限规则。
- 不处理已有会话从默认 `Craft` 到专家/专家团标识的异步切换；该问题需要独立的 tab 级专家身份恢复方案。

## 4. 期望行为与验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 冷打开已有会话 | 第一次 click 事件内即不使用 `chat-pane--creation-empty`；0–199ms 不得出现可见居中帧，Composer 位于底部 |
| AC-02 | 冷加载可见内容 | Welcome 和上一会话消息均不可见 |
| AC-03 | 冷加载输入 | textarea 可编辑；发送按钮和 Enter 均不能发送 |
| AC-04 | 过渡期输入 | 在 `ActivateTopic` 未返回时输入，激活完成后文本仍在目标会话 Composer 中；切回来源会话时来源草稿未被污染 |
| AC-05 | 快速加载 | 历史在 120ms 内完成时，不挂载可见加载提示 |
| AC-06 | 慢加载 | 超过约 120ms 后显示加载指示器和“正在加载会话…”，并提供 polite status 语义 |
| AC-07 | 加载完成 | 提示被历史内容原子替换，Composer 不发生中部到底部的位移 |
| AC-08 | 目标缓存 | 立即显示目标自身快照并恢复滚动锚点，不显示通用加载提示 |
| AC-09 | 新建空白会话 | 从已有内容/非 blank 表面进入时，首帧挂载完整 Office/draft Welcome；当前已绑定、同 scope/root、精确零轮且可复用的 blank 重复点击“新建对话”时保留当前 Coding/Design/Office，不先闪 Office。未能严格证明复用时仍预览 Office，后端若返回新 blank 则最终以返回 meta 的 Office 为准；pending 全控件禁用且节点不重排 |
| AC-10 | 历史失败 | 保留现有错误 Banner；不显示 Welcome；Composer 保持底部 |
| AC-11 | 快速连续导航/重命名 | 不同目标只有最后目标可更新意图、草稿 key、计时器和历史；同一可重命名行第二击取消待激活意图并恢复来源 |
| AC-12 | 入口一致性 | 会话中心、项目树、命令面板、tab/链接和恢复路径均满足上述行为 |
| AC-13 | 范围控制 | 仅校准项目树普通 topic 的目标身份/摘要及同版本缓存；历史/schema、Wails 方法签名、模型 Banner、专家身份恢复和 `scripts/code_guard_baseline.json` 不变 |
| AC-14 | 零消息会话入口幂等 | 同一零消息 `sessionPath` 从“新建对话”、全局任务、工作空间项目会话及具体会话子项进入时均使用 blank creation 布局；应用重启后，Controller 未就绪时的首次 `ListTabs` 也必须携带显式 `sessionTurns=0`，不得退化成已有内容布局 |
| AC-15 | 零轮持久化场景首帧 | 从任务栏或工作空间重开零轮会话时，首个可见帧显示 BranchMeta 保存的 Office/Coding/Design 与 draft/locked；不得先出现无选中占位或猜 Office。缺失、损坏、legacy 元数据展示完整 Office/draft 回退且不写盘；目标绑定前所有模式与二级场景不可交互，绑定后原节点启用 |

## 5. 选定方案

### 5.1 显式会话表面意图

在前端会话状态中增加有类型的目标意图，概念上为：

```ts
type SessionSurfaceIntent = "blank" | "existing-session" | "unknown";
```

- `sessionPath` 只证明目标运行时身份，不单独证明已有内容；明确目标 turn 数大于零时产生 `existing-session`，等于零时产生 `blank`，缺少可靠摘要时使用 `unknown`。
- `newSession`、`ensureBlankTab`、`ensureBlankSurface` 明确产生 `blank`。
- 兼容路径无法可靠分类时使用 `unknown`，不按 `hydrateReason` 猜测。
- 新导航原子替换旧意图；`reset`、`hydrate_reset`、成功和错误流程按当前目标保存或替换意图。
- 该意图只服务布局、加载反馈和发送门禁，不替代 `items`、`hydrateHistoryLoaded`、序列号或运行时绑定状态。

### 5.2 确定性目标与统一传播入口

- `ListProjectTree` 为实际存在会话的普通 `topic/global_topic` 预解析点击时会打开的具体路径和该目标的精确 turn 数；真正无会话的 topic 保持空路径且不伪造摘要。
- 选择必须与 `findTopicSessionForTargetForActor` 一致：使用当前 actor 会话目录，匹配 scope/规范化 workspace root，按 `UpdatedAt` 降序且同时间 path 升序；包含较新零 turn 会话，不从 topic ID 或聚合 turns 猜测。
- `ProjectNode` 增加可选 `sessionTurns/scene/sceneState`；全局 topic、项目 topic 及具体 session 子项传播各自将打开目标的精确摘要。零 turn 必须可序列化；现代 draft/locked 场景保真，缺失、损坏或 legacy 只生成 Office/draft 展示回退；现有聚合 `turns` 语义不变。
- 场景复用 topic session index 已读取的 BranchMeta 与同版本缓存，不增加目录扫描；匹配的运行时 tab 场景优先于缓存摘要，确保内存中的最新显式变更覆盖旧 sidecar 值。
- `TabMeta` 同样增加可选 `sessionTurns`。启动恢复复用 Controller 构建原本就要执行的 session preload，在 tab 对 `ListTabs` 可见前写入精确摘要；正常和损坏文件均不得为摘要额外重读。
- 运行期只从已加载 `Session` 或 Controller 内存历史刷新 tab 摘要：接受用户 turn 时先乐观增加，turn 完成、新建/清空及成功的 rewind/summarize 后以实际历史校正。`ListTabs` 只复制内存标量，禁止扫描 session 文件或依赖项目树缓存预热。
- 目标映射与现有 session list 使用同一个版本化缓存和 5 秒首次加载上限；重复刷新直接复用，`projectSessionCache.invalidate()` 后共同重建。
- `beginSurfaceTransition` 从 `SurfaceTarget.sessionPath/sessionTurns/scene/sceneState` 初始化意图和首帧场景；激活返回的运行时精确场景覆盖 prepared 摘要。
- 项目树在 path 行第一次 click 中同步启动 transition，并把 transition id 交给 200ms 后的共享打开请求复用；实际后端激活时序不变。
- 同一可重命名行第二击取消匹配 id 并恢复来源后进入重命名；快速点击不同目标先取消旧 id、再同步建立最新意图，旧计时器不得激活。
- `loadSessionDataForTab` 在 `hydrate_start` 中携带明确意图。
- `activateTopic`、`openTopicSession`、`openProjectTab`、`openGlobalTab`、`switchTab`、`resumeSession`、`openChannelSession` 和启动恢复路径传递已知目标。
- 已有会话冷加载不再把来源 tab 的 `hydratePlaceholderItems` 作为可见 transcript；目标自己的缓存快照保留。
- 沿用 `surfaceActivationSeq`、`sessionLoadSeq` 和 `stillCurrent` 丢弃过期响应。

### 5.3 稳定布局与加载提示

- creation 空白布局只有在“无内容且目标不是已有会话”时成立。
- 明确 blank 的水合阶段不抑制 Welcome；Welcome、场景入口与 Composer 在同一首帧挂载，禁止用动画遮掩重排。
- 显式新建 pending 默认显示完整 Office/draft；仅顶部“新建对话”明确允许、且当前已绑定 active tab、blank surface、精确零轮、同 scope/root、无运行任务/过渡时复用当前场景预览。请求保存该预览，后端 meta 返回后仍为最终事实源；专家和项目树入口不改变。
- pending 期间模式与全部二级场景使用原生 disabled 保持可见、不可点击且不可聚焦；绑定后复用同一 DOM 节点原位启用。现代 locked 绑定后仍按领域规则可见并禁用。
- 为“已有会话且历史仍在水合”建立按目标身份 keyed 的 120ms 延迟可见性；结束或切换目标时取消计时器。
- Transcript 接收明确的加载展示输入，在空历史区渲染紧凑提示。
- 同一水合状态只保留一个 `role="status"`，避免与 StatusBar 重复播报。
- reduced-motion 下加载图标不持续旋转。

### 5.4 过渡目标草稿身份

- 从 surface transition 第一帧开始，Composer 必须使用目标身份生成 draft key，而不是继续使用来源 `activeTabId`。
- 扩展现有 `composerDraftKeyForTab` 的身份输入，或增加同源纯函数，使 transition target 与激活后的 `TabMeta` 对同一会话生成完全相同的 key。
- `sessionPath` 继续优先于 topic/tab 身份；不得直接混用两种格式不同的 key，避免目标激活时发生第二次无意义切换。
- Controller 向 App 暴露足以生成当前“渲染表面”草稿 key 的目标身份；非 transition 时保持现有行为。
- 该变化只影响 Composer 内存草稿，不写入持久化 SessionViewSnapshot。

### 5.5 编辑与发送

- 已有会话历史或目标运行时未就绪时，把该状态加入 `submitDisabled`/`ready`。
- 不加入 textarea 的 `disabled`，保证用户可编辑目标草稿。
- 保留 Controller 的发送守卫作为第二道防线。

## 6. 方案取舍

| 未采用方案 | 原因 |
| --- | --- |
| 所有 `hydrating` 状态均使用底部布局 | 新建空白会话会先落底、完成后再居中，产生反向抖动 |
| 按 `hydrateReason` 推断目标类型 | `open-topic` 和 `startup` 同时覆盖空白与已有会话，语义不足 |
| 在 `onResumeSession` 维护局部 UI 标志 | 不能覆盖项目树、tab、启动恢复等入口，快速导航也容易遗留旧状态 |

“不显示上一会话”“模型 Banner 后续处理”是已确认边界，不再作为替代架构重复展开。

## 7. 影响范围与兼容性

预计实现文件：

- 后端目标、tab 恢复与缓存：`desktop/app.go`、`desktop/tabs.go`、`desktop/{tabs_topic_test.go,history_controller_decoupling_test.go}` 及邻近测试。
- 前端状态与身份：`features/session/{sessionState.ts,sessionReducer.ts,useController.ts}`、`lib/composerDraftKey.ts`。
- 前端入口与展示：`App.tsx`、`features/workspace/ui/ProjectTree.tsx`、`components/Transcript.tsx`、session/theme 样式与 `locales/zh.ts`。
- 对应后端、Controller、reducer、draft、ProjectTree、App/Transcript 和 creation 布局测试。

兼容性：

- 无网络、持久化格式或数据迁移；Wails 方法签名不变，`ProjectNode` 仅增加向后兼容的可选 `sessionTurns/scene/sceneState`，`TabMeta` 现有字段不变。
- 普通已有会话行开始填充既有 `sessionPath` 字段；真正空白 topic 仍为空，缺少可靠身份的数据保持 `unknown`。
- 多会话选择与实际打开保持一致，包括较新零 turn 会话和同时间 path tie-break；缓存失效沿用项目树现有版本信号。
- 表面意图优先从当前目标重建，不写入 SessionViewSnapshot；若实现证明必须持久化，则需更新 snapshot 版本并重新审批兼容方案。
- 单语前端只更新 `locales/zh.ts`。
- `App.tsx` 和 locale 属于共享前端表面，最终 Gate 预计运行完整前端验证。

## 8. 实现风险与控制

| 风险 | 真实触发条件 | 控制与验证 |
| --- | --- | --- |
| 草稿写入来源会话 | `ActivateTopic` pending 时用户输入 | 从首帧使用目标 draft key；延迟 activation 的交互测试验证目标保留、来源不污染 |
| 陈旧意图/计时器泄漏或重命名误激活 | 200ms click 延迟内换目标或同一行第二击 | transition id 匹配取消；不同目标只保留最新计时器；双击重命名测试 |
| 空白会话被误判为已有会话 | 用 `hydrating`、reason 或非空 path 代替内容摘要 | 路径与精确目标 turn 摘要分别传播；覆盖 `newSession` 与 `ensureBlank*` |
| 零消息持久化会话被误判为已有内容 | 全局/项目/子会话节点的 `sessionPath` 非空但目标 `turns=0` | 各节点精确 `sessionTurns=0` 传播 blank；覆盖创建后重开和应用重启 |
| 新建页内容分两帧挂载 | blank 历史尚未返回时统一抑制 Welcome | 明确 blank 水合仍渲染完整 creation 组合；deferred history 锁定前后节点与布局 |
| 零轮目标先猜 Office 或无选中 | 场景摘要只到后端索引，未进入点击目标 | 同缓存传播现代场景；运行时优先；Office/Coding/Design 与 fallback 首帧测试 |
| pending 场景仍可被点击 | 只禁用一级模式或仅隐藏内容 | 模式与全部二级场景原生 disabled；验证不可点击、不可聚焦及绑定后同节点启用 |
| 重复新建先闪 Office 或错误继承旧场景 | 所有 blank pending 一律 Office，或仅凭空历史继承 | 仅顶部新建的严格复用候选保留当前场景；existing、stale identity、不同 scope、未知 turns、专家/项目树/过渡均 Office；返回 meta 最终校正 |
| 树节点路径与实际打开漂移 | 只从排除零 turn 的 `ListSessions` 摘要选路径 | 与 actor-aware resolver 共用匹配与排序语义；覆盖空白、单/多会话、零 turn 和 tie-break |
| 侧栏重复刷新退化为目录 I/O | 每次按 topic 或目录重新解析目标 | 目标与列表同版本缓存，首次加载受 5 秒上限约束；测试验证复用与失效 |
| 启动 `ListTabs` 为摘要重复读盘或丢失零值 | Controller 未就绪且项目树缓存未预热 | 复用一次启动 preload 并把精确指针存入 tab；测试锁定 ready=false 首次快照、JSON 显式零、legacy 非零、损坏缺失和单次读取 |
| 重复状态播报 | 历史区和 StatusBar 同时渲染 status | 同一已有会话水合只保留一个 live status；可访问性测试 |

## 9. 回滚

- 实现作为一个可独立回滚的功能 slice，通过保持顺序的实现与后续修复提交序列交付；状态契约、目标传播、UI、测试和任务回执共同回滚。
- 回滚恢复旧 creation 判定、旧占位策略、旧 Composer key 来源以及项目树不传播精确 turn/scene 摘要的行为。
- 无数据迁移，因此不需要回滚数据。

## 10. 验证计划

### 自动化验证

使用 deferred Promise 和 fake timers 提供确定性条件：

- `tab-switch-hydration.test.tsx`
  - 冷已有目标首帧意图、来源 transcript 隐藏、缓存保留、快速导航和错误态。
  - fake timer 验证 119ms 不显示、120ms 后显示、完成/切换后取消。
- `composer-session-draft.test.tsx`
  - 保持 `ActivateTopic` pending，输入文本后完成激活；目标草稿保留，来源草稿不变。
  - transition target 与激活后 TabMeta 生成相同 key。
- `new-session-load-race.test.tsx`
  - reducer 意图生命周期、blank/new-session 和零消息持久化目标回归。
- App/Transcript/creation 测试
  - 布局条件、发送门禁、加载节点、status 语义和 reduced-motion。
  - existing→新建完整 Office；当前 blank Design/Coding 重复新建无 Office 预览；stale/different scope/真实新 tab 不继承；pending 全控件禁用及绑定后原节点启用。
  - 入口契约验证会话中心/命令面板复用 `onResumeSession`；`project-tree-navigation-intent.test.tsx` 用 fake timer 验证 click 同步意图、199/200ms、重命名取消及最新目标。
- Go 项目树测试
  - 普通项目/全局行、具体 session 子项、真正空白 topic、较新零 turn 会话和同时间 path tie-break 与实际 actor resolver 一致，并返回精确 `sessionTurns/scene/sceneState`。
  - Office/Coding/Design、draft/locked、legacy/缺失/损坏回退及运行时父子节点优先级均有覆盖。
  - 首次加载、重复刷新缓存复用和 `projectSessionCache.invalidate()` 后重建。
  - 持久化 tab 启动恢复在 Controller 未就绪时返回精确零/非零；损坏文件保持摘要缺失；同一次恢复只执行一次 session preload。

迭代期运行最邻近测试；应用行为稳定后运行：

```powershell
cd desktop/frontend
pnpm test:all
pnpm build
```

直接修改 `desktop` Go 包时运行该包 `go vet` 与 `go test`；最终按 Gate 分类扩展。

### 应用验收

阈值语义不依赖真实磁盘恰好慢于 120ms：

1. 优先使用 Codex 内置 in-app browser 控制浏览器开发 fixture/测试壳，并用可控 deferred `HistoryForTab`：
   - 119ms 前无提示；
   - 跨过 120ms 后出现提示；
   - resolve 后消息替换提示且 Composer 不移动；
   - activation pending 时输入，完成后草稿仍在目标会话。
2. 浏览器无法覆盖 Wails/原生桌面交互时，使用 Computer Use 控制实际 Desktop（`cd desktop && wails dev`）验证：
   - 从会话中心、项目树、命令面板打开冷已有会话；
   - Welcome 和旧消息不出现，Composer 始终在底部；
   - textarea 可编辑、发送受限、历史完成后草稿保留；
   - 新建空白首帧即显示完整 Office/draft Welcome + Composer，绑定前控件禁用且历史完成前后不移动；
   - 从全局任务、工作空间项目会话和具体会话子项重开零消息会话时，首帧保留其真实场景；legacy/缺失/损坏元数据展示完整 Office/draft 回退；
   - 快速切换、缓存恢复和历史错误行为符合 AC。
3. 若实际磁盘读取在 120ms 内完成，Desktop 验收只要求“不闪提示”；精确阈值由 fake timer 与可控 fixture 证明，不以人为制造慢磁盘作为 Gate。

实现者自测与最终应用验收分离；只有内置浏览器和 Computer Use 都不能可靠覆盖，或场景需要用户凭据/人工判断时，才请用户测试，并交付明确的数据、步骤、期望结果和观察点。

### PR 前 Gate

仅在获得 PR 授权、同步最新 `origin/main-v3`、完成应用验收后执行：

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code origin/main-v3...HEAD -- scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

任何 required change-related check 失败时不得创建或更新 PR。
