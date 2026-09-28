# Spec: 项目业务 P1/P2 状态与治理

## Objective

在不改变 Cloud 权威边界、不复用用户写接口给 Admin、不破坏旧 Asset 响应的前提下，完成：统一 mutation 失败转移；明确离线能力矩阵；展示 Automation 云端定义与本机调度双状态；提供版本化 Asset 处理状态；为 Admin 增加独立只读治理聚合。

## Tech Stack

- Desktop：Go/Wails、React 19、TypeScript、Vitest。
- Cloud：Go、Gin、GORM、PostgreSQL。
- Admin：Vue 3、Naive UI、Alova、Node contract tests。

## Commands

- Cloud 完整项目测试：`./scripts/workbuddy-project-full-gate.ps1 -EnvironmentFile .env -PostgresBin 'C:\Program Files\PostgreSQL\18\bin'`。
- Desktop Go 测试：在仓库根运行 `go test ./internal/project -count=1`，并在 `desktop/` 模块运行 `go test . -count=1`。
- Desktop UI 聚焦测试：`pnpm exec vitest run --config vitest.config.ts <test files>`。
- Desktop build：`pnpm build`（`desktop/frontend`）。
- Admin tests/build：`pnpm test`、`pnpm build`。

## Project Structure

- `desktop/project_*_app.go`、`internal/project/`：本地 Journal、Cloud Gateway 与 Wails 投影。
- `desktop/frontend/src/features/projects/`：项目状态策略、Automation 与 Asset UI。
- Cloud `internal/projects/`：Asset 状态和 Admin 安全聚合。
- Admin `src/api/reasonix/admin-projects.ts`、`src/views/reasonix/projects/`：只读治理消费面。

## Code Style

```go
// SettleProjectCommandFailure records only the shared lifecycle transition;
// each domain keeps its own payload validation and authoritative readback.
func SettleProjectCommandFailure(...) error
```

- 复用现有 Journal、Gateway、readiness 与 `adminReadDB`。
- 新响应只增加安全计数或枚举，不返回 prompt、正文、身份、SHA、对象键或运行摘要。
- 不用 Automation 名称推断本机与云端关联，只认 project_id + automation_id。

## Testing Strategy

- mutation 从公开 App seam 验证确定性拒绝清理与未知结果保留。
- 离线矩阵通过纯策略测试和 DOM seam 验证，禁止 stale 页面发起在线分片请求。
- Automation join 使用纯展示模型测试同名、孤儿、云失败与未调度状态。
- Asset 新 endpoint 保持旧 `/assets` payload 不变，并验证安全字段和项目隔离。
- Admin 治理接口通过独立 TEST_DATABASE_URL 验证桶计数、权限、no-store 和禁用字段；Admin 继续禁止任何项目写请求。

## Boundaries

- Always：使用独立临时数据库；保持旧响应兼容；Admin 只调用 `/admin/projects/**` GET。
- Ask first：新增数据库列、生产后台 worker、Admin 治理写命令、审计导出保留策略。
- Never：把运行中集成库作为测试库；按名称关联 Automation；向 Admin/浏览器泄露内容、身份、对象键或内部错误。

## Success Criteria

- Project/Instruction/WorkItem 的确定性拒绝统一清理 Journal，未知结果仍可恢复。
- stale 模式明确区分缓存摘要、本机会话/目录和必须联网的 Cloud 分片。
- Automation 同时显示云定义与本机调度状态，并识别孤儿链接。
- Asset 处理状态通过独立读取接口可见，旧 Asset DTO 不变。
- Admin 显示 Automation/Asset 健康聚合，仍无项目写接口。
- 三仓相关测试、构建和 Cloud 完整项目测试通过，临时数据库完成清理。

## Open Questions

- 项目封禁、成员冻结、资产隔离与审计导出继续 deferred；它们需要独立命令、持久审计源和权限设计。
