# 任务 001：恢复自动化下次运行 JSON 合同

## 任务元数据

| 项目 | 内容 |
| --- | --- |
| 状态 | 实施与功能验收完成；最终 PR Gate 被已确认的基线链接问题阻断 |
| 计划 | [`../plan.md`](../plan.md)，计划提交 `0bbb9049` |
| 交付 | 一个共享 DTO 序列化修复、一个聚焦回归测试、一个独立完成回执提交 |
| 依赖 | 无前置实现任务；必须先通过高风险文档审查并获得用户实施批准 |
| 执行代理 | fresh `gpt-5.6-luna`，reasoning `max`，context fork `none` |
| 验收责任 | 主 Agent 独立审查提交、diff、测试证据、真实应用 A/B 和最终 Gate |

## 目标与交付物

修复 `automation.TaskView` 的 JSON 输出，使服务已计算的 `NextRunAt` 以 `nextRunAt` 传给桌面 Wails 和 HTTP 列表消费者；保持 `Task` 的未知字段兼容和 nil 省略语义，不修改调度或前端。

交付物：

- `TaskView.MarshalJSON` 的最小实现。
- 在未修复基线上失败、修复后通过的 JSON 回归测试。
- 聚焦测试、受影响消费者测试、diff 审计和实际变更回执。
- 一个合规代码提交和一个独立 `docs:` 回执提交。

## 已确认的证据

| 证据 | 对本任务的约束 |
| --- | --- |
| `internal/automation/service.go` 的 `TaskView` | 嵌入 `Task` 并声明 `NextRunAt`，本任务只修复该响应 DTO |
| `taskViewWithNextRun` | 已正确计算 UTC 后续时间，不得修改算法 |
| `internal/automation/model_v2.go` 的 `Task.MarshalJSON` | 为 `UnknownFields` 提供前向兼容，必须继续复用 |
| `AutomationReloadTasks` 与 `/automation/tasks` | 都直接返回/编码 `[]TaskView`，共享修复必须同时生效 |
| `heartbeat.bridge.ts` 与 `HeartbeatPanel.tsx` | 已消费 `nextRunAt` 并在缺失时显示兜底，不需要前端修改 |
| 基线聚焦测试 | `TestServiceListTasksIncludesBackendOwnedNextRun` 通过，只证明 Go 字段存在，没有跨过 JSON 边界 |
| 实际应用截图与 profile 数据 | 启用的 `interval/30m` 任务持续成功运行，界面仍显示“暂无下次运行” |

## 允许范围

- `internal/automation/service.go`
- `internal/automation/service_test.go`
- `docs/ai03/修复-自动化下次运行显示/tasks/001-恢复下次运行JSON合同.md`，仅用于填写完成回执

实现者不得修改其他文件。若测试证明必须触及其他源码或测试文件，停止并向主 Agent 报告，不得自行扩大范围。

## 禁止范围

- `internal/automation/schedule.go`、Engine、Store、迁移和任务数据格式
- `internal/serve/**`、`desktop/**`、Wails bindings 和所有前端文件
- `Task.MarshalJSON`、`Task.UnmarshalJSON`、`knownTaskJSONFields` 或 `TaskView` 反序列化行为
- 依赖、`go.mod`、`go.sum`、构建配置、PR Gate 和 code guard baseline
- 通用 JSON helper、新 DTO、反射方案或无当前证据支持的防御性处理
- 推送、创建/更新 PR、合并、force-push 或继续委派

## 必须保持的行为

- `PreviewNextRuns`、时区、有效期、漏跑策略和调度触发完全不变。
- `Task.MarshalJSON` 继续决定所有持久化任务字段和未知字段透传。
- `NextRunAt == nil` 时 JSON 中没有 `nextRunAt`，而不是 `null` 或陈旧值。
- 其他 `TaskView` 字段的 JSON 名称和值不变。
- 任务创建、更新、开关、运行记录、HTTP handler 和前端兜底不变。
- 不把计算字段写入 `tasks.json`。

## 实现合同

开始前必须复述目标、证据、允许范围、保持行为和禁止范围；有物质歧义时停止，不得猜测。

1. 在 `service_test.go` 增加 `TestTaskViewJSONIncludesBackendOwnedNextRun` 或同等明确名称。
2. 测试必须把 `TaskView` 交给 `encoding/json`，从编码结果断言：
   - 非 nil `NextRunAt` 输出准确的 RFC3339 `nextRunAt`；
   - `Task.UnknownFields` 中一个无关字段仍原样存在；
   - nil `NextRunAt` 省略字段，即使 `UnknownFields` 含同名陈旧值。
3. 在只加入测试、尚未修复产品代码时运行该测试，记录预期失败输出；不要提交失败状态。
4. 在 `service.go` 为 `TaskView` 显式实现 `MarshalJSON`：先编码 `view.Task`，解析为 `map[string]json.RawMessage`，删除既有 `nextRunAt`，再按 `view.NextRunAt` 决定写入，最后编码对象。
5. 不提取 helper，不更改 `Task` 自身 marshaler，不增加与验收无关的错误包装或注释。
6. 运行聚焦测试与受影响包测试，审计实际 diff，确保每一行都能追溯到 AC-01、AC-02 或 AC-03。
7. 创建代码提交 `fix:修复自动化下次运行时间序列化`。
8. 填写本任务“完成回执”，创建独立提交 `docs:更新自动化下次运行修复回执`；回执不得记录承载自身的提交哈希。

若实现 materially 超出本合同、需要改变计划决定或发现根因不成立，立即停止，不得扩展设计。

## 验收映射

| 计划 AC | 本任务证明 |
| --- | --- |
| AC-01 | JSON 回归测试锁定准确 `nextRunAt`；实际应用 B 显示后端时间 |
| AC-02 | 回归测试锁定无关 `UnknownFields` 原样保留 |
| AC-03 | 回归测试锁定 nil 时删除同名陈旧未知字段并省略输出 |
| AC-04 | 主 Agent 在实现后完成串行 Wails dev A/B |
| AC-05 | 单一 `TaskView.MarshalJSON` 供 Wails 与 HTTP encoder 共用；受影响 consumer 测试通过，主 Agent 直接检查 HTTP 原始 JSON |
| AC-06 | 包测试、实际应用状态和 diff 审计证明调度及现有行为未改 |

## 聚焦验证

实现者按顺序运行并记录：

```powershell
go test ./internal/automation -run TestTaskViewJSONIncludesBackendOwnedNextRun -count=1
go test ./internal/automation
go test ./internal/serve
cd desktop
go test .
```

若测试名采用等价名称，首条命令同步调整。由于没有前端改动，禁止用 `pnpm build` 代替 Go consumer 验证，也不需要运行前端测试。

实现者还必须运行：

```powershell
git diff --check
git status --short
git diff --stat HEAD^..HEAD
git diff HEAD^..HEAD -- internal/automation/service.go internal/automation/service_test.go
```

完整 PR Gate、最新 `main-v3` 同步、HTTP 原始 JSON 检查和真实应用 A/B 由主 Agent 独立执行。

## 停止与回滚条件

- 测试无法在当前根因上稳定红绿。
- `Task.MarshalJSON` 的输出不能安全解析为 JSON 对象。
- 修复需要修改调度、持久化、Wails、Serve handler 或前端。
- 未知字段兼容、nil 省略或已有任务字段发生变化。
- 最新基线出现冲突修复或不同实现，导致计划依据不再成立。

回滚为整体撤销本任务的产品代码和回归测试；无数据或运行时状态回滚。

## 完成回执

| 项目 | 实施结果 |
| --- | --- |
| 最终状态 | 实现、独立代码审查、受影响包测试、串行真实应用 A/B 和 HTTP 原始 JSON 检查均完成；最终 PR Gate 被 `origin/main-v3` 已存在的 16 个缺失本地证据链接阻断，本变更未引入门禁失败。 |
| 实际变更文件 | `internal/automation/service.go`、`internal/automation/service_test.go`、本任务回执文件 |
| 红灯证据 | 仅加入测试时运行 `go test ./internal/automation -run TestTaskViewJSONIncludesBackendOwnedNextRun -count=1`，退出码 `1`；输出 `nextRunAt = "stale", want "2026-08-01T09:30:00Z"`，以及 nil 场景 `nextRunAt = "stale", want omitted when NextRunAt is nil`。未提交失败状态。 |
| 绿灯与包测试 | 聚焦测试同命令退出码 `0`；`go test ./internal/automation` 退出码 `0`（14.209s）；`go test ./internal/serve` 退出码 `0`（10.986s）；在 `desktop` 目录运行 `go test .` 退出码 `0`（91.755s）。 |
| 代码提交 | `4e3aa0b7`，`fix:修复自动化下次运行时间序列化` |
| diff 审计 | `git diff --check` 通过；代码提交仅修改允许的两个源码/测试文件，共 64 行新增；`TaskView.MarshalJSON` 复用 `Task.MarshalJSON`，删除同名未知字段后按 `NextRunAt` 写入或省略；未修改调度、存储、前端、Serve/Wails、依赖或计划。 |
| A/B 与 HTTP 证据路径 | A：未修复基线 `341f21d3` 在同一开发 profile 下显示启用的 `interval/30m` 任务为“暂无下次运行”，截图 `C:\Users\ai03\AppData\Local\Temp\reasonix-automation-next-run\baseline-a.png`。B：补丁版同一任务显示“每30分钟 · 17分钟后”，已执行单次任务仍显示“暂无下次运行”，截图 `C:\Users\ai03\AppData\Local\Temp\reasonix-automation-next-run\patched-b.png`。HTTP：补丁版同一 profile 的 `GET /automation/tasks` 返回 `200 application/json`，目标任务原始对象包含 `"nextRunAt":"2026-08-18T07:40:37.5709406Z"` 并可解析。A 启动时按既有策略补跑了一次逾期任务，因此 A/B 是共享状态连续对比，不宣称相同数据快照。 |
| 同步与最终 Gate | `git fetch origin main-v3` 后确认 `origin/main-v3` 仍为 `341f21d3`，无待同步提交。六条 common checks 中 code guard、baseline 文件存在性、baseline 文件三类 diff 和 `git diff --check` 均通过；完整 NUL 路径分类仅含本计划、任务回执和两个 automation 文件。`node ./scripts/pr-gate.mjs --base origin/main-v3 --run all` 正确分类为 Root Go targeted `./internal/automation`、无 Desktop Go、无 frontend，但在 Markdown 链接检查阶段因 `docs/ai01/2026-08-17-project-business-full-interaction-test.md:154-169` 的 16 个缺失本地产物失败；未改动基线 worktree 单独运行同一检查得到完全相同失败。 |
| 计划偏差 | 产品实现无偏差。先完成未修复代码红灯，再实施最小 marshaler 修复；未 push、未管理 PR、未扩大源码范围。真实 A 启动产生一次既有逾期补跑，已作为 A/B 初始状态限制如实记录。 |
| 剩余风险 | 产品验收未发现剩余行为风险；无法把完整 PR Gate 报告为通过，阻断项是本任务范围外且可在未修改基线上复现的 16 个缺失文档链接。按仓库规则未运行 `pnpm build`，因为没有前端变更；未创建或更新 PR。 |

## 交接要求

- 实现者只能在指定 worktree 和允许文件内工作，不能回退其他人的修改。
- 实现者完成后返回提交、测试命令与结果、变更文件、偏差和风险；不得只给结论。
- 主 Agent 必须直接审查提交和 diff，不得仅采信实现者报告。
- 任何复核失败返回同一实现者创建新的 `fix:` 提交和独立回执更新，不默认 amend 已审查提交。
