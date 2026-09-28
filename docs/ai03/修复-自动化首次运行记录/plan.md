# 自动化首次运行记录即时显示修复计划

## 1. 背景与问题

新建自动化任务后首次点击“立即执行”，左侧任务栏会马上出现新的会话，但自动化卡片仍显示“从未执行”，最近运行区域也为空。用户再次点击“立即执行”后，第一次运行的记录才出现。

该问题在新任务上最明显；已有任务也会在当前运行期间继续只显示旧记录，直到同步触发调用结束或其他操作重新加载列表。

期望行为：

- 首次点击后，卡片立即显示不可点击的“启动中”状态。
- 后端创建真实运行记录后，自动替换为“检查条件”“运行中”或终态记录。
- 同一任务到达终态前不能再次触发。
- 不要求用户刷新页面、关闭重开自动化页或重复点击。

## 2. 基线与证据

- Codegraph 主工作区索引在调查时为最新状态，对应源码提交 `f1777a582545d3146ec184a1a89c1f0a9c754910`。
- 本计划 worktree 从最新 `origin/main-v3` 的 `36e2623305f1f42e4b27cdd26b5f1b8edc568a49` 创建。
- 两个提交间只包含会话切换计划和任务文档；自动化相关文件无差异。
- Codegraph 只用于主工作区调用链导航；以下物质结论均已在独立 worktree 用实际源码重新确认。

| 证据 | 当前行为 |
| --- | --- |
| `HeartbeatPanel.tsx:692-703` | `handleTrigger` 等待 `heartbeatTriggerNow` 完成后才调用 `loadTasks` |
| `HeartbeatPanel.tsx:516-529` | 运行记录仅通过 `loadRuns` 写入 `runsByTask`；任务重载不等待该调用 |
| `HeartbeatPanel.tsx:1268-1275` | 立即执行按钮只按任务启用状态禁用，不表达本地触发中状态 |
| `HeartbeatPanel.tsx:1329-1347` | 只有真实 `runs.length > 0` 时才渲染最近运行区域 |
| `heartbeat.bridge.ts:26-28` | `heartbeatTriggerNow` 丢弃后端的 `runId/status` 响应 |
| `internal/automation/service.go:291-318` | `TriggerNow` 同步等待 Engine 执行完成 |
| `internal/automation/engine.go:206-235` | manifest 在桌面 Runner 创建/运行会话之前已经持久化 |
| `desktop/automation_runner.go:25-81` | Runner 先绑定自动化 topic，再同步运行完整 AI turn |
| `internal/automation/lease.go:35-53` | 同一任务使用任务级租约；竞争触发会快速返回且不创建新运行 |
| `internal/automation/service_test.go:234-291` | 测试锁定“竞争 TriggerNow 快速返回，但最终只有一个真实运行” |
| `automation-panel-contract.test.ts:143-150` | 现有前端测试只覆盖错误提示和禁用任务，未覆盖首次即时显示与重复点击 |

Wails v2.12 的桌面前端在 Windows、Linux 和 macOS 上均把绑定消息交给独立 goroutine 调度，因此长时间 `AutomationTriggerNow` 未返回时，可以并发读取 `AutomationListRuns`。Automation Store 的 manifest 写入是原子写，列表读取可以观察已创建的 queued/checking/running 记录。

### 上游对照

- `esengine/DeepSeek-Reasonix#4813` 引入旧 Heartbeat 时采用“等待 TriggerNow 完成后重载任务”的流程；当时没有运行历史列表，只需要在结束后更新 `lastRunAt`。
- 本分支的 `fb55d93b feat(frontend): show automation run details` 新增运行历史 UI，但保留了旧同步触发流程，形成此次回归；`aef5be17` 前端架构重构原样迁移了该逻辑。
- 上游 `#6131` 只处理 `heartbeat-tasks.json` 外部编辑，不涉及运行记录刷新。
- 上游当前 Heartbeat 页面没有 `AutomationListRuns` 运行历史能力，没有可直接移植的已合并修复。

## 3. 根因

首次触发的真实时序为：

1. 前端调用并等待同步 `AutomationTriggerNow`。
2. Engine 创建 manifest，桌面 Runner 创建或绑定会话；侧栏因此能立即看到会话。
3. AI turn 仍在执行，前端尚未调用 `loadRuns`，卡片没有本地触发中状态。
4. 立即执行按钮仍可点击。
5. 第二次点击因任务级租约竞争而快速返回。
6. 第二个点击处理器立即执行 `loadTasks → loadRuns`，读到第一次运行的 manifest。

所以第二次点击不是创建了第一条记录，而是意外充当了刷新按钮。

另有一个不在本次修复范围内的后端合同问题：租约竞争时 `service.TriggerNow` 会保留预生成、但没有对应 manifest 的 run ID，并以无错误响应返回。当前前端又丢弃该响应，因而没有暴露这一差异。

## 4. 目标与非目标

### 目标

- 第一次点击在同一前端交互周期内显示“启动中”。
- 只轮询当前正在触发的任务，并自动显示真实运行状态。
- 每个任务独立防重；不同任务仍可分别触发。
- 真实记录出现后不伪造 run ID，并保留现有运行详情入口。
- 触发完成或失败后执行一次最终刷新并停止轮询。
- 页面关闭、任务删除和组件卸载时不遗留计时器或陈旧状态更新。
- 保持现有会话创建、归档、取消、调度、任务保存和错误提示行为。

### 非目标

- 不把后端 `TriggerNow` 改成异步任务接口。
- 不修改 Engine、Runner、租约、manifest 或 Serve API。
- 不在本次修复租约竞争返回虚假 run ID 的后端合同。
- 不重新设计自动化卡片、运行详情或历史保留数量。
- 不新增全局事件总线或持久化前端触发状态。

## 5. 选定方案

### 5.1 任务级前端触发状态

- 在 `HeartbeatPanel` 保存按 task ID 分区的触发状态，至少包含本次触发起点和请求代次。
- 点击后先原子写入本地状态，再启动 `heartbeatTriggerNow`。
- `TaskCard` 在触发状态存在、且尚未观察到本次真实运行时，显示不可点击的“启动中”行。
- 立即执行按钮在 `!task.enabled || triggering` 时禁用，并提供可访问名称/忙碌语义。
- 不向 `AutomationRunSummary` 注入假的 run ID；本地启动行是独立展示状态。

### 5.2 单任务运行记录刷新

- 提取只刷新一个 task ID 的 `loadRunsForTask`，使用函数式 state 更新合并结果，不覆盖其他任务记录。
- 触发期间以约 500ms 的有界间隔读取该任务最近三条运行记录。
- 第一轮读取无需等待 `TriggerNow` 返回；manifest 出现后，本地启动行由真实状态行替代。
- 每个任务使用请求代次或等价守卫，丢弃旧轮询、旧 trigger 或整页加载返回的陈旧结果。
- `TriggerNow` resolve/reject 后执行一次最终单任务刷新，再清理该任务的轮询与触发状态。
- 页面关闭或任务被删除时停止对应 UI 轮询；后端运行继续，重新打开页面时沿用现有全量加载恢复真实状态。

### 5.3 错误与兼容

- 触发调用失败且没有真实运行时，移除“启动中”并保留现有可见错误。
- 单次 `ListRuns` 失败不清空已有记录；触发仍未结束时保留本地“启动中”，下一轮继续尝试。
- 不再用 `loadTasks()` 作为触发后的刷新手段，避免任务列表 loading 闪烁和无关请求。
- 全量 `loadRuns` 与单任务刷新共享陈旧响应保护，防止较慢的初始加载覆盖较新的轮询结果。

## 6. 验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 新任务首次点击立即执行 | 同一交互周期内出现不可点击“启动中”，无需第二次点击 |
| AC-02 | 后端 manifest 出现 | 最迟一个轮询周期内由真实 checking/running/terminal 记录替代本地启动行 |
| AC-03 | 同一任务运行中再次点击 | 按钮禁用，只产生一次 `AutomationTriggerNow` 调用和一个真实运行 |
| AC-04 | 不同任务分别触发 | 各自显示独立状态，不相互禁用或覆盖记录 |
| AC-05 | 正常完成 | 自动显示最终成功/失败/阻塞/跳过/取消状态，停止轮询并恢复按钮 |
| AC-06 | 触发失败且无 manifest | 启动行消失，现有错误提示可见，按钮恢复 |
| AC-07 | 运行记录读取暂时失败 | 不清空旧记录；启动状态保留，后续轮询可恢复 |
| AC-08 | 关闭并重开自动化页 | 无遗留计时器或 React 警告；重开后从后端恢复真实记录 |
| AC-09 | 页面初始加载与触发轮询竞态 | 较旧响应不能覆盖较新的真实记录 |
| AC-10 | 保持行为 | 左侧会话创建、运行详情、取消、任务开关和定时调度行为不变 |

## 7. 影响范围与兼容性

预计产品文件：

- `desktop/frontend/src/features/automation/heartbeat/HeartbeatPanel.tsx`
- 如现有样式无法表达非交互启动行，则最小修改 `heartbeat.css`
- 如采用本地化文案，则更新当前应用使用的中文 locale

预计测试文件：

- 新增或扩展自动化面板的 deferred Promise/fake timer 行为测试
- 更新 `automation-panel-contract.test.ts`，不再锁定“等待完整 TriggerNow 后才刷新”的旧形状

兼容性：

- 无 Go、Wails、HTTP、存储、数据格式或迁移变化。
- `heartbeatTriggerNow` 对外签名保持 `Promise<void>`。
- 最近运行仍以服务端 manifest 为唯一可点击事实源。
- 轮询只在本地页面存在且任务触发未结束时运行，不成为后台常驻请求。

## 8. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 初始全量加载覆盖新轮询结果 | 按 task ID 维护请求代次，只接受当前代次结果 |
| 页面关闭后计时器或异步回调泄漏 | effect cleanup、open 状态和请求代次共同失效旧工作 |
| 轮询造成额外 I/O | 只轮询活跃任务，每任务约 500ms，终态立即停止 |
| manifest 创建极快或执行极快导致状态跳变 | 本地启动行与真实记录按触发起点匹配；最终刷新锁定终态 |
| `ListRuns` 暂时失败造成空白 | 保留旧记录和启动行，不把错误转换成空数组覆盖 |
| 多任务并发更新互相覆盖 | 使用函数式按 task ID 合并，不整体替换 map |
| Wails 并发行为与真实应用不一致 | 依赖源码已证明独立 goroutine；最终仍用真实 Wails 应用验收 |

若实现发现必须改变后端 TriggerNow 生命周期、响应合同或租约行为，必须停止实现、更新本计划并重新进入讨论和审批。

## 9. 回滚

- 前端状态、轮询、启动行和测试作为一个原子实现 slice 回滚。
- 回滚后恢复当前“TriggerNow 结束后整页重载”的行为。
- 无数据迁移、后端任务或持久化状态需要回滚。

## 10. 验证计划

### 自动化验证

使用可控 deferred `AutomationTriggerNow`、fake timers 和 `AutomationListRuns` 响应：

- 首次点击立即显示启动状态且按钮禁用。
- Trigger Promise 未完成时推进 500ms，真实 running 记录出现并替换启动行。
- 连续点击只调用一次 TriggerNow。
- 两个 task 可独立触发和更新。
- 终态、失败、临时 ListRuns 错误、关闭/重开和陈旧响应均满足 AC。
- 运行详情只接受真实 run ID。

迭代期运行新增测试及现有自动化面板邻近测试。应用行为稳定后运行：

```powershell
cd desktop/frontend
pnpm test:all
pnpm build
```

本方案不直接修改 Go；若实际 diff 扩展到 Go，按受影响包运行 `go vet` 和 `go test`。

### 应用验收

1. 优先使用 Codex 内置 in-app browser 控制浏览器开发 fixture，验证即时启动行、轮询替换、禁用与错误态。
2. 浏览器无法覆盖 Wails 绑定时，使用 Computer Use 控制 `wails dev` 实际桌面应用：
   - 新建一个“每次新建会话”的自动化任务；
   - 首次点击后观察启动行和左侧新会话；
   - 不进行第二次点击，确认真实运行记录自动出现；
   - 确认运行中按钮不可点击、终态后恢复；
   - 关闭并重开自动化页，确认真实记录一致。
3. 实现者自测与最终应用验收分离。

### PR 前 Gate

仅在获得 PR 授权、完成应用验收并同步最新 `origin/main-v3` 后运行仓库规定的完整 PR Gate。任何 required change-related check 失败时不得创建或更新 PR。
