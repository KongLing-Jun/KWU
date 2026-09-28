# G29 · internal/bot · gateway 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G29 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/bot`、`internal/botruntime` |
| 涉及文件 | `internal/bot/gateway.go`<br>`internal/botruntime/runtime.go`<br>`internal/bot/feishu/feishu.go`<br>`internal/bot/weixin/weixin.go` |
| 主要治理目标 | 围绕 `internal/botruntime/runtime.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | bot / botruntime 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/im-session-refresh`；单文件最多集中 7 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 29 组；先于 G36、G49 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 权限、安全或凭据边界；会话、状态或恢复语义；并发、取消或事件顺序；配置、平台或向后兼容；返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **44/100**，对象 `*BotGateway.handleSlashCommand` 的分项为严重度 14.0、集中度 10.9、影响面 11.3、可执行性 8.0；最高超限 3.10x，文件内 5 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G36、G49，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-086` | `complexity\|internal/bot/gateway.go\|*BotGateway.handleSlashCommand` | 圈复杂度 | `internal/bot/gateway.go` | `*BotGateway.handleSlashCommand` | 509 | 31/10 | +21（3.10x） | P2 | 0 可信 + 1 待核 caller / 6 可信 + 6 待核 callee / 36 unresolved | 待修改 |
| `FN-012` | `func_lines\|internal/bot/gateway.go\|*BotGateway.handleSlashCommand` | 函数有效行数 | `internal/bot/gateway.go` | `*BotGateway.handleSlashCommand` | 509 | 124/80 | +44（1.55x） | P2 | 0 可信 + 1 待核 caller / 6 可信 + 6 待核 callee / 36 unresolved | 待修改 |
| `CX-098` | `complexity\|internal/botruntime/runtime.go\|AdapterBindingsWithPaths` | 圈复杂度 | `internal/botruntime/runtime.go` | `AdapterBindingsWithPaths` | 195 | 18/10 | +8（1.80x） | P2 | 3 可信 + 0 待核 caller / 6 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-099` | `complexity\|internal/botruntime/runtime.go\|rememberInbound` | 圈复杂度 | `internal/botruntime/runtime.go` | `rememberInbound` | 358 | 18/10 | +8（1.80x） | P2 | 2 可信 + 0 待核 caller / 8 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-082` | `complexity\|internal/bot/feishu/feishu.go\|*adapter.handleSDKMessage` | 圈复杂度 | `internal/bot/feishu/feishu.go` | `*adapter.handleSDKMessage` | 229 | 17/10 | +7（1.70x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-083` | `complexity\|internal/bot/feishu/feishu.go\|*adapter.sendSDKContent` | 圈复杂度 | `internal/bot/feishu/feishu.go` | `*adapter.sendSDKContent` | 523 | 12/10 | +2（1.20x） | P3 | 0 可信 + 2 待核 caller / 2 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-084` | `complexity\|internal/bot/feishu/feishu.go\|*adapter.runWebhook` | 圈复杂度 | `internal/bot/feishu/feishu.go` | `*adapter.runWebhook` | 661 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-094` | `complexity\|internal/bot/weixin/weixin.go\|*adapter.getUpdates` | 圈复杂度 | `internal/bot/weixin/weixin.go` | `*adapter.getUpdates` | 357 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 7 待核 callee / 0 unresolved | 待修改 |
| `FL-005` | `file_lines\|internal/bot/gateway.go\|` | 文件有效行数 | `internal/bot/gateway.go` | `internal/bot/gateway.go` | 1 | 962/800 | +162（1.20x） | P3 | 2 可信 + 0 待核 caller / 13 可信 + 7 待核 callee / 193 unresolved | 待修改 |
| `CX-085` | `complexity\|internal/bot/gateway.go\|*BotGateway.handleMessage` | 圈复杂度 | `internal/bot/gateway.go` | `*BotGateway.handleMessage` | 279 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 6 可信 + 6 待核 callee / 6 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-086 · 圈复杂度 · `*BotGateway.handleSlashCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-086 |
| Baseline key | `complexity\|internal/bot/gateway.go\|*BotGateway.handleSlashCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 44） |
| 文件 | `internal/bot/gateway.go` |
| 代码行 | 509 |
| 函数/方法/包 | `*BotGateway.handleSlashCommand` |
| Baseline / 当前复测 / 阈值 | 31 / 31 / 10 |
| 超出 | +21，3.10x |
| 同一治理对象关联问题 | `FN-012` |

#### 明确的问题描述

`*BotGateway.handleSlashCommand` 的圈复杂度当前为 **31**，超过限制 **10**（+21，3.10x）。定义范围为 509-642 行；可见控制流热点：L510 `switch {`；L526 `if state.cancel != nil {`；L555 `case strings.HasPrefix(msg.Text, "/deny"):`；L574 `if len(parts) < 3 {`；L603 `case strings.HasPrefix(msg.Text, "/yolo") || strings.HasPrefix(msg.Text, "/mode"):`；L628 `case strings.HasPrefix(msg.Text, "/help"):`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseToolApprovalModeCommand、parseAskAnswers）、状态与控制（parseToolApprovalModeCommand、BotGateway::setToolApprovalModeForMessage）、持久化/I-O（BotGateway::rememberSessionReady）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method BotGateway::handleSlashCommand（internal/bot/gateway.go:509-642）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `BotGateway::handleMessage`（`internal/bot/gateway.go:279`，调用点 L326，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `SessionManager::ForceRelease`（`internal/bot/session.go:190`，调用点 L518/L534，confidence=0.90，exact-match，2 条边）；`parseToolApprovalModeCommand`（`internal/bot/gateway.go:644`，调用点 L604，confidence=0.90，exact-match）；`toolApprovalModeChangedText`（`internal/bot/gateway.go:741`，调用点 L614，confidence=0.90，exact-match）；`toolApprovalModeLabel`（`internal/bot/gateway.go:752`，调用点 L626，confidence=0.90，exact-match）；`parseAskAnswers`（`internal/bot/gateway.go:1062`，调用点 L599，confidence=0.90，exact-match）；`SessionManager::ActiveCount`（`internal/bot/session.go:183`，调用点 L621，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `BotGateway::sendText`（`internal/bot/gateway.go:1045`，调用点 L519/L535/L541/L550，confidence=0.70，instance-method，16 条边，待源码复核）；`BotGateway::forgetPendingApproval`（`internal/bot/gateway.go:435`，调用点 L549/L566，confidence=0.70，instance-method，2 条边，待源码复核）；`BotGateway::setToolApprovalModeForMessage`（`internal/bot/gateway.go:680`，调用点 L613，confidence=0.70，instance-method，待源码复核）；`BotGateway::currentToolApprovalMode`（`internal/bot/gateway.go:722`，调用点 L625，confidence=0.70，instance-method，待源码复核）；`BotGateway::toolApprovalModeStatusText`（`internal/bot/gateway.go:736`，调用点 L610，confidence=0.70，instance-method，待源码复核）；`BotGateway::rememberSessionReady`（`internal/bot/gateway.go:954`，调用点 L532，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `BotGateway::handleMessage`（`internal/bot/gateway.go:279`，调用点 L326，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 36 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/bot/gateway_test.go`
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/gateway.go`、`internal/bot/session.go`、`internal/bot/gateway_test.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`

#### 待修改项

- [ ] 先冻结 `*BotGateway.handleSlashCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L510、L529、L564、L595、L628 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*BotGateway.handleSlashCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-086、FN-012 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/gateway.go|*BotGateway.handleSlashCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/gateway.go:509-642` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*BotGateway.handleSlashCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-086`、`FN-012` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/bot/gateway_test.go`。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/gateway.go|*BotGateway.handleSlashCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-012 · 函数有效行数 · `*BotGateway.handleSlashCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-012 |
| Baseline key | `func_lines\|internal/bot/gateway.go\|*BotGateway.handleSlashCommand` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 44） |
| 文件 | `internal/bot/gateway.go` |
| 代码行 | 509 |
| 函数/方法/包 | `*BotGateway.handleSlashCommand` |
| Baseline / 当前复测 / 阈值 | 124 / 124 / 80 |
| 超出 | +44，1.55x |
| 同一治理对象关联问题 | `CX-086` |

#### 明确的问题描述

`*BotGateway.handleSlashCommand` 的有效代码行 **124**，超过限制 **80**（+44）。CodeGraph 定义物理范围为 509-642 行，并记录 12 个直接下游候选；候选职责分布为 配置/解析（parseToolApprovalModeCommand、parseAskAnswers）、状态与控制（parseToolApprovalModeCommand、BotGateway::setToolApprovalModeForMessage）、持久化/I-O（BotGateway::rememberSessionReady）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method BotGateway::handleSlashCommand（internal/bot/gateway.go:509-642）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `BotGateway::handleMessage`（`internal/bot/gateway.go:279`，调用点 L326，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `SessionManager::ForceRelease`（`internal/bot/session.go:190`，调用点 L518/L534，confidence=0.90，exact-match，2 条边）；`parseToolApprovalModeCommand`（`internal/bot/gateway.go:644`，调用点 L604，confidence=0.90，exact-match）；`toolApprovalModeChangedText`（`internal/bot/gateway.go:741`，调用点 L614，confidence=0.90，exact-match）；`toolApprovalModeLabel`（`internal/bot/gateway.go:752`，调用点 L626，confidence=0.90，exact-match）；`parseAskAnswers`（`internal/bot/gateway.go:1062`，调用点 L599，confidence=0.90，exact-match）；`SessionManager::ActiveCount`（`internal/bot/session.go:183`，调用点 L621，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `BotGateway::sendText`（`internal/bot/gateway.go:1045`，调用点 L519/L535/L541/L550，confidence=0.70，instance-method，16 条边，待源码复核）；`BotGateway::forgetPendingApproval`（`internal/bot/gateway.go:435`，调用点 L549/L566，confidence=0.70，instance-method，2 条边，待源码复核）；`BotGateway::setToolApprovalModeForMessage`（`internal/bot/gateway.go:680`，调用点 L613，confidence=0.70，instance-method，待源码复核）；`BotGateway::currentToolApprovalMode`（`internal/bot/gateway.go:722`，调用点 L625，confidence=0.70，instance-method，待源码复核）；`BotGateway::toolApprovalModeStatusText`（`internal/bot/gateway.go:736`，调用点 L610，confidence=0.70，instance-method，待源码复核）；`BotGateway::rememberSessionReady`（`internal/bot/gateway.go:954`，调用点 L532，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `BotGateway::handleMessage`（`internal/bot/gateway.go:279`，调用点 L326，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 36 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/bot/gateway_test.go`
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/gateway.go`、`internal/bot/session.go`、`internal/bot/gateway_test.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`

#### 待修改项

- [ ] 先冻结 `*BotGateway.handleSlashCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*BotGateway.handleSlashCommand` 的 124 个有效行按 配置/解析、状态与控制、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L538 `从消息中解析 approval ID` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-086、FN-012 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/bot/gateway.go|*BotGateway.handleSlashCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/gateway.go:509-642` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*BotGateway.handleSlashCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。
- 必须与 `CX-086`、`FN-012` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/bot/gateway_test.go`。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/bot/gateway.go|*BotGateway.handleSlashCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-098 · 圈复杂度 · `AdapterBindingsWithPaths`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-098 |
| Baseline key | `complexity\|internal/botruntime/runtime.go\|AdapterBindingsWithPaths` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 44） |
| 文件 | `internal/botruntime/runtime.go` |
| 代码行 | 195 |
| 函数/方法/包 | `AdapterBindingsWithPaths` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`AdapterBindingsWithPaths` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 195-250 行；可见控制流热点：L196 `if cfg == nil {`；L202 `if !conn.Enabled {`；L211 `case bot.PlatformQQ:`；L222 `if feishuDomains != nil && !feishuDomains[feishuDomainKey(feishuCfg.Domain)] {`；L241 `if enabled[bot.PlatformFeishu] && !hasConnection[bot.PlatformFeishu] {`；L246 `if enabled[bot.PlatformWeixin] && !hasConnection[bot.PlatformWeixin] {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（ConnectionRuntimeID）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function AdapterBindingsWithPaths（internal/botruntime/runtime.go:195-250）
- **可信生产调用方（3）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L144，confidence=0.90，import）；`AdapterBindings`（`internal/botruntime/runtime.go:191`，调用点 L192，confidence=0.90，exact-match）；`botStart`（`internal/cli/bot.go:47`，调用点 L130，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `New`（`internal/bot/feishu/feishu.go:98`，调用点 L227/L243，confidence=0.90，import，2 条边）；`New`（`internal/bot/qq/adapter.go:21`，调用点 L216/L239，confidence=0.90，import，2 条边）；`NewWithPaths`（`internal/bot/weixin/weixin.go:135`，调用点 L234/L247，confidence=0.90，import，2 条边）；`feishuDomainKey`（`internal/botruntime/runtime.go:68`，调用点 L222/L242，confidence=0.90，exact-match，2 条边）；`Platform`（`internal/bot/types.go:8`，调用点 L205，confidence=0.90，import）；`ConnectionRuntimeID`（`internal/botruntime/runtime.go:252`，调用点 L209，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `firstNonEmptyString`（`internal/botruntime/runtime.go:553`，调用点 L214/L215/L221/L225，confidence=0.70，exact-match，7 条边，待源码复核）
- **上游入口候选（最多两跳）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L144，confidence=0.90，import，待源码复核）；`AdapterBindings`（`internal/botruntime/runtime.go:191`，调用点 L192，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/botruntime`、`desktop`、`internal/cli`、`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/botruntime/runtime_test.go`
- **需同步检查：** `internal/botruntime/runtime.go`、`desktop/bot_runtime_app.go`、`internal/cli/bot.go`、`internal/bot/feishu/feishu.go`、`internal/bot/qq/adapter.go`、`internal/bot/weixin/weixin.go`、`internal/botruntime/runtime_test.go`

#### 待修改项

- [ ] 逐段核对 L196、L206、L218、L238、L246 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `AdapterBindingsWithPaths` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/botruntime/runtime.go|AdapterBindingsWithPaths` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/botruntime/runtime.go:195-250` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `AdapterBindingsWithPaths` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/botruntime/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/botruntime`。
- [ ] 运行相关包 vet：`go vet ./internal/botruntime`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/botruntime/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/botruntime/runtime.go|AdapterBindingsWithPaths` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-099 · 圈复杂度 · `rememberInbound`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-099 |
| Baseline key | `complexity\|internal/botruntime/runtime.go\|rememberInbound` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 44） |
| 文件 | `internal/botruntime/runtime.go` |
| 代码行 | 358 |
| 函数/方法/包 | `rememberInbound` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`rememberInbound` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 358-436 行；可见控制流热点：L360 `if actualWorkspaceRoot != "" {`；L377 `for i := range cfg.Bot.Connections {`；L383 `for j := range conn.SessionMappings {`；L390 `if sessionID == "" {`；L407 `if strings.TrimSpace(conn.WorkspaceRoot) != "" {`；L432 `if !changed {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NewProfilePathResolver、Config::SaveToScope、ProfilePathResolver::UserConfigPath）、持久化/I-O（NewProfilePathResolver、Config::SaveToScope、ProfilePathResolver::UserConfigPath）、权限/安全（rememberAllowlist）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function rememberInbound（internal/botruntime/runtime.go:358-436）
- **可信生产调用方（2）：** `RememberInboundWithActor`（`internal/botruntime/runtime.go:342`，调用点 L343，confidence=0.90，exact-match）；`RememberInboundSessionWorkspaceWithActor`（`internal/botruntime/runtime.go:354`，调用点 L355，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `ActorContextForInbound`（`internal/bot/actor.go:20`，调用点 L363，confidence=0.90，import）；`botSessionMappingMatches`（`internal/botruntime/runtime.go:438`，调用点 L384，confidence=0.90，exact-match）；`botSessionMappingIdentity`（`internal/botruntime/runtime.go:456`，调用点 L414，confidence=0.90，exact-match）；`botSessionMappingHasExplicitTarget`（`internal/botruntime/runtime.go:471`，调用点 L395，confidence=0.90，exact-match）；`botSessionSource`（`internal/botruntime/runtime.go:479`，调用点 L418，confidence=0.90，exact-match）；`connectionMatchesInbound`（`internal/botruntime/runtime.go:486`，调用点 L379，confidence=0.90，exact-match）；`rememberAllowlist`（`internal/botruntime/runtime.go:496`，调用点 L429，confidence=0.90，exact-match）；`NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L364，confidence=0.90，import）
- **待源码复核被调候选（2）：** `Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L435，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L365，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `RememberInbound`（`internal/botruntime/runtime.go:338`，调用点 L339，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/botruntime`、`internal/bot`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/botruntime/runtime_test.go`
- **需同步检查：** `internal/botruntime/runtime.go`、`internal/bot/actor.go`、`internal/botruntime/runtime_test.go`

#### 待修改项

- [ ] 逐段核对 L360、L379、L389、L395、L432 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `rememberInbound` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/botruntime/runtime.go|rememberInbound` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/botruntime/runtime.go:358-436` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `rememberInbound` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：持久化、迁移或文件格式兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/botruntime/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/botruntime`。
- [ ] 运行相关包 vet：`go vet ./internal/botruntime`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/botruntime/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/botruntime/runtime.go|rememberInbound` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-082 · 圈复杂度 · `*adapter.handleSDKMessage`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-082 |
| Baseline key | `complexity\|internal/bot/feishu/feishu.go\|*adapter.handleSDKMessage` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 40） |
| 文件 | `internal/bot/feishu/feishu.go` |
| 代码行 | 229 |
| 函数/方法/包 | `*adapter.handleSDKMessage` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*adapter.handleSDKMessage` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 229-285 行；可见控制流热点：L230 `if event == nil || event.Event == nil || event.Event.Message == nil {`；L237 `if eventID != "" {`；L243 `if stringPtrValue(msg.MessageType) != "text" {`；L253 `if stringPtrValue(msg.ChatType) == "group" || stringPtrValue(msg.ChatType) == "topic_group" {`；L261 `if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {`；L280 `case a.msgCh <- ib:`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method adapter::handleSDKMessage（internal/bot/feishu/feishu.go:229-285）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `adapter::newEventDispatcher`（`internal/bot/feishu/feishu.go:205`，调用点 L208，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `stringPtrValue`（`internal/bot/feishu/feishu.go:642`，调用点 L243/L244/L248/L249，confidence=0.90，exact-match，16 条边）
- **待源码复核被调候选（3）：** `logHash`（`internal/bot/feishu/feishu.go:419`，调用点 L244/L249/L256/L281，confidence=0.70，exact-match，7 条边，待源码复核）；`adapter::markSeen`（`internal/bot/feishu/feishu.go:368`，调用点 L238，confidence=0.70，instance-method，待源码复核）；`firstNonEmpty`（`internal/bot/feishu/feishu.go:410`，调用点 L262，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/feishu/feishu_test.go`
- **需同步检查：** `internal/bot/feishu/feishu.go`、`internal/bot/feishu/feishu_test.go`

#### 待修改项

- [ ] 逐段核对 L230、L238、L248、L261、L280 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*adapter.handleSDKMessage` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/feishu/feishu.go|*adapter.handleSDKMessage` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/feishu/feishu.go:229-285` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*adapter.handleSDKMessage` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/feishu/feishu_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot/feishu`。
- [ ] 运行相关包 race：`go test -race ./internal/bot/feishu`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/feishu`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/feishu/feishu.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/feishu/feishu.go|*adapter.handleSDKMessage` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-083 · 圈复杂度 · `*adapter.sendSDKContent`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-083 |
| Baseline key | `complexity\|internal/bot/feishu/feishu.go\|*adapter.sendSDKContent` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/bot/feishu/feishu.go` |
| 代码行 | 523 |
| 函数/方法/包 | `*adapter.sendSDKContent` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*adapter.sendSDKContent` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 523-571 行；可见控制流热点：L525 `if err != nil {`；L534 `if err != nil {`；L540 `if !resp.Success() {`；L550 `if chatID == "" {`；L561 `if resp == nil {`；L567 `if resp.Data == nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method adapter::sendSDKContent（internal/bot/feishu/feishu.go:523-571）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `adapter::sendMessage`（`internal/bot/feishu/feishu.go:481`，调用点 L489/L491，confidence=0.70，instance-method，2 条边，待源码复核）；`adapter::sendCard`（`internal/bot/feishu/feishu.go:602`，调用点 L632，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `stringPtrValue`（`internal/bot/feishu/feishu.go:642`，调用点 L546/L570，confidence=0.90，exact-match，2 条边）；`feishuCodeError`（`internal/bot/feishu/feishu.go:649`，调用点 L541/L565，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（1）：** `adapter::sdkClient`（`internal/bot/feishu/feishu.go:503`，调用点 L524，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `adapter::sendMessage`（`internal/bot/feishu/feishu.go:481`，调用点 L489，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/feishu/feishu_test.go`
- **需同步检查：** `internal/bot/feishu/feishu.go`、`internal/bot/feishu/feishu_test.go`

#### 待修改项

- [ ] 逐段核对 L525、L537、L543、L561、L567 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*adapter.sendSDKContent` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/feishu/feishu.go|*adapter.sendSDKContent` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/feishu/feishu.go:523-571` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*adapter.sendSDKContent` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/feishu/feishu_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot/feishu`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/feishu`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/feishu/feishu.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/feishu/feishu.go|*adapter.sendSDKContent` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-084 · 圈复杂度 · `*adapter.runWebhook`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-084 |
| Baseline key | `complexity\|internal/bot/feishu/feishu.go\|*adapter.runWebhook` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/bot/feishu/feishu.go` |
| 代码行 | 661 |
| 函数/方法/包 | `*adapter.runWebhook` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*adapter.runWebhook` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 661-725 行；可见控制流热点：L663 `if port == 0 {`；L680 `if challenge.Type == "url_verification" {`；L686 `if err := json.NewEncoder(w).Encode(map[string]string{"challenge": challenge.Challenge}); err != ni…`；L693 `if err := json.Unmarshal(body, &evt); err != nil {`；L702 `if !a.handleCardAction(body) {`；L722 `if err := server.ListenAndServe(); err != http.ErrServerClosed {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Encode）、状态与控制（adapter::handleWSEvent、adapter::handleCardAction）、网络/协议（adapter::handleWSEvent）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method adapter::runWebhook（internal/bot/feishu/feishu.go:661-725）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `adapter::Start`（`internal/bot/feishu/feishu.go:109`，调用点 L126，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `Encode`（`internal/fileutil/encoding/encoding.go:166`，调用点 L686，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `adapter::verificationTokenValid`（`internal/bot/feishu/feishu.go:406`，调用点 L681/L697，confidence=0.70，instance-method，2 条边，待源码复核）；`adapter::handleWSEvent`（`internal/bot/feishu/feishu.go:287`，调用点 L704，confidence=0.70，instance-method，待源码复核）；`adapter::handleCardAction`（`internal/bot/feishu/feishu.go:307`，调用点 L702，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `adapter::Start`（`internal/bot/feishu/feishu.go:109`，调用点 L126，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/bot`、`internal/fileutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/feishu/feishu_test.go`
- **需同步检查：** `internal/bot/feishu/feishu.go`、`internal/fileutil/encoding/encoding.go`、`internal/bot/feishu/feishu_test.go`

#### 待修改项

- [ ] 逐段核对 L663、L680、L693、L702、L722 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*adapter.runWebhook` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/feishu/feishu.go|*adapter.runWebhook` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/feishu/feishu.go:661-725` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*adapter.runWebhook` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/feishu/feishu_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot/feishu`。
- [ ] 运行相关包 race：`go test -race ./internal/bot/feishu`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/feishu`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/feishu/feishu.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/feishu/feishu.go|*adapter.runWebhook` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-094 · 圈复杂度 · `*adapter.getUpdates`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-094 |
| Baseline key | `complexity\|internal/bot/weixin/weixin.go\|*adapter.getUpdates` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/bot/weixin/weixin.go` |
| 代码行 | 357 |
| 函数/方法/包 | `*adapter.getUpdates` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*adapter.getUpdates` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 357-415 行；可见控制流热点：L359 `if tok == "" {`；L382 `if err != nil {`；L388 `if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {`；L400 `if result.GetUpdatesBuf != "" {`；L403 `if len(result.Updates) > 0 {`；L410 `for _, msg := range result.Msgs {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Decode）、状态与控制（setIlinkHeaders、adapter::handleIlinkMessage）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method adapter::getUpdates（internal/bot/weixin/weixin.go:357-415）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `adapter::pollLoop`（`internal/bot/weixin/weixin.go:323`，调用点 L334，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `setIlinkHeaders`（`internal/bot/weixin/weixin.go:537`，调用点 L379，confidence=0.90，exact-match）；`Decode`（`internal/fileutil/encoding/encoding.go:122`，调用点 L388，confidence=0.90，exact-match）
- **待源码复核被调候选（7）：** `adapter::apiBase`（`internal/bot/weixin/weixin.go:214`，调用点 L363/L395，confidence=0.70，instance-method，2 条边，待源码复核）；`adapter::token`（`internal/bot/weixin/weixin.go:191`，调用点 L358，confidence=0.70，instance-method，待源码复核）；`adapter::tokenMissingError`（`internal/bot/weixin/weixin.go:206`，调用点 L360，confidence=0.70，instance-method，待源码复核）；`adapter::accountID`（`internal/bot/weixin/weixin.go:225`，调用点 L395，confidence=0.70，instance-method，待源码复核）；`adapter::logPollHealth`（`internal/bot/weixin/weixin.go:417`，调用点 L397，confidence=0.70，instance-method，待源码复核）；`adapter::handleIlinkMessage`（`internal/bot/weixin/weixin.go:468`，调用点 L411，confidence=0.70，instance-method，待源码复核）；`logHash`（`internal/bot/weixin/weixin.go:503`，调用点 L395，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `adapter::Start`（`internal/bot/weixin/weixin.go:147`，调用点 L156，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/bot`、`internal/fileutil`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/bot/weixin/weixin_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/bot/weixin/weixin.go`、`internal/fileutil/encoding/encoding.go`、`internal/bot/weixin/weixin_test.go`

#### 待修改项

- [ ] 逐段核对 L359、L382、L391、L403、L410 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*adapter.getUpdates` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/weixin/weixin.go|*adapter.getUpdates` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/weixin/weixin.go:357-415` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*adapter.getUpdates` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/bot/weixin/weixin_test.go`。
- [ ] 运行相关包测试：`go test ./internal/bot/weixin`。
- [ ] 运行相关包 vet：`go vet ./internal/bot/weixin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/weixin/weixin.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/weixin/weixin.go|*adapter.getUpdates` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-005 · 文件有效行数 · `internal/bot/gateway.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-005 |
| Baseline key | `file_lines\|internal/bot/gateway.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/bot/gateway.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/bot/gateway.go` |
| Baseline / 当前复测 / 阈值 | 962 / 962 / 800 |
| 超出 | +162，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/bot/gateway.go` 当前有效代码行 **962**，超过限制 **800**（+162）。CodeGraph 在文件中索引到的主要大符号包括 BotGateway::handleSlashCommand@509-642、BotGateway::runTurn@763-846、BotGateway::handleMessage@279-353、BotGateway::getOrCreateSession@848-906、BotGateway::sessionOptionsForMessage@988-1023、parseAskAnswers@1062-1094；文件同时承载的调用影响覆盖 12 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 47 个函数/方法节点。
- **可信生产调用方（2）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L150/L158/L154/L157，confidence=0.90，import，4 条边）；`botStart`（`internal/cli/bot.go:47`，调用点 L130，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（13）：** `hashID`（`internal/bot/hash.go:9`，调用点 L764/L769/L845/L294，confidence=0.90，exact-match，18 条边）；`ActorContextForInbound`（`internal/bot/actor.go:20`，调用点 L910/L920，confidence=0.90，exact-match，2 条边）；`SessionManager::ForceRelease`（`internal/bot/session.go:190`，调用点 L518/L534，confidence=0.90，exact-match，2 条边）；`Build`（`internal/boot/boot.go:152`，调用点 L863，confidence=0.90，import）；`ServiceActorContext`（`internal/bot/actor.go:10`，调用点 L914，confidence=0.90，exact-match）；`newRenderSink`（`internal/bot/render.go:47`，调用点 L793，confidence=0.90，exact-match）；`SessionManager::ActiveCount`（`internal/bot/session.go:183`，调用点 L621，confidence=0.90，exact-match）；`BuildSessionKey`（`internal/bot/session.go:17`，调用点 L288，confidence=0.90，exact-match）
- **待源码复核被调候选（7）：** `adapter::SendTyping`（`internal/bot/feishu/feishu.go:150`，调用点 L790，confidence=0.85，qualified-name，待源码复核）；`adapter::Platform`（`internal/bot/feishu/feishu.go:106`，调用点 L176，confidence=0.70，exact-match，待源码复核）；`adapter::Send`（`internal/bot/feishu/feishu.go:146`，调用点 L1046，confidence=0.85，qualified-name，待源码复核）；`BotGateway::addPendingReaction`（`internal/bot/gateway_helpers.go:8`，调用点 L330，confidence=0.70，instance-method，待源码复核）；`SessionManager::Release`（`internal/bot/session.go:147`，调用点 L767，confidence=0.70，exact-match，待源码复核）；`ProfilePathResolver::ProjectSessionDir`（`internal/config/paths.go:209`，调用点 L948，confidence=0.70，instance-method，待源码复核）；`Sink::Emit`（`internal/event/event.go:362`，调用点 L839，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 193 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 12 个文件；`internal/bot`、`desktop`、`internal/cli`、`internal/boot`、`internal/config`、`internal/event`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/bot/gateway_test.go`
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/gateway.go`、`desktop/bot_runtime_app.go`、`internal/cli/bot.go`、`internal/bot/hash.go`、`internal/bot/actor.go`、`internal/bot/session.go`、`internal/boot/boot.go`、`internal/bot/gateway_test.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`

#### 待修改项

- [ ] 先冻结 `internal/bot/gateway.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `BotGateway::handleSlashCommand`、`BotGateway::runTurn`、`BotGateway::handleMessage`、`BotGateway::getOrCreateSession`、`BotGateway::sessionOptionsForMessage` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-085、CX-086、CX-087、FN-012。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/bot/gateway.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/gateway.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/bot/gateway.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：并发、取消或事件顺序；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/bot/gateway_test.go`。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 race：`go test -race ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/bot/gateway.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-085 · 圈复杂度 · `*BotGateway.handleMessage`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-085 |
| Baseline key | `complexity\|internal/bot/gateway.go\|*BotGateway.handleMessage` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 27） |
| 文件 | `internal/bot/gateway.go` |
| 代码行 | 279 |
| 函数/方法/包 | `*BotGateway.handleMessage` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*BotGateway.handleMessage` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 279-353 行；可见控制流热点：L281 `if msg.ConnectionID == "" {`；L284 `if msg.Domain == "" {`；L310 `if gw.cfg.OnInbound != nil {`；L314 `if normalized, ok := gw.normalizeApprovalShortcut(key, msg.Text); ok {`；L334 `if merged {`；L338 `if !acquired {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（BotGateway::normalizeApprovalShortcut、BotGateway::normalizeAskShortcut）、状态与控制（BotGateway::handleSlashCommand）、权限/安全（BotGateway::checkAllowlist）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method BotGateway::handleMessage（internal/bot/gateway.go:279-353）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `hashID`（`internal/bot/hash.go:9`，调用点 L294/L295/L296/L297，confidence=0.90，exact-match，6 条边）；`decisionShortcutCommand`（`internal/bot/gateway.go:407`，调用点 L318，confidence=0.90，exact-match）；`BuildSessionKey`（`internal/bot/session.go:17`，调用点 L288，confidence=0.90，exact-match）；`IsSlashBypass`（`internal/bot/session.go:67`，调用点 L324，confidence=0.90，exact-match）；`SessionManager::TryAcquire`（`internal/bot/session.go:109`，调用点 L333，confidence=0.90，exact-match）；`SessionManager::IsActive`（`internal/bot/session.go:176`，调用点 L318，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `BotGateway::sendText`（`internal/bot/gateway.go:1045`，调用点 L307/L319，confidence=0.70，instance-method，2 条边，待源码复核）；`BotGateway::checkAllowlist`（`internal/bot/gateway.go:355`，调用点 L305，confidence=0.70，instance-method，待源码复核）；`BotGateway::normalizeApprovalShortcut`（`internal/bot/gateway.go:385`，调用点 L314，confidence=0.70，instance-method，待源码复核）；`BotGateway::normalizeAskShortcut`（`internal/bot/gateway.go:452`，调用点 L316，confidence=0.70，instance-method，待源码复核）；`BotGateway::handleSlashCommand`（`internal/bot/gateway.go:509`，调用点 L326，confidence=0.70，instance-method，待源码复核）；`BotGateway::addPendingReaction`（`internal/bot/gateway_helpers.go:8`，调用点 L330，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 6 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`、`internal/bot/render_test.go`、`internal/bot/session_test.go`、`internal/bot/turn_dispatch_test.go`、`internal/bot/weixin/weixin_test.go`
- **需同步检查：** `internal/bot/gateway.go`、`internal/bot/hash.go`、`internal/bot/session.go`、`internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`

#### 待修改项

- [ ] 先冻结 `*BotGateway.handleMessage` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L281、L305、L314、L324、L338 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*BotGateway.handleMessage` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/bot/gateway.go|*BotGateway.handleMessage` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/bot/gateway.go:279-353` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*BotGateway.handleMessage` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G36、G49 处理。
- 重点保护：并发、取消或事件顺序；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/bot/actor_test.go`、`internal/bot/connloop_test.go`、`internal/bot/feishu/feishu_test.go`、`internal/bot/gateway_test.go`、`internal/bot/qq/gateway_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/bot`。
- [ ] 运行相关包 race：`go test -race ./internal/bot`。
- [ ] 运行相关包 vet：`go vet ./internal/bot`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/bot/gateway.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/bot/gateway.go|*BotGateway.handleMessage` 下降或消失且无新增/恶化。
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
