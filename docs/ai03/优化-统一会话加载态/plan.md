# 统一会话加载态优化计划

## 1. 背景与现状

首次冷加载已有会话时，Transcript 当前显示三块模拟 prompt/response 的大型骨架；专家数据等待、专家/普通长会话的语义位置恢复则分别重复渲染相同的 spinner 与“正在加载会话…”文案。用户反馈这些块状区域过多，加载表面显得割裂。

进一步复核真实导航发现：当前选中已有会话 A 后进入模型广场等主工作区，再点击“新建会话”，`handleNewTab` 先把 `workspaceView` 切回 chat，随后才异步创建/复用空白会话 B。B 就绪前 Controller 的 `activeTabId` 仍为 A，因而出现 A 历史 → 模型广场 → A 历史 → B 新会话的中间帧。这里不能通过清空 A 解决：A 同时是模型、专家、技能/连接器、Composer 草稿与模式、右侧工作区、运行时操作和 Transcript 滚动缓存的会话 owner。

语义恢复遮罩本身不能删除：它负责在虚拟列表测量和程序化滚动收敛期间立即隐藏并禁用不稳定正文，避免重新暴露已修复的可见跳动。本任务只统一等待态的视觉和延迟所有权，不改变恢复算法。

## 2. 基线与源码证据

- Codegraph 主工作区索引/源码基线：`f98f9878b340948b9ed447d6fa948dfd0cbc5740`；仅用于导航，当前独立 worktree 的物质结论以直接源码和测试为准。
- 当前 worktree 已包含会话返回锚点修复及最终验收文档；最新 `origin/main-v3` 为 `538b94bb19d0e4ae4867efeac81f792fae57cf7f`。实现前必须处理当前分支与最新基线的同步/依赖关系，不能假定 Codegraph 基线等于最终实现基线。
- `Transcript.tsx` 对首次冷加载渲染 `.transcript__session-placeholder` 及三块 placeholder；专家 pending、legacy defer、persisted restore 和普通长会话 restore 又分别维护等待态 JSX。
- `session.css` 为三块 placeholder 定义卡片尺寸、断点宽度和 shimmer；等待态根节点同时承担普通流式布局、全屏恢复遮罩与失败面板样式。
- `useController.ts` 现有约 120ms 提示阈值只覆盖已有会话水合，不能作为所有 Transcript 等待状态的统一视觉延迟所有者。
- `App.tsx` 的 `handleNewTab` 当前先调用 `showChatView(true)`，再等待 `openBlankSession`；`openBlankSession` 又被项目 Composer、专家新任务和项目树创建复用，不能把用户可见导航行为直接塞进该底层 helper。
- `activeTabId` 是会话运行时 owner 而非纯 UI 高亮。模型广场直接按它设置模型，连接器/专家/设置按它选择目标，Composer 与右侧工作区按会话 identity 隔离状态；取消选中会破坏既有功能。
- `EnsureBlankSurface` 成功时会先保存其他可见 tab，再把 B 设为后端 active 并裁剪其他可见 tab；运行中的 A 可作为后台 runtime 保留。该调用没有 prepare/rollback 阶段，因此本任务不增加可取消的后端创建协议。
- 无 `topicId/sessionPath` 的伪 blank `SurfaceTarget` 会退化为共享的 `topic:` view identity 和 `tab\0` Composer key，不能用通用 `beginSurfaceTransition` 冒充尚未创建的 B。
- OpenCode 公开会话页采用稳定外壳、`messagesReady()` 后挂载真实 timeline 的方式，未在主会话页模拟多块消息骨架；Codex Desktop 未公开组件实现。本任务采用“稳定外壳 + 单一低干扰状态”的产品原则，不复制未公开实现。

## 3. 已确认决策

- 标题、专家成员区域和 Composer 等稳定外壳保持原位。
- 首次加载、专家数据等待和阅读位置恢复统一使用一套视觉、一句 `正在加载会话…`，不维护两套文案或多套等待组件。
- 新建空白会话不属于历史加载，不显示加载文案、空白遮罩或临时 Composer；已有会话加载视觉不得复用到该路径。
- 既有 Composer 固定壳体与草稿合同必须保持：目标身份确定后首帧直接显示目标草稿，不允许空 Composer→草稿闪烁；可复用 blank 的未发送 prompt 不得被静默清空。
- 保护表面立即生效；spinner 与文字在等待持续约 180ms 后才出现，快速完成不得闪提示。
- 删除三块大型消息骨架和 shimmer。
- 失败状态不并入等待态，继续使用现有“重试 / 显示当前内容”等有界操作面板并立即可见。

## 4. 目标与非目标

### 目标

- Transcript 内所有已有会话正常等待路径由一个共享展示组件渲染；Transcript 继续拥有恢复等待的连续延迟状态。
- 加载开始即保留必要的空白/遮罩几何；等待不足 180ms 时无 spinner、文字或 live-region 播报。
- 等待跨 `team-pending → team-persisted hidden` 等 presentation 切换时不重新计时。
- 超过阈值后只出现一个紧凑、居中的 spinner 与统一文案。
- 加载完成后真实内容一次性揭示，不新增位移、闪烁或可见滚动修正。
- A → 其他主工作区 → 新建不得重新显示 A 历史，也不得引入空白/加载表面；最终方案必须在真实 B identity 下首帧呈现正确 Welcome 与草稿。
- B 准备期间 A 的 active/runtime、Composer/能力/右侧工作区状态与语义滚动缓存不被清空或改写。

### 非目标

- 不改变 persisted-only、sidecar identity cache、legacy 兼容判定或专家数据协议。
- 不改变语义锚点、单次 virtual seek、30 帧稳定条件、closest physical boundary、缓存写回或虚拟器。
- 不重设计标题栏、成员栏、Composer、Welcome、错误文案或按钮。
- 不增加网络请求、依赖、持久化字段或 Wails/Go API。
- 不引入后端 blank prepare/commit/rollback，不让项目 Composer、专家新任务等非主工作区调用自动触发导航遮罩。

## 5. 选定实现方式

1. 在 `Transcript.tsx` 内建立一个等待活动布尔值，合并：已有会话冷水合、专家 sidecar pending、deferred expert return、persisted expert hidden restore、普通长会话 hidden restore；失败态从正常等待活动中排除。
2. 由同一个 hook/timer 持有约 180ms 延迟。活动连续跨 presentation 切换时保持计时；活动结束、view identity 变化或组件卸载时同步取消。
3. 提取一个共享 `SessionLoadingPresentation` 渲染已有会话正常等待态。它只接受是否 overlay 与延迟后是否可见等展示输入；不拥有锚点、presentation、active tab 或新会话导航决策。
4. 等待根表面从首帧挂载以维持遮罩/布局，但延迟前不设置 `role="status"`，不挂载可见 label；延迟后设置单一 polite status。
5. 首次加载删除 placeholder JSX；CSS 删除 placeholder 卡片、响应式宽度与 shimmer，只保留紧凑居中状态、overlay 和独立 failure modifier。
6. `App.tsx` 向 Transcript 传递“已有会话正在水合”的事实，而不是已经延迟后的可见性；旧提示计时只保留仍被非 Transcript 状态栏需要的语义，并统一常量或移除无消费者分支，避免双重 120ms/180ms 延迟。
7. 已否决并回退的方案：在 `handleNewTab` 外层显示全屏等待表面并把整个 layout 设为 inert。该方案虽消除 A 回闪，但造成新建白屏并违背 Composer 固定壳体合同，不再作为实现基础。
8. 任务 002 重新进入调查：先在恢复既有行为的生产基线上精确覆盖“模型广场→新建闪 A”以及“blank B 草稿→A→新建恢复 B 草稿”，再选择不伪造 identity、不显示加载且不扩大后台 blank helper 的最小修复。

交付按两个串行任务实施：任务 001 先建立共享等待视觉与 Transcript 延迟所有权；任务 002 再复用该组件实现 App 新会话原子导航。两者不复制状态机，导航行为保持独立提交和回滚边界。

## 6. 验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 冷加载在 180ms 内完成 | 稳定外壳保留；没有骨架、spinner、文字或 live-region 播报 |
| AC-02 | 冷加载超过约 180ms | 正文区域只有一个居中 spinner 和“正在加载会话…” |
| AC-03 | 专家 sidecar pending 转 persisted restore | 保护连续有效、延迟不重置，始终最多一个 status；无 legacy 可见内容 |
| AC-04 | 专家及普通长会话恢复 | 正文立即 hidden/inert，稳定前无可见内容；稳定后一次揭示且既有锚点合同不变 |
| AC-05 | 加载失败/恢复 hard limit | 失败面板立即可见，pointer/touch/Space 重试和“显示当前内容”保持可用 |
| AC-06 | reduced motion | 不持续旋转；不再存在 shimmer 动画 |
| AC-07 | 可访问性 | 正常等待最多一个 polite status；延迟前不播报；失败仍为 alert |
| AC-08 | 范围控制 | 无虚拟器、锚点缓存、Go/Wails、持久化、依赖和 locale 文案扩展 |
| AC-09 | 其他页面新建 | A → 模型广场/能力页 → 新建不显示 A 历史、空白遮罩或加载文案；真实 B 首帧直接呈现 Welcome 与正确草稿 |
| AC-10 | 选中态与恢复隔离 | B 准备期间 A 保持会话 owner，A 的模型/专家/草稿/右侧工作区和 canonical scroll 不被清空或写成 bottom；以后返回 A 仍恢复同一 identity/offset |
| AC-11 | 草稿与竞态 | 可复用 blank B 的草稿在 B→A→新建后首帧恢复；创建失败与重复请求不丢草稿，也不让陈旧 B 覆盖最终目标 |
| AC-12 | 复用入口隔离 | 项目 Composer、项目树创建和专家新任务仍使用既有 blank helper 语义，不被主工作区导航遮罩劫持 |

## 7. 验证计划

自动化先建立旧红，再修改生产代码：

- 更新 `existing-session-loading-presentation.test.tsx`：锁定首帧空保护面、179ms 无反馈、180ms 单一反馈、加载完成原子替换、无 placeholder/shimmer。
- 更新专家虚拟化回归：锁定 pending→persisted 不重置延迟、hidden/inert 首帧仍成立、稳定揭示与 failure controls 不回归。
- 更新 diagnostics：隐藏等待不记录 visible jump，等待展示不影响 restore generation/seek 数。
- 增加 App 导航回归：延迟 `EnsureBlankSurface`，锁定其他主工作区不重新挂载 A、无空白/加载表面、真实 B identity 首帧及失败/重复请求边界。
- 增加 Composer 草稿回归：blank B 输入草稿→切换 A→再次新建，必须在首帧恢复 B 草稿且不出现空→草稿闪烁。
- 增加 A canonical 回归：新建等待前后 A 无 scroll/bottom 写回，返回 A 仍使用相同语义 identity/offset；覆盖普通、专家及运行中 A 的状态所有权。
- 运行受影响的 Transcript/restore/anchor/diagnostics 聚焦测试、`pnpm typecheck` 与仓库规定的 `pnpm build`。

应用验收使用 clean 精确 HEAD 新构建 Desktop：

- 冷打开普通已有会话，分别观察快速与人为延迟加载；确认没有三块骨架和重复提示。
- 返回专家长会话与普通长会话；以 DEV trace 确认 `viewport.visible-jump=0`、`anchor.restore.incomplete=0`、virtual seek 不增加，并肉眼确认只有一个低干扰等待态。
- 触发或用确定性 fixture 覆盖失败面板的重试与显示当前内容。
- 从普通/专家 A 分别进入模型广场，点击一次新建；确认没有 A 历史回闪、白屏或加载文案，真实 B 首帧即为 Welcome 与正确草稿。随后从会话中心返回 A，复核 DEV trace 的 identity/offset、visible jump、seek 和 incomplete。

## 8. 风险、回滚与交付边界

| 风险 | 控制 |
| --- | --- |
| 延迟跨 presentation 切换被重置 | 延迟所有权提升到 Transcript 的合并活动状态，增加连续切换测试 |
| 为了简化 UI 意外取消保护遮罩 | 根节点首帧挂载，hidden/inert 与恢复状态仍由原逻辑决定 |
| 延迟前空白被误判为无障碍缺失 | 180ms 内保持静默；超过阈值才建立唯一 polite status；失败始终即时 alert |
| CSS 简化影响 failure panel 命中 | failure modifier 和交互隔离保留，pointer/touch/keyboard 回归 |
| 清空选中会话破坏跨页面能力目标 | 明确保留 A 的 `activeTabId`；等待层只控制可见表面和交互，不改 owner |
| 伪 blank identity 串草稿或滚动缓存 | 不创建无 topic/path 的 `SurfaceTarget`，等待层不挂载 Transcript/Composer target B |
| 为消除 A 回闪再次遮挡 Composer 或伪造 B | 先锁定固定壳体/草稿旧红；只在真实 identity 上呈现目标 Composer，不使用全屏等待事务 |
| 共享 `openBlankSession` 扩散到后台入口 | 导航事务只包裹用户可见新建入口，底层 helper 与项目/专家调用保持不变 |

回滚可整体撤回本任务的 Transcript 展示、CSS 和测试提交；不涉及数据迁移。未经单独授权不 push、不创建或更新 PR、不标记 Ready、不合并。
