# 任务 002：修复 Composer 会话身份闪烁

## 状态与交付

- 状态：完成；时间线诊断、版本化原子交接和真实原生验收均已完成。
- 执行组：Serial-02；不委派子代理。
- 依赖：任务 001 已完成；本任务沿用同一分支并以新的 `fix:` 提交交付。
- 提交策略：计划/任务、后端只读视图契约、前端原子呈现按可独立审查意图小步
  提交；不 amend、squash 或重写任务 001 的提交。

## 目标

点击已有会话后，从第一个目标切换帧开始，模型/计费来源、专家/专家团、协作
方式、工具审批、Token 模式和 Goal 必须属于目标会话，不能显示源会话、空专家
或非目标默认值。

## 根因与源码依据

- 历史 `SessionMeta` 和项目树 `ProjectNode` 不携带模型/计费等目标呈现身份。
- 冷 `beginSurfaceTransition` 使用无 meta 的 `initialState`；App 因此读取默认
  Composer profile，等待 `ActivateTopic` 后才得到目标 `TabMeta`。
- App 切换期显式传 `activeExpert={null}`，目标专家只能等异步
  `CurrentExpertForTab` 再恢复。
- BranchMeta 已持久化模型、计费和推理强度；SessionInfo 已提供专家 ID/名称，
  不需要新增会话存储。
- 协作、审批、Token 和 Goal 的非默认状态属于存活 runtime/内存快照；关闭会话
  当前会按安全默认值重开，视觉修复不得改变该权限语义。

## 允许范围

- `desktop/app.go`、`desktop/tabs.go` 的内部历史/项目树只读视图字段及相邻测试。
- `desktop/frontend/src/lib/types.ts`、ProjectTree 打开请求、App、session controller、
  snapshot 和直接相邻前端测试。
- 本计划目录中的计划、进度和任务回执。

## 禁止范围

- 数据库迁移、新依赖、模型 fallback、凭据、订阅和发送门禁。
- 将工具审批、协作、Token 或 Goal 新增长期写入 BranchMeta。
- 改变专家激活、模型切换或权限设置 API 的写入行为。
- 无关 UI 重构、完整 DOM 缓存或 `scripts/code_guard_baseline.json`。

## 实现约束

1. 模型/计费和专家复用现有 sidecar/SessionInfo；列表不得读取完整 transcript。
2. 存活 runtime 可覆盖静态身份；关闭会话使用后端当前重开默认值。
3. transition target 明确区分“已知无专家”和“专家身份未知”。
4. 第一目标帧直接由目标 identity 派生；后端激活只校正，不能先回退源身份。
5. 专家目录无法解析目标 ID 时显示无专家，不得显示源专家。
6. 旧可选字段缺失保持兼容；无数据迁移。

## 验收场景

1. 模型 A → 模型 B，按钮连续呈现值中不存在 A、空值或默认模型。
2. 专家团 A → 专家团 B、专家团 → 无专家、无专家 → 专家团均无中间闪烁。
3. `normal`、`plan`、`goal` 会话之间切换不经过错误协作方式。
4. 活 runtime 的审批/Token/Goal 首帧正确；关闭会话继续采用 ask/full/无 Goal
   等当前安全默认值，不恢复旧完全访问权限。
5. 快速切换 9 个真实会话 100 次无 renderer 退出、白屏或新 Crashpad。
6. 任务 001 的草稿、历史缓存、单 Transcript 和虚拟化验收继续通过。

## 聚焦验证

```powershell
Push-Location desktop
go test . -run 'Test(ListSessions|ListProjectTree).*Composer'
Pop-Location
Push-Location desktop/frontend
pnpm exec tsx src/__tests__/composer-session-identity-transition.test.tsx
pnpm exec tsx src/__tests__/tab-switch-hydration.test.tsx
pnpm exec tsx src/__tests__/app-chrome-tabs.test.ts
pnpm exec playwright test --config playwright.config.ts
pnpm build
Pop-Location
go run ./scripts/code_guard
git diff --check origin/main-v3...HEAD
```

最终按实际测试名称记录结果；Go 只运行直接受影响的 Desktop package 和必要相邻
测试，不运行无关 Root Go 全量测试。

## 失效的先前完成回执

- 状态：重新打开。已有实现和自动化结果保留，但“连续呈现无闪烁”的原生验收
  结论无效，不再作为完成证据。
- 重新打开原因：用户肉眼观察到点击已有会话时专家入口明显闪烁两三次；此前
  60/80ms 外部抽样没有覆盖每次 React commit/paint。
- 后端交付：新增 Composer identity 只读视图，将模型/计费与专家 sidecar 信息
  暴露给 `SessionMeta` 和 `ProjectNode`；存活 runtime 覆盖静态值，关闭会话不
  持久化审批、协作、Token 或 Goal 权限状态。
- 前端交付：新增目标 identity 投影，项目树、历史中心、路径恢复和打开请求在
  后端激活前同步建立目标 Meta；transition 不再沿用源 profile，也不再显式清空
  后再恢复目标专家。
- 主要变更文件：
  - `desktop/composer_identity_view.go`
  - `desktop/app.go`
  - `desktop/tabs.go`
  - `desktop/session_composer_identity_view_test.go`
  - `desktop/frontend/src/lib/composerSessionIdentity.ts`
  - `desktop/frontend/src/lib/types.ts`
  - `desktop/frontend/src/lib/openTopicCoalescing.ts`
  - `desktop/frontend/src/features/session/sessionMeta.ts`
  - `desktop/frontend/src/features/session/useController.ts`
  - `desktop/frontend/src/features/workspace/ui/ProjectTree.tsx`
  - `desktop/frontend/src/App.tsx`
  - 三个直接相邻的前端测试文件。
- 聚焦验证：
  - `go test . -run '^(TestListSessionsExposesPersistedComposerIdentity|TestListProjectTreeExposesStaticAndRuntimeComposerIdentity|TestListProjectTree)'`：通过。
  - 项目树即时导航 identity 用例：通过。
  - `tab-switch-hydration.test.tsx`：136/136 通过，包含模型、协作、审批、Token、
    Goal 和专家团六个第一帧断言。
  - `app-chrome-tabs.test.ts`：89/89 通过。
  - Playwright：10/10 通过；`pnpm build`：通过。
  - `go run ./scripts/code_guard` 与 `git diff --check origin/main-v3...HEAD`：通过。
- 原生验收：使用 dev profile 的真实历史数据，在“你好，无专家模式”“你好4”
  “当前是v4”“fix-accept-02 reply received”之间切换。加载期间“你好4”直接呈现
  会计核算专家与目标 gpt 模型，切回无专家会话时专家入口直接消失。以 80ms
  观察间隔发送 100 次交替切换请求，无工具操作错误，最终停在最新目标“你好4”
  且 identity 完整；`kwu-desktop-dev` PID 13172 响应正常，日志无崩溃关键词。
- 偏差：没有另建 `composer-session-identity-transition` 测试文件，第一帧契约并入
  hydration 测试，但该测试没有覆盖 App 在 transition 结束后切回异步
  `activeExpert` 的连续提交，因此不足以证明无闪烁。
- 剩余风险：缺少 `.expert.json` 的旧会话没有可供列表首帧使用的专家身份；已从
  当前账号模型列表移除的旧模型最终仍按既有逻辑显示“请选择模型”。两者均不再
  借用源会话身份，也未通过读取完整 transcript 或放宽权限语义来掩盖。

## 重新诊断协议

1. dev-only 锚点以项目树 click 为 `t=0`，生成单次 navigation trace ID。
2. 记录 transition begin/update/finish、active tab 变化、专家 refresh 的
   start/resolve/apply/ignored 和每次 App commit 的实际 displayed expert。
3. 每次 commit 后用 `requestAnimationFrame` 记录 paint 边界；记录专家 ID、名称、
   数据来源、refresh sequence、transition sequence 和相对毫秒。
4. 锚点只写有界内存 ring buffer 和开发控制台，不参与状态计算；生产构建无输出。
5. 在专家、无专家和不同专家会话间至少往返 5 轮，导出每轮完整时间线；确认闪烁
   次数和每次错误值的写入者后，才讨论修复方案。

## 重新诊断结果

- 5 轮均复现 `目标 → 错误中间值 → 目标`，且错误状态跨过真实 paint。
- 专家 → 无专家：旧专家回闪约 134–196ms。
- 无专家 → 专家：目标专家先出现，随后消失约 1.32–1.44s，再恢复。
- 根因是 transition identity 与异步 `activeExpert` 没有有版本的 handoff；缓存
  transition 先完成，源 tab refresh 和目标 runtime 恢复前后的多次 refresh 继续
  写独立状态。详细 trace 和写入链路见 `../evidence.md`。
- 修复前门槛：当时等待用户确认修复策略，尚未开始修复。

## 最终完成回执

- 采用版本化原子交接：`renderedViewTarget` 在 transition 完成后继续持有专家
  展示权，直到 `activeExpertResolution` 同时匹配目标 `viewKey`、目标 tab 且为
  authoritative；没有使用固定 loading 时间或隐藏专家入口。
- `useExpertCatalog` 对 refresh sequence、active tab 和 active view generation
  三重校验，并只在目标 runtime ready 后发布权威 resolution。
- 删除历史、消息通道和会话中心打开完成后通过旧闭包主动调用的专家刷新，避免
  查询源 tab；目标专家刷新由 hook 的 tab/view/ready 依赖唯一驱动。
- 新增 handoff 单元契约，覆盖 transition、源 tab、旧 view、provisional 结果、
  权威目标和普通 tab 六种所有权情况；同步更新 App 与专家源码契约。
- 原生验收在“你好，无专家模式”和“你好4（会计核算专家）”之间交替切换 5 次。
  五条 trace 的 commit/paint 唯一值都等于目标：三次专家目标始终为
  `exp_accounting`，两次无专家目标始终为 `<none>`。权威交接前后只改变来源，
  不改变显示值；详细毫秒时间线见 `../evidence.md`。
- 自动化通过：handoff 单元契约、专家契约 44/44、项目树导航、切换 hydration
  136/136、App chrome 89/89、Playwright 10/10、生产构建、`code_guard` 和
  `git diff --check origin/main-v3...HEAD`。
- 原生 dev 进程验收后仍响应，日志无 panic、fatal 或 renderer crash；应用保持
  运行，供用户继续复验历史数据。
- 无偏离：未新增依赖、迁移、权限持久化或模型 fallback，未扩大到专家
  AgentWorkspace 虚拟化，也未推送或创建 PR。
