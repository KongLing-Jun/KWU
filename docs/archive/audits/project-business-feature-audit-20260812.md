# 项目全业务功能与 WorkBuddy 复刻审查（2026-08-12）

## 结论

当前“项目”页面已经形成完整、可操作的业务表面：项目目录、模板创建、动态、五种计划视图、自动化任务、资产、成员邀请和项目配置均可到达；宽屏和移动端的视觉结构基本复刻了用户提供的 WorkBuddy 参考图，并继续使用现有 Desktop 设计 token 和组件风格。

但本轮不能判定为“可发布”：确认 **8 个 P1、11 个 P2、2 个 P3** 问题。最高风险集中在异步请求跨项目/筛选污染、非看板视图静默漏数据、多步写入部分成功以及任务授权边界。现有定向测试全部通过，但没有覆盖这些失败时序和完整浏览器业务链。

本轮是审查，不修改产品代码、不创建或删除业务数据。仅新增本报告和本地浏览器审查截图。

## 审查对象与方法

| 项目 | 当前证据 |
|---|---|
| Desktop 工作树 | `D:\project\Reasonix-worktrees\integration-main-v3-20260811` |
| 分支 / HEAD | `codex/integration-main-v3-20260811` / `eb354e3ac3e74af795b066b515f801aa6f3ad8c4` |
| Desktop Wails | `127.0.0.1:34238`，PID 53892，目标工作树二进制 |
| Desktop Vite | `127.0.0.1:5175`，PID 47812 |
| Cloud | `127.0.0.1:9090`，PID 47352，`reasonix-cloud-8e1f3c3c8d74.exe` |
| WorkBuddy | `D:\Programs\WorkBuddy\WorkBuddy.exe`，5.3.5，进程和窗口均在运行 |
| 浏览器 | 用户指定的 Chrome；另用 Playwright CLI 独立 headed 会话复核 |
| 视口 | `1653×657`、`1247×800`、`390×844` |

采用四类互相独立的只读审查：正确性/边界、测试/失败模式、架构/可维护性、性能/并发/资源。主审查随后逐项回读源码，删除重复项和仅凭猜测的意见。

WorkBuddy 原生窗口能够被唯一识别，但 Windows Computer Use 在读取窗口状态时持续返回 `node_repl exec context not found`，因此没有把无法取得的原生实时截图伪装成已验证结果。视觉比较以用户提供的 WorkBuddy 参考图为基准，并用 Chrome 与 Playwright CLI 对当前 Desktop 真实页面进行逐项点击和截图。

## 浏览器逐项覆盖

| 业务面 | 实际操作 | 结果 |
|---|---|---|
| 项目目录 | 打开项目、新建弹窗、5 个模板、搜索、刷新、项目菜单 | 入口和弹窗均可达；模板会预填长指令并保留连接器/专家/技能选择 |
| 动态 | 展开/取消留言、输入 `@`、键盘关闭建议、附件入口、筛选、刷新 | 主交互可达；取消后草稿保留；未提交留言或附件 |
| 计划 | 全部、看板、表格、列表、甘特、日历、自定义视图、筛选、搜索、标签设置、添加、编辑、键盘移动 | 五视图及弹窗均可达；未执行会写 Cloud 的创建、移动或删除 |
| 任务 | 任务筛选、触发来源、搜索、空状态 | 真实项目当前没有运行记录，因此详情和“加载更多”只能由代码/测试审查 |
| 资产 | 新建文件夹弹窗、类型、搜索、排序表头、上传入口 | 主交互可达；未选择文件、未创建目录；空目录下无法验证行菜单/批量操作 |
| 配置 | 展开/收起、Escape、指令、连接器、专家、技能、自动化弹窗 | 收起后 DOM 不残留右轨；移动端为覆盖抽屉，Escape 可关闭 |
| 成员与项目 | 邀请弹窗、项目编辑/归档/删除菜单 | 均可达；全部取消，未写业务数据 |
| 响应式 | 宽屏四列、移动端横向看板、移动端资产空态 | 工具栏 `overflow-x:auto`，四列语义保留；资产移动端切换为紧凑布局 |

浏览器证据位于 `output/playwright/project-business-audit-20260812/`。关键截图：

- `directory-1247x800.png`
- `activity-1247x800.png`
- `plan-board-expanded-1247x800.png`
- `plan-board-collapsed-1247x800.png`
- `plan-board-1653x657.png`
- `plan-board-390x844.png`
- `tasks-1247x800.png`
- `assets-1247x800.png`
- `assets-390x844.png`

## WorkBuddy 视觉与交互复刻判断

| 参考面 | 判断 | 说明 |
|---|---|---|
| 项目目录 | 基本达标 | 标题、副标题、新建、项目卡、搜索、模板网格的层级与参考一致；遵循当前暗色主题，不复制第二套样式系统 |
| 动态留言 | 基本达标 | 单一宽版编辑表面、左下附件、右下取消/发布、活动行密度与参考一致 |
| 计划工具栏 | 达标 | 左侧“全部 + 五视图 + 自定义”，右侧筛选/搜索/设置/添加；窄屏横向滚动而非杂乱折行 |
| 四列看板 | 基本达标 | 宽屏四列等高、独立列背景、紧凑卡片；移动端保持横向四列语义；底部会话框未遮挡主要内容 |
| 自动化任务 | 结构达标、数据态未充分验证 | 筛选、隐私文案、搜索、空态与参考相符；当前真实项目没有运行记录，未能实测行菜单和详情 |
| 资产 | 结构达标、数据态未充分验证 | 顶部操作/额度/筛选/搜索/表格和移动端空态符合参考；真实目录为空，未能视觉核对文件行和批量条 |
| 配置抽屉 | 基本达标 | 宽屏与主工作区共用视觉表面；移动端覆盖式抽屉支持 Escape。参考中的能力卡结构已保留 |

“基本达标”仅表示当前布局、层级和主交互接近参考，不表示错误处理、竞态、权限或大数据量已经正确。

## P1：发布阻断问题

### P1-1 非看板四视图静默遗漏第 50 项之后的数据

- `WorkItemBoard.tsx:173-189` 每个状态只读取 50 条。
- `WorkItemBoard.tsx:514-539` 只有看板列提供“加载更多”；表格、列表、甘特、日历直接使用当前已加载的合并数组。
- 任一状态超过 50 条时，四个视图表现得像完整视图但永久不可达剩余数据；甘特范围和日历事件也会错误。
- 现有 `project-work-item-ui.vitest.test.tsx:95-110` 反而锁定了“切换视图不重新请求”。

### P1-2 看板旧分页响应可污染新的搜索/筛选结果

- 首屏在 `WorkItemBoard.tsx:173-201` 使用 `requestRef`。
- 增量读取在 `WorkItemBoard.tsx:203-214` 只校验项目身份，不校验请求代次和完整筛选快照。
- 用户点击“加载更多”后修改搜索、负责人、优先级、标签或自定义视图，旧页会追加到新条件，并可写入旧游标、总数或错误。

### P1-3 动态读取、已读回写和发布缺少 latest-request-wins

- `ProjectActivityPanel.tsx:87-104` 无 generation/abort；同项目不同筛选的旧结果都能通过。
- `ProjectActivityPanel.tsx:95-99` 的旧已读回写可把当前列表全部改为已读，旧 `finally` 还会提前解除新请求的 pending。
- `ProjectActivityPanel.tsx:127-135` 的迟到发布可把旧项目消息插入新页面。

### P1-4 资产 mutation 完成后可把页面刷新回旧项目或旧目录

- 普通目录读取在 `ProjectAssetsPanel.tsx:112-144` 有 generation。
- 上传和其他写操作在 `ProjectAssetsPanel.tsx:168-175,210-217` 完成后调用旧 render 闭包的 `load(false)`。
- 写入期间切项目或进入其他目录后，旧闭包会递增共享 generation、使新读取过期，并把旧项目/目录写回当前页面。

### P1-5 项目目录和详情的旧失败分支可覆盖较新的成功结果

- 目录成功路径在 `CloudProjectsSectionImpl.tsx:223` 检查 request id；`240-249` 的 catch/finally 只校验账号 scope。
- 详情中的自动化/成员后置请求在 `CloudProjectsSectionImpl.tsx:405-433` 成功时校验选中项目，失败时却直接清空或写入“详情加载失败”。
- 结果是项目 B 成功显示后，项目 A 的迟到失败仍可清空 B 的数据或写错误状态。

### P1-6 自动化任务存在项目授权空洞和详情跨项目串页

- `project_task_app.go:53-76` 在列表入口没有先验证目标 CloudProject。
- 授权只在 `service_run_helpers.go:46-65,90-107` 遍历实际 run 时执行；无本地 run 的不存在/无权项目会返回成功空列表。
- 前端详情在 `ProjectTaskPanel.tsx:79-85` 直接调用不带 `projectId/projectScopeKey` 的通用 `automationReadRun`，也无详情 request id；切项目或快速点击 A/B 后可显示旧详情。

### P1-7 新建计划项的第二阶段失败会导致重试重复创建

- `WorkItemBoard.tsx:306` 先创建计划项。
- `311` 以后再逐个绑定标签和资产。
- 后续任一步失败只进入通用 catch，编辑器仍保持“新建”；用户重试会再次 create，产生重复项。

### P1-8 自动化能力保存可能先删除旧配置，再部分失败

- `ProjectCapabilityDialog.tsx:238-243` 先删除取消选择的日程。
- `244-263` 才新增定义和链接。
- `281-285` 失败时只显示错误，无权威回读、补偿或恢复，造成不可恢复的部分保存。

## P2：中等风险问题

1. **动态只显示最近 100 条。** `ProjectActivityPanel.tsx:87-100` 固定请求 100 条，却完全忽略公开响应中的 `nextCursor`（`projectTypes.ts:258`）。
2. **提及 ID 与正文脱节。** `ProjectActivityPanel.tsx:140-149` 只增加 `mentionUserIds`；用户删除已经插入的 `@成员` 文本后仍会通知该成员。
3. **任务筛选不随项目重置。** `ProjectTaskPanel.tsx:65-72` 没有清空 `taskId/trigger/search/detail`，旧项目任务 ID 会继续筛选新项目。
4. **文件夹删除的版本承诺被桥接层丢弃。** 类型含 `expectedVersion`，但 `projectGateway.ts:132` 只传 `projectId/folderId`；无法检测旧版本删除。
5. **资产冲突提示与行为不一致。** `ProjectAssetsPanel.tsx:475` 声称“列表已刷新”，mutation catch 在 `210-217` 实际没有刷新。
6. **资产额度被 Desktop 推断。** `ProjectAssetsPanel.tsx:120-134` 在目录 API 缺失或 quota 为零时硬替换为 Lite/5GB，可能伪造并不存在的套餐事实。
7. **自动化运行分页每次两遍全量归档扫描。** `service_run_helpers.go:14-77` 两次 `loadRuns`；`archive.go:233-306` 每次读所有 manifest 并全量排序，底层分页并不减少 I/O。
8. **资产处理状态为全项目 O(页大小×状态数)。** `ProjectAssetsPanel.tsx:87-99,292-308` 每次刷新全量读取，并为每行重复 filter/sort；应一次构建 `assetId -> latestState`。
9. **计划和动态附件旁路目录分页。** `WorkItemBoard.tsx:240-248` 和 `ProjectActivityPanel.tsx:81-85` 读取无分页的全项目扁平资产列表，并把全部选项渲染到选择器。
10. **100 MiB 上传至少形成两份完整内存副本。** `project_asset_app.go:309-322` 使用 `os.ReadFile`，`asset_gateway.go:399-417` 又写入完整 multipart buffer；单次接近 200 MiB，且无跨入口并发限制。
11. **Wails 会话签名漂移被双重断言掩盖。** Go 接收 `{projectId,tabId}`（`project_session_app.go:15-18,41`），`bridge.ts:448` 仍声明字符串，`projectGateway.ts:125` 以 `as unknown as` 绕过类型系统。

## P3：低风险/可维护性问题

1. **能力弹窗绕过 ProjectGateway。** `ProjectCapabilityDialog.tsx:130-150,238-263` 直接调用全局 `app`，在视图内用 `any` 解析 camelCase/PascalCase 并编排本地任务、Cloud 定义和调度；导致 mock、契约验证和失败恢复分裂。
2. **项目模态框键盘语义不一致。** 指令弹窗有 Escape、焦点锁定和焦点返回，任务详情在 `ProjectTaskPanel.tsx:107` 只借用看板 CSS，没有同等焦点管理；应复用统一 Dialog foundation。

## 测试与失败模式审查

已执行且通过：

- 6 个项目相关 Vitest 文件：**59/59**。
- `internal/automation` 定向 Go 测试。
- Desktop 项目定向 Go 测试。
- Chrome 当前登录态全业务只读点击。
- Playwright CLI headed 会话：项目入口、真实项目、390px 响应式快照。

这些通过结果不覆盖：

- deferred Promise 形式的跨项目、跨筛选乱序；
- 创建成功后标签/资产绑定失败；
- 自动化删除成功后新增/链接失败；
- 资产 CRUD、批量部分成功、quota/conflict 的完整前端点击链；
- 51+ 计划项的五视图可达性；
- 自动化运行详情的 A/B 逆序返回；
- 真实项目 390px 下逐页签 E2E。

现有 E2E 还存在契约漂移：`workbuddy-project.spec.ts` 保留已经删除的 workspace binding 场景；`workbuddy-project-webview2-probe.mjs:328` 仍发送旧 `state/source` 任务字段；390px 视觉 spec 没有进入四个业务页签。因此不能用现有 E2E 声称“每个按钮、事件、组件已验证”。

## 排除项

- 未发现 `CloudProject == LocalWorkspace` 的边界破坏；公开本地绑定 Wails 操作已经移除，内部只读兼容仓储仍保留。
- 配置收起使用条件渲染，DOM 不保留旧的 48px 空轨；Chrome 实测通过。
- 计划首屏四列请求本身有 request generation；问题限定于增量分页和辅助请求。
- 资产主目录读取本身有 generation；问题限定于迟到 mutation 触发的旧闭包刷新。
- 自动化游标已绑定 scope、task、project、trigger、search、limit，未发现跨筛选游标复用。
- 项目组件的全局事件监听均有 cleanup；没有把“组件很大”或“CSS 很长”本身列为缺陷。
- 暗色主题与 WorkBuddy 参考图的浅色主题差异来自现有产品主题，不构成第二套设计系统或复刻失败。

## 建议修复顺序

1. 统一所有 Project UI 请求身份：`accountScope + projectScope + projectId + query snapshot + generation`，成功、失败和 finally 全部同样校验。
2. 解决非看板视图分页，再补 51+ 项回归测试。
3. 为任务列表入口增加项目授权；任务详情通过 ProjectGateway 携带项目身份并独立隔离请求。
4. 把计划创建和自动化能力配置改成可恢复的分阶段状态机，或提供原子后端操作。
5. 修复资产 mutation 的旧闭包刷新、删除版本条件和真实 conflict 回读。
6. 给动态增加游标分页，并使 mention ID 从当前结构化 token 重建。
7. 优化自动化归档索引、资产处理状态 Map 和流式上传。
8. 更新浏览器 fixture、WebView2 probe 和移动端 E2E，使其覆盖当前真实契约。

在 P1 修复并新增对应时序测试前，不建议把当前“项目”业务判定为发布就绪。
