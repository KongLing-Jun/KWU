# 多会话切换 Transcript 虚拟化修复计划

## 背景与问题

PR #309 已合并到 `main-v3`，正确完成 Composer 模型、计费来源、推理强度的
会话级隔离，并将折叠工具详情改为惰性挂载；它明确不实现完整 Transcript
虚拟化。其压力验收仅覆盖两个会话以约 800ms 间隔往返，renderer 工作集曾从
约 192.89MB 扩到 517.55MB，强制 GC 只能证明没有线性存活 JS 对象泄漏，不能
证明多会话 DOM 峰值安全。

后续失败实验 `codex/fix-composer-smooth-transition@2387ccbc` 保留最多 4 棵完整
Transcript DOM。真实 dev 历史按以下序列稳定复现：

`12-turn 工具密集长会话 → 短会话 → v4 → 专家模式 → fix-02 → fix-01 → 长会话`

返回已淘汰长会话时，WebView2 renderer 在 1 秒内退出，宿主仍运行，页面显示
`STATUS_BREAKPOINT`。两份 Crashpad 转储均为 `0x80000003`，落在同一
`msedge.dll + 0x4d0d02f`，现场包含 V8 MarkCompact 与内存回收路径。

## 当前与期望行为

- 当前：切换期用轻量遮罩隐藏 Composer；缓存目标历史要等后端激活后才显示。
- 当前：Transcript 以 30 个用户轮次作为完整热区。复现会话仅 12 轮，因此约
  130 个结构项一次性全部构建；固定轮次忽略工具/Markdown 复杂度。
- 期望：Composer 外形始终固定，目标会话草稿首帧直接恢复，交互在后端激活前
  继续禁用。
- 期望：缓存历史数据立即可用，但浏览器只挂载当前视口与少量 overscan 节点。
- 期望：冷历史只在正文区域呈现轻量加载状态，不显示 Composer 骨架。
- 期望：6 个以上会话快速随机切换 100 次不崩溃，renderer 内存不持续爬升。
- 当前补充：点击已有会话后，模型选择器会先显示源会话模型或空值，再显示目标
  模型；专家/专家团在切换期被显式置空。冷切换的协作方式也会先采用默认
  `normal`，随后才采用目标 runtime 返回值。
- 期望补充：模型/计费来源、专家/专家团、协作方式、工具审批、Token 模式和
  Goal 作为同一个会话级 Composer 身份，在目标会话的第一个切换帧原子呈现。
- 复验发现：目标 transition identity 虽能在早期帧显示目标专家，但 transition
  结束会切回独立的异步 `activeExpert`；肉眼可见专家出现、消失或换值两三次。
  之前的外部 60/80ms 抽样只验证单个时刻，不能证明连续 React 提交无闪烁。
- 模型补充：当前身份无权使用目标会话绑定模型时，切换期间仍会乐观显示原始
  `modelRef`，权限目录结算后才改为“请选择模型”。未登录、账号绑定变化、套餐
  降级/过期和套餐移除模型均属于同一 entitlement 不匹配问题。

## 源码与基线证据

- Git 基线：`origin/main-v3@f0f5a90f3aefcc0a8208b3f2bf97cd71d662943d`，包含
  PR #309 merge commit `1ab72a1140f18fd2754245c4d289d755ae136ebe`。
- `features/session/useController.ts` 已有容量 8 的会话视图数据快照和滚动锚点，
  但缓存命中后仍无条件 `HistoryForTab`；这解释了“历史立即出现但每次仍重读”。
- `Composer.tsx` 已按 `sessionKey` 保存完整未发送草稿，并用 layout effect 切换；
  当前不可见是 App 的过渡遮罩造成，不是草稿缺失。
- `Transcript.tsx` 的 `HOT_TURNS = 30` 让 12-turn 复现会话全部进入完整热区，
  `hotZoneNodes` 会同步创建所有消息、工具卡和 Markdown 节点。
- `package.json` 已依赖 `@tanstack/react-virtual`，Workspace 与 Diff 组件已有用例，
  不需新增依赖。

## 目标

1. 保留 PR #309 的后端 Composer 隔离、Fork 继承、门禁和工具详情惰性挂载。
2. 同一时刻只挂载一个 Transcript，不缓存完整历史 DOM。
3. 虚拟化非专家 Transcript 的热区渲染节点，只挂载视口与有限 overscan。
4. 缓存命中时立即绑定目标历史数据和滚动锚点，后台校验不切换视图所有者。
5. Composer 固定壳体并在目标身份切换时同步恢复草稿；加载期只禁用交互。
6. 以真实原生 WebView2 多会话压力验收作为完成条件。
7. 消除会话级 Composer 身份的跨会话残留、置空和默认值闪烁。

## 非目标与边界

- 不回退 PR #309；允许给内部历史/项目树视图增加可选 Composer 身份字段，但
  不改变现有会话文件格式和模型路由语义。
- 不修改模型 fallback、凭据、订阅、发送门禁或场景归属。
- 不把工具审批、协作方式、Token 模式或 Goal 新增长期持久化到历史 sidecar；
  特别禁止因视觉修复让关闭会话恢复旧的完全访问权限。
- 不增加磁盘草稿持久化；草稿仍只在当前应用生命周期内按会话隔离。
- 不缓存多个 Transcript DOM，不用调大 WebView 内存或关闭崩溃保护规避问题。
- 专家团队 AgentWorkspace 暂保留现有专用布局；普通 Transcript 先完成虚拟化。
- 不新增依赖，不修改 `scripts/code_guard_baseline.json`。

## 选定方案

### 1. 会话切换呈现

- `beginSurfaceTransition` 在命中快照时直接采用快照 state，不等待后端激活。
- App 用目标 `sessionViewKey` 驱动唯一 Transcript 和 Composer draft identity。
- Composer 过渡遮罩移除，壳体尺寸、输入区和工具栏始终存在；交互仍禁用。
- 后端激活完成后保持同一目标 items 集合；快照保存 `sessionTurns`，后端轮次未变
  时不再读取历史，轮次变化或版本未知时才后台 revalidation。

### 2. Transcript 节点虚拟化

- 继续复用当前节点分组、工具折叠、WarmZone 与消息组件，不重写消息语义。
- 将非专家 `hotZoneNodes` 组织为稳定 key 的虚拟行；使用动态测量和有限 overscan。
- 虚拟容器以总估算高度支撑原滚动容器，实际只挂载可视行。
- 问题导航先定位对应虚拟行，再做语义锚点微调。
- 会话切换由 `sessionViewKey` 重置虚拟器测量，滚动仍按既有消息锚点恢复。

### 3. 内存边界

- 删除/禁止完整 Transcript DOM LRU；数据快照 LRU 维持 8。
- 隐藏会话只保留结构数据、标题和滚动锚点，不保留 Markdown/ToolCard DOM。
- 不用固定 30 轮作为 DOM 上限；视口决定挂载量。

### 4. Composer 会话身份原子切换

- 历史列表和项目树为目标会话携带可选模型、计费来源和已有专家身份；模型数据
  复用现有 BranchMeta，专家数据复用现有 SessionInfo，不新增存储。
- 对仍存活的 runtime，项目树同时携带当前协作方式、审批方式、Token 模式和
  Goal；对关闭会话采用后端重开时已有的安全默认值。
- `beginSurfaceTransition` 将目标身份放入 transition target，冷缓存也能在第一帧
  构造目标 Composer profile；快照命中仍优先使用更完整的目标快照。
- App 只从目标 transition/view identity 派生模型、计费和专家展示，不再回退到
  源会话 `activeTab` 或把专家显式置空。
- transition 结束后目标 view identity 继续持有专家展示权；只有携带同一
  `viewKey`、同一目标 tab 且标记为 authoritative 的刷新结果才可一次性交接给
  runtime `activeExpert`。源 tab、旧 view generation 和 provisional 空结果均不能
  获得展示权。
- 模型展示由目标 `modelRef + billingSource` 与当前账号的已结算 entitlement
  快照原子投影：明确 allowed 才显示目标模型，明确 unavailable 从首帧显示
  “请选择模型”，unknown 只保留固定宽度控件且不显示错误文字，结算后淡入一次
  最终值。BYOK 继续使用既有独立配置语义。
- 前端测试记录点击后的连续呈现值，禁止出现源会话、空专家或非目标默认值。
- 在继续修复前先增加 dev-only 观测锚点：以点击为 `t=0`，记录 transition
  begin/finish、active tab、每次专家 refresh start/resolve/apply、每次 App commit
  的 displayed expert，以及下一次 paint；锚点不得参与状态选择或改变时序。

## 兼容性与迁移

- 内部 Wails `SessionMeta`/`ProjectNode` 增加可选只读呈现字段；旧前端忽略这些
  字段，旧会话缺字段时按现有安全默认值处理。
- BranchMeta、数据库和依赖不变，无数据迁移。
- 旧会话继续走 PR #309 的 sidecar 兼容与惰性回填。
- 虚拟化只改变 DOM 生命周期；消息内容、顺序、工具展开和导出数据不变。

## 风险与控制

| 风险 | 控制 |
| --- | --- |
| 动态 Markdown/图片高度导致跳动 | 动态 `measureElement`、overscan、既有多帧置底 |
| 问题导航目标尚未挂载 | 先 `scrollToIndex`，下一帧按消息锚点校正 |
| 切换后恢复错误滚动位置 | 虚拟器按会话 key 重置；复用既有语义滚动锚点测试 |
| 流式消息高度持续变化 | 可视行动态测量；既有 pinned-to-bottom 多帧跟随 |
| 包装虚拟行破坏间距/布局 | 增加专用行 CSS，并做原生截图和工具展开验收 |
| 专家布局未虚拟化 | 保持现状并列为剩余边界；压力验收至少包含专家会话切换 |
| 同轮次的外部原地改写 | 常规历史为追加/分支语义；版本未知仍校验，删除、恢复和身份变化继续使缓存失效 |
| 目标身份元数据缺失 | 关闭会话只恢复当前后端本来会采用的安全默认值；激活结果仍最终校正 |
| 工具审批被意外持久化 | 不扩展 BranchMeta 权限字段；仅活 runtime/内存快照携带当前值 |
| 专家已删除或目录未加载 | 目标专家 ID 无法解析时显示无专家，后台刷新不回退到源专家 |
| 套餐或账号变化后目标模型无权使用 | 复用 entitlement 判定；目标首帧直接显示“请选择模型”，不修改会话绑定 |
| 权限目录尚未初始化 | 保持固定模型控件但隐藏文字；只在权威结果后呈现一次，不乐观显示潜在无权模型 |

回滚：撤销本修复提交即可恢复 PR #309 行为；无数据清理。

## 验收标准

1. 长会话、短会话及至少 4 个其他会话随机快速切换 100 次，无白屏、
   `STATUS_BREAKPOINT` 或 renderer 退出。
2. 压力序列必须包含超过数据快照/会话边界的回访，以及长会话重复重建路径。
3. 同一时间 DOM 中只有一个 Transcript；非专家长会话仅挂载虚拟可视行。
4. 长会话输入未发送草稿，切到其他会话不串稿；切回首帧直接显示草稿，不经过
   placeholder 或 Composer 骨架。
5. 缓存历史首帧显示可视区域；`sessionTurns` 未变不重复读盘，变化或未知时后台
   校验；冷缓存只在正文区域显示加载状态。
6. 问题导航、跳到底部、滚动锚点、加载更早历史、工具展开/关闭和流式置底正常。
7. renderer 在 100 次切换后保持存活且无 Crashpad；强制 GC 后活 JS 堆保持有界。
   Windows Graphics Capture 会让工作集保留截图/已提交堆，因此工作集单值只记录，
   不作为活对象泄漏的唯一判据。
8. 聚焦测试、Playwright、TypeScript、CSS、生产构建和仓库检查通过。
9. 在不同模型、不同专家团、不同协作方式的已有会话之间切换，模型与专家必须
   在同一目标帧更新；无专家目标不残留源专家，关闭会话不恢复旧完全访问权限。
10. 专家验收必须导出从点击到稳定的完整 commit/paint 时间线，而不是固定间隔
    截图；连续记录中不得出现源专家、空专家或不同目标专家的中间提交。
11. 当前身份无权使用目标 `subscription_pool` 模型时，模型选择器从第一个目标帧
    起始终显示“请选择模型”；有权使用时直接显示目标模型；权限 unknown 不得
    显示源模型或潜在无权模型。会话保存的模型和 BYOK 行为保持不变。

## 验证计划

- 组件：虚拟行数量、动态测量、会话 key 重置、问题导航与无多 DOM 缓存。
- 会话：缓存命中首帧、激活后同一 items、冷缓存加载、草稿同步恢复。
- 回归：现有 Transcript 滚动、Composer 草稿、工具卡、App chrome 测试。
- 浏览器：Wails bindings 不可用时只验证保护行为并记录，不代替原生验收。
- 原生：dev profile 真实历史，6+ 会话 100 次随机快速切换，逐轮检查页面、
  renderer PID/工作集；专家切换另导出前端锚点时间线并与录像/截图对照。

## 已解决决策

- 保留 PR #309，不回退其 Composer 会话隔离。
- 放弃失败 DOM LRU 分支，从最新 `main-v3` 新建纠正分支。
- 选择真实虚拟化，不再继续调整固定热区数量或 DOM LRU 大小。
- 用户已授权文档、任务、实现和实际验收连续完成；推送/PR 仍需单独授权。
- 用户确认把完整会话级 Composer 身份纳入同一修复，并要求按可独立审查意图
  小步提交，不 amend 或 squash 现有提交。
- 用户确认模型展示按当前身份 entitlement 而非登录布尔值判定；权限 unknown
  时固定控件布局、隐藏模型文字，最终值只呈现一次。
