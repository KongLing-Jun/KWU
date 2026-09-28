# Desktop 完成态永久加载与工具卡闪烁修复计划

## 1. 背景与问题

在 Desktop 的长工具型普通会话中，模型已经完成、`message` 与
`turn_done` 已到达、final 已唯一持久化、后端也不再运行时，当前会话正文仍可能永久显示
“正在加载会话…”。切换到其他会话再切回后，同一份历史立即正常显示 final。

该故障已经在权威基线、真实 `gpt/gpt-5.6-luna` 和前台窗口中连续三次复现。
本计划只处理这次实测的 Transcript 永久遮罩链路，不把其他能够显示相同文案的竞态混入同一修复。

同一次真实验收还捕获到工具卡在用户没有操作时短暂展开再收起。DEV 事件证明同一卡片在
nested 调用到达后由 `defaultOpen` 自动展开，并在完成时自动收起，全程没有 `tool.toggle`。
用户已确认工具详情只能由鼠标点击或明确键盘操作展开或收起；用户展开后，完成态必须保持展开。

## 2. 基线与证据

- 权威复现与根因调查基线：`61729d22a39c31cc79b665ff50913081df0e9d10`。
- PR 准备同步基线：`origin/main-v3@1e2ffd49fa041e4e3098d3722f45985922a11ed7`；远端新增路径与本修复无重叠。
- Codegraph 索引主工作区：`D:\Reasonix`；索引/代码基线：
  `70d919f4c50ca81f18952bc90c3817e9d9d78af8`，比权威基线少 19 个提交。
- Codegraph 仅用于调用链、消费者和影响面导航；所有物质结论均已用
  `D:\Reasonix-worktrees\prompt-baseline-luna-main-v3-20260812` 的直接源码、测试、Git 和运行时证据复核。
- 新真实复现证据目录：
  `C:\Users\ai03\AppData\Local\Temp\reasonix-completion-real-repro-20260812-182807`。
- 第三次运行的完成态快照：`hydrating=false`、`existingSessionLoading=false`、
  `itemCount=30`、`regularRowCount=20`、`virtualized=true`、
  `initialScrollAnchor=u0/-50`、`pendingScrollAnchor=null`、恢复尝试为 0；
  `message` 与 `turn_done` 相邻到达。
- 两个核验 JSONL 的最终 assistant 文本均只出现一次；切换恢复没有重新提交或重复 final。
- `pnpm exec tsx src/__tests__/expert-workspace-virtualization.test.tsx` 当前为 221/221 通过，
  但没有覆盖同一普通视图运行中从 bottom 跨入虚拟化、初始锚点 prop 随缓存移动的组合。

## 3. 根因

根因是普通 Transcript 的滚动恢复状态所有权不一致：

1. 空白 surface 绑定到真实 session identity 时，`semanticViewKey` 改变；bottom/undefined
   锚点会清空 `pendingScrollAnchor`，但 view-switch effect 不保证把新 key 标记为已展示。
2. 长工具任务使普通行达到 `TRANSCRIPT_VIRTUALIZE_MIN_ROWS=10`，触发普通渲染到虚拟列表的布局替换。
3. 布局替换产生一次非用户程序性滚动复位；普通路径没有 expert 路径已有的 pinned
   virtualization reset 保护，因而把 pinned 状态改为非 bottom，并捕获顶部 `u0` 锚点。
4. `onScrollAnchorChange` 把该实时位置写入 controller Map。后续流式 render 又把 Map 当前值作为
   `initialScrollAnchor` 传回同一 `semanticViewKey`，违反“初始恢复锚点是 identity 快照”的源码假设。
5. `ordinarySemanticRestoreHidden` 因“已虚拟化 + prop 为 semantic anchor + 当前 key 未 reveal”成立；
   但 view key 没再次变化，`pendingScrollAnchor` 仍为空。
6. 恢复 effect 在 `!pendingScrollAnchor` 时直接返回，没有尝试、失败态或 fail-open；正文已经存在，
   却被 loading overlay、`visibility:hidden` 和 `inert` 永久遮住。

### 工具卡闪烁的独立根因

`ToolCard` 将 `running && (hasNested || isShellTool)` 作为 `defaultOpen`。因此 nested 调用异步到达会
把未操作的卡片自动展开，完成态又使该表达式变为 false 并自动收起。它与 Transcript 永久隐藏
共现且会放大布局变化，但不是永久加载的直接因果。

此外，creation mode 中运行中的 shell 以独立 `ToolCard` 渲染，完成后会被
`isCreationGroupableTool` 重组进 `ToolGroup`。这会卸载旧卡并创建新卡；仅删除 `defaultOpen`
并保留组件局部 state，仍会丢失用户在运行中作出的展开选择。该边界已由 Computer Use 真实运行和
确定性 remount 红测共同确认。

切换会话会建立新的 view activation，并以稳定历史 ID 重新播种 pending anchor，因此恢复 effect
重新运行并展示同一份 JSONL。这也解释了为什么 final 不重复。

### 已确认但不纳入本次的独立风险

`loadSessionDataForTab` 分发 `hydrate_start` 后，blocking load 可能被后台
`bumpSessionLoadSeq` 作废，并在 `stillCurrent()` 处提前返回而未闭合 hydration。
该竞态需要独立修复；本次真实复现中 `hydrating=false`，所以它不是当前根因。

## 4. 目标、非目标与边界

### 目标

- 同一普通会话在运行中跨入虚拟化时，原本 pinned-to-bottom 的位置保持 pinned。
- 每个 `semanticViewKey` 的恢复锚点是不可变 activation snapshot；实时滚动缓存只供未来激活使用，
  不能在同一 view 内反向改变当前恢复任务。
- 只有真实存在的恢复任务可以隐藏正文；不存在 pending restore job 时必须 fail-open。
- `message` 和 `turn_done` 到达后 final 可见，不依赖切换会话恢复。
- 保持用户主动滚动、历史锚点恢复、专家/专家团 workspace、旧消息分页和 ID remap 行为。
- 所有工具卡默认折叠，运行状态、nested 到达和完成态均不改变展开状态。
- 用户通过点击或既有键盘快捷操作展开后，完成态保持展开。

### 非目标

- 不修复独立的 hydration/load-sequence 竞态。
- 不改变 JSONL、telemetry、Wails/Go API、模型 provider 配置、模型调用或会话持久化格式。
- 不调整 loading 文案、视觉样式、虚拟化阈值或整体 Transcript 设计。
- 不改变工具组的媒体预览策略、工具执行状态或产物呈现。
- 不通过关闭虚拟化、延迟 `turn_done` 或增加超时掩盖根因。
- 未经后续授权不提交、不 push、不创建或更新 PR。

## 5. 验收标准

| ID | 场景 | 期望结果 |
| --- | --- | --- |
| AC-01 | 空白 view 以 bottom anchor 激活后绑定真实 session key | 当前 key 进入已展示状态；没有无任务隐藏态 |
| AC-02 | 普通行从 9 增长到 10 及以上，布局程序性复位到顶部 | 原本 pinned 的会话继续 pinned，不写入伪 semantic anchor |
| AC-03 | 同一 view 的实时滚动缓存发生变化 | 当前 activation 的 restore anchor 不变；变化只供下一次激活使用 |
| AC-04 | 存在真实历史 semantic anchor | 正常隐藏、恢复并 reveal；失败时保留既有重试/显示当前内容出口 |
| AC-05 | `pendingScrollAnchor=null` | 正文不得因 semantic restore 被隐藏 |
| AC-06 | 长工具流发送 final 与 `turn_done` | loading overlay 在稳定帧后不存在，final 可见且只出现一次 |
| AC-07 | 切换离开再返回 | 同一历史仍正确恢复，不提交新请求、不重复 final |
| AC-08 | 用户主动离开底部 | 仍保存真实 semantic anchor，不被 pinned 保护误吞 |
| AC-09 | expert/persisted workspace 和历史 ID remap | 现有恢复、替换与失败行为无回归 |
| AC-10 | 范围控制 | hydration、Go、持久化 schema、模型 provider 配置和 code guard baseline 不变 |
| AC-11 | running shell 或运行中工具收到 nested 调用，用户未操作 | 卡片始终折叠，不挂载详情正文 |
| AC-12 | 用户手动展开运行中工具，随后 nested/status/output 更新、虚拟化 remount 或完成后重组进工具组 | 工具组继承该选择，卡片保持展开，直到用户主动收起 |

## 6. 选定方案

### 6.1 冻结 activation restore snapshot

- 在 Transcript 中显式区分“本次 view activation 的 restore anchor”和
  `onScrollAnchorChange` 写出的实时位置。
- restore snapshot 仅在 `semanticViewKey` 改变时初始化；同一 key 的普通流式 render 不重新采纳
  移动中的 `initialScrollAnchor`。
- bottom/undefined snapshot 明确将当前 semantic view 标记为已展示。
- 若实现需要调整 Controller，只允许改变前端内存态所有权，不改变外部合同或持久化格式。

### 6.2 保护普通虚拟化切换的 pinned 状态

- 将现有只对 expert virtualization 生效的 pinned reset 语义扩展到普通 Transcript，或建立等价的
  普通路径保护。
- 保护条件必须同时满足：同一 view、此前 pinned、非用户手势、普通列表刚发生非虚拟到虚拟转换。
- 真正的 wheel、pointer、keyboard 导航和显式语义恢复不能被忽略。

### 6.3 建立 fail-open 不变量

- “是否隐藏正文”由当前 activation 的显式恢复阶段驱动，不能只由可移动 prop 和旧 revealed key 推导。
- 没有有效 pending anchor 或恢复任务无法启动时，立即 reveal 当前内容；不得等待永远不会发生的 effect。
- 保留现有有任务但无法完成时的失败、重试与“显示当前内容”路径。

### 6.4 工具卡 disclosure 由用户独占

- 去除 running、shell、nested 和 completion 推导出的默认展开状态，初始始终折叠。
- 点击和既有 shell 键盘快捷操作是唯一能够改变展开状态的入口。
- 以 `semanticViewKey` 为边界在 Transcript 内保存用户对 tool ID 的选择，使同一会话中的虚拟化、
  lifecycle 投影和 completion-time grouping remount 不会丢失该选择；切换语义会话时清空。
- 新建或已存在的工具组吸收一个此前由用户展开的卡片时，在绘制前继承展开；普通未操作工具组仍折叠。
- 折叠态继续延迟挂载重型详情内容；重组后的展开卡首帧直接挂载详情，避免二次闪烁。

## 7. 影响范围与兼容性

预计代码影响集中在：

- `desktop/frontend/src/components/Transcript.tsx`
- `desktop/frontend/src/components/ToolCard.tsx`
- `desktop/frontend/src/components/ToolGroup.tsx`
- `desktop/frontend/src/lib/toolDisclosure.tsx`
- ToolCard、Transcript 与 expert workspace virtualization 邻近测试

兼容性：

- 前端内存行为调整，无数据迁移、网络/API 或磁盘格式变化。
- 短会话和未跨虚拟化阈值的会话行为不变。
- 用户主动滚动仍更新缓存；修复只拒绝布局替换造成的伪滚动位置。
- fail-open 最坏可能短暂显示位置跳动，但不能再永久隐藏已经完成的正文。

## 8. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 把真实用户滚动误判为程序性复位 | 使用现有 gesture refs，并严格限定 same-view、previously-pinned、virtualization-transition 条件 |
| semantic 历史恢复首帧提前展示 | 以显式 restore phase 控制隐藏；有有效 pending job 时仍维持既有遮罩 |
| expert workspace 回归 | 不改变其 replacement/persisted 语义；运行现有完整 221 项邻近测试及新增交叉测试 |
| moving prop 仍从其他入口污染恢复 | 红测直接在同一 key rerender 不同 prop，锁定 snapshot 不变量 |
| fail-open 掩盖真正恢复失败 | 仅无任务时 fail-open；有任务失败继续进入可诊断失败态 |
| running shell 不再自动展示“仍在运行”详情 | 状态仍在卡头可见；详情由用户显式展开，符合已确认的统一规则 |
| 完成态 grouping 或虚拟化 remount 丢失用户选择 | disclosure 由当前 semantic view 持有；直接卡到新/既有工具组两条路径均有回归测试 |
| 一个会话的工具选择泄漏到另一个会话 | Provider 以 `semanticViewKey` 为 key，语义会话切换即重建状态 |

## 9. 红测与验证计划

### 失败优先测试

1. 同一 Transcript 实例：blank key/bottom → session key/bottom → 普通行 9→10；模拟非用户
   scroll reset，随后把写出的 anchor 作为同 key prop 回传；断言当前代码会永久隐藏，修复后 final 可见。
2. 锁定 fail-open、9→10 阈值、真实用户离底与历史 semantic restore/remap，确保修复不吞用户滚动或重复提交。
3. running shell 与 nested 工具在未操作时始终折叠；点击后跨 status、nested 和 output 更新保持展开。
4. 用户展开独立 shell 后完成态重组为新 `ToolGroup`；组、卡和输出均保持展开。
5. 已存在的 shell 组吸收一个新完成且用户已展开的卡片；只继承该卡，组在绘制前展开。
6. Controller/Wails 可控事件跨过阈值后发送 `message` 与 `turn_done`；稳定帧后 loader 消失、final 唯一，
   且没有新增 submit/model 调用。测试壳不足时用同 props 的 React 集成测试证明。

### 自动化验证

运行新增 ToolCard/Transcript 测试、完整 expert workspace virtualization 邻近套件和 `pnpm build`；
本次没有 Go 变更则不运行 Go 测试，未经 PR 授权不运行 PR 发布流程。

### 应用验收

1. 以可控事件及一次真实长工具任务验证 final 当前页直接可见、JSONL 唯一、侧边栏非运行。
2. 验证用户主动上滚后追加内容保持位置，切换回来不重复 final 或提交。
3. 观察未操作工具跨 nested/完成始终折叠；手动展开后跨完成重组仍展开。

## 10. 回滚

- 修复应保持为一个可独立审阅和回滚的前端 slice：snapshot 所有权、普通 pinned 保护、fail-open 与红测共同回滚。
- 无数据迁移，回滚不需要处理 JSONL、缓存或配置。
- 若普通 pinned 保护出现兼容问题，可先回滚触发源修复而保留 fail-open 安全阀；不得回滚测试对“无任务不得永久隐藏”的不变量。
