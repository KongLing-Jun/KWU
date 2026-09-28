# Composer 会话审批姿态持久化修复计划

## 背景与问题

桌面 Composer 的 Ask、Auto、YOLO 是当前交互式会话的运行时审批姿态，不改变底层 permission rules。当前实现把姿态保存在 `WorkspaceTab` 和 `desktop-tabs.json`，并在前端选择时调用全局默认偏好写入；会话 sidecar 没有该字段。因此同一 UI tab 复用另一历史会话、单表面切换、应用重启或从历史恢复时，姿态可能来自 tab 或全局默认，而不是会话本身。

失败语义也不满足事务要求：`App.tsx` 先更新本地 Composer profile，再异步调用后端；`useController` 吞掉后端错误；后端先更新 tab/controller，再做不返回错误的 tab 快照保存。持久化失败时 UI 与运行时可能显示成功。

基线：`origin/main-v3` `0f64078be0a02828e1155719405c751c4b311a26`（2026-08-12 fetch 后）。本环境未提供 Codegraph 工具；以下结论均由该 worktree 源码、历史和测试直接复核。

## 已确认合同

- 底层 permission rules 全局行为不变。
- 设置页 Ask/Auto/YOLO 只决定普通新会话的全局默认。
- 桌面交互式 Composer 姿态按会话持久化；Composer 修改不再更新全局默认。
- legacy sidecar 缺少字段时安全显示并运行 Ask；只有用户修改后才写入显式值。
- 必须区分 legacy missing 与显式 Ask。
- fork/检查点派生会话继承父会话姿态，之后独立修改。
- 姿态修改先持久化成功，再更新 controller、tab 快照和 UI；失败时保持旧值并提示。
- 覆盖桌面全局/项目交互式会话、历史恢复、重启、fork/检查点。
- bot、IM、automation 的独立审批合同不在本修复范围。

## 目标

1. 让每个已绑定 session path 的桌面 Composer 以现有 `<session>.meta` 为审批姿态唯一持久化来源。
2. 普通新会话仍从设置默认初始化；首次形成 session sidecar 时保存该显式初始值。
3. 保证会话切换、tab 复用、单表面、历史、重启和 fork 都恢复正确姿态。
4. 让修改操作具备可观察的事务语义和竞态保护。

## 非目标

- 不修改 `internal/permission`、工具风险分类或审批规则。
- 不改变 CLI、bot/IM、automation 的姿态来源与持久化。
- 不新增数据库、独立 JSON 文件或平行 per-session 存储。
- 不把 collaboration mode、token mode、goal 一并迁移。
- 不回填所有 legacy sidecar；缺失保持缺失，直到用户实际修改或新会话首次持久化。

## 当前证据与根因

- `desktop/frontend/src/App.tsx` 的 `applyToolApprovalMode` 先 patch 当前 profile，同时调用 `toolApprovalDefaultPreference.remember` 和 `SetToolApprovalModeForTab`。
- `desktop/frontend/src/features/session/useController.ts` 捕获并忽略 setter 错误，随后继续刷新。
- `desktop/tabs.go` 的 `WorkspaceTab`/`desktopTabEntry` 按 UI tab 保存姿态；单表面会复用 tab，因此 tab 不是会话身份。
- `internal/agent/branch.go` 的 `BranchMeta` 已保存 Model/BillingSource/Effort，`InheritComposerState` 已被 fork 路径复用，但无审批字段。
- `desktop/session_composer_state.go` 已集中加载/应用会话 Composer 身份，是 additive 扩展点。
- `desktop/tabs.go:saveTabSessionMeta` 已是 sidecar 写入口；setter 当前没有先持久化再提交运行时的能力。
- controller ready 水合、模型重建和发送前同步复用了持久化 setter，会把 legacy missing 的安全 Ask 静默回填；这些内部同步必须与用户修改的持久化命令分离。
- 历史 `c80ba8ad` 引入审批模式，`6ba025d2` 建立会话 Composer 身份视图，`d3184ac6` 增加新会话默认偏好写入与等待；这些变更组合后导致“会话值”和“新会话默认”混在一起。
- 委托方提供的实际 dev 证据为：全局默认 YOLO、当前 tab Ask、session meta 无审批字段，证明现有三份状态可分叉。

## 方案

### 持久化合同

- 在 `BranchMeta` 增加 additive 指针字段 `ToolApprovalMode *string`，JSON key 使用现有命名风格；`nil` 表示 legacy missing，非 nil 的 `ask|auto|yolo` 表示显式值。
- `sessionComposerState` 同时携带姿态及 `approvalKnown`。无字段或非法历史值按 Ask 运行，但不自动写回。
- 扩展 `InheritComposerState`，深拷贝姿态指针；现有 fork/branch/checkpoint 派生路径自然继承。
- `saveTabSessionMeta` 保存 tab 的显式有效姿态。普通新会话从设置默认建 tab，首次创建/绑定 session sidecar 时落为该会话初始显式值。

### 事务更新

- `SetToolApprovalModeForTab` 返回 error。解析目标 tab 和 session path 后，在不持有 App 主锁的情况下先写 sidecar；写成功后重新验证 tab/session 身份仍相同，再提交 controller/tab/UI 可见状态与 tab 快照。
- 若目标仍是尚未形成 session path 的空白新会话，允许更新其待落地初始姿态；此时没有既有会话 sidecar，后续首次绑定时持久化。
- 持久化错误直接返回，controller、tab snapshot 和前端 profile 保持旧值。
- 快速连续切换由调用顺序/目标会话身份校验保护，旧请求不得覆盖已切换会话的 UI。

### 前端

- 移除 Composer 的 `toolApprovalDefaultPreference.remember`；设置页继续独占 `SetDefaultToolApprovalMode`。
- 将普通 Composer setter 改为传播错误，仅在后端成功后 patch profile；失败调用现有错误展示。
- 增加只同步 controller runtime 的内部入口，供启动水合、模型重建和发送前一致性修复使用；该入口不更新 tab 或 sidecar。
- 单表面项目 Composer：已准备/绑定 tab 时走同一持久化 setter，成功后 patch；尚未创建会话的草稿只更新其待创建 profile，创建时将值交给新会话 setter。
- YOLO toggle 复用同一异步提交路径；失败不改变 restore 状态或可见选中项。

## 影响面

- `internal/agent/branch.go` 及 branch 测试：additive sidecar 与继承。
- `desktop/session_composer_state.go`、`desktop/tabs.go`、`desktop/app.go` 及相关 Go 测试：加载、恢复、setter 事务、fork。
- `desktop/frontend/src/App.tsx`、session controller/bridge 类型、Composer callback 类型及前端契约测试：成功后提交和错误提示。
- 不触碰 bot、IM、automation 实现。

## 兼容、迁移与回滚

- JSON additive 字段兼容旧 sidecar；无批量迁移、无启动回写。
- 显式 Ask 必须序列化，不能用空字符串/`omitempty` 值语义代替指针存在性。
- 若回滚代码，新字段会被旧版本 JSON 解码忽略；不需要数据回滚。恢复旧行为只需回滚相关提交。

## 风险与控制

- sidecar 与 UI tab 快照短暂不一致：恢复必须优先 sidecar；tab 快照只作无 session path 的启动连续性。
- 写入期间切换/关闭/重绑 tab：提交前比较 tab 指针与规范化 session path。
- 设置默认与会话初始化竞态：保留“创建普通新会话前等待最新设置写入”，但 Composer 不再加入该写队列。
- 项目单表面草稿尚无 session：明确以 adapter profile 作为待创建值，不伪造 sidecar。
- controller 正在运行时变更审批姿态沿用现有 controller API，不改变 pending approval 的既有清理规则。

## 验收标准

完整场景矩阵见 `acceptance.md`。所有场景要求 UI、controller、`ListTabs/Meta` 与 session sidecar 一致；失败场景要求四者均保持旧值且用户看到错误。

## 自动化验证

- `go test ./internal/agent`
- 在 `desktop` module 运行 `go test .`（直接修改桌面主包且跨多条恢复/分叉路径，使用包级测试）
- 相关前端定向测试；最终所有前端改动执行 `pnpm build`。
- 最终运行仓库 common checks 与 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`。

## 应用验收

使用隔离 dev home/profile 启动独立桌面 dev 实例；优先内置浏览器控制，Wails native 场景若浏览器不可覆盖则使用 Computer Use。数据仅写隔离目录，不修改真实用户会话或设置。验收记录更新到 `acceptance.md` 与 `progress.md`。
