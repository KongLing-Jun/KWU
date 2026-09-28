# G21 · internal/guardian · complete 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G21 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/guardian`、`internal/cli` |
| 涉及文件 | `internal/guardian/transcript.go`<br>`internal/cli/complete.go`<br>`internal/cli/latex.go`<br>`internal/guardian/guardian.go` |
| 主要治理目标 | 围绕 `internal/cli/complete.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | guardian / cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 涉及核心运行链路 |
| 建议处理顺序 | 总序第 21 组；先于 G30 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；会话、状态或恢复语义；协议、序列化或流式事件兼容；权限、安全或凭据边界 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **48/100**，对象 `renderTranscript` 的分项为严重度 11.6、集中度 9.6、影响面 6.4、可执行性 20.0；最高超限 2.90x，文件内 3 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G30，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-266` | `complexity\|internal/guardian/transcript.go\|renderTranscript` | 圈复杂度 | `internal/guardian/transcript.go` | `renderTranscript` | 77 | 29/10 | +19（2.90x） | P2 | 0 可信 + 1 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `FN-037` | `func_lines\|internal/guardian/transcript.go\|renderTranscript` | 函数有效行数 | `internal/guardian/transcript.go` | `renderTranscript` | 77 | 88/80 | +8（1.10x） | P2 | 0 可信 + 1 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-131` | `complexity\|internal/cli/complete.go\|*chatTUI.fileItems` | 圈复杂度 | `internal/cli/complete.go` | `*chatTUI.fileItems` | 410 | 21/10 | +11（2.10x） | P2 | 0 可信 + 1 待核 caller / 1 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-137` | `complexity\|internal/cli/latex.go\|convertCommand` | 圈复杂度 | `internal/cli/latex.go` | `convertCommand` | 49 | 17/10 | +7（1.70x） | P2 | 1 可信 + 0 待核 caller / 8 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-139` | `complexity\|internal/cli/latex.go\|normalizeMath` | 圈复杂度 | `internal/cli/latex.go` | `normalizeMath` | 255 | 22/10 | +12（2.20x） | P2 | 0 可信 + 1 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-129` | `complexity\|internal/cli/complete.go\|*chatTUI.updateCompletion` | 圈复杂度 | `internal/cli/complete.go` | `*chatTUI.updateCompletion` | 119 | 11/10 | +1（1.10x） | P3 | 0 可信 + 4 待核 caller / 2 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-132` | `complexity\|internal/cli/complete.go\|chatTUI.renderCompletion` | 圈复杂度 | `internal/cli/complete.go` | `chatTUI.renderCompletion` | 652 | 11/10 | +1（1.10x） | P3 | 0 可信 + 2 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-130` | `complexity\|internal/cli/complete.go\|*chatTUI.branchArgItems` | 圈复杂度 | `internal/cli/complete.go` | `*chatTUI.branchArgItems` | 277 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-264` | `complexity\|internal/guardian/guardian.go\|*Session.Review` | 圈复杂度 | `internal/guardian/guardian.go` | `*Session.Review` | 114 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 9 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-265` | `complexity\|internal/guardian/transcript.go\|ExtractTranscript` | 圈复杂度 | `internal/guardian/transcript.go` | `ExtractTranscript` | 34 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-266 · 圈复杂度 · `renderTranscript`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-266 |
| Baseline key | `complexity\|internal/guardian/transcript.go\|renderTranscript` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 48） |
| 文件 | `internal/guardian/transcript.go` |
| 代码行 | 77 |
| 函数/方法/包 | `renderTranscript` |
| Baseline / 当前复测 / 阈值 | 29 / 29 / 10 |
| 超出 | +19，2.90x |
| 同一治理对象关联问题 | `FN-037` |

#### 明确的问题描述

`renderTranscript` 的圈复杂度当前为 **29**，超过限制 **10**（+19，2.90x）。定义范围为 77-182 行；可见控制流热点：L78 `if len(entries) == 0 {`；L108 `if e.Kind == "user" {`；L130 `for i := len(userIdx) - 1; i >= 0; i-- {`；L145 `if included[i] || entries[i].Kind == "user" {`；L166 `for i, r := range all {`；L178 `if omitted {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（truncateText）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function renderTranscript（internal/guardian/transcript.go:77-182）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `FormatTranscript`（`internal/guardian/transcript.go:185`，调用点 L186，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（1）：** `truncateText`（`internal/guardian/transcript.go:204`，调用点 L94，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `estimateTokens`（`internal/guardian/transcript.go:244`，调用点 L96，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/guardian`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/guardian/guardian_test.go`
- **需同步检查：** `internal/guardian/transcript.go`、`internal/guardian/guardian_test.go`

#### 待修改项

- [ ] 逐段核对 L78、L114、L135、L155、L178 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `renderTranscript` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-266、FN-037 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/guardian/transcript.go|renderTranscript` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/guardian/transcript.go:77-182` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `renderTranscript` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-266`、`FN-037` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/guardian/guardian_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/guardian`。
- [ ] 运行相关包 vet：`go vet ./internal/guardian`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/guardian/transcript.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/guardian/transcript.go|renderTranscript` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-037 · 函数有效行数 · `renderTranscript`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-037 |
| Baseline key | `func_lines\|internal/guardian/transcript.go\|renderTranscript` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 48） |
| 文件 | `internal/guardian/transcript.go` |
| 代码行 | 77 |
| 函数/方法/包 | `renderTranscript` |
| Baseline / 当前复测 / 阈值 | 88 / 88 / 80 |
| 超出 | +8，1.10x |
| 同一治理对象关联问题 | `CX-266` |

#### 明确的问题描述

`renderTranscript` 的有效代码行 **88**，超过限制 **80**（+8）。CodeGraph 定义物理范围为 77-182 行，并记录 2 个直接下游候选；候选职责分布为 状态与控制（truncateText）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function renderTranscript（internal/guardian/transcript.go:77-182）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `FormatTranscript`（`internal/guardian/transcript.go:185`，调用点 L186，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（1）：** `truncateText`（`internal/guardian/transcript.go:204`，调用点 L94，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `estimateTokens`（`internal/guardian/transcript.go:244`，调用点 L96，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/guardian`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/guardian/guardian_test.go`
- **需同步检查：** `internal/guardian/transcript.go`、`internal/guardian/guardian_test.go`

#### 待修改项

- [ ] 把 `renderTranscript` 的 88 个有效行按 状态与控制 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 以现有段落注释 L82 `Pre-compute rendered text and estimated token counts for every entry.`、L105 `Find user entry indices.`、L129 `Fill remaining message budget with user entries from newest to oldest.`、L164 `Build the result.` 为候选边界，确认其是否对应真实职责后再提取。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-266、FN-037 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/guardian/transcript.go|renderTranscript` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/guardian/transcript.go:77-182` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `renderTranscript` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-266`、`FN-037` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/guardian/guardian_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/guardian`。
- [ ] 运行相关包 vet：`go vet ./internal/guardian`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/guardian/transcript.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/guardian/transcript.go|renderTranscript` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-131 · 圈复杂度 · `*chatTUI.fileItems`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-131 |
| Baseline key | `complexity\|internal/cli/complete.go\|*chatTUI.fileItems` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 47） |
| 文件 | `internal/cli/complete.go` |
| 代码行 | 410 |
| 函数/方法/包 | `*chatTUI.fileItems` |
| Baseline / 当前复测 / 阈值 | 21 / 21 / 10 |
| 超出 | +11，2.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.fileItems` 的圈复杂度当前为 **21**，超过限制 **10**（+11，2.10x）。定义范围为 410-482 行；可见控制流热点：L413 `if m.ctrl != nil {`；L427 `if err != nil {`；L442 `if !showHidden && strings.HasPrefix(name, ".") {`；L457 `if !strings.Contains(token, "/") {`；L467 `if len(results) > remaining {`；L475 `if len(items) >= maxCompItems {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（chatTUI::searchFileRefs）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::fileItems（internal/cli/complete.go:410-482）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::atItems`（`internal/cli/complete.go:399`，调用点 L403，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `splitPathToken`（`internal/cli/complete.go:510`，调用点 L411，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `chatTUI::searchFileRefs`（`internal/cli/complete.go:486`，调用点 L466，confidence=0.70，instance-method，待源码复核）；`chatTUI::resourceItems`（`internal/cli/complete.go:533`，调用点 L479，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/complete_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/complete.go`、`internal/cli/complete_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.fileItems` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L413、L437、L450、L463、L475 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.fileItems` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/complete.go|*chatTUI.fileItems` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/complete.go:410-482` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.fileItems` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/complete_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/complete.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/complete.go|*chatTUI.fileItems` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-137 · 圈复杂度 · `convertCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-137 |
| Baseline key | `complexity\|internal/cli/latex.go\|convertCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 47） |
| 文件 | `internal/cli/latex.go` |
| 代码行 | 49 |
| 函数/方法/包 | `convertCommand` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`convertCommand` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 49-112 行；可见控制流热点：L51 `if j >= len(rs) {`；L56 `case '\\':`；L72 `switch cmd {`；L80 `if k < len(rs) && rs[k] == '[' {`；L94 `case "left", "right", "big", "Big", "bigg", "Bigg", "bigl", "bigr", "Bigl", "Bigr", "displaystyle",…`；L106 `if sym, ok := symbols[cmd]; ok {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readAtom、readBracket）、渲染/输出（renderFrac、renderSqrt）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function convertCommand（internal/cli/latex.go:49-112）
- **可信生产调用方（1）：** `convertMath`（`internal/cli/latex.go:17`，调用点 L23，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `readAtom`（`internal/cli/latex.go:117`，调用点 L74/L75/L83/L87，confidence=0.90，exact-match，7 条边）；`convertMath`（`internal/cli/latex.go:17`，调用点 L76/L84/L103，confidence=0.90，exact-match，4 条边）；`isASCIILetter`（`internal/cli/latex.go:247`，调用点 L54/L67，confidence=0.90，exact-match，2 条边）；`readBracket`（`internal/cli/latex.go:156`，调用点 L81，confidence=0.90，exact-match）；`renderFrac`（`internal/cli/latex.go:166`，调用点 L76，confidence=0.90，exact-match）；`renderSqrt`（`internal/cli/latex.go:170`，调用点 L84，confidence=0.90，exact-match）；`applyCombining`（`internal/cli/latex.go:192`，调用点 L103，confidence=0.90，exact-match）；`blackboard`（`internal/cli/latex.go:200`，调用点 L92，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/latex.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `convertCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L51、L58、L78、L90、L106 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `convertCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/latex.go|convertCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/latex.go:49-112` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `convertCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/latex.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/latex.go|convertCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-139 · 圈复杂度 · `normalizeMath`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-139 |
| Baseline key | `complexity\|internal/cli/latex.go\|normalizeMath` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 44） |
| 文件 | `internal/cli/latex.go` |
| 代码行 | 255 |
| 函数/方法/包 | `normalizeMath` |
| Baseline / 当前复测 / 阈值 | 22 / 22 / 10 |
| 超出 | +12，2.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`normalizeMath` 的圈复杂度当前为 **22**，超过限制 **10**（+12，2.20x）。定义范围为 255-326 行；可见控制流热点：L263 `for i := 0; i < n; {`；L272 `if r == '\`' && !inFenced {`；L285 `switch rs[i+1] {`；L290 `case '[':`；L304 `case ')':`；L316 `if r == '\n' && inDisplay {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function normalizeMath（internal/cli/latex.go:255-326）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `mdRenderer::Render`（`internal/cli/md.go:56`，调用点 L60，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2229，confidence=0.70，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/latex_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/latex.go`、`internal/cli/md.go`、`internal/cli/latex_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `normalizeMath` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L263、L278、L286、L300、L316 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `normalizeMath` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/latex.go|normalizeMath` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/latex.go:255-326` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `normalizeMath` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/latex_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/latex.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/latex.go|normalizeMath` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-129 · 圈复杂度 · `*chatTUI.updateCompletion`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-129 |
| Baseline key | `complexity\|internal/cli/complete.go\|*chatTUI.updateCompletion` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/cli/complete.go` |
| 代码行 | 119 |
| 函数/方法/包 | `*chatTUI.updateCompletion` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.updateCompletion` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 119-153 行；可见控制流热点：L124 `if at, token, ok := activeAtToken(val); ok {`；L125 `if items := m.atItems(token); len(items) > 0 {`；L131 `if strings.HasPrefix(val, "/") {`；L132 `if items, from, ok := m.explicitSubcommandItems(val); ok && len(items) > 0 {`；L136 `if !strings.ContainsAny(val, " \t\n") {`；L138 `if items := fuzzyFilterSlash(m.slashItems(), val); len(items) > 0 {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chatTUI::setCompletion）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::updateCompletion（internal/cli/complete.go:119-153）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（4）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L828/L834/L1345/L1370，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::acceptCompletion`（`internal/cli/complete.go:601`，调用点 L615/L618，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3329，confidence=0.70，instance-method，待源码复核）；`chatTUI::insertImageRef`（`internal/cli/chat_tui_paste.go:57`，调用点 L63，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `fuzzyFilterSlash`（`internal/cli/complete.go:327`，调用点 L138，confidence=0.90，exact-match）；`activeAtToken`（`internal/cli/complete.go:380`，调用点 L124，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `chatTUI::setCompletion`（`internal/cli/complete.go:313`，调用点 L126/L133/L139/L147，confidence=0.70，instance-method，4 条边，待源码复核）；`chatTUI::slashItems`（`internal/cli/complete.go:64`，调用点 L138，confidence=0.70，instance-method，待源码复核）；`chatTUI::slashArgItems`（`internal/cli/complete.go:161`，调用点 L145，confidence=0.70，instance-method，待源码复核）；`chatTUI::explicitSubcommandItems`（`internal/cli/complete.go:232`，调用点 L132，confidence=0.70，instance-method，待源码复核）；`chatTUI::bareSubcommandSpace`（`internal/cli/complete.go:253`，调用点 L142，confidence=0.70，instance-method，待源码复核）；`chatTUI::atItems`（`internal/cli/complete.go:399`，调用点 L125，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L828，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/complete_test.go`、`internal/cli/skill_picker_test.go`、`internal/cli/resume_test.go`、`internal/cli/skillslash_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/complete.go`、`internal/cli/chat_tui.go`、`internal/cli/chat_tui_paste.go`、`internal/cli/complete_test.go`、`internal/cli/skill_picker_test.go`、`internal/cli/resume_test.go`、`internal/cli/skillslash_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.updateCompletion` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L124、L125、L132、L136、L138 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.updateCompletion` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/complete.go|*chatTUI.updateCompletion` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/complete.go:119-153` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.updateCompletion` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/complete_test.go`、`internal/cli/skill_picker_test.go`、`internal/cli/resume_test.go`、`internal/cli/skillslash_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/complete.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/complete.go|*chatTUI.updateCompletion` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-132 · 圈复杂度 · `chatTUI.renderCompletion`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-132 |
| Baseline key | `complexity\|internal/cli/complete.go\|chatTUI.renderCompletion` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/cli/complete.go` |
| 代码行 | 652 |
| 函数/方法/包 | `chatTUI.renderCompletion` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.renderCompletion` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 652-695 行；可见控制流热点：L653 `if !m.completion.active || len(m.completion.items) == 0 {`；L660 `if start < 0 {`；L663 `if start > len(items)-maxCompRows {`；L673 `for i := start; i < end; i++ {`；L676 `if i == m.completion.sel {`；L690 `if m.completion.kind == compAt {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::renderCompletion（internal/cli/complete.go:652-695）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `chatTUI::bottomRows`（`internal/cli/chat_tui.go:1465`，调用点 L1474，confidence=0.70，instance-method，待源码复核）；`chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2348，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `padCompletionLine`（`internal/cli/complete.go:638`，调用点 L684/L693，confidence=0.90，exact-match，2 条边）；`dim`（`internal/cli/style.go:49`，调用点 L682/L693，confidence=0.90，exact-match，2 条边）；`accent`（`internal/cli/style.go:53`，调用点 L677，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2348，confidence=0.70，instance-method，待源码复核）；`chatREPL`（`internal/cli/cli.go:652`，调用点 L809，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/complete.go`、`internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.renderCompletion` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L653、L660、L668、L676、L690 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `chatTUI.renderCompletion` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/complete.go|chatTUI.renderCompletion` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/complete.go:652-695` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.renderCompletion` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/complete.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/complete.go|chatTUI.renderCompletion` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-130 · 圈复杂度 · `*chatTUI.branchArgItems`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-130 |
| Baseline key | `complexity\|internal/cli/complete.go\|*chatTUI.branchArgItems` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/cli/complete.go` |
| 代码行 | 277 |
| 函数/方法/包 | `*chatTUI.branchArgItems` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.branchArgItems` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 277-309 行；可见控制流热点：L279 `if cmdEnd < 0 || val[:cmdEnd] != "/switch" {`；L284 `if len(prior) != 1 || m.ctrl == nil {`；L288 `if err != nil {`；L295 `if cur != "" && !strings.HasPrefix(strings.ToLower(label), cur) &&`；L300 `if hint == "" {`；L303 `if hint != "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::branchArgItems（internal/cli/complete.go:277-309）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::slashArgItems`（`internal/cli/complete.go:161`，调用点 L162，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/complete.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.branchArgItems` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L279、L288、L293、L300、L303 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*chatTUI.branchArgItems` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/complete.go|*chatTUI.branchArgItems` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/complete.go:277-309` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.branchArgItems` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/complete.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/complete.go|*chatTUI.branchArgItems` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-264 · 圈复杂度 · `*Session.Review`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-264 |
| Baseline key | `complexity\|internal/guardian/guardian.go\|*Session.Review` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/guardian/guardian.go` |
| 代码行 | 114 |
| 函数/方法/包 | `*Session.Review` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Session.Review` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 114-213 行；可见控制流热点：L144 `switch {`；L147 `case needDelta:`；L167 `if agentErr == nil && reviewN%compactEvery == 0 {`；L184 `if parseErr != nil {`；L194 `if assessment.Outcome == "deny" {`；L209 `if assessment.Outcome == "deny" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（ParseAssessment、Session::Load）、渲染/输出（formatReviewRequest、formatDelta、FormatTranscript、Session::emitTo）、网络/协议（formatReviewRequest）、权限/安全（DenyReason、Session::recordAllow）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Session::Review（internal/guardian/guardian.go:114-213）
- **可信生产调用方（1）：** `gateApprover::ApproveWithReason`（`internal/control/controller.go:3450`，调用点 L3456，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（9）：** `Agent::CompactNow`（`internal/agent/agent.go:545`，调用点 L168，confidence=0.90，exact-match）；`subject`（`internal/guardian/guardian.go:399`，调用点 L207，confidence=0.90，exact-match）；`formatReviewRequest`（`internal/guardian/guardian.go:419`，调用点 L157，confidence=0.90，exact-match）；`formatDelta`（`internal/guardian/guardian.go:424`，调用点 L149，confidence=0.90，exact-match）；`ParseAssessment`（`internal/guardian/policy.go:41`，调用点 L183，confidence=0.90，exact-match）；`DenyReason`（`internal/guardian/policy.go:145`，调用点 L199，confidence=0.90，exact-match）；`CircuitBreakerReason`（`internal/guardian/policy.go:151`，调用点 L197，confidence=0.90，exact-match）；`ExtractTranscript`（`internal/guardian/transcript.go:34`，调用点 L121，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `Session::Load`（`internal/guardian/guardian.go:261`，调用点 L165，confidence=0.70，exact-match，待源码复核）；`Session::recordDenial`（`internal/guardian/guardian.go:344`，调用点 L195，confidence=0.70，instance-method，待源码复核）；`Session::recordAllow`（`internal/guardian/guardian.go:359`，调用点 L202，confidence=0.70，instance-method，待源码复核）；`Session::countRecentDenials`（`internal/guardian/guardian.go:367`，调用点 L197，confidence=0.70，instance-method，待源码复核）；`Session::emitTo`（`internal/guardian/guardian.go:379`，调用点 L207，confidence=0.70，instance-method，待源码复核）；`lastAssistantText`（`internal/guardian/guardian.go:445`，调用点 L181，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `gateApprover::ApproveWithReason`（`internal/control/controller.go:3450`，调用点 L3456，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/guardian`、`internal/control`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/guardian/guardian_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/guardian/guardian.go`、`internal/control/controller.go`、`internal/agent/agent.go`、`internal/guardian/guardian_test.go`

#### 待修改项

- [ ] 逐段核对 L144、L147、L173、L194、L209 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、渲染/输出、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*Session.Review` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/guardian/guardian.go|*Session.Review` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/guardian/guardian.go:114-213` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Session.Review` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：协议、序列化或流式事件兼容；权限、安全或凭据边界；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/guardian/guardian_test.go`。
- [ ] 运行相关包测试：`go test ./internal/guardian`。
- [ ] 运行相关包 vet：`go vet ./internal/guardian`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/guardian/guardian.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/guardian/guardian.go|*Session.Review` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-265 · 圈复杂度 · `ExtractTranscript`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-265 |
| Baseline key | `complexity\|internal/guardian/transcript.go\|ExtractTranscript` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/guardian/transcript.go` |
| 代码行 | 34 |
| 函数/方法/包 | `ExtractTranscript` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ExtractTranscript` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 34-72 行；可见控制流热点：L36 `for _, m := range msgs {`；L38 `case provider.RoleSystem:`；L42 `if text := strings.TrimSpace(m.Content); text != "" {`；L47 `if text == "" && len(m.ToolCalls) > 0 {`；L58 `if text == "" {`；L64 `if text == "" {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（firstRunesStr）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function ExtractTranscript（internal/guardian/transcript.go:34-72）
- **可信生产调用方（1）：** `Session::Review`（`internal/guardian/guardian.go:114`，调用点 L121，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `firstRunesStr`（`internal/guardian/guardian.go:437`，调用点 L52，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `gateApprover::ApproveWithReason`（`internal/control/controller.go:3450`，调用点 L3456，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/guardian`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/guardian/guardian_test.go`
- **需同步检查：** `internal/guardian/transcript.go`、`internal/guardian/guardian.go`、`internal/guardian/guardian_test.go`

#### 待修改项

- [ ] 逐段核对 L36、L41、L45、L58、L64 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ExtractTranscript` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/guardian/transcript.go|ExtractTranscript` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/guardian/transcript.go:34-72` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ExtractTranscript` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G30 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/guardian/guardian_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/guardian`。
- [ ] 运行相关包 vet：`go vet ./internal/guardian`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/guardian/transcript.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/guardian/transcript.go|ExtractTranscript` 下降或消失且无新增/恶化。
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
