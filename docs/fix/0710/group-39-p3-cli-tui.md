# G39 · CLI/TUI 状态更新与交互渲染

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G39 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli/chat_tui.go`<br>`internal/cli/mcp_manager_actions.go`<br>`internal/cli/gitstatus.go`<br>`internal/cli/language.go`<br>`internal/cli/provider.go`<br>`internal/cli/skill_hooks.go` |
| 主要治理目标 | 围绕 `internal/cli/chat_tui.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 单文件最多集中 20 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 39 组；等待 G02、G15、G36 的同文件变更合并 |
| 前置依赖 | G02（internal/cli/chat_tui.go）；G15（internal/cli/chat_tui.go）；G36（internal/cli/chat_tui.go） |
| 后续验证重点 | 协议、序列化或流式事件兼容；返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **40/100**，对象 `*chatTUI.streamToolOutput` 的分项为严重度 6.6、集中度 10.0、影响面 17.0、可执行性 6.0；最高超限 1.70x，文件内 20 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。

### 治理边界与组间关系

本组涉及 6 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G02、G15、G36。后续同文件组：G42，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-108` | `complexity\|internal/cli/chat_tui.go\|*chatTUI.streamToolOutput` | 圈复杂度 | `internal/cli/chat_tui.go` | `*chatTUI.streamToolOutput` | 1672 | 17/10 | +7（1.70x） | P3 | 0 可信 + 1 待核 caller / 3 可信 + 3 待核 callee / 10 unresolved | 待修改 |
| `CX-142` | `complexity\|internal/cli/mcp_manager_actions.go\|chatTUI.applyMCPMode` | 圈复杂度 | `internal/cli/mcp_manager_actions.go` | `chatTUI.applyMCPMode` | 134 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 6 可信 + 7 待核 callee / 9 unresolved | 待修改 |
| `CX-110` | `complexity\|internal/cli/chat_tui.go\|*chatTUI.collapseShellSlot` | 圈复杂度 | `internal/cli/chat_tui.go` | `*chatTUI.collapseShellSlot` | 1826 | 15/10 | +5（1.50x） | P3 | 0 可信 + 1 待核 caller / 3 可信 + 0 待核 callee / 18 unresolved | 待修改 |
| `CX-135` | `complexity\|internal/cli/gitstatus.go\|loadGitStatus` | 圈复杂度 | `internal/cli/gitstatus.go` | `loadGitStatus` | 41 | 12/10 | +2（1.20x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 1 待核 callee / 9 unresolved | 待修改 |
| `CX-136` | `complexity\|internal/cli/language.go\|*chatTUI.runLanguageSubcommand` | 圈复杂度 | `internal/cli/language.go` | `*chatTUI.runLanguageSubcommand` | 13 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 5 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-155` | `complexity\|internal/cli/provider.go\|*chatTUI.switchToProvider` | 圈复杂度 | `internal/cli/provider.go` | `*chatTUI.switchToProvider` | 73 | 14/10 | +4（1.40x） | P3 | 0 可信 + 1 待核 caller / 3 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-161` | `complexity\|internal/cli/skill_hooks.go\|*chatTUI.runSkillSubcommand` | 圈复杂度 | `internal/cli/skill_hooks.go` | `*chatTUI.runSkillSubcommand` | 17 | 13/10 | +3（1.30x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-113` | `complexity\|internal/cli/chat_tui.go\|chatTUI.renderTodoPanel` | 圈复杂度 | `internal/cli/chat_tui.go` | `chatTUI.renderTodoPanel` | 2620 | 13/10 | +3（1.30x） | P3 | 0 可信 + 2 待核 caller / 5 可信 + 1 待核 callee / 19 unresolved | 待修改 |
| `CX-111` | `complexity\|internal/cli/chat_tui.go\|*chatTUI.toggleShellOutput` | 圈复杂度 | `internal/cli/chat_tui.go` | `*chatTUI.toggleShellOutput` | 1890 | 13/10 | +3（1.30x） | P3 | 0 可信 + 1 待核 caller / 3 可信 + 1 待核 callee / 9 unresolved | 待修改 |
| `CX-107` | `complexity\|internal/cli/chat_tui.go\|chatTUI.renderTranscriptWithMainManager` | 圈复杂度 | `internal/cli/chat_tui.go` | `chatTUI.renderTranscriptWithMainManager` | 1568 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 0 待核 callee / 20 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-108 · 圈复杂度 · `*chatTUI.streamToolOutput`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-108 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|*chatTUI.streamToolOutput` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 40） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 1672 |
| 函数/方法/包 | `*chatTUI.streamToolOutput` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.streamToolOutput` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 1672-1746 行；可见控制流热点：L1673 `if id == "" {`；L1685 `if existingIdx, ok := m.shellTranscriptIdx[id]; ok && existingIdx >= 0 && existingIdx < len(m.trans…`；L1693 `if n > 0 {`；L1718 `if strings.HasPrefix(id, "shell-") {`；L1734 `if m.toolPartial != "" {`；L1741 `for i, ln := range vis {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chatTUI::collapseToolOutput）、任务/并发（connectorBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::streamToolOutput（internal/cli/chat_tui.go:1672-1746）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::ingestEvent`（`internal/cli/chat_tui.go:3004`，调用点 L3089，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `clampPlain`（`internal/cli/diffview.go:205`，调用点 L1742，confidence=0.90，exact-match）；`dim`（`internal/cli/style.go:49`，调用点 L1742，confidence=0.90，exact-match）；`connectorBlock`（`internal/cli/toolcard.go:19`，调用点 L1744，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L1713，confidence=0.70，instance-method，待源码复核）；`chatTUI::pushToolLine`（`internal/cli/chat_tui.go:1750`，调用点 L1728，confidence=0.70，instance-method，待源码复核）；`chatTUI::collapseToolOutput`（`internal/cli/chat_tui.go:1765`，调用点 L1704，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1200，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 10 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/diffview.go`、`internal/cli/style.go`、`internal/cli/toolcard.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.streamToolOutput` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1673、L1688、L1709、L1725、L1741 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.streamToolOutput` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|*chatTUI.streamToolOutput` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:1672-1746` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.streamToolOutput` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|*chatTUI.streamToolOutput` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-142 · 圈复杂度 · `chatTUI.applyMCPMode`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-142 |
| Baseline key | `complexity\|internal/cli/mcp_manager_actions.go\|chatTUI.applyMCPMode` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/cli/mcp_manager_actions.go` |
| 代码行 | 134 |
| 函数/方法/包 | `chatTUI.applyMCPMode` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.applyMCPMode` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 134-188 行；可见控制流热点：L136 `if !ok {`；L147 `for _, p := range merged.Plugins {`；L154 `if !found {`；L167 `if err := userCfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {`；L174 `if m.ctrl != nil && !mcpConnected(m.ctrl, v.Name) {`；L182 `if m.mcp != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeMCPTierForCLI、LoadForRootWithActor、chatTUI::mcpConfigContext、Config::UpsertPlugin）、状态与控制（PluginEntry::ShouldAutoStart）、持久化/I-O（Config::SaveToScope、ProfilePathResolver::UserConfigPath）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::applyMCPMode（internal/cli/mcp_manager_actions.go:134-188）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（6）：** `mcpManager::selectName`（`internal/cli/mcp_manager.go:273`，调用点 L184，confidence=0.90，exact-match）；`mcpManager::selectedServer`（`internal/cli/mcp_manager.go:284`，调用点 L135，confidence=0.90，exact-match）；`recordMCPModePluginFailure`（`internal/cli/mcp_manager_actions.go:190`，调用点 L176，confidence=0.90，exact-match）；`normalizeMCPTierForCLI`（`internal/cli/mcp_manager_actions.go:297`，调用点 L158，confidence=0.90，exact-match）；`mcpBoolPtr`（`internal/cli/mcp_manager_actions.go:499`，调用点 L160，confidence=0.90，exact-match）；`LoadForRootWithActor`（`internal/config/load.go:34`，调用点 L140，confidence=0.90，import）
- **待源码复核被调候选（7）：** `chatTUI::refreshMCPManager`（`internal/cli/mcp_manager.go:107`，调用点 L181，confidence=0.70，instance-method，待源码复核）；`chatTUI::mcpConfigContext`（`internal/cli/mcp_manager_actions.go:310`，调用点 L139，confidence=0.70，instance-method，待源码复核）；`mcpConnected`（`internal/cli/mcp_manager_actions.go:483`，调用点 L174，confidence=0.70，exact-match，待源码复核）；`PluginEntry::ShouldAutoStart`（`internal/config/config.go:1191`，调用点 L159，confidence=0.70，instance-method，待源码复核）；`Config::UpsertPlugin`（`internal/config/edit.go:694`，调用点 L163，confidence=0.70，instance-method，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L167，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L167，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/mcp_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_manager_actions.go`、`internal/cli/mcp_manager.go`、`internal/cli/mcp_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.applyMCPMode` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L136、L148、L163、L171、L182 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.applyMCPMode` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp_manager_actions.go|chatTUI.applyMCPMode` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_manager_actions.go:134-188` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.applyMCPMode` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/mcp_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_manager_actions.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp_manager_actions.go|chatTUI.applyMCPMode` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-110 · 圈复杂度 · `*chatTUI.collapseShellSlot`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-110 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|*chatTUI.collapseShellSlot` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 1826 |
| 函数/方法/包 | `*chatTUI.collapseShellSlot` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.collapseShellSlot` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 1826-1885 行；可见控制流热点：L1829 `if id == m.toolStreamID {`；L1836 `if resultOutput != "" {`；L1843 `if n < 0 {`；L1857 `if n == 0 {`；L1867 `if total > shellPreviewLines {`；L1876 `for i, ln := range lines {`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（connectorBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::collapseShellSlot（internal/cli/chat_tui.go:1826-1885）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::collapseToolOutput`（`internal/cli/chat_tui.go:1765`，调用点 L1809/L1813，confidence=0.70，instance-method，2 条边，待源码复核）
- **可信被调用函数（3）：** `dim`（`internal/cli/style.go:49`，调用点 L1870/L1872/L1877/L1882，confidence=0.90，exact-match，4 条边）；`connectorBlock`（`internal/cli/toolcard.go:19`，调用点 L1873/L1879/L1882，confidence=0.90，exact-match，3 条边）；`clampPlain`（`internal/cli/diffview.go:205`，调用点 L1870/L1877，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 18 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/cli/toolcard.go`、`internal/cli/diffview.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.collapseShellSlot` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1829、L1838、L1852、L1863、L1876 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.collapseShellSlot` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|*chatTUI.collapseShellSlot` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:1826-1885` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.collapseShellSlot` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|*chatTUI.collapseShellSlot` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-135 · 圈复杂度 · `loadGitStatus`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-135 |
| Baseline key | `complexity\|internal/cli/gitstatus.go\|loadGitStatus` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/cli/gitstatus.go` |
| 代码行 | 41 |
| 函数/方法/包 | `loadGitStatus` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`loadGitStatus` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 41-72 行；可见控制流热点：L43 `if err != nil {`；L47 `if root == "" {`；L52 `if branch, err := runGit(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil && st…`；L60 `if status.Branch == "" {`；L65 `if out, err := runGit(ctx, root, "diff", "--numstat", "HEAD", "--"); err == nil {`；L68 `if out, err := runGit(ctx, root, "status", "--porcelain=v1", "--untracked-files=normal"); err == ni…`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseGitNumstat）、状态与控制（runGit）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function loadGitStatus（internal/cli/gitstatus.go:41-72）
- **可信生产调用方（1）：** `fetchGitStatus`（`internal/cli/gitstatus.go:29`，调用点 L33，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `parseGitNumstat`（`internal/cli/gitstatus.go:99`，调用点 L66，confidence=0.90，exact-match）；`countUntracked`（`internal/cli/gitstatus.go:122`，调用点 L69，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `runGit`（`internal/cli/gitstatus.go:74`，调用点 L42/L52/L54/L57，confidence=0.70，exact-match，6 条边，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/gitstatus_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/gitstatus.go`、`internal/cli/gitstatus_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `loadGitStatus` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L43、L47、L60、L65、L68 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `loadGitStatus` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/gitstatus.go|loadGitStatus` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/gitstatus.go:41-72` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `loadGitStatus` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/gitstatus_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/gitstatus.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/gitstatus.go|loadGitStatus` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-136 · 圈复杂度 · `*chatTUI.runLanguageSubcommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-136 |
| Baseline key | `complexity\|internal/cli/language.go\|*chatTUI.runLanguageSubcommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/cli/language.go` |
| 代码行 | 13 |
| 函数/方法/包 | `*chatTUI.runLanguageSubcommand` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.runLanguageSubcommand` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 13-62 行；可见控制流热点：L15 `if len(args) < 2 {`；L26 `if len(args) > 2 {`；L38 `if path == "" {`；L42 `if err := edit.SetLanguage(lang); err != nil {`；L50 `if lang == "" {`；L58 `if m.ctrl != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeLanguageArg、Config::SetLanguage、Config::SaveToScope、ProfilePathResolver::UserConfigPath）、状态与控制（Config::SetLanguage）、持久化/I-O（Config::SaveToScope、ProfilePathResolver::UserConfigPath）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runLanguageSubcommand（internal/cli/language.go:13-62）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3360，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `languageDisplay`（`internal/cli/language.go:110`，调用点 L21/L61，confidence=0.90，exact-match，2 条边）；`DetectLanguage`（`internal/i18n/i18n.go:444`，调用点 L22/L57，confidence=0.90，import，2 条边）；`clearUserLanguageOverride`（`internal/cli/language.go:64`，调用点 L51，confidence=0.90，exact-match）；`normalizeLanguageArg`（`internal/cli/language.go:97`，调用点 L31，confidence=0.90，exact-match）；`describeLanguages`（`internal/cli/language.go:117`，调用点 L23，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `Config::SetLanguage`（`internal/config/edit.go:147`，调用点 L42，confidence=0.70，instance-method，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L46，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L37，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/cli`、`internal/i18n`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/language.go`、`internal/cli/chat_tui.go`、`internal/i18n/i18n.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runLanguageSubcommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L15、L26、L42、L50、L58 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.runLanguageSubcommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/language.go|*chatTUI.runLanguageSubcommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/language.go:13-62` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runLanguageSubcommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/language.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/language.go|*chatTUI.runLanguageSubcommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-155 · 圈复杂度 · `*chatTUI.switchToProvider`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-155 |
| Baseline key | `complexity\|internal/cli/provider.go\|*chatTUI.switchToProvider` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/cli/provider.go` |
| 代码行 | 73 |
| 函数/方法/包 | `*chatTUI.switchToProvider` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.switchToProvider` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 73-135 行；可见控制流热点：L75 `if err != nil {`；L82 `if p.Name == name && p.Configured() {`；L94 `if parts := strings.SplitN(m.modelRef, "/", 2); len(parts) == 2 {`；L102 `if len(models) == 0 {`；L110 `if entry.Name == curProvider && models[0] == "" {`；L130 `for _, model := range models {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（viewHeader、viewHint、viewCompactText）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::switchToProvider（internal/cli/provider.go:73-135）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runProviderCommand`（`internal/cli/provider.go:15`，调用点 L22，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `viewHeader`（`internal/cli/view_helpers.go:17`，调用点 L129，confidence=0.90，exact-match）；`viewHint`（`internal/cli/view_helpers.go:33`，调用点 L133，confidence=0.90，exact-match）；`viewCompactText`（`internal/cli/view_helpers.go:49`，调用点 L133，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/provider.go`、`internal/cli/view_helpers.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.switchToProvider` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L75、L87、L99、L110、L130 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.switchToProvider` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/provider.go|*chatTUI.switchToProvider` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/provider.go:73-135` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.switchToProvider` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/provider.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/provider.go|*chatTUI.switchToProvider` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-161 · 圈复杂度 · `*chatTUI.runSkillSubcommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-161 |
| Baseline key | `complexity\|internal/cli/skill_hooks.go\|*chatTUI.runSkillSubcommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/cli/skill_hooks.go` |
| 代码行 | 17 |
| 函数/方法/包 | `*chatTUI.runSkillSubcommand` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.runSkillSubcommand` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 17-58 行；可见控制流热点：L20 `if len(args) > 1 {`；L24 `case "":`；L30 `case "show", "cat":`；L36 `case "enable", "disable":`；L43 `if len(args) < 3 {`；L53 `if _, ok := m.ctrl.RunSkill("/" + args[1]); ok {`。CodeGraph 记录的下游职责候选主要涉及 状态与控制（chatTUI::skillSetEnabled）、持久化/I-O（chatTUI::openSkillPicker）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runSkillSubcommand（internal/cli/skill_hooks.go:17-58）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3309，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `containsArg`（`internal/cli/skill_hooks.go:296`，调用点 L47，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `chatTUI::openSkillPicker`（`internal/cli/skill_picker.go:61`，调用点 L25/L29，confidence=0.70，instance-method，2 条边，待源码复核）；`chatTUI::skillList`（`internal/cli/skill_hooks.go:60`，调用点 L27，confidence=0.70，instance-method，待源码复核）；`chatTUI::skillShow`（`internal/cli/skill_hooks.go:72`，调用点 L35，confidence=0.70，instance-method，待源码复核）；`chatTUI::skillSetEnabled`（`internal/cli/skill_hooks.go:101`，调用点 L41，confidence=0.70，instance-method，待源码复核）；`chatTUI::skillNew`（`internal/cli/skill_hooks.go:206`，调用点 L48，confidence=0.70，instance-method，待源码复核）；`chatTUI::skillPaths`（`internal/cli/skill_hooks.go:220`，调用点 L50，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/skill_picker_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/skill_hooks.go`、`internal/cli/chat_tui.go`、`internal/cli/skill_picker.go`、`internal/cli/skill_picker_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runSkillSubcommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L20、L26、L31、L42、L53 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.runSkillSubcommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/skill_hooks.go|*chatTUI.runSkillSubcommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/skill_hooks.go:17-58` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runSkillSubcommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/skill_picker_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/skill_hooks.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/skill_hooks.go|*chatTUI.runSkillSubcommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-113 · 圈复杂度 · `chatTUI.renderTodoPanel`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-113 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|chatTUI.renderTodoPanel` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 2620 |
| 函数/方法/包 | `chatTUI.renderTodoPanel` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.renderTodoPanel` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 2620-2665 行；可见控制流热点：L2624 `if err := json.Unmarshal([]byte(m.todoArgs), &p); err != nil || len(p.Todos) == 0 {`；L2629 `if t.Status == "completed" {`；L2640 `if start > 0 {`；L2648 `switch t.Status {`；L2651 `case "in_progress":`；L2661 `if end < len(p.Todos) {`。CodeGraph 记录的下游职责候选主要涉及 渲染/输出（mdRenderer::Render）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::renderTodoPanel（internal/cli/chat_tui.go:2620-2665）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `chatTUI::bottomRows`（`internal/cli/chat_tui.go:1465`，调用点 L1468，confidence=0.70，instance-method，待源码复核）；`chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2324，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `dim`（`internal/cli/style.go:49`，调用点 L2638/L2641/L2650/L2658，confidence=0.90，exact-match，5 条边）；`todoPanelWindow`（`internal/cli/chat_tui.go:2667`，调用点 L2639，confidence=0.90，exact-match）；`green`（`internal/cli/style.go:50`，调用点 L2650，confidence=0.90，exact-match）；`yellow`（`internal/cli/style.go:52`，调用点 L2656，confidence=0.90，exact-match）；`accent`（`internal/cli/style.go:53`，调用点 L2638，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `mdRenderer::Render`（`internal/cli/md.go:56`，调用点 L2664，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2324，confidence=0.70，instance-method，待源码复核）；`chatREPL`（`internal/cli/cli.go:652`，调用点 L809，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 19 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.renderTodoPanel` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L2624、L2633、L2645、L2649、L2661 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `chatTUI.renderTodoPanel` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|chatTUI.renderTodoPanel` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:2620-2665` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.renderTodoPanel` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|chatTUI.renderTodoPanel` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-111 · 圈复杂度 · `*chatTUI.toggleShellOutput`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-111 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|*chatTUI.toggleShellOutput` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 1890 |
| 函数/方法/包 | `*chatTUI.toggleShellOutput` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.toggleShellOutput` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 1890-1945 行；可见控制流热点：L1894 `for id, idx := range m.shellTranscriptIdx {`；L1900 `if lastID == "" {`；L1910 `if innerW < 10 {`；L1919 `for i := 0; i < shellPreviewLines; i++ {`；L1933 `for i := 0; i < show; i++ {`；L1942 `if m.nativeScrollback {`。CodeGraph 记录的下游职责候选主要涉及 任务/并发（connectorBlock）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::toggleShellOutput（internal/cli/chat_tui.go:1890-1945）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L763/L1076，confidence=0.70，instance-method，2 条边，待源码复核）
- **可信被调用函数（3）：** `dim`（`internal/cli/style.go:49`，调用点 L1920/L1922/L1934/L1937，confidence=0.90，exact-match，4 条边）；`clampPlain`（`internal/cli/diffview.go:205`，调用点 L1920/L1934，confidence=0.90，exact-match，2 条边）；`connectorBlock`（`internal/cli/toolcard.go:19`，调用点 L1923/L1939，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（1）：** `chatTUI::commitLine`（`internal/cli/chat_tui.go:1446`，调用点 L1943，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L763，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 9 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/style.go`、`internal/cli/diffview.go`、`internal/cli/toolcard.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.toggleShellOutput` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1894、L1904、L1917、L1929、L1942 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 任务/并发 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.toggleShellOutput` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|*chatTUI.toggleShellOutput` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:1890-1945` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.toggleShellOutput` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|*chatTUI.toggleShellOutput` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-107 · 圈复杂度 · `chatTUI.renderTranscriptWithMainManager`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-107 |
| Baseline key | `complexity\|internal/cli/chat_tui.go\|chatTUI.renderTranscriptWithMainManager` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/cli/chat_tui.go` |
| 代码行 | 1568 |
| 函数/方法/包 | `chatTUI.renderTranscriptWithMainManager` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.renderTranscriptWithMainManager` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1568-1604 行；可见控制流热点：L1570 `if h <= 0 {`；L1574 `if cw <= 0 {`；L1583 `if maxTranscriptRows > 0 && len(cardLines) > 0 && len(m.wrappedLines) > 0 {`；L1588 `if maxTranscriptRows > 0 {`；L1597 `for len(rows) < h {`；L1600 `for i, row := range rows {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::renderTranscriptWithMainManager（internal/cli/chat_tui.go:1568-1604）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2397，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `padRight`（`internal/cli/box.go:22`，调用点 L1601，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::View`（`internal/cli/chat_tui.go:2210`，调用点 L2397，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 20 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/chat_tui.go`、`internal/cli/box.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.renderTranscriptWithMainManager` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1570、L1579、L1588、L1593、L1600 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `chatTUI.renderTranscriptWithMainManager` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/chat_tui.go|chatTUI.renderTranscriptWithMainManager` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/chat_tui.go:1568-1604` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.renderTranscriptWithMainManager` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G42 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/chat_tui.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/chat_tui.go|chatTUI.renderTranscriptWithMainManager` 下降或消失且无新增/恶化。
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
