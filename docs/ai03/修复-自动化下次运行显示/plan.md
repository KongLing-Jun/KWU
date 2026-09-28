# 自动化下次运行显示修复计划

## 1. 问题与期望行为

自动化列表中的周期任务已启用并持续按计划执行，但卡片同时显示“暂无下次运行”。该文案是前端在 `nextRunAt` 缺失或不可解析时使用的兜底；当前任务实际仍有后续调度，因此界面状态与调度事实矛盾。

期望行为：

- 后端能够计算后续运行时间时，桌面 Wails 列表响应和 HTTP `/automation/tasks` 响应都包含 RFC3339 `nextRunAt`。
- 前端继续只使用后端返回的 `nextRunAt` 展示“即将运行”“N 分钟后”或具体时间。
- 确实没有后续运行时间时不返回该字段，前端仍显示“暂无下次运行”。

## 2. 基线、复现与失败证据

### 2.1 源码基线

- 独立 worktree：`D:\Reasonix-worktrees\fix-automation-next-run-display`
- 分支：`codex/fix-automation-next-run-display`
- 基线：最新 `origin/main-v3`，提交 `341f21d3`
- 主工作区存在用户的未跟踪 `artifacts/` 内容且本地 `main-v3` 落后远端，因此不在主工作区实施。
- Codegraph 在独立 worktree 中没有索引；本计划的物质结论来自该 worktree 的实际源码、Git 历史、测试输出和实际运行数据。

### 2.2 实际应用复现记录

环境与前置条件：

- Windows 桌面开发版，界面版本标识 `vdev`。
- 开发环境 ID `a6defe83d02e4e84`，配置目录为 `C:\Users\ai03\AppData\Roaming\kwu-dev\environments\a6defe83d02e4e84`。
- 任务 `task-35e8a262b7b8714e`（“测试ccs自动化任务”）处于启用状态，作用域工作区为 `D:\cc-switch`。
- 调度为 `interval/30m`，`effectiveFrom` 与 `effectiveUntil` 均未设置。
- 持久化记录显示任务持续由 `schedule` 触发成功；调查时 `lastRunAt` 为 `2026-08-18T03:35:09.6171968Z`。

复现步骤：

1. 从 `D:\Reasonix\desktop` 以现有 dev 配置运行 `wails dev`。
2. 打开“自动化 > 定时任务”。
3. 找到“测试ccs自动化任务”，确认开关开启并查看最近运行记录。
4. 查看周期右侧的下次运行文案。

实际结果：截图中任务已有 `8/18 09:54`、`10:24`、`10:55` 的连续成功记录，仍显示“每30分钟 · 暂无下次运行”。

期望结果：该时点应显示后端计算的下一次运行时间对应文案，约为最近计划运行后的 30 分钟，而不是“暂无下次运行”。

基线自动化证据：

```powershell
go test ./internal/automation -run TestServiceListTasksIncludesBackendOwnedNextRun -count=1
```

该测试通过，证明 `ListTasks` 返回的 Go `TaskView.NextRunAt` 已计算；但现有测试没有把 `TaskView` 编码为 JSON，因此没有覆盖实际失败边界。

## 3. 调用链与根因

| 位置 | 已确认行为 |
| --- | --- |
| `internal/automation/service.go` 的 `TaskView` | 嵌入 `Task`，另声明 `NextRunAt *time.Time` |
| `taskViewWithNextRun` | 调用 `PreviewNextRuns`，有后续时间时写入 UTC `NextRunAt` |
| `internal/automation/model_v2.go` 的 `Task.MarshalJSON` | 为保留未知任务字段，自定义整个 `Task` 的 JSON 编码 |
| `desktop/automation.go` | Wails 的 `AutomationReloadTasks` 直接返回 `[]TaskView` |
| `internal/serve/automation.go` | HTTP `/automation/tasks` 直接把 `[]TaskView` 交给 JSON encoder |
| `heartbeat.bridge.ts` | 将响应的 `task.nextRunAt` 原样映射到卡片模型 |
| `HeartbeatPanel.tsx` | 字段缺失或不可解析时显示“暂无下次运行” |

Go 会把匿名嵌入字段的方法提升到外层类型。`TaskView` 没有自己的 `MarshalJSON`，因此嵌入的 `Task.MarshalJSON` 被提升，JSON encoder 把整个 `TaskView` 当作 `json.Marshaler`，只输出 `Task` 的字段；外层 `NextRunAt` 从未进入响应。

根因不是调度器失效、`PreviewNextRuns` 算法错误或前端重新计算失败，而是共享 DTO 的 JSON 序列化合同遗漏了外层计算字段。

## 4. 目标与非目标

### 目标

- 修复 `TaskView` 的共享输出合同，使 Wails 和 HTTP 响应都包含有效 `nextRunAt`。
- 保留 `Task.MarshalJSON` 的未知字段透传能力。
- `NextRunAt == nil` 时严格省略 `nextRunAt`，避免输出持久化未知字段中的陈旧同名值。
- 增加能在未修复基线上失败、修复后通过的 JSON 回归测试。
- 通过真实桌面应用 A/B 证明基线显示失败、修复后显示后端时间。

### 非目标

- 不修改调度计算、漏跑策略、有效期、任务开关或运行引擎。
- 不修改前端文案、相对时间格式或前端类型。
- 不改变任务持久化格式，不新增迁移，不把 `nextRunAt` 写回 `tasks.json`。
- 不重构 `Task`、`TaskView` 或自动化 API，不新增通用 JSON 合并抽象。
- 不为作为响应 DTO 的 `TaskView` 新增反序列化合同；现有写接口继续接收 `Task`。

## 5. 方案选择与挑战

### 5.1 选定方案

为 `TaskView` 显式实现 `MarshalJSON`：

1. 先调用 `json.Marshal(view.Task)`，沿用当前任务字段和未知字段兼容语义。
2. 将结果解析为 `map[string]json.RawMessage`。
3. 删除已有 `nextRunAt`，确保该字段只由后端当前计算结果控制。
4. `view.NextRunAt != nil` 时编码并写入 `nextRunAt`；为 nil 时保持省略。
5. 编码合并后的对象。

该方案只修复输出 DTO，所有任务字段仍由原有 `Task.MarshalJSON` 负责。

### 5.2 被否决的替代方案

| 方案 | 否决原因 |
| --- | --- |
| 仅在桌面 Wails 方法拼装 map | HTTP `/automation/tasks` 仍违反同一 `TaskView` 合同，并在两个出口形成重复逻辑 |
| 前端重新计算下一次时间 | 违背现有“后端时间为唯一事实源”的合同，并复制时区、有效期和漏跑语义 |
| 移除 `Task.MarshalJSON` | 会破坏 `UnknownFields` 的前向兼容保证，风险远大于本次缺陷 |
| 将 `TaskView` 展开为全部显式字段 | 产生大量机械字段和持续同步成本，不是最小修复 |
| 引入通用 JSON 合并 helper | 当前只有一个明确调用点，没有证据支持新增抽象 |

## 6. 合同、消费者与影响文件

受影响合同：

- `automation.TaskView` 的 JSON 输出增加原本已声明但实际丢失的可选 `nextRunAt`。
- `Task` 的既有字段名、未知字段透传、空值省略和持久化格式保持不变。
- Wails `AutomationListTasks` / `AutomationReloadTasks` 与 HTTP `GET /automation/tasks` 同步恢复该字段。

当前消费者：

- 桌面自动化页 `heartbeat.bridge.ts` / `HeartbeatPanel.tsx`。
- 项目自动化能力读取路径使用同一 `TaskView`；新增可选响应字段不改变其现有字段消费。
- Serve HTTP 自动化列表接口。
- Service 的 `CreateTask`、`UpdateTask`、`SetTaskEnabled` 也返回 `TaskView`；当前桌面适配器会解包 `view.Task`，因此这些 Wails 写方法的响应不受本次 marshaler 影响。任何直接编码这些 service 返回值的 host 则自动获得同一可选字段合同。

预计产品与测试文件：

- `internal/automation/service.go`
- `internal/automation/service_test.go`
- 本任务回执文档

若实现需要修改前端、调度、存储、Serve handler、Wails binding、依赖或模块文件，必须停止并回到计划阶段。

## 7. 验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 周期任务存在有效后续时间 | `json.Marshal(TaskView)` 输出 RFC3339 `nextRunAt`，值等于 `PreviewNextRuns` 的 UTC 结果 |
| AC-02 | `Task` 含未知 JSON 字段 | `TaskView` 输出仍保留该字段和值 |
| AC-03 | `NextRunAt` 为 nil | 输出省略 `nextRunAt`，即使 `Task.UnknownFields` 中存在同名陈旧值也不得泄漏 |
| AC-04 | 桌面 dev 应用加载截图中的启用任务 | 卡片不再显示“暂无下次运行”，而显示后端时间对应文案 |
| AC-05 | HTTP/Wails 共享合同 | 两个出口无需各自补丁即可获得相同 `TaskView` JSON 行为 |
| AC-06 | 保持行为 | 调度、运行记录、开关、任务数据、未知字段兼容和无后续时间的兜底行为不变 |

## 8. 风险、兼容与回滚

该变更触及 Wails 与 HTTP 共用的响应 DTO，按公共 API 合同变更执行高风险文档审查。行为是恢复已声明的可选字段，不删除或改名现有字段。

高风险只读审查结论为 Ready，无阻塞项。审查建议已纳入：明确 service 写方法的实际适配边界、最终验收直接检查 HTTP 原始 JSON，并在回执记录 A/B 证据路径。

| 风险 | 控制 |
| --- | --- |
| 绕过 `Task.MarshalJSON` 导致未知字段丢失 | 先使用原方法编码，并在回归测试中锁定未知字段 |
| 持久化未知字段中的旧 `nextRunAt` 覆盖计算值 | 合并前无条件删除同名字段，再按当前 `TaskView.NextRunAt` 决定写入 |
| nil 指针被编码为 `null`，改变前端兜底 | nil 时不写字段，测试锁定省略语义 |
| 共享 DTO 影响 HTTP 消费者 | 只新增已声明的可选字段；运行 `internal/serve` 受影响依赖测试 |
| 应用 A/B 启动共享 profile 时调度器产生新运行 | A、B 串行；记录启动前状态与时间，正常关闭 A 并确认进程/端口/锁释放后再启动 B；只比较同一任务列表读取路径，不手工删除锁 |

回滚只需撤销 `TaskView.MarshalJSON` 和对应回归测试。没有数据迁移、缓存清理或持久化回滚。

## 9. 验证计划

### 9.1 回归测试与受影响包

实现时先加入 JSON 回归测试并在未修复代码上记录失败，再完成产品修复。测试同时覆盖 AC-01、AC-02、AC-03。

```powershell
go test ./internal/automation -run TestTaskViewJSONIncludesBackendOwnedNextRun -count=1
go test ./internal/automation
go test ./internal/serve
cd desktop
go test .
```

`TaskView` 是跨包共享输出合同，因此除直接修改的 `internal/automation` 外，扩展验证 HTTP consumer `internal/serve` 和桌面 consumer。没有前端文件变化，不运行 `pnpm build`；若实际 diff 出现前端文件，则按仓库规则补跑构建并重新审视范围。

`internal/serve/automation_test.go` 当前把响应重新解码为 `TaskView`，嵌入的 `Task.UnmarshalJSON` 方法提升使该测试不能证明 `NextRunAt` 被回填。主 Agent 最终验收应读取 `GET /automation/tasks` 的原始 JSON 对象并直接断言 `nextRunAt`，不为此扩大产品代码。

### 9.2 真实应用 A/B

使用与 `D:\Reasonix` 相同的 dev 配置、服务、环境 ID 和数据目录，不创建 per-worktree runtime profile：

1. 从同一 `341f21d3` 基线创建保持不变的 A worktree，从其 `desktop/` 运行 `wails dev`。
2. 记录有效 dev 配置、目标任务字段、当前时间和最近运行；打开自动化页并保存“每30分钟 · 暂无下次运行”证据及文件路径。
3. 正常关闭 A，确认进程停止、开发端口和 singleton lock 已释放。
4. 不手工删除锁；记录 A 期间是否产生新运行，确保 B 比较时使用同一当前持久化状态和观察步骤。
5. 从修复 worktree 的 `desktop/` 运行 `wails dev`，打开同一任务，确认显示有效下次运行文案且运行/开关状态保持，并保存对应证据路径。
6. 检查应用控制台和 Wails 日志没有新增序列化错误；B 通过后保持修复版 dev 进程运行，供用户手工验收。

如果共享 profile 在 A/B 间发生不可恢复的外部状态变化，则明确报告为“同配置、连续状态比较”，不得声称完全等价初始状态；JSON 回归测试仍提供可控的同输入红绿证据。

### 9.3 PR 前 Gate

仅在实现、独立复核、A/B 验收完成且获得 PR 授权后，同步最新 `main-v3` 并运行：

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code origin/main-v3...HEAD -- scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

使用 `git diff --name-only -z origin/main-v3...HEAD` 的完整 NUL 路径集核对分类摘要。任何 required change-related check 失败时不得创建或更新 PR。
