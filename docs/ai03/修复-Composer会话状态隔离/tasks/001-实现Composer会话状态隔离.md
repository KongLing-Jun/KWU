# 任务 001：实现 Composer 会话状态隔离

## 任务状态

- 状态：完成。
- 执行组：Serial-01。
- 依赖：已批准的 [`plan.md`](../plan.md)。
- 交付切片：一个可独立评审、验证和回滚的会话状态隔离修复。
- PR 映射：后续如获授权，本任务对应一个实现 PR；当前未授权 PR。
- 加速授权：用户明确授权计划、任务、代码连续完成并小步提交，无需等待
  独立文档 PR 合并。
- 指派：当前主会话，不委派子代理。

## 目标与预期产物

使每个 Desktop 会话独立拥有 Composer 的模型、计费来源和推理强度。
会话内切换不再改写全局默认，历史会话、应用重启与 Fork/分支均恢复或继承
正确的会话快照。

预期产物：

- 向后兼容的 `BranchMeta` Composer 字段。
- Desktop 会话快照的保存、恢复、旧数据推断与 `auto` 兼容。
- Composer 仅写当前会话、设置页继续管理全局默认的 API 语义。
- 新空白会话的全局默认强度行为。
- Desktop Fork 和 Controller 分支的完整快照继承。
- 自动化回归测试、应用验收证据、完成回执。

## 源码与影响面依据

- 基线：`origin/main-v3@5c9a4ffcf3060c67c83b86454e35696431b116c3`。
- 主工作区 Codegraph 路径：
  `SetModelRouteForTab -> setModelRouteForTab -> setModelRouteForTabWithPersistence ->
  persistUserDefaultModelSelection`。
- `desktop/tabs.go:3701` 附近：打开 Tab 文件已保存三个状态，可作为恢复契约参考。
- `internal/agent/branch.go:20` 附近：关闭后的会话 sidecar 只保存 `Model`。
- `desktop/tabs.go:1559` 附近：打开历史 Topic 先取全局默认来源，造成覆盖。
- `desktop/tabs.go:2893` 附近：Controller 构建会读 session 模型，但已在当前来源
  配置下解析，无法修复已选错的来源。
- `desktop/app.go:2186` 附近：Desktop Fork 复制模型/强度，但漏掉计费来源。
- `internal/control/controller.go:2112` 与 `:2192` 附近：Controller Fork/Branch 创建
  sidecar 时没有继承 Composer 字段。
- `desktop/tab_profile_test.go` 已证明运行时强度是 Tab-local，但只覆盖打开 Tab 文件，
  未覆盖关闭后历史 sidecar。

## 允许范围

- `internal/agent/branch.go`、`internal/agent/branch_test.go`。
- `internal/control/controller.go`、`internal/control/branches_test.go` 或直接相邻测试。
- `desktop/app.go`、`desktop/tabs.go`。
- Desktop 内与会话恢复、Topic、Tab profile、Fork、计费路由直接相关的测试。
- 本任务文档与 `../progress.md`。

## 禁止范围

- 不修改前端选择器交互、草稿、附件、Token/MCP/场景归属。
- 不修改设置页 `SetDefaultModelRoute` 的全局默认语义。
- 不修改自动化任务的显式模型路由。
- 不修改模型不可用 fallback、订阅登录、BYOK 配置、重试 UX。
- 不扫描或批量改写用户历史，不修改云端账本或已执行任务事实。
- 不新增依赖、网络接口或全局数据库迁移。
- 不推送、创建/更新/合并 PR，不修改 `scripts/code_guard_baseline.json`。

## 必须保留的行为

- 设置页修改默认路由后，之后新建会话使用该路由。
- 已打开 Tab 的 `desktop-tabs.json` 持久化、预加载和安全路径校验不退化。
- 同计费路线内模型下架时继续使用现有 fallback 和通知。
- 订阅认证、余额操作、BYOK 凭据缺失和网关错误继续由现有
  `tabSendAvailability` 表达。
- 运行任务、待处理提示或后台任务存在时仍禁止重建切换。
- 会话路径校验、profile/actor 隔离、话题标题与场景元数据不变。

## 实现约束

1. `(model, billingSource)` 必须成对读写；不得用当前其他会话或全局来源
   解释一个已有会话。
2. `effort` 使用指针保留“缺字段”与“显式 auto/空字符串”的区别。
3. 旧规范模型引用仅在现有路由规则只匹配一个来源时回填。
4. 先在同步恢复路径确定会话路由，再按该来源加载配置/凭据，避免跨路线
   短暂加载。
5. 旧 sidecar 只在正常打开、快照或切换后惰性回填，不引入批处理。
6. 不修改无关的已超限文件或 code guard baseline；优先小型帮助函数和聚焦测试。

## 验收标准

1. 两个有历史的会话分别保持 DeepSeek/GPT，互相切换不改写。
2. 关闭后从历史 Topic 重开以及应用重启均恢复各自模型、来源和强度。
3. Composer 切换成功后用户配置的全局默认模型/来源不变。
4. 设置页默认继续初始化新会话；新会话不复制活动会话强度。
5. 旧 `model`-only sidecar 能唯一恢复订阅/BYOK，且强度为 `auto`。
6. Fork/分支初始继承完整 Composer 状态，父子之后切换互不影响。
7. 现有模型 fallback、SendAvailability、路径安全和运行任务门禁测试不回归。
8. 受影响 package 测试、Desktop 构建所需编译、code guard 与 diff/EOL 检查通过。

## 聚焦验证

```powershell
go test ./internal/agent -run 'TestBranchMeta.*Composer|TestBranchMetaRoundTrip' -count=1
go test ./internal/control -run 'TestBranch.*Composer|TestFork.*Composer' -count=1
Push-Location desktop
go test . -run 'Test.*(Composer|Topic.*Model|SetModelForTab|EffortForTab|Fork|PersistentTabRestore)' -count=1
Pop-Location
go test ./internal/agent ./internal/control -count=1
Push-Location desktop
go test . -count=1
Pop-Location
go run ./scripts/code_guard
git diff --check origin/main-v3...HEAD
```

实现迭代中先运行最邻近的精确测试；行为稳定后再运行受影响 package 范围。
实际应用验收按 `plan.md` AC-01–AC-12 执行。

## 完成回执

- 状态：实现、自动化验证和实际应用验收完成。
- 变更文件：
  - `internal/agent/branch.go`、`internal/agent/branch_test.go`
  - `internal/control/controller.go`、`internal/control/branches_test.go`
  - `desktop/session_composer_state.go`、`desktop/session_composer_state_test.go`
  - `desktop/app.go`、`desktop/tabs.go` 及直接相关 Desktop 测试
  - 本任务回执与 `../progress.md`
- 测试结果：
  - `go test ./internal/agent ./internal/control`：通过。
  - Desktop Composer/恢复/Fork/新会话聚焦测试：通过。
  - Desktop `go test .`：通过，`ok kwu/desktop 108.477s`。
  - `go run ./scripts/code_guard`：通过，未修改 baseline。
  - 当前工作树 `wails dev`：前端及应用编译成功并启动。
- 应用验收：真实历史会话完成 DeepSeek → GPT → DeepSeek 往返，Composer
  依次显示 `云智企办/deepseek-v4-flash`、`gpt/gpt-5.3-codex-spark`、
  `云智企办/deepseek-v4-flash`；证明跨启动恢复和会话往返未串模型。
- 偏差：无。
- 同步复核：rebase 到最新 `origin/main-v3` 后，Composer 状态继承已整合进新增的
  Fork 原子持久化 helper，并保留保存失败时的清理；Root Go 受影响包、Desktop
  完整包与前端生产构建重新通过。
- 剩余风险：未在真实 UI 中主动制造凭据失效；该路径未改动，并由既有
  fallback、SendAvailability 与完整 Desktop 回归覆盖。
- 交接：用户已授权推送和创建 Draft PR；Ready 与合并仍需另行授权。
