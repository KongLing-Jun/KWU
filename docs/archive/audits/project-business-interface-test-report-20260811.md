# 项目业务接口测试报告（2026-08-11）

## 结论

本轮不能判定为“项目业务所有接口通过”。

- 已修复资产上传不可用：Cloud 已升级到 `projects-v2-20260811.5`，Desktop 仍嵌入旧 `.4` 契约，能力校验按设计 fail-closed，导致资产页与上传按钮被禁用。同步契约、版本与 SHA-256 后，资产页恢复。
- 已在真实 `team-integration` 运行栈完成无敏感文本文件的上传和删除闭环：原生选择器、Desktop multipart、Cloud 对象存储/数据库、目录回读、容量更新和删除均成功；测试资产与本地临时文件已清理。
- Cloud 注册了 60 条项目相关路由。无认证安全探测 60/60 返回 401，证明路由注册和认证门禁生效。
- Cloud `internal/projects` 全量门禁共 101 个顶层测试：97 PASS、4 FAIL、0 SKIP。四项失败中包含 1 个明确功能回归、2 个测试数据漂移、1 个性能预算失败。
- 当前团队运行库还有一个独立的 schema 漂移：`POST /projects/onboarding/claim` 返回 500，因为 `project_onboarding_claims` 表缺失；全新从 1 迁移到 91 的隔离数据库不会缺表，说明现有长寿命数据库曾跳过后来补入的 00088。
- Desktop 项目域 Go 定向测试、56 个核心 Vitest、生产构建、`go vet` 和 `code_guard` 通过；另有 2 个 CSS-free 结构契约与当前看板实现不一致。

## 测试环境

| 项目 | 值 |
|---|---|
| Cloud | `http://127.0.0.1:9090/` |
| Desktop Wails | `http://127.0.0.1:34238/` |
| 环境标识 | `team-integration` |
| Desktop 工作树 | `D:\project\Reasonix-worktrees\integration-main-v3-20260811` |
| Desktop 分支 / HEAD | `codex/integration-main-v3-20260811` / `eb354e3a` |
| Cloud 工作树 | `D:\project\reasonix-cloud-platform\Reasonix-Cloud-worktrees\integration-20260811` |
| Cloud 分支 / HEAD | `codex/integration-20260811` / `8e1f3c3c` |
| Cloud schema | 00091（资产目录与容量） |
| 公开契约 | Projects v1 `projects-v1-20260809`; Projects v2 `projects-v2-20260811.5` |
| Projects v2 SHA-256 | `8d57c4cbe739edaf060359f9ff23287a045b2d1bd3a55de9a2919ef99a7c6868` |

测试过程没有复制或输出 `.env`、数据库连接串、Token 或对象存储凭据。

## 接口清单

Cloud 路由注册证据来自 `internal/projects/handler.go:156-233`。全部路径前缀为 `/api/v1`。

| 分组 | 数量 | 接口 | 生产门禁 |
|---|---:|---|---|
| Readiness | 2 | `GET /projects-v1/readiness`; `GET /projects-v2/readiness` | Auth；对应 provider/gate 存在时注册 |
| V1 核心及 onboarding | 13 | `POST /projects`; `POST /projects/onboarding/claim`; `GET /projects`; `GET/PATCH /projects/{project_id}`; `GET/PUT /projects/{project_id}/instructions`; `GET/POST /projects/{project_id}/work-items`; `GET/PATCH/DELETE /work-items/{work_item_id}`; `POST /work-items/{work_item_id}/move` | Auth + V1 enabled；WorkItem 另需 WorkItems slice |
| V2 members | 10 | `POST /projects/aggregates`; `GET /projects/{id}/aggregate`; `GET /projects/{id}/members`; invite create/list/revoke/accept/status；member deactivate；`DELETE /projects/{id}` | Auth + members slice |
| V2 activity | 4 | activities list；message post；message client-id readback；activity-read put | Auth + activity slice |
| V2 labels | 6 | label list/create/patch/delete；WorkItem label put/delete | Auth + work_item_fields slice |
| V2 bindings | 2 | `GET/PUT /projects/{id}/bindings` | Auth + bindings slice |
| V2 automation | 8 | automation list/create/patch；run create/get/reclaim/abandon/result | Auth + manual_automation slice |
| V2 assets | 12 | upload/list/directory；folder create/patch/delete；asset patch/processing/download/delete；WorkItem asset put/delete | Auth + assets slice + migration 91/schema readiness |
| Admin | 3 | admin project list/detail/governance | no-store + Auth + Admin + Admin Projects gate |
| 合计 | **60** | 2 readiness + 55 用户业务 + 3 admin | 全部认证保护 |

Projects v1/v2 契约包含 53 条去重用户路由。下列两条生产路由尚未进入路由契约：

- `POST /projects/onboarding/claim`
- `GET /projects/{project_id}/assets/processing-states`

## Desktop 公开适配面

| 层 | 数量 | 说明 |
|---|---:|---|
| Desktop 导出 Wails 项目方法 | 64 | 项目 CRUD、绑定、指令、会话、成员/邀请、动态、能力、自动化、计划项/标签、任务视图、资产全生命周期 |
| 前端 `ProjectGateway` | 53 个唯一操作 | 上传存在 legacy/目录版两个 overload；部分自动化运行/调度方法尚未暴露给当前项目 UI |

主要 Wails 分组：

- 项目与工作树：`List/Get/Create/Patch/Archive/Restore/DeleteCloudProject`、绑定/重绑/解绑、onboarding claim。
- 指令与会话：指令读取/编辑/清空、`NewCloudProjectSession`、特性读取。
- 协作：成员、邀请、动态、消息、已读水位。
- 计划项：WorkItem 列表/创建/编辑/移动/删除，标签 CRUD 与关联。
- 能力与自动化：能力绑定、自动化列表/创建、调度保存/删除、run claim/reclaim/abandon/complete/execute。
- 资产：旧列表、目录查询、文件夹 CRUD、文件 patch、原生选择器上传、下载、删除、处理状态、WorkItem 附件关联。

## 验证结果

### Cloud

| 验证 | 结果 |
|---|---|
| `healthz / readyz / environmentz` | 200 / 200 / 200 |
| 60 条注册路由无认证安全探测 | 60/60 返回 401 |
| 非 integration 顶层测试 | 58/58 PASS |
| 隔离 PostgreSQL 非已知失败测试 | 37/37 PASS |
| 全部 `internal/projects` 顶层测试 | **97 PASS / 4 FAIL / 0 SKIP（101）** |
| Projects v1/v2 fixture 测试 | PASS |
| 00091 schema 测试 | PASS |
| 资产专项隔离 PostgreSQL 测试 | PASS |
| Payment/OperatorConsole 容量测试 | PASS |
| `go vet ./...` | PASS |
| 临时数据库 | migration 1–91 成功；测试后自动删除 |

未配置 `TEST_DATABASE_URL` 时直接运行迁移 integration tests 会按设计失败并提示必须提供隔离数据库；这不是测试通过，也没有被记作通过。

### Desktop / Frontend

| 验证 | 结果 |
|---|---|
| `go test ./internal/project -count=1` | PASS |
| Desktop 项目/Wails 定向 Go 测试 | PASS |
| 6 个项目核心 Vitest 文件 | 56/56 PASS |
| 资产 UI 回归文件 | 12/12 PASS |
| ProjectGateway CSS-free contract | PASS |
| 项目模板指令 contract | 16/16 PASS |
| 项目目录渲染 contract | 35 PASS / 2 FAIL |
| `pnpm build`（CSS、tokens、tsc、Vite） | PASS；仅有既有 chunk-size warning |
| `go vet ./...` | PASS |
| `go run ./scripts/code_guard` | PASS |

### 真实运行态成功路径

当前登录身份下已确认以下生产路径返回成功：项目列表/详情、项目指令读取、计划项列表、标签列表、资产目录、资产处理状态。

资产上传闭环证据：

1. Desktop 资产页显示 `0 B / 5 GB`，上传按钮可用。
2. Wails 原生文件选择器打开并选择 154 B 无敏感文本文件。
3. `POST /api/v1/projects/{project_id}/assets` 返回 200（234 ms）。
4. `GET /asset-directory` 返回 200，界面显示文件名、更新人、日期和 `154 B / 5 GB`。
5. 通过 UI 选择并删除测试资产，`DELETE /assets/{asset_id}` 返回 200；列表/处理状态/目录回读均为 200。
6. 界面恢复空目录；Cloud 测试对象与本地临时文件均已清理。

## 已发现问题

| ID | 严重度 | 状态 | 问题与影响 |
|---|---|---|---|
| P0 | 高 | LIVE FAIL | 当前团队运行库缺失 `project_onboarding_claims`，`POST /projects/onboarding/claim` 返回 500，项目页出现“详情加载失败”。00088 是后来补入的迁移，而长寿命库已在更高 goose version，普通重跑不会回补。需要单独批准修复迁移/运维回补。 |
| P1 | 高 | 功能回归 | `TestProjectServiceInitialInstructionFailureWindowsRotationAndPrivacy`：同一幂等 key 修改 `agent_instruction` 返回 nil，而非 `ErrIdempotencyConflict`；违反 V1 changed-digest 应为 409。 |
| P2 | 中 | 测试数据漂移 | `Shared project name` 超过当前 15 字符限制，幂等 target 隔离测试提前失败，目标场景未执行。 |
| P3 | 中 | 测试数据漂移 | `Stage 2 and 3 integration` 超过 15 字符，V2 综合 E2E 首步失败；members/activity/labels/bindings/automation/delete 的真实 PG 综合流未执行。 |
| P4 | 中 | 性能门禁 | WorkItem `total_count` 使每次列表由 4 queries 变为 5；100 次为 500，旧预算为 400。应优化合并计数或明确批准新预算。 |
| P5 | 中 | 契约缺口 | onboarding claim、asset processing-states 未纳入 v1/v2 route fixture；V2 测试只检查 47 条数量，不校验 handler 集合 parity。 |
| P6 | 中 | 并发契约缺口 | folder/file delete 未携带 `expected_version`，与既定“修改继续携带版本冲突保护”不一致。 |
| P7 | 低 | 前端测试漂移 | CSS-free contract 仍要求 `loadedItemCount` 和 `data-plan-toolbar`；当前实现使用完整云端 `totalCount` 和新的看板工具栏结构，2 条断言失败。需按当前产品契约更新测试，或恢复相应语义标记。 |

## 资产测试覆盖缺口

- 本轮真实运行态已证明默认 provider 的上传、对象保存、数据库回读、容量更新和删除；尚未验证签名下载、对象存储故障、删除清理失败及超额后的真实对象清理。
- 隔离 PG 资产测试使用 `projectAssetObjectFixture`，不能代替 provider E2E。
- 多数资产路由没有直接 handler 成功测试；现有直接测试主要覆盖 folder-delete envelope 和 processing-states。
- 缺少目录循环移动、搜索结果、分页/排序结果、并发容量竞争及目录权限隔离的实库测试。
- 已实现 `WITH RECURSIVE` 循环保护，但没有测试命中该分支。

## 建议处理顺序

1. 新增受审查的补偿迁移，安全创建缺失的 onboarding 表并在 `team-integration` 回补；先验证当前库对象与 goose 历史，不能直接降版本。
2. 修复 P1 的 instruction 幂等 digest 冲突回归并增加 handler 409 回归测试。
3. 修正 P2/P3 的测试数据，使完整 PG 流真正执行。
4. 决定 P4：把列表恢复到 4-query 预算，或记录并批准 5-query 新预算。
5. 建立 handler-route 与 fixture-route 集合等价测试，并把两条漏路由纳入公开契约。
6. 为 delete 加 `expected_version`，补资产 handler/provider/并发与权限测试。
7. 按当前云端计数和看板结构更新两条 CSS-free 断言。

## 关键命令

```powershell
# Desktop
go test ./internal/project -count=1
go test . -run 'Test.*(CloudProject|WorkBuddyProject|ProjectAsset|ProjectInstruction|ProjectCollaboration|ProjectContract|ProjectSession)' -count=1
pnpm.cmd exec vitest run <6 project test files> --reporter=dot
pnpm.cmd build
go vet ./...
go run ./scripts/code_guard

# Cloud
go test ./api/openapi/fixtures/projects-v1 ./api/openapi/fixtures/projects-v2 -count=1
go vet ./...
powershell.exe -File ./scripts/workbuddy-project-full-gate.ps1 -EnvironmentFile <隔离测试配置>
```

报告中的通过/失败均基于实际执行结果；没有把 compile-only、无认证 401、被前置失败阻断或未运行的成功路径记作通过。
