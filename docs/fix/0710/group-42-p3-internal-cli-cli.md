# G42 · internal/cli · cli 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G42 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/cli` |
| 涉及文件 | `internal/cli/skill_hooks.go`<br>`internal/cli/theme_osc_unix.go`<br>`internal/cli/cli.go`<br>`internal/cli/memory_v5.go`<br>`internal/cli/reasoning_language.go`<br>`internal/cli/mcp_import_picker.go`<br>`internal/cli/theme.go` |
| 主要治理目标 | 围绕 `internal/cli/cli.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | cli 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 高：建议串行治理、单一负责人 |
| 冲突依据 | 本地未合并远端引用重叠：`origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614`、`origin/expert-202607081622`；单文件最多集中 13 条问题；涉及核心运行链路 |
| 建议处理顺序 | 总序第 42 组；等待 G14、G39 的同文件变更合并 |
| 前置依赖 | G14（internal/cli/cli.go）；G39（internal/cli/skill_hooks.go） |
| 后续验证重点 | 持久化、迁移或文件格式兼容；会话、状态或恢复语义；协议、序列化或流式事件兼容；配置、平台或向后兼容；返回值、错误语义和现有调用顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **39/100**，对象 `*chatTUI.skillSaveEnabledChanges` 的分项为严重度 3.3、集中度 3.6、影响面 15.9、可执行性 16.0；最高超限 1.30x，文件内 2 条问题，CodeGraph 记录 0 个可信生产调用方、2 个待核候选。

### 治理边界与组间关系

本组涉及 7 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G14、G39。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-162` | `complexity\|internal/cli/skill_hooks.go\|*chatTUI.skillSaveEnabledChanges` | 圈复杂度 | `internal/cli/skill_hooks.go` | `*chatTUI.skillSaveEnabledChanges` | 105 | 13/10 | +3（1.30x） | P3 | 0 可信 + 2 待核 caller / 1 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-168` | `complexity\|internal/cli/theme_osc_unix.go\|queryTerminalBackground` | 圈复杂度 | `internal/cli/theme_osc_unix.go` | `queryTerminalBackground` | 19 | 16/10 | +6（1.60x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-128` | `complexity\|internal/cli/cli.go\|configReasoningLanguageCommand` | 圈复杂度 | `internal/cli/cli.go` | `configReasoningLanguageCommand` | 1974 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 7 可信 + 5 待核 callee / 18 unresolved | 待修改 |
| `CX-153` | `complexity\|internal/cli/memory_v5.go\|*chatTUI.runMemoryV5Command` | 圈复杂度 | `internal/cli/memory_v5.go` | `*chatTUI.runMemoryV5Command` | 10 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-156` | `complexity\|internal/cli/reasoning_language.go\|*chatTUI.runReasoningLanguageCommand` | 圈复杂度 | `internal/cli/reasoning_language.go` | `*chatTUI.runReasoningLanguageCommand` | 10 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-141` | `complexity\|internal/cli/mcp_import_picker.go\|chatTUI.handleMCPImportKey` | 圈复杂度 | `internal/cli/mcp_import_picker.go` | `chatTUI.handleMCPImportKey` | 40 | 12/10 | +2（1.20x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-127` | `complexity\|internal/cli/cli.go\|configMemoryV5Command` | 圈复杂度 | `internal/cli/cli.go` | `configMemoryV5Command` | 1927 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 5 可信 + 4 待核 callee / 15 unresolved | 待修改 |
| `CX-169` | `complexity\|internal/cli/theme.go\|*chatTUI.themeArgItems` | 圈复杂度 | `internal/cli/theme.go` | `*chatTUI.themeArgItems` | 501 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-126` | `complexity\|internal/cli/cli.go\|appendEnv` | 圈复杂度 | `internal/cli/cli.go` | `appendEnv` | 1800 | 16/10 | +6（1.60x） | P3 | 0 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 23 unresolved | 待修改 |
| `CX-125` | `complexity\|internal/cli/cli.go\|providersWithMissingKeys` | 圈复杂度 | `internal/cli/cli.go` | `providersWithMissingKeys` | 1688 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 12 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-162 · 圈复杂度 · `*chatTUI.skillSaveEnabledChanges`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-162 |
| Baseline key | `complexity\|internal/cli/skill_hooks.go\|*chatTUI.skillSaveEnabledChanges` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 39） |
| 文件 | `internal/cli/skill_hooks.go` |
| 代码行 | 105 |
| 函数/方法/包 | `*chatTUI.skillSaveEnabledChanges` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.skillSaveEnabledChanges` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 105-154 行；可见控制流热点：L106 `if len(changes) == 0 {`；L113 `if m.ctrl == nil {`；L122 `for _, sk := range m.ctrl.AllSkills() {`；L131 `for name, enabled := range changes {`；L139 `if len(changes) == 1 {`；L145 `if enabled {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::skillSaveEnabledChanges（internal/cli/skill_hooks.go:105-154）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `chatTUI::skillSetEnabled`（`internal/cli/skill_hooks.go:101`，调用点 L102，confidence=0.70，instance-method，待源码复核）；`chatTUI::saveSkillPick`（`internal/cli/skill_picker.go:288`，调用点 L299，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `enableVerb`（`internal/cli/skill_hooks.go:199`，调用点 L127/L134，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（1）：** `chatTUI::scheduleSkillSessionRefresh`（`internal/cli/skill_hooks.go:156`，调用点 L153，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/profile_entry_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/skill_hooks.go`、`internal/cli/skill_picker.go`、`internal/cli/profile_entry_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.skillSaveEnabledChanges` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L106、L117、L126、L133、L145 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*chatTUI.skillSaveEnabledChanges` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/skill_hooks.go|*chatTUI.skillSaveEnabledChanges` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/skill_hooks.go:105-154` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.skillSaveEnabledChanges` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/profile_entry_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/skill_hooks.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/skill_hooks.go|*chatTUI.skillSaveEnabledChanges` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-168 · 圈复杂度 · `queryTerminalBackground`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-168 |
| Baseline key | `complexity\|internal/cli/theme_osc_unix.go\|queryTerminalBackground` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/cli/theme_osc_unix.go` |
| 代码行 | 19 |
| 函数/方法/包 | `queryTerminalBackground` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`queryTerminalBackground` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 19-70 行；可见控制流热点：L20 `if !colorEnabled {`；L30 `if err != nil {`；L39 `if err := unix.SetNonblock(inFd, true); err != nil {`；L51 `for time.Now().Before(deadline) && len(response) < terminalBGQueryMaxBytes {`；L55 `if rgb, ok := parseOSC11Response(string(response)); ok {`；L63 `if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseOSC11Response）、网络/协议（parseOSC11Response）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function queryTerminalBackground（internal/cli/theme_osc_unix.go:19-70）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `parseOSC11Response`（`internal/cli/theme.go:241`，调用点 L55/L69，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/theme_osc_unix.go`、`internal/cli/theme.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `queryTerminalBackground` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L20、L36、L44、L55、L63 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `queryTerminalBackground` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/theme_osc_unix.go|queryTerminalBackground` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/theme_osc_unix.go:19-70` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `queryTerminalBackground` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/theme_osc_unix.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/theme_osc_unix.go|queryTerminalBackground` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-128 · 圈复杂度 · `configReasoningLanguageCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-128 |
| Baseline key | `complexity\|internal/cli/cli.go\|configReasoningLanguageCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 1974 |
| 函数/方法/包 | `configReasoningLanguageCommand` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`configReasoningLanguageCommand` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 1974-2038 行；可见控制流热点：L1977 `if err := fs.Parse(args); err != nil {`；L1985 `if len(rest) == 0 {`；L2000 `if *local {`；L2007 `if *local {`；L2022 `if err := cfg.SetReasoningLanguage(mode); err != nil {`；L2032 `if saveErr != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadCLIConfig、defaultCLIUserConfigPath、configReasoningLanguageUsage、parseCLIReasoningLanguage）、持久化/I-O（SaveMinimalProjectReasoningLanguage、Config::SaveTo、Config::SaveToScope）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function configReasoningLanguageCommand（internal/cli/cli.go:1974-2038）
- **可信生产调用方（1）：** `configCommand`（`internal/cli/cli.go:1865`，调用点 L1876，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（7）：** `loadCLIConfig`（`internal/cli/cli.go:244`，调用点 L1986，confidence=0.90，exact-match）；`defaultCLIUserConfigPath`（`internal/cli/cli.go:884`，调用点 L1999，confidence=0.90，exact-match）；`configReasoningLanguageUsage`（`internal/cli/cli.go:2060`，调用点 L1982，confidence=0.90，exact-match）；`parseCLIReasoningLanguage`（`internal/cli/reasoning_language.go:57`，调用点 L1994，confidence=0.90，exact-match）；`cliReasoningLanguageMode`（`internal/cli/reasoning_language.go:70`，调用点 L1991，confidence=0.90，exact-match）；`SaveMinimalProjectReasoningLanguage`（`internal/config/edit.go:957`，调用点 L2009，confidence=0.90，import）；`LoadForEdit`（`internal/config/load.go:392`，调用点 L2021，confidence=0.90，import）
- **待源码复核被调候选（5）：** `displayPath`（`internal/cli/cli.go:915`，调用点 L2014/L2036，confidence=0.70，exact-match，2 条边，待源码复核）；`Config::ReasoningLanguage`（`internal/config/config.go:396`，调用点 L1991/L2036，confidence=0.70，instance-method，2 条边，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L1977，confidence=0.70，instance-method，待源码复核）；`Config::SaveTo`（`internal/config/edit.go:809`，调用点 L2028，confidence=0.70，instance-method，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L2030，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L101，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 18 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/cli/reasoning_language.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `configReasoningLanguageCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1977、L1987、L2003、L2010、L2032 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `configReasoningLanguageCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|configReasoningLanguageCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:1974-2038` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `configReasoningLanguageCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|configReasoningLanguageCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-153 · 圈复杂度 · `*chatTUI.runMemoryV5Command`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-153 |
| Baseline key | `complexity\|internal/cli/memory_v5.go\|*chatTUI.runMemoryV5Command` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/cli/memory_v5.go` |
| 代码行 | 10 |
| 函数/方法/包 | `*chatTUI.runMemoryV5Command` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.runMemoryV5Command` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 10-54 行；可见控制流热点：L12 `if len(args) > 2 {`；L18 `if err != nil {`；L25 `if m.ctrl != nil && m.ctrl.Running() {`；L37 `if path == "" {`；L41 `if err := edit.SetMemoryCompilerEnabled(enabled); err != nil {`；L50 `if m.ctrl != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseCLIMemoryV5Mode、Config::SaveToScope、ProfilePathResolver::UserConfigPath）、持久化/I-O（Config::SaveToScope、ProfilePathResolver::UserConfigPath）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runMemoryV5Command（internal/cli/memory_v5.go:10-54）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3279，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `cliMemoryV5Mode`（`internal/cli/memory_v5.go:67`，调用点 L22/L53，confidence=0.90，exact-match，2 条边）；`parseCLIMemoryV5Mode`（`internal/cli/memory_v5.go:56`，调用点 L29，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L45，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L36，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/memory_v5.go`、`internal/cli/chat_tui.go`、`internal/config/edit.go`、`internal/config/paths.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runMemoryV5Command` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L12、L18、L30、L41、L50 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.runMemoryV5Command` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/memory_v5.go|*chatTUI.runMemoryV5Command` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/memory_v5.go:10-54` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runMemoryV5Command` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/memory_v5.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/memory_v5.go|*chatTUI.runMemoryV5Command` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-156 · 圈复杂度 · `*chatTUI.runReasoningLanguageCommand`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-156 |
| Baseline key | `complexity\|internal/cli/reasoning_language.go\|*chatTUI.runReasoningLanguageCommand` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 37） |
| 文件 | `internal/cli/reasoning_language.go` |
| 代码行 | 10 |
| 函数/方法/包 | `*chatTUI.runReasoningLanguageCommand` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.runReasoningLanguageCommand` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 10-55 行；可见控制流热点：L12 `if len(args) < 2 {`；L21 `if len(args) > 2 {`；L25 `if m.ctrl != nil && m.ctrl.Running() {`；L37 `if path == "" {`；L41 `if err := edit.SetReasoningLanguage(mode); err != nil {`；L51 `if m.ctrl != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseCLIReasoningLanguage、Config::SaveToScope、ProfilePathResolver::UserConfigPath）、持久化/I-O（Config::SaveToScope、ProfilePathResolver::UserConfigPath）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::runReasoningLanguageCommand（internal/cli/reasoning_language.go:10-55）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::runSlashCommand`（`internal/cli/chat_tui.go:3226`，调用点 L3276，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `parseCLIReasoningLanguage`（`internal/cli/reasoning_language.go:57`，调用点 L29，confidence=0.90，exact-match）；`cliReasoningLanguageMode`（`internal/cli/reasoning_language.go:70`，调用点 L18，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L45，confidence=0.70，instance-method，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L36，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L1173，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/profile_entry_test.go`、`internal/cli/chat_tui_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/reasoning_language.go`、`internal/cli/chat_tui.go`、`internal/config/edit.go`、`internal/config/paths.go`、`internal/cli/profile_entry_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.runReasoningLanguageCommand` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L12、L21、L30、L41、L51 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*chatTUI.runReasoningLanguageCommand` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/reasoning_language.go|*chatTUI.runReasoningLanguageCommand` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/reasoning_language.go:10-55` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.runReasoningLanguageCommand` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/profile_entry_test.go`、`internal/cli/chat_tui_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/reasoning_language.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/reasoning_language.go|*chatTUI.runReasoningLanguageCommand` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-141 · 圈复杂度 · `chatTUI.handleMCPImportKey`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-141 |
| Baseline key | `complexity\|internal/cli/mcp_import_picker.go\|chatTUI.handleMCPImportKey` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/cli/mcp_import_picker.go` |
| 代码行 | 40 |
| 函数/方法/包 | `chatTUI.handleMCPImportKey` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`chatTUI.handleMCPImportKey` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 40-77 行；可见控制流热点：L42 `switch msg.String() {`；L46 `case "up", "k":`；L50 `case "down", "j":`；L56 `case "enter":`；L59 `if ok {`；L69 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::handleMCPImportKey（internal/cli/mcp_import_picker.go:40-77）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L895，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `chatTUI::update`（`internal/cli/chat_tui.go:713`，调用点 L895，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/mcp_import_picker.go`、`internal/cli/chat_tui.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `chatTUI.handleMCPImportKey` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L42、L47、L54、L58、L69 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `chatTUI.handleMCPImportKey` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/mcp_import_picker.go|chatTUI.handleMCPImportKey` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/mcp_import_picker.go:40-77` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `chatTUI.handleMCPImportKey` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/mcp_import_picker.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/mcp_import_picker.go|chatTUI.handleMCPImportKey` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-127 · 圈复杂度 · `configMemoryV5Command`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-127 |
| Baseline key | `complexity\|internal/cli/cli.go\|configMemoryV5Command` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 1927 |
| 函数/方法/包 | `configMemoryV5Command` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`configMemoryV5Command` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 1927-1972 行；可见控制流热点：L1930 `if err := fs.Parse(args); err != nil {`；L1938 `if len(rest) > 1 {`；L1942 `if len(rest) == 0 || strings.EqualFold(rest[0], "status") {`；L1952 `if err != nil {`；L1957 `if path == "" {`；L1966 `if err := cfg.SaveToScope(path, config.RenderScopeUser); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadCLIConfig、defaultCLIUserConfigPath、configMemoryV5Usage、parseCLIMemoryV5Mode）、持久化/I-O（Config::SaveToScope）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function configMemoryV5Command（internal/cli/cli.go:1927-1972）
- **可信生产调用方（1）：** `configCommand`（`internal/cli/cli.go:1865`，调用点 L1874，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `loadCLIConfig`（`internal/cli/cli.go:244`，调用点 L1943，confidence=0.90，exact-match）；`defaultCLIUserConfigPath`（`internal/cli/cli.go:884`，调用点 L1956，confidence=0.90，exact-match）；`configMemoryV5Usage`（`internal/cli/cli.go:2054`，调用点 L1939，confidence=0.90，exact-match）；`parseCLIMemoryV5Mode`（`internal/cli/memory_v5.go:56`，调用点 L1951，confidence=0.90，exact-match）；`LoadForEdit`（`internal/config/load.go:392`，调用点 L1961，confidence=0.90，import）
- **待源码复核被调候选（4）：** `Config::MemoryCompilerEnabled`（`internal/config/config.go:883`，调用点 L1948/L1970，confidence=0.70，instance-method，2 条边，待源码复核）；`displayPath`（`internal/cli/cli.go:915`，调用点 L1970，confidence=0.70，exact-match，待源码复核）；`mathParser::Parse`（`internal/cli/mathnode.go:26`，调用点 L1930，confidence=0.70，instance-method，待源码复核）；`Config::SaveToScope`（`internal/config/edit.go:820`，调用点 L1966，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L101，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 15 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/cli/memory_v5.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `configMemoryV5Command` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1930、L1938、L1944、L1957、L1966 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `configMemoryV5Command` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|configMemoryV5Command` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:1927-1972` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `configMemoryV5Command` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|configMemoryV5Command` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-169 · 圈复杂度 · `*chatTUI.themeArgItems`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-169 |
| Baseline key | `complexity\|internal/cli/theme.go\|*chatTUI.themeArgItems` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/cli/theme.go` |
| 代码行 | 501 |
| 函数/方法/包 | `*chatTUI.themeArgItems` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*chatTUI.themeArgItems` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 501-539 行；可见控制流热点：L503 `if cmdEnd < 0 || val[:cmdEnd] != "/theme" {`；L508 `if len(prior) != 1 {`；L522 `for _, it := range items {`；L528 `for _, st := range cliThemeStyles {`；L529 `if cur != "" && !strings.HasPrefix(st.name, cur) {`；L533 `if st.name == activeCLITheme.style {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method chatTUI::themeArgItems（internal/cli/theme.go:501-539）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `chatTUI::slashArgItems`（`internal/cli/complete.go:161`，调用点 L168，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/cli_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/theme.go`、`internal/cli/complete.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`

#### 待修改项

- [ ] 先冻结 `*chatTUI.themeArgItems` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L503、L522、L523、L529、L533 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*chatTUI.themeArgItems` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/theme.go|*chatTUI.themeArgItems` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/theme.go:501-539` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*chatTUI.themeArgItems` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/theme.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/theme.go|*chatTUI.themeArgItems` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-126 · 圈复杂度 · `appendEnv`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-126 |
| Baseline key | `complexity\|internal/cli/cli.go\|appendEnv` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 32） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 1800 |
| 函数/方法/包 | `appendEnv` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`appendEnv` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 1800-1845 行；可见控制流热点：L1802 `for _, l := range lines {`；L1809 `if data, err := os.ReadFile(path); err == nil {`；L1813 `if k, _, ok := strings.Cut(check, "="); ok && target[strings.TrimSpace(k)] {`；L1828 `for _, l := range kept {`；L1835 `if k, v, ok := strings.Cut(l, "="); ok {`；L1840 `if err := os.MkdirAll(dir, 0o755); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function appendEnv（internal/cli/cli.go:1800-1845）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 23 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `appendEnv` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1802、L1810、L1820、L1835、L1840 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `appendEnv` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|appendEnv` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:1800-1845` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `appendEnv` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|appendEnv` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-125 · 圈复杂度 · `providersWithMissingKeys`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-125 |
| Baseline key | `complexity\|internal/cli/cli.go\|providersWithMissingKeys` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 29） |
| 文件 | `internal/cli/cli.go` |
| 代码行 | 1688 |
| 函数/方法/包 | `providersWithMissingKeys` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`providersWithMissingKeys` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 1688-1726 行；可见控制流热点：L1689 `if cfg == nil {`；L1697 `if !strings.EqualFold(strings.TrimSpace(cfg.Agent.AutoPlan), "off") {`；L1702 `for key := range cfg.Agent.SubagentModels {`；L1706 `for _, key := range keys {`；L1715 `if ref == "" {`；L1719 `if !ok || p.APIKeyEnv == "" || os.Getenv(p.APIKeyEnv) != "" || seen[p.APIKeyEnv] {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Config::ResolveModel）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function providersWithMissingKeys（internal/cli/cli.go:1688-1726）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Config::ResolveModel`（`internal/config/config.go:1321`，调用点 L1718，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 12 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/cli`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/cli/cli_test.go`
- **同目录测试候选：** `internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`、`internal/cli/bot_test.go`、`internal/cli/box_test.go`、`internal/cli/branch_test.go`、`internal/cli/chat_attachment_test.go`、`internal/cli/chat_ghost_test.go`、`internal/cli/chat_render_test.go`、`internal/cli/chat_tui_test.go`、`internal/cli/compaction_test.go`
- **需同步检查：** `internal/cli/cli.go`、`internal/config/config.go`、`internal/cli/cli_test.go`、`internal/cli/acp_test.go`、`internal/cli/actor_test.go`、`internal/cli/automation_boundary_test.go`

#### 待修改项

- [ ] 先冻结 `providersWithMissingKeys` 的现有输入输出、错误、副作用和调用顺序，补齐或确认特征测试；热点/冲突对象不得直接大拆。
- [ ] 逐段核对 L1689、L1700、L1706、L1713、L1719 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `providersWithMissingKeys` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/cli/cli.go|providersWithMissingKeys` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/cli/cli.go:1688-1726` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `providersWithMissingKeys` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用中 `origin/codex/im-session-refresh`、`origin/codex/integration-main-v3-20260708-2`、`origin/codex/suiyuan-official-provider`、`origin/codex/suiyuan-official-provider-main-v3`、`origin/expert-07-08-1614` 涉及该文件；认领前必须刷新远端并确认对应 PR/分支状态。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/cli/cli_test.go`。
- [ ] 运行相关包测试：`go test ./internal/cli`。
- [ ] 运行相关包 vet：`go vet ./internal/cli`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/cli/cli.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/cli/cli.go|providersWithMissingKeys` 下降或消失且无新增/恶化。
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
