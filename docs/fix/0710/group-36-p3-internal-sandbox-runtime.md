# G36 · internal/sandbox · runtime 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G36 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/sandbox`、`internal/botruntime`、`internal/cli` |
| 涉及文件 | `internal/sandbox/shell.go`<br>`internal/botruntime/runtime.go`<br>`internal/cli/chat_tui_paste.go`<br>`internal/cli/mathnode.go`<br>`internal/cli/review.go`<br>`internal/cli/chat_tui.go` |
| 主要治理目标 | 围绕 `internal/cli/chat_tui.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | sandbox / botruntime / cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 20 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 36 组；等待 G02、G15、G29 的同文件变更合并 |
| 前置依赖 | G02（internal/cli/chat_tui.go）；G15（internal/cli/chat_tui.go）；G29（internal/botruntime/runtime.go） |
| 后续验证重点 | 权限、安全或凭据边界；配置、平台或向后兼容；持久化、迁移或文件格式兼容；会话、状态或恢复语义；协议、序列化或流式事件兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **42/100**，对象 `resolveShell` 的分项为严重度 11.4、集中度 2.3、影响面 8.4、可执行性 20.0；最高超限 2.80x，文件内 1 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 6 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G02、G15、G29。后续同文件组：G39，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-351` | `complexity\|internal/sandbox/shell.go\|resolveShell` | 圈复杂度 | `internal/sandbox/shell.go` | `resolveShell` | 59 | 28/10 | +18（2.80x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `ND-006` | `nesting_depth\|internal/botruntime/runtime.go\|EnabledPlatforms` | 嵌套深度 | `internal/botruntime/runtime.go` | `EnabledPlatforms` | 21 | 5/4 | +1（1.25x） | P3 | 2 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-105` | `complexity\|internal/cli/chat_tui_paste.go\|pastedImagePath` | 圈复杂度 | `internal/cli/chat_tui_paste.go` | `pastedImagePath` | 235 | 15/10 | +5（1.50x） | P3 | 3 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-140` | `complexity\|internal/cli/mathnode.go\|*mathParser.Parse` | 圈复杂度 | `internal/cli/mathnode.go` | `*mathParser.Parse` | 26 | 14/10 | +4（1.40x） | P3 | 1 可信 + 14 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-157` | `complexity\|internal/cli/review.go\|reviewCommand` | 圈复杂度 | `internal/cli/review.go` | `reviewCommand` | 19 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 9 可信 + 3 待核 callee / 18 unresolved | 待修改 |
| `CX-118` | `complexity\|internal/cli/chat_tui.go\|*chatTUI.runMCPSubcommand` | 圈复杂度 | `internal/cli/chat_tui.go` | `*chatTUI.runMCPSubcommand` | 3492 | 19/10 | +9（1.90x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 3 待核 callee / 20 unresolved | 待修改 |
| `CX-096` | `complexity\|internal/botruntime/runtime.go\|ChannelConfigs` | 圈复杂度 | `internal/botruntime/runtime.go` | `ChannelConfigs` | 110 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-097` | `complexity\|internal/botruntime/runtime.go\|ConnectionChannelConfigs` | 圈复杂度 | `internal/botruntime/runtime.go` | `ConnectionChannelConfigs` | 145 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-095` | `complexity\|internal/botruntime/runtime.go\|PlatformConfigured` | 圈复杂度 | `internal/botruntime/runtime.go` | `PlatformConfigured` | 84 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-100` | `complexity\|internal/botruntime/runtime.go\|rememberAllowlist` | 圈复杂度 | `internal/botruntime/runtime.go` | `rememberAllowlist` | 496 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-351 · 圈复杂度 · `resolveShell`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-351 |
| Baseline key | `complexity\|internal/sandbox/shell.go\|resolveShell` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 42） |
| 文件 | `internal/sandbox/shell.go` |
| 代码行 | 59 |
| 函数/方法/包 | `resolveShell` |
| Baseline / 当前复测 / 阈值 | 28 / 28 / 10 |
| 超出 | +18，2.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`resolveShell` 的圈复杂度当前为 **28**，超过限制 **10**（+18，2.80x）。定义范围为 59-131 行；可见控制流热点：L61 `if p, err := lookPath("bash"); err == nil && !isWSL(p) && probe(p) {`；L73 `for _, p := range winPowerShellCandidates {`；L89 `if sh, ok := findBash(); ok {`；L101 `case "", "auto":`；L112 `case "powershell", "pwsh":`；L126 `if warn != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function resolveShell（internal/sandbox/shell.go:59-131）
- **可信生产调用方（1）：** `ResolveShell`（`internal/sandbox/shell.go:51`，调用点 L52，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `warnMissingShell`（`internal/sandbox/shell.go:133`，调用点 L110/L123，confidence=0.90，exact-match，2 条边）；`pathBase`（`internal/sandbox/shell.go:179`，调用点 L74，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Build`（`internal/boot/boot.go:152`，调用点 L362，confidence=0.90，import，待源码复核）；`Controller::RunShell`（`internal/control/controller.go:1063`，调用点 L1072，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/sandbox`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/sandbox/sandbox_test.go`
- **同目录测试候选：** `internal/sandbox/seatbelt_darwin_test.go`、`internal/sandbox/shell_nul_test.go`
- **需同步检查：** `internal/sandbox/shell.go`、`internal/sandbox/sandbox_test.go`、`internal/sandbox/seatbelt_darwin_test.go`、`internal/sandbox/shell_nul_test.go`

#### 待修改项

- [ ] 逐段核对 L61、L75、L93、L107、L126 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `resolveShell` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/sandbox/shell.go|resolveShell` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/sandbox/shell.go:59-131` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `resolveShell` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/sandbox/sandbox_test.go`。
- [ ] 运行相关包测试：`go test ./internal/sandbox`。
- [ ] 运行相关包 vet：`go vet ./internal/sandbox`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/sandbox/shell.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/sandbox/shell.go|resolveShell` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-006 · 嵌套深度 · `EnabledPlatforms`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-006 |
| Baseline key | `nesting_depth\|internal/botruntime/runtime.go\|EnabledPlatforms` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/botruntime/runtime.go` |
| 代码行 | 21 |
| 函数/方法/包 | `EnabledPlatforms` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`EnabledPlatforms` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L24 `if len(channels) > 0 {`；L25 `for _, ch := range channels {`；L27 `switch bot.Platform(ch) {`；L30 `case bot.PlatformFeishu:`；L32 `case bot.PlatformWeixin:`；L35 `if strings.EqualFold(ch, "lark") {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function EnabledPlatforms（internal/botruntime/runtime.go:21-48）
- **可信生产调用方（2）：** `desktopBotRuntimePlan`（`desktop/bot_runtime_app.go:224`，调用点 L234，confidence=0.90，import）；`botStart`（`internal/cli/bot.go:47`，调用点 L86，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `PlatformConfigured`（`internal/botruntime/runtime.go:84`，调用点 L29/L31/L33/L36，confidence=0.90，exact-match，7 条边）；`Platform`（`internal/bot/types.go:8`，调用点 L27，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L102，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/botruntime`、`desktop`、`internal/cli`、`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/botruntime/runtime_test.go`
- **需同步检查：** `internal/botruntime/runtime.go`、`desktop/bot_runtime_app.go`、`internal/cli/bot.go`、`internal/bot/types.go`、`internal/botruntime/runtime_test.go`

#### 待修改项

- [ ] 从 L24、L27、L28、L32、L35 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/botruntime/runtime.go|EnabledPlatforms` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/botruntime/runtime.go:21-48` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `EnabledPlatforms` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/botruntime/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/botruntime`。
- [ ] 运行相关包 vet：`go vet ./internal/botruntime`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/botruntime/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/botruntime/runtime.go|EnabledPlatforms` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-105 · 圈复杂度 · `pastedImagePath`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-105 |
| Baseline key | `complexity\|internal/cli/chat_tui_paste.go\|pastedImagePath` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/cli/chat_tui_paste.go` |
| 代码行 | 235 |
| 函数/方法/包 | `pastedImagePath` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`pastedImagePath` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 235-264 行；可见控制流热点：L240 `if src == "" {`；L243 `if !quoted && strings.ContainsAny(src, " \t\r\n") {`；L247 `if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {`；L252 `if err != nil || u.Path == "" {`；L257 `if strings.HasPrefix(src, "~/") {`；L259 `if err == nil && home != "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function pastedImagePath（internal/cli/chat_tui_paste.go:235-264）
- **可信生产调用方（3）：** `looksLikeImageSource`（`internal/cli/chat_tui_paste.go:204`，调用点 L208，confidence=0.90，exact-match）；`savePastedImageSource`（`internal/cli/chat_tui_paste.go:219`，调用点 L224，confidence=0.90，exact-match）；`pastedFileRef`（`internal/cli/chat_tui_paste.go:271`，调用点 L272，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L825，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui_paste.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `pastedImagePath` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L240、L247、L250、L257、L259 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `pastedImagePath` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui_paste.go|pastedImagePath` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui_paste.go:235-264` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `pastedImagePath` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui_paste.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui_paste.go|pastedImagePath` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-140 · 圈复杂度 · `*mathParser.Parse`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-140 |
| Baseline key | `complexity\|internal/cli/mathnode.go\|*mathParser.Parse` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/cli/mathnode.go` |
| 代码行 | 26 |
| 函数/方法/包 | `*mathParser.Parse` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*mathParser.Parse` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 26-66 行；可见控制流热点：L28 `if len(line) == 0 || line[0] != '$' {`；L33 `if display {`；L39 `if display {`；L48 `if len(bytes.TrimSpace(inner)) == 0 {`；L56 `if !display {`；L58 `if inner[0] == ' ' || inner[len(inner)-1] == ' ' ||`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method mathParser::Parse（internal/cli/mathnode.go:26-66）
- **可信生产调用方（1）：** `mdRenderer::Render`（`internal/cli/md.go:56`，调用点 L62，confidence=0.90，exact-match）
- **待源码复核调用方候选（14）：** `acpCommand`（`internal/cli/acp.go:34`，调用点 L37，confidence=0.70，instance-method，待源码复核）；`botStart`（`internal/cli/bot.go:47`，调用点 L53，confidence=0.70，instance-method，待源码复核）；`botDoctor`（`internal/cli/bot.go:164`，调用点 L168，confidence=0.70，instance-method，待源码复核）；`botWeixinLogin`（`internal/cli/bot.go:319`，调用点 L322，confidence=0.70，instance-method，待源码复核）；`runAgent`（`internal/cli/cli.go:387`，调用点 L398，confidence=0.70，instance-method，待源码复核）；`runServe`（`internal/cli/cli.go:508`，调用点 L519，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `latexToUnicode`（`internal/cli/latex.go:13`，调用点 L65，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2229，confidence=0.70，exact-match，待源码复核）；`Run`（`internal/cli/cli.go:51`，调用点 L110，confidence=0.90，exact-match，待源码复核）；`chatREPL`（`internal/cli/cli.go:652`，调用点 L663，confidence=0.70，instance-method，待源码复核）；`main`（`scripts/code_guard/main.go:72`，调用点 L73，confidence=0.70，exact-match，待源码复核）；`run`（`scripts/code_guard/main.go:76`，调用点 L100，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 10 个文件；`internal/cli`、`scripts`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mathnode.go`、`internal/cli/md.go`、`internal/cli/acp.go`、`internal/cli/bot.go`、`internal/cli/latex.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*mathParser.Parse` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L28、L39、L44、L56、L58 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*mathParser.Parse` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mathnode.go|*mathParser.Parse` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mathnode.go:26-66` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*mathParser.Parse` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mathnode.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mathnode.go|*mathParser.Parse` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-157 · 圈复杂度 · `reviewCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-157 |
| Baseline key | `complexity\|internal/cli/review.go\|reviewCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/cli/review.go` |
| 代码行 | 19 |
| 函数/方法/包 | `reviewCommand` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`reviewCommand` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 19-100 行；可见控制流热点：L25 `if err := fs.Parse(args); err != nil {`；L35 `if diff == "" {`；L47 `if modelName == "" {`；L55 `if err := cfg.Validate(modelName); err != nil {`；L70 `if err != nil {`；L93 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadReviewConfig、loadReviewSkill、mathParser::Parse、Config::NetworkProxySpec）、状态与控制（RunSubAgentWithSession）、渲染/输出（loadReviewConfig、loadReviewSkill、buildReviewSubagentRegistry、getReviewDiff）、网络/协议（NewProviderWithProxy）、任务/并发（buildReviewTask）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function reviewCommand（internal/cli/review.go:19-100）
- **可信生产调用方（1）：** `Run`（`internal/cli/cli.go:51`，调用点 L119，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `NewSession`（`internal/agent/session.go:29`，调用点 L87，confidence=0.90，import）；`RunSubAgentWithSession`（`internal/agent/task.go:804`，调用点 L87，confidence=0.90，import）；`NewProviderWithProxy`（`internal/boot/boot.go:1500`，调用点 L61，confidence=0.90，import）；`defaultCLIActorContext`（`internal/cli/cli.go:225`，调用点 L41，confidence=0.90，exact-match）；`loadReviewConfig`（`internal/cli/review.go:102`，调用点 L41，confidence=0.90，exact-match）；`loadReviewSkill`（`internal/cli/review.go:116`，调用点 L69，confidence=0.90，exact-match）；`buildReviewSubagentRegistry`（`internal/cli/review.go:128`，调用点 L76，confidence=0.90，exact-match）；`getReviewDiff`（`internal/cli/review.go:145`，调用点 L30，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L25，confidence=0.70，instance-method，待源码复核）；`Config::NetworkProxySpec`（`internal/config/config.go:610`，调用点 L61，confidence=0.70，instance-method，待源码复核）；`Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L50，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L119，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 18 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 7 个文件；`internal/cli`、`internal/agent`、`internal/boot`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/review.go`、`internal/cli/cli.go`、`internal/agent/session.go`、`internal/agent/task.go`、`internal/boot/boot.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `reviewCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L25、L42、L51、L70、L93 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `reviewCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/review.go|reviewCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/review.go:19-100` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `reviewCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：协议、序列化或流式事件兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/review.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/review.go|reviewCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-118 · 圈复杂度 · `*chatTUI.runMCPSubcommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-118 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|*chatTUI.runMCPSubcommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 40） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 3492 |
| 函数/方法/包 | `*chatTUI.runMCPSubcommand` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.runMCPSubcommand` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 3492-3563 行；可见控制流热点：L3494 `if len(args) < 2 {`；L3504 `if len(args) < 3 {`；L3515 `if m.mcp != nil {`；L3530 `case "connect":`；L3542 `case "remove", "rm":`；L3558 `case "import":`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseMCPAdd）、持久化/I-O（chatTUI::openMCPManager、chatTUI::openMCPImportPicker）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runMCPSubcommand（internal/cli/chat_tui.go:3492-3563）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3294，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `parseMCPAdd`（`internal/cli/mcp.go:26`，调用点 L3519，confidence=0.90，exact-match）；`tokenizeArgs`（`internal/cli/mcp.go:118`，调用点 L3493，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `chatTUI::openMCPManager`（`internal/cli/mcp_manager.go:98`，调用点 L3495/L3508/L3514，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::showMCPStatus`（`internal/cli/chat_tui.go:3567`，调用点 L3502，confidence=0.70，instance-method，待源码复核）；`chatTUI::openMCPImportPicker`（`internal/cli/mcp_import_picker.go:26`，调用点 L3559，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 20 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/mcp.go`、`internal/cli/mcp_manager.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runMCPSubcommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L3494、L3509、L3520、L3542、L3558 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.runMCPSubcommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|*chatTUI.runMCPSubcommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:3492-3563` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runMCPSubcommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|*chatTUI.runMCPSubcommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-096 · 圈复杂度 · `ChannelConfigs`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-096 |
| Baseline key | `complexity\|internal/botruntime/runtime.go\|ChannelConfigs` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/botruntime/runtime.go` |
| 代码行 | 110 |
| 函数/方法/包 | `ChannelConfigs` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ChannelConfigs` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 110-143 行；可见控制流热点：L111 `if len(connections) == 0 {`；L116 `if !conn.Enabled {`；L121 `case bot.PlatformQQ, bot.PlatformFeishu, bot.PlatformWeixin:`；L126 `if includeModel {`；L132 `if value := normalizeToolApprovalMode(conn.ToolApprovalMode); value != "" {`；L139 `if len(out) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeToolApprovalMode）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ChannelConfigs（internal/botruntime/runtime.go:110-143）
- **可信生产调用方（2）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L121，confidence=0.90，import）；`botStart`（`internal/cli/bot.go:47`，调用点 L107，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `Platform`（`internal/bot/types.go:8`，调用点 L119，confidence=0.90，import）
- **待源码复核被调候选（1）：** `normalizeToolApprovalMode`（`internal/botruntime/runtime.go:178`，调用点 L132，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L121，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/botruntime`、`desktop`、`internal/cli`、`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/botruntime/runtime_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/botruntime/runtime.go`、`desktop/bot_runtime_app.go`、`internal/cli/bot.go`、`internal/bot/types.go`、`internal/botruntime/runtime_test.go`

#### 待修改项

- [ ] 逐段核对 L111、L116、L126、L132、L139 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ChannelConfigs` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/botruntime/runtime.go|ChannelConfigs` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/botruntime/runtime.go:110-143` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ChannelConfigs` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/botruntime/runtime_test.go`。
- [ ] 运行相关包测试：`go test ./internal/botruntime`。
- [ ] 运行相关包 vet：`go vet ./internal/botruntime`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/botruntime/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/botruntime/runtime.go|ChannelConfigs` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-097 · 圈复杂度 · `ConnectionChannelConfigs`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-097 |
| Baseline key | `complexity\|internal/botruntime/runtime.go\|ConnectionChannelConfigs` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/botruntime/runtime.go` |
| 代码行 | 145 |
| 函数/方法/包 | `ConnectionChannelConfigs` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ConnectionChannelConfigs` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 145-176 行；可见控制流热点：L146 `if len(connections) == 0 {`；L151 `if !conn.Enabled {`；L155 `if id == "" {`；L162 `if includeWorkspaceRoot {`；L165 `if value := normalizeToolApprovalMode(conn.ToolApprovalMode); value != "" {`；L172 `if len(out) == 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeToolApprovalMode）、状态与控制（ConnectionRuntimeID）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ConnectionChannelConfigs（internal/botruntime/runtime.go:145-176）
- **可信生产调用方（2）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L122，confidence=0.90，import）；`botStart`（`internal/cli/bot.go:47`，调用点 L108，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `ConnectionRuntimeID`（`internal/botruntime/runtime.go:252`，调用点 L154，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `normalizeToolApprovalMode`（`internal/botruntime/runtime.go:178`，调用点 L165，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `desktopBotRuntime::apply`（`desktop/bot_runtime_app.go:95`，调用点 L122，confidence=0.90，import，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/botruntime`、`desktop`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/botruntime/runtime_test.go`、`internal/cli/bot_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/botruntime/runtime.go`、`desktop/bot_runtime_app.go`、`internal/cli/bot.go`、`internal/botruntime/runtime_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 逐段核对 L146、L151、L159、L165、L172 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ConnectionChannelConfigs` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/botruntime/runtime.go|ConnectionChannelConfigs` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/botruntime/runtime.go:145-176` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ConnectionChannelConfigs` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：权限、安全或凭据边界；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/botruntime/runtime_test.go`、`internal/cli/bot_test.go`。
- [ ] 运行相关包测试：`go test ./internal/botruntime`。
- [ ] 运行相关包 vet：`go vet ./internal/botruntime`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/botruntime/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/botruntime/runtime.go|ConnectionChannelConfigs` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-095 · 圈复杂度 · `PlatformConfigured`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-095 |
| Baseline key | `complexity\|internal/botruntime/runtime.go\|PlatformConfigured` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/botruntime/runtime.go` |
| 代码行 | 84 |
| 函数/方法/包 | `PlatformConfigured` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`PlatformConfigured` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 84-108 行；可见控制流热点：L85 `if cfg == nil {`；L89 `case bot.PlatformQQ:`；L93 `case bot.PlatformFeishu:`；L94 `if cfg.Bot.Feishu.Enabled {`；L98 `if cfg.Bot.Weixin.Enabled {`；L103 `if conn.Enabled && bot.Platform(strings.TrimSpace(conn.Provider)) == platform {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function PlatformConfigured（internal/botruntime/runtime.go:84-108）
- **可信生产调用方（1）：** `EnabledPlatforms`（`internal/botruntime/runtime.go:21`，调用点 L29/L31/L33/L36，confidence=0.90，exact-match，7 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `Platform`（`internal/bot/types.go:8`，调用点 L103，confidence=0.90，import）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/botruntime`、`internal/bot`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/botruntime/runtime_test.go`
- **需同步检查：** `internal/botruntime/runtime.go`、`internal/bot/types.go`、`internal/botruntime/runtime_test.go`

#### 待修改项

- [ ] 逐段核对 L85、L89、L94、L98、L103 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `PlatformConfigured` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/botruntime/runtime.go|PlatformConfigured` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/botruntime/runtime.go:84-108` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `PlatformConfigured` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/botruntime/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/botruntime`。
- [ ] 运行相关包 vet：`go vet ./internal/botruntime`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/botruntime/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/botruntime/runtime.go|PlatformConfigured` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-100 · 圈复杂度 · `rememberAllowlist`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-100 |
| Baseline key | `complexity\|internal/botruntime/runtime.go\|rememberAllowlist` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/botruntime/runtime.go` |
| 代码行 | 496 |
| 函数/方法/包 | `rememberAllowlist` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`rememberAllowlist` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 496-529 行；可见控制流热点：L497 `if allowlist == nil {`；L503 `switch platform {`；L506 `case bot.PlatformFeishu:`；L516 `if groupID == "" {`；L521 `case bot.PlatformQQ:`；L525 `case bot.PlatformWeixin:`。CodeGraph 记录的下游职责候选主要涉及 权限/安全（chatUsesGroupAllowlist）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function rememberAllowlist（internal/botruntime/runtime.go:496-529）
- **可信生产调用方（1）：** `rememberInbound`（`internal/botruntime/runtime.go:358`，调用点 L429，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `appendUniqueString`（`internal/botruntime/runtime.go:531`，调用点 L505/L507/L509/L522，confidence=0.90，exact-match，6 条边）
- **待源码复核被调候选（1）：** `chatUsesGroupAllowlist`（`internal/botruntime/runtime.go:544`，调用点 L512，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/botruntime`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/botruntime/runtime_test.go`
- **需同步检查：** `internal/botruntime/runtime.go`、`internal/botruntime/runtime_test.go`

#### 待修改项

- [ ] 逐段核对 L497、L504、L512、L520、L525 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `rememberAllowlist` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/botruntime/runtime.go|rememberAllowlist` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/botruntime/runtime.go:496-529` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `rememberAllowlist` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G39 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/botruntime/runtime_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/botruntime`。
- [ ] 运行相关包 vet：`go vet ./internal/botruntime`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/botruntime/runtime.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/botruntime/runtime.go|rememberAllowlist` 下降或消失且无新增/恶化。
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
