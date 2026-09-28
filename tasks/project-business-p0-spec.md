# Spec: 项目业务 P0 改进

## Objective

在不改变三仓公共 API、数据库结构和既有业务行为的前提下，完成三项 P0：统一 Desktop 项目业务术语；为 Cloud 无外键项目表提供只读一致性扫描；拆分 Desktop 项目工作台控制器职责。

## Tech Stack

- Desktop：React 19、TypeScript、Vite、Wails。
- Cloud：Go、Gin、GORM、PostgreSQL。
- Admin：本轮保持只读且不修改。

## Commands

- Desktop build：`pnpm build`（目录：`desktop/frontend`）。
- Desktop focused tests：`pnpm exec vitest run --config vitest.config.ts src/vitest/project-cloud-ui.vitest.test.tsx src/vitest/project-collaboration-v2.vitest.test.tsx src/vitest/project-work-item-ui.vitest.test.tsx`。
- Cloud scanner tests：`go test ./internal/projects -run '^Test(ProjectInvariantRulesStayReadOnlyAndExcludeIntentionalDetachedState|MissingInvariantTablesAreSorted)$' -count=1`，以及 `go test ./cmd/projectinvariants -count=1`。
- Cloud live read-only scan：`go run ./cmd/projectinvariants`。

## Project Structure

- `desktop/frontend/src/features/projects/`：项目 UI、Gateway 和领域展示类型。
- `desktop/frontend/src/locales/zh.ts`：唯一简体中文产品文案事实源。
- `internal/projects/`：Cloud 项目事务、读取模型和一致性检查。
- `cmd/projectinvariants/`：可由人工或外部调度器调用的只读扫描入口。

## Code Style

```go
// ScanProjectInvariants only reports inconsistencies; it never repairs rows.
func ScanProjectInvariants(ctx context.Context, db *gorm.DB) (InvariantReport, error)
```

- 复用现有 GORM、配置和数据库连接，不新增依赖。
- React 重构只移动现有状态与回调，不改变 Wails/Gateway 契约。
- 导出 Go API 和非显然边界必须保留有用注释。

## Testing Strategy

- 术语通过现有项目 UI 测试和生产构建验证。
- 扫描器通过聚焦 Go 测试验证只读查询、适用表门控、稳定输出和退出语义；再对当前集成数据库执行一次只读扫描。
- 工作台拆分通过现有项目 UI 测试和 TypeScript/Vite 生产构建证明行为与类型契约不变。

## Boundaries

- Always：保持 CloudProject、WorkItem、ProjectTask、LocalWorkspace 等 wire/type 名称不变；扫描 SQL 只读；每个切片后运行聚焦验证。
- Ask first：新增 migration、外键、生产后台周期任务、自动修复、公共 API 变更。
- Never：输出数据库凭据；把项目清理任务或删除墓碑误报为普通孤儿；修改 Admin 写权限；覆盖无关工作区改动。

## Success Criteria

- 项目 UI 将 WorkItem 一律称为“计划项”，ProjectSession 称为“执行会话”，ProjectTask 称为“会话历史/会话分支”，LocalWorkspace 称为“本地目录”。
- Cloud 命令输出机器可读报告；缺少可选表时标为不适用；发现违规时返回专用非零退出码；不执行任何写 SQL。
- `CloudProjectsSectionImpl.tsx` 的目录、工作区、生命周期和上下文控制职责迁入聚焦模块，外部 props 和运行行为不变。
- Desktop 构建、聚焦 UI 测试、Cloud 聚焦测试和只读 live scan 均有真实结果。

## Open Questions

- P1 mutation orchestration：后续先把 create/patch/archive/restore/delete 的 pending、receipt、冲突恢复整理为一个显式状态机，再逐个迁移调用方。
- P1 offline matrix：后续为计划项、会话历史、自动化和资产分别固定 offline/stale/read-only 行为，不用一个全局布尔值代替。
- P1 automation/assets：后续分别补齐本地/云端双状态展示，以及扫描、解析、索引、清理状态的可见性。
- P2 Admin：继续定位为治理与审计入口，不复制 Desktop 项目工作台。
