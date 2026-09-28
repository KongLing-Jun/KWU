# Desktop 终态恢复修复计划

## 1. 问题与证据

30 次真实 Desktop 稳定性测试中的 DEF-009（A06）出现了“最终回答已经完整可见，
Composer 仍显示 Stop/loading”的终态分裂：

- 调查基线为 `6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad`；A06 的 final 于
  `2026-08-12T20:18:05.2328555Z` 落盘。
- Desktop 侧 telemetry 在 `20:18:05.2737146Z` 已记录 `TurnDone`，即落盘后约
  40.9ms；控制器源码保证 `RuntimeStatus.Running=false` 先于 `TurnDone` 发出。
- `20:18:13.0436522Z` 的截图已显示完整 final，但 Composer 仍为 Stop/loading。
- 首次取证到相同 final 且 Composer idle 是 `20:18:39.726Z`。这是区间观测的上界，
  不能把 34.493s 的时间相关性直接解释为 30s watchdog 已触发。

权威原始证据保留在
`D:\Reasonix-worktrees\desktop-luna-stability-20260812-6cfe5af6\.codex-run\desktop-stability-20260812-6cfe5af6\`；
本修复不改动这些证据。

## 2. 基线、根因与现有缺口

- 实施基线：`origin/main-v3@32f006922a5ae570b6d9fdcb2eb96c46282bf127`。
- 主工作区 Codegraph 索引位于 `70d919f4c50ca81f18952bc90c3817e9d9d78af8`，与实施
  基线不同，只用于导航。目标符号、调用方和影响范围均已在当前 worktree 直接复核；
  相关生产文件在 A06 基线与实施基线之间没有变化。
- `eventProjection.ts` 的顶层 `message` 会完成 final 投影并关闭 live stream，但有意保留
  `running/turnActive`；只有 `turn_done` 或权威 `backend_status` 才能安全进入 idle。
- `tabs.go` 的 `ListTabs` 从控制器 `RuntimeStatus` 投影 `running/cancellable`；控制器在发出
  `TurnDone` 前已清除 `running`，因此它是丢失终态事件后的可用权威状态源。
- A06 已证明 backend 生命周期和 Desktop sink 在约 41ms 内完成；同时，若前端消费了
  该 `turn_done`，现有 reducer 会同步清除 Stop/loading。由此可证明终态事件没有有效到达
  前端状态投影；具体丢失发生在 runtime emitter、Wails bridge 还是浏览器事件回调之间，
  现有日志无法再细分。
- 现有 30s stale-turn watchdog 是一次性 timer。长回合中第一次到期后，无论发现了新活动
  还是查询到 backend 仍在运行，都不会重新布防；A06 回合约 213s，因此当前 watchdog
  不能解释或可靠修复 final 后的终态恢复。
- 现有测试只覆盖 reducer 和 `shouldReconcileStaleTurn` 谓词，没有覆盖 hook 的 timer、
  丢失/延迟 `turn_done`、查询失败或旧查询结果与新回合竞态。

根因分类：直接故障属于 Desktop 终态事件交付链，精确 Wails 子层未知；用户可见的长尾由
前端缺少及时且可重入的权威状态对账放大。修复不依赖猜出不可观测的 bridge 子层。

## 3. 目标与非目标

### 目标

- 顶层 final `message` 到达后给正常 `turn_done` 100ms 宽限；若未到达，查询权威 backend
  状态并在 backend 已 idle 时清除 Stop/loading。
- 在第一次查询仍为 running 时，300ms 后复核，随后以 1s 为上限继续退避复核；backend
  仍运行或查询失败时绝不提前解锁。
- 在本地 bridge 正常响应且 backend 已 idle 的常规丢事件场景中，用户可见恢复目标为
  100–300ms，确定性回归测试上限为 500ms，不引入固定 2s 等待。
- 修复通用 30s watchdog 的一次性缺陷，使长回合活动更新、backend 仍运行或查询失败后
  都能继续布防，同时保持最多每 30s 一次的低频兜底。
- 防止旧回合的异步 `ListTabs` 结果清除已经开始的新回合。
- 增加不含消息内容、错误详情、提示词或凭据的诊断 breadcrumb。

### 非目标

- 不修改 Go controller 生命周期、`TurnDone` 生成顺序、Wails 事件协议或 emitter。
- 不用 final `message` 本身推断 backend idle，不改变 reducer 的权威状态语义。
- 不改变 Composer 视觉、动画、文案、发送/取消协议、后台任务或审批行为。
- 不发起真实 Provider 请求，不读取或记录任何 API Key。
- 不修改运行时配置、依赖、稳定性证据或 `scripts/code_guard_baseline.json`。

## 4. 验收场景

| ID | 场景 | 预期 |
| --- | --- | --- |
| AC-01 | 顶层 final 后 100ms 内收到正常 `turn_done` | 立即按既有路径 idle，并取消待执行查询 |
| AC-02 | 顶层 final 后 `turn_done` 丢失，backend 已 idle | 100ms 后权威查询清除 Stop/loading；测试中不晚于 500ms |
| AC-03 | 第一次查询时 backend 仍 running，第二次变 idle | 第一次不清除；300ms 退避查询后清除 |
| AC-04 | backend 持续 running | 始终保留 Stop/loading，后续查询退避到 1s 并持续复核 |
| AC-05 | `ListTabs` 失败或找不到目标 tab | 不猜测 idle、不清除状态，按相同退避继续复核 |
| AC-06 | 子代理/子树 `message` | 不启动父回合终态对账 |
| AC-07 | 旧 final 的查询在新回合开始后才返回 idle | 旧结果被回合身份保护丢弃，新回合保持 running |
| AC-08 | 长回合中原 watchdog 到期后又有活动 | timer 根据最新活动重排；再次 stale 后仍查询并可恢复 |
| AC-09 | 组件卸载、切换终态或新回合 | 清理/失效相应 timer 和在途结果，无悬空状态写入 |
| AC-10 | 诊断信息 | 只记录阶段、状态、重试序号和 tab 标识，不记录用户/模型内容或错误文本 |

## 5. 选定方案与影响范围

在 `useController` 中增加每 tab 唯一的 terminal reconciliation 链：

1. 只在顶层 `message` 投影完成后捕获当前 `sessionGen + turnStartAt` 身份并安排 100ms timer。
2. `turn_done`、新的 `turn_started`、同 tab 的新 terminal 链或卸载会取消 timer，并使已经
   发出的旧查询失效。
3. timer 通过既有 `ListTabs`/runtime-status 投影读取 backend 状态；应用结果前再次核对
   tab、running/turnActive 和回合身份。
4. backend idle 时只派发既有 `backend_status`，不增加第二套清状态逻辑；backend running、
   查询失败或 tab 暂时缺失时使用 300ms、1s、1s……的单链重试。
5. 通用 stale watchdog 改为可自重排的单 timer：以最新活动时间计算下一次到期；执行一次
   对账后若同一回合仍在运行，至少 30s 后再查，避免紧循环。

预计修改：

- `desktop/frontend/src/features/session/useController.ts`：terminal timer、回合身份保护、
  可重入 stale watchdog、清理和 breadcrumb。
- `desktop/frontend/src/__tests__/use-controller-terminal-reconcile.test.tsx`：新增无 Provider、
  fake clock 的 hook 级确定性回归。
- 本主题任务文档：实施后记录真实变更、验证、偏差和剩余风险。

不修改 `eventProjection.ts` 或 `sessionReducer.ts`：final 与 idle 可分离是正确模型，缺陷在
丢失终态事件后的恢复时效和 timer 生命周期。

## 6. 风险、兼容性与回滚

| 风险 | 控制 |
| --- | --- |
| 中间顶层 message 后 backend 仍合法工作 | 以 `RuntimeStatus` 为唯一清除依据；running 时只重试 |
| 频繁 message 造成查询风暴 | 每 tab 单链，新的 message 替换旧 timer，退避上限 1s |
| 旧 idle 快照清除新回合 | 查询前后校验 `sessionGen + turnStartAt` 和当前 running 状态 |
| Wails/ListTabs 暂时失败 | 失败等价于“未知”，不清除并退避重试 |
| 长回合 watchdog 紧循环 | stale 查询后设置 30s 最小下次查询间隔 |
| breadcrumb 泄露内容 | 仅写枚举状态、重试序号和现有 tab 标识，不写内容或异常文本 |
| 背景任务被误认为前台运行 | 继续使用 `foregroundRunningFromRuntimeMeta` 现有契约 |

无 API、数据、迁移或依赖变化。出现回归时可整体回退前端实现与对应测试；现有
`turn_done` 主路径不变，无数据回滚。

## 7. 验证计划

实施者先运行新增聚焦测试及相邻既有测试：

```powershell
pnpm exec tsx src/__tests__/use-controller-terminal-reconcile.test.tsx
pnpm exec tsx src/__tests__/use-controller-meta.test.ts
pnpm exec tsx src/__tests__/use-controller-cancel-reconcile.test.tsx
pnpm build
```

主会话独立审查 timer 数量、异步身份保护、失败语义、清理和实际 Git diff，并复跑聚焦
测试与生产构建。因为不允许真实 Provider 请求，应用语义以 hook + Wails bindings mock、
受控 `WireEvent` 和 fake clock 验收；不把人工等待或截图采样当作精确时序证据。

本改动只涉及前端和文档，不运行 Go 测试。最终检查 EOL、`git diff --check`、
`scripts/code_guard_baseline.json` 未变及项目 common checks。未获得单独授权前不推送、
不创建或更新 PR。

## 8. 阶段门

用户已确认修复方向、100–300ms 体感目标、500ms 测试上限、backend-running/查询失败时
不提前解锁、持续退避、watchdog 重入和脱敏埋点。计划与任务契约提交后，再取得一次实现
审批；之后由单一实现子代理串行实施，主会话独立审查和验收。若根因、外部行为、协议、
范围或风险发生实质变化，停止实现并重新确认。
