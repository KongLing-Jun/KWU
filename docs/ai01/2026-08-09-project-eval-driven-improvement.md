# 项目业务 eval-driven 改进记录（2026-08-09）

## 冻结范围

| 仓库 | 分支 | 基线 HEAD | 运行端口 |
|---|---|---|---|
| Desktop / KWU | `codex/integration-main-v3-20260809` | `cecb04ce241578cf6a28b6f933582f29e046230c` | Wails `34238`、Vite `5175` |
| Cloud | `codex/integration-20260809` | `b6ddc1f8fede9d54195284b39374edb42f23ad0b` | `9090` |
| Admin | `codex/integration-20260809` | `1c6374c8b0d702b590d38a97a177da88cd13034b` | `5173` |

## Baseline

### 自动化证据

- Desktop `go test -json ./internal/project -count=1`：167 pass，0 fail。
- Desktop 项目相关 Vitest：5 files、33 tests 通过。
- Desktop `project-tree-runtime.test.ts`：12 pass、5 fail；旧期望未包含新增 `composerIdentity`。
- Desktop `desktop` 包 Project/WorkItem 联合测试：编译失败；测试仍引用已删除的 `CloudCandidateManifestSHA256`、`DesktopCandidateManifestSHA256`。
- Desktop `pnpm test:all`：测试 TypeScript 编译失败；其中项目 Gateway fake 缺少五个当前 capability 字段。
- Desktop production build：通过。
- Cloud `go test -json ./internal/projects -count=1`：99 pass、36 fail；36 个均因缺少隔离 `TEST_DATABASE_URL`，没有使用运行库代替测试库。
- Cloud `go test -json ./internal/http ./cmd/server -run Project -count=1`：10 pass、0 fail。
- Cloud migration Project/WorkBuddy：9 pass、8 fail；8 个均因缺少隔离 `TEST_DATABASE_URL`。
- Admin `pnpm test`：45 files 通过；`pnpm build` 通过。两个 `*.acceptance.js` 未由该命令执行，外部 manifest gate 也没有产生可信实时证据。

### 真实浏览器基线

- 创建、名称校验、指令、WorkItem 创建/移动/刷新持久化、搜索、邀请/撤销、绑定/解绑、归档/只读/恢复、永久删除：通过。
- 动态留言：提交后显示“结果未知”，Cloud 日志中 `POST /api/v1/projects/:id/messages` 连续返回 400；刷新没有恢复该留言。
- 新建项目：普通“项目说明”丢失；编辑项目说明后可保存。
- Assets、工作项附件：明确不可用。
- Plan：列表、甘特图、日历不可用；表格、看板可用。
- Automation：仅手动模式；计划/每日模式不可用。
- Admin：成员、动态、自动化聚合显示 v1 兼容不可用；当前 acceptance 没有断言 v2 数值。
- 归档/恢复期间观察到一次陈旧 modal 状态。
- 测试项目已清理；未修改既有项目。

视觉基线：`.codex-run/project-eval-loop-20260809/baseline-desktop-1280x720.png`。仓库已有 1280/1440/820/720 synthetic reference，但没有 390px 手机证据，也不等同于真实产品 reference 对比。

## 已确认根因与优先级

1. **P0 动态留言 400**：Cloud 严格要求 `mention_user_ids` present 且 non-null；Desktop Gateway 直接序列化 `MentionUserIDs`，nil 会成为 JSON `null`。持久化及按 client id 恢复链路本身已实现。
2. **P0 创建说明丢失**：Members v2 开启时 Desktop 改走 aggregate create；Desktop aggregate DTO、Cloud aggregate contract/parse/canonical/SQL 均没有普通 `description`，SQL 固定写 `NULL`。
3. **P1 evaluation 盲点**：动态提交、aggregate description、Admin v2 projection、390px 响应式均缺少有效自动化证明。
4. **P1 Admin v2 未就绪**：Admin 是刻意只读运营投影；Cloud v2 readiness 未满足时正确回退 v1，但现有绿色测试无法证明真实五项计数。
5. **P2 明确未交付**：Assets、附件、Plan 列表/甘特/日历、计划自动化，以及部分 capability 入口。

## 迭代记录

| 迭代 | 单一假设 | Evaluation before | 修改 | Evaluation after | 决策 |
|---|---|---|---|---|---|
| 1 | 无提及时稳定发送 `mention_user_ids: []` 可消除留言 400 | 真实浏览器失败；Cloud POST 400；新增边界测试 RED：读到 `nil`，exit 1 | Desktop Gateway 将 nil mention slice 归一化为空数组；新增请求体回归测试 | 目标测试 GREEN；`go test ./internal/project -count=1` GREEN；真实临时项目留言显示“已完成权威回读”，刷新后仍存在；Cloud POST 200、按 client id GET 200 | 保留，运行时验证完成 |
| 2 | aggregate 原子支持 `description` 后，新建说明可立即权威读回 | 临时项目填写说明后详情显示“暂无项目说明”；Cloud handler 新字段 RED 400 | Cloud strict DTO/4000 UTF-8 限额、canonical 三态、原子 SQL、Desktop Gateway/readback、`projects-v2-20260809` 双方契约与 hash 同步 | Cloud handler/canonical/contract/server build GREEN；Desktop `internal/project` 全量 GREEN；真实创建立即显示说明，整页重载后仍存在；临时项目已永久删除 | 保留，运行时验证完成 |
| 3 | 修复过期 test doubles/manifest/精确期望后，项目门禁可恢复有效运行 | Desktop 联合测试编译失败；Gateway fake 类型失败；project-tree 12 pass/5 fail | 补齐 joint manifest 字段、五项 capability fake、WorkItem v2 字段和 default composer identity 期望 | Desktop Wails 项目目标测试 GREEN；Gateway contract GREEN；project-tree 17/17 GREEN；全量 test typecheck 不再含项目错误，但仍有 7 个非项目测试编译错误 | 保留；项目门禁改善，仓库全量仍非 GREEN |
| 4 | 补齐关键响应式与 v2 投影证据，可暴露真实移动端交互缺陷而不是只依赖静态快照 | Desktop 无 390px 项目详情快照；不可用 Connector/Skill 仍可选择；Admin acceptance 不断言五项 v2 计数；同一 Admin 抽屉由 1280 缩到 390 后占满 390px（RED） | 新增 Desktop 390×844 视觉用例；禁用尚不可用的新能力选项但保留已绑定项的移除入口；Admin acceptance 增加五项计数、DPR、无横向溢出和 1280→390 resize；抽屉复用 VueUse 响应式断点 | Desktop 390 视觉测试 1/1、项目 Vitest 32/32；Admin mock-v2 acceptance 8/8，390 抽屉 358.8px（92vw）、720@125% 与 1280 均无页面溢出 | 保留；mock-v2 与 synthetic 视觉证据有效，但不冒充当前 live Cloud v2 就绪证据 |
| 5 | 旧视觉快照保留了源码已明确隐藏的原生 `details` 三角标记 | 全量视觉回归 3/7；4 张仅在生命周期菜单三角处有 62–737 像素（约 0.01）差异；单 worker 稳定复现 | 检查实际图、期望图和 CSS；确认当前源码、当前 Chromium 与新 390 基线均只显示 `•••` 后，仅重建 4 张旧基线 | `pnpm exec playwright test e2e/workbuddy-project-visual.spec.ts --workers=1`：7/7 PASS | 保留；没有降低断言或放宽像素阈值 |

## 当前最佳结果

- 真实 Desktop→Cloud 核心链路：创建（含说明）、权威读回、刷新持久化、动态留言、WorkItem、搜索、邀请/撤销、能力绑定/解绑、归档/恢复/永久删除均完成验证；所有测试临时项目已永久删除。
- P0 动态留言 400 与 aggregate 创建说明丢失均有 RED→GREEN 自动化、真实浏览器、Cloud 日志和刷新持久化证据。
- Desktop 项目专项：Go package、Wails 联合目标、4 个 Vitest 文件 32 个用例、7 张视觉快照与 production build 均通过。
- Cloud 项目专项：aggregate handler/canonical、双端 contract fixture、HTTP/Server Project 测试与 server build 均通过。
- Admin：45 个现有测试、production build、v2 mock 浏览器验收 8/8 通过；1280、720@125%、390 resize 均无页面级横向溢出。

## 仍未完成或未验证

| 能力 | 当前判断 | 证据边界 |
|---|---|---|
| Assets、工作项附件 | 未完成 | UI 明确不可用，未发现可完成用户目标的后端/前端闭环 |
| Plan 列表、甘特图、日历 | 未完成 | 当前仅表格、看板可用 |
| Automation 计划/每日触发 | 未完成 | 当前仅手动 Automation；没有计划/每日完整链路 |
| Public Connector、我的专家、已安装、SkillHub 等部分能力入口 | 未完成/未开放 | 目录或能力存在但不可用项现在不能再被新增绑定 |
| Core v1 WorkItem 永久删除 | 未完成 | 当前契约没有 delete；不要把状态迁移冒充删除 |
| Admin live v2 五项聚合 | 未就绪 | Cloud readiness/RC 条件未满足时正确回退 v1；8/8 是 mock-v2 契约与 UI 证据，不是 live-v2 证据 |
| Cloud PostgreSQL 集成与 migration 回归 | **NOT VERIFIED** | 没有隔离的 `TEST_DATABASE_URL`；未使用正在运行的 PostgreSQL 代替测试库 |
| Desktop 全仓 `pnpm test:all` | FAIL（非项目） | 仍有 7 个 capabilities/installer/model-marketplace/settings 测试 TypeScript 错误；本轮项目相关错误已清零 |

## 当前最大剩余瓶颈

当前项目专项门禁已经覆盖 Desktop Gateway/runtime/Wails、Cloud contract/handler/build、Admin v2 mock acceptance 以及 390px/720@125%/1280 视觉几何。最大的证据缺口是 Cloud 数据库集成测试缺少隔离的 `TEST_DATABASE_URL`，因此不会拿正在运行的 PostgreSQL 冒充测试库；当前 live Admin 也因 Cloud v2 readiness 未满足而正确回退 v1。产品侧最大剩余瓶颈为 Assets、附件、Plan 列表/甘特/日历、计划/每日自动化，以及部分 capability 入口明确未交付。仓库全量 Desktop test typecheck 仍有非项目测试错误，不应误报为全绿。
