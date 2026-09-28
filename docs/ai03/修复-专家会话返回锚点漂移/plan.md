# 专家会话返回可见跳动修复计划

## 背景与已证问题

问题最初来自 PR #344 合并后的专家会话返回：正确的语义问题 identity 和中部 offset 已保存，但返回时
`team-legacy` 与 `team-persisted` 先后恢复并覆盖视口。旧计划把完成条件改为 DOM、offset 与布局连续稳定，
解决了过早开放写回，却没有保证恢复过程本身不可见。

2026-08-10 在包含 DEV 可见跳动埋点的 clean HEAD 上再次执行原生 Desktop 单 wheel→稳定中部→打开模型
广场卸载 Transcript→返回同一专家会话，得到两个新的确定事实：

- App surface transition 暂不提供 `tabId` 时，sidecar 状态尚未定案，legacy 已成为可见 presentation 并先行
  恢复；随后 persisted 到达并替换它。双 presentation 是专家团专属问题。
- persisted 自身在一次返回中记录 179 次大于 1px 的可见位置变化，总绝对位移约 `164721.6px`，180 帧
  后以 `incomplete` 结束。目标问题标题按保存 offset 正常处于视口外并被虚拟器卸载；恢复代码把“DOM
  不在 overscan”误判为“尚未定位”，每帧重复 `scrollToIndex`，再被 offset correction 推回，形成反馈循环。

第二个问题位于普通聊天与专家团共享的 Transcript 恢复 effect；普通短会话不虚拟化，普通长会话在相同
几何条件下存在同类风险。当前原生 trace 只证明专家团实际发生，不能把普通聊天描述为已复现。

## 目标与非目标

### 目标

1. 新专家团会话只以 persisted sidecar 为权威 presentation；sidecar 未定案时不显示 legacy 猜测结果。
2. 专家团选择只有在 sidecar 原子保存成功后才成为已提交状态；失败必须可见、可重试且不留下内存/磁盘
   分裂状态。
3. 每个 semantic restore generation 最多执行一次 virtual seek；首次取得目标几何后，即使目标 DOM 被正常
   卸载，也使用虚拟几何继续纠偏和判稳。
4. 恢复中间态不对用户可见；最终 presentation 稳定后只揭示一次。真实用户输入仍可取消旧恢复。
5. 同一共享算法覆盖普通长聊天，且保留 true bottom、history remap、anchor 删除 fallback 与 PR #344 wheel
   clamp 行为。
6. clean 精确 HEAD 原生验收必须达到同一 identity；目标位置物理可达时 offset 误差 `≤8px`，不可达时必须
   停在 signed correction 指向的最近物理边界；可见跳动 `0`、virtual seek `≤1`、单次揭示且完成原因是 `stable`。

### 非目标

- 本阶段不做 Transcript keep-alive，不改变 App 在其他 surface 上卸载 Transcript 的生命周期。
- 不迁移或兼容缺少 sidecar 的历史专家团数据；当前代码新建的专家团会话必须拥有 persisted 数据。
- 不重构 Transcript 或替换 TanStack 虚拟器，不改变虚拟化阈值、overscan、Session anchor schema 或普通
  用户滚动策略。
- 不处理与语义返回无关的其他滚动问题，不 sync/archive PR #344 OpenSpec，不夹带清理。

## 根因与所有权边界

1. **presentation 所有权错误：** `tabId` 暂缺表示 resolution pending，不表示 persisted team 不存在；pending
   期间显示 legacy 会让同一 semantic view 出现两套可见 presentation。
2. **完成条件错误：** saved offset 可以合法地把问题标题放到视口上方。DOM 持续挂载不是正确完成条件；
   已测得的虚拟 row geometry 才能在 DOM 离开 overscan 后继续表示同一目标。
3. **保存提交错误：** 当前 `SetExpert` 先修改 controller 内存，再调用不返回错误的 `saveExpertMeta`；磁盘失败
   只发 warning，调用方仍可能认为选择成功。persisted-only 前必须收紧这条提交边界。

语义位置的 owner 仍是 `{viewKey, anchorMessageId, offset}`；presentation cache 只负责加速，不能成为另一套
语义来源。restore generation 拥有一次 seek、后续几何纠偏、完成或取消；旧 generation 不得操作新 view。

## 选定方案

### 1. Persisted-only 专家 presentation

- `activeExpert.mode === "team"` 且 session/tab identity 尚未可查询时为 `pending`，显示现有轻量 loading。
- sidecar 返回有效 team 后只挂载 `ExpertTeamSessionView`；legacy `AgentWorkspace` 不作为 pending fallback。
- 当前代码新建的专家团 sidecar 缺失或读取失败视为显式 error/retry，不静默回退 legacy。
- cache hit 可同步缩短 pending，但 cache 只是 persisted 查询的加速层。

### 2. 专家选择事务化

- 构造 next expert state，但不先发布到 controller/executor。
- 先用既有临时文件 + replace 机制写入 session expert sidecar，并把错误返回调用方。
- 写入成功后才提交内存选择、executor 与前端状态；写入失败保持原选择并返回可重试错误。
- 精确实现以红测验证的最小调用链为准，不改变 sidecar 格式。

### 3. 一次 seek + 虚拟几何收敛

- generation 首次无法取得目标几何时允许一次 `scrollToIndex(row, "start")`，并记录 seek 已发生。
- 目标一旦被 DOM 实测或虚拟器确认可解析，后续通过 row visual offset、saved anchor offset 和当前 layout
  revision 计算目标 `scrollTop`；DOM 不在 overscan 不得重新 seek。
- 稳定允许使用 DOM 或已取得的虚拟几何证明：identity/row 仍对应、正常可达时误差 `≤1px`、layout revision
  连续稳定。若保存 offset 因测量后总高度收缩而物理不可达，仅当同 identity 的 row 已测量、signed correction
  指向已抵达的 top/bottom 物理边界且 layout 连续稳定时，才把 closest clamped geometry 计入 30 个稳定帧；
  不改 canonical、不写 bottom、不增加 seek。继续保留有界上限；其他 `incomplete` 不得作为成功揭示。
- 用户 wheel、pointer/touch、滚动键、reveal、跨 view 或卸载取消旧 generation；旧 RAF 不得抢回。

### 4. 不可见预布局与一次揭示

- semantic middle return 在 persisted ready 后先让最终 workspace 参与真实尺寸布局，但设为不可见、不可点击、
  `aria-hidden`；不能使用会令尺寸归零的 `display:none`。
- 稳定后一次切换为 visible。若 pending 很短可避免 spinner 闪烁；是否增加短延迟仅在不削弱可访问性和测试
  确定性的前提下实施。
- 超限或 sidecar 错误保持明确状态并允许重试，不能揭示仍在反馈循环中的页面。

## 影响范围

预计生产代码限于：

- `desktop/frontend/src/components/Transcript.tsx`
- `desktop/frontend/src/components/TranscriptVirtualRows.tsx`
- 必要的相邻 frontend bridge/type/locale 文件
- `desktop/subagent_history.go`
- `internal/control/controller.go`
- `internal/control/selected_expert_refresh.go`
- `internal/agent/save.go`（仅在错误传播或精确事务边界确有需要时）

测试限于相邻 Transcript restore、expert workspace virtualization、DEV trace、Desktop expert selection/save tests。
OpenSpec change `fix-expert-session-return-anchor-drift`随本次边界更新；不触碰 PR #344 archive/sync。

## 验收与验证

### 自动化

- 旧红必须覆盖：tabId-less pending 暴露 legacy、persisted 稳定前可见、offscreen anchor 每帧重复 seek、
  保存失败后内存状态已改变。
- 新绿必须覆盖：pending/ready/error、cache hit/miss、seek 至多一次、DOM 卸载后的虚拟几何收敛、用户取消、
  aborted render/view ownership、true bottom、普通短/长 Transcript。
- 运行 restore、anchor、expert virtualization、diagnostics、相关 Go tests、前端 typecheck/build；应用稳定后再做
  比例化 Code Guard、Go test/vet 与 OpenSpec strict。

### 原生 Desktop

在 clean 精确 HEAD 使用同一类长专家团会话执行：bottom→一次原生向上 wheel→稳定中部→记录 identity/offset
→模型广场确认 Transcript 卸载→返回并等待稳定。缓存命中、未命中各至少一次，并核对：

- 没有可见 `team-legacy`；presentation 只揭示一次。
- `viewport.visible-jump` 为 0，virtual seek 不超过 1，完成原因 `stable`。
- identity 相同；可达 offset 误差 `≤8px`，物理不可达时为同 identity 的最近边界；无 bottom fallback、
  无第二 wheel、无崩溃。
- 另以普通长会话执行一次返回，证明共享恢复没有引入回归。

## 风险、回滚与后续

- persisted-only 会暴露此前被 warning 掩盖的保存失败；用事务红测、明确 error/retry 和原选择不变控制。
- 共享几何恢复影响普通长聊天；用普通虚拟化正反测试与原生验收控制，不改变 bottom/用户滚动路径。
- 不可见预布局可能影响焦点或可访问性；必须禁用交互、保持尺寸并验证揭示前后焦点不跳。
- keep-alive 是独立性能优化，只有本阶段稳定后仍有明显 loading 体感才另立计划。
- 生产实现可按 coherent commit 回滚；sidecar 与 Session anchor 格式不变，无数据迁移。
