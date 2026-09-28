# G02 · CLI/TUI 状态更新与交互渲染

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G02 |
| 问题数量 | 10 |
| 整体优先级 | P0 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli/chat_tui.go` |
| 主要治理目标 | 围绕 `internal/cli/chat_tui.go` 等治理对象降低复杂度/体量，统一处理 4 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 20 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 2 组；先于 G15、G36、G39 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；持久化、迁移或文件格式兼容；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **75/100**，对象 `chatTUI.update` 的分项为严重度 29.8、集中度 20.0、影响面 19.0、可执行性 6.0；最高超限 17.60x，文件内 20 条问题，CodeGraph 记录 0 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数、嵌套深度，必须合并治理。

### 治理边界与组间关系

本组问题集中在同一文件，建议由单一负责人统一处理。没有同文件前置组。后续同文件组：G15、G36、G39，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-106` | `complexity\|internal/cli/chat_tui.go\|chatTUI.update` | 圈复杂度 | `internal/cli/chat_tui.go` | `chatTUI.update` | 713 | 176/10 | +166（17.60x） | P0 | 0 可信 + 0 待核 caller / 21 可信 + 46 待核 callee / 104 unresolved | 待修改 |
| `FN-018` | `func_lines\|internal/cli/chat_tui.go\|chatTUI.update` | 函数有效行数 | `internal/cli/chat_tui.go` | `chatTUI.update` | 713 | 532/80 | +452（6.65x） | P0 | 0 可信 + 0 待核 caller / 21 可信 + 46 待核 callee / 104 unresolved | 待修改 |
| `ND-007` | `nesting_depth\|internal/cli/chat_tui.go\|chatTUI.update` | 嵌套深度 | `internal/cli/chat_tui.go` | `chatTUI.update` | 713 | 5/4 | +1（1.25x） | P0 | 0 可信 + 0 待核 caller / 21 可信 + 46 待核 callee / 104 unresolved | 待修改 |
| `CX-117` | `complexity\|internal/cli/chat_tui.go\|*chatTUI.runSlashCommand` | 圈复杂度 | `internal/cli/chat_tui.go` | `*chatTUI.runSlashCommand` | 3226 | 50/10 | +40（5.00x） | P1 | 0 可信 + 1 待核 caller / 6 可信 + 30 待核 callee / 33 unresolved | 待修改 |
| `FN-021` | `func_lines\|internal/cli/chat_tui.go\|*chatTUI.runSlashCommand` | 函数有效行数 | `internal/cli/chat_tui.go` | `*chatTUI.runSlashCommand` | 3226 | 162/80 | +82（2.02x） | P1 | 0 可信 + 1 待核 caller / 6 可信 + 30 待核 callee / 33 unresolved | 待修改 |
| `FL-006` | `file_lines\|internal/cli/chat_tui.go\|` | 文件有效行数 | `internal/cli/chat_tui.go` | `internal/cli/chat_tui.go` | 1 | 2809/800 | +2009（3.51x） | P2 | 0 可信 + 0 待核 caller / 20 可信 + 20 待核 callee / 601 unresolved | 待修改 |
| `CX-116` | `complexity\|internal/cli/chat_tui.go\|*chatTUI.ingestEvent` | 圈复杂度 | `internal/cli/chat_tui.go` | `*chatTUI.ingestEvent` | 3004 | 47/10 | +37（4.70x） | P2 | 0 可信 + 1 待核 caller / 10 可信 + 14 待核 callee / 9 unresolved | 待修改 |
| `FN-020` | `func_lines\|internal/cli/chat_tui.go\|*chatTUI.ingestEvent` | 函数有效行数 | `internal/cli/chat_tui.go` | `*chatTUI.ingestEvent` | 3004 | 147/80 | +67（1.84x） | P2 | 0 可信 + 1 待核 caller / 10 可信 + 14 待核 callee / 9 unresolved | 待修改 |
| `CX-112` | `complexity\|internal/cli/chat_tui.go\|chatTUI.View` | 圈复杂度 | `internal/cli/chat_tui.go` | `chatTUI.View` | 2210 | 47/10 | +37（4.70x） | P2 | 0 可信 + 0 待核 caller / 3 可信 + 24 待核 callee / 60 unresolved | 待修改 |
| `FN-019` | `func_lines\|internal/cli/chat_tui.go\|chatTUI.View` | 函数有效行数 | `internal/cli/chat_tui.go` | `chatTUI.View` | 2210 | 174/80 | +94（2.17x） | P2 | 0 可信 + 0 待核 caller / 3 可信 + 24 待核 callee / 60 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-106 · 圈复杂度 · `chatTUI.update`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-106 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|chatTUI.update` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P0（原始 P0，治理对象得分 75） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 713 |
| 函数/方法/包 | `chatTUI.update` |
| Baseline / 当前复测 / 阈值 | 176 / 176 / 10 |
| 超出 | +166，17.60x |
| 同一治理对象关联问题 | `FN-018`、`ND-007` |

#### 明确的问题描述

`chatTUI.update` 的圈复杂度当前为 **176**，超过限制 **10**（+166，17.60x）。定义范围为 713-1374 行；可见控制流热点：L716 `switch msg := msg.(type) {`；L845 `case "pgup":`；L952 `case "up":`；L1083 `case "enter":`；L1253 `case gitStatusMsg:`；L1369 `if _, ok := msg.(tea.KeyPressMsg); ok {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chatTUI::Update、setCurrentController、chatTUI::updateCompletion、chatTUI::startTurnWithRaw）、持久化/I-O（pastedFileRef、FileRefLine、chatTUI::openRewind）、渲染/输出（newMarkdownRenderer、renderTUIBanner、wrapForViewport、renderUserBubble）、网络/协议（fetchBalance、waitForAgentEvent、chatTUI::ingestEvent、chatTUI::cancelRequested）、任务/并发（waitForAgentEvent、chatTUI::navigateQueue、chatTUI::resetQueueNavigation、chatTUI::expandPastedBlocks）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::update（internal/cli/chat_tui.go:713-1374）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（21）：** `finalize`（`internal/cli/chat_tui.go:1379`，调用点 L755/L764/L823/L829，confidence=0.90，exact-match，28 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L867/L1001/L1049/L1085，confidence=0.90，exact-match，5 条边）；`selection::empty`（`internal/cli/transcript.go:72`，调用点 L752/L816/L1017/L1041，confidence=0.90，exact-match,instance-method，4 条边）；`chatTUI::Update`（`internal/cli/chat_tui.go:670`，调用点 L882/L1359/L1365，confidence=0.90，exact-match，3 条边）；`copyToClipboard`（`internal/cli/transcript.go:28`，调用点 L755/L1021/L1045，confidence=0.90，exact-match，3 条边）；`fetchBalance`（`internal/cli/chat_tui.go:408`，调用点 L1228/L1282，confidence=0.90，exact-match，2 条边）；`elapsedTick`（`internal/cli/chat_tui.go:3220`，调用点 L1158/L1353，confidence=0.90，exact-match，2 条边）；`pastedFileRef`（`internal/cli/chat_tui_paste.go:271`，调用点 L825/L1337，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（46）：** `chatTUI::growInputToFit`（`internal/cli/chat_tui.go:2817`，调用点 L827/L833/L884/L1344，confidence=0.70，instance-method，5 条边，待源码复核）；`chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L736/L1152/L1153/L1296，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::updateCompletion`（`internal/cli/complete.go:119`，调用点 L828/L834/L1345/L1370，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::startTurnWithRaw`（`internal/cli/chat_tui.go:2944`，调用点 L1182/L1194/L1318，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::selectedText`（`internal/cli/transcript.go:141`，调用点 L753/L1019/L1043，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::transcriptCaret`（`internal/cli/transcript.go:191`，调用点 L767/L779/L803，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::runStatusline`（`internal/cli/chat_tui.go:348`，调用点 L1229/L1283，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::recallSubmittedInput`（`internal/cli/chat_tui.go:544`，调用点 L957/L965，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 104 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 17 个文件；`internal/cli`、`internal/automation`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/automation/business.go`、`internal/cli/transcript.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.update` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L716、L865、L1014、L1227、L1369 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.update` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-106、FN-018、ND-007 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|chatTUI.update` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:713-1374` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.update` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-106`、`FN-018`、`ND-007` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|chatTUI.update` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-018 · 函数有效行数 · `chatTUI.update`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-018 |
| Baseline key | `func_lines\|internal/cli/chat_tui.go\|chatTUI.update` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P0（原始 P0，治理对象得分 75） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 713 |
| 函数/方法/包 | `chatTUI.update` |
| Baseline / 当前复测 / 阈值 | 532 / 532 / 80 |
| 超出 | +452，6.65x |
| 同一治理对象关联问题 | `CX-106`、`ND-007` |

#### 明确的问题描述

`chatTUI.update` 的有效代码行 **532**，超过限制 **80**（+452）。CodeGraph 定义物理范围为 713-1374 行，并记录 67 个直接下游候选；候选职责分布为 状态与控制（chatTUI::Update、setCurrentController、chatTUI::updateCompletion、chatTUI::startTurnWithRaw）、持久化/I-O（pastedFileRef、FileRefLine、chatTUI::openRewind）、渲染/输出（newMarkdownRenderer、renderTUIBanner、wrapForViewport、renderUserBubble）、网络/协议（fetchBalance、waitForAgentEvent、chatTUI::ingestEvent、chatTUI::cancelRequested）、任务/并发（waitForAgentEvent、chatTUI::navigateQueue、chatTUI::resetQueueNavigation、chatTUI::expandPastedBlocks）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::update（internal/cli/chat_tui.go:713-1374）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（21）：** `finalize`（`internal/cli/chat_tui.go:1379`，调用点 L755/L764/L823/L829，confidence=0.90，exact-match，28 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L867/L1001/L1049/L1085，confidence=0.90，exact-match，5 条边）；`selection::empty`（`internal/cli/transcript.go:72`，调用点 L752/L816/L1017/L1041，confidence=0.90，exact-match,instance-method，4 条边）；`chatTUI::Update`（`internal/cli/chat_tui.go:670`，调用点 L882/L1359/L1365，confidence=0.90，exact-match，3 条边）；`copyToClipboard`（`internal/cli/transcript.go:28`，调用点 L755/L1021/L1045，confidence=0.90，exact-match，3 条边）；`fetchBalance`（`internal/cli/chat_tui.go:408`，调用点 L1228/L1282，confidence=0.90，exact-match，2 条边）；`elapsedTick`（`internal/cli/chat_tui.go:3220`，调用点 L1158/L1353，confidence=0.90，exact-match，2 条边）；`pastedFileRef`（`internal/cli/chat_tui_paste.go:271`，调用点 L825/L1337，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（46）：** `chatTUI::growInputToFit`（`internal/cli/chat_tui.go:2817`，调用点 L827/L833/L884/L1344，confidence=0.70，instance-method，5 条边，待源码复核）；`chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L736/L1152/L1153/L1296，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::updateCompletion`（`internal/cli/complete.go:119`，调用点 L828/L834/L1345/L1370，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::startTurnWithRaw`（`internal/cli/chat_tui.go:2944`，调用点 L1182/L1194/L1318，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::selectedText`（`internal/cli/transcript.go:141`，调用点 L753/L1019/L1043，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::transcriptCaret`（`internal/cli/transcript.go:191`，调用点 L767/L779/L803，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::runStatusline`（`internal/cli/chat_tui.go:348`，调用点 L1229/L1283，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::recallSubmittedInput`（`internal/cli/chat_tui.go:544`，调用点 L957/L965，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 104 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 17 个文件；`internal/cli`、`internal/automation`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/automation/business.go`、`internal/cli/transcript.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.update` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `chatTUI.update` 的 532 个有效行按 状态与控制、持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L723 `Commit the banner — and a resumed session's transcript — once, now`、L919 `(↑/↓ move, Tab/Enter accept, Esc close); everything else falls through`、L1080 `Ask/Auto are explicit choices, and YOLO is a separate Ctrl+Y toggle.`、L1368 `Re-filter the autocomplete menu against the freshly-edited input.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-106、FN-018、ND-007 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/chat_tui.go|chatTUI.update` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:713-1374` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.update` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-106`、`FN-018`、`ND-007` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/chat_tui.go|chatTUI.update` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-007 · 嵌套深度 · `chatTUI.update`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-007 |
| Baseline key | `nesting_depth\|internal/cli/chat_tui.go\|chatTUI.update` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P0（原始 P3，治理对象得分 75） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 713 |
| 函数/方法/包 | `chatTUI.update` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-106`、`FN-018` |

#### 明确的问题描述

`chatTUI.update` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L716 `switch msg := msg.(type) {`；L845 `case "pgup":`；L952 `case "up":`；L1083 `case "enter":`；L1253 `case gitStatusMsg:`；L1369 `if _, ok := msg.(tea.KeyPressMsg); ok {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::update（internal/cli/chat_tui.go:713-1374）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（21）：** `finalize`（`internal/cli/chat_tui.go:1379`，调用点 L755/L764/L823/L829，confidence=0.90，exact-match，28 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L867/L1001/L1049/L1085，confidence=0.90，exact-match，5 条边）；`selection::empty`（`internal/cli/transcript.go:72`，调用点 L752/L816/L1017/L1041，confidence=0.90，exact-match,instance-method，4 条边）；`chatTUI::Update`（`internal/cli/chat_tui.go:670`，调用点 L882/L1359/L1365，confidence=0.90，exact-match，3 条边）；`copyToClipboard`（`internal/cli/transcript.go:28`，调用点 L755/L1021/L1045，confidence=0.90，exact-match，3 条边）；`fetchBalance`（`internal/cli/chat_tui.go:408`，调用点 L1228/L1282，confidence=0.90，exact-match，2 条边）；`elapsedTick`（`internal/cli/chat_tui.go:3220`，调用点 L1158/L1353，confidence=0.90，exact-match，2 条边）；`pastedFileRef`（`internal/cli/chat_tui_paste.go:271`，调用点 L825/L1337，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（46）：** `chatTUI::growInputToFit`（`internal/cli/chat_tui.go:2817`，调用点 L827/L833/L884/L1344，confidence=0.70，instance-method，5 条边，待源码复核）；`chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L736/L1152/L1153/L1296，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::updateCompletion`（`internal/cli/complete.go:119`，调用点 L828/L834/L1345/L1370，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::startTurnWithRaw`（`internal/cli/chat_tui.go:2944`，调用点 L1182/L1194/L1318，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::selectedText`（`internal/cli/transcript.go:141`，调用点 L753/L1019/L1043，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::transcriptCaret`（`internal/cli/transcript.go:191`，调用点 L767/L779/L803，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::runStatusline`（`internal/cli/chat_tui.go:348`，调用点 L1229/L1283，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::recallSubmittedInput`（`internal/cli/chat_tui.go:544`，调用点 L957/L965，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 104 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 17 个文件；`internal/cli`、`internal/automation`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/automation/business.go`、`internal/cli/transcript.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.update` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 从 L716、L865、L1014、L1227、L1369 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-106、FN-018、ND-007 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/cli/chat_tui.go|chatTUI.update` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:713-1374` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.update` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-106`、`FN-018`、`ND-007` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/cli/chat_tui.go|chatTUI.update` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-117 · 圈复杂度 · `*chatTUI.runSlashCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-117 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|*chatTUI.runSlashCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P0，治理对象得分 58） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 3226 |
| 函数/方法/包 | `*chatTUI.runSlashCommand` |
| Baseline / 当前复测 / 阈值 | 50 / 50 / 10 |
| 超出 | +40，5.00x |
| 同一治理对象关联问题 | `FN-021` |

#### 明确的问题描述

`*chatTUI.runSlashCommand` 的圈复杂度当前为 **50**，超过限制 **10**（+40，5.00x）。定义范围为 3226-3400 行；可见控制流热点：L3229 `if strings.HasPrefix(cmd, "/mcp__") {`；L3266 `case "/sandbox":`；L3292 `case "/mcp":`；L3322 `if m.ctrl.Running() {`；L3361 `case "/help":`；L3394 `if sent, ok := m.ctrl.RunSkill(input); ok {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（RunLegacyRescue、chatTUI::startTurn、chatTUI::runAutoPlanCommand、chatTUI::runBranchCommand）、持久化/I-O（chatTUI::openRewind）、渲染/输出（renderOutputStyles、chatTUI::resetFreshContextView）、网络/协议（chatTUI::runProviderCommand）、权限/安全（chatTUI::showSandboxStatus）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runSlashCommand（internal/cli/chat_tui.go:3226-3400）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `pasteClipboardImage`（`internal/cli/chat_tui_paste.go:109`，调用点 L3337，confidence=0.90，exact-match）；`renderOutputStyles`（`internal/cli/output_style_view.go:10`，调用点 L3344，confidence=0.90，exact-match）；`FuncSink`（`internal/event/event.go:366`，调用点 L3369，confidence=0.90，import）；`RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L3369，confidence=0.90，import）；`Dirs`（`internal/outputstyle/outputstyle.go:64`，调用点 L3340，confidence=0.90，import）；`List`（`internal/outputstyle/outputstyle.go:84`，调用点 L3340，confidence=0.90，import）
- **待源码复核被调候选（30）：** `chatTUI::echoLocalCommand`（`internal/cli/chat_tui.go:3433`，调用点 L3235/L3244/L3253/L3260，confidence=0.70，instance-method，25 条边，待源码复核）；`chatTUI::startTurn`（`internal/cli/chat_tui.go:2937`，调用点 L3392/L3395，confidence=0.70，instance-method，2 条边，待源码复核）；`acpFactory::NewSession`（`internal/cli/acp.go:68`，调用点 L3245，confidence=0.70，exact-match，待源码复核）；`chatTUI::runAutoPlanCommand`（`internal/cli/autoplan.go:10`，调用点 L3273，confidence=0.70，instance-method，待源码复核）；`chatTUI::showBranchTree`（`internal/cli/branch.go:13`，调用点 L3285，confidence=0.70，instance-method，待源码复核）；`chatTUI::runBranchCommand`（`internal/cli/branch.go:63`，调用点 L3288，confidence=0.70，instance-method，待源码复核）；`chatTUI::runSwitchCommand`（`internal/cli/branch.go:86`，调用点 L3291，confidence=0.70，instance-method，待源码复核）；`chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L3344，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 33 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 24 个文件；`internal/cli`、`internal/event`、`internal/migration`、`internal/outputstyle`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/chat_tui_paste.go`、`internal/cli/output_style_view.go`、`internal/event/event.go`、`internal/migration/migration.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runSlashCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L3229、L3271、L3307、L3355、L3394 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.runSlashCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-117、FN-021 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|*chatTUI.runSlashCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:3226-3400` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runSlashCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-117`、`FN-021` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|*chatTUI.runSlashCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-021 · 函数有效行数 · `*chatTUI.runSlashCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-021 |
| Baseline key | `func_lines\|internal/cli/chat_tui.go\|*chatTUI.runSlashCommand` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P1（原始 P1，治理对象得分 58） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 3226 |
| 函数/方法/包 | `*chatTUI.runSlashCommand` |
| Baseline / 当前复测 / 阈值 | 162 / 162 / 80 |
| 超出 | +82，2.02x |
| 同一治理对象关联问题 | `CX-117` |

#### 明确的问题描述

`*chatTUI.runSlashCommand` 的有效代码行 **162**，超过限制 **80**（+82）。CodeGraph 定义物理范围为 3226-3400 行，并记录 36 个直接下游候选；候选职责分布为 状态与控制（RunLegacyRescue、chatTUI::startTurn、chatTUI::runAutoPlanCommand、chatTUI::runBranchCommand）、持久化/I-O（chatTUI::openRewind）、渲染/输出（renderOutputStyles、chatTUI::resetFreshContextView）、网络/协议（chatTUI::runProviderCommand）、权限/安全（chatTUI::showSandboxStatus）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runSlashCommand（internal/cli/chat_tui.go:3226-3400）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `pasteClipboardImage`（`internal/cli/chat_tui_paste.go:109`，调用点 L3337，confidence=0.90，exact-match）；`renderOutputStyles`（`internal/cli/output_style_view.go:10`，调用点 L3344，confidence=0.90，exact-match）；`FuncSink`（`internal/event/event.go:366`，调用点 L3369，confidence=0.90，import）；`RunLegacyRescue`（`internal/migration/migration.go:78`，调用点 L3369，confidence=0.90，import）；`Dirs`（`internal/outputstyle/outputstyle.go:64`，调用点 L3340，confidence=0.90，import）；`List`（`internal/outputstyle/outputstyle.go:84`，调用点 L3340，confidence=0.90，import）
- **待源码复核被调候选（30）：** `chatTUI::echoLocalCommand`（`internal/cli/chat_tui.go:3433`，调用点 L3235/L3244/L3253/L3260，confidence=0.70，instance-method，25 条边，待源码复核）；`chatTUI::startTurn`（`internal/cli/chat_tui.go:2937`，调用点 L3392/L3395，confidence=0.70，instance-method，2 条边，待源码复核）；`acpFactory::NewSession`（`internal/cli/acp.go:68`，调用点 L3245，confidence=0.70，exact-match，待源码复核）；`chatTUI::runAutoPlanCommand`（`internal/cli/autoplan.go:10`，调用点 L3273，confidence=0.70，instance-method，待源码复核）；`chatTUI::showBranchTree`（`internal/cli/branch.go:13`，调用点 L3285，confidence=0.70，instance-method，待源码复核）；`chatTUI::runBranchCommand`（`internal/cli/branch.go:63`，调用点 L3288，confidence=0.70，instance-method，待源码复核）；`chatTUI::runSwitchCommand`（`internal/cli/branch.go:86`，调用点 L3291，confidence=0.70，instance-method，待源码复核）；`chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L3344，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 33 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 24 个文件；`internal/cli`、`internal/event`、`internal/migration`、`internal/outputstyle`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/chat_tui_paste.go`、`internal/cli/output_style_view.go`、`internal/event/event.go`、`internal/migration/migration.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runSlashCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*chatTUI.runSlashCommand` 的 162 个有效行按 状态与控制、持久化/I-O、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L3236 `Compaction makes a (network) summarizer call; run it off the Update loop`、L3238 `card as they arrive; compactDoneMsg only handles the terminal error /`、L3249 `Native scrollback keeps the old transcript; mark the fork with a fresh banner.`、L3390 `A custom command wins over a skill of the same name; both resolve to a turn.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-117、FN-021 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/chat_tui.go|*chatTUI.runSlashCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:3226-3400` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runSlashCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：持久化、迁移或文件格式兼容；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-117`、`FN-021` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/chat_tui.go|*chatTUI.runSlashCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FL-006 · 文件有效行数 · `internal/cli/chat_tui.go`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FL-006 |
| Baseline key | `file_lines\|internal/cli/chat_tui.go\|` |
| 规则 | file_lines（文件有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 57） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 1 |
| 函数/方法/包 | `internal/cli/chat_tui.go` |
| Baseline / 当前复测 / 阈值 | 2809 / 2809 / 800 |
| 超出 | +2009，3.51x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`internal/cli/chat_tui.go` 当前有效代码行 **2809**，超过限制 **800**（+2009）。CodeGraph 在文件中索引到的主要大符号包括 chatTUI::update@713-1374、chatTUI::View@2210-2413、chatTUI::ingestEvent@3004-3206、chatTUI::runSlashCommand@3226-3400、chatTUI::computeStatusLineCount@2717-2799、chatTUI::streamToolOutput@1672-1746；文件同时承载的调用影响覆盖 23 个文件。文件拆分必须围绕稳定职责，而不是按行数机械移动函数。

#### CodeGraph 代码定位

- **定义映射：** CodeGraph 在该文件中索引到 96 个函数/方法节点。
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（20）：** `dim`（`internal/cli/style.go:49`，调用点 L2278/L2280/L2311/L3046，confidence=0.90，exact-match，44 条边）；`connectorBlock`（`internal/cli/toolcard.go:19`，调用点 L1744/L1873/L1879/L1882，confidence=0.90，exact-match，12 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L867/L1001/L1049/L1085，confidence=0.90，exact-match，10 条边）；`clampPlain`（`internal/cli/diffview.go:205`，调用点 L1742/L1870/L1877/L1920，confidence=0.90，exact-match，7 条边）；`Server::answer`（`internal/serve/serve.go:784`，调用点 L2141/L2143/L2145/L2149，confidence=0.90，exact-match，7 条边）；`themeStyle`（`internal/cli/theme.go:356`，调用点 L458/L2454/L2456/L2470，confidence=0.90，exact-match，6 条边）；`accent`（`internal/cli/style.go:53`，调用点 L2638/L2428/L3664/L3636，confidence=0.90，exact-match，4 条边）；`selection::empty`（`internal/cli/transcript.go:72`，调用点 L752/L816/L1017/L1041，confidence=0.90，exact-match，4 条边）
- **待源码复核被调候选（20）：** `mdRenderer::Render`（`internal/cli/md.go:56`，调用点 L2229/L2246/L2363/L2378，confidence=0.70，exact-match，11 条边，待源码复核）；`chatTUI::updateCompletion`（`internal/cli/complete.go:119`，调用点 L828/L834/L1345/L1370，confidence=0.70，instance-method，5 条边，待源码复核）；`chatTUI::openMCPManager`（`internal/cli/mcp_manager.go:98`，调用点 L3495/L3508/L3514，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::selectedText`（`internal/cli/transcript.go:141`，调用点 L753/L1019/L1043，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::transcriptCaret`（`internal/cli/transcript.go:191`，调用点 L767/L779/L803，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::attachPastedImages`（`internal/cli/chat_tui_paste.go:133`，调用点 L822/L1334，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::shouldFoldPaste`（`internal/cli/chat_tui_paste.go:43`，调用点 L831/L1339，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::insertFoldedPaste`（`internal/cli/chat_tui_paste.go:47`，调用点 L832/L1340，confidence=0.70，instance-method，2 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 601 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 23 个文件；`internal/cli`、`internal/automation`、`internal/serve`、`internal/permission`、`internal/plugin`、`internal/agent`、`internal/billing`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`、`internal/cli/statusline_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/cli/toolcard.go`、`internal/cli/md.go`、`internal/automation/business.go`、`internal/cli/chat_tui_test.go`、`internal/cli/statusline_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`

#### 待修改项

- [ ] 先冻结 `internal/cli/chat_tui.go` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 围绕主要符号 `chatTUI::update`、`chatTUI::View`、`chatTUI::ingestEvent`、`chatTUI::runSlashCommand`、`chatTUI::computeStatusLineCount` 识别职责簇，确定哪些类型与函数应继续共置、哪些可以成为独立文件。
- [ ] 先拆职责边界，再移动实现；禁止通过创建大量单函数文件降低文件行数。
- [ ] 同步检查该文件中的其他 Code Guard 问题 ID：CX-106、CX-107、CX-108、CX-109、CX-110、CX-111、CX-112、CX-113、CX-114、CX-115、CX-116、CX-117、CX-118、FN-018、FN-019、FN-020、FN-021、ND-007、ND-008。
- [ ] 修复后重新扫描并显式确认 baseline key `file_lines|internal/cli/chat_tui.go|` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:1` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `internal/cli/chat_tui.go` 对外可见类型和接口，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：并发、取消或事件顺序；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`、`internal/cli/statusline_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `file_lines|internal/cli/chat_tui.go|` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-116 · 圈复杂度 · `*chatTUI.ingestEvent`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-116 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|*chatTUI.ingestEvent` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 57） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 3004 |
| 函数/方法/包 | `*chatTUI.ingestEvent` |
| Baseline / 当前复测 / 阈值 | 47 / 47 / 10 |
| 超出 | +37，4.70x |
| 同一治理对象关联问题 | `FN-020` |

#### 明确的问题描述

`*chatTUI.ingestEvent` 的圈复杂度当前为 **47**，超过限制 **10**（+37，4.70x）。定义范围为 3004-3206 行；可见控制流热点：L3005 `if e.Kind == event.Retrying {`；L3053 `case event.Text:`；L3079 `for _, ln := range block {`；L3117 `if e.Level == event.LevelWarn {`；L3149 `case event.CompactionDone:`；L3199 `if e.Err != nil && e.Err.Error() != "" && !strings.Contains(e.Err.Error(), "context canceled") {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（truncateSubject、chatTUI::collapseToolOutput、chatTUI::beginToolRunning、chatTUI::clearSubmittedPastes）、渲染/输出（FormatUsageLine、wrapForViewport）、网络/协议（chatTUI::finalizeStreamed、chatTUI::streamReasoning、chatTUI::streamToolOutput、chatTUI::streamAnswer）、任务/并发（diffBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::ingestEvent（internal/cli/chat_tui.go:3004-3206）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200/L1213，confidence=0.70，instance-method，2 条边，待源码复核）
- **可信被调用函数（10）：** `dim`（`internal/cli/style.go:49`，调用点 L3046/L3147，confidence=0.90，exact-match，2 条边）；`red`（`internal/cli/style.go:51`，调用点 L3103，confidence=0.90，exact-match，2 条边）；`FormatUsageLine`（`internal/agent/textsink.go:185`，调用点 L3110，confidence=0.90，import）；`compactionCardLines`（`internal/cli/chat_tui.go:2419`，调用点 L3156，confidence=0.90，exact-match）；`truncateSubject`（`internal/cli/chat_tui.go:2692`，调用点 L3128，confidence=0.90，exact-match）；`wrapForViewport`（`internal/cli/chat_tui.go:3645`，调用点 L3200，confidence=0.90，exact-match）；`newChooser`（`internal/cli/chooser.go:29`，调用点 L3176，confidence=0.90，exact-match）；`diffBlock`（`internal/cli/diffview.go:62`，调用点 L3078，confidence=0.90，exact-match）
- **待源码复核被调候选（14）：** `chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L3046/L3048/L3080/L3084，confidence=0.70，instance-method，13 条边，待源码复核）；`chatTUI::finalizeStreamed`（`internal/cli/chat_tui.go:3210`，调用点 L3069/L3102/L3111/L3120，confidence=0.70，instance-method，9 条边，待源码复核）；`chatTUI::commitReasoning`（`internal/cli/chat_tui.go:2002`，调用点 L3054/L3060/L3189，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::commitSpacer`（`internal/cli/chat_tui.go:1454`，调用点 L3043/L3077，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::streamReasoning`（`internal/cli/chat_tui.go:1618`，调用点 L3036/L3051，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::commitPending`（`internal/cli/chat_tui.go:2070`，调用点 L3061/L3190，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::confirmBubbleSent`（`internal/cli/chat_tui.go:2974`，调用点 L3027/L3194，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::streamToolOutput`（`internal/cli/chat_tui.go:1672`，调用点 L3089，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/cli`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/agent/textsink.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.ingestEvent` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L3005、L3063、L3106、L3139、L3199 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、渲染/输出、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.ingestEvent` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-116、FN-020 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|*chatTUI.ingestEvent` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:3004-3206` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.ingestEvent` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-116`、`FN-020` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|*chatTUI.ingestEvent` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-020 · 函数有效行数 · `*chatTUI.ingestEvent`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-020 |
| Baseline key | `func_lines\|internal/cli/chat_tui.go\|*chatTUI.ingestEvent` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 57） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 3004 |
| 函数/方法/包 | `*chatTUI.ingestEvent` |
| Baseline / 当前复测 / 阈值 | 147 / 147 / 80 |
| 超出 | +67，1.84x |
| 同一治理对象关联问题 | `CX-116` |

#### 明确的问题描述

`*chatTUI.ingestEvent` 的有效代码行 **147**，超过限制 **80**（+67）。CodeGraph 定义物理范围为 3004-3206 行，并记录 24 个直接下游候选；候选职责分布为 状态与控制（truncateSubject、chatTUI::collapseToolOutput、chatTUI::beginToolRunning、chatTUI::clearSubmittedPastes）、渲染/输出（FormatUsageLine、wrapForViewport）、网络/协议（chatTUI::finalizeStreamed、chatTUI::streamReasoning、chatTUI::streamToolOutput、chatTUI::streamAnswer）、任务/并发（diffBlock）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::ingestEvent（internal/cli/chat_tui.go:3004-3206）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200/L1213，confidence=0.70，instance-method，2 条边，待源码复核）
- **可信被调用函数（10）：** `dim`（`internal/cli/style.go:49`，调用点 L3046/L3147，confidence=0.90，exact-match，2 条边）；`red`（`internal/cli/style.go:51`，调用点 L3103，confidence=0.90，exact-match，2 条边）；`FormatUsageLine`（`internal/agent/textsink.go:185`，调用点 L3110，confidence=0.90，import）；`compactionCardLines`（`internal/cli/chat_tui.go:2419`，调用点 L3156，confidence=0.90，exact-match）；`truncateSubject`（`internal/cli/chat_tui.go:2692`，调用点 L3128，confidence=0.90，exact-match）；`wrapForViewport`（`internal/cli/chat_tui.go:3645`，调用点 L3200，confidence=0.90，exact-match）；`newChooser`（`internal/cli/chooser.go:29`，调用点 L3176，confidence=0.90，exact-match）；`diffBlock`（`internal/cli/diffview.go:62`，调用点 L3078，confidence=0.90，exact-match）
- **待源码复核被调候选（14）：** `chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L3046/L3048/L3080/L3084，confidence=0.70，instance-method，13 条边，待源码复核）；`chatTUI::finalizeStreamed`（`internal/cli/chat_tui.go:3210`，调用点 L3069/L3102/L3111/L3120，confidence=0.70，instance-method，9 条边，待源码复核）；`chatTUI::commitReasoning`（`internal/cli/chat_tui.go:2002`，调用点 L3054/L3060/L3189，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::commitSpacer`（`internal/cli/chat_tui.go:1454`，调用点 L3043/L3077，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::streamReasoning`（`internal/cli/chat_tui.go:1618`，调用点 L3036/L3051，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::commitPending`（`internal/cli/chat_tui.go:2070`，调用点 L3061/L3190，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::confirmBubbleSent`（`internal/cli/chat_tui.go:2974`，调用点 L3027/L3194，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::streamToolOutput`（`internal/cli/chat_tui.go:1672`，调用点 L3089，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 8 个文件；`internal/cli`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/agent/textsink.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.ingestEvent` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `*chatTUI.ingestEvent` 的 147 个有效行按 状态与控制、渲染/输出、网络/协议 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L3010 `Any other event means the connection got past the retry window (or the turn`、L3072 `The result decides whether this list becomes canonical; dispatch only`、L3167 `ctrl.Approve. At most one prompt is outstanding (the controller`、L3204 `nothing to detect here.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-116、FN-020 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/chat_tui.go|*chatTUI.ingestEvent` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:3004-3206` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.ingestEvent` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-116`、`FN-020` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/chat_tui.go|*chatTUI.ingestEvent` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-112 · 圈复杂度 · `chatTUI.View`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-112 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|chatTUI.View` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P0，治理对象得分 56） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 2210 |
| 函数/方法/包 | `chatTUI.View` |
| Baseline / 当前复测 / 阈值 | 47 / 47 / 10 |
| 超出 | +37，4.70x |
| 同一治理对象关联问题 | `FN-019` |

#### 明确的问题描述

`chatTUI.View` 的圈复杂度当前为 **47**，超过限制 **10**（+37，4.70x）。定义范围为 2210-2413 行；可见控制流热点：L2212 `if boxW < 10 {`；L2259 `case m.mcpImport != nil:`；L2277 `case m.ctrl.AutoApproveTools():`；L2324 `if todo := m.renderTodoPanel(); todo != "" {`；L2362 `if working != "" {`；L2406 `if cur := m.input.Cursor(); cur != nil {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chatTUI::runningWorkingLine）、渲染/输出（mdRenderer::Render、chatTUI::renderMainManager、chatTUI::renderQueueIndicator、chatTUI::renderMainManagerFooter）、网络/协议（chatTUI::cancelRequested）、任务/并发（chatTUI::renderQueueIndicator、chatTUI::jobsTag）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::View（internal/cli/chat_tui.go:2210-2413）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `wrapStatusLine`（`internal/cli/chat_tui.go:2705`，调用点 L2363/L2370，confidence=0.90，exact-match，3 条边）；`dim`（`internal/cli/style.go:49`，调用点 L2278/L2280/L2311，confidence=0.90，exact-match，3 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L2216，confidence=0.90，exact-match）
- **待源码复核被调候选（24）：** `mdRenderer::Render`（`internal/cli/md.go:56`，调用点 L2229/L2246/L2363/L2378，confidence=0.70，exact-match，4 条边，待源码复核）；`chatTUI::renderMainManager`（`internal/cli/chat_tui.go:1529`，调用点 L2353/L2396，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::cycleHint`（`internal/cli/chat_tui.go:2847`，调用点 L2278/L2280，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::renderQueueIndicator`（`internal/cli/chat_tui.go:627`，调用点 L2372，confidence=0.70，instance-method，待源码复核）；`chatTUI::hideComposer`（`internal/cli/chat_tui.go:1513`，调用点 L2215，confidence=0.70，instance-method，待源码复核）；`chatTUI::renderMainManagerFooter`（`internal/cli/chat_tui.go:1551`，调用点 L2366，confidence=0.70，instance-method，待源码复核）；`chatTUI::renderTranscriptWithMainManager`（`internal/cli/chat_tui.go:1568`，调用点 L2397，confidence=0.70，instance-method，待源码复核）；`chatTUI::cancelRequested`（`internal/cli/chat_tui.go:2170`，调用点 L2217，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 60 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 11 个文件；`internal/cli`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/automation/business.go`、`internal/cli/md.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.View` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2212、L2263、L2304、L2352、L2406 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、渲染/输出、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.View` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-112、FN-019 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|chatTUI.View` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:2210-2413` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.View` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-112`、`FN-019` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|chatTUI.View` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-019 · 函数有效行数 · `chatTUI.View`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-019 |
| Baseline key | `func_lines\|internal/cli/chat_tui.go\|chatTUI.View` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 56） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 2210 |
| 函数/方法/包 | `chatTUI.View` |
| Baseline / 当前复测 / 阈值 | 174 / 174 / 80 |
| 超出 | +94，2.17x |
| 同一治理对象关联问题 | `CX-112` |

#### 明确的问题描述

`chatTUI.View` 的有效代码行 **174**，超过限制 **80**（+94）。CodeGraph 定义物理范围为 2210-2413 行，并记录 27 个直接下游候选；候选职责分布为 状态与控制（chatTUI::runningWorkingLine）、渲染/输出（mdRenderer::Render、chatTUI::renderMainManager、chatTUI::renderQueueIndicator、chatTUI::renderMainManagerFooter）、网络/协议（chatTUI::cancelRequested）、任务/并发（chatTUI::renderQueueIndicator、chatTUI::jobsTag）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::View（internal/cli/chat_tui.go:2210-2413）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `wrapStatusLine`（`internal/cli/chat_tui.go:2705`，调用点 L2363/L2370，confidence=0.90，exact-match，3 条边）；`dim`（`internal/cli/style.go:49`，调用点 L2278/L2280/L2311，confidence=0.90，exact-match，3 条边）；`xlsxCell::Value`（`internal/automation/business.go:1089`，调用点 L2216，confidence=0.90，exact-match）
- **待源码复核被调候选（24）：** `mdRenderer::Render`（`internal/cli/md.go:56`，调用点 L2229/L2246/L2363/L2378，confidence=0.70，exact-match，4 条边，待源码复核）；`chatTUI::renderMainManager`（`internal/cli/chat_tui.go:1529`，调用点 L2353/L2396，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::cycleHint`（`internal/cli/chat_tui.go:2847`，调用点 L2278/L2280，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::renderQueueIndicator`（`internal/cli/chat_tui.go:627`，调用点 L2372，confidence=0.70，instance-method，待源码复核）；`chatTUI::hideComposer`（`internal/cli/chat_tui.go:1513`，调用点 L2215，confidence=0.70，instance-method，待源码复核）；`chatTUI::renderMainManagerFooter`（`internal/cli/chat_tui.go:1551`，调用点 L2366，confidence=0.70，instance-method，待源码复核）；`chatTUI::renderTranscriptWithMainManager`（`internal/cli/chat_tui.go:1568`，调用点 L2397，confidence=0.70，instance-method，待源码复核）；`chatTUI::cancelRequested`（`internal/cli/chat_tui.go:2170`，调用点 L2217，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 60 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 11 个文件；`internal/cli`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/automation/business.go`、`internal/cli/md.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.View` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 把 `chatTUI.View` 的 174 个有效行按 状态与控制、渲染/输出、网络/协议 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L2288 `The spinning "thinking…" indicator is its own line ABOVE the input box (shown`、L2296 `to strand — and computeStatusLineCount reserves the correct height.`、L2361 `instead of being truncated. Padding to full width prevents stale cells.`、L2404 `for PaddingLeft).` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-112、FN-019 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/cli/chat_tui.go|chatTUI.View` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:2210-2413` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.View` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G15、G36、G39 处理。
- 重点保护：并发、取消或事件顺序；协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-112`、`FN-019` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 race：`go test -race ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/cli/chat_tui.go|chatTUI.View` 下降或消失且无新增/恶化。
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
