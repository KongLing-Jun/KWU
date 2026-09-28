# 飞书 Bot 跨端交互可靠性修复计划

## 1. 背景与问题

飞书 Bot 当前存在两个可稳定解释的交互缺陷：

1. 用户在桌面端打开飞书自动创建的只读会话并保持停留时，手机新发消息和 Agent 完整回复不会进入当前正文；切换到其他会话再切回后才出现。
2. Agent 在飞书发送 Ask 或审批选项卡片后，原用户从手机点击选项，Bot 可能回复“抱歉，您没有使用此 bot 的权限。”

期望 Bot 一轮处理完成并落盘后，桌面端当前打开的同一飞书会话自动显示该轮用户消息和完整回复；卡片继续校验真实点击者的可信用户标识，原请求者标识只用于定位等待回答的会话。

## 2. 已确认决策

- 会话正文采用“一轮完成后刷新”，不实现跨端 token 级流式同步。
- Bot turn 成功、失败或取消后，只要产生了已落盘内容，都应允许当前只读会话刷新。
- 当前正文位于底部时跟随新增内容；用户正在上翻阅读时保持阅读位置。
- 飞书卡片鉴权接受真实点击者回调中的 `open_id`、`union_id`、`user_id` 任一可信标识命中白名单。
- 缺少真实点击者标识的卡片回调失败关闭，不回退使用卡片请求者标识鉴权。
- 保留现有群聊协作语义：白名单内的其他真实点击者可以操作群聊卡片；未授权点击者必须拒绝。
- 不迁移白名单，不改变 session key、session mapping schema、群组白名单或 Bot 会话所有权模型。

## 3. 基线与调查证据

Codegraph 主工作区调查基线为 `6b3321449ecdbe567ab5fe454e410cba29ee2397`；Stage 1 worktree 已同步最新 `origin/main-v3` 的 `7dc126d6df4652688467b81e6d6d52d51cd4e92c`。两者之间对 `desktop/app.go`、`App.tsx` 的变更仅涉及专家资产和媒体生成，未改变本计划依赖的会话读取与刷新路径；以下物质结论已用 worktree 当前源码、测试和 Git diff 复核。

| 证据 | 当前行为 |
| --- | --- |
| `internal/bot/gateway.go` 的 `runTurn` / `notifySessionUpdated` | `RunTurn` 返回并发送 `TurnDone` 后触发 `OnSessionUpdated`，成功和错误路径都会通知 |
| `desktop/bot_runtime_app.go` 的 `desktopBotSessionRemembererWithRefresh` | session ready/updated 都更新 mapping 并调用 `emitProjectTreeChanged` |
| `desktop/frontend/src/App.tsx` 的 `refreshSessionSurfaces` | 只刷新项目树 revision、tab meta、历史弹窗和 IM 侧栏，没有刷新当前正文 |
| `App.tsx` 的飞书 session 打开路径 | 自动映射的 channel session 使用 `openChannelSession`，tab 被标记为 `readOnly: true` |
| `useController.ts` 的 `openChannelSession` / `switchTab` | 首次打开或切回时重新水合历史，因此切走再切回能看到外部落盘内容 |
| `desktop/app.go` 的 `HistoryForTab` | tab 已有 Controller 时返回 Controller 内存历史，不重新读取 Bot 更新后的源 session 文件 |
| `desktop/app.go` 的 `PreviewSession` | 校验 session 路径后直接读取磁盘，可获得 Bot 最新落盘内容且不重绑 Controller |
| `useController.ts` 的历史 revalidation | 已有请求代次、surface activation、身份 key、历史指纹和运行时尾部守卫，可丢弃过期响应并避免无变化更新 |
| `Transcript.tsx` / `sessionViewSnapshot.ts` | 已记录 pinned-bottom 或消息锚点；内容变化时 pinned 视图跟随，非 pinned 视图保持阅读位置 |
| `feishu.go` 的普通消息解析 | 用户标识优先级为 `open_id → union_id → user_id` |
| `feishu.go` 的卡片回调解析 | 点击者标识优先级为 `union_id → open_id → user_id`，同时丢弃其他可验证别名 |
| `gateway.go` 的 `checkAllowlist` | 卡片存在 `OperatorID` 时只校验这一个值；不命中即发送用户报告的权限文案 |
| 现有飞书和 Gateway 测试 | 分别覆盖卡片解析和单值点击者鉴权，但没有覆盖“普通消息已授权的同一用户携带多种 ID 点击卡片” |

调查期运行 `go test ./internal/bot/...` 全部通过，证明现有测试未锁定跨事件身份一致性，而不是当前实现已经满足场景。

### 3.1 当前只读会话持有旧内存快照

Bot Controller 与桌面只读 tab Controller 是两个运行时。Bot 完成后更新同一个 session 文件并发出通用项目树 revision，但前端的 revision 处理没有重新读取当前正文；桌面 `HistoryForTab` 又优先返回只读 tab Controller 的旧内存历史。切换会话会重新执行水合，所以切回时才出现最新内容。

### 3.2 真实点击者被压缩为不一致的单一 ID

同一个飞书用户在普通消息中可能被表示为 `open_id`，在卡片回调中却因不同优先级被表示为 `union_id`。白名单通常保存普通消息链路观察到的 ID；Gateway 只校验卡片链路选择出的单值，导致同一用户被误判。卡片 action 中携带的请求者 ID 能正确定位原会话，但不能安全地证明谁实际点击。

## 4. 目标与非目标

### 4.1 目标

- 通用 session revision 到达后，只对当前打开、只读、路径有效的 channel session 做磁盘历史 revalidation。
- 只在磁盘历史真实变化时更新正文，保持现有导航竞态和运行时尾部保护。
- 复用现有滚动锚点，实现“底部跟随、上翻保持”。
- 飞书适配器保留真实点击者的全部非空可信 ID，并稳定去重。
- Gateway 对真实点击者 ID 集合执行“任一命中”白名单检查，同时保持群组白名单检查。
- 卡片请求者继续决定 session 路由；真实点击者继续决定授权。
- 为两个失败路径增加自动化回归覆盖，并完成真实桌面/手机飞书应用验收。

### 4.2 非目标

- 不把 Bot 运行时事件流转发到桌面 tab，不同步 reasoning、token、工具进度、中间 Ask 或未完成落盘的手机消息。
- 不把 channel session 改为可编辑，不允许桌面只读 tab 提交、审批或回答 Bot Controller 的待处理问题。
- 不新增文件 watcher、常驻轮询、session 事件协议或新的持久化字段。
- 不改变普通桌面会话、自动化会话、QQ/微信适配器的身份规则。
- 不重新设计 Bot 设置页或自动发现、展示飞书用户 ID。

## 5. 用户场景与验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 桌面停留在飞书只读会话，手机发送消息 | Bot 一轮结束并落盘后，当前正文自动出现手机消息和完整回复，无需切换 |
| AC-02 | Agent turn 失败或被取消并留下持久化内容 | revision 后显示磁盘中实际存在的内容；不伪造未落盘回复 |
| AC-03 | 当前正文位于底部 | revalidation 后跟随到新增内容 |
| AC-04 | 用户上翻查看旧消息 | revalidation 后保持原消息锚点和偏移，不强制跳到底部 |
| AC-05 | revision 属于其他 session | 当前 channel 历史指纹不变，不改变正文或滚动位置 |
| AC-06 | revision 到达时用户快速切换 tab/session | 旧读取结果被代次、activation 或身份守卫丢弃，不能覆盖新目标 |
| AC-07 | 当前 tab 非只读 channel session | 不触发外部历史 revalidation，不改变普通会话行为 |
| AC-08 | 普通消息以 `open_id` 进入白名单用户 A | A 点击同时携带 `open_id/union_id` 的卡片可通过并回答原会话 |
| AC-09 | 白名单保存 A 的 `union_id` 或 `user_id` | A 的同一回调仍可通过，无需迁移配置 |
| AC-10 | 未授权 B 点击 A 的卡片 | 即使卡片携带已授权请求者 A，仍以 B 的真实 ID 集合拒绝 |
| AC-11 | 群聊中已授权 B 点击 A 的卡片 | 保留现有协作语义，B 可操作，回答路由到卡片关联的原会话 |
| AC-12 | 卡片回调没有任何真实点击者 ID | 操作失败关闭，不以请求者 ID 通过鉴权 |
| AC-13 | 普通文本 `/answer`、`/approve` | 继续按消息发送者及现有 session key/白名单规则工作 |
| AC-14 | 重启桌面应用 | 显示结果与运行中自动刷新后的磁盘事实一致 |

## 6. 选定方案

### 6.1 当前只读 channel 历史按 revision 重验证

- 保留现有 `OnSessionUpdated → emitProjectTreeChanged → onProjectTreeChanged` 链路，不新增 Bot 专用事件名。
- 在 `useController` 增加有界的 channel history revalidation 入口，由 `App.refreshSessionSurfaces` 在当前 tab 为只读且存在 session path 时调用。
- 使用现有 `PreviewSession(path)` 读取磁盘事实；不能使用当前 `HistoryForTab`，因为其 Controller 分支返回旧内存快照。
- 在请求发起时固定 tab ID、surface activation、规范化 session 身份和当前 items 引用；响应后逐项核对，目标改变或正文已被其他事件更新时丢弃。
- 使用现有 `prepareHistoryRevalidation` 和 `history_revalidated` reducer action；指纹无变化时不更新 React 状态。
- 沿用 `refreshInFlightRef/refreshQueuedRef` 合并连续 revision；不增加固定频率正文轮询。现有 10 秒可见窗口 revision catch-up 只作为 Wails 事件丢失时的轻量兜底。
- 沿用 Transcript 的 pinned/anchor 机制。实现若发现 revalidation 不能同时满足 AC-03/AC-04，必须补齐行为测试，不能用强制 `scrollToBottom` 规避。

### 6.2 飞书真实点击者多标识鉴权

- 扩展内部 `InboundMessage` 的点击者身份表达，使其能携带一个兼容主 ID和去重后的可信别名集合；其他平台保持单值行为。
- 飞书卡片回调从嵌套和直接 operator 字段收集 `open_id`、`union_id`、`user_id`，只接受飞书签名回调中的 operator 数据。
- 卡片 action 的 `user_id` 继续写入路由用户，保证找到原请求者正在等待的 session；不得加入真实点击者白名单候选。
- Gateway 在存在点击者身份集合时仅检查该集合是否任一命中平台用户白名单；没有点击者身份时拒绝。非卡片消息继续检查 `UserID`。
- 群组白名单仍在用户鉴权成功后按 `ChatID` 检查，规则不变。
- 日志继续只记录 hash/命中结果，不输出原始飞书标识或卡片载荷。

### 6.3 兼容与失败处理

- 既有只包含 `open_id`、`union_id` 或 `user_id` 的 `feishu_users` 均无需迁移。
- `AllowAll` 保持显式放行；缺失 operator 是否在 `AllowAll` 下放行必须遵守“真实点击者必需”的卡片边界，在 Feishu adapter 入队前失败关闭。
- revalidation 读取失败不清空当前正文、不切换 session，只记录可诊断信息并等待下一次 revision/focus catch-up。
- 若实现证明必须改变 Wails 方法、session schema、Bot 事件载荷或群聊授权模型，停止实施、更新本计划并重新审批。

## 7. 预计影响范围

| 范围 | 预计文件 |
| --- | --- |
| Bot 产品代码 | `internal/bot/types.go`、`internal/bot/gateway.go`、`internal/bot/feishu/feishu.go` |
| 前端产品代码 | `desktop/frontend/src/App.tsx`、`desktop/frontend/src/features/session/useController.ts`，必要时新增纯函数文件 |
| Bot 测试 | `internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go` |
| 前端测试 | `im-session-refresh-contract.test.ts` 及历史 revalidation/滚动行为测试 |

预期不修改 `desktop/app.go`：现有 `PreviewSession` 已提供安全的新鲜磁盘读取。若实施中需要修改 Go 桌面层，应先证明前端复用接口不足，并按受影响 Desktop package 扩展验证。

## 8. 风险与控制

| 风险 | 控制与验证 |
| --- | --- |
| 旧磁盘响应覆盖刚切换的新会话 | tab、activation、身份 key、请求代次四层守卫；快速切换 deferred 测试 |
| revision 频繁造成重复磁盘读 | 复用现有合并队列和 10 秒轻量 revision 兜底；仅当前只读 channel 读取；指纹无变化不更新 |
| revalidation 打断滚动阅读 | 使用现有 pinned/消息锚点；分别覆盖 AC-03、AC-04 |
| 当前 Controller 内存历史继续遮蔽磁盘 | revalidation 明确调用 `PreviewSession`，测试在打开后追加源文件再刷新 |
| 合并历史产生重复消息或丢失归档工具 | 复用现有历史投影、指纹和 `mergeRevalidatedHistoryItems`，覆盖完整一轮追加 |
| 接受卡片请求者导致越权 | 请求者永不进入 operator 候选；未授权 B 点击 A 卡片测试 |
| 只改 ID 优先级破坏另一类旧白名单 | 不选单一优先级，检查所有可信 operator 别名 |
| 空 operator 在 `AllowAll` 下绕过 | Feishu adapter 入队前失败关闭并测试 |
| 扩展 `InboundMessage` 影响其他平台 | 新字段可选；QQ/微信和普通消息继续走现有单值路径；运行 `go test ./internal/bot/...` |

## 9. 回滚

- 当前会话 revalidation、飞书 operator 别名和对应测试作为可独立识别的实现 slice 回滚。
- 回滚后恢复“切换会话后看到外部更新”和卡片单值鉴权的旧行为。
- 无数据迁移、配置重写或持久化 schema 需要回滚。

## 10. 验证计划

### 10.1 自动化验证

迭代期先运行失败优先的邻近测试：

- 打开只读 channel、外部追加 session 文件、触发 revalidation 后正文更新。
- 相同指纹不更新；切换 tab/session 的迟到响应被丢弃。
- pinned-bottom 与非 pinned anchor 两类滚动行为。
- Feishu callback 同时提供多种 ID 时完整保留并去重。
- `open_id`、`union_id`、`user_id` 任一白名单命中通过。
- 未授权点击者、缺失点击者拒绝；请求者不能替代点击者。
- 已授权群聊协作者和普通文本命令保持兼容。

应用行为稳定后至少运行：

```powershell
go test ./internal/bot/...
cd desktop/frontend
pnpm build
```

前端行为测试按实际新增文件运行；不因前端变化运行无关 Go 全量测试。若实际修改 Desktop Go，则从 `desktop` module 运行受影响 package 测试。

### 10.2 应用验收

1. 优先用内置浏览器控制前端开发 fixture，验证 revision 后正文替换及两类滚动位置。
2. 使用 Computer Use 控制实际 Wails 桌面应用，打开飞书自动会话并保持停留。
3. 真实飞书凭据和手机点击需要用户参与时，按以下步骤验收：
   - 手机发送可辨识消息，桌面不切换会话，确认一轮完成后用户消息和完整回复自动出现；
   - 分别在底部和上翻状态重复，观察跟随与锚点保持；
   - 触发 Ask，手机点击选项，确认不再出现权限错误且 Agent 继续；
   - 如可用，在群聊让未授权用户与另一白名单用户分别点击，确认拒绝/允许边界。
4. 实现者自测不能替代最终应用验收；只有真实凭据场景才交给用户执行。

### 10.3 PR 前门禁

仅在获得 PR 授权、同步最新 `origin/main-v3`、完成应用验收后执行：

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code origin/main-v3...HEAD -- scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

任何 required change-related check 失败时不得创建或更新 PR。
