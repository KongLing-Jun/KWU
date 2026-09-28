# G30 · internal/cli · skill_picker 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G30 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli/latex.go`<br>`internal/cli/skill_picker.go`<br>`internal/cli/mcp_manager_view.go`<br>`internal/cli/rewind.go` |
| 主要治理目标 | 围绕 `internal/cli/skill_picker.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 5 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 30 组；等待 G21 的同文件变更合并 |
| 前置依赖 | G21（internal/cli/latex.go） |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容；权限、安全或凭据边界；配置、平台或向后兼容；会话、状态或恢复语义 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **44/100**，对象 `readAtom` 的分项为严重度 6.0、集中度 4.6、影响面 17.7、可执行性 16.0；最高超限 1.50x，文件内 3 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 4 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G21。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-138` | `complexity\|internal/cli/latex.go\|readAtom` | 圈复杂度 | `internal/cli/latex.go` | `readAtom` | 117 | 15/10 | +5（1.50x） | P2 | 2 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-163` | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerKey` | 圈复杂度 | `internal/cli/skill_picker.go` | `chatTUI.handleSkillPickerKey` | 89 | 20/10 | +10（2.00x） | P2 | 0 可信 + 1 待核 caller / 1 可信 + 7 待核 callee / 0 unresolved | 待修改 |
| `CX-144` | `complexity\|internal/cli/mcp_manager_view.go\|*mcpManager.renderDetail` | 圈复杂度 | `internal/cli/mcp_manager_view.go` | `*mcpManager.renderDetail` | 121 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 17 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-158` | `complexity\|internal/cli/rewind.go\|chatTUI.handleRewindKey` | 圈复杂度 | `internal/cli/rewind.go` | `chatTUI.handleRewindKey` | 50 | 25/10 | +15（2.50x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-145` | `complexity\|internal/cli/mcp_manager_view.go\|mcpActionsFor` | 圈复杂度 | `internal/cli/mcp_manager_view.go` | `mcpActionsFor` | 235 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-143` | `complexity\|internal/cli/mcp_manager_view.go\|*mcpManager.renderList` | 圈复杂度 | `internal/cli/mcp_manager_view.go` | `*mcpManager.renderList` | 53 | 11/10 | +1（1.10x） | P3 | 0 可信 + 0 待核 caller / 5 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-164` | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSkillsKey` | 圈复杂度 | `internal/cli/skill_picker.go` | `chatTUI.handleSkillPickerSkillsKey` | 145 | 13/10 | +3（1.30x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 5 待核 callee / 0 unresolved | 待修改 |
| `CX-166` | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSourceSkillsKey` | 圈复杂度 | `internal/cli/skill_picker.go` | `chatTUI.handleSkillPickerSourceSkillsKey` | 211 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-167` | `complexity\|internal/cli/skill_picker.go\|skillRootLines` | 圈复杂度 | `internal/cli/skill_picker.go` | `skillRootLines` | 524 | 12/10 | +2（1.20x） | P3 | 2 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-165` | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSourcesKey` | 圈复杂度 | `internal/cli/skill_picker.go` | `chatTUI.handleSkillPickerSourcesKey` | 179 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 2 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-138 · 圈复杂度 · `readAtom`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-138 |
| Baseline key | `complexity\|internal/cli/latex.go\|readAtom` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 44） |
| 文件 | `internal/cli/latex.go` |
| 代码行 | 117 |
| 函数/方法/包 | `readAtom` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`readAtom` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 117-154 行；可见控制流热点：L118 `if skipSpaces {`；L123 `if i >= len(rs) {`；L130 `for j := i; j < len(rs); j++ {`；L132 `case '{':`；L142 `case '\\':`；L147 `for k < len(rs) && isASCIILetter(rs[k]) {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function readAtom（internal/cli/latex.go:117-154）
- **可信生产调用方（2）：** `convertCommand`（`internal/cli/latex.go:49`，调用点 L74/L75/L83/L87，confidence=0.90，exact-match，7 条边）；`convertMath`（`internal/cli/latex.go:17`，调用点 L25/L29，confidence=0.90，exact-match，2 条边）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `isASCIILetter`（`internal/cli/latex.go:247`，调用点 L144/L147，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/latex.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `readAtom` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L118、L126、L131、L136、L147 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `readAtom` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/latex.go|readAtom` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/latex.go:117-154` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `readAtom` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/latex.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/latex.go|readAtom` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-163 · 圈复杂度 · `chatTUI.handleSkillPickerKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-163 |
| Baseline key | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 44） |
| 文件 | `internal/cli/skill_picker.go` |
| 代码行 | 89 |
| 函数/方法/包 | `chatTUI.handleSkillPickerKey` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.handleSkillPickerKey` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 89-143 行；可见控制流热点：L91 `if p == nil {`；L97 `case "esc":`；L108 `case "up", "k":`；L115 `if p.sel < len(filtered)-1 {`；L133 `case pickerSources:`；L139 `case pickerConfirmDelete:`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chatTUI::handleSkillPickerSkillsKey、chatTUI::handleSkillPickerSourcesKey、chatTUI::handleSkillPickerSourceSkillsKey、chatTUI::handleSkillPickerDetailKey）、持久化/I-O（chatTUI::saveSkillPick）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleSkillPickerKey（internal/cli/skill_picker.go:89-143）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L911，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `clampSel`（`internal/cli/skill_picker.go:666`，调用点 L105/L125，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（7）：** `skillPicker::filteredSkills`（`internal/cli/skill_picker.go:366`，调用点 L105/L114/L125，confidence=0.70，instance-method，3 条边，待源码复核）；`chatTUI::handleSkillPickerSkillsKey`（`internal/cli/skill_picker.go:145`，调用点 L132，confidence=0.70，instance-method，待源码复核）；`chatTUI::handleSkillPickerSourcesKey`（`internal/cli/skill_picker.go:179`，调用点 L134，confidence=0.70，instance-method，待源码复核）；`chatTUI::handleSkillPickerSourceSkillsKey`（`internal/cli/skill_picker.go:211`，调用点 L136，confidence=0.70，instance-method，待源码复核）；`chatTUI::handleSkillPickerDetailKey`（`internal/cli/skill_picker.go:238`，调用点 L138，confidence=0.70，instance-method，待源码复核）；`chatTUI::handleSkillPickerConfirmDeleteKey`（`internal/cli/skill_picker.go:265`，调用点 L140，confidence=0.70，instance-method，待源码复核）；`chatTUI::saveSkillPick`（`internal/cli/skill_picker.go:288`，调用点 L101，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L911，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/skill_picker_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/skill_picker.go`、`internal/cli/chat_tui.go`、`internal/cli/skill_picker_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleSkillPickerKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L91、L100、L113、L131、L139 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.handleSkillPickerKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/skill_picker.go:89-143` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleSkillPickerKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/skill_picker_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/skill_picker.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-144 · 圈复杂度 · `*mcpManager.renderDetail`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-144 |
| Baseline key | `complexity\|internal/cli/mcp_manager_view.go\|*mcpManager.renderDetail` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 43） |
| 文件 | `internal/cli/mcp_manager_view.go` |
| 代码行 | 121 |
| 函数/方法/包 | `*mcpManager.renderDetail` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*mcpManager.renderDetail` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 121-166 行；可见控制流热点：L123 `if !ok {`；L133 `if auth := mcpAuthLabel(v); auth != "" {`；L141 `if loc != "not saved" {`；L148 `if line := mcpCommandLine(v); line != "" {`；L154 `if v.Error != "" {`；L161 `for i, a := range actions {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（writeMCPDetailField）、渲染/输出（viewBudget、viewCompactText、viewMeta、viewCompactPath）、权限/安全（mcpAuthLabel）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method mcpManager::renderDetail（internal/cli/mcp_manager_view.go:121-166）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `mcpManager::render`（`internal/cli/mcp_manager_view.go:19`，调用点 L23，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（17）：** `writeMCPDetailField`（`internal/cli/mcp_manager_view.go:296`，调用点 L132/L134/L136/L138，confidence=0.90，exact-match，10 条边）；`viewBudget`（`internal/cli/view_helpers.go:82`，调用点 L142/L149/L155，confidence=0.90，exact-match，3 条边）；`fallbackText`（`internal/cli/mcp_manager.go:422`，调用点 L136/L140，confidence=0.90，exact-match，2 条边）；`viewCompactText`（`internal/cli/view_helpers.go:49`，调用点 L149/L155，confidence=0.90，exact-match，2 条边）；`rowLine`（`internal/cli/chooser.go:299`，调用点 L162，confidence=0.90，exact-match）；`titleText`（`internal/cli/mcp_manager.go:429`，调用点 L131，confidence=0.90，exact-match）；`mcpActionsFor`（`internal/cli/mcp_manager_view.go:235`，调用点 L126，confidence=0.90，exact-match）；`mcpStatusLabel`（`internal/cli/mcp_manager_view.go:303`，调用点 L132，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `mcpManager::selectedServer`（`internal/cli/mcp_manager.go:284`，调用点 L122，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/mcp_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_manager_view.go`、`internal/cli/view_helpers.go`、`internal/cli/mcp_manager.go`、`internal/cli/mcp_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*mcpManager.renderDetail` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L123、L133、L148、L154、L161 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出、权限/安全 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*mcpManager.renderDetail` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp_manager_view.go|*mcpManager.renderDetail` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_manager_view.go:121-166` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*mcpManager.renderDetail` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/mcp_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_manager_view.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp_manager_view.go|*mcpManager.renderDetail` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-158 · 圈复杂度 · `chatTUI.handleRewindKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-158 |
| Baseline key | `complexity\|internal/cli/rewind.go\|chatTUI.handleRewindKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 43） |
| 文件 | `internal/cli/rewind.go` |
| 代码行 | 50 |
| 函数/方法/包 | `chatTUI.handleRewindKey` |
| Baseline / 当前复测 / 阈值 | 25 / 25 / 10 |
| 超出 | +15，2.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.handleRewindKey` 的圈复杂度当前为 **25**，超过限制 **10**（+15，2.50x）。定义范围为 50-113 行；可见控制流热点：L52 `switch msg.String() {`；L60 `if r.stage == 0 {`；L75 `case "enter":`；L86 `case "c":`；L97 `if r.stage == 1 {`；L107 `if r.stage == 1 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleRewindKey（internal/cli/rewind.go:50-113）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L891，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `chatTUI::applyRewind`（`internal/cli/rewind.go:115`，调用点 L79/L84/L89/L94，confidence=0.70，instance-method，7 条边，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L891，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/rewind.go`、`internal/cli/chat_tui.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleRewindKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L52、L67、L81、L96、L107 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `chatTUI.handleRewindKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/rewind.go|chatTUI.handleRewindKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/rewind.go:50-113` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleRewindKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/rewind.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/rewind.go|chatTUI.handleRewindKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-145 · 圈复杂度 · `mcpActionsFor`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-145 |
| Baseline key | `complexity\|internal/cli/mcp_manager_view.go\|mcpActionsFor` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 42） |
| 文件 | `internal/cli/mcp_manager_view.go` |
| 代码行 | 235 |
| 函数/方法/包 | `mcpActionsFor` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`mcpActionsFor` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 235-271 行；可见控制流热点：L237 `if v.Tools > 0 || len(v.ToolList) > 0 {`；L241 `if mcpAuthStatus(v) == mcpdiag.AuthRequired {`；L252 `switch v.Status {`；L255 `case "deferred":`；L261 `if mcpCanClearAuth(v) {`；L267 `if !v.BuiltIn {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（appendMCPConfigActions）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mcpActionsFor（internal/cli/mcp_manager_view.go:235-271）
- **可信生产调用方（2）：** `chatTUI::handleMCPManagerKey`（`internal/cli/mcp_manager.go:115`，调用点 L169，confidence=0.90，exact-match）；`mcpManager::renderDetail`（`internal/cli/mcp_manager_view.go:121`，调用点 L126，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `appendMCPFailureSecondaryActions`（`internal/cli/mcp_manager_view.go:273`，调用点 L249，confidence=0.90，exact-match）；`appendMCPConfigActions`（`internal/cli/mcp_manager_view.go:287`，调用点 L260，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L903，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_manager_view.go`、`internal/cli/mcp_manager.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `mcpActionsFor` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L237、L246、L253、L261、L267 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `mcpActionsFor` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp_manager_view.go|mcpActionsFor` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_manager_view.go:235-271` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mcpActionsFor` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_manager_view.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp_manager_view.go|mcpActionsFor` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-143 · 圈复杂度 · `*mcpManager.renderList`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-143 |
| Baseline key | `complexity\|internal/cli/mcp_manager_view.go\|*mcpManager.renderList` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/cli/mcp_manager_view.go` |
| 代码行 | 53 |
| 函数/方法/包 | `*mcpManager.renderList` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*mcpManager.renderList` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 53-92 行；可见控制流热点：L57 `if p.snapshot.err != "" {`；L65 `if start > 0 {`；L69 `for i := start; i < end; i++ {`；L75 `if group != lastGroup {`；L76 `if lastGroup != "" {`；L88 `if end < len(p.snapshot.servers) {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（viewMeta、viewHeader、mcpManager::renderListRow）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method mcpManager::renderList（internal/cli/mcp_manager_view.go:53-92）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `viewMeta`（`internal/cli/view_helpers.go:25`，调用点 L56/L61/L66/L89，confidence=0.90，exact-match，4 条边）；`visibleRange`（`internal/cli/mcp_manager.go:394`，调用点 L64，confidence=0.90，exact-match）；`bold`（`internal/cli/style.go:48`，调用点 L83，confidence=0.90，exact-match）；`yellow`（`internal/cli/style.go:52`，调用点 L58，confidence=0.90，exact-match）；`viewHeader`（`internal/cli/view_helpers.go:17`，调用点 L55，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `mcpManager::renderListRow`（`internal/cli/mcp_manager_view.go:94`，调用点 L86，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_manager_view.go`、`internal/cli/view_helpers.go`、`internal/cli/mcp_manager.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*mcpManager.renderList` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L57、L65、L72、L76、L88 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*mcpManager.renderList` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp_manager_view.go|*mcpManager.renderList` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_manager_view.go:53-92` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*mcpManager.renderList` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_manager_view.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp_manager_view.go|*mcpManager.renderList` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-164 · 圈复杂度 · `chatTUI.handleSkillPickerSkillsKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-164 |
| Baseline key | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSkillsKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/cli/skill_picker.go` |
| 代码行 | 145 |
| 函数/方法/包 | `chatTUI.handleSkillPickerSkillsKey` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.handleSkillPickerSkillsKey` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 145-177 行；可见控制流热点：L147 `switch msg.String() {`；L150 `case "up", "k":`；L155 `if p.sel < len(p.skills)-1 {`；L160 `case " ", "space":`；L166 `case "/":`；L173 `case "r":`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（chatTUI::saveSkillPick、skillPicker::openDetail）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleSkillPickerSkillsKey（internal/cli/skill_picker.go:145-177）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::handleSkillPickerKey`（`internal/cli/skill_picker.go:89`，调用点 L132，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（5）：** `chatTUI::saveSkillPick`（`internal/cli/skill_picker.go:288`，调用点 L159，confidence=0.70，instance-method，待源码复核）；`chatTUI::rescanSkills`（`internal/cli/skill_picker.go:342`，调用点 L174，confidence=0.70，instance-method，待源码复核）；`skillPicker::selectedSkill`（`internal/cli/skill_picker.go:394`，调用点 L163，confidence=0.70，instance-method，待源码复核）；`skillPicker::toggleSelectedSkill`（`internal/cli/skill_picker.go:409`，调用点 L161，confidence=0.70，instance-method，待源码复核）；`skillPicker::openDetail`（`internal/cli/skill_picker.go:415`，调用点 L164，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L911，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/skill_picker.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleSkillPickerSkillsKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L147、L151、L158、L163、L173 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.handleSkillPickerSkillsKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerSkillsKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/skill_picker.go:145-177` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleSkillPickerSkillsKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/skill_picker.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerSkillsKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-166 · 圈复杂度 · `chatTUI.handleSkillPickerSourceSkillsKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-166 |
| Baseline key | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSourceSkillsKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/cli/skill_picker.go` |
| 代码行 | 211 |
| 函数/方法/包 | `chatTUI.handleSkillPickerSourceSkillsKey` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.handleSkillPickerSourceSkillsKey` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 211-236 行；可见控制流热点：L214 `switch msg.String() {`；L217 `case "up", "k":`；L221 `case "down", "j":`；L222 `if p.sourceSkillSel < len(skills)-1 {`；L226 `if p.sourceSkillSel >= 0 && p.sourceSkillSel < len(skills) {`；L230 `if p.sourceSkillSel >= 0 && p.sourceSkillSel < len(skills) {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（skillPicker::openDetail）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleSkillPickerSourceSkillsKey（internal/cli/skill_picker.go:211-236）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::handleSkillPickerKey`（`internal/cli/skill_picker.go:89`，调用点 L136，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `clampSel`（`internal/cli/skill_picker.go:666`，调用点 L234，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `skillPicker::openDetail`（`internal/cli/skill_picker.go:415`，调用点 L227，confidence=0.70，instance-method，待源码复核）；`skillPicker::toggleSkill`（`internal/cli/skill_picker.go:432`，调用点 L231，confidence=0.70，instance-method，待源码复核）；`skillPicker::selectedRootSkills`（`internal/cli/skill_picker.go:497`，调用点 L213，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L911，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/skill_picker.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleSkillPickerSourceSkillsKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L214、L217、L222、L226、L230 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.handleSkillPickerSourceSkillsKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerSourceSkillsKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/skill_picker.go:211-236` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleSkillPickerSourceSkillsKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/skill_picker.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerSourceSkillsKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-167 · 圈复杂度 · `skillRootLines`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-167 |
| Baseline key | `complexity\|internal/cli/skill_picker.go\|skillRootLines` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/cli/skill_picker.go` |
| 代码行 | 524 |
| 函数/方法/包 | `skillRootLines` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`skillRootLines` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 524-574 行；可见控制流热点：L527 `for i, r := range storeRoots {`；L535 `if s.Scope == skill.ScopeBuiltin {`；L540 `if lines[i].scope != s.Scope {`；L551 `for i := range lines {`；L559 `for _, s := range skills {`；L564 `if builtinCount > 0 {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::Roots）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function skillRootLines（internal/cli/skill_picker.go:524-574）
- **可信生产调用方（2）：** `chatTUI::openSkillPicker`（`internal/cli/skill_picker.go:61`，调用点 L83，confidence=0.90，exact-match）；`chatTUI::refreshSkillPickerData`（`internal/cli/skill_picker.go:347`，调用点 L354，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Store::Roots`（`internal/skill/skill.go:225`，调用点 L525，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`、`internal/skill`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/skill_picker.go`、`internal/skill/skill.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `skillRootLines` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L527、L539、L545、L559、L564 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `skillRootLines` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/skill_picker.go|skillRootLines` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/skill_picker.go:524-574` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `skillRootLines` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/skill_picker.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/skill_picker.go|skillRootLines` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-165 · 圈复杂度 · `chatTUI.handleSkillPickerSourcesKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-165 |
| Baseline key | `complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSourcesKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/cli/skill_picker.go` |
| 代码行 | 179 |
| 函数/方法/包 | `chatTUI.handleSkillPickerSourcesKey` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.handleSkillPickerSourcesKey` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 179-209 行；可见控制流热点：L182 `switch msg.String() {`；L185 `case "up", "k":`；L189 `case "down", "j":`；L193 `case "enter", "right", "l":`；L198 `case "d":`；L205 `case "r":`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleSkillPickerSourcesKey（internal/cli/skill_picker.go:179-209）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::handleSkillPickerKey`（`internal/cli/skill_picker.go:89`，调用点 L134，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `clampSel`（`internal/cli/skill_picker.go:666`，调用点 L200，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `chatTUI::rescanSkills`（`internal/cli/skill_picker.go:342`，调用点 L206，confidence=0.70，instance-method，待源码复核）；`skillPicker::visibleRoots`（`internal/cli/skill_picker.go:381`，调用点 L181，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L911，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/skill_picker.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleSkillPickerSourcesKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L182、L186、L190、L198、L205 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `chatTUI.handleSkillPickerSourcesKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerSourcesKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/skill_picker.go:179-209` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleSkillPickerSourcesKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/skill_picker.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/skill_picker.go|chatTUI.handleSkillPickerSourcesKey` 下降或消失且无新增/恶化。
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
