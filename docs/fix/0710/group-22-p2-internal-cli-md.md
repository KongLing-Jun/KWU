# G22 · internal/cli · md 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G22 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli/md.go`<br>`internal/cli/bot.go`<br>`internal/cli/mcp.go`<br>`internal/cli/memory_view.go`<br>`internal/cli/chooser.go`<br>`internal/cli/diffview.go` |
| 主要治理目标 | 围绕 `internal/cli/md.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；涉及核心运行链路 |
| 建议处理顺序 | 总序第 22 组；等待 G03 的同文件变更合并 |
| 前置依赖 | G03（internal/cli/bot.go） |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **47/100**，对象 `*mdRenderer.renderTable` 的分项为严重度 10.2、集中度 5.3、影响面 15.8、可执行性 16.0；最高超限 2.30x，文件内 4 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。

### 治理边界与组间关系

本组涉及 6 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G03。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-152` | `complexity\|internal/cli/md.go\|*mdRenderer.renderTable` | 圈复杂度 | `internal/cli/md.go` | `*mdRenderer.renderTable` | 365 | 23/10 | +13（2.30x） | P2 | 0 可信 + 1 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-103` | `complexity\|internal/cli/bot.go\|botStart` | 圈复杂度 | `internal/cli/bot.go` | `botStart` | 47 | 12/10 | +2（1.20x） | P2 | 1 可信 + 0 待核 caller / 16 可信 + 1 待核 callee / 25 unresolved | 待修改 |
| `FN-016` | `func_lines\|internal/cli/bot.go\|botStart` | 函数有效行数 | `internal/cli/bot.go` | `botStart` | 47 | 88/80 | +8（1.10x） | P2 | 1 可信 + 0 待核 caller / 16 可信 + 1 待核 callee / 25 unresolved | 待修改 |
| `CX-148` | `complexity\|internal/cli/mcp.go\|parseMCPAdd` | 圈复杂度 | `internal/cli/mcp.go` | `parseMCPAdd` | 26 | 27/10 | +17（2.70x） | P2 | 2 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 22 unresolved | 待修改 |
| `CX-149` | `complexity\|internal/cli/md.go\|fixCJKEmphasis` | 圈复杂度 | `internal/cli/md.go` | `fixCJKEmphasis` | 80 | 18/10 | +8（1.80x） | P2 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-154` | `complexity\|internal/cli/memory_view.go\|renderMemory` | 圈复杂度 | `internal/cli/memory_view.go` | `renderMemory` | 11 | 17/10 | +7（1.70x） | P2 | 1 可信 + 0 待核 caller / 8 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-119` | `complexity\|internal/cli/chooser.go\|chatTUI.handleChooserKey` | 圈复杂度 | `internal/cli/chooser.go` | `chatTUI.handleChooserKey` | 89 | 22/10 | +12（2.20x） | P2 | 0 可信 + 1 待核 caller / 0 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `CX-133` | `complexity\|internal/cli/diffview.go\|diffBody` | 圈复杂度 | `internal/cli/diffview.go` | `diffBody` | 77 | 16/10 | +6（1.60x） | P2 | 1 可信 + 0 待核 caller / 8 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-151` | `complexity\|internal/cli/md.go\|*mdRenderer.appendInline` | 圈复杂度 | `internal/cli/md.go` | `*mdRenderer.appendInline` | 316 | 13/10 | +3（1.30x） | P3 | 0 可信 + 2 待核 caller / 4 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-150` | `complexity\|internal/cli/md.go\|isCJKPunct` | 圈复杂度 | `internal/cli/md.go` | `isCJKPunct` | 140 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-152 · 圈复杂度 · `*mdRenderer.renderTable`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-152 |
| Baseline key | `complexity\|internal/cli/md.go\|*mdRenderer.renderTable` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 47） |
| 文件 | `internal/cli/md.go` |
| 代码行 | 365 |
| 函数/方法/包 | `*mdRenderer.renderTable` |
| Baseline / 当前复测 / 阈值 | 23 / 23 / 10 |
| 超出 | +13，2.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*mdRenderer.renderTable` 的圈复杂度当前为 **23**，超过限制 **10**（+13，2.30x）。定义范围为 365-446 行；可见控制流热点：L369 `for c := n.FirstChild(); c != nil; c = c.NextSibling() {`；L377 `if len(header) == 0 && len(rows) == 0 {`；L394 `if i < cols && w > widths[i] {`；L412 `if available < cols*3 {`；L422 `if widths[i] < 3 {`；L442 `for _, row := range rows {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（mdRenderer::renderTableRow、mdRenderer::collectCells）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method mdRenderer::renderTable（internal/cli/md.go:365-446）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `mdRenderer::renderBlock`（`internal/cli/md.go:172`，调用点 L189，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `dim`（`internal/cli/style.go:49`，调用点 L429/L436/L438，confidence=0.90，exact-match，3 条边）
- **待源码复核被调候选（3）：** `visibleWidth`（`internal/cli/box.go:14`，调用点 L399/L403，confidence=0.70，exact-match，2 条边，待源码复核）；`mdRenderer::renderTableRow`（`internal/cli/md.go:452`，调用点 L432/L443，confidence=0.70，instance-method，2 条边，待源码复核）；`mdRenderer::collectCells`（`internal/cli/md.go:488`，调用点 L372/L374，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** `mdRenderer::renderList`（`internal/cli/md.go:242`，调用点 L275，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/md.go`、`internal/cli/style.go`、`internal/cli/box.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*mdRenderer.renderTable` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L369、L382、L401、L420、L442 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*mdRenderer.renderTable` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/md.go|*mdRenderer.renderTable` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/md.go:365-446` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*mdRenderer.renderTable` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/md.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/md.go|*mdRenderer.renderTable` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-103 · 圈复杂度 · `botStart`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-103 |
| Baseline key | `complexity\|internal/cli/bot.go\|botStart` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 46） |
| 文件 | `internal/cli/bot.go` |
| 代码行 | 47 |
| 函数/方法/包 | `botStart` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | `FN-016` |

#### 明确的问题描述

`botStart` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 47-154 行；可见控制流热点：L53 `if err := fs.Parse(args); err != nil {`；L62 `if wd, err := os.Getwd(); err == nil {`；L71 `if err != nil {`；L80 `if !cfg.Bot.Allowlist.AllowAll && (!cfg.Bot.Allowlist.Enabled || botruntime.AllowlistUserCount(cfg.…`；L87 `for _, ch := range unknownChannels {`；L146 `if err := gw.Start(ctx); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（ChannelConfigs、ConnectionChannelConfigs、loadBotCommandConfigForActor、NewProfilePathResolver）、持久化/I-O（NewProfilePathResolver）、网络/协议（RequestedFeishuDomains）、权限/安全（AllowlistUserCount）、任务/并发（ChannelConfigs、ConnectionChannelConfigs、splitBotChannels）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function botStart（internal/cli/bot.go:47-154）
- **可信生产调用方（1）：** `botCommand`（`internal/cli/bot.go:21`，调用点 L32，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（16）：** `ServiceActorContext`（`internal/bot/actor.go:10`，调用点 L67，confidence=0.90，import）；`NewGatewayWithAdapterBindings`（`internal/bot/gateway.go:149`，调用点 L130，confidence=0.90，import）；`EnabledPlatforms`（`internal/botruntime/runtime.go:21`，调用点 L86，confidence=0.90，import）；`RequestedFeishuDomains`（`internal/botruntime/runtime.go:52`，调用点 L129，confidence=0.90，import）；`HasEnabledPlatform`（`internal/botruntime/runtime.go:75`，调用点 L90，confidence=0.90，import）；`ChannelConfigs`（`internal/botruntime/runtime.go:110`，调用点 L107，confidence=0.90，import）；`ConnectionChannelConfigs`（`internal/botruntime/runtime.go:145`，调用点 L108，confidence=0.90，import）；`AdapterBindingsWithPaths`（`internal/botruntime/runtime.go:195`，调用点 L130，confidence=0.90，import）
- **待源码复核被调候选（1）：** `mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L53，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L122，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 25 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/bot`、`internal/botruntime`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/bot.go`、`internal/bot/actor.go`、`internal/bot/gateway.go`、`internal/botruntime/runtime.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `botStart` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L53、L62、L76、L87、L146 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `botStart` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-103、FN-016 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/bot.go|botStart` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/bot.go:47-154` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `botStart` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-103`、`FN-016` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/bot.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/bot.go|botStart` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-016 · 函数有效行数 · `botStart`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-016 |
| Baseline key | `func_lines\|internal/cli/bot.go\|botStart` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 46） |
| 文件 | `internal/cli/bot.go` |
| 代码行 | 47 |
| 函数/方法/包 | `botStart` |
| Baseline / 当前复测 / 阈值 | 88 / 88 / 80 |
| 超出 | +8，1.10x |
| 同一治理对象关联问题 | `CX-103` |

#### 明确的问题描述

`botStart` 的有效代码行 **88**，超过限制 **80**（+8）。CodeGraph 定义物理范围为 47-154 行，并记录 17 个直接下游候选；候选职责分布为 配置/解析（ChannelConfigs、ConnectionChannelConfigs、loadBotCommandConfigForActor、NewProfilePathResolver）、持久化/I-O（NewProfilePathResolver）、网络/协议（RequestedFeishuDomains）、权限/安全（AllowlistUserCount）、任务/并发（ChannelConfigs、ConnectionChannelConfigs、splitBotChannels）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function botStart（internal/cli/bot.go:47-154）
- **可信生产调用方（1）：** `botCommand`（`internal/cli/bot.go:21`，调用点 L32，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（16）：** `ServiceActorContext`（`internal/bot/actor.go:10`，调用点 L67，confidence=0.90，import）；`NewGatewayWithAdapterBindings`（`internal/bot/gateway.go:149`，调用点 L130，confidence=0.90，import）；`EnabledPlatforms`（`internal/botruntime/runtime.go:21`，调用点 L86，confidence=0.90，import）；`RequestedFeishuDomains`（`internal/botruntime/runtime.go:52`，调用点 L129，confidence=0.90，import）；`HasEnabledPlatform`（`internal/botruntime/runtime.go:75`，调用点 L90，confidence=0.90，import）；`ChannelConfigs`（`internal/botruntime/runtime.go:110`，调用点 L107，confidence=0.90，import）；`ConnectionChannelConfigs`（`internal/botruntime/runtime.go:145`，调用点 L108，confidence=0.90，import）；`AdapterBindingsWithPaths`（`internal/botruntime/runtime.go:195`，调用点 L130，confidence=0.90，import）
- **待源码复核被调候选（1）：** `mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L53，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L122，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 25 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/bot`、`internal/botruntime`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/bot.go`、`internal/bot/actor.go`、`internal/bot/gateway.go`、`internal/botruntime/runtime.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `botStart` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `botStart` 的 88 个有效行按 配置/解析、持久化/I-O、网络/协议 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L100 `构建网关配置`、L132 `信号处理`、L151 `等待信号或 context 取消` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-103、FN-016 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/bot.go|botStart` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/bot.go:47-154` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `botStart` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-103`、`FN-016` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/bot.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/bot.go|botStart` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-148 · 圈复杂度 · `parseMCPAdd`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-148 |
| Baseline key | `complexity\|internal/cli/mcp.go\|parseMCPAdd` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 46） |
| 文件 | `internal/cli/mcp.go` |
| 代码行 | 26 |
| 函数/方法/包 | `parseMCPAdd` |
| Baseline / 当前复测 / 阈值 | 27 / 27 / 10 |
| 超出 | +17，2.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`parseMCPAdd` 的圈复杂度当前为 **27**，超过限制 **10**（+17，2.70x）。定义范围为 26-113 行；可见控制流热点：L28 `if len(args) == 0 {`；L52 `if *dst == nil {`；L68 `case key == "--http" || key == "--streamable-http":`；L81 `if v, err = next(key); err != nil {`；L95 `if key == "--header" {`；L109 `case e.URL == "" && e.Command == "":`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function parseMCPAdd（internal/cli/mcp.go:26-113）
- **可信生产调用方（2）：** `chatTUI::runMCPSubcommand`（`internal/cli/chat_tui.go:3492`，调用点 L3519，confidence=0.90，exact-match）；`mcpAddCLI`（`internal/cli/mcp.go:230`，调用点 L231，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 22 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/mcp_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp.go`、`internal/cli/chat_tui.go`、`internal/cli/mcp_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `parseMCPAdd` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L28、L59、L77、L90、L109 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `parseMCPAdd` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp.go|parseMCPAdd` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp.go:26-113` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `parseMCPAdd` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/mcp_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp.go|parseMCPAdd` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-149 · 圈复杂度 · `fixCJKEmphasis`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-149 |
| Baseline key | `complexity\|internal/cli/md.go\|fixCJKEmphasis` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/cli/md.go` |
| 代码行 | 80 |
| 函数/方法/包 | `fixCJKEmphasis` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`fixCJKEmphasis` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 80-135 行；可见控制流热点：L90 `for i := 0; i < n; i++ {`；L94 `if r == '\`' && i+2 < n && runes[i+1] == '\`' && runes[i+2] == '\`' {`；L101 `if r == '\`' && !inFenced {`；L113 `if r == '\n' {`；L119 `if r == '*' && i+1 < n && runes[i+1] == '*' {`；L126 `if !inEmphasis && i >= 2 && !isSpace(runes[i-2]) && isCJKPunct(runes[i-2]) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function fixCJKEmphasis（internal/cli/md.go:80-135）
- **可信生产调用方（1）：** `mdRenderer::Render`（`internal/cli/md.go:56`，调用点 L60，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `isCJKPunct`（`internal/cli/md.go:140`，调用点 L126，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `isSpace`（`internal/cli/md.go:162`，调用点 L126，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2229，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/md_cjk_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/md.go`、`internal/cli/md_cjk_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `fixCJKEmphasis` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L90、L101、L107、L119、L126 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `fixCJKEmphasis` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/md.go|fixCJKEmphasis` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/md.go:80-135` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `fixCJKEmphasis` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/md_cjk_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/md.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/md.go|fixCJKEmphasis` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-154 · 圈复杂度 · `renderMemory`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-154 |
| Baseline key | `complexity\|internal/cli/memory_view.go\|renderMemory` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/cli/memory_view.go` |
| 代码行 | 11 |
| 函数/方法/包 | `renderMemory` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`renderMemory` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 11-63 行；可见控制流热点：L14 `if len(set.Docs) > 0 {`；L24 `if hasSaved {`；L30 `for _, f := range facts {`；L42 `if len(archived) > 0 {`；L47 `for _, f := range archived {`；L57 `if (hasSaved || len(archived) > 0) && set.Store.Dir != "" {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（viewBudget、viewCompactText、viewSubhead、viewMeta）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function renderMemory（internal/cli/memory_view.go:11-63）
- **可信生产调用方（1）：** `chatTUI::showMemory`（`internal/cli/memory.go:13`，调用点 L19，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `viewBudget`（`internal/cli/view_helpers.go:82`，调用点 L18/L29/L37/L39，confidence=0.90，exact-match，10 条边）；`viewCompactText`（`internal/cli/view_helpers.go:49`，调用点 L29/L37/L39/L46，confidence=0.90，exact-match，8 条边）；`viewSubhead`（`internal/cli/view_helpers.go:21`，调用点 L15/L29/L46，confidence=0.90，exact-match，3 条边）；`viewMeta`（`internal/cli/view_helpers.go:25`，调用点 L18/L37/L53，confidence=0.90，exact-match，3 条边）；`viewCompactPath`（`internal/cli/view_helpers.go:44`，调用点 L18/L54，confidence=0.90，exact-match，2 条边）；`min`（`internal/profile/selector.go:451`，调用点 L37/L53，confidence=0.90，exact-match，2 条边）；`viewHeader`（`internal/cli/view_helpers.go:17`，调用点 L13，confidence=0.90，exact-match）；`viewHint`（`internal/cli/view_helpers.go:33`，调用点 L61，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `visibleWidth`（`internal/cli/box.go:14`，调用点 L18/L37/L39/L53，confidence=0.70，exact-match，4 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/cli`、`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/view_helpers_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/memory_view.go`、`internal/cli/memory.go`、`internal/cli/view_helpers.go`、`internal/cli/view_helpers_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `renderMemory` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L14、L25、L36、L43、L57 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `renderMemory` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/memory_view.go|renderMemory` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/memory_view.go:11-63` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `renderMemory` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/view_helpers_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/memory_view.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/memory_view.go|renderMemory` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-119 · 圈复杂度 · `chatTUI.handleChooserKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-119 |
| Baseline key | `complexity\|internal/cli/chooser.go\|chatTUI.handleChooserKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 44） |
| 文件 | `internal/cli/chooser.go` |
| 代码行 | 89 |
| 函数/方法/包 | `chatTUI.handleChooserKey` |
| Baseline / 当前复测 / 阈值 | 22 / 22 / 10 |
| 超出 | +12，2.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.handleChooserKey` 的圈复杂度当前为 **22**，超过限制 **10**（+12，2.20x）。定义范围为 89-149 行；可见控制流热点：L91 `switch msg.String() {`；L99 `if c.tab > 0 {`；L113 `switch msg.String() {`；L125 `case "up", "k":`；L133 `case " ", "space":`；L143 `if idx := int(s[0] - '1'); idx < len(q.Options) {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chooser::onSubmitTab）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleChooserKey（internal/cli/chooser.go:89-149）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L887，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（5）：** `chatTUI::chooserActivate`（`internal/cli/chooser.go:154`，调用点 L139/L144，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::chooserAnswer`（`internal/cli/chooser.go:195`，调用点 L97/L115，confidence=0.70，instance-method，2 条边，待源码复核）；`chooser::onSubmitTab`（`internal/cli/chooser.go:42`，调用点 L112，confidence=0.70，instance-method，待源码复核）；`chooser::rowCount`（`internal/cli/chooser.go:46`，调用点 L130，confidence=0.70，instance-method，待源码复核）；`chooser::answers`（`internal/cli/chooser.go:66`，调用点 L115，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L887，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chooser.go`、`internal/cli/chat_tui.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleChooserKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L91、L104、L116、L130、L143 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.handleChooserKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chooser.go|chatTUI.handleChooserKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chooser.go:89-149` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleChooserKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chooser.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chooser.go|chatTUI.handleChooserKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-133 · 圈复杂度 · `diffBody`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-133 |
| Baseline key | `complexity\|internal/cli/diffview.go\|diffBody` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 44） |
| 文件 | `internal/cli/diffview.go` |
| 代码行 | 77 |
| 函数/方法/包 | `diffBody` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`diffBody` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 77-130 行；可见控制流热点：L78 `if d.Diff == "" {`；L92 `for _, ln := range src {`；L97 `case '@':`；L101 `if hunks > 0 {`；L111 `case '\\':`；L124 `if maxLines > 0 && len(rows) > maxLines {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function diffBody（internal/cli/diffview.go:77-130）
- **可信生产调用方（1）：** `diffBlock`（`internal/cli/diffview.go:62`，调用点 L71，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `dim`（`internal/cli/style.go:49`，调用点 L102/L112/L127，confidence=0.90，exact-match，3 条边）；`diffBar`（`internal/cli/diffview.go:135`，调用点 L106/L109，confidence=0.90，exact-match，2 条边）；`atoi`（`internal/cli/diffview.go:192`，调用点 L99，confidence=0.90，exact-match，2 条边）；`fgSGR`（`internal/cli/theme.go:301`，调用点 L106/L109，confidence=0.90，exact-match，2 条边）；`bgSGR`（`internal/cli/theme.go:311`，调用点 L106/L109，confidence=0.90，exact-match，2 条边）；`diffContext`（`internal/cli/diffview.go:155`，调用点 L118，confidence=0.90，exact-match）；`gutterWidth`（`internal/cli/diffview.go:160`，调用点 L88，confidence=0.90，exact-match）；`clampPlain`（`internal/cli/diffview.go:205`，调用点 L112，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/diffview_test.go`、`internal/cli/render_edge_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/diffview.go`、`internal/cli/style.go`、`internal/cli/theme.go`、`internal/cli/diffview_test.go`、`internal/cli/render_edge_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`

#### 待修改项

- [ ] 先冻结 `diffBody` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L78、L93、L98、L108、L124 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `diffBody` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/diffview.go|diffBody` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/diffview.go:77-130` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `diffBody` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/diffview_test.go`、`internal/cli/render_edge_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/diffview.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/diffview.go|diffBody` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-151 · 圈复杂度 · `*mdRenderer.appendInline`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-151 |
| Baseline key | `complexity\|internal/cli/md.go\|*mdRenderer.appendInline` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/cli/md.go` |
| 代码行 | 316 |
| 函数/方法/包 | `*mdRenderer.appendInline` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*mdRenderer.appendInline` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 316-356 行；可见控制流热点：L317 `for c := n.FirstChild(); c != nil; c = c.NextSibling() {`；L321 `switch {`；L324 `case v.SoftLineBreak():`；L335 `case *ast.CodeSpan:`；L344 `case *ast.AutoLink:`；L350 `case *ast.String:`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（mdRenderer::appendInline）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method mdRenderer::appendInline（internal/cli/md.go:316-356）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `mdRenderer::appendInline`（`internal/cli/md.go:316`，调用点 L329/L337/L341/L353，confidence=0.70，instance-method，4 条边，待源码复核）；`mdRenderer::collectInline`（`internal/cli/md.go:310`，调用点 L312，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（4）：** `italic`（`internal/cli/md.go:46`，调用点 L333/L349，confidence=0.90，exact-match，2 条边）；`bold`（`internal/cli/style.go:48`，调用点 L331，confidence=0.90，exact-match）；`dim`（`internal/cli/style.go:49`，调用点 L343，confidence=0.90，exact-match）；`accent`（`internal/cli/style.go:53`，调用点 L338，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `mdRenderer::appendInline`（`internal/cli/md.go:316`，调用点 L329/L337/L341/L353，confidence=0.70，instance-method，4 条边，待源码复核）
- **上游入口候选（最多两跳）：** `mdRenderer::renderList`（`internal/cli/md.go:242`，调用点 L266，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/md.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*mdRenderer.appendInline` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L317、L321、L330、L344、L350 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*mdRenderer.appendInline` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/md.go|*mdRenderer.appendInline` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/md.go:316-356` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*mdRenderer.appendInline` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/md.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/md.go|*mdRenderer.appendInline` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-150 · 圈复杂度 · `isCJKPunct`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-150 |
| Baseline key | `complexity\|internal/cli/md.go\|isCJKPunct` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/cli/md.go` |
| 代码行 | 140 |
| 函数/方法/包 | `isCJKPunct` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`isCJKPunct` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 140-159 行；可见控制流热点：L141 `if r <= 0x7F {`；L145 `switch {`；L146 `case r >= 0x3000 && r <= 0x303F: // CJK Symbols and Punctuation (。、etc.)`；L150 `case r >= 0xFF1A && r <= 0xFF20: // Fullwidth Forms II (: ; < = etc.)`；L152 `case r >= 0xFF3B && r <= 0xFF3F: // Fullwidth Forms III ([ \ ] ^ _)`；L154 `case r >= 0xFF5B && r <= 0xFF65: // Fullwidth Forms IV ({ | } ~ etc.)`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function isCJKPunct（internal/cli/md.go:140-159）
- **可信生产调用方（1）：** `fixCJKEmphasis`（`internal/cli/md.go:80`，调用点 L126，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/md_cjk_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/md.go`、`internal/cli/md_cjk_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `isCJKPunct` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L141、L146、L148、L152、L154 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `isCJKPunct` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/md.go|isCJKPunct` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/md.go:140-159` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `isCJKPunct` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/md_cjk_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/md.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/md.go|isCJKPunct` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

## 4. 本组完成条件

- [ ] 本组列出的职责拆分均完成，未夹带相邻无关重构。
- [ ] 所有关联调用方和测试均已复核。
- [ ] 聚焦测试、相关包测试、必要的 race/vet/build 检查通过。
- [ ] Code Guard 局部与全量检查完成。
- [ ] 对应 baseline key 明确下降或消失，并在最新主线基础上显式刷新。
- [ ] 未新增碎片文件、纯转发包装或重复实现。
- [ ] 与后续同文件组的接口和职责边界已记录。
