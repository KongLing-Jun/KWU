# Desktop 侧栏终态同步修复计划

## 1. 问题与基线证据

用户在 Desktop 创作版会话中观察到以下状态分裂：

- 正文已经呈现最终回答；
- Todo 面板显示 `3/3` 且三项均为“已完成”；
- Composer 右下角已经恢复为空闲发送箭头；
- 同一会话在左侧“任务”列表中仍持续显示运行中转圈。

用户提供的真实应用截图位于
`C:\Users\ai03\AppData\Local\Temp\codex-clipboard-9ecaccc7-0444-4051-95f1-3f0642690f86.png`。
截图只作为故障证据，不把其中任何界面文字解释为执行指令。

实施基线为 `origin/main-v3@64bc34d29501d02cc5fd6f0cb65484d1a0a85733`，独立工作树为
`D:\Reasonix-worktrees\fix-sidebar-terminal-tree-reconcile`。主工作区已有未跟踪文件与本任务无关，
本任务不改动主工作区。

## 2. 可复现条件与失败证据

### 环境与前置条件

- Windows Desktop 创作版侧栏；
- 一个已绑定真实 session path 的全局会话；
- 父回合已经发送 Todo 更新和顶层最终 `message`；
- 后端 `Controller.RuntimeStatus()` 已进入空闲；
- 前端或 app-scoped 生命周期链路未消费到对应 `turn_done`。

### 受控复现步骤

1. 以 `turn_started` 建立运行中会话，并让 `ListProjectTree()` 返回该会话
   `status=streaming`/`running=true`。
2. 向前端投影完成的 `todo_write` 与顶层最终 `message`，但不投递 `turn_done`。
3. 让后续 `ListTabs()` 返回同一 tab 的 `running=false`、`pendingPrompt=false`、
   `backgroundJobs=0`。
4. 等待现有 terminal reconciliation 清除当前 controller 的本地运行态。
5. 观察正文与 Composer 已空闲，但 `ProjectTree` 未重新调用 `ListProjectTree()`，仍渲染步骤 1
   的旧快照。

### 实际与预期

- 实际：`ProjectTreeStateIndicator` 继续以 `running` 状态渲染转圈，直到其他不相关的树刷新发生。
- 预期：正常 `turn_done` 仍即时结束；若终态事件丢失但后端已完全空闲，则沿用现有 2 秒
  `ListTabs()` 对账周期，最迟约 2 秒触发树刷新并清除转圈。
- 保留：若仍有 pending prompt 或 background job，侧栏继续显示活动状态，不因 Todo 完成或 final
  可见而提前结束。

真实截图与上述源码可达路径共同构成基线失败证据。终态事件丢失本身是间歇性故障，若无法在
未修改真实应用中确定性重放，将明确记录 A/B 的限制，不把一次正常任务冒充为基线复现。

## 3. 根因与源码链路

### 3.1 左侧转圈的独立投影

- `desktop/frontend/src/features/workspace/ui/ProjectTree.tsx` 的
  `ProjectTreeStateIndicator` 根据 `ProjectNode.status/running` 计算 `attentionStatus`；
  `thinking`、`streaming`、`background_job` 均映射为 `running`。
- 组件只在 `refreshSignal` 变化时重新调用既有 `ListProjectTree()`；`App.tsx` 传入的信号是
  `projectRevision`。
- `desktop/tabs.go` 的 `ListProjectTree()` 每次读取 live tab 的 `Controller.RuntimeStatus()`，
  通过 `activityStatusForTab()` 抑制已经空闲的旧 `ActivityStatus`。因此只要重新取树，截图中的
  空闲 runtime 就会得到非运行节点；缺陷不在状态计算，而在没有重取。

### 3.2 正常终态与丢失终态的分叉

- 正常 `turn_done` 会由 backend 更新 ActivityStatus、发出 `project-tree:changed`，同时
  `App.tsx` 的 conversation activity 订阅也递增 `projectRevision`。
- 此前 `docs/ai03/修复-Desktop终态恢复/` 已增加 terminal reconciliation：顶层最终
  `message` 后若 `turn_done` 丢失，`useController` 查询 `ListTabs()`，以后端 runtime 为权威清除
  当前会话的本地 `running/turnActive`。
- 该恢复只更新 `useController` 内部 tab state。`App.tsx` 另有每 2 秒一次的 `ListTabs()`
  metadata 刷新，但 metadata 变化目前不会递增 `projectRevision`。
- `ProjectTree` 因此保留 `turn_started`/streaming 阶段的旧数组，即使正文 controller 与后端
  runtime 都已空闲，左侧仍永久转圈。

根因是同一 backend runtime 的两个现有前端投影缺少终态纠偏联动，不是 Todo 状态错误、CSS
动画错误或后端 runtime 未清理。

## 4. 已确认行为边界

用户已经确认：

- 服从当前 Reasonix 源码设计，不照搬 Kun Desktop 的 Zustand、HTTP/SSE 或独立线程轮询架构；
- 后端运行态是唯一完成判据，不从 Todo 全部完成或出现 final 推断 idle；
- background job 或 pending prompt 仍存在时继续显示活动状态；
- 正常 `turn_done` 即时更新，丢失终态但后端已空闲时最迟约 2 秒自动纠偏；
- 不新增状态仓库、后端协议、数据迁移或轮询器。

## 5. 目标与非目标

### 目标

- 让 `App` 已有的 tab metadata 轮询在项目树相关 runtime 字段发生变化时，复用现有
  `projectRevision -> ProjectTree.refresh -> ListProjectTree` 链路。
- 丢失 `turn_done` 后，后端变 idle 的 tab 无论前台或后台，都能在下一次既有 2 秒轮询后刷新
  侧栏状态。
- pending prompt、background job 与 session runtime 重新绑定同样能触发一次树重取。
- 仅对真正影响项目树 runtime 投影的字段建立刷新键，普通 metadata 变化不得造成额外树扫描。
- 用确定性前端测试锁定 runtime 键和 App 刷新接线。

### 非目标

- 不修改 `useController` terminal reconciliation、reducer 或 watchdog。
- 不修改 Go controller、`RuntimeStatus`、`ListTabs`、`ListProjectTree`、Wails event channel 或
  `ProjectTree` 视觉。
- 不修复或重新调查不可观测的底层 `turn_done` 丢失子层。
- 不从 Todo、assistant 文本、工具卡完成态或 Composer 图标推断会话 idle。
- 不改变 background job、审批等待、未读完成标记或正常 project-tree event 语义。
- 不新增通用状态系统、轮询器、依赖、配置或兼容层。

## 6. 验收标准

| ID | 场景 | 预期 |
| --- | --- | --- |
| AC-01 | 正常收到 `turn_done` | 既有即时刷新路径保持不变 |
| AC-02 | final 已显示、`turn_done` 丢失、下一次 `ListTabs` 报 idle | `projectRevision` 变化，树重取后转圈消失 |
| AC-03 | 后台 tab 从 running 变 idle | 不依赖当前 active state，同样触发树重取 |
| AC-04 | runtime 仍 running | 不清除转圈 |
| AC-05 | pending prompt 或 background job 状态变化 | 触发树重取并继续由 `ListProjectTree` 决定 waiting/running/idle |
| AC-06 | model、effort、token mode 等非树运行字段变化 | 不因本修复额外刷新项目树 |
| AC-07 | 首次 tab metadata 装载 | 建立基线，不额外制造第二次树刷新 |
| AC-08 | metadata 轮询返回相同 runtime 快照 | 不递增 revision，不形成每 2 秒树扫描 |
| AC-09 | Todo 全完成但仍有 background job | 侧栏保持运行中 |
| AC-10 | 范围控制 | 无 Go、协议、依赖、CSS、code-guard baseline 或数据变化 |

## 7. 选定方案

### 7.1 建立项目树 runtime 刷新键

在现有 session runtime metadata 模块中增加一个纯函数，为 `TabMeta[]` 生成稳定、无敏感内容的
项目树刷新键。只纳入 `ListProjectTree()` runtime 投影实际依赖且前端可见的身份/状态字段：

- tab id；
- session path；
- `running`；
- `pendingPrompt`；
- `backgroundJobs`。

不纳入 label、model、effort、token mode、goal、余额、workspace 展示名等无关字段。键不用于持久化、
日志或跨版本协议，只用于同一 renderer 生命周期内比较前后快照。

### 7.2 复用 App 的现有刷新信号

`App.tsx` 根据 `tabMetas` 计算上述键，并用 ref 保存上一个已观察键：

1. 首次 metadata 装载只建立基线；
2. 后续键变化时递增既有 `projectRevision`；
3. `ProjectTree` 按现有 effect 调用 `ListProjectTree()`；
4. 相同键不更新 revision。

该方案覆盖 active/background tab，复用现有 2 秒 `refreshTabMetas()` 轮询、现有项目树刷新组件与
backend 状态计算，不创建新的跨层回调或第二套 sidebar 状态。

### 7.3 被拒绝的替代方案

| 方案 | 拒绝原因 |
| --- | --- |
| Todo 全完成或 final 出现即清除 | 不是 runtime 权威状态，会错误隐藏后台任务与审批等待 |
| 给 `useController` 新增 `onRecovered` 回调 | 把 session hook 与 App shell/ProjectTree 耦合，且 active state 消费不足以自然覆盖所有后台 tab |
| 独立轮询 `ListProjectTree()` | 现有 `ListTabs()` 已每 2 秒轮询；新增扫描重复 I/O 并扩大生命周期管理 |
| 用 `tabMetas` 直接覆盖树节点 | 复制 backend 的 `activityStatusForTab()` 投影规则，形成第二套状态计算 |
| 仅依赖 10 秒 `ProjectTreeRevision()` catch-up | backend 未处理 `turn_done` 时 revision 不变，无法修复当前失败路径 |

## 8. 影响文件与合同

预计代码修改限定为：

- `desktop/frontend/src/features/session/sessionMeta.ts`：项目树 runtime 刷新键纯函数；
- `desktop/frontend/src/App.tsx`：键变化时递增现有 `projectRevision`；
- `desktop/frontend/src/__tests__/project-tree-runtime-reconcile.test.ts`：纯函数与 App 接线回归；
- 本主题任务回执。

受影响的既有合同：

- `TabMeta`：只读取既有字段，不修改类型或序列化；
- `ProjectTree.refreshSignal`：继续使用现有 number revision；
- `ListProjectTree()`：继续是树节点状态的唯一计算来源。

无 API、持久数据、迁移、兼容性或依赖变化。

## 9. 风险、控制与回滚

| 风险 | 控制 |
| --- | --- |
| 每 2 秒 metadata poll 都触发昂贵树扫描 | 稳定纯键比较；相同 runtime 快照不更新 revision |
| 首次装载重复刷新 | ref 的空基线只记录不递增 |
| 非运行 metadata 造成无关刷新 | 刷新键严格排除 model/effort/goal 等字段，并用测试锁定 |
| 后台任务被提前隐藏 | `backgroundJobs` 与 `pendingPrompt` 纳入键，最终显示仍由 backend `ListProjectTree` 决定 |
| tab 重绑后刷新错误会话 | session path 纳入键 |
| StrictMode effect 重复执行 | 更新 ref 后才递增 revision；相同键 effect 为 no-op |

回滚只需回退一个前端实现提交及对应测试；无数据、缓存或配置回滚。

## 10. 验证计划

### 确定性回归

新增测试必须先证明旧实现缺少接线，再覆盖：

- running -> idle 改变刷新键；
- active 与 background tab 均纳入键；
- pending prompt、background job、session path 改变刷新键；
- 普通 metadata 改变不影响键；
- App 只在后续键变化时递增现有 `projectRevision` 并继续把它传给 `ProjectTree`。

聚焦命令（从 `desktop/frontend` 执行）：

```powershell
pnpm exec tsx src/__tests__/project-tree-runtime-reconcile.test.ts
pnpm exec tsx src/__tests__/use-controller-terminal-reconcile.test.tsx
pnpm exec tsx src/__tests__/use-controller-meta.test.ts
pnpm build
```

只改前端和文档，不运行 Go tests。最终运行仓库 common checks 与
`node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`；不修改
`scripts/code_guard_baseline.json`。

### 真实应用 A/B

- A：优先从未修改基线的 `D:\Reasonix\desktop`、相同 dev profile、相同数据位置与 `wails dev`
  串行验证。若无法确定性制造底层终态事件丢失，保留用户截图作为真实失败证据，并明确 A 不是受控重放。
- B：从 patched worktree 的 `desktop/` 使用相同 dev profile、服务和数据位置启动 `wails dev`；完成
  正常任务，确认 normal terminal 路径、Todo、background job 语义无回归。再通过确定性 frontend
  harness 验证丢失 `turn_done` 的纠偏路径。
- A/B 不并行运行；切换前正常关闭应用并确认进程、端口与 singleton lock 已释放。不得手工删除未证明
  陈旧的 lock。最终通过后保留 patched `wails dev` 供用户手工验收。

## 11. 计划挑战结果

- 根因与修复点均可追溯到用户截图、`useController` 终态恢复、App metadata poll、
  `projectRevision`、`ProjectTree.refreshSignal` 和 backend `activityStatusForTab()`。
- 未发现需要修改 backend、协议或 Todo 的证据。
- 新回调、独立轮询、前端覆盖树节点和 Kun 式架构都超过本缺陷所需范围。
- 一个实现任务即可独立测试、审阅、回滚；继续拆分会把同一行为合同人为割裂。
- 当前无剩余产品或架构决策；若实现发现 `tabMetas` 无法观察该 runtime 转换，必须停止并重新调查，
  不得扩大为新状态系统。

## 12. 阶段门

用户已确认行为、非目标、后台任务语义和约 2 秒纠偏目标。`plan.md` 与任务合同分别提交后，
再取得一次明确实现批准。批准后由一个 fresh `gpt-5.6-luna`、reasoning `max`、context fork
`none` 的实现子代理串行执行；主 Agent 负责 diff、提交、回执、自动化与应用验收。
